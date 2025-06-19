source "nutanix" "centos" {
  nutanix_username = var.nutanix_username
  nutanix_password = var.nutanix_password
  nutanix_endpoint = var.nutanix_endpoint
  nutanix_port     = var.nutanix_port
  nutanix_insecure = var.nutanix_insecure
  cluster_name     = var.nutanix_cluster
  os_type          = "Linux"

  vm_disks {
      image_type = "DISK_IMAGE"
      source_image_name = var.centos_disk_image_name
      disk_size_gb = 40
  }

  vm_nics {
    subnet_name       = var.nutanix_subnet
  }

  image_categories {
    key = "TemplateType"
    value = "Vm"
  }

  vm_categories {
    key = "Environment"
    value = "Dev"
  }

  vtpm {
    enabled = true
  }

  image_name        = "centos-packer-image"
  image_export      = false
  force_deregister  = true
  user_data         = base64encode(file("scripts/cloud-init/cloud-config-centos.yaml"))

  boot_type         = "secure_boot"

  shutdown_command  = "echo 'packer' | sudo -S shutdown -P now"
  shutdown_timeout = "2m"
  ssh_password     = "packer"
  ssh_username     = "centos"
}

source "nutanix" "ubuntu" {
  nutanix_username = var.nutanix_username
  nutanix_password = var.nutanix_password
  nutanix_endpoint = var.nutanix_endpoint
  nutanix_port     = var.nutanix_port
  nutanix_insecure = var.nutanix_insecure
  cluster_name     = var.nutanix_cluster
  os_type          = "Linux"

  vm_disks {
    image_type = "DISK_IMAGE"
    source_image_name = var.ubuntu_disk_image_name
    disk_size_gb = 40
  }

  vm_nics {
    subnet_name       = var.nutanix_subnet
  }

  image_name        = "ubuntu-packer-image"
  force_deregister  = true
  user_data         = base64encode(file("scripts/cloud-init/cloud-config-ubuntu.yaml"))

  shutdown_command  = "echo 'packer' | sudo -S shutdown -P now"
  shutdown_timeout = "2m"
  ssh_password     = "packer"
  ssh_username     = "builder"
}

source "nutanix" "centos-kickstart" {
  nutanix_username = var.nutanix_username
  nutanix_password = var.nutanix_password
  nutanix_endpoint = var.nutanix_endpoint
  nutanix_port     = var.nutanix_port
  nutanix_insecure = var.nutanix_insecure
  cluster_name     = var.nutanix_cluster
  os_type          = "Linux"


  vm_disks {
      image_type = "ISO_IMAGE"
      source_image_name = var.centos_iso_image_name
  }

  vm_disks {
      image_type = "DISK"
      disk_size_gb = 40
  }

  vm_nics {
    subnet_name       = var.nutanix_subnet
  }

  cd_files          = ["scripts/ks.cfg"]
  cd_label          = "OEMDRV"

  boot_priority     = "disk"

  image_name        ="centos8-{{isotime `Jan-_2-15:04:05`}}"
  shutdown_command  = "echo 'packer' | sudo -S shutdown -P now"
  shutdown_timeout = "2m"
  ssh_password     = "packer"
  ssh_username     = "root"
}

source "nutanix" "ubuntu-autoinstall" {
  nutanix_username = var.nutanix_username
  nutanix_password = var.nutanix_password
  nutanix_endpoint = var.nutanix_endpoint
  nutanix_port     = var.nutanix_port
  nutanix_insecure = var.nutanix_insecure
  cluster_name     = var.nutanix_cluster
  os_type          = "Linux"


  vm_disks {
      image_type = "ISO_IMAGE"
      source_image_name = var.ubuntu_iso_image_name
  }

  vm_disks {
      image_type = "DISK"
      disk_size_gb = 40
  }

  vm_nics {
    subnet_name       = var.nutanix_subnet
  }

  boot_priority     = "disk"

  boot_command      = [
    "e<wait>",
    "<down><down><down>",
    "<end><bs><bs><bs><bs><wait>",
    "autoinstall ds=configdrive ---<wait>",
    "<f10><wait>"
  ]
  boot_wait         = "10s"
  user_data         = base64encode(file("scripts/cloud-init/autoinstall-ubuntu.yaml"))

  image_name        ="ubuntu-{{isotime `Jan-_2-15:04:05`}}"
  shutdown_command  = "echo 'ubuntu' | sudo -S shutdown -P now"
  shutdown_timeout = "2m"
  ssh_password     = "ubuntu"
  ssh_username     = "ubuntu"
}

source "nutanix" "windows" {
  nutanix_username = var.nutanix_username
  nutanix_password = var.nutanix_password
  nutanix_endpoint = var.nutanix_endpoint
  nutanix_insecure = var.nutanix_insecure
  cluster_name     = var.nutanix_cluster

  vm_disks {
      image_type = "ISO_IMAGE"
      source_image_name = var.windows_2016_iso_image_name
  }

  vm_disks {
      image_type = "ISO_IMAGE"
      source_image_name = var.virtio_iso_image_name
  }

  vm_disks {
      image_type = "DISK"
      disk_size_gb = 40
  }

  vm_nics {
    subnet_name       = var.nutanix_subnet
  }

  cd_files         = ["scripts/gui/autounattend.xml","scripts/win-update.ps1"]

  boot_priority     = "disk"

  image_name        ="win-{{isotime `Jan-_2-15:04:05`}}"
  shutdown_command  = "shutdown /s /t 10 /f /d p:4:1 /c \"Packer Shutdown\""
  shutdown_timeout  = "3m"
  cpu               = 2
  os_type           = "Windows"
  memory_mb         = "8192"
  communicator      = "winrm"
  winrm_port        = 5986
  winrm_insecure    = true
  winrm_use_ssl     = true
  winrm_timeout     = "45m"
  winrm_password    = "packer"
  winrm_username    = "Administrator"
}
