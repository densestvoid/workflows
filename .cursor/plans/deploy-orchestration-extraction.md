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
Deploy Gate → Build Docker → prepare-notify → Notify   # image only, no Terraform
```

---

## Naming — keep it simple

### Actions (composable steps)

| Name | Directory | Purpose |
|------|-----------|---------|
| **Build Go** | `build-go/` | Cached Go binary build; content-hash skip logic embedded |
| **Build Docker** | `build-docker/` | Cached container push to registry; confirm-or-build; **no Terraform/tfvars knowledge** |
| **Deploy Gate** | `deploy-gate/` | Deployable path diff vs `main` |
| **Deploy Terraform** | `deploy-terraform/` | Terraform init + apply; variables passed as inputs |
| **Terminate Terraform** | `terminate-terraform/` | Terraform destroy from state (`pr-destroy` pattern) |
| **Notify** | `notify/` | Slack + PR comment delivery |

Future steps (same composable pattern): **Release CLI**, **Deploy Static**, etc.

**Removed:** **Write Tfvars** — variables pass directly into **Deploy Terraform** (see below).

**No orchestrator workflows** in workflows repo beyond existing **Go Checks** (`go-checks.yml`).

---

## Build Docker — registry only

**Build Docker** must not know about Terraform, tfvars, or deployment targets. It:

- Confirms or builds and pushes an image to a registry (GHCR by default)
- Outputs: `image_ref`, `image_tag` (and optionally digest)

The caller decides what happens next — **Deploy Terraform**, a future release step, or nothing. Many apps may push to Docker Hub / GHCR without any Terraform step.

**Build Go** similarly outputs build artifacts only; no deploy assumptions.

---

## Deploy Terraform — variables as inputs, not tfvars files

**Decision:** Drop **Write Tfvars**. **Deploy Terraform** accepts variables as action inputs. Callers pass non-secret values explicitly; infra secrets are injected from workflows repo repository secrets.

### Proposed **Deploy Terraform** inputs

| Input | Purpose |
|-------|---------|
| `terraform_dir` | Path to Terraform root module |
| `backend_key` | S3 state key |
| `variables` | JSON object of non-secret Terraform variables (e.g. `{"deployment_id":"pr-123","docker_image_tag":"pr-123-abc"}`) |
| `var_files` | Optional list of committed var-file paths (e.g. `terraform/pr/terraform.tfvars.example` overrides) — only if app uses static files |

Secrets (`DO_TOKEN`, etc.) are **not** in `variables` — **Deploy Terraform** maps workflows repo secrets to `TF_VAR_*` or `-var` internally.

### Caller wiring (build → deploy)

```yaml
jobs:
  build:
    uses: ./.github/workflows/build.yml
    # Build Docker outputs image_tag

  deploy:
    needs: build
    steps:
      - uses: densestvoid/workflows/.github/actions/deploy-terraform@v0
        with:
          terraform_dir: terraform/pr
          backend_key: pr/pr-123.tfstate
          variables: |
            {
              "deployment_id": "pr-123",
              "docker_image_tag": "${{ needs.build.outputs.image_tag }}"
            }
```

### Issues with dropping tfvars files (and mitigations)

| Issue | Severity | Mitigation |
|-------|----------|------------|
| **GitHub Actions inputs are strings** | Low | `variables` is a JSON string; action parses and sets `TF_VAR_*` or `-var` per key |
| **Complex Terraform types** (objects, maps, lists) | Medium | JSON encoding in `variables` works for most types Terraform accepts via `-var`; document examples for `domain`-style objects. Apps with exotic HCL may use optional `var_files` for static committed files |
| **Many variables per app** | Low | Each app passes only what its module needs; no shared schema required. Verbose but explicit |
| **HCL escaping / special characters** | Medium | Action uses `jq` + `terraform -var` with proper quoting; document constraints |
| **Per-app variable names differ** | Low (feature) | No fixed action input per var — generic JSON map. Budget passes `docker_image_tag`; another app passes different keys |
| **Checked-in defaults** | Low | Optional `var_files` input for `*.tfvars` committed in app repo; dynamic values from build still passed via `variables` JSON |
| **Sensitive values in `variables` JSON** | High if misused | Document: never put secrets in `variables`; secrets only via workflows repo secret injection. Action rejects known secret key names? Optional guard |
| **Debugging** | Low | Action logs variable *keys* applied, not values (or masks values) |

**Verdict:** Passing variables as inputs is cleaner than **Write Tfvars** — better separation (build doesn't write deploy config), aligns with **Build Docker** having no Terraform knowledge. Optional `var_files` covers edge cases without making file construction a required step.

---

## Skip / cache behavior — must be preserved

| Layer | Budget today | v0 step |
|-------|--------------|---------|
| **PR deploy gate** | Skip when no deployable changes | **Deploy Gate** |
| **Go binary** | Content-hash cache | **Build Go** |
| **Docker image** | Content-hash tag; confirm-or-build | **Build Docker** |
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
    uses: ./.github/workflows/build.yml    # Build Go + Build Docker; outputs image_tag

  deploy:
    needs: build
    steps:
      - uses: densestvoid/workflows/.github/actions/deploy-terraform@v0
        with:
          terraform_dir: terraform/pr
          backend_key: pr/pr-${{ ... }}.tfstate
          variables: |
            {"deployment_id":"pr-${{ ... }}","docker_image_tag":"${{ needs.build.outputs.image_tag }}"}

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
| **Write Tfvars** action | Variables pass directly to **Deploy Terraform**; build steps don't write deploy config |
| Build Docker knows about Terraform | Registry push only; caller wires deploy separately |
| Full orchestrator workflows in workflows repo | Too thick; apps compose steps |
| `workflow_dispatch` central runner | Black box |
| Budget-first migration | Krogerrecipeshopper validates v0 first |
