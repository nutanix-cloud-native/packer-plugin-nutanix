package nutanix

import (
	"context"
	"fmt"
	"log"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
)

// stepBuildVM is the default struct which contains the step's information
type stepBuildVM struct {
}

// Run is the primary function to build the image
func (s *stepBuildVM) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	//Update UI
	ui := state.Get("ui").(packer.Ui)
	d := state.Get("driver").(Driver)
	config := state.Get("config").(*Config)

	// Determine if we even have a cd_files disk to attach
	log.Println("check for CD disk to attach")
	if cdPathRaw, ok := state.GetOk("cd_path"); ok {
		ui.Say("Uploading CD disk...")
		cdFilesPath := cdPathRaw.(string)
		log.Println("CD disk found " + cdFilesPath)
		cdfilesImage, err := d.CreateImageFile(ctx, cdFilesPath, config.VmConfig)
		if err != nil {
			ui.Error("Error uploading CD disk:" + err.Error())
			state.Put("error", err)
			return multistep.ActionHalt
		}
		ui.Message("CD disk uploaded " + *cdfilesImage.image.Spec.Name)
		state.Put("cd_uuid", *cdfilesImage.image.Metadata.UUID)
		temp_cd := VmDisk{
			ImageType:       "ISO_IMAGE",
			SourceImageUUID: *cdfilesImage.image.Metadata.UUID,
		}
		config.VmConfig.VmDisks = append(config.VmConfig.VmDisks, temp_cd)
	} else {
		log.Println("no CD disk, not attaching.")
	}

	log.Println("check for local ISO to upload and attach")
	// Loop through config.VmConfig.VmDisks and upload source_image_path if present
	for i, disk := range config.VmConfig.VmDisks {
		if disk.SourceImagePath != "" {
			ui.Say(fmt.Sprintf("Uploading disk %d from source image path...", i))
			log.Println("Disk source image path found: " + disk.SourceImagePath)
			uploadedImage, err := d.CreateImageFile(ctx, disk.SourceImagePath, config.VmConfig)
			if err != nil {
				ui.Error(fmt.Sprintf("Error uploading disk %d: %s", i, err.Error()))
				state.Put("error", err)
				return multistep.ActionHalt
			}
			ui.Message(fmt.Sprintf("Disk %d uploaded: %s", i, *uploadedImage.image.Spec.Name))
			state.Put(fmt.Sprintf("disk_%d_uuid", i), *uploadedImage.image.Metadata.UUID)
			config.VmConfig.VmDisks[i].SourceImageUUID = *uploadedImage.image.Metadata.UUID
			config.VmConfig.VmDisks[i].SourceImagePath = "" // Clear the path after upload
		} else {
			log.Printf("Disk %d has no source image path, skipping upload.", i)
		}
	}


	ui.Say("Creating Packer Builder virtual machine...")

	// Create VM Spec
	vmRequest, err := d.CreateRequest(ctx, config.VmConfig, state)
	if err != nil {
		ui.Error("Error creating virtual machine request: " + err.Error())
		state.Put("error", err)
		return multistep.ActionHalt
	}

	// Create VM
	vmInstance, err := d.Create(ctx, vmRequest)

	if err != nil {
		ui.Error("Unable to create virtual machine: " + err.Error())
		state.Put("error", err)
		return multistep.ActionHalt
	}
	log.Printf("Nutanix VM UUID: %s", *vmInstance.nutanix.Metadata.UUID)
	ui.Message(fmt.Sprintf("Virtual machine %s created", config.VMName))
	state.Put("destroy_vm", true)
	state.Put("vm_uuid", *vmInstance.nutanix.Metadata.UUID)
	state.Put("cluster_uuid", *vmInstance.nutanix.Spec.ClusterReference.UUID)

	return multistep.ActionContinue
}

// Cleanup will tear down the VM once the build is complete
func (s *stepBuildVM) Cleanup(state multistep.StateBag) {
	vmUUID := state.Get("vm_uuid")
	if vmUUID == nil {
		return
	}

	ui := state.Get("ui").(packer.Ui)
	d := state.Get("driver").(Driver)
	config := state.Get("config").(*Config)
	ctx, ok := state.Get("ctx").(context.Context)
	if !ok {
		ctx = context.Background()
	}

	if cdUUID, ok := state.GetOk("cd_uuid"); ok {
		ui.Say("Deleting temporary CD disk...")
		err := d.DeleteImage(ctx, cdUUID.(string))
		if err != nil {
			ui.Error("An error occurred while deleting CD disk")
			log.Println(err)
		}
		ui.Message("Temporary CD disk successfully deleted")
	}

	imageToDelete := state.Get("image_to_delete")

	for _, image := range imageToDelete.([]string) {
		ui.Say(fmt.Sprintf("Deleting marked source_image: %s...", image))
		err := d.DeleteImage(ctx, image)
		if err != nil {
			ui.Error(fmt.Sprintf("An error occurred while deleting image %s", image))
			log.Println(err)
		}
		ui.Message("Image successfully deleted")
	}

	_, cancelled := state.GetOk(multistep.StateCancelled)
	_, halted := state.GetOk(multistep.StateHalted)

	if cancelled || halted && !config.VmForceDelete {
		ui.Say("Task cancelled, virtual machine is not deleted")
		return
	} else if config.VmForceDelete && cancelled || halted {
		ui.Say("Force deleting virtual machine...")
	} else if config.VmRetain {
		ui.Say("Retaining virtual machine...")
		return
	} else {
		ui.Say("Deleting virtual machine...")
	}

	err := d.Delete(ctx, vmUUID.(string))
	if err != nil {
		ui.Error("An error occurred while deleting the Virtual machine")
		log.Println(err)
	} else {
		ui.Message("Virtual machine successfully deleted")
	}

}
