<!--
================================================================================
SYNC IMPACT REPORT
================================================================================

Version Change: (initial) → 1.0.0
Change Type: MINOR (initial constitution creation from codebase analysis)
Date: 2025-11-12

Modified Principles:
- N/A (initial creation)

Added Sections:
- Core Principles (10 principles derived from codebase)
- Code Quality Standards
- Project Organization
- Kubernetes Patterns
- Testing Strategies
- Build & Release Standards
- Documentation Standards
- Governance

Removed Sections:
- N/A (initial creation)

Template Alignment Status:
- ✅ plan-template.md: Constitution Check section present (line 30-34)
- ✅ spec-template.md: No constitution-specific sections required
- ✅ tasks-template.md: Task organization aligns with test isolation principle
- ✅ README.md: Existing structure matches documentation standards

Follow-up TODOs:
- None - all principles derived from existing codebase patterns

Constitutional Principles Summary:
1. Spec-First Reconciliation (NON-NEGOTIABLE)
2. Immutable Field Respect
3. Owner Reference Discipline
4. Error Handling Standard (NON-NEGOTIABLE)
5. Test Isolation & Anti-Fragility
6. Structured Logging & Observability
7. Feature Flag Pattern
8. Kustomize-Based Manifests
9. Security by Default
10. ConfigMap Content-Based Rolling Updates

================================================================================
-->

# llama-stack-k8s-operator Constitution

## Core Principles

### I. Spec-First Reconciliation (NON-NEGOTIABLE)

**Reconciliation MUST always be driven by the declared spec, with status reflecting observed reality.**

- Reconcile loops process spec → desired state → apply changes → update status
- Status updates happen at the end of reconciliation, always (even on errors)
- Never modify spec during reconciliation; spec changes come from users only
- Phase-based state management with clear transitions (Pending → Initializing → Ready → Failed)
- Separate conditions track individual component concerns (Deployment, Health, Storage, Service)

### II. Immutable Field Respect

**Never modify immutable Kubernetes fields; preserve them across updates.**

- Deployment selectors are immutable and MUST be preserved during updates
- Use server-side apply with ForceOwnership for safe updates
- Validate immutable fields before attempting updates
- Fail fast with clear errors when immutable constraints are violated

### III. Owner Reference Discipline

**Set owner references on namespace-scoped resources only for proper garbage collection.**

- Owner references enable automatic cleanup when CRs are deleted
- Never set owner references on cluster-scoped resources
- Each created resource MUST have consistent ownership metadata
- Use controller-runtime's owner reference helpers for consistency

### IV. Error Handling Standard (NON-NEGOTIABLE)

**All wrapped errors MUST start with "failed to" prefix for consistency and tooling.**

- Use `fmt.Errorf("failed to <action>: %w", err)` for all error wrapping
- Enforced by pre-commit hooks and CI checks
- Log errors with context before returning from functions
- Validate early, return errors immediately (fail fast pattern)
- Error chains preserve full context for debugging

### V. Test Isolation & Anti-Fragility

**Tests MUST be isolated, intention-revealing, and resistant to implementation changes.**

- **DAMP + DRY together**: Descriptive test scenarios (DAMP) with DRY setup utilities
- **AAA pattern**: Arrange, Act, Assert with clear separation
- **Unique namespaces**: Each test gets isolated namespace, no shared state
- **Builder pattern**: Fluent test instance builders (e.g., `NewDistributionBuilder()`)
- **Production constants in integration tests**: Verify defaults are applied correctly
- **Test-owned constants in E2E**: Focus on behavior, not implementation details
- **require.Eventually**: For async Kubernetes operations with proper timeouts

### VI. Structured Logging & Observability

**Use context-based structured logging with consistent key-value pairs.**

- Controller-runtime logger with contextual metadata
- Always include namespace and name for CR-related logs
- Use V(1) for debug/trace verbosity levels
- Dual output for diffs: fmt.Printf for readability + logger for structured data
- Log state transitions and key decisions for operational debugging

### VII. Feature Flag Pattern

**Use ConfigMap-based feature flags with YAML structure for gradual rollouts.**

