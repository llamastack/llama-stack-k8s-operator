package plugins

import (
	"sigs.k8s.io/kustomize/api/resmap"
)

// NamePrefixConfig holds configuration for the name prefix plugin
type NamePrefixConfig struct {
	// Prefix to add to resource names
	Prefix string
	// ResourceKinds specifies which resource kinds to apply the prefix to
	// If empty, applies to all resources
	ResourceKinds []string
	// ExcludeKinds specifies which resource kinds to exclude from prefixing
	ExcludeKinds []string
}

// CreateNamePrefixPlugin creates a transformer plugin that adds a prefix to resource names
func CreateNamePrefixPlugin(config NamePrefixConfig) resmap.TransformerPlugin {
	return &namePrefixTransformer{
		config: config,
	}
}

type namePrefixTransformer struct {
	config NamePrefixConfig
}

func (t *namePrefixTransformer) Transform(m resmap.ResMap) error {
	for _, res := range m.Resources() {
		// Skip if resource already has the prefix
		if hasPrefix(res.GetName(), t.config.Prefix+"-") {
			continue
		}

		// Check if we should apply prefix to this resource kind
		if !shouldApplyToKind(res.GetKind(), t.config.ResourceKinds, t.config.ExcludeKinds) {
			continue
		}

		// Add prefix to resource name
		res.SetName(t.config.Prefix + "-" + res.GetName())
	}
	return nil
}

func (t *namePrefixTransformer) Config(h *resmap.PluginHelpers, _ []byte) error {
	return nil
}

// shouldApplyToKind checks if a transformation should be applied to a resource kind
func shouldApplyToKind(kind string, includeKinds, excludeKinds []string) bool {
	// If exclude list is not empty and kind is in it, don't apply
	if len(excludeKinds) > 0 {
		for _, excludeKind := range excludeKinds {
			if kind == excludeKind {
				return false
			}
		}
	}

	// If include list is empty, apply to all kinds
	if len(includeKinds) == 0 {
		return true
	}

	// Check if kind is in include list
	for _, includeKind := range includeKinds {
		if kind == includeKind {
			return true
		}
	}

	return false
}

// hasPrefix checks if a string has a given prefix
func hasPrefix(s, prefix string) bool {
	return len(s) >= len(prefix) && s[:len(prefix)] == prefix
}
