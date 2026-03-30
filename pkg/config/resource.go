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
	"errors"
	"fmt"

	v1alpha2 "github.com/llamastack/llama-stack-k8s-operator/api/v1alpha2"
)

// ExpandResources converts the CR ResourcesSpec into config.yaml registered resource
// entries (models, tool_groups). Returns the entries and total count.
func ExpandResources(
	spec *v1alpha2.ResourcesSpec,
	userProviders map[string][]ProviderEntry,
	base *BaseConfig,
) ([]map[string]interface{}, []map[string]interface{}, int, error) {
	if spec == nil {
		return nil, nil, 0, nil
	}

	totalCount := 0

	models, err := expandModels(spec.Models, userProviders, base)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("failed to expand resources.models: %w", err)
	}
	totalCount += len(models)

	tools, err := expandTools(spec.Tools, userProviders, base)
	if err != nil {
		return nil, nil, 0, fmt.Errorf("failed to expand resources.tools: %w", err)
	}
	totalCount += len(tools)

	return models, tools, totalCount, nil
}

func expandModels(
	models []v1alpha2.ModelConfig,
	userProviders map[string][]ProviderEntry,
	base *BaseConfig,
) ([]map[string]interface{}, error) {
	if len(models) == 0 {
		return nil, nil
	}

	defaultProvider := GetDefaultInferenceProvider(userProviders, base)

	var result []map[string]interface{}
	for _, mc := range models {
		if mc.Name == "" {
			return nil, errors.New("model must have a 'name' field")
		}

		provider := mc.Provider
		if provider == "" {
			provider = defaultProvider
		}
		if provider == "" {
			return nil, fmt.Errorf("failed to expand model %q: no provider specified and no default inference provider found", mc.Name)
		}
		if mc.Provider != "" && !providerExists(provider, userProviders, base) {
			return nil, fmt.Errorf("failed to expand model %q: references unknown provider %q", mc.Name, provider)
		}

		result = append(result, buildModelEntry(mc, provider))
	}
	return result, nil
}

func buildModelEntry(mc v1alpha2.ModelConfig, provider string) map[string]interface{} {
	entry := map[string]interface{}{
		"model_id":    mc.Name,
		"provider_id": provider,
	}
	if mc.ContextLength != nil && *mc.ContextLength > 0 {
		entry["provider_model_id"] = mc.Name
		entry["metadata"] = map[string]interface{}{
			"context_length": *mc.ContextLength,
		}
	}
	if mc.ModelType != "" {
		meta := getOrCreateMeta(entry)
		meta["model_type"] = mc.ModelType
		entry["metadata"] = meta
	}
	if mc.Quantization != "" {
		meta := getOrCreateMeta(entry)
		meta["quantization"] = mc.Quantization
		entry["metadata"] = meta
	}
	return entry
}

func getOrCreateMeta(entry map[string]interface{}) map[string]interface{} {
	if m, ok := entry["metadata"].(map[string]interface{}); ok {
		return m
	}
	return make(map[string]interface{})
}

func expandTools(
	tools []string,
	userProviders map[string][]ProviderEntry,
	base *BaseConfig,
) ([]map[string]interface{}, error) {
	if len(tools) == 0 {
		return nil, nil
	}

	provider := getDefaultProviderForAPI("tool_runtime", userProviders, base)
	if provider == "" {
		return nil, errors.New("failed to expand tools: requires at least one toolRuntime provider to be configured")
	}

	var entries []map[string]interface{}
	for _, tool := range tools {
		entries = append(entries, map[string]interface{}{
			"toolgroup_id": tool,
			"provider_id":  provider,
		})
	}
	return entries, nil
}

// GetDefaultInferenceProvider returns the provider_id of the first inference provider.
// Checks user providers first, then falls back to base config.
func GetDefaultInferenceProvider(
	userProviders map[string][]ProviderEntry,
	base *BaseConfig,
) string {
	return getDefaultProviderForAPI("inference", userProviders, base)
}

func getDefaultProviderForAPI(
	apiType string,
	userProviders map[string][]ProviderEntry,
	base *BaseConfig,
) string {
	if entries, ok := userProviders[apiType]; ok && len(entries) > 0 {
		return entries[0].ProviderID
	}

	if base != nil && base.Providers != nil {
		if provList, ok := base.Providers[apiType]; ok {
			if list, ok := provList.([]interface{}); ok && len(list) > 0 {
				if m, ok := list[0].(map[string]interface{}); ok {
					if id, ok := m["provider_id"].(string); ok {
						return id
					}
				}
			}
		}
	}

	return ""
}

// providerExists checks whether a provider ID is defined in either
// user-specified providers or the base config's provider list.
func providerExists(id string, userProviders map[string][]ProviderEntry, base *BaseConfig) bool {
	for _, entries := range userProviders {
		for _, e := range entries {
			if e.ProviderID == id {
				return true
			}
		}
	}
	return providerExistsInBase(id, base)
}

func providerExistsInBase(id string, base *BaseConfig) bool {
	if base == nil || base.Providers == nil {
		return false
	}
	for _, v := range base.Providers {
		list, ok := v.([]interface{})
		if !ok {
			continue
		}
		for _, p := range list {
			m, ok := p.(map[string]interface{})
			if !ok {
				continue
			}
			if pid, ok := m["provider_id"].(string); ok && pid == id {
				return true
			}
		}
	}
	return false
}
