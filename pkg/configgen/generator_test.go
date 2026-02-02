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

package configgen

import (
	"testing"

	llamav1alpha1 "github.com/llamastack/llama-stack-k8s-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/yaml"
)

func TestGenerator_LoadBaseConfig(t *testing.T) {
	gen := NewGenerator()

	tests := []struct {
		name             string
		distributionName string
		wantErr          bool
		errContains      string
	}{
		{
			name:             "valid distribution loads successfully",
			distributionName: "starter",
			wantErr:          false,
		},
		{
			name:             "unknown distribution returns error",
			distributionName: "nonexistent",
			wantErr:          true,
			errContains:      "failed to find distribution",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := gen.loadBaseConfig(tt.distributionName)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got none")
				} else if tt.errContains != "" && !containsString(err.Error(), tt.errContains) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errContains)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if config == nil {
				t.Error("expected config but got nil")
				return
			}

			// Verify basic structure
			if _, ok := config["providers"]; !ok {
				t.Error("config should have providers section")
			}
		})
	}
}

func TestGenerator_ConfigMapName(t *testing.T) {
	gen := NewGenerator()

	tests := []struct {
		name        string
		llsdName    string
		content     []byte
		wantPrefix  string
		wantHashLen int
	}{
		{
			name:        "generates deterministic name",
			llsdName:    "my-stack",
			content:     []byte("test content"),
			wantPrefix:  "my-stack-config-",
			wantHashLen: 10,
		},
		{
			name:        "same content produces same hash",
			llsdName:    "test",
			content:     []byte("same content"),
			wantPrefix:  "test-config-",
			wantHashLen: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := gen.ConfigMapName(tt.llsdName, tt.content)

			if !hasPrefix(result, tt.wantPrefix) {
				t.Errorf("ConfigMapName() = %q, want prefix %q", result, tt.wantPrefix)
			}

			// Hash should be 10 characters after the prefix
			expectedLen := len(tt.wantPrefix) + tt.wantHashLen
			if len(result) != expectedLen {
				t.Errorf("ConfigMapName() length = %d, want %d", len(result), expectedLen)
			}

			// Same input should produce same output (deterministic)
			result2 := gen.ConfigMapName(tt.llsdName, tt.content)
			if result != result2 {
				t.Errorf("ConfigMapName() not deterministic: %q != %q", result, result2)
			}
		})
	}

	// Different content should produce different hash
	t.Run("different content produces different hash", func(t *testing.T) {
		result1 := gen.ConfigMapName("test", []byte("content1"))
		result2 := gen.ConfigMapName("test", []byte("content2"))
		if result1 == result2 {
			t.Error("different content should produce different hashes")
		}
	})
}

func TestGenerator_SingleProviderExpansion(t *testing.T) {
	gen := NewGenerator()

	instance := &llamav1alpha1.LlamaStackDistribution{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
		Spec: llamav1alpha1.LlamaStackDistributionSpec{
			Server: llamav1alpha1.ServerSpec{
				Distribution: llamav1alpha1.DistributionType{Name: "starter"},
				Providers: &llamav1alpha1.ProvidersSpec{
					Inference: &llamav1alpha1.ProviderConfigOrList{
						Single: &llamav1alpha1.ProviderEntry{
							Provider: "vllm",
							Endpoint: "http://vllm:8000",
						},
					},
				},
			},
		},
	}

	result, err := gen.Generate(instance)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("expected result but got nil")
	}

	if len(result.ConfigYAML) == 0 {
		t.Error("expected non-empty ConfigYAML")
	}

	// Parse the generated config
	var config map[string]interface{}
	if err := yaml.Unmarshal(result.ConfigYAML, &config); err != nil {
		t.Fatalf("failed to parse generated config: %v", err)
	}

	// Verify provider mapping
	providers, ok := config["providers"].(map[string]interface{})
	if !ok {
		t.Fatal("expected providers section")
	}

	inference, ok := providers["inference"].([]interface{})
	if !ok {
		t.Fatal("expected inference providers list")
	}

	if len(inference) != 1 {
		t.Errorf("expected 1 inference provider, got %d", len(inference))
	}

	firstProvider, ok := inference[0].(map[string]interface{})
	if !ok {
		t.Fatal("expected provider to be a map")
	}

	// Verify provider_type has remote:: prefix
	if pt, ok := firstProvider["provider_type"].(string); !ok || pt != "remote::vllm" {
		t.Errorf("expected provider_type 'remote::vllm', got %v", firstProvider["provider_type"])
	}

	// Verify provider_id is auto-generated
	if pid, ok := firstProvider["provider_id"].(string); !ok || pid != "vllm" {
		t.Errorf("expected provider_id 'vllm', got %v", firstProvider["provider_id"])
	}

	// Verify endpoint is mapped to config.url
	providerConfig, ok := firstProvider["config"].(map[string]interface{})
	if !ok {
		t.Fatal("expected config section in provider")
	}

	if url, ok := providerConfig["url"].(string); !ok || url != "http://vllm:8000" {
		t.Errorf("expected config.url 'http://vllm:8000', got %v", providerConfig["url"])
	}
}

