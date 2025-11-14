# Deploy LlamaStack

This guide walks you through deploying a LlamaStack distribution using the Kubernetes operator.

## Prerequisites

- Kubernetes cluster (v1.20+)
- LlamaStack Kubernetes Operator installed
- `kubectl` configured to access your cluster

## Basic Deployment

### 1. Create a LlamaStackDistribution

Create a basic LlamaStack distribution:

```yaml
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: my-llamastack
  namespace: default
spec:
  image: llamastack/llamastack:latest
  replicas: 1
  resources:
    requests:
      cpu: "1"
      memory: "2Gi"
    limits:
      cpu: "2"
      memory: "4Gi"
```

### 2. Apply the Configuration

```bash
kubectl apply -f llamastack-distribution.yaml
```

### 3. Verify Deployment

Check the status of your deployment:

```bash
# Check the distribution
kubectl get llamastackdistribution my-llamastack

# Check the pods
kubectl get pods -l app=my-llamastack

# Check logs
kubectl logs -l app=my-llamastack
```

## Advanced Deployment Options

### With Persistent Storage

```yaml
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: llamastack-with-storage
spec:
  image: llamastack/llamastack:latest
  storage:
    size: "20Gi"
    storageClass: "fast-ssd"
  persistence:
    enabled: true
```

### With Custom Configuration

```yaml
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: llamastack-custom
spec:
  image: llamastack/llamastack:latest
  config:
    models:
      - name: "llama2-7b"
        path: "/models/llama2-7b"
    inference:
      provider: "meta-reference"
```

## Scaling

### Horizontal Scaling

Scale your deployment by adjusting replicas:

```bash
kubectl patch llamastackdistribution my-llamastack -p '{"spec":{"replicas":3}}'
```

### Vertical Scaling

Update resource limits:

```yaml
spec:
  resources:
    requests:
      cpu: "2"
      memory: "4Gi"
    limits:
      cpu: "4"
      memory: "8Gi"
```

## Monitoring

Monitor your deployment:

```bash
# Check resource usage
kubectl top pods -l app=my-llamastack

# Check events
kubectl get events --field-selector involvedObject.name=my-llamastack
```

## Next Steps

- [Configure Storage](configure-storage.md)
- [Set up Monitoring](monitoring.md)
- [Scaling Guide](scaling.md)
- [Troubleshooting](troubleshooting.md)
