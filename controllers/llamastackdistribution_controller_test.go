package controllers

import (
	"context"
	"strings"
	"testing"

	llamav1alpha1 "github.com/meta-llama/llama-stack-k8s-operator/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// baseInstance returns a minimal valid LlamaStackDistribution instance
func baseInstance() *llamav1alpha1.LlamaStackDistribution {
	// Set the environment variable for the test
	llamav1alpha1.ImageMap["ollama"] = "lls/lls-ollama:1.0"
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

func TestStorageConfiguration(t *testing.T) {
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
				Size:      "20Gi",
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
				if expectedSize == "" {
					expectedSize = llamav1alpha1.DefaultStorageSize
				}
				verifyPVC(t, client, instance, expectedSize)
			}
		})
	}
}

func TestInvalidStorageSize(t *testing.T) {
	t.Run("should return error for invalid storage size", func(t *testing.T) {
		// Create a LlamaStackDistribution with invalid storage size
		instance := &llamav1alpha1.LlamaStackDistribution{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-invalid-storage",
				Namespace: "default",
			},
			Spec: llamav1alpha1.LlamaStackDistributionSpec{
				Server: llamav1alpha1.ServerSpec{
					Storage: &llamav1alpha1.StorageSpec{
						Size: "invalid-size", // Invalid size format
					},
				},
			},
		}

		_, reconciler := setupTestReconciler(instance)
		_, err := reconciler.Reconcile(context.Background(), ctrl.Request{
			NamespacedName: types.NamespacedName{
				Name:      instance.Name,
				Namespace: instance.Namespace,
			},
		})
		require.Error(t, err, "Expected error for invalid storage size")

		errMsg := strings.ToLower(err.Error())
		assert.Contains(t, errMsg, "invalid", "Error message should contain 'invalid'")
		assert.Contains(t, errMsg, "size", "Error message should contain 'size'")
		assert.Contains(t, errMsg, "storage", "Error message should contain 'storage'")
	})
}

// setupTestReconciler creates a fake client and reconciler for testing
func setupTestReconciler(instance *llamav1alpha1.LlamaStackDistribution) (client.WithWatch, *LlamaStackDistributionReconciler) {
	scheme := runtime.NewScheme()
	_ = llamav1alpha1.AddToScheme(scheme)
	_ = appsv1.AddToScheme(scheme)
	_ = corev1.AddToScheme(scheme)

	client := fake.NewClientBuilder().
		WithScheme(scheme).
		WithObjects(instance).
		Build()

	// Create the reconciler
	reconciler := &LlamaStackDistributionReconciler{
		Client: client,
		Scheme: scheme,
		Log:    ctrl.Log.WithName("controllers").WithName("LlamaStackDistribution"),
	}
	return client, reconciler
}

func verifyVolume(t *testing.T, volumes []corev1.Volume, expectedVolume corev1.Volume) {
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

func verifyPVC(t *testing.T, client client.Client, instance *llamav1alpha1.LlamaStackDistribution, expectedSize string) {
	pvc := &corev1.PersistentVolumeClaim{}
	err := client.Get(context.Background(), types.NamespacedName{
		Name:      instance.Name + "-pvc",
		Namespace: instance.Namespace,
	}, pvc)
	require.NoError(t, err, "should be able to get PVC")
	assert.Equal(t, expectedSize, pvc.Spec.Resources.Requests.Storage().String(),
		"PVC size should match")
}
