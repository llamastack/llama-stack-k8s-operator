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
	"context"
	"testing"

	v1alpha2 "github.com/llamastack/llama-stack-k8s-operator/api/v1alpha2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
)

func TestDetectConfigVersion(t *testing.T) {
	tests := []struct {
		name    string
		config  map[string]interface{}
		want    int
		wantErr bool
	}{
		{
			name:   "version 2",
			config: map[string]interface{}{"version": 2},
			want:   2,
		},
		{
			name:    "missing version field",
			config:  map[string]interface{}{"apis": []string{"inference"}},
			wantErr: true,
		},
		{
			name:   "version as float64 (from JSON unmarshal)",
			config: map[string]interface{}{"version": float64(2)},
			want:   2,
		},
		{
			name:   "version 99 (parses but doesn't validate)",
			config: map[string]interface{}{"version": 99},
			want:   99,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := DetectConfigVersion(tt.config)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestComputeContentHash(t *testing.T) {
	hash1 := ComputeContentHash("hello world")
	hash2 := ComputeContentHash("hello world")
	hash3 := ComputeContentHash("different content")

	assert.Equal(t, hash1, hash2, "same input should produce same hash")
	assert.NotEqual(t, hash1, hash3, "different input should produce different hash")
	assert.Len(t, hash1, 64, "SHA256 hex digest should be 64 characters")
}

func TestApplyDisabledAPIs(t *testing.T) {
	base := &BaseConfig{
		Version: 2,
		APIs:    []string{"inference", "safety", "agents", "telemetry"},
		Providers: map[string]interface{}{
			"inference": []interface{}{map[string]interface{}{"provider_id": "vllm"}},
			"safety":    []interface{}{map[string]interface{}{"provider_id": "guard"}},
			"agents":    []interface{}{map[string]interface{}{"provider_id": "agent"}},
		},
	}

	result := ApplyDisabledAPIs(base, []string{"safety", "agents"})

	assert.Equal(t, []string{"inference", "telemetry"}, result.APIs)
	assert.Contains(t, result.Providers, "inference")
	assert.NotContains(t, result.Providers, "safety")
	assert.NotContains(t, result.Providers, "agents")
}

func TestRenderConfigYAML(t *testing.T) {
	base := &BaseConfig{
		Version: 2,
		APIs:    []string{"inference"},
		Providers: map[string]interface{}{
			"inference": []interface{}{
				map[string]interface{}{
					"provider_id":   "vllm",
					"provider_type": "remote::vllm",
					"config":        map[string]interface{}{"url": "http://vllm:8000"},
				},
			},
		},
	}

	yaml, err := RenderConfigYAML(base)
	require.NoError(t, err)
	assert.Contains(t, yaml, "version: 2")
	assert.Contains(t, yaml, "inference")
	assert.Contains(t, yaml, "provider_id: vllm")
}

func TestResolverLoadEmbeddedConfig(t *testing.T) {
	resolver := NewBaseConfigResolver(nil, nil)

	tests := []struct {
		name    string
		dist    string
		wantErr bool
	}{
		{name: "starter", dist: "starter"},
		{name: "postgres-demo", dist: "postgres-demo"},
		{name: "unknown", dist: "nonexistent-distro", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := resolver.loadEmbeddedConfig(tt.dist)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.NotNil(t, config)
			assert.Positive(t, config.Version)
			assert.NotEmpty(t, config.APIs)
		})
	}
}

func TestEmbeddedDistributionNames(t *testing.T) {
	names, err := EmbeddedDistributionNames()
	require.NoError(t, err)
	assert.Contains(t, names, "starter")
	assert.Contains(t, names, "postgres-demo")
}

func TestNormalizeProviderType(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"vllm", "remote::vllm"},
		{"remote::vllm", "remote::vllm"},
		{"inline::meta-reference", "inline::meta-reference"},
		{"pgvector", "remote::pgvector"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.want, NormalizeProviderType(tt.input))
		})
	}
}

func TestGenerateProviderID(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"remote::vllm", "vllm"},
		{"inline::meta-reference", "meta-reference"},
		{"vllm", "vllm"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.want, GenerateProviderID(tt.input))
		})
	}
}

