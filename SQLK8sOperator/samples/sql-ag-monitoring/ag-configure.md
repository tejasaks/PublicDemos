# AG with Monitoring — T-SQL Configuration Guide

This guide sets up the **ProductionAG** Availability Group on 3 SQL Server
replicas that have Prometheus + Grafana monitoring enabled.

The T-SQL steps are identical to the standard HA scenario. Additional
monitoring verification steps are included at the end.

> **Automation**: Run `./ag-configure.sh all` to execute all steps below.

---

## Prerequisites

```bash
kubectl apply -f ag-deploy.yaml
kubectl -n mssql wait --for=condition=ready pod/sql-ag-0 pod/sql-ag-1 pod/sql-ag-2 --timeout=300s
```

Verify pods show **2/2** containers initially (mssql-server + sql-exporter):

```bash
kubectl get pods -n mssql
# sql-ag-0   2/2     Running   0          1m
# sql-ag-1   2/2     Running   0          1m
# sql-ag-2   2/2     Running   0          1m
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

GRANT VIEW SERVER STATE TO [ag_helper];
GRANT ALTER ANY AVAILABILITY GROUP TO [ag_helper];
GO
```

---

## Step 2 — Create Master Keys and Certificates

### 2a — Master keys (all replicas)

```sql
USE master;
CREATE MASTER KEY ENCRYPTION BY PASSWORD = N'MasterKey@Passw0rd!';
GO
```

### 2b — Certificates

On **sql-ag-0**:

```sql
CREATE CERTIFICATE AG_Cert_0
  WITH SUBJECT = 'AG Certificate for sql-ag-0',
       EXPIRY_DATE = '20301231';

BACKUP CERTIFICATE AG_Cert_0
  TO FILE = '/var/opt/mssql/data/AG_Cert_0.cer';
GO
```

Repeat on **sql-ag-1** and **sql-ag-2** with `AG_Cert_1` and `AG_Cert_2`.

### 2c — Exchange certificates

```bash
# Export from each pod
for i in 0 1 2; do
  kubectl -n mssql cp sql-ag-${i}:/var/opt/mssql/data/AG_Cert_${i}.cer ./AG_Cert_${i}.cer
done

# Import to each pod (all certs)
for i in 0 1 2; do
  for j in 0 1 2; do
    [ "$i" != "$j" ] && kubectl -n mssql cp ./AG_Cert_${j}.cer sql-ag-${i}:/var/opt/mssql/data/AG_Cert_${j}.cer
  done
done
```

### 2d — Import certificates (each replica imports the others)

On each replica, create logins and import the certificates from the other replicas:

```sql
-- On sql-ag-0: import certs from sql-ag-1 and sql-ag-2
CREATE LOGIN sql_ag_1_login WITH PASSWORD = N'ReplicaLogin@Passw0rd!';
CREATE USER sql_ag_1_user FOR LOGIN sql_ag_1_login;
CREATE CERTIFICATE AG_Cert_1 AUTHORIZATION sql_ag_1_user
  FROM FILE = '/var/opt/mssql/data/AG_Cert_1.cer';

CREATE LOGIN sql_ag_2_login WITH PASSWORD = N'ReplicaLogin@Passw0rd!';
CREATE USER sql_ag_2_user FOR LOGIN sql_ag_2_login;
CREATE CERTIFICATE AG_Cert_2 AUTHORIZATION sql_ag_2_user
  FROM FILE = '/var/opt/mssql/data/AG_Cert_2.cer';
GO
```

Repeat the pattern on sql-ag-1 (import 0 and 2) and sql-ag-2 (import 0 and 1).

---

## Step 3 — Create AG Endpoints (all replicas)

