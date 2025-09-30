package deploy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"slices"

	llamav1alpha1 "github.com/llamastack/llama-stack-k8s-operator/api/v1alpha1"
	"github.com/llamastack/llama-stack-k8s-operator/pkg/compare"
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
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/kustomize/api/krusty"
	"sigs.k8s.io/kustomize/api/resmap"
	"sigs.k8s.io/kustomize/api/resource"
	"sigs.k8s.io/kustomize/kyaml/filesys"
	yamlpkg "sigs.k8s.io/yaml"
)

// RenderManifest takes a manifest directory and transforms it through
// kustomization and plugins to produce final Kubernetes resources.
func RenderManifest(
	fs filesys.FileSystem,
	manifestPath string,
	ownerInstance *llamav1alpha1.LlamaStackDistribution,
) (*resmap.ResMap, error) {
	// fallback to the 'default' directory' if we cannot initially find
	// the kustomization file
	finalManifestPath := manifestPath
	if exists := fs.Exists(filepath.Join(manifestPath, "kustomization.yaml")); !exists {
		finalManifestPath = filepath.Join(manifestPath, "default")
	}

	k := krusty.MakeKustomizer(krusty.MakeDefaultOptions())

	resMapVal, err := k.Run(fs, finalManifestPath)
	if err != nil {
		return nil, fmt.Errorf("failed to run kustomize: %w", err)
	}
	if err := applyPlugins(&resMapVal, ownerInstance); err != nil {
		return nil, err
	}
	return &resMapVal, nil
}

// ApplyResources takes a Kustomize ResMap and applies the resources to the cluster.
func ApplyResources(
	ctx context.Context,
	cli client.Client,
	scheme *runtime.Scheme,
	ownerInstance *llamav1alpha1.LlamaStackDistribution,
	resMap *resmap.ResMap,
) error {
	for _, res := range (*resMap).Resources() {
		if err := manageResource(ctx, cli, scheme, res, ownerInstance); err != nil {
			return fmt.Errorf("failed to manage resource %s/%s: %w", res.GetKind(), res.GetName(), err)
		}
	}
	return nil
}

// manageResource acts as a dispatcher, checking if a resource exists and then
// deciding whether to create it or patch it.
func manageResource(
	ctx context.Context,
	cli client.Client,
	scheme *runtime.Scheme,
	res *resource.Resource,
	ownerInstance *llamav1alpha1.LlamaStackDistribution,
) error {
	// prevent the controller from trying to apply changes to its own CR
	if res.GetKind() == llamav1alpha1.LlamaStackDistributionKind && res.GetName() == ownerInstance.Name && res.GetNamespace() == ownerInstance.Namespace {
		return nil
	}

	u := &unstructured.Unstructured{}
	if err := yaml.Unmarshal([]byte(res.MustYaml()), u); err != nil {
		return fmt.Errorf("failed to unmarshal resource: %w", err)
	}

	// Check if ClusterRoleBinding references a ClusterRole that exists
	if u.GetKind() == "ClusterRoleBinding" {
		if shouldSkip, err := CheckClusterRoleExists(ctx, cli, u); err != nil {
			return fmt.Errorf("failed to check ClusterRole existence: %w", err)
		} else if shouldSkip {
			log.FromContext(ctx).V(1).Info("Skipping ClusterRoleBinding - referenced ClusterRole not found",
				"clusterRoleBinding", u.GetName())
			return nil
		}
	}

	kGvk := res.GetGvk()
	gvk := schema.GroupVersionKind{
		Group:   kGvk.Group,
		Version: kGvk.Version,
		Kind:    kGvk.Kind,
	}

	found := u.DeepCopy()
	err := cli.Get(ctx, client.ObjectKeyFromObject(u), found)
	if err != nil {
		if !k8serr.IsNotFound(err) {
			return fmt.Errorf("failed to get resource: %w", err)
		}
		return createResource(ctx, cli, u, ownerInstance, scheme, gvk)
	}
	return patchResource(ctx, cli, u, found, ownerInstance)
}

