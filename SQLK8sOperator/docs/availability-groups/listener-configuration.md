# AG Listener Configuration

This document covers the configuration and operation of Availability Group (AG) Listeners in the SQL Server Kubernetes Operator.

## Overview

An AG Listener provides a single connection point that automatically routes to the current primary replica. In traditional Windows Server Failover Clustering (WSFC), the listener uses a Virtual Network Name (VNN) with a floating IP managed by the cluster.

In Kubernetes, we implement a similar concept using:
- **Kubernetes Service** (without selector) - Provides the VIP and DNS name
- **Kubernetes Endpoints** - Managed by the operator to route to the current primary

> **Note:** The listener is **OPTIONAL**. You can use SQLServerAG resources without a listener for DR scenarios, manual failover workflows, or direct replica access. See [When to Use a Listener](#when-to-use-a-listener) below.

## When to Use a Listener

| Scenario | Listener Needed? | Notes |
|----------|------------------|-------|
| **Production HA** | ✅ Yes | Single VIP for application connection strings |
| **Read-scale** | ✅ Yes | Listener for writes, direct replica access for reads |
| **DR (Disaster Recovery)** | ❌ No | Connect directly to DR site replicas |
| **Manual Failover** | ❌ No | DBA controls failover, connects to specific replicas |
| **Development/Testing** | ❌ No | Direct replica access is simpler |
| **Multi-site with DNS** | ✅ Yes | Use with external-dns for cross-site routing |

### Without Listener

When no listener is configured, connect to individual replicas using their services:

```bash
# Get individual replica services
kubectl get svc sql-ag-0 sql-ag-1 sql-ag-2 -n mssql

# Find which replica is primary
kubectl get sqlserverag production-ag -n mssql -o jsonpath='{.status.primaryReplica}'

# Connect to specific replica
sqlcmd -S <sql-ag-0-service-ip>,1433 -U sa -P 'password'
```

See [sql-ag-dr/](../../samples/sql-ag-dr/) for a DR/manual failover example without listener.

## Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                        Client Application                            │
│                    sqlcmd -S productionag-listener,1433              │
└──────────────────────────────┬──────────────────────────────────────┘
                               │
                               ▼
┌─────────────────────────────────────────────────────────────────────┐
│              Kubernetes Service (productionag-listener)              │
│                    ClusterIP: 10.96.100.50                           │
│                    Port: 1433                                        │
│                    Selector: NONE (operator-managed)                 │
└──────────────────────────────┬──────────────────────────────────────┘
                               │
                               ▼
┌─────────────────────────────────────────────────────────────────────┐
│                    Kubernetes Endpoints                              │
│                    productionag-listener                             │
│                    Addresses: [10.244.0.5:1433]  ← Primary pod IP    │
└──────────────────────────────┬──────────────────────────────────────┘
                               │
                ┌──────────────┴──────────────┐
                ▼                              ▼
        ┌───────────────┐             ┌───────────────┐
        │   sql-ag-0    │             │   sql-ag-1    │
        │   PRIMARY     │ ◄────────── │   SECONDARY   │
        │  10.244.0.5   │             │  10.244.0.6   │
        └───────────────┘             └───────────────┘
```

## Listener Phases

| Phase | Description |
|-------|-------------|
| **Pending** | Service is being created, waiting for VIP assignment |
| **WaitingForListener** | VIP is available. Create the AG Listener in SQL Server using T-SQL |
| **Ready** | Listener is configured and routing traffic to the primary replica |
| **Degraded** | Listener exists but no primary is available (e.g., during failover) |
| **Maintenance** | Listener is in maintenance mode (set via annotation) |

## Configuration

### Basic Listener Configuration

```yaml
apiVersion: mssql.microsoft.com/v1alpha1
kind: SQLServerAG
metadata:
  name: production-ag
  namespace: mssql
spec:
  sqlServerRef:
    name: sql-ag
  availabilityGroup:
    name: ProductionAG
    instanceCount: 3
    # ... other AG config ...

  listener:
    name: productionag-listener
    port: 1433
    serviceType: ClusterIP
```

### Advanced Configuration

```yaml
listener:
  # Service name (DNS-compatible)
  name: productionag-listener
  
  # Listener port (default: 1433)
  # Use non-default ports for security or multi-instance scenarios
  port: 14333
  
  # Service type: ClusterIP, LoadBalancer, or NodePort
  serviceType: LoadBalancer
  
  # Static ClusterIP (optional, must be in cluster's service CIDR)
  clusterIP: "10.96.100.50"
  
  # Static LoadBalancer IP (optional, cloud-provider specific)
  loadBalancerIP: "20.50.100.25"
  
  # Custom annotations for cloud-specific features
  annotations:
    service.beta.kubernetes.io/azure-load-balancer-internal: "true"
    external-dns.alpha.kubernetes.io/hostname: "productionag.example.com"
```

## Deployment Workflow

### Step 1: Apply the SQLServerAG Resource

```bash
# AG resources are included in the unified ag-deploy.yaml
kubectl apply -f samples/sql-ag-ha/ag-deploy.yaml
```

### Step 2: Wait for VIP Assignment

```bash
# Check listener status
kubectl get sqlserverag production-ag -n mssql -o jsonpath='{.status.listener.phase}'
# Should show: WaitingForListener

# Get the assigned VIP
kubectl get sqlserverag production-ag -n mssql -o jsonpath='{.status.listener.vip}'
# Example output: 10.96.100.50
```

### Step 3: Get the Subnet Mask

The subnet mask depends on your cluster's service CIDR configuration:

```bash
# Method 1: From kubeadm config
kubectl get cm kubeadm-config -n kube-system -o jsonpath='{.data.ClusterConfiguration}' | grep serviceSubnet

# Method 2: From kube-apiserver flags
kubectl get pod -n kube-system -l component=kube-apiserver -o yaml | grep service-cluster-ip-range

# Method 3: Check existing services to infer CIDR
kubectl get svc -A -o jsonpath='{.items[*].spec.clusterIP}' | tr ' ' '\n' | sort -u | head -5
```

**Common CIDR to Subnet Mask:**

| CIDR | Subnet Mask | Example |
|------|-------------|--------|
| /12 | 255.240.0.0 | 10.96.0.0/12 |
| /16 | 255.255.0.0 | 10.0.0.0/16 |
| /20 | 255.255.240.0 | 10.96.0.0/20 |
| /24 | 255.255.255.0 | 10.96.0.0/24 |

### Step 4: Create AG Listener in SQL Server

Connect to the PRIMARY replica and run:

```sql
-- Replace <VIP> with the value from Step 2
-- Replace <SUBNET_MASK> with the value from Step 3
ALTER AVAILABILITY GROUP [ProductionAG]
ADD LISTENER 'productionag-listener' (
    WITH IP (('<VIP>', '<SUBNET_MASK>')),
    PORT = 1433
);

-- Verify listener was created
SELECT * FROM sys.availability_group_listeners;
```

### Step 5: Verify Listener is Ready

```bash
# Check phase is now Ready
kubectl get sqlserverag production-ag -n mssql -o jsonpath='{.status.listener.phase}'
# Should show: Ready

# View full listener status
kubectl get sqlserverag production-ag -n mssql -o jsonpath='{.status.listener}' | jq
```

### Step 6: Connect via Listener

```bash
# Get the VIP
VIP=$(kubectl get sqlserverag production-ag -n mssql -o jsonpath='{.status.listener.vip}')

# Connect using sqlcmd
sqlcmd -S $VIP,1433 -U sa -P 'YourStrong@Passw0rd!'

# Or for LoadBalancer type, use the external IP
EXTERNAL_IP=$(kubectl get sqlserverag production-ag -n mssql -o jsonpath='{.status.listener.externalIP}')
sqlcmd -S $EXTERNAL_IP,1433 -U sa -P 'YourStrong@Passw0rd!'
```

## Service Types

### ClusterIP (Default)

Internal cluster access only. Best for applications running within the same Kubernetes cluster.

```yaml
listener:
  name: productionag-listener
  serviceType: ClusterIP
```

### LoadBalancer

External access via cloud load balancer. Best for external applications or hybrid scenarios.

```yaml
listener:
  name: productionag-listener
  serviceType: LoadBalancer
  # Optional: Request specific external IP
  loadBalancerIP: "20.50.100.25"
  # Optional: Use internal load balancer (Azure example)
  annotations:
    service.beta.kubernetes.io/azure-load-balancer-internal: "true"
```

### NodePort

Exposes on each node's IP at a static port. Generally not recommended for production.

```yaml
listener:
  name: productionag-listener
  serviceType: NodePort
```

## Non-Default Ports

For security or multi-instance scenarios, use a non-default port:

```yaml
listener:
  name: productionag-listener
  port: 14333  # Non-default port
```

**Important:** The port in the `listener` spec must match the port used in the T-SQL `ADD LISTENER` command. Use the subnet mask appropriate for your cluster's service CIDR (see [Step 3](#step-3-get-the-subnet-mask) above).

```sql
-- Match the port from the listener spec
-- Replace <SUBNET_MASK> based on your cluster's service CIDR
ALTER AVAILABILITY GROUP [ProductionAG]
ADD LISTENER 'productionag-listener' (
    WITH IP (('<VIP>', '<SUBNET_MASK>')),
    PORT = 14333
);
```

## Maintenance Mode

Put the listener in maintenance mode to suppress warnings during planned maintenance:

```bash
# Enter maintenance mode
kubectl annotate sqlserverag production-ag -n mssql \
  mssql.microsoft.com/listener-maintenance=true

# Check phase
kubectl get sqlserverag production-ag -n mssql -o jsonpath='{.status.listener.phase}'
# Shows: Maintenance

# Exit maintenance mode
kubectl annotate sqlserverag production-ag -n mssql \
  mssql.microsoft.com/listener-maintenance-
```

## Monitoring and Troubleshooting

### Check Listener Status

```bash
# Full listener status
kubectl get sqlserverag production-ag -n mssql -o jsonpath='{.status.listener}' | jq

# Listener phase
kubectl get sqlserverag production-ag -n mssql -o jsonpath='{.status.listener.phase}'

# Listener VIP
kubectl get sqlserverag production-ag -n mssql -o jsonpath='{.status.listener.vip}'

# Current primary (where traffic is routed)
kubectl get sqlserverag production-ag -n mssql -o jsonpath='{.status.listener.currentPrimary}'
```

### Check Kubernetes Resources

```bash
# Listener Service
kubectl get svc productionag-listener -n mssql

# Listener Endpoints (should show primary pod IP)
kubectl get endpoints productionag-listener -n mssql -o yaml

# Events related to listener
kubectl get events -n mssql --field-selector involvedObject.name=production-ag
```

### Common Issues

#### Listener stuck in "WaitingForListener"

**Cause:** AG Listener not created in SQL Server via T-SQL.

**Solution:**
1. Get the VIP: `kubectl get sqlserverag production-ag -n mssql -o jsonpath='{.status.listener.vip}'`
2. Create the listener in SQL Server using T-SQL

#### Listener shows "Degraded"

**Cause:** No primary replica available.

**Possible causes:**
- AG is in the middle of a failover
- All replicas are unavailable
- Network connectivity issues

**Solution:**
1. Check primary replica: `kubectl get sqlserverag production-ag -n mssql -o jsonpath='{.status.primaryReplica}'`
2. Check AG health: `kubectl describe sqlserverag production-ag -n mssql`
3. Check pod status: `kubectl get pods -n mssql`

#### Connections fail through listener

**Cause:** Endpoints not pointing to the correct primary.

**Solution:**
1. Check Endpoints: `kubectl get endpoints productionag-listener -n mssql`
2. Verify primary pod IP matches: `kubectl get pod <primary-pod> -n mssql -o jsonpath='{.status.podIP}'`
3. Force reconcile: `kubectl annotate sqlserverag production-ag -n mssql force-reconcile=$(date +%s)`

#### Listener Service not created

**Cause:** Validation error in listener configuration.

**Solution:**
1. Check events: `kubectl get events -n mssql --sort-by='.lastTimestamp'`
2. Check operator logs: `kubectl logs deployment/mssql-operator -n mssql-system --tail=100`
3. Validate listener name is DNS-compatible (lowercase, alphanumeric, hyphens)

## Failover Behavior

When a failover occurs (automatic or manual):

1. Operator detects new primary replica
2. Updates Endpoints to point to new primary's IP
3. Listener continues to work with minimal disruption
4. Existing connections may be dropped (client retry required)

```bash
# Watch listener updates during failover
kubectl get endpoints productionag-listener -n mssql -w

# View failover events
kubectl get events -n mssql --field-selector reason=ListenerEndpointsUpdated
```

## Best Practices

1. **Use ClusterIP for internal applications** - Lower latency, no external exposure
2. **Use LoadBalancer for external access** - Consider internal load balancers for security
3. **Use non-default ports in production** - Adds a layer of obscurity
4. **Monitor listener phase** - Set up alerts for Degraded state
5. **Test failover regularly** - Ensure listener updates work correctly
6. **Use connection retry logic** - Clients should retry on connection failure

## Related Documentation

- [AG Operations Guide](../operations/ag-operations.md) - Quick reference kubectl commands
- [AG Deployment Guide](deployment-guide.md)
- [Failover Management](failover-management.md)
- [AG Helper Reference](ag-helper-reference.md)
- [Controller Workflow Details](controller-workflow-details.md)
