# Availability Groups Overview

[← Back to Documentation](../README.md)

SQL Server Availability Groups (AGs) provide high availability and disaster recovery for your databases running in Kubernetes.

> **New to AGs?** Start with the [Step-by-Step Tutorial](tutorial-ag-deployment.md) for a complete walkthrough from operator installation to working listener.

## Table of Contents

- [What is an Availability Group?](#what-is-an-availability-group)
- [Architecture](#architecture)
- [AG Helper Sidecar](#ag-helper-sidecar)
- [Health States](#health-states)
- [Traffic Routing](#traffic-routing)
- [AG Listener](listener-configuration.md) (detailed configuration)
- [When to Use AGs](#when-to-use-ags)

## What is an Availability Group?

An Availability Group is a SQL Server feature that enables:

- **High Availability**: Automatic failover when primary fails
- **Read Scale-Out**: Offload read workloads to secondaries
- **Zero Data Loss**: Synchronous commit between replicas
- **Disaster Recovery**: Replicas in different zones/regions

### Key Concepts

| Term | Description |
|------|-------------|
| **Primary Replica** | Handles all read-write operations |
| **Secondary Replica** | Receives changes from primary, can serve read-only queries |
| **Synchronous Commit** | Primary waits for secondary acknowledgment |
| **Asynchronous Commit** | Primary doesn't wait (allows data loss) |
| **Automatic Failover** | Controller promotes secondary when primary fails |

## Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                    SQL Server Availability Group                     │
│                                                                      │
│  ┌─────────────────────────────────────────────────────────────┐    │
│  │ Pod 0 (PRIMARY)                                              │    │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐          │    │
│  │  │ SQL Server  │  │ AG Helper   │  │SQL Exporter │          │    │
│  │  │ :1433       │  │ :8080       │  │ :9399       │          │    │
│  │  │             │  │             │  │             │          │    │
│  │  │ ┌─────────┐ │  │ - Monitor   │  │ Metrics     │          │    │
│  │  │ │ AG DB   │ │  │ - Failover  │  │             │          │    │
│  │  │ │ (R/W)   │ │  │ - Probes    │  │             │          │    │
│  │  │ └─────────┘ │  │             │  │             │          │    │
│  │  └─────────────┘  └─────────────┘  └─────────────┘          │    │
│  └─────────────────────────────────────────────────────────────┘    │
│                           │                                          │
│                    Sync Commit                                       │
│                           │                                          │
│         ┌─────────────────┼─────────────────┐                       │
│         ▼                                   ▼                        │
│  ┌──────────────────────┐     ┌──────────────────────┐              │
│  │ Pod 1 (SECONDARY)    │     │ Pod 2 (SECONDARY)    │              │
│  │ Read-Only Queries    │     │ Read-Only Queries    │              │
│  └──────────────────────┘     └──────────────────────┘              │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

### Components

| Component | Purpose |
|-----------|---------|
| **SQLServer CR** | Deploys SQL Server pods with HADR enabled |
| **SQLServerAG CR** | Configures AG services and endpoints |
| **AG Helper Sidecar** | Monitors AG health, provides failover API |
| **Primary Service** | Routes traffic to current primary |
| **Secondary Service** | Routes traffic to readable secondaries |

## AG Helper Sidecar

The AG Helper sidecar runs alongside SQL Server in each pod and provides:

1. **Health Monitoring**: Queries AG status every 10 seconds
2. **Kubernetes Probes**: Liveness and readiness for K8s integration
3. **Failover API**: HTTP endpoint to trigger failover
4. **Explicit AG Monitoring**: Each AG Helper monitors one specified AG

### Container Ports

| Port | Purpose |
|------|---------|
| 8080 | HTTP API and health probes |

### Key Endpoints

| Endpoint | Purpose |
|----------|---------|
| `/healthz` | Liveness probe |
| `/readyz` | Readiness probe |
| `/state` | Full AG state (JSON) |
| `/role` | Current replica role |
| `/failover` | Trigger failover (POST) |

See [AG Helper Reference](ag-helper-reference.md) for complete API documentation, or [Controller Workflow Details](controller-workflow-details.md) for a deep dive into the AG Helper and Controller internals.

## Health States

The AG Helper reports one of four health states:

```
┌──────────────────────────────────────────────────────────────────┐
│                    Health State Machine                           │
├──────────────────────────────────────────────────────────────────┤
│                                                                   │
│  ┌──────────┐     AG detected      ┌──────────┐                  │
│  │ WAITING  │ ─────────────────────▶│ HEALTHY  │                  │
│  │          │                       │          │                  │
│  │ Liveness:│                       │ Liveness:│                  │
│  │   PASS   │                       │   PASS   │                  │
│  │ Readiness│                       │ Readiness│                  │
│  │   FAIL   │                       │   PASS   │                  │
│  └──────────┘                       └────┬─────┘                  │
│       ▲                                  │                        │
│       │                                  │ sync issues            │
│       │ AG removed                       ▼                        │
│       │                            ┌──────────┐                   │
│       └────────────────────────────│ WARNING  │                   │
│                                    │          │                   │
│                                    │ Liveness:│                   │
│                                    │   PASS   │                   │
│                                    │ Readiness│                   │
│                                    │   PASS   │                   │
│                                    └────┬─────┘                   │
│                                         │                         │
│                                         │ AG broken               │
│                                         ▼                         │
│                                    ┌──────────┐                   │
│                                    │ CRITICAL │                   │
│                                    │          │                   │
│                                    │ Liveness:│                   │
│                                    │   FAIL   │                   │
│                                    │ Readiness│                   │
│                                    │   FAIL   │                   │
│                                    └──────────┘                   │
│                                                                   │
└──────────────────────────────────────────────────────────────────┘
```

| State | Description | Liveness | Readiness |
|-------|-------------|----------|-----------|
| **Waiting** | AG not configured yet | ✅ Pass | ❌ Fail |
| **Healthy** | AG running, all synced | ✅ Pass | ✅ Pass |
| **Warning** | AG running, some syncing | ✅ Pass | ✅ Pass |
| **Critical** | AG broken or unreachable | ❌ Fail | ❌ Fail |

## Traffic Routing

### AG Listener (Recommended)

The AG Listener provides a single connection point that automatically routes to the current PRIMARY replica. This is the recommended approach for production workloads.

```yaml
apiVersion: mssql.microsoft.com/v1alpha1
kind: SQLServerAG
spec:
  listener:
    name: productionag-listener
    port: 1433
    serviceType: ClusterIP
```

The operator creates a Kubernetes Service and manages Endpoints to always point to the current primary. See [Listener Configuration](listener-configuration.md) for detailed setup instructions.

**Connection String:**
```
Server=productionag-listener.mssql.svc.cluster.local,1433;
ApplicationIntent=ReadWrite;
```

### Individual Replica Services

For direct access to specific replicas (e.g., read workloads on secondaries), use the per-replica services created by the SQLServer resource:

```bash
# Get individual replica services
kubectl get svc sql-ag-0 sql-ag-1 sql-ag-2 -n mssql

# Connect to a specific replica
sqlcmd -S <sql-ag-1-service-ip>,1433 -U sa -P 'password'
```

Use for: Reporting queries, analytics, backups, maintenance

### Connection Strings

```
# Via Listener (always routes to primary - RECOMMENDED)
Server=productionag-listener.mssql.svc.cluster.local,1433;
ApplicationIntent=ReadWrite;

# Direct to specific replica (for read-only workloads)
Server=sql-ag-1.mssql.svc.cluster.local,1433;
ApplicationIntent=ReadOnly;
```

## When to Use AGs

### Use AGs When

- ✅ Zero-downtime requirements
- ✅ Mission-critical applications
- ✅ Read-scale workloads
- ✅ Multi-zone deployment
- ✅ Compliance requiring HA

### Don't Use AGs When

- ❌ Development/testing (overhead)
- ❌ Single-region, non-critical apps
- ❌ Very small databases (simpler backup/restore)
- ❌ Limited resources (3 replicas needed)

## Next Steps

- [Deployment Guide](deployment-guide.md) - Step-by-step AG setup
- [Failover Management](failover-management.md) - Automatic and manual failover
- [Multi-AG Scenarios](multi-ag-scenarios.md) - Multiple AGs
- [AG Helper Reference](ag-helper-reference.md) - Complete API docs
- [Controller Workflow Details](controller-workflow-details.md) - Deep dive into AG Helper and Controller internals
