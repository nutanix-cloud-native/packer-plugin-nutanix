package nutanix

import (
	"context"
	"fmt"
	"os"

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

	ui.Say(fmt.Sprintf("Exporting OVA for virtual machine %s...", s.VMName))

	// ExportOVA now returns the path to the downloaded file
	downloadedFilePath, err := d.ExportOVA(ctx, s.OvaConfig.Name)
	if err != nil {
		ui.Error("OVA export failed: " + err.Error())
		state.Put("error", err)
		return multistep.ActionHalt
	}

	// Rename the downloaded file to the final OVA name
	finalName := s.OvaConfig.Name + ".ova"
	err = os.Rename(downloadedFilePath, finalName)
	if err != nil {
		ui.Error("Failed to rename OVA file: " + err.Error())
		state.Put("error", err)
		return multistep.ActionHalt
	}

	ui.Say(fmt.Sprintf("OVA exported as \"%s\"", finalName))
	return multistep.ActionContinue
}

func (s *StepExportOVA) Cleanup(state multistep.StateBag) {}
