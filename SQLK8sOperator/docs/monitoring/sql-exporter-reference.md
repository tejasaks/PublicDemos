# SQL Exporter Reference

[← Back to Monitoring](overview.md) | [Documentation Home](../README.md)

Complete reference for the SQL Exporter sidecar metrics.

## Table of Contents

- [Overview](#overview)
- [Configuration](#configuration)
- [Instance Metrics](#instance-metrics)
- [Performance Metrics](#performance-metrics)
- [Resource Metrics](#resource-metrics)
- [Availability Group Metrics](#availability-group-metrics)
- [Database Metrics](#database-metrics)
- [Custom Queries](#custom-queries)
- [Environment Variables](#environment-variables)

## Overview

The SQL Exporter is a sidecar container that runs alongside SQL Server and exposes metrics in Prometheus format.

### Container Specification

```yaml
containers:
  - name: sql-exporter
    image: mssql-operator/sql-exporter:latest
    ports:
      - containerPort: 9399
        name: metrics
    env:
      - name: MSSQL_HOST
        value: "localhost"
      - name: MSSQL_PORT
        value: "1433"
      - name: MSSQL_USER
        value: "sa"
      - name: MSSQL_PASSWORD
        valueFrom:
          secretKeyRef:
            name: sql-sa-password
            key: password
```

### Metrics Endpoint

```bash
# Fetch all metrics
curl http://localhost:9399/metrics

# Example output
# HELP mssql_up Whether the SQL Server instance is up
# TYPE mssql_up gauge
mssql_up 1
```

## Configuration

### Enabling Monitoring

```yaml
apiVersion: mssql.microsoft.com/v1alpha1
kind: SQLServer
spec:
  monitoring:
    enabled: true
    # Optional: custom exporter image
    exporterImage: mssql-operator/sql-exporter:v1.0.0
    # Optional: scrape interval hint
    scrapeInterval: 15s
```

### Resource Limits

```yaml
spec:
  monitoring:
    enabled: true
    resources:
      requests:
        cpu: 50m
        memory: 64Mi
      limits:
        cpu: 200m
        memory: 128Mi
```

## Instance Metrics

### mssql_up

Whether the SQL Server instance is reachable.

| Attribute | Value |
|-----------|-------|
| Type | Gauge |
| Labels | instance |
| Values | 0 (down), 1 (up) |

```promql
mssql_up{instance="sql-prod-0:9399"} 1
```

### mssql_instance_info

SQL Server instance information.

| Attribute | Value |
|-----------|-------|
| Type | Gauge |
| Labels | version, edition, collation |
| Values | Always 1 |

```promql
mssql_instance_info{version="16.0.4095.4", edition="Developer Edition", collation="SQL_Latin1_General_CP1_CI_AS"} 1
```

### mssql_connections

Current number of connections.

| Attribute | Value |
|-----------|-------|
| Type | Gauge |
| Labels | instance, state |
| Unit | count |

```promql
mssql_connections{state="running"} 45
mssql_connections{state="sleeping"} 12
```

### mssql_uptime_seconds

Time since SQL Server started.

| Attribute | Value |
|-----------|-------|
| Type | Counter |
| Labels | instance |
| Unit | seconds |

```promql
mssql_uptime_seconds 86400
```

## Performance Metrics

### mssql_batch_requests_total

Total batch requests received.

| Attribute | Value |
|-----------|-------|
| Type | Counter |
| Labels | instance |
| Unit | count |

```promql
rate(mssql_batch_requests_total[5m])
```

### mssql_compilations_total

Total SQL compilations.

| Attribute | Value |
|-----------|-------|
| Type | Counter |
| Labels | instance |
| Unit | count |

```promql
rate(mssql_compilations_total[5m])
```

### mssql_recompilations_total

Total SQL recompilations.

| Attribute | Value |
|-----------|-------|
| Type | Counter |
| Labels | instance |
| Unit | count |

```promql
rate(mssql_recompilations_total[5m])
```

### mssql_page_life_expectancy

Buffer pool page life expectancy.

| Attribute | Value |
|-----------|-------|
| Type | Gauge |
| Labels | instance |
| Unit | seconds |

```promql
mssql_page_life_expectancy > 300
```

### mssql_lock_wait_time_ms_total

Total lock wait time.

| Attribute | Value |
|-----------|-------|
| Type | Counter |
| Labels | instance |
| Unit | milliseconds |

```promql
rate(mssql_lock_wait_time_ms_total[5m])
```

### mssql_deadlocks_total

Total number of deadlocks.

| Attribute | Value |
|-----------|-------|
| Type | Counter |
| Labels | instance |
| Unit | count |

```promql
increase(mssql_deadlocks_total[1h])
```

## Resource Metrics

### mssql_cpu_usage_percent

SQL Server CPU usage.

| Attribute | Value |
|-----------|-------|
| Type | Gauge |
| Labels | instance |
| Unit | percent |

```promql
mssql_cpu_usage_percent > 80
```

### mssql_server_memory_used_bytes

Memory used by SQL Server.

| Attribute | Value |
|-----------|-------|
| Type | Gauge |
| Labels | instance |
| Unit | bytes |

```promql
mssql_server_memory_used_bytes / 1024 / 1024 / 1024  # GB
```

### mssql_server_memory_total_bytes

Total memory available to SQL Server.

| Attribute | Value |
|-----------|-------|
| Type | Gauge |
| Labels | instance |
| Unit | bytes |

```promql
mssql_server_memory_used_bytes / mssql_server_memory_total_bytes * 100
```

### mssql_buffer_cache_hit_ratio

Buffer cache hit ratio.

| Attribute | Value |
|-----------|-------|
| Type | Gauge |
| Labels | instance |
| Unit | percent |

```promql
mssql_buffer_cache_hit_ratio < 99
```

### mssql_io_reads_total

Total I/O reads.

| Attribute | Value |
|-----------|-------|
| Type | Counter |
| Labels | instance, database, file_type |
| Unit | count |

```promql
rate(mssql_io_reads_total{database="AppDB"}[5m])
```

### mssql_io_writes_total

Total I/O writes.

| Attribute | Value |
|-----------|-------|
| Type | Counter |
| Labels | instance, database, file_type |
| Unit | count |

```promql
rate(mssql_io_writes_total{file_type="data"}[5m])
```

### mssql_io_latency_read_ms

Read I/O latency.

| Attribute | Value |
|-----------|-------|
| Type | Gauge |
| Labels | instance, database, file_type |
| Unit | milliseconds |

```promql
mssql_io_latency_read_ms > 20
```

### mssql_io_latency_write_ms

Write I/O latency.

| Attribute | Value |
|-----------|-------|
| Type | Gauge |
| Labels | instance, database, file_type |
| Unit | milliseconds |

```promql
mssql_io_latency_write_ms > 20
```

## Availability Group Metrics

### mssql_ag_is_primary

Whether this replica is the AG primary.

| Attribute | Value |
|-----------|-------|
| Type | Gauge |
| Labels | ag_name, instance |
| Values | 0 (secondary), 1 (primary) |

```promql
mssql_ag_is_primary{ag_name="ProductionAG"} 1
```

### mssql_ag_replica_role

Replica role in the AG.

| Attribute | Value |
|-----------|-------|
| Type | Gauge |
| Labels | ag_name, replica |
| Values | 0=RESOLVING, 1=PRIMARY, 2=SECONDARY |

```promql
mssql_ag_replica_role{ag_name="ProductionAG", replica="sql-prod-0"} 1
```

### mssql_ag_replica_connected_state

Whether replica is connected.

| Attribute | Value |
|-----------|-------|
| Type | Gauge |
| Labels | ag_name, replica |
| Values | 0=DISCONNECTED, 1=CONNECTED |

```promql
mssql_ag_replica_connected_state == 0  # Alert on disconnected
```

### mssql_ag_database_synchronization_state

Database synchronization state.

| Attribute | Value |
|-----------|-------|
| Type | Gauge |
| Labels | ag_name, database, replica |
| Values | 0=NOT_SYNC, 1=SYNCHRONIZING, 2=SYNCHRONIZED |

```promql
mssql_ag_database_synchronization_state != 2  # Not fully synced
```

### mssql_ag_redo_queue_size

Redo queue size on secondary.

| Attribute | Value |
|-----------|-------|
| Type | Gauge |
| Labels | ag_name, database, replica |
| Unit | bytes |

```promql
mssql_ag_redo_queue_size > 1073741824  # > 1GB
```

### mssql_ag_log_send_queue_size

Log send queue size.

| Attribute | Value |
|-----------|-------|
| Type | Gauge |
| Labels | ag_name, database, replica |
| Unit | bytes |

```promql
mssql_ag_log_send_queue_size > 10485760  # > 10MB
```

### mssql_ag_synchronization_health

Overall AG synchronization health.

| Attribute | Value |
|-----------|-------|
| Type | Gauge |
| Labels | ag_name |
| Values | 0=NOT_HEALTHY, 1=PARTIALLY, 2=HEALTHY |

```promql
mssql_ag_synchronization_health < 2
```

## Database Metrics

### mssql_database_size_bytes

Database size in bytes.

| Attribute | Value |
|-----------|-------|
| Type | Gauge |
| Labels | instance, database |
| Unit | bytes |

```promql
mssql_database_size_bytes / 1024 / 1024 / 1024  # GB
```

### mssql_database_log_size_bytes

Transaction log size.

| Attribute | Value |
|-----------|-------|
| Type | Gauge |
| Labels | instance, database |
| Unit | bytes |

```promql
mssql_database_log_size_bytes / mssql_database_size_bytes * 100  # Log %
```

### mssql_database_transactions_total

Total transactions per database.

| Attribute | Value |
|-----------|-------|
| Type | Counter |
| Labels | instance, database |
| Unit | count |

```promql
rate(mssql_database_transactions_total[5m])
```

### mssql_database_state

Database state.

| Attribute | Value |
|-----------|-------|
| Type | Gauge |
| Labels | instance, database |
| Values | 0=ONLINE, 1=RESTORING, 5=EMERGENCY, 6=OFFLINE |

```promql
mssql_database_state != 0  # Not online
```

## Custom Queries

### Adding Custom Metrics

Create a ConfigMap with custom queries:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: sql-exporter-custom-queries
  namespace: mssql
data:
  custom-queries.yaml: |
    queries:
      - name: mssql_table_row_count
        help: "Row count per table"
        labels: [database, schema, table]
        query: |
          SELECT 
            DB_NAME() as database,
            s.name as schema,
            t.name as table,
            SUM(p.rows) as row_count
          FROM sys.tables t
          JOIN sys.schemas s ON t.schema_id = s.schema_id
          JOIN sys.partitions p ON t.object_id = p.object_id
          WHERE p.index_id IN (0, 1)
          GROUP BY s.name, t.name
        value: row_count
      
      - name: mssql_index_fragmentation
        help: "Index fragmentation percentage"
        labels: [database, table, index]
        query: |
          SELECT 
            DB_NAME() as database,
            OBJECT_NAME(ips.object_id) as table,
            i.name as index,
            ips.avg_fragmentation_in_percent as fragmentation
          FROM sys.dm_db_index_physical_stats(DB_ID(), NULL, NULL, NULL, 'LIMITED') ips
          JOIN sys.indexes i ON ips.object_id = i.object_id AND ips.index_id = i.index_id
          WHERE ips.avg_fragmentation_in_percent > 10
        value: fragmentation
```

### Mount Custom Queries

```yaml
spec:
  monitoring:
    enabled: true
    customQueries:
      configMapRef:
        name: sql-exporter-custom-queries
        key: custom-queries.yaml
```

## Environment Variables

### Complete Reference

| Variable | Default | Required | Description |
|----------|---------|----------|-------------|
| `MSSQL_HOST` | `localhost` | No | SQL Server host |
| `MSSQL_PORT` | `1433` | No | SQL Server port |
| `MSSQL_USER` | `sa` | No | SQL user |
| `MSSQL_PASSWORD` | - | **Yes** | SQL password |
| `METRICS_PORT` | `9399` | No | Metrics port |
| `SCRAPE_INTERVAL` | `15s` | No | Internal collect interval |
| `QUERY_TIMEOUT` | `10s` | No | SQL query timeout |
| `LOG_LEVEL` | `info` | No | Logging level |
| `ENABLE_AG_METRICS` | `true` | No | Collect AG metrics |
| `ENABLE_DB_METRICS` | `true` | No | Collect database metrics |
| `CUSTOM_QUERIES_PATH` | - | No | Path to custom queries file |

## Next Steps

- [Prometheus Setup](prometheus-setup.md) - Configure scraping
- [Grafana Dashboards](grafana-dashboards.md) - Visualize metrics
- [Troubleshooting](../user-guide/troubleshooting.md) - Debug issues
