# Implementation Tasks: Operator-Generated Server Configuration (v1alpha2)

**Spec**: 002-operator-generated-config
**Created**: 2026-02-02

## Task Overview

| Phase | Tasks | Priority |
|-------|-------|----------|
| Phase 1: CRD Schema | 8 tasks | P1 |
| Phase 2: Config Generation | 8 tasks | P1 |
| Phase 3: Controller Integration | 12 tasks | P1 |
| Phase 4: Conversion Webhook | 4 tasks | P2 |
| Phase 5: Testing & Docs | 6 tasks | P2 |

---

## Phase 1: CRD Schema (v1alpha2)

### Task 1.1: Create v1alpha2 API Directory Structure

**Priority**: P1
**Blocked by**: None

**Description**:
Create the v1alpha2 API package with groupversion_info.go and base types.

**Files to create**:
- `api/v1alpha2/groupversion_info.go`
- `api/v1alpha2/doc.go`

**Acceptance criteria**:
- [ ] v1alpha2 package compiles
- [ ] GroupVersion is `llamastack.io/v1alpha2`
- [ ] Scheme registration works

---

### Task 1.2: Define Provider Types

**Priority**: P1
**Blocked by**: 1.1

**Description**:
Define ProvidersSpec and ProviderConfig types with polymorphic support.

**Types to define**:
- `ProvidersSpec` (inference, safety, vectorIo, toolRuntime, telemetry)
- `ProviderConfig` (id, provider, endpoint, apiKey, settings)
- `ProviderConfigOrList` (polymorphic wrapper using json.RawMessage)
- `SecretKeyRef` (name, key)

**Requirements covered**: FR-003, FR-004, FR-005

**Acceptance criteria**:
- [ ] Types marshal/unmarshal correctly
- [ ] Polymorphic parsing works (single and list forms)
- [ ] Kubebuilder validation tags present

---

### Task 1.3: Define Resource Types

**Priority**: P1
**Blocked by**: 1.1

**Description**:
Define ResourcesSpec and model/tool types with polymorphic support.

**Types to define**:
- `ResourcesSpec` (models, tools, shields)
- `ModelConfig` (name, provider, contextLength, modelType, quantization)
- `ModelConfigOrString` (polymorphic wrapper)

**Requirements covered**: FR-006, FR-007

**Acceptance criteria**:
- [ ] Types marshal/unmarshal correctly
- [ ] Simple string and object forms both work
- [ ] Kubebuilder validation tags present

---

### Task 1.4: Define Storage Types

**Priority**: P1
**Blocked by**: 1.1

**Description**:
Define StorageSpec with kv and sql subsections.

**Types to define**:
- `StorageSpec` (kv, sql)
- `KVStorageSpec` (type, endpoint, password)
- `SQLStorageSpec` (type, connectionString)

**Requirements covered**: FR-008, FR-050, FR-050a, FR-051, FR-052

**Acceptance criteria**:
- [ ] Types marshal/unmarshal correctly
- [ ] SecretKeyRef references work
- [ ] Enum validation for type field

---

### Task 1.5: Define Networking Types

**Priority**: P1
**Blocked by**: 1.1

**Description**:
Define NetworkingSpec with polymorphic expose support.

**Types to define**:
- `NetworkingSpec` (port, tls, expose, allowedFrom)
- `TLSSpec` (enabled, secretName, caBundle)
- `ExposeConfig` (enabled, hostname - polymorphic)
- `CABundleConfig` (from v1alpha1)

**Requirements covered**: FR-010, FR-011

**Acceptance criteria**:
- [ ] Types marshal/unmarshal correctly
- [ ] Polymorphic expose handles bool and object forms
- [ ] Defaults applied (port: 8321)

---

### Task 1.6: Define Workload Types

**Priority**: P1
**Blocked by**: 1.1

**Description**:
Define WorkloadSpec consolidating K8s deployment settings.

**Types to define**:
- `WorkloadSpec` (replicas, workers, resources, autoscaling, storage, pdb, topologySpread, overrides)
- `WorkloadOverrides` (serviceAccountName, env, command, args, volumes, volumeMounts)
- `PVCStorageSpec` (size, mountPath)
- Reuse existing: `AutoscalingSpec`, `PodDisruptionBudgetSpec`

