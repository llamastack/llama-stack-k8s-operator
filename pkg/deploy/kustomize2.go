package deploy

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	llamav1alpha1 "github.com/llamastack/llama-stack-k8s-operator/api/v1alpha1"
	"github.com/llamastack/llama-stack-k8s-operator/pkg/deploy/plugins"
	k8serr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8stypes "k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/yaml"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/api/resmap"
	"sigs.k8s.io/kustomize/api/resource"
	"sigs.k8s.io/kustomize/kyaml/filesys"
)

// ApplyKustomizeManifests renders manifests via Kustomize and
// reconciles each resource in the cluster using server-side apply.
// It sets the owner reference on each created object.
func ApplyKustomizeManifests2(
	ctx context.Context,
	cli client.Client,
	scheme *runtime.Scheme,
	ownerInstance *llamav1alpha1.LlamaStackDistribution,
	manifestPath string,
) error {
	// Create Kustomizer with default options
	k := krusty.MakeKustomizer(krusty.MakeDefaultOptions())
	fs := filesys.MakeFsOnDisk()

	// Check for kustomization.yaml or fall back to default overlay
	_, err := os.Stat(filepath.Join(manifestPath, "kustomization.yaml"))
	if err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("failed to check kustomization.yaml: %w", err)
		}
		manifestPath = filepath.Join(manifestPath, "default")
	}

	// Run Kustomize
	resMap, err := k.Run(fs, manifestPath)
	if err != nil {
		return fmt.Errorf("failed to run kustomize: %w", err)
	}

	// Apply all plugins
	if err := applyPlugins(resMap, ownerInstance); err != nil {
		return err
	}

	// Manage each resource
	for _, res := range resMap.Resources() {
		if err := manageResource(ctx, cli, scheme, res, ownerInstance); err != nil {
			return fmt.Errorf("failed to manage resource %s/%s: %w", res.GetKind(), res.GetName(), err)
		}
	}

	return nil
}

// manageResource handles the lifecycle of a single resource
func manageResource(
	ctx context.Context,
	cli client.Client,
	scheme *runtime.Scheme,
	res *resource.Resource,
	ownerInstance *llamav1alpha1.LlamaStackDistribution,
) error {
	// Skip if resource is the owner
	if res.GetKind() == ownerInstance.Kind && res.GetName() == ownerInstance.Name && res.GetNamespace() == ownerInstance.Namespace {
		return nil
	}

	// Convert to unstructured for client operations
	u := &unstructured.Unstructured{}
	if err := yaml.Unmarshal([]byte(res.MustYaml()), u); err != nil {
		return fmt.Errorf("failed to unmarshal resource: %w", err)
	}

	// Check if resource exists
	found := &unstructured.Unstructured{}
	gvk := schema.GroupVersionKind{
		Group:   res.GetGvk().Group,
		Version: res.GetApiVersion(),
		Kind:    res.GetKind(),
	}
	found.SetGroupVersionKind(gvk)
	err := cli.Get(ctx, k8stypes.NamespacedName{Name: res.GetName(), Namespace: ownerInstance.Namespace}, found)
	if err != nil {
		if !k8serr.IsNotFound(err) {
			return fmt.Errorf("failed to get resource: %w", err)
		}
		return createResource(ctx, cli, u, ownerInstance, scheme, gvk)
	}
	return patchResource(ctx, cli, u, found, ownerInstance)
}

// createResource creates a new resource
func createResource(ctx context.Context, cli client.Client, obj *unstructured.Unstructured, ownerInstance *llamav1alpha1.LlamaStackDistribution, scheme *runtime.Scheme, gvk schema.GroupVersionKind) error {
	// For other resources, check if they are cluster-scoped
	isClusterScoped := false
	gvr, err := cli.RESTMapper().RESTMapping(schema.GroupKind{Group: gvk.Group, Kind: gvk.Kind}, gvk.Version)
	if err == nil {
		isClusterScoped = gvr.Scope.Name() == meta.RESTScopeNameRoot
	}

	// Only set owner reference and namespace for namespace-scoped resources
	if !isClusterScoped {
		obj.SetNamespace(ownerInstance.Namespace)
		if err := ctrl.SetControllerReference(ownerInstance, obj, scheme); err != nil {
			return fmt.Errorf("failed to set controller reference for %s: %w", gvk.Kind, err)
		}
	}

	return cli.Create(ctx, obj)
}

// patchResource patches an existing resource using server-side apply
func patchResource(ctx context.Context, cli client.Client, desired, existing *unstructured.Unstructured, ownerInstance *llamav1alpha1.LlamaStackDistribution) error {
	// Check if existing resource has the same owner
	existingOwnerRefs := existing.GetOwnerReferences()
	hasSameOwner := false
	for _, ref := range existingOwnerRefs {
		if ref.Kind == "LlamaStackDistribution" && ref.Name == ownerInstance.Name {
			hasSameOwner = true
			break
		}
	}
	// Skip if doesn't have same owner
	if !hasSameOwner {
		return fmt.Errorf("resource %s/%s is owned by a different instance", existing.GetKind(), existing.GetName())
	}

	// Marshal the desired state to JSON
	data, err := json.Marshal(desired)
	if err != nil {
		return fmt.Errorf("failed to marshal desired state: %w", err)
	}

	// Apply the patch with force ownership
	return cli.Patch(
		ctx,
		existing,
		client.RawPatch(k8stypes.ApplyPatchType, data),
		client.ForceOwnership,
		client.FieldOwner(ownerInstance.GetName()),
	)
}

// applyPlugins applies all Kustomize plugins to the resource map
func applyPlugins(resMap resmap.ResMap, ownerInstance *llamav1alpha1.LlamaStackDistribution) error {
	// Apply name prefix plugin
	namePrefixPlugin := plugins.CreateNamePrefixPlugin(plugins.NamePrefixConfig{
		Prefix: ownerInstance.GetName(),
	})
	if err := namePrefixPlugin.Transform(resMap); err != nil {
		return fmt.Errorf("failed to apply name prefix: %w", err)
	}

	// Apply field transformer plugin
	fieldTransformerPlugin := plugins.CreateFieldTransformer(plugins.FieldTransformerConfig{
		Mappings: []plugins.FieldMapping{
			{
				SourceValue:       getStorageSize(ownerInstance),
				DefaultValue:      llamav1alpha1.DefaultStorageSize.String(),
				TargetField:       "spec.resources.requests.storage",
				TargetKind:        "PersistentVolumeClaim",
				CreateIfNotExists: true,
			},
		},
		// Extend this mapping to include all configurable fields in the LlamaStackDistribution spec
	})
	if err := fieldTransformerPlugin.Transform(resMap); err != nil {
		return fmt.Errorf("failed to apply field transformer: %w", err)
	}

	return nil
}

// getStorageSize returns the storage size to use, falling back to the default if not specified
func getStorageSize(instance *llamav1alpha1.LlamaStackDistribution) string {
	if instance.Spec.Server.Storage != nil && instance.Spec.Server.Storage.Size != nil {
		return instance.Spec.Server.Storage.Size.String()
	}
	return "" // Return empty string to trigger default value
}
