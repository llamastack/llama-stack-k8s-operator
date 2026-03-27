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
		mergeProviders(mergedConfig, userProviders)
	}
	if providerCount == 0 {
		providerCount = CountProviders(mergedConfig)
	}

	resourceCount, err := applyResources(spec, mergedConfig, userProviders, base)
	if err != nil {
		return nil, 0, 0, err
	}

	mergedConfig, err = ExpandStorage(spec.Storage, mergedConfig, secretRes.Substitutions)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("failed to expand storage: %w", err)
	}

	applyNetworkingOverrides(spec, mergedConfig)

	return mergedConfig, providerCount, resourceCount, nil
}

// applyResources expands CR resources and merges them into the config.
// Uses merge-by-ID so base config resources not overridden by the CR are preserved.
func applyResources(spec *v1alpha2.LlamaStackDistributionSpec, config *BaseConfig, userProviders map[string][]ProviderEntry, base *BaseConfig) (int, error) {
	models, tools, shields, resourceCount, err := ExpandResources(spec.Resources, userProviders, base)
	if err != nil {
		return 0, fmt.Errorf("failed to expand resources: %w", err)
	}
	if models != nil {
		config.RegisteredModels = mergeResourceMaps(config.RegisteredModels, models, "model_id")
	}
	if tools != nil {
		config.ToolGroups = mergeResourceMaps(config.ToolGroups, tools, "toolgroup_id")
	}
	if shields != nil {
		config.Shields = mergeResourceMaps(config.Shields, shields, "shield_id")
	}
	return resourceCount, nil
}

// mergeResourceMaps merges incoming typed resources into base by ID key.
// Matching IDs are replaced; unmatched incoming entries are appended;
// base entries not present in incoming are preserved.
func mergeResourceMaps(base, incoming []map[string]interface{}, idKey string) []map[string]interface{} {
	if len(base) == 0 {
		return incoming
	}
	result := make([]map[string]interface{}, 0, len(base)+len(incoming))
	result = append(result, base...)
	for _, entry := range incoming {
		if idx := findResourceIndex(result, idKey, entry); idx >= 0 {
			result[idx] = entry
		} else {
			result = append(result, entry)
		}
	}
	return result
}

// findResourceIndex returns the index of the entry in resources whose idKey
// matches the incoming entry's idKey, or -1 if not found.
func findResourceIndex(resources []map[string]interface{}, idKey string, entry map[string]interface{}) int {
	entryID, _ := entry[idKey].(string)
	if entryID == "" {
		return -1
	}
	for i, r := range resources {
		if r[idKey] == entryID {
			return i
		}
	}
	return -1
}

// applyNetworkingOverrides applies disabled APIs and port overrides from the CR spec.
func applyNetworkingOverrides(spec *v1alpha2.LlamaStackDistributionSpec, config *BaseConfig) {
	if len(spec.Disabled) > 0 {
		ApplyDisabledAPIs(config, spec.Disabled)
	}
	if spec.Networking != nil && spec.Networking.Port > 0 {
		overrideServerPort(config, spec.Networking.Port)
	}
}

// overrideServerPort sets server.port so the llama-stack server listens on the
// port specified in the CR.
func overrideServerPort(config *BaseConfig, port int32) {
	if config.Server == nil {
		config.Server = make(map[string]interface{})
	}
	config.Server["port"] = port
}

// mergeProviders merges user-specified providers into the base config by
// provider_id. Matching IDs are replaced; unmatched user providers are appended.
// Base config providers with IDs not specified by the user are preserved.
// The base config is mutated in place.
func mergeProviders(base *BaseConfig, userProviders map[string][]ProviderEntry) {
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
}

// CountProviders counts the total number of providers in the config.
func CountProviders(config *BaseConfig) int {
	count := 0
	for _, v := range config.Providers {
		if list, ok := v.([]interface{}); ok {
			count += len(list)
		}
	}
	return count
}

// ApplyDisabledAPIs removes disabled API types from the config.
// The disabled slice uses config-style snake_case names (enforced by CRD enum).
// Also cleans up registered resources that reference providers from disabled APIs
// to prevent startup failures from dangling references.
// The config is mutated in place.
func ApplyDisabledAPIs(config *BaseConfig, disabled []string) {
	disabledSet := make(map[string]bool, len(disabled))
	for _, api := range disabled {
		disabledSet[api] = true
	}

	disabledProviderIDs := collectDisabledProviderIDs(config, disabledSet)

	var filteredAPIs []string
	for _, api := range config.APIs {
		if !disabledSet[api] {
			filteredAPIs = append(filteredAPIs, api)
		}
	}
	config.APIs = filteredAPIs

	for api := range disabledSet {
		delete(config.Providers, api)
	}

	cleanupDisabledResources(config, disabledSet, disabledProviderIDs)
	cleanupExtraRegisteredResources(config, disabledSet, disabledProviderIDs)
}

