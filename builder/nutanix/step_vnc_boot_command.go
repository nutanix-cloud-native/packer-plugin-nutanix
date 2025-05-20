package nutanix

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/hashicorp/packer-plugin-sdk/bootcommand"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/hashicorp/packer-plugin-sdk/template/interpolate"
	"github.com/mitchellh/go-vnc"
)

type stepVNCBootCommand struct {
	Config *Config
}

func (s *stepVNCBootCommand) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	if s.Config.DisableVNC {
		log.Println("Skipping boot command step...")
		return multistep.ActionContinue
	}

	debug := state.Get("debug").(bool)
	ui := state.Get("ui").(packersdk.Ui)
	conn := state.Get("vnc_conn").(*vnc.ClientConn)
	defer conn.Close()

	// Wait for the virtual machine to boot.
	if int64(s.Config.BootWait) > 0 {
		ui.Sayf("Waiting %s for boot...", s.Config.BootWait.String())
		select {
		case <-time.After(s.Config.BootWait):
			break
		case <-ctx.Done():
			return multistep.ActionHalt
		}
	}
	var pauseFn multistep.DebugPauseFn
	if debug {
		pauseFn = state.Get("pauseFn").(multistep.DebugPauseFn)
	}

	d := bootcommand.NewVNCDriver(conn, s.Config.BootKeyInterval)

	ui.Say("Typing the boot command over VNC...")
	flatBootCommand := s.Config.FlatBootCommand()
	command, err := interpolate.Render(flatBootCommand, &s.Config.ctx)
	if err != nil {
		err = fmt.Errorf("error preparing boot command: %s", err)
		state.Put("error", err)
		ui.Error(err.Error())
		return multistep.ActionHalt
	}

	seq, err := bootcommand.GenerateExpressionSequence(command)
	if err != nil {
		err := fmt.Errorf("error generating boot command: %s", err)
		state.Put("error", err)
		ui.Error(err.Error())
		return multistep.ActionHalt
	}

	if err := seq.Do(ctx, d); err != nil {
		err = fmt.Errorf("error running boot command: %s", err)
		state.Put("error", err)
		ui.Error(err.Error())
		return multistep.ActionHalt
	}

	if pauseFn != nil {
		pauseFn(multistep.DebugLocationAfterRun,
			fmt.Sprintf("boot_command: %s", command), state)
	}
	return multistep.ActionContinue

}

func (s *stepVNCBootCommand) Cleanup(state multistep.StateBag) {
	// No cleanup needed
}
