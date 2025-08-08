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
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	llamav1alpha1 "github.com/llamastack/llama-stack-k8s-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// Constants for validation limits.
const (
	// maxConfigMapKeyLength defines the maximum allowed length for ConfigMap keys
	// based on Kubernetes DNS subdomain name limits.
	maxConfigMapKeyLength = 253
)

// Readiness probe configuration.
const (
	readinessProbeInitialDelaySeconds = 15 // Time to wait before the first probe
	readinessProbePeriodSeconds       = 10 // How often to probe
	readinessProbeTimeoutSeconds      = 5  // When the probe times out
	readinessProbeFailureThreshold    = 3  // Pod is marked Unhealthy after 3 consecutive failures
	readinessProbeSuccessThreshold    = 1  // Pod is marked Ready after 1 successful probe
)

// validConfigMapKeyRegex defines allowed characters for ConfigMap keys.
// Kubernetes ConfigMap keys must be valid DNS subdomain names or data keys.
var validConfigMapKeyRegex = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9\-_.]*[a-zA-Z0-9])?$`)

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

// buildContainerSpec creates the container specification.
func buildContainerSpec(ctx context.Context, r *LlamaStackDistributionReconciler, instance *llamav1alpha1.LlamaStackDistribution, image string) corev1.Container {
	container := corev1.Container{
		Name:            getContainerName(instance),
		Image:           image,
		Resources:       instance.Spec.Server.ContainerSpec.Resources,
		ImagePullPolicy: corev1.PullAlways,
		Ports:           []corev1.ContainerPort{{ContainerPort: getContainerPort(instance)}},
		ReadinessProbe: &corev1.Probe{
			ProbeHandler: corev1.ProbeHandler{
				HTTPGet: &corev1.HTTPGetAction{
					Path: "/v1/health",
					Port: intstr.FromInt(int(getContainerPort(instance))),
				},
			},
			InitialDelaySeconds: readinessProbeInitialDelaySeconds,
			PeriodSeconds:       readinessProbePeriodSeconds,
			TimeoutSeconds:      readinessProbeTimeoutSeconds,
			FailureThreshold:    readinessProbeFailureThreshold,
			SuccessThreshold:    readinessProbeSuccessThreshold,
		},
	}

	// Configure environment variables and mounts
	configureContainerEnvironment(ctx, r, instance, &container)
	configureContainerMounts(ctx, r, instance, &container)
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
func configureContainerEnvironment(ctx context.Context, r *LlamaStackDistributionReconciler, instance *llamav1alpha1.LlamaStackDistribution, container *corev1.Container) {
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
	if instance.Spec.Server.TLSConfig != nil && instance.Spec.Server.TLSConfig.CABundle != nil {
		// Set SSL_CERT_FILE to point to the specific CA bundle file
		container.Env = append(container.Env, corev1.EnvVar{
			Name:  "SSL_CERT_FILE",
			Value: CABundleMountPath,
		})
	} else if r != nil {
		// Check for auto-detected ODH trusted CA bundle
		if _, keys, err := r.detectODHTrustedCABundle(ctx, instance); err == nil && len(keys) > 0 {
			// Set SSL_CERT_FILE to point to the auto-detected consolidated CA bundle
			container.Env = append(container.Env, corev1.EnvVar{
				Name:  "SSL_CERT_FILE",
				Value: CABundleMountPath,
			})
		}
	}

	// Finally, add the user provided env vars
	container.Env = append(container.Env, instance.Spec.Server.ContainerSpec.Env...)
}

// configureContainerMounts sets up volume mounts for the container.
func configureContainerMounts(ctx context.Context, r *LlamaStackDistributionReconciler, instance *llamav1alpha1.LlamaStackDistribution, container *corev1.Container) {
	// Add volume mount for storage
	addStorageVolumeMount(instance, container)

	// Add ConfigMap volume mount if user config is specified
	addUserConfigVolumeMount(instance, container)

	// Add CA bundle volume mount if TLS config is specified or auto-detected
	addCABundleVolumeMount(ctx, r, instance, container)
}

// configureContainerCommands sets up container commands and args.
func configureContainerCommands(instance *llamav1alpha1.LlamaStackDistribution, container *corev1.Container) {
	// Override the container entrypoint to use the custom config file if user config is specified
	if instance.Spec.Server.UserConfig != nil && instance.Spec.Server.UserConfig.ConfigMapName != "" {
		container.Command = []string{"python", "-m", "llama_stack.distribution.server.server"}
		container.Args = []string{"--config", "/etc/llama-stack/run.yaml"}
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
	if instance.Spec.Server.UserConfig != nil && instance.Spec.Server.UserConfig.ConfigMapName != "" {
		container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
			Name:      "user-config",
			MountPath: "/etc/llama-stack/",
			ReadOnly:  true,
		})
	}
}

// addCABundleVolumeMount adds the CA bundle volume mount to the container if TLS config is specified.
// For multiple keys: the init container writes DefaultCABundleKey to the root of the emptyDir volume,
// and the main container mounts it with SubPath to CABundleMountPath.
// For single key: the main container directly mounts the ConfigMap key.
// Also handles auto-detected ODH trusted CA bundle ConfigMaps.
func addCABundleVolumeMount(ctx context.Context, r *LlamaStackDistributionReconciler, instance *llamav1alpha1.LlamaStackDistribution, container *corev1.Container) {
	if instance.Spec.Server.TLSConfig != nil && instance.Spec.Server.TLSConfig.CABundle != nil {
		container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
			Name:      CABundleVolumeName,
			MountPath: CABundleMountPath,
			SubPath:   DefaultCABundleKey,
			ReadOnly:  true,
		})
	} else if r != nil {
		// Check for auto-detected ODH trusted CA bundle
		if _, keys, err := r.detectODHTrustedCABundle(ctx, instance); err == nil && len(keys) > 0 {
			// Mount the auto-detected consolidated CA bundle
			container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
				Name:      CABundleVolumeName,
				MountPath: CABundleMountPath,
				SubPath:   DefaultCABundleKey,
				ReadOnly:  true,
			})
		}
	}
}

// createCABundleVolume creates the appropriate volume configuration for CA bundles.
// For single key: uses direct ConfigMap volume.
// For multiple keys: uses emptyDir volume with InitContainer to concatenate keys.
func createCABundleVolume(caBundleConfig *llamav1alpha1.CABundleConfig) corev1.Volume {
	// For multiple keys, we'll use an emptyDir that gets populated by an InitContainer
	if len(caBundleConfig.ConfigMapKeys) > 0 {
		return corev1.Volume{
			Name: CABundleVolumeName,
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		}
	}

	// For single key (legacy behavior), use direct ConfigMap volume
	return corev1.Volume{
		Name: CABundleVolumeName,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: caBundleConfig.ConfigMapName,
				},
			},
		},
	}
}

// createCABundleInitContainer creates an InitContainer that concatenates multiple CA bundle keys
// from a ConfigMap into a single file in the shared ca-bundle volume.
func createCABundleInitContainer(caBundleConfig *llamav1alpha1.CABundleConfig) (corev1.Container, error) {
	// Validate ConfigMap keys for security
	if err := validateConfigMapKeys(caBundleConfig.ConfigMapKeys); err != nil {
		return corev1.Container{}, fmt.Errorf("failed to validate ConfigMap keys: %w", err)
	}

	// Build the file list as a shell array embedded in the script
	// This ensures the arguments are properly passed to the script
	var fileListBuilder strings.Builder
	for i, key := range caBundleConfig.ConfigMapKeys {
		if i > 0 {
			fileListBuilder.WriteString(" ")
		}
		// Quote each key to handle any special characters safely
		fileListBuilder.WriteString(fmt.Sprintf("%q", key))
	}
	fileList := fileListBuilder.String()

	// Use a secure script approach that embeds the file list directly
	// This eliminates the issue with arguments not being passed to sh -c
	script := fmt.Sprintf(`#!/bin/sh
