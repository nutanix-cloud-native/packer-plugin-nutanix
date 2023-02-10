package nutanix

import (
	"context"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
)

type stepDestroyVM struct {
	Config *Config
}

func (s *stepDestroyVM) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packer.Ui)
	vmUUID := state.Get("vmUUID").(string)
	d := state.Get("driver").(Driver)

	if imageUUID, ok := state.GetOk("imageUUID"); ok {
		err := d.DeleteImage(imageUUID.(string))
		if err != nil {
			ui.Error("An error occurred while deleting CD disk")
			return multistep.ActionHalt
		} else {
			ui.Say("CD disk successfully deleted.")
		}
	}

	err := d.Delete(vmUUID)
	if err != nil {
		ui.Error("An error occurred while deleting the VM.")
		return multistep.ActionHalt
	} else {
		ui.Say("Nutanix VM successfully deleted.")
	}
	return multistep.ActionContinue
}

func (s *stepDestroyVM) Cleanup(state multistep.StateBag) {
}
