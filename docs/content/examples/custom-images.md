# Custom Images

Guide for building and using custom LlamaStack images with the Kubernetes operator.

## Building Custom Images

### Base Dockerfile

```dockerfile
FROM llamastack/llamastack:latest

# Add custom models
COPY models/ /models/

# Add custom configurations
COPY config/ /config/

# Install additional dependencies
RUN pip install custom-package

# Set custom entrypoint
COPY entrypoint.sh /entrypoint.sh
RUN chmod +x /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
```

### Multi-stage Build

```dockerfile
# Build stage
FROM python:3.11-slim as builder

WORKDIR /app
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# Runtime stage
FROM llamastack/llamastack:latest

COPY --from=builder /usr/local/lib/python3.11/site-packages /usr/local/lib/python3.11/site-packages
COPY custom-code/ /app/

CMD ["python", "/app/main.py"]
```

## Using Custom Images

### Basic Configuration

```yaml
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: custom-llamastack
spec:
  image: "myregistry.com/custom-llamastack:v1.0.0"
  imagePullPolicy: Always
  imagePullSecrets:
  - name: registry-credentials
```

### With Custom Configuration

```yaml
spec:
  image: "myregistry.com/llamastack-custom:latest"
  config:
    models:
    - name: "custom-model"
      path: "/models/custom-model"
      provider: "custom-provider"
```

## Next Steps

- [Production Setup](production-setup.md)
- [Basic Deployment](basic-deployment.md)
