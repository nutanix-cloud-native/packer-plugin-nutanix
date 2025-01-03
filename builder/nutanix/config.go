//go:generate packer-sdc mapstructure-to-hcl2 -type Config,Category,ClusterConfig,VmConfig,VmDisk,VmNIC,GPU

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

	// NutanixIdentifierBootPriorityDisk is a resource identifier identifying the boot priority as disk for virtual machines.
	NutanixIdentifierBootPriorityDisk string = "disk"

	// NutanixIdentifierBootPriorityCDROM is a resource identifier identifying the boot priority as cdrom for virtual machines.
	NutanixIdentifierBootPriorityCDROM string = "cdrom"
)

type Config struct {
	common.PackerConfig            `mapstructure:",squash"`
	CommConfig                     communicator.Config `mapstructure:",squash"`
	commonsteps.CDConfig           `mapstructure:",squash"`
	shutdowncommand.ShutdownConfig `mapstructure:",squash"`
	ClusterConfig                  `mapstructure:",squash"`
	VmConfig                       `mapstructure:",squash"`
	ForceDeregister                bool          `mapstructure:"force_deregister" json:"force_deregister" required:"false"`
	ImageDescription               string        `mapstructure:"image_description" json:"image_description" required:"false"`
	ImageCategories                []Category    `mapstructure:"image_categories" required:"false"`
	ImageDelete                    bool          `mapstructure:"image_delete" json:"image_delete" required:"false"`
	ImageExport                    bool          `mapstructure:"image_export" json:"image_export" required:"false"`
	WaitTimeout                    time.Duration `mapstructure:"ip_wait_timeout" json:"ip_wait_timeout" required:"false"`
	VmForceDelete                  bool          `mapstructure:"vm_force_delete" json:"vm_force_delete" required:"false"`

	ctx interpolate.Context
}

type GPU struct {
	Name string `mapstructure:"name" json:"name" required:"false"`
}

type Category struct {
	Key   string `mapstructure:"key" json:"key" required:"false"`
	Value string `mapstructure:"value" json:"value" required:"false"`
}

type ClusterConfig struct {
	Username string `mapstructure:"nutanix_username" required:"false"`
	Password string `mapstructure:"nutanix_password" required:"false"`
	Insecure bool   `mapstructure:"nutanix_insecure" required:"false"`
	Endpoint string `mapstructure:"nutanix_endpoint" required:"true"`
	Port     int32  `mapstructure:"nutanix_port" required:"false"`
}

type VmDisk struct {
	ImageType         string `mapstructure:"image_type" json:"image_type" required:"false"`
	SourceImageName   string `mapstructure:"source_image_name" json:"source_image_name" required:"false"`
	SourceImageUUID   string `mapstructure:"source_image_uuid" json:"source_image_uuid" required:"false"`
	SourceImageURI    string `mapstructure:"source_image_uri" json:"source_image_uri" required:"false"`
	SourceImageDelete bool   `mapstructure:"source_image_delete" json:"source_image_delete" required:"false"`
	SourceImageForce  bool   `mapstructure:"source_image_force" json:"source_image_force" required:"false"`
	DiskSizeGB        int64  `mapstructure:"disk_size_gb" json:"disk_size_gb" required:"false"`
}

