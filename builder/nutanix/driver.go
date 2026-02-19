package nutanix

import (
	"context"
	"fmt"
	"log"
	"net"
	"path"
	"sort"
	"strings"
	"time"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	client "github.com/nutanix-cloud-native/prism-go-client"
	"github.com/nutanix-cloud-native/prism-go-client/converged"
	convergedv4 "github.com/nutanix-cloud-native/prism-go-client/converged/v4"
	"github.com/nutanix-cloud-native/prism-go-client/environment/types"
	v3 "github.com/nutanix-cloud-native/prism-go-client/v3"
	v4 "github.com/nutanix-cloud-native/prism-go-client/v4"
	clusterModels "github.com/nutanix/ntnx-api-golang-clients/clustermgmt-go-client/v4/models/clustermgmt/v4/config"
	prismConfig "github.com/nutanix/ntnx-api-golang-clients/prism-go-client/v4/models/prism/v4/config"
	vmmPrismConfig "github.com/nutanix/ntnx-api-golang-clients/vmm-go-client/v4/models/prism/v4/config"
	vmmModels "github.com/nutanix/ntnx-api-golang-clients/vmm-go-client/v4/models/vmm/v4/ahv/config"
	imageModels "github.com/nutanix/ntnx-api-golang-clients/vmm-go-client/v4/models/vmm/v4/content"
	vmmError "github.com/nutanix/ntnx-api-golang-clients/vmm-go-client/v4/models/vmm/v4/error"
)

const (
	defaultImageBuiltDescription = "built by Packer"
	defaultImageDLDescription    = "added by Packer"
	vmDescription                = "Packer vm building image %s"
)

const (
	bytesPerMB = 1024 * 1024
	bytesPerGB = 1024 * 1024 * 1024
)

// Driver is able to talk to Nutanix PrismCentral and perform certain
// operations with it.
type Driver interface {
	CreateRequest(context.Context, VmConfig, multistep.StateBag) (*vmmModels.Vm, error)
	Create(context.Context, *vmmModels.Vm) (*nutanixInstance, error)
	UpdateVM(context.Context, string, *vmmModels.Vm) (*nutanixInstance, error)
	Delete(context.Context, string) error
	GetVM(context.Context, string) (*nutanixInstance, error)
	GetHost(context.Context, string) (*nutanixHost, error)
	PowerOff(context.Context, string) error
	CreateImageURL(context.Context, VmDisk, VmConfig) (*nutanixImage, error)
	CreateImageFile(context.Context, string, VmConfig) (*nutanixImage, error)
	DeleteImage(context.Context, string) error
	GetImage(context.Context, string) (*nutanixImage, error)
	CreateTemplate(context.Context, string, TemplateConfig) error
	CreateOVA(context.Context, string, string, string) error
	ExportOVA(context.Context, string) (string, error)
	ExportImage(context.Context, string) (string, error)
	SaveVMDisk(context.Context, string, int, []Category) (*nutanixImage, error)
	WaitForShutdown(string, <-chan struct{}) bool
	CleanCD(context.Context, string) error
	PowerOn(context.Context, string) error
	GenerateConsoleToken(context.Context, string) (token, wsUri string, err error)
}

// Verify that NutanixDriver implements the Driver interface
var _ Driver = &NutanixDriver{}

// NutanixDriver is a driver for Nutanix PrismCentral calls
type NutanixDriver struct {
	Config        Config
	ClusterConfig ClusterConfig
	vmEndCh       <-chan int
	v4Client      *convergedv4.Client
	v4SDKClient   *v4.Client
}

type nutanixInstance struct {
	vm *vmmModels.Vm
}

// UUID returns the VM's external ID (UUID)
func (n *nutanixInstance) UUID() string {
	if n.vm != nil && n.vm.ExtId != nil {
		return *n.vm.ExtId
	}
	return ""
}

// ClusterUUID returns the VM's cluster UUID
func (n *nutanixInstance) ClusterUUID() string {
	if n.vm != nil && n.vm.Cluster != nil && n.vm.Cluster.ExtId != nil {
		return *n.vm.Cluster.ExtId
	}
	return ""
}

// PowerState returns the VM's power state as a string
func (n *nutanixInstance) PowerState() string {
	if n.vm != nil && n.vm.PowerState != nil {
		return n.vm.PowerState.GetName()
	}
	return ""
}

// Addresses returns all IP addresses assigned to the VM
func (n *nutanixInstance) Addresses() []string {
	var addresses []string
	if n.vm == nil || n.vm.Nics == nil {
		return addresses
	}
	for _, nic := range n.vm.Nics {
		if nic.NetworkInfo != nil && nic.NetworkInfo.Ipv4Info != nil {
			for _, ipConfig := range nic.NetworkInfo.Ipv4Info.LearnedIpAddresses {
				if ipConfig.Value != nil {
					addresses = append(addresses, *ipConfig.Value)
				}
			}
		}
	}
	return addresses
}

// VM returns the underlying V4 Vm for direct access
func (n *nutanixInstance) VM() *vmmModels.Vm {
	return n.vm
}

// Disks returns all disks attached to the VM (for creating images)
func (n *nutanixInstance) Disks() []vmmModels.Disk {
	if n.vm == nil || n.vm.Disks == nil {
		return nil
	}
	return n.vm.Disks
}

type nutanixHost struct {
	host *clusterModels.Host
}

// UUID returns the host's external ID (UUID)
func (n *nutanixHost) UUID() string {
	if n.host != nil && n.host.ExtId != nil {
		return *n.host.ExtId
	}
	return ""
}

// Name returns the host's name
func (n *nutanixHost) Name() string {
	if n.host != nil && n.host.HostName != nil {
		return *n.host.HostName
	}
	return ""
}

// ClusterUUID returns the host's cluster UUID
func (n *nutanixHost) ClusterUUID() string {
	if n.host != nil && n.host.Cluster != nil && n.host.Cluster.Uuid != nil {
		return *n.host.Cluster.Uuid
	}
	return ""
}

type nutanixImage struct {
	image *imageModels.Image // V4 native type
}

// UUID returns the image's external ID (UUID)
func (n *nutanixImage) UUID() string {
	if n.image != nil && n.image.ExtId != nil {
		return *n.image.ExtId
	}
	return ""
}

// Name returns the image's name
func (n *nutanixImage) Name() string {
	if n.image != nil && n.image.Name != nil {
		return *n.image.Name
	}
	return ""
}

// SizeBytes returns the image's size in bytes
func (n *nutanixImage) SizeBytes() int64 {
	if n.image != nil && n.image.SizeBytes != nil {
		return *n.image.SizeBytes
	}
	return 0
}

// getConfigCreds returns the credentials for connecting to Prism Central
func (d *NutanixDriver) getConfigCreds() client.Credentials {
	return client.Credentials{
		URL:      fmt.Sprintf("%s:%d", d.ClusterConfig.Endpoint, d.ClusterConfig.Port),
		Endpoint: d.ClusterConfig.Endpoint,
		Username: d.ClusterConfig.Username,
		Password: d.ClusterConfig.Password,
		Port:     string(d.ClusterConfig.Port),
		Insecure: d.ClusterConfig.Insecure,
	}
}

// getV4Client returns the V4 converged client, creating it if needed
func (d *NutanixDriver) getV4Client() (*convergedv4.Client, error) {
	if d.v4Client != nil {
		return d.v4Client, nil
	}

	opts := []types.ClientOption[v4.Client]{}
	if d.ClusterConfig.ReadTimeout > 0 {
		opts = append(opts, v4.WithReadTimeout(d.ClusterConfig.ReadTimeout))
	}

	v4Client, err := convergedv4.NewClient(d.getConfigCreds(), opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create V4 client: %s", err.Error())
	}

	d.v4Client = v4Client
	return d.v4Client, nil
}

