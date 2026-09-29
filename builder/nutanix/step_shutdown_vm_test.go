package nutanix

import (
	"bytes"
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
	vmmModels "github.com/nutanix/ntnx-api-golang-clients/vmm-go-client/v4/models/vmm/v4/ahv/config"
)

// cancelledGetVMDriver fails GetVM with the context's error, as the v4 client does
// once the build context is cancelled. The embedded Driver is nil: any other
// method called by the test would panic.
type cancelledGetVMDriver struct {
	Driver
	getVMCalls int
}

func (d *cancelledGetVMDriver) GetVM(ctx context.Context, vmUUID string) (*nutanixInstance, error) {
	d.getVMCalls++
	return nil, ctx.Err()
}

// TestStepShutdownCancelledWhileWaiting checks that a cancelled build stops
// waiting for shutdown promptly instead of dereferencing the nil VM GetVM
// returns with its error.
func TestStepShutdownCancelledWhileWaiting(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	d := &cancelledGetVMDriver{}
	state := new(multistep.BasicStateBag)
	state.Put("ui", &packer.BasicUi{Reader: new(bytes.Buffer), Writer: io.Discard, ErrorWriter: io.Discard})
	state.Put("driver", d)
	state.Put("communicator", new(packer.MockCommunicator))
	state.Put("config", &Config{})
	state.Put("vm_uuid", "vm-1")

	step := &StepShutdown{Timeout: time.Minute, DisableStopInstance: true}

	done := make(chan multistep.StepAction, 1)
	go func() { done <- step.Run(ctx, state) }()

	select {
	case action := <-done:
		if action != multistep.ActionHalt {
			t.Errorf("expected ActionHalt, got %v", action)
		}
		if _, ok := state.GetOk("error"); !ok {
			t.Error("expected an error in state")
		}
		if d.getVMCalls != 1 {
			t.Errorf("expected 1 GetVM call, got %d", d.getVMCalls)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("StepShutdown did not return promptly after the build was cancelled")
	}
}

// flakyGetVMDriver fails GetVM failures times with a live context, then
// reports the VM as off (or never, if failures is negative).
type flakyGetVMDriver struct {
	Driver
	failures int
	calls    int
}

func (d *flakyGetVMDriver) GetVM(ctx context.Context, vmUUID string) (*nutanixInstance, error) {
	d.calls++
	if d.failures < 0 || d.calls <= d.failures {
		return nil, errors.New("401 Unauthorized")
	}
	return &nutanixInstance{vm: &vmmModels.Vm{PowerState: vmmModels.POWERSTATE_OFF.Ref()}}, nil
}

func waitState(d Driver) *multistep.BasicStateBag {
	state := new(multistep.BasicStateBag)
	state.Put("ui", &packer.BasicUi{Reader: new(bytes.Buffer), Writer: io.Discard, ErrorWriter: io.Discard})
	state.Put("driver", d)
	state.Put("communicator", new(packer.MockCommunicator))
	state.Put("config", &Config{})
	state.Put("vm_uuid", "vm-1")
	return state
}

// A GetVM error with a live context is retried rather than ending the wait.
func TestStepShutdownRetriesTransientGetVMError(t *testing.T) {
	d := &flakyGetVMDriver{failures: 2}
	step := &StepShutdown{Timeout: time.Minute, DisableStopInstance: true, pollInterval: 10 * time.Millisecond}
	if action := step.Run(context.Background(), waitState(d)); action != multistep.ActionContinue {
		t.Errorf("action = %v, want ActionContinue", action)
	}
	if d.calls != 3 {
		t.Errorf("GetVM calls = %d, want 3", d.calls)
	}
}

// A persistent GetVM error is reported in the timeout error, not hidden.
func TestStepShutdownTimeoutReportsGetVMError(t *testing.T) {
	d := &flakyGetVMDriver{failures: -1}
	state := waitState(d)
	step := &StepShutdown{Timeout: 50 * time.Millisecond, DisableStopInstance: true, pollInterval: 10 * time.Millisecond}
	if action := step.Run(context.Background(), state); action != multistep.ActionHalt {
		t.Errorf("action = %v, want ActionHalt", action)
	}
	rawErr, ok := state.GetOk("error")
	if !ok || !strings.Contains(rawErr.(error).Error(), "last error getting VM power state: 401 Unauthorized") {
		t.Errorf("error = %v, want it to include the GetVM error", rawErr)
	}
}

// offAfterGetVMDriver reports the VM as on until offAfter has passed since the
// driver was created, then as off.
type offAfterGetVMDriver struct {
	Driver
	offAt time.Time
}

func (d *offAfterGetVMDriver) GetVM(ctx context.Context, vmUUID string) (*nutanixInstance, error) {
	state := vmmModels.POWERSTATE_ON
	if time.Now().After(d.offAt) {
		state = vmmModels.POWERSTATE_OFF
	}
	return &nutanixInstance{vm: &vmmModels.Vm{PowerState: state.Ref()}}, nil
}

// The VM's state is checked once more after the timeout expires, so a VM that
// powers off between the last poll and the timeout is not reported as a failure.
func TestStepShutdownChecksOnceMoreAfterTimeout(t *testing.T) {
	d := &offAfterGetVMDriver{offAt: time.Now().Add(80 * time.Millisecond)}
	step := &StepShutdown{Timeout: 100 * time.Millisecond, DisableStopInstance: true, pollInterval: 150 * time.Millisecond}
	if action := step.Run(context.Background(), waitState(d)); action != multistep.ActionContinue {
		t.Errorf("action = %v, want ActionContinue", action)
	}
}

// onceFailingGetVMDriver fails the first GetVM, then reports the VM as on.
type onceFailingGetVMDriver struct {
	Driver
	calls int
}

func (d *onceFailingGetVMDriver) GetVM(ctx context.Context, vmUUID string) (*nutanixInstance, error) {
	d.calls++
	if d.calls == 1 {
		return nil, errors.New("503 transient")
	}
	return &nutanixInstance{vm: &vmmModels.Vm{PowerState: vmmModels.POWERSTATE_ON.Ref()}}, nil
}

// An error from an earlier poll must not be reported as the cause of a timeout
// once later polls succeed.
func TestStepShutdownTimeoutIgnoresClearedGetVMError(t *testing.T) {
	d := &onceFailingGetVMDriver{}
	state := waitState(d)
	step := &StepShutdown{Timeout: 50 * time.Millisecond, DisableStopInstance: true, pollInterval: 10 * time.Millisecond}
	if action := step.Run(context.Background(), state); action != multistep.ActionHalt {
		t.Errorf("action = %v, want ActionHalt", action)
	}
	rawErr, ok := state.GetOk("error")
	if !ok {
		t.Fatal("expected an error in state")
	}
	if msg := rawErr.(error).Error(); strings.Contains(msg, "503 transient") {
		t.Errorf("timeout error reports a cleared GetVM error: %q", msg)
	}
	if d.calls < 2 {
		t.Errorf("GetVM calls = %d, want at least 2", d.calls)
	}
}
