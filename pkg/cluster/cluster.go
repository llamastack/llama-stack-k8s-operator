package cluster

import (
	"context"
	"fmt"

	"github.com/llamastack/llama-stack-k8s-operator/pkg/deploy"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	distributionConfigMapName = "llama-stack-k8s-operator-distribution-images"
)

type ClusterInfo struct {
	OperatorNamespace  string
	DistributionImages map[string]string
}

// NewClusterInfo creates a new ClusterInfo object.
func NewClusterInfo(ctx context.Context, client client.Client) (*ClusterInfo, error) {
	clusterInfo := &ClusterInfo{}
	var err error

	clusterInfo.OperatorNamespace, err = deploy.GetOperatorNamespace()
	if err != nil {
		return clusterInfo, fmt.Errorf("failed to find operator namespace: %w", err)
	}

	configMap := &corev1.ConfigMap{}
	if err = client.Get(ctx, types.NamespacedName{
		Name:      distributionConfigMapName,
		Namespace: clusterInfo.OperatorNamespace,
	}, configMap); err != nil {
		return clusterInfo, fmt.Errorf("failed to get distribution ConfigMap: %w", err)
	}

	clusterInfo.DistributionImages = make(map[string]string)
	for k, v := range configMap.Data {
		clusterInfo.DistributionImages[k] = v
	}

	return clusterInfo, nil
}
