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

package deploy

import (
	"encoding/json"
	"fmt"

	llamav1alpha1 "github.com/llamastack/llama-stack-k8s-operator/api/v1alpha1"
	"github.com/llamastack/llama-stack-k8s-operator/pkg/provider"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	// ExternalProvidersVolumeName is the name of the shared volume for external providers.
	ExternalProvidersVolumeName = "external-providers"
	// ExternalProvidersMountPath is the mount path in the main container.
	ExternalProvidersMountPath = "/opt/llama-stack/external-providers"
	// ExternalProvidersPythonPackagesPath is the path where Python packages are installed.
	ExternalProvidersPythonPackagesPath = "/opt/llama-stack/external-providers/python-packages"
	// ExternalProvidersMetadataPath is the path where provider metadata is stored.
	ExternalProvidersMetadataPath = "/opt/llama-stack/external-providers/metadata"
	// BaseConfigMountPath is the mount path for base configuration.
	BaseConfigMountPath = "/opt/llama-stack/base-config"
	// FinalConfigMountPath is the mount path for the final generated configuration.
	FinalConfigMountPath = "/opt/llama-stack/config"
	// DefaultVolumeSizeLimit is the default size limit for the external providers volume.
	DefaultVolumeSizeLimit = "2Gi"
	// InitContainerPrefix is the prefix for provider init container names.
	InitContainerPrefix = "install-provider-"
	// ExtractConfigContainerName is the name of the config extraction init container.
	ExtractConfigContainerName = "extract-distribution-config"
	// GenerateConfigContainerName is the name of the config generation init container.
	GenerateConfigContainerName = "generate-config"
)

// providerWithAPI associates an external provider reference with its API type.
type providerWithAPI struct {
	ref llamav1alpha1.ExternalProviderRef
	api string
}

// GenerateInitContainers creates init containers for external providers.
// Returns containers in CRD order (the order they appear in the spec) for determinism.
// The order is: provider install containers, then config extraction (if needed), then config generation.
func GenerateInitContainers(instance *llamav1alpha1.LlamaStackDistribution, operatorImage string) []corev1.Container {
	if instance.Spec.Server.ExternalProviders == nil {
		return nil
	}

	// Collect all providers from all API types in CRD order
	allProviders := collectProvidersInCRDOrder(instance.Spec.Server.ExternalProviders)
	if len(allProviders) == 0 {
		return nil
	}

	containers := make([]corev1.Container, 0, len(allProviders)+2)

	// Phase 1: Provider install containers (one per provider, in CRD order)
	for _, p := range allProviders {
		containers = append(containers, generateProviderInitContainer(p.ref, p.api))
	}

	// Phase 2: Config extraction container (if no user ConfigMap)
	// This will be conditionally added by the controller based on userConfig settings

	// Phase 3: Config generation container (uses operator image)
	containers = append(containers, generateMergeConfigInitContainer(operatorImage))

	return containers
}

// collectProvidersInCRDOrder collects all external providers maintaining CRD specification order.
// Providers are collected in the order they appear in the CRD: inference, safety, agents, etc.
func collectProvidersInCRDOrder(ep *llamav1alpha1.ExternalProvidersSpec) []providerWithAPI {
	// Calculate total count for preallocation
	totalCount := len(ep.Inference) + len(ep.Safety) + len(ep.Agents) + len(ep.VectorIO) +
		len(ep.DatasetIO) + len(ep.Scoring) + len(ep.Eval) + len(ep.ToolRuntime) + len(ep.PostTraining)

	providers := make([]providerWithAPI, 0, totalCount)

	// Define API types in CRD order with their corresponding slices
	apiProviders := []struct {
		refs []llamav1alpha1.ExternalProviderRef
		api  string
	}{
		{ep.Inference, "inference"},
		{ep.Safety, "safety"},
		{ep.Agents, "agents"},
		{ep.VectorIO, "vectorIo"},
		{ep.DatasetIO, "datasetIo"},
		{ep.Scoring, "scoring"},
		{ep.Eval, "eval"},
		{ep.ToolRuntime, "toolRuntime"},
		{ep.PostTraining, "postTraining"},
	}

	for _, ap := range apiProviders {
		for _, p := range ap.refs {
			providers = append(providers, providerWithAPI{ref: p, api: ap.api})
		}
	}

	return providers
}

