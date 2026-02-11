# Input Validation & Security

[← Back to User Guide](../README.md) | [Documentation Home](../README.md)

This document describes the input validation rules and security measures enforced by the operator.

## Table of Contents

- [Validation Layers](#validation-layers)
- [Password Requirements](#password-requirements)
- [Resource Name Validation](#resource-name-validation)
- [Storage Class Validation](#storage-class-validation)
- [Secret Validation](#secret-validation)
- [AG Helper Credential Validation](#ag-helper-credential-validation)
- [Memory Validation](#memory-validation)
- [Port Validation](#port-validation)
- [Security Validations](#security-validations)
- [Webhook Certificate Management](#webhook-certificate-management)
- [Validation Configuration](#validation-configuration)

## Validation Layers

The operator validates input at multiple layers:

| Layer | When | Purpose | Behavior |
|-------|------|---------|----------|
| **CRD Schema** | `kubectl apply` | Type checking, enums, required fields | Reject invalid |
| **Admission Webhook** | Before resource creation | Cluster capability checks, security | Block or Warn |
| **Controller** | During reconciliation | Runtime validation, state-dependent checks | Update status |
| **AG Helper** | SQL execution | SQL injection prevention | Sanitize input |

## Password Requirements

The SA (system administrator) password must meet SQL Server's complexity requirements.

### Requirements

| Requirement | Description |
|-------------|-------------|
| **Minimum Length** | At least 8 characters |
| **Complexity** | Must contain characters from at least 3 of these 4 categories: |
| | - Uppercase letters (A-Z) |
| | - Lowercase letters (a-z) |
| | - Digits (0-9) |
| | - Special characters (!@#$%^&*()-_=+[]{}|;:,.<>?) |

### Examples

| Password | Valid | Reason |
|----------|-------|--------|
| `MyP@ssw0rd!` | ✅ | Uppercase, lowercase, digit, special |
| `Str0ng#Pass` | ✅ | Uppercase, lowercase, digit, special |
| `SQLServer2022!` | ✅ | Uppercase, lowercase, digit, special |
| `password` | ❌ | Too simple, only lowercase |
| `12345678` | ❌ | Only digits |
| `Pass123` | ❌ | Only 7 characters |
| `ALLUPPERCASE1!` | ❌ | Missing lowercase |

### Validation Behavior

- **Default:** Password complexity is **enforced** (blocks creation if invalid)
- **Configurable:** Can be set to `warn` mode via OperatorConfiguration

## Resource Name Validation

### Name Length Limits

| Resource | Max Length | Reason |
|----------|------------|--------|
| SQLServer name | 13 chars | SQL Server NetBIOS limit (15) - pod suffix (2) |
| SQLServerAG name | 13 chars | Kubernetes resource limit |
| Secret name | 253 chars | Kubernetes DNS subdomain |
| AG name (SQL) | 128 chars | SQL Server identifier limit |
| Database name | 128 chars | SQL Server identifier limit |

### Kubernetes Name Pattern

```
^[a-z0-9]([-a-z0-9]*[a-z0-9])?$
```

| Example | Valid | Reason |
|---------|-------|--------|
| `sql-prod-01` | ✅ | Lowercase, alphanumeric, hyphens |
| `my-sqlserver` | ✅ | Valid pattern |
| `SQL-Server` | ❌ | Uppercase not allowed |
| `-sql-prod` | ❌ | Cannot start with hyphen |
| `sql_prod` | ❌ | Underscores not allowed |

### SQL Identifier Pattern

SQL Server identifiers (AG name, database name) follow different rules:

```
^[a-zA-Z_@#][a-zA-Z0-9_@#$]{0,127}$
```

| Example | Valid | Reason |
|---------|-------|--------|
| `ProductionAG` | ✅ | Starts with letter |
| `My_Database` | ✅ | Underscores allowed |
| `@TempTable` | ✅ | Can start with @ |
| `#TempTable` | ✅ | Can start with # |
| `123Database` | ❌ | Cannot start with digit |
| `SELECT` | ⚠️ | Reserved keyword (warning) |

## Storage Class Validation

The operator validates that specified StorageClasses exist in the cluster before creating resources.

### Behavior

| Scenario | Default Behavior | Result |
|----------|------------------|--------|
| StorageClass exists | Allow | Resource created |
| StorageClass not found | **Block** | Error with helpful message |
| StorageClass field empty | Allow | Uses cluster default |
| Validation times out (>3s) | Allow with warning | Resource created |

### Error Message Example

```
Error: StorageClass 'managed-premium' not found in cluster. 
Available StorageClasses: [standard, local-path]. 
Update spec.instance.storage.data.storageClass or remove to use cluster default.
```

### How to Fix

1. List available StorageClasses:
   ```bash
   kubectl get storageclass
   ```

2. Update your manifest:
   ```yaml
   storage:
     data:
       size: 10Gi
       storageClass: standard  # Use an available class
   ```

3. Or remove to use default:
   ```yaml
   storage:
     data:
       size: 10Gi
       # storageClass omitted - uses cluster default
   ```

## Secret Validation

The operator checks if referenced Secrets exist before creating resources.

### Behavior

| Scenario | Default Behavior | Result |
|----------|------------------|--------|
| Secret exists | Allow | Resource created |
| Secret not found | **Warn** (allow) | Resource created with warning |
| Secret missing key | Error | Resource blocked |

### Warning Message Example

```
Warning: Secret 'mssql-sa-password' not found in namespace 'mssql'. 
Create it with: kubectl create secret generic mssql-sa-password --from-literal=password=<password> -n mssql
```

> **Note:** Secrets use "warn" mode by default because they might be created by external processes (e.g., external-secrets operator, GitOps pipelines).

## AG Helper Credential Validation

The AG Helper sidecar uses dedicated SQL credentials to monitor Availability Group health. The operator validates these credential configurations.

### Credential Configuration Options

| Configuration | Validation | Behavior |
|---------------|------------|----------|
| `secretRef` only | ✅ Recommended | Check secrets exist, validate names |
| Plain text only | ⚠️ Allowed with warning | Validate SQL identifier, password complexity |
| Both specified | ❌ Error | Cannot mix secretRef and plain text |
| Neither specified | ⚠️ Warning | Falls back to SA account (not recommended) |

### SecretRef Validation

When using `secretRef`, the operator validates:

| Field | Validation |
|-------|------------|
| `usernameSecret.name` | Required, Kubernetes name format (max 253 chars) |
| `usernameSecret.key` | Required, non-empty |
| `passwordSecret.name` | Required, Kubernetes name format, secret exists check |
| `passwordSecret.key` | Required, non-empty |

### Plain Text Credential Validation

When using plain text credentials (not recommended for production):

| Field | Validation |
|-------|------------|
| `username` | SQL identifier format, SQL injection pattern check |
| `password` | Password complexity check (warning only) |

### Example: Valid SecretRef Configuration

```yaml
availabilityGroup:
  name: ProductionAG
  healthCheckCredentials:
    secretRef:
      usernameSecret:
        name: ag-helper-creds    # ✅ Valid K8s name
        key: username             # ✅ Non-empty
      passwordSecret:
        name: ag-helper-creds    # ✅ Can be same secret
        key: password             # ✅ Non-empty
```

### Example: Plain Text (Not Recommended)

```yaml
availabilityGroup:
  name: ProductionAG
  healthCheckCredentials:
    username: "ag_helper"         # ✅ Valid SQL identifier
    password: "Str0ng#Pass!"      # ⚠️ Warning: plain text
```

**Warning issued:**
```
healthCheckCredentials: using plain text credentials is NOT RECOMMENDED for production. 
Consider using secretRef instead to avoid exposing credentials in manifests.
```

### Per-Replica Credential Validation

The `replicaCredentials` map allows overriding credentials per replica. Each entry is validated with the same rules:

```yaml
replicaCredentials:
  "0":    # Primary replica
    secretRef:
      usernameSecret:
        name: ag-helper-primary
        key: username
      passwordSecret:
        name: ag-helper-primary
        key: password
  "1":    # Secondary replica
    secretRef:
      usernameSecret:
        name: ag-helper-secondary
        key: username
      passwordSecret:
        name: ag-helper-secondary
        key: password
```

### Common Validation Errors

| Error | Cause | Fix |
|-------|-------|-----|
| `specify either secretRef OR plain text, not both` | Mixed credential types | Use one method only |
| `usernameSecret.name is required` | Missing secret name | Add the secret name |
| `Secret 'x' not found in namespace 'y'` | Secret doesn't exist | Create the secret first |
| `username contains invalid characters` | Invalid SQL identifier | Use valid SQL identifier pattern |

## Memory Validation

SQL Server requires a minimum of 2GB RAM.

### Rules

| Memory Limit | Result |
|--------------|--------|
| `≥ 2Gi` | ✅ Allowed |
| `< 2Gi` | ❌ Blocked with error |
| Not specified | ⚠️ Warning issued |

### Recommendations

| Workload | Recommended Memory |
|----------|-------------------|
| Development | 4Gi |
| Production | 8Gi - 16Gi |
| Large databases | 32Gi+ |

Leave ~2GB for the OS and sidecars:
```yaml
resources:
  limits:
    memory: 16Gi
  # ...
config:
  memoryLimitMB: 14336  # 14GB for SQL Server
```

## Port Validation

### Rules

| Port Range | Result |
|------------|--------|
| `1-65535` | ✅ Valid |
| `< 1024` | ⚠️ Warning (privileged port) |
| `0` or `> 65535` | ❌ Invalid |

### Standard Ports

| Port | Purpose |
|------|---------|
| 1433 | SQL Server TDS |
| 5022 | AG mirroring endpoint |
| 8080 | AG Helper HTTP |
| 9399 | SQL Exporter metrics |

## Security Validations

### SQL Injection Prevention

User-provided values are validated for potential SQL injection patterns:

| Blocked Pattern | Reason |
|-----------------|--------|
| `'; DROP TABLE` | SQL statement injection |
| `--` | SQL comment injection |
| `/*` or `*/` | Block comment injection |
| `xp_` or `sp_` | System procedure prefix |
| `EXEC(` or `EXECUTE(` | Dynamic execution |
| `UNION`, `SELECT`, `DELETE`, `UPDATE`, `INSERT` | SQL keywords |
| `OR 1=1`, `' OR '` | Tautology injection |

### Path Traversal Prevention

File paths (backup paths, certificate paths) are validated:

| Blocked Pattern | Reason |
|-----------------|--------|
| `..` | Parent directory traversal |
| Null bytes (`\x00`) | Path injection |
| `;`, `|`, `&`, `` ` `` | Command injection |

### Container Image Validation

Container image references are validated:

| Validation | Example Invalid Value |
|------------|----------------------|
| No spaces | `nginx latest` ❌ |
| No shell metacharacters | `nginx; rm -rf /` ❌ |
| Valid format | `my.registry.io/image:tag` ✅ |

### AG Helper Protections

The AG Helper sidecar includes runtime protections:

1. **SQL Identifier Sanitization**: AG and database names are sanitized using QUOTENAME-style escaping
2. **Input Truncation**: Long inputs are truncated before logging
3. **Read-Only Operations**: Most operations are read-only queries

## Webhook Certificate Management

The operator uses [Kubernetes admission webhooks](https://kubernetes.io/docs/reference/access-authn-authz/extensible-admission-controllers/) to validate `SQLServer` and `SQLServerAG` resources before they are persisted. The Kubernetes API server communicates with the webhook server over TLS, so the operator must present a valid TLS certificate.

The operator supports three certificate modes, controlled by the `WEBHOOK_CERT_MODE` environment variable on the operator Deployment.

### Certificate Modes Overview

| Mode | Value | Description | Suitable For |
|------|-------|-------------|--------------|
| **Self-Signed** | `self-signed` | Auto-generates CA + server cert at startup *(default)* | Dev/test, quick start |
| **Manual** | `manual` | Loads certs from a pre-created Kubernetes TLS secret | Enterprise / air-gapped |
| **cert-manager** | `cert-manager` | Uses [cert-manager](https://cert-manager.io/) to issue and rotate certs | Production with cert-manager |

### Mode 1: Self-Signed (Default)

This is the zero-configuration default. At startup the operator:

1. Generates an ECDSA P-256 CA certificate and a server certificate signed by that CA
2. Writes `tls.crt` and `tls.key` to an in-memory `emptyDir` volume (`/tmp/webhook-certs`)
3. Patches the `caBundle` field on the `ValidatingWebhookConfiguration` so the API server trusts the self-signed CA

No user action is required. Certificates are regenerated every time the operator pod restarts.

```yaml
# deploy/deployment.yaml — self-signed mode (default, no changes needed)
env:
  - name: WEBHOOK_CERT_MODE
    value: "self-signed"
```

> **Note:** Self-signed certificates are suitable for development and testing. For production environments, use **manual** or **cert-manager** mode.

### Mode 2: Manual (Enterprise Certificates)

Use manual mode when your organization provisions TLS certificates through an internal PKI, HashiCorp Vault, or any process outside of cert-manager.

#### Step 1: Create the TLS Secret

Create a Kubernetes TLS secret in the operator namespace containing your certificate and private key. Optionally include the CA certificate — if present, the operator will automatically patch the `caBundle`.

```bash
kubectl create secret tls mssql-operator-webhook-tls \
  --cert=path/to/tls.crt \
  --key=path/to/tls.key \
  -n mssql-system
```

To include the CA certificate for automatic `caBundle` patching:

```bash
kubectl create secret generic mssql-operator-webhook-tls \
  --from-file=tls.crt=path/to/tls.crt \
  --from-file=tls.key=path/to/tls.key \
  --from-file=ca.crt=path/to/ca.crt \
  -n mssql-system
```

> **Certificate requirements:**
> - The server certificate SAN (Subject Alternative Name) must include the webhook service DNS name:
>   `mssql-operator-webhook.mssql-system.svc`
> - The secret keys must be named `tls.crt`, `tls.key`, and optionally `ca.crt`

#### Step 2: Configure the Operator

Set the following environment variables in the operator Deployment:

```yaml
env:
  - name: WEBHOOK_CERT_MODE
    value: "manual"
  - name: WEBHOOK_TLS_SECRET_NAME
    value: "mssql-operator-webhook-tls"   # default value
```

#### Step 3: Set caBundle (if no ca.crt in secret)

If you do not include `ca.crt` in the secret, you must manually set the `caBundle` field on the `ValidatingWebhookConfiguration`:

```bash
# Base64-encode your CA certificate
CA_BUNDLE=$(cat path/to/ca.crt | base64 | tr -d '\n')

# Patch the webhook configuration
kubectl patch validatingwebhookconfiguration mssql-operator-validating-webhook \
  --type='json' \
  -p="[
    {\"op\": \"replace\", \"path\": \"/webhooks/0/clientConfig/caBundle\", \"value\": \"${CA_BUNDLE}\"},
    {\"op\": \"replace\", \"path\": \"/webhooks/1/clientConfig/caBundle\", \"value\": \"${CA_BUNDLE}\"}
  ]"
```

#### Certificate Rotation (Manual Mode)

To rotate certificates in manual mode:

1. Update the TLS secret with new certificate and key:
   ```bash
   kubectl create secret tls mssql-operator-webhook-tls \
     --cert=path/to/new-tls.crt \
     --key=path/to/new-tls.key \
     -n mssql-system \
     --dry-run=client -o yaml | kubectl apply -f -
   ```

2. Restart the operator to pick up the new certificates:
   ```bash
   kubectl rollout restart deployment/mssql-operator -n mssql-system
   ```

3. If the CA changed, update the `caBundle` (or include `ca.crt` in the secret for automatic patching).

### Mode 3: cert-manager

Use cert-manager mode when you have [cert-manager](https://cert-manager.io/) installed in your cluster. cert-manager will issue certificates, populate the TLS secret, and inject the CA bundle automatically.

#### Step 1: Install cert-manager

```bash
kubectl apply -f https://github.com/cert-manager/cert-manager/releases/latest/download/cert-manager.yaml
```

#### Step 2: Create Issuer and Certificate Resources

```yaml
# Self-signed issuer for webhook certificates
apiVersion: cert-manager.io/v1
kind: Issuer
metadata:
  name: mssql-operator-selfsigned
  namespace: mssql-system
spec:
  selfSigned: {}
---
# Certificate for the webhook server
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: mssql-operator-webhook-cert
  namespace: mssql-system
spec:
  secretName: mssql-operator-webhook-tls
  issuerRef:
    name: mssql-operator-selfsigned
    kind: Issuer
  dnsNames:
    - mssql-operator-webhook.mssql-system.svc
    - mssql-operator-webhook.mssql-system.svc.cluster.local
  duration: 8760h    # 1 year
  renewBefore: 720h  # 30 days before expiry
```

#### Step 3: Annotate the ValidatingWebhookConfiguration

Add the cert-manager CA injection annotation so cert-manager automatically sets the `caBundle`:

```yaml
apiVersion: admissionregistration.k8s.io/v1
kind: ValidatingWebhookConfiguration
metadata:
  name: mssql-operator-validating-webhook
  annotations:
    cert-manager.io/inject-ca-from: mssql-system/mssql-operator-webhook-cert
```

#### Step 4: Configure the Operator

```yaml
env:
  - name: WEBHOOK_CERT_MODE
    value: "cert-manager"
  - name: WEBHOOK_TLS_SECRET_NAME
    value: "mssql-operator-webhook-tls"
```

In cert-manager mode the operator:
- Loads certificates from the TLS secret (created by cert-manager)
- Does **not** patch `caBundle` — cert-manager handles this via the annotation
- Certificate rotation is fully automatic (cert-manager renews before expiry)

### Environment Variables Reference

| Variable | Default | Description |
|----------|---------|-------------|
| `WEBHOOK_CERT_MODE` | `self-signed` | Certificate mode: `self-signed`, `manual`, or `cert-manager` |
| `WEBHOOK_TLS_SECRET_NAME` | `mssql-operator-webhook-tls` | Name of the TLS secret (used in `manual` and `cert-manager` modes) |
| `OPERATOR_NAMESPACE` | `mssql-system` | Namespace where the operator is deployed |

### Troubleshooting Webhook TLS

| Symptom | Likely Cause | Solution |
|---------|-------------|----------|
| `connection refused` on apply | Webhook server not running or port 9443 not exposed | Check operator pod logs; verify containerPort 9443 in Deployment |
| `x509: certificate signed by unknown authority` | `caBundle` not set or mismatched | Re-check caBundle on ValidatingWebhookConfiguration matches the CA |
| `tls: bad certificate` | Certificate SAN doesn't match service DNS | Regenerate cert with SAN: `mssql-operator-webhook.mssql-system.svc` |
| `secret "..." not found` | TLS secret missing in manual/cert-manager mode | Create the secret or check cert-manager Certificate status |
| Webhook works initially, then fails | Certificate expired | Use cert-manager for auto-renewal, or rotate manually |

## Validation Configuration

Validation behavior can be customized via OperatorConfiguration:

```yaml
apiVersion: mssql.microsoft.com/v1alpha1
kind: OperatorConfiguration
metadata:
  name: default
spec:
  validation:
    # Enable/disable cluster capability checks
    clusterCapabilityChecks: true
    
    # Timeout for cluster API calls (allows with warning on timeout)
    validationTimeout: "3s"
    
    # StorageClass validation: "block" or "warn"
    storageClassValidation: "block"
    
    # Secret validation: "block" or "warn"
    secretValidation: "warn"
    
    # Password complexity: "enforce" or "warn"
    passwordComplexity: "enforce"
    
    # Node validation: "block" or "warn"
    nodeValidation: "block"
```

### Timeout Behavior

| Scenario | Behavior |
|----------|----------|
| Validation completes within timeout | Normal block/warn behavior |
| Validation times out | Resource is **allowed** with warning |

This ensures intermittent API issues don't block legitimate deployments.

## Next Steps

- [Configuration Reference](configuration-reference.md) - All available options
- [Troubleshooting](troubleshooting.md) - Common issues
- [Architecture](../architecture/overview.md) - How validation is implemented
