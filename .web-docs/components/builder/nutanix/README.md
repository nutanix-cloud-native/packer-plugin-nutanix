This document is going to detail all Nutanix plugin parameters.

## Principle
The Nutanix plugin will create a temporary VM as foundation of your Packer image, apply all providers you define to customize your image, then clone the VM disk image as your final Packer image.

## Environment configuration
These parameters allow to define information about platform and temporary VM used to create the image.

### Required
  - `nutanix_username` (string) - User used for Prism Central login.
  - `nutanix_password` (string) - Password of this user for Prism Central login.
  - `nutanix_endpoint` (string) - Prism Central FQDN or IP.
  - `cluster_name` or `cluster_uuid` (string) - Nutanix cluster name or uuid used to create and store image.
  - `os_type` (string) - OS Type ("Linux" or "Windows").

### Optional
  - `nutanix_port` (number) - Port used for connection to Prism Central.
  - `nutanix_insecure` (bool) - Authorize connection to Prism Central without valid certificate.
  - `vm_name` (string) - Name of the temporary VM to create. If not specified a random `packer-*` name will be used.
  - `cpu` (number) - Number of vCPU for temporary VM.
  - `memory_mb` (number) - Size of vRAM for temporary VM (in megabytes).
  - `cd_files` (array of strings) - A list of files to place onto a CD that is attached when the VM is booted. This can include either files or directories; any directories will be copied onto the CD recursively, preserving directory structure hierarchy.
  - `cd_label` (string) - Label of this CD Drive.
  - `boot_type` (string) - Type of boot used on the temporary VM ("legacy" or "uefi", default is "legacy").
  - `boot_priority` (string) - Priority of boot device ("cdrom" or "disk", default is "cdrom". UEFI support need AHV 8.0.12+, 9.1.1.2+, 9.1.3+, 9.2+ or 10.0+). 
  - `vm_categories` ([]Category) - Assign Categories to the vm.
  - `project` (string) - Assign Project to the vm.
  - `gpu` ([] GPU) - GPU in cluster name to be attached on temporary VM.
  - `serialport` (bool) - Add a serial port to the temporary VM. This is required for some Linux Cloud Images that will have a kernel panic if a serial port is not present on first boot.

## Output configuration
These parameters allow to configure everything around image creation, from the temporary VM connection to the final image definition.

### All OS
- `image_name` (string) - Name of the output image.
- `image_description` (string) - Description for output image.
- `image_categories` ([]Category) - Assign Categories to the image.
- `force_deregister` (bool) - Allow output image override if already exists.
- `image_delete` (bool) - Delete image once build process is completed (default is false).
- `image_export` (bool) - Export image in the current folder (default is false).
- `shutdown_command` (string) - Command line to shutdown your temporary VM.
- `shutdown_timeout` (string) - Timeout for VM shutdown (format : 2m).
- `vm_force_delete` (bool) - Delete vm even if build is not succesful (default is false).
- `communicator` (string) - Protocol used for Packer connection (ex "winrm" or "ssh"). Default is : "ssh".

### Dedicated to Linux
- `user_data` (string) - cloud-init content base64 encoded.
- `ssh_username` (string) - user for ssh connection initiated by Packer.
- `ssh_password` (string) - password for the ssh user.

### Dedicated to Windows
- `winrm_port` (number) - Port for WinRM communication (default is 5986).
- `winrm_insecure` (bool) - Allow insecure connection to WinRM.
- `winrm_use_ssl` (bool) - Request SSL connection with WinRM.
- `winrm_timeout` (string) - Timeout for WinRM (format 45m).
- `winrm_username` (string) - User login for WinRM connection.
- `winrm_password` (string) - Password this User.

## Disk configuration
Use `vm_disks{}` entry to configure disk to your VM image. If you want to configure several disks, use this entry multiple times.

All parameters of this `vm_disks` section are described below.

3 types of disk configurations can be used:
- disk (create an empty disk)
- disk image (create disk from Nutanix image library)
- ISO image (create disk from ISO image)

### Disk 
- `image_type` (string) - "DISK".
- `disk_size_gb` (number) - size of th disk (in gigabytes).

