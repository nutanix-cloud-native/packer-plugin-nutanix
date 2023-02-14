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
  image_category_key = "region"
  image_category_value = "paris"

  force_deregister  = true
  user_data         = "I2Nsb3VkLWNvbmZpZwp1c2VyczoKICAtIG5hbWU6IGNlbnRvcwogICAgc3VkbzogWydBTEw9KEFMTCkgTk9QQVNTV0Q6QUxMJ10KY2hwYXNzd2Q6CiAgbGlzdDogfAogICAgY2VudG9zOnBhY2tlcgogIGV4cGlyZTogRmFsc2UKc3NoX3B3YXV0aDogVHJ1ZQ=="

  shutdown_command  = "echo 'packer' | sudo -S shutdown -P now"
  shutdown_timeout = "2m"
  ssh_password     = "packer"
  ssh_username     = "centos"
}
