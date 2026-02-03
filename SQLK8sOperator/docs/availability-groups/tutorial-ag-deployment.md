# Tutorial: Deploy a SQL Server Availability Group on Kubernetes

[← Back to Availability Groups](overview.md) | [Documentation Home](../README.md)

This tutorial walks you through deploying a complete SQL Server Availability Group on Kubernetes from scratch. By the end, you'll have a 3-node AG with automatic health monitoring and a listener for client connections.

## What you'll learn

- ✅ Install the SQL Server Kubernetes Operator
- ✅ Deploy 3 SQL Server replicas configured for high availability
- ✅ Create an Availability Group with T-SQL
- ✅ Configure Kubernetes to monitor and manage the AG
- ✅ Set up an AG Listener for seamless client connections
- ✅ Verify the complete setup

## Prerequisites

| Requirement | Check Command |
|-------------|---------------|
| Kubernetes cluster (1.28+) | `kubectl version` |
| Cluster admin access | `kubectl auth can-i create crd` |
| Storage provisioner | `kubectl get storageclass` |
| 3+ nodes recommended | `kubectl get nodes` |

**Resource requirements:** Each SQL Server replica needs at least 2 CPU and 4GB RAM (4 CPU / 8GB recommended for production).

---

## Step 1: Install the Operator

Install the SQL Server Kubernetes Operator with a single command:

```bash
kubectl apply -f https://raw.githubusercontent.com/tejasaks/PublicDemos/main/SQLK8sOperator/install.yaml
```

**Expected output:**

```
namespace/mssql-system created
namespace/mssql created
serviceaccount/mssql-operator created
clusterrole.rbac.authorization.k8s.io/mssql-operator-role created
clusterrolebinding.rbac.authorization.k8s.io/mssql-operator-rolebinding created
customresourcedefinition.apiextensions.k8s.io/sqlservers.mssql.microsoft.com created
customresourcedefinition.apiextensions.k8s.io/sqlserverags.mssql.microsoft.com created
deployment.apps/mssql-operator created
```

Verify the operator is running:

```bash
kubectl get pods -n mssql-system
```

**Expected output:**

```
NAME                              READY   STATUS    RESTARTS   AGE
mssql-operator-xxxxxxxxx-xxxxx   1/1     Running   0          30s
```

> **Note:** For production installations, Helm-based deployments, or private registries, see [Getting Started](../getting-started.md).

---

## Step 2: Deploy SQL Server Replicas

Deploy 3 SQL Server replicas configured for Availability Groups.

### 2.1: Apply the Replicas Manifest

```bash
kubectl apply -f https://raw.githubusercontent.com/tejasaks/PublicDemos/main/SQLK8sOperator/samples/ag-step1-replicas.yaml
```

This creates:
- `mssql` namespace
- SQL Server with 3 replicas (hadrEnabled=true)
- SA password secret
- AG Helper credentials secret

### 2.2: Wait for Pods to be Ready

```bash
# Watch pods until all 3 show 1/1 Ready
kubectl get pods -n mssql -w
```

**Expected output (after 2-5 minutes):**

```
NAME       READY   STATUS    RESTARTS   AGE
sql-ag-0   1/1     Running   0          3m
sql-ag-1   1/1     Running   0          2m
sql-ag-2   1/1     Running   0          1m
```

> **Tip:** Press `Ctrl+C` to stop watching once all pods are Ready.

### 2.3: Verify Individual Replica Services

Each replica has its own service for direct access:

```bash
kubectl get svc -n mssql -l "mssql.microsoft.com/instance=sql-ag"
```

**Expected output:**

```
NAME       TYPE           CLUSTER-IP      EXTERNAL-IP     PORT(S)          AGE
sql-ag     LoadBalancer   10.96.100.10    <pending>       1433:31433/TCP   3m
sql-ag-0   LoadBalancer   10.96.100.11    <pending>       1433:31434/TCP   3m
sql-ag-1   LoadBalancer   10.96.100.12    <pending>       1433:31435/TCP   3m
sql-ag-2   LoadBalancer   10.96.100.13    <pending>       1433:31436/TCP   3m
```

> **Note:** If using minikube or kind, EXTERNAL-IP will show `<pending>`. Use `kubectl port-forward` or `minikube tunnel` to access.

