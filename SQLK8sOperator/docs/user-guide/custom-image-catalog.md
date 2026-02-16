# Custom Image Catalog

The MSSQL Operator supports an **image catalog** that allows cluster administrators
to register unlimited SQL Server container images and expose them to users via a
simple version key. This eliminates the need for users to know or type full image
references — they just set `spec.version` to a catalog key.

---

## How It Works

```
  OperatorConfiguration                    SQLServer CR
  ─────────────────────                    ─────────────
  images:                                  spec:
    catalog:                                 version: "2025-fts"   ──┐
      "2025-fts": registry/sql:2025-fts  <───────────────────────────┘
      "2025-ai":  registry/sql-ai:2025       (catalog lookup)
```

**Image Resolution Priority:**

| Priority | Source | Example |
|----------|--------|---------|
| 1 (highest) | `spec.instance.image` | Per-resource escape hatch |
| 2 | Catalog entry `images.catalog[version]` | `"2025-fts" → registry/sql:2025-fts` |
| 3 (lowest) | Built-in defaults | `"2022" → mcr.microsoft.com/mssql/server:2022-latest` |

Built-in defaults for `"2019"`, `"2022"`, and `"2025"` are always available without
any OperatorConfiguration. Custom keys (e.g., `"2025-fts"`, `"2025-ai"`) must be
registered in the catalog before users can reference them.

---

## Workflow 1: Pin CU Versions (Standard Images)

Pin all SQL Server deployments to specific Cumulative Update versions.

### Step 1: Create the OperatorConfiguration

```yaml
# operator-configuration.yaml
apiVersion: mssql.microsoft.com/v1alpha1
kind: OperatorConfiguration
metadata:
  name: default
spec:
  images:
    catalog:
      "2022": mcr.microsoft.com/mssql/server:2022-CU16-ubuntu-22.04
      "2025": mcr.microsoft.com/mssql/server:2025-latest
```

```bash
kubectl apply -f operator-configuration.yaml
```

### Step 2: Deploy SQL Server (no change for users)

```yaml
apiVersion: mssql.microsoft.com/v1alpha1
kind: SQLServer
metadata:
  name: my-sql
  namespace: mssql
spec:
  version: "2022"          # Resolves to 2022-CU16-ubuntu-22.04
  edition: Developer
  instance:
    count: 1
    storage:
      data:
        size: 10Gi
  credentials:
    saPasswordSecretRef:
      name: mssql-sa-password
```

### Step 3: Verify the resolved image

```bash
kubectl get mssql my-sql -n mssql -o jsonpath='{.status.currentImage}'
# Output: mcr.microsoft.com/mssql/server:2022-CU16-ubuntu-22.04
```

---

## Workflow 2: Register Custom SQL Server Images

Register organization-specific SQL Server images with additional features
(Full-Text Search, Polybase, AI extensions, etc.).

### Step 1: Build and push your custom image

```bash
# Example: SQL Server 2025 + Full-Text Search
docker build -t myregistry.azurecr.io/mssql/server:2025-fts -f Dockerfile.fts .
docker push myregistry.azurecr.io/mssql/server:2025-fts
```

### Step 2: Register the image in the catalog

```yaml
apiVersion: mssql.microsoft.com/v1alpha1
kind: OperatorConfiguration
metadata:
  name: default
spec:
  images:
    catalog:
      # Standard versions (pin CU)
      "2022": mcr.microsoft.com/mssql/server:2022-CU16-ubuntu-22.04
      "2025": mcr.microsoft.com/mssql/server:2025-latest
      # Custom images
      "2025-fts": myregistry.azurecr.io/mssql/server:2025-fts
    imagePullSecrets:
      - acr-pull-secret
```

```bash
kubectl apply -f operator-configuration.yaml
```

> **No operator restart required.** The operator reads the catalog on every
> reconciliation loop, so new entries are available immediately.

### Step 3: Deploy SQL Server with the custom image

```yaml
apiVersion: mssql.microsoft.com/v1alpha1
kind: SQLServer
metadata:
  name: search-sql
  namespace: mssql
spec:
  version: "2025-fts"     # ← Custom catalog key
  edition: Enterprise
  instance:
    count: 1
    storage:
      data:
        size: 50Gi
  credentials:
    saPasswordSecretRef:
      name: mssql-sa-password
```

### Step 4: Verify

```bash
kubectl get mssql search-sql -n mssql -o jsonpath='{.status.currentImage}'
# Output: myregistry.azurecr.io/mssql/server:2025-fts
```

---

## Workflow 3: AI-Enhanced SQL Server (SQL 2025 + Ollama)

Register a SQL Server image bundled with local AI capabilities.

### Step 1: Build the custom image

