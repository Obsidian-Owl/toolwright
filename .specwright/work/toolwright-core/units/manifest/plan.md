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

## As-Built Notes

### Plan deviations
1. **Tasks 2+3 merged**: Manifest types and parser implemented together (types.go contains both types and Parse/ParseFile). The tester wrote tests for both in `parser_test.go` since parsing requires types.
2. **Task 6 deferred**: `schemas/toolwright.schema.json` not created — the schema validator tests use inline `fstest.MapFS` schemas. The actual manifest JSON Schema will be created when needed by the CLI validate command (Unit 6).
3. **JSON Schema library**: Used `kaptinlin/jsonschema` instead of spec's `santhosh-tekuri/jsonschema/v6`. The kaptinlin library had a simpler API for the test patterns. All AC-9/AC-10 criteria are met. Can be swapped if needed.
4. **Example.MarshalYAML**: Added a custom `MarshalYAML` method on `Example` to handle round-trip tests (AC-11) — preserves non-nil empty slices correctly.

### Actual files
| File | Lines | Tests |
|------|-------|-------|
| `internal/manifest/types.go` | ~200 | 39 tests in parser_test.go |
| `internal/manifest/validate.go` | ~280 | 55+ tests in validate_test.go |
| `internal/schema/validator.go` | ~60 | 19 tests in validator_test.go |
| `embed.go` | 8 | — |
| `cmd/toolwright/main.go` | 4 | — |

### Test summary
- Total: 113+ tests across manifest and schema packages
- All pass, `go vet` clean, `go build` clean
