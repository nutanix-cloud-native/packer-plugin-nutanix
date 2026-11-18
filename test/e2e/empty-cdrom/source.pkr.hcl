source "nutanix" "centos" {
  nutanix_username = var.nutanix_username
  nutanix_password = var.nutanix_password
  nutanix_endpoint = var.nutanix_endpoint
  nutanix_port     = var.nutanix_port
  nutanix_insecure = var.nutanix_insecure
  cluster_name     = var.nutanix_cluster
  os_type          = "Linux"

  vm_disks {
    image_type                 = "DISK_IMAGE"
    source_image_uri           = "https://cloud.centos.org/centos/7/images/CentOS-7-x86_64-GenericCloud-2111.qcow2"
    source_image_checksum      = "4c34278cd7ba51e47d864a5cb34301a2ec7853786cb73877f3fe61bb1040edd4"
    source_image_checksum_type = "sha256"
    disk_size_gb               = 20
  }

  vm_disks {
    image_type                 = "ISO_IMAGE"
    source_image_uri           = "https://vault.centos.org/7.9.2009/isos/x86_64/CentOS-7-x86_64-Minimal-2009.iso"
    source_image_checksum      = "07b94e6b1a0b0260b94c83d6bb76b26bf7a310dc78d7a9c7432809fb9bc6194a"
    source_image_checksum_type = "sha256"
  }

  vm_nics {
    subnet_name = var.nutanix_subnet
  }

  image_categories {
    key   = "TemplateType"
    value = "Vm"
  }

  vm_categories {
    key   = "Environment"
    value = "Testing"
  }

  cd_files      = ["scripts/placeholder.txt"]
  cd_label      = "OEMDRV"
  boot_priority = "disk"

  vm_name    = "e2e-packer-${var.test}-${formatdate("MDYYhms", timestamp())}"
  image_name = "e2e-packer-${var.test}-${formatdate("MDYYhms", timestamp())}"
  image_delete     = true
  force_deregister = true

  vm_clean {
    keep_empty_cdrom = true
  }

  user_data = "I2Nsb3VkLWNvbmZpZwp1c2VyczoKICAtIG5hbWU6IGNlbnRvcwogICAgc3VkbzogWydBTEw9KEFMTCkgTk9QQVNTV0Q6QUxMJ10KY2hwYXNzd2Q6CiAgbGlzdDogfAogICAgY2VudG9zOnBhY2tlcgogIGV4cGlyZTogRmFsc2UKc3NoX3B3YXV0aDogVHJ1ZQ=="

  shutdown_command = "echo 'packer' | sudo -S shutdown -P now"
  shutdown_timeout = "2m"
  ssh_password     = "packer"
  ssh_username     = "centos"
}
