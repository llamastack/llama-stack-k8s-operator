package featureflags

// FeatureFlags represents the configuration for feature flags in the operator.
type FeatureFlags struct {
	// EnableNetworkPolicy controls whether NetworkPolicy resources should be created.
	EnableNetworkPolicy string `yaml:"enableNetworkPolicy"`
}

const (
	// FeatureFlagsKey is the key used in the ConfigMap to store feature flags.
	FeatureFlagsKey = "featureFlags"
	// EnableNetworkPolicyKey is the key for the network policy feature flag.
	EnableNetworkPolicyKey = "enableNetworkPolicy"
	// NetworkPolicyDefaultValue is the default value for the network policy feature flag.
	NetworkPolicyDefaultValue = false
)
