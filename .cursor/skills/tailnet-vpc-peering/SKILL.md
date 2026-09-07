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

**connect-tailnet** creates cloud peering and firewall rules for hub reachability — not Tailscale routes.

## Spoke firewall requirement

Tailnet access needs both **routing** (peering) and **firewall allow** (hub VPC CIDR).

| Cloud | App terraform must provide | connect-tailnet configures |
|-------|---------------------------|----------------------------|
| AWS | Security groups on spoke instances | SG ingress from hub VPC CIDR |
| DigitalOcean | A Cloud Firewall on spoke droplets | Companion firewall on same droplets/tags |

DigitalOcean has no standalone firewall-rule resource (unlike AWS `aws_vpc_security_group_ingress_rule`). connect-tailnet creates a **second** firewall (`tailnet-<deployment-id>`) scoped to the same droplets/tags as the app firewall. DO unions allow rules across firewalls; disconnect destroys only the tailnet firewall.

## Spoke isolation (no iptables)

Spoke↔spoke traffic is blocked by **topology**:

- Only **hub↔spoke** peering is created; spoke↔spoke peering never exists
- Cloud VPC peering is **non-transitive**

Tailnet path: client → hub router → one spoke. No hub-router OS firewall rules required for this model.

## Constraints

- **Same region:** hub VPC and spoke VPC must be in the same cloud region
- **PR/ephemeral spokes:** AWS connect root updates all route tables in each VPC — intended for small PR VPCs, not complex production hub layouts
- **Per-deployment firewalls:** use a dedicated spoke Cloud Firewall per deployment (not a shared production firewall)

## Terraform layout

```
.github/actions/tailnet/terraform/
  aws/              # connect: VPC peering + routes + SG ingress
  aws/destroy/      # disconnect: empty root (same state key)
  digitalocean/     # connect: VPC peering + tailnet hub firewall
  digitalocean/destroy/
```

State key: `tailnet/spokes/<deployment-id>.tfstate`

**disconnect-tailnet** applies the empty destroy root against the same state key (like **terminate-terraform**) — callers pass only `deployment-id` and credentials, not spoke resource IDs.

## Credentials

| Secret | When required |
|--------|----------------|
| `TERRAFORM_AWS_*` | Always (S3 state backend) |
| `DO_TOKEN` → `digitalocean-token` | `cloud-provider: digitalocean` only |

## Region input

| `cloud-provider` | `region` input |
|----------------|----------------|
| `digitalocean` | Optional; defaults to `nyc3` when omitted |
| `aws` | **Required** — use an AWS region (e.g. `us-east-1`), not a DO slug |

## App terraform outputs

```hcl
output "vpc_id" { value = digitalocean_vpc.main.id }
output "firewall_id" { value = digitalocean_firewall.app.id }

# AWS connect also needs:
output "vpc_cidr" { value = aws_vpc.main.cidr_block }
output "security_group_id" { value = aws_security_group.app.id }
```

Example DO app firewall (connect-tailnet adds hub access separately):

```hcl
resource "digitalocean_firewall" "app" {
  name = "pr-${var.pr_number}"
  tags = [digitalocean_tag.app.id]
  # app-specific inbound/outbound rules only
}
```

## Connect (after deploy)

**DigitalOcean:**

```yaml
- uses: densestvoid/workflows/.github/actions/connect-tailnet@main
  if: steps.deploy.outcome == 'success'
  with:
    deployment-id: pr-${{ github.event.pull_request.number }}
    hub-vpc-id: ${{ vars.TAILNET_HUB_VPC_ID }}
    spoke-vpc-id: ${{ steps.vpc.outputs.id }}
    spoke-firewall-id: ${{ steps.vpc.outputs.firewall-id }}
    cloud-provider: digitalocean
    digitalocean-token: ${{ secrets.DO_TOKEN }}
    terraform-aws-access-key-id: ${{ secrets.TERRAFORM_AWS_ACCESS_KEY_ID }}
    terraform-aws-secret-access-key: ${{ secrets.TERRAFORM_AWS_SECRET_ACCESS_KEY }}
    terraform-aws-region: ${{ secrets.TERRAFORM_AWS_REGION }}
```

**AWS:**

```yaml
    cloud-provider: aws
    region: us-east-1
    spoke-cidr: ${{ steps.vpc.outputs.cidr }}
    spoke-security-group-ids: ${{ steps.vpc.outputs.security-group-id }}
```

## Disconnect (PR close, before terminate)

Only `deployment-id`, `cloud-provider`, and credentials — same `cloud-provider` as connect:

```yaml
- uses: densestvoid/workflows/.github/actions/disconnect-tailnet@main
  with:
    deployment-id: pr-${{ github.event.pull_request.number }}
    cloud-provider: digitalocean
    digitalocean-token: ${{ secrets.DO_TOKEN }}
    terraform-aws-access-key-id: ${{ secrets.TERRAFORM_AWS_ACCESS_KEY_ID }}
    terraform-aws-secret-access-key: ${{ secrets.TERRAFORM_AWS_SECRET_ACCESS_KEY }}
    terraform-aws-region: ${{ secrets.TERRAFORM_AWS_REGION }}
```

## Anti-patterns

- Peering spoke↔spoke directly
- Running **terminate-terraform** before **disconnect-tailnet**
- Expecting **connect-tailnet** to configure Tailscale on the router
- Using `region: nyc3` with `cloud-provider: aws`
- Sharing one production Cloud Firewall across PR spokes (disconnect removes the tailnet companion firewall for that deployment only; shared app firewalls are fine if per-deployment)
