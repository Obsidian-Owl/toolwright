# Toolwright Specification

> **Version:** 0.1.0-draft
> **Owner:** Obsidian Owl (github.com/Obsidian-Owl)
> **Language:** Go 1.23+
> **License:** Apache 2.0

## Vision

Tools are CLIs. MCP is an optional transport layer you generate, not a development paradigm you adopt.

Toolwright is a CLI-first tool development framework. Developers define AI agent tools as standard CLIs using a YAML manifest (`toolwright.yaml`). Toolwright validates, tests, and generates distributable packages from that manifest — both standalone CLI binaries and MCP servers. The manifest is the single source of truth. Everything else is a compile target.

## Design Principles

1. **CLI-first, protocol-optional.** Every tool works immediately via bash. No MCP server, no JSON-RPC, no capability negotiation. Any agent with a shell can use it.
2. **Context-efficient by default.** CLI tools cost zero tokens until invoked. Generated CLIs and MCP servers are optimised for minimal context window consumption — progressive discovery, compact descriptions, structured output.
3. **Generate, don't hand-code.** CLI packages, MCP servers, and function schemas are generated from one manifest. One definition, many targets.
4. **Auth-aware, not auth-implementing.** Tools declare what authentication they need. Toolwright plumbs tokens securely to entrypoints and generates MCP OAuth scaffolding. It never implements the identity provider, authorization server, or token issuance.
5. **Five-minute Hello World.** `toolwright init` to a working, testable tool in under five minutes.
6. **Unix philosophy.** Structured JSON to stdout, diagnostics to stderr, semantic exit codes, pipe-composable.
7. **Evidence-based.** Every tool has a schema, every output is validated, every test is reproducible. Companion to Specwright.

## Developer Experience

| Command | Purpose |
|---------|---------|
| `toolwright init <n>` | Scaffold a new tool project (TUI wizard or `--yes` for CI) |
| `toolwright validate` | Check manifest, schemas, entrypoints, and auth config |
| `toolwright run <tool> [args...]` | Execute a tool locally with argument validation and auth token resolution |
| `toolwright test` | Run scenario-based integration tests |
| `toolwright list` | List tools in the manifest |
| `toolwright describe <tool>` | Output machine-readable tool metadata |
| `toolwright generate cli` | Generate a distributable CLI package |
| `toolwright generate mcp` | Generate a distributable MCP server (with OAuth scaffolding if configured) |
| `toolwright generate manifest` | AI-assisted manifest generation from natural language (optional, requires API key) |

All commands support `--json` for structured output. All destructive commands support `--dry-run`. `NO_COLOR` and `CI=true` are respected automatically. Error messages state what happened, why, and how to fix it.

---

## 1. Context Engineering: Why CLI-First Wins

This section captures the architectural reasoning that shapes every design decision.

### 1.1 The Token Cost Problem

MCP tool definitions load into the agent's context window upfront. Each consumes 200+ tokens. A five-server setup routinely consumes 55,000+ tokens before the conversation starts — 16%+ of a typical context window. Anthropic's engineering team documented 134,000 tokens consumed by tool definitions alone before optimisation.

CLI tools cost zero tokens in context until invoked. The agent calls `toolwright list --json` (compact response: names and one-line descriptions) to discover what's available, then `toolwright describe <tool>` for the full schema of just the tool it needs. This is progressive discovery — the same pattern as Anthropic's Tool Search Tool (85% token reduction), implemented with standard CLI invocation.

### 1.2 Agents Already Know CLIs

Microsoft's Azure SRE team replaced 100+ narrow MCP tools with two broad CLI tools (`az`, `kubectl`) and got dramatically better results:

- **Context compression**: Two tool entries instead of hundreds.
- **Capability expansion**: Full CLI surface area, not a pre-wrapped subset.
- **Better reasoning**: LLMs know CLIs from training data. Custom abstractions fight the model's priors. CLIs are self-describing via `--help` and produce high-signal errors.

### 1.3 How Toolwright Exploits This

**CLI path** (zero-cost discovery):
```
toolwright list --json            # ~100 tokens: names + descriptions
toolwright describe scan          # ~200 tokens: full schema for ONE tool
toolwright run scan ./src         # Execution, output only
```
Total: ~300 tokens for discovery + schema, paid only when needed.

**MCP path** (optimised for deferred loading):
Generated servers support `search_tools` / deferred loading. Tool definitions are marked `defer_loading: true` so clients that support Tool Search load schemas on demand.

Both paths invoke the same entrypoints. Behaviour is identical.

---

## 2. Manifest Format

### 2.1 Overview

`toolwright.yaml` in the project root. Single source of truth. Ships with a JSON Schema at `schemas/toolwright.schema.json`, registered with SchemaStore.org.

### 2.2 Structure

