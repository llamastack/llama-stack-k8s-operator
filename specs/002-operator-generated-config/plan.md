# Implementation Plan: Operator-Generated Server Configuration (v1alpha2)

**Branch**: `002-operator-generated-config` | **Date**: 2026-02-02 | **Spec**: [spec.md](spec.md)
**Status**: Ready for Implementation

## Summary

Introduce a v1alpha2 API version for the LlamaStackDistribution CRD that enables the operator to generate server configuration (config.yaml) from a high-level, abstracted CR specification. Users configure providers, resources, and storage with minimal YAML while the operator handles config generation, secret resolution, and atomic Deployment updates.

## Technical Context

**Language/Version**: Go 1.25 (go.mod)
**Primary Dependencies**: controller-runtime v0.22.4, kubebuilder, kustomize/api v0.21.0, client-go v0.34.3, go-containerregistry v0.20.7
**Storage**: Kubernetes ConfigMaps (generated), Secrets (referenced via secretKeyRef)
**Testing**: Go test, envtest (controller-runtime), testify v1.11.1
**Target Platform**: Kubernetes 1.30+
**Project Type**: Kubernetes operator (single binary)
**Performance Goals**: Config generation < 5 seconds (NFR-002)
**Constraints**: Namespace-scoped RBAC (constitution §1.1), air-gapped registry support, deterministic output (NFR-001)
**Scale/Scope**: Single CRD with 2 API versions, ~8 new Go packages

## Constitution Check

*GATE: PASS (1 documented deviation, 0 unresolved violations)*

| # | Principle | Status | Notes |
|---|-----------|--------|-------|
| §1.1 Namespace-Scoped | DEVIATION | ValidatingWebhookConfiguration is cluster-scoped (standard operator pattern). Documented in spec.md Security Considerations. |
| §1.2 Idempotent Reconciliation | PASS | Deterministic config generation (NFR-001). Hash-based change detection. |
| §1.3 Owner References | PASS | FR-025 requires owner refs on generated ConfigMaps. |
| §2.1 Kubebuilder Validation | PASS | CEL (FR-070-072), webhook (FR-076-078), kubebuilder tags. |
| §2.2 Optional Fields | PASS | Pointer types for optional structs throughout. |
| §2.3 Defaults | PASS | Constants for DefaultServerPort, storage type defaults. |
| §2.4 Status Subresource | PASS | New conditions: ConfigGenerated, DeploymentUpdated, Available, SecretsResolved. |
| §3.2 Conditions | PASS | Standard metav1.Condition with defined constants for types, reasons, messages. |
| §4.1 Error Wrapping | PASS | All errors wrapped with %w and context. |
| §6.1 Table-Driven Tests | PASS | Test plan follows constitution patterns. |
| §6.4 Builder Pattern | PASS | Existing test builders extended for v1alpha2. |
| §13.2 AI Attribution | PASS | Assisted-by format (no Co-Authored-By). |

## Project Structure

### Documentation (this feature)

```text
specs/002-operator-generated-config/
├── spec.md              # Feature specification
├── plan.md              # This file
├── research.md          # Phase 0 research decisions
├── data-model.md        # Entity definitions and relationships
├── quickstart.md        # Usage examples
├── contracts/           # Interface contracts
│   ├── crd-schema.yaml
│   ├── config-generation.yaml
│   └── status-conditions.yaml
├── tasks.md             # Implementation tasks
├── review_summary.md    # Executive brief
└── alternatives/        # Alternative approaches evaluated
```

### Source Code (repository root)

```text
api/
├── v1alpha1/            # Existing types + conversion spoke
│   ├── llamastackdistribution_types.go
│   └── llamastackdistribution_conversion.go  # New: v1alpha1 spoke
└── v1alpha2/            # New API version
    ├── groupversion_info.go
    ├── llamastackdistribution_types.go
    ├── llamastackdistribution_webhook.go      # Validating webhook
    ├── llamastackdistribution_conversion.go   # Hub (no-op)
    └── zz_generated.deepcopy.go               # Generated

pkg/config/              # New: config generation engine
├── config.go            # Main orchestration
├── generator.go         # YAML generation
├── resolver.go          # Base config resolution
├── provider.go          # Provider expansion
├── resource.go          # Resource expansion
├── storage.go           # Storage configuration
├── secret_resolver.go   # Secret reference resolution
├── version.go           # Config schema version handling
├── types.go             # Internal config types
└── oci_extractor.go     # Phase 2: OCI label extraction

configs/                 # New: embedded default configs
├── starter/config.yaml
├── remote-vllm/config.yaml
├── meta-reference-gpu/config.yaml
└── postgres-demo/config.yaml

controllers/             # Extended for v1alpha2
├── llamastackdistribution_controller.go  # Updated reconciliation
└── status.go                             # New conditions

config/webhook/          # New: webhook kustomize config
└── manifests.yaml

tests/e2e/               # Extended
└── config_generation_test.go  # New: v1alpha2 e2e tests
```

## Implementation Phases

The implementation is divided into 5 phases, designed to allow incremental delivery and testing:

