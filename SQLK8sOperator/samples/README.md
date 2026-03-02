# Samples

Ready-to-use manifests for deploying SQL Server on Kubernetes with the MSSQL Operator. Each AG scenario folder is self-contained with a deploy manifest, T-SQL configuration guide, automation script, and its own README.

## Availability Group Scenarios

### [sql-ag-ha/](sql-ag-ha/)

Deploys a 3-replica SQL Server instance and configures a **high-availability** Availability Group (`ProductionAG`). All three replicas use synchronous commit with automatic external failover. Includes a listener service for single-VIP client connections and LoadBalancer services for direct primary/secondary access. This is the recommended starting point for production HA workloads.

### [sql-ag-dr/](sql-ag-dr/)

Deploys a 3-replica SQL Server instance and configures a **disaster-recovery** Availability Group (`DisasterRecoveryAG`). Two replicas use synchronous commit while the third uses asynchronous commit with manual failover, simulating a remote DR site. No listener is configured — clients connect to individual replicas directly. Use this when you need an async DR replica that does not participate in automatic failover.

### [sql-ag-multiag/](sql-ag-multiag/)

Deploys a 3-replica SQL Server instance and configures **three separate Availability Groups** on the same set of replicas: `ProductionAG` (3 sync, auto failover), `ReportingAG` (1 sync + 2 async, manual failover), and `DisasterRecoveryAG` (1 sync + 1 async, manual seeding). Each AG manages a different database and exposes services on distinct port ranges. Demonstrates how to run multiple AGs with different sync modes, failover policies, and service configurations on a single cluster.

### [sql-ag-monitoring/](sql-ag-monitoring/)

Deploys a 3-replica SQL Server instance with a full **Prometheus + Grafana monitoring stack**. Each pod runs a sql-exporter sidecar that exposes SQL and AG metrics on port 9399. Prometheus auto-discovers exporter targets via `kubernetes_sd_configs`, and Grafana is pre-loaded with two dashboards (AG Monitoring and SQL Server Overview). The AG configuration is identical to `sql-ag-ha` — use this when you want HA plus end-to-end observability out of the box.

### [sql-ag-ha-diagnostics/](sql-ag-ha-diagnostics/)

Extends the `sql-ag-ha` scenario with **`sp_server_diagnostics` integration** via the `failureConditionLevel` CRD field (set to level 3). In addition to standard AG topology monitoring, the AG Helper sidecar runs `EXEC sp_server_diagnostics` each cycle and evaluates the five SQL Server internal components (system, resource, query_processing, io_subsystem, events). A component error at or above the configured level triggers automatic failover. Use this when you need deeper health detection that mirrors WSFC's failure condition level behavior.

## Standalone Instances

### [sqlserver-2025-standalone.yaml](sqlserver-2025-standalone.yaml)

Deploys a single SQL Server 2025 Developer instance (`sql2025-dev01`) with persistent storage for data, log, tempdb, and backups. Configures a LoadBalancer service on port 1433, SQL Agent enabled, and resource limits of 4 CPU / 8 GB. Good for development, testing, or evaluating SQL Server 2025 features without any AG complexity.

### [sqlserver-2022-standalone.yaml](sqlserver-2022-standalone.yaml)

Deploys a single SQL Server 2022 Developer instance (`sql2022-dev01`) with persistent data and log volumes. Uses lighter resource limits (2 CPU / 4 GB) compared to the 2025 sample. Includes commented-out optional settings for custom image tags, storage classes, and environment variables. Useful as a minimal starting point or for workloads that require SQL Server 2022 specifically.

### [sqlserver-with-ad.yaml](sqlserver-with-ad.yaml)

Deploys a SQL Server 2022 Enterprise instance configured for **Active Directory (Kerberos) authentication**. Includes AD domain join settings, keytab secret references, SPN configuration, and CoreDNS forwarding rules for domain controller lookups. Requires an existing AD domain with a service account that has permission to create SPNs. Use this for enterprise environments that need Windows Authentication alongside SQL authentication.

## Operator Configuration

### [operator-configuration.yaml](operator-configuration.yaml)

Comprehensive reference for the `OperatorConfiguration` custom resource. Documents all available settings for container image overrides, image catalog entries, version pinning, and operator behavior. Apply as `default` (cluster-scoped) to control image resolution for all SQL Server deployments cluster-wide.

### [operator-configuration-mcr-defaults.yaml](operator-configuration-mcr-defaults.yaml)

Preconfigured `OperatorConfiguration` that uses official **Microsoft Container Registry (MCR)** images for SQL Server with locally-built operator and AG Helper images. Pins specific Cumulative Update versions for consistency. Best for development, testing, CI/CD pipelines, and quick proof-of-concept deployments where no private registry is needed.

### [operator-configuration-private-registry.yaml](operator-configuration-private-registry.yaml)

Preconfigured `OperatorConfiguration` that sources **all container images from a private registry** (ACR, ECR, Harbor, etc.). Includes step-by-step instructions for pulling from MCR and pushing to your registry. Best for production, air-gapped, or regulated environments with corporate security and image scanning requirements.

### [operator-configuration-local-dev.yaml](operator-configuration-local-dev.yaml)

Overrides container images to use **locally-built** operator and AG Helper images instead of pulling from a remote registry. Intended for inner-loop development — build your images into minikube's Docker daemon, apply this config, and test local code changes immediately.

## Other Files

### [ag-helper-credentials-secret.yaml](ag-helper-credentials-secret.yaml)

Reference template for pre-creating AG Helper credential secrets separately from the AG deploy manifests. Useful for GitOps workflows or production environments where secrets are managed independently. For dev/test, the AG scenario manifests already include these secrets inline.

### [scripts/](scripts/)

Standalone T-SQL scripts for AG setup (`setup-availability-group.sql`, `join-secondary.sql`, `verify-ag-status.sql`). These are the raw SQL building blocks used by the scenario folders' `ag-configure.sh` scripts. See the [scripts README](scripts/README.md) for usage details.
