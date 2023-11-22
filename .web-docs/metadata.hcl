# For full specification on the configuration of this file visit:
# https://github.com/hashicorp/integration-template#metadata-configuration
integration {
  name = "Nutanix"
  description = "TODO"
  identifier = "packer/nutanix-cloud-native/nutanix"
  component {
    type = "builder"
    name = "Nutanix plugin"
    slug = "nutanix"
  }
}
