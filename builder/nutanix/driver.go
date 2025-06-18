package nutanix

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"path"
	"strings"
	"time"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	client "github.com/nutanix-cloud-native/prism-go-client"
	v3 "github.com/nutanix-cloud-native/prism-go-client/v3"
)

const (
	defaultImageBuiltDescription = "built by Packer"
	defaultImageDLDescription    = "added by Packer"
	vmDescription                = "Packer vm building image %s"
)

// Driver is able to talk to Nutanix PrismCentral and perform certain
// operations with it.
type Driver interface {
	CreateRequest(context.Context, VmConfig, multistep.StateBag) (*v3.VMIntentInput, error)
	Create(context.Context, *v3.VMIntentInput) (*nutanixInstance, error)
	Delete(context.Context, string) error
	GetVM(context.Context, string) (*nutanixInstance, error)
	GetHost(context.Context, string) (*nutanixHost, error)
	PowerOff(context.Context, string) error
	CreateImageURL(context.Context, VmDisk, VmConfig) (*nutanixImage, error)
	CreateImageFile(context.Context, string, VmConfig) (*nutanixImage, error)
	DeleteImage(context.Context, string) error
	GetImage(context.Context, string) (*nutanixImage, error)
	CreateOVA(context.Context, string, string, string) error
	ExportOVA(context.Context, string) (io.ReadCloser, error)
	ExportImage(context.Context, string) (io.ReadCloser, error)
	SaveVMDisk(context.Context, string, int, []Category) (*nutanixImage, error)
	WaitForShutdown(string, <-chan struct{}) bool
}

// Verify that NutanixDriver implements the Driver interface
var _ Driver = &NutanixDriver{}

// NutanixDriver is a driver for Nutanix PrismCentral calls
type NutanixDriver struct {
	Config        Config
	ClusterConfig ClusterConfig
	vmEndCh       <-chan int
}

type nutanixInstance struct {
	nutanix v3.VMIntentResponse
}

type nutanixHost struct {
	host v3.HostResponse
}

