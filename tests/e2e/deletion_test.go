package e2e

import (
	"testing"

	"github.com/meta-llama/llama-stack-k8s-operator/api/v1alpha1"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestDeletionSuite(t *testing.T) {
	if testOpts.skipDeletion {
		t.Skip("Skipping deletion test suite")
	}

	t.Run("should delete LlamaStackDistribution CR", func(t *testing.T) {
		if !testOpts.ShouldRunComponent("ollama") {
			t.Skip("Skipping Ollama component deletion")
		}

		instance := &v1alpha1.LlamaStackDistribution{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-llama-stack",
				Namespace: ollamaNS,
			},
		}

		// Delete the instance
		err := testEnv.Client.Delete(testEnv.Ctx, instance)
		require.NoError(t, err)

		// Verify deployment is deleted
		PollDeploymentDeleted(t, testEnv.Client, instance.Name, instance.Namespace, testTimeout)

		// Verify service is deleted
		PollServiceDeleted(t, testEnv.Client, instance.Name+"-service", instance.Namespace, testTimeout)

		// Verify CR is deleted
		PollCRDeleted(t, testEnv.Client, instance.Name, instance.Namespace, testTimeout)
	})
}
