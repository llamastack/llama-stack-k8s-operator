package deploy

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"path/filepath"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/yaml"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/api/types"
	"sigs.k8s.io/kustomize/kyaml/filesys"
	wyaml "sigs.k8s.io/yaml"
)

const yamlBufferSize = 4096

// ApplyKustomizeManifests renders manifests via Kustomize and
// reconciles each resource in the cluster using server-side apply.
// It sets the owner reference on each created object.
func ApplyKustomizeManifests(
	ctx context.Context,
	cli client.Client,
	scheme *runtime.Scheme,
	owner metav1.Object,
	ownerGVK schema.GroupVersionKind,
	fs filesys.FileSystem,
	manifestPath string,
	fieldOwner string,
) error {
	// Render all manifests to Unstructured objects.
	objs, err := RenderKustomize(fs, manifestPath)
	if err != nil {
		return err
	}

	// Filter out the owner object to prevent self-application.
	childObjs := filterOwnerFromObjects(objs, owner, ownerGVK)

	// Use server-side apply for each child object.
	for _, u := range childObjs {
		// Set the controller reference so the object is garbage collected
		// when the owner is deleted, and to establish ownership for the controller.
		if err := ctrl.SetControllerReference(owner, u, scheme); err != nil {
			return fmt.Errorf("failed to set controller reference on %s/%s: %w", u.GetKind(), u.GetName(), err)
		}

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
	// Create a Kustomizer with a fully-featured plugin configuration.
	// This is more robust than MakeDefaultOptions() and ensures that
	// all builtin transformers are enabled.
	pc := types.EnabledPluginConfig(types.BploLoadFromFileSys)
	bopt := &krusty.Options{
		PluginConfig: pc,
	}
	k := krusty.MakeKustomizer(bopt)

	// Produce the composed set of resources from base and overlays.
	resMap, err := k.Run(fs, manifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to run kustomize on %q: %w", manifestPath, err)
	}

	// Serialize to YAML because Kustomize's ResMap does not implement runtime.Object
	// and YAML is a universal interchange format for the Kubernetes decoder.
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

// CopyKustomizeBaseToMemory reads all files from a given path in the source
// filesystem (srcFs) and writes them into the destination filesystem (opFs).
// This is a generic utility for preparing an in-memory Kustomize operation.
func CopyKustomizeBaseToMemory(opFs filesys.FileSystem, srcFs fs.FS, basePath string) error {
	// Create the base directory in the destination filesystem.
	if err := opFs.MkdirAll(basePath); err != nil {
		return fmt.Errorf("failed to create in-memory base dir %s: %w", basePath, err)
	}

	// Read all files from the source directory.
	files, err := fs.ReadDir(srcFs, basePath)
	if err != nil {
		return fmt.Errorf("failed to read source directory %s: %w", basePath, err)
	}

	// Copy each file to the destination filesystem.
	for _, file := range files {
		srcPath := filepath.Join(basePath, file.Name())
		content, err := fs.ReadFile(srcFs, srcPath)
		if err != nil {
			return fmt.Errorf("failed to read source file %s: %w", srcPath, err)
		}

		destPath := filepath.Join(basePath, file.Name())
		if err := opFs.WriteFile(destPath, content); err != nil {
			return fmt.Errorf("failed to write in-memory file %s: %w", destPath, err)
		}
	}
	return nil
}

// AddInstanceToKustomizeFS dynamically prepares a Kustomize build environment in memory.
func AddInstanceToKustomizeFS(instance metav1.Object, instanceGVK schema.GroupVersionKind, fs filesys.FileSystem, basePath string) error {
	// The instance's GroupVersionKind must be set explicitly, as objects from the
	// client cache often lack this, and it's required for a valid manifest.
	robj, ok := instance.(runtime.Object)
	if !ok {
		return errors.New("failed to prepare kustomize inputs: instance is not a runtime.Object")
	}
	robj.GetObjectKind().SetGroupVersionKind(instanceGVK)

	instanceYAML, err := wyaml.Marshal(robj)
	if err != nil {
		return fmt.Errorf("failed to marshal instance to YAML: %w", err)
	}

	// The instance manifest must be written to the virtual filesystem so that
	// Kustomize can discover it as a resource for its transformers.
	instanceFilename := "crd-instance.yaml"
	instancePath := filepath.Join(basePath, instanceFilename)
	if err = fs.WriteFile(instancePath, instanceYAML); err != nil {
		return fmt.Errorf("failed to write instance YAML to in-memory fs: %w", err)
	}

	kustomizationPath := filepath.Join(basePath, "kustomization.yaml")
	kustomizationBytes, err := fs.ReadFile(kustomizationPath)
	if err != nil {
		return fmt.Errorf("failed to read in-memory kustomization.yaml: %w", err)
	}

	// Unmarshal into a generic map to preserve all original fields from the base file.
	var kustomization map[string]any
	if err = wyaml.Unmarshal(kustomizationBytes, &kustomization); err != nil {
		return fmt.Errorf("failed to unmarshal in-memory kustomization.yaml: %w", err)
	}

	// Defend against a minimal kustomization.yaml that may not have a 'resources' key.
	if kustomization["resources"] == nil {
		kustomization["resources"] = []any{}
	}
	resources, ok := kustomization["resources"].([]any)
	if !ok {
		return errors.New("failed to prepare kustomize inputs: kustomization.yaml 'resources' field is not a slice")
	}

	// Inject the instance for 'replacements' and set the namespace to leverage
	// Kustomize's built-in transformers for all generated resources.
	kustomization["resources"] = append(resources, instanceFilename)
	kustomization["namespace"] = instance.GetNamespace()

	updatedKustomizationBytes, err := wyaml.Marshal(&kustomization)
	if err != nil {
		return fmt.Errorf("failed to marshal updated kustomization.yaml: %w", err)
	}

	if err = fs.WriteFile(kustomizationPath, updatedKustomizationBytes); err != nil {
		return fmt.Errorf("failed to write updated kustomization.yaml to in-memory fs: %w", err)
	}
	return nil
}

// filterOwnerFromObjects removes the owner object from a slice of unstructured objects.
// This prevents the operator from trying to apply changes to its own parent resource.
func filterOwnerFromObjects(objs []*unstructured.Unstructured, owner metav1.Object, ownerGVK schema.GroupVersionKind) []*unstructured.Unstructured {
	var childObjs []*unstructured.Unstructured
	for _, obj := range objs {
		// append the object only if it is NOT the owner
		if !(obj.GroupVersionKind() == ownerGVK && obj.GetName() == owner.GetName() && obj.GetNamespace() == owner.GetNamespace()) {
			childObjs = append(childObjs, obj)
		}
	}
	return childObjs
}
