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
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"time"

	"github.com/go-logr/logr"
	"github.com/google/go-cmp/cmp"
	llamav1alpha1 "github.com/llamastack/llama-stack-k8s-operator/api/v1alpha1"
	"github.com/llamastack/llama-stack-k8s-operator/pkg/cluster"
	"github.com/llamastack/llama-stack-k8s-operator/pkg/deploy"
	"github.com/llamastack/llama-stack-k8s-operator/pkg/featureflags"
	"gopkg.in/yaml.v3"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/kustomize/kyaml/filesys"
)

const (
	operatorConfigData = "llama-stack-operator-config"
	manifestsBasePath  = "manifests/base"

	// CA Bundle related constants.
	DefaultCABundleKey = "ca-bundle.crt"
	CABundleMountPath  = "/etc/ssl/certs/ca-bundle.crt"
)

// LlamaStackDistributionReconciler reconciles a LlamaStack object.
type LlamaStackDistributionReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	// Feature flags
	EnableNetworkPolicy bool
	// Cluster info
	ClusterInfo *cluster.ClusterInfo
	httpClient  *http.Client
}

// hasCABundle checks if the instance has a valid TLSConfig with CABundle data.
// Returns true if configured, false otherwise.
func (r *LlamaStackDistributionReconciler) hasCABundle(instance *llamav1alpha1.LlamaStackDistribution) bool {
	return instance.Spec.Server.TLSConfig != nil && instance.Spec.Server.TLSConfig.CABundle != ""
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

	return kinds
}

// reconcileManifestResources applies resources that are managed by the operator
// based on the instance specification.
func (r *LlamaStackDistributionReconciler) reconcileManifestResources(ctx context.Context, instance *llamav1alpha1.LlamaStackDistribution) error {
	resMap, err := deploy.RenderManifest(filesys.MakeFsOnDisk(), manifestsBasePath, instance)
	if err != nil {
		return fmt.Errorf("failed to render manifests: %w", err)
	}

	kindsToExclude := r.determineKindsToExclude(instance)
	filteredResMap, err := deploy.FilterExcludeKinds(resMap, kindsToExclude)
	if err != nil {
		return fmt.Errorf("failed to filter manifests: %w", err)
	}

	if err := deploy.ApplyResources(ctx, r.Client, r.Scheme, instance, filteredResMap); err != nil {
		return fmt.Errorf("failed to apply manifests: %w", err)
	}

	return nil
}

// reconcileResources reconciles all resources for the LlamaStackDistribution instance.
func (r *LlamaStackDistributionReconciler) reconcileResources(ctx context.Context, instance *llamav1alpha1.LlamaStackDistribution) error {
	// Reconcile ConfigMaps
	if err := r.reconcileConfigMaps(ctx, instance); err != nil {
		return err
	}

	// Reconcile storage
	if err := r.reconcileStorage(ctx, instance); err != nil {
		return err
	}

	// Reconcile manifest-based resources
	if err := r.reconcileManifestResources(ctx, instance); err != nil {
		return err
	}

	// Reconcile the NetworkPolicy
	if err := r.reconcileNetworkPolicy(ctx, instance); err != nil {
		return fmt.Errorf("failed to reconcile NetworkPolicy: %w", err)
	}

	// Reconcile the Deployment
	if err := r.reconcileDeployment(ctx, instance); err != nil {
		return fmt.Errorf("failed to reconcile Deployment: %w", err)
	}

	return nil
}

func (r *LlamaStackDistributionReconciler) reconcileConfigMaps(ctx context.Context, instance *llamav1alpha1.LlamaStackDistribution) error {
	configMapName := instance.Name + "-config"

	// Check if we need the combined ConfigMap
	needsConfigMap := (instance.Spec.Server.UserConfig != nil && instance.Spec.Server.UserConfig.CustomConfig != "") || r.hasCABundle(instance)

	if needsConfigMap {
		// Reconcile the combined ConfigMap if either user config or CA bundle is specified
		if err := r.reconcileCombinedConfigMap(ctx, instance); err != nil {
			return fmt.Errorf("failed to reconcile combined ConfigMap: %w", err)
		}
	} else {
		// Clean up the ConfigMap if it exists but is no longer needed
		if err := r.cleanupConfigMapIfExists(ctx, configMapName, instance.Namespace); err != nil {
			return fmt.Errorf("failed to cleanup ConfigMap: %w", err)
		}
	}

	return nil
}

