//nolint:testpackage
package e2e

import (
	"testing"
)

func TestE2E(t *testing.T) {
	registerSchemes()
	// Run validation tests
	t.Run("validation", TestValidationSuite)

	// // Run creation tests
	t.Run("creation", TestCreationSuite)

	// Run deletion tests if not skipped
	t.Run("deletion", TestDeletionSuite)
}
