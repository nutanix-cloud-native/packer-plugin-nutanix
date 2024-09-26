package nutanix

import (
	"context"
	"errors"
	"fmt"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
)

type imageArtefact struct {
	uuid string
	size int64
}

type diskArtefact struct {
	uuid string
	size int64
}

type stepCopyImage struct {
	Config *Config
}

func (s *stepCopyImage) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packer.Ui)
	vmUUID := state.Get("vm_uuid").(string)
	d := state.Get("driver").(Driver)
	vm, _ := d.GetVM(ctx, vmUUID)

	ui.Say(fmt.Sprintf("Creating image(s) from virtual machine %s...", s.Config.VMName))

	// Choose disk to replicate - looking for first "DISK"
	var disksToCopy []diskArtefact

	for i := range vm.nutanix.Spec.Resources.DiskList {
		if *vm.nutanix.Spec.Resources.DiskList[i].DeviceProperties.DeviceType == "DISK" {
			disksToCopy = append(disksToCopy, diskArtefact{
				uuid: *vm.nutanix.Spec.Resources.DiskList[i].UUID,
				size: *vm.nutanix.Spec.Resources.DiskList[i].DiskSizeBytes,
			})
			diskID := fmt.Sprintf("%s:%d", *vm.nutanix.Spec.Resources.DiskList[i].DeviceProperties.DiskAddress.AdapterType, *vm.nutanix.Spec.Resources.DiskList[i].DeviceProperties.DiskAddress.DeviceIndex)
			ui.Message("Found disk to copy: " + diskID)
		}
	}

	if len(disksToCopy) == 0 {
		err := errors.New("no DISK was found to save, halting build")
		ui.Error(err.Error())
		state.Put("error", err)
		return multistep.ActionHalt
	}

	var imageList []imageArtefact

	for i, diskToCopy := range disksToCopy {

		imageResponse, err := d.SaveVMDisk(ctx, diskToCopy.uuid, i, s.Config.ImageCategories)
		if err != nil {
			ui.Error("Image creation failed: " + err.Error())
			state.Put("error", err)
			return multistep.ActionHalt
		}

		imageList = append(imageList, imageArtefact{
			uuid: *imageResponse.image.Metadata.UUID,
			size: diskToCopy.size,
		})

		ui.Message(fmt.Sprintf("Image successfully created: %s (%s)", *imageResponse.image.Spec.Name, *imageResponse.image.Metadata.UUID))
	}

	state.Put("image_uuid", imageList)
	return multistep.ActionContinue
}

func (s *stepCopyImage) Cleanup(state multistep.StateBag) {
	ui := state.Get("ui").(packer.Ui)
	d := state.Get("driver").(Driver)
	ctx, ok := state.Get("ctx").(context.Context)
	if !ok {
		ctx = context.Background()
	}

	if !s.Config.ImageDelete {
		return
	}

	if imgUUID, ok := state.GetOk("image_uuid"); ok {
		ui.Say(fmt.Sprintf("Deleting image(s) %s...", s.Config.ImageName))

		for _, image := range imgUUID.([]imageArtefact) {

			err := d.DeleteImage(ctx, image.uuid)
			if err != nil {
				ui.Error("An error occurred while deleting image")
				return
			} else {
				ui.Message(fmt.Sprintf("Image successfully deleted (%s)", image.uuid))
			}
		}
	}
}
