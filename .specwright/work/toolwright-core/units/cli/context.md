# Context: Unit 6 — cli

## Purpose

Wire everything together with Cobra commands, implement the TUI wizard for `toolwright init`, the AI manifest generation (optional), and the main.go entry point. This unit makes Toolwright a usable binary.

## Key Spec Sections

- §3.0: Global behavior (--json, --debug, --no-color, exit codes, error format)
- §3.1: `toolwright init` — TUI wizard, --yes, --runtime, scaffolding
- §3.2: `toolwright validate` — manifest + schema + auth validation, --json, --online
- §3.3: `toolwright run` — tool execution with auth resolution
- §3.4: `toolwright test` — delegates to testing framework
- §3.5: `toolwright list` — human table and --json
- §3.6: `toolwright describe` — JSON Schema output, --format
- §3.7: `toolwright login` — delegates to auth.Login
- §3.8-3.9: `toolwright generate cli/mcp` — delegates to codegen
- §3.10: `toolwright generate manifest` — AI-assisted (optional)

## Files to Create

```
internal/cli/
├── root.go              # Root command, global flags, error handling
├── init.go              # init subcommand (delegates to tui)
├── validate.go          # validate subcommand
├── run.go               # run subcommand (auth resolver + runner)
├── test.go              # test subcommand (delegates to testing)
├── list.go              # list subcommand
├── describe.go          # describe subcommand
├── login.go             # login subcommand (delegates to auth)
├── generate.go          # generate subcommand (cli, mcp, manifest)
└── helpers.go           # Shared output formatting, error helpers

internal/tui/
├── wizard.go            # Bubble Tea init wizard
└── wizard_test.go

internal/generate/
├── manifest.go          # AI manifest generation orchestration
├── provider.go          # Provider interface
├── anthropic.go         # Anthropic provider
├── openai.go            # OpenAI provider
├── gemini.go            # Gemini provider
└── prompt.go            # Prompt template for manifest generation

templates/init/
├── toolwright.yaml      # Template manifest
├── hello.sh             # Shell stub
├── hello-output.json    # Output schema
├── hello.test.yaml      # Test scenario
└── README.md            # Generated README

cmd/toolwright/main.go   # Entry point (replaces stub from Unit 1)
```

## Dependencies

- `internal/manifest` (Unit 1) — parsing, validation, types
- `internal/schema` (Unit 1) — manifest schema validation
- `internal/auth` (Unit 2) — resolver, login
- `internal/runner` (Unit 3) — tool execution
- `internal/testing` (Unit 4) — test framework
- `internal/codegen` (Unit 5) — code generation
- `charm.land/bubbletea/v2` — TUI wizard
- `charm.land/lipgloss/v2` — TUI styling
- `github.com/spf13/cobra` — CLI framework

## CLI Architecture

Constitution rule 15: CLI commands are thin. Each command file:
1. Defines Cobra command with flags
2. Parses manifest (if needed)
3. Delegates to domain package
4. Formats output (human or JSON)
5. Sets exit code

## Global Behavior

- `--json` on all commands: structured JSON to stdout
- `--debug`: timestamped diagnostics to stderr
- `--no-color` / `NO_COLOR` / `CI=true`: disable color
- Exit codes: 0=success, 1=command failed, 2=usage error, 3=IO error
- Error with `--json`: `{"error": {"code": "...", "message": "...", "hint": "..."}}`

## Init Scaffolding

`toolwright init <name>` creates:
```
<name>/
├── toolwright.yaml       # Minimal valid manifest
├── bin/hello             # Executable stub printing {"message":"hello"}
├── schemas/hello-output.json
├── tests/hello.test.yaml
└── README.md
```

TUI wizard asks: project name, description, runtime (shell/go/python/typescript), auth needs.
`--yes` skips wizard with defaults (shell runtime, no auth).

## Gotchas

1. **validate vs parse** — `validate` checks entrypoint existence (filesystem), parser does not. This is the CLI layer's concern, not manifest package's.
2. **run auth resolution** — CLI resolves token via `auth.Resolver`, then passes to `runner.Executor`. Runner never calls auth directly.
3. **list --json** — returns `{"tools": [...]}` not bare array
4. **describe --format** — json (default), mcp, openai format variants
5. **generate manifest** — optional, requires API key. Error clearly if no key found.
6. **CI mode** — `CI=true` disables TUI, uses `--yes` defaults
