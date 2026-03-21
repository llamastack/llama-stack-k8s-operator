package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	v1alpha2 "github.com/llamastack/llama-stack-k8s-operator/api/v1alpha2"
)

func TestExpandResources_ModelWithDefaultProvider(t *testing.T) {
	resources := &v1alpha2.ResourcesSpec{
		Models: []v1alpha2.ModelConfig{
			{Name: "llama3.2-8b"},
		},
	}
	providers := &v1alpha2.ProvidersSpec{
		Inference: []v1alpha2.ProviderConfig{
			{Provider: "vllm", Endpoint: "http://vllm:8000"},
		},
	}

	result, err := ExpandResources(resources, providers, nil)
	require.NoError(t, err)
	require.Len(t, result, 1)

	assert.Equal(t, "model", result[0].ResourceType)
	assert.Equal(t, "vllm", result[0].Provider.ProviderID)
	assert.Equal(t, "remote::vllm", result[0].Provider.ProviderType)
	assert.Equal(t, "llama3.2-8b", result[0].Params["model_id"])
}

func TestExpandResources_ModelWithExplicitProvider(t *testing.T) {
	resources := &v1alpha2.ResourcesSpec{
		Models: []v1alpha2.ModelConfig{
			{Name: "llama3.2-70b", Provider: "vllm-primary"},
		},
	}
	providers := &v1alpha2.ProvidersSpec{
		Inference: []v1alpha2.ProviderConfig{
			{ID: "vllm-primary", Provider: "vllm", Endpoint: "http://vllm:8000"},
		},
	}

	result, err := ExpandResources(resources, providers, nil)
	require.NoError(t, err)
	require.Len(t, result, 1)

	assert.Equal(t, "vllm-primary", result[0].Provider.ProviderID)
	assert.Equal(t, "remote::vllm", result[0].Provider.ProviderType, "provider_type should use the provider name, not the custom ID")
}

func TestExpandResources_ModelWithMetadata(t *testing.T) {
	contextLen := 8192
	resources := &v1alpha2.ResourcesSpec{
		Models: []v1alpha2.ModelConfig{
			{
				Name:          "llama3.2-8b",
				ContextLength: &contextLen,
				ModelType:     "llm",
				Quantization:  "fp16",
			},
		},
	}
	providers := &v1alpha2.ProvidersSpec{
		Inference: []v1alpha2.ProviderConfig{
			{Provider: "vllm", Endpoint: "http://vllm:8000"},
		},
	}

	result, err := ExpandResources(resources, providers, nil)
	require.NoError(t, err)
	require.Len(t, result, 1)

	assert.Equal(t, 8192, result[0].Params["context_length"])
	assert.Equal(t, "llm", result[0].Params["model_type"])
	assert.Equal(t, "fp16", result[0].Params["quantization"])
}

func TestExpandResources_Tools(t *testing.T) {
	resources := &v1alpha2.ResourcesSpec{
		Tools: []string{"websearch", "code-interpreter"},
	}
	providers := &v1alpha2.ProvidersSpec{
		ToolRuntime: []v1alpha2.ProviderConfig{
			{Provider: "brave-search", Endpoint: "http://brave:8080"},
		},
	}

	result, err := ExpandResources(resources, providers, nil)
	require.NoError(t, err)
	require.Len(t, result, 2)

	assert.Equal(t, "tool_group", result[0].ResourceType)
	assert.Equal(t, "websearch", result[0].Params["tool_group_id"])
	assert.Equal(t, "code-interpreter", result[1].Params["tool_group_id"])
}

func TestExpandResources_ToolsWithoutProvider_Error(t *testing.T) {
	resources := &v1alpha2.ResourcesSpec{
		Tools: []string{"websearch"},
	}

	_, err := ExpandResources(resources, &v1alpha2.ProvidersSpec{}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "toolRuntime provider")
}

func TestExpandResources_Shields(t *testing.T) {
	resources := &v1alpha2.ResourcesSpec{
		Shields: []string{"llama-guard"},
	}
	providers := &v1alpha2.ProvidersSpec{
		Safety: []v1alpha2.ProviderConfig{
			{Provider: "llama-guard"},
		},
	}

	result, err := ExpandResources(resources, providers, nil)
	require.NoError(t, err)
	require.Len(t, result, 1)

	assert.Equal(t, "shield", result[0].ResourceType)
	assert.Equal(t, "llama-guard", result[0].Params["shield_id"])
}

func TestExpandResources_ShieldsWithoutProvider_Error(t *testing.T) {
	resources := &v1alpha2.ResourcesSpec{
		Shields: []string{"llama-guard"},
	}

	_, err := ExpandResources(resources, &v1alpha2.ProvidersSpec{}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "safety provider")
}

func TestExpandResources_Nil(t *testing.T) {
	result, err := ExpandResources(nil, nil, nil)
	require.NoError(t, err)
	assert.Nil(t, result)
}

func TestExpandResources_FallbackToBaseConfig(t *testing.T) {
	resources := &v1alpha2.ResourcesSpec{
		Tools: []string{"websearch"},
	}

	baseConfig := map[string]interface{}{
		"providers": map[string]interface{}{
			"tool_runtime": []interface{}{
				map[string]interface{}{
					"provider_id":   "inline::rag-runtime",
					"provider_type": "inline::rag-runtime",
				},
			},
		},
	}

	result, err := ExpandResources(resources, &v1alpha2.ProvidersSpec{}, baseConfig)
	require.NoError(t, err)
	require.Len(t, result, 1)

	assert.Equal(t, "inline::rag-runtime", result[0].Provider.ProviderID)
	assert.Equal(t, "inline::rag-runtime", result[0].Provider.ProviderType)
}

func TestExpandResources_ModelNoProvider_Error(t *testing.T) {
	resources := &v1alpha2.ResourcesSpec{
		Models: []v1alpha2.ModelConfig{
			{Name: "llama3.2-8b"},
		},
	}

	_, err := ExpandResources(resources, &v1alpha2.ProvidersSpec{}, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no provider")
}
