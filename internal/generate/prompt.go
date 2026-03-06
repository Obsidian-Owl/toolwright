package generate

import "fmt"

// buildPrompt constructs the system prompt sent to the LLM provider.
// The description is embedded literally so tests can verify it appears in the prompt.
func buildPrompt(description string) string {
	return fmt.Sprintf(`Generate a valid toolwright.yaml manifest for the following toolkit:

%s

Requirements:
- The manifest must be a valid toolwright/v1 YAML document
- Set apiVersion to "toolwright/v1" and kind to "Toolkit"
- Include metadata with name (lowercase alphanumeric and hyphens only), version (SemVer), and description
- Include at least one tool with name, description, and entrypoint fields
- Output ONLY the YAML, wrapped in a fenced code block

Output format:
`+"```yaml"+`
<manifest content here>
`+"```", description)
}