type nutanixImage struct {
	image v3.ImageIntentResponse
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

func findClusterByName(ctx context.Context, conn *v3.Client, name string) (*v3.ClusterIntentResponse, error) {
	resp, err := conn.V3.ListAllCluster(ctx, "")
	if err != nil {
		return nil, err
	}
	entities := resp.Entities

	found := make([]*v3.ClusterIntentResponse, 0)
	for _, v := range entities {
		if strings.EqualFold(*v.Status.Name, name) {
			found = append(found, &v3.ClusterIntentResponse{
				Status:     v.Status,
				Spec:       v.Spec,
				Metadata:   v.Metadata,
				APIVersion: v.APIVersion,
			})
		}
	}

	if len(found) > 1 {
		return nil, fmt.Errorf("your query returned more than one result. Please use cluster_uuid argument instead")
	}

	if len(found) == 0 {
		return nil, fmt.Errorf("did not find cluster with name %s", name)
	}

	return found[0], nil
}

func findSubnetByUUID(ctx context.Context, conn *v3.Client, uuid string) (*v3.SubnetIntentResponse, error) {
	return conn.V3.GetSubnet(ctx, uuid)
}

func findSubnetByName(ctx context.Context, conn *v3.Client, name, clusterUUID string) (*v3.SubnetIntentResponse, error) {
	resp, err := conn.V3.ListAllSubnet(ctx, "", nil)
	if err != nil {
		return nil, err
	}

	entities := resp.Entities

	subnets := make([]*v3.SubnetIntentResponse, 0)
	for _, v := range entities {
		if strings.EqualFold(*v.Spec.Name, name) {
			subnets = append(subnets, v)
		}
	}

	if len(subnets) == 1 {
		return subnets[0], nil
	}

	if len(subnets) == 0 {
		return nil, fmt.Errorf("did not find subnet with name %s", name)
	}

	// More than one subnet. Try to narrow the subnets to one by filtering by cluster UUID.
	clusterSubnets := make([]*v3.SubnetIntentResponse, 0)
	for _, s := range subnets {
		if s.Spec.ClusterReference != nil &&
			s.Spec.ClusterReference.UUID != nil &&
			*s.Spec.ClusterReference.UUID == clusterUUID {
			clusterSubnets = append(clusterSubnets, s)
		}
	}
	if len(clusterSubnets) == 1 {
		return clusterSubnets[0], nil
	}

	return nil, fmt.Errorf("your query returned more than one result. Please use subnet_uuid argument instead")
}

func findGPUByName(ctx context.Context, conn *v3.Client, name string) (*v3.VMGpu, error) {
	hosts, err := conn.V3.ListAllHost(ctx)
	if err != nil {
		return nil, err
	}

	for _, host := range hosts.Entities {
		if host == nil ||
			host.Status == nil ||
			host.Status.ClusterReference == nil ||
			host.Status.Resources == nil ||
			len(host.Status.Resources.GPUList) == 0 {
			continue
		}

		for _, peGpu := range host.Status.Resources.GPUList {
			if peGpu == nil {
				continue
			}
			if strings.EqualFold(peGpu.Name, name) {
				return &v3.VMGpu{
					DeviceID: peGpu.DeviceID,
					Vendor:   &peGpu.Vendor,
					Mode:     &peGpu.Mode,
				}, nil
			}
		}
	}
	return nil, fmt.Errorf("failed to find GPU %s", name)
}

func sourceImageExists(ctx context.Context, conn *v3.Client, name string, uri string) (*v3.ImageIntentResponse, error) {
	resp, err := conn.V3.ListAllImage(ctx, "")
	if err != nil {
		return nil, err
	}

	entities := resp.Entities

	found := make([]*v3.ImageIntentResponse, 0)
	for _, v := range entities {
		if strings.EqualFold(*v.Spec.Name, name) && strings.EqualFold(*v.Status.Resources.SourceURI, uri) {
			found = append(found, v)
		}
	}

	if len(found) > 1 {
		return nil, fmt.Errorf("your query returned more than one result with same Name/URI")
	}

	if len(found) == 0 {
		return nil, nil
	}
	return found[0], nil
}

func findImageByUUID(ctx context.Context, conn *v3.Client, uuid string) (*v3.ImageIntentResponse, error) {
	return conn.V3.GetImage(ctx, uuid)
}

func findImageByName(ctx context.Context, conn *v3.Client, name string) (*v3.ImageIntentResponse, error) {
	resp, err := conn.V3.ListAllImage(ctx, "")
	if err != nil {
		return nil, err
	}

	entities := resp.Entities

	found := make([]*v3.ImageIntentResponse, 0)
	for _, v := range entities {
		if strings.EqualFold(*v.Spec.Name, name) {
			found = append(found, v)
		}
	}

	if len(found) > 1 {
		return nil, fmt.Errorf("your query returned multiple results with name %s. Please use soure_image_uuid argument instead", name)
	}

	if len(found) == 0 {
		return nil, fmt.Errorf("image %s not found", name)
	}

	return findImageByUUID(ctx, conn, *found[0].Metadata.UUID)
}

func checkTask(ctx context.Context, conn *v3.Client, taskUUID string, timeout int) error {
	log.Printf("checking task %s...", taskUUID)
	var task *v3.TasksResponse
	var err error
	for i := 0; i < (timeout / 5); i++ {
		task, err = conn.V3.GetTask(ctx, taskUUID)
		if err == nil {
			if *task.Status == "SUCCEEDED" {
				return nil
			} else if *task.Status == "FAILED" {
				return fmt.Errorf(*task.ErrorDetail)
			} else {
				log.Printf("task status is " + *task.Status)
				<-time.After(5 * time.Second)
			}
		} else {
			return err
		}
	}
	return fmt.Errorf("check task %s timeout", taskUUID)
}

func (d *nutanixInstance) Addresses() []string {
	var addresses []string
	if len(d.nutanix.Status.Resources.NicList) > 0 {
		for _, n := range d.nutanix.Status.Resources.NicList {
			addresses = append(addresses, *n.IPEndpointList[0].IP)
		}
	}
	return addresses
}

func (d *nutanixInstance) PowerState() string {
	return *d.nutanix.Status.Resources.PowerState
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

func (d *NutanixDriver) CreateRequest(ctx context.Context, vm VmConfig, state multistep.StateBag) (*v3.VMIntentInput, error) {
	configCreds := client.Credentials{
		URL:      fmt.Sprintf("%s:%d", d.ClusterConfig.Endpoint, d.ClusterConfig.Port),
		Endpoint: d.ClusterConfig.Endpoint,
		Username: d.ClusterConfig.Username,
		Password: d.ClusterConfig.Password,
		Port:     string(d.ClusterConfig.Port),
		Insecure: d.ClusterConfig.Insecure,
	}

	conn, err := v3.NewV3Client(configCreds)
	if err != nil {
		return nil, err
	}

	log.Printf("preparing vm %s...", d.Config.VMName)

	// If UserData exists, create GuestCustomization
	var guestCustomization *v3.GuestCustomization
	if vm.UserData == "" {
		guestCustomization = nil
	} else {
		if vm.OSType == "Windows" {
			installType := "FRESH"
			guestCustomization = &v3.GuestCustomization{
				Sysprep: &v3.GuestCustomizationSysprep{
					InstallType: &installType,
					UnattendXML: &vm.UserData,
				},
			}
		}
		if vm.OSType == "Linux" {
			guestCustomization = &v3.GuestCustomization{
				CloudInit: &v3.GuestCustomizationCloudInit{
					UserData: &vm.UserData,
				},
			}
		}
	}
	DiskList := []*v3.VMDisk{}
	SATAindex := 0
	SCSIindex := 0

	var imageToDelete []string

	for _, disk := range vm.VmDisks {
		if disk.ImageType == "DISK_IMAGE" {
			image := &v3.ImageIntentResponse{}
			if disk.SourceImageURI != "" {
				image, err := d.CreateImageURL(ctx, disk, vm)
				if err != nil {
					return nil, fmt.Errorf("error while findImageByUUID, Error %s", err.Error())
				}

				if disk.SourceImageDelete {
					log.Printf("mark this image to delete: %s (%s)", *image.image.Spec.Name, *image.image.Metadata.UUID)
					imageToDelete = append(imageToDelete, *image.image.Metadata.UUID)
				}

				disk.SourceImageUUID = *image.image.Metadata.UUID
			}
			if disk.SourceImageUUID != "" {
				image, err = findImageByUUID(ctx, conn, disk.SourceImageUUID)
				if err != nil {
					return nil, fmt.Errorf("error while findImageByUUID, Error %s", err.Error())
				}
			} else if disk.SourceImageName != "" {
				image, err = findImageByName(ctx, conn, disk.SourceImageName)
				if err != nil {
					return nil, fmt.Errorf("error while findImageByName, %s", err.Error())
				}
			}

			DeviceType := "DISK"
			AdapterType := "SCSI"
			DeviceIndex := int64(SCSIindex)
			DiskSizeMib := disk.DiskSizeGB * 1024
			if disk.DiskSizeGB == 0 {
				DiskSizeMib = *image.Status.Resources.SizeBytes / 1024 / 1024
			}
			newDisk := v3.VMDisk{
				DeviceProperties: &v3.VMDiskDeviceProperties{
					DeviceType: &DeviceType,
					DiskAddress: &v3.DiskAddress{
						AdapterType: &AdapterType,
						DeviceIndex: &DeviceIndex,
					},
				},
				DataSourceReference: BuildReference(*image.Metadata.UUID, "image"),
				DiskSizeMib:         &DiskSizeMib,
			}
			DiskList = append(DiskList, &newDisk)
			SCSIindex++
		}
		if disk.ImageType == "DISK" {
			DeviceType := "DISK"
			AdapterType := "SCSI"
			DeviceIndex := int64(SCSIindex)
			DiskSizeMib := disk.DiskSizeGB * 1024
			newDisk := v3.VMDisk{
				DeviceProperties: &v3.VMDiskDeviceProperties{
					DeviceType: &DeviceType,
					DiskAddress: &v3.DiskAddress{
						AdapterType: &AdapterType,
						DeviceIndex: &DeviceIndex,
					},
				},
				DiskSizeMib: &DiskSizeMib,
			}
			DiskList = append(DiskList, &newDisk)
			SCSIindex++
		}
		if disk.ImageType == "ISO_IMAGE" {
			image := &v3.ImageIntentResponse{}
			if disk.SourceImageURI != "" {
				image, err := d.CreateImageURL(ctx, disk, vm)
				if err != nil {
					return nil, fmt.Errorf("error while findImageByUUID, Error %s", err.Error())
				}

				if disk.SourceImageDelete {
					log.Printf("mark this image to delete %s:", *image.image.Status.Name)
					imageToDelete = append(imageToDelete, *image.image.Metadata.UUID)
				}

				disk.SourceImageUUID = *image.image.Metadata.UUID
			}
			if disk.SourceImageUUID != "" {
				image, err = findImageByUUID(ctx, conn, disk.SourceImageUUID)
				if err != nil {
					return nil, fmt.Errorf("error while findImageByUUID, %s", err.Error())
				}
			} else if disk.SourceImageName != "" {
				image, err = findImageByName(ctx, conn, disk.SourceImageName)
				if err != nil {
					return nil, fmt.Errorf("error while findImageByName, %s", err.Error())
				}
			}
			DeviceType := "CDROM"
			AdapterType := "SATA"
			DeviceIndex := int64(SATAindex)
			newDisk := v3.VMDisk{
				DeviceProperties: &v3.VMDiskDeviceProperties{
					DeviceType: &DeviceType,
					DiskAddress: &v3.DiskAddress{
						AdapterType: &AdapterType,
						DeviceIndex: &DeviceIndex,
					},
				},
				DataSourceReference: BuildReference(*image.Metadata.UUID, "image"),
			}
			DiskList = append(DiskList, &newDisk)
			SATAindex++
		}
	}

	state.Put("image_to_delete", imageToDelete)

	var cluster *v3.ClusterIntentResponse
	if vm.ClusterUUID != "" {
		cluster, err = conn.V3.GetCluster(ctx, vm.ClusterUUID)
		if err != nil {
			return nil, fmt.Errorf("error while GetCluster, %s", err.Error())
		}
	} else {
		cluster, err = findClusterByName(ctx, conn, vm.ClusterName)
		if err != nil {
			return nil, fmt.Errorf("error while findClusterByName, %s", err.Error())
		}
	}

	NICList := []*v3.VMNic{}
	for _, nic := range vm.VmNICs {
		var subnet *v3.SubnetIntentResponse
		if nic.SubnetUUID != "" {
			subnet, err = findSubnetByUUID(ctx, conn, nic.SubnetUUID)
			if err != nil {
				return nil, fmt.Errorf("error while findSubnetByUUID, %s", err.Error())
			}

			if subnet == nil {
				return nil, fmt.Errorf("subnet with UUID %s not found", nic.SubnetUUID)
			}
		} else if nic.SubnetName != "" {
			subnet, err = findSubnetByName(ctx, conn, nic.SubnetName, *cluster.Metadata.UUID)
			if err != nil {
				return nil, fmt.Errorf("error while findSubnetByName, %s", err.Error())
			}
		}

		isConnected := true
		newNIC := v3.VMNic{
			IsConnected:     &isConnected,
			SubnetReference: BuildReference(*subnet.Metadata.UUID, "subnet"),
		}
		NICList = append(NICList, &newNIC)
	}

	SerialIndex := 0
	SerialPortList := []*v3.VMSerialPort{}
	if vm.SerialPort {
		DeviceIndex := int64(SerialIndex)
		isConnected := true
		newVMSerialPort := v3.VMSerialPort{
			Index:       &DeviceIndex,
			IsConnected: &isConnected,
		}
		SerialPortList = append(SerialPortList, &newVMSerialPort)
	}

	GPUList := make([]*v3.VMGpu, 0, len(vm.GPU))
	for _, gpu := range vm.GPU {
		vmGPU, err := findGPUByName(ctx, conn, gpu.Name)
		if err != nil {
			return nil, fmt.Errorf("error while findGPUByName %s", err.Error())
		}
		GPUList = append(GPUList, vmGPU)
	}

	powerStateOn := "ON"
	req := &v3.VMIntentInput{
		Spec: &v3.VM{
			Name: &vm.VMName,
			Resources: &v3.VMResources{
				GuestCustomization: guestCustomization,
				NumSockets:         &vm.CPU,
				NumVcpusPerSocket:  &vm.Core,
				MemorySizeMib:      &vm.MemoryMB,
				PowerState:         &powerStateOn,
				DiskList:           DiskList,
				NicList:            NICList,
				GpuList:            GPUList,
				SerialPortList:     SerialPortList,
			},
			ClusterReference: BuildReference(*cluster.Metadata.UUID, "cluster"),
			Description:      StringPtr(fmt.Sprintf(vmDescription, d.Config.VmConfig.ImageName)),
		},
		Metadata: &v3.Metadata{
			Kind: StringPtr("vm"),
		},
	}

	var bootType string

	if vm.BootType == NutanixIdentifierBootTypeUEFI {
		bootType = strings.ToUpper(NutanixIdentifierBootTypeUEFI)

	} else {
		bootType = strings.ToUpper(NutanixIdentifierBootTypeLegacy)
	}

	var bootDeviceOrderList []*string

	if vm.BootPriority == "cdrom" {
		bootDeviceOrderList = []*string{
			StringPtr("CDROM"),
			StringPtr("DISK"),
			StringPtr("NETWORK"),
		}
	} else {
		bootDeviceOrderList = []*string{
			StringPtr("DISK"),
			StringPtr("CDROM"),
			StringPtr("NETWORK"),
		}
	}

	req.Spec.Resources.BootConfig = &v3.VMBootConfig{
		BootType:            &bootType,
		BootDeviceOrderList: bootDeviceOrderList,
	}

	if len(vm.VMCategories) != 0 {
		c := make(map[string]string)
		for _, category := range vm.VMCategories {
			c[category.Key] = category.Value
		}
		req.Metadata.Categories = c
	}

	if vm.Project != "" {
		project, err := findProjectByName(ctx, conn, vm.Project)
		if err != nil {
			return nil, fmt.Errorf("error while findProjectByName, %s", err.Error())
		}

		req.Metadata.ProjectReference = &v3.Reference{
			Kind: StringPtr("project"),
			UUID: project.Metadata.UUID,
		}
	}

	return req, nil

}

func (d *NutanixDriver) Create(ctx context.Context, req *v3.VMIntentInput) (*nutanixInstance, error) {

	configCreds := client.Credentials{
		URL:      fmt.Sprintf("%s:%d", d.ClusterConfig.Endpoint, d.ClusterConfig.Port),
		Endpoint: d.ClusterConfig.Endpoint,
		Username: d.ClusterConfig.Username,
		Password: d.ClusterConfig.Password,
		Port:     string(d.ClusterConfig.Port),
		Insecure: d.ClusterConfig.Insecure,
	}

	conn, err := v3.NewV3Client(configCreds)
	if err != nil {
		return nil, err
	}

	log.Printf("creating vm %s...", d.Config.VMName)

	resp, err := conn.V3.CreateVM(ctx, req)
	if err != nil {
		log.Printf("error creating vm: %s", err.Error())
		return nil, err
	}

	uuid := *resp.Metadata.UUID

	err = checkTask(ctx, conn, resp.Status.ExecutionContext.TaskUUID.(string), 600)

	if err != nil {
		log.Printf("error creating vm: %s", err.Error())
		return nil, err
	}

	log.Print("vm succesfully created")

	var vm *v3.VMIntentResponse
	vm, err = conn.V3.GetVM(ctx, uuid)
	if err != nil {
		log.Printf("error getting vm: %s", err.Error())
		return nil, err
	}

	return &nutanixInstance{nutanix: *vm}, nil
}

// WaitForIP waits for the virtual machine to obtain an IP address.
func (d *NutanixDriver) WaitForIP(ctx context.Context, uuid string, ipNet *net.IPNet) (string, error) {

	configCreds := client.Credentials{
		URL:      fmt.Sprintf("%s:%d", d.ClusterConfig.Endpoint, d.ClusterConfig.Port),
		Endpoint: d.ClusterConfig.Endpoint,
		Username: d.ClusterConfig.Username,
		Password: d.ClusterConfig.Password,
		Port:     string(d.ClusterConfig.Port),
		Insecure: d.ClusterConfig.Insecure,
	}

	conn, err := v3.NewV3Client(configCreds)
	if err != nil {
		return "", err
	}

	var IPAddress string

	var vm *v3.VMIntentResponse
	for {
		vm, err = conn.V3.GetVM(ctx, uuid)
		if err != nil {
			log.Printf("error getting vm: %s", err.Error())
			return "", err
		}
		if len(vm.Status.Resources.NicList) > 0 &&
			len(vm.Status.Resources.NicList[0].IPEndpointList) > 0 &&
			vm.Status.Resources.NicList[0].IPEndpointList[0].IP != nil &&
			*vm.Status.Resources.NicList[0].IPEndpointList[0].IP != "" {
			break
		}
		time.Sleep(5 * time.Second)
	}

	IPAddress = *vm.Status.Resources.NicList[0].IPEndpointList[0].IP
	log.Printf("VM (%s) configured with ip address %s", uuid, IPAddress)

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

	// Unable to find an IP address.

}

func (d *NutanixDriver) Delete(ctx context.Context, vmUUID string) error {

	configCreds := client.Credentials{
		URL:      fmt.Sprintf("%s:%d", d.ClusterConfig.Endpoint, d.ClusterConfig.Port),
		Endpoint: d.ClusterConfig.Endpoint,
		Username: d.ClusterConfig.Username,
		Password: d.ClusterConfig.Password,
		Port:     string(d.ClusterConfig.Port),
		Insecure: d.ClusterConfig.Insecure,
	}

	conn, err := v3.NewV3Client(configCreds)
	if err != nil {
		return err
	}

	_, err = conn.V3.DeleteVM(ctx, vmUUID)
	if err != nil {
		return err
	}
	return nil
}

// CreateImageURL (VmDisk, VmConfig) (*nutanixImage, error)
func (d *NutanixDriver) CreateImageURL(ctx context.Context, disk VmDisk, vm VmConfig) (*nutanixImage, error) {
	configCreds := client.Credentials{
		URL:      fmt.Sprintf("%s:%d", d.ClusterConfig.Endpoint, d.ClusterConfig.Port),
		Endpoint: d.ClusterConfig.Endpoint,
		Username: d.ClusterConfig.Username,
		Password: d.ClusterConfig.Password,
		Port:     string(d.ClusterConfig.Port),
		Insecure: d.ClusterConfig.Insecure,
	}

	conn, err := v3.NewV3Client(configCreds)
	if err != nil {
		return nil, err
	}

	_, file := path.Split(disk.SourceImageURI)

	cluster := &v3.ClusterIntentResponse{}
	if vm.ClusterUUID != "" {
		cluster, err = conn.V3.GetCluster(ctx, vm.ClusterUUID)
		if err != nil {
			return nil, fmt.Errorf("error while GetCluster, %s", err.Error())
		}
	} else if vm.ClusterName != "" {
		cluster, err = findClusterByName(ctx, conn, vm.ClusterName)
		if err != nil {
			return nil, fmt.Errorf("error while findClusterByName, %s", err.Error())
		}
	}

	refvalue := BuildReferenceValue(*cluster.Metadata.UUID, "cluster")
	InitialPlacementRef := []*v3.ReferenceValues{refvalue}
	req := &v3.ImageIntentInput{
		Spec: &v3.Image{
			Name: &file,
			Resources: &v3.ImageResources{
				ImageType:               &disk.ImageType,
				InitialPlacementRefList: InitialPlacementRef,
			},
			Description: StringPtr(defaultImageDLDescription),
		},
		Metadata: &v3.Metadata{
			Kind: StringPtr("image"),
		},
	}

	existingImage, err := sourceImageExists(ctx, conn, file, disk.SourceImageURI)
	if err != nil {
		return nil, fmt.Errorf("error while checking if image exists, %s", err.Error())
	}
	if existingImage != nil && !disk.SourceImageForce {
		log.Printf("reuse existing image: %s", *existingImage.Status.Name)
		return &nutanixImage{image: *existingImage}, nil
	} else if existingImage != nil && disk.SourceImageForce {
		log.Printf("delete existing image: %s", *existingImage.Status.Name)
		d.DeleteImage(ctx, *existingImage.Metadata.UUID)
		log.Printf("recreating image: %s", file)
	} else if existingImage == nil {
		log.Printf("creating image: %s", file)
	}
	req.Spec.Resources.SourceURI = &disk.SourceImageURI

	if disk.SourceImageChecksum != "" {

		req.Spec.Resources.Checksum = &v3.Checksum{
			ChecksumValue: &disk.SourceImageChecksum,
		}

		if disk.SourceImageChecksumType == NutanixIdentifierChecksunTypeSHA256 {
			req.Spec.Resources.Checksum.ChecksumAlgorithm = StringPtr("SHA_256")
		} else if disk.SourceImageChecksumType == NutanixIdentifierChecksunTypeSHA1 {
			req.Spec.Resources.Checksum.ChecksumAlgorithm = StringPtr("SHA_1")
		}

	}

	image, err := conn.V3.CreateImage(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("error while creating image: %s", err.Error())
	}

	err = checkTask(ctx, conn, image.Status.ExecutionContext.TaskUUID.(string), 600)
	if err != nil {
		return nil, fmt.Errorf("error while creating image: %s", err.Error())
	}
	log.Printf("image succesfully created")

	return &nutanixImage{image: *image}, nil
}

// CreateImageFile (VmDisk, VmConfig) (*nutanixImage, error)
func (d *NutanixDriver) CreateImageFile(ctx context.Context, filePath string, vm VmConfig) (*nutanixImage, error) {
	configCreds := client.Credentials{
		URL:      fmt.Sprintf("%s:%d", d.ClusterConfig.Endpoint, d.ClusterConfig.Port),
		Endpoint: d.ClusterConfig.Endpoint,
		Username: d.ClusterConfig.Username,
		Password: d.ClusterConfig.Password,
		Port:     string(d.ClusterConfig.Port),
		Insecure: d.ClusterConfig.Insecure,
	}

	conn, err := v3.NewV3Client(configCreds)
	if err != nil {
		return nil, err
	}

	_, file := path.Split(filePath)

	cluster := &v3.ClusterIntentResponse{}
	if vm.ClusterUUID != "" {
		cluster, err = conn.V3.GetCluster(ctx, vm.ClusterUUID)
		if err != nil {
			return nil, fmt.Errorf("error while GetCluster, %s", err.Error())
		}
	} else if vm.ClusterName != "" {
		cluster, err = findClusterByName(ctx, conn, vm.ClusterName)
		if err != nil {
			return nil, fmt.Errorf("error while findClusterByName, %s", err.Error())
		}
	}

	refvalue := BuildReferenceValue(*cluster.Metadata.UUID, "cluster")
	InitialPlacementRef := []*v3.ReferenceValues{refvalue}
	req := &v3.ImageIntentInput{
		Spec: &v3.Image{
			Name: &file,
			Resources: &v3.ImageResources{
				ImageType:               StringPtr("ISO_IMAGE"),
				InitialPlacementRefList: InitialPlacementRef,
			},
			Description: StringPtr(defaultImageDLDescription),
		},
		Metadata: &v3.Metadata{
			Kind: StringPtr("image"),
		},
	}

	log.Printf("creating image: %s", file)
	image, err := conn.V3.CreateImage(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("error while creating image: %s", err.Error())
	}

	err = checkTask(ctx, conn, image.Status.ExecutionContext.TaskUUID.(string), 600)
	if err != nil {
		return nil, fmt.Errorf("error while creating image: %s", err.Error())
	}

	log.Printf("uploading image: %s", filePath)
	err = conn.V3.UploadImage(ctx, *image.Metadata.UUID, filePath)
	if err != nil {
		return nil, fmt.Errorf("error while uploading image: %s", err.Error())
	}

	running, err := conn.V3.GetImage(ctx, *image.Metadata.UUID)
	if err != nil || *running.Status.State != "COMPLETE" {
		return nil, fmt.Errorf("error while upload image: %s", err.Error())
	}

	return &nutanixImage{image: *image}, nil

}

func (d *NutanixDriver) DeleteImage(ctx context.Context, imageUUID string) error {
	configCreds := client.Credentials{
		URL:      fmt.Sprintf("%s:%d", d.ClusterConfig.Endpoint, d.ClusterConfig.Port),
		Endpoint: d.ClusterConfig.Endpoint,
		Username: d.ClusterConfig.Username,
		Password: d.ClusterConfig.Password,
		Port:     string(d.ClusterConfig.Port),
		Insecure: d.ClusterConfig.Insecure,
	}

	conn, err := v3.NewV3Client(configCreds)
	if err != nil {
		return fmt.Errorf("error while creating new client connection, %s", err.Error())
	}
	_, err = conn.V3.DeleteImage(ctx, imageUUID)
	if err != nil {
		return fmt.Errorf("error while deleting image, %s", err.Error())
	}
	return nil
}

func (d *NutanixDriver) GetImage(ctx context.Context, imagename string) (*nutanixImage, error) {
	configCreds := client.Credentials{
		URL:      fmt.Sprintf("%s:%d", d.ClusterConfig.Endpoint, d.ClusterConfig.Port),
		Endpoint: d.ClusterConfig.Endpoint,
		Username: d.ClusterConfig.Username,
		Password: d.ClusterConfig.Password,
		Port:     string(d.ClusterConfig.Port),
		Insecure: d.ClusterConfig.Insecure,
	}

	conn, err := v3.NewV3Client(configCreds)
	if err != nil {
		return nil, fmt.Errorf("error while NewV3Client, %s", err.Error())
	}

	image, err := findImageByName(ctx, conn, imagename)
	if err != nil {
		return nil, fmt.Errorf("error while GetImage, %s", err.Error())
	}
	return &nutanixImage{image: *image}, nil
}

func (d *NutanixDriver) GetVM(ctx context.Context, vmUUID string) (*nutanixInstance, error) {
	configCreds := client.Credentials{
		URL:      fmt.Sprintf("%s:%d", d.ClusterConfig.Endpoint, d.ClusterConfig.Port),
		Endpoint: d.ClusterConfig.Endpoint,
		Username: d.ClusterConfig.Username,
		Password: d.ClusterConfig.Password,
		Port:     string(d.ClusterConfig.Port),
		Insecure: d.ClusterConfig.Insecure,
	}

	conn, err := v3.NewV3Client(configCreds)
	if err != nil {
		return nil, fmt.Errorf("error while NewV3Client, %s", err.Error())
	}

	vm, err := conn.V3.GetVM(ctx, vmUUID)
	if err != nil {
		return nil, fmt.Errorf("error while GetVM, %s", err.Error())
	}
	return &nutanixInstance{nutanix: *vm}, nil
}

func (d *NutanixDriver) getRequest(ctx context.Context, url string) (*http.Response, error) {
	customTransport := http.DefaultTransport.(*http.Transport).Clone()
	customTransport.TLSClientConfig = &tls.Config{InsecureSkipVerify: d.ClusterConfig.Insecure}
	httpClient := &http.Client{Transport: customTransport}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req = req.WithContext(ctx)
	req.SetBasicAuth(d.ClusterConfig.Username, d.ClusterConfig.Password)
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf(resp.Status)
	}
	return resp, nil
}

func (d *NutanixDriver) postRequest(ctx context.Context, url string, payload map[string]string) (*http.Response, error) {
	customTransport := http.DefaultTransport.(*http.Transport).Clone()
	customTransport.TLSClientConfig = &tls.Config{InsecureSkipVerify: d.ClusterConfig.Insecure}
	httpClient := &http.Client{Transport: customTransport}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	req = req.WithContext(ctx)
	req.SetBasicAuth(d.ClusterConfig.Username, d.ClusterConfig.Password)
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode < 200 || resp.StatusCode > 300 {
		err_return := fmt.Errorf(resp.Status)
		bodyBytes, err := io.ReadAll(resp.Body)
		if err == nil {
			return nil, fmt.Errorf("request returned non-200 status: %s, Response Body: %s", resp.Status, string(bodyBytes))
		}
		return nil, err_return
	}
	return resp, nil
}

func GetLatestOVAByName(ctx context.Context, entityType string, name string, conn *v3.Client) string {
	request := v3.GroupsGetEntitiesRequest{
		EntityType:     &entityType,
		FilterCriteria: fmt.Sprintf(`name==%s`, name),
	}

	var response *v3.GroupsGetEntitiesResponse
	response, err := conn.V3.GroupsGetEntities(ctx, &request)

	if err != nil {
		if response != nil {
			log.Printf("Partial response: %+v", response)
		}
	} else {
		groupResults := response.GroupResults
		if len(groupResults) > 0 {
			entityList := groupResults[0].EntityResults
			if len(entityList) > 0 {
				var latestEntity *v3.GroupsEntity
				var latestTime int64

				for _, entity := range entityList {
					for _, field := range entity.Data {
						for _, val := range field.Values {
							if val.Time > latestTime {
								latestTime = val.Time
								latestEntity = entity
							}
						}
					}
				}

				if latestEntity != nil {
					return latestEntity.EntityID
				}
			}
		}
	}
	return ""
}

func (d *NutanixDriver) CreateOVA(ctx context.Context, ovaName string, vmUUID string, diskFileFormat string) error {
	url := fmt.Sprintf("https://%s:%d/api/nutanix/v3/vms/%s/export", d.ClusterConfig.Endpoint, d.ClusterConfig.Port, vmUUID)
	log.Printf("export ova using api: %s", url)

	payload := map[string]string{
		"name":             ovaName,
		"disk_file_format": strings.ToUpper(diskFileFormat),
	}

	resp, err := d.postRequest(ctx, url, payload)
	if err != nil {
		return err
	}

	var result struct {
		TaskUUID string `json:"task_uuid"`
	}
	err = json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		return err
	}
	log.Printf("export task created with Task UUID: %s", result.TaskUUID)

	configCreds := client.Credentials{
		URL:      fmt.Sprintf("%s:%d", d.ClusterConfig.Endpoint, d.ClusterConfig.Port),
		Endpoint: d.ClusterConfig.Endpoint,
		Username: d.ClusterConfig.Username,
		Password: d.ClusterConfig.Password,
		Port:     string(d.ClusterConfig.Port),
		Insecure: d.ClusterConfig.Insecure,
	}

	conn, err := v3.NewV3Client(configCreds)
	if err != nil {
		return err
	}

	err = checkTask(ctx, conn, result.TaskUUID, 3600)
	if err != nil {
		return err
	}

	log.Printf("OVA export task %s completed successfully.", result.TaskUUID)
	return nil
}

