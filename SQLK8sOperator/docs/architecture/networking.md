# Networking Architecture

[← Back to Architecture](overview.md) | [Documentation Home](../README.md)

This document describes the network architecture, including Services, DNS, and traffic flow.

## Table of Contents

- [Service Types](#service-types)
- [Service Architecture](#service-architecture)
- [DNS Resolution](#dns-resolution)
- [Port Mapping](#port-mapping)
- [Traffic Flow](#traffic-flow)
- [Network Policies](#network-policies)
- [External Access](#external-access)
- [TLS/mTLS](#tlsmtls)

## Service Types

### Kubernetes Service Types Used

| Service Type | Use Case | External Access |
|--------------|----------|-----------------|
| **ClusterIP** | Internal cluster access | No |
| **Headless** | StatefulSet DNS | No |
| **LoadBalancer** | External access (cloud) | Yes |
| **NodePort** | External access (bare metal) | Yes |

## Service Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                        External Traffic                              │
└───────────────────────────────┬─────────────────────────────────────┘
                                │
        ┌───────────────────────┼───────────────────────┐
        │                                               │
        ▼                                               ▼
┌───────────────┐                               ┌───────────────────┐
│ sql-prod-01   │                               │ productionag-     │
│ (Headless)    │                               │ listener          │
│               │                               │ (Selectorless)    │
│ No ClusterIP  │                               │                   │
│ DNS only      │                               │ ClusterIP or LB   │
│               │                               │ Operator-managed  │
│ Port: 1433    │                               │ Endpoints         │
└───────┬───────┘                               │ Port: 1433        │
        │                                       └─────────┬─────────┘
        │                                                 │
        │                                                 │ Endpoints point
        │                                                 │ to current primary
        │                                                 │ pod IP only
        ▼                                                 ▼
┌─────────────────────────────────────────────────────────────────────┐
│                         Pod Endpoints                                │
│                                                                      │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐      │
│  │ sql-prod-01-0   │  │ sql-prod-01-1   │  │ sql-prod-01-2   │      │
│  │ (PRIMARY)       │  │ (SECONDARY)     │  │ (SECONDARY)     │      │
│  │ 10.0.1.10:1433  │  │ 10.0.1.11:1433  │  │ 10.0.1.12:1433  │      │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘      │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

### Services Created

| Resource | Creates | Purpose |
|----------|---------|---------|
| SQLServer | Headless Service | Pod DNS names |
| SQLServer | Client Service (optional) | Direct pod access |
| SQLServerAG | Listener Service (optional) | Route to current primary via operator-managed Endpoints |

## DNS Resolution

### StatefulSet DNS

StatefulSets with headless services get predictable DNS names:

| DNS Name | Resolves To |
|----------|-------------|
| `sql-prod-01-0.sql-prod-01-pods.mssql.svc.cluster.local` | Pod 0 IP |
| `sql-prod-01-1.sql-prod-01-pods.mssql.svc.cluster.local` | Pod 1 IP |
| `sql-prod-01-2.sql-prod-01-pods.mssql.svc.cluster.local` | Pod 2 IP |
| `sql-prod-01-pods.mssql.svc.cluster.local` | All pod IPs (headless) |

### Service DNS

| DNS Name | Resolves To |
|----------|-------------|
| `productionag-listener.mssql.svc.cluster.local` | Current primary pod IP (via operator-managed Endpoints) |

### DNS Format

```
<pod-name>.<headless-svc>.<namespace>.svc.<cluster-domain>
│           │              │          │    │
│           │              │          │    └── cluster.local (default)
│           │              │          └── svc (service subdomain)
│           │              └── mssql (namespace)
│           └── sql-prod-01-pods (headless service)
└── sql-prod-01-0 (pod name from StatefulSet)
```

## Port Mapping

### Container Ports

| Container | Port | Protocol | Purpose |
|-----------|------|----------|---------|
| mssql-server | 1433 | TCP | SQL Server TDS |
| mssql-server | 5022 | TCP | AG mirroring endpoint |
| ag-helper | 8080 | HTTP | Health probes, API |
| sql-exporter | 9399 | HTTP | Prometheus metrics |

### Service Port Mapping

```yaml
# Listener Service (selectorless, operator-managed Endpoints)
apiVersion: v1
kind: Service
metadata:
  name: productionag-listener
spec:
  type: ClusterIP           # or LoadBalancer
  ports:
    - name: sql
      port: 1433             # Service port
      targetPort: 1433       # Container port
      protocol: TCP
  # No selector — Endpoints managed by the operator
  # to always point to the current primary pod IP
```

## Traffic Flow

### Read-Write Traffic (via Listener)

```
┌──────────┐     ┌────────────────────────┐     ┌───────────────┐
│  Client  │────▶│ productionag-listener  │────▶│ sql-prod-01-0 │
│          │     │ Selectorless Service   │     │ (PRIMARY)     │
│          │     │ 10.0.0.100:1433       │     │ 10.0.1.10:1433│
└──────────┘     └────────────────────────┘     └───────────────┘
                   Endpoints managed by
                   operator → primary pod IP
```

### Read-Only Traffic (Direct Replica Access)

For read-only workloads, connect directly to secondary replica services
created by the SQLServer resource:

```
┌──────────┐     ┌───────────────────┐     ┌───────────────┐
│  Client  │────▶│ sql-prod-01-1     │────▶│ sql-prod-01-1 │
│          │     │ (direct service)  │     │ (SECONDARY)   │
│          │     │ 10.0.1.11:1433    │     │               │
└──────────┘     └───────────────────┘     └───────────────┘
```

### Failover Traffic Rerouting

When failover occurs, the operator updates the Endpoints resource to point to the new primary pod:

```
Before Failover:                    After Failover:
─────────────────                   ────────────────

Listener Endpoints ──▶ Pod 0 IP     Listener Endpoints ──▶ Pod 1 IP
                      (PRIMARY)                            (PRIMARY)
```

The operator queries each pod's AG Helper sidecar (`GET /state` on port 8080) to discover which pod is the current PRIMARY, then updates the Endpoints object to route the listener Service to that pod's IP. No pod labels are involved in traffic routing.

### AG Endpoint Traffic

```
┌─────────────────┐     TCP:5022    ┌─────────────────┐
│  sql-prod-01-0  │◀───────────────▶│  sql-prod-01-1  │
│  (PRIMARY)      │                 │  (SECONDARY)    │
│                 │◀───────────────▶│                 │
│                 │                 └─────────────────┘
│                 │     TCP:5022    ┌─────────────────┐
│                 │◀───────────────▶│  sql-prod-01-2  │
└─────────────────┘                 │  (SECONDARY)    │
                                    └─────────────────┘
```

## Network Policies

### Recommended Network Policies

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: sql-server-policy
  namespace: mssql
spec:
  podSelector:
    matchLabels:
      app: mssql
  policyTypes:
    - Ingress
    - Egress
  ingress:
    # Allow SQL connections from any namespace
    - ports:
        - port: 1433
          protocol: TCP
    # Allow AG replication between replicas
    - from:
        - podSelector:
            matchLabels:
              app: mssql
      ports:
        - port: 5022
          protocol: TCP
    # Allow Prometheus scraping
    - from:
        - namespaceSelector:
            matchLabels:
              name: monitoring
      ports:
        - port: 9399
          protocol: TCP
    # Allow health probes from anywhere in cluster
    - ports:
        - port: 8080
          protocol: TCP
  egress:
    # Allow DNS
    - to:
        - namespaceSelector: {}
      ports:
        - port: 53
          protocol: UDP
    # Allow AG replication
    - to:
        - podSelector:
            matchLabels:
              app: mssql
      ports:
        - port: 5022
          protocol: TCP
```

## External Access

### LoadBalancer (Cloud)

```yaml
spec:
  service:
    type: LoadBalancer
    port: 1433
    annotations:
      # Azure: internal LB
      service.beta.kubernetes.io/azure-load-balancer-internal: "true"
      # AWS: internal ALB
      service.beta.kubernetes.io/aws-load-balancer-internal: "true"
```

### NodePort (Bare Metal)

```yaml
spec:
  service:
    type: NodePort
    port: 1433
    nodePort: 31433  # Access via <any-node-ip>:31433
```

### Ingress (TCP)

For TCP ingress (not HTTP), use a TCP-capable ingress controller:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: tcp-services
  namespace: ingress-nginx
data:
  "1433": "mssql/productionag-listener:1433"
```

## TLS/mTLS

### SQL Server TLS

Enable TLS for SQL connections:

```yaml
spec:
  instance:
    config:
      tlsEnabled: true
      tlsCertSecretRef:
        name: mssql-tls-cert
```

Required secret format:
```yaml
apiVersion: v1
kind: Secret
metadata:
  name: mssql-tls-cert
type: kubernetes.io/tls
data:
  tls.crt: <base64-encoded-cert>
  tls.key: <base64-encoded-key>
```

### AG Endpoint Encryption

AG endpoints are encrypted by default:

```sql
CREATE ENDPOINT AG_Endpoint
    STATE = STARTED
    AS TCP (LISTENER_PORT = 5022)
    FOR DATABASE_MIRRORING (
        ROLE = ALL,
        AUTHENTICATION = CERTIFICATE AG_Cert,
        ENCRYPTION = REQUIRED ALGORITHM AES  -- Encrypted
    );
```

### Connection String with TLS

```
Server=productionag-listener.mssql.svc.cluster.local,1433;
Database=MyDB;
User Id=sa;
Password=xxx;
Encrypt=true;
TrustServerCertificate=false;
```

## Next Steps

- [Getting Started](../getting-started.md) - Deploy your first SQL Server
- [AG Deployment Guide](../availability-groups/deployment-guide.md) - Set up HA
- [External Access](../user-guide/deployment-scenarios.md#external-access) - More access options
