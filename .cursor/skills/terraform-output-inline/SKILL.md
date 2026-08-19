---
name: terraform-output-inline
description: >-
  Reads a Terraform output in the same job after deploy-terraform, without the
  removed terraform-output composite. Use when a workflow needs one output value
  (URL, ID) after apply in the same job.
---

# Terraform Output (inline)

The removed **terraform-output** action was a single `terraform output -raw` step. Use inline steps instead.

## Pattern

Run after **deploy-terraform** succeeds in the **same job**:

```yaml
- uses: densestvoid/workflows/.github/actions/deploy-terraform@main
  id: deploy
  env:
    TF_VAR_do_token: ${{ secrets.DO_TOKEN }}
  with:
    terraform-dir: terraform/pr
    backend-key: pr/pr-42.tfstate
    terraform-aws-access-key-id: ${{ secrets.TERRAFORM_AWS_ACCESS_KEY_ID }}
    terraform-aws-secret-access-key: ${{ secrets.TERRAFORM_AWS_SECRET_ACCESS_KEY }}
    terraform-aws-region: ${{ secrets.TERRAFORM_AWS_REGION }}

- name: Read service URL
  id: url
  if: steps.deploy.outcome == 'success'
  working-directory: terraform/pr
  run: |
    set -euo pipefail
    value=$(terraform output -raw service_url)
    echo "value=${value}" >> "$GITHUB_OUTPUT"

- run: echo "${{ steps.url.outputs.value }}"
```

**deploy-terraform** runs `hashicorp/setup-terraform`, so the CLI stays on PATH for later steps in that job.

## Notes

- **Single-line values only** — same limitation as the removed action (`-raw` breaks on multiline).
- Call once per output; repeat the read step with a different `name` input.
- Alternative: [`dflook/terraform-output@v4`](https://github.com/dflook/terraform-github-actions) if you prefer an action over inline `run`.
- Cross-job outputs require passing artifacts or re-running `terraform init` — prefer keeping read in the deploy job.
