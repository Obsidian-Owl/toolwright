# Spec: Unit 6 — cli

## Acceptance Criteria

### AC-1: Global --json flag produces structured JSON on all commands
- `toolwright validate --json` → JSON output per §3.2 format
- `toolwright list --json` → JSON output per §3.5 format
- `toolwright test --json` → JSON output per §3.4 format
- `toolwright describe <tool>` → JSON (default, no flag needed)

### AC-2: Global --json error output
- Command fails with `--json` → stdout contains `{"error": {"code": "...", "message": "...", "hint": "..."}}`
- Error codes are kebab-case: `manifest_invalid`, `tool_not_found`, `auth_required`

### AC-3: Exit codes match specification
- Validation errors → exit 1
- Invalid usage (missing required arg) → exit 2
- File not found (manifest missing) → exit 3
- Success → exit 0

### AC-4: NO_COLOR and CI detection
- `NO_COLOR=1` → no ANSI color codes in output
- `CI=true` → no color, no interactive TUI, line-buffered output
- Neither set, terminal attached → color enabled

### AC-5: --debug writes diagnostics to stderr
- `--debug` → timestamped lines on stderr (parsing steps, auth resolution, etc.)
- Debug output never written to stdout
- Without `--debug` → no diagnostic output

### AC-6: validate checks manifest and reports errors
- Valid manifest → exit 0, "valid" message
- Invalid manifest → exit 1, lists all errors with paths
- `--json` → structured error list per §3.2 JSON format
- `--online` with unreachable provider_url → warning (not error)

### AC-7: validate checks entrypoint existence
- Entrypoint file exists and is executable → passes
- Entrypoint file missing → error at `tools[N].entrypoint`
- Entrypoint exists but not executable → warning

### AC-8: list outputs tool table
- Human format: table with name, description, auth columns
- `--json`: `{"tools": [{"name": "...", "description": "...", "auth_type": "..."}]}`
- Empty tools list → empty table / empty array (not error)

### AC-9: describe outputs JSON Schema for tool
- `toolwright describe scan` → JSON Schema with name, description, auth, parameters
- `--format json` (default): generic format
- `--format mcp`: MCP inputSchema format
- `--format openai`: OpenAI parameters format
- Unknown tool name → error: `tool "xyz" not found`

### AC-10: run executes tool with auth
- `toolwright run scan ./src --severity high` → runs entrypoint with mapped args/flags
- Tool with `auth: token`, `TOKEN_ENV` set → token resolved and injected
- Tool with `auth: none` → no auth resolution
- Stdout/stderr from tool forwarded to Toolwright's stdout/stderr
- Toolwright exits with tool's exit code

### AC-11: run reports auth errors clearly
- Tool requires auth, no token found → error: `tool "{name}" requires authentication. Set {TOKEN_ENV} or run "toolwright login {name}".`
- Exit code 1

### AC-12: test runs scenarios and reports results
- `toolwright test` → finds `*.test.yaml` in `--tests` directory
- Runs all tests, outputs TAP
- `--json` → JSON report per §3.4 format
- `--filter` → regex filters test names
- `--parallel N` → runs N tests concurrently

### AC-13: login initiates OAuth flow
- `toolwright login deploy` → delegates to auth.Login, stores token
- `--no-browser` → prints URL instead of opening browser
- Tool with `auth: token` → error: "login is only available for tools with OAuth authentication"
- Tool with `auth: none` → error: "tool does not require authentication"

### AC-14: generate cli produces Go CLI project
- `toolwright generate cli` → generates Go CLI in `./cli-{name}/`
- `--output /tmp/out` → generates in specified directory
- `--target go` (default) → Go output
- `--target typescript` → error: "TypeScript CLI target is not yet implemented"
- `--dry-run` → lists files, writes nothing

### AC-15: generate mcp produces TypeScript MCP server
- `toolwright generate mcp` → generates TS MCP in `./mcp-server-{name}/`
- `--target typescript` (default) → TypeScript output
- `--transport stdio` → stdio-only server
- `--transport stdio,streamable-http` → dual-transport server
- `--dry-run` → lists files, writes nothing

### AC-16: init scaffolds a working project
- `toolwright init my-tool --yes` → creates `my-tool/` with valid manifest, stub, schema, test
- `toolwright validate` passes in the generated directory
- `toolwright run hello` succeeds in the generated directory
- `toolwright test` passes in the generated directory

### AC-17: init TUI wizard
- Interactive mode (no `--yes`) → Bubble Tea wizard asks name, description, runtime, auth
- `--runtime go` → Go stub instead of shell stub
- `CI=true` → falls back to `--yes` behavior (no TUI)

### AC-18: generate manifest (optional, AI-assisted)
- `toolwright generate manifest --provider anthropic` with `ANTHROPIC_API_KEY` set → generates manifest
- No API key → error: "Set ANTHROPIC_API_KEY or configure in ~/.config/toolwright/config.yaml"
- Generated manifest passes `toolwright validate`
- `--dry-run` → prints to stdout, writes nothing

### AC-19: Commands thin — delegate to domain packages
- CLI command files contain no business logic
- All parsing, validation, execution, testing, generation logic lives in domain packages
- CLI files only: define flags, call domain functions, format output

### AC-20: Binary builds and runs
- `go build -o toolwright ./cmd/toolwright` → produces working binary
- `./toolwright --help` shows all commands
- `./toolwright version` or `--version` shows version info
- `go test ./...` passes (all packages)
- `go vet ./...` clean