```
Phase 1: CRD Schema (v1alpha2)     ─────►  Foundation
Phase 2: Config Generation Engine  ─────►  Core Logic
Phase 3: Controller Integration    ─────►  Reconciliation
Phase 4: Conversion Webhook        ─────►  Backward Compat
Phase 5: Testing & Documentation   ─────►  Quality Gates
```

---

## Phase 1: CRD Schema (v1alpha2)

**Goal**: Define the new v1alpha2 API types with all new fields.

**Requirements Covered**: FR-001 to FR-014, FR-070 to FR-072

### Tasks

#### 1.1 Create v1alpha2 API Directory Structure

**Files to create**:
- `api/v1alpha2/groupversion_info.go`
- `api/v1alpha2/llamastackdistribution_types.go`
- `api/v1alpha2/zz_generated.deepcopy.go` (generated)

**Approach**:
1. Copy v1alpha1 as starting point
2. Restructure according to spec schema
3. Add new types for providers, resources, storage, networking, workload

#### 1.2 Define Provider Types

**New types**:
```go
// ProviderSpec supports polymorphic single/list form via json.RawMessage
type ProvidersSpec struct {
    Inference   *ProviderConfigOrList `json:"inference,omitempty"`
    Safety      *ProviderConfigOrList `json:"safety,omitempty"`
    VectorIo    *ProviderConfigOrList `json:"vectorIo,omitempty"`
    ToolRuntime *ProviderConfigOrList `json:"toolRuntime,omitempty"`
    Telemetry   *ProviderConfigOrList `json:"telemetry,omitempty"`
}

type ProviderConfig struct {
    ID       string                 `json:"id,omitempty"`
    Provider string                 `json:"provider"`
    Endpoint string                 `json:"endpoint,omitempty"`
    ApiKey   *SecretKeyRef          `json:"apiKey,omitempty"`
    Settings map[string]interface{} `json:"settings,omitempty"`
}
```

**Polymorphic handling**: Use `json.RawMessage` for `ProviderConfigOrList`, parse at runtime.

#### 1.3 Define Resource Types

**New types**:
```go
type ResourcesSpec struct {
    Models  []ModelConfigOrString  `json:"models,omitempty"`
    Tools   []string               `json:"tools,omitempty"`
    Shields []string               `json:"shields,omitempty"`
}

type ModelConfig struct {
    Name          string `json:"name"`
    Provider      string `json:"provider,omitempty"`
    ContextLength int    `json:"contextLength,omitempty"`
    ModelType     string `json:"modelType,omitempty"`
    Quantization  string `json:"quantization,omitempty"`
}
```

#### 1.4 Define Storage Types

**New types**:
```go
type StorageSpec struct {
    KV  *KVStorageSpec  `json:"kv,omitempty"`
    SQL *SQLStorageSpec `json:"sql,omitempty"`
}

type KVStorageSpec struct {
    Type     string       `json:"type,omitempty"` // sqlite, redis
    Endpoint string       `json:"endpoint,omitempty"`
    Password *SecretKeyRef `json:"password,omitempty"`
}

type SQLStorageSpec struct {
    Type             string       `json:"type,omitempty"` // sqlite, postgres
    ConnectionString *SecretKeyRef `json:"connectionString,omitempty"`
}
```

#### 1.5 Define Networking Types

**New types**:
```go
type NetworkingSpec struct {
    Port        int32           `json:"port,omitempty"`
    TLS         *TLSSpec        `json:"tls,omitempty"`
    Expose      *ExposeConfig   `json:"expose,omitempty"` // Polymorphic
    AllowedFrom *AllowedFromSpec `json:"allowedFrom,omitempty"`
}

type ExposeConfig struct {
    Enabled  *bool  `json:"enabled,omitempty"`
    Hostname string `json:"hostname,omitempty"`
}
```

#### 1.6 Define Workload Types

**New types**:
```go
type WorkloadSpec struct {
    Replicas                  *int32                      `json:"replicas,omitempty"`
    Workers                   *int32                      `json:"workers,omitempty"`
    Resources                 *corev1.ResourceRequirements `json:"resources,omitempty"`
    Autoscaling               *AutoscalingSpec            `json:"autoscaling,omitempty"`
    Storage                   *PVCStorageSpec             `json:"storage,omitempty"`
    PodDisruptionBudget       *PodDisruptionBudgetSpec    `json:"podDisruptionBudget,omitempty"`
    TopologySpreadConstraints []corev1.TopologySpreadConstraint `json:"topologySpreadConstraints,omitempty"`
    Overrides                 *WorkloadOverrides          `json:"overrides,omitempty"`
}

type WorkloadOverrides struct {
    ServiceAccountName string              `json:"serviceAccountName,omitempty"`
    Env                []corev1.EnvVar     `json:"env,omitempty"`
    Command            []string            `json:"command,omitempty"`
    Args               []string            `json:"args,omitempty"`
    Volumes            []corev1.Volume     `json:"volumes,omitempty"`
    VolumeMounts       []corev1.VolumeMount `json:"volumeMounts,omitempty"`
}
```

#### 1.7 Add CEL Validation Rules

