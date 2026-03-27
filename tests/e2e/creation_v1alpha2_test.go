//nolint:testpackage
package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/llamastack/llama-stack-k8s-operator/api/v1alpha2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const v1alpha2TestNS = "llama-stack-test-v1alpha2"

// runV1Alpha2CreationDeletionSuite runs the full lifecycle for a v1alpha2 distribution.
func runV1Alpha2CreationDeletionSuite(t *testing.T) {
	t.Helper()
	if TestOpts.SkipCreation {
		t.Skip("Skipping v1alpha2 creation-deletion test suite")
	}

	var creationFailed bool
	var created *v1alpha2.LlamaStackDistribution

	t.Run("creation", func(t *testing.T) {
		created = runV1Alpha2CreationTests(t)
		creationFailed = t.Failed()
	})

	switch {
	case !creationFailed && !TestOpts.SkipDeletion && created != nil:
		t.Run("deletion", func(t *testing.T) {
			runV1Alpha2DeletionTests(t, created)
		})
	case TestOpts.SkipDeletion:
		t.Log("Skipping v1alpha2 deletion tests (SkipDeletion=true)")
	default:
		t.Log("Skipping v1alpha2 deletion tests due to creation test failures")
	}
}

func runV1Alpha2CreationTests(t *testing.T) *v1alpha2.LlamaStackDistribution {
	t.Helper()

	var distribution *v1alpha2.LlamaStackDistribution

	t.Run("should create v1alpha2 LlamaStackDistribution", func(t *testing.T) {
		distribution = testCreateV1Alpha2Distribution(t)
	})

	t.Run("should generate ConfigMap from native providers", func(t *testing.T) {
		testV1Alpha2ConfigMapGeneration(t, distribution)
	})

	t.Run("should reach Ready phase", func(t *testing.T) {
		testV1Alpha2ReadyPhase(t, distribution)
	})

	t.Run("should populate config generation status", func(t *testing.T) {
		testV1Alpha2ConfigGenerationStatus(t, distribution)
	})

	t.Run("should inject secret env vars into deployment", func(t *testing.T) {
		testV1Alpha2SecretEnvVarsInjected(t)
	})

	return distribution
}

func testCreateV1Alpha2Distribution(t *testing.T) *v1alpha2.LlamaStackDistribution {
	t.Helper()

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: v1alpha2TestNS},
	}
	err := TestEnv.Client.Create(TestEnv.Ctx, ns)
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		require.NoError(t, err)
	}

	distribution := GetV1Alpha2SampleCR(t)
	distribution.Namespace = v1alpha2TestNS

	t.Logf("Creating v1alpha2 distribution: %s", distribution.Name)

	err = TestEnv.Client.Create(TestEnv.Ctx, distribution)
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		require.NoError(t, err)
	}

	err = EnsureResourceReady(t, TestEnv, schema.GroupVersionKind{
		Group:   "apps",
		Version: "v1",
		Kind:    "Deployment",
	}, distribution.Name, v1alpha2TestNS, ResourceReadyTimeout, isDeploymentReady)
	require.NoError(t, err, "Deployment should become ready for v1alpha2 CR")

	err = WaitForPodsReady(t, TestEnv, v1alpha2TestNS, distribution.Name, ResourceReadyTimeout)
	require.NoError(t, err, "Pods should be running and ready for v1alpha2 CR")

	err = EnsureResourceReady(t, TestEnv, schema.GroupVersionKind{
		Group:   "",
		Version: "v1",
		Kind:    "Service",
	}, distribution.Name+"-service", v1alpha2TestNS, ResourceReadyTimeout, func(u *unstructured.Unstructured) bool {
		spec, specFound, _ := unstructured.NestedMap(u.Object, "spec")
		return specFound && spec != nil
	})
	requireNoErrorWithDebugging(t, TestEnv, err, "Service readiness check failed for v1alpha2 CR", v1alpha2TestNS, distribution.Name)

	return distribution
}

