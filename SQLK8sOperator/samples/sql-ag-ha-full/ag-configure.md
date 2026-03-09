# T-SQL Configuration for Availability Group (Full HA Scenario)

[← Back to README](README.md)

This guide walks through the T-SQL commands to create the `ag1` Availability Group
with two databases (`appdb`, `auditdb`) and a listener after applying `ag-deploy.yaml`.
An automated shell script version is also available: `ag-configure.sh`.

## Prerequisites

- [ ] `ag-deploy.yaml` has been applied
- [ ] All 3 pods are Running and Ready (`kubectl get pods -n mssql`)
- [ ] AG health-check credentials secret exists (`kubectl get secret ag-health-default -n mssql`)

```bash
# Verify pods are ready
kubectl get pods -n mssql
# Expected: sql-ag-full-0, sql-ag-full-1, sql-ag-full-2 all showing Running

# Verify health-check credentials secret exists
kubectl get secret ag-health-default -n mssql
```

> **Note:** The SQLServerAG resource is already deployed (in `ag-deploy.yaml`). The AG controller
> will be in a retry loop showing phase="Creating" until the T-SQL steps below are completed.
> This is normal — everything converges automatically once the AG exists.

---

## Overview

| Step | Description | Run On |
|------|-------------|--------|
| 1 | Create AG health-check login | ALL replicas |
| 2 | Create master key and certificates | ALL replicas |
| 3 | Export/import certificates | Primary → Secondaries |
| 4 | Create database mirroring endpoints | ALL replicas |
| 5 | Create databases (appdb, auditdb) | Primary only |
| 6 | Create Availability Group | Primary only |
| 7 | Join secondary replicas | Secondaries only |
| 8 | Verify AG status | Any replica |
| 9 | Create AG Listener (optional) | Primary only |

---

## Step 1: Create AG Health-Check Login

Run on **ALL replicas** (sql-ag-full-0, sql-ag-full-1, sql-ag-full-2).

```bash
for i in 0 1 2; do
  echo "=== Creating AG health-check login on sql-ag-full-$i ==="
  kubectl exec -it sql-ag-full-$i -n mssql -c mssql -- \
    /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa \
    -P 'YourStrong@Passw0rd!' -C -Q "
CREATE LOGIN ag_health WITH PASSWORD = 'H3althProbe!Pwd';
GO
GRANT VIEW SERVER STATE TO ag_health;
GRANT ALTER ANY AVAILABILITY GROUP TO ag_health;
GO
PRINT 'AG health-check login created on replica $i';
GO
"
done
```

---

## Step 2: Create Master Key and Certificates

Run on **ALL replicas** (sql-ag-full-0, sql-ag-full-1, sql-ag-full-2).

```bash
for i in 0 1 2; do
  echo "=== Creating certificate on sql-ag-full-$i ==="
  kubectl exec -it sql-ag-full-$i -n mssql -c mssql -- \
    /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa \
    -P 'YourStrong@Passw0rd!' -C -Q "
CREATE MASTER KEY ENCRYPTION BY PASSWORD = 'MasterKey@Passw0rd!';
GO
CREATE CERTIFICATE AG_Cert_$i
    WITH SUBJECT = 'AG Certificate for sql-ag-full-$i',
    EXPIRY_DATE = '2030-12-31';
GO
PRINT 'Certificate created on sql-ag-full-$i';
GO
"
done
```

---

## Step 3: Export and Import Certificates

### 3.1: Backup Certificates

```bash
for i in 0 1 2; do
  echo "=== Backing up certificate on sql-ag-full-$i ==="
  kubectl exec -it sql-ag-full-$i -n mssql -c mssql -- \
    /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa \
    -P 'YourStrong@Passw0rd!' -C -Q "
BACKUP CERTIFICATE AG_Cert_$i
    TO FILE = '/var/opt/mssql/data/AG_Cert_$i.cer';
GO
"
done
```

### 3.2: Exchange Certificate Files

