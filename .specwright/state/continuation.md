# Continuation: generate unit — all tasks complete

**Work unit**: toolwright-completions / generate
**Last task**: task-6-cli (--model and --no-merge flags)
**Status**: All 6 tasks committed. Awaiting /sw-verify.

## Key files
- `internal/generate/provider.go` — LLMProvider interface
- `internal/generate/extract.go` — extractYAML
- `internal/generate/anthropic.go` / `openai.go` / `gemini.go` — 3 providers
- `internal/generate/generator.go` — Generator with retry
- `internal/generate/prompt.go` — buildPrompt
- `internal/cli/generate_manifest.go` — ManifestGenerator interface, --model/--no-merge flags

## Branch
`feat/generate` — 6 commits ahead of main

## Next
Push feat/generate and run /sw-verify
