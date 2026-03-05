# Context: toolwright-completions

## Existing Interfaces to Implement

### scaffolder (internal/cli/init.go)
```go
type scaffolder interface {
    Scaffold(ctx context.Context, opts ScaffoldOptions) (*ScaffoldResult, error)
}

type ScaffoldOptions struct {
    Name, Description, OutputDir, Runtime, Auth string
}

type ScaffoldResult struct {
    Dir   string
    Files []string
}
```
- Valid runtimes: shell, go, python, typescript (map at init.go:57)
- Valid auth: none, token, oauth2
- `runInit()` validates runtime before calling scaffolder
- Default description: `"A {name} toolkit"` for non-interactive mode

### initWizard (internal/cli/init.go)
```go
type initWizard interface {
    Run(ctx context.Context) (*WizardResult, error)
}

type WizardResult struct {
    Name        string  // DEAD — name comes from positional arg, not wizard
    Description string
    Runtime     string
    Auth        string
}
```
- `WizardResult.Name` is never used by `runInit()` — remove it
- Wizard is called only when `!yes && !isCI()` (init.go:114,126)
- Context carries cancellation — wizard must check it

### manifestGenerator (internal/cli/generate_manifest.go)
```go
type manifestGenerator interface {
    Generate(ctx context.Context, opts ManifestGenerateOptions) (*ManifestGenerateResult, error)
}

type ManifestGenerateOptions struct {
    Provider    string // "anthropic", "openai", "gemini"
    Description string
    OutputPath  string
    DryRun      bool
    // MISSING from current code — must add:
    // Model    string
    // NoMerge  bool
}

type ManifestGenerateResult struct {
    Manifest string
    Provider string
}
```
- Provider validated case-sensitively before reaching Generate()
- Output handling done by `outputManifestResult()` — generator returns YAML string

## Key File Paths

| File | Purpose |
|------|---------|
| `internal/cli/init.go` | scaffolder/wizard interfaces + runInit |
| `internal/cli/init_test.go` | Mock scaffolder/wizard, 40+ tests |
| `internal/cli/generate_manifest.go` | generator interface + runGenerateManifest |
| `internal/cli/generate_manifest_test.go` | Mock generator, 41+ tests |
| `internal/cli/wire.go` | Production dependency wiring (nil deps) |
| `internal/cli/helpers.go` | isColorDisabled(), debugLog(), isCI(), outputJSON |
| `internal/cli/root.go` | ExitSuccess/ExitError/ExitUsage/ExitIO constants |
| `cmd/toolwright/main.go` | Entry point — currently always exit(1) |
| `embed.go` | `//go:embed schemas/*` — extend for templates |
| `schemas/.gitkeep` | Empty placeholder |
| `templates/init/` | Empty directory — populate with templates |
| `internal/schema/validator.go` | Validator accepting fs.FS |
| `internal/manifest/validate.go` | manifest.Validate() for structural checks |
| `docs/spec.md` | Full specification — §3.1 init, §3.10 generate manifest |

## Spec Requirements (docs/spec.md)

### Init (§3.1)
- Scaffold structure: toolwright.yaml, bin/hello, schemas/hello-output.json, tests/hello.test.yaml, README.md
- "Works immediately" — validate, run, test all pass on scaffolded project
- Flags: --yes, --runtime (shell default)
- TUI wizard: Bubble Tea, asks description/runtime/auth

### Generate Manifest (§3.10)
- "Single LLM call with manifest schema as context, validates response, retries once on failure"
- Flags: --provider (required), --model, --output, --no-merge, --dry-run
- Key resolution: env vars, then ~/.config/toolwright/config.yaml
- Keys never logged or written to output

### Exit Codes (§3)
- 0: success
- 1: validation/general error
- 2: usage error (invalid args, missing flags)
- 3: IO error (file not found, permission denied)

### Debug (§3)
- --debug writes timestamped diagnostics to stderr
- Includes: manifest parsing, auth resolution, process execution, template rendering

## Gotchas

1. **go:embed `*` is not recursive** — use `all:templates/init` or named directory
2. **huh CI detection is narrow** — only `TERM=dumb` auto-detects, must pass `WithAccessible(true)` explicitly
3. **huh accessible mode doesn't support timeouts** — don't combine `WithTimeout()` and `WithAccessible(true)`
4. **`text/template` interprets `{{` in data** — pass data via struct fields, never raw string interpolation
5. **LLMs wrap YAML in markdown fences** — extraction must handle ` ```yaml ``` ` blocks
6. **`manifest.Validate()` operates on parsed Toolkit** — parse errors surface before validation
7. **Gemini uses query param auth, not headers** — different from Anthropic/OpenAI pattern
8. **`io.LimitReader` needed on LLM responses** — Constitution rule 26
9. **Existing tests use mock implementations** — real implementations must satisfy the same contracts
10. **Constitution rule 2: no init() functions** — scaffolded templates should also follow this
