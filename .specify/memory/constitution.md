<!--
================================================================================
SYNC IMPACT REPORT
================================================================================

Version Change: 1.0.0 → 2.0.0
Change Type: MAJOR (consolidation of duplicate constitution files)
Date: 2026-01-30

Modified Principles:
- All principles updated to comprehensive format from specs/constitution.md

Added Sections (from specs/constitution.md):
- Quick Reference: Top 10 Critical Rules
- API Design (CRD) with detailed validation patterns
- Status Reporting with conditions helpers
- Error Handling with detailed patterns
- Logging with structured patterns
- Testing with comprehensive strategies
- Code Organization file structure
- Naming Conventions
- Dependencies and Imports
- Documentation standards
- Feature Flags
- API Versioning
- Git Commit Guidelines
- Enforcement (automated tooling and code review)

Preserved from .specify/memory/constitution.md:
- SYNC IMPACT REPORT header format
- Kustomize-Based Manifests (Principle VIII details)
- ConfigMap Content-Based Rolling Updates (Principle X details)
- Security by Default specifics (FSGroup, init containers, ServiceAccount)
- Code Quality Standards (linter configuration)
- Build & Release Standards
- Governance section

Removed Files:
- specs/constitution.md (merged into this file)

Template Alignment Status:
- ✅ plan-template.md: Constitution Check section present
- ✅ spec-template.md: No constitution-specific sections required
- ✅ tasks-template.md: Task organization aligns with test isolation principle
- ✅ README.md: Existing structure matches documentation standards

Follow-up TODOs:
- None - consolidation complete

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

# Project Constitution: llama-stack-k8s-operator

**Purpose**: Define project-wide principles, patterns, and standards that guide all specifications and implementations.

**Last Updated**: 2026-01-30
**Version**: 2.0.0

---

## Constitution Status

This constitution documents **existing patterns** found in the codebase as of 2025-11-12. Rules are categorized as:

- **Current Practice**: Patterns consistently used in existing code (marked with ✅)
- **Aspirational**: Patterns we want to adopt going forward (marked with 🎯)
- **MUST**: Non-negotiable requirements for all new code
- **SHOULD**: Strong recommendations, exceptions allowed with justification

**Adoption Approach**: This is a living document. Apply these patterns when writing new code or significantly refactoring existing code. No need for wholesale rewrites.

---

## Quick Reference: Top 10 Critical Rules

Quick lookup for the most important rules during day-to-day development:

1. **✅ Reconciliation is Idempotent** (§1.2)
   Reconcile functions must produce same result when called multiple times with same input

2. **✅ Separate Reconciliation from Status** (§1.2)
   Pattern: `reconcileResources(ctx, instance)` separate from `updateStatus(ctx, instance, err)`

3. **✅ Wrap Errors with Context** (§4.1)
   Always use `fmt.Errorf("failed to X: %w", err)` to preserve error chain

4. **✅ Use Kubebuilder Validation** (§2.1)
   Field validation at CRD level prevents invalid resources: `// +kubebuilder:validation:Required`

5. **✅ Status Has Phase + Conditions** (§3)
   Status MUST include phase enum and Kubernetes standard conditions (`metav1.Condition`)

6. **✅ Logger in Context** (§5.1)
   Store logger with request values in context, retrieve in sub-functions

7. **✅ Table-Driven Tests** (§6.1)
   Use `tests := []struct{name, input, expected}` pattern for multiple test cases

8. **✅ Builder Pattern for Tests** (§6.4)
   Test instances created with `NewDistributionBuilder().WithX().Build()`

9. **✅ Owner References for Cleanup** (§1.3)
   All owned resources must have owner references for automatic garbage collection

10. **✅ Namespace-Scoped Resources** (§1.1)
    Operator must NOT require cluster-admin or use cluster-scoped resources

**Legend**: ✅ = Current practice | 🎯 = Aspirational | See section § for details

