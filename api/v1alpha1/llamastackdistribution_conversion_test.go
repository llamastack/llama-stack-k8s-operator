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

	// Workload
	require.NotNil(t, dst.Spec.Workload)
	require.NotNil(t, dst.Spec.Workload.Replicas)
	assert.Equal(t, int32(3), *dst.Spec.Workload.Replicas)
	require.NotNil(t, dst.Spec.Workload.Workers)
	assert.Equal(t, int32(4), *dst.Spec.Workload.Workers)

	// Networking
	require.NotNil(t, dst.Spec.Networking)
	assert.Equal(t, int32(9000), dst.Spec.Networking.Port)

	// Overrides
	require.NotNil(t, dst.Spec.Workload.Overrides)
	assert.Len(t, dst.Spec.Workload.Overrides.Env, 1)
	assert.Equal(t, "MY_VAR", dst.Spec.Workload.Overrides.Env[0].Name)

	// Status
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
	assert.Equal(t, "other-ns", dst.Annotations[annUserConfigNamespace])
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
	assert.Equal(t, "certs-ns", dst.Annotations[annCABundleNamespace])
	assert.Contains(t, dst.Annotations[annCABundleKeys], "ca.crt")
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

	var exposed bool
	require.NoError(t, json.Unmarshal(dst.Spec.Networking.Expose.Raw, &exposed))
	assert.True(t, exposed)

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
	assert.Equal(t, "60", dst.Annotations[annTerminationGracePeriod])
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

func TestConvertFromV1Alpha2WithOverrideConfig(t *testing.T) {
	src := &v1alpha2.LlamaStackDistribution{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test",
			Annotations: map[string]string{
				annUserConfigNamespace: "other-ns",
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

func TestRoundTrip(t *testing.T) {
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

	// Verify key fields survived the round trip
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

func int32Ptr(v int32) *int32 {
	return &v
}