**Requirements covered**: FR-012

**Acceptance criteria**:
- [ ] Types marshal/unmarshal correctly
- [ ] All fields from v1alpha1 ServerSpec accounted for
- [ ] Kubebuilder validation tags present

---

### Task 1.7: Define Main Spec and Add CEL Validation

**Priority**: P1
**Blocked by**: 1.2, 1.3, 1.4, 1.5, 1.6

**Description**:
Create the main LlamaStackDistributionSpec with all sections and add CEL validation rules.

**Types to define**:
- `LlamaStackDistributionSpec` (distribution, providers, resources, storage, disabled, networking, workload, externalProviders, overrideConfig)
- `LlamaStackDistribution` (main CRD type)
- `LlamaStackDistributionList`
- `OverrideConfigSpec` (configMapName; must be in same namespace as CR)

**CEL validations**:
- Mutual exclusivity: providers vs overrideConfig
- Mutual exclusivity: resources vs overrideConfig
- Mutual exclusivity: storage vs overrideConfig
- Mutual exclusivity: disabled vs overrideConfig

**Requirements covered**: FR-001, FR-002, FR-009, FR-013, FR-014, FR-070

**Printer columns** (constitution §2.5):
- `Phase` (`.status.phase`)
- `Distribution` (`.status.resolvedDistribution.image`, priority=1, wide output only)
- `Config` (`.status.configGeneration.configMapName`, priority=1, wide output only)
- `Providers` (`.status.configGeneration.providerCount`)
- `Available` (`.status.availableReplicas`)
- `Age` (`.metadata.creationTimestamp`)

**Acceptance criteria**:
- [ ] Complete spec structure compiles
- [ ] CEL validations reject invalid combinations
- [ ] Printer columns defined: Phase, Providers, Available, Age (default); Distribution, Config (wide)

---

### Task 1.8: Generate CRD Manifests and Verify

**Priority**: P1
**Blocked by**: 1.7

**Description**:
Run code generation and verify CRD manifests are correct.

**Commands**:
```bash
make generate
make manifests
```

**Verification**:
- [ ] CRD YAML generated in `config/crd/bases/`
- [ ] OpenAPI schema includes all new fields
- [ ] CEL validation rules in CRD
- [ ] Both v1alpha1 and v1alpha2 versions present
- [ ] v1alpha2 is storage version

---

## Phase 2: Config Generation Engine

### Task 2.1: Create Config Package Structure

**Priority**: P1
**Blocked by**: 1.7

**Description**:
Create the pkg/config package directory structure with basic types.

**Files to create**:
- `pkg/config/types.go` - Internal config types
- `pkg/config/config.go` - Main orchestration (stub)

**Acceptance criteria**:
- [ ] Package compiles
- [ ] Internal types defined for config.yaml structure

---

### Task 2.2: Implement Base Config Resolution (Phased)

**Priority**: P1
**Blocked by**: 2.1

**Description**:
Implement base config resolution with a phased approach. Phase 1 (MVP) uses configs embedded in the operator binary via `go:embed`. Phase 2 (Enhancement) adds OCI label-based extraction as an optional override.

**Files**:
- `pkg/config/resolver.go` - BaseConfigResolver with resolution priority logic
- `configs/` - Embedded default config directory (one `config.yaml` per named distribution)
- `Makefile` - Build-time validation target (`validate-configs`)

**Phase 1 (MVP) Approach**:
1. Create `configs/<name>/config.yaml` for each distribution in `distributions.json`
2. Embed via `//go:embed configs` in the resolver package
3. On resolution: lookup embedded config by `distribution.name`
4. For `distribution.image` without OCI labels: require `overrideConfig`

**Phase 2 (Enhancement) Approach**:
1. Add `pkg/config/oci_extractor.go` using `k8schain` for registry auth
2. Check OCI labels on resolved image first (takes precedence over embedded)
3. Fall back to embedded config if no labels found
4. Cache by image digest

**Functions**:
- `NewBaseConfigResolver(distributionImages, imageOverrides) *BaseConfigResolver`
- `(r *BaseConfigResolver) Resolve(ctx, distribution) (*BaseConfig, string, error)`
- `(r *BaseConfigResolver) loadEmbeddedConfig(name) (*BaseConfig, error)`
- `(r *BaseConfigResolver) resolveImage(distribution) (string, error)`

