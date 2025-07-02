//go:generate packer-sdc mapstructure-to-hcl2 -type Config,Category,ClusterConfig,VmConfig,VmDisk,VmNIC,GPU,OvaConfig,TemplateConfig,VTPM,VmClean

package nutanix

import (
	//"errors"
	"fmt"
	"log"
	"time"

	"github.com/hashicorp/packer-plugin-sdk/bootcommand"
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

	// NutanixIdentifierBootTypeSecureBoot is a resource identifier identifying the secure boot type for virtual machines.
	NutanixIdentifierBootTypeSecureBoot string = "secure_boot"

	// NutanixIdentifierBootPriorityDisk is a resource identifier identifying the boot priority as disk for virtual machines.
	NutanixIdentifierBootPriorityDisk string = "disk"

	// NutanixIdentifierBootPriorityCDROM is a resource identifier identifying the boot priority as cdrom for virtual machines.
	NutanixIdentifierBootPriorityCDROM string = "cdrom"

	// NutanixIdentifierChecksunTypeSHA256 is a resource identifier identifying the SHA-256 checksum type for virtual machines.
	NutanixIdentifierChecksunTypeSHA256 string = "sha256"

	// NutanixIdentifierChecksunTypeSHA1 is a resource identifier identifying the SHA-1 checksum type for virtual machines.
	NutanixIdentifierChecksunTypeSHA1 string = "sha1"
)

type Config struct {
	common.PackerConfig            `mapstructure:",squash"`
	WaitIpConfig                   `mapstructure:",squash"`
	Comm                           communicator.Config `mapstructure:",squash"`
	bootcommand.VNCConfig          `mapstructure:",squash"`
	commonsteps.CDConfig           `mapstructure:",squash"`
	shutdowncommand.ShutdownConfig `mapstructure:",squash"`
	ClusterConfig                  `mapstructure:",squash"`
	VmConfig                       `mapstructure:",squash"`
	OvaConfig                      OvaConfig      `mapstructure:"ova" required:"false"`
	TemplateConfig                 TemplateConfig `mapstructure:"template" required:"false"`
	ForceDeregister                bool           `mapstructure:"force_deregister" json:"force_deregister" required:"false"`
	ImageDescription               string         `mapstructure:"image_description" json:"image_description" required:"false"`
	ImageCategories                []Category     `mapstructure:"image_categories" required:"false"`
	ImageSkip                      bool           `mapstructure:"image_skip" json:"image_skip" required:"false"`
	ImageDelete                    bool           `mapstructure:"image_delete" json:"image_delete" required:"false"`
	ImageExport                    bool           `mapstructure:"image_export" json:"image_export" required:"false"`
	VmForceDelete                  bool           `mapstructure:"vm_force_delete" json:"vm_force_delete" required:"false"`
	VmRetain                       bool           `mapstructure:"vm_retain" json:"vm_retain" required:"false"`
	DisableStopInstance            bool           `mapstructure:"disable_stop_instance" required:"false"`

	ctx interpolate.Context
}

type GPU struct {
	Name string `mapstructure:"name" json:"name" required:"false"`
}

type VTPM struct {
	Enabled bool `mapstructure:"enabled" json:"enabled" required:"false"`
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
	ImageType               string `mapstructure:"image_type" json:"image_type" required:"false"`
	SourceImageName         string `mapstructure:"source_image_name" json:"source_image_name" required:"false"`
	SourceImageUUID         string `mapstructure:"source_image_uuid" json:"source_image_uuid" required:"false"`
	SourceImageURI          string `mapstructure:"source_image_uri" json:"source_image_uri" required:"false"`
	SourceImagePath         string `mapstructure:"source_image_path" json:"source_image_path" required:"false"`
	SourceImageChecksum     string `mapstructure:"source_image_checksum" json:"source_image_checksum" required:"false"`
	SourceImageChecksumType string `mapstructure:"source_image_checksum_type" json:"source_image_checksum_type" required:"false"`
	SourceImageDelete       bool   `mapstructure:"source_image_delete" json:"source_image_delete" required:"false"`
	SourceImageForce        bool   `mapstructure:"source_image_force" json:"source_image_force" required:"false"`
	DiskSizeGB              int64  `mapstructure:"disk_size_gb" json:"disk_size_gb" required:"false"`
	StorageContainerUUID    string `mapstructure:"storage_container_uuid" json:"storage_container_uuid" required:"false"`
}

