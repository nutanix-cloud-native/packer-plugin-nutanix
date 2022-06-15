package nutanix

import (
	"github.com/hashicorp/packer-plugin-sdk/multistep"
)

func commHost() func(multistep.StateBag) (string, error) {
	return func(state multistep.StateBag) (string, error) {
		if guestAddress, ok := state.Get("ip").(string); ok {
			return guestAddress, nil
		}

		return "127.0.0.1", nil
	}
}
