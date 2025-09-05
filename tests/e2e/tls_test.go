//nolint:testpackage
package e2e

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/llamastack/llama-stack-k8s-operator/api/v1alpha1"
	"github.com/llamastack/llama-stack-k8s-operator/controllers"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	tlsTestTimeout = 5 * time.Minute
	llsTestNS      = "llama-stack-test"
)

func TestTLSSuite(t *testing.T) {
	if TestOpts.SkipCreation {
		t.Skip("Skipping TLS test suite")
	}

	// Generate certificates before running any tests
	t.Run("should generate certificates", func(t *testing.T) {
		generateCertificates(t)
	})

	t.Run("should create test namespace", func(t *testing.T) {
		testCreateNamespace(t)
	})

	t.Run("should create LlamaStackDistribution with CA bundle", func(t *testing.T) {
		testLlamaStackWithCABundle(t)
	})

	t.Run("should cleanup TLS resources", func(t *testing.T) {
		testTLSCleanup(t)
	})
}

func testCreateNamespace(t *testing.T) {
	t.Helper()

	// Create test namespace
	testNs := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: llsTestNS,
		},
	}
	err := TestEnv.Client.Create(TestEnv.Ctx, testNs)
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		require.NoError(t, err)
	}
}

func testLlamaStackWithCABundle(t *testing.T) {
	t.Helper()

	// Deploy LlamaStackDistribution with CA bundle
	err := deployLlamaStackWithCABundle(t)
	require.NoError(t, err)

	// Verify the LlamaStack distribution is configured with TLS
	err = verifyLlamaStackTLSConfig(t, llsTestNS, "llamastack-with-config")
	require.NoError(t, err)

	// Wait for the operator to process the LlamaStackDistribution and create the deployment
	err = waitForDeploymentCreation(t, llsTestNS, "llamastack-with-config", 3*time.Minute)
	require.NoError(t, err, "LlamaStack deployment should be created by operator")

	// Verify certificate volumes are mounted correctly
	err = verifyCertificateMounts(t, llsTestNS, "llamastack-with-config")
	require.NoError(t, err, "Certificate volumes should be mounted correctly")

	// Verify environment variables are set correctly
	err = verifyEnvironmentVariables(t, llsTestNS, "llamastack-with-config")
	require.NoError(t, err, "Environment variables should be set correctly")
}

func testTLSCleanup(t *testing.T) {
	t.Helper()

	// Delete LlamaStackDistribution
	distribution := &v1alpha1.LlamaStackDistribution{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "llamastack-with-config",
			Namespace: llsTestNS,
		},
	}
	err := TestEnv.Client.Delete(TestEnv.Ctx, distribution)
	if err != nil && !k8serrors.IsNotFound(err) {
		require.NoError(t, err)
	}

	// Wait for LlamaStack resources to be cleaned up
	err = EnsureResourceDeleted(t, TestEnv, schema.GroupVersionKind{
		Group:   "apps",
		Version: "v1",
		Kind:    "Deployment",
	}, "llamastack-with-config", llsTestNS, ResourceReadyTimeout)
	require.NoError(t, err, "LlamaStack deployment should be deleted")
}

// Helper functions

func generateCertificates(t *testing.T) {
	t.Helper()

	// Get the project root path
	projectRoot, err := filepath.Abs("../..")
	require.NoError(t, err, "Failed to get project root")

	// Run the certificate generation script
	scriptPath := filepath.Join(projectRoot, "config", "samples", "generate_certificates.sh")
	t.Logf("Running certificate generation script: %s", scriptPath)

	// Change to the project root directory to run the script
	t.Chdir(projectRoot)

	// Execute the script
	cmd := exec.Command("bash", scriptPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("Certificate generation script output: %s", string(output))
		require.NoError(t, err, "Failed to run certificate generation script")
	}

	t.Log("Certificates generated successfully")
}

func verifyLlamaStackTLSConfig(t *testing.T, namespace, name string) error {
	t.Helper()

	// Get the LlamaStack distribution
	distribution := &v1alpha1.LlamaStackDistribution{}
	err := TestEnv.Client.Get(TestEnv.Ctx, client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}, distribution)
	if err != nil {
		return fmt.Errorf("failed to get LlamaStack distribution: %w", err)
	}

	// Verify TLS configuration is present
	if distribution.Spec.Server.TLSConfig == nil {
		return errors.New("LlamaStack distribution does not have TLS config")
	}

	if distribution.Spec.Server.TLSConfig.CABundle == "" {
		return errors.New("LlamaStack distribution TLS config does not have CA bundle")
	}

	return nil
}

