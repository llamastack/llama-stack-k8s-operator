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

package v1alpha2

import (
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	// DefaultServerPort is the default port for the LlamaStack server.
	DefaultServerPort int32 = 8321
)

// ---- Spec Types ----

// DistributionSpec identifies the LlamaStack distribution image to deploy.
// Exactly one of name or image must be specified.
// +kubebuilder:validation:XValidation:rule="!(has(self.name) && has(self.image))",message="only one of name or image can be specified"
// +kubebuilder:validation:XValidation:rule="has(self.name) || has(self.image)",message="one of name or image must be specified"
type DistributionSpec struct {
	// Name is the distribution name that maps to supported distributions via distributions.json.
	// +optional
	Name string `json:"name,omitempty"`

	// Image is a direct container image reference.
	// +optional
	Image string `json:"image,omitempty"`
}

// ProvidersSpec contains typed provider slices for each LlamaStack API type.
type ProvidersSpec struct {
	// Inference providers (e.g., vllm, ollama, fireworks).
	// +optional
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:XValidation:rule="self.size() <= 1 || self.all(p, has(p.id))",message="each provider must have an explicit id when multiple providers are specified"
	Inference []ProviderConfig `json:"inference,omitempty"`

	// Safety providers (e.g., llama-guard).
	// +optional
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:XValidation:rule="self.size() <= 1 || self.all(p, has(p.id))",message="each provider must have an explicit id when multiple providers are specified"
	Safety []ProviderConfig `json:"safety,omitempty"`

	// VectorIo providers (e.g., pgvector, chromadb).
	// +optional
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:XValidation:rule="self.size() <= 1 || self.all(p, has(p.id))",message="each provider must have an explicit id when multiple providers are specified"
	VectorIo []ProviderConfig `json:"vectorIo,omitempty"`

	// ToolRuntime providers (e.g., brave-search, rag-runtime).
	// +optional
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:XValidation:rule="self.size() <= 1 || self.all(p, has(p.id))",message="each provider must have an explicit id when multiple providers are specified"
	ToolRuntime []ProviderConfig `json:"toolRuntime,omitempty"`

	// Telemetry providers (e.g., opentelemetry).
	// +optional
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:XValidation:rule="self.size() <= 1 || self.all(p, has(p.id))",message="each provider must have an explicit id when multiple providers are specified"
	Telemetry []ProviderConfig `json:"telemetry,omitempty"`
}

// ProviderConfig defines configuration for a single LlamaStack provider instance.
// +kubebuilder:validation:XValidation:rule="!has(self.id) || self.id != ''",message="id must not be empty if specified"
type ProviderConfig struct {
	// ID is the unique provider identifier. Required when multiple providers are configured
	// for the same API type. Auto-generated from provider field for single-element lists.
	// +optional
	ID string `json:"id,omitempty"`

	// Provider is the provider type (e.g., vllm, ollama, pgvector).
	// Maps to provider_type with "remote::" prefix in config.yaml.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Provider string `json:"provider"`

	// Endpoint is the provider endpoint URL. Maps to config.url in config.yaml.
	// +optional
	Endpoint string `json:"endpoint,omitempty"`

	// APIKey references a Kubernetes Secret for API authentication.
	// Resolved to env var LLSD_<PROVIDER_ID>_API_KEY.
	// +optional
	APIKey *SecretKeyRef `json:"apiKey,omitempty"`

	// SecretRefs contains named secret references for provider-specific connection fields.
	// Each entry is resolved to an env var LLSD_<PROVIDER_ID>_<KEY>.
	// +optional
	// +kubebuilder:validation:MinProperties=1
	SecretRefs map[string]SecretKeyRef `json:"secretRefs,omitempty"`

	// Settings contains provider-specific settings merged into config.
	// No secret resolution is performed on this field.
	// +optional
	Settings *apiextensionsv1.JSON `json:"settings,omitempty"`
}

