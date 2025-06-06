packer {
  required_plugins {
    nutanix = {
      version = ">= 0.14.0"
      source = "github.com/nutanix-cloud-native/nutanix"
    }
  }
}