```yaml
$schema: https://raw.githubusercontent.com/Obsidian-Owl/toolwright/main/schemas/toolwright.schema.json
apiVersion: toolwright/v1
kind: Toolkit

metadata:
  name: my-tool              # Required. [a-z0-9-]+
  version: 0.1.0             # Required. SemVer.
  description: >-            # Required. Under 200 chars. Used in MCP tool descriptions,
    Short description for     # CLI --help, and agent discovery responses.
    LLM tool selection.
  author: org-or-person      # Optional.
  license: Apache-2.0        # Optional.
  repository: https://github.com/org/repo  # Optional.

tools:
  - name: scan
    description: >-
      Scan a file or directory for security issues and style violations.
    entrypoint: ./bin/scan
    args:
      - name: path
        type: string
        required: true
        description: File or directory to scan.
    flags:
      - name: severity
        type: string
        default: medium
        enum: [low, medium, high, critical]
        description: Minimum severity threshold.
      - name: format
        type: string
        default: json
        enum: [json, text, sarif]
        description: Output format.
    output:
      format: json
      schema: schemas/scan-output.json
    auth: none                          # This tool requires no authentication.
    examples:
      - description: Scan src for critical issues
        args: [./src]
        flags: { severity: critical }
    exit_codes:
      0: Success
      1: Issues found
      2: Invalid input

  - name: deploy
    description: >-
      Deploy a service to the target environment.
    entrypoint: ./bin/deploy
    args:
      - name: service
        type: string
        required: true
        description: Service name to deploy.
    flags:
      - name: environment
        type: string
        required: true
        enum: [dev, staging, prod]
        description: Target environment.
    output:
      format: json
    auth:                               # This tool requires OAuth2 authentication.
      type: oauth2
      provider_url: https://auth.example.com
      scopes: [deploy:write, services:read]
      token_env: DEPLOY_TOKEN           # Env var for CLI path token resolution.
      token_flag: --auth-token          # Flag name for passing token to entrypoint.

auth:                                   # Toolkit-level auth defaults. Tools inherit these
  type: token                           # unless they override with their own auth block.
  token_env: MY_TOOL_API_KEY
  token_flag: --api-key

generate:
  cli:
    target: go
  mcp:
    target: typescript
    transport: [stdio, streamable-http]
```

### 2.3 Type System

| Type | CLI | JSON |
|------|-----|------|
| `string` | Raw value | `"value"` |
| `int` | Integer string | `123` |
| `float` | Decimal string | `1.23` |
| `bool` | Flag presence / `true`/`false` | `true`/`false` |

### 2.4 Auth Configuration

Auth can be declared at toolkit level (applies to all tools) or per-tool (overrides toolkit default). Three modes:

**`none`** — No authentication required. The tool is public. This is the default if no auth block is present.

**`token`** — A static token (API key, bearer token) passed to the entrypoint. Toolwright resolves the token and passes it to the entrypoint.

```yaml
auth:
  type: token
  token_env: MY_API_KEY               # Required. Env var to resolve token from.
  token_flag: --api-key               # Required. Flag name passed to entrypoint.
  token_header: Authorization          # Optional. Header name for MCP HTTP transport.
```

**`oauth2`** — OAuth 2.1 flow. For the CLI path, Toolwright resolves the token from env/config. For the MCP path (streamable-http), Toolwright generates OAuth scaffolding compliant with the MCP authorization spec.

```yaml
auth:
  type: oauth2
  provider_url: https://auth.example.com   # Required. Authorization server base URL.
  scopes: [read, write]                    # Required. OAuth scopes to request.
  token_env: MY_OAUTH_TOKEN                # Required. Env var for CLI-path token resolution.
  token_flag: --auth-token                 # Required. Flag passed to entrypoint.
  audience: https://api.example.com        # Optional. Resource indicator (RFC 8707).
```

**What Toolwright does NOT do:**
- Toolwright never implements an authorization server, identity provider, or login flow.
- Toolwright never issues, refreshes, or validates tokens.
- Toolwright never stores user credentials.

**What Toolwright does:**
- Resolves tokens from environment variables or the platform keyring (see §2.5, §2.6).
- Passes tokens to entrypoints via the declared `token_flag`.
- In generated MCP servers (streamable-http transport), generates the Protected Resource Metadata endpoint (`.well-known/oauth-protected-resource`) and token validation middleware that delegates to the declared `provider_url`.
- In generated CLIs, generates a `login` subcommand that initiates the OAuth 2.1 Authorization Code + PKCE flow against `provider_url`, receives the callback, and stores the token securely.
- Never logs, prints, or includes tokens in error output.

### 2.5 Token Resolution Order

When `toolwright run` or a generated CLI executes an authenticated tool:

1. **`--auth-token` flag** on `toolwright run` (explicit override, highest priority — this is Toolwright's own flag, distinct from the manifest's `token_flag` which is passed to the entrypoint)
2. **Environment variable** named in `token_env`
3. **Platform keyring** via `go-keyring` (service: `toolwright`, key: `{toolkit-name}/{tool-name}`)
4. If none found: exit with error explaining where to set the token.

### 2.6 Token Storage

Tokens from `toolwright login` are stored in the platform keyring:

- **macOS**: Keychain (via Security framework, no CGO)
- **Linux**: Secret Service API (via D-Bus, no CGO)
- **Windows**: Credential Manager

If the platform keyring is unavailable (headless Linux, containers), Toolwright falls back to `$XDG_CONFIG_HOME/toolwright/tokens.json` with a warning. The fallback file stores tokens as JSON:

```json
{
  "version": 1,
  "tokens": {
    "{toolkit-name}/{tool-name}": {
      "access_token": "...",
      "refresh_token": "...",
      "token_type": "Bearer",
      "expiry": "2025-12-01T00:00:00Z",
      "scopes": ["deploy:write"]
    }
  }
}
```

**Fallback file permissions**: created with `0600`. Toolwright refuses to read the file if permissions are more permissive. No encryption — the file relies on filesystem permissions. This is the fallback path; the primary path (platform keyring) provides OS-level encryption.

**XDG compliance**: All paths under `~/.config/toolwright/` respect `$XDG_CONFIG_HOME`. When set, paths resolve to `$XDG_CONFIG_HOME/toolwright/` instead.

### 2.7 Invocation Mapping

`toolwright run <tool> [args...] [--flags...]` translates to:

```
<entrypoint> [positional-args...] [--flag value...] [--token-flag <resolved-token>]
```

The auth token is appended as the declared `token_flag` with the resolved value. For `auth: none`, no token flag is appended.

