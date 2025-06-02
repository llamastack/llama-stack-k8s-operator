package controllers

import (
	"context"
	"testing"

	llamav1alpha1 "github.com/llamastack/llama-stack-k8s-operator/api/v1alpha1"
	"github.com/llamastack/llama-stack-k8s-operator/pkg/cluster"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// baseInstance returns a minimal valid LlamaStackDistribution instance.
func baseInstance() *llamav1alpha1.LlamaStackDistribution {
	return &llamav1alpha1.LlamaStackDistribution{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
		Spec: llamav1alpha1.LlamaStackDistributionSpec{
			Replicas: 1,
			Server: llamav1alpha1.ServerSpec{
				Distribution: llamav1alpha1.DistributionType{
					Name: "ollama",
				},
				ContainerSpec: llamav1alpha1.ContainerSpec{
					Name: llamav1alpha1.DefaultContainerName,
					Port: llamav1alpha1.DefaultServerPort,
				},
			},
		},
	}
}

// setupTestReconciler creates a test reconciler with the given instance.
func setupTestReconciler(instance *llamav1alpha1.LlamaStackDistribution) (client.Client, *LlamaStackDistributionReconciler) {
	scheme := runtime.NewScheme()
	_ = llamav1alpha1.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)
	_ = networkingv1.AddToScheme(scheme)

	// Create a fake client with the instance and enable status subresource.
	fakeClient := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(instance).
		WithStatusSubresource(&llamav1alpha1.LlamaStackDistribution{}).
		Build()

	clusterInfo :=
		&cluster.ClusterInfo{
			OperatorNamespace: "default",
			DistributionImages: map[string]string{
				"ollama": "lls/lls-ollama:1.0",
			},
		}

	// Create the reconciler
	reconciler := &LlamaStackDistributionReconciler{
		Client:      fakeClient,
		Scheme:      scheme,
		Log:         ctrl.Log.WithName("controllers").WithName("LlamaStackDistribution"),
		ClusterInfo: clusterInfo,
	}
	return fakeClient, reconciler
}

func TestStorageConfiguration(t *testing.T) {
	// Setup test cluster info

	tests := []struct {
		name           string
		storage        *llamav1alpha1.StorageSpec
		expectedVolume corev1.Volume
		expectedMount  corev1.VolumeMount
	}{
		{
			name:    "No storage configuration - should use emptyDir",
			storage: nil,
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
			name:    "Storage with default values",
			storage: &llamav1alpha1.StorageSpec{},
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
			storage: &llamav1alpha1.StorageSpec{
				Size:      resource.NewQuantity(20*1024*1024*1024, resource.BinarySI), // 20Gi
				MountPath: "/custom/path",
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
			// Create instance with test-specific storage configuration
			instance := baseInstance()
			instance.Spec.Server.Storage = tt.storage

			// Create a fake client and the reconciler with the instance
			client, reconciler := setupTestReconciler(instance)

			// Reconcile
			_, err := reconciler.Reconcile(context.Background(), ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      instance.Name,
					Namespace: instance.Namespace,
				},
			})
			require.NoError(t, err, "reconcile should not fail")

			deployment := &appsv1.Deployment{}
			err = client.Get(context.Background(), types.NamespacedName{
				Name:      instance.Name,
				Namespace: instance.Namespace,
			}, deployment)
			require.NoError(t, err, "should be able to get deployment")

			verifyVolume(t, deployment.Spec.Template.Spec.Volumes, tt.expectedVolume)
			verifyVolumeMount(t, deployment.Spec.Template.Spec.Containers, tt.expectedMount)

			// If storage is configured, verify PVC
			if tt.storage != nil {
				expectedSize := tt.storage.Size
				if expectedSize == nil {
					expectedSize = &llamav1alpha1.DefaultStorageSize
				}
				verifyPVC(t, client, instance, expectedSize)
			}
		})
	}
}

// Helper functions for testing the LlamaStackDistribution controller.
func verifyVolume(t *testing.T, volumes []corev1.Volume, expectedVolume corev1.Volume) {
	t.Helper()
	var foundVolume *corev1.Volume
	for _, volume := range volumes {
		if volume.Name == expectedVolume.Name {
			foundVolume = &volume
			break
		}
	}
	require.NotNil(t, foundVolume, "expected volume %s not found", expectedVolume.Name)

	if expectedVolume.EmptyDir != nil {
		assert.NotNil(t, foundVolume.EmptyDir, "expected emptyDir volume")
		assert.Nil(t, foundVolume.PersistentVolumeClaim, "should not have PVC volume")
	} else {
		assert.NotNil(t, foundVolume.PersistentVolumeClaim, "expected PVC volume")
		assert.Nil(t, foundVolume.EmptyDir, "should not have emptyDir volume")
		assert.Equal(t, expectedVolume.PersistentVolumeClaim.ClaimName,
			foundVolume.PersistentVolumeClaim.ClaimName,
			"PVC claim name should match")
	}
}

func verifyVolumeMount(t *testing.T, containers []corev1.Container, expectedMount corev1.VolumeMount) {
	t.Helper()
	var foundMount *corev1.VolumeMount
	for _, container := range containers {
		for _, mount := range container.VolumeMounts {
			if mount.Name == expectedMount.Name {
				foundMount = &mount
				break
			}
		}
	}
	require.NotNil(t, foundMount, "expected volume mount %s not found", expectedMount.Name)
	assert.Equal(t, expectedMount.MountPath, foundMount.MountPath, "mount path should match")
}

func verifyPVC(t *testing.T, client client.Client, instance *llamav1alpha1.LlamaStackDistribution, expectedSize *resource.Quantity) {
	t.Helper()
	pvc := &corev1.PersistentVolumeClaim{}
	err := client.Get(context.Background(), types.NamespacedName{
		Name:      instance.Name + "-pvc",
		Namespace: instance.Namespace,
	}, pvc)
	require.NoError(t, err, "should be able to get PVC")
	assert.Equal(t, expectedSize.String(), pvc.Spec.Resources.Requests.Storage().String(),
		"PVC size should match")
}
