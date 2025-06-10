//nolint:testpackage
package e2e

import (
	"testing"
)

func TestE2E(t *testing.T) {
	registerSchemes()
	// Run validation tests
	t.Run("validation", TestValidationSuite)

	// Track if creation tests passed
	creationFailed := false

	// Run creation tests
	t.Run("creation", func(t *testing.T) {
		TestCreationSuite(t)
		creationFailed = t.Failed()
	})

	// Run deletion tests only if creation passed
	if !creationFailed {
		t.Run("deletion", TestDeletionSuite)
	} else {
		t.Log("Skipping deletion tests due to creation test failures")
	}
}
