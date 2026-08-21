---
name: dependabot-workflows
description: >-
  Authors and reviews Dependabot config for densestvoid/workflows and app repos
  using the toolbox. Use when adding or changing .github/dependabot.yml,
  grouping github-actions pins, or reviewing Dependabot PRs.
---

# Dependabot Workflows

## Basics

Config lives in `.github/dependabot.yml`. Schedule weekly on Monday. Use groups to batch related updates into one PR per group.

## What Dependabot updates

| `uses:` ref type | Updated by `github-actions` ecosystem? |
|------------------|----------------------------------------|
| `actions/*`, `dorny/*`, `hashicorp/*`, maintainer actions | Yes — `github-actions` group |
| `densestvoid/workflows/...` (toolbox reusable workflows/actions) | Yes — separate `workflows-toolbox` group |
| `./.github/actions/...` (local composites) | No |
| `$/.github/actions/...` (self-repo at running commit) | No — inherits the reusable workflow pin |
| actionlint binary (curl download script) | No — intentionally floats with upstream `main` script |

## This repo (workflows toolbox)

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

## App repos (budget pattern)

Full-stack app repos add every ecosystem they ship. One `package-ecosystem` entry per root (same model as terraform): each `go.mod` directory, each Dockerfile directory, each `terraform/<env>` directory.

| Ecosystem | `directory` | Group name | When to include |
|-----------|-------------|------------|-----------------|
| `github-actions` | `/` | `workflows-toolbox` | App pins `densestvoid/workflows@...` |
| `github-actions` | `/` | `github-actions` | All other third-party pins |
| `gomod` | `/` or module root | `gomod-<name>` | One entry per `go.mod` (e.g. `/`, `/backend`) |
| `docker` | `/` or image root | `docker-<name>` | One entry per Dockerfile directory |
| `terraform` | `/terraform/<env>` | `terraform-<env>` | One entry per Terraform root |

**github-actions** — split toolbox from everything else so toolbox bumps surface as their own PR:

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

## Reviewing Dependabot PRs

- **github-actions (workflows repo):** CI must pass — all pin bumps land in one grouped PR
- **workflows-toolbox (app repo):** CI must pass; verify every `densestvoid/workflows@...` pin moved together (reusable workflow + action refs at the same tag)
- **github-actions (app repo):** CI and deploy workflows exercise checkout, paths-filter, and artifact actions
- **gomod / docker / terraform:** scoped CI for that root; deploy-path filters that watch `go.mod`, `Dockerfile`, or `terraform/**`

## Review checklist

- [ ] Workflows repo groups all `github-actions` with `*`
- [ ] App repos split `workflows-toolbox` from third-party pins
- [ ] One `gomod` / `docker` / `terraform` entry per root directory
