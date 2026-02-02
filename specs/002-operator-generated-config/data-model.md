# Data Model: Operator-Generated Server Configuration

**Feature Branch**: `002-operator-generated-config`
**Date**: 2026-02-02

---

## Overview

This document defines the Go types that will be added to `api/v1alpha1/` to support operator-generated server configuration.

---

## Type Definitions

### ServerSpec Extensions

Add to existing `ServerSpec` in `llamastackdistribution_types.go`:

```go
// ServerSpec defines the configuration for the llama-stack server
type ServerSpec struct {
    // Existing fields...
    Distribution     DistributionType `json:"distribution"`
    ContainerSpec    *ContainerSpec   `json:"containerSpec,omitempty"`
    UserConfig       *UserConfigSpec  `json:"userConfig,omitempty"`

    // NEW: Operator-generated configuration fields

    // Providers configures inference, safety, and other API providers
    // Mutually exclusive with UserConfig.ConfigMapName
    // +optional
    Providers *ProvidersSpec `json:"providers,omitempty"`

    // Disabled lists provider types to exclude from configuration
    // Uses same keys as Providers (inference, safety, vectorIo, etc.)
    // +optional
    Disabled []string `json:"disabled,omitempty"`

    // Storage configures the persistence backend
    // +optional
    Storage *StorageConfigSpec `json:"storage,omitempty"`

    // Resources registers models, tools, and shields
    // +optional
    Resources *ResourcesSpec `json:"resources,omitempty"`

    // Port for the llama-stack server (default: 8321)
    // +optional
    // +kubebuilder:default:=8321
    Port int32 `json:"port,omitempty"`

    // TLS configures TLS for the server
    // +optional
    TLS *ServerTLSConfig `json:"tls,omitempty"`
}
```

### ProvidersSpec

```go
// ProvidersSpec contains configuration for all provider types
// +kubebuilder:validation:XValidation:rule="!(has(self.providers) && has(self.userConfig))",message="providers and userConfig are mutually exclusive"
type ProvidersSpec struct {
    // Inference configures LLM inference providers
    // Accepts single provider object or list of providers
    // +optional
    Inference *ProviderConfigOrList `json:"inference,omitempty"`

    // Safety configures content moderation providers
    // +optional
    Safety *ProviderConfigOrList `json:"safety,omitempty"`

    // VectorIo configures vector database providers
    // +optional
    VectorIo *ProviderConfigOrList `json:"vectorIo,omitempty"`

    // Agents configures agent orchestration providers
    // +optional
    Agents *ProviderConfigOrList `json:"agents,omitempty"`

    // Memory configures memory/conversation persistence
    // +optional
    Memory *ProviderConfigOrList `json:"memory,omitempty"`

    // ToolRuntime configures tool execution providers
    // +optional
    ToolRuntime *ProviderConfigOrList `json:"toolRuntime,omitempty"`

    // Telemetry configures observability providers
    // +optional
    Telemetry *ProviderConfigOrList `json:"telemetry,omitempty"`
}
```

### ProviderConfigOrList (Polymorphic Type)

```go
// ProviderConfigOrList allows either a single provider or a list of providers
// When single: auto-generates provider_id from type
// When list: requires explicit id field on each entry
// +kubebuilder:validation:XValidation:rule="!self.isList || self.entries.all(e, has(e.id))",message="Multiple providers require explicit 'id' field"
type ProviderConfigOrList struct {
    // Single provider configuration (common case)
    // Used when only one provider of this type is needed
    // +optional
    Single *ProviderEntry `json:"-"`

    // List of provider configurations (advanced case)
    // Each entry must have explicit 'id' field
    // +optional
    Entries []ProviderEntry `json:"-"`

    // Internal: tracks if list form was used
    isList bool `json:"-"`
}

// Custom JSON marshaling/unmarshaling for polymorphic behavior
func (p *ProviderConfigOrList) UnmarshalJSON(data []byte) error {
    // Try list first (check for array)
    var list []ProviderEntry
    if err := json.Unmarshal(data, &list); err == nil {
        p.Entries = list
        p.isList = true
        return nil
    }

    // Try single object
    var single ProviderEntry
    if err := json.Unmarshal(data, &single); err != nil {
        return err
    }
    p.Single = &single
    p.isList = false
    return nil
}

func (p ProviderConfigOrList) MarshalJSON() ([]byte, error) {
    if p.isList {
        return json.Marshal(p.Entries)
    }
    return json.Marshal(p.Single)
}

// GetEntries returns normalized list of entries
func (p *ProviderConfigOrList) GetEntries() []ProviderEntry {
    if p.isList {
        return p.Entries
    }
    if p.Single != nil {
        return []ProviderEntry{*p.Single}
    }
    return nil
}
```

