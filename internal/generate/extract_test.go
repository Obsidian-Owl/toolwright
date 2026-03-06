package generate

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

// ---------------------------------------------------------------------------
// Compile-time interface check: LLMProvider must exist with the correct shape.
// This fails to compile until provider.go defines the interface.
// ---------------------------------------------------------------------------

var _ LLMProvider = (*llmProviderCompileCheck)(nil)

type llmProviderCompileCheck struct{}

func (c *llmProviderCompileCheck) Complete(_ context.Context, _ string, _ string) (string, error) {
	return "", nil
}

func (c *llmProviderCompileCheck) Name() string         { return "" }
func (c *llmProviderCompileCheck) DefaultModel() string { return "" }

// ---------------------------------------------------------------------------
// extractYAML tests — table-driven per constitution rule 9.
// ---------------------------------------------------------------------------

func TestExtractYAML(t *testing.T) {
	tests := []struct {
		name string
		raw  string
		want string
	}{
		{
			name: "yaml fenced block",
			raw:  "```yaml\nkey: value\n```",
			want: "key: value",
		},
		{
			name: "generic fenced block",
			raw:  "```\nkey: value\n```",
			want: "key: value",
		},
		{
			name: "no fences falls back to raw text",
			raw:  "key: value",
			want: "key: value",
		},
		{
			name: "whitespace inside yaml fence is trimmed",
			raw:  "```yaml\n  key: value  \n```",
			want: "key: value",
		},
		{
			name: "empty string returns empty",
			raw:  "",
			want: "",
		},
		{
			name: "leading and trailing whitespace outside fence",
			raw:  "  \n```yaml\nkey: value\n```\n  ",
			want: "key: value",
		},
		{
			name: "raw text with leading and trailing whitespace trimmed",
			raw:  "  \n  key: value  \n  ",
			want: "key: value",
		},
		{
			name: "multiple fence blocks extracts first one only",
			raw:  "```yaml\nfirst: block\n```\nsome text\n```yaml\nsecond: block\n```",
			want: "first: block",
		},
		{
			name: "multiline yaml inside fence",
			raw:  "```yaml\nname: test\nversion: 1\nitems:\n  - a\n  - b\n```",
			want: "name: test\nversion: 1\nitems:\n  - a\n  - b",
		},
		{
			name: "fence with language tag yml also works",
			raw:  "```yml\nkey: value\n```",
			want: "key: value",
		},
		{
			name: "prose before and after fence is ignored",
			raw:  "Here is the YAML:\n```yaml\nkey: value\n```\nHope that helps!",
			want: "key: value",
		},
		{
			name: "generic fence preferred over raw when present",
			raw:  "preamble: ignored\n```\ninner: extracted\n```\npostamble: ignored",
			want: "inner: extracted",
		},
		{
			name: "fence with extra whitespace lines inside",
			raw:  "```yaml\n\n  key: value\n\n```",
			want: "key: value",
		},
		{
			name: "only backticks no content returns empty",
			raw:  "```yaml\n```",
			want: "",
		},
		{
			name: "windows line endings handled",
			raw:  "```yaml\r\nkey: value\r\n```",
			want: "key: value",
		},
		{
			name: "nested backticks inside content preserved",
			raw:  "```yaml\ncommand: |\n  echo ```hello```\n```",
			want: "command: |\n  echo ```hello```",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractYAML(tt.raw)
			assert.Equal(t, tt.want, got)
		})
	}
}
