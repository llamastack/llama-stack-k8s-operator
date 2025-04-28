package e2e

import (
	"context"
	"fmt"
	"log"
	"testing"
	"time"

	"github.com/meta-llama/llama-stack-k8s-operator/api/v1alpha1"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

const (
	ollamaNS             = "ollama-distro"
	testTimeout          = 5 * time.Minute
	pollInterval         = 10 * time.Second
	crdReadyTimeout      = 2 * time.Minute
	generalRetryInterval = 5 * time.Second
)

// TestEnvironment holds the test environment configuration.
type TestEnvironment struct {
	Client client.Client
	Ctx    context.Context
}

var (
	// TestEnv is the global test environment
	TestEnv *TestEnvironment
)

// SetupTestEnv sets up the test environment.
func SetupTestEnv() (*TestEnvironment, error) {
	// Get a config to talk to the apiserver
	cfg, err := config.GetConfig()
	if err != nil {
		return nil, err
	}

	// Add CRD scheme
	err = v1alpha1.AddToScheme(scheme.Scheme)
	if err != nil {
		return nil, err
	}

	// Create a new client
	cl, err := client.New(cfg, client.Options{Scheme: scheme.Scheme})
	if err != nil {
		return nil, err
	}

	// Create a new context
	ctx := context.Background()

	return &TestEnvironment{
		Client: cl,
		Ctx:    ctx,
	}, nil
}

// InitializeTestEnv initializes the test environment
func InitializeTestEnv() error {
	var err error
	TestEnv, err = SetupTestEnv()
	return err
}

// validateCRD checks if a CustomResourceDefinition is established.
func validateCRD(c client.Client, ctx context.Context, crdName string) error {
	crd := &apiextv1.CustomResourceDefinition{}
	obj := client.ObjectKey{
		Name: crdName,
	}

	err := wait.PollWithContext(ctx, generalRetryInterval, testTimeout, func(ctx context.Context) (bool, error) {
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

// GetDeployment gets a deployment by name and namespace
func GetDeployment(cl client.Client, ctx context.Context, name, namespace string) (*appsv1.Deployment, error) {
	deployment := &appsv1.Deployment{}
	err := cl.Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, deployment)
	return deployment, err
}

// PollDeploymentReady polls until the deployment is ready.
func PollDeploymentReady(t *testing.T, c client.Client, name, namespace string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for deployment to be ready")
		case <-ticker.C:
			deployment := &appsv1.Deployment{}
			err := c.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, deployment)
			if err != nil {
				continue
			}

			if deployment.Status.AvailableReplicas == deployment.Status.Replicas {
				return nil
			}
		}
	}
}

// PollServiceReady polls until the service is ready.
func PollServiceReady(t *testing.T, c client.Client, name, namespace string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for service to be ready")
		case <-ticker.C:
			service := &corev1.Service{}
			err := c.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, service)
			if err != nil {
				continue
			}

			if len(service.Status.LoadBalancer.Ingress) > 0 {
				return nil
			}
		}
	}
}

// findCondition finds a condition by type in a slice of conditions
func findCondition(conditions []v1alpha1.Condition, conditionType string) *v1alpha1.Condition {
	for _, condition := range conditions {
		if condition.Type == conditionType {
			return &condition
		}
	}
	return nil
}

// PollCRReady polls until the custom resource is ready.
func PollCRReady(t *testing.T, c client.Client, name, namespace string, timeout time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			require.Fail(t, "Timeout waiting for CR to be ready")
		case <-ticker.C:
			cr := &v1alpha1.LlamaStackDistribution{}
			err := c.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, cr)
			if err != nil {
				continue
			}

			if cr.Status.Ready {
				return
			}
		}
	}
}

// PollDeploymentDeleted polls until the deployment is deleted.
func PollDeploymentDeleted(t *testing.T, c client.Client, name, namespace string, timeout time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			require.Fail(t, "Timeout waiting for deployment to be deleted")
		case <-ticker.C:
			deployment := &appsv1.Deployment{}
			err := c.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, deployment)
			if errors.IsNotFound(err) {
				return
			}
		}
	}
}

// PollServiceDeleted polls until the service is deleted.
func PollServiceDeleted(t *testing.T, c client.Client, name, namespace string, timeout time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			require.Fail(t, "Timeout waiting for service to be deleted")
		case <-ticker.C:
			service := &corev1.Service{}
			err := c.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, service)
			if errors.IsNotFound(err) {
				return
			}
		}
	}
}

// PollCRDeleted polls until the custom resource is deleted.
func PollCRDeleted(t *testing.T, c client.Client, name, namespace string, timeout time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			require.Fail(t, "Timeout waiting for CR to be deleted")
		case <-ticker.C:
			cr := &v1alpha1.LlamaStackDistribution{}
			err := c.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, cr)
			if errors.IsNotFound(err) {
				return
			}
		}
	}
}

// CleanupTestEnv cleans up the test environment.
func CleanupTestEnv(env *TestEnvironment) {
	// Implementation will be added later
}
