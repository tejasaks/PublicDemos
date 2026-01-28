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
- **Graceful AG Setup**: AG Helper waits for AG configuration without errors or probe failures
- **Upgrades**: Zero-downtime rolling upgrades using OnDelete StatefulSet strategy
- **Scaling**: Scale replicas up/down with persistent storage
- **Monitoring**: Prometheus metrics via SQL Exporter sidecar + Grafana dashboards
- **Active Directory**: Windows Authentication support for SQL Server on Linux

### Planned (Phase 1.1)
- OpenShift support
- Multiple AG monitoring per sidecar

### Planned (Phase 2)
- Backup and Restore automation
- Point-in-time recovery
- Automated AG configuration via Jobs

## Architecture

```
┌─────────────────────────────────────────────────────────────────────────┐
│                         Kubernetes Cluster                               │
│                                                                          │
│  ┌────────────────────────────────────────────────────────────────────┐ │
│  │                    MSSQL Operator (Deployment)                      │ │
│  │  ┌─────────────────┐  ┌───────────────────┐  ┌──────────────────┐  │ │
│  │  │ SQLServer       │  │ SQLServerAG       │  │ Configuration    │  │ │
│  │  │ Controller      │  │ Controller        │  │ Controller       │  │ │
│  │  │                 │  │                   │  │                  │  │ │
│  │  │ Creates:        │  │ Creates:          │  │ Manages:         │  │ │
│  │  │ - StatefulSets  │  │ - Primary Svc     │  │ - Defaults       │  │ │
│  │  │ - Services      │  │ - Secondary Svc   │  │ - Image versions │  │ │
│  │  │ - Secrets       │  │ - Endpoints       │  │                  │  │ │
│  │  └─────────────────┘  └───────────────────┘  └──────────────────┘  │ │
│  └────────────────────────────────────────────────────────────────────┘ │
│                                    │                                     │
│                                    ▼                                     │
│  ┌────────────────────────────────────────────────────────────────────┐ │
│  │                    SQL Server StatefulSet                          │ │
│  │                                                                     │ │
│  │  ┌───────────────────────────────────────────────────────────────┐ │ │
│  │  │ Pod 0 (Primary)                                                │ │ │
│  │  │  ┌─────────────────┐ ┌─────────────────┐ ┌─────────────────┐  │ │ │
│  │  │  │ SQL Server      │ │ AG Helper       │ │ SQL Exporter    │  │ │ │
│  │  │  │                 │ │ Sidecar         │ │                 │  │ │ │
│  │  │  │ Port: 1433      │ │ Port: 8080      │ │ Port: 9399      │  │ │ │
│  │  │  │                 │ │                 │ │                 │  │ │ │
│  │  │  │ HADR Enabled    │ │ Health States:  │ │ Metrics:        │  │ │ │
│  │  │  │ AG Databases    │ │ - Waiting       │ │ - CPU/Memory    │  │ │ │
│  │  │  │ Endpoints       │ │ - Healthy       │ │ - Connections   │  │ │ │
│  │  │  │                 │ │ - Warning       │ │ - Batch/sec     │  │ │ │
│  │  │  │                 │ │ - Critical      │ │                 │  │ │ │
│  │  │  └─────────────────┘ └─────────────────┘ └─────────────────┘  │ │ │
│  │  └───────────────────────────────────────────────────────────────┘ │ │
│  │                                                                     │ │
│  │  ┌───────────────────────────────────────────────────────────────┐ │ │
│  │  │ Pod 1, 2, ... (Secondaries) - Same container structure        │ │ │
│  │  └───────────────────────────────────────────────────────────────┘ │ │
│  └────────────────────────────────────────────────────────────────────┘ │
│                                                                          │
│  ┌────────────────────────────────────────────────────────────────────┐ │
│  │                         Services                                    │ │
│  │  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────────┐ │ │
│  │  │ <ag>-primary    │  │ <ag>-secondary  │  │ Prometheus/Grafana  │ │ │
│  │  │ LoadBalancer    │  │ LoadBalancer    │  │ Monitoring Stack    │ │ │
│  │  │ :1433           │  │ :1434           │  │ :9090 / :3000       │ │ │
│  │  └─────────────────┘  └─────────────────┘  └─────────────────────┘ │ │
│  └────────────────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────────────────┘
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

## Availability Group Architecture

### Overview

The operator uses a **sidecar pattern** for Availability Group management:

```
┌────────────────────────────────────────────────────────────────────┐
│  SQL Server Pod                                                     │
│  ┌──────────────────┐  ┌──────────────────┐  ┌──────────────────┐  │
│  │   SQL Server     │  │   AG Helper      │  │  SQL Exporter    │  │
│  │   Container      │  │   Sidecar        │  │  (Metrics)       │  │
│  │                  │  │                  │  │                  │  │
│  │  - HADR enabled  │  │  - Monitors AG   │  │  - Prometheus    │  │
│  │  - AG databases  │  │  - Failover API  │  │    metrics       │  │
│  │  - Endpoints     │  │  - Health probes │  │                  │  │
│  └──────────────────┘  └──────────────────┘  └──────────────────┘  │
│           │                    │                                    │
│           └────── localhost ───┘                                    │
└────────────────────────────────────────────────────────────────────┘
```

### AG Helper Sidecar Behavior

The AG Helper sidecar is designed to be **AG-tolerant** - it gracefully handles the case where the Availability Group hasn't been configured yet.

#### Health States

| State | Description | Liveness Probe | Readiness Probe |
|-------|-------------|----------------|-----------------|
| **Waiting** | AG not configured yet, sidecar is waiting | ✅ Pass (200) | ❌ Fail (503) |
| **Healthy** | AG configured, all replicas synchronized | ✅ Pass (200) | ✅ Pass (200) |
| **Warning** | AG configured, some replicas synchronizing | ✅ Pass (200) | ✅ Pass (200) |
| **Critical** | AG exists but is broken or unreachable | ❌ Fail (503) | ❌ Fail (503) |

#### Sidecar HTTP Endpoints

| Endpoint | Purpose | Use Case |
|----------|---------|----------|
| `/health` or `/healthz` | Liveness check - passes even when waiting | Kubernetes liveness probe |
| `/ready` or `/readyz` | Readiness check - only passes when AG works | Kubernetes readiness probe |
| `/state` | Full AG state (JSON) | Debugging, monitoring |
| `/role` | Current replica role (PRIMARY/SECONDARY) | Service routing decisions |
| `/failover` | Trigger manual failover (POST) | Administrative operations |
| `/sequence` | Last hardened LSN for failover decisions | Failover target selection |
| `/ags` | List all discovered AGs | Multi-AG environments |
| `/state/{agName}` | Get specific AG state | Per-AG monitoring |
| `/discover` | Force immediate AG discovery | Debugging, testing |

#### Multi-AG Discovery Mode

The AG Helper supports two monitoring modes:

1. **Single AG Mode** (default): Specify `-ag-name=MyAG` to monitor a specific AG
2. **Auto-Discovery Mode**: Omit `-ag-name` to automatically discover and monitor ALL AGs

In auto-discovery mode, the sidecar:
- Queries `sys.availability_groups` periodically to find all AGs
- Automatically detects newly added AGs without pod restart
- Logs when new AGs are detected or removed
- Reports aggregate health across all AGs

**Example: Auto-Discovery Mode**
```yaml
containers:
  - name: ag-helper
    image: sqlserver-operator/ag-helper:latest
    args:
      # Omit -ag-name for auto-discovery mode
      - "-sql-host=localhost"
      - "-sql-port=1433"
