variable "node_count" { type = number }
variable "instance_type" { type = string }
variable "region" { type = string }
variable "ssh_public_key" { type = string }
variable "allow_ssh_cidr" { type = string }
variable "ttl_hours" { type = number }
variable "owner" { type = string }

locals {
  instance_type = var.instance_type != "" ? var.instance_type : "ccx33" # 8 dedicated vCPU / 32GB
  location      = var.region != "" ? var.region : "nbg1"
  roles         = [for i in range(var.node_count) : "node${i}"]
}
