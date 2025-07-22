//nolint:testpackage
package e2e

import (
	"context"
	"log"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/llamastack/llama-stack-k8s-operator/api/v1alpha1"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/yaml"
)

const (
	ollamaNS             = "ollama-dist"
	pollInterval         = 10 * time.Second
	ResourceReadyTimeout = 5 * time.Minute
	generalRetryInterval = 5 * time.Second
)

var (
	Scheme = runtime.NewScheme()
)

// TestEnvironment holds the test environment configuration.
type TestEnvironment struct {
	Client client.Client
	Ctx    context.Context //nolint:containedctx // Context is used for test environment
}

// SetupTestEnv sets up the test environment.
func SetupTestEnv() (*TestEnvironment, error) {
	// Get a config to talk to the apiserver
	cfg, err := config.GetConfig()
	if err != nil {
		return nil, err
	}

	// Create a new client
	cl, err := client.New(cfg, client.Options{Scheme: Scheme})
	if err != nil {
		return nil, err
	}

	return &TestEnvironment{
		Client: cl,
		Ctx:    context.TODO(),
	}, nil
}

// validateCRD checks if a CustomResourceDefinition is established.
func validateCRD(c client.Client, ctx context.Context, crdName string) error {
	crd := &apiextv1.CustomResourceDefinition{}
	obj := client.ObjectKey{
		Name: crdName,
	}

	err := wait.PollUntilContextTimeout(ctx, generalRetryInterval, ResourceReadyTimeout, true, func(ctx context.Context) (bool, error) {
		err := c.Get(ctx, obj, crd)
		if err != nil {
			if errors.IsNotFound(err) {
				return false, nil
			}
			log.Printf("Failed to get CRD %s", crdName)
			return false, err
		}

		for _, condition := range crd.Status.Conditions {
			if condition.Type == apiextv1.Established {
				if condition.Status == apiextv1.ConditionTrue {
					return true, nil
				}
			}
		}
		log.Printf("Error to get CRD %s condition's matching", crdName)
		return false, nil
	})

	return err
}

// GetDeployment gets a deployment by name and namespace.
func GetDeployment(cl client.Client, ctx context.Context, name, namespace string) (*appsv1.Deployment, error) {
	deployment := &appsv1.Deployment{}
	err := cl.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, deployment)
	return deployment, err
}

// EnsureResourceReady polls until the resource is ready.
func EnsureResourceReady(
	t *testing.T,
	testenv *TestEnvironment,
	gvk schema.GroupVersionKind,
	name, namespace string,
	timeout time.Duration,
	isReady func(*unstructured.Unstructured) bool,
) error {
	t.Helper()
	ctx, cancel := context.WithTimeout(testenv.Ctx, timeout)
	defer cancel()
	return wait.PollUntilContextTimeout(ctx, pollInterval, timeout, true, func(ctx context.Context) (bool, error) {
		obj := &unstructured.Unstructured{}
		obj.SetGroupVersionKind(gvk)
		err := testenv.Client.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, obj)
		if err != nil {
			if errors.IsNotFound(err) {
				return false, nil
			}
			return false, err
		}
		return isReady(obj), nil
	})
}

// EnsureResourceDeleted polls until the resource is deleted.
func EnsureResourceDeleted(t *testing.T, testenv *TestEnvironment, gvk schema.GroupVersionKind, name, namespace string, timeout time.Duration) error {
	t.Helper()
	ctx, cancel := context.WithTimeout(testenv.Ctx, timeout)
	defer cancel()
	return wait.PollUntilContextTimeout(ctx, pollInterval, timeout, true, func(ctx context.Context) (bool, error) {
		obj := &unstructured.Unstructured{}
		obj.SetGroupVersionKind(gvk)
		err := testenv.Client.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, obj)
		if errors.IsNotFound(err) {
			return true, nil
		}
		return false, nil
	})
}

// CleanupTestEnv cleans up the test environment.
func CleanupTestEnv(env *TestEnvironment) {
	// Implementation will be added later
}

// registerSchemes registers all necessary schemes for testing.
func registerSchemes() {
	schemes := []func(*runtime.Scheme) error{
		clientgoscheme.AddToScheme,
		apiextv1.AddToScheme,
		v1alpha1.AddToScheme,
	}

	for _, schemeFn := range schemes {
		utilruntime.Must(schemeFn(Scheme))
	}
}

