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
	"testing"

	llamav1alpha1 "github.com/llamastack/llama-stack-k8s-operator/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestGenerateInitContainers_NoExternalProviders(t *testing.T) {
	instance := &llamav1alpha1.LlamaStackDistribution{
		Spec: llamav1alpha1.LlamaStackDistributionSpec{
			Server: llamav1alpha1.ServerSpec{},
		},
	}

	containers := GenerateInitContainers(instance, "operator:latest")
	assert.Nil(t, containers)
}

func TestGenerateInitContainers_EmptyExternalProviders(t *testing.T) {
	instance := &llamav1alpha1.LlamaStackDistribution{
		Spec: llamav1alpha1.LlamaStackDistributionSpec{
			Server: llamav1alpha1.ServerSpec{
				ExternalProviders: &llamav1alpha1.ExternalProvidersSpec{},
			},
		},
	}

	containers := GenerateInitContainers(instance, "operator:latest")
	assert.Nil(t, containers)
}

func TestGenerateInitContainers_SingleProvider(t *testing.T) {
	instance := &llamav1alpha1.LlamaStackDistribution{
		Spec: llamav1alpha1.LlamaStackDistributionSpec{
			Server: llamav1alpha1.ServerSpec{
				ExternalProviders: &llamav1alpha1.ExternalProvidersSpec{
					Inference: []llamav1alpha1.ExternalProviderRef{
						{
							ProviderID: "custom-vllm",
							Image:      "registry.example.com/vllm-provider:v1.0",
						},
					},
				},
			},
		},
	}

	containers := GenerateInitContainers(instance, "operator:latest")

	// Should have 2 containers: 1 provider install + 1 config generation
	require.Len(t, containers, 2)

	// First container is the provider install
	assert.Equal(t, "install-provider-custom-vllm", containers[0].Name)
	assert.Equal(t, "registry.example.com/vllm-provider:v1.0", containers[0].Image)

	// Second container is the config generation
	assert.Equal(t, GenerateConfigContainerName, containers[1].Name)
	assert.Equal(t, "operator:latest", containers[1].Image)
}

func TestGenerateInitContainers_MultipleProvidersSameAPI(t *testing.T) {
	instance := &llamav1alpha1.LlamaStackDistribution{
		Spec: llamav1alpha1.LlamaStackDistributionSpec{
			Server: llamav1alpha1.ServerSpec{
				ExternalProviders: &llamav1alpha1.ExternalProvidersSpec{
					Inference: []llamav1alpha1.ExternalProviderRef{
						{ProviderID: "provider-a", Image: "image-a:v1"},
						{ProviderID: "provider-b", Image: "image-b:v1"},
						{ProviderID: "provider-c", Image: "image-c:v1"},
					},
				},
			},
		},
	}

	containers := GenerateInitContainers(instance, "operator:latest")

	// Should have 4 containers: 3 provider installs + 1 config generation
	require.Len(t, containers, 4)

	// Verify order matches CRD order
	assert.Equal(t, "install-provider-provider-a", containers[0].Name)
	assert.Equal(t, "install-provider-provider-b", containers[1].Name)
	assert.Equal(t, "install-provider-provider-c", containers[2].Name)
	assert.Equal(t, GenerateConfigContainerName, containers[3].Name)
}

func TestGenerateInitContainers_MultipleProvidersDifferentAPIs(t *testing.T) {
	instance := &llamav1alpha1.LlamaStackDistribution{
		Spec: llamav1alpha1.LlamaStackDistributionSpec{
			Server: llamav1alpha1.ServerSpec{
				ExternalProviders: &llamav1alpha1.ExternalProvidersSpec{
					Inference: []llamav1alpha1.ExternalProviderRef{
						{ProviderID: "inference-provider", Image: "inference:v1"},
					},
					Safety: []llamav1alpha1.ExternalProviderRef{
						{ProviderID: "safety-provider", Image: "safety:v1"},
					},
					Agents: []llamav1alpha1.ExternalProviderRef{
						{ProviderID: "agents-provider", Image: "agents:v1"},
					},
				},
			},
		},
	}

	containers := GenerateInitContainers(instance, "operator:latest")

	// Should have 4 containers: 3 provider installs + 1 config generation
	require.Len(t, containers, 4)

	// Verify order matches CRD order (inference, safety, agents)
	assert.Equal(t, "install-provider-inference-provider", containers[0].Name)
	assert.Equal(t, "install-provider-safety-provider", containers[1].Name)
	assert.Equal(t, "install-provider-agents-provider", containers[2].Name)
}

