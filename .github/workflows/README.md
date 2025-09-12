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

| Parameter | Description | Required | Default | Type |
|-----------|-------------|----------|---------|------|
| `go-version` | Go version to use for the checks | No | `stable` | string |
| `working-directory` | Working directory for the Go project | No | `.` | string |

## Secrets

| Secret | Description | Required | Default |
|--------|-------------|----------|---------|
| `github-token` | GitHub token for accessing private repositories | No | `${{ secrets.GITHUB_TOKEN }}` |

## Requirements

- Go modules enabled in your project
- `go.mod` file in the working directory
- Appropriate permissions for the workflow to run

## Outputs

This workflow doesn't produce outputs, but it will fail if any of the checks fail, making it suitable for CI/CD pipelines where you want to ensure code quality.

## Troubleshooting

### Common Issues

1. **Permission Denied**: Ensure the workflow has the necessary permissions to access your repository
2. **Go Version Not Found**: Make sure the specified Go version is valid and available
3. **Working Directory Not Found**: Verify the working directory path is correct relative to your repository root
4. **Tool Installation Fails**: This usually indicates network issues or Go module proxy problems

### Debug Mode

To enable debug logging, add the following to your calling workflow:

```yaml
env:
  ACTIONS_STEP_DEBUG: true
```