// generateProviderInitContainer creates an init container for a single external provider.
// The container installs the provider's wheel packages and copies metadata to the shared volume.
func generateProviderInitContainer(ref llamav1alpha1.ExternalProviderRef, apiType string) corev1.Container {
	initScript := buildProviderInstallScript(ref, apiType)

	pullPolicy := ref.ImagePullPolicy
	if pullPolicy == "" {
		pullPolicy = corev1.PullIfNotPresent
	}

	return corev1.Container{
		Name:            InitContainerPrefix + ref.ProviderID,
		Image:           ref.Image,
		ImagePullPolicy: pullPolicy,
		Command:         []string{"/bin/sh", "-c", initScript},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      ExternalProvidersVolumeName,
				MountPath: ExternalProvidersMountPath,
			},
		},
	}
}

// buildProviderInstallScript generates the shell script for installing a provider.
func buildProviderInstallScript(ref llamav1alpha1.ExternalProviderRef, apiType string) string {
	crdConfigJSON := buildCRDConfigJSON(ref, apiType)

	return fmt.Sprintf(`#!/bin/sh
set -e

PROVIDER_ID="%s"
IMAGE="%s"
METADATA_DIR="%s/${PROVIDER_ID}"

echo "Installing external provider: ${PROVIDER_ID}"
echo "Image: ${IMAGE}"

# Validate metadata file exists
if [ ! -f %s ]; then
  echo "ERROR: Missing %s in image ${IMAGE}"
  echo "Resolution: Ensure the provider image contains a valid lls-provider-spec.yaml at %s"
  exit 1
fi

# Validate package directory exists
if [ ! -d %s ]; then
  echo "ERROR: Missing %s directory in image ${IMAGE}"
  echo "Resolution: Ensure the provider image contains wheel packages in %s"
  exit 1
fi

# Create metadata directory for this provider
mkdir -p "${METADATA_DIR}"

# Copy metadata file for validation and config generation
cp %s "${METADATA_DIR}/lls-provider-spec.yaml"
echo "Copied provider metadata to ${METADATA_DIR}/lls-provider-spec.yaml"

# Write CRD config for this provider
cat > "${METADATA_DIR}/crd-config.yaml" << 'CRDCONFIG'
%s
CRDCONFIG
echo "Wrote CRD config to ${METADATA_DIR}/crd-config.yaml"

# Install all wheels to shared location
echo "Installing provider packages..."
pip install %s/*.whl \
  --target %s \
  --no-index \
  --find-links %s \
  --no-cache-dir \
  --disable-pip-version-check \
  2>&1 || {
    echo "ERROR: Failed to install provider packages for ${PROVIDER_ID}"
    echo "Image: ${IMAGE}"
    echo "Resolution: Check wheel compatibility and dependency conflicts"
    exit 1
  }

# Record installed packages for troubleshooting
echo "# Packages installed by provider: ${PROVIDER_ID}" >> %s/installed-packages.txt
pip list --path %s 2>/dev/null >> %s/installed-packages.txt || true

echo "Successfully installed provider: ${PROVIDER_ID}"
`,
		ref.ProviderID,
		ref.Image,
		ExternalProvidersMetadataPath,
		provider.MetadataPath,
		provider.MetadataPath,
		provider.MetadataPath,
		provider.PackagesPath,
		provider.PackagesPath,
		provider.PackagesPath,
		provider.MetadataPath,
		crdConfigJSON,
		provider.PackagesPath,
		ExternalProvidersPythonPackagesPath,
		provider.PackagesPath,
		ExternalProvidersMountPath,
		ExternalProvidersPythonPackagesPath,
		ExternalProvidersMountPath,
	)
}