```bash
mkdir -p /tmp/ag-certs && cd /tmp/ag-certs

# Download
kubectl cp mssql/sql-ag-full-0:/var/opt/mssql/data/AG_Cert_0.cer ./AG_Cert_0.cer
kubectl cp mssql/sql-ag-full-1:/var/opt/mssql/data/AG_Cert_1.cer ./AG_Cert_1.cer
kubectl cp mssql/sql-ag-full-2:/var/opt/mssql/data/AG_Cert_2.cer ./AG_Cert_2.cer

# Upload to other replicas
kubectl cp ./AG_Cert_1.cer mssql/sql-ag-full-0:/var/opt/mssql/data/AG_Cert_1.cer
kubectl cp ./AG_Cert_2.cer mssql/sql-ag-full-0:/var/opt/mssql/data/AG_Cert_2.cer

kubectl cp ./AG_Cert_0.cer mssql/sql-ag-full-1:/var/opt/mssql/data/AG_Cert_0.cer
kubectl cp ./AG_Cert_2.cer mssql/sql-ag-full-1:/var/opt/mssql/data/AG_Cert_2.cer

kubectl cp ./AG_Cert_0.cer mssql/sql-ag-full-2:/var/opt/mssql/data/AG_Cert_0.cer
kubectl cp ./AG_Cert_1.cer mssql/sql-ag-full-2:/var/opt/mssql/data/AG_Cert_1.cer

rm -rf /tmp/ag-certs
```

### 3.3: Import Certificates

```bash
# On sql-ag-full-0: import certs from 1 and 2
kubectl exec -it sql-ag-full-0 -n mssql -c mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa \
  -P 'YourStrong@Passw0rd!' -C -Q "
CREATE LOGIN sql_ag_1_login WITH PASSWORD = 'ReplicaLogin@Passw0rd!';
CREATE USER sql_ag_1_user FOR LOGIN sql_ag_1_login;
CREATE CERTIFICATE AG_Cert_1
    AUTHORIZATION sql_ag_1_user
    FROM FILE = '/var/opt/mssql/data/AG_Cert_1.cer';
GO
CREATE LOGIN sql_ag_2_login WITH PASSWORD = 'ReplicaLogin@Passw0rd!';
CREATE USER sql_ag_2_user FOR LOGIN sql_ag_2_login;
CREATE CERTIFICATE AG_Cert_2
    AUTHORIZATION sql_ag_2_user
    FROM FILE = '/var/opt/mssql/data/AG_Cert_2.cer';
GO
"

# On sql-ag-full-1: import certs from 0 and 2
kubectl exec -it sql-ag-full-1 -n mssql -c mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa \
  -P 'YourStrong@Passw0rd!' -C -Q "
CREATE LOGIN sql_ag_0_login WITH PASSWORD = 'ReplicaLogin@Passw0rd!';
CREATE USER sql_ag_0_user FOR LOGIN sql_ag_0_login;
CREATE CERTIFICATE AG_Cert_0
    AUTHORIZATION sql_ag_0_user
    FROM FILE = '/var/opt/mssql/data/AG_Cert_0.cer';
GO
CREATE LOGIN sql_ag_2_login WITH PASSWORD = 'ReplicaLogin@Passw0rd!';
CREATE USER sql_ag_2_user FOR LOGIN sql_ag_2_login;
CREATE CERTIFICATE AG_Cert_2
    AUTHORIZATION sql_ag_2_user
    FROM FILE = '/var/opt/mssql/data/AG_Cert_2.cer';
GO
"

# On sql-ag-full-2: import certs from 0 and 1
kubectl exec -it sql-ag-full-2 -n mssql -c mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa \
  -P 'YourStrong@Passw0rd!' -C -Q "
CREATE LOGIN sql_ag_0_login WITH PASSWORD = 'ReplicaLogin@Passw0rd!';
CREATE USER sql_ag_0_user FOR LOGIN sql_ag_0_login;
CREATE CERTIFICATE AG_Cert_0
    AUTHORIZATION sql_ag_0_user
    FROM FILE = '/var/opt/mssql/data/AG_Cert_0.cer';
GO
CREATE LOGIN sql_ag_1_login WITH PASSWORD = 'ReplicaLogin@Passw0rd!';
CREATE USER sql_ag_1_user FOR LOGIN sql_ag_1_login;
CREATE CERTIFICATE AG_Cert_1
    AUTHORIZATION sql_ag_1_user
    FROM FILE = '/var/opt/mssql/data/AG_Cert_1.cer';
GO
"
```

---

## Step 4: Create Database Mirroring Endpoints

Each replica needs an endpoint with CONNECT grants for the OTHER replicas' logins.

