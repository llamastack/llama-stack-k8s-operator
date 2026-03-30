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
	"gopkg.in/yaml.v3"
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
		APIs:    []string{"inference", "agents", "telemetry"},
		Providers: map[string]interface{}{
			"inference": []interface{}{map[string]interface{}{"provider_id": "vllm"}},
			"agents":    []interface{}{map[string]interface{}{"provider_id": "agent"}},
		},
	}

	ApplyDisabledAPIs(base, []string{"agents"})

	assert.Equal(t, []string{"inference", "telemetry"}, base.APIs)
	assert.Contains(t, base.Providers, "inference")
	assert.NotContains(t, base.Providers, "agents")
}

func TestApplyDisabledAPIsSnakeCaseEnum(t *testing.T) {
	base := &BaseConfig{
		Version: 2,
		APIs:    []string{"inference", "vector_io", "tool_runtime"},
		Providers: map[string]interface{}{
			"inference":    []interface{}{map[string]interface{}{"provider_id": "vllm"}},
			"vector_io":    []interface{}{map[string]interface{}{"provider_id": "pgvector"}},
			"tool_runtime": []interface{}{map[string]interface{}{"provider_id": "brave"}},
		},
	}

	ApplyDisabledAPIs(base, []string{"vector_io"})

	assert.Equal(t, []string{"inference", "tool_runtime"}, base.APIs,
		"disabled names use snake_case as enforced by CRD enum")
	assert.NotContains(t, base.Providers, "vector_io")
	assert.Contains(t, base.Providers, "inference")
	assert.Contains(t, base.Providers, "tool_runtime")
}

