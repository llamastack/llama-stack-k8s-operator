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
	"fmt"
	"sort"
	"strings"

	v1alpha2 "github.com/llamastack/llama-stack-k8s-operator/api/v1alpha2"
)

// apiTypeNames maps ProvidersSpec field names to config.yaml provider section keys.
// Only 5 API types are exposed as CRD provider fields. Other API types
// (agents, datasetio, eval, scoring) are intentionally excluded from the
// CRD schema — they can only be configured via base config defaults or
// overrideConfig.
var apiTypeNames = map[string]string{
	"inference":   "inference",
	"safety":      "safety",
	"vectorIo":    "vector_io",
	"toolRuntime": "tool_runtime",
	"telemetry":   "telemetry",
}

// ExpandProviders expands the CR ProvidersSpec into config.yaml-format provider entries
// keyed by API type.
func ExpandProviders(spec *v1alpha2.ProvidersSpec, substitutions map[string]string) (map[string][]ProviderEntry, int, error) {
	if spec == nil {
		return nil, 0, nil
	}

	result := make(map[string][]ProviderEntry)
	totalCount := 0

	fields := []struct {
		fieldName string
		configs   []v1alpha2.ProviderConfig
	}{
		{"inference", spec.Inference},
		{"safety", spec.Safety},
		{"vectorIo", spec.VectorIo},
		{"toolRuntime", spec.ToolRuntime},
		{"telemetry", spec.Telemetry},
	}

	for _, f := range fields {
		if len(f.configs) == 0 {
			continue
		}

		configKey := apiTypeNames[f.fieldName]
		entries := make([]ProviderEntry, 0, len(f.configs))

		for _, pc := range f.configs {
			entry, err := expandSingleProvider(pc, substitutions)
			if err != nil {
				return nil, 0, fmt.Errorf("failed to parse providers.%s: %w", f.fieldName, err)
			}
			entries = append(entries, entry)
		}

		result[configKey] = entries
		totalCount += len(entries)
	}

	return result, totalCount, nil
}

// mergeProviderSettings unmarshals the settings JSON and returns it as a map.
// Settings are passed through as-is without any secret resolution (FR-005).
func mergeProviderSettings(pc v1alpha2.ProviderConfig, providerID string) (map[string]interface{}, error) {
	if pc.Settings == nil || len(pc.Settings.Raw) == 0 {
		return nil, nil
	}
	var settings map[string]interface{}
	if err := json.Unmarshal(pc.Settings.Raw, &settings); err != nil {
		return nil, fmt.Errorf("failed to unmarshal settings for provider %q: %w", providerID, err)
	}
	return settings, nil
}

// expandSingleProvider converts a single ProviderConfig to a ProviderEntry.
// Settings are applied first so that structured CRD fields (endpoint,
// secretRefs) always take precedence over free-form settings keys.
func expandSingleProvider(pc v1alpha2.ProviderConfig, substitutions map[string]string) (ProviderEntry, error) {
	providerID := pc.ID
	if providerID == "" {
		providerID = GenerateProviderID(pc.Provider)
	}

	providerType := NormalizeProviderType(pc.Provider)

	cfg := make(map[string]interface{})

	settingsMap, err := mergeProviderSettings(pc, providerID)
	if err != nil {
		return ProviderEntry{}, err
	}
	for k, v := range settingsMap {
		cfg[k] = v
	}

	if pc.Endpoint != "" {
		cfg["base_url"] = pc.Endpoint
	}

	secretKeys := make([]string, 0, len(pc.SecretRefs))
	for key := range pc.SecretRefs {
		secretKeys = append(secretKeys, key)
	}
	sort.Strings(secretKeys)

	for _, key := range secretKeys {
		ident := providerID + ":" + key
		if sub, ok := substitutions[ident]; ok {
			cfg[key] = sub
		} else {
			envName := GenerateEnvVarName(providerID, key)
			cfg[key] = "${env." + envName + "}"
		}
	}

	return ProviderEntry{
		ProviderID:   providerID,
		ProviderType: providerType,
		Config:       cfg,
	}, nil
}

// NormalizeProviderType adds the "remote::" prefix if not already present.
// Providers that already have a "::" qualifier (e.g., "inline::meta-reference")
// are left as-is. Users can explicitly use "inline::meta-reference" to skip
// the automatic "remote::" prefix.
func NormalizeProviderType(provider string) string {
	if strings.Contains(provider, "::") {
		return provider
	}
	return "remote::" + provider
}

// GenerateProviderID generates a provider_id from the provider type.
// For "vllm" -> "vllm", for "remote::vllm" -> "vllm".
func GenerateProviderID(providerType string) string {
	if idx := strings.LastIndex(providerType, "::"); idx >= 0 {
		return providerType[idx+2:]
	}
	return providerType
}
