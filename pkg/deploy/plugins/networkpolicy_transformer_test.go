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

package plugins

import (
	"testing"

	llamav1alpha1 "github.com/llamastack/llama-stack-k8s-operator/api/v1alpha1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sigs.k8s.io/kustomize/api/resmap"
	"sigs.k8s.io/kustomize/api/resource"
)

const networkPolicyTestYAML = `
apiVersion: networking.k8s.io/v1
kind: NetworkPolicy
metadata:
  name: test-network-policy
spec:
  podSelector:
    matchLabels:
      app: llama-stack
  policyTypes:
  - Ingress
  ingress: []
`

func TestNetworkPolicyTransformer_Default(t *testing.T) {
	rf := resource.NewFactory(nil)
	res, err := rf.FromBytes([]byte(networkPolicyTestYAML))
	require.NoError(t, err)

	rm := resmap.New()
	require.NoError(t, rm.Append(res))

	transformer := CreateNetworkPolicyTransformer(NetworkPolicyTransformerConfig{
		InstanceName:      "test-instance",
		ServicePort:       8321,
		OperatorNamespace: "operator-ns",
		NetworkSpec:       nil,
	})

	err = transformer.Transform(rm)
	require.NoError(t, err)

	transformedRes := rm.Resources()[0]
	yamlBytes, err := transformedRes.AsYAML()
	require.NoError(t, err)

	yamlStr := string(yamlBytes)

	assert.Contains(t, yamlStr, "app.kubernetes.io/instance: test-instance")
	assert.Contains(t, yamlStr, "app.kubernetes.io/part-of: llama-stack")
	assert.Contains(t, yamlStr, "kubernetes.io/metadata.name: operator-ns")
	assert.Contains(t, yamlStr, "port: 8321")

	// Without AllowedTo, egress should be unrestricted (no Egress policyType, no egress rules)
	assert.NotContains(t, yamlStr, "Egress")
	assert.NotContains(t, yamlStr, "k8s-app: kube-dns")
	assert.NotContains(t, yamlStr, "cidr:")
}

func TestNetworkPolicyTransformer_AllNamespaces(t *testing.T) {
	rf := resource.NewFactory(nil)
	res, err := rf.FromBytes([]byte(networkPolicyTestYAML))
	require.NoError(t, err)

	rm := resmap.New()
	require.NoError(t, rm.Append(res))

	transformer := CreateNetworkPolicyTransformer(NetworkPolicyTransformerConfig{
		InstanceName:      "test-instance",
		ServicePort:       8321,
		OperatorNamespace: "operator-ns",
		NetworkSpec: &llamav1alpha1.NetworkSpec{
			AllowedFrom: &llamav1alpha1.AllowedFromSpec{
				Namespaces: []string{"*"},
			},
		},
	})

	err = transformer.Transform(rm)
	require.NoError(t, err)

	transformedRes := rm.Resources()[0]
	yamlBytes, err := transformedRes.AsYAML()
	require.NoError(t, err)

	yamlStr := string(yamlBytes)

	assert.Contains(t, yamlStr, "namespaceSelector: {}")
	assert.NotContains(t, yamlStr, "kubernetes.io/metadata.name: operator-ns")
}

func TestNetworkPolicyTransformer_ExplicitNamespaces(t *testing.T) {
	rf := resource.NewFactory(nil)
	res, err := rf.FromBytes([]byte(networkPolicyTestYAML))
	require.NoError(t, err)

	rm := resmap.New()
	require.NoError(t, rm.Append(res))

	transformer := CreateNetworkPolicyTransformer(NetworkPolicyTransformerConfig{
		InstanceName:      "test-instance",
		ServicePort:       8321,
		OperatorNamespace: "operator-ns",
		NetworkSpec: &llamav1alpha1.NetworkSpec{
			AllowedFrom: &llamav1alpha1.AllowedFromSpec{
				Namespaces: []string{"ns-a", "ns-b"},
			},
		},
	})

	err = transformer.Transform(rm)
	require.NoError(t, err)

	transformedRes := rm.Resources()[0]
	yamlBytes, err := transformedRes.AsYAML()
	require.NoError(t, err)

	yamlStr := string(yamlBytes)

	assert.Contains(t, yamlStr, "kubernetes.io/metadata.name: ns-a")
	assert.Contains(t, yamlStr, "kubernetes.io/metadata.name: ns-b")
	assert.Contains(t, yamlStr, "kubernetes.io/metadata.name: operator-ns")
}

func TestNetworkPolicyTransformer_LabelSelectors(t *testing.T) {
	rf := resource.NewFactory(nil)
	res, err := rf.FromBytes([]byte(networkPolicyTestYAML))
	require.NoError(t, err)

	rm := resmap.New()
	require.NoError(t, rm.Append(res))

	transformer := CreateNetworkPolicyTransformer(NetworkPolicyTransformerConfig{
		InstanceName:      "test-instance",
		ServicePort:       8321,
		OperatorNamespace: "operator-ns",
		NetworkSpec: &llamav1alpha1.NetworkSpec{
			AllowedFrom: &llamav1alpha1.AllowedFromSpec{
				Labels: []string{"myproject/lls-allowed", "team/authorized"},
			},
		},
	})

	err = transformer.Transform(rm)
	require.NoError(t, err)

	transformedRes := rm.Resources()[0]
	yamlBytes, err := transformedRes.AsYAML()
	require.NoError(t, err)

	yamlStr := string(yamlBytes)

	assert.Contains(t, yamlStr, "key: myproject/lls-allowed")
	assert.Contains(t, yamlStr, "key: team/authorized")
	assert.Contains(t, yamlStr, "operator: Exists")
}