func TestApplyDisabledAPIsCleanupResources(t *testing.T) {
	base := &BaseConfig{
		Version: 2,
		APIs:    []string{"inference", "tool_runtime"},
		Providers: map[string]interface{}{
			"inference":    []interface{}{map[string]interface{}{"provider_id": "vllm"}},
			"tool_runtime": []interface{}{map[string]interface{}{"provider_id": "rag"}},
		},
		ToolGroups: []map[string]interface{}{{"toolgroup_id": "websearch", "provider_id": "rag"}},
	}

	ApplyDisabledAPIs(base, []string{"tool_runtime"})

	assert.Nil(t, base.ToolGroups, "disabling tool_runtime must remove tool group registrations")
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

func TestExpandProviders(t *testing.T) {
	spec := &v1alpha2.ProvidersSpec{
		Inference: []v1alpha2.ProviderConfig{
			{Provider: "vllm", Endpoint: "http://vllm:8000"},
		},
	}

	providers, count, err := ExpandProviders(spec, nil)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
	assert.Contains(t, providers, "inference")
	assert.Len(t, providers["inference"], 1)
	assert.Equal(t, "remote::vllm", providers["inference"][0].ProviderType)
	assert.Equal(t, "http://vllm:8000", providers["inference"][0].Config["base_url"],
		"endpoint must map to config.base_url")
}

func TestExpandProviderWithSecretRefs(t *testing.T) {
	spec := &v1alpha2.ProvidersSpec{
		VectorIo: []v1alpha2.ProviderConfig{
			{
				ID:       "pgvector",
				Provider: "pgvector",
				SecretRefs: map[string]v1alpha2.SecretKeyRef{
					"host": {Name: "pg-creds", Key: "host"},
				},
				Settings: &apiextensionsv1.JSON{Raw: []byte(`{"port": 5432}`)},
			},
		},
	}

	subs := map[string]string{
		"pgvector:host": "${env.LLSD_PGVECTOR_HOST}",
	}

	providers, count, err := ExpandProviders(spec, subs)
	require.NoError(t, err)
	assert.Equal(t, 1, count)
	assert.Contains(t, providers, "vector_io")

	entry := providers["vector_io"][0]
	assert.Equal(t, "${env.LLSD_PGVECTOR_HOST}", entry.Config["host"],
		"secretRefs.host must be mapped to config.host with env var substitution")
	portVal, ok := entry.Config["port"].(float64)
	require.True(t, ok, "port must be a number")
	assert.Equal(t, 5432, int(portVal),
		"settings must be passed through as-is")
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

	spec := &v1alpha2.ResourcesSpec{
		Models: []v1alpha2.ModelConfig{
			{Name: "llama3.2-8b"},
			{Name: "llama3.2-70b", Provider: "vllm"},
		},
	}

	userProviders := map[string][]ProviderEntry{
		"inference": {{ProviderID: "vllm", ProviderType: "remote::vllm"}},
	}

	expandedModels, _, count, err := ExpandResources(spec, userProviders, base)
	require.NoError(t, err)
	assert.Equal(t, 2, count)
	assert.Len(t, expandedModels, 2)
}

func TestResolveSecrets(t *testing.T) {
	spec := &v1alpha2.LlamaStackDistributionSpec{
		Distribution: v1alpha2.DistributionSpec{Name: "starter"},
		Providers: &v1alpha2.ProvidersSpec{
			Inference: []v1alpha2.ProviderConfig{
				{
					Provider: "vllm",
					Endpoint: "http://vllm:8000",
					SecretRefs: map[string]v1alpha2.SecretKeyRef{
						"api_key": {Name: "my-secret", Key: "api-key"},
					},
				},
			},
		},
	}

	resolution, err := ResolveSecrets(spec)
	require.NoError(t, err)
	assert.NotEmpty(t, resolution.EnvVars)
	assert.NotEmpty(t, resolution.Substitutions)
}

func TestSettingsNeverResolvedAsSecrets(t *testing.T) {
	spec := &v1alpha2.LlamaStackDistributionSpec{
		Distribution: v1alpha2.DistributionSpec{Name: "starter"},
		Providers: &v1alpha2.ProvidersSpec{
			VectorIo: []v1alpha2.ProviderConfig{
				{
					Provider: "pgvector",
					Settings: &apiextensionsv1.JSON{Raw: []byte(`{
						"password": {"secretKeyRef": {"name": "pg-secret", "key": "password"}},
						"database": {"name": "mydb", "key": "primary"}
					}`)},
				},
			},
		},
	}

	resolution, err := ResolveSecrets(spec)
	require.NoError(t, err)
	assert.Empty(t, resolution.EnvVars,
		"settings must never be scanned for secrets (FR-005); use secretRefs instead")
	assert.Empty(t, resolution.Substitutions)
}

func TestSecretRefsFieldResolved(t *testing.T) {
	spec := &v1alpha2.LlamaStackDistributionSpec{
		Distribution: v1alpha2.DistributionSpec{Name: "starter"},
		Providers: &v1alpha2.ProvidersSpec{
			VectorIo: []v1alpha2.ProviderConfig{
				{
					Provider: "pgvector",
					SecretRefs: map[string]v1alpha2.SecretKeyRef{
						"host":     {Name: "pg-secret", Key: "host"},
						"password": {Name: "pg-secret", Key: "password"},
					},
				},
			},
		},
	}

	resolution, err := ResolveSecrets(spec)
	require.NoError(t, err)
	assert.Len(t, resolution.EnvVars, 2,
		"secretRefs entries must be resolved to env vars")
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
		{"vllm", "api_key", "LLSD_VLLM_API_KEY"},
		{"my-provider", "endpoint", "LLSD_MY_PROVIDER_ENDPOINT"},
		{"remote::vllm", "host", "LLSD_REMOTE_VLLM_HOST"},
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
			Inference: []v1alpha2.ProviderConfig{
				{Provider: "vllm", Endpoint: "http://vllm:8000"},
			},
		},
		Resources: &v1alpha2.ResourcesSpec{
			Models: []v1alpha2.ModelConfig{
				{Name: "llama3.2-8b"},
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
		"telemetry": []interface{}{
			map[string]interface{}{"provider_id": "custom-otel"},
		},
	}

	result, warnings := MergeExternalProviders(base, external)
	assert.Contains(t, result.Providers, "telemetry")
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

func TestGenerateConfigPreservesExtraFields(t *testing.T) {
	distImages := map[string]string{
		"starter": "quay.io/llamastack/distribution-starter:latest",
	}

	spec := &v1alpha2.LlamaStackDistributionSpec{
		Distribution: v1alpha2.DistributionSpec{Name: "starter"},
		Providers: &v1alpha2.ProvidersSpec{
			Inference: []v1alpha2.ProviderConfig{
				{Provider: "ollama", Endpoint: "http://ollama:11434"},
			},
		},
		Resources: &v1alpha2.ResourcesSpec{
			Models: []v1alpha2.ModelConfig{{Name: "llama3.2:1b"}},
		},
	}

	resolver := NewBaseConfigResolver(distImages, nil)
	gen, _, err := GenerateConfig(context.Background(), spec, resolver)
	require.NoError(t, err)

	assert.Contains(t, gen.ConfigYAML, "distro_name: starter",
		"generated config must include distro_name from embedded base config")
	assert.Contains(t, gen.ConfigYAML, "storage:",
		"generated config must include storage section from embedded base config")
	assert.Contains(t, gen.ConfigYAML, "registered_resources:",
		"generated config must include registered_resources from embedded base config")
}

func TestModelsAppearInRegisteredResources(t *testing.T) {
	distImages := map[string]string{
		"starter": "quay.io/llamastack/distribution-starter:latest",
	}

	spec := &v1alpha2.LlamaStackDistributionSpec{
		Distribution: v1alpha2.DistributionSpec{Name: "starter"},
		Providers: &v1alpha2.ProvidersSpec{
			Inference: []v1alpha2.ProviderConfig{
				{Provider: "ollama", Endpoint: "http://ollama:11434"},
			},
		},
		Resources: &v1alpha2.ResourcesSpec{
			Models: []v1alpha2.ModelConfig{
				{Name: "llama3.2:1b"},
			},
		},
	}

	resolver := NewBaseConfigResolver(distImages, nil)
	gen, _, err := GenerateConfig(context.Background(), spec, resolver)
	require.NoError(t, err)

	var parsed map[string]interface{}
	err = yaml.Unmarshal([]byte(gen.ConfigYAML), &parsed)
	require.NoError(t, err)

	_, hasTopLevelModels := parsed["models"]
	assert.False(t, hasTopLevelModels, "models must not appear at the top level; they belong under registered_resources")

	rr, ok := parsed["registered_resources"].(map[string]interface{})
	require.True(t, ok, "registered_resources must be a map")

	models, ok := rr["models"].([]interface{})
	require.True(t, ok, "registered_resources.models must be a list")

	var foundInference bool
	for _, m := range models {
		mm, ok := m.(map[string]interface{})
		if !ok {
			continue
		}
		if mm["model_id"] == "llama3.2:1b" {
			foundInference = true
		}
	}
	assert.True(t, foundInference, "inference model from CR must be in registered_resources.models")
}

func TestOverrideServerPort(t *testing.T) {
	tests := []struct {
		name          string
		initialServer map[string]interface{}
		port          int32
		wantPort      int32
	}{
		{
			name:          "nil server map is initialised",
			initialServer: nil,
			port:          8321,
			wantPort:      8321,
		},
		{
			name:          "existing server map is preserved",
			initialServer: map[string]interface{}{"tls_certfile": "/certs/tls.crt"},
			port:          9000,
			wantPort:      9000,
		},
		{
			name:          "existing port is overwritten",
			initialServer: map[string]interface{}{"port": int32(5000)},
			port:          8080,
			wantPort:      8080,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &BaseConfig{Server: tt.initialServer}
			overrideServerPort(config, tt.port)
			assert.Equal(t, tt.wantPort, config.Server["port"])

			if tt.initialServer != nil {
				for k, v := range tt.initialServer {
					if k == "port" {
						continue
					}
					assert.Equal(t, v, config.Server[k],
						"existing key %q should be preserved", k)
				}
			}
		})
	}
}

func TestApplyNetworkingOverrides(t *testing.T) {
	tests := []struct {
		name     string
		spec     v1alpha2.LlamaStackDistributionSpec
		baseAPIs []string
		wantAPIs []string
		wantPort interface{}
	}{
		{
			name: "disabled APIs are removed",
			spec: v1alpha2.LlamaStackDistributionSpec{
				Disabled: []string{"agents"},
			},
			baseAPIs: []string{"inference", "agents", "telemetry"},
			wantAPIs: []string{"inference", "telemetry"},
			wantPort: nil,
		},
		{
			name: "port is set from networking spec",
			spec: v1alpha2.LlamaStackDistributionSpec{
				Networking: &v1alpha2.NetworkingSpec{Port: 9000},
			},
			baseAPIs: []string{"inference"},
			wantAPIs: []string{"inference"},
			wantPort: int32(9000),
		},
		{
			name: "both disabled APIs and port applied together",
			spec: v1alpha2.LlamaStackDistributionSpec{
				Disabled:   []string{"telemetry"},
				Networking: &v1alpha2.NetworkingSpec{Port: 8321},
			},
			baseAPIs: []string{"inference", "telemetry"},
			wantAPIs: []string{"inference"},
			wantPort: int32(8321),
		},
		{
			name:     "no-op when nothing is specified",
			spec:     v1alpha2.LlamaStackDistributionSpec{},
			baseAPIs: []string{"inference", "telemetry"},
			wantAPIs: []string{"inference", "telemetry"},
			wantPort: nil,
		},
		{
			name: "zero port is ignored",
			spec: v1alpha2.LlamaStackDistributionSpec{
				Networking: &v1alpha2.NetworkingSpec{Port: 0},
			},
			baseAPIs: []string{"inference"},
			wantAPIs: []string{"inference"},
			wantPort: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := &BaseConfig{
				Version: 2,
				APIs:    append([]string{}, tt.baseAPIs...),
				Providers: map[string]interface{}{
					"inference": []interface{}{},
					"agents":    []interface{}{},
					"telemetry": []interface{}{},
				},
			}

			applyNetworkingOverrides(&tt.spec, config)
			assert.Equal(t, tt.wantAPIs, config.APIs)

			if tt.wantPort != nil {
				require.NotNil(t, config.Server)
				assert.Equal(t, tt.wantPort, config.Server["port"])
			} else if config.Server != nil {
				_, hasPort := config.Server["port"]
				assert.False(t, hasPort, "port should not be set")
			}
		})
	}
}