---

## 1. Kubernetes Operator Principles

### 1.1 Resource Scope
- **MUST**: All resources MUST be namespace-scoped (no cluster-wide permissions)
- **MUST**: Operator MUST NOT require cluster-admin privileges
- **RATIONALE**: Enables multi-tenant deployments and follows least-privilege principle

### 1.2 Reconciliation
- **MUST**: Reconciliation MUST be idempotent
- **MUST**: Reconciler MUST handle partial failures gracefully
- **MUST**: Status updates MUST occur even when reconciliation fails
- **PATTERN**: Separate reconciliation logic (`reconcileResources`) from status updates (`updateStatus`)
- **RATIONALE**: Ensures observable state even during failures

**Spec-First Reconciliation (NON-NEGOTIABLE)**:
- Reconcile loops process spec → desired state → apply changes → update status
- Status updates happen at the end of reconciliation, always (even on errors)
- Never modify spec during reconciliation; spec changes come from users only
- Phase-based state management with clear transitions (Pending → Initializing → Ready → Failed)
- Separate conditions track individual component concerns (Deployment, Health, Storage, Service)

### 1.3 Resource Ownership
- **MUST**: Owned resources MUST use owner references for garbage collection
- **MUST**: Resources owned by the operator MUST have consistent labeling (see Naming Conventions)
- **RATIONALE**: Enables automatic cleanup on CR deletion
- **MUST**: Never set owner references on cluster-scoped resources
- **MUST**: Use controller-runtime's owner reference helpers for consistency

### 1.4 Immutable Field Respect
- **MUST**: Never modify immutable Kubernetes fields; preserve them across updates
- **MUST**: Deployment selectors are immutable and MUST be preserved during updates
- **MUST**: Use server-side apply with ForceOwnership for safe updates
- **MUST**: Validate immutable fields before attempting updates
- **MUST**: Fail fast with clear errors when immutable constraints are violated

### 1.5 Kustomize-Based Manifests
- **PATTERN**: Use Kustomize with plugin transformations over template rendering
- Embedded YAML manifests with kustomization.yaml
- Custom plugins for namespace, name-prefix, field mutations
- Runtime transformations based on CR spec
- Conditional resource inclusion/exclusion (e.g., PVC only if storage defined)
- Delete excluded resources from previous reconciliations for cleanup

### 1.6 ConfigMap Content-Based Rolling Updates
- **PATTERN**: Hash ConfigMap contents in pod annotations to trigger rolling updates on config changes
- Watch referenced ConfigMaps with custom predicates
- Calculate hash of ConfigMap data and inject into pod annotations
- Hash change triggers Deployment rolling update automatically
- Field indexing for efficient ConfigMap lookups with fallback
- Only reconcile on spec changes (not status/metadata updates)

---

## 2. API Design (CRD)

### 2.1 Validation

- **MUST**: Use kubebuilder validation tags for field-level validation
  ```go
  // +kubebuilder:validation:Required
  // +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
  // +kubebuilder:validation:Enum=Value1;Value2;Value3
  ```

- **MUST**: Use XValidation for complex cross-field validation
  ```go
  // +kubebuilder:validation:XValidation:rule="!(has(self.name) && has(self.image))",message="Only one of name or image can be specified"
  ```

- **SHOULD**: Provide clear, actionable validation error messages
- **RATIONALE**: Fail fast at admission time rather than reconciliation time

### 2.2 Optional Fields

- **MUST**: Mark optional fields with `+optional` tag
- **MUST**: Use pointers for optional structs to distinguish between "not set" and "set to zero value"
  ```go
  Storage *StorageSpec `json:"storage,omitempty"` // +optional
  ```

- **RATIONALE**: Clear distinction between unset and zero value

### 2.3 Defaults

- **MUST**: Define constants for default values
  ```go
  const (
      DefaultContainerName = "llama-stack"
      DefaultServerPort int32 = 8321
  )
  ```

