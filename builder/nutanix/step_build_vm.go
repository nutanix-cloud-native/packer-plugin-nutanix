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
		log.Println("CD disk found, " + cdFilesPath)
		cdfilesImage, err := d.UploadImage(cdFilesPath, "PATH", "ISO_IMAGE", config.VmConfig)
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

	ui.Say("Creating Packer Builder virtual machine...")
	//CreateRequest()
	vmRequest, err := d.CreateRequest(config.VmConfig)
	if err != nil {
		ui.Error("Error creating virtual machine request: " + err.Error())
		state.Put("error", err)
		return multistep.ActionHalt
	}
	vmInstance, err := d.Create(vmRequest)

	if err != nil {
		ui.Error("Unable to create virtual machine: " + err.Error())
		state.Put("error", err)
		return multistep.ActionHalt
	}

	ui.Message(fmt.Sprintf("Virtual machine %s created", config.VMName))
	log.Printf("Nutanix VM UUID: %s", *vmInstance.nutanix.Metadata.UUID)
	state.Put("vm_uuid", *vmInstance.nutanix.Metadata.UUID)
	state.Put("ip", vmInstance.Addresses()[0])
	state.Put("destroy_vm", true)
	ui.Message("Found IP for virtual machine: " + vmInstance.Addresses()[0])

	return multistep.ActionContinue
}

// Cleanup will tear down the VM once the build is complete
func (s *stepBuildVM) Cleanup(state multistep.StateBag) {
	vmUUID := state.Get("vm_uuid")
	if vmUUID == nil {
		return
	}

	_, cancelled := state.GetOk(multistep.StateCancelled)
	_, halted := state.GetOk(multistep.StateHalted)

	d := state.Get("driver").(Driver)
	ui := state.Get("ui").(packer.Ui)

	if cancelled || halted {
		ui.Say("Task cancelled, virtual machine is not deleted")
		return
	}

	ui.Say("Deleting virtual machine...")

	if cdUUID, ok := state.GetOk("cd_uuid"); ok {
		err := d.DeleteImage(cdUUID.(string))
		if err != nil {
			ui.Error("An error occurred while deleting CD disk")
			return
		} else {
			ui.Message("CD disk successfully deleted")
		}
	}

	err := d.Delete(vmUUID.(string))
	if err != nil {
		ui.Error("An error occurred while deleting the Virtual machine")
		return
	} else {
		ui.Message("Virtual machine successfully deleted")
	}

}
