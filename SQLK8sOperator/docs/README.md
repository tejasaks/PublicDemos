# SQL Server Kubernetes Operator Documentation

Welcome to the SQL Server Kubernetes Operator documentation.

## Quick Links

| I want to... | Start here |
|--------------|------------|
| Deploy my first SQL Server | [Getting Started](getting-started.md) |
| Understand the architecture | [Architecture Overview](architecture/overview.md) |
| **Deploy an AG (Tutorial)** | **[AG Tutorial](availability-groups/tutorial-ag-deployment.md)** |
| Set up high availability | [AG Deployment Guide](availability-groups/deployment-guide.md) |
| Monitor my SQL Servers | [Monitoring Overview](monitoring/overview.md) |
| Contribute to the project | [Contributing](development/contributing.md) |

## Documentation Structure

### [Getting Started](getting-started.md)
First deployment guide - prerequisites, installation, and verification.

---

### Architecture
Deep dive into how the operator works.

| Document | Description |
|----------|-------------|
| [Overview](architecture/overview.md) | High-level architecture and design principles |
| [Operator Design](architecture/operator-design.md) | Controller patterns and reconciliation |
| [CRD Design](architecture/crd-design.md) | Custom resource definitions |
| [Sidecar Architecture](architecture/sidecar-architecture.md) | AG Helper and SQL Exporter |
| [Networking](architecture/networking.md) | Services, DNS, and traffic routing |

---

### User Guide
Day-to-day usage and configuration.

| Document | Description |
|----------|-------------|
| [Deployment Scenarios](user-guide/deployment-scenarios.md) | Standalone, HA, AD patterns |
| [Configuration Reference](user-guide/configuration-reference.md) | Complete CRD field reference |
| [Validation & Security](user-guide/validation-security.md) | Input validation and security rules |
| [Troubleshooting](user-guide/troubleshooting.md) | Common issues and solutions |

---

### Availability Groups
High availability configuration and management.

| Document | Description |
|----------|-------------|
| [Overview](availability-groups/overview.md) | AG concepts and architecture |
| **[Tutorial: AG Deployment](availability-groups/tutorial-ag-deployment.md)** | **Complete walkthrough from scratch** |
| [Deployment Guide](availability-groups/deployment-guide.md) | Step-by-step AG setup (reference) |
| [Listener Configuration](availability-groups/listener-configuration.md) | AG Listener options and setup |
| [Failover Management](availability-groups/failover-management.md) | Automatic and manual failover |
| [Multi-AG Scenarios](availability-groups/multi-ag-scenarios.md) | Multiple AGs on same cluster |
| [AG Helper Reference](availability-groups/ag-helper-reference.md) | Complete sidecar API |
| [Controller Workflow Details](availability-groups/controller-workflow-details.md) | Deep dive into AG Helper & Controller internals |

---

### Monitoring
Observability setup and dashboards.

| Document | Description |
|----------|-------------|
| [Overview](monitoring/overview.md) | Monitoring architecture |
| [Prometheus Setup](monitoring/prometheus-setup.md) | ServiceMonitor and alerts |
| [Grafana Dashboards](monitoring/grafana-dashboards.md) | Pre-built dashboards |
| [SQL Exporter Reference](monitoring/sql-exporter-reference.md) | All available metrics |

---

### Operations
Day-2 operations and maintenance.

| Document | Description |
|----------|-------------|
| [AG Operations](operations/ag-operations.md) | kubectl commands for AG management |
| [Upgrades](operations/upgrades.md) | Version and CU upgrades, operator patching |
| [Scaling](operations/scaling.md) | Horizontal and vertical scaling |
| [Backup & Restore](operations/backup-restore.md) | Data protection |
| [Active Directory](operations/active-directory.md) | AD/Kerberos integration |

---

### Development
Contributing to the operator.

| Document | Description |
|----------|-------------|
| [Local Development](development/local-development.md) | Development environment setup |
| [Building](development/building.md) | Build process and artifacts |
| [Testing](development/testing.md) | Test framework and coverage |
| [Contributing](development/contributing.md) | Contribution guidelines |

---

### Distribution
Packaging and installation.

| Document | Description |
|----------|-------------|
| [Packaging](distribution/packaging.md) | Release artifacts |
| [Helm Chart](distribution/helm-chart.md) | Helm installation and values |
| [End-User Installation](distribution/end-user-installation.md) | Installation methods |

---

## Version Compatibility

| Operator Version | SQL Server | Kubernetes | Go |
|-----------------|------------|------------|-----|
| v1.0.x | 2019, 2022, 2025 | 1.26+ | 1.21+ |

## Getting Help

- **Issues**: [GitHub Issues](https://github.com/yourorg/mssql-operator/issues)
- **Discussions**: [GitHub Discussions](https://github.com/yourorg/mssql-operator/discussions)

## License

[MIT License](../LICENSE)