- **SHOULD**: Use kubebuilder default tags where appropriate
  ```go
  // +kubebuilder:default:=1
  Replicas int32 `json:"replicas,omitempty"`
  ```

- **RATIONALE**: Single source of truth for defaults, testable

### 2.4 Status Subresource

- **MUST**: Use status subresource for all CRDs
  ```go
  //+kubebuilder:subresource:status
  ```

- **MUST**: Status MUST include Phase field (enum of valid phases)
- **MUST**: Status MUST include Conditions (Kubernetes standard metav1.Condition)
- **SHOULD**: Include version information in status for observability

### 2.5 Printer Columns

- **MUST**: Define useful printer columns for `kubectl get` output
  ```go
  //+kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
  //+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"
  ```

- **RATIONALE**: Better user experience with kubectl

### 2.6 Short Names

- **MUST**: Define short names for CRDs
  ```go
  //+kubebuilder:resource:shortName=llsd
  ```

- **RATIONALE**: Improves CLI usability

---

## 3. Status Reporting

### 3.1 Status Phases

- **MUST**: Define phase as enum type
  ```go
  // +kubebuilder:validation:Enum=Pending;Initializing;Ready;Failed;Terminating
  type DistributionPhase string
  ```

- **MUST**: Define constants for each phase
  ```go
  const (
      LlamaStackDistributionPhasePending DistributionPhase = "Pending"
      LlamaStackDistributionPhaseReady   DistributionPhase = "Ready"
      // ...
  )
  ```

### 3.2 Conditions

- **MUST**: Use Kubernetes standard `metav1.Condition` type
- **MUST**: Define constants for condition types, reasons, and messages
  ```go
  const (
      // Condition types
      ConditionTypeDeploymentReady = "DeploymentReady"

      // Condition reasons
      ReasonDeploymentReady = "DeploymentReady"
      ReasonDeploymentFailed = "DeploymentFailed"

      // Condition messages
      MessageDeploymentReady = "Deployment is ready"
      MessageDeploymentFailed = "Deployment failed"
  )
  ```

- **MUST**: Provide helper functions for setting conditions
  ```go
  func SetDeploymentReadyCondition(status *Status, ready bool, message string)
  ```

- **MUST**: Include timestamps using `metav1.NewTime(metav1.Now().UTC())`
- **SHOULD**: Provide helper functions for checking condition state
  ```go
  func IsConditionTrue(status *Status, conditionType string) bool
  ```

### 3.3 Condition Update Pattern

- **MUST**: Use generic `SetCondition` function that updates existing or appends new
  ```go
  func SetCondition(status *Status, condition metav1.Condition) {
      // Find and update existing, or append new
  }
  ```

- **RATIONALE**: Avoids duplicate conditions, maintains condition list integrity

---

## 4. Error Handling

### 4.1 Error Wrapping

- **MUST**: Wrap errors with context using `%w` verb
  ```go
  return fmt.Errorf("failed to reconcile storage: %w", err)
  ```

- **MUST**: Include resource identifiers in error messages
  ```go
  return fmt.Errorf("failed to fetch ConfigMap %s/%s: %w", namespace, name, err)
  ```

- **MUST**: All wrapped errors MUST start with "failed to" prefix for consistency and tooling
- **RATIONALE**: Preserves error chain for debugging, provides context, enforced by pre-commit hooks

### 4.2 Error Messages

- **MUST**: Error messages MUST be descriptive and include:
  - Operation that failed
  - Resource identifier (name, namespace)
  - Context about why it matters

- **SHOULD**: Start error messages with lowercase (Go convention)
  ```go
  // Good
  return fmt.Errorf("failed to create deployment: %w", err)

  // Bad
  return fmt.Errorf("Failed to create deployment: %w", err)
  ```

### 4.3 User-Facing Errors

