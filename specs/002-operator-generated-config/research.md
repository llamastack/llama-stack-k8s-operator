# Research: Operator-Generated Server Configuration

**Feature Branch**: `002-operator-generated-config`
**Date**: 2026-02-02
**Status**: Complete

---

## Research Area 1: Provider Type Mapping

### Question
What are all the llama-stack provider types and their config.yaml structure?

### Findings

Based on llama-stack documentation and config.yaml v2 schema:

**Provider Categories** (map to `providers` section in config.yaml):
- `inference` - LLM inference providers
- `safety` - Content moderation and safety
- `vector_io` - Vector database integration
- `agents` - Agent orchestration
- `tool_runtime` - Tool execution
- `memory` - Memory/conversation persistence
- `telemetry` - Observability/tracing
- `datasetio` - Dataset loading
- `scoring` - Model evaluation
- `eval` - Evaluation pipelines
- `post_training` - Fine-tuning/training

**Provider Type Format**:
```yaml
providers:
  inference:
    - provider_id: "unique-name"
      provider_type: "remote::vllm"  # Format: remote::{provider}
      config:
        url: "http://endpoint:8000"
        api_key: "${env.API_KEY}"
        # provider-specific fields...
```

**CRD to config.yaml Mapping** (confirmed from spec):
| CRD Field | config.yaml Field |
|-----------|-------------------|
| `provider` | `provider_type` (with `remote::` prefix) |
| `endpoint` | `config.url` |
| `apiKey.secretKeyRef` | `config.api_key` as `${env.VAR_NAME}` |
| `config.*` | `config.*` (passthrough) |

### Decision
Use exact mapping defined in spec. Provider type validation against known list prevents invalid configurations.

---

## Research Area 2: Distribution Config Extraction

### Question
How to read embedded config.yaml from distribution images?

### Findings

**Current Behavior** (from codebase analysis):
1. Distribution images contain `run.yaml` or `config.yaml` at known paths
2. The operator's startup script (`startupScript` in resource_helper.go) handles version detection
3. Config is expected at standard paths: `/etc/llama-stack/config.yaml` or passed via args

**Options Evaluated**:

| Option | Description | Pros | Cons |
|--------|-------------|------|------|
| A: Init container extraction | Run init container to copy config from distribution | Works offline, no runtime dependency | Adds complexity, slow startup |
| B: ConfigMap from image | Use `kubectl cp` or similar to extract | Simple | Requires cluster access during build |
| C: Default config embedded in operator | Ship known configs for each distribution | Fast, reliable | Must update operator for new distributions |
| D: Fetch at reconcile time | Download distribution config from registry | Always current | Requires image registry access |

**Existing Pattern**:
The operator already handles user-provided ConfigMaps. The distribution image's config serves as the "base" that user config overlays.

### Decision
**Option C (initially) + D (future)**: For MVP, embed known distribution configs in the operator. The `distribution.name` field maps to a known config template. Future enhancement can extract from image labels or annotations.

**Implementation**:
```go
// pkg/configgen/distributions.go
var DistributionConfigs = map[string][]byte{
    "starter": starterConfigYAML,
    "rh-dev": rhDevConfigYAML,
    // ...
}
```

---

## Research Area 3: CEL Validation Patterns

### Question
Best practices for polymorphic field validation (single object vs list)?

### Findings

**Challenge**: The `providers.inference` field can be either:
- Single object: `inference: {provider: vllm, endpoint: "..."}`
- List: `inference: [{id: a, ...}, {id: b, ...}]`

**CEL Validation Patterns**:

1. **Type checking with `type()` function**:
   ```yaml
   # CEL expression to detect type
   type(self.inference) == list ? self.inference.all(p, has(p.id)) : true
   ```

2. **XValidation on parent struct**:
   ```go
   // +kubebuilder:validation:XValidation:rule="!has(self.inference) || type(self.inference) != 'list' || self.inference.all(p, has(p.id))",message="Multiple providers require explicit 'id' field"
   ```

3. **Alternative: Separate fields** (rejected):
   ```go
   Inference *ProviderEntry `json:"inference,omitempty"`
   InferenceList []ProviderEntry `json:"inferenceList,omitempty"`
   ```
   - Rejected because: User experience degraded, schema bloat

**Go Implementation for Polymorphic**:
```go
// Custom JSON unmarshaling
type ProviderConfig struct {
    Entries []ProviderEntry
}

func (pc *ProviderConfig) UnmarshalJSON(data []byte) error {
    // Try single object first
    var single ProviderEntry
    if err := json.Unmarshal(data, &single); err == nil {
        pc.Entries = []ProviderEntry{single}
        return nil
    }
    // Try list
    return json.Unmarshal(data, &pc.Entries)
}
```

