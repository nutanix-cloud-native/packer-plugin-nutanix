package nutanix

import (
	"context"
	"fmt"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
)

type StepCreateOVA struct {
	VMName    string
	OvaConfig OvaConfig
}

func (s *StepCreateOVA) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packer.Ui)
	if s.OvaConfig.Format != "vmdk" && s.OvaConfig.Format != "qcow2" {
		ui.Say("OVA format is not supported. Please use 'vmdk' or 'qcow2'")
		return multistep.ActionContinue
	}
	d := state.Get("driver").(Driver)
	vmUUID := state.Get("vm_uuid")

	ui.Say(fmt.Sprintf("Creating OVA for virtual machine %s...", s.VMName))

	err := d.CreateOVA(ctx, s.OvaConfig.Name, vmUUID.(string), s.OvaConfig.Format)

	if err != nil {
		ui.Message("OVA creation failed")
		return multistep.ActionHalt
	}

	return multistep.ActionContinue
}

func (s *StepCreateOVA) Cleanup(state multistep.StateBag) {}
