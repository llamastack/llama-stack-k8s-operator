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

package controllers_test

//revive:disable:dot-imports
import (
	"context"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	llamaxk8siov1alpha1 "github.com/llamastack/llama-stack-k8s-operator/api/v1alpha1"
	llamaxk8siov1alpha2 "github.com/llamastack/llama-stack-k8s-operator/api/v1alpha2"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/utils/ptr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
)

var cfg *rest.Config
var k8sClient client.Client
var testEnv *envtest.Environment
var mgrCancel context.CancelFunc

// loadCRDWithConversion reads the base CRD and adds conversion webhook config
// so that envtest's API server routes conversion requests to the local webhook server.
func loadCRDWithConversion() []*apiextensionsv1.CustomResourceDefinition {
	crdPath := filepath.Join("..", "config", "crd", "bases", "llamastack.io_llamastackdistributions.yaml")
	data, err := os.ReadFile(crdPath)
	if err != nil {
		logf.Log.Error(err, "failed to read CRD file")
		os.Exit(1)
	}

	crdScheme := runtime.NewScheme()
	if schemeErr := apiextensionsv1.AddToScheme(crdScheme); schemeErr != nil {
		logf.Log.Error(schemeErr, "failed to register apiextensions scheme")
		os.Exit(1)
	}
	codecFactory := serializer.NewCodecFactory(crdScheme)
	decoder := codecFactory.UniversalDeserializer()

	obj, _, decodeErr := decoder.Decode(data, nil, nil)
	if decodeErr != nil {
		logf.Log.Error(decodeErr, "failed to decode CRD")
		os.Exit(1)
	}
	crd, ok := obj.(*apiextensionsv1.CustomResourceDefinition)
	if !ok {
		logf.Log.Error(nil, "decoded object is not a CRD")
		os.Exit(1)
	}

	crd.Spec.Conversion = &apiextensionsv1.CustomResourceConversion{
		Strategy: apiextensionsv1.WebhookConverter,
		Webhook: &apiextensionsv1.WebhookConversion{
			ClientConfig: &apiextensionsv1.WebhookClientConfig{
				Service: &apiextensionsv1.ServiceReference{
					Namespace: "system",
					Name:      "webhook-service",
					Path:      ptr.To("/convert"),
				},
			},
			ConversionReviewVersions: []string{"v1"},
		},
	}

	return []*apiextensionsv1.CustomResourceDefinition{crd}
}

// TestMain sets up the shared test environment for controller tests.
func TestMain(m *testing.M) {
	logf.SetLogger(zap.New(zap.UseDevMode(true)))

	// Register all schemes BEFORE starting envtest so that
	// modifyConversionWebhooks can find the convertible types.
	registerTestSchemes()

	testEnv = &envtest.Environment{
		CRDInstallOptions: envtest.CRDInstallOptions{
			CRDs:   loadCRDWithConversion(),
			Scheme: scheme.Scheme,
		},
		BinaryAssetsDirectory: os.Getenv("KUBEBUILDER_ASSETS"),
		WebhookInstallOptions: envtest.WebhookInstallOptions{
			Paths: []string{filepath.Join("..", "config", "webhook")},
		},
	}

	var err error
	cfg, err = testEnv.Start()
	if err != nil {
		logf.Log.Error(err, "failed to start test environment")
		os.Exit(1)
	}

	// Start a manager to serve webhooks (conversion + validation).
	// The conversion webhook is required because v1alpha2 is the storage version
	// and tests create v1alpha1 objects that need field-level conversion.
	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
		WebhookServer: webhook.NewServer(webhook.Options{
			Host:    testEnv.WebhookInstallOptions.LocalServingHost,
			Port:    testEnv.WebhookInstallOptions.LocalServingPort,
			CertDir: testEnv.WebhookInstallOptions.LocalServingCertDir,
		}),
		Metrics: metricsserver.Options{BindAddress: "0"},
	})
	if err != nil {
		logf.Log.Error(err, "failed to create manager for webhooks")
		os.Exit(1)
	}

	if webhookErr := llamaxk8siov1alpha2.SetupWebhookWithManager(mgr); webhookErr != nil {
		logf.Log.Error(webhookErr, "failed to set up webhooks")
		os.Exit(1)
	}

	var ctx context.Context
	ctx, mgrCancel = context.WithCancel(context.Background())
	go func() {
		if startErr := mgr.Start(ctx); startErr != nil {
			logf.Log.Error(startErr, "failed to start manager")
		}
	}()

	// Wait for the webhook server to be ready before running tests.
	webhookAddr := net.JoinHostPort(
		testEnv.WebhookInstallOptions.LocalServingHost,
		strconv.Itoa(testEnv.WebhookInstallOptions.LocalServingPort))
	dialer := &net.Dialer{Timeout: 1 * time.Second}
	maxRetries := 20
	for i := range maxRetries {
		conn, connErr := dialer.DialContext(context.Background(), "tcp", webhookAddr)
		if connErr == nil {
			conn.Close()
			break
		}
		if i == maxRetries-1 {
			logf.Log.Error(connErr, "timed out waiting for webhook server to be ready")
			os.Exit(1)
		}
		time.Sleep(500 * time.Millisecond)
	}

	k8sClient, err = client.New(cfg, client.Options{Scheme: scheme.Scheme})
	if err != nil {
		logf.Log.Error(err, "failed to create client")
		os.Exit(1)
	}

	code := m.Run()

	mgrCancel()
	err = testEnv.Stop()
	if err != nil {
		logf.Log.Error(err, "failed to stop test environment")
		os.Exit(1)
	}

	os.Exit(code)
}

func registerTestSchemes() {
	schemes := []func(s *runtime.Scheme) error{
		llamaxk8siov1alpha1.AddToScheme,
		llamaxk8siov1alpha2.AddToScheme,
		corev1.AddToScheme,
		appsv1.AddToScheme,
		networkingv1.AddToScheme,
		rbacv1.AddToScheme,
	}
	for _, addToScheme := range schemes {
		if err := addToScheme(scheme.Scheme); err != nil {
			logf.Log.Error(err, "failed to register scheme")
			os.Exit(1)
		}
	}
}