// createResource creates a new resource, setting an owner reference only if it's namespace-scoped.
func createResource(
	ctx context.Context,
	cli client.Client,
	obj *unstructured.Unstructured,
	ownerInstance *llamav1alpha1.LlamaStackDistribution,
	scheme *runtime.Scheme,
	gvk schema.GroupVersionKind,
) error {
	// Check if the resource is cluster-scoped (like a ClusterRole) to avoid
	// incorrectly setting a namespace-bound owner reference on it.
	isClusterScoped, err := isClusterScoped(cli.RESTMapper(), gvk)
	if err != nil {
		return fmt.Errorf("failed to determine resource scope: %w", err)
	}
	if !isClusterScoped {
		if err := ctrl.SetControllerReference(ownerInstance, obj, scheme); err != nil {
			return fmt.Errorf("failed to set controller reference for %s: %w", gvk.Kind, err)
		}
	}
	return cli.Create(ctx, obj)
}

// isClusterScoped checks if a given GVK refers to a cluster-scoped resource.
func isClusterScoped(mapper meta.RESTMapper, gvk schema.GroupVersionKind) (bool, error) {
	mapping, err := mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return false, fmt.Errorf("failed to get REST mapping for GVK %v: %w", gvk, err)
	}
	return mapping.Scope.Name() == meta.RESTScopeNameRoot, nil
}

// patchResource patches an existing resource, but only if we own it.
func patchResource(ctx context.Context, cli client.Client, desired, existing *unstructured.Unstructured, ownerInstance *llamav1alpha1.LlamaStackDistribution) error {
	logger := log.FromContext(ctx)

	// Critical safety check to prevent the operator from "stealing" or
	// overwriting a resource that was created by another user or controller.
	isOwner := false
	for _, ref := range existing.GetOwnerReferences() {
		if ref.UID == ownerInstance.GetUID() {
			isOwner = true
			break
		}
	}
	if !isOwner {
		logger.Info("Skipping resource not owned by this instance",
			"kind", existing.GetKind(),
			"name", existing.GetName(),
			"namespace", existing.GetNamespace())
		return nil
	}

	if existing.GetKind() == "PersistentVolumeClaim" {
		logger.Info("Skipping PVC patch - PVCs are immutable after creation",
			"name", existing.GetName(),
			"namespace", existing.GetNamespace())
		return nil
	} else if existing.GetKind() == "Service" {
		if err := compare.CheckAndLogServiceChanges(ctx, cli, desired); err != nil {
			return fmt.Errorf("failed to validate resource mutations while patching: %w", err)
		}
	}

	data, err := json.Marshal(desired)
	if err != nil {
		return fmt.Errorf("failed to marshal desired state: %w", err)
	}

	return cli.Patch(
		ctx,
		existing,
		client.RawPatch(k8stypes.ApplyPatchType, data),
		client.ForceOwnership,
		client.FieldOwner(ownerInstance.GetName()),
	)
}

// applyPlugins runs all Go-based transformations on the resource map.
func applyPlugins(resMap *resmap.ResMap, ownerInstance *llamav1alpha1.LlamaStackDistribution) error {
	namePrefixPlugin := plugins.CreateNamePrefixPlugin(plugins.NamePrefixConfig{
		Prefix: ownerInstance.GetName(),
		// Exclude Deployment to maintain backward compatibility with existing deployment names
		ExcludeKinds: []string{"Deployment"},
	})
	if err := namePrefixPlugin.Transform(*resMap); err != nil {
		return fmt.Errorf("failed to apply name prefix: %w", err)
	}

	namespaceSetterPlugin, err := plugins.CreateNamespacePlugin(ownerInstance.GetNamespace())
	if err != nil {
		return err
	}
	if err := namespaceSetterPlugin.Transform(*resMap); err != nil {
		return fmt.Errorf("failed to apply namespace setter plugin: %w", err)
	}

	fieldTransformerPlugin := plugins.CreateFieldMutator(plugins.FieldMutatorConfig{
		Mappings: getFieldMappings(ownerInstance),
	})
	if err := fieldTransformerPlugin.Transform(*resMap); err != nil {
		return fmt.Errorf("failed to apply field transformer: %w", err)
	}

	return nil
}

