# Spec Brief: Operator-Generated Config (v1alpha2)

**Full spec:** [spec.md](spec.md) | **Status:** Draft | **Priority:** P1

## Problem Statement

Users currently must provide a complete `config.yaml` via ConfigMap to configure LlamaStack. This requires deep knowledge of the config schema and results in verbose, error-prone YAML.

## Solution

Introduce v1alpha2 API with high-level abstractions that the operator expands into a complete `config.yaml`. Users write 10-20 lines instead of 200+.

## Before/After Example

**Before (v1alpha1):** User provides 200+ line ConfigMap manually

**After (v1alpha2):**
```yaml
apiVersion: llamastack.io/v1alpha2
kind: LlamaStackDistribution
metadata:
  name: my-stack
spec:
  distribution:
    name: starter
  providers:
    inference:
      provider: vllm
      endpoint: "http://vllm:8000"
      apiKey:
        secretKeyRef: {name: vllm-creds, key: token}
  resources:
    models: ["llama3.2-8b"]
  storage:
    sql:
      type: postgres
      connectionString:
        secretKeyRef: {name: pg-creds, key: url}
```

## Key Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Config extraction | OCI image labels | Single-phase reconcile, works with imagePullSecrets |
| Secret handling | Environment variables | Never embed secrets in ConfigMap |
| Multiple providers | Explicit `id` required for providers 2..N | Avoid ambiguity in provider references |
| Backward compat | Conversion webhook | v1alpha1 CRs continue working |
| Override escape hatch | `overrideConfig` field | Power users can bypass generation |

## Configuration Tiers

| Tier | Mechanism |
|------|-----------|
| Simple (80%) | Inline provider fields |
| Advanced (15%) | Per-provider `settings` |
| Full Control (5%) | ConfigMap override |

## New Spec Sections

```
spec:
  distribution:     # Image source (name or direct image)
  providers:        # Inference, safety, vectorIo, toolRuntime, telemetry
  resources:        # Models, tools, shields to register
  storage:          # KV (sqlite/redis) and SQL (sqlite/postgres)
  disabled:         # APIs to disable
  networking:       # Port, TLS, expose, allowedFrom
  workload:         # Replicas, resources, autoscaling, PDB
  overrideConfig:   # Escape hatch: use ConfigMap directly
```

## What Reviewers Should Focus On

1. **API Design**: Does the field structure make sense? Any awkward names?
2. **Polymorphic Fields**: Single object vs list forms (providers, models)
3. **Storage Abstraction**: Is kv/sql split intuitive?
4. **Edge Cases**: Are the 12 documented edge cases reasonable?
5. **Phased Base Config**: Is the embedded configs (Phase 1) + OCI labels (Phase 2) approach acceptable? Any better idea how to extract the `config.yaml` from the distribution OCI image?
6. **OQ-004**: Should the operator auto-create a default LLSD instance on install?

## Requirements Summary

| Category | Count | Coverage |
|----------|-------|----------|
| CRD Schema | FR-001 to FR-014 | All new fields defined |
| Config Generation | FR-020 to FR-029 | Extraction, merging, versioning |
| Providers | FR-030 to FR-035 | Field mapping, ID generation |
| Resources | FR-040 to FR-044 | Models, tools, shields |
| Storage | FR-050 to FR-053 | KV and SQL backends |
| Networking | FR-060 to FR-066 | Port, TLS, expose, NetworkPolicy |
| Validation | FR-070 to FR-075 | CEL rules, secret/ConfigMap checks |
| Conversion | FR-080 to FR-083 | v1alpha1 ↔ v1alpha2 webhook |
| Integration | FR-090 to FR-092 | Spec 001 external providers |

## User Stories (P1 only)

1. **Simple Inference**: Deploy with just `providers.inference` config
2. **Multiple Providers**: Configure primary + fallback providers
3. **Resource Registration**: Register models/tools declaratively
4. **State Storage**: Configure PostgreSQL for persistence

## Dependencies

- **Spec 001**: External providers merge into generated config (not mandatory, but was already included in this design)
- **Distribution images**: Must include OCI labels with base config (check: build system must support this, registry must support label queries (check for disconnected))

## Open Questions

Previously open questions (OQ-001 through OQ-003) have been resolved:

