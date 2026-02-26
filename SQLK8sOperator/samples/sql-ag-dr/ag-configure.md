# T-SQL Configuration for DR Availability Group

[← Back to README](README.md)

This guide creates the `DisasterRecoveryAG` Availability Group with 2 synchronous-commit
replicas (local HA) and 1 asynchronous-commit replica (geo-DR).

An automated shell script is available: `ag-configure.sh`.

## Prerequisites

- [ ] `ag-deploy.yaml` has been applied
- [ ] All 3 pods are Running (`kubectl get pods -n mssql`)

> **Note:** The SQLServerAG resource is already deployed and retrying. It will converge
> automatically once the T-SQL below is completed.

---

## Overview

| Step | Description | Run On |
|------|-------------|--------|
| 1 | Create AG Helper login | ALL replicas |
| 2 | Create master key and certificates | ALL replicas |
| 3 | Export/import certificates | Primary → Secondaries |
| 4 | Create database mirroring endpoints | ALL replicas |
| 5 | Create databases | Primary only |
| 6 | Create Availability Group (2 sync + 1 async) | Primary only |
| 7 | Join secondary replicas | Secondaries only |
| 8 | Verify AG status | Any replica |

---

## Steps 1–4: Identical to HA Scenario

Steps 1–4 (AG Helper login, certificates, endpoints) are the same across all AG scenarios.
Run the following commands exactly as documented below.

### Step 1: Create AG Helper Login (all replicas)

```bash
for i in 0 1 2; do
  echo "=== Creating AG Helper login on sql-ag-$i ==="
  kubectl exec -it sql-ag-$i -n mssql -c mssql -- \
    /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa \
    -P 'YourStrong@Passw0rd!' -C -Q "
CREATE LOGIN ag_helper WITH PASSWORD = 'AGHelper@Passw0rd!';
GO
GRANT VIEW SERVER STATE TO ag_helper;
GRANT ALTER ANY AVAILABILITY GROUP TO ag_helper;
GO
"
done
```

### Step 2: Create Master Key and Certificates (all replicas)

```bash
for i in 0 1 2; do
  kubectl exec -it sql-ag-$i -n mssql -c mssql -- \
    /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa \
    -P 'YourStrong@Passw0rd!' -C -Q "
CREATE MASTER KEY ENCRYPTION BY PASSWORD = 'MasterKey@Passw0rd!';
GO
CREATE CERTIFICATE AG_Cert_$i
    WITH SUBJECT = 'AG Certificate for sql-ag-$i',
    EXPIRY_DATE = '2030-12-31';
GO
"
done
```

### Step 3: Export and Import Certificates

