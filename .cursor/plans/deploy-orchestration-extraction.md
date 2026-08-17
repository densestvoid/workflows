# Deploy Orchestration Extraction — Locked-in decisions

## Core principle: composable steps only, no orchestration

The workflows repo provides **reusable actions** — atomic, composable steps. It does **not** provide orchestration workflows (no full pipelines, no prescribed job graphs).

Each app repo owns:
- `workflow_run` triggers
- Job ordering and `needs:` wiring
- Which steps to call and in what order
- Build composition (language, artifacts, count)
- Deploy strategy (Terraform, CLI/SDK release, Docker push only, etc.)

```
workflows repo = toolbox
app repo     = assembly instructions
```

Example compositions (app-defined):

```
detect-changes (go paths) → Go Checks (skip when unchanged)
detect-changes (deploy paths) → build.yml → Deploy Terraform → Notify
detect-changes (deploy paths) → build.yml → Release CLI → Notify
detect-changes (deploy paths) → Build Docker → Push Container → Notify
```

---

## Secret model — repo-level only (no org secrets for now)

Everything that needs to be a secret or variable lives in the **app repo** (repository secrets/variables, or GitHub Environment where applicable). No migration to organization-level secrets at this time.

| Layer | What |
|-------|------|
| **App repo secrets** | `DO_TOKEN`, `TERRAFORM_AWS_S3_*`, Slack webhooks, registry creds (`DOCKERHUB_*`), app tokens |
| **App repo variables** | `PRODUCTION_DOMAIN`, non-secret config |
| **GitHub Environment** (`termination-delay`) | Terminate job delay only |

**Deploy Terraform** and **Terminate Terraform** accept infra credentials as **action inputs** — caller passes `${{ secrets.* }}` from that app repo’s own secret store. Duplication across repos is accepted for now; the app-deploy-template documents the required secret names once.

Composite actions for all v0 steps.

**Rejected:** Central `workflow_dispatch` runner in workflows repo (black box; needs broad repo access).

---

## Naming — keep it simple

### Actions (composable steps)

| Name | Directory | Purpose |
|------|-----------|---------|
| **Detect Changes** | `detect-changes/` | Path-pattern diff + content hash for **any** caller-supplied paths (CI skip, deploy gating, cache keys) |
| **Build Go** | `build-go/` | Cached Go binary build — **one binary per invocation**; **no Docker awareness** |
| **Build Docker** | `build-docker/` | `docker buildx` — **one image per invocation**; uploads image as **artifact** |
| **Push Container** | `push-container/` | Push image artifact to all configured registries in one step |
| **Deploy Terraform** | `deploy-terraform/` | Terraform init + apply; variables as **action inputs** |
| **Terminate Terraform** | `terminate-terraform/` | Destroy via workflows-repo `pr-destroy` empty module; **delete S3 state file on success** |
| **Notify** | `notify/` | Slack + PR comment delivery; **text passed as input vars** |

### Existing actions (rename for clarity)

| Current | Rename to | Purpose |
|---------|-----------|---------|
| `setup/` | `setup-go/` | Checkout + Go toolchain setup |
| `install-tool/` | `install-go-tool/` | Install + **cache** a Go CLI tool (`actions/cache` on `~/go/bin/…`) |

**Go Checks** (`go-checks.yml`) — update to use renamed actions. App repos call **detect-changes** separately (e.g. Go paths for CI skip).

Future steps (same composable pattern): **Release CLI**, **Deploy Static**, etc.

**Removed:** **Deploy Gate** (superseded by **detect-changes** with deploy paths), **Write Tfvars**, full orchestrator workflows, `workflow_dispatch` central runner, **establish VPC**.

---

## Caching pattern

Established by `install-go-tool/` — cacheable actions use `actions/cache` **internally**. Callers do not receive cache status; they decide whether to invoke an action at all via workflow `if:` conditions.

1. **Key** — action-specific (see below)
2. **Restore** — `actions/cache` with exact key + `restore-keys` prefix fallback
3. **Save** — on miss, run work then let cache action save at step end

| Action | Cache key |
|--------|-----------|
| **build-go** | `content-key` (from detect-changes) + `main-package` |
| **build-docker** | Internal hash of dockerfile + context + input artifacts + `dockerfile` |
| **install-go-tool** | `tool-package` + `tool-version` |

