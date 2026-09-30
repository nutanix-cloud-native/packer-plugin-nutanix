package nutanix

import (
	"strings"
	"testing"
)

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
	c := &Config{}
	warnings, err := c.Prepare(minimalValidConfig(map[string]interface{}{
		"nutanix_api_key": "key123",
	}))
	if err != nil {
		t.Fatalf("expected Prepare to succeed with api-key only, got: %v", err)
	}
	for _, w := range warnings {
		if strings.Contains(w, "is used for API calls") {
			t.Errorf("unexpected precedence warning when only api-key is set: %s", w)
		}
	}
	if c.ClusterConfig.APIKey != "key123" {
		t.Errorf("APIKey not set: %q", c.ClusterConfig.APIKey)
	}
}

func TestPrepareAcceptsUsernamePasswordOnly(t *testing.T) {
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
		if strings.Contains(w, "is used for API calls") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected precedence warning, got warnings: %v", warnings)
	}
}

// Uploads need both credential sets, so that configuration gets no warning.
func TestPrepareNoPrecedenceWarningWhenUploading(t *testing.T) {
	c := &Config{}
	warnings, err := c.Prepare(minimalValidConfig(map[string]interface{}{
		"nutanix_username": "u",
		"nutanix_password": "p",
		"nutanix_api_key":  "key123",
		"cd_content":       map[string]string{"a.txt": "x"},
	}))
	if err != nil {
		t.Fatalf("expected Prepare to succeed, got: %v", err)
	}
	for _, w := range warnings {
		if strings.Contains(w, "is used for API calls") {
			t.Errorf("unexpected precedence warning when uploading: %s", w)
		}
	}
}

// Objects Lite signs uploads with username/password, so an API-key-only
// config that uploads images must fail at Prepare rather than mid-build.
func TestPrepareObjectsLiteUploadNeedsBasicAuth(t *testing.T) {
	cases := []struct {
		name    string
		extra   map[string]interface{}
		wantErr bool
	}{
		{"api key only, no upload", map[string]interface{}{"nutanix_api_key": "k"}, false},
		{"api key only, cd_content", map[string]interface{}{
			"nutanix_api_key": "k",
			"cd_content":      map[string]string{"a.txt": "x"},
		}, true},
		{"api key plus basic, cd_content", map[string]interface{}{
			"nutanix_api_key":  "k",
			"nutanix_username": "u",
			"nutanix_password": "p",
			"cd_content":       map[string]string{"a.txt": "x"},
		}, false},
		{"basic only, cd_content", map[string]interface{}{
			"nutanix_username": "u",
			"nutanix_password": "p",
			"cd_content":       map[string]string{"a.txt": "x"},
		}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := &Config{}
			_, err := c.Prepare(minimalValidConfig(tc.extra))
			if tc.wantErr {
				if err == nil || !strings.Contains(err.Error(), "Objects Lite signs uploads") {
					t.Errorf("expected Objects Lite credentials error, got: %v", err)
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestPrepareErrorsWhenNoAuth(t *testing.T) {
	c := &Config{}
	_, err := c.Prepare(minimalValidConfig(nil))
	if err == nil {
		t.Fatal("expected Prepare to fail without auth, got nil")
	}
	if !strings.Contains(err.Error(), "authentication required") {
		t.Errorf("expected authentication error, got: %v", err)
	}
}