### 2.8 Validation Rules

- `metadata.name` matches `^[a-z0-9-]+$`, `metadata.version` is valid SemVer
- `metadata.description` is non-empty and under 200 characters
- Each `tools[].name` is unique; arg/flag names unique within their tool
- `entrypoint` paths exist and are executable (`toolwright validate`)
- `enum` and `default` values match declared `type`
- `output.schema` file exists if specified
- For `auth.type: token`: `token_env` and `token_flag` are required
- For `auth.type: oauth2`: `provider_url`, `scopes`, `token_env`, `token_flag` are required
- `provider_url` is a valid HTTPS URL

---

## 3. CLI Commands

### 3.0 Global Behavior

**Global flags** (available on all commands):

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--json` | bool | false | Structured JSON output to stdout |
| `--debug` | bool | false | Verbose diagnostic output to stderr |
| `--no-color` | bool | auto | Disable colored output (also respects `NO_COLOR` env) |

**Exit codes** (for Toolwright itself, distinct from tool exit codes in the manifest):

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Command failed (validation errors, test failures, generation errors) |
| 2 | Usage error (invalid arguments, missing required flags) |
| 3 | IO error (file not found, permission denied, network unreachable) |

**Error output with `--json`:**

When `--json` is set and an error occurs, the error is returned as structured JSON to stdout (not stderr):

```json
{
  "error": {
    "code": "manifest_invalid",
    "message": "toolwright.yaml: tools[0].entrypoint: file ./bin/scan does not exist",
    "hint": "Create the entrypoint file or update the path in your manifest."
  }
}
```

Error codes are kebab-case identifiers: `manifest_invalid`, `tool_not_found`, `auth_required`, `auth_failed`, `schema_invalid`, `entrypoint_failed`, `timeout`, `io_error`.

**Debug output:** When `--debug` is set, Toolwright writes timestamped diagnostic lines to stderr. This includes manifest parsing steps, auth resolution attempts, process execution details, and template rendering progress. Debug output is never written to stdout (which is reserved for command output).

**CI detection:** When `CI=true` is set, Toolwright disables color, disables TUI interactive mode (falls back to `--yes` behavior for `init`), and uses line-buffered output.

### 3.1 `toolwright init <n>`

Scaffolds a new tool project. Interactive TUI wizard (Bubble Tea) or `--yes` for non-interactive with defaults (shell runtime, no auth).

Generated structure:

```
<n>/
├── toolwright.yaml
├── bin/hello                  # Stub printing {"message":"hello"}
├── schemas/hello-output.json
├── tests/hello.test.yaml
└── README.md
```

Works immediately. `toolwright validate` passes. `toolwright run hello` succeeds.

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--yes` | bool | false | Non-interactive |
| `--runtime` | string | shell | go, python, typescript, shell |

### 3.2 `toolwright validate`

Checks manifest, schemas, project structure, and auth configuration. For `oauth2` auth, validates that `provider_url` is reachable and serves HTTPS (with `--online` flag). Exits 0 if valid, 1 if errors.

**`--json` output:**

```json
{
  "valid": false,
  "errors": [
    {
      "path": "tools[0].entrypoint",
      "message": "file ./bin/scan does not exist",
      "rule": "entrypoint-exists"
    }
  ],
  "warnings": [
    {
      "path": "tools[1].auth.provider_url",
      "message": "provider URL not verified (use --online to check)",
      "rule": "oauth-provider-reachable"
    }
  ]
}
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--manifest` | string | ./toolwright.yaml | Path |
| `--json` | bool | false | Structured output |
| `--online` | bool | false | Verify auth provider URLs are reachable |

### 3.3 `toolwright run <tool> [args...] [flags...]`

Executes a tool locally. Validates arguments, resolves auth token (if required), executes entrypoint, optionally validates output schema. Exits with entrypoint's exit code.

If the tool requires auth and no token is found, exits with a clear error: `Error: tool "deploy" requires authentication. Set DEPLOY_TOKEN or run "toolwright login deploy".`

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--manifest` | string | ./toolwright.yaml | Path |
| `--no-validate` | bool | false | Skip output validation |
| `--timeout` | duration | 30s | Execution timeout |
| `--auth-token` | string | | Explicit token override |

### 3.4 `toolwright test`

Runs YAML test scenarios:

```yaml
tool: scan
tests:
  - name: finds sql injection
    args: [./fixtures/vulnerable.py]
    flags: { severity: low }
    expect:
      exit_code: 0
      stdout_is_json: true
      stdout_schema: schemas/scan-output.json
      stdout_contains:
        - path: $.findings[0].type
          equals: sql_injection
        - path: $.findings
          length: 3
        - path: $.metadata.version
          matches: "^\\d+\\.\\d+\\.\\d+$"
        - path: $.findings[0].severity
          exists: true
        - path: $.findings[*].type
          contains: sql_injection
    timeout: 10s

  - name: exits 2 on invalid path
    args: [/nonexistent/path]
    expect:
      exit_code: 2
      stderr_contains: ["path does not exist"]

  - name: deploys with auth
    args: [my-service]
    flags: { environment: staging }
    auth_token: "${DEPLOY_TEST_TOKEN}"    # Resolved from env var
    expect:
      exit_code: 0
