# Plan: Unit 5 — codegen

## Task Breakdown

### Task 1: Generator interface and engine
- File: `internal/codegen/engine.go`
- `Generator` interface: `Generate(ctx, data TemplateData, outputDir string) error`, `Mode() string`, `Target() string`
- `Engine` struct: registry of generators
- `Register(g Generator)`, `Generate(ctx, mode, target string, manifest Toolkit, outputDir string) error`
- `TemplateData` struct: `Manifest`, `Tool`, `Auth`, `Timestamp`, `Version`
- `.toolwright-generated` marker file: version, timestamp, mode, target
- Warn if marker exists and `--force` not set → return error

### Task 2: Go CLI generator
- File: `internal/codegen/cli_go.go`
- Implements `Generator` interface (Mode: "cli", Target: "go")
- Uses compiled quicktemplates from `templates/cli/golang/`
- Generates: main.go, root.go, per-tool command files, go.mod, Makefile, README
- Conditional: login.go only if any tool uses oauth2
- Conditional: resolver.go only if any tool uses token or oauth2
- Type mapping: manifest types → Go types

### Task 3: Go CLI quicktemplates
- Files: `templates/cli/golang/*.qtpl`
- `main.qtpl` — entry point with root command
- `root.qtpl` — root Cobra command with list, describe subcommands
- `command.qtpl` — per-tool subcommand with arg/flag mapping
- `login.qtpl` — OAuth PKCE login subcommand
- `resolver.qtpl` — token resolution (flag → env → keyring)
- `gomod.qtpl` — go.mod with dependencies
- `makefile.qtpl` — build and install targets
- Run `qtc` to generate `.qtpl.go` files, commit both

### Task 4: TypeScript MCP generator
- File: `internal/codegen/mcp_typescript.go`
- Implements `Generator` interface (Mode: "mcp", Target: "typescript")
- Uses compiled quicktemplates from `templates/mcp/typescript/`
- Generates: index.ts, per-tool handlers, search.ts, package.json, tsconfig.json, README
- Conditional: middleware.ts only if any tool uses token or oauth2
- Conditional: metadata.ts only if any tool uses oauth2 AND transport includes streamable-http
- Transport handling: stdio vs streamable-http detection in index.ts

### Task 5: TypeScript MCP quicktemplates
- Files: `templates/mcp/typescript/*.qtpl`
- `index.qtpl` — server entry point, transport detection
- `tool.qtpl` — per-tool handler (calls entrypoint via child_process)
- `search.qtpl` — search_tools meta-tool for progressive discovery
- `middleware.qtpl` — Authorization header validation
- `metadata.qtpl` — PRM endpoint (/.well-known/oauth-protected-resource)
- `package.qtpl` — package.json with @modelcontextprotocol/sdk dependency
- `tsconfig.qtpl` — TypeScript configuration
- Run `qtc` to generate `.qtpl.go` files, commit both

### Task 6: Golden-file tests
- File: `internal/codegen/engine_test.go`, `cli_go_test.go`, `mcp_typescript_test.go`
- Generate output for a reference manifest → compare against golden files
- Verify Go CLI output compiles (`go build` in test)
- Verify TS MCP output is valid TypeScript (structure check, not full tsc)

## File Change Map

| File | Action | Package |
|------|--------|---------|
| `internal/codegen/engine.go` | Create | codegen |
| `internal/codegen/engine_test.go` | Create | codegen |
| `internal/codegen/cli_go.go` | Create | codegen |
| `internal/codegen/cli_go_test.go` | Create | codegen |
| `internal/codegen/mcp_typescript.go` | Create | codegen |
| `internal/codegen/mcp_typescript_test.go` | Create | codegen |
| `templates/cli/golang/*.qtpl` | Create | templates |
| `templates/cli/golang/*.qtpl.go` | Create (generated) | templates |
| `templates/mcp/typescript/*.qtpl` | Create | templates |
| `templates/mcp/typescript/*.qtpl.go` | Create (generated) | templates |
| `go.mod` | Update | root (add quicktemplate) |

## As-Built Notes

### Plan Deviations

1. **text/template instead of quicktemplate**: Used Go's `text/template` with inline string
   constants rather than quicktemplate `.qtpl` files. Rationale: simpler build (no external
   `qtc` tool), no generated `.qtpl.go` files to maintain, templates compile into the binary
   as string constants. Satisfies all ACs without the complexity. Per pattern P2, embedded
   assets can be introduced later when a production consumer exists.

2. **Tasks 2+3 merged, Tasks 4+5 merged**: Per pattern P1, tightly coupled tasks sharing
   a test surface were merged. Generator code and template code are in the same file
   (`cli_go.go`, `mcp_typescript.go`) — splitting them would have created artificial
   boundaries with no testing benefit.

3. **No `templates/` directory**: Because templates are inline string constants in the
   generator files, the planned `templates/cli/golang/` and `templates/mcp/typescript/`
   directories were not created.

4. **`goIdentifier()` helper added**: Integration tests discovered that hyphenated tool
   names (e.g., `check-health`) produce invalid Go identifiers. Added `goIdentifier()`
   to convert hyphens to camelCase and `GoName` field to template data structs.

5. **`integration_test.go` as separate file**: Task 6 golden-file tests were placed in
   a dedicated `integration_test.go` rather than spreading across the existing test files.
   The integration tests write to disk and invoke `go build`/`go vet`, warranting separation.

### Actual File Paths

| File | Action |
|------|--------|
| `internal/codegen/engine.go` | Created — Generator interface, Engine, TemplateData, GenerateOptions, GenerateResult |
| `internal/codegen/engine_test.go` | Created — 37 tests |
| `internal/codegen/cli_go.go` | Created — GoCLIGenerator, Go templates as string constants |
| `internal/codegen/cli_go_test.go` | Created — 69+ tests |
| `internal/codegen/mcp_typescript.go` | Created — TSMCPGenerator, TS templates as string constants |
| `internal/codegen/mcp_typescript_test.go` | Created — 73 tests |
| `internal/codegen/integration_test.go` | Created — compilation/integration tests |
| `internal/codegen/testhelpers_test.go` | Created — shared test helpers (post-build review fix) |

### Implementation Decisions

- **Type mapping**: Go (`string→string`, `int→int`, `float→float64`, `bool→bool`),
  TypeScript (`string→string`, `int→number`, `float→number`, `bool→boolean`).
  Unknown types pass through as-is.
- **Conditional file generation**: `login.go` only for oauth2; `resolver.go` for
  token/oauth2; `middleware.ts` for token/oauth2; `metadata.ts` only for oauth2 +
  streamable-http transport.
- **Generated go.mod pins cobra v1.8.0**: May need version parameterization later.
- **219 total tests** across all codegen test files.
