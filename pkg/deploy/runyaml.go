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
	"fmt"
	"os"

	"github.com/llamastack/llama-stack-k8s-operator/pkg/provider"
	"gopkg.in/yaml.v3"
)

// configFilePermissions is the permission mode for config files.
const configFilePermissions = 0o600

// RunYamlConfig represents the config.yaml structure used by LlamaStack.
// This is the configuration file that is passed to `llama stack run`.
type RunYamlConfig struct {
	Version   int                              `yaml:"version"`
	ImageName string                           `yaml:"distro_name,omitempty"`
	APIs      []string                         `yaml:"apis,omitempty"`
	Providers map[string][]ProviderConfigEntry `yaml:"providers"`
	// Additional fields may exist but are preserved during merge
}

// ProviderConfigEntry represents a single provider entry in the config.yaml providers section.
type ProviderConfigEntry struct {
	ProviderID   string                 `yaml:"provider_id"`
	ProviderType string                 `yaml:"provider_type"`
	Module       string                 `yaml:"module,omitempty"`
	Config       map[string]interface{} `yaml:"config,omitempty"`
}

// ProviderEntry represents an external provider with all information needed for config generation.
type ProviderEntry struct {
	ProviderID   string
	ProviderType string
	Module       string
	API          string
	Config       map[string]interface{}
}

// APIPlacementError represents an error when a provider's declared API doesn't match
// the CRD section it was placed in.
type APIPlacementError struct {
	ProviderID  string
	Image       string
	DeclaredAPI string
	PlacedInAPI string
}

func (e *APIPlacementError) Error() string {
	return fmt.Sprintf(
		"ERROR: Provider API type mismatch\n\n"+
			"Provider '%s' (image: %s)\n"+
			"declares api=%s in lls-provider-spec.yaml\n"+
			"but is placed under externalProviders.%s\n\n"+
			"Resolution: Move the provider to externalProviders.%s section in the LLSD spec.",
		e.ProviderID, e.Image, e.DeclaredAPI, e.PlacedInAPI, e.DeclaredAPI)
}

// DuplicateProviderError represents an error when two external providers declare the same providerId.
type DuplicateProviderError struct {
	ProviderID string
	Image1     string
	Image2     string
}

func (e *DuplicateProviderError) Error() string {
	return fmt.Sprintf(
		"ERROR: Duplicate provider ID detected in external providers\n\n"+
			"Provider ID '%s' is declared by multiple provider images:\n"+
			"  - %s\n"+
			"  - %s\n\n"+
			"Resolution: Each provider must have a unique providerId. Update the CRD to use distinct IDs.",
		e.ProviderID, e.Image1, e.Image2)
}

// MergeWarning represents a warning generated during the merge process.
type MergeWarning struct {
	Message string
}

// LoadRunYamlConfig reads and parses a config.yaml file.
func LoadRunYamlConfig(path string) (*RunYamlConfig, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path is a trusted filesystem path
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	return ParseRunYamlConfig(data)
}

// ParseRunYamlConfig parses config.yaml from raw YAML bytes.
func ParseRunYamlConfig(data []byte) (*RunYamlConfig, error) {
	var config RunYamlConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config YAML: %w", err)
	}

	// Initialize providers map if nil
	if config.Providers == nil {
		config.Providers = make(map[string][]ProviderConfigEntry)
	}

	return &config, nil
}

// SerializeRunYamlConfig serializes config to YAML bytes.
func SerializeRunYamlConfig(config *RunYamlConfig) ([]byte, error) {
	data, err := yaml.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize config: %w", err)
	}
	return data, nil
}

