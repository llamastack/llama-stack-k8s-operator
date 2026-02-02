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

package configgen

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	llamav1alpha1 "github.com/llamastack/llama-stack-k8s-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"
)

// Generator handles config.yaml generation from LLSD spec
type Generator struct {
	distributionConfigs map[string][]byte
}

// GeneratorResult contains the output of config generation
type GeneratorResult struct {
	// ConfigYAML is the generated config.yaml content
	ConfigYAML []byte
	// EnvVars are environment variables to inject into the deployment
	EnvVars []corev1.EnvVar
	// ConfigMapName is the deterministic name for the ConfigMap
	ConfigMapName string
	// Warnings contains non-fatal issues encountered during generation
	Warnings []string
}

// NewGenerator creates a new Generator instance
func NewGenerator() *Generator {
	return &Generator{
		distributionConfigs: DistributionConfigs,
	}
}

// Generate creates config.yaml from an LLSD instance
func (g *Generator) Generate(instance *llamav1alpha1.LlamaStackDistribution) (*GeneratorResult, error) {
	// 1. Load distribution base config
	baseConfig, err := g.loadBaseConfig(instance.Spec.Server.Distribution.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to load distribution config for spec.server.distribution.name: %w", err)
	}

	// 2. Apply provider overrides
	config, envVars, err := g.applyProviders(baseConfig, instance.Spec.Server.Providers)
	if err != nil {
		return nil, err
	}

	// 3. Apply disabled list
	var warnings []string
	config, warnings = g.removeDisabled(config, instance.Spec.Server.Disabled)

	// 4. Apply storage config
	config, storageEnvVars, err := g.applyStorage(config, instance.Spec.Server.ConfigStorage)
	if err != nil {
		return nil, err
	}
	envVars = append(envVars, storageEnvVars...)

	// 5. Apply resources
	config, err = g.applyResources(config, instance.Spec.Server.Resources, instance.Spec.Server.Providers)
	if err != nil {
		return nil, err
	}

	// 6. Apply server settings (port, TLS)
	config = g.applyServerSettings(config, instance.Spec.Server.Port, instance.Spec.Server.ServerTLS)

	// 7. Serialize to YAML
	configYAML, err := yaml.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize config: %w", err)
	}

	return &GeneratorResult{
		ConfigYAML:    configYAML,
		EnvVars:       envVars,
		ConfigMapName: g.ConfigMapName(instance.Name, configYAML),
		Warnings:      warnings,
	}, nil
}

// loadBaseConfig reads the embedded distribution config
func (g *Generator) loadBaseConfig(distributionName string) (map[string]interface{}, error) {
	configBytes := GetDistributionConfig(distributionName)
	if configBytes == nil {
		return nil, fmt.Errorf("failed to find distribution '%s'", distributionName)
	}

	var config map[string]interface{}
	if err := yaml.Unmarshal(configBytes, &config); err != nil {
		return nil, fmt.Errorf("failed to parse distribution config: %w", err)
	}

	return config, nil
}

// ConfigMapName generates a deterministic ConfigMap name based on content hash
func (g *Generator) ConfigMapName(llsdName string, configContent []byte) string {
	hash := sha256.Sum256(configContent)
	shortHash := hex.EncodeToString(hash[:])[:10]
	return fmt.Sprintf("%s-config-%s", llsdName, shortHash)
}

// applyProviders merges user-defined providers into the base config
func (g *Generator) applyProviders(config map[string]interface{}, providers *llamav1alpha1.ProvidersSpec) (map[string]interface{}, []corev1.EnvVar, error) {
	if providers == nil {
		return config, nil, nil
	}

	var allEnvVars []corev1.EnvVar

	// Ensure providers section exists
	if config["providers"] == nil {
		config["providers"] = make(map[string]interface{})
	}
	providersSection, ok := config["providers"].(map[string]interface{})
	if !ok {
		providersSection = make(map[string]interface{})
		config["providers"] = providersSection
	}

	// Process each provider type
	providerTypes := map[string]*llamav1alpha1.ProviderConfigOrList{
		"inference":    providers.Inference,
		"safety":       providers.Safety,
		"vector_io":    providers.VectorIo,
		"agents":       providers.Agents,
		"memory":       providers.Memory,
		"tool_runtime": providers.ToolRuntime,
		"telemetry":    providers.Telemetry,
	}

	// Validate unique provider IDs across all provider types (FR-030)
	if err := g.validateUniqueProviderIDs(providerTypes); err != nil {
		return nil, nil, err
	}

	for providerType, providerConfig := range providerTypes {
		if providerConfig == nil {
			continue
		}

		entries := providerConfig.GetEntries()
		if len(entries) == 0 {
			continue
		}

		var providerList []map[string]interface{}
		for i, entry := range entries {
			providerEntry, envVars, err := g.mapProviderEntry(providerType, entry, i, len(entries) > 1)
			if err != nil {
				return nil, nil, err
			}
			providerList = append(providerList, providerEntry)
			allEnvVars = append(allEnvVars, envVars...)
		}

		providersSection[providerType] = providerList
	}

	return config, allEnvVars, nil
}