```bash
# 3.1: Backup
for i in 0 1 2; do
  kubectl exec -it sql-ag-$i -n mssql -c mssql -- \
    /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa \
    -P 'YourStrong@Passw0rd!' -C -Q "
BACKUP CERTIFICATE AG_Cert_$i TO FILE = '/var/opt/mssql/data/AG_Cert_$i.cer';
GO
"
done

# 3.2: Exchange
mkdir -p /tmp/ag-certs && cd /tmp/ag-certs
kubectl cp mssql/sql-ag-0:/var/opt/mssql/data/AG_Cert_0.cer ./AG_Cert_0.cer
kubectl cp mssql/sql-ag-1:/var/opt/mssql/data/AG_Cert_1.cer ./AG_Cert_1.cer
kubectl cp mssql/sql-ag-2:/var/opt/mssql/data/AG_Cert_2.cer ./AG_Cert_2.cer

kubectl cp ./AG_Cert_1.cer mssql/sql-ag-0:/var/opt/mssql/data/AG_Cert_1.cer
kubectl cp ./AG_Cert_2.cer mssql/sql-ag-0:/var/opt/mssql/data/AG_Cert_2.cer
kubectl cp ./AG_Cert_0.cer mssql/sql-ag-1:/var/opt/mssql/data/AG_Cert_0.cer
kubectl cp ./AG_Cert_2.cer mssql/sql-ag-1:/var/opt/mssql/data/AG_Cert_2.cer
kubectl cp ./AG_Cert_0.cer mssql/sql-ag-2:/var/opt/mssql/data/AG_Cert_0.cer
kubectl cp ./AG_Cert_1.cer mssql/sql-ag-2:/var/opt/mssql/data/AG_Cert_1.cer
rm -rf /tmp/ag-certs

# 3.3: Import (each replica imports the OTHER replicas' certs)
kubectl exec -it sql-ag-0 -n mssql -c mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P 'YourStrong@Passw0rd!' -C -Q "
CREATE LOGIN sql_ag_1_login WITH PASSWORD = 'ReplicaLogin@Passw0rd!';
CREATE USER sql_ag_1_user FOR LOGIN sql_ag_1_login;
CREATE CERTIFICATE AG_Cert_1 AUTHORIZATION sql_ag_1_user FROM FILE = '/var/opt/mssql/data/AG_Cert_1.cer';
GO
CREATE LOGIN sql_ag_2_login WITH PASSWORD = 'ReplicaLogin@Passw0rd!';
CREATE USER sql_ag_2_user FOR LOGIN sql_ag_2_login;
CREATE CERTIFICATE AG_Cert_2 AUTHORIZATION sql_ag_2_user FROM FILE = '/var/opt/mssql/data/AG_Cert_2.cer';
GO
"

kubectl exec -it sql-ag-1 -n mssql -c mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P 'YourStrong@Passw0rd!' -C -Q "
CREATE LOGIN sql_ag_0_login WITH PASSWORD = 'ReplicaLogin@Passw0rd!';
CREATE USER sql_ag_0_user FOR LOGIN sql_ag_0_login;
CREATE CERTIFICATE AG_Cert_0 AUTHORIZATION sql_ag_0_user FROM FILE = '/var/opt/mssql/data/AG_Cert_0.cer';
GO
CREATE LOGIN sql_ag_2_login WITH PASSWORD = 'ReplicaLogin@Passw0rd!';
CREATE USER sql_ag_2_user FOR LOGIN sql_ag_2_login;
CREATE CERTIFICATE AG_Cert_2 AUTHORIZATION sql_ag_2_user FROM FILE = '/var/opt/mssql/data/AG_Cert_2.cer';
GO
"

kubectl exec -it sql-ag-2 -n mssql -c mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P 'YourStrong@Passw0rd!' -C -Q "
CREATE LOGIN sql_ag_0_login WITH PASSWORD = 'ReplicaLogin@Passw0rd!';
CREATE USER sql_ag_0_user FOR LOGIN sql_ag_0_login;
CREATE CERTIFICATE AG_Cert_0 AUTHORIZATION sql_ag_0_user FROM FILE = '/var/opt/mssql/data/AG_Cert_0.cer';
GO
CREATE LOGIN sql_ag_1_login WITH PASSWORD = 'ReplicaLogin@Passw0rd!';
CREATE USER sql_ag_1_user FOR LOGIN sql_ag_1_login;
CREATE CERTIFICATE AG_Cert_1 AUTHORIZATION sql_ag_1_user FROM FILE = '/var/opt/mssql/data/AG_Cert_1.cer';
GO
"
```

### Step 4: Create Database Mirroring Endpoints (all replicas)

```bash
kubectl exec -it sql-ag-0 -n mssql -c mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P 'YourStrong@Passw0rd!' -C -Q "
CREATE ENDPOINT AG_Endpoint STATE = STARTED
    AS TCP (LISTENER_PORT = 5022, LISTENER_IP = ALL)
    FOR DATABASE_MIRRORING (ROLE = ALL, AUTHENTICATION = CERTIFICATE AG_Cert_0, ENCRYPTION = REQUIRED ALGORITHM AES);
GO
GRANT CONNECT ON ENDPOINT::AG_Endpoint TO sql_ag_1_login;
GRANT CONNECT ON ENDPOINT::AG_Endpoint TO sql_ag_2_login;
GO
"

kubectl exec -it sql-ag-1 -n mssql -c mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P 'YourStrong@Passw0rd!' -C -Q "
CREATE ENDPOINT AG_Endpoint STATE = STARTED
    AS TCP (LISTENER_PORT = 5022, LISTENER_IP = ALL)
    FOR DATABASE_MIRRORING (ROLE = ALL, AUTHENTICATION = CERTIFICATE AG_Cert_1, ENCRYPTION = REQUIRED ALGORITHM AES);
GO
GRANT CONNECT ON ENDPOINT::AG_Endpoint TO sql_ag_0_login;
GRANT CONNECT ON ENDPOINT::AG_Endpoint TO sql_ag_2_login;
GO
"

kubectl exec -it sql-ag-2 -n mssql -c mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P 'YourStrong@Passw0rd!' -C -Q "
CREATE ENDPOINT AG_Endpoint STATE = STARTED
    AS TCP (LISTENER_PORT = 5022, LISTENER_IP = ALL)
    FOR DATABASE_MIRRORING (ROLE = ALL, AUTHENTICATION = CERTIFICATE AG_Cert_2, ENCRYPTION = REQUIRED ALGORITHM AES);
GO
GRANT CONNECT ON ENDPOINT::AG_Endpoint TO sql_ag_0_login;
GRANT CONNECT ON ENDPOINT::AG_Endpoint TO sql_ag_1_login;
GO
"
```

