---
name: go-toolchain-setup
description: >-
  Configures Go in GitHub Actions with actions/checkout and actions/setup-go for
  densestvoid/workflows. Use when authoring or editing go-checks.yml, adding Go
  jobs to app repos, handling nested go.mod via working-directory, or choosing
  golangci-lint/govulncheck/gosec/staticcheck.
---

# Go Toolchain Setup

## When to read this skill

- Adding or editing `.github/workflows/go-checks.yml`
- Any Go job that needs checkout + toolchain
- Nested `go.mod` / `working-directory` inputs
- Choosing which linter/vuln/security action to use

## Job step order

1. `- name: Checkout` → `actions/checkout@v7`
2. `- name: Setup Go` → `actions/setup-go@v7` (from `go.mod` only)
3. Tool step (run, maintainer action, or `./.github/actions/install-go-tool`)

## Path resolution (reusable workflows)

Centralize module path fallbacks in workflow `env`. Canonical: `.github/workflows/go-checks.yml`.

**Expression syntax**

| Pattern | Meaning |
|---------|---------|
| `${{ A && B \|\| C }}` | if A is truthy then B, else C |
| `${{ format('{0}/go.mod', dir) }}` | interpolate `dir` into a path |
| `>-` folded scalar | YAML folds newlines to spaces for long expressions |

```yaml
env:
  GO_VERSION_FILE: >-
    ${{ inputs.go-version-file != '' && inputs.go-version-file
      || (inputs.working-directory == '.' && 'go.mod'
        || format('{0}/go.mod', inputs.working-directory)) }}
  GO_CACHE_DEPENDENCY_PATH: >-
    ${{ inputs.working-directory == '.' && 'go.sum'
      || format('{0}/go.sum', inputs.working-directory) }}
```

## Scoping checks to a module

| Tool | Input |
|------|-------|
| `go vet`, `staticcheck` | `defaults.run.working-directory` on the job |
| `golangci-lint-action` | `with.working-directory` |
| `govulncheck-action` | `with.work-dir` |
| `securego/gosec@v2` | `with.args` — repo-root package path (e.g. `./backend/...`) |

## Tool choice

| Tool | Action |
|------|--------|
| golangci-lint | `golangci/golangci-lint-action@v6` |
| govulncheck | `golangci/govulncheck-action@v1` |
| gosec | `securego/gosec@v2` |
| staticcheck | `./.github/actions/install-go-tool` |

Use maintainer actions or **install-go-tool** — do not ad-hoc `go install` per job.

## Anti-patterns

- `- uses: actions/checkout@v7` without `- name: Checkout`
- Dual Setup Go steps (explicit version + go-version-file)
- `densestvoid/workflows/...@main` for **install-go-tool** inside this repo
