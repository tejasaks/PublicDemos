# AG Helper and Controller Workflow Details

[← Back to Availability Groups](overview.md) | [Architecture Overview](../architecture/overview.md) | [Documentation Home](../README.md)

This document provides an in-depth explanation of how the AG Helper sidecar and SQLServerAG Controller work together to monitor and manage Availability Groups. This is essential reading for developers working on the operator codebase.

## Table of Contents

- [Architecture Overview](#architecture-overview)
- [Component Responsibilities](#component-responsibilities)
- [Part 1: AG Helper - Data Collection](#part-1-ag-helper---data-collection)
- [Part 2: AG Helper - Health Determination](#part-2-ag-helper---health-determination)
- [Part 3: AG Helper - HTTP API](#part-3-ag-helper---http-api)
- [Part 4: Controller - State Aggregation](#part-4-controller---state-aggregation)
- [Part 5: Controller - Failover Decision Logic](#part-5-controller---failover-decision-logic)
- [Part 6: Failover Execution](#part-6-failover-execution)
- [Part 7: Complete Scenario Walkthrough](#part-7-complete-scenario-walkthrough)
- [Summary: Data Flow Diagram](#summary-data-flow-diagram)
- [Key Takeaways](#key-takeaways)

## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                         KUBERNETES CLUSTER                                   │
│                                                                              │
│  ┌─────────────────────────────────────────────────────────────────────────┐│
│  │                    CONTROLLER (mssql-system)                            ││
│  │                                                                         ││
│  │  SQLServerAGReconciler                                                  ││
│  │  ┌─────────────────────┐                                                ││
│  │  │ Reconcile Loop      │◄─── Triggered every 10s (configurable)        ││
│  │  │ (monitor interval)  │                                                ││
│  │  └─────────┬───────────┘                                                ││
│  │            │                                                            ││
│  │            │ HTTP GET /state (queries each pod)                         ││
│  │            ▼                                                            ││
│  └─────────────────────────────────────────────────────────────────────────┘│
│                                    │                                         │
│            ┌───────────────────────┼───────────────────────┐                │
│            │                       │                       │                │
│            ▼                       ▼                       ▼                │
│  ┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐         │
│  │ Pod: sql-ag-0   │    │ Pod: sql-ag-1   │    │ Pod: sql-ag-2   │         │
│  │ ┌─────────────┐ │    │ ┌─────────────┐ │    │ ┌─────────────┐ │         │
│  │ │ SQL Server  │ │    │ │ SQL Server  │ │    │ │ SQL Server  │ │         │
│  │ │  (PRIMARY)  │ │    │ │ (SECONDARY) │ │    │ │ (SECONDARY) │ │         │
│  │ └──────┬──────┘ │    │ └──────┬──────┘ │    │ └──────┬──────┘ │         │
│  │        │        │    │        │        │    │        │        │         │
│  │        │SQL     │    │        │SQL     │    │        │SQL     │         │
│  │        │Queries │    │        │Queries │    │        │Queries │         │
│  │        ▼        │    │        ▼        │    │        ▼        │         │
│  │ ┌─────────────┐ │    │ ┌─────────────┐ │    │ ┌─────────────┐ │         │
│  │ │ AG Helper   │ │    │ │ AG Helper   │ │    │ │ AG Helper   │ │         │
│  │ │ (Sidecar)   │ │    │ │ (Sidecar)   │ │    │ │ (Sidecar)   │ │         │
│  │ │ :8080       │ │    │ │ :8080       │ │    │ │ :8080       │ │         │
│  │ └─────────────┘ │    │ └─────────────┘ │    │ └─────────────┘ │         │
│  └─────────────────┘    └─────────────────┘    └─────────────────┘         │
│                                                                              │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Component Responsibilities

| Component | Responsibility |
|-----------|----------------|
| **AG Helper** | Per-pod sidecar that queries SQL Server DMVs, calculates health, exposes HTTP API, executes failover T-SQL |
| **Controller** | Cluster-wide orchestrator that aggregates health from all AG Helpers, makes failover decisions, triggers actions |

---

## Part 1: AG Helper - Data Collection

The AG Helper runs as a sidecar in each SQL Server pod. It connects to the **local** SQL Server instance and queries Dynamic Management Views (DMVs) to collect AG state.

### SQL Queries Executed

#### 1. Get Local Replica Role

```sql
SELECT role_desc
FROM sys.dm_hadr_availability_replica_states ars
JOIN sys.availability_replicas ar ON ars.replica_id = ar.replica_id
WHERE is_local = 1 AND ar.group_id = (
    SELECT group_id FROM sys.availability_groups WHERE name = @agName
)
```

**Returns:** `PRIMARY`, `SECONDARY`, `RESOLVING`, or `NOT_AVAILABLE`

#### 2. Get Sequence Number (LSN)

```sql
SELECT MAX(last_hardened_lsn)
FROM sys.dm_hadr_database_replica_states drs
JOIN sys.availability_groups ag ON drs.group_id = ag.group_id
WHERE ag.name = @agName AND is_local = 1
```

**Returns:** The highest hardened LSN across all databases (used to determine which replica has the least data loss during failover candidate selection)

#### 3. Get All Replica States

```sql
SELECT 
    ar.replica_server_name,
    ars.role_desc,
    ar.availability_mode_desc,
    ar.failover_mode_desc,
    ars.synchronization_health_desc,
    ars.connected_state_desc,
    ars.is_local,
    ISNULL(MAX(drs.last_hardened_lsn), 0) as seq_num
FROM sys.availability_replicas ar
JOIN sys.dm_hadr_availability_replica_states ars ON ar.replica_id = ars.replica_id
LEFT JOIN sys.dm_hadr_database_replica_states drs ON ar.replica_id = drs.replica_id
WHERE ar.group_id = (SELECT group_id FROM sys.availability_groups WHERE name = @agName)
GROUP BY ar.replica_server_name, ars.role_desc, ar.availability_mode_desc, 
         ar.failover_mode_desc, ars.synchronization_health_desc, ars.connected_state_desc, ars.is_local
```

**Returns:** State of all replicas visible from this node, including their roles, sync health, and connection state.

#### 4. Get Database States

```sql
SELECT 
    db.name,
    ar.replica_server_name,
    drs.is_primary_replica,
    drs.synchronization_state_desc,
    drs.is_suspended,
    ISNULL(drs.suspend_reason_desc, ''),
    ISNULL(drs.last_hardened_lsn, 0),
    ISNULL(drs.last_commit_lsn, 0)
FROM sys.dm_hadr_database_replica_states drs
JOIN sys.databases db ON drs.database_id = db.database_id
JOIN sys.availability_replicas ar ON drs.replica_id = ar.replica_id
WHERE drs.group_id = (SELECT group_id FROM sys.availability_groups WHERE name = @agName)
```

**Returns:** Per-database synchronization state, including suspension status and LSN values.

### Monitor Loop

The AG Helper runs a continuous monitoring loop:

```go
func (h *AGHelper) MonitorLoop(ctx context.Context) {
    ticker := time.NewTicker(h.monitorInterval)  // Default: 10 seconds
    defer ticker.Stop()

    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            // Query SQL Server DMVs
            state, err := h.GetAGState(ctx)
            if err != nil {
                klog.Errorf("Failed to get AG state: %v", err)
                continue
            }
            // Update cached state for HTTP endpoints
            h.state = state
        }
    }
}
```

---

## Part 2: AG Helper - Health Determination

After collecting data, the AG Helper applies this logic to determine overall health:

```go
func (h *AGHelper) determineHealth(state *AGState) string {
    // State 1: AG not configured yet
    if state.Role == RoleNotAvailable {
        if len(state.Replicas) == 0 {
            return "Waiting"  // AG not created yet - OK during setup
        }
        return "Critical"     // AG exists but this replica can't see it
    }

    // Count synchronized replicas
    syncedCount := 0
    for _, r := range state.Replicas {
        if r.SynchronizationState == StateSynchronized {
            syncedCount++
        }
    }

    // State 2: No synchronized replicas
    if syncedCount == 0 {
        if state.IsLocalPrimary && len(state.Databases) == 0 {
            return "Waiting"  // Primary with no databases yet
        }
        return "Critical"     // Data not synchronized anywhere
    }

    // State 3: Partial synchronization
    if syncedCount < len(state.Replicas) {
        return "Warning"      // Some replicas not synced
    }

    // State 4: All replicas synchronized
    return "Healthy"
}
```

### Health States Summary

| Health | Condition | Liveness Probe | Readiness Probe |
|--------|-----------|----------------|-----------------|
| `Healthy` | All replicas synchronized | 200 OK | 200 OK |
| `Warning` | Some replicas not synchronized | 200 OK | 200 OK |
| `Waiting` | AG not configured yet | 200 OK | 503 Unavailable |
| `Critical` | AG broken or no sync | 503 Unavailable | 503 Unavailable |

### Probe Behavior Rationale

- **Liveness (`/healthz`)**: Returns 200 even for `Waiting` state because the pod is alive and functioning—it's just waiting for AG configuration. Returning 503 would cause Kubernetes to restart a perfectly healthy pod.
- **Readiness (`/readyz`)**: Returns 503 for `Waiting` because the pod shouldn't receive traffic until the AG is actually working.

---

## Part 3: AG Helper - HTTP API

The AG Helper exposes these endpoints on port 8080:

| Endpoint | Method | Purpose | Used By |
|----------|--------|---------|---------|
| `/healthz` | GET | Kubernetes liveness probe | kubelet |
| `/readyz` | GET | Kubernetes readiness probe | kubelet |
| `/state` | GET | Full AG state | Controller |
| `/role` | GET | Current replica role | Debug/monitoring |
| `/sequence` | GET | Current LSN/sequence number | Controller |
| `/failover` | POST | Trigger failover | Controller |


### Example `/state` Response

```json
{
  "agName": "ProductionAG",
  "localReplicaName": "sql-ag-prod01-0",
  "role": "PRIMARY",
  "syncState": "SYNCHRONIZED",
  "isLocalPrimary": true,
  "sequenceNumber": 12345678,
  "health": "Healthy",
  "lastUpdated": "2026-01-30T10:30:00Z",
  "replicas": [
    {
      "replicaName": "sql-ag-prod01-0",
      "role": "PRIMARY",
      "synchronizationState": "SYNCHRONIZED",
      "connectedState": "CONNECTED",
      "isLocal": true,
      "sequenceNumber": 12345678
    },
    {
      "replicaName": "sql-ag-prod01-1",
      "role": "SECONDARY",
      "synchronizationState": "SYNCHRONIZED",
      "connectedState": "CONNECTED",
      "isLocal": false,
      "sequenceNumber": 12345678
    },
    {
      "replicaName": "sql-ag-prod01-2",
      "role": "SECONDARY",
      "synchronizationState": "SYNCHRONIZED",
      "connectedState": "CONNECTED",
      "isLocal": false,
      "sequenceNumber": 12345678
    }
  ],
  "databases": [
    {
      "databaseName": "AppDB",
      "synchronizationState": "SYNCHRONIZED",
      "isPrimaryReplica": true,
      "isSuspended": false,
      "lastHardenedLsn": 12345678,
      "lastCommitLsn": 12345678
    }
  ]
}
```

---

## Part 4: Controller - State Aggregation

The SQLServerAGReconciler runs as part of the operator and is triggered:

1. **On SQLServerAG resource changes** (create/update/delete)
2. **On a timer** (every `monitorInterval`, default 10 seconds)

### Reconciliation Flow

```go
func (r *SQLServerAGReconciler) Reconcile(ctx, req) (Result, error) {
    // 1. Get the SQLServerAG custom resource
    ag := &SQLServerAG{}
    r.Get(ctx, req.NamespacedName, ag)

    // 2. Get the referenced SQLServer resource
    sqlServer := &SQLServer{}
    r.Get(ctx, NamespacedName{Name: ag.Spec.SQLServerRef.Name}, sqlServer)

    // 3. Ensure SQLServer is ready
    if !sqlServer.Status.Ready {
        return Result{RequeueAfter: 10 * time.Second}, nil
    }

    // 4. Create/update endpoint services (primary/secondary routing)
    r.reconcileEndpoints(ctx, ag, sqlServer)

    // 5. Query all pod sidecars and update CRD status
    r.updateAGStatus(ctx, ag, sqlServer)

    // 6. Check for failover if automatic failover is enabled
    if ag.Spec.AvailabilityGroup.AutomaticFailover {
        r.checkAndHandleFailover(ctx, ag, sqlServer)
    }

    // 7. Requeue for next check
    return Result{RequeueAfter: monitorInterval}, nil
}
```

### How Controller Queries Sidecars

```go
func (r *SQLServerAGReconciler) querySidecarStates(ctx, ag, sqlServer) ([]FailoverCandidate, hasPrimary, error) {
    // 1. List all pods matching the SQLServer instance
    podList := &PodList{}
    r.List(ctx, podList, MatchingLabels{
        "mssql.microsoft.com/instance": sqlServer.Name,
    })

    var candidates []FailoverCandidate
    hasPrimary := false

    for _, pod := range podList.Items {
        // Skip non-running pods or pods without IP
        if pod.Status.Phase != PodRunning || pod.Status.PodIP == "" {
            continue
        }

        // HTTP GET http://<pod-ip>:8080/state
        state, err := r.querySidecar(ctx, pod.Status.PodIP)
        if err != nil {
            continue  // Log and skip failed pods
        }

        if state.Role == "PRIMARY" {
            hasPrimary = true
        }

        // Collect healthy secondaries as failover candidates
        if state.Role == "SECONDARY" && 
           (state.Health == "Healthy" || state.Health == "Warning") {
            candidates = append(candidates, FailoverCandidate{
                PodName:        pod.Name,
                PodIP:          pod.Status.PodIP,
                SequenceNumber: state.SequenceNumber,
                SyncState:      state.SyncState,
                Health:         state.Health,
            })
        }
    }

    return candidates, hasPrimary, nil
}
```

---

## Part 5: Controller - Failover Decision Logic

The controller uses these decision points for automatic failover:

### Decision Tree

```
                        ┌─────────────────┐
                        │ Reconcile Loop  │
                        └────────┬────────┘
                                 │
                        ┌────────▼────────┐
                        │ Query all pods  │
                        │ via /state API  │
                        └────────┬────────┘
                                 │
                    ┌────────────▼────────────┐
                    │ Is primary responding?  │
                    └────────────┬────────────┘
                           Yes   │    No
                    ┌────────────┴────────────┐
                    │                         │
              ┌─────▼─────┐          ┌────────▼────────┐
              │ Clear     │          │ Is this first   │
              │ no-primary│          │ detection?      │
              │ timer     │          └────────┬────────┘
              │ All good! │                   │
              └───────────┘             Yes   │    No
                                 ┌────────────┴────────────┐
                                 │                         │
                        ┌────────▼────────┐       ┌────────▼────────┐
                        │ Start 30s grace │       │ Has 30s grace   │
                        │ period, record  │       │ period elapsed? │
                        │ timestamp       │       └────────┬────────┘
                        └─────────────────┘          Yes   │    No
                                                ┌──────────┴──────────┐
                                                │                     │
                                       ┌────────▼────────┐   ┌────────▼────────┐
                                       │ Select best     │   │ Wait, requeue   │
                                       │ candidate       │   │ in remaining    │
                                       └────────┬────────┘   │ grace time      │
                                                │            └─────────────────┘
                                       ┌────────▼────────┐
                                       │ Trigger failover│
                                       │ via HTTP POST   │
                                       │ /failover       │
                                       └─────────────────┘
```

### Key Configuration Constants

| Constant | Value | Purpose |
|----------|-------|---------|
| `NoPrimaryGracePeriod` | 30 seconds | Wait time before triggering failover after primary loss |
| `FailoverCooldownPeriod` | 60 seconds | Minimum time between successive failovers |
| `SidecarPort` | 8080 | Port to reach AG Helper HTTP API |

### Candidate Selection Algorithm

The controller selects the best failover candidate using this priority:

```go
func selectBestCandidate(candidates []FailoverCandidate) *FailoverCandidate {
    if len(candidates) == 0 {
        return nil
    }

    best := candidates[0]
    for _, candidate := range candidates[1:] {
        // Priority 1: Highest sequence number (least data loss)
        if candidate.SequenceNumber > best.SequenceNumber {
            best = candidate
            continue
        }

        if candidate.SequenceNumber == best.SequenceNumber {
            // Priority 2: SYNCHRONIZED state over SYNCHRONIZING
            if candidate.SyncState == "SYNCHRONIZED" && 
               best.SyncState != "SYNCHRONIZED" {
                best = candidate
                continue
            }

            // Priority 3: Healthy over Warning
            if candidate.SyncState == best.SyncState &&
               candidate.Health == "Healthy" && 
               best.Health != "Healthy" {
                best = candidate
            }
        }
    }
    return best
}
```

---

## Part 6: Failover Execution

When the controller decides to failover, it calls the AG Helper on the **target secondary** pod.

### Controller → AG Helper HTTP Call

```go
func (r *SQLServerAGReconciler) triggerFailover(ctx, ag, candidate) error {
    // Determine if force failover is needed (data loss possible)
    allowDataLoss := candidate.SyncState != "SYNCHRONIZED"

    if allowDataLoss {
        r.Recorder.Event(ag, Warning, "ForceFailover",
            "Forcing failover with potential data loss")
    }

    // HTTP POST http://<candidate-pod-ip>:8080/failover
    url := fmt.Sprintf("http://%s:8080/failover", candidate.PodIP)
    payload := map[string]bool{"allowDataLoss": allowDataLoss}
    
    resp, err := r.HTTPClient.Post(url, "application/json", payload)
    if err != nil {
        return fmt.Errorf("failover request failed: %w", err)
    }

    if resp.StatusCode != 200 {
        return fmt.Errorf("failover failed with status %d", resp.StatusCode)
    }

    return nil
}
```

### AG Helper → SQL Server T-SQL

The AG Helper on the target pod receives the HTTP request and executes T-SQL:

```go
func (h *AGHelper) Failover(ctx context.Context, allowDataLoss bool) error {
    // Validate current role
    role, _ := h.GetRole(ctx)
    if role == RolePrimary {
        return nil  // Already primary, nothing to do
    }

    // Build failover command
    var failoverQuery string
    if allowDataLoss {
        // Force failover - accepts potential data loss
        failoverQuery = fmt.Sprintf(
            `ALTER AVAILABILITY GROUP [%s] FORCE_FAILOVER_ALLOW_DATA_LOSS`, 
            h.agName)
    } else {
        // Planned failover - requires synchronized state
        failoverQuery = fmt.Sprintf(
            `ALTER AVAILABILITY GROUP [%s] FAILOVER`, 
            h.agName)
    }

    // Execute on SQL Server
    _, err := h.db.ExecContext(ctx, failoverQuery)
    if err != nil {
        return fmt.Errorf("failover failed: %w", err)
    }

    klog.Info("Failover completed successfully")
    return nil
}
```

### T-SQL Commands Used

| Scenario | T-SQL Command | When Used |
|----------|---------------|-----------|
| Planned failover | `ALTER AVAILABILITY GROUP [AGName] FAILOVER` | Target is SYNCHRONIZED |
| Forced failover | `ALTER AVAILABILITY GROUP [AGName] FORCE_FAILOVER_ALLOW_DATA_LOSS` | Target is NOT synchronized |

---

## Part 7: Complete Scenario Walkthrough

Let's walk through a complete example with a 3-node AG where the primary pod fails.

### Initial State (Healthy AG)

```
Time: T+0 seconds

┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│ sql-ag-0    │    │ sql-ag-1    │    │ sql-ag-2    │
│ (PRIMARY)   │    │ (SECONDARY) │    │ (SECONDARY) │
│ Health:OK   │    │ Health:OK   │    │ Health:OK   │
│ LSN: 12345  │    │ LSN: 12345  │    │ LSN: 12345  │
└─────────────┘    └─────────────┘    └─────────────┘
       │                  │                  │
       ▼                  ▼                  ▼
Controller queries all three pods → All respond → hasPrimary=true → No action
```

### T+0: Primary Pod Crashes

```
sql-ag-0 pod terminates unexpectedly (node failure, OOM kill, etc.)
```

### T+10s: First Detection Cycle

```
Controller reconcile loop runs:

1. Query sql-ag-0:8080/state
   └─► TIMEOUT/CONNECTION REFUSED (pod is down)

2. Query sql-ag-1:8080/state
   └─► Response: {role: "RESOLVING", health: "Warning", seqNum: 12345}

3. Query sql-ag-2:8080/state  
   └─► Response: {role: "RESOLVING", health: "Warning", seqNum: 12345}

Result:
- hasPrimary = false (no pod returned role=PRIMARY)
- candidates = [
    {pod: sql-ag-1, seqNum: 12345, syncState: "SYNCHRONIZED", health: "Warning"},
    {pod: sql-ag-2, seqNum: 12345, syncState: "SYNCHRONIZED", health: "Warning"}
  ]

Decision:
- No primary detected → First time → Start grace period timer
- Record: noPrimaryDetected["mssql/prod-ag-01"] = T+10s
- Emit Event: "NoPrimaryDetected: No primary replica detected, will failover in 30s"
- Requeue: RequeueAfter = 30 seconds
```

### T+20s: Kubernetes Restarts Pod (Recovery Path)

```
If Kubernetes restarts sql-ag-0 and SQL Server comes back as PRIMARY:

Controller reconcile (scheduled or triggered by pod change):
1. Query sql-ag-0:8080/state
   └─► Response: {role: "PRIMARY", health: "Healthy", seqNum: 12346}

Result:
- hasPrimary = true

Decision:
- Clear noPrimaryDetected timer
- Emit Event: "PrimaryRecovered: Primary replica is available again"
- No failover needed!
```

### T+40s: Grace Period Elapsed (Failover Path)

```
If sql-ag-0 did NOT recover within 30 seconds:

Controller reconcile loop runs:

Check: time.Since(noPrimaryDetected) >= 30s
       40s - 10s = 30s ✓ Grace period elapsed!

1. Query remaining pods:
   - sql-ag-1: {role: "SECONDARY", health: "Warning", seqNum: 12345, syncState: "SYNCHRONIZED"}
   - sql-ag-2: {role: "SECONDARY", health: "Warning", seqNum: 12345, syncState: "SYNCHRONIZED"}

2. Select best candidate:
   - Both have same seqNum (12345) → No preference
   - Both SYNCHRONIZED → No preference
   - Both Warning health → No preference
   - Pick first one: sql-ag-1

3. Trigger failover:
   - HTTP POST http://sql-ag-1-ip:8080/failover
   - Body: {"allowDataLoss": false}  // SYNCHRONIZED = no data loss

4. AG Helper on sql-ag-1 receives request:
   - Executes: ALTER AVAILABILITY GROUP [ProductionAG] FAILOVER
   - SQL Server promotes this replica to PRIMARY
   - Returns: 200 OK {"status": "failover initiated"}

5. Controller updates:
   - Record: lastFailoverTime["mssql/prod-ag-01"] = T+40s
   - Clear: noPrimaryDetected["mssql/prod-ag-01"]
   - Emit Event: "FailoverCompleted: Automatic failover completed to sql-ag-1"
   - Update status: ag.Status.PrimaryReplica = "sql-ag-1"
   - Requeue: RequeueAfter = 5 seconds (verify failover succeeded)
```

### T+45s: Verification Cycle

```
Controller reconcile loop runs:

1. Query sql-ag-1:8080/state
   └─► Response: {role: "PRIMARY", health: "Healthy", seqNum: 12350}

2. Query sql-ag-2:8080/state
   └─► Response: {role: "SECONDARY", health: "Healthy", seqNum: 12350}

Result:
- hasPrimary = true
- AG is now healthy with new primary

Final State:
┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│ sql-ag-0    │    │ sql-ag-1    │    │ sql-ag-2    │
│   (DOWN)    │    │ (PRIMARY)   │    │ (SECONDARY) │
│             │    │ Health:OK   │    │ Health:OK   │
│             │    │ LSN: 12350  │    │ LSN: 12350  │
└─────────────┘    └─────────────┘    └─────────────┘
```

### T+120s: Original Primary Rejoins

```
When sql-ag-0 pod comes back online:

1. SQL Server starts, AG Helper starts
2. AG Helper queries SQL Server → Sees AG exists, this replica is SECONDARY
3. AG Helper reports: {role: "SECONDARY", health: "Synchronizing"}

Controller sees 3 replicas again:
- sql-ag-0: SECONDARY (was primary, now rejoined as secondary)
- sql-ag-1: PRIMARY (current primary)
- sql-ag-2: SECONDARY

After databases synchronize (few seconds to minutes depending on data):

┌─────────────┐    ┌─────────────┐    ┌─────────────┐
│ sql-ag-0    │    │ sql-ag-1    │    │ sql-ag-2    │
│ (SECONDARY) │    │ (PRIMARY)   │    │ (SECONDARY) │
│ Health:OK   │    │ Health:OK   │    │ Health:OK   │
│ LSN: 12400  │    │ LSN: 12400  │    │ LSN: 12400  │
└─────────────┘    └─────────────┘    └─────────────┘

Full AG recovered with 3 synchronized replicas!
```

---

## Summary: Data Flow Diagram

```
                    ┌──────────────────────────────────────────────────────┐
                    │                 RECONCILE LOOP (every 10s)           │
                    └──────────────────────────────────────────────────────┘
                                           │
          ┌────────────────────────────────┼────────────────────────────────┐
          │                                │                                │
          ▼                                ▼                                ▼
   ┌─────────────┐                  ┌─────────────┐                  ┌─────────────┐
   │ AG Helper 0 │                  │ AG Helper 1 │                  │ AG Helper 2 │
   │ GET /state  │                  │ GET /state  │                  │ GET /state  │
   └──────┬──────┘                  └──────┬──────┘                  └──────┬──────┘
          │                                │                                │
          ▼                                ▼                                ▼
   ┌─────────────┐                  ┌─────────────┐                  ┌─────────────┐
   │ SQL Server  │                  │ SQL Server  │                  │ SQL Server  │
   │ DMV Queries │                  │ DMV Queries │                  │ DMV Queries │
   └──────┬──────┘                  └──────┬──────┘                  └──────┬──────┘
          │                                │                                │
          ▼                                ▼                                ▼
   ┌─────────────┐                  ┌─────────────┐                  ┌─────────────┐
   │{role,health,│                  │{role,health,│                  │{role,health,│
   │ seqNum,...} │                  │ seqNum,...} │                  │ seqNum,...} │
   └──────┬──────┘                  └──────┬──────┘                  └──────┬──────┘
          │                                │                                │
          └────────────────────────────────┼────────────────────────────────┘
                                           │
                                           ▼
                              ┌────────────────────────┐
                              │ Controller aggregates  │
                              │ - hasPrimary?          │
                              │ - failover candidates  │
                              │ - update CR status     │
                              └───────────┬────────────┘
                                          │
                           ┌──────────────┴──────────────┐
                           │                             │
                    hasPrimary=true              hasPrimary=false
                           │                             │
                           ▼                             ▼
                    ┌─────────────┐            ┌─────────────────────┐
                    │ All good!   │            │ Grace period logic  │
                    │ Requeue 10s │            │ - Wait 30s          │
                    └─────────────┘            │ - Select candidate  │
                                               │ - Trigger failover  │
                                               └──────────┬──────────┘
                                                          │
                                                          ▼
                                               ┌─────────────────────┐
                                               │ POST /failover to   │
                                               │ best candidate pod  │
                                               └──────────┬──────────┘
                                                          │
                                                          ▼
                                               ┌─────────────────────┐
                                               │ AG Helper executes  │
                                               │ ALTER AG...FAILOVER │
                                               └─────────────────────┘
```

---

## Key Takeaways

1. **AG Helper** is the "eyes and hands" on each pod — it queries SQL Server DMVs and executes T-SQL commands
2. **Controller** is the "brain" — it aggregates state from all AG Helpers and makes failover decisions
3. **Communication** is via HTTP REST API (Controller → AG Helper on port 8080)
4. **Failover commands** are always executed by the AG Helper on the **target** secondary replica
5. **Grace periods** prevent flapping (30s wait before failover, 60s cooldown between failovers)
6. **LSN-based selection** ensures the replica with the most recent data (least data loss) becomes primary
7. **Health states** differentiate between "not ready yet" (Waiting) and "broken" (Critical)

---

## Related Documentation

- [AG Helper Reference](ag-helper-reference.md) - HTTP API details, configuration options
- [Sidecar Architecture](../architecture/sidecar-architecture.md) - Container design patterns
- [Failover Management](failover-management.md) - Manual failover procedures
- [Operator Design](../architecture/operator-design.md) - Controller patterns and design decisions
