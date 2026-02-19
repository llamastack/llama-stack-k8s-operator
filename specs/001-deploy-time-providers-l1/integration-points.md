# Integration Points: External Providers Feature (Final)

**Purpose**: Document all integration points with existing codebase for implementing external provider support using LlamaStack's native `module:` field.

**Created**: 2025-11-13
**Updated**: 2026-01-29 (Simplified to use native module: support)

**Related**: See `extra-providers-schema.md` for provider entry schema and migration notes

---

## Executive Summary

The external providers feature uses LlamaStack's native `module:` field support for provider loading. Provider entries are added directly to config.yaml with the `module:` field.

**Architecture**:
- **Install Phase**: N init containers install provider packages from container images
- **Config Generation**: Single init container adds provider entries with `module:` field to config.yaml
- **Runtime**: LlamaStack imports providers via `importlib.import_module()` using the `module:` field

**Benefits of This Approach**:
- ✅ Uses LlamaStack's native module loading mechanism
- ✅ No separate extra-providers.yaml file needed
- ✅ Self-contained - packages come from container images, not external repos
- ✅ Simpler architecture - just add provider entries with module: field

---

## Init Container Architecture (Phase 1 - Current)

### Two-Phase Init Container Flow

```
Phase 1: INSTALL (N init containers, CRD order)
┌─────────────────────────────────────┐
│ install-provider-provider-1         │
│ Image: <provider-1-image>           │
│ → pip install packages              │
│ → Copy metadata to shared volume    │
└─────────────────────────────────────┘
              ↓
┌─────────────────────────────────────┐
│ install-provider-provider-2         │
│ Image: <provider-2-image>           │
│ → pip install packages              │
│ → Copy metadata to shared volume    │
└─────────────────────────────────────┘
              ↓

Phase 2: MERGE (1 init container)
┌─────────────────────────────────────┐
│ merge-config                        │
│ Image: <operator-image>             │
│ Binary: /usr/local/bin/generate-config
│ → Read provider metadata            │
│ → Generate config.yaml (with module: field)     │
│ → Read user config.yaml (if exists)    │
│ → Merge user + extra-providers      │
│ → Write to /shared/final/config.yaml  │
└─────────────────────────────────────┘
              ↓

MAIN CONTAINER
┌─────────────────────────────────────┐
│ llama-stack                         │
│ → Mounts /shared/final/config.yaml    │
│ → PYTHONPATH includes providers     │
│ → Starts server                     │
└─────────────────────────────────────┘
```

**Key Improvement**: No extraction init container needed! We generate `config.yaml (with module: field)` from metadata and merge with user ConfigMap (if provided).

---

## Integration Point 1: CRD API Types

**File**: `api/v1alpha1/llamastackdistribution_types.go`

**Current State**:
- Lines 75-88: `ServerSpec` struct
- Lines 196-210: `LlamaStackDistributionStatus` struct

**Required Changes**:
1. Add `ExternalProviders *ExternalProvidersSpec` field to `ServerSpec` (after line 87)
2. Add `ExternalProviderStatus []ExternalProviderStatus` field to `LlamaStackDistributionStatus` (after line 209)
3. Add new structs: `ExternalProvidersSpec`, `ExternalProviderRef`, `ExternalProviderStatus`

