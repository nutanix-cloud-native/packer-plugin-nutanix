## Examples
Validate Manifests:
`packer validate .`

Creating CentOS from local Image and running Provisioner:
`packer build -only nutanix.centos .`

Creating Ubuntu from Upstream Image and running Provisioner:
`packer build -only nutanix.ubuntu .`

Creating CentOS from ISO with Kickstart-File:
`packer build -only nutanix.centos-kickstart .`

Creating Ubuntu from ISO with Autoinstall:
`packer build -only nutanix.ubuntu-autoinstall .`

Windows Image (ISO Boot, VirtIO Drivers, cd_files)
`packer build -only nutanix.windows .`

