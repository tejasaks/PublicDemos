# Project Guidelines

## Workflow First

- Always create a short implementation plan before making any code changes.
- For non-trivial tasks, list steps, impacted files, and validation commands before editing.
- Prefer the smallest safe change set and avoid unrelated refactors.
- For any code change, update affected documentation, sample manifests/scripts, and cross-references in the same change set.
- For architectural or significant structural changes, also update the relevant copilot instruction files (`.github/copilot-instructions.md`, `.github/instructions/*.instructions.md`) and design evolution docs (`SQLK8sOperator/docs/development/design-evolution.md`, `SQLAICustomContainer/DESIGN-EVOLUTION.md`) in the same change set.
- For each change request, include a brief critique section that plays devil's advocate and evaluates potential positive and negative impacts, risks, and tradeoffs.
- After edits, run the nearest relevant tests/lint checks for the changed area.

## Response Template

For change requests, structure responses with these sections in order:

- Plan: brief implementation plan before edits.
- Changes: what was modified and where.
- Validation: commands run and outcomes.
- Critique: devil's-advocate analysis of positives, negatives, risks, and tradeoffs.

If a section is not applicable, explicitly state why.

## Workspace Overview

This workspace is a monorepo with two primary projects:

1. `SQLK8sOperator`: A Go-based Kubernetes operator to declaratively deploy and manage SQL Server instances, including high availability with Availability Groups, monitoring, and validation webhooks.
2. `SQLAICustomContainer`: A Docker-based SQL Server 2025 custom container that can optionally include Ollama (AI runtime), MinIO (object storage), Polybase, and Caddy (HTTPS reverse proxy).

### Project Maturity

- Both projects are in **preview** — not yet GA or broadly announced.
- The API version is `v1alpha1`; breaking changes are acceptable and have occurred (e.g., sidecar spec restructuring).
- The intended roadmap is: current preview → blog announcement → v1beta1 → GA v1.0 → v2, v3 with clear versioned scope boundaries.
- Treat code as sample/reference quality heading toward production-grade. Design for correctness and clarity over premature optimization.

### Monorepo Rationale and Cross-Project Relationship

The two projects are **logically complementary but independently usable**:

- **SQLAICustomContainer** builds custom SQL Server container images (specific configurations of SQL + Ollama + MinIO + Polybase + Caddy).
- **SQLK8sOperator** deploys and manages SQL Server containers on Kubernetes (any image from its extensible catalog).
- In enterprise scenarios, users will likely use both: build a custom image with SQLAICustomContainer, then register it in the operator's image catalog and deploy via CRDs.
- They remain in the same monorepo because they share a common domain (SQL Server on containers/Kubernetes) and co-evolve, while maintaining independent build/test/deploy workflows.
- If the projects diverge significantly in audience or release cadence, splitting into separate repos is reasonable — but for now the monorepo keeps cross-project context accessible.

## Technology Stack

### SQLK8sOperator

- Language: Go (go.mod targets Go 1.24; README states 1.21+)
- Platform: Kubernetes (README states 1.26+)
- Frameworks/libraries:
  - `sigs.k8s.io/controller-runtime` (controllers/webhooks/manager)
  - `k8s.io/*` client APIs
  - `github.com/denisenkom/go-mssqldb`
- Runtime artifacts:
  - Operator binary: `bin/mssql-operator`
  - AG helper sidecar binary: `bin/mssql-ag-helper`

### SQLAICustomContainer

- Build/runtime: Docker
- Base images: Microsoft SQL Server 2025 Ubuntu or RHEL variants
- Core services:
  - SQL Server 2025 + Full-Text Search (always)
  - Polybase (optional)
  - Ollama (optional, default enabled)
  - MinIO (optional, default disabled)
  - Caddy (always installed for TLS/reverse proxy)
- Primary orchestration script: `build-and-run.sh`

## Build, Test, and Run

Run commands from the project root you are working in.

### SQLK8sOperator commands

- Build operator: `make build`
- Build sidecar: `make build-sidecar`
- Run locally: `make run`
- Unit tests: `make test`
- Lint: `make lint`
- Format: `make fmt`
- Vet: `make vet`
- Generate CRDs/code: `make manifests generate`
- Deploy to cluster: `make deploy`
- Remove deployment: `make undeploy`
- Full shell tests: `./tests/run-all-tests.sh`

### CI/CD (GitHub Actions)

