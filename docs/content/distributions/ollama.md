# Ollama Distribution

Ollama is a user-friendly platform for running large language models locally. The LlamaStack Kubernetes operator provides built-in support for Ollama through a pre-configured distribution.

## Overview

Ollama offers several advantages:

- **Ease of Use**: Simple model management and deployment
- **Local Execution**: Run models entirely on your infrastructure
- **Model Library**: Access to a curated collection of popular models
- **Resource Efficiency**: Optimized for various hardware configurations
- **API Compatibility**: OpenAI-compatible API endpoints

## Pre-Built Ollama Distribution

The operator includes one pre-configured Ollama distribution:

### ollama
- **Image**: `docker.io/llamastack/distribution-ollama:latest`
- **Purpose**: Standard Ollama deployment
- **Requirements**: CPU or GPU resources depending on model
- **Use Case**: General-purpose local LLM inference

## Quick Start with Ollama

### 1. Create a LlamaStackDistribution

```yaml
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: my-ollama-distribution
  namespace: default
spec:
  replicas: 1
  server:
    distribution:
      name: "ollama"  # Use supported distribution
    containerSpec:
      port: 8321
      resources:
        requests:
          memory: "8Gi"
          cpu: "4"
        limits:
          memory: "16Gi"
          cpu: "8"
      env:
        - name: INFERENCE_MODEL
          value: "llama3.2:1b"
    storage:
      size: "20Gi"
      mountPath: "/.llama"
```

### 2. Deploy the Distribution

```bash
kubectl apply -f ollama-distribution.yaml
```

### 3. Verify Deployment

```bash
kubectl get llamastackdistribution my-ollama-distribution
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
          memory: "8Gi"
          cpu: "4"
        limits:
          memory: "16Gi"
          cpu: "8"
      env:
        - name: INFERENCE_MODEL
          value: "llama3.2:1b"
        - name: OLLAMA_HOST
          value: "0.0.0.0:11434"
        - name: OLLAMA_ORIGINS
          value: "*"
```

### Environment Variables

Configure Ollama behavior through environment variables:

```yaml
env:
  - name: INFERENCE_MODEL
    value: "llama2:7b"
  - name: OLLAMA_HOST
    value: "0.0.0.0:11434"
  - name: OLLAMA_ORIGINS
    value: "*"
  - name: OLLAMA_NUM_PARALLEL
    value: "4"
  - name: OLLAMA_MAX_LOADED_MODELS
    value: "3"
```

### Popular Models

You can specify different models using the `INFERENCE_MODEL` environment variable:

```yaml
# Llama 2 variants
- name: INFERENCE_MODEL
  value: "llama2:7b"      # 7B parameter model
# value: "llama2:13b"     # 13B parameter model
# value: "llama2:70b"     # 70B parameter model

# Code-focused models
# value: "codellama:7b"   # Code generation
# value: "codellama:13b"  # Larger code model

# Chat-optimized models
# value: "llama2:7b-chat"
# value: "llama2:13b-chat"

# Other popular models
# value: "mistral:7b"     # Mistral 7B
# value: "neural-chat:7b" # Intel's neural chat
# value: "orca-mini:3b"   # Smaller, efficient model
```

### Resource Requirements

```yaml
resources:
  requests:
    memory: "8Gi"
    cpu: "4"
  limits:
    memory: "16Gi"
    cpu: "8"
```

### GPU Support

For GPU acceleration:

```yaml
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
    value: "llama2:7b"
  - name: OLLAMA_GPU_LAYERS
    value: "35"  # Number of layers to run on GPU
```

### Storage Configuration

```yaml
storage:
  size: "20Gi"
  mountPath: "/.llama"  # Optional, defaults to "/.llama"
```

## Advanced Configuration

### Custom Model Management with Pod Overrides

```yaml
spec:
  server:
    podOverrides:
      volumes:
        - name: ollama-models
          persistentVolumeClaim:
            claimName: ollama-models-pvc
      volumeMounts:
        - name: ollama-models
          mountPath: /root/.ollama
    containerSpec:
      env:
        - name: INFERENCE_MODEL
          value: "llama3.2:1b"
        - name: OLLAMA_MODELS
          value: "/root/.ollama/models"
```

### Multiple Model Setup

```yaml
spec:
  server:
    containerSpec:
      env:
        - name: INFERENCE_MODEL
          value: "llama3.2:1b"  # Primary model
        - name: OLLAMA_MAX_LOADED_MODELS
          value: "3"
        - name: ADDITIONAL_MODELS
          value: "codellama:7b,mistral:7b"  # Additional models to pull
      resources:
        requests:
          memory: "24Gi"
          cpu: "8"
        limits:
          memory: "48Gi"
          cpu: "16"
```

### Scaling with Multiple Replicas

```yaml
spec:
  replicas: 2
  server:
    distribution:
      name: "ollama"
    containerSpec:
      resources:
        requests:
          memory: "8Gi"
          cpu: "4"
        limits:
          memory: "16Gi"
          cpu: "8"
```

## Using Ollama with the Kubernetes Operator

