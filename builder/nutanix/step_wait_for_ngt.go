package nutanix

import (
	"bytes"
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
)

const ngtPollInterval = 5 * time.Second

type stepWaitForNGT struct{}

func (s *stepWaitForNGT) Run(ctx context.Context, state multistep.StateBag) multistep.StepAction {
	config := state.Get("config").(*Config)
	if !config.EnableNGT {
		return multistep.ActionContinue
	}

	ui := state.Get("ui").(packer.Ui)
	driver := state.Get("driver").(Driver)
	vmUUID := state.Get("vm_uuid").(string)

	ui.Say("Waiting for Nutanix Guest Tools connectivity...")
	status, err := waitForNGT(ctx, driver, vmUUID, config.NGTWaitTimeout)
	if err == nil {
		return multistep.ActionContinue
	}

	if config.NGTRestartCommand == "" {
		ui.Error("Nutanix Guest Tools did not become reachable: " + err.Error())
		state.Put("error", err)
		return multistep.ActionHalt
	}

	ui.Say("Restarting Nutanix Guest Tools in the virtual machine...")
	if err := restartNGT(ctx, state, config.NGTRestartCommand); err != nil {
		ui.Error("Unable to restart Nutanix Guest Tools: " + err.Error())
		state.Put("error", err)
		return multistep.ActionHalt
	}

	ui.Say("Waiting for Nutanix Guest Tools connectivity after restart...")
	status, err = waitForNGT(ctx, driver, vmUUID, config.NGTWaitTimeout)
	if err != nil {
		err = fmt.Errorf("NGT status after restart: %w; last status: %+v", err, status)
		ui.Error(err.Error())
		state.Put("error", err)
		return multistep.ActionHalt
	}

	return multistep.ActionContinue
}

func (s *stepWaitForNGT) Cleanup(state multistep.StateBag) {}

func waitForNGT(ctx context.Context, driver Driver, vmUUID string, timeout time.Duration) (ngtStatus, error) {
	if timeout <= 0 {
		timeout = 10 * time.Minute
	}

	timer := time.NewTimer(timeout)
	defer timer.Stop()
	ticker := time.NewTicker(ngtPollInterval)
	defer ticker.Stop()

	var lastStatus ngtStatus
	var lastErr error
	for {
		status, err := driver.GetNGTStatus(ctx, vmUUID)
		if err == nil {
			lastStatus = status
			if status.Enabled && status.Installed && status.Reachable {
				return status, nil
			}
		} else {
			lastErr = err
		}

		select {
		case <-ctx.Done():
			return lastStatus, ctx.Err()
		case <-timer.C:
			if lastErr != nil {
				return lastStatus, fmt.Errorf("timed out waiting for NGT: %w", lastErr)
			}
			return lastStatus, fmt.Errorf("timed out waiting for NGT")
		case <-ticker.C:
		}
	}
}

func restartNGT(ctx context.Context, state multistep.StateBag, command string) error {
	communicator, ok := state.Get("communicator").(packer.Communicator)
	if !ok {
		return fmt.Errorf("communicator is not available")
	}

	var stdout, stderr bytes.Buffer
	remoteCmd := &packer.RemoteCmd{
		Command: command,
		Stdout:  &stdout,
		Stderr:  &stderr,
	}
	if err := communicator.Start(ctx, remoteCmd); err != nil {
		return err
	}
	if exitStatus := remoteCmd.Wait(); exitStatus != 0 {
		return fmt.Errorf("restart command exited with status %d: %s", exitStatus, stderr.String())
	}

	return nil
}
