# SQL Server AG with Monitoring

Deploy a 3-replica SQL Server Always On Availability Group with a full
**Prometheus + Grafana** monitoring stack on Kubernetes.

## What's Included

| Component | Description |
|-----------|-------------|
| **SQL Server** | 3-pod StatefulSet with HA-DR enabled |
| **sql-exporter** | Sidecar exporting SQL metrics on `:9399/metrics` |
| **AG Helper** | Sidecar managing AG health / failover |
| **Prometheus** | Auto-discovers sql-exporter targets via `kubernetes_sd_configs` |
| **Grafana** | Pre-loaded with 2 dashboards (AG Monitoring + SQL Server Overview) |

## Architecture

```
┌─────────────────────────────── mssql namespace ────────────────────────────────┐
│                                                                                │
│  sql-ag-0  (Primary)      sql-ag-1  (Secondary)    sql-ag-2  (Secondary)      │
│  ┌─────────────────────┐  ┌─────────────────────┐  ┌─────────────────────┐    │
│  │ mssql-server  :1433 │  │ mssql-server  :1433 │  │ mssql-server  :1433 │    │
│  │ sql-exporter  :9399 │  │ sql-exporter  :9399 │  │ sql-exporter  :9399 │    │
│  │ ag-helper     :5022 │  │ ag-helper     :5022 │  │ ag-helper     :5022 │    │
│  └────────┬────────────┘  └────────┬────────────┘  └────────┬────────────┘    │
│           │    :9399 /metrics      │    :9399 /metrics      │    :9399 /metrics│
└───────────┼────────────────────────┼────────────────────────┼─────────────────┘
            │                        │                        │
            └────────────────────────┼────────────────────────┘
                                     │  scrape targets
┌──────────────────────────── monitoring namespace ──────────────────────────────┐
│                                     ▼                                          │
│  ┌────────────────────┐    ┌────────────────────┐                              │
│  │    Prometheus       │    │     Grafana        │                              │
│  │    :9090            │◄───│     :3000          │                              │
│  │  auto-discovery via │    │  2 pre-built       │                              │
│  │  kubernetes_sd      │    │  dashboards        │                              │
│  └────────────────────┘    └────────────────────┘                              │
└────────────────────────────────────────────────────────────────────────────────┘
```

## Customization

> **Passwords:** The sample manifests and scripts ship with placeholder passwords (e.g. `YourStrong@Passw0rd!`). Before deploying, open `ag-deploy.yaml` and change the SA password and AG Helper password in the Secret resources. The Grafana default login (`admin`/`admin`) can also be changed in the Grafana Deployment environment variables in the same file. Then update the matching values at the top of `ag-configure.sh` (`SA_PASSWORD`, `AG_HELPER_PASSWORD`, `MASTER_KEY_PASSWORD`, `REPLICA_LOGIN_PASSWORD`), or in the manual T-SQL steps in `ag-configure.md`.

> **Instance and AG names:** You can rename the SQLServer resource (e.g. `sql-ag` → `my-sql`), the pod prefix, the AG name (`ProductionAG`), and the listener name. If you do, update them consistently in:
> - `ag-deploy.yaml` — SQLServer `.metadata.name`, SQLServerAG `.metadata.name`, `.spec.availabilityGroup.name`, `.spec.listener.name`, per-replica Service names, and the Prometheus scrape config job name / relabeling rules
> - `ag-configure.sh` — the `PRIMARY`, `REPLICAS`, `AG_NAME`, `AG_RESOURCE_NAME`, `AG_LISTENER_NAME`, and `DATABASE_NAME` variables at the top
> - `ag-configure.md` — the pod names and AG name referenced in every T-SQL command

## Quick Start

```bash
# 1. Apply the full stack (SQL Server + Prometheus + Grafana)
kubectl apply -f ag-deploy.yaml

# 2. Wait for SQL Server pods to be Ready
kubectl wait --for=condition=Ready pod/sql-ag-0 pod/sql-ag-1 pod/sql-ag-2 \
  -n mssql --timeout=180s

# 3. Configure the AG (T-SQL steps 1-8)
chmod +x ag-configure.sh
./ag-configure.sh all

# 4. Set up the AG Listener
./ag-configure.sh listener

# 5. Verify monitoring stack
./ag-configure.sh monitoring
```

See [ag-configure.md](ag-configure.md) for detailed T-SQL steps and
monitoring verification.

## Accessing Dashboards

### Grafana (port 3000)

```bash
kubectl port-forward svc/grafana -n monitoring 3000:3000
```

Open **http://localhost:3000** (login: `admin` / `admin`).

Two dashboards are pre-provisioned under the **SQL Server** folder:

| Dashboard | Panels |
|-----------|--------|
| **AG Monitoring** | AG Health, Sync State, Role Distribution, Redo Queue, Send Rate |
| **SQL Server Overview** | CPU %, Memory Usage, Batch Requests/sec, Connections, Buffer Cache Hit |

### Prometheus (port 9090)

```bash
kubectl port-forward svc/prometheus -n monitoring 9090:9090
```

Open **http://localhost:9090/targets** to confirm all sql-exporter
endpoints are `UP`.

## Useful PromQL Queries

| Query | Description |
|-------|-------------|
| `mssql_ag_synchronization_health` | AG sync health per replica |
| `mssql_ag_role` | Current role per replica (1 = Primary, 2 = Secondary) |
| `rate(mssql_batch_requests_total[5m])` | Batch requests per second |
| `mssql_connections` | Active SQL connections |
| `mssql_buffer_cache_hit_ratio` | Buffer pool hit ratio |

## Useful kubectl Commands

```bash
# AG status
kubectl get sqlserverag -n mssql

# Check sql-exporter metrics
kubectl exec sql-ag-0 -n mssql -c mssql -- \
  curl -s http://localhost:9399/metrics | grep mssql_ag_

# Manual failover (DR scenario)
kubectl exec sql-ag-1 -n mssql -c mssql -- /opt/mssql-tools18/bin/sqlcmd \
  -S localhost -U sa -P 'YourStrong@Passw0rd!' -C -Q \
  "ALTER AVAILABILITY GROUP [ProductionAG] FORCE_FAILOVER_ALLOW_DATA_LOSS;"

# Prometheus target health
kubectl exec -n monitoring deploy/prometheus -- \
  wget -qO- http://localhost:9090/api/v1/targets | python3 -m json.tool | head -30
```

## Troubleshooting

| Issue | Resolution |
|-------|-----------|
| Grafana shows "No data" | Confirm Prometheus data source URL is `http://prometheus:9090`, check Prometheus targets page |
| Prometheus target DOWN | Verify sql-exporter sidecar is running: `kubectl logs sql-ag-0 -n mssql -c sql-exporter` |
| AG metrics missing | AG must be fully configured (steps 1-8) before metrics appear |
| Grafana dashboards missing | Check ConfigMap `grafana-dashboards` and dashboard provider config are applied |

## Cleanup

```bash
# Remove SQL Server resources
kubectl delete -f ag-deploy.yaml

# Remove monitoring cluster-scoped resources
kubectl delete clusterrole prometheus-cluster-role
kubectl delete clusterrolebinding prometheus-cluster-role-binding

# Remove namespaces
kubectl delete namespace mssql monitoring
```
