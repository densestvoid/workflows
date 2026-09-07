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

variable "spoke_firewall_id" {
  description = "Spoke Cloud Firewall ID from app terraform; tailnet adds a companion firewall on the same droplets/tags"
  type        = string
}

variable "do_token" {
  description = "DigitalOcean API token"
  type        = string
  sensitive   = true
}
