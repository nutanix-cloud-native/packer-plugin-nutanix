variable "nutanix_username" {
  type    = string
  default = ""
}

variable "nutanix_password" {
  type      = string
  sensitive = true
  default   = ""
}

# Set nutanix_api_key (or the NUTANIX_API_KEY env var) instead of username/password
# to authenticate via Prism Central API key. If both are set, the api key wins.
variable "nutanix_api_key" {
  type      = string
  sensitive = true
  default   = ""
}

# Optional extra HTTP headers attached to every Prism Central request — useful
# behind reverse proxies like Cloudflare Access. Headers can also be supplied via
# NUTANIX_HEADER_* environment variables.
variable "nutanix_custom_headers" {
  type      = map(string)
  sensitive = true
  default   = {}
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