# Text Generation Inference (TGI) Distribution

!!! warning "Distribution Availability"
    The TGI distribution container image may not be currently maintained or available.
    Please verify the image exists at `docker.io/llamastack/distribution-tgi:latest` before using this distribution.
    For production use, consider using the `ollama` or `vllm` distributions which are actively maintained.

The **TGI** distribution integrates with Hugging Face's Text Generation Inference (TGI) server, providing high-performance inference for large language models with optimized serving capabilities.

## Overview

Text Generation Inference (TGI) is Hugging Face's solution for deploying and serving Large Language Models. The TGI distribution:

- **Connects to TGI servers** for optimized model inference
- **Supports streaming responses** for real-time applications
- **Provides high throughput** with batching and optimization
- **Compatible with Hugging Face models** from the Hub

## Distribution Details

| Property | Value |
|----------|-------|
| **Distribution Name** | `tgi` |
| **Image** | `docker.io/llamastack/distribution-tgi:latest` |
| **Use Case** | Hugging Face TGI server integration |
| **Requirements** | TGI server endpoint |
| **Recommended For** | High-performance inference, Hugging Face ecosystem |

## Prerequisites

### 1. TGI Server Setup

You need a running TGI server. You can deploy one using:

#### Option A: Deploy TGI in Kubernetes
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: tgi-server
spec:
  replicas: 1
  selector:
    matchLabels:
      app: tgi-server
  template:
    metadata:
      labels:
        app: tgi-server
    spec:
      containers:
      - name: tgi
        image: ghcr.io/huggingface/text-generation-inference:latest
        ports:
        - containerPort: 80
        env:
        - name: MODEL_ID
          value: "microsoft/DialoGPT-medium"
        - name: PORT
          value: "80"
        resources:
          requests:
            memory: "4Gi"
            cpu: "2"
          limits:
            memory: "8Gi"
            cpu: "4"
---
apiVersion: v1
kind: Service
metadata:
  name: tgi-server-service
spec:
  selector:
    app: tgi-server
  ports:
  - port: 80
    targetPort: 80
```

#### Option B: External TGI Server
Use an existing TGI deployment (cloud or on-premises).

## Quick Start

### 1. Create TGI Distribution

```yaml
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: my-tgi-llamastack
  namespace: default
spec:
  replicas: 1
  server:
    distribution:
      name: "tgi"
    containerSpec:
      port: 8321
      resources:
        requests:
          memory: "1Gi"
          cpu: "500m"
        limits:
          memory: "2Gi"
          cpu: "1"
      env:
        - name: TGI_URL
          value: "http://tgi-server-service:80"
        - name: MODEL_ID
          value: "microsoft/DialoGPT-medium"
    storage:
      size: "10Gi"
```

### 2. Deploy the Distribution

```bash
kubectl apply -f tgi-distribution.yaml
```

### 3. Verify Deployment

```bash
# Check the distribution status
kubectl get llamastackdistribution my-tgi-llamastack

# Check the pods
kubectl get pods -l app=llama-stack

# Test TGI connectivity
kubectl logs -l app=llama-stack
```

## Configuration Options

### Environment Variables

Configure TGI connection and behavior:

```yaml
env:
  - name: TGI_URL
    value: "http://tgi-server-service:80"
  - name: MODEL_ID
    value: "microsoft/DialoGPT-medium"
  - name: TGI_TIMEOUT
    value: "30"  # Request timeout in seconds
  - name: TGI_MAX_TOKENS
    value: "512"
  - name: TGI_TEMPERATURE
    value: "0.7"
  - name: TGI_TOP_P
    value: "0.9"
  - name: LOG_LEVEL
    value: "INFO"
```

### TGI Server Configuration

Common TGI server models and configurations:

#### Small Models (Development)
```yaml
env:
  - name: TGI_URL
    value: "http://tgi-server:80"
  - name: MODEL_ID
    value: "microsoft/DialoGPT-small"  # ~117M parameters
  - name: TGI_MAX_TOKENS
    value: "256"
```

#### Medium Models (Production)
```yaml
env:
  - name: TGI_URL
    value: "http://tgi-server:80"
  - name: MODEL_ID
    value: "microsoft/DialoGPT-medium"  # ~345M parameters
  - name: TGI_MAX_TOKENS
    value: "512"
```

#### Large Models (High Performance)
```yaml
env:
  - name: TGI_URL
    value: "http://tgi-server:80"
  - name: MODEL_ID
    value: "microsoft/DialoGPT-large"  # ~762M parameters
  - name: TGI_MAX_TOKENS
    value: "1024"
```

### Resource Requirements

#### Lightweight Setup
```yaml
resources:
  requests:
    memory: "512Mi"
    cpu: "250m"
  limits:
    memory: "1Gi"
    cpu: "500m"
```

#### Standard Setup
```yaml
resources:
  requests:
    memory: "1Gi"
    cpu: "500m"
  limits:
    memory: "2Gi"
    cpu: "1"
```

#### High-Performance Setup
```yaml
resources:
  requests:
    memory: "2Gi"
    cpu: "1"
  limits:
    memory: "4Gi"
    cpu: "2"
```

## Advanced Configuration

### Multiple TGI Servers

Connect to multiple TGI servers for load balancing:

```yaml
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: multi-tgi-llamastack
spec:
  replicas: 2
  server:
    distribution:
      name: "tgi"
    containerSpec:
      env:
        - name: TGI_URLS
          value: "http://tgi-server-1:80,http://tgi-server-2:80"
        - name: TGI_LOAD_BALANCE
          value: "round_robin"  # round_robin, random, least_connections
```

### TGI with GPU Support

For GPU-accelerated TGI servers:

```yaml
# TGI Server with GPU
apiVersion: apps/v1
kind: Deployment
metadata:
  name: tgi-gpu-server