// getFieldMappings returns essential field mappings for kustomize transformation.
func getFieldMappings(ownerInstance *llamav1alpha1.LlamaStackDistribution) []plugins.FieldMapping {
	return []plugins.FieldMapping{
		// PVC storage size
		{
			SourceValue:       getStorageSize(ownerInstance),
			DefaultValue:      llamav1alpha1.DefaultStorageSize.String(),
			TargetField:       "/spec/resources/requests/storage",
			TargetKind:        "PersistentVolumeClaim",
			CreateIfNotExists: true,
		},
		// Service ports
		{
			SourceValue:       getServicePort(ownerInstance),
			DefaultValue:      llamav1alpha1.DefaultServerPort,
			TargetField:       "/spec/ports/0/port",
			TargetKind:        "Service",
			CreateIfNotExists: true,
		},
		{
			SourceValue:       getServicePort(ownerInstance),
			DefaultValue:      llamav1alpha1.DefaultServerPort,
			TargetField:       "/spec/ports/0/targetPort",
			TargetKind:        "Service",
			CreateIfNotExists: true,
		},
		// Instance labels for all resources
		{
			SourceValue:       ownerInstance.GetName(),
			TargetField:       "/spec/selector/app.kubernetes.io~1instance",
			TargetKind:        "Service",
			CreateIfNotExists: true,
		},
		{
			SourceValue:       ownerInstance.GetName(),
			TargetField:       "/spec/podSelector/matchLabels/app.kubernetes.io~1instance",
			TargetKind:        "NetworkPolicy",
			CreateIfNotExists: true,
		},
		// Deployment name (for backward compatibility)
		{
			SourceValue:       ownerInstance.GetName(),
			TargetField:       "/metadata/name",
			TargetKind:        "Deployment",
			CreateIfNotExists: true,
		},
		// Deployment basic fields
		{
			SourceValue:       ownerInstance.GetName(),
			TargetField:       "/spec/selector/matchLabels/app.kubernetes.io~1instance",
			TargetKind:        "Deployment",
			CreateIfNotExists: true,
		},
		{
			SourceValue:       ownerInstance.GetName(),
			TargetField:       "/spec/template/metadata/labels/app.kubernetes.io~1instance",
			TargetKind:        "Deployment",
			CreateIfNotExists: true,
		},
		{
			SourceValue:       ownerInstance.Spec.Replicas,
			TargetField:       "/spec/replicas",
			TargetKind:        "Deployment",
			CreateIfNotExists: true,
		},
		// NetworkPolicy ports (both rules)
		{
			SourceValue:       getServicePort(ownerInstance),
			DefaultValue:      llamav1alpha1.DefaultServerPort,
			TargetField:       "/spec/ingress/0/ports/0/port",
			TargetKind:        "NetworkPolicy",
			CreateIfNotExists: true,
		},
		{
			SourceValue:       getServicePort(ownerInstance),
			DefaultValue:      llamav1alpha1.DefaultServerPort,
			TargetField:       "/spec/ingress/1/ports/0/port",
			TargetKind:        "NetworkPolicy",
			CreateIfNotExists: true,
		},
		// NetworkPolicy operator namespace
		{
			SourceValue:       getOperatorNamespace(),
			DefaultValue:      "llama-stack-k8s-operator-system",
			TargetField:       "/spec/ingress/1/from/0/namespaceSelector/matchLabels/kubernetes.io~1metadata.name",
			TargetKind:        "NetworkPolicy",
			CreateIfNotExists: true,
		},
		// ClusterRoleBinding namespace for ServiceAccount
		{
			SourceValue:       ownerInstance.Namespace,
			TargetField:       "/subjects/0/namespace",
			TargetKind:        "ClusterRoleBinding",
			CreateIfNotExists: true,
		},
	}
}

