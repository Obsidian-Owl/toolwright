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