- **MUST**: User-facing error messages (in status conditions) MUST be actionable
- **MUST**: Include what went wrong and how to fix it
- **EXAMPLE**:
  ```
  "Failed to find referenced ConfigMap user-config/default.
   Ensure the ConfigMap exists in the specified namespace."
  ```

### 4.4 Not Found Errors

- **MUST**: Check for `IsNotFound` errors when fetching optional resources
  ```go
  if err != nil {
      if k8serrors.IsNotFound(err) {
          // Handle not found case
          return fmt.Errorf("failed to find referenced ConfigMap %s/%s", ns, name)
      }
      return fmt.Errorf("failed to fetch ConfigMap %s/%s: %w", ns, name, err)
  }
  ```

---

## 5. Logging

### 5.1 Logger Pattern

- **MUST**: Store logger in context with request-specific values
  ```go
  logger := log.FromContext(ctx).WithValues("namespace", req.Namespace, "name", req.Name)
  ctx = logr.NewContext(ctx, logger)
  ```

- **MUST**: Retrieve logger from context in sub-functions
  ```go
  logger := log.FromContext(ctx)
  ```

- **RATIONALE**: Consistent logging across reconciliation, no need to pass logger

### 5.2 Log Levels

- **Info**: Normal operations, state transitions
- **Error**: Errors that need attention (with error object)
  ```go
  logger.Error(err, "failed to reconcile deployment")
  ```

- **V(1)**: Debug-level information
- **MUST NOT**: Use `fmt.Printf` or `log.Printf` - always use structured logging

### 5.3 Log Messages

- **MUST**: Use structured logging with key-value pairs
  ```go
  logger.Info("reconciling deployment",
      "deployment", deploymentName,
      "replicas", replicas)
  ```

- **PATTERN**: Dual output for diffs: fmt.Printf for readability + logger for structured data

---

## 6. Testing

### 6.1 Test Organization

- **MUST**: Use table-driven tests for multiple test cases
  ```go
  tests := []struct {
      name     string
      input    *Input
      expected *Expected
  }{
      {name: "test case 1", ...},
      {name: "test case 2", ...},
  }

  for _, tt := range tests {
      t.Run(tt.name, func(t *testing.T) {
          // test logic
      })
  }
  ```

### 6.2 Test Helpers

- **MUST**: Use `t.Helper()` in helper functions
  ```go
  func AssertDeploymentExists(t *testing.T, ...) {
      t.Helper()
      // assertion logic
  }
  ```

- **MUST**: Use `require` package for assertions (not `assert`)
- **RATIONALE**: `require` stops test on first failure, `assert` continues

### 6.3 Test Cleanup

- **MUST**: Use `t.Cleanup()` for resource cleanup
  ```go
  t.Cleanup(func() {
      if err := k8sClient.Delete(ctx, resource); err != nil {
          t.Logf("cleanup failed: %v", err)
      }
  })
  ```

### 6.4 Builder Pattern

- **MUST**: Use builder pattern for creating test instances
  ```go
  instance := NewDistributionBuilder().
      WithName("test").
      WithNamespace(namespace).
      WithStorage(DefaultTestStorage()).
      Build()
  ```

- **MUST**: Return deep copy from `Build()` method
  ```go
  func (b *DistributionBuilder) Build() *LlamaStackDistribution {
      return b.instance.DeepCopy()
  }
  ```

- **RATIONALE**: Immutability, prevents test contamination

### 6.5 Async Assertions

- **MUST**: Use `require.Eventually()` for async checks
  ```go
  require.Eventually(t, func() bool {
      err := k8sClient.Get(ctx, key, resource)
      return err == nil
  }, timeout, interval, "resource should exist")
  ```

- **MUST**: Define test timeout and interval as constants
  ```go
  const (
      testTimeout  = 5 * time.Second
      testInterval = 100 * time.Millisecond
  )
  ```

### 6.6 Test Naming

