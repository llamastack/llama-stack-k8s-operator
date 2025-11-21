# CLI Reference

Command-line interface reference for LlamaStack Kubernetes Operator.

## kubectl Commands

### Basic Operations

#### List Distributions

```bash
# List all LlamaStack distributions
kubectl get llamastackdistribution

# List with additional details
kubectl get llamastackdistribution -o wide

# List in all namespaces
kubectl get llamastackdistribution --all-namespaces
```

#### Describe Distribution

```bash
# Get detailed information
kubectl describe llamastackdistribution <name>

# Get YAML output
kubectl get llamastackdistribution <name> -o yaml

# Get JSON output
kubectl get llamastackdistribution <name> -o json
```

#### Create Distribution

```bash
# Create from file
kubectl apply -f llamastack-distribution.yaml

# Create from URL
kubectl apply -f https://example.com/llamastack.yaml

# Dry run to validate
kubectl apply -f llamastack.yaml --dry-run=client
```

#### Update Distribution

```bash
# Apply changes from file
kubectl apply -f llamastack-distribution.yaml

# Edit directly
kubectl edit llamastackdistribution <name>

# Patch specific fields
kubectl patch llamastackdistribution <name> -p '{"spec":{"replicas":3}}'
```

#### Delete Distribution

```bash
# Delete by name
kubectl delete llamastackdistribution <name>

# Delete from file
kubectl delete -f llamastack-distribution.yaml

# Delete all distributions
kubectl delete llamastackdistribution --all
```

### Advanced Operations

#### Scale Distribution

```bash
# Scale to specific replica count
kubectl scale llamastackdistribution <name> --replicas=5

# Scale multiple distributions
kubectl scale llamastackdistribution --all --replicas=3
```

#### Rollout Management

```bash
# Check rollout status
kubectl rollout status deployment/<name>

# Rollout history
kubectl rollout history deployment/<name>

# Rollback to previous version
kubectl rollout undo deployment/<name>

# Rollback to specific revision
kubectl rollout undo deployment/<name> --to-revision=2
```

#### Resource Management

```bash
# Get resource usage
kubectl top pods -l app=<distribution-name>

# Get node usage
kubectl top nodes

# Describe resource quotas
kubectl describe resourcequota
```

## Operator Management

### Installation

```bash
# Install operator
kubectl apply -f https://github.com/llamastack/llama-stack-k8s-operator/releases/latest/download/operator.yaml

# Install specific version
kubectl apply -f https://github.com/llamastack/llama-stack-k8s-operator/releases/download/v1.0.0/operator.yaml

# Install from local file
kubectl apply -f operator.yaml
```

### Operator Status

```bash
# Check operator pods
kubectl get pods -n llamastack-system

# Check operator logs
kubectl logs -n llamastack-system -l app=llamastack-operator

# Follow operator logs
kubectl logs -n llamastack-system -l app=llamastack-operator -f
```

### Operator Configuration

```bash
# Get operator configuration
kubectl get configmap -n llamastack-system llamastack-config -o yaml

# Update operator configuration
kubectl patch configmap -n llamastack-system llamastack-config -p '{"data":{"config.yaml":"..."}}'
```

## Debugging Commands

### Pod Operations

```bash
# List pods for a distribution
kubectl get pods -l app=<distribution-name>

# Get pod logs
kubectl logs <pod-name>

# Follow pod logs
kubectl logs -f <pod-name>

# Get previous container logs
kubectl logs <pod-name> --previous

# Execute commands in pod
kubectl exec -it <pod-name> -- /bin/bash

# Copy files to/from pod
kubectl cp <local-file> <pod-name>:<remote-path>
kubectl cp <pod-name>:<remote-path> <local-file>
```

### Service Operations

```bash
# List services
kubectl get svc -l app=<distribution-name>

# Describe service
kubectl describe svc <service-name>

# Get service endpoints
kubectl get endpoints <service-name>

# Port forward to service
kubectl port-forward svc/<service-name> 8080:8080
```

### Network Debugging

```bash
# Test DNS resolution
kubectl exec -it <pod-name> -- nslookup <service-name>

# Test connectivity
kubectl exec -it <pod-name> -- curl http://<service-name>:8080/health

# Check network policies
kubectl get networkpolicy
```

### Storage Operations

```bash
# List persistent volume claims
kubectl get pvc -l app=<distribution-name>

# Describe PVC
kubectl describe pvc <pvc-name>

# Check storage usage
kubectl exec -it <pod-name> -- df -h

# List storage classes
kubectl get storageclass
```

## Monitoring Commands

### Resource Monitoring

```bash
# Get resource usage for pods
kubectl top pods -l app=<distribution-name>

# Get resource usage for nodes
kubectl top nodes

# Get resource usage with containers
kubectl top pods -l app=<distribution-name> --containers
```

### Event Monitoring

