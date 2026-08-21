---
name: ci-playbooks
description: >-
  Prescribes repo-specific composable CI workflows — gating, parallel checks, no
  gate job; official ci.yml calls one local workflow so branch protection requires
  job ci only. Use when authoring ci.yml, ci-checks.yml, branch protection, or
  reviewing CI in any repo.
---

# CI Playbooks

## Playbook

Each repo owns two workflow files:

| File | Role |
|------|------|
| **`ci.yml`** | Triggers only (`pull_request`, `push` to `main`) + **one job** `ci` |
| **`ci-checks.yml`** | Repo-specific composable (`workflow_call`) — path filters + parallel conditional check jobs |

Official `ci.yml`:

```yaml
name: CI
on:
  pull_request:
  push:
    branches: [main]

jobs:
  ci:
    uses: $/.github/workflows/ci-checks.yml
```

Composable `ci-checks.yml` is **not** shared from the toolbox. List only checks this repo needs; compose from toolbox atoms (`actionlint`, `go-checks.yml`, …).

## Branch protection

Require job **`ci`** only (the caller job name). Failures in composable check jobs propagate to **`ci`**. Do not require sub-job names or a synthetic gate job.

## Composable layout

1. **`changes`** — checkout + `dorny/paths-filter`; job `outputs` re-export each filter name (required — step outputs are not visible across jobs)
2. **Check jobs** — `needs: changes`; `if: needs.changes.outputs.<filter> == 'true'` (paths-filter emits string `'true'` / `'false'`); call toolbox atoms
3. **No gate job** — no `always()` aggregation, no `echo "CI passed"`

This repo uses one check job; app repos add more parallel jobs (go-checks, …) sharing the same `changes` job.

Pass/fail: failing check → composable run fails → caller **`ci`** fails. Docs-only PR: check jobs skipped, **`ci`** green.

## This repo (workflows toolbox)

Same split layout as app repos — `changes` + `actionlint` only. See [`.github/workflows/ci-checks.yml`](../../.github/workflows/ci-checks.yml).

## App repo example

```yaml
# ci-checks.yml (in app repo)
jobs:
  changes:
    runs-on: ubuntu-latest
    outputs:
      go: ${{ steps.filter.outputs.go }}
      workflows: ${{ steps.filter.outputs.workflows }}
    steps:
      - uses: actions/checkout@<latest>
        with:
          fetch-depth: 0
      - uses: dorny/paths-filter@<latest>
        id: filter
        with:
          filters: |
            go:
              - '**/*.go'
              - 'go.mod'
              - 'go.sum'
            workflows:
              - '.github/workflows/**'
              - '.github/actions/**'

  actionlint:
    needs: changes
    if: needs.changes.outputs.workflows == 'true'
    runs-on: ubuntu-latest
    steps:
      - uses: densestvoid/workflows/.github/actions/actionlint@<toolbox-pin>

  go-checks:
    needs: changes
    if: needs.changes.outputs.go == 'true'
    uses: densestvoid/workflows/.github/workflows/go-checks.yml@<toolbox-pin>
```

## Anti-patterns

- Shared CI workflow or action published from toolbox for all repos
- Internal or caller gate job (`always()` + `echo "CI passed"`)
- Orchestration in official `ci.yml` (`changes`, conditional sub-jobs)
- `./.github/...` same-repo refs — use `$/.github/...`
- Trigger `paths` when required CI must still report
- Duplicating check steps in official `ci.yml`

## Review checklist

- [ ] Official `ci.yml` is triggers + one job `ci` → `$/.github/workflows/ci-checks.yml`
- [ ] Composable lists only this repo's checks; no gate job
- [ ] Path filters in composable `changes`, not trigger `paths`
- [ ] Branch protection requires **`ci`** only
- [ ] Toolbox atoms pinned at `<toolbox-pin>` in app composables
