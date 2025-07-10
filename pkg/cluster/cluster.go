package cluster

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/llamastack/llama-stack-k8s-operator/pkg/deploy"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type ClusterInfo struct {
	OperatorNamespace  string
	DistributionImages map[string]string
}

// NewClusterInfo creates a new ClusterInfo object using embedded distributions data.
func NewClusterInfo(ctx context.Context, client client.Client, embeddedDistributions []byte) (*ClusterInfo, error) {
	operatorNamespace, err := deploy.GetOperatorNamespace()
	if err != nil {
		return nil, fmt.Errorf("failed to find operator namespace: %w", err)
	}

	var distributionImages map[string]string
	if err := json.Unmarshal(embeddedDistributions, &distributionImages); err != nil {
		return nil, fmt.Errorf("failed to parse embedded distributions JSON: %w", err)
	}

	return &ClusterInfo{
		OperatorNamespace:  operatorNamespace,
		DistributionImages: distributionImages,
	}, nil
}
