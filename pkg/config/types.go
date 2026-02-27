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

package config

import corev1 "k8s.io/api/core/v1"

// BaseConfig represents the parsed base config.yaml from a distribution.
// Fields use interface types to preserve arbitrary structure during merging.
type BaseConfig struct {
	Version           int                      `json:"version"                       yaml:"version"`
	APIs              []string                 `json:"apis,omitempty"                yaml:"apis,omitempty"`
	Providers         map[string]interface{}   `json:"providers,omitempty"           yaml:"providers,omitempty"`
	RegisteredModels  []map[string]interface{} `json:"models,omitempty"              yaml:"models,omitempty"`
	Shields           []map[string]interface{} `json:"shields,omitempty"             yaml:"shields,omitempty"`
	ToolGroups        []map[string]interface{} `json:"tool_groups,omitempty"         yaml:"tool_groups,omitempty"`
	MetadataStore     map[string]interface{}   `json:"metadata_store,omitempty"      yaml:"metadata_store,omitempty"`
	InferenceStore    map[string]interface{}   `json:"inference_store,omitempty"     yaml:"inference_store,omitempty"`
	SafetyStore       map[string]interface{}   `json:"safety_store,omitempty"        yaml:"safety_store,omitempty"`
	VectorIOStore     map[string]interface{}   `json:"vector_io_store,omitempty"     yaml:"vector_io_store,omitempty"`
	ToolRuntimeStore  map[string]interface{}   `json:"tool_runtime_store,omitempty"  yaml:"tool_runtime_store,omitempty"`
	TelemetryStore    map[string]interface{}   `json:"telemetry_store,omitempty"     yaml:"telemetry_store,omitempty"`
	PostTrainingStore map[string]interface{}   `json:"post_training_store,omitempty" yaml:"post_training_store,omitempty"`
	ScoringStore      map[string]interface{}   `json:"scoring_store,omitempty"       yaml:"scoring_store,omitempty"`
	EvalStore         map[string]interface{}   `json:"eval_store,omitempty"          yaml:"eval_store,omitempty"`
	DatasetIOStore    map[string]interface{}   `json:"datasetio_store,omitempty"     yaml:"datasetio_store,omitempty"`
	Server            map[string]interface{}   `json:"server,omitempty"              yaml:"server,omitempty"`
	ExternalProviders map[string]interface{}   `json:"external_providers,omitempty"  yaml:"external_providers,omitempty"`
	Extra             map[string]interface{}   `json:"-"                             yaml:"-"`
}

// GeneratedConfig is the output of the config generation pipeline.
type GeneratedConfig struct {
	// ConfigYAML is the final config.yaml content.
	ConfigYAML string
	// EnvVars are environment variable definitions for the Deployment container
	// (injecting secret values).
	EnvVars []corev1.EnvVar
	// ContentHash is the SHA256 hex digest of ConfigYAML for change detection.
	ContentHash string
	// ProviderCount is the total number of configured providers (for status reporting).
	ProviderCount int
	// ResourceCount is the total number of registered resources (for status reporting).
	ResourceCount int
	// ConfigVersion is the detected config.yaml schema version.
	ConfigVersion int
}

// SecretResolution collects the results of secret reference resolution.
type SecretResolution struct {
	// EnvVars are corev1.EnvVar definitions to inject into the Deployment container.
	EnvVars []corev1.EnvVar
	// Substitutions maps original field identifiers to ${env.VAR_NAME} placeholders
	// for insertion into the generated config.yaml.
	Substitutions map[string]string
}

// ProviderEntry represents a single expanded provider in config.yaml format.
type ProviderEntry struct {
	ProviderID   string                 `json:"provider_id"   yaml:"provider_id"`
	ProviderType string                 `json:"provider_type" yaml:"provider_type"`
	Config       map[string]interface{} `json:"config"        yaml:"config"`
}

// ConfigSource describes where configuration comes from.
type ConfigSource string

const (
	ConfigSourceGenerated           ConfigSource = "generated"
	ConfigSourceOverride            ConfigSource = "override"
	ConfigSourceDistributionDefault ConfigSource = "distribution-default"
)

// SupportedConfigVersionMin is the oldest config.yaml schema version the operator
// supports (n-1). SupportedConfigVersionMax is the current version (n).
const (
	SupportedConfigVersionMin = 1
	SupportedConfigVersionMax = 2
)
