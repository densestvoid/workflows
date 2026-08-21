---
name: write-github-workflows
description: >-
  Authors and reviews GitHub Actions workflows and composite actions for the
  densestvoid/workflows toolbox. Use when adding or changing deploy workflows,
  .github/actions, README deploy examples, or reviewing PRs in this repo — not
  for CI layout or Dependabot (see routed skills).
---

# Write GitHub Workflows

## Skill routing

| Task | Skill |
|------|-------|
| CI layout, ci.yml, branch protection | [ci-playbooks](ci-playbooks/SKILL.md) |
| Dependabot config and PR review | [dependabot-workflows](dependabot-workflows/SKILL.md) |
| Go jobs, go-checks, nested go.mod | [go-toolchain-setup](go-toolchain-setup/SKILL.md) |
| Read Terraform output after deploy | [terraform-output-inline](terraform-output-inline/SKILL.md) |
| Deploy/build actions, toolbox authoring | This skill |

## Version pins

When authoring or editing workflow YAML, pin the **latest release tag** for every third-party action. Do not hardcode stale major-only pins from examples; look up current latest at authoring time. Dependabot bumps third-party `uses:` tags afterward.

**actionlint binary:** the **actionlint** composite downloads via the upstream script from `rhysd/actionlint` `main` (latest release). Do not pin the binary version — Dependabot cannot maintain a curl/script pin.

## Internal action refs (this repo only)

| Context | Pattern |
|---------|---------|
| Caller repo invokes toolbox | `densestvoid/workflows/.github/workflows/go-checks.yml@<tag>` or `densestvoid/workflows/.github/actions/<name>@<tag>` |
| This repo invokes sibling action / workflow | `$/.github/actions/<name>` or `$/.github/workflows/<file>` |

`$/` resolves to the repository of the running workflow/action at that commit (requires runner ≥ 2.336.0). No checkout needed to resolve the action. Do **not** use `./.github/actions/...` — that path resolves in the **workspace** checkout (and in reusable workflows, the **caller** checkout), so it breaks for consumer repos and needs a prior checkout even in this repo.

## Design principles

1. **Toolbox, not orchestration** — app repos own deploy triggers and job graphs; CI orchestration lives in each repo's [ci.yml](ci-playbooks/SKILL.md)
2. **Deploy skip in caller** — `dorny/paths-filter` (latest release) in deploy workflow `if:`, not trigger `paths` when required checks must still run
3. **Atomic actions** — one concern per composite; delegate to official/maintainer actions inside
4. **TF_VAR_*** — Terraform module variables via env on the invoking step
5. **Encapsulated internals** — cache keys and registry inspect logic stay inside actions

## Default choices

| Need | Use | Avoid |
|------|-----|-------|
| CI layout | Repo **ci.yml** + **ci-aggregate** — see [ci-playbooks](ci-playbooks/SKILL.md) | Local `workflow_call` CI composables; hand-rolled gate expressions |
| Path skip gates (deploy) | `dorny/paths-filter` (latest release) | Custom git diff; trigger `paths` blocking required CI |
| Workflow / action YAML lint | **actionlint** composite | Manual curl in every app; Marketplace actionlint wrappers |
| Docker build + push | **build-docker** | Raw buildx/login/metadata chain in callers |
| Go toolchain | checkout + setup-go | Removed **setup-go** composite |
| Go checks | Reusable **go-checks.yml** | Duplicating five parallel jobs |
| Terraform output (same job) | Inline `terraform output -raw` | Removed **terraform-output** |
| Internal same-repo refs | `$/.github/actions/...` | `./.github/actions/...` (workspace/caller checkout); `@main` self-references in this repo |
| Slack + PR notify | **notify** | Duplicate slack + github-script |
| Deploy / destroy | **deploy-terraform** / **terminate-terraform** | Inline init/apply/s3 rm |

## Toolbox actions (v1 pin in callers)

| Action | Purpose |
|--------|---------|
| **build-go** | One binary + artifact |
| **build-docker** | GHCR push; optional Docker Hub; `image-built` when tag exists |
| **deploy-terraform** | init + apply |
| **terminate-terraform** | destroy module + S3 state delete |
| **notify** | Slack + PR comment |
| **install-go-tool** | Cached `go install` |
| **actionlint** | Download rhysd/actionlint + lint workflows |
| **ci-aggregate** | Fail **`ci`** when any needed job failed/cancelled |
| **go-checks.yml** | Parallel vet/staticcheck/lint/vuln/gosec |

## Documentation conventions

- Action README sections describe that action only — chain composition lives in the deploy pipeline example
- Inline comments explain non-obvious bash or `${{ }}` syntax, not other actions
- Reusable workflow headers document expression syntax used in that file

## Review checklist

- [ ] CI layout follows [ci-playbooks](ci-playbooks/SKILL.md)
- [ ] Dependabot follows [dependabot-workflows](dependabot-workflows/SKILL.md)
- [ ] Deploy skip gates use `dorny/paths-filter` (latest release)
- [ ] Go jobs: named Checkout + Setup Go from go-version-file only
- [ ] Third-party actions pin latest release tags (not stale majors / invalid bare tags)
- [ ] Same-repo sibling refs use `$/.github/actions/...` (not `./`)
- [ ] Comments and docs stay scoped to the file they are in
- [ ] README example matches current action inputs
