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

// TestWaitForIPHonoursCancellation checks that WaitForIP returns promptly once
// its context is cancelled, for a VM that never reports an IP, rather than
// polling until one appears.
func TestWaitForIPHonoursCancellation(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"$objectType":"vmm.v4.ahv.config.GetVmApiResponse","data":{"$objectType":"vmm.v4.ahv.config.Vm","extId":"vm-1","nics":[]}}`)
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

	type result struct {
		ip  string
		err error
	}
	done := make(chan result, 1)
	start := time.Now()
	go func() {
		ip, err := d.WaitForIP(ctx, "vm-1", nil)
		done <- result{ip, err}
	}()

	select {
	case r := <-done:
		if !errors.Is(r.err, context.DeadlineExceeded) {
			t.Errorf("err = %v, want context.DeadlineExceeded", r.err)
		}
		if r.ip != "" {
			t.Errorf("ip = %q, want none", r.ip)
		}
		if elapsed := time.Since(start); elapsed > 3*time.Second {
			t.Errorf("WaitForIP took %s to return after cancellation", elapsed)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("WaitForIP did not return after its context was cancelled")
	}
}
