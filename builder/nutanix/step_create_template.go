package nutanix

import (
	"context"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
)

type stepCreateTemplate struct {
	Config *Config
}

func (s *stepCreateTemplate) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packer.Ui)
	vmUUID := state.Get("vm_uuid").(string)
	d := state.Get("driver").(Driver)

	ui.Sayf("Creating Template for virtual machine %s...", s.Config.VMName)

	err := d.CreateTemplate(ctx, vmUUID, s.Config.TemplateConfig)
	if err != nil {
		ui.Error("Failed to create template: " + err.Error())
		state.Put("error", err)
		return multistep.ActionHalt
	}

	ui.Sayf("Template %s created successfully.", s.Config.TemplateConfig.Name)

	return multistep.ActionContinue
}

func (s *stepCreateTemplate) Cleanup(state multistep.StateBag) {
	// No cleanup needed for template creation step
}
