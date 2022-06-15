# Packer Plugin Nutanix
The `Nutanix` multi-component plugin can be used with HashiCorp [Packer](https://www.packer.io)
to create custom images. For the full list of available features for this plugin see [docs](docs).

## Installation

### Using pre-built releases

#### Using the `packer init` command

Starting from version 1.7, Packer supports a new `packer init` command allowing
automatic installation of Packer plugins. Read the
[Packer documentation](https://www.packer.io/docs/commands/init) for more information.

To install this plugin, copy and paste this code into your Packer configuration .
Then, run [`packer init`](https://www.packer.io/docs/commands/init).

```hcl
packer {
  required_plugins {
    nutanix = {
      version = ">= 0.1.0"
      source  = "github.com/nutanix-cloud-native/nutanix"
    }
  }
}
```

### Configuration

For more information on how to configure the plugin, please find some examples in the  [`example/`](example) directory.

## Limitations
### Building temporary ISOs on MacOS
If you want to use the cd_files Option to create an additional iso-image for kickstart-files or similiar be aware that MacOS won´t create a suitable file.
Please install xorriso for support on MacOS.
```
 brew install xorriso
```
### Image Creation
Right now the plugin cannot upload source-images directly, but Terraform can be used to create a source image before running packer itself.
Create a Terraform Manifest using the Nutanix Provider to create your Image and define an output with your image uuid. You can pass this uuid into a Packer Variable
```
export PKR_VAR_centos_image=$(terraform output -raw centos_uuid)
```
## Contributing
See the [contributing docs](CONTRIBUTING.md).

## Support
### Community Plus

This code is developed in the open with input from the community through issues and PRs. A Nutanix engineering team serves as the maintainer. Documentation is available in the project repository. Troubleshooting support for Nutanix customers is available via the [Nutanix Support Portal](https://www.nutanix.com/support-services/product-support).

Issues and enhancement requests can be submitted in the [Issues tab of this repository](../../issues). Please search for and review the existing open issues before submitting a new issue.

## License
The project is released under version 2.0 of the [Apache license](http://www.apache.org/licenses/LICENSE-2.0).