type VmNIC struct {
	SubnetName string `mapstructure:"subnet_name" json:"subnet_name" required:"false"`
	SubnetUUID string `mapstructure:"subnet_uuid" json:"subnet_uuid" required:"false"`
	MacAddress string `mapstructure:"mac_address" json:"mac_address" required:"false"`
}
type VmConfig struct {
	VMName                 string     `mapstructure:"vm_name" json:"vm_name" required:"false"`
	OSType                 string     `mapstructure:"os_type" json:"os_type" required:"true"`
	BootType               string     `mapstructure:"boot_type" json:"boot_type" required:"false"`
	VTPM                   VTPM       `mapstructure:"vtpm" json:"vtpm" required:"false"`
	HardwareVirtualization bool       `mapstructure:"hardware_virtualization" json:"hardware_virtualization" required:"false"`
	BootPriority           string     `mapstructure:"boot_priority" json:"boot_priority" required:"false"`
	VmDisks                []VmDisk   `mapstructure:"vm_disks"`
	VmNICs                 []VmNIC    `mapstructure:"vm_nics"`
	ImageName              string     `mapstructure:"image_name" json:"image_name" required:"false"`
	ClusterUUID            string     `mapstructure:"cluster_uuid" json:"cluster_uuid" required:"false"`
	ClusterName            string     `mapstructure:"cluster_name" json:"cluster_name" required:"false"`
	CPU                    int64      `mapstructure:"cpu" json:"cpu" required:"false"`
	Core                   int64      `mapstructure:"core" json:"core" required:"false"`
	MemoryMB               int64      `mapstructure:"memory_mb" json:"memory_mb" required:"false"`
	UserData               string     `mapstructure:"user_data" json:"user_data" required:"false"`
	VMCategories           []Category `mapstructure:"vm_categories" required:"false"`
	Project                string     `mapstructure:"project" required:"false"`
	GPU                    []GPU      `mapstructure:"gpu" required:"false"`
	SerialPort             bool       `mapstructure:"serialport" json:"serialport" required:"false"`
	Clean                  VmClean    `mapstructure:"vm_clean" json:"vm_clean" required:"false"`
}

type VmClean struct {
	Cdrom bool `mapstructure:"cdrom" json:"cdrom" required:"false"`
}

type OvaConfig struct {
	Export bool   `mapstructure:"export" json:"export" required:"false"`
	Create bool   `mapstructure:"create" json:"create" required:"false"`
	Format string `mapstructure:"format" json:"format" required:"false"`
	Name   string `mapstructure:"name" json:"name" required:"false"`
}

