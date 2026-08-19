---
name: go-toolchain-setup
description: >-
  Sets up Go in GitHub Actions workflows using actions/checkout and
  actions/setup-go. Use when authoring Go jobs after setup-go was removed from
  densestvoid/workflows, or when configuring go-checks-style jobs with
  working-directory and nested go.mod paths.
---

# Go Toolchain Setup

Use when a workflow job needs Go but should not call the removed `setup-go` composite.

## Standard pattern

```yaml
- name: Checkout
  uses: actions/checkout@v7

- name: Setup Go
  uses: actions/setup-go@v7
  with:
    go-version-file: >-
      ${{ inputs.go-version-file != '' && inputs.go-version-file
        || (inputs.working-directory == '.' && 'go.mod'
          || format('{0}/go.mod', inputs.working-directory)) }}
    cache-dependency-path: >-
      ${{ inputs.working-directory == '.' && 'go.sum'
        || format('{0}/go.sum', inputs.working-directory) }}
```

Resolve version from `go.mod` (or `go-version-file` / `working-directory` inputs). Do not add a separate explicit `go-version` override step.

Replace `inputs.*` with literals when the job is not a reusable workflow.

## Tool choice

| Tool | Use |
|------|-----|
| `golangci/golangci-lint-action@v6` | golangci-lint (maintainer action) |
| `golangci/govulncheck-action@v1` | govulncheck |
| `securego/gosec@v2` | gosec (`args: ./...` or `./{dir}/...`) |
| `densestvoid/workflows/.github/actions/install-go-tool` | staticcheck and other tools without maintainer actions |

Run checks with `working-directory:` on `run` steps, or pass `working-directory` / `work-dir` to maintainer actions.

## Reference

See `.github/workflows/go-checks.yml` in the workflows repo for the canonical multi-job layout.