---

## Step 3: Configure the Availability Group (T-SQL)

With all 3 replicas running, configure the Availability Group using T-SQL. This step creates certificates, endpoints, and the AG itself inside SQL Server.

> **Complete T-SQL Reference:** For detailed explanations of each command, see [ag-step2-setup-ag.md](../../samples/ag-step2-setup-ag.md).

### 3.1: Create AG Helper Login (All Replicas)

The AG Helper needs a SQL login to monitor AG health:

```bash
for i in 0 1 2; do
  echo "=== Creating AG Helper login on sql-ag-$i ==="
  kubectl exec -it sql-ag-$i -n mssql -c mssql -- \
    /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa \
    -P 'YourStrong@Passw0rd!' -C -Q "
CREATE LOGIN ag_helper WITH PASSWORD = 'AGHelper@Passw0rd!';
GRANT VIEW SERVER STATE TO ag_helper;
GRANT ALTER ANY AVAILABILITY GROUP TO ag_helper;
PRINT 'AG Helper login created on replica $i';
"
done
```

### 3.2: Create Certificates (All Replicas)

Each replica needs a certificate for endpoint authentication:

```bash
for i in 0 1 2; do
  echo "=== Creating certificate on sql-ag-$i ==="
  kubectl exec -it sql-ag-$i -n mssql -c mssql -- \
    /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa \
    -P 'YourStrong@Passw0rd!' -C -Q "
CREATE MASTER KEY ENCRYPTION BY PASSWORD = 'MasterKey@Passw0rd!';
CREATE CERTIFICATE AG_Cert_$i
    WITH SUBJECT = 'AG Certificate for sql-ag-$i',
    EXPIRY_DATE = '2030-12-31';
BACKUP CERTIFICATE AG_Cert_$i 
    TO FILE = '/var/opt/mssql/data/AG_Cert_$i.cer';
PRINT 'Certificate created and backed up on sql-ag-$i';
"
done
```

### 3.3: Exchange Certificates

Copy certificates between replicas so they can authenticate each other:

```bash
# Download certificates to local machine
mkdir -p /tmp/ag-certs && cd /tmp/ag-certs
for i in 0 1 2; do
  kubectl cp mssql/sql-ag-$i:/var/opt/mssql/data/AG_Cert_$i.cer ./AG_Cert_$i.cer
done

# Upload each certificate to OTHER replicas
kubectl cp ./AG_Cert_1.cer mssql/sql-ag-0:/var/opt/mssql/data/AG_Cert_1.cer
kubectl cp ./AG_Cert_2.cer mssql/sql-ag-0:/var/opt/mssql/data/AG_Cert_2.cer

kubectl cp ./AG_Cert_0.cer mssql/sql-ag-1:/var/opt/mssql/data/AG_Cert_0.cer
kubectl cp ./AG_Cert_2.cer mssql/sql-ag-1:/var/opt/mssql/data/AG_Cert_2.cer

kubectl cp ./AG_Cert_0.cer mssql/sql-ag-2:/var/opt/mssql/data/AG_Cert_0.cer
kubectl cp ./AG_Cert_1.cer mssql/sql-ag-2:/var/opt/mssql/data/AG_Cert_1.cer

echo "Certificate exchange complete"
rm -rf /tmp/ag-certs
```

### 3.4: Import Certificates and Create Endpoints

Import the other replicas' certificates and create database mirroring endpoints.

**On sql-ag-0:**