- **MUST**: Use descriptive test names that explain what is being tested
  ```go
  "No storage configuration - should use emptyDir"
  "Storage with custom values"
  ```

- **RATIONALE**: Test name documents expected behavior

### 6.7 Test Isolation & Anti-Fragility

- **MUST**: Tests MUST be isolated, intention-revealing, and resistant to implementation changes
- **DAMP + DRY together**: Descriptive test scenarios (DAMP) with DRY setup utilities
- **AAA pattern**: Arrange, Act, Assert with clear separation
- **Unique namespaces**: Each test gets isolated namespace, no shared state
- **Production constants in integration tests**: Verify defaults are applied correctly
- **Test-owned constants in E2E**: Focus on behavior, not implementation details

---

## 7. Code Organization

### 7.1 File Structure

- **MUST**: Separate concerns into different files:
  - `*_types.go` - API types and CRD definitions
  - `*_controller.go` - Main reconciliation logic
  - `status.go` - Status condition helpers
  - `resource_helper.go` - Resource construction helpers
  - `*_test.go` - Tests
  - `testing_support_test.go` - Test helpers and builders

### 7.2 Function Organization

- **SHOULD**: Keep functions focused on single responsibility
- **SHOULD**: Extract complex logic into helper functions
- **SHOULD**: Place helper functions close to where they're used

### 7.3 Helper Function Patterns

- **PATTERN**: Provide both receiver methods and standalone helpers
  - Receiver methods when reconciler context is needed
  - Standalone helpers for use in other packages or tests

  ```go
  // Receiver method (has access to r.Client)
  func (r *Reconciler) hasUserConfigMap(instance *CR) bool

  // Standalone helper (used in watches, predicates)
  func hasValidUserConfig(instance *CR) bool
  ```

### 7.4 Constants

- **MUST**: Define constants for:
  - Default values
  - Resource names
  - Condition types, reasons, messages
  - Well-known labels and annotations

- **MUST**: Group related constants together
  ```go
  const (
      // Condition types
      ConditionTypeX = "X"
      ConditionTypeY = "Y"
  )
  ```

### 7.5 Package Structure

```
/api/v1alpha1          - CRD types and generated code (pure data types)
/controllers           - Main reconciler logic (orchestration, status management)
/pkg/deploy           - Resource rendering and application
/pkg/compare          - Resource comparison utilities
/pkg/cluster          - Cluster configuration management
/pkg/featureflags     - Feature flag definitions
/tests/e2e            - End-to-end tests
```

### 7.6 Separation of Concerns

- **API layer**: Pure data types in api/v1alpha1, minimal methods
- **Controller layer**: Orchestration logic, no business domain logic
- **Package layer**: Reusable utilities and domain logic
- **Manifest layer**: Declarative Kubernetes resources only
- **Test layer**: Isolated with dedicated builders and utilities

---

## 8. Naming Conventions

### 8.1 Resource Naming

- **PATTERN**: Owned resources use CR name as prefix
  ```
  {cr-name}-deployment
  {cr-name}-service
  {cr-name}-pvc
  ```

### 8.2 Labels

- **MUST**: Apply consistent labels to all owned resources
  ```go
  DefaultLabelKey   = "app"
  DefaultLabelValue = "llama-stack"
  ```

- **SHOULD**: Include additional labels for:
  - CR name
  - Component type
  - Managed-by operator

### 8.3 Variable Naming

- **MUST**: Use descriptive variable names
  ```go
  // Good
  configMapNamespace := r.getUserConfigMapNamespace(instance)

  // Bad
  ns := r.getUserConfigMapNamespace(instance)
  ```

- **EXCEPTION**: Loop variables can be short (`i`, `j`, `k`)

### 8.4 Go Coding Standards

