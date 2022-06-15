## Examples
Validate Manifests:
packer validate .

Creating from Image and running Provisioner:
packer build -only nutanix.centos .

Creating from ISO with Kickstart-File:
packer build -only nutanix.centos-kickerstart .

Windows Image (ISO Boot, VirtIO Drivers, cd_files)
packer build -only nutanix.windows .

