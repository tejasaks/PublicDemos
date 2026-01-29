# Troubleshooting

[← Back to User Guide](../README.md) | [Documentation Home](../README.md)

Common issues and solutions when using the MSSQL Kubernetes Operator.

## Table of Contents

- [Pod Issues](#pod-issues)
- [Storage Issues](#storage-issues)
- [Connection Issues](#connection-issues)
- [AG Helper Issues](#ag-helper-issues)
- [Validation Errors](#validation-errors)
- [Monitoring Issues](#monitoring-issues)
- [Debugging Commands](#debugging-commands)

## Pod Issues

### Pod Stuck in Pending

**Symptoms:**
```
NAME             READY   STATUS    RESTARTS   AGE
my-sqlserver-0   0/2     Pending   0          5m
```

**Causes & Solutions:**

| Cause | Diagnosis | Solution |
|-------|-----------|----------|
| No storage provisioner | `kubectl get pvc -n mssql` shows Pending | Install storage provisioner or use existing StorageClass |
| Insufficient resources | `kubectl describe pod` shows FailedScheduling | Reduce resource requests or add nodes |
| Node selector mismatch | `kubectl describe pod` shows NodeAffinity | Update nodeSelector or add matching labels to nodes |

### Pod in CrashLoopBackOff

**Symptoms:**
```
NAME             READY   STATUS             RESTARTS   AGE
my-sqlserver-0   0/2     CrashLoopBackOff   5          10m
```

**Causes & Solutions:**

| Cause | Diagnosis | Solution |
|-------|-----------|----------|
| Invalid password | Logs show password policy error | Use a strong password meeting requirements |
| Memory too low | Logs show memory allocation error | Set memory limit ≥ 2Gi |
| Permission denied | Logs show file permission errors | Check SecurityContext and PVC permissions |
| Image pull error | `kubectl describe pod` shows ImagePullBackOff | Check image name and imagePullSecrets |

**Check logs:**
```bash
kubectl logs my-sqlserver-0 -n mssql -c mssql-server --previous
```

### Pod Shows 1/2 Ready

**Symptoms:**
```
NAME             READY   STATUS    RESTARTS   AGE
my-sqlserver-0   1/2     Running   0          5m
```

**Causes:**
- SQL Exporter sidecar failing to connect
- AG Helper waiting for AG configuration

**Solutions:**
```bash
# Check which container is failing
kubectl describe pod my-sqlserver-0 -n mssql

# Check SQL Exporter logs
kubectl logs my-sqlserver-0 -n mssql -c sql-exporter

# Check AG Helper logs
kubectl logs my-sqlserver-0 -n mssql -c ag-helper
```

## Storage Issues

### PVC Stuck in Pending

**Symptoms:**
```bash
kubectl get pvc -n mssql
# NAME                    STATUS    VOLUME   CAPACITY   ACCESS MODES   STORAGECLASS   AGE
# my-sqlserver-data-0     Pending                                      fast-ssd       5m
```

**Causes & Solutions:**

| Cause | Diagnosis | Solution |
|-------|-----------|----------|
| StorageClass doesn't exist | `kubectl get sc` shows no fast-ssd | Use existing StorageClass or create one |
| No storage provisioner | No default StorageClass | Install CSI driver or use local-path |
| Insufficient capacity | Check provisioner logs | Add storage or reduce size |

**List available StorageClasses:**
```bash
kubectl get storageclass
```

### Storage Performance Issues

**Symptoms:**
- Slow query performance
- High disk latency in SQL Server

**Solutions:**
- Use SSD-backed StorageClass
- Separate data and log volumes
- Use dedicated tempdb volume

## Connection Issues

### Cannot Connect to SQL Server

**Diagnosis steps:**

```bash
# 1. Check pod is running
kubectl get pods -n mssql

# 2. Check service exists
kubectl get svc -n mssql

# 3. Test connectivity from within cluster
kubectl run test-sql --rm -it --image=mcr.microsoft.com/mssql-tools -- \
  /opt/mssql-tools/bin/sqlcmd -S my-sqlserver.mssql.svc.cluster.local -U sa -P 'password'

# 4. Port forward for external access
kubectl port-forward svc/my-sqlserver -n mssql 1433:1433
```

**Common causes:**

| Cause | Solution |
|-------|----------|
| Service not created | Check SQLServer status, describe resource |
| Wrong password | Verify secret contents |
| Network policy blocking | Check NetworkPolicy rules |
| SQL Server not ready | Wait for pod readiness |

### Login Failed for User 'sa'

**Causes:**
- Wrong password
- Password not meeting complexity requirements
- Secret key mismatch

**Solutions:**
```bash
# Verify secret exists and has correct key
kubectl get secret mssql-sa-password -n mssql -o yaml

# Check the key name matches saPasswordSecretRef.key
# Default is "password"
```

## AG Helper Issues

### Health: "Waiting" Indefinitely

**Symptoms:**
```bash
kubectl exec -it my-sqlserver-0 -n mssql -c ag-helper -- curl localhost:8080/state
# {"health":"Waiting","role":"NOT_AVAILABLE",...}
```

**Cause:** Availability Group hasn't been configured via T-SQL.

**Solution:** Run the AG setup scripts. See [AG Deployment Guide](../availability-groups/deployment-guide.md).

### Health: "Critical"

**Symptoms:**
```bash
kubectl exec -it my-sqlserver-0 -n mssql -c ag-helper -- curl localhost:8080/state
# {"health":"Critical","error":"connection refused",...}
```

**Causes & Solutions:**

| Cause | Solution |
|-------|----------|
| SQL Server not running | Check mssql-server container |
| HADR not enabled | Set `hadrEnabled: true` in spec |
| AG endpoint not created | Run endpoint creation T-SQL |
| Certificate authentication failed | Verify certificates between replicas |

### Readiness Probe Failing

**Symptoms:**
- Pod shows Running but service doesn't route traffic
- `kubectl describe pod` shows readiness probe failures

**Solutions:**
```bash
# Check readiness endpoint directly
kubectl exec -it my-sqlserver-0 -n mssql -c ag-helper -- curl localhost:8080/readyz

# Check AG synchronization status
kubectl exec -it my-sqlserver-0 -n mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P 'password' -C \
  -Q "SELECT synchronization_health_desc FROM sys.dm_hadr_availability_replica_states"
```

## Validation Errors

### StorageClass Not Found

```
Error: StorageClass 'managed-premium' not found in cluster.
Available StorageClasses: [standard, local-path].
```

**Solution:**
```bash
# List available StorageClasses
kubectl get storageclass

# Update manifest to use available class
# Or remove storageClass to use default
```

### Password Complexity

```
Error: Password does not meet SQL Server complexity requirements
```

**Solution:** Use a password with:
- At least 8 characters
- Characters from 3 of 4 categories: uppercase, lowercase, digits, special

### Name Too Long

```
Error: Name 'my-production-sql-server' exceeds maximum length of 13 characters
```

**Solution:** Use a shorter name (max 13 chars):
```yaml
metadata:
  name: sql-prod-01  # 11 characters
```

## Monitoring Issues

### SQL Exporter Not Starting

**Check logs:**
```bash
kubectl logs my-sqlserver-0 -n mssql -c sql-exporter
```

**Common causes:**

| Error | Solution |
|-------|----------|
| Connection refused | SQL Server still starting, wait |
| Login failed | Check exporter uses correct credentials |
| Config file not found | Verify ConfigMap mounted correctly |

### Prometheus Not Scraping

**Verify ServiceMonitor:**
```bash
kubectl get servicemonitor -n mssql

# Check Prometheus targets
# In Prometheus UI: Status > Targets
```

**Common causes:**
- ServiceMonitor not matching labels
- Prometheus not watching namespace
- Network policy blocking port 9399

## Debugging Commands

### Quick Status Check

```bash
# All resources
kubectl get all -n mssql

# SQLServer resources
kubectl get sqlserver,sqlserverag -n mssql

# Detailed status
kubectl describe sqlserver my-sqlserver -n mssql
```

### Log Collection

```bash
# SQL Server logs
kubectl logs my-sqlserver-0 -n mssql -c mssql-server

# AG Helper logs
kubectl logs my-sqlserver-0 -n mssql -c ag-helper

# SQL Exporter logs
kubectl logs my-sqlserver-0 -n mssql -c sql-exporter

# Previous container logs (after crash)
kubectl logs my-sqlserver-0 -n mssql -c mssql-server --previous
```

### Interactive Debugging

```bash
# Exec into SQL Server container
kubectl exec -it my-sqlserver-0 -n mssql -c mssql-server -- /bin/bash

# Run sqlcmd
kubectl exec -it my-sqlserver-0 -n mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P 'password' -C

# Check AG Helper endpoints
kubectl exec -it my-sqlserver-0 -n mssql -c ag-helper -- curl localhost:8080/state
```

### Event History

```bash
# Events for specific resource
kubectl get events -n mssql --field-selector involvedObject.name=my-sqlserver

# All recent events
kubectl get events -n mssql --sort-by='.lastTimestamp'
```

## Next Steps

- [AG Helper Reference](../availability-groups/ag-helper-reference.md) - Sidecar debugging
- [Monitoring Overview](../monitoring/overview.md) - Metrics troubleshooting
- [Architecture](../architecture/overview.md) - Understanding internals
