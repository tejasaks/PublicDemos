# Multi-AG T-SQL Configuration Guide

This guide creates **three Availability Groups** on the same SQL Server replicas,
each tailored for a different workload pattern.

| AG | Database | Sync Mode | Failover | Port |
|----|----------|-----------|----------|------|
| ProductionAG | AppDB | 3 synchronous | Automatic | 1433/1434 |
| ReportingAG | ReportingDB | 1 sync + 2 async | Manual | 2433/2434 |
| DisasterRecoveryAG | CriticalDB | 1 sync + 1 async | Manual | 3433/3434 |

> **Automation**: Run `./ag-configure.sh all` to execute all steps below.

---

## Prerequisites

```bash
kubectl apply -f ag-deploy.yaml
kubectl -n mssql wait --for=condition=ready pod/sql-ag-0 pod/sql-ag-1 pod/sql-ag-2 --timeout=300s
```

Get per-replica external IPs:

```bash
SQL0=$(kubectl -n mssql get svc sql-ag-0 -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
SQL1=$(kubectl -n mssql get svc sql-ag-1 -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
SQL2=$(kubectl -n mssql get svc sql-ag-2 -o jsonpath='{.status.loadBalancer.ingress[0].ip}')
SA_PWD="YourStrong@Passw0rd!"
```

---

## Step 1 — Create AG Helper Login (all replicas)

Run on **sql-ag-0**, **sql-ag-1**, and **sql-ag-2**:

```sql
CREATE LOGIN [ag_helper]
  WITH PASSWORD = N'AGHelper@Passw0rd!',
       CHECK_POLICY = OFF,
       CHECK_EXPIRATION = OFF;

ALTER SERVER ROLE [sysadmin] ADD MEMBER [ag_helper];
GO
```

---

## Step 2 — Create Database Master Keys (all replicas)

Run on **all three replicas**:

```sql
USE master;
CREATE MASTER KEY ENCRYPTION BY PASSWORD = N'MasterKey@Passw0rd!';
GO
```

---

## Step 3 — Create Certificates

On **sql-ag-0** (primary):

```sql
USE master;
CREATE CERTIFICATE ag_cert
  WITH SUBJECT = 'AG Endpoint Certificate',
       EXPIRY_DATE = '20301231';

BACKUP CERTIFICATE ag_cert
  TO FILE = '/var/opt/mssql/data/ag_cert.cer'
  WITH PRIVATE KEY (
    FILE = '/var/opt/mssql/data/ag_cert.key',
    ENCRYPTION BY PASSWORD = N'CertKey@Passw0rd!'
  );
GO
```

Copy the certificate files to other pods:

```bash
# Export from sql-ag-0
kubectl -n mssql cp sql-ag-0:/var/opt/mssql/data/ag_cert.cer ./ag_cert.cer
kubectl -n mssql cp sql-ag-0:/var/opt/mssql/data/ag_cert.key ./ag_cert.key

# Import to sql-ag-1 and sql-ag-2
for pod in sql-ag-1 sql-ag-2; do
  kubectl -n mssql cp ./ag_cert.cer ${pod}:/var/opt/mssql/data/ag_cert.cer
  kubectl -n mssql cp ./ag_cert.key ${pod}:/var/opt/mssql/data/ag_cert.key
done
```

On **sql-ag-1** and **sql-ag-2**:

```sql
USE master;
CREATE CERTIFICATE ag_cert
  FROM FILE = '/var/opt/mssql/data/ag_cert.cer'
  WITH PRIVATE KEY (
    FILE = '/var/opt/mssql/data/ag_cert.key',
    DECRYPTION BY PASSWORD = N'CertKey@Passw0rd!'
  );
GO
```

---

## Step 4 — Create AG Endpoints (all replicas)

Run on **all three replicas**:

