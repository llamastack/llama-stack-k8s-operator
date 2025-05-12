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
	ResourceReadyTimeout = 2 * time.Minute
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
