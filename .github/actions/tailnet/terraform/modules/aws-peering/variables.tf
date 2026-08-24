variable "deployment_id" {
  type = string
}

variable "hub_vpc_id" {
  type = string
}

variable "spoke_vpc_id" {
  type = string
}

variable "spoke_cidr" {
  type = string
}

variable "hub_route_table_ids" {
  type    = list(string)
  default = []
}

variable "spoke_route_table_ids" {
  type    = list(string)
  default = []
}
