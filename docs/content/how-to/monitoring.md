# Monitoring

Set up comprehensive monitoring for your LlamaStack distributions.

## Monitoring Overview

Monitor your LlamaStack deployments with:

- **Metrics**: Performance and resource usage
- **Logs**: Application and system logs
- **Alerts**: Proactive issue detection
- **Dashboards**: Visual monitoring

## Metrics Collection

### Prometheus Setup

Deploy Prometheus for metrics collection:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: prometheus-config
data:
  prometheus.yml: |
    global:
      scrape_interval: 15s
    scrape_configs:
    - job_name: 'llamastack'
      kubernetes_sd_configs:
      - role: pod
      relabel_configs:
      - source_labels: [__meta_kubernetes_pod_label_app]
        action: keep
        regex: llamastack
```

### ServiceMonitor

Create a ServiceMonitor for automatic discovery:

```yaml
apiVersion: monitoring.coreos.com/v1
kind: ServiceMonitor
metadata:
  name: llamastack-monitor
spec:
  selector:
    matchLabels:
      app: llamastack
  endpoints:
  - port: metrics
    interval: 30s
    path: /metrics
```

## Key Metrics

### Application Metrics

Monitor LlamaStack-specific metrics:

```yaml
# Custom metrics exposed by LlamaStack
llamastack_requests_total
llamastack_request_duration_seconds
llamastack_active_connections
llamastack_model_load_time_seconds
llamastack_inference_latency_seconds
```

### Resource Metrics

Track resource usage:

```yaml
# CPU and Memory
container_cpu_usage_seconds_total
container_memory_usage_bytes
container_memory_working_set_bytes

# Network
container_network_receive_bytes_total
container_network_transmit_bytes_total

# Storage
kubelet_volume_stats_used_bytes
kubelet_volume_stats_capacity_bytes
```

## Logging

### Centralized Logging

Set up log aggregation with Fluentd:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: fluentd-config
data:
  fluent.conf: |
    <source>
      @type tail
      path /var/log/containers/*llamastack*.log
      pos_file /var/log/fluentd-containers.log.pos
      tag kubernetes.*
      format json
    </source>
    
    <match kubernetes.**>
      @type elasticsearch
      host elasticsearch.logging.svc.cluster.local
      port 9200
      index_name llamastack-logs
    </match>
```

### Log Levels

Configure appropriate log levels:

```yaml
spec:
  env:
  - name: LOG_LEVEL
    value: "info"  # debug, info, warn, error
  - name: LOG_FORMAT
    value: "json"  # json, text
```

## Dashboards

### Grafana Dashboard

Create a comprehensive dashboard:

```json
{
  "dashboard": {
    "title": "LlamaStack Monitoring",
    "panels": [
      {
        "title": "Request Rate",
        "type": "graph",
        "targets": [
          {
            "expr": "rate(llamastack_requests_total[5m])",
            "legendFormat": "{{instance}}"
          }
        ]
      },
      {
        "title": "Response Time",
        "type": "graph",
        "targets": [
          {
            "expr": "histogram_quantile(0.95, rate(llamastack_request_duration_seconds_bucket[5m]))",
            "legendFormat": "95th percentile"
          }
        ]
      },
      {
        "title": "Resource Usage",
        "type": "graph",
        "targets": [
          {
            "expr": "rate(container_cpu_usage_seconds_total[5m])",
            "legendFormat": "CPU"
          },
          {
            "expr": "container_memory_usage_bytes",
            "legendFormat": "Memory"
          }
        ]
      }
    ]
  }
}
```

## Alerting

### Prometheus Alerts

Define critical alerts:

```yaml
groups:
- name: llamastack.rules
  rules:
  - alert: LlamaStackDown
    expr: up{job="llamastack"} == 0
    for: 1m
    labels:
      severity: critical
    annotations:
      summary: "LlamaStack instance is down"
      description: "LlamaStack instance {{ $labels.instance }} has been down for more than 1 minute."

  - alert: HighErrorRate
    expr: rate(llamastack_requests_total{status=~"5.."}[5m]) > 0.1
    for: 5m
    labels:
      severity: warning
    annotations:
      summary: "High error rate detected"
      description: "Error rate is {{ $value }} errors per second."

  - alert: HighLatency
    expr: histogram_quantile(0.95, rate(llamastack_request_duration_seconds_bucket[5m])) > 2
    for: 5m
    labels:
      severity: warning
    annotations:
      summary: "High latency detected"
      description: "95th percentile latency is {{ $value }} seconds."

  - alert: HighMemoryUsage
    expr: container_memory_usage_bytes / container_spec_memory_limit_bytes > 0.9
    for: 5m
    labels:
      severity: warning
    annotations:
      summary: "High memory usage"
      description: "Memory usage is above 90%."
```