// GetSampleCR returns a LlamaStackDistribution from the sample YAML file.
func GetSampleCR(t *testing.T) *v1alpha1.LlamaStackDistribution {
	t.Helper()
	// Get the absolute path of the project root
	projectRoot, err := filepath.Abs("../..")
	require.NoError(t, err)

	// Construct the path to the sample file
	samplePath := filepath.Join(projectRoot, "config", "samples", "_v1alpha1_llamastackdistribution.yaml")

	// Read the sample file
	yamlFile, err := os.ReadFile(samplePath)
	require.NoError(t, err)

	// Create and unmarshal the distribution
	distribution := &v1alpha1.LlamaStackDistribution{}
	err = yaml.Unmarshal(yamlFile, distribution)
	require.NoError(t, err)

	return distribution
}

// checkLlamaStackDistributionStatus helps identify if the custom resource reached the expected state during test execution.
func checkLlamaStackDistributionStatus(t *testing.T, testenv *TestEnvironment, namespace, name string) {
	t.Helper()

	llsDistro := &v1alpha1.LlamaStackDistribution{}
	err := testenv.Client.Get(testenv.Ctx, client.ObjectKey{Namespace: namespace, Name: name}, llsDistro)
	if err != nil {
		t.Logf("⚠️  Error getting LlamaStackDistribution: %v", err)
		return
	}

	t.Logf("LlamaStackDistribution status:")
	t.Logf("  Phase: %s", llsDistro.Status.Phase)
	t.Logf("  Generation: %d", llsDistro.Generation)
	t.Logf("  ResourceVersion: %s", llsDistro.ResourceVersion)
	t.Logf("  Conditions: %+v", llsDistro.Status.Conditions)
}

// checkNamespaceEvents reveals what Kubernetes operations occurred and why they may have failed.
func checkNamespaceEvents(t *testing.T, testenv *TestEnvironment, namespace string) {
	t.Helper()

	eventList := &corev1.EventList{}
	err := testenv.Client.List(testenv.Ctx, eventList, client.InNamespace(namespace))
	if err != nil {
		t.Logf("⚠️  Error getting events: %v", err)
		return
	}

	if len(eventList.Items) == 0 {
		t.Log("📝 No events found in namespace")
		return
	}

	maxEvents := 25
	if len(eventList.Items) > maxEvents {
		t.Logf("📝 Showing first %d events (of %d total):", maxEvents, len(eventList.Items))
		eventList.Items = eventList.Items[:maxEvents]
	} else {
		t.Logf("📝 Found %d events in namespace %s:", len(eventList.Items), namespace)
	}

	for _, event := range eventList.Items {
		t.Logf("  %s: %s (%s) - %s",
			event.LastTimestamp.Format("15:04:05"),
			event.Reason,
			event.Type,
			event.Message)
	}
}

// requireNoErrorWithDebugging provides comprehensive debugging context when tests fail to help identify root causes quickly.
func requireNoErrorWithDebugging(t *testing.T, testenv *TestEnvironment, err error, msg string, namespace, crName string) {
	t.Helper()
	if err != nil {
		t.Logf("💥 ERROR OCCURRED: %s - %v", msg, err)

		// Check custom resource status first to see if the operator processed the request correctly
		checkLlamaStackDistributionStatus(t, testenv, namespace, crName)

		// Check events to understand what Kubernetes operations were attempted and why they failed
		checkNamespaceEvents(t, testenv, namespace)

		// Check pod details to identify container startup issues or crash loops
		logPodDetails(t, testenv, namespace)

		// Check service endpoints to see if pods are being discovered by services
		logServiceEndpoints(t, testenv, namespace, crName+"-service")

		// Check service configuration to identify selector mismatches
		logServiceSpec(t, testenv, namespace, crName+"-service")

		// Check deployment spec to identify configuration problems preventing pod startup
		logDeploymentSpec(t, testenv, namespace, crName)

		require.NoError(t, err, msg)
	}
}

// logPodDetails helps diagnose pod startup issues and container restart problems during test failures.
func logPodDetails(t *testing.T, testenv *TestEnvironment, namespace string) {
	t.Helper()

	podList := &corev1.PodList{}
	err := testenv.Client.List(testenv.Ctx, podList, client.InNamespace(namespace))
	if err != nil {
		t.Logf("Failed to list pods: %v", err)
		return
	}

	t.Logf("📦 Found %d pods in namespace %s:", len(podList.Items), namespace)
	for _, pod := range podList.Items {
		t.Logf("Pod: %s, Phase: %s", pod.Name, pod.Status.Phase)

		for _, cs := range pod.Status.ContainerStatuses {
			// RestartCount indicates crash loops or configuration issues
			t.Logf("  Container %s: Ready=%v, RestartCount=%d",
				cs.Name, cs.Ready, cs.RestartCount)

			// Container states reveal why pods aren't starting or are crashing
			if cs.State.Waiting != nil {
				t.Logf("    Waiting: %s - %s",
					cs.State.Waiting.Reason, cs.State.Waiting.Message)
			}
			if cs.State.Terminated != nil {
				t.Logf("    Terminated: %s - %s",
					cs.State.Terminated.Reason, cs.State.Terminated.Message)
			}
		}

		// Pod logs would show startup errors but require different client access
		t.Logf("  (Pod logs require direct kubectl access)")
	}
}