See [SQLAICustomContainer](https://github.com/tejasaks/SQLAICustomContainer)
for an example Dockerfile that bundles SQL Server 2025 with Ollama, MinIO, and Caddy.

```bash
docker build -t myregistry.azurecr.io/mssql-ai:2025-ollama .
docker push myregistry.azurecr.io/mssql-ai:2025-ollama
```

### Step 2: Register in catalog

```yaml
apiVersion: mssql.microsoft.com/v1alpha1
kind: OperatorConfiguration
metadata:
  name: default
spec:
  images:
    catalog:
      "2025": mcr.microsoft.com/mssql/server:2025-latest
      "2025-ai": myregistry.azurecr.io/mssql-ai:2025-ollama
    imagePullSecrets:
      - acr-pull-secret
```

```bash
kubectl apply -f operator-configuration.yaml
```

### Step 3: Deploy

```yaml
apiVersion: mssql.microsoft.com/v1alpha1
kind: SQLServer
metadata:
  name: ai-sql
  namespace: mssql
spec:
  version: "2025-ai"      # ← AI-enhanced image
  edition: Enterprise
  instance:
    count: 1
    resources:
      requests:
        cpu: "4"
        memory: 16Gi
      limits:
        cpu: "8"
        memory: 32Gi
    storage:
      data:
        size: 100Gi
  credentials:
    saPasswordSecretRef:
      name: mssql-sa-password
```

---

## Workflow 4: Private Registry (Air-Gapped)

Mirror all images to an internal registry for air-gapped environments.

### Step 1: Mirror images

```bash
# Mirror standard SQL Server images
for tag in 2019-CU27-ubuntu-22.04 2022-CU16-ubuntu-22.04 2025-latest; do
  docker pull mcr.microsoft.com/mssql/server:$tag
  docker tag  mcr.microsoft.com/mssql/server:$tag myregistry.internal/mssql/server:$tag
  docker push myregistry.internal/mssql/server:$tag
done

# Mirror custom images
docker pull myregistry.azurecr.io/mssql/server:2025-fts
docker tag  myregistry.azurecr.io/mssql/server:2025-fts myregistry.internal/mssql/server:2025-fts
docker push myregistry.internal/mssql/server:2025-fts
```

### Step 2: Configure catalog

```yaml
apiVersion: mssql.microsoft.com/v1alpha1
kind: OperatorConfiguration
metadata:
  name: default
spec:
  images:
    catalog:
      "2019": myregistry.internal/mssql/server:2019-CU27-ubuntu-22.04
      "2022": myregistry.internal/mssql/server:2022-CU16-ubuntu-22.04
      "2025": myregistry.internal/mssql/server:2025-latest
      "2025-fts": myregistry.internal/mssql/server:2025-fts
    agHelper: myregistry.internal/mssql-operator/ag-helper:v1.0.0
    sqlExporter: myregistry.internal/third-party/sql-exporter:0.14.0
    imagePullSecrets:
      - internal-registry-secret
    defaultPullPolicy: IfNotPresent
```

### Step 3: Deploy — users never see the internal registry URL

```yaml
spec:
  version: "2025-fts"     # Same key regardless of registry
```

---

## Workflow 5: Per-Resource Image Override (Escape Hatch)

For one-off testing or images not yet in the catalog, use `spec.instance.image`
to bypass the catalog entirely.

```yaml
apiVersion: mssql.microsoft.com/v1alpha1
kind: SQLServer
metadata:
  name: test-sql
  namespace: mssql
spec:
  version: "2025"         # Still required, but ignored when image is set
  edition: Developer
  instance:
    image: myregistry.azurecr.io/mssql-experimental:2025-beta3
    count: 1
    storage:
      data:
        size: 10Gi
  credentials:
    saPasswordSecretRef:
      name: mssql-sa-password
```

> **Tip:** Once you've validated the custom image, register it in the catalog so
> all users can reference it by version key without knowing the full image reference.

---

## Operational Notes

### Adding a New Catalog Entry

| Step | Action | Who |
|------|--------|-----|
| 1 | Build and push the custom image to your registry | DevOps / Image Builder |
| 2 | Edit OperatorConfiguration to add the catalog entry | Cluster Admin |
| 3 | `kubectl apply -f operator-configuration.yaml` | Cluster Admin |
| 4 | Users reference the new key in `spec.version` | App Teams |

**Zero downtime. No operator restart. No CRD changes. No code rebuild.**

### Listing Available Versions

The webhook validation will report all available version keys if a user
specifies an unknown version:

```bash
kubectl apply -f bad-version.yaml
# Error: version "2025-gpu" does not resolve to any container image.
#        Available versions: 2019, 2022, 2025, 2025-ai, 2025-fts.
#        Add it to OperatorConfiguration.spec.images.catalog or set spec.instance.image directly
```

### Built-in Default Images

These version keys are always available without any OperatorConfiguration:

| Key | Image |
|-----|-------|
| `"2019"` | `mcr.microsoft.com/mssql/server:2019-latest` |
| `"2022"` | `mcr.microsoft.com/mssql/server:2022-latest` |
| `"2025"` | `mcr.microsoft.com/mssql/server:2025-latest` |

To override these defaults (e.g., pin a CU version), add the key to your catalog.
Catalog entries always take priority over built-in defaults.
