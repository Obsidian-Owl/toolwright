# Plan: Unit 3 — generate

## Tasks

### Task 1: Provider abstraction + YAML extraction

New package `internal/generate/`:

```go
// provider.go
type LLMProvider interface {
    Complete(ctx context.Context, prompt string, model string) (string, error)
    Name() string
    DefaultModel() string
}

// extract.go
func extractYAML(raw string) string  // fenced block extraction
```

### Task 2: Anthropic provider

```go
// anthropic.go
type AnthropicProvider struct {
    httpClient *http.Client
}

func NewAnthropicProvider(httpClient *http.Client) *AnthropicProvider
func (p *AnthropicProvider) Complete(ctx context.Context, prompt, model string) (string, error)
func (p *AnthropicProvider) Name() string        // "anthropic"
func (p *AnthropicProvider) DefaultModel() string // "claude-sonnet-4-20250514"
```

### Task 3: OpenAI provider

```go
// openai.go
type OpenAIProvider struct {
    httpClient *http.Client
}

func NewOpenAIProvider(httpClient *http.Client) *OpenAIProvider
func (p *OpenAIProvider) Complete(ctx context.Context, prompt, model string) (string, error)
func (p *OpenAIProvider) Name() string        // "openai"
func (p *OpenAIProvider) DefaultModel() string // "gpt-4o"
```

### Task 4: Gemini provider

```go
// gemini.go
type GeminiProvider struct {
    httpClient *http.Client
}

func NewGeminiProvider(httpClient *http.Client) *GeminiProvider
func (p *GeminiProvider) Complete(ctx context.Context, prompt, model string) (string, error)
func (p *GeminiProvider) Name() string        // "gemini"
func (p *GeminiProvider) DefaultModel() string // "gemini-2.0-flash"
```

### Task 5: Generator + prompt + retry

```go
// generator.go
type Generator struct {
    providers map[string]LLMProvider
}

func NewGenerator() *Generator  // registers all 3 providers
func (g *Generator) Generate(ctx context.Context, opts cli.ManifestGenerateOptions) (*cli.ManifestGenerateResult, error)

// prompt.go
func buildPrompt(description string) string  // system + user prompt for manifest generation
```

Generator.Generate():
1. Look up provider by name
2. Read API key from env, error if missing
3. Build prompt with description and manifest schema context
4. Call provider.Complete() — if error or invalid YAML, retry once
5. Extract YAML from response
6. Parse with yaml.Unmarshal, validate with manifest.Validate()
7. If --no-merge and output file exists, return error
8. Return ManifestGenerateResult

### Task 6: CLI flag additions

Update `internal/cli/generate_manifest.go`:
- Add `Model` and `NoMerge` fields to `ManifestGenerateOptions`
- Add `--model` and `--no-merge` flags to command
- Pass values in `runGenerateManifest()`

## File Change Map

| File | Action |
|------|--------|
| `internal/generate/provider.go` | CREATE — LLMProvider interface |
| `internal/generate/extract.go` | CREATE — YAML extraction from LLM response |
| `internal/generate/extract_test.go` | CREATE — extraction tests |
| `internal/generate/anthropic.go` | CREATE — Anthropic provider |
| `internal/generate/openai.go` | CREATE — OpenAI provider |
| `internal/generate/gemini.go` | CREATE — Gemini provider |
| `internal/generate/provider_test.go` | CREATE — provider tests (mock HTTP server) |
| `internal/generate/generator.go` | CREATE — Generator orchestration |
| `internal/generate/generator_test.go` | CREATE — generator tests |
| `internal/generate/prompt.go` | CREATE — prompt building |
| `internal/cli/generate_manifest.go` | EDIT — add Model, NoMerge fields + flags |

## Architecture Decisions

1. **HTTPClient injection**: Each provider accepts `*http.Client` for testability. Tests use `httptest.NewServer`.
2. **API key at call time**: Read from `os.Getenv()` inside `Complete()`. Never stored in struct.
3. **io.LimitReader**: Applied to all HTTP response bodies (256KB limit per Constitution rule 26).
4. **Provider map keyed by name**: Generator uses `providers[opts.Provider]` for O(1) lookup.
5. **Prompt includes manifest schema context**: The prompt describes the toolwright.yaml format so the LLM generates valid structure.
