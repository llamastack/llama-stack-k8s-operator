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

package v1alpha1

import (
	"encoding/json"
	"testing"

	v1alpha2 "github.com/llamastack/llama-stack-k8s-operator/api/v1alpha2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestConvertToV1Alpha2Basic(t *testing.T) {
	src := &LlamaStackDistribution{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-llsd",
			Namespace: "default",
		},
		Spec: LlamaStackDistributionSpec{
			Replicas: 3,
			Server: ServerSpec{
				Distribution: DistributionType{
					Name: "starter",
				},
				ContainerSpec: ContainerSpec{
					Port: 9000,
					Env: []corev1.EnvVar{
						{Name: "MY_VAR", Value: "my-value"},
					},
				},
				Workers: int32Ptr(4),
			},
		},
		Status: LlamaStackDistributionStatus{
			Phase:             LlamaStackDistributionPhaseReady,
			AvailableReplicas: 3,
			ServiceURL:        "http://test-llsd.default.svc:8321",
		},
	}

	dst := &v1alpha2.LlamaStackDistribution{}
	err := src.ConvertTo(dst)
	require.NoError(t, err)

	assert.Equal(t, "test-llsd", dst.Name)
	assert.Equal(t, "starter", dst.Spec.Distribution.Name)

	require.NotNil(t, dst.Spec.Workload)
	require.NotNil(t, dst.Spec.Workload.Replicas)
	assert.Equal(t, int32(3), *dst.Spec.Workload.Replicas)
	require.NotNil(t, dst.Spec.Workload.Workers)
	assert.Equal(t, int32(4), *dst.Spec.Workload.Workers)

	require.NotNil(t, dst.Spec.Networking)
	assert.Equal(t, int32(9000), dst.Spec.Networking.Port)

	require.NotNil(t, dst.Spec.Workload.Overrides)
	assert.Len(t, dst.Spec.Workload.Overrides.Env, 1)
	assert.Equal(t, "MY_VAR", dst.Spec.Workload.Overrides.Env[0].Name)

	assert.Equal(t, v1alpha2.PhaseReady, dst.Status.Phase)
	assert.Equal(t, int32(3), dst.Status.AvailableReplicas)
}

func TestConvertToV1Alpha2WithImage(t *testing.T) {
	src := &LlamaStackDistribution{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
		Spec: LlamaStackDistributionSpec{
			Server: ServerSpec{
				Distribution: DistributionType{
					Image: "quay.io/llamastack/distribution-starter:latest",
				},
			},
		},
	}

	dst := &v1alpha2.LlamaStackDistribution{}
	err := src.ConvertTo(dst)
	require.NoError(t, err)

	assert.Equal(t, "quay.io/llamastack/distribution-starter:latest", dst.Spec.Distribution.Image)
	assert.Empty(t, dst.Spec.Distribution.Name)
}

func TestConvertToV1Alpha2WithUserConfig(t *testing.T) {
	src := &LlamaStackDistribution{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
		Spec: LlamaStackDistributionSpec{
			Server: ServerSpec{
				Distribution: DistributionType{Name: "starter"},
				UserConfig: &UserConfigSpec{
					ConfigMapName:      "my-config",
					ConfigMapNamespace: "other-ns",
				},
			},
		},
	}

	dst := &v1alpha2.LlamaStackDistribution{}
	err := src.ConvertTo(dst)
	require.NoError(t, err)

	require.NotNil(t, dst.Spec.OverrideConfig)
	assert.Equal(t, "my-config", dst.Spec.OverrideConfig.ConfigMapName)

	// Verify v1alpha1 extras contain the namespace
	raw, ok := dst.Annotations[annV1Alpha1Extras]
	require.True(t, ok, "should have v1alpha1 extras annotation")
	var extras v1alpha1Extras
	require.NoError(t, json.Unmarshal([]byte(raw), &extras))
	assert.Equal(t, "other-ns", extras.UserConfigNamespace)

	// Verify legacy annotations are cleaned up
	assert.NotContains(t, dst.Annotations, legacyAnnUserConfigNamespace)
}

