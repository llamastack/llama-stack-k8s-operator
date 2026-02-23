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

// -----------------------------------------------------------------------------
// Constants
// -----------------------------------------------------------------------------

const (
	DefaultContainerName         = "llama-stack"
	DefaultServerPort      int32 = 8321
	DefaultServicePortName       = "http"
	DefaultLabelKey              = "app"
	DefaultLabelValue            = "llama-stack"
	DefaultMountPath             = "/.llama"

	LlamaStackDistributionKind = "LlamaStackDistribution"
)

var (
	DefaultStorageSize         = resource.MustParse("10Gi")
	DefaultServerCPURequest    = resource.MustParse("500m")
	DefaultServerMemoryRequest = resource.MustParse("1Gi")
)

// -----------------------------------------------------------------------------
// Distribution Types
// -----------------------------------------------------------------------------

// DistributionSpec identifies the LlamaStack distribution image to deploy.
// Exactly one of name or image must be specified.
// +kubebuilder:validation:XValidation:rule="!(has(self.name) && has(self.image))",message="Only one of name or image can be specified"
// +kubebuilder:validation:XValidation:rule="has(self.name) || has(self.image)",message="One of name or image must be specified"
type DistributionSpec struct {
	// Name is the distribution name that maps to a supported distribution (e.g., "starter", "remote-vllm").
	// Resolved to a container image via distributions.json and image-overrides.
	// +optional
	Name string `json:"name,omitempty"`
	// Image is a direct container image reference to use.
	// +optional
	Image string `json:"image,omitempty"`
}

// -----------------------------------------------------------------------------
// Provider Types (Task 1.2)
// -----------------------------------------------------------------------------

// SecretKeyRef references a specific key in a Kubernetes Secret.
type SecretKeyRef struct {
	// Name is the name of the Secret.
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
	// Key is the key within the Secret.
	// +kubebuilder:validation:MinLength=1
	Key string `json:"key"`
}

// ProviderConfig defines the configuration for a single LlamaStack provider instance.
// This type is used for JSON parsing of the polymorphic provider fields.
type ProviderConfig struct {
	// ID is a unique provider identifier. Required when multiple providers are
	// configured for the same API type. Auto-generated from provider when omitted
	// for single-provider configurations.
	// +optional
	ID string `json:"id,omitempty"`
	// Provider is the provider type (e.g., "vllm", "llama-guard", "pgvector").
	// Maps to provider_type with "remote::" prefix in config.yaml.
	// +kubebuilder:validation:MinLength=1
	Provider string `json:"provider"`
	// Endpoint is the provider endpoint URL.
	// +optional
	Endpoint string `json:"endpoint,omitempty"`
	// ApiKey references a Secret containing the API key for authentication.
	// +optional
	ApiKey *SecretKeyRef `json:"apiKey,omitempty"`
	// Settings contains provider-specific settings merged into the provider's
	// config section in config.yaml. Acts as an escape hatch for fields not
	// directly exposed in the CRD schema. The operator recognizes secretKeyRef
	// objects only at the top level of settings values.
	// +optional
	Settings *apiextensionsv1.JSON `json:"settings,omitempty"`
}

// ProvidersSpec defines provider configurations by API type.
// Each field supports polymorphic form: a single ProviderConfig object
// or a list of ProviderConfig objects with explicit id fields.
type ProvidersSpec struct {
	// Inference configures inference providers (e.g., vLLM, TGI).
	// +optional
	Inference *apiextensionsv1.JSON `json:"inference,omitempty"`
	// Safety configures safety providers (e.g., llama-guard).
	// +optional
	Safety *apiextensionsv1.JSON `json:"safety,omitempty"`
	// VectorIo configures vector I/O providers (e.g., pgvector, chromadb).
	// +optional
	VectorIo *apiextensionsv1.JSON `json:"vectorIo,omitempty"`
	// ToolRuntime configures tool runtime providers.
	// +optional
	ToolRuntime *apiextensionsv1.JSON `json:"toolRuntime,omitempty"`
	// Telemetry configures telemetry providers (e.g., opentelemetry).
	// +optional
	Telemetry *apiextensionsv1.JSON `json:"telemetry,omitempty"`
}

// -----------------------------------------------------------------------------
// Resource Types (Task 1.3)
// -----------------------------------------------------------------------------

// ModelConfig defines a model registration with optional provider assignment.
// This type is used for JSON parsing of polymorphic model entries.
type ModelConfig struct {
	// Name is the model identifier (e.g., "llama3.2-8b").
	// +kubebuilder:validation:MinLength=1
	Name string `json:"name"`
	// Provider is the ID of the provider to register this model with.
	// Defaults to the first inference provider when omitted.
	// +optional
	Provider string `json:"provider,omitempty"`
	// ContextLength is the model context window size.
	// +optional
	ContextLength int `json:"contextLength,omitempty"`
	// ModelType is the model type classification.
	// +optional
	ModelType string `json:"modelType,omitempty"`
	// Quantization is the quantization method.
	// +optional
	Quantization string `json:"quantization,omitempty"`
}

