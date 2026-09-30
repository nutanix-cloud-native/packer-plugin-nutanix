package nutanix

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	convergedv4 "github.com/nutanix-cloud-native/prism-go-client/converged/v4"
)

func TestV4CacheParamsManagementEndpointBasicAuth(t *testing.T) {
	p := &v4CacheParams{
		endpoint: "pc.example.com",
		port:     9440,
		username: "admin",
		password: "secret",
	}
	ep := p.ManagementEndpoint()
	if ep.Username != "admin" || ep.Password != "secret" {
		t.Errorf("basic auth credentials not preserved: %+v", ep.ApiCredentials)
	}
}

func TestV4CacheParamsManagementEndpointAPIKey(t *testing.T) {
	p := &v4CacheParams{
		endpoint: "pc.example.com",
		port:     9440,
		username: "admin",
		password: "secret",
		apiKey:   "key123",
	}
	ep := p.ManagementEndpoint()
	if ep.APIKey != "key123" {
		t.Errorf("expected APIKey=key123, got %q", ep.APIKey)
	}
	// Username/password must be dropped: the vmm SDK sends Basic auth from any
	// client that holds them, so they would travel on every request.
	if ep.Username != "" || ep.Password != "" {
		t.Errorf("expected no basic credentials alongside api key, got %+v", ep.ApiCredentials)
	}
}

func TestV4CacheParamsManagementEndpointObjectsUpload(t *testing.T) {
	p := &v4CacheParams{
		endpoint:      "pc.example.com",
		port:          9440,
		username:      "admin",
		password:      "secret",
		apiKey:        "key123",
		objectsUpload: true,
	}
	ep := p.ManagementEndpoint()
	// The Objects Lite upload signs its S3 requests with username/password, and
	// the image create after it runs on the same client, so it uses those alone.
	if ep.APIKey != "" || ep.Username != "admin" || ep.Password != "secret" {
		t.Errorf("expected basic credentials only for upload client, got %+v", ep.ApiCredentials)
	}
	mainParams := *p
	mainParams.objectsUpload = false
	if p.Key() == mainParams.Key() {
		t.Error("expected different cache key for the upload client")
	}
}

// TestV4TransferClientKeepsReadTimeout guards against the cache handing the
// transfer caller a client created without the transfer read timeout: a cache
// hit ignores client options, so creation order must not matter.
func TestV4TransferClientKeepsReadTimeout(t *testing.T) {
	d := &NutanixDriver{ClusterConfig: ClusterConfig{
		Endpoint:        "timeout-test.example.com",
		Port:            9440,
		Username:        "admin",
		Password:        "secret",
		APIKey:          "timeout-test-key",
		TransferTimeout: 45,
	}}

	// Create the main client first, as a build does before exporting.
	if _, err := d.getV4Client(); err != nil {
		t.Fatal(err)
	}
	if _, err := d.getV4TransferClient(); err != nil {
		t.Fatal(err)
	}

	params := &v4CacheParams{
		endpoint: d.ClusterConfig.Endpoint,
		port:     d.ClusterConfig.Port,
		username: d.ClusterConfig.Username,
		password: d.ClusterConfig.Password,
		apiKey:   d.ClusterConfig.APIKey,
		transfer: true,
	}
	c, err := v4SDKClientCache.GetOrCreate(params)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := c.ImagesApiInstance.ApiClient.ReadTimeout, 45*time.Minute; got != want {
		t.Errorf("transfer client ReadTimeout = %v, want %v", got, want)
	}
}

