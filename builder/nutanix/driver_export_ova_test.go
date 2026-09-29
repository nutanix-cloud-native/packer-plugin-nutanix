package nutanix

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"testing"
	"time"
)

// TestExportOVAWaitHonoursCancellation checks that ExportOVA's wait for the OVA to
// appear stops promptly once the build context is cancelled, instead of spending
// its remaining retries on calls that can only fail.
func TestExportOVAWaitHonoursCancellation(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"$objectType":"vmm.v4.content.ListOvasApiResponse","data":[],"metadata":{"totalAvailableResults":0}}`)
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
		Insecure: true,
	}}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	done := make(chan error, 1)
	start := time.Now()
	go func() {
		_, err := d.ExportOVA(ctx, "never-appears")
		done <- err
	}()

	select {
	case err := <-done:
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Errorf("err = %v, want context.DeadlineExceeded", err)
		}
		if elapsed := time.Since(start); elapsed > 3*time.Second {
			t.Errorf("ExportOVA took %s to return after cancellation", elapsed)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("ExportOVA did not return after its context was cancelled")
	}
}