**Example Integration**:
```go
type ServerSpec struct {
    Distribution  DistributionType `json:"distribution"`
    ContainerSpec ContainerSpec    `json:"containerSpec,omitempty"`
    PodOverrides  *PodOverrides    `json:"podOverrides,omitempty"`
    Storage       *StorageSpec     `json:"storage,omitempty"`
    UserConfig    *UserConfigSpec  `json:"userConfig,omitempty"`
    TLSConfig     *TLSConfig       `json:"tlsConfig,omitempty"`
    // NEW: External provider configuration
    ExternalProviders *ExternalProvidersSpec `json:"externalProviders,omitempty"`
}

type ExternalProvidersSpec struct {
    Inference    []ExternalProviderRef `json:"inference,omitempty"`
    Safety       []ExternalProviderRef `json:"safety,omitempty"`
    Agents       []ExternalProviderRef `json:"agents,omitempty"`
    VectorIO     []ExternalProviderRef `json:"vectorIo,omitempty"`
    DatasetIO    []ExternalProviderRef `json:"datasetIo,omitempty"`
    Scoring      []ExternalProviderRef `json:"scoring,omitempty"`
    Eval         []ExternalProviderRef `json:"eval,omitempty"`
    ToolRuntime  []ExternalProviderRef `json:"toolRuntime,omitempty"`
    PostTraining []ExternalProviderRef `json:"postTraining,omitempty"`
}

type ExternalProviderRef struct {
    // +kubebuilder:validation:Required
    // +kubebuilder:validation:Pattern=`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`
    ProviderID string `json:"providerId"`

    // +kubebuilder:validation:Required
    Image string `json:"image"`

    // +kubebuilder:default=IfNotPresent
    // +kubebuilder:validation:Enum=Always;Never;IfNotPresent
    ImagePullPolicy corev1.PullPolicy `json:"imagePullPolicy,omitempty"`

    Config *apiextensionsv1.JSON `json:"config,omitempty"`
}

type ExternalProviderStatus struct {
    ProviderID         string      `json:"providerId"`
    Image              string      `json:"image"`
    Phase              string      `json:"phase"` // Pending, Installing, Ready, Failed
    Message            string      `json:"message,omitempty"`
    InitContainerName  string      `json:"initContainerName,omitempty"`
    LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
}
```

**Note**: No per-provider imagePullSecrets field - uses pod-level secrets

**Dependencies**: None - can be done first

---

## Integration Point 2: Generate Init Containers (Explicit Step)

**File**: `controllers/llamastackdistribution_controller.go`
**Function**: `buildManifestContext()` (lines 323-366)

**Current Flow**:
```go
container := buildContainerSpec(ctx, r, instance, resolvedImage)
podSpec := configurePodStorage(ctx, r, instance, container)
```

**Required Changes**:
Add explicit init container generation as a **prominent, visible step**:

```go
// Build main container
container := buildContainerSpec(ctx, r, instance, resolvedImage)

// NEW: Build init containers for external providers (EXPLICIT STEP)
var initContainers []corev1.Container
if instance.Spec.Server.ExternalProviders != nil {
    initContainers = buildExternalProviderInitContainers(ctx, r, instance, resolvedImage)
}

// Build pod spec with both main container and init containers
podSpec := configurePodStorage(ctx, r, instance, container, initContainers)
```

**Why This Approach**:
- ✅ Init container generation is **explicit and visible** in the main deployment setup flow
- ✅ Clear separation of concerns (not hidden as side-effect in another function)
- ✅ Easy to understand the deployment sequence
- ✅ Follows pattern: build container → build init containers → assemble pod spec

**Dependencies**:
- Integration Point 1 (CRD types)
- New function: `buildExternalProviderInitContainers()` (see Integration Point 4)

---

## Integration Point 3: Assemble Pod Spec

**File**: `controllers/resource_helper.go`
**Function**: `configurePodStorage()` (lines 390-408)

**Current Signature**:
```go
func configurePodStorage(ctx context.Context, r *LlamaStackDistributionReconciler,
    instance *llamav1alpha1.LlamaStackDistribution, container corev1.Container) corev1.PodSpec
```

**Required Changes**:

**Update signature** to explicitly accept init containers:
```go
func configurePodStorage(ctx context.Context, r *LlamaStackDistributionReconciler,
    instance *llamav1alpha1.LlamaStackDistribution,
    container corev1.Container,
    initContainers []corev1.Container) corev1.PodSpec
```

**Implementation changes**:
```go
podSpec := corev1.PodSpec{
    Containers:     []corev1.Container{container},
    InitContainers: initContainers, // NEW: explicitly set from parameter
}

configureStorage(instance, &podSpec)

// NEW: Configure external provider volumes (if any external providers exist)
if instance.Spec.Server.ExternalProviders != nil {
    configureExternalProviderVolumes(instance, &podSpec)
}

configureTLSCABundle(ctx, r, instance, &podSpec, image)

// MODIFIED: Configure user config as source for merge (optional)
configureUserConfigSource(instance, &podSpec)

configurePodOverrides(instance, &podSpec)

return podSpec
```

