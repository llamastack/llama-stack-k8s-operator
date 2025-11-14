# Configure Storage

Learn how to configure persistent storage for your LlamaStack distributions.

## Storage Overview

LlamaStack distributions can use persistent storage for:

- Model files and weights
- Configuration data
- Logs and metrics
- User data and sessions

## Basic Storage Configuration

### Default Storage

By default, LlamaStack uses ephemeral storage:

```yaml
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: basic-llamastack
spec:
  image: llamastack/llamastack:latest
  # No storage configuration = ephemeral storage
```

### Persistent Storage

Enable persistent storage:

```yaml
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: persistent-llamastack
spec:
  image: llamastack/llamastack:latest
  storage:
    size: "50Gi"
    storageClass: "standard"
    accessMode: "ReadWriteOnce"
```

## Storage Classes

### Available Storage Classes

Common storage classes and their use cases:

| Storage Class | Performance | Use Case |
|---------------|-------------|----------|
| `standard` | Standard | General purpose |
| `fast-ssd` | High | Model inference |
| `slow-hdd` | Low | Archival storage |

### Custom Storage Class

Create a custom storage class for LlamaStack:

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: llamastack-storage
provisioner: kubernetes.io/aws-ebs
parameters:
  type: gp3
  iops: "3000"
  throughput: "125"
allowVolumeExpansion: true
```

## Advanced Storage Configurations

### Multiple Volumes

Configure separate volumes for different purposes:

```yaml
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: multi-volume-llamastack
spec:
  image: llamastack/llamastack:latest
  storage:
    models:
      size: "100Gi"
      storageClass: "fast-ssd"
      mountPath: "/models"
    data:
      size: "50Gi"
      storageClass: "standard"
      mountPath: "/data"
    logs:
      size: "10Gi"
      storageClass: "standard"
      mountPath: "/logs"
```

### Shared Storage

Configure shared storage across replicas:

```yaml
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: shared-storage-llamastack
spec:
  image: llamastack/llamastack:latest
  replicas: 3
  storage:
    size: "200Gi"
    storageClass: "nfs"
    accessMode: "ReadWriteMany"  # Allows multiple pods to mount
```

## Storage Optimization

### Performance Tuning

Optimize storage for model inference:

```yaml
spec:
  storage:
    size: "500Gi"
    storageClass: "nvme-ssd"
    iops: 10000
    throughput: "1000MB/s"
```

### Cost Optimization

Use tiered storage for cost efficiency:

```yaml
spec:
  storage:
    hot:
      size: "50Gi"
      storageClass: "fast-ssd"
      mountPath: "/models/active"
    warm:
      size: "200Gi"
      storageClass: "standard"
      mountPath: "/models/cache"
    cold:
      size: "1Ti"
      storageClass: "slow-hdd"
      mountPath: "/models/archive"
```

## Backup and Recovery

### Automated Backups

Configure automated backups:

```yaml
spec:
  backup:
    enabled: true
    schedule: "0 2 * * *"  # Daily at 2 AM
    retention: "30d"
    destination: "s3://my-backup-bucket"
```

### Manual Backup

Create manual backups:

```bash
# Create a snapshot
kubectl create volumesnapshot llamastack-backup \
  --from-pvc=llamastack-storage

# Restore from snapshot
kubectl apply -f - <<EOF
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: llamastack-restored
spec:
  dataSource:
    name: llamastack-backup
    kind: VolumeSnapshot
    apiGroup: snapshot.storage.k8s.io
  accessModes:
    - ReadWriteOnce
  resources:
    requests:
      storage: 50Gi
EOF
```

## Monitoring Storage

### Storage Metrics

Monitor storage usage:

```bash
# Check PVC status
kubectl get pvc

# Check storage usage
kubectl exec -it <pod-name> -- df -h

# Check I/O metrics
kubectl top pods --containers
```

### Alerts

Set up storage alerts:

```yaml
# Prometheus alert for high storage usage
- alert: HighStorageUsage
  expr: kubelet_volume_stats_used_bytes / kubelet_volume_stats_capacity_bytes > 0.8
  for: 5m
  labels:
    severity: warning
  annotations:
    summary: "High storage usage on {{ $labels.persistentvolumeclaim }}"
```

## Troubleshooting

### Common Issues

**PVC Stuck in Pending:**
```bash
# Check storage class
kubectl get storageclass

# Check events
kubectl describe pvc <pvc-name>
```

**Out of Space:**
```bash
# Expand volume (if supported)
kubectl patch pvc <pvc-name> -p '{"spec":{"resources":{"requests":{"storage":"100Gi"}}}}'
```

**Performance Issues:**
```bash
# Check I/O wait
kubectl exec -it <pod-name> -- iostat -x 1

# Check storage class parameters
kubectl describe storageclass <storage-class>
```

## Next Steps

- [Scaling Guide](scaling.md)
- [Monitoring Setup](monitoring.md)
- [Troubleshooting](troubleshooting.md)
