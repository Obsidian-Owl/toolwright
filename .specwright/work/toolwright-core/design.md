# Design: toolwright-core

## Overview

Implement the Toolwright CLI framework from scratch in Go. The system parses YAML manifests, validates them, runs tools locally, tests them, and generates distributable CLI and MCP server packages.

## Approach

Build bottom-up along the module dependency graph. Each package is self-contained with clear interfaces. TDD throughout — tests before implementation per Constitution rule 8.

### Build Strategy

**Phase 1 — Foundation (no internal deps):**
- `internal/manifest`: Types, YAML parser, custom `Auth.UnmarshalYAML`, validation rules
- `internal/schema`: JSON Schema validation wrapper around `santhosh-tekuri/jsonschema/v6`

**Phase 2 — Auth (depends on manifest):**
- `internal/auth`: Token resolver chain, platform keyring via `go-keyring`, fallback file store, OAuth 2.1 PKCE flow with discovery

**Phase 3 — Execution (depends on manifest + auth):**
- `internal/runner`: Process execution with `os/exec.CommandContext`, token injection, timeout, output capture

**Phase 4 — Testing (depends on manifest + runner + schema):**
- `internal/testing`: YAML test scenario parser, tool execution via runner, JSONPath assertions via `ojg`, TAP + JSON output

**Phase 5 — Code Generation (depends on manifest + auth):**
- `internal/codegen`: Generator interface, engine orchestration
- Go CLI target: quicktemplates producing Cobra project
- TypeScript MCP target: quicktemplates producing MCP SDK project

**Phase 6 — UI + AI (depends on manifest + schema):**
- `internal/tui`: Bubble Tea wizard for `toolwright init`
- `internal/generate`: AI-assisted manifest generation (optional, 3 LLM providers)

**Phase 7 — CLI Wiring (depends on all above):**
- `internal/cli`: Thin Cobra commands delegating to domain packages
- `cmd/toolwright/main.go`: Entry point

**Phase 8 — Static Assets:**
- `schemas/toolwright.schema.json`: Manifest schema
- `templates/init/`: Scaffolding files for `toolwright init`
- `embed.go`: `go:embed` for schemas and init templates

### Key Interfaces

```go
// codegen/engine.go
type Generator interface {
    Generate(ctx context.Context, data TemplateData, outputDir string) error
    Mode() string   // "cli" or "mcp"
    Target() string // "go", "typescript", "python"
}

// auth/resolver.go
type Resolver struct { /* ... */ }
func (r *Resolver) Resolve(ctx context.Context, auth manifest.Auth, flagValue string) (string, error)

// runner/executor.go
type Executor struct { /* ... */ }
func (e *Executor) Run(ctx context.Context, tool manifest.Tool, args []string, flags map[string]string, token string) (*Result, error)

// testing/runner.go
type TestRunner struct { /* ... */ }
func (tr *TestRunner) Run(ctx context.Context, suite TestSuite) (*TestReport, error)
```

### Integration Points

- **manifest** is the shared data layer — all packages import it for types
- **auth** is used by both `runner` (to resolve tokens for `toolwright run`) and `codegen` (to generate auth scaffolding in output)
- **schema** is used by both `testing` (to validate tool output) and `cli/validate.go` (to validate manifest schemas)
- **runner** is used only by `testing` and `cli/run.go`
- **codegen** is used only by `cli/generate.go`

### What This Design Does NOT Change

- No external services or APIs (except OAuth provider during `login`)
- No daemon or background processes
- No database or persistent state beyond token storage
- No network listeners except the temporary OAuth callback server

## Risk Assessment

| Risk | Severity | Mitigation |
|------|----------|------------|
| go-keyring CGO requirement on some platform | High | Test on all 3 platforms in CI. Fallback store is implemented and tested. |
| Quicktemplate learning curve | Medium | Templates are straightforward string generation. Compile errors surface at build time. |
| OAuth provider interoperability | Medium | Support RFC 8414 + OIDC discovery + manual endpoints. Test against Auth0/Okta. |
| MCP SDK TypeScript API stability | Medium | Pin SDK version in generated package.json. Template changes are isolated. |
| Generated code correctness | High | Golden-file tests: generate → compile → run. CI must have Go + Node toolchains. |

## Alternatives Considered

### Template Engine

| Option | Pros | Cons | Decision |
|--------|------|------|----------|
| `text/template` | Stdlib, no dep | Runtime errors, weak typing | Rejected |
| `quicktemplate` | Compile-time errors, fast, typed | Build-time code generation step | **Chosen** |
| Raw string builders | Maximum control | Unmaintainable for 2+ targets | Rejected |

### Token Storage

| Option | Pros | Cons | Decision |
|--------|------|------|----------|
| Platform keyring only | OS-level encryption | Fails in containers/headless | Rejected (no fallback) |
| Encrypted file only | Works everywhere | Key management complexity | Rejected |
| Keyring + file fallback | Best of both | Two code paths | **Chosen** |

### OAuth Implementation

| Option | Pros | Cons | Decision |
|--------|------|------|----------|
| `golang.org/x/oauth2` | Battle-tested, maintained | Heavier dep | **Chosen** |
| Raw `net/http` + `crypto` | No deps | Security risk, more code | Rejected |
