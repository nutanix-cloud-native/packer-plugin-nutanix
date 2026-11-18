package nutanix

import (
	"context"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
)

type stepCompleteGuestUpdate struct {
	Config *Config
}

func (s *stepCompleteGuestUpdate) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packer.Ui)
	d := state.Get("driver").(Driver)
	templateExtID := state.Get("template_ext_id").(string)

	versionName := s.Config.TemplateConfig.VersionName
	if versionName == "" {
		versionName = "packer-built"
	}

	versionDescription := s.Config.TemplateConfig.VersionDescription

	ui.Sayf("Completing guest update for template %s (version: %s)...", templateExtID, versionName)
	err := d.CompleteGuestUpdate(ctx, templateExtID, versionName, versionDescription)
	if err != nil {
		ui.Error("Failed to complete guest update: " + err.Error())
		state.Put("error", err)
		return multistep.ActionHalt
	}

	// Clear the initiated flag so cleanup doesn't try to cancel
	state.Put("guest_update_initiated", false)

	ui.Sayf("Template %s updated successfully, new version: %s", templateExtID, versionName)
	return multistep.ActionContinue
}

func (s *stepCompleteGuestUpdate) Cleanup(state multistep.StateBag) {
}