**Validation rules**:
```go
// +kubebuilder:validation:XValidation:rule="!(has(self.providers) && has(self.overrideConfig))",message="providers and overrideConfig are mutually exclusive"
// +kubebuilder:validation:XValidation:rule="!(has(self.resources) && has(self.overrideConfig))",message="resources and overrideConfig are mutually exclusive"
// +kubebuilder:validation:XValidation:rule="!(has(self.storage) && has(self.overrideConfig))",message="storage and overrideConfig are mutually exclusive"
// +kubebuilder:validation:XValidation:rule="!(has(self.disabled) && has(self.overrideConfig))",message="disabled and overrideConfig are mutually exclusive"
```

#### 1.8 Generate CRD Manifests

**Commands**:
```bash
make generate
make manifests
```

**Verification**:
- CRD YAML generated in `config/crd/bases/`
- OpenAPI schema includes all new fields
- CEL validation rules appear in CRD

### Deliverables

- [ ] `api/v1alpha2/` package with all types
- [ ] Generated CRD manifests
- [ ] CEL validation for mutual exclusivity
- [ ] Unit tests for type marshaling/unmarshaling

---

## Phase 2: Config Generation Engine

**Goal**: Implement the core config generation logic.

**Requirements Covered**: FR-020 to FR-029, FR-030 to FR-035, FR-040 to FR-044, FR-050 to FR-053, NFR-001, NFR-005, NFR-006

### Tasks

#### 2.1 Create Config Package Structure

**Directory**: `pkg/config/`

**Files**:
```
pkg/config/
├── config.go           # Main orchestration
├── generator.go        # YAML generation
├── resolver.go         # Base config resolution (embedded + OCI)
├── oci_extractor.go    # Phase 2: OCI label extraction
├── provider.go         # Provider expansion
├── resource.go         # Resource expansion
├── storage.go          # Storage configuration
├── secret_resolver.go  # Secret reference resolution
├── version.go          # Config schema version handling
└── types.go            # Internal config types
```

#### 2.2 Implement Base Config Resolution (Phased)

**Files**:
- `pkg/config/resolver.go` - BaseConfigResolver with resolution priority logic
- `configs/` - Embedded default config directory (one `config.yaml` per named distribution)
- `pkg/config/oci_extractor.go` - Phase 2: OCI label extraction

**Approach**: Base config resolution follows a phased strategy. Phase 1 (MVP) uses configs embedded in the operator binary via `go:embed`, requiring no changes to distribution image builds. Phase 2 (Enhancement) adds OCI label-based extraction as an optional override when distribution images support it.

> **Alternative**: An init container approach is documented in `alternatives/init-container-extraction.md` for cases where neither embedded configs nor OCI labels are available.

**Resolution Priority**:

```
┌─────────────────────────────────────────────────────────────┐
│  1. (Phase 2) Check OCI labels on resolved image            │
│     └── If present: extract config from labels              │
│         (takes precedence over embedded)                    │
│                                                             │
│  2. (Phase 1) Check embedded configs for distribution.name  │
│     └── If found: use go:embed config for that distribution │
│                                                             │
│  3. No config available:                                    │
│     ├── distribution.name → should not happen (build error) │
│     └── distribution.image → require overrideConfig         │
│         (set ConfigGenerated=False, reason BaseConfigReq.)  │
└─────────────────────────────────────────────────────────────┘
```

##### Phase 1: Embedded Default Configs (MVP)

The operator binary embeds default configs for all named distributions via `go:embed`:

```go
// pkg/config/resolver.go

import "embed"

//go:embed configs
var embeddedConfigs embed.FS

type BaseConfigResolver struct {
    distributionImages map[string]string // from distributions.json
    imageOverrides     map[string]string // from operator ConfigMap
    ociExtractor       *ImageConfigExtractor // nil in Phase 1
}

func NewBaseConfigResolver(distImages, overrides map[string]string) *BaseConfigResolver {
    return &BaseConfigResolver{
        distributionImages: distImages,
        imageOverrides:     overrides,
    }
}

func (r *BaseConfigResolver) Resolve(ctx context.Context, dist DistributionSpec) (*BaseConfig, string, error) {
    // Resolve distribution to concrete image reference
    image, err := r.resolveImage(dist)
    if err != nil {
        return nil, "", err
    }

    // Phase 2: Check OCI labels first (when ociExtractor is configured)
    if r.ociExtractor != nil {
        config, err := r.ociExtractor.Extract(ctx, image)
        if err == nil {
            return config, image, nil
        }
        // Fall through to embedded if OCI extraction fails
        log.FromContext(ctx).V(1).Info("OCI config extraction failed, falling back to embedded",
            "image", image, "error", err)
    }

    // Phase 1: Use embedded config for named distributions
    if dist.Name != "" {
        config, err := r.loadEmbeddedConfig(dist.Name)
        if err != nil {
            return nil, "", fmt.Errorf("failed to load embedded config for distribution %q: %w", dist.Name, err)
        }
        return config, image, nil
    }

    // distribution.image without OCI labels or embedded config
    return nil, "", fmt.Errorf("direct image references require either overrideConfig.configMapName or OCI config labels on the image")
}

func (r *BaseConfigResolver) loadEmbeddedConfig(name string) (*BaseConfig, error) {
    data, err := embeddedConfigs.ReadFile(fmt.Sprintf("configs/%s/config.yaml", name))
    if err != nil {
        return nil, fmt.Errorf("no embedded config for distribution %q: %w", name, err)
    }

    var config BaseConfig
    if err := yaml.Unmarshal(data, &config); err != nil {
        return nil, fmt.Errorf("invalid embedded config for distribution %q: %w", name, err)
    }

    return &config, nil
}

func (r *BaseConfigResolver) resolveImage(dist DistributionSpec) (string, error) {
    if dist.Image != "" {
        return dist.Image, nil
    }

    // Check image-overrides first (downstream builds)
    if override, ok := r.imageOverrides[dist.Name]; ok {
        return override, nil
    }

    // Look up in distributions.json
    if image, ok := r.distributionImages[dist.Name]; ok {
        return image, nil
    }

    return "", fmt.Errorf("unknown distribution name %q: not found in distributions.json", dist.Name)
}
```

