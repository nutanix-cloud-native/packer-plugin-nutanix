packer {
  required_plugins {
    nutanix = {
      version = ">= 0.6.0"
      source = "github.com/nutanix-cloud-native/nutanix"
    }
  }
}
