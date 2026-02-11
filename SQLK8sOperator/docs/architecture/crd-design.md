# CRD Design

[← Back to Architecture](overview.md) | [Documentation Home](../README.md)

This document describes the Custom Resource Definition (CRD) design decisions, including API versioning, field organization, and evolution strategy.

## Table of Contents

- [CRD Overview](#crd-overview)
- [API Versioning](#api-versioning)
- [SQLServer CRD](#sqlserver-crd)
- [SQLServerAG CRD](#sqlserverag-crd)
- [OperatorConfiguration CRD](#operatorconfiguration-crd)
- [Design Decisions](#design-decisions)
- [Validation Strategy](#validation-strategy)
- [Backward Compatibility](#backward-compatibility)

## CRD Overview

The operator defines three Custom Resource Definitions:

| CRD | Scope | Purpose |
|-----|-------|---------|
| `SQLServer` | Namespaced | Defines a SQL Server instance |
| `SQLServerAG` | Namespaced | Defines an Availability Group |
| `OperatorConfiguration` | Cluster | Operator-wide defaults and settings |

### Relationship Model

```
┌─────────────────────────────────────────────────────────────┐
│                    OperatorConfiguration                     │
│  (Cluster-scoped, singleton "default")                      │
│                                                              │
│  Provides:                                                   │
│  - Default images per SQL version                           │
│  - Validation behavior settings                             │
│  - Feature flags                                            │
└──────────────────────────┬──────────────────────────────────┘
                           │ provides defaults to
                           ▼
┌─────────────────────────────────────────────────────────────┐
│                        SQLServer                             │
│  (Namespace-scoped)                                          │
│                                                              │
│  Owns:                                                       │
│  - StatefulSet (SQL Server pods)                            │
│  - Services (headless, client)                              │
│  - PVCs (data, log, tempdb, backup)                         │
│  - ConfigMaps (mssql.conf)                                  │
└──────────────────────────┬──────────────────────────────────┘
                           │ referenced by
                           ▼
┌─────────────────────────────────────────────────────────────┐
│                       SQLServerAG                            │
│  (Namespace-scoped)                                          │
│                                                              │
│  References: SQLServer via spec.sqlServerRef                │
│                                                              │
│  Owns:                                                       │
│  - Service (primary endpoint)                               │
│  - Service (secondary endpoint)                             │
└─────────────────────────────────────────────────────────────┘
```

## API Versioning

### Current Version: v1alpha1

The API is currently at `v1alpha1`, indicating:
- API is experimental
- Breaking changes may occur between releases
- Not recommended for production without version pinning

### Version Graduation Path

```
v1alpha1 (current)
    │
    │ stabilize, gather feedback
    ▼
v1beta1
    │
    │ deprecation period, conversion webhooks
    ▼
v1 (stable)
```

### Version in Manifests

```yaml
apiVersion: mssql.microsoft.com/v1alpha1
kind: SQLServer
metadata:
  name: sql-prod-01
```

## SQLServer CRD

### Spec Structure

```yaml
spec:
  # Optional description for auditing
  description: string (max 1024 chars)
  
  # SQL Server version
  version: "2019" | "2022" | "2025"
  
  # SQL Server edition  
  edition: Developer | Express | Standard | Enterprise
  
  # Instance configuration
  instance:
    count: 1-9
    image: string (optional, uses default if omitted)
    imagePullPolicy: Always | Never | IfNotPresent
    imagePullSecrets: []
    resources:
      limits: {cpu, memory}
      requests: {cpu, memory}
    storage:
      data: {size, storageClass, accessMode}
      log: {size, storageClass, accessMode}
      tempdb: {size, storageClass, accessMode}
      backup: {size, storageClass, accessMode}
    config:
      agentEnabled: bool
      hadrEnabled: bool
      memoryLimitMB: int
      collation: string
      traceFlags: []int
  
  # Authentication
  credentials:
    saPasswordSecretRef:
      name: string
      key: string
  
  # Active Directory (optional)
  activeDirectory:
    enabled: bool
    realm: string
    domainControllers: []string
    # ... more AD settings
  
  # Service configuration
  service:
    type: ClusterIP | LoadBalancer | NodePort
    port: int
    nodePort: int (optional)
  
  # Monitoring
  monitoring:
    enabled: bool
    exporterImage: string
    exporterPort: int
```

### Status Structure

```yaml
status:
  phase: Pending | Creating | Running | Upgrading | Failed
  conditions:
    - type: Ready
      status: "True" | "False" | "Unknown"
      reason: string
      message: string
      lastTransitionTime: timestamp
  readyInstances: int
  currentInstances: int
  currentVersion: string
  targetVersion: string
```

## SQLServerAG CRD

### Spec Structure

```yaml
spec:
  # Optional description
  description: string
  
  # Reference to SQLServer resource
  sqlServerRef:
    name: string
  
  # AG configuration
  availabilityGroup:
    name: string (SQL identifier, max 128 chars)
    instanceCount: 2-9
    primaryConfig:
      availabilityMode: SynchronousCommit | AsynchronousCommit
      failoverMode: External
      readableSecondary: No | ReadOnly | All
    secondaryConfig:
      availabilityMode: SynchronousCommit | AsynchronousCommit
      failoverMode: External
      readableSecondary: No | ReadOnly | All
    seedingMode: Automatic | Manual
    databases:
      - name: string
        backupPath: string (optional)
    automaticFailover: bool
  
  # Failover configuration
  failover:
    automatic: bool
    dataLossThreshold: int
    healthCheckTimeout: duration
    leaseTimeout: duration
  
  # Service endpoints
  endpoints:
    primary:
      type: LoadBalancer | ClusterIP | NodePort
      port: int
    secondary:
      type: LoadBalancer | ClusterIP | NodePort
      port: int
  
  # AG Helper sidecar config
  sidecar:
    image: string
    monitorInterval: duration
    connectionTimeout: duration
```

## OperatorConfiguration CRD

### Spec Structure

```yaml
spec:
  # Image defaults per version
  images:
    sql2019: mcr.microsoft.com/mssql/server:2019-latest
    sql2022: mcr.microsoft.com/mssql/server:2022-latest
    sql2025: mcr.microsoft.com/mssql/server:2025-latest
    agHelper: mssql-operator/ag-helper:latest
    sqlExporter: burningalchemist/sql_exporter:latest
  
  # Validation settings
  validation:
    clusterCapabilityChecks: bool
    validationTimeout: duration
    storageClassValidation: block | warn
    secretValidation: block | warn
    passwordComplexity: enforce | warn
    nodeValidation: block | warn
  
  # Default resource limits
  defaults:
    resources:
      limits: {cpu, memory}
      requests: {cpu, memory}
```

## Design Decisions

### Nested vs Flat Structure

**Decision:** Use nested structure for logical grouping

```yaml
# Chosen: Nested (better organization)
spec:
  instance:
    count: 3
    storage:
      data:
        size: 10Gi

# Alternative: Flat (rejected - harder to read)
spec:
  instanceCount: 3
  instanceStorageDataSize: 10Gi
```

### Inline vs Reference

**Decision:** Use references for sensitive data

```yaml
# Chosen: SecretRef (secrets stay in Secrets)
credentials:
  saPasswordSecretRef:
    name: sql-sa-password
    key: password

# Rejected: Inline password
credentials:
  saPassword: "MyPassword123!"  # Bad! In etcd, in logs
```

### Required vs Optional Fields

**Guidelines:**
- Required: Essential for basic operation
- Optional with defaults: Common configurations
- Optional without defaults: Advanced/rare configurations

```go
// Required - no default, must be provided
Storage StorageSpec `json:"storage"`

// Optional with default
// +kubebuilder:default="2025"
Version string `json:"version,omitempty"`

// Optional without default
// +optional
ActiveDirectory *ActiveDirectorySpec `json:"activeDirectory,omitempty"`
```

## Validation Strategy

### Layered Validation

| Layer | When | What | Behavior |
|-------|------|------|----------|
| **CRD Schema** | kubectl apply | Types, enums, required | Reject |
| **Admission Webhook** | Before persist | Cluster checks, security | Block/Warn |
| **Controller** | Reconcile | Runtime state | Update status |

### CRD Schema Validation

```go
// Enum validation
// +kubebuilder:validation:Enum=Developer;Express;Standard;Enterprise
Edition string `json:"edition,omitempty"`

// Range validation
// +kubebuilder:validation:Minimum=1
// +kubebuilder:validation:Maximum=9
Replicas int32 `json:"replicas,omitempty"`

// Pattern validation
// +kubebuilder:validation:Pattern=^[a-z0-9]([-a-z0-9]*[a-z0-9])?$
Name string `json:"name"`

// Length validation
// +kubebuilder:validation:MaxLength=1024
Description string `json:"description,omitempty"`
```

### Webhook Validation

```go
func (v *SQLServerValidator) validate(sqlserver *SQLServer) *ValidationResult {
    // Cluster capability checks
    if storageClass != "" {
        result.Merge(v.validateStorageClass(storageClass))
    }
    
    // Security checks
    result.Merge(validation.ValidatePasswordComplexity(password))
    
    // Business logic
    if sqlserver.Spec.Instance.Count > 1 && !sqlserver.Spec.Instance.Config.HADREnabled {
        result.AddError("HADR must be enabled for instances > 1")
    }
}
```

## Backward Compatibility

### Adding Fields

Safe to add optional fields:

```go
// v1alpha1 - original
type SQLServerSpec struct {
    Version string `json:"version"`
}

// v1alpha1 - with new optional field
type SQLServerSpec struct {
    Version     string  `json:"version"`
    Description *string `json:"description,omitempty"` // New, optional
}
```

### Deprecating Fields

Use deprecation markers:

```go
// Deprecated: Use saPasswordSecretRef instead
// +optional
SAPassword string `json:"saPassword,omitempty"`
```

### Breaking Changes

For breaking changes, use conversion webhooks:

```go
func (src *SQLServerV1Alpha1) ConvertTo(dst *SQLServerV1Beta1) error {
    // Convert old format to new format
    dst.Spec.Credentials.SecretRef = src.Spec.Credentials.SAPasswordSecretRef
    return nil
}
```

## Next Steps

- [Sidecar Architecture](sidecar-architecture.md) - AG Helper and SQL Exporter
- [Networking](networking.md) - Services and traffic flow
- [Configuration Reference](../user-guide/configuration-reference.md) - All field details
