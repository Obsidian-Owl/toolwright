package auth

import "encoding/json"

// mapKeys extracts the keys from a JSON object parsed as map[string]json.RawMessage.
// Shared across test files to inspect JSON structure.
func mapKeys(m map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
