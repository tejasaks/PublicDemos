# Antigravity Assistant Instructions

## Workflow Rules & Agentic Mindset

As Antigravity (a powerful agentic AI coding assistant), adhere to the following workflow when assisting with this repository:

- **Planning First:** Always enter the `PLANNING` mode and create an `implementation_plan.md` artifact before making code changes. 
- **Step-by-Step Execution:** For non-trivial tasks, list steps, impacted files, and validation commands before editing. Prefer the smallest safe change set and avoid unrelated refactors.
- **Artifacts for Complex Tasks:** Use artifacts for summaries, test outcomes, or implementation details. Do not use artifacts for tiny one-off questions.
- **Critique & Devil's Advocate:** For change requests where it adds value, provide a brief critique section evaluating potential risks, tradeoffs, and impact (positive/negative). Skip the critique for straightforward tasks or simple commands where there is nothing meaningful to critique.
- **Industry Comparison:** For architectural or strategic questions, include a comparison with competitive products, industry offerings, or general industry standards. Indicate whether the approach aligns with common practice or is novel — and if novel, explain how it differs. Being different is not a blocker, but the distinction should be visible.
- **Documentation Sync:** For any code change, update affected documentation docs, sample manifests/scripts, and cross-references in the same change set.
- **Architectural Updates:** For structural changes, update the relevant instruction files (`.github/antigravity-instructions.md`) and design evolution docs (`SQLK8sOperator/docs/development/design-evolution.md`, `SQLAICustomContainer/DESIGN-EVOLUTION.md`) in the same change set.
- **Testing and Verification:** Whenever code is changed, verify the build and tests by running the relevant commands via the terminal seamlessly without asking.

## Workspace Overview

This workspace is a monorepo with two primary projects:

1. **`SQLK8sOperator`**: A Go-based Kubernetes operator to declaratively deploy and manage SQL Server instances, including high availability with Availability Groups, monitoring, and validation webhooks.
2. **`SQLAICustomContainer`**: A Docker-based SQL Server 2025 custom container that can optionally include Ollama (AI runtime), MinIO (object storage), Polybase, and Caddy (HTTPS reverse proxy).

### Project Maturity
- Both projects are in **preview** — not yet GA or broadly announced.
- The operator API version is `v1alpha1`; breaking changes are acceptable and have occurred. 
- The roadmap is: current preview → blog announcement → v1beta1 → GA v1.0.
- Treat code as sample/reference quality heading toward production-grade. Design for correctness and clarity over premature optimization.

### Monorepo Rationale and Cross-Project Relationship
The two projects are **logically complementary but independently usable**:
- **SQLAICustomContainer** builds custom SQL Server container images.
- **SQLK8sOperator** deploys and manages SQL Server containers on Kubernetes using its extensible catalog.
- Enterprise setups build custom images with the container project, then deploy them natively with the operator.

## Technology Stack

### SQLK8sOperator
- **Language:** Go (1.24 targeted by go.mod; README states 1.21+)
- **Platform:** Kubernetes (1.26+)
- **Frameworks:** `sigs.k8s.io/controller-runtime`, `k8s.io/*` client APIs, `github.com/denisenkom/go-mssqldb`
- **Runtime artifacts:** Operator binary (`bin/mssql-operator`), AG helper sidecar binary (`bin/mssql-ag-helper`).

### SQLAICustomContainer
- **Tech:** Docker
- **Base images:** Microsoft SQL Server 2025 Ubuntu or RHEL variants
- **Core services:** SQL Server 2025 + Full-Text Search (always), Polybase (optional), Ollama (optional, default enabled), MinIO (optional, default disabled), Caddy (always).
- **Core orchestrator:** `build-and-run.sh`.

## Build, Test, and Run 

Run commands from the corresponding project root using your bash tools.

