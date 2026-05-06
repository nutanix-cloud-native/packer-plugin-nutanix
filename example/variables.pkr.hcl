variable "nutanix_username" {
  type = string
}

variable "nutanix_password" {
  type =  string
  sensitive = true
}

variable "nutanix_endpoint" {
  type = string
}

variable "nutanix_port" {
  type = number
  default = 9440
}

variable "nutanix_insecure" {
  type = bool
  default = false
}

variable "nutanix_subnet" {
  type = string
}

variable "nutanix_cluster" {
  type = string
}

variable "centos_iso_image_name" {
  type = string
  default = null
}

variable "centos_disk_image_name" {
  type = string
  default = null
}

variable "ubuntu_disk_image_name" {
  type = string
  default = null
}

variable "ubuntu_iso_image_name" {
  type = string
  default = null
}

variable "windows_2016_iso_image_name" {
  type = string
  default = null
}

variable "virtio_iso_image_name" {
  type = string
  default = null
}