func findProjectByName(ctx context.Context, conn *v3.Client, name string) (*v3.Project, error) {
	resp, err := conn.V3.ListAllProject(ctx, "")
	if err != nil {
		return nil, err
	}
	entities := resp.Entities

	found := make([]*v3.Project, 0)
	for _, v := range entities {
		if strings.EqualFold(v.Status.Name, name) {
			found = append(found, &v3.Project{
				Status:     v.Status,
				Spec:       v.Spec,
				Metadata:   v.Metadata,
				APIVersion: v.APIVersion,
			})
		}
	}

	if len(found) > 1 {
		return nil, fmt.Errorf("your query returned more than one result")
	}

	if len(found) == 0 {
		return nil, fmt.Errorf("did not find project with name %s", name)
	}

	return found[0], nil
}

// sourceImageExists checks if an image with the given name exists using V4 API.
// It verifies images are ready (SizeBytes > 0) and optionally validates the checksum
// to detect corrupt/partial images. Matching priority: name+URL > name+checksum > name-only.
func sourceImageExists(ctx context.Context, v4Client *convergedv4.Client, name, uri, expectedChecksum string) (*imageModels.Image, error) {
	images, err := v4Client.Images.List(ctx, converged.WithFilter(fmt.Sprintf("name eq '%s'", name)))
	if err != nil {
		return nil, err
	}

	var urlMatched []*imageModels.Image
	var nameMatched []*imageModels.Image

	for i := range images {
		img := &images[i]
		if img.Name == nil || !strings.EqualFold(*img.Name, name) {
			continue
		}

		// Only consider images that are ready (SizeBytes > 0)
		if img.SizeBytes == nil || *img.SizeBytes <= 0 {
			extId := ""
			if img.ExtId != nil {
				extId = *img.ExtId
			}
			log.Printf("skipping image '%s' (extId: %s) - not ready (SizeBytes: %v)", name, extId, img.SizeBytes)
			continue
		}

		// Verify checksum if both expected and actual are available
		if expectedChecksum != "" && img.Checksum != nil {
			if checksumValue := img.Checksum.GetValue(); checksumValue != nil {
				actualDigest := ""
				switch cs := checksumValue.(type) {
				case imageModels.ImageSha256Checksum:
					if cs.HexDigest != nil {
						actualDigest = *cs.HexDigest
					}
				case imageModels.ImageSha1Checksum:
					if cs.HexDigest != nil {
						actualDigest = *cs.HexDigest
					}
				}
				if actualDigest != "" && !strings.EqualFold(actualDigest, expectedChecksum) {
					log.Printf("skipping image '%s' (extId: %s) - checksum mismatch (expected: %s, actual: %s)",
						name, *img.ExtId, expectedChecksum, actualDigest)
					continue
				}
			}
		}

		nameMatched = append(nameMatched, img)
		if img.Source != nil {
			if sourceValue := img.Source.GetValue(); sourceValue != nil {
				if urlSource, ok := sourceValue.(imageModels.UrlSource); ok && urlSource.Url != nil {
					if strings.EqualFold(*urlSource.Url, uri) {
						urlMatched = append(urlMatched, img)
					}
				}
			}
		}
	}

	// Prefer exact URL match
	if len(urlMatched) == 1 {
		return urlMatched[0], nil
	}
	if len(urlMatched) > 1 {
		return nil, fmt.Errorf("your query returned more than one result with same Name/URI")
	}

	// Fall back to name-only match (V4 API may not return Source for downloaded images)
	if len(nameMatched) == 1 {
		log.Printf("image '%s' found by name (source URL not available in API response)", name)
		return nameMatched[0], nil
	}
	if len(nameMatched) > 1 {
		return nil, fmt.Errorf("your query returned more than one result with name '%s'", name)
	}

	return nil, nil
}

// findImageByUUID finds an image by UUID using V4 API
func findImageByUUID(ctx context.Context, v4Client *convergedv4.Client, uuid string) (*nutanixImage, error) {
	img, err := findImageByUUIDHelper(ctx, v4Client, uuid)
	if err != nil {
		return nil, err
	}
	return &nutanixImage{image: img}, nil
}

// findImageByName finds an image by name using V4 API
func findImageByName(ctx context.Context, v4Client *convergedv4.Client, name string) (*nutanixImage, error) {
	img, err := findImageByNameHelper(ctx, v4Client, name)
	if err != nil {
		return nil, err
	}
	return &nutanixImage{image: img}, nil
}

func (d *NutanixDriver) WaitForShutdown(vmUUID string, cancelCh <-chan struct{}) bool {
	endCh := d.vmEndCh

	if endCh == nil {
		return true
	}

	select {
	case <-endCh:
		return true
	case <-cancelCh:
		return false
	}
}

