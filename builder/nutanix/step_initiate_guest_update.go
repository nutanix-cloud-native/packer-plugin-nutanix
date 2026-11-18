package nutanix

import (
	"context"
	"fmt"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
)

type stepInitiateGuestUpdate struct {
	Config *Config
}

func (s *stepInitiateGuestUpdate) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packer.Ui)
	d := state.Get("driver").(Driver)

	templateExtID := s.Config.TemplateConfig.ExtID

	// Resolve template ext ID from name if not provided directly
	if templateExtID == "" {
		ui.Sayf("Looking up template by name %q...", s.Config.TemplateConfig.Name)
		extID, err := d.FindTemplateByName(ctx, s.Config.TemplateConfig.Name)
		if err != nil {
			ui.Error("Failed to find template: " + err.Error())
			state.Put("error", err)
			return multistep.ActionHalt
		}
		templateExtID = extID
		ui.Sayf("Found template ext ID: %s", templateExtID)
	}

	state.Put("template_ext_id", templateExtID)

	ui.Sayf("Initiating guest update for template %s...", templateExtID)
	vmUUID, err := d.InitiateGuestUpdate(ctx, templateExtID, "")
	if err != nil {
		ui.Error("Failed to initiate guest update: " + err.Error())
		state.Put("error", err)
		return multistep.ActionHalt
	}

	state.Put("guest_update_initiated", true)
	state.Put("vm_uuid", vmUUID)

	ui.Sayf("Temporary VM created: %s", vmUUID)

	// Power on the temporary VM so provisioners can connect
	ui.Say("Powering on temporary VM...")
	if err := d.PowerOn(ctx, vmUUID); err != nil {
		ui.Error("Failed to power on temporary VM: " + err.Error())
		state.Put("error", err)
		return multistep.ActionHalt
	}

	ui.Say("Temporary VM is powered on and ready for provisioning")
	return multistep.ActionContinue
}

func (s *stepInitiateGuestUpdate) Cleanup(state multistep.StateBag) {
	_, initiated := state.GetOk("guest_update_initiated")
	if !initiated {
		return
	}

	_, cancelled := state.GetOk(multistep.StateCancelled)
	_, halted := state.GetOk(multistep.StateHalted)
	if !cancelled && !halted {
		return
	}

	templateExtID, ok := state.Get("template_ext_id").(string)
	if !ok || templateExtID == "" {
		return
	}

	ui := state.Get("ui").(packer.Ui)
	d := state.Get("driver").(Driver)
	ctx, ok := state.Get("ctx").(context.Context)
	if !ok {
		ctx = context.Background()
	}

	ui.Say("Build failed or cancelled, cancelling guest update...")
	err := d.CancelGuestUpdate(ctx, templateExtID)
	if err != nil {
		ui.Error(fmt.Sprintf("Warning: failed to cancel guest update: %s", err.Error()))
	} else {
		ui.Say("Guest update cancelled, temporary VM removed")
	}
}
