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

```bash
# Check Go version
go version  # Requires 1.21+

# Check Docker
docker version

# Install build tools
make install-tools
```

## Building the Operator

### Build Binary

```bash
# Build for current platform
make build

# Output: bin/manager

# Build with specific flags
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
  -ldflags="-w -s -X main.version=v1.0.0" \
  -o bin/manager cmd/main.go
```

### Build Docker Image

```bash
# Build with default tag
make docker-build IMG=mssql-operator:latest

# Build with version tag
make docker-build IMG=myregistry/mssql-operator:v1.0.0

# Build and push
make docker-build docker-push IMG=myregistry/mssql-operator:v1.0.0
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
