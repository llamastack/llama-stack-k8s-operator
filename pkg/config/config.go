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

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"

	v1alpha2 "github.com/llamastack/llama-stack-k8s-operator/api/v1alpha2"
	"gopkg.in/yaml.v3"
)

// GenerateConfig orchestrates the full config generation pipeline:
// 1. Resolve base config from distribution
// 2. Resolve secrets to env vars and substitutions
// 3. Expand user providers over base
// 4. Expand resources
// 5. Apply storage overrides
// 6. Apply disabled APIs
// 7. Render final YAML
func GenerateConfig(
	ctx context.Context,
	spec *v1alpha2.LlamaStackDistributionSpec,
	resolver *BaseConfigResolver,
) (*GeneratedConfig, string, error) {
	// Step 1: Resolve base config
	base, resolvedImage, err := resolver.Resolve(ctx, spec.Distribution)
	if err != nil {
		return nil, "", fmt.Errorf("resolving base config: %w", err)
	}

	// Detect and validate config version
	version := base.Version
	if err := ValidateConfigVersion(version); err != nil {
		return nil, "", fmt.Errorf("base config version: %w", err)
	}

	// Step 2: Resolve secrets
	secretRes, err := ResolveSecrets(spec)
	if err != nil {
		return nil, "", fmt.Errorf("resolving secrets: %w", err)
	}

	// Step 3: Expand user providers
	userProviders, providerCount, err := ExpandProviders(spec.Providers, secretRes.Substitutions)
	if err != nil {
		return nil, "", fmt.Errorf("expanding providers: %w", err)
	}

	// Merge user providers into base config
	mergedConfig := cloneBaseConfig(base)
	if userProviders != nil {
		mergedConfig = mergeProviders(mergedConfig, userProviders)
	}

	// Count base providers if no user providers specified
	if providerCount == 0 {
		providerCount = countBaseProviders(mergedConfig)
	}

	// Step 4: Expand resources
	models, tools, shields, resourceCount, err := ExpandResources(spec.Resources, userProviders, base)
	if err != nil {
		return nil, "", fmt.Errorf("expanding resources: %w", err)
	}

	if models != nil {
		mergedConfig.RegisteredModels = models
	}
	if tools != nil {
		mergedConfig.ToolGroups = tools
	}
	if shields != nil {
		mergedConfig.Shields = shields
	}

	// Step 5: Apply storage overrides
	mergedConfig, err = ExpandStorage(spec.Storage, mergedConfig)
	if err != nil {
		return nil, "", fmt.Errorf("expanding storage: %w", err)
	}

	// Step 6: Apply disabled APIs
	if len(spec.Disabled) > 0 {
		mergedConfig = ApplyDisabledAPIs(mergedConfig, spec.Disabled)
	}

	// Step 7: Render to YAML
	configYAML, err := RenderConfigYAML(mergedConfig)
	if err != nil {
		return nil, "", fmt.Errorf("rendering config YAML: %w", err)
	}

	contentHash := ComputeContentHash(configYAML)

	return &GeneratedConfig{
		ConfigYAML:    configYAML,
		EnvVars:       secretRes.EnvVars,
		ContentHash:   contentHash,
		ProviderCount: providerCount,
		ResourceCount: resourceCount,
		ConfigVersion: version,
	}, resolvedImage, nil
}

// mergeProviders replaces base config providers with user-specified providers
// for each API type that the user has configured.
func mergeProviders(base *BaseConfig, userProviders map[string][]ProviderEntry) *BaseConfig {
	if base.Providers == nil {
		base.Providers = make(map[string]interface{})
	}

	for apiType, entries := range userProviders {
		provList := make([]interface{}, len(entries))
		for i, e := range entries {
			provList[i] = map[string]interface{}{
				"provider_id":   e.ProviderID,
				"provider_type": e.ProviderType,
				"config":        e.Config,
			}
		}
		base.Providers[apiType] = provList
	}

	return base
}

// countBaseProviders counts the total number of providers in the base config.
func countBaseProviders(config *BaseConfig) int {
	count := 0
	for _, v := range config.Providers {
		if list, ok := v.([]interface{}); ok {
			count += len(list)
		}
	}
	return count
}