func (d *NutanixDriver) CreateRequest(ctx context.Context, vmConfig VmConfig, state multistep.StateBag) (*vmmModels.Vm, error) {
	v4Client, err := d.getV4Client()
	if err != nil {
		return nil, fmt.Errorf("error creating V4 client: %s", err.Error())
	}

	// V3 client needed for projects (no V4 Projects API yet)
	configCreds := d.getConfigCreds()
	conn, err := v3.NewV3Client(configCreds)
	if err != nil {
		return nil, err
	}

	log.Printf("preparing vm %s...", d.Config.VMName)

	clusterUUID, err := getClusterUUID(ctx, v4Client, vmConfig.ClusterName, vmConfig.ClusterUUID)
	if err != nil {
		return nil, fmt.Errorf("error while getting cluster: %s", err.Error())
	}

	v4vm := vmmModels.NewVm()
	v4vm.Name = &vmConfig.VMName
	v4vm.Description = StringPtr(fmt.Sprintf(vmDescription, d.Config.VmConfig.ImageName))

	v4vm.Cluster = vmmModels.NewClusterReference()
	v4vm.Cluster.ExtId = &clusterUUID

	numSockets := int(vmConfig.CPU)
	numCoresPerSocket := int(vmConfig.Core)
	memorySizeBytes := vmConfig.MemoryMB * bytesPerMB
	v4vm.NumSockets = &numSockets
	v4vm.NumCoresPerSocket = &numCoresPerSocket
	v4vm.MemorySizeBytes = &memorySizeBytes

	// Check if we have CdRoms (ISO_IMAGE) to set boot order accordingly.
	hasCdRoms := false
	for _, disk := range vmConfig.VmDisks {
		if disk.ImageType == "ISO_IMAGE" {
			hasCdRoms = true
			break
		}
	}

	// Build boot order based on CdRom presence and boot_priority.
	// CdRoms are added post-creation but BEFORE power-on, so including CDROM in
	// boot order during creation is safe when CdRoms will be attached.
	// Only include CDROM in boot order when CdRom devices exist - some Nutanix
	// versions return INTERNAL_ERROR when CDROM is in boot order without CdRom devices.
	var bootOrder []vmmModels.BootDeviceType
	if hasCdRoms {
		if vmConfig.BootPriority == NutanixIdentifierBootPriorityCDROM || vmConfig.BootPriority == "" {
			// ISO installation: boot from CdRom first (default when CdRoms exist)
			bootOrder = []vmmModels.BootDeviceType{
				vmmModels.BOOTDEVICETYPE_CDROM,
				vmmModels.BOOTDEVICETYPE_DISK,
				vmmModels.BOOTDEVICETYPE_NETWORK,
			}
		} else {
			// User explicitly chose disk boot priority even with CdRoms
			bootOrder = []vmmModels.BootDeviceType{
				vmmModels.BOOTDEVICETYPE_DISK,
				vmmModels.BOOTDEVICETYPE_CDROM,
				vmmModels.BOOTDEVICETYPE_NETWORK,
			}
		}
	} else if vmConfig.BootPriority == NutanixIdentifierBootPriorityCDROM {
		// No CdRoms but user explicitly requested cdrom boot priority - use DISK first
		// since there are no CdRom devices to boot from.
		log.Printf("WARNING: boot_priority is 'cdrom' but no CdRom devices configured, using DISK boot order")
		bootOrder = []vmmModels.BootDeviceType{
			vmmModels.BOOTDEVICETYPE_DISK,
			vmmModels.BOOTDEVICETYPE_NETWORK,
		}
	} else {
		bootOrder = []vmmModels.BootDeviceType{
			vmmModels.BOOTDEVICETYPE_DISK,
			vmmModels.BOOTDEVICETYPE_NETWORK,
		}
	}

	v4vm.BootConfig = vmmModels.NewOneOfVmBootConfig()
	switch vmConfig.BootType {
	case NutanixIdentifierBootTypeUEFI:
		uefiBoot := vmmModels.NewUefiBoot()
		uefiBoot.BootOrder = bootOrder
		if err := v4vm.BootConfig.SetValue(*uefiBoot); err != nil {
			return nil, fmt.Errorf("error setting UEFI boot config: %s", err.Error())
		}
	case NutanixIdentifierBootTypeSecureBoot:
		uefiBoot := vmmModels.NewUefiBoot()
		uefiBoot.BootOrder = bootOrder
		isSecureBootEnabled := true
		uefiBoot.IsSecureBootEnabled = &isSecureBootEnabled
		if err := v4vm.BootConfig.SetValue(*uefiBoot); err != nil {
			return nil, fmt.Errorf("error setting Secure Boot config: %s", err.Error())
		}
		// Force machine type to Q35, which is required for Secure Boot
		machineType := vmmModels.MACHINETYPE_Q35
		v4vm.MachineType = &machineType
	default:
		// Legacy boot (default)
		legacyBoot := vmmModels.NewLegacyBoot()
		legacyBoot.BootOrder = bootOrder
		if err := v4vm.BootConfig.SetValue(*legacyBoot); err != nil {
			return nil, fmt.Errorf("error setting Legacy boot config: %s", err.Error())
		}
	}

	// V4 API does NOT allow PowerState during creation - must use separate power-on call
	// Error: "Cannot specify power state as ON during VM creation. Please use the VM power action endpoints instead."

	var imageToDelete []string
	SATAindex := 0
	SCSIindex := 0

	for _, disk := range vmConfig.VmDisks {
		if disk.ImageType == "DISK_IMAGE" {
			var image *nutanixImage
			if disk.SourceImageURI != "" {
				image, err = d.CreateImageURL(ctx, disk, vmConfig)
				if err != nil {
					return nil, fmt.Errorf("error while CreateImageURL, Error %s", err.Error())
				}

				if disk.SourceImageDelete {
					log.Printf("mark this image to delete: %s (%s)", image.Name(), image.UUID())
					imageToDelete = append(imageToDelete, image.UUID())
				}

				disk.SourceImageUUID = image.UUID()
			}
			if disk.SourceImageUUID != "" {
				image, err = findImageByUUID(ctx, v4Client, disk.SourceImageUUID)
				if err != nil {
					return nil, fmt.Errorf("error while findImageByUUID, Error %s", err.Error())
				}

				if disk.SourceImageDelete && disk.SourceImagePath != "" {
					log.Printf("mark this image to delete %s:", image.Name())
					imageToDelete = append(imageToDelete, image.UUID())
				}
			} else if disk.SourceImageName != "" {
				image, err = findImageByName(ctx, v4Client, disk.SourceImageName)
				if err != nil {
					return nil, fmt.Errorf("error while findImageByName, %s", err.Error())
				}
			}

			v4Disk := vmmModels.NewDisk()
			v4Disk.DiskAddress = vmmModels.NewDiskAddress()
			v4Disk.DiskAddress.BusType = vmmModels.DISKBUSTYPE_SCSI.Ref()
			// Create a copy of index to avoid pointer aliasing issue
			scsiIdx := SCSIindex
			v4Disk.DiskAddress.Index = &scsiIdx

			vmDisk := vmmModels.NewVmDisk()
			diskSizeBytes := disk.DiskSizeGB * bytesPerGB
			if disk.DiskSizeGB == 0 {
				diskSizeBytes = image.SizeBytes()
			}
			vmDisk.DiskSizeBytes = &diskSizeBytes

			// Use NewDataSource() for proper initialization, set Reference directly to avoid discriminator
			imageUUID := image.UUID()
			imageRef := vmmModels.NewImageReference()
			imageRef.ImageExtId = &imageUUID
			dataSourceRef := vmmModels.NewOneOfDataSourceReference()
			if err := dataSourceRef.SetValue(*imageRef); err != nil {
				return nil, fmt.Errorf("error setting data source reference: %s", err.Error())
			}
			dataSource := vmmModels.NewDataSource()
			dataSource.Reference = dataSourceRef
			vmDisk.DataSource = dataSource

			// Directly assign BackingInfo to avoid $backingInfoItemDiscriminator in JSON
			backingInfo := vmmModels.NewOneOfDiskBackingInfo()
			if err := backingInfo.SetValue(*vmDisk); err != nil {
				return nil, fmt.Errorf("error setting disk backing info: %s", err.Error())
			}
			v4Disk.BackingInfo = backingInfo
			v4vm.Disks = append(v4vm.Disks, *v4Disk)
			SCSIindex++
		}

		if disk.ImageType == "DISK" {
			v4Disk := vmmModels.NewDisk()
			v4Disk.DiskAddress = vmmModels.NewDiskAddress()
			v4Disk.DiskAddress.BusType = vmmModels.DISKBUSTYPE_SCSI.Ref()
			// Create a copy of index to avoid pointer aliasing issue
			scsiIdx := SCSIindex
			v4Disk.DiskAddress.Index = &scsiIdx

			vmDisk := vmmModels.NewVmDisk()
			diskSizeBytes := disk.DiskSizeGB * bytesPerGB
			vmDisk.DiskSizeBytes = &diskSizeBytes

			if disk.StorageContainerUUID != "" {
				vmDisk.StorageContainer = vmmModels.NewVmDiskContainerReference()
				vmDisk.StorageContainer.ExtId = &disk.StorageContainerUUID
			}

			// Directly assign BackingInfo to avoid $backingInfoItemDiscriminator in JSON
			backingInfo := vmmModels.NewOneOfDiskBackingInfo()
			if err := backingInfo.SetValue(*vmDisk); err != nil {
				return nil, fmt.Errorf("error setting disk backing info: %s", err.Error())
			}
			v4Disk.BackingInfo = backingInfo
			v4vm.Disks = append(v4vm.Disks, *v4Disk)
			SCSIindex++
		}

		if disk.ImageType == "ISO_IMAGE" {
			var image *nutanixImage
			if disk.SourceImageURI != "" {
				image, err = d.CreateImageURL(ctx, disk, vmConfig)
				if err != nil {
					return nil, fmt.Errorf("error while CreateImageURL, Error %s", err.Error())
				}

				if disk.SourceImageDelete {
					log.Printf("mark this image to delete %s:", image.Name())
					imageToDelete = append(imageToDelete, image.UUID())
				}

				disk.SourceImageUUID = image.UUID()
			}
			if disk.SourceImageUUID != "" {
				image, err = findImageByUUID(ctx, v4Client, disk.SourceImageUUID)
				if err != nil {
					return nil, fmt.Errorf("error while findImageByUUID, %s", err.Error())
				}

				if disk.SourceImageDelete && disk.SourceImagePath != "" {
					log.Printf("mark this image to delete %s:", image.Name())
					imageToDelete = append(imageToDelete, image.UUID())
				}
			} else if disk.SourceImageName != "" {
				image, err = findImageByName(ctx, v4Client, disk.SourceImageName)
				if err != nil {
					return nil, fmt.Errorf("error while findImageByName, %s", err.Error())
				}
			}

			v4CdRom := vmmModels.NewCdRom()
			vmDisk := vmmModels.NewVmDisk()
			imageUUID := image.UUID()
			imageRef := vmmModels.NewImageReference()
			imageRef.ImageExtId = &imageUUID
			dataSourceRef := vmmModels.NewOneOfDataSourceReference()
			if err := dataSourceRef.SetValue(*imageRef); err != nil {
				return nil, fmt.Errorf("error setting data source reference: %s", err.Error())
			}
			dataSource := vmmModels.NewDataSource()
			dataSource.Reference = dataSourceRef
			vmDisk.DataSource = dataSource
			v4CdRom.BackingInfo = vmDisk

			// Set explicit DiskAddress to avoid "disk bus already in use" when
			// multiple CdRoms are created inline (e.g. ISO + kickstart CD).
			cdromAddr := vmmModels.NewCdRomAddress()
			cdromAddr.BusType = vmmModels.CDROMBUSTYPE_SATA.Ref()
			cdromAddr.Index = new(int)
			*cdromAddr.Index = SATAindex
			v4CdRom.DiskAddress = cdromAddr

			v4vm.CdRoms = append(v4vm.CdRoms, *v4CdRom)
			SATAindex++
		}
	}

	state.Put("image_to_delete", imageToDelete)

	for _, nic := range vmConfig.VmNICs {
		var subnetUUID string
		if nic.SubnetUUID != "" {
			subnetUUID, err = getSubnetUUID(ctx, v4Client, "", nic.SubnetUUID, clusterUUID)
			if err != nil {
				return nil, fmt.Errorf("error while findSubnetByUUID, %s", err.Error())
			}
		} else if nic.SubnetName != "" {
			subnetUUID, err = getSubnetUUID(ctx, v4Client, nic.SubnetName, "", clusterUUID)
			if err != nil {
				return nil, fmt.Errorf("error while findSubnetByName, %s", err.Error())
			}
		}

		v4Nic := vmmModels.NewNic()

		// Use VirtualEthernetNicNetworkInfo for standard VM NICs (v4.1+ API)
		// Note: BackingInfo and IsConnected are deprecated - use NicNetworkInfo instead
		// In v4.1+, NICs are connected by default when NicNetworkInfo is properly configured
		nicNetworkInfo := vmmModels.NewVirtualEthernetNicNetworkInfo()
		nicNetworkInfo.Subnet = vmmModels.NewSubnetReference()
		nicNetworkInfo.Subnet.ExtId = &subnetUUID

		// Directly assign NicNetworkInfo to avoid $nicNetworkInfoItemDiscriminator in JSON
		nicNetworkInfoWrapper := vmmModels.NewOneOfNicNicNetworkInfo()
		if err := nicNetworkInfoWrapper.SetValue(*nicNetworkInfo); err != nil {
			return nil, fmt.Errorf("error setting NIC network info: %s", err.Error())
		}
		v4Nic.NicNetworkInfo = nicNetworkInfoWrapper

		v4vm.Nics = append(v4vm.Nics, *v4Nic)
	}

	if vmConfig.SerialPort {
		serialPort := vmmModels.NewSerialPort()
		serialIndex := 0
		serialPort.Index = &serialIndex
		isConnected := true
		serialPort.IsConnected = &isConnected
		v4vm.SerialPorts = append(v4vm.SerialPorts, *serialPort)
	}

	for _, gpu := range vmConfig.GPU {
		v4GPU, err := getGPU(ctx, v4Client, gpu.Name, clusterUUID)
		if err != nil {
			return nil, fmt.Errorf("error while getGPU %s", err.Error())
		}
		v4vm.Gpus = append(v4vm.Gpus, *v4GPU)
	}

	if vmConfig.UserData != "" {
		log.Printf("Setting up GuestCustomization for OS type: %s", vmConfig.OSType)
		v4vm.GuestCustomization = vmmModels.NewGuestCustomizationParams()

		if vmConfig.OSType == "Linux" {
			cloudInit := vmmModels.NewCloudInit()
			userDataScript := vmmModels.NewUserdata()
			userDataScript.Value = &vmConfig.UserData
			cloudInit.CloudInitScript = vmmModels.NewOneOfCloudInitCloudInitScript()
			if err := cloudInit.CloudInitScript.SetValue(*userDataScript); err != nil {
				return nil, fmt.Errorf("error setting cloud-init script: %s", err.Error())
			}
			// Directly assign Config to avoid $configItemDiscriminator in JSON
			guestConfig := vmmModels.NewOneOfGuestCustomizationParamsConfig()
			if err := guestConfig.SetValue(*cloudInit); err != nil {
				return nil, fmt.Errorf("error setting guest customization config: %s", err.Error())
			}
			v4vm.GuestCustomization.Config = guestConfig
			log.Printf("CloudInit configured for Linux VM")
		} else if vmConfig.OSType == "Windows" {
			sysprep := vmmModels.NewSysprep()
			unattendXml := vmmModels.NewUnattendxml()
			unattendXml.Value = &vmConfig.UserData
			sysprep.SysprepScript = vmmModels.NewOneOfSysprepSysprepScript()
			if err := sysprep.SysprepScript.SetValue(*unattendXml); err != nil {
				return nil, fmt.Errorf("error setting sysprep script: %s", err.Error())
			}
			// Directly assign Config to avoid $configItemDiscriminator in JSON
			guestConfig := vmmModels.NewOneOfGuestCustomizationParamsConfig()
			if err := guestConfig.SetValue(*sysprep); err != nil {
				return nil, fmt.Errorf("error setting guest customization config: %s", err.Error())
			}
			v4vm.GuestCustomization.Config = guestConfig
			log.Printf("Sysprep configured for Windows VM")
		}
	}

	// Configure vTPM for UEFI/SecureBoot VMs
	if (vmConfig.BootType == NutanixIdentifierBootTypeUEFI || vmConfig.BootType == NutanixIdentifierBootTypeSecureBoot) && vmConfig.VTPM.Enabled {
		log.Printf("enabling VTPM for VM %s", vmConfig.VMName)
		v4vm.VtpmConfig = vmmModels.NewVtpmConfig()
		v4vm.VtpmConfig.IsVtpmEnabled = &vmConfig.VTPM.Enabled
	}

	if vmConfig.HardwareVirtualization {
		log.Printf("enabling Hardware Virtualization for VM %s", vmConfig.VMName)
		v4vm.IsVcpuHardPinningEnabled = &vmConfig.HardwareVirtualization
	}

	if len(vmConfig.VMCategories) != 0 {
		categoryExtIds, err := getCategoryExtIds(ctx, v4Client, vmConfig.VMCategories)
		if err != nil {
			return nil, fmt.Errorf("error getting category ExtIds: %s", err.Error())
		}
		v4vm.Categories = make([]vmmModels.CategoryReference, 0, len(categoryExtIds))
		for _, extId := range categoryExtIds {
			catRef := vmmModels.NewCategoryReference()
			catRef.ExtId = &extId
			v4vm.Categories = append(v4vm.Categories, *catRef)
		}
	}

	// Project lookup still uses V3 API
	if vmConfig.Project != "" {
		project, err := findProjectByName(ctx, conn, vmConfig.Project)
		if err != nil {
			return nil, fmt.Errorf("error while findProjectByName, %s", err.Error())
		}
		if project.Metadata != nil && project.Metadata.UUID != nil {
			v4vm.OwnershipInfo = vmmModels.NewOwnershipInfo()
			v4vm.OwnershipInfo.Owner = vmmModels.NewOwnerReference()
			v4vm.OwnershipInfo.Owner.ExtId = project.Metadata.UUID
		}
	}

	return v4vm, nil
}