func TestConvertToV1Alpha2WithTLS(t *testing.T) {
	src := &LlamaStackDistribution{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
		Spec: LlamaStackDistributionSpec{
			Server: ServerSpec{
				Distribution: DistributionType{Name: "starter"},
				TLSConfig: &TLSConfig{
					CABundle: &CABundleConfig{
						ConfigMapName:      "ca-bundle",
						ConfigMapNamespace: "certs-ns",
						ConfigMapKeys:      []string{"ca.crt", "intermediate.crt"},
					},
				},
			},
		},
	}

	dst := &v1alpha2.LlamaStackDistribution{}
	err := src.ConvertTo(dst)
	require.NoError(t, err)

	require.NotNil(t, dst.Spec.Networking)
	require.NotNil(t, dst.Spec.Networking.TLS)
	require.NotNil(t, dst.Spec.Networking.TLS.CABundle)
	assert.Equal(t, "ca-bundle", dst.Spec.Networking.TLS.CABundle.ConfigMapName)

	// Verify v1alpha1 extras contain TLS data
	raw, ok := dst.Annotations[annV1Alpha1Extras]
	require.True(t, ok)
	var extras v1alpha1Extras
	require.NoError(t, json.Unmarshal([]byte(raw), &extras))
	assert.Equal(t, "certs-ns", extras.CABundleNamespace)
	assert.Equal(t, []string{"ca.crt", "intermediate.crt"}, extras.CABundleKeys)

	// Verify legacy annotations are cleaned up
	assert.NotContains(t, dst.Annotations, legacyAnnCABundleNamespace)
	assert.NotContains(t, dst.Annotations, legacyAnnCABundleKeys)
}

func TestConvertToV1Alpha2WithNetwork(t *testing.T) {
	src := &LlamaStackDistribution{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
		Spec: LlamaStackDistributionSpec{
			Server: ServerSpec{
				Distribution: DistributionType{Name: "starter"},
			},
			Network: &NetworkSpec{
				ExposeRoute: true,
				AllowedFrom: &AllowedFromSpec{
					Namespaces: []string{"ns1", "ns2"},
					Labels:     []string{"team/allowed"},
				},
			},
		},
	}

	dst := &v1alpha2.LlamaStackDistribution{}
	err := src.ConvertTo(dst)
	require.NoError(t, err)

	require.NotNil(t, dst.Spec.Networking)
	require.NotNil(t, dst.Spec.Networking.Expose)

	require.NotNil(t, dst.Spec.Networking.AllowedFrom)
	assert.Equal(t, []string{"ns1", "ns2"}, dst.Spec.Networking.AllowedFrom.Namespaces)
}

func TestConvertToV1Alpha2WithPodOverrides(t *testing.T) {
	gracePeriod := int64(60)
	src := &LlamaStackDistribution{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
		Spec: LlamaStackDistributionSpec{
			Server: ServerSpec{
				Distribution: DistributionType{Name: "starter"},
				PodOverrides: &PodOverrides{
					ServiceAccountName:            "custom-sa",
					TerminationGracePeriodSeconds: &gracePeriod,
				},
			},
		},
	}

	dst := &v1alpha2.LlamaStackDistribution{}
	err := src.ConvertTo(dst)
	require.NoError(t, err)

	require.NotNil(t, dst.Spec.Workload)
	require.NotNil(t, dst.Spec.Workload.Overrides)
	assert.Equal(t, "custom-sa", dst.Spec.Workload.Overrides.ServiceAccountName)

	// Verify v1alpha1 extras contain termination grace period
	raw, ok := dst.Annotations[annV1Alpha1Extras]
	require.True(t, ok)
	var extras v1alpha1Extras
	require.NoError(t, json.Unmarshal([]byte(raw), &extras))
	require.NotNil(t, extras.TerminationGracePeriod)
	assert.Equal(t, int64(60), *extras.TerminationGracePeriod)

	// Verify legacy annotations are cleaned up
	assert.NotContains(t, dst.Annotations, legacyAnnTerminationGracePeriod)
}

