# GOOGLE_PROJECT (or provider project) + GOOGLE_APPLICATION_CREDENTIALS from env.

resource "google_compute_network" "bench" {
  name                    = "${var.owner}-net"
  auto_create_subnetworks = false
}

resource "google_compute_subnetwork" "bench" {
  name          = "${var.owner}-subnet"
  ip_cidr_range = "10.10.1.0/24"
  region        = local.region
  network       = google_compute_network.bench.id
}

resource "google_compute_firewall" "ssh" {
  name          = "${var.owner}-ssh"
  network       = google_compute_network.bench.name
  source_ranges = [var.allow_ssh_cidr]
  allow {
    protocol = "tcp"
    ports    = ["22"]
  }
}

resource "google_compute_firewall" "intra" {
  name          = "${var.owner}-intra"
  network       = google_compute_network.bench.name
  source_ranges = ["10.10.1.0/24"]
  allow { protocol = "all" }
}

resource "google_compute_resource_policy" "compact" {
  name   = "${var.owner}-compact"
  region = local.region
  group_placement_policy {
    collocation = "COLLOCATED"
  }
}

resource "google_compute_instance" "node" {
  count        = var.node_count
  name         = "${var.owner}-node${count.index}"
  machine_type = local.machine_type
  zone         = local.zone

  boot_disk {
    initialize_params { image = "ubuntu-os-cloud/ubuntu-2404-lts-amd64" }
  }

  network_interface {
    subnetwork = google_compute_subnetwork.bench.id
    network_ip = "10.10.1.${count.index + 10}"
    access_config {} # ephemeral public IP
  }

  metadata = {
    ssh-keys = "ubuntu:${var.ssh_public_key}"
  }

  labels = {
    owner     = var.owner
    ttl_hours = tostring(var.ttl_hours)
    role      = "node${count.index}"
  }

  resource_policies = [google_compute_resource_policy.compact.id]
}
