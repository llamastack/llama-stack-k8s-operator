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
// 7. Render final YAML.
func GenerateConfig(
	ctx context.Context,
	spec *v1alpha2.LlamaStackDistributionSpec,
	resolver *BaseConfigResolver,
) (*GeneratedConfig, string, error) {
	// Step 1: Resolve base config
	base, resolvedImage, err := resolver.Resolve(ctx, spec.Distribution)
	if err != nil {
		return nil, "", fmt.Errorf("failed to resolve base config: %w", err)
	}

	// Detect and validate config version
	version := base.Version
	if vErr := ValidateConfigVersion(version); vErr != nil {
		return nil, "", fmt.Errorf("failed to validate base config version: %w", vErr)
	}

	// Step 2: Resolve secrets
	secretRes, err := ResolveSecrets(spec)
	if err != nil {
		return nil, "", fmt.Errorf("failed to resolve secrets: %w", err)
	}

	mergedConfig, providerCount, resourceCount, err := expandSpecOverBase(spec, base, secretRes)
	if err != nil {
		return nil, "", err
	}

	// Step 7: Render to YAML
	configYAML, err := RenderConfigYAML(mergedConfig)
	if err != nil {
		return nil, "", fmt.Errorf("failed to render config YAML: %w", err)
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

func expandSpecOverBase(spec *v1alpha2.LlamaStackDistributionSpec, base *BaseConfig, secretRes *SecretResolution) (*BaseConfig, int, int, error) {
	userProviders, providerCount, err := ExpandProviders(spec.Providers, secretRes.Substitutions)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("failed to expand providers: %w", err)
	}

	mergedConfig := cloneBaseConfig(base)
	if userProviders != nil {
		mergedConfig = mergeProviders(mergedConfig, userProviders)
	}
	if providerCount == 0 {
		providerCount = countBaseProviders(mergedConfig)
	}

	resourceCount, err := applyResources(spec, mergedConfig, userProviders, base)
	if err != nil {
		return nil, 0, 0, err
	}

	mergedConfig, err = ExpandStorage(spec.Storage, mergedConfig)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("failed to expand storage: %w", err)
	}

	applyNetworkingOverrides(spec, mergedConfig)

	return mergedConfig, providerCount, resourceCount, nil
}

// applyResources expands CR resources and merges them into the config.
func applyResources(spec *v1alpha2.LlamaStackDistributionSpec, config *BaseConfig, userProviders map[string][]ProviderEntry, base *BaseConfig) (int, error) {
	models, tools, shields, resourceCount, err := ExpandResources(spec.Resources, userProviders, base)
	if err != nil {
		return 0, fmt.Errorf("failed to expand resources: %w", err)
	}
	if models != nil {
		config.RegisteredModels = models
	}
	if tools != nil {
		config.ToolGroups = tools
	}
	if shields != nil {
		config.Shields = shields
	}
	return resourceCount, nil
}

// applyNetworkingOverrides applies disabled APIs and port overrides from the CR spec.
func applyNetworkingOverrides(spec *v1alpha2.LlamaStackDistributionSpec, config *BaseConfig) {
	if len(spec.Disabled) > 0 {
		_ = ApplyDisabledAPIs(config, spec.Disabled)
	}
	if spec.Networking != nil && spec.Networking.Port > 0 {
		overrideServerPort(config, int(spec.Networking.Port))
	}
}

// overrideServerPort sets server.port in the config's Extra map so the
// llama-stack server listens on the port specified in the CR.
func overrideServerPort(config *BaseConfig, port int) {
	if config.Extra == nil {
		config.Extra = make(map[string]interface{})
	}
	serverSection, ok := config.Extra["server"].(map[string]interface{})
	if !ok {
		serverSection = make(map[string]interface{})
	}
	serverSection["port"] = port
	config.Extra["server"] = serverSection
}