// buildCRDConfigJSON creates the JSON string for the CRD config.
func buildCRDConfigJSON(ref llamav1alpha1.ExternalProviderRef, apiType string) string {
	normalizedAPI := provider.NormalizeAPIName(apiType)

	crdConfig := map[string]interface{}{
		"providerId": ref.ProviderID,
		"api":        normalizedAPI,
	}

	if ref.Config != nil {
		var configData interface{}
		if err := json.Unmarshal(ref.Config.Raw, &configData); err == nil {
			crdConfig["config"] = configData
		}
	}

	crdConfigJSON, err := json.Marshal(crdConfig)
	if err != nil {
		// Fall back to a minimal valid JSON if marshaling fails
		return fmt.Sprintf(`{"providerId":"%s","api":"%s"}`, ref.ProviderID, normalizedAPI)
	}

	return string(crdConfigJSON)
}

// generateMergeConfigInitContainer creates the init container that generates the final config.yaml.
// This container uses the operator image and runs the generate-config binary.
func generateMergeConfigInitContainer(operatorImage string) corev1.Container {
	return corev1.Container{
		Name:            GenerateConfigContainerName,
		Image:           operatorImage,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Command: []string{
			"/generate-config",
			"--metadata-dir", ExternalProvidersMetadataPath,
			"--base-config", BaseConfigMountPath + "/config.yaml",
			"--output", FinalConfigMountPath + "/config.yaml",
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      ExternalProvidersVolumeName,
				MountPath: ExternalProvidersMountPath,
				ReadOnly:  true,
			},
			{
				Name:      "base-config",
				MountPath: BaseConfigMountPath,
				ReadOnly:  true,
			},
			{
				Name:      "final-config",
				MountPath: FinalConfigMountPath,
			},
		},
	}
}

// GenerateExtractConfigInitContainer creates an init container that extracts config.yaml
// from the distribution image when no user ConfigMap is provided.
func GenerateExtractConfigInitContainer(distributionImage string) corev1.Container {
	extractScript := `#!/bin/sh
set -e

echo "Extracting base config.yaml from distribution image..."

# Try newer path first
if [ -f /opt/app-root/config.yaml ]; then
  echo "Found config.yaml at /opt/app-root/config.yaml"
  cp /opt/app-root/config.yaml /opt/llama-stack/base-config/config.yaml
  exit 0
fi

# Try legacy path
if [ -f /etc/llama-stack/config.yaml ]; then
  echo "Found config.yaml at /etc/llama-stack/config.yaml"
  cp /etc/llama-stack/config.yaml /opt/llama-stack/base-config/config.yaml
  exit 0
fi

# No config.yaml found - fail with error
echo "ERROR: No config.yaml found in distribution image"
echo "Checked paths:"
echo "  - /opt/app-root/config.yaml (not found)"
echo "  - /etc/llama-stack/config.yaml (not found)"
echo ""
echo "Resolution: Either provide a user ConfigMap with config.yaml or use a distribution image that includes config.yaml"
exit 1
`

	return corev1.Container{
		Name:            ExtractConfigContainerName,
		Image:           distributionImage,
		ImagePullPolicy: corev1.PullIfNotPresent,
		Command:         []string{"/bin/sh", "-c", extractScript},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "base-config",
				MountPath: BaseConfigMountPath,
			},
		},
	}
}

// AddExternalProvidersVolume adds the shared emptyDir volume for external providers to the pod spec.
func AddExternalProvidersVolume(podSpec *corev1.PodSpec, sizeLimit *resource.Quantity) {
	var limit *resource.Quantity
	if sizeLimit != nil {
		limit = sizeLimit
	} else {
		defaultLimit := resource.MustParse(DefaultVolumeSizeLimit)
		limit = &defaultLimit
	}

	podSpec.Volumes = append(podSpec.Volumes, corev1.Volume{
		Name: ExternalProvidersVolumeName,
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{
				SizeLimit: limit,
			},
		},
	})
}