Sample:
```hcl
  vm_disks {
      image_type = "DISK"
      disk_size_gb = 30
  }
```

### Disk image
- `image_type` (string) - "DISK_IMAGE" (you must use one of the three following parameters to source the image).
- `source_image_name` (string) - Name of the image used as disk source.
- `source_image_uuid` (string) - UUID of the image used as disk source.
- `source_image_uri` (string) - URI of the image used as disk source (if image is not already on the cluster, it will download and store it before launching output image creation process).
- `source_image_checksum` (string) - Checksum of the image used as disk source (work only with `source_image_uri` and if image is not already present in the library).
- `source_image_checksum_type` (string) - Type of checksum used for `source_image_checksum` (`sha256` or `sha1` ).
- `source_image_delete` (bool) - Delete source image once build process is completed (default is false).
- `source_image_force` (bool) - Always download and replace source image even if already exist (default is false).
- `disk_size_gb` (number) - size of the disk (in gigabytes).

Sample:
```hcl
  vm_disks {
      image_type = "DISK_IMAGE"
      source_image_name = "<myDiskImage>"
      disk_size_gb = 40
  }
```
### ISO Image
- `image_type` (string) - "ISO_IMAGE".
- `source_image_name` (string) - Name of the ISO image to mount.
- `source_image_uuid` (string) - UUID of the ISO image to mount.
- `source_image_delete` (bool) - Delete source image once build process is completed (default is false).
- `source_image_force` (bool) - Always download and replace source image even if already exist (default is false).

Sample:
```hcl
  vm_disks {
      image_type = "ISO_IMAGE"
      source_image_name = "<myISOimage>"
  }
```


## OVA Config
Use `ova{}` entry to configure the OVA creation & export

All parameters of this `ova` section are described below.

3 types of disk configurations can be used:
- `create` (bool) - Create OVA image for the vm (default is false).
- `export` (bool) - Export OVA image in the current folder (default is false).
- `format` (string) - Format of the ova image (allowed values: 'vmdk', 'qcow2', default 'vmdk').
- `name` (string) - Name of the the OVA image (default is false).

Sample:
```hcl
  ova {
      create = true
      export = true
      format = "vmdk"
      name = "myExportedOVA"
  }
```

## Network Configuration
Use `vm_nics{}` entry to configure NICs in your image

In this section, you have to define network you will to connect with one of this keyword :

- `subnet_name` (string) - Name of the cluster subnet to use.
- `subnet_uuid` (string) - UUID of the cluster subnet to use.

Sample
```hcl
  vm_nics {
    subnet_name = "<mySubnet>"
  }
```

### Categories Configuration

Use `image_categories{}` and `vm_categories{}` to assign category to your image or vm.  If you want to assign multiple categories , use the entry multiple times.

In this section, you have to define category you will to assign with the following parameters:

- `key` (string) - Name of the category to assign.
- `value` (string) - Value of the category to assign.

Sample
```hcl
  image_categories {
    key = "OSType"
    value = "ubuntu-22.04"
  }
```

Note: Categories must already be present in Prism Central.

## GPU Configuration

Use `GPU` to assign a GPU that is present on `cluster-name` on the temporary vm. Add the name of the GPU you wish to attach.

Sample

```hcl
  gpu {
    name = "Ampere 40"
  }
```

## Boot Configuration

<!-- Code generated from the comments of the BootConfig struct in bootcommand/config.go; DO NOT EDIT MANUALLY -->

The boot configuration is very important: `boot_command` specifies the keys
to type when the virtual machine is first booted in order to start the OS
installer. This command is typed after boot_wait, which gives the virtual
machine some time to actually load.

The boot_command is an array of strings. The strings are all typed in
sequence. It is an array only to improve readability within the template.

There are a set of special keys available. If these are in your boot
command, they will be replaced by the proper key:

-   `<bs>` - Backspace

-   `<del>` - Delete

-   `<enter> <return>` - Simulates an actual "enter" or "return" keypress.

-   `<esc>` - Simulates pressing the escape key.

-   `<tab>` - Simulates pressing the tab key.

-   `<f1> - <f12>` - Simulates pressing a function key.