```

**Multi-AG Endpoints:**
```bash
# List all AGs
curl http://pod-ip:8080/ags
# Response: {"count":2,"ags":{"ProductionAG":{"health":"Healthy"},"ReportingAG":{"health":"Healthy"}}}

# Get specific AG
curl http://pod-ip:8080/state/ProductionAG
# Response: {"agName":"ProductionAG","health":"Healthy","role":"PRIMARY",...}
```

#### What This Means for Deployment

1. **Pods start immediately**: SQL Server starts without waiting for AG
2. **Sidecar waits gracefully**: Reports "Waiting" status, not errors
3. **No log spam**: Minimal logging until AG is configured
4. **Readiness gates traffic**: K8s services only route to pods after AG is ready
5. **Dynamic AG discovery**: New AGs are detected automatically (in auto-discovery mode)

### AG Deployment Workflow

```
┌──────────────────────────────────────────────────────────────────┐
│ 1. Deploy SQLServer CR (pods start with AG Helper sidecar)      │
│    kubectl apply -f samples/sqlserver-availability-group.yaml   │
│                                                                  │
│    Status: Pods running, AG Helper reports "Waiting"            │
│    Liveness: ✅ PASS  |  Readiness: ❌ FAIL                      │
└──────────────────────────────┬───────────────────────────────────┘
                               │
                               ▼
