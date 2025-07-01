

source "nutanix" "windows11" {
    nutanix_username    = var.nutanix_username
    nutanix_password    = var.nutanix_password
    nutanix_endpoint    = var.nutanix_endpoint
    nutanix_port        = var.nutanix_port
    nutanix_insecure    = var.nutanix_insecure
    cluster_name        = var.nutanix_cluster
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
        storage_container_uuid  = var.nutanix_storage_container_uuid
    }
    vm_nics {
        subnet_name       = var.nutanix_subnet
    }
    cd_files = [ 
        "files/autounattend.xml",
        "files/scripts/EnableWinRMforPacker.ps1",
        "files/scripts/SetupComplete.cmd"
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

