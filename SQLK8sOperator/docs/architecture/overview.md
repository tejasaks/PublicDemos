# Architecture Overview

[← Back to Documentation](../README.md)

This document provides a high-level overview of the MSSQL Kubernetes Operator architecture, its components, and how they interact.

## Table of Contents

- [Architecture Overview](#architecture-overview)
  - [Table of Contents](#table-of-contents)
  - [System Overview](#system-overview)
  - [Design Principles](#design-principles)
  - [Component Architecture](#component-architecture)
  - [Technology Stack](#technology-stack)
  - [Design References](#design-references)
  - [Key Abstractions](#key-abstractions)
    - [Custom Resource Definitions](#custom-resource-definitions)
    - [Resource Ownership](#resource-ownership)
  - [Data Flow](#data-flow)
    - [User Request Flow](#user-request-flow)
    - [Runtime Data Flow](#runtime-data-flow)
  - [Next Steps](#next-steps)

## System Overview

The MSSQL Kubernetes Operator follows the standard Kubernetes operator pattern, extending the API server with Custom Resource Definitions (CRDs) and running controllers that reconcile the desired state with actual state.

The operator manages:
- **SQL Server instances** - Deployed as StatefulSets with persistent storage
- **Availability Groups** - High availability with automatic failover
- **Monitoring** - Prometheus metrics via SQL Exporter sidecar
- **Services** - Network access for primary and secondary replicas

## Design Principles

| Principle | Description |
|-----------|-------------|
| **Declarative** | Users declare desired state; operator handles implementation |
| **Eventual Consistency** | Reconciliation loops drive toward desired state |
| **Kubernetes-Native** | Uses StatefulSets, Services, PVCs, ConfigMaps |
| **Graceful Degradation** | Handles missing dependencies with warnings |
| **Separation of Concerns** | Sidecars for monitoring and AG management |

## Component Architecture

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                              Kubernetes Cluster                              │
├─────────────────────────────────────────────────────────────────────────────┤
│  ┌─────────────────────────────────────────────────────────────────────┐    │
│  │                      MSSQL Operator (Deployment)                     │    │
│  │  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────────┐  │    │
│  │  │ SQLServer       │  │ SQLServerAG     │  │ OperatorConfig      │  │    │
│  │  │ Controller      │  │ Controller      │  │ Controller          │  │    │
│  │  │                 │  │                 │  │                     │  │    │
│  │  │ Watches:        │  │ Watches:        │  │ Watches:            │  │    │
│  │  │ - SQLServer CR  │  │ - SQLServerAG   │  │ - OperatorConfig    │  │    │
│  │  │                 │  │                 │  │                     │  │    │
│  │  │ Creates:        │  │ Creates:        │  │ Manages:            │  │    │
│  │  │ - StatefulSet   │  │ - Services      │  │ - Defaults          │  │    │
│  │  │ - Services      │  │ - Endpoints     │  │ - Image mappings    │  │    │
│  │  │ - PVCs          │  │                 │  │ - Feature flags     │  │    │
│  │  │ - ConfigMaps    │  │                 │  │                     │  │    │
│  │  └────────┬────────┘  └────────┬────────┘  └─────────────────────┘  │    │
│  └───────────┼────────────────────┼────────────────────────────────────┘    │
│              │                    │                                          │
│              ▼                    ▼                                          │
│  ┌───────────────────────────────────────────────────────────────────────┐  │
│  │                         Managed Resources                              │  │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐   │  │
│  │  │ StatefulSet │  │  Services   │  │    PVCs     │  │ ConfigMaps  │   │  │
│  │  │             │  │ (Headless,  │  │ (Data, Log, │  │ (mssql.conf)│   │  │
│  │  │ Pods 0..N   │  │  Listener)  │  │  TempDB)    │  │             │   │  │
│  │  └─────────────┘  └─────────────┘  └─────────────┘  └─────────────┘   │  │
│  └───────────────────────────────────────────────────────────────────────┘  │
│                                                                              │
│  ┌───────────────────────────────────────────────────────────────────────┐  │
│  │                         SQL Server Pods                                │  │
│  │  ┌─────────────────────────────────────────────────────────────────┐  │  │
│  │  │  Pod (e.g., sql-prod-01-0)                                       │  │  │
│  │  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐               │  │  │
│  │  │  │ SQL Server  │  │ AG Helper   │  │SQL Exporter │               │  │  │
│  │  │  │ :1433       │  │ :8080       │  │ :9399       │               │  │  │
│  │  │  └─────────────┘  └─────────────┘  └─────────────┘               │  │  │
│  │  └─────────────────────────────────────────────────────────────────┘  │  │
│  └───────────────────────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Technology Stack

| Component | Technology | Version | Purpose |
|-----------|------------|---------|---------|
| Language | Go | 1.24+ | Operator implementation |
| Framework | controller-runtime | v0.17.0 | Kubernetes controller SDK |
| Client | client-go | v0.29.0 | Kubernetes API client |
| CRD Generation | controller-gen | v0.14.0 | CRD and RBAC generation |
| Container Runtime | Docker/Podman | Latest | Building images |
| Target Platform | Kubernetes | 1.28+ | Deployment target |

## Design References

The operator architecture was informed by established patterns from:

| Reference | What We Learned |
|-----------|-----------------|
| [Zalando postgres-operator](https://github.com/zalando/postgres-operator) | StatefulSet management, OnDelete update strategy |
| [CrunchyData postgres-operator](https://github.com/CrunchyData/postgres-operator) | controller-runtime patterns, reconciliation loops |
| [Microsoft mssql-server-ha](https://github.com/Microsoft/mssql-server-ha) | Pacemaker AG management (ported to AG Helper sidecar) |
| [Microsoft Learn](https://learn.microsoft.com/en-us/sql/linux/sql-server-linux-kubernetes-best-practices-statefulsets) | Best practices for StatefulSets |

## Key Abstractions

### Custom Resource Definitions

```
┌─────────────────────────────────────────────────────────────┐
│                    OperatorConfiguration                     │
│  (Cluster-scoped singleton)                                 │
│  - Default images per SQL version                           │
│  - Validation settings                                      │
│  - Feature flags                                            │
└──────────────────────────┬──────────────────────────────────┘
                           │ provides defaults to
                           ▼
┌─────────────────────────────────────────────────────────────┐
│                        SQLServer                             │
│  (Namespace-scoped)                                          │
│  - Defines SQL Server instance configuration                 │
│  - Owns: StatefulSet, Services, PVCs, ConfigMaps            │
└──────────────────────────┬──────────────────────────────────┘
                           │ referenced by
                           ▼
┌─────────────────────────────────────────────────────────────┐
│                       SQLServerAG                            │
│  (Namespace-scoped)                                          │
│  - Defines Availability Group configuration                  │
│  - Owns: Listener Service (optional), Endpoints              │
└─────────────────────────────────────────────────────────────┘
```

### Resource Ownership

All child resources include `ownerReferences` pointing to parent CRs, enabling:
- Automatic garbage collection on parent deletion
- Consistent resource lifecycle management
- Clear ownership visibility with `kubectl get --show-owners`

## Data Flow

### User Request Flow

```
┌──────────┐     ┌───────────────┐     ┌────────────────┐     ┌─────────────┐
│  User    │────▶│ kubectl apply │────▶│ API Server     │────▶│ etcd        │
│          │     │               │     │ + Validation   │     │ (storage)   │
└──────────┘     └───────────────┘     └───────┬────────┘     └─────────────┘
                                               │
                                               │ watch event
                                               ▼
┌──────────────────────────────────────────────────────────────────────────┐
│                         Operator Controller                               │
│  1. Receive SQLServer CR                                                 │
│  2. Validate configuration                                               │
│  3. Reconcile child resources (ConfigMap, PVC, StatefulSet, Service)    │
│  4. Update status                                                        │
└──────────────────────────────────────────────────────────────────────────┘
                                               │
                                               │ creates/updates
                                               ▼
┌──────────────────────────────────────────────────────────────────────────┐
│                         Kubernetes Resources                              │
│  - StatefulSet (manages pods)                                            │
│  - Services (network access)                                             │
│  - PVCs (persistent storage)                                             │
│  - ConfigMaps (SQL Server configuration)                                 │
└──────────────────────────────────────────────────────────────────────────┘
```

### Runtime Data Flow

```
┌─────────────────────────────────────────────────────────────────────────┐
│                           SQL Server Pod                                 │
│                                                                          │
│  ┌─────────────────┐         ┌─────────────────┐                        │
│  │   SQL Server    │◀───────▶│   AG Helper     │                        │
│  │                 │localhost│   Sidecar       │                        │
│  │  - User DBs     │         │  - Monitors AG  │                        │
│  │  - System DBs   │         │  - Failover API │                        │
│  │  - AG Endpoints │         │  - Health probes│                        │
│  └────────┬────────┘         └────────┬────────┘                        │
│           │                           │                                  │
│           │ :1433                     │ :8080                           │
│           ▼                           ▼                                  │
│  ┌─────────────────────────────────────────────────────────────────┐    │
│  │                        Kubernetes Services                       │    │
│  │   Listener Service (:1433) — routes to current primary          │    │
│  └─────────────────────────────────────────────────────────────────┘    │
│                                                                          │
│  ┌─────────────────┐                                                    │
│  │  SQL Exporter   │──────────▶ Prometheus ──────────▶ Grafana         │
│  │  :9399          │  metrics                                           │
│  └─────────────────┘                                                    │
└─────────────────────────────────────────────────────────────────────────┘
```

## Next Steps

- [Operator Design](operator-design.md) - Deep dive into controller patterns
- [CRD Design](crd-design.md) - Custom Resource Definition details
- [Sidecar Architecture](sidecar-architecture.md) - AG Helper and SQL Exporter
- [Networking](networking.md) - Services and traffic flow
- [AG Controller Workflow Details](../availability-groups/controller-workflow-details.md) - Complete AG Helper and Controller internals with failover scenario walkthrough
