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
	_ "embed"
)

// Embedded distribution configuration templates
// These serve as base configurations that are merged with user-provided overrides

//go:embed configs/starter.yaml
var starterConfigYAML []byte

// DistributionConfigs maps distribution names to their embedded config.yaml templates
var DistributionConfigs = map[string][]byte{
	"starter": starterConfigYAML,
}

// GetDistributionConfig returns the embedded config for a distribution
// Returns nil if the distribution is not found
func GetDistributionConfig(name string) []byte {
	config, ok := DistributionConfigs[name]
	if !ok {
		return nil
	}
	return config
}

// HasDistributionConfig checks if a distribution config exists
func HasDistributionConfig(name string) bool {
	_, ok := DistributionConfigs[name]
	return ok
}

// ListDistributions returns all available distribution names
func ListDistributions() []string {
	names := make([]string, 0, len(DistributionConfigs))
	for name := range DistributionConfigs {
		names = append(names, name)
	}
	return names
}
