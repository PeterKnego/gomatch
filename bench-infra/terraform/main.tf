locals {
  enable_hetzner = var.cloud == "hetzner"
  enable_aws     = var.cloud == "aws"
  enable_gcp     = var.cloud == "gcp"

  # Effective regions used by root-level provider blocks (mirror module defaults).
  aws_region = var.region != "" ? var.region : "us-east-1"
  gcp_region = var.region != "" ? var.region : "us-central1"
}

# Terraform configures EVERY declared provider even when its module is count=0,
# so the non-active clouds must configure without real credentials. When a cloud
# is NOT selected we feed throwaway static creds + skip the validation/metadata
# probes; its module has no resources, so the provider is never actually used.
# When the cloud IS selected, the dummies become null and the normal credential
# chain (env / profile / ADC) applies.
provider "aws" {
  region                      = local.aws_region
  access_key                  = local.enable_aws ? null : "unused-dummy"
  secret_key                  = local.enable_aws ? null : "unused-dummy"
  skip_credentials_validation = !local.enable_aws
  skip_requesting_account_id  = !local.enable_aws
  skip_metadata_api_check     = !local.enable_aws
}

provider "hcloud" {
  # 64-char dummy satisfies the provider's token-format check when Hetzner is
  # not the selected cloud (its module is count=0, so it is never used).
  token = local.enable_hetzner ? null : "0000000000000000000000000000000000000000000000000000000000000000"
}

provider "google" {
  region = local.gcp_region
  zone   = "${local.gcp_region}-a"
  # A static access_token short-circuits the Application Default Credentials
  # lookup; unused when count=0. Null when GCP is active → normal ADC/env path.
  access_token = local.enable_gcp ? null : "unused-dummy-token"
}

module "hetzner" {
  source = "./modules/hetzner"
  count  = local.enable_hetzner ? 1 : 0

  node_count     = var.node_count
  instance_type  = var.instance_type
  region         = var.region
  ssh_public_key = var.ssh_public_key
  allow_ssh_cidr = var.allow_ssh_cidr
  ttl_hours      = var.ttl_hours
  owner          = var.owner
}

module "aws" {
  source = "./modules/aws"
  count  = local.enable_aws ? 1 : 0

  node_count     = var.node_count
  instance_type  = var.instance_type
  region         = var.region
  ssh_public_key = var.ssh_public_key
  allow_ssh_cidr = var.allow_ssh_cidr
  ttl_hours      = var.ttl_hours
  owner          = var.owner
}

module "gcp" {
  source = "./modules/gcp"
  count  = local.enable_gcp ? 1 : 0

  node_count     = var.node_count
  instance_type  = var.instance_type
  region         = var.region
  ssh_public_key = var.ssh_public_key
  allow_ssh_cidr = var.allow_ssh_cidr
  ttl_hours      = var.ttl_hours
  owner          = var.owner
}

locals {
  active_module = (
    local.enable_hetzner ? module.hetzner[0] :
    local.enable_aws ? module.aws[0] :
    local.enable_gcp ? module.gcp[0] :
    null
  )
}
