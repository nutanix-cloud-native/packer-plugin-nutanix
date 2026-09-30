package nutanix

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
)

func noRedact(s string) string { return s }

func TestConsoleHeaders(t *testing.T) {
	cases := []struct {
		name       string
		cc         ClusterConfig
		wantAPIKey string
		wantBasic  bool
	}{
		{"api key", ClusterConfig{APIKey: "k", Username: "u", Password: "p"}, "k", false},
		{"legacy api key username", ClusterConfig{Username: "X-ntnx-api-key", Password: "legacy-key"}, "legacy-key", false},
		// The REST clients match the legacy username case-insensitively.
		{"legacy api key username, lower case", ClusterConfig{Username: "x-ntnx-api-key", Password: "legacy-key"}, "legacy-key", false},
		{"basic auth", ClusterConfig{Username: "u", Password: "p"}, "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tc.cc.CustomHeaders = map[string]string{"Cf-Access-Client-Id": "cf-id"}
			s := &stepVNCConnect{Config: &Config{ClusterConfig: tc.cc}}
			h := s.consoleHeaders()

			if got := h.Get("X-ntnx-api-key"); got != tc.wantAPIKey {
				t.Errorf("X-ntnx-api-key = %q, want %q", got, tc.wantAPIKey)
			}
			if got := h.Get("Authorization"); (got != "") != tc.wantBasic {
				t.Errorf("Authorization = %q, want basic=%v", got, tc.wantBasic)
			}
			if got := h.Get("Cf-Access-Client-Id"); got != "cf-id" {
				t.Errorf("custom header = %q, want cf-id", got)
			}
		})
	}
}

// The probe is given the websocket URL (wss://) and must still reach the
// server, with the same headers as the real handshake, and report its answer.
func TestProbeWebsocketHandshake(t *testing.T) {
	var gotHeader, gotKey string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHeader = r.Header.Get("Cf-Access-Client-Id")
		gotKey = r.Header.Get("X-ntnx-api-key")
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("denied"))
	}))
	defer srv.Close()

	s := &stepVNCConnect{Config: &Config{ClusterConfig: ClusterConfig{
		APIKey:        "k",
		Insecure:      true,
		CustomHeaders: map[string]string{"Cf-Access-Client-Id": "cf-id"},
	}}}
	wsURL := "wss://" + strings.TrimPrefix(srv.URL, "https://") + "/vnc/vm/x/proxy"
	got, err := s.probeWebsocketHandshake(wsURL, srv.URL, noRedact)
	if err != nil {
		t.Fatalf("probe failed: %v", err)
	}
	if !strings.Contains(got, "status=403") || !strings.Contains(got, "denied") {
		t.Errorf("probe result = %q, want the server's 403 and body", got)
	}
	if gotHeader != "cf-id" {
		t.Errorf("probe sent Cf-Access-Client-Id %q, want cf-id", gotHeader)
	}
	if gotKey != "k" {
		t.Errorf("probe sent X-ntnx-api-key %q, want k", gotKey)
	}
}

// A redirect (e.g. a gateway sending the probe to its login page) is reported,
// not followed, so the API key and custom headers never reach the target.
func TestProbeWebsocketHandshakeDoesNotFollowRedirect(t *testing.T) {
	var targetHit bool
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		targetHit = true
	}))
	defer target.Close()
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL+"/login", http.StatusFound)
	}))
	defer srv.Close()

	s := &stepVNCConnect{Config: &Config{ClusterConfig: ClusterConfig{
		APIKey:        "k",
		Insecure:      true,
		CustomHeaders: map[string]string{"Cf-Access-Client-Secret": "secret"},
	}}}
	wsURL := "wss://" + strings.TrimPrefix(srv.URL, "https://") + "/vnc/vm/x/proxy"
	got, err := s.probeWebsocketHandshake(wsURL, srv.URL, noRedact)
	if err != nil {
		t.Fatalf("probe failed: %v", err)
	}
	if !strings.Contains(got, "status=302") || !strings.Contains(got, "location="+target.URL+"/login") {
		t.Errorf("probe result = %q, want the 302 and its location", got)
	}
	if targetHit {
		t.Error("probe followed the redirect and sent its headers to the target")
	}
}

// A login redirect that echoes our path and query (console token included) in
// its own query is reported without that query or the body that repeats it.
func TestProbeWebsocketHandshakeOmitsEchoedQuery(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		login := "https://team.example.com/cdn-cgi/access/login/pc?redirect_url=" + url.QueryEscape(r.URL.RequestURI())
		http.Redirect(w, r, login, http.StatusFound)
	}))
	defer srv.Close()

	s := &stepVNCConnect{Config: &Config{ClusterConfig: ClusterConfig{APIKey: "k", Insecure: true}}}
	wsURL := "wss://" + strings.TrimPrefix(srv.URL, "https://") + "/vnc/vm/x/proxy?VmConsoleToken=TOK123"
	got, err := s.probeWebsocketHandshake(wsURL, srv.URL, noRedact)
	if err != nil {
		t.Fatalf("probe failed: %v", err)
	}
	if want := "status=302 location=https://team.example.com/cdn-cgi/access/login/pc"; got != want {
		t.Errorf("probe result = %q, want %q", got, want)
	}
}

