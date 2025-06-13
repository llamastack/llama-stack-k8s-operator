# API Reference

## Packages
- [llamastack.io/v1alpha1](#llamastackiov1alpha1)

## llamastack.io/v1alpha1

Package v1alpha1 contains API Schema definitions for the  v1alpha1 API group

### Resource Types
- [LlamaStackDistribution](#llamastackdistribution)
- [LlamaStackDistributionList](#llamastackdistributionlist)

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

#### DistributionConfig

DistributionConfig represents the configuration information from the providers endpoint.

_Appears in:_
- [LlamaStackDistributionStatus](#llamastackdistributionstatus)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `activeDistribution` _string_ | ActiveDistribution shows which distribution is currently being used |  |  |
| `providers` _[ProviderInfo](#providerinfo) array_ |  |  |  |
| `availableDistributions` _object (keys:string, values:string)_ | AvailableDistributions lists all available distributions and their images |  |  |

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

#### LlamaStackDistributionStatus

LlamaStackDistributionStatus defines the observed state of LlamaStackDistribution.

_Appears in:_
- [LlamaStackDistribution](#llamastackdistribution)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `version` _string_ |  |  |  |
| `distributionConfig` _[DistributionConfig](#distributionconfig)_ |  |  |  |
| `ready` _boolean_ |  |  |  |

#### PodOverrides

PodOverrides allows advanced pod-level customization.

_Appears in:_
- [ServerSpec](#serverspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
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
| `podOverrides` _[PodOverrides](#podoverrides)_ |  |  |  |
| `storage` _[StorageSpec](#storagespec)_ | Storage defines the persistent storage configuration |  |  |

#### StorageSpec

StorageSpec defines the persistent storage configuration

_Appears in:_
- [ServerSpec](#serverspec)

| Field | Description | Default | Validation |
| --- | --- | --- | --- |
| `size` _[Quantity](https://kubernetes.io/docs/reference/generated/kubernetes-api/v1.31/#quantity-resource-api)_ | Size is the size of the persistent volume claim created for holding persistent data of the llama-stack server |  |  |
| `mountPath` _string_ | MountPath is the path where the storage will be mounted in the container |  |  |