func TestConvertToV1Alpha2WithStorage(t *testing.T) {
	size := resource.MustParse("20Gi")
	src := &LlamaStackDistribution{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
		Spec: LlamaStackDistributionSpec{
			Server: ServerSpec{
				Distribution: DistributionType{Name: "starter"},
				Storage: &StorageSpec{
					Size:      &size,
					MountPath: "/data",
				},
			},
		},
	}

	dst := &v1alpha2.LlamaStackDistribution{}
	err := src.ConvertTo(dst)
	require.NoError(t, err)

	require.NotNil(t, dst.Spec.Workload)
	require.NotNil(t, dst.Spec.Workload.Storage)
	assert.Equal(t, "/data", dst.Spec.Workload.Storage.MountPath)
}

func TestConvertFromV1Alpha2Basic(t *testing.T) {
	replicas := int32(3)
	workers := int32(4)
	src := &v1alpha2.LlamaStackDistribution{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-llsd",
			Namespace: "default",
		},
		Spec: v1alpha2.LlamaStackDistributionSpec{
			Distribution: v1alpha2.DistributionSpec{Name: "starter"},
			Workload: &v1alpha2.WorkloadSpec{
				Replicas: &replicas,
				Workers:  &workers,
				Overrides: &v1alpha2.WorkloadOverrides{
					Env: []corev1.EnvVar{
						{Name: "MY_VAR", Value: "val"},
					},
				},
			},
			Networking: &v1alpha2.NetworkingSpec{
				Port: 9000,
			},
		},
		Status: v1alpha2.LlamaStackDistributionStatus{
			Phase:             v1alpha2.PhaseReady,
			AvailableReplicas: 3,
		},
	}

	dst := &LlamaStackDistribution{}
	err := dst.ConvertFrom(src)
	require.NoError(t, err)

	assert.Equal(t, "starter", dst.Spec.Server.Distribution.Name)
	assert.Equal(t, int32(3), dst.Spec.Replicas)
	assert.Equal(t, int32(4), *dst.Spec.Server.Workers)
	assert.Equal(t, int32(9000), dst.Spec.Server.ContainerSpec.Port)
	assert.Len(t, dst.Spec.Server.ContainerSpec.Env, 1)
	assert.Equal(t, LlamaStackDistributionPhaseReady, dst.Status.Phase)
}

func TestConvertFromV1Alpha2NilNetworking(t *testing.T) {
	src := &v1alpha2.LlamaStackDistribution{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
		Spec: v1alpha2.LlamaStackDistributionSpec{
			Distribution: v1alpha2.DistributionSpec{Name: "starter"},
		},
	}

	dst := &LlamaStackDistribution{}
	err := dst.ConvertFrom(src)
	require.NoError(t, err)

	assert.Equal(t, DefaultServerPort, dst.Spec.Server.ContainerSpec.Port,
		"nil networking should default port to DefaultServerPort so Service is created")
}

func TestConvertFromV1Alpha2ZeroPort(t *testing.T) {
	src := &v1alpha2.LlamaStackDistribution{
		ObjectMeta: metav1.ObjectMeta{Name: "test"},
		Spec: v1alpha2.LlamaStackDistributionSpec{
			Distribution: v1alpha2.DistributionSpec{Name: "starter"},
			Networking:   &v1alpha2.NetworkingSpec{Port: 0},
		},
	}

	dst := &LlamaStackDistribution{}
	err := dst.ConvertFrom(src)
	require.NoError(t, err)

	assert.Equal(t, DefaultServerPort, dst.Spec.Server.ContainerSpec.Port,
		"zero port should default to DefaultServerPort so Service is created")
}

