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

package deploy

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseRunYamlConfig_Basic(t *testing.T) {
	yamlContent := `
version: 2
distro_name: llamastack/distribution-ollama
apis:
  - inference
  - safety
providers:
  inference:
    - provider_id: ollama
      provider_type: remote::ollama
      config:
        url: http://localhost:11434
  safety:
    - provider_id: llama-guard
      provider_type: inline::llama-guard
`

	config, err := ParseRunYamlConfig([]byte(yamlContent))
	require.NoError(t, err)

	assert.Equal(t, 2, config.Version)
	assert.Equal(t, "llamastack/distribution-ollama", config.ImageName)
	assert.Len(t, config.APIs, 2)
	assert.Contains(t, config.APIs, "inference")
	assert.Contains(t, config.APIs, "safety")
	assert.Len(t, config.Providers["inference"], 1)
	assert.Equal(t, "ollama", config.Providers["inference"][0].ProviderID)
	assert.Equal(t, "remote::ollama", config.Providers["inference"][0].ProviderType)
}

func TestParseRunYamlConfig_EmptyProviders(t *testing.T) {
	yamlContent := `
version: 2
apis:
  - inference
`

	config, err := ParseRunYamlConfig([]byte(yamlContent))
	require.NoError(t, err)

	assert.NotNil(t, config.Providers)
	assert.Empty(t, config.Providers)
}

func TestParseRunYamlConfig_InvalidYAML(t *testing.T) {
	invalidYAML := `
version: [invalid
`

	_, err := ParseRunYamlConfig([]byte(invalidYAML))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse config YAML")
}

func TestLoadRunYamlConfig_FileNotFound(t *testing.T) {
	_, err := LoadRunYamlConfig("/nonexistent/path/config.yaml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read config file")
}

func TestLoadRunYamlConfig_ValidFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	content := `
version: 2
providers:
  inference:
    - provider_id: test
      provider_type: remote::test
`

	err := os.WriteFile(configPath, []byte(content), 0o644)
	require.NoError(t, err)

	config, err := LoadRunYamlConfig(configPath)
	require.NoError(t, err)
	assert.Equal(t, 2, config.Version)
	assert.Equal(t, "test", config.Providers["inference"][0].ProviderID)
}

func TestSerializeRunYamlConfig(t *testing.T) {
	config := &RunYamlConfig{
		Version:   2,
		ImageName: "test-image",
		Providers: map[string][]ProviderConfigEntry{
			"inference": {
				{
					ProviderID:   "test",
					ProviderType: "remote::test",
					Config:       map[string]interface{}{"url": "http://example.com"},
				},
			},
		},
	}

	data, err := SerializeRunYamlConfig(config)
	require.NoError(t, err)

	// Parse back to verify roundtrip
	parsed, err := ParseRunYamlConfig(data)
	require.NoError(t, err)

	assert.Equal(t, config.Version, parsed.Version)
	assert.Equal(t, config.ImageName, parsed.ImageName)
	assert.Equal(t, config.Providers["inference"][0].ProviderID, parsed.Providers["inference"][0].ProviderID)
}

func TestWriteRunYamlConfig(t *testing.T) {
	tmpDir := t.TempDir()
	outputPath := filepath.Join(tmpDir, "output.yaml")

	config := &RunYamlConfig{
		Version: 2,
		Providers: map[string][]ProviderConfigEntry{
			"inference": {
				{ProviderID: "test", ProviderType: "remote::test"},
			},
		},
	}

	err := WriteRunYamlConfig(config, outputPath)
	require.NoError(t, err)

	// Verify file exists and can be loaded
	loaded, err := LoadRunYamlConfig(outputPath)
	require.NoError(t, err)
	assert.Equal(t, "test", loaded.Providers["inference"][0].ProviderID)
}

func TestMergeExternalProviders_NewProvider(t *testing.T) {
	config := &RunYamlConfig{
		Version: 2,
		Providers: map[string][]ProviderConfigEntry{
			"inference": {
				{ProviderID: "existing", ProviderType: "remote::existing"},
			},
		},
	}

	providers := []ProviderEntry{
		{
			ProviderID:   "new-provider",
			ProviderType: "remote::new",
			Module:       "my_org.new_provider",
			API:          "inference",
			Config:       map[string]interface{}{"url": "http://new.example.com"},
		},
	}

	warnings, err := MergeExternalProviders(config, providers)
	require.NoError(t, err)
	assert.Empty(t, warnings)

	// Should have both providers
	assert.Len(t, config.Providers["inference"], 2)
	assert.Equal(t, "existing", config.Providers["inference"][0].ProviderID)
	assert.Equal(t, "new-provider", config.Providers["inference"][1].ProviderID)
}

