# Availability Group Operations Guide

[← Back to Documentation](../README.md) | [Failover Management](../availability-groups/failover-management.md)

This guide provides quick reference commands for day-to-day AG operations using kubectl.

## Table of Contents

- [Status and Monitoring](#status-and-monitoring)
- [Manual Failover](#manual-failover)
- [Listener Operations](#listener-operations)
- [Maintenance Mode](#maintenance-mode)
- [Troubleshooting](#troubleshooting)
- [Common Issues](#common-issues)

---

## Status and Monitoring

### View AG Status

```bash
# List all SQLServerAG resources
kubectl get sqlserverag -n mssql

# With wide output (shows listener phase and VIP)
kubectl get sqlserverag -n mssql -o wide

# Detailed status
kubectl describe sqlserverag production-ag -n mssql
```

### Check Primary Replica

```bash
# Get current primary replica name
kubectl get sqlserverag production-ag -n mssql -o jsonpath='{.status.primaryReplica}'

# With formatting
kubectl get sqlserverag production-ag -n mssql \
  -o jsonpath='Primary: {.status.primaryReplica}{"\n"}'
```

### Check Synchronization

```bash
# Get synchronized instance count
kubectl get sqlserverag production-ag -n mssql -o jsonpath='{.status.synchronizedInstances}'

# View all instance statuses
kubectl get sqlserverag production-ag -n mssql -o jsonpath='{.status.instances}' | jq

# One-liner instance summary
kubectl get sqlserverag production-ag -n mssql \
  -o jsonpath='Instances: {range .status.instances[*]}{.name}={.role} ({.synchronizationState}), {end}{"\n"}'
```

### View AG Phase

```bash
# Current phase: Pending, Creating, Synchronized, Degraded, Failed
kubectl get sqlserverag production-ag -n mssql -o jsonpath='{.status.phase}'
```

### View Events

```bash
# Events for specific AG
kubectl get events -n mssql --field-selector involvedObject.name=production-ag

# Recent events sorted by time
kubectl get events -n mssql --sort-by='.lastTimestamp' | tail -20

# Filter by reason (e.g., failover events)
kubectl get events -n mssql --field-selector reason=FailoverCompleted
kubectl get events -n mssql --field-selector reason=ListenerEndpointsUpdated
```

### Watch for Changes

```bash
# Watch AG status in real-time
kubectl get sqlserverag production-ag -n mssql -w

# Watch with wide output
kubectl get sqlserverag production-ag -n mssql -o wide -w

# Watch events
kubectl get events -n mssql -w
```

---

## Manual Failover

### Trigger Failover via Annotation

The operator supports manual failover via kubectl annotations:

```bash
# Failover to sql-ag-1
kubectl annotate sqlserverag production-ag -n mssql \
  mssql.microsoft.com/failover-to=sql-ag-1

# Failover to sql-ag-2
kubectl annotate sqlserverag production-ag -n mssql \
  mssql.microsoft.com/failover-to=sql-ag-2
```

### Monitor Failover Progress

```bash
# Check failover status annotation
kubectl get sqlserverag production-ag -n mssql \
  -o jsonpath='{.metadata.annotations.mssql\.microsoft\.com/failover-status}'

# Watch for primary change
kubectl get sqlserverag production-ag -n mssql -w

# View failover events
kubectl get events -n mssql --field-selector reason=FailoverCompleted
```

### Cancel/Clear Failed Failover

If a failover fails or needs to be cancelled:

```bash
kubectl annotate sqlserverag production-ag -n mssql \
  mssql.microsoft.com/failover-to- \
  mssql.microsoft.com/failover-status- \
  mssql.microsoft.com/failover-requested-
```

### Trigger Failover via AG Helper API

Alternative method using the AG Helper sidecar directly:

```bash
# Get current role (run on target replica)
kubectl exec -it sql-ag-1 -n mssql -c ag-helper -- \
  curl -s localhost:8080/role

# Check sync state before failover
kubectl exec -it sql-ag-1 -n mssql -c ag-helper -- \
  curl -s localhost:8080/state | jq '.syncState'

# Trigger failover (no data loss)
kubectl exec -it sql-ag-1 -n mssql -c ag-helper -- \
  curl -X POST localhost:8080/failover -d '{"allowDataLoss": false}'

# Force failover (allows data loss - use with caution!)
kubectl exec -it sql-ag-1 -n mssql -c ag-helper -- \
  curl -X POST localhost:8080/failover -d '{"allowDataLoss": true, "force": true}'
```

### Compare LSN Before Failover

To choose the best failover target:

```bash
# Check sequence number on all replicas
for i in 0 1 2; do
  echo "sql-ag-$i:"
  kubectl exec -it sql-ag-$i -n mssql -c ag-helper -- \
    curl -s localhost:8080/sequence
  echo ""
done
```

---

## Listener Operations

### Check Listener Status

```bash
# Full listener status (JSON)
kubectl get sqlserverag production-ag -n mssql -o jsonpath='{.status.listener}' | jq

# Listener phase: Pending, WaitingForListener, Ready, Degraded, Maintenance
kubectl get sqlserverag production-ag -n mssql -o jsonpath='{.status.listener.phase}'

# Listener VIP
kubectl get sqlserverag production-ag -n mssql -o jsonpath='{.status.listener.vip}'

# Current primary receiving traffic
kubectl get sqlserverag production-ag -n mssql -o jsonpath='{.status.listener.currentPrimary}'
```

### View Listener Service

```bash
# Get listener Service details
kubectl get svc productionag-listener -n mssql

# View Service YAML
kubectl get svc productionag-listener -n mssql -o yaml
```

### View Listener Endpoints

```bash
# Check Endpoints (should show primary pod IP)
kubectl get endpoints productionag-listener -n mssql

# Detailed Endpoints view
kubectl get endpoints productionag-listener -n mssql -o yaml

# Just the IP addresses
kubectl get endpoints productionag-listener -n mssql \
  -o jsonpath='{.subsets[*].addresses[*].ip}'
```

### Test Listener Connectivity

```bash
# Get the VIP
VIP=$(kubectl get sqlserverag production-ag -n mssql -o jsonpath='{.status.listener.vip}')
echo "Listener VIP: $VIP"

# Test connection from within cluster (using a debug pod)
kubectl run -it --rm sqltest --image=mcr.microsoft.com/mssql-tools -- \
  /opt/mssql-tools/bin/sqlcmd -S $VIP,1433 -U sa -P 'YourPassword' -Q "SELECT @@SERVERNAME"
```

---

## Maintenance Mode

### Listener Maintenance Mode

Put the listener in maintenance mode to suppress degraded warnings during planned maintenance:

```bash
# Enter maintenance mode
kubectl annotate sqlserverag production-ag -n mssql \
  mssql.microsoft.com/listener-maintenance=true

# Verify maintenance mode
kubectl get sqlserverag production-ag -n mssql -o jsonpath='{.status.listener.phase}'
# Should show: Maintenance

# Exit maintenance mode
kubectl annotate sqlserverag production-ag -n mssql \
  mssql.microsoft.com/listener-maintenance-
```

### Disable Automatic Failover for Maintenance

```bash
# Disable automatic failover
kubectl patch sqlserverag production-ag -n mssql --type merge \
  -p '{"spec":{"availabilityGroup":{"automaticFailover":false}}}'

# Perform maintenance...

# Re-enable automatic failover
kubectl patch sqlserverag production-ag -n mssql --type merge \
  -p '{"spec":{"availabilityGroup":{"automaticFailover":true}}}'
```

### Force Reconciliation

Trigger the operator to re-reconcile the AG:

```bash
# Add a timestamp annotation to trigger reconcile
kubectl annotate sqlserverag production-ag -n mssql \
  force-reconcile=$(date +%s)
```

---

## Troubleshooting

### Check Operator Logs

```bash
# Recent operator logs
kubectl logs deployment/mssql-operator -n mssql-system --tail=100

# Follow logs
kubectl logs deployment/mssql-operator -n mssql-system -f

# Filter for specific AG
kubectl logs deployment/mssql-operator -n mssql-system | grep production-ag
```

### Check AG Helper Logs

```bash
# AG Helper logs on specific replica
kubectl logs sql-ag-0 -n mssql -c ag-helper --tail=50

# Follow logs
kubectl logs sql-ag-0 -n mssql -c ag-helper -f
```

### Check Pod Status

```bash
# All SQL pods
kubectl get pods -n mssql -l app=mssql

# Pod details
kubectl describe pod sql-ag-0 -n mssql

# Pod conditions
kubectl get pod sql-ag-0 -n mssql -o jsonpath='{.status.conditions}' | jq
```

### Query AG Helper API

```bash
# Health check
kubectl exec -it sql-ag-0 -n mssql -c ag-helper -- curl -s localhost:8080/health

# Full AG state
kubectl exec -it sql-ag-0 -n mssql -c ag-helper -- curl -s localhost:8080/state | jq

# Role check
kubectl exec -it sql-ag-0 -n mssql -c ag-helper -- curl -s localhost:8080/role

# Listener info (if configured in SQL Server)
kubectl exec -it sql-ag-0 -n mssql -c ag-helper -- curl -s localhost:8080/listener | jq
```

### Verify SQLServer Resource

```bash
# Check SQLServer is Ready
kubectl get sqlserver sql-ag -n mssql -o jsonpath='{.status.ready}'
# Must return "true"

# List all SQLServer resources
kubectl get sqlserver -n mssql
```

---

## Common Issues

### Listener Stuck in "WaitingForListener"

The operator created the VIP Service but hasn't detected the AG Listener in SQL Server.

**Resolution:**
1. Get the VIP:
   ```bash
   kubectl get sqlserverag production-ag -n mssql -o jsonpath='{.status.listener.vip}'
   ```

2. Get the subnet mask (based on your cluster's service CIDR):
   ```bash
   # Check cluster service CIDR
   kubectl get cm kubeadm-config -n kube-system -o jsonpath='{.data.ClusterConfiguration}' | grep serviceSubnet
   # Or infer from existing services
   kubectl get svc -A -o jsonpath='{.items[*].spec.clusterIP}' | tr ' ' '\n' | sort -u | head -5
   ```
   
   Common mappings: `/12`=255.240.0.0, `/16`=255.255.0.0, `/24`=255.255.255.0

3. Create the listener in SQL Server:
   ```sql
   -- Replace <VIP> and <SUBNET_MASK> with actual values
   ALTER AVAILABILITY GROUP [ProductionAG]
   ADD LISTENER 'productionag-listener' (
       WITH IP (('<VIP>', '<SUBNET_MASK>')),
       PORT = 1433
   );
   ```

3. Verify in SQL Server:
   ```sql
   SELECT * FROM sys.availability_group_listeners;
   ```

### Listener Shows "Degraded"

The listener exists but no primary replica is available.

**Resolution:**
1. Check for primary:
   ```bash
   kubectl get sqlserverag production-ag -n mssql -o jsonpath='{.status.primaryReplica}'
   ```

2. If empty, check AG health and consider failover:
   ```bash
   kubectl describe sqlserverag production-ag -n mssql
   ```

### Connections Fail Through Listener

**Resolution:**
1. Check Endpoints point to correct primary:
   ```bash
   kubectl get endpoints productionag-listener -n mssql
   ```

2. Verify primary pod IP matches:
   ```bash
   PRIMARY=$(kubectl get sqlserverag production-ag -n mssql -o jsonpath='{.status.primaryReplica}')
   kubectl get pod $PRIMARY -n mssql -o jsonpath='{.status.podIP}'
   ```

3. Check primary pod is Ready:
   ```bash
   kubectl get pod $PRIMARY -n mssql
   ```

### Failover Not Happening

**Resolution:**
1. Check if automatic failover is enabled:
   ```bash
   kubectl get sqlserverag production-ag -n mssql \
     -o jsonpath='{.spec.availabilityGroup.automaticFailover}'
   ```

2. Check cooldown period (60s after last failover):
   ```bash
   kubectl get events -n mssql --field-selector reason=CooldownActive
   ```

3. Check for healthy secondaries:
   ```bash
   kubectl get sqlserverag production-ag -n mssql -o jsonpath='{.status.instances}' | jq
   ```

### Reconciliation Failing

**Resolution:**
1. Check operator logs:
   ```bash
   kubectl logs deployment/mssql-operator -n mssql-system --tail=100 | grep -i error
   ```

2. Check events:
   ```bash
   kubectl get events -n mssql --sort-by='.lastTimestamp' | tail -20
   ```

3. Force reconcile:
   ```bash
   kubectl annotate sqlserverag production-ag -n mssql force-reconcile=$(date +%s)
   ```

---

## Quick Reference Card

| Operation | Command |
|-----------|---------|
| List AGs | `kubectl get sqlserverag -n mssql` |
| Get primary | `kubectl get sqlserverag <ag> -n mssql -o jsonpath='{.status.primaryReplica}'` |
| Get sync count | `kubectl get sqlserverag <ag> -n mssql -o jsonpath='{.status.synchronizedInstances}'` |
| Failover to replica | `kubectl annotate sqlserverag <ag> -n mssql mssql.microsoft.com/failover-to=<pod>` |
| Get listener VIP | `kubectl get sqlserverag <ag> -n mssql -o jsonpath='{.status.listener.vip}'` |
| Listener maintenance on | `kubectl annotate sqlserverag <ag> -n mssql mssql.microsoft.com/listener-maintenance=true` |
| Listener maintenance off | `kubectl annotate sqlserverag <ag> -n mssql mssql.microsoft.com/listener-maintenance-` |
| Watch AG | `kubectl get sqlserverag <ag> -n mssql -w` |
| View events | `kubectl get events -n mssql --field-selector involvedObject.name=<ag>` |
| Operator logs | `kubectl logs deployment/mssql-operator -n mssql-system --tail=100` |

---

## Related Documentation

- [Failover Management](../availability-groups/failover-management.md) - Detailed failover configuration
- [Listener Configuration](../availability-groups/listener-configuration.md) - Listener setup and phases
- [AG Helper Reference](../availability-groups/ag-helper-reference.md) - Complete API reference
- [Deployment Guide](../availability-groups/deployment-guide.md) - Initial AG setup
