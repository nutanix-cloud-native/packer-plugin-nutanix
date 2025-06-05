package nutanix

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/mitchellh/go-vnc"
	"golang.org/x/net/websocket"
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

	var c *vnc.ClientConn
	var err error

	ui.Say("Connecting to VNC over websocket...")
	c, err = s.ConnectVNCOverWebsocketClient(state)
	if err != nil {
		err = fmt.Errorf("error connecting to VNC: %s", err)
		state.Put("error", err)
		ui.Error(err.Error())
		return multistep.ActionHalt
	}

	state.Put("vnc_conn", c)
	return multistep.ActionContinue

}

func (s *stepVNCConnect) ConnectVNCOverWebsocketClient(state multistep.StateBag) (*vnc.ClientConn, error) {

	log.Printf("retrieving auth cookie for VNC connection...")
	cookie, err := s.getAuthCookie(state)
	if err != nil {
		return nil, fmt.Errorf("failed to get auth cookie: %v", err)
	}

	vmUUID := state.Get("vm_uuid").(string)
	clusterUUID := state.Get("cluster_uuid").(string)

	wsURL := fmt.Sprintf("wss://%s:%d/vnc/vm/%s/proxy?proxyClusterUuid=%s", s.Config.ClusterConfig.Endpoint, s.Config.ClusterConfig.Port, vmUUID, clusterUUID)

	// Parse the URL
	u, err := url.Parse(wsURL)
	if err != nil {
		err = fmt.Errorf("error parsing websocket url: %s", err)
		return nil, err
	}

	// Configure websocket
	wsConfig := websocket.Config{
		Location: u,
		Origin:   &url.URL{Scheme: "http", Host: "localhost"},
		Version:  websocket.ProtocolVersionHybi13,
		Header:   http.Header{},
	}

	// Add cookies to the WebSocket configuration headers
	var cookieHeader []string
	for _, c := range cookie {
		cookieHeader = append(cookieHeader, fmt.Sprintf("%s=%s", c.Name, c.Value))
	}
	wsConfig.Header.Set("Cookie", strings.Join(cookieHeader, "; "))

	wsConfig.TlsConfig = &tls.Config{
		InsecureSkipVerify: s.Config.ClusterConfig.Insecure,
	}

	// Connect to the WebSocket server
	log.Printf("connecting to %s\n", wsURL)
	ws, err := websocket.DialConfig(&wsConfig)
	if err != nil {
		log.Printf("websocket connection failed: %v", err)
		return nil, fmt.Errorf("websocket connection failed: %v", err)
	}

	// Set up the VNC connection over the websocket.
	ccconfig := &vnc.ClientConfig{
		Auth:      []vnc.ClientAuth{new(vnc.ClientAuthNone)},
		Exclusive: false,
	}
	c, err := vnc.Client(ws, ccconfig)
	if err != nil {
		err = fmt.Errorf("error setting the VNC over websocket client: %s", err)

		return nil, err
	}

	return c, nil
}

func (s *stepVNCConnect) getAuthCookie(state multistep.StateBag) ([]*http.Cookie, error) {

	loginURL := fmt.Sprintf("https://%s:%d/api/nutanix/v3/users/me", s.Config.ClusterConfig.Endpoint, s.Config.ClusterConfig.Port)

	req, err := http.NewRequest("GET", loginURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Add("Content-Type", "application/json")
	req.SetBasicAuth(s.Config.ClusterConfig.Username, s.Config.ClusterConfig.Password)

	// Create a custom HTTP client that skips SSL verification
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: s.Config.ClusterConfig.Insecure},
	}

	// Create a cookie jar
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("error creating cookie jar: %v", err)
	}

	client := &http.Client{
		Transport: tr,
		Timeout:   10 * time.Second,
		Jar:       jar,
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %v", err)
	}
	defer resp.Body.Close()

	// Read and log response body for debugging
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %s", string(bodyBytes))
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("authentication failed: status %d", resp.StatusCode)
	}

	cookies := jar.Cookies(req.URL)

	for _, cookie := range cookies {
		if cookie.Name == "NTNX_MERCURY_IAM_SESSION" {
			log.Printf("auth cookie retrieved successfully")
			return cookies, nil
		}
	}

	return nil, fmt.Errorf("no session cookie found")
}

func (s *stepVNCConnect) Cleanup(state multistep.StateBag) {
	// No cleanup needed
}
