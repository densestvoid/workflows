# Go Checks Reusable Workflow

A reusable GitHub Actions workflow for comprehensive Go code quality checks.

## Features

The `go-checks.yml` workflow performs the following checks:

- **Vet**: Basic Go code analysis using `go vet`
- **Static Analysis**: Advanced static analysis using `staticcheck`
- **Linting**: Code linting using `golangci-lint`
- **Vulnerability Check**: Security vulnerability scanning using `govulncheck`
- **Security Check**: Security analysis using `gosec`

## Parameters

- `go-version` - Go version to use for the checks (optional, default: `stable`)
- `working-directory` - Working directory for the Go project (optional, default: `.`)

## Secrets

- `github-token` - GitHub token for accessing private repositories (optional, default: `${{ secrets.GITHUB_TOKEN }}`)

## Requirements

- Go modules enabled in your project
- `go.mod` file in the working directory
- Appropriate permissions for the workflow to run