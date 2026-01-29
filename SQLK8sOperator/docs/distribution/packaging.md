# Packaging

[← Back to Distribution](../README.md) | [Documentation Home](../README.md)

Guide to packaging the SQL Server Kubernetes Operator for distribution.

## Table of Contents

- [Overview](#overview)
- [Quick Start: Publishing a Release](#quick-start-publishing-a-release)
- [Release Artifacts](#release-artifacts)
- [Versioning](#versioning)
- [Container Images](#container-images)
- [Kubernetes Manifests](#kubernetes-manifests)
- [Release Process](#release-process)

## Overview

The operator is distributed as:

| Artifact | Format | Repository |
|----------|--------|------------|
| Container images | OCI | Container registry (ghcr.io, Docker Hub) |
| Kubernetes manifests | YAML | GitHub releases |
| Helm chart | tgz | Helm repository |

## Quick Start: Publishing a Release

Follow these steps to publish a new operator release that end users can install.

### Prerequisites

Before starting, ensure you have:

```bash
# Verify Go is installed (1.21+)
go version

# Verify Docker is installed
docker version

# Verify you're logged into your container registry
docker login ghcr.io

# Verify GitHub CLI is installed (for creating releases)
gh --version

# Verify kustomize is installed
kustomize version
```

### Step 1: Set Version and Registry

```bash
# Set your release version
export VERSION=v1.0.0

# Set your container registry
export REGISTRY=ghcr.io/yourorg
```

### Step 2: Run Tests

```bash
make test
```

**Expected output:**
```
?       github.com/yourorg/mssql-operator/cmd    [no test files]
ok      github.com/yourorg/mssql-operator/api/v1alpha1  0.234s
ok      github.com/yourorg/mssql-operator/internal/controller  2.567s
PASS
```

### Step 3: Build and Push Container Images

```bash
# Build and push the main operator image
make docker-build docker-push IMG=$REGISTRY/mssql-operator:$VERSION
```

**Expected output:**
```
docker build -t ghcr.io/yourorg/mssql-operator:v1.0.0 .
[+] Building 45.2s (15/15) FINISHED
 => [internal] load build definition from Dockerfile
 ...
docker push ghcr.io/yourorg/mssql-operator:v1.0.0
The push refers to repository [ghcr.io/yourorg/mssql-operator]
abc123: Pushed
v1.0.0: digest: sha256:... size: 1234
```

**Also build the helper images:**

```bash
# AG Helper
make docker-build-ag-helper docker-push-ag-helper IMG=$REGISTRY/mssql-operator/ag-helper:$VERSION

# SQL Exporter
make docker-build-exporter docker-push-exporter IMG=$REGISTRY/mssql-operator/sql-exporter:$VERSION
```

### Step 4: Generate Installation Manifests

The Makefile automatically generates a combined `install.yaml` file that enables direct URL installation.

**Option A: Using the Makefile (Recommended)**

```bash
# Generate install.yaml with correct image reference
make generate-install-yaml VERSION=$VERSION IMG=$REGISTRY/mssql-operator:$VERSION
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
  Path: /path/to/SQLK8sOperator/install.yaml
==========================================
```

**On Windows PowerShell:**

```powershell
.\scripts\generate-install-yaml.ps1 -Version $VERSION -Image "$REGISTRY/mssql-operator:$VERSION"
```

**Option B: Using Kustomize (Alternative)**

```bash
# Update the image reference in kustomization
cd config/manager
kustomize edit set image controller=$REGISTRY/mssql-operator:$VERSION
cd ../..

# Generate the single-file installation manifest
kustomize build config/default > release/install.yaml

# Generate CRDs-only manifest
kustomize build config/crd > release/crds.yaml
```

**Verify the install.yaml was created:**

```bash
ls -lh install.yaml

# Expected output:
# -rw-r--r--  1 user  staff  85K Jan 15 10:00 install.yaml
```

**Verify the image reference is correct:**

```bash
grep "image:.*mssql-operator" install.yaml

# Expected output:
# image: ghcr.io/yourorg/mssql-operator:v1.0.0
```

> **Important:** The `install.yaml` at the repository root is what enables direct URL installation:
> ```bash
> kubectl apply -f https://raw.githubusercontent.com/tejasaks/PublicDemos/main/SQLK8sOperator/install.yaml
> ```
> **Always commit the updated `install.yaml` after regenerating it.**

### Step 5: Create Checksums

```bash
cd release
sha256sum *.yaml > checksums.sha256
cat checksums.sha256
cd ..
```

**Expected output:**
```
abc123def456...  crds.yaml
789xyz012abc...  install.yaml
```

### Step 6: Create Git Tag

```bash
git add .
git commit -m "release: $VERSION"
git tag -a $VERSION -m "Release $VERSION"
git push origin main --tags
```

**Expected output:**
```
Enumerating objects: 5, done.
...
 * [new tag]         v1.0.0 -> v1.0.0
```

### Step 7: Create GitHub Release

```bash
gh release create $VERSION \
  release/install.yaml \
  release/crds.yaml \
  release/checksums.sha256 \
  --title "SQL Server Operator $VERSION" \
  --generate-notes
```

**Expected output:**
```
https://github.com/yourorg/mssql-operator/releases/tag/v1.0.0
```

### Step 8: Verify the Release

Test that end users can now install:

```bash
# This should work now!
kubectl apply -f https://github.com/yourorg/mssql-operator/releases/download/$VERSION/install.yaml
```

**Expected output:**
```
namespace/mssql-operator-system created
customresourcedefinition.apiextensions.k8s.io/sqlservers.mssql.microsoft.com created
...
deployment.apps/mssql-operator-controller-manager created
```

## Release Artifacts

### Complete Release Package

```
release/
├── install.yaml              # Single-file installation
├── crds.yaml                 # CRD definitions only
├── operator.yaml             # Operator deployment only
├── samples/                  # Example manifests
│   ├── sqlserver-basic.yaml
│   ├── sqlserver-ha.yaml
│   └── sqlserverag.yaml
├── helm/
│   └── mssql-operator-v1.0.0.tgz
└── checksums.sha256
```

### Generate Installation Manifest

```bash
# Generate single install.yaml
make release-manifests VERSION=v1.0.0

# Or manually with kustomize
cd config/manager && kustomize edit set image controller=ghcr.io/yourorg/mssql-operator:v1.0.0
kustomize build config/default > install.yaml
```

### CRDs Only

```bash
# Extract CRDs only
kustomize build config/crd > crds.yaml
```

## Versioning

### Semantic Versioning

```
MAJOR.MINOR.PATCH

v1.0.0 - Initial stable release
v1.1.0 - New features, backward compatible
v1.1.1 - Bug fixes
v2.0.0 - Breaking changes
```

### Version Matrix

| Operator Version | SQL Server | Kubernetes | Go |
|-----------------|------------|------------|-----|
| v1.0.x | 2019, 2022, 2025 | 1.26+ | 1.21+ |
| v1.1.x | 2019, 2022, 2025 | 1.27+ | 1.21+ |

### Pre-release Versions

```
v1.1.0-alpha.1  # Early testing
v1.1.0-beta.1   # Feature complete
v1.1.0-rc.1     # Release candidate
v1.1.0          # Stable release
```

## Container Images

### Image Tags

| Tag | Description |
|-----|-------------|
| `latest` | Most recent release |
| `v1.0.0` | Specific version |
| `v1.0` | Latest patch of v1.0 |
| `v1` | Latest minor of v1 |
| `sha-abc123` | Commit SHA |
| `main` | Latest main branch (unstable) |

### Multi-Architecture

Build for multiple architectures:

```bash
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  --tag ghcr.io/yourorg/mssql-operator:v1.0.0 \
  --push \
  .
```

### All Component Images

```bash
# Operator
ghcr.io/yourorg/mssql-operator:v1.0.0

# AG Helper
ghcr.io/yourorg/mssql-operator/ag-helper:v1.0.0

# SQL Exporter
ghcr.io/yourorg/mssql-operator/sql-exporter:v1.0.0
```

### Signing Images

```bash
# Sign with cosign
cosign sign --key cosign.key ghcr.io/yourorg/mssql-operator:v1.0.0

# Verify signature
cosign verify --key cosign.pub ghcr.io/yourorg/mssql-operator:v1.0.0
```

## Kubernetes Manifests

### Directory Structure

```
config/
├── crd/
│   └── bases/
│       ├── mssql.microsoft.com_sqlservers.yaml
│       └── mssql.microsoft.com_sqlserverags.yaml
├── rbac/
│   ├── role.yaml
│   ├── role_binding.yaml
│   └── service_account.yaml
├── manager/
│   ├── manager.yaml
│   └── kustomization.yaml
├── default/
│   └── kustomization.yaml
└── samples/
    ├── sqlserver-basic.yaml
    └── sqlserver-ha.yaml
```

### Kustomization

```yaml
# config/default/kustomization.yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

namespace: mssql-operator-system

resources:
  - ../crd
  - ../rbac
  - ../manager

images:
  - name: controller
    newName: ghcr.io/yourorg/mssql-operator
    newTag: v1.0.0
```

### Generate Manifests

```bash
# Build final manifests
kustomize build config/default > install.yaml

# With specific image
cd config/manager && kustomize edit set image controller=ghcr.io/yourorg/mssql-operator:v1.0.0
cd ../.. && kustomize build config/default > install.yaml
```

## Release Process

### Automated Release (GitHub Actions)

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
    permissions:
      contents: write
      packages: write
    
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      
      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: '1.21'
      
      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v3
      
      - name: Login to GitHub Container Registry
        uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}
      
      - name: Get version
        id: version
        run: echo "VERSION=${GITHUB_REF#refs/tags/}" >> $GITHUB_OUTPUT
      
      - name: Run tests
        run: make test
      
      - name: Build and push images
        run: |
          make docker-buildx IMG=ghcr.io/${{ github.repository }}:${{ steps.version.outputs.VERSION }}
          make docker-buildx-ag-helper IMG=ghcr.io/${{ github.repository }}/ag-helper:${{ steps.version.outputs.VERSION }}
          make docker-buildx-exporter IMG=ghcr.io/${{ github.repository }}/sql-exporter:${{ steps.version.outputs.VERSION }}
      
      - name: Generate release manifests
        run: |
          make release-manifests VERSION=${{ steps.version.outputs.VERSION }}
      
      - name: Generate checksums
        run: |
          cd release
          sha256sum *.yaml > checksums.sha256
      
      - name: Create GitHub Release
        uses: softprops/action-gh-release@v1
        with:
          files: |
            release/install.yaml
            release/crds.yaml
            release/checksums.sha256
          generate_release_notes: true
          draft: false
          prerelease: ${{ contains(steps.version.outputs.VERSION, 'alpha') || contains(steps.version.outputs.VERSION, 'beta') || contains(steps.version.outputs.VERSION, 'rc') }}
```

### Manual Release

```bash
# 1. Update version references
VERSION=v1.0.0

# 2. Update changelog
vim CHANGELOG.md

# 3. Commit and tag
git add .
git commit -m "release: $VERSION"
git tag -a $VERSION -m "Release $VERSION"
git push origin main --tags

# 4. Build and push images
make docker-build docker-push IMG=ghcr.io/yourorg/mssql-operator:$VERSION

# 5. Generate manifests
make release-manifests VERSION=$VERSION

# 6. Create GitHub release
gh release create $VERSION release/*.yaml --generate-notes
```

### Release Checklist

- [ ] All tests pass
- [ ] CHANGELOG.md updated
- [ ] Version bumped in code
- [ ] Documentation updated
- [ ] Images built and pushed
- [ ] Manifests generated
- [ ] GitHub release created
- [ ] Helm chart updated
- [ ] Announce release

## Next Steps

- [Helm Chart](helm-chart.md) - Helm packaging
- [End-User Installation](end-user-installation.md) - Installation guide
