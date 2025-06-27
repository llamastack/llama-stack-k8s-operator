# Understanding LlamaStack Distributions

This guide explains the different ways to deploy LlamaStack using the Kubernetes operator, focusing on the distinction between **Supported Distributions** and **Bring-Your-Own (BYO) Distributions**.

## Distribution Types Overview

The LlamaStack Kubernetes Operator supports two main approaches for deploying LlamaStack:

### 🎯 **Supported Distributions** (Recommended)
Pre-configured, tested distributions maintained by the LlamaStack team with specific provider integrations.

### 🛠️ **Bring-Your-Own (BYO) Distributions**
Custom container images that you build and maintain yourself.

## Supported Distributions

### What are Supported Distributions?

Supported distributions are **pre-built, tested container images** that include:
- ✅ **Specific provider integrations** (Ollama, vLLM, NVIDIA, etc.)
- ✅ **Optimized configurations** for each provider
- ✅ **Tested compatibility** with the operator
- ✅ **Regular updates** and security patches
- ✅ **Documentation and examples**

### Available Pre-Built Distributions

#### Self-Hosted Distributions
These require you to provide the underlying infrastructure:

| Distribution | Provider | Use Case | Infrastructure Required |
|--------------|----------|----------|------------------------|
| `ollama` | Ollama | Local inference with Ollama server | Ollama server |
| `vllm-gpu` | vLLM | High-performance GPU inference | GPU infrastructure |
| `tgi` | Text Generation Inference | Hugging Face TGI | TGI server setup |
| `nvidia` | NVIDIA | NVIDIA AI services | NVIDIA infrastructure |
| `remote-vllm` | Remote vLLM | Remote vLLM server | External vLLM server |
| `open-benchmark` | Benchmarking | Performance testing | Testing infrastructure |

#### External API Distributions
These only require API keys to external services:

| Distribution | Provider | Use Case | Requirements |
|--------------|----------|----------|--------------|
| `hf-endpoint` | Hugging Face | Hugging Face Inference Endpoints | HF API key |
| `hf-serverless` | Hugging Face | Hugging Face Serverless | HF API key |
| `bedrock` | AWS Bedrock | AWS Bedrock models | AWS credentials |
| `together` | Together AI | Together AI API | Together API key |
| `fireworks` | Fireworks AI | Fireworks AI API | Fireworks API key |
| `cerebras` | Cerebras | Cerebras inference | Cerebras API key |
| `sambanova` | SambaNova | SambaNova inference | SambaNova API key |
| `watsonx` | IBM watsonx | IBM watsonx models | IBM API key |
| `passthrough` | Generic | API passthrough | Target API access |

### Using Supported Distributions

#### Basic Syntax

```yaml
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: my-distribution
spec:
  server:
    distribution:
      name: "distribution-name"  # Use distribution name
    # ... other configuration
```

#### Example: Ollama Distribution

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
      resources:
        requests:
          cpu: "1"
          memory: "2Gi"
        limits:
          cpu: "2"
          memory: "4Gi"
      env:
      - name: OLLAMA_URL
        value: "http://ollama-server-service.ollama-dist.svc.cluster.local:11434"
    storage:
      size: "20Gi"
```

#### Example: vLLM GPU Distribution

```yaml
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: vllm-gpu-llamastack
spec:
  replicas: 1
  server:
    distribution:
      name: "vllm-gpu"
    containerSpec:
      port: 8321
      resources:
        requests:
          cpu: "2"
          memory: "8Gi"
          nvidia.com/gpu: "1"
        limits:
          cpu: "4"
          memory: "16Gi"
          nvidia.com/gpu: "1"
      env:
      - name: MODEL_NAME
        value: "meta-llama/Llama-2-7b-chat-hf"
      - name: TENSOR_PARALLEL_SIZE
        value: "1"
    storage:
      size: "50Gi"
```

### Benefits of Supported Distributions

- **🚀 Quick Setup**: No need to build custom images
- **🔒 Security**: Regular security updates from LlamaStack team
- **📚 Documentation**: Comprehensive guides and examples
- **🧪 Tested**: Thoroughly tested with the operator
- **🔧 Optimized**: Pre-configured for optimal performance
- **🆘 Support**: Community and official support available

## Bring-Your-Own (BYO) Distributions

### What are BYO Distributions?

BYO distributions allow you to use **custom container images** that you build and maintain:
- 🛠️ **Custom integrations** not available in supported distributions
- 🎨 **Specialized configurations** for your use case
- 🔧 **Custom dependencies** and libraries
- 📦 **Private or proprietary** model integrations

### Using BYO Distributions

#### Basic Syntax

```yaml
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: my-custom-distribution
spec:
  server:
    distribution:
      image: "your-registry.com/custom-llamastack:tag"  # Use custom image
    # ... other configuration
