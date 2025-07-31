packer {
  required_plugins {
    nutanix = {
      version = ">= 1.1.3"
      source = "github.com/nutanix-cloud-native/nutanix"
    }
  }
}
