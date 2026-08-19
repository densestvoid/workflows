---
name: write-github-workflows
description: >-
  Author and review GitHub Actions workflows and composite actions for the
  densestvoid/workflows toolbox. Use when adding or changing workflows, composite
  actions, or caller examples. Strongly favor officially supported third-party
  actions and this repo's composable actions over custom shell scripts and direct
  API/curl calls.
---
# Write GitHub Workflows

Use this skill when authoring workflows in app repos or composite actions in
`densestvoid/workflows`. Prefer small, composable steps and maintained actions
over bespoke bash.

## Default choices

| Need | Use | Avoid |
|------|-----|-------|
| Path change detection (in-job skip) | `dorny/paths-filter@v3` | Custom git diff/bash; workflow trigger `paths` when a required check must still run |
| Slack incoming webhook | `slackapi/slack-github-action@v4.0.0` (`webhook-type: incoming-webhook`) | `curl` to webhook URL |
| PR comment | `peter-evans/create-or-update-comment@v4` or `actions/github-script` | Raw `curl` + `GITHUB_TOKEN` |
| Terraform init/apply/output | `hashicorp/setup-terraform@v4` + `dflook/terraform-*` or inline `terraform` | Ad-hoc install scripts |
| Docker build + push | `docker/build-push-action@v6` + `docker/login-action@v4` | Custom `docker buildx` bash |
| AWS credentials | `aws-actions/configure-aws-credentials@v4` | Passing raw keys in bash when avoidable |
| Checkout | `actions/checkout@v7` | Assumed pre-checked-out workspace |
| Cache | `actions/cache/restore` + `actions/cache/save` (@v6) | Custom cache dirs without actions/cache |
| Artifact upload/download | `actions/upload-artifact` / `actions/download-artifact` | Custom artifact storage |
| Go toolchain | `actions/setup-go@v7` (optionally via `setup-go` wrapper) | Manual Go install |
| Go lint / vuln / security | `golangci/golangci-lint-action`, `golangci/govulncheck-action`, `securego/gosec` | Ad-hoc `go install` per job unless `install-go-tool` is intentional |
| Deploy / terminate Terraform | `densestvoid/workflows` `deploy-terraform` / `terminate-terraform` | Inline `terraform` + `aws` scripts in caller workflows |
| Terraform variables | `TF_VAR_*` env on the step that uses the action | Custom tfvars files in the action unless unavoidable |
| Working directory | `working-directory:` on `run` steps | `cd` in bash |
| Internal action state | `GITHUB_ENV` | Step `GITHUB_OUTPUT` unless exposing a step output |
| Caller-visible action results | `outputs:` in `action.yml` wired from step outputs or `env` | Undocumented env leakage |

## Slack payloads

`slack-github-action` accepts inline **`payload`** JSON (text, attachments, blocks). Pass it as **`slack-payload`** to **notify**. Budget builds rich payloads with `jq` in the caller workflow, then passes the JSON string inline (or via a workflow env var).

## This repo's toolbox actions

Compose caller workflows from atomic actions (see root `README.md`):

- `build-go`, `build-docker`, `push-container`
- `deploy-terraform`, `terminate-terraform`, `terraform-output`
- `notify` — Slack (`slack-payload` JSON) + PR comment
- `setup-go`, `install-go-tool`, reusable `go-checks.yml`

**Skip logic** lives in the caller workflow via `dorny/paths-filter` (not a toolbox action).

Callers pass secrets via `with:` for AWS/backend inputs and `env:` for `TF_VAR_*`.

## Composite action conventions

1. **One concern per action** — deploy does init+apply; notify delivers; skip logic stays caller-owned.
2. **Split steps for clarity** — separate init, apply, delete state, etc. for log visibility.
3. **Inputs over env for action API** — except `TF_VAR_*` and values callers already set on the invoking step.
4. **Document non-obvious behavior** in `description` and README caller notes.
5. **Pin major versions** on third-party actions (`@v6` cache, `@v7` checkout/setup-go, etc.) consistent with existing actions in this repo.

## When shell is acceptable

- Glue that has no maintained action (e.g. `aws s3 rm` after Terraform — Terraform does not delete remote state objects).
- `go run` for bundled helpers (`build-go/cachekey`).
- Resolution logic that cannot be expressed in `if:` (rare — prefer `if:` + inputs first).

## Review checklist

Before merging workflow or action changes:

- [ ] No `curl` to GitHub, Slack, or Docker APIs where an official action exists
- [ ] Path skip gates use `dorny/paths-filter`, not custom diff scripts
- [ ] Secrets only in `secrets.*` or masked inputs — never logged
- [ ] `if: always()` only where delivery must run after failure (e.g. notify)
- [ ] Composite `outputs:` declared only for caller-facing values
- [ ] README example updated when action inputs change
