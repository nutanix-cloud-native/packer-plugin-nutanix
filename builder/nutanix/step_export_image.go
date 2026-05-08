package nutanix

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"

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

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	for index, imageToExport := range imageList {
		name := s.ImageName
		if index > 0 {
			name = fmt.Sprintf("%s-disk%d", name, index+1)
		}

		ui.Say(fmt.Sprintf("Downloading image %s...", name))

		exportReader, err := d.ExportImage(ctx, imageToExport.uuid)
		if err != nil {
			ui.Error("Image export failed: " + err.Error())
			state.Put("error", err)
			return multistep.ActionHalt
		}
		defer exportReader.Close()

		tempDestinationPath := name + ".tmp"
		outFile, err := os.OpenFile(tempDestinationPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
		if err != nil {
			ui.Error("Image temp file create failed: " + err.Error())
			state.Put("error", err)
			return multistep.ActionHalt
		}

		progressTotal := int64(0)
		if imageToExport.size > 0 {
			progressTotal = imageToExport.size
		}
		trackedReader := ui.TrackProgress(name, 0, progressTotal, exportReader)

		copyDone := make(chan error, 1)
		go func() {
			_, copyErr := io.Copy(outFile, trackedReader)
			copyDone <- copyErr
		}()

		select {
		case copyErr := <-copyDone:
			if copyErr != nil {
				_ = outFile.Close()
				_ = os.Remove(tempDestinationPath)
				ui.Error("Image export copy failed: " + copyErr.Error())
				state.Put("error", copyErr)
				return multistep.ActionHalt
			}
			_ = trackedReader.Close()

			if err := outFile.Close(); err != nil {
				_ = os.Remove(tempDestinationPath)
				ui.Error("Image temp file close failed: " + err.Error())
				state.Put("error", err)
				return multistep.ActionHalt
			}

			fi, err := os.Stat(tempDestinationPath)
			if err != nil {
				_ = os.Remove(tempDestinationPath)
				ui.Error("Image temp stat failed: " + err.Error())
				state.Put("error", err)
				return multistep.ActionHalt
			}

			if imageToExport.size > 0 && fi.Size() != imageToExport.size {
				_ = os.Remove(tempDestinationPath)
				err := fmt.Errorf("image size mismatch: expected %d, got %d", imageToExport.size, fi.Size())
				ui.Error(err.Error())
				state.Put("error", err)
				return multistep.ActionHalt
			}

			finalName := name + ".img"
			if err := os.Rename(tempDestinationPath, finalName); err != nil {
				_ = os.Remove(tempDestinationPath)
				ui.Error("Failed to rename image file: " + err.Error())
				state.Put("error", err)
				return multistep.ActionHalt
			}

			ui.Say(fmt.Sprintf("Image %s exported", finalName))

		case <-sigChan:
			_ = trackedReader.Close()
			_ = outFile.Close()
			_ = os.Remove(tempDestinationPath)
			ui.Say("image export cancelled")
			return multistep.ActionHalt
		}
	}

	return multistep.ActionContinue
}

func (s *stepExportImage) Cleanup(state multistep.StateBag) {}
