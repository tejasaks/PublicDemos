# SQL Server AG — High Availability (Full Configuration)

Deploys a **3-replica synchronous-commit Availability Group** with every CRD field
explicitly specified, documented with defaults, ranges, and recommendations.
Includes a listener, failover tuning, `failureConditionLevel`, and full sidecar advanced config.

## Scenario

| Property | Value |
|----------|-------|
| Replicas | 3 (sql-ag-full-0, sql-ag-full-1, sql-ag-full-2) |
| Sync mode | All synchronous commit |
| Failover | Automatic (controller-managed) |
| Listener | Yes — `ag-listener` (ClusterIP → primary) |
| Read-only routing | Secondaries accept read-only connections |
| Databases | appdb, auditdb |
| Failure condition level | 3 (AG topology + system diagnostics) |
| Monitoring | SQL Exporter sidecar enabled |
| AG Name | ag1 |

## Files

| File | Description |
|------|-------------|
| `ag-deploy.yaml` | All Kubernetes resources with every field listed and annotated |
| `ag-configure.md` | Step-by-step T-SQL guide to create the AG, databases, and listener |
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
# Wait for sql-ag-full-0, sql-ag-full-1, sql-ag-full-2 to be Running
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
kubectl get sqlserverag -n mssql
kubectl get sqlserver -n mssql

./ag-configure.sh verify
```

## What's Different from sql-ag-ha-minimal

| Feature | sql-ag-ha-minimal | This (full) |
|---------|:-----------------:|:-----------:|
| All fields listed | ❌ | ✅ |
| Default/range comments | ❌ | ✅ |
| Listener | ❌ | ✅ |
| Failover config | Default | Explicit (all 6 fields) |
| failureConditionLevel | Default | Level 3 |
| Sidecar advanced tuning | Default | Explicit (all 5 fields) |
| Monitoring | Default | Explicit |
| Multiple databases | ❌ | ✅ (appdb + auditdb) |
| Storage volumes | Data only | Data + Log + TempDB + Backup |

## Customization

> **Passwords:** The sample manifests and scripts ship with placeholder passwords.
> Before deploying, update the Secret resources in `ag-deploy.yaml` and the matching
> variables at the top of `ag-configure.sh`.

> **Instance and AG names:** If you rename `sql-ag-full` or `ag1`, update them
> consistently in `ag-deploy.yaml`, `ag-configure.sh`, and `ag-configure.md`.

## Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│  Namespace: mssql                                                   │
│                                                                     │
│  ┌──────────────────┐ ┌──────────────────┐ ┌──────────────────┐    │
│  │  sql-ag-full-0   │ │  sql-ag-full-1   │ │  sql-ag-full-2   │    │
│  │  (PRIMARY)       │ │  (SECONDARY)     │ │  (SECONDARY)     │    │
│  │  Sync Commit     │ │  Sync Commit     │ │  Sync Commit     │    │
│  │  :1433 :5022     │ │  :1433 :5022     │ │  :1433 :5022     │    │
│  │  + Exporter:9399 │ │  + Exporter:9399 │ │  + Exporter:9399 │    │
│  │  + AG Sidecar    │ │  + AG Sidecar    │ │  + AG Sidecar    │    │
│  └────────┬─────────┘ └────────┬─────────┘ └────────┬─────────┘    │
│           │                    │                    │               │
│           └────────┬───────────┴───────────┬────────┘               │
│                    │                       │                        │
│           ┌────────▼──────┐  ┌─────────────▼───────────┐           │
│           │ ag-listener   │  │ Failover Controller      │           │
│           │ (VIP→primary) │  │ failureConditionLevel: 3 │           │
│           │ :1433         │  │ sp_server_diagnostics    │           │
│           └───────────────┘  └─────────────────────────┘           │
│                                                                     │
│  Databases: appdb, auditdb (auto-seeded to all replicas)           │
└─────────────────────────────────────────────────────────────────────┘
```
