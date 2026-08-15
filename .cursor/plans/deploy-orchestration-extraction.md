# Deploy Orchestration Extraction — Locked-in decisions

## Orchestration: thin app-repo chaining (Option A only)

**Decision:** App repos own thin `workflow_run` shells that statically wire shared building blocks. No `workflow_dispatch` black box.

```
gate → Build Go/Docker → Deploy PR/Prod → prepare-notify → notify
```

- `workflow_run` listeners stay in each app repo (GitHub requirement)
- Static `uses:` paths only — no dynamic workflow dispatch via API/gh CLI
- Workflows repo provides shared actions and reusable workflows
- Goal: avoid re-orchestrating deploy/terminate logic per repo, not a single entrypoint call

---

## Naming — keep it simple

Use plain, readable names. No `terraform-deploy`, `deploy-orchestrate`, `go-build-key`, etc.

### Workflows (reusable `workflow_call`)

| Name | File | Purpose |
|------|------|---------|
| **Deploy PR** | `deploy-pr.yml` | PR deploy pipeline (gate → build → terraform apply) |
| **Deploy Prod** | `deploy-production.yml` | Production deploy pipeline |
| **Terminate PR** | `terminate-pr.yml` | PR teardown from state |

### Actions (composable building blocks)

| Name | Directory | Purpose |
|------|-----------|---------|
| **Build Go** | `build-go/` | Cached Go binary build |
| **Build Docker** | `build-docker/` | Cached container push to GHCR; confirm-or-build |
| **Deploy Gate** | `deploy-gate/` | Deployable path diff vs `main` |
| **Deploy** | `deploy/` | Terraform init + apply (reads `tfvars_path`) |
| **Terminate** | `terminate/` | Terraform destroy from state (`pr-destroy` pattern) |
| **Write Tfvars** | `write-tfvars/` | Non-secret tfvars file writer |
| **Notify** | `notify/` | Slack + PR comment delivery |

Go content-hash logic lives inside **Build Go** — no separate `go-build-key` action unless it proves necessary.

Workflow display names in YAML: `Deploy PR`, `Deploy Prod`, `Build Go`, `Build Docker`, etc.

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
- **No secrets in tfvars** — **Deploy** action injects provider tokens from workflows repo secrets at apply time

---

## Build / deploy separation

Build and deploy remain **separate jobs/workflows**:

| Step | Responsibility |
|------|----------------|
| **Build Go** / **Build Docker** | Confirm-or-build, push if needed, write `tfvars_path` (last step) |
| **Deploy PR** / **Deploy Prod** | Terraform init + apply only; consumes `tfvars_path` |

---

## Extension points

| Piece | Owner |
|-------|-------|
| App `build.yml` | App repo — composes **Build Go** + **Build Docker**; owns artifact correctness |
| `terraform/` | App repo |
| `tfvars_path` | App repo — non-secret config |
| `deployable_paths` | App repo — arbitrary globs |
| `prepare-notify` | App repo — message content |
| **Notify** | Workflows repo — delivery only |
| **Deploy Gate**, **Deploy PR/Prod**, **Terminate PR** | Workflows repo |

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
| **v0** | Extract **Build Go**, **Build Docker**, **Deploy Gate**, **Deploy**, **Terminate**, **Write Tfvars**, **Notify**; add **Deploy PR**, **Deploy Prod**, **Terminate PR** workflows; README | **None** |
| **v1** | Budget adopts thin shells + app `build.yml` | Budget only, after v0 merge |

---

## Proposed workflows repo structure (v0)

```
densestvoid/workflows/
├── .github/
│   ├── workflows/
│   │   ├── go-checks.yml           # existing
│   │   ├── deploy-pr.yml           # Deploy PR
│   │   ├── deploy-production.yml   # Deploy Prod
│   │   └── terminate-pr.yml        # Terminate PR
│   └── actions/
│       ├── setup/                  # existing
│       ├── install-tool/           # existing
│       ├── build-go/               # Build Go
│       ├── build-docker/           # Build Docker
│       ├── deploy-gate/            # Deploy Gate
│       ├── deploy/                 # Deploy (terraform init + apply)
│       ├── terminate/              # Terminate
│       ├── write-tfvars/           # Write Tfvars
│       └── notify/                 # Notify
└── README.md
```

---

## Example app shell (future — not v0)

```yaml
# app/.github/workflows/deploy-pr.yml  (budget migration, post-v0)
jobs:
  gate:
    uses: densestvoid/workflows/.github/actions/deploy-gate@v0

  build:
    needs: gate
    uses: ./.github/workflows/build.yml    # composes Build Go + Build Docker

  deploy:
    needs: build
    uses: densestvoid/workflows/.github/workflows/deploy-pr.yml@v0

  prepare-notify:
    needs: deploy
    if: always()

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
