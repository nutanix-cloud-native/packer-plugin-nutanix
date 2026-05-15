packer {
  required_plugins {
    nutanix = {
      version = ">= 1.1.7"
      source = "github.com/nutanix-cloud-native/nutanix"
    }
  }
}
