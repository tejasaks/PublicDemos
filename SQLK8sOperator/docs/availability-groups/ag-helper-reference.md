# AG Helper Reference

[← Back to Availability Groups](overview.md) | [Documentation Home](../README.md)

Complete reference for the AG Helper sidecar container.

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

```yaml
containers:
  - name: ag-helper
    image: mssql-operator/ag-helper:latest
    ports:
      - containerPort: 8080
        name: http
    env:
      - name: MSSQL_HOST
        value: "localhost"
      - name: MSSQL_PORT
        value: "1433"
      - name: MSSQL_SA_PASSWORD
        valueFrom:
          secretKeyRef:
            name: sql-sa-password
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

## Configuration

The AG Helper is configured via environment variables:

| Variable | Default | Description |
|----------|---------|-------------|
| `MSSQL_HOST` | `localhost` | SQL Server hostname |
| `MSSQL_PORT` | `1433` | SQL Server port |
| `MSSQL_SA_PASSWORD` | (required) | SA password |
| `MONITOR_INTERVAL` | `10s` | AG check interval |
| `HTTP_PORT` | `8080` | API listen port |
| `LOG_LEVEL` | `info` | Logging level |
| `LOG_FORMAT` | `json` | Log format (json/text) |

### Advanced Configuration

| Variable | Default | Description |
|----------|---------|-------------|
| `CONNECTION_TIMEOUT` | `5s` | SQL connection timeout |
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
| GET | `/ags` | List all AGs |
| GET | `/ags/{name}` | Specific AG details |
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

### GET /ags

List all availability groups.

**Response:**
```json
{
  "availabilityGroups": [
    {
      "name": "ProductionAG",
      "role": "PRIMARY",
      "health": "HEALTHY"
    },
    {
      "name": "AnalyticsAG",
      "role": "SECONDARY",
      "health": "HEALTHY"
    }
  ]
}
```

---

### GET /ags/{name}

Get specific AG details.

**Request:**
```
GET /ags/ProductionAG
```

**Response:**
```json
{
  "name": "ProductionAG",
  "role": "PRIMARY",
  "operationalState": "ONLINE",
  "primaryReplica": "sql-ag-prod01-0",
  "databases": ["AppDB"],
  "replicas": 3
}
```

---

### POST /failover

Trigger manual failover to this replica.

**Request Body:**
```json
{
  "agName": "ProductionAG",   // Optional: specific AG (default: first AG)
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