- **Package names**: Lowercase, descriptive (deploy, compare, cluster, featureflags)
- **Exported constants**: PascalCase with descriptive prefixes
- **Private functions**: camelCase with clear verb prefixes
- **Receiver names**: Single letter (r for reconciler) or short abbreviations
- **Error early returns**: Fail fast pattern throughout
- **No naked returns**: Named returns only when clarity demands
- **Context propagation**: Context passed through call chains

---

## 9. Dependencies and Imports

### 9.1 Import Organization

- **MUST**: Organize imports in standard Go order:
  1. Standard library
  2. External dependencies
  3. Internal packages

- **TOOL**: Use `gci` linter to enforce (configured in `.golangci.yml`)

### 9.2 Import Aliases

- **MUST**: Use standard aliases for common packages:
  ```go
  corev1 "k8s.io/api/core/v1"
  appsv1 "k8s.io/api/apps/v1"
  metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
  k8serrors "k8s.io/apimachinery/pkg/api/errors"
  llamav1alpha1 "github.com/llamastack/llama-stack-k8s-operator/api/v1alpha1"
  ```

---

## 10. Documentation

### 10.1 Code Comments

- **MUST**: Export public functions, types, and constants have godoc comments
  ```go
  // LlamaStackDistributionReconciler reconciles a LlamaStack object.
  type LlamaStackDistributionReconciler struct { ... }
  ```

- **SHOULD**: Explain non-obvious logic with inline comments
- **MUST**: Document why, not what (code shows what)

### 10.2 Example Files

- **MUST**: Provide sample YAML files in `config/samples/`
- **MUST**: Include inline comments explaining configuration options
  ```yaml
  # Uncomment the storage section to use persistent storage
  # storage: {}  # Will use default size of 10Gi
  ```

### 10.3 API Documentation

- **MUST**: Use kubebuilder markers for field documentation
  ```go
  // Size is the size of the persistent volume claim
  // +kubebuilder:validation:Required
  Size *resource.Quantity `json:"size"`
  ```

### 10.4 Specification Document Formatting

- **MUST**: Use proper markdown bullet lists for all enumerated items
  ```markdown
  # Good - Proper markdown list
  - ✅ **Feature 1**: Description
  - ✅ **Feature 2**: Description
  - ❌ **Anti-pattern**: Description

  # Bad - Items without list markers
  ✅ **Feature 1**: Description
  ✅ **Feature 2**: Description
  ❌ **Anti-pattern**: Description
  ```

- **MUST**: Keep icons/emojis (✅, ❌, 🎯, etc.) but format as list items
- **RATIONALE**: Proper markdown lists render correctly on GitHub with consistent indentation and spacing
- **APPLIES TO**: All specification documents (spec.md, plan.md, tasks.md, pr-strategy.md, etc.)

---

## 11. Feature Flags

### 11.1 Feature Flag Pattern

- **MUST**: Use boolean fields in reconciler for feature flags
  ```go
  type Reconciler struct {
      client.Client
      Scheme *runtime.Scheme
      EnableNetworkPolicy bool  // Feature flag
  }
  ```

- **MUST**: Check feature flags before executing feature-specific code
  ```go
  if r.EnableNetworkPolicy {
      // NetworkPolicy logic
  }
  ```

- **RATIONALE**: Allows gradual rollout and easy disablement

### 11.2 ConfigMap-Based Feature Flags

- **PATTERN**: Use ConfigMap-based feature flags with YAML structure for gradual rollouts
- Feature flags defined in dedicated ConfigMap
- YAML-based structure with enable/disable semantics
- Features can be toggled without code changes or redeployments
- Optional features (e.g., NetworkPolicy) respect flag state
- Default values documented in code for clarity

---

## 12. API Versioning

### 12.1 Version Stability

- **CURRENT**: `v1alpha1` indicates API is not stable
- **FUTURE**: Promote to `v1beta1` when API stabilizes
- **FUTURE**: Promote to `v1` when API is production-ready

### 12.2 Breaking Changes

