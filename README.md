# DenseVoid Workflows

A collection of reusable GitHub Actions workflows and composite actions for common development tasks.

## Workflows

### Go Checks (`workflows/go-checks.yml`)

A comprehensive Go code quality workflow that performs multiple checks in parallel.

**Features:**
- **Vet**: Basic Go code analysis using `go vet`
- **Static Analysis**: Advanced static analysis using `staticcheck`
- **Linting**: Code linting using `golangci-lint`
- **Vulnerability Check**: Security vulnerability scanning using `govulncheck`
- **Security Check**: Security analysis using `gosec`

**Parameters:**
- `go-version` - Go version to use for the checks (optional, default: `stable`)
- `working-directory` - Working directory for the Go project (optional, default: `.`)

**Secrets:**
- `github-token` - GitHub token for accessing private repositories (optional, default: `${{ secrets.GITHUB_TOKEN }}`)

**Usage:**
```yaml
jobs:
  go-checks:
    uses: densestvoid/workflows/workflows/go-checks.yml@main
    with:
      go-version: '1.21'
      working-directory: './my-go-project'
    secrets:
      github-token: ${{ secrets.GITHUB_TOKEN }}
```

**Requirements:**
- Go modules enabled in your project
- `go.mod` file in the working directory
- Appropriate permissions for the workflow to run

## Actions

### Setup (`actions/setup`)

Sets up a Go environment with checkout and Go installation.

**Inputs:**
- `go-version` - Go version to use (optional, default: `stable`)

**Usage:**
```yaml
- name: Setup Go Environment
  uses: densestvoid/workflows/actions/setup@main
  with:
    go-version: '1.21'
```

### Install Tool (`actions/install-tool`)

Installs and caches a Go tool with specified version.

**Inputs:**
- `tool-package` - Go package path for the tool (required)
- `tool-version` - Version of the tool to install (optional, default: `latest`)

**Usage:**
```yaml
- name: Install golangci-lint
  uses: densestvoid/workflows/actions/install-tool@main
  with:
    tool-package: github.com/golangci/golangci-lint/cmd/golangci-lint
    tool-version: 'v1.54.2'  # or 'latest' for latest version
```

## Repository Structure

```
densestvoid/workflows/
├── workflows/           # Reusable workflows
│   └── go-checks.yml
├── actions/            # Composite actions
│   ├── setup/
│   │   └── action.yml
│   └── install-tool/
│       └── action.yml
└── README.md          # This file
```

## Contributing

This repository is designed as a workflow library. Workflows and actions are organized in separate directories for better version control and reusability across different repositories.