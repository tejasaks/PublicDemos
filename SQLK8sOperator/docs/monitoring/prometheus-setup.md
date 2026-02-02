# Prometheus Setup

[← Back to Monitoring](overview.md) | [Documentation Home](../README.md)

Step-by-step guide to setting up Prometheus for SQL Server monitoring.

## Table of Contents

- [Prerequisites](#prerequisites)
- [Installation Options](#installation-options)
- [Using Prometheus Operator](#using-prometheus-operator)
- [ServiceMonitor Configuration](#servicemonitor-configuration)
- [PrometheusRule for Alerts](#prometheusrule-for-alerts)
- [Verification](#verification)
- [Production Configuration](#production-configuration)

## Prerequisites

| Requirement | How to Check |
|-------------|--------------|
| Kubernetes 1.21+ | `kubectl version` |
| Helm 3.x | `helm version` |
| SQL Operator deployed | `kubectl get pods -n mssql-system` |
| Monitoring enabled | SQLServer spec has `monitoring.enabled: true` |

## Installation Options

### Option 1: kube-prometheus-stack (Recommended)

Full-featured Prometheus + Grafana + Alertmanager stack:

```bash
helm repo add prometheus-community https://prometheus-community.github.io/helm-charts
helm repo update

helm install prometheus prometheus-community/kube-prometheus-stack \
  --namespace monitoring \
  --create-namespace \
  --set prometheus.prometheusSpec.serviceMonitorSelectorNilUsesHelmValues=false \
  --set prometheus.prometheusSpec.podMonitorSelectorNilUsesHelmValues=false
```

### Option 2: Prometheus Operator Only

Just the operator, no pre-configured stack:

```bash
helm install prometheus-operator prometheus-community/prometheus-operator-crds \
  --namespace monitoring \
  --create-namespace
```

### Option 3: Standalone Prometheus

No operator, manual configuration. This option requires creating manifest files and applying them to your cluster.

**Step 1: Create the namespace**

```bash
kubectl create namespace monitoring
```

**Step 2: Create the Prometheus configuration file**

Create a file named `prometheus-config.yaml` using your preferred editor:

```bash
# Using nano (Linux/macOS)
nano prometheus-config.yaml

# Using notepad (Windows)
notepad prometheus-config.yaml
```

Paste the following content and save:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: prometheus-config
  namespace: monitoring
data:
  prometheus.yml: |
    global:
      scrape_interval: 15s
    scrape_configs:
      - job_name: 'mssql-exporter'
        kubernetes_sd_configs:
          - role: endpoints
            namespaces:
              names:
                - mssql
        relabel_configs:
          - source_labels: [__meta_kubernetes_service_label_mssql_microsoft_com_monitored]
            action: keep
            regex: "true"
```

Apply the configuration:

```bash
kubectl apply -f prometheus-config.yaml
```

**Expected output:**
```
configmap/prometheus-config created
```

**Step 3: Create the Prometheus deployment file**

Create a file named `prometheus-deployment.yaml`:

```bash
nano prometheus-deployment.yaml
```

Paste the following content and save:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: prometheus
  namespace: monitoring
spec:
  replicas: 1
  selector:
    matchLabels:
      app: prometheus
  template:
    metadata:
      labels:
        app: prometheus
    spec:
      containers:
        - name: prometheus
          image: prom/prometheus:v2.48.0
          args:
            - --config.file=/etc/prometheus/prometheus.yml
            - --storage.tsdb.path=/prometheus
            - --storage.tsdb.retention.time=15d
          ports:
            - containerPort: 9090
          volumeMounts:
            - name: config
              mountPath: /etc/prometheus
            - name: storage
              mountPath: /prometheus
      volumes:
        - name: config
          configMap:
            name: prometheus-config
        - name: storage
          emptyDir: {}
---
apiVersion: v1
kind: Service
metadata:
  name: prometheus
  namespace: monitoring
spec:
  selector:
    app: prometheus
  ports:
    - port: 9090
      targetPort: 9090
```

Apply the deployment:

```bash
kubectl apply -f prometheus-deployment.yaml
```

**Expected output:**
```
deployment.apps/prometheus created
service/prometheus created
```

**Step 4: Verify the deployment**

```bash
kubectl get pods -n monitoring

# Expected output (wait until STATUS is Running):
# NAME                          READY   STATUS    RESTARTS   AGE
# prometheus-xxxxxxxxx-xxxxx    1/1     Running   0          30s
```

Access Prometheus UI:

```bash
kubectl port-forward svc/prometheus -n monitoring 9090:9090
```

Open http://localhost:9090 in your browser to access the Prometheus UI.

## Using Prometheus Operator

The Prometheus Operator uses CRDs to configure Prometheus:

| CRD | Purpose |
|-----|---------|
| `ServiceMonitor` | Auto-discover services to scrape |
| `PodMonitor` | Auto-discover pods to scrape |
| `PrometheusRule` | Alert rules |
| `Prometheus` | Prometheus instance configuration |

## ServiceMonitor Configuration

Create a ServiceMonitor to scrape SQL Exporter metrics. The ServiceMonitor tells Prometheus which services to scrape.

### Basic ServiceMonitor

**Step 1: Create the ServiceMonitor file**

Create a file named `mssql-servicemonitor.yaml`:

```bash
nano mssql-servicemonitor.yaml
```

Paste the following content and save:

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: mssql-exporter
  namespace: monitoring
  labels:
    release: prometheus  # Match your Prometheus installation
spec:
  # Which namespaces to watch
  namespaceSelector:
    matchNames:
      - mssql
  
  # Which services to monitor
  selector:
    matchLabels:
      mssql.microsoft.com/monitored: "true"
  
  # How to scrape
  endpoints:
    - port: metrics
      interval: 15s
      path: /metrics
      scheme: http
```

**Step 2: Apply the ServiceMonitor**

```bash
kubectl apply -f mssql-servicemonitor.yaml
```

**Expected output:**
```
servicemonitor.monitoring.coreos.com/mssql-exporter created
```

**Step 3: Verify the ServiceMonitor was created**

```bash
kubectl get servicemonitor -n monitoring

# Expected output:
# NAME             AGE
# mssql-exporter   5s
```

### Alternative: Apply Inline

You can also apply the ServiceMonitor directly without creating a file:

```bash
kubectl apply -f - <<EOF
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: mssql-exporter
  namespace: monitoring
spec:
  namespaceSelector:
    matchNames:
      - mssql
  selector:
    matchLabels:
      mssql.microsoft.com/monitored: "true"
  endpoints:
    - port: metrics
      interval: 15s
      path: /metrics
EOF
```

This inline method is convenient for quick testing but using a file is recommended for production as you can version control it.

### Cross-Namespace Monitoring

To monitor SQL Server in any namespace, modify the `namespaceSelector` in your ServiceMonitor file:

```yaml
spec:
  namespaceSelector:
    any: true  # Watch all namespaces
```

## PrometheusRule for Alerts

Create alerting rules for SQL Server to get notified about issues.

**Step 1: Create the PrometheusRule file**

Create a file named `mssql-prometheus-rules.yaml`:

```bash
nano mssql-prometheus-rules.yaml
```

Paste the following content and save:

```yaml
apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  name: mssql-alerts
  namespace: monitoring
  labels:
    release: prometheus
spec:
  groups:
    - name: mssql-instance
      interval: 30s
      rules:
        - alert: SQLServerDown
          expr: mssql_up == 0
          for: 1m
          labels:
            severity: critical
          annotations:
            summary: "SQL Server instance is down"
            description: "{{ $labels.instance }} has been down for more than 1 minute."
        
        - alert: SQLServerHighMemory
          expr: mssql_server_memory_used_bytes / mssql_server_memory_total_bytes * 100 > 90
          for: 5m
          labels:
            severity: warning
          annotations:
            summary: "SQL Server high memory usage"
            description: "{{ $labels.instance }} memory usage is above 90%."
        
        - alert: SQLServerHighConnections
          expr: mssql_connections > 200
          for: 5m
          labels:
            severity: warning
          annotations:
            summary: "SQL Server high connection count"
            description: "{{ $labels.instance }} has {{ $value }} connections."
    
    - name: mssql-availability-groups
      interval: 30s
      rules:
        - alert: AGDatabaseNotSynchronized
          expr: mssql_ag_database_synchronization_state != 2
          for: 5m
          labels:
            severity: critical
          annotations:
            summary: "AG database not synchronized"
            description: "Database {{ $labels.database }} in AG {{ $labels.ag_name }} is not synchronized."
        
        - alert: AGReplicaNotConnected
          expr: mssql_ag_replica_connected_state != 1
          for: 2m
          labels:
            severity: critical
          annotations:
            summary: "AG replica not connected"
            description: "Replica {{ $labels.replica }} in AG {{ $labels.ag_name }} is not connected."
        
        - alert: AGFailover
          expr: changes(mssql_ag_is_primary[5m]) > 0
          labels:
            severity: info
          annotations:
            summary: "AG failover detected"
            description: "AG {{ $labels.ag_name }} primary changed in the last 5 minutes."
```

**Step 2: Apply the alert rules**

```bash
kubectl apply -f mssql-prometheus-rules.yaml
```

**Expected output:**
```
prometheusrule.monitoring.coreos.com/mssql-alerts created
```

**Step 3: Verify the rules are loaded**

```bash
kubectl get prometheusrule -n monitoring

# Expected output:
# NAME           AGE
# mssql-alerts   5s
```

You can also verify in Prometheus UI by navigating to Status → Rules.

## Verification

### Check ServiceMonitor

```bash
kubectl get servicemonitor -n monitoring

# Should show:
# NAME             AGE
# mssql-exporter   5m
```

### Check Prometheus Targets

```bash
# Port-forward to Prometheus
kubectl port-forward svc/prometheus-operated -n monitoring 9090:9090

# Open targets page
open http://localhost:9090/targets
```

You should see targets like:
- `monitoring/mssql-exporter/0` - UP

### Test Metrics Query

```bash
# In Prometheus UI or via API
curl 'http://localhost:9090/api/v1/query?query=mssql_up'

# Response
{
  "status": "success",
  "data": {
    "resultType": "vector",
    "result": [
      {
        "metric": {"instance": "sql-prod-0:9399", "job": "mssql-exporter"},
        "value": [1705320000, "1"]
      }
    ]
  }
}
```

### Check Alert Rules

```bash
# Port-forward and check rules
open http://localhost:9090/rules

# Check alerts (if any firing)
open http://localhost:9090/alerts
```

## Production Configuration

For production environments, you'll want to configure proper resource limits, storage, and alerting.

### Prometheus Resource Limits

**Step 1: Create the Prometheus custom resource file**

Create a file named `prometheus-production.yaml`:

```bash
nano prometheus-production.yaml
```

Paste the following content and save:

```yaml
apiVersion: monitoring.coreos.com/v1
kind: Prometheus
metadata:
  name: prometheus
  namespace: monitoring
spec:
  replicas: 2  # HA
  retention: 30d
  resources:
    requests:
      cpu: 500m
      memory: 2Gi
    limits:
      cpu: 2
      memory: 4Gi
  storage:
    volumeClaimTemplate:
      spec:
        storageClassName: standard
        resources:
          requests:
            storage: 100Gi
```

**Step 2: Apply the configuration**

```bash
kubectl apply -f prometheus-production.yaml
```

**Expected output:**
```
prometheus.monitoring.coreos.com/prometheus created
```

**Step 3: Verify Prometheus pods are running**

```bash
kubectl get pods -n monitoring -l app.kubernetes.io/name=prometheus

# Expected output (2 replicas for HA):
# NAME                      READY   STATUS    RESTARTS   AGE
# prometheus-prometheus-0   2/2     Running   0          1m
# prometheus-prometheus-1   2/2     Running   0          1m
```

### Remote Write (Long-term Storage)

To send metrics to a long-term storage like Thanos, add the following to your Prometheus spec:

```yaml
spec:
  remoteWrite:
    - url: "https://thanos-receive.example.com/api/v1/receive"
      basicAuth:
        username:
          name: thanos-creds
          key: username
        password:
          name: thanos-creds
          key: password
```

### Alertmanager Configuration

**Step 1: Create the Alertmanager configuration secret**

Create a file named `alertmanager-config.yaml`:

```bash
nano alertmanager-config.yaml
```

Paste the following content (replace webhook URLs with your actual values):

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: alertmanager-config
  namespace: monitoring
stringData:
  alertmanager.yaml: |
    global:
      resolve_timeout: 5m
    route:
      receiver: 'default'
      group_by: ['alertname', 'severity']
      group_wait: 30s
      group_interval: 5m
      repeat_interval: 4h
      routes:
        - match:
            severity: critical
          receiver: 'pagerduty'
    receivers:
      - name: 'default'
        slack_configs:
          - api_url: 'https://hooks.slack.com/services/YOUR/SLACK/WEBHOOK'
            channel: '#sql-alerts'
      - name: 'pagerduty'
        pagerduty_configs:
          - service_key: 'YOUR_PAGERDUTY_SERVICE_KEY'
```

**Step 2: Create the Alertmanager deployment**

Create a file named `alertmanager.yaml`:

```bash
nano alertmanager.yaml
```

Paste the following content:

```yaml
apiVersion: monitoring.coreos.com/v1
kind: Alertmanager
metadata:
  name: alertmanager
  namespace: monitoring
spec:
  replicas: 3
  configSecret: alertmanager-config
```

**Step 3: Apply both configurations**

```bash
# First apply the secret
kubectl apply -f alertmanager-config.yaml

# Expected output:
# secret/alertmanager-config created

# Then apply Alertmanager
kubectl apply -f alertmanager.yaml

# Expected output:
# alertmanager.monitoring.coreos.com/alertmanager created
```

**Step 4: Verify Alertmanager is running**

```bash
kubectl get pods -n monitoring -l app.kubernetes.io/name=alertmanager

# Expected output (3 replicas for HA):
# NAME                       READY   STATUS    RESTARTS   AGE
# alertmanager-alertmanager-0   2/2     Running   0          1m
# alertmanager-alertmanager-1   2/2     Running   0          1m
# alertmanager-alertmanager-2   2/2     Running   0          1m
```

**Step 5: Access Alertmanager UI**

```bash
kubectl port-forward svc/alertmanager-operated -n monitoring 9093:9093

# Open in browser:
# http://localhost:9093
```

## Next Steps

- [Grafana Dashboards](grafana-dashboards.md) - Visualization
- [SQL Exporter Reference](sql-exporter-reference.md) - All metrics
- [Alerting](grafana-dashboards.md#alerting) - Alert configuration
