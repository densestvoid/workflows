---
name: tailnet-vpc-peering
description: >-
  Hub-and-spoke VPC peering for Tailscale subnet router access via connect-tailnet
  and disconnect-tailnet. Use when wiring tailnet connectivity after deploy-terraform,
  authoring those actions, or reviewing tailnet/terraform changes.
---

# Tailnet VPC peering

## When to read this skill

- After **deploy-terraform** when a spoke VPC needs tailnet reachability
- On PR close, before **terminate-terraform**
- Authoring or reviewing **connect-tailnet** / **disconnect-tailnet** or `.github/actions/tailnet/terraform`

## Prerequisites (subnet router — one-time)

Documented in full in **connect-tailnet** `description`. Summary:

1. Subnet router in **hub VPC** with IP forwarding enabled
2. `tailscale set --advertise-routes=10.200.0.0/16` (supernet; PR spokes use `/24` slices)
3. Node tagged `tag:subnet-router`
4. Tailnet ACL `autoApprovers`:

```json
"autoApprovers": {
  "routes": {
    "tag:subnet-router": ["10.200.0.0/16"]
  }
}
```

**connect-tailnet** creates cloud peering only — not Tailscale routes.

## Spoke isolation (no iptables)

Spoke↔spoke traffic is blocked by **topology**:

- Only **hub↔spoke** peering is created; spoke↔spoke peering never exists
- Cloud VPC peering is **non-transitive**

Tailnet path: client → hub router → one spoke. No hub-router OS firewall rules required for this model.

## Constraints

- **Same region:** hub VPC and spoke VPC must be in the same cloud region
- **PR/ephemeral spokes:** AWS module updates all route tables in each VPC — intended for small PR VPCs, not complex production hub layouts

## Terraform layout

```
.github/actions/tailnet/terraform/
  aws/           # provider aws only
  digitalocean/  # provider digitalocean only
```

State key: `tailnet/spokes/<deployment-id>.tfstate`

| Module | Resources |
|--------|-----------|
| `aws/modules/peering` | VPC peering, routes, optional SG ingress from hub CIDR |
| `digitalocean/modules/peering` | `digitalocean_vpc_peering` |

## Credentials

| Secret | When required |
|--------|----------------|
| `TERRAFORM_AWS_*` | Always (S3 state backend) |
| `DO_TOKEN` → `digitalocean-token` | `cloud-provider: digitalocean` only |

## App terraform outputs

```hcl
output "vpc_id" { value = digitalocean_vpc.main.id }
output "vpc_cidr" { value = digitalocean_vpc.main.ip_range }
# AWS only, when using security groups:
output "security_group_id" { value = aws_security_group.app.id }
```

## Connect (after deploy)

```yaml
- uses: densestvoid/workflows/.github/actions/connect-tailnet@main
  if: steps.deploy.outcome == 'success'
  with:
    deployment-id: pr-${{ github.event.pull_request.number }}
    hub-vpc-id: ${{ vars.TAILNET_HUB_VPC_ID }}
    spoke-vpc-id: ${{ steps.vpc.outputs.id }}
    spoke-cidr: ${{ steps.vpc.outputs.cidr }}
    cloud-provider: digitalocean
    region: nyc3
    digitalocean-token: ${{ secrets.DO_TOKEN }}
    terraform-aws-access-key-id: ${{ secrets.TERRAFORM_AWS_ACCESS_KEY_ID }}
    terraform-aws-secret-access-key: ${{ secrets.TERRAFORM_AWS_SECRET_ACCESS_KEY }}
    terraform-aws-region: ${{ secrets.TERRAFORM_AWS_REGION }}
```

**AWS only** — optional ingress from hub VPC CIDR on spoke security groups:

```yaml
    cloud-provider: aws
    region: us-east-1
    spoke-security-group-ids: ${{ steps.vpc.outputs.security-group-id }}
```

Omit `digitalocean-token` when `cloud-provider` is `aws`.

**DigitalOcean** — peering handles routing between VPCs. If spoke workloads use a DO Cloud Firewall, allow the hub VPC CIDR in app terraform (this action does not manage DO firewalls — importing a shared firewall into tailnet state would risk deleting it on disconnect).

## Disconnect (PR close, before terminate)

Pass the **same** values as connect for `deployment-id`, hub/spoke IDs, `spoke-cidr`, `cloud-provider`, and credentials.

## Anti-patterns

- Peering spoke↔spoke directly
- Running **terminate-terraform** before **disconnect-tailnet**
- Expecting **connect-tailnet** to configure Tailscale on the router
