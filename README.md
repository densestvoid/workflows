# DensestVoid Workflows

Composable GitHub Actions toolbox for CI, build, deploy, and notify.

**Toolbox, not orchestration.** This repo provides atomic, reusable actions. Each app repo owns workflow triggers, job graphs, `needs:` wiring, and which steps to call. Compose by repeating steps — one `build-go` per binary, one `build-docker` per image.

## Workflows

### Go Checks (`.github/workflows/go-checks.yml`)

Parallel Go quality checks: vet, staticcheck, golangci-lint, govulncheck, gosec.

```yaml
jobs:
  go-checks:
    uses: densestvoid/workflows/.github/workflows/go-checks.yml@main
```

Pair with [`dorny/paths-filter`](https://github.com/dorny/paths-filter) in the caller repo to skip jobs when sources are unchanged — see [Path filters](#path-filters). For repos with multiple `go.mod` files, pass `working-directory` to **go-checks** — see [Go module layout](#go-module-layout).

## Actions

| Action | Purpose |
|--------|---------|
| [build-go](.github/actions/build-go) | One Go binary → artifact (binary cache internal to the action) |
| [build-docker](.github/actions/build-docker) | Build + push to GHCR; optional Docker Hub; skips when GHCR tag exists (`image-built`) |
| [deploy-terraform](.github/actions/deploy-terraform) | Terraform init + apply (`TF_VAR_*` env on the invoking step) |
| [terminate-terraform](.github/actions/terminate-terraform) | Empty destroy module + S3 state delete (`terraform-dir`, `TF_VAR_*` env) |
| [notify](.github/actions/notify) | Slack (inline JSON payload) + PR comment |
| [install-go-tool](.github/actions/install-go-tool) | Install + cache a Go CLI tool (`~/go/bin/<tool>`) |
| [actionlint](.github/actions/actionlint) | Download [rhysd/actionlint](https://github.com/rhysd/actionlint) and lint workflow files |

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
| `DOCKERHUB_*` (optional) | build-docker |
| `GITHUB_TOKEN` or PAT | build-docker (GHCR), notify (PR comments) |

## Typical PR deploy pipeline

```yaml
permissions:
  contents: read
  packages: write   # required for build-docker → GHCR

jobs:
  deploy:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v7
        with:
          fetch-depth: 0

      - uses: dorny/paths-filter@v3
        id: deploy-changes
        with:
          filters: |
            deployable:
              - '**/*.go'
              - 'go.mod'
              - 'go.sum'
              - 'Dockerfile'
              - 'terraform/**'

      - uses: densestvoid/workflows/.github/actions/build-go@main
        id: build-go
        if: steps.deploy-changes.outputs.deployable == 'true'
        with:
          main-package: ./cmd/server
          artifact-name: budget

      - uses: densestvoid/workflows/.github/actions/build-docker@main
        id: docker
        if: steps.deploy-changes.outputs.deployable == 'true'
        with:
          dockerfile: Dockerfile
          artifacts: ${{ steps.build-go.outputs.artifact-name }}
          tag: pr-${{ github.event.pull_request.number }}-${{ github.sha }}
          ghcr-image: ${{ github.repository }}/my-app
          ghcr-username: ${{ github.actor }}
          ghcr-password: ${{ secrets.GITHUB_TOKEN }}

      - uses: densestvoid/workflows/.github/actions/deploy-terraform@main
        id: deploy
        if: steps.deploy-changes.outputs.deployable == 'true'
        env:
          TF_VAR_deployment_id: pr-${{ github.event.pull_request.number }}
          TF_VAR_docker_image_tag: ${{ steps.docker.outputs.image-ref }}
          TF_VAR_do_token: ${{ secrets.DO_TOKEN }}
        with:
          terraform-dir: terraform/pr
          backend-key: pr/pr-${{ github.event.pull_request.number }}.tfstate
          terraform-aws-access-key-id: ${{ secrets.TERRAFORM_AWS_ACCESS_KEY_ID }}
          terraform-aws-secret-access-key: ${{ secrets.TERRAFORM_AWS_SECRET_ACCESS_KEY }}
          terraform-aws-region: ${{ secrets.TERRAFORM_AWS_REGION }}

      # deploy-terraform leaves terraform on PATH in this job; read outputs here, not in a separate job.
      - name: Read service URL
        id: service-url
        if: steps.deploy-changes.outputs.deployable == 'true' && steps.deploy.outcome == 'success'
        working-directory: terraform/pr
        run: |
          set -euo pipefail
          value=$(terraform output -raw service_url)
          echo "value=${value}" >> "$GITHUB_OUTPUT"

      - uses: densestvoid/workflows/.github/actions/notify@main
        if: always() && steps.deploy-changes.outputs.deployable == 'true'
        with:
          slack-webhook: ${{ secrets.SLACK_WEBHOOK }}
          slack-payload: |
            {"text": "Deployed PR #${{ github.event.pull_request.number }}: ${{ steps.service-url.outputs.value }}"}
          pr-number: ${{ github.event.pull_request.number }}
          pr-body: 'Deployed to ${{ steps.service-url.outputs.value }}'
          github-token: ${{ secrets.GITHUB_TOKEN }}
```

### CI skip (Go Checks + Actionlint)

Use in-job path filters — not workflow trigger `paths` — so a required aggregation job (e.g. `ci`) can still run on docs-only PRs.

```yaml
jobs:
  changes:
    runs-on: ubuntu-latest
    outputs:
      go: ${{ steps.filter.outputs.go }}
      workflows: ${{ steps.filter.outputs.workflows }}
    steps:
      - uses: actions/checkout@v7
        with:
          fetch-depth: 0

      - uses: dorny/paths-filter@v4
        id: filter
        with:
          filters: |
            go:
              - '**/*.go'
              - 'go.mod'
              - 'go.sum'
            workflows:
              - '.github/workflows/**'
              - '.github/actions/**'

  go-checks:
    needs: changes
    if: needs.changes.outputs.go == 'true'
    uses: densestvoid/workflows/.github/workflows/go-checks.yml@main

  actionlint:
    needs: changes
    if: needs.changes.outputs.workflows == 'true'
    runs-on: ubuntu-latest
    steps:
      - uses: densestvoid/workflows/.github/actions/actionlint@main

  ci:
    needs: [changes, go-checks, actionlint]
    if: >-
      always() &&
      needs.changes.result == 'success' &&
      (needs.go-checks.result == 'success' || needs.go-checks.result == 'skipped') &&
      (needs.actionlint.result == 'success' || needs.actionlint.result == 'skipped')
    runs-on: ubuntu-latest
    steps:
      - run: echo "CI passed"
```

### PR terminate

```yaml
- uses: densestvoid/workflows/.github/actions/terminate-terraform@main
  env:
    TF_VAR_do_token: ${{ secrets.DO_TOKEN }}
  with:
    terraform-dir: terraform/pr-destroy
    backend-key: pr/pr-${{ github.event.pull_request.number }}.tfstate
    terraform-s3-bucket: densestvoid-terraform
    terraform-aws-access-key-id: ${{ secrets.TERRAFORM_AWS_ACCESS_KEY_ID }}
    terraform-aws-secret-access-key: ${{ secrets.TERRAFORM_AWS_SECRET_ACCESS_KEY }}
    terraform-aws-region: ${{ secrets.TERRAFORM_AWS_REGION }}
```

**terminate-terraform** applies the app repo empty destroy module (`terraform-dir`), then removes the state file from S3 (`|| true`, same as budget). Module variables via `TF_VAR_*` env on the invoking step. Skip logic for empty state belongs in the caller workflow.

## Caching and skip logic

**Skip gates** are caller-owned — use [`dorny/paths-filter`](https://github.com/dorny/paths-filter) in workflow `if:` conditions, not workflow trigger `paths` when a required check must still run.

| Action | Caching |
|--------|---------|
| **build-go** | `actions/cache/restore` + `save` on the binary; key from bundled dep-tree helper + `GOOS`/`GOARCH` |
| **build-docker** | BuildKit GHA cache scoped to `ghcr-image` |
| **install-go-tool** | `actions/cache` on `~/go/bin/<tool>` per package; key includes `go.sum` hash; bare `go install` (consumer `tool` block) |

**build-docker** also skips the build when the caller's `tag` already exists on GHCR (`image-built: false`). Use that to gate container rollout while still applying infra-only Terraform changes.

## Caller notes

### Path filters

Use [`dorny/paths-filter@v3`](https://github.com/dorny/paths-filter) after `actions/checkout` with `fetch-depth: 0`.

```yaml
- uses: dorny/paths-filter@v3
  id: filter
  with:
    base: main   # optional; defaults suit PR/push events
    filters: |
      deployable:
        - '**/*.go'
        - 'terraform/**'
```

Gate downstream steps with `if: steps.filter.outputs.deployable == 'true'`. For `workflow_run` triggers (budget deploy-after-CI), checkout the head SHA first, then run `paths-filter` with `base: main`.

Named filters (`go`, `deployable`, …) let one step drive multiple `if:` conditions.

### Checkout

Every action that needs source code checks out the full repo itself. Callers should not pre-checkout for these actions. Chaining multiple actions in one job means multiple checkouts (wasteful but functional).

| Action | Checkout |
|--------|----------|
| build-go, build-docker, deploy-terraform, terminate-terraform, actionlint | Full repo |
| notify | None (uses github-script; optional checkout in caller) |

### build-docker

Wraps `docker/setup-buildx-action`, `docker/login-action`, `docker/metadata-action`, and `docker/build-push-action`. **GHCR is required**; Docker Hub is an optional second push. Requires `packages: write` for GHCR.

**Inputs**

| Input | Notes |
|-------|-------|
| `ghcr-image` | Path **without** `ghcr.io/` (e.g. `${{ github.repository }}/my-app`) |
| `tag` | Caller-supplied tag (e.g. `pr-42-${{ github.sha }}`) |
| `artifacts` | Comma-separated artifact names to download into the build context before build |

When a prior step uploaded a binary artifact, pass its name:

```yaml
artifacts: ${{ steps.build-go.outputs.artifact-name }}
```

**Outputs**

| Output | Use |
|--------|-----|
| `image-ref` | Full GHCR reference — pass to `TF_VAR_docker_image_tag` or similar |
| `image-built` | `false` when GHCR already had the tag |

Use a PAT when `GITHUB_TOKEN` lacks package scope.

### Go module layout

**build-go** expects `go.mod` at the repo root:

```yaml
- uses: densestvoid/workflows/.github/actions/build-go@main
  with:
    main-package: ./cmd/api
```

**go-checks** supports nested modules via `working-directory`:

```yaml
go-checks:
  uses: densestvoid/workflows/.github/workflows/go-checks.yml@main
  with:
    working-directory: backend
```

The workflow resolves `backend/go.mod` and `backend/go.sum` automatically. See header comments in `go-checks.yml` for the expression syntax.

### Terraform

**deploy-terraform** runs `terraform apply -auto-approve`; Terraform variables come from `TF_VAR_*` env vars set on the invoking workflow step (e.g. `TF_VAR_do_token`, `TF_VAR_deployment_id`). Those env vars propagate into the action automatically — no `with:` inputs for module variables. S3 backend credentials are action inputs passed via `terraform init -backend-config` (`access_key`, `secret_key`, `key`, `region`). App Terraform modules must declare `backend "s3"` with bucket and partial config; the action overrides `key` and `region` at init.

**terminate-terraform** applies an empty destroy module from the app repo (`terraform-dir`), then deletes the S3 state object with `aws s3 rm` (Terraform does not remove defunct state files). Steps: init → apply → delete state. Module variables via `TF_VAR_*` env on the invoking step.

Terraform CLI version is resolved by `hashicorp/setup-terraform` (latest release, not pinned).

Read outputs in the **same job**, immediately after **deploy-terraform** succeeds (see `.cursor/skills/terraform-output-inline/SKILL.md`):

```yaml
- uses: densestvoid/workflows/.github/actions/deploy-terraform@main
  id: deploy
  # ...

- name: Read service URL
  id: url
  if: steps.deploy.outcome == 'success'
  working-directory: terraform/pr
  run: |
    set -euo pipefail
    value=$(terraform output -raw service_url)
    echo "value=${value}" >> "$GITHUB_OUTPUT"
```

**deploy-terraform** runs `hashicorp/setup-terraform`, so `terraform` stays on PATH for later steps in the same job.

### notify (Slack)

**notify** uses `slackapi/slack-github-action` with inline **`slack-payload`** JSON (`text`, `attachments`, `blocks`, …). Callers build the payload in the workflow.

## Known limitations (v0)

| Area | Limitation |
|------|------------|
| **build-go** | `CGO_ENABLED=0`; default `GOOS=linux` / `GOARCH=amd64` — cgo packages won't build |
| **build-docker** | Checks out fresh; downloads `artifacts` when set |
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
├── workflows/
│   ├── go-checks.yml
│   └── ci.yml                   # toolbox self-CI (actionlint + aggregation)
├── actions/
│   ├── actionlint/
│   ├── install-go-tool/
│   ├── build-go/
│   │   └── cachekey/            # bundled Go helper for dep-tree fingerprint
│   ├── build-docker/
│   ├── deploy-terraform/
│   ├── terminate-terraform/
│   └── notify/

.cursor/
└── skills/
    ├── write-github-workflows/   # hub for toolbox authoring
    ├── go-toolchain-setup/
    └── terraform-output-inline/
```
