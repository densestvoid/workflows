locals {
  peering_name = "tailnet-${var.deployment_id}"
}

data "aws_vpc" "hub" {
  id = var.hub_vpc_id
}

data "aws_route_tables" "hub" {
  count = length(var.hub_route_table_ids) == 0 ? 1 : 0

  filter {
    name   = "vpc-id"
    values = [var.hub_vpc_id]
  }
}

data "aws_route_tables" "spoke" {
  count = length(var.spoke_route_table_ids) == 0 ? 1 : 0

  filter {
    name   = "vpc-id"
    values = [var.spoke_vpc_id]
  }
}

locals {
  hub_route_table_ids = length(var.hub_route_table_ids) > 0 ? var.hub_route_table_ids : data.aws_route_tables.hub[0].ids
  spoke_route_table_ids = length(var.spoke_route_table_ids) > 0 ? var.spoke_route_table_ids : data.aws_route_tables.spoke[0].ids
  hub_cidr              = data.aws_vpc.hub.cidr_block
}

resource "aws_vpc_peering_connection" "hub_spoke" {
  vpc_id      = var.hub_vpc_id
  peer_vpc_id = var.spoke_vpc_id
  auto_accept = true

  tags = {
    Name = local.peering_name
  }
}

resource "aws_route" "hub_to_spoke" {
  for_each = toset(local.hub_route_table_ids)

  route_table_id            = each.value
  destination_cidr_block    = var.spoke_cidr
  vpc_peering_connection_id = aws_vpc_peering_connection.hub_spoke.id
}

resource "aws_route" "spoke_to_hub" {
  for_each = toset(local.spoke_route_table_ids)

  route_table_id            = each.value
  destination_cidr_block    = local.hub_cidr
  vpc_peering_connection_id = aws_vpc_peering_connection.hub_spoke.id
}