// ApplyDisabledAPIs removes disabled API types from the config.
func ApplyDisabledAPIs(config *BaseConfig, disabled []string) *BaseConfig {
	disabledSet := make(map[string]bool, len(disabled))
	for _, api := range disabled {
		disabledSet[api] = true
	}

	// Remove disabled APIs from the APIs list
	var filteredAPIs []string
	for _, api := range config.APIs {
		if !disabledSet[api] {
			filteredAPIs = append(filteredAPIs, api)
		}
	}
	config.APIs = filteredAPIs

	// Remove disabled providers
	for api := range disabledSet {
		delete(config.Providers, api)
	}

	return config
}

// RenderConfigYAML serializes the config to deterministic YAML.
func RenderConfigYAML(config *BaseConfig) (string, error) {
	// Build an ordered map for deterministic output
	out := buildOrderedConfig(config)

	data, err := yaml.Marshal(out)
	if err != nil {
		return "", fmt.Errorf("marshaling config YAML: %w", err)
	}

	return string(data), nil
}

// buildOrderedConfig creates a map suitable for deterministic YAML serialization.
func buildOrderedConfig(config *BaseConfig) map[string]interface{} {
	out := make(map[string]interface{})
	out["version"] = config.Version

	if len(config.APIs) > 0 {
		apis := make([]string, len(config.APIs))
		copy(apis, config.APIs)
		sort.Strings(apis)
		out["apis"] = apis
	}

	if len(config.Providers) > 0 {
		provMap := make(map[string]interface{})
		keys := make([]string, 0, len(config.Providers))
		for k := range config.Providers {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			provMap[k] = config.Providers[k]
		}
		out["providers"] = provMap
	}

	if len(config.RegisteredModels) > 0 {
		out["models"] = config.RegisteredModels
	}
	if len(config.Shields) > 0 {
		out["shields"] = config.Shields
	}
	if len(config.ToolGroups) > 0 {
		out["tool_groups"] = config.ToolGroups
	}

	addStoreIfNonNil(out, "metadata_store", config.MetadataStore)
	addStoreIfNonNil(out, "inference_store", config.InferenceStore)
	addStoreIfNonNil(out, "safety_store", config.SafetyStore)
	addStoreIfNonNil(out, "vector_io_store", config.VectorIOStore)
	addStoreIfNonNil(out, "tool_runtime_store", config.ToolRuntimeStore)
	addStoreIfNonNil(out, "telemetry_store", config.TelemetryStore)
	addStoreIfNonNil(out, "post_training_store", config.PostTrainingStore)
	addStoreIfNonNil(out, "scoring_store", config.ScoringStore)
	addStoreIfNonNil(out, "eval_store", config.EvalStore)
	addStoreIfNonNil(out, "datasetio_store", config.DatasetIOStore)
	addStoreIfNonNil(out, "server", config.Server)

	return out
}

func addStoreIfNonNil(out map[string]interface{}, key string, store map[string]interface{}) {
	if store != nil {
		out[key] = store
	}
}

// ComputeContentHash returns the SHA256 hex digest of the config content.
func ComputeContentHash(content string) string {
	h := sha256.Sum256([]byte(content))
	return hex.EncodeToString(h[:])
}

// MergeExternalProviders adds external providers (from spec 001) to the config.
// On ID conflict, external providers override inline providers.
func MergeExternalProviders(config *BaseConfig, external map[string]interface{}) (*BaseConfig, []string) {
	if len(external) == 0 {
		return config, nil
	}

	var warnings []string

	for apiType, provs := range external {
		extList, ok := provs.([]interface{})
		if !ok {
			continue
		}

		existing := config.Providers[apiType]
		existingList, _ := existing.([]interface{})

		existingIDs := make(map[string]int)
		for i, p := range existingList {
			if m, ok := p.(map[string]interface{}); ok {
				if id, ok := m["provider_id"].(string); ok {
					existingIDs[id] = i
				}
			}
		}

		for _, ep := range extList {
			if m, ok := ep.(map[string]interface{}); ok {
				if id, ok := m["provider_id"].(string); ok {
					if idx, exists := existingIDs[id]; exists {
						warnings = append(warnings, fmt.Sprintf(
							"external provider %q for API %q overrides inline provider with same ID",
							id, apiType,
						))
						existingList[idx] = ep
						continue
					}
				}
			}
			existingList = append(existingList, ep)
		}

		config.Providers[apiType] = existingList
	}

	return config, warnings
}