func (r *LlamaStackDistributionReconciler) reconcileStorage(ctx context.Context, instance *llamav1alpha1.LlamaStackDistribution) error {
	// Reconcile the PVC if storage is configured
	if instance.Spec.Server.Storage != nil {
		resMap, err := deploy.RenderManifest(filesys.MakeFsOnDisk(), manifestsBasePath, instance)
		if err != nil {
			return fmt.Errorf("failed to render PVC manifests: %w", err)
		}
		if err := deploy.ApplyResources(ctx, r.Client, r.Scheme, instance, resMap); err != nil {
			return fmt.Errorf("failed to apply PVC manifests: %w", err)
		}
	}

	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *LlamaStackDistributionReconciler) SetupWithManager(ctx context.Context, mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&llamav1alpha1.LlamaStackDistribution{}, builder.WithPredicates(predicate.Funcs{
			UpdateFunc: r.llamaStackUpdatePredicate(mgr),
		})).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&networkingv1.NetworkPolicy{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Complete(r)
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

// reconcileDeployment manages the Deployment for the LlamaStack server.
func (r *LlamaStackDistributionReconciler) reconcileDeployment(ctx context.Context, instance *llamav1alpha1.LlamaStackDistribution) error {
	logger := log.FromContext(ctx)

	// Validate distribution configuration
	if err := r.validateDistribution(instance); err != nil {
		return err
	}

	// Get the image either from the map or direct reference
	resolvedImage, err := r.resolveImage(instance.Spec.Server.Distribution)
	if err != nil {
		return err
	}

	// Build container spec
	container := buildContainerSpec(instance, resolvedImage)

	// Configure storage
	podSpec := configurePodStorage(instance, container)

	// Set the service acc
	// Prepare annotations for the pod template
	podAnnotations, err := r.BuildPodAnnotations(ctx, instance)
	if err != nil {
		return err
	}

	// Create deployment object
	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name,
			Namespace: instance.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &instance.Spec.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					llamav1alpha1.DefaultLabelKey: llamav1alpha1.DefaultLabelValue,
					"app.kubernetes.io/instance":  instance.Name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						llamav1alpha1.DefaultLabelKey: llamav1alpha1.DefaultLabelValue,
						"app.kubernetes.io/instance":  instance.Name,
					},
					Annotations: podAnnotations,
				},
				Spec: podSpec,
			},
		},
	}

	return deploy.ApplyDeployment(ctx, r.Client, r.Scheme, instance, deployment, logger)
}

// BuildPodAnnotations creates annotations for the pod template to trigger restarts when configurations change.
func (r *LlamaStackDistributionReconciler) BuildPodAnnotations(ctx context.Context, instance *llamav1alpha1.LlamaStackDistribution) (map[string]string, error) {
	logger := log.FromContext(ctx)
	podAnnotations := make(map[string]string)

	// Only calculate checksum if there are configuration fields that could trigger pod restarts
	if r.hasConfigurationData(instance) {
		// Calculate a checksum of the configuration content to trigger pod restarts when configs change
		configChecksum, err := r.CalculateConfigurationChecksum(ctx, instance)
		if err != nil {
			logger.Error(err, "Failed to calculate configuration checksum")
			return nil, fmt.Errorf("failed to calculate configuration checksum: %w", err)
		}

		// Add the configuration checksum as an annotation
		// When this checksum changes, Kubernetes will restart the pods
		if configChecksum != "" {
			podAnnotations["llamastack.io/config-checksum"] = configChecksum
			logger.V(1).Info("Added configuration checksum annotation", "checksum", configChecksum)
		}
	} else {
		logger.V(1).Info("No configuration data present, skipping checksum calculation")
	}

	return podAnnotations, nil
}

// hasConfigurationData checks if the instance has any configuration data that should trigger
// pod restarts when changed. This includes userConfig and tlsConfig.
func (r *LlamaStackDistributionReconciler) hasConfigurationData(instance *llamav1alpha1.LlamaStackDistribution) bool {
	// Check for explicit user configuration
	if instance.Spec.Server.UserConfig != nil && instance.Spec.Server.UserConfig.CustomConfig != "" {
		return true
	}

	// Check for explicit TLS configuration
	if instance.Spec.Server.TLSConfig != nil && instance.Spec.Server.TLSConfig.CABundle != "" {
		return true
	}

	return false
}

