terraform {
  required_version = ">= 1.0"

  backend "s3" {
    bucket = "densestvoid-terraform"
    key    = "pr/placeholder.tfstate"
    region = "us-east-1"
  }

  required_providers {
    digitalocean = {
      source  = "digitalocean/digitalocean"
      version = "~> 2.0"
    }
    null = {
      source  = "hashicorp/null"
      version = "~> 3.0"
    }
  }
}

provider "digitalocean" {
  token = var.do_token
}