// getStorageSize extracts the storage size from the CR spec.
func getStorageSize(instance *llamav1alpha1.LlamaStackDistribution) string {
	if instance.Spec.Server.Storage != nil && instance.Spec.Server.Storage.Size != nil {
		return instance.Spec.Server.Storage.Size.String()
	}
	// Returning an empty string signals the field transformer to use the default value.
	return ""
}

// getServicePort returns the service port or nil if not specified.
func getServicePort(instance *llamav1alpha1.LlamaStackDistribution) any {
	if instance.Spec.Server.ContainerSpec.Port != 0 {
		return instance.Spec.Server.ContainerSpec.Port
	}
	// Returning nil signals the field transformer to use the default value.
	return nil
}

// getOperatorNamespace returns the operator namespace or empty string if not available.
func getOperatorNamespace() string {
	if ns, err := GetOperatorNamespace(); err == nil {
		return ns
	}
	// Returning empty string signals the field transformer to use the default value.
	return ""
}

// ManifestContext provides the necessary context for complex resource rendering.
type ManifestContext struct {
	ResolvedImage string
	ConfigMapHash string
	CABundleHash  string
	ContainerSpec map[string]any
	PodSpec       map[string]any
}

// RenderManifestWithContext renders manifests and enhances the Deployment with complex specs.
func RenderManifestWithContext(
	fs filesys.FileSystem,
	manifestsPath string,
	ownerInstance *llamav1alpha1.LlamaStackDistribution,
	manifestCtx *ManifestContext,
) (*resmap.ResMap, error) {
	// First, render the base manifests
	resMap, err := RenderManifest(fs, manifestsPath, ownerInstance)
	if err != nil {
		return nil, fmt.Errorf("failed to render base manifests: %w", err)
	}

	// If no manifest context provided, return base manifests
	if manifestCtx == nil {
		return resMap, nil
	}

	// Update the Deployment with the manifest context
	for _, res := range (*resMap).Resources() {
		if res.GetKind() != "Deployment" {
			continue
		}

		if err := updateDeploymentSpec(res, manifestCtx); err != nil {
			return nil, fmt.Errorf("failed to update Deployment: %w", err)
		}
	}

	return resMap, nil
}

// updateDeploymentSpec updates the Deployment spec with the manifest context.
func updateDeploymentSpec(res *resource.Resource, manifestCtx *ManifestContext) error {
	// Parse the deployment YAML
	data, err := parseDeploymentYAML(res)
	if err != nil {
		return err
	}

	// Navigate to template spec
	templateSpec, err := getDeploymentTemplateSpec(data)
	if err != nil {
		return err
	}

	// Apply pod spec enhancements
	if manifestCtx.PodSpec != nil {
		for key, value := range manifestCtx.PodSpec {
			templateSpec[key] = value
		}
	}

	// Add ConfigMap hash annotations
	if err := addConfigMapAnnotations(data, manifestCtx); err != nil {
		return err
	}

	// Update the resource with the manifest context
	return updateResourceFromData(res, data)
}

// parseDeploymentYAML parses the deployment resource YAML into a map.
func parseDeploymentYAML(res *resource.Resource) (map[string]any, error) {
	yamlBytes, err := res.AsYAML()
	if err != nil {
		return nil, fmt.Errorf("failed to get YAML: %w", err)
	}

	var data map[string]any
	if unmarshalErr := yamlpkg.Unmarshal(yamlBytes, &data); unmarshalErr != nil {
		return nil, fmt.Errorf("failed to unmarshal YAML: %w", unmarshalErr)
	}

	return data, nil
}