// CalculateConfigurationChecksum computes a simple non-cryptographic checksum of the configuration content
// that should trigger pod restarts when changed. This includes userConfig and tlsConfig.
// Uses FNV-1a hash which is fast, deterministic, and not cryptographic (FIPS-compliant).
func (r *LlamaStackDistributionReconciler) CalculateConfigurationChecksum(ctx context.Context, instance *llamav1alpha1.LlamaStackDistribution) (string, error) {
	logger := log.FromContext(ctx)

	// Create a structure to hold all configuration data that should trigger restarts
	configData := r.buildConfigurationData(instance)

	// Marshal the configuration data to JSON for consistent checksumming
	configJSON, err := json.Marshal(configData)
	if err != nil {
		return "", fmt.Errorf("failed to marshal configuration data: %w", err)
	}

	// Calculate FNV-1a hash (non-cryptographic, fast, deterministic)
	checksumString := r.calculateFNVHash(configJSON)

	r.logConfigurationChecksum(logger, checksumString, configData)

	return checksumString, nil
}

// buildConfigurationData creates the configuration data structure for checksumming.
func (r *LlamaStackDistributionReconciler) buildConfigurationData(instance *llamav1alpha1.LlamaStackDistribution) struct {
	UserConfig *llamav1alpha1.UserConfigSpec `json:"userConfig,omitempty"`
	TLSConfig  *llamav1alpha1.TLSConfig      `json:"tlsConfig,omitempty"`
} {
	configData := struct {
		UserConfig *llamav1alpha1.UserConfigSpec `json:"userConfig,omitempty"`
		TLSConfig  *llamav1alpha1.TLSConfig      `json:"tlsConfig,omitempty"`
	}{
		UserConfig: instance.Spec.Server.UserConfig,
		TLSConfig:  instance.Spec.Server.TLSConfig,
	}

	return configData
}

// calculateFNVHash computes the FNV-1a hash of the given data.
func (r *LlamaStackDistributionReconciler) calculateFNVHash(data []byte) string {
	hasher := fnv.New64a()
	hasher.Write(data)
	checksum := hasher.Sum64()
	return strconv.FormatUint(checksum, 16)
}

// logConfigurationChecksum logs the calculated checksum and configuration details.
func (r *LlamaStackDistributionReconciler) logConfigurationChecksum(logger logr.Logger, checksumString string, configData struct {
	UserConfig *llamav1alpha1.UserConfigSpec `json:"userConfig,omitempty"`
	TLSConfig  *llamav1alpha1.TLSConfig      `json:"tlsConfig,omitempty"`
}) {
	logger.V(1).Info("Calculated configuration checksum",
		"checksum", checksumString,
		"hasUserConfig", configData.UserConfig != nil && configData.UserConfig.CustomConfig != "",
		"hasTLSConfig", configData.TLSConfig != nil && configData.TLSConfig.CABundle != "")
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
	// Initialize OperatorVersion if not set
	if instance.Status.Version.OperatorVersion == "" {
		instance.Status.Version.OperatorVersion = os.Getenv("OPERATOR_VERSION")
	}

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

// reconcileNetworkPolicy manages the NetworkPolicy for the LlamaStack server.
func (r *LlamaStackDistributionReconciler) reconcileNetworkPolicy(ctx context.Context, instance *llamav1alpha1.LlamaStackDistribution) error {
	logger := log.FromContext(ctx)
	networkPolicy := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name + "-network-policy",
			Namespace: instance.Namespace,
		},
	}

	// If feature is disabled, delete the NetworkPolicy if it exists
	if !r.EnableNetworkPolicy {
		return deploy.HandleDisabledNetworkPolicy(ctx, r.Client, networkPolicy, logger)
	}

	port := deploy.GetServicePort(instance)

	// get operator namespace
	operatorNamespace, err := deploy.GetOperatorNamespace()
	if err != nil {
		return fmt.Errorf("failed to get operator namespace: %w", err)
	}

	networkPolicy.Spec = networkingv1.NetworkPolicySpec{
		PodSelector: metav1.LabelSelector{
			MatchLabels: map[string]string{
				llamav1alpha1.DefaultLabelKey: llamav1alpha1.DefaultLabelValue,
				"app.kubernetes.io/instance":  instance.Name,
			},
		},
		PolicyTypes: []networkingv1.PolicyType{
			networkingv1.PolicyTypeIngress,
		},
		Ingress: []networkingv1.NetworkPolicyIngressRule{
			{
				From: []networkingv1.NetworkPolicyPeer{
					{ // to match all pods in all namespaces
						PodSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app.kubernetes.io/part-of": llamav1alpha1.DefaultContainerName,
							},
						},
						NamespaceSelector: &metav1.LabelSelector{}, // Empty namespaceSelector to match all namespaces
					},
				},
				Ports: []networkingv1.NetworkPolicyPort{
					{
						Protocol: (*corev1.Protocol)(ptr.To("TCP")),
						Port: &intstr.IntOrString{
							IntVal: port,
						},
					},
				},
			},
			{
				From: []networkingv1.NetworkPolicyPeer{
					{ // to match all pods in matched namespace
						PodSelector: &metav1.LabelSelector{},
						NamespaceSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"kubernetes.io/metadata.name": operatorNamespace,
							},
						},
					},
				},
				Ports: []networkingv1.NetworkPolicyPort{
					{
						Protocol: (*corev1.Protocol)(ptr.To("TCP")),
						Port: &intstr.IntOrString{
							IntVal: port,
						},
					},
				},
			},
		},
	}

	return deploy.ApplyNetworkPolicy(ctx, r.Client, r.Scheme, instance, networkPolicy, logger)
}

