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

package v1alpha2

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func newValidator(distNames ...string) *LlamaStackDistributionValidator {
	return &LlamaStackDistributionValidator{EmbeddedDistributionNames: distNames}
}

func newCR(spec LlamaStackDistributionSpec) *LlamaStackDistribution {
	return &LlamaStackDistribution{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
		Spec:       spec,
	}
}

// --- deriveProviderID ---

func TestDeriveProviderID(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"vllm", "vllm"},
		{"remote::vllm", "vllm"},
		{"inline::meta-reference", "meta-reference"},
		{"remote::ollama", "ollama"},
		{"pgvector", "pgvector"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.want, deriveProviderID(tt.input))
		})
	}
}

// --- Distribution name validation ---

func TestValidateDistributionNameKnown(t *testing.T) {
	errs := validateDistributionName("starter", []string{"starter", "postgres-demo"})
	assert.Empty(t, errs)
}

func TestValidateDistributionNameUnknown(t *testing.T) {
	errs := validateDistributionName("nonexistent", []string{"starter", "postgres-demo"})
	require.Len(t, errs, 1)
	assert.Contains(t, errs[0].Detail, "unknown distribution")
	assert.Contains(t, errs[0].Detail, "starter")
}

func TestValidateDistributionNameEmptyRegistry(t *testing.T) {
	errs := validateDistributionName("anything", nil)
	assert.Empty(t, errs, "empty registry should skip validation")
}

// --- Provider ID uniqueness ---

func TestProviderIDUniquenessNoDuplicates(t *testing.T) {
	spec := &ProvidersSpec{
		Inference: []ProviderConfig{{Provider: "vllm"}},
		Safety:    []ProviderConfig{{Provider: "llama-guard"}},
	}
	errs := validateProviderIDUniqueness(spec)
	assert.Empty(t, errs)
}

func TestProviderIDUniquenessExplicitIDs(t *testing.T) {
	spec := &ProvidersSpec{
		Inference: []ProviderConfig{{ID: "my-provider", Provider: "vllm"}},
		Safety:    []ProviderConfig{{ID: "my-provider", Provider: "llama-guard"}},
	}
	errs := validateProviderIDUniqueness(spec)
	require.Len(t, errs, 1)
	assert.Contains(t, errs[0].Detail, "conflicts with provider in")
}

func TestProviderIDUniquenessDerivedCollision(t *testing.T) {
	spec := &ProvidersSpec{
		Inference:   []ProviderConfig{{Provider: "remote::vllm"}},
		ToolRuntime: []ProviderConfig{{Provider: "vllm"}},
	}
	errs := validateProviderIDUniqueness(spec)
	require.Len(t, errs, 1,
		"remote::vllm and vllm both derive to 'vllm', causing a collision")
}

func TestProviderIDUniquenessEmptySlices(t *testing.T) {
	spec := &ProvidersSpec{}
	errs := validateProviderIDUniqueness(spec)
	assert.Empty(t, errs)
}

func TestProviderIDUniquenessMultipleCollisions(t *testing.T) {
	spec := &ProvidersSpec{
		Inference: []ProviderConfig{{ID: "dup", Provider: "vllm"}},
		Safety:    []ProviderConfig{{ID: "dup", Provider: "guard"}},
		Telemetry: []ProviderConfig{{ID: "dup", Provider: "otel"}},
	}
	errs := validateProviderIDUniqueness(spec)
	assert.Len(t, errs, 2, "should report conflict for safety and telemetry")
}

// --- Provider reference validation ---

func TestProviderReferencesValid(t *testing.T) {
	providers := &ProvidersSpec{
		Inference: []ProviderConfig{{ID: "vllm", Provider: "vllm"}},
	}
	resources := &ResourcesSpec{
		Models: []ModelConfig{{Name: "llama3", Provider: "vllm"}},
	}
	errs := validateProviderReferences(resources, providers)
	assert.Empty(t, errs)
}

func TestProviderReferencesInvalid(t *testing.T) {
	providers := &ProvidersSpec{
		Inference: []ProviderConfig{{ID: "vllm", Provider: "vllm"}},
	}
	resources := &ResourcesSpec{
		Models: []ModelConfig{{Name: "llama3", Provider: "nonexistent"}},
	}
	errs := validateProviderReferences(resources, providers)
	require.Len(t, errs, 1)
	assert.Contains(t, errs[0].Detail, "references unknown provider ID")
	assert.Contains(t, errs[0].Detail, "vllm")
}

func TestProviderReferencesOmittedProvider(t *testing.T) {
	providers := &ProvidersSpec{
		Inference: []ProviderConfig{{Provider: "vllm"}},
	}
	resources := &ResourcesSpec{
		Models: []ModelConfig{{Name: "llama3"}},
	}
	errs := validateProviderReferences(resources, providers)
	assert.Empty(t, errs, "omitted provider should not be validated (uses default)")
}

func TestProviderReferencesDerivedID(t *testing.T) {
	providers := &ProvidersSpec{
		Inference: []ProviderConfig{{Provider: "remote::ollama"}},
	}
	resources := &ResourcesSpec{
		Models: []ModelConfig{{Name: "llama3", Provider: "ollama"}},
	}
	errs := validateProviderReferences(resources, providers)
	assert.Empty(t, errs,
		"model referencing derived provider ID 'ollama' (from remote::ollama) should be valid")
}

