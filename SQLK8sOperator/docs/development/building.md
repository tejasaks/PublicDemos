# Building

[← Back to Development](../README.md) | [Documentation Home](../README.md)

Guide to building the SQL Server Kubernetes Operator and related components.

## Table of Contents

- [Build Overview](#build-overview)
- [Prerequisites](#prerequisites)
- [Building the Operator](#building-the-operator)
- [Building AG Helper](#building-ag-helper)
- [Building SQL Exporter](#building-sql-exporter)
- [Multi-Architecture Builds](#multi-architecture-builds)
- [CI/CD Integration](#cicd-integration)

## Build Overview

### Components

| Component | Language | Image |
|-----------|----------|-------|
| Operator | Go | mssql-operator |
| AG Helper | Go | ag-helper |
| SQL Exporter | Go | sql-exporter |

### Build Targets

```bash
# Show all make targets
make help

# Key targets:
# - make build         Build operator binary
# - make docker-build  Build Docker image
# - make manifests     Generate CRD manifests
# - make generate      Generate DeepCopy code
```

## Prerequisites

Before building, ensure you have the required tools installed.

**Step 1: Check Go version**

```bash
go version
```

**Expected output (requires 1.21+):**
```
go version go1.21.5 linux/amd64
```

If Go is not installed, download from https://go.dev/dl/

**Step 2: Check Docker**

```bash
docker version
```

**Expected output:**
```
Client:
 Version:           24.0.7
 ...
Server:
 Engine:
  Version:          24.0.7
```

**Step 3: Install additional build tools**

```bash
make install-tools
```

**Expected output:**
```
go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.14.0
go install sigs.k8s.io/kustomize/kustomize/v5@v5.3.0
```

**Step 4: Verify tools are available**

```bash
controller-gen --version
kustomize version
```

## Building the Operator

### Build Binary

**Build for your current platform:**

```bash
make build
```

**Expected output:**
```
go build -o bin/manager cmd/main.go
```

**Verify the binary was created:**

```bash
ls -lh bin/manager

# Expected output:
# -rwxr-xr-x  1 user  staff  45M Jan 15 10:00 bin/manager
```

**Build with specific flags (for cross-compilation):**

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
  -ldflags="-w -s -X main.version=v1.0.0" \
  -o bin/manager cmd/main.go
```

### Build Docker Image

**Build with default tag:**

```bash
make docker-build IMG=mssql-operator:latest
```

**Expected output:**
```
docker build -t mssql-operator:latest .
[+] Building 45.2s (15/15) FINISHED
 => [internal] load build definition from Dockerfile
 => [internal] load .dockerignore
 => [internal] load metadata for gcr.io/distroless/static:nonroot
 ...
 => exporting to image
 => => naming to docker.io/library/mssql-operator:latest
```

**Build with version tag for your registry:**

```bash
make docker-build IMG=ghcr.io/yourorg/mssql-operator:v1.0.0
```

**Build and push to registry:**

```bash
# First, log in to your registry
docker login ghcr.io

# Then build and push
make docker-build docker-push IMG=ghcr.io/yourorg/mssql-operator:v1.0.0
```

**Expected output:**
```
docker build -t ghcr.io/yourorg/mssql-operator:v1.0.0 .
...
docker push ghcr.io/yourorg/mssql-operator:v1.0.0
The push refers to repository [ghcr.io/yourorg/mssql-operator]
abc123: Pushed
v1.0.0: digest: sha256:... size: 1234
```

### Dockerfile

```dockerfile
# Build stage
FROM golang:1.21 AS builder
WORKDIR /workspace

# Cache dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY cmd/ cmd/
COPY api/ api/
COPY internal/ internal/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-w -s" \
    -o manager cmd/main.go

# Runtime stage
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/manager .
USER 65532:65532
ENTRYPOINT ["/manager"]
```

### Generate Manifests

```bash
# Generate CRDs
make manifests

# Output: config/crd/bases/*.yaml

# Generate with specific version
controller-gen crd:crdVersions=v1 \
  rbac:roleName=manager-role \
  webhook \
  paths="./..." \
  output:crd:artifacts:config=config/crd/bases
```

### Generate install.yaml (Combined Installation Manifest)

The `install.yaml` file at the repository root enables users to install the operator with a single command directly from a URL. This file must be regenerated whenever you change any deployment manifests.

**Generate install.yaml:**

```bash
make generate-install-yaml VERSION=v1.0.0 IMG=ghcr.io/yourorg/mssql-operator:v1.0.0
```

**Expected output:**
```
Generating install.yaml...
  Version: v1.0.0
  Image: ghcr.io/yourorg/mssql-operator:v1.0.0
  Output: /path/to/SQLK8sOperator/install.yaml

  Added: deploy/namespace.yaml
  Added: deploy/serviceaccount.yaml
  Added: deploy/rbac.yaml

Adding CRDs...
  Added: deploy/crds/mssql.microsoft.com_sqlserverags.yaml
  Added: deploy/crds/mssql.microsoft.com_sqlservers.yaml

Adding deployment with image substitution...
  Added: deploy/deployment.yaml (with image: ghcr.io/yourorg/mssql-operator:v1.0.0)

==========================================
install.yaml generated successfully!
  Lines: 2500
  Size: 85K
==========================================
```

**On Windows PowerShell:**

```powershell
.\scripts\generate-install-yaml.ps1 -Version "v1.0.0" -Image "ghcr.io/yourorg/mssql-operator:v1.0.0"
```

**Verify the install.yaml:**

```bash
# Check image reference
grep "image:.*mssql-operator" install.yaml

# Expected output:
# image: ghcr.io/yourorg/mssql-operator:v1.0.0

# Check file size
ls -lh install.yaml

# Expected output:
# -rw-r--r--  1 user  staff  85K Jan 15 10:00 install.yaml
```

> **Important:** The `install.yaml` is what enables direct URL installation:
> ```bash
> kubectl apply -f https://raw.githubusercontent.com/tejasaks/PublicDemos/main/SQLK8sOperator/install.yaml
> ```
> **Always commit the updated `install.yaml` after regenerating it.**

**When to regenerate install.yaml:**
- After modifying any file in `deploy/`
- After regenerating CRDs with `make manifests`
- Before creating a new release
- After changing the operator image version

### Generate Code

```bash
# Generate DeepCopy, DeepCopyInto, DeepCopyObject
make generate

# Verify generated code is up to date
make generate
git diff --exit-code
```

## Building AG Helper

### Build Binary

```bash
cd ag-helper

# Build for current platform
go build -o bin/ag-helper ./cmd/main.go

# Build for Linux
GOOS=linux GOARCH=amd64 go build -o bin/ag-helper ./cmd/main.go
```

### Build Docker Image

```bash
make docker-build-ag-helper IMG=ag-helper:latest

# Or manually
docker build -t ag-helper:latest -f ag-helper/Dockerfile ag-helper/
```

### AG Helper Dockerfile

```dockerfile
FROM golang:1.21 AS builder
WORKDIR /workspace

COPY go.mod go.sum ./
RUN go mod download

COPY cmd/ cmd/
COPY internal/ internal/

RUN CGO_ENABLED=0 GOOS=linux go build -o ag-helper cmd/main.go

FROM gcr.io/distroless/static:nonroot
COPY --from=builder /workspace/ag-helper /ag-helper
USER 65532:65532
ENTRYPOINT ["/ag-helper"]
```

## Building SQL Exporter

### Build Binary

```bash
cd sql-exporter

go build -o bin/sql-exporter ./cmd/main.go
```

### Build Docker Image

```bash
make docker-build-exporter IMG=sql-exporter:latest
```

## Multi-Architecture Builds

### Using Docker Buildx

```bash
# Create builder
docker buildx create --name multiarch --use

# Build multi-arch image
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  --tag myregistry/mssql-operator:v1.0.0 \
  --push \
  .
```

### Makefile Target

```makefile
.PHONY: docker-buildx
docker-buildx:
	docker buildx build \
		--platform $(PLATFORMS) \
		--tag $(IMG) \
		--push \
		.

PLATFORMS ?= linux/amd64,linux/arm64
```

### Build All Components

```bash
# Build all images
make docker-build-all

# Push all images
make docker-push-all

# Or individually
make docker-build IMG=mssql-operator:v1.0.0
make docker-build-ag-helper IMG=ag-helper:v1.0.0
make docker-build-exporter IMG=sql-exporter:v1.0.0
```

## CI/CD Integration

### GitHub Actions

```yaml
# .github/workflows/build.yaml
name: Build

on:
  push:
    branches: [main]
  pull_request:
    branches: [main]

jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.21'
      
      - name: Cache Go modules
        uses: actions/cache@v3
        with:
          path: ~/go/pkg/mod
          key: ${{ runner.os }}-go-${{ hashFiles('**/go.sum') }}
      
      - name: Build
        run: make build
      
      - name: Test
        run: make test
      
      - name: Lint
        uses: golangci/golangci-lint-action@v3
        with:
          version: latest

  docker:
    needs: build
    runs-on: ubuntu-latest
    if: github.event_name == 'push'
    steps:
      - uses: actions/checkout@v4
      
      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v3
      
      - name: Login to Registry
        uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}
      
      - name: Build and Push
        uses: docker/build-push-action@v5
        with:
          context: .
          platforms: linux/amd64,linux/arm64
          push: true
          tags: |
            ghcr.io/${{ github.repository }}:${{ github.sha }}
            ghcr.io/${{ github.repository }}:latest
          cache-from: type=gha
          cache-to: type=gha,mode=max
```

### Release Workflow

```yaml
# .github/workflows/release.yaml
name: Release

on:
  push:
    tags:
      - 'v*'

jobs:
  release:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      
      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.21'
      
      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v3
      
      - name: Login to Registry
        uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}
      
      - name: Get version
        id: version
        run: echo "VERSION=${GITHUB_REF#refs/tags/}" >> $GITHUB_OUTPUT
      
      - name: Build and Push Operator
        uses: docker/build-push-action@v5
        with:
          context: .
          platforms: linux/amd64,linux/arm64
          push: true
          tags: |
            ghcr.io/${{ github.repository }}:${{ steps.version.outputs.VERSION }}
            ghcr.io/${{ github.repository }}:latest
      
      - name: Generate Release Manifests
        run: |
          make manifests
          cd config/manager && kustomize edit set image controller=ghcr.io/${{ github.repository }}:${{ steps.version.outputs.VERSION }}
          kustomize build config/default > install.yaml
      
      - name: Create Release
        uses: softprops/action-gh-release@v1
        with:
          files: |
            install.yaml
          generate_release_notes: true
```

## Build Optimization

### Reduce Image Size

```dockerfile
# Use multi-stage build with scratch
FROM golang:1.21 AS builder
# ... build ...

FROM scratch
COPY --from=builder /workspace/manager /manager
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
USER 65532:65532
ENTRYPOINT ["/manager"]
```

### Build Cache

```bash
# Use Go build cache
GOCACHE=/tmp/go-build-cache go build ...

# In Docker, mount cache
docker build --build-arg GOCACHE=/go-cache ...
```

### Parallel Builds

```makefile
.PHONY: build-all
build-all:
	$(MAKE) -j4 build build-ag-helper build-exporter
```

## Next Steps

- [Testing](testing.md) - Test framework
- [Contributing](contributing.md) - Contribution guidelines
- [Packaging](../distribution/packaging.md) - Release packaging