// collectDisabledProviderIDs gathers provider_id values from providers that
// belong to disabled API types, so downstream cleanup can remove dangling refs.
func collectDisabledProviderIDs(config *BaseConfig, disabledSet map[string]bool) map[string]bool {
	ids := make(map[string]bool)
	for api := range disabledSet {
		provList, ok := config.Providers[api]
		if !ok {
			continue
		}
		list, ok := provList.([]interface{})
		if !ok {
			continue
		}
		for _, p := range list {
			if m, ok := p.(map[string]interface{}); ok {
				if id, ok := m["provider_id"].(string); ok {
					ids[id] = true
				}
			}
		}
	}
	return ids
}

// cleanupDisabledResources nils out top-level resource slices whose API type
// has been disabled and filters models whose provider_id is disabled.
func cleanupDisabledResources(config *BaseConfig, disabledSet map[string]bool, disabledProviderIDs map[string]bool) {
	if disabledSet["safety"] {
		config.Shields = nil
	}
	if disabledSet["tool_runtime"] {
		config.ToolGroups = nil
	}
	if len(disabledProviderIDs) > 0 {
		config.RegisteredModels = filterResourcesByProvider(config.RegisteredModels, "provider_id", disabledProviderIDs)
	}
}

// filterResourcesByProvider removes entries whose provider_id is in the disabled set.
func filterResourcesByProvider(resources []map[string]interface{}, providerKey string, disabledIDs map[string]bool) []map[string]interface{} {
	if len(resources) == 0 {
		return resources
	}
	var filtered []map[string]interface{}
	for _, r := range resources {
		if pid, ok := r[providerKey].(string); ok && disabledIDs[pid] {
			continue
		}
		filtered = append(filtered, r)
	}
	return filtered
}

func cleanupExtraRegisteredResources(config *BaseConfig, disabledSet map[string]bool, disabledProviderIDs map[string]bool) {
	if config.Extra == nil {
		return
	}
	rr, ok := config.Extra["registered_resources"].(map[string]interface{})
	if !ok {
		return
	}

	if disabledSet["safety"] {
		delete(rr, "shields")
	}
	if disabledSet["tool_runtime"] {
		delete(rr, "tool_groups")
	}
	if models, ok := rr["models"].([]interface{}); ok && len(disabledProviderIDs) > 0 {
		rr["models"] = filterExtraModelsByProvider(models, disabledProviderIDs)
	}
}

// filterExtraModelsByProvider removes model entries from Extra.registered_resources
// whose provider_id is in the disabled set.
func filterExtraModelsByProvider(models []interface{}, disabledIDs map[string]bool) []interface{} {
	var filtered []interface{}
	for _, m := range models {
		if entry, ok := m.(map[string]interface{}); ok {
			if pid, ok := entry["provider_id"].(string); ok && disabledIDs[pid] {
				continue
			}
		}
		filtered = append(filtered, m)
	}
	return filtered
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
// It reads from config but never mutates it, so RenderConfigYAML is side-effect-free.
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
		provMap := make(map[string]interface{}, len(config.Providers))
		for k, v := range config.Providers {
			provMap[k] = v
		}
		out["providers"] = provMap
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

	// Emit all Extra fields that were captured via the inline YAML tag
	// (e.g. distro_name, image_name, storage, registered_resources,
	// vector_stores, safety, connectors). Skip keys already set above.
	for k, v := range config.Extra {
		if _, exists := out[k]; !exists {
			out[k] = v
		}
	}

	// Merge CR resources into registered_resources in the output map
	// (not in config.Extra) to keep rendering side-effect-free.
	writeResourcesToOutput(out, config)

	return out
}

// writeResourcesToOutput merges RegisteredModels, Shields, and ToolGroups from
// the config struct into out["registered_resources"]. A deep copy of the
// existing registered_resources is used so config.Extra is never mutated.
func writeResourcesToOutput(out map[string]interface{}, config *BaseConfig) {
	if len(config.RegisteredModels) == 0 && len(config.Shields) == 0 && len(config.ToolGroups) == 0 {
		return
	}

	var rr map[string]interface{}
	if existing, ok := out["registered_resources"].(map[string]interface{}); ok {
		rr = copyMap(existing)
	} else {
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

	out["registered_resources"] = rr
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
