package nutanix

import (
	"time"

	"fmt"
	"log"
	"path"

	client "github.com/nutanix-cloud-native/prism-go-client/pkg/nutanix"
	v3 "github.com/nutanix-cloud-native/prism-go-client/pkg/nutanix/v3"
)

// A driver is able to talk to Nutanix PrismCentral and perform certain
// operations with it.
type Driver interface {
	CreateRequest(VmConfig) (*v3.VMIntentInput, error)
	Create(*v3.VMIntentInput) (*nutanixInstance, error)
	Delete(string) error
	GetVM(string) (*nutanixInstance, error)
	//GetImage(string) (*nutanixImage, error)
	GetHost(string) (*nutanixHost, error)
	PowerOff(string) error
	UploadImage(string, VmConfig) (*nutanixImage, error)
	DeleteImage(string) error
	SaveVMDisk(string, string) (*nutanixImage, error)
	WaitForShutdown(string, <-chan struct{}) bool
}

type NutanixDriver struct {
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

func findClusterByName(conn *v3.Client, name string) (*v3.ClusterIntentResponse, error) {
	filter := fmt.Sprintf("name==%s", name)
	resp, err := conn.V3.ListAllCluster(filter)
	if err != nil {
		return nil, err
	}
	entities := resp.Entities

	found := make([]*v3.ClusterIntentResponse, 0)
	for _, v := range entities {
		if *v.Status.Name == name {
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

func findSubnetByUUID(conn *v3.Client, uuid string) (*v3.SubnetIntentResponse, error) {
	return conn.V3.GetSubnet(uuid)
}

func findSubnetByName(conn *v3.Client, name string) (*v3.SubnetIntentResponse, error) {
	filter := fmt.Sprintf("name==%s", name)
	resp, err := conn.V3.ListAllSubnet(filter, getEmptyClientSideFilter())
	if err != nil {
		return nil, err
	}

	entities := resp.Entities

	found := make([]*v3.SubnetIntentResponse, 0)
	for _, v := range entities {
		if *v.Spec.Name == name {
			found = append(found, v)
		}
	}

	if len(found) > 1 {
		return nil, fmt.Errorf("your query returned more than one result. Please use subnet_uuid argument or use additional filters instead")
	}

	if len(found) == 0 {
		return nil, fmt.Errorf("subnet with the given name, not found")
	}

	return found[0], nil
}

func findImageByUUID(conn *v3.Client, uuid string) (*v3.ImageIntentResponse, error) {
	return conn.V3.GetImage(uuid)
}

func findImageByName(conn *v3.Client, name string) (*v3.ImageIntentResponse, error) {
	filter := fmt.Sprintf("name==%s", name)
	resp, err := conn.V3.ListAllImage(filter)
	if err != nil {
		return nil, err
	}

	entities := resp.Entities

	found := make([]*v3.ImageIntentResponse, 0)
	for _, v := range entities {
		if *v.Spec.Name == name {
			found = append(found, v)
		}
	}

	if len(found) > 1 {
		return nil, fmt.Errorf("your query returned more than one result. Please use soure_image_uuid argument instead")
	}

	if len(found) == 0 {
		return nil, fmt.Errorf("image with the given name, not found")
	}

	return findImageByUUID(conn, *found[0].Metadata.UUID)
}

func (d *nutanixInstance) Addresses() []string {
	var addresses []string
	if len(d.nutanix.Spec.Resources.NicList) > 0 {
		for _, n := range d.nutanix.Spec.Resources.NicList {
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
func (d *NutanixDriver) CreateRequest(vm VmConfig) (*v3.VMIntentInput, error) {

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
	for _, disk := range vm.VmDisks {
		if disk.ImageType == "DISK_IMAGE" {
			image := &v3.ImageIntentResponse{}
			if disk.SourceImageUUID != "" {
				image, err = findImageByUUID(conn, disk.SourceImageUUID)
				if err != nil {
					return nil, fmt.Errorf("image not found %s", disk.SourceImageUUID)
				}
			} else if disk.SourceImageName != "" {
				image, err = findImageByName(conn, disk.SourceImageName)
				if err != nil {
					return nil, fmt.Errorf("image not found %s", disk.SourceImageName)
				}
			}

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
			if disk.SourceImageUUID != "" {
				image, err = findImageByUUID(conn, disk.SourceImageUUID)
				if err != nil {
					return nil, fmt.Errorf("image not found %s", disk.SourceImageUUID)
				}
			} else if disk.SourceImageName != "" {
				image, err = findImageByName(conn, disk.SourceImageName)
				if err != nil {
					return nil, fmt.Errorf("image not found %s", disk.SourceImageName)
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

	NICList := []*v3.VMNic{}
	for _, nic := range vm.VmNICs {
		subnet := &v3.SubnetIntentResponse{}
		if nic.SubnetUUID != "" {
			subnet, err = findSubnetByUUID(conn, nic.SubnetUUID)
			if err != nil {
				return nil, fmt.Errorf("subnet not found %s", nic.SubnetUUID)
			}
		} else if nic.SubnetName != "" {
			subnet, err = findSubnetByName(conn, nic.SubnetName)
			if err != nil {
				return nil, fmt.Errorf("subnet not found %s", nic.SubnetName)
			}
		}

		isConnected := true
		newNIC := v3.VMNic{
			IsConnected:     &isConnected,
			SubnetReference: BuildReference(*subnet.Metadata.UUID, "subnet"),
		}
		NICList = append(NICList, &newNIC)
	}
	PowerStateOn := "ON"

	cluster := &v3.ClusterIntentResponse{}
	if vm.ClusterUUID != "" {
		cluster, err = conn.V3.GetCluster(vm.ClusterUUID)
		if err != nil {
			return nil, fmt.Errorf("cluster not found %s", vm.ClusterUUID)
		}
	} else if vm.ClusterName != "" {
		cluster, err = findClusterByName(conn, vm.ClusterName)
		if err != nil {
			return nil, fmt.Errorf("cluster not found %s", vm.ClusterName)
		}
	}

	req := &v3.VMIntentInput{
		Spec: &v3.VM{
			Name: &vm.VMName,
			Resources: &v3.VMResources{
				GuestCustomization: guestCustomization,
				MemorySizeMib:      &vm.MemoryMB,
				PowerState:         &PowerStateOn,
				DiskList:           DiskList,
				NicList:            NICList,
			},
			ClusterReference: BuildReference(*cluster.Metadata.UUID, "cluster"),
		},
		Metadata: &v3.Metadata{
			Kind: StringPtr("vm"),
		},
	}

	return req, nil

}

func (d *NutanixDriver) Create(req *v3.VMIntentInput) (*nutanixInstance, error) {

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

	resp, err := conn.V3.CreateVM(req)
	if err != nil {
		log.Printf("Error creating vm: [%v]", err)
		return nil, err
	}

	uuid := *resp.Metadata.UUID
	taskUUID := resp.Status.ExecutionContext.TaskUUID.(string)

	log.Printf("waiting for vm (%s) to create: %s", uuid, taskUUID)

	for {
		vm, err := conn.V3.GetVM(uuid)
		if err == nil {
			if *vm.Status.State == "COMPLETE" {
				return &nutanixInstance{nutanix: *vm}, err

			} else if *vm.Status.State == "ERROR" {
				var errTxt string
				for i := 0; i < len(vm.Status.MessageList); i++ {
					errTxt = *(vm.Status.MessageList)[i].Message
					log.Printf("Nutanix Error Message: %s", *(vm.Status.MessageList)[i].Message)
					log.Printf("Nutanix Error Reason: %s", *(vm.Status.MessageList)[i].Reason)
					log.Printf("Nutanix Error Details: %s", (vm.Status.MessageList)[i].Details)
				}
				return nil, fmt.Errorf(errTxt)
			} else {
				log.Printf("Current status is: " + *vm.Status.State)
				time.Sleep(5 * time.Second)
			}
		} else {
			log.Printf("Error while getting VM Status")
			return nil, err
		}
	}

}

func (d *NutanixDriver) Delete(vmUUID string) error {

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

	_, err = conn.V3.DeleteVM(vmUUID)
	if err != nil {
		return err
	}
	return nil
}

//UploadImage (string, VmConfig) (*nutanixImage, error)
func (d *NutanixDriver) UploadImage(imagePath string, vm VmConfig) (*nutanixImage, error) {
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

	_, file := path.Split(imagePath)

	cluster := &v3.ClusterIntentResponse{}
	if vm.ClusterUUID != "" {
		cluster, err = conn.V3.GetCluster(vm.ClusterUUID)
		if err != nil {
			return nil, fmt.Errorf("cluster not found %s", vm.ClusterUUID)
		}
	} else if vm.ClusterName != "" {
		cluster, err = findClusterByName(conn, vm.ClusterName)
		if err != nil {
			return nil, fmt.Errorf("cluster not found %s", vm.ClusterName)
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
		},
		Metadata: &v3.Metadata{
			Kind: StringPtr("image"),
		},
	}

	image, err := conn.V3.CreateImage(req)
	if err != nil {
		log.Printf("Error while Image Create, %s", err.Error())
		return nil, err
	}

	for {
		running, err := conn.V3.GetImage(*image.Metadata.UUID)
		if err != nil {
			log.Printf("Error while retrieve image create status, %s", err.Error())
			return nil, err
		}
		log.Printf("Creating Image: %s", *running.Status.State)
		if *running.Status.State == "COMPLETE" {
			break
		}
		time.Sleep(5 * time.Second)
	}

	err = conn.V3.UploadImage(*image.Metadata.UUID, imagePath)
	if err != nil {
		log.Printf("Error while upload, %s", err.Error())
		return nil, err
	}
	for {
		running, err := conn.V3.GetImage(*image.Metadata.UUID)
		if err != nil {
			log.Printf("Error while retrieve upload status, %s", err.Error())
			return nil, err
		}
		if *running.Status.State == "COMPLETE" {
			break
		}
		time.Sleep(5 * time.Second)
	}
	return &nutanixImage{image: *image}, nil

}

func (d *NutanixDriver) DeleteImage(imageUUID string) error {
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
		log.Printf("Error while creating new client connection, %s", err.Error())
		return err
	}
	_, err = conn.V3.DeleteImage(imageUUID)
	if err != nil {
		log.Printf("Error while deleting image, %s", err.Error())
		return err
	}
	return nil
}

func (d *NutanixDriver) GetVM(vmUUID string) (*nutanixInstance, error) {

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

	vm, err := conn.V3.GetVM(vmUUID)
	if err != nil {
		return nil, err
	}
	return &nutanixInstance{nutanix: *vm}, nil
}

func (d *NutanixDriver) GetHost(hostUUID string) (*nutanixHost, error) {

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

	host, err := conn.V3.GetHost(hostUUID)
	if err != nil {
		return nil, err
	}
	return &nutanixHost{host: *host}, nil
}

func (d *NutanixDriver) PowerOff(vmUUID string) error {
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
	vmResp, err := conn.V3.GetVM(vmUUID)
	if err != nil {
		return err
	}

	// Prepare VM update request
	request := &v3.VMIntentInput{}
	request.Spec = vmResp.Spec
	request.Metadata = vmResp.Metadata
	request.Spec.Resources.PowerState = StringPtr("OFF")

	resp, err := conn.V3.UpdateVM(vmUUID, request)
	if err != nil {
		return err
	}

	taskUUID := resp.Status.ExecutionContext.TaskUUID.(string)

	// Wait for the VM to be deleted
	for i := 0; i < 1200; i++ {
		resp, err := conn.V3.GetTask(taskUUID)
		if err != nil || *resp.Status != "SUCCEEDED" {
			<-time.After(1 * time.Second)
			continue
		}
		return err
	}

	log.Printf("PowerOff task: %s", taskUUID)
	return nil
}
func (d *NutanixDriver) SaveVMDisk(diskUUID string, imageName string) (*nutanixImage, error) {

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

	req := &v3.ImageIntentInput{
		Spec: &v3.Image{
			Name: &imageName,
			Resources: &v3.ImageResources{
				ImageType:           StringPtr("DISK_IMAGE"),
				DataSourceReference: BuildReference(diskUUID, "vm_disk"),
			},
		},
		Metadata: &v3.Metadata{
			Kind: StringPtr("image"),
		},
	}

	image, err := conn.V3.CreateImage(req)
	if err != nil {
		return nil, err
	}
	return &nutanixImage{image: *image}, nil
}

func getEmptyClientSideFilter() []*client.AdditionalFilter {
	return make([]*client.AdditionalFilter, 0)
}
