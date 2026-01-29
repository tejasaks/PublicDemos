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
        │                       │                       │
        ▼                       ▼                       ▼
┌───────────────┐       ┌───────────────┐       ┌───────────────┐
│ sql-prod-01   │       │ prod-ag-      │       │ prod-ag-      │
│ (Headless)    │       │ primary       │       │ secondary     │
│               │       │ (LoadBalancer)│       │ (LoadBalancer)│
│ No ClusterIP  │       │               │       │               │
│ DNS only      │       │ ClusterIP +   │       │ ClusterIP +   │
│               │       │ External IP   │       │ External IP   │
│               │       │               │       │               │
│ Port: 1433    │       │ Port: 1433    │       │ Port: 1434    │
└───────┬───────┘       └───────┬───────┘       └───────┬───────┘
        │                       │                       │
        │                       │ Selector:             │ Selector:
        │                       │ role=primary          │ role=secondary
        │                       │                       │
        ▼                       ▼                       ▼
┌─────────────────────────────────────────────────────────────────────┐
│                         Pod Endpoints                                │
│                                                                      │
│  ┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐      │
│  │ sql-prod-01-0   │  │ sql-prod-01-1   │  │ sql-prod-01-2   │      │
│  │ (PRIMARY)       │  │ (SECONDARY)     │  │ (SECONDARY)     │      │
│  │ 10.0.1.10:1433  │  │ 10.0.1.11:1433  │  │ 10.0.1.12:1433  │      │
│  │                 │  │                 │  │                 │      │
│  │ Labels:         │  │ Labels:         │  │ Labels:         │      │
│  │ role: primary   │  │ role: secondary │  │ role: secondary │      │
│  └─────────────────┘  └─────────────────┘  └─────────────────┘      │
│                                                                      │
└─────────────────────────────────────────────────────────────────────┘
```

### Services Created

| Resource | Creates | Purpose |
|----------|---------|---------|
| SQLServer | Headless Service | Pod DNS names |
| SQLServer | Client Service (optional) | Direct pod access |
| SQLServerAG | Primary Service | Route to current primary |
| SQLServerAG | Secondary Service | Route to readable secondaries |

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
| `prod-ag-primary.mssql.svc.cluster.local` | Current primary pod IP |
| `prod-ag-secondary.mssql.svc.cluster.local` | Secondary pod IPs |

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
# Primary AG Service
apiVersion: v1
kind: Service
metadata:
  name: prod-ag-primary
spec:
  type: LoadBalancer
  ports:
    - name: sql
      port: 1433        # Service port (external)
      targetPort: 1433  # Container port
      protocol: TCP
  selector:
    mssql.microsoft.com/ag-role: primary
```

## Traffic Flow

### Read-Write Traffic (Primary)

```
┌──────────┐     ┌───────────────────┐     ┌───────────────┐
│  Client  │────▶│ prod-ag-primary   │────▶│ sql-prod-01-0 │
│          │     │ LoadBalancer      │     │ (PRIMARY)     │
│          │     │ 10.0.0.100:1433   │     │ 10.0.1.10:1433│
└──────────┘     └───────────────────┘     └───────────────┘
```

### Read-Only Traffic (Secondary)

```
┌──────────┐     ┌───────────────────┐     ┌───────────────┐
│  Client  │────▶│ prod-ag-secondary │────▶│ sql-prod-01-1 │
│          │     │ LoadBalancer      │     │ (SECONDARY)   │
│          │     │ 10.0.0.101:1434   │     │ 10.0.1.11:1433│
└──────────┘     └───────────────────┘     │               │
                                           │      OR       │
                                           │               │
                                           │ sql-prod-01-2 │
                                           │ (SECONDARY)   │
                                           │ 10.0.1.12:1433│
                                           └───────────────┘
```

### Failover Traffic Rerouting

When failover occurs:

```
Before Failover:                    After Failover:
─────────────────                   ────────────────

Primary Service ──▶ Pod 0           Primary Service ──▶ Pod 1
                   (PRIMARY)                            (PRIMARY)
                                    
Pod 0 labels:                       Pod 0 labels:
  role: primary                       role: secondary

Pod 1 labels:                       Pod 1 labels:
  role: secondary                     role: primary
```

The operator updates pod labels, causing Kubernetes to automatically reroute traffic.

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
  "1433": "mssql/prod-ag-primary:1433"
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
Server=prod-ag-primary.mssql.svc.cluster.local,1433;
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
