variable "cloud_provider" {
  description = "Cloud provider for spoke peering: aws or digitalocean"
  type        = string

  validation {
    condition     = contains(["aws", "digitalocean"], var.cloud_provider)
    error_message = "cloud_provider must be aws or digitalocean."
  }
}

variable "deployment_id" {
  description = "Deployment identifier used for resource naming (e.g. pr-42, production)"
  type        = string
}

variable "hub_vpc_id" {
  description = "Hub VPC identifier (AWS VPC ID or DigitalOcean VPC UUID)"
  type        = string
}

variable "spoke_vpc_id" {
  description = "Spoke VPC identifier to peer with the hub"
  type        = string
}

variable "spoke_cidr" {
  description = "Spoke VPC CIDR block (used for AWS hub route tables)"
  type        = string
}

variable "region" {
  description = "Cloud region (AWS region or DigitalOcean region slug)"
  type        = string
}

variable "do_token" {
  description = "DigitalOcean API token (required when cloud_provider is digitalocean)"
  type        = string
  sensitive   = true
  default     = ""
}