---

## Per-action contracts

### Detect Changes (`detect-changes`)

Generalized change detection for **any** path set — not language-specific. Caller supplies the globs; same action handles CI skip, deploy gating, and source cache keys.

| Input | Purpose |
|-------|---------|
| `paths` | Multiline glob patterns to include in diff + hash |
| `base-ref` | Ref to diff against (default `main`) |

| Output | Purpose |
|--------|---------|
| `changed` | `true` when any matched path changed vs base |
| `content-key` | Content hash of matched paths (for **build-go** cache and caller-driven skip/`tag` decisions) |

**Checkout:** Sparse — only files matching `paths`. Minimum git history for merge-base. Own checkout; caller does not pre-checkout.

Examples:

```yaml
# Go Checks skip
- uses: densestvoid/workflows/.github/actions/detect-changes@v1
  id: go-changes
  with:
    paths: |
      **/*.go
      go.mod
      go.sum

# Deploy job gate
- uses: densestvoid/workflows/.github/actions/detect-changes@v1
  id: deploy-changes
  with:
    paths: |
      terraform/**
      cmd/**
      Dockerfile
```

---

### Build Go

**One binary per invocation.** Multiple binaries → multiple **build-go** steps in the app workflow.

**Must not know about Docker.** Builds and uploads one binary artifact. Caching is internal.

| Input | Purpose |
|-------|---------|
| `working-directory` | Go module root |
| `content-key` | From **detect-changes** (or caller) |
| `main-package` | Package path to build (e.g. `./cmd/server`) |
| `artifact-name` | Optional. Default: **basename of `main-package`** (e.g. `./cmd/server` → `server`) |

| Output | Purpose |
|--------|---------|
| `artifact-name` | Resolved artifact name (default or override) |

Whether to call **build-go** at all (e.g. when sources unchanged) is a **caller `if:`** decision — not an action input.

**Artifact:** Preserves output path layout so Dockerfile `COPY` paths need no remapping.

**Checkout:** Sparse — Go sources + `go.mod`/`go.sum` only.

---

### Build Docker

**One image per invocation.** Multiple Dockerfiles → multiple **build-docker** steps in the app workflow.

Mirrors **build-go**:

1. Compute internal cache key (hash of dockerfile + context + input artifacts)
2. **`actions/cache`** — restore/save image tar (`docker save`) keyed on cache key + `dockerfile`
3. On cache miss — `docker build` → `docker save` → save to cache
4. **Upload artifact** — always, for downstream handoff (cross-job or push-container)

| Input | Purpose |
|-------|---------|
| `context` | Docker build context |
| `dockerfile` | Dockerfile path |
| `artifacts` | Multiline list of artifact names to download (from **build-go** steps) |
| `artifact-name` | Optional. Default: **basename of `dockerfile`** (e.g. `Dockerfile` → `image`, `Dockerfile.worker` → `Dockerfile.worker`) |

| Output | Purpose |
|--------|---------|
| `artifact-name` | Resolved artifact name (default or override) |

Whether to call **build-docker** is a **caller `if:`** decision. Cache restore is internal — only the image artifact is exposed downstream.

**No registry, no push, no Terraform.**

**Checkout:** Sparse — Dockerfile + context files; input artifacts downloaded separately.

---

### Push Container

Multiplexed across registries. Try **all configured** registries; skip unset; **fail if any configured registry fails**.

| Input | Purpose |
|-------|---------|
| `image-artifact` | Artifact name from **build-docker** (downloads, `docker load`) |
| `tag` | Remote registry tag (caller-supplied) |
| `check-only` | Manifest inspect only — no push |

**Per-registry inputs (all optional):**

| Registry | Inputs |
|----------|--------|
| **GHCR** | `ghcr-image`, `ghcr-username`, `ghcr-password` |
| **Docker Hub** | `dockerhub-image`, `dockerhub-username`, `dockerhub-password` |

**Outputs:** Per-registry `*_image_ref`, `*_pushed`, `*_exists`; aggregate `exists`; **`image-ref`** — primary registry image reference for downstream (e.g. Terraform).

Whether to call **push-container** (or call with `check-only` first) is a **caller `if:`** decision.

**Checkout:** None.

**Multi-image:** Each **build-docker** → **push-container** pair has its own artifact and tag.

---

### Deploy Terraform

