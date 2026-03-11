package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	v1alpha2 "github.com/llamastack/llama-stack-k8s-operator/api/v1alpha2"
)

func TestGenerateConfig_MinimalInference(t *testing.T) {
	resolver := NewEmbeddedConfigResolver()
	baseConfig, err := resolver.Resolve("starter")
	require.NoError(t, err)

	spec := &v1alpha2.LlamaStackDistributionSpec{
		Providers: &v1alpha2.ProvidersSpec{
			Inference: []v1alpha2.ProviderConfig{
				{
					ID:       "remote::vllm",
					Provider: "vllm",
					Endpoint: "http://vllm:8000",
				},
			},
		},
	}

	result, err := GenerateConfig(spec, baseConfig)
	require.NoError(t, err)

	assert.NotEmpty(t, result.ConfigYAML)
	assert.NotEmpty(t, result.ContentHash)
	assert.Len(t, result.ContentHash, 64, "SHA256 hex hash should be 64 chars")
	assert.Empty(t, result.EnvVars, "no secrets configured")
	assert.Positive(t, result.ProviderCount)
	assert.Equal(t, 0, result.ResourceCount)
	assert.Equal(t, 2, result.ConfigVersion)
}

func TestGenerateConfig_WithAPIKey(t *testing.T) {
	resolver := NewEmbeddedConfigResolver()
	baseConfig, err := resolver.Resolve("starter")
	require.NoError(t, err)

	spec := &v1alpha2.LlamaStackDistributionSpec{
		Providers: &v1alpha2.ProvidersSpec{
			Inference: []v1alpha2.ProviderConfig{
				{
					Provider: "vllm",
					Endpoint: "http://vllm:8000",
					APIKey:   &v1alpha2.SecretKeyRef{Name: "vllm-creds", Key: "token"},
				},
			},
		},
	}

	result, err := GenerateConfig(spec, baseConfig)
	require.NoError(t, err)

	assert.Len(t, result.EnvVars, 1)
	assert.Equal(t, "LLSD_VLLM_API_KEY", result.EnvVars[0].Name)

	// Config should contain env var reference
	assert.Contains(t, result.ConfigYAML, "${env.LLSD_VLLM_API_KEY}")
}

func TestGenerateConfig_WithResources(t *testing.T) {
	resolver := NewEmbeddedConfigResolver()
	baseConfig, err := resolver.Resolve("starter")
	require.NoError(t, err)

	spec := &v1alpha2.LlamaStackDistributionSpec{
		Providers: &v1alpha2.ProvidersSpec{
			Inference: []v1alpha2.ProviderConfig{
				{
					Provider: "vllm",
					Endpoint: "http://vllm:8000",
				},
			},
		},
		Resources: &v1alpha2.ResourcesSpec{
			Models: []v1alpha2.ModelConfig{
				{Name: "llama3.2-8b"},
				{Name: "llama3.2-70b"},
			},
		},
	}

	result, err := GenerateConfig(spec, baseConfig)
	require.NoError(t, err)

	assert.Equal(t, 2, result.ResourceCount)
	assert.Contains(t, result.ConfigYAML, "llama3.2-8b")
	assert.Contains(t, result.ConfigYAML, "llama3.2-70b")
}

func TestGenerateConfig_WithDisabledAPIs(t *testing.T) {
	resolver := NewEmbeddedConfigResolver()
	baseConfig, err := resolver.Resolve("starter")
	require.NoError(t, err)

	spec := &v1alpha2.LlamaStackDistributionSpec{
		Disabled: []string{"eval", "scoring"},
	}

	result, err := GenerateConfig(spec, baseConfig)
	require.NoError(t, err)

	assert.NotContains(t, result.ConfigYAML, "eval:")
	assert.NotContains(t, result.ConfigYAML, "scoring:")
}

