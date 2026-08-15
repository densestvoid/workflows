# Deploy Orchestration Extraction — Locked-in decisions

## Orchestration: thin app-repo chaining (Option A only)

**Decision:** App repos own thin `workflow_run` shells that statically wire shared building blocks. No `workflow_dispatch` black box.

```
gate → ./.github/workflows/build.yml → terraform-deploy@v0 → prepare-notify → notify
```

- `workflow_run` listeners stay in each app repo (GitHub requirement)
- Static `uses:` paths only — no dynamic workflow dispatch via API/gh CLI
- Workflows repo provides shared actions and `terraform-deploy` / `terminate-pr` reusables
- Goal: avoid re-orchestrating deploy/terminate logic per repo, not a single entrypoint call

---

## Secrets and variables

### GitHub Environments

**Only `termination-delay`** is a GitHub Environment (wait timer for PR auto-termination). It lives in **app repos** (budget today) on the terminate workflow job — not in the workflows repo.

All other configuration uses **repository secrets and repository variables** — no `deploy` environment or other environment-based secret stores.

### Shared / common secrets → workflows repo

Infrastructure secrets used across apps are stored as **repository secrets on the workflows repo**:

| Secret | Purpose |
|--------|---------|
| `DO_TOKEN` | DigitalOcean API |
| `TERRAFORM_AWS_S3_ACCESS_KEY` | Terraform state backend |
| `TERRAFORM_AWS_S3_ACCESS_KEY_SECRET` | Terraform state backend |
| `TERRAFORM_AWS_S3_REGION` | Terraform state backend |

`terraform-deploy` / `terraform-apply` (workflows repo) read these directly when invoked via `uses: densestvoid/workflows/...`. App deploy jobs do **not** pass infra secrets on the happy path.

**Optional hybrid:** `terraform-deploy` may declare optional `workflow_call` secrets so an app can override a shared secret when needed (different cloud account, etc.). Default: workflows repo secrets.

### Repo-specific secrets → app repo

Each app repo holds secrets and variables only it needs:

| Secret / variable | Example |
|-------------------|---------|
| `SLACK_WEBHOOK_PR` | Budget PR notifications |
| `SLACK_WEBHOOK_PROD` | Budget production notifications |
| `PRODUCTION_DOMAIN` (variable) | Budget DNS zone |
| `TERMINATION_DELAY_MINUTES` (variable, optional) | Override termination timing in notify |

Build jobs use the app repo's `GITHUB_TOKEN` (automatic) for GHCR push.

### tfvars — non-secret only

- tfvars files contain deployment config only: `deployment_id`, `docker_image_tag`, `domain_name`, `region`, etc.
- **No secrets in tfvars** — `terraform-apply` injects provider tokens from workflows repo secrets as `TF_VAR_*` / env at apply time

---

## Extension points (unchanged)

| Piece | Owner |
|-------|-------|
| `build.yml` | App repo — owns artifact correctness; may compose `go-build` / `docker-build` from workflows repo |
| `terraform/` | App repo |
| `tfvars_path` | App repo — non-secret config; written by build as final step after artifacts confirmed |
| `deployable_paths` | App repo — arbitrary globs |
| `prepare-notify` | App repo — message content |
| `notify` action | Workflows repo — delivery only |
| `deploy-gate`, `terraform-deploy`, `terminate-pr` | Workflows repo |

---

## Internal constants (workflows repo)

- S3 state bucket: `densestvoid-terraform`
- Terraform version: `1.5.7`
- Version pin during iteration: `@v0`

---

## Progress

### Budget repo — import removal (done, pending merge)

| Item | Status |
|------|--------|
| Remove "Import existing resources (optional)" from `deploy-reusable.yml` | Done — branch `cursor/remove-import-step-ed43`, draft PR #21 |
| `SETUP.md` force_cleanup clarification | Done |
| Manual A–F validation matrix | Skipped — not warranted for straight deletion |
| Workflows-repo extraction | Deferred until PR #21 merges and budget is stable on `main` |

**Budget decisions (do not re-introduce during workflows extraction):**

| Decision | Rationale |
|----------|-----------|
| No replacement import logic | Step was unreliable (`continue-on-error`), incomplete (~6 of ~10 resource types) |
| No `scripts/destroy-pr-orphans.sh` | DO API orphan discovery/delete rejected |
| No enhanced pre-cleanup / terminate for empty state | State is source of truth; destroy paths require S3 state |
| Scenario C out of scope | Delete S3 state manually → `force_cleanup` fails with name collision — operator error, acceptable |

**Expected impact (unchanged designed paths):**

| Scenario | Impact |
|----------|--------|
| A/B — normal / no-op PR redeploy | None (`is_redeployment=true`; import never ran) |
| D — failed deploy → terminate | None (state-driven `pr-destroy`) |
| E — successful terminate | None |
| F — redeploy after clean terminate | None (fresh state, no orphans) |
| C — delete S3 state → force_cleanup | Fails (name collision) — undocumented recovery; acceptable |

**Next for workflows repo:** Begin Phase 1 (`go-build-key`, `go-build`, `docker-build`, `write-tfvars`) after budget PR #21 merges.

---

## Tech debt (budget)

- ~~Remove automatic Terraform import step~~ — **done** in PR #21; merge to `main` then close
- Replace image-exists crutch with confirm-or-build in `docker-build` action (during Phase 1 extraction)

## Example app deploy shell

```yaml
# app/.github/workflows/deploy-pr.yml
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
    with:
      deployment_id: pr-${{ ... }}
      tfvars_path: terraform/pr/terraform.tfvars
      git_ref: ${{ ... }}
    secrets: inherit   # GITHUB_TOKEN for GHCR — app repo

  deploy:
    needs: build
    uses: densestvoid/workflows/.github/workflows/terraform-deploy.yml@v0
    with:
      terraform_dir: terraform/pr
      backend_key: pr/pr-${{ ... }}.tfstate
      tfvars_path: terraform/pr/terraform.tfvars
    # DO_TOKEN, TERRAFORM_AWS_S3_* — workflows repo repository secrets

  prepare-notify:
    needs: deploy
    if: always()
    # app-specific; uses SLACK_WEBHOOK_* from app repo

  notify:
    needs: prepare-notify
    steps:
      - uses: densestvoid/workflows/.github/actions/notify@v0
```

Terminate workflow in app repo uses `environment: termination-delay` for scheduled PR teardown; infra destroy secrets come from workflows repo via `terminate-pr` reusable workflow.
