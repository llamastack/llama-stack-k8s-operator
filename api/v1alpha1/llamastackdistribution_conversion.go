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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/conversion"
)

const (
	annV1Alpha1Extras = "llamastack.io/v1alpha1-extras"
	annV1Alpha2Extras = "llamastack.io/v1alpha2-extras"

	// Legacy individual annotation keys (read-only, for backward compat).
	legacyAnnPrefix                 = "llamastack.io/v1alpha1-"
	legacyAnnContainerName          = legacyAnnPrefix + "container-name"
	legacyAnnTerminationGracePeriod = legacyAnnPrefix + "termination-grace-period"
	legacyAnnUserConfigNamespace    = legacyAnnPrefix + "user-config-namespace"
	legacyAnnCABundleNamespace      = legacyAnnPrefix + "ca-bundle-namespace"
	legacyAnnCABundleKeys           = legacyAnnPrefix + "ca-bundle-keys"
)

// v1alpha1Extras holds v1alpha1-only fields preserved through round-trips.
type v1alpha1Extras struct {
	ContainerName          string   `json:"containerName,omitempty"`
	TerminationGracePeriod *int64   `json:"terminationGracePeriod,omitempty"`
	UserConfigNamespace    string   `json:"userConfigNamespace,omitempty"`
	CABundleNamespace      string   `json:"caBundleNamespace,omitempty"`
	CABundleKeys           []string `json:"caBundleKeys,omitempty"`
}

// v1alpha2Extras holds v1alpha2-only fields preserved through round-trips.
type v1alpha2Extras struct {
	Providers         *v1alpha2.ProvidersSpec         `json:"providers,omitempty"`
	Resources         *v1alpha2.ResourcesSpec         `json:"resources,omitempty"`
	Storage           *v1alpha2.StateStorageSpec      `json:"storage,omitempty"`
	Disabled          []string                        `json:"disabled,omitempty"`
	ExternalProviders *v1alpha2.ExternalProvidersSpec `json:"externalProviders,omitempty"`
	ExposeHostname    string                          `json:"exposeHostname,omitempty"`
}

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

	dst.Spec.Distribution = v1alpha2.DistributionSpec{
		Name:  src.Spec.Server.Distribution.Name,
		Image: src.Spec.Server.Distribution.Image,
	}

	dst.Spec.Workload = convertToWorkload(src)
	dst.Spec.Networking = convertToNetworking(src)

	if src.Spec.Server.UserConfig != nil {
		dst.Spec.OverrideConfig = &v1alpha2.OverrideConfigSpec{
			ConfigMapName: src.Spec.Server.UserConfig.ConfigMapName,
		}
	}

	convertToStatus(src, dst)

	saveV1Alpha1Extras(src, dst)
	restoreV1Alpha2Extras(dst)
	cleanupLegacyAnnotations(dst.Annotations)

	return nil
}

// ConvertFrom converts the Hub version (v1alpha2) to this v1alpha1 LlamaStackDistribution.
func (dst *LlamaStackDistribution) ConvertFrom(srcRaw conversion.Hub) error {
	src, ok := srcRaw.(*v1alpha2.LlamaStackDistribution)
	if !ok {
		return fmt.Errorf("failed to convert: expected *v1alpha2.LlamaStackDistribution, got %T", srcRaw)
	}

	dst.ObjectMeta = src.ObjectMeta

	dst.Spec.Server.Distribution = DistributionType{
		Name:  src.Spec.Distribution.Name,
		Image: src.Spec.Distribution.Image,
	}

	convertFromWorkload(dst, src.Spec.Workload)
	convertFromNetworking(dst, src.Spec.Networking)

	if src.Spec.OverrideConfig != nil {
		dst.Spec.Server.UserConfig = &UserConfigSpec{
			ConfigMapName: src.Spec.OverrideConfig.ConfigMapName,
		}
	}

	convertFromStatus(src, dst)

	saveV1Alpha2Extras(src, dst)
	restoreV1Alpha1Extras(dst)
	cleanupLegacyAnnotations(dst.Annotations)

	return nil
}

// ---------------------------------------------------------------------------
// ConvertTo helpers (v1alpha1 → v1alpha2)
// ---------------------------------------------------------------------------