// mapProviderEntry converts a ProviderEntry to config.yaml format
func (g *Generator) mapProviderEntry(providerType string, entry llamav1alpha1.ProviderEntry, index int, requiresID bool) (map[string]interface{}, []corev1.EnvVar, error) {
	// Validate provider type
	if err := ValidateProviderType(entry.Provider); err != nil {
		return nil, nil, fmt.Errorf("failed to validate spec.server.providers.%s[%d].provider: %w", providerType, index, err)
	}

	// Validate ID is present for list form
	if requiresID && entry.ID == "" {
		return nil, nil, fmt.Errorf("failed to validate spec.server.providers.%s[%d].id: required when multiple providers are configured", providerType, index)
	}

	result := make(map[string]interface{})
	var envVars []corev1.EnvVar

	// Set provider_id (auto-generate if single)
	if entry.ID != "" {
		result["provider_id"] = entry.ID
	} else {
		result["provider_id"] = entry.Provider
	}

	// Map provider type with remote:: prefix
	result["provider_type"] = MapProviderType(entry.Provider)

	// Build config section
	configSection := make(map[string]interface{})

	// Map endpoint to url
	if entry.Endpoint != "" {
		configSection["url"] = entry.Endpoint
	}

	// Handle API key secret reference
	if entry.APIKey != nil {
		envVarName := GenerateEnvVarName(providerType, entry.ID, "api_key")
		configSection["api_key"] = fmt.Sprintf("${env.%s}", envVarName)
		envVars = append(envVars, corev1.EnvVar{
			Name: envVarName,
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &entry.APIKey.SecretKeyRef,
			},
		})
	}

	// Handle host secret reference
	if entry.Host != nil {
		envVarName := GenerateEnvVarName(providerType, entry.ID, "host")
		configSection["host"] = fmt.Sprintf("${env.%s}", envVarName)
		envVars = append(envVars, corev1.EnvVar{
			Name: envVarName,
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &entry.Host.SecretKeyRef,
			},
		})
	}

	// Add region if specified
	if entry.Region != "" {
		configSection["region"] = entry.Region
	}

	// Add deployment name if specified
	if entry.DeploymentName != "" {
		configSection["deployment_name"] = entry.DeploymentName
	}

	// Merge pass-through config fields
	if entry.Config != nil && entry.Config.Raw != nil {
		var customConfig map[string]interface{}
		if err := yaml.Unmarshal(entry.Config.Raw, &customConfig); err == nil {
			for k, v := range customConfig {
				// Don't override already set fields
				if _, exists := configSection[k]; !exists {
					configSection[k] = v
				}
			}
		}
	}

	if len(configSection) > 0 {
		result["config"] = configSection
	}

	return result, envVars, nil
}

// removeDisabled removes disabled provider types from config
// Returns the modified config and a list of warnings for disabled types not present in config
func (g *Generator) removeDisabled(config map[string]interface{}, disabled []string) (map[string]interface{}, []string) {
	if len(disabled) == 0 {
		return config, nil
	}

	providersSection, ok := config["providers"].(map[string]interface{})
	if !ok {
		return config, nil
	}

	var warnings []string
	for _, providerType := range disabled {
		// Convert camelCase to snake_case for config.yaml
		configKey := camelToSnake(providerType)

		// Check if the provider type exists before removing (FR-035)
		if _, exists := providersSection[configKey]; !exists {
			warnings = append(warnings,
				fmt.Sprintf("disabled provider type '%s' not present in distribution config", providerType))
		}
		delete(providersSection, configKey)
	}

	return config, warnings
}

// applyStorage merges storage configuration
func (g *Generator) applyStorage(config map[string]interface{}, storage *llamav1alpha1.StorageConfigSpec) (map[string]interface{}, []corev1.EnvVar, error) {
	if storage == nil {
		return config, nil, nil
	}

	var envVars []corev1.EnvVar

	metadataStore := make(map[string]interface{})
	metadataStore["type"] = storage.Type

	if storage.Type == "postgres" && storage.ConnectionString != nil {
		envVarName := GenerateEnvVarName("storage", "", "connection_string")
		metadataStore["connection_string"] = fmt.Sprintf("${env.%s}", envVarName)
		envVars = append(envVars, corev1.EnvVar{
			Name: envVarName,
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &storage.ConnectionString.SecretKeyRef,
			},
		})
	} else if storage.Type == "sqlite" {
		metadataStore["db_path"] = "/.llama/runtime/metadata.db"
	}

	config["metadata_store"] = metadataStore

	return config, envVars, nil
}

