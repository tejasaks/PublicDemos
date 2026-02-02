# SQL Server Upgrades

[← Back to Operations](../README.md) | [Documentation Home](../README.md)

Guide to upgrading SQL Server instances managed by the operator.

## Table of Contents

- [Upgrade Types](#upgrade-types)
- [Patching the Operators](#patching-the-operators)
  - [Operator Patch Workflow](#operator-patch-workflow)
  - [Update CRDs Only](#update-crds-only)
  - [Update Operator Image Only](#update-operator-image-only)
  - [Full Operator Update](#full-operator-update)
  - [AG Helper Patching](#ag-helper-patching)
  - [Minikube Quick Patching](#minikube-quick-patching)
  - [Patching Without Downtime](#patching-without-downtime)
- [Version Upgrade](#version-upgrade)
- [Cumulative Update (CU)](#cumulative-update-cu)
- [Rolling Upgrade (HA)](#rolling-upgrade-ha)
- [Pre-Upgrade Checklist](#pre-upgrade-checklist)
- [Post-Upgrade Validation](#post-upgrade-validation)
- [Rollback](#rollback)
- [Best Practices](#best-practices)

## Upgrade Types

| Type | Description | Downtime |
|------|-------------|----------|
| Major Version | 2019 → 2022 | Requires planning |
| Cumulative Update | CU10 → CU15 | Minimal with HA |
| Container Image | Patch updates | Rolling update |

## Patching the Operators

This section covers how to incrementally update the SQL Server Operator and AG Helper without reconfiguring your entire deployment from scratch. This is the recommended approach for applying bug fixes, new features, or configuration changes.

### Operator Patch Workflow

The operator patching process follows these general steps:

```
┌────────────────────────────────────────────────────────────────────┐
│                     Operator Patching Workflow                      │
├────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  1. Pull latest code changes                                        │
│     └─ git pull origin main                                        │
│                                                                     │
│  2. Regenerate CRDs (if types changed)                             │
│     └─ make manifests                                              │
│     └─ kubectl apply -f deploy/crds/                               │
│                                                                     │
│  3. Rebuild operator image                                          │
│     └─ make docker-build IMG=<registry>/mssql-operator:<tag>       │
│     └─ make docker-push IMG=<registry>/mssql-operator:<tag>        │
│                                                                     │
│  4. Restart operator deployment                                     │
│     └─ kubectl rollout restart deployment/mssql-operator -n ...    │
│                                                                     │
│  5. Verify operator is running                                      │
│     └─ kubectl get pods -n mssql-system                            │
│     └─ kubectl logs deployment/mssql-operator -n mssql-system      │
│                                                                     │
└────────────────────────────────────────────────────────────────────┘
```

### Update CRDs Only

If you've only changed the API types (added new fields, modified validation), you need to update the CRDs without redeploying the operator.

**Step 1: Regenerate CRD manifests**

```bash
make manifests
```

**Expected output:**
```
go mod tidy
controller-gen crd paths="./pkg/apis/mssql.microsoft.com/v1alpha1" output:crd:dir=./deploy/crds
"CRDs generated in deploy/crds/"
```

**Step 2: Apply the updated CRDs to the cluster**

```bash
kubectl apply -f deploy/crds/ --force
```

**Expected output:**
```
customresourcedefinition.apiextensions.k8s.io/sqlservers.mssql.microsoft.com configured
customresourcedefinition.apiextensions.k8s.io/sqlserverags.mssql.microsoft.com configured
```

**Step 3: Verify CRDs are updated**

```bash
kubectl get crd sqlservers.mssql.microsoft.com -o jsonpath='{.metadata.resourceVersion}'
```

> **Note:** Updating CRDs does not affect existing resources. Kubernetes validates new resources against the updated schema. Existing resources remain unchanged until modified.

### Update Operator Image Only

If you've made code changes to the operator but haven't modified the API types, you can simply rebuild and redeploy the operator image.

**Step 1: Build the new operator image**

```bash
# Build with your registry and tag
make docker-build IMG=ghcr.io/yourorg/mssql-operator:v1.0.1
```

**Expected output:**
```
docker build -t ghcr.io/yourorg/mssql-operator:v1.0.1 .
[+] Building 45.2s (15/15) FINISHED
...
=> exporting to image
=> => naming to ghcr.io/yourorg/mssql-operator:v1.0.1
```

**Step 2: Push the image to your registry**

```bash
make docker-push IMG=ghcr.io/yourorg/mssql-operator:v1.0.1
```

**Expected output:**
```
docker push ghcr.io/yourorg/mssql-operator:v1.0.1
The push refers to repository [ghcr.io/yourorg/mssql-operator]
abc123: Pushed
v1.0.1: digest: sha256:... size: 1234
```

**Step 3: Update the operator deployment to use the new image**

Option A - Rollout restart (if using `imagePullPolicy: Always`):

```bash
kubectl rollout restart deployment/mssql-operator -n mssql-system
```

Option B - Patch the deployment with the new image:

```bash
kubectl set image deployment/mssql-operator \
  manager=ghcr.io/yourorg/mssql-operator:v1.0.1 \
  -n mssql-system
```

**Step 4: Verify the rollout**

```bash
kubectl rollout status deployment/mssql-operator -n mssql-system
```

**Expected output:**
```
deployment "mssql-operator" successfully rolled out
```

**Step 5: Check the operator is running with the new image**

```bash
kubectl get pods -n mssql-system -o wide
kubectl logs deployment/mssql-operator -n mssql-system --tail=20
```

### Full Operator Update

When you've made changes to both API types and controller code, perform a complete update:

**Step 1: Pull latest changes**

```bash
git pull origin main
```

**Step 2: Regenerate all manifests**

```bash
make generate manifests
```

**Step 3: Build and push the new operator image**

```bash
make docker-build docker-push IMG=ghcr.io/yourorg/mssql-operator:v1.0.1
```

**Step 4: Apply CRD updates**

```bash
kubectl apply -f deploy/crds/ --force
```

**Step 5: Restart the operator deployment**

```bash
kubectl rollout restart deployment/mssql-operator -n mssql-system
```

**Step 6: Verify everything is working**

```bash
# Check operator pod
kubectl get pods -n mssql-system

# Check operator logs for errors
kubectl logs deployment/mssql-operator -n mssql-system --tail=50

# Verify your SQLServer resources are still healthy
kubectl get sqlserver -n mssql
kubectl get sqlserverag -n mssql
```

### AG Helper Patching

The AG Helper is deployed as a separate pod per Availability Group. To update the AG Helper:

**Step 1: Build and push the new AG Helper image**

```bash
make docker-build-ag-helper IMG=ghcr.io/yourorg/ag-helper:v1.0.1
make docker-push-ag-helper IMG=ghcr.io/yourorg/ag-helper:v1.0.1
```

**Step 2: Update the OperatorConfiguration (if using custom images)**

Edit your OperatorConfiguration to specify the new AG Helper image:

```yaml
apiVersion: mssql.microsoft.com/v1alpha1
kind: OperatorConfiguration
metadata:
  name: mssql-operator-config
  namespace: mssql-system
spec:
  imageConfiguration:
    agHelper:
      repository: ghcr.io/yourorg/ag-helper
      tag: v1.0.1
```

Apply the updated configuration:

```bash
kubectl apply -f operator-configuration.yaml
```

**Step 3: Delete existing AG Helper pods to pick up the new image**

```bash
# List AG Helper pods
kubectl get pods -n mssql -l app.kubernetes.io/component=ag-helper

# Delete them to force recreation with new image
kubectl delete pods -n mssql -l app.kubernetes.io/component=ag-helper
```

**Expected output:**
```
pod "productionag-ag-helper" deleted
```

The operator will automatically recreate the AG Helper pods with the new image.

### Minikube Quick Patching

When developing locally with minikube, use this streamlined workflow for fast iteration:

**Quick Patch Script (All-in-One)**

```bash
# 1. Point Docker to minikube's daemon
eval $(minikube docker-env)

# 2. Build the operator image locally
make docker-build IMG=mssql-operator:latest

# 3. Restart the operator to pick up changes
kubectl rollout restart deployment/mssql-operator -n mssql-system

# 4. Watch the logs
kubectl logs -f deployment/mssql-operator -n mssql-system --tail=50
```

**With CRD Changes**

```bash
# Point Docker to minikube
eval $(minikube docker-env)

# Regenerate CRDs and build
make manifests
make docker-build IMG=mssql-operator:latest

# Apply CRDs and restart operator
kubectl apply -f deploy/crds/ --force
kubectl rollout restart deployment/mssql-operator -n mssql-system

# Watch logs
kubectl logs -f deployment/mssql-operator -n mssql-system --tail=50
```

**Quick Patch AG Helper**

```bash
eval $(minikube docker-env)
make docker-build-ag-helper IMG=ag-helper:latest
kubectl delete pods -n mssql -l app.kubernetes.io/component=ag-helper
```

> **Tip:** Create a shell alias for quick iteration:
> ```bash
> alias patch-operator='eval $(minikube docker-env) && make docker-build IMG=mssql-operator:latest && kubectl rollout restart deployment/mssql-operator -n mssql-system'
> ```

### Patching Without Downtime

Both the SQLServer Operator and AG Helper support hot-patching without affecting running SQL Server workloads:

| Component | Restart Impact | Workload Impact |
|-----------|----------------|-----------------|
| mssql-operator | Brief pause in reconciliation | None - SQL Server pods continue running |
| AG Helper | Brief pause in health monitoring | None - AG continues functioning |

**What happens during operator restart:**

1. The operator deployment receives a rolling update
2. For a few seconds, no new reconciliation occurs
3. Once the new pod is ready, reconciliation resumes
4. Any pending changes are picked up and applied

> **Important:** Restarting the operator does NOT restart your SQL Server pods or affect running databases. Your workloads continue uninterrupted.

## Version Upgrade

### Supported Upgrade Paths

| From | To | Supported |
|------|-----|-----------|
| 2017 | 2019 | ✅ Yes |
| 2017 | 2022 | ✅ Yes |
| 2019 | 2022 | ✅ Yes |
| 2022 | 2019 | ❌ No (downgrade) |

### Update SQLServer Resource

**Step 1: Edit your SQLServer manifest file**

Open your existing SQLServer manifest file:

```bash
nano sqlserver-prod.yaml
```

Change the version field:

```yaml
apiVersion: mssql.microsoft.com/v1alpha1
kind: SQLServer
metadata:
  name: sql-prod
spec:
  version: "2022"  # Changed from "2019"
  # ... rest unchanged
```

### Apply Update

**Step 2: Apply the updated manifest**

```bash
kubectl apply -f sqlserver-prod.yaml
```

**Expected output:**
```
sqlserver.mssql.microsoft.com/sql-prod configured
```

**Step 3: Watch the upgrade progress**

```bash
kubectl get pods -n mssql -w

# You'll see pods restart one by one:
# sql-prod-0   1/1     Terminating   0          5d
# sql-prod-0   0/1     Pending       0          0s
# sql-prod-0   0/1     ContainerCreating   0   0s
# sql-prod-0   1/1     Running       0          45s
```

### What Happens

1. Controller detects version change
2. StatefulSet image updated
3. Pods restarted in order (0, 1, 2...)
4. Each pod waits for Ready before next

## Cumulative Update (CU)

Cumulative Updates are regularly released patches that include bug fixes and security updates.

### Check Current CU

First, set the SA password environment variable:

```bash
export SA_PWD=$(kubectl get secret sql-prod-sa -n mssql -o jsonpath='{.data.password}' | base64 -d)
```

Then check the current version:

```bash
kubectl exec -it sql-prod-0 -n mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P "$SA_PWD" -C \
  -Q "SELECT @@VERSION"
```

**Expected output (example):**
```
Microsoft SQL Server 2022 (RTM-CU10) (KB5031778) - 16.0.4095.4 (X64) 
    Oct 30 2023 16:12:44 
    ...
```

### Update Image Tag

The operator uses image tags in format: `2022-CU10-ubuntu-22.04`

**Option 1: Specify exact image**

Edit your SQLServer manifest:

```bash
nano sqlserver-prod.yaml
```

Add or update the image field:

```yaml
spec:
  version: "2022"
  instance:
    # Specify exact image (optional)
    image: mcr.microsoft.com/mssql/server:2022-CU15-ubuntu-22.04
```

**Option 2: Let the operator choose the latest CU**

Simply update the version and remove any image specification:

```yaml
spec:
  version: "2022"
  # No image specified = operator uses latest CU for 2022
```

**Apply the change:**

```bash
kubectl apply -f sqlserver-prod.yaml
```

**Expected output:**
```
sqlserver.mssql.microsoft.com/sql-prod configured
```

**Verify the upgrade:**

```bash
# Wait for pod to restart, then check version again
kubectl exec -it sql-prod-0 -n mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P "$SA_PWD" -C \
  -Q "SELECT @@VERSION"
```

## Rolling Upgrade (HA)

For deployments with Availability Groups, the operator performs rolling upgrades.

### Rolling Upgrade Flow

```
┌────────────────────────────────────────────────────────────────────┐
│                     Rolling Upgrade Process                         │
├────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  1. Pre-flight checks                                               │
│     └─ Verify all replicas healthy                                 │
│     └─ Verify AG synchronized                                      │
│                                                                     │
│  2. Upgrade secondaries first                                       │
│     ┌─────────────────────────────────────────────────────┐        │
│     │  Pod 2 (SECONDARY)                                   │        │
│     │    a. Drain connections                              │        │
│     │    b. Suspend database sync                          │        │
│     │    c. Delete pod (triggers recreate)                 │        │
│     │    d. Wait for new pod Ready                         │        │
│     │    e. Resume database sync                           │        │
│     │    f. Wait for SYNCHRONIZED                          │        │
│     └─────────────────────────────────────────────────────┘        │
│                                                                     │
│     Repeat for Pod 1 (SECONDARY)                                   │
│                                                                     │
│  3. Failover to upgraded secondary                                  │
│     └─ Automatic failover to pod 1 or 2                            │
│                                                                     │
│  4. Upgrade old primary                                             │
│     └─ Pod 0 now secondary, upgrade same as above                  │
│                                                                     │
│  5. (Optional) Fail back to original primary                       │
│                                                                     │
└────────────────────────────────────────────────────────────────────┘
```

### Upgrade Configuration

```yaml
apiVersion: mssql.microsoft.com/v1alpha1
kind: SQLServer
spec:
  upgrade:
    strategy: RollingUpdate
    
    # Wait time between pods
    minReadySeconds: 60
    
    # Max pods unavailable during upgrade
    maxUnavailable: 1
    
    # Automatically failover during upgrade
    autoFailover: true
    
    # Fail back after upgrade completes
    failbackAfterUpgrade: true
```

### Monitor Upgrade Progress

**Watch pod updates:**

```bash
kubectl get pods -n mssql -w
```

**Expected output during rolling upgrade:**
```
NAME         READY   STATUS        RESTARTS   AGE
sql-prod-0   1/1     Running       0          5d
sql-prod-1   1/1     Running       0          5d
sql-prod-2   0/1     Terminating   0          5d
sql-prod-2   0/1     Pending       0          0s
sql-prod-2   0/1     ContainerCreating   0   0s
sql-prod-2   1/1     Running       0          45s
sql-prod-1   0/1     Terminating   0          5d
...
```

**Check rollout status:**

```bash
kubectl rollout status statefulset/sql-prod -n mssql
```

**Expected output:**
```
Waiting for 1 pods to be ready...
statefulset rolling update complete 3 pods at revision sql-prod-7d8b9c6f5...
```

**Check AG sync after each pod:**

```bash
kubectl exec -it sql-prod-0 -n mssql -c ag-helper -- \
  curl -s localhost:8080/state | jq '.availabilityGroups[0].databases'
```

**Expected output (all databases should show SYNCHRONIZED):**
```json
[
  {
    "name": "AppDB",
    "synchronizationState": "SYNCHRONIZED",
    "synchronizationHealth": "HEALTHY"
  }
]
```

## Pre-Upgrade Checklist

Before upgrading, verify your environment is healthy and backed up.

### Verify Current State

**Step 1: Check all pods are running**

```bash
kubectl get pods -n mssql
```

**Expected output:**
```
NAME         READY   STATUS    RESTARTS   AGE
sql-prod-0   1/1     Running   0          5d
sql-prod-1   1/1     Running   0          5d
sql-prod-2   1/1     Running   0          5d
```

**Step 2: Verify AG is healthy**

```bash
kubectl exec -it sql-prod-0 -n mssql -c ag-helper -- \
  curl -s localhost:8080/state | jq '.health'
```

**Expected output:**
```
"Healthy"
```

**Step 3: Verify all databases are synchronized**

```bash
export SA_PWD=$(kubectl get secret sql-prod-sa -n mssql -o jsonpath='{.data.password}' | base64 -d)

kubectl exec -it sql-prod-0 -n mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P "$SA_PWD" -C \
  -Q "SELECT database_name, synchronization_state_desc FROM sys.dm_hadr_database_replica_states"
```

**Expected output:**
```
database_name    synchronization_state_desc
---------------  --------------------------
AppDB            SYNCHRONIZED
AppDB            SYNCHRONIZED
AppDB            SYNCHRONIZED
```

### Backup Before Upgrade

Create a full backup of all critical databases:

```bash
kubectl exec -it sql-prod-0 -n mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P "$SA_PWD" -C \
  -Q "BACKUP DATABASE AppDB TO DISK = '/var/opt/mssql/backup/AppDB_pre_upgrade.bak' WITH COMPRESSION"
```

**Expected output:**
```
Processed 336 pages for database 'AppDB', file 'AppDB' on file 1.
BACKUP DATABASE successfully processed 336 pages in 0.156 seconds.
```

### Check Compatibility

**Check database compatibility levels:**

```bash
kubectl exec -it sql-prod-0 -n mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P "$SA_PWD" -C \
  -Q "SELECT name, compatibility_level FROM sys.databases"
```

**Expected output:**
```
name         compatibility_level
-----------  -------------------
master       150
tempdb       150
model        150
msdb         150
AppDB        150
```

> **Note:** Compatibility level 150 = SQL Server 2019, 160 = SQL Server 2022

### Resource Headroom

Ensure your cluster has capacity for the upgrade (pods may temporarily use more resources during restart):

```bash
kubectl describe nodes | grep -A 5 "Allocated resources"
```

**Look for:** At least 2GB free memory and 1 CPU core per node.

## Post-Upgrade Validation

After the upgrade completes, verify everything is working correctly.

### Verify Version

```bash
export SA_PWD=$(kubectl get secret sql-prod-sa -n mssql -o jsonpath='{.data.password}' | base64 -d)

kubectl exec -it sql-prod-0 -n mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P "$SA_PWD" -C \
  -Q "SELECT @@VERSION"
```

**Expected output (for SQL Server 2022 CU15):**
```
Microsoft SQL Server 2022 (RTM-CU15) (KB5037331) - 16.0.4150.1 (X64)
    Sep 25 2024 15:53:42
    ...
```

### Check AG Status

```bash
kubectl exec -it sql-prod-0 -n mssql -c ag-helper -- \
  curl -s localhost:8080/state | jq
```

**Expected output:**
```json
{
  "health": "Healthy",
  "isPrimary": true,
  "availabilityGroups": [
    {
      "name": "ProductionAG",
      "databases": [
        {
          "name": "AppDB",
          "synchronizationState": "SYNCHRONIZED",
          "synchronizationHealth": "HEALTHY"
        }
      ]
    }
  ]
}
```

### Run Application Tests

**Test basic connectivity:**

```bash
kubectl exec -it sql-prod-0 -n mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P "$SA_PWD" -C \
  -Q "SELECT 1 AS connectivity_test"
```

**Expected output:**
```
connectivity_test
-----------------
              1
```

**Test database access:**

```bash
kubectl exec -it sql-prod-0 -n mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P "$SA_PWD" -C \
  -Q "USE AppDB; SELECT COUNT(*) AS row_count FROM YourTable"
```

### Update Compatibility Level

After a major version upgrade (e.g., 2019 → 2022), update the database compatibility level to take advantage of new features:

```bash
kubectl exec -it sql-prod-0 -n mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P "$SA_PWD" -C \
  -Q "ALTER DATABASE AppDB SET COMPATIBILITY_LEVEL = 160"
```

**Expected output:**
```
Command(s) completed successfully.
```

> **Note:** Compatibility levels: 140 = SQL 2017, 150 = SQL 2019, 160 = SQL 2022

## Rollback

If an upgrade fails or causes issues, you may need to rollback.

### Before Rollback

> ⚠️ **Warning**: Major version rollbacks require restore from backup. CU rollbacks may work by reverting image.

### CU Rollback (Image Revert)

If you upgraded from CU10 to CU15 and need to revert:

**Step 1: Edit your SQLServer manifest**

```bash
nano sqlserver-prod.yaml
```

Change the image back to the previous version:

```yaml
spec:
  instance:
    image: mcr.microsoft.com/mssql/server:2022-CU10-ubuntu-22.04  # Previous version
```

**Step 2: Apply the change**

```bash
kubectl apply -f sqlserver-prod.yaml
```

**Expected output:**
```
sqlserver.mssql.microsoft.com/sql-prod configured
```

**Step 3: Monitor the rollback**

```bash
kubectl get pods -n mssql -w
```

### Major Version Rollback

Major version rollbacks (e.g., 2022 → 2019) require creating a new instance and restoring from backup.

**Step 1: Create a new SQLServer manifest for the previous version**

Create a file named `sqlserver-prod-2019.yaml`:

```bash
nano sqlserver-prod-2019.yaml
```

Paste your configuration with the previous version:

```yaml
apiVersion: mssql.microsoft.com/v1alpha1
kind: SQLServer
metadata:
  name: sql-prod-2019
  namespace: mssql
spec:
  version: "2019"
  # ... same configuration as before
  credentials:
    saPasswordSecretRef:
      name: sql-prod-sa
      key: password
```

**Step 2: Deploy the new instance**

```bash
kubectl apply -f sqlserver-prod-2019.yaml
```

**Expected output:**
```
sqlserver.mssql.microsoft.com/sql-prod-2019 created
```

**Step 3: Wait for the new instance to be ready**

```bash
kubectl get pods -n mssql -l app=sql-prod-2019 -w
```

**Step 4: Restore from the pre-upgrade backup**

```bash
export SA_PWD=$(kubectl get secret sql-prod-sa -n mssql -o jsonpath='{.data.password}' | base64 -d)

kubectl exec -it sql-prod-2019-0 -n mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P "$SA_PWD" -C \
  -Q "RESTORE DATABASE AppDB FROM DISK = '/var/opt/mssql/backup/AppDB_pre_upgrade.bak' WITH REPLACE"
```

**Expected output:**
```
Processed 336 pages for database 'AppDB', file 'AppDB' on file 1.
RESTORE DATABASE successfully processed 336 pages in 0.234 seconds.
```

**Step 5: Update your services to point to the new instance**

Edit your application configuration or update Kubernetes Services to point to `sql-prod-2019` instead of `sql-prod`.

**Step 6: Delete the failed instance (optional, after verification)**

```bash
kubectl delete sqlserver sql-prod -n mssql
```

## Best Practices

| Practice | Description |
|----------|-------------|
| Test in non-prod | Always upgrade dev/staging first |
| Backup first | Full backup before any upgrade |
| Monitor closely | Watch metrics during upgrade |
| Schedule window | Plan for rollback time |
| Document version | Record current/target versions |

## Next Steps

- [Scaling](scaling.md) - Adjust replicas
- [Backup & Restore](backup-restore.md) - Data protection
- [Troubleshooting](../user-guide/troubleshooting.md) - Common issues
