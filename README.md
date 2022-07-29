# Packer Plugin Nutanix
The `Nutanix` multi-component plugin can be used with HashiCorp [Packer](https://www.packer.io)
to create custom images. For the full list of available features for this plugin see [docs](docs).

---

[![Go Report Card](https://goreportcard.com/badge/github.com/nutanix-cloud-native/packer-plugin-nutanix)](https://goreportcard.com/report/github.com/nutanix-cloud-native/packer-plugin-nutanix)
![CI](https://github.com/nutanix-cloud-native/packer-plugin-nutanix/actions/workflows/integration.yml/badge.svg)
![Release](https://github.com/nutanix-cloud-native/packer-plugin-nutanix/actions/workflows/release.yml/badge.svg)

[![release](https://img.shields.io/github/release-pre/nutanix-cloud-native/packer-plugin-nutanix.svg)](https://github.com/nutanix-cloud-native/packer-plugin-nutanix/releases)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://github.com/nutanix-cloud-native/packer-plugin-nutanix/blob/master/LICENSE)
![Proudly written in Golang](https://img.shields.io/badge/written%20in-Golang-92d1e7.svg)
[![Releases](https://img.shields.io/github/downloads/nutanix-cloud-native/packer-plugin-nutanix/total.svg)](https://github.com/nutanix-cloud-native/packer-plugin-nutanix/releases)

---

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
      version = ">= 0.1.3"
      source  = "github.com/nutanix-cloud-native/nutanix"
    }
  }
}
```

#### Manual installation

You can find pre-built binary releases of the plugin [here](https://github.com/nutanix-cloud-native/packer-plugin-nutanix/releases).
Once you have downloaded the latest archive corresponding to your target OS,
uncompress it to retrieve the plugin binary file corresponding to your platform.
To install the plugin, please follow the official Packer documentation on [installing a plugin](https://www.packer.io/docs/extending/plugins/#installing-plugins).


#### From Source

If you prefer to build the plugin from its source code, clone the GitHub repository locally and run the command `make build` from the root directory.
Upon successful compilation, a `packer-plugin-nutanix` plugin binary file can be found in the root directory.
To install the compiled plugin, please follow the official Packer documentation on [installing a plugin](https://www.packer.io/docs/extending/plugins/#installing-plugins).

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
Create a Terraform Manifest using the Nutanix Provider to create your Image and define an output with your image uuid. You can pass this uuid into a Packer Variable. In that case the centos_iso_image_name Variable in the example settings file must be commented.
```
export PKR_VAR_centos_iso_image_name=$(terraform output -raw centos_uuid)
```
## Contributing
See the [contributing docs](CONTRIBUTING.md).

## Support
### Community Plus

This code is developed in the open with input from the community through issues and PRs. A Nutanix engineering team serves as the maintainer. Documentation is available in the project repository.

Issues and enhancement requests can be submitted in the [Issues tab of this repository](../../issues). Please search for and review the existing open issues before submitting a new issue.

## License
The project is released under version 2.0 of the [Apache license](http://www.apache.org/licenses/LICENSE-2.0).
