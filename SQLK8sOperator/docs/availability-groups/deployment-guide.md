# AG Deployment Guide

[← Back to Availability Groups](overview.md) | [Documentation Home](../README.md)

Step-by-step guide to deploying a SQL Server Availability Group on Kubernetes.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Step 1: Deploy SQLServer Resource](#step-1-deploy-sqlserver-resource)
- [Step 2: Deploy SQLServerAG Resource](#step-2-deploy-sqlserverag-resource)
- [Step 2.5: Create AG Helper Credentials](#step-25-create-ag-helper-credentials)
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

### AG Helper Authentication

The AG Helper sidecar that monitors AG health uses a **dedicated least-privilege SQL login** (recommended) to connect to SQL Server. This follows the [Pacemaker pattern](https://learn.microsoft.com/en-us/sql/linux/sql-server-linux-availability-group-cluster-pacemaker) from Microsoft's SQL Server Linux documentation.

**Required steps (see Step 2.5 below for details):**
1. Create the `ag_helper` SQL login on ALL replicas
2. Grant `VIEW SERVER STATE` and `ALTER ANY AVAILABILITY GROUP` permissions
3. Create a Kubernetes secret with the AG Helper credentials
4. Reference the secret in the SQLServerAG manifest

See [AG Helper Reference](ag-helper-reference.md#authentication) for complete details.

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
  version: "2025"
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

### Minimal vs Full Configuration

The SQLServerAG CRD has sensible defaults for most fields. You can choose between:

| Approach | When to Use |
|----------|-------------|
| **Minimal** | Quick setup, accepting all defaults |
| **Full** | Production deployments needing explicit control |

**Minimal required fields:**
- `sqlServerRef.name` - Reference to the SQLServer resource
- `availabilityGroup.name` - AG name as it appears in SQL Server
- `availabilityGroup.primaryConfig` - Can be empty `{}` for defaults
- `availabilityGroup.secondaryConfig` - Can be empty `{}` for defaults

**Defaults applied automatically:**

| Field | Default Value |
|-------|---------------|
| `replicas` | 3 |
| `automaticFailover` | true |
| `seedingMode` | Automatic |
| `dbFailover` | true |
| `clusterType` | External |
| `endpointPort` | 5022 |
| `primaryConfig.availabilityMode` | SynchronousCommit |
| `primaryConfig.failoverMode` | External |
| `primaryConfig.readableSecondary` | ReadOnly |
| `sidecar.image` | mssql-ag-helper:latest |
| `sidecar.monitorInterval` | 10s |

See [samples/sqlserverag-minimal.yaml](../../samples/sqlserverag-minimal.yaml) for a minimal example.

> **Tip:** The sample manifests include Secret definitions for dev/test convenience. For production, see [Step 2.5](#step-25-create-ag-helper-credentials) for the recommended approach of pre-creating secrets.

### Option A: Minimal SQLServerAG Manifest

```bash
cat > sqlserverag-minimal.yaml << 'EOF'
apiVersion: mssql.microsoft.com/v1alpha1
kind: SQLServerAG
metadata:
  name: prod-ag-01
  namespace: mssql
spec:
  sqlServerRef:
    name: sql-ag-prod01
  availabilityGroup:
    name: ProductionAG
    primaryConfig: {}     # Use all defaults
    secondaryConfig: {}   # Use all defaults
EOF
```

### Option B: Full SQLServerAG Manifest (Recommended for Production)

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

## Step 2.5: Create AG Helper Credentials

Before creating the Availability Group, set up the dedicated health check login for the AG Helper sidecar.

### Credential Workflow Options

Choose the appropriate workflow based on your environment:

| Workflow | Best For | Description |
|----------|----------|-------------|
| **Option A: Dev/Test** | Quick testing, demos | Use sample manifests as-is (secrets included inline) |
| **Option B: Production** | Secure deployments | Pre-create secrets, then deploy manifests without secret sections |

#### Option A: Dev/Test (Secrets Included in Manifests)

The sample manifests (`samples/sqlserver-availability-group.yaml`, `samples/sqlserverag-minimal.yaml`) include Secret definitions for convenience. Simply apply the manifest and both the AG resources and secrets are created together:

```bash
kubectl apply -f samples/sqlserver-availability-group.yaml
```

> **Note:** This is convenient for testing but not recommended for production since credentials are visible in the manifest.

#### Option B: Production (Pre-Create Secrets)

For production deployments:

1. **Pre-create secrets** using `kubectl`, external-secrets operator, HashiCorp Vault, or your organization's secret management solution
2. **Modify the sample manifests** to remove or comment out the `SECRETS SECTION`
3. **Ensure** `healthCheckCredentials.secretRef` points to your pre-created secret names

If you use Option B, skip to [Create Kubernetes Secret](#create-kubernetes-secret) below, then proceed to Step 3.

### Why Not Use SA?

The AG Helper only needs to:
- Read AG state from DMVs (`VIEW SERVER STATE`)
- Perform failover operations (`ALTER ANY AVAILABILITY GROUP`)

Using SA grants unrestricted access, violating the principle of least privilege. A compromised AG Helper with SA credentials could drop databases, create logins, etc.

### Create AG Helper Login on ALL Replicas

Run this T-SQL on **each** replica (pod 0, 1, and 2):

```bash
# Connect to pod 0
kubectl exec -it sql-ag-prod01-0 -n mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P 'YourStrong@Passw0rd!' -C
```

```sql
-- Create the AG Helper login
CREATE LOGIN ag_helper WITH PASSWORD = 'AGHelper@Passw0rd!';
GO

-- Grant required permissions
GRANT VIEW SERVER STATE TO ag_helper;
GRANT ALTER ANY AVAILABILITY GROUP TO ag_helper;
GO
```

Repeat for pods 1 and 2:
```bash
kubectl exec -it sql-ag-prod01-1 -n mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P 'YourStrong@Passw0rd!' -C -Q "
    CREATE LOGIN ag_helper WITH PASSWORD = 'AGHelper@Passw0rd!';
    GRANT VIEW SERVER STATE TO ag_helper;
    GRANT ALTER ANY AVAILABILITY GROUP TO ag_helper;
  "

kubectl exec -it sql-ag-prod01-2 -n mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P 'YourStrong@Passw0rd!' -C -Q "
    CREATE LOGIN ag_helper WITH PASSWORD = 'AGHelper@Passw0rd!';
    GRANT VIEW SERVER STATE TO ag_helper;
    GRANT ALTER ANY AVAILABILITY GROUP TO ag_helper;
  "
```

### Create Kubernetes Secret

```bash
kubectl create secret generic sql-ag-prod-aghelper \
  --namespace mssql \
  --from-literal=username=ag_helper \
  --from-literal=password='AGHelper@Passw0rd!'
```

### Update SQLServerAG Manifest

Add `healthCheckCredentials` to your SQLServerAG manifest:

```yaml
spec:
  availabilityGroup:
    name: ProductionAG
    # ... other fields ...
    healthCheckCredentials:
      secretRef:
        usernameSecret:
          name: sql-ag-prod-aghelper
          key: username
        passwordSecret:
          name: sql-ag-prod-aghelper
          key: password
```

See [samples/ag-helper-credentials-secret.yaml](../../samples/ag-helper-credentials-secret.yaml) for a complete example.

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
