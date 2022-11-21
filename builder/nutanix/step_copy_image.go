package nutanix

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
)

type stepCopyImage struct {
	Config *Config
}

func (s *stepCopyImage) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packer.Ui)
	vmUUID := state.Get("vmUUID").(string)
	ui.Say("Retrieving Status for uuid: " + vmUUID)
	d := state.Get("driver").(Driver)
	vm, _ := d.GetVM(vmUUID)

	ui.Say("Creating image for uuid: " + vmUUID)

	ui.Message("Initiatiating save VM DISK task.")
	// Choose disk to replicate - looking for first "DISK"
	var diskToCopy string

	for i := range vm.nutanix.Spec.Resources.DiskList {
		if *vm.nutanix.Spec.Resources.DiskList[i].DeviceProperties.DeviceType == "DISK" {
			diskToCopy = *vm.nutanix.Spec.Resources.DiskList[i].UUID
			ui.Message("Found DISK to copy: " + diskToCopy)
			break
		}
	}

	if diskToCopy == "" {
		err := errors.New("no DISK was found to save, halting build")
		ui.Error(err.Error())
		state.Put("error", err)
		return multistep.ActionHalt
	}

	imageResponse, err := d.SaveVMDisk(diskToCopy)
	if err != nil {
		ui.Error("Unexpected Nutanix Task status: " + err.Error())
		state.Put("error", err)
		return multistep.ActionHalt
	}
	ui.Message(fmt.Sprintf("Successfully created image: %s (%s)", *imageResponse.image.Spec.Name, *imageResponse.image.Metadata.UUID))
	state.Put("vm_disk_uuid", (*imageResponse.image.Metadata.UUID))
	return multistep.ActionContinue
}

func (s *stepCopyImage) Cleanup(state multistep.StateBag) {
}
