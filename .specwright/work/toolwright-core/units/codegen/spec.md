# Spec: Unit 5 — codegen

## Acceptance Criteria

### AC-1: Generator interface is target-agnostic
- Engine accepts generators via `Register(g Generator)`
- `Generate(ctx, "cli", "go", ...)` dispatches to Go CLI generator
- `Generate(ctx, "mcp", "typescript", ...)` dispatches to TS MCP generator
- Unknown mode/target combination → error with available options listed

### AC-2: Go CLI generates valid project structure
- Input: manifest with 2 tools (one `auth: none`, one `auth: token`) → output directory contains:
  - `cmd/{name}/main.go`
  - `internal/commands/root.go`
  - `internal/commands/{tool1}.go`
  - `internal/commands/{tool2}.go`
  - `internal/auth/resolver.go` (because one tool uses auth)
  - `go.mod` with correct module path and dependencies
  - `Makefile`, `README.md`
- No `login.go` (no oauth2 tool)

### AC-3: Go CLI generates login command for oauth2 tools
- Manifest with an `auth: oauth2` tool → `internal/commands/login.go` generated
- Login command includes PKCE flow scaffolding

### AC-4: Go CLI list and describe subcommands
- Generated root.go includes `list` subcommand outputting tool names/descriptions
- `list --json` outputs JSON array
- Generated root.go includes `describe {tool}` subcommand outputting JSON Schema

### AC-5: Go CLI per-tool subcommands map args and flags
- Generated command file maps manifest `args` to Cobra positional args
- Generated command file maps manifest `flags` to Cobra flags with types, defaults, enums
- Required flags enforced
- Tool description used in Cobra help text

### AC-6: Go CLI output compiles
- Generated Go project passes `go build ./...` (tested in CI or integration test)
- go.mod has valid module path and all required dependencies

### AC-7: TypeScript MCP generates valid project structure
- Input: manifest with 2 tools, transport `[stdio]` → output contains:
  - `src/index.ts` (stdio server)
  - `src/tools/{tool1}.ts`, `src/tools/{tool2}.ts`
  - `src/search.ts` (search_tools meta-tool)
  - `package.json` with `@modelcontextprotocol/sdk` dependency
  - `tsconfig.json`
  - `README.md`
- No auth middleware (both tools `auth: none`)

### AC-8: TypeScript MCP generates auth middleware for token auth
- Manifest with `auth: token` tool → `src/auth/middleware.ts` generated
- Middleware validates `Authorization: Bearer` header

### AC-9: TypeScript MCP generates PRM endpoint for oauth2
- Manifest with `auth: oauth2` tool + `transport: [streamable-http]` →
  - `src/auth/metadata.ts` generated with `/.well-known/oauth-protected-resource`
  - PRM response includes `resource`, `authorization_servers`, `scopes_supported`
- `auth: oauth2` + `transport: [stdio]` only → no metadata.ts (stdio is local)

### AC-10: TypeScript MCP search_tools meta-tool
- Generated `search.ts` implements a tool that lists available tools with names and descriptions
- Progressive discovery: agents call search first, then get full schema on demand

### AC-11: Type mapping is correct
- Manifest `type: string` → Go `string`, TS `string`
- Manifest `type: int` → Go `int`, TS `number`
- Manifest `type: float` → Go `float64`, TS `number`
- Manifest `type: bool` → Go `bool`, TS `boolean`

### AC-12: .toolwright-generated marker file
- Generated output includes `.toolwright-generated` at root
- Contains Toolwright version, timestamp, mode, target
- Generating into directory with existing marker and no `--force` → error
- Generating with `--force` → overwrites

### AC-13: No secrets in generated code
- Generated source files never contain token values, API keys, or credentials
- Auth scaffolding references env vars and flags, not literal secrets

### AC-14: Dry run outputs without writing
- `--dry-run` → lists files that would be generated (to stdout), writes nothing to disk

### AC-15: Generated code handles tools with no auth
- Tool with `auth: none` → no auth code in that tool's generated handler
- Mixed auth (some tools none, some token) → auth code only for tools that need it

### AC-16: Build and tests pass
- `go build ./...` succeeds
- `go test ./internal/codegen/...` passes
- `go vet ./...` clean
