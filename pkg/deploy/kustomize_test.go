package deploy_test

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/llamastack/llama-stack-k8s-operator/pkg/deploy"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"sigs.k8s.io/kustomize/kyaml/filesys"
)

func TestRenderKustomize(t *testing.T) {
	// Use Kustomize in memory filesystem, so that we can define
	// our test fixtures in this test rather than relying on real
	// files on disk.
	memFs := filesys.MakeFsInMemory()

	// --- Set up a minimal Kustomize base manifest ---
	baseDir := "manifests/base"
	require.NoError(t, memFs.MkdirAll(baseDir))
	require.NoError(t, memFs.WriteFile(
		filepath.Join(baseDir, "kustomization.yaml"),
		[]byte(`resources:
- deployment.yaml
`),
	))

	expKind := "Deployment"
	expName := "foo"
	require.NoError(t, memFs.WriteFile(
		filepath.Join(baseDir, "deployment.yaml"),
		fmt.Appendf(nil, `apiVersion: apps/v1
kind: %s
metadata:
  name: %s
spec:
  replicas: 1
`, expKind, expName),
	))

	// --- Define an overlay that patches the base ---
	overlayDir := "manifests/overlay"
	require.NoError(t, memFs.MkdirAll(overlayDir))

	require.NoError(t, memFs.WriteFile(
		filepath.Join(overlayDir, "kustomization.yaml"),
		fmt.Appendf(nil, `resources:
- ../base
patches:
- path: replica-patch.yaml
  target:
    kind: %s
    name: %s
`, expKind, expName),
	))

	expReplicas := 3
	require.NoError(t, memFs.WriteFile(
		filepath.Join(overlayDir, "replica-patch.yaml"),
		fmt.Appendf(nil, `apiVersion: apps/v1
kind: %s
metadata:
  name: %s
spec:
  replicas: %d
`, expKind, expName, expReplicas),
	))

	// render all resources in the manifest
	objs, err := deploy.RenderKustomize(memFs, overlayDir)
	require.NoError(t, err)
	require.Len(t, objs, 1, "should render exactly one object")

	// validate deployment
	u := objs[0]
	require.Equal(t, expKind, u.GetKind())
	require.Equal(t, expName, u.GetName())

	// confirm the patch changed replicas to our expected replicas
	rep, found, err := unstructured.NestedInt64(u.Object, "spec", "replicas")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, int64(expReplicas), rep)
}