// mergeProviders merges user-specified providers into the base config by
// provider_id. Matching IDs are replaced; unmatched user providers are appended.
// Base config providers with IDs not specified by the user are preserved.
func mergeProviders(base *BaseConfig, userProviders map[string][]ProviderEntry) *BaseConfig {
	if base.Providers == nil {
		base.Providers = make(map[string]interface{})
	}

	for apiType, entries := range userProviders {
		baseList, _ := base.Providers[apiType].([]interface{})

		for _, e := range entries {
			userEntry := map[string]interface{}{
				"provider_id":   e.ProviderID,
				"provider_type": e.ProviderType,
				"config":        e.Config,
			}

			replaced := false
			for i, bp := range baseList {
				if m, ok := bp.(map[string]interface{}); ok {
					if m["provider_id"] == e.ProviderID {
						baseList[i] = userEntry
						replaced = true
						break
					}
				}
			}

			if !replaced {
				baseList = append(baseList, userEntry)
			}
		}

		base.Providers[apiType] = baseList
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
		return "", fmt.Errorf("failed to marshal config YAML: %w", err)
	}

	return string(data), nil
}

// buildOrderedConfig creates a map suitable for deterministic YAML serialization.
func buildOrderedConfig(config *BaseConfig) map[string]interface{} {
	out := make(map[string]interface{})
	out["version"] = config.Version

	// Explicit struct fields take precedence over Extra; write them first so
	// the Extra loop (below) skips keys that are already set.

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

	mergeResourcesIntoExtra(config)

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

	// Emit all Extra fields that were captured via the inline YAML tag
	// (e.g. distro_name, image_name, storage, registered_resources,
	// vector_stores, safety, connectors). Skip keys already set above.
	for k, v := range config.Extra {
		if _, exists := out[k]; !exists {
			out[k] = v
		}
	}

	return out
}

// mergeResourcesIntoExtra merges RegisteredModels, Shields, and ToolGroups
// from the struct fields into Extra["registered_resources"] so they appear
// under the correct YAML key for the llama-stack server (StackConfig).
// Resources are merged by their ID field (model_id, shield_id, toolgroup_id):
// matching IDs are replaced, unmatched entries are appended, and base config
// entries not specified by the CR are preserved.
func mergeResourcesIntoExtra(config *BaseConfig) {
	if len(config.RegisteredModels) == 0 && len(config.Shields) == 0 && len(config.ToolGroups) == 0 {
		return
	}

	if config.Extra == nil {
		config.Extra = make(map[string]interface{})
	}

	rr, ok := config.Extra["registered_resources"].(map[string]interface{})
	if !ok {
		rr = make(map[string]interface{})
	}

	if len(config.RegisteredModels) > 0 {
		rr["models"] = mergeResourceList(rr["models"], config.RegisteredModels, "model_id")
	}
	if len(config.Shields) > 0 {
		rr["shields"] = mergeResourceList(rr["shields"], config.Shields, "shield_id")
	}
	if len(config.ToolGroups) > 0 {
		rr["tool_groups"] = mergeResourceList(rr["tool_groups"], config.ToolGroups, "toolgroup_id")
	}

	config.Extra["registered_resources"] = rr
}

// mergeResourceList merges new entries into an existing list by idKey.
// Entries with matching IDs are replaced; unmatched entries are appended.
func mergeResourceList(existing interface{}, incoming []map[string]interface{}, idKey string) []interface{} {
	var base []interface{}
	if list, ok := existing.([]interface{}); ok {
		base = append(base, list...)
	}

	for _, entry := range incoming {
		entryID, _ := entry[idKey].(string)
		replaced := false
		if entryID != "" {
			for i, b := range base {
				if m, ok := b.(map[string]interface{}); ok && m[idKey] == entryID {
					base[i] = entry
					replaced = true
					break
				}
			}
		}
		if !replaced {
			base = append(base, entry)
		}
	}
	return base
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

	if config.Providers == nil {
		config.Providers = make(map[string]interface{})
	}

	var warnings []string

	for apiType, provs := range external {
		extList, ok := provs.([]interface{})
		if !ok {
			continue
		}

		existing := config.Providers[apiType]
		mergedList, apiWarnings := mergeExternalAPIProviders(existing, extList, apiType)
		config.Providers[apiType] = mergedList
		warnings = append(warnings, apiWarnings...)
	}

	return config, warnings
}

func mergeExternalAPIProviders(existing interface{}, extList []interface{}, apiType string) ([]interface{}, []string) {
	existingList, _ := existing.([]interface{})

	existingIDs := make(map[string]int)
	for i, p := range existingList {
		if m, ok := p.(map[string]interface{}); ok {
			if id, ok := m["provider_id"].(string); ok {
				existingIDs[id] = i
			}
		}
	}

	var warnings []string
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

	return existingList, warnings
}
