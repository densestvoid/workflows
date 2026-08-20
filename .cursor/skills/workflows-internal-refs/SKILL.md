---
name: workflows-internal-refs
description: >-
  References toolbox actions from reusable workflows in densestvoid/workflows using
  relative paths (./.github/actions/...). Use when a workflow in this repo calls
  another action in the same repo, when fixing @main self-references, when
  authoring go-checks or new reusable workflows, or when asked whether relative
  paths work for cross-repo callers.
---

# Workflows Internal Action References

## When to read this skill

Read this **before** adding `uses:` lines inside `.github/workflows/*.yml` in this repo that point at another action in the same repo.

Do **not** use `densestvoid/workflows/.github/actions/foo@main` inside this repo.

## Rules

| Context | Reference pattern | Example |
|---------|-------------------|---------|
| **Caller repo** invokes toolbox | Full ref with tag | `densestvoid/workflows/.github/workflows/go-checks.yml@v1` |
| **Caller repo** invokes one action | Full ref with tag | `densestvoid/workflows/.github/actions/build-go@v1` |
| **Reusable workflow in this repo** invokes sibling action | Relative path | `./.github/actions/install-go-tool` |

Relative `./` paths resolve to **this repository at the ref the caller pinned** on the reusable workflow — not the caller's repo, not `@main`.

```yaml
# app repo
go-checks:
  uses: densestvoid/workflows/.github/workflows/go-checks.yml@v1

# inside go-checks.yml (workflows repo)
- uses: ./.github/actions/install-go-tool
```

Both resolve at `@v1`.

## Why

- Avoids chicken-and-egg (`@main` before `@v1` exists)
- PR CI in this repo tests the branch's action code, not main
- One pin for callers; internal refs version-lock automatically

## Caveat

`github.ref`, `github.sha`, and `github.repository` refer to the **caller** workflow. Only matters when the reusable workflow must checkout **this repo** to run scripts or read files beyond the action definition itself. For composite actions whose steps are self-contained (e.g. **install-go-tool**), relative `uses:` is enough.

## Anti-patterns

| Avoid | Use instead |
|-------|-------------|
| `densestvoid/workflows/...@main` inside this repo | `./.github/actions/...` |
| Pinning each internal action separately in reusable workflows | Relative path; caller pins the workflow ref |
| `./.github/workflows/foo.yml` from another repo | `owner/repo/.github/workflows/foo.yml@ref` |

## Canonical example

`.github/workflows/go-checks.yml` — Static-Analysis job uses `./.github/actions/install-go-tool`.
