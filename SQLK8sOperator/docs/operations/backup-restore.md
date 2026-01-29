# Backup and Restore

[← Back to Operations](../README.md) | [Documentation Home](../README.md)

Guide to backing up and restoring SQL Server databases in Kubernetes.

## Table of Contents

- [Backup Overview](#backup-overview)
- [Manual Backup](#manual-backup)
- [Automated Backups](#automated-backups)
- [Backup to Cloud Storage](#backup-to-cloud-storage)
- [Restore](#restore)
- [Point-in-Time Recovery](#point-in-time-recovery)
- [Disaster Recovery](#disaster-recovery)

## Backup Overview

### Backup Types

| Type | Description | Use Case |
|------|-------------|----------|
| Full | Complete database | Weekly, before changes |
| Differential | Changes since last full | Daily |
| Transaction Log | Log records | Every 15-60 min |

### Backup Storage Options

| Location | Pros | Cons |
|----------|------|------|
| PVC (backup volume) | Fast, local | Node failure risk |
| Azure Blob Storage | Durable, cheap | Network latency |
| AWS S3 | Durable, cheap | Network latency |
| NFS | Shared storage | Setup complexity |

## Manual Backup

Before running backup commands, you need to set the SA password as an environment variable:

```bash
# Get the SA password from the secret
export SA_PWD=$(kubectl get secret sql-prod-sa -n mssql -o jsonpath='{.data.password}' | base64 -d)

# Verify (optional - shows first few characters)
echo "${SA_PWD:0:3}..."
```

### Full Backup

Run the following command to create a full backup of the AppDB database:

```bash
kubectl exec -it sql-prod-0 -n mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P "$SA_PWD" -C \
  -Q "BACKUP DATABASE AppDB TO DISK = '/var/opt/mssql/backup/AppDB_full_$(date +%Y%m%d).bak' WITH COMPRESSION, INIT"
```

**Expected output:**
```
Processed 336 pages for database 'AppDB', file 'AppDB' on file 1.
Processed 2 pages for database 'AppDB', file 'AppDB_log' on file 1.
BACKUP DATABASE successfully processed 338 pages in 0.156 seconds (16.920 MB/sec).
```

### Differential Backup

Run the following command to create a differential backup (only changes since last full backup):

```bash
kubectl exec -it sql-prod-0 -n mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P "$SA_PWD" -C \
  -Q "BACKUP DATABASE AppDB TO DISK = '/var/opt/mssql/backup/AppDB_diff_$(date +%Y%m%d%H%M).bak' WITH DIFFERENTIAL, COMPRESSION"
```

**Expected output:**
```
Processed 48 pages for database 'AppDB', file 'AppDB' on file 1.
Processed 1 pages for database 'AppDB', file 'AppDB_log' on file 1.
BACKUP DATABASE WITH DIFFERENTIAL successfully processed 49 pages in 0.042 seconds (9.125 MB/sec).
```

### Transaction Log Backup

Run the following command to back up the transaction log:

```bash
kubectl exec -it sql-prod-0 -n mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P "$SA_PWD" -C \
  -Q "BACKUP LOG AppDB TO DISK = '/var/opt/mssql/backup/AppDB_log_$(date +%Y%m%d%H%M).trn' WITH COMPRESSION"
```

**Expected output:**
```
Processed 24 pages for database 'AppDB', file 'AppDB_log' on file 1.
BACKUP LOG successfully processed 24 pages in 0.012 seconds (15.625 MB/sec).
```

### Verify Backup

Run the following command to verify a backup file is valid without actually restoring it:

```bash
kubectl exec -it sql-prod-0 -n mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P "$SA_PWD" -C \
  -Q "RESTORE VERIFYONLY FROM DISK = '/var/opt/mssql/backup/AppDB_full_20240115.bak'"
```

**Expected output:**
```
The backup set on file 1 is valid.
```

### List Backup Files

To see all backups on the pod:

```bash
kubectl exec -it sql-prod-0 -n mssql -- ls -lh /var/opt/mssql/backup/

# Expected output:
# total 12M
# -rw-r----- 1 mssql mssql 10M Jan 15 02:00 AppDB_full_20240115.bak
# -rw-r----- 1 mssql mssql 1.2M Jan 15 10:15 AppDB_log_202401151015.trn
# -rw-r----- 1 mssql mssql 800K Jan 18 02:00 AppDB_diff_202401180200.bak
```

## Automated Backups

Automated backups use Kubernetes CronJobs to schedule regular backups.

### Prerequisites

Before setting up automated backups, ensure:
1. Your SQL Server pods have a backup PVC mounted at `/var/opt/mssql/backup`
2. You have a Kubernetes secret named `sql-prod-sa` with the SA password

### CronJob for Full Backup

**Step 1: Create the CronJob file**

Create a file named `sql-backup-full-cronjob.yaml`:

```bash
# On Linux/macOS
nano sql-backup-full-cronjob.yaml

# On Windows (PowerShell)
notepad sql-backup-full-cronjob.yaml
```

Paste the following content and save:

```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: sql-backup-full
  namespace: mssql
spec:
  schedule: "0 2 * * 0"  # Every Sunday at 2 AM
  jobTemplate:
    spec:
      template:
        spec:
          containers:
            - name: backup
              image: mcr.microsoft.com/mssql-tools:latest
              command:
                - /bin/bash
                - -c
                - |
                  /opt/mssql-tools18/bin/sqlcmd \
                    -S sql-prod-0.sql-prod-pods.mssql.svc.cluster.local \
                    -U sa -P "$SA_PASSWORD" -C \
                    -Q "BACKUP DATABASE AppDB TO DISK = '/backup/AppDB_full_$(date +%Y%m%d).bak' WITH COMPRESSION, INIT"
              env:
                - name: SA_PASSWORD
                  valueFrom:
                    secretKeyRef:
                      name: sql-prod-sa
                      key: password
              volumeMounts:
                - name: backup
                  mountPath: /backup
          volumes:
            - name: backup
              persistentVolumeClaim:
                claimName: sql-prod-0-backup
          restartPolicy: OnFailure
```

**Step 2: Apply the CronJob**

```bash
kubectl apply -f sql-backup-full-cronjob.yaml
```

**Expected output:**
```
cronjob.batch/sql-backup-full created
```

**Step 3: Verify the CronJob was created**

```bash
kubectl get cronjob -n mssql

# Expected output:
# NAME              SCHEDULE      SUSPEND   ACTIVE   LAST SCHEDULE   AGE
# sql-backup-full   0 2 * * 0     False     0        <none>          5s
```

**Step 4: Test the CronJob (optional)**

To test immediately without waiting for the schedule:

```bash
kubectl create job --from=cronjob/sql-backup-full sql-backup-test -n mssql

# Watch the job
kubectl get jobs -n mssql -w

# Check job logs
kubectl logs job/sql-backup-test -n mssql
```

### CronJob for Log Backup

**Step 1: Create the CronJob file**

Create a file named `sql-backup-log-cronjob.yaml`:

```bash
nano sql-backup-log-cronjob.yaml
```

Paste the following content and save:

```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: sql-backup-log
  namespace: mssql
spec:
  schedule: "*/15 * * * *"  # Every 15 minutes
  jobTemplate:
    spec:
      template:
        spec:
          containers:
            - name: backup
              image: mcr.microsoft.com/mssql-tools:latest
              command:
                - /bin/bash
                - -c
                - |
                  /opt/mssql-tools18/bin/sqlcmd \
                    -S sql-prod-0.sql-prod-pods.mssql.svc.cluster.local \
                    -U sa -P "$SA_PASSWORD" -C \
                    -Q "BACKUP LOG AppDB TO DISK = '/backup/AppDB_log_$(date +%Y%m%d%H%M).trn' WITH COMPRESSION"
              env:
                - name: SA_PASSWORD
                  valueFrom:
                    secretKeyRef:
                      name: sql-prod-sa
                      key: password
              volumeMounts:
                - name: backup
                  mountPath: /backup
          volumes:
            - name: backup
              persistentVolumeClaim:
                claimName: sql-prod-0-backup
          restartPolicy: OnFailure
```

**Step 2: Apply the CronJob**

```bash
kubectl apply -f sql-backup-log-cronjob.yaml
```

**Expected output:**
```
cronjob.batch/sql-backup-log created
```

**Step 3: Verify all backup CronJobs**

```bash
kubectl get cronjob -n mssql

# Expected output:
# NAME              SCHEDULE        SUSPEND   ACTIVE   LAST SCHEDULE   AGE
# sql-backup-full   0 2 * * 0       False     0        <none>          5m
# sql-backup-log    */15 * * * *    False     0        <none>          5s
```

### Cleanup Old Backups

**Step 1: Create the cleanup CronJob file**

Create a file named `sql-backup-cleanup-cronjob.yaml`:

```bash
nano sql-backup-cleanup-cronjob.yaml
```

Paste the following content and save:

```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: sql-backup-cleanup
  namespace: mssql
spec:
  schedule: "0 4 * * 0"  # Every Sunday at 4 AM
  jobTemplate:
    spec:
      template:
        spec:
          containers:
            - name: cleanup
              image: busybox
              command:
                - /bin/sh
                - -c
                - |
                  # Keep last 4 full backups
                  find /backup -name "*.bak" -mtime +28 -delete
                  # Keep last 7 days of log backups
                  find /backup -name "*.trn" -mtime +7 -delete
              volumeMounts:
                - name: backup
                  mountPath: /backup
          volumes:
            - name: backup
              persistentVolumeClaim:
                claimName: sql-prod-0-backup
          restartPolicy: OnFailure
```

**Step 2: Apply the cleanup CronJob**

```bash
kubectl apply -f sql-backup-cleanup-cronjob.yaml
```

**Expected output:**
```
cronjob.batch/sql-backup-cleanup created
```

**Step 3: Verify all CronJobs**

```bash
kubectl get cronjob -n mssql

# Expected output:
# NAME                 SCHEDULE        SUSPEND   ACTIVE   LAST SCHEDULE   AGE
# sql-backup-cleanup   0 4 * * 0       False     0        <none>          5s
# sql-backup-full      0 2 * * 0       False     0        <none>          10m
# sql-backup-log       */15 * * * *    False     0        45s             5m
```

## Backup to Cloud Storage

### Azure Blob Storage

You can back up directly to Azure Blob Storage using SQL Server's native URL backup feature.

**Step 1: Get your Azure Blob Storage SAS token**

In the Azure Portal, navigate to your storage account → Shared access signature → Generate SAS and connection string. Copy the SAS token (starts with `sv=`).

**Step 2: Create the SQL credential**

Connect to your SQL Server and run:

```bash
kubectl exec -it sql-prod-0 -n mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P "$SA_PWD" -C \
  -Q "CREATE CREDENTIAL AzureBlobCredential WITH IDENTITY = 'SHARED ACCESS SIGNATURE', SECRET = 'sv=2021-06-01&ss=b&srt=sco&sp=rwdlacup&se=...YOUR_SAS_TOKEN...'"
```

**Expected output:**
```
Command(s) completed successfully.
```

**Step 3: Backup to Azure Blob**

```bash
kubectl exec -it sql-prod-0 -n mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P "$SA_PWD" -C \
  -Q "BACKUP DATABASE AppDB TO URL = 'https://mystorageaccount.blob.core.windows.net/backups/AppDB_full.bak' WITH CREDENTIAL = 'AzureBlobCredential', COMPRESSION"
```

**Expected output:**
```
Processed 336 pages for database 'AppDB', file 'AppDB' on file 1.
BACKUP DATABASE successfully processed 336 pages in 2.345 seconds (1.120 MB/sec).
```

**Step 4: Verify in Azure Portal**

Navigate to your storage account → Containers → backups to see the backup file.

### AWS S3 (via Sidecar Sync)

SQL Server doesn't natively support S3, so we use a sidecar container to sync local backups to S3.

**Step 1: Create AWS credentials secret**

First, create a Kubernetes secret with your AWS credentials:

```bash
kubectl create secret generic aws-creds \
  --from-literal=access-key=YOUR_AWS_ACCESS_KEY_ID \
  --from-literal=secret-key=YOUR_AWS_SECRET_ACCESS_KEY \
  -n mssql
```

**Expected output:**
```
secret/aws-creds created
```

**Step 2: Add the sync sidecar to your SQL Server deployment**

Add the following container to your SQLServer spec. Create a file named `sql-with-s3-sync.yaml`:

```bash
nano sql-with-s3-sync.yaml
```

Add this sidecar container section to your SQLServer spec:

```yaml
spec:
  additionalContainers:
    - name: backup-sync
      image: amazon/aws-cli:latest
      command:
        - /bin/sh
        - -c
        - |
          while true; do
            aws s3 sync /backup s3://mybucket/sql-backups/ --delete
            sleep 3600
          done
      volumeMounts:
        - name: backup
          mountPath: /backup
      env:
        - name: AWS_ACCESS_KEY_ID
          valueFrom:
            secretKeyRef:
              name: aws-creds
              key: access-key
        - name: AWS_SECRET_ACCESS_KEY
          valueFrom:
            secretKeyRef:
              name: aws-creds
              key: secret-key
```

**Step 3: Apply the updated configuration**

```bash
kubectl apply -f sql-with-s3-sync.yaml
```

**Step 4: Verify the sidecar is running**

```bash
kubectl get pods -n mssql -l app=sql-prod -o jsonpath='{.items[*].spec.containers[*].name}'

# Expected output should include backup-sync:
# mssql backup-sync
```

## Restore

Before running restore commands, set the SA password as an environment variable:

```bash
export SA_PWD=$(kubectl get secret sql-prod-sa -n mssql -o jsonpath='{.data.password}' | base64 -d)
```

### Restore Full Backup

Restore a database from a full backup, replacing any existing database:

```bash
kubectl exec -it sql-prod-0 -n mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P "$SA_PWD" -C \
  -Q "RESTORE DATABASE AppDB FROM DISK = '/var/opt/mssql/backup/AppDB_full_20240115.bak' WITH REPLACE, RECOVERY"
```

**Expected output:**
```
Processed 336 pages for database 'AppDB', file 'AppDB' on file 1.
Processed 2 pages for database 'AppDB', file 'AppDB_log' on file 1.
RESTORE DATABASE successfully processed 338 pages in 0.234 seconds (11.274 MB/sec).
```

**Verify the restore:**

```bash
kubectl exec -it sql-prod-0 -n mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P "$SA_PWD" -C \
  -Q "SELECT name, state_desc FROM sys.databases WHERE name = 'AppDB'"

# Expected output:
# name     state_desc
# -------- ----------
# AppDB    ONLINE
```

### Restore with NORECOVERY (for additional restores)

Use `NORECOVERY` when you plan to restore additional differential or log backups:

```bash
kubectl exec -it sql-prod-0 -n mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P "$SA_PWD" -C \
  -Q "RESTORE DATABASE AppDB FROM DISK = '/var/opt/mssql/backup/AppDB_full_20240115.bak' WITH REPLACE, NORECOVERY"
```

**Expected output:**
```
Processed 336 pages for database 'AppDB', file 'AppDB' on file 1.
RESTORE DATABASE successfully processed 336 pages in 0.234 seconds (11.274 MB/sec).
```

> **Note:** The database will be in RESTORING state until you run a final restore with RECOVERY.

### Restore Differential

After restoring a full backup with NORECOVERY, restore the differential:

```bash
kubectl exec -it sql-prod-0 -n mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P "$SA_PWD" -C \
  -Q "RESTORE DATABASE AppDB FROM DISK = '/var/opt/mssql/backup/AppDB_diff_20240118.bak' WITH NORECOVERY"
```

**Expected output:**
```
Processed 48 pages for database 'AppDB', file 'AppDB' on file 1.
RESTORE DATABASE WITH DIFFERENTIAL successfully processed 48 pages in 0.042 seconds.
```

### Restore Transaction Logs

After restoring full and differential backups, restore transaction logs in order:

```bash
# Restore multiple log backups
for log in AppDB_log_202401180000.trn AppDB_log_202401180015.trn; do
  kubectl exec -it sql-prod-0 -n mssql -- \
    /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P "$SA_PWD" -C \
    -Q "RESTORE LOG AppDB FROM DISK = '/var/opt/mssql/backup/$log' WITH NORECOVERY"
done
```

**Expected output (for each log):**
```
Processed 24 pages for database 'AppDB', file 'AppDB_log' on file 1.
RESTORE LOG successfully processed 24 pages in 0.012 seconds.
```

**Final restore with recovery to bring database online:**

```bash
kubectl exec -it sql-prod-0 -n mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P "$SA_PWD" -C \
  -Q "RESTORE DATABASE AppDB WITH RECOVERY"
```

**Expected output:**
```
RESTORE DATABASE successfully processed 0 pages in 0.001 seconds.
```

**Verify the database is online:**

```bash
kubectl exec -it sql-prod-0 -n mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P "$SA_PWD" -C \
  -Q "SELECT name, state_desc FROM sys.databases WHERE name = 'AppDB'"

# Expected output:
# name     state_desc
# -------- ----------
# AppDB    ONLINE
```

## Point-in-Time Recovery

Point-in-time recovery allows you to restore a database to a specific moment before data was corrupted or deleted.

### Restore to Specific Time

**Step 1: Identify the target time**

Determine the exact time you want to restore to (before the data corruption/deletion occurred).

**Step 2: Restore the full backup with NORECOVERY**

```bash
kubectl exec -it sql-prod-0 -n mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P "$SA_PWD" -C \
  -Q "RESTORE DATABASE AppDB FROM DISK = '/var/opt/mssql/backup/AppDB_full_20240115.bak' WITH NORECOVERY, REPLACE"
```

**Expected output:**
```
Processed 336 pages for database 'AppDB', file 'AppDB' on file 1.
RESTORE DATABASE successfully processed 336 pages in 0.234 seconds.
```

**Step 3: Restore logs up to the specific time**

Use `STOPAT` to specify the exact recovery point:

```bash
kubectl exec -it sql-prod-0 -n mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P "$SA_PWD" -C \
  -Q "RESTORE LOG AppDB FROM DISK = '/var/opt/mssql/backup/AppDB_log_20240118.trn' WITH STOPAT = '2024-01-18T10:30:00', RECOVERY"
```

**Expected output:**
```
Processed 24 pages for database 'AppDB', file 'AppDB_log' on file 1.
RESTORE LOG successfully processed 24 pages in 0.012 seconds.
```

**Step 4: Verify the database is online**

```bash
kubectl exec -it sql-prod-0 -n mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P "$SA_PWD" -C \
  -Q "SELECT name, state_desc FROM sys.databases WHERE name = 'AppDB'"

# Expected output:
# name     state_desc
# -------- ----------
# AppDB    ONLINE
```

### Mark Transaction for Recovery

For important operations, mark transactions so you can restore to that exact point later.

**Step 1: Mark a transaction before critical operations**

Run this in your application or SQL session before making important changes:

```bash
kubectl exec -it sql-prod-0 -n mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P "$SA_PWD" -C \
  -Q "BEGIN TRANSACTION PreUpdate WITH MARK 'Before schema update'; /* Your operations here */ COMMIT TRANSACTION;"
```

**Expected output:**
```
Command(s) completed successfully.
```

**Step 2: Later, if needed, restore to that marked transaction**

```bash
kubectl exec -it sql-prod-0 -n mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P "$SA_PWD" -C \
  -Q "RESTORE LOG AppDB FROM DISK = '/var/opt/mssql/backup/AppDB_log.trn' WITH STOPATMARK = 'PreUpdate', RECOVERY"
```

**Expected output:**
```
Processed 24 pages for database 'AppDB', file 'AppDB_log' on file 1.
RESTORE LOG successfully processed 24 pages in 0.012 seconds.
```

## Disaster Recovery

### Recovery Scenarios

| Scenario | Recovery Method |
|----------|-----------------|
| Pod failure | AG automatic failover |
| Node failure | Pod reschedules, AG failover |
| Zone failure | Cross-zone replica promotion |
| Region failure | Restore from off-site backup |
| Data corruption | Point-in-time restore |

### Cross-Cluster Restore

When you need to restore a database to a different Kubernetes cluster (e.g., for disaster recovery testing or actual DR):

**Step 1: Copy backup from source cluster to local machine**

```bash
kubectl cp sql-prod-0:/var/opt/mssql/backup/AppDB_full.bak \
  ./AppDB_full.bak -n mssql
```

**Expected output:**
```
tar: Removing leading `/' from member names
```

**Verify the file was copied:**

```bash
ls -lh AppDB_full.bak

# Expected output:
# -rw-r--r--  1 user  staff  10M Jan 15 10:00 AppDB_full.bak
```

**Step 2: Copy backup to DR cluster**

Switch your kubectl context to the DR cluster, then copy the file:

```bash
# Switch to DR cluster context
kubectl config use-context dr-cluster

# Copy backup to DR cluster
kubectl cp ./AppDB_full.bak \
  sql-dr-0:/var/opt/mssql/backup/AppDB_full.bak -n mssql-dr
```

**Expected output:**
```
(no output on success)
```

**Step 3: Verify backup was copied to DR cluster**

```bash
kubectl exec -it sql-dr-0 -n mssql-dr -- ls -lh /var/opt/mssql/backup/

# Expected output:
# total 10M
# -rw-r--r-- 1 mssql mssql 10M Jan 15 10:05 AppDB_full.bak
```

**Step 4: Restore on DR cluster**

Set the SA password for the DR cluster:

```bash
export SA_PWD_DR=$(kubectl get secret sql-dr-sa -n mssql-dr -o jsonpath='{.data.password}' | base64 -d)
```

Restore the database:

```bash
kubectl exec -it sql-dr-0 -n mssql-dr -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P "$SA_PWD_DR" -C \
  -Q "RESTORE DATABASE AppDB FROM DISK = '/var/opt/mssql/backup/AppDB_full.bak' WITH REPLACE"
```

**Expected output:**
```
Processed 336 pages for database 'AppDB', file 'AppDB' on file 1.
RESTORE DATABASE successfully processed 336 pages in 0.234 seconds.
```

**Step 5: Verify the restore on DR cluster**

```bash
kubectl exec -it sql-dr-0 -n mssql-dr -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P "$SA_PWD_DR" -C \
  -Q "SELECT name, state_desc, create_date FROM sys.databases WHERE name = 'AppDB'"

# Expected output:
# name     state_desc  create_date
# -------- ----------  -----------------------
# AppDB    ONLINE      2024-01-15 10:00:00.000
```

### Recovery Time Objectives

| Tier | RTO | Backup Strategy |
|------|-----|-----------------|
| Tier 1 | < 1 hour | AG with sync replicas |
| Tier 2 | < 4 hours | AG with async replicas |
| Tier 3 | < 24 hours | Log shipping |
| Tier 4 | < 1 week | Daily backups to cloud |

## Best Practices

| Practice | Description |
|----------|-------------|
| 3-2-1 Rule | 3 copies, 2 media types, 1 off-site |
| Test restores | Monthly restore testing |
| Document RPO/RTO | Define and meet objectives |
| Encrypt backups | Use backup encryption |
| Monitor backup jobs | Alert on failures |

## Next Steps

- [Active Directory](active-directory.md) - AD authentication
- [Upgrades](upgrades.md) - Version upgrades
- [Troubleshooting](../user-guide/troubleshooting.md) - Backup issues
