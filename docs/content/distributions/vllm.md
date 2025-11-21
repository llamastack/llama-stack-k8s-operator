# vLLM Distribution

vLLM is a high-performance inference engine optimized for large language models. The LlamaStack Kubernetes operator provides built-in support for vLLM through pre-configured distributions.

## Overview

vLLM offers excellent performance characteristics:

- **High Throughput**: Optimized for serving multiple concurrent requests
- **Memory Efficiency**: Advanced memory management and attention mechanisms
- **GPU Acceleration**: Native CUDA support for NVIDIA GPUs
- **Model Compatibility**: Supports a wide range of popular model architectures

## Pre-Built vLLM Distributions

The operator includes two pre-built vLLM distributions:

### vllm-gpu (Self-Hosted)
- **Image**: `docker.io/llamastack/distribution-vllm-gpu:latest`
- **Purpose**: GPU-accelerated vLLM inference with local model serving
- **Requirements**: NVIDIA GPU with CUDA support
- **Infrastructure**: You provide GPU infrastructure
- **Use Case**: High-performance inference for production workloads

### remote-vllm (External Connection)
- **Image**: `docker.io/llamastack/distribution-remote-vllm:latest`
- **Purpose**: Connect to external vLLM server
- **Requirements**: Access to external vLLM endpoint
- **Infrastructure**: External vLLM server required
- **Use Case**: Using existing vLLM deployments or managed services

## Quick Start with vLLM

### 1. Create a LlamaStackDistribution

```yaml
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: my-vllm-distribution
  namespace: default
spec:
  replicas: 1
  server:
    distribution:
      name: "vllm-gpu"  # Use supported distribution
    containerSpec:
      port: 8321
      resources:
        requests:
          nvidia.com/gpu: "1"
          memory: "16Gi"
          cpu: "4"
        limits:
          nvidia.com/gpu: "1"
          memory: "32Gi"
          cpu: "8"
      env:
        - name: INFERENCE_MODEL
          value: "meta-llama/Llama-2-7b-chat-hf"
    storage:
      size: "50Gi"
      mountPath: "/.llama"
```

### 2. Deploy the Distribution

```bash
kubectl apply -f vllm-distribution.yaml
```

### 3. Verify Deployment

```bash
kubectl get llamastackdistribution my-vllm-distribution
kubectl get pods -l app=llama-stack
```

## Configuration Options

### Container Specification

The `containerSpec` section allows you to configure the container:

```yaml
spec:
  server:
    containerSpec:
      name: "llama-stack"  # Optional, defaults to "llama-stack"
      port: 8321           # Optional, defaults to 8321
      resources:
        requests:
          nvidia.com/gpu: "1"
          memory: "16Gi"
          cpu: "4"
        limits:
          nvidia.com/gpu: "1"
          memory: "32Gi"
          cpu: "8"
      env:
        - name: INFERENCE_MODEL
          value: "meta-llama/Llama-2-7b-chat-hf"
        - name: VLLM_GPU_MEMORY_UTILIZATION
          value: "0.9"
        - name: VLLM_MAX_SEQ_LEN
          value: "4096"
```

### Environment Variables

Configure vLLM behavior through environment variables:

```yaml
env:
  - name: INFERENCE_MODEL
    value: "meta-llama/Llama-2-7b-chat-hf"
  - name: VLLM_GPU_MEMORY_UTILIZATION
    value: "0.9"
  - name: VLLM_MAX_SEQ_LEN
    value: "4096"
  - name: VLLM_MAX_BATCH_SIZE
    value: "32"
  - name: VLLM_TENSOR_PARALLEL_SIZE
    value: "1"
```

### Resource Requirements

```yaml
resources:
  requests:
    nvidia.com/gpu: "1"
    memory: "16Gi"
    cpu: "4"
  limits:
    nvidia.com/gpu: "1"
    memory: "32Gi"
    cpu: "8"
```

### Storage Configuration

```yaml
storage:
  size: "50Gi"
  mountPath: "/.llama"  # Optional, defaults to "/.llama"
```

## Advanced Configuration

### Multi-GPU Setup

For larger models requiring multiple GPUs:

```yaml
spec:
  server:
    containerSpec:
      resources:
        requests:
          nvidia.com/gpu: "4"
          memory: "64Gi"
          cpu: "16"
        limits:
          nvidia.com/gpu: "4"
          memory: "128Gi"
          cpu: "32"
      env:
        - name: INFERENCE_MODEL
          value: "meta-llama/Llama-2-70b-chat-hf"
        - name: VLLM_TENSOR_PARALLEL_SIZE
          value: "4"
```

### Custom Volumes with Pod Overrides

```yaml
spec:
  server:
    podOverrides:
      volumes:
        - name: model-cache
          persistentVolumeClaim:
            claimName: model-cache-pvc
      volumeMounts:
        - name: model-cache
          mountPath: /models
    containerSpec:
      env:
        - name: INFERENCE_MODEL
          value: "/models/custom-llama-model"
```

### Scaling with Multiple Replicas

