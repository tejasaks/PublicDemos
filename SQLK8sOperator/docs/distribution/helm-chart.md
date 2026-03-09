# Helm Chart

[← Back to Distribution](../README.md) | [Documentation Home](../README.md)

Guide to the Helm chart for SQL Server Kubernetes Operator.

## Table of Contents

- [Overview](#overview)
- [Chart Structure](#chart-structure)
- [Installing with Helm](#installing-with-helm)
- [Configuration Values](#configuration-values)
- [Customization](#customization)
- [Upgrading](#upgrading)
- [Development](#development)

## Overview

The Helm chart provides:

- Easy installation and upgrades
- Configurable values
- Environment-specific overrides
- Dependency management

### Chart Repository

```bash
# Add repository
helm repo add mssql-operator https://yourorg.github.io/mssql-operator
helm repo update

# Search available versions
helm search repo mssql-operator
```

## Chart Structure

```
charts/mssql-operator/
├── Chart.yaml
├── values.yaml
├── templates/
│   ├── _helpers.tpl
│   ├── deployment.yaml
│   ├── service.yaml
│   ├── serviceaccount.yaml
│   ├── clusterrole.yaml
│   ├── clusterrolebinding.yaml
│   └── crds/
│       ├── mssql.microsoft.com_sqlservers.yaml
│       └── mssql.microsoft.com_sqlserverags.yaml
├── crds/
│   └── (CRD files installed outside Helm lifecycle)
└── README.md
```

### Chart.yaml

```yaml
apiVersion: v2
name: mssql-operator
description: SQL Server Kubernetes Operator
type: application
version: 1.0.0
appVersion: "v1.0.0"

keywords:
  - sql-server
  - database
  - operator
  - kubernetes

home: https://github.com/yourorg/mssql-operator
sources:
  - https://github.com/yourorg/mssql-operator

maintainers:
  - name: Your Team
    email: team@example.com

dependencies: []
```

## Installing with Helm

### Quick Install

```bash
# Install with defaults
helm install mssql-operator mssql-operator/mssql-operator \
  --namespace mssql-system \
  --create-namespace
```

### Install with Custom Values

```bash
# Install with custom values file
helm install mssql-operator mssql-operator/mssql-operator \
  --namespace mssql-system \
  --create-namespace \
  --values my-values.yaml
```

### Install from Local Chart

```bash
# Install from local directory
helm install mssql-operator ./charts/mssql-operator \
  --namespace mssql-system \
  --create-namespace
```

### Verify Installation

```bash
# Check release status
helm status mssql-operator -n mssql-system

# List releases
helm list -n mssql-system
```

## Configuration Values

### values.yaml

```yaml
# Operator image configuration
image:
  repository: ghcr.io/yourorg/mssql-operator
  tag: ""  # Defaults to appVersion
  pullPolicy: IfNotPresent

# Image pull secrets
imagePullSecrets: []

# Operator replicas (use 1 for leader election)
replicaCount: 1

# Service account
serviceAccount:
  create: true
  name: mssql-operator
  annotations: {}

# RBAC
rbac:
  create: true

# Resource limits
resources:
  limits:
    cpu: 500m
    memory: 256Mi
  requests:
    cpu: 100m
    memory: 128Mi

# Node selector
nodeSelector: {}

# Tolerations
tolerations: []

# Affinity
affinity: {}

# Pod security context
podSecurityContext:
  runAsNonRoot: true
  seccompProfile:
    type: RuntimeDefault

# Container security context
securityContext:
  allowPrivilegeEscalation: false
  capabilities:
    drop:
      - ALL
  readOnlyRootFilesystem: true

# Leader election
leaderElection:
  enabled: true

# Metrics
metrics:
  enabled: true
  port: 8080
  serviceMonitor:
    enabled: false
    interval: 30s

# Health probes
healthProbes:
  port: 8081

# Logging
logging:
  level: info
  format: json

# Watch configuration
watch:
  namespaces: []  # Empty = all namespaces
  labelSelector: ""

# AG Helper defaults
agHelper:
  image:
    repository: ghcr.io/yourorg/mssql-operator/ag-helper
    tag: ""  # Defaults to appVersion
  advanced:
    monitorInterval: 10s
    maxRetries: 3             # Retry attempts for transient SQL errors (1-30)
    retryInterval: 5s         # Delay between retries
    stalenessThreshold: 30s   # Data older than this → stale

# SQL Exporter defaults
sqlExporter:
  image:
    repository: ghcr.io/yourorg/mssql-operator/sql-exporter
    tag: ""  # Defaults to appVersion
  scrapeInterval: 15s

# CRD installation
crds:
  install: true
  keep: true  # Keep CRDs on uninstall
```

### Common Overrides

#### Production Values

```yaml
# production-values.yaml
replicaCount: 1

resources:
  limits:
    cpu: 1000m
    memory: 512Mi
  requests:
    cpu: 250m
    memory: 256Mi

metrics:
  serviceMonitor:
    enabled: true

logging:
  level: info

affinity:
  nodeAffinity:
    preferredDuringSchedulingIgnoredDuringExecution:
      - weight: 100
        preference:
          matchExpressions:
            - key: node-role.kubernetes.io/control-plane
              operator: DoesNotExist
```

#### Development Values

```yaml
# dev-values.yaml
resources:
  limits:
    cpu: 500m
    memory: 256Mi
  requests:
    cpu: 50m
    memory: 64Mi

logging:
  level: debug

watch:
  namespaces:
    - dev-mssql
```

#### Namespace-Scoped

```yaml
# namespace-scoped-values.yaml
rbac:
  create: true
  scope: namespace  # vs 'cluster'

watch:
  namespaces:
    - team-a-mssql
    - team-b-mssql
```

## Customization

### Set Values via CLI

```bash
helm install mssql-operator mssql-operator/mssql-operator \
  --set image.tag=v1.1.0 \
  --set resources.limits.memory=512Mi \
  --set metrics.serviceMonitor.enabled=true
```

### Multiple Value Files

```bash
helm install mssql-operator mssql-operator/mssql-operator \
  --values base-values.yaml \
  --values production-values.yaml \
  --values secrets-values.yaml
```

### Template Helpers

```yaml
# templates/_helpers.tpl
{{/*
Create chart name and version
*/}}
{{- define "mssql-operator.chart" -}}
{{- printf "%s-%s" .Chart.Name .Chart.Version | replace "+" "_" | trunc 63 | trimSuffix "-" }}
{{- end }}

{{/*
Common labels
*/}}
{{- define "mssql-operator.labels" -}}
helm.sh/chart: {{ include "mssql-operator.chart" . }}
app.kubernetes.io/name: {{ .Chart.Name }}
app.kubernetes.io/instance: {{ .Release.Name }}
app.kubernetes.io/version: {{ .Chart.AppVersion | quote }}
app.kubernetes.io/managed-by: {{ .Release.Service }}
{{- end }}

{{/*
Selector labels
*/}}
{{- define "mssql-operator.selectorLabels" -}}
app.kubernetes.io/name: {{ .Chart.Name }}
app.kubernetes.io/instance: {{ .Release.Name }}
{{- end }}
```

## Upgrading

### Upgrade to New Version

```bash
# Update repo
helm repo update

# Check diff before upgrade
helm diff upgrade mssql-operator mssql-operator/mssql-operator \
  --namespace mssql-system

# Upgrade
helm upgrade mssql-operator mssql-operator/mssql-operator \
  --namespace mssql-system \
  --values my-values.yaml
```

### Rollback

```bash
# List revision history
helm history mssql-operator -n mssql-system

# Rollback to previous
helm rollback mssql-operator 1 -n mssql-system
```

### CRD Upgrades

CRDs are not upgraded by Helm. Update manually:

```bash
# Apply updated CRDs
kubectl apply -f https://github.com/yourorg/mssql-operator/releases/download/v1.1.0/crds.yaml

# Then upgrade Helm release
helm upgrade mssql-operator mssql-operator/mssql-operator
```

## Development

### Package Chart

```bash
# Lint chart
helm lint charts/mssql-operator

# Package
helm package charts/mssql-operator

# Output: mssql-operator-1.0.0.tgz
```

### Test Installation

```bash
# Dry run
helm install mssql-operator ./charts/mssql-operator \
  --dry-run \
  --debug

# Template only
helm template mssql-operator ./charts/mssql-operator
```

### Update Chart Repository

```bash
# Generate index
helm repo index . --url https://yourorg.github.io/mssql-operator

# Push to GitHub Pages
# (typically automated via CI)
```

### Chart Testing

```yaml
# ct.yaml (chart-testing config)
target-branch: main
chart-dirs:
  - charts
validate-maintainers: false
```

```bash
# Run chart-testing
ct lint --config ct.yaml
ct install --config ct.yaml
```

## Uninstalling

```bash
# Uninstall release
helm uninstall mssql-operator -n mssql-system

# CRDs are kept by default, delete manually if needed
kubectl delete crd sqlservers.mssql.microsoft.com
kubectl delete crd sqlserverags.mssql.microsoft.com
```

## Next Steps

- [End-User Installation](end-user-installation.md) - Installation methods
- [Packaging](packaging.md) - Release packaging
- [Getting Started](../getting-started.md) - First deployment
