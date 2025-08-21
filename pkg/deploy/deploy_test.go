package deploy

import (
	"testing"

	llamav1alpha1 "github.com/llamastack/llama-stack-k8s-operator/api/v1alpha1"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

func TestApplyDeploymentPreservesSelector(t *testing.T) {
	ctx := t.Context()
	logger := logf.Log.WithName("test-apply-deployment")

	instance := &llamav1alpha1.LlamaStackDistribution{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test-instance",
			Namespace: "default",
			UID:       "test-uid",
		},
	}

	deploymentName := "test-deployment-selector"
	namespace := "default"

	// Initial deployment with a specific selector
	initialDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName,
			Namespace: namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "initial"},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "initial"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "llamastack",
							Image: "quay.io/llamastack/llama-stack-k8s-operator:v0.0.1",
						},
					},
				},
			},
		},
	}

	err := ApplyDeployment(ctx, k8sClient, k8sClient.Scheme(), instance, initialDeployment.DeepCopy(), logger)
	require.NoError(t, err)

	// Verify the deployment was created
	foundDeployment := &appsv1.Deployment{}
	err = k8sClient.Get(ctx, types.NamespacedName{Name: deploymentName, Namespace: namespace}, foundDeployment)
	require.NoError(t, err)
	require.NotNil(t, foundDeployment.Spec.Selector)
	require.Equal(t, "initial", foundDeployment.Spec.Selector.MatchLabels["app"])

	// Updated deployment with changes.
	// The fix should preserve the original selector.
	updatedDeployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      deploymentName,
			Namespace: namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{"app": "initial"}, // Must match existing selector
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{"app": "initial"},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "llamastack",
							Image: "quay.io/llamastack/llama-stack-k8s-operator:v0.0.2",
						},
					},
				},
			},
		},
	}

	err = ApplyDeployment(ctx, k8sClient, k8sClient.Scheme(), instance, updatedDeployment.DeepCopy(), logger)
	require.NoError(t, err)

	err = k8sClient.Get(ctx, types.NamespacedName{Name: deploymentName, Namespace: namespace}, foundDeployment)
	require.NoError(t, err)

	// The selector should be preserved from the initial deployment
	require.NotNil(t, foundDeployment.Spec.Selector)
	require.Equal(t, "initial", foundDeployment.Spec.Selector.MatchLabels["app"])

	// And the other updates should be applied
	require.Equal(t, "quay.io/llamastack/llama-stack-k8s-operator:v0.0.2", foundDeployment.Spec.Template.Spec.Containers[0].Image)
}