func convertToWorkload(src *LlamaStackDistribution) *v1alpha2.WorkloadSpec {
	w := &v1alpha2.WorkloadSpec{}
	hasContent := false

	if src.Spec.Replicas > 0 {
		r := src.Spec.Replicas
		w.Replicas = &r
		hasContent = true
	}

	if src.Spec.Server.Workers != nil {
		w.Workers = src.Spec.Server.Workers
		hasContent = true
	}

	cs := src.Spec.Server.ContainerSpec
	if !isEmptyResources(cs.Resources) {
		w.Resources = &cs.Resources
		hasContent = true
	}

	if overrides := convertToOverrides(src); overrides != nil {
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

func convertToOverrides(src *LlamaStackDistribution) *v1alpha2.WorkloadOverrides {
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
	}

	if !hasContent {
		return nil
	}
	return o
}

func convertToNetworking(src *LlamaStackDistribution) *v1alpha2.NetworkingSpec {
	n := &v1alpha2.NetworkingSpec{}
	hasContent := false

	if src.Spec.Server.ContainerSpec.Port > 0 {
		n.Port = src.Spec.Server.ContainerSpec.Port
		hasContent = true
	}

	if convertToTLS(src, n) {
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

func convertToTLS(src *LlamaStackDistribution, n *v1alpha2.NetworkingSpec) bool {
	if src.Spec.Server.TLSConfig == nil || src.Spec.Server.TLSConfig.CABundle == nil {
		return false
	}
	cab := src.Spec.Server.TLSConfig.CABundle
	n.TLS = &v1alpha2.TLSSpec{
		CABundle: &v1alpha2.CABundleConfig{
			ConfigMapName: cab.ConfigMapName,
		},
	}
	return true
}

func convertToNetworkAccess(src *LlamaStackDistribution, n *v1alpha2.NetworkingSpec) bool {
	if src.Spec.Network == nil {
		return false
	}
	hasContent := false
	if src.Spec.Network.ExposeRoute {
		n.Expose = &v1alpha2.ExposeConfig{}
		hasContent = true
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

	if len(src.Status.Conditions) > 0 {
		dst.Status.Conditions = make([]metav1.Condition, len(src.Status.Conditions))
		copy(dst.Status.Conditions, src.Status.Conditions)
	}

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

// ---------------------------------------------------------------------------
// ConvertFrom helpers (v1alpha2 → v1alpha1)
// ---------------------------------------------------------------------------

func convertFromWorkload(dst *LlamaStackDistribution, w *v1alpha2.WorkloadSpec) {
	if w == nil {
		dst.Spec.Replicas = 1
		return
	}

	if w.Replicas != nil {
		dst.Spec.Replicas = *w.Replicas
	} else {
		dst.Spec.Replicas = 1
	}

	dst.Spec.Server.Workers = w.Workers

	if w.Resources != nil {
		dst.Spec.Server.ContainerSpec.Resources = *w.Resources
	}

	convertFromOverrides(dst, w)
	convertFromWorkloadScaling(dst, w)
}

func convertFromOverrides(dst *LlamaStackDistribution, w *v1alpha2.WorkloadSpec) {
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

func convertFromTLS(dst *LlamaStackDistribution, n *v1alpha2.NetworkingSpec) {
	if n.TLS == nil || n.TLS.CABundle == nil {
		return
	}
	dst.Spec.Server.TLSConfig = &TLSConfig{
		CABundle: &CABundleConfig{
			ConfigMapName: n.TLS.CABundle.ConfigMapName,
		},
	}
}

func convertFromNetworking(dst *LlamaStackDistribution, n *v1alpha2.NetworkingSpec) {
	if n == nil {
		dst.Spec.Server.ContainerSpec.Port = DefaultServerPort
		return
	}

	if n.Port > 0 {
		dst.Spec.Server.ContainerSpec.Port = n.Port
	} else {
		dst.Spec.Server.ContainerSpec.Port = DefaultServerPort
	}

	convertFromTLS(dst, n)

	if n.Expose != nil {
		if dst.Spec.Network == nil {
			dst.Spec.Network = &NetworkSpec{}
		}
		dst.Spec.Network.ExposeRoute = true
	}

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

// ---------------------------------------------------------------------------
// Extras: JSON-blob annotation save/restore for lossless round-trips
// ---------------------------------------------------------------------------

var convLog = ctrl.Log.WithName("conversion")

// saveV1Alpha1Extras serializes v1alpha1-only fields into a JSON annotation on
// the v1alpha2 destination so they survive a v1alpha1 → v1alpha2 → v1alpha1 trip.
func saveV1Alpha1Extras(src *LlamaStackDistribution, dst *v1alpha2.LlamaStackDistribution) {
	extras := collectV1Alpha1Extras(src)
	marshalExtrasAnnotation(dst.Annotations, annV1Alpha1Extras, extras)
}

func collectV1Alpha1Extras(src *LlamaStackDistribution) v1alpha1Extras {
	extras := v1alpha1Extras{}
	if cs := src.Spec.Server.ContainerSpec; cs.Name != "" && cs.Name != DefaultContainerName {
		extras.ContainerName = cs.Name
	}
	if po := src.Spec.Server.PodOverrides; po != nil && po.TerminationGracePeriodSeconds != nil {
		extras.TerminationGracePeriod = po.TerminationGracePeriodSeconds
	}
	if uc := src.Spec.Server.UserConfig; uc != nil && uc.ConfigMapNamespace != "" {
		extras.UserConfigNamespace = uc.ConfigMapNamespace
	}
	if tc := src.Spec.Server.TLSConfig; tc != nil && tc.CABundle != nil {
		extras.CABundleNamespace = tc.CABundle.ConfigMapNamespace
		extras.CABundleKeys = tc.CABundle.ConfigMapKeys
	}
	return extras
}

func marshalExtrasAnnotation(annotations map[string]string, key string, v interface{}) {
	data, err := json.Marshal(v)
	if err != nil {
		convLog.Error(err, "failed to marshal extras annotation", "key", key)
		return
	}
	if string(data) == "{}" {
		return
	}
	annotations[key] = string(data)
}

// restoreV1Alpha2Extras deserializes v1alpha2-only fields from a JSON annotation
// carried on the v1alpha2 destination (originally set by saveV1Alpha2Extras during
// a previous ConvertFrom) and applies them to the v1alpha2 object. The annotation
// is deleted after restoration since the fields are now in native spec fields.
func restoreV1Alpha2Extras(dst *v1alpha2.LlamaStackDistribution) {
	raw, ok := dst.Annotations[annV1Alpha2Extras]
	if !ok {
		return
	}

	var extras v1alpha2Extras
	if err := json.Unmarshal([]byte(raw), &extras); err != nil {
		convLog.Error(err, "failed to unmarshal v1alpha2 extras annotation")
		return
	}

	applyV1Alpha2Extras(&dst.Spec, extras)
	delete(dst.Annotations, annV1Alpha2Extras)
}

func applyV1Alpha2Extras(spec *v1alpha2.LlamaStackDistributionSpec, extras v1alpha2Extras) {
	if spec.Providers == nil {
		spec.Providers = extras.Providers
	}
	if spec.Resources == nil {
		spec.Resources = extras.Resources
	}
	if spec.Storage == nil {
		spec.Storage = extras.Storage
	}
	if len(spec.Disabled) == 0 {
		spec.Disabled = extras.Disabled
	}
	if spec.ExternalProviders == nil {
		spec.ExternalProviders = extras.ExternalProviders
	}
	if extras.ExposeHostname != "" {
		if spec.Networking == nil {
			spec.Networking = &v1alpha2.NetworkingSpec{}
		}
		if spec.Networking.Expose != nil {
			spec.Networking.Expose.Hostname = extras.ExposeHostname
		} else {
			spec.Networking.Expose = &v1alpha2.ExposeConfig{Hostname: extras.ExposeHostname}
		}
	}
}

// saveV1Alpha2Extras serializes v1alpha2-only fields into a JSON annotation on
// the v1alpha1 destination so they survive a v1alpha2 → v1alpha1 → v1alpha2 trip.
func saveV1Alpha2Extras(src *v1alpha2.LlamaStackDistribution, dst *LlamaStackDistribution) {
	extras := v1alpha2Extras{
		Providers:         src.Spec.Providers,
		Resources:         src.Spec.Resources,
		Storage:           src.Spec.Storage,
		Disabled:          src.Spec.Disabled,
		ExternalProviders: src.Spec.ExternalProviders,
	}
	if src.Spec.Networking != nil && src.Spec.Networking.Expose != nil {
		extras.ExposeHostname = src.Spec.Networking.Expose.Hostname
	}

	if dst.Annotations == nil {
		dst.Annotations = make(map[string]string)
	}
	marshalExtrasAnnotation(dst.Annotations, annV1Alpha2Extras, extras)
}

// restoreV1Alpha1Extras restores v1alpha1-only fields from either the JSON blob
// annotation (preferred) or legacy individual annotations (backward compat).
func restoreV1Alpha1Extras(dst *LlamaStackDistribution) {
	if dst.Annotations == nil {
		return
	}

	if raw, ok := dst.Annotations[annV1Alpha1Extras]; ok {
		if restoreV1Alpha1ExtrasFromBlob(dst, raw) {
			delete(dst.Annotations, annV1Alpha1Extras)
		}
		return
	}

	restoreV1Alpha1ExtrasLegacy(dst)
}

func restoreV1Alpha1ExtrasFromBlob(dst *LlamaStackDistribution, raw string) bool {
	var extras v1alpha1Extras
	if err := json.Unmarshal([]byte(raw), &extras); err != nil {
		convLog.Error(err, "failed to unmarshal v1alpha1 extras annotation")
		return false
	}
	applyV1Alpha1Extras(dst, extras)
	return true
}

func applyV1Alpha1Extras(dst *LlamaStackDistribution, extras v1alpha1Extras) {
	if extras.ContainerName != "" {
		dst.Spec.Server.ContainerSpec.Name = extras.ContainerName
	}
	if extras.TerminationGracePeriod != nil {
		if dst.Spec.Server.PodOverrides == nil {
			dst.Spec.Server.PodOverrides = &PodOverrides{}
		}
		dst.Spec.Server.PodOverrides.TerminationGracePeriodSeconds = extras.TerminationGracePeriod
	}
	if extras.UserConfigNamespace != "" && dst.Spec.Server.UserConfig != nil {
		dst.Spec.Server.UserConfig.ConfigMapNamespace = extras.UserConfigNamespace
	}
	applyV1Alpha1CABundleExtras(dst, extras)
}

func applyV1Alpha1CABundleExtras(dst *LlamaStackDistribution, extras v1alpha1Extras) {
	if dst.Spec.Server.TLSConfig == nil || dst.Spec.Server.TLSConfig.CABundle == nil {
		return
	}
	if extras.CABundleNamespace != "" {
		dst.Spec.Server.TLSConfig.CABundle.ConfigMapNamespace = extras.CABundleNamespace
	}
	if len(extras.CABundleKeys) > 0 {
		dst.Spec.Server.TLSConfig.CABundle.ConfigMapKeys = extras.CABundleKeys
	}
}

// restoreV1Alpha1ExtrasLegacy reads from the old per-field annotations written
// by earlier code versions. Only used when the JSON blob annotation is absent.
func restoreV1Alpha1ExtrasLegacy(dst *LlamaStackDistribution) {
	if name, ok := dst.Annotations[legacyAnnContainerName]; ok {
		dst.Spec.Server.ContainerSpec.Name = name
	}
	if tgp, ok := dst.Annotations[legacyAnnTerminationGracePeriod]; ok {
		if v, err := strconv.ParseInt(tgp, 10, 64); err == nil {
			if dst.Spec.Server.PodOverrides == nil {
				dst.Spec.Server.PodOverrides = &PodOverrides{}
			}
			dst.Spec.Server.PodOverrides.TerminationGracePeriodSeconds = &v
		}
	}
	if ns, ok := dst.Annotations[legacyAnnUserConfigNamespace]; ok && dst.Spec.Server.UserConfig != nil {
		dst.Spec.Server.UserConfig.ConfigMapNamespace = ns
	}
	restoreLegacyCABundleAnnotations(dst)
}

func restoreLegacyCABundleAnnotations(dst *LlamaStackDistribution) {
	if dst.Spec.Server.TLSConfig == nil || dst.Spec.Server.TLSConfig.CABundle == nil {
		return
	}
	if ns, ok := dst.Annotations[legacyAnnCABundleNamespace]; ok {
		dst.Spec.Server.TLSConfig.CABundle.ConfigMapNamespace = ns
	}
	if keysJSON, ok := dst.Annotations[legacyAnnCABundleKeys]; ok {
		if err := json.Unmarshal([]byte(keysJSON), &dst.Spec.Server.TLSConfig.CABundle.ConfigMapKeys); err != nil {
			convLog.Error(err, "failed to unmarshal legacy CA bundle keys annotation", "value", keysJSON)
		}
	}
}

// cleanupLegacyAnnotations removes old per-field annotations that have been
// superseded by the JSON blob annotations.
func cleanupLegacyAnnotations(annotations map[string]string) {
	for _, key := range []string{
		legacyAnnContainerName,
		legacyAnnTerminationGracePeriod,
		legacyAnnUserConfigNamespace,
		legacyAnnCABundleNamespace,
		legacyAnnCABundleKeys,
	} {
		delete(annotations, key)
	}
}

func isEmptyResources(r corev1.ResourceRequirements) bool {
	return len(r.Limits) == 0 && len(r.Requests) == 0
}