// SecretKeyRef references a specific key in a Kubernetes Secret.
type SecretKeyRef struct {
	// Name is the name of the Kubernetes Secret.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// Key is the key within the Secret.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Key string `json:"key"`
}

// ResourcesSpec defines declarative registration of models, tools, and shields.
type ResourcesSpec struct {
	// Models to register with inference providers.
	// +optional
	// +kubebuilder:validation:MinItems=1
	Models []ModelConfig `json:"models,omitempty"`

	// Tools are tool group names to register with the toolRuntime provider.
	// +optional
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:items:MinLength=1
	Tools []string `json:"tools,omitempty"`

	// Shields are safety shield names to register with the safety provider.
	// +optional
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:items:MinLength=1
	Shields []string `json:"shields,omitempty"`
}

// ModelConfig defines a model registration with optional provider assignment and metadata.
// +kubebuilder:validation:XValidation:rule="!has(self.provider) || self.provider != ''",message="provider must not be empty if specified"
// +kubebuilder:validation:XValidation:rule="!has(self.modelType) || self.modelType != ''",message="modelType must not be empty if specified"
// +kubebuilder:validation:XValidation:rule="!has(self.quantization) || self.quantization != ''",message="quantization must not be empty if specified"
type ModelConfig struct {
	// Name is the model identifier (e.g., "llama3.2-8b").
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// Provider is the provider ID to register this model with.
	// Defaults to the first inference provider if not specified.
	// +optional
	Provider string `json:"provider,omitempty"`

	// ContextLength is the model context window size.
	// +optional
	ContextLength *int `json:"contextLength,omitempty"`

	// ModelType is the model type classification.
	// +optional
	ModelType string `json:"modelType,omitempty"`

	// Quantization is the quantization method used.
	// +optional
	Quantization string `json:"quantization,omitempty"`
}

// StorageSpec configures state storage backends for the LlamaStack server.
type StorageSpec struct {
	// KV configures the key-value storage backend.
	// +optional
	KV *KVStorageSpec `json:"kv,omitempty"`

	// SQL configures the relational storage backend.
	// +optional
	SQL *SQLStorageSpec `json:"sql,omitempty"`
}

// KVStorageSpec configures the key-value storage backend.
// +kubebuilder:validation:XValidation:rule="self.type != 'redis' || has(self.endpoint)",message="endpoint is required when type is redis"
// +kubebuilder:validation:XValidation:rule="!has(self.endpoint) || self.type == 'redis'",message="endpoint is only valid when type is redis"
// +kubebuilder:validation:XValidation:rule="!has(self.password) || self.type == 'redis'",message="password is only valid when type is redis"
type KVStorageSpec struct {
	// Type is the storage backend type.
	// +optional
	// +kubebuilder:default:="sqlite"
	// +kubebuilder:validation:Enum=sqlite;redis
	Type string `json:"type,omitempty"`

	// Endpoint is the Redis endpoint URL. Required when type is redis.
	// +optional
	Endpoint string `json:"endpoint,omitempty"`

	// Password references a Kubernetes Secret for Redis authentication.
	// +optional
	Password *SecretKeyRef `json:"password,omitempty"`
}

// SQLStorageSpec configures the relational storage backend.
// +kubebuilder:validation:XValidation:rule="self.type != 'postgres' || has(self.connectionString)",message="connectionString is required when type is postgres"
// +kubebuilder:validation:XValidation:rule="!has(self.connectionString) || self.type == 'postgres'",message="connectionString is only valid when type is postgres"
type SQLStorageSpec struct {
	// Type is the storage backend type.
	// +optional
	// +kubebuilder:default:="sqlite"
	// +kubebuilder:validation:Enum=sqlite;postgres
	Type string `json:"type,omitempty"`

	// ConnectionString references a Kubernetes Secret containing the database connection string.
	// Required when type is postgres.
	// +optional
	ConnectionString *SecretKeyRef `json:"connectionString,omitempty"`
}

