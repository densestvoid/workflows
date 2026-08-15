# Deploy Orchestration Extraction — Locked-in decisions

## Orchestration: thin composition in each app repo

**Decision:** Each app repo owns and composes its own thin `workflow_run` shells. The workflows repo provides **building blocks** (actions); apps wire them together. No black box, no `workflow_dispatch`, no centralized full-pipeline workflow in workflows repo.

```
gate → build.yml → Deploy Terraform → prepare-notify → notify
  │        │              │
  │        │              └── workflows repo action
  │        └── app repo (composes Build Go + Build Docker)
  └── workflows repo action
```

- `workflow_run` listeners live in each app repo (GitHub requirement)
- Static `uses:` paths only
- Apps differ in `with:` inputs, `deployable_paths`, `build.yml`, and `prepare-notify` — that is intentional
- **Repo template** (post-v0) bootstraps new apps with canonical copies of the thin shells — not runtime centralization

**Rejected alternatives:** identical shell + repo variables only, `workflow_dispatch` central runner, full **Deploy PR** orchestrator workflow in workflows repo that hides composition.

---

## Naming — keep it simple

Use plain, readable names. No `terraform-deploy`, `deploy-orchestrate`, `go-build-key`, etc.

### Workflows repo provides actions only (v0)

No full **Deploy PR** / **Deploy Prod** / **Terminate PR** orchestrator workflows in workflows repo. Apps compose these jobs locally using the actions below.

### Actions (composable building blocks)

| Name | Directory | Purpose |
|------|-----------|---------|
| **Build Go** | `build-go/` | Cached Go binary build |
| **Build Docker** | `build-docker/` | Cached container push to GHCR; confirm-or-build |
| **Deploy Gate** | `deploy-gate/` | Deployable path diff vs `main` — skip deploy when no deployable changes |
| **Deploy Terraform** | `deploy-terraform/` | Terraform init + apply (reads `tfvars_path`) |
| **Terminate** | `terminate/` | Terraform destroy from state (`pr-destroy` pattern) |
| **Write Tfvars** | `write-tfvars/` | Non-secret tfvars file writer |
| **Notify** | `notify/` | Slack + PR comment delivery |

Go content-hash logic lives inside **Build Go** (no separate `go-build-key` action) — but must preserve budget's existing skip behavior (see below).

Workflow display names in YAML: `Build Go`, `Build Docker`, `Deploy Gate`, `Deploy Terraform`, `Terminate`, `Notify`.

### App repo owns workflow files (composed from actions)

| App workflow file | Composes |
|-------------------|----------|
| `deploy-pr.yml` | `workflow_run` → gate → `build.yml` → Deploy Terraform → prepare-notify → notify |
| `deploy-production.yml` | `workflow_run` → gate (optional) → `build.yml` → Deploy Terraform → prepare-notify → notify |
| `terminate-pr-deployment.yml` | `workflow_run` → Terminate → prepare-notify → notify |
| `build.yml` | Build Go → Build Docker → Write Tfvars |

---

## Skip / cache behavior — must be preserved

v0 extraction must maintain the same skip logic budget uses today. No regression on unnecessary builds or deploys.

| Layer | Current behavior (budget) | v0 equivalent |
|-------|---------------------------|---------------|
| **PR deploy gate** | Skip entire deploy when branch has no deployable changes vs `main` | **Deploy Gate** |
| **Go binary** | Content-hash cache key; skip compile when cache hit | **Build Go** (internal hash + cache) |
| **Docker image** | Content-hash tag (`{deployment_id}-{docker_hash}`); skip push when manifest exists in GHCR | **Build Docker** (confirm-or-build) |
| **CI go-checks** | Skip when Go sources unchanged vs base | **Build Go** outputs `go_sources_changed` for caller (e.g. `go-checks` workflow) |

**Build Go** embeds the logic from budget's `go-build-key` action — hash `*.go` / `go.mod` / `go.sum`, detect change vs base revision, drive cache keys and skip decisions. Not a separate published action; same behavior, simpler surface.

**Build Docker** runs only when needed; writes `tfvars_path` last, after image is confirmed. **Deploy Terraform** never checks images — it only applies.

**Deploy Gate** globs are per-app; examples in docs are not requirements.

---

## Initial iteration scope

**Workflows repo only. Do not touch the budget app.**

- Build and validate all actions/workflows here first
- Pin `@v0` for internal iteration
- Budget migration is a **later phase** — after workflows repo pieces are stable and documented
- Budget PR #21 (import removal) proceeds independently; no workflows-repo work should depend on or modify budget during v0

---

## Secrets and variables

### GitHub Environments

**Only `termination-delay`** is a GitHub Environment (wait timer for PR auto-termination). It lives in **app repos** on the terminate workflow job — not in the workflows repo.

All other configuration uses **repository secrets and repository variables**.

### Shared / common secrets → workflows repo

| Secret | Purpose |
|--------|---------|
| `DO_TOKEN` | DigitalOcean API |
| `TERRAFORM_AWS_S3_ACCESS_KEY` | Terraform state backend |
| `TERRAFORM_AWS_S3_ACCESS_KEY_SECRET` | Terraform state backend |
| `TERRAFORM_AWS_S3_REGION` | Terraform state backend |

**Deploy PR** / **Deploy Prod** read these from workflows repo repository secrets. App deploy jobs do not pass infra secrets on the happy path.

**Optional hybrid:** reusable workflows may declare optional `workflow_call` secrets for per-app overrides. Default: workflows repo secrets.

### Repo-specific secrets → app repo

