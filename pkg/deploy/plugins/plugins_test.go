package plugins

import (
	"maps"
	"testing"

	"github.com/stretchr/testify/require"
	"sigs.k8s.io/kustomize/api/resource"
	yaml "sigs.k8s.io/kustomize/kyaml/yaml" // Corrected and aliased import
)

// newTestResource is a helper function to create a kustomize resource for testing.
// It is the central helper for all tests in the 'plugins' package.
func newTestResource(t *testing.T, apiVersion, kind, name, namespace string, content map[string]any) *resource.Resource {
	t.Helper()

	obj := map[string]any{
		"apiVersion": apiVersion,
		"kind":       kind,
		"metadata":   map[string]any{"name": name, "namespace": namespace},
	}

	// Logic to construct the object body based on Kind.
	// This ensures resources like Deployment and ClusterRole are valid.
	switch kind {
	case "Deployment":
		baseDeploymentContent := map[string]any{
			"selector": map[string]any{
				"matchLabels": map[string]any{"app": name},
			},
			"template": map[string]any{
				"metadata": map[string]any{"labels": map[string]any{"app": name}},
				"spec": map[string]any{
					"containers": []any{
						map[string]any{"name": "test-container", "image": "nginx"},
					},
				},
			},
		}
		// Overlay caller's content (e.g., "replicas") onto the base Deployment structure.
		maps.Copy(baseDeploymentContent, content)
		obj["spec"] = baseDeploymentContent
	case "ClusterRole":
		// For ClusterRole, its main content (rules) is at the top level, not under 'spec'.
		maps.Copy(obj, content)
	default:
		// For other simple types (like Service, PVC), assume content is the 'spec'.
		obj["spec"] = content
	}

	yamlBytes, err := yaml.Marshal(obj)
	require.NoError(t, err)

	rf := resource.NewFactory(nil)
	res, err := rf.FromBytes(yamlBytes)
	require.NoError(t, err)

	return res
}
