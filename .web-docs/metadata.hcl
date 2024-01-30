# For full specification on the configuration of this file visit:
# https://github.com/hashicorp/integration-template#metadata-configuration
integration {
  name = "Nutanix"
  description = "A multi-component plugin can be used with Packer to create custom images."
  identifier = "packer/nutanix-cloud-native/nutanix"
  component {
    type = "builder"
    name = "Nutanix plugin"
    slug = "nutanix"
  }
}
