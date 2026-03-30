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

package config

import (
	"sort"
	"strings"

	v1alpha2 "github.com/llamastack/llama-stack-k8s-operator/api/v1alpha2"
	corev1 "k8s.io/api/core/v1"
)

// secretRefEntry represents a single secret reference found in provider or storage spec.
type secretRefEntry struct {
	ProviderID string
	Field      string
	SecretName string
	SecretKey  string
}

// ResolveSecrets scans the spec for all secretKeyRef references and produces:
// 1. EnvVar definitions for the Deployment container
// 2. A substitution map: "original identifier" -> "${env.VAR_NAME}"
// Naming convention: LLSD_<PROVIDER_ID>_<FIELD> where provider ID is uppercased with hyphens replaced by underscores.
func ResolveSecrets(spec *v1alpha2.LlamaStackDistributionSpec) (*SecretResolution, error) {
	resolution := &SecretResolution{
		EnvVars:       make([]corev1.EnvVar, 0),
		Substitutions: make(map[string]string),
	}

	if spec == nil {
		return resolution, nil
	}

	if spec.Providers != nil {
		resolveProviderSecrets(resolution, spec.Providers)
	}

	if spec.Storage != nil {
		if spec.Storage.KV != nil && spec.Storage.KV.Password != nil {
			e := secretRefEntry{
				ProviderID: "kv-redis",
				Field:      "password",
				SecretName: spec.Storage.KV.Password.Name,
				SecretKey:  spec.Storage.KV.Password.Key,
			}
			addSecretToResolution(resolution, e)
		}
		if spec.Storage.SQL != nil && spec.Storage.SQL.ConnectionString != nil {
			e := secretRefEntry{
				ProviderID: "sql-postgres",
				Field:      "connectionString",
				SecretName: spec.Storage.SQL.ConnectionString.Name,
				SecretKey:  spec.Storage.SQL.ConnectionString.Key,
			}
			addSecretToResolution(resolution, e)
		}
	}

	return resolution, nil
}

func resolveProviderSecrets(resolution *SecretResolution, providers *v1alpha2.ProvidersSpec) {
	providerSlices := [][]v1alpha2.ProviderConfig{
		providers.Inference,
		providers.VectorIo,
		providers.ToolRuntime,
		providers.Telemetry,
	}

	for _, slice := range providerSlices {
		for _, pc := range slice {
			providerID := pc.ID
			if providerID == "" {
				providerID = GenerateProviderID(pc.Provider)
			}

			keys := make([]string, 0, len(pc.SecretRefs))
			for key := range pc.SecretRefs {
				keys = append(keys, key)
			}
			sort.Strings(keys)

			for _, key := range keys {
				ref := pc.SecretRefs[key]
				addSecretToResolution(resolution, secretRefEntry{
					ProviderID: providerID,
					Field:      key,
					SecretName: ref.Name,
					SecretKey:  ref.Key,
				})
			}
		}
	}
}

// addSecretToResolution adds an env var and substitution for a secret ref.
func addSecretToResolution(resolution *SecretResolution, e secretRefEntry) {
	envName := GenerateEnvVarName(e.ProviderID, e.Field)
	ident := e.ProviderID + ":" + e.Field

	if _, exists := resolution.Substitutions[ident]; exists {
		return
	}

	resolution.EnvVars = append(resolution.EnvVars, corev1.EnvVar{
		Name: envName,
		ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{Name: e.SecretName},
				Key:                  e.SecretKey,
			},
		},
	})
	resolution.Substitutions[ident] = "${env." + envName + "}"
}

// GenerateEnvVarName creates a deterministic env var name from provider ID and field name.
// Convention: LLSD_<PROVIDER_ID>_<FIELD> uppercased, with hyphens and colons replaced
// by underscores to produce valid Kubernetes env var names ([A-Z0-9_]+).
func GenerateEnvVarName(providerID, field string) string {
	r := strings.NewReplacer("-", "_", "::", "_", ":", "_")
	normalized := strings.ToUpper(r.Replace(providerID))
	fieldNorm := strings.ToUpper(r.Replace(field))
	return "LLSD_" + normalized + "_" + fieldNorm
}