-   `<up> <down> <left> <right>` - Simulates pressing an arrow key.

-   `<spacebar>` - Simulates pressing the spacebar.

-   `<insert>` - Simulates pressing the insert key.

-   `<home> <end>` - Simulates pressing the home and end keys.

  - `<pageUp> <pageDown>` - Simulates pressing the page up and page down
    keys.

-   `<menu>` - Simulates pressing the Menu key.

-   `<leftAlt> <rightAlt>` - Simulates pressing the alt key.

-   `<leftCtrl> <rightCtrl>` - Simulates pressing the ctrl key.

-   `<leftShift> <rightShift>` - Simulates pressing the shift key.

-   `<leftSuper> <rightSuper>` - Simulates pressing the ⌘ or Windows key.

  - `<wait> <wait5> <wait10>` - Adds a 1, 5 or 10 second pause before
    sending any additional keys. This is useful if you have to generally
    wait for the UI to update before typing more.

  - `<waitXX>` - Add an arbitrary pause before sending any additional keys.
    The format of `XX` is a sequence of positive decimal numbers, each with
    optional fraction and a unit suffix, such as `300ms`, `1.5h` or `2h45m`.
    Valid time units are `ns`, `us` (or `µs`), `ms`, `s`, `m`, `h`. For
    example `<wait10m>` or `<wait1m20s>`.

  - `<XXXOn> <XXXOff>` - Any printable keyboard character, and of these
    "special" expressions, with the exception of the `<wait>` types, can
    also be toggled on or off. For example, to simulate ctrl+c, use
    `<leftCtrlOn>c<leftCtrlOff>`. Be sure to release them, otherwise they
    will be held down until the machine reboots. To hold the `c` key down,
    you would use `<cOn>`. Likewise, `<cOff>` to release.

  - `{{ .HTTPIP }} {{ .HTTPPort }}` - The IP and port, respectively of an
    HTTP server that is started serving the directory specified by the
    `http_directory` configuration parameter. If `http_directory` isn't
    specified, these will be blank!

-   `{{ .Name }}` - The name of the VM.

Example boot command. This is actually a working boot command used to start an
CentOS 6.4 installer:

In JSON:

```json
"boot_command": [

	   "<tab><wait>",
	   " ks=http://{{ .HTTPIP }}:{{ .HTTPPort }}/centos6-ks.cfg<enter>"
	]

```

In HCL2:

```hcl
boot_command = [

	   "<tab><wait>",
	   " ks=http://{{ .HTTPIP }}:{{ .HTTPPort }}/centos6-ks.cfg<enter>"
	]

```

The example shown below is a working boot command used to start an Ubuntu
12.04 installer:

In JSON:

```json
"boot_command": [

	"<esc><esc><enter><wait>",
	"/install/vmlinuz noapic ",
	"preseed/url=http://{{ .HTTPIP }}:{{ .HTTPPort }}/preseed.cfg ",
	"debian-installer=en_US auto locale=en_US kbd-chooser/method=us ",
	"hostname={{ .Name }} ",
	"fb=false debconf/frontend=noninteractive ",
	"keyboard-configuration/modelcode=SKIP keyboard-configuration/layout=USA ",
	"keyboard-configuration/variant=USA console-setup/ask_detect=false ",
	"initrd=/install/initrd.gz -- <enter>"

]
```

In HCL2:

```hcl
boot_command = [

	"<esc><esc><enter><wait>",
	"/install/vmlinuz noapic ",
	"preseed/url=http://{{ .HTTPIP }}:{{ .HTTPPort }}/preseed.cfg ",
	"debian-installer=en_US auto locale=en_US kbd-chooser/method=us ",
	"hostname={{ .Name }} ",
	"fb=false debconf/frontend=noninteractive ",
	"keyboard-configuration/modelcode=SKIP keyboard-configuration/layout=USA ",
	"keyboard-configuration/variant=USA console-setup/ask_detect=false ",
	"initrd=/install/initrd.gz -- <enter>"

]
```