set -e
output_file="%s"
source_dir="%s"

# Clear the output file
> "$output_file"

# Process each validated key file (keys are pre-validated)
for key in %s; do
    file_path="$source_dir/$key"
    if [ -f "$file_path" ]; then
        cat "$file_path" >> "$output_file"
        echo >> "$output_file"  # Add newline between certificates
    else
        echo "Warning: Certificate file $file_path not found" >&2
    fi
done`, CABundleTempPath, CABundleSourceDir, fileList)

	return corev1.Container{
		Name:    CABundleInitName,
		Image:   "registry.access.redhat.com/ubi9/ubi-minimal:latest",
		Command: []string{"/bin/sh", "-c", script},
		// No Args needed since we embed the file list in the script
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      CABundleSourceVolName,
				MountPath: CABundleSourceDir,
				ReadOnly:  true,
			},
			{
				Name:      CABundleVolumeName,
				MountPath: CABundleTempDir,
			},
		},
		SecurityContext: &corev1.SecurityContext{
			AllowPrivilegeEscalation: &[]bool{false}[0],
			RunAsNonRoot:             &[]bool{false}[0],
			Capabilities: &corev1.Capabilities{
				Drop: []corev1.Capability{"ALL"},
			},
		},
	}, nil
}

// configurePodStorage configures the pod storage and returns the complete pod spec.
func configurePodStorage(ctx context.Context, r *LlamaStackDistributionReconciler, instance *llamav1alpha1.LlamaStackDistribution, container corev1.Container) corev1.PodSpec {
	podSpec := corev1.PodSpec{
		Containers: []corev1.Container{container},
	}

	// Configure storage volumes and init containers
	configureStorage(instance, &podSpec)

	// Configure TLS CA bundle (with auto-detection support)
	configureTLSCABundle(ctx, r, instance, &podSpec)

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
// For multiple keys: adds a ca-bundle-init init container that concatenates all keys into a single file
// in a shared emptyDir volume, which the main container then mounts via SubPath.
// For single key: uses a direct ConfigMap volume mount.
// If no explicit CA bundle is configured, it checks for the well-known ODH trusted CA bundle ConfigMap.
func configureTLSCABundle(ctx context.Context, r *LlamaStackDistributionReconciler, instance *llamav1alpha1.LlamaStackDistribution, podSpec *corev1.PodSpec) {
	tlsConfig := instance.Spec.Server.TLSConfig

	// Handle explicit CA bundle configuration first
	if tlsConfig != nil && tlsConfig.CABundle != nil {
		addExplicitCABundle(ctx, tlsConfig.CABundle, podSpec)
		return
	}

	// If no explicit CA bundle is configured, check for ODH trusted CA bundle auto-detection
	if r != nil {
		addAutoDetectedCABundle(ctx, r, instance, podSpec)
	}
}

// addExplicitCABundle handles explicitly configured CA bundles.
func addExplicitCABundle(ctx context.Context, caBundleConfig *llamav1alpha1.CABundleConfig, podSpec *corev1.PodSpec) {
	// Add CA bundle InitContainer if multiple keys are specified
	if len(caBundleConfig.ConfigMapKeys) > 0 {
		caBundleInitContainer, err := createCABundleInitContainer(caBundleConfig)
		if err != nil {
			log.FromContext(ctx).Error(err, "Failed to create CA bundle init container")
			return
		}
		podSpec.InitContainers = append(podSpec.InitContainers, caBundleInitContainer)
	}

	// Add CA bundle ConfigMap volume
	volume := createCABundleVolume(caBundleConfig)
	podSpec.Volumes = append(podSpec.Volumes, volume)

	// Add source ConfigMap volume for multiple keys scenario
	if len(caBundleConfig.ConfigMapKeys) > 0 {
		sourceVolume := corev1.Volume{
			Name: CABundleSourceVolName,
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: caBundleConfig.ConfigMapName,
					},
				},
			},
		}
		podSpec.Volumes = append(podSpec.Volumes, sourceVolume)
	}
}

// addAutoDetectedCABundle handles auto-detection of ODH trusted CA bundle ConfigMap.
func addAutoDetectedCABundle(ctx context.Context, r *LlamaStackDistributionReconciler, instance *llamav1alpha1.LlamaStackDistribution, podSpec *corev1.PodSpec) {
	if r == nil {
		return
	}

	configMap, keys, err := r.detectODHTrustedCABundle(ctx, instance)
	if err != nil {
		// Log error but don't fail the reconciliation
		log.FromContext(ctx).Error(err, "Failed to detect ODH trusted CA bundle ConfigMap")
		return
	}

	if configMap == nil || len(keys) == 0 {
		// No ODH trusted CA bundle found or no keys available
		return
	}

	// Create a virtual CA bundle config for auto-detected ConfigMap
	autoCaBundleConfig := &llamav1alpha1.CABundleConfig{
		ConfigMapName: configMap.Name,
		ConfigMapKeys: keys, // Use all available keys
	}

	// Use the same logic as explicit configuration
	caBundleInitContainer, err := createCABundleInitContainer(autoCaBundleConfig)
	if err != nil {
		// Log error and skip auto-detected CA bundle configuration
		log.FromContext(ctx).Error(err, "Failed to create CA bundle init container for auto-detected ConfigMap")
		return
	}
	podSpec.InitContainers = append(podSpec.InitContainers, caBundleInitContainer)

	// Add CA bundle emptyDir volume for auto-detected ConfigMap
	volume := createCABundleVolume(autoCaBundleConfig)
	podSpec.Volumes = append(podSpec.Volumes, volume)

	// Add source ConfigMap volume for auto-detected ConfigMap
	sourceVolume := corev1.Volume{
		Name: CABundleSourceVolName,
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: configMap.Name,
				},
			},
		},
	}
	podSpec.Volumes = append(podSpec.Volumes, sourceVolume)

	log.FromContext(ctx).Info("Auto-configured ODH trusted CA bundle",
		"configMapName", configMap.Name,
		"keys", keys)
}

// configureUserConfig handles user configuration setup.
func configureUserConfig(instance *llamav1alpha1.LlamaStackDistribution, podSpec *corev1.PodSpec) {
	userConfig := instance.Spec.Server.UserConfig
	if userConfig == nil || userConfig.ConfigMapName == "" {
		return
	}

	// Add ConfigMap volume if user config is specified
	podSpec.Volumes = append(podSpec.Volumes, corev1.Volume{
		Name: "user-config",
		VolumeSource: corev1.VolumeSource{
			ConfigMap: &corev1.ConfigMapVolumeSource{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: userConfig.ConfigMapName,
				},
			},
		},
	})
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
