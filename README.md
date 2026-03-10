# PublicDemos

Sample projects for running SQL Server on containers and Kubernetes — from a single Docker container to a fully managed, highly available deployment.

## Projects

### [SQL Server Kubernetes Operator](SQLK8sOperator/)

A Kubernetes operator that declaratively deploys and manages SQL Server instances. Define your SQL Server as a Custom Resource and the operator handles provisioning, monitoring, and high availability with Availability Groups — including automated failover, health monitoring via `sp_server_diagnostics`, and Prometheus metrics export.

- **Deploy SQL Server 2019, 2022, or 2025** with a single `kubectl apply`
- **High availability** through native SQL Server Availability Groups with VIP-based listener routing
- **Monitoring** via a built-in SQL Exporter sidecar for Prometheus/Grafana
- **Validation webhooks** to catch misconfigurations before they reach the cluster

See the [operator documentation](SQLK8sOperator/docs/) for architecture details, getting started guides, and sample manifests.

### [SQL Server 2025 Custom Container](SQLAICustomContainer/)

A Docker-based SQL Server 2025 container with optional AI and data platform components. Build a single image with exactly the stack you need — SQL Server with Full-Text Search is always included, and you can toggle Ollama (AI model runtime), MinIO (S3-compatible object storage), Polybase (external data connectivity), and Caddy (HTTPS reverse proxy).

- **7 deployment configurations** from minimal SQL-only to the full stack
- **Single Dockerfile** supporting both Ubuntu and RHEL base images
- **One script** (`build-and-run.sh`) to build and launch any configuration

See the [container documentation](SQLAICustomContainer/README.md) for configuration options, deployment examples, and test instructions.

## Using Them Together

Both projects are **independently usable** — you don't need one to use the other. However, they complement each other in enterprise scenarios: build a custom SQL Server image with the container project, then deploy and manage it at scale with the operator. The operator's [extensible image catalog](SQLK8sOperator/docs/architecture/operator-design.md) accepts any SQL Server container image, including custom-built ones.

## Status

Both projects are in **preview**. See individual project READMEs for current capabilities and known limitations.