func (d *NutanixDriver) Create(ctx context.Context, v4vm *vmmModels.Vm) (*nutanixInstance, error) {
	v4Client, err := d.getV4Client()
	if err != nil {
		return nil, fmt.Errorf("error creating V4 client: %s", err.Error())
	}

	log.Printf("creating vm %s...", d.Config.VMName)

	operation, err := v4Client.VMs.CreateAsync(ctx, v4vm)
	if err != nil {
		return nil, fmt.Errorf("error creating VM: %s", err.Error())
	}

	result, err := operation.Wait(ctx)
	if err != nil {
		return nil, fmt.Errorf("error waiting for VM creation: %s", err.Error())
	}

	if len(result) == 0 {
		return nil, fmt.Errorf("VM creation completed but no VM returned")
	}

	createdVM := result[0]
	vmUUID := *createdVM.ExtId

	v4VMResult, err := v4Client.VMs.Get(ctx, vmUUID)
	if err != nil {
		log.Printf("error getting vm after creation: %s", err.Error())
		return nil, err
	}

	log.Printf("vm %s created successfully (powered off)", vmUUID)
	return &nutanixInstance{vm: v4VMResult}, nil
}

// PowerOn powers on a VM. Called after CdRoms are attached so the OS installer can see them.
func (d *NutanixDriver) PowerOn(ctx context.Context, vmUUID string) error {
	v4Client, err := d.getV4Client()
	if err != nil {
		return fmt.Errorf("error creating V4 client for PowerOn: %s", err.Error())
	}

	log.Printf("powering on vm %s...", vmUUID)
	powerOnOp, err := v4Client.VMs.PowerOnVM(vmUUID)
	if err != nil {
		log.Printf("error initiating power on for vm: %s", err.Error())
		return fmt.Errorf("failed to power on VM: %s", err.Error())
	}

	_, err = powerOnOp.Wait(ctx)
	if err != nil {
		log.Printf("error waiting for power on completion: %s", err.Error())
		return fmt.Errorf("failed waiting for VM power on: %s", err.Error())
	}
	log.Printf("vm %s powered on successfully", vmUUID)
	return nil
}