- Feature flags defined in dedicated ConfigMap
- YAML-based structure with enable/disable semantics
- Features can be toggled without code changes or redeployments
- Optional features (e.g., NetworkPolicy) respect flag state
- Default values documented in code for clarity

### VIII. Kustomize-Based Manifests

**Use Kustomize with plugin transformations over template rendering.**

- Embedded YAML manifests with kustomization.yaml
- Custom plugins for namespace, name-prefix, field mutations
- Runtime transformations based on CR spec
- Conditional resource inclusion/exclusion (e.g., PVC only if storage defined)
- Delete excluded resources from previous reconciliations for cleanup

### IX. Security by Default

**Apply restrictive security contexts and validate all security-related inputs.**

- **Pod-level FSGroup**: Set to 1001 for volume write access
- **Init containers**: AllowPrivilegeEscalation=false, RunAsNonRoot=true, drop ALL capabilities
- **PEM validation**: Validate certificate format before use
- **ServiceAccount per instance**: Following instance-name-sa pattern
- **SSL/TLS injection**: Proper CA bundle handling with auto-detection

### X. ConfigMap Content-Based Rolling Updates

**Hash ConfigMap contents in pod annotations to trigger rolling updates on config changes.**

- Watch referenced ConfigMaps with custom predicates
- Calculate hash of ConfigMap data and inject into pod annotations
- Hash change triggers Deployment rolling update automatically
- Field indexing for efficient ConfigMap lookups with fallback
- Only reconcile on spec changes (not status/metadata updates)

## Code Quality Standards

### Linter Configuration

- **Enable-all philosophy**: Start with all linters enabled, disable only with clear justification
- **Cyclomatic complexity**: Max 30 for functions
- **Function length**: Max 100 lines/statements (ignoring comments)
- **Line length**: 180 characters maximum
- **Magic numbers**: Contextual exceptions (0, 1, 2, 5, 10, 100)

### Go Coding Standards

- **Package names**: Lowercase, descriptive (deploy, compare, cluster, featureflags)
- **Exported constants**: PascalCase with descriptive prefixes
- **Private functions**: camelCase with clear verb prefixes
- **Receiver names**: Single letter (r for reconciler) or short abbreviations
- **Import organization (gci)**: Standard lib → Third-party → Blank → Dot
- **Error early returns**: Fail fast pattern throughout
- **No naked returns**: Named returns only when clarity demands
- **Context propagation**: Context passed through call chains

### Function Decomposition

- **Single Responsibility**: Each function has one clear purpose
- **DRY for implementation**: Extract common logic into helpers
- **Builder pattern**: For complex object construction (especially tests)
- **Explicit over implicit**: Clear intent over brevity

## Project Organization

### Package Structure

```
/api/v1alpha1          - CRD types and generated code (pure data types)
/controllers           - Main reconciler logic (orchestration, status management)
/pkg/deploy           - Resource rendering and application
/pkg/compare          - Resource comparison utilities
/pkg/cluster          - Cluster configuration management
/pkg/featureflags     - Feature flag definitions
/tests/e2e            - End-to-end tests
```

### Separation of Concerns

- **API layer**: Pure data types in api/v1alpha1, minimal methods
- **Controller layer**: Orchestration logic, no business domain logic
- **Package layer**: Reusable utilities and domain logic
- **Manifest layer**: Declarative Kubernetes resources only
- **Test layer**: Isolated with dedicated builders and utilities

## Kubernetes Patterns

### CRD Design

- **Kubebuilder markers**: Extensive validation with +kubebuilder:validation:
- **Default values**: Set via markers (+kubebuilder:default:)
- **Short names**: Easy kubectl access (llsd for LlamaStackDistribution)
- **Custom columns**: Phase, versions, replicas in kubectl output
- **XValidation**: CEL rules for cross-field validation
- **Namespace-scoped**: All CRDs are namespaced, not cluster-scoped

### Resource Management

- **Watches pattern**:
  - Owns: Deployment, Service, NetworkPolicy, PVC
  - Watches: ConfigMap with custom predicates