func TestParsePolymorphicProvider(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantLen int
		wantErr bool
	}{
		{
			name:    "single provider object",
			raw:     `{"provider": "vllm", "endpoint": "http://vllm:8000"}`,
			wantLen: 1,
		},
		{
			name:    "array of providers",
			raw:     `[{"id": "vllm-1", "provider": "vllm"}, {"id": "vllm-2", "provider": "vllm"}]`,
			wantLen: 2,
		},
		{
			name:    "invalid json",
			raw:     `not json`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := &apiextensionsv1.JSON{Raw: []byte(tt.raw)}
			providers, err := ParsePolymorphicProvider(raw)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Len(t, providers, tt.wantLen)
		})
	}
}

func TestParsePolymorphicModel(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		want    string
		wantErr bool
	}{
		{
			name: "string model name",
			raw:  `"llama3.2-8b"`,
			want: "llama3.2-8b",
		},
		{
			name: "object model config",
			raw:  `{"name": "llama3.2-8b", "provider": "vllm-1"}`,
			want: "llama3.2-8b",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := &apiextensionsv1.JSON{Raw: []byte(tt.raw)}
			mc, err := ParsePolymorphicModel(raw)
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, mc.Name)
		})
	}
}

func TestExpandProviders(t *testing.T) {
	raw := &apiextensionsv1.JSON{Raw: []byte(`{"provider": "vllm", "endpoint": "http://vllm:8000"}`)}
	spec := &v1alpha2.ProvidersSpec{
		Inference: raw,
	}

	providers, count, err := ExpandProviders(spec, nil)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
	assert.Contains(t, providers, "inference")
	assert.Len(t, providers["inference"], 1)
	assert.Equal(t, "remote::vllm", providers["inference"][0].ProviderType)
}

func TestExpandResources(t *testing.T) {
	base := &BaseConfig{
		Version: 2,
		APIs:    []string{"inference"},
		Providers: map[string]interface{}{
			"inference": []interface{}{
				map[string]interface{}{
					"provider_id":   "vllm",
					"provider_type": "remote::vllm",
				},
			},
		},
	}

	models := []apiextensionsv1.JSON{
		{Raw: []byte(`"llama3.2-8b"`)},
		{Raw: []byte(`{"name": "llama3.2-70b", "provider": "vllm"}`)},
	}

	spec := &v1alpha2.ResourcesSpec{
		Models: models,
	}

	userProviders := map[string][]ProviderEntry{
		"inference": {{ProviderID: "vllm", ProviderType: "remote::vllm"}},
	}

	expandedModels, _, _, count, err := ExpandResources(spec, userProviders, base)
	require.NoError(t, err)
	assert.Equal(t, 2, count)
	assert.Len(t, expandedModels, 2)
}

func TestResolveSecrets(t *testing.T) {
	spec := &v1alpha2.LlamaStackDistributionSpec{
		Distribution: v1alpha2.DistributionSpec{Name: "starter"},
		Providers: &v1alpha2.ProvidersSpec{
			Inference: &apiextensionsv1.JSON{Raw: []byte(`{
				"provider": "vllm",
				"endpoint": "http://vllm:8000",
				"apiKey": {"name": "my-secret", "key": "api-key"}
			}`)},
		},
	}

	resolution, err := ResolveSecrets(spec)
	require.NoError(t, err)
	assert.NotEmpty(t, resolution.EnvVars)
	assert.NotEmpty(t, resolution.Substitutions)
}

func TestExpandKVStorage(t *testing.T) {
	tests := []struct {
		name string
		kv   *v1alpha2.KVStorageSpec
		want string
	}{
		{
			name: "sqlite",
			kv:   &v1alpha2.KVStorageSpec{Type: "sqlite"},
			want: "sqlite",
		},
		{
			name: "redis with endpoint",
			kv:   &v1alpha2.KVStorageSpec{Type: "redis", Endpoint: "redis://localhost:6379"},
			want: "redis",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExpandKVStorage(tt.kv, nil)
			assert.Equal(t, tt.want, result["type"])
		})
	}
}

