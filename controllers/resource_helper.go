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
	"strings"

	llamav1alpha1 "github.com/llamastack/llama-stack-k8s-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

// buildContainerSpec creates the container specification.
func buildContainerSpec(instance *llamav1alpha1.LlamaStackDistribution, image string) corev1.Container {
	container := corev1.Container{
		Name:            getContainerName(instance),
		Image:           image,
		Resources:       instance.Spec.Server.ContainerSpec.Resources,
		ImagePullPolicy: corev1.PullAlways,
		Ports:           []corev1.ContainerPort{{ContainerPort: getContainerPort(instance)}},
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
	if instance.Spec.Server.TLSConfig != nil && instance.Spec.Server.TLSConfig.CABundle != nil {
		caBundleKey := getCABundleKey(instance.Spec.Server.TLSConfig.CABundle)

		// Set SSL_CERT_FILE to point to the specific CA bundle file
		container.Env = append(container.Env, corev1.EnvVar{
			Name:  "SSL_CERT_FILE",
			Value: "/etc/ssl/certs/" + caBundleKey,
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
func addCABundleVolumeMount(instance *llamav1alpha1.LlamaStackDistribution, container *corev1.Container) {
	if instance.Spec.Server.TLSConfig != nil && instance.Spec.Server.TLSConfig.CABundle != nil {
		caBundleKey := getCABundleKey(instance.Spec.Server.TLSConfig.CABundle)

		container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
			Name:      "ca-bundle",
			MountPath: "/etc/ssl/certs/" + caBundleKey,
			SubPath:   caBundleKey,
			ReadOnly:  true,
		})
	}
}

// getCABundleKey returns the effective CA bundle key name for volume mounting.
// When multiple keys are specified, we use "ca-bundle.crt" as the consolidated filename.
func getCABundleKey(caBundleConfig *llamav1alpha1.CABundleConfig) string {
	// If multiple keys are specified, use a standard filename for the consolidated bundle
	if len(caBundleConfig.ConfigMapKeys) > 0 {
		return "ca-bundle.crt"
	}

	// Use legacy single key behavior
	if caBundleConfig.ConfigMapKey != "" {
		return caBundleConfig.ConfigMapKey
	}

	// Default fallback
	return defaultCABundleKey
}

// createCABundleVolume creates the appropriate volume configuration for CA bundles.
// For single key: uses direct ConfigMap volume.
// For multiple keys: uses emptyDir volume with InitContainer to concatenate keys.
func createCABundleVolume(caBundleConfig *llamav1alpha1.CABundleConfig) corev1.Volume {
	// For multiple keys, we'll use an emptyDir that gets populated by an InitContainer
	if len(caBundleConfig.ConfigMapKeys) > 0 {
		return corev1.Volume{
			Name: "ca-bundle",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		}
	}

	// For single key (legacy behavior), use direct ConfigMap volume
	return corev1.Volume{
		Name: "ca-bundle",
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
func createCABundleInitContainer(caBundleConfig *llamav1alpha1.CABundleConfig) corev1.Container {
	// Build the shell command to concatenate all specified keys
	keyPaths := make([]string, 0, len(caBundleConfig.ConfigMapKeys))

	for _, key := range caBundleConfig.ConfigMapKeys {
		keyPaths = append(keyPaths, "/tmp/ca-source/"+key)
	}

	// Command to concatenate all keys into the target file
	command := fmt.Sprintf("cat %s > /etc/ssl/certs/ca-bundle.crt", strings.Join(keyPaths, " "))

	return corev1.Container{
		Name:  "ca-bundle-init",
		Image: "registry.access.redhat.com/ubi9/ubi-minimal:latest",
		Command: []string{
			"/bin/sh",
			"-c",
			command,
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      "ca-bundle-source",
				MountPath: "/tmp/ca-source",
				ReadOnly:  true,
			},
			{
				Name:      "ca-bundle",
				MountPath: "/etc/ssl/certs",
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
		fmt.Sprintf("mkdir -p %s", mountPath),
		fmt.Sprintf("(chown 1001:0 %s 2>/dev/null || echo 'Warning: Could not change ownership')", mountPath),
		fmt.Sprintf("ls -la %s", mountPath),
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
func configureTLSCABundle(instance *llamav1alpha1.LlamaStackDistribution, podSpec *corev1.PodSpec) {
	tlsConfig := instance.Spec.Server.TLSConfig
	if tlsConfig == nil || tlsConfig.CABundle == nil {
		return
	}

	// Add CA bundle InitContainer if multiple keys are specified
	if len(tlsConfig.CABundle.ConfigMapKeys) > 0 {
		caBundleInitContainer := createCABundleInitContainer(tlsConfig.CABundle)
		podSpec.InitContainers = append(podSpec.InitContainers, caBundleInitContainer)
	}

	// Add CA bundle ConfigMap volume
	volume := createCABundleVolume(tlsConfig.CABundle)
	podSpec.Volumes = append(podSpec.Volumes, volume)

	// Add source ConfigMap volume for multiple keys scenario
	if len(tlsConfig.CABundle.ConfigMapKeys) > 0 {
		sourceVolume := corev1.Volume{
			Name: "ca-bundle-source",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: tlsConfig.CABundle.ConfigMapName,
					},
				},
			},
		}
		podSpec.Volumes = append(podSpec.Volumes, sourceVolume)
	}
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