### ProviderEntry

```go
// ProviderEntry defines a single provider configuration
type ProviderEntry struct {
    // ID is the unique identifier for this provider
    // Required when multiple providers of same type are configured
    // Auto-generated from provider type when single provider is used
    // +optional
    ID string `json:"id,omitempty"`

    // Provider specifies the provider type (e.g., vllm, ollama, openai)
    // Maps to provider_type with remote:: prefix in config.yaml
    // +kubebuilder:validation:Required
    // +kubebuilder:validation:Enum=vllm;ollama;openai;anthropic;bedrock;azure;gemini;together;fireworks;groq;nvidia;llama-guard;pgvector
    Provider string `json:"provider"`

    // Endpoint is the URL for the provider service
    // Maps to config.url in config.yaml
    // +optional
    Endpoint string `json:"endpoint,omitempty"`

    // APIKey references a Kubernetes secret for authentication
    // Resolved to environment variable, referenced as ${env.VAR_NAME}
    // +optional
    APIKey *SecretKeyRefSource `json:"apiKey,omitempty"`

    // Config contains additional provider-specific configuration
    // Passed through as-is to config.yaml
    // +optional
    // +kubebuilder:pruning:PreserveUnknownFields
    Config *runtime.RawExtension `json:"config,omitempty"`

    // Host references a secret for database/service hostname
    // Used primarily for vectorIo providers
    // +optional
    Host *SecretKeyRefSource `json:"host,omitempty"`

    // Region for cloud providers (required for bedrock)
    // +optional
    Region string `json:"region,omitempty"`

    // DeploymentName for Azure OpenAI (required for azure)
    // +optional
    DeploymentName string `json:"deploymentName,omitempty"`
}
```

### SecretKeyRefSource

```go
// SecretKeyRefSource references a key in a Kubernetes Secret
type SecretKeyRefSource struct {
    // SecretKeyRef references a specific key in a Secret
    // +kubebuilder:validation:Required
    SecretKeyRef corev1.SecretKeySelector `json:"secretKeyRef"`
}
```

### StorageConfigSpec

```go
// StorageConfigSpec defines the storage backend configuration
type StorageConfigSpec struct {
    // Type specifies the storage backend type
    // +kubebuilder:validation:Enum=sqlite;postgres
    // +kubebuilder:default:=sqlite
    Type string `json:"type,omitempty"`

    // ConnectionString references a secret containing the database URL
    // Required when Type is postgres
    // +optional
    ConnectionString *SecretKeyRefSource `json:"connectionString,omitempty"`
}
```

### ResourcesSpec

```go
// ResourcesSpec defines registered resources (models, tools, shields)
type ResourcesSpec struct {
    // Models to register with inference providers
    // Accepts simple strings or detailed model configurations
    // +optional
    Models []ModelEntry `json:"models,omitempty"`

    // Tools (tool groups) to register
    // +optional
    Tools []string `json:"tools,omitempty"`

    // Shields to register with safety providers
    // +optional
    Shields []string `json:"shields,omitempty"`
}

// ModelEntry defines a model registration
// Can be specified as just a string (model name) or detailed config
type ModelEntry struct {
    // Name of the model (simple form: just the model name)
    // +kubebuilder:validation:Required
    Name string `json:"name"`

    // Provider ID to use for this model
    // If not specified, uses first configured inference provider
    // +optional
    Provider string `json:"provider,omitempty"`

    // Metadata contains model-specific properties
    // +optional
    Metadata *ModelMetadata `json:"metadata,omitempty"`
}

// ModelMetadata contains model-specific configuration
type ModelMetadata struct {
    // ContextLength is the maximum context window size
    // +optional
    ContextLength int64 `json:"contextLength,omitempty"`

    // EmbeddingDimension for embedding models
    // +optional
    EmbeddingDimension int32 `json:"embeddingDimension,omitempty"`
}
```

### ServerTLSConfig

