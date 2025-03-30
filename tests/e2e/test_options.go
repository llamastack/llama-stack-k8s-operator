package e2e

import (
	"flag"
	"fmt"
	"strings"
)

// TestOptions defines the configuration for running tests
type TestOptions struct {
	skipValidation    bool
	skipCreation      bool
	skipDeletion      bool
	skipSCCValidation bool
	components        string
	operatorNS        string
}

// String returns a string representation of the test options
func (o *TestOptions) String() string {
	return fmt.Sprintf("Test Options: skipValidation=%v, skipCreation=%v, skipDeletion=%v, skipSCCValidation=%v, components=%s, operatorNS=%s",
		o.skipValidation, o.skipCreation, o.skipDeletion, o.skipSCCValidation, o.components, o.operatorNS)
}

// ParseFlags parses command line flags and returns TestOptions
func ParseFlags() *TestOptions {
	opts := &TestOptions{}

	flag.BoolVar(&opts.skipValidation, "skip-validation", false, "Skip validation test suite")
	flag.BoolVar(&opts.skipCreation, "skip-creation", false, "Skip creation test suite")
	flag.BoolVar(&opts.skipDeletion, "skip-deletion", false, "Skip deletion test suite")
	flag.BoolVar(&opts.skipSCCValidation, "skip-scc-validation", false, "Skip SCC validation")
	flag.StringVar(&opts.components, "components", "all", "Components to test (all, ollama, etc.)")
	flag.StringVar(&opts.operatorNS, "operator-ns", "llama-stack", "Namespace where the operator is deployed")
	flag.Parse()

	return opts
}

// ShouldRunComponent checks if a specific component should be tested
func (o *TestOptions) ShouldRunComponent(component string) bool {
	if o.components == "all" {
		return true
	}
	components := strings.Split(o.components, ",")
	for _, c := range components {
		if strings.TrimSpace(c) == component {
			return true
		}
	}
	return false
}