┌──────────────────────────────────────────────────────────────────┐
│ 2. Deploy SQLServerAG CR (creates K8s Services)                 │
│    - Primary service: routes to current primary replica         │
│    - Secondary service: routes to readable secondaries          │
│                                                                  │
│    Status: Services created, waiting for AG configuration       │
└──────────────────────────────┬───────────────────────────────────┘
                               │
                               ▼
┌──────────────────────────────────────────────────────────────────┐
│ 3. Configure AG via T-SQL (see samples/scripts/)                │
│    - Run setup-availability-group.sql on primary                │
│    - Run join-secondary.sql on each secondary                   │
│                                                                  │
│    Status: AG configured, databases seeding                     │
└──────────────────────────────┬───────────────────────────────────┘
                               │
                               ▼
┌──────────────────────────────────────────────────────────────────┐
│ 4. AG Helper detects AG automatically                           │
│    - Transitions from "Waiting" to "Healthy"                    │
│    - Readiness probe starts passing                             │
│    - Services begin routing traffic                             │
│                                                                  │
│    Status: Fully operational                                    │
│    Liveness: ✅ PASS  |  Readiness: ✅ PASS                      │
└──────────────────────────────────────────────────────────────────┘
```

### Scenario: Adding a New Availability Group

If you have an existing SQL Server deployment and want to add a new AG:

#### Step 1: Update SQLServer CR to Enable HADR

If not already enabled, update your SQLServer resource:

```yaml
spec:
  instance:
    replicas: 3  # Increase if needed
    config:
      hadrEnabled: true  # Required for AG
      agentEnabled: true
```

```bash
kubectl apply -f your-sqlserver.yaml
```

#### Step 2: Create the SQLServerAG Resource

```yaml
apiVersion: mssql.microsoft.com/v1alpha1
kind: SQLServerAG
metadata:
  name: new-ag-01
  namespace: mssql
spec:
  description: "New AG for reporting workloads"
  sqlServerRef:
    name: your-sqlserver  # Reference to existing SQLServer
  availabilityGroup:
    name: ReportingAG
    replicas: 3
    databases:
      - name: ReportingDB
  endpoints:
    primary:
      type: LoadBalancer
      port: 1433
    secondary:
      type: LoadBalancer
      port: 1434
```

```bash
kubectl apply -f new-ag.yaml
```

#### Step 3: Configure AG via T-SQL

Connect to the primary replica and create the AG:

```bash
# Connect to primary
kubectl exec -it your-sqlserver-0 -n mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P 'YourPassword' -C
```

```sql
-- Create database
CREATE DATABASE ReportingDB;
ALTER DATABASE ReportingDB SET RECOVERY FULL;
BACKUP DATABASE ReportingDB TO DISK = '/var/opt/mssql/backup/ReportingDB.bak';

