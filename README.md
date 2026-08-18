# DenseVoid Workflows

Composable GitHub Actions toolbox for CI, build, deploy, and notify.

Apps own workflow triggers, job graphs, and composition. This repo provides atomic steps only.

## Workflows

### Go Checks (`.github/workflows/go-checks.yml`)

Parallel Go quality checks: vet, staticcheck, golangci-lint, govulncheck, gosec.

```yaml
jobs:
  go-checks:
    uses: densestvoid/workflows/.github/workflows/go-checks.yml@main
    with:
      working-directory: '.'
```

Pair with **detect-changes** in the caller repo to skip when Go sources are unchanged.

## Actions

| Action | Purpose |
|--------|---------|
| [detect-changes](.github/actions/detect-changes) | Path diff + `content-key` for skip gates and image tags |
| [build-go](.github/actions/build-go) | One Go binary → artifact (`go list -deps` cache) |
| [build-docker](.github/actions/build-docker) | One Docker image → artifact (BuildKit `type=gha` layer cache) |
| [push-container](.github/actions/push-container) | Load image artifact; push to GHCR and/or Docker Hub |
| [deploy-terraform](.github/actions/deploy-terraform) | Terraform init + apply (`-var` flags) |
| [terraform-output](.github/actions/terraform-output) | Read one Terraform output (same job, after deploy) |
| [terminate-terraform](.github/actions/terminate-terraform) | Empty-module destroy + S3 state delete |
| [notify](.github/actions/notify) | Slack + PR comment delivery |
| [setup-go](.github/actions/setup-go) | Checkout + Go toolchain |
| [install-go-tool](.github/actions/install-go-tool) | Install + cache a Go CLI tool (`~/go/bin/<tool>`) |

Pin every action at the same ref (e.g. `@main` during v0, `@v1` when released).

## Typical PR deploy pipeline

```yaml
permissions:
  contents: read
  packages: write   # required for push-container → GHCR

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: densestvoid/workflows/.github/actions/detect-changes@main
        id: deploy-changes
        with:
          paths: |
            **/*.go
            go.mod
            go.sum
            Dockerfile
            terraform/**

      - uses: densestvoid/workflows/.github/actions/build-go@main
        id: build-go
        if: steps.deploy-changes.outputs.changed == 'true'
        with:
          main-package: ./cmd/server
          artifact-name: budget

      - uses: densestvoid/workflows/.github/actions/build-docker@main
        id: docker
        if: steps.deploy-changes.outputs.changed == 'true'
        with:
          dockerfile: Dockerfile
          artifacts: ${{ steps.build-go.outputs.artifact-name }}

      - uses: densestvoid/workflows/.github/actions/push-container@main
        id: push
        if: steps.deploy-changes.outputs.changed == 'true'
        with:
          image-artifact: ${{ steps.docker.outputs.artifact-name }}
          tag: pr-${{ github.event.pull_request.number }}-${{ steps.deploy-changes.outputs.content-key }}
          ghcr-image: ${{ github.repository }}/my-app
          ghcr-username: ${{ github.actor }}
          ghcr-password: ${{ secrets.GITHUB_TOKEN }}

      - uses: densestvoid/workflows/.github/actions/deploy-terraform@main
        if: steps.deploy-changes.outputs.changed == 'true'
        with:
          terraform-dir: terraform/pr
          backend-key: pr/pr-${{ github.event.pull_request.number }}.tfstate
          do-token: ${{ secrets.DO_TOKEN }}
          terraform-aws-access-key-id: ${{ secrets.TERRAFORM_AWS_ACCESS_KEY_ID }}
          terraform-aws-secret-access-key: ${{ secrets.TERRAFORM_AWS_SECRET_ACCESS_KEY }}
          terraform-aws-region: ${{ secrets.TERRAFORM_AWS_REGION }}
          variables: |
            deployment_id=pr-${{ github.event.pull_request.number }}
            docker_image_tag=${{ steps.push.outputs.image-ref }}

      - uses: densestvoid/workflows/.github/actions/terraform-output@main
        id: service-url
        if: steps.deploy-changes.outputs.changed == 'true'
        with:
          terraform-dir: terraform/pr
          name: service_url

      - uses: densestvoid/workflows/.github/actions/notify@main
        if: always() && steps.deploy-changes.outputs.changed == 'true'
        with:
          slack-webhook: ${{ secrets.SLACK_WEBHOOK }}
          slack-text: 'Deployed PR #${{ github.event.pull_request.number }}: ${{ steps.service-url.outputs.value }}'
          pr-number: ${{ github.event.pull_request.number }}
          pr-body: 'Deployed to ${{ steps.service-url.outputs.value }}'
          github-token: ${{ secrets.GITHUB_TOKEN }}
```