**Embedded config directory** (created at build time):
```
configs/
├── starter/config.yaml
├── remote-vllm/config.yaml
├── meta-reference-gpu/config.yaml
└── postgres-demo/config.yaml
```

**Air-gapped support**: Embedded configs work regardless of registry access. The `image-overrides` mechanism allows downstream builds (e.g., RHOAI) to remap distribution names to internal registry images without rebuilding the operator.

##### Phase 2: OCI Label Extraction (Enhancement)

**File**: `pkg/config/oci_extractor.go`

When distribution images include OCI config labels, the extracted config takes precedence over embedded defaults. This enables `distribution.image` usage without `overrideConfig`.

**OCI Label Convention**:

| Label | Purpose | When Used |
|-------|---------|-----------|
| `io.llamastack.config.base64` | Base64-encoded config.yaml | Small configs (< 50KB) |
| `io.llamastack.config.layer` | Layer digest containing config | Large configs |
| `io.llamastack.config.path` | Path within the layer | Used with layer reference |
| `io.llamastack.config.version` | Config schema version | Always |

**Registry Authentication**:

Uses `k8schain` from `go-containerregistry` to authenticate the same way kubelet does:

```go
import (
    "github.com/google/go-containerregistry/pkg/authn/k8schain"
    "github.com/google/go-containerregistry/pkg/crane"
)

// k8schain checks (in order):
// 1. ServiceAccount imagePullSecrets
// 2. Namespace default ServiceAccount
// 3. Node credentials (GCR, ECR, ACR)
// 4. Anonymous access
```

**Implementation**:

```go
// pkg/config/oci_extractor.go

type ConfigLocation struct {
    Base64      string  // Inline base64 encoded config
    LayerDigest string  // Layer digest containing config file
    Path        string  // Path within the layer
    Version     string  // Config schema version
}

type ImageConfigExtractor struct {
    k8sClient      client.Client
    namespace      string
    serviceAccount string
    cache          *sync.Map // image digest -> BaseConfig
}

func NewImageConfigExtractor(client client.Client, namespace, sa string) *ImageConfigExtractor {
    return &ImageConfigExtractor{
        k8sClient:      client,
        namespace:      namespace,
        serviceAccount: sa,
        cache:          &sync.Map{},
    }
}

func (e *ImageConfigExtractor) Extract(ctx context.Context, imageRef string) (*BaseConfig, error) {
    // Build keychain from Kubernetes secrets (same as kubelet)
    keychain, err := k8schain.NewInCluster(ctx, k8schain.Options{
        Namespace:          e.namespace,
        ServiceAccountName: e.serviceAccount,
    })
    if err != nil {
        return nil, fmt.Errorf("failed to create keychain: %w", err)
    }

    // Get image digest for cache key
    digest, err := crane.Digest(imageRef, crane.WithAuthFromKeychain(keychain))
    if err != nil {
        return nil, fmt.Errorf("failed to resolve image digest: %w", err)
    }

    // Check cache
    if cached, ok := e.cache.Load(digest); ok {
        return cached.(*BaseConfig), nil
    }

    // Fetch config location from image labels
    loc, err := e.getConfigLocation(imageRef, keychain)
    if err != nil {
        return nil, err
    }

    var config *BaseConfig

    // Strategy 1: Inline base64
    if loc.Base64 != "" {
        config, err = e.extractFromBase64(loc.Base64)
        if err != nil {
            return nil, err
        }
    } else if loc.LayerDigest != "" && loc.Path != "" {
        // Strategy 2: Layer reference
        config, err = e.extractFromLayer(ctx, imageRef, loc.LayerDigest, loc.Path, keychain)
        if err != nil {
            return nil, err
        }
    } else {
        // No labels found
        return nil, fmt.Errorf("distribution image %s missing config labels (io.llamastack.config.base64 or io.llamastack.config.layer)", imageRef)
    }

    // Cache by digest
    e.cache.Store(digest, config)

    return config, nil
}

func (e *ImageConfigExtractor) getConfigLocation(imageRef string, kc authn.Keychain) (*ConfigLocation, error) {
    configJSON, err := crane.Config(imageRef, crane.WithAuthFromKeychain(kc))
    if err != nil {
        return nil, fmt.Errorf("failed to fetch image config: %w", err)
    }

    var imgConfig v1.ConfigFile
    if err := json.Unmarshal(configJSON, &imgConfig); err != nil {
        return nil, err
    }

    labels := imgConfig.Config.Labels
    return &ConfigLocation{
        Base64:      labels["io.llamastack.config.base64"],
        LayerDigest: labels["io.llamastack.config.layer"],
        Path:        labels["io.llamastack.config.path"],
        Version:     labels["io.llamastack.config.version"],
    }, nil
}

func (e *ImageConfigExtractor) extractFromBase64(b64 string) (*BaseConfig, error) {
    data, err := base64.StdEncoding.DecodeString(b64)
    if err != nil {
        return nil, fmt.Errorf("invalid base64 config: %w", err)
    }

    var config BaseConfig
    if err := yaml.Unmarshal(data, &config); err != nil {
        return nil, fmt.Errorf("invalid config YAML: %w", err)
    }

    return &config, nil
}

func (e *ImageConfigExtractor) extractFromLayer(
    ctx context.Context,
    imageRef string,
    layerDigest string,
    path string,
    kc authn.Keychain,
) (*BaseConfig, error) {
    ref, err := name.ParseReference(imageRef)
    if err != nil {
        return nil, err
    }

    // Fetch only the specific layer by digest
    layerRef := ref.Context().Digest(layerDigest)
    layer, err := remote.Layer(layerRef, remote.WithAuthFromKeychain(kc))
    if err != nil {
        return nil, fmt.Errorf("failed to fetch layer %s: %w", layerDigest, err)
    }

    reader, err := layer.Uncompressed()
    if err != nil {
        return nil, err
    }
    defer reader.Close()

    // Extract file from layer tar
    tr := tar.NewReader(reader)
    targetPath := strings.TrimPrefix(path, "/")

    for {
        header, err := tr.Next()
        if err == io.EOF {
            return nil, fmt.Errorf("config file %s not found in layer", path)
        }
        if err != nil {
            return nil, err
        }

        if strings.TrimPrefix(header.Name, "./") == targetPath {
            data, err := io.ReadAll(tr)
            if err != nil {
                return nil, err
            }

            var config BaseConfig
            if err := yaml.Unmarshal(data, &config); err != nil {
                return nil, err
            }

            return &config, nil
        }
    }
}
```