func TestMergeExternalProviders_OverrideExisting(t *testing.T) {
	config := &RunYamlConfig{
		Version: 2,
		Providers: map[string][]ProviderConfigEntry{
			"inference": {
				{
					ProviderID:   "override-me",
					ProviderType: "remote::original",
					Config:       map[string]interface{}{"key": "original-value"},
				},
			},
		},
	}

	providers := []ProviderEntry{
		{
			ProviderID:   "override-me",
			ProviderType: "remote::external",
			Module:       "my_org.external",
			API:          "inference",
			Config:       map[string]interface{}{"key": "new-value"},
		},
	}

	warnings, err := MergeExternalProviders(config, providers)
	require.NoError(t, err)

	// Should have warning about override
	require.Len(t, warnings, 1)
	assert.Contains(t, warnings[0].Message, "override-me")
	assert.Contains(t, warnings[0].Message, "overrides base provider")

	// Should be overridden
	assert.Len(t, config.Providers["inference"], 1)
	assert.Equal(t, "remote::external", config.Providers["inference"][0].ProviderType)
	assert.Equal(t, "new-value", config.Providers["inference"][0].Config["key"])
}

func TestMergeExternalProviders_NewAPISection(t *testing.T) {
	config := &RunYamlConfig{
		Version: 2,
		Providers: map[string][]ProviderConfigEntry{
			"inference": {
				{ProviderID: "existing", ProviderType: "remote::existing"},
			},
		},
	}

	providers := []ProviderEntry{
		{
			ProviderID:   "safety-provider",
			ProviderType: "remote::safety",
			Module:       "my_org.safety",
			API:          "safety",
		},
	}

	warnings, err := MergeExternalProviders(config, providers)
	require.NoError(t, err)
	assert.Empty(t, warnings)

	// Should have new API section
	assert.Len(t, config.Providers["safety"], 1)
	assert.Equal(t, "safety-provider", config.Providers["safety"][0].ProviderID)
}

func TestMergeExternalProviders_DuplicateProviderIDs(t *testing.T) {
	config := &RunYamlConfig{
		Version:   2,
		Providers: make(map[string][]ProviderConfigEntry),
	}

	providers := []ProviderEntry{
		{
			ProviderID:   "duplicate-id",
			ProviderType: "remote::first",
			Module:       "first_module",
			API:          "inference",
		},
		{
			ProviderID:   "duplicate-id",
			ProviderType: "remote::second",
			Module:       "second_module",
			API:          "safety",
		},
	}

	_, err := MergeExternalProviders(config, providers)
	require.Error(t, err)

	var dupErr *DuplicateProviderError
	require.ErrorAs(t, err, &dupErr)
	assert.Equal(t, "duplicate-id", dupErr.ProviderID)
}

func TestMergeExternalProviders_NilProvidersMap(t *testing.T) {
	config := &RunYamlConfig{
		Version:   2,
		Providers: nil,
	}

	providers := []ProviderEntry{
		{
			ProviderID:   "test",
			ProviderType: "remote::test",
			API:          "inference",
		},
	}

	warnings, err := MergeExternalProviders(config, providers)
	require.NoError(t, err)
	assert.Empty(t, warnings)

	assert.NotNil(t, config.Providers)
	assert.Len(t, config.Providers["inference"], 1)
}

func TestMergeExternalProviders_PreservesModule(t *testing.T) {
	config := &RunYamlConfig{
		Version:   2,
		Providers: make(map[string][]ProviderConfigEntry),
	}

	providers := []ProviderEntry{
		{
			ProviderID:   "test",
			ProviderType: "remote::test",
			Module:       "my_org.custom_module",
			API:          "inference",
		},
	}

	_, err := MergeExternalProviders(config, providers)
	require.NoError(t, err)

	assert.Equal(t, "my_org.custom_module", config.Providers["inference"][0].Module)
}

func TestAPIPlacementError(t *testing.T) {
	err := &APIPlacementError{
		ProviderID:  "my-provider",
		Image:       "my-image:v1",
		DeclaredAPI: "inference",
		PlacedInAPI: "safety",
	}

	errMsg := err.Error()
	assert.Contains(t, errMsg, "my-provider")
	assert.Contains(t, errMsg, "my-image:v1")
	assert.Contains(t, errMsg, "inference")
	assert.Contains(t, errMsg, "safety")
	assert.Contains(t, errMsg, "Resolution")
}

func TestDuplicateProviderError(t *testing.T) {
	err := &DuplicateProviderError{
		ProviderID: "dup-id",
		Image1:     "image1:v1",
		Image2:     "image2:v1",
	}

	errMsg := err.Error()
	assert.Contains(t, errMsg, "dup-id")
	assert.Contains(t, errMsg, "image1:v1")
	assert.Contains(t, errMsg, "image2:v1")
	assert.Contains(t, errMsg, "Duplicate provider ID")
}

