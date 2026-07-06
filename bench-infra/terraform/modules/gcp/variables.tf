variable "node_count" { type = number }
variable "instance_type" { type = string }
variable "region" { type = string }
variable "ssh_public_key" { type = string }
variable "allow_ssh_cidr" { type = string }
variable "ttl_hours" { type = number }
variable "owner" { type = string }

locals {
  machine_type = var.instance_type != "" ? var.instance_type : "c3-highcpu-8"
  region       = var.region != "" ? var.region : "us-central1"
  zone         = "${local.region}-a"
}