// NetworkingSpec configures network access for the LlamaStack service.
type NetworkingSpec struct {
	// Port is the server listen port.
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	// +kubebuilder:default:=8321
	Port int32 `json:"port,omitempty"`

	// TLS configures TLS for the server.
	// +optional
	TLS *TLSSpec `json:"tls,omitempty"`

	// Expose controls external service exposure via Ingress/Route.
	// +optional
	Expose *ExposeConfig `json:"expose,omitempty"`

	// AllowedFrom defines namespace-based access controls for NetworkPolicy.
	// +optional
	AllowedFrom *AllowedFromSpec `json:"allowedFrom,omitempty"`
}

// TLSSpec configures TLS for the LlamaStack server.
// +kubebuilder:validation:XValidation:rule="!self.enabled || has(self.secretName)",message="secretName is required when TLS is enabled"
// +kubebuilder:validation:XValidation:rule="!has(self.secretName) || self.enabled",message="secretName is only valid when TLS is enabled"
// +kubebuilder:validation:XValidation:rule="!has(self.caBundle) || self.enabled",message="caBundle is only valid when TLS is enabled"
type TLSSpec struct {
	// Enabled activates TLS on the server.
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// SecretName references a Kubernetes TLS Secret. Required when enabled is true.
	// +optional
	SecretName string `json:"secretName,omitempty"`

	// CABundle configures custom CA certificates.
	// +optional
	CABundle *CABundleConfig `json:"caBundle,omitempty"`
}

// CABundleConfig defines the CA bundle configuration for custom certificates.
// +kubebuilder:validation:XValidation:rule="!has(self.configMapNamespace) || self.configMapNamespace != ''",message="configMapNamespace must not be empty if specified"
type CABundleConfig struct {
	// ConfigMapName is the name of the ConfigMap containing CA bundle certificates.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	ConfigMapName string `json:"configMapName"`

	// ConfigMapNamespace is the namespace of the ConfigMap. Defaults to the CR namespace.
	// +optional
	ConfigMapNamespace string `json:"configMapNamespace,omitempty"`

	// ConfigMapKeys specifies keys within the ConfigMap containing CA bundle data.
	// All certificates from these keys will be concatenated. Defaults to ["ca-bundle.crt"].
	// +optional
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:MaxItems=50
	// +kubebuilder:validation:items:MinLength=1
	ConfigMapKeys []string `json:"configMapKeys,omitempty"`
}

// ExposeConfig controls external service exposure via Ingress/Route.
// +kubebuilder:validation:XValidation:rule="!has(self.hostname) || self.hostname != ''",message="hostname must not be empty if specified"
type ExposeConfig struct {
	// Enabled activates external access via Ingress/Route.
	// nil = not specified (no Ingress), false = explicitly disabled, true = create Ingress.
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// Hostname sets a custom hostname for the Ingress/Route.
	// When omitted, an auto-generated hostname is used.
	// +optional
	Hostname string `json:"hostname,omitempty"`
}

// AllowedFromSpec defines namespace-based access controls for NetworkPolicies.
type AllowedFromSpec struct {
	// Namespaces is a list of namespace names allowed to access the service.
	// Use "*" to allow all namespaces.
	// +optional
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:items:MinLength=1
	Namespaces []string `json:"namespaces,omitempty"`

	// Labels is a list of namespace label keys for access control (OR semantics).
	// +optional
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:items:MinLength=1
	Labels []string `json:"labels,omitempty"`
}

