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

import "fmt"

// DetectConfigVersion extracts the version from a base config map.
// The version field is expected at the top level as an integer.
// Accepts both int and float64 (JSON numbers decode as float64).
func DetectConfigVersion(config map[string]interface{}) (int, error) {
	if config == nil {
		return 0, fmt.Errorf("config map is nil")
	}
	raw, ok := config["version"]
	if !ok {
		return 0, fmt.Errorf("config missing required 'version' field")
	}
	switch v := raw.(type) {
	case int:
		return v, nil
	case int64:
		return int(v), nil
	case float64:
		return int(v), nil
	default:
		return 0, fmt.Errorf("invalid config version type: %T (expected int)", raw)
	}
}

// ValidateConfigVersion checks that the version is within the supported range
// [SupportedConfigVersionMin, SupportedConfigVersionMax].
func ValidateConfigVersion(version int) error {
	if version < SupportedConfigVersionMin || version > SupportedConfigVersionMax {
		return fmt.Errorf("config version %d is not supported; supported versions: %v",
			version, SupportedVersions())
	}
	return nil
}

// SupportedVersions returns the list of supported config schema versions.
func SupportedVersions() []int {
	versions := make([]int, 0, SupportedConfigVersionMax-SupportedConfigVersionMin+1)
	for v := SupportedConfigVersionMin; v <= SupportedConfigVersionMax; v++ {
		versions = append(versions, v)
	}
	return versions
}
