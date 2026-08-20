---
name: go-toolchain-setup
description: >-
  Configures Go in GitHub Actions with actions/checkout and actions/setup-go for
  densestvoid/workflows. Use when authoring or editing go-checks.yml, adding Go
  jobs to app repos, handling nested go.mod via working-directory, choosing
  golangci-lint/govulncheck/gosec/staticcheck, or replacing the removed setup-go
  composite.
---

# Go Toolchain Setup

## When to read this skill

- Adding or editing `.github/workflows/go-checks.yml`
- Any Go job that needs checkout + toolchain (app repo or this repo)
- Nested `go.mod` / `working-directory` inputs
- Choosing which linter/vuln/security action to use

## Job step order

Every Go check job follows this order:

1. `- name: Checkout` → `actions/checkout@v7`
2. `- name: Setup Go` → `actions/setup-go@v7` (from `go.mod` only — no explicit `go-version` input)
3. Tool step (run, maintainer action, or `./.github/actions/install-go-tool`)

## Path resolution (reusable workflows)

Centralize fallbacks in workflow `env`; document in the workflow header. Canonical: `.github/workflows/go-checks.yml`.

```yaml
env:
  GO_VERSION_FILE: >-
    ${{ inputs.go-version-file != '' && inputs.go-version-file
      || (inputs.working-directory == '.' && 'go.mod'
        || format('{0}/go.mod', inputs.working-directory)) }}
  GO_CACHE_DEPENDENCY_PATH: >-
    ${{ inputs.working-directory == '.' && 'go.sum'
      || format('{0}/go.sum', inputs.working-directory) }}

- name: Setup Go
  uses: actions/setup-go@v7
  with:
    go-version-file: ${{ env.GO_VERSION_FILE }}
    cache-dependency-path: ${{ env.GO_CACHE_DEPENDENCY_PATH }}
```

Replace `inputs.*` with literals in non-reusable jobs.

## working-directory by tool type

| Tool | How to scope |
|------|--------------|
| `go vet`, `staticcheck` | `defaults.run.working-directory` on job + `./...` in command |
| `golangci-lint-action` | `with.working-directory` |
| `govulncheck-action` | `with.work-dir` |
| `securego/gosec@v2` | Repo-root-relative `args` only (Docker action; no `working-directory` input) — see Security-Check job in go-checks |

## Tool choice

| Tool | Action |
|------|--------|
| golangci-lint | `golangci/golangci-lint-action@v6` |
| govulncheck | `golangci/govulncheck-action@v1` |
| gosec | `securego/gosec@v2` |
| staticcheck | `./.github/actions/install-go-tool` then `run: staticcheck ./...` |

Do not ad-hoc `go install` per job when **install-go-tool** or a maintainer action exists.

## Anti-patterns

- Plain `- uses: actions/checkout@v7` without `- name: Checkout`
- Dual Setup Go steps (explicit version + go-version-file)
- `densestvoid/workflows/...@main` for **install-go-tool** inside this repo — use `./.github/actions/install-go-tool`
- Putting gosec scan paths in shared workflow `env` — keep inline at the gosec step with a comment explaining why