The LlamaStack Kubernetes operator supports Ollama in two ways:

### 1. Pre-Built Distribution (Recommended)

Use the pre-built, maintained distribution with the `distribution.name` field:

```yaml
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: ollama-distribution
  namespace: default
spec:
  replicas: 1
  server:
    distribution:
      name: "ollama"  # Supported distribution
    containerSpec:
      port: 8321
      resources:
        requests:
          memory: "8Gi"
          cpu: "4"
        limits:
          memory: "16Gi"
          cpu: "8"
      env:
        - name: INFERENCE_MODEL
          value: "llama3.2:1b"
        - name: OLLAMA_URL
          value: "http://ollama-server-service.ollama-dist.svc.cluster.local:11434"
    storage:
      size: "20Gi"
```

#### With GPU Support

```yaml
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: ollama-gpu-distribution
  namespace: default
spec:
  replicas: 1
  server:
    distribution:
      name: "ollama"  # Supported distribution
    containerSpec:
      resources:
        requests:
          nvidia.com/gpu: "1"
          memory: "16Gi"
          cpu: "8"
        limits:
          nvidia.com/gpu: "1"
          memory: "32Gi"
          cpu: "16"
      env:
        - name: INFERENCE_MODEL
          value: "llama2:7b"
        - name: OLLAMA_GPU_LAYERS
          value: "35"
        - name: OLLAMA_NUM_PARALLEL
          value: "4"
    storage:
      size: "50Gi"
```

### 2. Bring Your Own (BYO) Custom Images

Use custom-built distributions with the `distribution.image` field:

```yaml
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: custom-ollama-distribution
  namespace: default
spec:
  replicas: 1
  server:
    distribution:
      image: "my-registry.com/custom-ollama:v1.0.0"  # Custom image
    containerSpec:
      resources:
        requests:
          memory: "16Gi"
          cpu: "8"
        limits:
          memory: "32Gi"
          cpu: "16"
      env:
        - name: INFERENCE_MODEL
          value: "custom-model:latest"
        - name: CUSTOM_OLLAMA_SETTING
          value: "optimized"
    storage:
      size: "100Gi"
```

## Building Custom Ollama Distributions

### Step 1: Build with LlamaStack CLI

#### Option A: From Template

```bash
# Install LlamaStack CLI
pip install llama-stack

# Build from Ollama template
llama stack build --template ollama --image-type container --image-name my-ollama-dist
```

#### Option B: Custom Configuration

Create `custom-ollama-build.yaml`:

```yaml
name: custom-ollama
distribution_spec:
  description: Custom Ollama distribution with pre-loaded models
  providers:
    inference: remote::ollama
    memory: inline::faiss
    safety: inline::llama-guard
    agents: inline::meta-reference
    telemetry: inline::meta-reference
image_name: custom-ollama
image_type: container
```

Build the distribution:

```bash
llama stack build --config custom-ollama-build.yaml
```

### Step 2: Enhance with Custom Dockerfile

Create `Dockerfile.enhanced`:

```dockerfile
FROM distribution-custom-ollama:dev

# Install additional tools
RUN apt-get update && apt-get install -y \
    curl \
    jq \
    htop \
    && rm -rf /var/lib/apt/lists/*

# Pre-pull popular models
RUN ollama pull llama3.2:1b && \
    ollama pull llama3.2:3b && \
    ollama pull codellama:7b && \
    ollama pull mistral:7b

# Add custom model management scripts
COPY scripts/model-manager.sh /usr/local/bin/model-manager
COPY scripts/health-check.sh /usr/local/bin/health-check
RUN chmod +x /usr/local/bin/model-manager /usr/local/bin/health-check

# Add custom Ollama configuration
COPY ollama-config.json /etc/ollama/config.json

# Set optimized environment variables
ENV OLLAMA_HOST=0.0.0.0:11434
ENV OLLAMA_ORIGINS=*
ENV OLLAMA_NUM_PARALLEL=4
ENV OLLAMA_MAX_LOADED_MODELS=3
ENV OLLAMA_KEEP_ALIVE=5m

# Add health check
HEALTHCHECK --interval=30s --timeout=10s --start-period=60s --retries=3 \
  CMD health-check

EXPOSE 8321 11434
```

### Step 3: Deploy with Operator