-- Create AG (see samples/scripts/setup-availability-group.sql for full script)
CREATE AVAILABILITY GROUP ReportingAG
    WITH (CLUSTER_TYPE = EXTERNAL, DB_FAILOVER = ON)
    FOR DATABASE ReportingDB
    REPLICA ON ...;
```

#### Step 4: Join Secondary Replicas

```bash
# On each secondary
kubectl exec -it your-sqlserver-1 -n mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P 'YourPassword' -C \
  -Q "ALTER AVAILABILITY GROUP ReportingAG JOIN WITH (CLUSTER_TYPE = EXTERNAL);"
```

#### Step 5: Verify AG Helper Detected the AG

```bash
# Check sidecar health
kubectl exec -it your-sqlserver-0 -n mssql -c ag-helper -- \
  curl -s localhost:8080/state | jq

# Should show:
# {
#   "agName": "ReportingAG",
#   "role": "PRIMARY",
#   "health": "Healthy",
#   ...
# }
```

### Scenario: Multiple Availability Groups

SQL Server supports multiple AGs on the same replicas. The AG Helper supports this in two ways:

#### Option 1: Auto-Discovery Mode (Recommended)

Omit the `-ag-name` flag to enable auto-discovery mode. The sidecar will automatically discover and monitor ALL AGs:

```yaml
# In your StatefulSet pod spec:
containers:
  - name: ag-helper
    args:
      # Omit -ag-name for auto-discovery
      - "-sql-host=localhost"
      - "-sql-port=1433"
```

With auto-discovery:
- **New AGs are detected automatically** - no pod restart needed
- **Removed AGs are cleaned up** - state map updated on next discovery
- **Aggregate health reported** - worst health across all AGs determines overall health
- **Per-AG APIs available** - use `/state/{agName}` for individual AG status

**Check all AGs:**
```bash
kubectl exec -it your-sqlserver-0 -n mssql -c ag-helper -- \
  curl -s localhost:8080/ags

# Response:
# {
#   "count": 2,
#   "ags": {
#     "ProductionAG": {"health": "Healthy", "role": "PRIMARY", ...},
#     "ReportingAG": {"health": "Healthy", "role": "SECONDARY", ...}
#   }
# }
```

#### Option 2: Single AG Mode

Specify `-ag-name` to monitor only one specific AG:

```yaml
containers:
  - name: ag-helper
    args:
      - "-ag-name=ProductionAG"  # Only monitor this AG
      - "-sql-host=localhost"
```

#### Adding a New AG to an Existing Deployment

With auto-discovery mode enabled:

1. **Create the new AG via T-SQL** on the primary replica
2. **The sidecar detects it** within the next monitoring interval (default: 10s)
3. **Check detection**: `curl localhost:8080/ags`
4. **Create SQLServerAG resource** for the new AG's K8s services

No pod restarts required!

### Automatic Failover

The operator supports **controller-driven automatic failover** when the primary replica becomes unavailable.

#### How It Works

```
┌─────────────────────────────────────────────────────────────────────────────┐
│                    CONTROLLER-DRIVEN AUTOMATIC FAILOVER                     │
└─────────────────────────────────────────────────────────────────────────────┘

        Normal Operation                 Primary Failure Detected
        ─────────────────                ────────────────────────

    ┌─────────────────────┐         ┌─────────────────────────────┐
    │ Controller monitors │   ──►   │ No PRIMARY role detected    │
    │ all AG Helper pods  │         │ from any sidecar            │
    │ via /state endpoint │         └──────────────┬──────────────┘
    └─────────────────────┘                        │
                                                   ▼
                                    ┌─────────────────────────────┐
                                    │ 30-second grace period      │
                                    │ (configurable)              │
                                    └──────────────┬──────────────┘
                                                   │
                                                   ▼
                          ┌────────────────────────────────────────────┐
                          │ Select best failover candidate:           │
                          │  1. Highest sequence number (least loss)  │
                          │  2. Prefer SYNCHRONIZED over SYNCHRONIZING│
                          │  3. Prefer Healthy over Warning           │
                          └──────────────────┬─────────────────────────┘
                                             │
                                             ▼
                          ┌────────────────────────────────────────────┐
                          │ POST /failover to selected candidate      │
                          │ (with allowDataLoss based on sync state)  │
                          └──────────────────┬─────────────────────────┘
                                             │
                                             ▼
                          ┌────────────────────────────────────────────┐
                          │ New PRIMARY established                   │
                          │ Kubernetes Events recorded                │
                          │ 60-second cooldown before next failover   │
                          └────────────────────────────────────────────┘