```sql
CREATE ENDPOINT [ag_endpoint]
  STATE = STARTED
  AS TCP (LISTENER_PORT = 5022)
  FOR DATABASE_MIRRORING (
    ROLE = ALL,
    AUTHENTICATION = CERTIFICATE ag_cert,
    ENCRYPTION = REQUIRED ALGORITHM AES
  );
GO
```

---

## Step 5 — Create Databases (sql-ag-0 only)

Create one database per AG on the **primary replica**:

```sql
-- Database for ProductionAG
CREATE DATABASE [AppDB];
ALTER DATABASE [AppDB] SET RECOVERY FULL;
BACKUP DATABASE [AppDB] TO DISK = '/var/opt/mssql/backup/AppDB.bak';
GO

-- Database for ReportingAG
CREATE DATABASE [ReportingDB];
ALTER DATABASE [ReportingDB] SET RECOVERY FULL;
BACKUP DATABASE [ReportingDB] TO DISK = '/var/opt/mssql/backup/ReportingDB.bak';
GO

-- Database for DisasterRecoveryAG
CREATE DATABASE [CriticalDB];
ALTER DATABASE [CriticalDB] SET RECOVERY FULL;
BACKUP DATABASE [CriticalDB] TO DISK = '/var/opt/mssql/backup/CriticalDB.bak';
GO
```

---

## Step 6 — Create Availability Groups (sql-ag-0 only)

### 6a — ProductionAG (3 sync, auto failover)

```sql
CREATE AVAILABILITY GROUP [ProductionAG]
  WITH (
    CLUSTER_TYPE = EXTERNAL,
    DB_FAILOVER = ON,
    AUTOMATED_BACKUP_PREFERENCE = SECONDARY
  )
  FOR DATABASE [AppDB]
  REPLICA ON
    N'sql-ag-0' WITH (
      ENDPOINT_URL = N'TCP://sql-ag-0.mssql.svc.cluster.local:5022',
      AVAILABILITY_MODE = SYNCHRONOUS_COMMIT,
      FAILOVER_MODE = EXTERNAL,
      SEEDING_MODE = AUTOMATIC,
      SESSION_TIMEOUT = 10,
      SECONDARY_ROLE (ALLOW_CONNECTIONS = READ_ONLY)
    ),
    N'sql-ag-1' WITH (
      ENDPOINT_URL = N'TCP://sql-ag-1.mssql.svc.cluster.local:5022',
      AVAILABILITY_MODE = SYNCHRONOUS_COMMIT,
      FAILOVER_MODE = EXTERNAL,
      SEEDING_MODE = AUTOMATIC,
      SESSION_TIMEOUT = 10,
      SECONDARY_ROLE (ALLOW_CONNECTIONS = READ_ONLY)
    ),
    N'sql-ag-2' WITH (
      ENDPOINT_URL = N'TCP://sql-ag-2.mssql.svc.cluster.local:5022',
      AVAILABILITY_MODE = SYNCHRONOUS_COMMIT,
      FAILOVER_MODE = EXTERNAL,
      SEEDING_MODE = AUTOMATIC,
      SESSION_TIMEOUT = 10,
      SECONDARY_ROLE (ALLOW_CONNECTIONS = READ_ONLY)
    );
GO
```

### 6b — ReportingAG (1 sync + 2 async, read scale-out)

