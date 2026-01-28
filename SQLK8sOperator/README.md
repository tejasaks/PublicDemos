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
- kubectl configured
- Docker (for building images)

### Installation

1. **Install the operator:**

```bash
# Install CRDs
kubectl apply -f deploy/crds/

# Install operator (namespace, RBAC, deployment)
kubectl apply -f deploy/namespace.yaml
kubectl apply -f deploy/serviceaccount.yaml
kubectl apply -f deploy/rbac.yaml
kubectl apply -f deploy/deployment.yaml
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

## Resource Naming Constraints

### SQL Server NetBIOS Name Limit

**Important:** SQL Server resource names must be **maximum 13 characters**.

This is due to SQL Server's NetBIOS name limit of 15 characters. Since Kubernetes pods get a 2-character suffix (e.g., `-0`, `-1`), the base name must be ≤13 characters.

| ✅ Valid Names | ❌ Invalid Names |
|----------------|------------------|
| `sql2022-dev01` | `mssql-2022-standalone` (21 chars) |
| `sql-ag-prod01` | `my-production-sqlserver` (23 chars) |
| `sqlprod-01` | `sqlserver-availability-group` (28 chars) |

The CRD enforces this with validation:
```yaml
metadata:
  name: sql2022-dev01  # Max 13 characters!
```

### Description Field for Searchability

Both SQLServer and SQLServerAG CRDs support a `description` field for auditing and searchability:

```yaml
spec:
  description: "Production SQL Server for order processing - Team Alpha"