```

#### Configuration

Enable automatic failover in your SQLServerAG spec:

```yaml
spec:
  availabilityGroup:
    name: ProductionAG
    automaticFailover: true  # Enable controller-driven failover
    
  failover:
    automatic: true
    dataLossThreshold: 0        # Only allow synchronized failover
    healthCheckTimeout: "30s"   # Grace period before failover
```

#### Failover Selection Algorithm

When multiple secondaries are available, the controller selects the best candidate:

| Priority | Criteria | Reason |
|----------|----------|--------|
| 1 | Highest sequence number | Minimizes data loss |
| 2 | SYNCHRONIZED state | No data loss possible |
| 3 | Healthy status | Stable replica |

#### Events During Failover

The controller emits Kubernetes events for visibility:

```bash
kubectl get events -n mssql --field-selector involvedObject.name=prod-ag-01

# Example events:
# NoPrimaryDetected    No primary replica detected, will failover in 30s if not recovered
# ForceFailover        Forcing failover to sql-ag-prod01-1 with potential data loss
# FailoverCompleted    Automatic failover completed to sql-ag-prod01-1
# PrimaryRecovered     Primary replica is available again
```

#### Disabling Automatic Failover

For manual-only failover control:

```yaml
spec:
  availabilityGroup:
    automaticFailover: false  # Disable automatic failover
```

With automatic failover disabled, you can still manually trigger failover:

```bash
# Manually failover to a specific pod
kubectl exec -it sql-ag-prod01-1 -n mssql -c ag-helper -- \
  curl -X POST localhost:8080/failover -d '{"allowDataLoss": false}'
```

### Troubleshooting AG Helper

#### Check Sidecar Status

```bash
# View AG Helper logs
kubectl logs your-sqlserver-0 -n mssql -c ag-helper

# Check health endpoint
kubectl exec -it your-sqlserver-0 -n mssql -c ag-helper -- \
  curl -s localhost:8080/health

# Get full state
kubectl exec -it your-sqlserver-0 -n mssql -c ag-helper -- \
  curl -s localhost:8080/state
```

#### Common Issues

| Symptom | Cause | Solution |
|---------|-------|----------|
| Health: "Waiting" indefinitely | AG not configured via T-SQL | Run setup scripts |
| Health: "Critical" | AG exists but this replica can't connect | Check SQL Server logs, endpoints |
| Readiness failing | AG not synchronized | Wait for seeding, check `sys.dm_hadr_database_replica_states` |
| Role shows "NOT_AVAILABLE" | HADR not enabled or AG doesn't exist | Enable `hadrEnabled: true` in spec |
| Failover not triggering | `automaticFailover: false` | Enable in spec |
| Multiple failovers in quick succession | Cooldown period active | Wait 60s between failovers |

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

## Packaging & Distribution

This section covers how to package the SQL Server Kubernetes Operator for distribution to other users in your organization. After following these steps, users can deploy SQL Server containers without building anything from scratch.

### Overview

The distribution package consists of:

| Component | Description | How to Generate |
|-----------|-------------|-----------------|
| **Operator Image** | Controller managing SQL Server resources | `docker build` |
| **AG Helper Image** | Sidecar for Availability Group monitoring | `docker build` |
| **CRD Manifests** | Custom Resource Definitions | `controller-gen` |
| **Deployment Manifests** | RBAC, ServiceAccount, Deployment | Pre-built in `deploy/` |
| **Sample Configurations** | Example SQL Server deployments | Pre-built in `samples/` |

### Step 1: Build Container Images

#### Prerequisites

- Go 1.22+ (for building)
- Docker or Podman (for container images)
- Access to a container registry (ACR, Docker Hub, ECR, etc.)

#### Build the Operator Image

```bash
# Set your registry (examples)
export REGISTRY=myregistry.azurecr.io/sqlserver-operator
# or: export REGISTRY=docker.io/myorg/sqlserver-operator
# or: export REGISTRY=ghcr.io/myorg/sqlserver-operator

