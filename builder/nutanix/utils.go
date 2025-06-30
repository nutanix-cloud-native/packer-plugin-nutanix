package nutanix

import (
	"errors"
	"os"
	"strings"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	v3 "github.com/nutanix-cloud-native/prism-go-client/v3"
)

// BuildReference create reference from defined object
func BuildReference(uuid, kind string) *v3.Reference {
	return &v3.Reference{
		Kind: StringPtr(kind),
		UUID: StringPtr(uuid),
	}
}

// BuildReferenceValue create referencevalue from defined object
func BuildReferenceValue(uuid, kind string) *v3.ReferenceValues {
	return &v3.ReferenceValues{
		Kind: kind,
		UUID: uuid,
	}
}

const prismCentralService = "PRISM_CENTRAL"

// IsPrismCentral checks if the cluster is a prism central instance or not
// by checking if the service running on the cluster is PRISM_CENTRAL
func IsPrismCentral(cluster *v3.ClusterIntentResponse) bool {
	if cluster.Status == nil ||
		cluster.Status.Resources == nil ||
		cluster.Status.Resources.Config == nil ||
		cluster.Status.Resources.Config.ServiceList == nil ||
		len(cluster.Status.Resources.Config.ServiceList) == 0 {
		return false
	}

	for _, service := range cluster.Status.Resources.Config.ServiceList {
		if service != nil && strings.EqualFold(*service, prismCentralService) {
			return true
		}
	}

	return false
}

func commHost(host string) func(multistep.StateBag) (string, error) {
	return func(state multistep.StateBag) (string, error) {
		if host != "" {
			return host, nil
		} else if guestAddress, ok := state.Get("ip").(string); ok {
			return guestAddress, nil
		} else {
			return "127.0.0.1", nil
		}
	}
}

func fileExists(filename string) bool {
	_, err := os.Stat(filename)
	return !errors.Is(err, os.ErrNotExist)
}
