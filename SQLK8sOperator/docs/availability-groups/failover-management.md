# Failover Management

[← Back to Availability Groups](overview.md) | [Documentation Home](../README.md)

Guide to configuring and managing failover for SQL Server Availability Groups.

## Table of Contents

- [Automatic Failover](#automatic-failover)
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
│  5. Service Update                                              │
│     └─ Pod labels updated (role: primary)                      │
│     └─ Primary service reroutes traffic                        │
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

1. **Synchronization State**: SYNCHRONIZED replicas preferred
2. **Hardened LSN**: Highest log sequence number (least data loss)
3. **Health Status**: Only HEALTHY replicas considered
4. **Data Loss Policy**: Respects `dataLossThreshold` setting

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

- [Multi-AG Scenarios](multi-ag-scenarios.md) - Multiple AGs
- [AG Helper Reference](ag-helper-reference.md) - Complete API
- [Troubleshooting](../user-guide/troubleshooting.md) - Failover issues