// getV4SDKClient returns the V4 SDK client (for DeleteCdRomById API), creating it if needed.
func (d *NutanixDriver) getV4SDKClient() (*v4.Client, error) {
	if d.v4SDKClient != nil {
		return d.v4SDKClient, nil
	}
	opts := []types.ClientOption[v4.Client]{}
	if d.ClusterConfig.ReadTimeout > 0 {
		opts = append(opts, v4.WithReadTimeout(d.ClusterConfig.ReadTimeout))
	}
	sdkClient, err := v4.NewV4Client(d.getConfigCreds(), opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to create V4 SDK client: %s", err.Error())
	}
	d.v4SDKClient = sdkClient
	return d.v4SDKClient, nil
}

func (d *NutanixDriver) WaitForIP(ctx context.Context, vmUUID string, ipNet *net.IPNet) (string, error) {
	v4Client, err := d.getV4Client()
	if err != nil {
		return "", fmt.Errorf("error creating V4 client: %s", err.Error())
	}

	var IPAddress string

	for {
		vm, err := v4Client.VMs.Get(ctx, vmUUID)
		if err != nil {
			log.Printf("error getting vm: %s", err.Error())
			return "", err
		}

		// Check for IP address in NICs
		// V4 separates IPs into LearnedIpAddresses (guest agent) and Ipv4Config (IPAM)
		// V3 had both in IpEndpointList, so we check both to restore V3 parity
		if len(vm.Nics) > 0 {
			nic := vm.Nics[0]
			nicInfo := nic.GetNicNetworkInfo()

			if nicInfo != nil {
				// Handle VirtualEthernetNicNetworkInfo (standard VM NICs)
				if netInfo, ok := nicInfo.(vmmModels.VirtualEthernetNicNetworkInfo); ok {
					// Priority 1: Check LearnedIpAddresses (from guest agent)
					// This is the "high quality" IP that guest tools report
					if netInfo.Ipv4Info != nil &&
						len(netInfo.Ipv4Info.LearnedIpAddresses) > 0 &&
						netInfo.Ipv4Info.LearnedIpAddresses[0].Value != nil &&
						*netInfo.Ipv4Info.LearnedIpAddresses[0].Value != "" {
						IPAddress = *netInfo.Ipv4Info.LearnedIpAddresses[0].Value
						log.Printf("Found learned IP from guest agent: %s", IPAddress)
						break
					}

					// Priority 2: Fallback to Ipv4Config (from IPAM)
					// This restores V3 parity - allows ISO builds to proceed before guest agent is installed
					if netInfo.Ipv4Config != nil &&
						netInfo.Ipv4Config.IpAddress != nil &&
						netInfo.Ipv4Config.IpAddress.Value != nil &&
						*netInfo.Ipv4Config.IpAddress.Value != "" {
						IPAddress = *netInfo.Ipv4Config.IpAddress.Value
						log.Printf("Found configured IP from IPAM: %s", IPAddress)
						break
					}
				}
			}
		}

		time.Sleep(5 * time.Second)
	}

	log.Printf("VM (%s) configured with ip address %s", vmUUID, IPAddress)

	parseIP := net.ParseIP(IPAddress)
	if ipNet != nil && !ipNet.Contains(parseIP) {
		// IP address is not in the expected range.
		return "", nil
	}
	// Default to IPv4 if no IPNet is provided.
	if ipNet == nil && parseIP.To4() == nil {
		return "", nil
	}
	return IPAddress, nil
}

func (d *NutanixDriver) Delete(ctx context.Context, vmUUID string) error {
	v4Client, err := d.getV4Client()
	if err != nil {
		return fmt.Errorf("error creating V4 client: %s", err.Error())
	}

	err = v4Client.VMs.Delete(ctx, vmUUID)
	if err != nil {
		return err
	}
	return nil
}

