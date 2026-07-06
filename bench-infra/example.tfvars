cloud                = "hetzner"
instance_type        = ""               # "" = module default (ccx33)
region               = ""               # "" = module default
ssh_public_key       = "ssh-ed25519 AAAA... you@host"
ssh_private_key_file = "~/.ssh/id_ed25519"
allow_ssh_cidr       = "203.0.113.4/32" # your IP/32
ttl_hours            = 4
owner                = "gomatch-bench"