```bash
# sql-ag-full-0
kubectl exec -it sql-ag-full-0 -n mssql -c mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa \
  -P 'YourStrong@Passw0rd!' -C -Q "
CREATE ENDPOINT AG_Endpoint
    STATE = STARTED
    AS TCP (LISTENER_PORT = 5022, LISTENER_IP = ALL)
    FOR DATABASE_MIRRORING (
        ROLE = ALL,
        AUTHENTICATION = CERTIFICATE AG_Cert_0,
        ENCRYPTION = REQUIRED ALGORITHM AES
    );
GO
GRANT CONNECT ON ENDPOINT::AG_Endpoint TO sql_ag_1_login;
GRANT CONNECT ON ENDPOINT::AG_Endpoint TO sql_ag_2_login;
GO
"

# sql-ag-full-1
kubectl exec -it sql-ag-full-1 -n mssql -c mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa \
  -P 'YourStrong@Passw0rd!' -C -Q "
CREATE ENDPOINT AG_Endpoint
    STATE = STARTED
    AS TCP (LISTENER_PORT = 5022, LISTENER_IP = ALL)
    FOR DATABASE_MIRRORING (
        ROLE = ALL,
        AUTHENTICATION = CERTIFICATE AG_Cert_1,
        ENCRYPTION = REQUIRED ALGORITHM AES
    );
GO
GRANT CONNECT ON ENDPOINT::AG_Endpoint TO sql_ag_0_login;
GRANT CONNECT ON ENDPOINT::AG_Endpoint TO sql_ag_2_login;
GO
"

# sql-ag-full-2
kubectl exec -it sql-ag-full-2 -n mssql -c mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa \
  -P 'YourStrong@Passw0rd!' -C -Q "
CREATE ENDPOINT AG_Endpoint
    STATE = STARTED
    AS TCP (LISTENER_PORT = 5022, LISTENER_IP = ALL)
    FOR DATABASE_MIRRORING (
        ROLE = ALL,
        AUTHENTICATION = CERTIFICATE AG_Cert_2,
        ENCRYPTION = REQUIRED ALGORITHM AES
    );
GO
GRANT CONNECT ON ENDPOINT::AG_Endpoint TO sql_ag_0_login;
GRANT CONNECT ON ENDPOINT::AG_Endpoint TO sql_ag_1_login;
GO
"
```

---

## Step 5: Create Databases

Run on **PRIMARY only** (sql-ag-full-0). Creates both `appdb` and `auditdb`.

```bash
kubectl exec -it sql-ag-full-0 -n mssql -c mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa \
  -P 'YourStrong@Passw0rd!' -C -Q "
-- Create appdb
CREATE DATABASE appdb;
GO
ALTER DATABASE appdb SET RECOVERY FULL;
GO
BACKUP DATABASE appdb
    TO DISK = '/var/opt/mssql/backup/appdb_init.bak'
    WITH INIT, COMPRESSION;
GO

-- Create auditdb
CREATE DATABASE auditdb;
GO
ALTER DATABASE auditdb SET RECOVERY FULL;
GO
BACKUP DATABASE auditdb
    TO DISK = '/var/opt/mssql/backup/auditdb_init.bak'
    WITH INIT, COMPRESSION;
GO
"
```

---

## Step 6: Create Availability Group

Run on **PRIMARY only** (sql-ag-full-0). All 3 replicas use synchronous commit
with read-only routing on secondaries.

```bash
kubectl exec -it sql-ag-full-0 -n mssql -c mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa \
  -P 'YourStrong@Passw0rd!' -C -Q "
CREATE AVAILABILITY GROUP ag1
    WITH (
        CLUSTER_TYPE = EXTERNAL,
        DB_FAILOVER = ON,
        REQUIRED_SYNCHRONIZED_SECONDARIES_TO_COMMIT = 1
    )
    FOR DATABASE appdb, auditdb
    REPLICA ON
        N'sql-ag-full-0' WITH (
            ENDPOINT_URL = N'TCP://sql-ag-full-0.mssql.svc.cluster.local:5022',
            AVAILABILITY_MODE = SYNCHRONOUS_COMMIT,
            FAILOVER_MODE = EXTERNAL,
            SEEDING_MODE = AUTOMATIC,
            SESSION_TIMEOUT = 10,
            SECONDARY_ROLE (ALLOW_CONNECTIONS = READ_ONLY)
        ),
        N'sql-ag-full-1' WITH (
            ENDPOINT_URL = N'TCP://sql-ag-full-1.mssql.svc.cluster.local:5022',
            AVAILABILITY_MODE = SYNCHRONOUS_COMMIT,
            FAILOVER_MODE = EXTERNAL,
            SEEDING_MODE = AUTOMATIC,
            SESSION_TIMEOUT = 10,
            SECONDARY_ROLE (ALLOW_CONNECTIONS = READ_ONLY)
        ),
        N'sql-ag-full-2' WITH (
            ENDPOINT_URL = N'TCP://sql-ag-full-2.mssql.svc.cluster.local:5022',
            AVAILABILITY_MODE = SYNCHRONOUS_COMMIT,
            FAILOVER_MODE = EXTERNAL,
            SEEDING_MODE = AUTOMATIC,
            SESSION_TIMEOUT = 10,
            SECONDARY_ROLE (ALLOW_CONNECTIONS = READ_ONLY)
        );
GO
"
```