// WorkloadSpec configures Kubernetes Deployment settings.
type WorkloadSpec struct {
	// Replicas is the number of Pod replicas.
	// +optional
	// +kubebuilder:validation:Minimum=0
	// +kubebuilder:default:=1
	Replicas *int32 `json:"replicas,omitempty"`

	// Workers configures the number of uvicorn worker processes.
	// +optional
	// +kubebuilder:validation:Minimum=1
	Workers *int32 `json:"workers,omitempty"`

	// Resources configures CPU/memory requests and limits.
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// Autoscaling configures HorizontalPodAutoscaler.
	// +optional
	Autoscaling *AutoscalingSpec `json:"autoscaling,omitempty"`

	// Storage configures the PVC for persistent data.
	// +optional
	Storage *PVCStorageSpec `json:"storage,omitempty"`

	// PodDisruptionBudget controls voluntary disruption tolerance.
	// +optional
	PodDisruptionBudget *PodDisruptionBudgetSpec `json:"podDisruptionBudget,omitempty"`

	// TopologySpreadConstraints defines Pod spreading rules.
	// +optional
	// +kubebuilder:validation:MinItems=1
	TopologySpreadConstraints []corev1.TopologySpreadConstraint `json:"topologySpreadConstraints,omitempty"`

	// Overrides provides low-level Pod customization.
	// +optional
	Overrides *WorkloadOverrides `json:"overrides,omitempty"`
}

// PVCStorageSpec configures persistent volume storage.
// +kubebuilder:validation:XValidation:rule="!has(self.mountPath) || self.mountPath != ''",message="mountPath must not be empty if specified"
type PVCStorageSpec struct {
	// Size is the size of the persistent volume claim.
	// +optional
	Size *resource.Quantity `json:"size,omitempty"`

	// MountPath is the container path where storage is mounted.
	// +optional
	// +kubebuilder:default:="/.llama"
	MountPath string `json:"mountPath,omitempty"`
}

// AutoscalingSpec configures HorizontalPodAutoscaler targets.
// +kubebuilder:validation:XValidation:rule="!has(self.minReplicas) || self.maxReplicas >= *self.minReplicas",message="maxReplicas must be greater than or equal to minReplicas"
type AutoscalingSpec struct {
	// MinReplicas is the lower bound replica count.
	// +optional
	// +kubebuilder:validation:Minimum=1
	MinReplicas *int32 `json:"minReplicas,omitempty"`

	// MaxReplicas is the upper bound replica count.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=1
	MaxReplicas int32 `json:"maxReplicas"`

	// TargetCPUUtilizationPercentage configures CPU-based scaling.
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	TargetCPUUtilizationPercentage *int32 `json:"targetCPUUtilizationPercentage,omitempty"`

	// TargetMemoryUtilizationPercentage configures memory-based scaling.
	// +optional
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	TargetMemoryUtilizationPercentage *int32 `json:"targetMemoryUtilizationPercentage,omitempty"`
}

// PodDisruptionBudgetSpec defines voluntary disruption controls.
// +kubebuilder:validation:XValidation:rule="has(self.minAvailable) || has(self.maxUnavailable)",message="at least one of minAvailable or maxUnavailable must be specified"
// +kubebuilder:validation:XValidation:rule="!(has(self.minAvailable) && has(self.maxUnavailable))",message="minAvailable and maxUnavailable are mutually exclusive"
type PodDisruptionBudgetSpec struct {
	// MinAvailable is the minimum number of pods that must remain available.
	// +optional
	MinAvailable *intstr.IntOrString `json:"minAvailable,omitempty"`

	// MaxUnavailable is the maximum number of pods that can be disrupted simultaneously.
	// +optional
	MaxUnavailable *intstr.IntOrString `json:"maxUnavailable,omitempty"`
}

