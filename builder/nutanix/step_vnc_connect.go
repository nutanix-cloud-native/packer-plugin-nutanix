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
	"sort"
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
	wsConfig := websocket.Config{
		Location: u,
		Origin:   originURL,
		Version:  websocket.ProtocolVersionHybi13,
		Header:   s.consoleHeaders(),
		TlsConfig: &tls.Config{
			InsecureSkipVerify: s.Config.ClusterConfig.Insecure,
		},
	}

	log.Printf("connecting to VNC websocket (Origin: %s)...", originURL.String())
	ws, err := websocket.DialConfig(&wsConfig)
	if err != nil {
		// The dial error includes the full URL, console token and all, and the
		// probe reports what the server sent back, which can echo what we sent.
		// Redact every secret we sent before either is logged or returned.
		redact := s.secretRedactor(token)
		// Probe to capture HTTP status when handshake fails (helps debug 401/403 etc).
		// The probe result comes back already redacted.
		if probeBody, _ := s.probeWebsocketHandshake(wsURL, originURL.String(), redact); probeBody != "" {
			log.Printf("websocket handshake failed - probe response: %s", probeBody)
			return nil, fmt.Errorf("websocket connection failed: %s (probe: %s)", redact(err.Error()), probeBody)
		}
		return nil, fmt.Errorf("websocket connection failed: %s", redact(err.Error()))
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

// consoleHeaders returns the auth and custom headers for the console
// websocket upgrade. Auth prefers the explicit APIKey field, then the legacy
// Username == "X-ntnx-api-key" form, otherwise Basic Auth; IAM-enabled PCs
// require auth on the upgrade request. Custom headers (e.g. Cloudflare Access
// service tokens) are the same set the HTTP API path uses; without them, a
// service-token gateway in front of Prism Central rejects the upgrade.
func (s *stepVNCConnect) consoleHeaders() http.Header {
	cc := s.Config.ClusterConfig
	header := http.Header{}
	switch {
	case cc.APIKey != "":
		header.Set(ntnxAPIKeyHeaderName, cc.APIKey)
	case strings.EqualFold(cc.Username, ntnxAPIKeyHeaderName):
		header.Set(ntnxAPIKeyHeaderName, cc.Password)
	default:
		header.Set("Authorization", "Basic "+basicAuth(cc.Username, cc.Password))
	}
	for k, v := range cc.CustomHeaders {
		header.Set(k, v)
	}
	return header
}

func basicAuth(username, password string) string {
	auth := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(auth))
}

func (s *stepVNCConnect) Cleanup(state multistep.StateBag) {
	// No cleanup needed
}

// secretRedactor returns a function that replaces every secret the console
// handshake sends (the console token, the API key or password, the Basic
// credentials, and custom header values) with "<redacted>". Each is matched
// raw, URL-escaped (upper- or lower-case hex) and double-escaped: a gateway
// that redirects often echoes the original path and query in an escaped
// parameter, and a debug page may echo request headers.
//
// The token, API key and password have no minimum length on purpose: a floor
// would let a short real password through, and a mangled error message is the
// lesser cost.
func (s *stepVNCConnect) secretRedactor(consoleToken string) func(string) string {
	cc := s.Config.ClusterConfig
	secrets := []string{consoleToken, cc.APIKey, cc.Password}
	if cc.Username != "" || cc.Password != "" {
		secrets = append(secrets, basicAuth(cc.Username, cc.Password))
	}
	for _, v := range cc.CustomHeaders {
		// Short values ("true", "https") are not secrets and would mangle
		// unrelated text if replaced.
		if len(v) >= 6 {
			secrets = append(secrets, v)
		}
	}
	seen := map[string]bool{}
	var forms []string
	add := func(f string) {
		if f != "" && !seen[f] {
			seen[f] = true
			forms = append(forms, f)
		}
	}
	for _, v := range secrets {
		for _, f := range []string{v, url.QueryEscape(v), url.PathEscape(v), url.QueryEscape(url.QueryEscape(v))} {
			add(f)
			add(lowerPercentHex(f))
		}
	}
	// Longest first: strings.Replacer tries the pairs in order at each
	// position, so a secret containing another is replaced whole. One pass,
	// so "<redacted>" itself is never rescanned.
	sort.SliceStable(forms, func(i, j int) bool { return len(forms[i]) > len(forms[j]) })
	pairs := make([]string, 0, 2*len(forms))
	for _, f := range forms {
		pairs = append(pairs, f, "<redacted>")
	}
	replacer := strings.NewReplacer(pairs...)
	return replacer.Replace
}

// lowerPercentHex lower-cases the hex digits of every %XX escape in s.
func lowerPercentHex(s string) string {
	b := []byte(s)
	for i := 0; i+2 < len(b); i++ {
		if b[i] == '%' {
			for j := i + 1; j <= i+2; j++ {
				if b[j] >= 'A' && b[j] <= 'F' {
					b[j] += 'a' - 'A'
				}
			}
			i += 2
		}
	}
	return string(b)
}

// probeWebsocketHandshake sends an HTTP request mimicking a websocket upgrade to capture
// the server's response status and body. Used for debugging when the real websocket
// handshake fails (e.g. 401, 403, 302). The result is passed through redact
// before the body is cut to length, so a secret at the cut is not half-left.
func (s *stepVNCConnect) probeWebsocketHandshake(wsURL, origin string, redact func(string) string) (string, error) {
	// net/http only speaks http(s); the websocket URL is wss:// (or ws://).
	probeURL, err := url.Parse(wsURL)
	if err != nil {
		return "", err
	}
	switch probeURL.Scheme {
	case "wss":
		probeURL.Scheme = "https"
	case "ws":
		probeURL.Scheme = "http"
	}
	req, err := http.NewRequest("GET", probeURL.String(), nil)
	if err != nil {
		return "", err
	}
	// Send the same auth and custom headers as the real handshake, so the
	// probe shows Prism Central's response rather than a gateway's.
	req.Header = s.consoleHeaders()
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
		// Report a redirect rather than follow it: following would send the API
		// key and custom headers to the redirect target, and the real handshake
		// does not follow redirects either.
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	result := fmt.Sprintf("status=%d", resp.StatusCode)
	if loc := resp.Header.Get("Location"); loc != "" {
		// Where the redirect goes is the useful part. Its query and the body
		// (a link to the same place) are where gateways echo our URL, so leave
		// them out.
		if locURL, err := url.Parse(loc); err == nil {
			locURL.RawQuery, locURL.Fragment = "", ""
			loc = locURL.String()
		}
		return redact(result + " location=" + loc), nil
	}
	const maxBody = 4096
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	body := redact(string(raw))
	if len(body) > maxBody {
		body = body[:maxBody]
	}
	return result + " body=" + body, nil
}