func deployLlamaStackWithCABundle(t *testing.T) error {
	t.Helper()

	// Read the generated CA certificate
	projectRoot, err := filepath.Abs("../..")
	if err != nil {
		return fmt.Errorf("failed to get project root: %w", err)
	}

	caCertPath := filepath.Join(projectRoot, "ca.crt")
	caCertData, err := os.ReadFile(caCertPath)
	if err != nil {
		return fmt.Errorf("failed to read CA certificate: %w", err)
	}

	// Create the LlamaStackDistribution directly instead of using YAML template
	llsd := &v1alpha1.LlamaStackDistribution{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "llamastack-with-config",
			Namespace: llsTestNS,
		},
		Spec: v1alpha1.LlamaStackDistributionSpec{
			Replicas: 1,
			Server: v1alpha1.ServerSpec{
				Distribution: v1alpha1.DistributionType{
					Name: "remote-vllm",
				},
				ContainerSpec: v1alpha1.ContainerSpec{
					Port: 8321,
					Env: []corev1.EnvVar{
						{
							Name:  "INFERENCE_MODEL",
							Value: "meta-llama/Llama-3.2-1B-Instruct",
						},
						{
							Name:  "VLLM_URL",
							Value: "https://vllm-server.vllm-dist.svc.cluster.local:8000/v1",
						},
						{
							Name:  "VLLM_TLS_VERIFY",
							Value: "/etc/ssl/certs/ca-bundle.crt",
						},
					},
				},
				UserConfig: &v1alpha1.UserConfigSpec{
					CustomConfig: `# Llama Stack Configuration
version: '2'
image_name: remote-vllm
apis:
- inference
providers:
  inference:
  - provider_id: vllm
    provider_type: "remote::vllm"
    config:
      url: "https://vllm-server.vllm-dist.svc.cluster.local:8000/v1"
models:
  - model_id: "meta-llama/Llama-3.2-1B-Instruct"
    provider_id: vllm
    model_type: llm
server:
  port: 8321`,
				},
				TLSConfig: &v1alpha1.TLSConfig{
					CABundle: string(caCertData),
				},
			},
		},
	}

	err = TestEnv.Client.Create(TestEnv.Ctx, llsd)
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		return fmt.Errorf("failed to create LlamaStack resource: %w", err)
	}

	return nil
}

func verifyCertificateMounts(t *testing.T, namespace, name string) error {
	t.Helper()

	// Get the deployment
	deployment := &appsv1.Deployment{}
	err := TestEnv.Client.Get(TestEnv.Ctx, client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}, deployment)
	if err != nil {
		return fmt.Errorf("failed to get deployment: %w", err)
	}

	// Check if CA bundle volume is defined
	if !hasCABundleVolume(deployment.Spec.Template.Spec.Volumes) {
		return errors.New("CA bundle volume not found in deployment")
	}

	// Check if CA bundle is mounted in any container
	if !hasCABundleMount(deployment.Spec.Template.Spec.Containers) {
		return errors.New("CA bundle mount not found in any container")
	}

	return nil
}

func hasCABundleVolume(volumes []corev1.Volume) bool {
	for _, volume := range volumes {
		if volume.ConfigMap != nil && volume.Name == controllers.CombinedConfigVolumeName {
			return true
		}
	}
	return false
}

func hasCABundleMount(containers []corev1.Container) bool {
	for _, container := range containers {
		if hasCABundleMountInContainer(container.VolumeMounts) {
			return true
		}
	}
	return false
}

func hasCABundleMountInContainer(mounts []corev1.VolumeMount) bool {
	for _, mount := range mounts {
		if mount.MountPath == controllers.CABundleMountPath ||
			mount.Name == controllers.CombinedConfigVolumeName ||
			strings.Contains(mount.MountPath, "ca-bundle") {
			return true
		}
	}
	return false
}

func verifyEnvironmentVariables(t *testing.T, namespace, name string) error {
	t.Helper()

	// Get the deployment
	deployment := &appsv1.Deployment{}
	err := TestEnv.Client.Get(TestEnv.Ctx, client.ObjectKey{
		Namespace: namespace,
		Name:      name,
	}, deployment)
	if err != nil {
		return fmt.Errorf("failed to get deployment: %w", err)
	}

	// Check for TLS-related environment variables
	tlsEnvVarsFound := 0
	expectedEnvVars := map[string]string{
		"VLLM_TLS_VERIFY": controllers.CABundleMountPath,
	}

	for _, container := range deployment.Spec.Template.Spec.Containers {
		for _, env := range container.Env {
			if expectedValue, exists := expectedEnvVars[env.Name]; exists {
				if env.Value == expectedValue {
					tlsEnvVarsFound++
				} else {
					t.Logf("Found env var with unexpected value: %s=%s (expected: %s)",
						env.Name, env.Value, expectedValue)
				}
			}
		}
	}

	if tlsEnvVarsFound == 0 {
		return errors.New("no expected TLS-related environment variables found")
	}

	return nil
}

func waitForDeploymentCreation(t *testing.T, namespace, name string, timeout time.Duration) error {
	t.Helper()

	return wait.PollUntilContextTimeout(TestEnv.Ctx, 10*time.Second, timeout, true, func(ctx context.Context) (bool, error) {
		// First check if the LlamaStackDistribution is being processed
		distribution := &v1alpha1.LlamaStackDistribution{}
		err := TestEnv.Client.Get(ctx, client.ObjectKey{
			Namespace: namespace,
			Name:      name,
		}, distribution)
		if err != nil {
			t.Logf("LlamaStackDistribution not found yet: %v", err)
			return false, nil
		}

		t.Logf("LlamaStackDistribution status: Phase=%s", distribution.Status.Phase)

		// Then check if the deployment has been created
		deployment := &appsv1.Deployment{}
		err = TestEnv.Client.Get(ctx, client.ObjectKey{
			Namespace: namespace,
			Name:      name,
		}, deployment)
		if err != nil {
			if k8serrors.IsNotFound(err) {
				t.Logf("Deployment %s not created yet by operator, continuing to wait...", name)
				return false, nil
			}
			t.Logf("Error getting deployment: %v", err)
			return false, err
		}

		t.Logf("Deployment %s found, created by operator", name)
		return true, nil
	})
}
