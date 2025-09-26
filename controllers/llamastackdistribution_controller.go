/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"
	llamav1alpha1 "github.com/llamastack/llama-stack-k8s-operator/api/v1alpha1"
	"github.com/llamastack/llama-stack-k8s-operator/pkg/cluster"
	"github.com/llamastack/llama-stack-k8s-operator/pkg/deploy"
	"github.com/llamastack/llama-stack-k8s-operator/pkg/featureflags"
	"gopkg.in/yaml.v3"
	appsv1 "k8s.io/api/apps/v1"
	autoscalingv2 "k8s.io/api/autoscaling/v2"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	policyv1 "k8s.io/api/policy/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/kustomize/kyaml/filesys"
)

const (
	operatorConfigData = "llama-stack-operator-config"
	manifestsBasePath  = "manifests/base"

	// CA Bundle related constants.
	DefaultCABundleKey             = "ca-bundle.crt"
	CABundleVolumeName             = "ca-bundle"
	ManagedCABundleConfigMapSuffix = "-ca-bundle"
	ManagedCABundleKey             = "ca-bundle.crt"
	ManagedCABundleMountPath       = "/etc/ssl/certs/ca-bundle"
	ManagedCABundleFilePath        = "/etc/ssl/certs/ca-bundle/ca-bundle.crt"

	// Security limits for CA bundle processing.
	MaxCABundleSize         = 10 * 1024 * 1024 // 10MB max total size
	MaxCABundleCertificates = 1000             // Maximum number of certificates

	// ODH/RHOAI well-known ConfigMap for trusted CA bundles.
	odhTrustedCABundleConfigMap = "odh-trusted-ca-bundle"
)

// LlamaStackDistributionReconciler reconciles a LlamaStack object.
//
// ConfigMap Watching Feature:
// This reconciler watches for changes to ConfigMaps referenced by LlamaStackDistribution CRs.
// When a ConfigMap's data changes, it automatically triggers reconciliation of the referencing
// LlamaStackDistribution, which recalculates a content-based hash and updates the deployment's
// pod template annotations. This causes Kubernetes to restart the pods with the updated configuration.
//
// Operator ConfigMap Watching Feature:
// This reconciler also watches for changes to the operator configuration ConfigMap. When the operator
// config changes, it triggers reconciliation of all LlamaStackDistribution resources.
type LlamaStackDistributionReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	// Feature flags
	EnableNetworkPolicy bool
	// Cluster info
	ClusterInfo *cluster.ClusterInfo
	httpClient  *http.Client
}

// hasUserConfigMap checks if the instance has a valid UserConfig with ConfigMapName.
// Returns true if configured, false otherwise.
func (r *LlamaStackDistributionReconciler) hasUserConfigMap(instance *llamav1alpha1.LlamaStackDistribution) bool {
	return instance.Spec.Server.UserConfig != nil && instance.Spec.Server.UserConfig.ConfigMapName != ""
}

// getUserConfigMapNamespace returns the resolved ConfigMap namespace.
// If ConfigMapNamespace is specified, it returns that; otherwise, it returns the instance's namespace.
func (r *LlamaStackDistributionReconciler) getUserConfigMapNamespace(instance *llamav1alpha1.LlamaStackDistribution) string {
	if instance.Spec.Server.UserConfig.ConfigMapNamespace != "" {
		return instance.Spec.Server.UserConfig.ConfigMapNamespace
	}
	return instance.Namespace
}

// hasCABundleConfigMap checks if the instance has a valid TLSConfig with CABundle ConfigMapName.
// Returns true if configured, false otherwise.
func (r *LlamaStackDistributionReconciler) hasCABundleConfigMap(instance *llamav1alpha1.LlamaStackDistribution) bool {
	return instance.Spec.Server.TLSConfig != nil && instance.Spec.Server.TLSConfig.CABundle != nil && instance.Spec.Server.TLSConfig.CABundle.ConfigMapName != ""
}

// getCABundleConfigMapNamespace returns the resolved CA bundle ConfigMap namespace.
// If ConfigMapNamespace is specified, it returns that; otherwise, it returns the instance's namespace.
func (r *LlamaStackDistributionReconciler) getCABundleConfigMapNamespace(instance *llamav1alpha1.LlamaStackDistribution) string {
	if instance.Spec.Server.TLSConfig.CABundle.ConfigMapNamespace != "" {
		return instance.Spec.Server.TLSConfig.CABundle.ConfigMapNamespace
	}
	return instance.Namespace
}

// hasValidUserConfig is a standalone helper function to check if a LlamaStackDistribution has valid UserConfig.
// This is used by functions that don't have access to the reconciler receiver.
func hasValidUserConfig(llsd *llamav1alpha1.LlamaStackDistribution) bool {
	return llsd.Spec.Server.UserConfig != nil && llsd.Spec.Server.UserConfig.ConfigMapName != ""
}

// getUserConfigMapNamespaceStandalone returns the resolved ConfigMap namespace without needing a receiver.
func getUserConfigMapNamespaceStandalone(llsd *llamav1alpha1.LlamaStackDistribution) string {
	if llsd.Spec.Server.UserConfig.ConfigMapNamespace != "" {
		return llsd.Spec.Server.UserConfig.ConfigMapNamespace
	}
	return llsd.Namespace
}

// hasValidCABundleConfig is a standalone helper function to check if a LlamaStackDistribution has valid CA bundle config.
// This is used by functions that don't have access to the reconciler receiver.
func hasValidCABundleConfig(llsd *llamav1alpha1.LlamaStackDistribution) bool {
	return llsd.Spec.Server.TLSConfig != nil && llsd.Spec.Server.TLSConfig.CABundle != nil && llsd.Spec.Server.TLSConfig.CABundle.ConfigMapName != ""
}

// getCABundleConfigMapNamespaceStandalone returns the resolved CA bundle ConfigMap namespace without needing a receiver.
func getCABundleConfigMapNamespaceStandalone(llsd *llamav1alpha1.LlamaStackDistribution) string {
	if llsd.Spec.Server.TLSConfig.CABundle.ConfigMapNamespace != "" {
		return llsd.Spec.Server.TLSConfig.CABundle.ConfigMapNamespace
	}
	return llsd.Namespace
}

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// the LlamaStack object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.12.2/pkg/reconcile
func (r *LlamaStackDistributionReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	// Create a logger with request-specific values and store it in the context.
	// This ensures consistent logging across the reconciliation process and its sub-functions.
	// The logger is retrieved from the context in each sub-function that needs it, maintaining
	// the request-specific values throughout the call chain.
	// Always ensure the name of the CR and the namespace are included in the logger.
	logger := log.FromContext(ctx).WithValues("namespace", req.Namespace, "name", req.Name)
	ctx = logr.NewContext(ctx, logger)

	// Fetch the LlamaStack instance
	instance, err := r.fetchInstance(ctx, req.NamespacedName)
	if err != nil {
		return ctrl.Result{}, err
	}

	if instance == nil {
		logger.Info("LlamaStackDistribution resource not found, skipping reconciliation")
		return ctrl.Result{}, nil
	}

	// Reconcile all resources, storing the error for later.
	reconcileErr := r.reconcileResources(ctx, instance)

	// Update the status, passing in any reconciliation error.
	if statusUpdateErr := r.updateStatus(ctx, instance, reconcileErr); statusUpdateErr != nil {
		// Log the status update error, but prioritize the reconciliation error for return.
		logger.Error(statusUpdateErr, "failed to update status")
		if reconcileErr != nil {
			return ctrl.Result{}, reconcileErr
		}
		return ctrl.Result{}, statusUpdateErr
	}

	// If reconciliation failed, return the error to trigger a requeue.
	if reconcileErr != nil {
		return ctrl.Result{}, reconcileErr
	}

	// Check if requeue is needed based on phase
	if instance.Status.Phase == llamav1alpha1.LlamaStackDistributionPhaseInitializing {
		return ctrl.Result{RequeueAfter: 10 * time.Second}, nil
	}

	logger.Info("Successfully reconciled LlamaStackDistribution")
	return ctrl.Result{}, nil
}

// fetchInstance retrieves the LlamaStackDistribution instance.
func (r *LlamaStackDistributionReconciler) fetchInstance(ctx context.Context, namespacedName types.NamespacedName) (*llamav1alpha1.LlamaStackDistribution, error) {
	logger := log.FromContext(ctx)
	instance := &llamav1alpha1.LlamaStackDistribution{}
	if err := r.Get(ctx, namespacedName, instance); err != nil {
		if k8serrors.IsNotFound(err) {
			logger.Info("failed to find LlamaStackDistribution resource")
			return nil, nil
		}
		return nil, fmt.Errorf("failed to fetch LlamaStackDistribution: %w", err)
	}
	return instance, nil
}

