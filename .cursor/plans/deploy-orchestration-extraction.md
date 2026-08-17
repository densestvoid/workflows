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
Deploy Gate → build.yml → Deploy Terraform → prepare-notify → Notify
Deploy Gate → build.yml → Release CLI → prepare-notify → Notify
Deploy Gate → Build Docker → Push Container → Notify   # image to registry only
```

---

## Naming — keep it simple

### Actions (composable steps)

| Name | Directory | Purpose |
|------|-----------|---------|
| **Build Go** | `build-go/` | Cached Go binary build; content-hash skip logic embedded |
| **Build Docker** | `build-docker/` | Build container image locally (`docker buildx`); outputs local tag — **no registry, no push** |
| **Push Container** | `push-container/` | Push image to any registry; caller passes registry + credentials; confirm-if-exists skip |
| **Deploy Gate** | `deploy-gate/` | Deployable path diff vs `main` |
| **Deploy Terraform** | `deploy-terraform/` | Terraform init + apply; variables passed as inputs |
| **Terminate Terraform** | `terminate-terraform/` | Terraform destroy from state (`pr-destroy` pattern) |
| **Notify** | `notify/` | Slack + PR comment delivery |

Future steps (same composable pattern): **Release CLI**, **Deploy Static**, etc.

**Removed:** **Write Tfvars** — variables pass directly into **Deploy Terraform** (see below).

**No orchestrator workflows** in workflows repo beyond existing **Go Checks** (`go-checks.yml`).

---

## Build Docker + Push Container — separate steps

**Yes — split build and push.** Building an image and publishing it to a registry are different operations with different inputs, auth, and skip logic.

### Build Docker

- Runs `docker buildx build` (load to local daemon or `--output type=docker`)
- Inputs: `context`, `dockerfile`, `tags` (local tag), optional binary artifact from **Build Go**
- Outputs: `image_tag`, `docker_build_key` (content hash for cache/skip decisions)
- **No registry, no push, no Terraform**

### Push Container

Single action, **multiplexed across registries** — same pattern as **Notify**: one invocation can push to **all configured registries**; skips any registry not configured.

**Notify analogy:**

| Notify | Push Container |
|--------|----------------|
| `slack_webhook` + payload → Slack | `ghcr_*` credentials + image → GHCR |
| `pr_number` + `pr_body` → PR comment | `dockerhub_*` credentials + image → Docker Hub |
| Both in one step | All configured registries in one step |
| Skips unset channels | Skips unset registries |

**Shared inputs:**

| Input | Purpose |
|-------|---------|
| `local_tag` | Local image tag to push (from **Build Docker**) |
| `tag` | Remote tag (e.g. content-hash tag) |
| `check_only` | If true, manifest inspect only — no push |

**Per-registry inputs (all optional — omit to skip that registry):**

| Registry | Inputs |
|----------|--------|
| **GHCR** | `ghcr_image`, `ghcr_username`, `ghcr_password` |
| **Docker Hub** | `dockerhub_image`, `dockerhub_username`, `dockerhub_password` |

Caller passes credentials for each registry it wants. Action pushes to every configured registry in one step.

**Outputs (per registry, e.g.):** `ghcr_image_ref`, `ghcr_pushed`, `ghcr_exists`, `dockerhub_image_ref`, …  
**Aggregate:** `exists` — true when **all configured** registries already have the manifest (for skip-build decisions).

**Behavior per configured registry:** login → manifest inspect → tag → push (or inspect-only when `check_only`).

**No Terraform, no build.** Registries requiring non-`docker login` auth (ECR, GAR) are future additions as new optional input groups in the same multiplexing pattern.

### Typical composition in app `build.yml`

```yaml
steps:
  - uses: densestvoid/workflows/.github/actions/push-container@v0
    id: check
    with:
      local_tag: my-app:local
      tag: pr-123-${{ steps.hash.outputs.docker_build_key }}
      check_only: true
      ghcr_image: ${{ github.repository }}/my-app
      ghcr_username: ${{ github.actor }}
      ghcr_password: ${{ secrets.GITHUB_TOKEN }}
      # dockerhub_image: myuser/my-app
      # dockerhub_username: ${{ secrets.DOCKERHUB_USERNAME }}
      # dockerhub_password: ${{ secrets.DOCKERHUB_TOKEN }}

  - uses: densestvoid/workflows/.github/actions/build-go@v0
    if: steps.check.outputs.exists != 'true'

  - uses: densestvoid/workflows/.github/actions/build-docker@v0
    if: steps.check.outputs.exists != 'true'

  - uses: densestvoid/workflows/.github/actions/push-container@v0
    if: steps.check.outputs.exists != 'true'
    with:
      local_tag: my-app:local
      tag: pr-123-${{ steps.hash.outputs.docker_build_key }}
      ghcr_image: ${{ github.repository }}/my-app
      ghcr_username: ${{ github.actor }}
      ghcr_password: ${{ secrets.GITHUB_TOKEN }}
      dockerhub_image: myuser/my-app
      dockerhub_username: ${{ secrets.DOCKERHUB_USERNAME }}
      dockerhub_password: ${{ secrets.DOCKERHUB_TOKEN }}
