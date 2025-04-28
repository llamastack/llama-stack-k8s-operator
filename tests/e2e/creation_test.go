package e2e

import (
	"testing"
	"time"

	"github.com/meta-llama/llama-stack-k8s-operator/api/v1alpha1"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestLlamaStackDistributionCreation(t *testing.T) {
	if TestOpts.SkipCreation {
		t.Skip("Skipping creation test suite")
	}

	// Create test namespace
	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: "llama-stack-test",
		},
	}
	err := TestEnv.Client.Create(TestEnv.Ctx, ns)
	require.NoError(t, err)
	defer func() {
		err := TestEnv.Client.Delete(TestEnv.Ctx, ns)
		require.NoError(t, err)
	}()

	// Create LlamaStackDistribution
	distribution := &v1alpha1.LlamaStackDistribution{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-distribution",
			Namespace: ns.Name,
		},
		Spec: v1alpha1.LlamaStackDistributionSpec{
			Replicas: 1,
			Server: v1alpha1.ServerSpec{
				Distribution: v1alpha1.DistributionType{
					Name: "ollama",
				},
				ContainerSpec: v1alpha1.ContainerSpec{
					Name: "llama-stack",
					Port: 8321,
				},
			},
		},
	}

	err = TestEnv.Client.Create(TestEnv.Ctx, distribution)
	require.NoError(t, err)

	// Wait for deployment to be ready
	err = PollDeploymentReady(t, TestEnv.Client, "llama-stack", ns.Name, 5*time.Minute)
	require.NoError(t, err)

	// Wait for service to be ready
	err = PollServiceReady(t, TestEnv.Client, "llama-stack", ns.Name, 5*time.Minute)
	require.NoError(t, err)
}

func TestCreationSuite(t *testing.T) {
	if TestOpts.SkipCreation {
		t.Skip("Skipping creation test suite")
	}

	var distribution *v1alpha1.LlamaStackDistribution

	t.Run("should create LlamaStackDistribution", func(t *testing.T) {
		// Create test namespace
		ns := &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{
				Name: "llama-stack-test",
			},
		}
		err := TestEnv.Client.Create(TestEnv.Ctx, ns)
		require.NoError(t, err)

		// Create LlamaStackDistribution
		distribution = &v1alpha1.LlamaStackDistribution{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-distribution",
				Namespace: ns.Name,
			},
			Spec: v1alpha1.LlamaStackDistributionSpec{
				Replicas: 1,
				Server: v1alpha1.ServerSpec{
					Distribution: v1alpha1.DistributionType{
						Name: "ollama",
					},
					ContainerSpec: v1alpha1.ContainerSpec{
						Name: "llama-stack",
						Port: 8321,
					},
				},
			},
		}
		err = TestEnv.Client.Create(TestEnv.Ctx, distribution)
		require.NoError(t, err)

		// Wait for deployment to be ready
		err = PollDeploymentReady(t, TestEnv.Client, "llama-stack", ns.Name, 5*time.Minute)
		require.NoError(t, err)

		// Verify service is created
		err = PollServiceReady(t, TestEnv.Client, "llama-stack", ns.Name, 5*time.Minute)
		require.NoError(t, err)
	})

	t.Run("should handle direct deployment updates", func(t *testing.T) {
		if !TestOpts.ShouldRunComponent("ollama") {
			t.Skip("Skipping Ollama component update test")
		}

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
	})

	t.Run("should update deployment through CR", func(t *testing.T) {
		if !TestOpts.ShouldRunComponent("ollama") {
			t.Skip("Skipping Ollama component update test")
		}

		// Update CR
		err := TestEnv.Client.Get(TestEnv.Ctx, client.ObjectKey{
			Namespace: distribution.Namespace,
			Name:      distribution.Name,
		}, distribution)
		require.NoError(t, err)

		distribution.Spec.Replicas = 2
		err = TestEnv.Client.Update(TestEnv.Ctx, distribution)
		require.NoError(t, err)

		// Verify deployment is updated
		err = PollDeploymentReady(t, TestEnv.Client, distribution.Name, distribution.Namespace, 5*time.Minute)
		require.NoError(t, err)

		deployment := &appsv1.Deployment{}
		err = TestEnv.Client.Get(TestEnv.Ctx, client.ObjectKey{
			Namespace: distribution.Namespace,
			Name:      distribution.Name,
		}, deployment)
		require.NoError(t, err)
		require.Equal(t, int32(2), *deployment.Spec.Replicas, "Deployment replicas should be updated")
	})

	t.Run("should check health status", func(t *testing.T) {
		if !TestOpts.ShouldRunComponent("ollama") {
			t.Skip("Skipping Ollama component health check")
		}

		// Get the CR
		err := TestEnv.Client.Get(TestEnv.Ctx, client.ObjectKey{
			Namespace: distribution.Namespace,
			Name:      distribution.Name,
		}, distribution)
		require.NoError(t, err)

		// Check status conditions
		require.NotEmpty(t, distribution.Status.Conditions, "Status conditions should not be empty")
		readyCondition := findCondition(distribution.Status.Conditions, "Ready")
		require.NotNil(t, readyCondition, "Ready condition should exist")
		require.Equal(t, "True", string(readyCondition.Status), "Ready condition should be True")
	})
}