// logServiceEndpoints logs service endpoint details to see if pods are ready.
func logServiceEndpoints(t *testing.T, testenv *TestEnvironment, namespace, serviceName string) {
	t.Helper()

	endpoints := &corev1.Endpoints{}
	err := testenv.Client.Get(testenv.Ctx, types.NamespacedName{
		Name:      serviceName,
		Namespace: namespace,
	}, endpoints)

	if err != nil {
		t.Logf("Failed to get endpoints for service %s: %v", serviceName, err)
		return
	}

	t.Logf("🔗 Service %s endpoints:", serviceName)
	for i, subset := range endpoints.Subsets {
		t.Logf("  Subset %d:", i)
		// Ready addresses indicate pods that passed health checks and can receive traffic
		t.Logf("    Ready addresses: %d", len(subset.Addresses))
		for _, addr := range subset.Addresses {
			t.Logf("      - %s", addr.IP)
		}
		// Not ready addresses show pods that exist but failed health checks
		t.Logf("    Not ready addresses: %d", len(subset.NotReadyAddresses))
		for _, addr := range subset.NotReadyAddresses {
			t.Logf("      - %s", addr.IP)
		}
		t.Logf("    Ports:")
		for _, port := range subset.Ports {
			t.Logf("      - %s: %d", port.Name, port.Port)
		}
	}
}

// logDeploymentSpec helps identify configuration mismatches that prevent pods from starting correctly.
func logDeploymentSpec(t *testing.T, testenv *TestEnvironment, namespace, name string) {
	t.Helper()

	deployment := &appsv1.Deployment{}
	err := testenv.Client.Get(testenv.Ctx, types.NamespacedName{
		Name:      name,
		Namespace: namespace,
	}, deployment)

	if err != nil {
		t.Logf("Failed to get deployment: %v", err)
		return
	}

	t.Logf("🚀 Deployment %s spec:", name)
	t.Logf("  Replicas: %d", *deployment.Spec.Replicas)
	// Selector must match pod labels or pods won't be managed by deployment
	t.Logf("  Selector: %+v", deployment.Spec.Selector.MatchLabels)
	t.Logf("  Template labels: %+v", deployment.Spec.Template.Labels)

	for _, container := range deployment.Spec.Template.Spec.Containers {
		t.Logf("  Container: %s", container.Name)
		t.Logf("    Image: %s", container.Image)
		t.Logf("    Ports:")
		for _, port := range container.Ports {
			t.Logf("      - %d", port.ContainerPort)
		}
		// Environment variables can cause startup failures if misconfigured
		t.Logf("    Env vars:")
		for _, env := range container.Env {
			t.Logf("      %s=%s", env.Name, env.Value)
		}
		// Readiness probe configuration affects when pods become service endpoints
		if container.ReadinessProbe != nil {
			t.Logf("    Readiness probe: %+v", container.ReadinessProbe)
		}
	}
}

// logServiceSpec logs the actual service configuration to debug selector issues.
func logServiceSpec(t *testing.T, testenv *TestEnvironment, namespace, serviceName string) {
	t.Helper()

	service := &corev1.Service{}
	err := testenv.Client.Get(testenv.Ctx, types.NamespacedName{
		Name:      serviceName,
		Namespace: namespace,
	}, service)

	if err != nil {
		t.Logf("Failed to get service %s: %v", serviceName, err)
		return
	}

	t.Logf("🔧 Service %s spec:", serviceName)
	t.Logf("  Type: %s", service.Spec.Type)
	// Selector must match pod labels or service won't route traffic to pods
	t.Logf("  Selector: %+v", service.Spec.Selector)
	t.Logf("  Ports:")
	for _, port := range service.Spec.Ports {
		t.Logf("    - %s: %d -> %s", port.Name, port.Port, port.TargetPort.String())
	}
}