func TestProviderReferencesNoModels(t *testing.T) {
	providers := &ProvidersSpec{
		Inference: []ProviderConfig{{Provider: "vllm"}},
	}
	resources := &ResourcesSpec{}
	errs := validateProviderReferences(resources, providers)
	assert.Empty(t, errs)
}

// --- collectAllProviderIDs ---

func TestCollectAllProviderIDs(t *testing.T) {
	spec := &ProvidersSpec{
		Inference: []ProviderConfig{
			{ID: "my-vllm", Provider: "vllm"},
			{Provider: "remote::ollama"},
		},
		Safety:   []ProviderConfig{{Provider: "llama-guard"}},
		VectorIo: []ProviderConfig{{ID: "pgvec", Provider: "pgvector"}},
	}

	ids := collectAllProviderIDs(spec)
	assert.True(t, ids["my-vllm"])
	assert.True(t, ids["ollama"], "remote::ollama derives to ollama")
	assert.True(t, ids["llama-guard"])
	assert.True(t, ids["pgvec"])
	assert.Len(t, ids, 4)
}

// --- Full validate (integration-style) ---

func TestValidateCreateAcceptsValidCR(t *testing.T) {
	v := newValidator("starter")
	cr := newCR(LlamaStackDistributionSpec{
		Distribution: DistributionSpec{Name: "starter"},
		Providers: &ProvidersSpec{
			Inference: []ProviderConfig{{Provider: "vllm", Endpoint: "http://vllm:8000"}},
		},
		Resources: &ResourcesSpec{
			Models: []ModelConfig{{Name: "llama3", Provider: "vllm"}},
		},
	})

	warnings, err := v.ValidateCreate(context.Background(), cr)
	require.NoError(t, err)
	assert.Empty(t, warnings)
}

func TestValidateCreateRejectsUnknownDistribution(t *testing.T) {
	v := newValidator("starter", "postgres-demo")
	cr := newCR(LlamaStackDistributionSpec{
		Distribution: DistributionSpec{Name: "bad-distro"},
	})

	_, err := v.ValidateCreate(context.Background(), cr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown distribution")
}

func TestValidateCreateRejectsDuplicateProviderIDs(t *testing.T) {
	v := newValidator("starter")
	cr := newCR(LlamaStackDistributionSpec{
		Distribution: DistributionSpec{Name: "starter"},
		Providers: &ProvidersSpec{
			Inference: []ProviderConfig{{ID: "dup", Provider: "vllm"}},
			Safety:    []ProviderConfig{{ID: "dup", Provider: "guard"}},
		},
	})

	_, err := v.ValidateCreate(context.Background(), cr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "conflicts with provider")
}

func TestValidateCreateRejectsInvalidProviderRef(t *testing.T) {
	v := newValidator("starter")
	cr := newCR(LlamaStackDistributionSpec{
		Distribution: DistributionSpec{Name: "starter"},
		Providers: &ProvidersSpec{
			Inference: []ProviderConfig{{Provider: "vllm"}},
		},
		Resources: &ResourcesSpec{
			Models: []ModelConfig{{Name: "m1", Provider: "nonexistent"}},
		},
	})

	_, err := v.ValidateCreate(context.Background(), cr)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "references unknown provider ID")
}

func TestValidateUpdateUsesNewObject(t *testing.T) {
	v := newValidator("starter")
	oldCR := newCR(LlamaStackDistributionSpec{
		Distribution: DistributionSpec{Name: "starter"},
	})
	newCR := newCR(LlamaStackDistributionSpec{
		Distribution: DistributionSpec{Name: "bad-distro"},
	})

	_, err := v.ValidateUpdate(context.Background(), oldCR, newCR)
	require.Error(t, err, "update validation should use the new object")
}

func TestValidateDeleteAlwaysSucceeds(t *testing.T) {
	v := newValidator("starter")
	cr := newCR(LlamaStackDistributionSpec{})

	_, err := v.ValidateDelete(context.Background(), cr)
	require.NoError(t, err)
}

func TestValidateNilProvidersAndResources(t *testing.T) {
	v := newValidator("starter")
	cr := newCR(LlamaStackDistributionSpec{
		Distribution: DistributionSpec{Name: "starter"},
	})

	warnings, err := v.ValidateCreate(context.Background(), cr)
	require.NoError(t, err)
	assert.Empty(t, warnings)
}

func TestValidateResourcesWithoutProviders(t *testing.T) {
	v := newValidator("starter")
	cr := newCR(LlamaStackDistributionSpec{
		Distribution: DistributionSpec{Name: "starter"},
		Resources: &ResourcesSpec{
			Models: []ModelConfig{{Name: "llama3", Provider: "vllm"}},
		},
	})

	warnings, err := v.ValidateCreate(context.Background(), cr)
	require.NoError(t, err, "provider ref validation is skipped when providers is nil")
	assert.Empty(t, warnings)
}

func TestValidateWrongObjectType(t *testing.T) {
	v := newValidator()
	_, err := v.ValidateCreate(context.Background(), &metav1.Status{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expected *LlamaStackDistribution")
}

