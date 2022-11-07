//go:generate packer-sdc mapstructure-to-hcl2 -type Config,ClusterConfig,VmConfig,VmDisk,VmNIC

package nutanix

import (
	//"errors"
	"fmt"
	"log"
	"time"

	"github.com/hashicorp/packer-plugin-sdk/common"
	"github.com/hashicorp/packer-plugin-sdk/communicator"

	"github.com/hashicorp/packer-plugin-sdk/multistep/commonsteps"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/hashicorp/packer-plugin-sdk/random"
	"github.com/hashicorp/packer-plugin-sdk/shutdowncommand"
	"github.com/hashicorp/packer-plugin-sdk/template/config"
	"github.com/hashicorp/packer-plugin-sdk/template/interpolate"
)

const (
	// NutanixIdentifierBootTypeLegacy is a resource identifier identifying the legacy boot type for virtual machines.
	NutanixIdentifierBootTypeLegacy string = "legacy"

	// NutanixIdentifierBootTypeUEFI is a resource identifier identifying the UEFI boot type for virtual machines.
	NutanixIdentifierBootTypeUEFI string = "uefi"
)

type Config struct {
	common.PackerConfig            `mapstructure:",squash"`
	CommConfig communicator.Config `mapstructure:",squash"`
	commonsteps.CDConfig           `mapstructure:",squash"`
	shutdowncommand.ShutdownConfig `mapstructure:",squash"`
	ClusterConfig                  `mapstructure:",squash"`
	VmConfig                       `mapstructure:",squash"`
	ForceDeregister         bool   `mapstructure:"force_deregister" json:"force_deregister" required:"false"`

	

	ctx interpolate.Context
}

type ClusterConfig struct {
	Username string `mapstructure:"nutanix_username" required:"false"`
	Password string `mapstructure:"nutanix_password" required:"false"`
	Insecure bool   `mapstructure:"nutanix_insecure" required:"false"`
	Endpoint string `mapstructure:"nutanix_endpoint" required:"true"`
	Port     int32  `mapstructure:"nutanix_port" required:"false"`
}

type VmDisk struct {
	ImageType       string `mapstructure:"image_type" json:"image_type" required:"false"`
	SourceImageName string `mapstructure:"source_image_name" json:"source_image_name" required:"false"`
	SourceImageUUID string `mapstructure:"source_image_uuid" json:"source_image_uuid" required:"false"`
	SourceImageURI string `mapstructure:"source_image_uri" json:"source_image_uri" required:"false"`
	DiskSizeGB      int64  `mapstructure:"disk_size_gb" json:"disk_size_gb" required:"false"`
}

type VmNIC struct {
	SubnetName string `mapstructure:"subnet_name" json:"subnet_name" required:"false"`
	SubnetUUID string `mapstructure:"subnet_uuid" json:"subnet_uuid" required:"false"`
}
type VmConfig struct {
	VMName      string   `mapstructure:"vm_name" json:"vm_name" required:"false"`
	OSType      string   `mapstructure:"os_type" json:"os_type" required:"true"`
	BootType    string   `mapstructure:"boot_type" json:"boot_type" required:"false"`
	VmDisks     []VmDisk `mapstructure:"vm_disks"`
	VmNICs      []VmNIC  `mapstructure:"vm_nics"`
	ImageName   string   `mapstructure:"image_name" json:"image_name" required:"false"`
	ClusterUUID string   `mapstructure:"cluster_uuid" json:"cluster_uuid" required:"false"`
	ClusterName string   `mapstructure:"cluster_name" json:"cluster_name" required:"false"`
	CPU         int64    `mapstructure:"cpu" json:"cpu" required:"false"`
	MemoryMB    int64    `mapstructure:"memory_mb" json:"memory_mb" required:"false"`
	UserData    string   `mapstructure:"user_data" json:"user_data" required:"false"`
}

func (c *Config) Prepare(raws ...interface{}) ([]string, error) {
	err := config.Decode(c, &config.DecodeOpts{
		PluginType:         BuilderId,
		Interpolate:        true,
		InterpolateContext: &c.ctx,
		InterpolateFilter: &interpolate.RenderFilter{
			Exclude: []string{
				"boot_command",
			},
		},
	}, raws...)
	if err != nil {
		return nil, err
	}

	// Accumulate any errors and warnings
	var errs *packersdk.MultiError
	warnings := make([]string, 0)

	// if c.VmConfig.DiskSizeGB == 0 {
	// 	c.VmConfig.DiskSizeGB = 40
	// }

	if c.CommConfig.Type == "" {
		if c.CommConfig.SSHUsername != "" {
			log.Println("No Communicator Type assigned but SSH Creds available, setting to 'SSH'")
			c.CommConfig.Type = "ssh"
		} else {
			log.Println("No Communicator Type set, setting to 'none'")
			c.CommConfig.Type = "none"
		}
	}

	if c.CPU == 0 {
		log.Println("No CPU configured, defaulting to '1'")
		c.CPU = 1
	}

	if c.MemoryMB == 0 {
		log.Println("No VM Memory configured, defaulting to '4096'")
		c.MemoryMB = 4096
	}
	if c.ClusterConfig.Port == 0 {
		log.Println("No Nutanix Port configured, defaulting to '9440'")
		c.ClusterConfig.Port = 9440
	}

	if c.BootType != NutanixIdentifierBootTypeLegacy && c.BootType != NutanixIdentifierBootTypeUEFI {
		log.Println("No correct VM Boot Type configured, defaulting to 'legacy'")
		c.BootType = string(NutanixIdentifierBootTypeLegacy)
	}

	// Validate Cluster Username
	if c.ClusterConfig.Username == "" {
		log.Println("Nutanix Username missing from configuration")
		errs = packersdk.MultiErrorAppend(errs, fmt.Errorf("missing nutanix_username"))
	}

	// if c.VmConfig.SourceImageName == "" {
	// 	errs = packersdk.MultiErrorAppend(errs, errors.New("Missing source_image_name"))
	// }

	if c.VmConfig.VMName == "" {
		p := fmt.Sprintf("Packer-%s", random.String(random.PossibleAlphaNumUpper, 8))
		log.Println("No vmname assigned, setting to " + p)

		c.VmConfig.VMName = p
	}

	if c.VmConfig.ImageName == "" {
		log.Println("No image_name assigned, setting to vm_name")

		c.VmConfig.ImageName = c.VmConfig.VMName
	}

	if c.CommConfig.SSHPort == 0 {
		log.Println("SSHPort not set, defaulting to 22")
		c.CommConfig.SSHPort = 22
	}

	if c.CommConfig.SSHTimeout == 0 {
		log.Println("SSHTimeout not set, defaulting to 20min")
		c.CommConfig.SSHTimeout = 20 * time.Minute
	}

	errs = packersdk.MultiErrorAppend(errs, c.ShutdownConfig.Prepare(&c.ctx)...)
	errs = packersdk.MultiErrorAppend(errs, c.CDConfig.Prepare(&c.ctx)...)
	errs = packersdk.MultiErrorAppend(errs, c.CommConfig.Prepare(&c.ctx)...)

	if errs != nil && len(errs.Errors) > 0 {
		return warnings, errs
	}

	return warnings, nil
}
