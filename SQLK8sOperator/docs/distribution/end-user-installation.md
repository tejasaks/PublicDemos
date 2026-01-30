# End-User Installation

[← Back to Distribution](../README.md) | [Documentation Home](../README.md)

Guide for end users to install the SQL Server Kubernetes Operator.

## Table of Contents

- [Where to Get the Operator](#where-to-get-the-operator)
  - [Option A: Use a Published Release](#option-a-use-a-published-release-recommended-for-production)
  - [Option B: Build and Publish Your Own](#option-b-build-and-publish-your-own-recommended-for-production)
  - [Option C: Direct Download (Dev/Test)](#option-c-direct-download-from-github-devtest-only)
- [Prerequisites](#prerequisites)
- [Installation Methods](#installation-methods)
- [Post-Installation](#post-installation)
- [Verification](#verification)
- [Upgrading](#upgrading)
- [Uninstalling](#uninstalling)

## Where to Get the Operator

Before installing, you need to know where the operator artifacts are published. The operator is distributed through:

| Artifact | Location | Description |
|----------|----------|-------------|
| Container Images | `ghcr.io/yourorg/mssql-operator` | Operator container image |
| Installation YAML | GitHub Releases | Single-file Kubernetes manifest |
| Helm Chart | Helm Repository | Packaged Helm chart |
| Direct Download | GitHub Raw URL | Direct YAML download (dev/test only) |

### Option A: Use a Published Release (Recommended for Production)

If your organization has already built and published the operator, obtain these URLs from your platform team:

- **GitHub Releases URL**: `https://github.com/yourorg/mssql-operator/releases`
- **Container Registry**: `ghcr.io/yourorg/mssql-operator`
- **Helm Repository**: `https://yourorg.github.io/mssql-operator`

Replace `yourorg` with your actual GitHub organization name throughout this guide.

### Option B: Build and Publish Your Own (Recommended for Production)

If you need to build and publish the operator yourself, follow these steps:

#### Step 1: Clone the Repository

**Option B1: Clone this reference implementation:**

```bash
# Clone the reference implementation from this repository
git clone https://github.com/tejasaks/PublicDemos.git
cd PublicDemos/SQLK8sOperator
```

**Expected output:**
```
Cloning into 'PublicDemos'...
remote: Enumerating objects: 500, done.
remote: Counting objects: 100% (500/500), done.
remote: Compressing objects: 100% (350/350), done.
Receiving objects: 100% (500/500), 2.5 MiB | 10.00 MiB/s, done.
```

**Option B2: Clone your organization's fork:**

```bash
# Clone your organization's repository (replace yourorg with your org name)
git clone https://github.com/yourorg/mssql-operator.git
cd mssql-operator
```

#### Step 2: Build the Container Images

```bash
# Set your registry and version
export REGISTRY=ghcr.io/yourorg
export VERSION=v1.0.0

# Build and push the operator image
make docker-build docker-push IMG=$REGISTRY/mssql-operator:$VERSION
```

**Expected output:**
```
docker build -t ghcr.io/yourorg/mssql-operator:v1.0.0 .
...
docker push ghcr.io/yourorg/mssql-operator:v1.0.0
The push refers to repository [ghcr.io/yourorg/mssql-operator]
v1.0.0: digest: sha256:abc123... size: 1234
```

#### Step 3: Generate the Installation Manifest

```bash
# Generate the install.yaml with your image
cd config/manager && kustomize edit set image controller=$REGISTRY/mssql-operator:$VERSION
cd ../..
kustomize build config/default > install.yaml
```

**Verify the file was created:**
```bash
ls -lh install.yaml

# Expected output:
# -rw-r--r--  1 user  staff  45K Jan 15 10:00 install.yaml
```

#### Step 4: Create a GitHub Release

```bash
# Tag the release
git tag -a $VERSION -m "Release $VERSION"
git push origin $VERSION

# Create a GitHub release and upload artifacts
gh release create $VERSION install.yaml --title "Release $VERSION" --notes "SQL Server Kubernetes Operator $VERSION"
```

**Expected output:**
```
https://github.com/yourorg/mssql-operator/releases/tag/v1.0.0
```

Now users can install using:
```bash
kubectl apply -f https://github.com/yourorg/mssql-operator/releases/download/v1.0.0/install.yaml
```

#### Step 5: Set Up a Helm Repository (Optional)

For Helm-based installations, see [Helm Chart Packaging](helm-chart.md).

> **For complete build and release instructions**, see:
> - [Building Guide](../development/building.md) - Building from source
> - [Packaging Guide](packaging.md) - Creating release artifacts

### Option C: Direct Download from GitHub (Dev/Test Only)

> ⚠️ **Warning:** This option is **NOT recommended for production environments**. It is suitable for:
> - Quick evaluation and proof-of-concept
> - Development and testing environments
> - Learning and experimentation
>
> For production, use Option A (published releases) or Option B (build your own) to ensure you have control over the container images and manifests.

For quick dev/test scenarios, you can download and apply the operator directly from this repository:

#### Using kubectl (Recommended for Dev/Test)

```bash
# Apply directly from the raw GitHub URL
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

#### Using wget (Linux/macOS)

```bash
# Download the install.yaml first
wget https://raw.githubusercontent.com/tejasaks/PublicDemos/main/SQLK8sOperator/install.yaml

# Review the contents (recommended)
less install.yaml

# Apply to your cluster
kubectl apply -f install.yaml
```

**Expected output (wget):**
```
--2026-01-29 10:00:00--  https://raw.githubusercontent.com/tejasaks/PublicDemos/main/SQLK8sOperator/install.yaml
Resolving raw.githubusercontent.com... 185.199.108.133
Connecting to raw.githubusercontent.com... connected.
HTTP request sent, awaiting response... 200 OK
Length: 195680 (191K) [text/plain]
Saving to: 'install.yaml'

install.yaml              100%[=====================================>] 191.09K  --.-KB/s    in 0.02s

2026-01-29 10:00:00 (9.5 MB/s) - 'install.yaml' saved [195680/195680]
```

#### Using curl (Linux/macOS)

```bash
# Download with curl
curl -LO https://raw.githubusercontent.com/tejasaks/PublicDemos/main/SQLK8sOperator/install.yaml

# Apply to your cluster
kubectl apply -f install.yaml
```

#### Using PowerShell (Windows)

```powershell
# Download the install.yaml
Invoke-WebRequest -Uri "https://raw.githubusercontent.com/tejasaks/PublicDemos/main/SQLK8sOperator/install.yaml" -OutFile "install.yaml"

# Review the contents (recommended)
Get-Content install.yaml | Select-Object -First 50

# Apply to your cluster
kubectl apply -f install.yaml
```

**Expected output (PowerShell download):**
```
StatusCode        : 200
StatusDescription : OK
Content           : # =============================================================================
                    # SQL Server Kubernetes Operator - Combined Installation Manifest
                    ...
```

#### Why Not for Production?

| Concern | Explanation |
|---------|-------------|
| **No version control** | The `main` branch may change without notice |
| **Unknown container images** | References pre-built images you don't control |
| **No security review** | You should review and approve manifests before production |
| **Network dependency** | Requires GitHub access during installation |
| **No rollback path** | No versioned releases to roll back to |

**For production**, always:
1. Fork the repository to your organization
2. Build and push container images to your own registry
3. Create versioned releases with signed artifacts
4. Use a private Helm repository or artifact store

## Prerequisites

### Kubernetes Cluster

| Requirement | Minimum | Recommended |
|-------------|---------|-------------|
| Kubernetes | 1.26+ | 1.28+ |
| Nodes | 1 | 3+ |
| CPU per node | 4 cores | 8+ cores |
| Memory per node | 8GB | 16GB+ |

### Storage

Verify a storage class exists in your cluster:

```bash
kubectl get storageclass
```

**Expected output:**
```
NAME                 PROVISIONER             RECLAIMPOLICY   VOLUMEBINDINGMODE   ALLOWVOLUMEEXPANSION   AGE
standard (default)   kubernetes.io/gce-pd    Delete          Immediate           true                   10d
```

You should see at least one storage class. The one marked `(default)` will be used if you don't specify a storage class.

### kubectl Access

Verify you can access your cluster:

```bash
kubectl cluster-info
```

**Expected output:**
```
Kubernetes control plane is running at https://your-cluster-endpoint:6443
CoreDNS is running at https://your-cluster-endpoint:6443/api/v1/namespaces/kube-system/services/kube-dns:dns/proxy
```

```bash
kubectl get nodes
```

**Expected output:**
```
NAME                 STATUS   ROLES           AGE   VERSION
node-1               Ready    control-plane   10d   v1.28.0
node-2               Ready    <none>          10d   v1.28.0
node-3               Ready    <none>          10d   v1.28.0
```

All nodes should show `Ready` status.

## Installation Methods

Choose the installation method that best fits your environment.

### Method 0: Direct URL (Fastest - Recommended for Quick Start)

The absolute fastest way to install. A single command pulls the installation manifest directly from this repository and applies it.

> **This is the recommended method for quick evaluation and getting started.**

**Step 1: Install directly from the repository**

```bash
# Option A: Apply directly from URL (Linux/macOS/Windows with curl)
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

**Or download first, then apply (useful for air-gapped environments):**

```bash
# Option B: Download first, then apply
# Linux/macOS:
wget https://raw.githubusercontent.com/tejasaks/PublicDemos/main/SQLK8sOperator/install.yaml
kubectl apply -f install.yaml

# Windows PowerShell:
Invoke-WebRequest -Uri "https://raw.githubusercontent.com/tejasaks/PublicDemos/main/SQLK8sOperator/install.yaml" -OutFile "install.yaml"
kubectl apply -f install.yaml
```

**Expected output (wget):**
```
--2024-01-15 10:00:00--  https://raw.githubusercontent.com/tejasaks/PublicDemos/main/SQLK8sOperator/install.yaml
Resolving raw.githubusercontent.com... 185.199.108.133
Connecting to raw.githubusercontent.com... connected.
HTTP request sent, awaiting response... 200 OK
Length: 15234 (15K) [text/plain]
Saving to: 'install.yaml'

install.yaml              100%[=====================================>]  14.88K  --.-KB/s    in 0.001s  

2024-01-15 10:00:00 (14.9 MB/s) - 'install.yaml' saved [15234/15234]
```

**Step 2: Verify the installation**

```bash
# Check operator pod is running
kubectl get pods -n mssql-system
```

**Expected output:**
```
NAME                              READY   STATUS    RESTARTS   AGE
mssql-operator-7d8f9c6b4d-x2k9m   1/1     Running   0          30s
```

```bash
# Check CRDs are installed
kubectl get crds | grep mssql
```

**Expected output:**
```
sqlserverags.mssql.microsoft.com    2024-01-15T10:00:00Z
sqlservers.mssql.microsoft.com      2024-01-15T10:00:00Z
```

**Step 3: Deploy your first SQL Server instance**

```bash
# Create a secret for the SA password
kubectl create secret generic mssql-secret \
  --from-literal=SA_PASSWORD='YourStrong!Passw0rd' \
  -n mssql
```

**Expected output:**
```
secret/mssql-secret created
```

```bash
# Apply a sample SQL Server deployment
kubectl apply -f https://raw.githubusercontent.com/tejasaks/PublicDemos/main/SQLK8sOperator/samples/sqlserver-2025-standalone.yaml
```

**Expected output:**
```
sqlserver.mssql.microsoft.com/mssql-standalone created
```

```bash
# Watch the SQL Server pod come up
kubectl get pods -n mssql -w
```

**Expected output (after ~1-2 minutes):**
```
NAME                 READY   STATUS    RESTARTS   AGE
mssql-standalone-0   1/1     Running   0          90s
```

**To uninstall:**

```bash
# Remove all SQL Server instances first
kubectl delete sqlservers --all -A
kubectl delete sqlserverags --all -A

# Then remove the operator
kubectl delete -f https://raw.githubusercontent.com/tejasaks/PublicDemos/main/SQLK8sOperator/install.yaml
```

**Expected output:**
```
namespace "mssql-system" deleted
namespace "mssql" deleted
serviceaccount "mssql-operator" deleted
clusterrole.rbac.authorization.k8s.io "mssql-operator-role" deleted
clusterrolebinding.rbac.authorization.k8s.io "mssql-operator-rolebinding" deleted
customresourcedefinition.apiextensions.k8s.io "sqlservers.mssql.microsoft.com" deleted
customresourcedefinition.apiextensions.k8s.io "sqlserverags.mssql.microsoft.com" deleted
deployment.apps "mssql-operator" deleted
```

---

### Method 1: kubectl apply from Releases

Use this method for production environments where you want a specific versioned release.

**Step 1: Install the latest release**

```bash
kubectl apply -f https://github.com/yourorg/mssql-operator/releases/latest/download/install.yaml
```

**Expected output:**
```
namespace/mssql-operator-system created
customresourcedefinition.apiextensions.k8s.io/sqlservers.mssql.microsoft.com created
customresourcedefinition.apiextensions.k8s.io/sqlserverags.mssql.microsoft.com created
serviceaccount/mssql-operator-controller-manager created
role.rbac.authorization.k8s.io/mssql-operator-leader-election-role created
clusterrole.rbac.authorization.k8s.io/mssql-operator-manager-role created
...
deployment.apps/mssql-operator-controller-manager created
```

**Or install a specific version:**

```bash
kubectl apply -f https://github.com/yourorg/mssql-operator/releases/download/v1.0.0/install.yaml
```

**Step 2: Verify the installation**

See [Post-Installation](#post-installation) below.

### Method 2: Helm (Recommended for Production)

Helm provides better version management and configuration options.

**Step 1: Add the Helm repository**

```bash
helm repo add mssql-operator https://yourorg.github.io/mssql-operator
helm repo update
```

**Expected output:**
```
"mssql-operator" has been added to your repositories
Hang tight while we grab the latest from your chart repositories...
...Successfully got an update from the "mssql-operator" chart repository
Update Complete. ⎈Happy Helming!⎈
```

**Step 2: Install the operator**

```bash
helm install mssql-operator mssql-operator/mssql-operator \
  --namespace mssql-operator-system \
  --create-namespace
```

**Expected output:**
```
NAME: mssql-operator
LAST DEPLOYED: Mon Jan 15 10:00:00 2024
NAMESPACE: mssql-operator-system
STATUS: deployed
REVISION: 1
TEST SUITE: None
NOTES:
The MSSQL Operator has been installed.
...
```

**Optional: Install with custom values**

```bash
helm install mssql-operator mssql-operator/mssql-operator \
  --namespace mssql-operator-system \
  --create-namespace \
  --set resources.limits.memory=512Mi
```

### Method 3: Kustomize

Kustomize allows you to customize the installation without modifying the original manifests.

**Step 1: Create a directory for your customization**

```bash
mkdir mssql-operator-install
cd mssql-operator-install
```

**Step 2: Create the kustomization.yaml file**

```bash
# On Linux/macOS
nano kustomization.yaml

# On Windows (PowerShell)
notepad kustomization.yaml
```

Paste the following content and save:

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
  - https://github.com/yourorg/mssql-operator/releases/download/v1.0.0/install.yaml

namespace: mssql-operator-system

# Optional: Customize resources
patchesStrategicMerge:
  - manager_resources_patch.yaml
```

**Step 3: Create a patch file for custom resources (optional)**

Create a file named `manager_resources_patch.yaml`:

```bash
nano manager_resources_patch.yaml
```

Paste the following content:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: mssql-operator-controller-manager
  namespace: mssql-operator-system
spec:
  template:
    spec:
      containers:
        - name: manager
          resources:
            limits:
              cpu: 1000m
              memory: 512Mi
```

**Step 4: Apply with Kustomize**

```bash
kubectl apply -k .
```

**Expected output:**
```
namespace/mssql-operator-system created
customresourcedefinition.apiextensions.k8s.io/sqlservers.mssql.microsoft.com created
...
deployment.apps/mssql-operator-controller-manager created
```

### Method 4: OLM (Operator Lifecycle Manager)

For clusters using OLM for operator management.

**Step 1: Install OLM if not present**

```bash
curl -sL https://github.com/operator-framework/operator-lifecycle-manager/releases/download/v0.26.0/install.sh | bash -s v0.26.0
```

**Expected output:**
```
customresourcedefinition.apiextensions.k8s.io/catalogsources.operators.coreos.com created
...
Waiting for deployment "olm-operator" rollout to finish...
deployment "olm-operator" successfully rolled out
```

**Step 2: Create the subscription**

Create a file named `mssql-operator-subscription.yaml`:

```bash
nano mssql-operator-subscription.yaml
```

Paste the following content:

```yaml
apiVersion: operators.coreos.com/v1alpha1
kind: Subscription
metadata:
  name: mssql-operator
  namespace: operators
spec:
  channel: stable
  name: mssql-operator
  source: operatorhubio-catalog
  sourceNamespace: olm
```

**Step 3: Apply the subscription**

```bash
kubectl apply -f mssql-operator-subscription.yaml
```

**Expected output:**
```
subscription.operators.coreos.com/mssql-operator created
```

## Post-Installation

After installing the operator, verify everything is working correctly.

### Verify Operator is Running

```bash
kubectl get pods -n mssql-operator-system
```

**Expected output:**
```
NAME                                                  READY   STATUS    RESTARTS   AGE
mssql-operator-controller-manager-5d4b8c9f6d-abc12    1/1     Running   0          1m
```

The pod should show `Running` status and `1/1` ready.

### Check CRDs Installed

```bash
kubectl get crds | grep mssql
```

**Expected output:**
```
sqlserverags.mssql.microsoft.com        2024-01-15T10:00:00Z
sqlservers.mssql.microsoft.com          2024-01-15T10:00:00Z
```

You should see both CRDs (Custom Resource Definitions) listed.

### Create SQL Server Namespace

Create a namespace for your SQL Server instances:

```bash
kubectl create namespace mssql
```

**Expected output:**
```
namespace/mssql created
```

## Verification

Test the installation by deploying a test SQL Server instance.

### Deploy Test Instance

**Step 1: Create the password secret**

```bash
kubectl create secret generic sql-test-sa \
  --from-literal=password='YourStr0ng!Passw0rd' \
  -n mssql
```

**Expected output:**
```
secret/sql-test-sa created
```

**Step 2: Create the test SQLServer manifest file**

Create a file named `sql-test.yaml`:

```bash
# On Linux/macOS
nano sql-test.yaml

# On Windows (PowerShell)
notepad sql-test.yaml
```

Paste the following content and save:

```yaml
apiVersion: mssql.microsoft.com/v1alpha1
kind: SQLServer
metadata:
  name: sql-test
  namespace: mssql
spec:
  version: "2025"
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
      name: sql-test-sa
      key: password
```

**Step 3: Apply the manifest**

```bash
kubectl apply -f sql-test.yaml
```

**Expected output:**
```
sqlserver.mssql.microsoft.com/sql-test created
```

### Check Status

**Step 4: Watch the pod come up**

```bash
kubectl get pods -n mssql -w
```

**Expected output (wait for Running status):**
```
NAME          READY   STATUS    RESTARTS   AGE
sql-test-0    0/1     Pending   0          5s
sql-test-0    0/1     ContainerCreating   0   10s
sql-test-0    1/1     Running   0          45s
```

Press `Ctrl+C` to stop watching once the pod is Running.

**Step 5: Check the SQLServer resource status**

```bash
kubectl get sqlserver -n mssql
```

**Expected output:**
```
NAME       VERSION   EDITION     STATUS   AGE
sql-test   2025      Developer   Ready    2m
```

**Step 6: Get detailed information (optional)**

```bash
kubectl describe sqlserver sql-test -n mssql
```

### Connect to SQL Server

**Step 7: Port forward to the SQL Server service**

```bash
kubectl port-forward svc/sql-test -n mssql 1433:1433
```

**Expected output:**
```
Forwarding from 127.0.0.1:1433 -> 1433
Forwarding from [::1]:1433 -> 1433
```

Leave this terminal running.

**Step 8: Connect with sqlcmd (in another terminal)**

```bash
sqlcmd -S localhost,1433 -U sa -P 'YourStr0ng!Passw0rd' -C -Q "SELECT @@VERSION"
```

**Expected output:**
```
Microsoft SQL Server 2025 (RTM-CU1) - 17.0.1001.1 (X64)
        ...
```

If you see the SQL Server version, the installation is successful!

### Clean Up Test

After verification, remove the test instance:

```bash
kubectl delete sqlserver sql-test -n mssql
kubectl delete secret sql-test-sa -n mssql
```

**Expected output:**
```
sqlserver.mssql.microsoft.com "sql-test" deleted
secret "sql-test-sa" deleted
```

## Upgrading

### Check Current Version

**For Helm installations:**

```bash
helm list -n mssql-operator-system
```

**Expected output:**
```
NAME            NAMESPACE                REVISION  UPDATED                                 STATUS    CHART                  APP VERSION
mssql-operator  mssql-operator-system    1         2024-01-15 10:00:00.000000 -0800 PST    deployed  mssql-operator-1.0.0   1.0.0
```

**For kubectl installations:**

```bash
kubectl get deployment -n mssql-operator-system -o jsonpath='{.items[0].spec.template.spec.containers[0].image}'
```

**Expected output:**
```
yourorg/mssql-operator:v1.0.0
```

### Upgrade with Helm

**Step 1: Update the Helm repository**

```bash
helm repo update
```

**Expected output:**
```
Hang tight while we grab the latest from your chart repositories...
...Successfully got an update from the "mssql-operator" chart repository
Update Complete. ⎈Happy Helming!⎈
```

**Step 2: View available versions**

```bash
helm search repo mssql-operator --versions
```

**Expected output:**
```
NAME                            CHART VERSION   APP VERSION     DESCRIPTION
mssql-operator/mssql-operator   1.1.0           1.1.0           SQL Server Kubernetes Operator
mssql-operator/mssql-operator   1.0.0           1.0.0           SQL Server Kubernetes Operator
```

**Step 3: Upgrade to the latest version**

```bash
helm upgrade mssql-operator mssql-operator/mssql-operator \
  --namespace mssql-operator-system \
  --reuse-values
```

**Expected output:**
```
Release "mssql-operator" has been upgraded. Happy Helming!
NAME: mssql-operator
LAST DEPLOYED: Mon Jan 22 10:00:00 2024
NAMESPACE: mssql-operator-system
STATUS: deployed
REVISION: 2
```

### Upgrade with kubectl

**Step 1: Apply the new version**

```bash
kubectl apply -f https://github.com/yourorg/mssql-operator/releases/download/v1.1.0/install.yaml
```

**Expected output:**
```
customresourcedefinition.apiextensions.k8s.io/sqlservers.mssql.microsoft.com configured
customresourcedefinition.apiextensions.k8s.io/sqlserverags.mssql.microsoft.com configured
...
deployment.apps/mssql-operator-controller-manager configured
```

> **Note:** If CRDs have changed, you may need to apply CRDs first:
> ```bash
> kubectl apply -f https://github.com/yourorg/mssql-operator/releases/download/v1.1.0/crds.yaml
> ```

### Rolling Back

If the upgrade causes issues, rollback to the previous version.

**For Helm:**

```bash
helm rollback mssql-operator 1 -n mssql-operator-system
```

**Expected output:**
```
Rollback was a success! Happy Helming!
```

**For kubectl:**

```bash
kubectl apply -f https://github.com/yourorg/mssql-operator/releases/download/v1.0.0/install.yaml
```

## Uninstalling

Follow these steps carefully to completely remove the operator.

### Remove SQL Server Resources First

> ⚠️ **Important**: Delete all SQL Server instances before uninstalling the operator to ensure clean resource cleanup.

**Step 1: Delete all SQL Server instances**

```bash
kubectl delete sqlserver --all -A
kubectl delete sqlserverag --all -A
```

**Expected output:**
```
sqlserver.mssql.microsoft.com "sql-prod" deleted
sqlserverag.mssql.microsoft.com "prod-ag" deleted
```

**Step 2: Wait for all pods to terminate**

```bash
kubectl get pods -n mssql
```

**Expected output (should be empty or show terminating):**
```
No resources found in mssql namespace.
```

### Uninstall Operator

**For Helm installations:**

```bash
helm uninstall mssql-operator -n mssql-operator-system
```

**Expected output:**
```
release "mssql-operator" uninstalled
```

**For kubectl installations:**

```bash
kubectl delete -f https://github.com/yourorg/mssql-operator/releases/latest/download/install.yaml
```

**Expected output:**
```
namespace "mssql-operator-system" deleted
customresourcedefinition.apiextensions.k8s.io "sqlservers.mssql.microsoft.com" deleted
...
deployment.apps "mssql-operator-controller-manager" deleted
```

### Remove CRDs (Optional)

> ⚠️ **Warning**: This permanently removes all SQLServer resource definitions. Only do this if you're completely done with the operator.

```bash
kubectl delete crd sqlservers.mssql.microsoft.com
kubectl delete crd sqlserverags.mssql.microsoft.com
```

**Expected output:**
```
customresourcedefinition.apiextensions.k8s.io "sqlservers.mssql.microsoft.com" deleted
customresourcedefinition.apiextensions.k8s.io "sqlserverags.mssql.microsoft.com" deleted
```

### Remove Namespaces

```bash
kubectl delete namespace mssql-operator-system
kubectl delete namespace mssql
```

**Expected output:**
```
namespace "mssql-operator-system" deleted
namespace "mssql" deleted
```

## Troubleshooting Installation

### Operator Pod Not Starting

**Step 1: Check pod status**

```bash
kubectl describe pod -n mssql-operator-system -l control-plane=controller-manager
```

Look for the "Events" section at the bottom for error messages.

**Step 2: Check container logs**

```bash
kubectl logs -n mssql-operator-system -l control-plane=controller-manager
```

**Common issues and solutions:**

| Issue | Cause | Solution |
|-------|-------|----------|
| `ImagePullBackOff` | Cannot pull container image | Check registry access, image name |
| `CrashLoopBackOff` | Pod crashing on startup | Check logs for error message |
| `Pending` | No node can schedule pod | Check resource availability |
| `Error: forbidden` | RBAC permission denied | Verify ClusterRole/Binding |

### CRDs Not Found

If you get errors like "no matches for kind 'SQLServer'":

**Step 1: Reinstall CRDs**

```bash
kubectl apply -f https://github.com/yourorg/mssql-operator/releases/latest/download/crds.yaml
```

**Expected output:**
```
customresourcedefinition.apiextensions.k8s.io/sqlservers.mssql.microsoft.com created
customresourcedefinition.apiextensions.k8s.io/sqlserverags.mssql.microsoft.com created
```

**Step 2: Verify CRDs are installed**

```bash
kubectl get crd | grep mssql
```

**Expected output:**
```
sqlserverags.mssql.microsoft.com        2024-01-15T10:00:00Z
sqlservers.mssql.microsoft.com          2024-01-15T10:00:00Z
```

### Permission Denied

If the operator reports permission errors:

**Step 1: Check if the service account has permissions**

```bash
kubectl auth can-i create sqlservers --as=system:serviceaccount:mssql-operator-system:mssql-operator-controller-manager
```

**Expected output:**
```
yes
```

If the output is `no`, the RBAC configuration is incorrect.

**Step 2: Check ClusterRoleBinding exists**

```bash
kubectl get clusterrolebinding | grep mssql
```

**Expected output:**
```
mssql-operator-manager-rolebinding    ClusterRole/mssql-operator-manager-role    5m
```

**Step 3: Reinstall to fix RBAC**

```bash
kubectl apply -f https://github.com/yourorg/mssql-operator/releases/latest/download/install.yaml
```

## Next Steps

After installation:

1. [Getting Started](../getting-started.md) - Deploy your first SQL Server
2. [Deployment Scenarios](../user-guide/deployment-scenarios.md) - Production patterns
3. [Monitoring Setup](../monitoring/overview.md) - Enable observability
