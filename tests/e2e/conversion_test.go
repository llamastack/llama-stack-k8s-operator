//nolint:testpackage
package e2e

import (
	"testing"

	"github.com/llamastack/llama-stack-k8s-operator/api/v1alpha1"
	"github.com/llamastack/llama-stack-k8s-operator/api/v1alpha2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const conversionTestNS = "llama-stack-test-conversion"

// TestConversionSuite runs round-trip conversion tests between v1alpha1 and v1alpha2.
func TestConversionSuite(t *testing.T) {
	t.Helper()
	if TestOpts.SkipCreation {
		t.Skip("Skipping conversion test suite")
	}

	ns := &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{Name: conversionTestNS},
	}
	err := TestEnv.Client.Create(TestEnv.Ctx, ns)
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		require.NoError(t, err)
	}

	t.Run("v1alpha1 CR should be readable as v1alpha2", testV1Alpha1ToV1Alpha2Conversion)
	t.Run("v1alpha2 CR should be readable as v1alpha1", testV1Alpha2ToV1Alpha1Conversion)
}

func testV1Alpha1ToV1Alpha2Conversion(t *testing.T) {
	t.Helper()

	v1CR := GetSampleCRForDistribution(t, starterDistType)
	v1CR.Namespace = conversionTestNS
	v1CR.Name = "conversion-test-v1"
	v1CR.Spec.Replicas = 1
	v1CR.Spec.Server.Storage = nil
	v1CR.Spec.Server.PodDisruptionBudget = nil
	v1CR.Spec.Server.Autoscaling = nil
	v1CR.Spec.Server.TopologySpreadConstraints = nil

	err := TestEnv.Client.Create(TestEnv.Ctx, v1CR)
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		require.NoError(t, err)
	}
	defer TestEnv.Client.Delete(TestEnv.Ctx, v1CR)

	v2CR := &v1alpha2.LlamaStackDistribution{}
	err = TestEnv.Client.Get(TestEnv.Ctx, client.ObjectKey{
		Namespace: conversionTestNS,
		Name:      "conversion-test-v1",
	}, v2CR)
	require.NoError(t, err, "Should be able to read v1alpha1 CR as v1alpha2")

	assert.Equal(t, v1CR.Spec.Server.Distribution.Name, v2CR.Spec.Distribution.Name,
		"Distribution name should be preserved through conversion")

	if v2CR.Spec.Workload != nil && v2CR.Spec.Workload.Replicas != nil {
		assert.Equal(t, v1CR.Spec.Replicas, *v2CR.Spec.Workload.Replicas,
			"Replicas should be preserved through conversion")
	}
}

func testV1Alpha2ToV1Alpha1Conversion(t *testing.T) {
	t.Helper()

	v2CR := GetV1Alpha2SampleCR(t)
	v2CR.Namespace = conversionTestNS
	v2CR.Name = "conversion-test-v2"

	err := TestEnv.Client.Create(TestEnv.Ctx, v2CR)
	if err != nil && !k8serrors.IsAlreadyExists(err) {
		require.NoError(t, err)
	}
	defer TestEnv.Client.Delete(TestEnv.Ctx, v2CR)

	v1CR := &v1alpha1.LlamaStackDistribution{}
	err = TestEnv.Client.Get(TestEnv.Ctx, client.ObjectKey{
		Namespace: conversionTestNS,
		Name:      "conversion-test-v2",
	}, v1CR)
	require.NoError(t, err, "Should be able to read v1alpha2 CR as v1alpha1")

	assert.Equal(t, v2CR.Spec.Distribution.Name, v1CR.Spec.Server.Distribution.Name,
		"Distribution name should be preserved through conversion")
}
