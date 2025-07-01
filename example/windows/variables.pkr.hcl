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
}

variable "nutanix_insecure" {
  type = bool
  default = true
}

variable "nutanix_subnet" {
  type = string
}

variable "nutanix_cluster" {
  type = string
}

variable "windows_11_iso_image_name" {
  type = string
}

variable "virtio_iso_image_name" {
  type = string
}

variable "winrm_username" {
  type = string
}

variable "winrm_password" {
  type = string
}

variable "nutanix_storage_container_uuid" {
  type = string
  description = "UUID of the storage container where the VM disk will be created."
}