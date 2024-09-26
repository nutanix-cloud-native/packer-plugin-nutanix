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
	// vm, _ := d.GetVM(vmUUID)

	ui.Say(fmt.Sprintf("Exporting image(s) from virtual machine %s...", s.VMName))

	// Create a channel to receive signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	for index, imageToExport := range imageList {

		name := s.ImageName
		if index > 0 {
			name = fmt.Sprintf("%s-disk%d", name, index+1)
		}

		file, err := d.ExportImage(ctx, imageToExport.uuid)
		if err != nil {
			ui.Error("Image export failed: " + err.Error())
			state.Put("error", err)
			return multistep.ActionHalt
		}
		defer file.Close()

		toRead := ui.TrackProgress(name, 0, imageToExport.size, file)

		tempDestinationPath := name + ".tmp"

		f, err := os.OpenFile(tempDestinationPath, os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return multistep.ActionHalt
		}

		// Use a goroutine to copy the data, so that we can
		// interrupt it if necessary
		copyDone := make(chan bool)
		go func() {
			io.Copy(f, toRead)
			copyDone <- true
		}()

		select {
		case <-copyDone:
			toRead.Close()

			// Check if size is OK
			fi, err := f.Stat()
			if err != nil {
				ui.Error("Image stat failed: " + err.Error())
				state.Put("error", err)
				return multistep.ActionHalt
			}

			if fi.Size() != imageToExport.size {
				os.Remove(tempDestinationPath)
				ui.Error("image size mistmatch")
				state.Put("error", fmt.Errorf("image size mistmatch"))
				return multistep.ActionHalt
			}

			name = name + ".img"
			os.Rename(tempDestinationPath, name)

			ui.Message(fmt.Sprintf("image %s exported", name))

		case <-sigChan:
			// We received a signal, cancel the copy operation
			toRead.Close()
			f.Close()
			os.Remove(tempDestinationPath)
			ui.Message("image export cancelled")
			return multistep.ActionHalt
		}

	}
	return multistep.ActionContinue
}

func (s *stepExportImage) Cleanup(state multistep.StateBag) {}