### Decision
Use **runtime.RawExtension** for CRD with custom controller unmarshaling. CEL validation checks list form requires `id` field. This maintains clean user YAML while enabling full validation.

---

## Research Area 4: Secret Resolution Timing

### Question
When to resolve secretKeyRef: during reconciliation or at pod startup?

### Findings

**Options**:

| Option | When | Pros | Cons |
|--------|------|------|------|
| A: Reconcile-time resolution | Controller reads secret, puts value in ConfigMap | Simple config, no secret mount | **Security risk**: Secret in ConfigMap (etcd, logs) |
| B: Env var injection | Controller creates env var in deployment referencing secret | Secret never in ConfigMap | Config must use `${env.VAR}` syntax |
| C: Pod-time resolution | Pod reads secret via mounted volume | Most secure | Requires init container, complex |

**Spec Requirement** (FR-017, SC-004):
> "Secret values never appear in generated ConfigMap (only environment variable references)"

**Current Operator Pattern**:
The operator already uses environment variables for container configuration (see `buildContainerSpec` in resource_helper.go).

### Decision
**Option B: Env var injection**

1. Controller generates deterministic env var name: `LLSD_{PROVIDER}_{FIELD}_SECRET`
2. Deployment references secret via `secretKeyRef` in env
3. Config.yaml uses `${env.LLSD_INFERENCE_API_KEY}` pattern

**Implementation**:
```go
// Generated env var
{
    Name: "LLSD_INFERENCE_API_KEY",
    ValueFrom: &corev1.EnvVarSource{
        SecretKeyRef: &corev1.SecretKeySelector{
            LocalObjectReference: corev1.LocalObjectReference{
                Name: "vllm-creds",
            },
            Key: "token",
        },
    },
}

// In config.yaml
api_key: "${env.LLSD_INFERENCE_API_KEY}"
```

---

## Research Area 5: Hash Algorithm Selection

### Question
Which hash algorithm for ConfigMap naming?

### Findings

**Requirements**:
- Deterministic (same input = same hash)
- Collision-resistant for practical use
- Short enough for Kubernetes name limits (63 chars max)
- URL-safe characters only

**Options Evaluated**:

| Algorithm | Output Length | Truncated | Notes |
|-----------|---------------|-----------|-------|
| SHA256 | 64 hex chars | Yes (8-12 chars) | Standard, secure |
| SHA1 | 40 hex chars | Optional | Legacy but fast |
| xxHash | 16 hex chars | No | Very fast, less common |
| FNV-1a | 8 hex chars | No | Simple, fast |

**Kubernetes Patterns**:
- Helm uses 5-char suffix for release names
- ConfigMap hash annotations typically use full SHA256
- ReplicaSet uses 8-char suffix for pod names

**Collision Analysis**:
- 8 hex chars = 4 billion combinations
- For single LLSD resource, collision probability negligible
- Name format: `{llsd-name}-config-{8-char-hash}` stays well under 63 chars

### Decision
**SHA256 truncated to 10 characters** (40 bits)

- Provides ~1 trillion combinations
- Format: `<llsd-name>-config-<10-char-hash>`
- Example: `my-stack-config-a1b2c3d4e5`

**Implementation**:
```go
func ConfigMapName(llsdName string, configContent []byte) string {
    hash := sha256.Sum256(configContent)
    shortHash := hex.EncodeToString(hash[:])[:10]
    return fmt.Sprintf("%s-config-%s", llsdName, shortHash)
}
```

---

## Summary of Decisions

| Research Area | Decision | Rationale |
|---------------|----------|-----------|
| Provider Type Mapping | Use spec-defined mapping | Clear, testable, extensible |
| Distribution Config | Embed known configs, future: extract from image | Pragmatic MVP, reliable |
| CEL Validation | RawExtension + custom unmarshal + CEL for list ID | Clean UX, full validation |
| Secret Resolution | Env var injection at reconcile time | Secure (SC-004), aligns with existing patterns |
| Hash Algorithm | SHA256 truncated to 10 chars | Deterministic, collision-resistant, fits name limits |

---

## References

- llama-stack config.yaml v2 schema
- Kubernetes CEL validation documentation
- Existing operator codebase patterns (controllers/resource_helper.go)
- Kubernetes API conventions for naming
