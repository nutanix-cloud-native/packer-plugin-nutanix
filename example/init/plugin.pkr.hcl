packer {
  required_plugins {
    nutanix = {
      version = ">= 1.1.2"
      source = "github.com/nutanix-cloud-native/nutanix"
    }
  }
}
