package e2e

import (
	"testing"
)

func TestE2E(t *testing.T) {
	// Run validation tests
	t.Run("validation", TestValidationSuite)

	// Run creation tests
	t.Run("creation", TestCreationSuite)

	// Run deletion tests if not skipped
	if !testOpts.skipDeletion {
		t.Run("deletion", TestDeletionSuite)
	}
}
