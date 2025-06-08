# Quick Start Guide

This guide will help you deploy your first LlamaStack instance using the Kubernetes operator in just a few minutes.

## Prerequisites

- LlamaStack Operator installed ([Installation Guide](installation.md))
- kubectl configured and connected to your cluster
- At least 4GB of available memory in your cluster

## Step 1: Create a Basic LlamaStack Instance

Create a file named `basic-llamastack.yaml`:

```yaml
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: my-first-llamastack
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
```

## Step 2: Deploy the Instance

Apply the configuration to your cluster:

```bash
kubectl apply -f basic-llamastack.yaml
```

## Step 3: Monitor the Deployment

Watch the deployment progress:

```bash
# Check the LlamaStackDistribution status
kubectl get llamastackdistribution my-first-llamastack

# Watch the pods being created
kubectl get pods -l app=llama-stack -w

# Check deployment status
kubectl get deployment my-first-llamastack
```

Expected output:
```
NAME                  READY   STATUS    RESTARTS   AGE
my-first-llamastack   1/1     Running   0          2m
```

## Step 4: Access Your LlamaStack Instance

### Port Forward (Development)

For development and testing, use port forwarding:

```bash
kubectl port-forward service/my-first-llamastack 8321:8321
```

Now you can access LlamaStack at `http://localhost:8321`.

### Service Exposure (Production)

For production access, expose the service:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: my-first-llamastack-external
spec:
  type: LoadBalancer
  selector:
    app: llama-stack
    llamastack.io/instance: my-first-llamastack
  ports:
  - port: 80
    targetPort: 8321
    protocol: TCP
```

Apply the service:
```bash
kubectl apply -f service.yaml
```

## Step 5: Test the API

Test that your LlamaStack instance is working:

```bash
# Health check
curl http://localhost:8321/health

# List available providers
curl http://localhost:8321/providers

# Get distribution info
curl http://localhost:8321/distribution/info
```

Expected response for health check:
```json
{
  "status": "healthy",
  "version": "0.0.1",
  "distribution": "meta-reference"
}
```

## Step 6: Explore the API

LlamaStack provides a comprehensive API for AI applications. Here are some key endpoints:

### Models API
```bash
# List available models
curl http://localhost:8321/models

# Get model info
curl http://localhost:8321/models/{model_id}
```

### Inference API
```bash
# Text completion
curl -X POST http://localhost:8321/inference/completion \
  -H "Content-Type: application/json" \
  -d '{
    "model": "meta-llama/Llama-2-7b-chat-hf",
    "prompt": "Hello, how are you?",
    "max_tokens": 100
  }'
```

### Memory API
```bash
# Create memory bank
curl -X POST http://localhost:8321/memory/create \
  -H "Content-Type: application/json" \
  -d '{
    "bank_id": "my-memory",
    "config": {
      "type": "vector",
      "embedding_model": "all-MiniLM-L6-v2"
    }
  }'
```

## Configuration Examples

### Custom Distribution

Use a different LlamaStack distribution:

```yaml
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: ollama-llamastack
spec:
  replicas: 1
  server:
    distribution:
      name: "ollama"
    containerSpec:
      port: 8321
      env:
      - name: OLLAMA_HOST
        value: "0.0.0.0"
```

### Custom Container Image

Use your own LlamaStack image:

```yaml
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: custom-llamastack
spec:
  replicas: 1
  server:
    distribution:
      image: "my-registry.com/llamastack:custom"
    containerSpec:
      port: 8321
```

### With Persistent Storage

Add persistent storage for models and data:

```yaml
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: persistent-llamastack
spec:
  replicas: 1
  server:
    distribution:
      name: "meta-reference"
    containerSpec:
      port: 8321
    storage:
      size: "50Gi"
      mountPath: "/.llama"
```

### High Availability Setup

Deploy multiple replicas with load balancing:

```yaml
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: ha-llamastack
spec:
  replicas: 3
  server:
    distribution:
      name: "meta-reference"
    containerSpec:
      port: 8321
      resources:
        requests:
          memory: "4Gi"
          cpu: "1"
        limits:
          memory: "8Gi"
          cpu: "2"
    storage:
      size: "100Gi"
      mountPath: "/.llama"
```

## Monitoring and Observability

### Check Resource Usage

Monitor resource consumption:

```bash
# Pod resource usage
kubectl top pods -l app=llama-stack

# Node resource usage
kubectl top nodes
```

### View Logs

Access application logs:

```bash
# View recent logs
kubectl logs deployment/my-first-llamastack

# Follow logs in real-time
kubectl logs -f deployment/my-first-llamastack

# View logs from all replicas
kubectl logs -l app=llama-stack --tail=100
```

### Health Checks

The operator automatically configures health checks:

```yaml
# Readiness probe
readinessProbe:
  httpGet:
    path: /health
    port: 8321
  initialDelaySeconds: 30
  periodSeconds: 10

# Liveness probe
livenessProbe:
  httpGet:
    path: /health
    port: 8321
  initialDelaySeconds: 60
  periodSeconds: 30
```

## Scaling

### Manual Scaling

Scale your deployment manually:

```bash
# Scale to 3 replicas
kubectl patch llamastackdistribution my-first-llamastack -p '{"spec":{"replicas":3}}'

# Verify scaling
kubectl get pods -l app=llama-stack
```

### Resource Updates

Update resource allocations:

```bash
kubectl patch llamastackdistribution my-first-llamastack -p '{
  "spec": {
    "server": {
      "containerSpec": {
        "resources": {
          "requests": {
            "memory": "4Gi",
            "cpu": "1"
          },
          "limits": {
            "memory": "8Gi",
            "cpu": "2"
          }
        }
      }
    }
  }
}'
```

## Cleanup

When you're done experimenting, clean up the resources:

```bash
# Delete the LlamaStack instance
kubectl delete llamastackdistribution my-first-llamastack

# Delete any additional services
kubectl delete service my-first-llamastack-external

# Verify cleanup
kubectl get pods -l app=llama-stack
```

## Troubleshooting

### Common Issues

**Pod not starting:**
```bash
# Check pod events
kubectl describe pod <pod-name>

# Check resource constraints
kubectl describe node <node-name>
```

**Service not accessible:**
```bash
# Check service endpoints
kubectl get endpoints my-first-llamastack

# Verify port forwarding
kubectl port-forward service/my-first-llamastack 8321:8321 --address 0.0.0.0
```

**API errors:**
```bash
# Check application logs
kubectl logs deployment/my-first-llamastack

# Verify configuration
kubectl get llamastackdistribution my-first-llamastack -o yaml
```

## Next Steps

Now that you have a working LlamaStack instance:

1. **[Learn about configuration options](configuration.md)** - Explore advanced configuration
2. **[Check out examples](../examples/basic-deployment.md)** - See real-world use cases
3. **[Read the API reference](../reference/api.md)** - Understand all available options
4. **[Set up monitoring](../how-to/monitoring.md)** - Add observability to your deployment

## Getting Help

If you encounter issues:

- Check the [troubleshooting guide](../how-to/troubleshooting.md)
- Review [GitHub issues](https://github.com/llamastack/llama-stack-k8s-operator/issues)
- Join the [community discussions](https://github.com/llamastack/llama-stack-k8s-operator/discussions)
