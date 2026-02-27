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
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	llamav1alpha1 "github.com/llamastack/llama-stack-k8s-operator/api/v1alpha1"
	v1alpha2 "github.com/llamastack/llama-stack-k8s-operator/api/v1alpha2"
	llsconfig "github.com/llamastack/llama-stack-k8s-operator/pkg/config"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// v1alpha2 Condition types.
const (
	ConditionTypeConfigGenerated   = "ConfigGenerated"
	ConditionTypeDeploymentUpdated = "DeploymentUpdated"
	ConditionTypeAvailable         = "Available"
	ConditionTypeSecretsResolved   = "SecretsResolved"
)

// v1alpha2 Condition reasons.
const (
	ReasonConfigGenSucceeded  = "ConfigGenerationSucceeded"
	ReasonConfigGenFailed     = "ConfigGenerationFailed"
	ReasonBaseConfigRequired  = "BaseConfigRequired"
	ReasonDeployUpdateSucceed = "DeploymentUpdateSucceeded"
	ReasonDeployUpdateFailed  = "DeploymentUpdateFailed"
	ReasonMinReplicasAvail    = "MinimumReplicasAvailable"
	ReasonReplicasUnavail     = "ReplicasUnavailable"
	ReasonAllSecretsFound     = "AllSecretsFound"
	ReasonSecretNotFound      = "SecretNotFound"
	ReasonUpgradeConfigFail   = "UpgradeConfigFailure"
)

// DetermineConfigSource determines the configuration source for a v1alpha2 CR.
func DetermineConfigSource(spec *v1alpha2.LlamaStackDistributionSpec) llsconfig.ConfigSource {
	if spec.OverrideConfig != nil {
		return llsconfig.ConfigSourceOverride
	}
	if spec.Providers != nil || spec.Resources != nil || spec.Storage != nil {
		return llsconfig.ConfigSourceGenerated
	}
	return llsconfig.ConfigSourceDistributionDefault
}

// ReconcileV1Alpha2Config handles config generation and ConfigMap management for v1alpha2 CRs.
// Returns the generated config map name, content hash, env vars, and any error.
func (r *LlamaStackDistributionReconciler) ReconcileV1Alpha2Config(
	ctx context.Context,
	instance *v1alpha2.LlamaStackDistribution,
) (*llsconfig.GeneratedConfig, string, error) {
	logger := log.FromContext(ctx)

	configSource := DetermineConfigSource(&instance.Spec)
	logger.V(1).Info("determined config source", "source", configSource)

	switch configSource {
	case llsconfig.ConfigSourceOverride:
		return r.handleOverrideConfig(ctx, instance)

	case llsconfig.ConfigSourceGenerated:
		return r.handleGeneratedConfig(ctx, instance)

	case llsconfig.ConfigSourceDistributionDefault:
		return r.handleDistributionDefault(ctx, instance)
	}

	return nil, "", fmt.Errorf("failed to determine config source: unknown value %s", configSource)
}

// handleOverrideConfig uses the user-provided ConfigMap directly.
func (r *LlamaStackDistributionReconciler) handleOverrideConfig(
	ctx context.Context,
	instance *v1alpha2.LlamaStackDistribution,
) (*llsconfig.GeneratedConfig, string, error) {
	logger := log.FromContext(ctx)

	cmName := instance.Spec.OverrideConfig.ConfigMapName
	cm := &corev1.ConfigMap{}
	if err := r.Get(ctx, types.NamespacedName{
		Name:      cmName,
		Namespace: instance.Namespace,
	}, cm); err != nil {
		if k8serrors.IsNotFound(err) {
			return nil, "", fmt.Errorf("failed to find overrideConfig ConfigMap %q in namespace %q", cmName, instance.Namespace)
		}
		return nil, "", fmt.Errorf("failed to get overrideConfig ConfigMap: %w", err)
	}

	configData, ok := cm.Data["config.yaml"]
	if !ok {
		return nil, "", fmt.Errorf("failed to read config from ConfigMap %q: missing 'config.yaml' key", cmName)
	}

	resolvedImage, err := r.resolveV1Alpha2Image(instance)
	if err != nil {
		return nil, "", err
	}

	contentHash := llsconfig.ComputeContentHash(configData)
	logger.V(1).Info("using override config", "configMap", cmName, "hash", contentHash[:8])

	return &llsconfig.GeneratedConfig{
		ConfigYAML:  configData,
		ContentHash: contentHash,
	}, resolvedImage, nil
}

