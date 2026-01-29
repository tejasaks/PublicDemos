# Local Development

[← Back to Development](../README.md) | [Documentation Home](../README.md)

Guide to setting up a local development environment for the SQL Server Kubernetes Operator.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Environment Setup](#environment-setup)
- [Running Locally](#running-locally)
- [Development Workflow](#development-workflow)
- [IDE Configuration](#ide-configuration)
- [Debugging](#debugging)

## Prerequisites

### Required Tools

| Tool | Version | Installation |
|------|---------|--------------|
| Go | 1.21+ | [go.dev/dl](https://go.dev/dl/) |
| Docker | 20.10+ | [docker.com](https://docker.com) |
| kubectl | 1.28+ | [kubernetes.io](https://kubernetes.io/docs/tasks/tools/) |
| Kind or Minikube | Latest | See below |
| Make | 3.81+ | System package manager |

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
kubectl get pods -n mssql-operator-system
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
kubectl apply -f samples/sqlserver-basic.yaml

# 5. Check logs
kubectl logs -f deployment/mssql-operator-controller-manager -n mssql-operator-system
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
kubectl port-forward deployment/mssql-operator-controller-manager 40000:40000 -n mssql-operator-system
```

Connect VS Code debugger to `localhost:40000`.

### View Controller Logs

```bash
# Follow operator logs
kubectl logs -f deployment/mssql-operator-controller-manager -n mssql-operator-system

# With increased verbosity
kubectl logs -f deployment/mssql-operator-controller-manager -n mssql-operator-system | grep -i sqlserver
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
