# SQL Server AG — High Availability with Server Diagnostics

Self-contained sample that deploys a **3-replica synchronous-commit Availability Group**
with automatic failover, an AG Listener, and **sp_server_diagnostics-based health detection**
at failure condition level 3.

## What Makes This Different from sql-ag-ha

The standard [sql-ag-ha](../sql-ag-ha/) sample monitors AG topology health only — it detects
when a primary pod is down or when AG replicas lose synchronization. This sample adds
**SQL Server internal health monitoring** via `sp_server_diagnostics`, which catches failures
that are invisible to topology checks alone:

| Detection Layer | sql-ag-ha (Level 1) | This Sample (Level 3) |
|-----------------|:-------------------:|:---------------------:|
| Pod down / unreachable | ✅ | ✅ |
| AG role loss | ✅ | ✅ |
| No synchronized replicas | ✅ | ✅ |
| sp_server_diagnostics unresponsive | ❌ | ✅ |
| System errors (spinlock, OOM, access violations) | ❌ | ✅ |
| Resource pressure (memory/scheduler) | ❌ | ❌ (Level 4) |
| Query processing errors (runaway queries) | ❌ | ❌ (Level 5) |

For a full comparison of failure condition levels and how they map to Microsoft's WSFC
model, see the [Health Detection Comparison](../../docs/availability-groups/health-detection-comparison.md) guide.

## Scenario

| Property | Value |
|----------|-------|
| Replicas | 3 (sql-ag-0, sql-ag-1, sql-ag-2) |
| Sync mode | All synchronous commit |
| Failover | Automatic (controller-managed) |
| Listener | Yes (single VIP routing to primary) |
| Read-only routing | Secondaries accept read-only connections |
| Failure condition level | 3 (AG topology + system diagnostics) |
| sp_server_diagnostics | Active on every health check cycle |

## Files

| File | Description |
|------|-------------|
| `ag-deploy.yaml` | All Kubernetes resources (namespace, secrets, SQLServer, services, SQLServerAG with `failureConditionLevel: 3`) |
| `ag-configure.md` | Step-by-step T-SQL guide to create the AG inside SQL Server |
| `ag-configure.sh` | Automated shell script version of the T-SQL guide |

## Quick Start

### 1. Install the Operator

```bash
kubectl apply -f https://raw.githubusercontent.com/tejasaks/PublicDemos/main/SQLK8sOperator/install.yaml
```

### 2. Deploy Kubernetes Resources

```bash
kubectl apply -f ag-deploy.yaml
kubectl get pods -n mssql -w
# Wait for sql-ag-0, sql-ag-1, sql-ag-2 to be Running
```

### 3. Configure the AG in SQL Server

**Option A — Automated:**

```bash
chmod +x ag-configure.sh
./ag-configure.sh
```

**Option B — Manual (step-by-step):**

Follow [ag-configure.md](ag-configure.md).

### 4. Set Up the AG Listener

```bash
./ag-configure.sh listener
```

Or follow Step 9 in [ag-configure.md](ag-configure.md).

### 5. Verify

```bash
# Kubernetes status
kubectl get sqlserverag -n mssql
kubectl get sqlserver -n mssql

# AG health via T-SQL
./ag-configure.sh verify
```

### 6. Verify Server Diagnostics

```bash
# Check that diagnostics are being collected
kubectl exec -it sql-ag-0 -n mssql -c ag-helper -- \
  curl -s localhost:8080/state | jq '.diagnostics'

# Expected output:
# {
#   "components": [
#     {"name": "system", "state": 1},
#     {"name": "resource", "state": 1},
#     {"name": "query_processing", "state": 1},
#     {"name": "io_subsystem", "state": 1},
#     {"name": "events", "state": 1}
#   ],
#   "collectedAt": "2026-01-30T10:30:00Z"
# }
```

## Understanding Server Diagnostics Output

The `diagnostics` field in the `/state` response contains the output of SQL Server's
`sp_server_diagnostics` stored procedure. Each component has a state value:

| State | Meaning | Failover Impact |
|-------|---------|-----------------|
| 0 | Unknown | Treated as potential issue |
| 1 | Clean (healthy) | No impact |
| 2 | Warning | Health reported as "Warning" if baseline was "Healthy" |
| 3 | Error | **Triggers automatic failover** at the configured level |

### Component Descriptions

| Component | What It Monitors | Level Required |
|-----------|------------------|:--------------:|
| `system` | Spinlocks, severe access violations, non-yielding tasks, out-of-memory | 3 |
| `resource` | Memory pressure, scheduler overload, I/O resource exhaustion | 4 |
| `query_processing` | Runaway queries, persistent blocking, worker thread exhaustion | 5 |
| `io_subsystem` | I/O latency and error rates | Informational only |
| `events` | Ring buffer events and extended events | Informational only |

> **Note:** At level 3, only the `system` component is evaluated for failover decisions.
> The `resource`, `query_processing`, `io_subsystem`, and `events` components are still
> collected and reported in the `/state` response — their warning states will cause the
> health to be reported as "Warning" but will not trigger failover.

## Changing the Failure Condition Level

