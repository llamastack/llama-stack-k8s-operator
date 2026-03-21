package config

import (
	"encoding/json"
	"fmt"
	"sort"

	v1alpha2 "github.com/llamastack/llama-stack-k8s-operator/api/v1alpha2"
)

// configProvider represents a provider entry in config.yaml.
type configProvider struct {
	ProviderID   string                 `json:"provider_id"`
	ProviderType string                 `json:"provider_type"`
	Config       map[string]interface{} `json:"config"`
}

// ExpandProviders converts CRD ProviderConfig entries into config.yaml provider entries.
// It handles auto-ID generation, remote:: prefix, endpoint mapping, and settings merge.
//
// Note: The CRD only supports configuring remote:: providers. Inline providers
// (inline::) from the base config are preserved during merge but cannot be
// added or configured through the CRD's providers spec.
func ExpandProviders(providers []v1alpha2.ProviderConfig) ([]configProvider, error) {
	result := make([]configProvider, 0, len(providers))

	for _, p := range providers {
		cp, err := expandSingleProvider(p, len(providers) == 1)
		if err != nil {
			return nil, err
		}
		result = append(result, cp)
	}

	return result, nil
}

func expandSingleProvider(p v1alpha2.ProviderConfig, isSingle bool) (configProvider, error) {
	id := p.ID
	if id == "" {
		if !isSingle {
			return configProvider{}, fmt.Errorf("failed to expand provider %q: missing required 'id' field in multi-provider list", p.Provider)
		}
		// Auto-generate ID from provider field for single-element lists (FR-035)
		id = p.Provider
	}

	// CRD-configured providers are always remote (inline:: comes from base config only)
	providerType := "remote::" + p.Provider

	cfg := make(map[string]interface{})

	if p.Endpoint != "" {
		cfg["url"] = p.Endpoint
	}

	// Merge settings into config (FR-033)
	if p.Settings != nil {
		var settings map[string]interface{}
		if err := json.Unmarshal(p.Settings.Raw, &settings); err != nil {
			return configProvider{}, fmt.Errorf("failed to parse settings for provider %q: %w", id, err)
		}
		// Merge settings into config, settings take precedence
		for k, v := range settings {
			cfg[k] = v
		}
	}

	// apiKey and secretRefs are handled by CollectSecretRefs, which writes
	// env var references (${env.LLSD_...}) into the config. We add those
	// placeholders here so the config is complete.
	if p.APIKey != nil {
		cfg["api_key"] = fmt.Sprintf("${env.%s}", envVarName(id, "API_KEY"))
	}

	if len(p.SecretRefs) > 0 {
		keys := sortedKeys(p.SecretRefs)
		for _, k := range keys {
			cfg[k] = fmt.Sprintf("${env.%s}", envVarName(id, normalizeEnvVarSegment(k)))
		}
	}

	return configProvider{
		ProviderID:   id,
		ProviderType: providerType,
		Config:       cfg,
	}, nil
}

// AllProviderConfigs returns all provider configs across all API types with their API type labels.
type apiProviders struct {
	APIType   string
	Providers []v1alpha2.ProviderConfig
}

// AllAPIProviders returns all configured API types and their providers in deterministic order.
func AllAPIProviders(providers *v1alpha2.ProvidersSpec) []apiProviders {
	if providers == nil {
		return nil
	}

	var result []apiProviders
	if len(providers.Inference) > 0 {
		result = append(result, apiProviders{APIType: "inference", Providers: providers.Inference})
	}
	if len(providers.Safety) > 0 {
		result = append(result, apiProviders{APIType: "safety", Providers: providers.Safety})
	}
	if len(providers.VectorIo) > 0 {
		result = append(result, apiProviders{APIType: "vector_io", Providers: providers.VectorIo})
	}
	if len(providers.ToolRuntime) > 0 {
		result = append(result, apiProviders{APIType: "tool_runtime", Providers: providers.ToolRuntime})
	}
	if len(providers.Telemetry) > 0 {
		result = append(result, apiProviders{APIType: "telemetry", Providers: providers.Telemetry})
	}
	return result
}

// ResolveProviderID returns the effective provider ID for a ProviderConfig.
// Uses the explicit ID if set, otherwise falls back to the Provider field.
func ResolveProviderID(p v1alpha2.ProviderConfig) string {
	if p.ID != "" {
		return p.ID
	}
	return p.Provider
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
