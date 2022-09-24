package nutanix

import (
	"context"
	"fmt"
	"log"

	"github.com/hashicorp/hcl/v2/hcldec"
	"github.com/hashicorp/packer-plugin-sdk/communicator"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/multistep/commonsteps"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
)

const BuilderId = "packer.nutanix"

// Builder - struct for building nutanix-builder
type Builder struct {
	config Config
	runner multistep.Runner
}

var _ packersdk.Builder = &Builder{}

func (b *Builder) ConfigSpec() hcldec.ObjectSpec { return b.config.FlatMapstructure().HCL2Spec() }

func (b *Builder) Prepare(raws ...interface{}) ([]string, []string, error) {

	warnings, errs := b.config.Prepare(raws...)
	if errs != nil {
		return nil, warnings, errs
	}

	return nil, warnings, nil
}

// Run the nutanix builder
func (b *Builder) Run(ctx context.Context, ui packersdk.Ui, hook packersdk.Hook) (packersdk.Artifact, error) {

	// Create the driver that we'll use to communicate with Nutanix
	log.Printf("PC Host from config: %s", b.config.ClusterConfig.Endpoint)
	driver, err := b.newDriver(b.config.ClusterConfig)
	if err != nil {
		return nil, fmt.Errorf("failed creating nutanix driver: %s", err)
	}

	// Setup the state bag
	state := new(multistep.BasicStateBag)
	state.Put("config", &b.config)
	state.Put("debug", b.config.PackerDebug)
	state.Put("driver", driver)
	state.Put("hook", hook)
	state.Put("ui", ui)

	steps := []multistep.Step{
		&stepPrepareImage{
			Config: &b.config,
		},
		&commonsteps.StepCreateCD{
			Files:   b.config.CDConfig.CDFiles,
			Content: b.config.CDConfig.CDContent,
			Label:   b.config.CDConfig.CDLabel,
		},
		&stepBuildVM{},
		&communicator.StepConnect{
			Config:    &b.config.CommConfig,
			SSHConfig: b.config.CommConfig.SSHConfigFunc(),
			Host:      commHost(),
		},
		new(commonsteps.StepProvision),
		&StepShutdown{
			Command: b.config.ShutdownCommand,
			Timeout: b.config.ShutdownTimeout,
		},
		&stepCopyImage{
			Config: &b.config,
		},
		&stepDestroyVM{
			Config: &b.config,
		},
	}

	b.runner = &multistep.BasicRunner{Steps: steps}
	b.runner.Run(ctx, state)

	if rawErr, ok := state.GetOk("error"); ok {
		return nil, rawErr.(error)
	}

	if vmUUID, ok := state.GetOk("vm_disk_uuid"); ok {
		if vmUUID != nil {
			artifact := &Artifact{
				Name: b.config.VmConfig.VMName,
				UUID: vmUUID.(string),
			}
			return artifact, nil
		}
	}
	return nil, nil
}

// builder.go
func (b *Builder) newDriver(cConfig ClusterConfig) (Driver, error) {
	driver := &NutanixDriver{
		ClusterConfig: cConfig,
	}

	return driver, nil
}
