# Basic Deployment Example

This example demonstrates a simple LlamaStack deployment suitable for development and testing environments.

## Overview

This configuration creates a single-replica LlamaStack instance using the ollama distribution with basic resource allocation.

## Configuration

```yaml
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: basic-llamastack
  namespace: default
  labels:
    app: llamastack
    environment: development
spec:
  replicas: 1
  server:
    distribution:
      name: "ollama"
    containerSpec:
      name: "llama-stack"
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
        value: "info"
      - name: INFERENCE_MODEL
        value: "meta-llama/Llama-3.2-3B-Instruct"
```

## Deployment Steps

1. **Save the configuration** to a file named `basic-deployment.yaml`

2. **Apply the configuration**:
   ```bash
   kubectl apply -f basic-deployment.yaml
   ```

3. **Verify the deployment**:
   ```bash
   kubectl get llamastackdistribution basic-llamastack
   kubectl get pods -l app=llama-stack
   ```

4. **Check the status**:
   ```bash
   kubectl describe llamastackdistribution basic-llamastack
   ```

## Expected Resources

This deployment will create:

- **Deployment**: `basic-llamastack` with 1 replica
- **Service**: `basic-llamastack` exposing port 8321
- **ConfigMap**: Configuration for the LlamaStack instance
- **Pod**: Single pod running the LlamaStack container

## Accessing the Service

### Port Forward (Development)

```bash
kubectl port-forward service/basic-llamastack 8321:8321
```

Access at: `http://localhost:8321`

### Service Exposure (Testing)

Create a NodePort service for external access:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: basic-llamastack-nodeport
spec:
  type: NodePort
  selector:
    app: llama-stack
    llamastack.io/instance: basic-llamastack
  ports:
  - port: 8321
    targetPort: 8321
    nodePort: 30321
    protocol: TCP
```

## Testing the Deployment

### Health Check

```bash
curl http://localhost:8321/health
```

Expected response:
```json
{
  "status": "healthy",
  "version": "0.0.1",
  "distribution": "meta-reference"
}
```

### API Endpoints

```bash
# List providers
curl http://localhost:8321/providers

# Get distribution info
curl http://localhost:8321/distribution/info

# List available models
curl http://localhost:8321/models
```

## Resource Usage

This basic deployment typically uses:

- **CPU**: 0.5-1 core
- **Memory**: 2-4 GB
- **Storage**: Ephemeral (no persistent storage)
- **Network**: Single service port (8321)

## Monitoring

### Pod Status

```bash
# Check pod status
kubectl get pods -l app=llama-stack

# View pod details
kubectl describe pod -l app=llama-stack

# Check resource usage
kubectl top pod -l app=llama-stack
```

### Logs

```bash
# View recent logs
kubectl logs deployment/basic-llamastack

# Follow logs in real-time
kubectl logs -f deployment/basic-llamastack

# View logs with timestamps
kubectl logs deployment/basic-llamastack --timestamps
```

## Scaling

### Manual Scaling

Scale the deployment to multiple replicas:

```bash
# Scale to 3 replicas
kubectl scale llamastackdistribution basic-llamastack --replicas=3

# Verify scaling
kubectl get pods -l app=llama-stack
```

### Resource Updates

Update resource allocations:

```yaml
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: basic-llamastack
spec:
  replicas: 1
  server:
    distribution:
      name: "meta-reference"
    containerSpec:
      port: 8321
      resources:
        requests:
          memory: "4Gi"  # Increased from 2Gi
          cpu: "1"       # Increased from 500m
        limits:
          memory: "8Gi"  # Increased from 4Gi
          cpu: "2"       # Increased from 1
```

Apply the update:
```bash
kubectl apply -f basic-deployment.yaml
```

## Troubleshooting

### Common Issues

**Pod not starting:**
```bash
# Check pod events
kubectl describe pod -l app=llama-stack

# Check resource constraints
kubectl describe node
```

**Service not accessible:**
```bash
# Check service endpoints
kubectl get endpoints basic-llamastack

# Verify service configuration
kubectl describe service basic-llamastack
```

**Application errors:**
```bash
# Check application logs
kubectl logs deployment/basic-llamastack --tail=50

# Check for configuration issues
kubectl get configmap -l app=llama-stack
```

### Debug Commands

```bash
# Get detailed resource information
kubectl get llamastackdistribution basic-llamastack -o yaml

# Check events in the namespace
kubectl get events --sort-by=.metadata.creationTimestamp

# Exec into the pod for debugging
kubectl exec -it deployment/basic-llamastack -- /bin/bash
```

## Cleanup

Remove the deployment:

```bash
# Delete the LlamaStack instance
kubectl delete llamastackdistribution basic-llamastack

# Verify cleanup
kubectl get pods -l app=llama-stack
kubectl get services -l app=llama-stack
```

## Next Steps

After successfully deploying this basic example:

1. **[Try the production setup](production-setup.md)** - Learn about production-ready configurations
2. **[Add persistent storage](../how-to/configure-storage.md)** - Configure persistent volumes
3. **[Set up monitoring](../how-to/monitoring.md)** - Add observability
4. **[Configure scaling](../how-to/scaling.md)** - Learn about auto-scaling

## Variations

### Different Distribution

Use the Ollama distribution instead:

```yaml
spec:
  server:
    distribution:
      name: "ollama"
    containerSpec:
      port: 8321
      env:
      - name: OLLAMA_HOST
        value: "0.0.0.0"
```

### Custom Environment Variables

Add custom configuration:

```yaml
spec:
  server:
    containerSpec:
      env:
      - name: LLAMASTACK_CONFIG_PATH
        value: "/config/llamastack.yaml"
      - name: MODEL_CACHE_DIR
        value: "/tmp/models"
      - name: MAX_CONCURRENT_REQUESTS
        value: "10"
```

### Resource Constraints

For resource-constrained environments:

```yaml
spec:
  server:
    containerSpec:
      resources:
        requests:
          memory: "1Gi"
          cpu: "250m"
        limits:
          memory: "2Gi"
          cpu: "500m"