func testV1Alpha2ConfigMapGeneration(t *testing.T, distribution *v1alpha2.LlamaStackDistribution) {
	t.Helper()
	if distribution == nil {
		t.Skip("Skipping: v1alpha2 distribution creation failed")
	}

	generatedCMLabels := client.MatchingLabels{
		"app.kubernetes.io/instance":  distribution.Name,
		"app.kubernetes.io/component": "generated-config",
	}

	err := wait.PollUntilContextTimeout(TestEnv.Ctx, pollInterval, ResourceReadyTimeout, true, func(ctx context.Context) (bool, error) {
		cmList := &corev1.ConfigMapList{}
		listErr := TestEnv.Client.List(ctx, cmList,
			client.InNamespace(v1alpha2TestNS),
			generatedCMLabels,
		)
		if listErr != nil {
			return false, listErr
		}
		return len(cmList.Items) > 0, nil
	})
	require.NoError(t, err, "Generated ConfigMap should exist for v1alpha2 distribution")

	cmList := &corev1.ConfigMapList{}
	require.NoError(t, TestEnv.Client.List(TestEnv.Ctx, cmList,
		client.InNamespace(v1alpha2TestNS),
		generatedCMLabels,
	))
	require.NotEmpty(t, cmList.Items, "Should find at least one generated ConfigMap")

	cm := cmList.Items[0]
	configYAML, exists := cm.Data["config.yaml"]
	assert.True(t, exists, "ConfigMap should contain config.yaml key")
	assert.NotEmpty(t, configYAML, "config.yaml should not be empty")
	assert.Contains(t, configYAML, "ollama", "config.yaml should contain ollama provider configuration")
}

func testV1Alpha2ReadyPhase(t *testing.T, distribution *v1alpha2.LlamaStackDistribution) {
	t.Helper()
	if distribution == nil {
		t.Skip("Skipping: v1alpha2 distribution creation failed")
	}

	err := wait.PollUntilContextTimeout(TestEnv.Ctx, 1*time.Minute, 5*time.Minute, true, func(ctx context.Context) (bool, error) {
		updated := &v1alpha2.LlamaStackDistribution{}
		getErr := TestEnv.Client.Get(ctx, client.ObjectKey{
			Namespace: distribution.Namespace,
			Name:      distribution.Name,
		}, updated)
		if getErr != nil {
			return false, getErr
		}
		return updated.Status.Phase == v1alpha2.PhaseReady, nil
	})
	requireNoErrorWithDebugging(t, TestEnv, err, "v1alpha2 distribution should reach Ready phase", v1alpha2TestNS, distribution.Name)
}

func testV1Alpha2ConfigGenerationStatus(t *testing.T, distribution *v1alpha2.LlamaStackDistribution) {
	t.Helper()
	if distribution == nil {
		t.Skip("Skipping: v1alpha2 distribution creation failed")
	}

	updated := &v1alpha2.LlamaStackDistribution{}
	require.NoError(t, TestEnv.Client.Get(TestEnv.Ctx, client.ObjectKey{
		Namespace: distribution.Namespace,
		Name:      distribution.Name,
	}, updated))

	if updated.Status.ConfigGeneration != nil {
		assert.NotEmpty(t, updated.Status.ConfigGeneration.ConfigMapName, "ConfigMapName should be populated")
		assert.Positive(t, updated.Status.ConfigGeneration.ProviderCount, "ProviderCount should be > 0")
	}

	if updated.Status.ResolvedDistribution != nil {
		assert.NotEmpty(t, updated.Status.ResolvedDistribution.Image, "Resolved image should be populated")
	}
}

const v1alpha2SecretEnvNS = "llama-stack-test-v1a2-secret"

