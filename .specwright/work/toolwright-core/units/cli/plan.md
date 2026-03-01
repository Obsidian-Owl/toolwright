# Plan: Unit 6 — cli

## Task Breakdown

### Task 1: Root command and global flags
- File: `internal/cli/root.go`
- Root Cobra command with `--json`, `--debug`, `--no-color` persistent flags
- `NO_COLOR` and `CI=true` env detection
- Global error handler: formats errors per §3.0 (human or JSON)
- Exit code mapping: command errors → 1, usage errors → 2, IO errors → 3

### Task 2: Shared helpers
- File: `internal/cli/helpers.go`
- `loadManifest(path string) (*manifest.Toolkit, error)` — shared by most commands
- `outputJSON(w io.Writer, v any) error` — consistent JSON output
- `outputError(w io.Writer, code, message, hint string, jsonMode bool)` — error formatting
- `isCI() bool` — CI detection

### Task 3: validate command
- File: `internal/cli/validate.go`
- Parse manifest, run `manifest.Validate()`, optionally check entrypoint existence
- `--online`: verify provider URLs reachable (HTTP HEAD)
- Human output: colored pass/fail list. JSON output: §3.2 format.

### Task 4: list command
- File: `internal/cli/list.go`
- Parse manifest, output tool table (name, description, auth_type)
- `--json`: `{"tools": [{name, description, auth_type}]}`

### Task 5: describe command
- File: `internal/cli/describe.go`
- Parse manifest, find tool by name, output JSON Schema
- `--format json` (default): compatible with OpenAI/Anthropic/MCP
- `--format mcp`: MCP inputSchema format
- `--format openai`: OpenAI parameters format

### Task 6: run command
- File: `internal/cli/run.go`
- Parse manifest, find tool, resolve auth via `auth.Resolver`
- Delegate to `runner.Executor`
- Forward stdout/stderr from tool to Toolwright's stdout/stderr
- Exit with tool's exit code

### Task 7: test command
- File: `internal/cli/test.go`
- Parse manifest, parse test files from `--tests` directory
- Filter by `--filter` regex
- Run via `testing.TestRunner` with `--parallel` workers
- Output TAP (default) or JSON (`--json`)

### Task 8: login command
- File: `internal/cli/login.go`
- Find tool, check auth type is oauth2
- Delegate to `auth.Login()`
- `--no-browser`: print URL instead of opening

### Task 9: generate commands
- File: `internal/cli/generate.go`
- `generate cli` → codegen.Engine with mode=cli
- `generate mcp` → codegen.Engine with mode=mcp
- `generate manifest` → generate.ManifestGenerator
- Shared flags: --output, --target, --dry-run, --force
- MCP-specific: --transport

### Task 10: init command + TUI wizard
- File: `internal/cli/init.go`, `internal/tui/wizard.go`
- `--yes`: non-interactive, defaults (shell, no auth)
- Interactive: Bubble Tea wizard asking name, description, runtime, auth
- Scaffold output directory from `templates/init/`
- Generated manifest passes `toolwright validate`

### Task 11: Init templates
- Files: `templates/init/*`
- `toolwright.yaml` — minimal valid manifest
- `bin/hello` — executable shell stub printing `{"message":"hello"}`
- `schemas/hello-output.json` — JSON Schema for hello output
- `tests/hello.test.yaml` — passing test scenario
- `README.md` — basic usage docs

### Task 12: AI manifest generation (optional)
- Files: `internal/generate/*.go`
- `ManifestGenerator` with provider interface
- Anthropic, OpenAI, Gemini providers
- Single LLM call with manifest schema as context
- Validates response, retries once on failure
- Key resolution: env vars → `~/.config/toolwright/config.yaml`

### Task 13: main.go entry point
- File: `cmd/toolwright/main.go`
- Wire root command, execute, exit with code

## File Change Map

| File | Action | Package |
|------|--------|---------|
| `internal/cli/root.go` | Create | cli |
| `internal/cli/helpers.go` | Create | cli |
| `internal/cli/validate.go` | Create | cli |
| `internal/cli/list.go` | Create | cli |
| `internal/cli/describe.go` | Create | cli |
| `internal/cli/run.go` | Create | cli |
| `internal/cli/test.go` | Create | cli |
| `internal/cli/login.go` | Create | cli |
| `internal/cli/generate.go` | Create | cli |
| `internal/cli/init.go` | Create | cli |
| `internal/tui/wizard.go` | Create | tui |
| `internal/tui/wizard_test.go` | Create | tui |
| `internal/generate/manifest.go` | Create | generate |
| `internal/generate/provider.go` | Create | generate |
| `internal/generate/anthropic.go` | Create | generate |
| `internal/generate/openai.go` | Create | generate |
| `internal/generate/gemini.go` | Create | generate |
| `internal/generate/prompt.go` | Create | generate |
| `templates/init/*` | Create | embedded assets |
| `cmd/toolwright/main.go` | Replace | main |
| `go.mod` | Update | root (add cobra, bubbletea, lipgloss) |
