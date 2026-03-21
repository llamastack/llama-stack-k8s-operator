package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEmbeddedConfigResolver_Resolve(t *testing.T) {
	resolver := NewEmbeddedConfigResolver()

	tests := []struct {
		name        string
		distro      string
		wantErr     bool
		errContains string
	}{
		{
			name:   "starter distribution",
			distro: "starter",
		},
		{
			name:   "remote-vllm distribution",
			distro: "remote-vllm",
		},
		{
			name:   "meta-reference-gpu distribution",
			distro: "meta-reference-gpu",
		},
		{
			name:   "postgres-demo distribution",
			distro: "postgres-demo",
		},
		{
			name:        "unknown distribution returns error",
			distro:      "nonexistent",
			wantErr:     true,
			errContains: "failed to load embedded config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := resolver.Resolve(tt.distro)
			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.errContains)
				return
			}
			require.NoError(t, err)
			assert.NotEmpty(t, data)

			// Verify it's valid YAML with version field
			config, err := parseConfig(data)
			require.NoError(t, err)
			version, err := DetectConfigVersion(config)
			require.NoError(t, err)
			assert.Equal(t, SupportedConfigVersion, version)
		})
	}
}
