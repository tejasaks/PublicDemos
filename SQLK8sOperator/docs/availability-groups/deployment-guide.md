# AG Deployment Guide

[← Back to Availability Groups](overview.md) | [Documentation Home](../README.md)

Step-by-step guide to deploying a SQL Server Availability Group on Kubernetes.

## Table of Contents

- [Overview](#overview)
- [Prerequisites](#prerequisites)
- [Step 1: Deploy SQL Server Replicas](#step-1-deploy-sql-server-replicas)
- [Step 2: Configure AG via T-SQL](#step-2-configure-ag-via-t-sql)
- [Step 3: Deploy AG Helper and K8s Services](#step-3-deploy-ag-helper-and-k8s-services)
- [Step 4: Verify AG Helper Detection](#step-4-verify-ag-helper-detection)
- [Quick Reference](#quick-reference)

## Overview

### Architecture

Each SQLServerAG resource manages exactly **one** Availability Group:
- The AG name must be specified explicitly
- For multiple AGs, create multiple SQLServerAG resources

### Failover Modes

| Mode | automaticFailover | Description | When to Use |
|------|-------------------|-------------|-------------|
| **Monitoring Only** | `false` (default) | Operator monitors AG health; failover is manual | Dev/test, DBA-controlled failover |
| **Automatic Failover** | `true` | Controller automatically promotes secondary when primary fails | Production HA, automated recovery |

### Correct Deployment Order

```
┌─────────────────────────────────────────────────────────────────────┐
│  STEP 1: Deploy SQL Replicas + AG Resources                        │
│  ──────────────────────────────────────────                          │
│  - Apply ag-deploy.yaml from a scenario folder                      │
│    (e.g. samples/sql-ag-ha/ag-deploy.yaml)                         │
│  - Wait for pods to be Running                                      │
│  - SQL Server running, AG Helper sidecar attached                   │
│                                                                     │
│  STEP 2: Configure AG via T-SQL                                     │
│  ─────────────────────────────────                                  │
│  - Follow ag-configure.md or run ag-configure.sh                    │
│  - Create AG Helper login on all replicas                           │
│  - Create certificates and endpoints                                │
│  - Create databases and backup                                      │
│  - Create Availability Group on primary                             │
│  - Join secondaries to AG                                           │
│                                                                     │
│  STEP 3: Verify AG Helper Detection                                 │
│  ────────────────────────────────────────────                       │
│  - Check AG Helper logs for "Discovered N Availability Group(s)"    │
│  - Verify health monitoring is active                               │
│                                                                     │
│  STEP 4: (Optional) Set up AG Listener                              │
│  ──────────────────────────────────────────                          │
│  - Run ag-configure.sh listener                                     │
│  - Verify listener is Ready                                         │
└─────────────────────────────────────────────────────────────────────┘
```

> **Important:** The AG must exist in SQL Server before AG Helper can monitor it. Apply the deploy manifest first, then complete T-SQL setup before the AG Helper can discover the AG.

## Prerequisites

Before deploying an AG, ensure:

| Requirement | How to Check |
|-------------|--------------|
| Operator installed | `kubectl get deployment -n mssql-system` |
| 3+ nodes (recommended) | `kubectl get nodes` |
| Storage provisioner | `kubectl get storageclass` |
| Sufficient resources | 4+ CPU, 8+ GB per replica |

## Step 1: Deploy SQL Server Replicas

Deploy SQL Server replicas configured for Availability Groups.

### Using Sample Manifest

```bash
# High Availability scenario (pick the scenario folder you need)
kubectl apply -f samples/sql-ag-ha/ag-deploy.yaml
```

This creates:
- `mssql` namespace
- SQLServer CR with 3 instances (`sql-ag-0`, `sql-ag-1`, `sql-ag-2`)
- SA password secret
- AG Helper credentials secret (for use in Step 3)

> **Note:** AG Helper sidecar is NOT deployed yet. It will be added in Step 3 after you create the AG via T-SQL.

### Or Create Manually

```bash
# Create namespace and secrets
kubectl create namespace mssql

kubectl create secret generic sql-ag-sa \
  --from-literal=password='YourStrong@Passw0rd!' \
  -n mssql

kubectl create secret generic sql-ag-helper \
  --from-literal=username=ag_helper \
  --from-literal=password='AGHelper@Passw0rd!' \
  -n mssql
```

Then create SQLServer manifest:

```yaml
apiVersion: mssql.microsoft.com/v1alpha1
kind: SQLServer
metadata:
  name: sql-ag
  namespace: mssql
spec:
  version: "2025"
  edition: Developer
  instance:
    count: 3
    resources:
      limits:
        cpu: "4"
        memory: 8Gi
      requests:
        cpu: "2"
        memory: 4Gi
    storage:
      data:
        size: 50Gi
      log:
        size: 20Gi
      backup:
        size: 50Gi
    config:
      hadrEnabled: true   # Required for AG
      agentEnabled: true
  credentials:
    saPasswordSecretRef:
      name: sql-ag-sa
      key: password
  service:
    type: LoadBalancer
    port: 1433
```

### Wait for Pods

```bash
kubectl get pods -n mssql -w

# Wait until all 3 pods show 1/1 Ready (no AG Helper yet)
# NAME        READY   STATUS    RESTARTS   AGE
# sql-ag-0    1/1     Running   0          5m
# sql-ag-1    1/1     Running   0          4m
# sql-ag-2    1/1     Running   0          3m
```

## Step 2: Configure AG via T-SQL

The complete T-SQL setup is documented in each scenario folder's `ag-configure.md`, e.g. [sql-ag-ha/ag-configure.md](../../samples/sql-ag-ha/ag-configure.md).

### Quick Summary

| Step | Description | Run On |
|------|-------------|--------|
| 2.1 | Create AG Helper login | ALL replicas |
| 2.2 | Create master key and certificates | ALL replicas |
| 2.3 | Exchange certificates between replicas | kubectl + T-SQL |
| 2.4 | Create database mirroring endpoints | ALL replicas |
| 2.5 | Create databases | Primary only |
| 2.6 | Create Availability Group | Primary only |
| 2.7 | Join secondary replicas | Secondaries only |
| 2.8 | Verify AG status | Any replica |

### Step 2.1: Create AG Helper Login

Run on **ALL replicas**:

```bash
for i in 0 1 2; do
  kubectl exec -it sql-ag-$i -n mssql -c mssql -- \
    /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa \
    -P 'YourStrong@Passw0rd!' -C -Q "
      CREATE LOGIN ag_helper WITH PASSWORD = 'AGHelper@Passw0rd!';
      GRANT VIEW SERVER STATE TO ag_helper;
      GRANT ALTER ANY AVAILABILITY GROUP TO ag_helper;
    "
done
```

### Step 2.2-2.4: Certificates and Endpoints

See [sql-ag-ha/ag-configure.md](../../samples/sql-ag-ha/ag-configure.md) for complete certificate exchange steps including `kubectl cp` commands, or run the automated `ag-configure.sh` script.

### Step 2.5: Create Database

Run on **PRIMARY** (sql-ag-0):

```bash
kubectl exec -it sql-ag-0 -n mssql -c mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa \
  -P 'YourStrong@Passw0rd!' -C -Q "
    CREATE DATABASE ApplicationDB;
    ALTER DATABASE ApplicationDB SET RECOVERY FULL;
    BACKUP DATABASE ApplicationDB 
      TO DISK = '/var/opt/mssql/backup/ApplicationDB_init.bak' WITH INIT;
  "
```

### Step 2.6: Create Availability Group

Run on **PRIMARY** (sql-ag-0):

```bash
kubectl exec -it sql-ag-0 -n mssql -c mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa \
  -P 'YourStrong@Passw0rd!' -C -Q "
    CREATE AVAILABILITY GROUP ProductionAG
      WITH (CLUSTER_TYPE = EXTERNAL, DB_FAILOVER = ON,
            REQUIRED_SYNCHRONIZED_SECONDARIES_TO_COMMIT = 1)
      FOR DATABASE ApplicationDB
      REPLICA ON
        N'sql-ag-0' WITH (
          ENDPOINT_URL = N'TCP://sql-ag-0.mssql.svc.cluster.local:5022',
          AVAILABILITY_MODE = SYNCHRONOUS_COMMIT,
          FAILOVER_MODE = EXTERNAL,
          SEEDING_MODE = AUTOMATIC,
          SECONDARY_ROLE (ALLOW_CONNECTIONS = READ_ONLY)),
        N'sql-ag-1' WITH (
          ENDPOINT_URL = N'TCP://sql-ag-1.mssql.svc.cluster.local:5022',
          AVAILABILITY_MODE = SYNCHRONOUS_COMMIT,
          FAILOVER_MODE = EXTERNAL,
          SEEDING_MODE = AUTOMATIC,
          SECONDARY_ROLE (ALLOW_CONNECTIONS = READ_ONLY)),
        N'sql-ag-2' WITH (
          ENDPOINT_URL = N'TCP://sql-ag-2.mssql.svc.cluster.local:5022',
          AVAILABILITY_MODE = SYNCHRONOUS_COMMIT,
          FAILOVER_MODE = EXTERNAL,
          SEEDING_MODE = AUTOMATIC,
          SECONDARY_ROLE (ALLOW_CONNECTIONS = READ_ONLY));
  "
```

### Step 2.7: Join Secondaries

Run on **SECONDARIES** (sql-ag-1, sql-ag-2):

```bash
for i in 1 2; do
  kubectl exec -it sql-ag-$i -n mssql -c mssql -- \
    /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa \
    -P 'YourStrong@Passw0rd!' -C -Q "
      ALTER AVAILABILITY GROUP ProductionAG JOIN WITH (CLUSTER_TYPE = EXTERNAL);
      ALTER AVAILABILITY GROUP ProductionAG GRANT CREATE ANY DATABASE;
    "
done
```

## Step 3: Deploy AG Helper and K8s Services

After T-SQL setup, deploy the AG Helper sidecar via SQLServerAG resource.

### Apply the SQLServerAG Manifest

```bash
# AG resources are included in the unified ag-deploy.yaml
kubectl apply -f samples/sql-ag-ha/ag-deploy.yaml
```

This creates:
- AG Helper sidecar on each pod (monitors the specified AG)
- Primary LoadBalancer service (routes to current primary)
- Secondary LoadBalancer service (routes to readable secondaries)

### Failover Behavior

By default, `automaticFailover: false` (monitoring only):
- Operator monitors AG health via AG Helper sidecar
- LoadBalancer services route traffic to primary/secondaries
- **Failover must be triggered manually:**

```bash
# Via T-SQL (on target secondary)
kubectl exec -it sql-ag-1 -n mssql -c mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa \
  -P 'YourStrong@Passw0rd!' -C -Q \
  "ALTER AVAILABILITY GROUP ProductionAG FAILOVER;"

# Or via AG Helper API
kubectl exec -it sql-ag-0 -n mssql -c ag-helper -- \
  curl -X POST localhost:8080/failover \
  -H "Content-Type: application/json" \
  -d '{"targetReplica": "sql-ag-1", "force": false}'
```

### Enable Automatic Failover (Production HA)

For controller-managed automatic failover, set `automaticFailover: true` in the manifest:

```yaml
availabilityGroup:
  name: ProductionAG
  automaticFailover: true  # Enable controller-managed failover
```

With automatic failover enabled:
- Monitors AG health continuously
- Detects primary failure (configurable timeout via `failover.healthCheckTimeout`)
- Automatically promotes best synchronized secondary
- Updates LoadBalancer service endpoints

### Wait for Pods to Update

After applying either option, pods will restart to add the AG Helper sidecar:

```bash
kubectl get pods -n mssql -w

# Wait until all 3 pods show 2/2 Ready
# NAME        READY   STATUS    RESTARTS   AGE
# sql-ag-0    2/2     Running   1          10m
# sql-ag-1    2/2     Running   1          9m
# sql-ag-2    2/2     Running   1          8m
```

## Step 4: Verify AG Helper Detection

Check that AG Helper has discovered and is monitoring the AG.

### Check AG Helper Logs

```bash
kubectl logs sql-ag-0 -n mssql -c ag-helper | tail -10

# Expected output:
# [INFO] Discovered 1 Availability Group(s): [ProductionAG]
# [INFO] AG 'ProductionAG' State: role=PRIMARY, sync=SYNCHRONIZED, health=Healthy
```

### Check AG State via API

```bash
kubectl exec -it sql-ag-0 -n mssql -c ag-helper -- \
  curl -s localhost:8080/state | jq

# Expected:
# {
#   "agName": "ProductionAG",
#   "role": "PRIMARY",
#   "health": "Healthy",
#   ...
# }
```

### Verify Replica States

```bash
kubectl exec -it sql-ag-0 -n mssql -c mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa \
  -P 'YourStrong@Passw0rd!' -C -Q "
    SELECT replica_server_name, role_desc, synchronization_health_desc 
    FROM sys.dm_hadr_availability_replica_states
  "
```

**Expected:** All replicas show `HEALTHY`, primary shows `PRIMARY`, others show `SECONDARY`.

### Verify Services

```bash
kubectl get svc -n mssql

# NAME                         TYPE        CLUSTER-IP     EXTERNAL-IP   PORT(S)
# productionag-listener        ClusterIP   10.0.100.10    <none>        1433/TCP
```

### Connect via Listener Service

```bash
# For minikube
minikube tunnel

# Get service IP
kubectl get svc productionag-listener -n mssql

# Connect
sqlcmd -S <CLUSTER_IP>,1433 -U sa -P 'YourStrong@Passw0rd!'
```

## Quick Reference

### Sample Files

| File | Purpose |
|------|---------|
| [sql-ag-ha/](../../samples/sql-ag-ha/) | High Availability: 3 sync replicas, auto failover, listener |
| [sql-ag-dr/](../../samples/sql-ag-dr/) | Disaster Recovery: 2 sync + 1 async, manual failover |
| [sql-ag-multiag/](../../samples/sql-ag-multiag/) | Multiple AGs on same replicas |
| [sql-ag-monitoring/](../../samples/sql-ag-monitoring/) | HA + Prometheus + Grafana monitoring stack |

### Common Commands

```bash
# Check pod status
kubectl get pods -n mssql

# Check AG Helper logs
kubectl logs sql-ag-0 -n mssql -c ag-helper

# Check AG state
kubectl exec -it sql-ag-0 -n mssql -c ag-helper -- curl -s localhost:8080/state | jq

# Manual failover (when automaticFailover: false)
kubectl exec -it sql-ag-1 -n mssql -c mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa \
  -P 'YourStrong@Passw0rd!' -C -Q \
  "ALTER AVAILABILITY GROUP ProductionAG FAILOVER;"

# Check SQLServerAG resources
kubectl get sqlserverag -n mssql

# Check services
kubectl get svc -n mssql
```

## Next Steps

- [Failover Management](failover-management.md) - Configure automatic failover
- [Multi-AG Scenarios](multi-ag-scenarios.md) - Multiple AGs on same replicas
- [AG Helper Reference](ag-helper-reference.md) - Detailed sidecar documentation
- [Troubleshooting](../user-guide/troubleshooting.md) - Common issues
