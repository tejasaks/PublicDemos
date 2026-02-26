# Local Development

[← Back to Development](../README.md) | [Documentation Home](../README.md)

Guide to setting up a local development environment for the SQL Server Kubernetes Operator.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Image Source Modes](#image-source-modes)
- [Environment Setup](#environment-setup)
- [Running Locally](#running-locally)
- [Development Workflow](#development-workflow)
- [Quick Patching (Incremental Updates)](#quick-patching-incremental-updates)
- [IDE Configuration](#ide-configuration)
- [Debugging](#debugging)

## Prerequisites

### Required Tools

| Tool | Version | Installation |
|------|---------|--------------|
| Go | 1.24+ | [go.dev/dl](https://go.dev/dl/) |
| Docker | 20.10+ | [docker.com](https://docker.com) |
| kubectl | 1.28+ | [kubernetes.io](https://kubernetes.io/docs/tasks/tools/) |
| Kind or Minikube | Latest | See below |
| Make | 3.81+ | System package manager |

---

## Image Source Modes

The development environment supports two image source modes:

| Mode | Flag | Use Case |
|------|------|----------|
| **Local** (default) | `--local` | Testing local code changes |
| **Remote** | `--remote` | Testing published images from ghcr.io |

### Local Mode (Default)

Uses locally-built images from minikube's Docker cache. **Use this when developing.**

```bash
# Build local images first
eval $(minikube docker-env)
make docker-build IMG=mssql-operator:dev
make docker-build-sidecar IMG_SIDECAR=mssql-ag-helper:dev

# Install with local images (default)
scripts/dev-setup.sh install

# Or explicitly specify local mode
scripts/dev-setup.sh --local install
```

**Images used in Local mode:**
```
┌─────────────────────────────────────────────────────────────────────────┐
│  IMAGE SOURCE: LOCAL (minikube docker cache)                            │
├─────────────────────────────────────────────────────────────────────────┤
│  Operator:     mssql-operator:dev          (built locally)              │
│  AG Helper:    mssql-ag-helper:dev         (built locally)              │
│  SQL Server:   mcr.microsoft.com/mssql/server:2022-latest (pulled)      │
│  Exporter:     burningalchemist/sql_exporter:latest       (pulled)      │
│                                                                         │
│  imagePullPolicy: Never                                                 │
└─────────────────────────────────────────────────────────────────────────┘
```

### Remote Mode

Uses pre-built images from GitHub Container Registry. **Use this to test published releases.**

```bash
# Install with remote images
scripts/dev-setup.sh --remote install

# Or set via environment variable
IMAGE_SOURCE=remote scripts/dev-setup.sh install
```

**Images used in Remote mode:**
```
┌─────────────────────────────────────────────────────────────────────────┐
│  IMAGE SOURCE: REMOTE (ghcr.io)                                         │
├─────────────────────────────────────────────────────────────────────────┤
│  Operator:     ghcr.io/tejasaks/mssql-operator:v1.0.0     (pulled)      │
│  AG Helper:    ghcr.io/tejasaks/mssql-ag-helper:v1.0.0    (pulled)      │
│  SQL Server:   mcr.microsoft.com/mssql/server:2022-latest (pulled)      │
│  Exporter:     burningalchemist/sql_exporter:latest       (pulled)      │
│                                                                         │
│  imagePullPolicy: IfNotPresent                                          │
└─────────────────────────────────────────────────────────────────────────┘
```

### How It Works

| Mode | Operator Deployment | OperatorConfiguration |
|------|---------------------|----------------------|
| Local | `deploy/deployment.yaml` | `samples/operator-configuration-local-dev.yaml` |
| Remote | Inline YAML (ghcr.io images) | Default (ghcr.io AG Helper) |

The `dev-setup.sh` script will display which images are being used when you run `install`:

```bash
scripts/dev-setup.sh install
# Output:
# ╔════════════════════════════════════════════════════════════════╗
# ║  IMAGE SOURCE: LOCAL (minikube docker cache)                   ║
# ╠════════════════════════════════════════════════════════════════╣
# [IMAGE] Operator:   mssql-operator:dev
# [IMAGE] AG Helper:  mssql-ag-helper:dev
# [IMAGE] Pull Policy: Never (uses local images)
# ╚════════════════════════════════════════════════════════════════╝
```

---

### Install Kind (recommended)

```bash
# macOS
brew install kind

# Linux
curl -Lo ./kind https://kind.sigs.k8s.io/dl/v0.20.0/kind-linux-amd64
chmod +x ./kind
sudo mv ./kind /usr/local/bin/kind

# Windows
choco install kind
```

### Install Minikube (alternative)

```bash
# macOS
brew install minikube

# Linux
curl -LO https://storage.googleapis.com/minikube/releases/latest/minikube-linux-amd64
sudo install minikube-linux-amd64 /usr/local/bin/minikube

# Windows
choco install minikube
```

## Environment Setup

### Clone Repository

```bash
git clone https://github.com/yourorg/sql-server-k8s-operator.git
cd sql-server-k8s-operator
```

### Install Go Dependencies

```bash
go mod download
go mod verify
```

### Install Development Tools

```bash
make install-tools

# This installs:
# - controller-gen (CRD generation)
# - kustomize (manifest building)
# - ginkgo (testing framework)
# - golangci-lint (linting)
```

### Create Local Cluster

```bash
# Kind (recommended)
make kind-create

# Or manually
kind create cluster --name mssql-operator --config hack/kind-config.yaml
```

Kind configuration (`hack/kind-config.yaml`):

```yaml
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
  - role: control-plane
  - role: worker
  - role: worker
  - role: worker
```

> **💡 Quick Setup with dev-setup.sh (Linux + minikube)**
>
> If you're on Linux and prefer minikube, the `scripts/dev-setup.sh` script can automate the entire setup process including prerequisites checking, cluster creation, building, and deployment:
>
> ```bash
> # Full setup with one command
> ./scripts/dev-setup.sh all
> ```
>
> See [Building > Alternative: dev-setup.sh Script](building.md#alternative-dev-setupsh-script) for details.

### Load Development Images

```bash
# Build and load operator image
make docker-build IMG=mssql-operator:dev
kind load docker-image mssql-operator:dev --name mssql-operator

# Build and load AG Helper image
make docker-build-ag-helper IMG=ag-helper:dev
kind load docker-image ag-helper:dev --name mssql-operator
```

## Running Locally

### Option 1: Run Outside Cluster (Development)

Run the operator on your local machine, connecting to the cluster:

```bash
# Install CRDs first
make install

# Run operator locally
make run

# Or with specific options
go run ./cmd/main.go \
  --metrics-bind-address=:8080 \
  --health-probe-bind-address=:8081 \
  --leader-elect=false
```

### Option 2: Deploy to Cluster

Build and deploy to your Kind cluster:

```bash
# Build, load, and deploy
make deploy IMG=mssql-operator:dev

# Verify
kubectl get pods -n mssql-system
```

### Option 3: Tilt (Hot Reload)

Use Tilt for automatic rebuilds on file changes:

```bash
# Install Tilt
curl -fsSL https://raw.githubusercontent.com/tilt-dev/tilt/master/scripts/install.sh | bash

# Start Tilt
tilt up
```

Tiltfile:

```python
# Tiltfile
load('ext://restart_process', 'docker_build_with_restart')

docker_build_with_restart(
    'mssql-operator',
    '.',
    entrypoint=['/manager'],
    dockerfile='Dockerfile',
    only=['cmd/', 'api/', 'internal/', 'go.mod', 'go.sum'],
    live_update=[
        sync('./cmd', '/workspace/cmd'),
        sync('./api', '/workspace/api'),
        sync('./internal', '/workspace/internal'),
    ],
)

k8s_yaml(kustomize('config/default'))
k8s_resource('controller-manager', port_forwards=8080)
```

## Development Workflow

### Typical Workflow

```bash
# 1. Make code changes
vim internal/controller/sqlserver_controller.go

# 2. Run tests
make test

# 3. Build and deploy
make docker-build docker-push deploy IMG=mssql-operator:dev

# 4. Test with sample resources
kubectl apply -f samples/sqlserver-2025-standalone.yaml

# 5. Check logs
kubectl logs -f deployment/mssql-operator-controller-manager -n mssql-system
```

### Regenerate Generated Code

After modifying API types:

```bash
# Generate DeepCopy methods
make generate

# Generate CRD manifests
make manifests

# Generate both
make generate manifests
```

### Run Linter

```bash
make lint

# Or directly
golangci-lint run ./...
```

### Format Code

```bash
make fmt

# Or directly
go fmt ./...
```

## Quick Patching (Incremental Updates)

During development, you'll frequently need to patch the operator or AG Helper with code changes without reconfiguring everything from scratch. This section covers the fast iteration workflow.

### Patch Operator (Minikube)

The fastest way to test operator changes in minikube:

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

### Patch Operator with CRD Changes

When you've modified the API types (in `pkg/apis/`):

```bash
eval $(minikube docker-env)

# Regenerate CRDs and build
make manifests
make docker-build IMG=mssql-operator:latest

# Apply CRDs and restart operator
kubectl apply -f deploy/crds/ --force
kubectl rollout restart deployment/mssql-operator -n mssql-system
```

### Patch AG Helper

```bash
eval $(minikube docker-env)
make docker-build-ag-helper IMG=ag-helper:latest

# Delete AG Helper pods to pick up new image
kubectl delete pods -n mssql -l app.kubernetes.io/component=ag-helper
```

### One-Liner Aliases

Add these to your `~/.bashrc` or `~/.zshrc` for quick iteration:

```bash
# Quick patch operator
alias patch-op='eval $(minikube docker-env) && make docker-build IMG=mssql-operator:latest && kubectl rollout restart deployment/mssql-operator -n mssql-system'

# Quick patch with CRDs
alias patch-op-crd='eval $(minikube docker-env) && make manifests && make docker-build IMG=mssql-operator:latest && kubectl apply -f deploy/crds/ --force && kubectl rollout restart deployment/mssql-operator -n mssql-system'

# Quick patch AG Helper
alias patch-ag='eval $(minikube docker-env) && make docker-build-ag-helper IMG=ag-helper:latest && kubectl delete pods -n mssql -l app.kubernetes.io/component=ag-helper'
```

### Patch with Kind

For Kind clusters, you need to load the image:

```bash
# Build and load operator
make docker-build IMG=mssql-operator:latest
kind load docker-image mssql-operator:latest --name mssql-operator

# Restart operator
kubectl rollout restart deployment/mssql-operator -n mssql-system
```

### Verify Patch Success

After patching, verify the operator is running correctly:

```bash
# Check pod status
kubectl get pods -n mssql-system

# Check logs for errors
kubectl logs deployment/mssql-operator -n mssql-system --tail=30

# Verify resources are still healthy
kubectl get sqlserver -n mssql
kubectl get sqlserverag -n mssql
```

> **📚 Full Documentation:** For production patching procedures and detailed explanations, see [Operations > Upgrades > Patching the Operators](../operations/upgrades.md#patching-the-operators).

## IDE Configuration

### VS Code

Recommended extensions:
- Go (golang.go)
- Kubernetes (ms-kubernetes-tools.vscode-kubernetes-tools)
- YAML (redhat.vscode-yaml)

`.vscode/settings.json`:

```json
{
  "go.useLanguageServer": true,
  "go.lintTool": "golangci-lint",
  "go.lintFlags": ["--fast"],
  "go.formatTool": "goimports",
  "go.testFlags": ["-v"],
  "editor.formatOnSave": true,
  "[go]": {
    "editor.codeActionsOnSave": {
      "source.organizeImports": true
    }
  },
  "yaml.schemas": {
    "kubernetes": ["config/**/*.yaml", "samples/**/*.yaml"]
  }
}
```

`.vscode/launch.json`:

```json
{
  "version": "0.2.0",
  "configurations": [
    {
      "name": "Launch Operator",
      "type": "go",
      "request": "launch",
      "mode": "auto",
      "program": "${workspaceFolder}/cmd/main.go",
      "args": [
        "--metrics-bind-address=:8080",
        "--health-probe-bind-address=:8081",
        "--leader-elect=false"
      ],
      "env": {
        "KUBECONFIG": "${env:HOME}/.kube/config"
      }
    },
    {
      "name": "Debug Tests",
      "type": "go",
      "request": "launch",
      "mode": "test",
      "program": "${workspaceFolder}/internal/controller",
      "args": ["-test.v"]
    }
  ]
}
```

### GoLand / IntelliJ IDEA

1. Open project as Go module
2. Configure GOPATH to project
3. Enable Go Modules integration
4. Set Run Configuration:
   - Package path: `./cmd/main.go`
   - Program arguments: `--leader-elect=false`

## Debugging

### Debug Operator

```bash
# Run with delve
dlv debug ./cmd/main.go -- --leader-elect=false

# Or attach to running process
dlv attach <pid>
```

### Debug in Cluster

Deploy debug build:

```dockerfile
# Dockerfile.debug
FROM golang:1.21 AS builder
RUN go install github.com/go-delve/delve/cmd/dlv@latest
WORKDIR /workspace
COPY . .
RUN CGO_ENABLED=0 go build -gcflags="all=-N -l" -o manager cmd/main.go

FROM gcr.io/distroless/static:nonroot
COPY --from=builder /go/bin/dlv /dlv
COPY --from=builder /workspace/manager /manager
ENTRYPOINT ["/dlv", "--listen=:40000", "--headless=true", "--api-version=2", "exec", "/manager"]
```

Port-forward to debugger:

```bash
kubectl port-forward deployment/mssql-operator-controller-manager 40000:40000 -n mssql-system
```

Connect VS Code debugger to `localhost:40000`.

### View Controller Logs

```bash
# Follow operator logs
kubectl logs -f deployment/mssql-operator-controller-manager -n mssql-system

# With increased verbosity
kubectl logs -f deployment/mssql-operator-controller-manager -n mssql-system | grep -i sqlserver
```

### Enable Debug Logging

```go
// In main.go, set log level
import (
    "sigs.k8s.io/controller-runtime/pkg/log/zap"
)

opts := zap.Options{
    Development: true,  // Enables debug logging
}
ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))
```

### Test Specific Controller

```bash
# Run only SQLServer controller tests
go test -v ./internal/controller -run TestSQLServerReconciler

# With coverage
go test -v ./internal/controller -run TestSQLServerReconciler -coverprofile=coverage.out
go tool cover -html=coverage.out
```

## Cleanup

```bash
# Delete test resources
kubectl delete sqlserver --all -n mssql

# Uninstall operator
make undeploy

# Delete Kind cluster
kind delete cluster --name mssql-operator

# Clean build artifacts
make clean
```

## Next Steps

- [Building](building.md) - Build artifacts
- [Testing](testing.md) - Test framework
- [Contributing](contributing.md) - Contribution guide
