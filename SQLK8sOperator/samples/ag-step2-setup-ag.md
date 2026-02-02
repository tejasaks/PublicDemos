# Step 2: T-SQL Setup for Availability Group

[← Back to Samples](../README.md) | [Step 1: Deploy Replicas](ag-step1-replicas.yaml) | [Step 3: AG Config](ag-step3-ag-config.yaml)

This guide walks through the T-SQL commands to create an Availability Group after deploying SQL Server replicas.

## Prerequisites

Before proceeding, ensure:

- [ ] `ag-step1-replicas.yaml` has been applied
- [ ] All 3 pods are Running and Ready (`kubectl get pods -n mssql`)
- [ ] AG Helper credentials secret exists (created in Step 1)

```bash
# Verify pods are ready
kubectl get pods -n mssql
# Expected: sql-ag-0, sql-ag-1, sql-ag-2 all showing 1/1 Ready

# Verify AG Helper credentials secret exists
kubectl get secret sql-ag-helper -n mssql
# Expected: secret/sql-ag-helper (will be used by AG Helper in Step 3)
```

> **Note:** AG Helper is NOT running yet. It will be deployed in Step 3 when you create the SQLServerAG resource. The new architecture deploys one AG Helper pod per Availability Group (not per replica).

---

## Overview

| Step | Description | Run On |
|------|-------------|--------|
| 2.1 | Create AG Helper login | ALL replicas |
| 2.2 | Create master key and certificates | ALL replicas |
| 2.3 | Export/import certificates | Primary → Secondaries |
| 2.4 | Create database mirroring endpoints | ALL replicas |
| 2.5 | Create databases | Primary only |
| 2.6 | Create Availability Group | Primary only |
| 2.7 | Join secondary replicas | Secondaries only |
| 2.8 | Verify AG status | Any replica |

---

## Step 2.1: Create AG Helper Login

Run on **ALL replicas** (sql-ag-0, sql-ag-1, sql-ag-2).

The AG Helper (deployed in Step 3) will need a SQL login to monitor AG health and perform failover operations. Create the login now so it's ready when the AG Helper connects.

```bash
# Connect to each replica and run the T-SQL
for i in 0 1 2; do
  echo "=== Creating AG Helper login on sql-ag-$i ==="
  kubectl exec -it sql-ag-$i -n mssql -c mssql -- \
    /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa \
    -P 'YourStrong@Passw0rd!' -C -Q "
-- Create AG Helper login (must match secret in ag-step1-replicas.yaml)
CREATE LOGIN ag_helper WITH PASSWORD = 'AGHelper@Passw0rd!';
GO

-- Grant required permissions for AG health monitoring
GRANT VIEW SERVER STATE TO ag_helper;
GRANT ALTER ANY AVAILABILITY GROUP TO ag_helper;
GO

PRINT 'AG Helper login created successfully on replica $i';
GO
"
done
```

---

## Step 2.2: Create Master Key and Certificates

Run on **ALL replicas** (sql-ag-0, sql-ag-1, sql-ag-2).

Each replica needs its own certificate for database mirroring authentication.

```bash
# Create master key and certificate on each replica
for i in 0 1 2; do
  echo "=== Creating certificate on sql-ag-$i ==="
  kubectl exec -it sql-ag-$i -n mssql -c mssql -- \
    /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa \
    -P 'YourStrong@Passw0rd!' -C -Q "
-- Create master key
CREATE MASTER KEY ENCRYPTION BY PASSWORD = 'MasterKey@Passw0rd!';
GO

-- Create certificate for this replica
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

## Step 2.3: Export and Import Certificates

Certificates must be exchanged between replicas for mutual authentication.

### 2.3.1: Backup Certificates on Each Replica

```bash
# Backup certificate on each replica
for i in 0 1 2; do
  echo "=== Backing up certificate on sql-ag-$i ==="
  kubectl exec -it sql-ag-$i -n mssql -c mssql -- \
    /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa \
    -P 'YourStrong@Passw0rd!' -C -Q "
BACKUP CERTIFICATE AG_Cert_$i 
    TO FILE = '/var/opt/mssql/data/AG_Cert_$i.cer';
