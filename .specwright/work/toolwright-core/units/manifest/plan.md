# Plan: Unit 1 — manifest

## Task Breakdown

### Task 1: Project scaffold
- `go mod init github.com/Obsidian-Owl/toolwright`
- Create directory structure: `cmd/toolwright/`, `internal/manifest/`, `internal/schema/`, `schemas/`, `templates/init/`
- Create stub `cmd/toolwright/main.go` (just `package main` + `func main()`)
- Create `embed.go` with `//go:embed schemas/*`
- `go mod tidy`

### Task 2: Manifest types
- File: `internal/manifest/types.go`
- Types: `Toolkit`, `Metadata`, `Tool`, `Auth`, `Endpoints`, `Arg`, `Flag`, `Output`, `Example`, `Generate`, `CLIConfig`, `MCPConfig`
- `Auth.UnmarshalYAML` — handle string shorthand (`none`) and object form
- `Toolkit.ResolvedAuth(tool Tool) Auth` — inheritance with fallback

### Task 3: Manifest parser
- File: `internal/manifest/parser.go`
- `Parse(r io.Reader) (*Toolkit, error)` — YAML decode
- `ParseFile(path string) (*Toolkit, error)` — convenience wrapper

### Task 4: Manifest validation
- File: `internal/manifest/validate.go`
- `Validate(t *Toolkit) []ValidationError`
- Rules from §2.8:
  1. `metadata.name` matches `^[a-z0-9-]+$`
  2. `metadata.version` is valid SemVer
  3. `metadata.description` non-empty, under 200 chars
  4. Each `tools[].name` is unique
  5. Arg/flag names unique within their tool
  6. `enum` values match declared `type`
  7. `default` value matches declared `type`
  8. For `auth.type: token`: `token_env` and `token_flag` required
  9. For `auth.type: oauth2`: `provider_url`, `scopes`, `token_env`, `token_flag` required
  10. `provider_url` is a valid HTTPS URL

- `ValidationError` type: `Path string`, `Message string`, `Rule string`
- Note: entrypoint existence is NOT checked here (that's a CLI-level concern using the filesystem)

### Task 5: JSON Schema validator
- File: `internal/schema/validator.go`
- `Validator` struct wrapping `jsonschema/v6`
- `NewValidator(schemaFS embed.FS) *Validator`
- `Validate(schemaPath string, data []byte) error`

### Task 6: Manifest JSON Schema
- File: `schemas/toolwright.schema.json`
- JSON Schema (draft 2020-12) for `toolwright.yaml`
- Covers: apiVersion, kind, metadata, tools, auth, generate

## File Change Map

| File | Action | Package |
|------|--------|---------|
| `go.mod` | Create | root |
| `cmd/toolwright/main.go` | Create | main (stub) |
| `embed.go` | Create | root |
| `internal/manifest/types.go` | Create | manifest |
| `internal/manifest/parser.go` | Create | manifest |
| `internal/manifest/validate.go` | Create | manifest |
| `internal/manifest/parser_test.go` | Create | manifest |
| `internal/manifest/validate_test.go` | Create | manifest |
| `internal/schema/validator.go` | Create | schema |
| `internal/schema/validator_test.go` | Create | schema |
| `schemas/toolwright.schema.json` | Create | embedded asset |