type VmNIC struct {
	SubnetName string `mapstructure:"subnet_name" json:"subnet_name" required:"false"`
	SubnetUUID string `mapstructure:"subnet_uuid" json:"subnet_uuid" required:"false"`
}
type VmConfig struct {
	VMName       string     `mapstructure:"vm_name" json:"vm_name" required:"false"`
	OSType       string     `mapstructure:"os_type" json:"os_type" required:"true"`
	BootType     string     `mapstructure:"boot_type" json:"boot_type" required:"false"`
	BootPriority string     `mapstructure:"boot_priority" json:"boot_priority" required:"false"`
	VmDisks      []VmDisk   `mapstructure:"vm_disks"`
	VmNICs       []VmNIC    `mapstructure:"vm_nics"`
	ImageName    string     `mapstructure:"image_name" json:"image_name" required:"false"`
	ClusterUUID  string     `mapstructure:"cluster_uuid" json:"cluster_uuid" required:"false"`
	ClusterName  string     `mapstructure:"cluster_name" json:"cluster_name" required:"false"`
	CPU          int64      `mapstructure:"cpu" json:"cpu" required:"false"`
	MemoryMB     int64      `mapstructure:"memory_mb" json:"memory_mb" required:"false"`
	UserData     string     `mapstructure:"user_data" json:"user_data" required:"false"`
	VMCategories []Category `mapstructure:"vm_categories" required:"false"`
	Project      string     `mapstructure:"project" required:"false"`
	GPU          []GPU      `mapstructure:"gpu" required:"false"`
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

	if c.CommConfig.Type == "" {
		log.Println("No Communicator Type set, setting to 'ssh'")
		c.CommConfig.Type = "ssh"
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

	if c.BootType == NutanixIdentifierBootTypeUEFI && c.BootPriority != "" {
		log.Println("Boot Priority is not supported for UEFI boot type")
		errs = packersdk.MultiErrorAppend(errs, fmt.Errorf("UEFI does not support boot priority"))
	}

	if c.BootPriority != NutanixIdentifierBootPriorityDisk && c.BootPriority != NutanixIdentifierBootPriorityCDROM {
		log.Println("No correct VM Boot Priority configured, defaulting to 'cdrom'")
		c.BootPriority = string(NutanixIdentifierBootPriorityCDROM)
	}

	// Validate Cluster Endpoint
	if c.ClusterConfig.Endpoint == "" {
		log.Println("Nutanix Endpoint missing from configuration")
		errs = packersdk.MultiErrorAppend(errs, fmt.Errorf("missing nutanix_endpoint"))
	}

	// Validate Cluster Name
	if c.VmConfig.ClusterName == "" && c.VmConfig.ClusterUUID == "" {
		log.Println("Nutanix Cluster Name or UUID missing from configuration")
		errs = packersdk.MultiErrorAppend(errs, fmt.Errorf("missing cluster_name or cluster_uuid"))
	}

	// Validate VM disks
	if len(c.VmConfig.VmDisks) == 0 {
		log.Println("Nutanix VM Disks missing from configuration")
		errs = packersdk.MultiErrorAppend(errs, fmt.Errorf("missing vm_disks"))
	}

	if c.CommConfig.Type != "none" {

		// Validate VM nics
		if len(c.VmConfig.VmNICs) == 0 {
			log.Println("Nutanix VM Nics missing from configuration")
			errs = packersdk.MultiErrorAppend(errs, fmt.Errorf("missing vm_nics"))
		}

		// Validate VM Subnet
		for i, nic := range c.VmConfig.VmNICs {
			if nic.SubnetName == "" && nic.SubnetUUID == "" {
				log.Printf("Nutanix Subnet is missing in nic %d configuration", i+1)
				errs = packersdk.MultiErrorAppend(errs, fmt.Errorf("missing subnet in vm_nics %d", i+1))
			}
		}
	}
	// Validate Cluster Username
	if c.ClusterConfig.Username == "" {
		log.Println("Nutanix Username missing from configuration")
		errs = packersdk.MultiErrorAppend(errs, fmt.Errorf("missing nutanix_username"))
	}

	// Validate Cluster Password
	if c.ClusterConfig.Password == "" {
		log.Println("Nutanix Password missing from configuration")
		errs = packersdk.MultiErrorAppend(errs, fmt.Errorf("missing nutanix_password"))
	}

	if c.VmConfig.VMName == "" {
		p := fmt.Sprintf("Packer-%s", random.String(random.PossibleAlphaNumUpper, 8))
		log.Println("No vmname assigned, setting to " + p)

		c.VmConfig.VMName = p
	}

	if c.VmConfig.ImageName == "" {
		log.Println("No image_name assigned, setting to vm_name")

		c.VmConfig.ImageName = c.VmConfig.VMName
	}

	// Validate if both Image Category key and value are given in same time
	for _, imageCategory := range c.ImageCategories {
		if imageCategory.Key != "" && imageCategory.Value == "" {
			log.Println("Nutanix Image Category value missing from configuration")
			errs = packersdk.MultiErrorAppend(errs, fmt.Errorf("missing value for image category %s", imageCategory.Key))
		}

		if imageCategory.Key == "" && imageCategory.Value != "" {
			log.Println("Nutanix Image Category name missing from configuration")
			errs = packersdk.MultiErrorAppend(errs, fmt.Errorf("missing name for an image categories with value %s", imageCategory.Value))
		}
	}

	// Validate if both VM Category key and value are given in same time
	for _, vmCategory := range c.VMCategories {
		if vmCategory.Key != "" && vmCategory.Value == "" {
			log.Println("Nutanix VM Category value missing from configuration")
			errs = packersdk.MultiErrorAppend(errs, fmt.Errorf("missing value for vm category %s", vmCategory.Key))
		}

		if vmCategory.Key == "" && vmCategory.Value != "" {
			log.Println("Nutanix VM Category name missing from configuration")
			errs = packersdk.MultiErrorAppend(errs, fmt.Errorf("missing name for a vm categories with value %s", vmCategory.Value))
		}
	}

	if c.CommConfig.SSHPort == 0 {
		log.Println("SSHPort not set, defaulting to 22")
		c.CommConfig.SSHPort = 22
	}

	if c.CommConfig.SSHTimeout == 0 {
		log.Println("SSHTimeout not set, defaulting to 20min")
		c.CommConfig.SSHTimeout = 20 * time.Minute
	}

	// Define default ip_wait_timeout to 15 min
	if c.WaitTimeout == 0 {
		c.WaitTimeout = 15 * time.Minute
	}

	errs = packersdk.MultiErrorAppend(errs, c.ShutdownConfig.Prepare(&c.ctx)...)
	errs = packersdk.MultiErrorAppend(errs, c.CDConfig.Prepare(&c.ctx)...)
	errs = packersdk.MultiErrorAppend(errs, c.CommConfig.Prepare(&c.ctx)...)

	if errs != nil && len(errs.Errors) > 0 {
		return warnings, errs
	}

	return warnings, nil
}