// applyResources adds resource registrations to config
func (g *Generator) applyResources(config map[string]interface{}, resources *llamav1alpha1.ResourcesSpec, providers *llamav1alpha1.ProvidersSpec) (map[string]interface{}, error) {
	if resources == nil {
		return config, nil
	}

	registeredResources := make(map[string]interface{})

	// Process models
	if len(resources.Models) > 0 {
		var models []map[string]interface{}
		defaultProvider := g.getFirstInferenceProvider(providers)

		for _, model := range resources.Models {
			modelEntry := map[string]interface{}{
				"identifier": model.Name,
			}

			// Use specified provider or default to first inference provider
			if model.Provider != "" {
				modelEntry["provider_id"] = model.Provider
			} else if defaultProvider != "" {
				modelEntry["provider_id"] = defaultProvider
			}

			// Add metadata if present
			if model.Metadata != nil {
				metadata := make(map[string]interface{})
				if model.Metadata.ContextLength > 0 {
					metadata["context_length"] = model.Metadata.ContextLength
				}
				if model.Metadata.EmbeddingDimension > 0 {
					metadata["embedding_dimension"] = model.Metadata.EmbeddingDimension
				}
				if len(metadata) > 0 {
					modelEntry["metadata"] = metadata
				}
			}

			models = append(models, modelEntry)
		}
		registeredResources["models"] = models
	}

	// Process tools (tool_groups)
	if len(resources.Tools) > 0 {
		var toolGroups []map[string]interface{}
		for _, tool := range resources.Tools {
			toolGroups = append(toolGroups, map[string]interface{}{
				"identifier": tool,
			})
		}
		registeredResources["tool_groups"] = toolGroups
	}

	// Process shields
	if len(resources.Shields) > 0 {
		var shields []map[string]interface{}
		for _, shield := range resources.Shields {
			shields = append(shields, map[string]interface{}{
				"identifier": shield,
			})
		}
		registeredResources["shields"] = shields
	}

	if len(registeredResources) > 0 {
		config["registered_resources"] = registeredResources
	}

	return config, nil
}

// getFirstInferenceProvider returns the ID of the first configured inference provider
func (g *Generator) getFirstInferenceProvider(providers *llamav1alpha1.ProvidersSpec) string {
	if providers == nil || providers.Inference == nil {
		return ""
	}

	entries := providers.Inference.GetEntries()
	if len(entries) == 0 {
		return ""
	}

	// Return explicit ID or provider name
	if entries[0].ID != "" {
		return entries[0].ID
	}
	return entries[0].Provider
}

// applyServerSettings applies port and TLS configuration
func (g *Generator) applyServerSettings(config map[string]interface{}, port int32, tls *llamav1alpha1.ServerTLSConfig) map[string]interface{} {
	if port > 0 {
		config["port"] = port
	}

	if tls != nil && tls.Enabled {
		tlsConfig := map[string]interface{}{
			"enabled": true,
		}
		if tls.SecretName != "" {
			tlsConfig["secret_name"] = tls.SecretName
		}
		config["tls"] = tlsConfig
	}

	return config
}

// camelToSnake converts camelCase to snake_case
func camelToSnake(s string) string {
	var result []byte
	for i, c := range s {
		if c >= 'A' && c <= 'Z' {
			if i > 0 {
				result = append(result, '_')
			}
			result = append(result, byte(c+'a'-'A'))
		} else {
			result = append(result, byte(c))
		}
	}
	return string(result)
}

// validateUniqueProviderIDs checks that all provider IDs are unique across all provider types (FR-030)
func (g *Generator) validateUniqueProviderIDs(providerTypes map[string]*llamav1alpha1.ProviderConfigOrList) error {
	seenIDs := make(map[string]string) // id -> provider type where first seen

	for providerType, providerConfig := range providerTypes {
		if providerConfig == nil {
			continue
		}

		entries := providerConfig.GetEntries()
		for i, entry := range entries {
			// Determine the effective ID (explicit or auto-generated)
			effectiveID := entry.ID
			if effectiveID == "" {
				effectiveID = entry.Provider
			}

			if existingType, exists := seenIDs[effectiveID]; exists {
				return fmt.Errorf(
					"spec.server.providers.%s[%d].id: duplicate provider ID '%s' (already used in %s)",
					providerType, i, effectiveID, existingType,
				)
			}
			seenIDs[effectiveID] = providerType
		}
	}

	return nil
}