export VERSION=1.0.0

# Build the operator image
docker build -t ${REGISTRY}/operator:${VERSION} -f Dockerfile.operator .

# Build the AG Helper sidecar image
docker build -t ${REGISTRY}/ag-helper:${VERSION} -f Dockerfile.ag-helper .

# Tag as latest
docker tag ${REGISTRY}/operator:${VERSION} ${REGISTRY}/operator:latest
docker tag ${REGISTRY}/ag-helper:${VERSION} ${REGISTRY}/ag-helper:latest
```

#### Push to Container Registry

```bash
# Login to your registry (example for ACR)
az acr login --name myregistry

# Push images
docker push ${REGISTRY}/operator:${VERSION}
docker push ${REGISTRY}/operator:latest
docker push ${REGISTRY}/ag-helper:${VERSION}
docker push ${REGISTRY}/ag-helper:latest
```

### Step 2: Generate CRD Manifests

Use `controller-gen` to generate the Custom Resource Definition YAML files:

```bash
# Install controller-gen if not already installed
go install sigs.k8s.io/controller-tools/cmd/controller-gen@latest

# Generate CRDs
controller-gen crd paths="./pkg/apis/mssql.microsoft.com/v1alpha1" output:crd:dir=./deploy/crds

# Verify generated CRDs
ls -la deploy/crds/
# Expected files:
#   mssql.microsoft.com_sqlservers.yaml
#   mssql.microsoft.com_sqlserverags.yaml
#   mssql.microsoft.com_operatorconfigurations.yaml
```

### Step 3: Update Deployment Manifests

Update the operator deployment to use your registry:

```bash
# Update image references in deployment manifest
sed -i "s|image: .*operator:.*|image: ${REGISTRY}/operator:${VERSION}|g" deploy/operator.yaml
sed -i "s|image: .*ag-helper:.*|image: ${REGISTRY}/ag-helper:${VERSION}|g" deploy/operator.yaml
```

Or manually edit [deploy/operator.yaml](deploy/operator.yaml):

```yaml
spec:
  containers:
    - name: operator
      image: myregistry.azurecr.io/sqlserver-operator/operator:1.0.0  # Update this
      env:
        - name: AG_HELPER_IMAGE
          value: myregistry.azurecr.io/sqlserver-operator/ag-helper:1.0.0  # And this
```

### Step 4: Create Distribution Package

#### Option A: Simple ZIP/TAR Package

Create a distribution archive with all necessary files:

```bash
# Create distribution directory
mkdir -p dist/sqlserver-operator-${VERSION}

# Copy required files
cp -r deploy/ dist/sqlserver-operator-${VERSION}/
cp -r samples/ dist/sqlserver-operator-${VERSION}/
cp README.md dist/sqlserver-operator-${VERSION}/
cp LICENSE dist/sqlserver-operator-${VERSION}/

# Create installation script
cat > dist/sqlserver-operator-${VERSION}/install.sh << 'EOF'
#!/bin/bash
set -e

echo "Installing SQL Server Kubernetes Operator..."

# Create namespace
kubectl create namespace mssql-operator --dry-run=client -o yaml | kubectl apply -f -
kubectl create namespace mssql --dry-run=client -o yaml | kubectl apply -f -

# Install CRDs
kubectl apply -f deploy/crds/

