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

// Package provider implements external provider metadata parsing and validation.
package provider

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

const (
	// MetadataAPIVersion is the expected apiVersion for provider metadata files.
	MetadataAPIVersion = "llamastack.io/v1alpha1"
	// MetadataKind is the expected kind for provider metadata files.
	MetadataKind = "ProviderPackage"
	// MetadataPath is the standard path for provider metadata in container images.
	MetadataPath = "/lls-provider/lls-provider-spec.yaml"
	// PackagesPath is the standard path for provider wheel packages in container images.
	PackagesPath = "/lls-provider/packages"
)

// ProviderMetadata represents the lls-provider-spec.yaml structure.
// This metadata file is required in all external provider container images
// and describes the provider package contents.
type ProviderMetadata struct {
	APIVersion string                   `yaml:"apiVersion"`
	Kind       string                   `yaml:"kind"`
	Metadata   ProviderMetadataMetadata `yaml:"metadata"`
	Spec       ProviderMetadataSpec     `yaml:"spec"`
}

// ProviderMetadataMetadata contains descriptive metadata about the provider package.
type ProviderMetadataMetadata struct {
	// Name is the package identifier (required)
	Name string `yaml:"name"`
	// Version is the semantic version of the package (required)
	Version string `yaml:"version"`
	// Vendor is the organization identifier (required)
	Vendor string `yaml:"vendor"`
	// Description is a human-readable description (optional)
	Description string `yaml:"description,omitempty"`
	// Maintainer is the contact email (optional)
	Maintainer string `yaml:"maintainer,omitempty"`
}

// ProviderMetadataSpec contains the technical specification of the provider.
type ProviderMetadataSpec struct {
	// PackageName is the Python module path (e.g., "my_org.custom_vllm").
	// LlamaStack imports this using importlib.import_module() and calls
	// get_provider_impl() or get_adapter_impl().
	PackageName string `yaml:"packageName"`
	// ProviderType is the provider type (e.g., "remote::custom-vllm")
	ProviderType string `yaml:"providerType"`
	// API is the API name (e.g., "inference", "safety", "agents")
	API string `yaml:"api"`
	// WheelPath is the path to the main wheel file
	WheelPath string `yaml:"wheelPath"`
	// DependencyWheels is an optional list of paths to dependency wheel files
	DependencyWheels []string `yaml:"dependencyWheels,omitempty"`
}

// LoadProviderMetadata reads and parses provider metadata from a file path.
// It validates the metadata structure and returns any errors encountered.
func LoadProviderMetadata(path string) (*ProviderMetadata, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path is a trusted filesystem path
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata file: %w", err)
	}

	return ParseProviderMetadata(data)
}

// ParseProviderMetadata parses provider metadata from raw YAML bytes.
// It validates the metadata structure and returns any errors encountered.
func ParseProviderMetadata(data []byte) (*ProviderMetadata, error) {
	var metadata ProviderMetadata
	if err := yaml.Unmarshal(data, &metadata); err != nil {
		return nil, fmt.Errorf("failed to parse metadata YAML: %w", err)
	}

	if err := ValidateMetadata(&metadata); err != nil {
		return nil, fmt.Errorf("failed to validate metadata: %w", err)
	}

	return &metadata, nil
}