func TestGenerator_SecretRefToEnvVar(t *testing.T) {
	gen := NewGenerator()

	instance := &llamav1alpha1.LlamaStackDistribution{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
		Spec: llamav1alpha1.LlamaStackDistributionSpec{
			Server: llamav1alpha1.ServerSpec{
				Distribution: llamav1alpha1.DistributionType{Name: "starter"},
				Providers: &llamav1alpha1.ProvidersSpec{
					Inference: &llamav1alpha1.ProviderConfigOrList{
						Single: &llamav1alpha1.ProviderEntry{
							Provider: "openai",
							APIKey: &llamav1alpha1.SecretKeyRefSource{
								SecretKeyRef: corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "openai-creds",
									},
									Key: "api-key",
								},
							},
						},
					},
				},
			},
		},
	}

	result, err := gen.Generate(instance)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify env vars are generated
	if len(result.EnvVars) == 0 {
		t.Error("expected env vars to be generated for secret ref")
	}

	// Find the API key env var
	var apiKeyEnv *corev1.EnvVar
	for i := range result.EnvVars {
		if containsString(result.EnvVars[i].Name, "API_KEY") {
			apiKeyEnv = &result.EnvVars[i]
			break
		}
	}

	if apiKeyEnv == nil {
		t.Fatal("expected API_KEY env var")
	}

	// Verify it references the secret
	if apiKeyEnv.ValueFrom == nil || apiKeyEnv.ValueFrom.SecretKeyRef == nil {
		t.Fatal("expected env var to reference secret")
	}

	if apiKeyEnv.ValueFrom.SecretKeyRef.Name != "openai-creds" {
		t.Errorf("expected secret name 'openai-creds', got %q", apiKeyEnv.ValueFrom.SecretKeyRef.Name)
	}

	// Verify config.yaml references the env var
	var config map[string]interface{}
	if err := yaml.Unmarshal(result.ConfigYAML, &config); err != nil {
		t.Fatalf("failed to parse config: %v", err)
	}

	providers := config["providers"].(map[string]interface{})
	inference := providers["inference"].([]interface{})
	firstProvider := inference[0].(map[string]interface{})
	providerConfig := firstProvider["config"].(map[string]interface{})

	apiKeyRef, ok := providerConfig["api_key"].(string)
	if !ok {
		t.Fatal("expected api_key in config")
	}

	if !containsString(apiKeyRef, "${env.") {
		t.Errorf("expected api_key to be env var reference, got %q", apiKeyRef)
	}
}

func TestGenerator_ErrorMessagesIncludeFieldPath(t *testing.T) {
	gen := NewGenerator()

	tests := []struct {
		name            string
		instance        *llamav1alpha1.LlamaStackDistribution
		wantErrContains string
	}{
		{
			name: "invalid distribution name includes field path",
			instance: &llamav1alpha1.LlamaStackDistribution{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec: llamav1alpha1.LlamaStackDistributionSpec{
					Server: llamav1alpha1.ServerSpec{
						Distribution: llamav1alpha1.DistributionType{Name: "nonexistent"},
					},
				},
			},
			wantErrContains: "spec.server.distribution.name",
		},
		{
			name: "invalid provider type includes field path",
			instance: &llamav1alpha1.LlamaStackDistribution{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec: llamav1alpha1.LlamaStackDistributionSpec{
					Server: llamav1alpha1.ServerSpec{
						Distribution: llamav1alpha1.DistributionType{Name: "starter"},
						Providers: &llamav1alpha1.ProvidersSpec{
							Inference: &llamav1alpha1.ProviderConfigOrList{
								Single: &llamav1alpha1.ProviderEntry{
									Provider: "invalid-provider",
								},
							},
						},
					},
				},
			},
			wantErrContains: "spec.server.providers.inference",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := gen.Generate(tt.instance)
			if err == nil {
				t.Error("expected error but got none")
				return
			}

			if !containsString(err.Error(), tt.wantErrContains) {
				t.Errorf("error %q should contain field path %q", err.Error(), tt.wantErrContains)
			}
		})
	}
}