func TestSecretResolverProviderIDMatchesConfigPipeline(t *testing.T) {
	spec := &v1alpha2.LlamaStackDistributionSpec{
		Distribution: v1alpha2.DistributionSpec{Name: "starter"},
		Providers: &v1alpha2.ProvidersSpec{
			Inference: []v1alpha2.ProviderConfig{
				{
					Provider: "remote::vllm",
					Endpoint: "http://vllm:8000",
					SecretRefs: map[string]v1alpha2.SecretKeyRef{
						"api_key": {Name: "vllm-secret", Key: "api-key"},
					},
				},
			},
		},
	}

	resolution, err := ResolveSecrets(spec)
	require.NoError(t, err)
	require.Len(t, resolution.EnvVars, 1)

	envVar := resolution.EnvVars[0]
	assert.Equal(t, "LLSD_VLLM_API_KEY", envVar.Name,
		"env var name must use normalized provider ID (strip remote:: prefix)")
	assert.Equal(t, "vllm-secret", envVar.ValueFrom.SecretKeyRef.Name)

	assert.Contains(t, resolution.Substitutions, "vllm:api_key",
		"substitution key must use normalized provider ID matching the config pipeline")
}

func TestExpandToolsRegistration(t *testing.T) {
	userProviders := map[string][]ProviderEntry{
		"tool_runtime": {{ProviderID: "rag-runtime", ProviderType: "inline::rag-runtime"}},
	}

	tools, err := expandTools(
		[]string{"builtin::websearch", "builtin::rag"},
		userProviders,
		nil,
	)
	require.NoError(t, err)
	require.Len(t, tools, 2)
	assert.Equal(t, "builtin::websearch", tools[0]["toolgroup_id"])
	assert.Equal(t, "rag-runtime", tools[0]["provider_id"])
	assert.Equal(t, "builtin::rag", tools[1]["toolgroup_id"])
	assert.Equal(t, "rag-runtime", tools[1]["provider_id"])
}

