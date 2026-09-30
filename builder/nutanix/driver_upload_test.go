package nutanix

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// TestCreateImageFileObjectsLiteRequest checks the Objects Lite S3 upload on
// the wire: it carries the custom headers, and it is signed with the real
// username/password even when the build uses an API key.
func TestCreateImageFileObjectsLiteRequest(t *testing.T) {
	var mu sync.Mutex
	var put http.Header
	var putPath string
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPut && strings.HasPrefix(r.URL.Path, "/api/prism/v4.0/objects/") {
			mu.Lock()
			put = r.Header.Clone()
			putPath = r.URL.Path
			mu.Unlock()
			w.WriteHeader(http.StatusOK)
			return
		}
		// Fail the image create that follows the upload; only the upload
		// request is under test.
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
		Username:      "admin",
		Password:      "secret",
		APIKey:        "upload-test-key",
		Insecure:      true,
		CustomHeaders: map[string]string{"Cf-Access-Client-Id": "upload-test-client"},
	}}

	file := filepath.Join(t.TempDir(), "upload-test.iso")
	if err := os.WriteFile(file, []byte("image"), 0o600); err != nil {
		t.Fatal(err)
	}

	_, _ = d.CreateImageFile(context.Background(), file, VmConfig{})

	mu.Lock()
	defer mu.Unlock()
	if put == nil {
		t.Fatal("no Objects Lite upload reached the test server")
	}
	if putPath != "/api/prism/v4.0/objects/vmm-images/upload-test.iso" {
		t.Errorf("upload path = %q", putPath)
	}
	if got := put.Get("Cf-Access-Client-Id"); got != "upload-test-client" {
		t.Errorf("custom header on upload = %q, want upload-test-client", got)
	}
	wantCred := "Credential=" + base64.StdEncoding.EncodeToString([]byte("admin:secret")) + "/"
	if got := put.Get("Authorization"); !strings.Contains(got, wantCred) {
		t.Errorf("upload not signed with username/password: Authorization = %q", got)
	}
}