func TestExpandSQLStorage(t *testing.T) {
	tests := []struct {
		name string
		sql  *v1alpha2.SQLStorageSpec
		want string
	}{
		{
			name: "sqlite",
			sql:  &v1alpha2.SQLStorageSpec{Type: "sqlite"},
			want: "sqlite",
		},
		{
			name: "postgres with connection string secret",
			sql: &v1alpha2.SQLStorageSpec{
				Type:             "postgres",
				ConnectionString: &v1alpha2.SecretKeyRef{Name: "pg-secret", Key: "dsn"},
			},
			want: "postgres",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExpandSQLStorage(tt.sql, nil)
			assert.Equal(t, tt.want, result["type"])
		})
	}
}

func TestGenerateEnvVarName(t *testing.T) {
	tests := []struct {
		providerID string
		field      string
		want       string
	}{
		{"vllm", "API_KEY", "LLSD_VLLM_API_KEY"},
		{"my-provider", "endpoint", "LLSD_MY_PROVIDER_ENDPOINT"},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			assert.Equal(t, tt.want, GenerateEnvVarName(tt.providerID, tt.field))
		})
	}
}

func TestGenerateConfigDeterministic(t *testing.T) {
	distImages := map[string]string{
		"starter": "quay.io/llamastack/distribution-starter:latest",
	}

	spec := &v1alpha2.LlamaStackDistributionSpec{
		Distribution: v1alpha2.DistributionSpec{Name: "starter"},
		Providers: &v1alpha2.ProvidersSpec{
			Inference: &apiextensionsv1.JSON{Raw: []byte(`{"provider": "vllm", "endpoint": "http://vllm:8000"}`)},
		},
		Resources: &v1alpha2.ResourcesSpec{
			Models: []apiextensionsv1.JSON{
				{Raw: []byte(`"llama3.2-8b"`)},
			},
		},
	}

	resolver := NewBaseConfigResolver(distImages, nil)

	gen1, img1, err1 := GenerateConfig(context.Background(), spec, resolver)
	require.NoError(t, err1)

	gen2, img2, err2 := GenerateConfig(context.Background(), spec, resolver)
	require.NoError(t, err2)

	assert.Equal(t, gen1.ContentHash, gen2.ContentHash, "deterministic: same input should produce same hash")
	assert.Equal(t, gen1.ConfigYAML, gen2.ConfigYAML, "deterministic: same input should produce same YAML")
	assert.Equal(t, img1, img2)
}

func TestMergeExternalProviders(t *testing.T) {
	base := &BaseConfig{
		Version: 2,
		APIs:    []string{"inference"},
		Providers: map[string]interface{}{
			"inference": []interface{}{
				map[string]interface{}{"provider_id": "vllm"},
			},
		},
	}

	external := map[string]interface{}{
		"safety": []interface{}{
			map[string]interface{}{"provider_id": "custom-safety"},
		},
	}

	result, warnings := MergeExternalProviders(base, external)
	assert.Contains(t, result.Providers, "safety")
	assert.Empty(t, warnings, "no conflicts expected")
}

func TestMergeExternalProvidersConflict(t *testing.T) {
	base := &BaseConfig{
		Version: 2,
		APIs:    []string{"inference"},
		Providers: map[string]interface{}{
			"inference": []interface{}{
				map[string]interface{}{"provider_id": "vllm", "provider_type": "remote::vllm"},
			},
		},
	}

	external := map[string]interface{}{
		"inference": []interface{}{
			map[string]interface{}{"provider_id": "vllm", "provider_type": "remote::tgi"},
		},
	}

	result, warnings := MergeExternalProviders(base, external)
	assert.Len(t, warnings, 1, "should warn about overridden provider")
	assert.Contains(t, result.Providers, "inference")
}

func TestValidateConfigVersion(t *testing.T) {
	require.NoError(t, ValidateConfigVersion(1))
	require.NoError(t, ValidateConfigVersion(2))
	require.Error(t, ValidateConfigVersion(0))
	require.Error(t, ValidateConfigVersion(99))
}