- **Server-side apply**: For safe, non-destructive updates
- **Conditional creation**: Skip resources based on spec (e.g., Service only if ports defined)
- **Resource cleanup**: Delete resources excluded in current spec

### Operational Patterns

- **Health & Readiness**: Startup probes (15s delay, 30s timeout, 3 failures)
- **Version detection**: Dynamic llama-stack CLI version detection
- **Provider info**: Query /v1/providers for runtime status
- **Graceful degradation**: Continue on non-critical health check failures
- **Service URL tracking**: Internal service URL in status for observability

## Testing Strategies

### Test Types & Coverage

- **Unit tests**: Package-level (*_test.go alongside code)
- **Integration tests**: Controller tests using envtest framework
- **E2E tests**: Separate tests/e2e directory with kind/real clusters
- **Suite pattern**: BeforeSuite/AfterSuite for setup/teardown
- **Coverage tracking**: -coverprofile cover.out, targeted TEST_PKGS

### Test Utilities

- **testify/require**: For test control flow (stop on failure)
- **testify/assert**: For non-blocking assertions
- **require.Eventually**: For async operations with proper timeouts
- **Focused testing**: -run flag for TDD workflows
- **Unique resources**: Generated names/namespaces per test

## Build & Release Standards

### Makefile Organization

- **Phony targets**: All targets declared as .PHONY
- **Help system**: Auto-generated help from inline comments
- **Tool versioning**: Pinned versions (kustomize, controller-gen, etc.)
- **Local overrides**: -include local.mk for custom development values
- **Release coordination**: VERSION and LLAMASTACK_VERSION updated together

### Image Management

- **Default registry**: quay.io/llamastack
- **Multi-platform**: linux/arm64, amd64, s390x, ppc64le support
- **Container tool agnostic**: CONTAINER_TOOL variable (podman/docker)
- **Version tagging**: Semantic versioning with latest fallback

### Code Quality Gates (NON-NEGOTIABLE)

- **Pre-commit hooks**: Mandatory formatting, linting, error message validation
- **CI enforcement**: Pre-commit runs in CI, blocks merges on failure
- **Linting**: golangci-lint with 100+ enabled linters
- **Format enforcement**: gci for imports, yamlfmt for YAML
- **Error message linting**: Custom check for "failed to" prefix in wrapped errors

## Documentation Standards

### Code Documentation

- **File headers**: Apache 2.0 license in all source files
- **Package comments**: Describe package purpose and responsibilities
- **Exported items**: All exported types/functions MUST be documented
- **Complex logic**: Inline comments explaining "why" not "what"
- **Examples**: Config samples in config/samples/ for user guidance

### API Documentation

- **Generated docs**: crd-ref-docs for API reference
- **Field descriptions**: Inline godoc for all CRD fields
- **Validation docs**: Markers describe constraints and valid values
- **Status subresource**: Separate documentation for status fields

### README Requirements

- Quick Start section with installation steps
- Developer Guide with prerequisites and build instructions
- API Overview reference pointing to generated docs
- Testing instructions (unit, integration, E2E)
- Clear examples for different use cases

## Governance

### Constitutional Authority

This constitution supersedes all other practices and guidelines. All specifications, implementations, code reviews, and pull requests MUST verify compliance with these principles.

### Amendment Process

1. Proposed amendments MUST be documented with clear rationale
2. Impact analysis on existing code and practices required
3. Approval from project maintainers needed
4. Migration plan for affected code (if any)
5. Update constitution version and last amended date

### Compliance Enforcement

- All PRs MUST verify compliance during code review
- Pre-commit hooks enforce mechanical checks (error messages, formatting)
- Complexity or deviations MUST be justified in PR descriptions
- Regular audits of codebase against constitutional principles

### Specification-Driven Development Integration

- Specifications MUST reference relevant constitutional principles
- Implementation plans MUST show how they uphold these standards
- Code reviews verify spec compliance AND constitutional compliance
- Evolutions MUST maintain constitutional alignment

**Version**: 1.0.0 | **Ratified**: 2025-11-12 | **Last Amended**: 2025-11-12
