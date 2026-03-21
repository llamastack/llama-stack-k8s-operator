package config

import (
	"embed"
	"fmt"
)

//go:embed configs
var embeddedConfigs embed.FS

// ConfigResolver resolves a base config.yaml for a distribution.
type ConfigResolver interface {
	Resolve(distributionName string) ([]byte, error)
}

// EmbeddedConfigResolver resolves configs from embedded filesystem.
type EmbeddedConfigResolver struct {
	fs embed.FS
}

// NewEmbeddedConfigResolver creates a resolver using the operator's embedded configs.
func NewEmbeddedConfigResolver() *EmbeddedConfigResolver {
	return &EmbeddedConfigResolver{fs: embeddedConfigs}
}

// NewEmbeddedConfigResolverFromFS creates a resolver using a custom embedded FS (for testing).
func NewEmbeddedConfigResolverFromFS(fs embed.FS) *EmbeddedConfigResolver {
	return &EmbeddedConfigResolver{fs: fs}
}

// Resolve loads the base config.yaml for the given distribution name.
func (r *EmbeddedConfigResolver) Resolve(name string) ([]byte, error) {
	data, err := r.fs.ReadFile(fmt.Sprintf("configs/%s/config.yaml", name))
	if err != nil {
		return nil, fmt.Errorf("failed to load embedded config for distribution %q: %w", name, err)
	}
	return data, nil
}
