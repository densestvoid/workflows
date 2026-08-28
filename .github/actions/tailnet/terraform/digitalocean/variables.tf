variable "deployment_id" {
  description = "Deployment identifier used for resource naming (e.g. pr-42, production)"
  type        = string
}

variable "hub_vpc_id" {
  description = "Hub VPC UUID"
  type        = string
}

variable "spoke_vpc_id" {
  description = "Spoke VPC UUID to peer with the hub"
  type        = string
}

variable "spoke_cidr" {
  description = "Spoke VPC CIDR (unified connect/disconnect API; not used by DO peering)"
  type        = string
}

variable "do_token" {
  description = "DigitalOcean API token"
  type        = string
  sensitive   = true
}