**Distribution Image Build Integration** (Phase 2):

Labels are added post-build using `crane mutate` (solves the chicken-and-egg problem where layer digests are only known after build):

```bash
#!/bin/bash
# build-distribution.sh

IMAGE_REF="quay.io/llamastack/distribution-starter"
VERSION="${1:-latest}"
CONFIG_PATH="/app/config.yaml"
MAX_INLINE_SIZE=51200  # 50KB

# Step 1: Build image normally
docker build -t "${IMAGE_REF}:build" .
docker push "${IMAGE_REF}:build"

# Step 2: Extract config and determine strategy
CONFIG_DATA=$(crane export "${IMAGE_REF}:build" - | tar -xO "${CONFIG_PATH#/}" 2>/dev/null || echo "")
CONFIG_SIZE=${#CONFIG_DATA}

# Step 3: Find layer containing config
LAYER_DIGEST=""
LAYERS=$(crane manifest "${IMAGE_REF}:build" | jq -r '.layers[].digest')
for layer in $LAYERS; do
    if crane blob "${IMAGE_REF}@${layer}" | tar -tz 2>/dev/null | grep -q "${CONFIG_PATH#/}"; then
        LAYER_DIGEST="$layer"
        break
    fi
done

# Step 4: Add labels based on config size
if [ "$CONFIG_SIZE" -lt "$MAX_INLINE_SIZE" ]; then
    CONFIG_B64=$(echo "$CONFIG_DATA" | base64 -w0)
    crane mutate "${IMAGE_REF}:build" \
        --label "io.llamastack.config.base64=${CONFIG_B64}" \
        --label "io.llamastack.config.version=2" \
        -t "${IMAGE_REF}:${VERSION}"
else
    crane mutate "${IMAGE_REF}:build" \
        --label "io.llamastack.config.layer=${LAYER_DIGEST}" \
        --label "io.llamastack.config.path=${CONFIG_PATH}" \
        --label "io.llamastack.config.version=2" \
        -t "${IMAGE_REF}:${VERSION}"
fi
```

**Key Points**:
- `crane mutate` updates only the config blob, not layers (layer digests unchanged)
- Labels added after build, so layer digest is known
- Works with any registry that supports OCI manifests

**Air-Gapped / OpenShift Support** (Phase 2):

The `k8schain` authenticator handles:
- imagePullSecrets from ServiceAccount
- ImageContentSourcePolicy (OpenShift image mirroring)
- Internal registry authentication

```
┌─────────────────────────────────────────────────────────────┐
│  Air-Gapped Cluster                                         │
│                                                             │
│  ┌──────────────┐      ┌─────────────────────────────────┐ │
│  │  Operator    │─────►│  Internal Registry              │ │
│  │  (k8schain)  │      │  (mirror.internal:5000)         │ │
│  └──────────────┘      │                                 │ │
│         │              │  └─ llamastack/dist-starter     │ │
│         ▼              └─────────────────────────────────┘ │
│  ┌──────────────┐                    ▲                     │
│  │ ServiceAcct  │──imagePullSecrets──┘                     │
│  └──────────────┘                                          │
└─────────────────────────────────────────────────────────────┘
```

