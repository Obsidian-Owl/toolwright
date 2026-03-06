package generate

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/Obsidian-Owl/toolwright/internal/manifest"
)

// Generator dispatches manifest generation requests to a registered LLMProvider.
type Generator struct {
	providers map[string]LLMProvider
}

// NewGenerator creates a Generator with all 3 providers registered.
// API keys are read from environment variables at construction time and
// passed to provider constructors. Ensure env vars are set before calling.
func NewGenerator() *Generator {
	providers := map[string]LLMProvider{
		"anthropic": NewAnthropicProvider(os.Getenv("ANTHROPIC_API_KEY"), nil),
		"openai":    NewOpenAIProvider(os.Getenv("OPENAI_API_KEY"), nil),
		"gemini":    NewGeminiProvider(os.Getenv("GEMINI_API_KEY"), nil),
	}
	return &Generator{providers: providers}
}

// NewGeneratorWithProviders is for testing — inject mock providers.
func NewGeneratorWithProviders(providers map[string]LLMProvider) *Generator {
	return &Generator{providers: providers}
}

// Providers returns a copy of the registered provider map (used in tests).
func (g *Generator) Providers() map[string]LLMProvider {
	out := make(map[string]LLMProvider, len(g.providers))
	for k, v := range g.providers {
		out[k] = v
	}
	return out
}

// Generate produces a toolwright manifest from the given options.
// It attempts at most 2 calls to the provider; the second is a retry on error
// or invalid YAML.
func (g *Generator) Generate(ctx context.Context, opts ManifestGenerateOptions) (*ManifestGenerateResult, error) {
	provider, ok := g.providers[opts.Provider]
	if !ok {
		return nil, fmt.Errorf("generate: unknown provider %q", opts.Provider)
	}

	// NoMerge check: fail fast before any LLM call if the output file already exists.
	if opts.NoMerge && opts.OutputPath != "" {
		if _, err := os.Stat(opts.OutputPath); err == nil {
			return nil, fmt.Errorf("generate: output file already exists: %s", opts.OutputPath)
		}
	}

	model := opts.Model
	if model == "" {
		model = provider.DefaultModel()
	}

	prompt := buildPrompt(opts.Description)

	var (
		lastErr error
		yaml    string
	)

	for attempt := 0; attempt < 2; attempt++ {
		if attempt > 0 && ctx.Err() != nil {
			lastErr = ctx.Err()
			break
		}
		raw, err := provider.Complete(ctx, prompt, model)
		if err != nil {
			lastErr = fmt.Errorf("generate: provider call failed: %w", err)
			continue
		}

		extracted := extractYAML(raw)
		tk, err := manifest.Parse(strings.NewReader(extracted))
		if err != nil {
			lastErr = fmt.Errorf("generate: invalid manifest YAML: %w", err)
			continue
		}

		errs := manifest.Validate(tk)
		if len(errs) > 0 {
			lastErr = fmt.Errorf("generate: manifest validation failed: %v", errs[0].Message)
			continue
		}

		yaml = extracted
		lastErr = nil
		break
	}

	if lastErr != nil {
		return nil, lastErr
	}

	return &ManifestGenerateResult{
		Manifest: yaml,
		Provider: opts.Provider,
	}, nil
}
