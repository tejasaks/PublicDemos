# Scripts

Development and build automation scripts for the SQL Server Kubernetes Operator.

## Files

### [dev-setup.sh](dev-setup.sh)

All-in-one local development script for **Linux/Ubuntu with minikube**. Handles prerequisites checking, cluster bootstrap, image building, operator installation, sample deployment, monitoring setup, and port-forwarding — all via subcommands.

```bash
./scripts/dev-setup.sh all                          # Full local dev setup
./scripts/dev-setup.sh deploy samples/sql-ag-ha/ag-deploy.yaml  # Deploy a sample
./scripts/dev-setup.sh monitoring                   # Add Prometheus + Grafana
./scripts/dev-setup.sh connect                      # Port-forward to SQL Server
```

Use `--remote` to pull images from ghcr.io instead of building locally.

### [generate-install-yaml.sh](generate-install-yaml.sh)

Bash script that combines all manifests from `deploy/` into a single `install.yaml` at the project root. This is the file users apply for one-command operator installation. Accepts optional version, operator image, and sidecar image arguments.

```bash
./scripts/generate-install-yaml.sh v1.0.0
./scripts/generate-install-yaml.sh v1.0.0 ghcr.io/myorg/mssql-operator:v1.0.0 ghcr.io/myorg/mssql-ag-helper:v1.0.0
```

### [generate-install-yaml.ps1](generate-install-yaml.ps1)

PowerShell equivalent of `generate-install-yaml.sh` for Windows development environments. Same output, same parameters.

```powershell
.\scripts\generate-install-yaml.ps1 -Version "v1.0.0"
```

## Documentation

- **dev-setup.sh usage and subcommands:** [Building & Development](../docs/development/building.md#alternative-dev-setupsh-script)
- **install.yaml generation workflow:** [Building & Development](../docs/development/building.md#generate-installation-manifest)
- **Local development guide:** [Local Development](../docs/development/local-development.md)
