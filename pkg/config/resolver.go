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
	"context"
	"embed"
	"fmt"

	"github.com/go-logr/logr"
	v1alpha2 "github.com/llamastack/llama-stack-k8s-operator/api/v1alpha2"
	"gopkg.in/yaml.v3"
)

//go:embed configs
var embeddedConfigs embed.FS

// BaseConfigResolver resolves the base config.yaml for a distribution.
// Phase 1 uses embedded configs; Phase 2 will add OCI label extraction.
type BaseConfigResolver struct {
	distributionImages map[string]string
	imageOverrides     map[string]string
}

// NewBaseConfigResolver creates a resolver with distribution image mappings.
func NewBaseConfigResolver(distImages, overrides map[string]string) *BaseConfigResolver {
	return &BaseConfigResolver{
		distributionImages: distImages,
		imageOverrides:     overrides,
	}
}

// Resolve returns the base config and resolved image for the given distribution spec.
// Resolution priority:
// 1. (Phase 2) OCI labels on resolved image
// 2. Embedded config for distribution.name
// 3. Error requiring overrideConfig
func (r *BaseConfigResolver) Resolve(ctx context.Context, dist v1alpha2.DistributionSpec) (*BaseConfig, string, error) {
	log := logr.FromContextOrDiscard(ctx)

	image, err := r.resolveImage(dist)
	if err != nil {
		return nil, "", err
	}

	// Phase 1: Use embedded config for named distributions
	if dist.Name != "" {
		config, err := r.loadEmbeddedConfig(dist.Name)
		if err != nil {
			return nil, "", fmt.Errorf("failed to load embedded config for distribution %q: %w", dist.Name, err)
		}
		log.V(1).Info("resolved distribution config", "name", dist.Name, "image", image, "source", "embedded")
		return config, image, nil
	}

	// distribution.image without embedded config
	return nil, "", fmt.Errorf(
		"direct image references require either overrideConfig.configMapName or OCI config labels on the image; " +
			"see docs/configuration.md for details",
	)
}

// loadEmbeddedConfig reads and parses the embedded config.yaml for a named distribution.
func (r *BaseConfigResolver) loadEmbeddedConfig(name string) (*BaseConfig, error) {
	data, err := embeddedConfigs.ReadFile(fmt.Sprintf("configs/%s/config.yaml", name))
	if err != nil {
		return nil, fmt.Errorf("no embedded config for distribution %q: %w", name, err)
	}

	var config BaseConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("invalid embedded config for distribution %q: %w", name, err)
	}

	return &config, nil
}

// resolveImage determines the concrete container image for a distribution spec.
func (r *BaseConfigResolver) resolveImage(dist v1alpha2.DistributionSpec) (string, error) {
	if dist.Image != "" {
		return dist.Image, nil
	}

	if override, ok := r.imageOverrides[dist.Name]; ok {
		return override, nil
	}

	if image, ok := r.distributionImages[dist.Name]; ok {
		return image, nil
	}

	return "", fmt.Errorf("unknown distribution name %q: not found in distributions.json", dist.Name)
}

// EmbeddedDistributionNames returns the list of distribution names that have embedded configs.
func EmbeddedDistributionNames() ([]string, error) {
	entries, err := embeddedConfigs.ReadDir("configs")
	if err != nil {
		return nil, fmt.Errorf("failed to read embedded configs directory: %w", err)
	}

	var names []string
	for _, entry := range entries {
		if entry.IsDir() {
			names = append(names, entry.Name())
		}
	}
	return names, nil
}