```bash
kubectl exec -it sql-ag-0 -n mssql -c mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa \
  -P 'YourStrong@Passw0rd!' -C -Q "
-- Create logins and import certificates for other replicas
CREATE LOGIN sql_ag_1_login WITH PASSWORD = 'ReplicaLogin@Passw0rd!';
CREATE USER sql_ag_1_user FOR LOGIN sql_ag_1_login;
CREATE CERTIFICATE AG_Cert_1 AUTHORIZATION sql_ag_1_user
    FROM FILE = '/var/opt/mssql/data/AG_Cert_1.cer';

CREATE LOGIN sql_ag_2_login WITH PASSWORD = 'ReplicaLogin@Passw0rd!';
CREATE USER sql_ag_2_user FOR LOGIN sql_ag_2_login;
CREATE CERTIFICATE AG_Cert_2 AUTHORIZATION sql_ag_2_user
    FROM FILE = '/var/opt/mssql/data/AG_Cert_2.cer';

-- Create the mirroring endpoint FIRST
CREATE ENDPOINT Hadr_endpoint
    STATE = STARTED
    AS TCP (LISTENER_PORT = 5022)
    FOR DATABASE_MIRRORING (
        ROLE = ALL,
        AUTHENTICATION = CERTIFICATE AG_Cert_0,
        ENCRYPTION = REQUIRED ALGORITHM AES
    );

-- Grant connect permissions AFTER endpoint exists
GRANT CONNECT ON ENDPOINT::Hadr_endpoint TO sql_ag_1_login;
GRANT CONNECT ON ENDPOINT::Hadr_endpoint TO sql_ag_2_login;

PRINT 'Endpoint created on sql-ag-0';
"
```

**On sql-ag-1:**

```bash
kubectl exec -it sql-ag-1 -n mssql -c mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa \
  -P 'YourStrong@Passw0rd!' -C -Q "
CREATE LOGIN sql_ag_0_login WITH PASSWORD = 'ReplicaLogin@Passw0rd!';
CREATE USER sql_ag_0_user FOR LOGIN sql_ag_0_login;
CREATE CERTIFICATE AG_Cert_0 AUTHORIZATION sql_ag_0_user
    FROM FILE = '/var/opt/mssql/data/AG_Cert_0.cer';

CREATE LOGIN sql_ag_2_login WITH PASSWORD = 'ReplicaLogin@Passw0rd!';
CREATE USER sql_ag_2_user FOR LOGIN sql_ag_2_login;
CREATE CERTIFICATE AG_Cert_2 AUTHORIZATION sql_ag_2_user
    FROM FILE = '/var/opt/mssql/data/AG_Cert_2.cer';

-- Create the mirroring endpoint FIRST
CREATE ENDPOINT Hadr_endpoint
    STATE = STARTED
    AS TCP (LISTENER_PORT = 5022)
    FOR DATABASE_MIRRORING (
        ROLE = ALL,
        AUTHENTICATION = CERTIFICATE AG_Cert_1,
        ENCRYPTION = REQUIRED ALGORITHM AES
    );

-- Grant connect permissions AFTER endpoint exists
GRANT CONNECT ON ENDPOINT::Hadr_endpoint TO sql_ag_0_login;
GRANT CONNECT ON ENDPOINT::Hadr_endpoint TO sql_ag_2_login;

PRINT 'Endpoint created on sql-ag-1';
"
```

**On sql-ag-2:**

```bash
kubectl exec -it sql-ag-2 -n mssql -c mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa \
  -P 'YourStrong@Passw0rd!' -C -Q "
CREATE LOGIN sql_ag_0_login WITH PASSWORD = 'ReplicaLogin@Passw0rd!';
CREATE USER sql_ag_0_user FOR LOGIN sql_ag_0_login;
CREATE CERTIFICATE AG_Cert_0 AUTHORIZATION sql_ag_0_user
    FROM FILE = '/var/opt/mssql/data/AG_Cert_0.cer';

CREATE LOGIN sql_ag_1_login WITH PASSWORD = 'ReplicaLogin@Passw0rd!';
CREATE USER sql_ag_1_user FOR LOGIN sql_ag_1_login;
CREATE CERTIFICATE AG_Cert_1 AUTHORIZATION sql_ag_1_user
    FROM FILE = '/var/opt/mssql/data/AG_Cert_1.cer';

-- Create the mirroring endpoint FIRST
CREATE ENDPOINT Hadr_endpoint
    STATE = STARTED
    AS TCP (LISTENER_PORT = 5022)
    FOR DATABASE_MIRRORING (
        ROLE = ALL,
        AUTHENTICATION = CERTIFICATE AG_Cert_2,
        ENCRYPTION = REQUIRED ALGORITHM AES
    );

-- Grant connect permissions AFTER endpoint exists
GRANT CONNECT ON ENDPOINT::Hadr_endpoint TO sql_ag_0_login;
GRANT CONNECT ON ENDPOINT::Hadr_endpoint TO sql_ag_1_login;

PRINT 'Endpoint created on sql-ag-2';
"
```