// determineKindsToExclude returns a list of resource kinds that should be excluded
// based on the instance specification.
func (r *LlamaStackDistributionReconciler) determineKindsToExclude(instance *llamav1alpha1.LlamaStackDistribution) []string {
	var kinds []string

	// Exclude PersistentVolumeClaim if storage is not configured
	if instance.Spec.Server.Storage == nil {
		kinds = append(kinds, "PersistentVolumeClaim")
	}

	// Exclude NetworkPolicy if the feature is disabled
	if !r.EnableNetworkPolicy {
		kinds = append(kinds, "NetworkPolicy")
	}

	// Exclude Service if no ports are defined
	if !instance.HasPorts() {
		kinds = append(kinds, "Service")
	}

	if !needsPodDisruptionBudget(instance) {
		kinds = append(kinds, "PodDisruptionBudget")
	}

	if instance.Spec.Server.Autoscaling == nil {
		kinds = append(kinds, "HorizontalPodAutoscaler")
	}

	return kinds
}

// reconcileAllManifestResources applies all manifest-based resources using kustomize.
func (r *LlamaStackDistributionReconciler) reconcileAllManifestResources(ctx context.Context, instance *llamav1alpha1.LlamaStackDistribution) error {
	// Build manifest context for Deployment
	manifestCtx, err := r.buildManifestContext(ctx, instance)
	if err != nil {
		return fmt.Errorf("failed to build manifest context: %w", err)
	}

	// Render manifests with context
	resMap, err := deploy.RenderManifestWithContext(filesys.MakeFsOnDisk(), manifestsBasePath, instance, manifestCtx)
	if err != nil {
		return fmt.Errorf("failed to render manifests: %w", err)
	}

	kindsToExclude := r.determineKindsToExclude(instance)
	filteredResMap, err := deploy.FilterExcludeKinds(resMap, kindsToExclude)
	if err != nil {
		return fmt.Errorf("failed to filter manifests: %w", err)
	}

	// Delete excluded resources that might exist from previous reconciliations
	if err := r.deleteExcludedResources(ctx, instance, kindsToExclude); err != nil {
		return fmt.Errorf("failed to delete excluded resources: %w", err)
	}

	// Apply resources to cluster
	if err := deploy.ApplyResources(ctx, r.Client, r.Scheme, instance, filteredResMap); err != nil {
		return fmt.Errorf("failed to apply manifests: %w", err)
	}

	return nil
}

// deleteExcludedResources deletes resources that are excluded from the current reconciliation
// but might exist from previous reconciliations.
func (r *LlamaStackDistributionReconciler) deleteExcludedResources(ctx context.Context, instance *llamav1alpha1.LlamaStackDistribution, kindsToExclude []string) error {
	logger := log.FromContext(ctx)

	if slices.Contains(kindsToExclude, "NetworkPolicy") {
		if err := r.deleteNetworkPolicyIfExists(ctx, instance); err != nil {
			logger.Error(err, "Failed to delete NetworkPolicy")
			return err
		}
	}

	if slices.Contains(kindsToExclude, "PodDisruptionBudget") {
		if err := r.deletePodDisruptionBudgetIfExists(ctx, instance); err != nil {
			logger.Error(err, "Failed to delete PodDisruptionBudget")
			return err
		}
	}

	if slices.Contains(kindsToExclude, "HorizontalPodAutoscaler") {
		if err := r.deleteHorizontalPodAutoscalerIfExists(ctx, instance); err != nil {
			logger.Error(err, "Failed to delete HorizontalPodAutoscaler")
			return err
		}
	}

	return nil
}

// deleteNetworkPolicyIfExists deletes the NetworkPolicy if it exists.
func (r *LlamaStackDistributionReconciler) deleteNetworkPolicyIfExists(ctx context.Context, instance *llamav1alpha1.LlamaStackDistribution) error {
	logger := log.FromContext(ctx)

	networkPolicy := &networkingv1.NetworkPolicy{}
	networkPolicyName := instance.Name + "-network-policy"
	key := types.NamespacedName{Name: networkPolicyName, Namespace: instance.Namespace}

	err := r.Get(ctx, key, networkPolicy)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			// NetworkPolicy doesn't exist, nothing to delete
			return nil
		}
		return fmt.Errorf("failed to get NetworkPolicy: %w", err)
	}

	// Check if this NetworkPolicy is owned by our instance
	if !metav1.IsControlledBy(networkPolicy, instance) {
		logger.V(1).Info("NetworkPolicy not owned by this instance, skipping deletion",
			"networkPolicy", networkPolicyName)
		return nil
	}

	logger.Info("Deleting NetworkPolicy as feature is disabled", "networkPolicy", networkPolicyName)
	if err := r.Delete(ctx, networkPolicy); err != nil {
		return fmt.Errorf("failed to delete NetworkPolicy: %w", err)
	}

	return nil
}

func (r *LlamaStackDistributionReconciler) deletePodDisruptionBudgetIfExists(ctx context.Context, instance *llamav1alpha1.LlamaStackDistribution) error {
	logger := log.FromContext(ctx)

	pdb := &policyv1.PodDisruptionBudget{}
	pdbName := instance.Name + "-pdb"
	key := types.NamespacedName{Name: pdbName, Namespace: instance.Namespace}

	if err := r.Get(ctx, key, pdb); err != nil {
		if k8serrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to get PodDisruptionBudget: %w", err)
	}

	if !metav1.IsControlledBy(pdb, instance) {
		logger.V(1).Info("PodDisruptionBudget not owned by this instance, skipping deletion", "pdb", pdbName)
		return nil
	}

	logger.Info("Deleting PodDisruptionBudget as feature is disabled", "pdb", pdbName)
	if err := r.Delete(ctx, pdb); err != nil {
		return fmt.Errorf("failed to delete PodDisruptionBudget: %w", err)
	}

	return nil
}

func (r *LlamaStackDistributionReconciler) deleteHorizontalPodAutoscalerIfExists(ctx context.Context, instance *llamav1alpha1.LlamaStackDistribution) error {
	logger := log.FromContext(ctx)

	hpa := &autoscalingv2.HorizontalPodAutoscaler{}
	hpaName := instance.Name + "-hpa"
	key := types.NamespacedName{Name: hpaName, Namespace: instance.Namespace}

	if err := r.Get(ctx, key, hpa); err != nil {
		if k8serrors.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to get HorizontalPodAutoscaler: %w", err)
	}

	if !metav1.IsControlledBy(hpa, instance) {
		logger.V(1).Info("HorizontalPodAutoscaler not owned by this instance, skipping deletion", "hpa", hpaName)
		return nil
	}

	logger.Info("Deleting HorizontalPodAutoscaler as feature is disabled", "hpa", hpaName)
	if err := r.Delete(ctx, hpa); err != nil {
		return fmt.Errorf("failed to delete HorizontalPodAutoscaler: %w", err)
	}

	return nil
}

// buildManifestContext creates the manifest context for Deployment using existing helper functions.
func (r *LlamaStackDistributionReconciler) buildManifestContext(ctx context.Context, instance *llamav1alpha1.LlamaStackDistribution) (*deploy.ManifestContext, error) {
	// Validate distribution configuration
	if err := r.validateDistribution(instance); err != nil {
		return nil, err
	}

	resolvedImage, err := r.resolveImage(instance.Spec.Server.Distribution)
	if err != nil {
		return nil, err
	}

	container := buildContainerSpec(ctx, r, instance, resolvedImage)
	podSpec := configurePodStorage(ctx, r, instance, container)

	// Get UserConfigMap hash if needed
	var configMapHash string
	if r.hasUserConfigMap(instance) {
		configMapHash, err = r.getConfigMapHash(ctx, instance)
		if err != nil {
			return nil, fmt.Errorf("failed to get ConfigMap hash: %w", err)
		}
	}

	// Get CA bundle hash if needed
	var caBundleHash string
	if r.hasCABundleConfigMap(instance) {
		caBundleHash, err = r.getCABundleConfigMapHash(ctx, instance)
		if err != nil {
			return nil, fmt.Errorf("failed to get CA bundle ConfigMap hash: %w", err)
		}
	}

	podSpecMap, err := runtime.DefaultUnstructuredConverter.ToUnstructured(&podSpec)
	if err != nil {
		return nil, fmt.Errorf("failed to convert pod spec to map: %w", err)
	}

	pdbSpec := buildPodDisruptionBudgetSpec(instance)
	hpaSpec := buildHPASpec(instance)

	return &deploy.ManifestContext{
		ResolvedImage:           resolvedImage,
		ConfigMapHash:           configMapHash,
		CABundleHash:            caBundleHash,
		PodSpec:                 podSpecMap,
		PodDisruptionBudgetSpec: pdbSpec,
		HPASpec:                 hpaSpec,
	}, nil
}

// reconcileResources reconciles all resources for the LlamaStackDistribution instance.
func (r *LlamaStackDistributionReconciler) reconcileResources(ctx context.Context, instance *llamav1alpha1.LlamaStackDistribution) error {
	// Reconcile ConfigMaps first
	if err := r.reconcileConfigMaps(ctx, instance); err != nil {
		return err
	}

	// Reconcile all manifest-based resources including Deployment: PVC, ServiceAccount, Service, NetworkPolicy, Deployment
	if err := r.reconcileAllManifestResources(ctx, instance); err != nil {
		return err
	}

	return nil
}