func TestGenerateInitContainers_ImagePullPolicy(t *testing.T) {
	instance := &llamav1alpha1.LlamaStackDistribution{
		Spec: llamav1alpha1.LlamaStackDistributionSpec{
			Server: llamav1alpha1.ServerSpec{
				ExternalProviders: &llamav1alpha1.ExternalProvidersSpec{
					Inference: []llamav1alpha1.ExternalProviderRef{
						{
							ProviderID:      "always-pull",
							Image:           "image:latest",
							ImagePullPolicy: corev1.PullAlways,
						},
						{
							ProviderID: "default-pull",
							Image:      "image:v1.0",
							// No explicit policy
						},
					},
				},
			},
		},
	}

	containers := GenerateInitContainers(instance, "operator:latest")

	require.Len(t, containers, 3)
	assert.Equal(t, corev1.PullAlways, containers[0].ImagePullPolicy)
	assert.Equal(t, corev1.PullIfNotPresent, containers[1].ImagePullPolicy)
}

func TestGenerateInitContainers_VolumeMounts(t *testing.T) {
	instance := &llamav1alpha1.LlamaStackDistribution{
		Spec: llamav1alpha1.LlamaStackDistributionSpec{
			Server: llamav1alpha1.ServerSpec{
				ExternalProviders: &llamav1alpha1.ExternalProvidersSpec{
					Inference: []llamav1alpha1.ExternalProviderRef{
						{ProviderID: "test", Image: "test:v1"},
					},
				},
			},
		},
	}

	containers := GenerateInitContainers(instance, "operator:latest")

	// Provider container should have external-providers mount
	require.Len(t, containers[0].VolumeMounts, 1)
	assert.Equal(t, ExternalProvidersVolumeName, containers[0].VolumeMounts[0].Name)
	assert.Equal(t, ExternalProvidersMountPath, containers[0].VolumeMounts[0].MountPath)

	// Config generation container should have 3 mounts
	require.Len(t, containers[1].VolumeMounts, 3)
}

func TestGenerateExtractConfigInitContainer(t *testing.T) {
	container := GenerateExtractConfigInitContainer("llama-stack:v1.0")

	assert.Equal(t, ExtractConfigContainerName, container.Name)
	assert.Equal(t, "llama-stack:v1.0", container.Image)
	assert.Equal(t, corev1.PullIfNotPresent, container.ImagePullPolicy)

	// Should have base-config mount
	require.Len(t, container.VolumeMounts, 1)
	assert.Equal(t, "base-config", container.VolumeMounts[0].Name)
	assert.Equal(t, BaseConfigMountPath, container.VolumeMounts[0].MountPath)

	// Script should check both paths
	script := container.Command[2]
	assert.Contains(t, script, "/opt/app-root/config.yaml")
	assert.Contains(t, script, "/etc/llama-stack/config.yaml")
}

func TestAddExternalProvidersVolume(t *testing.T) {
	podSpec := &corev1.PodSpec{}

	// Test with nil size limit (should use default)
	AddExternalProvidersVolume(podSpec, nil)

	require.Len(t, podSpec.Volumes, 1)
	assert.Equal(t, ExternalProvidersVolumeName, podSpec.Volumes[0].Name)
	assert.NotNil(t, podSpec.Volumes[0].EmptyDir)
	assert.Equal(t, resource.MustParse(DefaultVolumeSizeLimit), *podSpec.Volumes[0].EmptyDir.SizeLimit)
}

