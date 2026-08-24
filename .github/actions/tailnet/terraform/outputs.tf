output "peering_id" {
  description = "Cloud peering connection identifier"
  value = var.cloud_provider == "aws" ? (
    length(module.aws_peering) > 0 ? module.aws_peering[0].peering_id : ""
  ) : (
    length(module.digitalocean_peering) > 0 ? module.digitalocean_peering[0].peering_id : ""
  )
}