func TestExpandToolsFallsBackToBaseProvider(t *testing.T) {
	base := &BaseConfig{
		Providers: map[string]interface{}{
			"tool_runtime": []interface{}{
				map[string]interface{}{"provider_id": "brave-search"},
			},
		},
	}

	tools, err := expandTools([]string{"builtin::websearch"}, nil, base)
	require.NoError(t, err)
	require.Len(t, tools, 1)
	assert.Equal(t, "brave-search", tools[0]["provider_id"],
		"should fall back to base config tool_runtime provider")
}

func TestExpandToolsErrorsWithNoProvider(t *testing.T) {
	_, err := expandTools([]string{"builtin::websearch"}, nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "toolRuntime provider")
}

func TestSecretRefsEnvVarNaming(t *testing.T) {
	spec := &v1alpha2.LlamaStackDistributionSpec{
		Distribution: v1alpha2.DistributionSpec{Name: "starter"},
		Providers: &v1alpha2.ProvidersSpec{
			VectorIo: []v1alpha2.ProviderConfig{
				{
					ID:       "pgvector",
					Provider: "pgvector",
					SecretRefs: map[string]v1alpha2.SecretKeyRef{
						"host": {Name: "pg-creds", Key: "host"},
					},
				},
			},
		},
	}

	resolution, err := ResolveSecrets(spec)
	require.NoError(t, err)
	require.Len(t, resolution.EnvVars, 1)
	assert.Equal(t, "LLSD_PGVECTOR_HOST", resolution.EnvVars[0].Name,
		"secretRefs key must map to env var: LLSD_<PROVIDER_ID>_<KEY>")
	assert.Equal(t, "pg-creds", resolution.EnvVars[0].ValueFrom.SecretKeyRef.Name)
	assert.Contains(t, resolution.Substitutions, "pgvector:host",
		"substitution key must use provider ID and secretRefs key")
	assert.Equal(t, "${env.LLSD_PGVECTOR_HOST}", resolution.Substitutions["pgvector:host"])
}

