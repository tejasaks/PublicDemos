# Gemini Code Assistant Context

This document provides context for the Gemini AI code assistant to understand the SQL Server Kubernetes Operator project.

## Project Overview

This project is a Kubernetes operator for managing SQL Server instances. It is written in Go and uses the controller-runtime library. The operator defines Custom Resource Definitions (CRDs) for `SQLServer` and `SQLServerAG` to manage SQL Server instances and Availability Groups, respectively.

The project follows the standard operator pattern with controllers that reconcile the state of the custom resources. It also includes a sidecar container (`ag-helper`) for managing Availability Groups.

**Key Technologies:**

*   Go
*   Kubernetes
*   controller-runtime
*   Docker

## Architecture

The operator consists of a few key components:

*   **Operator:** The main process that runs the controllers. It watches for changes to the custom resources and reconciles the state of the cluster.
*   **AG Helper:** A sidecar container that runs alongside SQL Server in each pod. It is responsible for monitoring the health of the local SQL Server instance and reporting it back to the operator. It also has an API to trigger failovers.
*   **SQL Exporter:** A sidecar container that exports Prometheus metrics from SQL Server.

The operator manages the following Kubernetes resources:

*   **StatefulSets:** To deploy the SQL Server pods.
*   **Services:** To expose the SQL Server instances to the network.
*   **PVCs:** For persistent storage.
*   **ConfigMaps:** To store SQL Server configuration.

## Custom Resources (CRDs)

### `SQLServer`

This CRD defines a SQL Server instance. It includes properties for:

*   The version of SQL Server to deploy.
*   The edition of SQL Server (Developer, Express, Standard, or Enterprise).
*   The number of instances.
*   The resources to allocate to each instance (CPU, memory).
*   The storage configuration.
*   The credentials for the `sa` user.

### `SQLServerAG`

This CRD defines an Availability Group. It includes properties for:

*   A reference to the `SQLServer` resource that the AG belongs to.
*   The name of the Availability Group.
*   The number of instances in the AG.
*   Whether to enable automatic failover.
*   The configuration for the listener.

## Controllers

### `SQLServer` Controller

This controller is responsible for reconciling `SQLServer` resources. It creates and manages the StatefulSet, Services, PVCs, and ConfigMaps for the SQL Server instance.

### `SQLServerAG` Controller

This controller is responsible for reconciling `SQLServerAG` resources. It monitors the health of the AG by querying the `ag-helper` sidecars. It can also trigger automatic failovers if a primary replica becomes unavailable.

## AG Helper

The `ag-helper` is a critical component for managing Availability Groups. It runs as a sidecar in each SQL Server pod and is responsible for:

*   Querying SQL Server DMVs to get the health of the local replica.
*   Exposing an HTTP API with the replica's state.
*   Executing failover commands when instructed by the controller.

## Development

The project uses a `Makefile` for all common development tasks.

### Setup

1.  Install Go, Docker, kubectl, and Kind or Minikube.
2.  Create a local Kubernetes cluster: `make kind-create`
3.  Install dependencies: `go mod download`
4.  Install development tools: `make install-tools`

### Building

*   `make build`: Builds the operator binary.
*   `make build-sidecar`: Builds the `ag-helper` sidecar binary.
*   `make docker-build`: Builds the operator Docker image.
*   `make docker-build-sidecar`: Builds the `ag-helper` sidecar Docker image.

### Running

*   `make run`: Runs the operator locally.
*   `make deploy`: Deploys the operator to a Kubernetes cluster.
*   `make minikube-deploy`: Deploys the operator to a local Minikube cluster.

### Testing

*   `make test`: Runs unit tests.

### Code Generation

*   `make generate`: Generates DeepCopy methods.
*   `make manifests`: Generates CRD manifests.

### Linting

*   `make lint`: Runs the linter.

### Quick Patching

For faster development, you can use the following aliases to patch the operator and AG helper:

```bash
# Quick patch operator
alias patch-op='eval $(minikube docker-env) && make docker-build IMG=mssql-operator:latest && kubectl rollout restart deployment/mssql-operator -n mssql-system'

# Quick patch with CRDs
alias patch-op-crd='eval $(minikube docker-env) && make manifests && make docker-build IMG=mssql-operator:latest && kubectl apply -f deploy/crds/ --force && kubectl rollout restart deployment/mssql-operator -n mssql-system'

# Quick patch AG Helper
alias patch-ag='eval $(minikube docker-env) && make docker-build-ag-helper IMG=ag-helper:latest && kubectl delete pods -n mssql -l app.kubernetes.io/component=ag-helper'
```
