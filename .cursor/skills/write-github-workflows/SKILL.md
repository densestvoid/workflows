---
name: write-github-workflows
description: >-
  Authors and reviews GitHub Actions workflows and composite actions for the
  densestvoid/workflows toolbox. Use when adding or changing .github/workflows,
  .github/actions, README deploy examples, caller skip logic, Docker/Go/Terraform
  deploy pipelines, or reviewing PRs in this repo.
---

# Write GitHub Workflows

## Skill routing

| Task | Skill |
|------|-------|
| Go jobs, go-checks, nested go.mod | [go-toolchain-setup](go-toolchain-setup/SKILL.md) |
| Read Terraform output after deploy | [terraform-output-inline](terraform-output-inline/SKILL.md) |
| Everything else in this repo | This skill |

## Internal action refs (this repo only)

Inside `.github/workflows/*.yml` in **this repo**, reference sibling actions with `./.github/actions/...` — not `densestvoid/workflows/...@main`.

| Context | Pattern |
|---------|---------|
| Caller repo invokes toolbox | `densestvoid/workflows/.github/workflows/go-checks.yml@v1` |
| Reusable workflow invokes sibling action | `./.github/actions/install-go-tool` |

Relative paths resolve at the **ref the caller pinned** on the reusable workflow. Example: app pins `go-checks.yml@v1` → `./.github/actions/install-go-tool` is also `@v1`.

`github.ref` / `github.sha` refer to the **caller** — only matters if the workflow must checkout this repo for scripts beyond the action definition.

Canonical: `.github/workflows/go-checks.yml` Static-Analysis job.

## Design principles

1. **Toolbox, not orchestration** — app repos own triggers, job graphs, `needs:`, and skip gates
2. **Caller-owned skip** — `dorny/paths-filter@v3` in workflow `if:`, not workflow trigger `paths` when required checks must still run
3. **Atomic actions** — one concern per composite; delegate to official/maintainer actions inside composites
4. **TF_VAR_*** — Terraform module variables via env on the invoking step, not action inputs
5. **Encapsulated cache** — **build-go** caches internally; **build-docker** `tag` is caller-supplied for registry skip

## Default choices

| Need | Use | Avoid |
|------|-----|-------|
| Path skip gates | `dorny/paths-filter@v3` | Custom git diff; trigger `paths` blocking required CI |
| Docker build + push | **build-docker** (GHCR required; Docker Hub optional) | Raw buildx/login chain in callers; metadata-action boilerplate |
| Go toolchain | checkout + setup-go — see **go-toolchain-setup** | Removed **setup-go** composite |
| Go checks | Reusable **go-checks.yml** | Duplicating five parallel jobs in every app |
| Terraform output (same job) | Inline `terraform output -raw` — see **terraform-output-inline** | Removed **terraform-output** |
| Internal refs in reusable workflows | `./.github/actions/...` | `@main` self-references in this repo |
| Slack + PR notify | **notify** | Duplicate slack + github-script |
| Deploy / destroy | **deploy-terraform** / **terminate-terraform** | Inline init/apply/s3 rm |
| Cache | `actions/cache/restore` + `actions/cache/save` @v6 | Custom cache dirs |

## Toolbox actions (v1 pin in callers)

| Action | Purpose |
|--------|---------|
| **build-go** | One binary + artifact; binary cache internal |
| **build-docker** | GHCR push + optional Docker Hub; `image-built` when tag exists |
| **deploy-terraform** | init + apply |
| **terminate-terraform** | destroy module + S3 state delete |
| **notify** | Slack + PR comment |
| **install-go-tool** | Cached `go install` |
| **go-checks.yml** | Parallel vet/staticcheck/lint/vuln/gosec |

Pin callers at one repo ref (`@v1`). Bump all pins together on release.

## Composite conventions

1. One concern per action; skip logic stays in callers
2. Wrap official actions inside composites (**build-docker** pattern)
3. Named steps for checkout/setup/tool (log visibility)
4. Document non-obvious behavior in README caller notes

## Review checklist

- [ ] Skip gates use `dorny/paths-filter`
- [ ] **build-docker** in callers, not raw Docker action chain
- [ ] Go jobs: named Checkout + Setup Go from go-version-file only
- [ ] Internal reusable-workflow refs use `./.github/actions/...`
- [ ] README example matches current action inputs
- [ ] Focused skill updated if pattern changed
