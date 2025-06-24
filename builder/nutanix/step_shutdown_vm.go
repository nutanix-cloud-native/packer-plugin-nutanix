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
	Command string
	Timeout time.Duration
}

func (s *StepShutdown) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	comm := state.Get("communicator").(packersdk.Communicator)
	driver := state.Get("driver").(Driver)
	ui := state.Get("ui").(packersdk.Ui)
	config := state.Get("config").(*Config)
	vmUUID := state.Get("vm_uuid").(string)

	if config.CommConfig.Type == "none" {
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

	// Wait for the machine to actually shut down
	log.Printf("waiting max %s for shutdown to complete", s.Timeout)
	shutdownTimer := time.After(s.Timeout)
	for {
		running, _ := driver.GetVM(ctx, vmUUID)
		if running.PowerState() == "OFF" {
			log.Printf("VM powered off")
			break
		}

		select {
		case <-shutdownTimer:
			err := errors.New("timeout while waiting for machine to shutdown")
			state.Put("error", err)
			ui.Error(err.Error())
			return multistep.ActionHalt
		default:
			time.Sleep(15 * time.Second)
		}
	}

	log.Println("VM shut down.")

	return multistep.ActionContinue
}

func (s *StepShutdown) Cleanup(state multistep.StateBag) {}
