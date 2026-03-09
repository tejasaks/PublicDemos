# T-SQL Configuration for Availability Group (Minimal HA Scenario)

[← Back to README](README.md)

This guide walks through the T-SQL commands to create the `ag1` Availability Group
after applying `ag-deploy.yaml`. An automated shell script version is also available: `ag-configure.sh`.

## Prerequisites

- [ ] `ag-deploy.yaml` has been applied
- [ ] All 3 pods are Running and Ready (`kubectl get pods -n mssql`)
- [ ] AG health-check credentials secret exists (`kubectl get secret ag-health-login -n mssql`)

```bash
# Verify pods are ready
kubectl get pods -n mssql
# Expected: sql-ag-0, sql-ag-1, sql-ag-2 all showing Running

# Verify health-check credentials secret exists
kubectl get secret ag-health-login -n mssql
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
| 5 | Create database | Primary only |
| 6 | Create Availability Group | Primary only |
| 7 | Join secondary replicas | Secondaries only |
| 8 | Verify AG status | Any replica |

---

## Step 1: Create AG Health-Check Login

Run on **ALL replicas** (sql-ag-0, sql-ag-1, sql-ag-2).

```bash
for i in 0 1 2; do
  echo "=== Creating AG health-check login on sql-ag-$i ==="
  kubectl exec -it sql-ag-$i -n mssql -c mssql -- \
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

Run on **ALL replicas** (sql-ag-0, sql-ag-1, sql-ag-2).

```bash
for i in 0 1 2; do
  echo "=== Creating certificate on sql-ag-$i ==="
  kubectl exec -it sql-ag-$i -n mssql -c mssql -- \
    /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa \
    -P 'YourStrong@Passw0rd!' -C -Q "
CREATE MASTER KEY ENCRYPTION BY PASSWORD = 'MasterKey@Passw0rd!';
GO
CREATE CERTIFICATE AG_Cert_$i
    WITH SUBJECT = 'AG Certificate for sql-ag-$i',
    EXPIRY_DATE = '2030-12-31';
GO
PRINT 'Certificate created on sql-ag-$i';
GO
"
done
```

---

## Step 3: Export and Import Certificates

### 3.1: Backup Certificates

```bash
for i in 0 1 2; do
  echo "=== Backing up certificate on sql-ag-$i ==="
  kubectl exec -it sql-ag-$i -n mssql -c mssql -- \
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
kubectl cp mssql/sql-ag-0:/var/opt/mssql/data/AG_Cert_0.cer ./AG_Cert_0.cer
kubectl cp mssql/sql-ag-1:/var/opt/mssql/data/AG_Cert_1.cer ./AG_Cert_1.cer
kubectl cp mssql/sql-ag-2:/var/opt/mssql/data/AG_Cert_2.cer ./AG_Cert_2.cer

# Upload to other replicas
kubectl cp ./AG_Cert_1.cer mssql/sql-ag-0:/var/opt/mssql/data/AG_Cert_1.cer
kubectl cp ./AG_Cert_2.cer mssql/sql-ag-0:/var/opt/mssql/data/AG_Cert_2.cer

kubectl cp ./AG_Cert_0.cer mssql/sql-ag-1:/var/opt/mssql/data/AG_Cert_0.cer
kubectl cp ./AG_Cert_2.cer mssql/sql-ag-1:/var/opt/mssql/data/AG_Cert_2.cer

kubectl cp ./AG_Cert_0.cer mssql/sql-ag-2:/var/opt/mssql/data/AG_Cert_0.cer
kubectl cp ./AG_Cert_1.cer mssql/sql-ag-2:/var/opt/mssql/data/AG_Cert_1.cer

rm -rf /tmp/ag-certs
```

### 3.3: Import Certificates

```bash
# On sql-ag-0: import certs from 1 and 2
kubectl exec -it sql-ag-0 -n mssql -c mssql -- \
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

# On sql-ag-1: import certs from 0 and 2
kubectl exec -it sql-ag-1 -n mssql -c mssql -- \
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

# On sql-ag-2: import certs from 0 and 1
kubectl exec -it sql-ag-2 -n mssql -c mssql -- \
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
# sql-ag-0
kubectl exec -it sql-ag-0 -n mssql -c mssql -- \
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

# sql-ag-1
kubectl exec -it sql-ag-1 -n mssql -c mssql -- \
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

# sql-ag-2
kubectl exec -it sql-ag-2 -n mssql -c mssql -- \
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

## Step 5: Create Database

Run on **PRIMARY only** (sql-ag-0).

```bash
kubectl exec -it sql-ag-0 -n mssql -c mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa \
  -P 'YourStrong@Passw0rd!' -C -Q "
CREATE DATABASE SampleDB;
GO
ALTER DATABASE SampleDB SET RECOVERY FULL;
GO
BACKUP DATABASE SampleDB
    TO DISK = '/var/opt/mssql/backup/SampleDB_init.bak'
    WITH INIT, COMPRESSION;
GO
"
```

---

## Step 6: Create Availability Group

Run on **PRIMARY only** (sql-ag-0). All 3 replicas use synchronous commit.

```bash
kubectl exec -it sql-ag-0 -n mssql -c mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa \
  -P 'YourStrong@Passw0rd!' -C -Q "
CREATE AVAILABILITY GROUP ag1
    WITH (
        CLUSTER_TYPE = EXTERNAL,
        DB_FAILOVER = ON,
        REQUIRED_SYNCHRONIZED_SECONDARIES_TO_COMMIT = 1
    )
    FOR DATABASE SampleDB
    REPLICA ON
        N'sql-ag-0' WITH (
            ENDPOINT_URL = N'TCP://sql-ag-0.mssql.svc.cluster.local:5022',
            AVAILABILITY_MODE = SYNCHRONOUS_COMMIT,
            FAILOVER_MODE = EXTERNAL,
            SEEDING_MODE = AUTOMATIC
        ),
        N'sql-ag-1' WITH (
            ENDPOINT_URL = N'TCP://sql-ag-1.mssql.svc.cluster.local:5022',
            AVAILABILITY_MODE = SYNCHRONOUS_COMMIT,
            FAILOVER_MODE = EXTERNAL,
            SEEDING_MODE = AUTOMATIC
        ),
        N'sql-ag-2' WITH (
            ENDPOINT_URL = N'TCP://sql-ag-2.mssql.svc.cluster.local:5022',
            AVAILABILITY_MODE = SYNCHRONOUS_COMMIT,
            FAILOVER_MODE = EXTERNAL,
            SEEDING_MODE = AUTOMATIC
        );
GO
"
```

---

## Step 7: Join Secondary Replicas

Run on **SECONDARIES only** (sql-ag-1, sql-ag-2).

```bash
for i in 1 2; do
  echo "=== Joining sql-ag-$i to ag1 ==="
  kubectl exec -it sql-ag-$i -n mssql -c mssql -- \
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
kubectl exec -it sql-ag-0 -n mssql -c mssql -- \
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

Also verify through Kubernetes:

```bash
kubectl get sqlserverag -n mssql
kubectl get sqlserver -n mssql
```

**Expected:** All replicas `CONNECTED` + `HEALTHY`, database `SYNCHRONIZED`.