```

**Assertion operators:**

| Operator | Type | Description |
|----------|------|-------------|
| `equals` | any | Exact match (strings, numbers, booleans) |
| `contains` | string/array | String contains substring, or array contains element |
| `matches` | string | Regex match |
| `exists` | bool | Path exists (true) or does not exist (false) |
| `length` | int | Array or string length equals value |

**Auth in tests:** Tests for authenticated tools use `auth_token` in the test definition. The value supports `${ENV_VAR}` expansion. Alternatively, set `TOOLWRIGHT_TEST_TOKEN` as a default for all tests.

TAP output by default, JSON with `--json`.

**`--json` output:**

```json
{
  "tool": "scan",
  "total": 3,
  "passed": 2,
  "failed": 1,
  "results": [
    {
      "name": "finds sql injection",
      "status": "pass",
      "duration_ms": 42
    },
    {
      "name": "exits 2 on invalid path",
      "status": "fail",
      "duration_ms": 15,
      "error": "expected exit_code 2, got 1",
      "stdout": "...",
      "stderr": "..."
    }
  ]
}
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--tests` | string | ./tests | Test directory |
| `--filter` | string | | Regex |
| `--json` | bool | false | JSON output |
| `--verbose` | bool | false | Show tool stdout/stderr |
| `--parallel` | int | 1 | Parallel count |

### 3.5 `toolwright list`

Lists tools. Default: human-readable table showing name, description, and auth requirement. `--json`: array of tool summaries. Deliberately compact for progressive discovery.

**`--json` output:**

```json
{
  "tools": [
    {
      "name": "scan",
      "description": "Scan a file or directory for security issues.",
      "auth_type": "none"
    },
    {
      "name": "deploy",
      "description": "Deploy a service to the target environment.",
      "auth_type": "oauth2"
    }
  ]
}
```

### 3.6 `toolwright describe <tool>`

Full schema for one tool. JSON Schema format, compatible with OpenAI `parameters`, MCP `inputSchema`, Anthropic `input_schema`. Includes auth metadata:

```json
{
  "name": "deploy",
  "description": "Deploy a service to the target environment.",
  "auth": {
    "type": "oauth2",
    "scopes": ["deploy:write", "services:read"]
  },
  "parameters": {
    "type": "object",
    "properties": {
      "service": { "type": "string", "description": "Service name." },
      "environment": { "type": "string", "enum": ["dev", "staging", "prod"] }
    },
    "required": ["service", "environment"]
  }
}
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--manifest` | string | ./toolwright.yaml | Path |
| `--format` | string | json | json, mcp, openai |

### 3.7 `toolwright login <tool>`

Initiates OAuth 2.1 Authorization Code + PKCE flow for tools with `auth.type: oauth2`.

**Flow:**

1. Discover authorization server metadata via `{provider_url}/.well-known/oauth-authorization-server` (RFC 8414). Falls back to `{provider_url}/.well-known/openid-configuration` (OIDC Discovery). If neither responds, exit with error naming the two URLs tried and suggesting manual endpoint configuration via `auth.endpoints`.
2. Generate PKCE `code_verifier` (43-128 character URL-safe string, per RFC 7636 §4.1) and `code_challenge` (SHA-256, S256 method).
3. Generate `state` parameter (32-byte random, base64url-encoded) for CSRF protection.
4. Start a temporary local HTTP callback server on `127.0.0.1`. Port selection: try `8085`, then fall back to OS-assigned (`port 0`). The redirect URI is `http://127.0.0.1:{port}/callback`.
5. Open the authorization URL in the user's browser (or print it with `--no-browser`).
6. Wait for the callback (timeout: 120 seconds). Validate `state` parameter matches.
7. Exchange the authorization code for tokens at the token endpoint.
8. Store the access token (and refresh token, if issued) in the platform keyring (see §2.6).
9. Shut down the callback server.

**Token refresh:** When the resolver (§2.5) finds a stored token that is expired and a refresh token is available, it attempts a silent refresh against the token endpoint. If refresh fails (revoked, expired), the resolver returns an error directing the user to re-run `toolwright login`.

**Error handling:**
- User cancels in browser → callback server times out → exit with "Login cancelled or timed out. Run `toolwright login <tool>` to try again."
- Provider unreachable → exit with "Cannot reach authorization server at {provider_url}. Check the URL and your network."
- Invalid redirect / state mismatch → exit with "Security check failed (state mismatch). This may indicate a CSRF attack. Run `toolwright login <tool>` to try again."

Only available for tools with `oauth2` auth. For `token` auth, the user sets the env var directly.

**Manual endpoint configuration** (fallback when discovery fails):

```yaml
auth:
  type: oauth2
  provider_url: https://auth.example.com
  endpoints:                                    # Optional. Overrides discovery.
    authorization: https://auth.example.com/authorize
    token: https://auth.example.com/oauth/token
    jwks: https://auth.example.com/.well-known/jwks.json
  scopes: [deploy:write]
  token_env: DEPLOY_TOKEN
  token_flag: --auth-token
```

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--manifest` | string | ./toolwright.yaml | Path |
| `--no-browser` | bool | false | Print URL instead of opening browser |

### 3.8 `toolwright generate cli`

Generates a distributable CLI. Each tool becomes a subcommand. Standalone project, no Toolwright dependency.

Agent-optimised features in the generated CLI:
- `<binary> list --json` — compact tool enumeration
- `<binary> describe <tool>` — progressive discovery baked in
- `<binary> --help` — structured help text (LLMs parse this well)
- `<binary> --json` on every subcommand
- `<binary> login <tool>` — generated OAuth login flow for `oauth2` tools

Auth in the generated CLI:
- For `token` auth: token resolved from env var or `--<token-flag>` argument.
- For `oauth2` auth: `login` subcommand generated, performing the Authorization Code + PKCE flow. Tokens stored in platform keychain (macOS Keychain, Linux Secret Service, Windows Credential Manager) via `go-keyring`, falling back to encrypted file.

**Go target** (default): Cobra CLI, single binary.
**TypeScript target**: Commander.js, npm distribution.
**Python target**: Click, PyPI distribution.

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--output` | string | ./cli-\<name\> | Output directory |
| `--target` | string | go | go, typescript, python |
| `--dry-run` | bool | false | Preview |
| `--force` | bool | false | Overwrite |