GO
PRINT 'Certificate backed up to /var/opt/mssql/data/AG_Cert_$i.cer';
GO
"
done
```

### 2.3.2: Copy Certificates Between Replicas

Use `kubectl cp` to exchange certificate files between pods.

```bash
# Create a temp directory for certificates
mkdir -p /tmp/ag-certs
cd /tmp/ag-certs

# Copy each certificate from its source replica to local machine
echo "=== Downloading certificates from pods ==="
kubectl cp mssql/sql-ag-0:/var/opt/mssql/data/AG_Cert_0.cer ./AG_Cert_0.cer
kubectl cp mssql/sql-ag-1:/var/opt/mssql/data/AG_Cert_1.cer ./AG_Cert_1.cer
kubectl cp mssql/sql-ag-2:/var/opt/mssql/data/AG_Cert_2.cer ./AG_Cert_2.cer

# Verify files downloaded
ls -la AG_Cert_*.cer

# Copy certificates to other replicas
# Each replica needs the OTHER replicas' certificates

echo "=== Uploading certificates to sql-ag-0 ==="
kubectl cp ./AG_Cert_1.cer mssql/sql-ag-0:/var/opt/mssql/data/AG_Cert_1.cer
kubectl cp ./AG_Cert_2.cer mssql/sql-ag-0:/var/opt/mssql/data/AG_Cert_2.cer

echo "=== Uploading certificates to sql-ag-1 ==="
kubectl cp ./AG_Cert_0.cer mssql/sql-ag-1:/var/opt/mssql/data/AG_Cert_0.cer
kubectl cp ./AG_Cert_2.cer mssql/sql-ag-1:/var/opt/mssql/data/AG_Cert_2.cer

echo "=== Uploading certificates to sql-ag-2 ==="
kubectl cp ./AG_Cert_0.cer mssql/sql-ag-2:/var/opt/mssql/data/AG_Cert_0.cer
kubectl cp ./AG_Cert_1.cer mssql/sql-ag-2:/var/opt/mssql/data/AG_Cert_1.cer

echo "=== Certificate exchange complete ==="

# Cleanup
rm -rf /tmp/ag-certs
```

### 2.3.3: Create Logins and Import Certificates

Each replica needs logins for the OTHER replicas, associated with their certificates.

```bash
# On sql-ag-0: Create logins for sql-ag-1 and sql-ag-2
echo "=== Importing certificates on sql-ag-0 ==="
kubectl exec -it sql-ag-0 -n mssql -c mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa \
  -P 'YourStrong@Passw0rd!' -C -Q "
-- Login for sql-ag-1
CREATE LOGIN sql_ag_1_login WITH PASSWORD = 'ReplicaLogin@Passw0rd!';
CREATE USER sql_ag_1_user FOR LOGIN sql_ag_1_login;
CREATE CERTIFICATE AG_Cert_1 
    AUTHORIZATION sql_ag_1_user
    FROM FILE = '/var/opt/mssql/data/AG_Cert_1.cer';
GO

-- Login for sql-ag-2  
CREATE LOGIN sql_ag_2_login WITH PASSWORD = 'ReplicaLogin@Passw0rd!';
CREATE USER sql_ag_2_user FOR LOGIN sql_ag_2_login;
CREATE CERTIFICATE AG_Cert_2
    AUTHORIZATION sql_ag_2_user
    FROM FILE = '/var/opt/mssql/data/AG_Cert_2.cer';
GO

PRINT 'Certificates imported on sql-ag-0';
GO
"

# On sql-ag-1: Create logins for sql-ag-0 and sql-ag-2
echo "=== Importing certificates on sql-ag-1 ==="
kubectl exec -it sql-ag-1 -n mssql -c mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa \
  -P 'YourStrong@Passw0rd!' -C -Q "
-- Login for sql-ag-0
CREATE LOGIN sql_ag_0_login WITH PASSWORD = 'ReplicaLogin@Passw0rd!';
CREATE USER sql_ag_0_user FOR LOGIN sql_ag_0_login;
CREATE CERTIFICATE AG_Cert_0
    AUTHORIZATION sql_ag_0_user
    FROM FILE = '/var/opt/mssql/data/AG_Cert_0.cer';
GO

