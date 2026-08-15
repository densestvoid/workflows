# Deploy Orchestration Extraction — Locked-in decisions

## Core principle: composable steps only, no orchestration

The workflows repo provides **reusable actions** — atomic, composable steps. It does **not** provide orchestration workflows (no **Deploy PR**, no full pipelines, no prescribed job graphs).

Each app repo owns:
- `workflow_run` triggers
- Job ordering and `needs:` wiring
- Which steps to call and in what order
- Build composition (language, artifacts, count)
- Deploy strategy (Terraform, CLI/SDK release, or something else)

```
workflows repo = toolbox
app repo     = assembly instructions
```

A typical Go + Terraform app might compose:

```
Deploy Gate → build.yml → Deploy Terraform → prepare-notify → Notify
```

A CLI release app might compose:

```
Deploy Gate → build.yml → Release CLI → prepare-notify → Notify
```

Both are valid. The workflows repo does not prefer one over the other.

---

## Naming — keep it simple

### Actions (composable steps)

| Name | Directory | Purpose |
|------|-----------|---------|
| **Build Go** | `build-go/` | Cached Go binary build; content-hash skip logic embedded |
| **Build Docker** | `build-docker/` | Cached GHCR push; confirm-or-build |
| **Deploy Gate** | `deploy-gate/` | Deployable path diff vs `main` — skip when no deployable changes |
| **Deploy Terraform** | `deploy-terraform/` | Terraform init + apply; reads `tfvars_path`; injects infra secrets |
| **Terminate** | `terminate/` | Terraform destroy from state (`pr-destroy` pattern) |
| **Write Tfvars** | `write-tfvars/` | Non-secret tfvars file writer |
| **Notify** | `notify/` | Slack + PR comment delivery |

Future deploy steps (not v0, same composable pattern):

| Name | Purpose |
|------|---------|
| **Release CLI** | Example: publish binaries to a release target via CLI/SDK |
| **Deploy Static** | Example: upload assets to object storage |

Workflow display names in YAML match the action names: `Build Go`, `Deploy Terraform`, etc.

**No orchestrator workflows** in workflows repo beyond existing **Go Checks** (`go-checks.yml`).

---

## Skip / cache behavior — must be preserved

v0 steps must preserve budget's skip logic where applicable:

| Layer | Budget today | v0 step |
|-------|--------------|---------|
| **PR deploy gate** | Skip deploy when no deployable changes | **Deploy Gate** |
| **Go binary** | Content-hash cache; skip compile on hit | **Build Go** |
| **Docker image** | Content-hash tag; skip push when manifest exists | **Build Docker** |
| **CI go-checks** | Skip when Go sources unchanged | **Build Go** → `go_sources_changed` |

**Build Go** embeds budget's `go-build-key` logic internally.

**Build Docker** writes `tfvars_path` last (when used); **Deploy Terraform** only applies — no image checks.

Apps that don't use Go/Docker simply don't call those steps.

---

## Build — fully app-composed

Each app owns `build.yml` (or inline jobs) and composes whichever build steps it needs:

```yaml
# budget — Go + Docker
jobs:
  build:
    steps:
      - uses: densestvoid/workflows/.github/actions/build-go@v0
      - uses: densestvoid/workflows/.github/actions/build-docker@v0
      - uses: densestvoid/workflows/.github/actions/write-tfvars@v0
```

```yaml
# hypothetical CLI app — no Docker
jobs:
  build:
    steps:
      - run: npm ci && npm run build
      - uses: densestvoid/workflows/.github/actions/release-cli@v0   # future
```

Build steps have **no mandated outputs** to the orchestrator. Whatever the deploy step needs (tfvars, version tag, artifact path) is written by the app’s build job.

---

## Deploy — composable strategy, not Terraform-by-default

**Deploy Terraform** is one deploy step, not the default pipeline ending.

- Apps using infra-as-code call **Deploy Terraform** after build
- Apps using CLI/SDK release call a future **Release CLI** (or their own steps) instead
- Apps can chain multiple deploy steps if needed

**Deploy Terraform** contract:
- Inputs: `terraform_dir`, `backend_key`, `tfvars_path`
- Infra secrets from workflows repo repository secrets (injected at apply time)
- tfvars is non-secret config only

---