// ResourcesSpec defines resources to register with the LlamaStack server.
type ResourcesSpec struct {
	// Models to register. Each item can be a simple string (model name)
	// or a ModelConfig object with provider assignment and metadata.
	// +optional
	Models []apiextensionsv1.JSON `json:"models,omitempty"`
	// Tools are tool group names to register with the toolRuntime provider.
	// +optional
	Tools []string `json:"tools,omitempty"`
	// Shields are safety shield names to register with the safety provider.
	// +optional
	Shields []string `json:"shields,omitempty"`
}

// -----------------------------------------------------------------------------
// Storage Types (Task 1.4)
// -----------------------------------------------------------------------------

// KVStorageSpec configures the key-value storage backend.
type KVStorageSpec struct {
	// Type is the KV storage backend type.
	// +kubebuilder:validation:Enum=sqlite;redis
	// +kubebuilder:default:="sqlite"
	// +optional
	Type string `json:"type,omitempty"`
	// Endpoint is the Redis endpoint URL. Required when type is "redis".
	// +optional
	Endpoint string `json:"endpoint,omitempty"`
	// Password references a Secret for Redis authentication.
	// +optional
	Password *SecretKeyRef `json:"password,omitempty"`
}

// SQLStorageSpec configures the relational storage backend.
type SQLStorageSpec struct {
	// Type is the SQL storage backend type.
	// +kubebuilder:validation:Enum=sqlite;postgres
	// +kubebuilder:default:="sqlite"
	// +optional
	Type string `json:"type,omitempty"`
	// ConnectionString references a Secret containing the database connection string.
	// Required when type is "postgres".
	// +optional
	ConnectionString *SecretKeyRef `json:"connectionString,omitempty"`
}

// StateStorageSpec configures state storage backends for the LlamaStack server.
type StateStorageSpec struct {
	// KV configures the key-value storage backend (sqlite or redis).
	// +optional
	KV *KVStorageSpec `json:"kv,omitempty"`
	// SQL configures the relational storage backend (sqlite or postgres).
	// +optional
	SQL *SQLStorageSpec `json:"sql,omitempty"`
}

// -----------------------------------------------------------------------------
// Networking Types (Task 1.5)
// -----------------------------------------------------------------------------

// CABundleConfig defines the CA bundle configuration for custom certificates.
type CABundleConfig struct {
	// ConfigMapName is the name of the ConfigMap containing CA bundle certificates.
	// Must be in the same namespace as the CR.
	// +kubebuilder:validation:MinLength=1
	ConfigMapName string `json:"configMapName"`
}

// TLSSpec configures TLS for the LlamaStack server.
type TLSSpec struct {
	// Enabled enables TLS on the server.
	// +optional
	Enabled bool `json:"enabled,omitempty"`
	// SecretName references a Kubernetes TLS Secret. Required when enabled is true.
	// +optional
	SecretName string `json:"secretName,omitempty"`
	// CABundle configures custom CA certificates via ConfigMap reference.
	// +optional
	CABundle *CABundleConfig `json:"caBundle,omitempty"`
}

// AllowedFromSpec defines namespace-based access controls for NetworkPolicies.
type AllowedFromSpec struct {
	// Namespaces is an explicit list of namespace names allowed to access the service.
	// Use "*" to allow all namespaces.
	// +optional
	Namespaces []string `json:"namespaces,omitempty"`
	// Labels is a list of namespace label keys that grant access (OR semantics).
	// +optional
	Labels []string `json:"labels,omitempty"`
}

// NetworkingSpec consolidates network configuration for the LlamaStack service.
type NetworkingSpec struct {
	// Port is the server listen port.
	// +kubebuilder:default:=8321
	// +optional
	Port int32 `json:"port,omitempty"`
	// TLS configures TLS for the server.
	// +optional
	TLS *TLSSpec `json:"tls,omitempty"`
	// Expose controls external service exposure via Ingress/Route.
	// Supports polymorphic form: boolean (true/false) or object with hostname.
	// +optional
	Expose *apiextensionsv1.JSON `json:"expose,omitempty"`
	// AllowedFrom configures NetworkPolicy for namespace-based access control.
	// +optional
	AllowedFrom *AllowedFromSpec `json:"allowedFrom,omitempty"`
}

// -----------------------------------------------------------------------------
// Workload Types (Task 1.6)
// -----------------------------------------------------------------------------