```

#### Example: Custom Image

```yaml
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: custom-llamastack
spec:
  replicas: 1
  server:
    distribution:
      image: "myregistry.com/custom-llamastack:v1.0.0"
    containerSpec:
      port: 8321
      resources:
        requests:
          cpu: "2"
          memory: "4Gi"
        limits:
          cpu: "4"
          memory: "8Gi"
      env:
      - name: CUSTOM_CONFIG_PATH
        value: "/app/config/custom.yaml"
      - name: API_KEY
        valueFrom:
          secretKeyRef:
            name: custom-credentials
            key: api-key
    storage:
      size: "100Gi"
    podOverrides:
      volumes:
      - name: custom-config
        configMap:
          name: custom-llamastack-config
      volumeMounts:
      - name: custom-config
        mountPath: "/app/config"
        readOnly: true
```

### Building Custom Images

#### Example Dockerfile

```dockerfile
# Start from a supported distribution or base image
FROM llamastack/llamastack:latest

# Add your custom dependencies
RUN pip install custom-package-1 custom-package-2

# Copy custom configuration
COPY custom-config/ /app/config/

# Copy custom code
COPY src/ /app/src/

# Set custom environment variables
ENV CUSTOM_SETTING=value

# Override entrypoint if needed
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh
ENTRYPOINT ["/entrypoint.sh"]
```

#### Building and Pushing

```bash
# Build your custom image
docker build -t myregistry.com/custom-llamastack:v1.0.0 .

# Push to your registry
docker push myregistry.com/custom-llamastack:v1.0.0
```

### BYO Distribution Considerations

#### Advantages
- **🎯 Full Control**: Complete customization of the stack
- **🔧 Custom Integrations**: Add proprietary or specialized providers
- **📦 Private Models**: Include private or fine-tuned models
- **⚡ Optimizations**: Custom performance optimizations

#### Responsibilities
- **🔒 Security**: You maintain security updates
- **🧪 Testing**: You test compatibility with the operator
- **📚 Documentation**: You document your custom setup
- **🆘 Support**: Limited community support for custom images
- **🔄 Updates**: You manage updates and compatibility

## Key Differences Summary

| Aspect | Supported Distributions | BYO Distributions |
|--------|------------------------|-------------------|
| **Setup Complexity** | ✅ Simple (just specify name) | 🔧 Complex (build & maintain image) |
| **Maintenance** | ✅ Handled by LlamaStack team | ❌ Your responsibility |
| **Security Updates** | ✅ Automatic | ❌ Manual |
| **Documentation** | ✅ Comprehensive | ❌ You create |
| **Support** | ✅ Community + Official | ⚠️ Limited |
| **Customization** | ⚠️ Limited to configuration | ✅ Full control |
| **Testing** | ✅ Pre-tested | ❌ You test |
| **Time to Deploy** | ✅ Minutes | ⏱️ Hours/Days |

## Choosing the Right Approach

### Use Supported Distributions When:
- ✅ Your use case matches available providers (Ollama, vLLM, etc.)
- ✅ You want quick setup and deployment
- ✅ You prefer maintained and tested solutions
- ✅ You need community support
- ✅ Security and updates are important

### Use BYO Distributions When:
- 🛠️ You need custom provider integrations
- 🔧 You have specialized requirements
- 📦 You use proprietary or private models
- ⚡ You need specific performance optimizations
- 🎯 You have the expertise to maintain custom images

## Migration Between Approaches

### From Supported to BYO
```yaml
# Before (supported)
spec:
  server:
    distribution:
      name: "ollama"

# After (BYO)
spec:
  server:
    distribution:
      image: "myregistry.com/custom-ollama:v1.0.0"
```

### From BYO to Supported
```yaml
# Before (BYO)
spec:
  server:
    distribution:
      image: "myregistry.com/custom-vllm:v1.0.0"

# After (supported)
spec:
  server:
    distribution:
      name: "vllm-gpu"
```

## Best Practices

### For Supported Distributions
1. **Start Simple**: Begin with basic configuration
2. **Use Environment Variables**: Configure via `env` section
3. **Monitor Resources**: Set appropriate resource limits
4. **Check Documentation**: Review provider-specific guides

### For BYO Distributions
1. **Base on Supported Images**: Start from `llamastack/llamastack:latest`
2. **Document Everything**: Maintain clear documentation
3. **Test Thoroughly**: Test with the operator before production
4. **Version Control**: Tag and version your custom images
5. **Security Scanning**: Regularly scan for vulnerabilities

## Next Steps

- [Configuration Reference](../reference/configuration.md) - Detailed configuration options
- [Basic Deployment](../examples/basic-deployment.md) - Simple deployment examples
- [Production Setup](../examples/production-setup.md) - Production-ready configurations
- [Custom Images Guide](../examples/custom-images.md) - Building custom images
- [Troubleshooting](../how-to/troubleshooting.md) - Common issues and solutions