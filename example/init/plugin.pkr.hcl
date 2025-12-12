packer {
  required_plugins {
    nutanix = {
      version = ">= 1.1.4"
      source = "github.com/nutanix-cloud-native/nutanix"
    }
  }
}