### 3.5: Create Availability Group (Primary Only)

Create the AG on the primary replica (sql-ag-0):

```bash
kubectl exec -it sql-ag-0 -n mssql -c mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa \
  -P 'YourStrong@Passw0rd!' -C -Q "
-- Create the Availability Group
CREATE AVAILABILITY GROUP [ProductionAG]
WITH (
    CLUSTER_TYPE = EXTERNAL,
    DB_FAILOVER = ON
)
FOR REPLICA ON
    N'sql-ag-0' WITH (
        ENDPOINT_URL = N'tcp://sql-ag-0.mssql.svc.cluster.local:5022',
        AVAILABILITY_MODE = SYNCHRONOUS_COMMIT,
        FAILOVER_MODE = EXTERNAL,
        SEEDING_MODE = AUTOMATIC,
        SECONDARY_ROLE (ALLOW_CONNECTIONS = READ_ONLY)
    ),
    N'sql-ag-1' WITH (
        ENDPOINT_URL = N'tcp://sql-ag-1.mssql.svc.cluster.local:5022',
        AVAILABILITY_MODE = SYNCHRONOUS_COMMIT,
        FAILOVER_MODE = EXTERNAL,
        SEEDING_MODE = AUTOMATIC,
        SECONDARY_ROLE (ALLOW_CONNECTIONS = READ_ONLY)
    ),
    N'sql-ag-2' WITH (
        ENDPOINT_URL = N'tcp://sql-ag-2.mssql.svc.cluster.local:5022',
        AVAILABILITY_MODE = SYNCHRONOUS_COMMIT,
        FAILOVER_MODE = EXTERNAL,
        SEEDING_MODE = AUTOMATIC,
        SECONDARY_ROLE (ALLOW_CONNECTIONS = READ_ONLY)
    );

ALTER AVAILABILITY GROUP [ProductionAG] GRANT CREATE ANY DATABASE;
PRINT 'Availability Group ProductionAG created on sql-ag-0';
"
```

### 3.6: Join Secondary Replicas

Join sql-ag-1 and sql-ag-2 to the AG:

```bash
# Join sql-ag-1
kubectl exec -it sql-ag-1 -n mssql -c mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa \
  -P 'YourStrong@Passw0rd!' -C -Q "
ALTER AVAILABILITY GROUP [ProductionAG] JOIN WITH (CLUSTER_TYPE = EXTERNAL);
ALTER AVAILABILITY GROUP [ProductionAG] GRANT CREATE ANY DATABASE;
PRINT 'sql-ag-1 joined ProductionAG';
"

# Join sql-ag-2
kubectl exec -it sql-ag-2 -n mssql -c mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa \
  -P 'YourStrong@Passw0rd!' -C -Q "
ALTER AVAILABILITY GROUP [ProductionAG] JOIN WITH (CLUSTER_TYPE = EXTERNAL);
ALTER AVAILABILITY GROUP [ProductionAG] GRANT CREATE ANY DATABASE;
PRINT 'sql-ag-2 joined ProductionAG';
"
```

### 3.7: Verify the AG is Healthy

```bash
kubectl exec -it sql-ag-0 -n mssql -c mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa \
  -P 'YourStrong@Passw0rd!' -C -Q "
SELECT 
    ar.replica_server_name,
    ars.role_desc,
    ars.synchronization_health_desc,
    ars.connected_state_desc
FROM sys.availability_replicas ar
JOIN sys.dm_hadr_availability_replica_states ars 
    ON ar.replica_id = ars.replica_id
WHERE ar.group_id = (SELECT group_id FROM sys.availability_groups WHERE name = 'ProductionAG');
"
```

**Expected output:**

```
replica_server_name  role_desc  synchronization_health_desc  connected_state_desc
sql-ag-0             PRIMARY    HEALTHY                      CONNECTED
sql-ag-1             SECONDARY  HEALTHY                      CONNECTED
sql-ag-2             SECONDARY  HEALTHY                      CONNECTED
```

### 3.8: Create a Database and Add to AG (Primary Only)

