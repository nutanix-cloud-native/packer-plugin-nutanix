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
	vmUUID := state.Get("vm_uuid").(string)
	d := state.Get("driver").(Driver)
	vm, _ := d.GetVM(vmUUID)

	ui.Say(fmt.Sprintf("Creating image from virtual machine %s...", s.Config.VMName))

	// Choose disk to replicate - looking for first "DISK"
	var diskToCopy string

	for i := range vm.nutanix.Spec.Resources.DiskList {
		if *vm.nutanix.Spec.Resources.DiskList[i].DeviceProperties.DeviceType == "DISK" {
			diskToCopy = *vm.nutanix.Spec.Resources.DiskList[i].UUID
			diskID := fmt.Sprintf("%s:%d", *vm.nutanix.Spec.Resources.DiskList[i].DeviceProperties.DiskAddress.AdapterType, *vm.nutanix.Spec.Resources.DiskList[i].DeviceProperties.DiskAddress.DeviceIndex)
			ui.Message("Found disk to copy: " + diskID)
			break
		}
	}

	if diskToCopy == "" {
		err := errors.New("no DISK was found to save, halting build")
		ui.Error(err.Error())
		state.Put("error", err)
		return multistep.ActionHalt
	}

	imageResponse, err := d.SaveVMDisk(diskToCopy, s.Config.ImageCategories)
	if err != nil {
		ui.Error("Image creation failed: " + err.Error())
		state.Put("error", err)
		return multistep.ActionHalt
	}
	ui.Message(fmt.Sprintf("Successfully created image: %s (%s)", *imageResponse.image.Spec.Name, *imageResponse.image.Metadata.UUID))
	state.Put("image_uuid", (*imageResponse.image.Metadata.UUID))
	return multistep.ActionContinue
}

func (s *stepCopyImage) Cleanup(state multistep.StateBag) {
	ui := state.Get("ui").(packer.Ui)
	d := state.Get("driver").(Driver)

	if !s.Config.ImageDelete {
		return
	}

	if imgUUID, ok := state.GetOk("image_uuid"); ok {
		ui.Say(fmt.Sprintf("Deleting image %s...", s.Config.ImageName))

		err := d.DeleteImage(imgUUID.(string))
		if err != nil {
			ui.Error("An error occurred while deleting image")
			return
		} else {
			ui.Message("Image successfully deleted")
		}
	}
}
