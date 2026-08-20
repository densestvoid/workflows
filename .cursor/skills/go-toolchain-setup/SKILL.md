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

## Version pins

When writing workflow YAML, look up and pin the **latest release tag** for every third-party action (checkout, setup-go, golangci-lint, govulncheck, gosec, paths-filter, etc.). Do not copy stale `@vN` examples from docs or this skill — resolve current latest at authoring time. Dependabot can bump pins afterward.

## Job step order

1. `- name: Checkout` → `actions/checkout` (latest release)
2. `- name: Setup Go` → `actions/setup-go` (latest release; from `go.mod` only)
3. Tool step (run, maintainer action, or `$/.github/actions/install-go-tool`)

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
| `securego/gosec` | `with.args` — repo-root package path (e.g. `./backend/...`) |

## Tool choice

| Tool | Action |
|------|--------|
| golangci-lint | `golangci/golangci-lint-action` — set `install-mode: goinstall` so lint is built with the `setup-go` toolchain (needed when consumer `go.mod` targets a newer Go than the action’s prebuilt binary) |
| govulncheck | `golang/govulncheck-action` |
| gosec | `securego/gosec` |
| staticcheck | `$/.github/actions/install-go-tool` |

Use maintainer actions or **install-go-tool** — do not ad-hoc `go install` per job.

## Anti-patterns

- `- uses: actions/checkout` without `- name: Checkout`
- Dual Setup Go steps (explicit version + go-version-file)
- `densestvoid/workflows/...@main` for **install-go-tool** inside this repo
- `./.github/actions/...` for same-repo actions — resolves in the workspace (and in reusable workflows, the **caller** checkout); use `$/.github/actions/...` instead
