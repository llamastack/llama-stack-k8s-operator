package config

import (
	"fmt"
	"strings"

	v1alpha2 "github.com/llamastack/llama-stack-k8s-operator/api/v1alpha2"
	corev1 "k8s.io/api/core/v1"
)

// CollectSecretRefs extracts all secret references from the spec and produces
// Kubernetes EnvVar definitions with valueFrom.secretKeyRef.
func CollectSecretRefs(spec *v1alpha2.LlamaStackDistributionSpec) []corev1.EnvVar {
	var envVars []corev1.EnvVar

	// Collect from providers
	if spec.Providers != nil {
		for _, ap := range AllAPIProviders(spec.Providers) {
			for _, p := range ap.Providers {
				providerID := ResolveProviderID(p)
				envVars = append(envVars, collectProviderSecrets(providerID, p)...)
			}
		}
	}

	// Collect from storage
	if spec.Storage != nil {
		envVars = append(envVars, collectStorageSecrets(spec.Storage)...)
	}

	return envVars
}

func collectProviderSecrets(providerID string, p v1alpha2.ProviderConfig) []corev1.EnvVar {
	var envVars []corev1.EnvVar

	if p.APIKey != nil {
		envVars = append(envVars, corev1.EnvVar{
			Name: envVarName(providerID, "API_KEY"),
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: p.APIKey.Name},
					Key:                  p.APIKey.Key,
				},
			},
		})
	}

	if len(p.SecretRefs) > 0 {
		keys := sortedKeys(p.SecretRefs)
		for _, k := range keys {
			ref := p.SecretRefs[k]
			envVars = append(envVars, corev1.EnvVar{
				Name: envVarName(providerID, normalizeEnvVarSegment(k)),
				ValueFrom: &corev1.EnvVarSource{
					SecretKeyRef: &corev1.SecretKeySelector{
						LocalObjectReference: corev1.LocalObjectReference{Name: ref.Name},
						Key:                  ref.Key,
					},
				},
			})
		}
	}

	return envVars
}

func collectStorageSecrets(storage *v1alpha2.StorageSpec) []corev1.EnvVar {
	var envVars []corev1.EnvVar

	if storage.KV != nil && storage.KV.Password != nil {
		envVars = append(envVars, corev1.EnvVar{
			Name: "LLSD_STORAGE_KV_PASSWORD",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: storage.KV.Password.Name},
					Key:                  storage.KV.Password.Key,
				},
			},
		})
	}

	if storage.SQL != nil && storage.SQL.ConnectionString != nil {
		envVars = append(envVars, corev1.EnvVar{
			Name: "LLSD_STORAGE_SQL_CONNECTION_STRING",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: &corev1.SecretKeySelector{
					LocalObjectReference: corev1.LocalObjectReference{Name: storage.SQL.ConnectionString.Name},
					Key:                  storage.SQL.ConnectionString.Key,
				},
			},
		})
	}

	return envVars
}


// envVarName constructs the environment variable name: LLSD_<PROVIDER_ID>_<FIELD>.
func envVarName(providerID, field string) string {
	return fmt.Sprintf("LLSD_%s_%s", normalizeEnvVarSegment(providerID), field)
}

// normalizeEnvVarSegment uppercases and replaces hyphens with underscores.
func normalizeEnvVarSegment(s string) string {
	return strings.ToUpper(strings.ReplaceAll(s, "-", "_"))
}