// testV1Alpha2SecretEnvVarsInjected verifies that secretRefs on a provider
// result in the corresponding LLSD_*_ env vars being injected into the
// Deployment pod template (FR-022, FR-032).
func testV1Alpha2SecretEnvVarsInjected(t *testing.T) {
	t.Helper()

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: v1alpha2SecretEnvNS},
	}
	err := TestEnv.Client.Create(TestEnv.Ctx, ns)
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		require.NoError(t, err)
	}

	// Create a Secret for the provider to reference.
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-provider-creds",
			Namespace: v1alpha2SecretEnvNS,
		},
		Data: map[string][]byte{
			"api-key": []byte("test-value"),
		},
	}
	err = TestEnv.Client.Create(TestEnv.Ctx, secret)
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		require.NoError(t, err)
	}

	// Create a v1alpha2 CR with secretRefs.
	distribution := &v1alpha2.LlamaStackDistribution{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-secret-env",
			Namespace: v1alpha2SecretEnvNS,
		},
		Spec: v1alpha2.LlamaStackDistributionSpec{
			Distribution: v1alpha2.DistributionSpec{Name: "starter"},
			Providers: &v1alpha2.ProvidersSpec{
				Inference: []v1alpha2.ProviderConfig{
					{
						Provider: "ollama",
						Endpoint: "http://ollama:11434/v1",
						SecretRefs: map[string]v1alpha2.SecretKeyRef{
							"api_key": {Name: "test-provider-creds", Key: "api-key"},
						},
					},
				},
			},
		},
	}
	err = TestEnv.Client.Create(TestEnv.Ctx, distribution)
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		require.NoError(t, err)
	}

	// Wait for the Deployment to be created.
	err = EnsureResourceReady(t, TestEnv, schema.GroupVersionKind{
		Group:   "apps",
		Version: "v1",
		Kind:    "Deployment",
	}, distribution.Name, v1alpha2SecretEnvNS, ResourceReadyTimeout, func(u *unstructured.Unstructured) bool {
		// Any non-nil Deployment is sufficient; we don't need pods running.
		return u != nil
	})
	require.NoError(t, err, "Deployment should be created for v1alpha2 CR with secretRefs")

	// Fetch the Deployment and verify the env var is present.
	deploy, err := GetDeployment(TestEnv.Client, TestEnv.Ctx, distribution.Name, v1alpha2SecretEnvNS)
	require.NoError(t, err, "Should be able to fetch the Deployment")
	require.NotEmpty(t, deploy.Spec.Template.Spec.Containers, "Deployment must have at least one container")

	container := deploy.Spec.Template.Spec.Containers[0]

	var secretEnv *corev1.EnvVar
	for i := range container.Env {
		if container.Env[i].Name == "LLSD_OLLAMA_API_KEY" {
			secretEnv = &container.Env[i]
			break
		}
	}

	require.NotNilf(t, secretEnv,
		"Deployment container must have LLSD_OLLAMA_API_KEY env var from secretRefs (FR-032); got env: %v",
		envVarNames(container.Env))
	require.NotNil(t, secretEnv.ValueFrom, "Secret env var must use ValueFrom")
	require.NotNil(t, secretEnv.ValueFrom.SecretKeyRef, "Secret env var must reference a SecretKeyRef")
	require.Equal(t, "test-provider-creds", secretEnv.ValueFrom.SecretKeyRef.Name)
	require.Equal(t, "api-key", secretEnv.ValueFrom.SecretKeyRef.Key)

	// Verify the generated config.yaml contains the env var placeholder.
	cmList := &corev1.ConfigMapList{}
	require.NoError(t, TestEnv.Client.List(TestEnv.Ctx, cmList,
		client.InNamespace(v1alpha2SecretEnvNS),
		client.MatchingLabels{
			"app.kubernetes.io/instance":  distribution.Name,
			"app.kubernetes.io/component": "generated-config",
		},
	))
	require.NotEmpty(t, cmList.Items, "Generated ConfigMap should exist")

	configYAML := cmList.Items[0].Data["config.yaml"]
	require.Contains(t, configYAML, "${env.LLSD_OLLAMA_API_KEY}",
		"Generated config.yaml must contain env var placeholder for secret (FR-032)")

	t.Log("Secret env var injection verified: LLSD_OLLAMA_API_KEY present in Deployment and config.yaml")

	// Cleanup
	if !TestOpts.SkipDeletion {
		_ = TestEnv.Client.Delete(TestEnv.Ctx, distribution)
		_ = TestEnv.Client.Delete(TestEnv.Ctx, secret)
	}
}

// envVarNames extracts env var names for diagnostic messages.
func envVarNames(envVars []corev1.EnvVar) []string {
	names := make([]string, len(envVars))
	for i, e := range envVars {
		names[i] = e.Name
	}
	return names
}

func runV1Alpha2DeletionTests(t *testing.T, instance *v1alpha2.LlamaStackDistribution) {
	t.Helper()

	t.Run("should delete v1alpha2 CR and cleanup resources", func(t *testing.T) {
		err := TestEnv.Client.Delete(TestEnv.Ctx, instance)
		require.NoError(t, err)

		err = EnsureResourceDeleted(t, TestEnv, schema.GroupVersionKind{
			Group:   "apps",
			Version: "v1",
			Kind:    "Deployment",
		}, instance.Name, instance.Namespace, ResourceReadyTimeout)
		require.NoError(t, err, "Deployment should be deleted")

		err = EnsureResourceDeleted(t, TestEnv, schema.GroupVersionKind{
			Group:   "",
			Version: "v1",
			Kind:    "Service",
		}, instance.Name+"-service", instance.Namespace, ResourceReadyTimeout)
		require.NoError(t, err, "Service should be deleted")

		err = EnsureResourceDeleted(t, TestEnv, schema.GroupVersionKind{
			Group:   "llamastack.io",
			Version: "v1alpha2",
			Kind:    "LlamaStackDistribution",
		}, instance.Name, instance.Namespace, ResourceReadyTimeout)
		require.NoError(t, err, "v1alpha2 CR should be deleted")

		cmList := &corev1.ConfigMapList{}
		require.NoError(t, TestEnv.Client.List(TestEnv.Ctx, cmList,
			client.InNamespace(instance.Namespace),
			client.MatchingLabels{
				"app.kubernetes.io/instance":  instance.Name,
				"app.kubernetes.io/component": "generated-config",
			},
		))
		assert.Empty(t, cmList.Items, "Generated ConfigMaps should be cleaned up")
	})
}