// PVCStorageSpec defines PVC storage for persistent data.
type PVCStorageSpec struct {
	// Size is the size of the PVC.
	// +optional
	Size *resource.Quantity `json:"size,omitempty"`
	// MountPath is the container mount path for the PVC.
	// +optional
	MountPath string `json:"mountPath,omitempty"`
}

// PodDisruptionBudgetSpec defines voluntary disruption controls.
type PodDisruptionBudgetSpec struct {
	// MinAvailable is the minimum number of pods that must remain available.
	// +optional
	MinAvailable *intstr.IntOrString `json:"minAvailable,omitempty"`
	// MaxUnavailable is the maximum number of pods that can be disrupted simultaneously.
	// +optional
	MaxUnavailable *intstr.IntOrString `json:"maxUnavailable,omitempty"`
}

// AutoscalingSpec configures HorizontalPodAutoscaler targets.
type AutoscalingSpec struct {
	// MinReplicas is the lower bound replica count.
	// +optional
	MinReplicas *int32 `json:"minReplicas,omitempty"`
	// MaxReplicas is the upper bound replica count.
	MaxReplicas int32 `json:"maxReplicas"`
	// TargetCPUUtilizationPercentage configures CPU-based scaling.
	// +optional
	TargetCPUUtilizationPercentage *int32 `json:"targetCPUUtilizationPercentage,omitempty"`
	// TargetMemoryUtilizationPercentage configures memory-based scaling.
	// +optional
	TargetMemoryUtilizationPercentage *int32 `json:"targetMemoryUtilizationPercentage,omitempty"`
}

// WorkloadOverrides allows low-level customization of the Pod template.
type WorkloadOverrides struct {
	// ServiceAccountName specifies a custom ServiceAccount.
	// +optional
	ServiceAccountName string `json:"serviceAccountName,omitempty"`
	// Env specifies additional environment variables.
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`
	// Command overrides the container command.
	// +optional
	Command []string `json:"command,omitempty"`
	// Args overrides the container arguments.
	// +optional
	Args []string `json:"args,omitempty"`
	// Volumes adds additional volumes to the Pod.
	// +optional
	Volumes []corev1.Volume `json:"volumes,omitempty"`
	// VolumeMounts adds additional volume mounts to the container.
	// +optional
	VolumeMounts []corev1.VolumeMount `json:"volumeMounts,omitempty"`
}

// WorkloadSpec consolidates Kubernetes deployment settings.
type WorkloadSpec struct {
	// Replicas is the desired Pod replica count.
	// +kubebuilder:default:=1
	// +optional
	Replicas *int32 `json:"replicas,omitempty"`
	// Workers configures the number of uvicorn worker processes.
	// +kubebuilder:validation:Minimum=1
	// +optional
	Workers *int32 `json:"workers,omitempty"`
	// Resources specifies CPU/memory requests and limits.
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`
	// Autoscaling configures HPA.
	// +optional
	Autoscaling *AutoscalingSpec `json:"autoscaling,omitempty"`
	// Storage configures PVC for persistent data.
	// +optional
	Storage *PVCStorageSpec `json:"storage,omitempty"`
	// PodDisruptionBudget configures PDB.
	// +optional
	PodDisruptionBudget *PodDisruptionBudgetSpec `json:"podDisruptionBudget,omitempty"`
	// TopologySpreadConstraints defines Pod spreading rules.
	// +optional
	TopologySpreadConstraints []corev1.TopologySpreadConstraint `json:"topologySpreadConstraints,omitempty"`
	// Overrides provides low-level Pod template customization.
	// +optional
	Overrides *WorkloadOverrides `json:"overrides,omitempty"`
}

// -----------------------------------------------------------------------------
// Override Config Types
// -----------------------------------------------------------------------------

// OverrideConfigSpec specifies a user-provided ConfigMap for full config.yaml override.
// Mutually exclusive with providers, resources, storage, and disabled.
type OverrideConfigSpec struct {
	// ConfigMapName is the name of the ConfigMap containing config.yaml.
	// Must be in the same namespace as the CR.
	// +kubebuilder:validation:MinLength=1
	ConfigMapName string `json:"configMapName"`
}

// -----------------------------------------------------------------------------
// Main Spec and CRD Types (Task 1.7)
// -----------------------------------------------------------------------------

