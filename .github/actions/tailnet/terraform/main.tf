provider "aws" {
  region = var.region
}

provider "digitalocean" {
  token = var.do_token
}

module "aws_peering" {
  count  = var.cloud_provider == "aws" ? 1 : 0
  source = "./modules/aws-peering"

  deployment_id         = var.deployment_id
  hub_vpc_id            = var.hub_vpc_id
  spoke_vpc_id          = var.spoke_vpc_id
  spoke_cidr            = var.spoke_cidr
  hub_route_table_ids   = var.hub_route_table_ids
  spoke_route_table_ids = var.spoke_route_table_ids
}

module "digitalocean_peering" {
  count  = var.cloud_provider == "digitalocean" ? 1 : 0
  source = "./modules/digitalocean-peering"

  deployment_id = var.deployment_id
  hub_vpc_id    = var.hub_vpc_id
  spoke_vpc_id  = var.spoke_vpc_id
}