```bash
# Get events for a distribution
kubectl get events --field-selector involvedObject.name=<distribution-name>

# Get recent events
kubectl get events --sort-by=.metadata.creationTimestamp

# Watch events in real-time
kubectl get events --watch
```

### Metrics Access

```bash
# Port forward to metrics endpoint
kubectl port-forward <pod-name> 9090:9090

# Access metrics via curl
kubectl exec -it <pod-name> -- curl http://localhost:9090/metrics
```

## Configuration Management

### ConfigMaps

```bash
# Create ConfigMap from file
kubectl create configmap llamastack-config --from-file=config.yaml

# Create ConfigMap from literal values
kubectl create configmap llamastack-config --from-literal=key1=value1

# Update ConfigMap
kubectl patch configmap llamastack-config -p '{"data":{"key":"new-value"}}'

# Get ConfigMap
kubectl get configmap llamastack-config -o yaml
```

### Secrets

```bash
# Create secret from file
kubectl create secret generic llamastack-secret --from-file=secret.txt

# Create secret from literal values
kubectl create secret generic llamastack-secret --from-literal=password=secret123

# Get secret (base64 encoded)
kubectl get secret llamastack-secret -o yaml

# Decode secret
kubectl get secret llamastack-secret -o jsonpath='{.data.password}' | base64 -d
```

## Backup and Recovery

### Backup

```bash
# Backup distribution configuration
kubectl get llamastackdistribution <name> -o yaml > backup.yaml

# Backup all distributions
kubectl get llamastackdistribution -o yaml > all-distributions-backup.yaml

# Create volume snapshot
kubectl create volumesnapshot <snapshot-name> --from-pvc=<pvc-name>
```

### Recovery

```bash
# Restore from backup
kubectl apply -f backup.yaml

# Restore from volume snapshot
kubectl apply -f - <<EOF
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: restored-pvc
spec:
  dataSource:
    name: <snapshot-name>
    kind: VolumeSnapshot
    apiGroup: snapshot.storage.k8s.io
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 50Gi
EOF
```

## Useful Aliases

Add these to your shell configuration:

```bash
# Basic aliases
alias k='kubectl'
alias kgd='kubectl get llamastackdistribution'
alias kdd='kubectl describe llamastackdistribution'
alias ked='kubectl edit llamastackdistribution'

# Pod aliases
alias kgp='kubectl get pods'
alias kdp='kubectl describe pod'
alias kl='kubectl logs'
alias kex='kubectl exec -it'

# Service aliases
alias kgs='kubectl get svc'
alias kds='kubectl describe svc'
alias kpf='kubectl port-forward'

# Monitoring aliases
alias ktop='kubectl top'
alias kge='kubectl get events --sort-by=.metadata.creationTimestamp'
```

## Bash Completion

Enable kubectl completion:

```bash
# For bash
echo 'source <(kubectl completion bash)' >>~/.bashrc

# For zsh
echo 'source <(kubectl completion zsh)' >>~/.zshrc

# For fish
kubectl completion fish | source
```

## Common Workflows

### Development Workflow

```bash
# 1. Create development distribution
kubectl apply -f dev-llamastack.yaml

# 2. Check status
kubectl get llamastackdistribution dev-llamastack

# 3. Check pods
kubectl get pods -l app=dev-llamastack

# 4. View logs
kubectl logs -f -l app=dev-llamastack

# 5. Test connectivity
kubectl port-forward svc/dev-llamastack 8080:8080

# 6. Update configuration
kubectl edit llamastackdistribution dev-llamastack

# 7. Clean up
kubectl delete llamastackdistribution dev-llamastack
```

### Production Deployment

```bash
# 1. Validate configuration
kubectl apply -f prod-llamastack.yaml --dry-run=client

# 2. Deploy
kubectl apply -f prod-llamastack.yaml

# 3. Monitor rollout
kubectl rollout status deployment/prod-llamastack

# 4. Verify health
kubectl get pods -l app=prod-llamastack
kubectl logs -l app=prod-llamastack | grep "Ready"

# 5. Scale if needed
kubectl scale llamastackdistribution prod-llamastack --replicas=5

# 6. Monitor metrics
kubectl top pods -l app=prod-llamastack
```

### Troubleshooting Workflow

```bash
# 1. Check distribution status
kubectl describe llamastackdistribution <name>

# 2. Check pod status
kubectl get pods -l app=<name>
kubectl describe pod <pod-name>

# 3. Check logs
kubectl logs <pod-name>
kubectl logs <pod-name> --previous

# 4. Check events
kubectl get events --field-selector involvedObject.name=<name>

# 5. Debug network
kubectl exec -it <pod-name> -- curl http://localhost:8080/health

# 6. Check resources
kubectl top pods -l app=<name>
kubectl describe node <node-name>
```

## Next Steps

- [API Reference](api.md)
- [Configuration Reference](configuration.md)
- [Troubleshooting Guide](../how-to/troubleshooting.md)
