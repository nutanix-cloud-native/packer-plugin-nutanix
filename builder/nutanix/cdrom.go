package nutanix

import (
	"context"
	"fmt"
	"log"

	vmmModels "github.com/nutanix/ntnx-api-golang-clients/vmm-go-client/v4/models/vmm/v4/ahv/config"
)

// EmptyCD leaves exactly one empty CD-ROM on the VM.
// Build media (OS ISO, VirtIO ISO, kickstart CD) is removed: extra devices are
// deleted, and the ISO in the remaining device is ejected. The VM must be powered off.
// A VM that already has no CD-ROM is left unchanged.
func (d *NutanixDriver) EmptyCD(ctx context.Context, vmUUID string) error {
	v4Client, err := d.getV4Client()
	if err != nil {
		return fmt.Errorf("failed to get V4 client: %s", err.Error())
	}

	vm, err := v4Client.VMs.Get(ctx, vmUUID)
	if err != nil {
		return fmt.Errorf("failed to get VM for CdRom cleanup: %s", err.Error())
	}
	if len(vm.CdRoms) == 0 {
		log.Println("No CdRoms to empty")
		return nil
	}

	keeper := keeperCdRomIndex(vm.CdRoms)
	if keeper < 0 || vm.CdRoms[keeper].ExtId == nil {
		return fmt.Errorf("CdRom devices have no ExtId, cannot leave an empty CD-ROM")
	}
	keeperID := *vm.CdRoms[keeper].ExtId

	for i, cdrom := range vm.CdRoms {
		if cdrom.ExtId == nil {
			return fmt.Errorf("CdRom %d has no ExtId, cannot leave a single empty CD-ROM", i+1)
		}
		if *cdrom.ExtId == keeperID {
			continue
		}
		log.Printf("Removing extra CdRom %s", *cdrom.ExtId)
		if err := v4Client.VMs.DeleteCdRom(ctx, vmUUID, *cdrom.ExtId); err != nil {
			return fmt.Errorf("failed to delete extra CdRom %s: %s", *cdrom.ExtId, err.Error())
		}
	}

	vm, err = v4Client.VMs.Get(ctx, vmUUID)
	if err != nil {
		return fmt.Errorf("failed to re-read VM after CdRom cleanup: %s", err.Error())
	}

	var kept *vmmModels.CdRom
	for i := range vm.CdRoms {
		if vm.CdRoms[i].ExtId != nil && *vm.CdRoms[i].ExtId == keeperID {
			kept = &vm.CdRoms[i]
			break
		}
	}
	if kept == nil {
		return fmt.Errorf("CdRom %s disappeared before it could be ejected", keeperID)
	}
	if !cdRomHasMedia(*kept) {
		log.Printf("CdRom %s is already empty", keeperID)
		return nil
	}

	log.Printf("Ejecting media from CdRom %s", keeperID)
	if err := v4Client.VMs.EjectCdRom(ctx, vmUUID, keeperID); err != nil {
		return fmt.Errorf("failed to eject CD-ROM %s: %s", keeperID, err.Error())
	}
	log.Printf("CdRom %s ejected; VM has one empty CD-ROM", keeperID)
	return nil
}

// keeperCdRomIndex returns the CD-ROM to keep: the lowest bus index that has an ExtId.
// Devices without an address sort last, so an addressed device is preferred.
func keeperCdRomIndex(cdroms []vmmModels.CdRom) int {
	best := -1
	bestBus := int(^uint(0) >> 1)
	for i, cdrom := range cdroms {
		if cdrom.ExtId == nil {
			continue
		}
		bus := int(^uint(0) >> 1)
		if cdrom.DiskAddress != nil && cdrom.DiskAddress.Index != nil {
			bus = *cdrom.DiskAddress.Index
		}
		if best == -1 || bus < bestBus {
			best = i
			bestBus = bus
		}
	}
	return best
}

// cdRomHasMedia reports whether a CD-ROM has an inserted ISO.
// An empty tray has no data source, no disk id, and a zero size.
func cdRomHasMedia(cdrom vmmModels.CdRom) bool {
	if cdrom.BackingInfo == nil {
		return false
	}
	info := cdrom.BackingInfo
	if info.DataSource != nil {
		return true
	}
	if info.DiskExtId != nil && *info.DiskExtId != "" {
		return true
	}
	if info.DiskSizeBytes != nil && *info.DiskSizeBytes > 0 {
		return true
	}
	return false
}