```

One push step → GHCR and Docker Hub together when both are configured.

---

## Deploy Terraform — variables via `TF_VAR_*` env, not tfvars files or JSON

**Decision:** Drop **Write Tfvars**. Callers pass Terraform variables as **separate `env: TF_VAR_<name>` entries** on the step — native YAML, one variable per line, no JSON map construction.

**Deploy Terraform** inputs are only what the action itself needs:

| Input | Purpose |
|-------|---------|
| `terraform_dir` | Path to Terraform root module |
| `backend_key` | S3 state key |

Infra secrets (`DO_TOKEN`, etc.) are injected inside the action from workflows repo repository secrets as `TF_VAR_*` — not passed by the caller.

All **non-secret** Terraform variables are set by the caller on the step `env:` block. Terraform reads them automatically.

### Caller wiring

```yaml
  deploy:
    needs: build
    steps:
      - uses: densestvoid/workflows/.github/actions/deploy-terraform@v0
        with:
          terraform_dir: terraform/pr
          backend_key: pr/pr-123.tfstate
        env:
          TF_VAR_deployment_id: pr-123
          TF_VAR_docker_image_tag: ${{ needs.build.outputs.image_tag }}
          TF_VAR_domain_name: ${{ vars.PRODUCTION_DOMAIN }}