**Pros**:
- Single-phase reconciliation (no two-phase complexity)
- Minimal network transfer (~10KB for manifest+config)
- Uses same auth as kubelet (imagePullSecrets work automatically)
- In-memory caching by digest (fast for repeated reconciles)

**Cons**:
- Phase 2 requires distribution images to include config labels
- Requires `go-containerregistry` dependency
- Distribution build process must use `crane mutate` (Phase 2)

#### 2.3 Implement Provider Expansion

**File**: `pkg/config/provider.go`

**Functionality**:
1. Parse polymorphic provider input (single vs list)
2. Auto-generate provider IDs for single providers
3. Map CRD fields to config.yaml format
4. Merge settings into provider config

**Key functions**:
```go
func ExpandProviders(spec *v1alpha2.ProvidersSpec) ([]ProviderConfig, error)
func NormalizeProviderType(provider string) string  // Add "remote::" prefix
func GenerateProviderID(providerType string) string
```

**Field mapping**:
| CRD Field | config.yaml Field |
|-----------|-------------------|
| provider | provider_type (with remote:: prefix) |
| endpoint | config.url |
| apiKey | config.api_key (via env var) |
| settings.* | config.* |

#### 2.4 Implement Resource Expansion

**File**: `pkg/config/resource.go`

**Functionality**:
1. Parse polymorphic model input (string vs object)
2. Assign default provider for simple model strings
3. Generate registered_resources section

**Key functions**:
```go
func ExpandResources(spec *v1alpha2.ResourcesSpec, providers []ProviderConfig) (*RegisteredResources, error)
func GetDefaultInferenceProvider(providers []ProviderConfig) string
```

#### 2.5 Implement Storage Configuration

**File**: `pkg/config/storage.go`

**Functionality**:
1. Map kv and sql storage specs to config.yaml format
2. Handle secret references for connection strings
3. Preserve distribution defaults when not specified

**Key functions**:
```go
func ExpandStorage(spec *v1alpha2.StorageSpec, base *BaseConfig) (*StorageConfig, error)
```

#### 2.6 Implement Secret Resolution

**File**: `pkg/config/secret_resolver.go`

**Functionality**:
1. Collect all secretKeyRef references from the spec
2. Generate deterministic environment variable names
3. Create env var definitions for Deployment
4. Replace references with `${env.VAR_NAME}` in config

**Key functions**:
```go
func ResolveSecrets(spec *v1alpha2.LlamaStackDistributionSpec) (*SecretResolution, error)

type SecretResolution struct {
    EnvVars      []corev1.EnvVar           // For Deployment
    Substitutions map[string]string         // Original -> ${env.VAR}
}
```

**Naming convention**: `LLSD_<PROVIDER>_<FIELD>` (e.g., `LLSD_INFERENCE_API_KEY`)

#### 2.7 Implement Config Generation Orchestration

**File**: `pkg/config/config.go`

**Functionality**:
1. Orchestrate the full config generation flow
2. Merge user config over base config
3. Apply disabled APIs
4. Generate final config.yaml content

**Key functions**:
```go
func GenerateConfig(ctx context.Context, spec *v1alpha2.LlamaStackDistributionSpec, image string) (*GeneratedConfig, error)

type GeneratedConfig struct {
    ConfigYAML    string              // Final config.yaml content
    EnvVars       []corev1.EnvVar     // Environment variables for secrets
    ContentHash   string              // SHA256 of config content
    ProviderCount int                 // For status reporting
    ResourceCount int                 // For status reporting
}
```

#### 2.8 Implement Version Detection

**File**: `pkg/config/version.go`

**Functionality**:
1. Detect config.yaml schema version from base config
2. Validate version is supported (n or n-1)
3. Return clear error for unsupported versions

**Key functions**:
```go
func DetectConfigVersion(config map[string]interface{}) (int, error)
func ValidateConfigVersion(version int) error
```

### Deliverables

- [ ] `pkg/config/` package with all components
- [ ] Unit tests for each component (>80% coverage)
- [ ] Integration tests for full config generation
- [ ] Determinism tests (same input → same output)

---

## Phase 3: Controller Integration

**Goal**: Integrate config generation into the reconciliation loop.

**Requirements Covered**: FR-023 to FR-026, FR-060 to FR-066, FR-073 to FR-075, FR-090 to FR-092

### Tasks

#### 3.1 Create v1alpha2 Controller

**File**: `controllers/llamastackdistribution_v1alpha2_controller.go`

**Approach**:
- Extend existing controller to handle v1alpha2
- Add config generation step in reconciliation
- Maintain compatibility with v1alpha1 flow

**Reconciliation flow**:
```
Reconcile()
├── Fetch LLSD CR
├── Validate (secrets, ConfigMaps exist)
├── DetermineConfigSource()
│   ├── If overrideConfig → Use referenced ConfigMap
│   └── If providers/resources → GenerateConfig()
├── ReconcileGeneratedConfigMap()
├── ReconcileManifestResources() (existing)
├── MergeExternalProviders() (spec 001 integration)
└── UpdateStatus()
```