Create a database on the primary and add it to the Availability Group:

```bash
kubectl exec -it sql-ag-0 -n mssql -c mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa \
  -P 'YourStrong@Passw0rd!' -C -Q "
-- Create application database
CREATE DATABASE SampleDB;
GO

-- Set to FULL recovery model (required for AG)
ALTER DATABASE SampleDB SET RECOVERY FULL;
GO

-- Take initial full backup (required before adding to AG)
BACKUP DATABASE SampleDB 
    TO DISK = '/var/opt/mssql/backup/SampleDB_init.bak'
    WITH INIT, COMPRESSION;
GO

-- Add database to the Availability Group
ALTER AVAILABILITY GROUP [ProductionAG] ADD DATABASE SampleDB;
GO

PRINT 'SampleDB created and added to ProductionAG';
"
```

Verify the database is synchronized across all replicas:

```bash
kubectl exec -it sql-ag-0 -n mssql -c mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa \
  -P 'YourStrong@Passw0rd!' -C -Q "
SELECT 
    d.name AS database_name,
    drs.replica_id,
    ar.replica_server_name,
    drs.synchronization_state_desc,
    drs.synchronization_health_desc
FROM sys.dm_hadr_database_replica_states drs
JOIN sys.databases d ON drs.database_id = d.database_id
JOIN sys.availability_replicas ar ON drs.replica_id = ar.replica_id
WHERE d.name = 'SampleDB';
"
```

**Expected output:**

```
database_name  replica_server_name  synchronization_state_desc  synchronization_health_desc
SampleDB       sql-ag-0             SYNCHRONIZED                HEALTHY
SampleDB       sql-ag-1             SYNCHRONIZED                HEALTHY
SampleDB       sql-ag-2             SYNCHRONIZED                HEALTHY
```

---

## Step 4: Deploy the SQLServerAG Resource

Now that the AG exists in SQL Server, deploy the SQLServerAG resource for Kubernetes management.

```bash
kubectl apply -f https://raw.githubusercontent.com/tejasaks/PublicDemos/main/SQLK8sOperator/samples/ag-step3-ag-config.yaml
```

This creates:
- SQLServerAG resource (monitors the AG)
- AG Helper pod (queries SQL Server for health)
- Listener Service (VIP for client connections)

Verify the AG Helper pod is running:

```bash
kubectl get pods -n mssql -l "mssql.microsoft.com/ag=production-ag"
```

**Expected output:**

```
NAME                          READY   STATUS    RESTARTS   AGE
production-ag-helper-xxxxx    1/1     Running   0          30s
```

---

## Step 5: Configure the AG Listener in SQL Server

The operator creates a Kubernetes Service for the listener, but you need to configure the listener inside SQL Server to match.

### 5.1: Get the Listener VIP

```bash
# Get the listener VIP assigned by Kubernetes
VIP=$(kubectl get svc productionag-listener -n mssql -o jsonpath='{.spec.clusterIP}')
echo "Listener VIP: $VIP"
```

### 5.2: Determine the Subnet Mask

The subnet mask depends on your cluster's service CIDR:

```bash
# Method 1: Check existing services to infer the CIDR
kubectl get svc -A -o jsonpath='{.items[*].spec.clusterIP}' | tr ' ' '\n' | sort -u | head -5
```

**Common CIDR to Subnet Mask mapping:**

| CIDR | Subnet Mask |
|------|-------------|
| /12 (e.g., 10.96.0.0/12) | 255.240.0.0 |
| /16 (e.g., 10.0.0.0/16) | 255.255.0.0 |
| /24 (e.g., 10.96.0.0/24) | 255.255.255.0 |

> **Note:** Most Kubernetes clusters use /12 (255.240.0.0) or /16 (255.255.0.0) for service CIDRs.

### 5.3: Create the AG Listener (T-SQL)

Replace `<VIP>` and `<SUBNET_MASK>` with your values:

```bash
kubectl exec -it sql-ag-0 -n mssql -c mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa \
  -P 'YourStrong@Passw0rd!' -C -Q "
ALTER AVAILABILITY GROUP [ProductionAG]
ADD LISTENER 'productionag-listener' (
    WITH IP (('<VIP>', '<SUBNET_MASK>')),
    PORT = 1433
);
PRINT 'AG Listener created with VIP: <VIP>';
"
```

