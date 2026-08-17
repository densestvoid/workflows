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
detect-go-changes → Go Checks (skip when unchanged)
Deploy Gate → build.yml → Deploy Terraform → prepare-notify → Notify
Deploy Gate → build.yml → Release CLI → prepare-notify → Notify
Deploy Gate → Build Docker → Push Container → Notify   # image to registry only
```

---

## Actions vs workflows — and why secrets matter

These are different GitHub primitives with different secret behavior. This is the main reason “put secrets in workflows repo” does not automatically solve cross-repo sharing.

| | **Composite action** (`uses: repo/.github/actions/foo`) | **Reusable workflow** (`uses: repo/.github/workflows/foo.yml`) |
|---|---|---|
| **What it is** | A bundle of steps that run **inside the caller’s job** on the same runner | A separate workflow (one or more jobs) invoked by the caller |
| **Workspace** | Shares the caller job workspace unless the action does its own checkout | Fresh workspace per job unless explicitly passed via artifacts |
| **Speed** | Lighter — no extra job boundary | Heavier — job startup, artifact handoff between jobs |
| **Orchestration** | Single job only | Can define `needs:`, multiple jobs, environments, concurrency |
| **`secrets` context** | Composite actions **cannot** read `secrets.*` directly; values must be passed as **inputs** | Reusable workflow jobs can use `secrets.*`, but only secrets from the **caller** (plus org/environment) |

### Cross-repo secret resolution (critical)

When `budget` or `krogerrecipeshopper` calls something in `densestvoid/workflows`:

- The `secrets` context is always the **caller’s** secrets (explicitly mapped or `secrets: inherit`), plus **organization** and **environment** secrets visible to the caller.
- **Repository secrets stored only in `workflows` are not available** to a composite action or reusable workflow invoked from another repo.

So “configure `DO_TOKEN` once in workflows repo” does **not** work for app repos calling workflows actions/workflows cross-repo. That was the implicit assumption behind “workflows-repo secrets + composite actions” — it only applies to workflows that **run inside the workflows repo itself**.

### Secret model — repo-level only (no org secrets for now)

Everything that needs to be a secret or variable lives in the **app repo** (repository secrets/variables, or GitHub Environment where applicable). No migration to organization-level secrets at this time.

| Layer | What |
|-------|------|
| **App repo secrets** | `DO_TOKEN`, `TERRAFORM_AWS_S3_*`, Slack webhooks, registry creds (`DOCKERHUB_*`), app tokens |
| **App repo variables** | `PRODUCTION_DOMAIN`, non-secret config |
| **GitHub Environment** (`termination-delay`) | Terminate job delay only |

**Deploy Terraform** and **Terminate Terraform** accept infra credentials as **action inputs** — caller passes `${{ secrets.* }}` from that app repo’s own secret store. Duplication across repos is accepted for now; the app-deploy-template documents the required secret names once.

Workflows-repo repository secrets are **not** visible to cross-repo callers anyway (see above). Composite actions for all v0 steps.

**Rejected:** Central `workflow_dispatch` runner in workflows repo (black box; needs broad repo access).

---

## Naming — keep it simple

### Actions (composable steps)

| Name | Directory | Purpose |
|------|-----------|---------|
| **Detect Go Changes** | `detect-go-changes/` | Composable pre-CI check: Go source diff + content hash; skip signal for Go Checks and build |
| **Build Go** | `build-go/` | Cached Go binary build; content-hash skip logic; **no Docker awareness** |
| **Build Docker** | `build-docker/` | Local `docker buildx` only; receives binary via **artifact** |
| **Push Container** | `push-container/` | Push to all configured registries in one step (Notify-style multiplex) |
| **Deploy Gate** | `deploy-gate/` | Deployable path diff vs `main`; **checks out what it needs** |
| **Deploy Terraform** | `deploy-terraform/` | Terraform init + apply; variables as **action inputs** |
| **Terminate Terraform** | `terminate-terraform/` | Destroy via `pr-destroy` empty-module pattern; **delete S3 state file on success** |
| **Notify** | `notify/` | Slack + PR comment delivery; **text passed as input vars** |

Future steps (same composable pattern): **Release CLI**, **Deploy Static**, etc.

**Removed:** **Write Tfvars**, full orchestrator workflows, `workflow_dispatch` central runner, **establish VPC** (VPC is app Terraform logic — erroneously included in an earlier rollout draft; not workflows-repo scope).

**Existing:** **Go Checks** reusable workflow (`go-checks.yml`) — stays as-is; pairs with **detect-go-changes** in app repos.

---

## Per-action contracts

### Detect Go Changes (`detect-go-changes`)

Composable pre-CI step. Expect more checks in this family (lint skip, docs skip, etc.) — keep each as a separate composable action.

| Input | Purpose |
|-------|---------|
| `working-directory` | Go module root (default `.`) |
| `base-ref` | Ref to diff against (default `main`) |

| Output | Purpose |
|--------|---------|
| `changed` | `true` when Go sources changed vs base |
| `build-key` | Content hash for cache/skip decisions (**owned here**, not Build Go) |

**Checkout:** Sparse — `go.mod`, `go.sum`, and `**/*.go` under working directory only. No other paths. Own checkout; caller does not pre-checkout.

---

### Build Go

**Must not know about `docker_build_key` or Docker.** Builds and uploads a binary artifact only.

| Input | Purpose |
|-------|---------|
| `working-directory` | Go module root |
| `build-key` | Cache key from **detect-go-changes** (or caller) |
| `binary-name` | Output binary name |
| `main-package` | Package path to build |
| `skip` | Skip build when cache hit |

| Output | Purpose |
|--------|---------|
| `cache-hit` | Whether build was skipped |
| `artifact-name` | Name of uploaded artifact (for downstream download) |

**Artifact:** Uploads built binary (e.g. `app-binary`). **Build Docker** downloads this artifact — no shared filesystem path input.

**Checkout:** Thin — Go sources + `go.mod`/`go.sum` only.

---

### Build Docker

| Input | Purpose |
|-------|---------|
| `context` | Docker build context |
| `dockerfile` | Dockerfile path |
| `local-tag` | Local image tag (e.g. `my-app:local`) |
| `binary-artifact` | Artifact name from **Build Go** (downloads internally) |
| `binary-path` | Path inside artifact / build context where binary lands |
| `skip` | Skip when image already exists remotely |

| Output | Purpose |
|--------|---------|
| `local-tag` | Tag loaded in local daemon |
| `docker-build-key` | Content hash for image tag / skip decisions |

**No registry, no push, no Terraform.**

**Checkout:** Thin — Dockerfile, context files, and downloaded binary artifact only.

---

### Push Container

Single action, **multiplexed across registries** — same pattern as **Notify**.

**Behavior:** Try **all configured** registries. Skip unset registries. **Fail the step if any configured registry fails** (partial success is not success).

| Notify | Push Container |
|--------|----------------|
| `slack_webhook` + text → Slack | `ghcr_*` + image → GHCR |
| `pr_number` + `pr_body` → PR comment | `dockerhub_*` + image → Docker Hub |
| Skips unset channels | Skips unset registries |
| Fails if a configured channel fails | Fails if a configured registry fails |

**Shared inputs:**

| Input | Purpose |
|-------|---------|
| `local-tag` | Local image tag (from **Build Docker**) |
| `tag` | Remote tag (e.g. content-hash tag from `docker-build-key`) |
| `check-only` | Manifest inspect only — no push |

**Per-registry inputs (all optional):**

| Registry | Inputs |
|----------|--------|
| **GHCR** | `ghcr-image`, `ghcr-username`, `ghcr-password` |
| **Docker Hub** | `dockerhub-image`, `dockerhub-username`, `dockerhub-password` |

**Outputs:** Per-registry `*_image_ref`, `*_pushed`, `*_exists`; aggregate `exists` — `true` when **all configured** registries already have the manifest.

**Checkout:** None (operates on local Docker image only).

---

### Deploy Gate

Encapsulates its own checkout so callers don’t need git history for gating.

| Input | Purpose |
|-------|---------|
| `base-ref` | Branch to diff against (default `main`) |
| `paths` | Glob list of deployable paths (app-defined) |

| Output | Purpose |
|--------|---------|
| `should-deploy` | `true` when deployable paths changed vs base |

**Checkout:** Sparse — only the `paths` globs. Fetch the minimum git history needed for merge-base against `base-ref` (may require deeper fetch than `fetch-depth: 1`, but never checks out unrelated paths or files). Principle: **exactly what this action needs, no more** — not a full-repo clone.

---

### Deploy Terraform

Variables as **action inputs** (`with:`), not caller-defined `env: TF_VAR_*`. The action maps inputs → Terraform variables internally. Avoids env-var boilerplate on every caller step.

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

**Static defaults:** `*.auto.tfvars` / `variables.tf` defaults in app repo; CI only passes dynamic values (image tag, deployment id).

**Checkout:** Thin — `terraform-dir` only (+ workflows-repo `pr-destroy` module reference for terminate, not deploy).

#### Inputs vs `TF_VAR_*` env

| Approach | Pros | Cons |
|----------|------|------|
| **Action inputs** (chosen) | Clean caller `with:` block; action owns TF wiring; no step-level `env:` | Action must map names → TF variables |
| `env: TF_VAR_*` on step | Native Terraform discovery | Verbose; leaks TF naming into every caller |
| JSON map only | Compact | Awkward for single values; harder to read in YAML |

---

### Terminate Terraform

Same `pr-destroy` empty-module pattern as budget today. Empty destroy module lives in **workflows repo** (generic). On successful destroy, **delete the S3 state file**.

| Input | Purpose |
|-------|---------|
| `terraform-dir` | App Terraform root (for variable context if needed) |
| `backend-key` | S3 state key to destroy + delete |
| `do-token`, `terraform-aws-*` | Same as Deploy Terraform |
| `variables` | Terraform variables needed for destroy context |

| Output | Purpose |
|--------|---------|
| `destroyed` | `true` on success |
| `state-deleted` | `true` when S3 state file removed |

**Checkout:** Thin — app `terraform-dir` + workflows-repo destroy module.

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

## Build / deploy job separation

Separate jobs in app repos:

```
Build Go → Build Docker → Push Container → Deploy Terraform
```

Caller uses **detect-go-changes** / **Push Container** `exists` for skip logic. When `exists == true`, skip Build Go, Build Docker, and push — go straight to deploy.

### Typical composition in app `build.yml`

```yaml
steps:
  - uses: densestvoid/workflows/.github/actions/detect-go-changes@v1
    id: changes

  - uses: densestvoid/workflows/.github/actions/push-container@v1
    id: check
    with:
      local-tag: my-app:local
      tag: pr-123-${{ steps.changes.outputs.build-key }}
      check-only: true
      ghcr-image: ${{ github.repository }}/my-app
      ghcr-username: ${{ github.actor }}
      ghcr-password: ${{ secrets.GITHUB_TOKEN }}

  - uses: densestvoid/workflows/.github/actions/build-go@v1
    if: steps.check.outputs.exists != 'true'
    with:
      build-key: ${{ steps.changes.outputs.build-key }}

  - uses: densestvoid/workflows/.github/actions/build-docker@v1
    if: steps.check.outputs.exists != 'true'
    with:
      binary-artifact: ${{ steps.build-go.outputs.artifact-name }}
      local-tag: my-app:local

  - uses: densestvoid/workflows/.github/actions/push-container@v1
    if: steps.check.outputs.exists != 'true'
    with:
      local-tag: my-app:local
      tag: pr-123-${{ steps.changes.outputs.build-key }}
      ghcr-image: ${{ github.repository }}/my-app
      ghcr-username: ${{ github.actor }}
      ghcr-password: ${{ secrets.GITHUB_TOKEN }}
      dockerhub-image: myuser/my-app
      dockerhub-username: ${{ secrets.DOCKERHUB_USERNAME }}
      dockerhub-password: ${{ secrets.DOCKERHUB_TOKEN }}
```

---

## Checkout convention

**Every action checks out exactly what it needs — no more.** Sparse paths, minimum history, no full-repo clones. Callers should not pre-checkout for actions.

| Action | Checkout scope |
|--------|----------------|
| detect-go-changes | `go.mod`, `go.sum`, `**/*.go` under working directory |
| Build Go | Go sources + module files for build |
| Build Docker | Dockerfile + context files + binary artifact |
| Push Container | None |
| Deploy Gate | Sparse: deployable `paths` only; minimum history for merge-base |
| Deploy Terraform | `terraform-dir` only |
| Terminate Terraform | `terraform-dir` + destroy module ref |
| Notify | None |

Existing **setup** action does full checkout today — acceptable for Go Checks; new actions follow the thin convention above.

---

## Skip / cache behavior

| Layer | v0 step |
|-------|---------|
| **CI go-checks** | **detect-go-changes** → `changed` |
| **PR deploy gate** | **Deploy Gate** → `should-deploy` |
| **Go binary** | **Build Go** content-hash cache |
| **Docker image** | **Push Container** `exists`; caller skips build when true |

---

## Versioning

**Action directory names have no version** — always `build-go`, `deploy-terraform`, etc. Version lives only in the **git ref** (`@…`), which is standard GitHub Actions practice.

### Default: coupled repo tags

Pin every action at the same repo ref. One tag = one tested snapshot of the whole toolbox.

```yaml
uses: densestvoid/workflows/.github/actions/build-go@v1
uses: densestvoid/workflows/.github/actions/deploy-terraform@v1
```

Both `@v1` resolve to the **same commit**. Bump all pins together when releasing. Simple, conventional, no redundant naming.

| Ref | When |
|-----|------|
| `@main` | v0 iteration / bleeding edge |
| `@v1`, `@v2` | Stable releases (major = breaking change to any action) |
| `@<sha>` | Pin to exact commit for debugging |

### Escape hatch: independent per-action versions

If one action needs to ship a fix without cutting a full toolbox release, use **namespaced git tags** — version in the ref, not the directory:

```yaml
uses: densestvoid/workflows/.github/actions/deploy-terraform@deploy-terraform/v1.1.0
```

Tag `deploy-terraform/v1.1.0` points at a commit; only callers of that action need to update. Avoid `@build-go-v1` style tags — redundant with the path and awkward to read.

**Rejected:** Version baked into action directory names (`build-go-v1/`); version repeated in tag and path (`@build-go-v1`).

**Go Checks** reusable workflow: `uses: densestvoid/workflows/.github/workflows/go-checks.yml@v1` (same coupled ref).

---

## Rollout plan

| Phase | Scope | Repos touched |
|-------|-------|---------------|
| **v0** | Build composable actions in workflows repo | `workflows` |
| **v0 test** | Validate composition with **krogerrecipeshopper** (Go + Terraform; deployment details TBD — likely very similar to budget post-migration) | `workflows`, `krogerrecipeshopper` |
| **v1** | Cut `@v1` repo tag; create `densestvoid/app-deploy-template` GitHub template repo | `workflows`, `app-deploy-template` |
| **v1 migrate** | Update **budget** to composed steps | `budget` |

**Budget is last** — not touched during v0 iteration. **Krogerrecipeshopper** is the proving ground.

**Testing strategy:** Integration test via kroger repo — not unit tests in workflows repo.

**Removed from v0:** “establish shared VPC / infra foundations” — VPC and network logic belong in each app’s Terraform (e.g. budget deployment module), not the workflows toolbox.

---

## GitHub template repository (v1)

Separate repo: `densestvoid/app-deploy-template`

- Enable **Settings → Template repository**
- Example compositions pinned at `@v1`
- `gh repo create my-app --template densestvoid/app-deploy-template`

Workflows repo stays actions-only — no template files inside it.

---

## Workflows repo structure (v0)

```
densestvoid/workflows/
├── .github/
│   ├── workflows/
│   │   └── go-checks.yml
│   ├── actions/
│   │   ├── setup/
│   │   ├── install-tool/
│   │   ├── detect-go-changes/
│   │   ├── build-go/
│   │   ├── build-docker/
│   │   ├── push-container/
│   │   ├── deploy-gate/
│   │   ├── deploy-terraform/
│   │   ├── terminate-terraform/
│   │   └── notify/
│   └── terraform/
│       └── pr-destroy/          # empty module for Terminate Terraform
└── README.md
```

---

## Example composition (Go + Terraform)

```yaml
jobs:
  gate:
    steps:
      - uses: densestvoid/workflows/.github/actions/deploy-gate@v1
        with:
          paths: |
            terraform/**
            cmd/**

  build:
    needs: gate
    if: needs.gate.outputs.should-deploy == 'true'
  # ... build.yml: detect-go-changes → push-container check → build-go → build-docker → push-container

  deploy:
    needs: build
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
            docker_image_tag=${{ needs.build.outputs.image_tag }}
            domain_name=${{ vars.PRODUCTION_DOMAIN }}

  notify:
    needs: [gate, build, deploy]
    if: always()
    steps:
      - uses: densestvoid/workflows/.github/actions/notify@v1
        with:
          slack-webhook: ${{ secrets.SLACK_WEBHOOK }}
          slack-text: ${{ needs.build.outputs.notify-slack-text }}
          pr-number: ${{ github.event.pull_request.number }}
          pr-body: ${{ needs.deploy.outputs.notify-pr-body }}
          github-token: ${{ secrets.GITHUB_TOKEN }}
```

App jobs construct `notify-slack-text` / `notify-pr-body` however they want — Notify only delivers.

---

## Progress

### Budget — import removal

PR #21 (independent). Budget migration deferred until v1 after krogerrecipeshopper validation.

### Internal constants

- S3 state bucket: `densestvoid-terraform`
- Terraform version: `1.5.7`

---

## Rejected

| Approach | Why |
|----------|-----|
| **Write Tfvars** / JSON-only variable maps | Action inputs + optional `variables` block; complex types in Terraform HCL |
| **Build Go knows docker_build_key** | Hash owned by **detect-go-changes**; binary via artifacts |
| **Workflows repo secrets for cross-repo deploy** | Caller `secrets` context only; secrets live in each app repo |
| **Organization secrets** | Not using org-level secrets at this time |
| Build steps know about Terraform | **Build Docker** / **Push Container** are registry/build only |
| Full orchestrator workflows in workflows repo | Too thick; apps compose steps |
| `workflow_dispatch` central runner | Black box |
| Budget-first migration | Krogerrecipeshopper validates v0 first |
| **Establish VPC in workflows repo** | App Terraform concern; erroneously pulled into rollout draft |
| Version in action directory or tag name (`@build-go-v1`) | Version in git ref only (`@v1` or `deploy-terraform/v1.1.0`) |