func TestGenerator_UniqueProviderIDs(t *testing.T) {
	gen := NewGenerator()

	tests := []struct {
		name        string
		instance    *llamav1alpha1.LlamaStackDistribution
		wantErr     bool
		errContains string
	}{
		{
			name: "unique IDs across provider types succeeds",
			instance: &llamav1alpha1.LlamaStackDistribution{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec: llamav1alpha1.LlamaStackDistributionSpec{
					Server: llamav1alpha1.ServerSpec{
						Distribution: llamav1alpha1.DistributionType{Name: "starter"},
						Providers: &llamav1alpha1.ProvidersSpec{
							Inference: &llamav1alpha1.ProviderConfigOrList{
								Single: &llamav1alpha1.ProviderEntry{
									Provider: "vllm",
								},
							},
							Safety: &llamav1alpha1.ProviderConfigOrList{
								Single: &llamav1alpha1.ProviderEntry{
									Provider: "llama-guard",
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "duplicate auto-generated IDs fails",
			instance: &llamav1alpha1.LlamaStackDistribution{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec: llamav1alpha1.LlamaStackDistributionSpec{
					Server: llamav1alpha1.ServerSpec{
						Distribution: llamav1alpha1.DistributionType{Name: "starter"},
						Providers: &llamav1alpha1.ProvidersSpec{
							Inference: &llamav1alpha1.ProviderConfigOrList{
								Single: &llamav1alpha1.ProviderEntry{
									Provider: "vllm",
								},
							},
							Safety: &llamav1alpha1.ProviderConfigOrList{
								Single: &llamav1alpha1.ProviderEntry{
									ID:       "vllm", // Duplicate of auto-generated inference ID
									Provider: "llama-guard",
								},
							},
						},
					},
				},
			},
			wantErr:     true,
			errContains: "duplicate provider ID 'vllm'",
		},
		{
			name: "duplicate explicit IDs in list fails",
			instance: &llamav1alpha1.LlamaStackDistribution{
				ObjectMeta: metav1.ObjectMeta{Name: "test"},
				Spec: llamav1alpha1.LlamaStackDistributionSpec{
					Server: llamav1alpha1.ServerSpec{
						Distribution: llamav1alpha1.DistributionType{Name: "starter"},
						Providers: &llamav1alpha1.ProvidersSpec{
							Inference: llamav1alpha1.NewProviderConfigList([]llamav1alpha1.ProviderEntry{
								{ID: "my-provider", Provider: "vllm"},
								{ID: "my-provider", Provider: "ollama"}, // Duplicate
							}),
						},
					},
				},
			},
			wantErr:     true,
			errContains: "duplicate provider ID 'my-provider'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := gen.Generate(tt.instance)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got none")
					return
				}
				if tt.errContains != "" && !containsString(err.Error(), tt.errContains) {
					t.Errorf("error %q should contain %q", err.Error(), tt.errContains)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}

func TestGenerator_MultipleProvidersExpansion(t *testing.T) {
	gen := NewGenerator()

	instance := &llamav1alpha1.LlamaStackDistribution{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
		Spec: llamav1alpha1.LlamaStackDistributionSpec{
			Server: llamav1alpha1.ServerSpec{
				Distribution: llamav1alpha1.DistributionType{Name: "starter"},
				Providers: &llamav1alpha1.ProvidersSpec{
					Inference: llamav1alpha1.NewProviderConfigList([]llamav1alpha1.ProviderEntry{
						{
							ID:       "vllm-local",
							Provider: "vllm",
							Endpoint: "http://vllm-local:8000",
						},
						{
							ID:       "vllm-gpu",
							Provider: "vllm",
							Endpoint: "http://vllm-gpu:8000",
						},
						{
							ID:       "ollama-dev",
							Provider: "ollama",
							Endpoint: "http://ollama:11434",
						},
					}),
				},
			},
		},
	}

	result, err := gen.Generate(instance)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Parse the generated config
	var config map[string]interface{}
	if err := yaml.Unmarshal(result.ConfigYAML, &config); err != nil {
		t.Fatalf("failed to parse generated config: %v", err)
	}

	// Verify providers section
	providers, ok := config["providers"].(map[string]interface{})
	if !ok {
		t.Fatal("expected providers section")
	}

	inference, ok := providers["inference"].([]interface{})
	if !ok {
		t.Fatal("expected inference providers list")
	}

	// Verify we have all 3 providers
	if len(inference) != 3 {
		t.Errorf("expected 3 inference providers, got %d", len(inference))
	}

	// Verify each provider has correct structure
	expectedProviders := []struct {
		id           string
		providerType string
		url          string
	}{
		{"vllm-local", "remote::vllm", "http://vllm-local:8000"},
		{"vllm-gpu", "remote::vllm", "http://vllm-gpu:8000"},
		{"ollama-dev", "remote::ollama", "http://ollama:11434"},
	}

	for i, expected := range expectedProviders {
		provider, ok := inference[i].(map[string]interface{})
		if !ok {
			t.Fatalf("provider[%d] is not a map", i)
		}

		if pid := provider["provider_id"].(string); pid != expected.id {
			t.Errorf("provider[%d] id = %q, want %q", i, pid, expected.id)
		}

		if pt := provider["provider_type"].(string); pt != expected.providerType {
			t.Errorf("provider[%d] type = %q, want %q", i, pt, expected.providerType)
		}

		providerConfig, ok := provider["config"].(map[string]interface{})
		if !ok {
			t.Fatalf("provider[%d] config is not a map", i)
		}

		if url := providerConfig["url"].(string); url != expected.url {
			t.Errorf("provider[%d] url = %q, want %q", i, url, expected.url)
		}
	}
}

func TestGenerator_MissingIDInListForm(t *testing.T) {
	gen := NewGenerator()

	// Create a provider list where one entry is missing the required ID
	instance := &llamav1alpha1.LlamaStackDistribution{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
		Spec: llamav1alpha1.LlamaStackDistributionSpec{
			Server: llamav1alpha1.ServerSpec{
				Distribution: llamav1alpha1.DistributionType{Name: "starter"},
				Providers: &llamav1alpha1.ProvidersSpec{
					Inference: llamav1alpha1.NewProviderConfigList([]llamav1alpha1.ProviderEntry{
						{
							ID:       "first-provider",
							Provider: "vllm",
							Endpoint: "http://vllm:8000",
						},
						{
							// Missing ID - should fail validation
							Provider: "ollama",
							Endpoint: "http://ollama:11434",
						},
					}),
				},
			},
		},
	}

	_, err := gen.Generate(instance)
	if err == nil {
		t.Error("expected error for missing ID in list form")
		return
	}

	// Verify error message includes field path and mentions ID requirement
	if !containsString(err.Error(), "spec.server.providers.inference") {
		t.Errorf("error should contain field path, got: %v", err)
	}
	if !containsString(err.Error(), "id") {
		t.Errorf("error should mention 'id' requirement, got: %v", err)
	}
}

func TestGenerator_ConfigPassthrough(t *testing.T) {
	gen := NewGenerator()

	instance := &llamav1alpha1.LlamaStackDistribution{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
		Spec: llamav1alpha1.LlamaStackDistributionSpec{
			Server: llamav1alpha1.ServerSpec{
				Distribution: llamav1alpha1.DistributionType{Name: "starter"},
				Providers: &llamav1alpha1.ProvidersSpec{
					Inference: &llamav1alpha1.ProviderConfigOrList{
						Single: &llamav1alpha1.ProviderEntry{
							Provider: "vllm",
							Endpoint: "http://vllm:8000",
							Config: &runtime.RawExtension{
								Raw: []byte(`{"model": "llama-3.2-3b", "max_tokens": 4096, "temperature": 0.7}`),
							},
						},
					},
				},
			},
		},
	}

	result, err := gen.Generate(instance)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Parse the generated config
	var config map[string]interface{}
	if err := yaml.Unmarshal(result.ConfigYAML, &config); err != nil {
		t.Fatalf("failed to parse generated config: %v", err)
	}

	// Get the provider config section
	providers := config["providers"].(map[string]interface{})
	inference := providers["inference"].([]interface{})
	provider := inference[0].(map[string]interface{})
	providerConfig := provider["config"].(map[string]interface{})

	// Verify pass-through fields are present
	if model, ok := providerConfig["model"].(string); !ok || model != "llama-3.2-3b" {
		t.Errorf("expected model 'llama-3.2-3b', got %v", providerConfig["model"])
	}

	// max_tokens should be parsed as float64 by yaml unmarshaling
	if maxTokens, ok := providerConfig["max_tokens"].(float64); !ok || maxTokens != 4096 {
		t.Errorf("expected max_tokens 4096, got %v", providerConfig["max_tokens"])
	}

	if temp, ok := providerConfig["temperature"].(float64); !ok || temp != 0.7 {
		t.Errorf("expected temperature 0.7, got %v", providerConfig["temperature"])
	}

	// Verify simplified field (url from endpoint) is also present
	if url, ok := providerConfig["url"].(string); !ok || url != "http://vllm:8000" {
		t.Errorf("expected url 'http://vllm:8000', got %v", providerConfig["url"])
	}
}

func TestGenerator_SimplifiedFieldsOverrideConfig(t *testing.T) {
	gen := NewGenerator()

	// Test that simplified fields (endpoint) override config.url
	instance := &llamav1alpha1.LlamaStackDistribution{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
		Spec: llamav1alpha1.LlamaStackDistributionSpec{
			Server: llamav1alpha1.ServerSpec{
				Distribution: llamav1alpha1.DistributionType{Name: "starter"},
				Providers: &llamav1alpha1.ProvidersSpec{
					Inference: &llamav1alpha1.ProviderConfigOrList{
						Single: &llamav1alpha1.ProviderEntry{
							Provider: "vllm",
							Endpoint: "http://correct-url:8000",
							Config: &runtime.RawExtension{
								// This url should be ignored because endpoint is set
								Raw: []byte(`{"url": "http://wrong-url:9999", "custom_field": "value"}`),
							},
						},
					},
				},
			},
		},
	}

	result, err := gen.Generate(instance)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Parse the generated config
	var config map[string]interface{}
	if err := yaml.Unmarshal(result.ConfigYAML, &config); err != nil {
		t.Fatalf("failed to parse generated config: %v", err)
	}

	// Get the provider config section
	providers := config["providers"].(map[string]interface{})
	inference := providers["inference"].([]interface{})
	provider := inference[0].(map[string]interface{})
	providerConfig := provider["config"].(map[string]interface{})

	// Verify endpoint takes precedence over config.url
	if url, ok := providerConfig["url"].(string); !ok || url != "http://correct-url:8000" {
		t.Errorf("expected url 'http://correct-url:8000' (from endpoint), got %v", providerConfig["url"])
	}

	// Verify other config fields are still present
	if customField, ok := providerConfig["custom_field"].(string); !ok || customField != "value" {
		t.Errorf("expected custom_field 'value', got %v", providerConfig["custom_field"])
	}
}

func TestGenerator_SimpleModelRegistration(t *testing.T) {
	gen := NewGenerator()

	instance := &llamav1alpha1.LlamaStackDistribution{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
		Spec: llamav1alpha1.LlamaStackDistributionSpec{
			Server: llamav1alpha1.ServerSpec{
				Distribution: llamav1alpha1.DistributionType{Name: "starter"},
				Providers: &llamav1alpha1.ProvidersSpec{
					Inference: &llamav1alpha1.ProviderConfigOrList{
						Single: &llamav1alpha1.ProviderEntry{
							Provider: "vllm",
							Endpoint: "http://vllm:8000",
						},
					},
				},
				Resources: &llamav1alpha1.ResourcesSpec{
					Models: []llamav1alpha1.ModelEntry{
						{Name: "meta-llama/Llama-3.2-3B-Instruct"},
						{Name: "meta-llama/Llama-3.2-7B-Instruct"},
					},
				},
			},
		},
	}

	result, err := gen.Generate(instance)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var config map[string]interface{}
	if err := yaml.Unmarshal(result.ConfigYAML, &config); err != nil {
		t.Fatalf("failed to parse config: %v", err)
	}

	// Verify registered_resources section
	registeredResources, ok := config["registered_resources"].(map[string]interface{})
	if !ok {
		t.Fatal("expected registered_resources section")
	}

	models, ok := registeredResources["models"].([]interface{})
	if !ok {
		t.Fatal("expected models list")
	}

	if len(models) != 2 {
		t.Errorf("expected 2 models, got %d", len(models))
	}

	// Verify first model uses default provider (vllm)
	firstModel := models[0].(map[string]interface{})
	if id := firstModel["identifier"].(string); id != "meta-llama/Llama-3.2-3B-Instruct" {
		t.Errorf("expected model identifier 'meta-llama/Llama-3.2-3B-Instruct', got %v", id)
	}
	if pid := firstModel["provider_id"].(string); pid != "vllm" {
		t.Errorf("expected provider_id 'vllm' (auto-assigned), got %v", pid)
	}
}

func TestGenerator_DetailedModelWithProvider(t *testing.T) {
	gen := NewGenerator()

	instance := &llamav1alpha1.LlamaStackDistribution{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
		Spec: llamav1alpha1.LlamaStackDistributionSpec{
			Server: llamav1alpha1.ServerSpec{
				Distribution: llamav1alpha1.DistributionType{Name: "starter"},
				Providers: &llamav1alpha1.ProvidersSpec{
					Inference: llamav1alpha1.NewProviderConfigList([]llamav1alpha1.ProviderEntry{
						{ID: "vllm-gpu", Provider: "vllm", Endpoint: "http://vllm-gpu:8000"},
						{ID: "vllm-cpu", Provider: "vllm", Endpoint: "http://vllm-cpu:8000"},
					}),
				},
				Resources: &llamav1alpha1.ResourcesSpec{
					Models: []llamav1alpha1.ModelEntry{
						{
							Name:     "meta-llama/Llama-3.2-70B-Instruct",
							Provider: "vllm-gpu",
							Metadata: &llamav1alpha1.ModelMetadata{
								ContextLength: 8192,
							},
						},
						{
							Name:     "meta-llama/Llama-3.2-3B-Instruct",
							Provider: "vllm-cpu",
							Metadata: &llamav1alpha1.ModelMetadata{
								ContextLength:      4096,
								EmbeddingDimension: 1024,
							},
						},
					},
				},
			},
		},
	}

	result, err := gen.Generate(instance)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var config map[string]interface{}
	if err := yaml.Unmarshal(result.ConfigYAML, &config); err != nil {
		t.Fatalf("failed to parse config: %v", err)
	}

	registeredResources := config["registered_resources"].(map[string]interface{})
	models := registeredResources["models"].([]interface{})

	// Verify first model with explicit provider
	firstModel := models[0].(map[string]interface{})
	if pid := firstModel["provider_id"].(string); pid != "vllm-gpu" {
		t.Errorf("expected provider_id 'vllm-gpu', got %v", pid)
	}

	// Verify metadata (YAML unmarshals numbers as float64)
	metadata := firstModel["metadata"].(map[string]interface{})
	if cl, ok := metadata["context_length"].(float64); !ok || cl != 8192 {
		t.Errorf("expected context_length 8192, got %v", metadata["context_length"])
	}

	// Verify second model
	secondModel := models[1].(map[string]interface{})
	if pid := secondModel["provider_id"].(string); pid != "vllm-cpu" {
		t.Errorf("expected provider_id 'vllm-cpu', got %v", pid)
	}

	secondMetadata := secondModel["metadata"].(map[string]interface{})
	if ed, ok := secondMetadata["embedding_dimension"].(float64); !ok || ed != 1024 {
		t.Errorf("expected embedding_dimension 1024, got %v", secondMetadata["embedding_dimension"])
	}
}

func TestGenerator_ToolsAndShieldsMapping(t *testing.T) {
	gen := NewGenerator()

	instance := &llamav1alpha1.LlamaStackDistribution{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
		Spec: llamav1alpha1.LlamaStackDistributionSpec{
			Server: llamav1alpha1.ServerSpec{
				Distribution: llamav1alpha1.DistributionType{Name: "starter"},
				Resources: &llamav1alpha1.ResourcesSpec{
					Tools: []string{
						"builtin::code_interpreter",
						"builtin::websearch",
						"mcp::my-custom-tool",
					},
					Shields: []string{
						"meta-llama/Llama-Guard-3-8B",
						"custom-content-filter",
					},
				},
			},
		},
	}

	result, err := gen.Generate(instance)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var config map[string]interface{}
	if err := yaml.Unmarshal(result.ConfigYAML, &config); err != nil {
		t.Fatalf("failed to parse config: %v", err)
	}

	registeredResources := config["registered_resources"].(map[string]interface{})

	// Verify tool_groups
	toolGroups, ok := registeredResources["tool_groups"].([]interface{})
	if !ok {
		t.Fatal("expected tool_groups list")
	}
	if len(toolGroups) != 3 {
		t.Errorf("expected 3 tool_groups, got %d", len(toolGroups))
	}

	// Verify first tool
	firstTool := toolGroups[0].(map[string]interface{})
	if id := firstTool["identifier"].(string); id != "builtin::code_interpreter" {
		t.Errorf("expected identifier 'builtin::code_interpreter', got %v", id)
	}

	// Verify shields
	shields, ok := registeredResources["shields"].([]interface{})
	if !ok {
		t.Fatal("expected shields list")
	}
	if len(shields) != 2 {
		t.Errorf("expected 2 shields, got %d", len(shields))
	}

	// Verify first shield
	firstShield := shields[0].(map[string]interface{})
	if id := firstShield["identifier"].(string); id != "meta-llama/Llama-Guard-3-8B" {
		t.Errorf("expected identifier 'meta-llama/Llama-Guard-3-8B', got %v", id)
	}
}

func TestGenerator_DisabledProviderRemoval(t *testing.T) {
	gen := NewGenerator()

	instance := &llamav1alpha1.LlamaStackDistribution{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
		Spec: llamav1alpha1.LlamaStackDistributionSpec{
			Server: llamav1alpha1.ServerSpec{
				Distribution: llamav1alpha1.DistributionType{Name: "starter"},
				// Disable inference and safety providers
				Disabled: []string{"inference", "safety"},
			},
		},
	}

	result, err := gen.Generate(instance)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var config map[string]interface{}
	if err := yaml.Unmarshal(result.ConfigYAML, &config); err != nil {
		t.Fatalf("failed to parse config: %v", err)
	}

	providers, ok := config["providers"].(map[string]interface{})
	if !ok {
		t.Fatal("expected providers section")
	}

	// Verify inference is removed
	if _, exists := providers["inference"]; exists {
		t.Error("expected 'inference' to be removed from providers")
	}

	// Verify safety is removed
	if _, exists := providers["safety"]; exists {
		t.Error("expected 'safety' to be removed from providers")
	}

	// Note: Whether other providers remain depends on base distribution config
}

func TestGenerator_DisabledCamelToSnakeConversion(t *testing.T) {
	gen := NewGenerator()

	// Test that camelCase disabled names are converted to snake_case
	instance := &llamav1alpha1.LlamaStackDistribution{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
		Spec: llamav1alpha1.LlamaStackDistributionSpec{
			Server: llamav1alpha1.ServerSpec{
				Distribution: llamav1alpha1.DistributionType{Name: "starter"},
				// CamelCase names should be converted to snake_case
				Disabled: []string{"vectorIo", "toolRuntime"},
			},
		},
	}

	result, err := gen.Generate(instance)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var config map[string]interface{}
	if err := yaml.Unmarshal(result.ConfigYAML, &config); err != nil {
		t.Fatalf("failed to parse config: %v", err)
	}

	providers, ok := config["providers"].(map[string]interface{})
	if !ok {
		t.Fatal("expected providers section")
	}

	// Verify vector_io is removed (converted from vectorIo)
	if _, exists := providers["vector_io"]; exists {
		t.Error("expected 'vector_io' to be removed from providers")
	}

	// Verify tool_runtime is removed (converted from toolRuntime)
	if _, exists := providers["tool_runtime"]; exists {
		t.Error("expected 'tool_runtime' to be removed from providers")
	}
}

func TestGenerator_DisabledWarnings(t *testing.T) {
	gen := NewGenerator()

	instance := &llamav1alpha1.LlamaStackDistribution{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
		Spec: llamav1alpha1.LlamaStackDistributionSpec{
			Server: llamav1alpha1.ServerSpec{
				Distribution: llamav1alpha1.DistributionType{Name: "starter"},
				Disabled:     []string{"nonExistentProvider", "anotherMissing"},
			},
		},
	}

	result, err := gen.Generate(instance)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have warnings for non-existent disabled providers
	if len(result.Warnings) != 2 {
		t.Errorf("expected 2 warnings, got %d", len(result.Warnings))
	}

	// Verify warnings mention the provider types
	foundNonExistent := false
	foundAnother := false
	for _, warning := range result.Warnings {
		if containsString(warning, "nonExistentProvider") {
			foundNonExistent = true
		}
		if containsString(warning, "anotherMissing") {
			foundAnother = true
		}
	}

	if !foundNonExistent {
		t.Error("expected warning for 'nonExistentProvider'")
	}
	if !foundAnother {
		t.Error("expected warning for 'anotherMissing'")
	}
}

func TestGenerator_StorageConfigurationSqlite(t *testing.T) {
	gen := NewGenerator()

	instance := &llamav1alpha1.LlamaStackDistribution{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
		Spec: llamav1alpha1.LlamaStackDistributionSpec{
			Server: llamav1alpha1.ServerSpec{
				Distribution: llamav1alpha1.DistributionType{Name: "starter"},
				ConfigStorage: &llamav1alpha1.StorageConfigSpec{
					Type: "sqlite",
				},
			},
		},
	}

	result, err := gen.Generate(instance)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var config map[string]interface{}
	if err := yaml.Unmarshal(result.ConfigYAML, &config); err != nil {
		t.Fatalf("failed to parse config: %v", err)
	}

	// Verify metadata_store section
	metadataStore, ok := config["metadata_store"].(map[string]interface{})
	if !ok {
		t.Fatal("expected metadata_store section")
	}

	if storeType := metadataStore["type"].(string); storeType != "sqlite" {
		t.Errorf("expected type 'sqlite', got %v", storeType)
	}

	if dbPath := metadataStore["db_path"].(string); dbPath != "/.llama/runtime/metadata.db" {
		t.Errorf("expected db_path '/.llama/runtime/metadata.db', got %v", dbPath)
	}

	// SQLite shouldn't generate env vars for connection
	if len(result.EnvVars) != 0 {
		t.Errorf("expected no env vars for sqlite, got %d", len(result.EnvVars))
	}
}

func TestGenerator_StorageConfigurationPostgres(t *testing.T) {
	gen := NewGenerator()

	instance := &llamav1alpha1.LlamaStackDistribution{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
		Spec: llamav1alpha1.LlamaStackDistributionSpec{
			Server: llamav1alpha1.ServerSpec{
				Distribution: llamav1alpha1.DistributionType{Name: "starter"},
				ConfigStorage: &llamav1alpha1.StorageConfigSpec{
					Type: "postgres",
					ConnectionString: &llamav1alpha1.SecretKeyRefSource{
						SecretKeyRef: corev1.SecretKeySelector{
							LocalObjectReference: corev1.LocalObjectReference{
								Name: "postgres-credentials",
							},
							Key: "connection-string",
						},
					},
				},
			},
		},
	}

	result, err := gen.Generate(instance)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var config map[string]interface{}
	if err := yaml.Unmarshal(result.ConfigYAML, &config); err != nil {
		t.Fatalf("failed to parse config: %v", err)
	}

	// Verify metadata_store section
	metadataStore, ok := config["metadata_store"].(map[string]interface{})
	if !ok {
		t.Fatal("expected metadata_store section")
	}

	if storeType := metadataStore["type"].(string); storeType != "postgres" {
		t.Errorf("expected type 'postgres', got %v", storeType)
	}

	// Verify connection_string references env var
	connStr, ok := metadataStore["connection_string"].(string)
	if !ok {
		t.Fatal("expected connection_string")
	}
	if !containsString(connStr, "${env.") {
		t.Errorf("expected connection_string to be env var reference, got %v", connStr)
	}

	// Verify env var is generated
	var connEnv *corev1.EnvVar
	for i := range result.EnvVars {
		if containsString(result.EnvVars[i].Name, "CONNECTION_STRING") {
			connEnv = &result.EnvVars[i]
			break
		}
	}

	if connEnv == nil {
		t.Fatal("expected CONNECTION_STRING env var")
	}

	if connEnv.ValueFrom.SecretKeyRef.Name != "postgres-credentials" {
		t.Errorf("expected secret name 'postgres-credentials', got %v", connEnv.ValueFrom.SecretKeyRef.Name)
	}
}

func TestGenerator_ServerSettingsPort(t *testing.T) {
	gen := NewGenerator()

	instance := &llamav1alpha1.LlamaStackDistribution{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
		Spec: llamav1alpha1.LlamaStackDistributionSpec{
			Server: llamav1alpha1.ServerSpec{
				Distribution: llamav1alpha1.DistributionType{Name: "starter"},
				Port:         8443,
			},
		},
	}

	result, err := gen.Generate(instance)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var config map[string]interface{}
	if err := yaml.Unmarshal(result.ConfigYAML, &config); err != nil {
		t.Fatalf("failed to parse config: %v", err)
	}

	// Verify port is set (YAML unmarshals as float64)
	if port, ok := config["port"].(float64); !ok || port != 8443 {
		t.Errorf("expected port 8443, got %v", config["port"])
	}
}

func TestGenerator_ServerSettingsTLS(t *testing.T) {
	gen := NewGenerator()

	instance := &llamav1alpha1.LlamaStackDistribution{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
		Spec: llamav1alpha1.LlamaStackDistributionSpec{
			Server: llamav1alpha1.ServerSpec{
				Distribution: llamav1alpha1.DistributionType{Name: "starter"},
				Port:         8443,
				ServerTLS: &llamav1alpha1.ServerTLSConfig{
					Enabled:    true,
					SecretName: "llama-tls-cert",
				},
			},
		},
	}

	result, err := gen.Generate(instance)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var config map[string]interface{}
	if err := yaml.Unmarshal(result.ConfigYAML, &config); err != nil {
		t.Fatalf("failed to parse config: %v", err)
	}

	// Verify TLS section
	tlsConfig, ok := config["tls"].(map[string]interface{})
	if !ok {
		t.Fatal("expected tls section")
	}

	if enabled, ok := tlsConfig["enabled"].(bool); !ok || !enabled {
		t.Errorf("expected tls.enabled true, got %v", tlsConfig["enabled"])
	}

	if secretName, ok := tlsConfig["secret_name"].(string); !ok || secretName != "llama-tls-cert" {
		t.Errorf("expected tls.secret_name 'llama-tls-cert', got %v", tlsConfig["secret_name"])
	}
}

// Helper functions
func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstring(s, substr))
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
