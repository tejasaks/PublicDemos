# SQLK8sOperator Design Evolution

[← Back to Development](../README.md) | [Architecture Overview](../architecture/overview.md)

This document traces how the SQL Server Kubernetes Operator reached its current design. Each phase describes what changed, why, and what constraints it introduced. New contributors should read this to understand the reasoning behind the current architecture before proposing changes.

---

## Phase 1: Alpha Skeleton

**Commit:** `3423c34` — "Alpha commit of the Kubernetes operator for SQL server"

**What shipped:**
- Basic operator binary (`cmd/mssql-operator/main.go`) using `controller-runtime`
- Three CRDs: `SQLServer`, `SQLServerAG`, `OperatorConfiguration` (all `v1alpha1`)
- Makefile-driven build (`make build`, `make test`, `make run`)
- Initial controller stubs for `SQLServer` and `SQLServerAG`
- AG Helper sidecar binary (`cmd/ag-helper/main.go`)
- Deploy manifests under `deploy/`

**Why this shape:** Follow the established Kubernetes operator pattern (Zalando postgres-operator, CrunchyData postgres-operator) with controller-runtime. Three CRDs separate concerns: instance definition, HA configuration, and operator-wide defaults.

**Constraints introduced:** CRD API group `mssql.microsoft.com`, sidecar pattern for AG management (vs. building monitoring into the SQL image itself).

---

## Phase 2: Helm Removal

**Commit:** `90914c3` — "Removed Helm dependency for this sample Operator"

**What changed:** Removed Helm charts in favor of plain YAML manifests (`deploy/`, `install.yaml`).

**Why:** Helm added complexity without proportional value for a sample operator. Pure YAML keeps the deployment story simple and self-contained. The `install.yaml` file is a single-file install for quick starts, generated from the individual manifests under `deploy/`.

**Constraints introduced:** All deployment customization happens via `OperatorConfiguration` CRD rather than Helm values. `scripts/generate-install-yaml.sh` concatenates manifests into `install.yaml`.

---

## Phase 3: Monitoring & Naming

**Commits:** `cb0a3e0` — "Several enhancement in naming, monitoring and desc"

**What changed:**
- SQL Exporter sidecar added to pods when `spec.monitoring.enabled: true`
- Standardized naming: resource names, label conventions (`app.kubernetes.io/*`, `mssql.microsoft.com/*`)
- Name length validation (≤13 chars) for NetBIOS compatibility

**Why:** Prometheus metrics are expected in production Kubernetes. The ≤13-char name limit comes from SQL Server NetBIOS constraints (15-char max minus 2 for StatefulSet pod suffix `-0`).

**Constraints introduced:** Name limit is enforced at webhook and controller level — cannot be relaxed without changing the pod naming scheme.

---

## Phase 4: Comprehensive Validations

**Commit:** `ca00b5b` — "Comprehensive validations update"

**What changed:**
- Webhook validation for `SQLServer` and `SQLServerAG` CRDs
- Validation modes: `block` (deny) vs `warn` (allow with warning)
- Storage class existence check, secret existence check, password complexity
- Configurable via `OperatorConfiguration` validation settings

**Why:** Early validation prevents runtime failures and provides clear error messages. The block/warn modes balance strictness with flexibility for different environments (e.g., CI may not have real storage classes).

**Constraints introduced:** Any new CRD field that can be user-misconfigured should have webhook validation. The validation config is in OperatorConfiguration, not per-resource.

---

## Phase 5: Availability Group Capability

**Commits:** `444a373` through `5979563` — Multiple AG-related commits

**What changed:**
- SQLServerAG controller with full reconciliation loop
- AG Helper sidecar: queries DMVs (`sys.dm_hadr_availability_replicas`, etc.), exposes HTTP API on `:8080`
- Listener Service: selectorless Kubernetes Service with operator-managed Endpoints
- Manual failover via annotation (`mssql.microsoft.com/failover-initiated=true`)
- Automatic failover when primary is unhealthy

