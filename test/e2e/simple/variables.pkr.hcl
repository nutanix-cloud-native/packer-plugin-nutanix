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

variable "centos_disk_image_name" {
  type = string
}
