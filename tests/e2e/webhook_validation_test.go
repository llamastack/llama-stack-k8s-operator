//nolint:testpackage
package e2e

import (
	"encoding/json"
	"testing"

	"github.com/llamastack/llama-stack-k8s-operator/api/v1alpha2"
	"github.com/stretchr/testify/require"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const webhookTestNS = "llama-stack-test-v1alpha2"

// TestWebhookValidationSuite exercises the validating webhook by submitting invalid v1alpha2 CRs.
func TestWebhookValidationSuite(t *testing.T) {
	t.Helper()
	if TestOpts.SkipValidation {
		t.Skip("Skipping webhook validation test suite")
	}

	t.Run("should reject CR missing distribution name and image", testRejectMissingDistribution)
	t.Run("should reject CR with duplicate provider IDs", testRejectDuplicateProviderIDs)
	t.Run("should reject CR with invalid provider reference in model", testRejectInvalidModelProviderRef)
}

func testRejectMissingDistribution(t *testing.T) {
	t.Helper()

	cr := &v1alpha2.LlamaStackDistribution{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "webhook-test-no-dist",
			Namespace: webhookTestNS,
		},
		Spec: v1alpha2.LlamaStackDistributionSpec{
			Distribution: v1alpha2.DistributionSpec{},
		},
	}

	err := TestEnv.Client.Create(TestEnv.Ctx, cr)
	require.Error(t, err, "CR with no distribution name or image should be rejected by CEL validation")
}

func testRejectDuplicateProviderIDs(t *testing.T) {
	t.Helper()

	inferenceJSON := mustMarshalJSON(t, map[string]interface{}{
		"provider": "ollama",
		"id":       "my-inference",
		"endpoint": "http://ollama:11434",
	})
	safetyJSON := mustMarshalJSON(t, map[string]interface{}{
		"provider": "llama-guard",
		"id":       "my-inference",
	})

	cr := &v1alpha2.LlamaStackDistribution{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "webhook-test-dup-ids",
			Namespace: webhookTestNS,
		},
		Spec: v1alpha2.LlamaStackDistributionSpec{
			Distribution: v1alpha2.DistributionSpec{
				Name: "starter",
			},
			Providers: &v1alpha2.ProvidersSpec{
				Inference: &apiextensionsv1.JSON{Raw: inferenceJSON},
				Safety:    &apiextensionsv1.JSON{Raw: safetyJSON},
			},
		},
	}

	err := TestEnv.Client.Create(TestEnv.Ctx, cr)
	require.Error(t, err, "CR with duplicate provider IDs across API types should be rejected")
	if err != nil {
		defer TestEnv.Client.Delete(TestEnv.Ctx, cr)
	}
}

func testRejectInvalidModelProviderRef(t *testing.T) {
	t.Helper()

	inferenceJSON := mustMarshalJSON(t, map[string]interface{}{
		"provider": "ollama",
		"endpoint": "http://ollama:11434",
	})
	modelJSON := mustMarshalJSON(t, map[string]interface{}{
		"name":     "llama3.2:1b",
		"provider": "nonexistent-provider",
	})

	cr := &v1alpha2.LlamaStackDistribution{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "webhook-test-bad-ref",
			Namespace: webhookTestNS,
		},
		Spec: v1alpha2.LlamaStackDistributionSpec{
			Distribution: v1alpha2.DistributionSpec{
				Name: "starter",
			},
			Providers: &v1alpha2.ProvidersSpec{
				Inference: &apiextensionsv1.JSON{Raw: inferenceJSON},
			},
			Resources: &v1alpha2.ResourcesSpec{
				Models: []apiextensionsv1.JSON{{Raw: modelJSON}},
			},
		},
	}

	err := TestEnv.Client.Create(TestEnv.Ctx, cr)
	require.Error(t, err, "CR with model referencing nonexistent provider should be rejected")
	if err != nil {
		defer TestEnv.Client.Delete(TestEnv.Ctx, cr)
	}
}

func mustMarshalJSON(t *testing.T, v interface{}) []byte {
	t.Helper()
	data, err := json.Marshal(v)
	require.NoError(t, err)
	return data
}
