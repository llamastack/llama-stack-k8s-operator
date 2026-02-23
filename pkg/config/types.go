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
	Version           int                      `yaml:"version" json:"version"`
	APIs              []string                 `yaml:"apis,omitempty" json:"apis,omitempty"`
	Providers         map[string]interface{}   `yaml:"providers,omitempty" json:"providers,omitempty"`
	RegisteredModels  []map[string]interface{} `yaml:"models,omitempty" json:"models,omitempty"`
	Shields           []map[string]interface{} `yaml:"shields,omitempty" json:"shields,omitempty"`
	ToolGroups        []map[string]interface{} `yaml:"tool_groups,omitempty" json:"tool_groups,omitempty"`
	MetadataStore     map[string]interface{}   `yaml:"metadata_store,omitempty" json:"metadata_store,omitempty"`
	InferenceStore    map[string]interface{}   `yaml:"inference_store,omitempty" json:"inference_store,omitempty"`
	SafetyStore       map[string]interface{}   `yaml:"safety_store,omitempty" json:"safety_store,omitempty"`
	VectorIOStore     map[string]interface{}   `yaml:"vector_io_store,omitempty" json:"vector_io_store,omitempty"`
	ToolRuntimeStore  map[string]interface{}   `yaml:"tool_runtime_store,omitempty" json:"tool_runtime_store,omitempty"`
	TelemetryStore    map[string]interface{}   `yaml:"telemetry_store,omitempty" json:"telemetry_store,omitempty"`
	PostTrainingStore map[string]interface{}   `yaml:"post_training_store,omitempty" json:"post_training_store,omitempty"`
	ScoringStore      map[string]interface{}   `yaml:"scoring_store,omitempty" json:"scoring_store,omitempty"`
	EvalStore         map[string]interface{}   `yaml:"eval_store,omitempty" json:"eval_store,omitempty"`
	DatasetIOStore    map[string]interface{}   `yaml:"datasetio_store,omitempty" json:"datasetio_store,omitempty"`
	Server            map[string]interface{}   `yaml:"server,omitempty" json:"server,omitempty"`
	ExternalProviders map[string]interface{}   `yaml:"external_providers,omitempty" json:"external_providers,omitempty"`
	Extra             map[string]interface{}   `yaml:"-" json:"-"`
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
	ProviderID   string                 `yaml:"provider_id" json:"provider_id"`
	ProviderType string                 `yaml:"provider_type" json:"provider_type"`
	Config       map[string]interface{} `yaml:"config" json:"config"`
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