func TestNetworkPolicyTransformer_CustomPort(t *testing.T) {
	rf := resource.NewFactory(nil)
	res, err := rf.FromBytes([]byte(networkPolicyTestYAML))
	require.NoError(t, err)

	rm := resmap.New()
	require.NoError(t, rm.Append(res))

	transformer := CreateNetworkPolicyTransformer(NetworkPolicyTransformerConfig{
		InstanceName:      "test-instance",
		ServicePort:       9000,
		OperatorNamespace: "operator-ns",
		NetworkSpec:       nil,
	})

	err = transformer.Transform(rm)
	require.NoError(t, err)

	transformedRes := rm.Resources()[0]
	yamlBytes, err := transformedRes.AsYAML()
	require.NoError(t, err)

	yamlStr := string(yamlBytes)

	assert.Contains(t, yamlStr, "port: 9000")
}

func TestNetworkPolicyTransformer_RouterPeersWhenNetworkSpecProvided(t *testing.T) {
	rf := resource.NewFactory(nil)
	res, err := rf.FromBytes([]byte(networkPolicyTestYAML))
	require.NoError(t, err)

	rm := resmap.New()
	require.NoError(t, rm.Append(res))

	transformer := CreateNetworkPolicyTransformer(NetworkPolicyTransformerConfig{
		InstanceName:      "test-instance",
		ServicePort:       8321,
		OperatorNamespace: "operator-ns",
		NetworkSpec: &llamav1alpha1.NetworkSpec{
			ExposeRoute: true,
		},
	})

	err = transformer.Transform(rm)
	require.NoError(t, err)

	transformedRes := rm.Resources()[0]
	yamlBytes, err := transformedRes.AsYAML()
	require.NoError(t, err)

	yamlStr := string(yamlBytes)

	assert.Contains(t, yamlStr, "network.openshift.io/policy-group: ingress")
}

func TestNetworkPolicyTransformer_NoRouterPeersWhenNetworkSpecNil(t *testing.T) {
	rf := resource.NewFactory(nil)
	res, err := rf.FromBytes([]byte(networkPolicyTestYAML))
	require.NoError(t, err)

	rm := resmap.New()
	require.NoError(t, rm.Append(res))

	transformer := CreateNetworkPolicyTransformer(NetworkPolicyTransformerConfig{
		InstanceName:      "test-instance",
		ServicePort:       8321,
		OperatorNamespace: "operator-ns",
		NetworkSpec:       nil,
	})

	err = transformer.Transform(rm)
	require.NoError(t, err)

	transformedRes := rm.Resources()[0]
	yamlBytes, err := transformedRes.AsYAML()
	require.NoError(t, err)

	yamlStr := string(yamlBytes)

	assert.NotContains(t, yamlStr, "network.openshift.io/policy-group: ingress")
}

func TestNetworkPolicyTransformer_AllowedTo(t *testing.T) {
	rf := resource.NewFactory(nil)
	res, err := rf.FromBytes([]byte(networkPolicyTestYAML))
	require.NoError(t, err)

	rm := resmap.New()
	require.NoError(t, rm.Append(res))

	ollamaPort := int32(11434)
	transformer := CreateNetworkPolicyTransformer(NetworkPolicyTransformerConfig{
		InstanceName:      "test-instance",
		ServicePort:       8321,
		OperatorNamespace: "operator-ns",
		APIServerHost:     "10.96.0.1",
		APIServerPort:     443,
		NetworkSpec: &llamav1alpha1.NetworkSpec{
			AllowedTo: []llamav1alpha1.EgressRule{
				{Namespace: "ollama-dist", Port: &ollamaPort},
			},
		},
	})

	err = transformer.Transform(rm)
	require.NoError(t, err)

	transformedRes := rm.Resources()[0]
	yamlBytes, err := transformedRes.AsYAML()
	require.NoError(t, err)

	yamlStr := string(yamlBytes)

	// Egress policyType should be added
	assert.Contains(t, yamlStr, "Egress")

	// DNS egress rules
	assert.Contains(t, yamlStr, "k8s-app: kube-dns")
	assert.Contains(t, yamlStr, "kubernetes.io/metadata.name: kube-system")
	assert.Contains(t, yamlStr, "port: 53")

	// API server egress rule
	assert.Contains(t, yamlStr, "cidr: 10.96.0.1/32")
	assert.Contains(t, yamlStr, "port: 443")

	// User-specified destination
	assert.Contains(t, yamlStr, "kubernetes.io/metadata.name: ollama-dist")
	assert.Contains(t, yamlStr, "port: 11434")
}

func TestNetworkPolicyTransformer_AllowedToWithoutPort(t *testing.T) {
	rf := resource.NewFactory(nil)
	res, err := rf.FromBytes([]byte(networkPolicyTestYAML))
	require.NoError(t, err)

	rm := resmap.New()
	require.NoError(t, rm.Append(res))

	transformer := CreateNetworkPolicyTransformer(NetworkPolicyTransformerConfig{
		InstanceName:      "test-instance",
		ServicePort:       8321,
		OperatorNamespace: "operator-ns",
		APIServerHost:     "10.96.0.1",
		APIServerPort:     443,
		NetworkSpec: &llamav1alpha1.NetworkSpec{
			AllowedTo: []llamav1alpha1.EgressRule{
				{Namespace: "model-serving"},
			},
		},
	})

	err = transformer.Transform(rm)
	require.NoError(t, err)

	transformedRes := rm.Resources()[0]
	yamlBytes, err := transformedRes.AsYAML()
	require.NoError(t, err)

	yamlStr := string(yamlBytes)

	assert.Contains(t, yamlStr, "Egress")
	assert.Contains(t, yamlStr, "kubernetes.io/metadata.name: model-serving")
}
