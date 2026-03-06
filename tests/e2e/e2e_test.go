//nolint:testpackage
package e2e

import (
	"testing"

	"github.com/llamastack/llama-stack-k8s-operator/api/v1alpha1"
)

func TestE2E(t *testing.T) {
	registerSchemes()
	// Run validation tests
	t.Run("validation", TestValidationSuite)

	// Run combined creation and deletion tests for multiple distributions
	// starter: newer image currently being actively updated
	distributions := []string{"starter"}
	for _, dist := range distributions {
		t.Run("creation-deletion-"+dist, func(t *testing.T) {
			// Set distribution type for this test run
			t.Logf("Testing distribution: %s", dist)
			runCreationDeletionSuiteForDistribution(t, dist)
		})
	}

	// Run v1alpha2-specific creation and deletion tests
	t.Run("v1alpha2-lifecycle", func(t *testing.T) {
		runV1Alpha2CreationDeletionSuite(t)
	})

	// Run conversion round-trip tests
	t.Run("conversion", func(t *testing.T) {
		TestConversionSuite(t)
	})

	// Run webhook validation tests
	t.Run("webhook-validation", func(t *testing.T) {
		TestWebhookValidationSuite(t)
	})

	// Run TLS tests
	t.Run("tls", func(t *testing.T) {
		TestTLSSuite(t)
	})
}

// runCreationDeletionSuiteForDistribution runs creation tests followed by deletion tests for a specific distribution.
func runCreationDeletionSuiteForDistribution(t *testing.T, distType string) {
	t.Helper()
	if TestOpts.SkipCreation {
		t.Skip("Skipping creation-deletion test suite")
	}

	var creationFailed bool
	var createdDistribution *v1alpha1.LlamaStackDistribution

	// Run all creation tests
	t.Run("creation", func(t *testing.T) {
		createdDistribution = runCreationTestsForDistribution(t, distType)
		creationFailed = t.Failed()
	})

	// Run deletion tests only if creation passed
	if !creationFailed && !TestOpts.SkipDeletion && createdDistribution != nil {
		t.Run("deletion", func(t *testing.T) {
			runDeletionTests(t, createdDistribution)
		})
	} else {
		if TestOpts.SkipDeletion {
			t.Log("Skipping deletion tests (SkipDeletion=true)")
		} else {
			t.Log("Skipping deletion tests due to creation test failures")
		}
	}
}
