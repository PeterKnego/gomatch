# AWS example. Copy to terraform.tfvars (gitignored) and fill in your values:
#     cp example.aws.tfvars terraform.tfvars
# Credentials are NOT here — set AWS_PROFILE or AWS_ACCESS_KEY_ID/SECRET in .env
# (see .env.example). Use a scoped IAM principal, NOT root account keys.
cloud = "aws"

# c7i.4xlarge: sized for SUSTAINED (non-bursting) network bandwidth — smaller
# instances get credit-based "up to X Gbps" that adds RTT jitter at the top of
# the rate ladder. ~$0.71/hr each on-demand in us-east-1; 3 hosts × a few
# hours ≈ a few dollars. Nothing auto-reaps: run `make destroy` yourself.
instance_type = "c7i.4xlarge"

# Same-region single-AZ + cluster placement group (set by the module) keeps
# the inter-node private path low-latency.
region = "us-east-1"

ssh_public_key       = "ssh-ed25519 AAAA... you@host"  # AWS key pairs accept ed25519
ssh_private_key_file = "~/.ssh/id_ed25519"

allow_ssh_cidr = "203.0.113.4/32"  # your IP/32 — NOT 0.0.0.0/0

ttl_hours = 4          # advisory tag only — nothing auto-reaps; run `make destroy`
owner     = "gomatch-bench"