// handleGeneratedConfig generates config from the v1alpha2 spec.
func (r *LlamaStackDistributionReconciler) handleGeneratedConfig(
	ctx context.Context,
	instance *v1alpha2.LlamaStackDistribution,
) (*llsconfig.GeneratedConfig, string, error) {
	logger := log.FromContext(ctx)

	resolver := llsconfig.NewBaseConfigResolver(
		r.ClusterInfo.DistributionImages,
		r.ImageMappingOverrides,
	)

	generated, resolvedImage, err := llsconfig.GenerateConfig(ctx, &instance.Spec, resolver)
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate config: %w", err)
	}

	logger.Info("generated config",
		"hash", generated.ContentHash[:8],
		"providers", generated.ProviderCount,
		"resources", generated.ResourceCount,
		"version", generated.ConfigVersion,
	)

	return generated, resolvedImage, nil
}

// handleDistributionDefault uses the base config from the distribution as-is.
func (r *LlamaStackDistributionReconciler) handleDistributionDefault(
	ctx context.Context,
	instance *v1alpha2.LlamaStackDistribution,
) (*llsconfig.GeneratedConfig, string, error) {
	logger := log.FromContext(ctx)

	resolver := llsconfig.NewBaseConfigResolver(
		r.ClusterInfo.DistributionImages,
		r.ImageMappingOverrides,
	)

	base, resolvedImage, err := resolver.Resolve(ctx, instance.Spec.Distribution)
	if err != nil {
		return nil, "", fmt.Errorf("failed to resolve distribution default: %w", err)
	}

	configYAML, err := llsconfig.RenderConfigYAML(base)
	if err != nil {
		return nil, "", fmt.Errorf("failed to render default config: %w", err)
	}

	contentHash := llsconfig.ComputeContentHash(configYAML)
	providerCount := 0
	for _, v := range base.Providers {
		if list, ok := v.([]interface{}); ok {
			providerCount += len(list)
		}
	}

	logger.V(1).Info("using distribution default config", "hash", contentHash[:8])

	return &llsconfig.GeneratedConfig{
		ConfigYAML:    configYAML,
		ContentHash:   contentHash,
		ProviderCount: providerCount,
		ConfigVersion: base.Version,
	}, resolvedImage, nil
}

// ReconcileGeneratedConfigMap creates or manages the generated ConfigMap for a v1alpha2 CR.
// Uses immutable pattern: new ConfigMap on changes, cleanup of old ones.
func (r *LlamaStackDistributionReconciler) ReconcileGeneratedConfigMap(
	ctx context.Context,
	instance *v1alpha2.LlamaStackDistribution,
	generated *llsconfig.GeneratedConfig,
) (string, error) {
	logger := log.FromContext(ctx)

	configMapName := fmt.Sprintf("%s-config-%s", instance.Name, generated.ContentHash[:8])

	// Check if this ConfigMap already exists
	existing := &corev1.ConfigMap{}
	err := r.Get(ctx, types.NamespacedName{
		Name:      configMapName,
		Namespace: instance.Namespace,
	}, existing)

	if err == nil {
		logger.V(1).Info("generated ConfigMap already exists", "configMap", configMapName)
		return configMapName, nil
	}

	if !k8serrors.IsNotFound(err) {
		return "", fmt.Errorf("failed to check generated ConfigMap: %w", err)
	}

	// Create new ConfigMap
	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: instance.Namespace,
			Labels: map[string]string{
				"app.kubernetes.io/managed-by": "llama-stack-operator",
				"app.kubernetes.io/instance":   instance.Name,
				"app.kubernetes.io/component":  "generated-config",
			},
		},
		Data: map[string]string{
			"config.yaml": generated.ConfigYAML,
		},
	}

	if err := ctrl.SetControllerReference(instance, cm, r.Scheme); err != nil {
		return "", fmt.Errorf("failed to set owner reference on generated ConfigMap: %w", err)
	}

	if err := r.Create(ctx, cm); err != nil {
		return "", fmt.Errorf("failed to create generated ConfigMap: %w", err)
	}

	logger.Info("created generated ConfigMap", "configMap", configMapName)

	// Cleanup old ConfigMaps (keep last 2)
	if err := r.cleanupOldConfigMaps(ctx, instance, configMapName); err != nil {
		logger.Error(err, "failed to cleanup old ConfigMaps")
	}

	return configMapName, nil
}

