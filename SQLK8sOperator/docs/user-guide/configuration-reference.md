# Configuration Reference

[← Back to User Guide](../README.md) | [Documentation Home](../README.md)

Complete reference for all configuration options in the MSSQL Kubernetes Operator CRDs.

## Table of Contents

- [SQLServer CRD](#sqlserver-crd)
- [SQLServerAG CRD](#sqlserverag-crd)
- [OperatorConfiguration CRD](#operatorconfiguration-crd)

## SQLServer CRD

### Full Spec Reference

```yaml
apiVersion: mssql.microsoft.com/v1alpha1
kind: SQLServer
metadata:
  name: string           # Max 13 characters
  namespace: string
spec:
  # Optional description for auditing (max 1024 chars)
  description: string
  
  # SQL Server version
  version: "2019" | "2022" | "2025"  # Default: "2025"
  
  # SQL Server edition
  edition: Developer | Express | Standard | Enterprise  # Default: Developer
  
  # Instance configuration
  instance:
    replicas: 1-9                    # Default: 1
    image: string                    # Optional, uses default based on version
    imagePullPolicy: Always | Never | IfNotPresent  # Default: IfNotPresent
    imagePullSecrets:                # Optional
      - name: string
    
    resources:
      limits:
        cpu: string                  # e.g., "4"
        memory: string               # e.g., "16Gi" (min 2Gi)
      requests:
        cpu: string
        memory: string
    
    storage:
      data:                          # Required
        size: string                 # e.g., "100Gi"
        storageClass: string         # Optional, uses default
        accessMode: ReadWriteOnce | ReadWriteMany | ReadOnlyMany
      log:                           # Optional
        size: string
        storageClass: string
        accessMode: string
      tempdb:                        # Optional
        size: string
        storageClass: string
        accessMode: string
      backup:                        # Optional
        size: string
        storageClass: string
        accessMode: string
    
    config:
      agentEnabled: bool             # Default: true
      hadrEnabled: bool              # Default: true (required for AG)
      memoryLimitMB: int             # Optional, SQL Server memory limit
      collation: string              # Default: SQL_Latin1_General_CP1_CI_AS
      lcid: int                      # Default: 1033 (English)
      traceFlags: [int]              # Optional, e.g., [1222, 3226]
      tlsEnabled: bool               # Default: false
      tlsCertSecretRef:
        name: string
      customMSSQLConf: string        # Raw mssql.conf content
    
    securityContext:                 # Pod security context
      # Standard Kubernetes PodSecurityContext
    
    nodeSelector:
      key: value
    
    tolerations:
      - key: string
        operator: Exists | Equal
        value: string
        effect: NoSchedule | PreferNoSchedule | NoExecute
    
    affinity:
      # Standard Kubernetes Affinity
    
    priorityClassName: string
  
  # Authentication
  credentials:
    saPasswordSecretRef:
      name: string                   # Required
      key: string                    # Default: "password"
    createDefaultLogin: bool         # Default: false
  
  # Active Directory (optional)
  activeDirectory:
    enabled: bool
    realm: string                    # e.g., "CONTOSO.COM"
    domainControllers: [string]
    serviceAccountSecretRef:
      name: string
    spnPrefix: string                # Default: "MSSQLSvc"
    netBIOSDomain: string
    dnsSuffix: string
    adminGroup: string
  
  # Service configuration
  service:
    type: ClusterIP | LoadBalancer | NodePort  # Default: ClusterIP
    port: int                        # Default: 1433
    nodePort: int                    # Optional, for NodePort type
    loadBalancerIP: string           # Optional, for LoadBalancer
    annotations:
      key: value
  
  # Monitoring
  monitoring:
    enabled: bool                    # Default: false
    exporterImage: string            # Default: burningalchemist/sql_exporter:latest
    exporterPort: int                # Default: 9399
  
  # Labels/annotations for child resources
  metadata:
    labels:
      key: value
    annotations:
      key: value
  
  # Graceful shutdown
  shutdown: bool                     # Default: false
```

### Status Fields

```yaml
status:
  phase: Pending | Creating | Running | Upgrading | Failed
  conditions:
    - type: Ready | Available | Progressing
      status: "True" | "False" | "Unknown"
      reason: string
      message: string
      lastTransitionTime: timestamp
  readyReplicas: int
  currentReplicas: int
  currentVersion: string
  targetVersion: string
```

## SQLServerAG CRD

### Full Spec Reference

```yaml
apiVersion: mssql.microsoft.com/v1alpha1
kind: SQLServerAG
metadata:
  name: string           # Max 13 characters
  namespace: string
spec:
  # Optional description
  description: string
  
  # Reference to SQLServer resource
  sqlServerRef:
    name: string                     # Required
  
  # Availability Group configuration
  availabilityGroup:
    name: string                     # AG name in SQL Server (max 128 chars)
    replicas: 2-9                    # Default: 3
    
    primaryConfig:
      availabilityMode: SynchronousCommit | AsynchronousCommit
      failoverMode: External         # Always External for K8s
      readableSecondary: No | ReadOnly | All
      sessionTimeout: int            # Default: 10
    
    secondaryConfig:
      availabilityMode: SynchronousCommit | AsynchronousCommit
      failoverMode: External
      readableSecondary: No | ReadOnly | All
      sessionTimeout: int
    
    seedingMode: Automatic | Manual  # Default: Automatic
    
    databases:
      - name: string                 # Database name
        backupPath: string           # For manual seeding
    
    dbFailover: bool                 # Default: true
    automaticFailover: bool          # Default: false (monitoring only)
    clusterType: External            # Always External
    endpointPort: int                # Default: 5022
    externalWriteLeaseValidity: string  # Default: "20s"
  
  # Failover configuration
  failover:
    automatic: bool
    dataLossThreshold: int           # 0 = no data loss allowed
    healthCheckTimeout: string       # e.g., "30s"
    leaseTimeout: string             # e.g., "60s"
    requiredSynchronizedSecondaries: int  # -1 = auto-calculate
  
  # Service endpoints
  endpoints:
    primary:
      type: LoadBalancer | ClusterIP | NodePort
      port: int                      # Default: 1433
      nodePort: int
      annotations:
        key: value
    secondary:
      type: LoadBalancer | ClusterIP | NodePort
      port: int                      # Default: 1434
      nodePort: int
      annotations:
        key: value
  
  # AG Helper sidecar configuration
  sidecar:
    image: string
    monitorInterval: string          # Default: "10s"
    connectionTimeout: string        # Default: "30s"
```

## OperatorConfiguration CRD

### Full Spec Reference

```yaml
apiVersion: mssql.microsoft.com/v1alpha1
kind: OperatorConfiguration
metadata:
  name: default          # Singleton, must be "default"
spec:
  # Default images per SQL version
  images:
    sql2019: string      # Default: mcr.microsoft.com/mssql/server:2019-latest
    sql2022: string      # Default: mcr.microsoft.com/mssql/server:2022-latest
    sql2025: string      # Default: mcr.microsoft.com/mssql/server:2025-latest
    agHelper: string     # Default: mssql-operator/ag-helper:latest
    sqlExporter: string  # Default: burningalchemist/sql_exporter:latest
  
  # Validation settings
  validation:
    clusterCapabilityChecks: bool    # Default: true
    validationTimeout: string        # Default: "3s"
    storageClassValidation: block | warn  # Default: block
    secretValidation: block | warn   # Default: warn
    passwordComplexity: enforce | warn  # Default: enforce
    nodeValidation: block | warn     # Default: block
  
  # Default resource limits
  defaults:
    resources:
      limits:
        cpu: string
        memory: string
      requests:
        cpu: string
        memory: string
```

## Common Patterns

### Minimal Configuration

```yaml
apiVersion: mssql.microsoft.com/v1alpha1
kind: SQLServer
metadata:
  name: sql-dev-01
  namespace: mssql
spec:
  instance:
    storage:
      data:
        size: 10Gi
  credentials:
    saPasswordSecretRef:
      name: sql-sa-password
```

### Production with Monitoring

```yaml
apiVersion: mssql.microsoft.com/v1alpha1
kind: SQLServer
metadata:
  name: sql-prod-01
  namespace: production
spec:
  version: "2025"
  edition: Standard
  instance:
    replicas: 1
    resources:
      limits:
        cpu: "4"
        memory: 16Gi
      requests:
        cpu: "2"
        memory: 8Gi
    storage:
      data:
        size: 100Gi
        storageClass: fast-ssd
      log:
        size: 50Gi
      backup:
        size: 200Gi
    config:
      agentEnabled: true
      memoryLimitMB: 14336
  credentials:
    saPasswordSecretRef:
      name: sql-prod-sa
  service:
    type: LoadBalancer
  monitoring:
    enabled: true
```

## Next Steps

- [Deployment Scenarios](deployment-scenarios.md) - Use case examples
- [Validation & Security](validation-security.md) - Input requirements
- [CRD Design](../architecture/crd-design.md) - Design decisions