Run on **all three replicas** (using each replica's own certificate):

```sql
-- On sql-ag-0:
CREATE ENDPOINT AG_Endpoint
  STATE = STARTED
  AS TCP (LISTENER_PORT = 5022, LISTENER_IP = ALL)
  FOR DATABASE_MIRRORING (
    ROLE = ALL,
    AUTHENTICATION = CERTIFICATE AG_Cert_0,
    ENCRYPTION = REQUIRED ALGORITHM AES
  );

GRANT CONNECT ON ENDPOINT::AG_Endpoint TO sql_ag_1_login;
GRANT CONNECT ON ENDPOINT::AG_Endpoint TO sql_ag_2_login;
GO
```

Repeat on sql-ag-1 (cert 1, grant to 0 and 2) and sql-ag-2 (cert 2, grant to 0 and 1).

---

## Step 4 — Create Database (sql-ag-0 only)

```sql
CREATE DATABASE [ApplicationDB];
ALTER DATABASE [ApplicationDB] SET RECOVERY FULL;
BACKUP DATABASE [ApplicationDB]
  TO DISK = '/var/opt/mssql/backup/ApplicationDB_init.bak'
  WITH INIT, COMPRESSION;
GO
```

---

## Step 5 — Create Availability Group (sql-ag-0 only)

```sql
CREATE AVAILABILITY GROUP [ProductionAG]
  WITH (
    CLUSTER_TYPE = EXTERNAL,
    DB_FAILOVER = ON,
    REQUIRED_SYNCHRONIZED_SECONDARIES_TO_COMMIT = 1
  )
  FOR DATABASE [ApplicationDB]
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

---

## Step 6 — Join Secondary Replicas

On **sql-ag-1** and **sql-ag-2**:

```sql
ALTER AVAILABILITY GROUP [ProductionAG] JOIN WITH (CLUSTER_TYPE = EXTERNAL);
ALTER AVAILABILITY GROUP [ProductionAG] GRANT CREATE ANY DATABASE;
GO
```

---

## Step 7 — Verify AG Status

```sql
SELECT ag.name AS ag_name, ar.replica_server_name,
    ars.role_desc, ars.synchronization_health_desc, ars.connected_state_desc
FROM sys.availability_groups ag
JOIN sys.availability_replicas ar ON ag.group_id = ar.group_id
JOIN sys.dm_hadr_availability_replica_states ars ON ar.replica_id = ars.replica_id
ORDER BY ar.replica_server_name;
GO
```

### Expected results

| Replica | Role | Sync Health | Connected |
|---------|------|-------------|-----------|
| sql-ag-0 | PRIMARY | HEALTHY | CONNECTED |
| sql-ag-1 | SECONDARY | HEALTHY | CONNECTED |
| sql-ag-2 | SECONDARY | HEALTHY | CONNECTED |

---

## Step 8 — Verify Monitoring

After the AG is created, metrics appear automatically within ~30 seconds.

### 8a — Check sql-exporter metrics

```bash
kubectl port-forward pod/sql-ag-0 -n mssql 9399:9399 &
curl -s http://localhost:9399/metrics | grep mssql_ag
```

Expected output:

```
mssql_ag_is_primary{ag_name="ProductionAG"} 1
mssql_ag_replica_role{ag_name="ProductionAG",replica="sql-ag-0"} 1
mssql_ag_synchronization_health{ag_name="ProductionAG"} 2
```

### 8b — Verify Prometheus targets

```bash
kubectl port-forward svc/prometheus -n monitoring 9090:9090 &
```

Open http://localhost:9090/targets — you should see 3 targets under "sql-exporter", all UP.

### 8c — Access Grafana

```bash
kubectl port-forward svc/grafana -n monitoring 3000:3000 &
```

Open http://localhost:3000

- Login: **admin** / **admin**
- Navigate: Dashboards → SQL Server folder
- Two dashboards available:
  - **SQL Server AG Monitoring** — AG roles, sync state, redo/log queue lag
  - **SQL Server Overview** — CPU, memory, connections, batch req/sec

### 8d — Useful PromQL queries

```promql
# Which replica is primary?
mssql_ag_replica_role{ag_name="ProductionAG"} == 1

# Any databases NOT synchronized?
mssql_ag_database_synchronization_state{ag_name="ProductionAG"} != 2

# Redo queue > 1GB (secondary falling behind)
mssql_ag_redo_queue_size{ag_name="ProductionAG"} > 1073741824

# CPU over 80% on any replica
mssql_cpu_percent{sqlserver_instance="sql-ag"} > 80
```

---

## Step 9 — AG Listener (optional)

Follow the same listener setup as the HA scenario:

```bash
./ag-configure.sh listener
```

Or manually — see the sql-ag-ha sample for detailed listener T-SQL.

---

## Troubleshooting

### Grafana shows "No data"
- Verify Prometheus targets are UP: http://localhost:9090/targets
- Check sql-exporter logs: `kubectl logs sql-ag-0 -n mssql -c sql-exporter`
- Confirm pod annotations: `kubectl get pod sql-ag-0 -n mssql -o yaml | grep prometheus`

### AG Helper not detecting AG
- Check AG Helper logs: `kubectl logs sql-ag-0 -n mssql -c ag-helper --tail=10`
- Verify pods are 3/3 Ready: `kubectl get pods -n mssql`
- Check SQLServerAG status: `kubectl get sqlserverag -n mssql`

### Prometheus not scraping
- Ensure pods have annotation `prometheus.io/scrape: "true"`
- Check Prometheus config: `kubectl get cm prometheus-config -n monitoring -o yaml`
- Verify RBAC: `kubectl get clusterrolebinding prometheus-mssql-monitoring`
