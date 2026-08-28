provider "digitalocean" {
  token = var.do_token
}

module "peering" {
  source = "./modules/peering"

  deployment_id = var.deployment_id
  hub_vpc_id    = var.hub_vpc_id
  spoke_vpc_id  = var.spoke_vpc_id
}
