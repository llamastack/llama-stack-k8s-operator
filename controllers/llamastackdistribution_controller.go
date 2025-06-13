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
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"

	"github.com/go-logr/logr"
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
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	operatorConfigData = "llama-stack-operator-config"
)

// LlamaStackDistributionReconciler reconciles a LlamaStack object.
type LlamaStackDistributionReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
	// Feature flags
	EnableNetworkPolicy bool
	// Cluster info
	ClusterInfo *cluster.ClusterInfo
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
	r.Log = r.Log.WithValues("llamastack", req.NamespacedName)

	// Fetch the LlamaStack instance
	instance, err := r.fetchInstance(ctx, req.NamespacedName)
	if err != nil {
		return ctrl.Result{}, err
	}
	if instance == nil {
		r.Log.Info("LlamaStackDistribution resource not found, skipping reconciliation", "namespacedName", req.NamespacedName)
		return ctrl.Result{}, nil
	}

	// Reconcile all resources, storing the error for later.
	reconcileErr := r.reconcileResources(ctx, instance)

	// Update the status, passing in any reconciliation error.
	if statusUpdateErr := r.updateStatus(ctx, instance, reconcileErr); statusUpdateErr != nil {
		// Log the status update error, but prioritize the reconciliation error for return.
		r.Log.Error(statusUpdateErr, "failed to update status")
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

	r.Log.Info("Successfully reconciled LlamaStackDistribution")
	return ctrl.Result{}, nil
}

// fetchInstance retrieves the LlamaStackDistribution instance.
func (r *LlamaStackDistributionReconciler) fetchInstance(ctx context.Context, namespacedName types.NamespacedName) (*llamav1alpha1.LlamaStackDistribution, error) {
	instance := &llamav1alpha1.LlamaStackDistribution{}
	if err := r.Get(ctx, namespacedName, instance); err != nil {
		if k8serrors.IsNotFound(err) {
			r.Log.Info("failed to find LlamaStackDistribution resource")
			return nil, nil
		}
		return nil, fmt.Errorf("failed to fetch LlamaStackDistribution: %w", err)
	}
	return instance, nil
}

// reconcileResources reconciles all resources for the LlamaStackDistribution instance.
func (r *LlamaStackDistributionReconciler) reconcileResources(ctx context.Context, instance *llamav1alpha1.LlamaStackDistribution) error {
	// Reconcile the PVC if storage is configured
	if instance.Spec.Server.Storage != nil {
		if err := r.reconcilePVC(ctx, instance); err != nil {
			return fmt.Errorf("failed to reconcile PVC: %w", err)
		}
	}

	// Reconcile the NetworkPolicy
	if err := r.reconcileNetworkPolicy(ctx, instance); err != nil {
		return fmt.Errorf("failed to reconcile NetworkPolicy: %w", err)
	}

	// Reconcile the Deployment
	if err := r.reconcileDeployment(ctx, instance); err != nil {
		return fmt.Errorf("failed to reconcile Deployment: %w", err)
	}

	// Reconcile the Service if ports are defined, else use default port
	if instance.HasPorts() {
		if err := r.reconcileService(ctx, instance); err != nil {
			return fmt.Errorf("failed to reconcile service: %w", err)
		}
	}
	return nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *LlamaStackDistributionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&llamav1alpha1.LlamaStackDistribution{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&networkingv1.NetworkPolicy{}).
		Owns(&corev1.PersistentVolumeClaim{}).
		Complete(r)
}

// reconcilePVC creates or updates the PVC for the LlamaStack server.
func (r *LlamaStackDistributionReconciler) reconcilePVC(ctx context.Context, instance *llamav1alpha1.LlamaStackDistribution) error {
	logger := log.FromContext(ctx)

	// Use default size if none specified
	size := instance.Spec.Server.Storage.Size
	if size == nil {
		size = &llamav1alpha1.DefaultStorageSize
	}

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name + "-pvc",
			Namespace: instance.Namespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: *size,
				},
			},
		},
	}

	if err := ctrl.SetControllerReference(instance, pvc, r.Scheme); err != nil {
		return fmt.Errorf("failed to set controller reference: %w", err)
	}

	found := &corev1.PersistentVolumeClaim{}
	err := r.Get(ctx, client.ObjectKeyFromObject(pvc), found)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			logger.Info("Creating PVC", "pvc", pvc.Name)
			return r.Create(ctx, pvc)
		}
		return fmt.Errorf("failed to fetch PVC: %w", err)
	}
	// PVCs are immutable after creation, so we don't need to update them
	return nil
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
				},
				Spec: podSpec,
			},
		},
	}

	return deploy.ApplyDeployment(ctx, r.Client, r.Scheme, instance, deployment, logger)
}