- **OQ-001** (Resolved): `expose: {}` is treated as `expose: true`
- **OQ-002** (Resolved): Disabled API + provider config conflict produces a warning (not error). Disabled takes precedence.
- **OQ-003** (Resolved): Env var naming uses provider ID: `LLSD_<PROVIDER_ID>_<FIELD>` (unique, collision-free)

Currently open:

- **OQ-004**: Should the operator create a default LlamaStackDistribution instance when installed? If adopted, it should be opt-in via operator configuration (e.g., a Helm value or OLM parameter).

## Implementation Estimate

5 phases, 38 tasks (see [tasks.md](tasks.md) for details)

---

## Changes Since Initial Spec (2026-02-10)

The following spec.md updates were applied after a cross-artifact consistency analysis. These are additive refinements, not structural redesigns.

### New: User Story 8 (Runtime Configuration Updates, P1)

Covers day-2 operations: CR updates trigger config regeneration, no-op detection (skip restart when config unchanged), failure preservation (current Deployment kept running on error), and atomic image+config updates on distribution changes.

### New: Phased Base Config Extraction (FR-027a to FR-027j)

Replaced the single "OCI label extraction" approach with a two-phase strategy:
- **Phase 1 (MVP)**: Embedded default configs via `go:embed`, no distribution image changes needed
- **Phase 2 (Enhancement)**: OCI label extraction takes precedence when labels are present

This introduces new **Operator Build Requirements**: the operator binary must ship with `distributions.json` (mapping distribution names to image references) and a `configs/<name>/config.yaml` for each named distribution. These are maintained together and updated as part of the operator release process. Downstream builds (e.g., RHOAI) use the existing `image-overrides` mechanism to remap image references without rebuilding the operator.

### New: Runtime Configuration Requirements (FR-095 to FR-101)

- **FR-095-096**: Regenerate on spec change; skip restart when content hash is identical
- **FR-097**: On failure, preserve the current running Deployment unchanged
- **FR-098-099**: Atomic updates when distribution changes; status conditions reflect update state
- **FR-100**: Image + config updated in a single Deployment update (no intermediate mismatch)
- **FR-101**: Operator upgrade failure handling with `UpgradeConfigFailure` reason

### New: Validation Webhook Requirements (FR-076 to FR-078)

Validating admission webhook for constraints beyond CEL: Secret existence, ConfigMap references, cross-field provider ID validation. Deployed via kustomize. Cluster-scoped `ValidatingWebhookConfiguration` documented as an accepted deviation from constitution §1.1.

### Expanded: Six New Edge Cases

- CR update during active rollout (supersedes in-progress rollout)
- Operator upgrade with running instances (atomic image+config update)
- Config generation failure on update (preserve current Deployment)
- Deeply nested secretKeyRef (top-level only, deeper nesting passed through)
- Tools without toolRuntime provider (fallback to base config, then error)
- Shields without safety provider (same fallback pattern)

### Refined: Existing Requirements

- **FR-005**: secretKeyRef discovery depth constrained to top-level settings values only
- **FR-013**: overrideConfig ConfigMap must be in the same namespace as the CR
- **FR-020**: Expanded into FR-020/020a/020b/020c covering distribution resolution, status tracking, and image+config consistency
- **FR-032**: Env var naming clarified with provider ID example (`LLSD_VLLM_PRIMARY_API_KEY`)
- **FR-043/FR-044**: Tool/shield provider assignment now falls back to base config before erroring
- **FR-070**: Mutual exclusivity expanded to cover all four fields (`providers`, `resources`, `storage`, `disabled`)

### New: Printer Columns (constitution §2.5)

Default `kubectl get llsd`: Phase, Providers, Available, Age. Wide output adds Distribution image and Config name.

### New: Status Fields

- `resolvedDistribution` (image, configSource, configHash) for change detection across reconciliations and operator upgrades
- `DeploymentUpdated` and `Available` conditions added alongside existing `ConfigGenerated` and `SecretsResolved`

### CRD Schema Corrections

- API group fixed: `llamastack.io` (was `llamastack.ai` in draft)
- `targetCPUUtilizationPercentage` aligned with existing v1alpha1 naming
- Provider `host` field moved into `settings` (uniform provider schema)

---

**Ready for detailed review?** See [spec.md](spec.md) for full requirements.
