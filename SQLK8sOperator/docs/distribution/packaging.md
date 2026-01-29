# Packaging

[← Back to Distribution](../README.md) | [Documentation Home](../README.md)

Guide to packaging the SQL Server Kubernetes Operator for distribution.

## Table of Contents

- [Overview](#overview)
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
| v1.0.x | 2019, 2022 | 1.26+ | 1.21+ |
| v1.1.x | 2019, 2022 | 1.27+ | 1.21+ |

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
