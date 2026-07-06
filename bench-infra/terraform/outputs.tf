output "nodes" {
  description = "Ordered [node0, node1, node2] with role + public/private IPs."
  value       = local.active_module.nodes
}

output "ssh_user" {
  description = "SSH username for Ansible (per-cloud default image user)."
  value       = local.active_module.ssh_user
}