func TestConvertFromV1Alpha2WithOverrideConfig(t *testing.T) {
	src := &v1alpha2.LlamaStackDistribution{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
			Annotations: map[string]string{
				annV1Alpha1Extras: `{"userConfigNamespace":"other-ns"}`,
			},
		},
		Spec: v1alpha2.LlamaStackDistributionSpec{
			Distribution: v1alpha2.DistributionSpec{Name: "starter"},
			OverrideConfig: &v1alpha2.OverrideConfigSpec{
				ConfigMapName: "my-config",
			},
		},
	}

	dst := &LlamaStackDistribution{}
	err := dst.ConvertFrom(src)
	require.NoError(t, err)

	require.NotNil(t, dst.Spec.Server.UserConfig)
	assert.Equal(t, "my-config", dst.Spec.Server.UserConfig.ConfigMapName)
	assert.Equal(t, "other-ns", dst.Spec.Server.UserConfig.ConfigMapNamespace)
}

func TestConvertFromV1Alpha2WithLegacyAnnotations(t *testing.T) {
	src := &v1alpha2.LlamaStackDistribution{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
			Annotations: map[string]string{
				legacyAnnUserConfigNamespace:    "legacy-ns",
				legacyAnnContainerName:          "custom-container",
				legacyAnnTerminationGracePeriod: "30",
			},
		},
		Spec: v1alpha2.LlamaStackDistributionSpec{
			Distribution: v1alpha2.DistributionSpec{Name: "starter"},
			OverrideConfig: &v1alpha2.OverrideConfigSpec{
				ConfigMapName: "my-config",
			},
			Workload: &v1alpha2.WorkloadSpec{
				Overrides: &v1alpha2.WorkloadOverrides{
					ServiceAccountName: "sa",
				},
			},
		},
	}

	dst := &LlamaStackDistribution{}
	err := dst.ConvertFrom(src)
	require.NoError(t, err)

	assert.Equal(t, "legacy-ns", dst.Spec.Server.UserConfig.ConfigMapNamespace)
	assert.Equal(t, "custom-container", dst.Spec.Server.ContainerSpec.Name)
	require.NotNil(t, dst.Spec.Server.PodOverrides)
	require.NotNil(t, dst.Spec.Server.PodOverrides.TerminationGracePeriodSeconds)
	assert.Equal(t, int64(30), *dst.Spec.Server.PodOverrides.TerminationGracePeriodSeconds)

	// Legacy annotations should be cleaned up
	assert.NotContains(t, dst.Annotations, legacyAnnUserConfigNamespace)
	assert.NotContains(t, dst.Annotations, legacyAnnContainerName)
	assert.NotContains(t, dst.Annotations, legacyAnnTerminationGracePeriod)
}

// --- v1alpha1 round-trip ---

