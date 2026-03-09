# SQL Server AG — High Availability (Minimal)

Absolute-minimum deployment of a **3-replica synchronous-commit Availability Group**.
Only required fields are specified — everything else uses operator defaults.

## Scenario

| Property | Value |
|----------|-------|
| Replicas | 3 (sql-ag-0, sql-ag-1, sql-ag-2) |
| Sync mode | All synchronous commit |
| Failover | Automatic (controller-managed) |
| Listener | No (use pod DNS or add one later) |
| AG Name | ag1 |
| Database | SampleDB |

## Files

| File | Description |
|------|-------------|
| `ag-deploy.yaml` | Namespace, secrets, SQLServer (3 replicas), SQLServerAG — minimal fields only |
| `ag-configure.md` | Step-by-step T-SQL guide to set up the AG inside SQL Server |
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

### 4. Verify

```bash
kubectl get sqlserverag -n mssql
kubectl get sqlserver -n mssql

./ag-configure.sh verify
```

## What's Different from sql-ag-ha

This sample omits every optional field to show the absolute minimum needed:

| Feature | sql-ag-ha | This (minimal) |
|---------|:---------:|:---------------:|
| Listener | ✅ | ❌ |
| Services per replica | ✅ | ❌ |
| Database mirroring endpoint port | Explicit (5022) | Default (5022) |
| Sidecar config | Explicit | Default |
| Failover config | Explicit | Default |

For the full-options reference see [sql-ag-ha-full](../sql-ag-ha-full/).
