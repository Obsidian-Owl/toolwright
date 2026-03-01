# Context: Unit 1 — manifest

## Purpose

Bootstrap the Go project and implement the two leaf packages: `internal/manifest` (YAML parsing, types, validation) and `internal/schema` (JSON Schema validation). This unit creates the project foundation that all other units depend on.

## Project State

Greenfield. No code exists. This unit creates `go.mod`, the directory structure, `embed.go`, and the first two packages.

## Key Spec Sections

- §2.2: Manifest YAML structure (full example)
- §2.3: Type system (string, int, float, bool)
- §2.4: Auth configuration (none, token, oauth2) with string shorthand
- §2.8: Validation rules (10 rules)
- §4.1: Module layout
- §4.2: Core types (Go struct definitions)

## Files to Create

```
toolwright/
├── cmd/toolwright/main.go           # Stub (placeholder for unit 6)
├── internal/
│   ├── manifest/
│   │   ├── types.go                 # Toolkit, Tool, Auth, Arg, Flag, etc.
│   │   ├── parser.go                # YAML parsing with Auth.UnmarshalYAML
│   │   ├── validate.go              # Validation rules from §2.8
│   │   ├── parser_test.go
│   │   └── validate_test.go
│   └── schema/
│       ├── validator.go             # JSON Schema validation via jsonschema/v6
│       └── validator_test.go
├── schemas/toolwright.schema.json   # Manifest JSON Schema
├── embed.go                         # //go:embed schemas/*
├── go.mod
└── go.sum
```

## Gotchas

1. **Auth UnmarshalYAML**: `auth: none` (string) vs `auth: {type: oauth2, ...}` (object). Requires custom `UnmarshalYAML` that checks the YAML node kind.
2. **Flag.Required**: The `Flag` struct has a `Required` field (added during spec review). Don't forget it.
3. **Validation rule: entrypoint exists** — `toolwright validate` checks this, but the manifest parser should NOT check file existence (that's a validation concern, not a parsing concern). Separate parse from validate.
4. **Module path**: `github.com/Obsidian-Owl/toolwright`
5. **YAML library**: Use `go.yaml.in/yaml/v3` (not the deprecated `gopkg.in/yaml.v3`)
6. **go-cmp for struct comparison** in tests, testify for assertions

## Dependencies on Other Units

None. This is the foundation.

## What Other Units Expect From This One

- All units import `internal/manifest` for types (`Toolkit`, `Tool`, `Auth`, `Arg`, `Flag`, etc.)
- `internal/schema` is used by units 4 (testing) and 6 (cli/validate)
- `embed.go` provides `Schemas embed.FS` for JSON Schema files
