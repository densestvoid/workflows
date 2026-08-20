---
name: terraform-output-inline
description: >-
  Reads Terraform outputs with terraform output -raw in the same job after
  deploy-terraform, replacing the removed terraform-output composite. Use when
  a deploy workflow needs service_url or other single-line outputs for notify,
  PR comments, or downstream steps in the same job.
---

# Terraform Output (inline)

## When to read this skill

- After **deploy-terraform** when the workflow needs a value from `terraform output`
- Replacing references to the removed **terraform-output** action
- Wiring notify/PR comment with a deployed URL or ID

Do **not** use this for cross-job output without re-init — keep the read in the deploy job.

## Pattern

Run in the **same job**, immediately after **deploy-terraform** succeeds:

```yaml
- uses: densestvoid/workflows/.github/actions/deploy-terraform@v1
  id: deploy
  env:
    TF_VAR_do_token: ${{ secrets.DO_TOKEN }}
  with:
    terraform-dir: terraform/pr
    backend-key: pr/pr-${{ github.event.pull_request.number }}.tfstate
    terraform-aws-access-key-id: ${{ secrets.TERRAFORM_AWS_ACCESS_KEY_ID }}
    terraform-aws-secret-access-key: ${{ secrets.TERRAFORM_AWS_SECRET_ACCESS_KEY }}
    terraform-aws-region: ${{ secrets.TERRAFORM_AWS_REGION }}

- name: Read service URL
  id: service-url
  if: steps.deploy.outcome == 'success'
  working-directory: terraform/pr
  run: |
    set -euo pipefail
    value=$(terraform output -raw service_url)
    echo "value=${value}" >> "$GITHUB_OUTPUT"
```

**deploy-terraform** runs `hashicorp/setup-terraform`, so `terraform` stays on PATH for later steps in that job.

## Constraints

- **Single-line values only** — `-raw` fails on multiline outputs
- One read step per output name; change the output key in `terraform output -raw <name>`
- Gate with `if: steps.deploy.outcome == 'success'` (not merely `success()` if deploy was skipped)
- `working-directory` must match **deploy-terraform**'s `terraform-dir`

## Alternative

[`dflook/terraform-output@v4`](https://github.com/dflook/terraform-github-actions) if an action is preferred over inline `run`. Default for this toolbox is inline.

## Anti-patterns

- Removed **terraform-output** composite — do not recreate it
- Reading outputs in a separate job without `terraform init`