// TestV4ClientAuthHeadersOnWire checks what actually reaches Prism Central:
// with an API key set, the main client must send the key and no Basic auth,
// while the upload client sends Basic only, for the Objects Lite upload.
func TestV4ClientAuthHeadersOnWire(t *testing.T) {
	var mu sync.Mutex
	var seen []http.Header
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen = append(seen, r.Header.Clone())
		mu.Unlock()
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		t.Fatal(err)
	}
	d := &NutanixDriver{ClusterConfig: ClusterConfig{
		Endpoint: u.Hostname(),
		Port:     int32(port),
		Username: "admin",
		Password: "secret",
		APIKey:   "wire-test-key",
		Insecure: true,
		CustomHeaders: map[string]string{
			"Cf-Access-Client-Id": "wire-test-client",
		},
	}}

	request := func(t *testing.T, get func() (*convergedv4.Client, error), call func(*convergedv4.Client)) []http.Header {
		t.Helper()
		mu.Lock()
		seen = nil
		mu.Unlock()
		c, err := get()
		if err != nil {
			t.Fatal(err)
		}
		call(c)
		mu.Lock()
		defer mu.Unlock()
		if len(seen) == 0 {
			t.Fatal("no request reached the test server")
		}
		return seen
	}
	listImages := func(c *convergedv4.Client) { _, _ = c.Images.List(context.Background()) }

	// One call per SDK ApiClient the plugin uses, so a header missing from any
	// of them fails here.
	ctx := context.Background()
	mainCalls := map[string]func(*convergedv4.Client){
		"vmm":                   listImages,
		"networking":            func(c *convergedv4.Client) { _, _ = c.Subnets.List(ctx) },
		"clustermgmt":           func(c *convergedv4.Client) { _, _ = c.Clusters.List(ctx) },
		"clustermgmt (storage)": func(c *convergedv4.Client) { _, _ = c.StorageContainers.List(ctx) },
		"prism (tasks)":         func(c *convergedv4.Client) { _, _ = c.Tasks.Get(ctx, "task-1") },
		"volumes":               func(c *convergedv4.Client) { _, _ = c.VolumeGroups.List(ctx) },
		"iam":                   func(c *convergedv4.Client) { _, _ = c.Users.List(ctx) },
	}
	for name, call := range mainCalls {
		t.Run("main client "+name, func(t *testing.T) {
			for _, h := range request(t, d.getV4Client, call) {
				if h.Get("X-ntnx-api-key") != "wire-test-key" {
					t.Errorf("expected X-ntnx-api-key, got headers %v", h)
				}
				if a := h.Get("Authorization"); a != "" {
					t.Errorf("expected no Authorization header with an api key, got %q", a)
				}
				if got := h.Get("Cf-Access-Client-Id"); got != "wire-test-client" {
					t.Errorf("expected custom header, got %q", got)
				}
			}
		})
	}

	var sawBasic bool
	for _, h := range request(t, d.getV4UploadClient, listImages) {
		if strings.HasPrefix(h.Get("Authorization"), "Basic ") {
			sawBasic = true
		}
		if k := h.Get("X-ntnx-api-key"); k != "" {
			t.Errorf("upload client: expected Basic auth only, also got X-ntnx-api-key %q", k)
		}
		if got := h.Get("Cf-Access-Client-Id"); got != "wire-test-client" {
			t.Errorf("upload client: expected custom header, got %q", got)
		}
	}
	if !sawBasic {
		t.Error("upload client: expected Basic credentials for the Objects Lite upload")
	}
}

// TestV4ClientCustomHeadersNoRaceOnCacheHit gets the cached client repeatedly
// while another goroutine makes requests with it. Writing the SDK's default
// headers on every cache hit made the Go runtime abort with "concurrent map
// read and map write".
func TestV4ClientCustomHeadersNoRaceOnCacheHit(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	u, err := url.Parse(srv.URL)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		t.Fatal(err)
	}
	d := &NutanixDriver{ClusterConfig: ClusterConfig{
		Endpoint:      u.Hostname(),
		Port:          int32(port),
		APIKey:        "race-test-key",
		Insecure:      true,
		CustomHeaders: map[string]string{"Cf-Access-Client-Id": "race-test-client"},
	}}

	c, err := d.getV4Client()
	if err != nil {
		t.Fatal(err)
	}

	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
				_, _ = c.Images.List(context.Background())
			}
		}
	}()

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		if _, err := d.getV4Client(); err != nil {
			t.Fatal(err)
		}
	}
	close(stop)
	wg.Wait()
}

func TestV4CacheParamsKeyDifferentiates(t *testing.T) {
	base := &v4CacheParams{endpoint: "pc.example.com", port: 9440, username: "u", password: "p"}
	withAPIKey := *base
	withAPIKey.apiKey = "k"
	withHeaders := *base
	withHeaders.customHeaders = map[string]string{"X-Foo": "bar"}

	// Credentials are not hashed into the key; the cache's validation hash of
	// ManagementEndpoint replaces the client when they change.
	if base.ManagementEndpoint().APIKey == withAPIKey.ManagementEndpoint().APIKey {
		t.Error("expected the api key to reach ManagementEndpoint")
	}
	if base.Key() == withHeaders.Key() {
		t.Error("expected different cache key when custom headers are set")
	}
	// Sanity: same params produce the same key.
	other := *base
	if base.Key() != other.Key() {
		t.Error("expected stable cache key for identical params")
	}
}

// TestV4ClientCacheSeparatesCredentials guards that credentials, which are not
// part of Key(), still never share a cached client: the cache's validation hash
// of ManagementEndpoint must replace the client when they change.
func TestV4ClientCacheSeparatesCredentials(t *testing.T) {
	a := &v4CacheParams{endpoint: "cred-test.example.com", port: 9440, username: "user-a", password: "p"}
	b := &v4CacheParams{endpoint: "cred-test.example.com", port: 9440, username: "user-b", password: "p"}
	if a.Key() != b.Key() {
		t.Fatal("test expects the two params to share a cache key")
	}

	for i, p := range []*v4CacheParams{a, b, a} {
		c, err := v4SDKClientCache.GetOrCreate(p)
		if err != nil {
			t.Fatal(err)
		}
		if got := c.VmApiInstance.ApiClient.Username; got != p.username {
			t.Errorf("call %d: cached client has username %q, want %q", i, got, p.username)
		}
	}
}
