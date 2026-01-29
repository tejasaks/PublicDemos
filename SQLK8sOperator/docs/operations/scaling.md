# Scaling SQL Server

[← Back to Operations](../README.md) | [Documentation Home](../README.md)

Guide to scaling SQL Server deployments horizontally and vertically.

## Table of Contents

- [Scaling Types](#scaling-types)
- [Horizontal Scaling (Replicas)](#horizontal-scaling-replicas)
- [Vertical Scaling (Resources)](#vertical-scaling-resources)
- [Storage Scaling](#storage-scaling)
- [Read Scale-Out](#read-scale-out)
- [Scaling Best Practices](#scaling-best-practices)

## Scaling Types

| Type | What Changes | Use Case |
|------|--------------|----------|
| Horizontal | Number of replicas | HA, read scale-out |
| Vertical | CPU, memory per pod | Performance |
| Storage | Disk size | Data growth |

## Horizontal Scaling (Replicas)

### Adding Replicas

Increase replica count for HA or read scale-out.

**Step 1: Edit your SQLServer manifest**

```bash
nano sqlserver-prod.yaml
```

Update the replica count:

```yaml
apiVersion: mssql.microsoft.com/v1alpha1
kind: SQLServer
metadata:
  name: sql-prod
spec:
  instance:
    replicas: 5  # Increased from 3
```

**Step 2: Apply the change**

```bash
kubectl apply -f sqlserver-prod.yaml
```

**Expected output:**
```
sqlserver.mssql.microsoft.com/sql-prod configured
```

**Step 3: Watch new pods being created**

```bash
kubectl get pods -n mssql -w
```

**Expected output:**
```
NAME         READY   STATUS    RESTARTS   AGE
sql-prod-0   1/1     Running   0          5d
sql-prod-1   1/1     Running   0          5d
sql-prod-2   1/1     Running   0          5d
sql-prod-3   0/1     Pending   0          5s
sql-prod-3   0/1     ContainerCreating   0   10s
sql-prod-3   1/1     Running   0          45s
sql-prod-4   0/1     Pending   0          5s
...
```

### What Happens

1. StatefulSet creates new pods (sql-prod-3, sql-prod-4)
2. PVCs created for new pods
3. SQL Server initialized on new pods
4. If AG exists, new replicas must be joined manually

### Join New Replicas to AG

If you have an Availability Group, you must manually join new replicas.

**Step 1: Set the SA password**

```bash
export SA_PWD=$(kubectl get secret sql-prod-sa -n mssql -o jsonpath='{.data.password}' | base64 -d)
```

**Step 2: Add the replica on the primary**

Run on the primary pod (sql-prod-0):

```bash
kubectl exec -it sql-prod-0 -n mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P "$SA_PWD" -C \
  -Q "ALTER AVAILABILITY GROUP ProductionAG ADD REPLICA ON N'sql-prod-3' WITH (ENDPOINT_URL = N'TCP://sql-prod-3.sql-prod-pods.mssql.svc.cluster.local:5022', AVAILABILITY_MODE = SYNCHRONOUS_COMMIT, FAILOVER_MODE = EXTERNAL, SEEDING_MODE = AUTOMATIC, SECONDARY_ROLE (ALLOW_CONNECTIONS = READ_ONLY));"
```

**Expected output:**
```
Command(s) completed successfully.
```

**Step 3: Join the AG on the new secondary**

Run on the new pod (sql-prod-3):

```bash
kubectl exec -it sql-prod-3 -n mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P "$SA_PWD" -C \
  -Q "ALTER AVAILABILITY GROUP ProductionAG JOIN WITH (CLUSTER_TYPE = EXTERNAL); ALTER AVAILABILITY GROUP ProductionAG GRANT CREATE ANY DATABASE;"
```

**Expected output:**
```
Command(s) completed successfully.
```

**Step 4: Verify the new replica is synchronized**

```bash
kubectl exec -it sql-prod-0 -n mssql -c ag-helper -- curl -s localhost:8080/state | jq
```

### Removing Replicas

Reduce replica count by first removing replicas from the AG.

**Step 1: Remove replica from AG (if applicable)**

> ⚠️ **Warning**: Remove replicas from AG first before scaling down.

```bash
kubectl exec -it sql-prod-0 -n mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P "$SA_PWD" -C \
  -Q "ALTER AVAILABILITY GROUP ProductionAG REMOVE REPLICA ON N'sql-prod-4'; ALTER AVAILABILITY GROUP ProductionAG REMOVE REPLICA ON N'sql-prod-3';"
```

**Expected output:**
```
Command(s) completed successfully.
```

**Step 2: Update the manifest with reduced replica count**

```bash
nano sqlserver-prod.yaml
```

```yaml
spec:
  instance:
    replicas: 3  # Reduced from 5
```

**Step 3: Apply the change**

```bash
kubectl apply -f sqlserver-prod.yaml
```

**Expected output:**
```
sqlserver.mssql.microsoft.com/sql-prod configured
```

**Step 4: Verify pods are terminated**

```bash
kubectl get pods -n mssql

# Expected output:
# NAME         READY   STATUS    RESTARTS   AGE
# sql-prod-0   1/1     Running   0          5d
# sql-prod-1   1/1     Running   0          5d
# sql-prod-2   1/1     Running   0          5d
```

### Scaling Limits

| Scenario | Min | Max | Notes |
|----------|-----|-----|-------|
| Standalone | 1 | 1 | No HA |
| With AG | 3 | 9 | SQL Server AG limit |
| Read-only | 0 | ∞ | Snapshot replicas |

## Vertical Scaling (Resources)

Increase CPU and memory allocated to each SQL Server pod.

### Update Resource Requests/Limits

**Step 1: Edit your SQLServer manifest**

```bash
nano sqlserver-prod.yaml
```

Update the resource section:

```yaml
apiVersion: mssql.microsoft.com/v1alpha1
kind: SQLServer
metadata:
  name: sql-prod
spec:
  instance:
    resources:
      requests:
        cpu: "4"      # Increased from 2
        memory: 16Gi   # Increased from 8Gi
      limits:
        cpu: "8"       # Increased from 4
        memory: 32Gi   # Increased from 16Gi
```

### Apply Changes

**Step 2: Apply the updated manifest**

```bash
kubectl apply -f sqlserver-prod.yaml
```

**Expected output:**
```
sqlserver.mssql.microsoft.com/sql-prod configured
```

**Step 3: Monitor the rolling update**

```bash
kubectl get pods -n mssql -w
```

**Expected output:**
```
NAME         READY   STATUS        RESTARTS   AGE
sql-prod-0   1/1     Running       0          5d
sql-prod-1   1/1     Running       0          5d
sql-prod-2   0/1     Terminating   0          5d
sql-prod-2   0/1     Pending       0          0s
sql-prod-2   0/1     ContainerCreating   0   5s
sql-prod-2   1/1     Running       0          45s
...
```

### What Happens

1. StatefulSet updated
2. Pods restarted one at a time (rolling update)
3. SQL Server automatically uses new resources

### SQL Server Memory Configuration

After scaling memory, verify SQL Server sees the new memory:

**Step 1: Set the SA password**

```bash
export SA_PWD=$(kubectl get secret sql-prod-sa -n mssql -o jsonpath='{.data.password}' | base64 -d)
```

**Step 2: Check physical memory**

```bash
kubectl exec -it sql-prod-0 -n mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P "$SA_PWD" -C \
  -Q "SELECT physical_memory_kb/1024 AS physical_memory_mb FROM sys.dm_os_sys_info"
```

**Expected output:**
```
physical_memory_mb
------------------
             32768
```

### Resource Recommendations

| Workload | CPU | Memory | Notes |
|----------|-----|--------|-------|
| Development | 1-2 | 2-4Gi | Minimum viable |
| Small prod | 2-4 | 8-16Gi | Light OLTP |
| Medium prod | 4-8 | 16-32Gi | Standard OLTP |
| Large prod | 8-16 | 32-64Gi | Heavy workloads |
| Enterprise | 16+ | 64Gi+ | Mission-critical |

## Storage Scaling

Expand storage for growing databases.

### Expand PVC Size

> ⚠️ **Note**: Storage can only be expanded, not shrunk. Requires storage class with `allowVolumeExpansion: true`.

**Step 1: Edit your SQLServer manifest**

```bash
nano sqlserver-prod.yaml
```

Update the storage sizes:

```yaml
spec:
  instance:
    storage:
      data:
        size: 100Gi  # Increased from 50Gi
      log:
        size: 50Gi   # Increased from 20Gi
      backup:
        size: 100Gi  # Increased from 50Gi
```

### Verify Expansion Support

Before applying, check if your storage class supports expansion:

```bash
kubectl get storageclass -o jsonpath='{range .items[*]}{.metadata.name}: {.allowVolumeExpansion}{"\n"}{end}'
```

**Expected output:**
```
standard: true
fast-ssd: true
```

If a storage class shows `false` or `<nil>`, you cannot expand PVCs using that class.

### Apply Storage Changes

**Step 2: Apply the manifest**

```bash
kubectl apply -f sqlserver-prod.yaml
```

**Expected output:**
```
sqlserver.mssql.microsoft.com/sql-prod configured
```

**Step 3: Watch PVC resize**

```bash
kubectl get pvc -n mssql -w
```

**Expected output:**
```
NAME                        STATUS   VOLUME      CAPACITY   STORAGECLASS   AGE
data-sql-prod-0             Bound    pvc-abc123  100Gi      fast-ssd       5d
data-sql-prod-1             Bound    pvc-def456  100Gi      fast-ssd       5d
data-sql-prod-2             Bound    pvc-ghi789  100Gi      fast-ssd       5d
```

### Verify New Size

**Check PVC capacity:**

```bash
kubectl get pvc -n mssql
```

**Expected output:**
```
NAME                  STATUS   VOLUME      CAPACITY   STORAGECLASS   AGE
data-sql-prod-0       Bound    pvc-abc123  100Gi      fast-ssd       5d
log-sql-prod-0        Bound    pvc-log123  50Gi       fast-ssd       5d
backup-sql-prod-0     Bound    pvc-bak123  100Gi      standard       5d
```

**Check from SQL Server:**

```bash
kubectl exec -it sql-prod-0 -n mssql -- df -h /var/opt/mssql
```

**Expected output:**
```
Filesystem      Size  Used Avail Use% Mounted on
/dev/sdb        100G   15G   85G  15% /var/opt/mssql
```

### Add Storage Volumes

Add new storage types (e.g., dedicated tempdb volume):

**Step 1: Edit your manifest to add a new volume**

```yaml
spec:
  instance:
    storage:
      data:
        size: 100Gi
      log:
        size: 50Gi
      backup:
        size: 100Gi
      tempdb:        # New volume for tempdb
        size: 20Gi
        storageClass: fast-ssd
```

**Step 2: Apply the change**

```bash
kubectl apply -f sqlserver-prod.yaml
```

**Note:** Adding new volumes may require pod restart. The operator will handle this automatically.

## Read Scale-Out

### Configure Secondary Services

Route read traffic to secondary replicas:

```yaml
apiVersion: mssql.microsoft.com/v1alpha1
kind: SQLServerAG
spec:
  endpoints:
    primary:
      type: LoadBalancer
      port: 1433
    secondary:
      type: LoadBalancer
      port: 1434
```

### Application Connection Strings

```
# Read-write queries (primary)
Server=prod-ag-primary.mssql.svc,1433;ApplicationIntent=ReadWrite;

# Read-only queries (secondary)
Server=prod-ag-secondary.mssql.svc,1434;ApplicationIntent=ReadOnly;
```

### Load Distribution

Secondaries use round-robin by default:

```
Request 1 → sql-prod-1
Request 2 → sql-prod-2
Request 3 → sql-prod-1
Request 4 → sql-prod-2
```

### Dedicated Read Replicas

Add replicas specifically for read workloads:

```yaml
spec:
  instance:
    replicas: 5  # 1 primary + 2 sync + 2 async read
```

Configure async replicas for read-only:

```sql
-- On primary
ALTER AVAILABILITY GROUP ProductionAG
    MODIFY REPLICA ON N'sql-prod-3' WITH (
        AVAILABILITY_MODE = ASYNCHRONOUS_COMMIT,
        SECONDARY_ROLE (ALLOW_CONNECTIONS = READ_ONLY)
    );

ALTER AVAILABILITY GROUP ProductionAG
    MODIFY REPLICA ON N'sql-prod-4' WITH (
        AVAILABILITY_MODE = ASYNCHRONOUS_COMMIT,
        SECONDARY_ROLE (ALLOW_CONNECTIONS = READ_ONLY)
    );
```

## Scaling Best Practices

### Before Scaling

| Check | Command |
|-------|---------|
| Current usage | `kubectl top pods -n mssql` |
| Storage class | `kubectl get pvc -n mssql` |
| Node capacity | `kubectl describe nodes` |
| AG health | `curl localhost:8080/state` |

### During Scaling

| Practice | Description |
|----------|-------------|
| One change at a time | Don't scale horizontally and vertically simultaneously |
| Monitor metrics | Watch Prometheus/Grafana during scale |
| Business hours | Scale during low-traffic periods |
| Test in non-prod | Validate scaling in staging first |

### After Scaling

| Task | Command |
|------|---------|
| Verify pods | `kubectl get pods -n mssql` |
| Check AG sync | `curl localhost:8080/state` |
| Application test | Run smoke tests |
| Update documentation | Record new capacity |

### Anti-Patterns

| Don't | Why |
|-------|-----|
| Scale down during high load | May cause connection drops |
| Remove primary replica | Causes failover |
| Exceed node capacity | Pods won't schedule |
| Ignore storage limits | Data loss risk |

## Next Steps

- [Upgrades](upgrades.md) - Version upgrades
- [Backup & Restore](backup-restore.md) - Data protection
- [Monitoring](../monitoring/overview.md) - Track performance