// reconcileService manages the Service if ports are defined.
func (r *LlamaStackDistributionReconciler) reconcileService(ctx context.Context, instance *llamav1alpha1.LlamaStackDistribution) error {
	// Use the container's port (defaulted to 8321 if unset)
	port := deploy.GetServicePort(instance)

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploy.GetServiceName(instance),
			Namespace: instance.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				llamav1alpha1.DefaultLabelKey: llamav1alpha1.DefaultLabelValue,
				"app.kubernetes.io/instance":  instance.Name,
			},
			Ports: []corev1.ServicePort{{
				Name: llamav1alpha1.DefaultServicePortName,
				Port: port,
				TargetPort: intstr.IntOrString{
					IntVal: port,
				},
			}},
			Type: corev1.ServiceTypeClusterIP,
		},
	}

	return deploy.ApplyService(ctx, r.Client, r.Scheme, instance, service, r.Log)
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

// checkHealth makes an HTTP request to the health endpoint.
func (r *LlamaStackDistributionReconciler) checkHealth(ctx context.Context, instance *llamav1alpha1.LlamaStackDistribution) (bool, error) {
	u := r.getServerURL(instance, "/v1/health")

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return false, fmt.Errorf("failed to create health check request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return false, fmt.Errorf("failed to make health check request: %w", err)
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK, nil
}

// getProviderInfo makes an HTTP request to the providers endpoint.
func (r *LlamaStackDistributionReconciler) getProviderInfo(ctx context.Context, instance *llamav1alpha1.LlamaStackDistribution) ([]llamav1alpha1.ProviderInfo, error) {
	u := r.getServerURL(instance, "/v1/providers")

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create providers request: %w", err)
	}

	resp, err := client.Do(req)
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

// updateStatus refreshes the LlamaStack status.
func (r *LlamaStackDistributionReconciler) updateStatus(ctx context.Context, instance *llamav1alpha1.LlamaStackDistribution, reconcileErr error) error {
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
			r.performHealthChecks(ctx, instance)
		} else {
			// If not ready, health can't be checked. Set condition appropriately.
			SetHealthCheckCondition(&instance.Status, false, "Deployment not ready")
			instance.Status.DistributionConfig.Providers = nil // Clear providers
		}
	}

	// Always update the status at the end of the function.
	instance.Status.Version.LastUpdated = metav1.Now()
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
	default:
		instance.Status.Phase = llamav1alpha1.LlamaStackDistributionPhaseReady
		deploymentReady = true
		SetDeploymentReadyCondition(&instance.Status, true, MessageDeploymentReady)
		if instance.Status.Version.LlamaStackVersion == "" {
			instance.Status.Version.LlamaStackVersion = os.Getenv("LLAMASTACK_VERSION")
		}
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
	if !instance.HasPorts() {
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

func (r *LlamaStackDistributionReconciler) performHealthChecks(ctx context.Context, instance *llamav1alpha1.LlamaStackDistribution) {
	healthy, err := r.checkHealth(ctx, instance)
	if err != nil {
		SetHealthCheckCondition(&instance.Status, false, fmt.Sprintf("Health check failed: %v", err))
	} else {
		var healthMessage string
		if healthy {
			healthMessage = MessageHealthCheckPassed
		} else {
			healthMessage = MessageHealthCheckFailed
		}
		SetHealthCheckCondition(&instance.Status, healthy, healthMessage)
	}

	// If health check is not passing, set phase to failed.
	if !IsConditionTrue(&instance.Status, ConditionTypeHealthCheck) {
		instance.Status.Phase = llamav1alpha1.LlamaStackDistributionPhaseFailed
	}

	providers, err := r.getProviderInfo(ctx, instance)
	if err != nil {
		r.Log.Error(err, "failed to get provider info, clearing provider list")
		instance.Status.DistributionConfig.Providers = nil
	} else {
		instance.Status.DistributionConfig.Providers = providers
	}
}

// reconcileNetworkPolicy manages the NetworkPolicy for the LlamaStack server.
func (r *LlamaStackDistributionReconciler) reconcileNetworkPolicy(ctx context.Context, instance *llamav1alpha1.LlamaStackDistribution) error {
	networkPolicy := &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name + "-network-policy",
			Namespace: instance.Namespace,
		},
	}

	// If feature is disabled, delete the NetworkPolicy if it exists
	if !r.EnableNetworkPolicy {
		return deploy.HandleDisabledNetworkPolicy(ctx, r.Client, networkPolicy, r.Log)
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

	return deploy.ApplyNetworkPolicy(ctx, r.Client, r.Scheme, instance, networkPolicy, r.Log)
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
	log := log.FromContext(ctx).WithName("controller")
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
		Log:                 log,
		EnableNetworkPolicy: enableNetworkPolicy,
		ClusterInfo:         clusterInfo,
	}, nil
}
