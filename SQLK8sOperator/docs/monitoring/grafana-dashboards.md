# Grafana Dashboards

[← Back to Monitoring](overview.md) | [Documentation Home](../README.md)

Pre-built Grafana dashboards for SQL Server monitoring.

## Table of Contents

- [Dashboard Overview](#dashboard-overview)
- [Installing Dashboards](#installing-dashboards)
- [SQL Server Overview Dashboard](#sql-server-overview-dashboard)
- [Availability Group Dashboard](#availability-group-dashboard)
- [Query Performance Dashboard](#query-performance-dashboard)
- [Resource Utilization Dashboard](#resource-utilization-dashboard)
- [Custom Dashboard Tips](#custom-dashboard-tips)

## Dashboard Overview

| Dashboard | Description | Use Case |
|-----------|-------------|----------|
| SQL Server Overview | High-level instance health | Daily monitoring |
| Availability Group | AG status and sync state | HA monitoring |
| Query Performance | Query stats, wait times | Performance tuning |
| Resource Utilization | CPU, memory, I/O | Capacity planning |

## Installing Dashboards

There are three ways to install dashboards in Grafana. Choose the method that best fits your workflow.

### Option 1: ConfigMap (Kubernetes-native)

This method uses a ConfigMap with Grafana's sidecar auto-discovery feature.

**Step 1: Create the ConfigMap file**

Create a file named `grafana-dashboard-configmap.yaml`:

```bash
# On Linux/macOS
nano grafana-dashboard-configmap.yaml

# On Windows (PowerShell)
notepad grafana-dashboard-configmap.yaml
```

Paste the following content (note: replace `{ ... }` with your actual dashboard JSON):

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: grafana-dashboards-mssql
  namespace: monitoring
  labels:
    grafana_dashboard: "1"  # Sidecar auto-discovery label
data:
  mssql-overview.json: |
    {
      "dashboard": {
        "title": "SQL Server Overview",
        "uid": "mssql-overview",
        ... your dashboard JSON here ...
      }
    }
```

**Step 2: Apply the ConfigMap**

```bash
kubectl apply -f grafana-dashboard-configmap.yaml
```

**Expected output:**
```
configmap/grafana-dashboards-mssql created
```

**Step 3: Verify the ConfigMap was created**

```bash
kubectl get configmap grafana-dashboards-mssql -n monitoring

# Expected output:
# NAME                       DATA   AGE
# grafana-dashboards-mssql   1      5s
```

**Step 4: Verify the dashboard appears in Grafana**

The Grafana sidecar automatically detects ConfigMaps with the `grafana_dashboard: "1"` label and loads them. Wait a few seconds, then:

```bash
# Port-forward to Grafana
kubectl port-forward svc/grafana -n monitoring 3000:80
```

Open http://localhost:3000 in your browser and navigate to **Dashboards**. Your dashboard should appear.

### Option 2: Grafana UI (Manual Import)

Use this method for quick one-time imports or testing.

**Step 1: Port-forward to Grafana**

```bash
kubectl port-forward svc/grafana -n monitoring 3000:80
```

**Step 2: Get the admin password**

```bash
kubectl get secret grafana -n monitoring -o jsonpath='{.data.admin-password}' | base64 -d

# Note the password that's displayed (no newline at end)
```

**Step 3: Open Grafana and log in**

Open http://localhost:3000 in your browser. Log in with:
- Username: `admin`
- Password: (the password from Step 2)

**Step 4: Import the dashboard**

1. Click on **Dashboards** in the left sidebar
2. Click **New** → **Import**
3. Either:
   - Paste the dashboard JSON directly, OR
   - Upload a JSON file, OR
   - Enter a Grafana.com dashboard ID (e.g., `9628` for a community SQL Server dashboard)
4. Select your **Prometheus** data source
5. Click **Import**

**Expected result:** The dashboard opens and displays your SQL Server metrics.

### Option 3: Grafana API (Scripted Import)

Use this method for automated deployments or CI/CD pipelines.

**Step 1: Get the admin password**

```bash
export GRAFANA_PASSWORD=$(kubectl get secret grafana -n monitoring -o jsonpath='{.data.admin-password}' | base64 -d)
```

**Step 2: Port-forward to Grafana**

In a separate terminal (or background the process):

```bash
kubectl port-forward svc/grafana -n monitoring 3000:80 &
```

**Step 3: Create the dashboard JSON file**

Create a file named `mssql-dashboard.json`:

```bash
nano mssql-dashboard.json
```

Paste your dashboard JSON (see examples below in this document).

**Step 4: Import the dashboard via API**

```bash
curl -X POST http://admin:$GRAFANA_PASSWORD@localhost:3000/api/dashboards/db \
  -H 'Content-Type: application/json' \
  -d @mssql-dashboard.json
```

**Expected output:**
```json
{"id":1,"slug":"sql-server-overview","status":"success","uid":"mssql-overview","url":"/d/mssql-overview/sql-server-overview","version":1}
```

**Step 5: Verify in Grafana**

Open http://localhost:3000/dashboards to see your imported dashboard.

## SQL Server Overview Dashboard

This dashboard provides a high-level view of SQL Server instance health including status, connections, and memory usage.

### Panels

| Panel | Metric | Description |
|-------|--------|-------------|
| Instance Status | `mssql_up` | Up/down indicator |
| Connections | `mssql_connections` | Current connection count |
| Batch Requests | `mssql_batch_requests_per_sec` | Throughput |
| Memory Usage | `mssql_server_memory_used_bytes` | Buffer pool |
| CPU Usage | `mssql_cpu_usage_percent` | Instance CPU |

### Creating the Dashboard

**Step 1: Create the dashboard JSON file**

Create a file named `mssql-overview-dashboard.json`:

```bash
# On Linux/macOS
nano mssql-overview-dashboard.json

# On Windows (PowerShell)
notepad mssql-overview-dashboard.json
```

Paste the following content and save:

### Dashboard JSON

```json
{
  "title": "SQL Server Overview",
  "uid": "mssql-overview",
  "tags": ["mssql", "database"],
  "timezone": "browser",
  "panels": [
    {
      "title": "Instance Status",
      "type": "stat",
      "gridPos": {"x": 0, "y": 0, "w": 4, "h": 4},
      "targets": [
        {
          "expr": "mssql_up{job=\"mssql-exporter\"}",
          "legendFormat": "{{instance}}"
        }
      ],
      "fieldConfig": {
        "defaults": {
          "mappings": [
            {"type": "value", "options": {"0": {"text": "DOWN", "color": "red"}}},
            {"type": "value", "options": {"1": {"text": "UP", "color": "green"}}}
          ],
          "thresholds": {
            "mode": "absolute",
            "steps": [
              {"value": null, "color": "red"},
              {"value": 1, "color": "green"}
            ]
          }
        }
      }
    },
    {
      "title": "Active Connections",
      "type": "timeseries",
      "gridPos": {"x": 4, "y": 0, "w": 10, "h": 8},
      "targets": [
        {
          "expr": "mssql_connections{job=\"mssql-exporter\"}",
          "legendFormat": "{{instance}}"
        }
      ]
    },
    {
      "title": "Batch Requests/sec",
      "type": "timeseries",
      "gridPos": {"x": 14, "y": 0, "w": 10, "h": 8},
      "targets": [
        {
          "expr": "rate(mssql_batch_requests_total[5m])",
          "legendFormat": "{{instance}}"
        }
      ]
    },
    {
      "title": "Memory Usage",
      "type": "gauge",
      "gridPos": {"x": 0, "y": 8, "w": 6, "h": 6},
      "targets": [
        {
          "expr": "mssql_server_memory_used_bytes / mssql_server_memory_total_bytes * 100"
        }
      ],
      "fieldConfig": {
        "defaults": {
          "unit": "percent",
          "max": 100,
          "thresholds": {
            "steps": [
              {"value": 0, "color": "green"},
              {"value": 70, "color": "yellow"},
              {"value": 90, "color": "red"}
            ]
          }
        }
      }
    }
  ]
}
```

**Step 2: Import the dashboard**

Choose one of the methods from [Installing Dashboards](#installing-dashboards) above.

**Using the API method:**

```bash
# Make sure you have the Grafana password exported
export GRAFANA_PASSWORD=$(kubectl get secret grafana -n monitoring -o jsonpath='{.data.admin-password}' | base64 -d)

# Port-forward if not already running
kubectl port-forward svc/grafana -n monitoring 3000:80 &

# Import the dashboard
curl -X POST "http://admin:$GRAFANA_PASSWORD@localhost:3000/api/dashboards/db" \
  -H 'Content-Type: application/json' \
  -d '{"dashboard": '"$(cat mssql-overview-dashboard.json)"', "overwrite": true}'
```

**Expected output:**
```json
{"id":1,"slug":"sql-server-overview","status":"success","uid":"mssql-overview","url":"/d/mssql-overview/sql-server-overview","version":1}
```

**Step 3: View the dashboard**

Open http://localhost:3000/d/mssql-overview/sql-server-overview in your browser.

## Availability Group Dashboard

This dashboard monitors SQL Server Availability Group health, synchronization state, and replica status.

### Panels

| Panel | Metric | Description |
|-------|--------|-------------|
| AG Role | `mssql_ag_is_primary` | Primary/Secondary per pod |
| Sync State | `mssql_ag_database_sync_state` | Per-database sync |
| Replica Health | `mssql_ag_replica_connected_state` | Connection status |
| Redo Queue | `mssql_ag_redo_queue_size` | Secondary lag |
| Failover Count | `changes(mssql_ag_is_primary[1h])` | Failovers/hour |

### Creating the Dashboard

**Step 1: Create the dashboard JSON file**

Create a file named `mssql-ag-dashboard.json`:

```bash
nano mssql-ag-dashboard.json
```

Paste the following content and save:

### Dashboard JSON

```json
{
  "title": "SQL Server Availability Groups",
  "uid": "mssql-ag",
  "tags": ["mssql", "availability-groups", "ha"],
  "panels": [
    {
      "title": "Availability Group Status",
      "type": "table",
      "gridPos": {"x": 0, "y": 0, "w": 24, "h": 6},
      "targets": [
        {
          "expr": "mssql_ag_is_primary{job=\"mssql-exporter\"}",
          "format": "table",
          "instant": true
        }
      ],
      "transformations": [
        {
          "id": "organize",
          "options": {
            "renameByName": {
              "Value": "Is Primary",
              "ag_name": "AG Name",
              "instance": "Instance"
            }
          }
        }
      ]
    },
    {
      "title": "Database Synchronization State",
      "type": "stat",
      "gridPos": {"x": 0, "y": 6, "w": 8, "h": 6},
      "targets": [
        {
          "expr": "mssql_ag_database_synchronization_state{job=\"mssql-exporter\"}",
          "legendFormat": "{{database}}"
        }
      ],
      "fieldConfig": {
        "defaults": {
          "mappings": [
            {"type": "value", "options": {"0": {"text": "NOT SYNCHRONIZING"}}},
            {"type": "value", "options": {"1": {"text": "SYNCHRONIZING"}}},
            {"type": "value", "options": {"2": {"text": "SYNCHRONIZED"}}}
          ],
          "thresholds": {
            "steps": [
              {"value": 0, "color": "red"},
              {"value": 1, "color": "yellow"},
              {"value": 2, "color": "green"}
            ]
          }
        }
      }
    },
    {
      "title": "Redo Queue Size",
      "type": "timeseries",
      "gridPos": {"x": 8, "y": 6, "w": 16, "h": 6},
      "targets": [
        {
          "expr": "mssql_ag_redo_queue_size{job=\"mssql-exporter\"}",
          "legendFormat": "{{instance}} - {{database}}"
        }
      ],
      "fieldConfig": {
        "defaults": {
          "unit": "bytes"
        }
      }
    },
    {
      "title": "Failovers (Last 24h)",
      "type": "stat",
      "gridPos": {"x": 0, "y": 12, "w": 6, "h": 4},
      "targets": [
        {
          "expr": "changes(mssql_ag_is_primary{job=\"mssql-exporter\"}[24h])",
          "legendFormat": "{{ag_name}}"
        }
      ]
    }
  ]
}
```

**Step 2: Import the dashboard**

Use the same import method as described above for the Overview dashboard.

## Query Performance Dashboard

This dashboard helps identify query performance issues including compilations, lock waits, and deadlocks.

### Panels

| Panel | Metric | Description |
|-------|--------|-------------|
| Query Compilations | `mssql_compilations_per_sec` | Plan compilations |
| Recompilations | `mssql_recompilations_per_sec` | Plan recompilations |
| Page Life Expectancy | `mssql_page_life_expectancy` | Buffer pool health |
| Lock Waits | `mssql_lock_wait_time_ms` | Blocking |
| Deadlocks | `mssql_deadlocks_per_sec` | Deadlock rate |

### Creating the Dashboard

**Step 1: Create the dashboard JSON file**

Create a file named `mssql-performance-dashboard.json`:

```bash
nano mssql-performance-dashboard.json
```

Paste the following content and save:

### Dashboard JSON

```json
{
  "title": "SQL Server Query Performance",
  "uid": "mssql-performance",
  "tags": ["mssql", "performance"],
  "panels": [
    {
      "title": "Query Compilations",
      "type": "timeseries",
      "gridPos": {"x": 0, "y": 0, "w": 12, "h": 8},
      "targets": [
        {
          "expr": "rate(mssql_compilations_total[5m])",
          "legendFormat": "Compilations - {{instance}}"
        },
        {
          "expr": "rate(mssql_recompilations_total[5m])",
          "legendFormat": "Recompilations - {{instance}}"
        }
      ]
    },
    {
      "title": "Page Life Expectancy",
      "type": "timeseries",
      "gridPos": {"x": 12, "y": 0, "w": 12, "h": 8},
      "targets": [
        {
          "expr": "mssql_page_life_expectancy{job=\"mssql-exporter\"}",
          "legendFormat": "{{instance}}"
        }
      ],
      "fieldConfig": {
        "defaults": {
          "unit": "s",
          "thresholds": {
            "steps": [
              {"value": 0, "color": "red"},
              {"value": 300, "color": "yellow"},
              {"value": 600, "color": "green"}
            ]
          }
        }
      }
    },
    {
      "title": "Lock Waits",
      "type": "timeseries",
      "gridPos": {"x": 0, "y": 8, "w": 12, "h": 8},
      "targets": [
        {
          "expr": "rate(mssql_lock_wait_time_ms_total[5m])",
          "legendFormat": "{{instance}}"
        }
      ],
      "fieldConfig": {
        "defaults": {"unit": "ms"}
      }
    },
    {
      "title": "Deadlocks",
      "type": "stat",
      "gridPos": {"x": 12, "y": 8, "w": 6, "h": 4},
      "targets": [
        {
          "expr": "sum(increase(mssql_deadlocks_total[1h]))",
          "legendFormat": "Last Hour"
        }
      ]
    }
  ]
}
```

**Step 2: Import the dashboard**

Use the same import method as described for the Overview dashboard above.

## Resource Utilization Dashboard

This dashboard monitors CPU, memory, and disk I/O for capacity planning.

### Panels

| Panel | Metric | Description |
|-------|--------|-------------|
| CPU Usage | `mssql_cpu_usage_percent` | Instance CPU |
| Memory Breakdown | `mssql_server_memory_*` | Memory components |
| Disk I/O | `mssql_io_*` | Reads/writes |
| Disk Latency | `mssql_io_latency_*` | I/O latency |
| Buffer Cache Hit | `mssql_buffer_cache_hit_ratio` | Cache efficiency |

### Creating the Dashboard

**Step 1: Create the dashboard JSON file**

Create a file named `mssql-resources-dashboard.json`:

```bash
nano mssql-resources-dashboard.json
```

Paste the following content and save:

### Dashboard JSON

```json
{
  "title": "SQL Server Resources",
  "uid": "mssql-resources",
  "tags": ["mssql", "resources"],
  "panels": [
    {
      "title": "CPU Usage",
      "type": "gauge",
      "gridPos": {"x": 0, "y": 0, "w": 6, "h": 6},
      "targets": [
        {
          "expr": "mssql_cpu_usage_percent{job=\"mssql-exporter\"}"
        }
      ],
      "fieldConfig": {
        "defaults": {
          "unit": "percent",
          "max": 100,
          "thresholds": {
            "steps": [
              {"value": 0, "color": "green"},
              {"value": 70, "color": "yellow"},
              {"value": 90, "color": "red"}
            ]
          }
        }
      }
    },
    {
      "title": "Memory Breakdown",
      "type": "piechart",
      "gridPos": {"x": 6, "y": 0, "w": 8, "h": 6},
      "targets": [
        {
          "expr": "mssql_server_memory_used_bytes",
          "legendFormat": "Used"
        },
        {
          "expr": "mssql_server_memory_total_bytes - mssql_server_memory_used_bytes",
          "legendFormat": "Available"
        }
      ]
    },
    {
      "title": "Disk I/O (IOPS)",
      "type": "timeseries",
      "gridPos": {"x": 14, "y": 0, "w": 10, "h": 8},
      "targets": [
        {
          "expr": "rate(mssql_io_reads_total[5m])",
          "legendFormat": "Reads - {{instance}}"
        },
        {
          "expr": "rate(mssql_io_writes_total[5m])",
          "legendFormat": "Writes - {{instance}}"
        }
      ]
    },
    {
      "title": "Buffer Cache Hit Ratio",
      "type": "stat",
      "gridPos": {"x": 0, "y": 6, "w": 6, "h": 4},
      "targets": [
        {
          "expr": "mssql_buffer_cache_hit_ratio{job=\"mssql-exporter\"}"
        }
      ],
      "fieldConfig": {
        "defaults": {
          "unit": "percent",
          "thresholds": {
            "steps": [
              {"value": 0, "color": "red"},
              {"value": 90, "color": "yellow"},
              {"value": 99, "color": "green"}
            ]
          }
        }
      }
    }
  ]
}
```

**Step 2: Import the dashboard**

Use the same import method as described for the Overview dashboard above.

## Custom Dashboard Tips

When creating your own custom dashboards, use these features for better flexibility.

### Variables

Add dashboard variables for filtering by instance or database. Create a dashboard JSON with a templating section:

```json
{
  "templating": {
    "list": [
      {
        "name": "instance",
        "type": "query",
        "query": "label_values(mssql_up, instance)",
        "refresh": 1
      },
      {
        "name": "database",
        "type": "query",
        "query": "label_values(mssql_database_size_bytes, database)",
        "refresh": 1
      }
    ]
  }
}
```

Then use `$instance` or `$database` in your panel queries:
```promql
mssql_connections{instance="$instance"}
```

### Annotations

Mark events on graphs such as failovers. Add an annotations section to your dashboard JSON:

```json
{
  "annotations": {
    "list": [
      {
        "name": "Failovers",
        "datasource": "Prometheus",
        "expr": "changes(mssql_ag_is_primary[1m]) > 0",
        "tagKeys": "ag_name"
      }
    ]
  }
}
```

### Row Repeats

Repeat a row of panels for each instance automatically. Add to your row definition:

```json
{
  "title": "Instance: $instance",
  "type": "row",
  "repeat": "instance",
  "repeatDirection": "h"
}
```

This creates one row per instance, each showing that instance's metrics.

## Exporting Dashboards

To export an existing dashboard for backup or sharing:

**Step 1: Access the dashboard settings**

1. Open the dashboard in Grafana
2. Click the **gear icon** (Settings) in the top right
3. Click **JSON Model** in the left sidebar

**Step 2: Copy the JSON**

Copy the entire JSON displayed. 

**Step 3: Save to a file**

Create a file and paste the JSON:

```bash
nano my-exported-dashboard.json
```

You can now import this dashboard to other Grafana instances.

## Next Steps

- [SQL Exporter Reference](sql-exporter-reference.md) - All available metrics
- [Prometheus Setup](prometheus-setup.md) - Alert configuration
- [Operations Guide](../operations/upgrades.md) - Maintenance windows