### SQLK8sOperator Commands (Root: `SQLK8sOperator/`)
- Build operator: `make build`
- Build sidecar: `make build-sidecar`
- Run locally: `make run`
- Unit tests: `make test`
- Lint/Format/Vet: `make lint`, `make fmt`, `make vet`
- Generate CRDs/deepcopy: `make manifests generate`
- Deploy to K8s: `make deploy`
- Full E2E shell tests: `./tests/run-all-tests.sh`
- *Note:* CI guards `main` pushes running `go test ./... -v -short` and building images (`ghcr.io/tejasaks/mssql-operator` / `mssql-ag-helper`).

### SQLAICustomContainer Commands (Root: `SQLAICustomContainer/`)
- Default build/run (Ubuntu + Ollama + Caddy): `./build-and-run.sh --sa-password 'YourStrong@Pass123'`
- Minimal (No Ollama): `./build-and-run.sh --sa-password 'YourStrong@Pass123' --install-ollama false`
- Full Stack: `./build-and-run.sh --sa-password 'YourStrong@Pass123' --polybase true --install-minio true`
- Tests: `cd tests && ./test-prerequisites.sh`, `cd tests && ./run-all-tests.sh`.

## Conventions and Pitfalls

### SQLK8sOperator
- **Idempotency:** Reconciliation logic MUST remain idempotent.
- **CRD Generations:** Keep CRD/API changes in sync by running `make manifests generate` whenever struct tags change in `pkg/apis/`.
- **Documentation Parity:** Keep code changes synced with docs under `docs/`, examples under `samples/`, and README links.
- **Scripts:** Keep shell scripts executable (`make ensure-scripts-executable`).
- **K8s Assumptions:** Validate Kubernetes assumptions (namespaces, hardcoded secret names) in tests and docs.

### SQLAICustomContainer
- **SA Password:** `--sa-password` is mandatory and must satisfy strict complexity checks.
- **Executable Flags:** Shell scripts usually require executable permissions (`chmod +x *.sh tests/*.sh`).
- **Port/Volume Ripple Effects:** Optional services drastically affect port/volume mappings. Keep build scripts, README, and Dockerfile logically consistent when toggling build defaults.
- **OS Variants:** Be extremely explicit about Ubuntu vs RHEL behaviors when touching package installers.

## Where to Look First

- Operator Entrypoint: `SQLK8sOperator/cmd/mssql-operator/main.go`
- Operator Controllers: `SQLK8sOperator/internal/controller/`
- Operator APIs: `SQLK8sOperator/pkg/apis/`
- CICD Workflows: `.github/workflows/build-operator.yaml`
- Container Lifecycle: `SQLAICustomContainer/Dockerfile` & `SQLAICustomContainer/build-and-run.sh`
- Design Evolution: `SQLK8sOperator/docs/development/design-evolution.md` & `SQLAICustomContainer/DESIGN-EVOLUTION.md`

## Design Evolution Summary

Understanding historical context prevents re-introducing solved bugs.

### SQLAICustomContainer Evolution
1. **Single-purpose container** — Fixed bundle of SQL Server + Ollama + Caddy.
2. **Optional components** — MinIO and Polybase made toggleable via build args.
3. **Dual-OS support** — Single Dockerfile supporting both Ubuntu and RHEL runtime detection.
4. **Test automation** — Deep matrix tests for deployment scenarios.
5. **Docs matrix** — Highly structured testing references (CHECKLIST, INDEX).

### SQLK8sOperator Evolution
1. **Alpha skeleton** — Basic controller layout.
2. **Simplification** — Stripped Helm dependency; relies purely on Kustomize/YAML manifests.
3. **Core features** — Sidecar exporter logic, comprehensive webhook validators.
4. **Availability Groups** — AG controller, sidecar VIP listener routing, automatic/manual failover.
5. **Webhook TLS** — Multi-mode cert handling (self-signed, manual, cert-manager).
6. **Extensible image catalog** — Removed hardcoded images in favor of mapping inside `OperatorConfiguration`.
7. **Connection resilience** — AG Helper state machine tracks `Connected -> Reconnecting -> Disconnected`.
8. **sp_server_diagnostics** — Real WSFC behavior mirrored via Levels 1-5 health evaluations.
9. **Sidecar restructuring** — Shifted tuning metadata under `spec.sidecar.advanced`.