## Notify — delivery only

**Notify** posts caller-supplied Slack payload and/or PR comment.

**prepare-notify** stays in each app repo — message content is app-specific.

---

## Secrets

| Where | What |
|-------|------|
| **Workflows repo secrets** | Shared infra: `DO_TOKEN`, `TERRAFORM_AWS_S3_*` (used by **Deploy Terraform**, **Terminate**) |
| **App repo secrets/variables** | Slack webhooks, `PRODUCTION_DOMAIN`, app-specific tokens |
| **App repo environment** | `termination-delay` only (wait timer on terminate job) |

No secrets in tfvars.

Optional hybrid: **Deploy Terraform** accepts override secrets via inputs; default is workflows repo secrets.

---

## Initial iteration scope

**v0 — workflows repo only. Do not touch budget.**

- Ship composable actions listed above
- README documents step contracts, composition examples, and extension pattern for new deploy steps
- Pin `@v0`
- Budget migration deferred

---

## Repo template (v1)

A **GitHub template repo** (or `template/` in workflows repo) provides **example compositions**, not a runtime orchestrator:

```
template/.github/workflows/
  deploy-pr.yml              # example: gate → build.yml → Deploy Terraform → notify
  deploy-production.yml
  terminate-pr-deployment.yml
  build.yml                  # example: Build Go + Build Docker
  ci.yml
```

New apps copy the template and customize:
- Which build steps to compose
- Which deploy step(s) to call (Terraform, CLI, etc.)
- `deployable_paths`, inputs, prepare-notify content

Template updates are synced manually or via regen — apps own their files after creation.

---

## Example app composition (Go + Terraform — one pattern, not the only pattern)

```yaml
# app/.github/workflows/deploy-pr.yml
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
        terraform/**

  build:
    needs: gate
    if: needs.gate.outputs.should_deploy == 'true'
    uses: ./.github/workflows/build.yml
    secrets: inherit

  deploy:
    needs: build
    runs-on: ubuntu-latest
    steps:
      - uses: densestvoid/workflows/.github/actions/deploy-terraform@v0
        with:
          terraform_dir: terraform/pr
          backend_key: pr/pr-${{ ... }}.tfstate
          tfvars_path: terraform/pr/terraform.tfvars

  prepare-notify:
    needs: [gate, build, deploy]
    if: always()

  notify:
    needs: prepare-notify
    steps:
      - uses: densestvoid/workflows/.github/actions/notify@v0
```

Apps swap `deploy` job steps freely. Budget uses **Deploy Terraform**; a future app might replace that job entirely.

---

## Workflows repo structure (v0)

```
densestvoid/workflows/
├── .github/
│   ├── workflows/
│   │   └── go-checks.yml           # existing — CI checks, not deploy orchestration
│   └── actions/
│       ├── setup/                  # existing
│       ├── install-tool/           # existing
│       ├── build-go/
│       ├── build-docker/
│       ├── deploy-gate/
│       ├── deploy-terraform/
│       ├── terminate/
│       ├── write-tfvars/
│       └── notify/
├── template/                       # v1 — example compositions for new apps
└── README.md                       # step contracts + composition patterns
```

---

## Phases

| Phase | Scope | Budget |
|-------|-------|--------|
| **v0** | Composable actions + README | None |
| **v1** | Repo template + budget migration to composed steps | Budget |

---

## Progress

### Budget — import removal (independent)

PR #21 removes automatic Terraform import. Do not re-introduce import, orphan scripts, or empty-state cleanup hacks.

### Budget migration (v1)

Replace `deploy-reusable.yml` monolith with composed steps from workflows repo. Budget's orchestration stays in budget; only the step implementations move shared.

---

## Internal constants

- S3 state bucket: `densestvoid-terraform` (used by **Deploy Terraform** / **Terminate**)
- Terraform version: `1.5.7`
- Pin: `@v0` during iteration

---

## Rejected

| Approach | Why |
|----------|-----|
| Full **Deploy PR/Prod** orchestrator workflows in workflows repo | Too thick; can't customize build/deploy per app |
| `workflow_dispatch` central runner | Black box; async; awkward notify |
| Identical shell + repo variables only | Hides composition; inflexible deploy strategy |
| Terraform as implicit default pipeline | Deploy strategy must be composable |