// cleanupOldConfigMaps removes generated ConfigMaps older than the last 2.
func (r *LlamaStackDistributionReconciler) cleanupOldConfigMaps(
	ctx context.Context,
	instance *v1alpha2.LlamaStackDistribution,
	currentName string,
) error {
	logger := log.FromContext(ctx)

	cmList := &corev1.ConfigMapList{}
	matchingLabels := map[string]string{
		"app.kubernetes.io/component": "generated-config",
		"app.kubernetes.io/instance":  instance.Name,
	}
	if err := r.List(ctx, cmList, matchLabels(matchingLabels)); err != nil {
		return fmt.Errorf("failed to list generated ConfigMaps: %w", err)
	}

	// Sort by creation timestamp (newest first)
	sort.Slice(cmList.Items, func(i, j int) bool {
		return cmList.Items[i].CreationTimestamp.After(cmList.Items[j].CreationTimestamp.Time)
	})

	// Keep the 2 newest, delete the rest
	for i, cm := range cmList.Items {
		if i < 2 || cm.Name == currentName {
			continue
		}
		if err := r.Delete(ctx, &cmList.Items[i]); err != nil && !k8serrors.IsNotFound(err) {
			logger.Error(err, "failed to delete old ConfigMap", "configMap", cm.Name)
		} else {
			logger.V(1).Info("deleted old generated ConfigMap", "configMap", cm.Name)
		}
	}

	return nil
}

// matchLabels wraps a map as a client.MatchingLabels list option.
func matchLabels(labels map[string]string) matchingLabelsOption {
	return matchingLabelsOption(labels)
}

type matchingLabelsOption map[string]string

func (o matchingLabelsOption) ApplyToList(opts *client.ListOptions) {
	sel := labels.SelectorFromSet(map[string]string(o))
	opts.LabelSelector = sel
}

// ValidateV1Alpha2SecretRefs validates that all secretKeyRef references in a v1alpha2 spec
// point to existing Secrets.
func (r *LlamaStackDistributionReconciler) ValidateV1Alpha2SecretRefs(
	ctx context.Context,
	spec *v1alpha2.LlamaStackDistributionSpec,
	namespace string,
) error {
	resolution, err := llsconfig.ResolveSecrets(spec)
	if err != nil {
		return fmt.Errorf("failed to collect secret references: %w", err)
	}

	for _, envVar := range resolution.EnvVars {
		if envVar.ValueFrom == nil || envVar.ValueFrom.SecretKeyRef == nil {
			continue
		}

		secretName := envVar.ValueFrom.SecretKeyRef.Name
		secretKey := envVar.ValueFrom.SecretKeyRef.Key

		secret := &corev1.Secret{}
		if err := r.Get(ctx, types.NamespacedName{
			Name:      secretName,
			Namespace: namespace,
		}, secret); err != nil {
			if k8serrors.IsNotFound(err) {
				return fmt.Errorf("failed to find Secret %q in namespace %q (referenced by env var %s)", secretName, namespace, envVar.Name)
			}
			return fmt.Errorf("failed to get Secret %q: %w", secretName, err)
		}

		if _, ok := secret.Data[secretKey]; !ok {
			return fmt.Errorf("failed to find key %q in Secret %q in namespace %q", secretKey, secretName, namespace)
		}
	}

	return nil
}

