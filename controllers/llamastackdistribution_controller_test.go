package controllers

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	llamav1alpha1 "github.com/llamastack/llama-stack-k8s-operator/api/v1alpha1"
	"github.com/llamastack/llama-stack-k8s-operator/pkg/cluster"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	kubernetesscheme "k8s.io/client-go/kubernetes/scheme" // Alias to avoid conflict
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
)

const (
	eventuallyTimeout  = 5 * time.Second
	eventuallyInterval = 100 * time.Millisecond
)

// testenvNamespaceCounter is used to generate unique namespace names for test isolation.
var testenvNamespaceCounter int

// baseInstance returns a minimal valid LlamaStackDistribution instance.
// Namespace will be set to "default" and should be overridden by the caller if needed for specific test contexts.
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

func setupTestReconciler(ctrlRuntimeClient client.Client, currentScheme *runtime.Scheme) *LlamaStackDistributionReconciler {
	// ClusterInfo is required by the reconciler. We provide static test data for it.
	clusterInfo := &cluster.ClusterInfo{
		OperatorNamespace: "default",
		DistributionImages: map[string]string{
			"ollama": "lls/lls-ollama:1.0",
		},
	}
	return &LlamaStackDistributionReconciler{
		Client:      ctrlRuntimeClient,
		Scheme:      currentScheme,
		ClusterInfo: clusterInfo,
	}
}

func TestStorageConfiguration(t *testing.T) {
	ctrl.SetLogger(zap.New(zap.UseDevMode(true)))

	testEnv := &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
		// BinaryAssetsDirectory tells envtest where to find etcd and kube-apiserver binaries.
		// It's set from KUBEBUILDER_ASSETS, typically managed by 'make test' in Operator SDK projects,
		// which uses setup-envtest to download project-specific Kubernetes binaries.
		BinaryAssetsDirectory: os.Getenv("KUBEBUILDER_ASSETS"),
	}

	cfg, err := testEnv.Start()
	require.NoError(t, err)
	defer func() { require.NoError(t, testEnv.Stop()) }()

	// The Scheme is a registry of Go types for Kubernetes API objects. We must add all the
	// types that ctrlRuntimeClient will interact with to this scheme so the client knows how to
	// handle them (e.g., for Get, Create, and other operations).
	k8sScheme := runtime.NewScheme()
	require.NoError(t, kubernetesscheme.AddToScheme(k8sScheme))
	require.NoError(t, llamav1alpha1.AddToScheme(k8sScheme))
	require.NoError(t, corev1.AddToScheme(k8sScheme))
	require.NoError(t, appsv1.AddToScheme(k8sScheme))
	require.NoError(t, networkingv1.AddToScheme(k8sScheme))
	require.NoError(t, rbacv1.AddToScheme(k8sScheme))

	ctrlRuntimeClient, err := client.New(cfg, client.Options{Scheme: k8sScheme})
	require.NoError(t, err)
	require.NotNil(t, ctrlRuntimeClient)

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
			// envtest does not fully support namespace deletion and cleanup between test cases.
			// To ensure test isolation and avoid interference, a unique namespace is created for each test run.
			testenvNamespaceCounter++
			nsName := fmt.Sprintf("test-storage-%d", testenvNamespaceCounter)
			ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: nsName}}
			require.NoError(t, ctrlRuntimeClient.Create(context.Background(), ns))

			// Attempt to delete the namespace after the test. While envtest might not fully reclaim it,
			// this is good practice and helps keep the test environment cleaner.
			defer func() {
				if err := ctrlRuntimeClient.Delete(context.Background(), ns); err != nil && !apierrors.IsNotFound(err) {
					t.Logf("Failed to delete test namespace %s: %v", nsName, err)
				}
			}()

			// baseInstance creates a generic LlamaStackDistribution object.
			// The namespace is then overridden here to use the unique namespace for this test case.
			instance := baseInstance()
			instance.Namespace = nsName
			instance.Spec.Server.Storage = tt.storage
			require.NoError(t, ctrlRuntimeClient.Create(context.Background(), instance))

			// Attempt to delete the LlamaStackDistribution instance after the test.
			defer func() {
				if err := ctrlRuntimeClient.Delete(context.Background(), instance); err != nil && !apierrors.IsNotFound(err) {
					t.Logf("Failed to delete LlamaStackDistribution instance %s/%s: %v", instance.Namespace, instance.Name, err)
				}
			}()

			// setupTestReconciler creates a reconciler instance with the real Kubernetes client and scheme provided by envtest.
			reconciler := setupTestReconciler(ctrlRuntimeClient, k8sScheme)

			_, reconcileErr := reconciler.Reconcile(context.Background(), ctrl.Request{
				NamespacedName: types.NamespacedName{
					Name:      instance.Name,
					Namespace: instance.Namespace,
				},
			})
			require.NoError(t, reconcileErr, "reconcile should not fail")

			deployment := &appsv1.Deployment{}
			deploymentKey := types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}
			// envtest interacts with a real API server, which is eventually consistent.
			// We use require.Eventually to poll until the Deployment becomes available.
			require.Eventually(t, func() bool {
				err := ctrlRuntimeClient.Get(context.Background(), deploymentKey, deployment)
				return err == nil
			}, eventuallyTimeout, eventuallyInterval, "timed out waiting for deployment %s to be available", deploymentKey)

			verifyVolume(t, deployment.Spec.Template.Spec.Volumes, tt.expectedVolume)
			verifyVolumeMount(t, deployment.Spec.Template.Spec.Containers, tt.expectedMount)

			if tt.storage != nil {
				expectedSize := tt.storage.Size
				if expectedSize == nil {
					expectedSize = &llamav1alpha1.DefaultStorageSize
				}
				verifyPVC(t, ctrlRuntimeClient, instance, expectedSize)
			}
		})
	}
}

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

func verifyPVC(t *testing.T, ctrlRuntimeClient client.Client, instance *llamav1alpha1.LlamaStackDistribution, expectedSize *resource.Quantity) {
	t.Helper()
	pvc := &corev1.PersistentVolumeClaim{}
	pvcKey := types.NamespacedName{Name: instance.Name + "-pvc", Namespace: instance.Namespace}

	// envtest interacts with a real API server, which is eventually consistent.
	// We use require.Eventually to poll until the PVC becomes available after reconciliation.
	require.Eventually(t, func() bool {
		err := ctrlRuntimeClient.Get(context.Background(), pvcKey, pvc)
		return err == nil
	}, eventuallyTimeout, eventuallyInterval, "timed out waiting for PVC %s to be available", pvcKey)

	storageRequest, ok := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
	require.True(t, ok, "PVC does not have storage request")
	assert.Equal(t, expectedSize.String(), storageRequest.String(),
		"PVC size should match")
}
