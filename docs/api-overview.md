# API Reference

## Packages
- [llamastack.io/v1alpha1](#llamastackiov1alpha1)
- [llamastack.io/v1alpha2](#llamastackiov1alpha2)

## llamastack.io/v1alpha1

Package v1alpha1 contains API Schema definitions for the  v1alpha1 API group

### Resource Types
- [LlamaStackDistribution](#llamastackdistribution)
- [LlamaStackDistributionList](#llamastackdistributionlist)

#### AllowedFromSpec

AllowedFromSpec defines namespace-based access controls for NetworkPolicies.

_Appears in:_
- [NetworkSpec](#networkspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `namespaces` _string array_ | Namespaces is an explicit list of namespace names allowed to access the service.<br />Use "*" to allow all namespaces. |  |  |
| `labels` _string array_ | Labels is a list of namespace label keys that are allowed to access the service.<br />A namespace matching any of these labels will be granted access (OR semantics).<br />Example: ["myproject/lls-allowed", "team/authorized"] |  |  |

#### AutoscalingSpec

AutoscalingSpec configures HorizontalPodAutoscaler targets.

_Appears in:_
- [ServerSpec](#serverspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `minReplicas` _integer_ | MinReplicas is the lower bound replica count maintained by the HPA |  |  |
| `maxReplicas` _integer_ | MaxReplicas is the upper bound replica count maintained by the HPA |  |  |
| `targetCPUUtilizationPercentage` _integer_ | TargetCPUUtilizationPercentage configures CPU based scaling |  |  |
| `targetMemoryUtilizationPercentage` _integer_ | TargetMemoryUtilizationPercentage configures memory based scaling |  |  |

#### CABundleConfig

CABundleConfig defines the CA bundle configuration for custom certificates

_Appears in:_
- [TLSConfig](#tlsconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `configMapName` _string_ | ConfigMapName is the name of the ConfigMap containing CA bundle certificates |  |  |
| `configMapNamespace` _string_ | ConfigMapNamespace is the namespace of the ConfigMap (defaults to the same namespace as the CR) |  |  |
| `configMapKeys` _string array_ | ConfigMapKeys specifies multiple keys within the ConfigMap containing CA bundle data<br />All certificates from these keys will be concatenated into a single CA bundle file<br />If not specified, defaults to [DefaultCABundleKey] |  | MaxItems: 50 <br /> |

#### ContainerSpec

ContainerSpec defines the llama-stack server container configuration.

_Appears in:_
- [ServerSpec](#serverspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ |  | llama-stack |  |
| `port` _integer_ |  |  |  |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#resourcerequirements-v1-core)_ |  |  |  |
| `env` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#envvar-v1-core) array_ |  |  |  |
| `command` _string array_ |  |  |  |
| `args` _string array_ |  |  |  |

#### DistributionConfig

DistributionConfig represents the configuration information from the providers endpoint.

_Appears in:_
- [LlamaStackDistributionStatus](#llamastackdistributionstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `activeDistribution` _string_ | ActiveDistribution shows which distribution is currently being used |  |  |
| `providers` _[ProviderInfo](#providerinfo) array_ |  |  |  |
| `availableDistributions` _object (keys:string, values:string)_ | AvailableDistributions lists all available distributions and their images |  |  |

#### DistributionPhase

_Underlying type:_ _string_

LlamaStackDistributionPhase represents the current phase of the LlamaStackDistribution

_Validation:_
- Enum: [Pending Initializing Ready Failed Terminating]

_Appears in:_
- [LlamaStackDistributionStatus](#llamastackdistributionstatus)

| Field | Description |
| --- | --- |
| `Pending` | LlamaStackDistributionPhasePending indicates that the distribution is pending initialization<br /> |
| `Initializing` | LlamaStackDistributionPhaseInitializing indicates that the distribution is being initialized<br /> |
| `Ready` | LlamaStackDistributionPhaseReady indicates that the distribution is ready to use<br /> |
| `Failed` | LlamaStackDistributionPhaseFailed indicates that the distribution has failed<br /> |
| `Terminating` | LlamaStackDistributionPhaseTerminating indicates that the distribution is being terminated<br /> |

#### DistributionType

DistributionType defines the distribution configuration for llama-stack.

_Appears in:_
- [ServerSpec](#serverspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name is the distribution name that maps to supported distributions. |  |  |
| `image` _string_ | Image is the direct container image reference to use |  |  |

#### LlamaStackDistribution

_Appears in:_
- [LlamaStackDistributionList](#llamastackdistributionlist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `llamastack.io/v1alpha1` | | |
| `kind` _string_ | `LlamaStackDistribution` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[LlamaStackDistributionSpec](#llamastackdistributionspec)_ |  |  |  |
| `status` _[LlamaStackDistributionStatus](#llamastackdistributionstatus)_ |  |  |  |

#### LlamaStackDistributionList

LlamaStackDistributionList contains a list of LlamaStackDistribution.

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `llamastack.io/v1alpha1` | | |
| `kind` _string_ | `LlamaStackDistributionList` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[LlamaStackDistribution](#llamastackdistribution) array_ |  |  |  |

#### LlamaStackDistributionSpec

LlamaStackDistributionSpec defines the desired state of LlamaStackDistribution.

_Appears in:_
- [LlamaStackDistribution](#llamastackdistribution)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `replicas` _integer_ |  | 1 |  |
| `server` _[ServerSpec](#serverspec)_ |  |  |  |
| `network` _[NetworkSpec](#networkspec)_ | Network defines network access controls for the LlamaStack service |  |  |

#### LlamaStackDistributionStatus

LlamaStackDistributionStatus defines the observed state of LlamaStackDistribution.

_Appears in:_
- [LlamaStackDistribution](#llamastackdistribution)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `phase` _[DistributionPhase](#distributionphase)_ | Phase represents the current phase of the distribution |  | Enum: [Pending Initializing Ready Failed Terminating] <br /> |
| `version` _[VersionInfo](#versioninfo)_ | Version contains version information for both operator and deployment |  |  |
| `distributionConfig` _[DistributionConfig](#distributionconfig)_ | DistributionConfig contains the configuration information from the providers endpoint |  |  |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#condition-v1-meta) array_ | Conditions represent the latest available observations of the distribution's current state |  |  |
| `availableReplicas` _integer_ | AvailableReplicas is the number of available replicas |  |  |
| `serviceURL` _string_ | ServiceURL is the internal Kubernetes service URL where the distribution is exposed |  |  |
| `routeURL` _string_ | RouteURL is the external URL where the distribution is exposed (when exposeRoute is true).<br />nil when external access is not configured, empty string when Ingress exists but URL not ready. |  |  |

#### NetworkSpec

NetworkSpec defines network access controls for the LlamaStack service.

_Appears in:_
- [LlamaStackDistributionSpec](#llamastackdistributionspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `exposeRoute` _boolean_ | ExposeRoute when true, creates an Ingress for external access.<br />Default is false (internal access only). | false |  |
| `allowedFrom` _[AllowedFromSpec](#allowedfromspec)_ | AllowedFrom defines which namespaces are allowed to access the LlamaStack service.<br />By default, only the LLSD namespace and the operator namespace are allowed. |  |  |

#### PodDisruptionBudgetSpec

PodDisruptionBudgetSpec defines voluntary disruption controls.

_Appears in:_
- [ServerSpec](#serverspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `minAvailable` _[IntOrString](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#intorstring-intstr-util)_ | MinAvailable is the minimum number of pods that must remain available |  |  |
| `maxUnavailable` _[IntOrString](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#intorstring-intstr-util)_ | MaxUnavailable is the maximum number of pods that can be disrupted simultaneously |  |  |

#### PodOverrides

PodOverrides allows advanced pod-level customization.

_Appears in:_
- [ServerSpec](#serverspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `serviceAccountName` _string_ | ServiceAccountName allows users to specify their own ServiceAccount<br />If not specified, the operator will use the default ServiceAccount |  |  |
| `terminationGracePeriodSeconds` _[int64](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#int64-v1-core)_ | TerminationGracePeriodSeconds is the time allowed for graceful pod shutdown.<br />If not specified, Kubernetes defaults to 30 seconds. |  |  |
| `volumes` _[Volume](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#volume-v1-core) array_ |  |  |  |
| `volumeMounts` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#volumemount-v1-core) array_ |  |  |  |

#### ProviderHealthStatus

HealthStatus represents the health status of a provider

_Appears in:_
- [ProviderInfo](#providerinfo)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `status` _string_ |  |  |  |
| `message` _string_ |  |  |  |

#### ProviderInfo

ProviderInfo represents a single provider from the providers endpoint.

_Appears in:_
- [DistributionConfig](#distributionconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `api` _string_ |  |  |  |
| `provider_id` _string_ |  |  |  |
| `provider_type` _string_ |  |  |  |
| `config` _[JSON](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#json-v1-apiextensions-k8s-io)_ |  |  |  |
| `health` _[ProviderHealthStatus](#providerhealthstatus)_ |  |  |  |

#### ServerSpec

ServerSpec defines the desired state of llama server.

_Appears in:_
- [LlamaStackDistributionSpec](#llamastackdistributionspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `distribution` _[DistributionType](#distributiontype)_ |  |  |  |
| `containerSpec` _[ContainerSpec](#containerspec)_ |  |  |  |
| `workers` _integer_ | Workers configures the number of uvicorn worker processes to run.<br />When set, the operator will launch llama-stack using uvicorn with the specified worker count.<br />Ref: https://fastapi.tiangolo.com/deployment/server-workers/<br />CPU requests are set to the number of workers when set, otherwise 1 full core |  | Minimum: 1 <br /> |
| `podOverrides` _[PodOverrides](#podoverrides)_ |  |  |  |
| `podDisruptionBudget` _[PodDisruptionBudgetSpec](#poddisruptionbudgetspec)_ | PodDisruptionBudget controls voluntary disruption tolerance for the server pods |  |  |
| `topologySpreadConstraints` _[TopologySpreadConstraint](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#topologyspreadconstraint-v1-core) array_ | TopologySpreadConstraints defines fine-grained spreading rules |  |  |
| `autoscaling` _[AutoscalingSpec](#autoscalingspec)_ | Autoscaling configures HorizontalPodAutoscaler for the server pods |  |  |
| `storage` _[StorageSpec](#storagespec)_ | Storage defines the persistent storage configuration |  |  |
| `userConfig` _[UserConfigSpec](#userconfigspec)_ | UserConfig defines the user configuration for the llama-stack server |  |  |
| `tlsConfig` _[TLSConfig](#tlsconfig)_ | TLSConfig defines the TLS configuration for the llama-stack server |  |  |

#### StorageSpec

StorageSpec defines the persistent storage configuration

_Appears in:_
- [ServerSpec](#serverspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `size` _[Quantity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#quantity-resource-api)_ | Size is the size of the persistent volume claim created for holding persistent data of the llama-stack server |  |  |
| `mountPath` _string_ | MountPath is the path where the storage will be mounted in the container |  |  |

#### TLSConfig

TLSConfig defines the TLS configuration for the llama-stack server

_Appears in:_
- [ServerSpec](#serverspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `caBundle` _[CABundleConfig](#cabundleconfig)_ | CABundle defines the CA bundle configuration for custom certificates |  |  |

#### UserConfigSpec

_Appears in:_
- [ServerSpec](#serverspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `configMapName` _string_ | ConfigMapName is the name of the ConfigMap containing user configuration |  |  |
| `configMapNamespace` _string_ | ConfigMapNamespace is the namespace of the ConfigMap (defaults to the same namespace as the CR) |  |  |

#### VersionInfo

VersionInfo contains version-related information

_Appears in:_
- [LlamaStackDistributionStatus](#llamastackdistributionstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `operatorVersion` _string_ | OperatorVersion is the version of the operator managing this distribution |  |  |
| `llamaStackServerVersion` _string_ | LlamaStackServerVersion is the version of the LlamaStack server |  |  |
| `lastUpdated` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#time-v1-meta)_ | LastUpdated represents when the version information was last updated |  |  |

## llamastack.io/v1alpha2

Package v1alpha2 contains API Schema definitions for the v1alpha2 API group.
v1alpha2 introduces operator-generated server configuration, enabling the operator
to generate config.yaml from a high-level specification in the CR.

### Resource Types
- [LlamaStackDistribution](#llamastackdistribution)
- [LlamaStackDistributionList](#llamastackdistributionlist)

#### AllowedFromSpec

AllowedFromSpec defines namespace-based access controls for NetworkPolicies.

_Appears in:_
- [NetworkingSpec](#networkingspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `namespaces` _string array_ | Namespaces is an explicit list of namespace names allowed to access the service.<br />Use "*" to allow all namespaces. |  |  |
| `labels` _string array_ | Labels is a list of namespace label keys that grant access (OR semantics). |  |  |

#### AutoscalingSpec

AutoscalingSpec configures HorizontalPodAutoscaler targets.

_Appears in:_
- [WorkloadSpec](#workloadspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `minReplicas` _integer_ | MinReplicas is the lower bound replica count. |  |  |
| `maxReplicas` _integer_ | MaxReplicas is the upper bound replica count. |  |  |
| `targetCPUUtilizationPercentage` _integer_ | TargetCPUUtilizationPercentage configures CPU-based scaling. |  |  |
| `targetMemoryUtilizationPercentage` _integer_ | TargetMemoryUtilizationPercentage configures memory-based scaling. |  |  |

#### CABundleConfig

CABundleConfig defines the CA bundle configuration for custom certificates.

_Appears in:_
- [TLSSpec](#tlsspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `configMapName` _string_ | ConfigMapName is the name of the ConfigMap containing CA bundle certificates.<br />Must be in the same namespace as the CR. |  | MinLength: 1 <br /> |

#### ConfigGenerationStatus

ConfigGenerationStatus tracks config generation details.

_Appears in:_
- [LlamaStackDistributionStatus](#llamastackdistributionstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `configMapName` _string_ | ConfigMapName is the name of the generated ConfigMap. |  |  |
| `generatedAt` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#time-v1-meta)_ | GeneratedAt is the timestamp of the last generation. |  |  |
| `providerCount` _integer_ | ProviderCount is the number of configured providers. |  |  |
| `resourceCount` _integer_ | ResourceCount is the number of registered resources. |  |  |
| `configVersion` _integer_ | ConfigVersion is the config.yaml schema version. |  |  |

#### DistributionConfig

DistributionConfig represents configuration info from the providers endpoint.

_Appears in:_
- [LlamaStackDistributionStatus](#llamastackdistributionstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `activeDistribution` _string_ |  |  |  |
| `providers` _[ProviderInfo](#providerinfo) array_ |  |  |  |
| `availableDistributions` _object (keys:string, values:string)_ |  |  |  |

#### DistributionPhase

_Underlying type:_ _string_

DistributionPhase represents the current phase of the LlamaStackDistribution.

_Validation:_
- Enum: [Pending Initializing Ready Failed Terminating]

_Appears in:_
- [LlamaStackDistributionStatus](#llamastackdistributionstatus)

| Field | Description |
| --- | --- |
| `Pending` |  |
| `Initializing` |  |
| `Ready` |  |
| `Failed` |  |
| `Terminating` |  |

#### DistributionSpec

DistributionSpec identifies the LlamaStack distribution image to deploy.
Exactly one of name or image must be specified.

_Appears in:_
- [LlamaStackDistributionSpec](#llamastackdistributionspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name is the distribution name that maps to a supported distribution (e.g., "starter", "remote-vllm").<br />Resolved to a container image via distributions.json and image-overrides. |  |  |
| `image` _string_ | Image is a direct container image reference to use. |  |  |

#### KVStorageSpec

KVStorageSpec configures the key-value storage backend.

_Appears in:_
- [StateStorageSpec](#statestoragespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `type` _string_ | Type is the KV storage backend type. | sqlite | Enum: [sqlite redis] <br /> |
| `endpoint` _string_ | Endpoint is the Redis endpoint URL. Required when type is "redis". |  |  |
| `password` _[SecretKeyRef](#secretkeyref)_ | Password references a Secret for Redis authentication. |  |  |

#### LlamaStackDistribution

LlamaStackDistribution is the Schema for the llamastackdistributions API.

_Appears in:_
- [LlamaStackDistributionList](#llamastackdistributionlist)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `llamastack.io/v1alpha2` | | |
| `kind` _string_ | `LlamaStackDistribution` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ObjectMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#objectmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `spec` _[LlamaStackDistributionSpec](#llamastackdistributionspec)_ |  |  |  |
| `status` _[LlamaStackDistributionStatus](#llamastackdistributionstatus)_ |  |  |  |

#### LlamaStackDistributionList

LlamaStackDistributionList contains a list of LlamaStackDistribution.

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `apiVersion` _string_ | `llamastack.io/v1alpha2` | | |
| `kind` _string_ | `LlamaStackDistributionList` | | |
| `kind` _string_ | Kind is a string value representing the REST resource this object represents.<br />Servers may infer this from the endpoint the client submits requests to.<br />Cannot be updated.<br />In CamelCase.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds |  |  |
| `apiVersion` _string_ | APIVersion defines the versioned schema of this representation of an object.<br />Servers should convert recognized schemas to the latest internal value, and<br />may reject unrecognized values.<br />More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources |  |  |
| `metadata` _[ListMeta](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#listmeta-v1-meta)_ | Refer to Kubernetes API documentation for fields of `metadata`. |  |  |
| `items` _[LlamaStackDistribution](#llamastackdistribution) array_ |  |  |  |

#### LlamaStackDistributionSpec

LlamaStackDistributionSpec defines the desired state of LlamaStackDistribution.

_Appears in:_
- [LlamaStackDistribution](#llamastackdistribution)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `distribution` _[DistributionSpec](#distributionspec)_ | Distribution identifies the LlamaStack distribution to deploy. |  |  |
| `providers` _[ProvidersSpec](#providersspec)_ | Providers configures LlamaStack providers by API type. |  |  |
| `resources` _[ResourcesSpec](#resourcesspec)_ | Resources declares models, tools, and shields to register. |  |  |
| `storage` _[StateStorageSpec](#statestoragespec)_ | Storage configures state storage backends (KV and SQL). |  |  |
| `disabled` _string array_ | Disabled is a list of LlamaStack API names to disable. |  |  |
| `networking` _[NetworkingSpec](#networkingspec)_ | Networking consolidates network configuration. |  |  |
| `workload` _[WorkloadSpec](#workloadspec)_ | Workload consolidates Kubernetes deployment settings. |  |  |
| `externalProviders` _[JSON](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#json-v1-apiextensions-k8s-io)_ | ExternalProviders integrates with spec 001 deploy-time provider injection. |  |  |
| `overrideConfig` _[OverrideConfigSpec](#overrideconfigspec)_ | OverrideConfig specifies a user-provided ConfigMap for full config.yaml override.<br />Mutually exclusive with providers, resources, storage, and disabled. |  |  |

#### LlamaStackDistributionStatus

LlamaStackDistributionStatus defines the observed state of LlamaStackDistribution.

_Appears in:_
- [LlamaStackDistribution](#llamastackdistribution)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `phase` _[DistributionPhase](#distributionphase)_ | Phase represents the current phase of the distribution. |  | Enum: [Pending Initializing Ready Failed Terminating] <br /> |
| `conditions` _[Condition](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#condition-v1-meta) array_ | Conditions represent the latest available observations of the distribution's current state. |  |  |
| `resolvedDistribution` _[ResolvedDistributionStatus](#resolveddistributionstatus)_ | ResolvedDistribution tracks the resolved image and config source. |  |  |
| `configGeneration` _[ConfigGenerationStatus](#configgenerationstatus)_ | ConfigGeneration tracks config generation details. |  |  |
| `version` _[VersionInfo](#versioninfo)_ | Version contains version information for both operator and deployment. |  |  |
| `distributionConfig` _[DistributionConfig](#distributionconfig)_ | DistributionConfig contains configuration info from the providers endpoint. |  |  |
| `availableReplicas` _integer_ | AvailableReplicas is the number of available replicas. |  |  |
| `serviceURL` _string_ | ServiceURL is the internal Kubernetes service URL. |  |  |
| `routeURL` _string_ | RouteURL is the external URL when external access is configured. |  |  |

#### NetworkingSpec

NetworkingSpec consolidates network configuration for the LlamaStack service.

_Appears in:_
- [LlamaStackDistributionSpec](#llamastackdistributionspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `port` _integer_ | Port is the server listen port. | 8321 |  |
| `tls` _[TLSSpec](#tlsspec)_ | TLS configures TLS for the server. |  |  |
| `expose` _[JSON](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#json-v1-apiextensions-k8s-io)_ | Expose controls external service exposure via Ingress/Route.<br />Supports polymorphic form: boolean (true/false) or object with hostname. |  |  |
| `allowedFrom` _[AllowedFromSpec](#allowedfromspec)_ | AllowedFrom configures NetworkPolicy for namespace-based access control. |  |  |

#### OverrideConfigSpec

OverrideConfigSpec specifies a user-provided ConfigMap for full config.yaml override.
Mutually exclusive with providers, resources, storage, and disabled.

_Appears in:_
- [LlamaStackDistributionSpec](#llamastackdistributionspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `configMapName` _string_ | ConfigMapName is the name of the ConfigMap containing config.yaml.<br />Must be in the same namespace as the CR. |  | MinLength: 1 <br /> |

#### PVCStorageSpec

PVCStorageSpec defines PVC storage for persistent data.

_Appears in:_
- [WorkloadSpec](#workloadspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `size` _[Quantity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#quantity-resource-api)_ | Size is the size of the PVC. |  |  |
| `mountPath` _string_ | MountPath is the container mount path for the PVC. |  |  |

#### PodDisruptionBudgetSpec

PodDisruptionBudgetSpec defines voluntary disruption controls.

_Appears in:_
- [WorkloadSpec](#workloadspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `minAvailable` _[IntOrString](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#intorstring-intstr-util)_ | MinAvailable is the minimum number of pods that must remain available. |  |  |
| `maxUnavailable` _[IntOrString](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#intorstring-intstr-util)_ | MaxUnavailable is the maximum number of pods that can be disrupted simultaneously. |  |  |

#### ProviderHealthStatus

ProviderHealthStatus represents the health status of a provider.

_Appears in:_
- [ProviderInfo](#providerinfo)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `status` _string_ |  |  |  |
| `message` _string_ |  |  |  |

#### ProviderInfo

ProviderInfo represents a single provider from the providers endpoint.

_Appears in:_
- [DistributionConfig](#distributionconfig)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `api` _string_ |  |  |  |
| `provider_id` _string_ |  |  |  |
| `provider_type` _string_ |  |  |  |
| `config` _[JSON](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#json-v1-apiextensions-k8s-io)_ |  |  |  |
| `health` _[ProviderHealthStatus](#providerhealthstatus)_ |  |  |  |

#### ProvidersSpec

ProvidersSpec defines provider configurations by API type.
Each field supports polymorphic form: a single ProviderConfig object
or a list of ProviderConfig objects with explicit id fields.

_Appears in:_
- [LlamaStackDistributionSpec](#llamastackdistributionspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `inference` _[JSON](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#json-v1-apiextensions-k8s-io)_ | Inference configures inference providers (e.g., vLLM, TGI). |  |  |
| `safety` _[JSON](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#json-v1-apiextensions-k8s-io)_ | Safety configures safety providers (e.g., llama-guard). |  |  |
| `vectorIo` _[JSON](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#json-v1-apiextensions-k8s-io)_ | VectorIo configures vector I/O providers (e.g., pgvector, chromadb). |  |  |
| `toolRuntime` _[JSON](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#json-v1-apiextensions-k8s-io)_ | ToolRuntime configures tool runtime providers. |  |  |
| `telemetry` _[JSON](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#json-v1-apiextensions-k8s-io)_ | Telemetry configures telemetry providers (e.g., opentelemetry). |  |  |

#### ResolvedDistributionStatus

ResolvedDistributionStatus tracks the resolved distribution image for change detection.

_Appears in:_
- [LlamaStackDistributionStatus](#llamastackdistributionstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `image` _string_ | Image is the resolved container image reference (with digest when available). |  |  |
| `configSource` _string_ | ConfigSource indicates the config origin: "embedded" or "oci-label". |  |  |
| `configHash` _string_ | ConfigHash is the SHA256 hash of the base config used. |  |  |

#### ResourcesSpec

ResourcesSpec defines resources to register with the LlamaStack server.

_Appears in:_
- [LlamaStackDistributionSpec](#llamastackdistributionspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `models` _[JSON](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#json-v1-apiextensions-k8s-io) array_ | Models to register. Each item can be a simple string (model name)<br />or a ModelConfig object with provider assignment and metadata. |  |  |
| `tools` _string array_ | Tools are tool group names to register with the toolRuntime provider. |  |  |
| `shields` _string array_ | Shields are safety shield names to register with the safety provider. |  |  |

#### SQLStorageSpec

SQLStorageSpec configures the relational storage backend.

_Appears in:_
- [StateStorageSpec](#statestoragespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `type` _string_ | Type is the SQL storage backend type. | sqlite | Enum: [sqlite postgres] <br /> |
| `connectionString` _[SecretKeyRef](#secretkeyref)_ | ConnectionString references a Secret containing the database connection string.<br />Required when type is "postgres". |  |  |

#### SecretKeyRef

SecretKeyRef references a specific key in a Kubernetes Secret.

_Appears in:_
- [KVStorageSpec](#kvstoragespec)
- [ProviderConfig](#providerconfig)
- [SQLStorageSpec](#sqlstoragespec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `name` _string_ | Name is the name of the Secret. |  | MinLength: 1 <br /> |
| `key` _string_ | Key is the key within the Secret. |  | MinLength: 1 <br /> |

#### StateStorageSpec

StateStorageSpec configures state storage backends for the LlamaStack server.

_Appears in:_
- [LlamaStackDistributionSpec](#llamastackdistributionspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `kv` _[KVStorageSpec](#kvstoragespec)_ | KV configures the key-value storage backend (sqlite or redis). |  |  |
| `sql` _[SQLStorageSpec](#sqlstoragespec)_ | SQL configures the relational storage backend (sqlite or postgres). |  |  |

#### TLSSpec

TLSSpec configures TLS for the LlamaStack server.

_Appears in:_
- [NetworkingSpec](#networkingspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `enabled` _boolean_ | Enabled enables TLS on the server. |  |  |
| `secretName` _string_ | SecretName references a Kubernetes TLS Secret. Required when enabled is true. |  |  |
| `caBundle` _[CABundleConfig](#cabundleconfig)_ | CABundle configures custom CA certificates via ConfigMap reference. |  |  |

#### VersionInfo

VersionInfo contains version-related information.

_Appears in:_
- [LlamaStackDistributionStatus](#llamastackdistributionstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `operatorVersion` _string_ |  |  |  |
| `llamaStackServerVersion` _string_ |  |  |  |
| `lastUpdated` _[Time](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#time-v1-meta)_ |  |  |  |

#### WorkloadOverrides

WorkloadOverrides allows low-level customization of the Pod template.

_Appears in:_
- [WorkloadSpec](#workloadspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `serviceAccountName` _string_ | ServiceAccountName specifies a custom ServiceAccount. |  |  |
| `env` _[EnvVar](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#envvar-v1-core) array_ | Env specifies additional environment variables. |  |  |
| `command` _string array_ | Command overrides the container command. |  |  |
| `args` _string array_ | Args overrides the container arguments. |  |  |
| `volumes` _[Volume](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#volume-v1-core) array_ | Volumes adds additional volumes to the Pod. |  |  |
| `volumeMounts` _[VolumeMount](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#volumemount-v1-core) array_ | VolumeMounts adds additional volume mounts to the container. |  |  |

#### WorkloadSpec

WorkloadSpec consolidates Kubernetes deployment settings.

_Appears in:_
- [LlamaStackDistributionSpec](#llamastackdistributionspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `replicas` _integer_ | Replicas is the desired Pod replica count. | 1 |  |
| `workers` _integer_ | Workers configures the number of uvicorn worker processes. |  | Minimum: 1 <br /> |
| `resources` _[ResourceRequirements](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#resourcerequirements-v1-core)_ | Resources specifies CPU/memory requests and limits. |  |  |
| `autoscaling` _[AutoscalingSpec](#autoscalingspec)_ | Autoscaling configures HPA. |  |  |
| `storage` _[PVCStorageSpec](#pvcstoragespec)_ | Storage configures PVC for persistent data. |  |  |
| `podDisruptionBudget` _[PodDisruptionBudgetSpec](#poddisruptionbudgetspec)_ | PodDisruptionBudget configures PDB. |  |  |
| `topologySpreadConstraints` _[TopologySpreadConstraint](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#topologyspreadconstraint-v1-core) array_ | TopologySpreadConstraints defines Pod spreading rules. |  |  |
| `overrides` _[WorkloadOverrides](#workloadoverrides)_ | Overrides provides low-level Pod template customization. |  |  |
