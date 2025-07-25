package controllers_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	llamav1alpha1 "github.com/llamastack/llama-stack-k8s-operator/api/v1alpha1"
	controllers "github.com/llamastack/llama-stack-k8s-operator/controllers"
	"github.com/llamastack/llama-stack-k8s-operator/pkg/cluster"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	testTimeout  = 5 * time.Second
	testInterval = 100 * time.Millisecond

	// Test-specific identifiers (not production defaults).
	testImage             = "lls/lls-ollama:1.0"
	testOperatorNamespace = "default"
	testStorageVolumeName = "lls-storage"
	testInstanceName      = "test-instance"
)

// DistributionBuilder - Builder pattern for test instances of operator custom resource.
type DistributionBuilder struct {
	instance *llamav1alpha1.LlamaStackDistribution
}

func NewDistributionBuilder() *DistributionBuilder {
	return &DistributionBuilder{
		instance: &llamav1alpha1.LlamaStackDistribution{
			ObjectMeta: metav1.ObjectMeta{
				Name:      testInstanceName,
				Namespace: "default", // Will be overridden in tests
			},
			Spec: llamav1alpha1.LlamaStackDistributionSpec{
				Replicas: 1,
				Server: llamav1alpha1.ServerSpec{
					Distribution: llamav1alpha1.DistributionType{
						Name: "starter", // Real distribution from distributions.json
					},
					ContainerSpec: llamav1alpha1.ContainerSpec{
						Name: llamav1alpha1.DefaultContainerName,
						Port: llamav1alpha1.DefaultServerPort,
					},
				},
			},
		},
	}
}

func (b *DistributionBuilder) WithName(name string) *DistributionBuilder {
	b.instance.Name = name
	return b
}

func (b *DistributionBuilder) WithNamespace(namespace string) *DistributionBuilder {
	b.instance.Namespace = namespace
	return b
}

func (b *DistributionBuilder) WithPort(port int32) *DistributionBuilder {
	b.instance.Spec.Server.ContainerSpec.Port = port
	return b
}

func (b *DistributionBuilder) WithReplicas(replicas int32) *DistributionBuilder {
	b.instance.Spec.Replicas = replicas
	return b
}

func (b *DistributionBuilder) WithStorage(storage *llamav1alpha1.StorageSpec) *DistributionBuilder {
	b.instance.Spec.Server.Storage = storage
	return b
}

func (b *DistributionBuilder) WithDistribution(distributionName string) *DistributionBuilder {
	b.instance.Spec.Server.Distribution.Name = distributionName
	return b
}

func (b *DistributionBuilder) WithResources(resources corev1.ResourceRequirements) *DistributionBuilder {
	b.instance.Spec.Server.ContainerSpec.Resources = resources
	return b
}

func (b *DistributionBuilder) WithServiceAccountName(serviceAccountName string) *DistributionBuilder {
	if b.instance.Spec.Server.PodOverrides == nil {
		b.instance.Spec.Server.PodOverrides = &llamav1alpha1.PodOverrides{}
	}
	b.instance.Spec.Server.PodOverrides.ServiceAccountName = serviceAccountName
	return b
}

func (b *DistributionBuilder) WithUserConfig(configMapName string) *DistributionBuilder {
	b.instance.Spec.Server.UserConfig = &llamav1alpha1.UserConfigSpec{
		ConfigMapName: configMapName,
	}
	return b
}

func (b *DistributionBuilder) Build() *llamav1alpha1.LlamaStackDistribution {
	return b.instance.DeepCopy()
}

func DefaultTestStorage() *llamav1alpha1.StorageSpec {
	return &llamav1alpha1.StorageSpec{}
}

func CustomTestStorage(size string, mountPath string) *llamav1alpha1.StorageSpec {
	sizeQuantity := resource.MustParse(size)
	return &llamav1alpha1.StorageSpec{
		Size:      &sizeQuantity,
		MountPath: mountPath,
	}
}

func AssertDeploymentHasCorrectImage(t *testing.T, deployment *appsv1.Deployment, expectedImage string) {
	t.Helper()
	require.NotEmpty(t, deployment.Spec.Template.Spec.Containers,
		"deployment should have at least one container")

	actualImage := deployment.Spec.Template.Spec.Containers[0].Image
	require.Equal(t, expectedImage, actualImage,
		"deployment container should use the correct image")
}

func AssertDeploymentHasPort(t *testing.T, deployment *appsv1.Deployment, expectedPort int32) {
	t.Helper()
	require.NotEmpty(t, deployment.Spec.Template.Spec.Containers,
		"deployment should have at least one container")

	container := deployment.Spec.Template.Spec.Containers[0]
	require.NotEmpty(t, container.Ports, "container should expose at least one port")

	actualPort := container.Ports[0].ContainerPort
	require.Equal(t, expectedPort, actualPort,
		"container should expose port %d", expectedPort)
}

