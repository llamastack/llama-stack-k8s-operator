# Bring Your Own (BYO) Distributions

The LlamaStack Kubernetes operator supports both pre-built distributions and custom "Bring Your Own" (BYO) distributions. This guide shows you how to build, customize, and deploy your own LlamaStack distributions.

## Overview

### Supported vs BYO Distributions

| Type | Description | Use Case | Configuration |
|------|-------------|----------|---------------|
| **Supported** | Pre-built distributions maintained by the LlamaStack team | Quick deployment, standard configurations | Use `distribution.name` field |
| **BYO** | Custom distributions you build and maintain | Custom providers, specialized configurations | Use `distribution.image` field |

### Why Build Custom Distributions?

- **Custom Providers**: Integrate with proprietary or specialized inference engines
- **Specific Configurations**: Tailor the stack for your exact requirements
- **External Dependencies**: Include additional libraries or tools
- **Security Requirements**: Control the entire build process and dependencies
- **Performance Optimization**: Optimize for your specific hardware or use case

## Building LlamaStack Distributions

### Prerequisites

1. **Install LlamaStack CLI**:
   ```bash
   pip install llama-stack
   ```

2. **Docker or Podman** (for container builds):
   ```bash
   # Verify Docker is running
   docker --version
   ```

3. **Conda** (for conda builds):
   ```bash
   # Verify Conda is available
   conda --version
   ```

### Quick Start: Building from Templates

#### 1. List Available Templates

```bash
llama stack build --list-templates
```

This shows available templates like:
- `ollama` - Ollama-based inference
- `vllm-gpu` - vLLM with GPU support
- `meta-reference-gpu` - Meta's reference implementation
- `bedrock` - AWS Bedrock integration
- `fireworks` - Fireworks AI integration

#### 2. Build from Template

```bash
# Build a container image from Ollama template
llama stack build --template ollama --image-type container

# Build a conda environment from vLLM template
llama stack build --template vllm-gpu --image-type conda

# Build with custom name
llama stack build --template ollama --image-type container --image-name my-custom-ollama
```

#### 3. Interactive Build

```bash
llama stack build
```

This launches an interactive wizard:

```
> Enter a name for your Llama Stack (e.g. my-local-stack): my-custom-stack
> Enter the image type you want your Llama Stack to be built as (container or conda or venv): container

Llama Stack is composed of several APIs working together. Let's select
the provider types (implementations) you want to use for these APIs.

> Enter provider for API inference: inline::meta-reference
> Enter provider for API safety: inline::llama-guard
> Enter provider for API agents: inline::meta-reference
> Enter provider for API memory: inline::faiss
> Enter provider for API datasetio: inline::meta-reference
> Enter provider for API scoring: inline::meta-reference
> Enter provider for API eval: inline::meta-reference
> Enter provider for API telemetry: inline::meta-reference

> (Optional) Enter a short description for your Llama Stack: My custom distribution
```

### Advanced: Custom Configuration Files

#### 1. Create a Custom Build Configuration

Create `my-custom-build.yaml`:

```yaml
name: my-custom-stack
distribution_spec:
  description: Custom distribution with external Ollama
  providers:
    inference: remote::ollama
    memory: inline::faiss
    safety: inline::llama-guard
    agents: inline::meta-reference
    telemetry: inline::meta-reference
    datasetio: inline::meta-reference
    scoring: inline::meta-reference
    eval: inline::meta-reference
image_name: my-custom-stack
image_type: container

# Optional: External providers directory
external_providers_dir: ~/.llama/providers.d
```

#### 2. Build from Custom Configuration

```bash
llama stack build --config my-custom-build.yaml
```

### Image Types

#### Container Images

Best for production deployments and Kubernetes:

```bash
llama stack build --template ollama --image-type container
```

**Advantages**:
- Consistent across environments
- Easy to deploy in Kubernetes
- Isolated dependencies
- Reproducible builds

#### Conda Environments

Good for development and local testing:

```bash
llama stack build --template ollama --image-type conda
```

**Advantages**:
- Fast iteration during development
- Easy dependency management
- Good for experimentation

#### Virtual Environments

Lightweight option for Python-only setups:

```bash
llama stack build --template ollama --image-type venv
```

## Custom Providers

### Adding External Providers

#### 1. Create Provider Configuration

Create `~/.llama/providers.d/custom-ollama.yaml`:

