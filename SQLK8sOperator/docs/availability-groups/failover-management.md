# Failover Management

[← Back to Availability Groups](overview.md) | [Documentation Home](../README.md)

Guide to configuring and managing failover for SQL Server Availability Groups.

## Table of Contents

- [Automatic Failover](#automatic-failover)
- [Failure Condition Levels](#failure-condition-levels)
- [Manual Failover](#manual-failover)
- [Failover Selection Algorithm](#failover-selection-algorithm)
- [Cooldown and Flapping Prevention](#cooldown-and-flapping-prevention)
- [Failover Events](#failover-events)
- [Disabling Automatic Failover](#disabling-automatic-failover)

## Automatic Failover

By default, `automaticFailover` is set to `false` (monitoring only with manual failover).
When explicitly enabled, the operator automatically detects primary failure and promotes the best secondary replica.

### How It Works

```
┌────────────────────────────────────────────────────────────────┐
│                 Automatic Failover Sequence                     │
├────────────────────────────────────────────────────────────────┤
│                                                                 │
│  1. Primary Detection                                           │
│     └─ Controller monitors pod status every 10s                 │
│                                                                 │
│  2. Failure Detected                                            │
│     └─ Primary pod NotReady or deleted                         │
│     └─ Grace period: 30s                                       │
│                                                                 │
│  3. Candidate Selection                                         │
│     └─ Query /sequence on all secondaries                      │
│     └─ Select replica with highest hardened LSN                │
│                                                                 │
│  4. Failover Execution                                          │
│     └─ POST /failover to selected secondary                    │
│     └─ AG Helper promotes to primary                           │
│                                                                 │
│  5. Endpoint Update                                            │
│     └─ Listener Endpoints updated to new primary pod IP         │
│     └─ Listener service reroutes traffic                        │
│                                                                 │
│  6. Cooldown                                                    │
│     └─ No failover for 60s to prevent flapping                 │
│                                                                 │
└────────────────────────────────────────────────────────────────┘
```

### Configuration

```yaml
apiVersion: mssql.microsoft.com/v1alpha1
kind: SQLServerAG
spec:
  availabilityGroup:
    # Enable automatic failover (defaults to false for monitoring-only)
    automaticFailover: true
  
  failover:
    automatic: true
    
    # Data loss threshold (0 = no data loss allowed)
    # Only sync replicas are eligible for failover
    dataLossThreshold: 0
    
    # Time to wait before declaring primary lost
    healthCheckTimeout: "30s"
    
    # External write lease validity
    leaseTimeout: "60s"
    
    # Required synchronized secondaries (-1 = auto-calculate)
    requiredSynchronizedSecondaries: -1
    
    # Failure condition level (optional, default: 1)
    # Controls which SQL Server internal health signals trigger failover.
    # See "Failure Condition Levels" section below.
    failureConditionLevel: 3  # 1-5
```

### Failover Timeline

| Time | Event |
|------|-------|
| T+0s | Primary pod failure detected |
| T+30s | Health check timeout expires |
| T+31s | Query LSN from all secondaries |
| T+32s | POST /failover to best candidate |
| T+35s | New primary confirmed |
| T+36s | Service labels updated |
| T+96s | Cooldown period ends |

## Failure Condition Levels

The `failureConditionLevel` field controls which SQL Server internal health signals
(via `sp_server_diagnostics`) trigger automatic failover. This is modeled after the
[WSFC FAILURE_CONDITION_LEVEL](https://learn.microsoft.com/en-us/sql/sql-server/failover-clusters/windows/configure-failureconditionlevel-property-settings)
setting.

### Level Summary

| Level | Triggers Failover On | Recommended For |
|:-----:|----------------------|-----------------|
| **1** (default) | AG topology failure only (role loss, no sync, AG broken) | Dev/test, monitoring-only setups |
| **2** | + `sp_server_diagnostics` call fails entirely | Conservative HA |
| **3** | + `system` component error (spinlock, OOM, access violation) | Standard HA ✅ |
| **4** | + `resource` component error (memory/scheduler pressure) | Mission-critical |
| **5** | + `query_processing` component error (runaway queries, blocking) | Maximum detection |

Levels are **cumulative** — level 3 includes all checks from levels 1 and 2.

### How It Works

When `failureConditionLevel >= 2`:

1. The AG Helper sidecar calls `sp_server_diagnostics` on every health check cycle
   alongside the standard DMV queries
2. Each SQL Server component returns a state: `1` (clean), `2` (warning), or `3` (error)
3. The AG Helper evaluates component states against the configured level:
   - **Level 2**: Checks if the diagnostics call itself succeeded
   - **Level 3**: Also checks the `system` component for error state
   - **Level 4**: Also checks the `resource` component
   - **Level 5**: Also checks the `query_processing` component
4. If a relevant component reports error state (3), health becomes **Critical** → triggering failover
5. If a relevant component reports warning state (2) but baseline is Healthy, health becomes **Warning** (no failover)

### Configuration Example

```yaml
failover:
  automatic: true
  failureConditionLevel: 3
```

When omitted or set to `1`, the AG Helper does not call `sp_server_diagnostics` —
behavior is identical to previous versions.

### Warning vs Error States

| Component State | Effect on Health | Triggers Failover? |
|:---------------:|------------------|:------------------:|
| 1 (clean) | No change | No |
| 2 (warning) | Health → "Warning" if baseline was "Healthy" | No |
| 3 (error) | Health → "Critical" if component is at configured level | **Yes** |

### Important Caveats

- **Over-TDS limitation:** The operator calls `sp_server_diagnostics` via a standard TDS
  connection, unlike WSFC which uses a preemptive thread. A complete SQL Server scheduler
  hang may prevent the diagnostics call from completing. The staleness threshold provides
  a backstop for this scenario.
- **Higher levels = more sensitive:** Levels 4 and 5 may trigger failover during legitimate
  peak workloads. Test thoroughly before deploying in production.
- **Backward compatible:** Omitting this field preserves the exact pre-existing behavior
  (level 1: AG topology checks only).

For a detailed comparison of WSFC vs operator health detection, see the
[Health Detection Comparison](health-detection-comparison.md) guide.

## Manual Failover

Trigger failover manually via the AG Helper API or kubectl.

### Using AG Helper API

```bash
# Failover without data loss (sync replicas only)
kubectl exec -it sql-ag-prod01-1 -n mssql -c ag-helper -- \
  curl -X POST localhost:8080/failover -d '{"allowDataLoss": false}'

# Force failover with potential data loss
kubectl exec -it sql-ag-prod01-1 -n mssql -c ag-helper -- \
  curl -X POST localhost:8080/failover -d '{"allowDataLoss": true, "force": true}'
```

### Failover Response

```json
{
  "success": true,
  "previousPrimary": "sql-ag-prod01-0",
  "newPrimary": "sql-ag-prod01-1",
  "dataLoss": false,
  "message": "Failover completed successfully"
}
```

### Pre-Failover Checks

Before initiating failover:

```bash
# Check current primary
kubectl exec -it sql-ag-prod01-0 -n mssql -c ag-helper -- \
  curl -s localhost:8080/role

# Check sync state of target
kubectl exec -it sql-ag-prod01-1 -n mssql -c ag-helper -- \
  curl -s localhost:8080/state | jq '.synchronizationState'

# Check LSN on all replicas
for i in 0 1 2; do
  echo "Pod $i:"
  kubectl exec -it sql-ag-prod01-$i -n mssql -c ag-helper -- \
    curl -s localhost:8080/sequence
done
```

## Failover Selection Algorithm

When multiple secondaries are available, the operator selects the best candidate:

### Selection Criteria (in order)

1. **Data Staleness**: Replicas with `dataStale=true` are immediately excluded — their role and sync state cannot be trusted
2. **Synchronization State**: SYNCHRONIZED replicas preferred
3. **Hardened LSN**: Highest log sequence number (least data loss)
4. **Health Status**: Only HEALTHY replicas considered
5. **Data Loss Policy**: Respects `dataLossThreshold` setting

> **Staleness and failover timing:** When a pod's sidecar reports stale data, the controller
> skips it for failover candidate evaluation. This means a pod with stale data that was formerly
> the primary will not be counted as "having a primary", which accelerates failover detection.
> Worst-case failover detection is bounded at ~70s (10s poll miss + 30s staleness threshold +
> 30s `NoPrimaryGracePeriod`). Without staleness detection, a permanently stale "Healthy" response
> would prevent failover indefinitely.

### Selection Flowchart

```
┌─────────────────────────────────────────────────────────────┐
│               Failover Candidate Selection                   │
├─────────────────────────────────────────────────────────────┤
│                                                              │
│  Get all secondary replicas                                  │
│              │                                               │
│              ▼                                               │
│  ┌─────────────────────────────────┐                        │
│  │ Filter: dataStale == false      │                        │
│  │  (stale pods are excluded)      │                        │
│  └─────────────────┬───────────────┘                        │
│                    │                                         │
│                    ▼                                         │
│  ┌─────────────────────────────────┐                        │
│  │ Filter: Health == HEALTHY       │                        │
│  │         or Health == WARNING    │                        │
│  └─────────────────┬───────────────┘                        │
│                    │                                         │
│                    ▼                                         │
│  ┌─────────────────────────────────┐                        │
│  │ If dataLossThreshold == 0:      │                        │
│  │   Filter: SyncState == SYNC     │                        │
│  └─────────────────┬───────────────┘                        │
│                    │                                         │
│                    ▼                                         │
│  ┌─────────────────────────────────┐                        │
│  │ Sort by: HardenedLSN DESC       │                        │
│  │ (highest LSN = least data loss) │                        │
│  └─────────────────┬───────────────┘                        │
│                    │                                         │
│                    ▼                                         │
│  Return first candidate (or error if none)                   │
│                                                              │
└─────────────────────────────────────────────────────────────┘
```

## Cooldown and Flapping Prevention

### Cooldown Period

After a failover, the operator enters a cooldown period to prevent rapid failovers:

| Setting | Default | Description |
|---------|---------|-------------|
| Cooldown duration | 60 seconds | Time before next failover allowed |
| Grace period | 30 seconds | Time to wait before declaring failure |

### Flapping Prevention

If a replica repeatedly fails:
1. Exponential backoff increases cooldown
2. Events recorded for investigation
3. Alert fired via Prometheus

## Failover Events

The operator records Kubernetes events during failover:

```bash
kubectl get events -n mssql --field-selector reason=Failover

# Example events:
# Normal   PrimaryLost        Primary replica sql-ag-prod01-0 is not ready
# Normal   FailoverStarted    Initiating failover to sql-ag-prod01-1
# Normal   FailoverCompleted  Automatic failover completed to sql-ag-prod01-1
```

### Event Types

| Event | Reason | Description |
|-------|--------|-------------|
| Normal | PrimaryLost | Primary detected as unavailable |
| Normal | FailoverStarted | Failover process initiated |
| Normal | FailoverCompleted | New primary confirmed |
| Warning | FailoverFailed | Failover attempt failed |
| Warning | ForceFailover | Failover with potential data loss |
| Normal | CooldownActive | Failover blocked by cooldown |

## Disabling Automatic Failover

For manual-only failover control:

```yaml
apiVersion: mssql.microsoft.com/v1alpha1
kind: SQLServerAG
spec:
  availabilityGroup:
    automaticFailover: false  # Disable automatic failover
  
  failover:
    automatic: false
```

With automatic failover disabled:
- Operator monitors but doesn't act
- Use AG Helper API for manual failover
- Suitable for maintenance windows

### Planned Maintenance Example

```bash
# 1. Disable automatic failover
kubectl patch sqlserverag prod-ag-01 -n mssql --type merge \
  -p '{"spec":{"availabilityGroup":{"automaticFailover":false}}}'

# 2. Manually failover before maintenance
kubectl exec -it sql-ag-prod01-1 -n mssql -c ag-helper -- \
  curl -X POST localhost:8080/failover

# 3. Perform maintenance on old primary

# 4. Re-enable automatic failover
kubectl patch sqlserverag prod-ag-01 -n mssql --type merge \
  -p '{"spec":{"availabilityGroup":{"automaticFailover":true}}}'
```

## Next Steps

- [Health Detection Comparison](health-detection-comparison.md) - WSFC vs operator health model
- [AG Operations Guide](../operations/ag-operations.md) - Quick reference kubectl commands
- [Multi-AG Scenarios](multi-ag-scenarios.md) - Multiple AGs
- [AG Helper Reference](ag-helper-reference.md) - Complete API
- [Troubleshooting](../user-guide/troubleshooting.md) - Failover issues
