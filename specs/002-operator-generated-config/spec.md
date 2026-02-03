# Feature: Operator-Generated Server Configuration (v1alpha2)

**Status**: Draft
**Created**: 2026-02-02
**Priority**: P1
**Depends on**: 001-deploy-time-providers-l1 (external providers merge into generated config)
**Design Doc**: [LlamaStackDistribution CRD v1alpha2 Schema Design](https://docs.google.com/document/d/10VhoQPb8bLGUo9yka4MXuEGIZClGf1oBr31TpK4NLD0/edit)

## Purpose

Enable the llama-stack Kubernetes operator to generate the server configuration (config.yaml) from a high-level, abstracted specification in the LlamaStackDistribution CR v1alpha2, rather than requiring users to provide a complete ConfigMap. This empowers users to configure LlamaStack with minimal YAML while preserving the flexibility to override any setting when needed.

## User Scenarios & Testing

### User Story 1 - Simple Inference Configuration (Priority: P1)

As a developer, I want to deploy a llama-stack instance with a vLLM backend using just a few lines of YAML, so that I can get started quickly without understanding the full config.yaml schema.

**Why this priority**: Core value proposition. Most users need simple inference configuration.

**Independent Test**: Deploy a LLSD CR with minimal `providers.inference` configuration and verify the server starts with the provider accessible via the `/v1/providers` API.

**Acceptance Scenarios**:

1. **Given** a LLSD CR with `providers.inference: {provider: vllm, endpoint: "http://vllm:8000"}`, **When** I apply the CR, **Then** the operator generates a valid config.yaml with the vLLM provider configured
2. **Given** a LLSD CR with `providers.inference.apiKey.secretKeyRef`, **When** I apply the CR, **Then** the secret value is injected via environment variable and the provider can authenticate
3. **Given** a LLSD CR with only `distribution.name: starter`, **When** I apply the CR, **Then** the distribution's default config.yaml is used unchanged

### User Story 2 - Multiple Providers Configuration (Priority: P1)

As a platform engineer, I want to configure multiple inference providers (primary and fallback) in a single LLSD, so that I can provide high availability without managing complex ConfigMaps.

**Why this priority**: Production deployments commonly require multiple providers.

**Independent Test**: Deploy a LLSD CR with multiple inference providers using list form, verify all providers appear in the `/v1/providers` API.

**Acceptance Scenarios**:

1. **Given** a LLSD CR with `providers.inference` as a list of two providers with explicit IDs, **When** I apply the CR, **Then** both providers are configured and accessible
2. **Given** a LLSD CR with multiple providers without explicit IDs, **When** I apply the CR, **Then** validation fails with a clear error message requiring explicit IDs
3. **Given** a LLSD CR with duplicate provider IDs, **When** I apply the CR, **Then** validation fails with a clear error message listing the duplicate IDs

### User Story 3 - Resource Registration (Priority: P1)

As a developer, I want to register models and tools declaratively in the CR, so that they are available immediately when the server starts.

**Why this priority**: Models and tools are essential for using llama-stack.

**Independent Test**: Deploy a LLSD CR with `resources.models` and `resources.tools`, verify resources appear in the respective API endpoints.

**Acceptance Scenarios**:

1. **Given** a LLSD CR with `resources.models: ["llama3.2-8b"]`, **When** I apply the CR, **Then** the model is registered with the first configured inference provider
2. **Given** a LLSD CR with a model specifying explicit provider assignment, **When** I apply the CR, **Then** the model is registered with the specified provider
3. **Given** a LLSD CR with `resources.tools: [websearch, rag]`, **When** I apply the CR, **Then** the tool groups are registered and available

### User Story 4 - State Storage Configuration (Priority: P1)

As a platform operator, I want to configure PostgreSQL for state storage, so that llama-stack data persists across restarts and scales horizontally.

**Why this priority**: Production deployments require persistent storage.

**Independent Test**: Deploy a LLSD CR with `storage.sql` configuration, verify the server uses PostgreSQL for state storage.

**Acceptance Scenarios**:

1. **Given** a LLSD CR with `storage.sql: {type: postgres, connectionString: {secretKeyRef: ...}}`, **When** I apply the CR, **Then** the server uses PostgreSQL for SQL storage
2. **Given** a LLSD CR with `storage.kv: {type: redis, endpoint: "..."}`, **When** I apply the CR, **Then** the server uses Redis for key-value storage
3. **Given** a LLSD CR without storage configuration, **When** I apply the CR, **Then** the server uses default SQLite storage

### User Story 5 - Network Exposure Configuration (Priority: P2)

As a platform operator, I want to expose the llama-stack service externally with TLS, so that clients can access it securely from outside the cluster.

**Why this priority**: External access is a common production requirement.

**Independent Test**: Deploy a LLSD CR with `networking.expose: true` and `networking.tls`, verify Ingress/Route is created with TLS configured.

**Acceptance Scenarios**:

1. **Given** a LLSD CR with `networking.expose: true`, **When** I apply the CR, **Then** an Ingress/Route is created with an auto-generated hostname
2. **Given** a LLSD CR with `networking.expose: {hostname: "llama.example.com"}`, **When** I apply the CR, **Then** an Ingress/Route is created with the specified hostname
3. **Given** a LLSD CR with `networking.tls: {enabled: true, secretName: "..."}`, **When** I apply the CR, **Then** the server uses the specified TLS certificate

### User Story 6 - Full ConfigMap Override (Priority: P2)

As a power user, I want to provide my own complete config.yaml via ConfigMap, so that I can use features not yet exposed in the CRD schema.

**Why this priority**: Escape hatch for advanced use cases.

**Independent Test**: Deploy a LLSD CR with `overrideConfig.configMapName`, verify the server uses the ConfigMap contents.

**Acceptance Scenarios**:

1. **Given** a LLSD CR with `overrideConfig.configMapName: my-config`, **When** I apply the CR, **Then** the server uses the ConfigMap's config.yaml
2. **Given** a LLSD CR with both `providers` and `overrideConfig`, **When** I apply the CR, **Then** validation fails with a mutual exclusivity error
3. **Given** a LLSD CR with `overrideConfig` referencing a non-existent ConfigMap, **When** I apply the CR, **Then** the status shows a clear error about the missing ConfigMap

### User Story 7 - Migration from v1alpha1 (Priority: P2)

As an existing user, I want my v1alpha1 CRs to continue working after the operator upgrade, so that I don't experience downtime during the transition.

**Why this priority**: Backward compatibility is essential for adoption.

**Independent Test**: Apply a v1alpha1 CR, upgrade operator, verify the CR continues to work and can be retrieved as v1alpha2.

**Acceptance Scenarios**:

1. **Given** an existing v1alpha1 LLSD CR, **When** I upgrade the operator to support v1alpha2, **Then** the existing CR continues to function
2. **Given** a v1alpha1 CR, **When** I retrieve it as v1alpha2, **Then** the conversion webhook translates fields correctly
3. **Given** a v1alpha2 CR, **When** I retrieve it as v1alpha1, **Then** the conversion webhook translates fields correctly (where mappable)

### Edge Cases

- **Provider with settings escape hatch**:
  - What: User specifies `providers.inference.settings: {max_tokens: 8192}`
  - Expected: Extra fields are passed through to config.yaml provider config section

- **Secret reference to non-existent secret**:
  - What: `secretKeyRef` points to a secret that doesn't exist
  - Expected: Reconciliation fails with clear error, status shows "Secret not found: {name}"

- **Polymorphic expose with empty object**:
  - What: User specifies `expose: {}`
  - Expected: Treated as `expose: true` (enabled with defaults)

- **Disabled APIs conflict with providers**:
  - What: User configures `providers.inference` but also `disabled: [inference]`
  - Expected: Warning logged, disabled takes precedence, provider config is ignored

- **Model references non-existent provider**:
  - What: `resources.models[].provider` references an ID not in `providers`
  - Expected: Validation fails with clear error listing available provider IDs

- **Config.yaml schema version mismatch**:
  - What: Distribution image has unsupported config.yaml version
  - Expected: Reconciliation fails with clear error about version incompatibility

## Requirements

### Functional Requirements

#### CRD Schema (v1alpha2)

- **FR-001**: The CRD MUST define a new API version `v1alpha2` with the redesigned schema
- **FR-002**: The `spec.distribution` field MUST support both `name` (mapped) and `image` (direct) forms, mutually exclusive
- **FR-003**: The `spec.providers` section MUST support provider types: `inference`, `safety`, `vectorIo`, `toolRuntime`, `telemetry`
- **FR-004**: Each provider MUST support polymorphic form: single object OR list of objects with explicit `id` field
- **FR-005**: Each provider MUST support fields: `provider` (type), `endpoint`, `apiKey` (secretKeyRef), `settings` (escape hatch)
- **FR-006**: The `spec.resources` section MUST support: `models`, `tools`, `shields`
- **FR-007**: Resources MUST support polymorphic form: simple string OR object with metadata
- **FR-008**: The `spec.storage` section MUST have subsections: `kv` (key-value) and `sql` (relational)
- **FR-009**: The `spec.disabled` field MUST be a list of API names to disable
- **FR-010**: The `spec.networking` section MUST consolidate: `port`, `tls`, `expose`, `allowedFrom`
- **FR-011**: The `networking.expose` field MUST support polymorphic form: boolean OR object with `hostname`
- **FR-012**: The `spec.workload` section MUST contain K8s deployment settings: `replicas`, `workers`, `resources`, `autoscaling`, `storage`, `podDisruptionBudget`, `topologySpreadConstraints`, `overrides`
- **FR-013**: The `spec.overrideConfig` field MUST be mutually exclusive with `providers`, `resources`, `storage`, `disabled`
- **FR-014**: The `spec.externalProviders` field MUST remain for integration with spec 001

#### Configuration Generation

- **FR-020**: The operator MUST extract base config.yaml from the distribution container image
- **FR-021**: The operator MUST generate a complete config.yaml by merging user configuration over base defaults
- **FR-022**: The operator MUST resolve `secretKeyRef` references to environment variables with deterministic naming
- **FR-023**: The operator MUST create a ConfigMap containing the generated config.yaml
- **FR-024**: The ConfigMap name MUST include a content hash for change detection
- **FR-025**: The operator MUST set owner references on the generated ConfigMap for garbage collection
- **FR-026**: The operator MUST add a hash annotation to the Deployment to trigger rollouts on config changes
- **FR-027**: The operator MUST detect the config.yaml schema version from the base configuration
- **FR-028**: The operator MUST support config.yaml schema versions n and n-1 (current and previous)
- **FR-029**: The operator MUST reject unsupported config.yaml versions with error: "Unsupported config.yaml version {version}. Supported versions: {list}"

#### Base Config Extraction from OCI Images

- **FR-027a**: Distribution images SHOULD include `config.yaml` in OCI labels using one of:
  - `io.llamastack.config.base64`: Base64-encoded config (for configs < 50KB)
  - `io.llamastack.config.layer` + `io.llamastack.config.path`: Layer digest and path reference (for larger configs)
- **FR-027b**: The operator MUST extract base config from OCI labels in priority order: `base64` → `layer reference`
- **FR-027c**: The operator MUST use `k8schain` authentication to access registries using the same credentials as kubelet (imagePullSecrets, ServiceAccount)
- **FR-027d**: The operator MUST cache extracted configs by image digest to avoid repeated registry fetches
- **FR-027e**: When distribution image lacks config labels, the operator MUST:
  1. Set status condition `ConfigGenerated=False` with reason `MissingConfigLabels`
  2. Return error: "Distribution image {image} missing config labels. Add labels using `crane mutate` or use `overrideConfig` to provide configuration manually. See docs/configuration.md for details."
- **FR-027f**: The operator MUST support air-gapped environments where images are mirrored to internal registries
- **FR-027g**: When OCI labels are missing, users MAY use `overrideConfig.configMapName` as a workaround to provide full configuration manually

#### Provider Configuration

- **FR-030**: Provider `provider` field MUST map to `provider_type` with `remote::` prefix (e.g., `vllm` becomes `remote::vllm`)
- **FR-031**: Provider `endpoint` field MUST map to `config.url` in config.yaml
- **FR-032**: Provider `apiKey.secretKeyRef` MUST be resolved to an environment variable and referenced as `${env.LLSD_<PROVIDER>_API_KEY}`
- **FR-033**: Provider `settings` MUST be merged into the provider's `config` section in config.yaml
- **FR-034**: When multiple providers are specified, each MUST have an explicit `id` field
- **FR-035**: Single provider without `id` MUST auto-generate `provider_id` from the `provider` field value

#### Resource Registration

- **FR-040**: Simple model strings MUST be registered with the inference provider. When multiple inference providers are configured (list form), the first provider in list order is used. When a single provider is configured (object form), that provider is used.
- **FR-041**: Model objects with explicit `provider` MUST be registered with the specified provider
- **FR-042**: Model objects MAY include metadata fields: `contextLength`, `modelType`, `quantization`
- **FR-043**: Tools MUST be registered as tool groups with default provider configuration
- **FR-044**: Shields MUST be registered with the configured safety provider

#### Storage Configuration

- **FR-050**: `storage.kv.type` MUST support values: `sqlite`, `redis`
- **FR-050a**: `storage.kv` for type `redis` MUST support fields: `endpoint`, `password` (secretKeyRef)
- **FR-050b**: When `storage.kv` is specified without `type`, the operator MUST default to `sqlite`
- **FR-051**: `storage.sql.type` MUST support values: `sqlite`, `postgres`
- **FR-051a**: When `storage.sql` is specified without `type`, the operator MUST default to `sqlite`
- **FR-052**: `storage.sql.connectionString` MUST support `secretKeyRef` for secure credential handling
- **FR-053**: When `storage` section is not specified at all, distribution defaults MUST be preserved (no override)

#### Networking Configuration

- **FR-060**: `networking.port` MUST default to 8321 if not specified
- **FR-061**: `networking.tls.enabled: true` MUST configure the server for TLS
- **FR-062**: `networking.tls.secretName` MUST reference a Kubernetes TLS secret
- **FR-063**: `networking.tls.caBundle` MUST support custom CA certificates via ConfigMap reference
- **FR-064**: `networking.expose: true` MUST create an Ingress/Route with auto-generated hostname
- **FR-065**: `networking.expose.hostname` MUST create an Ingress/Route with the specified hostname
- **FR-066**: `networking.allowedFrom` MUST configure NetworkPolicy for namespace-based access control

#### Validation

- **FR-070**: CEL validation MUST enforce mutual exclusivity between `providers` and `overrideConfig`
- **FR-071**: CEL validation MUST require explicit `id` when multiple providers are specified for the same API
- **FR-072**: CEL validation MUST enforce unique provider IDs across all provider types
- **FR-073**: Controller validation MUST verify referenced Secrets exist before generating config
- **FR-074**: Controller validation MUST verify referenced ConfigMaps exist for `overrideConfig` and `caBundle`
- **FR-075**: Validation errors MUST include actionable messages with field paths

#### API Version Conversion

- **FR-080**: A conversion webhook MUST translate between v1alpha1 and v1alpha2
- **FR-081**: v1alpha2 MUST be the storage version
- **FR-082**: v1alpha1 CRs MUST continue to be served for backward compatibility
- **FR-083**: Field mapping MUST follow the documented migration table (see Design Doc)

#### Integration with Spec 001

- **FR-090**: Generated config MUST serve as the base for external provider merging
- **FR-091**: External providers (from spec 001) MUST be additive to inline providers
- **FR-092**: When external provider ID conflicts with inline provider, external MUST override with warning

### Non-Functional Requirements

- **NFR-001**: Configuration generation MUST be deterministic (same inputs produce same outputs)
- **NFR-002**: Configuration generation MUST complete within 5 seconds for typical configurations
- **NFR-003**: Secret resolution MUST NOT expose secret values in logs or status
- **NFR-004**: Error messages MUST be actionable (user can resolve without operator knowledge)
- **NFR-005**: The generated ConfigMap MUST be immutable (new ConfigMap on changes, not updates)
- **NFR-006**: Config extraction from images MUST be cached to avoid repeated image pulls

### External Dependencies

#### Distribution Image Build Requirements

Distribution images must include OCI labels for base config extraction. These labels are added post-build using `crane mutate`:

| Label | Description | Required |
|-------|-------------|----------|
| `io.llamastack.config.base64` | Base64-encoded config.yaml | For configs < 50KB |
| `io.llamastack.config.layer` | Layer digest containing config | For configs >= 50KB |
| `io.llamastack.config.path` | Path to config.yaml within layer | Required with `layer` |
| `io.llamastack.config.version` | Config schema version (e.g., "2") | Recommended |

**Build Process**:
```bash
# 1. Build and push image
docker build -t ${IMAGE}:build . && docker push ${IMAGE}:build

# 2. Add labels post-build (crane mutate updates config blob only, layers unchanged)
crane mutate ${IMAGE}:build \
  --label "io.llamastack.config.base64=$(crane export ${IMAGE}:build - | tar -xO app/config.yaml | base64 -w0)" \
  --label "io.llamastack.config.version=2" \
  -t ${IMAGE}:latest
```

**Why post-build**: Labels containing layer digests cannot be added during build (the layer digest is only known after build). `crane mutate` solves this by updating only the config blob without modifying layers.

### Key Entities

- **ProviderSpec**: Configuration for a single provider (inference, safety, vectorIo, etc.)
- **ResourceSpec**: Configuration for registered resources (models, tools, shields)
- **StorageSpec**: Configuration for state storage (kv and sql backends)
- **NetworkingSpec**: Configuration for network exposure (port, TLS, expose, allowedFrom)
- **WorkloadSpec**: Kubernetes deployment settings (replicas, resources, autoscaling)
- **ExposeConfig**: Polymorphic expose configuration (bool or object with hostname)

## CRD Schema

### Complete v1alpha2 Spec Structure

```yaml
apiVersion: llamastack.ai/v1alpha2
kind: LlamaStackDistribution
metadata:
  name: my-stack
spec:
  distribution:
    name: starter              # OR image: "registry/image:tag"

  providers:
    inference:
      provider: vllm
      endpoint: "http://vllm:8000"
      apiKey:
        secretKeyRef: {name: vllm-creds, key: token}
      settings:
        max_tokens: 8192
    safety:
      provider: llama-guard
    vectorIo:
      provider: pgvector
      host:
        secretKeyRef: {name: pg-creds, key: host}

  resources:
    models:
      - "llama3.2-8b"
      - name: "llama3.2-70b"
        provider: vllm-primary
        contextLength: 128000
    tools:
      - websearch
      - rag
    shields:
      - llama-guard

  storage:
    kv:
      type: sqlite
    sql:
      type: postgres
      connectionString:
        secretKeyRef: {name: pg-creds, key: url}

  disabled:
    - postTraining
    - eval

  networking:
    port: 8321
    tls:
      enabled: true
      secretName: llama-tls
      caBundle:
        configMapName: custom-ca
    expose: true
    # OR: expose: {hostname: "llama.example.com"}
    allowedFrom:
      namespaces: ["app-ns"]
      labels: ["llama-access"]

  workload:
    replicas: 1
    workers: 2
    resources:
      requests: {cpu: "500m", memory: "1Gi"}
      limits: {cpu: "2", memory: "4Gi"}
    autoscaling:
      minReplicas: 1
      maxReplicas: 5
      targetCPUUtilization: 80
    storage:
      size: "10Gi"
      mountPath: "/.llama"
    podDisruptionBudget:
      minAvailable: 1
    topologySpreadConstraints: []
    overrides:
      serviceAccountName: "custom-sa"
      env: []
      command: []
      args: []
      volumes: []
      volumeMounts: []

  externalProviders:
    inference:
      - providerId: custom-inference
        image: registry.example.com/custom:v1

  # overrideConfig:
  #   configMapName: my-full-config
```

### Field Mapping: v1alpha1 to v1alpha2

| v1alpha1 Path | v1alpha2 Path |
|---------------|---------------|
| `spec.server.distribution` | `spec.distribution` |
| `spec.server.containerSpec.port` | `spec.networking.port` |
| `spec.server.containerSpec.resources` | `spec.workload.resources` |
| `spec.server.containerSpec.env` | `spec.workload.overrides.env` |
| `spec.server.containerSpec.command` | `spec.workload.overrides.command` |
| `spec.server.containerSpec.args` | `spec.workload.overrides.args` |
| `spec.server.userConfig` | `spec.overrideConfig` |
| `spec.server.storage` | `spec.workload.storage` |
| `spec.server.tlsConfig.caBundle` | `spec.networking.tls.caBundle` |
| `spec.server.autoscaling` | `spec.workload.autoscaling` |
| `spec.server.workers` | `spec.workload.workers` |
| `spec.server.podOverrides` | `spec.workload.overrides` |
| `spec.server.podDisruptionBudget` | `spec.workload.podDisruptionBudget` |
| `spec.server.topologySpreadConstraints` | `spec.workload.topologySpreadConstraints` |
| `spec.replicas` | `spec.workload.replicas` |
| `spec.network.exposeRoute` | `spec.networking.expose` |
| `spec.network.allowedFrom` | `spec.networking.allowedFrom` |
| *(new)* | `spec.providers` |
| *(new)* | `spec.resources` |
| *(new)* | `spec.storage` |
| *(new)* | `spec.disabled` |

## Controller Flow

```
┌─────────────────────────────────────────────────────────────────┐
│                      RECONCILIATION FLOW                        │
└─────────────────────────────────────────────────────────────────┘

1. Fetch LLSD CR
        │
        ▼
2. Validate Configuration
   ├── Check mutual exclusivity (providers vs overrideConfig)
   ├── Validate secret references exist
   └── Validate ConfigMap references exist
        │
        ▼
3. Determine Config Source
   ├── If overrideConfig: Use referenced ConfigMap directly
   └── If providers/resources: Generate config
        │
        ▼
4. Generate Configuration (if not using overrideConfig)
   ├── Extract base config.yaml from distribution image
   ├── Expand providers to full config.yaml format
   ├── Expand resources to registered_resources format
   ├── Apply storage configuration
   ├── Apply disabled APIs
   └── Resolve secretKeyRef to environment variables
        │
        ▼
5. Create/Update ConfigMap
   ├── Generate ConfigMap with content hash in name
   ├── Set owner reference
   └── Create new ConfigMap (immutable pattern)
        │
        ▼
6. Update Deployment
   ├── Mount generated ConfigMap
   ├── Inject environment variables for secrets
   └── Add hash annotation for rollout trigger
        │
        ▼
7. Merge External Providers (from spec 001)
   ├── Add external providers to config
   └── Override on ID conflict (with warning)
        │
        ▼
8. Update Status
   ├── Set phase
   ├── Update conditions
   └── Record config generation details
```

## Configuration Tiers

| Tier | Use Case | Mechanism | Example |
|------|----------|-----------|---------|
| 1 | Simple (80%) | Inline provider fields | `providers.inference: {provider: vllm, endpoint: "..."}` |
| 2 | Advanced (15%) | Per-provider settings | `providers.inference: {..., settings: {max_tokens: 8192}}` |
| 3 | Full Control (5%) | ConfigMap override | `overrideConfig: {configMapName: my-config}` |

## Status Reporting

The status MUST include:

```yaml
status:
  phase: Ready
  conditions:
    - type: ConfigGenerated
      status: "True"
      reason: ConfigGenerationSucceeded
      message: "Generated config.yaml with 3 providers and 2 models"
    - type: SecretsResolved
      status: "True"
      reason: AllSecretsFound
      message: "Resolved 2 secret references"
  configGeneration:
    configMapName: my-stack-config-abc123
    generatedAt: "2026-02-02T12:00:00Z"
    providerCount: 3
    resourceCount: 2
```

## Security Considerations

- **Secret Handling**: Secret values MUST only be passed via environment variables, never embedded in ConfigMap
- **Environment Variable Naming**: Use deterministic, prefixed names: `LLSD_<PROVIDER>_<FIELD>` (e.g., `LLSD_INFERENCE_API_KEY`)
- **ConfigMap Permissions**: Generated ConfigMaps inherit namespace RBAC
- **Image Extraction**: Config extraction from images uses read-only operations

## References

- Design Document: [LlamaStackDistribution CRD v1alpha2 Schema Design](https://docs.google.com/document/d/10VhoQPb8bLGUo9yka4MXuEGIZClGf1oBr31TpK4NLD0/edit)
- Kubernetes API Conventions: https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md
- Gateway API Design Guide: https://gateway-api.sigs.k8s.io/guides/api-design/
- Kubebuilder Good Practices: https://book.kubebuilder.io/reference/good-practices
- Related Spec: 001-deploy-time-providers-l1 (External Provider Injection)
