package cluster

import (
	"encoding/json"
	"os"
	"testing"
)

// TestDistributionsJSONIsValid ensures that the distributions.json file always
// contains well-formed JSON and that all keys and values are non-empty.
func TestDistributionsJSONIsValid(t *testing.T) {
	data, err := os.ReadFile("../../distributions.json")
	if err != nil {
		t.Fatalf("failed to read distributions.json: %v", err)
	}

	var dist map[string]string
	if err := json.Unmarshal(data, &dist); err != nil {
		t.Fatalf("failed to validate distributions.json: %v", err)
	}

	for k, v := range dist {
		if k == "" {
			t.Fatalf("failed to validate distributions.json: contains an empty key")
		}
		if v == "" {
			t.Fatalf("failed to validate distributions.json: contains an empty value for key %q", k)
		}
	}
}
