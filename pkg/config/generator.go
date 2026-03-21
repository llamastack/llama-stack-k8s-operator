package config

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"

	v1alpha2 "github.com/llamastack/llama-stack-k8s-operator/api/v1alpha2"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/yaml"
)

// GeneratedConfig is the output of the config generation pipeline.
type GeneratedConfig struct {
	// ConfigYAML is the final config.yaml content.
	ConfigYAML string
	// ContentHash is the SHA256 hash of ConfigYAML (for change detection).
	ContentHash string
	// EnvVars contains environment variable definitions for the Deployment.
	EnvVars []corev1.EnvVar
	// ProviderCount is the number of configured providers.
	ProviderCount int
	// ResourceCount is the number of registered resources.
	ResourceCount int
	// ConfigVersion is the detected config.yaml schema version.
	ConfigVersion int
}

// GenerateConfig runs the full config generation pipeline.
// It takes a v1alpha2 spec and base config (YAML bytes), and produces a GeneratedConfig.
// This is a pure function with no Kubernetes API calls.
func GenerateConfig(spec *v1alpha2.LlamaStackDistributionSpec, baseConfigYAML []byte) (*GeneratedConfig, error) {
	// Parse base config
	baseConfig, err := parseConfig(baseConfigYAML)
	if err != nil {
		return nil, fmt.Errorf("failed to parse base config: %w", err)
	}

	// Detect and validate config version
	configVersion, err := DetectConfigVersion(baseConfig)
	if err != nil {
		return nil, err
	}
	if vErr := ValidateConfigVersion(configVersion); vErr != nil {
		return nil, vErr
	}

	// Merge user config onto base config
	merged, err := MergeConfig(baseConfig, spec)
	if err != nil {
		return nil, fmt.Errorf("failed to merge config: %w", err)
	}

	// Expand and add resources
	resources, err := ExpandResources(spec.Resources, spec.Providers, merged)
	if err != nil {
		return nil, fmt.Errorf("failed to expand resources: %w", err)
	}
	if len(resources) > 0 {
		merged["registered_resources"] = appendResources(merged, resources)
	}

	// Collect secret references
	envVars := CollectSecretRefs(spec)

	// Count providers in merged config
	providerCount := countMergedProviders(merged)

	// Serialize to YAML (deterministic)
	configYAML, err := serializeConfig(merged)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize config: %w", err)
	}

	// Compute content hash
	hash := sha256.Sum256([]byte(configYAML))
	contentHash := hex.EncodeToString(hash[:])

	return &GeneratedConfig{
		ConfigYAML:    configYAML,
		ContentHash:   contentHash,
		EnvVars:       envVars,
		ProviderCount: providerCount,
		ResourceCount: len(resources),
		ConfigVersion: configVersion,
	}, nil
}

func parseConfig(data []byte) (map[string]interface{}, error) {
	var config map[string]interface{}
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}
	return config, nil
}

// serializeConfig produces deterministic YAML output with sorted keys.
func serializeConfig(config map[string]interface{}) (string, error) {
	// yaml.Marshal from sigs.k8s.io/yaml produces sorted keys by default
	// because it goes through JSON (which sorts keys) first.
	data, err := yaml.Marshal(config)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func appendResources(config map[string]interface{}, resources []registeredResource) []interface{} {
	var existing []interface{}
	if rr, ok := config["registered_resources"].([]interface{}); ok {
		existing = rr
	}

	// Build a set of existing resource keys for deduplication
	existingKeys := make(map[string]bool)
	for _, r := range existing {
		if rm, ok := r.(map[string]interface{}); ok {
			key := resourceKey(rm)
			if key != "" {
				existingKeys[key] = true
			}
		}
	}

	for _, r := range resources {
		key := r.ResourceType + "/" + resourceIDFromParams(r.Params)
		if existingKeys[key] {
			continue
		}
		existing = append(existing, map[string]interface{}{
			"resource_type": r.ResourceType,
			"provider": map[string]interface{}{
				"provider_id":   r.Provider.ProviderID,
				"provider_type": r.Provider.ProviderType,
			},
			"params": r.Params,
		})
	}

	return existing
}

// resourceKey extracts a deduplication key from an existing registered_resources entry.
func resourceKey(rm map[string]interface{}) string {
	rt, _ := rm["resource_type"].(string)
	if rt == "" {
		return ""
	}
	params, _ := rm["params"].(map[string]interface{})
	if params == nil {
		return ""
	}
	return rt + "/" + resourceIDFromParams(params)
}

// resourceIDFromParams extracts the identifying field from resource params.
func resourceIDFromParams(params map[string]interface{}) string {
	// Each resource type has its own ID field
	for _, key := range []string{"model_id", "shield_id", "tool_group_id"} {
		if id, ok := params[key].(string); ok {
			return id
		}
	}
	return ""
}

func countMergedProviders(config map[string]interface{}) int {
	providersMap, ok := config["providers"].(map[string]interface{})
	if !ok {
		return 0
	}

	count := 0
	for _, v := range providersMap {
		if providers, ok := v.([]interface{}); ok {
			count += len(providers)
		}
	}
	return count
}
