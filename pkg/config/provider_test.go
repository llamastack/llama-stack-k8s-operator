package config

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"

	v1alpha2 "github.com/llamastack/llama-stack-k8s-operator/api/v1alpha2"
)

func jsonSettings(m map[string]interface{}) *apiextensionsv1.JSON {
	data, _ := json.Marshal(m)
	return &apiextensionsv1.JSON{Raw: data}
}

func TestExpandProviders_SingleProvider(t *testing.T) {
	providers := []v1alpha2.ProviderConfig{
		{
			Provider: "vllm",
			Endpoint: "http://vllm:8000",
		},
	}

	result, err := ExpandProviders(providers)
	require.NoError(t, err)
	require.Len(t, result, 1)

	assert.Equal(t, "vllm", result[0].ProviderID, "auto-generated ID from provider field")
	assert.Equal(t, "remote::vllm", result[0].ProviderType)
	assert.Equal(t, "http://vllm:8000", result[0].Config["url"])
}

func TestExpandProviders_SingleProviderWithExplicitID(t *testing.T) {
	providers := []v1alpha2.ProviderConfig{
		{
			ID:       "my-vllm",
			Provider: "vllm",
			Endpoint: "http://vllm:8000",
		},
	}

	result, err := ExpandProviders(providers)
	require.NoError(t, err)
	require.Len(t, result, 1)

	assert.Equal(t, "my-vllm", result[0].ProviderID)
}

func TestExpandProviders_MultipleProviders(t *testing.T) {
	providers := []v1alpha2.ProviderConfig{
		{
			ID:       "primary",
			Provider: "vllm",
			Endpoint: "http://vllm:8000",
		},
		{
			ID:       "fallback",
			Provider: "ollama",
			Endpoint: "http://ollama:11434",
		},
	}

	result, err := ExpandProviders(providers)
	require.NoError(t, err)
	require.Len(t, result, 2)

	assert.Equal(t, "primary", result[0].ProviderID)
	assert.Equal(t, "remote::vllm", result[0].ProviderType)
	assert.Equal(t, "fallback", result[1].ProviderID)
	assert.Equal(t, "remote::ollama", result[1].ProviderType)
}

func TestExpandProviders_MultipleWithoutID_Error(t *testing.T) {
	providers := []v1alpha2.ProviderConfig{
		{Provider: "vllm", Endpoint: "http://vllm:8000"},
		{Provider: "ollama", Endpoint: "http://ollama:11434"},
	}

	_, err := ExpandProviders(providers)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required 'id' field")
}

func TestExpandProviders_WithSettings(t *testing.T) {
	providers := []v1alpha2.ProviderConfig{
		{
			Provider: "vllm",
			Endpoint: "http://vllm:8000",
			Settings: jsonSettings(map[string]interface{}{
				"max_tokens":  4096,
				"temperature": 0.7,
			}),
		},
	}

	result, err := ExpandProviders(providers)
	require.NoError(t, err)
	require.Len(t, result, 1)

	assert.Equal(t, "http://vllm:8000", result[0].Config["url"])
	assert.Equal(t, float64(4096), result[0].Config["max_tokens"])
	assert.Equal(t, 0.7, result[0].Config["temperature"])
}

func TestExpandProviders_WithAPIKey(t *testing.T) {
	providers := []v1alpha2.ProviderConfig{
		{
			Provider: "vllm",
			Endpoint: "http://vllm:8000",
			APIKey: &v1alpha2.SecretKeyRef{
				Name: "vllm-creds",
				Key:  "token",
			},
		},
	}

	result, err := ExpandProviders(providers)
	require.NoError(t, err)
	require.Len(t, result, 1)

	assert.Equal(t, "${env.LLSD_VLLM_API_KEY}", result[0].Config["api_key"])
}

func TestExpandProviders_WithSecretRefs(t *testing.T) {
	providers := []v1alpha2.ProviderConfig{
		{
			ID:       "pgvector",
			Provider: "pgvector",
			SecretRefs: map[string]v1alpha2.SecretKeyRef{
				"host":     {Name: "pg-creds", Key: "host"},
				"password": {Name: "pg-creds", Key: "password"},
			},
		},
	}

	result, err := ExpandProviders(providers)
	require.NoError(t, err)
	require.Len(t, result, 1)

	assert.Equal(t, "${env.LLSD_PGVECTOR_HOST}", result[0].Config["host"])
	assert.Equal(t, "${env.LLSD_PGVECTOR_PASSWORD}", result[0].Config["password"])
}

func TestExpandProviders_HyphenatedID(t *testing.T) {
	providers := []v1alpha2.ProviderConfig{
		{
			ID:       "vllm-primary",
			Provider: "vllm",
			APIKey: &v1alpha2.SecretKeyRef{
				Name: "vllm-creds",
				Key:  "token",
			},
		},
	}

	result, err := ExpandProviders(providers)
	require.NoError(t, err)

	assert.Equal(t, "${env.LLSD_VLLM_PRIMARY_API_KEY}", result[0].Config["api_key"])
}

func TestCountProviders(t *testing.T) {
	tests := []struct {
		name      string
		providers *v1alpha2.ProvidersSpec
		want      int
	}{
		{
			name:      "nil providers",
			providers: nil,
			want:      0,
		},
		{
			name: "single inference",
			providers: &v1alpha2.ProvidersSpec{
				Inference: []v1alpha2.ProviderConfig{{Provider: "vllm"}},
			},
			want: 1,
		},
		{
			name: "multiple types",
			providers: &v1alpha2.ProvidersSpec{
				Inference:   []v1alpha2.ProviderConfig{{Provider: "vllm"}, {Provider: "ollama"}},
				Safety:      []v1alpha2.ProviderConfig{{Provider: "llama-guard"}},
				ToolRuntime: []v1alpha2.ProviderConfig{{Provider: "brave-search"}},
			},
			want: 4,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, CountProviders(tt.providers))
		})
	}
}