```yaml
adapter:
  adapter_type: custom_ollama
  pip_packages:
    - ollama
    - aiohttp
    - llama-stack-provider-ollama
  config_class: llama_stack_ollama_provider.config.OllamaImplConfig
  module: llama_stack_ollama_provider
api_dependencies: []
optional_api_dependencies: []
```

#### 2. Reference in Build Configuration

```yaml
name: custom-external-stack
distribution_spec:
  description: Custom distro with external providers
  providers:
    inference: remote::custom_ollama
    memory: inline::faiss
    safety: inline::llama-guard
    agents: inline::meta-reference
    telemetry: inline::meta-reference
image_type: container
image_name: custom-external-stack
external_providers_dir: ~/.llama/providers.d
```

## Using Custom Distributions with Kubernetes

### 1. Build and Push Container Image

```bash
# Build the distribution
llama stack build --template ollama --image-type container --image-name my-ollama-dist

# Tag for your registry
docker tag distribution-my-ollama-dist:dev my-registry.com/my-ollama-dist:v1.0.0

# Push to registry
docker push my-registry.com/my-ollama-dist:v1.0.0
```

### 2. Deploy with Kubernetes Operator

```yaml
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: my-custom-distribution
  namespace: default
spec:
  replicas: 1
  server:
    distribution:
      image: "my-registry.com/my-ollama-dist:v1.0.0"  # Custom image
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
          value: "http://ollama-server:11434"
    storage:
      size: "20Gi"
```

### 3. Verify Deployment

```bash
kubectl get llamastackdistribution my-custom-distribution
kubectl get pods -l app=llama-stack
kubectl logs -l app=llama-stack
```

## Examples

### Example 1: Custom Ollama Distribution

#### Build Configuration (`custom-ollama-build.yaml`)

```yaml
name: custom-ollama
distribution_spec:
  description: Custom Ollama distribution with additional tools
  providers:
    inference: remote::ollama
    memory: inline::faiss
    safety: inline::llama-guard
    agents: inline::meta-reference
    telemetry: inline::meta-reference
image_name: custom-ollama
image_type: container
```

#### Build and Deploy

```bash
# Build the distribution
llama stack build --config custom-ollama-build.yaml

# Tag and push
docker tag distribution-custom-ollama:dev my-registry.com/custom-ollama:latest
docker push my-registry.com/custom-ollama:latest
```

#### Kubernetes Deployment

```yaml
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: custom-ollama-dist
spec:
  replicas: 2
  server:
    distribution:
      image: "my-registry.com/custom-ollama:latest"
    containerSpec:
      resources:
        requests:
          memory: "8Gi"
          cpu: "4"
        limits:
          memory: "16Gi"
          cpu: "8"
      env:
        - name: INFERENCE_MODEL
          value: "llama3.2:3b"
        - name: OLLAMA_URL
          value: "http://ollama-service:11434"
```

### Example 2: Custom vLLM Distribution

#### Build Configuration (`custom-vllm-build.yaml`)

```yaml
name: custom-vllm
distribution_spec:
  description: Custom vLLM distribution with GPU optimization
  providers:
    inference: inline::vllm
    memory: inline::faiss
    safety: inline::llama-guard
    agents: inline::meta-reference
    telemetry: inline::meta-reference
image_name: custom-vllm
image_type: container
```

#### Enhanced Dockerfile

Create a custom Dockerfile to extend the base distribution:

```dockerfile
FROM distribution-custom-vllm:dev

# Install additional dependencies
RUN pip install custom-optimization-library

# Add custom configuration
COPY custom-vllm-config.json /app/config.json

# Set environment variables
ENV VLLM_OPTIMIZATION_LEVEL=high
ENV CUSTOM_GPU_SETTINGS=enabled

# Expose port
EXPOSE 8321
```

#### Build and Deploy

```bash
# Build the LlamaStack distribution
llama stack build --config custom-vllm-build.yaml

# Build enhanced Docker image
docker build -t my-registry.com/enhanced-vllm:latest .

# Push to registry
docker push my-registry.com/enhanced-vllm:latest
```

#### Kubernetes Deployment

```yaml
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: enhanced-vllm-dist
spec:
  replicas: 1
  server:
    distribution:
      image: "my-registry.com/enhanced-vllm:latest"
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
        - name: VLLM_GPU_MEMORY_UTILIZATION
          value: "0.9"
        - name: VLLM_TENSOR_PARALLEL_SIZE
          value: "2"
```

