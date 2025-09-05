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

package controllers

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	llamav1alpha1 "github.com/llamastack/llama-stack-k8s-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

const (
	// Constants for validation limits.
	// maxConfigMapKeyLength defines the maximum allowed length for ConfigMap keys
	// based on Kubernetes DNS subdomain name limits.
	maxConfigMapKeyLength = 253
	// Constants for volume and ConfigMap names.
	// CombinedConfigVolumeName is the name used for the combined configuration volume.
	CombinedConfigVolumeName = "combined-config"
	// Probes configuration.
	startupProbeInitialDelaySeconds = 15 // Time to wait before the first probe
	startupProbeTimeoutSeconds      = 30 // When the probe times out
	startupProbeFailureThreshold    = 3  // Pod is marked Unhealthy after 3 consecutive failures
	startupProbeSuccessThreshold    = 1  // Pod is marked Ready after 1 successful probe
)

// validConfigMapKeyRegex defines allowed characters for ConfigMap keys.
// Kubernetes ConfigMap keys must be valid DNS subdomain names or data keys.
var validConfigMapKeyRegex = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9\-_.]*[a-zA-Z0-9])?$`)

// StartupScript is the script that will be used to start the server.
var StartupScript = `
set -e

    if python -c "
import sys

try:
    from importlib.metadata import version
    from packaging import version as pkg_version

    llama_version = version('llama_stack')
    print(f'Determined llama-stack version {llama_version}')
    if pkg_version.parse(llama_version) < pkg_version.parse('0.2.17'):
        print('llama-stack version is less than 0.2.17 usin old module path llama_stack.distribution.server.server to start the server')
        sys.exit(0)
    else:
        print('llama-stack version is greater than or equal to 0.2.17 using new module path llama_stack.core.server.server to start the server')
        sys.exit(1)
except Exception as e:
    print(f'Failed to determine version: assume newer version if we cannot determine using new module path llama_stack.core.server.server to start the server: {e}')
    sys.exit(1)

"; then
    python3 -m llama_stack.distribution.server.server --config /etc/llama-stack/run.yaml
else
    python3 -m llama_stack.core.server.server /etc/llama-stack/run.yaml