// isValidPEM validates that the given data contains valid PEM formatted content.
func isValidPEM(data []byte) bool {
	// Basic PEM validation using pem.Decode.
	block, _ := pem.Decode(data)
	return block != nil
}

// reconcileCombinedConfigMap creates or updates an operator-managed ConfigMap containing both user configuration and CA bundle data.
func (r *LlamaStackDistributionReconciler) reconcileCombinedConfigMap(ctx context.Context, instance *llamav1alpha1.LlamaStackDistribution) error {
	logger := log.FromContext(ctx)
	configMapName := instance.Name + "-config"

	// Prepare the ConfigMap data
	configMapData, err := r.buildCombinedConfigMapData(instance, logger)
	if err != nil {
		return err
	}

	// Skip if no data to store
	if len(configMapData) == 0 {
		return nil
	}

	// Create the ConfigMap with the combined data
	configMap := r.createConfigMapObject(configMapName, instance, configMapData)

	// Set controller reference so the ConfigMap is owned by this LlamaStackDistribution
	if err := ctrl.SetControllerReference(instance, configMap, r.Scheme); err != nil {
		return fmt.Errorf("failed to set controller reference for combined ConfigMap: %w", err)
	}

	// Handle ConfigMap creation or update
	return r.createOrUpdateConfigMap(ctx, configMapName, instance, configMap, configMapData, logger)
}

// buildCombinedConfigMapData prepares the data for the operator-managed ConfigMap.
func (r *LlamaStackDistributionReconciler) buildCombinedConfigMapData(instance *llamav1alpha1.LlamaStackDistribution, logger logr.Logger) (map[string]string, error) {
	configMapData := make(map[string]string)

	// Add user config if specified
	if instance.Spec.Server.UserConfig != nil && instance.Spec.Server.UserConfig.CustomConfig != "" {
		configMapData["run.yaml"] = instance.Spec.Server.UserConfig.CustomConfig
	}

	// Add CA bundle if specified
	if r.hasCABundle(instance) {
		pemData := instance.Spec.Server.TLSConfig.CABundle
		if !isValidPEM([]byte(pemData)) {
			logger.Error(nil, "CA bundle contains invalid PEM data")
			return nil, errors.New("CA bundle contains invalid PEM data")
		}
		configMapData[DefaultCABundleKey] = pemData
	}

	return configMapData, nil
}