---

## Step 7: Join Secondary Replicas

Run on **SECONDARIES only** (sql-ag-full-1, sql-ag-full-2).

```bash
for i in 1 2; do
  echo "=== Joining sql-ag-full-$i to ag1 ==="
  kubectl exec -it sql-ag-full-$i -n mssql -c mssql -- \
    /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa \
    -P 'YourStrong@Passw0rd!' -C -Q "
ALTER AVAILABILITY GROUP ag1 JOIN WITH (CLUSTER_TYPE = EXTERNAL);
GO
ALTER AVAILABILITY GROUP ag1 GRANT CREATE ANY DATABASE;
GO
"
done
```

> After the secondaries join, the AG controller will automatically detect the AG
> and transition the SQLServerAG phase from "Creating" to "Running".

---

## Step 8: Verify AG Status

```bash
kubectl exec -it sql-ag-full-0 -n mssql -c mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa \
  -P 'YourStrong@Passw0rd!' -C -Q "
SELECT
    ag.name AS ag_name,
    ar.replica_server_name,
    ars.role_desc,
    ars.synchronization_health_desc,
    ars.connected_state_desc
FROM sys.availability_groups ag
JOIN sys.availability_replicas ar ON ag.group_id = ar.group_id
JOIN sys.dm_hadr_availability_replica_states ars ON ar.replica_id = ars.replica_id
ORDER BY ar.replica_server_name;
GO
"
```

Verify both databases are synchronized:

```bash
kubectl exec -it sql-ag-full-0 -n mssql -c mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa \
  -P 'YourStrong@Passw0rd!' -C -Q "
SELECT ag.name AS ag_name, d.name AS database_name,
    drs.synchronization_state_desc, drs.is_primary_replica, drs.synchronization_health_desc
FROM sys.dm_hadr_database_replica_states drs
JOIN sys.databases d ON drs.database_id = d.database_id
JOIN sys.availability_groups ag ON drs.group_id = ag.group_id
ORDER BY d.name, drs.is_primary_replica DESC;
GO
"
```

Also verify through Kubernetes:

```bash
kubectl get sqlserverag -n mssql
kubectl get sqlserver -n mssql
```

**Expected:** All replicas `CONNECTED` + `HEALTHY`, both databases `SYNCHRONIZED`.

---

## Step 9: Create AG Listener (Optional)

The AG Listener provides a single VIP that always routes to the current primary.
The Kubernetes Service was already created by the operator — you just need to register
it in SQL Server.

### 9.1: Get the Listener VIP

```bash
# Get the VIP assigned by Kubernetes
kubectl get sqlserverag ag-full -n mssql -o jsonpath='{.status.listener.vip}'

# Or from the Service directly
kubectl get svc ag-listener -n mssql -o jsonpath='{.spec.clusterIP}'
```

### 9.2: Determine Subnet Mask

```bash
# Method 1: From kube-apiserver
kubectl get pod -n kube-system -l component=kube-apiserver -o yaml | grep service-cluster-ip-range

# Method 2: From kubeadm config
kubectl get cm kubeadm-config -n kube-system -o jsonpath='{.data.ClusterConfiguration}' | grep serviceSubnet

# Common masks: /12 = 255.240.0.0, /16 = 255.255.0.0, /24 = 255.255.255.0
```

### 9.3: Create the Listener in SQL Server

```bash
# Replace <VIP> and <SUBNET_MASK> with values from above
kubectl exec -it sql-ag-full-0 -n mssql -c mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa \
  -P 'YourStrong@Passw0rd!' -C -Q "
ALTER AVAILABILITY GROUP ag1
ADD LISTENER 'ag-listener' (
    WITH IP (('<VIP>', '<SUBNET_MASK>')),
    PORT = 1433
);
GO
"
```

### 9.4: Verify the Listener

```bash
kubectl get sqlserverag ag-full -n mssql -o jsonpath='{.status.listener}'
```

Connect via the listener:

```bash
kubectl run sqlcmd --rm -it --image=mcr.microsoft.com/mssql-tools18 -- \
  sqlcmd -S ag-listener.mssql.svc.cluster.local -U sa -P 'YourStrong@Passw0rd!' -C
```
