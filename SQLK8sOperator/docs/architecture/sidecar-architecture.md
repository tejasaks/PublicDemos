# Sidecar Architecture

[← Back to Architecture](overview.md) | [Documentation Home](../README.md)

This document describes the sidecar container design, including the AG Helper and SQL Exporter sidecars.

## Table of Contents

- [Sidecar Pattern](#sidecar-pattern)
- [Pod Container Structure](#pod-container-structure)
- [AG Helper Sidecar](#ag-helper-sidecar)
- [SQL Exporter Sidecar](#sql-exporter-sidecar)
- [Inter-Container Communication](#inter-container-communication)
- [Health Probe Design](#health-probe-design)
- [Graceful Shutdown](#graceful-shutdown)
- [Resource Sharing](#resource-sharing)

## Sidecar Pattern

### Why Sidecars?

| Approach | Pros | Cons | Decision |
|----------|------|------|----------|
| **Sidecar containers** | Separation of concerns, independent updates, stock MS image | More containers | ✅ Chosen |
| Built into SQL image | Simpler pod | Coupled releases, custom image required | ❌ Rejected |
| External DaemonSet | Shared across pods | Complex routing, security concerns | ❌ Rejected |
| CronJob polling | Simple implementation | Not real-time, no failover support | ❌ Rejected |

### Benefits of Sidecar Pattern

1. **Separation of Concerns** - Each container has a single responsibility
2. **Independent Updates** - Update AG Helper without touching SQL Server
3. **Stock Images** - Use official Microsoft SQL Server images
4. **Shared Lifecycle** - Sidecars start/stop with the main container
5. **Shared Network** - All containers share `localhost`

## Pod Container Structure

```
┌─────────────────────────────────────────────────────────────────────┐
│  SQL Server Pod (e.g., sql-prod-01-0)                               │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  ┌─────────────────────────────────────────────────────────────┐    │
│  │                    Init Containers                           │    │
│  │  ┌───────────────────┐                                       │    │
│  │  │ init-permissions  │  Sets file permissions on volumes    │    │
│  │  └───────────────────┘  (runs as root, then exits)          │    │
│  └─────────────────────────────────────────────────────────────┘    │
│                                                                      │
│  ┌─────────────────────────────────────────────────────────────┐    │
│  │                    Main Containers                           │    │
│  │                                                              │    │
│  │  ┌─────────────────┐  ┌─────────────────┐  ┌──────────────┐ │    │
│  │  │   mssql-server  │  │   ag-helper     │  │ sql-exporter │ │    │
│  │  │                 │  │                 │  │              │ │    │
│  │  │ Port: 1433      │  │ Port: 8080      │  │ Port: 9399   │ │    │
│  │  │                 │  │                 │  │              │ │    │
│  │  │ SQL Server      │  │ AG monitoring   │  │ Prometheus   │ │    │
│  │  │ database engine │  │ Failover API    │  │ metrics      │ │    │
│  │  │                 │  │ Health probes   │  │              │ │    │
│  │  └────────┬────────┘  └────────┬────────┘  └──────┬───────┘ │    │
│  │           │                    │                   │         │    │
│  │           └────── localhost ───┴───────────────────┘         │    │
│  │                 (127.0.0.1 communication)                    │    │
│  └─────────────────────────────────────────────────────────────┘    │
│                                                                      │
│  ┌─────────────────────────────────────────────────────────────┐    │
│  │                    Shared Volumes                            │    │
│  │                                                              │    │
│  │  /var/opt/mssql/data    (PVC: data)     - SQL data files    │    │
│  │  /var/opt/mssql/log     (PVC: log)      - Transaction logs  │    │
│  │  /var/opt/mssql/secrets (Secret)        - SA password       │    │
│  │  /etc/mssql             (ConfigMap)     - mssql.conf        │    │
│  └─────────────────────────────────────────────────────────────┘    │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

## AG Helper Sidecar

### Purpose

The AG Helper sidecar provides:
- Availability Group health monitoring
- Failover API for manual and automatic failover
- Kubernetes probe endpoints
- Multi-AG discovery and management

### Container Specification

```yaml
containers:
  - name: ag-helper
    image: mssql-operator/ag-helper:latest
    ports:
      - containerPort: 8080
        name: http
        protocol: TCP
    args:
      - "-sql-host=localhost"
      - "-sql-port=1433"
      - "-http-port=8080"
      - "-monitor-interval=10s"
      - "-ag-name=ProductionAG"  # Required: explicit AG name
      - "-max-retries=3"         # Retry transient SQL errors (1-30)
      - "-retry-interval=5s"     # Delay between retries
      - "-staleness-threshold=30s" # Data older than this = stale
      # Optional: sp_server_diagnostics integration (level 1 = off)
      - "-failure-condition-level=1"  # 1-5, see health-detection-comparison.md
    env:
      - name: SQL_PASSWORD
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
      initialDelaySeconds: 10
      periodSeconds: 5
    resources:
      limits:
        cpu: 100m
        memory: 128Mi
      requests:
        cpu: 50m
        memory: 64Mi
```

### Health States

```
┌──────────────────────────────────────────────────────────────────────────┐
│                    AG Helper State Machine                               │
├──────────────────────────────────────────────────────────────────────────┤
│                                                                          │
│  ┌──────────┐     AG detected      ┌──────────┐                         │
│  │ WAITING  │ ─────────────────────▶│ HEALTHY  │                         │
│  │          │                       │          │                         │
│  │ Liveness:│◀───── AG removed ─────│ Liveness:│                         │
│  │   PASS   │                       │   PASS   │                         │
│  │ Readiness│                       │ Readiness│                         │
│  │   FAIL   │                       │   PASS   │                         │
│  └──────────┘                       └────┬─────┘                         │
│                                          │                               │
│                           ┌──────────────┼──────────────┐                │
│                           │ (baseline)   │ (if level≥2) │                │
│                           ▼              ▼              │                │
│                    replica not     diagnostics show     │                │
│                    synced          component warning    │                │
│                           │              │              │                │
│                           ▼              ▼              │                │
│                      ┌──────────┐                       │                │
│                      │ WARNING  │◀──────────────────────┘                │
│                      │          │                                        │
│                      │ Liveness:│                                        │
│                      │   PASS   │                                        │
│                      │ Readiness│                                        │
│                      │   PASS   │                                        │
│                      └────┬─────┘                                        │
│                           │                                              │
│                ┌──────────┼──────────────┐                               │
│                │(baseline)│ (if level≥2) │                               │
│                ▼          ▼              │                               │
│          AG broken   diagnostics show   │                               │
│          unreachable component error    │                               │
│                │          │              │                               │
│                ▼          ▼              │                               │
│           ┌──────────┐                  │                               │
│           │ CRITICAL │◀─────────────────┘                               │
│           │          │                                                   │
│           │ Liveness:│                                                   │
│           │   FAIL   │                                                   │
│           │ Readiness│                                                   │
│           │   FAIL   │                                                   │
│           └──────────┘                                                   │
│                                                                          │
│                                                                          │
│  Connection State Machine (overlays health states above):                │
│                                                                          │
│  ┌───────────┐  transient error    ┌──────────────┐  retries exhausted  │
│  │ CONNECTED │ ───────────────────▶│ RECONNECTING │ ──────────────────▶ │
│  │           │◀────────────────────│              │                     │
│  │ Queries   │  retry succeeds     │ Retrying SQL │   ┌──────────────┐ │
│  │ succeed   │                     │ with backoff │   │ DISCONNECTED │ │
│  └───────────┘                     └──────────────┘   │              │ │
│       ▲                                                │ All queries  │ │
│       └────────────── reconnection succeeds ──────────│ fail         │ │
│                                                        └──────────────┘ │
│                                                                          │
│  Staleness Detection (orthogonal to above):                              │
│  - If lastSuccessfulQuery > stalenessThreshold ago → dataStale=true     │
│  - Stale data: /health → 503, /ready → 503                              │
│  - Controller maps stale to connectedState=STALE, health=Warning        │
│                                                                          │
└──────────────────────────────────────────────────────────────────────────┘
```

### HTTP Endpoints

| Endpoint | Method | Purpose | Notes |
|----------|--------|---------|-------|
| `/health`, `/healthz` | GET | Liveness probe | Returns 503 when data is stale |
| `/ready`, `/readyz` | GET | Readiness probe | Returns 503 when data is stale |
| `/state` | GET | Full AG state (JSON) | Includes `dataStale`, `connectionState`, `consecutiveFailures` |
| `/role` | GET | Current replica role | |
| `/failover` | POST | Trigger failover | |
| `/sequence` | GET | Last hardened LSN | |
| `/ags` | GET | List all discovered AGs | |
| `/state/{agName}` | GET | Specific AG state | Includes staleness metadata |
| `/discover` | POST | Force AG discovery | |

## SQL Exporter Sidecar

### Purpose

The SQL Exporter sidecar provides Prometheus metrics for SQL Server monitoring.

### Container Specification

```yaml
containers:
  - name: sql-exporter
    image: burningalchemist/sql_exporter:latest
    ports:
      - containerPort: 9399
        name: metrics
        protocol: TCP
    args:
      - "-config.file=/etc/sql_exporter/sql_exporter.yml"
    env:
      - name: SQLSERVER_DATA_SOURCE
        value: "sqlserver://sa:$(SQL_PASSWORD)@localhost:1433"
      - name: SQL_PASSWORD
        valueFrom:
          secretKeyRef:
            name: sql-sa-password
            key: password
    volumeMounts:
      - name: sql-exporter-config
        mountPath: /etc/sql_exporter
    resources:
      limits:
        cpu: 100m
        memory: 128Mi
      requests:
        cpu: 50m
        memory: 64Mi
```

### Metrics Exposed

| Metric | Type | Description |
|--------|------|-------------|
| `mssql_up` | Gauge | SQL Server availability |
| `mssql_connections` | Gauge | Active connections |
| `mssql_batch_requests_total` | Counter | Batch requests per second |
| `mssql_page_life_expectancy` | Gauge | Buffer pool page life |
| `mssql_buffer_cache_hit_ratio` | Gauge | Buffer cache hit ratio |
| `mssql_deadlocks_total` | Counter | Deadlock count |
| `mssql_user_errors_total` | Counter | User error count |

## Inter-Container Communication

### Localhost Networking

All containers in a pod share the same network namespace:

```
┌─────────────────────────────────────────────────────────┐
│  Pod Network Namespace (shared)                          │
│                                                          │
│  127.0.0.1:1433  ──▶ mssql-server                       │
│  127.0.0.1:8080  ──▶ ag-helper                          │
│  127.0.0.1:9399  ──▶ sql-exporter                       │
│                                                          │
│  ┌─────────┐    localhost:1433    ┌───────────┐         │
│  │ag-helper│ ─────────────────────▶│mssql-server│         │
│  └─────────┘                      └───────────┘         │
│                                                          │
│  ┌────────────┐  localhost:1433   ┌───────────┐         │
│  │sql-exporter│ ──────────────────▶│mssql-server│         │
│  └────────────┘                   └───────────┘         │
└─────────────────────────────────────────────────────────┘
```

### Connection Flow

1. **AG Helper → SQL Server**: Queries `sys.dm_hadr_*` views for AG status
2. **AG Helper → SQL Server**: Calls `sp_server_diagnostics` for component health (when `failureConditionLevel >= 2`)
3. **SQL Exporter → SQL Server**: Queries performance counters
4. **External → AG Helper**: Health probes, failover API
5. **Prometheus → SQL Exporter**: Scrapes metrics endpoint

> **Note:** Connection flow #2 is optional. When `failureConditionLevel` is 1 (default),
> AG Helper only queries DMV views. At level 2+, it additionally runs `EXEC sp_server_diagnostics`
> to evaluate SQL Server component health (system, resource, query_processing, io_subsystem, events).
> See [Health Detection Comparison](../availability-groups/health-detection-comparison.md) for details.

## Health Probe Design

### Probe Strategy

| Probe | Container | Path | Purpose |
|-------|-----------|------|---------|
| **Liveness** | ag-helper | `/healthz` | Restart if stuck |
| **Readiness** | ag-helper | `/readyz` | Route traffic only when AG ready |
| **Startup** | mssql-server | tcp:1433 | Wait for SQL to start |

### Why AG Helper Handles Both Probes

The AG Helper provides application-level health, not just process health:

```yaml
# AG Helper readiness determines traffic routing
readinessProbe:
  httpGet:
    path: /readyz
    port: 8080
  # Returns 200 only when:
  # - SQL Server is running
  # - AG is configured (or we're not in AG mode)
  # - This replica is synchronized
  # - Data is not stale (lastSuccessfulQuery within stalenessThreshold)
  #
  # Returns 503 when:
  # - Data is stale (no successful SQL query within stalenessThreshold)
  # - Response includes connectionState and dataStale fields
```

### Probe Timing

```yaml
livenessProbe:
  initialDelaySeconds: 30  # Wait for SQL Server to start
  periodSeconds: 10        # Check every 10s
  failureThreshold: 3      # 3 failures = restart
  
readinessProbe:
  initialDelaySeconds: 10  # Start checking early
  periodSeconds: 5         # Check frequently
  failureThreshold: 2      # 2 failures = remove from service
```

## Graceful Shutdown

### PreStop Hook

```yaml
lifecycle:
  preStop:
    exec:
      command:
        - /bin/sh
        - -c
        - |
          # Signal AG Helper to stop monitoring
          curl -X POST localhost:8080/shutdown
          # Wait for in-flight requests
          sleep 5
```

### Shutdown Sequence

```
1. Pod receives SIGTERM
    │
    ▼
2. PreStop hooks execute (parallel)
    ├── ag-helper: stops monitoring, drains requests
    └── sql-exporter: completes current scrape
    │
    ▼
3. Containers receive SIGTERM
    ├── ag-helper: graceful shutdown
    ├── sql-exporter: graceful shutdown
    └── mssql-server: checkpoint, shutdown
    │
    ▼
4. terminationGracePeriodSeconds timeout (30s default)
    │
    ▼
5. SIGKILL if still running
```

## Resource Sharing

### Shared Volumes

```yaml
volumes:
  - name: data
    persistentVolumeClaim:
      claimName: sql-prod-01-data
  - name: secrets
    secret:
      secretName: sql-sa-password
  - name: mssql-conf
    configMap:
      name: sql-prod-01-conf
```

### Volume Mounts by Container

| Volume | mssql-server | ag-helper | sql-exporter |
|--------|--------------|-----------|--------------|
| data | ✅ RW | ❌ | ❌ |
| log | ✅ RW | ❌ | ❌ |
| secrets | ✅ RO | ✅ RO | ✅ RO |
| mssql-conf | ✅ RO | ❌ | ❌ |

### Resource Limits

Total pod resources = sum of all containers:

```yaml
# Pod totals
resources:
  limits:
    cpu: "4200m"      # 4000m + 100m + 100m
    memory: "8.25Gi"  # 8Gi + 128Mi + 128Mi
  requests:
    cpu: "2100m"      # 2000m + 50m + 50m  
    memory: "4.125Gi" # 4Gi + 64Mi + 64Mi
```

## Next Steps

- [Networking](networking.md) - Services and traffic flow
- [AG Helper Reference](../availability-groups/ag-helper-reference.md) - Complete API docs
- [Monitoring Overview](../monitoring/overview.md) - Metrics and dashboards
- [Health Detection Comparison](../availability-groups/health-detection-comparison.md) - WSFC vs Operator health detection, failure condition levels
