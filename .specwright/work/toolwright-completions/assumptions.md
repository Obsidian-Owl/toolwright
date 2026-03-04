# Assumptions: toolwright-completions

## RESOLVED

### A1: huh v0.8.0 accessible mode is testable with bytes.Buffer
- **Category**: integration
- **Status**: RESOLVED (reference verified)
- **Resolution**: huh's accessible mode uses `runAccessible(w io.Writer, r io.Reader)` for line-by-line prompts. Testable with `bytes.Buffer` input containing newline-delimited answers. Confirmed via huh source and huh_test.go patterns.

### A2: go:embed directive for recursive template directories
- **Category**: technical
- **Status**: RESOLVED (reference verified)
- **Resolution**: `//go:embed all:templates/init` embeds the directory recursively including dot-prefixed files. The `all:` prefix is needed only if dot-files must be included; otherwise `//go:embed templates/init` suffices for non-dotfile templates.

### A3: Scaffolded structure matches spec exactly
- **Category**: behavioral
- **Status**: RESOLVED (spec verified)
- **Resolution**: docs/spec.md §3.1 defines: toolwright.yaml, bin/hello, schemas/hello-output.json, tests/hello.test.yaml, README.md. AC-17 says `--runtime go` → "Go stub instead of shell stub". Resolution: structure stays the same; bin/hello content varies by runtime. Non-shell runtimes may add source files (e.g., src/hello/main.go for Go).

### A4: manifest.Validate() is sufficient for AI-generated manifest validation
- **Category**: data
- **Status**: RESOLVED (accepted)
- **Resolution**: manifest.Validate() checks structural rules on a parsed Toolkit. Unknown YAML fields are silently dropped by yaml.Unmarshal. For AI-generated manifests, this is acceptable — the validation catches structural errors. JSON Schema validation against the raw YAML would catch unknown fields but is optional enhancement, not a requirement for initial implementation.

## ACCEPTED

### A5: LLM provider APIs remain stable for raw HTTP integration
- **Category**: integration
- **Status**: ACCEPTED (risk acknowledged)
- **Risk**: Each provider (Anthropic, OpenAI, Gemini) has different API shapes. Without SDK dependencies, API changes break providers silently. Mitigation: each provider is isolated (~100 lines), version-pin API endpoints, add integration test markers for CI skip.

### A6: huh v0.8.0 will not have breaking changes before v1.0
- **Category**: integration
- **Status**: ACCEPTED (risk acknowledged)
- **Risk**: huh is pre-1.0 and could change API. Mitigation: single-file consumption, pin version in go.mod, wizard is a thin wrapper. Migration to charm.land/huh/v2 when it ships stable.

## NOT APPLICABLE

### A7: Template rendering with user input containing template delimiters
- **Category**: security
- **Status**: N/A (non-issue by design)
- **Resolution**: All template data is passed via typed struct fields. User input (project name, description) flows through Go struct fields, never through raw string concatenation. Static files (gitignore, etc.) are copied verbatim without template processing.
