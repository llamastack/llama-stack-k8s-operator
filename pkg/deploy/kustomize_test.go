package deploy_test

import (
	"path/filepath"
	"testing"

	"github.com/llamastack/llama-stack-k8s-operator/pkg/deploy"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/kyaml/filesys"
)

func TestRenderKustomize(t *testing.T) {
	// Use Kustomize in memory filesystem, so that we can define
	// our test fixtures in this test rather than relying on real
	// files on disk.
	memFs := filesys.MakeFsInMemory()

	// Set up a minimal Kustomize base manifest
	baseDir := "manifests/base"
	require.NoError(t, memFs.MkdirAll(baseDir))
	require.NoError(t, memFs.WriteFile(
		filepath.Join(baseDir, "kustomization.yaml"),
		[]byte(`resources:
- deployment.yaml
`),
	))
	require.NoError(t, memFs.WriteFile(
		filepath.Join(baseDir, "deployment.yaml"),
		[]byte(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: foo
spec:
  replicas: 1
`),
	))

	// Define an overlay that patches the base
	overlayDir := "manifests/overlay"
	require.NoError(t, memFs.MkdirAll(overlayDir))
	require.NoError(t, memFs.WriteFile(
		filepath.Join(overlayDir, "kustomization.yaml"),
		[]byte(`resources:
- ../base
patches:
- path: replica-patch.yaml
  target:
    kind: Deployment
    name: foo
`),
	))
	require.NoError(t, memFs.WriteFile(
		filepath.Join(overlayDir, "replica-patch.yaml"),
		[]byte(`apiVersion: apps/v1
kind: Deployment
metadata:
  name: foo
spec:
  replicas: 3
`),
	))

	// --- Build and render the kubernetes api objects ---
	// create a Kustomizer to execute the overlay
	k := krusty.MakeKustomizer(krusty.MakeDefaultOptions())

	// render all resources in the manifest
	objs, err := deploy.RenderKustomize(memFs, k, overlayDir)
	require.NoError(t, err)
	require.Len(t, objs, 1, "should render exactly one object")

	// validate deployment
	u := objs[0]
	require.Equal(t, "Deployment", u.GetKind())
	require.Equal(t, "foo", u.GetName())

	// confirm that the patch changed replicas to 3
	rep, found, err := unstructured.NestedInt64(u.Object, "spec", "replicas")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, int64(3), rep)
}
