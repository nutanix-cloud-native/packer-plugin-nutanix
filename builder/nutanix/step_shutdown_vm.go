package nutanix

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
)

// This step shuts down the machine. It first attempts to do so gracefully,
// but ultimately forcefully shuts it down if that fails.
//
// Uses:
//
//	communicator packersdk.Communicator
//	driver Driver
//	ui     packersdk.Ui
//	vmName string
//
// Produces:
//
//	<nothing>
type StepShutdown struct {
	Command             string
	Timeout             time.Duration
	DisableStopInstance bool

	// pollInterval is how often the VM's power state is checked; zero means
	// defaultShutdownPollInterval. Set by tests.
	pollInterval time.Duration
}

const defaultShutdownPollInterval = 15 * time.Second

func (s *StepShutdown) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	comm := state.Get("communicator").(packersdk.Communicator)
	driver := state.Get("driver").(Driver)
	ui := state.Get("ui").(packersdk.Ui)
	config := state.Get("config").(*Config)
	vmUUID := state.Get("vm_uuid").(string)

	if !s.DisableStopInstance {

		if config.Comm.Type == "none" {
			ui.Say("No Communicator configured, halting the virtual machine...")
			if err := driver.PowerOff(ctx, vmUUID); err != nil {
				err := fmt.Errorf("error stopping VM: %s", err)
				state.Put("error", err)
				ui.Error(err.Error())
				return multistep.ActionHalt
			}

		} else if s.Command != "" {
			ui.Say("Gracefully halting virtual machine...")
			log.Printf("executing shutdown command: %s", s.Command)
			cmd := &packersdk.RemoteCmd{Command: s.Command}
			if err := cmd.RunWithUi(ctx, comm, ui); err != nil {
				err := fmt.Errorf("failed to send shutdown command: %s", err)
				state.Put("error", err)
				ui.Error(err.Error())
				return multistep.ActionHalt
			}

		} else {
			ui.Say("Halting the virtual machine...")
			if err := driver.PowerOff(ctx, vmUUID); err != nil {
				err := fmt.Errorf("error stopping VM: %s", err)
				state.Put("error", err)
				ui.Error(err.Error())
				return multistep.ActionHalt
			}
		}
	} else {
		ui.Say("Automatic instance stop disabled. Please stop instance manually.")
	}

	// Wait for the machine to actually shut down
	log.Printf("waiting max %s for shutdown to complete", s.Timeout)
	// The deadline is checked after each poll, so the VM's state is always
	// checked once more after the timeout expires, as before.
	deadline := time.Now().Add(s.Timeout)
	pollInterval := s.pollInterval
	if pollInterval == 0 {
		pollInterval = defaultShutdownPollInterval
	}
	// lastGetVMErr holds the most recent poll's GetVM error, and is reported if
	// the wait times out, so a persistent error (e.g. 401 or 404) is not hidden
	// behind a bare timeout. A successful poll clears it.
	var lastGetVMErr error
	for {
		// GetVM honours ctx, so it errors once the build is cancelled; a nil
		// VM must not be dereferenced.
		running, err := driver.GetVM(ctx, vmUUID)
		lastGetVMErr = err
		if err != nil {
			log.Printf("error getting VM power state: %s", err)
		} else if running.PowerState() == "OFF" {
			log.Printf("VM powered off")
			break
		}

		if time.Now().After(deadline) {
			err := errors.New("timeout while waiting for machine to shutdown")
			if lastGetVMErr != nil {
				err = fmt.Errorf("timeout while waiting for machine to shutdown; last error getting VM power state: %w", lastGetVMErr)
			}
			state.Put("error", err)
			ui.Error(err.Error())
			return multistep.ActionHalt
		}

		select {
		case <-ctx.Done():
			err := fmt.Errorf("build cancelled while waiting for machine to shutdown: %w", ctx.Err())
			state.Put("error", err)
			ui.Error(err.Error())
			return multistep.ActionHalt
		case <-time.After(pollInterval):
		}
	}

	log.Println("VM shut down.")

	return multistep.ActionContinue
}

func (s *StepShutdown) Cleanup(state multistep.StateBag) {}