func (d *NutanixDriver) ExportOVA(ctx context.Context, ovaName string) (io.ReadCloser, error) {
	log.Printf("starting OVA export for OVA: %s", ovaName)
	configCreds := client.Credentials{
		URL:      fmt.Sprintf("%s:%d", d.ClusterConfig.Endpoint, d.ClusterConfig.Port),
		Endpoint: d.ClusterConfig.Endpoint,
		Username: d.ClusterConfig.Username,
		Password: d.ClusterConfig.Password,
		Port:     string(d.ClusterConfig.Port),
		Insecure: d.ClusterConfig.Insecure,
	}

	conn, err := v3.NewV3Client(configCreds)
	if err != nil {
		return nil, fmt.Errorf("error while NewV3Client, %s", err.Error())
	}

	var ovaUUID string

	// Recheck for a little while to make sure the OVA with name ovaName appears
	for i := 0; i < (60 / 5); i++ {
		ovaUUID = GetLatestOVAByName(ctx, "ova", ovaName, conn)
		if ovaUUID == "" {
			<-time.After(5 * time.Second)
		}
	}

	if ovaUUID == "" {
		return nil, fmt.Errorf("timeout waiting for OVA entity to appear")
	}

	ova_download_url := fmt.Sprintf("https://%s:%d/api/nutanix/v3/ovas/%s/file", d.ClusterConfig.Endpoint, d.ClusterConfig.Port, ovaUUID)
	log.Printf("The ova download url is: %s", ova_download_url)
	ova_resp, err := d.getRequest(ctx, ova_download_url)
	if err != nil {
		return nil, err
	}
	return ova_resp.Body, nil
}

