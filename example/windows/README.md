# Windows 11 Template Creation on Nutanix using Packer

This Packer template automates the creation of a Windows 11 VM template on a Nutanix cluster. The process configures the VM and performs several actions to ensure a complete and automated setup.

## Actions Performed

- **Create the VM**
  - Allocate 1 vCPU, 4 cores, and 8192 MB of memory.
  - Deploy the VM on the specified Nutanix cluster.

- **Enable Security Features**
  - Turn on Secure Boot.
  - Add a virtual TPM (vTPM).
  - Enable Credential Guard via `hardware_virtualization`.

- **Attach Networking**
  - Connect a NIC to the specified Nutanix subnet.

- **Configure Boot Media**
  - Attach a CD-ROM with a stock Windows 11 ISO.
  - Attach a second CD-ROM with the Nutanix VirtIO ISO for drivers.

- **Attach Storage**
  - Add a 60 GB disk on the specified storage container.

- **Set Boot Priority**
  - Configure the VM to boot from CD-ROM.
  - Wait 10 seconds after initial boot (depending on your hardware, this value may need to be adjusted).
  - Send keyboard input to bypass the "Press any key to boot from CD or DVD" prompt.

- **Automate the Installation**
  - Use `autounattend.xml` to:
    - Install Windows automatically.
    - Load VirtIO drivers from the attached ISO.
    - Run `SetupComplete.cmd` to perform post-install tasks (e.g., enable WinRM).

- **Connect for Provisioning**
  - Use WinRM on port 5985.
  - Set the connection timeout to 30 minutes.

- **Clean Up**
  - Detach all CD-ROMs after the build completes.

- **Finalize the Build**
  - Do **not** create a Nutanix image.
  - Retain the VM for further testing or manual modifications.
  - Create a Nutanix template from the built VM with the specified name and description.