- **MUST**: Breaking changes in alpha versions are acceptable
- **MUST**: Beta versions should minimize breaking changes
- **MUST**: v1 versions MUST maintain backwards compatibility

### 12.3 Deprecation

- **MUST**: Deprecated fields MUST include deprecation notice in comments
- **MUST**: Maintain deprecated fields for at least one minor version
- **SHOULD**: Log warnings when deprecated fields are used

---

## 13. Security

### 13.1 Security by Default

- **MUST**: Apply restrictive security contexts and validate all security-related inputs
- **Pod-level FSGroup**: Set to 1001 for volume write access
- **Init containers**: AllowPrivilegeEscalation=false, RunAsNonRoot=true, drop ALL capabilities
- **PEM validation**: Validate certificate format before use
- **ServiceAccount per instance**: Following instance-name-sa pattern
- **SSL/TLS injection**: Proper CA bundle handling with auto-detection

---

## 14. Git Commit Guidelines

### 14.1 Commit Messages

- **MUST**: Use conventional commit format when applicable (feat:, fix:, docs:, etc.)
- **SHOULD**: Include context and reasoning in commit body for non-trivial changes
- **SHOULD**: Reference related issues/PRs when applicable

### 14.2 AI-Assisted Commits

- **MUST NOT**: Include `Co-Authored-By: Claude <noreply@anthropic.com>` or similar AI co-author attributions
- **RATIONALE**: AI co-authorship causes Contributor License Agreement (CLA) check failures
- **MUST**: Use `Assisted-by:` trailer for AI-assisted commits
- **FORMAT**: Add trailer after commit message body, before `Signed-off-by:`

**AI Attribution Format**:
```
Assisted-by: 🤖 Claude Code
```

**Example commit message**:
```bash
git commit -s -m "feat: add new feature

This feature implements...

Assisted-by: 🤖 Claude Code
Signed-off-by: Your Name <your.email@example.com>"
```

**Rationale**: The `Assisted-by:` trailer provides a consistent format for acknowledging AI assistance while maintaining CLA compliance and avoiding co-authorship issues.

### 14.3 Commit Sign-Off

- **MUST**: All commits MUST be signed off using `git commit --signoff` (or `-s`)
- **RATIONALE**: Sign-off indicates agreement with Developer Certificate of Origin (DCO)
- **FORMAT**: Adds `Signed-off-by: Your Name <your.email@example.com>` to commit message
- **ENFORCEMENT**: Pre-commit hooks or CI checks should verify sign-off is present

**Example**:
```bash
git commit -s -m "feat: add new feature"
# Results in:
# feat: add new feature
#
# Signed-off-by: John Doe <john.doe@example.com>
```

### 14.4 Commit Hygiene

- **SHOULD**: Make atomic commits (one logical change per commit)
- **SHOULD**: Ensure each commit passes tests and builds successfully
- **MUST**: Commits MUST pass all pre-commit hooks before being accepted

---

## 15. Build & Release Standards

### 15.1 Makefile Organization

- **Phony targets**: All targets declared as .PHONY
- **Help system**: Auto-generated help from inline comments
- **Tool versioning**: Pinned versions (kustomize, controller-gen, etc.)
- **Local overrides**: -include local.mk for custom development values
- **Release coordination**: VERSION and LLAMASTACK_VERSION updated together

### 15.2 Image Management

- **Default registry**: quay.io/llamastack
- **Multi-platform**: linux/arm64, amd64, s390x, ppc64le support
- **Container tool agnostic**: CONTAINER_TOOL variable (podman/docker)
- **Version tagging**: Semantic versioning with latest fallback

### 15.3 Code Quality Gates (NON-NEGOTIABLE)

- **Pre-commit hooks**: Mandatory formatting, linting, error message validation
- **CI enforcement**: Pre-commit runs in CI, blocks merges on failure
- **Linting**: golangci-lint with 100+ enabled linters
- **Format enforcement**: gci for imports, yamlfmt for YAML
- **Error message linting**: Custom check for "failed to" prefix in wrapped errors

