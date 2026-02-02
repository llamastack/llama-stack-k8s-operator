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
	"fmt"
	"strings"
)

// SupportedProviders is the list of valid provider types
var SupportedProviders = map[string]bool{
	"vllm":        true,
	"ollama":      true,
	"openai":      true,
	"anthropic":   true,
	"bedrock":     true,
	"azure":       true,
	"gemini":      true,
	"together":    true,
	"fireworks":   true,
	"groq":        true,
	"nvidia":      true,
	"llama-guard": true,
	"pgvector":    true,
}

// ValidateProviderType checks if a provider type is supported
func ValidateProviderType(provider string) error {
	if _, ok := SupportedProviders[provider]; !ok {
		supported := make([]string, 0, len(SupportedProviders))
		for p := range SupportedProviders {
			supported = append(supported, p)
		}
		return fmt.Errorf("failed to validate provider type '%s': must be one of: %s", provider, strings.Join(supported, ", "))
	}
	return nil
}

// MapProviderType converts a CRD provider value to config.yaml provider_type
// Adds the "remote::" prefix for all remote providers
func MapProviderType(provider string) string {
	return fmt.Sprintf("remote::%s", provider)
}

// MapEndpoint converts an endpoint URL to the config.yaml format
// In config.yaml, endpoint is stored as config.url
func MapEndpoint(endpoint string) string {
	return endpoint
}
