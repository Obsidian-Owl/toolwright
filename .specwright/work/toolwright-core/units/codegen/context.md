# Context: Unit 5 — codegen

## Purpose

Implement the code generation engine and the two MVP targets: Go CLI (Cobra) and TypeScript MCP server (@modelcontextprotocol/sdk). Uses quicktemplate for compiled templates.

## Key Spec Sections

- §3.8: `toolwright generate cli` — flags, output directory, targets
- §3.9: `toolwright generate mcp` — flags, transports, auth scaffolding
- §4.4: Code generation pipeline (template data, type mapping)
- §4.4.1: Generated Go CLI structure
- §4.4.2: Generated TypeScript MCP server structure
- §4.4.3: Generated code ownership (.toolwright-generated marker)
- §4.4.4: Deferred targets list
- §5.9: Why quicktemplate

## Files to Create

```
internal/codegen/
├── engine.go            # Generator interface, orchestration
├── engine_test.go
├── cli_go.go            # Go CLI generator
├── cli_go_test.go
├── mcp_typescript.go    # TypeScript MCP generator
└── mcp_typescript_test.go

templates/
├── cli/golang/          # .qtpl files for Go CLI generation
│   ├── main.qtpl
│   ├── root.qtpl
│   ├── command.qtpl
│   ├── login.qtpl       # Only if any tool uses oauth2
│   ├── resolver.qtpl
│   ├── gomod.qtpl
│   └── makefile.qtpl
└── mcp/typescript/      # .qtpl files for TS MCP generation
    ├── index.qtpl
    ├── tool.qtpl
    ├── search.qtpl
    ├── middleware.qtpl   # Only if any tool uses token/oauth2
    ├── metadata.qtpl     # PRM endpoint (only if oauth2 + streamable-http)
    ├── package.qtpl
    └── tsconfig.qtpl
```

## Dependencies

- `internal/manifest` (Unit 1) — `Toolkit`, `Tool`, `Auth` types
- `github.com/valyala/quicktemplate` — compiled template engine

## Template Data

```go
type TemplateData struct {
    Manifest  manifest.Toolkit
    Tool      manifest.Tool
    Auth      manifest.Auth     // Resolved auth for current tool
    Timestamp string
    Version   string
}
```

## Type Mapping

| Manifest | Go | TypeScript |
|----------|-----|-----------|
| string | string | string |
| int | int | number |
| float | float64 | number |
| bool | bool | boolean |

## Generated Go CLI Structure

```
cli-{name}/
├── cmd/{name}/main.go
├── internal/commands/root.go, {tool}.go, login.go
├── internal/auth/resolver.go
├── go.mod, go.sum, Makefile, README.md
```

## Generated TS MCP Server Structure

```
mcp-server-{name}/
├── src/index.ts, tools/{tool}.ts, auth/middleware.ts, auth/metadata.ts, search.ts
├── package.json, tsconfig.json, README.md
```

## Gotchas

1. **Quicktemplate workflow**: Write `.qtpl` files → run `qtc` to generate `.qtpl.go` → commit both. The `.qtpl.go` files are regular Go code compiled into the binary.
2. **Conditional auth code**: Generated projects only include auth code if any tool requires auth. `login.go` only generated if any tool uses `oauth2`. Middleware only if `token` or `oauth2`.
3. **Transport handling for MCP**: When both `stdio` and `streamable-http` transports, single server binary supports both. Entry point checks invocation mode.
4. **`.toolwright-generated` marker**: Written to output dir root. Contains Toolwright version and timestamp. `generate` warns if marker exists and `--force` not set.
5. **Generated code must be valid**: Go CLI must `go build`, TS MCP must `tsc` compile. Golden-file tests verify this.
6. **No secrets in generated code** — Constitution rule 25. Templates never include token values.
