variable "cloud" {
  description = "Which cloud module to use."
  type        = string
  default     = "hetzner"
  validation {
    condition     = contains(["hetzner", "aws", "gcp"], var.cloud)
    error_message = "cloud must be one of: hetzner, aws, gcp."
  }
}

variable "node_count" {
  description = "Number of cluster hosts (fixed at 3 for topology B)."
  type        = number
  default     = 3
}

variable "instance_type" {
  description = "Per-cloud instance type. Empty string uses the module default."
  type        = string
  default     = ""
}

variable "region" {
  description = "Per-cloud region/location. Empty string uses the module default."
  type        = string
  default     = ""
}

variable "ssh_public_key" {
  description = "SSH public key contents to install on the hosts."
  type        = string
}

variable "ssh_private_key_file" {
  description = "Path to the matching private key, written into the inventory for Ansible."
  type        = string
}

variable "allow_ssh_cidr" {
  description = "CIDR allowed to SSH to the hosts (e.g. your IP/32)."
  type        = string
}

variable "ttl_hours" {
  description = "Advisory TTL tag for the cost guard."
  type        = number
  default     = 4
}

variable "owner" {
  description = "Owner tag/label for resources."
  type        = string
  default     = "gomatch-bench"
}