func TestRoundTripV1Alpha1(t *testing.T) {
	gracePeriod := int64(60)
	size := resource.MustParse("20Gi")

	original := &LlamaStackDistribution{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "roundtrip-test",
			Namespace: "default",
		},
		Spec: LlamaStackDistributionSpec{
			Replicas: 2,
			Server: ServerSpec{
				Distribution: DistributionType{Name: "starter"},
				ContainerSpec: ContainerSpec{
					Port: 9000,
					Env:  []corev1.EnvVar{{Name: "KEY", Value: "val"}},
				},
				Workers: int32Ptr(3),
				PodOverrides: &PodOverrides{
					ServiceAccountName:            "my-sa",
					TerminationGracePeriodSeconds: &gracePeriod,
				},
				Storage: &StorageSpec{
					Size:      &size,
					MountPath: "/data",
				},
				PodDisruptionBudget: &PodDisruptionBudgetSpec{},
				Autoscaling: &AutoscalingSpec{
					MaxReplicas: 5,
				},
				UserConfig: &UserConfigSpec{
					ConfigMapName:      "my-config",
					ConfigMapNamespace: "other-ns",
				},
				TLSConfig: &TLSConfig{
					CABundle: &CABundleConfig{
						ConfigMapName:      "ca-bundle",
						ConfigMapNamespace: "certs",
						ConfigMapKeys:      []string{"ca.crt"},
					},
				},
			},
			Network: &NetworkSpec{
				ExposeRoute: true,
				AllowedFrom: &AllowedFromSpec{
					Namespaces: []string{"ns1"},
					Labels:     []string{"team/ok"},
				},
			},
		},
		Status: LlamaStackDistributionStatus{
			Phase:             LlamaStackDistributionPhaseReady,
			AvailableReplicas: 2,
			ServiceURL:        "http://test.svc:8321",
		},
	}

	// v1alpha1 → v1alpha2
	hub := &v1alpha2.LlamaStackDistribution{}
	require.NoError(t, original.ConvertTo(hub))

	// v1alpha2 → v1alpha1
	roundtripped := &LlamaStackDistribution{}
	require.NoError(t, roundtripped.ConvertFrom(hub))

	assert.Equal(t, original.Spec.Replicas, roundtripped.Spec.Replicas)
	assert.Equal(t, original.Spec.Server.Distribution.Name, roundtripped.Spec.Server.Distribution.Name)
	assert.Equal(t, original.Spec.Server.ContainerSpec.Port, roundtripped.Spec.Server.ContainerSpec.Port)
	assert.Equal(t, *original.Spec.Server.Workers, *roundtripped.Spec.Server.Workers)

	require.NotNil(t, roundtripped.Spec.Server.PodOverrides)
	assert.Equal(t, "my-sa", roundtripped.Spec.Server.PodOverrides.ServiceAccountName)
	require.NotNil(t, roundtripped.Spec.Server.PodOverrides.TerminationGracePeriodSeconds)
	assert.Equal(t, int64(60), *roundtripped.Spec.Server.PodOverrides.TerminationGracePeriodSeconds)

	require.NotNil(t, roundtripped.Spec.Server.UserConfig)
	assert.Equal(t, "my-config", roundtripped.Spec.Server.UserConfig.ConfigMapName)
	assert.Equal(t, "other-ns", roundtripped.Spec.Server.UserConfig.ConfigMapNamespace)

	require.NotNil(t, roundtripped.Spec.Server.TLSConfig)
	require.NotNil(t, roundtripped.Spec.Server.TLSConfig.CABundle)
	assert.Equal(t, "ca-bundle", roundtripped.Spec.Server.TLSConfig.CABundle.ConfigMapName)
	assert.Equal(t, "certs", roundtripped.Spec.Server.TLSConfig.CABundle.ConfigMapNamespace)
	assert.Equal(t, []string{"ca.crt"}, roundtripped.Spec.Server.TLSConfig.CABundle.ConfigMapKeys)

	require.NotNil(t, roundtripped.Spec.Network)
	assert.True(t, roundtripped.Spec.Network.ExposeRoute)
	assert.Equal(t, []string{"ns1"}, roundtripped.Spec.Network.AllowedFrom.Namespaces)

	assert.Equal(t, original.Status.Phase, roundtripped.Status.Phase)
	assert.Equal(t, original.Status.AvailableReplicas, roundtripped.Status.AvailableReplicas)
}

// --- v1alpha2 round-trip tests ---

func TestRoundTripV1Alpha2Providers(t *testing.T) {
	settings := apiextensionsv1.JSON{Raw: []byte(`{"temperature":0.7}`)}
	original := &v1alpha2.LlamaStackDistribution{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
		Spec: v1alpha2.LlamaStackDistributionSpec{
			Distribution: v1alpha2.DistributionSpec{Name: "starter"},
			Providers: &v1alpha2.ProvidersSpec{
				Inference: []v1alpha2.ProviderConfig{
					{ID: "vllm-1", Provider: "vllm", Endpoint: "http://vllm:8000", Settings: &settings},
				},
				Telemetry: []v1alpha2.ProviderConfig{
					{Provider: "otel"},
				},
			},
		},
	}

	// v1alpha2 → v1alpha1
	v1 := &LlamaStackDistribution{}
	require.NoError(t, v1.ConvertFrom(original))

	// v1alpha1 → v1alpha2
	roundtripped := &v1alpha2.LlamaStackDistribution{}
	require.NoError(t, v1.ConvertTo(roundtripped))

	require.NotNil(t, roundtripped.Spec.Providers)
	require.Len(t, roundtripped.Spec.Providers.Inference, 1)
	assert.Equal(t, "vllm-1", roundtripped.Spec.Providers.Inference[0].ID)
	assert.Equal(t, "http://vllm:8000", roundtripped.Spec.Providers.Inference[0].Endpoint)
	require.NotNil(t, roundtripped.Spec.Providers.Inference[0].Settings)
	assert.JSONEq(t, `{"temperature":0.7}`, string(roundtripped.Spec.Providers.Inference[0].Settings.Raw))
	require.Len(t, roundtripped.Spec.Providers.Telemetry, 1)
	assert.Equal(t, "otel", roundtripped.Spec.Providers.Telemetry[0].Provider)
}

