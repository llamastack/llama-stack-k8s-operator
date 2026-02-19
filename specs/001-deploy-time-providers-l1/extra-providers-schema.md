# External Provider Entry Schema

**Purpose**: Define the schema for external provider entries in config.yaml using LlamaStack's native `module:` field support.

**Created**: 2025-11-13
**Updated**: 2026-01-29 (Simplified to use native module: support)

---

## Executive Summary

External providers are added directly to the config.yaml `providers` section using LlamaStack's native `module:` field. No separate extra-providers.yaml file is needed.

**How it works**:
1. Init containers install provider packages to shared volume
2. Config generation init container adds provider entries with `module:` field to config.yaml
3. PYTHONPATH is set to include the installed packages
4. LlamaStack imports providers via `importlib.import_module()` at runtime

---

## Provider Entry Schema

### Provider Entry in config.yaml

```yaml
providers:
  <api-type>:  # One of: inference, safety, agents, vector_io, datasetio, scoring, eval, tool_runtime, post_training
    - provider_id: string       # REQUIRED: Unique instance identifier (from CRD)
      provider_type: string     # REQUIRED: Provider type (from metadata, e.g., "remote::vllm")
      module: string            # REQUIRED: Python module path (from metadata, e.g., "my_org.vllm_provider")
      config: object            # OPTIONAL: Provider-specific configuration (from CRD)
```

### Field Mapping

| Field | Source | Description |
|-------|--------|-------------|
| `provider_id` | CRD `externalProviders.<api>.<n>.providerId` | User-assigned unique identifier |
| `provider_type` | Provider metadata `spec.providerType` | Provider type (e.g., "remote::vllm") |
| `module` | Provider metadata `spec.packageName` | Python module path for import via `importlib.import_module()` |
| `config` | CRD `externalProviders.<api>.<n>.config` | Provider-specific configuration |

---

## Example

### Complete config.yaml with External Providers

```yaml
version: 2
distro_name: my-custom-distribution

apis:
  - inference
  - safety
  - agents

providers:
  inference:
    # Base provider from distribution
    - provider_id: ollama
      provider_type: remote::ollama
      config:
        url: http://ollama:11434

    # External provider (added by operator)
    - provider_id: my-custom-vllm
      provider_type: remote::custom-vllm
      module: my_custom_vllm_provider      # LlamaStack imports this module
      config:
        url: http://vllm.default.svc:8000
        api_key: ${VLLM_API_KEY}

  safety:
    # External provider (added by operator)
    - provider_id: my-safety-provider
      provider_type: inline::custom-safety
      module: my_safety_provider           # LlamaStack imports this module
      config:
        safety_level: high

server:
  port: 8321
```

---

## How LlamaStack Processes the `module:` Field

When LlamaStack sees a provider entry with a `module:` field, it:

1. **Import the module**:
   ```python
   module = importlib.import_module(provider.module)
   ```

2. **Get provider specification**:
   ```python
   spec = module.get_provider_spec()
   ```

3. **Instantiate the provider**:
   - For remote providers: `impl = await module.get_adapter_impl(config, deps)`
   - For inline providers: `impl = await module.get_provider_impl(config, deps)`

4. **Register and serve** the provider via the appropriate API

---

## Config Generation Process

The config generation init container performs these steps:

1. **Read base config.yaml** (from distribution or user ConfigMap)

2. **Scan provider metadata** from `/opt/llama-stack/external-providers/metadata/`

3. **For each external provider**:
   - Read `lls-provider-spec.yaml` → get `providerType`, `packageName`, `api`
   - Read `crd-config.yaml` → get `providerId`, `config`
   - Add entry to config.yaml `providers.<api>` section:
     ```yaml
     - provider_id: {providerId}
       provider_type: {providerType}
       module: {packageName}
       config: {config}
     ```

4. **Write final config.yaml** to `/opt/llama-stack/config/config.yaml`

---

## Validation

### Pre-Generation Validation

Before adding provider entries:

1. **No duplicate provider IDs** across all API types
2. **API type match** - lls-provider-spec.yaml `api` must match CRD section
3. **Required fields present** - providerId, providerType, packageName

### Runtime Validation (by LlamaStack)

LlamaStack validates at runtime:

1. **Module importable** - `importlib.import_module()` succeeds
2. **get_provider_spec() exists** - module has required function
3. **ProviderSpec valid** - returned spec has required fields
4. **Provider instantiation** - get_adapter_impl/get_provider_impl succeeds

---

## Migration from Deprecated Approaches

### From `external_providers_dir`

The `external_providers_dir` StackConfig field is deprecated. Instead:

**Old approach** (deprecated):
```yaml
external_providers_dir: /path/to/providers
```

**New approach** (use module: field):
```yaml
providers:
  inference:
    - provider_id: my-provider
      provider_type: remote::my-provider
      module: my_provider_package    # Package must be in PYTHONPATH
      config: {}
```

### From extra-providers.yaml Merge

The separate extra-providers.yaml file and merge step are no longer needed. Provider entries are added directly to config.yaml.

---

## Benefits of Native Module Support

- ✅ **Simpler architecture** - No separate schema file, no merge step
- ✅ **Native LlamaStack support** - Uses existing `module:` field mechanism
- ✅ **Self-contained** - Init containers install packages, PYTHONPATH is set
- ✅ **No external dependencies** - No pip install from external indexes
- ✅ **Kubernetes-native** - Provider packages come from container images
