# Migration Guide: v1alpha1 to v1alpha2

This guide helps existing users migrate `LlamaStackDistribution` custom resources from `v1alpha1` to `v1alpha2`.

## What Changed and Why

`v1alpha2` restructures the CR spec to make LlamaStack configuration **declarative and Kubernetes-native**. Instead of passing provider URLs and model names through environment variables, you now configure them directly in the CR spec. The operator generates the `config.yaml` ConfigMap automatically.

**Key improvements:**

- **No more environment variables for configuration** : providers, models, and storage are declared in the spec
- **Automatic config generation** : the operator builds and mounts `config.yaml` for you
- **Cleaner structure** : networking, workload, and configuration concerns are separated into dedicated sections
- **CEL validation** : the CRD rejects invalid configurations at admission time

Both API versions are served simultaneously. Existing `v1alpha1` resources continue to work without changes — the conversion webhook handles translation automatically. You can migrate at your own pace.

## Quick Comparison

```yaml
# v1alpha1 — environment variable based
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: my-llsd
spec:
  replicas: 1
  server:
    distribution:
      name: starter
    containerSpec:
      port: 8321
      env:
      - name: OLLAMA_INFERENCE_MODEL
        value: "llama3.2:1b"
      - name: OLLAMA_URL
        value: "http://ollama-server-service.ollama-dist.svc.cluster.local:11434"
    storage:
      size: "20Gi"
      mountPath: "/home/lls/.lls"
```

```yaml
# v1alpha2 — declarative, native configuration
apiVersion: llamastack.io/v1alpha2
kind: LlamaStackDistribution
metadata:
  name: my-llsd
spec:
  distribution:
    name: starter
  providers:
    inference:
      provider: ollama
      endpoint: http://ollama-server-service.ollama-dist.svc.cluster.local:11434
  resources:
    models:
      - name: "llama3.2:1b"
  networking:
    port: 8321
  workload:
    replicas: 1
    storage:
      size: "20Gi"
      mountPath: "/home/lls/.lls"
```

## Field Mapping Reference

### Top-Level Structure

| v1alpha1 Path | v1alpha2 Path | Notes |
|---|---|---|
| `spec.replicas` | `spec.workload.replicas` | Moved under `workload`. Type changed from `int32` to `*int32` |
| `spec.server.distribution` | `spec.distribution` | Promoted to top level. Type renamed from `DistributionType` to `DistributionSpec` |
| `spec.server.containerSpec` | *(removed)* | Fields redistributed (see below) |
| `spec.network` | `spec.networking` | Renamed and expanded |

### Distribution

| v1alpha1 | v1alpha2 | Notes |
|---|---|---|
| `spec.server.distribution.name` | `spec.distribution.name` | Same behavior |
| `spec.server.distribution.image` | `spec.distribution.image` | Same behavior |

### Container & Server Settings

| v1alpha1 | v1alpha2 | Notes |
|---|---|---|
| `spec.server.containerSpec.port` | `spec.networking.port` | Moved to networking. Defaults to 8321 |
| `spec.server.containerSpec.env` | `spec.workload.overrides.env` | Moved to workload overrides |
| `spec.server.containerSpec.name` | *(removed)* | Container name is always `llama-stack` |
| `spec.server.containerSpec.resources` | `spec.workload.resources` | Moved to workload |
| `spec.server.containerSpec.command` | `spec.workload.overrides.command` | Moved to workload overrides |
| `spec.server.containerSpec.args` | `spec.workload.overrides.args` | Moved to workload overrides |
| `spec.server.workers` | `spec.workload.workers` | Moved to workload |

### Storage

| v1alpha1 | v1alpha2 | Notes |
|---|---|---|
| `spec.server.storage.size` | `spec.workload.storage.size` | Moved under workload |
| `spec.server.storage.mountPath` | `spec.workload.storage.mountPath` | Moved under workload |

### Pod Overrides

| v1alpha1 | v1alpha2 | Notes |
|---|---|---|
| `spec.server.podOverrides.serviceAccountName` | `spec.workload.overrides.serviceAccountName` | Moved to workload overrides |
| `spec.server.podOverrides.volumes` | `spec.workload.overrides.volumes` | Moved to workload overrides |
| `spec.server.podOverrides.volumeMounts` | `spec.workload.overrides.volumeMounts` | Moved to workload overrides |
| `spec.server.podOverrides.terminationGracePeriodSeconds` | *(removed)* | Not available in v1alpha2 spec. Preserved via annotation during conversion |

### Scaling

