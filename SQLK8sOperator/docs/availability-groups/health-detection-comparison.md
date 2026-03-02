# Health Detection Comparison: WSFC vs Kubernetes Operator

[← Back to Availability Groups](overview.md) | [Failover Management](failover-management.md) | [Documentation Home](../README.md)

This document compares the health detection capabilities of the SQL Server Kubernetes Operator
with the Windows Server Failover Clustering (WSFC) model used in traditional SQL Server AG
deployments. Understanding these differences is essential for choosing the right
`failureConditionLevel` and setting appropriate expectations for failover behavior.

## Table of Contents

- [Background: WSFC Health Detection](#background-wsfc-health-detection)
- [Operator Health Detection Model](#operator-health-detection-model)
- [Failure Condition Levels](#failure-condition-levels)
- [Detection Capability Matrix](#detection-capability-matrix)
- [How sp_server_diagnostics Works](#how-sp_server_diagnostics-works)
- [Architecture: Over-TDS vs Preemptive Thread](#architecture-over-tds-vs-preemptive-thread)
- [Choosing a Failure Condition Level](#choosing-a-failure-condition-level)
- [Configuration](#configuration)
- [Monitoring Diagnostics](#monitoring-diagnostics)

## Background: WSFC Health Detection

In traditional Windows-based SQL Server AG deployments, the Windows Server Failover Clustering
(WSFC) infrastructure provides multi-layered health detection:

### WSFC Health Check Layers

| Layer | Mechanism | What It Detects |
|-------|-----------|-----------------|
| **LooksAlive** | Lightweight resource DLL check (every 5s) | Process existence, basic responsiveness |
| **IsAlive** | Full T-SQL connectivity test (every 30s) | SQL Server query processing capability |
| **Lease Timeout** | Preemptive-thread timer (default 20s) | SQL Server hang (scheduler dumps, spinlocks, OOM freeze) |
| **sp_server_diagnostics** | Dedicated session on a preemptive thread | Internal component health (system, resource, query_processing, io_subsystem, events) |
| **Failure Condition Level** | Policy (levels 1–5) | Controls which diagnostic signals trigger automatic failover |

The key architectural advantage of WSFC is the **preemptive thread**: `sp_server_diagnostics`
runs on thread that is not subject to SQL Server's cooperative scheduler. This means it can
detect hangs where the engine is completely stuck — something that over-TDS connections cannot
guarantee because they are subject to the same scheduler.

### WSFC Failure Condition Levels

| Level | WSFC Behavior |
|-------|---------------|
| 1 | Failover when SQL Server service is down |
| 2 | + Failover when `sp_server_diagnostics` does not respond within `HealthCheckTimeout` |
| 3 | + Failover when `system` component reports error (spinlock, access violation, OOM) |
| 4 | + Failover when `resource` component reports error (memory/scheduler pressure) |
| 5 | + Failover when `query_processing` component reports error (runaway queries) |

> **Reference:** [Microsoft: Configure FailureConditionLevel](https://learn.microsoft.com/en-us/sql/sql-server/failover-clusters/windows/configure-failureconditionlevel-property-settings)

## Operator Health Detection Model

The SQL Server Kubernetes Operator uses a different architecture but aims to achieve
equivalent detection coverage for levels 1–5:

### Operator Health Check Layers

| Layer | Mechanism | What It Detects |
|-------|-----------|-----------------|
| **Pod Status** | Kubernetes kubelet probes (liveness/readiness) | Container crash, process death |
| **AG Topology** | DMV queries: `sys.dm_hadr_availability_replica_states`, `sys.dm_hadr_database_replica_states` | Role loss, sync failure, replica disconnection |
| **Connection Resilience** | Connection state machine with retry/staleness | SQL Server unreachable, TDS timeout |
| **sp_server_diagnostics** | `EXEC sp_server_diagnostics` over TDS (opt-in) | Internal component health (system, resource, query_processing, io_subsystem, events) |
| **Failure Condition Level** | CRD field `failureConditionLevel` (1–5) | Controls which diagnostic signals trigger automatic failover |

### What the Operator Monitors by Default (Level 1)

Without `failureConditionLevel` (or when set to 1), the AG Helper sidecar queries these DMVs
on every health check cycle:

1. **Local replica role** — `sys.dm_hadr_availability_replica_states` (is this replica PRIMARY/SECONDARY?)
2. **Sequence number** — `sys.dm_hadr_database_replica_states` (highest hardened LSN)
3. **All replica states** — Roles, sync health, connection state across the AG
4. **Database states** — Per-database synchronization, suspension status, LSN gaps
5. **AG group information** — `sys.availability_groups` (AG exists, name matches)

This provides excellent AG topology awareness — the operator knows the exact state of every
replica and database in the AG. It can detect primary loss, synchronization failures, and
replica disconnection reliably.

### What the Operator Adds at Level 2+ (sp_server_diagnostics)

When `failureConditionLevel >= 2`, the AG Helper additionally calls `sp_server_diagnostics`
on each health check cycle and evaluates the component states against the configured level.

## Failure Condition Levels

### Operator Implementation

| Level | What Triggers Failover | Equivalent To |
|-------|----------------------|---------------|
| **1** (default) | AG topology failure: role loss, no synchronized replicas, AG broken | WSFC Level 1 + partial Level 2 |
| **2** | + `sp_server_diagnostics` call fails or times out entirely | WSFC Level 2 |
| **3** | + `system` component reports error state (state = 3) | WSFC Level 3 |
| **4** | + `resource` component reports error state | WSFC Level 4 |
| **5** | + `query_processing` component reports error state | WSFC Level 5 |

### Important: What Each Level Adds (Cumulative)

Levels are cumulative — each higher level includes all checks from lower levels:

```
Level 1:  [AG Topology]
Level 2:  [AG Topology] + [Diagnostics Responsiveness]
Level 3:  [AG Topology] + [Diagnostics Responsiveness] + [System Errors]
Level 4:  [AG Topology] + [Diagnostics Responsiveness] + [System Errors] + [Resource Errors]
Level 5:  [AG Topology] + [Diagnostics Responsiveness] + [System Errors] + [Resource Errors] + [Query Processing Errors]
```

### Warning States

At any level ≥ 2, component warning states (state = 2) are reported but do **not** trigger
failover. If the AG topology baseline health is "Healthy" but a monitored component shows
a warning, the overall health is reported as "Warning". This allows operators to observe
degradation without triggering automatic failover.

## Detection Capability Matrix

### Side-by-Side Comparison

| Failure Scenario | WSFC | Operator L1 | Operator L2 | Operator L3 | Operator L5 |
|-----------------|:----:|:-----------:|:-----------:|:-----------:|:-----------:|
| SQL Server service down | ✅ | ✅ | ✅ | ✅ | ✅ |
| Pod crash / OOM kill | ✅ | ✅ | ✅ | ✅ | ✅ |
| AG role loss | ✅ | ✅ | ✅ | ✅ | ✅ |
| No synchronized replicas | ✅ | ✅ | ✅ | ✅ | ✅ |
| Replica disconnection | ✅ | ✅ | ✅ | ✅ | ✅ |
| TDS connection timeout | ✅ | ✅¹ | ✅ | ✅ | ✅ |
| Complete SQL Server hang | ✅² | ⚠️³ | ⚠️³ | ⚠️³ | ⚠️³ |
| sp_server_diagnostics unresponsive | ✅ | ❌ | ✅ | ✅ | ✅ |
| Spinlock contention / access violations | ✅ | ❌ | ❌ | ✅ | ✅ |
| Out-of-memory (SQL internal) | ✅ | ❌ | ❌ | ✅ | ✅ |
| Memory/scheduler pressure | ✅ | ❌ | ❌ | ❌ | ✅⁴ |
| Runaway queries / blocking | ✅ | ❌ | ❌ | ❌ | ✅ |
| I/O subsystem latency | ⚠️⁵ | ❌ | ℹ️⁶ | ℹ️⁶ | ℹ️⁶ |

**Notes:**

1. **¹** Via staleness detection — if the AG Helper cannot reach SQL Server for longer than
   `stalenessThreshold` (default 30s), both probes return 503.
2. **²** WSFC detects hangs via the preemptive-thread lease timeout — this is the key
   architectural difference (see [Architecture section](#architecture-over-tds-vs-preemptive-thread)).
3. **³** A complete SQL Server hang will eventually be detected via TDS connection timeout
   and staleness threshold, but detection may be slower than WSFC's lease timeout.
   Worst case: `connectionTimeout` (30s) + `stalenessThreshold` (30s) = ~60s.
4. **⁴** At level 4, not 5. Level 5 adds query_processing on top of level 4's resource checks.
5. **⁵** WSFC reports I/O diagnostics but does not trigger failover based on them.
6. **⁶** I/O subsystem data is collected and available in the `/state` response but is not
   evaluated for failover decisions at any level.

## How sp_server_diagnostics Works

### The Stored Procedure

`sp_server_diagnostics` is a SQL Server system stored procedure that returns health
information about five internal components. When called without a `REPEAT_INTERVAL`
parameter, it returns a single snapshot (one row per component) and exits.

### Components

| Component | component_type | What It Monitors |
|-----------|:--------------:|------------------|
| `system` | 0 | Spinlocks, severe access violations, non-yielding tasks, out-of-memory conditions |
| `resource` | 1 | Physical and virtual memory consumption, buffer pool, scheduler overload |
| `query_processing` | 2 | Worker threads, long-running queries, blocking chains, CPU utilization |
| `io_subsystem` | 3 | I/O latency, pending I/O count, stalled I/O |
| `events` | 4 | Ring buffer events, error log entries, extended events |

### State Values

| State | Meaning | Action |
|:-----:|---------|--------|
| 0 | Unknown | Logged; no failover action |
| 1 | Clean (healthy) | No action |
| 2 | Warning | Reported as "Warning" health; no failover |
| 3 | Error | **Triggers failover** if component is at or below configured level |

### Example Output

```
component_type | component_name     | state | state_desc | data (XML)
0              | system             | 1     | clean      | <system .../>
1              | resource           | 1     | clean      | <resource .../>
2              | query_processing   | 1     | clean      | <queryProcessing .../>
3              | io_subsystem       | 1     | clean      | <ioSubsystem .../>
4              | events             | 1     | clean      | <events .../>
```

### How the AG Helper Calls It

The AG Helper calls `EXEC sp_server_diagnostics` (non-repeat mode) via a standard
TDS connection. It reads the `component_name` and `state` columns from each row,
discarding the XML `data` column to minimize overhead.

```go
// Simplified — see cmd/ag-helper/main.go for full implementation
rows, err := db.QueryContext(ctx, "EXEC sp_server_diagnostics")
for rows.Next() {
    rows.Scan(&componentType, &componentName, &state, &stateDesc, &data)
    diagnostics.Components = append(diagnostics.Components, ComponentState{
        Name:  componentName,
        State: state,
    })
}
```

## Architecture: Over-TDS vs Preemptive Thread

### The Key Difference

| Aspect | WSFC | Kubernetes Operator |
|--------|------|---------------------|
| **How sp_server_diagnostics runs** | Dedicated session on a preemptive (OS-scheduled) thread | Standard TDS connection over TCP |
| **Can detect complete scheduler hang?** | ✅ Yes — preemptive thread bypasses SQL scheduler | ⚠️ Indirectly — TDS timeout + staleness threshold |
| **Detection latency for total hang** | ~20s (lease timeout) | ~60s (connectionTimeout + stalenessThreshold) |
| **Can detect partial degradation?** | ✅ Yes | ✅ Yes (identical: same stored procedure) |

### Why This Matters

In the vast majority of failure scenarios — component errors, resource pressure, query
processing issues — the operator's over-TDS approach provides **identical detection** to
WSFC because the same `sp_server_diagnostics` procedure returns the same component states.

The difference only manifests in a **complete SQL Server scheduler hang** where no threads
can execute any work, including TDS requests. In this extreme scenario:

- **WSFC** detects via the preemptive-thread lease timeout (~20s)
- **Operator** detects via TDS connection timeout + staleness threshold (~60s)

This 40-second gap is the primary trade-off. However, complete scheduler hangs are rare in
practice — they typically indicate severe hardware issues or kernel-level problems that would
also affect Kubernetes pod health detection.

### Mitigation

The operator's staleness detection (`stalenessThreshold`) provides a built-in backstop:
if the AG Helper cannot complete any SQL query within the threshold, both Kubernetes probes
return 503, and the controller treats the pod as stale (excluded from failover evaluation).
This ensures that even a total TDS hang results in eventual failover, with the worst-case
detection time being:

```
Worst-case detection = connectionTimeout + stalenessThreshold + NoPrimaryGracePeriod
                     = 30s + 30s + 30s = ~90s
```

For most environments, this is an acceptable trade-off compared to the operational complexity
of running Windows-based WSFC clusters.

## Choosing a Failure Condition Level

### Recommendations

| Environment | Recommended Level | Rationale |
|-------------|:-----------------:|-----------|
| Development / Testing | 1 (default) | No need for internal diagnostics; topology checks are sufficient |
| Standard HA | 1 or 3 | Level 3 catches system-level errors (spinlocks, OOM) with minimal false positives |
| Mission-Critical HA | 3 | Good balance of detection coverage and stability |
| Maximum Detection | 5 | Catches resource and query processing issues; risk of more frequent failovers under heavy load |

### Trade-offs by Level

| Level | Additional Detection | Risk |
|-------|---------------------|------|
| 1 | None (topology only) | May miss SQL-internal failures that don't affect AG topology |
| 2 | Diagnostics call failure | Very low risk — only fires if `sp_server_diagnostics` itself breaks |
| 3 | System component errors | Low risk — system errors are severe and rare (spinlocks, OOM) |
| 4 | Resource pressure | Medium risk — resource warnings can be transient under high load |
| 5 | Query processing errors | Higher risk — blocking and long queries may be normal workload |

### When NOT to Increase the Level

- **Transient spikes are expected** — Level 4+ may trigger failover during legitimate peak load
- **Read-heavy secondary workloads** — Query processing warnings on secondaries can be normal
- **Development environments** — Level 1 is sufficient; higher levels add complexity without benefit
- **With very aggressive `healthCheckTimeout`** — Combining short timeouts with high levels increases false positive risk

## Configuration

### CRD Configuration

```yaml
apiVersion: mssql.microsoft.com/v1alpha1
kind: SQLServerAG
spec:
  failover:
    automatic: true
    failureConditionLevel: 3   # 1-5, default: 1 (omit for topology-only)
```

### Runtime Behavior

When the operator processes the `failureConditionLevel` field:

1. The value is passed to the AG Helper sidecar as the `-failure-condition-level` CLI flag
   (or `FAILURE_CONDITION_LEVEL` environment variable)
2. At level 1 or when omitted, the AG Helper skips `sp_server_diagnostics` — existing
   behavior is unchanged
3. At level 2+, the AG Helper calls `sp_server_diagnostics` on every monitoring cycle
   alongside the standard DMV queries
4. The AG Helper evaluates component states against the configured level and adjusts
   the overall health accordingly
5. The controller sees the updated health state and diagnostics data in the `/state` response
   and logs any degraded components

### Verifying Configuration

```bash
# Check the configured level in the CR
kubectl get sqlserverag production-ag -n mssql -o jsonpath='{.spec.failover.failureConditionLevel}'

# Check AG Helper startup logs for level confirmation
kubectl logs sql-ag-0 -n mssql -c ag-helper | grep "failure condition level"

# Check diagnostics in /state response
kubectl exec -it sql-ag-0 -n mssql -c ag-helper -- \
  curl -s localhost:8080/state | jq '.diagnostics'
```

## Monitoring Diagnostics

### In the /state Response

When `failureConditionLevel >= 2`, the `/state` response includes a `diagnostics` field:

```json
{
  "agName": "ProductionAG",
  "role": "PRIMARY",
  "health": "Healthy",
  "diagnostics": {
    "components": [
      {"name": "system", "state": 1},
      {"name": "resource", "state": 1},
      {"name": "query_processing", "state": 1},
      {"name": "io_subsystem", "state": 1},
      {"name": "events", "state": 1}
    ],
    "collectedAt": "2026-01-30T10:30:00Z"
  }
}
```

### Controller Logging

The AG controller logs diagnostics information for each pod:

- **Diagnostics errors:** If `sp_server_diagnostics` fails, the error is logged at Warning level
- **Degraded components:** Components with state ≥ 2 are logged at Warning level with the
  component name and state value

### Prometheus Metrics

When metrics are enabled, the AG Helper exports additional diagnostics metrics:

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `mssql_server_diagnostics_component_state` | Gauge | `component` | Component state (0-3) |
| `mssql_server_diagnostics_collection_duration_seconds` | Histogram | | Time to collect diagnostics |
| `mssql_server_diagnostics_errors_total` | Counter | | Number of diagnostics collection failures |

## Related Documentation

- [Failover Management](failover-management.md) — Failover configuration and behavior
- [AG Helper Reference](ag-helper-reference.md) — CLI flags, HTTP API, environment variables
- [Controller Workflow Details](controller-workflow-details.md) — End-to-end AG monitoring workflow
- [Sidecar Architecture](../architecture/sidecar-architecture.md) — Pod container design
- [HA with Diagnostics Sample](../../samples/sql-ag-ha-diagnostics/) — Ready-to-use deployment manifest
