output "nodes" {
  value = [
    for i, s in hcloud_server.node : {
      name       = s.name
      role       = "node${i}"
      public_ip  = s.ipv4_address
      private_ip = "10.10.1.${i + 10}"
    }
  ]
}

output "ssh_user" {
  value = "root" # Hetzner Ubuntu images log in as root
}