func TestRoundTripV1Alpha2Resources(t *testing.T) {
	ctxLen := 8192
	original := &v1alpha2.LlamaStackDistribution{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
		Spec: v1alpha2.LlamaStackDistributionSpec{
			Distribution: v1alpha2.DistributionSpec{Name: "starter"},
			Resources: &v1alpha2.ResourcesSpec{
				Models: []v1alpha2.ModelConfig{
					{Name: "llama3", Provider: "vllm", ContextLength: &ctxLen, ModelType: "llm"},
				},
				Tools: []string{"code_interpreter"},
			},
		},
	}

	v1 := &LlamaStackDistribution{}
	require.NoError(t, v1.ConvertFrom(original))

	roundtripped := &v1alpha2.LlamaStackDistribution{}
	require.NoError(t, v1.ConvertTo(roundtripped))

	require.NotNil(t, roundtripped.Spec.Resources)
	require.Len(t, roundtripped.Spec.Resources.Models, 1)
	assert.Equal(t, "llama3", roundtripped.Spec.Resources.Models[0].Name)
	assert.Equal(t, "vllm", roundtripped.Spec.Resources.Models[0].Provider)
	require.NotNil(t, roundtripped.Spec.Resources.Models[0].ContextLength)
	assert.Equal(t, 8192, *roundtripped.Spec.Resources.Models[0].ContextLength)
	assert.Equal(t, "llm", roundtripped.Spec.Resources.Models[0].ModelType)
	assert.Equal(t, []string{"code_interpreter"}, roundtripped.Spec.Resources.Tools)
}

func TestRoundTripV1Alpha2Storage(t *testing.T) {
	original := &v1alpha2.LlamaStackDistribution{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
		Spec: v1alpha2.LlamaStackDistributionSpec{
			Distribution: v1alpha2.DistributionSpec{Name: "starter"},
			Storage: &v1alpha2.StateStorageSpec{
				KV: &v1alpha2.KVStorageSpec{
					Type:     "redis",
					Endpoint: "redis://localhost:6379",
					Password: &v1alpha2.SecretKeyRef{Name: "redis-secret", Key: "password"},
				},
				SQL: &v1alpha2.SQLStorageSpec{
					Type:             "postgres",
					ConnectionString: &v1alpha2.SecretKeyRef{Name: "pg-secret", Key: "dsn"},
				},
			},
		},
	}

	v1 := &LlamaStackDistribution{}
	require.NoError(t, v1.ConvertFrom(original))

	roundtripped := &v1alpha2.LlamaStackDistribution{}
	require.NoError(t, v1.ConvertTo(roundtripped))

	require.NotNil(t, roundtripped.Spec.Storage)
	require.NotNil(t, roundtripped.Spec.Storage.KV)
	assert.Equal(t, "redis", roundtripped.Spec.Storage.KV.Type)
	assert.Equal(t, "redis://localhost:6379", roundtripped.Spec.Storage.KV.Endpoint)
	require.NotNil(t, roundtripped.Spec.Storage.KV.Password)
	assert.Equal(t, "redis-secret", roundtripped.Spec.Storage.KV.Password.Name)

	require.NotNil(t, roundtripped.Spec.Storage.SQL)
	assert.Equal(t, "postgres", roundtripped.Spec.Storage.SQL.Type)
	require.NotNil(t, roundtripped.Spec.Storage.SQL.ConnectionString)
	assert.Equal(t, "pg-secret", roundtripped.Spec.Storage.SQL.ConnectionString.Name)
}