**Requirements covered**: FR-020, FR-027a through FR-027e (Phase 1), FR-027f through FR-027j (Phase 2), NFR-006

**Acceptance criteria**:
- [ ] Embedded configs loaded via `go:embed` for all named distributions
- [ ] `distribution.name` resolves to embedded config
- [ ] `distribution.image` without OCI labels returns clear error requiring `overrideConfig`
- [ ] Build-time validation ensures all distributions have configs
- [ ] (Phase 2) OCI label extraction takes precedence over embedded when available
- [ ] (Phase 2) Caching by image digest prevents repeated extraction
- [ ] Unit tests for resolution priority logic

---

### Task 2.3: Implement Config Version Detection

**Priority**: P1
**Blocked by**: 2.2

**Description**:
Implement config.yaml schema version detection and validation.

**File**: `pkg/config/version.go`

**Functions**:
- `DetectConfigVersion(config) (int, error)`
- `ValidateConfigVersion(version) error`
- `SupportedVersions() []int`

**Requirements covered**: FR-027, FR-028, FR-029

**Acceptance criteria**:
- [ ] Detects version from base config
- [ ] Validates against supported versions (n, n-1)
- [ ] Returns clear error for unsupported versions

---

### Task 2.4: Implement Provider Expansion

**Priority**: P1
**Blocked by**: 2.1

**Description**:
Implement provider spec expansion to config.yaml format.

**File**: `pkg/config/provider.go`

**Functions**:
- `ExpandProviders(spec) ([]ProviderConfig, error)`
- `NormalizeProviderType(provider) string`
- `GenerateProviderID(providerType) string`
- `ParsePolymorphicProvider(raw json.RawMessage) ([]ProviderConfig, error)`

**Requirements covered**: FR-030, FR-031, FR-033, FR-034, FR-035

**Acceptance criteria**:
- [ ] Single provider expands correctly
- [ ] List of providers expands correctly
- [ ] Auto-generates IDs for single providers
- [ ] Merges settings into config section

---

### Task 2.5: Implement Resource Expansion

**Priority**: P1
**Blocked by**: 2.4

**Description**:
Implement resource spec expansion to registered_resources format.

**File**: `pkg/config/resource.go`

**Functions**:
- `ExpandResources(spec, providers) (*RegisteredResources, error)`
- `GetDefaultInferenceProvider(providers) string`
- `ParsePolymorphicModel(raw) ([]ModelConfig, error)`

**Requirements covered**: FR-040, FR-041, FR-042, FR-043, FR-044

**Acceptance criteria**:
- [ ] Simple model strings expand correctly
- [ ] Model objects expand correctly
- [ ] Default provider assignment works
- [ ] Tools and shields expand correctly
- [ ] Tools fail with actionable error when no toolRuntime provider exists (user or base config)
- [ ] Shields fail with actionable error when no safety provider exists (user or base config)

---

### Task 2.6: Implement Storage Expansion

**Priority**: P1
**Blocked by**: 2.1

**Description**:
Implement storage spec expansion to config.yaml format.

**File**: `pkg/config/storage.go`

**Functions**:
- `ExpandStorage(spec, base) (*StorageConfig, error)`
- `ExpandKVStorage(kv) (*KVConfig, error)`
- `ExpandSQLStorage(sql) (*SQLConfig, error)`

**Requirements covered**: FR-050, FR-050a, FR-051, FR-052, FR-053

**Acceptance criteria**:
- [ ] KV storage (sqlite, redis) expands correctly
- [ ] SQL storage (sqlite, postgres) expands correctly
- [ ] Preserves defaults when not specified

---

### Task 2.7: Implement Secret Resolution

**Priority**: P1
**Blocked by**: 2.4, 2.6

**Description**:
Implement secret reference resolution to environment variables.

**File**: `pkg/config/secret_resolver.go`

**Functions**:
- `ResolveSecrets(spec) (*SecretResolution, error)`
- `GenerateEnvVarName(providerType, field) string`
- `CollectSecretRefs(spec) []SecretRef`

