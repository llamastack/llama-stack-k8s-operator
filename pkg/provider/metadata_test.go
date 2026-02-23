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

package provider

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseProviderMetadata_ValidMetadata(t *testing.T) {
	validYAML := `
apiVersion: llamastack.io/v1alpha1
kind: ProviderPackage
metadata:
  name: custom-vllm
  version: 1.0.0
  vendor: my-org
  description: Custom vLLM provider
  maintainer: team@example.com
spec:
  packageName: my_org.custom_vllm
  providerType: remote::custom-vllm
  api: inference
  wheelPath: /lls-provider/packages/custom_vllm-1.0.0-py3-none-any.whl
  dependencyWheels:
    - /lls-provider/packages/dep1-1.0.0-py3-none-any.whl
    - /lls-provider/packages/dep2-2.0.0-py3-none-any.whl
`

	metadata, err := ParseProviderMetadata([]byte(validYAML))
	require.NoError(t, err)

	assert.Equal(t, MetadataAPIVersion, metadata.APIVersion)
	assert.Equal(t, MetadataKind, metadata.Kind)
	assert.Equal(t, "custom-vllm", metadata.Metadata.Name)
	assert.Equal(t, "1.0.0", metadata.Metadata.Version)
	assert.Equal(t, "my-org", metadata.Metadata.Vendor)
	assert.Equal(t, "Custom vLLM provider", metadata.Metadata.Description)
	assert.Equal(t, "team@example.com", metadata.Metadata.Maintainer)
	assert.Equal(t, "my_org.custom_vllm", metadata.Spec.PackageName)
	assert.Equal(t, "remote::custom-vllm", metadata.Spec.ProviderType)
	assert.Equal(t, "inference", metadata.Spec.API)
	assert.Equal(t, "/lls-provider/packages/custom_vllm-1.0.0-py3-none-any.whl", metadata.Spec.WheelPath)
	assert.Len(t, metadata.Spec.DependencyWheels, 2)
}

func TestParseProviderMetadata_MinimalValid(t *testing.T) {
	minimalYAML := `
apiVersion: llamastack.io/v1alpha1
kind: ProviderPackage
metadata:
  name: minimal-provider
  version: 0.1.0
  vendor: test
spec:
  packageName: minimal_provider
  providerType: inline::minimal
  api: safety
  wheelPath: /lls-provider/packages/minimal-0.1.0.whl
`

	metadata, err := ParseProviderMetadata([]byte(minimalYAML))
	require.NoError(t, err)

	assert.Equal(t, "minimal-provider", metadata.Metadata.Name)
	assert.Equal(t, "safety", metadata.Spec.API)
	assert.Empty(t, metadata.Metadata.Description)
	assert.Empty(t, metadata.Spec.DependencyWheels)
}

func TestParseProviderMetadata_InvalidYAML(t *testing.T) {
	invalidYAML := `
apiVersion: llamastack.io/v1alpha1
kind: ProviderPackage
metadata:
  name: [invalid yaml structure
`

	_, err := ParseProviderMetadata([]byte(invalidYAML))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse metadata YAML")
}

func TestParseProviderMetadata_InvalidAPIVersion(t *testing.T) {
	invalidAPIVersion := `
apiVersion: wrong/v1
kind: ProviderPackage
metadata:
  name: test
  version: 1.0.0
  vendor: test
spec:
  packageName: test_pkg
  providerType: remote::test
  api: inference
  wheelPath: /wheel.whl
`

	_, err := ParseProviderMetadata([]byte(invalidAPIVersion))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "apiVersion")
}

func TestParseProviderMetadata_InvalidKind(t *testing.T) {
	invalidKind := `
apiVersion: llamastack.io/v1alpha1
kind: WrongKind
metadata:
  name: test
  version: 1.0.0
  vendor: test
spec:
  packageName: test_pkg
  providerType: remote::test
  api: inference
  wheelPath: /wheel.whl
`

	_, err := ParseProviderMetadata([]byte(invalidKind))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "kind")
}

func TestParseProviderMetadata_AllValidAPIs(t *testing.T) {
	apis := []string{"inference", "safety", "agents", "vector_io", "datasetio", "scoring", "eval", "tool_runtime", "post_training"}

	for _, api := range apis {
		t.Run(api, func(t *testing.T) {
			yaml := `
apiVersion: llamastack.io/v1alpha1
kind: ProviderPackage
metadata:
  name: test
  version: 1.0.0
  vendor: test
spec:
  packageName: test_pkg
  providerType: remote::test
  api: ` + api + `
  wheelPath: /wheel.whl
`
			metadata, err := ParseProviderMetadata([]byte(yaml))
			require.NoError(t, err)
			assert.Equal(t, api, metadata.Spec.API)
		})
	}
}

func TestParseProviderMetadata_InvalidAPI(t *testing.T) {
	invalidAPI := `
apiVersion: llamastack.io/v1alpha1
kind: ProviderPackage
metadata:
  name: test
  version: 1.0.0
  vendor: test
spec:
  packageName: test_pkg
  providerType: remote::test
  api: invalid_api
  wheelPath: /wheel.whl
`

	_, err := ParseProviderMetadata([]byte(invalidAPI))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "spec.api")
}

func TestLoadProviderMetadata_FileNotFound(t *testing.T) {
	_, err := LoadProviderMetadata("/nonexistent/path/metadata.yaml")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to read metadata file")
}

func TestLoadProviderMetadata_ValidFile(t *testing.T) {
	// Create a temporary file with valid metadata
	content := `
apiVersion: llamastack.io/v1alpha1
kind: ProviderPackage
metadata:
  name: file-test
  version: 1.0.0
  vendor: test
spec:
  packageName: file_test
  providerType: remote::file-test
  api: inference
  wheelPath: /wheel.whl
`

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "lls-provider-spec.yaml")
	err := os.WriteFile(tmpFile, []byte(content), 0o644)
	require.NoError(t, err)

	metadata, err := LoadProviderMetadata(tmpFile)
	require.NoError(t, err)
	assert.Equal(t, "file-test", metadata.Metadata.Name)
}
