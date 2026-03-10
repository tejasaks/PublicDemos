---
description: "Use when working in SQLK8sOperator for Go controller changes, CRD/API updates, Kubernetes manifests, or operator tests. Enforces plan-first edits, idempotent reconciliation, and targeted validation."
name: "SQLK8sOperator Instructions"
applyTo: "SQLK8sOperator/**"
---
# SQLK8sOperator Guidelines

- Start with a short implementation plan before any edits.
- Keep reconciliation logic idempotent and safe on repeated runs.
- Prefer minimal diffs; avoid unrelated refactors.
- If code behavior changes, update impacted docs in `docs/`, samples in `samples/`, and any references in `README.md` or sub-readmes in the same PR.
- Include a concise devil's-advocate critique in responses for change requests, covering expected gains, potential regressions, and operational risks.
- For CRD/API changes in `pkg/apis/`, run `make manifests generate` and verify generated files in `deploy/crds/`.
- When touching controllers in `internal/controller/`, validate with at least `make test` and run focused shell tests when behavior changes.

## Documentation and Samples Sync Checklist

- Confirm user-facing behavior changes are reflected in docs and examples.
- Update sample YAML/manifests when defaults, fields, or required values change.
- Update cross-references and links pointing to renamed or moved docs/samples.
- Do not leave code and docs out of sync across `README.md`, `docs/`, and `samples/`.

## Response Template

For change requests, structure responses with these sections in order:

- Plan: brief implementation plan before edits.
- Changes: what was modified and where.
- Validation: commands run and outcomes.
- Critique: devil's-advocate analysis of positives, negatives, risks, and tradeoffs.

If a section is not applicable, explicitly state why.

## Validation Commands

Run from `SQLK8sOperator/` and choose the nearest relevant checks:

- `make fmt`
- `make vet`
- `make test`
- `make lint`
- `./tests/run-all-tests.sh`

## Key File Anchors

- Entrypoint: `cmd/mssql-operator/main.go`
- Reconciliation code: `internal/controller/`
- API/CRD types: `pkg/apis/`
- Deployment manifests: `deploy/`
- Docs and guides: `README.md`, `docs/`
- Examples and manifests: `samples/`
- Test scripts: `tests/`