**Types**:
```go
type SecretResolution struct {
    EnvVars      []corev1.EnvVar
    Substitutions map[string]string  // placeholder -> ${env.VAR}
}
```

**Requirements covered**: FR-022, FR-032, NFR-003

**Acceptance criteria**:
- [ ] Collects all secretKeyRef references
- [ ] Generates deterministic env var names
- [ ] Creates env var definitions for Deployment
- [ ] Returns substitution map for config generation

---

### Task 2.8: Implement Config Generation Orchestration

**Priority**: P1
**Blocked by**: 2.2, 2.3, 2.4, 2.5, 2.6, 2.7

**Description**:
Implement the main config generation orchestration.

**File**: `pkg/config/config.go`

**Functions**:
- `GenerateConfig(ctx, spec, image) (*GeneratedConfig, error)`
- `MergeConfig(base, user) (map[string]interface{}, error)`
- `ApplyDisabledAPIs(config, disabled) map[string]interface{}`
- `RenderConfigYAML(config) (string, error)`
- `ComputeContentHash(yaml string) string`

**Types**:
```go
type GeneratedConfig struct {
    ConfigYAML    string
    EnvVars       []corev1.EnvVar
    ContentHash   string
    ProviderCount int
    ResourceCount int
    ConfigVersion int
}
```

**Requirements covered**: FR-021, FR-023, FR-024, NFR-001

**Acceptance criteria**:
- [ ] Generates complete config.yaml
- [ ] Merges user config over base
- [ ] Applies disabled APIs
- [ ] Returns content hash
- [ ] Deterministic output (same input → same output)

---

## Phase 3: Controller Integration

### Task 3.1: Extend Controller for v1alpha2

**Priority**: P1
**Blocked by**: Phase 1, Phase 2

**Description**:
Extend the existing controller to handle v1alpha2 resources.

**File**: `controllers/llamastackdistribution_controller.go`

**Changes**:
- Add v1alpha2 type imports
- Update Reconcile() to handle both versions
- Add helper functions for version detection

**Acceptance criteria**:
- [ ] Controller handles v1alpha2 resources
- [ ] Existing v1alpha1 behavior unchanged
- [ ] Version-specific logic isolated

---

### Task 3.2: Implement Config Source Determination

**Priority**: P1
**Blocked by**: 3.1

**Description**:
Implement logic to determine config source (generated, override, or default).

**Function**: `DetermineConfigSource(instance) ConfigSource`

**Logic**:
- If overrideConfig specified → ConfigSourceOverride
- If providers/resources/storage specified → ConfigSourceGenerated
- Otherwise → ConfigSourceDistributionDefault

**Requirements covered**: FR-013

**Acceptance criteria**:
- [ ] Correctly identifies config source
- [ ] Handles all combinations

---

### Task 3.3: Implement Generated ConfigMap Reconciliation

**Priority**: P1
**Blocked by**: 3.2

**Description**:
Implement creation and management of generated ConfigMaps.

**Function**: `ReconcileGeneratedConfigMap(ctx, instance) error`

**Logic**:
1. Call pkg/config.GenerateConfig()
2. Create ConfigMap: `{name}-config-{hash[:8]}`
3. Set owner reference
4. Clean up old ConfigMaps (keep last 2)

**Requirements covered**: FR-023, FR-024, FR-025, NFR-005

**Acceptance criteria**:
- [ ] Creates ConfigMap with hash-based name
- [ ] Sets owner reference correctly
- [ ] Cleans up old ConfigMaps
- [ ] Immutable pattern (new CM on changes)

---

### Task 3.4: Extend ManifestContext for Config

**Priority**: P1
**Blocked by**: 3.3

**Description**:
Extend ManifestContext to include generated config information.

**File**: `pkg/deploy/kustomizer.go`

**New fields**:
```go
type ManifestContext struct {
    // ... existing fields
    GeneratedConfigMapName string
    GeneratedConfigHash    string
    SecretEnvVars          []corev1.EnvVar
}
```

**Requirements covered**: FR-026

**Acceptance criteria**:
- [ ] ManifestContext includes new fields
- [ ] Deployment template uses new fields
- [ ] Hash annotation triggers rollouts

---

### Task 3.5: Implement Networking Configuration

**Priority**: P1
**Blocked by**: 3.1

