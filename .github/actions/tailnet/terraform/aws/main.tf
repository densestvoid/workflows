provider "aws" {
  region = var.region
}

data "aws_vpc" "hub" {
  id = var.hub_vpc_id
}

data "aws_route_tables" "hub" {
  filter {
    name   = "vpc-id"
    values = [var.hub_vpc_id]
  }
}

data "aws_route_tables" "spoke" {
  filter {
    name   = "vpc-id"
    values = [var.spoke_vpc_id]
  }
}

resource "aws_vpc_peering_connection" "hub_spoke" {
  vpc_id      = var.hub_vpc_id
  peer_vpc_id = var.spoke_vpc_id
  auto_accept = true

  tags = {
    Name = "tailnet-${var.deployment_id}"
  }
}

resource "aws_route" "hub_to_spoke" {
  for_each = toset(data.aws_route_tables.hub.ids)

  route_table_id            = each.value
  destination_cidr_block    = var.spoke_cidr
  vpc_peering_connection_id = aws_vpc_peering_connection.hub_spoke.id
}

resource "aws_route" "spoke_to_hub" {
  for_each = toset(data.aws_route_tables.spoke.ids)

  route_table_id            = each.value
  destination_cidr_block    = data.aws_vpc.hub.cidr_block
  vpc_peering_connection_id = aws_vpc_peering_connection.hub_spoke.id
}