### AlertManager Configuration

Configure alert routing:

```yaml
global:
  smtp_smarthost: 'localhost:587'
  smtp_from: 'alerts@example.com'

route:
  group_by: ['alertname']
  group_wait: 10s
  group_interval: 10s
  repeat_interval: 1h
  receiver: 'web.hook'

receivers:
- name: 'web.hook'
  email_configs:
  - to: 'admin@example.com'
    subject: 'LlamaStack Alert: {{ .GroupLabels.alertname }}'
    body: |
      {{ range .Alerts }}
      Alert: {{ .Annotations.summary }}
      Description: {{ .Annotations.description }}
      {{ end }}
```

## Health Checks

### Liveness Probe

Configure liveness probes:

```yaml
spec:
  containers:
  - name: llamastack
    livenessProbe:
      httpGet:
        path: /health
        port: 8080
      initialDelaySeconds: 30
      periodSeconds: 10
      timeoutSeconds: 5
      failureThreshold: 3
```

### Readiness Probe

Configure readiness probes:

```yaml
spec:
  containers:
  - name: llamastack
    readinessProbe:
      httpGet:
        path: /ready
        port: 8080
      initialDelaySeconds: 5
      periodSeconds: 5
      timeoutSeconds: 3
      failureThreshold: 3
```

## Performance Monitoring

### Custom Metrics

Expose custom application metrics:

```python
# Example Python code for custom metrics
from prometheus_client import Counter, Histogram, Gauge

REQUEST_COUNT = Counter('llamastack_requests_total', 'Total requests', ['method', 'endpoint'])
REQUEST_LATENCY = Histogram('llamastack_request_duration_seconds', 'Request latency')
ACTIVE_CONNECTIONS = Gauge('llamastack_active_connections', 'Active connections')

# In your application code
REQUEST_COUNT.labels(method='POST', endpoint='/inference').inc()
REQUEST_LATENCY.observe(response_time)
ACTIVE_CONNECTIONS.set(current_connections)
```

### Distributed Tracing

Set up distributed tracing with Jaeger:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: jaeger-config
data:
  config.yaml: |
    jaeger:
      endpoint: "http://jaeger-collector:14268/api/traces"
      service_name: "llamastack"
      sampler:
        type: "probabilistic"
        param: 0.1
```

## Monitoring Best Practices

### Resource Monitoring

Monitor these key resources:

```bash
# CPU usage
kubectl top pods -l app=llamastack

# Memory usage
kubectl top pods -l app=llamastack --containers

# Storage usage
kubectl exec -it <pod> -- df -h

# Network usage
kubectl exec -it <pod> -- netstat -i
```

### Log Analysis

Analyze logs for issues:

```bash
# Check error logs
kubectl logs -l app=llamastack | grep ERROR

# Check recent logs
kubectl logs -l app=llamastack --since=1h

# Follow logs in real-time
kubectl logs -f -l app=llamastack
```

## Troubleshooting Monitoring

### Common Issues

**Metrics Not Appearing:**
```bash
# Check ServiceMonitor
kubectl get servicemonitor

# Check Prometheus targets
kubectl port-forward svc/prometheus 9090:9090
# Visit http://localhost:9090/targets
```

**High Resource Usage:**
```bash
# Check resource limits
kubectl describe pod <pod-name>

# Check node resources
kubectl describe node <node-name>
```

**Alert Fatigue:**
```bash
# Review alert thresholds
kubectl get prometheusrule

# Check alert history
kubectl logs -l app=alertmanager
```

## Next Steps

- [Troubleshooting Guide](troubleshooting.md)
- [Scaling Guide](scaling.md)
- [Configure Storage](configure-storage.md)
