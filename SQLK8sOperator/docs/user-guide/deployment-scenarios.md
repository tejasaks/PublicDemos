# Deployment Scenarios

[← Back to User Guide](../README.md) | [Documentation Home](../README.md)

This guide covers different SQL Server deployment patterns for various use cases.

## Table of Contents

- [Standalone Development](#standalone-development)
- [Standalone Production](#standalone-production)
- [High Availability with AG](#high-availability-with-ag)
- [Active Directory Authentication](#active-directory-authentication)
- [Multi-Tenant Deployments](#multi-tenant-deployments)
- [External Access](#external-access)

## Standalone Development

A minimal configuration for development and testing.

### Characteristics
- Single replica
- Minimal resources
- Developer edition (free)
- No high availability

### Deployment Steps

**Step 1: Create the namespace**

```bash
kubectl create namespace mssql
```

**Expected output:**
```
namespace/mssql created
```

**Step 2: Create the SA password secret**

```bash
kubectl create secret generic sql-dev-01-sa \
  --from-literal=password='YourStr0ngP@ssword!' \
  -n mssql
```

**Expected output:**
```
secret/sql-dev-01-sa created
```

**Step 3: Create the SQLServer manifest file**

Create a file named `sql-dev-standalone.yaml`:

```bash
# On Linux/macOS
nano sql-dev-standalone.yaml

# On Windows (PowerShell)
notepad sql-dev-standalone.yaml
```

Paste the following content and save:

### Sample Manifest

```yaml
apiVersion: mssql.microsoft.com/v1alpha1
kind: SQLServer
metadata:
  name: sql-dev-01
  namespace: mssql
spec:
  description: "Development SQL Server for local testing"
  version: "2022"
  edition: Developer
  instance:
    replicas: 1
    resources:
      limits:
        cpu: "2"
        memory: 4Gi
      requests:
        cpu: "500m"
        memory: 2Gi
    storage:
      data:
        size: 10Gi
  credentials:
    saPasswordSecretRef:
      name: sql-dev-01-sa
      key: password
  service:
    type: ClusterIP
    port: 1433
```

**Step 4: Apply the manifest**

```bash
kubectl apply -f sql-dev-standalone.yaml
```

**Expected output:**
```
sqlserver.mssql.microsoft.com/sql-dev-01 created
```

**Step 5: Wait for the pod to be ready**

```bash
kubectl get pods -n mssql -w

# Wait until the pod shows Running and 1/1 Ready:
# NAME           READY   STATUS    RESTARTS   AGE
# sql-dev-01-0   1/1     Running   0          2m
```

**Step 6: Verify connectivity**

```bash
# Port-forward to the SQL Server
kubectl port-forward svc/sql-dev-01 -n mssql 1433:1433

# In another terminal, test connection
sqlcmd -S localhost,1433 -U sa -P 'YourStr0ngP@ssword!' -C -Q "SELECT @@VERSION"
```

### When to Use
- Local development
- CI/CD pipelines
- Feature testing
- Learning/experimentation

## Standalone Production

A production-ready standalone instance with proper resource allocation.

### Characteristics
- Single replica (no HA)
- Production resource limits
- Separate storage volumes
- Monitoring enabled
- TLS optional

### Deployment Steps

**Step 1: Create the namespace**

```bash
kubectl create namespace production
```

**Expected output:**
```
namespace/production created
```

**Step 2: Create the SA password secret with a strong password**

```bash
kubectl create secret generic sql-prod-01-sa \
  --from-literal=password='$(openssl rand -base64 24)' \
  -n production
```

**Expected output:**
```
secret/sql-prod-01-sa created
```

**Step 3: Create the SQLServer manifest file**

Create a file named `sql-prod-standalone.yaml`:

```bash
nano sql-prod-standalone.yaml
```

Paste the following content and save:

### Sample Manifest

```yaml
apiVersion: mssql.microsoft.com/v1alpha1
kind: SQLServer
metadata:
  name: sql-prod-01
  namespace: production
spec:
  description: "Production SQL Server for application backend"
  version: "2022"
  edition: Standard  # Or Enterprise for advanced features
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
        storageClass: fast-ssd
      tempdb:
        size: 20Gi
        storageClass: fast-ssd
      backup:
        size: 200Gi
        storageClass: standard
    config:
      agentEnabled: true
      memoryLimitMB: 14336  # Leave 2GB for OS
      traceFlags: [1222, 3226]  # Deadlock logging, suppress backup success
  credentials:
    saPasswordSecretRef:
      name: sql-prod-01-sa
      key: password
  service:
    type: LoadBalancer
    port: 1433
  monitoring:
    enabled: true
```

**Step 4: Apply the manifest**

```bash
kubectl apply -f sql-prod-standalone.yaml
```

**Expected output:**
```
sqlserver.mssql.microsoft.com/sql-prod-01 created
```

**Step 5: Verify the deployment**

```bash
kubectl get sqlserver -n production

# Expected output:
# NAME          VERSION   EDITION    STATUS   AGE
# sql-prod-01   2022      Standard   Ready    2m

kubectl get pods -n production

# Expected output:
# NAME            READY   STATUS    RESTARTS   AGE
# sql-prod-01-0   1/1     Running   0          2m
```

**Step 6: Get the LoadBalancer external IP**

```bash
kubectl get svc sql-prod-01 -n production

# Expected output (after a minute for LB provisioning):
# NAME          TYPE           CLUSTER-IP    EXTERNAL-IP      PORT(S)          AGE
# sql-prod-01   LoadBalancer   10.96.1.100   203.0.113.100    1433:31433/TCP   2m
```

### When to Use
- Single-server workloads
- Applications tolerant of brief downtime
- Cost-sensitive deployments
- Workloads that don't require HA

## High Availability with AG

A 3-replica Availability Group deployment for high availability.

### Characteristics
- 3 synchronous replicas
- Automatic failover
- Read-scale secondaries
- Zero data loss

### Deployment Steps

**Step 1: Create the SA password secret**

```bash
kubectl create secret generic sql-ha-01-sa \
  --from-literal=password='$(openssl rand -base64 24)' \
  -n production
```

**Expected output:**
```
secret/sql-ha-01-sa created
```

**Step 2: Create the manifest file**

Create a file named `sql-ha-with-ag.yaml`:

```bash
nano sql-ha-with-ag.yaml
```

Paste the following content and save:

### Sample Manifest

See [Availability Groups Deployment Guide](../availability-groups/deployment-guide.md) for complete setup.

```yaml
apiVersion: mssql.microsoft.com/v1alpha1
kind: SQLServer
metadata:
  name: sql-ha-01
  namespace: production
spec:
  version: "2022"
  edition: Developer  # Enterprise for production
  instance:
    replicas: 3
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
      log:
        size: 50Gi
    config:
      hadrEnabled: true
      agentEnabled: true
    affinity:
      podAntiAffinity:
        requiredDuringSchedulingIgnoredDuringExecution:
          - labelSelector:
              matchLabels:
                app: mssql
            topologyKey: kubernetes.io/hostname
  credentials:
    saPasswordSecretRef:
      name: sql-ha-01-sa
      key: password
---
apiVersion: mssql.microsoft.com/v1alpha1
kind: SQLServerAG
metadata:
  name: ha-ag-01
  namespace: production
spec:
  sqlServerRef:
    name: sql-ha-01
  availabilityGroup:
    name: ProductionAG
    replicas: 3
    automaticFailover: true
    primaryConfig:
      availabilityMode: SynchronousCommit
    secondaryConfig:
      availabilityMode: SynchronousCommit
      readableSecondary: ReadOnly
  endpoints:
    primary:
      type: LoadBalancer
      port: 1433
    secondary:
      type: LoadBalancer
      port: 1434
```

**Step 3: Apply the manifest**

```bash
kubectl apply -f sql-ha-with-ag.yaml
```

**Expected output:**
```
sqlserver.mssql.microsoft.com/sql-ha-01 created
sqlserverag.mssql.microsoft.com/ha-ag-01 created
```

**Step 4: Wait for all replicas to be ready**

```bash
kubectl get pods -n production -w

# Wait until all 3 pods show Running and 1/1 Ready:
# NAME           READY   STATUS    RESTARTS   AGE
# sql-ha-01-0    1/1     Running   0          3m
# sql-ha-01-1    1/1     Running   0          2m
# sql-ha-01-2    1/1     Running   0          1m
```

**Step 5: Verify the Availability Group status**

```bash
kubectl get sqlserverag -n production

# Expected output:
# NAME        SQLSERVER    AG NAME       STATUS   PRIMARY       AGE
# ha-ag-01    sql-ha-01    ProductionAG  Ready    sql-ha-01-0   5m
```

**Step 6: Get the LoadBalancer endpoints**

```bash
kubectl get svc -n production | grep sql-ha

# Expected output:
# sql-ha-01-primary    LoadBalancer   10.96.1.100   203.0.113.100    1433:31433/TCP   5m
# sql-ha-01-secondary  LoadBalancer   10.96.1.101   203.0.113.101    1434:31434/TCP   5m
```

### When to Use
- Mission-critical applications
- Zero-downtime requirements
- Read-scale workloads
- Disaster recovery requirements

## Active Directory Authentication

Enterprise deployment with Windows Authentication via Kerberos.

### Prerequisites
- Active Directory domain
- Service account with SPN permissions
- Network connectivity to domain controllers
- CoreDNS configured for AD DNS forwarding

### Sample Manifest

```yaml
apiVersion: mssql.microsoft.com/v1alpha1
kind: SQLServer
metadata:
  name: sql-ad-01
  namespace: enterprise
spec:
  version: "2022"
  edition: Enterprise
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
    config:
      tlsEnabled: true
      tlsCertSecretRef:
        name: mssql-tls-cert
  credentials:
    saPasswordSecretRef:
      name: sql-ad-01-sa
      key: password
  activeDirectory:
    enabled: true
    realm: CONTOSO.COM
    domainControllers:
      - dc1.contoso.com
      - dc2.contoso.com
    serviceAccountSecretRef:
      name: mssql-ad-service-account
    netBIOSDomain: CONTOSO
    dnsSuffix: contoso.com
    adminGroup: SQLServerAdmins
  service:
    type: LoadBalancer
    port: 1433
```

See [Active Directory Setup](../operations/active-directory.md) for complete configuration.

## Multi-Tenant Deployments

Running multiple SQL Server instances for different teams or applications.

### Namespace Isolation

Each team gets their own namespace with isolated SQL Server instances.

**Step 1: Create namespaces for each team**

```bash
kubectl create namespace team-a-sql
kubectl create namespace team-b-sql
```

**Expected output:**
```
namespace/team-a-sql created
namespace/team-b-sql created
```

**Step 2: Create the multi-tenant manifest file**

Create a file named `multi-tenant-sql.yaml`:

```bash
nano multi-tenant-sql.yaml
```

Paste the following content and save:

```yaml
# Team A namespace (already created)
---
# Team A SA password secret
apiVersion: v1
kind: Secret
metadata:
  name: sql-team-a-sa
  namespace: team-a-sql
type: Opaque
stringData:
  password: "TeamA-Str0ngP@ss!"
---
# Team A SQL Server
apiVersion: mssql.microsoft.com/v1alpha1
kind: SQLServer
metadata:
  name: sql-team-a
  namespace: team-a-sql
spec:
  version: "2022"
  edition: Developer
  instance:
    replicas: 1
    resources:
      limits:
        cpu: "2"
        memory: 8Gi
    storage:
      data:
        size: 50Gi
  credentials:
    saPasswordSecretRef:
      name: sql-team-a-sa
      key: password
---
# Team B SA password secret
apiVersion: v1
kind: Secret
metadata:
  name: sql-team-b-sa
  namespace: team-b-sql
type: Opaque
stringData:
  password: "TeamB-Str0ngP@ss!"
---
# Team B SQL Server
apiVersion: mssql.microsoft.com/v1alpha1
kind: SQLServer
metadata:
  name: sql-team-b
  namespace: team-b-sql
spec:
  version: "2022"
  edition: Developer
  instance:
    replicas: 1
    resources:
      limits:
        cpu: "2"
        memory: 8Gi
    storage:
      data:
        size: 50Gi
  credentials:
    saPasswordSecretRef:
      name: sql-team-b-sa
      key: password
```

**Step 3: Apply the manifest**

```bash
kubectl apply -f multi-tenant-sql.yaml
```

**Expected output:**
```
secret/sql-team-a-sa created
sqlserver.mssql.microsoft.com/sql-team-a created
secret/sql-team-b-sa created
sqlserver.mssql.microsoft.com/sql-team-b created
```

**Step 4: Verify both instances are running**

```bash
kubectl get sqlserver --all-namespaces

# Expected output:
# NAMESPACE     NAME         VERSION   EDITION     STATUS   AGE
# team-a-sql    sql-team-a   2022      Developer   Ready    2m
# team-b-sql    sql-team-b   2022      Developer   Ready    2m
```

### Resource Quotas

Limit resources each team can use.

**Step 1: Create the resource quota file**

Create a file named `sql-resource-quota.yaml`:

```bash
nano sql-resource-quota.yaml
```

Paste the following content and save:

```yaml
apiVersion: v1
kind: ResourceQuota
metadata:
  name: sql-quota
  namespace: team-a-sql
spec:
  hard:
    requests.cpu: "8"
    requests.memory: 32Gi
    limits.cpu: "16"
    limits.memory: 64Gi
    persistentvolumeclaims: "10"
    requests.storage: 500Gi
```

**Step 2: Apply the quota**

```bash
kubectl apply -f sql-resource-quota.yaml
```

**Expected output:**
```
resourcequota/sql-quota created
```

**Step 3: Verify the quota**

```bash
kubectl describe resourcequota sql-quota -n team-a-sql

# Expected output shows used vs hard limits:
# Name:                   sql-quota
# Resource                Used   Hard
# --------                ----   ----
# limits.cpu              2      16
# limits.memory           8Gi    64Gi
# ...
```

## External Access

There are several ways to expose SQL Server outside the Kubernetes cluster.

### LoadBalancer (Cloud Providers)

For cloud environments (Azure, AWS, GCP), use LoadBalancer type.

Add the following to your SQLServer spec:

```yaml
spec:
  service:
    type: LoadBalancer
    port: 1433
    annotations:
      # Azure: Internal LB
      service.beta.kubernetes.io/azure-load-balancer-internal: "true"
      # AWS: Internal ALB
      service.beta.kubernetes.io/aws-load-balancer-internal: "true"
```

**Get the external IP:**

```bash
kubectl get svc sql-prod-01 -n production

# Expected output (may take 1-2 minutes for LB provisioning):
# NAME          TYPE           CLUSTER-IP    EXTERNAL-IP      PORT(S)          AGE
# sql-prod-01   LoadBalancer   10.96.1.100   203.0.113.100    1433:31433/TCP   2m
```

**Connect using the external IP:**

```bash
sqlcmd -S 203.0.113.100,1433 -U sa -P 'YourPassword' -C
```

### NodePort (Bare Metal/On-Premises)

For on-premises or bare-metal clusters without a cloud load balancer.

Add the following to your SQLServer spec:

```yaml
spec:
  service:
    type: NodePort
    port: 1433
    nodePort: 31433  # Access via <node-ip>:31433
```

**Get the NodePort:**

```bash
kubectl get svc sql-prod-01 -n production

# Expected output:
# NAME          TYPE       CLUSTER-IP    EXTERNAL-IP   PORT(S)           AGE
# sql-prod-01   NodePort   10.96.1.100   <none>        1433:31433/TCP    2m
```

**Get node IPs:**

```bash
kubectl get nodes -o wide

# Note the INTERNAL-IP column
```

**Connect using any node IP:**

```bash
sqlcmd -S <node-ip>:31433 -U sa -P 'YourPassword' -C
```

### Port Forward (Development Only)

For local development, use kubectl port-forward. This is the simplest method but only works while the command is running.

**Start the port forward:**

```bash
kubectl port-forward svc/sql-dev-01 -n mssql 1433:1433
```

**Expected output:**
```
Forwarding from 127.0.0.1:1433 -> 1433
Forwarding from [::1]:1433 -> 1433
```

Leave this terminal running. In a new terminal, connect:

```bash
sqlcmd -S localhost,1433 -U sa -P 'YourPassword' -C
```

Press `Ctrl+C` to stop the port forward when done.

## Sample Files

Pre-built sample configurations are available in the [samples/](../../samples/) directory:

| File | Description |
|------|-------------|
| `sqlserver-2022-standalone.yaml` | Basic 2022 standalone instance |
| `sqlserver-2025-standalone.yaml` | SQL Server 2025 instance |
| `sqlserver-availability-group.yaml` | 3-replica AG with services |
| `sqlserver-with-ad.yaml` | Active Directory authentication |

## Next Steps

- [Configuration Reference](configuration-reference.md) - All available options
- [Validation & Security](validation-security.md) - Input requirements
- [Availability Groups](../availability-groups/overview.md) - HA setup