To adjust sensitivity, edit `ag-deploy.yaml` and change `failureConditionLevel`:

```yaml
failover:
  # Level 1: AG topology only (most conservative)
  # Level 3: + system errors (recommended for most HA deployments)
  # Level 5: + query processing (most aggressive)
  failureConditionLevel: 3
```

Then reapply:

```bash
kubectl apply -f ag-deploy.yaml
```

The AG Helper sidecar will pick up the new level on the next pod restart. To force
an immediate restart:

```bash
kubectl rollout restart statefulset sql-ag -n mssql
```

## Customization

> **Passwords:** The sample manifests and scripts ship with placeholder passwords (e.g. `YourStrong@Passw0rd!`). Before deploying, open `ag-deploy.yaml` and change the SA password and AG Helper password in the Secret resources. Then update the matching values at the top of `ag-configure.sh` (`SA_PASSWORD`, `AG_HELPER_PASSWORD`, `MASTER_KEY_PASSWORD`, `REPLICA_LOGIN_PASSWORD`), or in the manual T-SQL steps in `ag-configure.md`.

> **Instance and AG names:** You can rename the SQLServer resource (e.g. `sql-ag` → `my-sql`), the pod prefix, the AG name (`ProductionAG`), and the listener name. If you do, update them consistently in:
> - `ag-deploy.yaml` — SQLServer `.metadata.name`, SQLServerAG `.metadata.name`, `.spec.availabilityGroup.name`, `.spec.listener.name`, and the per-replica Service names
> - `ag-configure.sh` — the `PRIMARY`, `REPLICAS`, `AG_NAME`, `AG_RESOURCE_NAME`, `AG_LISTENER_NAME`, and `DATABASE_NAME` variables at the top
> - `ag-configure.md` — the pod names and AG name referenced in every T-SQL command

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│  Namespace: mssql                                                       │
│                                                                         │
│  ┌──────────────────┐  ┌──────────────────┐  ┌──────────────────┐      │
│  │  sql-ag-0        │  │  sql-ag-1        │  │  sql-ag-2        │      │
│  │  (PRIMARY)       │  │  (SECONDARY)     │  │  (SECONDARY)     │      │
│  │  Sync Commit     │  │  Sync Commit     │  │  Sync Commit     │      │
│  │  :1433 :5022     │  │  :1433 :5022     │  │  :1433 :5022     │      │
│  │                  │  │                  │  │                  │      │
│  │  AG Helper       │  │  AG Helper       │  │  AG Helper       │      │
│  │  ├─ DMV queries  │  │  ├─ DMV queries  │  │  ├─ DMV queries  │      │
│  │  └─ sp_server_   │  │  └─ sp_server_   │  │  └─ sp_server_   │      │
│  │     diagnostics  │  │     diagnostics  │  │     diagnostics  │      │
│  └────────┬─────────┘  └────────┬─────────┘  └────────┬─────────┘      │
│           │                     │                     │                 │
│           └─────────┬───────────┴───────────┬─────────┘                 │
│                     │                       │                           │
│            ┌────────▼──────┐       ┌────────▼────────┐                 │
│            │ AG Listener   │       │ AG Controller   │                 │
│            │ (VIP→primary  │       │ (failover on AG │                 │
│            │  :1433)       │       │  topology OR    │                 │
│            └───────────────┘       │  system errors) │                 │
│                                    └─────────────────┘                 │
│                                                                         │
│  Operator: automatic failover, status tracking, diagnostics logging     │
└─────────────────────────────────────────────────────────────────────────┘
```

## Useful Commands

```bash
# Check which replica is primary
kubectl get sqlserverag production-ag -n mssql -o jsonpath='{.status.primaryReplica}'

# Check server diagnostics on primary
kubectl exec -it sql-ag-0 -n mssql -c ag-helper -- \
  curl -s localhost:8080/state | jq '.diagnostics'

# Check AG Helper logs for diagnostics activity
kubectl logs sql-ag-0 -n mssql -c ag-helper | grep -i "diagnostics\|failure.condition"

# Manual failover to sql-ag-1
kubectl annotate sqlserverag production-ag -n mssql \
  mssql.microsoft.com/failover-to=sql-ag-1

# Get listener VIP
kubectl get sqlserverag production-ag -n mssql -o jsonpath='{.status.listener.vip}'

# View events
kubectl get events -n mssql --sort-by='.lastTimestamp' | tail -20
```

## Cleanup

```bash
kubectl delete -f ag-deploy.yaml
kubectl delete namespace mssql

# To remove the operator:
kubectl delete -f https://raw.githubusercontent.com/tejasaks/PublicDemos/main/SQLK8sOperator/install.yaml
```

## Related Documentation

- [Health Detection Comparison](../../docs/availability-groups/health-detection-comparison.md) — WSFC vs operator health model
- [Failover Management](../../docs/availability-groups/failover-management.md) — Failover configuration and levels
- [AG Helper Reference](../../docs/availability-groups/ag-helper-reference.md) — CLI flags, HTTP API, diagnostics
- [Controller Workflow Details](../../docs/availability-groups/controller-workflow-details.md) — End-to-end workflow