```yaml
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: enhanced-ollama-dist
  namespace: production
spec:
  replicas: 2
  server:
    distribution:
      image: "my-registry.com/enhanced-ollama:v1.0.0"
    containerSpec:
      resources:
        requests:
          memory: "16Gi"
          cpu: "8"
          nvidia.com/gpu: "1"
        limits:
          memory: "32Gi"
          cpu: "16"
          nvidia.com/gpu: "1"
      env:
        - name: INFERENCE_MODEL
          value: "llama3.2:3b"
        - name: OLLAMA_NUM_PARALLEL
          value: "4"
        - name: OLLAMA_MAX_LOADED_MODELS
          value: "2"
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

| Aspect | Pre-Built Distribution | BYO Custom Images |
|--------|----------------------|-------------------|
| **Setup Complexity** | Simple - just specify `name` | Complex - build and maintain images |
| **Maintenance** | Maintained by LlamaStack team | You maintain the images |
| **Model Management** | Runtime model pulling | Pre-loaded models possible |
| **Customization** | Limited to environment variables | Full control over Ollama configuration |
| **Security** | Vetted by maintainers | You control security scanning and updates |
| **Performance** | Standard Ollama setup | Custom optimizations possible |
| **Support** | Community and official support | Self-supported |
| **Updates** | Automatic with operator updates | Manual image rebuilds required |

### When to Use Pre-Built Distribution

- **Quick deployment** and standard use cases
- **Production environments** where stability is key
- **Dynamic model management** (pull models at runtime)
- **Teams without container expertise**
- **Standard Ollama configurations**

### When to Use BYO Custom Images

- **Pre-loaded models** for faster startup
- **Custom Ollama configurations** or patches
- **Additional tools** and utilities
- **Compliance requirements** for image provenance
- **Integration** with existing model management systems
- **Custom model formats** or converters

## Model Management

### Accessing the Ollama Container

```bash
# Connect to running Ollama pod
kubectl exec -it <ollama-pod> -- bash

# Pull models
ollama pull llama2:7b

# List available models
ollama list

# Remove unused models
ollama rm old-model:tag
```

### Model Information

```bash
# Show model details
kubectl exec -it <ollama-pod> -- ollama show llama2:7b

# Check model size and parameters
kubectl exec -it <ollama-pod> -- ollama show llama2:7b --modelfile
```

## API Usage

### REST API

Ollama provides OpenAI-compatible endpoints:

```bash
# Generate completion
curl -X POST http://ollama-service:8321/v1/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "llama2:7b",
    "prompt": "Why is the sky blue?",
    "max_tokens": 100
  }'

# Chat completion
curl -X POST http://ollama-service:8321/v1/chat/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "llama2:7b",
    "messages": [
      {"role": "user", "content": "Hello!"}
    ]
  }'
```

### Python Client

```python
import requests

# Generate text
response = requests.post(
    "http://ollama-service:8321/v1/completions",
    json={
        "model": "llama2:7b",
        "prompt": "Explain quantum computing",
        "max_tokens": 200
    }
)

print(response.json())
```

## Monitoring and Troubleshooting

### Health Checks

```bash
# Check pod status
kubectl get pods -l app=llama-stack

# View logs
kubectl logs -l app=llama-stack

# Test API endpoint
kubectl port-forward svc/my-ollama-distribution-service 8321:8321
curl http://localhost:8321/v1/health
```

### Performance Monitoring

```bash
# Monitor resource usage
kubectl top pods -l app=llama-stack

# Check model loading status
kubectl exec -it <ollama-pod> -- ollama ps
```

### Common Issues

1. **Model Download Failures**
   - Check internet connectivity
   - Verify sufficient storage space
   - Ensure proper permissions

2. **Out of Memory**
   - Use smaller models (3b, 7b instead of 13b, 70b)
   - Increase memory limits
   - Reduce concurrent requests

3. **Slow Performance**
   - Enable GPU acceleration
   - Use faster storage for model cache
   - Optimize model selection for hardware

## Best Practices

### Resource Planning

- **Memory**: Allocate 2-4x model size in RAM
- **Storage**: Plan for model downloads and cache
- **CPU**: More cores improve concurrent request handling

### Model Selection

```yaml
# For development/testing
env:
  - name: INFERENCE_MODEL
    value: "orca-mini:3b"    # Fast, lightweight

# For general use
env:
  - name: INFERENCE_MODEL
    value: "llama2:7b"       # Good balance of quality/performance

# For high-quality responses
env:
  - name: INFERENCE_MODEL
    value: "llama2:13b"      # Better quality, more resources

# For code generation
env:
  - name: INFERENCE_MODEL
    value: "codellama:7b"    # Specialized for coding tasks
```

### Security Considerations

- Use private registries for custom images
- Implement network policies for API access
- Secure model storage with appropriate permissions
- Monitor API usage and implement rate limiting

## Examples

### Production Setup

```yaml
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: production-ollama
  namespace: llama-production
spec:
  replicas: 2
  server:
    distribution:
      name: "ollama"
    containerSpec:
      resources:
        requests:
          memory: "16Gi"
          cpu: "8"
          nvidia.com/gpu: "1"
        limits:
          memory: "32Gi"
          cpu: "16"
          nvidia.com/gpu: "1"
      env:
        - name: INFERENCE_MODEL
          value: "llama3.2:1b"
        - name: OLLAMA_NUM_PARALLEL
          value: "4"
        - name: OLLAMA_MAX_LOADED_MODELS
          value: "2"
    storage:
      size: "100Gi"
```

### Development Setup

```yaml
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: dev-ollama
  namespace: development
spec:
  replicas: 1
  server:
    distribution:
      name: "ollama"
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
          value: "orca-mini:3b"
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
- [vLLM Distribution](vllm.md)
- [Understanding Distributions](../getting-started/distributions.md)