func (r *LlamaStackDistributionReconciler) reconcileConfigMaps(ctx context.Context, instance *llamav1alpha1.LlamaStackDistribution) error {
	if err := r.validateCABundleKeys(instance); err != nil {
		return err
	}

	if err := r.reconcileUserAndCABundleConfigMaps(ctx, instance); err != nil {
		return err
	}

	return r.reconcileManagedCABundle(ctx, instance)
}

func (r *LlamaStackDistributionReconciler) validateCABundleKeys(instance *llamav1alpha1.LlamaStackDistribution) error {
	if instance.Spec.Server.TLSConfig == nil || instance.Spec.Server.TLSConfig.CABundle == nil {
		return nil
	}

	if len(instance.Spec.Server.TLSConfig.CABundle.ConfigMapKeys) > 0 {
		if err := validateConfigMapKeys(instance.Spec.Server.TLSConfig.CABundle.ConfigMapKeys); err != nil {
			return fmt.Errorf("failed to validate CA bundle ConfigMap keys: %w", err)
		}
	}

	return nil
}

func (r *LlamaStackDistributionReconciler) reconcileUserAndCABundleConfigMaps(ctx context.Context, instance *llamav1alpha1.LlamaStackDistribution) error {
	if r.hasUserConfigMap(instance) {
		if err := r.reconcileUserConfigMap(ctx, instance); err != nil {
			return fmt.Errorf("failed to reconcile user ConfigMap: %w", err)
		}
	}

	if r.hasCABundleConfigMap(instance) {
		if err := r.reconcileCABundleConfigMap(ctx, instance); err != nil {
			return fmt.Errorf("failed to reconcile CA bundle ConfigMap: %w", err)
		}
	}

	return nil
}

