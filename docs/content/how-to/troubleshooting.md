# Troubleshooting

Common issues and solutions for LlamaStack Kubernetes Operator.

## Quick Diagnostics

### Check Operator Status

```bash
# Check operator pod
kubectl get pods -n llamastack-system

# Check operator logs
kubectl logs -n llamastack-system -l app=llamastack-operator

# Check CRD installation
kubectl get crd llamastackdistributions.llamastack.io
```

### Check Distribution Status

```bash
# List all distributions
kubectl get llamastackdistribution

# Check specific distribution
kubectl describe llamastackdistribution <name>

# Check distribution events
kubectl get events --field-selector involvedObject.name=<distribution-name>
```

## Common Issues

### 1. Operator Not Starting

**Symptoms:**
- Operator pod in CrashLoopBackOff
- No CRDs created

**Diagnosis:**
```bash
kubectl logs -n llamastack-system -l app=llamastack-operator
kubectl describe pod -n llamastack-system -l app=llamastack-operator
```

**Solutions:**
```bash
# Check RBAC permissions
kubectl auth can-i create llamastackdistributions --as=system:serviceaccount:llamastack-system:llamastack-operator

# Reinstall operator
kubectl delete -f operator.yaml
kubectl apply -f operator.yaml

# Check resource limits
kubectl describe pod -n llamastack-system -l app=llamastack-operator
```

### 2. Distribution Not Creating Pods

**Symptoms:**
- LlamaStackDistribution exists but no pods created
- Status shows "Pending" or "Failed"

**Diagnosis:**
```bash
kubectl describe llamastackdistribution <name>
kubectl get events --field-selector involvedObject.name=<name>
```

**Solutions:**
```bash
# Check image availability
kubectl run test --image=<llamastack-image> --dry-run=client

# Check resource quotas
kubectl describe resourcequota

# Check node capacity
kubectl describe nodes
```

### 3. Pods Failing to Start

**Symptoms:**
- Pods in CrashLoopBackOff or Error state
- Container exits immediately

**Diagnosis:**
```bash
kubectl logs <pod-name>
kubectl describe pod <pod-name>
kubectl get events --field-selector involvedObject.name=<pod-name>
```

**Solutions:**
```bash
# Check image pull secrets
kubectl get secrets

# Check resource limits
kubectl describe pod <pod-name>

# Check volume mounts
kubectl exec -it <pod-name> -- ls -la /mnt
```

### 4. Storage Issues

**Symptoms:**
- PVCs stuck in Pending
- Pods can't mount volumes
- Out of disk space

**Diagnosis:**
```bash
kubectl get pvc
kubectl describe pvc <pvc-name>
kubectl get storageclass
```

**Solutions:**
```bash
# Check storage class
kubectl describe storageclass <storage-class>

# Check available storage
kubectl get nodes -o custom-columns=NAME:.metadata.name,CAPACITY:.status.capacity.storage

# Expand volume (if supported)
kubectl patch pvc <pvc-name> -p '{"spec":{"resources":{"requests":{"storage":"100Gi"}}}}'
```

### 5. Network Connectivity Issues

**Symptoms:**
- Services not accessible
- Pods can't communicate
- External traffic not reaching pods

**Diagnosis:**
```bash
kubectl get svc
kubectl describe svc <service-name>
kubectl get endpoints <service-name>
```

**Solutions:**
```bash
# Check service selector
kubectl get pods --show-labels
kubectl describe svc <service-name>

# Test connectivity
kubectl exec -it <pod-name> -- wget -qO- http://<service-name>:8080/health

# Check network policies
kubectl get networkpolicy
```

## Performance Issues

### High CPU Usage

**Diagnosis:**
```bash
kubectl top pods -l app=llamastack
kubectl exec -it <pod-name> -- top
```

**Solutions:**
```bash
# Increase CPU limits
kubectl patch llamastackdistribution <name> -p '{"spec":{"resources":{"limits":{"cpu":"4"}}}}'

# Scale horizontally
kubectl patch llamastackdistribution <name> -p '{"spec":{"replicas":3}}'
```

### High Memory Usage

**Diagnosis:**
```bash
kubectl top pods -l app=llamastack --containers
kubectl exec -it <pod-name> -- free -h
```