func TestGenerateConfig_Deterministic(t *testing.T) {
	resolver := NewEmbeddedConfigResolver()
	baseConfig, err := resolver.Resolve("starter")
	require.NoError(t, err)

	spec := &v1alpha2.LlamaStackDistributionSpec{
		Providers: &v1alpha2.ProvidersSpec{
			Inference: []v1alpha2.ProviderConfig{
				{
					Provider: "vllm",
					Endpoint: "http://vllm:8000",
					APIKey:   &v1alpha2.SecretKeyRef{Name: "creds", Key: "key"},
				},
			},
		},
		Resources: &v1alpha2.ResourcesSpec{
			Models: []v1alpha2.ModelConfig{
				{Name: "model-a"},
				{Name: "model-b"},
			},
		},
	}

	result1, err := GenerateConfig(spec, baseConfig)
	require.NoError(t, err)

	result2, err := GenerateConfig(spec, baseConfig)
	require.NoError(t, err)

	assert.Equal(t, result1.ConfigYAML, result2.ConfigYAML, "same inputs must produce identical output")
	assert.Equal(t, result1.ContentHash, result2.ContentHash, "same inputs must produce identical hash")
}

func TestGenerateConfig_InvalidBaseConfig(t *testing.T) {
	spec := &v1alpha2.LlamaStackDistributionSpec{}

	_, err := GenerateConfig(spec, []byte("not: yaml: at: all: ["))
	require.Error(t, err)
}

func TestGenerateConfig_UnsupportedVersion(t *testing.T) {
	spec := &v1alpha2.LlamaStackDistributionSpec{}

	_, err := GenerateConfig(spec, []byte("version: 99\n"))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to validate config version")
}

func TestGenerateConfig_WithStorage(t *testing.T) {
	resolver := NewEmbeddedConfigResolver()
	baseConfig, err := resolver.Resolve("starter")
	require.NoError(t, err)

	spec := &v1alpha2.LlamaStackDistributionSpec{
		Storage: &v1alpha2.StorageSpec{
			SQL: &v1alpha2.SQLStorageSpec{
				Type:             "postgres",
				ConnectionString: &v1alpha2.SecretKeyRef{Name: "pg-creds", Key: "conn"},
			},
		},
	}

	result, err := GenerateConfig(spec, baseConfig)
	require.NoError(t, err)

	assert.Contains(t, result.ConfigYAML, "postgres")
	assert.Contains(t, result.ConfigYAML, "${env.LLSD_STORAGE_SQL_CONNECTION_STRING}")
	assert.Len(t, result.EnvVars, 1)
	assert.Equal(t, "LLSD_STORAGE_SQL_CONNECTION_STRING", result.EnvVars[0].Name)
}

func TestGenerateConfig_EmptySpec(t *testing.T) {
	resolver := NewEmbeddedConfigResolver()
	baseConfig, err := resolver.Resolve("starter")
	require.NoError(t, err)

	spec := &v1alpha2.LlamaStackDistributionSpec{}

	result, err := GenerateConfig(spec, baseConfig)
	require.NoError(t, err)

	assert.NotEmpty(t, result.ConfigYAML)
	assert.Positive(t, result.ProviderCount, "base config providers should be counted")
	assert.Equal(t, 0, result.ResourceCount)
}

func TestGenerateConfig_OverlayPreservesBaseProviders(t *testing.T) {
	resolver := NewEmbeddedConfigResolver()
	baseConfig, err := resolver.Resolve("starter")
	require.NoError(t, err)

	// Override only the inference provider, all other base providers should remain
	spec := &v1alpha2.LlamaStackDistributionSpec{
		Providers: &v1alpha2.ProvidersSpec{
			Inference: []v1alpha2.ProviderConfig{
				{
					ID:       "remote::vllm",
					Provider: "ollama",
					Endpoint: "http://ollama:11434",
				},
			},
		},
	}

	result, err := GenerateConfig(spec, baseConfig)
	require.NoError(t, err)

	// Should still contain base config providers for other API types
	assert.Contains(t, result.ConfigYAML, "inline::llama-guard")
	assert.Contains(t, result.ConfigYAML, "inline::meta-reference")
	assert.Contains(t, result.ConfigYAML, "inline::faiss")
	// Inference should be replaced
	assert.Contains(t, result.ConfigYAML, "remote::ollama")
}