func TestGenerateProviderEntriesFromMetadata(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a provider directory with metadata
	providerDir := filepath.Join(tmpDir, "test-provider")
	err := os.MkdirAll(providerDir, 0o755)
	require.NoError(t, err)

	// Write provider metadata
	metadata := `
apiVersion: llamastack.io/v1alpha1
kind: ProviderPackage
metadata:
  name: test-provider
  version: 1.0.0
  vendor: test-org
spec:
  packageName: test_org.test_provider
  providerType: remote::test-provider
  api: inference
  wheelPath: /lls-provider/packages/test-1.0.0.whl
`
	err = os.WriteFile(filepath.Join(providerDir, "lls-provider-spec.yaml"), []byte(metadata), 0o644)
	require.NoError(t, err)

	// Write CRD config
	crdConfig := `
providerId: test-provider
api: inference
config:
  url: http://test.example.com
`
	err = os.WriteFile(filepath.Join(providerDir, "crd-config.yaml"), []byte(crdConfig), 0o644)
	require.NoError(t, err)

	// Generate entries
	providers, err := GenerateProviderEntriesFromMetadata(tmpDir)
	require.NoError(t, err)

	require.Len(t, providers, 1)
	assert.Equal(t, "test-provider", providers[0].ProviderID)
	assert.Equal(t, "remote::test-provider", providers[0].ProviderType)
	assert.Equal(t, "test_org.test_provider", providers[0].Module)
	assert.Equal(t, "inference", providers[0].API)
	assert.Equal(t, "http://test.example.com", providers[0].Config["url"])
}

func TestGenerateProviderEntriesFromMetadata_APIMismatch(t *testing.T) {
	tmpDir := t.TempDir()

	providerDir := filepath.Join(tmpDir, "mismatched-provider")
	err := os.MkdirAll(providerDir, 0o755)
	require.NoError(t, err)

	// Metadata declares inference
	metadata := `
apiVersion: llamastack.io/v1alpha1
kind: ProviderPackage
metadata:
  name: mismatched
  version: 1.0.0
  vendor: test
spec:
  packageName: test_pkg
  providerType: remote::test
  api: inference
  wheelPath: /test.whl
`
	err = os.WriteFile(filepath.Join(providerDir, "lls-provider-spec.yaml"), []byte(metadata), 0o644)
	require.NoError(t, err)

	// CRD config says safety (mismatch!)
	crdConfig := `
providerId: mismatched
api: safety
`
	err = os.WriteFile(filepath.Join(providerDir, "crd-config.yaml"), []byte(crdConfig), 0o644)
	require.NoError(t, err)

	_, err = GenerateProviderEntriesFromMetadata(tmpDir)
	require.Error(t, err)

	var apiErr *APIPlacementError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, "inference", apiErr.DeclaredAPI)
	assert.Equal(t, "safety", apiErr.PlacedInAPI)
}

func TestGenerateConfig_Integration(t *testing.T) {
	tmpDir := t.TempDir()

	// Create base config
	baseConfigPath := filepath.Join(tmpDir, "base-config.yaml")
	baseConfig := `
version: 2
distro_name: test-distro
providers:
  inference:
    - provider_id: builtin
      provider_type: remote::builtin
`
	err := os.WriteFile(baseConfigPath, []byte(baseConfig), 0o644)
	require.NoError(t, err)

	// Create metadata directory with a provider
	metadataDir := filepath.Join(tmpDir, "metadata")
	providerDir := filepath.Join(metadataDir, "external-provider")
	err = os.MkdirAll(providerDir, 0o755)
	require.NoError(t, err)

	metadata := `
apiVersion: llamastack.io/v1alpha1
kind: ProviderPackage
metadata:
  name: external
  version: 1.0.0
  vendor: test
spec:
  packageName: test_org.external
  providerType: remote::external
  api: inference
  wheelPath: /test.whl
`
	err = os.WriteFile(filepath.Join(providerDir, "lls-provider-spec.yaml"), []byte(metadata), 0o644)
	require.NoError(t, err)

	crdConfig := `
providerId: external-provider
api: inference
config:
  custom_key: custom_value
`
	err = os.WriteFile(filepath.Join(providerDir, "crd-config.yaml"), []byte(crdConfig), 0o644)
	require.NoError(t, err)

	// Run generation
	outputPath := filepath.Join(tmpDir, "output.yaml")
	warnings, err := GenerateConfig(baseConfigPath, metadataDir, outputPath)
	require.NoError(t, err)
	assert.Empty(t, warnings)

	// Verify output
	result, err := LoadRunYamlConfig(outputPath)
	require.NoError(t, err)

	assert.Equal(t, 2, result.Version)
	assert.Equal(t, "test-distro", result.ImageName)
	require.Len(t, result.Providers["inference"], 2)
	assert.Equal(t, "builtin", result.Providers["inference"][0].ProviderID)
	assert.Equal(t, "external-provider", result.Providers["inference"][1].ProviderID)
	assert.Equal(t, "test_org.external", result.Providers["inference"][1].Module)
	assert.Equal(t, "custom_value", result.Providers["inference"][1].Config["custom_key"])
}