**Example with actual values:**

```bash
kubectl exec -it sql-ag-0 -n mssql -c mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa \
  -P 'YourStrong@Passw0rd!' -C -Q "
ALTER AVAILABILITY GROUP [ProductionAG]
ADD LISTENER 'productionag-listener' (
    WITH IP (('10.96.100.50', '255.240.0.0')),
    PORT = 1433
);
"
```

---

## Step 6: Verify the Complete Setup

### 6.1: Check SQLServerAG Status

```bash
kubectl get sqlserverag -n mssql
```

**Expected output:**

```
NAME            PRIMARY    SYNCED   HEALTH    LISTENER   VIP            AGE
production-ag   sql-ag-0   3        Healthy   Ready      10.96.100.50   5m
```

### 6.2: View Detailed Status

```bash
kubectl describe sqlserverag production-ag -n mssql
```

Look for:
- `Status.Primary Replica: sql-ag-0`
- `Status.Synchronized Replicas: 3`
- `Status.Listener.Phase: Ready`
- `Status.Listener.VIP: 10.96.100.50`

### 6.3: Verify Listener Endpoints

The operator automatically manages Endpoints to route to the primary:

```bash
kubectl get endpoints productionag-listener -n mssql
```

**Expected output:**

```
NAME                     ENDPOINTS           AGE
productionag-listener    10.244.0.5:1433     5m
```

### 6.4: Test Connection via Listener

```bash
# Connect through the listener (always routes to primary)
kubectl exec -it sql-ag-0 -n mssql -c mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S productionag-listener -U sa \
  -P 'YourStrong@Passw0rd!' -C -Q "
SELECT @@SERVERNAME AS 'Connected To', 
       CASE WHEN EXISTS(SELECT 1 FROM sys.dm_hadr_availability_replica_states WHERE is_local = 1 AND role_desc = 'PRIMARY')
            THEN 'PRIMARY' ELSE 'SECONDARY' END AS 'Role';
"
```

**Expected output:**

```
Connected To    Role
sql-ag-0        PRIMARY
```

---

## Cleanup

To remove everything created in this tutorial:

```bash
# Delete the SQLServerAG resource
kubectl delete sqlserverag production-ag -n mssql

# Delete the SQL Server replicas
kubectl delete sqlserver sql-ag -n mssql

# Delete secrets
kubectl delete secret sql-ag-sa sql-ag-helper -n mssql

# Delete the namespace (removes all remaining resources)
kubectl delete namespace mssql

# Optionally, uninstall the operator
kubectl delete -f https://raw.githubusercontent.com/tejasaks/PublicDemos/main/SQLK8sOperator/install.yaml
```

---

## Next Steps

Now that you have a working AG, explore these topics:

| Topic | Link |
|-------|------|
| Manual failover with kubectl | [Failover Management](failover-management.md) |
| Automatic failover configuration | [Deployment Guide](deployment-guide.md#failover-modes) |
| Listener options (LoadBalancer, NodePort) | [Listener Configuration](listener-configuration.md) |
| Multiple AGs on one cluster | [Multi-AG Scenarios](multi-ag-scenarios.md) |
| Day-2 operations | [AG Operations](../operations/ag-operations.md) |

---

## Troubleshooting

### Pods stuck in Pending

```bash
kubectl describe pod sql-ag-0 -n mssql
```

Check for:
- Insufficient resources (CPU/memory)
- PVC not binding (storage issues)

### AG Helper can't connect

```bash
kubectl logs -l "mssql.microsoft.com/ag=production-ag" -n mssql
```

Check that the `ag_helper` login exists and has correct permissions.

### Listener stuck in WaitingForListener

The listener T-SQL hasn't been created yet. Run Step 5.3 to create it.

### Listener shows Degraded

No primary replica is available. Check the AG health:

```bash
kubectl exec -it sql-ag-0 -n mssql -c mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa \
  -P 'YourStrong@Passw0rd!' -C -Q "
SELECT * FROM sys.dm_hadr_availability_replica_states;
"
```

For more troubleshooting, see [Troubleshooting Guide](../user-guide/troubleshooting.md).