### 3.9 `toolwright generate mcp`

Generates a standalone MCP server. Context-efficient and auth-aware.

Context optimisations:
- All tools `defer_loading: true` for Tool Search support.
- `search_tools` meta-tool for on-demand discovery.
- Compact descriptions from the manifest's 200-char limit.

Auth in the generated MCP server:
- For `auth: none` tools: no auth middleware. Tool is publicly accessible.
- For `auth: token` tools: server expects `Authorization: Bearer <token>` header on HTTP transport. For stdio transport, token is read from env var.
- For `auth: oauth2` tools with **streamable-http** transport: generates MCP-spec-compliant OAuth scaffolding:
  - `/.well-known/oauth-protected-resource` endpoint returning Protected Resource Metadata (RFC 9728) pointing to the declared `provider_url`.
  - Token validation middleware that verifies access tokens against the authorization server's JWKS endpoint.
  - PKCE enforcement (S256 method).
  - Resource Indicators (RFC 8707) with the declared `audience`.
- For `auth: oauth2` tools with **stdio** transport: token resolved from env var (same as CLI path). Stdio is local — the auth boundary is at the MCP client, not the server.

**TypeScript target** (default): `@modelcontextprotocol/sdk`.
**Go target**: `github.com/mark3labs/mcp-go`. Single binary.
**Python target**: `fastmcp`.

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--output` | string | ./mcp-server-\<name\> | Output directory |
| `--target` | string | typescript | typescript, go, python |
| `--transport` | []string | [stdio] | stdio, streamable-http |
| `--dry-run` | bool | false | Preview |
| `--force` | bool | false | Overwrite |

### 3.10 `toolwright generate manifest` (Optional)

AI-assisted manifest generation from natural language. Single LLM call with manifest schema as context, validates response, retries once on failure.

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `--provider` | string | | Required. anthropic, openai, gemini |
| `--model` | string | provider default | Override |
| `--output` | string | ./toolwright.yaml | Output |
| `--no-merge` | bool | false | Fail if exists |
| `--dry-run` | bool | false | Stdout only |

Key resolution: `ANTHROPIC_API_KEY` / `OPENAI_API_KEY` / `GEMINI_API_KEY` env vars, then `~/.config/toolwright/config.yaml`. Keys never logged or written to output.

---

## 4. Architecture

### 4.1 Module Layout

```
toolwright/
├── cmd/toolwright/main.go
├── internal/
│   ├── cli/                              # Cobra command definitions (thin wiring)
│   │   ├── root.go
│   │   ├── init.go
│   │   ├── validate.go
│   │   ├── run.go
│   │   ├── test.go
│   │   ├── list.go
│   │   ├── describe.go
│   │   ├── login.go                      # OAuth login flow
│   │   └── generate.go                   # cli, mcp, manifest subcommands
│   ├── manifest/
│   │   ├── types.go                      # All manifest types including Auth
│   │   ├── parser.go
│   │   └── parser_test.go
│   ├── schema/
│   │   ├── validator.go
│   │   └── validator_test.go
│   ├── auth/                             # Auth token resolution and storage
│   │   ├── resolver.go                   # Token resolution chain (flag → env → keyring)
│   │   ├── keyring.go                    # Platform keyring access via go-keyring
│   │   ├── store.go                      # Fallback file-based token storage (§2.6)
│   │   ├── oauth.go                      # PKCE flow, callback server, discovery
│   │   └── resolver_test.go
│   ├── runner/
│   │   ├── executor.go                   # Process execution with token injection
│   │   ├── output.go
│   │   └── executor_test.go
│   ├── testing/
│   │   ├── runner.go
│   │   ├── assertions.go                 # equals, contains, matches, exists, length
│   │   └── runner_test.go
│   ├── codegen/
│   │   ├── engine.go                     # Generator interface and orchestration
│   │   ├── cli_go.go                     # Go CLI generator (MVP)
│   │   ├── mcp_typescript.go             # TypeScript MCP generator (MVP)
│   │   └── engine_test.go
│   ├── generate/                         # AI manifest generation (optional)
│   │   ├── manifest.go
│   │   ├── provider.go
│   │   ├── anthropic.go
│   │   ├── openai.go
│   │   ├── gemini.go
│   │   └── prompt.go
│   └── tui/
│       └── wizard.go
├── templates/
│   ├── init/                             # Static scaffolding files for `toolwright init`
│   ├── cli/golang/                       # Quicktemplates (.qtpl) for Go CLI generation
│   └── mcp/typescript/                   # Quicktemplates (.qtpl) for TS MCP generation
├── schemas/toolwright.schema.json
├── embed.go                              # //go:embed schemas/* templates/init/*
├── go.mod
└── go.sum
```

### 4.2 Core Types

```go
// internal/manifest/types.go

type Toolkit struct {
    APIVersion string   `yaml:"apiVersion" json:"apiVersion"`
    Kind       string   `yaml:"kind" json:"kind"`
    Metadata   Metadata `yaml:"metadata" json:"metadata"`
    Tools      []Tool   `yaml:"tools" json:"tools"`
    Auth       *Auth    `yaml:"auth,omitempty" json:"auth,omitempty"` // Toolkit-level default
    Generate   Generate `yaml:"generate,omitempty" json:"generate,omitempty"`
}