```go
// ServerTLSConfig defines TLS configuration for the server
type ServerTLSConfig struct {
    // Enabled indicates whether TLS is enabled
    // +optional
    Enabled bool `json:"enabled,omitempty"`

    // SecretName references a TLS secret (type kubernetes.io/tls)
    // +optional
    SecretName string `json:"secretName,omitempty"`
}
```

---

## Status Extensions

Add to `LlamaStackDistributionStatus`:

```go
// LlamaStackDistributionStatus defines the observed state
type LlamaStackDistributionStatus struct {
    // Existing fields...

    // Conditions represent the latest available observations
    // Includes: DeploymentReady, ServiceReady, ValidationSucceeded, SecretsResolved, ConfigReady
    Conditions []metav1.Condition `json:"conditions,omitempty"`

    // GeneratedConfigMap is the name of the operator-generated ConfigMap
    // Format: <llsd-name>-config-<hash>
    // +optional
    GeneratedConfigMap string `json:"generatedConfigMap,omitempty"`
}
```

### Condition Types

```go
const (
    // Existing condition types...
    ConditionTypeDeploymentReady = "DeploymentReady"
    ConditionTypeServiceReady    = "ServiceReady"

    // NEW: Config generation conditions

    // ConditionTypeValidationSucceeded indicates CRD schema validation passed
    ConditionTypeValidationSucceeded = "ValidationSucceeded"

    // ConditionTypeSecretsResolved indicates all secretKeyRef references resolved
    ConditionTypeSecretsResolved = "SecretsResolved"

    // ConditionTypeConfigReady indicates config.yaml generation succeeded
    ConditionTypeConfigReady = "ConfigReady"
)

// Condition reasons
const (
    ReasonValidationSucceeded = "ValidationSucceeded"
    ReasonValidationFailed    = "ValidationFailed"
    ReasonSecretsResolved     = "SecretsResolved"
    ReasonSecretNotFound      = "SecretNotFound"
    ReasonConfigGenerated     = "ConfigGenerated"
    ReasonConfigFailed        = "ConfigGenerationFailed"
)
```

---

## Validation Rules (CEL)

### Mutual Exclusivity

```go
// On ServerSpec
// +kubebuilder:validation:XValidation:rule="!(has(self.providers) && has(self.userConfig) && has(self.userConfig.configMapName))",message="spec.server.providers and spec.server.userConfig.configMapName are mutually exclusive"
```

### Provider ID Requirement for Lists

```go
// On ProviderConfigOrList
// +kubebuilder:validation:XValidation:rule="!isList(self) || self.all(e, has(e.id))",message="Multiple providers require explicit 'id' field"
```

### Provider-Specific Required Fields

```go
// On ProviderEntry
// +kubebuilder:validation:XValidation:rule="self.provider != 'bedrock' || has(self.region)",message="region is required for bedrock provider"
// +kubebuilder:validation:XValidation:rule="self.provider != 'azure' || has(self.deploymentName)",message="deploymentName is required for azure provider"
```

### Storage Connection String Requirement

```go
// On StorageConfigSpec
// +kubebuilder:validation:XValidation:rule="self.type != 'postgres' || has(self.connectionString)",message="connectionString is required when storage type is postgres"
```

---

## Entity Relationships

```
LlamaStackDistribution
├── ServerSpec
│   ├── ProvidersSpec
│   │   ├── Inference (ProviderConfigOrList)
│   │   │   └── ProviderEntry[]
│   │   │       ├── SecretKeyRefSource (apiKey)
│   │   │       └── RawExtension (config)
│   │   ├── Safety (ProviderConfigOrList)
│   │   ├── VectorIo (ProviderConfigOrList)
│   │   └── ... (other provider types)
│   ├── StorageConfigSpec
│   │   └── SecretKeyRefSource (connectionString)
│   ├── ResourcesSpec
│   │   ├── ModelEntry[]
│   │   ├── Tools[]
│   │   └── Shields[]
│   ├── Disabled[]
│   ├── Port
│   └── ServerTLSConfig
└── Status
    ├── Conditions[]
    └── GeneratedConfigMap
```

---

## Notes

1. **Pointer Types**: All optional nested structs use pointers to distinguish "not set" from "empty"
2. **RawExtension**: Used for `config` field to allow arbitrary provider-specific fields
3. **CEL Validation**: Complex cross-field validation done via XValidation rules
4. **Polymorphic Handling**: Custom JSON unmarshaling handles single-vs-list pattern