| v1alpha1 | v1alpha2 | Notes |
|---|---|---|
| `spec.server.autoscaling` | `spec.workload.autoscaling` | Moved under workload |
| `spec.server.podDisruptionBudget` | `spec.workload.podDisruptionBudget` | Moved under workload |
| `spec.server.topologySpreadConstraints` | `spec.workload.topologySpreadConstraints` | Moved under workload |

### Networking

| v1alpha1 | v1alpha2 | Notes |
|---|---|---|
| `spec.network.exposeRoute` | `spec.networking.expose` | Changed to polymorphic: boolean or `{enabled: true, hostname: "..."}` |
| `spec.network.allowedFrom` | `spec.networking.allowedFrom` | Same structure |
| `spec.server.tlsConfig.caBundle.configMapName` | `spec.networking.tls.caBundle.configMapName` | Moved under networking. Simplified (namespace/keys removed) |

### User Config

| v1alpha1 | v1alpha2 | Notes |
|---|---|---|
| `spec.server.userConfig.configMapName` | `spec.overrideConfig.configMapName` | Renamed. Always same namespace as the CR |
| `spec.server.userConfig.configMapNamespace` | *(removed)* | ConfigMap must be in the same namespace |

### New in v1alpha2 (No v1alpha1 Equivalent)

| v1alpha2 Field | Description |
|---|---|
| `spec.providers` | Declarative provider configuration by API type (inference, safety, vectorIo, etc.) |
| `spec.resources` | Models, tools, and shields to register with providers |
| `spec.storage` | State storage backends (KV: sqlite/redis, SQL: sqlite/postgres) |
| `spec.disabled` | List of API names to disable |
| `spec.externalProviders` | Integration with deploy-time provider injection |
| `status.resolvedDistribution` | Tracks resolved image and config source |
| `status.configGeneration` | Tracks generated ConfigMap name, provider/resource counts |

## Migration Examples

### Basic Ollama Setup

**Before (v1alpha1):**
```yaml
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: my-llsd
spec:
  replicas: 1
  server:
    distribution:
      name: starter
    containerSpec:
      env:
      - name: OLLAMA_INFERENCE_MODEL
        value: "llama3.2:1b"
      - name: OLLAMA_URL
        value: "http://ollama-server-service.ollama-dist.svc.cluster.local:11434"
```

**After (v1alpha2):**
```yaml
apiVersion: llamastack.io/v1alpha2
kind: LlamaStackDistribution
metadata:
  name: my-llsd
spec:
  distribution:
    name: starter
  providers:
    inference:
      provider: ollama
      endpoint: http://ollama-server-service.ollama-dist.svc.cluster.local:11434
  resources:
    models:
      - name: "llama3.2:1b"
  workload:
    replicas: 1
```

### vLLM with API Key

**Before (v1alpha1):**
```yaml
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: my-llsd
spec:
  replicas: 1
  server:
    distribution:
      name: starter
    containerSpec:
      env:
      - name: INFERENCE_MODEL
        value: "meta-llama/Llama-3.2-1B"
      - name: VLLM_URL
        value: "http://vllm-service.vllm-ns.svc:8000/v1"
      - name: VLLM_API_KEY
        valueFrom:
          secretKeyRef:
            name: vllm-secret
            key: api-key
```

**After (v1alpha2):**

The v1alpha1 `secretKeyRef` env var pattern is replaced by the `apiKey` field, which is itself a secret reference (`name` = Secret name, `key` = key within the Secret). The operator injects it as an environment variable at runtime.

```yaml
apiVersion: llamastack.io/v1alpha2
kind: LlamaStackDistribution
metadata:
  name: my-llsd
spec:
  distribution:
    name: starter
  providers:
    inference:
      provider: vllm
      endpoint: http://vllm-service.vllm-ns.svc:8000/v1
      apiKey:
        name: vllm-secret       # Secret name in the same namespace
        key: api-key             # Key within the Secret
  resources:
    models:
      - name: "meta-llama/Llama-3.2-1B"
  workload:
    replicas: 1
```

### ConfigMap Override (Full config.yaml)

If you prefer to manage the full `config.yaml` yourself, use `overrideConfig` instead of `providers`/`resources`:

**Before (v1alpha1):**
```yaml
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: my-llsd
spec:
  replicas: 1
  server:
    distribution:
      name: starter
    userConfig:
      configMapName: llama-stack-config
```

**After (v1alpha2):**
```yaml
apiVersion: llamastack.io/v1alpha2
kind: LlamaStackDistribution
metadata:
  name: my-llsd
spec:
  distribution:
    name: starter
  overrideConfig:
    configMapName: llama-stack-config
  workload:
    replicas: 1
```

