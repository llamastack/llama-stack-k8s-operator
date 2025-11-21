# Together AI Distribution

!!! warning "Distribution Availability"
    The Together distribution container image may not be currently maintained or available.
    Please verify the image exists at `docker.io/llamastack/distribution-together:latest` before using this distribution.
    For production use, consider using the `ollama` or `vllm` distributions which are actively maintained.

The **Together** distribution integrates with Together AI's inference platform, providing access to a wide variety of open-source models through their optimized API service.

## Overview

Together AI offers fast, scalable inference for open-source language models. The Together distribution:

- **Connects to Together AI API** for model inference
- **Supports multiple open-source models** (Llama, Mistral, CodeLlama, etc.)
- **Provides high-performance inference** with optimized serving
- **Offers cost-effective scaling** with pay-per-use pricing

## Distribution Details

| Property | Value |
|----------|-------|
| **Distribution Name** | `together` |
| **Image** | `docker.io/llamastack/distribution-together:latest` |
| **Use Case** | Together AI API integration |
| **Requirements** | Together AI API key |
| **Recommended For** | Open-source models, cost-effective inference |

## Prerequisites

### 1. Together AI Account

- Sign up at [together.ai](https://together.ai)
- Get your API key from the dashboard
- Choose your preferred models

### 2. API Key Setup

Create a Kubernetes secret with your Together AI API key:

```bash
kubectl create secret generic together-api-key \
  --from-literal=TOGETHER_API_KEY=your-api-key-here
```

## Quick Start

### 1. Create Together Distribution

```yaml
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: my-together-llamastack
  namespace: default
spec:
  replicas: 1
  server:
    distribution:
      name: "together"
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
        - name: TOGETHER_API_KEY
          valueFrom:
            secretKeyRef:
              name: together-api-key
              key: TOGETHER_API_KEY
        - name: TOGETHER_MODEL
          value: "meta-llama/Llama-2-7b-chat-hf"
    storage:
      size: "10Gi"
```

### 2. Deploy the Distribution

```bash
kubectl apply -f together-distribution.yaml
```

### 3. Verify Deployment

```bash
# Check the distribution status
kubectl get llamastackdistribution my-together-llamastack

# Check the pods
kubectl get pods -l app=llama-stack

# Check logs for Together AI connectivity
kubectl logs -l app=llama-stack
```

## Configuration Options

### Supported Models

Together AI supports many popular open-source models:

#### Meta Llama Models
```yaml
env:
  - name: TOGETHER_MODEL
    value: "meta-llama/Llama-2-7b-chat-hf"
  # value: "meta-llama/Llama-2-13b-chat-hf"
  # value: "meta-llama/Llama-2-70b-chat-hf"
  # value: "meta-llama/CodeLlama-7b-Instruct-hf"
  # value: "meta-llama/CodeLlama-13b-Instruct-hf"
```

#### Mistral Models
```yaml
env:
  - name: TOGETHER_MODEL
    value: "mistralai/Mistral-7B-Instruct-v0.1"
  # value: "mistralai/Mixtral-8x7B-Instruct-v0.1"
```

#### Other Popular Models
```yaml
env:
  - name: TOGETHER_MODEL
    value: "togethercomputer/RedPajama-INCITE-7B-Chat"
  # value: "NousResearch/Nous-Hermes-2-Mixtral-8x7B-DPO"
  # value: "teknium/OpenHermes-2.5-Mistral-7B"
```

### Environment Variables

Configure Together AI connection and model parameters:

```yaml
env:
  - name: TOGETHER_API_KEY
    valueFrom:
      secretKeyRef:
        name: together-api-key
        key: TOGETHER_API_KEY
  - name: TOGETHER_MODEL
    value: "meta-llama/Llama-2-7b-chat-hf"
  - name: TOGETHER_MAX_TOKENS
    value: "512"
  - name: TOGETHER_TEMPERATURE
    value: "0.7"
  - name: TOGETHER_TOP_P
    value: "0.9"
  - name: TOGETHER_TOP_K
    value: "50"
  - name: TOGETHER_REPETITION_PENALTY
    value: "1.0"
  - name: TOGETHER_TIMEOUT
    value: "30"  # Request timeout in seconds
  - name: LOG_LEVEL
    value: "INFO"
```

### Resource Requirements

#### Development Setup
```yaml
resources:
  requests:
    memory: "512Mi"
    cpu: "250m"
  limits:
    memory: "1Gi"
    cpu: "500m"
```

#### Production Setup
```yaml
resources:
  requests:
    memory: "1Gi"
    cpu: "500m"
  limits:
    memory: "2Gi"
    cpu: "1"
```

#### High-Throughput Setup
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

### Multiple Models

Deploy different distributions for different models:

```yaml
# Llama 2 7B for general chat
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: together-llama2-7b
spec:
  server:
    distribution:
      name: "together"
    containerSpec:
      env:
        - name: TOGETHER_MODEL
          value: "meta-llama/Llama-2-7b-chat-hf"
---
# CodeLlama for code generation
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: together-codellama
spec:
  server:
    distribution:
      name: "together"
    containerSpec:
      env:
        - name: TOGETHER_MODEL
          value: "meta-llama/CodeLlama-7b-Instruct-hf"
```

### Production Configuration

```yaml
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: production-together
  namespace: production
spec:
  replicas: 3
  server:
    distribution:
      name: "together"
    containerSpec:
      resources:
        requests:
          memory: "2Gi"
          cpu: "1"
        limits:
          memory: "4Gi"
          cpu: "2"
      env:
        - name: TOGETHER_API_KEY
          valueFrom:
            secretKeyRef:
              name: together-api-key
              key: TOGETHER_API_KEY
        - name: TOGETHER_MODEL
          value: "meta-llama/Llama-2-13b-chat-hf"
        - name: TOGETHER_MAX_TOKENS
          value: "1024"
        - name: TOGETHER_TEMPERATURE
          value: "0.7"
        - name: TOGETHER_TIMEOUT
          value: "60"
        - name: LOG_LEVEL
          value: "WARNING"
        - name: ENABLE_TELEMETRY
          value: "true"
    storage:
      size: "20Gi"
```

### Custom Configuration with ConfigMap

```yaml
apiVersion: v1
kind: ConfigMap
metadata:
  name: together-config
data:
  together-settings.json: |
    {
      "default_model": "meta-llama/Llama-2-7b-chat-hf",
      "max_tokens": 512,
      "temperature": 0.7,
      "top_p": 0.9,
      "top_k": 50,
      "repetition_penalty": 1.0,
      "stop_sequences": ["</s>", "[INST]", "[/INST]"],
      "retry_config": {
        "max_retries": 3,
        "backoff_factor": 2,
        "max_backoff": 60
      }
    }
---
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: custom-together
spec:
  server:
    distribution:
      name: "together"
    containerSpec:
      env:
        - name: TOGETHER_CONFIG_FILE
          value: "/config/together-settings.json"
    podOverrides:
      volumes:
        - name: together-config
          configMap:
            name: together-config
      volumeMounts:
        - name: together-config
          mountPath: /config
```

## Use Cases

### 1. Development and Prototyping

```yaml
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: dev-together
  namespace: development
spec:
  replicas: 1
  server:
    distribution:
      name: "together"
    containerSpec:
      resources:
        requests:
          memory: "512Mi"
          cpu: "250m"
        limits:
          memory: "1Gi"
          cpu: "500m"
      env:
        - name: TOGETHER_API_KEY
          valueFrom:
            secretKeyRef:
              name: together-api-key
              key: TOGETHER_API_KEY
        - name: TOGETHER_MODEL
          value: "meta-llama/Llama-2-7b-chat-hf"
        - name: TOGETHER_MAX_TOKENS
          value: "256"
        - name: LOG_LEVEL
          value: "DEBUG"
    storage:
      size: "5Gi"
```

### 2. Code Generation Service

```yaml
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: code-generation-together
  namespace: default
spec:
  replicas: 2
  server:
    distribution:
      name: "together"
    containerSpec:
      resources:
        requests:
          memory: "1Gi"
          cpu: "500m"
        limits:
          memory: "2Gi"
          cpu: "1"
      env:
        - name: TOGETHER_API_KEY
          valueFrom:
            secretKeyRef:
              name: together-api-key
              key: TOGETHER_API_KEY
        - name: TOGETHER_MODEL
          value: "meta-llama/CodeLlama-13b-Instruct-hf"
        - name: TOGETHER_MAX_TOKENS
          value: "2048"
        - name: TOGETHER_TEMPERATURE
          value: "0.1"  # Lower temperature for code
    storage:
      size: "15Gi"
```

### 3. High-Volume Production

```yaml
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: high-volume-together
  namespace: production
spec:
  replicas: 5
  server:
    distribution:
      name: "together"
    containerSpec:
      resources:
        requests:
          memory: "2Gi"
          cpu: "1"
        limits:
          memory: "4Gi"
          cpu: "2"
      env:
        - name: TOGETHER_API_KEY
          valueFrom:
            secretKeyRef:
              name: together-api-key
              key: TOGETHER_API_KEY
        - name: TOGETHER_MODEL
          value: "meta-llama/Llama-2-70b-chat-hf"
        - name: TOGETHER_MAX_TOKENS
          value: "1024"
        - name: TOGETHER_TIMEOUT
          value: "120"
        - name: ENABLE_TELEMETRY
          value: "true"
    storage:
      size: "50Gi"
```

## Monitoring and Troubleshooting

### Health Checks

```bash
# Check distribution status
kubectl get llamastackdistribution

# Check API connectivity
kubectl logs -l app=llama-stack | grep -i together

# Test API key
kubectl exec -it <pod-name> -- curl -H "Authorization: Bearer $TOGETHER_API_KEY" \
  https://api.together.xyz/v1/models
```

### Performance Monitoring

```bash
# Monitor resource usage
kubectl top pods -l app=llama-stack

# Check API response times
kubectl logs -l app=llama-stack | grep -i "response_time"

# Monitor API usage
kubectl logs -l app=llama-stack | grep -i "api_usage"
```

### Common Issues

1. **Invalid API Key**
   ```bash
   # Verify API key in secret
   kubectl get secret together-api-key -o yaml
   
   # Test API key manually
   kubectl exec -it <pod-name> -- env | grep TOGETHER_API_KEY
   ```

2. **Model Not Available**
   - Check if model exists in Together AI catalog
   - Verify model name spelling and format
   - Some models may have usage restrictions

3. **Rate Limiting**
   - Monitor API usage and limits
   - Implement request queuing
   - Consider upgrading Together AI plan

4. **Timeout Issues**
   - Increase `TOGETHER_TIMEOUT` value
   - Check network connectivity
   - Monitor Together AI service status

## Best Practices

### Cost Optimization
- Choose appropriate models for your use case
- Monitor token usage and optimize prompts
- Use smaller models for development/testing
- Implement caching for repeated requests
- Set up usage alerts and budgets

### Performance
- Scale replicas based on request volume
- Use connection pooling and keep-alive
- Implement request batching where possible
- Monitor and optimize timeout values

### Security
- Store API keys in Kubernetes Secrets
- Use least-privilege access controls
- Monitor API usage for anomalies
- Rotate API keys regularly
- Implement rate limiting and request validation

### Reliability
- Implement retry logic with exponential backoff
- Use multiple replicas for high availability
- Monitor Together AI service status
- Have fallback mechanisms for service outages

## Cost Management

### Usage Monitoring
```yaml
env:
  - name: ENABLE_USAGE_TRACKING
    value: "true"
  - name: USAGE_LOG_LEVEL
    value: "INFO"
  - name: COST_ALERT_THRESHOLD
    value: "100"  # Alert when daily cost exceeds $100
```

### Budget Controls
- Set up billing alerts in Together AI dashboard
- Implement request quotas per user/application
- Monitor token usage patterns
- Use smaller models for non-critical workloads

## Next Steps

- [Configure Monitoring](../how-to-guides/monitoring.md)
- [Set up Scaling](../how-to-guides/scaling.md)
- [Security Best Practices](../how-to-guides/security.md)
- [Cost Optimization](../how-to-guides/cost-optimization.md)

## API Reference

For complete API documentation, see:
- [API Reference](../reference/api.md)
- [Configuration Reference](../reference/configuration.md)
- [Together AI API Documentation](https://docs.together.ai/)
