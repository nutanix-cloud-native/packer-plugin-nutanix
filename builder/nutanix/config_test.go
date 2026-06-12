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
		if strings.Contains(w, "takes precedence") {
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
	c := &Config{}
	_, err := c.Prepare(minimalValidConfig(nil))
	if err == nil {
		t.Fatal("expected Prepare to fail without auth, got nil")
	}
	if !strings.Contains(err.Error(), "authentication required") {
		t.Errorf("expected authentication error, got: %v", err)
	}
}
