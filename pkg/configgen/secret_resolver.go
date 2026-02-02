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

package configgen

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// SecretResolver validates and resolves secret references
type SecretResolver struct {
	client    client.Client
	namespace string
}

// NewSecretResolver creates a new SecretResolver
func NewSecretResolver(c client.Client, namespace string) *SecretResolver {
	return &SecretResolver{
		client:    c,
		namespace: namespace,
	}
}

// GenerateEnvVarName creates a deterministic environment variable name
// Format: LLSD_{PROVIDER_TYPE}_{PROVIDER_ID}_{FIELD}
func GenerateEnvVarName(providerType, providerID, field string) string {
	parts := []string{"LLSD"}

	if providerType != "" {
		parts = append(parts, strings.ToUpper(providerType))
	}

	if providerID != "" {
		// Convert hyphens to underscores for env var compatibility
		parts = append(parts, strings.ToUpper(strings.ReplaceAll(providerID, "-", "_")))
	}

	if field != "" {
		parts = append(parts, strings.ToUpper(field))
	}

	return strings.Join(parts, "_")
}

// ValidateSecretExists checks if a secret and key exist
func (r *SecretResolver) ValidateSecretExists(ctx context.Context, secretName, secretKey string) error {
	secret := &corev1.Secret{}
	if err := r.client.Get(ctx, types.NamespacedName{
		Name:      secretName,
		Namespace: r.namespace,
	}, secret); err != nil {
		return fmt.Errorf("failed to get secret '%s/%s'", r.namespace, secretName)
	}

	if _, ok := secret.Data[secretKey]; !ok {
		return fmt.Errorf("failed to find key '%s' in secret '%s/%s'", secretKey, r.namespace, secretName)
	}

	return nil
}

// ResolveSecretRef creates an EnvVar that references a secret
func ResolveSecretRef(envVarName string, secretRef corev1.SecretKeySelector) corev1.EnvVar {
	return corev1.EnvVar{
		Name: envVarName,
		ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &secretRef,
		},
	}
}

// GenerateEnvVars creates environment variable configurations for all secret references
func GenerateEnvVars(envVarName string, secretRef corev1.SecretKeySelector) corev1.EnvVar {
	return corev1.EnvVar{
		Name: envVarName,
		ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: &secretRef,
		},
	}
}
