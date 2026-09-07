provider "digitalocean" {
  token = var.do_token
}

data "digitalocean_vpc" "hub" {
  id = var.hub_vpc_id
}

data "digitalocean_firewall" "spoke" {
  firewall_id = var.spoke_firewall_id
}

resource "digitalocean_vpc_peering" "hub_spoke" {
  name = "tailnet-${var.deployment_id}"
  vpc_ids = [
    var.hub_vpc_id,
    var.spoke_vpc_id,
  ]
}

resource "digitalocean_firewall" "tailnet_hub" {
  name        = "tailnet-${var.deployment_id}"
  droplet_ids = data.digitalocean_firewall.spoke.droplet_ids
  tags        = data.digitalocean_firewall.spoke.tags

  inbound_rule {
    protocol         = "tcp"
    port_range       = "1-65535"
    source_addresses = [data.digitalocean_vpc.hub.ip_range]
  }

  inbound_rule {
    protocol         = "udp"
    port_range       = "1-65535"
    source_addresses = [data.digitalocean_vpc.hub.ip_range]
  }

  inbound_rule {
    protocol         = "icmp"
    source_addresses = [data.digitalocean_vpc.hub.ip_range]
  }
}