### CI skip (Go Checks)

```yaml
jobs:
  changes:
    runs-on: ubuntu-latest
    outputs:
      changed: ${{ steps.go-changes.outputs.changed }}
    steps:
      - uses: densestvoid/workflows/.github/actions/detect-changes@main
        id: go-changes
        with:
          paths: |
            **/*.go
            go.mod
            go.sum

  go-checks:
    needs: changes
    if: needs.changes.outputs.changed == 'true'
    uses: densestvoid/workflows/.github/workflows/go-checks.yml@main
```

### PR terminate

```yaml
- uses: densestvoid/workflows/.github/actions/terminate-terraform@main
  with:
    backend-key: pr/pr-${{ github.event.pull_request.number }}.tfstate
    do-token: ${{ secrets.DO_TOKEN }}
    terraform-aws-access-key-id: ${{ secrets.TERRAFORM_AWS_ACCESS_KEY_ID }}
    terraform-aws-secret-access-key: ${{ secrets.TERRAFORM_AWS_SECRET_ACCESS_KEY }}
    terraform-aws-region: ${{ secrets.TERRAFORM_AWS_REGION }}
```

## Caching

| Action | Mechanism |
|--------|-----------|
| **build-go** | `actions/cache` keyed on `go list -deps` file hash + `go.mod`/`go.sum` |
| **build-docker** | BuildKit `--cache-from` / `--cache-to type=gha` (`.dockerignore` applied natively) |
| **install-go-tool** | `actions/cache` on `~/go/bin/<tool>` per package |

Skip logic is caller-owned: use **detect-changes** `changed` in workflow `if:` conditions. Build actions do not expose cache-hit outputs.

## Caller notes

### push-container (GHCR)

Requires `packages: write` and a token with `write:packages`. Use a PAT when `GITHUB_TOKEN` lacks package scope.

### Terraform outputs

**deploy-terraform** does init + apply only. Read outputs in the same job:

```yaml
- uses: densestvoid/workflows/.github/actions/terraform-output@main
  id: url
  with:
    terraform-dir: terraform
    name: service_url

- run: echo "${{ steps.url.outputs.value }}"
```

Call once per output. Or run `terraform output -raw <name>` directly — `setup-terraform` leaves the CLI on PATH for the job.

### build-docker artifacts

**build-docker** downloads **build-go** artifacts via `gh run download` in the same workflow run. Split build and docker across jobs only if the caller downloads artifacts between jobs.

## Repository structure

```
.github/
├── workflows/go-checks.yml
├── actions/
│   ├── setup-go/
│   ├── install-go-tool/
│   ├── detect-changes/
│   ├── build-go/
│   ├── build-docker/
│   ├── push-container/
│   ├── deploy-terraform/
│   ├── terraform-output/
│   ├── terminate-terraform/
│   │   └── pr-destroy/          # bundled empty destroy module
│   └── notify/
```

## Internal constants

- Terraform version: `1.5.7`
- S3 state bucket: `densestvoid-terraform`