type Metadata struct {
    Name        string `yaml:"name" json:"name"`
    Version     string `yaml:"version" json:"version"`
    Description string `yaml:"description" json:"description"`
    Author      string `yaml:"author,omitempty" json:"author,omitempty"`
    License     string `yaml:"license,omitempty" json:"license,omitempty"`
    Repository  string `yaml:"repository,omitempty" json:"repository,omitempty"`
}

type Tool struct {
    Name        string         `yaml:"name" json:"name"`
    Description string         `yaml:"description" json:"description"`
    Entrypoint  string         `yaml:"entrypoint" json:"entrypoint"`
    Args        []Arg          `yaml:"args,omitempty" json:"args,omitempty"`
    Flags       []Flag         `yaml:"flags,omitempty" json:"flags,omitempty"`
    Output      Output         `yaml:"output" json:"output"`
    Auth        *Auth          `yaml:"auth,omitempty" json:"auth,omitempty"` // Per-tool override
    Examples    []Example      `yaml:"examples,omitempty" json:"examples,omitempty"`
    ExitCodes   map[int]string `yaml:"exit_codes,omitempty" json:"exit_codes,omitempty"`
}

// Auth configures authentication for a tool or toolkit.
// Accepts either a string shorthand ("none") or a full object.
// Custom UnmarshalYAML handles both: `auth: none` (string) and `auth: {type: oauth2, ...}` (object).
type Auth struct {
    Type        string     `yaml:"type" json:"type"`                                   // none, token, oauth2
    TokenEnv    string     `yaml:"token_env,omitempty" json:"token_env,omitempty"`      // Env var name
    TokenFlag   string     `yaml:"token_flag,omitempty" json:"token_flag,omitempty"`    // Flag passed to entrypoint
    TokenHeader string     `yaml:"token_header,omitempty" json:"token_header,omitempty"`// HTTP header (MCP)
    ProviderURL string     `yaml:"provider_url,omitempty" json:"provider_url,omitempty"`// OAuth2 AS base URL
    Endpoints   *Endpoints `yaml:"endpoints,omitempty" json:"endpoints,omitempty"`      // Manual OAuth endpoint override
    Scopes      []string   `yaml:"scopes,omitempty" json:"scopes,omitempty"`            // OAuth2 scopes
    Audience    string     `yaml:"audience,omitempty" json:"audience,omitempty"`         // RFC 8707 resource
}

// UnmarshalYAML handles the string shorthand: `auth: none` becomes Auth{Type: "none"}.
// Full object form is unmarshaled normally.
func (a *Auth) UnmarshalYAML(value *yaml.Node) error { /* ... */ }

// Endpoints provides manual OAuth endpoint configuration when discovery fails.
type Endpoints struct {
    Authorization string `yaml:"authorization" json:"authorization"` // Authorization endpoint URL
    Token         string `yaml:"token" json:"token"`                 // Token endpoint URL
    JWKS          string `yaml:"jwks,omitempty" json:"jwks,omitempty"` // JWKS endpoint URL (for MCP token validation)
}

// ResolvedAuth returns the effective auth config for a tool,
// falling back to toolkit-level defaults.
func (t *Toolkit) ResolvedAuth(tool Tool) Auth {
    if tool.Auth != nil {
        return *tool.Auth
    }
    if t.Auth != nil {
        return *t.Auth
    }
    return Auth{Type: "none"}
}

type Arg struct {
    Name        string `yaml:"name" json:"name"`
    Type        string `yaml:"type" json:"type"`
    Required    bool   `yaml:"required" json:"required"`
    Description string `yaml:"description" json:"description"`
}

type Flag struct {
    Name        string   `yaml:"name" json:"name"`
    Type        string   `yaml:"type" json:"type"`
    Required    bool     `yaml:"required,omitempty" json:"required,omitempty"`
    Default     any      `yaml:"default,omitempty" json:"default,omitempty"`
    Enum        []string `yaml:"enum,omitempty" json:"enum,omitempty"`
    Description string   `yaml:"description" json:"description"`
}

type Output struct {
    Format string `yaml:"format" json:"format"`
    Schema string `yaml:"schema,omitempty" json:"schema,omitempty"`
}

type Example struct {
    Description string            `yaml:"description" json:"description"`
    Args        []string          `yaml:"args" json:"args"`
    Flags       map[string]string `yaml:"flags,omitempty" json:"flags,omitempty"`
}

type Generate struct {
    CLI CLIConfig `yaml:"cli,omitempty" json:"cli,omitempty"`
    MCP MCPConfig `yaml:"mcp,omitempty" json:"mcp,omitempty"`
}

type CLIConfig struct {
    Target string `yaml:"target" json:"target"`
}

type MCPConfig struct {
    Target    string   `yaml:"target" json:"target"`
    Transport []string `yaml:"transport" json:"transport"`
}
```

### 4.3 Auth Resolution Pipeline

```
toolwright run deploy my-service --environment prod
       │
       ▼
  manifest.ResolvedAuth(tool) → Auth{type: "oauth2", token_env: "DEPLOY_TOKEN", ...}
       │
       ▼
  auth.Resolver.Resolve(ctx, authConfig)
       │
       ├── 1. Check --auth-token CLI flag         → found? use it
       ├── 2. Check DEPLOY_TOKEN env var           → found? use it
       ├── 3. Check platform keyring (§2.6)        → found & not expired? use it
       └── 4. None found → exit with error + guidance
       │
       ▼
  runner.Executor.Run(tool, args, flags, token)
       │
       ▼
  exec: ./bin/deploy my-service --environment prod --auth-token <resolved-token>
