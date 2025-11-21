# Installation Guide

This guide walks you through installing the LlamaStack Kubernetes Operator in your cluster.

## Prerequisites

Before installing the operator, ensure you have:

- **Kubernetes cluster** (version 1.25 or later)
- **kubectl** configured to access your cluster
- **Cluster admin permissions** to install CRDs and RBAC resources
- **Container runtime** that supports pulling images from public registries

## Installation Methods

### Method 1: Kustomize (Recommended)

The recommended way to install the operator is using Kustomize:

```bash
# Clone the repository
git clone https://github.com/llamastack/llama-stack-k8s-operator.git
cd llama-stack-k8s-operator

# Install using Kustomize
kubectl apply -k config/default
```

This will:
- Install the Custom Resource Definitions (CRDs)
- Create the necessary RBAC resources
- Deploy the operator in the `llama-stack-k8s-operator-system` namespace

### Method 2: Build from Source

For development or customized builds:

```bash
# Clone the repository
git clone https://github.com/llamastack/llama-stack-k8s-operator.git
cd llama-stack-k8s-operator

# Build and deploy
make docker-build docker-push IMG=<your-registry>/llama-stack-k8s-operator:tag
make deploy IMG=<your-registry>/llama-stack-k8s-operator:tag
```

## Verification

After installation, verify that the operator is running:

```bash
# Check operator deployment
kubectl get deployment -n llama-stack-k8s-operator-system llama-stack-k8s-operator-controller-manager

# Check operator logs
kubectl logs -n llama-stack-k8s-operator-system deployment/llama-stack-k8s-operator-controller-manager

# Verify CRDs are installed
kubectl get crd llamastackdistributions.llamastack.io
```

Expected output:
```
NAME                                        CREATED AT
llamastackdistributions.llamastack.io      2024-01-15T10:30:00Z
```

## Configuration

### Resource Requirements

The operator has minimal resource requirements:

```yaml
resources:
  limits:
    cpu: 500m
    memory: 128Mi
  requests:
    cpu: 10m
    memory: 64Mi
```

### Environment Variables

Configure the operator behavior using environment variables:

| Variable | Description | Default |
|----------|-------------|---------|
| `METRICS_BIND_ADDRESS` | Metrics server bind address | `:8080` |
| `HEALTH_PROBE_BIND_ADDRESS` | Health probe bind address | `:8081` |
| `LEADER_ELECT` | Enable leader election | `false` |
| `LOG_LEVEL` | Logging level | `info` |

### Custom Configuration

For custom configurations, create a `kustomization.yaml`:

```yaml
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization

resources:
- https://github.com/llamastack/llama-stack-k8s-operator/config/default

patchesStrategicMerge:
- manager_config_patch.yaml

images:
- name: quay.io/llamastack/llama-stack-k8s-operator
  newTag: v0.1.0
```

## Namespace Configuration

### Default Namespace

By default, the operator watches all namespaces. To restrict to specific namespaces:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: llamastack-operator-controller-manager
spec:
  template:
    spec:
      containers:
      - name: manager
        env:
        - name: WATCH_NAMESPACE
          value: "llamastack-system,production"
```

### Multi-tenant Setup

For multi-tenant environments, install the operator with namespace restrictions:

```bash
# Install operator in tenant namespace
kubectl create namespace tenant-a
kubectl apply -f operator.yaml -n tenant-a

# Configure RBAC for tenant isolation
kubectl apply -f tenant-rbac.yaml
```

## Security Configuration

### RBAC

The operator requires the following permissions:

```yaml
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: llamastack-operator-manager-role
rules:
- apiGroups: ["llamastack.io"]
  resources: ["llamastackdistributions"]
  verbs: ["create", "delete", "get", "list", "patch", "update", "watch"]
- apiGroups: ["apps"]
  resources: ["deployments"]
  verbs: ["create", "delete", "get", "list", "patch", "update", "watch"]
- apiGroups: [""]
  resources: ["services", "configmaps", "persistentvolumeclaims"]
  verbs: ["create", "delete", "get", "list", "patch", "update", "watch"]
```

### Network Policies

Secure your deployment with network policies:

```yaml
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: llamastack-operator-netpol
  namespace: llama-stack-k8s-operator-system
spec:
  podSelector:
    matchLabels:
      control-plane: controller-manager
  policyTypes:
  - Ingress
  - Egress
  ingress:
  - from:
    - namespaceSelector: {}
    ports:
    - protocol: TCP
      port: 8080
    - protocol: TCP
      port: 8081
```

## Troubleshooting

### Common Issues

**1. CRD Installation Failed**
```bash
# Check if CRDs exist
kubectl get crd | grep llamastack

# Manually install CRDs
kubectl apply -f https://raw.githubusercontent.com/llamastack/llama-stack-k8s-operator/main/config/crd/bases/llamastack.io_llamastackdistributions.yaml
```

**2. Operator Pod Not Starting**
```bash
# Check pod status
kubectl get pods -n llama-stack-k8s-operator-system

# Check events
kubectl describe pod -n llama-stack-k8s-operator-system <pod-name>

# Check logs
kubectl logs -n llama-stack-k8s-operator-system <pod-name>
```

**3. Permission Denied Errors**
```bash
# Check RBAC configuration
kubectl auth can-i create llamastackdistributions --as=system:serviceaccount:llama-stack-k8s-operator-system:llama-stack-k8s-operator-controller-manager

# Verify service account
kubectl get serviceaccount -n llama-stack-k8s-operator-system
```

### Debug Mode

Enable debug logging for troubleshooting:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: llamastack-operator-controller-manager
spec:
  template:
    spec:
      containers:
      - name: manager
        env:
        - name: LOG_LEVEL
          value: "debug"
```

## Uninstallation

To remove the operator:

```bash
# Delete operator deployment
kubectl delete -f https://github.com/llamastack/llama-stack-k8s-operator/releases/latest/download/operator.yaml

# Clean up CRDs (this will delete all LlamaStackDistribution resources)
kubectl delete crd llamastackdistributions.llamastack.io
```

!!! warning "Data Loss Warning"
    Deleting the CRD will remove all LlamaStackDistribution resources and their associated data. Make sure to backup any important configurations before uninstalling.

## Next Steps

After successful installation:

1. [Deploy your first LlamaStack instance](quick-start.md)
2. [Learn about configuration options](configuration.md)
3. [Explore examples](../examples/basic-deployment.md)
