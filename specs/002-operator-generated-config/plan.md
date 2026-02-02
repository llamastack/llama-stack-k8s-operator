# Implementation Plan: Operator-Generated Server Configuration

**Branch**: `002-operator-generated-config` | **Date**: 2026-02-02 | **Spec**: [spec.md](./spec.md)
**Input**: Feature specification from `/specs/002-operator-generated-config/spec.md`

## Summary

Enable the llama-stack Kubernetes operator to generate `config.yaml` from a simplified, provider-centric schema in the LlamaStackDistribution CR. Users specify providers (inference, safety, vectorIo, etc.) with minimal YAML, and the operator expands these to full config.yaml structure, resolving secrets via environment variables and merging with distribution defaults.

**Technical Approach**: Extend the existing CRD types to add `providers`, `storage`, `resources`, and `disabled` fields. Implement a config generator that reads the distribution's embedded config.yaml, merges user overrides, resolves secretKeyRef references, and creates an immutable, hash-named ConfigMap. Status conditions track generation progress.

## Technical Context

**Language/Version**: Go 1.25 (from go.mod/Dockerfile)
**Primary Dependencies**: controller-runtime, kubebuilder, kustomize, client-go
**Storage**: Kubernetes ConfigMaps (generated), Secrets (referenced via secretKeyRef)
**Testing**: Ginkgo/Gomega (E2E in tests/e2e/), standard Go testing (unit tests)
**Target Platform**: Kubernetes 1.21+ (CEL validation support required)
**Project Type**: Kubernetes Operator (kubebuilder-based)
**Performance Goals**: Config generation completes within reconciliation timeout (30s default)
**Constraints**: Namespace-scoped resources only (per constitution), no cluster-admin privileges
**Scale/Scope**: Single LLSD resource generates one ConfigMap; typical deployment 1-10 providers

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Constitution Rule | Status | Notes |
|-------------------|--------|-------|
| §1.1 Namespace-Scoped Resources | ✅ PASS | Generated ConfigMaps are namespace-scoped |
| §1.2 Reconciliation Idempotent | ✅ PASS | Config generation deterministic via hash |
| §1.3 Owner References | ✅ PASS | FR-040 requires owner reference for GC |
| §2.1 Kubebuilder Validation | ✅ PASS | FR-012, FR-034 require CEL validation |
| §2.2 Optional Fields | ✅ PASS | Schema uses pointers for optional structs |
| §2.4 Status Subresource | ✅ PASS | FR-032-038 define status conditions |
| §3.2 Conditions | ✅ PASS | Using metav1.Condition with defined types |
| §4.1 Error Wrapping | ✅ PASS | FR-033 requires field path in errors |
| §4.3 User-Facing Errors | ✅ PASS | Actionable messages per spec |
| §6.4 Builder Pattern | ✅ PASS | Test builders exist for LLSD |

**Gate Status**: ✅ PASSED - No violations, proceed to Phase 0

## Project Structure

### Documentation (this feature)

```text
specs/002-operator-generated-config/
├── plan.md              # This file
├── research.md          # Phase 0 output
├── data-model.md        # Phase 1 output
├── quickstart.md        # Phase 1 output
├── contracts/           # Phase 1 output (CEL validation rules)
└── tasks.md             # Phase 2 output (from /speckit.tasks)
```

### Source Code (repository root)

```text
api/v1alpha1/
├── llamastackdistribution_types.go    # ADD: ProvidersSpec, StorageConfigSpec, ResourcesSpec
├── provider_types.go                   # NEW: Provider configuration types
├── zz_generated.deepcopy.go           # Regenerated

controllers/
├── llamastackdistribution_controller.go  # MODIFY: Add reconcileGeneratedConfig()
├── conditions.go                          # NEW: Condition type and reason constants
├── status.go                              # MODIFY: Add condition helpers
└── resource_helper.go                     # MODIFY: Mount generated ConfigMap

pkg/
├── configgen/                             # NEW: Config generation package
│   ├── generator.go                       # Main generation logic
│   ├── provider_mapper.go                 # Provider type mapping
│   ├── secret_resolver.go                 # SecretKeyRef resolution
│   └── generator_test.go                  # Unit tests

tests/e2e/
├── provider_config_test.go                # NEW: E2E tests for provider config
└── test_utils.go                          # MODIFY: Add config helpers

config/
├── crd/bases/                             # Regenerated CRD manifests
└── samples/
    ├── example-with-providers.yaml        # NEW: Sample with inline providers
    └── example-minimal-inference.yaml     # NEW: Minimal inference example
```

**Structure Decision**: Follows existing operator structure. New `pkg/configgen/` package encapsulates config generation logic for testability. Controller calls into package for generation.

## Complexity Tracking

> No constitution violations to justify.

| Violation | Why Needed | Simpler Alternative Rejected Because |
|-----------|------------|-------------------------------------|
| N/A | N/A | N/A |

---

## Phase 0: Research

### Research Areas

1. **Provider Type Mapping**: Validate the full list of llama-stack provider types and their config.yaml structure
2. **Distribution Config Extraction**: How to read embedded config.yaml from distribution images
3. **CEL Validation Patterns**: Best practices for polymorphic field validation (single vs list)
4. **Secret Resolution Timing**: When to resolve secretKeyRef (reconcile vs pod start)
5. **Hash Algorithm Selection**: Which hash for ConfigMap naming (deterministic, collision-resistant)

See [research.md](./research.md) for detailed findings.

---

## Phase 1: Design

### 1.1 Data Model

See [data-model.md](./data-model.md) for complete type definitions.

**Key Types**:
- `ProvidersSpec` - Container for all provider types (inference, safety, vectorIo, etc.)
- `ProviderConfig` - Polymorphic: single object OR list of provider entries
- `ProviderEntry` - Individual provider with id, provider type, endpoint, apiKey, config
- `SecretKeyRefSource` - Kubernetes-native secret reference
- `ResourcesSpec` - Models, tools, shields registration
- `StorageConfigSpec` - Database type and connection string

### 1.2 API Contracts

See [contracts/](./contracts/) directory for:
- `cel-validation-rules.yaml` - CEL expressions for CRD validation
- `config-schema.yaml` - Generated config.yaml structure
- `status-conditions.yaml` - Condition type definitions

### 1.3 Integration Points

| Component | Integration Method | Notes |
|-----------|-------------------|-------|
| Spec 001 (External Providers) | Additive merge | External providers merge into generated config |
| Distribution Image | Config extraction | Read /etc/llama-stack/config.yaml or similar |
| Kubernetes Secrets | secretKeyRef resolution | Inject as env vars, reference in config |
| Status Reporting | Condition updates | ValidationSucceeded, SecretsResolved, ConfigReady |

---

## Implementation Phases Summary

### Phase 1: CRD Schema & Types (Foundation)
- Add new types to api/v1alpha1/
- Implement CEL validation rules
- Regenerate CRD manifests

### Phase 2: Config Generation Engine
- Implement pkg/configgen/ package
- Provider type mapping
- Secret resolution
- Config merging

### Phase 3: Controller Integration
- Add reconcileGeneratedConfig()
- Status condition updates
- ConfigMap lifecycle management

### Phase 4: Testing & Documentation
- E2E tests for all user stories
- Sample manifests
- Migration guide

---

## Next Steps

1. Run `/speckit.tasks` to generate detailed task breakdown
2. Implement in phases, starting with CRD schema
3. Each phase should have passing tests before proceeding
