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
	// DefaultContainerName is the default name for the container.
	DefaultContainerName = "llama-stack"
	// DefaultServerPort is the default port for the server.
	DefaultServerPort int32 = 8321
	// DefaultServicePortName is the default name for the service port.
	DefaultServicePortName = "http"
	// DefaultLabelKey is the default key for labels.
	DefaultLabelKey = "app"
	// DefaultLabelValue is the default value for labels.
	DefaultLabelValue = "llama-stack"
	// DefaultMountPath is the default mount path for storage.
	DefaultMountPath = "/.llama"
	// LlamaStackDistributionKind is the kind name for LlamaStackDistribution resources.
	LlamaStackDistributionKind = "LlamaStackDistribution"
)

var (
	// DefaultStorageSize is the default size for persistent storage.
	DefaultStorageSize = resource.MustParse("10Gi")
	// DefaultServerCPURequest is the default CPU request for the server.
	DefaultServerCPURequest = resource.MustParse("500m")
	// DefaultServerMemoryRequest is the default memory request for the server.
	DefaultServerMemoryRequest = resource.MustParse("1Gi")
)

// --- Condition types, reasons, and event types ---

const (
	// Condition types
	ConditionTypeConfigGenerated  = "ConfigGenerated"
	ConditionTypeDeploymentUpdated = "DeploymentUpdated"
	ConditionTypeAvailable        = "Available"
	ConditionTypeSecretsResolved  = "SecretsResolved"

	// ConfigGenerated reasons
	ReasonConfigGenerationSucceeded = "ConfigGenerationSucceeded"
	ReasonConfigGenerationFailed    = "ConfigGenerationFailed"
	ReasonBaseConfigRequired        = "BaseConfigRequired"
	ReasonUnsupportedConfigVersion  = "UnsupportedConfigVersion"
	ReasonUpgradeConfigFailure      = "UpgradeConfigFailure"
	ReasonMissingProviderForResource = "MissingProviderForResource"

	// DeploymentUpdated reasons
	ReasonDeploymentUpdateSucceeded = "DeploymentUpdateSucceeded"
	ReasonDeploymentUpdateFailed    = "DeploymentUpdateFailed"
	ReasonDeploymentUpdateSkipped   = "DeploymentUpdateSkipped"

	// Available reasons
	ReasonMinimumReplicasAvailable = "MinimumReplicasAvailable"
	ReasonReplicasUnavailable      = "ReplicasUnavailable"
	ReasonRolloutInProgress        = "RolloutInProgress"

	// SecretsResolved reasons
	ReasonAllSecretsFound  = "AllSecretsFound"
	ReasonSecretNotFound   = "SecretNotFound"
	ReasonSecretKeyMissing = "SecretKeyMissing"

	// Event types
	EventConfigGenerated        = "ConfigGenerated"
	EventDeploymentUpdated      = "DeploymentUpdated"
	EventConfigGenerationFailed = "ConfigGenerationFailed"
	EventSecretResolutionFailed = "SecretResolutionFailed"

	// Annotations
	AnnotationConfigHash     = "llamastack.io/config-hash"
	AnnotationV1Alpha2Fields = "llamastack.io/v1alpha2-fields"

	// Labels
	LabelManagedBy = "app.kubernetes.io/managed-by"
	LabelComponent = "app.kubernetes.io/component"
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

// --- Top-level CRD ---

// LlamaStackDistributionSpec defines the desired state of LlamaStackDistribution.
// +kubebuilder:validation:XValidation:rule="!(has(self.providers) && has(self.overrideConfig))",message="providers and overrideConfig are mutually exclusive"
// +kubebuilder:validation:XValidation:rule="!(has(self.resources) && has(self.overrideConfig))",message="resources and overrideConfig are mutually exclusive"
// +kubebuilder:validation:XValidation:rule="!(has(self.storage) && has(self.overrideConfig))",message="storage and overrideConfig are mutually exclusive"
// +kubebuilder:validation:XValidation:rule="!(has(self.disabled) && has(self.overrideConfig))",message="disabled and overrideConfig are mutually exclusive"
type LlamaStackDistributionSpec struct {
	// Distribution identifies the LlamaStack distribution image to deploy.
	Distribution DistributionSpec `json:"distribution"`

	// Providers configures LlamaStack providers (inference, safety, vectorIo, toolRuntime, telemetry).
	// Mutually exclusive with overrideConfig.
	// +optional
	Providers *ProvidersSpec `json:"providers,omitempty"`

	// Resources declares models, tools, and shields to register on startup.
	// Mutually exclusive with overrideConfig.
	// +optional
	Resources *ResourcesSpec `json:"resources,omitempty"`

	// Storage configures state storage backends (kv and sql).
	// Mutually exclusive with overrideConfig.
	// +optional
	Storage *StateStorageSpec `json:"storage,omitempty"`

	// Disabled lists API names to disable (e.g., postTraining, eval).
	// Mutually exclusive with overrideConfig.
	// +optional
	Disabled []string `json:"disabled,omitempty"`

	// Networking configures network settings (port, TLS, expose, allowedFrom).
	// +optional
	Networking *NetworkingSpec `json:"networking,omitempty"`

	// Workload configures Kubernetes Deployment settings.
	// +optional
	Workload *WorkloadSpec `json:"workload,omitempty"`

	// ExternalProviders configures external provider injection (from spec 001).
	// +optional
	ExternalProviders *ExternalProvidersSpec `json:"externalProviders,omitempty"`

	// OverrideConfig provides a user-supplied ConfigMap as config.yaml.
	// Mutually exclusive with providers, resources, storage, and disabled.
	// +optional
	OverrideConfig *OverrideConfigSpec `json:"overrideConfig,omitempty"`
}

// --- Distribution ---

// DistributionSpec identifies the LlamaStack distribution image to deploy.
// +kubebuilder:validation:XValidation:rule="!(has(self.name) && has(self.image))",message="only one of name or image can be specified"
// +kubebuilder:validation:XValidation:rule="has(self.name) || has(self.image)",message="one of name or image must be specified"
type DistributionSpec struct {
	// Name is a distribution name mapped to an image via distributions.json.
	// Mutually exclusive with image.
	// +optional
	Name string `json:"name,omitempty"`

	// Image is a direct container image reference.
	// Mutually exclusive with name.
	// +optional
	Image string `json:"image,omitempty"`
}

// --- Providers ---

// ProvidersSpec configures LlamaStack providers by API type.
// Each field is a list of ProviderConfig; a single provider uses a one-element list.
type ProvidersSpec struct {
	// Inference providers (e.g., vllm, ollama).
	// +optional
	// +kubebuilder:validation:XValidation:rule="self.size() <= 1 || self.all(p, has(p.id))",message="each provider must have an explicit id when multiple providers are specified"
	Inference []ProviderConfig `json:"inference,omitempty"`

	// Safety providers (e.g., llama-guard).
	// +optional
	// +kubebuilder:validation:XValidation:rule="self.size() <= 1 || self.all(p, has(p.id))",message="each provider must have an explicit id when multiple providers are specified"
	Safety []ProviderConfig `json:"safety,omitempty"`

	// VectorIo providers (e.g., pgvector, chromadb).
	// +optional
	// +kubebuilder:validation:XValidation:rule="self.size() <= 1 || self.all(p, has(p.id))",message="each provider must have an explicit id when multiple providers are specified"
	VectorIo []ProviderConfig `json:"vectorIo,omitempty"`

	// ToolRuntime providers (e.g., brave-search, rag-runtime).
	// +optional
	// +kubebuilder:validation:XValidation:rule="self.size() <= 1 || self.all(p, has(p.id))",message="each provider must have an explicit id when multiple providers are specified"
	ToolRuntime []ProviderConfig `json:"toolRuntime,omitempty"`

	// Telemetry providers (e.g., opentelemetry).
	// +optional
	// +kubebuilder:validation:XValidation:rule="self.size() <= 1 || self.all(p, has(p.id))",message="each provider must have an explicit id when multiple providers are specified"
	Telemetry []ProviderConfig `json:"telemetry,omitempty"`
}

// ProviderConfig defines a single LlamaStack provider instance.
type ProviderConfig struct {
	// ID is a unique provider identifier. Required when multiple providers are
	// specified for the same API type. Auto-generated from provider field for
	// single-element lists.
	// +optional
	ID string `json:"id,omitempty"`

	// Provider is the provider type (e.g., vllm, llama-guard, pgvector).
	// Maps to provider_type with "remote::" prefix in config.yaml.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Provider string `json:"provider"`

	// Endpoint is the provider endpoint URL. Maps to config.url in config.yaml.
	// +optional
	Endpoint string `json:"endpoint,omitempty"`

	// APIKey is a secret reference for API authentication.
	// Resolved to env var LLSD_<PROVIDER_ID>_API_KEY.
	// +optional
	APIKey *SecretKeyRef `json:"apiKey,omitempty"`

	// SecretRefs are named secret references for provider-specific connection fields.
	// Each key becomes the env var field suffix: LLSD_<PROVIDER_ID>_<KEY>.
	// +optional
	SecretRefs map[string]SecretKeyRef `json:"secretRefs,omitempty"`

	// Settings is an escape hatch for provider-specific configuration.
	// Merged into the provider's config section in config.yaml.
	// No secret resolution is performed on settings values.
	// +optional
	Settings *apiextensionsv1.JSON `json:"settings,omitempty"`
}

// SecretKeyRef references a specific key in a Kubernetes Secret.
type SecretKeyRef struct {
	// Name is the name of the Secret.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// Key is the key within the Secret.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Key string `json:"key"`
}

// --- Resources ---

// ResourcesSpec declares resources to register on startup.
type ResourcesSpec struct {
	// Models to register with inference providers.
	// +optional
	Models []ModelConfig `json:"models,omitempty"`

	// Tools are tool group names to register with the toolRuntime provider.
	// +optional
	Tools []string `json:"tools,omitempty"`

	// Shields are safety shield names to register with the safety provider.
	// +optional
	Shields []string `json:"shields,omitempty"`
}

// ModelConfig defines a model to register.
type ModelConfig struct {
	// Name is the model identifier (e.g., llama3.2-8b).
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`

	// Provider is the provider ID to register this model with.
	// Defaults to the first inference provider when omitted.
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

// --- Storage ---

// StateStorageSpec configures state storage backends.
type StateStorageSpec struct {
	// KV configures key-value storage (sqlite or redis).
	// +optional
	KV *KVStorageSpec `json:"kv,omitempty"`

	// SQL configures relational storage (sqlite or postgres).
	// +optional
	SQL *SQLStorageSpec `json:"sql,omitempty"`
}

// KVStorageSpec configures key-value storage.
// +kubebuilder:validation:XValidation:rule="!has(self.type) || self.type != 'redis' || has(self.endpoint)",message="endpoint is required when type is redis"
type KVStorageSpec struct {
	// Type is the storage backend type.
	// +optional
	// +kubebuilder:default:="sqlite"
	// +kubebuilder:validation:Enum=sqlite;redis
	Type string `json:"type,omitempty"`

	// Endpoint is the Redis endpoint URL. Required when type is redis.
	// +optional
	Endpoint string `json:"endpoint,omitempty"`

	// Password is a secret reference for Redis authentication.
	// +optional
	Password *SecretKeyRef `json:"password,omitempty"`
}

// SQLStorageSpec configures relational storage.
// +kubebuilder:validation:XValidation:rule="!has(self.type) || self.type != 'postgres' || has(self.connectionString)",message="connectionString is required when type is postgres"
type SQLStorageSpec struct {
	// Type is the storage backend type.
	// +optional
	// +kubebuilder:default:="sqlite"
	// +kubebuilder:validation:Enum=sqlite;postgres
	Type string `json:"type,omitempty"`

	// ConnectionString is a secret reference for the database connection string.
	// Required when type is postgres.
	// +optional
	ConnectionString *SecretKeyRef `json:"connectionString,omitempty"`
}

// --- Networking ---

// NetworkingSpec configures network settings for the LlamaStack service.
type NetworkingSpec struct {
	// Port is the server listen port.
	// +optional
	// +kubebuilder:default:=8321
	Port int32 `json:"port,omitempty"`

	// TLS configures TLS for the server.
	// +optional
	TLS *TLSSpec `json:"tls,omitempty"`

	// Expose controls external service exposure via Ingress/Route.
	// +optional
	Expose *ExposeConfig `json:"expose,omitempty"`

	// AllowedFrom configures namespace-based access control via NetworkPolicy.
	// +optional
	AllowedFrom *AllowedFromSpec `json:"allowedFrom,omitempty"`
}

// TLSSpec configures TLS for the server.
// +kubebuilder:validation:XValidation:rule="!self.enabled || has(self.secretName)",message="secretName is required when TLS is enabled"
type TLSSpec struct {
	// Enabled enables TLS on the server.
	// +optional
	Enabled bool `json:"enabled,omitempty"`

	// SecretName references a Kubernetes TLS Secret. Required when enabled is true.
	// +optional
	SecretName string `json:"secretName,omitempty"`

	// CABundle configures custom CA certificates.
	// +optional
	CABundle *CABundleConfig `json:"caBundle,omitempty"`
}

// CABundleConfig references a ConfigMap containing CA certificates.
type CABundleConfig struct {
	// ConfigMapName is the name of the ConfigMap containing CA bundle certificates.
	// +kubebuilder:validation:Required
	ConfigMapName string `json:"configMapName"`

	// ConfigMapNamespace is the namespace of the ConfigMap.
	// Defaults to the same namespace as the CR.
	// +optional
	ConfigMapNamespace string `json:"configMapNamespace,omitempty"`

	// ConfigMapKeys specifies keys within the ConfigMap containing CA bundle data.
	// Defaults to ["ca-bundle.crt"].
	// +optional
	// +kubebuilder:validation:MaxItems=50
	ConfigMapKeys []string `json:"configMapKeys,omitempty"`
}

// ExposeConfig controls external service exposure via Ingress/Route.
type ExposeConfig struct {
	// Enabled enables external access via Ingress/Route.
	// +optional
	Enabled *bool `json:"enabled,omitempty"`

	// Hostname sets a custom hostname for the Ingress/Route.
	// When omitted, an auto-generated hostname is used.
	// +optional
	Hostname string `json:"hostname,omitempty"`
}

// AllowedFromSpec defines namespace-based access controls for NetworkPolicies.
type AllowedFromSpec struct {
	// Namespaces is an explicit list of namespace names allowed to access the service.
	// +optional
	Namespaces []string `json:"namespaces,omitempty"`

	// Labels is a list of namespace label keys that grant access (OR semantics).
	// +optional
	Labels []string `json:"labels,omitempty"`
}

// --- Workload ---

// WorkloadSpec configures Kubernetes Deployment settings.
type WorkloadSpec struct {
	// Replicas is the number of Pod replicas.
	// +optional
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

	// Storage configures PVC for persistent data.
	// +optional
	Storage *PVCStorageSpec `json:"storage,omitempty"`

	// PodDisruptionBudget controls voluntary disruption tolerance.
	// +optional
	PodDisruptionBudget *PodDisruptionBudgetSpec `json:"podDisruptionBudget,omitempty"`

	// TopologySpreadConstraints defines Pod spreading rules.
	// +optional
	TopologySpreadConstraints []corev1.TopologySpreadConstraint `json:"topologySpreadConstraints,omitempty"`

	// Overrides provides low-level Pod customization.
	// +optional
	Overrides *WorkloadOverrides `json:"overrides,omitempty"`
}

// AutoscalingSpec configures HorizontalPodAutoscaler targets.
type AutoscalingSpec struct {
	// MinReplicas is the lower bound replica count.
	// +optional
	MinReplicas *int32 `json:"minReplicas,omitempty"`

	// MaxReplicas is the upper bound replica count.
	// +kubebuilder:validation:Required
	MaxReplicas int32 `json:"maxReplicas"`

	// TargetCPUUtilizationPercentage configures CPU-based scaling.
	// +optional
	TargetCPUUtilizationPercentage *int32 `json:"targetCPUUtilizationPercentage,omitempty"`

	// TargetMemoryUtilizationPercentage configures memory-based scaling.
	// +optional
	TargetMemoryUtilizationPercentage *int32 `json:"targetMemoryUtilizationPercentage,omitempty"`
}

// PVCStorageSpec configures persistent volume storage.
type PVCStorageSpec struct {
	// Size is the PVC size (e.g., "10Gi").
	// +optional
	Size *resource.Quantity `json:"size,omitempty"`

	// MountPath is where storage is mounted in the container.
	// +optional
	// +kubebuilder:default:="/.llama"
	MountPath string `json:"mountPath,omitempty"`
}

// PodDisruptionBudgetSpec defines voluntary disruption controls.
type PodDisruptionBudgetSpec struct {
	// MinAvailable is the minimum number of pods that must remain available.
	// +optional
	MinAvailable *intstr.IntOrString `json:"minAvailable,omitempty"`

	// MaxUnavailable is the maximum number of pods that can be disrupted.
	// +optional
	MaxUnavailable *intstr.IntOrString `json:"maxUnavailable,omitempty"`
}

// WorkloadOverrides provides low-level Pod customization.
type WorkloadOverrides struct {
	// ServiceAccountName overrides the ServiceAccount.
	// +optional
	ServiceAccountName string `json:"serviceAccountName,omitempty"`

	// Env adds environment variables to the container.
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// Command overrides the container entrypoint.
	// +optional
	Command []string `json:"command,omitempty"`

	// Args overrides the container arguments.
	// +optional
	Args []string `json:"args,omitempty"`

	// Volumes adds volumes to the Pod.
	// +optional
	Volumes []corev1.Volume `json:"volumes,omitempty"`

	// VolumeMounts adds volume mounts to the container.
	// +optional
	VolumeMounts []corev1.VolumeMount `json:"volumeMounts,omitempty"`
}

// --- External Providers (spec 001) ---

// ExternalProvidersSpec configures external provider injection.
type ExternalProvidersSpec struct {
	// Inference external providers.
	// +optional
	Inference []ExternalProviderConfig `json:"inference,omitempty"`
}

// ExternalProviderConfig defines an external provider sidecar.
type ExternalProviderConfig struct {
	// ProviderID is the unique identifier for this provider.
	// +kubebuilder:validation:Required
	ProviderID string `json:"providerId"`

	// Image is the container image for the provider sidecar.
	// +kubebuilder:validation:Required
	Image string `json:"image"`
}

// --- Override Config ---

// OverrideConfigSpec provides a user-supplied ConfigMap as config.yaml.
type OverrideConfigSpec struct {
	// ConfigMapName is the name of the ConfigMap containing config.yaml.
	// Must reside in the same namespace as the CR.
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	ConfigMapName string `json:"configMapName"`
}

// --- Status ---

// ResolvedDistributionStatus tracks the resolved distribution image.
type ResolvedDistributionStatus struct {
	// Image is the resolved container image reference.
	Image string `json:"image,omitempty"`

	// ConfigSource is the origin of the base config: "embedded" or "oci-label".
	ConfigSource string `json:"configSource,omitempty"`

	// ConfigHash is the SHA-256 hash of the base config used.
	ConfigHash string `json:"configHash,omitempty"`
}

// ConfigGenerationStatus tracks config generation details.
type ConfigGenerationStatus struct {
	// ConfigMapName is the name of the generated ConfigMap.
	ConfigMapName string `json:"configMapName,omitempty"`

	// GeneratedAt is when the config was last generated.
	GeneratedAt *metav1.Time `json:"generatedAt,omitempty"`

	// ProviderCount is the number of configured providers.
	ProviderCount int `json:"providerCount,omitempty"`

	// ResourceCount is the number of registered resources.
	ResourceCount int `json:"resourceCount,omitempty"`

	// ConfigVersion is the config.yaml schema version.
	ConfigVersion int `json:"configVersion,omitempty"`
}

// VersionInfo contains version-related information.
type VersionInfo struct {
	// OperatorVersion is the version of the operator.
	OperatorVersion string `json:"operatorVersion,omitempty"`

	// LlamaStackServerVersion is the version of the LlamaStack server.
	LlamaStackServerVersion string `json:"llamaStackServerVersion,omitempty"`

	// LastUpdated is when the version information was last updated.
	LastUpdated metav1.Time `json:"lastUpdated,omitempty"`
}

// ProviderHealthStatus represents the health status of a provider.
type ProviderHealthStatus struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// ProviderInfo represents a single provider from the providers endpoint.
type ProviderInfo struct {
	API          string               `json:"api"`
	ProviderID   string               `json:"provider_id"`
	ProviderType string               `json:"provider_type"`
	Config       apiextensionsv1.JSON `json:"config"`
	Health       ProviderHealthStatus `json:"health"`
}

// DistributionConfig contains configuration from the providers endpoint.
type DistributionConfig struct {
	ActiveDistribution     string            `json:"activeDistribution,omitempty"`
	Providers              []ProviderInfo    `json:"providers,omitempty"`
	AvailableDistributions map[string]string `json:"availableDistributions,omitempty"`
}

// LlamaStackDistributionStatus defines the observed state of LlamaStackDistribution.
type LlamaStackDistributionStatus struct {
	// Phase is the current phase of the distribution.
	Phase DistributionPhase `json:"phase,omitempty"`

	// Version contains version information.
	Version VersionInfo `json:"version,omitempty"`

	// DistributionConfig contains provider configuration from the server.
	DistributionConfig DistributionConfig `json:"distributionConfig,omitempty"`

	// Conditions represent the latest available observations.
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ResolvedDistribution tracks the resolved image and config source.
	ResolvedDistribution *ResolvedDistributionStatus `json:"resolvedDistribution,omitempty"`

	// ConfigGeneration tracks config generation details.
	ConfigGeneration *ConfigGenerationStatus `json:"configGeneration,omitempty"`

	// AvailableReplicas is the number of available replicas.
	AvailableReplicas int32 `json:"availableReplicas,omitempty"`

	// ServiceURL is the internal Kubernetes service URL.
	ServiceURL string `json:"serviceURL,omitempty"`

	// RouteURL is the external URL (when expose is enabled).
	// +optional
	RouteURL *string `json:"routeURL,omitempty"`
}

// --- Root types ---

//+kubebuilder:object:root=true
//+kubebuilder:resource:shortName=llsd
//+kubebuilder:subresource:status
//+kubebuilder:storageversion
//+kubebuilder:printcolumn:name="Phase",type="string",JSONPath=".status.phase"
//+kubebuilder:printcolumn:name="Distribution",type="string",JSONPath=".status.resolvedDistribution.image",priority=1
//+kubebuilder:printcolumn:name="Config",type="string",JSONPath=".status.configGeneration.configMapName",priority=1
//+kubebuilder:printcolumn:name="Providers",type="integer",JSONPath=".status.configGeneration.providerCount"
//+kubebuilder:printcolumn:name="Available",type="integer",JSONPath=".status.availableReplicas"
//+kubebuilder:printcolumn:name="Age",type="date",JSONPath=".metadata.creationTimestamp"

// LlamaStackDistribution is the Schema for the llamastackdistributions API.
type LlamaStackDistribution struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   LlamaStackDistributionSpec   `json:"spec"`
	Status LlamaStackDistributionStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// LlamaStackDistributionList contains a list of LlamaStackDistribution.
type LlamaStackDistributionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []LlamaStackDistribution `json:"items"`
}

func init() { //nolint:gochecknoinits
	SchemeBuilder.Register(&LlamaStackDistribution{}, &LlamaStackDistributionList{})
}