Variables as **action inputs** (`with:`), not caller-defined `env: TF_VAR_*`. The action maps inputs → Terraform variables internally.

Infra secrets (`do-token`, `terraform-aws-s3-*`) are **action inputs** — caller passes from that repo’s secrets.

| Input | Purpose |
|-------|---------|
| `terraform-dir` | Path to Terraform root module |
| `backend-key` | S3 state key (bucket `densestvoid-terraform` is internal constant) |
| `do-token` | DigitalOcean API token (repo secret, passed by caller) |
| `terraform-aws-access-key-id` | S3 backend credentials |
| `terraform-aws-secret-access-key` | S3 backend credentials |
| `terraform-aws-region` | S3 backend region |
| `variables` | Multi-line `KEY=value` or JSON map of **non-secret** Terraform variables (app module schema) |

| Output | Purpose |
|--------|---------|
| `outputs` | JSON object of **all** Terraform outputs (`terraform output -json`) |
| Individual outputs | Also exposed as `output.<name>` for convenience (dynamic keys from module) |

**Domain / complex types:** CI passes scalars only. Structured values (e.g. `{ hostname, zone }`) are built in Terraform `locals` inside the app module — not in CI.

**Checkout:** Sparse — `terraform-dir` only.

#### Inputs vs `TF_VAR_*` env

| Approach | Pros | Cons |
|----------|------|------|
| **Action inputs** (chosen) | Clean caller `with:` block; action owns TF wiring; no step-level `env:` | Action must map names → TF variables |
| `env: TF_VAR_*` on step | Native Terraform discovery | Verbose; leaks TF naming into every caller |
| JSON map only | Compact | Awkward for single values; harder to read in YAML |

---

### Terminate Terraform

Destroy from existing state using the **workflows-repo** `pr-destroy` empty module — no app `terraform-dir` checkout. On successful destroy, **delete the S3 state file**.

| Input | Purpose |
|-------|---------|
| `backend-key` | S3 state key to destroy + delete |
| `do-token`, `terraform-aws-*` | Same as Deploy Terraform |
| `variables` | Terraform variables if required by destroy |

| Output | Purpose |
|--------|---------|
| `destroyed` | `true` on success |
| `state-deleted` | `true` when S3 state file removed |

**Checkout:** Workflows-repo `terraform/pr-destroy` module only.

---

### Notify

Minimal delivery wrapper. **App repos own all message construction** — workflows repo does not prescribe format, files, or templates.

| Input | Purpose |
|-------|---------|
| `slack-webhook` | Optional Slack incoming webhook |
| `slack-text` | Slack message body (**text var, not file/artifact**) |
| `pr-number` | Optional PR number |
| `pr-body` | PR comment body (**text var, not file/artifact**) |
| `github-token` | Token for PR comment API |

**Behavior:** Send to all configured channels. Skip unset channels. **Fail if any configured channel fails.**

**Checkout:** None.

---

## Build / deploy composition

**Compose by repeating steps** — one **build-go** per binary, one **build-docker** per Dockerfile, one **push-container** per image:

```
build-go (server) ─┐
build-go (worker) ─┼→ build-docker (api) → push-container → Deploy Terraform
                   └→ build-docker (worker) → push-container
```

**Skip logic:** Callers use **detect-changes** `changed` / `content-key` and **push-container** `exists` in workflow `if:` conditions — actions are not invoked when there is nothing to do.

**Jobs vs steps:** Whether build and deploy run in the same job (shared workspace) or separate jobs (artifact upload/download between jobs) is **left to app repos**.

### Typical composition in app `build.yml` (single binary / single image)

