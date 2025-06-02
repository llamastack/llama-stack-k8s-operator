//nolint:testpackage
package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/llamastack/llama-stack-k8s-operator/api/v1alpha1"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestCreationSuite(t *testing.T) {
	if TestOpts.SkipCreation {
		t.Skip("Skipping creation test suite")
	}

	var llsdistributionCR *v1alpha1.LlamaStackDistribution

	t.Run("should create LlamaStackDistribution", func(t *testing.T) {
		llsdistributionCR = testCreateDistribution(t)
	})

	t.Run("should create PVC if storage is configured", func(t *testing.T) {
		testPVCConfiguration(t, llsdistributionCR)
	})

	t.Run("should handle direct deployment updates", func(t *testing.T) {
		testDirectDeploymentUpdates(t, llsdistributionCR)
	})

	t.Run("should check health status", func(t *testing.T) {
		testHealthStatus(t, llsdistributionCR)
	})

	t.Run("should update deployment through CR", func(t *testing.T) {
		testCRDeploymentUpdate(t, llsdistributionCR)
	})

	t.Run("should update distribution status", func(t *testing.T) {
		testDistributionStatus(t, llsdistributionCR)
	})
}

func testCreateDistribution(t *testing.T) *v1alpha1.LlamaStackDistribution {
	t.Helper()
	// Create test namespace
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "llama-stack-test",
		},
	}
	err := TestEnv.Client.Create(TestEnv.Ctx, ns)
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		require.NoError(t, err)
	}

	// Get sample CR
	llsdistributionCR := GetSampleCR(t)
	llsdistributionCR.Namespace = ns.Name

	err = TestEnv.Client.Create(TestEnv.Ctx, llsdistributionCR)
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		require.NoError(t, err)
	}

	// Wait for deployment to be ready
	err = EnsureResourceReady(t, TestEnv, schema.GroupVersionKind{
		Group:   "apps",
		Version: "v1",
		Kind:    "Deployment",
	}, llsdistributionCR.Name, ns.Name, ResourceReadyTimeout, isDeploymentReady)
	require.NoError(t, err)

	// Verify service is created
	err = EnsureResourceReady(t, TestEnv, schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "Service",
	}, llsdistributionCR.Name+"-service", ns.Name, ResourceReadyTimeout, func(u *unstructured.Unstructured) bool {
		// Check if the service has a valid spec and status
		spec, specFound, _ := unstructured.NestedMap(u.Object, "spec")
		status, statusFound, _ := unstructured.NestedMap(u.Object, "status")
		return specFound && statusFound && spec != nil && status != nil
	})
	require.NoError(t, err)

	return llsdistributionCR
}

func testDirectDeploymentUpdates(t *testing.T, distribution *v1alpha1.LlamaStackDistribution) {
	t.Helper()
	// Get the deployment
	deployment := &appsv1.Deployment{}
	err := TestEnv.Client.Get(TestEnv.Ctx, client.ObjectKey{
		Namespace: distribution.Namespace,
		Name:      distribution.Name,
	}, deployment)
	require.NoError(t, err)

	originalReplicas := *deployment.Spec.Replicas
	*deployment.Spec.Replicas = 2
	err = TestEnv.Client.Update(TestEnv.Ctx, deployment)
	require.NoError(t, err)

	// Wait for operator to reconcile
	time.Sleep(5 * time.Second)

	// Verify deployment is reverted to original state
	err = TestEnv.Client.Get(TestEnv.Ctx, client.ObjectKey{
		Namespace: distribution.Namespace,
		Name:      distribution.Name,
	}, deployment)
	require.NoError(t, err)
	require.Equal(t, originalReplicas, *deployment.Spec.Replicas, "Deployment should be reverted to original state")
}

func testCRDeploymentUpdate(t *testing.T, distribution *v1alpha1.LlamaStackDistribution) {
	t.Helper()
	// Update CR
	err := TestEnv.Client.Get(TestEnv.Ctx, client.ObjectKey{
		Namespace: distribution.Namespace,
		Name:      distribution.Name,
	}, distribution)
	require.NoError(t, err)

	// Update replicas
	distribution.Spec.Replicas = 2
	err = TestEnv.Client.Update(TestEnv.Ctx, distribution)
	require.NoError(t, err)

	// Wait for deployment to be updated and ready with new replicas
	err = EnsureResourceReady(t, TestEnv, schema.GroupVersionKind{
		Group:   "apps",
		Version: "v1",
		Kind:    "Deployment",
	}, distribution.Name, distribution.Namespace, ResourceReadyTimeout, func(u *unstructured.Unstructured) bool {
		availableReplicas, found, nestedErr := unstructured.NestedInt64(u.Object, "status", "availableReplicas")
		if !found || nestedErr != nil {
			return false
		}
		return availableReplicas == 2
	})
	require.NoError(t, err, "Failed to wait for deployment to update replicas")

	// Verify deployment is updated
	deployment := &appsv1.Deployment{}
	err = TestEnv.Client.Get(TestEnv.Ctx, client.ObjectKey{
		Namespace: distribution.Namespace,
		Name:      distribution.Name,
	}, deployment)
	require.NoError(t, err)
	require.Equal(t, int32(2), deployment.Status.AvailableReplicas, "Deployment should have 2 available replicas")
}

