# Starter Distribution

The **Starter** distribution is the recommended default distribution for new users of the LlamaStack Kubernetes Operator. It provides a general-purpose LlamaStack deployment that's easy to set up and suitable for most use cases.

## Overview

The Starter distribution is designed to:

- **Get you started quickly** with minimal configuration
- **Provide a stable foundation** for LlamaStack applications
- **Support common use cases** out of the box
- **Serve as a learning platform** for understanding LlamaStack concepts

## Distribution Details

| Property | Value |
|----------|-------|
| **Distribution Name** | `starter` |
| **Image** | `docker.io/llamastack/distribution-starter:latest` |
| **Use Case** | General-purpose LlamaStack deployment |
| **Requirements** | Basic Kubernetes resources |
| **Recommended For** | New users, development, prototyping |

## Quick Start

### 1. Create a Basic Starter Distribution

```yaml
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: my-starter-llamastack
  namespace: default
spec:
  replicas: 1
  server:
    distribution:
      name: "starter"
    containerSpec:
      port: 8321
      resources:
        requests:
          memory: "2Gi"
          cpu: "500m"
        limits:
          memory: "4Gi"
          cpu: "1"
    storage:
      size: "20Gi"
```

### 2. Deploy the Distribution

```bash
kubectl apply -f starter-distribution.yaml
```

### 3. Verify Deployment

```bash
# Check the distribution status
kubectl get llamastackdistribution my-starter-llamastack

# Check the pods
kubectl get pods -l app=llama-stack

# Check the service
kubectl get svc my-starter-llamastack-service
```

## Configuration Options

### Basic Configuration

```yaml
spec:
  replicas: 1
  server:
    distribution:
      name: "starter"
    containerSpec:
      port: 8321
      resources:
        requests:
          memory: "2Gi"
          cpu: "500m"
        limits:
          memory: "4Gi"
          cpu: "1"
      env:
        - name: LOG_LEVEL
          value: "INFO"
    storage:
      size: "20Gi"
      mountPath: "/.llama"
```

### Environment Variables

Common environment variables for the Starter distribution:

```yaml
env:
  - name: LOG_LEVEL
    value: "INFO"  # DEBUG, INFO, WARNING, ERROR
  - name: SERVER_PORT
    value: "8321"
  - name: ENABLE_TELEMETRY
    value: "true"
```

### Resource Requirements

#### Development Setup
```yaml
resources:
  requests:
    memory: "1Gi"
    cpu: "250m"
  limits:
    memory: "2Gi"
    cpu: "500m"
```

#### Production Setup
```yaml
resources:
  requests:
    memory: "4Gi"
    cpu: "1"
  limits:
    memory: "8Gi"
    cpu: "2"
```

## Advanced Configuration

### Using ConfigMaps

You can provide custom configuration using ConfigMaps:

```yaml
spec:
  server:
    distribution:
      name: "starter"
    userConfig:
      configMapName: "my-llamastack-config"
      configMapNamespace: "default"  # Optional, defaults to same namespace
```

Create the ConfigMap:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: my-llamastack-config
  namespace: default
data:
  run.yaml: |
    built_with: llama-stack-0.0.53
    called_from: /tmp
    distribution:
      description: Built by `llama stack build` from `starter` template
      name: starter
      providers:
        agents: meta-reference
        inference: meta-reference
        memory: meta-reference
        safety: meta-reference
        telemetry: meta-reference
    image_name: starter
```

### Scaling

Scale your Starter distribution horizontally:

```yaml
spec:
  replicas: 3
  server:
    distribution:
      name: "starter"
    containerSpec:
      resources:
        requests:
          memory: "2Gi"
          cpu: "500m"
        limits:
          memory: "4Gi"
          cpu: "1"
```

### Custom Storage

Configure persistent storage for your data:

```yaml
spec:
  server:
    storage:
      size: "50Gi"
      mountPath: "/.llama"
```

## Use Cases

### 1. Learning and Development

Perfect for developers new to LlamaStack:

```yaml
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: learning-llamastack
  namespace: development
spec:
  replicas: 1
  server:
    distribution:
      name: "starter"
    containerSpec:
      resources:
        requests:
          memory: "1Gi"
          cpu: "250m"
        limits:
          memory: "2Gi"
          cpu: "500m"
      env:
        - name: LOG_LEVEL
          value: "DEBUG"
    storage:
      size: "10Gi"
```

### 2. Prototyping Applications

For building and testing LlamaStack applications:

```yaml
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: prototype-llamastack
  namespace: default
spec:
  replicas: 1
  server:
    distribution:
      name: "starter"
    containerSpec:
      resources:
        requests:
          memory: "2Gi"
          cpu: "500m"
        limits:
          memory: "4Gi"
          cpu: "1"
    storage:
      size: "20Gi"
```

### 3. Small Production Workloads

For lightweight production deployments:

```yaml
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: production-starter
  namespace: production
spec:
  replicas: 2
  server:
    distribution:
      name: "starter"
    containerSpec:
      resources:
        requests:
          memory: "4Gi"
          cpu: "1"
        limits:
          memory: "8Gi"
          cpu: "2"
      env:
        - name: LOG_LEVEL
          value: "WARNING"
        - name: ENABLE_TELEMETRY
          value: "true"
    storage:
      size: "100Gi"
```

## Monitoring and Troubleshooting

### Health Checks

Check the health of your Starter distribution:

```bash
# Check pod status
kubectl get pods -l app=llama-stack

# View logs
kubectl logs -l app=llama-stack

# Check service endpoints
kubectl get svc -l app=llama-stack
```

### Common Issues

1. **Pod Not Starting**
   - Check resource availability in your cluster
   - Verify image pull permissions
   - Review pod events: `kubectl describe pod <pod-name>`

2. **Service Not Accessible**
   - Verify service creation: `kubectl get svc`
   - Check port configuration
   - Ensure network policies allow traffic

3. **Storage Issues**
   - Verify PVC creation: `kubectl get pvc`
   - Check storage class availability
   - Ensure sufficient cluster storage

## Best Practices

### Resource Planning
- Start with minimal resources and scale up as needed
- Monitor resource usage with `kubectl top pods`
- Use resource requests to ensure scheduling

### Configuration Management
- Use ConfigMaps for complex configurations
- Store sensitive data in Secrets
- Version your configuration files

### Monitoring
- Enable telemetry for production deployments
- Set up log aggregation
- Monitor pod health and resource usage

## Next Steps

Once you're comfortable with the Starter distribution, consider:

1. **[Ollama Distribution](ollama.md)** - For local inference with Ollama
2. **[vLLM Distribution](vllm.md)** - For high-performance GPU inference
3. **[Bedrock Distribution](bedrock.md)** - For AWS Bedrock integration
4. **[Custom Images](bring-your-own.md)** - For specialized requirements

## API Reference

For complete API documentation, see:
- [API Reference](../reference/api.md)
- [Configuration Reference](../reference/configuration.md)