#### 3.2 Implement Config Source Determination

**Function**: `DetermineConfigSource()`

**Logic**:
```go
func (r *Reconciler) DetermineConfigSource(instance *v1alpha2.LlamaStackDistribution) ConfigSource {
    if instance.Spec.OverrideConfig != nil {
        return ConfigSourceOverride
    }
    if instance.Spec.Providers != nil || instance.Spec.Resources != nil || instance.Spec.Storage != nil {
        return ConfigSourceGenerated
    }
    return ConfigSourceDistributionDefault
}
```

#### 3.3 Implement Generated ConfigMap Reconciliation

**Function**: `ReconcileGeneratedConfigMap()`

**Logic**:
1. Call `pkg/config.GenerateConfig()` to generate config
2. Create ConfigMap with hash-based name: `{name}-config-{hash[:8]}`
3. Set owner reference for garbage collection
4. Clean up old ConfigMaps (keep last 2)

**Key considerations**:
- Immutable ConfigMaps (create new, don't update)
- Content hash ensures change detection
- Owner references enable automatic cleanup

#### 3.4 Extend ManifestContext

**File**: `pkg/deploy/kustomizer.go`

**Additions to ManifestContext**:
```go
type ManifestContext struct {
    // Existing fields...

    // New fields for v1alpha2
    GeneratedConfigMapName string
    GeneratedConfigHash    string
    SecretEnvVars          []corev1.EnvVar
}
```

#### 3.5 Implement Networking Configuration

**Functions**:
- `ReconcileNetworking()`: Handle port, TLS, expose
- Extend existing Ingress reconciliation for polymorphic expose

**Logic for polymorphic expose**:
```go
func (r *Reconciler) ShouldExposeRoute(spec *v1alpha2.NetworkingSpec) bool {
    if spec == nil || spec.Expose == nil {
        return false
    }
    if spec.Expose.Enabled != nil {
        return *spec.Expose.Enabled
    }
    // expose: {} (non-nil pointer, all zero-valued fields) is treated as
    // expose: true per edge case "Polymorphic expose with empty object"
    return true
}
```

#### 3.6 Implement Validation

**Functions**:
- `ValidateSecretReferences()`: Verify all secretKeyRefs exist
- `ValidateConfigMapReferences()`: Verify overrideConfig and caBundle exist
- `ValidateProviderReferences()`: Verify model → provider references

**Error message format**:
```
Secret "vllm-creds" not found in namespace "default".
Referenced by: spec.providers.inference.apiKey.secretKeyRef
```

#### 3.7 Implement Spec 001 Integration

**Function**: `MergeExternalProviders()`

**Logic**:
1. Get generated config (base)
2. Apply external providers from spec 001
3. Log warning on ID conflicts
4. External providers override inline providers

#### 3.8 Extend Status Reporting

**New status fields**:
```go
type ConfigGenerationStatus struct {
    ConfigMapName  string      `json:"configMapName,omitempty"`
    GeneratedAt    metav1.Time `json:"generatedAt,omitempty"`
    ProviderCount  int         `json:"providerCount,omitempty"`
    ResourceCount  int         `json:"resourceCount,omitempty"`
    ConfigVersion  int         `json:"configVersion,omitempty"`
}
```

**New conditions**:
- `ConfigGenerated`: True when config successfully generated
- `SecretsResolved`: True when all secret references valid

### Deliverables

- [ ] Extended controller with v1alpha2 support
- [ ] Config generation integration
- [ ] Networking configuration
- [ ] Validation with actionable errors
- [ ] Spec 001 integration
- [ ] Status reporting extensions

---

## Phase 4: Conversion Webhook

**Goal**: Enable backward compatibility between v1alpha1 and v1alpha2.

**Requirements Covered**: FR-080 to FR-083

### Tasks

#### 4.1 Implement Conversion Hub

**File**: `api/v1alpha2/llamastackdistribution_conversion.go`

**Approach**: v1alpha2 is the hub (storage version). In controller-runtime, the Hub type only implements a `Hub()` marker method. Conversion logic (`ConvertTo`/`ConvertFrom`) lives on the Spoke (v1alpha1).

```go
// Hub marks v1alpha2 as the storage version for conversion.
// The Hub interface requires only this marker method.
// All conversion logic is implemented on the v1alpha1 spoke.
func (dst *LlamaStackDistribution) Hub() {}
```

#### 4.2 Implement v1alpha1 Spoke Conversion

**File**: `api/v1alpha1/llamastackdistribution_conversion.go`

**v1alpha1 → v1alpha2**:
```go
func (src *LlamaStackDistribution) ConvertTo(dstRaw conversion.Hub) error {
    dst := dstRaw.(*v1alpha2.LlamaStackDistribution)

    // Map fields according to migration table
    dst.Spec.Distribution = convertDistribution(src.Spec.Server.Distribution)
    dst.Spec.Networking = convertNetworking(src.Spec.Server, src.Spec.Network)
    dst.Spec.Workload = convertWorkload(src.Spec)
    dst.Spec.OverrideConfig = convertUserConfig(src.Spec.Server.UserConfig)
    // etc.

    return nil
}
```

**v1alpha2 → v1alpha1**:
```go
func (dst *LlamaStackDistribution) ConvertFrom(srcRaw conversion.Hub) error {
    src := srcRaw.(*v1alpha2.LlamaStackDistribution)

    // Reverse mapping
    dst.Spec.Server.Distribution = convertDistributionBack(src.Spec.Distribution)
    // etc.

    // Note: New fields (providers, resources, storage) cannot be represented in v1alpha1
    // These are lost in down-conversion

    return nil
}
```

#### 4.3 Configure Webhook

**File**: `config/webhook/manifests.yaml`

**Enable conversion webhook**:
```yaml
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  name: llamastackdistributions.llamastack.io
spec:
  conversion:
    strategy: Webhook
    webhook:
      conversionReviewVersions: ["v1"]
      clientConfig:
        service:
          namespace: system
          name: webhook-service
          path: /convert
```

#### 4.4 Register Webhook in Main

**File**: `main.go`

```go
if err = (&llamastackv1alpha1.LlamaStackDistribution{}).SetupWebhookWithManager(mgr); err != nil {
    setupLog.Error(err, "unable to create webhook", "webhook", "LlamaStackDistribution")
    os.Exit(1)
}
```

### Deliverables

- [ ] Conversion webhook implementation
- [ ] v1alpha1 ↔ v1alpha2 field mapping
- [ ] Webhook configuration
- [ ] Conversion tests

---

## Phase 5: Testing & Documentation

**Goal**: Ensure quality and provide user guidance.

**Requirements Covered**: All NFRs, User Stories

### Tasks

#### 5.1 Unit Tests

**Coverage targets**:
- `pkg/config/`: >80%
- `api/v1alpha2/`: >80%
- Conversion logic: 100%

**Test files**:
- `pkg/config/config_test.go`
- `pkg/config/provider_test.go`
- `pkg/config/resource_test.go`
- `api/v1alpha2/conversion_test.go`

#### 5.2 Integration Tests

**Test scenarios** (from spec user stories):
1. Simple inference configuration
2. Multiple providers
3. Resource registration
4. State storage configuration
5. Network exposure
6. Override config
7. v1alpha1 migration

**Test file**: `controllers/llamastackdistribution_v1alpha2_test.go`

#### 5.3 E2E Tests

**Test scenarios**:
1. Deploy LLSD with generated config, verify server starts
2. Update provider config, verify rolling update
3. Migration from v1alpha1 to v1alpha2

**Test file**: `tests/e2e/config_generation_test.go`

#### 5.4 Sample Manifests

**Files to create**:
- `config/samples/v1alpha2-simple.yaml`
- `config/samples/v1alpha2-full.yaml`
- `config/samples/v1alpha2-postgres.yaml`
- `config/samples/v1alpha2-multi-provider.yaml`

#### 5.5 Documentation

**Files to update**:
- `README.md`: Add v1alpha2 overview
- `docs/configuration.md`: Detailed configuration guide
- `docs/migration-v1alpha1-to-v1alpha2.md`: Migration guide

### Deliverables

- [ ] Unit tests with >80% coverage
- [ ] Integration tests for all user stories
- [ ] E2E tests
- [ ] Sample manifests
- [ ] Documentation

---

## Implementation Order

```
Week 1-2: Phase 1 (CRD Schema)
    └── Foundation for all other phases

Week 3-4: Phase 2 (Config Generation Engine)
    └── Core logic, can be tested independently

Week 5-6: Phase 3 (Controller Integration)
    └── Depends on Phase 1 and 2

Week 7: Phase 4 (Conversion Webhook)
    └── Depends on Phase 1

Week 8: Phase 5 (Testing & Documentation)
    └── Depends on all phases
```

---

## Risk Mitigation

| Risk | Mitigation |
|------|------------|
| Polymorphic JSON parsing complexity | Use json.RawMessage with well-tested parsing functions |
| Config extraction from images | OCI label approach with k8schain auth; clear error message guides users to use `overrideConfig` as fallback when labels missing |
| Registry authentication failures | Use k8schain (same auth as kubelet); respects imagePullSecrets automatically |
| Conversion webhook failures | Comprehensive unit tests, fallback to direct storage access |
| Breaking changes in config.yaml schema | Version detection and n-1 support |

---

## Success Criteria

- [ ] All FR requirements implemented and tested
- [ ] All NFR requirements met
- [ ] All user stories have passing integration tests
- [ ] v1alpha1 CRs continue to work after upgrade
- [ ] Documentation complete and reviewed
- [ ] No regression in existing functionality

---

## References

- Spec: `specs/002-operator-generated-config/spec.md`
- Design Doc: [LlamaStackDistribution CRD v1alpha2 Schema Design](https://docs.google.com/document/d/10VhoQPb8bLGUo9yka4MXuEGIZClGf1oBr31TpK4NLD0/edit)
- Constitution: `specs/constitution.md`
- Related Spec: `specs/001-deploy-time-providers-l1/spec.md`
