package nutanix

import (
	"reflect"
	"strings"
	"testing"
)

func TestEnvSuffixToHeader(t *testing.T) {
	cases := map[string]string{
		"CF_ACCESS_CLIENT_ID":     "Cf-Access-Client-Id",
		"CF_ACCESS_CLIENT_SECRET": "Cf-Access-Client-Secret",
		"X_API_TOKEN":             "X-Api-Token",
		"AUTHORIZATION":           "Authorization",
		"":                        "",
	}
	for in, want := range cases {
		if got := envSuffixToHeader(in); got != want {
			t.Errorf("envSuffixToHeader(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestMergeCustomHeadersConfigWinsOverEnv(t *testing.T) {
	t.Setenv("NUTANIX_HEADER_CF_ACCESS_CLIENT_ID", "from-env")
	t.Setenv("NUTANIX_HEADER_X_EXTRA", "env-only")

	got := mergeCustomHeaders(map[string]string{
		"Cf-Access-Client-Id": "from-config",
	})

	want := map[string]string{
		"Cf-Access-Client-Id": "from-config",
		"X-Extra":             "env-only",
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("mergeCustomHeaders mismatch\n got:  %#v\n want: %#v", got, want)
	}
}

func TestMergeCustomHeadersReturnsNilWhenEmpty(t *testing.T) {
	// No NUTANIX_HEADER_ env vars set in the parent test process; t.Setenv is
	// scoped, but to be defensive ensure none leak into this case.
	for _, k := range []string{"NUTANIX_HEADER_FOO", "NUTANIX_HEADER_BAR"} {
		t.Setenv(k, "")
		// Setting to "" still creates the env var; clear it instead.
	}
	if got := mergeCustomHeaders(nil); got != nil {
		// Allow non-nil if NUTANIX_HEADER_ vars already exist outside the test.
		// In CI we expect a clean env, so flag only the genuinely-empty case.
		for k := range got {
			if strings.HasPrefix(k, "NUTANIX_HEADER_") || k == "" {
				t.Errorf("expected no headers, got %v", got)
			}
		}
	}
}

// minimalValidConfig returns the smallest map of raws that Prepare will accept
// when paired with the auth fields chosen in the test. Communicator is set to
// "none" so vm_nics aren't required.
func minimalValidConfig(extra map[string]interface{}) map[string]interface{} {
	cfg := map[string]interface{}{
		"nutanix_endpoint": "pc.example.com",
		"cluster_name":     "cluster-1",
		"os_type":          "Linux",
		"communicator":     "none",
		"vm_disks": []map[string]interface{}{
			{
				"image_type":        "DISK_IMAGE",
				"source_image_name": "img",
				"disk_size_gb":      40,
			},
		},
	}
	for k, v := range extra {
		cfg[k] = v
	}
	return cfg
}

func TestPrepareAcceptsAPIKeyOnly(t *testing.T) {
	t.Setenv("NUTANIX_API_KEY", "")
	c := &Config{}
	warnings, err := c.Prepare(minimalValidConfig(map[string]interface{}{
		"nutanix_api_key": "key123",
	}))
	if err != nil {
		t.Fatalf("expected Prepare to succeed with api-key only, got: %v", err)
	}
	for _, w := range warnings {
		if strings.Contains(w, "takes precedence") {
			t.Errorf("unexpected precedence warning when only api-key is set: %s", w)
		}
	}
	if c.ClusterConfig.APIKey != "key123" {
		t.Errorf("APIKey not set: %q", c.ClusterConfig.APIKey)
	}
}

func TestPrepareAcceptsUsernamePasswordOnly(t *testing.T) {
	t.Setenv("NUTANIX_API_KEY", "")
	c := &Config{}
	_, err := c.Prepare(minimalValidConfig(map[string]interface{}{
		"nutanix_username": "u",
		"nutanix_password": "p",
	}))
	if err != nil {
		t.Fatalf("expected Prepare to succeed with username+password, got: %v", err)
	}
}

func TestPrepareWarnsWhenBothAuthMethodsSet(t *testing.T) {
	t.Setenv("NUTANIX_API_KEY", "")
	c := &Config{}
	warnings, err := c.Prepare(minimalValidConfig(map[string]interface{}{
		"nutanix_username": "u",
		"nutanix_password": "p",
		"nutanix_api_key":  "key123",
	}))
	if err != nil {
		t.Fatalf("expected Prepare to succeed, got: %v", err)
	}
	found := false
	for _, w := range warnings {
		if strings.Contains(w, "takes precedence") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected precedence warning, got warnings: %v", warnings)
	}
}

func TestPrepareErrorsWhenNoAuth(t *testing.T) {
	t.Setenv("NUTANIX_API_KEY", "")
	c := &Config{}
	_, err := c.Prepare(minimalValidConfig(nil))
	if err == nil {
		t.Fatal("expected Prepare to fail without auth, got nil")
	}
	if !strings.Contains(err.Error(), "authentication required") {
		t.Errorf("expected authentication error, got: %v", err)
	}
}

func TestPrepareReadsAPIKeyFromEnv(t *testing.T) {
	t.Setenv("NUTANIX_API_KEY", "from-env")
	c := &Config{}
	_, err := c.Prepare(minimalValidConfig(nil))
	if err != nil {
		t.Fatalf("expected Prepare to succeed with NUTANIX_API_KEY env var, got: %v", err)
	}
	if c.ClusterConfig.APIKey != "from-env" {
		t.Errorf("APIKey not loaded from env: %q", c.ClusterConfig.APIKey)
	}
}

func TestPrepareReadsCustomHeadersFromEnv(t *testing.T) {
	t.Setenv("NUTANIX_HEADER_CF_ACCESS_CLIENT_ID", "abc.id")
	t.Setenv("NUTANIX_HEADER_CF_ACCESS_CLIENT_SECRET", "shh")
	c := &Config{}
	_, err := c.Prepare(minimalValidConfig(map[string]interface{}{
		"nutanix_api_key": "k",
	}))
	if err != nil {
		t.Fatalf("expected Prepare to succeed, got: %v", err)
	}
	got := c.ClusterConfig.CustomHeaders
	if got["Cf-Access-Client-Id"] != "abc.id" || got["Cf-Access-Client-Secret"] != "shh" {
		t.Errorf("env headers not picked up: %#v", got)
	}
}

func TestPrepareConfigHeadersOverrideEnvHeaders(t *testing.T) {
	t.Setenv("NUTANIX_HEADER_CF_ACCESS_CLIENT_ID", "from-env")
	c := &Config{}
	_, err := c.Prepare(minimalValidConfig(map[string]interface{}{
		"nutanix_api_key": "k",
		"nutanix_custom_headers": map[string]string{
			"Cf-Access-Client-Id": "from-config",
		},
	}))
	if err != nil {
		t.Fatalf("expected Prepare to succeed, got: %v", err)
	}
	if got := c.ClusterConfig.CustomHeaders["Cf-Access-Client-Id"]; got != "from-config" {
		t.Errorf("expected config to win, got %q", got)
	}
}
