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

Dependabot config lives in `.github/dependabot.yml`. Schedule weekly on Monday; group updates so each ecosystem opens one PR per week.

### What Dependabot updates

| `uses:` ref type | Updated by `github-actions` ecosystem? |
|------------------|----------------------------------------|
| `actions/*`, `dorny/*`, `hashicorp/*`, maintainer actions | Yes |
| `densestvoid/workflows/...` (toolbox reusable workflows/actions) | No — bump on toolbox release (`@v1`, `@main`) |
| `./.github/actions/...` (local composites) | No |

### This repo (workflows toolbox)

Only `github-actions` — no app `gomod`, `docker`, or `terraform` trees to track.

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
          - "actions/*"
```

Narrow the group to `actions/*` so third-party pins (e.g. `dorny/paths-filter`, `golangci/*`) stay in separate PRs and are easier to review.

### App repos (budget pattern)

Full-stack app repos add every ecosystem they ship. Reference: `densestvoid/budget` `.github/dependabot.yml`.

| Ecosystem | `directory` | When to include |
|-----------|-------------|-----------------|
| `github-actions` | `/` | Always |
| `gomod` | `/` (or module root) | Go apps |
| `docker` | `/` | Dockerfile at repo root |
| `terraform` | `/terraform/<env>` | One entry per Terraform root (e.g. `pr`, `production`) |

```yaml
version: 2
updates:
  - package-ecosystem: github-actions
    directory: /
    schedule: { interval: weekly, day: monday }
    groups:
      github-actions:
        patterns: ["*"]

  - package-ecosystem: gomod
    directory: /
    schedule: { interval: weekly, day: monday }
    groups:
      gomod:
        patterns: ["*"]

  - package-ecosystem: docker
    directory: /
    schedule: { interval: weekly, day: monday }
    groups:
      docker:
        patterns: ["*"]

  - package-ecosystem: terraform
    directory: /terraform/pr
    schedule: { interval: weekly, day: monday }
    groups:
      terraform:
        patterns: ["*"]
```

App repos use `patterns: ["*"]` per ecosystem — one grouped PR for all pins in that ecosystem. Duplicate the `terraform` block for each environment directory.

### Reviewing Dependabot PRs

- **github-actions (workflows repo):** run **go-checks** — version bumps in `go-checks.yml` and composite actions must pass vet/lint/vuln/gosec
- **github-actions (app repo):** CI must pass; deploy workflows exercise checkout, paths-filter, and artifact actions
- **gomod / docker / terraform:** CI plus any deploy-path filters that watch `go.mod`, `Dockerfile`, or `terraform/**`
- **Toolbox pins:** when cutting `@v1`, update app repos manually — Dependabot will not bump `densestvoid/workflows@...`

## Review checklist

- [ ] Skip gates use `dorny/paths-filter`
- [ ] Go jobs: named Checkout + Setup Go from go-version-file only
- [ ] Internal reusable-workflow refs use `./.github/actions/...`
- [ ] Comments and docs stay scoped to the file they are in
- [ ] README example matches current action inputs
- [ ] Dependabot: correct ecosystems per repo type; toolbox refs excluded from auto-bump