**Dependencies**:
- Integration Point 2 (init container generation)
- New function: `configureExternalProviderVolumes()` (Integration Point 4)

---

## Integration Point 4: Init Container & Volume Functions

**File**: `controllers/resource_helper.go` or new file `pkg/deploy/external_providers.go`

### Function 1: `buildExternalProviderInitContainers()`

**Purpose**: Build init containers for external provider support (2 phases)

**Implementation**:
```go
func buildExternalProviderInitContainers(
    ctx context.Context,
    r *LlamaStackDistributionReconciler,
    instance *llamav1alpha1.LlamaStackDistribution,
    distributionImage string,
) []corev1.Container {
    var initContainers []corev1.Container

    // Phase 1: Install provider packages (in CRD order)
    providerContainers := createProviderInstallInitContainers(instance)
    initContainers = append(initContainers, providerContainers...)

    // Phase 2: Generate config.yaml (with module: field) and merge with user config
    mergeContainer := createMergeConfigInitContainer(instance, r.getOperatorImage())
    initContainers = append(initContainers, mergeContainer)

    return initContainers
}
```

### Phase 1: Provider Install Init Containers

```go
func createProviderInstallInitContainers(instance *llamav1alpha1.LlamaStackDistribution) []corev1.Container {
    var initContainers []corev1.Container

    // Collect all providers in CRD order
    allProviders := collectProvidersInCRDOrder(instance)

    // Generate one init container per provider
    for _, provider := range allProviders {
        initContainers = append(initContainers, createProviderInstallInitContainer(provider))
    }

    return initContainers
}

func collectProvidersInCRDOrder(instance *llamav1alpha1.LlamaStackDistribution) []providerWithAPI {
    var providers []providerWithAPI

    // Follow the order in ExternalProvidersSpec struct definition
    if instance.Spec.Server.ExternalProviders.Inference != nil {
        for _, p := range instance.Spec.Server.ExternalProviders.Inference {
            providers = append(providers, providerWithAPI{ref: p, api: "inference"})
        }
    }
    if instance.Spec.Server.ExternalProviders.Safety != nil {
        for _, p := range instance.Spec.Server.ExternalProviders.Safety {
            providers = append(providers, providerWithAPI{ref: p, api: "safety"})
        }
    }
    // ... continue for all API types in struct order

    return providers
}

func createProviderInstallInitContainer(provider providerWithAPI) corev1.Container {
    script := fmt.Sprintf(`
set -e
echo "Installing external provider: %s"

# Validate metadata file exists
if [ ! -f /lls-provider/lls-provider-spec.yaml ]; then
  echo "ERROR: Missing /lls-provider/lls-provider-spec.yaml in image %s"
  exit 1
fi

# Validate package directory exists
if [ ! -d /lls-provider/packages ]; then
  echo "ERROR: Missing /lls-provider/packages/ directory in image %s"
  exit 1
fi

