# DenseVoid Workflows

A collection of reusable GitHub Actions workflows and composite actions for common development tasks.

## Workflows

### Go Checks (`.github/workflows/go-checks.yml`)

A comprehensive Go code quality workflow that performs multiple checks in parallel.

**Features:**
- **Vet**: Basic Go code analysis using `go vet`
- **Static Analysis**: Advanced static analysis using `staticcheck`
- **Linting**: Code linting using `golangci-lint`
- **Vulnerability Check**: Security vulnerability scanning using `govulncheck`
- **Security Check**: Security analysis using `gosec`

**Parameters:**
- `go-version` - Explicit Go version (optional; overrides `go-version-file` when set)
- `go-version-file` - Path to `go.mod`, `go.work`, etc. (optional; default: `{working-directory}/go.mod`)
- `working-directory` - Working directory for the Go project (optional, default: `.`)

When neither `go-version` nor `go-version-file` is set, the Go toolchain is read from `go.mod` in the working directory.

**Secrets:**
- `github-token` - GitHub token for accessing private repositories (optional, default: `${{ secrets.GITHUB_TOKEN }}`)

**Usage:**
```yaml
jobs:
  go-checks:
    uses: densestvoid/workflows/.github/workflows/go-checks.yml@main
    with:
      working-directory: './my-go-project'
    # Or pin explicitly: go-version: '1.21'
    secrets:
      github-token: ${{ secrets.GITHUB_TOKEN }}
```

**Requirements:**
- Go modules enabled in your project
- `go.mod` file in the working directory
- Appropriate permissions for the workflow to run

## Actions

### Setup (`.github/actions/setup`)

Sets up a Go environment with checkout and Go installation.

**Inputs:**
- `go-version` - Explicit Go version (optional; overrides `go-version-file` when set)
- `go-version-file` - Path to version file (optional; default: `{working-directory}/go.mod`)
- `working-directory` - Go project root relative to repo root (optional, default: `.`)

**Usage:**
```yaml
- name: Setup Go Environment
  uses: densestvoid/workflows/.github/actions/setup@main
  with:
    working-directory: '.'
```

### Install Tool (`.github/actions/install-tool`)

Installs and caches a Go tool with specified version.

**Inputs:**
- `tool-package` - Go package path for the tool (required)
- `tool-version` - Version of the tool to install (optional, default: `latest`)

**Usage:**
```yaml
- name: Install golangci-lint
  uses: densestvoid/workflows/.github/actions/install-tool@main
  with:
    tool-package: github.com/golangci/golangci-lint/cmd/golangci-lint
    tool-version: 'v1.54.2'  # or 'latest' for latest version
```

## Repository Structure

```
densestvoid/workflows/
├── .github/
│   ├── workflows/      # Reusable workflows
│   │   └── go-checks.yml
│   └── actions/        # Composite actions
│       ├── setup/
│       │   └── action.yml
│       └── install-tool/
│           └── action.yml
└── README.md          # This file
```

## Contributing

This repository is designed as a workflow library. Workflows and actions are organized in the `.github` directory following GitHub's standard conventions for reusable workflows and composite actions.