package nutanix

import (
	"strings"
	"testing"
)

func minimalValidConfig(extra map[string]interface{}) map[string]interface{} {
	cfg := map[string]interface{}{
		"nutanix_endpoint": "pc.example.com",
		"nutanix_username": "admin",
		"nutanix_password": "password",
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

func TestPrepareRejectsInvalidWindowsInstallType(t *testing.T) {
	c := &Config{}
	_, err := c.Prepare(minimalValidConfig(map[string]interface{}{
		"windows_install_type": "fresh",
	}))
	if err != nil {
		t.Fatalf("expected case-insensitive match to succeed, got: %v", err)
	}

	c2 := &Config{}
	_, err = c2.Prepare(minimalValidConfig(map[string]interface{}{
		"windows_install_type": "INVALID",
	}))
	if err == nil {
		t.Fatal("expected Prepare to fail with invalid windows_install_type")
	}
	if !strings.Contains(err.Error(), "windows_install_type must be FRESH or PREPARED") {
		t.Errorf("unexpected error: %v", err)
	}
}