-- Login for sql-ag-2
CREATE LOGIN sql_ag_2_login WITH PASSWORD = 'ReplicaLogin@Passw0rd!';
CREATE USER sql_ag_2_user FOR LOGIN sql_ag_2_login;
CREATE CERTIFICATE AG_Cert_2
    AUTHORIZATION sql_ag_2_user
    FROM FILE = '/var/opt/mssql/data/AG_Cert_2.cer';
GO

PRINT 'Certificates imported on sql-ag-1';
GO
"

# On sql-ag-2: Create logins for sql-ag-0 and sql-ag-1
echo "=== Importing certificates on sql-ag-2 ==="
kubectl exec -it sql-ag-2 -n mssql -c mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa \
  -P 'YourStrong@Passw0rd!' -C -Q "
-- Login for sql-ag-0
CREATE LOGIN sql_ag_0_login WITH PASSWORD = 'ReplicaLogin@Passw0rd!';
CREATE USER sql_ag_0_user FOR LOGIN sql_ag_0_login;
CREATE CERTIFICATE AG_Cert_0
    AUTHORIZATION sql_ag_0_user
    FROM FILE = '/var/opt/mssql/data/AG_Cert_0.cer';
GO

-- Login for sql-ag-1
CREATE LOGIN sql_ag_1_login WITH PASSWORD = 'ReplicaLogin@Passw0rd!';
CREATE USER sql_ag_1_user FOR LOGIN sql_ag_1_login;
CREATE CERTIFICATE AG_Cert_1
    AUTHORIZATION sql_ag_1_user
    FROM FILE = '/var/opt/mssql/data/AG_Cert_1.cer';
GO

PRINT 'Certificates imported on sql-ag-2';
GO
"
```

---

## Step 2.4: Create Database Mirroring Endpoints

Run on **ALL replicas** (sql-ag-0, sql-ag-1, sql-ag-2).

Each replica needs an endpoint, and we grant connect permission only to the logins that exist on that replica (i.e., logins for the *other* replicas).

```bash
# Create endpoint on sql-ag-0 (grant to logins 1 and 2)
echo "=== Creating endpoint on sql-ag-0 ==="
kubectl exec -it sql-ag-0 -n mssql -c mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa \
  -P 'YourStrong@Passw0rd!' -C -Q "
CREATE ENDPOINT AG_Endpoint
    STATE = STARTED
    AS TCP (
        LISTENER_PORT = 5022,
        LISTENER_IP = ALL
    )
    FOR DATABASE_MIRRORING (
        ROLE = ALL,
        AUTHENTICATION = CERTIFICATE AG_Cert_0,
        ENCRYPTION = REQUIRED ALGORITHM AES
    );
GO

-- Grant connect to the OTHER replica logins (1 and 2 exist on this node)
GRANT CONNECT ON ENDPOINT::AG_Endpoint TO sql_ag_1_login;
GRANT CONNECT ON ENDPOINT::AG_Endpoint TO sql_ag_2_login;
GO

PRINT 'Endpoint created on sql-ag-0';
GO
"

# Create endpoint on sql-ag-1 (grant to logins 0 and 2)
echo "=== Creating endpoint on sql-ag-1 ==="
kubectl exec -it sql-ag-1 -n mssql -c mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa \
  -P 'YourStrong@Passw0rd!' -C -Q "
CREATE ENDPOINT AG_Endpoint
    STATE = STARTED
    AS TCP (
        LISTENER_PORT = 5022,
        LISTENER_IP = ALL
    )
    FOR DATABASE_MIRRORING (
        ROLE = ALL,
        AUTHENTICATION = CERTIFICATE AG_Cert_1,
        ENCRYPTION = REQUIRED ALGORITHM AES
    );
GO

-- Grant connect to the OTHER replica logins (0 and 2 exist on this node)
GRANT CONNECT ON ENDPOINT::AG_Endpoint TO sql_ag_0_login;
GRANT CONNECT ON ENDPOINT::AG_Endpoint TO sql_ag_2_login;
GO

PRINT 'Endpoint created on sql-ag-1';
GO
"

# Create endpoint on sql-ag-2 (grant to logins 0 and 1)
echo "=== Creating endpoint on sql-ag-2 ==="
kubectl exec -it sql-ag-2 -n mssql -c mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa \
  -P 'YourStrong@Passw0rd!' -C -Q "
