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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateMetadata_MissingName(t *testing.T) {
	m := &ProviderMetadata{
		APIVersion: MetadataAPIVersion,
		Kind:       MetadataKind,
		Metadata: ProviderMetadataMetadata{
			Version: "1.0.0",
			Vendor:  "test",
		},
		Spec: ProviderMetadataSpec{
			PackageName:  "test_pkg",
			ProviderType: "remote::test",
			API:          "inference",
			WheelPath:    "/wheel.whl",
		},
	}

	err := ValidateMetadata(m)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "metadata.name")
}

func TestValidateMetadata_MissingVersion(t *testing.T) {
	m := &ProviderMetadata{
		APIVersion: MetadataAPIVersion,
		Kind:       MetadataKind,
		Metadata: ProviderMetadataMetadata{
			Name:   "test",
			Vendor: "test",
		},
		Spec: ProviderMetadataSpec{
			PackageName:  "test_pkg",
			ProviderType: "remote::test",
			API:          "inference",
			WheelPath:    "/wheel.whl",
		},
	}

	err := ValidateMetadata(m)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "metadata.version")
}

func TestValidateMetadata_MissingVendor(t *testing.T) {
	m := &ProviderMetadata{
		APIVersion: MetadataAPIVersion,
		Kind:       MetadataKind,
		Metadata: ProviderMetadataMetadata{
			Name:    "test",
			Version: "1.0.0",
		},
		Spec: ProviderMetadataSpec{
			PackageName:  "test_pkg",
			ProviderType: "remote::test",
			API:          "inference",
			WheelPath:    "/wheel.whl",
		},
	}

	err := ValidateMetadata(m)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "metadata.vendor")
}

func TestValidateMetadata_MissingPackageName(t *testing.T) {
	m := &ProviderMetadata{
		APIVersion: MetadataAPIVersion,
		Kind:       MetadataKind,
		Metadata: ProviderMetadataMetadata{
			Name:    "test",
			Version: "1.0.0",
			Vendor:  "test",
		},
		Spec: ProviderMetadataSpec{
			ProviderType: "remote::test",
			API:          "inference",
			WheelPath:    "/wheel.whl",
		},
	}

	err := ValidateMetadata(m)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "spec.packageName")
}

func TestValidateMetadata_InvalidPackageName(t *testing.T) {
	testCases := []struct {
		name        string
		packageName string
		shouldError bool
	}{
		{"valid simple", "mypackage", false},
		{"valid with underscore", "my_package", false},
		{"valid dotted", "my_org.my_package", false},
		{"valid nested", "my_org.sub.package", false},
		{"starts with number", "1package", true},
		{"contains hyphen", "my-package", true},
		{"contains space", "my package", true},
		{"empty component", "my..package", true},
		{"leading dot", ".mypackage", true},
		{"trailing dot", "mypackage.", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			m := &ProviderMetadata{
				APIVersion: MetadataAPIVersion,
				Kind:       MetadataKind,
				Metadata: ProviderMetadataMetadata{
					Name:    "test",
					Version: "1.0.0",
					Vendor:  "test",
				},
				Spec: ProviderMetadataSpec{
					PackageName:  tc.packageName,
					ProviderType: "remote::test",
					API:          "inference",
					WheelPath:    "/wheel.whl",
				},
			}

			err := ValidateMetadata(m)
			if tc.shouldError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "spec.packageName")
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateMetadata_InvalidProviderType(t *testing.T) {
	testCases := []struct {
		name         string
		providerType string
		shouldError  bool
	}{
		{"valid remote", "remote::my-provider", false},
		{"valid inline", "inline::my-provider", false},
		{"valid with numbers", "remote::provider-123", false},
		{"missing prefix", "my-provider", true},
		{"wrong prefix", "local::my-provider", true},
		{"uppercase", "REMOTE::my-provider", true},
		{"missing name", "remote::", true},
		{"no double colon", "remote:my-provider", true},
		{"invalid chars in name", "remote::my_provider", true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			m := &ProviderMetadata{
				APIVersion: MetadataAPIVersion,
				Kind:       MetadataKind,
				Metadata: ProviderMetadataMetadata{
					Name:    "test",
					Version: "1.0.0",
					Vendor:  "test",
				},
				Spec: ProviderMetadataSpec{
					PackageName:  "test_pkg",
					ProviderType: tc.providerType,
					API:          "inference",
					WheelPath:    "/wheel.whl",
				},
			}

			err := ValidateMetadata(m)
			if tc.shouldError {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "spec.providerType")
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestValidateMetadata_MissingWheelPath(t *testing.T) {
	m := &ProviderMetadata{
		APIVersion: MetadataAPIVersion,
		Kind:       MetadataKind,
		Metadata: ProviderMetadataMetadata{
			Name:    "test",
			Version: "1.0.0",
			Vendor:  "test",
		},
		Spec: ProviderMetadataSpec{
			PackageName:  "test_pkg",
			ProviderType: "remote::test",
			API:          "inference",
		},
	}

	err := ValidateMetadata(m)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "spec.wheelPath")
}

func TestNormalizeAPIName(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"inference", "inference"},
		{"safety", "safety"},
		{"agents", "agents"},
		{"vectorIo", "vector_io"},
		{"datasetIo", "datasetio"},
		{"scoring", "scoring"},
		{"eval", "eval"},
		{"toolRuntime", "tool_runtime"},
		{"postTraining", "post_training"},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result := NormalizeAPIName(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestConfigToCRDAPIName(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{"inference", "inference"},
		{"safety", "safety"},
		{"agents", "agents"},
		{"vector_io", "vectorIo"},
		{"datasetio", "datasetIo"},
		{"scoring", "scoring"},
		{"eval", "eval"},
		{"tool_runtime", "toolRuntime"},
		{"post_training", "postTraining"},
	}

	for _, tc := range testCases {
		t.Run(tc.input, func(t *testing.T) {
			result := ConfigToCRDAPIName(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestValidationError(t *testing.T) {
	err := &ValidationError{
		Field:   "test.field",
		Message: "test error message",
	}

	assert.Equal(t, "test.field: test error message", err.Error())
}
