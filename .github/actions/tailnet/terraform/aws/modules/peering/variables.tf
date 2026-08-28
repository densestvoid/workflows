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

variable "spoke_security_group_ids" {
  type    = list(string)
  default = []
}