// WorkloadOverrides provides low-level Pod customization.
// +kubebuilder:validation:XValidation:rule="!has(self.serviceAccountName) || self.serviceAccountName != ''",message="serviceAccountName must not be empty if specified"
type WorkloadOverrides struct {
	// ServiceAccountName allows specifying a custom ServiceAccount.
	// +optional
	ServiceAccountName string `json:"serviceAccountName,omitempty"`

	// TerminationGracePeriodSeconds is the time allowed for graceful pod shutdown.
	// +optional
	TerminationGracePeriodSeconds *int64 `json:"terminationGracePeriodSeconds,omitempty"`

	// Env contains additional environment variables for the container.
	// +optional
	// +kubebuilder:validation:MinItems=1
	Env []corev1.EnvVar `json:"env,omitempty"`

	// Command overrides the container entrypoint.
	// +optional
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:items:MinLength=1
	Command []string `json:"command,omitempty"`

	// Args overrides the container arguments.
	// +optional
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:items:MinLength=1
	Args []string `json:"args,omitempty"`

	// Volumes defines additional volumes for the Pod.
	// +optional
	// +kubebuilder:validation:MinItems=1
	Volumes []corev1.Volume `json:"volumes,omitempty"`

	// VolumeMounts defines additional volume mounts for the container.
	// +optional
	// +kubebuilder:validation:MinItems=1
	VolumeMounts []corev1.VolumeMount `json:"volumeMounts,omitempty"`
}

// OverrideConfigSpec provides a full config.yaml override via user-provided ConfigMap.
// Mutually exclusive with providers, resources, storage, and disabled.
type OverrideConfigSpec struct {
	// ConfigMapName is the name of the ConfigMap containing config.yaml.
	// The ConfigMap must reside in the same namespace as the CR.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	ConfigMapName string `json:"configMapName"`
}

// ExternalProvidersSpec defines external provider injection (from spec 001).
type ExternalProvidersSpec struct {
	// Inference lists external inference providers to inject.
	// +optional
	// +kubebuilder:validation:MinItems=1
	Inference []ExternalProviderRef `json:"inference,omitempty"`
}

// ExternalProviderRef references an external provider image.
type ExternalProviderRef struct {
	// ProviderID is the unique provider identifier.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	ProviderID string `json:"providerId"`

	// Image is the container image containing the provider implementation.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Image string `json:"image"`
}

// ---- Status Types ----

// ResolvedDistributionStatus records the resolved distribution details.
type ResolvedDistributionStatus struct {
	// Image is the resolved container image reference.
	Image string `json:"image,omitempty"`

	// ConfigSource indicates the config origin: "embedded" or "oci-label".
	ConfigSource string `json:"configSource,omitempty"`

	// ConfigHash is the SHA256 hash of the base config used.
	ConfigHash string `json:"configHash,omitempty"`
}

// ConfigGenerationStatus records config generation details.
type ConfigGenerationStatus struct {
	// ConfigMapName is the name of the generated ConfigMap.
	ConfigMapName string `json:"configMapName,omitempty"`

	// GeneratedAt is the timestamp of the last generation.
	GeneratedAt metav1.Time `json:"generatedAt,omitempty"`

	// ProviderCount is the number of configured providers.
	ProviderCount int `json:"providerCount,omitempty"`

	// ResourceCount is the number of registered resources.
	ResourceCount int `json:"resourceCount,omitempty"`

	// ConfigVersion is the detected config.yaml schema version.
	ConfigVersion int `json:"configVersion,omitempty"`
}

// Condition type constants for v1alpha2 status conditions.
const (
	ConditionTypeConfigGenerated   = "ConfigGenerated"
	ConditionTypeDeploymentUpdated = "DeploymentUpdated"
	ConditionTypeAvailable         = "Available"
	ConditionTypeSecretsResolved   = "SecretsResolved"

	// ConfigGenerated reasons
	ReasonConfigGenerationSucceeded = "ConfigGenerationSucceeded"
	ReasonConfigGenerationFailed    = "ConfigGenerationFailed"
	ReasonBaseConfigRequired        = "BaseConfigRequired"

	// DeploymentUpdated reasons
	ReasonDeploymentUpdateSucceeded = "DeploymentUpdateSucceeded"
	ReasonDeploymentUpdateFailed    = "DeploymentUpdateFailed"

	// Available reasons
	ReasonMinimumReplicasAvailable = "MinimumReplicasAvailable"
	ReasonReplicasUnavailable      = "ReplicasUnavailable"

	// SecretsResolved reasons
	ReasonAllSecretsFound = "AllSecretsFound"
	ReasonSecretNotFound  = "SecretNotFound"
)