fi`

// validateConfigMapKeys validates that all ConfigMap keys contain only safe characters.
// Note: This function validates key names only. PEM content validation is performed
// separately in the controller's reconcileCABundleConfigMap function.
func validateConfigMapKeys(keys []string) error {
	for _, key := range keys {
		if key == "" {
			return errors.New("ConfigMap key cannot be empty")
		}
		if len(key) > maxConfigMapKeyLength {
			return fmt.Errorf("failed to validate ConfigMap key '%s': too long (max %d characters)", key, maxConfigMapKeyLength)
		}
		if !validConfigMapKeyRegex.MatchString(key) {
			return fmt.Errorf("failed to validate ConfigMap key '%s': contains invalid characters. Only alphanumeric characters, hyphens, underscores, and dots are allowed", key)
		}
		// Additional security check: prevent path traversal attempts
		if strings.Contains(key, "..") || strings.Contains(key, "/") {
			return fmt.Errorf("failed to validate ConfigMap key '%s': contains invalid path characters", key)
		}
	}
	return nil
}

// getHealthProbe returns the health probe handler for the container.
func getHealthProbe(instance *llamav1alpha1.LlamaStackDistribution) corev1.ProbeHandler {
	return corev1.ProbeHandler{
		HTTPGet: &corev1.HTTPGetAction{
			Path: "/v1/health",
			Port: intstr.FromInt(int(getContainerPort(instance))),
		},
	}
}

// getStartupProbe returns the startup probe for the container.
func getStartupProbe(instance *llamav1alpha1.LlamaStackDistribution) *corev1.Probe {
	return &corev1.Probe{
		ProbeHandler:        getHealthProbe(instance),
		InitialDelaySeconds: startupProbeInitialDelaySeconds,
		TimeoutSeconds:      startupProbeTimeoutSeconds,
		FailureThreshold:    startupProbeFailureThreshold,
		SuccessThreshold:    startupProbeSuccessThreshold,
	}
}

// buildContainerSpec creates the container specification.
func buildContainerSpec(instance *llamav1alpha1.LlamaStackDistribution, image string) corev1.Container {
	container := corev1.Container{
		Name:            getContainerName(instance),
		Image:           image,
		Resources:       instance.Spec.Server.ContainerSpec.Resources,
		ImagePullPolicy: corev1.PullAlways,
		Ports:           []corev1.ContainerPort{{ContainerPort: getContainerPort(instance)}},
		StartupProbe:    getStartupProbe(instance),
	}

	// Configure environment variables and mounts
	configureContainerEnvironment(instance, &container)
	configureContainerMounts(instance, &container)
	configureContainerCommands(instance, &container)

	return container
}

// getContainerName returns the container name, using custom name if specified.
func getContainerName(instance *llamav1alpha1.LlamaStackDistribution) string {
	if instance.Spec.Server.ContainerSpec.Name != "" {
		return instance.Spec.Server.ContainerSpec.Name
	}
	return llamav1alpha1.DefaultContainerName
}

// getContainerPort returns the container port, using custom port if specified.
func getContainerPort(instance *llamav1alpha1.LlamaStackDistribution) int32 {
	if instance.Spec.Server.ContainerSpec.Port != 0 {
		return instance.Spec.Server.ContainerSpec.Port
	}
	return llamav1alpha1.DefaultServerPort
}

// configureContainerEnvironment sets up environment variables for the container.
func configureContainerEnvironment(instance *llamav1alpha1.LlamaStackDistribution, container *corev1.Container) {
	mountPath := getMountPath(instance)

	// Add HF_HOME variable to our mount path so that downloaded models and datasets are stored
	// on the same volume as the storage. This is not critical but useful if the server is
	// restarted so the models and datasets are not lost and need to be downloaded again.
	// For more information, see https://huggingface.co/docs/datasets/en/cache
	container.Env = append(container.Env, corev1.EnvVar{
		Name:  "HF_HOME",
		Value: mountPath,
	})

	// Add CA bundle environment variable if TLS config is specified
	if instance.Spec.Server.TLSConfig != nil && instance.Spec.Server.TLSConfig.CABundle != "" {
		// Set SSL_CERT_FILE to point to the specific CA bundle file
		container.Env = append(container.Env, corev1.EnvVar{
			Name:  "SSL_CERT_FILE",
			Value: CABundleMountPath,
		})
	}

	// Finally, add the user provided env vars
	container.Env = append(container.Env, instance.Spec.Server.ContainerSpec.Env...)
}

// configureContainerMounts sets up volume mounts for the container.
func configureContainerMounts(instance *llamav1alpha1.LlamaStackDistribution, container *corev1.Container) {
	// Add volume mount for storage
	addStorageVolumeMount(instance, container)

	// Add ConfigMap volume mount if user config is specified
	addUserConfigVolumeMount(instance, container)

	// Add CA bundle volume mount if TLS config is specified
	addCABundleVolumeMount(instance, container)
}

// configureContainerCommands sets up container commands and args.
func configureContainerCommands(instance *llamav1alpha1.LlamaStackDistribution, container *corev1.Container) {
	// Override the container entrypoint to use the custom config file if user config is specified
	if instance.Spec.Server.UserConfig != nil && instance.Spec.Server.UserConfig.CustomConfig != "" {
		// Override the container entrypoint to use the custom config file instead of the default
		// template. The script will determine the llama-stack version and use the appropriate module
		// path to start the server.

		container.Command = []string{"/bin/sh", "-c", StartupScript}
		container.Args = []string{}
	}

	// Apply user-specified command and args (takes precedence)
	if len(instance.Spec.Server.ContainerSpec.Command) > 0 {
		container.Command = instance.Spec.Server.ContainerSpec.Command
	}

	if len(instance.Spec.Server.ContainerSpec.Args) > 0 {
		container.Args = instance.Spec.Server.ContainerSpec.Args
	}
}

// getMountPath returns the mount path, using custom path if specified.
func getMountPath(instance *llamav1alpha1.LlamaStackDistribution) string {
	if instance.Spec.Server.Storage != nil && instance.Spec.Server.Storage.MountPath != "" {
		return instance.Spec.Server.Storage.MountPath
	}
	return llamav1alpha1.DefaultMountPath
}

// addStorageVolumeMount adds the storage volume mount to the container.
func addStorageVolumeMount(instance *llamav1alpha1.LlamaStackDistribution, container *corev1.Container) {
	mountPath := getMountPath(instance)
	container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
		Name:      "lls-storage",
		MountPath: mountPath,
	})
}

// addUserConfigVolumeMount adds the user config volume mount to the container if specified.
func addUserConfigVolumeMount(instance *llamav1alpha1.LlamaStackDistribution, container *corev1.Container) {
	if instance.Spec.Server.UserConfig != nil && instance.Spec.Server.UserConfig.CustomConfig != "" {
		container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
			Name:      CombinedConfigVolumeName,
			MountPath: "/etc/llama-stack/run.yaml",
			SubPath:   "run.yaml",
			ReadOnly:  true,
		})
	}
}

// addCABundleVolumeMount adds the CA bundle volume mount to the container if TLS config is specified.
// Mounts the combined ConfigMap created from inline PEM data.
func addCABundleVolumeMount(instance *llamav1alpha1.LlamaStackDistribution, container *corev1.Container) {
	if instance.Spec.Server.TLSConfig != nil && instance.Spec.Server.TLSConfig.CABundle != "" {
		container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
			Name:      CombinedConfigVolumeName,
			MountPath: CABundleMountPath,
			SubPath:   DefaultCABundleKey,
			ReadOnly:  true,
		})
	}
}

// createCombinedConfigVolume creates the volume configuration for the operator-managed ConfigMap.
// Used when either user config or CA bundle (or both) are specified.
func createCombinedConfigVolume(instance *llamav1alpha1.LlamaStackDistribution) corev1.Volume {
	configMapName := instance.Name + "-config"
	return corev1.Volume{
		Name: CombinedConfigVolumeName,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: configMapName,
				},
			},
		},
	}
}

// configurePodStorage configures the pod storage and returns the complete pod spec.
func configurePodStorage(instance *llamav1alpha1.LlamaStackDistribution, container corev1.Container) corev1.PodSpec {
	podSpec := corev1.PodSpec{
		Containers: []corev1.Container{container},
	}

	// Configure storage volumes and init containers
	configureStorage(instance, &podSpec)

	// Configure TLS CA bundle
	configureTLSCABundle(instance, &podSpec)

	// Configure user config
	configureUserConfig(instance, &podSpec)

	// Apply pod overrides including ServiceAccount, volumes, and volume mounts
	configurePodOverrides(instance, &podSpec)

	return podSpec
}

// configureStorage handles storage volume configuration.
func configureStorage(instance *llamav1alpha1.LlamaStackDistribution, podSpec *corev1.PodSpec) {
	if instance.Spec.Server.Storage != nil {
		configurePersistentStorage(instance, podSpec)
	} else {
		configureEmptyDirStorage(podSpec)
	}
}

// configurePersistentStorage sets up PVC-based storage with init container for permissions.
func configurePersistentStorage(instance *llamav1alpha1.LlamaStackDistribution, podSpec *corev1.PodSpec) {
	// Use PVC for persistent storage
	podSpec.Volumes = append(podSpec.Volumes, corev1.Volume{
		Name: "lls-storage",
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: instance.Name + "-pvc",
			},
		},
	})

	// Add init container to fix permissions on the PVC mount.
	mountPath := llamav1alpha1.DefaultMountPath
	if instance.Spec.Server.Storage.MountPath != "" {
		mountPath = instance.Spec.Server.Storage.MountPath
	}

	commands := []string{
		fmt.Sprintf("mkdir -p %s 2>&1 || echo 'Warning: Could not create directory'", mountPath),
		fmt.Sprintf("(chown 1001:0 %s 2>&1 || echo 'Warning: Could not change ownership')", mountPath),
		fmt.Sprintf("ls -la %s 2>&1", mountPath),
	}
	command := strings.Join(commands, " && ")

	initContainer := corev1.Container{
		Name:  "update-pvc-permissions",
		Image: "registry.access.redhat.com/ubi9/ubi-minimal:latest",
		Command: []string{
			"/bin/sh",
			"-c",
			// Try to set permissions, but don't fail if we can't
			command,
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "lls-storage",
				MountPath: mountPath,
			},
		},
		SecurityContext: &corev1.SecurityContext{
			RunAsUser:  ptr.To(int64(0)), // Run as root to be able to change ownership
			RunAsGroup: ptr.To(int64(0)),
		},
	}

	podSpec.InitContainers = append(podSpec.InitContainers, initContainer)
}

// configureEmptyDirStorage sets up temporary storage using emptyDir.
func configureEmptyDirStorage(podSpec *corev1.PodSpec) {
	// Use emptyDir for non-persistent storage
	podSpec.Volumes = append(podSpec.Volumes, corev1.Volume{
		Name: "lls-storage",
		VolumeSource: corev1.VolumeSource{
			EmptyDir: &corev1.EmptyDirVolumeSource{},
		},
	})
}

// configureTLSCABundle handles TLS CA bundle configuration.
// Adds the combined ConfigMap volume if CA bundle is explicitly configured.
func configureTLSCABundle(instance *llamav1alpha1.LlamaStackDistribution, podSpec *corev1.PodSpec) {
	tlsConfig := instance.Spec.Server.TLSConfig

	// Handle explicit CA bundle configuration
	if tlsConfig != nil && tlsConfig.CABundle != "" {
		addExplicitCABundle(instance, podSpec)
	}
}

// addExplicitCABundle handles explicitly configured CA bundles.
func addExplicitCABundle(instance *llamav1alpha1.LlamaStackDistribution, podSpec *corev1.PodSpec) {
	// Add combined ConfigMap volume if not already added
	if !hasVolumeWithName(podSpec, CombinedConfigVolumeName) {
		volume := createCombinedConfigVolume(instance)
		podSpec.Volumes = append(podSpec.Volumes, volume)
	}
}

// configureUserConfig handles user configuration setup.
func configureUserConfig(instance *llamav1alpha1.LlamaStackDistribution, podSpec *corev1.PodSpec) {
	userConfig := instance.Spec.Server.UserConfig
	if userConfig == nil || userConfig.CustomConfig == "" {
		return
	}

	// Add combined ConfigMap volume if not already added
	if !hasVolumeWithName(podSpec, CombinedConfigVolumeName) {
		volume := createCombinedConfigVolume(instance)
		podSpec.Volumes = append(podSpec.Volumes, volume)
	}
}

// hasVolumeWithName checks if a pod spec already has a volume with the given name.
func hasVolumeWithName(podSpec *corev1.PodSpec, volumeName string) bool {
	for _, volume := range podSpec.Volumes {
		if volume.Name == volumeName {
			return true
		}
	}
	return false
}

// configurePodOverrides applies pod-level overrides from the LlamaStackDistribution spec.
func configurePodOverrides(instance *llamav1alpha1.LlamaStackDistribution, podSpec *corev1.PodSpec) {
	// Set ServiceAccount name - use override if specified, otherwise use default
	if instance.Spec.Server.PodOverrides != nil && instance.Spec.Server.PodOverrides.ServiceAccountName != "" {
		podSpec.ServiceAccountName = instance.Spec.Server.PodOverrides.ServiceAccountName
	} else {
		podSpec.ServiceAccountName = instance.Name + "-sa"
	}

	// Apply other pod overrides if specified
	if instance.Spec.Server.PodOverrides != nil {
		// Add volumes if specified
		if len(instance.Spec.Server.PodOverrides.Volumes) > 0 {
			podSpec.Volumes = append(podSpec.Volumes, instance.Spec.Server.PodOverrides.Volumes...)
		}

		// Add volume mounts if specified
		if len(instance.Spec.Server.PodOverrides.VolumeMounts) > 0 {
			if len(podSpec.Containers) > 0 {
				podSpec.Containers[0].VolumeMounts = append(podSpec.Containers[0].VolumeMounts, instance.Spec.Server.PodOverrides.VolumeMounts...)
			}
		}
	}
}

// validateDistribution validates the distribution configuration.
func (r *LlamaStackDistributionReconciler) validateDistribution(instance *llamav1alpha1.LlamaStackDistribution) error {
	// If using distribution name, validate it exists in clusterInfo
	if instance.Spec.Server.Distribution.Name != "" {
		if r.ClusterInfo == nil {
			return errors.New("failed to initialize cluster info")
		}
		if _, exists := r.ClusterInfo.DistributionImages[instance.Spec.Server.Distribution.Name]; !exists {
			return fmt.Errorf("failed to validate distribution: %s. Distribution name not supported", instance.Spec.Server.Distribution.Name)
		}
	}

	return nil
}

// resolveImage determines the container image to use based on the distribution configuration.
// It returns the resolved image and any error encountered.
func (r *LlamaStackDistributionReconciler) resolveImage(distribution llamav1alpha1.DistributionType) (string, error) {
	distributionMap := r.ClusterInfo.DistributionImages
	switch {
	case distribution.Name != "":
		if _, exists := distributionMap[distribution.Name]; !exists {
			return "", fmt.Errorf("failed to validate distribution name: %s", distribution.Name)
		}
		return distributionMap[distribution.Name], nil
	case distribution.Image != "":
		return distribution.Image, nil
	default:
		return "", errors.New("failed to validate distribution: either distribution.name or distribution.image must be set")
	}
}
