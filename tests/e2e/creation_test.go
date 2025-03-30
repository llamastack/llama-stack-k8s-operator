package e2e

import (
	"testing"

	"github.com/meta-llama/llama-stack-k8s-operator/api/v1alpha1"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCreationSuite(t *testing.T) {
	if testOpts.skipCreation {
		t.Skip("Skipping creation test suite")
	}

	instance := &v1alpha1.LlamaStackDistribution{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-llama-stack",
			Namespace: ollamaNS,
		},
		Spec: v1alpha1.LlamaStackDistributionSpec{
			Replicas: 1,
			Server: v1alpha1.ServerSpec{
				Distribution: v1alpha1.Ollamadistribution,
				ContainerSpec: v1alpha1.ContainerSpec{
					Port: 8321,
					Resources: corev1.ResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceCPU:    resource.MustParse("500m"),
							corev1.ResourceMemory: resource.MustParse("1Gi"),
						},
					},
				},
			},
		},
	}

	t.Run("should create LlamaStackDistribution CR", func(t *testing.T) {
		if !testOpts.ShouldRunComponent("ollama") {
			t.Skip("Skipping Ollama component creation")
		}

		err := testEnv.Client.Create(testEnv.Ctx, instance)
		require.NoError(t, err)

		// Verify deployment is created and ready
		PollDeploymentReady(t, testEnv.Client, instance.Name, instance.Namespace, testTimeout)

		// Verify service is created and ready
		PollServiceReady(t, testEnv.Client, instance.Name+"-service", instance.Namespace, testTimeout)

		// Verify CR is ready
		PollCRReady(t, testEnv.Client, instance.Name, instance.Namespace, testTimeout)
	})
}
