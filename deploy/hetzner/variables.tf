variable "name" {
  description = "Name of the server and prefix of the other cloud resources."
  type        = string
  default     = "wso2is-lab"
}

variable "server_type" {
  description = "Hetzner server type. WSO2 Identity Server, PostgreSQL and k3s need at least 4 vCPUs and 8 GB of memory."
  type        = string
  default     = "cpx32"
}

variable "location" {
  description = "Hetzner location of the server (fsn1, nbg1, hel1, ash, hil or sin)."
  type        = string
  default     = "nbg1"
}

variable "image" {
  description = "Operating system image. Ubuntu 24.04 runs k3s without extra AppArmor configuration."
  type        = string
  default     = "ubuntu-24.04"
}

variable "k3s_version" {
  description = "k3s release to install (see https://update.k3s.io/v1-release/channels)."
  type        = string
  default     = "v1.37.2+k3s1"
}

variable "ssh_public_key" {
  description = "SSH public key (file contents) that may log in as root."
  type        = string
}

variable "ssh_allowed_cidrs" {
  description = "Networks that may reach SSH (port 22). The Kubernetes API is reached through an SSH tunnel."
  type        = list(string)
  default     = ["0.0.0.0/0", "::/0"]
}

variable "dns_zone" {
  description = "Existing Hetzner DNS zone that receives the lab records."
  type        = string
  default     = "software-cloud.com"
}

variable "dns_subdomain" {
  description = "Subdomain of dns_zone for the lab. The server answers for this name and every name below it, for example wso2is.<subdomain>.<zone>."
  type        = string
  default     = "lab"
}
