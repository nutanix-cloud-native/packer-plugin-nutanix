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
	// multistep's step.Cleanup doesn't admit context but context is actually needed for cleanup,
	// so we put it in the state bag to be used by the cleanup step
	state.Put("ctx", ctx)

	steps := []multistep.Step{
		&commonsteps.StepCreateCD{
			Files:   b.config.CDConfig.CDFiles,
			Content: b.config.CDConfig.CDContent,
			Label:   b.config.CDConfig.CDLabel,
		},
		&stepBuildVM{},
		&stepVNCConnect{
			Config: &b.config,
		},
		&stepVNCBootCommand{
			Config: &b.config,
		},
		&stepWaitForIp{
			Config: &b.config.WaitIpConfig,
		},
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
	}

	if !b.config.ImageSkip {
		steps = append(steps, &stepCreateImage{
			Config: &b.config,
		})
	}

	if b.config.OvaConfig.Create {
		steps = append(steps, &StepCreateOVA{
			VMName:    b.config.VMName,
			OvaConfig: b.config.OvaConfig,
		})
	}

	if b.config.OvaConfig.Export {
		steps = append(steps, &StepExportOVA{
			VMName:    b.config.VMName,
			OvaConfig: b.config.OvaConfig,
		})
	}

	if b.config.ImageExport {
		steps = append(steps, &stepExportImage{
			VMName:    b.config.VMName,
			ImageName: b.config.VmConfig.ImageName,
		})
	}

	b.runner = &multistep.BasicRunner{Steps: steps}
	b.runner.Run(ctx, state)

	if rawErr, ok := state.GetOk("error"); ok {
		return nil, rawErr.(error)
	}

	if imageUUID, ok := state.GetOk("image_uuid"); ok {
		if imageUUID != nil {
			artifact := &Artifact{
				Name: b.config.ImageName,
				UUID: imageUUID.([]imageArtefact)[0].uuid,
			}
			return artifact, nil
		}
	}
	return nil, nil
}

// builder.go
func (b *Builder) newDriver(cConfig ClusterConfig) (Driver, error) {
	driver := &NutanixDriver{
		Config:        b.config,
		ClusterConfig: cConfig,
	}

	return driver, nil
}
