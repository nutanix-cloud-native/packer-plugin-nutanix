package nutanix

import "testing"

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
		username: "admin",  // ignored when apiKey is set
		password: "secret", // ignored when apiKey is set
		apiKey:   "key123",
	}
	ep := p.ManagementEndpoint()
	if ep.Username != ntnxAPIKeyHeader {
		t.Errorf("expected Username=%q for api-key auth, got %q", ntnxAPIKeyHeader, ep.Username)
	}
	if ep.Password != "key123" {
		t.Errorf("expected Password=api-key, got %q", ep.Password)
	}
}

func TestV4CacheParamsKeyDifferentiates(t *testing.T) {
	base := &v4CacheParams{endpoint: "pc.example.com", port: 9440, username: "u", password: "p"}
	withAPIKey := *base
	withAPIKey.apiKey = "k"
	withHeaders := *base
	withHeaders.customHeaders = map[string]string{"X-Foo": "bar"}

	if base.Key() == withAPIKey.Key() {
		t.Error("expected different cache key when apiKey is set")
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
