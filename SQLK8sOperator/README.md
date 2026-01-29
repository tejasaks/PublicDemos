# SQL Server Kubernetes Operator

A Kubernetes operator for managing SQL Server instances with high availability, automated upgrades, and monitoring.

[![Go Version](https://img.shields.io/badge/Go-1.21+-blue.svg)](https://go.dev)
[![Kubernetes](https://img.shields.io/badge/Kubernetes-1.26+-blue.svg)](https://kubernetes.io)
[![SQL Server](https://img.shields.io/badge/SQL%20Server-2019%20|%202022%20|%202025-orange.svg)](https://www.microsoft.com/sql-server)

## ⚡ Quick Start (30 seconds)

Get up and running immediately with a single command:

```bash
# Install the operator directly from this repository
kubectl apply -f https://raw.githubusercontent.com/tejasaks/PublicDemos/main/SQLK8sOperator/install.yaml
```

That's it! The operator is now installed. Verify with:

```bash
kubectl get pods -n mssql-system
# Expected: mssql-operator-xxx   1/1   Running
```

**Deploy your first SQL Server:**

```bash
# Create a password secret
kubectl create secret generic mssql-secret --from-literal=SA_PASSWORD='YourStrong!Passw0rd' -n mssql

# Deploy SQL Server 2025
kubectl apply -f https://raw.githubusercontent.com/tejasaks/PublicDemos/main/SQLK8sOperator/samples/sqlserver-2025-standalone.yaml

# Watch it come up
kubectl get pods -n mssql -w
```

→ See [Installation Guide](docs/distribution/end-user-installation.md) for more options including Helm and Kustomize.

---

## Features

- **Deploy SQL Server Instances**: Declarative management of SQL Server 2019, 2022, and 2025
- **High Availability**: Availability Groups with automatic failover
- **Zero-Downtime Upgrades**: Rolling upgrades with AG failover
- **Scaling**: Scale replicas with persistent storage
- **Monitoring**: Prometheus metrics and Grafana dashboards
- **Active Directory**: Windows Authentication support

## Detailed Quick Start

### Prerequisites

- Kubernetes 1.26+
- kubectl configured
- Storage class available

### Install the Operator

```bash
# Option 1: Direct from repository (recommended for quick start)
kubectl apply -f https://raw.githubusercontent.com/tejasaks/PublicDemos/main/SQLK8sOperator/install.yaml

# Option 2: From GitHub Releases (for versioned releases)
kubectl apply -f https://github.com/yourorg/mssql-operator/releases/latest/download/install.yaml
```

### Deploy SQL Server

```bash
# Create namespace and password secret
kubectl create namespace mssql
kubectl create secret generic sql-demo-sa \
  --from-literal=password='YourStr0ng!Passw0rd' \
  -n mssql

# Deploy SQL Server
kubectl apply -f - <<EOF
apiVersion: mssql.microsoft.com/v1alpha1
kind: SQLServer
metadata:
  name: sql-demo
  namespace: mssql
spec:
  version: "2025"
  edition: Developer
  instance:
    replicas: 1
    resources:
      limits:
        cpu: "2"
        memory: 4Gi
      requests:
        cpu: "1"
        memory: 2Gi
    storage:
      data:
        size: 10Gi
  credentials:
    saPasswordSecretRef:
      name: sql-demo-sa
      key: password
EOF
```

### Verify Deployment

```bash
kubectl get sqlserver -n mssql
kubectl get pods -n mssql
```

→ See [Getting Started Guide](docs/getting-started.md) for detailed instructions.

## Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                      SQL Server Kubernetes Operator                  │
├─────────────────────────────────────────────────────────────────────┤
│                                                                      │
│  ┌─────────────────────────────────────────────────────────────┐    │
│  │                 Operator (Deployment)                        │    │
│  │  ┌───────────────┐  ┌───────────────┐  ┌───────────────┐    │    │
│  │  │ SQLServer     │  │ SQLServerAG   │  │ Configuration │    │    │
│  │  │ Controller    │  │ Controller    │  │ Controller    │    │    │
│  │  └───────────────┘  └───────────────┘  └───────────────┘    │    │
│  └─────────────────────────────────────────────────────────────┘    │
│                              │                                       │
│                              ▼                                       │
│  ┌─────────────────────────────────────────────────────────────┐    │
│  │              SQL Server Pod (StatefulSet)                    │    │
│  │  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐          │    │
│  │  │ SQL Server  │  │ AG Helper   │  │SQL Exporter │          │    │
│  │  │ :1433       │  │ :8080       │  │ :9399       │          │    │
│  │  └─────────────┘  └─────────────┘  └─────────────┘          │    │
│  └─────────────────────────────────────────────────────────────┘    │
│                                                                      │
│  ┌─────────────────────────────────────────────────────────────┐    │
│  │                        Services                              │    │
│  │  ┌───────────────┐  ┌────────────────┐  ┌────────────────┐  │    │
│  │  │ Primary       │  │ Secondary      │  │ Metrics        │  │    │
│  │  │ :1433         │  │ :1434          │  │ :9399          │  │    │
│  │  └───────────────┘  └────────────────┘  └────────────────┘  │    │
│  └─────────────────────────────────────────────────────────────┘    │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

→ See [Architecture Overview](docs/architecture/overview.md) for detailed design.

## Documentation

| Topic | Description |
|-------|-------------|
| **[Getting Started](docs/getting-started.md)** | First deployment guide |
| **[Architecture](docs/architecture/overview.md)** | System design and components |
| **[User Guide](docs/user-guide/deployment-scenarios.md)** | Deployment patterns and configuration |
| **[Availability Groups](docs/availability-groups/overview.md)** | High availability setup |
| **[Monitoring](docs/monitoring/overview.md)** | Prometheus and Grafana setup |
| **[Operations](docs/operations/upgrades.md)** | Upgrades, scaling, backup |
| **[Development](docs/development/local-development.md)** | Contributing to the operator |
| **[Installation](docs/distribution/end-user-installation.md)** | Installation methods |

→ See [Full Documentation](docs/README.md) for complete reference.

## Custom Resources

### SQLServer

Manages SQL Server instances:

```yaml
apiVersion: mssql.microsoft.com/v1alpha1
kind: SQLServer
metadata:
  name: sql-prod
spec:
  version: "2022"
  edition: Enterprise
  instance:
    replicas: 3
    config:
      hadrEnabled: true
  credentials:
    saPasswordSecretRef:
      name: sql-prod-sa
      key: password
```

→ See [Configuration Reference](docs/user-guide/configuration-reference.md)

### SQLServerAG

Manages Availability Groups:

```yaml
apiVersion: mssql.microsoft.com/v1alpha1
kind: SQLServerAG
metadata:
  name: prod-ag
spec:
  sqlServerRef:
    name: sql-prod
  availabilityGroup:
    name: ProductionAG
    replicas: 3
    automaticFailover: true
```

→ See [AG Deployment Guide](docs/availability-groups/deployment-guide.md)

## Validation Rules

The operator validates all inputs to ensure reliable deployments:

| Rule | Requirement |
|------|-------------|
| Resource names | ≤13 characters, lowercase, alphanumeric with hyphens |
| SA password | ≥8 chars, 3+ character types, via Secret reference |
| Memory | ≥2Gi (SQL Server requirement) |
| Edition | Developer, Express, Standard, or Enterprise |

→ See [Validation & Security](docs/user-guide/validation-security.md)

## Samples

Sample manifests are available in the [`samples/`](samples/) directory:

| Sample | Description |
|--------|-------------|
| [`sqlserver-basic.yaml`](samples/sqlserver-basic.yaml) | Standalone development instance |
| [`sqlserver-production.yaml`](samples/sqlserver-production.yaml) | Production-ready configuration |
| [`sqlserver-availability-group.yaml`](samples/sqlserver-availability-group.yaml) | HA with Availability Groups |
| [`sqlserverag.yaml`](samples/sqlserverag.yaml) | AG service configuration |

## Project Information

| Tool | Version | Purpose |
|------|---------|---------|
| Go | 1.21+ | Programming language |
| controller-runtime | v0.17.0 | Kubernetes controller framework |
| Kubernetes | 1.26+ | Target platform |

### Design References

- [Zalando postgres-operator](https://github.com/zalando/postgres-operator) - StatefulSet patterns
- [CrunchyData postgres-operator](https://github.com/CrunchyData/postgres-operator) - controller-runtime patterns
- [Microsoft mssql-server-ha](https://github.com/Microsoft/mssql-server-ha) - AG management logic

## Contributing

We welcome contributions! Please see:

- [Contributing Guide](docs/development/contributing.md)
- [Local Development](docs/development/local-development.md)
- [Testing Guide](docs/development/testing.md)

## License

[MIT License](LICENSE)

## Support

- **Documentation**: [docs/](docs/README.md)
- **Issues**: [GitHub Issues](https://github.com/yourorg/mssql-operator/issues)
- **Discussions**: [GitHub Discussions](https://github.com/yourorg/mssql-operator/discussions)