**Why:** AG is the standard SQL Server HA mechanism. The sidecar pattern avoids coupling monitoring to the SQL Server image. Selectorless Service with operator-managed Endpoints allows routing the listener VIP exclusively to the current primary pod, without label-based endpoint thrashing.

**Key design decision — Listener routing:** Using a selectorless Service where the operator updates the Endpoints object every `monitorInterval` (10s). This was chosen over pod-label-based selectors because label updates on pods cause immediate endpoint changes, creating thrashing during AG role transitions. The operator-managed approach provides controlled, intentional routing.

**Constraints introduced:** AG Helper must run in every AG pod. The `/state` HTTP endpoint is the contract between controller and sidecar.

---

## Phase 6: Webhook TLS

**Commit:** `3e0ed28`, `3c4ec9e` — Webhook TLS certificate management

**What changed:**
- Three TLS certificate modes controlled by `WEBHOOK_CERT_MODE` env var:
  1. `self-signed` (default): Auto-generate CA + cert, patch `ValidatingWebhookConfiguration.caBundle`
  2. `manual`: Load from admin-created Kubernetes TLS Secret
  3. `cert-manager`: Load from cert-manager-managed Secret
- Code in `internal/webhook/certs/`

**Why:** Webhooks require TLS. Self-signed makes initial setup zero-config. Manual and cert-manager modes support enterprise PKI requirements.

**Constraints introduced:** The operator needs RBAC to patch `ValidatingWebhookConfiguration` in self-signed mode. Cert rotation is not automatic in manual mode.

---

## Phase 7: Extensible Image Catalog

**Commit:** `8b99669` — "feat: replace fixed SQL image slots with extensible catalog map"

**What changed:**
- `OperatorConfiguration.spec.images.catalog` changed from fixed `sql2019`/`sql2022`/`sql2025` fields to a `map[string]string`
- `SQLServer.spec.version` is now a key lookup into this catalog (e.g., `"2022"`, `"2025-fts"`, `"2025-ai"`)
- Users can register custom images: `"2025-ai": "myregistry/sql-server-ai:latest"`

**Why:** Fixed version fields couldn't accommodate custom images built with SQLAICustomContainer. The map-based catalog is infinitely extensible and lets users register any image variant.

**Constraints introduced:** The `version` field is a catalog key, not a raw image reference. Unknown keys fail validation. The `spec.instance.image` field still allows direct override bypassing the catalog.

---

## Phase 8: Connection Resilience

**Commit:** `ccb881b` — "feat: connection resilience for AG Helper sidecar"

**What changed:**
- Connection state machine: `Connected → Reconnecting → Disconnected`
- `executeWithRetry` wrapper for transient SQL error recovery
- Staleness detection: `lastSuccessfulQuery > stalenessThreshold` → `dataStale=true`
- Stale pods return 503 on `/healthz` and `/readyz`
- AG controller excludes stale pods from failover candidates
- New CLI flags: `--max-retries`, `--retry-interval`, `--staleness-threshold`
- New CRD fields under `spec.sidecar.advanced`: `maxRetries`, `retryInterval`, `stalenessThreshold`
- Connection pool tuning: 30min lifetime, 1 idle conn, 3 max open, 10min idle time

**Why:** SQL Server connections are brittle — network blips, AG reconfiguration, and SQL restarts cause transient failures. Without resilience, a single failed query could mark a healthy replica as disconnected. Staleness detection prevents the controller from making failover decisions on stale data.

**Key design decision — Staleness vs. disconnection:** These are separate concepts. Stale means data is old (connection might be fine); disconnected means connection failed after retries. A stale pod might still be healthy — it's just not reporting fresh data. The distinction prevents over-reacting to temporary query delays.

**Constraints introduced:** The `advanced` section in the sidecar spec is the home for all tuning knobs going forward.

---

## Phase 9: sp_server_diagnostics Integration

**Commit:** `b1383df` — "feat: add sp_server_diagnostics integration with failure condition levels"

