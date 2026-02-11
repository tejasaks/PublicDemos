# Multi-AG Scenarios

[← Back to Availability Groups](overview.md) | [Documentation Home](../README.md)

Guide to deploying and managing multiple Availability Groups on the same SQL Server cluster.

## Table of Contents

- [Overview](#overview)
- [Multiple AG Architecture](#multiple-ag-architecture)
- [Deploying Multiple AGs](#deploying-multiple-ags)
- [Adding a New AG to Existing Cluster](#adding-a-new-ag-to-existing-cluster)
- [Independent vs Shared Replicas](#independent-vs-shared-replicas)
- [Traffic Routing](#traffic-routing)
- [Monitoring Multiple AGs](#monitoring-multiple-ags)

## Overview

Multiple Availability Groups allow you to:

- **Separate workloads**: Different databases in different AGs
- **Independent failover**: Each AG fails over independently
- **Different SLAs**: Some AGs sync, others async
- **Resource isolation**: Route to different services

### Use Cases

| Scenario | Description |
|----------|-------------|
| Multi-tenant | Each tenant's databases in separate AG |
| Mixed workloads | OLTP in sync AG, analytics in async AG |
| Phased migration | Migrate databases gradually |
| DR tiers | Critical dbs sync, others async |

## Multiple AG Architecture

```
┌──────────────────────────────────────────────────────────────────────┐
│                    SQL Server Instance (Pod 0)                        │
│                                                                       │
│  ┌────────────────────┐    ┌────────────────────┐                    │
│  │  AG: ProductionAG   │    │  AG: AnalyticsAG   │                    │
│  │                     │    │                     │                    │
│  │  Databases:         │    │  Databases:         │                    │
│  │  - Orders           │    │  - Analytics        │                    │
│  │  - Inventory        │    │  - Reports          │                    │
│  │                     │    │                     │                    │
│  │  Mode: Sync         │    │  Mode: Async        │                    │
│  │  Failover: Auto     │    │  Failover: Manual   │                    │
│  │  Endpoint: 5022     │    │  Endpoint: 5023     │                    │
│  └────────────────────┘    └────────────────────┘                    │
│                                                                       │
└──────────────────────────────────────────────────────────────────────┘
                │                            │
                ▼                            ▼
     ┌──────────────────┐         ┌──────────────────┐
     │ prod-ag-primary  │         │ analytics-primary│
     │ :1433            │         │ :2433            │
     └──────────────────┘         └──────────────────┘
```

## Deploying Multiple AGs

### Architecture: One SQLServerAG per AG

> **Important:** Each SQLServerAG resource manages exactly **one** Availability Group.
> For multiple AGs, create multiple SQLServerAG resources (one per AG).
> Each AG Helper sidecar monitors the specific AG named in its SQLServerAG resource.

### Step 1: Single SQLServer Resource

Both AGs share the same SQL Server instances:

```yaml
apiVersion: mssql.microsoft.com/v1alpha1
kind: SQLServer
metadata:
  name: multiag
  namespace: mssql
spec:
  version: \"2025\"
  edition: Enterprise  # Enterprise required for multiple AGs
  instance:
    count: 3
    config:
      hadrEnabled: true
  credentials:
    saPasswordSecretRef:
      name: multiag-sa
      key: password
```

### Step 2: First AG (Production)

```yaml
apiVersion: mssql.microsoft.com/v1alpha1
kind: SQLServerAG
metadata:
  name: production-ag
  namespace: mssql
spec:
  description: "Production transactional AG"
  sqlServerRef:
    name: multiag
  availabilityGroup:
    name: ProductionAG
    instanceCount: 3
    primaryConfig:
      availabilityMode: SynchronousCommit
      failoverMode: External
    seedingMode: Automatic
    databases:
      - name: Orders
      - name: Inventory
    automaticFailover: true  # Opt-in: enable controller failover
    endpointPort: 5022
  endpoints:
    primary:
      type: LoadBalancer
      port: 1433
    secondary:
      type: LoadBalancer
      port: 1434
```

### Step 3: Second AG (Analytics)

```yaml
apiVersion: mssql.microsoft.com/v1alpha1
kind: SQLServerAG
metadata:
  name: analytics-ag
  namespace: mssql
spec:
  description: "Analytics read-heavy AG"
  sqlServerRef:
    name: multiag  # Same SQL Server!
  availabilityGroup:
    name: AnalyticsAG
    instanceCount: 3
    primaryConfig:
      availabilityMode: AsynchronousCommit  # Async for analytics
      failoverMode: Manual
    seedingMode: Automatic
    databases:
      - name: Analytics
      - name: Reports
    automaticFailover: false  # Manual failover only
    endpointPort: 5023  # Different port!
  endpoints:
    primary:
      type: LoadBalancer
      port: 2433  # Different port!
    secondary:
      type: LoadBalancer
      port: 2434
```

### Step 4: Create AGs via T-SQL

```sql
-- Create first AG (ProductionAG) - same as single AG setup
CREATE AVAILABILITY GROUP ProductionAG
    WITH (CLUSTER_TYPE = EXTERNAL)
    FOR DATABASE Orders, Inventory
    REPLICA ON ...

-- Create second AG (AnalyticsAG)
CREATE AVAILABILITY GROUP AnalyticsAG
    WITH (CLUSTER_TYPE = EXTERNAL)
    FOR DATABASE Analytics, Reports
    REPLICA ON
        N'multiag-0' WITH (
            ENDPOINT_URL = N'TCP://multiag-0.multiag-pods.mssql.svc:5023',
            AVAILABILITY_MODE = ASYNCHRONOUS_COMMIT,
            FAILOVER_MODE = MANUAL
        ),
        N'multiag-1' WITH (
            ENDPOINT_URL = N'TCP://multiag-1.multiag-pods.mssql.svc:5023',
            AVAILABILITY_MODE = ASYNCHRONOUS_COMMIT,
            FAILOVER_MODE = MANUAL
        ),
        N'multiag-2' WITH (
            ENDPOINT_URL = N'TCP://multiag-2.multiag-pods.mssql.svc:5023',
            AVAILABILITY_MODE = ASYNCHRONOUS_COMMIT,
            FAILOVER_MODE = MANUAL
        );
GO
```

## Adding a New AG to Existing Cluster

### Step 1: Verify Existing AG

```bash
kubectl exec -it multiag-0 -n mssql -c ag-helper -- \
  curl -s localhost:8080/state | jq '.availabilityGroups'
```

### Step 2: Create New Databases

```sql
CREATE DATABASE NewAppDB;
GO
ALTER DATABASE NewAppDB SET RECOVERY FULL;
GO
BACKUP DATABASE NewAppDB TO DISK = '/var/opt/mssql/backup/NewAppDB.bak';
GO
```

### Step 3: Create New Endpoint

Each AG needs its own mirroring endpoint:

```sql
CREATE ENDPOINT NewAG_Endpoint
    STATE = STARTED
    AS TCP (LISTENER_PORT = 5024)
    FOR DATABASE_MIRRORING (
        ROLE = ALL,
        AUTHENTICATION = CERTIFICATE AG_Cert,
        ENCRYPTION = REQUIRED ALGORITHM AES
    );
GO
```

### Step 4: Deploy SQLServerAG Resource

```yaml
apiVersion: mssql.microsoft.com/v1alpha1
kind: SQLServerAG
metadata:
  name: new-ag
  namespace: mssql
spec:
  sqlServerRef:
    name: multiag
  availabilityGroup:
    name: NewAG
    endpointPort: 5024
    databases:
      - name: NewAppDB
```

### Step 5: Create and Join AG

```sql
-- On primary
CREATE AVAILABILITY GROUP NewAG
    WITH (CLUSTER_TYPE = EXTERNAL)
    FOR DATABASE NewAppDB
    REPLICA ON ...

-- On secondaries
ALTER AVAILABILITY GROUP NewAG JOIN WITH (CLUSTER_TYPE = EXTERNAL);
ALTER AVAILABILITY GROUP NewAG GRANT CREATE ANY DATABASE;
```

## Independent vs Shared Replicas

### Independent Primary/Secondary

Each AG can have different primary replicas:

```
Instance 0: ProductionAG=PRIMARY, AnalyticsAG=SECONDARY
Instance 1: ProductionAG=SECONDARY, AnalyticsAG=PRIMARY  
Instance 2: ProductionAG=SECONDARY, AnalyticsAG=SECONDARY
```

This allows:
- Load distribution across instances
- Independent failover
- Workload isolation

### Checking Roles

```bash
# Check which instance is primary for each AG
for i in 0 1 2; do
  echo "Instance $i:"
  kubectl exec -it multiag-$i -n mssql -c ag-helper -- \
    curl -s localhost:8080/state | jq -c '.availabilityGroups[] | {name, role}'
done

# Output:
# Instance 0:
# {"name":"ProductionAG","role":"PRIMARY"}
# {"name":"AnalyticsAG","role":"SECONDARY"}
# Instance 1:
# {"name":"ProductionAG","role":"SECONDARY"}
# {"name":"AnalyticsAG","role":"PRIMARY"}
```

## Traffic Routing

### Multiple Services

Each AG gets its own services:

```bash
kubectl get svc -n mssql

# NAME                     TYPE           PORT(S)
# production-ag-primary    LoadBalancer   1433
# production-ag-secondary  LoadBalancer   1434
# analytics-ag-primary     LoadBalancer   2433
# analytics-ag-secondary   LoadBalancer   2434
```

### Connection Strings

```
# Production databases (Orders, Inventory)
Server=production-ag-primary.mssql.svc.cluster.local,1433;

# Analytics databases (Analytics, Reports)  
Server=analytics-ag-primary.mssql.svc.cluster.local,2433;

# Read-only analytics
Server=analytics-ag-secondary.mssql.svc.cluster.local,2434;
ApplicationIntent=ReadOnly;
```

### Role Labels

Each pod gets labels for EACH AG:

```yaml
labels:
  mssql.microsoft.com/ProductionAG-role: primary
  mssql.microsoft.com/AnalyticsAG-role: secondary
```

Services use label selectors:

```yaml
# production-ag-primary service
selector:
  mssql.microsoft.com/ProductionAG-role: primary

# analytics-ag-primary service
selector:
  mssql.microsoft.com/AnalyticsAG-role: primary
```

## Monitoring Multiple AGs

### AG Helper Multi-AG Support

The AG Helper automatically discovers all AGs:

```bash
kubectl exec -it multiag-0 -n mssql -c ag-helper -- \
  curl -s localhost:8080/state | jq

# Response includes all AGs
{
  "instanceName": "multiag-0",
  "health": "Healthy",
  "availabilityGroups": [
    {
      "name": "ProductionAG",
      "role": "PRIMARY",
      "syncState": "SYNCHRONIZED",
      "databases": ["Orders", "Inventory"]
    },
    {
      "name": "AnalyticsAG",
      "role": "SECONDARY",
      "syncState": "SYNCHRONIZING",
      "databases": ["Analytics", "Reports"]
    }
  ]
}
```

### Prometheus Metrics

Metrics are exported per AG:

```
# Primary status (1 = primary, 0 = secondary)
mssql_ag_is_primary{ag_name="ProductionAG"} 1
mssql_ag_is_primary{ag_name="AnalyticsAG"} 0

# Sync state per database
mssql_ag_database_synchronization_state{ag_name="ProductionAG",database="Orders"} 2
mssql_ag_database_synchronization_state{ag_name="AnalyticsAG",database="Analytics"} 1
```

### Alerting

Configure separate alerts per AG:

```yaml
groups:
  - name: multi-ag-alerts
    rules:
      - alert: ProductionAGNotSynchronized
        expr: mssql_ag_database_synchronization_state{ag_name="ProductionAG"} != 2
        for: 5m
        labels:
          severity: critical
      
      - alert: AnalyticsAGNotSynchronizing
        expr: mssql_ag_database_synchronization_state{ag_name="AnalyticsAG"} == 0
        for: 15m
        labels:
          severity: warning  # Less critical for async AG
```

## Next Steps

- [AG Helper Reference](ag-helper-reference.md) - Multi-AG API details
- [Monitoring Overview](../monitoring/overview.md) - Prometheus setup
- [Troubleshooting](../user-guide/troubleshooting.md) - Multi-AG issues
