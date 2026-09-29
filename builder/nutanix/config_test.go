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

func TestPrepareWindowsInstallType(t *testing.T) {
	windows := map[string]interface{}{"os_type": "Windows", "user_data": "PHVuYXR0ZW5kLz4="}
	cases := []struct {
		name         string
		value        string
		extra        map[string]interface{}
		wantErr      bool
		wantNoEffect bool
	}{
		{"unset", "", windows, false, false},
		{"PREPARED", "PREPARED", windows, false, false},
		{"lowercase prepared", "prepared", windows, false, false},
		{"FRESH", "FRESH", windows, false, false},
		{"lowercase fresh", "fresh", windows, false, false},
		{"invalid", "bogus", windows, true, false},
		{"linux os_type", "FRESH", map[string]interface{}{}, false, true},
		{"windows without user_data", "FRESH", map[string]interface{}{"os_type": "Windows"}, false, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			extra := map[string]interface{}{}
			for k, v := range tc.extra {
				extra[k] = v
			}
			if tc.value != "" {
				extra["windows_install_type"] = tc.value
			}
			// minimalValidConfig carries no credentials.
			extra["nutanix_username"] = "admin"
			extra["nutanix_password"] = "password"

			c := &Config{}
			warnings, err := c.Prepare(minimalValidConfig(extra))
			if tc.wantErr {
				if err == nil || !strings.Contains(err.Error(), "windows_install_type must be FRESH or PREPARED") {
					t.Errorf("expected windows_install_type error, got: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			noEffect := false
			for _, w := range warnings {
				if strings.Contains(w, "windows_install_type has no effect") {
					noEffect = true
				}
			}
			if noEffect != tc.wantNoEffect {
				t.Errorf("no-effect warning = %v, want %v (warnings: %v)", noEffect, tc.wantNoEffect, warnings)
			}
		})
	}
}