func (r *LlamaStackDistributionReconciler) reconcileManagedCABundle(ctx context.Context, instance *llamav1alpha1.LlamaStackDistribution) error {
	logger := log.FromContext(ctx)
	managedConfigMapName := getManagedCABundleConfigMapName(instance)

	if !r.hasCABundleConfigMap(instance) && !r.hasODHTrustedCABundle(ctx, instance) {
		// No CA bundles configured, delete managed ConfigMap if it exists
		existingConfigMap := &corev1.ConfigMap{}
		err := r.Get(ctx, types.NamespacedName{
			Name:      managedConfigMapName,
			Namespace: instance.Namespace,
		}, existingConfigMap)

		if err == nil {
			// ConfigMap exists but is no longer needed, delete it
			logger.Info("Deleting unused managed CA bundle ConfigMap", "configMap", managedConfigMapName)
			if delErr := r.Delete(ctx, existingConfigMap); delErr != nil && !k8serrors.IsNotFound(delErr) {
				return fmt.Errorf("failed to delete unused managed CA bundle ConfigMap: %w", delErr)
			}
			logger.Info("Successfully deleted unused managed CA bundle ConfigMap", "configMap", managedConfigMapName)
		} else if !k8serrors.IsNotFound(err) {
			// Unexpected error
			return fmt.Errorf("failed to check for managed CA bundle ConfigMap: %w", err)
		}
		return nil
	}

	if err := r.reconcileManagedCABundleConfigMap(ctx, instance); err != nil {
		return fmt.Errorf("failed to reconcile managed CA bundle ConfigMap: %w", err)
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *LlamaStackDistributionReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	// Create a field indexer for ConfigMap references to improve performance
	if err := r.createConfigMapFieldIndexer(ctx, mgr); err != nil {
		return err
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&llamav1alpha1.LlamaStackDistribution{}, builder.WithPredicates(predicate.Funcs{
			UpdateFunc: r.llamaStackUpdatePredicate(mgr),
		})).
		Owns(&appsv1.Deployment{}).
		Owns(&policyv1.PodDisruptionBudget{}).
		Owns(&autoscalingv2.HorizontalPodAutoscaler{}).
		Owns(&corev1.Service{}).
		Owns(&networkingv1.NetworkPolicy{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Watches(
			&corev1.ConfigMap{},
			handler.EnqueueRequestsFromMapFunc(r.findLlamaStackDistributionsForConfigMap),
			builder.WithPredicates(predicate.Funcs{
				UpdateFunc: r.configMapUpdatePredicate,
				CreateFunc: r.configMapCreatePredicate,
				DeleteFunc: r.configMapDeletePredicate,
			}),
		).
		Complete(r)
}

// createConfigMapFieldIndexer creates a field indexer for ConfigMap references.
// On older Kubernetes versions that don't support custom field labels for custom resources,
// this will fail gracefully and the operator will fall back to manual searching.
func (r *LlamaStackDistributionReconciler) createConfigMapFieldIndexer(ctx context.Context, mgr ctrl.Manager) error {
	// Create index for user config ConfigMaps
	if err := mgr.GetFieldIndexer().IndexField(
		ctx,
		&llamav1alpha1.LlamaStackDistribution{},
		"spec.server.userConfig.configMapName",
		r.configMapIndexFunc,
	); err != nil {
		// Log warning but don't fail startup - older Kubernetes versions may not support this
		mgr.GetLogger().V(1).Info("Field indexer for ConfigMap references not supported, will use manual search fallback",
			"error", err.Error())
		return nil
	}

	// Create index for CA bundle ConfigMaps
	if err := mgr.GetFieldIndexer().IndexField(
		ctx,
		&llamav1alpha1.LlamaStackDistribution{},
		"spec.server.tlsConfig.caBundle.configMapName",
		r.caBundleConfigMapIndexFunc,
	); err != nil {
		// Log warning but don't fail startup - older Kubernetes versions may not support this
		mgr.GetLogger().Info("Field indexer for CA bundle ConfigMap references not supported, will use manual search fallback",
			"error", err.Error())
		return nil
	}

	mgr.GetLogger().V(1).Info("Successfully created field indexer for ConfigMap references - will use efficient lookups")
	return nil
}

// configMapIndexFunc is the indexer function for ConfigMap references.
func (r *LlamaStackDistributionReconciler) configMapIndexFunc(rawObj client.Object) []string {
	llsd, ok := rawObj.(*llamav1alpha1.LlamaStackDistribution)
	if !ok {
		return nil
	}
	if !hasValidUserConfig(llsd) {
		return nil
	}

	// Create index key as "namespace/name" format
	configMapNamespace := getUserConfigMapNamespaceStandalone(llsd)
	indexKey := fmt.Sprintf("%s/%s", configMapNamespace, llsd.Spec.Server.UserConfig.ConfigMapName)
	return []string{indexKey}
}

// caBundleConfigMapIndexFunc is the indexer function for CA bundle ConfigMap references.
func (r *LlamaStackDistributionReconciler) caBundleConfigMapIndexFunc(rawObj client.Object) []string {
	llsd, ok := rawObj.(*llamav1alpha1.LlamaStackDistribution)
	if !ok {
		return nil
	}
	if !hasValidCABundleConfig(llsd) {
		return nil
	}

	// Create index key as "namespace/name" format
	configMapNamespace := getCABundleConfigMapNamespaceStandalone(llsd)
	indexKey := fmt.Sprintf("%s/%s", configMapNamespace, llsd.Spec.Server.TLSConfig.CABundle.ConfigMapName)
	return []string{indexKey}
}

// llamaStackUpdatePredicate returns a predicate function for LlamaStackDistribution updates.
func (r *LlamaStackDistributionReconciler) llamaStackUpdatePredicate(mgr ctrl.Manager) func(event.UpdateEvent) bool {
	return func(e event.UpdateEvent) bool {
		// Safely type assert old object
		oldObj, ok := e.ObjectOld.(*llamav1alpha1.LlamaStackDistribution)
		if !ok {
			return false
		}
		oldObjCopy := oldObj.DeepCopy()

		// Safely type assert new object
		newObj, ok := e.ObjectNew.(*llamav1alpha1.LlamaStackDistribution)
		if !ok {
			return false
		}
		newObjCopy := newObj.DeepCopy()

		// Compare only spec, ignoring metadata and status
		if diff := cmp.Diff(oldObjCopy.Spec, newObjCopy.Spec); diff != "" {
			logger := mgr.GetLogger().WithValues("namespace", newObjCopy.Namespace, "name", newObjCopy.Name)
			logger.Info("LlamaStackDistribution CR spec changed")
			// Note that both the logger and fmt.Printf could appear entangled in the output
			// but there is no simple way to avoid this (forcing the logger to flush its output).
			// When the logger is used to print the diff the output is hard to read,
			// fmt.Printf is better for readability.
			fmt.Printf("%s\n", diff)
		}

		return true
	}
}

// configMapUpdatePredicate handles ConfigMap update events.
func (r *LlamaStackDistributionReconciler) configMapUpdatePredicate(e event.UpdateEvent) bool {
	oldConfigMap, oldOk := e.ObjectOld.(*corev1.ConfigMap)
	newConfigMap, newOk := e.ObjectNew.(*corev1.ConfigMap)

	if !oldOk || !newOk {
		return false
	}

	// Parse the feature flags if the operator config ConfigMap has changed
	operatorNamespace, err := deploy.GetOperatorNamespace()
	if err != nil {
		return false
	}
	if newConfigMap.Name == operatorConfigData && newConfigMap.Namespace == operatorNamespace {
		EnableNetworkPolicy, err := parseFeatureFlags(newConfigMap.Data)
		if err != nil {
			log.FromContext(context.Background()).Error(err, "Failed to parse feature flags")
		} else {
			r.EnableNetworkPolicy = EnableNetworkPolicy
		}
		return true
	}

	// Only proceed if this ConfigMap is referenced by any LlamaStackDistribution
	if !r.isConfigMapReferenced(newConfigMap) {
		return false
	}

	// Only trigger if Data or BinaryData has changed
	dataChanged := !cmp.Equal(oldConfigMap.Data, newConfigMap.Data)
	binaryDataChanged := !cmp.Equal(oldConfigMap.BinaryData, newConfigMap.BinaryData)

	// Log ConfigMap changes for debugging (only for referenced ConfigMaps)
	if dataChanged || binaryDataChanged {
		r.logConfigMapDiff(oldConfigMap, newConfigMap, dataChanged, binaryDataChanged)
	}

	return dataChanged || binaryDataChanged
}

// logConfigMapDiff logs the differences between old and new ConfigMaps.
func (r *LlamaStackDistributionReconciler) logConfigMapDiff(oldConfigMap, newConfigMap *corev1.ConfigMap, dataChanged, binaryDataChanged bool) {
	logger := log.FromContext(context.Background()).WithValues(
		"configMapName", newConfigMap.Name,
		"configMapNamespace", newConfigMap.Namespace)

	logger.Info("Referenced ConfigMap change detected")

	if dataChanged {
		if dataDiff := cmp.Diff(oldConfigMap.Data, newConfigMap.Data); dataDiff != "" {
			logger.Info("ConfigMap Data changed")
			fmt.Printf("ConfigMap %s/%s Data diff:\n%s\n", newConfigMap.Namespace, newConfigMap.Name, dataDiff)
		}
	}

	if binaryDataChanged {
		if binaryDataDiff := cmp.Diff(oldConfigMap.BinaryData, newConfigMap.BinaryData); binaryDataDiff != "" {
			logger.Info("ConfigMap BinaryData changed")
			fmt.Printf("ConfigMap %s/%s BinaryData diff:\n%s\n", newConfigMap.Namespace, newConfigMap.Name, binaryDataDiff)
		}
	}
}

// configMapCreatePredicate handles ConfigMap create events.
func (r *LlamaStackDistributionReconciler) configMapCreatePredicate(e event.CreateEvent) bool {
	configMap, ok := e.Object.(*corev1.ConfigMap)
	if !ok {
		return false
	}

	isReferenced := r.isConfigMapReferenced(configMap)
	// Log create events for referenced ConfigMaps
	if isReferenced {
		log.FromContext(context.Background()).Info("ConfigMap create event detected for referenced ConfigMap",
			"configMapName", configMap.Name,
			"configMapNamespace", configMap.Namespace)
	}

	return isReferenced
}

// configMapDeletePredicate handles ConfigMap delete events.
func (r *LlamaStackDistributionReconciler) configMapDeletePredicate(e event.DeleteEvent) bool {
	configMap, ok := e.Object.(*corev1.ConfigMap)
	if !ok {
		return false
	}

	isReferenced := r.isConfigMapReferenced(configMap)
	// Log delete events for referenced ConfigMaps - this is critical for deployment health
	if isReferenced {
		log.FromContext(context.Background()).Error(nil,
			"CRITICAL: ConfigMap delete event detected for referenced ConfigMap - this will break dependent deployments",
			"configMapName", configMap.Name,
			"configMapNamespace", configMap.Namespace)
	}

	return isReferenced
}

// isConfigMapReferenced checks if a ConfigMap is referenced by any LlamaStackDistribution.
func (r *LlamaStackDistributionReconciler) isConfigMapReferenced(configMap client.Object) bool {
	logger := log.FromContext(context.Background()).WithValues(
		"configMapName", configMap.GetName(),
		"configMapNamespace", configMap.GetNamespace())

	// Use field indexer for efficient lookup - create the same index key format
	indexKey := fmt.Sprintf("%s/%s", configMap.GetNamespace(), configMap.GetName())

	// Check for user config ConfigMap references
	userConfigLlamaStacks := llamav1alpha1.LlamaStackDistributionList{}
	err := r.List(context.Background(), &userConfigLlamaStacks, client.MatchingFields{"spec.server.userConfig.configMapName": indexKey})
	if err != nil {
		// Field indexer failed (likely due to older Kubernetes version not supporting custom field labels)
		// Fall back to a manual check instead of assuming all ConfigMaps are referenced
		logger.V(1).Info("Field indexer not supported, falling back to manual ConfigMap reference check", "error", err.Error())
		return r.manuallyCheckConfigMapReference(configMap)
	}

	found := len(userConfigLlamaStacks.Items) > 0

	// Check for CA bundle ConfigMap references if not found in user config
	if !found {
		caBundleLlamaStacks := llamav1alpha1.LlamaStackDistributionList{}
		err := r.List(context.Background(), &caBundleLlamaStacks, client.MatchingFields{"spec.server.tlsConfig.caBundle.configMapName": indexKey})
		if err != nil {
			// Field indexer failed for CA bundle, fall back to manual check
			logger.Info("CA bundle field indexer not supported, falling back to manual ConfigMap reference check", "error", err.Error())
			return r.manuallyCheckConfigMapReference(configMap)
		}
		found = len(caBundleLlamaStacks.Items) > 0
	}

	if !found {
		// Fallback: manually check all LlamaStackDistributions
		manuallyFound := r.manuallyCheckConfigMapReference(configMap)
		if manuallyFound {
			return true
		}
	}

	return found
}

// manuallyCheckConfigMapReference manually checks if any LlamaStackDistribution references the given ConfigMap.
func (r *LlamaStackDistributionReconciler) manuallyCheckConfigMapReference(configMap client.Object) bool {
	logger := log.FromContext(context.Background()).WithValues(
		"configMapName", configMap.GetName(),
		"configMapNamespace", configMap.GetNamespace())

	allLlamaStacks := llamav1alpha1.LlamaStackDistributionList{}
	err := r.List(context.Background(), &allLlamaStacks)
	if err != nil {
		logger.Error(err, "CRITICAL: Failed to list all LlamaStackDistributions for manual ConfigMap reference check - assuming ConfigMap is referenced")
		return true // Return true to trigger reconciliation when we can't determine reference status
	}

	targetNamespace := configMap.GetNamespace()
	targetName := configMap.GetName()

	for _, ls := range allLlamaStacks.Items {
		// Check user config ConfigMap references
		if hasValidUserConfig(&ls) {
			configMapNamespace := getUserConfigMapNamespaceStandalone(&ls)

			if configMapNamespace == targetNamespace && ls.Spec.Server.UserConfig.ConfigMapName == targetName {
				// found a LlamaStackDistribution that references the ConfigMap
				return true
			}
		}

		// Check CA bundle ConfigMap references
		if hasValidCABundleConfig(&ls) {
			configMapNamespace := getCABundleConfigMapNamespaceStandalone(&ls)

			if configMapNamespace == targetNamespace && ls.Spec.Server.TLSConfig.CABundle.ConfigMapName == targetName {
				// found a LlamaStackDistribution that references the CA bundle ConfigMap
				return true
			}
		}
	}

	// no LlamaStackDistribution found that references the ConfigMap
	return false
}

// findLlamaStackDistributionsForConfigMap maps ConfigMap changes to LlamaStackDistribution reconcile requests.
func (r *LlamaStackDistributionReconciler) findLlamaStackDistributionsForConfigMap(ctx context.Context, configMap client.Object) []reconcile.Request {
	logger := log.FromContext(ctx).WithValues(
		"configMapName", configMap.GetName(),
		"configMapNamespace", configMap.GetNamespace())

	operatorNamespace, err := deploy.GetOperatorNamespace()
	if err != nil {
		log.FromContext(context.Background()).Error(err, "Failed to get operator namespace for config map event processing")
		return nil
	}
	// If the operator config was changed, we reconcile all LlamaStackDistributions
	if configMap.GetName() == operatorConfigData && configMap.GetNamespace() == operatorNamespace {
		// List all LlamaStackDistribution resources across all namespaces
		allLlamaStacks := llamav1alpha1.LlamaStackDistributionList{}
		err = r.List(ctx, &allLlamaStacks)
		if err != nil {
			logger.Error(err, "Failed to list all LlamaStackDistributions for operator config change")
			return nil
		}
		return r.convertToReconcileRequests(allLlamaStacks)
	}

	// Try field indexer lookup first
	attachedLlamaStacks, found := r.tryFieldIndexerLookup(ctx, configMap)
	if !found {
		// Fallback to manual search if field indexer returns no results
		attachedLlamaStacks = r.performManualSearch(ctx, configMap)
	}

	// Convert to reconcile requests
	requests := r.convertToReconcileRequests(attachedLlamaStacks)

	return requests
}

// tryFieldIndexerLookup attempts to find LlamaStackDistributions using the field indexer.
func (r *LlamaStackDistributionReconciler) tryFieldIndexerLookup(ctx context.Context, configMap client.Object) (llamav1alpha1.LlamaStackDistributionList, bool) {
	logger := log.FromContext(ctx).WithValues(
		"configMapName", configMap.GetName(),
		"configMapNamespace", configMap.GetNamespace())

	indexKey := fmt.Sprintf("%s/%s", configMap.GetNamespace(), configMap.GetName())

	// Check for user config ConfigMap references
	userConfigLlamaStacks := llamav1alpha1.LlamaStackDistributionList{}
	err := r.List(ctx, &userConfigLlamaStacks, client.MatchingFields{"spec.server.userConfig.configMapName": indexKey})
	if err != nil {
		logger.V(1).Info("Field indexer not supported, will fall back to a manual search for ConfigMap event processing",
			"indexKey", indexKey, "error", err.Error())
		return userConfigLlamaStacks, false
	}

	// Check for CA bundle ConfigMap references
	caBundleLlamaStacks := llamav1alpha1.LlamaStackDistributionList{}
	err = r.List(ctx, &caBundleLlamaStacks, client.MatchingFields{"spec.server.tlsConfig.caBundle.configMapName": indexKey})
	if err != nil {
		logger.Info("CA bundle field indexer not supported, will fall back to a manual search for ConfigMap event processing",
			"indexKey", indexKey, "error", err.Error())
		return userConfigLlamaStacks, len(userConfigLlamaStacks.Items) > 0
	}

	// Combine results from both searches
	combinedLlamaStacks := llamav1alpha1.LlamaStackDistributionList{}
	combinedLlamaStacks.Items = append(combinedLlamaStacks.Items, userConfigLlamaStacks.Items...)
	combinedLlamaStacks.Items = append(combinedLlamaStacks.Items, caBundleLlamaStacks.Items...)

	return combinedLlamaStacks, len(combinedLlamaStacks.Items) > 0
}

// performManualSearch performs a manual search and filtering when field indexer returns no results.
func (r *LlamaStackDistributionReconciler) performManualSearch(ctx context.Context, configMap client.Object) llamav1alpha1.LlamaStackDistributionList {
	logger := log.FromContext(ctx).WithValues(
		"configMapName", configMap.GetName(),
		"configMapNamespace", configMap.GetNamespace())

	allLlamaStacks := llamav1alpha1.LlamaStackDistributionList{}
	err := r.List(ctx, &allLlamaStacks)
	if err != nil {
		logger.Error(err, "CRITICAL: Failed to list all LlamaStackDistributions for manual ConfigMap reference search")
		return allLlamaStacks
	}

	// Filter for ConfigMap references
	filteredItems := r.filterLlamaStacksForConfigMap(allLlamaStacks.Items, configMap)
	allLlamaStacks.Items = filteredItems

	return allLlamaStacks
}

// filterLlamaStacksForConfigMap filters LlamaStackDistributions that reference the given ConfigMap.
func (r *LlamaStackDistributionReconciler) filterLlamaStacksForConfigMap(llamaStacks []llamav1alpha1.LlamaStackDistribution,
	configMap client.Object) []llamav1alpha1.LlamaStackDistribution {
	var filteredItems []llamav1alpha1.LlamaStackDistribution
	targetNamespace := configMap.GetNamespace()
	targetName := configMap.GetName()

	for _, ls := range llamaStacks {
		if r.doesLlamaStackReferenceConfigMap(ls, targetNamespace, targetName) {
			filteredItems = append(filteredItems, ls)
		}
	}

	return filteredItems
}

// doesLlamaStackReferenceConfigMap checks if a LlamaStackDistribution references the specified ConfigMap.
func (r *LlamaStackDistributionReconciler) doesLlamaStackReferenceConfigMap(ls llamav1alpha1.LlamaStackDistribution, targetNamespace, targetName string) bool {
	// Check user config ConfigMap references
	if hasValidUserConfig(&ls) {
		configMapNamespace := getUserConfigMapNamespaceStandalone(&ls)
		if configMapNamespace == targetNamespace && ls.Spec.Server.UserConfig.ConfigMapName == targetName {
			return true
		}
	}

	// Check CA bundle ConfigMap references
	if hasValidCABundleConfig(&ls) {
		configMapNamespace := getCABundleConfigMapNamespaceStandalone(&ls)
		if configMapNamespace == targetNamespace && ls.Spec.Server.TLSConfig.CABundle.ConfigMapName == targetName {
			return true
		}
	}

	return false
}

// convertToReconcileRequests converts LlamaStackDistribution items to reconcile requests.
func (r *LlamaStackDistributionReconciler) convertToReconcileRequests(attachedLlamaStacks llamav1alpha1.LlamaStackDistributionList) []reconcile.Request {
	requests := make([]reconcile.Request, 0, len(attachedLlamaStacks.Items))
	for _, llamaStack := range attachedLlamaStacks.Items {
		requests = append(requests, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      llamaStack.Name,
				Namespace: llamaStack.Namespace,
			},
		})
	}
	return requests
}

// getServerURL returns the URL for the LlamaStack server.
func (r *LlamaStackDistributionReconciler) getServerURL(instance *llamav1alpha1.LlamaStackDistribution, path string) *url.URL {
	serviceName := deploy.GetServiceName(instance)
	port := deploy.GetServicePort(instance)

	return &url.URL{
		Scheme: "http",
		Host:   fmt.Sprintf("%s.%s.svc.cluster.local:%d", serviceName, instance.Namespace, port),
		Path:   path,
	}
}

// getProviderInfo makes an HTTP request to the providers endpoint.
func (r *LlamaStackDistributionReconciler) getProviderInfo(ctx context.Context, instance *llamav1alpha1.LlamaStackDistribution) ([]llamav1alpha1.ProviderInfo, error) {
	u := r.getServerURL(instance, "/v1/providers")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create providers request: %w", err)
	}

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make providers request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to query providers endpoint: returned status code %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read providers response: %w", err)
	}

	var response struct {
		Data []llamav1alpha1.ProviderInfo `json:"data"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal providers response: %w", err)
	}

	return response.Data, nil
}

// getVersionInfo makes an HTTP request to the version endpoint.
func (r *LlamaStackDistributionReconciler) getVersionInfo(ctx context.Context, instance *llamav1alpha1.LlamaStackDistribution) (string, error) {
	u := r.getServerURL(instance, "/v1/version")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return "", fmt.Errorf("failed to create version request: %w", err)
	}

	resp, err := r.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to make version request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to query version endpoint: returned status code %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read version response: %w", err)
	}

	var response struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return "", fmt.Errorf("failed to unmarshal version response: %w", err)
	}

	return response.Version, nil
}

// updateStatus refreshes the LlamaStack status.
func (r *LlamaStackDistributionReconciler) updateStatus(ctx context.Context, instance *llamav1alpha1.LlamaStackDistribution, reconcileErr error) error {
	logger := log.FromContext(ctx)
	instance.Status.Version.OperatorVersion = os.Getenv("OPERATOR_VERSION")
	// A reconciliation error is the highest priority. It overrides all other status checks.
	if reconcileErr != nil {
		instance.Status.Phase = llamav1alpha1.LlamaStackDistributionPhaseFailed
		SetDeploymentReadyCondition(&instance.Status, false, fmt.Sprintf("Resource reconciliation failed: %v", reconcileErr))
	} else {
		// If reconciliation was successful, proceed with detailed status checks.
		deploymentReady, err := r.updateDeploymentStatus(ctx, instance)
		if err != nil {
			return err // Early exit if we can't get deployment status
		}

		r.updateStorageStatus(ctx, instance)
		r.updateServiceStatus(ctx, instance)
		r.updateDistributionConfig(instance)

		if deploymentReady {
			instance.Status.Phase = llamav1alpha1.LlamaStackDistributionPhaseReady

			providers, err := r.getProviderInfo(ctx, instance)
			if err != nil {
				logger.Error(err, "failed to get provider info, clearing provider list")
				instance.Status.DistributionConfig.Providers = nil
			} else {
				instance.Status.DistributionConfig.Providers = providers
			}

			version, err := r.getVersionInfo(ctx, instance)
			if err != nil {
				logger.Error(err, "failed to get version info from API endpoint")
				// Don't clear the version if we cant fetch it - keep the existing one
			} else {
				instance.Status.Version.LlamaStackServerVersion = version
				logger.V(1).Info("Updated LlamaStack version from API endpoint", "version", version)
			}

			SetHealthCheckCondition(&instance.Status, true, MessageHealthCheckPassed)
		} else {
			// If not ready, health can't be checked. Set condition appropriately.
			SetHealthCheckCondition(&instance.Status, false, "Deployment not ready")
			instance.Status.DistributionConfig.Providers = nil // Clear providers
		}
	}

	// Always update the status at the end of the function.
	instance.Status.Version.LastUpdated = metav1.NewTime(metav1.Now().UTC())
	if err := r.Status().Update(ctx, instance); err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}

	return nil
}

func (r *LlamaStackDistributionReconciler) updateDeploymentStatus(ctx context.Context, instance *llamav1alpha1.LlamaStackDistribution) (bool, error) {
	deployment := &appsv1.Deployment{}
	deploymentErr := r.Get(ctx, types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, deployment)
	if deploymentErr != nil && !k8serrors.IsNotFound(deploymentErr) {
		return false, fmt.Errorf("failed to fetch deployment for status: %w", deploymentErr)
	}

	deploymentReady := false

	switch {
	case deploymentErr != nil: // This case covers when the deployment is not found
		instance.Status.Phase = llamav1alpha1.LlamaStackDistributionPhasePending
		SetDeploymentReadyCondition(&instance.Status, false, MessageDeploymentPending)
	case deployment.Status.ReadyReplicas == 0:
		instance.Status.Phase = llamav1alpha1.LlamaStackDistributionPhaseInitializing
		SetDeploymentReadyCondition(&instance.Status, false, MessageDeploymentPending)
	case deployment.Status.ReadyReplicas < instance.Spec.Replicas:
		instance.Status.Phase = llamav1alpha1.LlamaStackDistributionPhaseInitializing
		deploymentMessage := fmt.Sprintf("Deployment is scaling: %d/%d replicas ready", deployment.Status.ReadyReplicas, instance.Spec.Replicas)
		SetDeploymentReadyCondition(&instance.Status, false, deploymentMessage)
	case deployment.Status.ReadyReplicas > instance.Spec.Replicas:
		instance.Status.Phase = llamav1alpha1.LlamaStackDistributionPhaseInitializing
		deploymentMessage := fmt.Sprintf("Deployment is scaling down: %d/%d replicas ready", deployment.Status.ReadyReplicas, instance.Spec.Replicas)
		SetDeploymentReadyCondition(&instance.Status, false, deploymentMessage)
	default:
		instance.Status.Phase = llamav1alpha1.LlamaStackDistributionPhaseReady
		deploymentReady = true
		SetDeploymentReadyCondition(&instance.Status, true, MessageDeploymentReady)
	}
	instance.Status.AvailableReplicas = deployment.Status.ReadyReplicas
	return deploymentReady, nil
}

func (r *LlamaStackDistributionReconciler) updateStorageStatus(ctx context.Context, instance *llamav1alpha1.LlamaStackDistribution) {
	if instance.Spec.Server.Storage == nil {
		return
	}
	pvc := &corev1.PersistentVolumeClaim{}
	err := r.Get(ctx, types.NamespacedName{Name: instance.Name + "-pvc", Namespace: instance.Namespace}, pvc)
	if err != nil {
		SetStorageReadyCondition(&instance.Status, false, fmt.Sprintf("Failed to get PVC: %v", err))
		return
	}

	ready := pvc.Status.Phase == corev1.ClaimBound
	var message string
	if ready {
		message = MessageStorageReady
	} else {
		message = fmt.Sprintf("PVC is not bound: %s", pvc.Status.Phase)
	}
	SetStorageReadyCondition(&instance.Status, ready, message)
}

func (r *LlamaStackDistributionReconciler) updateServiceStatus(ctx context.Context, instance *llamav1alpha1.LlamaStackDistribution) {
	logger := log.FromContext(ctx)
	if !instance.HasPorts() {
		logger.Info("No ports defined, skipping service status update")
		return
	}
	service := &corev1.Service{}
	err := r.Get(ctx, types.NamespacedName{Name: instance.Name + "-service", Namespace: instance.Namespace}, service)
	if err != nil {
		SetServiceReadyCondition(&instance.Status, false, fmt.Sprintf("Failed to get Service: %v", err))
		return
	}

	// Set the service URL in the status
	serviceURL := r.getServerURL(instance, "")
	instance.Status.ServiceURL = serviceURL.String()

	SetServiceReadyCondition(&instance.Status, true, MessageServiceReady)
}

func (r *LlamaStackDistributionReconciler) updateDistributionConfig(instance *llamav1alpha1.LlamaStackDistribution) {
	instance.Status.DistributionConfig.AvailableDistributions = r.ClusterInfo.DistributionImages
	var activeDistribution string
	if instance.Spec.Server.Distribution.Name != "" {
		activeDistribution = instance.Spec.Server.Distribution.Name
	} else if instance.Spec.Server.Distribution.Image != "" {
		activeDistribution = "custom"
	}
	instance.Status.DistributionConfig.ActiveDistribution = activeDistribution
}

// reconcileUserConfigMap validates that the referenced ConfigMap exists.
func (r *LlamaStackDistributionReconciler) reconcileUserConfigMap(ctx context.Context, instance *llamav1alpha1.LlamaStackDistribution) error {
	logger := log.FromContext(ctx)

	if !r.hasUserConfigMap(instance) {
		logger.V(1).Info("No user ConfigMap specified, skipping")
		return nil
	}

	// Determine the ConfigMap namespace - default to the same namespace as the LlamaStackDistribution.
	configMapNamespace := r.getUserConfigMapNamespace(instance)

	logger.V(1).Info("Validating referenced ConfigMap exists",
		"configMapName", instance.Spec.Server.UserConfig.ConfigMapName,
		"configMapNamespace", configMapNamespace)

	// Check if the ConfigMap exists
	configMap := &corev1.ConfigMap{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      instance.Spec.Server.UserConfig.ConfigMapName,
		Namespace: configMapNamespace,
	}, configMap)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			logger.Error(err, "Referenced ConfigMap not found",
				"configMapName", instance.Spec.Server.UserConfig.ConfigMapName,
				"configMapNamespace", configMapNamespace)
			return fmt.Errorf("failed to find referenced ConfigMap %s/%s", configMapNamespace, instance.Spec.Server.UserConfig.ConfigMapName)
		}
		return fmt.Errorf("failed to fetch ConfigMap %s/%s: %w", configMapNamespace, instance.Spec.Server.UserConfig.ConfigMapName, err)
	}

	logger.V(1).Info("User ConfigMap found and validated",
		"configMap", configMap.Name,
		"namespace", configMap.Namespace,
		"dataKeys", len(configMap.Data))
	return nil
}

// reconcileCABundleConfigMap validates that the referenced CA bundle ConfigMap exists.
func (r *LlamaStackDistributionReconciler) reconcileCABundleConfigMap(ctx context.Context, instance *llamav1alpha1.LlamaStackDistribution) error {
	logger := log.FromContext(ctx)

	if !r.hasCABundleConfigMap(instance) {
		logger.V(1).Info("No CA bundle ConfigMap specified, skipping")
		return nil
	}

	// Determine the ConfigMap namespace - default to the same namespace as the LlamaStackDistribution.
	configMapNamespace := r.getCABundleConfigMapNamespace(instance)

	logger.V(1).Info("Validating referenced CA bundle ConfigMap exists",
		"configMapName", instance.Spec.Server.TLSConfig.CABundle.ConfigMapName,
		"configMapNamespace", configMapNamespace)

	// Check if the ConfigMap exists
	configMap := &corev1.ConfigMap{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      instance.Spec.Server.TLSConfig.CABundle.ConfigMapName,
		Namespace: configMapNamespace,
	}, configMap)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			logger.Error(err, "Referenced CA bundle ConfigMap not found",
				"configMapName", instance.Spec.Server.TLSConfig.CABundle.ConfigMapName,
				"configMapNamespace", configMapNamespace)
			return fmt.Errorf("failed to find referenced CA bundle ConfigMap %s/%s", configMapNamespace, instance.Spec.Server.TLSConfig.CABundle.ConfigMapName)
		}
		return fmt.Errorf("failed to fetch CA bundle ConfigMap %s/%s: %w", configMapNamespace, instance.Spec.Server.TLSConfig.CABundle.ConfigMapName, err)
	}

	// Validate that the specified keys exist in the ConfigMap
	var keysToValidate []string
	if len(instance.Spec.Server.TLSConfig.CABundle.ConfigMapKeys) > 0 {
		keysToValidate = instance.Spec.Server.TLSConfig.CABundle.ConfigMapKeys
	} else {
		// Default to DefaultCABundleKey when no keys are specified
		keysToValidate = []string{DefaultCABundleKey}
	}

	for _, key := range keysToValidate {
		if _, exists := configMap.Data[key]; !exists {
			logger.Error(err, "CA bundle key not found in ConfigMap",
				"configMapName", instance.Spec.Server.TLSConfig.CABundle.ConfigMapName,
				"configMapNamespace", configMapNamespace,
				"key", key)
			return fmt.Errorf("failed to find CA bundle key '%s' in ConfigMap %s/%s", key, configMapNamespace, instance.Spec.Server.TLSConfig.CABundle.ConfigMapName)
		}

		// Note: Detailed PEM validation is performed later
		// in extractValidCertificates() which validates all PEM blocks.
		logger.V(1).Info("CA bundle key found",
			"configMapName", instance.Spec.Server.TLSConfig.CABundle.ConfigMapName,
			"configMapNamespace", configMapNamespace,
			"key", key)
	}

	logger.V(1).Info("CA bundle ConfigMap found and validated",
		"configMap", configMap.Name,
		"namespace", configMap.Namespace,
		"keys", keysToValidate,
		"dataKeys", len(configMap.Data))
	return nil
}

// getConfigMapHash calculates a hash of the ConfigMap data to detect changes.
func (r *LlamaStackDistributionReconciler) getConfigMapHash(ctx context.Context, instance *llamav1alpha1.LlamaStackDistribution) (string, error) {
	if !r.hasUserConfigMap(instance) {
		return "", nil
	}

	configMapNamespace := r.getUserConfigMapNamespace(instance)

	configMap := &corev1.ConfigMap{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      instance.Spec.Server.UserConfig.ConfigMapName,
		Namespace: configMapNamespace,
	}, configMap)
	if err != nil {
		return "", err
	}

	// Create a content-based hash that will change when the ConfigMap data changes
	return fmt.Sprintf("%s-%s", configMap.ResourceVersion, configMap.Name), nil
}

// getCABundleConfigMapHash calculates a hash of the managed CA bundle ConfigMap to detect changes.
func (r *LlamaStackDistributionReconciler) getCABundleConfigMapHash(ctx context.Context, instance *llamav1alpha1.LlamaStackDistribution) (string, error) {
	// Check if any CA bundles are configured
	if !r.hasCABundleConfigMap(instance) && !r.hasODHTrustedCABundle(ctx, instance) {
		return "", nil
	}

	// Get the managed ConfigMap
	managedConfigMapName := getManagedCABundleConfigMapName(instance)
	configMap := &corev1.ConfigMap{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      managedConfigMapName,
		Namespace: instance.Namespace,
	}, configMap)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			// ConfigMap doesn't exist yet, return empty hash
			return "", nil
		}
		return "", err
	}

	// Create a content-based hash that will change when the ConfigMap data changes
	return fmt.Sprintf("%s-%s", configMap.ResourceVersion, configMap.Name), nil
}

// hasODHTrustedCABundle checks if the ODH trusted CA bundle ConfigMap exists and has valid keys.
func (r *LlamaStackDistributionReconciler) hasODHTrustedCABundle(ctx context.Context, instance *llamav1alpha1.LlamaStackDistribution) bool {
	_, keys, err := r.detectODHTrustedCABundle(ctx, instance)
	return err == nil && len(keys) > 0
}

// gatherCABundleData collects all CA certificate data from source ConfigMaps and concatenates them.
// This function implements security measures to prevent injection attacks:
// - Validates PEM structure and X.509 certificate format during processing.
// - Enforces size limits to prevent resource exhaustion.
// - Only extracts valid CERTIFICATE blocks using PEM decoder and X.509 parser.
func (r *LlamaStackDistributionReconciler) gatherCABundleData(ctx context.Context, instance *llamav1alpha1.LlamaStackDistribution) (string, error) {
	logger := log.FromContext(ctx)
	collector := &certificateCollector{logger: logger}

	if err := r.gatherExplicitCABundle(ctx, instance, collector); err != nil {
		return "", err
	}

	if err := r.gatherODHCABundle(ctx, instance, collector); err != nil {
		return "", err
	}

	return collector.concatenate()
}

type certificateCollector struct {
	logger           logr.Logger
	certificates     []string
	totalSize        int
	certificateCount int
}

func (c *certificateCollector) add(certs []string, size, count int, configMapName, key string) error {
	c.totalSize += size
	c.certificateCount += count

	if c.totalSize > MaxCABundleSize {
		return fmt.Errorf("failed to process CA bundle: total size exceeds maximum allowed size of %d bytes", MaxCABundleSize)
	}

	if c.certificateCount > MaxCABundleCertificates {
		return fmt.Errorf("failed to process CA bundle: contains more than %d certificates (maximum allowed)", MaxCABundleCertificates)
	}

	c.certificates = append(c.certificates, certs...)
	c.logger.V(1).Info("Processed CA bundle key",
		"configMap", configMapName,
		"key", key,
		"certificates", count,
		"size", size)

	return nil
}

func (c *certificateCollector) concatenate() (string, error) {
	if len(c.certificates) == 0 {
		return "", errors.New("failed to find valid certificates in CA bundle ConfigMaps")
	}

	// Use strings.Builder for efficient memory usage with large bundles
	var builder strings.Builder
	builder.Grow(c.totalSize + len(c.certificates)) // Pre-allocate with space for newlines
	for i, cert := range c.certificates {
		if i > 0 {
			builder.WriteString("\n")
		}
		builder.WriteString(cert)
	}

	concatenated := builder.String()
	c.logger.Info("Successfully gathered CA bundle data",
		"totalCertificates", c.certificateCount,
		"totalSize", len(concatenated))

	return concatenated, nil
}

func (r *LlamaStackDistributionReconciler) gatherExplicitCABundle(ctx context.Context, instance *llamav1alpha1.LlamaStackDistribution, collector *certificateCollector) error {
	if !r.hasCABundleConfigMap(instance) {
		return nil
	}

	configMapNamespace := r.getCABundleConfigMapNamespace(instance)
	configMap := &corev1.ConfigMap{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      instance.Spec.Server.TLSConfig.CABundle.ConfigMapName,
		Namespace: configMapNamespace,
	}, configMap)
	if err != nil {
		return fmt.Errorf("failed to get CA bundle ConfigMap %s/%s: %w",
			configMapNamespace, instance.Spec.Server.TLSConfig.CABundle.ConfigMapName, err)
	}

	keysToProcess := instance.Spec.Server.TLSConfig.CABundle.ConfigMapKeys
	if len(keysToProcess) == 0 {
		keysToProcess = []string{DefaultCABundleKey}
	}

	return r.processConfigMapKeys(configMap, keysToProcess, configMapNamespace, instance.Spec.Server.TLSConfig.CABundle.ConfigMapName, collector)
}

func (r *LlamaStackDistributionReconciler) gatherODHCABundle(ctx context.Context, instance *llamav1alpha1.LlamaStackDistribution, collector *certificateCollector) error {
	configMap, keys, err := r.detectODHTrustedCABundle(ctx, instance)
	if err != nil {
		// Log but don't fail - ODH bundle is optional
		collector.logger.V(1).Info("Could not detect ODH trusted CA bundle", "error", err)
		return nil
	}
	if configMap == nil || len(keys) == 0 {
		return nil
	}

	return r.processODHConfigMapKeys(configMap, keys, collector)
}

func (r *LlamaStackDistributionReconciler) processConfigMapKeys(configMap *corev1.ConfigMap, keys []string, namespace, name string, collector *certificateCollector) error {
	for _, key := range keys {
		data, exists := configMap.Data[key]
		if !exists {
			return fmt.Errorf("failed to find CA bundle key '%s' in ConfigMap %s/%s", key, namespace, name)
		}

		certs, size, count, err := extractValidCertificates([]byte(data), key)
		if err != nil {
			return fmt.Errorf("failed to process CA bundle key '%s' from ConfigMap %s/%s: %w", key, namespace, name, err)
		}

		if err := collector.add(certs, size, count, configMap.Name, key); err != nil {
			return err
		}
	}

	return nil
}

func (r *LlamaStackDistributionReconciler) processODHConfigMapKeys(configMap *corev1.ConfigMap, keys []string, collector *certificateCollector) error {
	for _, key := range keys {
		data, exists := configMap.Data[key]
		if !exists {
			collector.logger.V(1).Info("ODH CA bundle key not found, skipping", "key", key)
			continue
		}

		certs, size, count, err := extractValidCertificates([]byte(data), key)
		if err != nil {
			collector.logger.Error(err, "Failed to process ODH CA bundle key, skipping",
				"configMap", configMap.Name,
				"key", key)
			continue
		}

		if err := collector.add(certs, size, count, configMap.Name, key); err != nil {
			return err
		}
	}

	return nil
}

// extractValidCertificates extracts only valid CERTIFICATE blocks from PEM data.
// This function validates PEM structure and X.509 certificate format for all blocks.
// It filters out non-certificate PEM blocks (e.g., private keys, public keys) and
// rejects invalid X.509 certificates.
// Returns: (certificates as strings, total size, certificate count, error).
func extractValidCertificates(data []byte, keyName string) ([]string, int, int, error) {
	if len(data) == 0 {
		return nil, 0, 0, fmt.Errorf("failed to process CA bundle key '%s': contains no data", keyName)
	}

	var certificates []string
	totalSize := 0
	remaining := data

	for {
		block, rest := pem.Decode(remaining)
		if block == nil {
			break
		}

		// Only accept CERTIFICATE blocks, reject other PEM types
		if block.Type != "CERTIFICATE" {
			// Skip non-certificate blocks (could be private keys, etc.)
			remaining = rest
			continue
		}

		// Validate that this is actually a valid X.509 certificate
		if _, err := x509.ParseCertificate(block.Bytes); err != nil {
			return nil, 0, 0, fmt.Errorf("failed to parse X.509 certificate from key '%s': %w", keyName, err)
		}

		// Re-encode the certificate to ensure it's properly formatted
		certPEM := pem.EncodeToMemory(block)
		if certPEM == nil {
			return nil, 0, 0, fmt.Errorf("failed to encode certificate from key '%s'", keyName)
		}

		certificates = append(certificates, string(certPEM))
		totalSize += len(certPEM)
		remaining = rest
	}

	if len(certificates) == 0 {
		return nil, 0, 0, fmt.Errorf("failed to find valid certificates in CA bundle key '%s'", keyName)
	}

	return certificates, totalSize, len(certificates), nil
}

// reconcileManagedCABundleConfigMap creates or updates the managed CA bundle ConfigMap.
func (r *LlamaStackDistributionReconciler) reconcileManagedCABundleConfigMap(ctx context.Context, instance *llamav1alpha1.LlamaStackDistribution) error {
	logger := log.FromContext(ctx)

	// Gather all CA certificate data
	caBundleData, err := r.gatherCABundleData(ctx, instance)
	if err != nil {
		return fmt.Errorf("failed to gather CA bundle data: %w", err)
	}

	managedConfigMapName := getManagedCABundleConfigMapName(instance)

	// Check if the managed ConfigMap already exists
	existingConfigMap := &corev1.ConfigMap{}
	err = r.Get(ctx, types.NamespacedName{
		Name:      managedConfigMapName,
		Namespace: instance.Namespace,
	}, existingConfigMap)

	if err != nil && !k8serrors.IsNotFound(err) {
		return fmt.Errorf("failed to get managed CA bundle ConfigMap: %w", err)
	}

	// Create the desired ConfigMap
	desiredConfigMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      managedConfigMapName,
			Namespace: instance.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "llama-stack-operator",
				"app.kubernetes.io/instance":   instance.Name,
				"app.kubernetes.io/component":  "ca-bundle",
			},
		},
		Data: map[string]string{
			ManagedCABundleKey: caBundleData,
		},
	}

	// Set owner reference so the ConfigMap is deleted when the LlamaStackDistribution is deleted
	if refErr := ctrl.SetControllerReference(instance, desiredConfigMap, r.Scheme); refErr != nil {
		return fmt.Errorf("failed to set controller reference on managed CA bundle ConfigMap: %w", refErr)
	}

	if k8serrors.IsNotFound(err) {
		// ConfigMap doesn't exist, create it
		logger.Info("Creating managed CA bundle ConfigMap", "configMap", managedConfigMapName)
		if err := r.Create(ctx, desiredConfigMap); err != nil {
			return fmt.Errorf("failed to create managed CA bundle ConfigMap: %w", err)
		}
		logger.Info("Successfully created managed CA bundle ConfigMap", "configMap", managedConfigMapName)
	} else {
		// ConfigMap exists, update it if the data has changed
		if existingConfigMap.Data[ManagedCABundleKey] != caBundleData {
			logger.Info("Updating managed CA bundle ConfigMap", "configMap", managedConfigMapName)
			// Use Patch instead of Update to avoid race conditions
			patch := client.MergeFrom(existingConfigMap.DeepCopy())
			existingConfigMap.Data = desiredConfigMap.Data
			existingConfigMap.Labels = desiredConfigMap.Labels
			if err := r.Patch(ctx, existingConfigMap, patch); err != nil {
				if k8serrors.IsConflict(err) {
					// Conflict detected, will be retried by controller
					return fmt.Errorf("failed to patch managed CA bundle ConfigMap (conflict): %w", err)
				}
				return fmt.Errorf("failed to patch managed CA bundle ConfigMap: %w", err)
			}
			logger.Info("Successfully updated managed CA bundle ConfigMap", "configMap", managedConfigMapName)
		} else {
			logger.V(1).Info("Managed CA bundle ConfigMap is up to date", "configMap", managedConfigMapName)
		}
	}

	return nil
}

// detectODHTrustedCABundle checks if the well-known ODH trusted CA bundle ConfigMap
// exists in the same namespace as the LlamaStackDistribution and returns its available keys.
// Returns the ConfigMap and a list of data keys if found, or nil and empty slice if not found.
func (r *LlamaStackDistributionReconciler) detectODHTrustedCABundle(ctx context.Context, instance *llamav1alpha1.LlamaStackDistribution) (*corev1.ConfigMap, []string, error) {
	logger := log.FromContext(ctx)

	configMap := &corev1.ConfigMap{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      odhTrustedCABundleConfigMap,
		Namespace: instance.Namespace,
	}, configMap)

	if err != nil {
		if k8serrors.IsNotFound(err) {
			logger.V(1).Info("ODH trusted CA bundle ConfigMap not found, skipping auto-detection",
				"configMapName", odhTrustedCABundleConfigMap,
				"namespace", instance.Namespace)
			return nil, nil, nil
		}
		return nil, nil, fmt.Errorf("failed to check for ODH trusted CA bundle ConfigMap %s/%s: %w",
			instance.Namespace, odhTrustedCABundleConfigMap, err)
	}

	// Extract available data keys
	// PEM data is validated in extractValidCertificates()
	// which properly validates all PEM blocks.
	keys := make([]string, 0, len(configMap.Data))

	for key := range configMap.Data {
		keys = append(keys, key)
		logger.V(1).Info("Auto-detected CA bundle key",
			"configMapName", odhTrustedCABundleConfigMap,
			"namespace", instance.Namespace,
			"key", key)
	}

	logger.V(1).Info("ODH trusted CA bundle ConfigMap detected",
		"configMapName", odhTrustedCABundleConfigMap,
		"namespace", instance.Namespace,
		"availableKeys", keys)

	return configMap, keys, nil
}

// createDefaultConfigMap creates a ConfigMap with default feature flag values.
func createDefaultConfigMap(configMapName types.NamespacedName) (*corev1.ConfigMap, error) {
	featureFlags := featureflags.FeatureFlags{
		EnableNetworkPolicy: featureflags.FeatureFlag{
			Enabled: featureflags.NetworkPolicyDefaultValue,
		},
	}

	featureFlagsYAML, err := yaml.Marshal(featureFlags)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal default feature flags: %w", err)
	}

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName.Name,
			Namespace: configMapName.Namespace,
		},
		Data: map[string]string{
			featureflags.FeatureFlagsKey: string(featureFlagsYAML),
		},
	}, nil
}

// parseFeatureFlags extracts and parses feature flags from ConfigMap data.
func parseFeatureFlags(configMapData map[string]string) (bool, error) {
	enableNetworkPolicy := featureflags.NetworkPolicyDefaultValue

	featureFlagsYAML, exists := configMapData[featureflags.FeatureFlagsKey]
	if !exists {
		return enableNetworkPolicy, nil
	}

	var flags featureflags.FeatureFlags
	if err := yaml.Unmarshal([]byte(featureFlagsYAML), &flags); err != nil {
		return false, fmt.Errorf("failed to parse feature flags: %w", err)
	}

	return flags.EnableNetworkPolicy.Enabled, nil
}

// NewLlamaStackDistributionReconciler creates a new reconciler with default image mappings.
func NewLlamaStackDistributionReconciler(ctx context.Context, client client.Client, scheme *runtime.Scheme,
	clusterInfo *cluster.ClusterInfo) (*LlamaStackDistributionReconciler, error) {
	// get operator namespace
	operatorNamespace, err := deploy.GetOperatorNamespace()
	if err != nil {
		return nil, fmt.Errorf("failed to get operator namespace: %w", err)
	}

	// Get the ConfigMap
	// If the ConfigMap doesn't exist, create it with default feature flags
	// If the ConfigMap exists, parse the feature flags from the Configmap
	configMap := &corev1.ConfigMap{}
	configMapName := types.NamespacedName{
		Name:      operatorConfigData,
		Namespace: operatorNamespace,
	}

	if err = client.Get(ctx, configMapName, configMap); err != nil {
		if !k8serrors.IsNotFound(err) {
			return nil, fmt.Errorf("failed to get ConfigMap: %w", err)
		}

		// ConfigMap doesn't exist, create it with defaults
		configMap, err = createDefaultConfigMap(configMapName)
		if err != nil {
			return nil, fmt.Errorf("failed to generate default configMap: %w", err)
		}

		if err = client.Create(ctx, configMap); err != nil {
			return nil, fmt.Errorf("failed to create ConfigMap: %w", err)
		}
	}

	// Parse feature flags from ConfigMap
	enableNetworkPolicy, err := parseFeatureFlags(configMap.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to parse feature flags: %w", err)
	}
	return &LlamaStackDistributionReconciler{
		Client:              client,
		Scheme:              scheme,
		EnableNetworkPolicy: enableNetworkPolicy,
		ClusterInfo:         clusterInfo,
		httpClient:          &http.Client{Timeout: 5 * time.Second},
	}, nil
}

// NewTestReconciler creates a reconciler for testing, allowing injection of a custom http client and feature flags.
func NewTestReconciler(client client.Client, scheme *runtime.Scheme, clusterInfo *cluster.ClusterInfo,
	httpClient *http.Client, enableNetworkPolicy bool) *LlamaStackDistributionReconciler {
	return &LlamaStackDistributionReconciler{
		Client:              client,
		Scheme:              scheme,
		ClusterInfo:         clusterInfo,
		httpClient:          httpClient,
		EnableNetworkPolicy: enableNetworkPolicy,
	}
}
