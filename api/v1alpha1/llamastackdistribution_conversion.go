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

package v1alpha1

import (
	"encoding/json"
	"fmt"
	"strconv"

	v1alpha2 "github.com/llamastack/llama-stack-k8s-operator/api/v1alpha2"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

const (
	annPrefix                 = "llamastack.io/v1alpha1-"
	annContainerName          = annPrefix + "container-name"
	annTerminationGracePeriod = annPrefix + "termination-grace-period"
	annUserConfigNamespace    = annPrefix + "user-config-namespace"
	annCABundleNamespace      = annPrefix + "ca-bundle-namespace"
	annCABundleKeys           = annPrefix + "ca-bundle-keys"
)

var _ conversion.Convertible = &LlamaStackDistribution{}

// ConvertTo converts this v1alpha1 LlamaStackDistribution to the Hub version (v1alpha2).
func (src *LlamaStackDistribution) ConvertTo(dstRaw conversion.Hub) error {
	dst, ok := dstRaw.(*v1alpha2.LlamaStackDistribution)
	if !ok {
		return fmt.Errorf("failed to convert: expected *v1alpha2.LlamaStackDistribution, got %T", dstRaw)
	}

	dst.ObjectMeta = src.ObjectMeta
	if dst.Annotations == nil {
		dst.Annotations = make(map[string]string)
	}

	// Distribution
	dst.Spec.Distribution = v1alpha2.DistributionSpec{
		Name:  src.Spec.Server.Distribution.Name,
		Image: src.Spec.Server.Distribution.Image,
	}

	// Workload
	dst.Spec.Workload = convertToWorkload(src, dst)

	// Networking
	dst.Spec.Networking = convertToNetworking(src, dst)

	// UserConfig → OverrideConfig
	if src.Spec.Server.UserConfig != nil {
		dst.Spec.OverrideConfig = &v1alpha2.OverrideConfigSpec{
			ConfigMapName: src.Spec.Server.UserConfig.ConfigMapName,
		}
		if src.Spec.Server.UserConfig.ConfigMapNamespace != "" {
			dst.Annotations[annUserConfigNamespace] = src.Spec.Server.UserConfig.ConfigMapNamespace
		}
	}

	// Status
	convertToStatus(src, dst)

	return nil
}

// ConvertFrom converts the Hub version (v1alpha2) to this v1alpha1 LlamaStackDistribution.
func (dst *LlamaStackDistribution) ConvertFrom(srcRaw conversion.Hub) error {
	src, ok := srcRaw.(*v1alpha2.LlamaStackDistribution)
	if !ok {
		return fmt.Errorf("failed to convert: expected *v1alpha2.LlamaStackDistribution, got %T", srcRaw)
	}

	dst.ObjectMeta = src.ObjectMeta

	// Distribution
	dst.Spec.Server.Distribution = DistributionType{
		Name:  src.Spec.Distribution.Name,
		Image: src.Spec.Distribution.Image,
	}

	// Workload → ServerSpec + Replicas
	convertFromWorkload(src, dst)

	// Networking → Network + TLS
	convertFromNetworking(src, dst)

	// OverrideConfig → UserConfig
	if src.Spec.OverrideConfig != nil {
		ns := ""
		if src.Annotations != nil {
			ns = src.Annotations[annUserConfigNamespace]
		}
		dst.Spec.Server.UserConfig = &UserConfigSpec{
			ConfigMapName:      src.Spec.OverrideConfig.ConfigMapName,
			ConfigMapNamespace: ns,
		}
	}

	// Status
	convertFromStatus(src, dst)

	return nil
}

func convertToWorkload(src *LlamaStackDistribution, dst *v1alpha2.LlamaStackDistribution) *v1alpha2.WorkloadSpec {
	w := &v1alpha2.WorkloadSpec{}
	hasContent := false

	// Replicas
	if src.Spec.Replicas > 0 {
		r := src.Spec.Replicas
		w.Replicas = &r
		hasContent = true
	}

	// Workers
	if src.Spec.Server.Workers != nil {
		w.Workers = src.Spec.Server.Workers
		hasContent = true
	}

	// Resources
	cs := src.Spec.Server.ContainerSpec
	if !isEmptyResources(cs.Resources) {
		w.Resources = &cs.Resources
		hasContent = true
	}

	// Container name (if non-default, save in annotation)
	if cs.Name != "" && cs.Name != DefaultContainerName {
		dst.Annotations[annContainerName] = cs.Name
	}

	// Overrides from ContainerSpec and PodOverrides
	overrides := convertToOverrides(src, dst)
	if overrides != nil {
		w.Overrides = overrides
		hasContent = true
	}

	if convertToWorkloadScaling(src, w) {
		hasContent = true
	}

	if !hasContent {
		return nil
	}
	return w
}

func convertToWorkloadScaling(src *LlamaStackDistribution, w *v1alpha2.WorkloadSpec) bool {
	hasContent := false
	if src.Spec.Server.Storage != nil {
		w.Storage = &v1alpha2.PVCStorageSpec{
			Size:      src.Spec.Server.Storage.Size,
			MountPath: src.Spec.Server.Storage.MountPath,
		}
		hasContent = true
	}
	if src.Spec.Server.PodDisruptionBudget != nil {
		w.PodDisruptionBudget = &v1alpha2.PodDisruptionBudgetSpec{
			MinAvailable:   src.Spec.Server.PodDisruptionBudget.MinAvailable,
			MaxUnavailable: src.Spec.Server.PodDisruptionBudget.MaxUnavailable,
		}
		hasContent = true
	}
	if src.Spec.Server.Autoscaling != nil {
		w.Autoscaling = &v1alpha2.AutoscalingSpec{
			MinReplicas:                       src.Spec.Server.Autoscaling.MinReplicas,
			MaxReplicas:                       src.Spec.Server.Autoscaling.MaxReplicas,
			TargetCPUUtilizationPercentage:    src.Spec.Server.Autoscaling.TargetCPUUtilizationPercentage,
			TargetMemoryUtilizationPercentage: src.Spec.Server.Autoscaling.TargetMemoryUtilizationPercentage,
		}
		hasContent = true
	}
	if len(src.Spec.Server.TopologySpreadConstraints) > 0 {
		w.TopologySpreadConstraints = src.Spec.Server.TopologySpreadConstraints
		hasContent = true
	}
	return hasContent
}

func convertToOverrides(src *LlamaStackDistribution, dst *v1alpha2.LlamaStackDistribution) *v1alpha2.WorkloadOverrides {
	o := &v1alpha2.WorkloadOverrides{}
	hasContent := false

	cs := src.Spec.Server.ContainerSpec
	if len(cs.Env) > 0 {
		o.Env = cs.Env
		hasContent = true
	}
	if len(cs.Command) > 0 {
		o.Command = cs.Command
		hasContent = true
	}
	if len(cs.Args) > 0 {
		o.Args = cs.Args
		hasContent = true
	}

	if src.Spec.Server.PodOverrides != nil {
		po := src.Spec.Server.PodOverrides
		if po.ServiceAccountName != "" {
			o.ServiceAccountName = po.ServiceAccountName
			hasContent = true
		}
		if len(po.Volumes) > 0 {
			o.Volumes = po.Volumes
			hasContent = true
		}
		if len(po.VolumeMounts) > 0 {
			o.VolumeMounts = po.VolumeMounts
			hasContent = true
		}
		if po.TerminationGracePeriodSeconds != nil {
			dst.Annotations[annTerminationGracePeriod] = strconv.FormatInt(*po.TerminationGracePeriodSeconds, 10)
		}
	}

	if !hasContent {
		return nil
	}
	return o
}

func convertToNetworking(src *LlamaStackDistribution, dst *v1alpha2.LlamaStackDistribution) *v1alpha2.NetworkingSpec {
	n := &v1alpha2.NetworkingSpec{}
	hasContent := false

	if src.Spec.Server.ContainerSpec.Port > 0 {
		n.Port = src.Spec.Server.ContainerSpec.Port
		hasContent = true
	}

	if convertToTLS(src, dst, n) {
		hasContent = true
	}

	if convertToNetworkAccess(src, n) {
		hasContent = true
	}

	if !hasContent {
		return nil
	}
	return n
}

func convertToTLS(src *LlamaStackDistribution, dst *v1alpha2.LlamaStackDistribution, n *v1alpha2.NetworkingSpec) bool {
	if src.Spec.Server.TLSConfig == nil || src.Spec.Server.TLSConfig.CABundle == nil {
		return false
	}
	cab := src.Spec.Server.TLSConfig.CABundle
	n.TLS = &v1alpha2.TLSSpec{
		CABundle: &v1alpha2.CABundleConfig{
			ConfigMapName: cab.ConfigMapName,
		},
	}
	if cab.ConfigMapNamespace != "" {
		dst.Annotations[annCABundleNamespace] = cab.ConfigMapNamespace
	}
	if len(cab.ConfigMapKeys) > 0 {
		if keysJSON, err := json.Marshal(cab.ConfigMapKeys); err == nil {
			dst.Annotations[annCABundleKeys] = string(keysJSON)
		}
	}
	return true
}

func convertToNetworkAccess(src *LlamaStackDistribution, n *v1alpha2.NetworkingSpec) bool {
	if src.Spec.Network == nil {
		return false
	}
	hasContent := false
	if src.Spec.Network.ExposeRoute {
		if raw, err := json.Marshal(true); err == nil {
			n.Expose = &apiextensionsv1.JSON{Raw: raw}
			hasContent = true
		}
	}
	if src.Spec.Network.AllowedFrom != nil {
		n.AllowedFrom = &v1alpha2.AllowedFromSpec{
			Namespaces: src.Spec.Network.AllowedFrom.Namespaces,
			Labels:     src.Spec.Network.AllowedFrom.Labels,
		}
		hasContent = true
	}
	return hasContent
}

func convertToStatus(src *LlamaStackDistribution, dst *v1alpha2.LlamaStackDistribution) {
	dst.Status.Phase = v1alpha2.DistributionPhase(src.Status.Phase)
	dst.Status.Version = v1alpha2.VersionInfo{
		OperatorVersion:         src.Status.Version.OperatorVersion,
		LlamaStackServerVersion: src.Status.Version.LlamaStackServerVersion,
		LastUpdated:             src.Status.Version.LastUpdated,
	}
	dst.Status.AvailableReplicas = src.Status.AvailableReplicas
	dst.Status.ServiceURL = src.Status.ServiceURL
	dst.Status.RouteURL = src.Status.RouteURL

	// Convert conditions
	if len(src.Status.Conditions) > 0 {
		dst.Status.Conditions = make([]metav1.Condition, len(src.Status.Conditions))
		copy(dst.Status.Conditions, src.Status.Conditions)
	}

	// Convert distributionConfig
	if len(src.Status.DistributionConfig.Providers) > 0 || src.Status.DistributionConfig.ActiveDistribution != "" {
		dst.Status.DistributionConfig = v1alpha2.DistributionConfig{
			ActiveDistribution: src.Status.DistributionConfig.ActiveDistribution,
		}
		if len(src.Status.DistributionConfig.AvailableDistributions) > 0 {
			dst.Status.DistributionConfig.AvailableDistributions = src.Status.DistributionConfig.AvailableDistributions
		}
		for _, p := range src.Status.DistributionConfig.Providers {
			dst.Status.DistributionConfig.Providers = append(
				dst.Status.DistributionConfig.Providers,
				v1alpha2.ProviderInfo{
					API:          p.API,
					ProviderID:   p.ProviderID,
					ProviderType: p.ProviderType,
					Config:       p.Config,
					Health: v1alpha2.ProviderHealthStatus{
						Status:  p.Health.Status,
						Message: p.Health.Message,
					},
				},
			)
		}
	}
}

func convertFromWorkload(src *v1alpha2.LlamaStackDistribution, dst *LlamaStackDistribution) {
	if src.Spec.Workload == nil {
		dst.Spec.Replicas = 1
		return
	}
	w := src.Spec.Workload

	// Replicas
	if w.Replicas != nil {
		dst.Spec.Replicas = *w.Replicas
	} else {
		dst.Spec.Replicas = 1
	}

	// Workers
	dst.Spec.Server.Workers = w.Workers

	// Resources
	if w.Resources != nil {
		dst.Spec.Server.ContainerSpec.Resources = *w.Resources
	}

	// Container name from annotation
	if src.Annotations != nil {
		if name, ok := src.Annotations[annContainerName]; ok {
			dst.Spec.Server.ContainerSpec.Name = name
		}
	}

	convertFromOverrides(src, dst, w)
	convertFromWorkloadScaling(dst, w)
}

func convertFromOverrides(src *v1alpha2.LlamaStackDistribution, dst *LlamaStackDistribution, w *v1alpha2.WorkloadSpec) {
	if w.Overrides == nil {
		return
	}
	o := w.Overrides
	dst.Spec.Server.ContainerSpec.Env = o.Env
	dst.Spec.Server.ContainerSpec.Command = o.Command
	dst.Spec.Server.ContainerSpec.Args = o.Args

	if o.ServiceAccountName != "" || len(o.Volumes) > 0 || len(o.VolumeMounts) > 0 {
		dst.Spec.Server.PodOverrides = &PodOverrides{
			ServiceAccountName: o.ServiceAccountName,
			Volumes:            o.Volumes,
			VolumeMounts:       o.VolumeMounts,
		}
	}

	if src.Annotations != nil {
		if tgp, ok := src.Annotations[annTerminationGracePeriod]; ok {
			if v, err := strconv.ParseInt(tgp, 10, 64); err == nil {
				if dst.Spec.Server.PodOverrides == nil {
					dst.Spec.Server.PodOverrides = &PodOverrides{}
				}
				dst.Spec.Server.PodOverrides.TerminationGracePeriodSeconds = &v
			}
		}
	}
}

func convertFromWorkloadScaling(dst *LlamaStackDistribution, w *v1alpha2.WorkloadSpec) {
	if w.Storage != nil {
		dst.Spec.Server.Storage = &StorageSpec{
			Size:      w.Storage.Size,
			MountPath: w.Storage.MountPath,
		}
	}
	if w.PodDisruptionBudget != nil {
		dst.Spec.Server.PodDisruptionBudget = &PodDisruptionBudgetSpec{
			MinAvailable:   w.PodDisruptionBudget.MinAvailable,
			MaxUnavailable: w.PodDisruptionBudget.MaxUnavailable,
		}
	}
	if w.Autoscaling != nil {
		dst.Spec.Server.Autoscaling = &AutoscalingSpec{
			MinReplicas:                       w.Autoscaling.MinReplicas,
			MaxReplicas:                       w.Autoscaling.MaxReplicas,
			TargetCPUUtilizationPercentage:    w.Autoscaling.TargetCPUUtilizationPercentage,
			TargetMemoryUtilizationPercentage: w.Autoscaling.TargetMemoryUtilizationPercentage,
		}
	}
	dst.Spec.Server.TopologySpreadConstraints = w.TopologySpreadConstraints
}

func convertFromTLS(src *v1alpha2.LlamaStackDistribution, dst *LlamaStackDistribution, n *v1alpha2.NetworkingSpec) {
	if n.TLS == nil || n.TLS.CABundle == nil {
		return
	}
	cab := &CABundleConfig{
		ConfigMapName: n.TLS.CABundle.ConfigMapName,
	}
	if src.Annotations != nil {
		cab.ConfigMapNamespace = src.Annotations[annCABundleNamespace]
		if keysJSON, ok := src.Annotations[annCABundleKeys]; ok {
			_ = json.Unmarshal([]byte(keysJSON), &cab.ConfigMapKeys)
		}
	}
	dst.Spec.Server.TLSConfig = &TLSConfig{
		CABundle: cab,
	}
}

func convertFromNetworking(src *v1alpha2.LlamaStackDistribution, dst *LlamaStackDistribution) {
	if src.Spec.Networking == nil {
		return
	}
	n := src.Spec.Networking

	// Port
	if n.Port > 0 {
		dst.Spec.Server.ContainerSpec.Port = n.Port
	}

	convertFromTLS(src, dst, n)

	// Expose → ExposeRoute
	expose, _ := parseExpose(n.Expose)
	if expose {
		if dst.Spec.Network == nil {
			dst.Spec.Network = &NetworkSpec{}
		}
		dst.Spec.Network.ExposeRoute = true
	}

	// AllowedFrom
	if n.AllowedFrom != nil {
		if dst.Spec.Network == nil {
			dst.Spec.Network = &NetworkSpec{}
		}
		dst.Spec.Network.AllowedFrom = &AllowedFromSpec{
			Namespaces: n.AllowedFrom.Namespaces,
			Labels:     n.AllowedFrom.Labels,
		}
	}
}

func parseExpose(raw *apiextensionsv1.JSON) (bool, string) {
	if raw == nil || len(raw.Raw) == 0 {
		return false, ""
	}
	var boolVal bool
	if err := json.Unmarshal(raw.Raw, &boolVal); err == nil {
		return boolVal, ""
	}
	var obj struct {
		Enabled  *bool  `json:"enabled,omitempty"`
		Hostname string `json:"hostname,omitempty"`
	}
	if err := json.Unmarshal(raw.Raw, &obj); err == nil {
		if obj.Enabled != nil {
			return *obj.Enabled, obj.Hostname
		}
		return true, obj.Hostname
	}
	return false, ""
}

func convertFromStatus(src *v1alpha2.LlamaStackDistribution, dst *LlamaStackDistribution) {
	dst.Status.Phase = DistributionPhase(src.Status.Phase)
	dst.Status.Version = VersionInfo{
		OperatorVersion:         src.Status.Version.OperatorVersion,
		LlamaStackServerVersion: src.Status.Version.LlamaStackServerVersion,
		LastUpdated:             src.Status.Version.LastUpdated,
	}
	dst.Status.AvailableReplicas = src.Status.AvailableReplicas
	dst.Status.ServiceURL = src.Status.ServiceURL
	dst.Status.RouteURL = src.Status.RouteURL

	if len(src.Status.Conditions) > 0 {
		dst.Status.Conditions = make([]metav1.Condition, len(src.Status.Conditions))
		copy(dst.Status.Conditions, src.Status.Conditions)
	}

	if len(src.Status.DistributionConfig.Providers) > 0 || src.Status.DistributionConfig.ActiveDistribution != "" {
		dst.Status.DistributionConfig = DistributionConfig{
			ActiveDistribution: src.Status.DistributionConfig.ActiveDistribution,
		}
		if len(src.Status.DistributionConfig.AvailableDistributions) > 0 {
			dst.Status.DistributionConfig.AvailableDistributions = src.Status.DistributionConfig.AvailableDistributions
		}
		for _, p := range src.Status.DistributionConfig.Providers {
			dst.Status.DistributionConfig.Providers = append(
				dst.Status.DistributionConfig.Providers,
				ProviderInfo{
					API:          p.API,
					ProviderID:   p.ProviderID,
					ProviderType: p.ProviderType,
					Config:       p.Config,
					Health: ProviderHealthStatus{
						Status:  p.Health.Status,
						Message: p.Health.Message,
					},
				},
			)
		}
	}
}

func isEmptyResources(r corev1.ResourceRequirements) bool {
	return len(r.Limits) == 0 && len(r.Requests) == 0
}
