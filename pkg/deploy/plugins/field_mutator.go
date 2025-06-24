package plugins

import (
	"fmt"
	"strings"

	"sigs.k8s.io/kustomize/api/resmap"
	"sigs.k8s.io/kustomize/api/resource"
	"sigs.k8s.io/yaml"
)

// FieldMapping defines a single field mapping.
type FieldMapping struct {
	// SourceValue is the value to copy to the target field.
	SourceValue any `json:"sourceValue"`
	// DefaultValue is the value to use if SourceValue is empty.
	// This provides a fallback mechanism, making transformations more robust.
	DefaultValue any `json:"defaultValue,omitempty"`
	// TargetField is the dot-notation path to the field in the target object.
	TargetField string `json:"targetField"`
	// TargetKind is the kind of resource to apply the transformation to.
	TargetKind string `json:"targetKind"`
	// CreateIfNotExists will create the target field and any intermediate
	// map structures if they don't exist in the target resource.
	CreateIfNotExists bool `json:"createIfNotExists,omitempty"`
}

// FieldMutatorConfig is a collection of FieldMappings.
type FieldMutatorConfig struct {
	// Mappings is a list of field mappings to apply.
	Mappings []FieldMapping `json:"mappings"`
}

// CreateFieldMutator creates a mutator plugin that sets a value for a given field.
func CreateFieldMutator(config FieldMutatorConfig) *fieldMutator {
	return &fieldMutator{config: config}
}

type fieldMutator struct {
	config FieldMutatorConfig
}

// isEmpty checks if a value is nil or an empty string, slice, or map.
func isEmpty(v any) bool {
	if v == nil {
		return true
	}

	switch val := v.(type) {
	case string:
		return val == ""
	case map[string]any:
		return len(val) == 0
	case []any:
		return len(val) == 0
	}

	return false
}

func (t *fieldMutator) Transform(m resmap.ResMap) error {
	for _, mapping := range t.config.Mappings {
		// Get the value to use, falling back to the default if the source is empty.
		value := mapping.SourceValue
		if isEmpty(value) {
			value = mapping.DefaultValue
		}

		// Skip this mapping if both source and default values are empty.
		if isEmpty(value) {
			continue
		}

		for _, res := range m.Resources() {
			if res.GetKind() != mapping.TargetKind {
				continue
			}

			if err := setTargetField(res, value, mapping); err != nil {
				return fmt.Errorf("failed to set target field for mapping %s: %w", mapping.TargetField, err)
			}
		}
	}

	return nil
}

func (t *fieldMutator) Config(h *resmap.PluginHelpers, _ []byte) error {
	return nil
}

// setTargetField modifies the resource by setting the specified value at the
// given dot-notation path.
func setTargetField(res *resource.Resource, value any, mapping FieldMapping) error {
	yamlBytes, err := res.AsYAML()
	if err != nil {
		return fmt.Errorf("failed to get YAML: %w", err)
	}

	var data map[string]any
	if unmarshalErr := yaml.Unmarshal(yamlBytes, &data); unmarshalErr != nil {
		return fmt.Errorf("failed to unmarshal YAML: %w", unmarshalErr)
	}

	// This loop navigates to the parent of the target field. We must stop one
	// level short because Golang does not allow taking a pointer to a map value
	// (e.g., `&myMap["key"]`). To mutate the map, we need a reference to the
	// parent container to use the `parent[key] = value` syntax.
	//
	// This "stop at the parent" approach also gives us the `CreateIfNotExists`
	// behavior for free, as we can create missing parent maps during traversal.
	fields := strings.Split(mapping.TargetField, ".")
	current := data
	for _, field := range fields[:len(fields)-1] {
		next, ok := current[field]
		if !ok {
			if !mapping.CreateIfNotExists {
				return fmt.Errorf("failed to find field %s", field)
			}
			next = make(map[string]any)
			current[field] = next
		}

		nextMap, ok := next.(map[string]any)
		if !ok {
			return fmt.Errorf("failed to convert field %s to map", field)
		}

		current = nextMap
	}

	lastField := fields[len(fields)-1]
	current[lastField] = value

	// After modifying the map, we must marshal it back to YAML and create a new
	// resource object to ensure the internal state is consistent.
	updatedYAML, err := yaml.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal updated YAML: %w", err)
	}

	rf := resource.NewFactory(nil)
	newRes, err := rf.FromBytes(updatedYAML)
	if err != nil {
		return fmt.Errorf("failed to create resource from updated YAML: %w", err)
	}

	// Atomically replace the old resource content with the new, fully updated content
	// to prevent partial updates or data loss.
	res.ResetRNode(newRes)
	return nil
}
