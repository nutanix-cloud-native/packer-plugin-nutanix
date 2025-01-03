source "nutanix" "centos" {
  nutanix_username = var.nutanix_username
  nutanix_password = var.nutanix_password
  nutanix_endpoint = var.nutanix_endpoint
  nutanix_port     = var.nutanix_port
  nutanix_insecure = var.nutanix_insecure
  cluster_name     = var.nutanix_cluster
  os_type          = "Linux"
  
 vm_disks {
      image_type = "ISO_IMAGE"
      source_image_uri = "https://vault.centos.org/7.9.2009/isos/x86_64/CentOS-7-x86_64-Minimal-2009.iso"
  }

  vm_disks {
      image_type = "DISK"
      disk_size_gb = 40
  }

  vm_disks {
      image_type = "DISK"
      disk_size_gb = 20
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
    value = "Testing"
  }

  cd_files          = ["scripts/ks.cfg"]
  cd_label          = "OEMDRV"

  vm_name        = "e2e-packer-${var.test}-${formatdate("MDYYhms", timestamp())}"
  image_name        = "e2e-packer-${var.test}-${formatdate("MDYYhms", timestamp())}"
  image_delete      = true


  force_deregister  = true

  shutdown_command  = "echo 'packer' | sudo -S shutdown -P now"
  shutdown_timeout = "2m"
  ssh_password     = "packer"
  ssh_username     = "root"
}