func testHealthStatus(t *testing.T, distribution *v1alpha1.LlamaStackDistribution) {
	t.Helper()
	// Wait for status to be updated with a longer interval to avoid rate limiting
	err := wait.PollUntilContextTimeout(TestEnv.Ctx, 1*time.Minute, 5*time.Minute, true, func(ctx context.Context) (bool, error) {
		// Get the latest state of the distribution
		updatedDistribution := &v1alpha1.LlamaStackDistribution{}
		err := TestEnv.Client.Get(ctx, client.ObjectKey{
			Namespace: distribution.Namespace,
			Name:      distribution.Name,
		}, updatedDistribution)
		if err != nil {
			return false, err
		}
		return updatedDistribution.Status.Ready, nil
	})
	require.NoError(t, err, "Failed to wait for distribution status update")
}

func testDistributionStatus(t *testing.T, llsdistributionCR *v1alpha1.LlamaStackDistribution) {
	t.Helper()
	// Wait for status to be updated with distribution info
	err := wait.PollUntilContextTimeout(TestEnv.Ctx, 1*time.Minute, 5*time.Minute, true, func(ctx context.Context) (bool, error) {
		updatedDistribution := &v1alpha1.LlamaStackDistribution{}
		err := TestEnv.Client.Get(ctx, client.ObjectKey{
			Namespace: llsdistributionCR.Namespace,
			Name:      llsdistributionCR.Name,
		}, updatedDistribution)
		if err != nil {
			return false, err
		}

		// Check that distribution config is populated
		if len(updatedDistribution.Status.DistributionConfig.AvailableDistributions) == 0 {
			return false, nil
		}

		// Verify that the active distribution is set
		if updatedDistribution.Status.DistributionConfig.ActiveDistribution == "" {
			return false, nil
		}

		return true, nil
	})
	require.NoError(t, err, "Failed to wait for distribution status update")

	// Get final state and verify
	updatedDistribution := &v1alpha1.LlamaStackDistribution{}
	err = TestEnv.Client.Get(TestEnv.Ctx, client.ObjectKey{
		Namespace: llsdistributionCR.Namespace,
		Name:      llsdistributionCR.Name,
	}, updatedDistribution)
	require.NoError(t, err)

	// Verify distribution config
	require.NotEmpty(t, updatedDistribution.Status.DistributionConfig.AvailableDistributions,
		"Available distributions should be populated")
	require.Equal(t, updatedDistribution.Spec.Server.Distribution.Name,
		updatedDistribution.Status.DistributionConfig.ActiveDistribution,
		"Active distribution should match the spec")
}

func testPVCConfiguration(t *testing.T, distribution *v1alpha1.LlamaStackDistribution) {
	t.Helper()
	pvcName := distribution.Name + "-pvc"
	pvc := &corev1.PersistentVolumeClaim{}
	err := TestEnv.Client.Get(TestEnv.Ctx, client.ObjectKey{
		Namespace: distribution.Namespace,
		Name:      pvcName,
	}, pvc)
	if distribution.Spec.Server.Storage == nil {
		require.Error(t, err, "PVC should not exist when storage is not configured")
		require.True(t, k8serrors.IsNotFound(err), "Expected not found error for PVC when storage is not configured")
	} else {
		require.NoError(t, err, "PVC should be created when storage is configured")
		// Check storage size
		expectedSize := v1alpha1.DefaultStorageSize
		if distribution.Spec.Server.Storage.Size != nil {
			expectedSize = *distribution.Spec.Server.Storage.Size
		}
		actualSize := pvc.Spec.Resources.Requests[corev1.ResourceStorage]
		require.Equal(t, expectedSize.String(), actualSize.String(), "PVC storage size should match CR")
	}
}

func isDeploymentReady(u *unstructured.Unstructured) bool {
	replicas, found, err := unstructured.NestedInt64(u.Object, "status", "replicas")
	if !found || err != nil {
		return false
	}
	availableReplicas, found, err := unstructured.NestedInt64(u.Object, "status", "availableReplicas")
	return found && err == nil && availableReplicas == replicas
}
