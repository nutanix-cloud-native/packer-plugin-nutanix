#//
# This template will create a Windows 11 template on Nutanix using Packer. It configures and actions the following:
# - Creates a VM with the specified resources (1 vCPU, 4 cores, 8192 MB memory) on the defined Nutanix cluster
# - Enables secure boot and adds vTPM
# - Enables Credential Guard (hardware_virtualization)
# - Attaches a NIC to the specified Nutanix subnet
# - Attaches a CD ROM and Boots from a stock Winows 11 ISO file
# - Attaches a Nutanix VirtIO ISO for drivers
# - Attaches a disk of 60GB size on a specified storage container
# - Configures the boot priority to CD ROM
# - Waits 10 seconds after the intial boot and then sends the boot commands to skip the Windows "Press any key to boot from CD or DVD" prompt
# - Uses an autounattend.xml file to automate the Windows installation including adding the VirtIO drivers (from attached ISO) and using a SetupComplete.cmd script to run post-installation tasks such as enabling WinRM for Packer
# - Connects to the VM using WinRM on port 5985 with a timeout of 30 minutes
# - Removes all CD ROMs after the build is complete
# - Skips creating a Nutanix image, as a template will be used
# - Retains the VM after the build is complete for further testing or modifications
# - Creates a Nutanix template from the VM with a specified name and description
# //

source "nutanix" "windows11" {
    nutanix_username    = var.nutanix_username
    nutanix_password    = var.nutanix_password
    nutanix_endpoint    = var.nutanix_endpoint
    nutanix_port        = var.nutanix_port
    nutanix_insecure    = var.nutanix_insecure
    cluster_name        = var.nutanix_cluster
    # winrm_host          = "10.2.3.4"
    os_type             = "Windows"
    communicator        = "winrm"
    cpu                 = 1
    core                = 4
    memory_mb           = 8192
    boot_type           = "secure_boot"
    boot_priority       = "cdrom"
    boot_wait = "10s" 
    boot_command = [
        "<spacebar><wait><spacebar><wait><spacebar><wait><spacebar><wait><spacebar><wait><spacebar><wait><spacebar><enter>"
    ]
    vtpm {
        enabled = true
    }
    hardware_virtualization = true
    vm_disks {
        image_type        = "ISO_IMAGE"
        source_image_name = var.windows_11_iso_image_name
    }
    vm_disks {
        image_type        = "ISO_IMAGE"
        source_image_name = "Nutanix-VirtIO-1.2.4.iso"
    }
    vm_disks {
        image_type              = "DISK"
        disk_size_gb            = 60
    }
    vm_nics {
        subnet_name       = var.nutanix_subnet
    }
    cd_files = [ 
        "files/autounattend.xml",
        "files/scripts/EnableWinRMforPacker.ps1",
        "files/scripts/SetupComplete.cmd",
        "files/scripts/StaticIP.ps1"
    ]
    image_skip          = true
    vm_retain = true
    vm_clean {
        cdrom = true
    }
    template {
        create = true
        name = "Windows-11-Template-{{isotime}}"
        description = "Windows 11 Template Created by Packer"
    }
    winrm_port          = 5985
    winrm_timeout       = "30m"
    winrm_use_ssl       = false
    winrm_username      = var.winrm_username
    winrm_password      = var.winrm_password
}

