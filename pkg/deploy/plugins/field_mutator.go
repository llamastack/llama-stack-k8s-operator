package plugins

import (
	"fmt"
	"regexp"
	"strconv"
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
	// Supports array indices like "spec.ports[0].port"
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

type fieldSegment struct {
	name     string // field name (e.g., "ports")
	isArray  bool   // true if this segment has an array index
	index    int    // array index if isArray is true
	original string // original segment string for error messages
}

func parseFieldPath(path string) ([]fieldSegment, error) {
	// Regular expression to match field names with optional array indices
	// Matches: "fieldname" or "fieldname[123]"
	re := regexp.MustCompile(`^([a-zA-Z_][a-zA-Z0-9_]*)\[(\d+)\]$`)

	parts := strings.Split(path, ".")
	segments := make([]fieldSegment, len(parts))

	for i, part := range parts {
		if matches := re.FindStringSubmatch(part); matches != nil {
			index, err := strconv.Atoi(matches[2])
			if err != nil {
				return nil, fmt.Errorf("failed to parse array index in field path segment %q: %w", part, err)
			}
			segments[i] = fieldSegment{
				name:     matches[1],
				isArray:  true,
				index:    index,
				original: part,
			}
		} else {
			segments[i] = fieldSegment{
				name:     part,
				isArray:  false,
				index:    -1,
				original: part,
			}
		}
	}

	return segments, nil
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
// given dot-notation path with support for array indices.
func setTargetField(res *resource.Resource, value any, mapping FieldMapping) error {
	yamlBytes, err := res.AsYAML()
	if err != nil {
		return fmt.Errorf("failed to get YAML: %w", err)
	}

	var data map[string]any
	if unmarshalErr := yaml.Unmarshal(yamlBytes, &data); unmarshalErr != nil {
		return fmt.Errorf("failed to unmarshal YAML: %w", unmarshalErr)
	}

	segments, err := parseFieldPath(mapping.TargetField)
	if err != nil {
		return fmt.Errorf("failed to parse field path %q: %w", mapping.TargetField, err)
	}

	// Navigate to parent first, then set value - arrays vs maps need different handling.
	current, err := navigateToParent(data, segments, mapping.CreateIfNotExists)
	if err != nil {
		return err
	}

	lastSegment := segments[len(segments)-1]
	if setErr := setFieldValue(current, lastSegment, value, mapping.CreateIfNotExists); setErr != nil {
		return fmt.Errorf("failed to set field %q: %w", lastSegment.original, setErr)
	}

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

// navigateToParent walks through all field segments except the last one,
// returning the parent container where the final field should be set.
func navigateToParent(data map[string]any, segments []fieldSegment, createIfNotExists bool) (map[string]any, error) {
	current := data
	// This loop navigates to the parent of the target field. We must stop one
	// level short because Golang does not allow taking a pointer to a map value
	// (e.g., `&myMap["key"]`). To mutate the map, we need a reference to the
	// parent container to use the `parent[key] = value` syntax.
	//
	// This "stop at the parent" approach also gives us the `CreateIfNotExists`
	// behavior for free, as we can create missing parent maps during traversal.
	for _, segment := range segments[:len(segments)-1] {
		next, navErr := navigateToField(current, segment, createIfNotExists)
		if navErr != nil {
			if strings.Contains(navErr.Error(), "failed to find field") {
				return nil, navErr
			}
			return nil, fmt.Errorf("failed to navigate to field %q: %w", segment.original, navErr)
		}
		currentMap, ok := next.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("failed to convert field %s to map", segment.name)
		}
		current = currentMap
	}
	return current, nil
}

// navigateToField returns the value at the specified field, creating it if needed.
func navigateToField(current any, segment fieldSegment, createIfNotExists bool) (any, error) {
	currentMap, ok := current.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("failed to convert current value to map[string]any, got %T", current)
	}

	next, exists := currentMap[segment.name]
	if !exists {
		if !createIfNotExists {
			return nil, fmt.Errorf("failed to find field %s", segment.name)
		}

		if segment.isArray {
			next = make([]any, segment.index+1)
		} else {
			next = make(map[string]any)
		}
		currentMap[segment.name] = next
	}

	if segment.isArray {
		return handleArrayAccess(currentMap, segment, next, createIfNotExists)
	}

	return next, nil
}

// handleArrayAccess navigates to a specific array index, expanding the array and
// creating missing elements if needed. Returns the element at the specified index.
func handleArrayAccess(currentMap map[string]any, segment fieldSegment, next any, createIfNotExists bool) (any, error) {
	arr, err := ensureArrayWithCapacity(currentMap, segment, next, createIfNotExists)
	if err != nil {
		return nil, err
	}

	if arr[segment.index] == nil {
		if !createIfNotExists {
			return nil, fmt.Errorf("failed to access array element at index %d for field %q", segment.index, segment.name)
		}
		arr[segment.index] = make(map[string]any)
	}

	return arr[segment.index], nil
}

// ensureArrayWithCapacity ensures an array exists at the specified field with sufficient capacity.
// If the array doesn't exist or is too small, it creates or expands it as needed.
func ensureArrayWithCapacity(currentMap map[string]any, segment fieldSegment, field any, createIfNotExists bool) ([]any, error) {
	if field == nil {
		if !createIfNotExists {
			return nil, fmt.Errorf("failed to find array field %q", segment.name)
		}
		arr := make([]any, segment.index+1)
		currentMap[segment.name] = arr
		return arr, nil
	}

	arr, ok := field.([]any)
	if !ok {
		return nil, fmt.Errorf("failed to convert field %q to array, got %T", segment.name, field)
	}

	if segment.index >= len(arr) {
		if !createIfNotExists {
			return nil, fmt.Errorf("failed to access array index %d for field %q (length: %d)", segment.index, segment.name, len(arr))
		}
		newArr := make([]any, segment.index+1)
		copy(newArr, arr)
		for i := len(arr); i <= segment.index; i++ {
			newArr[i] = make(map[string]any)
		}
		currentMap[segment.name] = newArr
		return newArr, nil
	}

	return arr, nil
}

func setFieldValue(current any, segment fieldSegment, value any, createIfNotExists bool) error {
	currentMap, ok := current.(map[string]any)
	if !ok {
		return fmt.Errorf("failed to convert current value to map[string]any, got %T", current)
	}

	if segment.isArray {
		field := currentMap[segment.name]
		arr, err := ensureArrayWithCapacity(currentMap, segment, field, createIfNotExists)
		if err != nil {
			return err
		}
		arr[segment.index] = value
		return nil
	}

	currentMap[segment.name] = value
	return nil
}