```sql
CREATE AVAILABILITY GROUP [ReportingAG]
  WITH (
    CLUSTER_TYPE = EXTERNAL,
    DB_FAILOVER = OFF,
    AUTOMATED_BACKUP_PREFERENCE = SECONDARY
  )
  FOR DATABASE [ReportingDB]
  REPLICA ON
    N'sql-ag-0' WITH (
      ENDPOINT_URL = N'TCP://sql-ag-0.mssql.svc.cluster.local:5022',
      AVAILABILITY_MODE = SYNCHRONOUS_COMMIT,
      FAILOVER_MODE = EXTERNAL,
      SEEDING_MODE = AUTOMATIC,
      SESSION_TIMEOUT = 10,
      SECONDARY_ROLE (ALLOW_CONNECTIONS = NO)
    ),
    N'sql-ag-1' WITH (
      ENDPOINT_URL = N'TCP://sql-ag-1.mssql.svc.cluster.local:5022',
      AVAILABILITY_MODE = ASYNCHRONOUS_COMMIT,
      FAILOVER_MODE = MANUAL,
      SEEDING_MODE = AUTOMATIC,
      SESSION_TIMEOUT = 30,
      SECONDARY_ROLE (ALLOW_CONNECTIONS = READ_ONLY)
    ),
    N'sql-ag-2' WITH (
      ENDPOINT_URL = N'TCP://sql-ag-2.mssql.svc.cluster.local:5022',
      AVAILABILITY_MODE = ASYNCHRONOUS_COMMIT,
      FAILOVER_MODE = MANUAL,
      SEEDING_MODE = AUTOMATIC,
      SESSION_TIMEOUT = 30,
      SECONDARY_ROLE (ALLOW_CONNECTIONS = READ_ONLY)
    );
GO
```

### 6c — DisasterRecoveryAG (1 sync + 1 async)

> Only sql-ag-0 and sql-ag-1 participate in this AG.

```sql
CREATE AVAILABILITY GROUP [DisasterRecoveryAG]
  WITH (
    CLUSTER_TYPE = EXTERNAL,
    DB_FAILOVER = OFF,
    AUTOMATED_BACKUP_PREFERENCE = PRIMARY
  )
  FOR DATABASE [CriticalDB]
  REPLICA ON
    N'sql-ag-0' WITH (
      ENDPOINT_URL = N'TCP://sql-ag-0.mssql.svc.cluster.local:5022',
      AVAILABILITY_MODE = SYNCHRONOUS_COMMIT,
      FAILOVER_MODE = EXTERNAL,
      SEEDING_MODE = MANUAL,
      SESSION_TIMEOUT = 10,
      SECONDARY_ROLE (ALLOW_CONNECTIONS = NO)
    ),
    N'sql-ag-1' WITH (
      ENDPOINT_URL = N'TCP://sql-ag-1.mssql.svc.cluster.local:5022',
      AVAILABILITY_MODE = ASYNCHRONOUS_COMMIT,
      FAILOVER_MODE = MANUAL,
      SEEDING_MODE = MANUAL,
      SESSION_TIMEOUT = 60,
      SECONDARY_ROLE (ALLOW_CONNECTIONS = NO)
    );
GO
```

---

## Step 7 — Join Secondaries

### 7a — sql-ag-1 (joins all 3 AGs)

```sql
ALTER AVAILABILITY GROUP [ProductionAG] JOIN WITH (CLUSTER_TYPE = EXTERNAL);
ALTER AVAILABILITY GROUP [ProductionAG] GRANT CREATE ANY DATABASE;
GO

ALTER AVAILABILITY GROUP [ReportingAG] JOIN WITH (CLUSTER_TYPE = EXTERNAL);
ALTER AVAILABILITY GROUP [ReportingAG] GRANT CREATE ANY DATABASE;
GO

ALTER AVAILABILITY GROUP [DisasterRecoveryAG] JOIN WITH (CLUSTER_TYPE = EXTERNAL);
-- Note: manual seeding for DR — must restore database manually
GO
```

### 7b — sql-ag-2 (joins ProductionAG and ReportingAG only)

```sql
ALTER AVAILABILITY GROUP [ProductionAG] JOIN WITH (CLUSTER_TYPE = EXTERNAL);
ALTER AVAILABILITY GROUP [ProductionAG] GRANT CREATE ANY DATABASE;
GO

ALTER AVAILABILITY GROUP [ReportingAG] JOIN WITH (CLUSTER_TYPE = EXTERNAL);
ALTER AVAILABILITY GROUP [ReportingAG] GRANT CREATE ANY DATABASE;
GO
```