func TestGenerateConfigContentHashDeterminesRestart(t *testing.T) {
	distImages := map[string]string{
		"starter": "quay.io/llamastack/distribution-starter:latest",
	}

	spec := &v1alpha2.LlamaStackDistributionSpec{
		Distribution: v1alpha2.DistributionSpec{Name: "starter"},
		Providers: &v1alpha2.ProvidersSpec{
			Inference: []v1alpha2.ProviderConfig{
				{Provider: "vllm", Endpoint: "http://vllm:8000"},
			},
		},
	}

	resolver := NewBaseConfigResolver(distImages, nil)

	gen1, _, err := GenerateConfig(context.Background(), spec, resolver)
	require.NoError(t, err)

	gen2, _, err := GenerateConfig(context.Background(), spec, resolver)
	require.NoError(t, err)
	assert.Equal(t, gen1.ContentHash, gen2.ContentHash,
		"identical spec must produce identical content hash (FR-096: skip restart)")

	specChanged := &v1alpha2.LlamaStackDistributionSpec{
		Distribution: v1alpha2.DistributionSpec{Name: "starter"},
		Providers: &v1alpha2.ProvidersSpec{
			Inference: []v1alpha2.ProviderConfig{
				{Provider: "vllm", Endpoint: "http://vllm:9999"},
			},
		},
	}

	gen3, _, err := GenerateConfig(context.Background(), specChanged, resolver)
	require.NoError(t, err)
	assert.NotEqual(t, gen1.ContentHash, gen3.ContentHash,
		"changed spec must produce different content hash (triggers restart)")
}

