variable "deployment_id" {
  description = "Deployment identifier used for resource naming (e.g. pr-42, production)"
  type        = string
}

variable "hub_vpc_id" {
  description = "Hub VPC identifier"
  type        = string
}

variable "spoke_vpc_id" {
  description = "Spoke VPC identifier to peer with the hub"
  type        = string
}

variable "spoke_cidr" {
  description = "Spoke VPC CIDR block (hub route tables)"
  type        = string
}

variable "region" {
  description = "AWS region"
  type        = string
}