### Example 3: Multi-Provider Distribution

#### Build Configuration (`multi-provider-build.yaml`)

```yaml
name: multi-provider
distribution_spec:
  description: Distribution with multiple inference providers
  providers:
    inference: 
      - remote::ollama
      - remote::vllm
    memory: inline::faiss
    safety: inline::llama-guard
    agents: inline::meta-reference
    telemetry: inline::meta-reference
image_name: multi-provider
image_type: container
```

## Testing Custom Distributions

### Local Testing

#### 1. Run Locally with Docker

```bash
# Set environment variables
export LLAMA_STACK_PORT=8321
export INFERENCE_MODEL="llama3.2:1b"

# Run the custom distribution
docker run -d \
  -p $LLAMA_STACK_PORT:$LLAMA_STACK_PORT \
  -v ~/.llama:/root/.llama \
  distribution-custom-ollama:dev \
  --port $LLAMA_STACK_PORT \
  --env INFERENCE_MODEL=$INFERENCE_MODEL \
  --env OLLAMA_URL=http://host.docker.internal:11434
```

#### 2. Test API Endpoints

```bash
# Health check
curl http://localhost:8321/v1/health

# List providers
curl http://localhost:8321/v1/providers

# Test inference
curl -X POST http://localhost:8321/v1/completions \
  -H "Content-Type: application/json" \
  -d '{
    "model": "llama3.2:1b",
    "prompt": "Hello, world!",
    "max_tokens": 50
  }'
```

### Kubernetes Testing

#### 1. Deploy to Test Namespace

```bash
kubectl create namespace llama-test
kubectl apply -f custom-distribution.yaml -n llama-test
```

#### 2. Port Forward for Testing

```bash
kubectl port-forward svc/my-custom-distribution-service 8321:8321 -n llama-test
```

#### 3. Run Tests

```bash
# Test from within cluster
kubectl run test-pod --image=curlimages/curl --rm -it -- \
  curl http://my-custom-distribution-service:8321/v1/health
```

## Best Practices

### Security

1. **Use Private Registries**: Store custom images in private container registries
2. **Scan Images**: Use container scanning tools to check for vulnerabilities
3. **Minimal Base Images**: Use slim or distroless base images when possible
4. **Secrets Management**: Use Kubernetes secrets for API keys and credentials

### Performance

1. **Multi-stage Builds**: Use multi-stage Dockerfiles to reduce image size
2. **Layer Caching**: Optimize Dockerfile layer ordering for better caching
3. **Resource Limits**: Set appropriate CPU and memory limits
4. **GPU Optimization**: Configure GPU settings for inference workloads

### Maintenance

1. **Version Tags**: Use semantic versioning for your custom images
2. **Documentation**: Document your custom configurations and dependencies
3. **Testing**: Implement automated testing for custom distributions
4. **Monitoring**: Set up monitoring and logging for custom deployments

### Development Workflow

1. **Local Development**: Use conda/venv builds for rapid iteration
2. **CI/CD Integration**: Automate building and testing of custom distributions
3. **Staging Environment**: Test in staging before production deployment
4. **Rollback Strategy**: Maintain previous versions for quick rollbacks

## Troubleshooting

### Common Issues

#### Build Failures

```bash
# Check build logs
llama stack build --template ollama --image-type container --verbose

# Verify dependencies
llama stack build --config my-build.yaml --print-deps-only
```

#### Runtime Issues

```bash
# Check container logs
docker logs <container-id>

# Debug with interactive shell
docker run -it --entrypoint /bin/bash distribution-custom:dev
```

#### Kubernetes Issues

```bash
# Check pod status
kubectl describe pod <pod-name>

# View logs
kubectl logs <pod-name> -f

# Check events
kubectl get events --sort-by=.metadata.creationTimestamp
```

### Getting Help

1. **LlamaStack Documentation**: [Official docs](https://github.com/meta-llama/llama-stack)
2. **GitHub Issues**: Report bugs and ask questions
3. **Community Forums**: Join the LlamaStack community discussions
4. **Operator Documentation**: Check the Kubernetes operator guides

## Next Steps

- [vLLM Distribution](vllm.md) - Learn about vLLM-specific configurations
- [Ollama Distribution](ollama.md) - Explore Ollama distribution options
- [Configuration Reference](../reference/configuration.md) - Complete API reference
- [Scaling Guide](../how-to/scaling.md) - Scale your custom distributions