**Description**:
Implement networking spec handling (port, TLS, expose, allowedFrom).

**Functions**:
- `GetServerPort(spec) int32`
- `ShouldExposeRoute(spec) bool`
- `GetExposeHostname(spec) string`
- `GetTLSConfig(spec) *TLSConfig`
- `GetCABundleVolume(spec) (*corev1.Volume, *corev1.VolumeMount)` - FR-063
- Extend existing Ingress reconciliation

**Requirements covered**: FR-060 to FR-066

**Acceptance criteria**:
- [ ] Port defaults to 8321
- [ ] Polymorphic expose works (bool and object)
- [ ] TLS configuration applied (FR-061, FR-062)
- [ ] CA bundle mounted from ConfigMap when specified (FR-063)
- [ ] Ingress/Route created with correct hostname (FR-064, FR-065)
- [ ] NetworkPolicy configured for allowedFrom (FR-066)

---

### Task 3.6: Implement Validation

**Priority**: P1
**Blocked by**: 3.1

**Description**:
Implement controller-level validation for secret and ConfigMap references.

**Functions**:
- `ValidateSecretReferences(ctx, spec, namespace) error`
- `ValidateConfigMapReferences(ctx, spec, namespace) error`
- `ValidateProviderReferences(spec) error`

**Requirements covered**: FR-073, FR-074, FR-075

**Acceptance criteria**:
- [ ] Validates all secretKeyRef references
- [ ] Validates overrideConfig and caBundle references
- [ ] Validates model → provider references
- [ ] Error messages include field paths

---

### Task 3.7: Implement Spec 001 Integration

**Priority**: P1
**Blocked by**: 3.3

**Description**:
Implement merging of external providers from spec 001.

**Function**: `MergeExternalProviders(generated, external) (*MergedConfig, []string)`

**Logic**:
1. Take generated config as base
2. Add external providers
3. Log warnings for ID conflicts
4. Return merged config and warnings

**Requirements covered**: FR-090, FR-091, FR-092

**Acceptance criteria**:
- [ ] External providers added to generated config
- [ ] ID conflicts logged as warnings
- [ ] External providers override inline

---

### Task 3.8: Extend Status Reporting

**Priority**: P1
**Blocked by**: 3.3

**Description**:
Add config generation status fields and conditions.

**File**: `controllers/status.go`

**New conditions**:
- `ConfigGenerated`: True when config successfully generated
- `DeploymentUpdated`: True when Deployment spec updated with current config
- `Available`: True when at least one Pod is ready with current config
- `SecretsResolved`: True when all secret references valid

**New status fields**:
```go
type ConfigGenerationStatus struct {
    ConfigMapName  string
    GeneratedAt    metav1.Time
    ProviderCount  int
    ResourceCount  int
    ConfigVersion  int
}

type ResolvedDistributionStatus struct {
    Image        string // Resolved image from distribution.name
    ConfigSource string // "embedded" or "oci-label"
    ConfigHash   string // Hash of current base config
}
```

**Requirements covered**: FR-020a, FR-099

**Acceptance criteria**:
- [ ] New conditions set correctly
- [ ] Config generation details in status
- [ ] `resolvedDistribution` recorded in status
- [ ] Status updated on each reconcile

---

### Task 3.9: Implement Validating Admission Webhook

**Priority**: P1
**Blocked by**: 1.7

**Description**:
Implement a validating webhook for constraints that cannot be expressed in CEL, complementing the CEL rules added in Phase 1 (Task 1.7).

**File**: `api/v1alpha2/llamastackdistribution_webhook.go`

**Validation logic**:
- Verify referenced Secrets exist in the namespace (fast admission-time feedback)
- Verify referenced ConfigMaps exist for `overrideConfig` and `caBundle`
- Validate provider ID references in `resources.models[].provider`
- Cross-field semantic validation (e.g., model provider references valid provider IDs)

**Configuration**:
- Webhook deployment via kustomize manifests (`config/webhook/`)
- Certificate management using cert-manager or operator-managed self-signed certs
- Failure policy: `Fail` (reject CR if webhook unreachable)