func TestAddExternalProvidersVolume_CustomSize(t *testing.T) {
	podSpec := &corev1.PodSpec{}
	customSize := resource.MustParse("5Gi")

	AddExternalProvidersVolume(podSpec, &customSize)

	require.Len(t, podSpec.Volumes, 1)
	assert.Equal(t, resource.MustParse("5Gi"), *podSpec.Volumes[0].EmptyDir.SizeLimit)
}

func TestAddBaseConfigVolume_WithConfigMap(t *testing.T) {
	podSpec := &corev1.PodSpec{}

	AddBaseConfigVolume(podSpec, "my-config")

	require.Len(t, podSpec.Volumes, 1)
	assert.Equal(t, "base-config", podSpec.Volumes[0].Name)
	assert.NotNil(t, podSpec.Volumes[0].ConfigMap)
	assert.Equal(t, "my-config", podSpec.Volumes[0].ConfigMap.Name)
}

func TestAddBaseConfigVolume_EmptyDir(t *testing.T) {
	podSpec := &corev1.PodSpec{}

	AddBaseConfigVolume(podSpec, "")

	require.Len(t, podSpec.Volumes, 1)
	assert.Equal(t, "base-config", podSpec.Volumes[0].Name)
	assert.NotNil(t, podSpec.Volumes[0].EmptyDir)
	assert.Nil(t, podSpec.Volumes[0].ConfigMap)
}

func TestAddFinalConfigVolume(t *testing.T) {
	podSpec := &corev1.PodSpec{}

	AddFinalConfigVolume(podSpec)

	require.Len(t, podSpec.Volumes, 1)
	assert.Equal(t, "final-config", podSpec.Volumes[0].Name)
	assert.NotNil(t, podSpec.Volumes[0].EmptyDir)
}

func TestMountExternalProvidersVolume(t *testing.T) {
	container := &corev1.Container{}

	MountExternalProvidersVolume(container, true)

	require.Len(t, container.VolumeMounts, 1)
	assert.Equal(t, ExternalProvidersVolumeName, container.VolumeMounts[0].Name)
	assert.Equal(t, ExternalProvidersMountPath, container.VolumeMounts[0].MountPath)
	assert.True(t, container.VolumeMounts[0].ReadOnly)
}

func TestMountFinalConfigVolume(t *testing.T) {
	container := &corev1.Container{}

	MountFinalConfigVolume(container)

	require.Len(t, container.VolumeMounts, 1)
	assert.Equal(t, "final-config", container.VolumeMounts[0].Name)
	assert.Equal(t, FinalConfigMountPath, container.VolumeMounts[0].MountPath)
	assert.True(t, container.VolumeMounts[0].ReadOnly)
}

func TestUpdatePythonPath_NewEnv(t *testing.T) {
	container := &corev1.Container{}

	UpdatePythonPath(container)

	require.Len(t, container.Env, 1)
	assert.Equal(t, "PYTHONPATH", container.Env[0].Name)
	assert.Equal(t, ExternalProvidersPythonPackagesPath, container.Env[0].Value)
}

func TestUpdatePythonPath_ExistingEnv(t *testing.T) {
	container := &corev1.Container{
		Env: []corev1.EnvVar{
			{Name: "OTHER", Value: "value"},
			{Name: "PYTHONPATH", Value: "/existing/path"},
		},
	}

	UpdatePythonPath(container)

	require.Len(t, container.Env, 2)
	assert.Equal(t, ExternalProvidersPythonPackagesPath+":/existing/path", container.Env[1].Value)
}