func TestResolveImageAndConfigTogether(t *testing.T) {
	distImages := map[string]string{
		"starter": "quay.io/llamastack/distribution-starter:latest",
	}

	spec := &v1alpha2.LlamaStackDistributionSpec{
		Distribution: v1alpha2.DistributionSpec{Name: "starter"},
		Providers: &v1alpha2.ProvidersSpec{
			Inference: []v1alpha2.ProviderConfig{
				{Provider: "vllm", Endpoint: "http://vllm:8000"},
			},
		},
	}

	resolver := NewBaseConfigResolver(distImages, nil)

	gen, resolvedImage, err := GenerateConfig(context.Background(), spec, resolver)
	require.NoError(t, err)
	assert.NotEmpty(t, gen.ConfigYAML, "config must be generated")
	assert.NotEmpty(t, gen.ContentHash, "content hash must be set")
	assert.Equal(t, "quay.io/llamastack/distribution-starter:latest", resolvedImage,
		"image must be resolved from the same pipeline as config (FR-100: atomic update)")
}

func TestEmbeddingProviderPreservedAfterMerge(t *testing.T) {
	distImages := map[string]string{
		"starter": "quay.io/llamastack/distribution-starter:latest",
	}

	spec := &v1alpha2.LlamaStackDistributionSpec{
		Distribution: v1alpha2.DistributionSpec{Name: "starter"},
		Providers: &v1alpha2.ProvidersSpec{
			Inference: []v1alpha2.ProviderConfig{
				{Provider: "ollama", Endpoint: "http://ollama:11434"},
			},
		},
	}

	resolver := NewBaseConfigResolver(distImages, nil)
	gen, _, err := GenerateConfig(context.Background(), spec, resolver)
	require.NoError(t, err)

	assert.Contains(t, gen.ConfigYAML, "sentence-transformers",
		"sentence-transformers provider must be preserved for embedding support")
}

func TestResourceMergePreservesBaseModels(t *testing.T) {
	baseModels := []map[string]interface{}{
		{"model_id": "base-model", "provider_id": "vllm"},
	}
	incomingModels := []map[string]interface{}{
		{"model_id": "new-model", "provider_id": "vllm"},
	}

	result := mergeResourceMaps(baseModels, incomingModels, "model_id")
	assert.Len(t, result, 2, "base model must be preserved when CR adds a new model")
}

func TestResourceMergeReplacesMatchingIDs(t *testing.T) {
	baseModels := []map[string]interface{}{
		{"model_id": "shared-model", "provider_id": "old-provider"},
	}
	incomingModels := []map[string]interface{}{
		{"model_id": "shared-model", "provider_id": "new-provider"},
	}

	result := mergeResourceMaps(baseModels, incomingModels, "model_id")
	assert.Len(t, result, 1, "matching IDs should replace, not duplicate")
	assert.Equal(t, "new-provider", result[0]["provider_id"])
}

func TestSettingsDoNotOverrideStructuredFields(t *testing.T) {
	spec := &v1alpha2.ProvidersSpec{
		Inference: []v1alpha2.ProviderConfig{
			{
				Provider: "vllm",
				Endpoint: "http://vllm:8000",
				Settings: &apiextensionsv1.JSON{Raw: []byte(`{"base_url": "http://should-lose:9999", "custom_key": "custom_val"}`)},
			},
		},
	}

	providers, _, err := ExpandProviders(spec, nil)
	require.NoError(t, err)

	cfg := providers["inference"][0].Config
	assert.Equal(t, "http://vllm:8000", cfg["base_url"],
		"endpoint must override settings.base_url because structured fields take precedence")
	assert.Equal(t, "custom_val", cfg["custom_key"],
		"non-conflicting settings keys must be preserved")
}

