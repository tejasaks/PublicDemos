# Availability Groups Overview

[← Back to Documentation](../README.md)

SQL Server Availability Groups (AGs) provide high availability and disaster recovery for your databases running in Kubernetes.

## Table of Contents

- [What is an Availability Group?](#what-is-an-availability-group)
- [Architecture](#architecture)
- [AG Helper Sidecar](#ag-helper-sidecar)
- [Health States](#health-states)
- [Traffic Routing](#traffic-routing)
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
4. **Multi-AG Support**: Discover and monitor all AGs automatically

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

### Primary Service

Routes all traffic to the current PRIMARY replica:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: prod-ag-primary
spec:
  selector:
    mssql.microsoft.com/ag-role: primary
  ports:
    - port: 1433
```

Use for: INSERT, UPDATE, DELETE, transactions

### Secondary Service

Routes traffic to SECONDARY replicas (round-robin):

```yaml
apiVersion: v1
kind: Service
metadata:
  name: prod-ag-secondary
spec:
  selector:
    mssql.microsoft.com/ag-role: secondary
  ports:
    - port: 1434
```

Use for: Reporting queries, analytics, backups

### Connection Strings

```
# Read-write (primary)
Server=prod-ag-primary.mssql.svc.cluster.local,1433;
ApplicationIntent=ReadWrite;

# Read-only (secondary)
Server=prod-ag-secondary.mssql.svc.cluster.local,1434;
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
