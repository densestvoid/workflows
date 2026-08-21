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
- Choosing how to install linter/vuln/security tools

## Version pins

When writing workflow YAML, look up and pin the **latest release tag** for every third-party action (checkout, setup-go, paths-filter, etc.). Do not copy stale `@vN` examples from docs or this skill — resolve current latest at authoring time. Dependabot can bump pins afterward.

Go CLI tools (staticcheck, golangci-lint, govulncheck, gosec) are **not** pinned in this toolbox — consumers declare them in a `go.mod` `tool` block; **install-go-tool** runs bare `go install <pkg>` so module-selected versions apply.

## Job step order

1. `- name: Checkout` → `actions/checkout` (latest release)
2. `- name: Setup Go` → `actions/setup-go` (latest release; from `go.mod` only)
3. Tool step → `$/.github/actions/install-go-tool` then `run:` the binary

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
| `go vet`, `staticcheck`, `golangci-lint`, `govulncheck`, `gosec` | `defaults.run.working-directory` on the job |
| `install-go-tool` | `with.working-directory` + `with.cache-dependency-path` (pass workflow env paths) |

## Tool choice

| Tool | Action |
|------|--------|
| staticcheck | `$/.github/actions/install-go-tool` + `staticcheck ./...` |
| golangci-lint | `$/.github/actions/install-go-tool` + `golangci-lint run ./...` |
| govulncheck | `$/.github/actions/install-go-tool` + `govulncheck ./...` |
| gosec | `$/.github/actions/install-go-tool` + `gosec ./...` |

Use **install-go-tool** — do not ad-hoc `go install` per job, and do not use maintainer actions that float tool versions (`@latest` / unpinned goinstall).

## Anti-patterns

- `- uses: actions/checkout` without `- name: Checkout`
- Dual Setup Go steps (explicit version + go-version-file)
- `densestvoid/workflows/...@main` for **install-go-tool** inside this repo
- `./.github/actions/...` for same-repo actions — resolves in the workspace (and in reusable workflows, the **caller** checkout); use `$/.github/actions/...` instead
- `go install <pkg>@latest` (or any `@version` in **install-go-tool**) — ignores the consumer `tool` block
- `golangci-lint-action` / `govulncheck-action` / `securego/gosec` for go-checks — they do not read the consumer `tool` block