```

Each app passes only the `TF_VAR_*` keys its Terraform module defines. No shared schema in the action.

### Domain / complex types — stay in Terraform, not CI

CI should pass **scalars only** (`domain_name`, `deployment_id`, `docker_image_tag`). If a module needs a structured value (e.g. `{ hostname, zone }`), the **Terraform module** constructs it in `locals` from scalar inputs — not passed from GitHub Actions.

Budget's `domain` object belongs inside `terraform/` HCL, not in the deploy step env.

### Static defaults

Committed defaults live in the app repo as `*.auto.tfvars` or `variables.tf` defaults — Terraform picks them up without CI wiring. Dynamic values (image tag from build) are the only ones set via `TF_VAR_*` at deploy time.

### Tradeoffs vs JSON map

| Approach | Pros | Cons |
|----------|------|------|
| **`env: TF_VAR_*` per var** (chosen) | Explicit, no JSON, native GHA YAML, one arg per variable | Verbose for many vars; typos in var names fail at terraform plan |
| JSON map input | Compact | Awkward construction; user preference against it |
| tfvars file construction | Familiar to terraform users | Extra step; build/deploy coupling |

---

## Skip / cache behavior — must be preserved

| Layer | Budget today | v0 step |
|-------|--------------|---------|
| **PR deploy gate** | Skip when no deployable changes | **Deploy Gate** |
| **Go binary** | Content-hash cache | **Build Go** |
| **Docker image** | Content-hash tag; skip push when manifest exists in registry | **Push Container** (confirm-if-exists); caller skips **Build Docker** when `exists` |
| **CI go-checks** | Skip when Go unchanged | **Build Go** → `go_sources_changed` |

---

## Secrets

| Where | What |
|-------|------|
| **Workflows repo secrets** | `DO_TOKEN`, `TERRAFORM_AWS_S3_*` — used by **Deploy Terraform**, **Terminate Terraform** |
| **App repo secrets/variables** | Slack webhooks, `PRODUCTION_DOMAIN`, app-specific tokens |
| **App repo environment** | `termination-delay` only |

---

## Rollout plan

| Phase | Scope | Repos touched |
|-------|-------|---------------|
| **v0** | Composable actions in workflows repo; establish shared VPC / infra foundations as needed | `workflows` |
| **v0 test** | Validate composition with **krogerrecipeshopper** — iterate on actions until happy | `workflows`, `krogerrecipeshopper` |
| **v1** | Tag `@v1`; create `densestvoid/app-deploy-template` GitHub template repo | `workflows`, `app-deploy-template` |
| **v1 migrate** | Update **budget** to composed steps | `budget` |

**Budget is last** — not touched during v0 iteration. Krogerrecipeshopper is the proving ground.

---

## GitHub template repository (v1)

Separate repo: `densestvoid/app-deploy-template`

- Enable **Settings → Template repository**
- Example compositions calling `densestvoid/workflows@v1` actions
- `gh repo create my-app --template densestvoid/app-deploy-template`

Workflows repo stays actions-only — no template files inside it.

---

## Workflows repo structure (v0)

```
densestvoid/workflows/
├── .github/
│   ├── workflows/
│   │   └── go-checks.yml
│   └── actions/
│       ├── setup/
│       ├── install-tool/
│       ├── build-go/
│       ├── build-docker/
│       ├── push-container/
│       ├── deploy-gate/
│       ├── deploy-terraform/
│       ├── terminate-terraform/
│       └── notify/
└── README.md
```

---

## Example composition (Go + Terraform — one pattern)

```yaml
jobs:
  gate:
    uses: densestvoid/workflows/.github/actions/deploy-gate@v0

  build:
    needs: gate
    uses: ./.github/workflows/build.yml    # Build Go + Build Docker + Push Container; outputs image_tag

  deploy:
    needs: build
    steps:
      - uses: densestvoid/workflows/.github/actions/deploy-terraform@v0
        with:
          terraform_dir: terraform/pr
          backend_key: pr/pr-${{ ... }}.tfstate
        env:
          TF_VAR_deployment_id: pr-${{ ... }}
          TF_VAR_docker_image_tag: ${{ needs.build.outputs.image_tag }}
          TF_VAR_domain_name: ${{ vars.PRODUCTION_DOMAIN }}

  prepare-notify:
    needs: [gate, build, deploy]
    if: always()

  notify:
    needs: prepare-notify
    steps:
      - uses: densestvoid/workflows/.github/actions/notify@v0
```

---

## Progress

### Budget — import removal

PR #21 (independent). Budget migration deferred until v1 after krogerrecipeshopper validation.

### Internal constants

- S3 state bucket: `densestvoid-terraform`
- Terraform version: `1.5.7`
- Pin `@v0` during iteration → `@v1` after krogerrecipeshopper validation

---

## Rejected

| Approach | Why |
|----------|-----|
| **Write Tfvars** / JSON variable maps | Callers pass `env: TF_VAR_*` per variable; complex types built in Terraform HCL |
| Build steps know about Terraform | **Build Docker** / **Push Container** are registry/build only |
| Full orchestrator workflows in workflows repo | Too thick; apps compose steps |
| `workflow_dispatch` central runner | Black box |
| Budget-first migration | Krogerrecipeshopper validates v0 first |
