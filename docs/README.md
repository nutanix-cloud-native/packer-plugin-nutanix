
The `Nutanix` multi-component plugin can be used with HashiCorp [Packer](https://www.packer.io)
to create custom images.

### Installation

To install this plugin, copy and paste this code into your Packer configuration, then run [`packer init`](https://www.packer.io/docs/commands/init).

```
packer {
  required_plugins {
    nutanix = {
      version = ">= 0.8.0"
      source  = "github.com/nutanix-cloud-native/nutanix"
    }
  }
}
```

Alternatively, you can use `packer plugins install` to manage installation of this plugin.

```sh
$ packer plugins install github.com/nutanix-cloud-native/nutanix
```

### Components

#### Builders

- [gridscale](/packer/integrations/gridscale/latest/components/builder/gridscale) - The builder takes a template (in gridscale) or an iso-image, runs any provisioning necessary on the template/iso-image after launching it, then snapshots it into a reusable template. This reusable template can then be used as the foundation of new servers that are provisioned within gridscale user space.

### Limitations
#### Building temporary ISOs on MacOS
If you want to use the cd_files Option to create an additional iso-image for kickstart-files or similar be aware that MacOS won´t create a suitable file.
Please install xorriso for support on MacOS.
```
 brew install xorriso
```

### Contributing
See the [contributing docs](https://github.com/nutanix-cloud-native/packer-plugin-nutanix/blob/main/CONTRIBUTING.md).

### Support
#### Community Plus

This code is developed in the open with input from the community through issues and PRs. A Nutanix engineering team serves as the maintainer. Documentation is available in the project repository.

Issues and enhancement requests can be submitted in the [Issues tab of this repository](https://github.com/nutanix-cloud-native/packer-plugin-nutanix/issues). Please search for and review the existing open issues before submitting a new issue.

### License
The project is released under version 2.0 of the [Apache license](http://www.apache.org/licenses/LICENSE-2.0).