For more examples of various boot commands, see the sample projects from our
[community templates page](https://packer.io/community-tools#templates).

<!-- End of code generated from the comments of the BootConfig struct in bootcommand/config.go; -->


<!-- Code generated from the comments of the VNCConfig struct in bootcommand/config.go; DO NOT EDIT MANUALLY -->

The boot command "typed" character for character over a VNC connection to
the machine, simulating a human actually typing the keyboard.

Keystrokes are typed as separate key up/down events over VNC with a default
100ms delay. The delay alleviates issues with latency and CPU contention.
You can tune this delay on a per-builder basis by specifying
"boot_key_interval" in your Packer template.

<!-- End of code generated from the comments of the VNCConfig struct in bootcommand/config.go; -->


**Optional**:

<!-- Code generated from the comments of the BootConfig struct in bootcommand/config.go; DO NOT EDIT MANUALLY -->

- `boot_keygroup_interval` (duration string | ex: "1h5m2s") - Time to wait after sending a group of key pressses. The value of this
  should be a duration. Examples are `5s` and `1m30s` which will cause
  Packer to wait five seconds and one minute 30 seconds, respectively. If
  this isn't specified, a sensible default value is picked depending on
  the builder type.

- `boot_wait` (duration string | ex: "1h5m2s") - The time to wait after booting the initial virtual machine before typing
  the `boot_command`. The value of this should be a duration. Examples are
  `5s` and `1m30s` which will cause Packer to wait five seconds and one
  minute 30 seconds, respectively. If this isn't specified, the default is
  `10s` or 10 seconds. To set boot_wait to 0s, use a negative number, such
  as "-1s"

- `boot_command` ([]string) - This is an array of commands to type when the virtual machine is first
  booted. The goal of these commands should be to type just enough to
  initialize the operating system installer. Special keys can be typed as
  well, and are covered in the section below on the boot command. If this
  is not specified, it is assumed the installer will start itself.

<!-- End of code generated from the comments of the BootConfig struct in bootcommand/config.go; -->


<!-- Code generated from the comments of the VNCConfig struct in bootcommand/config.go; DO NOT EDIT MANUALLY -->

- `disable_vnc` (bool) - Whether to create a VNC connection or not. A boot_command cannot be used
  when this is true. Defaults to false.

- `boot_key_interval` (duration string | ex: "1h5m2s") - Time in ms to wait between each key press

<!-- End of code generated from the comments of the VNCConfig struct in bootcommand/config.go; -->


## IP Wait configuration

**Optional**:

<!-- Code generated from the comments of the WaitIpConfig struct in builder/nutanix/step_wait_for_ip.go; DO NOT EDIT MANUALLY -->

- `ip_wait_timeout` (duration string | ex: "1h5m2s") - Amount of time to wait for VM's IP, similar to 'ssh_timeout'.
  Defaults to `30m` (30 minutes). Refer to the Golang
  [ParseDuration](https://golang.org/pkg/time/#ParseDuration)
  documentation for full details.

- `ip_settle_timeout` (duration string | ex: "1h5m2s") - Amount of time to wait for VM's IP to settle down, sometimes VM may
  report incorrect IP initially, then it is recommended to set that
  parameter to apx. 2 minutes. Examples `45s` and `10m`.
  Defaults to `5s` (5 seconds). Refer to the Golang
  [ParseDuration](https://golang.org/pkg/time/#ParseDuration)
  documentation for full details.

- `ip_wait_address` (\*string) - Set this to a CIDR address to cause the service to wait for an address that is contained in
  this network range. Defaults to `0.0.0.0/0` for any IPv4 address. Examples include:
  
  * empty string ("") - remove all filters
  * `0:0:0:0:0:0:0:0/0` - allow only ipv6 addresses
  * `192.168.1.0/24` - only allow ipv4 addresses from 192.168.1.1 to 192.168.1.254

<!-- End of code generated from the comments of the WaitIpConfig struct in builder/nutanix/step_wait_for_ip.go; -->


## Communicator Configuration

**Optional**:

##### Common

<!-- Code generated from the comments of the Config struct in communicator/config.go; DO NOT EDIT MANUALLY -->

- `communicator` (string) - Packer currently supports three kinds of communicators:
  
  -   `none` - No communicator will be used. If this is set, most
      provisioners also can't be used.
  
  -   `ssh` - An SSH connection will be established to the machine. This
      is usually the default.
  
  -   `winrm` - A WinRM connection will be established.
  
  In addition to the above, some builders have custom communicators they
  can use. For example, the Docker builder has a "docker" communicator
  that uses `docker exec` and `docker cp` to execute scripts and copy
  files.

- `pause_before_connecting` (duration string | ex: "1h5m2s") - We recommend that you enable SSH or WinRM as the very last step in your
  guest's bootstrap script, but sometimes you may have a race condition
  where you need Packer to wait before attempting to connect to your
  guest.
  
  If you end up in this situation, you can use the template option
  `pause_before_connecting`. By default, there is no pause. For example if
  you set `pause_before_connecting` to `10m` Packer will check whether it
  can connect, as normal. But once a connection attempt is successful, it
  will disconnect and then wait 10 minutes before connecting to the guest
  and beginning provisioning.

<!-- End of code generated from the comments of the Config struct in communicator/config.go; -->


##### SSH

<!-- Code generated from the comments of the SSH struct in communicator/config.go; DO NOT EDIT MANUALLY -->

- `ssh_host` (string) - The address to SSH to. This usually is automatically configured by the
  builder.

- `ssh_port` (int) - The port to connect to SSH. This defaults to `22`.

- `ssh_username` (string) - The username to connect to SSH with. Required if using SSH.

- `ssh_password` (string) - A plaintext password to use to authenticate with SSH.

- `ssh_ciphers` ([]string) - This overrides the value of ciphers supported by default by Golang.
  The default value is [
    "aes128-gcm@openssh.com",
    "chacha20-poly1305@openssh.com",
    "aes128-ctr", "aes192-ctr", "aes256-ctr",
  ]
  
  Valid options for ciphers include:
  "aes128-ctr", "aes192-ctr", "aes256-ctr", "aes128-gcm@openssh.com",
  "chacha20-poly1305@openssh.com",
  "arcfour256", "arcfour128", "arcfour", "aes128-cbc", "3des-cbc",

- `ssh_clear_authorized_keys` (bool) - If true, Packer will attempt to remove its temporary key from
  `~/.ssh/authorized_keys` and `/root/.ssh/authorized_keys`. This is a
  mostly cosmetic option, since Packer will delete the temporary private
  key from the host system regardless of whether this is set to true
  (unless the user has set the `-debug` flag). Defaults to "false";
  currently only works on guests with `sed` installed.

- `ssh_key_exchange_algorithms` ([]string) - If set, Packer will override the value of key exchange (kex) algorithms
  supported by default by Golang. Acceptable values include:
  "curve25519-sha256@libssh.org", "ecdh-sha2-nistp256",
  "ecdh-sha2-nistp384", "ecdh-sha2-nistp521",
  "diffie-hellman-group14-sha1", and "diffie-hellman-group1-sha1".

- `ssh_certificate_file` (string) - Path to user certificate used to authenticate with SSH.
  The `~` can be used in path and will be expanded to the
  home directory of current user.

- `ssh_pty` (bool) - If `true`, a PTY will be requested for the SSH connection. This defaults
  to `false`.

- `ssh_timeout` (duration string | ex: "1h5m2s") - The time to wait for SSH to become available. Packer uses this to
  determine when the machine has booted so this is usually quite long.
  Example value: `10m`.
  This defaults to `5m`, unless `ssh_handshake_attempts` is set.

- `ssh_disable_agent_forwarding` (bool) - If true, SSH agent forwarding will be disabled. Defaults to `false`.

- `ssh_handshake_attempts` (int) - The number of handshakes to attempt with SSH once it can connect.
  This defaults to `10`, unless a `ssh_timeout` is set.

- `ssh_bastion_host` (string) - A bastion host to use for the actual SSH connection.

- `ssh_bastion_port` (int) - The port of the bastion host. Defaults to `22`.

- `ssh_bastion_agent_auth` (bool) - If `true`, the local SSH agent will be used to authenticate with the
  bastion host. Defaults to `false`.

- `ssh_bastion_username` (string) - The username to connect to the bastion host.

- `ssh_bastion_password` (string) - The password to use to authenticate with the bastion host.

- `ssh_bastion_interactive` (bool) - If `true`, the keyboard-interactive used to authenticate with bastion host.

- `ssh_bastion_private_key_file` (string) - Path to a PEM encoded private key file to use to authenticate with the
  bastion host. The `~` can be used in path and will be expanded to the
  home directory of current user.

- `ssh_bastion_certificate_file` (string) - Path to user certificate used to authenticate with bastion host.
  The `~` can be used in path and will be expanded to the
  home directory of current user.

- `ssh_file_transfer_method` (string) - `scp` or `sftp` - How to transfer files, Secure copy (default) or SSH
  File Transfer Protocol.
  
  **NOTE**: Guests using Windows with Win32-OpenSSH v9.1.0.0p1-Beta, scp
  (the default protocol for copying data) returns a a non-zero error code since the MOTW
  cannot be set, which cause any file transfer to fail. As a workaround you can override the transfer protocol
  with SFTP instead `ssh_file_transfer_method = "sftp"`.

- `ssh_proxy_host` (string) - A SOCKS proxy host to use for SSH connection

- `ssh_proxy_port` (int) - A port of the SOCKS proxy. Defaults to `1080`.

- `ssh_proxy_username` (string) - The optional username to authenticate with the proxy server.

- `ssh_proxy_password` (string) - The optional password to use to authenticate with the proxy server.

- `ssh_keep_alive_interval` (duration string | ex: "1h5m2s") - How often to send "keep alive" messages to the server. Set to a negative
  value (`-1s`) to disable. Example value: `10s`. Defaults to `5s`.

- `ssh_read_write_timeout` (duration string | ex: "1h5m2s") - The amount of time to wait for a remote command to end. This might be
  useful if, for example, packer hangs on a connection after a reboot.
  Example: `5m`. Disabled by default.

- `ssh_remote_tunnels` ([]string) - 

- `ssh_local_tunnels` ([]string) - 

<!-- End of code generated from the comments of the SSH struct in communicator/config.go; -->


- `ssh_private_key_file` (string) - Path to a PEM encoded private key file to use to authenticate with SSH.
  The `~` can be used in path and will be expanded to the home directory
  of current user.


##### Windows Remote Management (WinRM)

<!-- Code generated from the comments of the WinRM struct in communicator/config.go; DO NOT EDIT MANUALLY -->

- `winrm_username` (string) - The username to use to connect to WinRM.

- `winrm_password` (string) - The password to use to connect to WinRM.

- `winrm_host` (string) - The address for WinRM to connect to.
  
  NOTE: If using an Amazon EBS builder, you can specify the interface
  WinRM connects to via
  [`ssh_interface`](/packer/integrations/nutanix-cloud-native/amazon/latest/components/builder/ebs#ssh_interface)

- `winrm_no_proxy` (bool) - Setting this to `true` adds the remote
  `host:port` to the `NO_PROXY` environment variable. This has the effect of
  bypassing any configured proxies when connecting to the remote host.
  Default to `false`.

- `winrm_port` (int) - The WinRM port to connect to. This defaults to `5985` for plain
  unencrypted connection and `5986` for SSL when `winrm_use_ssl` is set to
  true.

- `winrm_timeout` (duration string | ex: "1h5m2s") - The amount of time to wait for WinRM to become available. This defaults
  to `30m` since setting up a Windows machine generally takes a long time.

- `winrm_use_ssl` (bool) - If `true`, use HTTPS for WinRM.

- `winrm_insecure` (bool) - If `true`, do not check server certificate chain and host name.

- `winrm_use_ntlm` (bool) - If `true`, NTLMv2 authentication (with session security) will be used
  for WinRM, rather than default (basic authentication), removing the
  requirement for basic authentication to be enabled within the target
  guest. Further reading for remote connection authentication can be found
  [here](https://msdn.microsoft.com/en-us/library/aa384295(v=vs.85).aspx).

<!-- End of code generated from the comments of the WinRM struct in communicator/config.go; -->


## Samples

You can find samples [here](https://github.com/nutanix-cloud-native/packer-plugin-nutanix/tree/main/example) for these instructions usage.