**Solutions:**
```bash
# Increase memory limits
kubectl patch llamastackdistribution <name> -p '{"spec":{"resources":{"limits":{"memory":"8Gi"}}}}'

# Check for memory leaks
kubectl exec -it <pod-name> -- ps aux --sort=-%mem
```

### Slow Response Times

**Diagnosis:**
```bash
# Check application logs
kubectl logs -l app=llamastack | grep -i latency

# Test response time
kubectl exec -it <pod-name> -- curl -w "@curl-format.txt" http://localhost:8080/health
```

**Solutions:**
```bash
# Optimize resource allocation
kubectl patch llamastackdistribution <name> -p '{"spec":{"resources":{"requests":{"cpu":"2","memory":"4Gi"}}}}'

# Enable caching
kubectl patch llamastackdistribution <name> -p '{"spec":{"config":{"cache":{"enabled":true}}}}'
```

## Debugging Tools

### Log Analysis

```bash
# Get all logs
kubectl logs -l app=llamastack --all-containers=true

# Follow logs
kubectl logs -f -l app=llamastack

# Get previous container logs
kubectl logs <pod-name> --previous

# Search for errors
kubectl logs -l app=llamastack | grep -i error
```

### Resource Monitoring

```bash
# Check resource usage
kubectl top pods -l app=llamastack
kubectl top nodes

# Check resource limits
kubectl describe pod <pod-name> | grep -A 5 "Limits\|Requests"

# Check resource quotas
kubectl describe resourcequota
```

### Network Debugging

```bash
# Test DNS resolution
kubectl exec -it <pod-name> -- nslookup kubernetes.default

# Test service connectivity
kubectl exec -it <pod-name> -- telnet <service-name> 8080

# Check iptables rules
kubectl exec -it <pod-name> -- iptables -L
```

## Advanced Debugging

### Debug Container

Run a debug container in the same network namespace:

```bash
kubectl debug <pod-name> -it --image=nicolaka/netshoot
```

### Port Forwarding

Access services directly:

```bash
# Forward to pod
kubectl port-forward <pod-name> 8080:8080

# Forward to service
kubectl port-forward svc/<service-name> 8080:8080
```

### Exec into Container

Access container shell:

```bash
# Get shell access
kubectl exec -it <pod-name> -- /bin/bash

# Run specific commands
kubectl exec <pod-name> -- ps aux
kubectl exec <pod-name> -- netstat -tulpn
```

## Configuration Issues

### Invalid YAML

**Symptoms:**
- kubectl apply fails
- Validation errors

**Solutions:**
```bash
# Validate YAML syntax
kubectl apply --dry-run=client -f distribution.yaml

# Check API version
kubectl api-resources | grep llamastack

# Validate against schema
kubectl explain llamastackdistribution.spec
```

### Missing Dependencies

**Symptoms:**
- Operator fails to start
- Missing CRDs

**Solutions:**
```bash
# Install CRDs
kubectl apply -f https://raw.githubusercontent.com/llamastack/llama-stack-k8s-operator/main/config/crd/bases/llamastack.io_llamastackdistributions.yaml

# Check operator dependencies
kubectl get deployment -n llamastack-system
```

## Getting Help

### Collect Debug Information

```bash
#!/bin/bash
# Debug information collection script

echo "=== Operator Status ==="
kubectl get pods -n llamastack-system
kubectl logs -n llamastack-system -l app=llamastack-operator --tail=100

echo "=== Distributions ==="
kubectl get llamastackdistribution -o wide
kubectl describe llamastackdistribution

echo "=== Pods ==="
kubectl get pods -l app=llamastack -o wide
kubectl describe pods -l app=llamastack

echo "=== Events ==="
kubectl get events --sort-by=.metadata.creationTimestamp

echo "=== Resources ==="
kubectl top nodes
kubectl top pods -l app=llamastack
```

### Support Channels

- **GitHub Issues**: [Report bugs and feature requests](https://github.com/llamastack/llama-stack-k8s-operator/issues)
- **Documentation**: [Official documentation](https://llamastack-k8s-operator.pages.dev)
- **Community**: [Join the discussion](https://github.com/llamastack/llama-stack-k8s-operator/discussions)

## Next Steps

- [Monitoring Setup](monitoring.md)
- [Scaling Guide](scaling.md)
- [Configure Storage](configure-storage.md)
