# Monitoring Overview

[← Back to Documentation](../README.md)

Comprehensive monitoring guide for SQL Server deployments managed by the operator.

## Table of Contents

- [Monitoring Architecture](#monitoring-architecture)
- [Components](#components)
- [Metrics Overview](#metrics-overview)
- [Alerting](#alerting)
- [Quick Start](#quick-start)

## Monitoring Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                      Monitoring Architecture                         │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  ┌──────────────────────────────────────────────────────────────┐   │
│  │                     SQL Server Pods                           │   │
│  │                                                               │   │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐           │   │
│  │  │ sql-prod-0  │  │ sql-prod-1  │  │ sql-prod-2  │           │   │
│  │  │             │  │             │  │             │           │   │
│  │  │ SQL Exporter│  │ SQL Exporter│  │ SQL Exporter│           │   │
│  │  │   :9399     │  │   :9399     │  │   :9399     │           │   │
│  │  └──────┬──────┘  └──────┬──────┘  └──────┬──────┘           │   │
│  │         │                │                │                   │   │
│  └─────────┼────────────────┼────────────────┼───────────────────┘   │
│            │                │                │                       │
│            └────────────────┼────────────────┘                       │
│                             │                                        │
│                             ▼                                        │
│  ┌──────────────────────────────────────────────────────────────┐   │
│  │                        Prometheus                             │   │
│  │                                                               │   │
│  │  ServiceMonitor auto-discovers SQL Exporter endpoints        │   │
│  │  Scrapes every 15s                                           │   │
│  │  Stores metrics for 15d                                      │   │
│  └──────────────────────────────────────────────────────────────┘   │
│                             │                                        │
│                             ▼                                        │
│  ┌──────────────────────────────────────────────────────────────┐   │
│  │                        Grafana                                │   │
│  │                                                               │   │
│  │  Pre-built dashboards:                                       │   │
│  │  - SQL Server Overview                                       │   │
│  │  - Availability Group Status                                 │   │
│  │  - Query Performance                                         │   │
│  │  - Resource Utilization                                      │   │
│  └──────────────────────────────────────────────────────────────┘   │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

## Components

### SQL Exporter

A sidecar container that exposes SQL Server metrics in Prometheus format.

| Feature | Description |
|---------|-------------|
| Port | 9399 |
| Protocol | HTTP |
| Format | Prometheus text |
| Interval | Scraped by Prometheus |

See [SQL Exporter Reference](sql-exporter-reference.md) for full documentation.

### Prometheus

Time-series database for metrics storage and querying.

| Feature | Description |
|---------|-------------|
| Discovery | ServiceMonitor CRD |
| Retention | 15 days default |
| Scrape interval | 15 seconds |

See [Prometheus Setup](prometheus-setup.md) for installation.

### Grafana

Visualization and dashboards for SQL Server metrics.

| Feature | Description |
|---------|-------------|
| Dashboards | Pre-built JSON |
| Alerts | Grafana alerting |
| Data source | Prometheus |

See [Grafana Dashboards](grafana-dashboards.md) for dashboard details.

## Metrics Overview

### Categories

| Category | Description | Example Metrics |
|----------|-------------|-----------------|
| **Instance** | SQL Server health | connections, uptime |
| **Performance** | Query performance | batch/sec, compilations |
| **Resources** | Resource usage | memory, CPU, I/O |
| **AG Status** | Availability Groups | sync state, role |
| **Databases** | Database metrics | size, transactions |

### Key Metrics

| Metric | Description | Threshold |
|--------|-------------|-----------|
| `mssql_up` | Instance reachable | 1 = up |
| `mssql_connections` | Active connections | < max |
| `mssql_batch_requests_per_sec` | Batches/second | baseline |
| `mssql_ag_is_primary` | AG primary status | 1 = primary |
| `mssql_ag_database_sync_state` | DB sync state | 2 = synced |
| `mssql_memory_used_percent` | Memory usage | < 90% |

## Alerting

### Built-in Alert Rules

| Alert | Severity | Description |
|-------|----------|-------------|
| SQLServerDown | Critical | Instance unreachable |
| AGNotSynchronized | Critical | Database not synced |
| HighMemoryUsage | Warning | Memory > 90% |
| HighCPUUsage | Warning | CPU > 80% |
| LongRunningQuery | Warning | Query > 5 min |
| AGFailover | Info | Failover occurred |

### Alert Channels

- Prometheus Alertmanager
- Grafana alerting
- PagerDuty, Slack, Email integrations

## Quick Start

### 1. Enable Monitoring

```yaml
apiVersion: mssql.microsoft.com/v1alpha1
kind: SQLServer
metadata:
  name: sql-prod
spec:
  monitoring:
    enabled: true
```

### 2. Install Prometheus Stack

```bash
helm install prometheus prometheus-community/kube-prometheus-stack \
  --namespace monitoring \
  --create-namespace
```

### 3. Apply ServiceMonitor

```bash
kubectl apply -f config/monitoring/servicemonitor.yaml
```

### 4. Import Grafana Dashboards

```bash
kubectl apply -f config/monitoring/grafana-dashboards.yaml
```

### 5. Verify Metrics

```bash
# Port-forward to Prometheus
kubectl port-forward svc/prometheus-operated -n monitoring 9090:9090

# Check targets
open http://localhost:9090/targets

# Query SQL metrics
open http://localhost:9090/graph?g0.expr=mssql_up
```

## Next Steps

- [Prometheus Setup](prometheus-setup.md) - Detailed installation
- [Grafana Dashboards](grafana-dashboards.md) - Dashboard configuration
- [SQL Exporter Reference](sql-exporter-reference.md) - All metrics
- [Troubleshooting](../user-guide/troubleshooting.md) - Common issues