---

## Step 5: Create Database (primary only)

```bash
kubectl exec -it sql-ag-0 -n mssql -c mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa \
  -P 'YourStrong@Passw0rd!' -C -Q "
CREATE DATABASE CriticalDB;
GO
ALTER DATABASE CriticalDB SET RECOVERY FULL;
GO
BACKUP DATABASE CriticalDB 
    TO DISK = '/var/opt/mssql/backup/CriticalDB_init.bak'
    WITH INIT, COMPRESSION;
GO
"
```

---

## Step 6: Create Availability Group (primary only)

**Key difference from the HA scenario:** sql-ag-2 uses `ASYNCHRONOUS_COMMIT` with
`FAILOVER_MODE = MANUAL` and a longer `SESSION_TIMEOUT` for geo-replication tolerance.

```bash
kubectl exec -it sql-ag-0 -n mssql -c mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa \
  -P 'YourStrong@Passw0rd!' -C -Q "
CREATE AVAILABILITY GROUP DisasterRecoveryAG
    WITH (
        CLUSTER_TYPE = EXTERNAL,
        DB_FAILOVER = ON,
        REQUIRED_SYNCHRONIZED_SECONDARIES_TO_COMMIT = 1
    )
    FOR DATABASE CriticalDB
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
        -- DR replica: async commit, manual failover, longer timeout
        N'sql-ag-2' WITH (
            ENDPOINT_URL = N'TCP://sql-ag-2.mssql.svc.cluster.local:5022',
            AVAILABILITY_MODE = ASYNCHRONOUS_COMMIT,
            FAILOVER_MODE = MANUAL,
            SEEDING_MODE = AUTOMATIC,
            SESSION_TIMEOUT = 30,
            SECONDARY_ROLE (ALLOW_CONNECTIONS = NO)
        );
GO
"
```

---

## Step 7: Join Secondary Replicas

```bash
for i in 1 2; do
  echo "=== Joining sql-ag-$i ==="
  kubectl exec -it sql-ag-$i -n mssql -c mssql -- \
    /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa \
    -P 'YourStrong@Passw0rd!' -C -Q "
ALTER AVAILABILITY GROUP DisasterRecoveryAG JOIN WITH (CLUSTER_TYPE = EXTERNAL);
GO
ALTER AVAILABILITY GROUP DisasterRecoveryAG GRANT CREATE ANY DATABASE;
GO
"
done
```

---

## Step 8: Verify AG Status

```bash
kubectl exec -it sql-ag-0 -n mssql -c mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa \
  -P 'YourStrong@Passw0rd!' -C -Q "
SELECT ag.name AS ag_name, ar.replica_server_name,
    ars.role_desc, ars.synchronization_health_desc, ars.connected_state_desc
FROM sys.availability_groups ag
JOIN sys.availability_replicas ar ON ag.group_id = ar.group_id
JOIN sys.dm_hadr_availability_replica_states ars ON ar.replica_id = ars.replica_id
ORDER BY ar.replica_server_name;
GO
"
```

**Expected:**
- sql-ag-0: `PRIMARY`, `HEALTHY`, `CONNECTED`
- sql-ag-1: `SECONDARY`, `HEALTHY` (synchronous), `CONNECTED`
- sql-ag-2: `SECONDARY`, `HEALTHY` or `PARTIALLY_HEALTHY` (async may lag), `CONNECTED`

---

## Manual Failover

Since this is a DR scenario with no automatic failover, use kubectl:

```bash
# Failover to sql-ag-1 (synchronous secondary — no data loss)
kubectl annotate sqlserverag dr-ag -n mssql \
  mssql.microsoft.com/failover-to=sql-ag-1

# Failover to sql-ag-2 (async DR — potential data loss!)
# Only use in actual disaster scenarios
kubectl annotate sqlserverag dr-ag -n mssql \
  mssql.microsoft.com/failover-to=sql-ag-2
```

---

## Troubleshooting

See the troubleshooting section in the [HA sample](../sql-ag-ha/ag-configure.md#troubleshooting) —
the same diagnostics apply here.
