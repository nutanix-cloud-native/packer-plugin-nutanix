package nutanix

import (
	"context"
	"fmt"
	"os"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
)

type stepExportImage struct {
	VMName    string
	ImageName string
}

func (s *stepExportImage) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packer.Ui)
	imageList := state.Get("image_uuid").([]imageArtefact)
	d := state.Get("driver").(Driver)

	ui.Say(fmt.Sprintf("Exporting image(s) from virtual machine %s...", s.VMName))

	for index, imageToExport := range imageList {

		name := s.ImageName
		if index > 0 {
			name = fmt.Sprintf("%s-disk%d", name, index+1)
		}

		ui.Say(fmt.Sprintf("Downloading image %s...", name))

		// ExportImage now returns the path to the downloaded file
		downloadedFilePath, err := d.ExportImage(ctx, imageToExport.uuid)
		if err != nil {
			ui.Error("Image export failed: " + err.Error())
			state.Put("error", err)
			return multistep.ActionHalt
		}

		// Check if size is OK
		fi, err := os.Stat(downloadedFilePath)
		if err != nil {
			ui.Error("Image stat failed: " + err.Error())
			state.Put("error", err)
			return multistep.ActionHalt
		}

		if imageToExport.size > 0 && fi.Size() != imageToExport.size {
			os.Remove(downloadedFilePath)
			ui.Error("image size mismatch")
			state.Put("error", fmt.Errorf("image size mismatch: expected %d, got %d", imageToExport.size, fi.Size()))
			return multistep.ActionHalt
		}

		// Rename to final name
		finalName := name + ".img"
		err = os.Rename(downloadedFilePath, finalName)
		if err != nil {
			ui.Error("Failed to rename image file: " + err.Error())
			state.Put("error", err)
			return multistep.ActionHalt
		}

		ui.Say(fmt.Sprintf("Image %s exported", finalName))
	}

	return multistep.ActionContinue
}

func (s *stepExportImage) Cleanup(state multistep.StateBag) {}