```go
func (r *LlamaStackDistribution) ValidateCreate() (admission.Warnings, error) {
    return r.validate()
}

func (r *LlamaStackDistribution) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
    return r.validate()
}

func (r *LlamaStackDistribution) validate() (admission.Warnings, error) {
    var allErrs field.ErrorList
    // Validate secret references exist
    // Validate ConfigMap references exist
    // Validate provider ID cross-references
    return nil, allErrs.ToAggregate()
}
```

**Requirements covered**: FR-076, FR-077, FR-078

**Acceptance criteria**:
- [ ] Webhook validates Secret existence at admission time
- [ ] Webhook validates ConfigMap references
- [ ] Webhook validates cross-field provider ID references
- [ ] Clear error messages with field paths
- [ ] Webhook kustomize manifests configured

---

### Task 3.10: Implement Runtime Configuration Update Logic

**Priority**: P1
**Blocked by**: 3.3

**Description**:
On every reconciliation, compare the generated config hash with the currently deployed config hash. Only update the Deployment when content actually changes. On failure, preserve the current running Deployment.

**File**: `controllers/llamastackdistribution_controller.go`

**Logic**:
```
Reconcile()
├── Generate config (or use overrideConfig)
├── Compute content hash of generated config
├── Compare with status.configGeneration.configMapName hash
├── If identical → skip update, no Pod restart
└── If different:
    ├── Create new ConfigMap
    ├── Update Deployment atomically (image + config + env)
    ├── On success → update status
    └── On failure → preserve current Deployment, report error
```

**Failure preservation**:
```go
func (r *Reconciler) reconcileConfig(ctx context.Context, instance *v1alpha2.LlamaStackDistribution) error {
    generated, err := config.GenerateConfig(ctx, instance.Spec, resolvedImage)
    if err != nil {
        // Preserve current running state, report error
        meta.SetStatusCondition(&instance.Status.Conditions, metav1.Condition{
            Type:    "ConfigGenerated",
            Status:  metav1.ConditionFalse,
            Reason:  "ConfigGenerationFailed",
            Message: err.Error(),
        })
        return nil  // Don't requeue, let user fix the CR
    }
    // ... proceed with update
}
```

**Requirements covered**: FR-095, FR-096, FR-097

**Acceptance criteria**:
- [ ] Config hash comparison prevents unnecessary restarts
- [ ] Failed config generation preserves current Deployment
- [ ] Error reported in status conditions on failure
- [ ] Successful update reflected in status

---

### Task 3.11: Implement Atomic Deployment Updates

**Priority**: P1
**Blocked by**: 3.10

**Description**:
When the Deployment needs updating, apply all changes (image, ConfigMap mount, env vars, hash annotation) in a single `client.Update()` call to prevent intermediate states where the running image and config are mismatched.

**File**: `controllers/llamastackdistribution_controller.go`

```go
func (r *Reconciler) updateDeploymentAtomically(
    ctx context.Context,
    deployment *appsv1.Deployment,
    resolvedImage string,
    configMapName string,
    envVars []corev1.EnvVar,
    configHash string,
) error {
    // Update all fields in one mutation
    deployment.Spec.Template.Spec.Containers[0].Image = resolvedImage
    // ... update ConfigMap volume, env vars, hash annotation
    return r.Client.Update(ctx, deployment)
}
```

**Operator upgrade handling**: When the operator is upgraded and `distributions.json` maps a name to a new image, the reconciler detects the image change via `status.resolvedDistribution.image` comparison and triggers an atomic update with the new base config.

**Requirements covered**: FR-098, FR-100, FR-101

**Acceptance criteria**:
- [ ] Image + ConfigMap + env vars updated in single API call
- [ ] No intermediate state where image and config mismatch
- [ ] Operator upgrade triggers atomic update when image changes
- [ ] Failed update preserves current Deployment (see FR-097)

---

### Task 3.12: Implement Distribution Resolution Tracking

**Priority**: P1
**Blocked by**: 3.3

**Description**:
Track the resolved image in `status.resolvedDistribution` so the controller can detect changes across reconciliations (e.g., after operator upgrade where `distributions.json` maps a name to a new image).

**File**: `controllers/llamastackdistribution_controller.go`

**Logic**:
1. Resolve `distribution.name` to concrete image using `distributions.json` + `image-overrides`
2. Compare with `status.resolvedDistribution.image`
3. If different: regenerate config with new base, update atomically
4. Record new resolved image in status

