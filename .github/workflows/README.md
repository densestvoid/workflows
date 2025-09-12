# Go Checks Reusable Workflow

This repository contains a reusable GitHub Actions workflow for comprehensive Go code quality checks.

## Features

The `go-checks.yml` workflow performs the following checks:

- **Vet**: Basic Go code analysis using `go vet`
- **Static Analysis**: Advanced static analysis using `staticcheck`
- **Linting**: Code linting using `golangci-lint`
- **Vulnerability Check**: Security vulnerability scanning using `govulncheck`
- **Security Check**: Security analysis using `gosec`

## Usage

### Basic Usage

To use this workflow in your repository, create a workflow file (e.g., `.github/workflows/ci.yml`) with the following content:

```yaml
name: CI

on:
  push:
    branches: [ main, develop ]
  pull_request:
    branches: [ main ]

jobs:
  go-checks:
    uses: your-username/your-repo/.github/workflows/go-checks.yml@main
```

### Advanced Usage with Custom Parameters

```yaml
name: CI

on:
  push:
    branches: [ main, develop ]
  pull_request:
    branches: [ main ]

jobs:
  go-checks:
    uses: your-username/your-repo/.github/workflows/go-checks.yml@main
    with:
      go-version: '1.21'
      working-directory: './cmd/myapp'
    secrets:
      github-token: ${{ secrets.GITHUB_TOKEN }}
```

### Using with Different Go Versions

```yaml
jobs:
  go-checks-stable:
    uses: your-username/your-repo/.github/workflows/go-checks.yml@main
    with:
      go-version: 'stable'
      
  go-checks-121:
    uses: your-username/your-repo/.github/workflows/go-checks.yml@main
    with:
      go-version: '1.21'
      
  go-checks-120:
    uses: your-username/your-repo/.github/workflows/go-checks.yml@main
    with:
      go-version: '1.20'
```

### Using with Monorepo Structure

```yaml
jobs:
  go-checks-api:
    uses: your-username/your-repo/.github/workflows/go-checks.yml@main
    with:
      working-directory: './services/api'
      
  go-checks-worker:
    uses: your-username/your-repo/.github/workflows/go-checks.yml@main
    with:
      working-directory: './services/worker'
```

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

## Examples

### Simple Go Project

```yaml
name: Go CI

on:
  push:
    branches: [ main ]
  pull_request:
    branches: [ main ]

jobs:
  quality-checks:
    uses: your-username/your-repo/.github/workflows/go-checks.yml@main
```

### Multi-Service Go Project

```yaml
name: Multi-Service CI

on:
  push:
    branches: [ main ]
  pull_request:
    branches: [ main ]

jobs:
  api-checks:
    uses: your-username/your-repo/.github/workflows/go-checks.yml@main
    with:
      working-directory: './api'
      
  worker-checks:
    uses: your-username/your-repo/.github/workflows/go-checks.yml@main
    with:
      working-directory: './worker'
      
  cli-checks:
    uses: your-username/your-repo/.github/workflows/go-checks.yml@main
    with:
      working-directory: './cli'
```

## Troubleshooting

### Common Issues

1. **Permission Denied**: Ensure the workflow has the necessary permissions to access your repository
2. **Go Version Not Found**: Make sure the specified Go version is valid and available
3. **Working Directory Not Found**: Verify the working directory path is correct relative to your repository root
4. **Tool Installation Fails**: This usually indicates network issues or Go module proxy problems

### Debug Mode

To enable debug logging, you can add the following to your calling workflow:

```yaml
jobs:
  go-checks:
    uses: your-username/your-repo/.github/workflows/go-checks.yml@main
    with:
      go-version: '1.21'
    env:
      ACTIONS_STEP_DEBUG: true
```

## Contributing

To contribute to this workflow:

1. Fork the repository
2. Make your changes
3. Test the workflow in your fork
4. Submit a pull request

## License

This workflow is provided as-is under the same license as the repository it's hosted in.