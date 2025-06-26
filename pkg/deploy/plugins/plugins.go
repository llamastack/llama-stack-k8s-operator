package plugins

import (
	"fmt"
	"strings"

	k8svalidation "k8s.io/apimachinery/pkg/util/validation"
)

// ValidateName uses the strictest naming rule (for Namespaces) to ensure
// the provided name string is valid for any given Resource.
func ValidateName(name string) error {
	if errs := k8svalidation.IsDNS1123Label(name); len(errs) > 0 {
		return fmt.Errorf("failed to validate name %q: %s", name, strings.Join(errs, ", "))
	}
	return nil
}
