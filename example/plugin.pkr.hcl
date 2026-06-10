packer {
  required_plugins {
    nutanix = {
      version = ">= 1.1.9"
      source = "github.com/nutanix-cloud-native/nutanix"
    }
  }
}