```yaml
steps:
  - uses: densestvoid/workflows/.github/actions/detect-changes@v1
    id: deploy-changes
    with:
      paths: |
        **/*.go
        go.mod
        go.sum
        Dockerfile

  - uses: densestvoid/workflows/.github/actions/detect-changes@v1
    id: go-changes
    with:
      paths: |
        **/*.go
        go.mod
        go.sum

  - uses: densestvoid/workflows/.github/actions/build-go@v1
    id: build-go
    if: steps.go-changes.outputs.changed == 'true'
    with:
      content-key: ${{ steps.go-changes.outputs.content-key }}
      main-package: ./cmd/server

  - uses: densestvoid/workflows/.github/actions/build-docker@v1
    id: docker
    if: steps.deploy-changes.outputs.changed == 'true'
    with:
      dockerfile: Dockerfile
      artifacts: ${{ steps.build-go.outputs.artifact-name }}

  - uses: densestvoid/workflows/.github/actions/push-container@v1
    id: check
    if: steps.deploy-changes.outputs.changed == 'true'
    with:
      image-artifact: ${{ steps.docker.outputs.artifact-name }}
      tag: pr-123-${{ steps.deploy-changes.outputs.content-key }}
      check-only: true
      ghcr-image: ${{ github.repository }}/my-app
      ghcr-username: ${{ github.actor }}
      ghcr-password: ${{ secrets.GITHUB_TOKEN }}

  - uses: densestvoid/workflows/.github/actions/push-container@v1
    id: push
    if: steps.check.outputs.exists != 'true'
    with:
      image-artifact: ${{ steps.docker.outputs.artifact-name }}
      tag: pr-123-${{ steps.deploy-changes.outputs.content-key }}
      ghcr-image: ${{ github.repository }}/my-app
      ghcr-username: ${{ github.actor }}
      ghcr-password: ${{ secrets.GITHUB_TOKEN }}
      dockerhub-image: myuser/my-app
      dockerhub-username: ${{ secrets.DOCKERHUB_USERNAME }}
      dockerhub-password: ${{ secrets.DOCKERHUB_TOKEN }}
```

Multiple binaries or images: add another **build-go** / **build-docker** / **push-container** block each.

---

## Checkout convention

**Every action checks out exactly what it needs — no more.** Sparse paths, minimum history, no full-repo clones. Callers should not pre-checkout for actions.

| Action | Checkout scope |
|--------|----------------|
| detect-changes | Sparse: only `paths` globs; minimum history for merge-base |
| Build Go | Go sources + module files for build |
| Build Docker | Dockerfile + context files; input artifacts downloaded |
| Push Container | None (downloads image artifact only) |
| Deploy Terraform | `terraform-dir` only |
| Terminate Terraform | Workflows-repo `pr-destroy` module only |
| Notify | None |

Existing **setup-go** does full checkout — acceptable for Go Checks; new actions follow the thin convention above.

---

## Skip / cache behavior

| Layer | Mechanism |
|-------|-----------|
| **CI go-checks** | Caller `if:` on **detect-changes** (go paths) → `changed` |
| **Deploy job gate** | Caller `if:` on **detect-changes** (deploy paths) → `changed` |
| **Go binary** | **build-go** internal `actions/cache`; caller `if:` decides invocation |
| **Docker image** | **build-docker** internal `actions/cache`; caller `if:` on **push-container** `exists` |

---

## Versioning

**Action directory names have no version** — always `build-go`, `deploy-terraform`, etc. Version lives only in the **git ref** (`@…`), which is standard GitHub Actions practice.

### Default: coupled repo tags

Pin every action at the same repo ref. One tag = one tested snapshot of the whole toolbox.

```yaml
uses: densestvoid/workflows/.github/actions/build-go@v1
uses: densestvoid/workflows/.github/actions/deploy-terraform@v1
```

Both `@v1` resolve to the **same commit**. Bump all pins together when releasing.

| Ref | When |
|-----|------|
| `@main` | v0 iteration / bleeding edge |
| `@v1`, `@v2` | Stable releases (major = breaking change to any action) |
| `@<sha>` | Pin to exact commit for debugging |

### Escape hatch: independent per-action versions

```yaml
uses: densestvoid/workflows/.github/actions/deploy-terraform@deploy-terraform/v1.1.0
```

**Go Checks:** `uses: densestvoid/workflows/.github/workflows/go-checks.yml@v1` (same coupled ref).

---

## Rollout plan

| Phase | Scope | Repos touched |
|-------|-------|---------------|
| **v0** | Build composable actions in workflows repo | `workflows` |
| **v0 test** | Validate composition with **krogerrecipeshopper** | `workflows`, `krogerrecipeshopper` |
| **v1** | Cut `@v1` repo tag; create `densestvoid/app-deploy-template` | `workflows`, `app-deploy-template` |
| **v1 migrate** | Update **budget** to composed steps | `budget` |

**Budget is last.** Integration test via kroger — not unit tests in workflows repo.

---

## GitHub template repository (v1)

