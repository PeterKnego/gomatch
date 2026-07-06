# Token comes from HCLOUD_TOKEN env var (provider reads it automatically).
resource "hcloud_ssh_key" "bench" {
  name       = "${var.owner}-key"
  public_key = var.ssh_public_key
}

resource "hcloud_network" "bench" {
  name     = "${var.owner}-net"
  ip_range = "10.10.0.0/16"
}

resource "hcloud_network_subnet" "bench" {
  network_id   = hcloud_network.bench.id
  type         = "cloud"
  network_zone = "eu-central"
  ip_range     = "10.10.1.0/24"
}

resource "hcloud_firewall" "bench" {
  name = "${var.owner}-fw"
  rule {
    direction  = "in"
    protocol   = "tcp"
    port       = "22"
    source_ips = [var.allow_ssh_cidr]
  }
}

resource "hcloud_server" "node" {
  count        = var.node_count
  name         = "${var.owner}-node${count.index}"
  server_type  = local.instance_type
  image        = "ubuntu-24.04"
  location     = local.location
  ssh_keys     = [hcloud_ssh_key.bench.id]
  firewall_ids = [hcloud_firewall.bench.id]

  labels = {
    owner     = var.owner
    ttl_hours = tostring(var.ttl_hours)
    role      = "node${count.index}"
  }

  network {
    network_id = hcloud_network.bench.id
    ip         = "10.10.1.${count.index + 10}"
  }

  depends_on = [hcloud_network_subnet.bench]
}
