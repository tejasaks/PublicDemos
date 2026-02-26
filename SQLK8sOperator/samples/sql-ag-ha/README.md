# SQL Server AG — High Availability Sample

Self-contained sample that deploys a **3-replica synchronous-commit Availability Group**
with automatic failover and an AG Listener on Kubernetes.

## Scenario

| Property | Value |
|----------|-------|
| Replicas | 3 (sql-ag-0, sql-ag-1, sql-ag-2) |
| Sync mode | All synchronous commit |
| Failover | Automatic (controller-managed) |
| Listener | Yes (single VIP routing to primary) |
| Read-only routing | Secondaries accept read-only connections |

## Files

| File | Description |
|------|-------------|
| `ag-deploy.yaml` | All Kubernetes resources (namespace, secrets, SQLServer, services, SQLServerAG) |
| `ag-configure.md` | Step-by-step T-SQL guide to create the AG inside SQL Server |
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

### 4. Set Up the AG Listener

```bash
./ag-configure.sh listener
```

Or follow Step 9 in [ag-configure.md](ag-configure.md).

### 5. Verify

```bash
# Kubernetes status
kubectl get sqlserverag -n mssql
kubectl get sqlserver -n mssql

# AG health via T-SQL
./ag-configure.sh verify
```

## Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│  Namespace: mssql                                                   │
│                                                                     │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐              │
│  │  sql-ag-0    │  │  sql-ag-1    │  │  sql-ag-2    │              │
│  │  (PRIMARY)   │  │  (SECONDARY) │  │  (SECONDARY) │              │
│  │  Sync Commit │  │  Sync Commit │  │  Sync Commit │              │
│  │  :1433 :5022 │  │  :1433 :5022 │  │  :1433 :5022 │              │
│  └──────┬───────┘  └──────┬───────┘  └──────┬───────┘              │
│         │                 │                 │                       │
│         └────────┬────────┴────────┬────────┘                       │
│                  │                 │                                 │
│         ┌────────▼──────┐  ┌───────▼───────┐                       │
│         │ AG Listener   │  │ AG Helper Pod │                       │
│         │ (VIP → primary│  │ (health mon.) │                       │
│         │  :1433)       │  │               │                       │
│         └───────────────┘  └───────────────┘                       │
│                                                                     │
│  Operator: automatic failover, status tracking                      │
└─────────────────────────────────────────────────────────────────────┘
```

## Useful Commands

```bash
# Check which replica is primary
kubectl get sqlserverag production-ag -n mssql -o jsonpath='{.status.primaryReplica}'

# Manual failover to sql-ag-1
kubectl annotate sqlserverag production-ag -n mssql \
  mssql.microsoft.com/failover-to=sql-ag-1

# Get listener VIP
kubectl get sqlserverag production-ag -n mssql -o jsonpath='{.status.listener.vip}'

# View events
kubectl get events -n mssql --sort-by='.lastTimestamp' | tail -20
```

## Cleanup

```bash
kubectl delete -f ag-deploy.yaml
kubectl delete namespace mssql

# To remove the operator:
kubectl delete -f https://raw.githubusercontent.com/tejasaks/PublicDemos/main/SQLK8sOperator/install.yaml
```
