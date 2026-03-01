# Context: toolwright-core

## Project State

Greenfield Go CLI project. No code exists. Only `docs/spec.md` (1090 lines, updated with gap resolutions).

## Key Files

| File | Purpose |
|------|---------|
| `docs/spec.md` | Complete specification (source of truth) |
| `.specwright/CHARTER.md` | Project identity and invariants |
| `.specwright/CONSTITUTION.md` | 27 development rules |
| `.specwright/config.json` | Specwright project config |

## Technology Stack

| Technology | Version | Purpose |
|-----------|---------|---------|
| Go | 1.23+ | Implementation language |
| Cobra | v1.8+ | CLI framework |
| Bubble Tea | v2+ | TUI wizard for `init` |
| Lipgloss | v1+ | TUI styling |
| yaml.v3 | latest | Manifest parsing |
| jsonschema/v6 | latest | JSON Schema validation |
| ojg | latest | JSONPath for test assertions |
| go-keyring | latest | Platform keyring (token storage) |
| x/oauth2 | latest | OAuth 2.1 + PKCE |
| quicktemplate | latest | Compiled template engine for codegen |
| testify | v1.9+ | Test assertions |
| go-cmp | v0.6+ | Struct comparison |

## Module Dependency Graph (Build Order)

```
Phase 1 (leaves):     manifest, schema
Phase 2 (one-dep):    auth (→ manifest)
Phase 3 (two-dep):    runner (→ manifest, auth)
Phase 4 (multi-dep):  testing (→ manifest, runner, schema)
Phase 5 (multi-dep):  codegen (→ manifest, auth)
Phase 6 (low-dep):    tui (→ manifest), generate (→ manifest, schema)
Phase 7 (top):        cli (→ all above)
Phase 8 (entry):      cmd/toolwright/main.go
```

No circular dependency risk. `manifest` is the pure leaf node.

## Codegen MVP Scope

Only two generation targets in initial implementation:
- **Go CLI** (Cobra) — `toolwright generate cli --target go`
- **TypeScript MCP** (@modelcontextprotocol/sdk) — `toolwright generate mcp --target typescript`

Deferred: TypeScript CLI, Python CLI, Go MCP, Python MCP.

## Auth Architecture

Three modes: `none`, `token`, `oauth2`.

Resolution chain: `--auth-token` flag → env var → platform keyring → error.

Storage: platform keyring primary, fallback to `$XDG_CONFIG_HOME/toolwright/tokens.json` (0600 permissions, no encryption).

OAuth login: RFC 8414 discovery → OIDC discovery fallback → manual endpoints. PKCE with S256. Callback server on 127.0.0.1:8085 (fallback to port 0).

## Template Engine

Quicktemplate (`.qtpl` → `.qtpl.go`). Templates compile to Go code at build time. Compiled files committed to repo. Static assets (schemas, init scaffolding) embedded via `go:embed`. Template output is NOT embedded.

## Key Gotchas

1. **Auth `none` string shorthand** requires custom `UnmarshalYAML` — `auth: none` (string) vs `auth: {type: oauth2, ...}` (object)
2. **Flag.Required** field exists in manifest examples but was missing from struct definition (now added)
3. **go-keyring** must work without CGO on all platforms — Charter invariant
4. **Test assertions** support 5 operators: `equals`, `contains`, `matches`, `exists`, `length`
5. **Error output** with `--json` goes to stdout (not stderr) as structured JSON with `error.code`, `error.message`, `error.hint`
6. **Exit codes** for Toolwright itself: 0=success, 1=command failed, 2=usage error, 3=IO error
7. **Generated code** is not meant to be hand-edited — `.toolwright-generated` marker file
