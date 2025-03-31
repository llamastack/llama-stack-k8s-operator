package e2e_test

import (
	"os"
	"testing"
)

var testEnv *TestEnvironment
var testOpts *TestOptions

func TestMain(m *testing.M) {
	// Parse test options
	testOpts = ParseFlags()

	// Set up test environment
	var err error
	testEnv, err = SetupTestEnv()
	if err != nil {
		os.Exit(1)
	}

	// Run tests
	code := m.Run()

	// Clean up test environment
	CleanupTestEnv(testEnv)

	os.Exit(code)
}
