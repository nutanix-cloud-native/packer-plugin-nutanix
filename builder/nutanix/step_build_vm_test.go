package nutanix

import (
	"testing"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/nutanix/ntnx-api-golang-clients/vmm-go-client/v4/models/vmm/v4/ahv/config"
)

func TestSetVMStatePublishesBuildID(t *testing.T) {
	vmUUID := "vm-uuid"
	vm := &config.Vm{
		ExtId: &vmUUID,
	}
	state := new(multistep.BasicStateBag)

	setVMState(state, &nutanixInstance{vm: vm})

	if got := state.Get("instance_id"); got != vmUUID {
		t.Fatalf("instance_id = %v, want %s", got, vmUUID)
	}
	if got := state.Get("vm_uuid"); got != vmUUID {
		t.Fatalf("vm_uuid = %v, want %s", got, vmUUID)
	}
	if got := state.Get("cluster_uuid"); got != "" {
		t.Fatalf("cluster_uuid = %v, want empty string", got)
	}
}