- Workflow: `.github/workflows/build-operator.yaml`
- Triggers: push to `main` (when SQLK8sOperator code/build files change — docs, samples, tests, and markdown are excluded), git tag `v*.*.*`, or manual `workflow_dispatch`
- CI gates: `go test ./... -v -short` runs before any image build
- Images built: `ghcr.io/tejasaks/mssql-operator` and `ghcr.io/tejasaks/mssql-ag-helper` (linux/amd64 + arm64)
- Tag push creates a draft GitHub Release with install instructions
- When proposing changes to operator code, ensure `make test` and `make lint` pass — these mirror what CI enforces.

### SQLAICustomContainer commands

- Build/run default configuration: `./build-and-run.sh --sa-password 'YourStrong@Pass123'`
- Minimal (no Ollama): `./build-and-run.sh --sa-password 'YourStrong@Pass123' --install-ollama false`
- Full stack: `./build-and-run.sh --sa-password 'YourStrong@Pass123' --polybase true --install-minio true`
- Full test matrix: `cd tests && ./run-all-tests.sh`
- Prerequisite test: `cd tests && ./test-prerequisites.sh`
- Deployment scenario tests: `cd tests && ./test-deployments.sh`

## Conventions and Pitfalls

### SQLK8sOperator

- Reconciliation logic should remain idempotent.
- Keep CRD/API changes in sync by running generation targets before proposing CRD-related changes.
- Keep code changes synchronized with docs under `docs/`, examples under `samples/`, and any README links that reference changed behavior.
- Keep shell scripts executable where required (`make ensure-scripts-executable` target exists).
- Validate Kubernetes assumptions in docs/tests (storage class, namespace, secret names).

### SQLAICustomContainer

- `--sa-password` is required and should satisfy SQL Server complexity requirements.
- Shell scripts may require executable permission after clone (`chmod +x *.sh tests/*.sh`).
- Optional services affect port/volume mappings; keep script, README, and Dockerfile aligned when changing options.
- When runtime behavior or flags change, update `README.md`, `tests/*.md`, and any sample commands to keep docs and runnable examples consistent.
- Be explicit about Ubuntu vs RHEL behavior when touching package/install logic.

## Where to Look First

- Operator entrypoint: `SQLK8sOperator/cmd/mssql-operator/main.go`
- Operator controllers: `SQLK8sOperator/internal/controller/`
- Operator APIs/CRDs: `SQLK8sOperator/pkg/apis/` and `SQLK8sOperator/deploy/crds/`
- Operator build/test automation: `SQLK8sOperator/Makefile`, `SQLK8sOperator/tests/`
- CI/CD workflow: `.github/workflows/build-operator.yaml`
- Container build/runtime flow: `SQLAICustomContainer/Dockerfile`, `SQLAICustomContainer/build-and-run.sh`
- Container test docs/scripts: `SQLAICustomContainer/tests/`
- Design evolution context: `SQLK8sOperator/docs/development/design-evolution.md`, `SQLAICustomContainer/DESIGN-EVOLUTION.md`

## Design Evolution Summary

Understanding how the projects reached their current design helps avoid re-introducing solved problems or contradicting established patterns.

### SQLAICustomContainer Evolution

1. **Single-purpose container** — Initial commit: SQL Server + Ollama + Caddy as a fixed bundle.
2. **Optional components** — MinIO and Polybase made toggleable via build args; Ollama made optional.
3. **Dual-OS support** — Single Dockerfile with runtime Ubuntu/RHEL detection.
4. **Test automation** — Prerequisite checks, deployment scenario matrix, cleanup scripts.
5. **Documentation maturity** — Blog post, structured test docs (CHECKLIST, QUICKSTART, INDEX).

### SQLK8sOperator Evolution

1. **Alpha skeleton** — Basic operator with CRDs, Makefile, controllers.
2. **Simplification** — Removed Helm dependency; pure YAML manifests.
3. **Core features** — Monitoring (SQL Exporter sidecar), naming conventions, comprehensive validations.
4. **AG capability** — Availability Group controller, AG Helper sidecar, listener VIP routing, manual/automatic failover.
5. **Webhook TLS** — Three certificate modes (self-signed, manual, cert-manager).
6. **Extensible image catalog** — Replaced fixed SQL version slots with a user-extensible map in OperatorConfiguration.
7. **Connection resilience** — AG Helper state machine (Connected → Reconnecting → Disconnected), staleness detection, retry logic.
8. **sp_server_diagnostics** — Failure condition levels 1-5 mirroring WSFC behavior for deeper health evaluation.
9. **Sidecar restructuring** — Moved tuning fields under `spec.sidecar.advanced.*` (v1alpha1 breaking change).
10. **Samples reorganization** — Self-contained scenario folders with per-sample READMEs and shell scripts.

For detailed evolution with commit references and design rationale, see the design evolution documents linked above.
