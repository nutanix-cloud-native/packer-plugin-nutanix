package nutanix

import (
	"context"
	"crypto/tls"
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/mitchellh/go-vnc"
	"golang.org/x/net/websocket"
)

const (
	ntnxAPIKeyHeaderKey = "X-ntnx-api-key"
)

type stepVNCConnect struct {
	Config *Config
}

func (s *stepVNCConnect) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packer.Ui)

	if s.Config.BootCommand == nil {
		return multistep.ActionContinue
	}

	if s.Config.DisableVNC {
		return multistep.ActionContinue
	}

	ui.Say("Connecting to VNC over websocket...")
	c, err := s.ConnectVNCOverWebsocketClient(ctx, state)
	if err != nil {
		err = fmt.Errorf("error connecting to VNC: %s", err)
		state.Put("error", err)
		ui.Error(err.Error())
		return multistep.ActionHalt
	}

	state.Put("vnc_conn", c)
	return multistep.ActionContinue
}

func (s *stepVNCConnect) ConnectVNCOverWebsocketClient(ctx context.Context, state multistep.StateBag) (*vnc.ClientConn, error) {
	vmUUID := state.Get("vm_uuid").(string)
	driver := state.Get("driver").(Driver)

	log.Printf("generating VNC console token for VM %s via V4 API...", vmUUID)
	token, wsUri, err := driver.GenerateConsoleToken(ctx, vmUUID)
	if err != nil {
		return nil, fmt.Errorf("failed to generate console token: %v", err)
	}

	wsURL := fmt.Sprintf("wss://%s:%d%s?VmConsoleToken=%s",
		s.Config.ClusterConfig.Endpoint, s.Config.ClusterConfig.Port, wsUri, url.QueryEscape(token))
	log.Printf("VNC websocket target: wss://%s:%d%s?VmConsoleToken=<redacted>",
		s.Config.ClusterConfig.Endpoint, s.Config.ClusterConfig.Port, wsUri)

	u, err := url.Parse(wsURL)
	if err != nil {
		return nil, fmt.Errorf("error parsing websocket url: %s", err)
	}

	// Origin must match Prism Central URL - server validates this for console access
	originURL := &url.URL{
		Scheme: "https",
		Host:   fmt.Sprintf("%s:%d", s.Config.ClusterConfig.Endpoint, s.Config.ClusterConfig.Port),
	}
	header := http.Header{}
	// Include Basic Auth when not using API key - some IAM-enabled PCs require it for console
	if s.Config.ClusterConfig.Username != "X-ntnx-api-key" {
		header.Set("Authorization", "Basic "+basicAuth(s.Config.ClusterConfig.Username, s.Config.ClusterConfig.Password))
	} else {
		header.Set(ntnxAPIKeyHeaderKey, s.Config.ClusterConfig.Password)
	}
	wsConfig := websocket.Config{
		Location: u,
		Origin:   originURL,
		Version:  websocket.ProtocolVersionHybi13,
		Header:   header,
		TlsConfig: &tls.Config{
			InsecureSkipVerify: s.Config.ClusterConfig.Insecure,
		},
	}

	log.Printf("connecting to VNC websocket (Origin: %s)...", originURL.String())
	ws, err := websocket.DialConfig(&wsConfig)
	if err != nil {
		// Probe to capture HTTP status when handshake fails (helps debug 401/403 etc)
		if probeBody, _ := s.probeWebsocketHandshake(wsURL, originURL.String()); probeBody != "" {
			log.Printf("websocket handshake failed - probe response: %s", probeBody)
			return nil, fmt.Errorf("websocket connection failed: %v (probe: %s)", err, probeBody)
		}
		return nil, fmt.Errorf("websocket connection failed: %v", err)
	}

	c, err := vnc.Client(ws, &vnc.ClientConfig{
		Auth:      []vnc.ClientAuth{new(vnc.ClientAuthNone)},
		Exclusive: false,
	})
	if err != nil {
		return nil, fmt.Errorf("error setting the VNC over websocket client: %s", err)
	}

	return c, nil
}

func basicAuth(username, password string) string {
	auth := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(auth))
}

func (s *stepVNCConnect) Cleanup(state multistep.StateBag) {
	// No cleanup needed
}

// probeWebsocketHandshake sends an HTTP request mimicking a websocket upgrade to capture
// the server's response status and body. Used for debugging when the real websocket
// handshake fails (e.g. 401, 403, 302).
func (s *stepVNCConnect) probeWebsocketHandshake(wsURL, origin string) (string, error) {
	req, err := http.NewRequest("GET", wsURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "Upgrade")
	req.Header.Set("Origin", origin)
	req.Header.Set("Sec-WebSocket-Version", "13")
	req.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")

	client := &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: s.Config.ClusterConfig.Insecure},
		},
		Timeout: 10 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return fmt.Sprintf("status=%d body=%s", resp.StatusCode, string(body)), nil
}
