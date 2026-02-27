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
	"strings"

	v1alpha2 "github.com/llamastack/llama-stack-k8s-operator/api/v1alpha2"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

// apiTypeNames maps ProvidersSpec field names to config.yaml provider section keys.
var apiTypeNames = map[string]string{
	"inference":   "inference",
	"safety":      "safety",
	"vectorIo":    "vector_io",
	"toolRuntime": "tool_runtime",
	"telemetry":   "telemetry",
}

// ExpandProviders expands the CR ProvidersSpec into config.yaml-format provider entries
// keyed by API type. It replaces the base config's providers for each API type that
// the user specifies.
func ExpandProviders(spec *v1alpha2.ProvidersSpec, substitutions map[string]string) (map[string][]ProviderEntry, int, error) {
	if spec == nil {
		return nil, 0, nil
	}

	result := make(map[string][]ProviderEntry)
	totalCount := 0

	fields := []struct {
		fieldName string
		raw       *apiextensionsv1.JSON
	}{
		{"inference", spec.Inference},
		{"safety", spec.Safety},
		{"vectorIo", spec.VectorIo},
		{"toolRuntime", spec.ToolRuntime},
		{"telemetry", spec.Telemetry},
	}

	for _, f := range fields {
		if f.raw == nil {
			continue
		}

		configs, err := ParsePolymorphicProvider(f.raw)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to parse providers.%s: %w", f.fieldName, err)
		}

		configKey := apiTypeNames[f.fieldName]
		entries := make([]ProviderEntry, 0, len(configs))

		for _, pc := range configs {
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

func mergeProviderSettings(pc v1alpha2.ProviderConfig, providerID string, substitutions map[string]string) (map[string]interface{}, error) {
	if pc.Settings == nil || len(pc.Settings.Raw) == 0 {
		return nil, nil
	}
	var settings map[string]interface{}
	if err := json.Unmarshal(pc.Settings.Raw, &settings); err != nil {
		return nil, fmt.Errorf("failed to unmarshal settings for provider %q: %w", providerID, err)
	}
	out := make(map[string]interface{}, len(settings))
	for k, v := range settings {
		if m, ok := v.(map[string]interface{}); ok {
			if ref := extractSecretRef(m); ref != nil {
				ident := providerID + ":" + k
				if sub, ok2 := substitutions[ident]; ok2 {
					out[k] = sub
					continue
				}
			}
		}
		out[k] = v
	}
	return out, nil
}

// expandSingleProvider converts a single ProviderConfig to a ProviderEntry.
func expandSingleProvider(pc v1alpha2.ProviderConfig, substitutions map[string]string) (ProviderEntry, error) {
	providerID := pc.ID
	if providerID == "" {
		providerID = GenerateProviderID(pc.Provider)
	}

	providerType := NormalizeProviderType(pc.Provider)

	cfg := make(map[string]interface{})

	// Map endpoint -> config.url
	if pc.Endpoint != "" {
		cfg["url"] = pc.Endpoint
	}

	// Map apiKey -> config.api_key via env var substitution
	if pc.ApiKey != nil {
		ident := providerID + ":apiKey"
		if sub, ok := substitutions[ident]; ok {
			cfg["api_key"] = sub
		} else {
			envName := GenerateEnvVarName(providerID, "apiKey")
			cfg["api_key"] = "${env." + envName + "}"
		}
	}

	// Merge settings into config
	settingsMap, err := mergeProviderSettings(pc, providerID, substitutions)
	if err != nil {
		return ProviderEntry{}, err
	}
	for k, v := range settingsMap {
		cfg[k] = v
	}

	return ProviderEntry{
		ProviderID:   providerID,
		ProviderType: providerType,
		Config:       cfg,
	}, nil
}

// NormalizeProviderType adds the "remote::" prefix if not already present.
// Providers that already have a "::" qualifier (e.g., "inline::meta-reference") are left as-is.
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

// ParsePolymorphicProvider parses a JSON value that can be either a single
// ProviderConfig object or an array of ProviderConfig objects.
func ParsePolymorphicProvider(raw *apiextensionsv1.JSON) ([]v1alpha2.ProviderConfig, error) {
	if raw == nil || len(raw.Raw) == 0 {
		return nil, nil
	}

	// Try single object first
	var single v1alpha2.ProviderConfig
	if err := json.Unmarshal(raw.Raw, &single); err == nil && single.Provider != "" {
		return []v1alpha2.ProviderConfig{single}, nil
	}

	// Try array
	var list []v1alpha2.ProviderConfig
	if err := json.Unmarshal(raw.Raw, &list); err != nil {
		return nil, fmt.Errorf("failed to parse provider: expected object or array: %w", err)
	}

	// Validate: when list form, each item must have an explicit ID
	if len(list) > 1 {
		for i, pc := range list {
			if pc.ID == "" {
				return nil, fmt.Errorf("failed to validate provider at index %d: must have an explicit 'id' when multiple providers are specified", i)
			}
		}
	}

	return list, nil
}