// cleanupConfigMapIfExists deletes the ConfigMap if it exists but is no longer needed.
func (r *LlamaStackDistributionReconciler) cleanupConfigMapIfExists(ctx context.Context, configMapName, namespace string) error {
	logger := log.FromContext(ctx)

	// Check if the ConfigMap exists
	configMap := &corev1.ConfigMap{}
	err := r.Get(ctx, client.ObjectKey{Name: configMapName, Namespace: namespace}, configMap)
	if err != nil {
		if client.IgnoreNotFound(err) == nil {
			// ConfigMap doesn't exist, nothing to clean up
			return nil
		}
		return fmt.Errorf("failed to get ConfigMap %s: %w", configMapName, err)
	}

	// Delete the ConfigMap
	logger.Info("Cleaning up ConfigMap as it's no longer needed", "configMapName", configMapName)
	if err := r.Delete(ctx, configMap); err != nil {
		return fmt.Errorf("failed to delete ConfigMap %s: %w", configMapName, err)
	}

	return nil
}

// createConfigMapObject creates a new ConfigMap object with the given data.
func (r *LlamaStackDistributionReconciler) createConfigMapObject(
	configMapName string,
	instance *llamav1alpha1.LlamaStackDistribution,
	configMapData map[string]string,
) *corev1.ConfigMap {
	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: instance.Namespace,
		},
		Data: configMapData,
	}
}

// createOrUpdateConfigMap handles the creation or update of the ConfigMap.
func (r *LlamaStackDistributionReconciler) createOrUpdateConfigMap(
	ctx context.Context,
	configMapName string,
	instance *llamav1alpha1.LlamaStackDistribution,
	configMap *corev1.ConfigMap,
	configMapData map[string]string,
	logger logr.Logger,
) error {
	// Check if ConfigMap already exists
	existing := &corev1.ConfigMap{}
	err := r.Get(ctx, client.ObjectKey{Name: configMapName, Namespace: instance.Namespace}, existing)
	if err != nil {
		return r.handleConfigMapCreation(ctx, err, configMap, configMapName, logger)
	}

	// ConfigMap exists, update it if the data has changed
	return r.handleConfigMapUpdate(ctx, existing, configMapData, configMapName, logger)
}

// handleConfigMapCreation handles the creation of a new ConfigMap.
func (r *LlamaStackDistributionReconciler) handleConfigMapCreation(ctx context.Context, err error, configMap *corev1.ConfigMap, configMapName string, logger logr.Logger) error {
	if !k8serrors.IsNotFound(err) {
		return fmt.Errorf("failed to get existing combined ConfigMap: %w", err)
	}

	// ConfigMap doesn't exist, create it
	logger.Info("Creating combined ConfigMap", "configMapName", configMapName)
	if err := r.Create(ctx, configMap); err != nil {
		return fmt.Errorf("failed to create combined ConfigMap: %w", err)
	}
	return nil
}

// handleConfigMapUpdate handles the update of an existing ConfigMap.
func (r *LlamaStackDistributionReconciler) handleConfigMapUpdate(
	ctx context.Context,
	existing *corev1.ConfigMap,
	configMapData map[string]string,
	configMapName string,
	logger logr.Logger,
) error {
	dataChanged := r.hasConfigMapDataChanged(existing, configMapData)

	if dataChanged {
		logger.Info("Updating combined ConfigMap", "configMapName", configMapName)
		existing.Data = configMapData
		if err := r.Update(ctx, existing); err != nil {
			return fmt.Errorf("failed to update combined ConfigMap: %w", err)
		}
	}

	r.logConfigMapReconciliation(logger, configMapName, configMapData)
	return nil
}

// hasConfigMapDataChanged checks if the ConfigMap data has changed.
func (r *LlamaStackDistributionReconciler) hasConfigMapDataChanged(existing *corev1.ConfigMap, configMapData map[string]string) bool {
	// Check if any values have changed
	for key, value := range configMapData {
		if existing.Data[key] != value {
			return true
		}
	}

	// Check if any keys were removed
	for key := range existing.Data {
		if _, exists := configMapData[key]; !exists {
			return true
		}
	}

	return false
}

// logConfigMapReconciliation logs the successful reconciliation of the ConfigMap.
func (r *LlamaStackDistributionReconciler) logConfigMapReconciliation(logger logr.Logger, configMapName string, configMapData map[string]string) {
	keys := make([]string, 0, len(configMapData))
	for key := range configMapData {
		keys = append(keys, key)
	}

	logger.V(1).Info("Combined ConfigMap reconciled successfully",
		"configMapName", configMapName,
		"keys", keys)
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
