package generate

import "strings"

// extractYAML extracts YAML content from an LLM response.
// It handles:
//   - ```yaml\n...\n``` fenced blocks (preferred)
//   - ```\n...\n``` generic fenced blocks
//   - Raw text (fallback)
func extractYAML(raw string) string {
	// Normalise Windows line endings so all logic works on \n.
	raw = strings.ReplaceAll(raw, "\r\n", "\n")

	// Try yaml/yml-tagged fence first, then generic fence.
	for _, opener := range []string{"```yaml\n", "```yml\n", "```\n"} {
		if content, ok := extractFenced(raw, opener); ok {
			return strings.TrimSpace(content)
		}
	}

	// Fall back to the raw text with whitespace trimmed.
	return strings.TrimSpace(raw)
}

// extractFenced locates the first occurrence of opener inside s and returns
// everything up to (but not including) the next closing ```, which must appear
// on its own line. It returns ("", false) when no such block is found.
func extractFenced(s, opener string) (string, bool) {
	start := strings.Index(s, opener)
	if start == -1 {
		return "", false
	}
	// Advance past the opener line.
	body := s[start+len(opener):]

	// The closing fence is ``` on its own line — it may be preceded by a
	// newline but must not be part of the content (e.g. nested backticks).
	// We scan line-by-line so that ``` embedded inside content is preserved.
	lines := strings.Split(body, "\n")
	var contentLines []string
	for _, line := range lines {
		if line == "```" {
			return strings.Join(contentLines, "\n"), true
		}
		contentLines = append(contentLines, line)
	}

	// Opener found but no closing fence — treat as not found.
	return "", false
}