// LlamaStackDistributionSpec defines the desired state of LlamaStackDistribution.
// +kubebuilder:validation:XValidation:rule="!(has(self.providers) && has(self.overrideConfig))",message="providers and overrideConfig are mutually exclusive"
// +kubebuilder:validation:XValidation:rule="!(has(self.resources) && has(self.overrideConfig))",message="resources and overrideConfig are mutually exclusive"
// +kubebuilder:validation:XValidation:rule="!(has(self.storage) && has(self.overrideConfig))",message="storage and overrideConfig are mutually exclusive"
// +kubebuilder:validation:XValidation:rule="!(has(self.disabled) && has(self.overrideConfig))",message="disabled and overrideConfig are mutually exclusive"
type LlamaStackDistributionSpec struct {
	// Distribution identifies the LlamaStack distribution to deploy.
	Distribution DistributionSpec `json:"distribution"`
	// Providers configures LlamaStack providers by API type.
	// +optional
	Providers *ProvidersSpec `json:"providers,omitempty"`
	// Resources declares models, tools, and shields to register.
	// +optional
	Resources *ResourcesSpec `json:"resources,omitempty"`
	// Storage configures state storage backends (KV and SQL).
	// +optional
	Storage *StateStorageSpec `json:"storage,omitempty"`
	// Disabled is a list of LlamaStack API names to disable.
	// +optional
	Disabled []string `json:"disabled,omitempty"`
	// Networking consolidates network configuration.
	// +optional
	Networking *NetworkingSpec `json:"networking,omitempty"`
	// Workload consolidates Kubernetes deployment settings.
	// +optional
	Workload *WorkloadSpec `json:"workload,omitempty"`
	// ExternalProviders integrates with spec 001 deploy-time provider injection.
	// +optional
	ExternalProviders *apiextensionsv1.JSON `json:"externalProviders,omitempty"`
	// OverrideConfig specifies a user-provided ConfigMap for full config.yaml override.
	// Mutually exclusive with providers, resources, storage, and disabled.
	// +optional
	OverrideConfig *OverrideConfigSpec `json:"overrideConfig,omitempty"`
}

// -----------------------------------------------------------------------------
// Status Types
// -----------------------------------------------------------------------------

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

// DistributionConfig represents configuration info from the providers endpoint.
type DistributionConfig struct {
	ActiveDistribution     string            `json:"activeDistribution,omitempty"`
	Providers              []ProviderInfo    `json:"providers,omitempty"`
	AvailableDistributions map[string]string `json:"availableDistributions,omitempty"`
}

// VersionInfo contains version-related information.
type VersionInfo struct {
	OperatorVersion         string      `json:"operatorVersion,omitempty"`
	LlamaStackServerVersion string      `json:"llamaStackServerVersion,omitempty"`
	LastUpdated             metav1.Time `json:"lastUpdated,omitempty"`
}

// ResolvedDistributionStatus tracks the resolved distribution image for change detection.
type ResolvedDistributionStatus struct {
	// Image is the resolved container image reference (with digest when available).
	Image string `json:"image,omitempty"`
	// ConfigSource indicates the config origin: "embedded" or "oci-label".
	ConfigSource string `json:"configSource,omitempty"`
	// ConfigHash is the SHA256 hash of the base config used.
	ConfigHash string `json:"configHash,omitempty"`
}

// ConfigGenerationStatus tracks config generation details.
type ConfigGenerationStatus struct {
	// ConfigMapName is the name of the generated ConfigMap.
	ConfigMapName string `json:"configMapName,omitempty"`
	// GeneratedAt is the timestamp of the last generation.
	GeneratedAt metav1.Time `json:"generatedAt,omitempty"`
	// ProviderCount is the number of configured providers.
	ProviderCount int `json:"providerCount,omitempty"`
	// ResourceCount is the number of registered resources.
	ResourceCount int `json:"resourceCount,omitempty"`
	// ConfigVersion is the config.yaml schema version.
	ConfigVersion int `json:"configVersion,omitempty"`
}

// LlamaStackDistributionStatus defines the observed state of LlamaStackDistribution.
type LlamaStackDistributionStatus struct {
	// Phase represents the current phase of the distribution.
	Phase DistributionPhase `json:"phase,omitempty"`
	// Conditions represent the latest available observations of the distribution's current state.
	Conditions []metav1.Condition `json:"conditions,omitempty"`
	// ResolvedDistribution tracks the resolved image and config source.
	// +optional
	ResolvedDistribution *ResolvedDistributionStatus `json:"resolvedDistribution,omitempty"`
	// ConfigGeneration tracks config generation details.
	// +optional
	ConfigGeneration *ConfigGenerationStatus `json:"configGeneration,omitempty"`
	// Version contains version information for both operator and deployment.
	Version VersionInfo `json:"version,omitempty"`
	// DistributionConfig contains configuration info from the providers endpoint.
	DistributionConfig DistributionConfig `json:"distributionConfig,omitempty"`
	// AvailableReplicas is the number of available replicas.
	AvailableReplicas int32 `json:"availableReplicas,omitempty"`
	// ServiceURL is the internal Kubernetes service URL.
	ServiceURL string `json:"serviceURL,omitempty"`
	// RouteURL is the external URL when external access is configured.
	// +optional
	RouteURL *string `json:"routeURL,omitempty"`
}

// -----------------------------------------------------------------------------
// Root CRD Types
// -----------------------------------------------------------------------------

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
