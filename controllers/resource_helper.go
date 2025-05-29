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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/utils/ptr"
)

// Define a map that translates user-friendly names to actual image references.
var imageMap = llamav1alpha1.ImageMap

// buildContainerSpec creates the container specification.
func buildContainerSpec(instance *llamav1alpha1.LlamaStackDistribution, image string) corev1.Container {
	container := corev1.Container{
		Name:            llamav1alpha1.DefaultContainerName,
		Image:           image,
		Resources:       instance.Spec.Server.ContainerSpec.Resources,
		Env:             instance.Spec.Server.ContainerSpec.Env,
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

	// Determine mount path
	mountPath := llamav1alpha1.DefaultMountPath
	if instance.Spec.Server.Storage != nil && instance.Spec.Server.Storage.MountPath != "" {
		mountPath = instance.Spec.Server.Storage.MountPath
	}

	// Add volume mount for storage
	container.VolumeMounts = append(container.VolumeMounts, corev1.VolumeMount{
		Name:      "lls-storage",
		MountPath: mountPath,
	})

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
	} else {
		// Use emptyDir for non-persistent storage
		podSpec.Volumes = append(podSpec.Volumes, corev1.Volume{
			Name: "lls-storage",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{},
			},
		})
	}

	// Add any pod overrides
	if instance.Spec.Server.PodOverrides != nil {
		podSpec.Volumes = append(podSpec.Volumes, instance.Spec.Server.PodOverrides.Volumes...)
		container.VolumeMounts = append(container.VolumeMounts, instance.Spec.Server.PodOverrides.VolumeMounts...)
		podSpec.Containers[0] = container // Update with volume mounts
	}

	return podSpec
}

// validateDistribution validates the distribution configuration.
func (r *LlamaStackDistributionReconciler) validateDistribution(instance *llamav1alpha1.LlamaStackDistribution) error {
	if instance.Spec.Server.Distribution.Name != "" && instance.Spec.Server.Distribution.Image != "" {
		return errors.New("only one of distribution.name or distribution.image can be set")
	}

	if instance.Spec.Server.Distribution.Name == "" && instance.Spec.Server.Distribution.Image == "" {
		return errors.New("failed to validate distribution: either distribution.name or distribution.image must be set")
	}

	return nil
}

// resolveImage resolves the container image from either name or direct reference.
func (r *LlamaStackDistributionReconciler) resolveImage(instance *llamav1alpha1.LlamaStackDistribution) (string, error) {
	if instance.Spec.Server.Distribution.Name != "" {
		resolvedImage := imageMap[instance.Spec.Server.Distribution.Name]
		if resolvedImage == "" {
			return "", fmt.Errorf("failed to validate distribution name: %s", instance.Spec.Server.Distribution.Name)
		}
		return resolvedImage, nil
	}

	return instance.Spec.Server.Distribution.Image, nil
}

func BuildDeployment(instance *llamav1alpha1.LlamaStackDistribution, podSpec corev1.PodSpec) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name,
			Namespace: instance.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &instance.Spec.Replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{llamav1alpha1.DefaultLabelKey: llamav1alpha1.DefaultLabelValue},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{llamav1alpha1.DefaultLabelKey: llamav1alpha1.DefaultLabelValue},
				},
				Spec: podSpec,
			},
		},
	}
}

func BuildPVC(instance *llamav1alpha1.LlamaStackDistribution) *corev1.PersistentVolumeClaim {
	// Use default size if none specified
	size := instance.Spec.Server.Storage.Size
	if size == nil {
		size = &llamav1alpha1.DefaultStorageSize
	}

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name + "-pvc",
			Namespace: instance.Namespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: *size,
				},
			},
		},
	}

	return pvc
}

func BuildService(instance *llamav1alpha1.LlamaStackDistribution) *corev1.Service {
	// Use the container's port (defaulted to 8321 if unset)
	port := instance.Spec.Server.ContainerSpec.Port
	if port == 0 {
		port = llamav1alpha1.DefaultServerPort
	}

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name + "-service",
			Namespace: instance.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{llamav1alpha1.DefaultLabelKey: llamav1alpha1.DefaultLabelValue},
			Ports: []corev1.ServicePort{{
				Name: llamav1alpha1.DefaultServicePortName,
				Port: port,
				TargetPort: intstr.IntOrString{
					IntVal: port,
				},
			}},
			Type: corev1.ServiceTypeClusterIP,
		},
	}
}

func BaseNetworkPolicy(instance *llamav1alpha1.LlamaStackDistribution) *networkingv1.NetworkPolicy {
	return &networkingv1.NetworkPolicy{
		ObjectMeta: metav1.ObjectMeta{
			Name:      instance.Name + "-network-policy",
			Namespace: instance.Namespace,
		},
	}
}

func BuildNetworkPolicy(instance *llamav1alpha1.LlamaStackDistribution, operatorNamespace string) *networkingv1.NetworkPolicy {
	// Use the container's port (defaulted to 8321 if unset)
	port := instance.Spec.Server.ContainerSpec.Port
	if port == 0 {
		port = llamav1alpha1.DefaultServerPort
	}
	np := BaseNetworkPolicy(instance)
	np.Spec = networkingv1.NetworkPolicySpec{
		PodSelector: metav1.LabelSelector{
			MatchLabels: map[string]string{
				llamav1alpha1.DefaultLabelKey: llamav1alpha1.DefaultLabelValue,
			},
		},
		PolicyTypes: []networkingv1.PolicyType{
			networkingv1.PolicyTypeIngress,
		},
		Ingress: []networkingv1.NetworkPolicyIngressRule{
			{
				From: []networkingv1.NetworkPolicyPeer{
					{
						PodSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"app.kubernetes.io/part-of": llamav1alpha1.DefaultContainerName,
							},
						},
						NamespaceSelector: &metav1.LabelSelector{}, // Empty namespaceSelector to match all namespaces
					},
				},
				Ports: []networkingv1.NetworkPolicyPort{
					{
						Protocol: (*corev1.Protocol)(ptr.To("TCP")),
						Port: &intstr.IntOrString{
							IntVal: port,
						},
					},
				},
			},
			{
				From: []networkingv1.NetworkPolicyPeer{
					{
						PodSelector: &metav1.LabelSelector{}, // Empty podSelector to match all pods
						NamespaceSelector: &metav1.LabelSelector{
							MatchLabels: map[string]string{
								"kubernetes.io/metadata.name": operatorNamespace,
							},
						},
					},
				},
				Ports: []networkingv1.NetworkPolicyPort{
					{
						Protocol: (*corev1.Protocol)(ptr.To("TCP")),
						Port: &intstr.IntOrString{
							IntVal: port,
						},
					},
				},
			},
			{
				From: []networkingv1.NetworkPolicyPeer{
					{
						PodSelector: &metav1.LabelSelector{}, // Empty podSelector to match all pods
					},
				},
				Ports: []networkingv1.NetworkPolicyPort{
					{
						Protocol: (*corev1.Protocol)(ptr.To("TCP")),
						Port: &intstr.IntOrString{
							IntVal: port,
						},
					},
				},
			},
		},
	}

	return np
}
