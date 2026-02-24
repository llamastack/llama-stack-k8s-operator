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
	"encoding/json"
	"errors"
	"fmt"

	v1alpha2 "github.com/llamastack/llama-stack-k8s-operator/api/v1alpha2"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

// ExpandResources converts the CR ResourcesSpec into config.yaml registered resource
// entries (models, tool_groups, shields). Returns the entries and total count.
func ExpandResources(
	spec *v1alpha2.ResourcesSpec,
	userProviders map[string][]ProviderEntry,
	base *BaseConfig,
) ([]map[string]interface{}, []map[string]interface{}, []map[string]interface{}, int, error) {
	if spec == nil {
		return nil, nil, nil, 0, nil
	}

	totalCount := 0

	// Expand models
	models, err := expandModels(spec.Models, userProviders, base)
	if err != nil {
		return nil, nil, nil, 0, fmt.Errorf("failed to expand resources.models: %w", err)
	}
	totalCount += len(models)

	// Expand tools
	tools, err := expandTools(spec.Tools, userProviders, base)
	if err != nil {
		return nil, nil, nil, 0, fmt.Errorf("failed to expand resources.tools: %w", err)
	}
	totalCount += len(tools)

	// Expand shields
	shields, err := expandShields(spec.Shields, userProviders, base)
	if err != nil {
		return nil, nil, nil, 0, fmt.Errorf("failed to expand resources.shields: %w", err)
	}
	totalCount += len(shields)

	return models, tools, shields, totalCount, nil
}

func expandModels(
	raw []apiextensionsv1.JSON,
	userProviders map[string][]ProviderEntry,
	base *BaseConfig,
) ([]map[string]interface{}, error) {
	if len(raw) == 0 {
		return nil, nil
	}

	defaultProvider := GetDefaultInferenceProvider(userProviders, base)

	var models []map[string]interface{}
	for i, item := range raw {
		mc, err := ParsePolymorphicModel(&item)
		if err != nil {
			return nil, fmt.Errorf("failed to parse model[%d]: %w", i, err)
		}

		provider := mc.Provider
		if provider == "" {
			provider = defaultProvider
		}

		entry := map[string]interface{}{
			"model_id":    mc.Name,
			"provider_id": provider,
		}

		if mc.ContextLength > 0 {
			if entry["provider_model_id"] == nil {
				entry["provider_model_id"] = mc.Name
			}
			entry["metadata"] = map[string]interface{}{
				"context_length": mc.ContextLength,
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

		models = append(models, entry)
	}
	return models, nil
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

func expandShields(
	shields []string,
	userProviders map[string][]ProviderEntry,
	base *BaseConfig,
) ([]map[string]interface{}, error) {
	if len(shields) == 0 {
		return nil, nil
	}

	provider := getDefaultProviderForAPI("safety", userProviders, base)
	if provider == "" {
		return nil, errors.New("failed to expand shields: requires at least one safety provider to be configured")
	}

	var entries []map[string]interface{}
	for _, shield := range shields {
		entries = append(entries, map[string]interface{}{
			"shield_id":   shield,
			"provider_id": provider,
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
	// Check user-specified providers first
	if entries, ok := userProviders[apiType]; ok && len(entries) > 0 {
		return entries[0].ProviderID
	}

	// Fall back to base config providers
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

// ParsePolymorphicModel parses a JSON value that can be either a simple string
// (model name) or a ModelConfig object.
func ParsePolymorphicModel(raw *apiextensionsv1.JSON) (*v1alpha2.ModelConfig, error) {
	if raw == nil || len(raw.Raw) == 0 {
		return nil, errors.New("failed to parse model: empty entry")
	}

	// Try as string first
	var name string
	if err := json.Unmarshal(raw.Raw, &name); err == nil && name != "" {
		return &v1alpha2.ModelConfig{Name: name}, nil
	}

	// Try as object
	var mc v1alpha2.ModelConfig
	if err := json.Unmarshal(raw.Raw, &mc); err != nil {
		return nil, fmt.Errorf("failed to parse model: expected string or object: %w", err)
	}

	if mc.Name == "" {
		return nil, errors.New("failed to parse model: object must have a 'name' field")
	}

	return &mc, nil
}
