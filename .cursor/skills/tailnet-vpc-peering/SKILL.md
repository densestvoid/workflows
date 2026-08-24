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

## Architecture

Long-lived Tailscale **subnet router** lives in a **hub VPC**. Each deployed spoke VPC peers **only** to the hub — spokes never peer to each other (cloud peering is non-transitive).

```
Tailnet client → Tailscale tunnel → subnet router (hub VPC) → hub↔spoke peering → spoke resource
```

**connect-tailnet** / **disconnect-tailnet** manage cloud peering only. Tailscale route advertisement stays on the manually bootstrapped router (advertise a spoke supernet once, e.g. `10.200.0.0/16`, with ACL `autoApprovers` for `tag:subnet-router`).

Spoke isolation also needs hub-router `iptables`/`nftables` DROP forward rules between spoke CIDRs (one-time manual setup).

## Terraform layout

Shared root at `.github/actions/tailnet/terraform/` — **not** nested under either action:

| Module | Cloud | Resources |
|--------|-------|-----------|
| `modules/aws-peering` | AWS | VPC peering, hub↔spoke routes |
| `modules/digitalocean-peering` | DigitalOcean | `digitalocean_vpc_peering` |

State key (derived inside actions): `tailnet/spokes/<deployment-id>.tfstate`

## Repo secrets

Same as **deploy-terraform** — wire explicitly per action step:

| Secret | Input |
|--------|-------|
| `DO_TOKEN` | `digitalocean-token` |
| `TERRAFORM_AWS_ACCESS_KEY_ID` | `terraform-aws-access-key-id` |
| `TERRAFORM_AWS_SECRET_ACCESS_KEY` | `terraform-aws-secret-access-key` |
| `TERRAFORM_AWS_REGION` | `terraform-aws-region` |

Repo variable `TAILNET_HUB_VPC_ID` (or equivalent) for `hub-vpc-id`.

## App terraform outputs

```hcl
output "vpc_id" { value = digitalocean_vpc.main.id }
output "vpc_cidr" { value = digitalocean_vpc.main.ip_range }
```

Read in the deploy job after **deploy-terraform** (see [terraform-output-inline](terraform-output-inline/SKILL.md)).

## Connect (after deploy)

```yaml
- uses: densestvoid/workflows/.github/actions/connect-tailnet@main
  if: steps.deploy.outcome == 'success'
  with:
    deployment-id: pr-${{ github.event.pull_request.number }}
    hub-vpc-id: ${{ vars.TAILNET_HUB_VPC_ID }}
    spoke-vpc-id: ${{ steps.vpc.outputs.id }}
    spoke-cidr: ${{ steps.vpc.outputs.cidr }}
    digitalocean-token: ${{ secrets.DO_TOKEN }}
    terraform-aws-access-key-id: ${{ secrets.TERRAFORM_AWS_ACCESS_KEY_ID }}
    terraform-aws-secret-access-key: ${{ secrets.TERRAFORM_AWS_SECRET_ACCESS_KEY }}
    terraform-aws-region: ${{ secrets.TERRAFORM_AWS_REGION }}
```

Optional inputs: `cloud-provider` (`aws` | `digitalocean`, default `digitalocean`), `region` (default `nyc3`), comma-separated `hub-route-table-ids` / `spoke-route-table-ids` for AWS.

## Disconnect (PR close, before terminate)

Pass the **same** `deployment-id`, `hub-vpc-id`, and spoke values as connect — Terraform validates root variables on destroy.

```yaml
- uses: densestvoid/workflows/.github/actions/disconnect-tailnet@main
  with:
    deployment-id: pr-${{ github.event.pull_request.number }}
    hub-vpc-id: ${{ vars.TAILNET_HUB_VPC_ID }}
    spoke-vpc-id: ${{ steps.vpc.outputs.id }}
    spoke-cidr: ${{ steps.vpc.outputs.cidr }}
    digitalocean-token: ${{ secrets.DO_TOKEN }}
    terraform-aws-access-key-id: ${{ secrets.TERRAFORM_AWS_ACCESS_KEY_ID }}
    terraform-aws-secret-access-key: ${{ secrets.TERRAFORM_AWS_SECRET_ACCESS_KEY }}
    terraform-aws-region: ${{ secrets.TERRAFORM_AWS_REGION }}

- uses: densestvoid/workflows/.github/actions/terminate-terraform@main
  # ...
```

## Action behavior

| Action | Checkout | Terraform op |
|--------|----------|--------------|
| **connect-tailnet** | None (bundled terraform in action ref) | `apply` |
| **disconnect-tailnet** | None | `destroy` + S3 state delete |

## Anti-patterns

- Peering spoke↔spoke directly
- Running **terminate-terraform** before **disconnect-tailnet** (hub keeps routing to a dying spoke)
- Expecting these actions to SSH to the subnet router or manage Tailscale ACLs (v1 scope is cloud peering only)