// A redirect that echoes our URL in its path (not its query) is still redacted.
func TestProbeWebsocketHandshakeRedactsRedirectPath(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "https://gw.example.com/login/"+url.PathEscape(r.URL.RequestURI()), http.StatusFound)
	}))
	defer srv.Close()

	s := &stepVNCConnect{Config: &Config{ClusterConfig: ClusterConfig{APIKey: "k", Insecure: true}}}
	wsURL := "wss://" + strings.TrimPrefix(srv.URL, "https://") + "/vnc/vm/x/proxy?VmConsoleToken=TOK123456"
	got, err := s.probeWebsocketHandshake(wsURL, srv.URL, s.secretRedactor("TOK123456"))
	if err != nil {
		t.Fatalf("probe failed: %v", err)
	}
	if strings.Contains(got, "TOK123456") || !strings.Contains(got, "status=302") {
		t.Errorf("probe result = %q, want the 302 with the token redacted", got)
	}
}

func TestSecretRedactor(t *testing.T) {
	s := &stepVNCConnect{Config: &Config{ClusterConfig: ClusterConfig{
		APIKey:        "api-key-123",
		CustomHeaders: map[string]string{"Cf-Access-Client-Secret": "cf secret/value+1", "X-Short": "true"},
	}}}
	token := "tok en/with+chars"
	redact := s.secretRedactor(token)
	for _, secret := range []string{token, "api-key-123", "cf secret/value+1"} {
		for _, form := range []string{secret, url.QueryEscape(secret), url.PathEscape(secret), url.QueryEscape(url.QueryEscape(secret))} {
			if got := redact("before " + form + " after"); got != "before <redacted> after" {
				t.Errorf("redact(%q) = %q", form, got)
			}
		}
	}
	// A short, non-secret header value is left alone rather than mangling text.
	if got := redact("true story"); got != "true story" {
		t.Errorf("short value redacted: %q", got)
	}
	// Lower-case percent escapes, as some encoders write them.
	if got := redact("x " + strings.ToLower(url.QueryEscape(token)) + " y"); got != "x <redacted> y" {
		t.Errorf("lower-case escape not redacted: %q", got)
	}
}

func TestSecretRedactorBasicAndOverlap(t *testing.T) {
	s := &stepVNCConnect{Config: &Config{ClusterConfig: ClusterConfig{
		Username: "admin",
		Password: "red",
		// A header value that starts with the console token (checked after it).
		CustomHeaders: map[string]string{"X-Token-Longer": "token-abcdef"},
	}}}
	redact := s.secretRedactor("token-abc")
	// A debug page echoing the Basic header shows base64(user:pass).
	if got := redact("Authorization: Basic " + basicAuth("admin", "red")); got != "Authorization: Basic <redacted>" {
		t.Errorf("basic credentials not redacted: %q", got)
	}
	// The longer secret is replaced whole, not left with a tail.
	if got := redact("t=token-abcdef;"); got != "t=<redacted>;" {
		t.Errorf("overlapping secrets: %q", got)
	}
	// A short password that also appears in "<redacted>" does not compound.
	if got := redact("a red car"); got != "a <redacted> car" {
		t.Errorf("redaction compounded: %q", got)
	}
}

// A secret straddling the body limit is redacted before the body is cut.
func TestProbeWebsocketHandshakeRedactsBeforeTruncating(t *testing.T) {
	const secret = "0123456789abcdef"
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(strings.Repeat("x", 4090) + secret))
	}))
	defer srv.Close()

	s := &stepVNCConnect{Config: &Config{ClusterConfig: ClusterConfig{APIKey: secret, Insecure: true}}}
	wsURL := "wss://" + strings.TrimPrefix(srv.URL, "https://") + "/vnc/vm/x/proxy"
	got, err := s.probeWebsocketHandshake(wsURL, srv.URL, s.secretRedactor(""))
	if err != nil {
		t.Fatalf("probe failed: %v", err)
	}
	if strings.Contains(got, "012345") {
		t.Errorf("part of the secret survived the cut: ...%q", got[len(got)-20:])
	}
}

// consoleTokenDriver hands out a fixed console token; any other call panics.
type consoleTokenDriver struct{ Driver }

func (consoleTokenDriver) GenerateConsoleToken(context.Context, string) (string, string, error) {
	return "SECRET-CONSOLE-TOKEN", "/vnc/vm/x/proxy", nil
}

// A failed handshake's error (which wraps the websocket library's dial error,
// full URL included) and probe result must not carry the token or the API key.
func TestConnectVNCErrorRedactsSecrets(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte("forbidden: " + r.URL.RequestURI() + " key=" + r.Header.Get("X-ntnx-api-key")))
	}))
	defer srv.Close()
	u, _ := url.Parse(srv.URL)
	port, _ := strconv.Atoi(u.Port())

	s := &stepVNCConnect{Config: &Config{ClusterConfig: ClusterConfig{
		Endpoint: u.Hostname(), Port: int32(port), APIKey: "SECRET-API-KEY", Insecure: true,
	}}}
	state := new(multistep.BasicStateBag)
	state.Put("vm_uuid", "vm-1")
	state.Put("driver", consoleTokenDriver{})
	_, err := s.ConnectVNCOverWebsocketClient(context.Background(), state)
	if err == nil {
		t.Fatal("expected the handshake to fail")
	}
	msg := err.Error()
	for _, secret := range []string{"SECRET-CONSOLE-TOKEN", "SECRET-API-KEY"} {
		if strings.Contains(msg, secret) {
			t.Errorf("error leaks %s: %q", secret, msg)
		}
	}
	if !strings.Contains(msg, "status=403") {
		t.Errorf("error = %q, want the probe's 403", msg)
	}
}
