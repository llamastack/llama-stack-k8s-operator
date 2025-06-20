package plugins

import (
	"errors"
	"fmt"

	"sigs.k8s.io/kustomize/api/resmap"
)

// CreateNamespacePlugin creates a new namespace plugin.
func CreateNamespacePlugin(namespace string) *namespacePlugin {
	return &namespacePlugin{
		namespace: namespace,
	}
}

type namespacePlugin struct {
	namespace string
}

// Config implements the TransformerPlugin interface.
func (p *namespacePlugin) Config(h *resmap.PluginHelpers, config []byte) error {
	return nil
}

// Transform implements the TransformerPlugin interface.
func (p *namespacePlugin) Transform(m resmap.ResMap) error {
	// do not transform an invalid namespace
	if p.namespace == "" {
		return errors.New("failed to set namespace: namespace cannot be empty")
	}
	for _, res := range m.Resources() {
		// skip cluster-scoped resources because they don't have a namespace
		if res.GetGvk().IsClusterScoped() {
			continue
		}
		if err := res.SetNamespace(p.namespace); err != nil {
			return fmt.Errorf("failed to set namespace for resource %s/%s: %w", res.GetKind(), res.GetName(), err)
		}
	}
	return nil
}
