# MSSQL Kubernetes Operator

A Kubernetes operator for managing SQL Server instances with high availability, automated upgrades, and monitoring.

## Project Information

This project was created using:

| Tool | Version | Purpose |
|------|---------|---------|
| **Visual Studio Code** | 1.96+ | Primary IDE |
| **GitHub Copilot** | Latest | AI-powered code assistance |
| **Claude Opus 4.5** | claude-opus-4-20250514 | AI model for code generation and architecture design |
| **Go** | 1.22+ | Programming language |
| **controller-runtime** | v0.17.0 | Kubernetes controller framework |
| **Kubernetes** | v1.28+ | Target platform |

### Design References

The operator architecture was informed by:
- [Zalando postgres-operator](https://github.com/zalando/postgres-operator) - StatefulSet management patterns, OnDelete update strategy
- [CrunchyData postgres-operator](https://github.com/CrunchyData/postgres-operator) - controller-runtime patterns, reconciliation loops
- [Microsoft mssql-server-ha](https://github.com/Microsoft/mssql-server-ha) - Pacemaker AG management logic (ported to AG Helper sidecar)
- [Microsoft Learn SQL Server on Kubernetes](https://learn.microsoft.com/en-us/sql/linux/sql-server-linux-kubernetes-best-practices-statefulsets) - Best practices for StatefulSets

## Features

### Phase 1 (Current)
- **Deploy SQL Server Instances**: Declarative management of SQL Server 2019, 2022, and 2025
- **High Availability**: Availability Groups with automatic failover via AG Helper sidecar
- **Upgrades**: Zero-downtime rolling upgrades using OnDelete StatefulSet strategy
- **Scaling**: Scale replicas up/down with persistent storage
- **Monitoring**: Prometheus metrics via SQL Exporter sidecar
- **Active Directory**: Windows Authentication support for SQL Server on Linux

### Planned (Phase 1.1)
- OpenShift support

### Planned (Phase 2)
- Backup and Restore automation
- Point-in-time recovery

## Architecture

```
┌─────────────────────────────────────────────────────────────────┐
│                     Kubernetes Cluster                           │
│                                                                   │
│  ┌─────────────────────────────────────────────────────────────┐ │
│  │                MSSQL Operator (Deployment)                   │ │
│  │  ┌─────────────┐  ┌─────────────────┐  ┌──────────────────┐ │ │
│  │  │ SQLServer   │  │ SQLServerAG     │  │ Configuration    │ │ │
│  │  │ Controller  │  │ Controller      │  │ Controller       │ │ │
│  │  └─────────────┘  └─────────────────┘  └──────────────────┘ │ │
│  └─────────────────────────────────────────────────────────────┘ │
│                              │                                    │
│                              ▼                                    │
│  ┌─────────────────────────────────────────────────────────────┐ │
│  │               SQL Server StatefulSet                         │ │
│  │  ┌─────────────────────────────────────────────────────────┐ │ │
│  │  │ Pod 0 (Primary)                                          │ │ │
│  │  │  ┌─────────────┐ ┌─────────────┐ ┌─────────────────────┐│ │ │
│  │  │  │ SQL Server  │ │ AG Helper   │ │ SQL Exporter        ││ │ │
│  │  │  │ Container   │ │ Sidecar     │ │ (Monitoring)        ││ │ │
│  │  │  └─────────────┘ └─────────────┘ └─────────────────────┘│ │ │
│  │  └─────────────────────────────────────────────────────────┘ │ │
│  │  ┌─────────────────────────────────────────────────────────┐ │ │
│  │  │ Pod 1 (Secondary)                                        │ │ │
│  │  │  ┌─────────────┐ ┌─────────────┐ ┌─────────────────────┐│ │ │
│  │  │  │ SQL Server  │ │ AG Helper   │ │ SQL Exporter        ││ │ │
│  │  │  │ Container   │ │ Sidecar     │ │ (Monitoring)        ││ │ │
│  │  │  └─────────────┘ └─────────────┘ └─────────────────────┘│ │ │
│  │  └─────────────────────────────────────────────────────────┘ │ │
│  └─────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────┘
```

## Quick Start

### Prerequisites

- Kubernetes 1.28+
- Helm 3.0+
- kubectl configured

### Installation

1. **Install the operator using Helm:**

```bash
# Add the Helm repository (when available)
# helm repo add mssql-operator https://microsoft.github.io/mssql-operator

# Or install from local chart
helm install mssql-operator ./helm/mssql-operator \
  --namespace mssql-system \
  --create-namespace
```

2. **Create a SQL Server instance:**

```yaml
apiVersion: mssql.microsoft.com/v1alpha1
kind: SQLServer
metadata:
  name: my-sqlserver
  namespace: mssql
spec:
  version: "2022"
  edition: Developer
  instance:
    replicas: 1
    resources:
      limits:
        cpu: "2"
        memory: 4Gi
      requests:
        cpu: "1"
        memory: 2Gi
    storage:
      data:
        size: 10Gi
  credentials:
    saPasswordSecretRef:
      name: mssql-sa-password
      key: password
  service:
    type: ClusterIP
    port: 1433
  monitoring:
    enabled: true
```

3. **Apply the manifest:**

```bash
# Create namespace
kubectl create namespace mssql

# Create SA password secret
kubectl create secret generic mssql-sa-password \
  --from-literal=password='YourStrong@Passw0rd!' \
  -n mssql

# Apply SQL Server manifest
kubectl apply -f my-sqlserver.yaml
```

4. **Check status:**

```bash
kubectl get sqlserver -n mssql
kubectl get pods -n mssql
```

## Custom Resources

### SQLServer

Manages standalone SQL Server instances or AG replicas.

```yaml
apiVersion: mssql.microsoft.com/v1alpha1
kind: SQLServer
spec:
  version: "2022"        # 2019, 2022, or 2025
  edition: Developer     # Developer, Express, Standard, Enterprise
  instance:
    replicas: 1
    storage:
      data:
        size: 10Gi
  credentials:
    saPasswordSecretRef:
      name: secret-name
```

### SQLServerAG

Manages Availability Groups for high availability.

```yaml
apiVersion: mssql.microsoft.com/v1alpha1
kind: SQLServerAG
spec:
  sqlServerRef:
    name: my-sqlserver
  availabilityGroup:
    name: ProductionAG
    replicas: 3
    primaryConfig:
      availabilityMode: SynchronousCommit
      failoverMode: External
    secondaryConfig:
      availabilityMode: SynchronousCommit
      failoverMode: External
    databases:
      - name: MyDatabase
```

## Development

### Setting Execute Permissions

Before running any shell scripts, ensure they have execute permissions:

```bash
# From project root, set permissions on all shell scripts
chmod +x scripts/*.sh
chmod +x tests/*.sh
```

> **Note**: This step is required on Linux/macOS. Windows users running via WSL or Git Bash may also need to run this.

### Local Development with minikube

```bash
# Run the development setup script (deploys SQL 2025 standalone by default)
./scripts/dev-setup.sh all

# Or step by step:
./scripts/dev-setup.sh prereq  # Check prerequisites
./scripts/dev-setup.sh start   # Start minikube
./scripts/dev-setup.sh build   # Build operator images
./scripts/dev-setup.sh install # Install operator
./scripts/dev-setup.sh deploy  # Deploy SQL 2025 standalone (default)
./scripts/dev-setup.sh status  # Check status

# Deploy a specific configuration:
./scripts/dev-setup.sh deploy samples/sqlserver-2022-standalone.yaml
./scripts/dev-setup.sh deploy samples/sqlserver-availability-group.yaml
./scripts/dev-setup.sh deploy samples/sqlserver-with-ad.yaml
```

### Building

```bash
# Build operator binary
make build

# Build Docker images
make docker-build

# Run tests
make test

# Generate CRDs
make manifests
```

## Configuration Files Reference

This section describes all configuration files that end users need to review and customize for successful deployment.

### Sample Deployment Files

Located in `samples/` directory:

| File | Description | When to Use |
|------|-------------|-------------|
| `sqlserver-2025-standalone.yaml` | SQL Server 2025 single instance | Default for dev-setup.sh; latest SQL version |
| `sqlserver-2022-standalone.yaml` | SQL Server 2022 single instance | Production-ready stable version |
| `sqlserver-availability-group.yaml` | 3-replica HA cluster with AG | High availability requirements |
| `sqlserver-with-ad.yaml` | SQL Server with AD authentication | Windows Authentication needed |

### Required Configuration Changes

Before deploying, you **must** update the following:

#### 1. SA Password Secret

**File:** All sample YAML files contain a Secret definition

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: mssql-sa-password
  namespace: mssql
type: Opaque
stringData:
  # CHANGE THIS! Use a strong password meeting SQL Server requirements:
  # - Minimum 8 characters
  # - Uppercase letter, lowercase letter, number, and special character
  password: "YourStrong@Passw0rd!"
```

**Security Note:** In production, use external secret management (Azure Key Vault, HashiCorp Vault, etc.) instead of inline secrets.

#### 2. Storage Configuration

**Section:** `spec.instance.storage`

```yaml
storage:
  data:
    size: 10Gi                    # Adjust based on data requirements
    storageClass: managed-premium  # Set to your cluster's StorageClass
  log:
    size: 5Gi                     # Separate log volume (recommended)
    storageClass: managed-premium
  tempdb:
    size: 5Gi                     # Optional but recommended
  backup:
    size: 10Gi                    # For backup files
```

Run `kubectl get storageclass` to see available options in your cluster.

#### 3. Resource Limits

**Section:** `spec.instance.resources`

```yaml
resources:
  limits:
    cpu: "4"           # Max CPU cores
    memory: 8Gi        # Max memory (SQL Server uses up to 80% by default)
  requests:
    cpu: "2"           # Guaranteed CPU
    memory: 4Gi        # Guaranteed memory
```

**Recommendation:** SQL Server minimum is 2GB RAM; production should have 4GB+.

#### 4. Service Type

**Section:** `spec.service`

```yaml
service:
  type: LoadBalancer    # ClusterIP, NodePort, or LoadBalancer
  port: 1433
  # For cloud providers:
  annotations:
    service.beta.kubernetes.io/azure-load-balancer-internal: "true"
```

### Active Directory Configuration

**File:** `samples/sqlserver-with-ad.yaml`

Required secrets and configuration for AD authentication:

| Configuration | Description | Required |
|--------------|-------------|----------|
| `activeDirectory.realm` | Kerberos realm (e.g., `CONTOSO.COM`) | Yes |
| `activeDirectory.domainControllers` | List of DC hostnames/IPs | Yes |
| `mssql-ad-service-account` Secret | AD service account credentials | Yes |
| `activeDirectory.adminGroup` | AD group for sysadmin access | Recommended |
| `mssql-keytab` Secret | Pre-generated keytab file | Optional |
| `mssql-tls-cert` Secret | TLS certificate for encryption | Recommended |

```yaml
# AD Service Account Secret
apiVersion: v1
kind: Secret
metadata:
  name: mssql-ad-service-account
stringData:
  username: sqlsvc@contoso.com    # UPDATE: Your AD service account
  password: "ADPassword!"         # UPDATE: Service account password
```

### Availability Group Configuration

**File:** `samples/sqlserver-availability-group.yaml`

Key settings to customize:

| Setting | Location | Description |
|---------|----------|-------------|
| `replicas` | `spec.instance.replicas` | Number of AG replicas (2-9) |
| `availabilityGroup.name` | `spec.availabilityGroup.name` | AG name in SQL Server |
| `databases` | `spec.availabilityGroup.databases` | Databases to include in AG |
| `seedingMode` | `spec.availabilityGroup.seedingMode` | `Automatic` or `Manual` |
| `endpoints.primary` | `spec.endpoints.primary` | Primary replica service |
| `endpoints.secondary` | `spec.endpoints.secondary` | Read-only routing service |

### Monitoring Configuration

**Section:** `spec.monitoring`

The SQL Exporter sidecar provides Prometheus metrics:

```yaml
monitoring:
  enabled: true
  exporterImage: burningalchemist/sql_exporter:latest
  exporterPort: 9399
  # Custom SQL queries for additional metrics
  customQueries: |
    collector_name: mssql_custom
    metrics:
      - metric_name: mssql_connections
        type: gauge
        help: 'Number of active connections'
        values: [count]
        query: |
          SELECT COUNT(*) as count 
          FROM sys.dm_exec_connections
```

**Custom Metrics:** The `customQueries` field accepts SQL Exporter configuration format. See [sql_exporter documentation](https://github.com/burningalchemist/sql_exporter) for query syntax.

### Helm Values (Operator Configuration)

**File:** `helm/mssql-operator/values.yaml`

Key operator settings:

```yaml
# Operator image (update for custom builds)
image:
  repository: mssql-operator
  tag: latest

# Default SQL Server images
sqlServer:
  image2022: mcr.microsoft.com/mssql/server:2022-latest
  image2025: mcr.microsoft.com/mssql/server:2025-latest

# AG Helper sidecar
agHelper:
  image:
    repository: mssql-ag-helper
    tag: latest

# Monitoring exporter default
monitoring:
  exporterImage: burningalchemist/sql_exporter:latest

# Operator behavior
operator:
  workers: 8              # Concurrent reconciliation workers
  resyncPeriod: 30m       # Full resync interval
  watchNamespace: ""      # Empty = all namespaces
```

### SQL Server Configuration Options

**Section:** `spec.instance.config`

```yaml
config:
  agentEnabled: true              # SQL Server Agent
  hadrEnabled: true               # Always On AG support
  collation: SQL_Latin1_General_CP1_CI_AS
  lcid: 1033                      # Language ID (1033 = English)
  memoryLimitMB: 4096             # Memory limit for SQL Server
  traceFlags: [1222, 3226]        # Trace flags to enable
  tlsEnabled: true                # Enable TLS encryption
  tlsCertSecretRef:
    name: mssql-tls-cert          # TLS certificate secret
  customMSSQLConf: |              # Raw mssql.conf additions
    [network]
    forceencryption = 1
```

### Configuration Checklist

Before deploying to production:

- [ ] Change all passwords in Secret resources
- [ ] Set appropriate `storageClass` for your cluster
- [ ] Adjust `resources` based on workload requirements
- [ ] Configure `service.type` for network access
- [ ] Enable TLS encryption (`tlsEnabled: true`)
- [ ] Set up Active Directory if Windows Auth is needed
- [ ] Configure `customQueries` for application-specific metrics
- [ ] Set `podAntiAffinity` for HA deployments
- [ ] Review and set appropriate `tolerations` and `nodeSelector`

## Operator Configuration

The operator can be configured via Helm values or the `OperatorConfiguration` CRD:

```yaml
apiVersion: mssql.microsoft.com/v1alpha1
kind: OperatorConfiguration
metadata:
  name: default
spec:
  workers: 8
  resyncPeriod: 30m
  resourceDefaults:
    defaultCPURequest: "1"
    defaultMemoryRequest: "2Gi"
  kubernetes:
    enablePodDisruptionBudget: true
    podAntiAffinity:
      enabled: true
```

### Active Directory Configuration

For Windows Authentication:

```yaml
apiVersion: mssql.microsoft.com/v1alpha1
kind: SQLServer
spec:
  activeDirectory:
    enabled: true
    realm: CONTOSO.COM
    domainControllers:
      - dc1.contoso.com
    serviceAccountSecretRef:
      name: ad-service-account
    adminGroup: SQLServerAdmins
```

## Monitoring

The operator integrates with Prometheus via the SQL Exporter sidecar:

```yaml
spec:
  monitoring:
    enabled: true
    exporterImage: burningalchemist/sql_exporter:latest
    exporterPort: 9399
```

Metrics are exposed at `/metrics` on port 9399.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for details.

## License

MIT License - see [LICENSE](LICENSE) for details.

## References

- [SQL Server on Kubernetes Best Practices](https://learn.microsoft.com/en-us/sql/linux/sql-server-linux-kubernetes-best-practices-statefulsets)
- [SQL Server Active Directory Authentication](https://learn.microsoft.com/en-us/sql/linux/sql-server-linux-active-directory-authentication)
- [Availability Groups on Linux](https://learn.microsoft.com/en-us/sql/linux/sql-server-linux-availability-group-overview)
