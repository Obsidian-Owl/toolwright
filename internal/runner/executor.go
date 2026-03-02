package runner

import (
	"fmt"

	"github.com/Obsidian-Owl/toolwright/internal/manifest"
)

// BuildArgs assembles the argument slice to pass to a tool's entrypoint.
// Order: positional args, then flags (in tool.Flags definition order), then token.
func BuildArgs(tool manifest.Tool, positionalArgs []string, flags map[string]string, token string) []string {
	result := make([]string, 0, len(positionalArgs))

	// 1. Positional args in order.
	result = append(result, positionalArgs...)

	// 2. Flags in tool.Flags slice order (deterministic, not map iteration order).
	for _, f := range tool.Flags {
		val, ok := flags[f.Name]
		if !ok || val == "" {
			continue
		}
		if f.Type == "bool" {
			if val == "true" {
				result = append(result, fmt.Sprintf("--%s", f.Name))
			}
			// val == "false" → skip entirely
			continue
		}
		result = append(result, fmt.Sprintf("--%s", f.Name), val)
	}

	// 3. Token last, using the exact TokenFlag string (already includes --).
	if tool.Auth != nil && tool.Auth.TokenFlag != "" && token != "" {
		result = append(result, tool.Auth.TokenFlag, token)
	}

	return result
}
