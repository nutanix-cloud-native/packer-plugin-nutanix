package nutanix

import (
	"context"
	"fmt"
	"strings"

	"github.com/nutanix-cloud-native/prism-go-client/converged"
	convergedv4 "github.com/nutanix-cloud-native/prism-go-client/converged/v4"
	clusterModels "github.com/nutanix/ntnx-api-golang-clients/clustermgmt-go-client/v4/models/clustermgmt/v4/config"
	subnetModels "github.com/nutanix/ntnx-api-golang-clients/networking-go-client/v4/models/networking/v4/config"
	vmmModels "github.com/nutanix/ntnx-api-golang-clients/vmm-go-client/v4/models/vmm/v4/ahv/config"
	imageModels "github.com/nutanix/ntnx-api-golang-clients/vmm-go-client/v4/models/vmm/v4/content"
)

// VM helpers

// findVMByUUID finds a VM by UUID using V4 API
//
//lint:ignore U1000 kept for future use
func findVMByUUID(ctx context.Context, client *convergedv4.Client, uuid string) (*vmmModels.Vm, error) {
	vm, err := client.VMs.Get(ctx, uuid)
	if err != nil {
		if strings.Contains(fmt.Sprint(err), "VM_NOT_FOUND") {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find VM by UUID %s: %s", uuid, err.Error())
	}
	return vm, nil
}

// findVMByName finds a VM by name using V4 API
//
//lint:ignore U1000 kept for future use
func findVMByName(ctx context.Context, client *convergedv4.Client, name string) (*vmmModels.Vm, error) {
	vms, err := client.VMs.List(ctx, converged.WithFilter(fmt.Sprintf("name eq '%s'", name)))
	if err != nil {
		return nil, err
	}

	if len(vms) > 1 {
		return nil, fmt.Errorf("found more than one VM with name %s", name)
	}

	if len(vms) == 0 {
		return nil, nil
	}

	// Get full VM details
	return findVMByUUID(ctx, client, *vms[0].ExtId)
}

// Image helpers

// findImageByUUIDHelper finds an image by UUID using V4 API
func findImageByUUIDHelper(ctx context.Context, client *convergedv4.Client, uuid string) (*imageModels.Image, error) {
	img, err := client.Images.Get(ctx, uuid)
	if err != nil {
		return nil, fmt.Errorf("failed to find image by UUID %s: %s", uuid, err.Error())
	}
	return img, nil
}

// findImageByNameHelper finds an image by name using V4 API
func findImageByNameHelper(ctx context.Context, client *convergedv4.Client, name string) (*imageModels.Image, error) {
	images, err := client.Images.List(ctx, converged.WithFilter(fmt.Sprintf("name eq '%s'", name)))
	if err != nil {
		return nil, err
	}

	found := make([]*imageModels.Image, 0)
	for i := range images {
		if images[i].Name != nil && strings.EqualFold(*images[i].Name, name) {
			found = append(found, &images[i])
		}
	}

	if len(found) > 1 {
		return nil, fmt.Errorf("found more than one image with name %s", name)
	}

	if len(found) == 0 {
		return nil, fmt.Errorf("image %s not found", name)
	}

	if found[0].ExtId == nil {
		return nil, fmt.Errorf("image %s has no ExtId", name)
	}
	return findImageByUUIDHelper(ctx, client, *found[0].ExtId)
}

// Cluster helpers

// isPECluster checks if a V4 cluster is a Prism Element (AOS) cluster
func isPECluster(cluster *clusterModels.Cluster) bool {
	if cluster.Config == nil || cluster.Config.ClusterFunction == nil {
		return false
	}
	for _, fn := range cluster.Config.ClusterFunction {
		if strings.EqualFold(fn.GetName(), clusterModels.CLUSTERFUNCTIONREF_AOS.GetName()) {
			return true
		}
	}
	return false
}

// findClusterByName finds a cluster by name using V4 API and returns its UUID
func findClusterByName(ctx context.Context, client *convergedv4.Client, name string) (string, error) {
	clusters, err := client.Clusters.List(ctx, converged.WithFilter(fmt.Sprintf("name eq '%s'", name)))
	if err != nil {
		return "", err
	}

	found := make([]clusterModels.Cluster, 0)
	for _, c := range clusters {
		if c.Name != nil && strings.EqualFold(*c.Name, name) && isPECluster(&c) {
			found = append(found, c)
		}
	}

	if len(found) > 1 {
		return "", fmt.Errorf("found more than one cluster with name %s", name)
	}

	if len(found) == 0 {
		return "", fmt.Errorf("cluster %s not found", name)
	}

	if found[0].ExtId == nil {
		return "", fmt.Errorf("cluster %s has no ExtId", name)
	}
	return *found[0].ExtId, nil
}

// getClusterUUID gets cluster UUID by name or UUID using V4 API
func getClusterUUID(ctx context.Context, client *convergedv4.Client, clusterName, clusterUUID string) (string, error) {
	if clusterUUID != "" {
		cluster, err := client.Clusters.Get(ctx, clusterUUID)
		if err != nil {
			return "", fmt.Errorf("failed to get cluster by UUID %s: %s", clusterUUID, err.Error())
		}
		return *cluster.ExtId, nil
	}

	if clusterName != "" {
		return findClusterByName(ctx, client, clusterName)
	}

	return "", fmt.Errorf("cluster name or UUID must be provided")
}

// Subnet helpers

const subnetTypeOverlay = "OVERLAY"

// subnetBelongsToCluster checks if a subnet belongs to the specified PE cluster
func subnetBelongsToCluster(subnet *subnetModels.Subnet, clusterUUID string) bool {
	if subnet.ClusterReference != nil && *subnet.ClusterReference == clusterUUID {
		return true
	}
	if subnet.ClusterReferenceList != nil {
		for _, ref := range subnet.ClusterReferenceList {
			if ref == clusterUUID {
				return true
			}
		}
	}
	return false
}

// getSubnetUUID gets subnet UUID by name or UUID using V4 API
func getSubnetUUID(ctx context.Context, client *convergedv4.Client, subnetName, subnetUUID, clusterUUID string) (string, error) {
	if subnetUUID != "" {
		subnet, err := client.Subnets.Get(ctx, subnetUUID)
		if err != nil {
			return "", fmt.Errorf("failed to find subnet with UUID %s: %s", subnetUUID, err.Error())
		}
		return *subnet.ExtId, nil
	}

	if subnetName != "" {
		subnets, err := client.Subnets.List(ctx, converged.WithFilter(fmt.Sprintf("name eq '%s'", subnetName)))
		if err != nil {
			return "", err
		}

		found := make([]subnetModels.Subnet, 0)
		for _, subnet := range subnets {
			if subnet.Name == nil || subnet.SubnetType == nil {
				continue
			}
			if strings.EqualFold(*subnet.Name, subnetName) {
				if subnet.SubnetType.GetName() == subnetTypeOverlay {
					found = append(found, subnet)
					continue
				}
				if subnetBelongsToCluster(&subnet, clusterUUID) {
					found = append(found, subnet)
				}
			}
		}

		if len(found) == 0 {
			return "", fmt.Errorf("subnet %s not found", subnetName)
		}
		if len(found) > 1 {
			return "", fmt.Errorf("found more than one subnet with name %s", subnetName)
		}

		if found[0].ExtId == nil {
			return "", fmt.Errorf("subnet %s has no ExtId", subnetName)
		}
		return *found[0].ExtId, nil
	}

	return "", fmt.Errorf("subnet name or UUID must be provided")
}

// GPU helpers

// getGPU finds a GPU by name using V4 API and returns a V4 GPU struct
func getGPU(ctx context.Context, client *convergedv4.Client, name string, clusterUUID string) (*vmmModels.Gpu, error) {
	// Try physical GPUs first
	physicalGPUs, err := client.Clusters.ListClusterPhysicalGPUs(ctx, clusterUUID,
		converged.WithFilter(fmt.Sprintf("physicalGpuConfig/deviceName eq '%s'", name)))
	if err == nil {
		for _, gpu := range physicalGPUs {
			if gpu.PhysicalGpuConfig != nil &&
				gpu.PhysicalGpuConfig.DeviceName != nil &&
				strings.EqualFold(*gpu.PhysicalGpuConfig.DeviceName, name) &&
				(gpu.PhysicalGpuConfig.IsInUse == nil || !*gpu.PhysicalGpuConfig.IsInUse) {

				vmGpu := vmmModels.NewGpu()
				vmGpu.Name = gpu.PhysicalGpuConfig.DeviceName
				if gpu.PhysicalGpuConfig.DeviceId != nil {
					deviceId := int(*gpu.PhysicalGpuConfig.DeviceId)
					vmGpu.DeviceId = &deviceId
				}
				vmGpu.Mode = vmmModels.GPUMODE_PASSTHROUGH_COMPUTE.Ref()
				if gpu.PhysicalGpuConfig.Type != nil &&
					strings.Contains(gpu.PhysicalGpuConfig.Type.GetName(), "GRAPHICS") {
					vmGpu.Mode = vmmModels.GPUMODE_PASSTHROUGH_GRAPHICS.Ref()
				}
				if gpu.PhysicalGpuConfig.VendorName != nil {
					vmGpu.Vendor = gpuVendorStringToGpuVendor(*gpu.PhysicalGpuConfig.VendorName)
				}
				return vmGpu, nil
			}
		}
	}

	// Try virtual GPUs
	virtualGPUs, err := client.Clusters.ListClusterVirtualGPUs(ctx, clusterUUID,
		converged.WithFilter(fmt.Sprintf("virtualGpuConfig/deviceName eq '%s'", name)))
	if err == nil {
		for _, gpu := range virtualGPUs {
			if gpu.VirtualGpuConfig != nil &&
				gpu.VirtualGpuConfig.DeviceName != nil &&
				strings.EqualFold(*gpu.VirtualGpuConfig.DeviceName, name) &&
				(gpu.VirtualGpuConfig.IsInUse == nil || !*gpu.VirtualGpuConfig.IsInUse) {

				vmGpu := vmmModels.NewGpu()
				vmGpu.Name = gpu.VirtualGpuConfig.DeviceName
				if gpu.VirtualGpuConfig.DeviceId != nil {
					deviceId := int(*gpu.VirtualGpuConfig.DeviceId)
					vmGpu.DeviceId = &deviceId
				}
				vmGpu.Mode = vmmModels.GPUMODE_VIRTUAL.Ref()
				if gpu.VirtualGpuConfig.VendorName != nil {
					vmGpu.Vendor = gpuVendorStringToGpuVendor(*gpu.VirtualGpuConfig.VendorName)
				}
				return vmGpu, nil
			}
		}
	}

	return nil, fmt.Errorf("GPU %s not found", name)
}

func gpuVendorStringToGpuVendor(vendor string) *vmmModels.GpuVendor {
	switch vendor {
	case "kNvidia", "NVIDIA":
		return vmmModels.GPUVENDOR_NVIDIA.Ref()
	case "kIntel", "INTEL":
		return vmmModels.GPUVENDOR_INTEL.Ref()
	case "kAmd", "AMD":
		return vmmModels.GPUVENDOR_AMD.Ref()
	default:
		return vmmModels.GPUVENDOR_UNKNOWN.Ref()
	}
}

// Host helpers

// findHostByUUID finds a host by UUID using V4 API
func findHostByUUID(ctx context.Context, client *convergedv4.Client, hostUUID string) (*clusterModels.Host, error) {
	hosts, err := client.Clusters.ListAllHosts(ctx, converged.WithFilter(fmt.Sprintf("extId eq '%s'", hostUUID)))
	if err != nil {
		return nil, err
	}

	if len(hosts) == 0 {
		return nil, fmt.Errorf("host %s not found", hostUUID)
	}

	return &hosts[0], nil
}

// Category helpers

// getCategoryExtIds converts category key/value pairs to their V4 ExtIds
func getCategoryExtIds(ctx context.Context, client *convergedv4.Client, categories []Category) ([]string, error) {
	if len(categories) == 0 {
		return nil, nil
	}

	extIds := make([]string, 0, len(categories))
	for _, cat := range categories {
		filter := fmt.Sprintf("key eq '%s' and value eq '%s'", cat.Key, cat.Value)
		cats, err := client.Categories.List(ctx, converged.WithFilter(filter))
		if err != nil {
			return nil, fmt.Errorf("error looking up category %s:%s: %s", cat.Key, cat.Value, err.Error())
		}

		if len(cats) == 0 {
			return nil, fmt.Errorf("category %s:%s not found", cat.Key, cat.Value)
		}

		if cats[0].ExtId != nil {
			extIds = append(extIds, *cats[0].ExtId)
		}
	}

	return extIds, nil
}
