package deploy

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/yaml"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/kyaml/filesys"
)

const yamlBufferSize = 4096

// ApplyKustomizeManifests renders manifests via Kustomize and
// reconciles each resource in the cluster using server-side apply.
// Accepts a customizable file system and Kustomizer to support in-memory testing.
func ApplyKustomizeManifests(
	ctx context.Context,
	cli client.Client,
	scheme *runtime.Scheme,
	fs filesys.FileSystem,
	manifestPath string,
	fieldOwner string,
) error {
	// Render all manifests to Unstructured objects.
	objs, err := RenderKustomize(fs, manifestPath)
	if err != nil {
		return err
	}

	// Use server-side apply for each object so the API server
	// manages field ownership and merging of concurrent updates.
	for _, u := range objs {
		if err := cli.Patch(ctx, u, client.Apply, client.FieldOwner(fieldOwner)); err != nil {
			return fmt.Errorf("failed to patch %s/%s: %w", u.GetKind(), u.GetName(), err)
		}
	}
	return nil
}

// RenderKustomize reads manifests from the given file system at path,
// runs the full Kustomize pipeline (bases, overlays, patches, vars, etc.),
// and returns a list of Unstructured objects ready for reconciliation.
// Splitting rendering into its own function enables fast, in-memory testing
// of manifest composition without involving the cluster client.
func RenderKustomize(
	fs filesys.FileSystem,
	manifestPath string,
) ([]*unstructured.Unstructured, error) {
	// --- Build and render the kubernetes api objects ---
	// create a Kustomizer to execute the overlay
	k := krusty.MakeKustomizer(krusty.MakeDefaultOptions())

	// Produce the composed set of resources from base and overlays.
	resMap, err := k.Run(fs, manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to run kustomize on %q: %w", manifestPath, err)
	}

	// Serialize to YAML because Kustomize's ResMap does not implement runtime.Object;
	// YAML is a universal interchange format for the Kubernetes decoder.
	yamlDocs, err := resMap.AsYaml()
	if err != nil {
		return nil, fmt.Errorf("failed to serialize kustomize output: %w", err)
	}

	// Convert the YAML documents into Unstructured types so the controller-runtime
	// client can apply them generically without requiring typed structs.
	objs, err := decodeToUnstructured(yamlDocs)
	if err != nil {
		return nil, fmt.Errorf("failed to decode resources: %w", err)
	}
	return objs, nil
}

// DecodeToUnstructured transforms the multi-document YAML output from Kustomize
// into a slice of Unstructured objects so they can be consumed directly by the
// controller-runtime client. We use a streaming decoder to avoid buffering large
// manifests in memory, and explicitly set each object's GroupVersionKind
// for correct routing of dynamic client operations.
func decodeToUnstructured(yamlDocs []byte) ([]*unstructured.Unstructured, error) {
	reader := bytes.NewReader(yamlDocs)
	// Streaming decoder allows incremental parsing of each document.
	dec := yaml.NewYAMLOrJSONDecoder(reader, yamlBufferSize)

	var objs []*unstructured.Unstructured
	for {
		u := &unstructured.Unstructured{}
		if err := dec.Decode(u); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, fmt.Errorf("failed to decode YAML to Unstructured: %w", err)
		}

		// Parse apiVersion into Group and Version for setting GVK.
		gv, err := schema.ParseGroupVersion(u.GetAPIVersion())
		if err != nil {
			return nil, fmt.Errorf("failed to parse apiVersion %q: %w", u.GetAPIVersion(), err)
		}
		u.SetGroupVersionKind(schema.GroupVersionKind{
			Group:   gv.Group,
			Version: gv.Version,
			Kind:    u.GetKind(),
		})

		objs = append(objs, u)
	}
	return objs, nil
}
