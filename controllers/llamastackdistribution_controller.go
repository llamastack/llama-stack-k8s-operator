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
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/go-logr/logr"
	llamav1alpha1 "github.com/meta-llama/llama-stack-k8s-operator/api/v1alpha1"
	"github.com/meta-llama/llama-stack-k8s-operator/pkg/deploy"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/intstr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	defaultContainerName   = "llama-stack"
	defaultPort            = 8321 // Matches the QuickStart guide
	defaultServicePortName = "http"
	defaultLabelKey        = "app"
	defaultLabelValue      = "llama-stack"
)

// Define a map that translates user-friendly names to actual image references.
var imageMap = llamav1alpha1.ImageMap

// LlamaStackDistributionReconciler reconciles a LlamaStack object.
type LlamaStackDistributionReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Log    logr.Logger
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
	instance := &llamav1alpha1.LlamaStackDistribution{}
	if err := r.Get(ctx, req.NamespacedName, instance); err != nil {
		if k8serrors.IsNotFound(err) {
			r.Log.Info("failed to find LlamaStackDistribution resource")
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, fmt.Errorf("failed to fetch LlamaStackDistribution: %w", err)
	}

	// Reconcile the Deployment
	if err := r.reconcileDeployment(ctx, instance); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to reconcile Deployment: %w", err)
	}

	// Reconcile the Service if ports are defined, else use default port
	if instance.HasPorts() {
		if err := r.reconcileService(ctx, instance); err != nil {
			return ctrl.Result{}, fmt.Errorf("failed to reconcile service: %w", err)
		}
	}

	// Update status
	if err := r.updateStatus(ctx, instance); err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to update status: %w", err)
	}

	r.Log.Info("Successfully reconciled LlamaStackDistribution")
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *LlamaStackDistributionReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&llamav1alpha1.LlamaStackDistribution{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Complete(r)
}

// reconcileDeployment manages the Deployment for the LlamaStack server.
func (r *LlamaStackDistributionReconciler) reconcileDeployment(ctx context.Context, instance *llamav1alpha1.LlamaStackDistribution) error {
	logger := log.FromContext(ctx)

	// Validate that only one of name or image is set
	if instance.Spec.Server.Distribution.Name != "" && instance.Spec.Server.Distribution.Image != "" {
		return errors.New("only one of distribution.name or distribution.image can be set")
	}

	// Get the image either from the map or direct reference
	var resolvedImage string
	switch {
	case instance.Spec.Server.Distribution.Name != "":
		resolvedImage = imageMap[instance.Spec.Server.Distribution.Name]
		if resolvedImage == "" {
			return fmt.Errorf("failed to validate distribution name: %s", instance.Spec.Server.Distribution.Name)
		}
	case instance.Spec.Server.Distribution.Image != "":
		resolvedImage = instance.Spec.Server.Distribution.Image
	default:
		return errors.New("failed to validate distribution: either distribution.name or distribution.image must be set")
	}

	// Build the container spec
	container := corev1.Container{
		Name:      defaultContainerName,
		Image:     resolvedImage,
		Resources: instance.Spec.Server.ContainerSpec.Resources,
		Env:       instance.Spec.Server.ContainerSpec.Env,
	}
	if instance.Spec.Server.ContainerSpec.Name != "" {
		container.Name = instance.Spec.Server.ContainerSpec.Name
	}
	if instance.Spec.Server.ContainerSpec.Port != 0 {
		container.Ports = []corev1.ContainerPort{{ContainerPort: instance.Spec.Server.ContainerSpec.Port}}
	} else {
		container.Ports = []corev1.ContainerPort{{ContainerPort: defaultPort}}
	}

	podSpec := corev1.PodSpec{
		Containers: []corev1.Container{container},
	}
	if instance.Spec.Server.PodOverrides != nil {
		podSpec.Volumes = instance.Spec.Server.PodOverrides.Volumes
		container.VolumeMounts = instance.Spec.Server.PodOverrides.VolumeMounts
		podSpec.Containers[0] = container // Update with volume mounts
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name,
			Namespace: instance.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &instance.Spec.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{defaultLabelKey: defaultLabelValue},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{defaultLabelKey: defaultLabelValue},
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
		port = defaultPort
	}

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name + "-service",
			Namespace: instance.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{defaultLabelKey: defaultLabelValue},
			Ports: []corev1.ServicePort{{
				Name: defaultServicePortName,
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
		port = defaultPort
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

// NewLlamaStackDistributionReconciler creates a new reconciler with default image mappings.
func NewLlamaStackDistributionReconciler(client client.Client, scheme *runtime.Scheme) *LlamaStackDistributionReconciler {
	return &LlamaStackDistributionReconciler{
		Client: client,
		Scheme: scheme,
		Log:    ctrl.Log.WithName("controller"),
	}
}
