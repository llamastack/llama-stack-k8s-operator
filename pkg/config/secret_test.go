package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	v1alpha2 "github.com/llamastack/llama-stack-k8s-operator/api/v1alpha2"
)

func TestCollectSecretRefs_APIKey(t *testing.T) {
	spec := &v1alpha2.LlamaStackDistributionSpec{
		Providers: &v1alpha2.ProvidersSpec{
			Inference: []v1alpha2.ProviderConfig{
				{
					Provider: "vllm",
					APIKey: &v1alpha2.SecretKeyRef{
						Name: "vllm-creds",
						Key:  "token",
					},
				},
			},
		},
	}

	envVars := CollectSecretRefs(spec)
	require.Len(t, envVars, 1)

	assert.Equal(t, "LLSD_VLLM_API_KEY", envVars[0].Name)
	assert.Equal(t, "vllm-creds", envVars[0].ValueFrom.SecretKeyRef.Name)
	assert.Equal(t, "token", envVars[0].ValueFrom.SecretKeyRef.Key)
}

func TestCollectSecretRefs_SecretRefs(t *testing.T) {
	spec := &v1alpha2.LlamaStackDistributionSpec{
		Providers: &v1alpha2.ProvidersSpec{
			VectorIo: []v1alpha2.ProviderConfig{
				{
					ID:       "pgvector",
					Provider: "pgvector",
					SecretRefs: map[string]v1alpha2.SecretKeyRef{
						"host":     {Name: "pg-creds", Key: "host"},
						"password": {Name: "pg-creds", Key: "password"},
					},
				},
			},
		},
	}

	envVars := CollectSecretRefs(spec)
	require.Len(t, envVars, 2)

	// Sorted by key name
	assert.Equal(t, "LLSD_PGVECTOR_HOST", envVars[0].Name)
	assert.Equal(t, "LLSD_PGVECTOR_PASSWORD", envVars[1].Name)
}

func TestCollectSecretRefs_HyphenatedProviderID(t *testing.T) {
	spec := &v1alpha2.LlamaStackDistributionSpec{
		Providers: &v1alpha2.ProvidersSpec{
			Inference: []v1alpha2.ProviderConfig{
				{
					ID:       "vllm-primary",
					Provider: "vllm",
					APIKey: &v1alpha2.SecretKeyRef{
						Name: "vllm-creds",
						Key:  "token",
					},
				},
			},
		},
	}

	envVars := CollectSecretRefs(spec)
	require.Len(t, envVars, 1)
	assert.Equal(t, "LLSD_VLLM_PRIMARY_API_KEY", envVars[0].Name)
}

func TestCollectSecretRefs_StorageKVPassword(t *testing.T) {
	spec := &v1alpha2.LlamaStackDistributionSpec{
		Storage: &v1alpha2.StorageSpec{
			KV: &v1alpha2.KVStorageSpec{
				Type:     "redis",
				Endpoint: "redis://redis:6379",
				Password: &v1alpha2.SecretKeyRef{
					Name: "redis-creds",
					Key:  "password",
				},
			},
		},
	}

	envVars := CollectSecretRefs(spec)
	require.Len(t, envVars, 1)
	assert.Equal(t, "LLSD_STORAGE_KV_PASSWORD", envVars[0].Name)
	assert.Equal(t, "redis-creds", envVars[0].ValueFrom.SecretKeyRef.Name)
}

func TestCollectSecretRefs_StorageSQLConnectionString(t *testing.T) {
	spec := &v1alpha2.LlamaStackDistributionSpec{
		Storage: &v1alpha2.StorageSpec{
			SQL: &v1alpha2.SQLStorageSpec{
				Type: "postgres",
				ConnectionString: &v1alpha2.SecretKeyRef{
					Name: "pg-creds",
					Key:  "connection-string",
				},
			},
		},
	}

	envVars := CollectSecretRefs(spec)
	require.Len(t, envVars, 1)
	assert.Equal(t, "LLSD_STORAGE_SQL_CONNECTION_STRING", envVars[0].Name)
}

func TestCollectSecretRefs_MultipleProviders(t *testing.T) {
	spec := &v1alpha2.LlamaStackDistributionSpec{
		Providers: &v1alpha2.ProvidersSpec{
			Inference: []v1alpha2.ProviderConfig{
				{
					ID:       "primary",
					Provider: "vllm",
					APIKey:   &v1alpha2.SecretKeyRef{Name: "vllm-creds", Key: "token"},
				},
				{
					ID:       "fallback",
					Provider: "ollama",
					APIKey:   &v1alpha2.SecretKeyRef{Name: "ollama-creds", Key: "key"},
				},
			},
		},
	}

	envVars := CollectSecretRefs(spec)
	require.Len(t, envVars, 2)
	assert.Equal(t, "LLSD_PRIMARY_API_KEY", envVars[0].Name)
	assert.Equal(t, "LLSD_FALLBACK_API_KEY", envVars[1].Name)
}

func TestCollectSecretRefs_NoSecrets(t *testing.T) {
	spec := &v1alpha2.LlamaStackDistributionSpec{
		Providers: &v1alpha2.ProvidersSpec{
			Inference: []v1alpha2.ProviderConfig{
				{Provider: "vllm", Endpoint: "http://vllm:8000"},
			},
		},
	}

	envVars := CollectSecretRefs(spec)
	assert.Empty(t, envVars)
}

func TestNormalizeEnvVarSegment(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"vllm", "VLLM"},
		{"vllm-primary", "VLLM_PRIMARY"},
		{"api_key", "API_KEY"},
		{"host", "HOST"},
		{"my-long-name", "MY_LONG_NAME"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.want, normalizeEnvVarSegment(tt.input))
		})
	}
}