```yaml
spec:
  replicas: 3
  server:
    distribution:
      name: "vllm-gpu"
    containerSpec:
      resources:
        requests:
          nvidia.com/gpu: "1"
          memory: "16Gi"
        limits:
          nvidia.com/gpu: "1"
          memory: "32Gi"
```

## Using vLLM with the Kubernetes Operator

The LlamaStack Kubernetes operator supports vLLM in two ways:

### 1. Pre-Built Distributions (Recommended)

Use pre-built, maintained distributions with the `distribution.name` field:

#### vllm-gpu Distribution

```yaml
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: vllm-gpu-distribution
  namespace: default
spec:
  replicas: 1
  server:
    distribution:
      name: "vllm-gpu"  # Supported distribution
    containerSpec:
      resources:
        requests:
          nvidia.com/gpu: "1"
          memory: "16Gi"
          cpu: "4"
        limits:
          nvidia.com/gpu: "1"
          memory: "32Gi"
          cpu: "8"
      env:
        - name: INFERENCE_MODEL
          value: "meta-llama/Llama-2-7b-chat-hf"
        - name: VLLM_GPU_MEMORY_UTILIZATION
          value: "0.9"
    storage:
      size: "50Gi"
```

#### remote-vllm Distribution

```yaml
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: remote-vllm-distribution
  namespace: default
spec:
  replicas: 1
  server:
    distribution:
      name: "remote-vllm"  # Supported distribution
    containerSpec:
      resources:
        requests:
          memory: "4Gi"
          cpu: "2"
        limits:
          memory: "8Gi"
          cpu: "4"
      env:
        - name: INFERENCE_MODEL
          value: "meta-llama/Llama-2-7b-chat-hf"
        - name: VLLM_URL
          value: "http://external-vllm-service:8000"
```

### 2. Bring Your Own (BYO) Custom Images

Use custom-built distributions with the `distribution.image` field:

```yaml
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: custom-vllm-distribution
  namespace: default
spec:
  replicas: 1
  server:
    distribution:
      image: "my-registry.com/custom-vllm:v1.0.0"  # Custom image
    containerSpec:
      resources:
        requests:
          nvidia.com/gpu: "2"
          memory: "32Gi"
          cpu: "8"
        limits:
          nvidia.com/gpu: "2"
          memory: "64Gi"
          cpu: "16"
      env:
        - name: INFERENCE_MODEL
          value: "my-custom-model"
        - name: CUSTOM_VLLM_SETTING
          value: "optimized"
    storage:
      size: "100Gi"
```

## Building Custom vLLM Distributions

### Step 1: Build with LlamaStack CLI

#### Option A: From Template

```bash
# Install LlamaStack CLI
pip install llama-stack

# Build from vLLM template
llama stack build --template vllm-gpu --image-type container --image-name my-vllm-dist
```

#### Option B: Custom Configuration

Create `custom-vllm-build.yaml`:

```yaml
name: custom-vllm
distribution_spec:
  description: Custom vLLM distribution with optimizations
  providers:
    inference: inline::vllm
    memory: inline::faiss
    safety: inline::llama-guard
    agents: inline::meta-reference
    telemetry: inline::meta-reference
image_name: custom-vllm
image_type: container
```

Build the distribution:

```bash
llama stack build --config custom-vllm-build.yaml
```

### Step 2: Enhance with Custom Dockerfile

Create `Dockerfile.enhanced`:

```dockerfile
FROM distribution-custom-vllm:dev

# Install additional dependencies
RUN pip install \
    flash-attn \
    custom-optimization-lib \
    monitoring-tools

# Add custom configurations
COPY vllm-config.json /app/config.json
COPY custom-models/ /app/models/

# Set optimization environment variables
ENV VLLM_USE_FLASH_ATTN=1
ENV VLLM_OPTIMIZATION_LEVEL=high
ENV CUSTOM_GPU_SETTINGS=enabled

# Add health check script
COPY health-check.sh /app/health-check.sh
RUN chmod +x /app/health-check.sh

HEALTHCHECK --interval=30s --timeout=10s --start-period=60s --retries=3 \
  CMD /app/health-check.sh

EXPOSE 8321
```

Build the enhanced image:

```bash
docker build -f Dockerfile.enhanced -t my-registry.com/enhanced-vllm:v1.0.0 .
```

### Step 3: Push to Registry

```bash
# Tag for your registry
docker tag my-registry.com/enhanced-vllm:v1.0.0 my-registry.com/enhanced-vllm:latest

# Push to registry
docker push my-registry.com/enhanced-vllm:v1.0.0
docker push my-registry.com/enhanced-vllm:latest
```

### Step 4: Deploy with Operator

