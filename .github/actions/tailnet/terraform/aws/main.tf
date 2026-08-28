provider "aws" {
  region = var.region
}

module "peering" {
  source = "./modules/peering"

  deployment_id            = var.deployment_id
  hub_vpc_id               = var.hub_vpc_id
  spoke_vpc_id             = var.spoke_vpc_id
  spoke_cidr               = var.spoke_cidr
  spoke_security_group_ids = var.spoke_security_group_ids
}
