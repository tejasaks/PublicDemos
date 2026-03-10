# Project Guidelines

## Workflow First

- Always create a short implementation plan before making any code changes.
- For non-trivial tasks, list steps, impacted files, and validation commands before editing.
- Prefer the smallest safe change set and avoid unrelated refactors.
- For any code change, update affected documentation, sample manifests/scripts, and cross-references in the same change set.
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
- Container build/runtime flow: `SQLAICustomContainer/Dockerfile`, `SQLAICustomContainer/build-and-run.sh`
- Container test docs/scripts: `SQLAICustomContainer/tests/`
