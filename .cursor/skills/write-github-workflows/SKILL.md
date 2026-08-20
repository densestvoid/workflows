---
name: write-github-workflows
description: >-
  Authors and reviews GitHub Actions workflows and composite actions for the
  densestvoid/workflows toolbox. Use when adding or changing .github/workflows,
  .github/actions, .github/dependabot.yml, README deploy examples, caller skip
  logic, or reviewing PRs in this repo.
---

# Write GitHub Workflows

## Skill routing

| Task | Skill |
|------|-------|
| Go jobs, go-checks, nested go.mod | [go-toolchain-setup](go-toolchain-setup/SKILL.md) |
| Read Terraform output after deploy | [terraform-output-inline](terraform-output-inline/SKILL.md) |
| Everything else in this repo | This skill |

## Internal action refs (this repo only)

| Context | Pattern |
|---------|---------|
| Caller repo invokes toolbox | `densestvoid/workflows/.github/workflows/go-checks.yml@v1` |
| Reusable workflow invokes sibling action | `./.github/actions/install-go-tool` |

Relative `./` paths resolve at the ref the caller pinned on the reusable workflow.

## Design principles

1. **Toolbox, not orchestration** — app repos own triggers, job graphs, `needs:`, and skip gates
2. **Caller-owned skip** — `dorny/paths-filter@v3` in workflow `if:`, not trigger `paths` when required checks must still run
3. **Atomic actions** — one concern per composite; delegate to official/maintainer actions inside
4. **TF_VAR_*** — Terraform module variables via env on the invoking step
5. **Encapsulated internals** — cache keys and registry inspect logic stay inside actions

## Default choices

| Need | Use | Avoid |
|------|-----|-------|
| Path skip gates | `dorny/paths-filter@v3` | Custom git diff; trigger `paths` blocking required CI |
| Docker build + push | **build-docker** | Raw buildx/login/metadata chain in callers |
| Go toolchain | checkout + setup-go | Removed **setup-go** composite |
| Go checks | Reusable **go-checks.yml** | Duplicating five parallel jobs |
| Terraform output (same job) | Inline `terraform output -raw` | Removed **terraform-output** |
| Internal refs in reusable workflows | `./.github/actions/...` | `@main` self-references in this repo |
| Slack + PR notify | **notify** | Duplicate slack + github-script |
| Deploy / destroy | **deploy-terraform** / **terminate-terraform** | Inline init/apply/s3 rm |

## Toolbox actions (v1 pin in callers)

| Action | Purpose |
|--------|---------|
| **build-go** | One binary + artifact |
| **build-docker** | GHCR push; optional Docker Hub; `image-built` when tag exists |
| **deploy-terraform** | init + apply |
| **terminate-terraform** | destroy module + S3 state delete |
| **notify** | Slack + PR comment |
| **install-go-tool** | Cached `go install` |
| **go-checks.yml** | Parallel vet/staticcheck/lint/vuln/gosec |

## Documentation conventions

- Action README sections describe that action only — chain composition lives in the deploy pipeline example
- Inline comments explain non-obvious bash or `${{ }}` syntax, not other actions
- Reusable workflow headers document expression syntax used in that file

## Dependabot

Dependabot config lives in `.github/dependabot.yml`. Schedule weekly on Monday. Use groups to batch related updates into one PR per group.

### What Dependabot updates

| `uses:` ref type | Updated by `github-actions` ecosystem? |
|------------------|----------------------------------------|
| `actions/*`, `dorny/*`, `hashicorp/*`, maintainer actions | Yes — `github-actions` group |
| `densestvoid/workflows/...` (toolbox reusable workflows/actions) | Yes — separate `workflows-toolbox` group |
| `./.github/actions/...` (local composites) | No |

### This repo (workflows toolbox)

Only `github-actions` — no app `gomod`, `docker`, or `terraform` trees to track. Group **all** action and workflow pins in one PR (`patterns: ["*"]`); do not split by publisher (e.g. `actions/*` only).

```yaml
version: 2
updates:
  - package-ecosystem: github-actions
    directory: /
    schedule:
      interval: weekly
      day: monday
    groups:
      github-actions:
        patterns:
          - "*"
```

Canonical file: `.github/dependabot.yml`.

### App repos (budget pattern)

Full-stack app repos add every ecosystem they ship. One `package-ecosystem` entry per root (same model as terraform): each `go.mod` directory, each Dockerfile directory, each `terraform/<env>` directory.

| Ecosystem | `directory` | Group name | When to include |
|-----------|-------------|------------|-----------------|
| `github-actions` | `/` | `workflows-toolbox` | App pins `densestvoid/workflows@...` |
| `github-actions` | `/` | `github-actions` | All other third-party pins |
| `gomod` | `/` or module root | `gomod-<name>` | One entry per `go.mod` (e.g. `/`, `/backend`) |
| `docker` | `/` or image root | `docker-<name>` | One entry per Dockerfile directory |
| `terraform` | `/terraform/<env>` | `terraform-<env>` | One entry per Terraform root |

**github-actions** — split toolbox from everything else so toolbox bumps surface as their own PR (which apps need updating, and to what ref):

```yaml
  - package-ecosystem: github-actions
    directory: /
    schedule: { interval: weekly, day: monday }
    groups:
      workflows-toolbox:
        patterns:
          - "densestvoid/workflows*"
      github-actions:
        patterns:
          - "*"
        exclude-patterns:
          - "densestvoid/workflows*"
```

**gomod / docker / terraform** — duplicate the block per root; name each group after the path:

```yaml
  - package-ecosystem: gomod
    directory: /
    schedule: { interval: weekly, day: monday }
    groups:
      gomod-root:
        patterns: ["*"]

  - package-ecosystem: gomod
    directory: /backend
    schedule: { interval: weekly, day: monday }
    groups:
      gomod-backend:
        patterns: ["*"]

  - package-ecosystem: docker
    directory: /
    schedule: { interval: weekly, day: monday }
    groups:
      docker-root:
        patterns: ["*"]

  - package-ecosystem: terraform
    directory: /terraform/pr
    schedule: { interval: weekly, day: monday }
    groups:
      terraform-pr:
        patterns: ["*"]

  - package-ecosystem: terraform
    directory: /terraform/production
    schedule: { interval: weekly, day: monday }
    groups:
      terraform-production:
        patterns: ["*"]
```

Reference: `densestvoid/budget` `.github/dependabot.yml` (single root `gomod`/`docker`; two terraform roots).

### Reviewing Dependabot PRs

- **github-actions (workflows repo):** run **go-checks** — all pin bumps land in one grouped PR
- **workflows-toolbox (app repo):** CI must pass; verify every `densestvoid/workflows@...` pin moved together (reusable workflow + action refs at the same tag)
- **github-actions (app repo):** CI and deploy workflows exercise checkout, paths-filter, and artifact actions
- **gomod / docker / terraform:** scoped CI for that root; deploy-path filters that watch `go.mod`, `Dockerfile`, or `terraform/**`

## Review checklist

- [ ] Skip gates use `dorny/paths-filter`
- [ ] Go jobs: named Checkout + Setup Go from go-version-file only
- [ ] Internal reusable-workflow refs use `./.github/actions/...`
- [ ] Comments and docs stay scoped to the file they are in
- [ ] README example matches current action inputs
- [ ] Dependabot: workflows repo groups all `github-actions` with `*`; app repos split `workflows-toolbox` from third-party pins and use one entry per `go.mod`, Dockerfile, and terraform root
