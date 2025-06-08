# Configuration Reference

Complete reference for configuring LlamaStack Kubernetes Operator based on the actual API.

## LlamaStackDistribution Specification

### Basic Structure

```yaml
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: string
  namespace: string
spec:
  replicas: integer  # Default: 1
  server:
    distribution:
      # Either name OR image (mutually exclusive)
      name: string     # Distribution name from supported distributions
      image: string    # Direct container image reference
    containerSpec:
      name: string     # Default: "llama-stack"
      port: integer    # Default: 8321
      resources:
        requests:
          cpu: string
          memory: string
        limits:
          cpu: string
          memory: string
      env:
      - name: string
        value: string
    podOverrides:      # Optional pod-level customization
      volumes:
      - name: string
        # ... volume spec
      volumeMounts:
      - name: string
        mountPath: string
    storage:           # Optional persistent storage
      size: string     # Default: "10Gi"
      mountPath: string # Default: "/.llama"
```

## Core Configuration

### Distribution Configuration

You can specify either a distribution name OR a direct image reference:

```yaml
# Option 1: Use a named distribution (recommended)
spec:
  server:
    distribution:
      name: "starter"  # Maps to supported distributions

# Option 2: Use a direct image
spec:
  server:
    distribution:
      image: "llamastack/llamastack:latest"
```

### Supported Distribution Names

The operator supports the following **7 pre-configured distributions**:

| Distribution Name | Image | Description |
|-------------------|-------|-------------|
| `starter` | `docker.io/llamastack/distribution-starter:latest` | **Recommended default** - General purpose LlamaStack distribution |
| `ollama` | `docker.io/llamastack/distribution-ollama:latest` | Ollama-based distribution for local inference |
| `bedrock` | `docker.io/llamastack/distribution-bedrock:latest` | AWS Bedrock distribution for cloud-based models |
| `remote-vllm` | `docker.io/llamastack/distribution-remote-vllm:latest` | Remote vLLM server integration |
| `tgi` | `docker.io/llamastack/distribution-tgi:latest` | Hugging Face Text Generation Inference |
| `together` | `docker.io/llamastack/distribution-together:latest` | Together AI API integration |
| `vllm-gpu` | `docker.io/llamastack/distribution-vllm-gpu:latest` | High-performance GPU inference with vLLM |
| `remote-vllm` | `docker.io/llamastack/distribution-remote-vllm:latest` | Remote vLLM distribution |
| `sambanova` | `docker.io/llamastack/distribution-sambanova:latest` | SambaNova distribution |
| `tgi` | `docker.io/llamastack/distribution-tgi:latest` | Text Generation Inference distribution |
| `together` | `docker.io/llamastack/distribution-together:latest` | Together AI distribution |
| `vllm-gpu` | `docker.io/llamastack/distribution-vllm-gpu:latest` | vLLM GPU distribution |
| `watsonx` | `docker.io/llamastack/distribution-watsonx:latest` | IBM watsonx distribution |
| `fireworks` | `docker.io/llamastack/distribution-fireworks:latest` | Fireworks AI distribution |

**Examples:**

```yaml
# Ollama distribution
spec:
  server:
    distribution:
      name: "ollama"

# Hugging Face Endpoint
spec:
  server:
    distribution:
      name: "hf-endpoint"

# NVIDIA distribution
spec:
  server:
    distribution:
      name: "nvidia"

# vLLM GPU distribution
spec:
  server:
    distribution:
      name: "vllm-gpu"
```

### Replica Configuration

```yaml
spec:
  replicas: 3  # Default: 1
```

### Container Configuration

```yaml
spec:
  server:
    containerSpec:
      name: "llama-stack"  # Default container name
      port: 8321           # Default port
      resources:
        requests:
          cpu: "1"
          memory: "2Gi"
        limits:
          cpu: "2"
          memory: "4Gi"
      env:
      - name: "INFERENCE_MODEL"
        value: "llama2-7b"
      - name: "LOG_LEVEL"
        value: "INFO"
```

## Storage Configuration

### Basic Storage

```yaml
spec:
  server:
    storage:
      size: "50Gi"              # Default: "10Gi"
      mountPath: "/.llama"      # Default mount path
```

### Custom Mount Path

```yaml
spec:
  server:
    storage:
      size: "100Gi"
      mountPath: "/custom/path"
```

## Advanced Pod Customization

### Additional Volumes

```yaml
spec:
  server:
    podOverrides:
      volumes:
      - name: "model-cache"
        emptyDir:
          sizeLimit: "20Gi"
      - name: "config"
        configMap:
          name: "llamastack-config"
      volumeMounts:
      - name: "model-cache"
        mountPath: "/cache"
      - name: "config"
        mountPath: "/config"
        readOnly: true
```

### ConfigMap Integration

```yaml
spec:
  server:
    podOverrides:
      volumes:
      - name: "llamastack-config"
        configMap:
          name: "my-llamastack-config"
      volumeMounts:
      - name: "llamastack-config"
        mountPath: "/app/config"
```

## Configuration Examples

### Minimal Configuration

```yaml
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: simple-llamastack
spec:
  server:
    distribution:
      name: "ollama"
```

### Development Configuration

