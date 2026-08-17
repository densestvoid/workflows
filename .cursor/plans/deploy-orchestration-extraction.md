# Deploy Orchestration Extraction — Decisions

## Architecture

- Workflows repo = composable **actions** only (toolbox). No orchestration workflows.
- App repos own triggers, job graph, and composition.
- Composite actions for all v0 steps.

## Secrets

- All secrets/vars live in the **app repo** (no org secrets for now).
- Terraform actions take credentials as **inputs**; caller passes `${{ secrets.* }}`.
- `termination-delay` is the only GitHub Environment secret (terminate job).
- App-deploy-template documents required secret names.

## Actions

| Action | Directory | Decision |
|--------|-----------|----------|
| Detect Go Changes | `detect-go-changes/` | Pre-CI diff + content hash; outputs `changed`, `build-key` |
| Build Go | `build-go/` | Binary build + artifact upload; **no Docker awareness** |
| Build Docker | `build-docker/` | Local `docker buildx`; binary via **artifact** from Build Go |
| Push Container | `push-container/` | Multiplex all configured registries (Notify pattern); skip unset; **fail if any configured registry fails** |
| Deploy Gate | `deploy-gate/` | Path diff vs `main`; own checkout |
| Deploy Terraform | `deploy-terraform/` | Init + apply; variables as **action inputs**; outputs = full `terraform output -json` |
| Terminate Terraform | `terminate-terraform/` | `pr-destroy` empty module (in workflows repo); delete S3 state on success |
| Notify | `notify/` | Slack + PR comment; **text inputs only**; app owns message construction |
| Go Checks | `go-checks.yml` | Existing reusable workflow; pairs with detect-go-changes |

Future: Release CLI, Deploy Static (same pattern).

## Contracts

### detect-go-changes

| In | Out |
|----|-----|
| `working-directory`, `base-ref` | `changed`, `build-key` |

### build-go

| In | Out |
|----|-----|
| `working-directory`, `build-key`, `binary-name`, `main-package`, `skip` | `cache-hit`, `artifact-name` |

Uploads binary artifact. No Docker.

### build-docker

| In | Out |
|----|-----|
| `context`, `dockerfile`, `local-tag`, `binary-artifact`, `binary-path`, `skip` | `local-tag`, `docker-build-key` |

No registry, push, or Terraform.

### push-container

| In | Out |
|----|-----|
| `local-tag`, `tag`, `check-only` | per-registry `*_image_ref`, `*_pushed`, `*_exists`; aggregate `exists` |
| optional: `ghcr-*`, `dockerhub-*` | |

### deploy-gate

| In | Out |
|----|-----|
| `base-ref`, `paths` | `should-deploy` |

### deploy-terraform

| In | Out |
|----|-----|
| `terraform-dir`, `backend-key`, `do-token`, `terraform-aws-*`, `variables` | `outputs` (JSON), `output.<name>` |

- S3 bucket: `densestvoid-terraform` (hardcoded)
- Terraform version: `1.5.7`
- CI passes scalars only; complex types built in app Terraform `locals`
- Static defaults in app `*.auto.tfvars` / `variables.tf`

### terminate-terraform

| In | Out |
|----|-----|
| `terraform-dir`, `backend-key`, `do-token`, `terraform-aws-*`, `variables` | `destroyed`, `state-deleted` |

### notify

| In | Out |
|----|-----|
| `slack-webhook`, `slack-text`, `pr-number`, `pr-body`, `github-token` | — |

## Cross-cutting

**Checkout:** Each action checks out exactly what it needs — sparse paths, minimum history, no full-repo clones.

**Skip logic:** detect-go-changes (`changed`) · Deploy Gate (`should-deploy`) · Build Go (cache) · Push Container (`exists` → caller skips build/push)

**Build pipeline:** Build Go → Build Docker → Push Container → Deploy Terraform (separate jobs)

**Versioning:** Action dirs are unversioned (`build-go/`). Pin via git ref: `@v1` (coupled, default) or `@deploy-terraform/v1.1.0` (independent escape hatch). Dev: `@main` or SHA.

## Rollout

| Phase | Scope |
|-------|-------|
| v0 | Build actions in `workflows` |
| v0 test | Validate via `krogerrecipeshopper` |
| v1 | Cut `@v1`; create `densestvoid/app-deploy-template` |
| v1 migrate | Budget last |

Integration test via kroger — no unit tests in workflows repo.

## Repo structure

```
.github/
├── workflows/go-checks.yml
├── actions/{setup,install-tool,detect-go-changes,build-go,build-docker,push-container,deploy-gate,deploy-terraform,terminate-terraform,notify}
└── terraform/pr-destroy/
```