func AssertDeploymentUsesEmptyDirStorage(t *testing.T, deployment *appsv1.Deployment) {
	t.Helper()
	volume := findVolumeByName(t, deployment, testStorageVolumeName)
	require.NotNil(t, volume.EmptyDir, "deployment should use emptyDir storage")
	require.Nil(t, volume.PersistentVolumeClaim, "deployment should not use PVC storage")
}

func AssertDeploymentUsesPVCStorage(t *testing.T, deployment *appsv1.Deployment, expectedPVCName string) {
	t.Helper()
	volume := findVolumeByName(t, deployment, testStorageVolumeName)
	require.NotNil(t, volume.PersistentVolumeClaim, "deployment should use PVC storage")
	require.Nil(t, volume.EmptyDir, "deployment should not use emptyDir storage")
	require.Equal(t, expectedPVCName, volume.PersistentVolumeClaim.ClaimName,
		"deployment should reference the correct PVC")
}

func AssertDeploymentHasVolumeMount(t *testing.T, deployment *appsv1.Deployment, expectedMountPath string) {
	t.Helper()
	require.NotEmpty(t, deployment.Spec.Template.Spec.Containers,
		"deployment should have at least one container")

	container := deployment.Spec.Template.Spec.Containers[0]
	mount := findVolumeMountByName(t, container, testStorageVolumeName)
	require.Equal(t, expectedMountPath, mount.MountPath,
		"volume should be mounted at the correct path")
}

func AssertPVCExists(t *testing.T, client client.Client, namespace, name string) *corev1.PersistentVolumeClaim {
	t.Helper()
	pvc := &corev1.PersistentVolumeClaim{}
	key := types.NamespacedName{Name: name, Namespace: namespace}

	require.Eventually(t, func() bool {
		return client.Get(context.Background(), key, pvc) == nil
	}, testTimeout, testInterval, "PVC %s should exist in namespace %s", name, namespace)

	return pvc
}

func AssertServiceExposesDeployment(t *testing.T, service *corev1.Service, deployment *appsv1.Deployment) {
	t.Helper()

	// Behavior: Service should target deployment pods via selectors
	require.Equal(t, service.Spec.Selector, deployment.Spec.Template.Labels,
		"service selector should match deployment pod labels for traffic routing")

	// Behavior: Service port should route to deployment container port
	require.NotEmpty(t, service.Spec.Ports, "service should expose at least one port")
	require.NotEmpty(t, deployment.Spec.Template.Spec.Containers, "deployment should have at least one container")

	servicePort := service.Spec.Ports[0]
	containerPort := deployment.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort
	require.Equal(t, servicePort.TargetPort.IntVal, containerPort,
		"service target port should route to deployment container port")
}

func AssertNetworkPolicyProtectsDeployment(t *testing.T, networkPolicy *networkingv1.NetworkPolicy, deployment *appsv1.Deployment) {
	t.Helper()

	// Behavior: NetworkPolicy should target the same pods as deployment
	require.Equal(t, deployment.Spec.Template.Labels, networkPolicy.Spec.PodSelector.MatchLabels,
		"network policy should protect the same pods as deployment")

	// Behavior: NetworkPolicy should allow traffic on deployment container port
	require.NotEmpty(t, deployment.Spec.Template.Spec.Containers, "deployment should have containers")
	require.NotEmpty(t, networkPolicy.Spec.Ingress, "network policy should have ingress rules")
	require.NotEmpty(t, networkPolicy.Spec.Ingress[0].Ports, "network policy should allow specific ports")

	containerPort := deployment.Spec.Template.Spec.Containers[0].Ports[0].ContainerPort
	policyPort := networkPolicy.Spec.Ingress[0].Ports[0].Port.IntVal
	require.Equal(t, containerPort, policyPort,
		"network policy should allow traffic on deployment container port")
}

func AssertResourceOwnedByInstance(t *testing.T, resource metav1.Object, instance *llamav1alpha1.LlamaStackDistribution) {
	t.Helper()

	// Behavior: Resource should be owned by the LlamaStackDistribution instance
	ownerRefs := resource.GetOwnerReferences()
	require.Len(t, ownerRefs, 1, "resource should have exactly one owner reference")
	require.Equal(t, instance.GetUID(), ownerRefs[0].UID,
		"resource should be owned by the LlamaStackDistribution instance for garbage collection")
}

func AssertClusterRoleBindingLinksServiceAccount(t *testing.T, crb *rbacv1.ClusterRoleBinding, serviceAccount *corev1.ServiceAccount) {
	t.Helper()

	// Behavior: ClusterRoleBinding should grant permissions to the ServiceAccount
	require.NotEmpty(t, crb.Subjects, "cluster role binding should have subjects")

	found := false
	for _, subject := range crb.Subjects {
		if subject.Kind == "ServiceAccount" &&
			subject.Name == serviceAccount.Name &&
			subject.Namespace == serviceAccount.Namespace {
			found = true
			break
		}
	}
	require.True(t, found,
		"cluster role binding should grant permissions to the service account %s/%s",
		serviceAccount.Namespace, serviceAccount.Name)
}

