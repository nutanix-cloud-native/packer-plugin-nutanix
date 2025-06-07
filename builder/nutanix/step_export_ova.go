package nutanix

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
)

type StepExportOVA struct {
	VMName    string
	OvaConfig OvaConfig
}

func (s *StepExportOVA) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packer.Ui)
	d := state.Get("driver").(Driver)
	vmUUID := state.Get("vm_uuid")

	ui.Say(fmt.Sprintf("Exporting OVA for virtual machine %s...", s.VMName))

	// Create a channel to receive signals
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	file, err := d.ExportOVA(ctx, s.OvaConfig.Name)
	if err != nil {
		ui.Error("Image export failed: " + err.Error())
		state.Put("error", err)
		return multistep.ActionHalt
	}
	defer file.Close()

	toRead := ui.TrackProgress(s.VMName, 0, 0, file)

	tempDestinationPath := vmUUID.(string) + ".tmp"

	f, err := os.OpenFile(tempDestinationPath, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return multistep.ActionHalt
	}

	copyDone := make(chan bool)
	go func() {
		io.Copy(f, toRead)
		copyDone <- true
	}()

	select {
	case <-copyDone:
		toRead.Close()

		// Check if size is OK
		_, err := f.Stat()
		if err != nil {
			ui.Error("Image stat failed: " + err.Error())
			state.Put("error", err)
			return multistep.ActionHalt
		}

		name := s.OvaConfig.Name + "." + strings.ToLower(s.OvaConfig.Format)
		os.Rename(tempDestinationPath, name)

		ui.Message(fmt.Sprintf("Image exported as \"%s\"", name))

	case <-sigChan:
		toRead.Close()
		f.Close()
		os.Remove(tempDestinationPath)
		ui.Message("image export cancelled")
		return multistep.ActionHalt
	}

	return multistep.ActionContinue
}

func (s *StepExportOVA) Cleanup(state multistep.StateBag) {}
