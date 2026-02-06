package nutanix

import (
	"context"
	"log"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
)

// stepCleanVM is the default struct which contains the step's information
type stepCleanVM struct {
	Config *Config
}

// Run is the primary function to clean up the VM
func (s *stepCleanVM) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	// Update UI
	ui := state.Get("ui").(packer.Ui)
	d := state.Get("driver").(Driver)
	vmUUID := state.Get("vm_uuid").(string)

	if !s.Config.Clean.Cdrom {
		log.Printf("No vm cleaning requested, skipping step.")
		return multistep.ActionContinue
	}

	vmResp, err := d.GetVM(ctx, vmUUID)
	if err != nil {
		ui.Error("Error retrieving virtual machine: " + err.Error())
		return multistep.ActionHalt
	}

	// Get the V4 VM directly for modification
	v4vm := vmResp.VM()

	if s.Config.Clean.Cdrom {
		ui.Say("Cleaning up CD-ROM in virtual machine...")
		d.CleanCD(ctx, v4vm)
	}

	_, err = d.UpdateVM(ctx, vmUUID, v4vm)
	if err != nil {
		ui.Error("Error updating virtual machine: " + err.Error())
		return multistep.ActionHalt
	}

	ui.Say("Virtual machine cleaned successfully.")
	return multistep.ActionContinue
}

func (s *stepCleanVM) Cleanup(state multistep.StateBag) {
	// No cleanup needed for VM cleaning step
}
