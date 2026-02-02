# AG Helper Reference

[← Back to Availability Groups](overview.md) | [Controller Workflow Details](controller-workflow-details.md) | [Documentation Home](../README.md)

Complete reference for the AG Helper sidecar container.

> **Developer Deep Dive:** For a comprehensive walkthrough of how the AG Helper and Controller work together, including SQL queries, health determination logic, and a complete failover scenario, see [Controller Workflow Details](controller-workflow-details.md).

## Table of Contents

- [Overview](#overview)
- [Configuration](#configuration)
- [HTTP API](#http-api)
- [Health Probes](#health-probes)
- [Failover API](#failover-api)
- [Metrics](#metrics)
- [Environment Variables](#environment-variables)
- [Logging](#logging)

## Overview

The AG Helper is a sidecar container that runs alongside SQL Server in each pod. It provides:

- Health monitoring for Kubernetes probes
- HTTP API for AG state inspection
- Failover coordination
- Prometheus metrics export

### Container Specification

The operator automatically injects the AG Helper sidecar container. Here's what it looks like:

```yaml
containers:
  - name: ag-helper
    image: mssql-operator/ag-helper:latest
    args:
      - "-ag-name=$(AG_NAME)"
      - "-sql-host=localhost"
      - "-sql-port=1433"
    ports:
      - containerPort: 8080
        name: http
    env:
      - name: AG_NAME
        value: "ProductionAG"
      - name: AG_HELPER_USERNAME
        valueFrom:
          secretKeyRef:
            name: sql-ag-aghelper  # Dedicated AG Helper credentials secret
            key: username
      - name: AG_HELPER_PASSWORD
        valueFrom:
          secretKeyRef:
            name: sql-ag-aghelper
            key: password
    livenessProbe:
      httpGet:
        path: /healthz
        port: 8080
      initialDelaySeconds: 30
      periodSeconds: 10
    readinessProbe:
      httpGet:
        path: /readyz
        port: 8080
      initialDelaySeconds: 30
      periodSeconds: 10
```

> **Important:** The AG Helper uses a dedicated SQL login with minimal permissions for health monitoring. You must create this login on all replicas before deploying the AG. See [Creating the AG Helper Login](#creating-the-ag-helper-login) below.

## Configuration

The AG Helper is configured via environment variables and command-line flags.

### Authentication

The AG Helper connects to SQL Server using a **dedicated least-privilege login** (recommended), following the [Pacemaker pattern](https://learn.microsoft.com/en-us/sql/linux/sql-server-linux-availability-group-cluster-pacemaker) from Microsoft's SQL Server Linux documentation.

#### Recommended: Dedicated AG Helper Login

Create a dedicated SQL login with only the required permissions:

```sql
-- Run on ALL replicas
CREATE LOGIN ag_helper WITH PASSWORD = 'YourStrong@AGHelperPassw0rd!';
GRANT VIEW SERVER STATE TO ag_helper;
GRANT ALTER ANY AVAILABILITY GROUP TO ag_helper;
```

The AG Helper reads credentials from environment variables:

| Source | Variable | Description |
|--------|----------|-------------|
| Environment | `AG_HELPER_USERNAME` | SQL login username (recommended: `ag_helper`) |
| Environment | `AG_HELPER_PASSWORD` | SQL login password |

#### Fallback: SA Account (Not Recommended for Production)

For backward compatibility, the AG Helper falls back to SA credentials if `AG_HELPER_*` variables are not set:

| Source | Variable | Description |
|--------|----------|-------------|
| Environment | `SA_PASSWORD` | SA password (fallback only) |

> **Warning:** Using SA credentials gives the AG Helper unrestricted access to SQL Server. For production deployments, always use a dedicated least-privilege login.

### Creating the AG Helper Login

Run these T-SQL commands on **ALL** AG replicas before deploying the SQLServerAG resource:

```sql
-- Step 1: Create the login
CREATE LOGIN ag_helper WITH PASSWORD = 'YourStrong@AGHelperPassw0rd!';
GO

-- Step 2: Grant required permissions
-- VIEW SERVER STATE: Query sys.dm_hadr_* DMVs for AG health
GRANT VIEW SERVER STATE TO ag_helper;
GO

-- ALTER ANY AVAILABILITY GROUP: Perform failover operations
GRANT ALTER ANY AVAILABILITY GROUP TO ag_helper;
GO
```

Create the Kubernetes secret:

```bash
kubectl create secret generic sql-ag-aghelper \
  --namespace mssql \
  --from-literal=username=ag_helper \
  --from-literal=password='YourStrong@AGHelperPassw0rd!'
```

Or use a YAML manifest (see [samples/ag-helper-credentials-secret.yaml](../../samples/ag-helper-credentials-secret.yaml)):

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: sql-ag-aghelper
  namespace: mssql
type: Opaque
stringData:
  username: "ag_helper"
  password: "YourStrong@AGHelperPassw0rd!"
```

> **Tip: Dev/Test vs Production**
> - **Dev/Test:** The sample AG manifests include Secret definitions inline for convenience. Simply apply the manifest as-is.
> - **Production:** Pre-create secrets using the methods above, then remove/comment out the SECRETS SECTION from the sample manifests before applying.
>
> See [Deployment Guide - Step 2.5](deployment-guide.md#step-25-create-ag-helper-credentials) for the complete workflow.

### Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `AG_NAME` | (required) | Name of the Availability Group to monitor |
| `AG_HELPER_USERNAME` | (fallback to `sa`) | SQL login username for health monitoring |
| `AG_HELPER_PASSWORD` | (fallback to `SA_PASSWORD`) | SQL login password |
| `SA_PASSWORD` | - | Fallback password (not recommended for production) |
| `MONITOR_INTERVAL` | `10s` | AG health check interval |
| `HTTP_PORT` | `8080` | HTTP API listen port |

### Command-Line Flags

| Flag | Default | Description |
|------|---------|-------------|
| `-ag-name` | `$AG_NAME` | AG name (can also use env var) |
| `-sql-host` | `localhost` | SQL Server hostname |
| `-sql-port` | `1433` | SQL Server port |
| `-sql-user` | `$AG_HELPER_USERNAME` or `sa` | SQL Server username |
| `-sql-password` | `$AG_HELPER_PASSWORD` or `$SA_PASSWORD` | SQL Server password |
| `-monitor-interval` | `10s` | AG check interval |
| `-connection-timeout` | `30s` | SQL connection timeout |
| `-http-port` | `8080` | HTTP API port |

### Advanced Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `CONNECTION_TIMEOUT` | `30s` | SQL connection timeout |
| `QUERY_TIMEOUT` | `10s` | SQL query timeout |
| `MAX_RETRIES` | `3` | Connection retry count |
| `RETRY_DELAY` | `1s` | Delay between retries |
| `ENABLE_METRICS` | `true` | Export Prometheus metrics |
| `METRICS_PORT` | `9399` | Metrics endpoint port |

## HTTP API

### Base URL

```
http://localhost:8080
```

### Endpoints Summary

| Method | Endpoint | Description |
|--------|----------|-------------|
| GET | `/healthz` | Liveness probe |
| GET | `/readyz` | Readiness probe |
| GET | `/state` | Full AG state |
| GET | `/role` | Current replica role |
| GET | `/sequence` | Current LSN (for failover) |
| POST | `/failover` | Trigger failover |

---

### GET /healthz

Liveness probe endpoint. Returns 200 if AG Helper is running.

**Response Codes:**
- `200 OK` - Healthy, Warning, or Waiting state
- `503 Service Unavailable` - Critical state

**Response:**
```json
{
  "status": "ok",
  "health": "Healthy"
}
```

---

### GET /readyz

Readiness probe endpoint. Returns 200 only if replica can serve traffic.

**Response Codes:**
- `200 OK` - Healthy or Warning state
- `503 Service Unavailable` - Waiting or Critical state

**Response:**
```json
{
  "status": "ready",
  "health": "Healthy",
  "role": "PRIMARY"
}
```

---

### GET /state

Full AG state including all availability groups and databases.

**Response:**
```json
{
  "instanceName": "sql-ag-prod01-0",
  "health": "Healthy",
  "timestamp": "2024-01-15T10:30:00Z",
  "availabilityGroups": [
    {
      "name": "ProductionAG",
      "role": "PRIMARY",
      "operationalState": "ONLINE",
      "synchronizationHealth": "HEALTHY",
      "primaryReplica": "sql-ag-prod01-0",
      "databases": [
        {
          "name": "AppDB",
          "synchronizationState": "SYNCHRONIZED",
          "isSuspended": false,
          "lastHardenedLsn": "0x00000027:000001A8:0001"
        }
      ],
      "replicas": [
        {
          "name": "sql-ag-prod01-0",
          "role": "PRIMARY",
          "connectedState": "CONNECTED",
          "synchronizationHealth": "HEALTHY"
        },
        {
          "name": "sql-ag-prod01-1",
          "role": "SECONDARY",
          "connectedState": "CONNECTED",
          "synchronizationHealth": "HEALTHY"
        },
        {
          "name": "sql-ag-prod01-2",
          "role": "SECONDARY",
          "connectedState": "CONNECTED",
          "synchronizationHealth": "HEALTHY"
        }
      ]
    }
  ]
}
```

---

### GET /role

Returns current replica role (PRIMARY, SECONDARY, or RESOLVING).

**Response:**
```json
{
  "role": "PRIMARY"
}
```

Or for multi-AG:
```json
{
  "roles": {
    "ProductionAG": "PRIMARY",
    "AnalyticsAG": "SECONDARY"
  }
}
```

---

### GET /sequence

Returns hardened LSN for failover candidate selection.

**Response:**
```json
{
  "instanceName": "sql-ag-prod01-1",
  "hardenedLsn": "0x00000027:000001A8:0001",
  "truncationLsn": "0x00000027:000001A0:0001",
  "lastCommitTime": "2024-01-15T10:29:55Z"
}
```

---

### POST /failover

Trigger manual failover to this replica.

**Request Body:**
```json
{
  "allowDataLoss": false,      // Required: must explicitly allow
  "force": false               // Optional: force even if not sync
}
```

**Response (Success):**
```json
{
  "success": true,
  "previousPrimary": "sql-ag-prod01-0",
  "newPrimary": "sql-ag-prod01-1",
  "agName": "ProductionAG",
  "dataLoss": false,
  "message": "Failover completed successfully"
}
```

**Response (Error):**
```json
{
  "success": false,
  "error": "Cannot failover: replica not synchronized",
  "currentState": "SYNCHRONIZING",
  "requiredState": "SYNCHRONIZED"
}
```

**Error Codes:**
- `400 Bad Request` - Invalid request body
- `409 Conflict` - Already primary
- `412 Precondition Failed` - Not synchronized and allowDataLoss=false
- `500 Internal Server Error` - Failover failed

## Health Probes

### Kubernetes Integration

```yaml
livenessProbe:
  httpGet:
    path: /healthz
    port: 8080
  initialDelaySeconds: 30
  periodSeconds: 10
  failureThreshold: 3

readinessProbe:
  httpGet:
    path: /readyz
    port: 8080
  initialDelaySeconds: 30
  periodSeconds: 10
  failureThreshold: 3
```

### Health State Matrix

| State | Liveness | Readiness | Description |
|-------|----------|-----------|-------------|
| Waiting | ✅ 200 | ❌ 503 | No AG configured yet |
| Healthy | ✅ 200 | ✅ 200 | AG online, all synced |
| Warning | ✅ 200 | ✅ 200 | AG online, some syncing |
| Critical | ❌ 503 | ❌ 503 | AG broken, SQL unreachable |

### Primary-Only Readiness

The AG Helper can be configured for primary-only readiness:

```yaml
env:
  - name: READINESS_PRIMARY_ONLY
    value: "true"
```

With this setting, only PRIMARY replicas return 200 for readiness.

## Failover API

### Failover Request Flow

```
┌───────────────────────────────────────────────────────────────┐
│                    Failover Request Flow                       │
├───────────────────────────────────────────────────────────────┤
│                                                                │
│  POST /failover                                                │
│       │                                                        │
│       ▼                                                        │
│  ┌─────────────────────────┐                                  │
│  │ Validate Request        │                                  │
│  │ - Parse JSON            │                                  │
│  │ - Check allowDataLoss   │                                  │
│  └───────────┬─────────────┘                                  │
│              │                                                 │
│              ▼                                                 │
│  ┌─────────────────────────┐                                  │
│  │ Check Current Role      │───▶ If PRIMARY: Error 409        │
│  │                         │                                  │
│  └───────────┬─────────────┘                                  │
│              │                                                 │
│              ▼                                                 │
│  ┌─────────────────────────┐                                  │
│  │ Check Sync State        │───▶ If NOT SYNC && !allowData:   │
│  │                         │     Error 412                    │
│  └───────────┬─────────────┘                                  │
│              │                                                 │
│              ▼                                                 │
│  ┌─────────────────────────┐                                  │
│  │ Execute Failover        │                                  │
│  │ ALTER AG ... FAILOVER   │                                  │
│  └───────────┬─────────────┘                                  │
│              │                                                 │
│              ▼                                                 │
│  ┌─────────────────────────┐                                  │
│  │ Verify New Role         │                                  │
│  │ Wait for PRIMARY        │                                  │
│  └───────────┬─────────────┘                                  │
│              │                                                 │
│              ▼                                                 │
│  Return success response                                       │
│                                                                │
└───────────────────────────────────────────────────────────────┘
```

### Force Failover

Force failover bypasses sync checks (DANGER: may lose data):

```bash
curl -X POST localhost:8080/failover \
  -H "Content-Type: application/json" \
  -d '{"allowDataLoss": true, "force": true}'
```

## Metrics

The AG Helper exports Prometheus metrics on port 9399.

### Available Metrics

| Metric | Type | Labels | Description |
|--------|------|--------|-------------|
| `ag_helper_up` | Gauge | | Helper is running (1/0) |
| `ag_helper_health` | Gauge | state | Current health state |
| `mssql_ag_is_primary` | Gauge | ag_name | Is this replica primary |
| `mssql_ag_replica_role` | Gauge | ag_name, replica | Replica role |
| `mssql_ag_database_sync_state` | Gauge | ag_name, database | Sync state |
| `mssql_ag_failover_total` | Counter | ag_name, success | Failover count |
| `mssql_ag_failover_duration_seconds` | Histogram | ag_name | Failover duration |

### Example Metrics Output

```
# HELP ag_helper_up AG Helper is up and running
# TYPE ag_helper_up gauge
ag_helper_up 1

# HELP mssql_ag_is_primary Whether this replica is PRIMARY
# TYPE mssql_ag_is_primary gauge
mssql_ag_is_primary{ag_name="ProductionAG"} 1

# HELP mssql_ag_database_sync_state Database synchronization state
# TYPE mssql_ag_database_sync_state gauge
mssql_ag_database_sync_state{ag_name="ProductionAG",database="AppDB"} 2
```

### Sync State Values

| Value | Meaning |
|-------|---------|
| 0 | NOT_SYNCHRONIZING |
| 1 | SYNCHRONIZING |
| 2 | SYNCHRONIZED |
| 3 | REVERTING |
| 4 | INITIALIZING |

## Environment Variables

### Complete Reference

| Variable | Default | Required | Description |
|----------|---------|----------|-------------|
| `MSSQL_HOST` | `localhost` | No | SQL Server host |
| `MSSQL_PORT` | `1433` | No | SQL Server port |
| `MSSQL_SA_PASSWORD` | - | **Yes** | SA password |
| `HTTP_PORT` | `8080` | No | API port |
| `METRICS_PORT` | `9399` | No | Prometheus port |
| `MONITOR_INTERVAL` | `10s` | No | Check interval |
| `CONNECTION_TIMEOUT` | `5s` | No | SQL connect timeout |
| `QUERY_TIMEOUT` | `10s` | No | SQL query timeout |
| `MAX_RETRIES` | `3` | No | Retry count |
| `RETRY_DELAY` | `1s` | No | Retry delay |
| `LOG_LEVEL` | `info` | No | debug/info/warn/error |
| `LOG_FORMAT` | `json` | No | json/text |
| `ENABLE_METRICS` | `true` | No | Export metrics |
| `READINESS_PRIMARY_ONLY` | `false` | No | Only primary is ready |

## Logging

### Log Format (JSON)

```json
{
  "level": "info",
  "ts": "2024-01-15T10:30:00.000Z",
  "msg": "AG state updated",
  "ag_name": "ProductionAG",
  "role": "PRIMARY",
  "health": "Healthy",
  "databases": 1,
  "replicas": 3
}
```

### Log Levels

| Level | Description |
|-------|-------------|
| `debug` | Detailed debugging info |
| `info` | Normal operation events |
| `warn` | Potential issues |
| `error` | Errors requiring attention |

### Viewing Logs

```bash
# AG Helper logs
kubectl logs sql-ag-prod01-0 -n mssql -c ag-helper

# Follow logs
kubectl logs sql-ag-prod01-0 -n mssql -c ag-helper -f

# Last 100 lines
kubectl logs sql-ag-prod01-0 -n mssql -c ag-helper --tail=100
```

## Next Steps

- [Failover Management](failover-management.md) - Automatic failover
- [Monitoring Overview](../monitoring/overview.md) - Prometheus integration
- [Troubleshooting](../user-guide/troubleshooting.md) - Common issues
