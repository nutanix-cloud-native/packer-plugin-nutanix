package nutanix

import (
	"log"

	registryimage "github.com/hashicorp/packer-plugin-sdk/packer/registry/image"
)

// Artifact contains the unique keys for the nutanix artifact produced from Packer
type Artifact struct {
	Name string
	UUID string

	StateData map[string]interface{}
	//VM   *driver.VirtualMachine
}

// BuilderId will return the unique builder id
func (a *Artifact) BuilderId() string {
	return BuilderId
}

// Files will return the files from the builder
func (a *Artifact) Files() []string {
	return []string{}
}

// Id returns the UUID for the saved image
func (a *Artifact) Id() string {
	return a.UUID
}

// String returns a String name of the artifact
func (a *Artifact) String() string {
	return a.Name
}

// State returns nothing important right now
func (a *Artifact) State(name string) interface{} {
	if name == registryimage.ArtifactStateURI {
		img, err := registryimage.FromArtifact(a)
		if err != nil {
			log.Printf("[DEBUG] error encountered when creating a registry image %v", err)
			return nil
		}
		return img
	}
	return a.StateData[name]
}

// Destroy returns nothing important right now
func (a *Artifact) Destroy() error {
	return nil
	//return a.VM.Destroy()
}

// stateHCPPackerRegistryMetadata will write the metadata as an hcpRegistryImage for each of the images
// present in this artifact.
func (a *Artifact) stateHCPPackerRegistryMetadata() interface{} {

	labels := make(map[string]interface{})

	labels["source_image_url"] = "sourceURL"

	sourceID := "isoPath"

	img, _ := registryimage.FromArtifact(a,
		registryimage.WithID(a.UUID),
		registryimage.WithRegion("pc-prod"),
		registryimage.WithProvider("nutanix"),
		registryimage.WithSourceID(sourceID),
		registryimage.SetLabels(labels),
	)

	return img
}
