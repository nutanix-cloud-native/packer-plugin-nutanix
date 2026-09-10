package nutanix

import (
	"context"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
)

type stepEnableNGT struct{}

func (s *stepEnableNGT) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	config := state.Get("config").(*Config)
	if !config.EnableNGT {
		return multistep.ActionContinue
	}

	ui := state.Get("ui").(packer.Ui)
	driver := state.Get("driver").(Driver)
	vmUUID := state.Get("vm_uuid").(string)

	ui.Say("Enabling Nutanix Guest Tools on the virtual machine...")
	if err := driver.EnableNGT(ctx, vmUUID); err != nil {
		ui.Error("Unable to enable Nutanix Guest Tools: " + err.Error())
		state.Put("error", err)
		return multistep.ActionHalt
	}

	return multistep.ActionContinue
}

func (s *stepEnableNGT) Cleanup(state multistep.StateBag) {}
