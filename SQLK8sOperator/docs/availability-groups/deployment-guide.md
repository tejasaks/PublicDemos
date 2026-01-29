# AG Deployment Guide

[← Back to Availability Groups](overview.md) | [Documentation Home](../README.md)

Step-by-step guide to deploying a SQL Server Availability Group on Kubernetes.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Step 1: Deploy SQLServer Resource](#step-1-deploy-sqlserver-resource)
- [Step 2: Deploy SQLServerAG Resource](#step-2-deploy-sqlserverag-resource)
- [Step 3: Create AG via T-SQL](#step-3-create-ag-via-t-sql)
- [Step 4: Join Secondary Replicas](#step-4-join-secondary-replicas)
- [Step 5: Verify AG Status](#step-5-verify-ag-status)
- [Complete Example](#complete-example)

## Prerequisites

Before deploying an AG, ensure:

| Requirement | How to Check |
|-------------|--------------|
| Operator installed | `kubectl get deployment -n mssql-operator-system` |
| 3+ nodes (recommended) | `kubectl get nodes` |
| Storage provisioner | `kubectl get storageclass` |
| Sufficient resources | 4+ CPU, 8+ GB per replica |

## Step 1: Deploy SQLServer Resource

Create a SQLServer resource with HADR enabled and 3 replicas.

### Create Password Secret

```bash
kubectl create namespace mssql

kubectl create secret generic sql-ag-prod-sa \
  --from-literal=password='YourStrong@Passw0rd!' \
  -n mssql
```

### Create SQLServer Manifest

```bash
cat > sqlserver-ag.yaml << 'EOF'
apiVersion: mssql.microsoft.com/v1alpha1
kind: SQLServer
metadata:
  name: sql-ag-prod01
  namespace: mssql
spec:
  description: "Production SQL Server AG cluster"
  version: "2022"
  edition: Developer  # Use Enterprise for production
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
    # Distribute pods across nodes
    affinity:
      podAntiAffinity:
        requiredDuringSchedulingIgnoredDuringExecution:
          - labelSelector:
              matchLabels:
                app: mssql
                mssql.microsoft.com/instance: sql-ag-prod01
            topologyKey: kubernetes.io/hostname
  credentials:
    saPasswordSecretRef:
      name: sql-ag-prod-sa
      key: password
  service:
    type: ClusterIP
    port: 1433
  monitoring:
    enabled: true
EOF
```

### Deploy

```bash
kubectl apply -f sqlserver-ag.yaml
```

### Wait for Pods

```bash
kubectl get pods -n mssql -w

# Wait until all 3 pods are Running
# NAME               READY   STATUS    RESTARTS   AGE
# sql-ag-prod01-0    2/2     Running   0          5m
# sql-ag-prod01-1    2/2     Running   0          4m
# sql-ag-prod01-2    2/2     Running   0          3m
```

## Step 2: Deploy SQLServerAG Resource

Create a SQLServerAG resource to define the AG configuration and services.

### Create SQLServerAG Manifest

```bash
cat > sqlserverag.yaml << 'EOF'
apiVersion: mssql.microsoft.com/v1alpha1
kind: SQLServerAG
metadata:
  name: prod-ag-01
  namespace: mssql
spec:
  description: "Production AG for order processing"
  sqlServerRef:
    name: sql-ag-prod01
  availabilityGroup:
    name: ProductionAG
    replicas: 3
    primaryConfig:
      availabilityMode: SynchronousCommit
      failoverMode: External
      readableSecondary: ReadOnly
    secondaryConfig:
      availabilityMode: SynchronousCommit
      failoverMode: External
      readableSecondary: ReadOnly
    seedingMode: Automatic
    databases:
      - name: AppDB
    dbFailover: true
    automaticFailover: true
    endpointPort: 5022
  endpoints:
    primary:
      type: LoadBalancer
      port: 1433
    secondary:
      type: LoadBalancer
      port: 1434
  sidecar:
    monitorInterval: "10s"
EOF
```

### Deploy

```bash
kubectl apply -f sqlserverag.yaml
```

### Verify Services Created

```bash
kubectl get svc -n mssql

# NAME                 TYPE           CLUSTER-IP     EXTERNAL-IP    PORT(S)
# prod-ag-01-primary   LoadBalancer   10.0.100.10    <pending>      1433:31433/TCP
# prod-ag-01-secondary LoadBalancer   10.0.100.11    <pending>      1434:31434/TCP
```

## Step 3: Create AG via T-SQL

The operator creates the infrastructure, but the AG must be configured via T-SQL.

### Connect to Primary

```bash
kubectl exec -it sql-ag-prod01-0 -n mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P 'YourStrong@Passw0rd!' -C
```

### Create Master Key and Certificate

Run on ALL replicas (0, 1, 2):

```sql
-- Create master key
CREATE MASTER KEY ENCRYPTION BY PASSWORD = 'MasterKeyP@ssw0rd!';
GO

-- Create certificate
CREATE CERTIFICATE AG_Cert
    WITH SUBJECT = 'AG Authentication Certificate',
    EXPIRY_DATE = '2030-12-31';
GO

-- Create endpoint
CREATE ENDPOINT AG_Endpoint
    STATE = STARTED
    AS TCP (LISTENER_PORT = 5022)
    FOR DATABASE_MIRRORING (
        ROLE = ALL,
        AUTHENTICATION = CERTIFICATE AG_Cert,
        ENCRYPTION = REQUIRED ALGORITHM AES
    );
GO
```

### Create Database on Primary

Run on PRIMARY (pod 0):

```sql
-- Create database
CREATE DATABASE AppDB;
GO

ALTER DATABASE AppDB SET RECOVERY FULL;
GO

-- Take backup (required for AG)
BACKUP DATABASE AppDB TO DISK = '/var/opt/mssql/backup/AppDB.bak';
GO
```

### Create Availability Group

Run on PRIMARY (pod 0):

```sql
CREATE AVAILABILITY GROUP ProductionAG
    WITH (
        CLUSTER_TYPE = EXTERNAL,
        DB_FAILOVER = ON,
        REQUIRED_SYNCHRONIZED_SECONDARIES_TO_COMMIT = 1
    )
    FOR DATABASE AppDB
    REPLICA ON
        N'sql-ag-prod01-0' WITH (
            ENDPOINT_URL = N'TCP://sql-ag-prod01-0.sql-ag-prod01-pods.mssql.svc.cluster.local:5022',
            AVAILABILITY_MODE = SYNCHRONOUS_COMMIT,
            FAILOVER_MODE = EXTERNAL,
            SEEDING_MODE = AUTOMATIC,
            SECONDARY_ROLE (ALLOW_CONNECTIONS = READ_ONLY)
        ),
        N'sql-ag-prod01-1' WITH (
            ENDPOINT_URL = N'TCP://sql-ag-prod01-1.sql-ag-prod01-pods.mssql.svc.cluster.local:5022',
            AVAILABILITY_MODE = SYNCHRONOUS_COMMIT,
            FAILOVER_MODE = EXTERNAL,
            SEEDING_MODE = AUTOMATIC,
            SECONDARY_ROLE (ALLOW_CONNECTIONS = READ_ONLY)
        ),
        N'sql-ag-prod01-2' WITH (
            ENDPOINT_URL = N'TCP://sql-ag-prod01-2.sql-ag-prod01-pods.mssql.svc.cluster.local:5022',
            AVAILABILITY_MODE = SYNCHRONOUS_COMMIT,
            FAILOVER_MODE = EXTERNAL,
            SEEDING_MODE = AUTOMATIC,
            SECONDARY_ROLE (ALLOW_CONNECTIONS = READ_ONLY)
        );
GO
```

## Step 4: Join Secondary Replicas

Run on EACH secondary (pods 1 and 2):

```bash
# Pod 1
kubectl exec -it sql-ag-prod01-1 -n mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P 'YourStrong@Passw0rd!' -C \
  -Q "ALTER AVAILABILITY GROUP ProductionAG JOIN WITH (CLUSTER_TYPE = EXTERNAL); ALTER AVAILABILITY GROUP ProductionAG GRANT CREATE ANY DATABASE;"

# Pod 2
kubectl exec -it sql-ag-prod01-2 -n mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P 'YourStrong@Passw0rd!' -C \
  -Q "ALTER AVAILABILITY GROUP ProductionAG JOIN WITH (CLUSTER_TYPE = EXTERNAL); ALTER AVAILABILITY GROUP ProductionAG GRANT CREATE ANY DATABASE;"
```

## Step 5: Verify AG Status

### Check AG Helper

```bash
kubectl exec -it sql-ag-prod01-0 -n mssql -c ag-helper -- \
  curl -s localhost:8080/state | jq

# Expected output:
# {
#   "agName": "ProductionAG",
#   "role": "PRIMARY",
#   "health": "Healthy",
#   ...
# }
```

### Check Replica States

```bash
kubectl exec -it sql-ag-prod01-0 -n mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P 'YourStrong@Passw0rd!' -C \
  -Q "SELECT replica_server_name, role_desc, synchronization_health_desc FROM sys.dm_hadr_availability_replica_states"
```

### Check Database Sync

```bash
kubectl exec -it sql-ag-prod01-0 -n mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P 'YourStrong@Passw0rd!' -C \
  -Q "SELECT d.name, drs.synchronization_state_desc FROM sys.dm_hadr_database_replica_states drs JOIN sys.databases d ON drs.database_id = d.database_id"
```

### Check Pod Readiness

```bash
kubectl get pods -n mssql

# All pods should show 2/2 Ready
# NAME               READY   STATUS    RESTARTS   AGE
# sql-ag-prod01-0    2/2     Running   0          15m
# sql-ag-prod01-1    2/2     Running   0          14m
# sql-ag-prod01-2    2/2     Running   0          13m
```

### Test Primary Service

```bash
# Port forward primary service
kubectl port-forward svc/prod-ag-01-primary -n mssql 1433:1433

# Connect with sqlcmd or SSMS
sqlcmd -S localhost,1433 -U sa -P 'YourStrong@Passw0rd!' -Q "SELECT @@SERVERNAME"
```

## Complete Example

Full sample files are available at:
- [samples/sqlserver-availability-group.yaml](../../samples/sqlserver-availability-group.yaml)
- [samples/scripts/setup-availability-group.sql](../../samples/scripts/setup-availability-group.sql)
- [samples/scripts/join-secondary.sql](../../samples/scripts/join-secondary.sql)

## Next Steps

- [Failover Management](failover-management.md) - Configure automatic failover
- [Multi-AG Scenarios](multi-ag-scenarios.md) - Multiple AGs
- [Troubleshooting](../user-guide/troubleshooting.md) - Common issues
