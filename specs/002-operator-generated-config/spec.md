# Feature Specification: Operator-Generated Server Configuration

**Feature Branch**: `002-operator-generated-config`
**Created**: 2026-02-02
**Status**: Draft
**Priority**: P1
**Related**: Spec 001 (Deploy-Time Providers), RHAISTRAT-1061
**Design Decisions**: [Google Doc](https://docs.google.com/document/d/1Txelyq3juJbfNe1HOzuR4XuIJcm0lefo-Fchup6XINY/edit)

## Purpose

Enable the llama-stack Kubernetes operator to generate server configuration (`config.yaml`) from a high-level, abstracted specification in the LlamaStackDistribution CR, rather than requiring users to provide a complete ConfigMap. This empowers users to configure LlamaStack with minimal YAML while preserving distribution defaults and providing Kubernetes-native secret handling.

## Goals

- Enable users to configure LlamaStack with minimal YAML
- Preserve distribution's default configuration, only override specified fields
- Provide Kubernetes-native secret handling via `secretKeyRef`
- Validate configuration at apply time
- Support seamless upgrades when llama-stack config schema changes
- Maintain backward compatibility with existing `userConfig` ConfigMap approach
- Mirror `config.yaml` structure for principle of least surprise

## Non-Goals

- 1:1 mapping of every `config.yaml` field to LLSD spec
- Support for `config.yaml` version 1 schema
- Runtime configuration changes (requires pod restart)
- Hot reloading of configuration

## User Scenarios & Testing

### User Story 1 - Simple Provider Configuration (Priority: P1)

As a platform operator, I want to configure a basic LlamaStack deployment with a single inference provider using minimal YAML, so that I can get started quickly without understanding the full config.yaml schema.

**Why this priority**: Core value proposition. This is the 80% use case where users need one inference provider with basic settings.

**Independent Test**: Deploy LLSD CR with single inference provider configuration and verify the generated config.yaml contains correct provider settings and the server starts successfully.

**Acceptance Scenarios**:

1. **Given** a LLSD CR with `providers.inference.provider: vllm` and `providers.inference.endpoint: "http://vllm:8000"`, **When** the operator reconciles, **Then** the generated config.yaml contains a properly formatted inference provider with `provider_type: remote::vllm` and `config.url: "http://vllm:8000"`

2. **Given** a LLSD CR with inference provider and storage configuration, **When** the operator reconciles, **Then** both providers and storage sections are correctly generated in config.yaml

3. **Given** a LLSD CR with `providers.inference.apiKey.secretKeyRef`, **When** the operator reconciles, **Then** the secret value is injected via environment variable and config.yaml references `${env.LLSD_INFERENCE_API_KEY}`

---

### User Story 2 - Multiple Providers Configuration (Priority: P2)

As a platform operator, I want to configure multiple inference providers (e.g., primary vLLM and fallback Ollama), so that I can support different models or provide redundancy.

**Why this priority**: Common advanced use case for production deployments requiring multiple backends.

**Independent Test**: Deploy LLSD CR with list of inference providers and verify all providers appear in generated config.yaml with correct IDs.

**Acceptance Scenarios**:

1. **Given** a LLSD CR with `providers.inference` as a list containing two providers with explicit `id` fields, **When** the operator reconciles, **Then** both providers appear in config.yaml with their specified `provider_id` values

2. **Given** a LLSD CR with multiple providers in list form but missing `id` field, **When** the operator validates, **Then** validation fails with clear error message requiring explicit IDs for multiple providers

3. **Given** a LLSD CR with duplicate provider IDs across providers, **When** the operator validates, **Then** validation fails with error listing the duplicate ID

---

### User Story 3 - Per-Provider Custom Configuration (Priority: P2)

As a platform operator, I want to pass additional config.yaml fields that aren't exposed in the simplified schema, so that I can customize provider behavior without reverting to full ConfigMap.

**Why this priority**: Enables power users to extend beyond simplified schema without losing its benefits.

**Independent Test**: Deploy LLSD CR with `providers.inference.config` containing custom fields and verify they appear in generated config.yaml.

**Acceptance Scenarios**:

1. **Given** a LLSD CR with `providers.inference.config.max_tokens: 8192`, **When** the operator reconciles, **Then** the generated config.yaml contains `config.max_tokens: 8192` in the inference provider section

2. **Given** a LLSD CR with both simplified fields (`endpoint`) and custom config fields, **When** the operator reconciles, **Then** simplified fields are mapped correctly and custom config fields are passed through as-is

---

### User Story 4 - Resource Registration (Priority: P2)

As a platform operator, I want to register models, tools, and shields using a simplified syntax, so that I can configure resources without understanding the full registered_resources schema.

**Why this priority**: Completes the configuration story by handling resources, not just providers.

**Independent Test**: Deploy LLSD CR with `resources.models` list and verify models appear in config.yaml's `registered_resources` section.

**Acceptance Scenarios**:

1. **Given** a LLSD CR with `resources.models: ["llama3.2-8b", "llama3.2-70b"]` (simple strings), **When** the operator reconciles, **Then** both models are registered using the first configured inference provider

2. **Given** a LLSD CR with detailed model entry including `provider` field, **When** the operator reconciles, **Then** the model is registered with the specified provider

3. **Given** a LLSD CR with `resources.tools: ["websearch", "rag"]`, **When** the operator reconciles, **Then** the tools appear in registered_resources.tool_groups

---

### User Story 5 - Disable Providers/APIs (Priority: P3)

As a platform operator, I want to explicitly disable certain provider types (e.g., post_training, eval), so that I can reduce attack surface and resource usage.

**Why this priority**: Security and resource optimization for production deployments.

**Independent Test**: Deploy LLSD CR with `disabled` list and verify those APIs are not present in generated config.yaml.

**Acceptance Scenarios**:

1. **Given** a LLSD CR with `disabled: [postTraining, eval]`, **When** the operator reconciles, **Then** the generated config.yaml does not include post_training or eval provider sections

2. **Given** a distribution with default providers for postTraining, **When** LLSD CR includes `disabled: [postTraining]`, **Then** the distribution default is removed from generated config.yaml

---

### User Story 6 - ConfigMap Escape Hatch (Priority: P3)

As a power user, I want to use a full ConfigMap for complete control while still benefiting from operator lifecycle management, so that I can handle edge cases not covered by the simplified schema.

**Why this priority**: Maintains backward compatibility and provides escape hatch for complex scenarios.

**Independent Test**: Deploy LLSD CR with `userConfig.configMapName` and verify the operator uses the ConfigMap directly without modification.

**Acceptance Scenarios**:

1. **Given** a LLSD CR with `userConfig.configMapName: my-full-config`, **When** the operator reconciles, **Then** the operator uses the ConfigMap content as-is for config.yaml

2. **Given** a LLSD CR with both `userConfig.configMapName` and `providers` fields, **When** the operator validates, **Then** validation fails with error: "spec.server.providers and spec.server.userConfig are mutually exclusive"

---

### Edge Cases

- **Empty providers section**: Distribution defaults are used unchanged
- **Provider type not recognized**: Validation error with list of supported provider types
- **Secret reference doesn't exist**: Reconciliation fails with clear error identifying missing secret
- **Invalid provider combination**: Validation based on llama-stack compatibility matrix
- **Schema version mismatch**: Operator logs warning if distribution uses unsupported config.yaml version
- **Single provider written as list**: Accept and normalize to single provider internally
- **List provider written as single**: Accept single provider without requiring `id` field

## Requirements

### Functional Requirements

#### CRD Schema

- **FR-001**: LLSD CRD MUST add `providers` field to `ServerSpec` with subfields for each provider type (inference, safety, vectorIo, agents, etc.)
- **FR-002**: Each provider type MUST accept either a single provider object OR a list of provider objects
- **FR-003**: Single provider objects MUST auto-generate `provider_id` from provider type name
- **FR-004**: List provider objects MUST require explicit `id` field for each provider
- **FR-005**: Provider objects MUST support `provider`, `endpoint`, `apiKey`, and `config` fields
- **FR-006**: `apiKey` field MUST support `secretKeyRef` for Kubernetes-native secret handling
- **FR-007**: `config` field MUST accept arbitrary key-value pairs for pass-through to config.yaml
- **FR-008**: LLSD CRD MUST add `disabled` field accepting list of provider type names to disable
- **FR-009**: LLSD CRD MUST add `storage` field with `type` and `connectionString` subfields
- **FR-010**: LLSD CRD MUST add `resources` field with `models`, `tools`, and `shields` subfields
- **FR-011**: LLSD CRD MUST add `port` and `tls` fields at server level for common settings
- **FR-012**: `providers` and `userConfig.configMapName` MUST be mutually exclusive (CEL validation)

#### Configuration Generation

- **FR-013**: Operator MUST read distribution's embedded config.yaml as base configuration
- **FR-014**: Operator MUST expand simplified provider fields to full config.yaml structure
- **FR-015**: Operator MUST map `provider` field to `provider_type` with `remote::` prefix
- **FR-016**: Operator MUST map `endpoint` field to `config.url`
- **FR-017**: Operator MUST resolve `secretKeyRef` to environment variables and reference via `${env.VAR_NAME}`
- **FR-018**: Operator MUST pass through `config.*` fields as-is to provider config section
- **FR-019**: Operator MUST generate unique, deterministic environment variable names for secrets
- **FR-020**: Operator MUST apply `disabled` list by removing those provider sections from generated config
- **FR-039**: Operator MUST name generated ConfigMap as `<llsd-name>-config-<content-hash>` for immutability
- **FR-040**: Operator MUST set owner reference on generated ConfigMap for garbage collection

#### Resource Registration

- **FR-021**: Simple model strings MUST be registered with first configured inference provider
- **FR-022**: Detailed model objects with `provider` field MUST use specified provider
- **FR-023**: Tools MUST be mapped to `registered_resources.tool_groups`
- **FR-024**: Shields MUST be mapped to `registered_resources.shields`

#### Integration with External Providers (Spec 001)

- **FR-025**: Generated config MUST serve as base for external provider merge (spec 001)
- **FR-026**: External providers MUST be additive to generated providers
- **FR-027**: External provider with same ID as generated provider MUST override with warning

#### Validation

- **FR-028**: Operator MUST validate provider types against supported list
- **FR-029**: Operator MUST validate that referenced secrets exist before generating config
- **FR-030**: Operator MUST validate provider IDs are unique across all sources
- **FR-031**: Operator MUST validate mutual exclusivity of `providers` and `userConfig`
- **FR-034**: CRD schema MUST define provider-specific required fields (e.g., `region` for bedrock, `deploymentName` for azure) with CEL validation rules
- **FR-035**: Operator MUST log warning (non-blocking) when `disabled` list contains provider types not present in distribution

#### Status Reporting

- **FR-032**: LLSD status MUST include conditions for config generation: `ValidationSucceeded`, `SecretsResolved`, `ConfigReady`
- **FR-033**: Config generation errors MUST include field path and actionable resolution
- **FR-036**: `ValidationSucceeded` condition MUST reflect CRD schema and CEL validation status
- **FR-037**: `SecretsResolved` condition MUST reflect whether all secretKeyRef references resolved successfully
- **FR-038**: `ConfigReady` condition MUST reflect final config.yaml generation status

### Key Entities

- **ProviderEntry**: Individual provider configuration with provider type, endpoint, apiKey, and pass-through config
- **ProviderConfigOrList**: Polymorphic type accepting single ProviderEntry or list of entries
- **ProvidersSpec**: Container for all provider types (inference, safety, vectorIo, etc.)
- **ResourcesSpec**: Resource registration for models, tools, and shields
- **StorageConfigSpec**: Storage backend configuration with type and connection string
- **GeneratedConfigMap**: Operator-generated ConfigMap containing final config.yaml

## Success Criteria

### Measurable Outcomes

- **SC-001**: Users can deploy a functional LlamaStack with inference provider using 10 or fewer lines of YAML in the LLSD CR
- **SC-002**: 100% of provider configuration errors include the specific field path and suggested resolution
- **SC-003**: Generated config.yaml is byte-for-byte deterministic given the same CR input
- **SC-004**: Secret values never appear in generated ConfigMap (only environment variable references)
- **SC-005**: Users can seamlessly migrate from simplified schema to ConfigMap escape hatch without downtime
- **SC-006**: External providers (spec 001) merge correctly with generated config without user intervention

## Schema Reference

### Complete CRD Schema

```yaml
spec:
  server:
    distribution:
      name: starter

    # Simplified provider configuration
    providers:
      # Single provider (common case)
      inference:
        provider: vllm                    # Maps to provider_type: remote::vllm
        endpoint: "http://vllm:8000"      # Maps to config.url
        apiKey:
          secretKeyRef:
            name: vllm-creds
            key: token
        config:                           # Pass-through to config section
          max_tokens: 8192

      # Or list form for multiple providers
      # inference:
      #   - id: vllm-primary
      #     provider: vllm
      #     endpoint: "http://vllm:8000"
      #   - id: ollama-fallback
      #     provider: ollama
      #     endpoint: "http://ollama:11434"

      safety:
        provider: llama-guard

      vectorIo:
        provider: pgvector
        host:
          secretKeyRef:
            name: pg-creds
            key: host

    # Explicitly disabled provider types
    disabled:
      - postTraining
      - eval
      - benchmarks

    # Storage configuration
    storage:
      type: postgres                      # sqlite | postgres
      connectionString:
        secretKeyRef:
          name: pg-creds
          key: url

    # Resource registration
    resources:
      models:
        - "llama3.2-8b"                   # Simple: uses first inference provider
        - name: "llama3.2-70b"
          provider: vllm-primary          # Explicit provider assignment
          metadata:
            contextLength: 128000
      tools:
        - websearch
        - rag
      shields:
        - llama-guard

    # Common server settings
    port: 8321
    tls:
      enabled: true
      secretName: llama-tls

    # === ESCAPE HATCH (mutually exclusive with providers) ===
    # userConfig:
    #   configMapName: my-full-config

    # External providers (from spec 001, additive)
    externalProviders:
      inference:
        - providerId: custom-inference
          image: registry.example.com/custom:v1
```

### Supported Provider Types

| CRD Value | Generated provider_type | Typical Endpoint |
|-----------|------------------------|------------------|
| vllm | remote::vllm | http://vllm:8000/v1 |
| ollama | remote::ollama | http://ollama:11434 |
| openai | remote::openai | https://api.openai.com/v1 |
| anthropic | remote::anthropic | https://api.anthropic.com |
| bedrock | remote::bedrock | (AWS region-based) |
| azure | remote::azure | (Azure endpoint) |
| gemini | remote::gemini | (Google endpoint) |
| together | remote::together | https://api.together.xyz/v1 |
| fireworks | remote::fireworks | https://api.fireworks.ai/inference/v1 |
| groq | remote::groq | https://api.groq.com/openai/v1 |
| nvidia | remote::nvidia | https://integrate.api.nvidia.com/v1 |

### Field Mappings

| CRD Field | config.yaml Field | Notes |
|-----------|-------------------|-------|
| `provider` | `provider_type` | Prefixed with `remote::` |
| `endpoint` | `config.url` | Renamed for clarity |
| `apiKey` | `config.api_key` | Resolved via env var |
| `id` | `provider_id` | Auto-generated if single |
| `config.*` | `config.*` | Passed through as-is |

## Dependencies

### Internal Dependencies

- **Spec 001 (Deploy-Time Providers)**: Generated config serves as base for external provider merge
- Integration point: Generated config.yaml placed at same location expected by spec 001 merge

### External Dependencies

- Kubernetes 1.21+ (CEL validation support)
- llama-stack distribution images with embedded config.yaml

## Constraints

- Provider types limited to those supported by llama-stack
- Secret values cannot be directly embedded (must use secretKeyRef); no inline secrets supported
- Configuration changes require pod restart (no hot reload)
- Single provider auto-ID conflicts with list provider explicit IDs

## Assumptions

- Distribution images contain valid embedded config.yaml at known paths
- llama-stack maintains backward compatibility for provider_type naming
- Users prefer Kubernetes-native patterns (secretKeyRef) over inline secrets

## Out of Scope

- Dynamic configuration updates without pod restart
- Automatic provider type detection from endpoint URL
- Multi-cluster configuration synchronization
- Configuration diff/audit logging
- GUI for configuration management

## Clarifications

### Session 2026-02-02

- Q: How to handle provider-specific required fields (e.g., region for bedrock)? → A: Define provider-specific required fields in CRD schema
- Q: Should disabled providers be validated against distribution's available providers? → A: Warn if disabled provider not in distribution (non-blocking)
- Q: Should we support inline secrets with deprecation warning for migration? → A: No inline secrets (secretKeyRef only, clean break)
- Q: Which status condition types for config generation? → A: Multiple conditions: ConfigReady, SecretsResolved, ValidationSucceeded
- Q: Generated ConfigMap naming strategy? → A: Hash suffix `<llsd-name>-config-<hash>` (immutable, new on change)

## Open Questions

- [x] ~~Should we support inline secrets with deprecation warning for migration?~~ → Resolved: No inline secrets (secretKeyRef only)
- [x] ~~How to handle provider-specific required fields (e.g., region for bedrock)?~~ → Resolved: CRD schema will define provider-specific required fields
- [x] ~~Should disabled providers be validated against distribution's available providers?~~ → Resolved: Warn if not in distribution (non-blocking)

## Acceptance

Feature is complete when:

- [ ] All functional requirements implemented and tested
- [ ] All user story acceptance scenarios pass
- [ ] CEL validation rules enforce mutual exclusivity and ID requirements
- [ ] Generated config.yaml matches expected format for all supported provider types
- [ ] Secret handling via environment variables works correctly
- [ ] Integration with spec 001 external providers verified
- [ ] Documentation covers all schema options with examples
- [ ] Migration guide from full ConfigMap to simplified schema available