// ValidateV1Alpha2ConfigMapRefs validates ConfigMap references in overrideConfig and caBundle.
func (r *LlamaStackDistributionReconciler) ValidateV1Alpha2ConfigMapRefs(
	ctx context.Context,
	spec *v1alpha2.LlamaStackDistributionSpec,
	namespace string,
) error {
	// Validate overrideConfig ConfigMap
	if spec.OverrideConfig != nil {
		cm := &corev1.ConfigMap{}
		if err := r.Get(ctx, types.NamespacedName{
			Name:      spec.OverrideConfig.ConfigMapName,
			Namespace: namespace,
		}, cm); err != nil {
			if k8serrors.IsNotFound(err) {
				return fmt.Errorf("failed to find overrideConfig ConfigMap %q in namespace %q", spec.OverrideConfig.ConfigMapName, namespace)
			}
			return fmt.Errorf("failed to get overrideConfig ConfigMap: %w", err)
		}
	}

	// Validate caBundle ConfigMap
	if spec.Networking != nil && spec.Networking.TLS != nil && spec.Networking.TLS.CABundle != nil {
		cm := &corev1.ConfigMap{}
		if err := r.Get(ctx, types.NamespacedName{
			Name:      spec.Networking.TLS.CABundle.ConfigMapName,
			Namespace: namespace,
		}, cm); err != nil {
			if k8serrors.IsNotFound(err) {
				return fmt.Errorf("failed to find caBundle ConfigMap %q in namespace %q", spec.Networking.TLS.CABundle.ConfigMapName, namespace)
			}
			return fmt.Errorf("failed to get caBundle ConfigMap: %w", err)
		}
	}

	return nil
}

// ValidateProviderReferences validates that model provider references are valid provider IDs.
func ValidateProviderReferences(spec *v1alpha2.LlamaStackDistributionSpec) error {
	if spec.Resources == nil || spec.Providers == nil {
		return nil
	}

	providerIDs := collectProviderIDs(spec.Providers)

	for i, raw := range spec.Resources.Models {
		mc, err := llsconfig.ParsePolymorphicModel(&raw)
		if err != nil {
			return fmt.Errorf("failed to parse resources.models[%d]: %w", i, err)
		}
		if mc.Provider != "" {
			if _, ok := providerIDs[mc.Provider]; !ok {
				return fmt.Errorf(
					"resources.models[%d].provider: provider ID %q not found; available providers: %s",
					i, mc.Provider, strings.Join(sortedKeys(providerIDs), ", "),
				)
			}
		}
	}

	return nil
}

