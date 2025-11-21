# AWS Bedrock Distribution

!!! warning "Distribution Availability"
    The Bedrock distribution container image may not be currently maintained or available.
    Please verify the image exists at `docker.io/llamastack/distribution-bedrock:latest` before using this distribution.
    For production use, consider using the `ollama` or `vllm` distributions which are actively maintained.

The **Bedrock** distribution enables seamless integration with Amazon Bedrock, AWS's fully managed service for foundation models. This distribution allows you to leverage AWS Bedrock's powerful models through the LlamaStack Kubernetes Operator.

## Overview

Amazon Bedrock provides access to high-performing foundation models from leading AI companies through a single API. The Bedrock distribution:

- **Connects to AWS Bedrock** for model inference
- **Manages AWS credentials** securely
- **Provides unified API** through LlamaStack
- **Supports multiple Bedrock models** (Claude, Llama, Titan, etc.)

## Distribution Details

| Property | Value |
|----------|-------|
| **Distribution Name** | `bedrock` |
| **Image** | `docker.io/llamastack/distribution-bedrock:latest` |
| **Use Case** | AWS Bedrock model integration |
| **Requirements** | AWS credentials and Bedrock access |
| **Recommended For** | AWS users, enterprise deployments |

## Prerequisites

### 1. AWS Account Setup

- AWS account with Bedrock access
- IAM user/role with Bedrock permissions
- Bedrock model access enabled in your AWS region

### 2. Required AWS Permissions

Your AWS credentials need the following permissions:

```json
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Effect": "Allow",
            "Action": [
                "bedrock:InvokeModel",
                "bedrock:InvokeModelWithResponseStream",
                "bedrock:ListFoundationModels",
                "bedrock:GetFoundationModel"
            ],
            "Resource": "*"
        }
    ]
}
```

### 3. Enable Bedrock Models

Enable the models you want to use in the AWS Bedrock console:
- Anthropic Claude models
- Meta Llama models  
- Amazon Titan models
- Cohere Command models

## Quick Start

### 1. Create AWS Credentials Secret

```bash
kubectl create secret generic aws-credentials \
  --from-literal=AWS_ACCESS_KEY_ID=your-access-key \
  --from-literal=AWS_SECRET_ACCESS_KEY=your-secret-key \
  --from-literal=AWS_DEFAULT_REGION=us-east-1
```

### 2. Create Bedrock Distribution

```yaml
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: my-bedrock-llamastack
  namespace: default
spec:
  replicas: 1
  server:
    distribution:
      name: "bedrock"
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
        - name: AWS_ACCESS_KEY_ID
          valueFrom:
            secretKeyRef:
              name: aws-credentials
              key: AWS_ACCESS_KEY_ID
        - name: AWS_SECRET_ACCESS_KEY
          valueFrom:
            secretKeyRef:
              name: aws-credentials
              key: AWS_SECRET_ACCESS_KEY
        - name: AWS_DEFAULT_REGION
          valueFrom:
            secretKeyRef:
              name: aws-credentials
              key: AWS_DEFAULT_REGION
        - name: BEDROCK_MODEL_ID
          value: "anthropic.claude-3-sonnet-20240229-v1:0"
    storage:
      size: "10Gi"
```

### 3. Deploy the Distribution

```bash
kubectl apply -f bedrock-distribution.yaml
```

### 4. Verify Deployment

```bash
# Check the distribution status
kubectl get llamastackdistribution my-bedrock-llamastack

# Check the pods
kubectl get pods -l app=llama-stack

# Check logs for AWS connection
kubectl logs -l app=llama-stack
```

## Configuration Options

### Supported Bedrock Models

Configure different Bedrock models using the `BEDROCK_MODEL_ID` environment variable:

#### Anthropic Claude Models
```yaml
env:
  - name: BEDROCK_MODEL_ID
    value: "anthropic.claude-3-sonnet-20240229-v1:0"  # Claude 3 Sonnet
  # value: "anthropic.claude-3-haiku-20240307-v1:0"   # Claude 3 Haiku
  # value: "anthropic.claude-v2:1"                     # Claude 2.1
```

#### Meta Llama Models
```yaml
env:
  - name: BEDROCK_MODEL_ID
    value: "meta.llama2-70b-chat-v1"  # Llama 2 70B Chat
  # value: "meta.llama2-13b-chat-v1"  # Llama 2 13B Chat
```

#### Amazon Titan Models
```yaml
env:
  - name: BEDROCK_MODEL_ID
    value: "amazon.titan-text-express-v1"  # Titan Text Express
  # value: "amazon.titan-text-lite-v1"     # Titan Text Lite
```

### AWS Authentication Methods

#### Method 1: Access Keys (Secrets)
```yaml
env:
  - name: AWS_ACCESS_KEY_ID
    valueFrom:
      secretKeyRef:
        name: aws-credentials
        key: AWS_ACCESS_KEY_ID
  - name: AWS_SECRET_ACCESS_KEY
    valueFrom:
      secretKeyRef:
        name: aws-credentials
        key: AWS_SECRET_ACCESS_KEY
  - name: AWS_DEFAULT_REGION
    value: "us-east-1"
```

#### Method 2: IAM Roles for Service Accounts (IRSA)
```yaml
spec:
  server:
    podOverrides:
      serviceAccountName: bedrock-service-account
      annotations:
        eks.amazonaws.com/role-arn: arn:aws:iam::123456789012:role/BedrockRole
```