**Requirements covered**: FR-020a, FR-020b, FR-020c

**Acceptance criteria**:
- [ ] Resolved image recorded in `status.resolvedDistribution`
- [ ] Image change detected between reconciliations
- [ ] Image change triggers config regeneration and atomic update
- [ ] Config source ("embedded" or "oci-label") recorded in status

---

## Phase 4: Conversion Webhook

### Task 4.1: Implement v1alpha2 Hub

**Priority**: P2
**Blocked by**: Phase 1

**Description**:
Mark v1alpha2 as the conversion hub (storage version).

**File**: `api/v1alpha2/llamastackdistribution_conversion.go`

**Implementation**:

In controller-runtime, the Hub only implements a marker method. Conversion logic lives on the Spoke (v1alpha1).

```go
// Hub marks v1alpha2 as the storage version for conversion.
func (dst *LlamaStackDistribution) Hub() {}
```

**Requirements covered**: FR-081

**Acceptance criteria**:
- [ ] v1alpha2 implements `conversion.Hub` interface via `Hub()` marker method
- [ ] No conversion logic on the hub (all conversion is on the v1alpha1 spoke)

---

### Task 4.2: Implement v1alpha1 to v1alpha2 Conversion

**Priority**: P2
**Blocked by**: 4.1

**Description**:
Implement conversion from v1alpha1 to v1alpha2.

**File**: `api/v1alpha1/llamastackdistribution_conversion.go`

**Field mapping**:
- `spec.server.distribution` → `spec.distribution`
- `spec.server.containerSpec.port` → `spec.networking.port`
- `spec.server.containerSpec.resources` → `spec.workload.resources`
- (see spec for full mapping table)

**Requirements covered**: FR-080, FR-083

**Acceptance criteria**:
- [ ] All v1alpha1 fields converted correctly
- [ ] No data loss for existing fields
- [ ] New fields have sensible defaults

---

### Task 4.3: Implement v1alpha2 to v1alpha1 Conversion

**Priority**: P2
**Blocked by**: 4.1

**Description**:
Implement conversion from v1alpha2 to v1alpha1 (where possible).

**File**: `api/v1alpha1/llamastackdistribution_conversion.go`

**Notes**:
- New fields (providers, resources, storage) cannot be represented in v1alpha1
- These are lost in down-conversion
- Log warnings for data loss

**Requirements covered**: FR-080, FR-082

**Acceptance criteria**:
- [ ] Mappable fields converted correctly
- [ ] New fields handled gracefully (ignored with warning)
- [ ] Existing v1alpha1 CRs continue working

---

### Task 4.4: Configure and Test Webhook

**Priority**: P2
**Blocked by**: 4.2, 4.3

**Description**:
Configure conversion webhook and test thoroughly.

**Files**:
- `config/webhook/manifests.yaml`
- `main.go`

**Tests**:
- v1alpha1 → v1alpha2 → v1alpha1 round-trip
- v1alpha2 → v1alpha1 → v1alpha2 round-trip
- Edge cases (empty fields, defaults)

**Acceptance criteria**:
- [ ] Webhook registered and working
- [ ] Round-trip conversion works
- [ ] No data loss for v1alpha1 fields

---

## Phase 5: Testing & Documentation

### Task 5.1: Unit Tests for Config Package

**Priority**: P2
**Blocked by**: Phase 2

**Description**:
Write comprehensive unit tests for pkg/config.

**Files**:
- `pkg/config/config_test.go`
- `pkg/config/provider_test.go`
- `pkg/config/resource_test.go`
- `pkg/config/storage_test.go`
- `pkg/config/secret_resolver_test.go`
- `pkg/config/version_test.go`

**Coverage target**: >80%

**Acceptance criteria**:
- [ ] All functions tested
- [ ] Edge cases covered
- [ ] Determinism verified

---

### Task 5.2: Integration Tests for Controller

**Priority**: P2
**Blocked by**: Phase 3

**Description**:
Write integration tests for v1alpha2 controller logic.

**File**: `controllers/llamastackdistribution_v1alpha2_test.go`