# Install all wheels to shared location
echo "Installing provider packages..."
pip install /lls-provider/packages/*.whl \
  --target /opt/external-providers/python-packages \
  --no-index \
  --find-links /lls-provider/packages \
  --no-cache-dir \
  --disable-pip-version-check

# Copy metadata for merge step
mkdir -p /opt/external-providers/metadata
cp /lls-provider/lls-provider-spec.yaml /opt/external-providers/metadata/%s.yaml

echo "Successfully installed provider: %s"
`, provider.ref.ProviderID, provider.ref.Image, provider.ref.Image, provider.ref.ProviderID, provider.ref.ProviderID)

    pullPolicy := provider.ref.ImagePullPolicy
    if pullPolicy == "" {
        pullPolicy = corev1.PullIfNotPresent
    }

    return corev1.Container{
        Name:            fmt.Sprintf("install-provider-%s", provider.ref.ProviderID),
        Image:           provider.ref.Image,
        ImagePullPolicy: pullPolicy,
        Command:         []string{"/bin/sh", "-c", script},
        VolumeMounts: []corev1.VolumeMount{
            {
                Name:      "external-providers",
                MountPath: "/opt/external-providers",
            },
        },
        SecurityContext: &corev1.SecurityContext{
            RunAsNonRoot:             ptr.To(true),
            RunAsUser:                ptr.To(int64(1001)),
            AllowPrivilegeEscalation: ptr.To(false),
            Capabilities: &corev1.Capabilities{
                Drop: []corev1.Capability{"ALL"},
            },
        },
        Resources: corev1.ResourceRequirements{
            Requests: corev1.ResourceList{
                corev1.ResourceCPU:    resource.MustParse("100m"),
                corev1.ResourceMemory: resource.MustParse("256Mi"),
            },
            Limits: corev1.ResourceList{
                corev1.ResourceMemory: resource.MustParse("512Mi"),
            },
        },
    }
}
```

### Phase 2: Merge Config Init Container

```go
func createMergeConfigInitContainer(
    instance *llamav1alpha1.LlamaStackDistribution,
    operatorImage string,
) corev1.Container {
    // Build command arguments
    args := []string{
        "--metadata-dir=/opt/external-providers/metadata",
        "--extra-providers-output=/shared/config.yaml (with module: field)",
        "--output=/shared/final/config.yaml",
    }

    // Add user config if exists (this becomes the base for merge)
    if hasValidUserConfig(instance) {
        args = append(args, "--base=/etc/user-config-source/config.yaml")
    }

    volumeMounts := []corev1.VolumeMount{
        {
            Name:      "config-merge",
            MountPath: "/shared",
        },
        {
            Name:      "external-providers",
            MountPath: "/opt/external-providers",
            ReadOnly:  true,
        },
    }

    // Add user config volume mount if exists
    if hasValidUserConfig(instance) {
        volumeMounts = append(volumeMounts, corev1.VolumeMount{
            Name:      "user-config-source",
            MountPath: "/etc/user-config-source",
            ReadOnly:  true,
        })
    }

    return corev1.Container{
        Name:         "merge-config",
        Image:        operatorImage,
        Command:      []string{"/usr/local/bin/generate-config"},
        Args:         args,
        VolumeMounts: volumeMounts,
        SecurityContext: &corev1.SecurityContext{
            RunAsNonRoot:             ptr.To(true),
            RunAsUser:                ptr.To(int64(1001)),
            AllowPrivilegeEscalation: ptr.To(false),
            Capabilities: &corev1.Capabilities{
                Drop: []corev1.Capability{"ALL"},
            },
        },
        Resources: corev1.ResourceRequirements{
            Requests: corev1.ResourceList{
                corev1.ResourceCPU:    resource.MustParse("100m"),
                corev1.ResourceMemory: resource.MustParse("256Mi"),
            },
            Limits: corev1.ResourceList{
                corev1.ResourceMemory: resource.MustParse("512Mi"),
            },
        },
    }
}

func (r *LlamaStackDistributionReconciler) getOperatorImage() string {
    operatorImage := os.Getenv("OPERATOR_IMAGE")
    if operatorImage == "" {
        operatorImage = "ghcr.io/llamastack/llama-stack-k8s-operator:latest"
    }
    return operatorImage
}
```

### Function 2: `configureExternalProviderVolumes()`

**Purpose**: Add volumes for external provider support

```go
func configureExternalProviderVolumes(
    instance *llamav1alpha1.LlamaStackDistribution,
    podSpec *corev1.PodSpec,
) {
    // Volume 1: emptyDir for config merge process
    podSpec.Volumes = append(podSpec.Volumes, corev1.Volume{
        Name: "config-merge",
        VolumeSource: corev1.VolumeSource{
            EmptyDir: &corev1.EmptyDirVolumeSource{},
        },
    })

    // Volume 2: emptyDir for external provider packages
    podSpec.Volumes = append(podSpec.Volumes, corev1.Volume{
        Name: "external-providers",
        VolumeSource: corev1.VolumeSource{
            EmptyDir: &corev1.EmptyDirVolumeSource{},
        },
    })

    // Add volume mounts to main container
    for i := range podSpec.Containers {
        // Mount merged config
        podSpec.Containers[i].VolumeMounts = append(podSpec.Containers[i].VolumeMounts, corev1.VolumeMount{
            Name:      "config-merge",
            MountPath: "/etc/llama-stack/config.yaml",
            SubPath:   "final/config.yaml",
            ReadOnly:  true,
        })

        // Mount external provider packages (for PYTHONPATH)
        podSpec.Containers[i].VolumeMounts = append(podSpec.Containers[i].VolumeMounts, corev1.VolumeMount{
            Name:      "external-providers",
            MountPath: "/opt/external-providers",
            ReadOnly:  true,
        })
    }
}
```

**Volumes Summary**:
- `config-merge` - emptyDir for merge process (config.yaml (with module: field) + final/config.yaml)
- `external-providers` - emptyDir for provider packages + metadata
- `user-config-source` - ConfigMap for user-provided config.yaml (if exists)

**Dependencies**: Integration Point 3

---

## Integration Point 5: Container Environment Variables

**File**: `controllers/resource_helper.go`
**Function**: `configureContainerEnvironment()` (lines 171-203)

**Current Logic**:
```go
// Set HF_HOME
// Set SSL_CERT_FILE (if TLS config)
// Add user env vars
```

**Required Changes**:
Add PYTHONPATH configuration before user env vars (after line 199):

```go
// Set SSL_CERT_FILE for custom CA bundle
if r.hasCABundleConfigMap(instance) {
    container.Env = append(container.Env, corev1.EnvVar{
        Name:  "SSL_CERT_FILE",
        Value: CABundleMountPath,
    })
}

// NEW: Set PYTHONPATH for external providers
if instance.Spec.Server.ExternalProviders != nil {
    addExternalProvidersToPythonPath(container)
}

// Append user-provided environment variables
container.Env = append(container.Env, instance.Spec.Server.ContainerSpec.Env...)
```

**New Helper Function**:
```go
func addExternalProvidersToPythonPath(container *corev1.Container) {
    externalPath := "/opt/external-providers/python-packages"

    // Find existing PYTHONPATH and prepend our path
    for i := range container.Env {
        if container.Env[i].Name == "PYTHONPATH" {
            if container.Env[i].Value == "" {
                container.Env[i].Value = externalPath
            } else {
                // Prepend external providers path
                container.Env[i].Value = externalPath + ":" + container.Env[i].Value
            }
            return
        }
    }

    // PYTHONPATH not found, create it
    container.Env = append(container.Env, corev1.EnvVar{
        Name:  "PYTHONPATH",
        Value: externalPath,
    })
}
```

**Dependencies**: Integration Point 1

---

## Integration Point 6: User Config as Source (Optional Merge Input)

**File**: `controllers/resource_helper.go`
**Function**: `configureUserConfig()` (lines 551-568)

**Current Function**: Mounts user ConfigMap at `/etc/llama-stack/`

**Required Changes**:

**Rename function** to `configureUserConfigSource()` and change behavior:

```go
func configureUserConfigSource(instance *llamav1alpha1.LlamaStackDistribution, podSpec *corev1.PodSpec) {
    // Only add user config volume if BOTH:
    // 1. User provided a ConfigMap
    // 2. External providers exist (otherwise config is mounted directly by existing logic)
    if !hasValidUserConfig(instance) || instance.Spec.Server.ExternalProviders == nil {
        return
    }

    configMapName := instance.Spec.Server.UserConfig.ConfigMapName

    // Add ConfigMap volume as SOURCE for merge (not main config)
    podSpec.Volumes = append(podSpec.Volumes, corev1.Volume{
        Name: "user-config-source",
        VolumeSource: corev1.VolumeSource{
            ConfigMap: &corev1.ConfigMapVolumeSource{
                LocalObjectReference: corev1.LocalObjectReference{
                    Name: configMapName,
                },
            },
        },
    })

    // Note: This volume is mounted ONLY in the config generation init container
    // Main container mounts the merged config from config-merge volume
}
```

**Important**: When external providers exist, user ConfigMap is:
- Mounted in config generation init container at `/etc/user-config-source/`
- NOT mounted in main container
- Used as base for merging with config.yaml (with module: field)

**Dependencies**: Integration Point 4

---

## Integration Point 7: Status Updates

**File**: `controllers/llamastackdistribution_controller.go`
**Function**: `updateStatus()` (lines 899-956)

**Required Changes**:
Add external provider status tracking (after line 919):

```go
r.updateDeploymentStatus(ctx, instance)
r.updateStorageStatus(ctx, instance)
r.updateServiceStatus(ctx, instance)

// NEW: Update external provider status
if instance.Spec.Server.ExternalProviders != nil {
    if err := r.updateExternalProviderStatus(ctx, instance); err != nil {
        logger.Error(err, "failed to update external provider status")
    }
}

r.updateDistributionConfig(ctx, instance)
```

**New Function**: See Integration Points v3 for full implementation

**Dependencies**: Integration Point 1

---

## Integration Point 8: Status Conditions

**File**: `controllers/status.go`

**Required Changes**: See Integration Points v3 - no changes from previous version

**Dependencies**: Integration Point 7

---

## Integration Point 9: ConfigMap Hash for Rolling Updates

**File**: `controllers/llamastackdistribution_controller.go`
**Function**: `buildManifestContext()` (lines 323-366)

**Required Changes**: See Integration Points v3 - no changes from previous version

**Dependencies**: Integration Point 1

---

## Integration Point 10: Merge Tool in Operator Image

**New Component**: `cmd/generate-config/main.go` in operator repository

**Purpose**: Generate `config.yaml (with module: field)` from metadata, merge with user config.yaml

**Implementation**:
```go
package main

import (
    "flag"
    "fmt"
    "io/ioutil"
    "os"
    "path/filepath"

    "gopkg.in/yaml.v3"
    "github.com/llamastack/llama-stack-k8s-operator/pkg/deploy"
    "github.com/llamastack/llama-stack-k8s-operator/pkg/provider"
)

func main() {
    basePath := flag.String("base", "", "Path to user config.yaml (optional - if not provided, only extra-providers)")
    metadataDir := flag.String("metadata-dir", "", "Directory containing provider metadata files")
    extraProvidersOutput := flag.String("extra-providers-output", "", "Path to write config.yaml (with module: field)")
    outputPath := flag.String("output", "", "Path to write final merged config.yaml")

    flag.Parse()

    if *metadataDir == "" || *extraProvidersOutput == "" || *outputPath == "" {
        fmt.Fprintf(os.Stderr, "Usage: generate-config --metadata-dir=<dir> --extra-providers-output=<path> --output=<path> [--base=<path>]\n")
        os.Exit(1)
    }

    // Step 1: Generate config.yaml (with module: field) from provider metadata
    fmt.Println("Generating config.yaml (with module: field) from provider metadata...")
    extraProviders, err := provider.GenerateExtraProvidersFromMetadata(*metadataDir)
    if err != nil {
        fmt.Fprintf(os.Stderr, "ERROR: Failed to generate config.yaml (with module: field): %v\n", err)
        os.Exit(1)
    }

    // Write config.yaml (with module: field)
    extraProvidersData, err := yaml.Marshal(extraProviders)
    if err != nil {
        fmt.Fprintf(os.Stderr, "ERROR: Failed to serialize config.yaml (with module: field): %v\n", err)
        os.Exit(1)
    }

    if err := os.MkdirAll(filepath.Dir(*extraProvidersOutput), 0755); err != nil {
        fmt.Fprintf(os.Stderr, "ERROR: Failed to create output directory: %v\n", err)
        os.Exit(1)
    }

    if err := ioutil.WriteFile(*extraProvidersOutput, extraProvidersData, 0644); err != nil {
        fmt.Fprintf(os.Stderr, "ERROR: Failed to write config.yaml (with module: field): %v\n", err)
        os.Exit(1)
    }

    fmt.Printf("✓ Generated config.yaml (with module: field): %s\n", *extraProvidersOutput)

    // Step 2: Merge with base config.yaml (if provided)
    var baseConfig *deploy.RunYamlConfig

    if *basePath != "" {
        fmt.Printf("Reading base config.yaml from: %s\n", *basePath)
        baseData, err := ioutil.ReadFile(*basePath)
        if err != nil {
            fmt.Fprintf(os.Stderr, "ERROR: Failed to read base config.yaml: %v\n", err)
            os.Exit(1)
        }

        baseConfig = &deploy.RunYamlConfig{}
        if err := yaml.Unmarshal(baseData, baseConfig); err != nil {
            fmt.Fprintf(os.Stderr, "ERROR: Failed to parse base config.yaml: %v\n", err)
            os.Exit(1)
        }
    } else {
        // No base config - create minimal structure
        fmt.Println("No base config.yaml provided, creating minimal structure")
        baseConfig = &deploy.RunYamlConfig{
            Version:   2,
            Providers: make(map[string][]deploy.ProviderConfigEntry),
        }
    }

    // Merge extra-providers into base
    mergedConfig, warnings, err := deploy.MergeExtraProviders(baseConfig, extraProviders)
    if err != nil {
        fmt.Fprintf(os.Stderr, "ERROR: Failed to merge configurations: %v\n", err)
        os.Exit(1)
    }

    // Log warnings
    for _, warning := range warnings {
        fmt.Fprintf(os.Stderr, "WARNING: %s\n", warning)
    }

    // Write final merged config.yaml
    mergedData, err := yaml.Marshal(mergedConfig)
    if err != nil {
        fmt.Fprintf(os.Stderr, "ERROR: Failed to serialize merged config.yaml: %v\n", err)
        os.Exit(1)
    }

    if err := os.MkdirAll(filepath.Dir(*outputPath), 0755); err != nil {
        fmt.Fprintf(os.Stderr, "ERROR: Failed to create output directory: %v\n", err)
        os.Exit(1)
    }

    if err := ioutil.WriteFile(*outputPath, mergedData, 0644); err != nil {
        fmt.Fprintf(os.Stderr, "ERROR: Failed to write merged config.yaml: %v\n", err)
        os.Exit(1)
    }

    fmt.Printf("✓ Generated merged config.yaml: %s\n", *outputPath)
    fmt.Println("Merge completed successfully!")
}
```

**Operator Dockerfile Changes**:
```dockerfile
# Build merge tool
FROM golang:1.21 AS merge-builder
WORKDIR /workspace
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o /generate-config ./cmd/generate-config

# Final operator image
FROM gcr.io/distroless/static:nonroot
WORKDIR /
COPY --from=builder /workspace/manager .
COPY --from=merge-builder /generate-config /usr/local/bin/generate-config
USER 65532:65532
ENTRYPOINT ["/manager"]
```

**Dependencies**: pkg/deploy/runyaml.go, pkg/provider/extra_providers.go

---

## Summary of Integration Points

| # | Integration Point | File | Function | Type |
|---|-------------------|------|----------|------|
| 1 | CRD API Types | `api/v1alpha1/llamastackdistribution_types.go` | Add structs | New fields |
| 2 | Init Container Generation | `controllers/llamastackdistribution_controller.go` | `buildManifestContext()` | New code |
| 3 | Pod Spec Assembly | `controllers/resource_helper.go` | `configurePodStorage()` | Modified signature |
| 4 | Init Container Functions | `controllers/resource_helper.go` | New functions (2 phases) | New code |
| 5 | Environment Variables | `controllers/resource_helper.go` | `configureContainerEnvironment()` | New code |
| 6 | User Config Source | `controllers/resource_helper.go` | `configureUserConfigSource()` | Rename + modify |
| 7 | Status Updates | `controllers/llamastackdistribution_controller.go` | `updateStatus()` | New code |
| 8 | Status Conditions | `controllers/status.go` | New constant + function | New code |
| 9 | ConfigMap Hash | `controllers/llamastackdistribution_controller.go` | `buildManifestContext()` | New code |
| 10 | Merge Tool Binary | `cmd/generate-config/main.go` | New binary | New file |

---

## New Files Required

1. `pkg/provider/metadata.go` - Provider metadata parsing
2. `pkg/provider/extra_providers.go` - config.yaml (with module: field) generation from metadata
3. `pkg/deploy/runyaml.go` - config.yaml merging logic
4. `cmd/generate-config/main.go` - Merge tool binary (included in operator image)
5. `controllers/external_providers.go` - Status tracking (optional)
6. `tests/unit/metadata_test.go` - Metadata parsing tests
7. `tests/unit/extra_providers_test.go` - config.yaml (with module: field) generation tests
8. `tests/unit/merge_test.go` - config.yaml merge tests
9. `tests/integration/external_providers_test.go` - Integration tests

---

## Modified Files Summary

| File | Changes | Complexity |
|------|---------|------------|
| `api/v1alpha1/llamastackdistribution_types.go` | Add 3 structs, ~100 lines | Low |
| `controllers/llamastackdistribution_controller.go` | 3 integration points, ~100 lines | Medium |
| `controllers/resource_helper.go` | 4 integration points, ~400 lines | High |
| `controllers/status.go` | Add constant + function, ~40 lines | Low |
| `pkg/deploy/kustomizer.go` | Add hash annotation, ~5 lines | Low |
| `Dockerfile` | Add merge binary, ~2 lines | Low |

**Total Modified Files**: 6 existing files
**Total New Files**: 9 new files
**Estimated Total Lines of New/Modified Code**: ~1200-1500 lines (excluding tests)

---

## Init Container Execution Order (Final)

```
1. install-provider-<id-1> (provider image 1)
   ↓ Writes: /opt/external-providers/python-packages/*, /opt/external-providers/metadata/provider-1.yaml

2. install-provider-<id-2> (provider image 2)
   ↓ Writes: /opt/external-providers/python-packages/*, /opt/external-providers/metadata/provider-2.yaml

... (one per provider, in CRD order)

N. merge-config (operator image)
   ↓ Reads: /etc/user-config-source/config.yaml (optional), /opt/external-providers/metadata/*.yaml
   ↓ Generates: /shared/config.yaml (with module: field)
   ↓ Merges: user config + extra-providers
   ↓ Writes: /shared/final/config.yaml

MAIN CONTAINER
   ↓ Uses: /etc/llama-stack/config.yaml (from /shared/final/config.yaml)
   ↓ PYTHONPATH: /opt/external-providers/python-packages
```

---

## Forward Compatibility (Phase 2 - Future)

When LlamaStack adds `--extra-providers` support, migration is simple:

**Remove**: Merge init container
**Update**: Main container args to include `--extra-providers /etc/extra-providers/config.yaml (with module: field)`
**Mount**: `config.yaml (with module: field)` generated by merge tool (which becomes a simple generator, not merger)

See `extra-providers-schema.md` for full migration path.

---

## References

- Init Container Pattern: `createCABundleInitContainer()` (resource_helper.go:321-387)
- Volume Mount Pattern: `configureUserConfig()` (resource_helper.go:551-568)
- Status Condition Pattern: `SetDeploymentReadyCondition()` (status.go:65-81)
- Hash Calculation Pattern: `getConfigMapHash()` (llamastackdistribution_controller.go:1173-1191)
- Extra Providers Schema: `extra-providers-schema.md` (this spec directory)

---

## Critical Architecture Decisions (Final)

1. ✅ **config.yaml (with module: field) schema** - Forward-compatible, enables Phase 2 migration
2. ✅ **Two-phase init containers** - Install → Merge (no extraction needed!)
3. ✅ **Merge tool in operator image** - Reuses logic, generates config.yaml (with module: field)
4. ✅ **Init containers in CRD order** - User-controlled, predictable
5. ✅ **Pod-level imagePullSecrets** - Simpler, reuses existing mechanism
6. ✅ **Explicit init container generation** - Clear, not hidden as side-effect
7. ✅ **Hardcoded resource limits** - 100m CPU, 256Mi memory for all init containers
8. ✅ **User ConfigMap as optional merge input** - Only mounted if exists AND external providers exist
9. ✅ **No config.yaml extraction** - Cleaner, more robust, prepares for future LlamaStack enhancement
