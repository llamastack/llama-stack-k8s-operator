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
	"strings"
	"sync"
	"time"

	"github.com/go-logr/logr"
	llamav1alpha1 "github.com/llamastack/llama-stack-k8s-operator/api/v1alpha1"
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

// LlamaStackDistributionReconciler reconciles a LlamaStack object.
type LlamaStackDistributionReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
	// Feature flags
	EnableNetworkPolicy bool
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

	// Instance not found - skip reconciliation
	if instance == nil {
		return ctrl.Result{}, nil
	}

	// Reconcile all resources
	if err := r.reconcileResources(ctx, instance); err != nil {
		return ctrl.Result{}, err
	}

	// Check if requeue is needed
	if !instance.Status.Ready {
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

	// Update status
	if err := r.updateStatus(ctx, instance); err != nil {
		return fmt.Errorf("failed to update status: %w", err)
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
	err := r.Get(ctx, types.NamespacedName{Name: pvc.Name, Namespace: pvc.Namespace}, found)
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

	// Resolve the container image
	resolvedImage, err := r.resolveImage(instance)
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
				MatchLabels: map[string]string{llamav1alpha1.DefaultLabelKey: llamav1alpha1.DefaultLabelValue},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{llamav1alpha1.DefaultLabelKey: llamav1alpha1.DefaultLabelValue},
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
	port := instance.Spec.Server.ContainerSpec.Port
	if port == 0 {
		port = llamav1alpha1.DefaultServerPort
	}

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name + "-service",
			Namespace: instance.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{llamav1alpha1.DefaultLabelKey: llamav1alpha1.DefaultLabelValue},
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
	serviceName := fmt.Sprintf("%s-service", instance.Name)
	port := instance.Spec.Server.ContainerSpec.Port
	if port == 0 {
		port = llamav1alpha1.DefaultServerPort
	}

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
		return nil, fmt.Errorf("providers endpoint returned status code %d", resp.StatusCode)
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
func (r *LlamaStackDistributionReconciler) updateStatus(ctx context.Context, instance *llamav1alpha1.LlamaStackDistribution) error {
	deployment := &appsv1.Deployment{}
	err := r.Get(ctx, types.NamespacedName{Name: instance.Name, Namespace: instance.Namespace}, deployment)
	if err != nil && !k8serrors.IsNotFound(err) {
		return fmt.Errorf("failed to fetch deployment for status: %w", err)
	}

	// Check if deployment is ready
	expectedReplicas := instance.Spec.Replicas
	deploymentReady := err == nil && deployment.Status.ReadyReplicas == expectedReplicas

	// Only check health and providers if deployment is ready
	if deploymentReady {
		// Use goroutines for concurrent health and provider checks
		// Use a channel of size 1 to avoid goroutine leaks due to blocking sends
		var wg sync.WaitGroup
		healthChan := make(chan struct {
			healthy bool
			err     error
		}, 1)
		providersChan := make(chan struct {
			providers []llamav1alpha1.ProviderInfo
			err       error
		}, 1)

		// Check health endpoint in a goroutine
		wg.Add(1)
		go func() {
			defer wg.Done()
			healthy, err := r.checkHealth(ctx, instance)
			healthChan <- struct {
				healthy bool
				err     error
			}{healthy, err}
		}()

		// Get provider information in a goroutine
		wg.Add(1)
		go func() {
			defer wg.Done()
			providers, err := r.getProviderInfo(ctx, instance)
			providersChan <- struct {
				providers []llamav1alpha1.ProviderInfo
				err       error
			}{providers, err}
		}()

		// Wait for both goroutines to complete and collect results
		wg.Wait()

		// Process health check results by reading from the channel
		healthResult := <-healthChan
		if healthResult.err != nil {
			r.Log.Error(healthResult.err, "failed to check health endpoint")
		} else {
			instance.Status.Ready = healthResult.healthy
		}

		// Process provider information results
		providersResult := <-providersChan
		if providersResult.err != nil {
			r.Log.Error(providersResult.err, "failed to get provider information")
		} else {
			instance.Status.DistributionConfig.Providers = providersResult.providers
		}
	} else {
		instance.Status.Ready = false
	}

	if err := r.Status().Update(ctx, instance); err != nil {
		return fmt.Errorf("failed to update status: %w", err)
	}

	return nil
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

	// Use the container's port (defaulted to 8321 if unset)
	port := instance.Spec.Server.ContainerSpec.Port
	if port == 0 {
		port = llamav1alpha1.DefaultServerPort
	}

	// get operator namespace
	operatorNamespace, err := deploy.GetOperatorNamespace()
	if err != nil {
		return fmt.Errorf("failed to get operator namespace: %w", err)
	}

	networkPolicy.Spec = networkingv1.NetworkPolicySpec{
		PodSelector: metav1.LabelSelector{
			MatchLabels: map[string]string{
				llamav1alpha1.DefaultLabelKey: llamav1alpha1.DefaultLabelValue,
			},
		},
		PolicyTypes: []networkingv1.PolicyType{
			networkingv1.PolicyTypeIngress,
		},
		Ingress: []networkingv1.NetworkPolicyIngressRule{
			{
				From: []networkingv1.NetworkPolicyPeer{
					{
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
					{
						PodSelector: &metav1.LabelSelector{}, // Empty podSelector to match all pods
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
			{
				From: []networkingv1.NetworkPolicyPeer{
					{
						PodSelector: &metav1.LabelSelector{}, // Empty podSelector to match all pods
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

// NewLlamaStackDistributionReconciler creates a new reconciler with default image mappings.
func NewLlamaStackDistributionReconciler(ctx context.Context, client client.Client, scheme *runtime.Scheme) (*LlamaStackDistributionReconciler, error) {
	log := log.FromContext(ctx).WithName("controller")
	// get operator namespace
	operatorNamespace, err := deploy.GetOperatorNamespace()
	if err != nil {
		return nil, fmt.Errorf("failed to get operator namespace: %w", err)
	}

	// Read the ConfigMap
	configMap := &corev1.ConfigMap{}
	configMapName := types.NamespacedName{
		Name:      "llama-stack-operator-config",
		Namespace: operatorNamespace,
	}
	err = client.Get(ctx, configMapName, configMap)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			// Create the ConfigMap if it doesn't exist
			configMap = &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      configMapName.Name,
					Namespace: configMapName.Namespace,
				},
				Data: map[string]string{
					featureflags.FeatureFlagsKey: fmt.Sprintf("%s: %v",
						featureflags.EnableNetworkPolicyKey, featureflags.NetworkPolicyDefaultValue),
				},
			}
			if err = client.Create(ctx, configMap); err != nil {
				return nil, fmt.Errorf("failed to create ConfigMap: %w", err)
			}
		} else {
			return nil, fmt.Errorf("failed to read ConfigMap: %w", err)
		}
	}

	// Parse the feature flag
	enableNetworkPolicy := featureflags.NetworkPolicyDefaultValue
	if featureFlagsYAML, exists := configMap.Data[featureflags.FeatureFlagsKey]; exists {
		var flags featureflags.FeatureFlags
		if err := yaml.Unmarshal([]byte(featureFlagsYAML), &flags); err != nil {
			log.Error(err, "failed to parse feature flags")
		} else if flags.EnableNetworkPolicy != "" {
			enableNetworkPolicy = strings.ToLower(flags.EnableNetworkPolicy) == "true"
		}
	}

	return &LlamaStackDistributionReconciler{
		Client:              client,
		Scheme:              scheme,
		Log:                 log,
		EnableNetworkPolicy: enableNetworkPolicy,
	}, nil
}