func TestRoundTripV1Alpha2Disabled(t *testing.T) {
	original := &v1alpha2.LlamaStackDistribution{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
		Spec: v1alpha2.LlamaStackDistributionSpec{
			Distribution: v1alpha2.DistributionSpec{Name: "starter"},
			Disabled:     []string{"agents", "telemetry"},
		},
	}

	v1 := &LlamaStackDistribution{}
	require.NoError(t, v1.ConvertFrom(original))

	roundtripped := &v1alpha2.LlamaStackDistribution{}
	require.NoError(t, v1.ConvertTo(roundtripped))

	assert.Equal(t, []string{"agents", "telemetry"}, roundtripped.Spec.Disabled)
}

func TestRoundTripV1Alpha2ExternalProviders(t *testing.T) {
	original := &v1alpha2.LlamaStackDistribution{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
		Spec: v1alpha2.LlamaStackDistributionSpec{
			Distribution: v1alpha2.DistributionSpec{Name: "starter"},
			ExternalProviders: &v1alpha2.ExternalProvidersSpec{
				Inference: []v1alpha2.ExternalProviderRef{
					{ProviderID: "custom-llm", Image: "quay.io/my/provider:latest"},
				},
			},
		},
	}

	v1 := &LlamaStackDistribution{}
	require.NoError(t, v1.ConvertFrom(original))

	roundtripped := &v1alpha2.LlamaStackDistribution{}
	require.NoError(t, v1.ConvertTo(roundtripped))

	require.NotNil(t, roundtripped.Spec.ExternalProviders)
	require.Len(t, roundtripped.Spec.ExternalProviders.Inference, 1)
	assert.Equal(t, "custom-llm", roundtripped.Spec.ExternalProviders.Inference[0].ProviderID)
	assert.Equal(t, "quay.io/my/provider:latest", roundtripped.Spec.ExternalProviders.Inference[0].Image)
}

func TestRoundTripV1Alpha2ExposeHostname(t *testing.T) {
	original := &v1alpha2.LlamaStackDistribution{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
		Spec: v1alpha2.LlamaStackDistributionSpec{
			Distribution: v1alpha2.DistributionSpec{Name: "starter"},
			Networking: &v1alpha2.NetworkingSpec{
				Port: 9000,
				Expose: &v1alpha2.ExposeConfig{
					Hostname: "llama.example.com",
				},
			},
		},
	}

	v1 := &LlamaStackDistribution{}
	require.NoError(t, v1.ConvertFrom(original))

	// Verify the v1alpha1 exposeRoute is set
	require.NotNil(t, v1.Spec.Network)
	assert.True(t, v1.Spec.Network.ExposeRoute)

	roundtripped := &v1alpha2.LlamaStackDistribution{}
	require.NoError(t, v1.ConvertTo(roundtripped))

	require.NotNil(t, roundtripped.Spec.Networking)
	require.NotNil(t, roundtripped.Spec.Networking.Expose)
	assert.Equal(t, "llama.example.com", roundtripped.Spec.Networking.Expose.Hostname)
	assert.Equal(t, int32(9000), roundtripped.Spec.Networking.Port)
}

func TestRoundTripV1Alpha2SecretRefs(t *testing.T) {
	original := &v1alpha2.LlamaStackDistribution{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
		Spec: v1alpha2.LlamaStackDistributionSpec{
			Distribution: v1alpha2.DistributionSpec{Name: "starter"},
			Providers: &v1alpha2.ProvidersSpec{
				Inference: []v1alpha2.ProviderConfig{
					{
						Provider: "vllm",
						SecretRefs: map[string]v1alpha2.SecretKeyRef{
							"api_key": {Name: "api-secret", Key: "key"},
						},
					},
				},
			},
		},
	}

	v1 := &LlamaStackDistribution{}
	require.NoError(t, v1.ConvertFrom(original))

	roundtripped := &v1alpha2.LlamaStackDistribution{}
	require.NoError(t, v1.ConvertTo(roundtripped))

	require.NotNil(t, roundtripped.Spec.Providers)
	require.Len(t, roundtripped.Spec.Providers.Inference, 1)
	ref, ok := roundtripped.Spec.Providers.Inference[0].SecretRefs["api_key"]
	require.True(t, ok)
	assert.Equal(t, "api-secret", ref.Name)
	assert.Equal(t, "key", ref.Key)
}

