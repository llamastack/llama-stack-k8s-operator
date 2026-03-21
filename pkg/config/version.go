package config

import (
	"errors"
	"fmt"
)

const (
	// SupportedConfigVersion is the config.yaml version this operator supports.
	SupportedConfigVersion = 2
)

// DetectConfigVersion reads the "version" field from parsed config.
// Returns the version number or an error if missing/invalid.
func DetectConfigVersion(config map[string]interface{}) (int, error) {
	v, ok := config["version"]
	if !ok {
		return 0, errors.New("config.yaml missing required 'version' field")
	}

	switch val := v.(type) {
	case int:
		return val, nil
	case float64:
		return int(val), nil
	case int64:
		return int(val), nil
	default:
		return 0, fmt.Errorf("failed to parse config.yaml version: unexpected type %T", v)
	}
}

// ValidateConfigVersion checks that the config version is supported.
func ValidateConfigVersion(version int) error {
	if version != SupportedConfigVersion {
		return fmt.Errorf("failed to validate config version: version %d is not supported (expected %d)", version, SupportedConfigVersion)
	}
	return nil
}
