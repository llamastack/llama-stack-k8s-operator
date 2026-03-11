package config

import (
	v1alpha2 "github.com/llamastack/llama-stack-k8s-operator/api/v1alpha2"
)

// MergeConfig applies user configuration from the CRD spec onto the base config.
// Merge strategies per section:
//   - providers: overlay_by_provider_id (match→replace, new→append, unmatched→preserve)
//   - storage: merge_by_subsection (kv and sql independently replaced)
//   - resources: additive (user resources appended)
//   - disabled: subtractive (matching APIs removed)
func MergeConfig(
	baseConfig map[string]interface{},
	spec *v1alpha2.LlamaStackDistributionSpec,
) (map[string]interface{}, error) {
	// Start with a copy of the base config
	merged := shallowCopyMap(baseConfig)

	// Overlay providers by provider_id
	if spec.Providers != nil {
		if err := overlayProviders(merged, spec.Providers); err != nil {
			return nil, err
		}
	}

	// Apply storage (merge_by_subsection)
	ApplyStorage(spec.Storage, merged)

	// Apply disabled (subtractive)
	applyDisabled(merged, spec.Disabled)

	return merged, nil
}

// overlayProviders overlays user providers onto base config using provider-ID matching.
// For each API type where the user specifies providers:
//   - Base providers with matching IDs are replaced
//   - User providers with new IDs are appended
//   - Base providers with unmatched IDs are preserved
func overlayProviders(config map[string]interface{}, providers *v1alpha2.ProvidersSpec) error {
	providersMap, ok := config["providers"].(map[string]interface{})
	if !ok {
		providersMap = make(map[string]interface{})
		config["providers"] = providersMap
	}

	for _, ap := range AllAPIProviders(providers) {
		expanded, err := ExpandProviders(ap.Providers)
		if err != nil {
			return err
		}

		baseProviders := getBaseProviderList(providersMap, ap.APIType)
		providersMap[ap.APIType] = overlayProviderList(baseProviders, expanded)
	}

	return nil
}

// overlayProviderList overlays user providers onto base providers by provider_id.
func overlayProviderList(base []map[string]interface{}, user []configProvider) []interface{} {
	// Build a set of user provider IDs for quick lookup
	userByID := make(map[string]configProvider, len(user))
	for _, u := range user {
		userByID[u.ProviderID] = u
	}

	// Start with base providers, replacing matching IDs
	var result []interface{}
	matched := make(map[string]bool)

	for _, bp := range base {
		baseID, _ := bp["provider_id"].(string)
		if up, found := userByID[baseID]; found {
			// Matching ID: user provider replaces base provider
			result = append(result, providerToMap(up))
			matched[baseID] = true
		} else {
			// Unmatched base ID: preserved
			result = append(result, bp)
		}
	}

	// Append unmatched user providers (new IDs)
	for _, u := range user {
		if !matched[u.ProviderID] {
			result = append(result, providerToMap(u))
		}
	}

	return result
}

// getBaseProviderList extracts the provider list for an API type from the base config.
func getBaseProviderList(providersMap map[string]interface{}, apiType string) []map[string]interface{} {
	providerList, ok := providersMap[apiType].([]interface{})
	if !ok {
		return nil
	}

	result := make([]map[string]interface{}, 0, len(providerList))
	for _, p := range providerList {
		if pm, ok := p.(map[string]interface{}); ok {
			result = append(result, pm)
		}
	}
	return result
}

func providerToMap(p configProvider) map[string]interface{} {
	m := map[string]interface{}{
		"provider_id":   p.ProviderID,
		"provider_type": p.ProviderType,
	}
	if len(p.Config) > 0 {
		m["config"] = p.Config
	} else {
		m["config"] = map[string]interface{}{}
	}
	return m
}

// applyDisabled removes disabled API types from the config.
func applyDisabled(config map[string]interface{}, disabled []string) {
	if len(disabled) == 0 {
		return
	}

	disabledSet := make(map[string]bool, len(disabled))
	for _, d := range disabled {
		disabledSet[d] = true
	}

	// Remove from apis list
	if apis, ok := config["apis"].([]interface{}); ok {
		filtered := make([]interface{}, 0, len(apis))
		for _, api := range apis {
			if apiStr, ok := api.(string); ok && !disabledSet[apiStr] {
				filtered = append(filtered, api)
			}
		}
		config["apis"] = filtered
	}

	// Remove from providers map
	if providersMap, ok := config["providers"].(map[string]interface{}); ok {
		for _, d := range disabled {
			delete(providersMap, d)
		}
	}
}

func shallowCopyMap(m map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{}, len(m))
	for k, v := range m {
		result[k] = v
	}
	return result
}
