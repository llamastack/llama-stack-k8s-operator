/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1alpha1

import (
	"encoding/json"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// SecretKeyRefSource references a key in a Kubernetes Secret.
type SecretKeyRefSource struct {
	// SecretKeyRef references a specific key in a Secret
	// +kubebuilder:validation:Required
	SecretKeyRef corev1.SecretKeySelector `json:"secretKeyRef"`
}

// ProviderEntry defines a single provider configuration
// +kubebuilder:validation:XValidation:rule="self.provider != 'bedrock' || has(self.region)",message="region is required for bedrock provider"
// +kubebuilder:validation:XValidation:rule="self.provider != 'azure' || has(self.deploymentName)",message="deploymentName is required for azure provider"
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

// ProviderConfigOrList allows either a single provider or a list of providers
// When single: auto-generates provider_id from type
// When list: requires explicit id field on each entry.
type ProviderConfigOrList struct {
	// Single provider configuration (common case)
	// Used when only one provider of this type is needed
	// +optional
	Single *ProviderEntry `json:"-"`

	// List of provider configurations (advanced case)
	// Each entry must have explicit 'id' field
	// +optional
	Entries []ProviderEntry `json:"-"`

	// isList tracks if list form was used (internal, not serialized)
	isList bool `json:"-"`
}

// UnmarshalJSON implements custom JSON unmarshaling for polymorphic behavior.
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

// MarshalJSON implements custom JSON marshaling for polymorphic behavior
func (p ProviderConfigOrList) MarshalJSON() ([]byte, error) {
	if p.isList {
		return json.Marshal(p.Entries)
	}
	return json.Marshal(p.Single)
}

// GetEntries returns normalized list of entries
func (p *ProviderConfigOrList) GetEntries() []ProviderEntry {
	if p == nil {
		return nil
	}
	if p.isList {
		return p.Entries
	}
	if p.Single != nil {
		return []ProviderEntry{*p.Single}
	}
	return nil
}

// IsList returns true if the list form was used
func (p *ProviderConfigOrList) IsList() bool {
	if p == nil {
		return false
	}
	return p.isList
}

// NewProviderConfigList creates a ProviderConfigOrList from a list of entries
func NewProviderConfigList(entries []ProviderEntry) *ProviderConfigOrList {
	return &ProviderConfigOrList{
		Entries: entries,
		isList:  true,
	}
}

// NewProviderConfigSingle creates a ProviderConfigOrList from a single entry
func NewProviderConfigSingle(entry *ProviderEntry) *ProviderConfigOrList {
	return &ProviderConfigOrList{
		Single: entry,
		isList: false,
	}
}

// ProvidersSpec contains configuration for all provider types
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

// StorageConfigSpec defines the storage backend configuration
// +kubebuilder:validation:XValidation:rule="self.type != 'postgres' || has(self.connectionString)",message="connectionString is required when storage type is postgres"
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

// ServerTLSConfig defines TLS configuration for the server
type ServerTLSConfig struct {
	// Enabled indicates whether TLS is enabled
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// SecretName references a TLS secret (type kubernetes.io/tls)
	// +optional
	SecretName string `json:"secretName,omitempty"`
}

// ValidDisabledProviderTypes contains the list of valid provider types for the disabled field
var ValidDisabledProviderTypes = []string{
	"inference", "safety", "vectorIo", "agents", "memory",
	"toolRuntime", "telemetry", "datasetio", "scoring",
	"eval", "postTraining",
}
