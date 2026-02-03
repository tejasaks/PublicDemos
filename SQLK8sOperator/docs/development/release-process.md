# Release Process and Versioning

[← Back to Development](local-development.md) | [Documentation Home](../README.md)

This document describes the release process, versioning strategy, and how to publish new versions of the SQL Server Kubernetes Operator.

## Table of Contents

- [Semantic Versioning](#semantic-versioning)
- [Version Locations](#version-locations)
- [Image Tagging Strategy](#image-tagging-strategy)
- [Release Workflow](#release-workflow)
- [Quick Reference](#quick-reference)
- [Hotfix Releases](#hotfix-releases)
- [Pre-release Versions](#pre-release-versions)

---

## Semantic Versioning

The operator follows [Semantic Versioning 2.0.0](https://semver.org/):

```
MAJOR.MINOR.PATCH (e.g., 1.2.3)
```

| Version Part | When to Increment | Example Change |
|--------------|-------------------|----------------|
| **MAJOR** | Breaking changes to CRDs or APIs | Removing a CRD field, changing field types |
| **MINOR** | New features, backward compatible | Adding new CRD fields, new controller features |
| **PATCH** | Bug fixes, no new features | Fixing a reconciliation bug, security patches |

### Breaking Changes (MAJOR version bump)

A change is **breaking** if it requires users to:
- Modify their existing CR manifests
- Migrate data before upgrading
- Update their automation/scripts

Examples:
- Renaming or removing a CRD field
- Changing the structure of `.spec` or `.status`
- Changing default behaviors that users depend on

### CRD Versioning

CRDs use API versions like `v1alpha1`, `v1beta1`, `v1`:

| CRD Version | Operator Version | Stability |
|-------------|------------------|-----------|
| `v1alpha1` | 0.x.x, 1.0.x | Experimental, may change |
| `v1beta1` | 1.x.x | Feature complete, minor changes possible |
| `v1` | 2.0.0+ | Stable, backward compatible |

---

## Version Locations

Version information is stored in several places:

| File | Variable/Field | Purpose |
|------|----------------|---------|
| `Makefile` | `VERSION ?= 1.0.0` | **Source of truth** for builds |
| `install.yaml` | `image: ghcr.io/.../mssql-operator:v1.0.0` | User-facing install manifest |
| Git tags | `v1.0.0` | Triggers CI/CD release builds |
| GitHub Releases | Release title and notes | User-facing changelog |

### Updating Version

When preparing a release, update the version in `Makefile`:

```makefile
# In Makefile
VERSION ?= 1.1.0  # Change this
```

Then regenerate manifests:

```bash
make manifests generate
make generate-install-yaml
```

---

## Image Tagging Strategy

Container images are pushed to GitHub Container Registry (`ghcr.io`):

| Image Tag | When Created | Purpose | Mutable? |
|-----------|--------------|---------|----------|
| `ghcr.io/tejasaks/mssql-operator:v1.0.0` | On git tag push | Specific release | ❌ No |
| `ghcr.io/tejasaks/mssql-operator:latest` | On release publish | Latest stable | ✅ Yes |
| `ghcr.io/tejasaks/mssql-operator:main` | On main branch push | Development/testing | ✅ Yes |
| `ghcr.io/tejasaks/mssql-operator:sha-abc1234` | On every push | Debugging specific commits | ❌ No |

### Which Tag Should Users Use?

| Use Case | Recommended Tag |
|----------|-----------------|
| Production | Specific version: `v1.0.0` |
| Following latest stable | `latest` (with caution) |
| Testing new features | `main` |
| Reproducing issues | SHA tag: `sha-abc1234` |

---

## Release Workflow

### Step-by-Step Release Process

#### 1. Prepare the Release

```bash
# Ensure you're on main and up to date
git checkout main
git pull origin main

# Update version in Makefile
# Edit VERSION ?= X.Y.Z

# Update CHANGELOG.md with release notes
# Follow Keep a Changelog format: https://keepachangelog.com/

# Regenerate manifests with new version
make manifests generate
make generate-install-yaml

# Verify everything builds
make build
make test

# Commit the release preparation
git add -A
git commit -m "Release v1.1.0"
git push origin main
```

#### 2. Create and Push the Tag

```bash
# Create an annotated tag
git tag -a v1.1.0 -m "Release v1.1.0

Highlights:
- Feature X
- Bug fix Y
- Improvement Z"

# Push the tag to GitHub
git push origin v1.1.0
```

#### 3. GitHub Actions Builds Automatically

When the tag is pushed, GitHub Actions will:
1. Build the Docker image
2. Push to `ghcr.io/tejasaks/mssql-operator:v1.1.0`
3. Create a draft GitHub Release

#### 4. Publish the GitHub Release

1. Go to [GitHub Releases](https://github.com/tejasaks/PublicDemos/releases)
2. Find the draft release for `v1.1.0`
3. Edit the release notes (copy from CHANGELOG.md)
4. Click **Publish release**

This will:
- Make the release visible to users
- Update the `latest` image tag

---

## Quick Reference

### Release Checklist

```
□ Update VERSION in Makefile
□ Update CHANGELOG.md
□ Run: make manifests generate
□ Run: make generate-install-yaml  
□ Run: make test
□ Commit: "Release vX.Y.Z"
□ Push to main
□ Create tag: git tag -a vX.Y.Z -m "Release vX.Y.Z"
□ Push tag: git push origin vX.Y.Z
□ Wait for GitHub Actions to complete
□ Publish GitHub Release
□ Verify: kubectl apply -f install.yaml works
```

### Common Commands

```bash
# Check current version
grep "VERSION ?=" Makefile

# List all tags
git tag -l

# Delete a tag (if you made a mistake)
git tag -d v1.1.0
git push origin --delete v1.1.0

# View GitHub Actions status
# Visit: https://github.com/tejasaks/PublicDemos/actions
```

---

## Hotfix Releases

For critical bug fixes that need to be released immediately:

### Scenario: v1.0.0 is in production, v1.1.0 is in development, critical bug found

```bash
# Create a release branch from the tag
git checkout -b release/v1.0 v1.0.0

# Apply the fix
# ... make changes ...

# Update version to patch
# Edit Makefile: VERSION ?= 1.0.1

# Regenerate and test
make manifests generate
make generate-install-yaml
make test

# Commit
git add -A
git commit -m "Fix critical bug X"

# Tag and push
git tag -a v1.0.1 -m "Hotfix: Fix critical bug X"
git push origin release/v1.0
git push origin v1.0.1

# Also apply fix to main (cherry-pick or merge)
git checkout main
git cherry-pick <commit-hash>
git push origin main
```

---

## Pre-release Versions

For testing before a stable release:

### Alpha Releases (early testing)

```bash
git tag -a v1.1.0-alpha.1 -m "Alpha release for testing"
git push origin v1.1.0-alpha.1
```

### Beta Releases (feature complete, testing)

```bash
git tag -a v1.1.0-beta.1 -m "Beta release for wider testing"
git push origin v1.1.0-beta.1
```

### Release Candidates (final testing)

```bash
git tag -a v1.1.0-rc.1 -m "Release candidate 1"
git push origin v1.1.0-rc.1
```

Pre-release images are tagged accordingly:
- `ghcr.io/tejasaks/mssql-operator:v1.1.0-alpha.1`
- `ghcr.io/tejasaks/mssql-operator:v1.1.0-beta.1`
- `ghcr.io/tejasaks/mssql-operator:v1.1.0-rc.1`

---

## Branch Strategy

```
main ─────●─────●─────●─────●─────●─────●─────→ (active development)
          │           │           │
          │           │           └─── v1.1.0 (new features)
          │           │
          │           └─── v1.0.1 (hotfix)
          │
          └─── v1.0.0 (initial GA)
                      │
                      └─── release/v1.0 ─────●───→ (maintenance branch)
```

### Branch Types

| Branch | Purpose | Lifetime |
|--------|---------|----------|
| `main` | Active development | Permanent |
| `release/v1.0` | Maintenance for 1.0.x | Until EOL |
| `feature/xyz` | Feature development | Until merged |
| `hotfix/issue-123` | Emergency fixes | Until merged |

---

## Troubleshooting

### GitHub Actions Failed

1. Check the [Actions tab](https://github.com/tejasaks/PublicDemos/actions)
2. Click on the failed workflow
3. Review logs for errors
4. Common issues:
   - Docker build failed: Check Dockerfile syntax
   - Tests failed: Run `make test` locally
   - Push failed: Check GITHUB_TOKEN permissions

### Image Not Found

If users report "image not found":

```bash
# Verify the image exists
docker pull ghcr.io/tejasaks/mssql-operator:v1.0.0

# Check if the package is public
# Visit: https://github.com/tejasaks?tab=packages
```

### Wrong Version Deployed

```bash
# Check what version is running
kubectl get deployment mssql-operator -n mssql-system -o jsonpath='{.spec.template.spec.containers[0].image}'

# Force update to specific version
kubectl set image deployment/mssql-operator \
  operator=ghcr.io/tejasaks/mssql-operator:v1.0.0 \
  -n mssql-system
```

---

## Related Documentation

- [Local Development](local-development.md) - Development setup
- [Upgrades](../operations/upgrades.md) - User upgrade guide
- [GitHub Actions Workflow](../../.github/workflows/build-operator.yaml) - CI/CD configuration
