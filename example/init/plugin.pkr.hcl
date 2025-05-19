packer {
  required_plugins {
    nutanix = {
      version = ">= 0.12.2"
      source = "github.com/nutanix-cloud-native/nutanix"
    }
  }
}
