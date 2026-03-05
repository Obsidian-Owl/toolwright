# Spec: Unit 3 — generate

## Acceptance Criteria

### AC-1: Generator implements manifestGenerator interface
- `generate.Generator` satisfies `cli.manifestGenerator` interface (compile-time check)
- `NewGenerator()` returns a `*Generator` with all 3 providers registered
- `Generate(ctx, opts)` returns `(*cli.ManifestGenerateResult, error)`

### AC-2: Provider abstraction with 3 implementations
- `LLMProvider` interface with `Complete(ctx, prompt, model)`, `Name()`, `DefaultModel()`
- `AnthropicProvider` sends POST to `api.anthropic.com/v1/messages` with `x-api-key` and `anthropic-version` headers
- `OpenAIProvider` sends POST to `api.openai.com/v1/chat/completions` with `Authorization: Bearer` header
- `GeminiProvider` sends POST to `generativelanguage.googleapis.com` with API key as query param
- Each provider reads API key from the correct env var at call time

### AC-3: API key missing returns clear error
- Anthropic without `ANTHROPIC_API_KEY` → error mentioning the env var name
- OpenAI without `OPENAI_API_KEY` → error mentioning the env var name
- Gemini without `GEMINI_API_KEY` → error mentioning the env var name
- Error messages never contain the actual API key value

### AC-4: API keys never leak
- API keys are not stored in struct fields that could be serialized
- Error messages from HTTP failures do not include request headers
- No API key appears in any output, log, or error message
- `debugLog` calls (if any) do not log request headers

### AC-5: YAML extraction handles LLM response formats
- Extracts YAML from ` ```yaml\n...\n``` ` fenced blocks
- Extracts YAML from generic ` ```\n...\n``` ` fenced blocks
- Falls back to raw response text when no fences present
- Handles leading/trailing whitespace in extracted YAML

### AC-6: Generated manifest is valid
- Result YAML parses into `manifest.Toolkit` without error
- Parsed toolkit passes `manifest.Validate()`
- Result includes `apiVersion: toolwright/v1` and `kind: Toolkit`

### AC-7: Retry once on failure
- If first LLM call returns error, retries exactly once
- If first call returns invalid YAML, retries exactly once
- If second call also fails, returns the error to caller
- Total attempts never exceed 2

### AC-8: --model flag overrides provider default
- `ManifestGenerateOptions.Model` is passed to `provider.Complete()`
- When Model is empty, provider uses `DefaultModel()`
- When Model is set, provider uses the specified model
- `--model` flag added to generate manifest CLI command

### AC-9: --no-merge flag prevents overwrite
- `ManifestGenerateOptions.NoMerge` is supported
- When NoMerge=true and output file exists, Generate() returns error
- Error message mentions the existing file path
- When NoMerge=false, existing files may be overwritten
- `--no-merge` flag added to generate manifest CLI command

### AC-10: Response body size is limited
- HTTP response body read through `io.LimitReader` (256KB limit)
- Responses exceeding the limit are truncated, not crash-inducing

### AC-11: Context timeout is respected
- Providers pass `ctx` to `http.NewRequestWithContext`
- A cancelled context aborts the HTTP request
- Timeout errors return a clear error message

### AC-12: HTTPClient injection for testability
- Each provider constructor accepts `*http.Client`
- Tests use `httptest.NewServer` to mock API responses
- No real HTTP calls in unit tests
