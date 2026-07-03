
The `Nutanix` multi-component plugin can be used with HashiCorp [Packer](https://www.packer.io) to create custom images.

### Installation

To install this plugin, copy and paste this code into your Packer configuration, then run [`packer init`](https://www.packer.io/docs/commands/init).

```
packer {
  required_plugins {
    nutanix = {
      version = ">= 1.1.9"
      source  = "github.com/nutanix-cloud-native/nutanix"
    }
  }
}
```

Alternatively, you can use `packer plugins install` to manage installation of this plugin.

```sh
$ packer plugins install github.com/nutanix-cloud-native/nutanix
```

### Release Branches and Version Lineage

This plugin currently has two maintained release lines:

- `release-1.2`: clean cut before the v4 API migration, created from the `v1.1.4` baseline.
- `main`: post-v4 API migration changes and the latest line. `v1.3.0` was introduced from the latest `1.1.x` baseline, and subsequent `v1.3.x` releases are cut from `main`.

This structure preserves pre-v4 API compatibility, keeps release ownership clear for maintainers, and allows current development to continue on the post-v4 line.

Going forward, release tags are created from the corresponding branch lines:

- `v1.2.x` tags from `release-1.2` (pre-v4 API).
- `v1.3.x` tags from `main` (post-v4 API, latest line).

### Components

#### Builders

- [nutanix](/packer/integrations/nutanix-cloud-native/nutanix/latest/components/builder/nutanix) - The Nutanix builder will create a temporary VM as foundation of your Packer image, apply all providers you define to customize your image, then clone the VM disk image as your final Packer image.

### Limitations
#### Building temporary ISOs on MacOS
If you want to use the `cd_files` option to create an additional ISO image for kickstart files or similar purposes, be aware that macOS does not generate a compatible file by default.  
To enable support on macOS, please install xorriso.
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