// DistributionPhase represents the current phase of the LlamaStackDistribution.
// +kubebuilder:validation:Enum=Pending;Initializing;Ready;Failed;Terminating
type DistributionPhase string

const (
	PhasePending      DistributionPhase = "Pending"
	PhaseInitializing DistributionPhase = "Initializing"
	PhaseReady        DistributionPhase = "Ready"
	PhaseFailed       DistributionPhase = "Failed"
	PhaseTerminating  DistributionPhase = "Terminating"
)

// VersionInfo contains version-related information.
type VersionInfo struct {
	// OperatorVersion is the version of the operator managing this distribution.
	OperatorVersion string `json:"operatorVersion,omitempty"`

	// LlamaStackServerVersion is the version of the LlamaStack server.
	LlamaStackServerVersion string `json:"llamaStackServerVersion,omitempty"`

	// LastUpdated represents when the version information was last updated.
	LastUpdated metav1.Time `json:"lastUpdated,omitempty"`
}

// DistributionConfig contains configuration information from the providers endpoint.
type DistributionConfig struct {
	// ActiveDistribution shows which distribution is currently being used.
	ActiveDistribution string `json:"activeDistribution,omitempty"`

	// Providers lists the configured providers.
	Providers []ProviderInfo `json:"providers,omitempty"`

	// AvailableDistributions lists all available distributions and their images.
	AvailableDistributions map[string]string `json:"availableDistributions,omitempty"`
}

// ProviderInfo represents a single provider from the providers endpoint.
type ProviderInfo struct {
	API          string `json:"api"`
	ProviderID   string `json:"provider_id"`
	ProviderType string `json:"provider_type"`
	Config       string `json:"config"`
	Health       string `json:"health"`
}

// ---- Root Types ----

// LlamaStackDistributionSpec defines the desired state of LlamaStackDistribution.
// +kubebuilder:validation:XValidation:rule="!has(self.overrideConfig) || !has(self.providers)",message="overrideConfig and providers are mutually exclusive"
// +kubebuilder:validation:XValidation:rule="!has(self.overrideConfig) || !has(self.resources)",message="overrideConfig and resources are mutually exclusive"
// +kubebuilder:validation:XValidation:rule="!has(self.overrideConfig) || !has(self.storage)",message="overrideConfig and storage are mutually exclusive"
// +kubebuilder:validation:XValidation:rule="!has(self.overrideConfig) || !has(self.disabled)",message="overrideConfig and disabled are mutually exclusive"
// +kubebuilder:validation:XValidation:rule="!has(self.providers) || !has(self.disabled) || !self.disabled.exists(d, d == 'inference') || !has(self.providers.inference) || self.providers.inference.size() == 0",message="inference cannot be both in providers and disabled"
// +kubebuilder:validation:XValidation:rule="!has(self.providers) || !has(self.disabled) || !self.disabled.exists(d, d == 'safety') || !has(self.providers.safety) || self.providers.safety.size() == 0",message="safety cannot be both in providers and disabled"
// +kubebuilder:validation:XValidation:rule="!has(self.providers) || !has(self.disabled) || !self.disabled.exists(d, d == 'vectorIo') || !has(self.providers.vectorIo) || self.providers.vectorIo.size() == 0",message="vectorIo cannot be both in providers and disabled"
// +kubebuilder:validation:XValidation:rule="!has(self.providers) || !has(self.disabled) || !self.disabled.exists(d, d == 'toolRuntime') || !has(self.providers.toolRuntime) || self.providers.toolRuntime.size() == 0",message="toolRuntime cannot be both in providers and disabled"
// +kubebuilder:validation:XValidation:rule="!has(self.providers) || !has(self.disabled) || !self.disabled.exists(d, d == 'telemetry') || !has(self.providers.telemetry) || self.providers.telemetry.size() == 0",message="telemetry cannot be both in providers and disabled"
type LlamaStackDistributionSpec struct {
	// Distribution identifies the LlamaStack distribution to deploy.
	// +kubebuilder:validation:Required
	Distribution DistributionSpec `json:"distribution"`

	// Providers configures LlamaStack providers by API type.
	// Mutually exclusive with overrideConfig.
	// +optional
	Providers *ProvidersSpec `json:"providers,omitempty"`

	// Resources defines models, tools, and shields to register.
	// Mutually exclusive with overrideConfig.
	// +optional
	Resources *ResourcesSpec `json:"resources,omitempty"`

	// Storage configures state storage backends (kv and sql).
	// Mutually exclusive with overrideConfig.
	// +optional
	Storage *StorageSpec `json:"storage,omitempty"`

	// Disabled lists API names to remove from the generated config.
	// Mutually exclusive with overrideConfig.
	// +optional
	Disabled []string `json:"disabled,omitempty"`

	// Networking configures network access for the service.
	// +optional
	Networking *NetworkingSpec `json:"networking,omitempty"`

	// Workload configures Kubernetes Deployment settings.
	// +optional
	Workload *WorkloadSpec `json:"workload,omitempty"`

	// ExternalProviders configures external provider injection (from spec 001).
	// +optional
	ExternalProviders *ExternalProvidersSpec `json:"externalProviders,omitempty"`

	// OverrideConfig provides a full config.yaml override via ConfigMap.
	// Mutually exclusive with providers, resources, storage, and disabled.
	// +optional
	OverrideConfig *OverrideConfigSpec `json:"overrideConfig,omitempty"`
}