```yaml
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: llamastack-dev
spec:
  replicas: 1
  server:
    distribution:
      image: "llamastack/llamastack:latest"
    containerSpec:
      port: 8321
      resources:
        requests:
          cpu: "500m"
          memory: "1Gi"
        limits:
          cpu: "1"
          memory: "2Gi"
      env:
      - name: "LOG_LEVEL"
        value: "DEBUG"
    storage:
      size: "20Gi"
```

### Production Configuration

```yaml
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: llamastack-prod
spec:
  replicas: 3
  server:
    distribution:
      image: "llamastack/llamastack:v1.0.0"
    containerSpec:
      name: "llama-stack"
      port: 8321
      resources:
        requests:
          cpu: "2"
          memory: "4Gi"
        limits:
          cpu: "4"
          memory: "8Gi"
      env:
      - name: "INFERENCE_MODEL"
        value: "llama2-70b"
      - name: "MAX_WORKERS"
        value: "4"
    storage:
      size: "500Gi"
      mountPath: "/.llama"
    podOverrides:
      volumes:
      - name: "model-cache"
        emptyDir:
          sizeLimit: "100Gi"
      volumeMounts:
      - name: "model-cache"
        mountPath: "/cache"
```

### Custom Image with Configuration

```yaml
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: custom-llamastack
spec:
  replicas: 2
  server:
    distribution:
      image: "myregistry.com/custom-llamastack:v1.0"
    containerSpec:
      port: 8321
      resources:
        requests:
          cpu: "1"
          memory: "2Gi"
        limits:
          cpu: "2"
          memory: "4Gi"
      env:
      - name: "CUSTOM_CONFIG"
        value: "/config/custom.yaml"
    storage:
      size: "100Gi"
    podOverrides:
      volumes:
      - name: "custom-config"
        configMap:
          name: "llamastack-custom-config"
      volumeMounts:
      - name: "custom-config"
        mountPath: "/config"
        readOnly: true
```

### Distribution-Specific Examples

#### Ollama Distribution

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
      - name: OLLAMA_URL
        value: "http://ollama-server-service.ollama-dist.svc.cluster.local:11434"
    storage:
      size: "20Gi"
```

#### Hugging Face Endpoint

```yaml
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: hf-endpoint-llamastack
spec:
  server:
    distribution:
      name: "hf-endpoint"
    containerSpec:
      env:
      - name: HF_TOKEN
        valueFrom:
          secretKeyRef:
            name: hf-credentials
            key: token
      - name: HF_MODEL_ID
        value: "meta-llama/Llama-2-7b-chat-hf"
```

#### NVIDIA Distribution

```yaml
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: nvidia-llamastack
spec:
  server:
    distribution:
      name: "nvidia"
    containerSpec:
      resources:
        requests:
          nvidia.com/gpu: "1"
        limits:
          nvidia.com/gpu: "1"
      env:
      - name: NVIDIA_API_KEY
        valueFrom:
          secretKeyRef:
            name: nvidia-credentials
            key: api-key
```

#### vLLM GPU Distribution

```yaml
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: vllm-gpu-llamastack
spec:
  server:
    distribution:
      name: "vllm-gpu"
    containerSpec:
      resources:
        requests:
          nvidia.com/gpu: "1"
          memory: "8Gi"
        limits:
          nvidia.com/gpu: "1"
          memory: "16Gi"
      env:
      - name: MODEL_NAME
        value: "meta-llama/Llama-2-7b-chat-hf"
    storage:
      size: "50Gi"
```

#### AWS Bedrock Distribution

```yaml
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: bedrock-llamastack
spec:
  server:
    distribution:
      name: "bedrock"
    containerSpec:
      env:
      - name: AWS_REGION
        value: "us-east-1"
      - name: AWS_ACCESS_KEY_ID
        valueFrom:
          secretKeyRef:
            name: aws-credentials
            key: access-key-id
      - name: AWS_SECRET_ACCESS_KEY
        valueFrom:
          secretKeyRef:
            name: aws-credentials
            key: secret-access-key
```

#### Together AI Distribution

```yaml
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: together-llamastack
spec:
  server:
    distribution:
      name: "together"
    containerSpec:
      env:
      - name: TOGETHER_API_KEY
        valueFrom:
          secretKeyRef:
            name: together-credentials
            key: api-key
      - name: MODEL_NAME
        value: "meta-llama/Llama-2-7b-chat-hf"
```

## Status Information

The operator provides status information about the distribution:

```yaml
status:
  version: "1.0.0"
  ready: true
  distributionConfig:
    activeDistribution: "meta-reference"
    providers:
    - api: "inference"
      provider_id: "meta-reference"
      provider_type: "inference"
    availableDistributions:
      "meta-reference": "llamastack/llamastack:latest"
```

## Constants and Defaults

The API defines several constants:

- **Default Container Name**: `llama-stack`
- **Default Server Port**: `8321`
- **Default Service Port Name**: `http`
- **Default Mount Path**: `/.llama`
- **Default Storage Size**: `10Gi`
- **Default Label Key**: `app`
- **Default Label Value**: `llama-stack`

## Validation Rules

The API includes validation:

- **Distribution**: Only one of `name` or `image` can be specified
- **Port**: Must be a valid port number
- **Resources**: Follow Kubernetes resource requirements format
- **Storage Size**: Must be a valid Kubernetes quantity

## Next Steps

- [API Reference](api.md)
- [CLI Reference](cli.md)
- [How-to Guides](../how-to/deploy-llamastack.md)