spec:
  template:
    spec:
      containers:
      - name: tgi
        image: ghcr.io/huggingface/text-generation-inference:latest
        env:
        - name: MODEL_ID
          value: "meta-llama/Llama-2-7b-chat-hf"
        - name: CUDA_VISIBLE_DEVICES
          value: "0"
        resources:
          requests:
            nvidia.com/gpu: "1"
            memory: "16Gi"
          limits:
            nvidia.com/gpu: "1"
            memory: "32Gi"
```

### Custom TGI Configuration

Use ConfigMaps for complex TGI configurations:

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: tgi-config
data:
  tgi-settings.json: |
    {
      "max_concurrent_requests": 128,
      "max_best_of": 2,
      "max_stop_sequences": 4,
      "max_input_length": 1024,
      "max_total_tokens": 2048,
      "waiting_served_ratio": 1.2,
      "max_batch_prefill_tokens": 4096,
      "max_batch_total_tokens": 8192
    }
---
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: custom-tgi-llamastack
spec:
  server:
    distribution:
      name: "tgi"
    containerSpec:
      env:
        - name: TGI_CONFIG_FILE
          value: "/config/tgi-settings.json"
    podOverrides:
      volumes:
        - name: tgi-config
          configMap:
            name: tgi-config
      volumeMounts:
        - name: tgi-config
          mountPath: /config
```

## Use Cases

### 1. Development Environment

```yaml
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: dev-tgi
  namespace: development
spec:
  replicas: 1
  server:
    distribution:
      name: "tgi"
    containerSpec:
      resources:
        requests:
          memory: "512Mi"
          cpu: "250m"
        limits:
          memory: "1Gi"
          cpu: "500m"
      env:
        - name: TGI_URL
          value: "http://tgi-dev-server:80"
        - name: MODEL_ID
          value: "microsoft/DialoGPT-small"
        - name: LOG_LEVEL
          value: "DEBUG"
    storage:
      size: "5Gi"
```

### 2. Production Deployment

```yaml
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: production-tgi
  namespace: production
spec:
  replicas: 3
  server:
    distribution:
      name: "tgi"
    containerSpec:
      resources:
        requests:
          memory: "2Gi"
          cpu: "1"
        limits:
          memory: "4Gi"
          cpu: "2"
      env:
        - name: TGI_URL
          value: "http://tgi-prod-server:80"
        - name: MODEL_ID
          value: "meta-llama/Llama-2-7b-chat-hf"
        - name: TGI_MAX_TOKENS
          value: "1024"
        - name: TGI_TEMPERATURE
          value: "0.7"
        - name: ENABLE_TELEMETRY
          value: "true"
    storage:
      size: "50Gi"
```

### 3. High-Throughput Setup

```yaml
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: high-throughput-tgi
  namespace: production
spec:
  replicas: 5
  server:
    distribution:
      name: "tgi"
    containerSpec:
      resources:
        requests:
          memory: "4Gi"
          cpu: "2"
        limits:
          memory: "8Gi"
          cpu: "4"
      env:
        - name: TGI_URLS
          value: "http://tgi-1:80,http://tgi-2:80,http://tgi-3:80"
        - name: TGI_LOAD_BALANCE
          value: "least_connections"
        - name: TGI_TIMEOUT
          value: "60"
        - name: TGI_MAX_CONCURRENT_REQUESTS
          value: "256"
```

## Monitoring and Troubleshooting

### Health Checks

```bash
# Check distribution status
kubectl get llamastackdistribution

# Check TGI connectivity
kubectl logs -l app=llama-stack | grep -i tgi

# Test TGI server directly
kubectl exec -it <pod-name> -- curl http://tgi-server:80/health
```

### Performance Monitoring

```bash
# Monitor resource usage
kubectl top pods -l app=llama-stack

# Check TGI server metrics
kubectl exec -it <tgi-pod> -- curl http://localhost:80/metrics

# Monitor request latency
kubectl logs -l app=llama-stack | grep -i "response_time"
```

### Common Issues

1. **TGI Server Unreachable**
   ```bash
   # Check TGI server status
   kubectl get pods -l app=tgi-server
   kubectl logs -l app=tgi-server
   
   # Test connectivity
   kubectl exec -it <llamastack-pod> -- curl http://tgi-server:80/health
   ```

2. **Model Loading Failures**
   - Verify model ID exists on Hugging Face Hub
   - Check TGI server has sufficient resources
   - Ensure model is compatible with TGI

3. **Timeout Issues**
   - Increase `TGI_TIMEOUT` value
   - Check TGI server performance
   - Monitor network latency

## Best Practices

### Performance Optimization
- Use appropriate batch sizes for your workload
- Configure TGI server with optimal parameters
- Monitor and tune timeout values
- Use multiple TGI servers for high availability

### Resource Management
- Size TGI servers based on model requirements
- Monitor GPU utilization if using GPU acceleration
- Scale LlamaStack replicas based on request volume
- Use resource requests and limits

### Security
- Secure TGI server endpoints with authentication
- Use network policies to restrict access
- Monitor API usage and implement rate limiting
- Keep TGI server images updated

### Model Management
- Version your models and TGI configurations
- Test model changes in development first
- Monitor model performance and accuracy
- Have rollback procedures for model updates

## Next Steps

- [Configure Scaling](../how-to-guides/scaling.md)
- [Set up Monitoring](../how-to-guides/monitoring.md)
- [Security Configuration](../how-to-guides/security.md)
- [Performance Tuning](../how-to-guides/performance.md)

## API Reference

For complete API documentation, see:
- [API Reference](../reference/api.md)
- [Configuration Reference](../reference/configuration.md)