### 15.4 Linter Configuration

- **Enable-all philosophy**: Start with all linters enabled, disable only with clear justification
- **Cyclomatic complexity**: Max 30 for functions
- **Function length**: Max 100 lines/statements (ignoring comments)
- **Line length**: 180 characters maximum
- **Magic numbers**: Contextual exceptions (0, 1, 2, 5, 10, 100)

---

## 16. Enforcement

This constitution is enforced through **automated tooling** and **code review practices**:

### Automated Enforcement

The project uses **pre-commit hooks** (`.pre-commit-config.yaml`) that run on every commit:

#### Standard Checks
- **Trailing whitespace** - Removed automatically
- **End-of-file fixer** - Ensures files end with newline
- **Large files** - Prevents commits > 1000KB
- **YAML validation** - Validates all YAML files
- **Private key detection** - Prevents accidental credential commits
- **Line ending consistency** - Forces LF (not CRLF)

#### Go-Specific Checks
- **`make lint`** - Runs golangci-lint with configured linters
  - **gci**: Import organization (stdlib → external → internal)
  - **gocyclo**: Cyclomatic complexity (max 30)
  - **errcheck**: Unchecked errors
  - **goconst**: Repeated strings that should be constants
  - **revive**: Style and best practices
  - **gocritic**: Bug detection and performance
  - Full configuration in `.golangci.yml`

- **`make generate manifests`** - Regenerates CRD manifests from code
  - Ensures kubebuilder markers are up-to-date
  - Validates CRD schema

- **`make api-docs`** - Regenerates API documentation
  - Keeps documentation in sync with code

- **`./hack/check_go_errors.py`** - Custom error message validation
  - Enforces lowercase error messages (Go convention)
  - Ensures error wrapping patterns

#### Security Checks
- **GitHub Actions pinning** - Ensures workflow actions use SHA hashes (not tags)
- **Prevents direct commits to main** - Forces pull requests

### Code Review Checklist

When reviewing code, verify:

- [ ] Constitution compliance for changed code (see Quick Reference)
- [ ] Kubebuilder validation tags for new CRD fields
- [ ] Status conditions updated appropriately
- [ ] Errors wrapped with `%w` and include context
- [ ] Tests use builder pattern and table-driven structure
- [ ] Owner references set on created resources
- [ ] Logging uses context logger with structured fields
- [ ] No `fmt.Printf` for logging

### Exceptions and Deviations

If you need to deviate from a MUST rule:

1. **Document why** in code comments
2. **Get approval** in PR review
3. **Create issue** if pattern needs updating
4. **Update constitution** if exception becomes common pattern

Example:
```go
// Deviation from constitution §4.1: Not wrapping error here because
// the error is immediately returned to user and stack trace not needed.
// See issue #123 for discussion.
return errors.New("invalid configuration")
```

---

## 17. Governance

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

---

## Adoption Guidelines

### For New Features

1. Read this constitution before starting implementation
2. Follow existing patterns for consistency
3. Add new patterns to constitution if they're project-wide
4. Update constitution when patterns change

### For Existing Code

- **Gradual Adoption**: Apply constitution patterns when touching code
- **No Big Rewrite**: Don't refactor everything at once
- **Document Exceptions**: If constitution doesn't apply, document why

### For Code Reviews

- Constitution compliance is a review criterion
- Suggest improvements based on constitution
- Challenge patterns that diverge without good reason

---

## References

- **Kubebuilder Book**: https://book.kubebuilder.io/
- **Kubernetes API Conventions**: https://github.com/kubernetes/community/blob/master/contributors/devel/sig-architecture/api-conventions.md
- **Controller-runtime**: https://pkg.go.dev/sigs.k8s.io/controller-runtime