// AddBaseConfigVolume adds the base config volume to the pod spec.
// If a ConfigMap name is provided, it mounts from ConfigMap; otherwise, it uses emptyDir
// for extraction from the distribution image.
func AddBaseConfigVolume(podSpec *corev1.PodSpec, configMapName string) {
	var volumeSource corev1.VolumeSource

	if configMapName != "" {
		// Mount from user-provided ConfigMap
		volumeSource = corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: configMapName,
				},
			},
		}
	} else {
		// Use emptyDir for extraction from distribution image
		volumeSource = corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		}
	}

	podSpec.Volumes = append(podSpec.Volumes, corev1.Volume{
		Name:         "base-config",
		VolumeSource: volumeSource,
	})
}

// AddFinalConfigVolume adds the final config volume to the pod spec.
func AddFinalConfigVolume(podSpec *corev1.PodSpec) {
	podSpec.Volumes = append(podSpec.Volumes, corev1.Volume{
		Name: "final-config",
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	})
}

// MountExternalProvidersVolume adds the external providers volume mount to a container.
func MountExternalProvidersVolume(container *corev1.Container, readOnly bool) {
	container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
		Name:      ExternalProvidersVolumeName,
		MountPath: ExternalProvidersMountPath,
		ReadOnly:  readOnly,
	})
}

// MountFinalConfigVolume adds the final config volume mount to a container.
func MountFinalConfigVolume(container *corev1.Container) {
	container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
		Name:      "final-config",
		MountPath: FinalConfigMountPath,
		ReadOnly:  true,
	})
}

// UpdatePythonPath prepends the external providers Python packages path to PYTHONPATH.
func UpdatePythonPath(container *corev1.Container) {
	pythonPath := ExternalProvidersPythonPackagesPath

	// Find existing PYTHONPATH or create new
	found := false
	for i := range container.Env {
		if container.Env[i].Name == "PYTHONPATH" {
			container.Env[i].Value = pythonPath + ":" + container.Env[i].Value
			found = true
			break
		}
	}

	if !found {
		container.Env = append(container.Env, corev1.EnvVar{
			Name:  "PYTHONPATH",
			Value: pythonPath,
		})
	}
}

// HasExternalProviders checks if the instance has any external providers configured.
func HasExternalProviders(instance *llamav1alpha1.LlamaStackDistribution) bool {
	if instance.Spec.Server.ExternalProviders == nil {
		return false
	}

	ep := instance.Spec.Server.ExternalProviders
	return len(ep.Inference) > 0 ||
		len(ep.Safety) > 0 ||
		len(ep.Agents) > 0 ||
		len(ep.VectorIO) > 0 ||
		len(ep.DatasetIO) > 0 ||
		len(ep.Scoring) > 0 ||
		len(ep.Eval) > 0 ||
		len(ep.ToolRuntime) > 0 ||
		len(ep.PostTraining) > 0
}

// GetAllExternalProviders returns all external providers from all API types.
func GetAllExternalProviders(ep *llamav1alpha1.ExternalProvidersSpec) []llamav1alpha1.ExternalProviderRef {
	if ep == nil {
		return nil
	}

	// Calculate total count for preallocation
	totalCount := len(ep.Inference) + len(ep.Safety) + len(ep.Agents) + len(ep.VectorIO) +
		len(ep.DatasetIO) + len(ep.Scoring) + len(ep.Eval) + len(ep.ToolRuntime) + len(ep.PostTraining)

	providers := make([]llamav1alpha1.ExternalProviderRef, 0, totalCount)
	providers = append(providers, ep.Inference...)
	providers = append(providers, ep.Safety...)
	providers = append(providers, ep.Agents...)
	providers = append(providers, ep.VectorIO...)
	providers = append(providers, ep.DatasetIO...)
	providers = append(providers, ep.Scoring...)
	providers = append(providers, ep.Eval...)
	providers = append(providers, ep.ToolRuntime...)
	providers = append(providers, ep.PostTraining...)

	return providers
}