#### Method 3: Instance Profile (EKS Nodes)
```yaml
# No additional configuration needed if EKS nodes have Bedrock permissions
env:
  - name: AWS_DEFAULT_REGION
    value: "us-east-1"
```

### Environment Variables

```yaml
env:
  - name: BEDROCK_MODEL_ID
    value: "anthropic.claude-3-sonnet-20240229-v1:0"
  - name: AWS_DEFAULT_REGION
    value: "us-east-1"
  - name: BEDROCK_MAX_TOKENS
    value: "4096"
  - name: BEDROCK_TEMPERATURE
    value: "0.7"
  - name: LOG_LEVEL
    value: "INFO"
```

## Advanced Configuration

### Multi-Model Setup

Deploy multiple Bedrock distributions for different models:

```yaml
# Claude 3 Sonnet Distribution
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: bedrock-claude-sonnet
spec:
  server:
    distribution:
      name: "bedrock"
    containerSpec:
      env:
        - name: BEDROCK_MODEL_ID
          value: "anthropic.claude-3-sonnet-20240229-v1:0"
---
# Llama 2 70B Distribution  
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: bedrock-llama2-70b
spec:
  server:
    distribution:
      name: "bedrock"
    containerSpec:
      env:
        - name: BEDROCK_MODEL_ID
          value: "meta.llama2-70b-chat-v1"
```

### Production Configuration

```yaml
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: production-bedrock
  namespace: production
spec:
  replicas: 3
  server:
    distribution:
      name: "bedrock"
    containerSpec:
      resources:
        requests:
          memory: "2Gi"
          cpu: "1"
        limits:
          memory: "4Gi"
          cpu: "2"
      env:
        - name: AWS_ACCESS_KEY_ID
          valueFrom:
            secretKeyRef:
              name: aws-credentials
              key: AWS_ACCESS_KEY_ID
        - name: AWS_SECRET_ACCESS_KEY
          valueFrom:
            secretKeyRef:
              name: aws-credentials
              key: AWS_SECRET_ACCESS_KEY
        - name: AWS_DEFAULT_REGION
          value: "us-east-1"
        - name: BEDROCK_MODEL_ID
          value: "anthropic.claude-3-sonnet-20240229-v1:0"
        - name: LOG_LEVEL
          value: "WARNING"
        - name: ENABLE_TELEMETRY
          value: "true"
    storage:
      size: "20Gi"
```

## Use Cases

### 1. Enterprise AI Applications

```yaml
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: enterprise-bedrock
  namespace: enterprise
spec:
  replicas: 5
  server:
    distribution:
      name: "bedrock"
    containerSpec:
      resources:
        requests:
          memory: "4Gi"
          cpu: "2"
        limits:
          memory: "8Gi"
          cpu: "4"
      env:
        - name: BEDROCK_MODEL_ID
          value: "anthropic.claude-3-sonnet-20240229-v1:0"
        - name: AWS_DEFAULT_REGION
          value: "us-east-1"
```

### 2. Development and Testing

```yaml
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: dev-bedrock
  namespace: development
spec:
  replicas: 1
  server:
    distribution:
      name: "bedrock"
    containerSpec:
      resources:
        requests:
          memory: "512Mi"
          cpu: "250m"
        limits:
          memory: "1Gi"
          cpu: "500m"
      env:
        - name: BEDROCK_MODEL_ID
          value: "anthropic.claude-3-haiku-20240307-v1:0"  # Faster, cheaper model
        - name: LOG_LEVEL
          value: "DEBUG"
```

## Monitoring and Troubleshooting

### Health Checks

```bash
# Check distribution status
kubectl get llamastackdistribution

# Check pod logs for AWS connectivity
kubectl logs -l app=llama-stack

# Test AWS credentials
kubectl exec -it <pod-name> -- aws bedrock list-foundation-models
```

### Common Issues

1. **AWS Credentials Invalid**
   ```bash
   # Verify credentials in secret
   kubectl get secret aws-credentials -o yaml
   
   # Test credentials
   kubectl exec -it <pod-name> -- aws sts get-caller-identity
   ```

2. **Model Access Denied**
   - Enable model access in AWS Bedrock console
   - Verify IAM permissions include `bedrock:InvokeModel`
   - Check if model is available in your AWS region

3. **Region Issues**
   - Ensure Bedrock is available in your region
   - Verify `AWS_DEFAULT_REGION` matches model availability

### Cost Monitoring

Monitor AWS Bedrock costs:
- Use AWS Cost Explorer to track Bedrock usage
- Set up billing alerts for unexpected usage
- Consider using cheaper models for development

## Best Practices

### Security
- Use IAM roles instead of access keys when possible
- Store credentials in Kubernetes Secrets
- Implement least-privilege IAM policies
- Enable AWS CloudTrail for audit logging

### Performance
- Choose appropriate models for your use case
- Use Haiku for speed, Sonnet for balance, Opus for quality
- Scale replicas based on request volume
- Monitor response times and adjust accordingly

### Cost Optimization
- Use smaller models for development/testing
- Implement request caching where appropriate
- Monitor token usage and optimize prompts
- Set up cost alerts and budgets

## Next Steps

- [Configure Storage](../how-to-guides/storage.md)
- [Set up Monitoring](../how-to-guides/monitoring.md)
- [Scaling Guide](../how-to-guides/scaling.md)
- [Security Best Practices](../how-to-guides/security.md)

## API Reference

For complete API documentation, see:
- [API Reference](../reference/api.md)
- [Configuration Reference](../reference/configuration.md)