func collectProviderIDs(spec *v1alpha2.ProvidersSpec) map[string]bool {
	ids := make(map[string]bool)

	fields := []struct {
		name string
		raw  *apiextensionsv1.JSON
	}{
		{"inference", spec.Inference},
		{"safety", spec.Safety},
		{"vectorIo", spec.VectorIo},
		{"toolRuntime", spec.ToolRuntime},
		{"telemetry", spec.Telemetry},
	}

	for _, f := range fields {
		if f.raw == nil {
			continue
		}
		providers, err := llsconfig.ParsePolymorphicProvider(f.raw)
		if err != nil {
			continue
		}
		for _, p := range providers {
			id := p.ID
			if id == "" {
				id = llsconfig.GenerateProviderID(p.Provider)
			}
			ids[id] = true
		}
	}

	return ids
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// resolveV1Alpha2Image resolves the container image for a v1alpha2 distribution spec.
func (r *LlamaStackDistributionReconciler) resolveV1Alpha2Image(instance *v1alpha2.LlamaStackDistribution) (string, error) {
	dist := instance.Spec.Distribution

	if dist.Image != "" {
		return dist.Image, nil
	}

	if override, ok := r.ImageMappingOverrides[dist.Name]; ok {
		return override, nil
	}

	if image, ok := r.ClusterInfo.DistributionImages[dist.Name]; ok {
		return image, nil
	}

	return "", fmt.Errorf("failed to resolve distribution name %q: not found in distributions.json", dist.Name)
}

// updateV1Alpha2SpecificStatus sets v1alpha2-specific status fields. Only updates
// GeneratedAt when the config actually changes to avoid unnecessary status churn.
func (r *LlamaStackDistributionReconciler) updateV1Alpha2SpecificStatus(
	instance *v1alpha2.LlamaStackDistribution,
	generated *llsconfig.GeneratedConfig,
	resolvedImage string,
	configMapName string,
	configSource string,
) {
	if instance.Status.ResolvedDistribution == nil {
		instance.Status.ResolvedDistribution = &v1alpha2.ResolvedDistributionStatus{}
	}
	instance.Status.ResolvedDistribution.Image = resolvedImage
	instance.Status.ResolvedDistribution.ConfigSource = configSource

	if generated != nil {
		instance.Status.ResolvedDistribution.ConfigHash = generated.ContentHash

		if instance.Status.ConfigGeneration == nil {
			instance.Status.ConfigGeneration = &v1alpha2.ConfigGenerationStatus{}
		}
		configChanged := instance.Status.ConfigGeneration.ConfigMapName != configMapName
		instance.Status.ConfigGeneration.ConfigMapName = configMapName
		if configChanged || instance.Status.ConfigGeneration.GeneratedAt.IsZero() {
			instance.Status.ConfigGeneration.GeneratedAt = metav1.Now()
		}
		instance.Status.ConfigGeneration.ProviderCount = generated.ProviderCount
		instance.Status.ConfigGeneration.ResourceCount = generated.ResourceCount
		instance.Status.ConfigGeneration.ConfigVersion = generated.ConfigVersion
	}
}

// SetV1Alpha2Condition sets a condition on the v1alpha2 status.
func SetV1Alpha2Condition(
	status *v1alpha2.LlamaStackDistributionStatus,
	condType string,
	condStatus metav1.ConditionStatus,
	reason, message string,
) {
	if status.Conditions == nil {
		status.Conditions = make([]metav1.Condition, 0)
	}

	condition := metav1.Condition{
		Type:               condType,
		Status:             condStatus,
		Reason:             reason,
		Message:            message,
		LastTransitionTime: metav1.Now(),
	}

	for i := range status.Conditions {
		if status.Conditions[i].Type == condType {
			if status.Conditions[i].Status != condStatus {
				status.Conditions[i] = condition
			} else {
				status.Conditions[i].Reason = reason
				status.Conditions[i].Message = message
			}
			return
		}
	}

	status.Conditions = append(status.Conditions, condition)
}

// v1alpha2ConfigResult holds the results of v1alpha2 native config generation.
type v1alpha2ConfigResult struct {
	Generated    *llsconfig.GeneratedConfig
	ResolvedImg  string
	ConfigMap    string
	ConfigSource string
}

// handleV1Alpha2NativeConfig fetches the CR as v1alpha2 and, if native providers/resources
// are defined, generates a ConfigMap and wires it into the v1alpha1 instance for mounting.
// Returns nil result if the CR does not use v1alpha2 native config features.
func (r *LlamaStackDistributionReconciler) handleV1Alpha2NativeConfig(
	ctx context.Context,
	key types.NamespacedName,
	v1Instance *llamav1alpha1.LlamaStackDistribution,
) (*v1alpha2ConfigResult, error) {
	logger := log.FromContext(ctx)

	v2Instance := &v1alpha2.LlamaStackDistribution{}
	if err := r.Get(ctx, key, v2Instance); err != nil {
		return nil, fmt.Errorf("failed to fetch v1alpha2 instance: %w", err)
	}

	configSource := DetermineConfigSource(&v2Instance.Spec)
	// Only generate config when v1alpha2-native fields (providers/resources/storage)
	// are present. For override and distribution-default cases (including converted
	// v1alpha1 CRs), let the standard reconciliation handle things.
	if configSource != llsconfig.ConfigSourceGenerated {
		return nil, nil
	}

	generated, resolvedImage, err := r.ReconcileV1Alpha2Config(ctx, v2Instance)
	if err != nil {
		return nil, fmt.Errorf("failed to reconcile v1alpha2 config: %w", err)
	}

	cmName, err := r.ReconcileGeneratedConfigMap(ctx, v2Instance, generated)
	if err != nil {
		return nil, fmt.Errorf("failed to reconcile generated ConfigMap: %w", err)
	}

	logger.Info("v1alpha2 native config applied", "configMap", cmName, "source", configSource)

	// Wire the generated ConfigMap into the v1alpha1 instance so the standard
	// reconciliation creates the volume mount and hash annotation.
	v1Instance.Spec.Server.UserConfig = &llamav1alpha1.UserConfigSpec{
		ConfigMapName: cmName,
	}

	return &v1alpha2ConfigResult{
		Generated:    generated,
		ResolvedImg:  resolvedImage,
		ConfigMap:    cmName,
		ConfigSource: string(configSource),
	}, nil
}

// persistV1Alpha2Status merges the computed v1alpha1 status with v1alpha2-specific
// fields and persists as a single v1alpha2 status update. This avoids an infinite
// reconciliation loop that would occur if v1alpha1 and v1alpha2 status were updated
// separately (the v1alpha1 update erases v1alpha2-specific fields via conversion).
func (r *LlamaStackDistributionReconciler) persistV1Alpha2Status(
	ctx context.Context,
	key types.NamespacedName,
	v1Instance *llamav1alpha1.LlamaStackDistribution,
	result *v1alpha2ConfigResult,
) error {
	v2Instance := &v1alpha2.LlamaStackDistribution{}
	if err := r.Get(ctx, key, v2Instance); err != nil {
		return fmt.Errorf("failed to fetch v1alpha2 instance for status update: %w", err)
	}

	// Convert computed v1alpha1 status to v1alpha2 format using standard conversion.
	converted := &v1alpha2.LlamaStackDistribution{}
	if err := v1Instance.ConvertTo(converted); err != nil {
		return fmt.Errorf("failed to convert status to v1alpha2: %w", err)
	}

	// Apply converted common status fields.
	v2Instance.Status.Phase = converted.Status.Phase
	v2Instance.Status.Version = converted.Status.Version
	v2Instance.Status.AvailableReplicas = converted.Status.AvailableReplicas
	v2Instance.Status.ServiceURL = converted.Status.ServiceURL
	v2Instance.Status.RouteURL = converted.Status.RouteURL
	v2Instance.Status.Conditions = converted.Status.Conditions
	v2Instance.Status.DistributionConfig = converted.Status.DistributionConfig

	// Set v1alpha2-specific fields.
	r.updateV1Alpha2SpecificStatus(v2Instance, result.Generated, result.ResolvedImg, result.ConfigMap, result.ConfigSource)
	SetV1Alpha2Condition(&v2Instance.Status, ConditionTypeConfigGenerated,
		metav1.ConditionTrue, ReasonConfigGenSucceeded, "Configuration generated successfully")

	if err := r.Status().Update(ctx, v2Instance); err != nil {
		return fmt.Errorf("failed to update v1alpha2 status: %w", err)
	}
	return nil
}

// GetV1Alpha2ServerPort returns the server port from a v1alpha2 networking spec.
func GetV1Alpha2ServerPort(spec *v1alpha2.NetworkingSpec) int32 {
	if spec != nil && spec.Port > 0 {
		return spec.Port
	}
	return v1alpha2.DefaultServerPort
}

// ShouldExposeV1Alpha2 determines whether external access should be created.
func ShouldExposeV1Alpha2(spec *v1alpha2.NetworkingSpec) (bool, string) {
	if spec == nil || spec.Expose == nil || len(spec.Expose.Raw) == 0 {
		return false, ""
	}

	// Try as boolean
	var boolVal bool
	if err := json.Unmarshal(spec.Expose.Raw, &boolVal); err == nil {
		return boolVal, ""
	}

	// Try as object
	var objVal struct {
		Enabled  *bool  `json:"enabled,omitempty"`
		Hostname string `json:"hostname,omitempty"`
	}
	if err := json.Unmarshal(spec.Expose.Raw, &objVal); err == nil {
		if objVal.Enabled != nil {
			return *objVal.Enabled, objVal.Hostname
		}
		// Empty object {} treated as true
		return true, objVal.Hostname
	}

	return false, ""
}