> **Note:** `overrideConfig` is mutually exclusive with `providers`, `resources`, `storage`, and `disabled`. Use one approach or the other.

### Network Policy & TLS

**Before (v1alpha1):**
```yaml
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: my-llsd
spec:
  replicas: 1
  server:
    distribution:
      name: starter
    containerSpec:
      port: 9000
    tlsConfig:
      caBundle:
        configMapName: custom-ca-bundle
  network:
    exposeRoute: true
    allowedFrom:
      namespaces: ["monitoring"]
```

**After (v1alpha2):**
```yaml
apiVersion: llamastack.io/v1alpha2
kind: LlamaStackDistribution
metadata:
  name: my-llsd
spec:
  distribution:
    name: starter
  networking:
    port: 9000
    expose: true
    tls:
      caBundle:
        configMapName: custom-ca-bundle
    allowedFrom:
      namespaces: ["monitoring"]
  workload:
    replicas: 1
```

### Full-Featured Example with Autoscaling

**Before (v1alpha1):**
```yaml
apiVersion: llamastack.io/v1alpha1
kind: LlamaStackDistribution
metadata:
  name: my-llsd
spec:
  replicas: 2
  server:
    distribution:
      name: starter
    workers: 4
    containerSpec:
      port: 8321
      env:
      - name: OLLAMA_INFERENCE_MODEL
        value: "llama3.2:1b"
      - name: OLLAMA_URL
        value: "http://ollama.default.svc:11434"
    storage:
      size: "20Gi"
    podOverrides:
      serviceAccountName: custom-sa
    autoscaling:
      minReplicas: 2
      maxReplicas: 10
      targetCPUUtilizationPercentage: 75
    podDisruptionBudget:
      minAvailable: 1
```

**After (v1alpha2):**
```yaml
apiVersion: llamastack.io/v1alpha2
kind: LlamaStackDistribution
metadata:
  name: my-llsd
spec:
  distribution:
    name: starter
  providers:
    inference:
      provider: ollama
      endpoint: http://ollama.default.svc:11434
  resources:
    models:
      - name: "llama3.2:1b"
  networking:
    port: 8321
  workload:
    replicas: 2
    workers: 4
    storage:
      size: "20Gi"
    overrides:
      serviceAccountName: custom-sa
    autoscaling:
      minReplicas: 2
      maxReplicas: 10
      targetCPUUtilizationPercentage: 75
    podDisruptionBudget:
      minAvailable: 1
```

## Multiple Providers

`v1alpha2` supports configuring multiple providers per API type using a list:

```yaml
spec:
  providers:
    inference:
      - id: ollama-local
        provider: ollama
        endpoint: http://ollama.local.svc:11434
      - id: vllm-gpu
        provider: vllm
        endpoint: http://vllm.gpu.svc:8000/v1
        apiKey:                        # secretKeyRef
          name: vllm-secret
          key: api-key
  resources:
    models:
      - name: "llama3.2:1b"
        provider: ollama-local
      - name: "meta-llama/Llama-3.2-70B"
        provider: vllm-gpu
```

When using multiple providers for the same API type, each provider must have a unique `id` field. Model resources can reference a specific provider by its `id`.

## State Storage Configuration

`v1alpha2` introduces declarative state storage backends:

```yaml
spec:
  storage:
    kv:
      type: redis
      endpoint: redis://redis.default.svc:6379
      password:
        name: redis-secret
        key: password
    sql:
      type: postgres
      connectionString:
        name: postgres-secret
        key: dsn
```

If omitted, the operator defaults to SQLite for both KV and SQL storage.

## Conversion Webhook

The operator includes a conversion webhook that automatically translates between `v1alpha1` and `v1alpha2` when resources are read or written. This means:

- Existing `v1alpha1` CRs continue to work without modification
- You can read any CR as either version: `kubectl get llsd my-llsd -o yaml` returns `v1alpha2` (the storage version)
- The conversion preserves all fields, with v1alpha1-only fields stored as annotations when needed

## Step-by-Step Migration

1. **Verify the operator is updated** : Ensure the operator version supports v1alpha2
2. **Review your existing CRs** : List them with `kubectl get llsd -A`
3. **Create the v1alpha2 version** : Use the field mapping reference above to translate your CR
4. **Apply the new CR** : `kubectl apply -f my-llsd-v1alpha2.yaml`
5. **Verify** : Check that the distribution reaches `Ready` phase: `kubectl get llsd my-llsd`
6. **Delete the old CR** : If you created a new resource with a different name, delete the old one

If you applied the new CR with the **same name**, the update is in-place, no need to delete anything.
