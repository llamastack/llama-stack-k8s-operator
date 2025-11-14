# Configuration

This guide covers how to configure the LlamaStack Kubernetes Operator for your environment.

## Basic Configuration

The operator can be configured through various methods:

### Environment Variables

Key environment variables for the operator:

```bash
# Operator configuration
OPERATOR_NAMESPACE=llamastack-system
LOG_LEVEL=info
METRICS_ADDR=:8080
```

### ConfigMaps

The operator uses ConfigMaps for distribution configurations:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: llamastack-config
  namespace: llamastack-system
data:
  config.yaml: |
    distributions:
      - name: default
        image: llamastack/llamastack:latest
```

## Advanced Configuration

### Resource Limits

Configure resource limits for LlamaStack distributions:

```yaml
spec:
  resources:
    limits:
      cpu: "2"
      memory: "4Gi"
    requests:
      cpu: "1"
      memory: "2Gi"
```

### Storage Configuration

Configure persistent storage for your distributions:

```yaml
spec:
  storage:
    size: "10Gi"
    storageClass: "fast-ssd"
```

## Next Steps

- [Quick Start Guide](quick-start.md)
- [API Reference](../reference/api.md)
- [Troubleshooting](../how-to/troubleshooting.md)