```yaml
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: enhanced-vllm-dist
  namespace: production
spec:
  replicas: 2
  server:
    distribution:
      image: "my-registry.com/enhanced-vllm:v1.0.0"
    containerSpec:
      resources:
        requests:
          nvidia.com/gpu: "2"
          memory: "32Gi"
          cpu: "8"
        limits:
          nvidia.com/gpu: "2"
          memory: "64Gi"
          cpu: "16"
      env:
        - name: INFERENCE_MODEL
          value: "meta-llama/Llama-2-13b-chat-hf"
        - name: VLLM_TENSOR_PARALLEL_SIZE
          value: "2"
        - name: VLLM_GPU_MEMORY_UTILIZATION
          value: "0.85"
        - name: CUSTOM_OPTIMIZATION
          value: "enabled"
    storage:
      size: "200Gi"
    podOverrides:
      volumes:
        - name: model-cache
          persistentVolumeClaim:
            claimName: shared-model-cache
      volumeMounts:
        - name: model-cache
          mountPath: /shared-models
```

## Comparison: Pre-Built vs BYO

| Aspect | Pre-Built Distributions | BYO Custom Images |
|--------|------------------------|-------------------|
| **Setup Complexity** | Simple - just specify `name` | Complex - build and maintain images |
| **Maintenance** | Maintained by LlamaStack team | You maintain the images |
| **Customization** | Limited to environment variables | Full control over dependencies and configuration |
| **Security** | Vetted by maintainers | You control security scanning and updates |
| **Performance** | Standard optimizations | Custom optimizations possible |
| **Support** | Community and official support | Self-supported |
| **Updates** | Automatic with operator updates | Manual image rebuilds required |

### When to Use Pre-Built Distributions

- **Quick deployment** and standard use cases
- **Production environments** where stability is key
- **Limited customization** requirements
- **Teams without container expertise**

### When to Use BYO Custom Images

- **Specialized models** or inference engines
- **Custom optimizations** for specific hardware
- **Additional dependencies** not in standard images
- **Compliance requirements** for image provenance
- **Integration** with existing infrastructure

## Monitoring and Troubleshooting

### Health Checks

The vLLM distribution includes built-in health checks:

```bash
# Check pod status
kubectl get pods -l app=llama-stack

# View logs
kubectl logs -l app=llama-stack

# Check service endpoints
kubectl get svc my-vllm-distribution-service
```

### Performance Monitoring

```bash
# Monitor GPU utilization
kubectl exec -it <vllm-pod> -- nvidia-smi

# Check memory usage
kubectl top pods -l app=llama-stack
```

### Common Issues

1. **GPU Not Available**
   - Ensure NVIDIA device plugin is installed
   - Verify GPU resources in node capacity

2. **Out of Memory**
   - Reduce `VLLM_GPU_MEMORY_UTILIZATION`
   - Increase memory limits
   - Use smaller models

3. **Model Loading Failures**
   - Check model path and permissions
   - Verify sufficient storage space
   - Check environment variable values

## Best Practices

### Resource Planning

- **GPU Memory**: Ensure sufficient VRAM for model + batch processing
- **CPU**: Allocate adequate CPU for preprocessing and coordination
- **Storage**: Use fast storage (NVMe SSD) for model loading

### Environment Variable Guidelines

- Use `INFERENCE_MODEL` to specify the model to load
- Set `VLLM_GPU_MEMORY_UTILIZATION` to control GPU memory usage (0.8-0.9 recommended)
- Configure `VLLM_MAX_SEQ_LEN` based on your use case requirements
- Use `VLLM_TENSOR_PARALLEL_SIZE` for multi-GPU setups

### Security

- Use private registries for custom images
- Implement proper RBAC for distribution management
- Secure model storage with appropriate access controls

## Examples

### Production Setup

```yaml
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: production-vllm
  namespace: llama-production
spec:
  replicas: 2
  server:
    distribution:
      name: "vllm-gpu"
    containerSpec:
      resources:
        requests:
          nvidia.com/gpu: "2"
          memory: "32Gi"
          cpu: "8"
        limits:
          nvidia.com/gpu: "2"
          memory: "64Gi"
          cpu: "16"
      env:
        - name: INFERENCE_MODEL
          value: "meta-llama/Llama-2-13b-chat-hf"
        - name: VLLM_TENSOR_PARALLEL_SIZE
          value: "2"
        - name: VLLM_GPU_MEMORY_UTILIZATION
          value: "0.85"
    storage:
      size: "100Gi"
```

### Development Setup

```yaml
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: dev-vllm
  namespace: development
spec:
  replicas: 1
  server:
    distribution:
      name: "vllm-gpu"
    containerSpec:
      resources:
        requests:
          nvidia.com/gpu: "1"
          memory: "8Gi"
          cpu: "2"
        limits:
          nvidia.com/gpu: "1"
          memory: "16Gi"
          cpu: "4"
      env:
        - name: INFERENCE_MODEL
          value: "microsoft/DialoGPT-small"
    storage:
      size: "20Gi"
```

## API Reference

For complete API documentation, see:
- [API Reference](../reference/api.md)
- [Configuration Reference](../reference/configuration.md)

## Next Steps

- [Configure Storage](../how-to/configure-storage.md)
- [Scaling Guide](../how-to/scaling.md)
- [Monitoring Setup](../how-to/monitoring.md)
- [Ollama Distribution](ollama.md)