func (d *NutanixDriver) ExportImage(ctx context.Context, imageUUID string) (io.ReadCloser, error) {
	customTransport := http.DefaultTransport.(*http.Transport).Clone()
	customTransport.TLSClientConfig = &tls.Config{InsecureSkipVerify: d.ClusterConfig.Insecure}
	httpClient := &http.Client{Transport: customTransport}

	url := fmt.Sprintf("https://%s:%d/api/nutanix/v3/images/%s/file", d.ClusterConfig.Endpoint, d.ClusterConfig.Port, imageUUID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	req = req.WithContext(ctx)
	req.SetBasicAuth(d.ClusterConfig.Username, d.ClusterConfig.Password)
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf(resp.Status)
	}

	return resp.Body, nil
}

func (d *NutanixDriver) GetHost(ctx context.Context, hostUUID string) (*nutanixHost, error) {

	configCreds := client.Credentials{
		URL:      fmt.Sprintf("%s:%d", d.ClusterConfig.Endpoint, d.ClusterConfig.Port),
		Endpoint: d.ClusterConfig.Endpoint,
		Username: d.ClusterConfig.Username,
		Password: d.ClusterConfig.Password,
		Port:     string(d.ClusterConfig.Port),
		Insecure: d.ClusterConfig.Insecure,
	}

	conn, err := v3.NewV3Client(configCreds)
	if err != nil {
		return nil, fmt.Errorf("error while NewV3Client, %s", err.Error())
	}

	host, err := conn.V3.GetHost(ctx, hostUUID)
	if err != nil {
		return nil, fmt.Errorf("error while GetHost, %s", err.Error())
	}
	return &nutanixHost{host: *host}, nil
}

