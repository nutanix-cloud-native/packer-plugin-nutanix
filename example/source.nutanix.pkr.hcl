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
  
  image_name        = "centos-packer-image"
  force_deregister  = true
  user_data         = "I2Nsb3VkLWNvbmZpZwp1c2VyczoKICAtIG5hbWU6IGNlbnRvcwogICAgc3VkbzogWydBTEw9KEFMTCkgTk9QQVNTV0Q6QUxMJ10KY2hwYXNzd2Q6CiAgbGlzdDogfAogICAgY2VudG9zOnBhY2tlcgogIGV4cGlyZTogRmFsc2UKc3NoX3B3YXV0aDogVHJ1ZQ=="

  shutdown_command  = "echo 'packer' | sudo -S shutdown -P now"
  shutdown_timeout = "2m"
  ssh_password     = "packer"
  ssh_username     = "centos"
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

  image_name        ="centos8-{{isotime `Jan-_2-15:04:05`}}"
  shutdown_command  = "echo 'packer' | sudo -S shutdown -P now"
  shutdown_timeout = "2m"
  ssh_password     = "packer"
  ssh_username     = "root"
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
