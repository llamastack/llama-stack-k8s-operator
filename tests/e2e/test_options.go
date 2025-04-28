package e2e

import (
	"flag"
	"fmt"
	"strings"
)

// TestOptions defines the configuration for running tests.
type TestOptions struct {
	SkipValidation    bool
	SkipCreation      bool
	SkipDeletion      bool
	SkipSCCValidation bool
	Components        string
	OperatorNS        string
}

// String returns a string representation of the test options.
func (o *TestOptions) String() string {
	return fmt.Sprintf("Test Options: skipValidation=%v, skipCreation=%v, skipDeletion=%v, skipSCCValidation=%v, components=%s, operatorNS=%s",
		o.SkipValidation, o.SkipCreation, o.SkipDeletion, o.SkipSCCValidation, o.Components, o.OperatorNS)
}

// ParseFlags parses command line flags and returns TestOptions.
func ParseFlags() *TestOptions {
	opts := &TestOptions{}

	flag.BoolVar(&opts.SkipValidation, "skip-validation", false, "Skip validation test suite")
	flag.BoolVar(&opts.SkipCreation, "skip-creation", false, "Skip creation test suite")
	flag.BoolVar(&opts.SkipDeletion, "skip-deletion", false, "Skip deletion test suite")
	flag.BoolVar(&opts.SkipSCCValidation, "skip-scc-validation", false, "Skip SCC validation")
	flag.StringVar(&opts.Components, "components", "all", "Components to test (all, ollama, etc.)")
	flag.StringVar(&opts.OperatorNS, "operator-ns", "llama-stack", "Namespace where the operator is deployed")
	flag.Parse()

	return opts
}

// ShouldRunComponent checks if a specific component should be tested.
func (o *TestOptions) ShouldRunComponent(component string) bool {
	if o.Components == "all" {
		return true
	}
	components := strings.Split(o.Components, ",")
	for _, c := range components {
		if strings.TrimSpace(c) == component {
			return true
		}
	}
	return false
}

var (
	TestOpts = ParseFlags()
)