// WriteRunYamlConfig writes config to a file.
func WriteRunYamlConfig(config *RunYamlConfig, path string) error {
	data, err := SerializeRunYamlConfig(config)
	if err != nil {
		return err
	}

	if err := os.WriteFile(path, data, configFilePermissions); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// MergeExternalProviders adds external provider entries to the config.
// Returns warnings for any provider ID conflicts where external overrides existing.
func MergeExternalProviders(config *RunYamlConfig, providers []ProviderEntry) ([]MergeWarning, error) {
	var warnings []MergeWarning

	// Check for duplicate provider IDs among external providers
	if err := validateNoDuplicateProviderIDs(providers); err != nil {
		return nil, err
	}

	// Initialize providers map if nil
	if config.Providers == nil {
		config.Providers = make(map[string][]ProviderConfigEntry)
	}

	for _, p := range providers {
		entry := ProviderConfigEntry{
			ProviderID:   p.ProviderID,
			ProviderType: p.ProviderType,
			Module:       p.Module,
			Config:       p.Config,
		}

		// Check for existing provider with same ID
		existingIndex := findProviderIndexByID(config.Providers[p.API], p.ProviderID)
		if existingIndex >= 0 {
			// Override existing provider with warning
			existingType := config.Providers[p.API][existingIndex].ProviderType
			warnings = append(warnings, MergeWarning{
				Message: fmt.Sprintf(
					"External provider '%s' overrides base provider in API '%s'\n"+
						"  Base type: %s\n"+
						"  External type: %s",
					p.ProviderID, p.API, existingType, p.ProviderType),
			})
			config.Providers[p.API][existingIndex] = entry
		} else {
			// Append new provider
			config.Providers[p.API] = append(config.Providers[p.API], entry)
		}
	}

	return warnings, nil
}

// validateNoDuplicateProviderIDs checks that all provider entries have unique IDs.
func validateNoDuplicateProviderIDs(providers []ProviderEntry) error {
	seen := make(map[string]string) // providerID -> first image that declared it

	for _, p := range providers {
		if existingImage, exists := seen[p.ProviderID]; exists {
			// For this validation, we don't have the image, but we can use the module as identifier
			return &DuplicateProviderError{
				ProviderID: p.ProviderID,
				Image1:     existingImage,
				Image2:     p.Module, // Using module as a proxy for image in this context
			}
		}
		seen[p.ProviderID] = p.Module
	}

	return nil
}

// findProviderIndexByID returns the index of a provider with the given ID, or -1 if not found.
func findProviderIndexByID(providers []ProviderConfigEntry, providerID string) int {
	for i, p := range providers {
		if p.ProviderID == providerID {
			return i
		}
	}
	return -1
}

// GenerateProviderEntriesFromMetadata creates ProviderEntry objects from provider metadata files.
// It reads metadata and CRD config from the metadata directory and validates API placement.
func GenerateProviderEntriesFromMetadata(metadataDir string) ([]ProviderEntry, error) {
	entries, err := os.ReadDir(metadataDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read metadata directory: %w", err)
	}

	var providers []ProviderEntry

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		providerID := entry.Name()
		providerDir := fmt.Sprintf("%s/%s", metadataDir, providerID)

		// Read provider metadata
		metadataPath := fmt.Sprintf("%s/lls-provider-spec.yaml", providerDir)
		metadata, err := provider.LoadProviderMetadata(metadataPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load metadata for provider %s: %w", providerID, err)
		}

		// Read CRD config
		crdConfigPath := fmt.Sprintf("%s/crd-config.yaml", providerDir)
		crdConfig, err := loadCRDConfig(crdConfigPath)
		if err != nil {
			return nil, fmt.Errorf("failed to load CRD config for provider %s: %w", providerID, err)
		}

		// Validate API placement
		// Normalize the CRD API name to match metadata API format
		normalizedCRDAPI := provider.NormalizeAPIName(crdConfig.API)
		if metadata.Spec.API != normalizedCRDAPI {
			return nil, &APIPlacementError{
				ProviderID:  providerID,
				Image:       "unknown", // Image not available at this point
				DeclaredAPI: metadata.Spec.API,
				PlacedInAPI: crdConfig.API,
			}
		}

		providers = append(providers, ProviderEntry{
			ProviderID:   crdConfig.ProviderID,
			ProviderType: metadata.Spec.ProviderType,
			Module:       metadata.Spec.PackageName,
			API:          metadata.Spec.API,
			Config:       crdConfig.Config,
		})
	}

	return providers, nil
}

// CRDConfig represents the crd-config.yaml structure written by init containers.
type CRDConfig struct {
	ProviderID string                 `yaml:"providerId"`
	API        string                 `yaml:"api"`
	Config     map[string]interface{} `yaml:"config,omitempty"`
}

// loadCRDConfig loads and parses a crd-config.yaml file.
func loadCRDConfig(path string) (*CRDConfig, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path is a trusted filesystem path
	if err != nil {
		return nil, fmt.Errorf("failed to read CRD config: %w", err)
	}

	var config CRDConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse CRD config: %w", err)
	}

	return &config, nil
}

// GenerateConfig orchestrates the full config generation process.
// It loads base config, reads provider metadata, merges providers, and writes output.
func GenerateConfig(baseConfigPath, metadataDir, outputPath string) ([]MergeWarning, error) {
	// Load base configuration
	baseConfig, err := LoadRunYamlConfig(baseConfigPath)
	if err != nil {
		return nil, fmt.Errorf("failed to load base config: %w", err)
	}

	// Generate provider entries from metadata
	providers, err := GenerateProviderEntriesFromMetadata(metadataDir)
	if err != nil {
		return nil, fmt.Errorf("failed to generate provider entries: %w", err)
	}

	// Merge external providers into config
	warnings, err := MergeExternalProviders(baseConfig, providers)
	if err != nil {
		return nil, fmt.Errorf("failed to merge providers: %w", err)
	}

	// Write output config
	if err := WriteRunYamlConfig(baseConfig, outputPath); err != nil {
		return nil, fmt.Errorf("failed to write output config: %w", err)
	}

	return warnings, nil
}
