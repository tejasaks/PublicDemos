# Getting Started

[← Back to Documentation](README.md)

This guide walks you through deploying your first SQL Server instance using the MSSQL Kubernetes Operator.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Step 1: Install the Operator](#step-1-install-the-operator)
- [Step 2: Create the SA Password Secret](#step-2-create-the-sa-password-secret)
- [Step 3: Create a SQL Server Manifest](#step-3-create-a-sql-server-manifest)
- [Step 4: Deploy SQL Server](#step-4-deploy-sql-server)
- [Step 5: Verify Deployment](#step-5-verify-deployment)
- [Step 6: Connect to SQL Server](#step-6-connect-to-sql-server)
- [Next Steps](#next-steps)

## Prerequisites

Before you begin, ensure you have:

| Requirement | Minimum Version | Check Command |
|-------------|-----------------|---------------|
| Kubernetes cluster | 1.28+ | `kubectl version` |
| kubectl | 1.28+ | `kubectl version --client` |
| Cluster admin access | - | `kubectl auth can-i create crd` |
| Storage provisioner | - | `kubectl get storageclass` |

### Cluster Resources

SQL Server requires significant resources:

| Resource | Minimum | Recommended |
|----------|---------|-------------|
| CPU | 2 cores | 4 cores |
| Memory | 2Gi | 4Gi+ |
| Storage | 10Gi | 50Gi+ |

## Step 1: Install the Operator

Install the Custom Resource Definitions (CRDs) and the operator components.

### Quick Test Option (Dev/Test Only)

For quick evaluation or development testing, you can install directly from this repository:

```bash
# Install the operator with a single command
kubectl apply -f https://raw.githubusercontent.com/tejasaks/PublicDemos/main/SQLK8sOperator/install.yaml
```

**Expected output:**
```
namespace/mssql-system created
namespace/mssql created
serviceaccount/mssql-operator created
clusterrole.rbac.authorization.k8s.io/mssql-operator-role created
clusterrolebinding.rbac.authorization.k8s.io/mssql-operator-rolebinding created
customresourcedefinition.apiextensions.k8s.io/sqlservers.mssql.microsoft.com created
customresourcedefinition.apiextensions.k8s.io/sqlserverags.mssql.microsoft.com created
deployment.apps/mssql-operator created
```

> ⚠️ **Note:** This direct installation method is suitable for development and testing only. For production environments, see the [End-User Installation Guide](distribution/end-user-installation.md) which covers building your own images, versioned releases, and Helm-based installations.

### Standard Installation (From Repository)

```bash
# Clone the repository (if you haven't already)
git clone https://github.com/microsoft/mssql-operator.git
cd mssql-operator

# Install CRDs (defines SQLServer and SQLServerAG resource types)
kubectl apply -f deploy/crds/

# Create the operator namespace
kubectl apply -f deploy/namespace.yaml

# Install RBAC (permissions for the operator)
kubectl apply -f deploy/serviceaccount.yaml
kubectl apply -f deploy/rbac.yaml

# Deploy the operator
kubectl apply -f deploy/deployment.yaml
```

### Verify the Operator is Running

```bash
kubectl get pods -n mssql-system

# Expected output:
# NAME                              READY   STATUS    RESTARTS   AGE
# mssql-operator-xxxxxxxxx-xxxxx   1/1     Running   0          30s
```

### Webhook TLS Certificates

The operator automatically generates self-signed TLS certificates for its admission webhooks at startup — no configuration required. For production environments that require enterprise-managed or cert-manager-issued certificates, see [Webhook Certificate Management](user-guide/validation-security.md#webhook-certificate-management).

### Configure Container Images (Optional)

By default, the operator pulls SQL Server images from Microsoft Container Registry (MCR), which requires no authentication. For private registry or air-gapped deployments, see [Private Registry Deployment](user-guide/deployment-scenarios.md#private-registry-deployment).

| Sample Configuration | Use Case |
|---------------------|----------|
| [operator-configuration-mcr-defaults.yaml](../samples/operator-configuration-mcr-defaults.yaml) | Pin specific CU versions from MCR |
| [operator-configuration-private-registry.yaml](../samples/operator-configuration-private-registry.yaml) | Use your own private container registry |

## Step 2: Create the SA Password Secret

Before creating a SQL Server instance, you must create a Kubernetes Secret containing the SA (system administrator) password.

### Password Requirements

The password must meet SQL Server complexity requirements:
- Minimum 8 characters
- Contains at least 3 of these 4 categories:
  - Uppercase letters (A-Z)
  - Lowercase letters (a-z)
  - Digits (0-9)
  - Special characters (!@#$%^&*...)

```bash
# Create namespace for your SQL Server resources
kubectl create namespace mssql

# Create the SA password secret
# Replace 'YourStrong@Passw0rd!' with your actual password
kubectl create secret generic mssql-sa-password \
  --from-literal=password='YourStrong@Passw0rd!' \
  -n mssql
```

> **Security Note:** In production, consider using a secrets management solution like Azure Key Vault, HashiCorp Vault, or External Secrets Operator.

## Step 3: Create a SQL Server Manifest

Create a YAML manifest file that describes your desired SQL Server configuration.

### Sample Manifests

This project includes ready-to-use sample manifests in the [samples/](../samples/) directory that you can review and use as starting points:

| Sample File | Description |
|-------------|-------------|
| [sqlserver-2025-standalone.yaml](../samples/sqlserver-2025-standalone.yaml) | Basic SQL Server 2025 standalone instance (recommended starting point) |
| [sqlserver-2022-standalone.yaml](../samples/sqlserver-2022-standalone.yaml) | SQL Server 2022 standalone instance |
| [sql-ag-ha/](../samples/sql-ag-ha/) | High Availability AG (3 sync replicas, auto failover, listener) |
| [sql-ag-dr/](../samples/sql-ag-dr/) | Disaster Recovery AG (2 sync + 1 async, manual failover) |
| [sql-ag-multiag/](../samples/sql-ag-multiag/) | Multiple AGs on same replicas |
| [sql-ag-monitoring/](../samples/sql-ag-monitoring/) | HA AG + Prometheus + Grafana monitoring stack |
| [sqlserver-with-ad.yaml](../samples/sqlserver-with-ad.yaml) | Active Directory authentication configuration |

> **Note on AG Samples:** The Availability Group samples include Secret definitions for SA and AG Helper credentials for dev/test convenience. For production, see the [AG Deployment Guide](availability-groups/deployment-guide.md#step-25-create-ag-helper-credentials) for guidance on pre-creating secrets securely.

You can apply a sample directly:

```bash
# Create the password secret first (see Step 2)
kubectl create secret generic mssql-sa-password --from-literal=password='YourStrong@Passw0rd!' -n mssql

# Apply a sample manifest
kubectl apply -f samples/sqlserver-2025-standalone.yaml
```

Or create your own custom manifest using the options below.

### Option A: Using `cat` (Linux/macOS/WSL)

```bash
cat > my-sqlserver.yaml << 'EOF'
apiVersion: mssql.microsoft.com/v1alpha1
kind: SQLServer
metadata:
  name: my-sqlserver
  namespace: mssql
spec:
  version: "2025"
  edition: Developer
  instance:
    count: 1
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
EOF
```

### Option B: Using a Text Editor

```bash
nano my-sqlserver.yaml
# Paste the YAML content above, save with Ctrl+O, exit with Ctrl+X
```

### Option C: Using PowerShell (Windows)

```powershell
@"
apiVersion: mssql.microsoft.com/v1alpha1
kind: SQLServer
metadata:
  name: my-sqlserver
  namespace: mssql
spec:
  version: "2025"
  edition: Developer
  instance:
    count: 1
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
"@ | Out-File -FilePath my-sqlserver.yaml -Encoding UTF8
```

> **Important:** The resource name (`my-sqlserver`) must be **13 characters or fewer** due to SQL Server's NetBIOS naming limit. See [Resource Naming Constraints](user-guide/validation-security.md#resource-name-validation).

## Step 4: Deploy SQL Server

Apply the manifest to deploy SQL Server:

```bash
kubectl apply -f my-sqlserver.yaml
```

This command submits your configuration to Kubernetes. The operator will:
1. Create a ConfigMap for SQL Server configuration
2. Create PersistentVolumeClaims for data storage
3. Create a StatefulSet to manage SQL Server pods
4. Create Services for network access
5. Deploy SQL Exporter sidecar for monitoring (if enabled)

## Step 5: Verify Deployment

### Check the SQLServer Resource

```bash
kubectl get sqlserver -n mssql

# Expected output:
# NAME           VERSION   EDITION     COUNT   READY   STATUS    AGE
# my-sqlserver   2025      Developer   1       1       Running   2m
```

### Check the Pods

```bash
kubectl get pods -n mssql

# Expected output:
# NAME             READY   STATUS    RESTARTS   AGE
# my-sqlserver-0   2/2     Running   0          2m
```

> **Note:** `2/2` indicates both the SQL Server container and the SQL Exporter sidecar are running.

### Check Detailed Status

```bash
kubectl describe sqlserver my-sqlserver -n mssql
```

### Watch Pod Logs

```bash
# SQL Server logs
kubectl logs my-sqlserver-0 -n mssql -c mssql-server

# SQL Exporter logs (if monitoring enabled)
kubectl logs my-sqlserver-0 -n mssql -c sql-exporter
```

### Common Issues

| Symptom | Cause | Solution |
|---------|-------|----------|
| Pod stuck in `Pending` | No storage provisioner | Check `kubectl get pvc -n mssql` |
| Pod `CrashLoopBackOff` | Invalid password or low memory | Check logs, ensure ≥2Gi memory |
| `0/1` Ready | SQL Server starting | Wait 1-2 minutes |

## Step 6: Connect to SQL Server

### Port Forward (Development)

The easiest way to connect for testing:

```bash
# Forward local port 1433 to the SQL Server pod
kubectl port-forward svc/my-sqlserver -n mssql 1433:1433
```

Then connect using your SQL client:
- **Server:** `localhost,1433`
- **Username:** `sa`
- **Password:** Your password from the secret

### Using sqlcmd

```bash
# Connect directly via kubectl exec
kubectl exec -it my-sqlserver-0 -n mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P 'YourStrong@Passw0rd!' -C

# Run a simple query
1> SELECT @@VERSION
2> GO
```

### Connection String

For applications:

```
Server=my-sqlserver.mssql.svc.cluster.local,1433;
Database=master;
User Id=sa;
Password=YourStrong@Passw0rd!;
Encrypt=true;
TrustServerCertificate=true;
```

## Next Steps

Now that you have SQL Server running, explore:

| Goal | Guide |
|------|-------|
| **High Availability** | [Availability Groups Deployment](availability-groups/deployment-guide.md) |
| **Monitoring** | [Prometheus & Grafana Setup](monitoring/overview.md) |
| **Production Config** | [Deployment Scenarios](user-guide/deployment-scenarios.md) |
| **All Options** | [Configuration Reference](user-guide/configuration-reference.md) |
| **Security** | [Validation & Security](user-guide/validation-security.md) |
| **Installation Options** | [End-User Installation](distribution/end-user-installation.md) |

### Additional Resources

- [Sample Manifests](../samples/) - More ready-to-use configurations
- [Architecture Overview](architecture/overview.md) - How the operator works
- [Troubleshooting](user-guide/troubleshooting.md) - Common issues and solutions
