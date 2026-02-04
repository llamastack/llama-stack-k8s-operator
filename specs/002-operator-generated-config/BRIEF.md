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
apiVersion: llamastack.ai/v1alpha2
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
| Multiple providers | Explicit `id` required | Avoid ambiguity in provider references |
| Backward compat | Conversion webhook | v1alpha1 CRs continue working |
| Override escape hatch | `overrideConfig` field | Power users can bypass generation |

## Configuration Tiers

| Tier | Users | Mechanism |
|------|-------|-----------|
| Simple (80%) | Most users | Inline provider fields |
| Advanced (15%) | Platform engineers | Per-provider `settings` |
| Full Control (5%) | Power users | ConfigMap override |

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
4. **Edge Cases**: Are the 6 documented edge cases reasonable?
5. **External Dependencies**: Is the OCI label approach for base config extraction acceptable?

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

- **Spec 001**: External providers merge into generated config
- **Distribution images**: Must include OCI labels with base config

## Open Questions for Review

1. Should `expose: {}` (empty object) be treated as `expose: true`?
2. Should disabled API + provider config conflict cause validation error or warning?
3. Is the `LLSD_<PROVIDER>_<FIELD>` env var naming convention clear?

## Implementation Estimate

5 phases, 33 tasks (see [tasks.md](tasks.md) for details)

---

**Ready for detailed review?** See [spec.md](spec.md) for full requirements.
