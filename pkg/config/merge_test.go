package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	v1alpha2 "github.com/llamastack/llama-stack-k8s-operator/api/v1alpha2"
)

// Test helpers for safe type assertions.
func requireMapValue(t *testing.T, m map[string]interface{}, key string) map[string]interface{} {
	t.Helper()
	v, ok := m[key].(map[string]interface{})
	require.True(t, ok, "expected map[string]interface{} at key %q", key)
	return v
}

func requireSliceValue(t *testing.T, m map[string]interface{}, key string) []interface{} {
	t.Helper()
	v, ok := m[key].([]interface{})
	require.True(t, ok, "expected []interface{} at key %q", key)
	return v
}

func requireMapAt(t *testing.T, s []interface{}, idx int) map[string]interface{} {
	t.Helper()
	require.Greater(t, len(s), idx, "index %d out of range", idx)
	v, ok := s[idx].(map[string]interface{})
	require.True(t, ok, "expected map[string]interface{} at index %d", idx)
	return v
}

func baseConfigWithProviders() map[string]interface{} {
	return map[string]interface{}{
		"version": 2,
		"apis":    []interface{}{"inference", "safety", "agents"},
		"providers": map[string]interface{}{
			"inference": []interface{}{
				map[string]interface{}{
					"provider_id":   "remote::vllm",
					"provider_type": "remote::vllm",
					"config":        map[string]interface{}{"url": "http://old-vllm:8000"},
				},
				map[string]interface{}{
					"provider_id":   "inline::sentence-transformers",
					"provider_type": "inline::sentence-transformers",
					"config":        map[string]interface{}{},
				},
			},
			"safety": []interface{}{
				map[string]interface{}{
					"provider_id":   "inline::llama-guard",
					"provider_type": "inline::llama-guard",
					"config":        map[string]interface{}{},
				},
			},
		},
	}
}

func TestOverlayProviders_MatchReplace(t *testing.T) {
	base := baseConfigWithProviders()

	spec := &v1alpha2.LlamaStackDistributionSpec{
		Providers: &v1alpha2.ProvidersSpec{
			Inference: []v1alpha2.ProviderConfig{
				{
					ID:       "remote::vllm",
					Provider: "vllm",
					Endpoint: "http://new-vllm:8000",
				},
			},
		},
	}

	merged, err := MergeConfig(base, spec)
	require.NoError(t, err)

	providers := requireMapValue(t, merged, "providers")
	infProviders := requireSliceValue(t, providers, "inference")

	require.Len(t, infProviders, 2, "should preserve unmatched base provider")

	// First provider should be replaced
	first := requireMapAt(t, infProviders, 0)
	assert.Equal(t, "remote::vllm", first["provider_id"])
	assert.Equal(t, "remote::vllm", first["provider_type"])
	cfg := requireMapValue(t, first, "config")
	assert.Equal(t, "http://new-vllm:8000", cfg["url"])

	// Second provider (unmatched base) should be preserved
	second := requireMapAt(t, infProviders, 1)
	assert.Equal(t, "inline::sentence-transformers", second["provider_id"])
}

func TestOverlayProviders_AppendNew(t *testing.T) {
	base := baseConfigWithProviders()

	spec := &v1alpha2.LlamaStackDistributionSpec{
		Providers: &v1alpha2.ProvidersSpec{
			Inference: []v1alpha2.ProviderConfig{
				{
					ID:       "custom-ollama",
					Provider: "ollama",
					Endpoint: "http://ollama:11434",
				},
			},
		},
	}

	merged, err := MergeConfig(base, spec)
	require.NoError(t, err)

	providers := requireMapValue(t, merged, "providers")
	infProviders := requireSliceValue(t, providers, "inference")

	require.Len(t, infProviders, 3, "should have 2 base + 1 appended")

	// Base providers preserved
	first := requireMapAt(t, infProviders, 0)
	assert.Equal(t, "remote::vllm", first["provider_id"])

	second := requireMapAt(t, infProviders, 1)
	assert.Equal(t, "inline::sentence-transformers", second["provider_id"])

	// New provider appended
	third := requireMapAt(t, infProviders, 2)
	assert.Equal(t, "custom-ollama", third["provider_id"])
	assert.Equal(t, "remote::ollama", third["provider_type"])
}

func TestOverlayProviders_NoUserProviders(t *testing.T) {
	base := baseConfigWithProviders()

	spec := &v1alpha2.LlamaStackDistributionSpec{}

	merged, err := MergeConfig(base, spec)
	require.NoError(t, err)

	providers := requireMapValue(t, merged, "providers")
	infProviders := requireSliceValue(t, providers, "inference")
	require.Len(t, infProviders, 2, "base providers should be fully preserved")
}

func TestOverlayProviders_DifferentAPIType(t *testing.T) {
	base := baseConfigWithProviders()

	spec := &v1alpha2.LlamaStackDistributionSpec{
		Providers: &v1alpha2.ProvidersSpec{
			Safety: []v1alpha2.ProviderConfig{
				{
					ID:       "inline::llama-guard",
					Provider: "llama-guard",
					Endpoint: "http://safety:8080",
				},
			},
		},
	}

	merged, err := MergeConfig(base, spec)
	require.NoError(t, err)

	providers := requireMapValue(t, merged, "providers")

	// Inference should be unchanged
	infProviders := requireSliceValue(t, providers, "inference")
	require.Len(t, infProviders, 2)

	// Safety should be replaced
	safetyProviders := requireSliceValue(t, providers, "safety")
	require.Len(t, safetyProviders, 1)
	first := requireMapAt(t, safetyProviders, 0)
	assert.Equal(t, "inline::llama-guard", first["provider_id"])
	cfg := requireMapValue(t, first, "config")
	assert.Equal(t, "http://safety:8080", cfg["url"])
}

func TestApplyDisabled(t *testing.T) {
	config := map[string]interface{}{
		"apis": []interface{}{"inference", "safety", "agents"},
		"providers": map[string]interface{}{
			"inference": []interface{}{map[string]interface{}{"provider_id": "vllm"}},
			"safety":    []interface{}{map[string]interface{}{"provider_id": "guard"}},
			"agents":    []interface{}{map[string]interface{}{"provider_id": "meta"}},
		},
	}

	spec := &v1alpha2.LlamaStackDistributionSpec{
		Disabled: []string{"safety"},
	}

	merged, err := MergeConfig(config, spec)
	require.NoError(t, err)

	apis := requireSliceValue(t, merged, "apis")
	assert.Len(t, apis, 2)
	assert.Contains(t, apis, "inference")
	assert.Contains(t, apis, "agents")
	assert.NotContains(t, apis, "safety")

	providers := requireMapValue(t, merged, "providers")
	assert.Contains(t, providers, "inference")
	assert.Contains(t, providers, "agents")
	assert.NotContains(t, providers, "safety")
}

func TestMergeConfig_DoesNotMutateBase(t *testing.T) {
	base := baseConfigWithProviders()
	origAPIs := requireSliceValue(t, base, "apis")
	origLen := len(origAPIs)

	spec := &v1alpha2.LlamaStackDistributionSpec{
		Disabled: []string{"safety"},
	}

	_, err := MergeConfig(base, spec)
	require.NoError(t, err)

	// Base config apis should be unchanged
	assert.Len(t, requireSliceValue(t, base, "apis"), origLen)
}