```

Tokens from the store are checked for expiry. If a refresh token is available and the access token is expired, the resolver attempts a silent refresh before falling back to error.

### 4.4 Code Generation Pipeline

```
toolwright.yaml
       │
       ▼
  manifest.Parser.Parse()                → Toolkit
       │
       ▼
  codegen.Engine.Generate(mode, target)
       │
       ├── mode=cli, target=go
       │   └── Compiled quicktemplates → Cobra CLI project
       │
       └── mode=mcp, target=typescript
           └── Compiled quicktemplates → MCP SDK project
```

**MVP targets:** Go CLI and TypeScript MCP. Additional targets (Python CLI, Go MCP, Python MCP, TypeScript CLI) are deferred to future work units.

**Template engine:** `valyala/quicktemplate`. Templates are `.qtpl` files in `templates/`. The `qtc` compiler generates `.qtpl.go` files at build time (committed to repo). Generated Go code is embedded via `go:embed` alongside static assets (schemas, scaffolding files).

Template data:

```go
type TemplateData struct {
    Manifest  Toolkit
    Tool      Tool
    Auth      Auth     // Resolved auth for current tool
    Timestamp string   // Generation timestamp
    Version   string   // Toolwright version that generated this
}
```

**Type mapping** (used in templates):

| Manifest type | Go | TypeScript | Python |
|---------------|-----|-----------|--------|
| `string` | `string` | `string` | `str` |
| `int` | `int` | `number` | `int` |
| `float` | `float64` | `number` | `float` |
| `bool` | `bool` | `boolean` | `bool` |

#### 4.4.1 Generated Go CLI Structure

```
cli-{name}/
├── cmd/{name}/main.go              # Entry point
├── internal/
│   ├── commands/
│   │   ├── root.go                 # Root Cobra command (list, describe)
│   │   ├── {tool}.go               # One file per tool subcommand
│   │   └── login.go                # OAuth login (only if any tool uses oauth2)
│   └── auth/
│       └── resolver.go             # Token resolution (env → keyring → error)
├── go.mod
├── go.sum
├── Makefile                        # build, install targets
└── README.md                       # Generated usage docs
```

**Auth in generated Go CLI:**
- `auth: none` → no auth code generated for that tool
- `auth: token` → `resolver.go` checks `--{token_flag}` flag, then env var (same priority order as §2.5)
- `auth: oauth2` → `login.go` subcommand with PKCE flow via `golang.org/x/oauth2`, token stored in platform keyring via `go-keyring`

#### 4.4.2 Generated TypeScript MCP Server Structure

```
mcp-server-{name}/
├── src/
│   ├── index.ts                    # Server entry point
│   ├── tools/
│   │   └── {tool}.ts               # Tool handler (calls entrypoint via child_process)
│   ├── auth/
│   │   ├── middleware.ts           # Token validation (only if any tool uses token/oauth2)
│   │   └── metadata.ts            # PRM endpoint (only if any tool uses oauth2 + streamable-http)
│   └── search.ts                  # search_tools meta-tool for progressive discovery
├── package.json
├── tsconfig.json
└── README.md
```

**Transport handling:** When `transport` includes both `stdio` and `streamable-http`, a single server binary supports both. The entry point checks how it was invoked:
- stdio: reads JSON-RPC from stdin, writes to stdout. Auth tokens from env vars.
- streamable-http: starts an HTTP server. OAuth tools get PRM endpoint + token validation middleware.

**Auth in generated TypeScript MCP server:**
- `auth: none` → no middleware on that tool's handler
- `auth: token` → `Authorization: Bearer` header validation middleware
- `auth: oauth2` + streamable-http → PRM endpoint (`.well-known/oauth-protected-resource`) returning `{ resource: "{audience}", authorization_servers: ["{provider_url}"], scopes_supported: [...] }`, plus JWKS-based token validation middleware

#### 4.4.3 Generated Code Ownership

Generated code is **not meant to be hand-edited**. Regeneration (`--force`) overwrites the entire output directory. Users who need customization should:
1. Generate once, then stop using Toolwright's generator (eject)
2. Or contribute to Toolwright's templates to support their use case

A `.toolwright-generated` marker file in the output directory root contains the Toolwright version and generation timestamp. `toolwright generate` warns if the marker exists and `--force` is not set.

#### 4.4.4 Deferred Targets

The following generation targets are planned but not included in the initial implementation:

| Target | Mode | Framework | Status |
|--------|------|-----------|--------|
| TypeScript CLI | cli | Commander.js | Deferred |
| Python CLI | cli | Click | Deferred |
| Go MCP | mcp | mcp-go | Deferred |
| Python MCP | mcp | FastMCP | Deferred |

The `codegen.Engine` interface is target-agnostic. Adding a target means implementing the `Generator` interface and providing `.qtpl` templates. No changes to the engine or CLI layer.

### 4.5 Process Execution

Child processes via `os/exec.CommandContext`. Timeout via context. Process group IDs for clean termination. stdout/stderr captured separately. Working directory is project root. Auth tokens injected as the declared `token_flag` — never via environment variable passthrough (prevents token leakage to child process environment).

### 4.6 Embedded Resources

```go
//go:embed schemas/*
var Schemas embed.FS