// CreateImageURL (VmDisk, VmConfig) (*nutanixImage, error)
func (d *NutanixDriver) CreateImageURL(ctx context.Context, disk VmDisk, vm VmConfig) (*nutanixImage, error) {
	v4Client, err := d.getV4Client()
	if err != nil {
		return nil, fmt.Errorf("error creating V4 client: %s", err.Error())
	}

	_, file := path.Split(disk.SourceImageURI)

	clusterUUID, err := getClusterUUID(ctx, v4Client, vm.ClusterName, vm.ClusterUUID)
	if err != nil {
		return nil, fmt.Errorf("error while getting cluster: %s", err.Error())
	}

	existingImage, err := sourceImageExists(ctx, v4Client, file, disk.SourceImageURI, disk.SourceImageChecksum)
	if err != nil {
		return nil, fmt.Errorf("error while checking if image exists, %s", err.Error())
	}
	if existingImage != nil && !disk.SourceImageForce {
		log.Printf("reuse existing image: %s", *existingImage.Name)
		return &nutanixImage{image: existingImage}, nil
	} else if existingImage != nil && disk.SourceImageForce {
		log.Printf("delete existing image: %s", *existingImage.Name)
		if err := d.DeleteImage(ctx, *existingImage.ExtId); err != nil {
			log.Printf("warning: failed to delete existing image: %s", err.Error())
		}
		log.Printf("recreating image: %s", file)
	} else if existingImage == nil {
		log.Printf("creating image: %s", file)
	}

	v4Image := imageModels.NewImage()
	v4Image.Name = &file
	v4Image.Description = StringPtr(defaultImageDLDescription)

	if disk.ImageType == "DISK_IMAGE" {
		v4Image.Type = imageModels.IMAGETYPE_DISK_IMAGE.Ref()
	} else if disk.ImageType == "ISO_IMAGE" {
		v4Image.Type = imageModels.IMAGETYPE_ISO_IMAGE.Ref()
	}

	urlSource := imageModels.NewUrlSource()
	urlSource.Url = &disk.SourceImageURI

	v4Image.Source = imageModels.NewOneOfImageSource()
	if err := v4Image.Source.SetValue(*urlSource); err != nil {
		return nil, fmt.Errorf("error setting image source: %s", err.Error())
	}

	if disk.SourceImageChecksum != "" {
		v4Image.Checksum = imageModels.NewOneOfImageChecksum()
		switch disk.SourceImageChecksumType {
		case NutanixIdentifierChecksunTypeSHA256:
			sha256Checksum := imageModels.NewImageSha256Checksum()
			sha256Checksum.HexDigest = &disk.SourceImageChecksum
			if err := v4Image.Checksum.SetValue(*sha256Checksum); err != nil {
				return nil, fmt.Errorf("error setting SHA256 checksum: %s", err.Error())
			}
			log.Printf("image checksum (SHA256): %s", disk.SourceImageChecksum)
		case NutanixIdentifierChecksunTypeSHA1:
			sha1Checksum := imageModels.NewImageSha1Checksum()
			sha1Checksum.HexDigest = &disk.SourceImageChecksum
			if err := v4Image.Checksum.SetValue(*sha1Checksum); err != nil {
				return nil, fmt.Errorf("error setting SHA1 checksum: %s", err.Error())
			}
			log.Printf("image checksum (SHA1): %s", disk.SourceImageChecksum)
		}
	}

	v4Image.ClusterLocationExtIds = []string{clusterUUID}

	log.Printf("Creating image - Name: %s, Type: %s, Cluster: %s", *v4Image.Name, v4Image.Type.GetName(), clusterUUID)

	createdImage, err := v4Client.Images.Create(ctx, v4Image)
	if err != nil {
		log.Printf("ERROR: Image creation failed: %s", err.Error())
		log.Printf("Full error details: %+v", err)
		return nil, fmt.Errorf("error while creating image: %s", err.Error())
	}

	log.Printf("image successfully created")

	// Verify image is fully ready before returning
	// The V4 API task may complete before the image is fully usable for VM disk cloning
	// Using 5-second intervals to match V3 API's checkTask polling behavior
	imageUUID := *createdImage.ExtId
	log.Printf("Verifying image %s is ready for use...", imageUUID)

	maxRetries := 12 // 12 retries * 5 seconds = 60 seconds max wait
	for i := 0; i < maxRetries; i++ {
		verifiedImage, verifyErr := v4Client.Images.Get(ctx, imageUUID)
		if verifyErr != nil {
			log.Printf("Error verifying image (attempt %d/%d): %s", i+1, maxRetries, verifyErr.Error())
			time.Sleep(5 * time.Second)
			continue
		}

		// Check if SizeBytes is set - indicates image data is available
		if verifiedImage.SizeBytes != nil && *verifiedImage.SizeBytes > 0 {
			log.Printf("Image %s is ready (size: %d bytes)", imageUUID, *verifiedImage.SizeBytes)
			return &nutanixImage{image: verifiedImage}, nil
		}

		log.Printf("Image %s not ready yet (SizeBytes is nil or 0), waiting... (attempt %d/%d)", imageUUID, i+1, maxRetries)
		time.Sleep(5 * time.Second)
	}

	log.Printf("WARNING: Image %s readiness check timed out, proceeding anyway...", imageUUID)
	return &nutanixImage{image: createdImage}, nil
}

// CreateImageFile uploads a local file as a new image using Objects Lite.
func (d *NutanixDriver) CreateImageFile(ctx context.Context, filePath string, vm VmConfig) (*nutanixImage, error) {
	v4Client, err := d.getV4Client()
	if err != nil {
		return nil, fmt.Errorf("error creating V4 client: %s", err.Error())
	}

	_, file := path.Split(filePath)

	log.Printf("creating and uploading image: %s", file)

	err = v4Client.Images.Upload(ctx, file, filePath)
	if err != nil {
		return nil, fmt.Errorf("error while uploading image: %s", err.Error())
	}

	createdImage, err := findImageByName(ctx, v4Client, file)
	if err != nil {
		return nil, fmt.Errorf("error while getting created image: %s", err.Error())
	}

	log.Printf("image successfully uploaded: %s", file)

	return createdImage, nil
}

func (d *NutanixDriver) DeleteImage(ctx context.Context, imageUUID string) error {
	v4Client, err := d.getV4Client()
	if err != nil {
		return fmt.Errorf("error creating V4 client: %s", err.Error())
	}

	err = v4Client.Images.Delete(ctx, imageUUID)
	if err != nil {
		return fmt.Errorf("error while deleting image: %s", err.Error())
	}
	return nil
}

func (d *NutanixDriver) GetImage(ctx context.Context, imagename string) (*nutanixImage, error) {
	v4Client, err := d.getV4Client()
	if err != nil {
		return nil, fmt.Errorf("error creating V4 client: %s", err.Error())
	}

	image, err := findImageByName(ctx, v4Client, imagename)
	if err != nil {
		return nil, fmt.Errorf("error while GetImage, %s", err.Error())
	}
	return image, nil
}

func (d *NutanixDriver) GetVM(ctx context.Context, vmUUID string) (*nutanixInstance, error) {
	v4Client, err := d.getV4Client()
	if err != nil {
		return nil, fmt.Errorf("error creating V4 client: %s", err.Error())
	}

	vm, err := v4Client.VMs.Get(ctx, vmUUID)
	if err != nil {
		return nil, fmt.Errorf("error while GetVM, %s", err.Error())
	}

	return &nutanixInstance{vm: vm}, nil
}

// findOvaByName finds the latest OVA by name using V4 API and returns its UUID
func findOvaByName(ctx context.Context, v4Client *convergedv4.Client, name string) (string, error) {
	ovas, err := v4Client.Ovas.List(ctx, converged.WithFilter(fmt.Sprintf("name eq '%s'", name)))
	if err != nil {
		return "", err
	}

	if len(ovas) == 0 {
		return "", nil
	}

	// Filter by exact name match
	found := make([]imageModels.Ova, 0)
	for _, ova := range ovas {
		if ova.Name != nil && strings.EqualFold(*ova.Name, name) {
			found = append(found, ova)
		}
	}

	if len(found) == 0 {
		return "", nil
	}

	// Sort by CreateTime to get latest (descending order)
	sort.Slice(found, func(i, j int) bool {
		if found[i].CreateTime == nil || found[j].CreateTime == nil {
			return found[i].CreateTime != nil
		}
		return found[i].CreateTime.After(*found[j].CreateTime)
	})

	if found[0].ExtId == nil {
		return "", fmt.Errorf("OVA %s has no ExtId", name)
	}
	return *found[0].ExtId, nil
}