```

This allows you to search resources:
```bash
kubectl get sqlserver -o jsonpath='{range .items[*]}{.metadata.name}: {.spec.description}{"\n"}{end}'
```

## Development

This section covers development workflows. Choose **one** of the two approaches based on your needs:

| Approach | Use Case | Prerequisites |
|----------|----------|---------------|
| **Local Development (minikube)** | Full end-to-end testing with a local K8s cluster | Docker, minikube |
| **Building Only** | Compile binaries/images without deploying | Go 1.22+, Docker (for images) |

### Setting Execute Permissions (Required)

Before running any shell scripts, ensure they have execute permissions:

```bash
# From project root, set permissions on all shell scripts
chmod +x scripts/*.sh
chmod +x tests/*.sh
```

> **Note**: This step is required on Linux/macOS. Windows users running via WSL or Git Bash may also need to run this.

---

### Option A: Local Development with minikube (Recommended for Testing)

This is the **recommended approach** for most developers. It sets up a complete local environment including:
- minikube Kubernetes cluster
- Operator images built and loaded into minikube
- Operator deployed and running
- Sample SQL Server instance

#### Quick Start (All-in-One)

```bash
# Run everything with a single command (deploys SQL 2025 standalone by default)
./scripts/dev-setup.sh all
```

#### Step-by-Step Execution

If you prefer more control, run each step individually:

| Step | Command | Description | Required? |
|------|---------|-------------|-----------|
| 1 | `./scripts/dev-setup.sh prereq` | Check/install prerequisites (minikube, kubectl, Go) | Yes |
| 2 | `./scripts/dev-setup.sh start` | Start minikube cluster with recommended settings | Yes |
| 3 | `./scripts/dev-setup.sh build` | Build operator and sidecar Docker images | Yes |
| 4 | `./scripts/dev-setup.sh install` | Install CRDs and deploy operator | Yes |
| 5 | `./scripts/dev-setup.sh deploy [yaml]` | Deploy a sample SQL Server instance | Optional |
| 6 | `./scripts/dev-setup.sh status` | Check operator and SQL Server status | Optional |

#### Deploy Specific Configurations

```bash
# Deploy SQL 2025 standalone (default)
./scripts/dev-setup.sh deploy

# Deploy SQL 2022 standalone
./scripts/dev-setup.sh deploy samples/sqlserver-2022-standalone.yaml

# Deploy Availability Group (3 replicas)
./scripts/dev-setup.sh deploy samples/sqlserver-availability-group.yaml

# Deploy with Active Directory
./scripts/dev-setup.sh deploy samples/sqlserver-with-ad.yaml
```

---

### Option B: Building Only (Without Deployment)

Use this approach when you only need to compile the code, build images, or run tests—without deploying to a cluster.

#### Prerequisites

- Go 1.22+ installed
- Docker (only for building images)
- No Kubernetes cluster required

#### Available Make Targets

| Command | Description | When to Use |
|---------|-------------|-------------|
| `make build` | Build operator binary to `bin/mssql-operator` | Local development, CI pipelines |
| `make build-sidecar` | Build AG helper binary to `bin/mssql-ag-helper` | Testing sidecar changes |
| `make docker-build` | Build operator Docker image | Preparing for deployment |
| `make docker-build-sidecar` | Build AG helper Docker image | Preparing for deployment |
| `make generate` | Generate DeepCopy functions from types | After modifying `*_types.go` files |
| `make manifests` | Generate CRD YAML manifests | After modifying `*_types.go` files |
| `make test` | Run unit tests with coverage | Before committing changes |
| `make fmt` | Format Go code | Before committing changes |
| `make vet` | Run Go static analysis | Before committing changes |
| `make lint` | Run golangci-lint | Before committing changes |

#### Typical Development Workflow

```bash
# 1. Make code changes to types
vim pkg/apis/mssql.microsoft.com/v1alpha1/sqlserver_types.go

# 2. Regenerate code and manifests
make generate manifests

# 3. Build and test
make build test

# 4. Build Docker images (when ready to deploy)
make docker-build docker-build-sidecar
```

---

### Running Tests

See [tests/README.md](tests/README.md) for comprehensive testing documentation.

```bash
# Run unit tests
make test

# Run end-to-end tests (requires operator deployed)
./tests/run-all-tests.sh

# Run specific test
./tests/test-sqlserver-standalone.sh
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

### Operator Deployment Configuration

**File:** `deploy/deployment.yaml`

Key operator settings (via environment variables):

```yaml
env:
  # Namespace where operator is deployed
  - name: OPERATOR_NAMESPACE
    valueFrom:
      fieldRef:
        fieldPath: metadata.namespace
  
  # AG Helper sidecar image
  - name: AG_HELPER_IMAGE
    value: "mssql-ag-helper:dev"
  - name: AG_HELPER_IMAGE_PULL_POLICY
    value: "Never"  # Use IfNotPresent for production
  
  # SQL Exporter image for monitoring
  - name: SQL_EXPORTER_IMAGE
    value: "burningalchemist/sql_exporter:latest"

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

The operator can be configured via environment variables in `deploy/deployment.yaml` or the `OperatorConfiguration` CRD:

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

### SQL Exporter Sidecar

Each SQL Server pod includes a SQL Exporter sidecar for metrics:

```yaml
spec:
  monitoring:
    enabled: true
    exporterImage: burningalchemist/sql_exporter:latest
    exporterPort: 9399
```

Metrics are exposed at `/metrics` on port 9399.

### Prometheus + Grafana Stack

Deploy a complete monitoring stack with pre-configured SQL Server dashboards:

```bash
# Deploy monitoring stack
./scripts/dev-setup.sh monitoring

# Or during full setup
./scripts/dev-setup.sh all-with-monitoring
```

**Manual deployment:**
```bash
kubectl apply -f deploy/monitoring/prometheus.yaml
kubectl apply -f deploy/monitoring/grafana.yaml
```

**Prometheus configuration:**
- Retention: 7 days / 1GB (configurable in `prometheus.yaml`)
- Auto-discovers SQL pods with `prometheus.io/scrape: "true"` annotation
- Accessible at `prometheus.monitoring:9090`

**Grafana configuration:**
- Pre-configured with SQL Server Overview dashboard
- Default credentials: `admin / admin`
- Accessible at `grafana.monitoring:3000`

**Access dashboards:**
```bash
# Port forward Grafana
kubectl port-forward -n monitoring svc/grafana 3000:3000
# Open: http://localhost:3000

# Port forward Prometheus
kubectl port-forward -n monitoring svc/prometheus 9090:9090
# Open: http://localhost:9090
```

## External Access

### LoadBalancer (Default)

Sample files use `LoadBalancer` service type by default:

```yaml
service:
  type: LoadBalancer
  port: 1433
```

**For minikube:**
```bash
# Start tunnel in a separate terminal (keeps running)
minikube tunnel

# Get external IP
kubectl get svc -n mssql
```

### NodePort Alternative

For environments without LoadBalancer support:

```yaml
service:
  type: NodePort
  port: 1433
  nodePort: 31433  # Optional: specific port
```

Connect using `minikube ip`:
```bash
sqlcmd -S $(minikube ip),31433 -U sa -P 'YourPassword'
```

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for details.

## License

MIT License - see [LICENSE](LICENSE) for details.

## References

- [SQL Server on Kubernetes Best Practices](https://learn.microsoft.com/en-us/sql/linux/sql-server-linux-kubernetes-best-practices-statefulsets)
- [SQL Server Active Directory Authentication](https://learn.microsoft.com/en-us/sql/linux/sql-server-linux-active-directory-authentication)
- [Availability Groups on Linux](https://learn.microsoft.com/en-us/sql/linux/sql-server-linux-availability-group-overview)
