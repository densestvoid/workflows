---
name: ci-playbooks
description: >-
  Prescribes repo CI in one ci.yml — path filters, parallel conditional checks,
  alls-green aggregate gate; branch protection requires job ci only. Use when
  authoring ci.yml, branch protection, or reviewing CI in any repo.
---

# CI Playbooks

## Playbook

Each repo owns **`ci.yml`** — triggers, a **`changes`** job, conditional check jobs, and a final **`ci`** aggregate job.

| Job | Role |
|-----|------|
| **`changes`** | checkout + `dorny/paths-filter`; job `outputs` re-export each filter name |
| **Check jobs** | `needs: changes`; `if: needs.changes.outputs.<filter> == 'true'`; call toolbox atoms |
| **`ci`** | `needs: [changes, …]`; `if: always()`; **[re-actors/alls-green](https://github.com/re-actors/alls-green)** |

Do **not** split CI into a local `workflow_call` composable — reusable workflows only expose sub-job checks (`ci / changes`, …), not a single merge check named **`ci`**.

## Branch protection

Require job **`ci`** only. The aggregate job is a real top-level job; skipped check jobs do not block merge when listed in **`allowed-skips`**.

## Layout

1. **`changes`** — checkout + `dorny/paths-filter`; job `outputs` re-export each filter name (required — step outputs are not visible across jobs)
2. **Check jobs** — `needs: changes`; `if: needs.changes.outputs.<filter> == 'true'` (paths-filter emits string `'true'` / `'false'`); call toolbox atoms
3. **`ci`** — list every check job in `needs`; `if: always()`; one step: **alls-green** with `jobs: ${{ toJSON(needs) }}`

Pin the **latest release tag** of [re-actors/alls-green](https://github.com/re-actors/alls-green/releases) (Dependabot bumps `github-actions` afterward).

```yaml
  ci:
    needs: [changes, actionlint]
    if: always()
    runs-on: ubuntu-latest
    steps:
      - uses: re-actors/alls-green@v1.2.2
        with:
          jobs: ${{ toJSON(needs) }}
          allowed-skips: actionlint
```

**alls-green** reads the **`ci`** job `needs` context — fails on failure/cancelled; **`allowed-skips`** lists conditional check jobs that may be skipped (docs-only PRs). **`changes`** must always run and must not be in **`allowed-skips`**. Add new check jobs to **`ci`** `needs` and to **`allowed-skips`** when they use path-filter `if:`.

This repo: `changes` + `actionlint` only. See [`.github/workflows/ci.yml`](../../.github/workflows/ci.yml).

## App repo example

```yaml
# ci.yml (in app repo)
name: CI
on:
  pull_request:
  push:
    branches: [main]

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

  ci:
    needs: [changes, actionlint, go-checks]
    if: always()
    runs-on: ubuntu-latest
    steps:
      - uses: re-actors/alls-green@v1.2.2
        with:
          jobs: ${{ toJSON(needs) }}
          allowed-skips: actionlint, go-checks
```

## Anti-patterns

- Local **`ci-checks.yml`** `workflow_call` for merge gating (sub-jobs become separate required checks)
- Shared CI workflow published from toolbox for all repos
- Custom composite actions or bash loops to aggregate `needs` — use **alls-green**
- Hand-rolled `contains(needs.*.result, …)` when **alls-green** covers the case
- Omitting **`allowed-skips`** for path-filtered check jobs (skipped jobs fail the gate)
- Trigger `paths` when required CI must still report
- `./.github/...` same-repo refs — use `$/.github/...`

## Review checklist

- [ ] Single **`ci.yml`** with `changes`, conditional checks, and **`ci`** aggregate job
- [ ] **`ci`** `needs` lists every check job; **alls-green** with `jobs: ${{ toJSON(needs) }}`
- [ ] Conditional check jobs listed in **`allowed-skips`**
- [ ] Path filters in **`changes`**, not trigger `paths`
- [ ] Branch protection requires **`ci`** only
- [ ] Toolbox atoms pinned at `<toolbox-pin>`; **alls-green** pinned at latest release tag
