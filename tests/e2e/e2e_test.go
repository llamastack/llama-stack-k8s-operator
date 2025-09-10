//nolint:testpackage
package e2e

import (
	"testing"

	"github.com/llamastack/llama-stack-k8s-operator/api/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestE2E(t *testing.T) {
	registerSchemes()
	// Run validation tests
	t.Run("validation", TestValidationSuite)

	// Run combined creation and deletion tests
	t.Run("creation-deletion", TestCreationDeletionSuite)

	// Run TLS tests
	t.Run("tls", func(t *testing.T) {
		TestTLSSuite(t)
	})
}

// TestCreationDeletionSuite runs creation tests followed by deletion tests
// This allows for complete lifecycle testing with different distribution images
func TestCreationDeletionSuite(t *testing.T) {
	if TestOpts.SkipCreation {
		t.Skip("Skipping creation-deletion test suite")
	}

	var creationFailed bool

	// Run all creation tests
	t.Run("creation", func(t *testing.T) {
		TestCreationSuite(t)
		creationFailed = t.Failed()
	})

	// Run deletion tests only if creation passed
	if !creationFailed && !TestOpts.SkipDeletion {
		t.Run("deletion", func(t *testing.T) {
			// Create distribution instance for deletion
			instance := &v1alpha1.LlamaStackDistribution{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "llamastackdistribution-sample",
					Namespace: "llama-stack-test",
				},
			}
			runDeletionTests(t, instance)
		})
	} else {
		if TestOpts.SkipDeletion {
			t.Log("Skipping deletion tests (SkipDeletion=true)")
		} else {
			t.Log("Skipping deletion tests due to creation test failures")
		}
	}
}