### 7c — Manual restore for DR AG (sql-ag-1)

Since DisasterRecoveryAG uses manual seeding, restore the database on sql-ag-1:

```bash
# Copy backup from sql-ag-0 to sql-ag-1
kubectl -n mssql cp sql-ag-0:/var/opt/mssql/backup/CriticalDB.bak ./CriticalDB.bak
kubectl -n mssql cp ./CriticalDB.bak sql-ag-1:/var/opt/mssql/backup/CriticalDB.bak
```

On **sql-ag-1**:

```sql
RESTORE DATABASE [CriticalDB]
  FROM DISK = '/var/opt/mssql/backup/CriticalDB.bak'
  WITH NORECOVERY, REPLACE;

RESTORE LOG [CriticalDB]
  FROM DISK = '/var/opt/mssql/backup/CriticalDB.bak'
  WITH NORECOVERY;

ALTER DATABASE [CriticalDB] SET HADR AVAILABILITY GROUP = [DisasterRecoveryAG];
GO
```

---

## Step 8 — Verify All AGs

Run on **sql-ag-0**:

```sql
SELECT
    ag.name AS ag_name,
    rs.role_desc,
    rs.connected_state_desc,
    rs.synchronization_health_desc,
    db.database_name,
    dbs.synchronization_state_desc
FROM sys.dm_hadr_availability_replica_states rs
JOIN sys.availability_groups ag ON rs.group_id = ag.group_id
LEFT JOIN sys.dm_hadr_database_replica_states dbs ON rs.replica_id = dbs.replica_id
LEFT JOIN sys.availability_databases_cluster db ON dbs.group_database_id = db.group_database_id
ORDER BY ag.name, rs.role_desc;
GO
```

### Expected results

| AG | Replica | Role | Sync State |
|----|---------|------|------------|
| ProductionAG | sql-ag-0 | PRIMARY | SYNCHRONIZED |
| ProductionAG | sql-ag-1 | SECONDARY | SYNCHRONIZED |
| ProductionAG | sql-ag-2 | SECONDARY | SYNCHRONIZED |
| ReportingAG | sql-ag-0 | PRIMARY | SYNCHRONIZED |
| ReportingAG | sql-ag-1 | SECONDARY | SYNCHRONIZING |
| ReportingAG | sql-ag-2 | SECONDARY | SYNCHRONIZING |
| DisasterRecoveryAG | sql-ag-0 | PRIMARY | SYNCHRONIZED |
| DisasterRecoveryAG | sql-ag-1 | SECONDARY | SYNCHRONIZING |

> **Note**: Async replicas show SYNCHRONIZING — this is expected.

---

## Troubleshooting

### AG Not Creating
```sql
-- Check endpoint status
SELECT name, state_desc, port FROM sys.endpoints WHERE type_desc = 'DATABASE_MIRRORING';
-- Verify certificates
SELECT name, expiry_date FROM sys.certificates WHERE name = 'ag_cert';
```

### Seeding Failures
```sql
-- Check automatic seeding status
SELECT * FROM sys.dm_hadr_automatic_seeding ORDER BY start_time DESC;
```

### Manual Failover (ReportingAG or DisasterRecoveryAG)
```sql
-- On the target secondary (will become new primary):
ALTER AVAILABILITY GROUP [ReportingAG] FORCE_FAILOVER_ALLOW_DATA_LOSS;
-- For DR AG:
ALTER AVAILABILITY GROUP [DisasterRecoveryAG] FORCE_FAILOVER_ALLOW_DATA_LOSS;
```

### Port Mapping Quick Reference
```
OLTP writes       → production-ag-primary:1433
OLTP reads         → production-ag-secondary:1434 or productionag-listener:1433
Reporting queries  → reporting-ag-secondary:2434
DR (internal)      → dr-ag-primary:3433
```