//go:embed templates/init/*
var InitTemplates embed.FS
```

Compiled quicktemplate output (`.qtpl.go` files) is regular Go code — it does not need embedding. Static assets (JSON schemas, init scaffolding files) are embedded. The `templates/` directory contains both `.qtpl` source files (for the `qtc` compiler) and static files (for `go:embed`).

---

## 5. Design Decisions

### 5.1 Why YAML Manifest, Not Code Decorators

Language-agnostic, diffable, reviewable as data. Entrypoint can be any executable. Schema validation is static. Auth requirements are declarative data, not runtime assertions.

### 5.2 Why Code Generation, Not Runtime Bridging

Generated packages are distributable (npm, PyPI, GitHub Releases), have no Toolwright dependency, are auditable, and enforce type safety at generation time. Generated OAuth scaffolding is real code the consumer owns and can customise.

### 5.3 Why Both CLI and MCP Generation

CLI embodies "bash is all you need" — inherently context-efficient. MCP is the bridge for environments that mandate it. Both invoke the same entrypoints. Users ship both.

### 5.4 Why Auth Is Declarative, Not Implemented

Toolwright declares auth requirements and generates scaffolding. It never implements the identity provider. This is deliberate: enterprises already have IdPs (Okta, Auth0, Entra ID). The MCP spec (June 2025+) separates the MCP server (resource server) from the authorization server. Toolwright follows this separation — it generates the resource server side (PRM endpoint, token validation) and points at the external authorization server.

### 5.5 Why Tokens Are Passed via Flag, Not Environment

Passing tokens as command-line flags to entrypoints (not env vars) prevents token leakage to child processes that the entrypoint may spawn. The flag is visible in the process's argv, but not inherited by children. On systems where `/proc/{pid}/cmdline` is world-readable, argv visibility is a concern — but it is less severe than environment inheritance (which silently propagates to all child processes). For highly sensitive environments, entrypoint authors should design their tool to read tokens from a temporary file or Unix domain socket.

### 5.6 Why Go

Single binary, zero dependencies, cross-compilation, sub-10ms startup. `os/exec` and `embed` stdlib directly serve process execution and resource embedding. `golang.org/x/oauth2` provides a battle-tested OAuth 2.1 + PKCE implementation. Proven: Docker, Terraform, kubectl.

### 5.7 Why JSON Schema for Parameters

Universal standard. MCP `inputSchema`, OpenAI `parameters`, Anthropic `input_schema` all use it.

### 5.8 Why Progressive Discovery

MCP's default upfront loading scales poorly (55K+ tokens for five servers). Generated CLIs and MCP servers both implement search → describe → execute. Agents pay only for tools they use.

### 5.9 Why Quicktemplate for Code Generation

Code generation templates produce security-sensitive code (auth scaffolding, token handling). `text/template` catches errors at runtime; `quicktemplate` compiles templates to Go code, catching structural errors at compile time. Templates live in `templates/` as `.qtpl` files; `quicktemplate` generates `.qtpl.go` files that are committed to the repository. The compiled `.qtpl.go` files are regular Go source — they are imported directly, not embedded.

---

## 6. Go Dependencies

| Dependency | Version | Purpose |
|-----------|---------|---------|
| `github.com/spf13/cobra` | v1.10+ | CLI framework |
| `charm.land/bubbletea/v2` | v2.0+ | TUI wizard |
| `charm.land/lipgloss/v2` | v2.0+ | TUI styling |
| `go.yaml.in/yaml/v3` | latest | YAML parsing |
| `github.com/santhosh-tekuri/jsonschema/v6` | v6.0+ | JSON Schema validation |
| `github.com/ohler55/ojg` | latest | JSONPath for test assertions |
| `github.com/zalando/go-keyring` | v0.2+ | Platform keychain access (token storage) |
| `golang.org/x/oauth2` | latest | OAuth 2.1 Authorization Code + PKCE flow |
| `github.com/valyala/quicktemplate` | v1.7+ | Compiled template engine for code generation |
| `github.com/stretchr/testify` | v1.10+ | Test assertions |
| `github.com/google/go-cmp` | v0.7+ | Struct comparison |

No CGO. Pure Go. `go-keyring` uses D-Bus on Linux (no CGO), Keychain API on macOS, Credential Manager on Windows. `quicktemplate` compiles templates to Go code at build time — template errors are caught at compile time, not runtime.

Note: Cobra v1.10+ migrated from `gopkg.in/yaml.v3` to `go.yaml.in/yaml/v3`. Bubble Tea v2 and Lipgloss v2 migrated from `github.com/charmbracelet/*` to `charm.land/*`. All imports in Toolwright use the new module paths.

---

## 7. Agent Integration

### 7.1 CLI Path (Any Agent with Bash)

Zero configuration. Progressive discovery. Auth-aware:

```bash
toolwright list --json                           # What tools exist? Which need auth?
toolwright describe deploy                       # Full schema + auth requirements
DEPLOY_TOKEN=xxx toolwright run deploy svc-1 --environment prod  # Execute with token

# Or with generated CLI (Toolwright not required):
my-tool list --json
my-tool login deploy                              # OAuth flow, stores token
my-tool deploy svc-1 --environment prod --json
```

### 7.2 MCP Path (Claude Code, Cursor, VS Code Copilot)

Register in `.mcp.json`:

```json
{
  "mcpServers": {
    "my-tool": {
      "command": "node",
      "args": ["./mcp-server-my-tool/dist/index.js"],
      "env": {
        "DEPLOY_TOKEN": "${DEPLOY_TOKEN}"
      }
    }
  }
}
```

For streamable-http transport with OAuth, the MCP client handles the OAuth flow using the server's PRM endpoint. For stdio transport, tokens are passed via environment.

### 7.3 Relationship Between Paths

Both paths invoke the same entrypoints. A typical distribution:

```
my-tool/
├── toolwright.yaml                # Source of truth (including auth config)
├── bin/{scan,deploy}              # Entrypoint scripts
├── cli-my-tool/                   # toolwright generate cli
│   └── (single binary with login, list, describe, subcommands)
└── mcp-server-my-tool/            # toolwright generate mcp
    └── (MCP server with PRM endpoint, search_tools, deferred loading)
```

The CLI is the thesis. The MCP server is the bridge. Both respect auth. Users ship both.