func (d *NutanixDriver) CreateTemplate(ctx context.Context, vmUUID string, templateConfig TemplateConfig) error {
	v4Client, err := d.getV4Client()
	if err != nil {
		return fmt.Errorf("error creating V4 client: %s", err.Error())
	}

	log.Printf("creating template %s from VM %s", templateConfig.Name, vmUUID)

	vmRef := imageModels.NewTemplateVmReference()
	vmRef.ExtId = &vmUUID

	versionSpec := imageModels.NewTemplateVersionSpec()
	isActive := true
	isGcOverride := true
	versionSpec.IsActiveVersion = &isActive
	versionSpec.IsGcOverrideEnabled = &isGcOverride
	if err := versionSpec.SetVersionSource(*vmRef); err != nil {
		return fmt.Errorf("error setting template version source: %s", err.Error())
	}

	template := imageModels.NewTemplate()
	template.TemplateName = &templateConfig.Name
	template.TemplateDescription = &templateConfig.Description
	template.TemplateVersionSpec = versionSpec

	_, err = v4Client.Templates.Create(ctx, template)
	if err != nil {
		return fmt.Errorf("error creating template: %s", err.Error())
	}

	log.Printf("Template %s created successfully", templateConfig.Name)
	return nil
}

func (d *NutanixDriver) CreateOVA(ctx context.Context, ovaName string, vmUUID string, diskFileFormat string) error {
	v4Client, err := d.getV4Client()
	if err != nil {
		return fmt.Errorf("error creating V4 client: %s", err.Error())
	}

	log.Printf("creating OVA %s from VM %s with disk format %s", ovaName, vmUUID, diskFileFormat)

	vmSource := imageModels.NewOvaVmSource()
	vmSource.VmExtId = &vmUUID

	switch strings.ToUpper(diskFileFormat) {
	case "QCOW2":
		vmSource.DiskFileFormat = imageModels.OVADISKFORMAT_QCOW2.Ref()
	case "VMDK":
		vmSource.DiskFileFormat = imageModels.OVADISKFORMAT_VMDK.Ref()
	default:
		vmSource.DiskFileFormat = imageModels.OVADISKFORMAT_QCOW2.Ref()
	}

	ova := imageModels.NewOva()
	ova.Name = &ovaName
	if err := ova.SetSource(*vmSource); err != nil {
		return fmt.Errorf("error setting OVA source: %s", err.Error())
	}

	_, err = v4Client.Ovas.Create(ctx, ova)
	if err != nil {
		return fmt.Errorf("error creating OVA: %s", err.Error())
	}

	log.Printf("OVA %s created successfully", ovaName)
	return nil
}

func (d *NutanixDriver) ExportOVA(ctx context.Context, ovaName string) (string, error) {
	log.Printf("starting OVA export for OVA: %s", ovaName)

	v4Client, err := d.getV4Client()
	if err != nil {
		return "", fmt.Errorf("error creating V4 client: %s", err.Error())
	}

	var ovaUUID string

	// Wait for OVA to appear in list (up to 60s)
	for i := 0; i < 12; i++ {
		ovaUUID, err = findOvaByName(ctx, v4Client, ovaName)
		if err != nil {
			log.Printf("error finding OVA: %s", err.Error())
		}
		if ovaUUID == "" {
			<-time.After(5 * time.Second)
		} else {
			break
		}
	}

	if ovaUUID == "" {
		return "", fmt.Errorf("timeout waiting for OVA entity to appear")
	}

	log.Printf("downloading OVA %s", ovaUUID)
	fileDetail, err := v4Client.Ovas.GetFile(ctx, ovaUUID)
	if err != nil {
		return "", fmt.Errorf("error downloading OVA: %s", err.Error())
	}

	if fileDetail == nil || fileDetail.Path == nil {
		return "", fmt.Errorf("OVA download returned no file path")
	}

	log.Printf("OVA downloaded to: %s", *fileDetail.Path)
	return *fileDetail.Path, nil
}

func (d *NutanixDriver) ExportImage(ctx context.Context, imageUUID string) (string, error) {
	log.Printf("downloading image %s", imageUUID)

	v4Client, err := d.getV4Client()
	if err != nil {
		return "", fmt.Errorf("error creating V4 client: %s", err.Error())
	}

	fileDetail, err := v4Client.Images.GetFile(ctx, imageUUID)
	if err != nil {
		return "", fmt.Errorf("error downloading image: %s", err.Error())
	}

	if fileDetail == nil || fileDetail.Path == nil {
		return "", fmt.Errorf("image download returned no file path")
	}

	log.Printf("Image downloaded to: %s", *fileDetail.Path)
	return *fileDetail.Path, nil
}

func (d *NutanixDriver) GetHost(ctx context.Context, hostUUID string) (*nutanixHost, error) {
	v4Client, err := d.getV4Client()
	if err != nil {
		return nil, fmt.Errorf("error creating V4 client: %s", err.Error())
	}

	host, err := findHostByUUID(ctx, v4Client, hostUUID)
	if err != nil {
		return nil, fmt.Errorf("error while GetHost: %s", err.Error())
	}

	return &nutanixHost{host: host}, nil
}

func (d *NutanixDriver) PowerOff(ctx context.Context, vmUUID string) error {
	v4Client, err := d.getV4Client()
	if err != nil {
		return fmt.Errorf("error creating V4 client: %s", err.Error())
	}

	log.Printf("stopping VM: %s", d.Config.VMName)

	operation, err := v4Client.VMs.PowerOffVM(vmUUID)
	if err != nil {
		return fmt.Errorf("error while PowerOff VM: %s", err.Error())
	}

	_, err = operation.Wait(ctx)
	if err != nil {
		return fmt.Errorf("error while stopping VM: %s", err.Error())
	}

	return nil
}

func (d *NutanixDriver) SaveVMDisk(ctx context.Context, diskUUID string, index int, imageCategories []Category) (*nutanixImage, error) {
	v4Client, err := d.getV4Client()
	if err != nil {
		return nil, fmt.Errorf("error creating V4 client: %s", err.Error())
	}

	name := d.Config.VmConfig.ImageName
	if index > 0 {
		name = fmt.Sprintf("%s-disk%d", name, index+1)
	}

	if d.Config.ForceDeregister || d.Config.FailIfImageExists {
		log.Println("check if image already exists")
		images, err := v4Client.Images.List(ctx, converged.WithFilter(fmt.Sprintf("name eq '%s'", name)))
		if err != nil {
			return nil, fmt.Errorf("error while listing images: %s", err.Error())
		}

		found := make([]*imageModels.Image, 0)
		for i := range images {
			if images[i].Name != nil && strings.EqualFold(*images[i].Name, name) {
				found = append(found, &images[i])
			}
		}

		if len(found) == 0 {
			log.Println("image with given Name not found, no need to deregister")
		} else if len(found) > 1 {
			if d.Config.FailIfImageExists {
				return nil, fmt.Errorf("more than one image with name %s found, please use a unique name", name)
			}
			log.Println("more than one image with given Name found, will not deregister")
		} else if len(found) == 1 {
			if d.Config.FailIfImageExists {
				return nil, fmt.Errorf("one image with name %s found, please use a unique name", name)
			}

			if found[0].ExtId == nil {
				return nil, fmt.Errorf("image %s has no ExtId", name)
			}

			log.Println("one image with given Name found, will deregister")
			log.Printf("deleting image %s...\n", *found[0].ExtId)

			err := v4Client.Images.Delete(ctx, *found[0].ExtId)
			if err != nil {
				return nil, fmt.Errorf("error while Deleting Image: %s", err.Error())
			}
		}
	}

	imgDescription := defaultImageBuiltDescription
	if d.Config.ImageDescription != "" {
		imgDescription = d.Config.ImageDescription
	}

	v4Image := imageModels.NewImage()
	v4Image.Name = &name
	v4Image.Description = &imgDescription
	v4Image.Type = imageModels.IMAGETYPE_DISK_IMAGE.Ref()

	vmDiskSource := imageModels.NewVmDiskSource()
	vmDiskSource.ExtId = &diskUUID

	v4Image.Source = imageModels.NewOneOfImageSource()
	if err := v4Image.Source.SetValue(*vmDiskSource); err != nil {
		return nil, fmt.Errorf("error setting VM disk source: %s", err.Error())
	}

	if len(imageCategories) != 0 {
		categoryExtIds, err := getCategoryExtIds(ctx, v4Client, imageCategories)
		if err != nil {
			return nil, fmt.Errorf("error getting category ExtIds: %s", err.Error())
		}
		v4Image.CategoryExtIds = categoryExtIds
	}

	log.Printf("creating image %s from VM disk %s...", name, diskUUID)
	createdImage, err := v4Client.Images.Create(ctx, v4Image)
	if err != nil {
		return nil, fmt.Errorf("error while Creating Image: %s", err.Error())
	}

	log.Printf("image %s created successfully", *createdImage.ExtId)

	return &nutanixImage{image: createdImage}, nil
}