# Install RBAC and ServiceAccounts
kubectl apply -f deploy/rbac.yaml
kubectl apply -f deploy/serviceaccount.yaml

# Deploy operator
kubectl apply -f deploy/operator.yaml

# Wait for operator to be ready
echo "Waiting for operator to be ready..."
kubectl wait --for=condition=available --timeout=120s deployment/mssql-operator -n mssql-operator

echo "✅ SQL Server Operator installed successfully!"
echo ""
echo "Next steps:"
echo "  1. Create a secret for SA password:"
echo "     kubectl create secret generic sql-sa-secret --from-literal=password='YourStrong@Passw0rd!' -n mssql"
echo ""
echo "  2. Deploy a SQL Server instance:"
echo "     kubectl apply -f samples/sqlserver-2025-standalone.yaml"
EOF
chmod +x dist/sqlserver-operator-${VERSION}/install.sh

# Create uninstall script
cat > dist/sqlserver-operator-${VERSION}/uninstall.sh << 'EOF'
#!/bin/bash
set -e

echo "Uninstalling SQL Server Kubernetes Operator..."

# Delete operator
kubectl delete -f deploy/operator.yaml --ignore-not-found

# Delete RBAC
kubectl delete -f deploy/rbac.yaml --ignore-not-found
kubectl delete -f deploy/serviceaccount.yaml --ignore-not-found

# Delete CRDs (WARNING: This deletes all SQL Server resources!)
read -p "Delete CRDs? This will delete ALL SQLServer resources! (y/N): " confirm
if [[ "$confirm" == "y" || "$confirm" == "Y" ]]; then
    kubectl delete -f deploy/crds/ --ignore-not-found
fi

echo "✅ Uninstall complete"
EOF
chmod +x dist/sqlserver-operator-${VERSION}/uninstall.sh

# Create archive
cd dist
tar -czvf sqlserver-operator-${VERSION}.tar.gz sqlserver-operator-${VERSION}/
zip -r sqlserver-operator-${VERSION}.zip sqlserver-operator-${VERSION}/

echo "Distribution packages created:"
ls -la sqlserver-operator-${VERSION}.*
```

#### Option B: Helm Chart (Recommended for Enterprise)

Create a Helm chart for more flexible deployment:

```bash
# Create Helm chart structure
mkdir -p charts/sqlserver-operator/templates
mkdir -p charts/sqlserver-operator/crds

