provider "digitalocean" {
  token = var.do_token
}

locals {
  peering_name = "tailnet-${var.deployment_id}"
}

resource "digitalocean_vpc_peering" "hub_spoke" {
  name = local.peering_name
  vpc_ids = [
    var.hub_vpc_id,
    var.spoke_vpc_id,
  ]
}
