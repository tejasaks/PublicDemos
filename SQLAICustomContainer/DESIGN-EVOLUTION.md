# SQLAICustomContainer Design Evolution

This document traces how the SQL Server 2025 custom container project reached its current design. Each phase describes what changed, why, and what constraints it introduced.

---

## Phase 1: Single-Purpose Container

**Commit:** `627a1e5` — "The first commit for SQLAICustomContainer sample"

**What shipped:**
- Single Dockerfile: SQL Server 2025 + Ollama + Caddy as a fixed bundle
- `build-and-run.sh` orchestration script
- SQL Server on port 1433, Ollama behind Caddy HTTPS on port 11435
- Caddy provides automatic TLS certificate generation and acts as HTTPS reverse proxy

**Why this shape:** Demonstrate SQL Server 2025's AI capabilities (sp_invoke_external_rest_endpoint calling local Ollama models) in a single container with HTTPS handled by Caddy.

**Constraints introduced:** Caddy is always installed (provides TLS infrastructure even when Ollama is disabled). The entrypoint script manages multiple processes in a single container.

---

## Phase 2: Optional Components

**Commits:** `426a092` — "Updating the several component installs optional"; `d7e9562` — "Updating the solution with S3 support scenarios"

**What changed:**
- MinIO (S3-compatible object storage) added as optional component
- Polybase added as optional component (external data connectivity)
- Ollama made optional (default: enabled)
- Build args: `INSTALL_OLLAMA`, `INSTALL_MINIO`, `ENABLE_POLYBASE`
- `build-and-run.sh` flags: `--install-ollama`, `--install-minio`, `--polybase`
- Port/volume mappings conditional on enabled components

**Why:** Not every user needs AI capabilities. Some want just SQL Server + Polybase for external data, or SQL + MinIO for S3-compatible storage. Making components optional reduces image size and attack surface.

**Key design decisions:**
- **Ollama defaults ON, MinIO defaults OFF** — The "AI container" identity is preserved by default, but users can strip it down. MinIO is less commonly needed and adds significant complexity (TLS certs, additional ports).
- **Port/volume mapping is dynamic** — `build-and-run.sh` only maps ports and volumes for enabled components. This prevents port conflicts and unnecessary volume creation.

**Constraints introduced:** Every new optional component must follow the pattern: build arg in Dockerfile, flag in `build-and-run.sh`, conditional port/volume mapping, and documentation for each combination.

---

## Phase 3: Dual-OS Support

**Part of Phase 2 commits, refined in subsequent fixes**

**What changed:**
- Single Dockerfile detects Ubuntu vs RHEL at build time via `/etc/debian_version` and `/etc/redhat-release`
- Package installation uses `apt-get` (Ubuntu) or `yum/dnf` (RHEL) accordingly
- Works with both `mcr.microsoft.com/mssql/server:2025-latest` (Ubuntu) and `mcr.microsoft.com/mssql/rhel/server:2025-latest` (RHEL)
- `--base-image` flag allows switching base

**Why:** Enterprise customers often require RHEL-based images for compliance. Supporting both from a single Dockerfile avoids maintaining two separate Dockerfiles.

**Constraints introduced:** Any package installation must include both Ubuntu and RHEL paths. Cannot use distro-specific features that don't exist on the other.

---

## Phase 4: Test Automation

**Commits:** `f19d641` through `3f5a836` — Test infrastructure evolution

**What changed:**
- `test-prerequisites.sh` — Validates Docker, ports, disk, memory before running tests
- `test-deployments.sh` — Matrix of deployment scenarios (all 7 component combinations)
- `test-scenarios.conf` — Configuration file defining test scenarios
- `run-all-tests.sh` — Orchestrator running prerequisites + deployments
- `cleanup.sh` — Removes test containers and volumes
- `diagnose.sh` — Collects diagnostic data from running containers
- `--no-follow` flag added to `build-and-run.sh` for non-interactive test automation

**Why:** Manual testing of 7 deployment configurations was error-prone and slow. Automated tests validate all combinations in CI-like fashion.

**Key design decision — `--no-follow` flag:** Without this, `build-and-run.sh` tails container logs indefinitely, blocking automation. The flag was added specifically for test harness use.

**Constraints introduced:** New flags or behavioral changes must not break the test matrix. `test-scenarios.conf` must be updated when deployment configurations change.

---

## Phase 5: Documentation Maturity

**Commits:** `052406b` blog post; test doc restructuring

**What changed:**
- `blog-post.md` — Narrative walkthrough for publication
- `tests/CHECKLIST.md` — Manual verification checklist
- `tests/QUICKSTART.md` — Fast-path for running tests
- `tests/INDEX.md` — Test documentation index
- `tests/FILE-STRUCTURE.md` — Explanation of test directory layout
- `tests/EXAMPLE-OUTPUT.md` — Expected output for reference

**Why:** Self-contained test documentation helps new contributors run and understand the test suite without external context.

---

## Deployment Configuration Matrix

The following 7 configurations are tested and documented:

| # | Configuration | Ports | Key Use Case |
|---|--------------|-------|-------------|
| 1 | SQL + FTS only | 1433 | Minimal footprint |
| 2 | SQL + FTS + Polybase | 1433 | External data connectivity |
| 3 | SQL + FTS + MinIO | 1433, 9000, 9001 | S3-compatible storage |
| 4 | SQL + FTS + Polybase + MinIO | 1433, 9000, 9001 | Polybase → MinIO queries |
| 5 | SQL + FTS + Ollama (default) | 1433, 11435 | AI-enabled SQL Server |
| 6 | SQL + FTS + Ollama + Polybase | 1433, 11435 | AI + external data |
| 7 | Full stack (all components) | 1433, 9000, 9001, 11435 | Complete demo |

---

## Architecture Invariants

These decisions should not be changed without careful consideration:

1. **Single Dockerfile** — One Dockerfile for both Ubuntu and RHEL. Do not split into separate Dockerfiles.
2. **Caddy always installed** — Even when Ollama and MinIO are disabled. It provides TLS infrastructure and may be used for future services.
3. **Ollama defaults ON** — Preserves the "AI container" identity.
4. **Dynamic port/volume mapping** — `build-and-run.sh` only maps what's enabled.
5. **`--sa-password` required** — Never hardcode or default a password.
6. **Test matrix covers all 7 configurations** — New configurations must be added to the test suite.
