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
	"encoding/json"
	"fmt"
	"strings"

	v1alpha2 "github.com/llamastack/llama-stack-k8s-operator/api/v1alpha2"
	corev1 "k8s.io/api/core/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
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

	// Scan providers for apiKey and settings secretKeyRefs
	if spec.Providers != nil {
		providerFields := []struct {
			name string
			raw  *apiextensionsv1.JSON
		}{
			{"inference", spec.Providers.Inference},
			{"safety", spec.Providers.Safety},
			{"vectorIo", spec.Providers.VectorIo},
			{"toolRuntime", spec.Providers.ToolRuntime},
			{"telemetry", spec.Providers.Telemetry},
		}
		for _, pf := range providerFields {
			if pf.raw == nil {
				continue
			}
			entries, err := collectSecretRefsFromProviderField(pf.raw)
			if err != nil {
				return nil, fmt.Errorf("providers.%s: %w", pf.name, err)
			}
			for _, e := range entries {
				addSecretToResolution(resolution, e)
			}
		}
	}

	// Scan storage for password and connectionString secretKeyRefs
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

// addSecretToResolution adds an env var and substitution for a secret ref.
func addSecretToResolution(resolution *SecretResolution, e secretRefEntry) {
	envName := GenerateEnvVarName(e.ProviderID, e.Field)
	ident := e.ProviderID + ":" + e.Field

	// Avoid duplicates
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

// collectSecretRefsFromProviderField parses a provider field (single object or array) and returns all secret refs.
func collectSecretRefsFromProviderField(raw *apiextensionsv1.JSON) ([]secretRefEntry, error) {
	if raw == nil || len(raw.Raw) == 0 {
		return nil, nil
	}
	var decoded interface{}
	if err := json.Unmarshal(raw.Raw, &decoded); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}

	var providers []map[string]interface{}
	switch v := decoded.(type) {
	case map[string]interface{}:
		providers = []map[string]interface{}{v}
	case []interface{}:
		for _, item := range v {
			if m, ok := item.(map[string]interface{}); ok {
				providers = append(providers, m)
			}
		}
	default:
		return nil, fmt.Errorf("expected object or array, got %T", decoded)
	}

	var entries []secretRefEntry
	for _, p := range providers {
		providerID := getProviderID(p)
		provRaw := &apiextensionsv1.JSON{Raw: mustMarshal(p)}
		refs, err := CollectSecretRefs(providerID, provRaw)
		if err != nil {
			return nil, err
		}
		entries = append(entries, refs...)
	}
	return entries, nil
}

func getProviderID(p map[string]interface{}) string {
	if id, ok := p["id"].(string); ok && id != "" {
		return id
	}
	if prov, ok := p["provider"].(string); ok && prov != "" {
		return prov
	}
	return "default"
}

func mustMarshal(v interface{}) []byte {
	b, _ := json.Marshal(v)
	return b
}

// GenerateEnvVarName creates a deterministic env var name from provider ID and field name.
// Convention: LLSD_<PROVIDER_ID>_<FIELD> uppercased, hyphens -> underscores
func GenerateEnvVarName(providerID, field string) string {
	normalized := strings.ToUpper(strings.ReplaceAll(providerID, "-", "_"))
	fieldNorm := strings.ToUpper(strings.ReplaceAll(field, "-", "_"))
	return "LLSD_" + normalized + "_" + fieldNorm
}

// CollectSecretRefs traverses a provider JSON and returns all found SecretKeyRef entries.
// Only looks at top-level fields (apiKey) and top-level settings values (settings.<key>.secretKeyRef).
func CollectSecretRefs(providerID string, raw *apiextensionsv1.JSON) ([]secretRefEntry, error) {
	if raw == nil || len(raw.Raw) == 0 {
		return nil, nil
	}
	var m map[string]interface{}
	if err := json.Unmarshal(raw.Raw, &m); err != nil {
		return nil, fmt.Errorf("invalid provider JSON: %w", err)
	}

	var entries []secretRefEntry

	// apiKey: {name, key} or {secretKeyRef: {name, key}}
	if ak, ok := m["apiKey"]; ok && ak != nil {
		if ref := extractSecretRef(ak); ref != nil {
			entries = append(entries, secretRefEntry{
				ProviderID: providerID,
				Field:      "apiKey",
				SecretName: ref.Name,
				SecretKey:  ref.Key,
			})
		}
	}

	// settings.<key>: {secretKeyRef: {name, key}} or {name, key}
	if s, ok := m["settings"]; ok && s != nil {
		if sm, ok := s.(map[string]interface{}); ok {
			for key, val := range sm {
				if val == nil {
					continue
				}
				if ref := extractSecretRef(val); ref != nil {
					entries = append(entries, secretRefEntry{
						ProviderID: providerID,
						Field:      key,
						SecretName: ref.Name,
						SecretKey:  ref.Key,
					})
				}
			}
		}
	}

	return entries, nil
}

// extractSecretRef extracts name+key from a value that may be {name, key} or {secretKeyRef: {name, key}}.
func extractSecretRef(val interface{}) *v1alpha2.SecretKeyRef {
	var name, key string
	switch v := val.(type) {
	case map[string]interface{}:
		// Direct form: {name: x, key: y}
		if n, ok := v["name"].(string); ok && n != "" {
			name = n
		}
		if k, ok := v["key"].(string); ok && k != "" {
			key = k
		}
		if name != "" && key != "" {
			return &v1alpha2.SecretKeyRef{Name: name, Key: key}
		}
		// Nested form: {secretKeyRef: {name, key}}
		if skr, ok := v["secretKeyRef"].(map[string]interface{}); ok {
			if n, ok := skr["name"].(string); ok && n != "" {
				name = n
			}
			if k, ok := skr["key"].(string); ok && k != "" {
				key = k
			}
			if name != "" && key != "" {
				return &v1alpha2.SecretKeyRef{Name: name, Key: key}
			}
		}
	}
	return nil
}