func AssertDeploymentUsesServiceAccount(t *testing.T, deployment *appsv1.Deployment, serviceAccount *corev1.ServiceAccount) {
	t.Helper()

	// Behavior: Deployment should use the ServiceAccount for pod identity
	require.Equal(t, serviceAccount.Name, deployment.Spec.Template.Spec.ServiceAccountName,
		"deployment pods should use the service account for proper permissions")
}

func AssertPVCHasSize(t *testing.T, pvc *corev1.PersistentVolumeClaim, expectedSize string) {
	t.Helper()
	storageRequest, ok := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
	require.True(t, ok, "PVC should have storage request")

	expectedQuantity := resource.MustParse(expectedSize)
	require.True(t, expectedQuantity.Equal(storageRequest),
		"PVC should request %s storage, got %s", expectedSize, storageRequest.String())
}

func ReconcileDistribution(t *testing.T, instance *llamav1alpha1.LlamaStackDistribution) {
	t.Helper()
	// Create reconciler and run reconciliation
	reconciler := createTestReconciler()
	_, err := reconciler.Reconcile(context.Background(), ctrl.Request{
		NamespacedName: types.NamespacedName{
			Name:      instance.Name,
			Namespace: instance.Namespace,
		},
	})
	require.NoError(t, err, "reconciliation should succeed")
}

func ResourceTestName(instanceName, suffix string) string {
	return instanceName + suffix
}

func createTestReconciler() *controllers.LlamaStackDistributionReconciler {
	clusterInfo := &cluster.ClusterInfo{
		OperatorNamespace: testOperatorNamespace,
		DistributionImages: map[string]string{
			"starter": testImage, // Use same distribution as builder
		},
	}
	return &controllers.LlamaStackDistributionReconciler{
		Client:      k8sClient,
		Scheme:      scheme.Scheme,
		ClusterInfo: clusterInfo,
	}
}

func findVolumeByName(t *testing.T, deployment *appsv1.Deployment, volumeName string) *corev1.Volume {
	t.Helper()
	for _, volume := range deployment.Spec.Template.Spec.Volumes {
		if volume.Name == volumeName {
			return &volume
		}
	}
	require.Fail(t, "volume not found", "deployment should have volume named %s", volumeName)
	return nil
}

func findVolumeMountByName(t *testing.T, container corev1.Container, volumeName string) *corev1.VolumeMount {
	t.Helper()
	for _, mount := range container.VolumeMounts {
		if mount.Name == volumeName {
			return &mount
		}
	}
	require.Fail(t, "volume mount not found", "container should have volume mount named %s", volumeName)
	return nil
}

// waitForResource waits for a resource to exist (convenience version).
func waitForResource(t *testing.T, client client.Client, namespace, name string, resource client.Object) {
	t.Helper()
	key := types.NamespacedName{Name: name, Namespace: namespace}
	waitForResourceWithKey(t, client, key, resource)
}

// waitForResourceWithKey waits for a resource using an existing NamespacedName.
func waitForResourceWithKey(t *testing.T, client client.Client, key types.NamespacedName, resource client.Object) {
	t.Helper()
	waitForResourceWithKeyAndCondition(t, client, key, resource, nil, fmt.Sprintf("timed out waiting for %T %s to be available", resource, key))
}

// waitForResourceWithKeyAndCondition provides the full flexibility for complex conditions.
func waitForResourceWithKeyAndCondition(t *testing.T, client client.Client, key types.NamespacedName, resource client.Object, condition func() bool, message string) {
	t.Helper()
	// envtest interacts with a real API server, which is eventually consistent.
	require.Eventually(t, func() bool {
		err := client.Get(context.Background(), key, resource)
		if err != nil {
			return false
		}
		// If no condition specified, just check existence
		if condition == nil {
			return true
		}
		// Otherwise check the custom condition
		return condition()
	}, testTimeout, testInterval, message)
}

// createTestNamespace creates a unique test namespace and registers cleanup.
func createTestNamespace(t *testing.T, namePrefix string) *corev1.Namespace {
	t.Helper()
	// envtest does not fully support namespace deletion and cleanup between test cases.
	// To ensure test isolation and avoid interference, a unique namespace is created for each test run.
	testenvNamespaceCounter++
	nsName := fmt.Sprintf("%s-%d", namePrefix, testenvNamespaceCounter)
	namespace := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: nsName,
		},
	}
	require.NoError(t, k8sClient.Create(context.Background(), namespace))

	// Attempt to delete the namespace after the test. While envtest might not fully reclaim it,
	// this is good practice and helps keep the test environment cleaner.
	t.Cleanup(func() {
		if err := k8sClient.Delete(context.Background(), namespace); err != nil {
			t.Logf("Failed to delete test namespace %s: %v", namespace.Name, err)
		}
	})
	return namespace
}