func TestRoundTripV1Alpha2FullCR(t *testing.T) {
	ctxLen := 4096
	original := &v1alpha2.LlamaStackDistribution{
		ObjectMeta: metav1.ObjectMeta{Name: "full-test", Namespace: "default"},
		Spec: v1alpha2.LlamaStackDistributionSpec{
			Distribution: v1alpha2.DistributionSpec{Name: "starter"},
			Providers: &v1alpha2.ProvidersSpec{
				Inference: []v1alpha2.ProviderConfig{
					{ID: "vllm", Provider: "vllm", Endpoint: "http://vllm:8000"},
				},
			},
			Resources: &v1alpha2.ResourcesSpec{
				Models: []v1alpha2.ModelConfig{
					{Name: "llama3", Provider: "vllm", ContextLength: &ctxLen},
				},
			},
			Storage: &v1alpha2.StateStorageSpec{
				SQL: &v1alpha2.SQLStorageSpec{Type: "sqlite"},
			},
			Disabled: []string{"agents"},
			Networking: &v1alpha2.NetworkingSpec{
				Port:   9000,
				Expose: &v1alpha2.ExposeConfig{Hostname: "llama.example.com"},
				TLS: &v1alpha2.TLSSpec{
					CABundle: &v1alpha2.CABundleConfig{ConfigMapName: "ca"},
				},
			},
			Workload: &v1alpha2.WorkloadSpec{
				Replicas: int32Ptr(3),
			},
		},
	}

	// v1alpha2 → v1alpha1
	v1 := &LlamaStackDistribution{}
	require.NoError(t, v1.ConvertFrom(original))

	// v1alpha1 → v1alpha2
	roundtripped := &v1alpha2.LlamaStackDistribution{}
	require.NoError(t, v1.ConvertTo(roundtripped))

	// Common fields
	assert.Equal(t, "starter", roundtripped.Spec.Distribution.Name)
	require.NotNil(t, roundtripped.Spec.Workload)
	assert.Equal(t, int32(3), *roundtripped.Spec.Workload.Replicas)
	assert.Equal(t, int32(9000), roundtripped.Spec.Networking.Port)

	// v1alpha2-only fields (restored from extras)
	require.NotNil(t, roundtripped.Spec.Providers)
	assert.Len(t, roundtripped.Spec.Providers.Inference, 1)
	assert.Equal(t, "vllm", roundtripped.Spec.Providers.Inference[0].ID)

	require.NotNil(t, roundtripped.Spec.Resources)
	assert.Len(t, roundtripped.Spec.Resources.Models, 1)
	assert.Equal(t, "llama3", roundtripped.Spec.Resources.Models[0].Name)

	require.NotNil(t, roundtripped.Spec.Storage)
	assert.Equal(t, "sqlite", roundtripped.Spec.Storage.SQL.Type)

	assert.Equal(t, []string{"agents"}, roundtripped.Spec.Disabled)

	require.NotNil(t, roundtripped.Spec.Networking.Expose)
	assert.Equal(t, "llama.example.com", roundtripped.Spec.Networking.Expose.Hostname)

	require.NotNil(t, roundtripped.Spec.Networking.TLS)
	require.NotNil(t, roundtripped.Spec.Networking.TLS.CABundle)
	assert.Equal(t, "ca", roundtripped.Spec.Networking.TLS.CABundle.ConfigMapName)

	// Extras annotations should be cleaned up on the final v1alpha2 object
	assert.NotContains(t, roundtripped.Annotations, annV1Alpha2Extras)
}

func int32Ptr(v int32) *int32 {
	return &v
}
