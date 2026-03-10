---
description: "Use when editing GitHub Actions workflow files. Ensures CI/CD documentation stays synchronized with workflow changes."
applyTo: ".github/workflows/**"
---

# CI Workflow Change Synchronization

When modifying any GitHub Actions workflow under `.github/workflows/`:

## Required Sync Points

1. **Global instructions** — Update the `CI/CD (GitHub Actions)` subsection in `.github/copilot-instructions.md` to reflect any changes to triggers, jobs, gates, image tags, or release behavior.
2. **Operator instructions** — If the change affects operator build/test gates, update the `CI Gate Mirror` section in `.github/instructions/sqlk8soperator.instructions.md` so local development commands stay aligned with CI expectations.
3. **README references** — If build badges, image names, or install instructions reference CI artifacts, update the relevant `README.md` files.

## Key Facts About Current CI

- Workflow: `.github/workflows/build-operator.yaml`
- Triggers: push to `main` (when SQLK8sOperator code/build files change — `cmd/`, `internal/`, `pkg/`, `deploy/`, `hack/`, `go.mod`, `go.sum`, `Makefile`, `Dockerfile*`, `install.yaml`; docs, samples, tests, and markdown are excluded), tag `v*.*.*`, or manual `workflow_dispatch`.
- CI gate: `go test ./... -v -short` must pass before image builds.
- Images: `ghcr.io/tejasaks/mssql-operator` and `ghcr.io/tejasaks/mssql-ag-helper` (linux/amd64 + arm64).
- Tag push creates a draft GitHub Release with install instructions.

## Validation

After editing a workflow file, verify:

- YAML is well-formed (no syntax errors).
- Job dependencies (`needs:`) are correct.
- Secrets/environment references still resolve.
- Documentation sync points above are updated in the same change set.
