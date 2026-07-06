output "nodes" {
  value = [
    for i, s in google_compute_instance.node : {
      name       = s.name
      role       = "node${i}"
      public_ip  = s.network_interface[0].access_config[0].nat_ip
      private_ip = s.network_interface[0].network_ip
    }
  ]
}

output "ssh_user" {
  value = "ubuntu"
}