func (d *NutanixDriver) UpdateVM(ctx context.Context, vmUUID string, v4vm *vmmModels.Vm) (*nutanixInstance, error) {
	v4Client, err := d.getV4Client()
	if err != nil {
		return nil, fmt.Errorf("error creating V4 client: %s", err.Error())
	}

	updatedVM, err := v4Client.VMs.Update(ctx, vmUUID, v4vm)
	if err != nil {
		return nil, fmt.Errorf("error while Updating VM: %s", err.Error())
	}

	return &nutanixInstance{vm: updatedVM}, nil
}

// CleanCD removes all CD-ROM devices from a VM via the V4 DeleteCdRomById sub-resource API.
// The V4 API does not allow changing disk/CdRom count via UpdateVM, so each CdRom must be
// deleted individually. The VM must be powered off.
func (d *NutanixDriver) CleanCD(ctx context.Context, vmUUID string) error {
	sdkClient, err := d.getV4SDKClient()
	if err != nil {
		return fmt.Errorf("failed to get V4 SDK client: %s", err.Error())
	}

	v4Client, err := d.getV4Client()
	if err != nil {
		return fmt.Errorf("failed to get V4 client: %s", err.Error())
	}

	vm, err := v4Client.VMs.Get(ctx, vmUUID)
	if err != nil {
		return fmt.Errorf("failed to get VM for CdRom cleanup: %s", err.Error())
	}

	if len(vm.CdRoms) == 0 {
		log.Println("No CdRoms to clean")
		return nil
	}

	log.Printf("Cleaning %d CdRom(s) from VM %s", len(vm.CdRoms), vmUUID)
	for i, cdrom := range vm.CdRoms {
		if cdrom.ExtId == nil {
			log.Printf("CdRom %d has no ExtId, skipping", i+1)
			continue
		}
		cdromID := *cdrom.ExtId

		_, etagArgs, err := convergedv4.GetEntityAndEtag(v4Client.VMs.Get(ctx, vmUUID))
		if err != nil {
			return fmt.Errorf("failed to get VM ETag before deleting CdRom %d: %s", i+1, err.Error())
		}

		taskRef, err := convergedv4.CallAPI[*vmmModels.DeleteCdRomApiResponse, vmmPrismConfig.TaskReference](
			sdkClient.VmApiInstance.DeleteCdRomById(&vmUUID, &cdromID, etagArgs),
		)
		if err != nil {
			return fmt.Errorf("failed to delete CdRom %d (%s): %s", i+1, cdromID, err.Error())
		}
		if taskRef.ExtId == nil {
			return fmt.Errorf("task reference ExtId is nil for CdRom %d deletion", i+1)
		}
		taskID := *taskRef.ExtId
		log.Printf("CdRom %d: delete task started: %s", i+1, taskID)

		waiter := convergedv4.NewOperation(
			taskID,
			sdkClient,
			func(ctx context.Context, uuid string) (*converged.NoEntity, error) {
				return converged.NoEntityGetter(ctx, uuid)
			},
		)
		_, err = waiter.Wait(ctx)
		if err != nil {
			return fmt.Errorf("CdRom %d deletion task failed: %s", i+1, err.Error())
		}
		log.Printf("CdRom %d (%s) deleted successfully", i+1, cdromID)
	}
	return nil
}

// GenerateConsoleToken obtains a JWT token and WebSocket URI for VNC console access.
// It calls the V4 generate-console-token API (async 202), polls the task until SUCCEEDED
// via NewOperation+Wait, then extracts VmConsoleToken and WsUri from task CompletionDetails.
// Used by stepVNCConnect for boot commands over VNC during ISO-based builds.
//
// Parameters:
//   - ctx: context for cancellation
//   - vmExtId: VM external ID (UUID)
//
// Returns: token (JWT), wsUri (path e.g. /console/launch/{vmId}), or error.
func (d *NutanixDriver) GenerateConsoleToken(ctx context.Context, vmExtId string) (token, wsUri string, err error) {
	sdkClient, err := d.getV4SDKClient()
	if err != nil {
		return "", "", fmt.Errorf("failed to get V4 SDK client: %s", err.Error())
	}

	resp, err := sdkClient.VmApiInstance.GenerateConsoleTokenById(&vmExtId)
	if err != nil {
		return "", "", fmt.Errorf("generate-console-token request failed: %w", err)
	}
	if resp == nil || resp.Data == nil {
		return "", "", fmt.Errorf("generate-console-token returned empty response")
	}

	dataVal := resp.Data.GetValue()
	if dataVal == nil {
		return "", "", fmt.Errorf("generate-console-token response data is nil")
	}

	taskRef, ok := dataVal.(vmmPrismConfig.TaskReference)
	if !ok {
		if errResp, ok := dataVal.(vmmError.ErrorResponse); ok {
			return "", "", fmt.Errorf("generate-console-token failed: %v", errResp)
		}
		return "", "", fmt.Errorf("generate-console-token returned unexpected type: %T", dataVal)
	}
	if taskRef.ExtId == nil {
		return "", "", fmt.Errorf("task reference ExtId is nil")
	}
	taskID := *taskRef.ExtId
	log.Printf("console token task started: %s", taskID)

	// Poll until SUCCEEDED (same pattern as CleanCD). Result is in CompletionDetails, not EntitiesAffected.
	waiter := convergedv4.NewOperation(
		taskID,
		sdkClient,
		func(ctx context.Context, uuid string) (*converged.NoEntity, error) {
			return converged.NoEntityGetter(ctx, uuid)
		},
	)
	_, err = waiter.Wait(ctx)
	if err != nil {
		return "", "", fmt.Errorf("console token task: %w", err)
	}

	// Get completed task; token and wsUri are in CompletionDetails KVPairs.
	taskResp, err := convergedv4.CallAPI[*prismConfig.GetTaskApiResponse, prismConfig.Task](
		sdkClient.TasksApiInstance.GetTaskById(&taskID, nil),
	)
	if err != nil {
		return "", "", fmt.Errorf("failed to get completed task: %w", err)
	}

	var tokenVal, wsUriVal string
	for _, kv := range taskResp.CompletionDetails {
		if kv.Name == nil {
			continue
		}
		switch *kv.Name {
		case "VmConsoleToken":
			if kv.Value != nil {
				if v := kv.Value.GetValue(); v != nil {
					if s, ok := v.(string); ok {
						tokenVal = s
					}
				}
			}
		case "WsUri":
			if kv.Value != nil {
				if v := kv.Value.GetValue(); v != nil {
					if s, ok := v.(string); ok {
						wsUriVal = s
					}
				}
			}
		}
	}
	if tokenVal == "" || wsUriVal == "" {
		return "", "", fmt.Errorf("task completionDetails missing VmConsoleToken or WsUri")
	}
	log.Printf("console token generated, wsUri=%s", wsUriVal)
	return tokenVal, wsUriVal, nil
}