| Secret / variable | Example |
|-------------------|---------|
| `SLACK_WEBHOOK_PR` | PR notifications |
| `SLACK_WEBHOOK_PROD` | Production notifications |
| `PRODUCTION_DOMAIN` (variable) | DNS zone |
| `TERMINATION_DELAY_MINUTES` (variable, optional) | Termination timing in notify |

Build uses the app repo's `GITHUB_TOKEN` for GHCR push.

### tfvars — non-secret only

- Deployment config only: `deployment_id`, `docker_image_tag`, `domain_name`, `region`, etc.
- **No secrets in tfvars** — **Deploy Terraform** injects provider tokens from workflows repo secrets at apply time

---

## Build / deploy separation

Build and deploy remain **separate jobs/workflows**:

| Step | Responsibility |
|------|----------------|
| **Build Go** / **Build Docker** | Confirm-or-build, push if needed, write `tfvars_path` (last step) |
| **Deploy Terraform** | Terraform init + apply only; consumes `tfvars_path` |

**Deploy PR** / **Deploy Prod** are app-repo workflow *names*, not workflows-repo reusables.

---

## Extension points

| Piece | Owner |
|-------|-------|
| App `build.yml` | App repo — composes **Build Go** + **Build Docker**; owns artifact correctness |
| `terraform/` | App repo |
| `tfvars_path` | App repo — non-secret config |
| `deployable_paths` | App repo — arbitrary globs |
| `prepare-notify` | App repo — message content |
| **Deploy Gate**, **Deploy Terraform**, **Terminate**, **Notify**, **Build Go**, **Build Docker** | Workflows repo (actions) |
| `deploy-pr.yml`, `build.yml`, `prepare-notify`, `terminate-pr-deployment.yml` | App repo (composed) |

---

## Internal constants (workflows repo)

- S3 state bucket: `densestvoid-terraform`
- Terraform version: `1.5.7`
- Version pin during iteration: `@v0`

---

## Progress

### Budget repo — import removal (independent, do not touch during v0)

| Item | Status |
|------|--------|
| Remove import step from `deploy-reusable.yml` | Done — PR #21 (draft) |
| `SETUP.md` force_cleanup clarification | Done |
| Budget migration to workflows repo | **Deferred** — after v0 stabilizes |

### Workflows repo — v0 (current work)

| Phase | Scope | Budget changes |
|-------|-------|----------------|
| **v0** | Extract actions: **Build Go**, **Build Docker**, **Deploy Gate**, **Deploy Terraform**, **Terminate**, **Write Tfvars**, **Notify**; README | **None** |
| **v1** | Repo template with canonical thin shells; budget adopts from template | Budget + template |

---

## Proposed workflows repo structure (v0)

```
densestvoid/workflows/
├── .github/
│   ├── workflows/
│   │   └── go-checks.yml           # existing
│   └── actions/
│       ├── setup/                  # existing
│       ├── install-tool/           # existing
│       ├── build-go/               # Build Go
│       ├── build-docker/           # Build Docker
│       ├── deploy-gate/            # Deploy Gate
│       ├── deploy-terraform/       # Deploy Terraform
│       ├── terminate/              # Terminate
│       ├── write-tfvars/           # Write Tfvars
│       └── notify/                 # Notify
├── template/                       # v1 — repo template for new apps
│   └── .github/workflows/
│       ├── deploy-pr.yml
│       ├── deploy-production.yml
│       ├── terminate-pr-deployment.yml
│       ├── build.yml
│       └── ci.yml
└── README.md
```

---

## Repo template (v1)

New apps are bootstrapped from a **GitHub template repository** (or `template/` in workflows repo) containing canonical thin orchestration files. Each repo copies and customizes:

- `with:` inputs (`deployable_paths`, `terraform_dir`, `image_name`, etc.)
- `build.yml` composition (which actions to call)
- `prepare-notify` message content

The template is the **starting point**, not a runtime dependency. Apps own their workflow files after creation; sync template updates manually or via scripted regen when the canonical pattern changes.

---

## Example app composition (canonical pattern)

```yaml
# app/.github/workflows/deploy-pr.yml — composed by each app (from template)
name: Deploy PR

on:
  workflow_run:
    workflows: [CI]
    types: [completed]
    branches-ignore: [main]

jobs:
  gate:
    uses: densestvoid/workflows/.github/actions/deploy-gate@v0
    with:
      deployable_paths: |
        **/*.go
        go.mod
        Dockerfile*
        terraform/**

  build:
    needs: gate
    if: needs.gate.outputs.should_deploy == 'true'
    uses: ./.github/workflows/build.yml
    with:
      deployment_id: pr-${{ github.event.workflow_run.pull_requests[0].number }}
      tfvars_path: terraform/pr/terraform.tfvars
      git_ref: ${{ github.event.workflow_run.head_sha }}
    secrets: inherit

  deploy:
    needs: build
    runs-on: ubuntu-latest
    steps:
      - uses: densestvoid/workflows/.github/actions/deploy-terraform@v0
        with:
          terraform_dir: terraform/pr
          backend_key: pr/pr-${{ github.event.workflow_run.pull_requests[0].number }}.tfstate
          tfvars_path: terraform/pr/terraform.tfvars

  prepare-notify:
    needs: deploy
    if: always()
    # app-specific message construction

  notify:
    needs: prepare-notify
    steps:
      - uses: densestvoid/workflows/.github/actions/notify@v0
```

---

## Budget import removal — do not re-introduce

| Rejected | Rationale |
|----------|-----------|
| Replacement import logic | Unreliable, incomplete |
| `destroy-pr-orphans.sh` | DO API orphan scripts rejected |
| Enhanced pre-cleanup/terminate for empty state | State is source of truth |

Scenario C (manual S3 state delete → force_cleanup fails) is acceptable operator error.