CREATE ENDPOINT AG_Endpoint
    STATE = STARTED
    AS TCP (
        LISTENER_PORT = 5022,
        LISTENER_IP = ALL
    )
    FOR DATABASE_MIRRORING (
        ROLE = ALL,
        AUTHENTICATION = CERTIFICATE AG_Cert_2,
        ENCRYPTION = REQUIRED ALGORITHM AES
    );
GO

-- Grant connect to the OTHER replica logins (0 and 1 exist on this node)
GRANT CONNECT ON ENDPOINT::AG_Endpoint TO sql_ag_0_login;
GRANT CONNECT ON ENDPOINT::AG_Endpoint TO sql_ag_1_login;
GO

PRINT 'Endpoint created on sql-ag-2';
GO
"
```

> **Note:** The endpoint port (5022) is configured here in T-SQL. You can change this if needed, but ensure all replicas use the same port.

---

## Step 2.5: Create Databases

Run on **PRIMARY only** (sql-ag-0).

```bash
echo "=== Creating databases on primary (sql-ag-0) ==="
kubectl exec -it sql-ag-0 -n mssql -c mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa \
  -P 'YourStrong@Passw0rd!' -C -Q "
-- Create application database
CREATE DATABASE ApplicationDB;
GO

-- Set to FULL recovery model (required for AG)
ALTER DATABASE ApplicationDB SET RECOVERY FULL;
GO

-- Take initial full backup (required before adding to AG with automatic seeding)
BACKUP DATABASE ApplicationDB 
    TO DISK = '/var/opt/mssql/backup/ApplicationDB_init.bak'
    WITH INIT, COMPRESSION;
GO

PRINT 'ApplicationDB created and backed up';
GO
"
```

---

## Step 2.6: Create Availability Group

Run on **PRIMARY only** (sql-ag-0).

### Basic AG (3 Synchronous Replicas)

```bash
echo "=== Creating Availability Group on primary ==="
kubectl exec -it sql-ag-0 -n mssql -c mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa \
  -P 'YourStrong@Passw0rd!' -C -Q "
CREATE AVAILABILITY GROUP ProductionAG
    WITH (
        CLUSTER_TYPE = EXTERNAL,
        DB_FAILOVER = ON,
        REQUIRED_SYNCHRONIZED_SECONDARIES_TO_COMMIT = 1
    )
    FOR DATABASE ApplicationDB
    REPLICA ON
        N'sql-ag-0' WITH (
            ENDPOINT_URL = N'TCP://sql-ag-0.sql-ag-headless.mssql.svc.cluster.local:5022',
            AVAILABILITY_MODE = SYNCHRONOUS_COMMIT,
            FAILOVER_MODE = EXTERNAL,
            SEEDING_MODE = AUTOMATIC,
            SESSION_TIMEOUT = 10,
            SECONDARY_ROLE (ALLOW_CONNECTIONS = READ_ONLY)
        ),
        N'sql-ag-1' WITH (
            ENDPOINT_URL = N'TCP://sql-ag-1.sql-ag-headless.mssql.svc.cluster.local:5022',
            AVAILABILITY_MODE = SYNCHRONOUS_COMMIT,
            FAILOVER_MODE = EXTERNAL,
            SEEDING_MODE = AUTOMATIC,
            SESSION_TIMEOUT = 10,
            SECONDARY_ROLE (ALLOW_CONNECTIONS = READ_ONLY)
        ),
        N'sql-ag-2' WITH (
            ENDPOINT_URL = N'TCP://sql-ag-2.sql-ag-headless.mssql.svc.cluster.local:5022',
            AVAILABILITY_MODE = SYNCHRONOUS_COMMIT,
            FAILOVER_MODE = EXTERNAL,
            SEEDING_MODE = AUTOMATIC,
            SESSION_TIMEOUT = 10,
            SECONDARY_ROLE (ALLOW_CONNECTIONS = READ_ONLY)
        );
GO

