//nolint:testpackage
package e2e

import (
	"testing"

	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	apiextv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestValidationSuite(t *testing.T) {
	if TestOpts.SkipValidation {
		t.Skip("Skipping validation test suite")
	}

	t.Run("should validate CRDs", func(t *testing.T) {
		err := validateCRD(TestEnv.Client, TestEnv.Ctx, "llamastackdistributions.llamastack.io")
		require.NoErrorf(t, err, "error in validating CRD: llamastackdistributions.llamastack.io")
	})

	t.Run("should validate CRD has conversion webhook configured", func(t *testing.T) {
		crd := &apiextv1.CustomResourceDefinition{}
		err := TestEnv.Client.Get(TestEnv.Ctx, client.ObjectKey{
			Name: "llamastackdistributions.llamastack.io",
		}, crd)
		require.NoError(t, err)

		require.NotNil(t, crd.Spec.Conversion, "CRD should have conversion config")
		require.Equal(t, apiextv1.WebhookConverter, crd.Spec.Conversion.Strategy,
			"CRD conversion strategy should be Webhook")
		require.NotNil(t, crd.Spec.Conversion.Webhook, "CRD should have webhook config")
		require.NotNil(t, crd.Spec.Conversion.Webhook.ClientConfig, "Webhook should have clientConfig")
	})

	t.Run("should validate CRD serves both v1alpha1 and v1alpha2", func(t *testing.T) {
		crd := &apiextv1.CustomResourceDefinition{}
		err := TestEnv.Client.Get(TestEnv.Ctx, client.ObjectKey{
			Name: "llamastackdistributions.llamastack.io",
		}, crd)
		require.NoError(t, err)

		versions := make(map[string]bool)
		for _, v := range crd.Spec.Versions {
			versions[v.Name] = v.Served
		}
		require.True(t, versions["v1alpha1"], "v1alpha1 should be served")
		require.True(t, versions["v1alpha2"], "v1alpha2 should be served")
	})

	t.Run("should validate operator deployment", func(t *testing.T) {
		deployment, err := GetDeployment(TestEnv.Client, TestEnv.Ctx, "llama-stack-k8s-operator-controller-manager", TestOpts.OperatorNS)
		require.NoError(t, err, "Operator deployment not found")
		require.Equal(t, int32(1), deployment.Status.ReadyReplicas, "Operator deployment not ready")
	})

	t.Run("should validate operator pods", func(t *testing.T) {
		podList := &corev1.PodList{}
		err := TestEnv.Client.List(TestEnv.Ctx, podList, client.InNamespace(TestOpts.OperatorNS))
		require.NoError(t, err)

		operatorPodFound := false
		for _, pod := range podList.Items {
			if pod.Labels["app.kubernetes.io/name"] == "llama-stack-k8s-operator" {
				operatorPodFound = true
				require.Equal(t, corev1.PodRunning, pod.Status.Phase)
				break
			}
		}
		require.True(t, operatorPodFound, "Operator pod not found in namespace %s", TestOpts.OperatorNS)
	})

	t.Run("should validate webhook service exists", func(t *testing.T) {
		svc := &corev1.Service{}
		err := TestEnv.Client.Get(TestEnv.Ctx, client.ObjectKey{
			Namespace: TestOpts.OperatorNS,
			Name:      "llama-stack-k8s-operator-webhook-service",
		}, svc)
		require.NoError(t, err, "Webhook service should exist in operator namespace")
		require.Equal(t, int32(443), svc.Spec.Ports[0].Port, "Webhook service should listen on port 443")
	})

	t.Run("should validate webhook TLS secret exists", func(t *testing.T) {
		secret := &corev1.Secret{}
		err := TestEnv.Client.Get(TestEnv.Ctx, client.ObjectKey{
			Namespace: TestOpts.OperatorNS,
			Name:      "llama-stack-operator-webhook-cert",
		}, secret)
		require.NoError(t, err, "Webhook TLS secret should exist")
		require.Contains(t, secret.Data, "tls.crt", "TLS secret should contain tls.crt")
		require.Contains(t, secret.Data, "tls.key", "TLS secret should contain tls.key")
	})

	t.Run("should validate prerequisites", func(t *testing.T) {
		deployment, err := GetDeployment(TestEnv.Client, TestEnv.Ctx, "ollama-server", ollamaNS)
		require.NoError(t, err, "Ollama deployment not found")
		require.Equal(t, int32(1), deployment.Status.ReadyReplicas, "Ollama deployment not ready")

		podList := &corev1.PodList{}
		err = TestEnv.Client.List(TestEnv.Ctx, podList, client.InNamespace(ollamaNS))
		require.NoError(t, err)
		require.NotEmpty(t, podList.Items, "No Ollama pods found")
		require.Equal(t, corev1.PodRunning, podList.Items[0].Status.Phase, "Ollama pod not running")
	})
}
