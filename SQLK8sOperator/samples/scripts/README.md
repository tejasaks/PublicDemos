# Availability Group Setup Scripts

This folder contains T-SQL scripts to configure an Availability Group after the SQL Server pods are deployed.

## Why Manual Steps?

The MSSQL Kubernetes Operator creates and manages:
- SQL Server StatefulSets with multiple replicas
- AG Helper sidecars for health monitoring and failover
- Kubernetes Services for primary/secondary routing

However, the actual Availability Group configuration (T-SQL) must be executed manually because:
1. Certificate exchange between replicas requires coordination
2. Database creation is application-specific
3. AG configuration options vary by workload

## Quick Start

### 1. Deploy the SQL Server pods

```bash
kubectl apply -f samples/sqlserver-availability-group.yaml
```

### 2. Wait for all pods to be running

```bash
kubectl get pods -n mssql -w
# Wait until all 3 pods show: Running 2/2 (SQL Server + AG Helper sidecar)
```

### 3. Copy scripts to the primary pod

```bash
kubectl cp samples/scripts/setup-availability-group.sql \
  mssql/sql-ag-prod01-0:/var/opt/mssql/scripts/setup-ag.sql

kubectl cp samples/scripts/join-secondary.sql \
  mssql/sql-ag-prod01-1:/var/opt/mssql/scripts/join-secondary.sql

kubectl cp samples/scripts/join-secondary.sql \
  mssql/sql-ag-prod01-2:/var/opt/mssql/scripts/join-secondary.sql

kubectl cp samples/scripts/verify-ag-status.sql \
  mssql/sql-ag-prod01-0:/var/opt/mssql/scripts/verify-ag.sql
```

### 4. Run setup on primary

```bash
kubectl exec -it sql-ag-prod01-0 -n mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa \
  -P 'YourStrong@Passw0rd!' -C -i /var/opt/mssql/scripts/setup-ag.sql
```

### 5. Join secondary replicas

```bash
# Join replica 1
kubectl exec -it sql-ag-prod01-1 -n mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa \
  -P 'YourStrong@Passw0rd!' -C -i /var/opt/mssql/scripts/join-secondary.sql

# Join replica 2
kubectl exec -it sql-ag-prod01-2 -n mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa \
  -P 'YourStrong@Passw0rd!' -C -i /var/opt/mssql/scripts/join-secondary.sql
```

### 6. Verify AG status

```bash
kubectl exec -it sql-ag-prod01-0 -n mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa \
  -P 'YourStrong@Passw0rd!' -C -i /var/opt/mssql/scripts/verify-ag.sql
```

## Scripts

| Script | Run On | Purpose |
|--------|--------|---------|
| `setup-availability-group.sql` | Primary (pod-0) | Creates databases, endpoints, and AG |
| `join-secondary.sql` | Secondaries (pod-1, pod-2) | Joins replica to AG, enables seeding |
| `verify-ag-status.sql` | Any replica | Shows AG health and sync status |

## What Gets Created

The setup script creates:

1. **Master Key** - For encrypting certificates
2. **AG Authentication Certificate** - For endpoint authentication
3. **Database Mirroring Endpoint** - Port 5022 for AG communication
4. **Sample Databases**:
   - `ApplicationDB` - Example application database
   - `ReportingDB` - Example reporting database
5. **Availability Group** - `ProductionAG` with:
   - 3 synchronous commit replicas
   - External failover mode (Kubernetes-managed)
   - Automatic database seeding
   - Read-only access on secondaries

## Customization

### Different AG Name

Edit the `CREATE AVAILABILITY GROUP` statement in `setup-availability-group.sql`:
```sql
CREATE AVAILABILITY GROUP YourAGName
```

### Different Databases

Replace `ApplicationDB` and `ReportingDB` with your databases:
```sql
CREATE DATABASE YourDatabase;
ALTER DATABASE YourDatabase SET RECOVERY FULL;
BACKUP DATABASE YourDatabase TO DISK = '...';
```

### Different Pod Names

If your SQLServer resource has a different name, update the endpoint URLs:
```sql
ENDPOINT_URL = N'TCP://your-pod-name-0.your-pod-name-pods.namespace.svc.cluster.local:5022'
```

## Troubleshooting

### Pods not connecting

Check endpoint status:
```sql
SELECT * FROM sys.database_mirroring_endpoints;
SELECT * FROM sys.dm_hadr_availability_replica_states;
```

### Databases not seeding

Check seeding status:
```sql
SELECT * FROM sys.dm_hadr_automatic_seeding;
```

Check for errors:
```sql
SELECT * FROM sys.dm_hadr_automatic_seeding 
WHERE failure_state_desc IS NOT NULL;
```

### Certificate issues

Ensure certificates match across replicas. For production, exchange certificates:
```sql
-- On each replica, backup and share certificates
BACKUP CERTIFICATE AG_Auth_Cert TO FILE = '/path/cert.cer';
-- Copy to other replicas and create from file
```

## Connection Strings

After AG is configured:

```
# Primary (read-write)
Server=ProductionAG-primary.mssql.svc.cluster.local;Database=ApplicationDB;...

# Secondary (read-only)
Server=ProductionAG-secondary.mssql.svc.cluster.local;Database=ApplicationDB;...ApplicationIntent=ReadOnly
```
