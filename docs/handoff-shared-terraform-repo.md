# Handoff: Shared Terraform Modules Repository

## Purpose

Create a **shared, application-agnostic Terraform modules repository** — a common place to publish reusable infrastructure building blocks.

Same philosophy as a composable toolbox: **modules, not orchestration**. Consumer repos own root modules, state, deploy triggers, and wiring. This repo owns **reusable, versioned modules** with no knowledge of specific applications.

This is **not** a single-purpose observability or Grafana repo. Those are candidate module domains among others.

---

## Design principles

1. **Application-agnostic** — modules take inputs from callers; no hardcoded app names or environment-specific defaults.
2. **Library, not deployer** — no root modules that apply production infrastructure on their own (except `examples/` for documentation).
3. **No orchestration** — no deploy pipelines for consumer apps; optional CI limited to validating modules in this repo.
4. **Versioned** — consumers pin module sources to git refs (e.g. `?ref=v1`).
5. **Documented contracts** — each module documents inputs, outputs, usage patterns, and provider caveats.

---

## Repo boundaries

### Shared Terraform repo (this repo)

| Owns | Does not own |
|------|----------------|
| Reusable modules under `modules/` | Application resources (apps, VPCs, databases, etc.) |
| `examples/` showing module consumption | Consumer deploy state |
| Per-module READMEs + catalog README | Consumer secrets |
| Optional `terraform fmt` / `validate` CI | Per-app registration or catalog entries |

### Consumer app repos

| Owns |
|------|
| Root modules and state backends |
| CI/CD that runs `terraform apply` |
| Module inputs and credentials (`TF_VAR_*`, secrets) |
| Merging module outputs into their resources |
| Application code and runtime configuration |
| Anything app-specific (dashboards, alerts, naming, SLOs) |

**Rule of thumb:**
- **Reusable shape** (connection pattern, standard fields, shared conventions) → shared module
- **Values and attachment** (which app, which deploy, which state file) → consumer repo

---

## Suggested repository layout

```
modules/
├── <domain>/
│   └── <module>/
│       ├── main.tf
│       ├── variables.tf
│       ├── outputs.tf
│       └── README.md
└── ...

examples/
└── <domain>/
    └── <module>-<platform>/
        └── main.tf

.github/workflows/
└── validate.yml          # optional: fmt + validate on PR

README.md                 # module catalog + consumption conventions
```

**Versioning:** git tags (`v1`, `v2`). Consumers source modules via:

```hcl
source = "github.com/<org>/<repo>//modules/<domain>/<module>?ref=v1"
```

---

## Module authoring conventions

- One concern per module; compose in consumer roots rather than nesting orchestration here.
- Inputs and outputs are explicit; avoid hidden provider configuration where possible.
- README per module: purpose, inputs, outputs, minimal example, known provider pitfalls.
- Examples use placeholder values only — no real app or environment names.
- New modules do not require changes to unrelated modules.

---

## Candidate first use case (design TBD)

**Observability / Grafana Cloud** for deployments on managed platforms (e.g. DigitalOcean App Platform) is a likely first module domain. The exact module API, signals (traces, metrics, logs), and wiring pattern are **not prescribed here** — to be designed by the implementing agent and repo owner.

Context worth preserving for that design conversation (not decisions):

- Connection may need to live in consumer deploy state rather than a separate action-owned state.
- Preview/PR environments should ideally appear in production dashboards via labels or variables, without per-deploy Grafana resource churn.
- Consumer repos likely already pass deploy identifiers and secrets via `TF_VAR_*` in CI.

---

## Implementation order (repo scaffold)

1. Scaffold repo structure (`modules/`, `examples/`, catalog `README.md`)
2. Add optional validate CI
3. Land first module once its contract is agreed
4. Tag `v1` when the first module is stable
5. Document consumer onboarding in README

---

## Future module domains (examples only)

Any domain that benefits from a shared, caller-parameterized pattern:

- Observability
- Networking
- DNS
- Databases
- IAM / access patterns

Each follows the same conventions above.

---

## Open questions (for repo owner + implementing agent)

- Repository name and org
- Which module domain ships first and its exact contract
- Org-level vs per-consumer secrets
- Whether optional template modules (dashboards, alerts) belong in this repo or stay in consumer repos

---

## Pitfalls checklist

- [ ] No hardcoded application or service identifiers in modules
- [ ] Examples are placeholders, not production config
- [ ] README states clearly: consumers own state and roots
- [ ] Module READMEs document provider-specific caveats
- [ ] Tags/releases are used; consumers pin refs