func TestHasExternalProviders(t *testing.T) {
	testCases := []struct {
		name     string
		instance *llamav1alpha1.LlamaStackDistribution
		expected bool
	}{
		{
			name: "nil external providers",
			instance: &llamav1alpha1.LlamaStackDistribution{
				Spec: llamav1alpha1.LlamaStackDistributionSpec{
					Server: llamav1alpha1.ServerSpec{},
				},
			},
			expected: false,
		},
		{
			name: "empty external providers",
			instance: &llamav1alpha1.LlamaStackDistribution{
				Spec: llamav1alpha1.LlamaStackDistributionSpec{
					Server: llamav1alpha1.ServerSpec{
						ExternalProviders: &llamav1alpha1.ExternalProvidersSpec{},
					},
				},
			},
			expected: false,
		},
		{
			name: "has inference provider",
			instance: &llamav1alpha1.LlamaStackDistribution{
				Spec: llamav1alpha1.LlamaStackDistributionSpec{
					Server: llamav1alpha1.ServerSpec{
						ExternalProviders: &llamav1alpha1.ExternalProvidersSpec{
							Inference: []llamav1alpha1.ExternalProviderRef{{ProviderID: "test"}},
						},
					},
				},
			},
			expected: true,
		},
		{
			name: "has safety provider",
			instance: &llamav1alpha1.LlamaStackDistribution{
				Spec: llamav1alpha1.LlamaStackDistributionSpec{
					Server: llamav1alpha1.ServerSpec{
						ExternalProviders: &llamav1alpha1.ExternalProvidersSpec{
							Safety: []llamav1alpha1.ExternalProviderRef{{ProviderID: "test"}},
						},
					},
				},
			},
			expected: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := HasExternalProviders(tc.instance)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestGetAllExternalProviders(t *testing.T) {
	ep := &llamav1alpha1.ExternalProvidersSpec{
		Inference: []llamav1alpha1.ExternalProviderRef{
			{ProviderID: "inf-1"},
			{ProviderID: "inf-2"},
		},
		Safety: []llamav1alpha1.ExternalProviderRef{
			{ProviderID: "safe-1"},
		},
	}

	providers := GetAllExternalProviders(ep)

	require.Len(t, providers, 3)
	assert.Equal(t, "inf-1", providers[0].ProviderID)
	assert.Equal(t, "inf-2", providers[1].ProviderID)
	assert.Equal(t, "safe-1", providers[2].ProviderID)
}

func TestGetAllExternalProviders_Nil(t *testing.T) {
	providers := GetAllExternalProviders(nil)
	assert.Nil(t, providers)
}

func TestCollectProvidersInCRDOrder(t *testing.T) {
	ep := &llamav1alpha1.ExternalProvidersSpec{
		// Add in non-alphabetical order to verify CRD order is preserved
		PostTraining: []llamav1alpha1.ExternalProviderRef{{ProviderID: "post"}},
		Inference:    []llamav1alpha1.ExternalProviderRef{{ProviderID: "inf"}},
		Safety:       []llamav1alpha1.ExternalProviderRef{{ProviderID: "safe"}},
	}

	providers := collectProvidersInCRDOrder(ep)

	require.Len(t, providers, 3)
	// Should be in struct field order, not alphabetical
	assert.Equal(t, "inf", providers[0].ref.ProviderID)
	assert.Equal(t, "inference", providers[0].api)
	assert.Equal(t, "safe", providers[1].ref.ProviderID)
	assert.Equal(t, "safety", providers[1].api)
	assert.Equal(t, "post", providers[2].ref.ProviderID)
	assert.Equal(t, "postTraining", providers[2].api)
}

func TestGenerateProviderInitContainer_ScriptContent(t *testing.T) {
	instance := &llamav1alpha1.LlamaStackDistribution{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-llsd",
			Namespace: "test-ns",
		},
		Spec: llamav1alpha1.LlamaStackDistributionSpec{
			Server: llamav1alpha1.ServerSpec{
				ExternalProviders: &llamav1alpha1.ExternalProvidersSpec{
					Inference: []llamav1alpha1.ExternalProviderRef{
						{
							ProviderID: "my-provider",
							Image:      "my-image:v1",
						},
					},
				},
			},
		},
	}

	containers := GenerateInitContainers(instance, "operator:latest")
	require.Len(t, containers, 2)

	script := containers[0].Command[2]

	// Verify script contains essential elements
	assert.Contains(t, script, "my-provider")
	assert.Contains(t, script, "my-image:v1")
	assert.Contains(t, script, "/lls-provider/lls-provider-spec.yaml")
	assert.Contains(t, script, "/lls-provider/packages")
	assert.Contains(t, script, "pip install")
	assert.Contains(t, script, ExternalProvidersPythonPackagesPath)
}
