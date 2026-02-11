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
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// ValidAPIs is the set of valid LlamaStack API types.
var ValidAPIs = map[string]bool{
	"inference":     true,
	"safety":        true,
	"agents":        true,
	"vector_io":     true,
	"datasetio":     true,
	"scoring":       true,
	"eval":          true,
	"tool_runtime":  true,
	"post_training": true,
}

// providerTypePattern validates provider type format: (remote|inline)::[a-z0-9-]+.
var providerTypePattern = regexp.MustCompile(`^(remote|inline)::[a-z0-9-]+$`)

// ValidationError represents a validation error with context.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ValidateMetadata validates provider metadata structure and values.
// It returns an error if any required field is missing or has an invalid value.
func ValidateMetadata(m *ProviderMetadata) error {
	if err := validateMetadataHeader(m); err != nil {
		return err
	}

	if err := validateMetadataFields(m); err != nil {
		return err
	}

	return validateSpecFields(m)
}

// validateMetadataHeader validates the apiVersion and kind fields.
func validateMetadataHeader(m *ProviderMetadata) error {
	if m.APIVersion != MetadataAPIVersion {
		return &ValidationError{
			Field:   "apiVersion",
			Message: fmt.Sprintf("expected %s, got %s", MetadataAPIVersion, m.APIVersion),
		}
	}

	if m.Kind != MetadataKind {
		return &ValidationError{
			Field:   "kind",
			Message: fmt.Sprintf("expected %s, got %s", MetadataKind, m.Kind),
		}
	}

	return nil
}

// validateMetadataFields validates the required metadata section fields.
func validateMetadataFields(m *ProviderMetadata) error {
	if m.Metadata.Name == "" {
		return &ValidationError{
			Field:   "metadata.name",
			Message: "is required",
		}
	}

	if m.Metadata.Version == "" {
		return &ValidationError{
			Field:   "metadata.version",
			Message: "is required",
		}
	}

	if m.Metadata.Vendor == "" {
		return &ValidationError{
			Field:   "metadata.vendor",
			Message: "is required",
		}
	}

	return nil
}

// validateSpecFields validates the required spec section fields.
func validateSpecFields(m *ProviderMetadata) error {
	if err := validatePackageName(m.Spec.PackageName); err != nil {
		return err
	}

	if err := validateProviderType(m.Spec.ProviderType); err != nil {
		return err
	}

	if err := validateAPI(m.Spec.API); err != nil {
		return err
	}

	if m.Spec.WheelPath == "" {
		return &ValidationError{
			Field:   "spec.wheelPath",
			Message: "is required",
		}
	}

	return nil
}

func validatePackageName(packageName string) error {
	if packageName == "" {
		return &ValidationError{
			Field:   "spec.packageName",
			Message: "is required",
		}
	}

	if err := validatePythonModuleName(packageName); err != nil {
		return &ValidationError{
			Field:   "spec.packageName",
			Message: err.Error(),
		}
	}

	return nil
}

func validateProviderType(providerType string) error {
	if providerType == "" {
		return &ValidationError{
			Field:   "spec.providerType",
			Message: "is required",
		}
	}

	if !providerTypePattern.MatchString(providerType) {
		return &ValidationError{
			Field:   "spec.providerType",
			Message: fmt.Sprintf("must match pattern (remote|inline)::[a-z0-9-]+, got %s", providerType),
		}
	}

	return nil
}

func validateAPI(api string) error {
	if api == "" {
		return &ValidationError{
			Field:   "spec.api",
			Message: "is required",
		}
	}

	if !ValidAPIs[api] {
		validAPIsList := make([]string, 0, len(ValidAPIs))
		for validAPI := range ValidAPIs {
			validAPIsList = append(validAPIsList, validAPI)
		}
		return &ValidationError{
			Field:   "spec.api",
			Message: fmt.Sprintf("must be one of %v, got %s", validAPIsList, api),
		}
	}

	return nil
}

// validatePythonModuleName validates that a string is a valid Python module identifier.
// Python module names must be valid Python identifiers separated by dots.
func validatePythonModuleName(name string) error {
	if name == "" {
		return errors.New("failed to validate: module name cannot be empty")
	}

	// Python identifier pattern: starts with letter or underscore, followed by letters, digits, or underscores
	identifierPattern := regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

	parts := strings.Split(name, ".")
	for _, part := range parts {
		if part == "" {
			return errors.New("failed to validate: contains empty component")
		}
		if !identifierPattern.MatchString(part) {
			return fmt.Errorf("failed to validate: component '%s' is not a valid Python identifier", part)
		}
	}

	return nil
}

// NormalizeAPIName converts CRD API field names to config.yaml API names.
// CRD uses camelCase (vectorIo, datasetIo, toolRuntime, postTraining)
// while config.yaml uses snake_case (vector_io, datasetio, tool_runtime, post_training).
func NormalizeAPIName(crdAPIName string) string {
	mapping := map[string]string{
		"vectorIo":     "vector_io",
		"datasetIo":    "datasetio",
		"toolRuntime":  "tool_runtime",
		"postTraining": "post_training",
	}

	if normalized, ok := mapping[crdAPIName]; ok {
		return normalized
	}
	return crdAPIName
}

// CRDToConfigAPIName converts CRD section names to config.yaml API names.
func CRDToConfigAPIName(crdSection string) string {
	return NormalizeAPIName(crdSection)
}

// ConfigToCRDAPIName converts config.yaml API names to CRD section names.
func ConfigToCRDAPIName(configAPI string) string {
	mapping := map[string]string{
		"vector_io":     "vectorIo",
		"datasetio":     "datasetIo",
		"tool_runtime":  "toolRuntime",
		"post_training": "postTraining",
	}

	if crdName, ok := mapping[configAPI]; ok {
		return crdName
	}
	return configAPI
}
