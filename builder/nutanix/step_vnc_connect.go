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
	VNCEnabled         bool
	InsecureConnection bool
	ClusterConfig      *ClusterConfig
}

func (s *stepVNCConnect) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	if !s.VNCEnabled {
		return multistep.ActionContinue
	}
	ui := state.Get("ui").(packer.Ui)

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
	cookie, err := s.getAuthCookie(state)
	if err != nil {
		return nil, fmt.Errorf("failed to get auth cookie: %v", err)
	}

	vmUUID := state.Get("vm_uuid").(string)
	clusterUUID := state.Get("cluster_uuid").(string)

	wsURL := fmt.Sprintf("wss://%s:9440/vnc/vm/%s/proxy?proxyClusterUuid=%s", s.ClusterConfig.Endpoint, vmUUID, clusterUUID)

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

	// Connect to the WebSocket server
	log.Printf("Connecting to %s\n", wsURL)
	ws, err := websocket.DialConfig(&wsConfig)
	if err != nil {
		log.Fatalf("WebSocket connection failed: %v", err)
		return nil, err
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

	loginURL := fmt.Sprintf("https://%s:9440/api/nutanix/v3/clusters/list", s.ClusterConfig.Endpoint)
	payload := strings.NewReader(`{}`)

	req, err := http.NewRequest("POST", loginURL, payload)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %v", err)
	}
	req.Header.Add("Content-Type", "application/json")
	req.SetBasicAuth(s.ClusterConfig.Username, s.ClusterConfig.Password)

	// Create a custom HTTP client that skips SSL verification
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
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
		return nil, fmt.Errorf("failed to read response body: %v", fmt.Errorf("failed to read response body: %s", string(bodyBytes)))
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("authentication failed: status %d", resp.StatusCode)
	}

	cookies := jar.Cookies(req.URL)

	for _, cookie := range cookies {
		if cookie.Name == "NTNX_MERCURY_IAM_SESSION" {
			return cookies, nil
		}
	}

	return nil, fmt.Errorf("no session cookie found")
}

func (s *stepVNCConnect) Cleanup(state multistep.StateBag) {
	// No cleanup needed
}