func TestExpandStorageV2UpdatesBackends(t *testing.T) {
	config := &BaseConfig{
		Version: 2,
		Extra: map[string]interface{}{
			"storage": map[string]interface{}{
				"backends": map[string]interface{}{
					"kv_default": map[string]interface{}{
						"type":    "kv_sqlite",
						"db_path": "/old/path/kvstore.db",
					},
					"sql_default": map[string]interface{}{
						"type":    "sql_sqlite",
						"db_path": "/old/path/sqlstore.db",
					},
				},
			},
		},
	}

	spec := &v1alpha2.StateStorageSpec{
		SQL: &v1alpha2.SQLStorageSpec{
			Type:             "postgres",
			ConnectionString: &v1alpha2.SecretKeyRef{Name: "pg-secret", Key: "dsn"},
		},
	}

	subs := map[string]string{
		"sql-postgres:connectionString": "${env.LLSD_SQL_POSTGRES_CONNECTIONSTRING}",
	}

	_, err := ExpandStorage(spec, config, subs)
	require.NoError(t, err)

	storage, ok := config.Extra["storage"].(map[string]interface{})
	require.True(t, ok, "Extra[storage] must be a map")
	backends, ok := storage["backends"].(map[string]interface{})
	require.True(t, ok, "storage[backends] must be a map")

	sqlBackend, ok := backends["sql_default"].(map[string]interface{})
	require.True(t, ok, "backends[sql_default] must be a map")
	assert.Equal(t, "sql_postgres", sqlBackend["type"],
		"v2 backend type must use sql_postgres, not postgres")
	assert.Equal(t, "${env.LLSD_SQL_POSTGRES_CONNECTIONSTRING}", sqlBackend["connection_string"])
	_, hasDBPath := sqlBackend["db_path"]
	assert.False(t, hasDBPath, "db_path from old sqlite config must not leak into postgres")

	assert.Nil(t, config.InferenceStore,
		"v2 storage must not create top-level inference_store (would produce orphan keys)")
}

func TestExpandStorageV1FallsBackToTopLevelStores(t *testing.T) {
	config := &BaseConfig{
		Version:       1,
		MetadataStore: map[string]interface{}{"type": "sqlite"},
	}

	spec := &v1alpha2.StateStorageSpec{
		KV: &v1alpha2.KVStorageSpec{
			Type:     "redis",
			Endpoint: "redis://localhost:6379",
		},
	}

	_, err := ExpandStorage(spec, config, nil)
	require.NoError(t, err)

	assert.Equal(t, "redis", config.MetadataStore["type"],
		"v1 config must continue to use top-level metadata_store")
}

func TestRenderConfigYAMLDoesNotMutateConfig(t *testing.T) {
	config := &BaseConfig{
		Version: 2,
		APIs:    []string{"inference"},
		RegisteredModels: []map[string]interface{}{
			{"model_id": "llama3", "provider_id": "vllm"},
		},
		Extra: map[string]interface{}{
			"registered_resources": map[string]interface{}{
				"models": []interface{}{
					map[string]interface{}{"model_id": "base-model", "provider_id": "base"},
				},
			},
		},
	}

	rr, ok := config.Extra["registered_resources"].(map[string]interface{})
	require.True(t, ok)
	baseModels, ok := rr["models"].([]interface{})
	require.True(t, ok)
	originalLen := len(baseModels)

	_, err := RenderConfigYAML(config)
	require.NoError(t, err)

	rrAfter, ok := config.Extra["registered_resources"].(map[string]interface{})
	require.True(t, ok)
	modelsAfter, ok := rrAfter["models"].([]interface{})
	require.True(t, ok)
	assert.Len(t, modelsAfter, originalLen,
		"RenderConfigYAML must not mutate config.Extra (side-effect-free rendering)")
}
