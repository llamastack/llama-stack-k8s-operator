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

	llamav1alpha1 "github.com/llamastack/llama-stack-k8s-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/utils/ptr"
)

// buildContainerSpec creates the container specification.
func buildContainerSpec(instance *llamav1alpha1.LlamaStackDistribution, image string) corev1.Container {
	container := corev1.Container{
		Name:            llamav1alpha1.DefaultContainerName,
		Image:           image,
		Resources:       instance.Spec.Server.ContainerSpec.Resources,
		ImagePullPolicy: corev1.PullAlways,
	}

	if instance.Spec.Server.ContainerSpec.Name != "" {
		container.Name = instance.Spec.Server.ContainerSpec.Name
	}

	port := llamav1alpha1.DefaultServerPort
	if instance.Spec.Server.ContainerSpec.Port != 0 {
		port = instance.Spec.Server.ContainerSpec.Port
	}
	container.Ports = []corev1.ContainerPort{{ContainerPort: port}}

	mountPath := llamav1alpha1.DefaultMountPath
	if instance.Spec.Server.Storage != nil {
		// Determine mount path
		if instance.Spec.Server.Storage.MountPath != "" {
			mountPath = instance.Spec.Server.Storage.MountPath
		}
	}

	// Add HF_HOME variable to our mount path so that downloaded models and datasets are stored
	// on the same volume as the storage. This is not critical but useful if the server is
	// restarted so the models and datasets are not lost and need to be downloaded again.
	// For more information, see https://huggingface.co/docs/datasets/en/cache
	container.Env = append(container.Env, corev1.EnvVar{
		Name:  "HF_HOME",
		Value: mountPath,
	})

	// Finally, add the user provided env vars
	container.Env = append(container.Env, instance.Spec.Server.ContainerSpec.Env...)

	// Add volume mount for storage
	container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
		Name:      "lls-storage",
		MountPath: mountPath,
	})

	// Add ConfigMap volume mount if user config is specified
	if instance.Spec.Server.UserConfig != nil && instance.Spec.Server.UserConfig.ConfigMapName != "" {
		container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
			Name:      "user-config",
			MountPath: "/etc/llama-stack/",
			ReadOnly:  true,
		})

		// Override the container entrypoint to use the custom config file instead of the default template
		container.Command = []string{"python", "-m", "llama_stack.distribution.server.server"}
		container.Args = []string{"--config", "/etc/llama-stack/run.yaml"}
	}

	if len(instance.Spec.Server.ContainerSpec.Command) > 0 {
		container.Command = instance.Spec.Server.ContainerSpec.Command
	}

	if len(instance.Spec.Server.ContainerSpec.Args) > 0 {
		container.Args = instance.Spec.Server.ContainerSpec.Args
	}
	return container
}

// configurePodStorage configures the pod storage and returns the complete pod spec.
func configurePodStorage(instance *llamav1alpha1.LlamaStackDistribution, container corev1.Container) corev1.PodSpec {
	podSpec := corev1.PodSpec{
		Containers: []corev1.Container{container},
	}

	// Add storage volume
	if instance.Spec.Server.Storage != nil {
		// Use PVC for persistent storage
		podSpec.Volumes = append(podSpec.Volumes, corev1.Volume{
			Name: "lls-storage",
			VolumeSource: corev1.VolumeSource{
				PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
					ClaimName: instance.Name + "-pvc",
				},
			},
		})

		// Add init container to fix permissions on the PVC mount
		mountPath := llamav1alpha1.DefaultMountPath
		if instance.Spec.Server.Storage.MountPath != "" {
			mountPath = instance.Spec.Server.Storage.MountPath
		}

		initContainer := corev1.Container{
			Name:  "update-pvc-permissions",
			Image: "registry.access.redhat.com/ubi9/ubi-minimal:latest",
			Command: []string{
				"/bin/sh",
				"-c",
				fmt.Sprintf("chown --verbose --recursive 1001:0 %s", mountPath),
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
	} else {
		// Use emptyDir for non-persistent storage
		podSpec.Volumes = append(podSpec.Volumes, corev1.Volume{
			Name: "lls-storage",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		})
	}

	// Add ConfigMap volume if user config is specified
	if instance.Spec.Server.UserConfig != nil && instance.Spec.Server.UserConfig.ConfigMapName != "" {
		podSpec.Volumes = append(podSpec.Volumes, corev1.Volume{
			Name: "user-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: instance.Spec.Server.UserConfig.ConfigMapName,
					},
				},
			},
		})
	}

	// Apply pod overrides including ServiceAccount, volumes, and volume mounts
	configurePodOverrides(instance, &podSpec)

	return podSpec
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