PRINT 'Availability Group ProductionAG created';
GO
"
```

### Advanced AG Configurations

<details>
<summary><b>Mixed Sync/Async (2 Sync + 1 Async for Geo-DR)</b></summary>

```sql
CREATE AVAILABILITY GROUP ProductionAG
    WITH (
        CLUSTER_TYPE = EXTERNAL,
        DB_FAILOVER = ON,
        REQUIRED_SYNCHRONIZED_SECONDARIES_TO_COMMIT = 1
    )
    FOR DATABASE ApplicationDB
    REPLICA ON
        N'sql-ag-0' WITH (
            ENDPOINT_URL = N'TCP://sql-ag-0.sql-ag-headless.mssql.svc.cluster.local:5022',
            AVAILABILITY_MODE = SYNCHRONOUS_COMMIT,
            FAILOVER_MODE = EXTERNAL,
            SEEDING_MODE = AUTOMATIC,
            SESSION_TIMEOUT = 10,
            SECONDARY_ROLE (ALLOW_CONNECTIONS = READ_ONLY)
        ),
        N'sql-ag-1' WITH (
            ENDPOINT_URL = N'TCP://sql-ag-1.sql-ag-headless.mssql.svc.cluster.local:5022',
            AVAILABILITY_MODE = SYNCHRONOUS_COMMIT,
            FAILOVER_MODE = EXTERNAL,
            SEEDING_MODE = AUTOMATIC,
            SESSION_TIMEOUT = 10,
            SECONDARY_ROLE (ALLOW_CONNECTIONS = READ_ONLY)
        ),
        -- Geo-DR replica: Async for low latency on primary
        N'sql-ag-2' WITH (
            ENDPOINT_URL = N'TCP://sql-ag-2.sql-ag-headless.mssql.svc.cluster.local:5022',
            AVAILABILITY_MODE = ASYNCHRONOUS_COMMIT,
            FAILOVER_MODE = MANUAL,  -- Must be MANUAL for async
            SEEDING_MODE = AUTOMATIC,
            SESSION_TIMEOUT = 30,    -- Higher timeout for geo-replication
            SECONDARY_ROLE (ALLOW_CONNECTIONS = READ_ONLY)
        );
GO
```

</details>

<details>
<summary><b>Manual Seeding (Backup/Restore)</b></summary>

Use when automatic seeding is too slow or databases are large.

```sql
-- On PRIMARY: Create AG without databases first
CREATE AVAILABILITY GROUP ProductionAG
    WITH (CLUSTER_TYPE = EXTERNAL, DB_FAILOVER = ON)
    REPLICA ON
        N'sql-ag-0' WITH (
            ENDPOINT_URL = N'TCP://sql-ag-0.sql-ag-headless.mssql.svc.cluster.local:5022',
            AVAILABILITY_MODE = SYNCHRONOUS_COMMIT,
            FAILOVER_MODE = EXTERNAL,
            SEEDING_MODE = MANUAL,  -- Manual seeding
            SECONDARY_ROLE (ALLOW_CONNECTIONS = READ_ONLY)
        ),
        N'sql-ag-1' WITH (
            ENDPOINT_URL = N'TCP://sql-ag-1.sql-ag-headless.mssql.svc.cluster.local:5022',
            AVAILABILITY_MODE = SYNCHRONOUS_COMMIT,
            FAILOVER_MODE = EXTERNAL,
            SEEDING_MODE = MANUAL,
            SECONDARY_ROLE (ALLOW_CONNECTIONS = READ_ONLY)
        ),
        N'sql-ag-2' WITH (
            ENDPOINT_URL = N'TCP://sql-ag-2.sql-ag-headless.mssql.svc.cluster.local:5022',
            AVAILABILITY_MODE = SYNCHRONOUS_COMMIT,
            FAILOVER_MODE = EXTERNAL,
            SEEDING_MODE = MANUAL,
            SECONDARY_ROLE (ALLOW_CONNECTIONS = READ_ONLY)
        );
GO

-- Take full and log backup on primary
BACKUP DATABASE ApplicationDB TO DISK = '/var/opt/mssql/backup/ApplicationDB_full.bak' WITH INIT;
BACKUP LOG ApplicationDB TO DISK = '/var/opt/mssql/backup/ApplicationDB_log.trn' WITH INIT;
GO

-- Copy backups to secondaries using kubectl cp (see Step 2.3.2 pattern)
-- Then restore WITH NORECOVERY on each secondary

