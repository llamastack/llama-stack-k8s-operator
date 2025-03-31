package deploy

import (
	"context"
	"fmt"
	"os"

	"github.com/go-logr/logr"
	llamav1alpha1 "github.com/meta-llama/llama-stack-k8s-operator/api/v1alpha1"
	networkingv1 "k8s.io/api/networking/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// IsOpenShift checks if the cluster is running OpenShift by looking for OpenShift-specific resources.
func IsOpenShift(ctx context.Context, c client.Client) error {
	// Try to get the OpenShift version from the cluster version
	clusterVersion := &unstructured.Unstructured{}
	clusterVersion.SetGroupVersionKind(schema.GroupVersionKind{
		Group:   "config.openshift.io",
		Version: "v1",
		Kind:    "ClusterVersion",
	})

	err := c.Get(ctx, types.NamespacedName{Name: "version"}, clusterVersion)
	return err
}

// ApplyNetworkPolicy creates or updates a NetworkPolicy.
func ApplyNetworkPolicy(ctx context.Context, c client.Client, scheme *runtime.Scheme,
	instance *llamav1alpha1.LlamaStackDistribution, networkPolicy *networkingv1.NetworkPolicy, log logr.Logger) error {
	// Only apply NetworkPolicy if running on OpenShift
	if err := IsOpenShift(ctx, c); err != nil {
		if k8serrors.IsNotFound(err) {
			log.Info("skipping NetworkPolicy creation - not running on OpenShift")
			return nil
		}
		return fmt.Errorf("failed to check if running on OpenShift: %w", err)
	}

	// Set the controller reference
	if err := ctrl.SetControllerReference(instance, networkPolicy, scheme); err != nil {
		return fmt.Errorf("failed to set controller reference: %w", err)
	}

	// Check if the NetworkPolicy already exists
	existing := &networkingv1.NetworkPolicy{}
	err := c.Get(ctx, client.ObjectKeyFromObject(networkPolicy), existing)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			// Create the NetworkPolicy if it doesn't exist
			if err := c.Create(ctx, networkPolicy); err != nil {
				return fmt.Errorf("failed to create NetworkPolicy: %w", err)
			}
			log.Info("Created NetworkPolicy", "name", networkPolicy.Name)
			return nil
		}
		return fmt.Errorf("failed to get NetworkPolicy: %w", err)
	}

	// Update the NetworkPolicy if it exists
	networkPolicy.ResourceVersion = existing.ResourceVersion
	if err := c.Update(ctx, networkPolicy); err != nil {
		return fmt.Errorf("failed to update NetworkPolicy: %w", err)
	}
	log.Info("Updated NetworkPolicy", "name", networkPolicy.Name)
	return nil
}

func GetOperatorNamespace() (string, error) {
	operatorNS, exist := os.LookupEnv("OPERATOR_NAMESPACE")
	if exist && operatorNS != "" {
		return operatorNS, nil
	}
	data, err := os.ReadFile("/var/run/secrets/kubernetes.io/serviceaccount/namespace")
	return string(data), err
}
