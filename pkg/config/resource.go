package config

import (
	"errors"
	"fmt"

	v1alpha2 "github.com/llamastack/llama-stack-k8s-operator/api/v1alpha2"
)

// registeredResource represents a resource entry in config.yaml's registered_resources.
type registeredResource struct {
	ResourceType string                 `json:"resource_type"`
	Provider     resourceProvider       `json:"provider"`
	Params       map[string]interface{} `json:"params"`
}

type resourceProvider struct {
	ProviderID   string `json:"provider_id"`
	ProviderType string `json:"provider_type"`
}

// ExpandResources converts CRD ResourcesSpec into config.yaml registered_resources entries.
// It validates provider references and assigns default providers where needed.
func ExpandResources(
	resources *v1alpha2.ResourcesSpec,
	userProviders *v1alpha2.ProvidersSpec,
	baseConfig map[string]interface{},
) ([]registeredResource, error) {
	if resources == nil {
		return nil, nil
	}

	var result []registeredResource

	// Expand models
	for _, model := range resources.Models {
		r, err := expandModel(model, userProviders, baseConfig)
		if err != nil {
			return nil, err
		}
		result = append(result, r)
	}

	// Expand tools
	if len(resources.Tools) > 0 {
		providerID, providerType, err := findProviderForAPI("tool_runtime", userProviders, baseConfig)
		if err != nil {
			return nil, errors.New("resources.tools requires at least one toolRuntime provider to be configured")
		}
		for _, tool := range resources.Tools {
			result = append(result, registeredResource{
				ResourceType: "tool_group",
				Provider: resourceProvider{
					ProviderID:   providerID,
					ProviderType: providerType,
				},
				Params: map[string]interface{}{
					"tool_group_id": tool,
				},
			})
		}
	}

	// Expand shields
	if len(resources.Shields) > 0 {
		providerID, providerType, err := findProviderForAPI("safety", userProviders, baseConfig)
		if err != nil {
			return nil, errors.New("resources.shields requires at least one safety provider to be configured")
		}
		for _, shield := range resources.Shields {
			result = append(result, registeredResource{
				ResourceType: "shield",
				Provider: resourceProvider{
					ProviderID:   providerID,
					ProviderType: providerType,
				},
				Params: map[string]interface{}{
					"shield_id": shield,
				},
			})
		}
	}

	return result, nil
}

func expandModel(
	model v1alpha2.ModelConfig,
	userProviders *v1alpha2.ProvidersSpec,
	baseConfig map[string]interface{},
) (registeredResource, error) {
	var providerID, providerType string

	if model.Provider != "" {
		// Explicit provider assignment (FR-041)
		providerID = model.Provider
		providerType = "remote::" + providerID
	} else {
		// Default to first inference provider (FR-040)
		var err error
		providerID, providerType, err = findProviderForAPI("inference", userProviders, baseConfig)
		if err != nil {
			return registeredResource{}, fmt.Errorf("failed to register model %q: no provider specified and no inference provider is configured", model.Name)
		}
	}

	params := map[string]interface{}{
		"model_id": model.Name,
	}
	if model.ContextLength != nil {
		params["context_length"] = *model.ContextLength
	}
	if model.ModelType != "" {
		params["model_type"] = model.ModelType
	}
	if model.Quantization != "" {
		params["quantization"] = model.Quantization
	}

	return registeredResource{
		ResourceType: "model",
		Provider: resourceProvider{
			ProviderID:   providerID,
			ProviderType: providerType,
		},
		Params: params,
	}, nil
}

// findProviderForAPI finds the first provider for the given API type,
// checking user providers first, then falling back to base config.
func findProviderForAPI(
	apiType string,
	userProviders *v1alpha2.ProvidersSpec,
	baseConfig map[string]interface{},
) (string, string, error) {
	// Check user providers first
	if id, pt, ok := findUserProvider(apiType, userProviders); ok {
		return id, pt, nil
	}

	// Fall back to base config
	if id, pt, ok := findBaseConfigProvider(apiType, baseConfig); ok {
		return id, pt, nil
	}

	return "", "", fmt.Errorf("failed to find %s provider", apiType)
}

func findUserProvider(apiType string, providers *v1alpha2.ProvidersSpec) (string, string, bool) {
	if providers == nil {
		return "", "", false
	}

	var list []v1alpha2.ProviderConfig
	switch apiType {
	case "inference":
		list = providers.Inference
	case "safety":
		list = providers.Safety
	case "vector_io":
		list = providers.VectorIo
	case "tool_runtime":
		list = providers.ToolRuntime
	case "telemetry":
		list = providers.Telemetry
	}

	if len(list) == 0 {
		return "", "", false
	}

	p := list[0]
	id := p.ID
	if id == "" {
		id = p.Provider
	}
	return id, "remote::" + p.Provider, true
}

func findBaseConfigProvider(apiType string, baseConfig map[string]interface{}) (string, string, bool) {
	if baseConfig == nil {
		return "", "", false
	}
	providersMap, ok := baseConfig["providers"].(map[string]interface{})
	if !ok {
		return "", "", false
	}
	apiProviderList, ok := providersMap[apiType].([]interface{})
	if !ok || len(apiProviderList) == 0 {
		return "", "", false
	}
	firstProvider, ok := apiProviderList[0].(map[string]interface{})
	if !ok {
		return "", "", false
	}
	pid, _ := firstProvider["provider_id"].(string)
	pt, _ := firstProvider["provider_type"].(string)
	if pid == "" {
		return "", "", false
	}
	return pid, pt, true
}