-- On SECONDARIES: Restore database
RESTORE DATABASE ApplicationDB FROM DISK = '/var/opt/mssql/backup/ApplicationDB_full.bak' 
    WITH NORECOVERY;
RESTORE LOG ApplicationDB FROM DISK = '/var/opt/mssql/backup/ApplicationDB_log.trn' 
    WITH NORECOVERY;
GO

-- Then add database to AG on primary
ALTER AVAILABILITY GROUP ProductionAG ADD DATABASE ApplicationDB;
GO

-- Join database on secondaries
ALTER DATABASE ApplicationDB SET HADR AVAILABILITY GROUP = ProductionAG;
GO
```

</details>

<details>
<summary><b>Reporting-Only Async Replica</b></summary>

```sql
CREATE AVAILABILITY GROUP ReportingAG
    WITH (
        CLUSTER_TYPE = EXTERNAL,
        DB_FAILOVER = OFF  -- No automatic database failover
    )
    FOR DATABASE ReportingDB
    REPLICA ON
        N'sql-ag-0' WITH (
            ENDPOINT_URL = N'TCP://sql-ag-0.sql-ag-headless.mssql.svc.cluster.local:5022',
            AVAILABILITY_MODE = SYNCHRONOUS_COMMIT,
            FAILOVER_MODE = EXTERNAL,
            SEEDING_MODE = AUTOMATIC,
            PRIMARY_ROLE (ALLOW_CONNECTIONS = ALL),
            SECONDARY_ROLE (ALLOW_CONNECTIONS = NO)  -- Primary only
        ),
        -- Read-only reporting replica
        N'sql-ag-1' WITH (
            ENDPOINT_URL = N'TCP://sql-ag-1.sql-ag-headless.mssql.svc.cluster.local:5022',
            AVAILABILITY_MODE = ASYNCHRONOUS_COMMIT,
            FAILOVER_MODE = MANUAL,
            SEEDING_MODE = AUTOMATIC,
            SESSION_TIMEOUT = 60,  -- Longer timeout acceptable for async
            SECONDARY_ROLE (ALLOW_CONNECTIONS = READ_ONLY)
        );
GO
```

</details>

<details>
<summary><b>Different Session Timeouts per Replica</b></summary>

```sql
-- Fast failover for local replicas, tolerant for geo-replicas
CREATE AVAILABILITY GROUP ProductionAG
    WITH (CLUSTER_TYPE = EXTERNAL, DB_FAILOVER = ON)
    FOR DATABASE ApplicationDB
    REPLICA ON
        N'sql-ag-0' WITH (
            ENDPOINT_URL = N'TCP://sql-ag-0.sql-ag-headless.mssql.svc.cluster.local:5022',
            AVAILABILITY_MODE = SYNCHRONOUS_COMMIT,
            FAILOVER_MODE = EXTERNAL,
            SEEDING_MODE = AUTOMATIC,
            SESSION_TIMEOUT = 10,  -- 10 seconds for local
            SECONDARY_ROLE (ALLOW_CONNECTIONS = READ_ONLY)
        ),
        N'sql-ag-1' WITH (
            ENDPOINT_URL = N'TCP://sql-ag-1.sql-ag-headless.mssql.svc.cluster.local:5022',
            AVAILABILITY_MODE = SYNCHRONOUS_COMMIT,
            FAILOVER_MODE = EXTERNAL,
            SEEDING_MODE = AUTOMATIC,
            SESSION_TIMEOUT = 10,  -- 10 seconds for local
            SECONDARY_ROLE (ALLOW_CONNECTIONS = READ_ONLY)
        ),
        N'sql-ag-2' WITH (
            ENDPOINT_URL = N'TCP://sql-ag-2.sql-ag-headless.mssql.svc.cluster.local:5022',
            AVAILABILITY_MODE = ASYNCHRONOUS_COMMIT,
            FAILOVER_MODE = MANUAL,
            SEEDING_MODE = AUTOMATIC,
            SESSION_TIMEOUT = 60,  -- 60 seconds for geo-replica
            SECONDARY_ROLE (ALLOW_CONNECTIONS = READ_ONLY)
        );