# Copy CRDs to Helm crds directory (auto-installed by Helm)
cp deploy/crds/*.yaml charts/sqlserver-operator/crds/

# Create Chart.yaml
cat > charts/sqlserver-operator/Chart.yaml << EOF
apiVersion: v2
name: sqlserver-operator
description: A Kubernetes operator for managing SQL Server instances
type: application
version: ${VERSION}
appVersion: "${VERSION}"
keywords:
  - sqlserver
  - database
  - operator
  - mssql
maintainers:
  - name: Your Team
    email: team@example.com
EOF

# Create values.yaml
cat > charts/sqlserver-operator/values.yaml << EOF
# Operator configuration
operator:
  image:
    repository: myregistry.azurecr.io/sqlserver-operator/operator
    tag: "${VERSION}"
    pullPolicy: IfNotPresent
  
  replicas: 1
  
  resources:
    limits:
      cpu: 500m
      memory: 512Mi
    requests:
      cpu: 100m
      memory: 128Mi

# AG Helper sidecar configuration
agHelper:
  image:
    repository: myregistry.azurecr.io/sqlserver-operator/ag-helper
    tag: "${VERSION}"
    pullPolicy: IfNotPresent

# Namespace for SQL Server instances
sqlNamespace: mssql

# Image pull secrets (if using private registry)
imagePullSecrets: []
  # - name: regcred

# RBAC settings
rbac:
  create: true

# ServiceAccount settings
serviceAccount:
  create: true
  name: mssql-operator
EOF

# Package Helm chart
helm package charts/sqlserver-operator -d dist/
```

#### Option C: OCI Artifact (Modern Approach)

Push manifests as OCI artifacts for GitOps workflows:

```bash
# Package as OCI artifact using ORAS
oras push ${REGISTRY}/manifests:${VERSION} \
  --config /dev/null:application/vnd.unknown.config.v1+json \
  deploy/:application/vnd.cncf.helm.config.v1+json \
  samples/:application/vnd.cncf.helm.config.v1+json
```

### Step 5: Document for End Users

Create a quick-start guide for your organization:

```markdown
# SQL Server Kubernetes Operator - Quick Start

## Prerequisites
- Kubernetes cluster (1.25+)
- kubectl configured
- Access to container registry: myregistry.azurecr.io

## Installation

### Option 1: Using install script
\`\`\`bash
tar -xzf sqlserver-operator-1.0.0.tar.gz
cd sqlserver-operator-1.0.0
./install.sh
\`\`\`

### Option 2: Using Helm
\`\`\`bash
helm install sqlserver-operator ./sqlserver-operator-1.0.0.tgz \
  --namespace mssql-operator \
  --create-namespace
\`\`\`

### Option 3: Manual installation
\`\`\`bash
kubectl apply -f deploy/crds/
kubectl apply -f deploy/rbac.yaml
kubectl apply -f deploy/serviceaccount.yaml
kubectl apply -f deploy/operator.yaml
\`\`\`

## Deploy Your First SQL Server

1. Create SA password secret:
\`\`\`bash
kubectl create namespace mssql
kubectl create secret generic sql-sa-secret \
  --from-literal=password='YourStrong@Passw0rd!' \
  -n mssql
\`\`\`

2. Deploy SQL Server:
\`\`\`bash
kubectl apply -f samples/sqlserver-2025-standalone.yaml
\`\`\`

3. Verify deployment:
\`\`\`bash
kubectl get sqlserver -n mssql
kubectl get pods -n mssql
\`\`\`
```

### Distribution Checklist

Before sharing with your organization, verify:

- [ ] Container images pushed to registry and accessible
- [ ] Image pull secrets configured (if private registry)
- [ ] CRDs generated and included in package
- [ ] Deployment manifests updated with correct image references
- [ ] Sample configurations tested and working
- [ ] Install/uninstall scripts tested
- [ ] Documentation reviewed and accurate
- [ ] Version numbers consistent across all files

### Registry Authentication

If using a private registry, users need to configure image pull secrets:

```bash
# Create image pull secret
kubectl create secret docker-registry regcred \
  --docker-server=myregistry.azurecr.io \
  --docker-username=<username> \
  --docker-password=<password> \
  --namespace mssql-operator

kubectl create secret docker-registry regcred \
  --docker-server=myregistry.azurecr.io \
  --docker-username=<username> \
  --docker-password=<password> \
  --namespace mssql

# Reference in deployments
kubectl patch serviceaccount default -n mssql \
  -p '{"imagePullSecrets": [{"name": "regcred"}]}'
```

### Versioning Strategy

Recommended versioning approach:

| Version | When to Use |
|---------|-------------|
| `1.0.0` | Initial stable release |
| `1.0.1` | Bug fixes only |
| `1.1.0` | New features, backward compatible |
| `2.0.0` | Breaking changes |
| `latest` | Always points to most recent stable |
| `dev` | Development/testing builds |

```bash
# Tag and push multiple versions
docker tag ${REGISTRY}/operator:${VERSION} ${REGISTRY}/operator:latest
docker tag ${REGISTRY}/operator:${VERSION} ${REGISTRY}/operator:1.0
docker tag ${REGISTRY}/operator:${VERSION} ${REGISTRY}/operator:1

docker push ${REGISTRY}/operator:${VERSION}
docker push ${REGISTRY}/operator:latest
docker push ${REGISTRY}/operator:1.0
docker push ${REGISTRY}/operator:1
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
