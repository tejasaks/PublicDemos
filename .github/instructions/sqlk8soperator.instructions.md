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
- For architectural or significant structural changes, also update `docs/development/design-evolution.md`, the Design Evolution section in `.github/instructions/sqlk8soperator.instructions.md`, and the Design Evolution Summary in `.github/copilot-instructions.md`.
- Include a concise devil's-advocate critique in responses for change requests where it adds value, covering expected gains, potential regressions, and operational risks. Skip the critique for straightforward tasks with nothing meaningful to critique.
- For architectural or strategic questions, include a brief comparison with competitive Kubernetes operators or industry-standard approaches, noting alignment or novel divergence.
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

### CI Gate Mirror

The GitHub Actions workflow (`.github/workflows/build-operator.yaml`) runs `go test ./... -v -short` before building images. CI triggers only on code/build file changes (`cmd/`, `internal/`, `pkg/`, `deploy/`, `hack/`, `go.mod`, `go.sum`, `Makefile`, `Dockerfile*`, `install.yaml`); changes to `docs/`, `samples/`, `tests/`, and markdown files do not trigger image rebuilds. Locally, `make test` and `make lint` replicate the CI gates. Always ensure these pass before proposing changes.

## Key File Anchors

- Entrypoint: `cmd/mssql-operator/main.go`
- Reconciliation code: `internal/controller/`
- API/CRD types: `pkg/apis/`
- Deployment manifests: `deploy/`
- Docs and guides: `README.md`, `docs/`
- Examples and manifests: `samples/`
- Test scripts: `tests/`
- CI/CD workflow: `.github/workflows/build-operator.yaml`
- Design evolution context: `docs/development/design-evolution.md`

## Design Evolution (Quick Reference)

The operator evolved through these phases (see `docs/development/design-evolution.md` for commit-level detail):

1. Alpha skeleton with CRDs and Makefile
2. Helm removal → pure YAML manifests
3. Monitoring integration (SQL Exporter sidecar)
4. Comprehensive webhook validation
5. AG capability (controller, AG Helper sidecar, listener VIP, failover)
6. Webhook TLS (3 modes: self-signed, manual, cert-manager)
7. Extensible image catalog in OperatorConfiguration
8. Connection resilience (state machine, retries, staleness detection)
9. sp_server_diagnostics integration (failure condition levels 1-5)
10. Sidecar spec restructuring (`spec.sidecar.advanced.*` — v1alpha1 breaking change)

When making changes, check whether the feature you’re touching has prior evolution context that constrains the design.