**Test scenarios**:
- Simple inference configuration
- Multiple providers
- Resource registration
- State storage configuration
- Network exposure
- Override config
- Validation errors
- Runtime config updates (US8): CR update triggers config regeneration
- Atomic Deployment updates: image + config updated together
- Webhook validation: invalid references rejected at admission
- Distribution resolution tracking: operator upgrade triggers update
- Config generation failure: current Deployment preserved

**Acceptance criteria**:
- [ ] All user stories have tests (including US8)
- [ ] Edge cases covered
- [ ] Error scenarios tested
- [ ] Webhook validation tested
- [ ] Atomic update scenarios tested

---

### Task 5.3: Conversion Tests

**Priority**: P2
**Blocked by**: Phase 4

**Description**:
Write tests for conversion webhook.

**File**: `api/v1alpha2/conversion_test.go`

**Test scenarios**:
- v1alpha1 → v1alpha2 for all field combinations
- v1alpha2 → v1alpha1 with data preservation
- Round-trip conversion

**Acceptance criteria**:
- [ ] All field mappings tested
- [ ] Data loss scenarios documented
- [ ] Round-trip works

---

### Task 5.4: Sample Manifests

**Priority**: P2
**Blocked by**: Phase 3

**Description**:
Create sample v1alpha2 manifests for users.

**Files**:
- `config/samples/v1alpha2-simple.yaml`
- `config/samples/v1alpha2-full.yaml`
- `config/samples/v1alpha2-postgres.yaml`
- `config/samples/v1alpha2-multi-provider.yaml`
- `config/samples/v1alpha2-override.yaml`

**Acceptance criteria**:
- [ ] Samples are valid and deploy successfully
- [ ] Cover common use cases
- [ ] Include inline comments

---

### Task 5.5: Documentation

**Priority**: P2
**Blocked by**: Phase 3, Phase 4

**Description**:
Update documentation for v1alpha2.

**Files**:
- `README.md` - Add v1alpha2 overview
- `docs/configuration.md` - Detailed config guide
- `docs/migration-v1alpha1-to-v1alpha2.md` - Migration guide
- `docs/api-reference.md` - API reference update

**Acceptance criteria**:
- [ ] All new features documented
- [ ] Migration path clear
- [ ] Examples included

---

### Task 5.6: Performance Benchmarks

**Priority**: P2
**Blocked by**: 2.8

**Description**:
Write Go benchmark tests to verify config generation completes within the NFR-002 threshold (5 seconds for typical configurations).

**File**: `pkg/config/config_benchmark_test.go`

**Benchmark scenarios**:
- Single provider, single model (minimal config)
- 5 providers, 10 models, storage, networking (typical production)
- 10 providers, 50 models, all features enabled (stress test)

**Requirements covered**: NFR-002

**Acceptance criteria**:
- [ ] Benchmark tests pass under 5 seconds for typical configuration
- [ ] Results documented in test output
- [ ] CI runs benchmarks (optional, for regression detection)

---

## Task Dependencies Graph

```
Phase 1 (CRD Schema)
├── 1.1 ─► 1.2, 1.3, 1.4, 1.5, 1.6
├── 1.2 ─┐
├── 1.3 ─┤
├── 1.4 ─┼─► 1.7 ─► 1.8
├── 1.5 ─┤
└── 1.6 ─┘

Phase 2 (Config Generation)
├── 2.1 ─► 2.2, 2.4, 2.6
├── 2.2 ─► 2.3
├── 2.4 ─┐
├── 2.5 ─┼─► 2.7 ─► 2.8
└── 2.6 ─┘

Phase 3 (Controller)
├── 3.1 ─► 3.2, 3.5, 3.6
├── 3.2 ─► 3.3
├── 3.3 ─► 3.4, 3.7, 3.8, 3.10, 3.12
├── 3.10 ─► 3.11
├── 3.9 (blocked by 1.7)
└── 3.12 (parallel with 3.10)

Phase 4 (Webhook)
└── 4.1 ─► 4.2, 4.3 ─► 4.4

Phase 5 (Testing)
├── 5.1 (blocked by Phase 2)
├── 5.2 (blocked by Phase 3)
├── 5.3 (blocked by Phase 4)
├── 5.4, 5.5 (blocked by Phase 3, 4)
└── 5.6 (blocked by 2.8)
```
