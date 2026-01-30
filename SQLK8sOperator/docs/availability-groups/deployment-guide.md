# AG Deployment Guide

[← Back to Availability Groups](overview.md) | [Documentation Home](../README.md)

Step-by-step guide to deploying a SQL Server Availability Group on Kubernetes.

## Table of Contents

- [Overview](#overview)
- [Prerequisites](#prerequisites)
- [Step 1: Deploy SQL Server Replicas](#step-1-deploy-sql-server-replicas)
- [Step 2: Configure AG via T-SQL](#step-2-configure-ag-via-t-sql)
- [Step 3: Verify AG Helper Detection](#step-3-verify-ag-helper-detection)
- [Step 4: Enable Kubernetes AG Management (Optional)](#step-4-enable-kubernetes-ag-management-optional)
- [Quick Reference](#quick-reference)

## Overview

### Deployment Approaches

| Approach | Description | When to Use |
|----------|-------------|-------------|
| **Auto-Discovery** | AG Helper monitors all AGs automatically | Default, simplest setup |
| **Explicit Management** | SQLServerAG CR for K8s services + controller failover | Need LoadBalancer services or controller failover |

### Correct Deployment Order

```
┌─────────────────────────────────────────────────────────────────────┐
│  STEP 1: Deploy SQL Replicas                                       │
│  ─────────────────────────────                                      │
│  - Apply ag-step1-replicas.yaml                                     │
│  - Wait for pods to be Running (2/2 Ready)                          │
│  - AG Helper starts in "waiting" mode                               │
│                                                                     │
│  STEP 2: Configure AG via T-SQL                                     │
│  ─────────────────────────────────                                  │
│  - Create AG Helper login on all replicas                           │
│  - Create certificates and endpoints                                │
│  - Create databases and backup                                      │
│  - Create Availability Group on primary                             │
│  - Join secondaries to AG                                           │
│                                                                     │
│  STEP 3: Verify AG Helper Detection                                 │
│  ─────────────────────────────────────                              │
│  - Check AG Helper logs for "Discovered N Availability Group(s)"    │
│  - Verify health monitoring is active                               │
│                                                                     │
│  STEP 4: Enable K8s AG Management (OPTIONAL)                        │
│  ────────────────────────────────────────────                       │
│  - Apply ag-step3-ag-config.yaml                                    │
│  - Creates LoadBalancer services for primary/secondary routing      │
│  - Enables controller-managed automatic failover                    │
└─────────────────────────────────────────────────────────────────────┘
```

> **Important:** The AG Helper requires the AG to exist before it can monitor it. Always complete T-SQL setup (Step 2) before expecting AG Helper to report healthy status.

## Prerequisites

Before deploying an AG, ensure:

| Requirement | How to Check |
|-------------|--------------|
| Operator installed | `kubectl get deployment -n mssql-operator-system` |
| 3+ nodes (recommended) | `kubectl get nodes` |
| Storage provisioner | `kubectl get storageclass` |
| Sufficient resources | 4+ CPU, 8+ GB per replica |

## Step 1: Deploy SQL Server Replicas

Deploy SQL Server with AG Helper in auto-discovery mode.

### Using Sample Manifest

```bash
kubectl apply -f samples/ag-step1-replicas.yaml
```

This creates:
- `mssql` namespace
- SQLServer CR with 3 replicas (`sql-ag-0`, `sql-ag-1`, `sql-ag-2`)
- SA password secret
- AG Helper credentials secret
- AG Helper in auto-discovery mode

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
    replicas: 3
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
  # AG Helper in auto-discovery mode
  agHelper:
    enabled: true
    image: mssql-ag-helper:latest
    credentials:
      secretRef:
        usernameSecret:
          name: sql-ag-helper
          key: username
        passwordSecret:
          name: sql-ag-helper
          key: password
```

### Wait for Pods

```bash
kubectl get pods -n mssql -w

# Wait until all 3 pods show 2/2 Ready
# NAME        READY   STATUS    RESTARTS   AGE
# sql-ag-0    2/2     Running   0          5m
# sql-ag-1    2/2     Running   0          4m
# sql-ag-2    2/2     Running   0          3m
```

### Verify AG Helper is Waiting

```bash
kubectl logs sql-ag-0 -n mssql -c ag-helper | tail -5

# Expected output:
# [INFO] No Availability Groups found, waiting for AG configuration...
```

## Step 2: Configure AG via T-SQL

The complete T-SQL setup is documented in [samples/ag-step2-setup-ag.md](../../samples/ag-step2-setup-ag.md).

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

See [samples/ag-step2-setup-ag.md](../../samples/ag-step2-setup-ag.md) for complete certificate exchange steps including `kubectl cp` commands.

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
          ENDPOINT_URL = N'TCP://sql-ag-0.sql-ag-headless.mssql.svc.cluster.local:5022',
          AVAILABILITY_MODE = SYNCHRONOUS_COMMIT,
          FAILOVER_MODE = EXTERNAL,
          SEEDING_MODE = AUTOMATIC,
          SECONDARY_ROLE (ALLOW_CONNECTIONS = READ_ONLY)),
        N'sql-ag-1' WITH (
          ENDPOINT_URL = N'TCP://sql-ag-1.sql-ag-headless.mssql.svc.cluster.local:5022',
          AVAILABILITY_MODE = SYNCHRONOUS_COMMIT,
          FAILOVER_MODE = EXTERNAL,
          SEEDING_MODE = AUTOMATIC,
          SECONDARY_ROLE (ALLOW_CONNECTIONS = READ_ONLY)),
        N'sql-ag-2' WITH (
          ENDPOINT_URL = N'TCP://sql-ag-2.sql-ag-headless.mssql.svc.cluster.local:5022',
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

## Step 3: Verify AG Helper Detection

After T-SQL setup, AG Helper automatically detects the new AG.

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

## Step 4: Enable Kubernetes AG Management (Optional)

If you need **LoadBalancer services** or **controller-managed failover**, apply a SQLServerAG resource.

### When to Apply Step 4

| Need | Auto-Discovery | SQLServerAG CR |
|------|----------------|----------------|
| Health monitoring | ✅ Built-in | ✅ |
| Primary/Secondary Services | ❌ | ✅ |
| Controller-managed failover | ❌ | ✅ |
| kubectl get sqlserverag | ❌ | ✅ |

### Apply AG Configuration

```bash
kubectl apply -f samples/ag-step3-ag-config.yaml
```

Or create manually:

```yaml
apiVersion: mssql.microsoft.com/v1alpha1
kind: SQLServerAG
metadata:
  name: production-ag
  namespace: mssql
spec:
  sqlServerRef:
    name: sql-ag
  availabilityGroup:
    name: ProductionAG  # Must match T-SQL AG name
    replicas: 3
    primaryConfig:
      availabilityMode: SynchronousCommit
      failoverMode: External
    secondaryConfig:
      availabilityMode: SynchronousCommit
      failoverMode: External
    automaticFailover: true
    healthCheckCredentials:
      secretRef:
        usernameSecret:
          name: sql-ag-helper
          key: username
        passwordSecret:
          name: sql-ag-helper
          key: password
  endpoints:
    primary:
      type: LoadBalancer
      port: 1433
    secondary:
      type: LoadBalancer
      port: 1434
  failover:
    automatic: true
    healthCheckTimeout: "30s"
```

### Verify Services Created

```bash
kubectl get svc -n mssql

# NAME                      TYPE           CLUSTER-IP     EXTERNAL-IP   PORT(S)
# production-ag-primary     LoadBalancer   10.0.100.10    <pending>     1433/TCP
# production-ag-secondary   LoadBalancer   10.0.100.11    <pending>     1434/TCP
```

### Connect via Primary Service

```bash
# For minikube
minikube tunnel

# Get external IP
kubectl get svc production-ag-primary -n mssql

# Connect
sqlcmd -S <EXTERNAL_IP>,1433 -U sa -P 'YourStrong@Passw0rd!'
```

## Quick Reference

### Sample Files

| File | Purpose |
|------|---------|
| [ag-step1-replicas.yaml](../../samples/ag-step1-replicas.yaml) | Step 1: SQL replicas + AG Helper |
| [ag-step2-setup-ag.md](../../samples/ag-step2-setup-ag.md) | Step 2: T-SQL setup guide |
| [ag-step3-ag-config.yaml](../../samples/ag-step3-ag-config.yaml) | Step 4: K8s AG management |
| [ag-step3-multi-ag.yaml](../../samples/ag-step3-multi-ag.yaml) | Advanced: Multiple AGs |

### Common Commands

```bash
# Check pod status
kubectl get pods -n mssql

# Check AG Helper logs
kubectl logs sql-ag-0 -n mssql -c ag-helper

# Check AG state
kubectl exec -it sql-ag-0 -n mssql -c ag-helper -- curl -s localhost:8080/state | jq

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
