package nutanix

import (
	"context"
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
	//config := state.Get("config").(*Config)
	ui.Say("Creating Packer Builder VM on Nutanix Cluster.")
	d := state.Get("driver").(Driver)
	config := state.Get("config").(*Config)

	// Determine if we even have a cd_files disk to attach
	log.Println("Check for temporary iso-disks to attach")
	if cdPathRaw, ok := state.GetOk("cd_path"); ok {
		cdFilesPath := cdPathRaw.(string)
		log.Println("temporary iso found, " + cdFilesPath)
		cdfilesImage, err := d.UploadImage(cdFilesPath, "PATH", "ISO_IMAGE",config.VmConfig)
		if err != nil {
			ui.Error("Error uploading temporary image:")
			ui.Error(err.Error())
			return multistep.ActionHalt
		}
		ui.Say("Temporary ISO uploaded")
		state.Put("imageUUID", *cdfilesImage.image.Metadata.UUID)
		temp_cd := VmDisk{
			ImageType:       "ISO_IMAGE",
			SourceImageUUID: *cdfilesImage.image.Metadata.UUID,
		}
		config.VmConfig.VmDisks = append(config.VmConfig.VmDisks, temp_cd)
	} else {
		log.Println("No temporary iso, not attaching.")
	}

	//CreateRequest()
	vmRequest, err := d.CreateRequest(config.VmConfig)
	if err != nil {
		ui.Error("Error creating Request: " + err.Error())
		return multistep.ActionHalt
	}
	vmInstance, err := d.Create(vmRequest)

	if err != nil {
		ui.Error("Unable to create Nutanix VM request: " + err.Error())
		state.Put("error", err)
		return multistep.ActionHalt
	}
	log.Printf("Nutanix VM UUID: %s", *vmInstance.nutanix.Metadata.UUID)
	state.Put("vmUUID", *vmInstance.nutanix.Metadata.UUID)
	state.Put("ip", vmInstance.Addresses()[0])
	ui.Say("IP for Nutanix device: " + vmInstance.Addresses()[0])

	return multistep.ActionContinue
}

// Cleanup will tear down the VM once the build is complete
func (s *stepBuildVM) Cleanup(state multistep.StateBag) {
	_, cancelled := state.GetOk(multistep.StateCancelled)
	_, halted := state.GetOk(multistep.StateHalted)
	if !cancelled && !halted {
		return
	}

	ui := state.Get("ui").(packer.Ui)

	if vmUUID, ok := state.GetOk("vmUUID"); ok {
		if vmUUID != "" {
			ui.Say("Cleaning up Nutanix VM.")

		}
	}
}