Separate repo: `densestvoid/app-deploy-template` — template repository with example compositions pinned at `@v1`.

---

## Workflows repo structure (v0)

```
densestvoid/workflows/
├── .github/
│   ├── workflows/
│   │   └── go-checks.yml
│   ├── actions/
│   │   ├── setup-go/            # was setup/
│   │   ├── install-go-tool/     # was install-tool/
│   │   ├── detect-changes/
│   │   ├── build-go/
│   │   ├── build-docker/
│   │   ├── push-container/
│   │   ├── deploy-terraform/
│   │   ├── terminate-terraform/
│   │   └── notify/
│   └── terraform/
│       └── pr-destroy/
└── README.md
```

---

## Example composition (Go + Terraform)

```yaml
jobs:
  gate:
    runs-on: ubuntu-latest
    outputs:
      changed: ${{ steps.deploy-changes.outputs.changed }}
    steps:
      - uses: densestvoid/workflows/.github/actions/detect-changes@v1
        id: deploy-changes
        with:
          paths: |
            terraform/**
            cmd/**
            Dockerfile

  build:
    needs: gate
    if: needs.gate.outputs.changed == 'true'
    runs-on: ubuntu-latest
    outputs:
      image-ref: ${{ steps.push.outputs.image-ref }}
    steps:
      # ... build-go → build-docker → push-container (see build.yml above)

  deploy:
    needs: build
    runs-on: ubuntu-latest
    steps:
      - uses: densestvoid/workflows/.github/actions/deploy-terraform@v1
        with:
          terraform-dir: terraform/pr
          backend-key: pr/pr-${{ github.event.pull_request.number }}.tfstate
          do-token: ${{ secrets.DO_TOKEN }}
          terraform-aws-access-key-id: ${{ secrets.TERRAFORM_AWS_ACCESS_KEY_ID }}
          terraform-aws-secret-access-key: ${{ secrets.TERRAFORM_AWS_SECRET_ACCESS_KEY }}
          terraform-aws-region: ${{ secrets.TERRAFORM_AWS_REGION }}
          variables: |
            deployment_id=pr-${{ github.event.pull_request.number }}
            docker_image_tag=${{ needs.build.outputs.image-ref }}
            domain_name=${{ vars.PRODUCTION_DOMAIN }}

  notify:
    needs: [gate, build, deploy]
    if: always()
    runs-on: ubuntu-latest
    steps:
      - uses: densestvoid/workflows/.github/actions/notify@v1
        with:
          slack-webhook: ${{ secrets.SLACK_WEBHOOK }}
          slack-text: ${{ needs.build.outputs.notify-slack-text }}
          pr-number: ${{ github.event.pull_request.number }}
          pr-body: ${{ needs.deploy.outputs.notify-pr-body }}
          github-token: ${{ secrets.GITHUB_TOKEN }}
```

---

## Rejected

| Approach | Why |
|----------|-----|
| **Deploy Gate** | Superseded by **detect-changes** with deploy paths |
| **`skip` / `cache-hit` on build actions** | Caller `if:` decides invocation; cache is internal |
| **`local-tag` handoff** | **build-docker** uploads image artifact; **push-container** loads it |
| **`content-key` output on build-docker** | Only `artifact-name` exposed; cache key is internal |
| **Write Tfvars** / JSON-only variable maps | Action inputs + optional `variables` block; complex types in Terraform HCL |
| **Static tfvars defaults in v0** | Deferred |
| **`terraform-dir` on terminate** | Empty module lives in workflows repo; destroy from state only |
| **binaries list on build-go** | One **build-go** step per binary; app composes the graph |
| **Multiple images in one build-docker** | One **build-docker** step per Dockerfile |
| **Workflows repo secrets for cross-repo deploy** | Caller `secrets` context only; secrets live in each app repo |
| **Organization secrets** | Not using org-level secrets at this time |
| Build steps know about Terraform | **Build Docker** / **Push Container** are registry/build only |
| Full orchestrator workflows in workflows repo | Too thick; apps compose steps |
| `workflow_dispatch` central runner | Black box |
| Budget-first migration | Krogerrecipeshopper validates v0 first |
| **Establish VPC in workflows repo** | App Terraform concern |
| Version in action directory or tag name (`@build-go-v1`) | Version in git ref only (`@v1` or `deploy-terraform/v1.1.0`) |
