# DenseVoid Workflows

Composable GitHub Actions toolbox for CI, build, deploy, and notify.

**Toolbox, not orchestration.** This repo provides atomic, reusable actions. Each app repo owns workflow triggers, job graphs, `needs:` wiring, and which steps to call. Compose by repeating steps — one `build-go` per binary, one `build-docker` per Dockerfile, one `push-container` per image.

## Workflows

### Go Checks (`.github/workflows/go-checks.yml`)

Parallel Go quality checks: vet, staticcheck, golangci-lint, govulncheck, gosec.

```yaml
jobs:
  go-checks:
    uses: densestvoid/workflows/.github/workflows/go-checks.yml@main
```

Pair with **detect-changes** in the caller repo to skip when Go sources are unchanged. For repos with multiple `go.mod` files, pass `working-directory` to **go-checks** — see [Go module layout](#go-module-layout).

## Actions

| Action | Purpose |
|--------|---------|
| [detect-changes](.github/actions/detect-changes) | Path diff + `content-key` for skip gates and image tags |
| [build-go](.github/actions/build-go) | One Go binary → artifact (binary cache keyed by dep-tree source contents) |
| [build-docker](.github/actions/build-docker) | One Docker image → artifact (BuildKit `type=gha` layer cache) |
| [push-container](.github/actions/push-container) | Load image artifact; push to GHCR and/or Docker Hub |
| [deploy-terraform](.github/actions/deploy-terraform) | Terraform init + apply (`variables-json` → `-var-file`) |
| [terraform-output](.github/actions/terraform-output) | Read one Terraform output (same job, after deploy) |
| [terminate-terraform](.github/actions/terminate-terraform) | Empty-module destroy + S3 state delete |
| [notify](.github/actions/notify) | Slack + PR comment delivery |
| [setup-go](.github/actions/setup-go) | Checkout + Go toolchain |
| [install-go-tool](.github/actions/install-go-tool) | Install + cache a Go CLI tool (`~/go/bin/<tool>`) |

## Versioning

Action directory names are unversioned (`build-go`, `deploy-terraform`, …). Version lives in the **git ref**:

```yaml
uses: densestvoid/workflows/.github/actions/build-go@v1
uses: densestvoid/workflows/.github/actions/deploy-terraform@v1
```

Pin every action at the same repo ref (`@main` during v0, `@v1` when released). One tag = one tested snapshot of the whole toolbox. Bump all pins together on release.

| Ref | When |
|-----|------|
| `@main` | v0 iteration |
| `@v1`, `@v2` | Stable releases (major = breaking change to any action) |
| `@<sha>` | Debug pin |

Escape hatch: per-action tags like `@deploy-terraform/v1.1.0` when you need independent versioning.

## Secrets

Secrets live in **each app repo** (repository secrets/variables, or GitHub Environments). Callers pass `${{ secrets.* }}` as action inputs — the workflows repo does not hold cross-repo deploy credentials.

| Typical app secrets | Used by |
|---------------------|---------|
| `DO_TOKEN`, `TERRAFORM_AWS_*` | deploy/terminate terraform |
| `SLACK_WEBHOOK` | notify |
| `DOCKERHUB_*` | push-container |
| `GITHUB_TOKEN` or PAT | push-container (GHCR), notify (PR comments) |

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
          variables-json: '{"deployment_id":"pr-${{ github.event.pull_request.number }}","docker_image_tag":"${{ steps.push.outputs.image-ref }}"}'

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
    terraform-s3-bucket: densestvoid-terraform
    do-token: ${{ secrets.DO_TOKEN }}
    terraform-aws-access-key-id: ${{ secrets.TERRAFORM_AWS_ACCESS_KEY_ID }}
    terraform-aws-secret-access-key: ${{ secrets.TERRAFORM_AWS_SECRET_ACCESS_KEY }}
    terraform-aws-region: ${{ secrets.TERRAFORM_AWS_REGION }}
```

**terminate-terraform** uses a bundled empty destroy module at `${{ github.action_path }}/pr-destroy` — GitHub copies only the action directory onto the runner, so the module must live inside the action and versions with the action pin. Pass `terraform-s3-bucket` to match the bucket declared in your app Terraform backend (same value deploy init uses).

## Caching

| Action | Mechanism |
|--------|-----------|
| **build-go** | `actions/cache/restore` + `actions/cache/save` on binary at repo root; key from bundled `cachekey` helper (`go/packages` dep tree: `go.mod`/`go.sum`, sources, `EmbedFiles`; active `go` toolchain version) + `GOOS`/`GOARCH`. `setup-go` separately caches module download (`go.sum`). |
| **build-docker** | BuildKit `--cache-from` / `--cache-to type=gha` (`.dockerignore` applied natively) |
| **install-go-tool** | `actions/cache` on `~/go/bin/<tool>` per package |

Skip logic is caller-owned: use **detect-changes** `changed` in workflow `if:` conditions. Build actions do not expose cache-hit outputs.

## Caller notes

### Checkout

Every action that needs source code checks out the full repo itself. Callers should not pre-checkout for these actions. Chaining multiple actions in one job means multiple checkouts (wasteful but functional).

| Action | Checkout |
|--------|----------|
| detect-changes, build-go, build-docker, deploy-terraform, setup-go | Full repo |
| push-container, notify, terraform-output | None (artifacts / existing workspace) |
| terminate-terraform | Bundled `pr-destroy` module only |

### push-container (GHCR)

Requires `packages: write` and a token with `write:packages`. Uses `docker/login-action` for registry auth. Use a PAT when `GITHUB_TOKEN` lacks package scope.

### Go module layout

**build-go** expects `go.mod` at the repo root. Pass only `main-package`:

```yaml
- uses: densestvoid/workflows/.github/actions/build-go@main
  with:
    main-package: ./cmd/api
```

For nested modules (multiple `go.mod` in one repo), pass `working-directory` to **go-checks**:

```yaml
go-checks:
  uses: densestvoid/workflows/.github/workflows/go-checks.yml@main
  with:
    working-directory: backend
```

### Terraform

**deploy-terraform** writes a temp `ci.auto.tfvars.json` from `variables-json` and applies with `-var-file`. `do-token` is merged into the JSON automatically. App Terraform modules must declare `backend "s3"` with bucket and partial config; the action overrides `key` and `region` at init.

**terminate-terraform** uses a partial S3 backend in `pr-destroy` — pass `terraform-s3-bucket`, `backend-key`, and `terraform-aws-region` at init (same bucket your deploy module declares).

Terraform CLI version is resolved by `hashicorp/setup-terraform` (latest release, not pinned).

Read outputs in the same job:

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

`upload-artifact` stores files in GitHub's artifact service — they are not kept on disk for later steps. **build-docker** also runs a fresh checkout, which wipes the workspace. Pass prior artifact names via `artifacts` as a comma-separated list (e.g. `budget,worker`). **build-docker** passes them to `actions/download-artifact` `pattern` — no custom download scripting.

Split build and docker across jobs by downloading artifacts in the caller workflow between jobs.

## Known limitations (v0)

| Area | Limitation |
|------|------------|
| **build-go** | `CGO_ENABLED=0`; default `GOOS=linux` / `GOARCH=amd64` (override via inputs) — cgo packages won't build |
| **build-docker** | `actions/download-artifact` when `artifacts` is set; required because checkout wipes workspace |
| **Testing** | No integration tests in this repo — validate via krogerrecipeshopper rollout |

## Rollout

| Phase | Scope |
|-------|-------|
| **v0** | Build actions in workflows repo |
| **v0 test** | Validate composition with krogerrecipeshopper |
| **v1** | Cut `@v1` tag; create `app-deploy-template` |
| **v1 migrate** | Update budget (last) |

## Repository structure

```
.github/
├── workflows/go-checks.yml
├── actions/
│   ├── setup-go/
│   ├── install-go-tool/
│   ├── detect-changes/
│   ├── build-go/
│   │   └── cachekey/            # bundled Go helper for dep-tree fingerprint
│   ├── build-docker/
│   ├── push-container/
│   ├── deploy-terraform/
│   ├── terraform-output/
│   ├── terminate-terraform/
│   │   └── pr-destroy/          # bundled empty destroy module
│   └── notify/
```
