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
| Docker build + push | `densestvoid/workflows` **build-docker** | Raw action chaining in callers; tar artifact round-trip |
| Go checkout + toolchain | `actions/checkout@v7` + `actions/setup-go@v7` | Removed `setup-go` composite — see **go-toolchain-setup** skill |
| Go lint / vuln / security | Maintainer actions in **go-checks**; `install-go-tool` for staticcheck | Ad-hoc `go install` per job |
| Terraform output (same job) | Inline `terraform output -raw` — see **terraform-output-inline** skill | Removed `terraform-output` composite |
| Slack + PR notify | **notify** composite | Duplicating slack + github-script in every app |
| Deploy / terminate Terraform | **deploy-terraform** / **terminate-terraform** | Inline init/apply/s3 rm in callers |
| Terraform variables | `TF_VAR_*` env on the invoking step | Custom tfvars in actions unless unavoidable |
| Checkout | `actions/checkout@v7` | Assumed pre-checked-out workspace |
| Cache | `actions/cache/restore` + `actions/cache/save` (@v6) | Custom cache dirs |
| Internal action state | `GITHUB_ENV` | Undocumented env leakage |
| Caller-visible results | `outputs:` in `action.yml` | Step outputs not wired to action outputs |

## This repo's toolbox actions

- **build-go** — Go binary + dep-tree `cache-key` (binary cache only; not Docker tags)
- **build-docker** — Docker build/push via official actions; `image-built` for deploy gating
- **deploy-terraform**, **terminate-terraform**
- **notify** — Slack (`slack-payload`) + PR comment
- **install-go-tool** — generic cached `go install`
- Reusable **go-checks.yml**

**Skip logic** lives in caller workflows via `dorny/paths-filter`.

## Related skills

- [go-toolchain-setup](go-toolchain-setup/SKILL.md) — checkout + setup-go; maintainer lint actions
- [terraform-output-inline](terraform-output-inline/SKILL.md) — read outputs after deploy

## Composite action conventions

1. **One concern per action** — skip logic stays caller-owned.
2. **Delegate to official actions inside composites** — e.g. build-docker wraps setup-buildx, login, metadata, build-push.
3. **Split steps for log visibility.**
4. **Document non-obvious behavior** in README caller notes.

## Review checklist

- [ ] Path skip gates use `dorny/paths-filter`
- [ ] Docker uses **build-docker**, not raw buildx bash in callers
- [ ] Go jobs use checkout + setup-go (not removed setup-go)
- [ ] README example updated when action inputs change
