# Scaling

Learn how to scale your LlamaStack distributions for optimal performance and cost efficiency.

## Scaling Overview

LlamaStack supports both horizontal and vertical scaling:

- **Horizontal Scaling**: Add more replicas
- **Vertical Scaling**: Increase resources per replica
- **Auto Scaling**: Automatic scaling based on metrics

## Horizontal Scaling

### Manual Scaling

Scale replicas manually:

```bash
# Scale to 3 replicas
kubectl patch llamastackdistribution my-llamastack \
  -p '{"spec":{"replicas":3}}'

# Or edit the resource directly
kubectl edit llamastackdistribution my-llamastack
```

### Declarative Scaling

Update your YAML configuration:

```yaml
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: scaled-llamastack
spec:
  image: llamastack/llamastack:latest
  replicas: 5  # Scale to 5 replicas
  resources:
    requests:
      cpu: "1"
      memory: "2Gi"
```

## Vertical Scaling

### Resource Adjustment

Increase CPU and memory:

```yaml
spec:
  resources:
    requests:
      cpu: "2"      # Increased from 1
      memory: "4Gi" # Increased from 2Gi
    limits:
      cpu: "4"      # Increased from 2
      memory: "8Gi" # Increased from 4Gi
```

### GPU Scaling

Add GPU resources:

```yaml
spec:
  resources:
    requests:
      nvidia.com/gpu: "1"
    limits:
      nvidia.com/gpu: "2"
```

## Auto Scaling

### Horizontal Pod Autoscaler (HPA)

Create an HPA for automatic scaling:

```yaml
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: llamastack-hpa
spec:
  scaleTargetRef:
    apiVersion: llamastack.io/v1alpha1
    kind: LlamaStackDistribution
    name: my-llamastack
  minReplicas: 2
  maxReplicas: 10
  metrics:
  - type: Resource
    resource:
      name: cpu
      target:
        type: Utilization
        averageUtilization: 70
  - type: Resource
    resource:
      name: memory
      target:
        type: Utilization
        averageUtilization: 80
```

### Vertical Pod Autoscaler (VPA)

Enable automatic resource adjustment:

```yaml
apiVersion: autoscaling.k8s.io/v1
kind: VerticalPodAutoscaler
metadata:
  name: llamastack-vpa
spec:
  targetRef:
    apiVersion: llamastack.io/v1alpha1
    kind: LlamaStackDistribution
    name: my-llamastack
  updatePolicy:
    updateMode: "Auto"
  resourcePolicy:
    containerPolicies:
    - containerName: llamastack
      maxAllowed:
        cpu: "4"
        memory: "8Gi"
      minAllowed:
        cpu: "100m"
        memory: "128Mi"
```

## Performance Considerations

### Load Balancing

Configure load balancing for multiple replicas:

```yaml
apiVersion: v1
kind: Service
metadata:
  name: llamastack-service
spec:
  selector:
    app: my-llamastack
  ports:
  - port: 8080
    targetPort: 8080
  type: LoadBalancer
  sessionAffinity: None  # Round-robin
```

### Resource Requests vs Limits

Best practices for resource configuration:

```yaml
spec:
  resources:
    requests:
      cpu: "1"      # Guaranteed resources
      memory: "2Gi"
    limits:
      cpu: "2"      # Maximum allowed (2x requests)
      memory: "4Gi" # Maximum allowed (2x requests)
```

## Monitoring Scaling

### Scaling Metrics

Monitor key scaling metrics:

```bash
# Check HPA status
kubectl get hpa

# Check resource usage
kubectl top pods -l app=my-llamastack

# Check scaling events
kubectl describe hpa llamastack-hpa
```

### Custom Metrics

Scale based on custom metrics:

```yaml
metrics:
- type: Pods
  pods:
    metric:
      name: requests_per_second
    target:
      type: AverageValue
      averageValue: "100"
```

## Scaling Strategies

### Blue-Green Scaling

Deploy new version alongside old:

```yaml
# Blue deployment (current)
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: llamastack-blue
spec:
  image: llamastack/llamastack:v1.0
  replicas: 3

---
# Green deployment (new)
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: llamastack-green
spec:
  image: llamastack/llamastack:v1.1
  replicas: 3
```

### Canary Scaling

Gradual rollout with traffic splitting:

```yaml
# Main deployment (90% traffic)
spec:
  replicas: 9
  version: "stable"

---
# Canary deployment (10% traffic)
spec:
  replicas: 1
  version: "canary"
```

## Cost Optimization

### Spot Instances

Use spot instances for cost savings:

```yaml
spec:
  nodeSelector:
    node-type: "spot"
  tolerations:
  - key: "spot"
    operator: "Equal"
    value: "true"
    effect: "NoSchedule"
```

### Scheduled Scaling

Scale down during off-hours:

```yaml
# CronJob for scaling down
apiVersion: batch/v1
kind: CronJob
metadata:
  name: scale-down-llamastack
spec:
  schedule: "0 18 * * *"  # 6 PM daily
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: kubectl
            image: bitnami/kubectl
            command:
            - kubectl
            - patch
            - llamastackdistribution
            - my-llamastack
            - -p
            - '{"spec":{"replicas":1}}'
```

## Troubleshooting Scaling

### Common Issues

**Pods Not Scaling:**
```bash
# Check HPA conditions
kubectl describe hpa llamastack-hpa

# Check resource metrics
kubectl top nodes
kubectl top pods
```

**Resource Constraints:**
```bash
# Check node capacity
kubectl describe nodes

# Check resource quotas
kubectl describe resourcequota
```

**Scaling Too Aggressive:**
```bash
# Adjust HPA behavior
kubectl patch hpa llamastack-hpa -p '{"spec":{"behavior":{"scaleUp":{"stabilizationWindowSeconds":300}}}}'
```

## Next Steps

- [Monitoring Setup](monitoring.md)
- [Configure Storage](configure-storage.md)
- [Troubleshooting](troubleshooting.md)
