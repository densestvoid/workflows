# DenseVoid Workflows

Composable GitHub Actions toolbox for CI, build, deploy, and notify.

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
| [detect-changes](.github/actions/detect-changes) | Path diff + content hash for any glob set |
| [build-go](.github/actions/build-go) | One Go binary → artifact (cached) |
| [build-docker](.github/actions/build-docker) | One Docker image → artifact (cached) |
| [push-container](.github/actions/push-container) | Load image artifact; push to GHCR and/or Docker Hub |
| [deploy-terraform](.github/actions/deploy-terraform) | Terraform init + apply |
| [terminate-terraform](.github/actions/terminate-terraform) | Empty-module destroy + S3 state delete |
| [notify](.github/actions/notify) | Slack + PR comment delivery |
| [setup-go](.github/actions/setup-go) | Checkout + Go toolchain |
| [install-go-tool](.github/actions/install-go-tool) | Install + cache a Go CLI tool |

Pin actions at the same ref (e.g. `@main` during v0, `@v1` when released):

```yaml
- uses: densestvoid/workflows/.github/actions/build-go@main
  with:
    content-key: ${{ steps.go-changes.outputs.content-key }}
    main-package: ./cmd/server
    artifact-name: budget   # or bin/server — path within artifact
```

## Typical build pipeline

```yaml
steps:
  - uses: densestvoid/workflows/.github/actions/detect-changes@main
    id: deploy-changes
    with:
      paths: |
        **/*.go
        go.mod
        go.sum
        Dockerfile

  - uses: densestvoid/workflows/.github/actions/build-go@main
    id: build-go
    if: steps.deploy-changes.outputs.changed == 'true'
    with:
      content-key: ${{ steps.deploy-changes.outputs.content-key }}
      main-package: ./cmd/server

  - uses: densestvoid/workflows/.github/actions/build-docker@main
    id: docker
    if: steps.deploy-changes.outputs.changed == 'true'
    with:
      artifacts: ${{ steps.build-go.outputs.artifact-name }}
      dockerfile: Dockerfile

  - uses: densestvoid/workflows/.github/actions/push-container@main
    id: check
    if: steps.deploy-changes.outputs.changed == 'true'
    with:
      image-artifact: ${{ steps.docker.outputs.artifact-name }}
      tag: pr-123-${{ steps.deploy-changes.outputs.content-key }}
      check-only: true
      ghcr-image: ${{ github.repository }}/my-app
      ghcr-username: ${{ github.actor }}
      ghcr-password: ${{ secrets.GITHUB_TOKEN }}

  - uses: densestvoid/workflows/.github/actions/push-container@main
    if: steps.check.outputs.exists != 'true'
    with:
      image-artifact: ${{ steps.docker.outputs.artifact-name }}
      tag: pr-123-${{ steps.deploy-changes.outputs.content-key }}
      ghcr-image: ${{ github.repository }}/my-app
      ghcr-username: ${{ github.actor }}
      ghcr-password: ${{ secrets.GITHUB_TOKEN }}
```

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
│   ├── terminate-terraform/
│   │   └── pr-destroy/          # bundled empty destroy module
│   └── notify/
```

## Internal constants

- Terraform version: `1.5.7`
- S3 state bucket: `densestvoid-terraform`
