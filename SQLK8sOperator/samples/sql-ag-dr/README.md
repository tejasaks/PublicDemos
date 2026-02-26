# SQL Server AG — Disaster Recovery Sample

Self-contained sample that deploys a **DR-oriented Availability Group** with
2 synchronous-commit replicas for local HA and 1 asynchronous-commit replica
for geo-disaster recovery. Manual failover only.

## Scenario

| Property | Value |
|----------|-------|
| Replicas | 3 (sql-ag-0, sql-ag-1, sql-ag-2) |
| Sync mode | sql-ag-0/1: synchronous, sql-ag-2: **asynchronous** |
| Failover | **Manual only** (DBA controlled) |
| Listener | None (direct replica access) |
| DB name | CriticalDB |
| DR replica | sql-ag-2 (not readable, standby only) |

## Files

| File | Description |
|------|-------------|
| `ag-deploy.yaml` | All K8s resources (SQLServer, services, SQLServerAG with manual failover) |
| `ag-configure.md` | Step-by-step T-SQL guide |
| `ag-configure.sh` | Automated shell script |

## Quick Start

### 1. Install the Operator

```bash
kubectl apply -f https://raw.githubusercontent.com/tejasaks/PublicDemos/main/SQLK8sOperator/install.yaml
```

### 2. Deploy

```bash
kubectl apply -f ag-deploy.yaml
kubectl get pods -n mssql -w
```

### 3. Configure

```bash
chmod +x ag-configure.sh
./ag-configure.sh
```

Or follow [ag-configure.md](ag-configure.md).

### 4. Verify

```bash
kubectl get sqlserverag -n mssql
./ag-configure.sh verify
```

## Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│  Namespace: mssql                                                   │
│                                                                     │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐              │
│  │  sql-ag-0    │  │  sql-ag-1    │  │  sql-ag-2    │              │
│  │  PRIMARY     │  │  SECONDARY   │  │  DR STANDBY  │              │
│  │  Sync Commit │  │  Sync Commit │  │  Async Commit│              │
│  │  :1433 :5022 │  │  :1433 :5022 │  │  :1433 :5022 │              │
│  └──────────────┘  └──────────────┘  └──────────────┘              │
│         │                 │                 │                       │
│  ┌──────▼─────────────────▼─────────────────▼──────┐               │
│  │              AG Helper Pod (monitoring)          │               │
│  │              Manual failover via kubectl         │               │
│  └──────────────────────────────────────────────────┘               │
│                                                                     │
│  No Listener — connect directly to replica services                 │
└─────────────────────────────────────────────────────────────────────┘
```

## Manual Failover

```bash
# Failover to sync secondary (no data loss)
kubectl annotate sqlserverag dr-ag -n mssql \
  mssql.microsoft.com/failover-to=sql-ag-1

# Failover to DR replica (potential data loss — disaster scenario only!)
kubectl annotate sqlserverag dr-ag -n mssql \
  mssql.microsoft.com/failover-to=sql-ag-2

# Check current primary
kubectl get sqlserverag dr-ag -n mssql -o jsonpath='{.status.primaryReplica}'
```

## Cleanup

```bash
kubectl delete -f ag-deploy.yaml
kubectl delete namespace mssql
```
