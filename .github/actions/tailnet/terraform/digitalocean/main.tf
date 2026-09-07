provider "digitalocean" {
  token = var.do_token
}

resource "digitalocean_vpc_peering" "hub_spoke" {
  name = "tailnet-${var.deployment_id}"
  vpc_ids = [
    var.hub_vpc_id,
    var.spoke_vpc_id,
  ]
}