**What changed:**
- Optional `failureConditionLevel` field (1-5) in `spec.failover`
- When level ≥ 2, AG Helper calls `EXEC sp_server_diagnostics` each monitor cycle
- Evaluates SQL Server's 5 internal components: system, resource, query_processing, io_subsystem, events
- Component errors at/above configured level trigger health=CRITICAL → automatic failover
- Mirrors Windows Server Failover Clustering (WSFC) failure condition levels
- Level 1 (default): AG topology only — fully backward compatible

**Why:** DMV-based monitoring (phases 5/8) only sees AG role and sync state. It can't detect internal SQL Server issues (memory pressure, spinlocks, deadlocks). sp_server_diagnostics provides OS-level and engine-level health signals that WSFC has used for years.

**Levels explained:**
| Level | What's evaluated | Typical use |
|-------|-----------------|-------------|
| 1 | AG topology + sync state only | Default, lowest overhead |
| 2 | sp_server_diagnostics responsiveness | Detects unresponsive SQL Server |
| 3 | + system component (spinlock, OOM) | Recommended for most production |
| 4 | + resource component (memory, schedulers) | High availability priority |
| 5 | + query_processing (deadlocks, workers) | Maximum detection sensitivity |

**Constraints introduced:** Level ≥ 2 requires additional SQL permissions for the sidecar login. Higher levels produce more health state changes — the controller must handle increased churn gracefully.

---

## Phase 10: Sidecar Spec Restructuring

**Commit:** `a6b668e` — "refactor: move sidecar tuning fields under spec.sidecar.advanced"

**What changed:**
- **BREAKING CHANGE** (v1alpha1): Operational tuning fields (`monitorInterval`, `connectionTimeout`, `maxRetries`, `retryInterval`, `stalenessThreshold`) moved from `spec.sidecar.*` to `spec.sidecar.advanced.*`
- `image` and `resources` remain at `spec.sidecar`
- Existing YAML that explicitly set tuning fields must add `advanced:` nesting

**Why:** The sidecar spec was becoming flat with too many fields at the same level. Separating container-spec fields (`image`, `resources`) from operational tuning (`monitorInterval`, `maxRetries`, etc.) improves readability and signals that the `advanced` fields are optional expert knobs.

**Constraints introduced:** This established the convention that new sidecar tuning fields go under `spec.sidecar.advanced.*`. Container-level fields stay at `spec.sidecar.*`.

---

## Phase 11: Samples Reorganization

**Commits:** `396659f` through `87616ce` — Samples restructuring

**What changed:**
- Samples organized into self-contained scenario folders: `sql-ag-ha/`, `sql-ag-ha-minimal/`, `sql-ag-ha-full/`, `sql-ag-ha-diagnostics/`, `sql-ag-dr/`, `sql-ag-monitoring/`, `sql-ag-multiag/`
- Each folder has its own README with prerequisites, steps, and customization guidance
- Shell scripts for multi-step scenarios (e.g., `setup-ag-step2.sh`)
- Root `samples/README.md` with per-sample descriptions

**Why:** Samples were scattered and lacked self-contained documentation. Each folder is now a complete runnable scenario that users can follow without cross-referencing other files.

---

## Architecture Invariants

These decisions are load-bearing and should not be changed without careful consideration:

1. **Sidecar pattern** — AG Helper runs alongside SQL Server, communicates via localhost. Do not move monitoring into the SQL image.
2. **Selectorless listener Service** — Endpoints managed by operator, not by pod labels. This prevents endpoint thrashing.
3. **Level-triggered reconciliation** — Controllers react to current state, not events. Keep reconciliation idempotent.
4. **≤13-char name limit** — NetBIOS constraint. Cannot be relaxed without changing pod naming.
5. **OperatorConfiguration singleton** — One cluster-scoped config. Per-namespace config was considered and rejected for simplicity.
6. **Catalog-based image resolution** — `version` is a catalog key, not an image reference.
7. **Advanced tuning under `spec.sidecar.advanced`** — New tuning fields go here, not at the sidecar root level.
