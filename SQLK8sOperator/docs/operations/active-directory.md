# Active Directory Integration

[← Back to Operations](../README.md) | [Documentation Home](../README.md)

Guide to configuring SQL Server with Active Directory authentication in Kubernetes.

## Table of Contents

- [Overview](#overview)
- [Prerequisites](#prerequisites)
- [Configuration Steps](#configuration-steps)
- [Kerberos Authentication](#kerberos-authentication)
- [Windows Authentication](#windows-authentication)
- [Troubleshooting AD](#troubleshooting-ad)

## Overview

Active Directory (AD) integration enables:

- Windows/Kerberos authentication
- Centralized user management
- Single sign-on (SSO)
- Group-based permissions

### Authentication Flow

```
┌─────────────────────────────────────────────────────────────────────┐
│                    AD Authentication Flow                            │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│   Client                  SQL Server                AD Domain        │
│     │                         │                       Controller     │
│     │                         │                            │         │
│     │ 1. Request ticket       │                            │         │
│     │ ─────────────────────────────────────────────────────▶│        │
│     │                         │                            │         │
│     │ 2. Kerberos ticket      │                            │         │
│     │ ◀─────────────────────────────────────────────────────│        │
│     │                         │                            │         │
│     │ 3. Connect with ticket  │                            │         │
│     │ ────────────────────────▶                            │         │
│     │                         │                            │         │
│     │                         │ 4. Validate ticket         │         │
│     │                         │ ───────────────────────────▶│        │
│     │                         │                            │         │
│     │                         │ 5. Ticket valid            │         │
│     │                         │ ◀───────────────────────────│        │
│     │                         │                            │         │
│     │ 6. Connection           │                            │         │
│     │    established          │                            │         │
│     │ ◀────────────────────────                            │         │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

## Prerequisites

| Requirement | Description |
|-------------|-------------|
| AD Domain | Accessible from Kubernetes cluster |
| DNS | Pods must resolve AD domain |
| Service Account | AD account for SQL Server |
| Keytab | Kerberos keytab file |
| SPN | Service Principal Name registered |

### Network Requirements

- Pods can reach AD DCs on ports 88 (Kerberos), 389 (LDAP), 636 (LDAPS)
- DNS configured to resolve AD domain
- Time synchronized with AD (NTP)

## Configuration Steps

### Step 1: Create AD Service Account

In Active Directory, create a service account for SQL Server:

```powershell
# On AD Domain Controller
New-ADUser -Name "sql-prod-svc" `
    -UserPrincipalName "sql-prod-svc@CONTOSO.COM" `
    -SamAccountName "sql-prod-svc" `
    -AccountPassword (ConvertTo-SecureString "P@ssw0rd!" -AsPlainText -Force) `
    -Enabled $true `
    -PasswordNeverExpires $true
```

### Step 2: Register SPN

```powershell
# Register SPN for SQL Server
setspn -A MSSQLSvc/sql-prod-0.mssql.svc.cluster.local:1433 CONTOSO\sql-prod-svc
setspn -A MSSQLSvc/sql-prod-primary.mssql.svc.cluster.local:1433 CONTOSO\sql-prod-svc
```

### Step 3: Create Keytab

On Windows, run the following command on a machine with access to the AD domain:

```powershell
ktpass -princ MSSQLSvc/sql-prod-0.mssql.svc.cluster.local:1433@CONTOSO.COM `
    -mapuser CONTOSO\sql-prod-svc `
    -pass "P@ssw0rd!" `
    -crypto AES256-SHA1 `
    -ptype KRB5_NT_PRINCIPAL `
    -out mssql.keytab
```

**Expected output:**
```
Targeting domain controller: dc01.contoso.com
Using legacy password setting method
Successfully mapped MSSQLSvc/sql-prod-0.mssql.svc.cluster.local:1433 to sql-prod-svc.
Output keytab to mssql.keytab:
```

Verify the keytab file was created:

```powershell
dir mssql.keytab

# Expected output:
# Mode                 LastWriteTime         Length Name
# ----                 -------------         ------ ----
# -a----          1/15/2024  10:00            214 mssql.keytab
```

### Step 4: Create Kubernetes Secrets

**Step 4a: Create the keytab secret**

```bash
kubectl create secret generic mssql-keytab \
    --from-file=mssql.keytab=./mssql.keytab \
    -n mssql
```

**Expected output:**
```
secret/mssql-keytab created
```

**Step 4b: Create the krb5.conf file**

Create a file named `krb5.conf`:

```bash
# On Linux/macOS
nano krb5.conf

# On Windows (PowerShell)
notepad krb5.conf
```

Paste the following content (replace `CONTOSO.COM` and `dc01.contoso.com` with your AD domain):

```ini
[libdefaults]
    default_realm = CONTOSO.COM
    dns_lookup_realm = false
    dns_lookup_kdc = true
    ticket_lifetime = 24h
    renew_lifetime = 7d
    forwardable = true
    rdns = false

[realms]
    CONTOSO.COM = {
        kdc = dc01.contoso.com
        admin_server = dc01.contoso.com
    }

[domain_realm]
    .contoso.com = CONTOSO.COM
    contoso.com = CONTOSO.COM
```

**Step 4c: Create the krb5.conf secret**

```bash
kubectl create secret generic mssql-krb5 \
    --from-file=krb5.conf=./krb5.conf \
    -n mssql
```

**Expected output:**
```
secret/mssql-krb5 created
```

**Step 4d: Verify both secrets were created**

```bash
kubectl get secrets -n mssql | grep mssql-k

# Expected output:
# mssql-keytab     Opaque   1      5s
# mssql-krb5       Opaque   1      2s
```

### Step 5: Configure SQLServer CR

**Step 5a: Create the SQLServer manifest file**

Create a file named `sql-prod-ad.yaml`:

```bash
nano sql-prod-ad.yaml
```

Paste the following content (replace AD-specific values with your environment):

```yaml
apiVersion: mssql.microsoft.com/v1alpha1
kind: SQLServer
metadata:
  name: sql-prod
  namespace: mssql
spec:
  version: "2022"
  edition: Enterprise
  
  instance:
    replicas: 3
    config:
      hadrEnabled: true
      agentEnabled: true
  
  # Active Directory configuration
  activeDirectory:
    enabled: true
    realm: CONTOSO.COM
    
    # Kerberos configuration
    kerberos:
      keytabSecretRef:
        name: mssql-keytab
        key: mssql.keytab
      krb5ConfSecretRef:
        name: mssql-krb5
        key: krb5.conf
    
    # Service account
    serviceAccount: sql-prod-svc
    
    # DNS configuration
    dnsServerIP: "10.0.0.4"  # AD DNS server
    
    # Additional settings
    adConnectorPort: 389
  
  credentials:
    saPasswordSecretRef:
      name: sql-prod-sa
      key: password
```

**Step 5b: Apply the SQLServer manifest**

```bash
kubectl apply -f sql-prod-ad.yaml
```

**Expected output:**
```
sqlserver.mssql.microsoft.com/sql-prod created
```

**Step 5c: Verify the SQLServer pods are running**

```bash
kubectl get pods -n mssql -w

# Wait until all pods are Running:
# NAME         READY   STATUS    RESTARTS   AGE
# sql-prod-0   1/1     Running   0          2m
# sql-prod-1   1/1     Running   0          1m
# sql-prod-2   1/1     Running   0          30s
```

## Kerberos Authentication

Once your SQL Server is configured with Active Directory, the operator automatically configures Kerberos authentication.

### mssql.conf for AD

The operator generates this configuration automatically (for reference):

```ini
[network]
privilegedadaccount = sql-prod-svc
kerberoskeytabfile = /var/opt/mssql/secrets/mssql.keytab

[security]
enablekerberos = true
```

### Volume Mounts (Generated by Operator)

The operator creates these volume mounts automatically (for reference):

```yaml
volumeMounts:
  - name: keytab
    mountPath: /var/opt/mssql/secrets
    readOnly: true
  - name: krb5
    mountPath: /etc/krb5.conf
    subPath: krb5.conf
    readOnly: true

volumes:
  - name: keytab
    secret:
      secretName: mssql-keytab
  - name: krb5
    secret:
      secretName: mssql-krb5
```

### Test Kerberos

**Step 1: Get a Kerberos ticket**

```bash
kubectl exec -it sql-prod-0 -n mssql -- kinit sql-prod-svc@CONTOSO.COM
```

When prompted, enter the service account password.

**Expected output:**
```
Password for sql-prod-svc@CONTOSO.COM: 
```

**Step 2: Verify the ticket was obtained**

```bash
kubectl exec -it sql-prod-0 -n mssql -- klist
```

**Expected output:**
```
Ticket cache: FILE:/tmp/krb5cc_0
Default principal: sql-prod-svc@CONTOSO.COM

Valid starting       Expires              Service principal
01/15/2024 10:00:00  01/16/2024 10:00:00  krbtgt/CONTOSO.COM@CONTOSO.COM
```

**Step 3: Connect with AD authentication**

```bash
kubectl exec -it sql-prod-0 -n mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -E
```

**Expected output:**
```
1> 
```

You should get a SQL prompt. Type `SELECT SUSER_NAME()` and press Enter, then `GO` to verify you're connected as the AD user.

## Windows Authentication

After configuring AD integration, you can create logins for AD users and groups.

### Create AD Login

First, set the SA password:

```bash
export SA_PWD=$(kubectl get secret sql-prod-sa -n mssql -o jsonpath='{.data.password}' | base64 -d)
```

**Create a login for an AD user:**

```bash
kubectl exec -it sql-prod-0 -n mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P "$SA_PWD" -C \
  -Q "CREATE LOGIN [CONTOSO\john.doe] FROM WINDOWS;"
```

**Expected output:**
```
Command(s) completed successfully.
```

**Create a login for an AD group:**

```bash
kubectl exec -it sql-prod-0 -n mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P "$SA_PWD" -C \
  -Q "CREATE LOGIN [CONTOSO\SQLAdmins] FROM WINDOWS;"
```

**Expected output:**
```
Command(s) completed successfully.
```

**Grant sysadmin to the AD group:**

```bash
kubectl exec -it sql-prod-0 -n mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P "$SA_PWD" -C \
  -Q "ALTER SERVER ROLE sysadmin ADD MEMBER [CONTOSO\SQLAdmins];"
```

**Expected output:**
```
Command(s) completed successfully.
```

### Grant Database Access

**Create a database user and grant permissions:**

```bash
kubectl exec -it sql-prod-0 -n mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P "$SA_PWD" -C \
  -Q "USE AppDB; CREATE USER [CONTOSO\john.doe] FOR LOGIN [CONTOSO\john.doe]; ALTER ROLE db_datareader ADD MEMBER [CONTOSO\john.doe]; ALTER ROLE db_datawriter ADD MEMBER [CONTOSO\john.doe];"
```

**Expected output:**
```
Command(s) completed successfully.
```

**Verify the user was created:**

```bash
kubectl exec -it sql-prod-0 -n mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P "$SA_PWD" -C \
  -Q "USE AppDB; SELECT name, type_desc FROM sys.database_principals WHERE name LIKE '%john.doe%'"
```

**Expected output:**
```
name                 type_desc
-------------------- --------------------
CONTOSO\john.doe     WINDOWS_USER
```

### Connection String

Use this connection string format for Windows Authentication from your applications:

```
Server=sql-prod-primary.mssql.svc.cluster.local,1433;
Database=AppDB;
Integrated Security=SSPI;
TrustServerCertificate=True;
```

## Troubleshooting AD

### Common Issues

| Issue | Cause | Solution |
|-------|-------|----------|
| KDC unreachable | Network/DNS | Check DNS, firewall |
| Clock skew | Time sync | Configure NTP |
| SPN not found | Missing SPN | Register with setspn |
| Keytab invalid | Wrong password | Regenerate keytab |

### Diagnostic Commands

**Check DNS resolution:**

```bash
kubectl exec -it sql-prod-0 -n mssql -- nslookup dc01.contoso.com
```

**Expected output:**
```
Server:    10.96.0.10
Address:   10.96.0.10#53

Name:	dc01.contoso.com
Address: 10.0.0.4
```

If DNS fails, check your CoreDNS configuration or the `dnsServerIP` in your SQLServer CR.

**Check Kerberos connectivity:**

```bash
kubectl exec -it sql-prod-0 -n mssql -- kinit -V sql-prod-svc@CONTOSO.COM
```

**Expected output (success):**
```
Using default cache: /tmp/krb5cc_0
Using principal: sql-prod-svc@CONTOSO.COM
Password for sql-prod-svc@CONTOSO.COM: 
Authenticated to Kerberos v5
```

**Check time sync (clock skew):**

```bash
kubectl exec -it sql-prod-0 -n mssql -- date
```

Compare with the time on your AD Domain Controller. They should be within 5 minutes of each other.

**Check keytab contents:**

```bash
kubectl exec -it sql-prod-0 -n mssql -- klist -k /var/opt/mssql/secrets/mssql.keytab
```

**Expected output:**
```
Keytab name: FILE:/var/opt/mssql/secrets/mssql.keytab
KVNO Principal
---- --------------------------------------------------------------------------
   1 MSSQLSvc/sql-prod-0.mssql.svc.cluster.local:1433@CONTOSO.COM
```

**Check SQL Server AD status:**

```bash
export SA_PWD=$(kubectl get secret sql-prod-sa -n mssql -o jsonpath='{.data.password}' | base64 -d)

kubectl exec -it sql-prod-0 -n mssql -- \
  /opt/mssql-tools18/bin/sqlcmd -S localhost -U sa -P "$SA_PWD" -C \
  -Q "SELECT * FROM sys.dm_os_host_info"
```

**Expected output:** Shows host information including OS details.

### Logs

**Check SQL Server error log for Kerberos errors:**

```bash
kubectl exec -it sql-prod-0 -n mssql -- \
  cat /var/opt/mssql/log/errorlog | grep -i kerberos
```

**Expected output (success):**
```
Kerberos authentication is enabled
```

If you see errors, they'll provide details about what's failing.

**Check pod events for secret mounting issues:**

```bash
kubectl describe pod sql-prod-0 -n mssql
```

Look for events related to mounting the keytab or krb5.conf secrets.

### SPN Verification

On a Windows machine with domain access:

```powershell
setspn -L CONTOSO\sql-prod-svc
```

**Expected output:**
```
Registered ServicePrincipalNames for CN=sql-prod-svc,OU=ServiceAccounts,DC=contoso,DC=com:
        MSSQLSvc/sql-prod-0.mssql.svc.cluster.local:1433
        MSSQLSvc/sql-prod-primary.mssql.svc.cluster.local:1433
```

If SPNs are missing, register them using the commands in Step 2.

### Test AD Connectivity

**Test LDAP connectivity:**

```bash
kubectl exec -it sql-prod-0 -n mssql -- \
  ldapsearch -x -H ldap://dc01.contoso.com -b "dc=contoso,dc=com" "(samaccountname=sql-prod-svc)"
```

**Expected output:**
```
# sql-prod-svc, ServiceAccounts, contoso.com
dn: CN=sql-prod-svc,OU=ServiceAccounts,DC=contoso,DC=com
objectClass: user
...
```

If this fails, check network connectivity and firewall rules for port 389.

## Best Practices

| Practice | Description |
|----------|-------------|
| Use managed identities | When running in Azure |
| Separate service accounts | One per SQL instance |
| Rotate keytabs | Regular rotation schedule |
| Monitor auth failures | Alert on login failures |
| Use groups | Avoid individual user logins |

## Next Steps

- [Upgrades](upgrades.md) - Version upgrades
- [Backup & Restore](backup-restore.md) - Data protection
- [Troubleshooting](../user-guide/troubleshooting.md) - Common issues
