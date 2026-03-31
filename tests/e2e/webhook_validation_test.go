//nolint:testpackage
package e2e

import (
	"testing"

	"github.com/llamastack/llama-stack-k8s-operator/api/v1alpha2"
	"github.com/stretchr/testify/require"
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
				Inference: []v1alpha2.ProviderConfig{
					{ID: "my-inference", Provider: "ollama", Endpoint: "http://ollama:11434"},
				},
				Telemetry: []v1alpha2.ProviderConfig{
					{ID: "my-inference", Provider: "otel"},
				},
			},
		},
	}

	err := TestEnv.Client.Create(TestEnv.Ctx, cr)
	require.Error(t, err, "CR with duplicate provider IDs across API types should be rejected")
	if err == nil {
		defer TestEnv.Client.Delete(TestEnv.Ctx, cr)
	}
}

func testRejectInvalidModelProviderRef(t *testing.T) {
	t.Helper()

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
				Inference: []v1alpha2.ProviderConfig{
					{Provider: "ollama", Endpoint: "http://ollama:11434"},
				},
			},
			Resources: &v1alpha2.ResourcesSpec{
				Models: []v1alpha2.ModelConfig{
					{Name: "llama3.2:1b", Provider: "nonexistent-provider"},
				},
			},
		},
	}

	err := TestEnv.Client.Create(TestEnv.Ctx, cr)
	require.Error(t, err, "CR with model referencing nonexistent provider should be rejected")
	if err == nil {
		defer TestEnv.Client.Delete(TestEnv.Ctx, cr)
	}
}