type TemplateConfig struct {
	Create      bool   `mapstructure:"create" json:"create" required:"false"`
	Name        string `mapstructure:"name" json:"name" required:"false"`
	Description string `mapstructure:"description" json:"description" required:"false"`
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

	// Set Default Communicator Type
	if c.Comm.Type == "" {
		log.Println("No Communicator Type set, setting to 'ssh'")
		c.Comm.Type = "ssh"
	}

	// Set Default CPU Configuration
	if c.CPU == 0 {
		log.Println("No CPU configured, defaulting to '1'")
		c.CPU = 1
	}

	// Set Default Core Configuration
	if c.Core == 0 {
		log.Println("No Core configured, defaulting to '1'")
		c.Core = 1
	}

	// Set Default Memory Configuration
	if c.MemoryMB == 0 {
		log.Println("No VM Memory configured, defaulting to '4096'")
		c.MemoryMB = 4096
	}

	// Set Default Nutanix Port
	if c.ClusterConfig.Port == 0 {
		log.Println("No Nutanix Port configured, defaulting to '9440'")
		c.ClusterConfig.Port = 9440
	}

	// Validate Boot Type
	if c.BootType != NutanixIdentifierBootTypeLegacy && c.BootType != NutanixIdentifierBootTypeUEFI && c.BootType != NutanixIdentifierBootTypeSecureBoot {
		log.Println("No correct VM Boot Type configured, defaulting to 'legacy'")
		c.BootType = string(NutanixIdentifierBootTypeLegacy)
	}

	// Validate VTPM is not used with legacy boot type
	if c.VTPM.Enabled && c.BootType == NutanixIdentifierBootTypeLegacy {
		log.Println("vTPM is not supported with legacy boot type")
		errs = packersdk.MultiErrorAppend(errs, fmt.Errorf("vTPM is not supported with legacy boot type"))
	}

	// Validate Boot Priority
	if c.BootPriority != NutanixIdentifierBootPriorityDisk && c.BootPriority != NutanixIdentifierBootPriorityCDROM {
		log.Println("No correct VM Boot Priority configured, defaulting to 'cdrom'")
		c.BootPriority = string(NutanixIdentifierBootPriorityCDROM)
	}

	// Validate Cluster Endpoint
	if c.ClusterConfig.Endpoint == "" {
		log.Println("Nutanix Endpoint missing from configuration")
		errs = packersdk.MultiErrorAppend(errs, fmt.Errorf("missing nutanix_endpoint"))
	}

	// When trying to export OVA, it should always be created
	if c.OvaConfig.Export && !c.OvaConfig.Create {
		log.Println("Setting ova.create to 'true', because ova.export is 'true'")
		c.OvaConfig.Create = true
	}

	// When trying to export image, it should always be created
	if c.ImageSkip && c.ImageExport {
		log.Println("Setting image_skip to 'false' and image_delete to 'true', because image_export is 'true'")
		c.ImageSkip = false
		c.ImageDelete = true
	}

	// Set OVA format if not provided
	if c.OvaConfig.Create && c.OvaConfig.Format == "" {
		c.OvaConfig.Format = "vmdk"
	}

	// OvaConfig format should be vmdk or qcow2
	if c.OvaConfig.Create && c.OvaConfig.Format != "vmdk" && c.OvaConfig.Format != "qcow2" {
		log.Println("Incorrect ova format")
		errs = packersdk.MultiErrorAppend(errs, fmt.Errorf("ova.format should be 'vmdk' or 'qcow2'"))
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

	if c.Comm.Type != "none" {

		// Validate VM nics
		if len(c.VmConfig.VmNICs) == 0 {
			log.Println("Nutanix VM Nics missing from configuration")
			errs = packersdk.MultiErrorAppend(errs, fmt.Errorf("missing vm_nics"))
		}
	}

	// Validate VM Subnet
	for i, nic := range c.VmConfig.VmNICs {
		if nic.SubnetName == "" && nic.SubnetUUID == "" {
			log.Printf("Nutanix Subnet is missing in nic %d configuration", i+1)
			errs = packersdk.MultiErrorAppend(errs, fmt.Errorf("missing subnet in vm_nics %d", i+1))
		}

		if nic.MacAddress != "" && !isValidMACAddress(nic.MacAddress) {
			log.Printf("Mac address is invalid for nic %d", i+1)
			errs = packersdk.MultiErrorAppend(errs, fmt.Errorf("mac address is invalid in vm_nics %d", i+1))
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

	// Set name for OVA if not provided
	if c.OvaConfig.Create && c.OvaConfig.Name == "" {
		log.Println("No ova.name defined, setting to vm_name")
		c.OvaConfig.Name = c.VmConfig.VMName
	}

	if c.VmConfig.ImageName == "" {
		log.Println("No image_name defined, setting to vm_name")

		c.VmConfig.ImageName = c.VmConfig.VMName
	}

	// Set name for Template if not provided
	if c.TemplateConfig.Create && c.TemplateConfig.Name == "" {
		log.Println("No template.name defined, setting to vm_name")
		c.TemplateConfig.Name = c.VmConfig.VMName
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

	// Validate each disk
	for index, disk := range c.VmConfig.VmDisks {

		// Validate checksum only with uri
		if disk.SourceImageChecksum != "" && disk.SourceImageURI == "" {
			log.Printf("disk %d: Checksum work only with Source Image URI\n", index)
			errs = packersdk.MultiErrorAppend(errs, fmt.Errorf("disk %d: source_image_checksum work only with source_image_uri", index))
		}

		// Validate supported checksum type
		if disk.SourceImageChecksumType != "" && disk.SourceImageChecksumType != NutanixIdentifierChecksunTypeSHA1 && disk.SourceImageChecksumType != NutanixIdentifierChecksunTypeSHA256 {
			log.Printf("disk %d: Checksum type %s not supported\n", index, disk.SourceImageChecksumType)
			errs = packersdk.MultiErrorAppend(errs, fmt.Errorf("disk %d: checksum_type %s not supported", index, disk.SourceImageChecksumType))
		}

		// Validate Checksum type always defined with checksum
		if disk.SourceImageChecksum != "" && disk.SourceImageChecksumType == "" {
			log.Printf("disk %d: Checksum type need to be defined\n", index)
			errs = packersdk.MultiErrorAppend(errs, fmt.Errorf("disk %d: source_image_checksum_type need to be defined", index))
		}

		// Validate Checksum type is never alone
		if disk.SourceImageChecksumType != "" && disk.SourceImageChecksum == "" {
			log.Printf("disk %d: No checksum set despite checksum type configure\n", index)
			errs = packersdk.MultiErrorAppend(errs, fmt.Errorf("disk %d: no source_image_checksum set despite checksum_type configured", index))
		}

		// Validate storage container defined only with disk type
		if (disk.StorageContainerUUID != "") && disk.ImageType != "DISK" {
			log.Printf("disk %d: Storage container UUID can be set only with DISK image type\n", index)
			errs = packersdk.MultiErrorAppend(errs, fmt.Errorf("disk %d: storage_container_uuid can be set only with DISK image type", index))
		}

		// Validate if file to upload exists
		if disk.SourceImagePath != "" {
			log.Printf("Checking if file exists: %s\n", disk.SourceImagePath)
			if !fileExists(disk.SourceImagePath) {
				log.Printf("disk %d: Source image  %s does not exist\n", index, disk.SourceImagePath)
				errs = packersdk.MultiErrorAppend(errs, fmt.Errorf("disk %d: Source image %s does not exist", index, disk.SourceImagePath))
			}
		}

		// Validate if delete is used only with path or URI
		if disk.SourceImageDelete && disk.SourceImagePath == "" && disk.SourceImageURI == "" {
			log.Printf("disk %d: Source image delete can be used only with source_image_path or source_image_uri", index)
			errs = packersdk.MultiErrorAppend(errs, fmt.Errorf("disk %d: source_image_delete can be used only with source_image_path or source_image_uri", index))
		}
	}

	if c.Comm.SSHPort == 0 {
		log.Println("SSHPort not set, defaulting to 22")
		c.Comm.SSHPort = 22
	}

	if c.Comm.SSHTimeout == 0 {
		log.Println("SSHTimeout not set, defaulting to 20min")
		c.Comm.SSHTimeout = 20 * time.Minute
	}

	errs = packersdk.MultiErrorAppend(errs, c.ShutdownConfig.Prepare(&c.ctx)...)
	errs = packersdk.MultiErrorAppend(errs, c.CDConfig.Prepare(&c.ctx)...)
	errs = packersdk.MultiErrorAppend(errs, c.Comm.Prepare(&c.ctx)...)
	errs = packersdk.MultiErrorAppend(errs, c.WaitIpConfig.Prepare()...)

	if errs != nil && len(errs.Errors) > 0 {
		return warnings, errs
	}

	return warnings, nil
}
