package generate

import "context"

// LLMProvider is the interface all LLM provider implementations must satisfy.
type LLMProvider interface {
	Complete(ctx context.Context, prompt string, model string) (string, error)
	Name() string
	DefaultModel() string
}
