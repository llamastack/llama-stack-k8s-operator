package plugins

import (
	"encoding/json"
	"fmt"

	"github.com/go-openapi/jsonpointer"
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
	// TargetField is the JSON Pointer path to the field in the target object.
	// Uses RFC 6901 JSON Pointer syntax like "/spec/ports/0/port"
	// Special characters in field names are escaped: ~0 for ~ and ~1 for /
	// Example: "/spec/selector/app.kubernetes.io~1instance"
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
// given JSON Pointer path.
func setTargetField(res *resource.Resource, value any, mapping FieldMapping) error {
	yamlBytes, err := res.AsYAML()
	if err != nil {
		return fmt.Errorf("failed to get YAML: %w", err)
	}

	var data any
	if unmarshalErr := yaml.Unmarshal(yamlBytes, &data); unmarshalErr != nil {
		return fmt.Errorf("failed to unmarshal YAML: %w", unmarshalErr)
	}

	ptr, err := jsonpointer.New(mapping.TargetField)
	if err != nil {
		return fmt.Errorf("failed to parse JSON Pointer path %q: %w", mapping.TargetField, err)
	}

	var updatedData any
	if mapping.CreateIfNotExists {
		updatedData, err = setWithPathCreation(data, ptr, value)
	} else {
		updatedData, err = ptr.Set(data, value)
	}

	if err != nil {
		return fmt.Errorf("failed to set field at path %q: %w", mapping.TargetField, err)
	}

	return updateResource(res, updatedData)
}

func setWithPathCreation(data any, ptr jsonpointer.Pointer, value any) (any, error) {
	// use manual path traversal to avoid jsonpointer panics on nil intermediate values
	tokens := ptr.DecodedTokens()
	if len(tokens) == 0 {
		return value, nil
	}

	// start with a copy of the original data to avoid mutations on errors
	result := deepCopyData(data)

	// build path step by step using recursive helper
	return setValueAtPath(result, tokens, value)
}

// setValueAtPath recursively sets a value at the given path, creating intermediate structures as needed.
func setValueAtPath(data any, tokens []string, value any) (any, error) {
	if len(tokens) == 0 {
		return value, nil
	}

	if len(tokens) == 1 {
		return setFinalValue(data, tokens[0], value)
	}

	return setIntermediateValue(data, tokens, value)
}

// setFinalValue sets the final value in the path (last token).
func setFinalValue(data any, token string, value any) (any, error) {
	if isNumericString(token) {
		return setArrayValue(data, token, value)
	}
	return setMapValue(data, token, value)
}

// setArrayValue sets a value at an array index.
func setArrayValue(data any, token string, value any) (any, error) {
	arr, ok := data.([]any)
	if !ok {
		return nil, fmt.Errorf("failed to access array index %q: expected array", token)
	}
	index := parseNumericToken(token)
	// extend array if necessary
	for len(arr) <= index {
		arr = append(arr, nil)
	}
	arr[index] = value
	return arr, nil
}

// setMapValue sets a value at a map key.
func setMapValue(data any, token string, value any) (any, error) {
	m, ok := data.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("failed to access map key %q: expected map", token)
	}
	m[token] = value
	return m, nil
}

// setIntermediateValue handles intermediate tokens in the path.
func setIntermediateValue(data any, tokens []string, value any) (any, error) {
	token := tokens[0]
	if isNumericString(token) {
		return setIntermediateArrayValue(data, tokens, value)
	}
	return setIntermediateMapValue(data, tokens, value)
}

// setIntermediateArrayValue handles array traversal for intermediate tokens.
func setIntermediateArrayValue(data any, tokens []string, value any) (any, error) {
	arr, ok := data.([]any)
	if !ok {
		return nil, fmt.Errorf("failed to access array index %q: expected array", tokens[0])
	}
	index := parseNumericToken(tokens[0])
	// extend array if necessary
	for len(arr) <= index {
		arr = append(arr, nil)
	}

	// get or create next container
	next := arr[index]
	if next == nil {
		next = createContainer(tokens[1])
	}

	// recursively set the rest of the path
	updated, err := setValueAtPath(next, tokens[1:], value)
	if err != nil {
		return nil, err
	}
	arr[index] = updated
	return arr, nil
}

// setIntermediateMapValue handles map traversal for intermediate tokens.
func setIntermediateMapValue(data any, tokens []string, value any) (any, error) {
	m, ok := data.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("failed to access map key %q: expected map", tokens[0])
	}

	// get or create next container
	next, exists := m[tokens[0]]
	if !exists || next == nil {
		next = createContainer(tokens[1])
	}

	// recursively set the rest of the path
	updated, err := setValueAtPath(next, tokens[1:], value)
	if err != nil {
		return nil, err
	}
	m[tokens[0]] = updated
	return m, nil
}

// createContainer creates an appropriate container based on the next token.
func createContainer(nextToken string) any {
	if isNumericString(nextToken) {
		return make([]any, 0)
	}
	return make(map[string]any)
}

// parseNumericToken converts a numeric string to int, assuming it's already validated.
func parseNumericToken(s string) int {
	result := 0
	for _, r := range s {
		result = result*10 + int(r-'0')
	}
	return result
}

// deepCopyData creates a deep copy of the data structure using JSON marshal/unmarshal.
func deepCopyData(data any) any {
	// Use JSON marshal/unmarshal for deep copy - simple and reliable
	jsonBytes, err := json.Marshal(data)
	if err != nil {
		return data // fallback to original if copy fails
	}

	var result any
	if err := json.Unmarshal(jsonBytes, &result); err != nil {
		return data // fallback to original if copy fails
	}

	return result
}

func updateResource(res *resource.Resource, updatedData any) error {
	// After modifying the map, we must marshal it back to YAML and create a new
	// resource object to ensure the internal state is consistent.
	updatedYAML, err := yaml.Marshal(updatedData)
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

// isNumericString checks if a string represents a valid non-negative integer.
func isNumericString(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
