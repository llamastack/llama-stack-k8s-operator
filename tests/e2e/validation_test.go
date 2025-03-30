package e2e

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestValidationSuite(t *testing.T) {
	if testOpts.skipValidation {
		t.Skip("Skipping validation test suite")
	}

	t.Run("should validate CRDs", func(t *testing.T) {
		err := validateCRD(testEnv.Client, testEnv.Ctx, "llamastackdistributions.llama.x-k8s.io")
		require.NoErrorf(t, err, "error in validating CRD: llamastackdistributions.llama.x-k8s.io")
	})

	t.Run("should validate operator pod is running", func(t *testing.T) {
		podList := &corev1.PodList{}
		err := testEnv.Client.List(testEnv.Ctx, podList, client.InNamespace(testOpts.operatorNS))
		require.NoError(t, err)

		operatorPodFound := false
		for _, pod := range podList.Items {
			if pod.Labels["app.kubernetes.io/name"] == "llama-stack-k8s-operator" {
				operatorPodFound = true
				require.Equal(t, corev1.PodRunning, pod.Status.Phase)
				break
			}
		}
		require.True(t, operatorPodFound, "Operator pod not found in namespace %s", testOpts.operatorNS)
	})

	t.Run("should validate Ollama server", func(t *testing.T) {
		if !testOpts.ShouldRunComponent("ollama") {
			t.Skip("Skipping Ollama server validation")
		}
		// ... existing Ollama server validation code ...
	})

	t.Run("should validate service account and SCC", func(t *testing.T) {
		if testOpts.skipSCCValidation {
			t.Skip("Skipping SCC validation")
		}
		// ... existing SCC validation code ...
	})
}