// getDeploymentTemplateSpec navigates to the deployment template spec.
func getDeploymentTemplateSpec(data map[string]any) (map[string]any, error) {
	spec, ok := data["spec"].(map[string]any)
	if !ok {
		return nil, errors.New("failed to find deployment spec")
	}

	template, ok := spec["template"].(map[string]any)
	if !ok {
		return nil, errors.New("failed to find deployment template")
	}

	templateSpec, ok := template["spec"].(map[string]any)
	if !ok {
		return nil, errors.New("failed to find deployment template spec")
	}

	return templateSpec, nil
}

// addConfigMapAnnotations adds ConfigMap hash annotations to the deployment template.
func addConfigMapAnnotations(data map[string]any, manifestCtx *ManifestContext) error {
	spec, ok := data["spec"].(map[string]any)
	if !ok {
		return errors.New("failed to find deployment spec in data")
	}

	template, ok := spec["template"].(map[string]any)
	if !ok {
		return errors.New("failed to find deployment template in spec")
	}

	templateMeta, ok := template["metadata"].(map[string]any)
	if !ok {
		templateMeta = make(map[string]any)
		template["metadata"] = templateMeta
	}

	annotations, ok := templateMeta["annotations"].(map[string]any)
	if !ok {
		annotations = make(map[string]any)
		templateMeta["annotations"] = annotations
	}

	if manifestCtx.ConfigMapHash != "" {
		annotations["configmap.hash/user-config"] = manifestCtx.ConfigMapHash
	}
	if manifestCtx.CABundleHash != "" {
		annotations["configmap.hash/ca-bundle"] = manifestCtx.CABundleHash
	}

	return nil
}

// updateResourceFromData updates the resource with the modified data.
func updateResourceFromData(res *resource.Resource, data map[string]any) error {
	updatedJSON, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal updated data: %w", err)
	}

	updatedYAML, err := yamlpkg.JSONToYAML(updatedJSON)
	if err != nil {
		return fmt.Errorf("failed to convert JSON to YAML: %w", err)
	}

	rf := resource.NewFactory(nil)
	newRes, err := rf.FromBytes(updatedYAML)
	if err != nil {
		return fmt.Errorf("failed to create resource from updated YAML: %w", err)
	}

	res.ResetRNode(newRes)
	return nil
}

func FilterExcludeKinds(resMap *resmap.ResMap, kindsToExclude []string) (*resmap.ResMap, error) {
	filteredResMap := resmap.New()
	for _, res := range (*resMap).Resources() {
		if !slices.Contains(kindsToExclude, res.GetKind()) {
			if err := filteredResMap.Append(res); err != nil {
				return nil, fmt.Errorf("failed to append resource while filtering %s/%s: %w", res.GetKind(), res.GetName(), err)
			}
		}
	}
	return &filteredResMap, nil
}

// CheckClusterRoleExists checks if a ClusterRoleBinding should be skipped due to missing ClusterRole.
func CheckClusterRoleExists(ctx context.Context, cli client.Client, crb *unstructured.Unstructured) (bool, error) {
	roleRef, found, _ := unstructured.NestedMap(crb.Object, "roleRef")
	if !found {
		return false, nil // No roleRef, don't skip
	}

	roleName, _, _ := unstructured.NestedString(roleRef, "name")
	if roleName == "" {
		return false, nil // Empty roleName, don't skip
	}

	// Check if the referenced ClusterRole exists
	clusterRole := &unstructured.Unstructured{}
	clusterRole.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "rbac.authorization.k8s.io",
		Version: "v1",
		Kind:    "ClusterRole",
	})
	clusterRole.SetName(roleName)

	err := cli.Get(ctx, client.ObjectKey{Name: roleName}, clusterRole)
	if err != nil && k8serr.IsNotFound(err) {
		return true, nil
	} else if err != nil {
		return false, err
	}
	return false, nil
}