func (d *NutanixDriver) PowerOff(ctx context.Context, vmUUID string) error {
	configCreds := client.Credentials{
		URL:      fmt.Sprintf("%s:%d", d.ClusterConfig.Endpoint, d.ClusterConfig.Port),
		Endpoint: d.ClusterConfig.Endpoint,
		Username: d.ClusterConfig.Username,
		Password: d.ClusterConfig.Password,
		Port:     string(d.ClusterConfig.Port),
		Insecure: d.ClusterConfig.Insecure,
	}

	conn, err := v3.NewV3Client(configCreds)
	if err != nil {
		return fmt.Errorf("error while NewV3Client, %s", err.Error())
	}
	vmResp, err := conn.V3.GetVM(ctx, vmUUID)
	if err != nil {
		return fmt.Errorf("error while GetVM, %s", err.Error())
	}

	// Prepare VM update request
	request := &v3.VMIntentInput{}
	request.Spec = vmResp.Spec
	request.Metadata = vmResp.Metadata
	request.Spec.Resources.PowerState = StringPtr("OFF")

	resp, err := conn.V3.UpdateVM(ctx, vmUUID, request)
	if err != nil {
		return fmt.Errorf("error while UpdateVM, %s", err.Error())
	}

	taskUUID := resp.Status.ExecutionContext.TaskUUID.(string)

	// Wait for the VM to be stopped
	log.Printf("stopping VM: %s", d.Config.VMName)
	err = checkTask(ctx, conn, taskUUID, 600)
	if err != nil {
		return fmt.Errorf("error while stopping VM: %s", err.Error())
	}

	return nil
}
func (d *NutanixDriver) SaveVMDisk(ctx context.Context, diskUUID string, index int, imageCategories []Category) (*nutanixImage, error) {

	configCreds := client.Credentials{
		URL:      fmt.Sprintf("%s:%d", d.ClusterConfig.Endpoint, d.ClusterConfig.Port),
		Endpoint: d.ClusterConfig.Endpoint,
		Username: d.ClusterConfig.Username,
		Password: d.ClusterConfig.Password,
		Port:     string(d.ClusterConfig.Port),
		Insecure: d.ClusterConfig.Insecure,
	}

	conn, err := v3.NewV3Client(configCreds)
	if err != nil {
		return nil, fmt.Errorf("error while NewV3Client, %s", err.Error())
	}

	name := d.Config.VmConfig.ImageName
	if index > 0 {
		name = fmt.Sprintf("%s-disk%d", name, index+1)
	}

	// When force_deregister, check if image already exists
	if d.Config.ForceDeregister {
		log.Println("force_deregister is set, check if image already exists")
		resp, err := conn.V3.ListAllImage(ctx, "")
		if err != nil {
			return nil, fmt.Errorf("error while ListAllImage, %s", err.Error())
		}

		entities := resp.Entities

		found := make([]*v3.ImageIntentResponse, 0)
		for _, v := range entities {
			if strings.EqualFold(*v.Spec.Name, name) {
				found = append(found, v)
			}
		}

		if len(found) == 0 {
			log.Println("image with given Name not found, no need to deregister")
		} else if len(found) > 1 {
			log.Println("more than one image with given Name found, will not deregister")
		} else if len(found) == 1 {
			log.Println("exactly one image with given Name found, will deregister")

			resp, err := conn.V3.DeleteImage(ctx, *found[0].Metadata.UUID)
			if err != nil {
				return nil, fmt.Errorf("error while Deleting Image, %s", err.Error())
			}

			log.Printf("deleting image %s...\n", *found[0].Metadata.UUID)
			err = checkTask(ctx, conn, resp.Status.ExecutionContext.TaskUUID.(string), 600)

			if err != nil {
				return nil, fmt.Errorf("error while Deleting Image, %s", err.Error())
			}
		}
	}

	imgDescription := defaultImageBuiltDescription
	if d.Config.ImageDescription != "" {
		imgDescription = d.Config.ImageDescription
	}

	req := &v3.ImageIntentInput{
		Spec: &v3.Image{
			Name: &name,
			Resources: &v3.ImageResources{
				ImageType:           StringPtr("DISK_IMAGE"),
				DataSourceReference: BuildReference(diskUUID, "vm_disk"),
			},
			Description: StringPtr(imgDescription),
		},
		Metadata: &v3.Metadata{
			Kind: StringPtr("image"),
		},
	}

	if len(imageCategories) != 0 {
		c := make(map[string]string)
		for _, category := range imageCategories {
			c[category.Key] = category.Value
		}
		req.Metadata.Categories = c
	}

	image, err := conn.V3.CreateImage(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("error while Creating Image, %s", err.Error())
	}
	log.Printf("creating image %s...\n", *image.Metadata.UUID)
	err = checkTask(ctx, conn, image.Status.ExecutionContext.TaskUUID.(string), 600)
	if err != nil {
		return nil, fmt.Errorf("error while Creating Image, %s", err.Error())
	} else {
		return &nutanixImage{image: *image}, nil
	}

}