// LlamaStackDistributionStatus defines the observed state of LlamaStackDistribution.
type LlamaStackDistributionStatus struct {
	// Phase represents the current phase of the distribution.
	Phase DistributionPhase `json:"phase,omitempty"`

	// Conditions represent the latest available observations of the distribution's state.
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ResolvedDistribution records the resolved distribution details.
	// +optional
	ResolvedDistribution *ResolvedDistributionStatus `json:"resolvedDistribution,omitempty"`

	// ConfigGeneration records config generation details.
	// +optional
	ConfigGeneration *ConfigGenerationStatus `json:"configGeneration,omitempty"`

	// Version contains version information for operator and deployment.
	Version VersionInfo `json:"version,omitempty"`

	// DistributionConfig contains the configuration information from the providers endpoint.
	DistributionConfig DistributionConfig `json:"distributionConfig,omitempty"`

	// AvailableReplicas is the number of available replicas.
	AvailableReplicas int32 `json:"availableReplicas,omitempty"`

	// ServiceURL is the internal Kubernetes service URL.
	ServiceURL string `json:"serviceURL,omitempty"`

	// RouteURL is the external URL when external access is configured.
	// +optional
	RouteURL *string `json:"routeURL,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName=llsd
// +kubebuilder:subresource:status
// +kubebuilder:storageversion
// +kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
// +kubebuilder:printcolumn:name="Operator Version",type="string",JSONPath=".status.version.operatorVersion"
// +kubebuilder:printcolumn:name="Server Version",type="string",JSONPath=".status.version.llamaStackServerVersion"
// +kubebuilder:printcolumn:name="Available",type="integer",JSONPath=".status.availableReplicas"
// +kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// LlamaStackDistribution is the Schema for the llamastackdistributions API.
type LlamaStackDistribution struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LlamaStackDistributionSpec   `json:"spec"`
	Status LlamaStackDistributionStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// LlamaStackDistributionList contains a list of LlamaStackDistribution.
type LlamaStackDistributionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LlamaStackDistribution `json:"items"`
}

func init() { //nolint:gochecknoinits
	SchemeBuilder.Register(&LlamaStackDistribution{}, &LlamaStackDistributionList{})
}