GO
```

</details>

---

## Step 2.7: Join Secondary Replicas

Run on **SECONDARIES only** (sql-ag-1, sql-ag-2).

```bash
# Join secondaries to the AG
for i in 1 2; do
  echo "=== Joining sql-ag-$i to Availability Group ==="
  kubectl exec -it sql-ag-$i -n mssql -c mssql -- \
    /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa \
    -P 'YourStrong@Passw0rd!' -C -Q "
-- Join this replica to the AG
ALTER AVAILABILITY GROUP ProductionAG JOIN WITH (CLUSTER_TYPE = EXTERNAL);
GO

-- Grant automatic seeding permission
ALTER AVAILABILITY GROUP ProductionAG GRANT CREATE ANY DATABASE;
GO

PRINT 'sql-ag-$i joined to ProductionAG';
GO
"
done
```

---

## Step 2.8: Verify AG Status

Run on **any replica**.

```bash
echo "=== Verifying AG status ==="
kubectl exec -it sql-ag-0 -n mssql -c mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa \
  -P 'YourStrong@Passw0rd!' -C -Q "
-- Check replica status
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

-- Check database status
SELECT 
    ag.name AS ag_name,
    d.name AS database_name,
    drs.synchronization_state_desc,
    drs.is_primary_replica,
    drs.synchronization_health_desc
FROM sys.dm_hadr_database_replica_states drs
JOIN sys.databases d ON drs.database_id = d.database_id
JOIN sys.availability_groups ag ON drs.group_id = ag.group_id
ORDER BY d.name;
GO
"
```

**Expected output:**
- All replicas show `CONNECTED` and `HEALTHY`
- Primary shows `PRIMARY`, secondaries show `SECONDARY`
- Databases show `SYNCHRONIZED` (for sync replicas)

---

## What's Next: Deploy AG Helper

At this point, the T-SQL Availability Group is configured and running, but there is **no AG Helper monitoring it yet**. The AG Helper will be deployed in Step 3 when you create the SQLServerAG resource.

> **Architecture Note:** The operator uses a **single AG Helper pod per Availability Group** (not per replica). This centralized approach:
> - Provides a single source of truth for AG health
> - Eliminates coordination conflicts between multiple helpers
> - Simplifies credential management
> - Reduces resource overhead

---

## Next Steps

After completing T-SQL setup, proceed to Step 3:

**Apply [ag-step3-ag-config.yaml](ag-step3-ag-config.yaml)** to:

- ✅ **Deploy AG Helper pod** - A single pod that monitors AG health across all replicas
- ✅ **Create Primary/Secondary Services** - Route traffic to current primary or secondaries
- ✅ **Enable optional automatic failover** - Set `automaticFailover: true` for controller-managed failover

The SQLServerAG resource triggers deployment of the AG Helper, which will:
1. Connect to each replica using the credentials from `sql-ag-helper` secret
2. Monitor the Availability Group health status
3. Update the SQLServerAG status with current primary and sync state
4. Perform automatic failover if enabled and primary becomes unhealthy

---

## Troubleshooting

### Verify AG Helper Login Works (Before Step 3)

Test that the AG Helper login you created can connect:

```bash
# Verify AG Helper login works on all replicas
for i in 0 1 2; do
  echo "=== Testing AG Helper login on sql-ag-$i ==="
  kubectl exec -it sql-ag-$i -n mssql -c mssql -- \
    /opt/mssql-tools18/bin/sqlcmd -S localhost -U ag_helper \
    -P 'AGHelper@Passw0rd!' -C -Q "SELECT @@SERVERNAME AS ServerName, SUSER_NAME() AS LoginName"
done
```

### Secondaries not joining

```bash
# Check endpoint connectivity
kubectl exec -it sql-ag-1 -n mssql -c mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa \
  -P 'YourStrong@Passw0rd!' -C -Q "
SELECT * FROM sys.tcp_endpoints;
SELECT * FROM sys.dm_hadr_availability_replica_states;
GO
"
```

### Certificate errors

```bash
# Verify certificates exist
kubectl exec -it sql-ag-0 -n mssql -c mssql -- ls -la /var/opt/mssql/data/AG_Cert*.cer

# Check certificate in SQL
kubectl exec -it sql-ag-0 -n mssql -c mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa \
  -P 'YourStrong@Passw0rd!' -C -Q "SELECT name, thumbprint FROM sys.certificates"
```
