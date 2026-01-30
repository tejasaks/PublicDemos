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
