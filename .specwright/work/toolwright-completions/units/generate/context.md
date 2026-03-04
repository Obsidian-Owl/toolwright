# Context: Unit 3 — generate

## Purpose

Implement `internal/generate/` package — AI-assisted manifest generation with 3 LLM providers (Anthropic, OpenAI, Gemini). Also adds missing `--model` and `--no-merge` CLI flags to `generate_manifest.go`.

## Interface to Implement

From `internal/cli/generate_manifest.go`:

```go
type manifestGenerator interface {
    Generate(ctx context.Context, opts ManifestGenerateOptions) (*ManifestGenerateResult, error)
}

type ManifestGenerateOptions struct {
    Provider    string // "anthropic", "openai", "gemini"
    Description string
    OutputPath  string
    DryRun      bool
    // MUST ADD in this unit:
    Model       string // Override provider default model
    NoMerge     bool   // Fail if output file already exists
}

type ManifestGenerateResult struct {
    Manifest string // Generated YAML content
    Provider string // Provider used
}
```

## CLI Layer Changes

`internal/cli/generate_manifest.go` needs:
1. Add `Model` and `NoMerge` fields to `ManifestGenerateOptions`
2. Add `--model` flag: `cmd.Flags().String("model", "", "override provider default model")`
3. Add `--no-merge` flag: `cmd.Flags().Bool("no-merge", false, "fail if output file already exists")`
4. Read and pass these flags in `runGenerateManifest()`

## Provider Architecture

```go
// internal/generate/provider.go
type LLMProvider interface {
    Complete(ctx context.Context, prompt string, model string) (string, error)
    Name() string
    DefaultModel() string
}
```

Three implementations, all using raw `net/http`:

| Provider | Env Var | Auth | Endpoint | Default Model |
|----------|---------|------|----------|---------------|
| Anthropic | `ANTHROPIC_API_KEY` | `x-api-key` header + `anthropic-version: 2023-06-01` | `https://api.anthropic.com/v1/messages` | claude-sonnet-4-20250514 |
| OpenAI | `OPENAI_API_KEY` | `Authorization: Bearer` | `https://api.openai.com/v1/chat/completions` | gpt-4o |
| Gemini | `GEMINI_API_KEY` | `?key=` query param | `https://generativelanguage.googleapis.com/v1beta/models/{model}:generateContent` | gemini-2.0-flash |

## YAML Extraction Strategy

LLMs often wrap YAML in markdown code fences. Extraction order:
1. Look for ` ```yaml\n...\n``` ` — extract inner content
2. Look for ` ```\n...\n``` ` — extract inner content
3. Fall back to raw response text
4. Parse with `yaml.Unmarshal` into `manifest.Toolkit`
5. Validate with `manifest.Validate()`

## Retry Logic

Per spec §3.10: "retries once on failure." Two attempts maximum:
1. First call — if error or invalid YAML, retry
2. Second call — if error or invalid YAML, return error to user

## Security (Constitution Rules 23, 23a, 26)

- API keys read from `os.Getenv()` at call time — never stored in struct fields
- API keys never included in error messages
- `io.LimitReader` on HTTP response body (256KB limit)
- No key fields in any struct that flows to serialization
- `debugLog` must not log request headers (they contain API keys)

## Spec Requirements (docs/spec.md §3.10)

- Provider is required (`--provider` flag)
- `--model` overrides provider default
- `--output` defaults to `./toolwright.yaml`
- `--no-merge` fails if output file exists
- `--dry-run` prints to stdout instead of writing
- API key from env vars; keys never logged or written to output
- Single LLM call with manifest schema as context
- Validates response, retries once on failure

## Key Files

| File | Purpose |
|------|---------|
| `internal/cli/generate_manifest.go` | manifestGenerator interface, ManifestGenerateOptions, runGenerateManifest |
| `internal/cli/generate_manifest_test.go` | mockManifestGenerator, 41+ existing tests |
| `internal/manifest/types.go` | Toolkit type for YAML parsing |
| `internal/manifest/validate.go` | Validate() for generated manifest validation |

## Gotchas

1. Gemini uses query param auth, not headers — different pattern from Anthropic/OpenAI
2. Anthropic requires `anthropic-version` header alongside `x-api-key`
3. Each provider has different response envelope: Anthropic `content[].text`, OpenAI `choices[].message.content`, Gemini `candidates[].content.parts[].text`
4. `io.LimitReader` must be applied before `json.Decode` on HTTP response
5. Context carries timeout — pass to `http.NewRequestWithContext`
6. Existing `generate_manifest_test.go` tests mock the interface — adding fields to ManifestGenerateOptions must not break existing tests (new fields have zero values)
7. `validManifestProviders` in generate_manifest.go defines the valid provider set — case-sensitive
