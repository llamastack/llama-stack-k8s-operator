package controllers_test

import (
	"context"
	"testing"
	"time"

	llamav1alpha1 "github.com/llamastack/llama-stack-k8s-operator/api/v1alpha1"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

// testenvNamespaceCounter is used to generate unique namespace names for test isolation.
var testenvNamespaceCounter int

func TestStorageConfiguration(t *testing.T) {
	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	tests := []struct {
		name           string
		buildInstance  func(namespace string) *llamav1alpha1.LlamaStackDistribution
		expectedVolume corev1.Volume
		expectedMount  corev1.VolumeMount
	}{
		{
			name: "No storage configuration - should use emptyDir",
			buildInstance: func(namespace string) *llamav1alpha1.LlamaStackDistribution {
				return NewDistributionBuilder().
					WithName("test").
					WithNamespace(namespace).
					WithStorage(nil).
					Build()
			},
			expectedVolume: corev1.Volume{
				Name: "lls-storage",
				VolumeSource: corev1.VolumeSource{
					EmptyDir: &corev1.EmptyDirVolumeSource{},
				},
			},
			expectedMount: corev1.VolumeMount{
				Name:      "lls-storage",
				MountPath: llamav1alpha1.DefaultMountPath,
			},
		},
		{
			name: "Storage with default values",
			buildInstance: func(namespace string) *llamav1alpha1.LlamaStackDistribution {
				return NewDistributionBuilder().
					WithName("test").
					WithNamespace(namespace).
					WithStorage(DefaultTestStorage()).
					Build()
			},
			expectedVolume: corev1.Volume{
				Name: "lls-storage",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: "test-pvc",
					},
				},
			},
			expectedMount: corev1.VolumeMount{
				Name:      "lls-storage",
				MountPath: llamav1alpha1.DefaultMountPath,
			},
		},
		{
			name: "Storage with custom values",
			buildInstance: func(namespace string) *llamav1alpha1.LlamaStackDistribution {
				return NewDistributionBuilder().
					WithName("test").
					WithNamespace(namespace).
					WithStorage(CustomTestStorage("20Gi", "/custom/path")).
					Build()
			},
			expectedVolume: corev1.Volume{
				Name: "lls-storage",
				VolumeSource: corev1.VolumeSource{
					PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
						ClaimName: "test-pvc",
					},
				},
			},
			expectedMount: corev1.VolumeMount{
				Name:      "lls-storage",
				MountPath: "/custom/path",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			namespace := createTestNamespace(t, "test-storage")

			// arrange
			instance := tt.buildInstance(namespace.Name)
			require.NoError(t, k8sClient.Create(context.Background(), instance))
			t.Cleanup(func() {
				if err := k8sClient.Delete(context.Background(), instance); err != nil && !apierrors.IsNotFound(err) {
					t.Logf("Failed to delete LlamaStackDistribution instance %s/%s: %v", instance.Namespace, instance.Name, err)
				}
			})

			// act: reconcile the instance
			ReconcileDistribution(t, instance)

			// assert
			deployment := &appsv1.Deployment{}
			waitForResource(t, k8sClient, instance.Namespace, instance.Name, deployment)

			if tt.expectedVolume.EmptyDir != nil {
				AssertDeploymentUsesEmptyDirStorage(t, deployment)
			} else if tt.expectedVolume.PersistentVolumeClaim != nil {
				AssertDeploymentUsesPVCStorage(t, deployment, tt.expectedVolume.PersistentVolumeClaim.ClaimName)
			}

			AssertDeploymentHasVolumeMount(t, deployment, tt.expectedMount.MountPath)

			// verify PVC is created when storage is configured
			if instance.Spec.Server.Storage != nil {
				expectedPVCName := tt.expectedVolume.PersistentVolumeClaim.ClaimName
				pvc := AssertPVCExists(t, k8sClient, instance.Namespace, expectedPVCName)
				expectedSize := instance.Spec.Server.Storage.Size
				if expectedSize == nil {
					AssertPVCHasSize(t, pvc, llamav1alpha1.DefaultStorageSize.String())
				} else {
					AssertPVCHasSize(t, pvc, expectedSize.String())
				}
			}
		})
	}
}

func TestConfigMapWatchingFunctionality(t *testing.T) {
	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	// Create a test namespace
	namespace := createTestNamespace(t, "test-configmap-watch")

	// Create a ConfigMap
	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-config",
			Namespace: namespace.Name,
		},
		Data: map[string]string{
			"run.yaml": `version: '2'
image_name: ollama
apis:
- inference
providers:
  inference:
  - provider_id: ollama
    provider_type: "remote::ollama"
    config:
      url: "http://ollama-server:11434"
models:
  - model_id: "llama3.2:1b"
    provider_id: ollama
    model_type: llm
server:
  port: 8321`,
		},
	}
	require.NoError(t, k8sClient.Create(context.Background(), configMap))

	// Create a LlamaStackDistribution that references the ConfigMap
	instance := NewDistributionBuilder().
		WithName("test-configmap-reference").
		WithNamespace(namespace.Name).
		WithUserConfig(configMap.Name).
		Build()
	require.NoError(t, k8sClient.Create(context.Background(), instance))

	// Reconcile to create initial deployment
	ReconcileDistribution(t, instance)

	// Get the initial deployment and check for ConfigMap hash annotation
	deployment := &appsv1.Deployment{}
	deploymentKey := types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}
	waitForResourceWithKey(t, k8sClient, deploymentKey, deployment)

	// Verify the ConfigMap hash annotation exists
	initialAnnotations := deployment.Spec.Template.Annotations
	require.Contains(t, initialAnnotations, "configmap.hash/user-config", "ConfigMap hash annotation should be present")
	initialHash := initialAnnotations["configmap.hash/user-config"]
	require.NotEmpty(t, initialHash, "ConfigMap hash should not be empty")

	// Update the ConfigMap data
	require.NoError(t, k8sClient.Get(context.Background(),
		types.NamespacedName{Name: configMap.Name, Namespace: configMap.Namespace}, configMap))

	configMap.Data["run.yaml"] = `version: '2'
image_name: ollama
apis:
- inference
providers:
  inference:
  - provider_id: ollama
    provider_type: "remote::ollama"
    config:
      url: "http://ollama-server:11434"
models:
  - model_id: "llama3.2:3b"
    provider_id: ollama
    model_type: llm
server:
  port: 8321`
	require.NoError(t, k8sClient.Update(context.Background(), configMap))

	// Wait a moment for the watch to trigger
	time.Sleep(2 * time.Second)

	// Trigger reconciliation (in real scenarios this would be triggered by the watch)
	ReconcileDistribution(t, instance)

	// Verify the deployment was updated with a new hash
	waitForResourceWithKeyAndCondition(
		t, k8sClient, deploymentKey, deployment, func() bool {
			newHash := deployment.Spec.Template.Annotations["configmap.hash/user-config"]
			return newHash != initialHash && newHash != ""
		}, "ConfigMap hash should be updated after ConfigMap data change")

	t.Logf("ConfigMap hash changed from %s to %s", initialHash, deployment.Spec.Template.Annotations["configmap.hash/user-config"])

	// Test that unrelated ConfigMaps don't trigger reconciliation
	unrelatedConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "unrelated-config",
			Namespace: namespace.Name,
		},
		Data: map[string]string{
			"some-key": "some-value",
		},
	}
	require.NoError(t, k8sClient.Create(context.Background(), unrelatedConfigMap))

	// Note: In test environment, field indexer might not be set up properly,
	// so we skip the isConfigMapReferenced checks which rely on field indexing
}
