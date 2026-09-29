package nutanix

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
)

// cleanupCall is the state of the context a delete call received, captured
// during the call: each operation's context is cancelled once it returns.
type cleanupCall struct {
	err         error
	hasDeadline bool
}

// cleanupDriver records each delete call's context state. The embedded Driver
// is nil: any other method called by the test would panic.
type cleanupDriver struct {
	Driver
	calls []cleanupCall
}

func (d *cleanupDriver) record(ctx context.Context) {
	_, ok := ctx.Deadline()
	d.calls = append(d.calls, cleanupCall{err: ctx.Err(), hasDeadline: ok})
}

func (d *cleanupDriver) DeleteImage(ctx context.Context, imageUUID string) error {
	d.record(ctx)
	return nil
}

func (d *cleanupDriver) Delete(ctx context.Context, vmUUID string) error {
	d.record(ctx)
	return nil
}

// assertLiveBoundedContexts checks that every delete got a context that was
// not cancelled with the build, and that each carries its own deadline.
func assertLiveBoundedContexts(t *testing.T, d *cleanupDriver, want int) {
	t.Helper()
	if len(d.calls) != want {
		t.Fatalf("expected %d delete calls, got %d", want, len(d.calls))
	}
	for i, c := range d.calls {
		if c.err != nil {
			t.Errorf("delete call %d got a cancelled context: %v", i, c.err)
		}
		if !c.hasDeadline {
			t.Errorf("delete call %d has no deadline", i)
		}
	}
}

func cancelledCleanupState(d Driver) *multistep.BasicStateBag {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	state := new(multistep.BasicStateBag)
	state.Put("ui", &packer.BasicUi{Reader: new(bytes.Buffer), Writer: io.Discard, ErrorWriter: io.Discard})
	state.Put("driver", d)
	state.Put("ctx", ctx)
	return state
}

// TestStepCreateImageCleanupAfterCancel checks that cleanup still issues its
// deletes with a live context when the build context has been cancelled.
func TestStepCreateImageCleanupAfterCancel(t *testing.T) {
	d := &cleanupDriver{}
	state := cancelledCleanupState(d)
	state.Put("image_uuid", []imageArtefact{{uuid: "img-1"}, {uuid: "img-2"}})

	step := &stepCreateImage{Config: &Config{}}
	step.Config.ImageDelete = true
	step.Cleanup(state)

	assertLiveBoundedContexts(t, d, 2)
}

// TestStepBuildVMCleanupAfterCancel does the same for the build VM step. On a
// real cancel the runner sets StateCancelled, so the step deletes the CD image
// and the marked source images but keeps the VM.
func TestStepBuildVMCleanupAfterCancel(t *testing.T) {
	d := &cleanupDriver{}
	state := cancelledCleanupState(d)
	state.Put(multistep.StateCancelled, true)
	state.Put("vm_uuid", "vm-1")
	state.Put("config", &Config{})
	state.Put("cd_uuid", "cd-1")
	state.Put("image_to_delete", []string{"src-1"})

	(&stepBuildVM{}).Cleanup(state)

	assertLiveBoundedContexts(t, d, 2)
}

func TestSleepCtx(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	start := time.Now()
	if err := sleepCtx(ctx, time.Minute); !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
	if time.Since(start) > time.Second {
		t.Error("sleepCtx did not return promptly on a cancelled context")
	}
	if err := sleepCtx(context.Background(), time.Millisecond); err != nil {
		t.Errorf("err = %v, want nil after the duration", err)
	}
}
