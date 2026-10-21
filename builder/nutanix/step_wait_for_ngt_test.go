package nutanix

import (
	"context"
	"testing"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
)

type ngtTestDriver struct {
	Driver
	status ngtStatus
}

func (d *ngtTestDriver) GetNGTStatus(context.Context, string) (ngtStatus, error) {
	return d.status, nil
}

func TestWaitForNGT(t *testing.T) {
	status := ngtStatus{Installed: true, Reachable: true, Enabled: true}
	driver := &ngtTestDriver{status: status}

	got, err := waitForNGT(context.Background(), driver, "vm-uuid", 1)
	if err != nil {
		t.Fatalf("waitForNGT returned an error: %v", err)
	}
	if got != status {
		t.Fatalf("status = %+v, want %+v", got, status)
	}
}

func TestRestartNGT(t *testing.T) {
	communicator := &packer.MockCommunicator{}
	state := new(multistep.BasicStateBag)
	state.Put("communicator", communicator)

	if err := restartNGT(context.Background(), state, "Restart-Service NutanixGuestTools"); err != nil {
		t.Fatalf("restartNGT returned an error: %v", err)
	}
	if !communicator.StartCalled {
		t.Fatal("restart command was not started")
	}
	if communicator.StartCmd.Command != "Restart-Service NutanixGuestTools" {
		t.Fatalf("command = %q, want %q", communicator.StartCmd.Command, "Restart-Service NutanixGuestTools")
	}
}
