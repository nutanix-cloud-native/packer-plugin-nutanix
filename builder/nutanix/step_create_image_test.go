package nutanix

import (
	"bytes"
	"context"
	"io"
	"testing"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
)

// TestStepCreateImageRunGetVMError checks that a failed GetVM (as happens once
// the build is cancelled) halts the step instead of dereferencing a nil VM.
func TestStepCreateImageRunGetVMError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	state := new(multistep.BasicStateBag)
	state.Put("ui", &packer.BasicUi{Reader: new(bytes.Buffer), Writer: io.Discard, ErrorWriter: io.Discard})
	state.Put("driver", &cancelledGetVMDriver{})
	state.Put("vm_uuid", "vm-1")

	step := &stepCreateImage{Config: &Config{}}
	if action := step.Run(ctx, state); action != multistep.ActionHalt {
		t.Errorf("expected ActionHalt, got %v", action)
	}
	if _, ok := state.GetOk("error"); !ok {
		t.Error("expected an error in state")
	}
}
