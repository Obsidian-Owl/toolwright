# Design: toolwright-completions

## Overview

Implement the remaining features from the original design vision that were deferred during the initial build. Three new packages, embedded assets, and infrastructure wiring to close all WARN-level gaps from the CLI unit's verification.

## Scope

### New Packages
1. **`internal/scaffold/`** — Project scaffolder implementing `cli.scaffolder` interface
2. **`internal/tui/`** — Bubble Tea wizard implementing `cli.initWizard` interface
3. **`internal/generate/`** — AI manifest generator implementing `cli.manifestGenerator` interface

### Embedded Assets
4. **`templates/init/`** — Scaffolding template files (embedded via `go:embed`)
5. **`schemas/toolwright.schema.json`** — Manifest JSON Schema (embedded via `go:embed`)
6. **`embed.go`** — Extended to embed both schemas and templates

### Infrastructure Wiring
7. **Exit code differentiation** — `main.go` maps error types to exit codes 1/2/3
8. **Debug logging** — Wire `debugLog()` into commands at key diagnostic points
9. **Production wiring** — Replace nil deps in `wire.go` with real implementations
10. **CLI flag additions** — `--model` and `--no-merge` on generate manifest

## Approach

### 1. Scaffolder (`internal/scaffold/`)

New package implementing the existing `scaffolder` interface from `internal/cli/init.go`:

```go
type scaffolder interface {
    Scaffold(ctx context.Context, opts ScaffoldOptions) (*ScaffoldResult, error)
}
```

**Architecture**: The `Scaffolder` struct accepts an `fs.FS` for template access (Constitution 17a). Templates are `text/template` files (.tmpl) embedded from `templates/init/`. Static files (non-.tmpl) are copied verbatim.

**Scaffolded project structure** follows the spec (docs/spec.md §3.1):

```
<n>/
├── toolwright.yaml
├── bin/hello                  # Entrypoint stub
├── schemas/hello-output.json  # Output JSON Schema
├── tests/hello.test.yaml      # Passing test scenario
└── README.md
```

**Runtime-specific behavior**: The `--runtime` flag controls the content of `bin/hello`:
- **shell** (default): Bash script printing `{"message":"hello"}`
- **go**: Bash wrapper calling `go run ./src/hello/main.go "$@"`. Additional file: `src/hello/main.go`
- **python**: Python script printing `{"message":"hello"}`
- **typescript**: Node script printing JSON. Additional file: `package.json`

For non-shell runtimes, additional source files are generated alongside the spec structure. `bin/hello` is always the entrypoint referenced in `toolwright.yaml`, and is always executable.

**Validation contracts**: The scaffolded project must pass:
- `toolwright validate` — valid manifest, entrypoint exists and is executable
- `toolwright run hello` — prints `{"message":"hello"}` and exits 0
- `toolwright test` — test scenario passes (asserts output contains "hello")

**Auth scaffolding**: When `opts.Auth` is `"token"` or `"oauth2"`, the generated `toolwright.yaml` includes the auth block. The hello stub doesn't use auth — it demonstrates the manifest structure.

**Failure handling**: Check for existing directory before writing. Fail fast with clear error if the target directory exists (no silent overwrite). All file writes happen after template rendering — if any template fails, no files are written (render all, then write all).

### 2. TUI Wizard (`internal/tui/`)

New package using `github.com/charmbracelet/huh` v0.8.0 (stable form framework built on Bubble Tea). Implements the existing `initWizard` interface:

```go
type initWizard interface {
    Run(ctx context.Context) (*WizardResult, error)
}
```

**Why huh v0.8.0, not charm.land/huh/v2**: The spec's dependency table lists `charm.land/bubbletea/v2`, but huh v2 is unreleased (PR #609 still open). huh v0.8.0 is the stable release, uses bubbletea v1, and satisfies the Charter's Bubble Tea requirement. We'll migrate when huh v2 ships as stable.

**Form fields** (3 fields, 2 groups):
- Group 1: Description (text input, required)
- Group 2: Runtime (select: shell/go/python/typescript), Auth type (select: none/token/oauth2)

Name comes from the CLI positional arg — the wizard doesn't ask for it. Note: `WizardResult.Name` field is dead code and should be removed.

**Accessibility / CI detection**: The wizard constructor accepts an `accessible bool` parameter. huh only auto-detects `TERM=dumb`; we pass `isColorDisabled()` from `wire.go`. Note that when `CI=true`, `runInit()` already short-circuits to non-interactive mode before the wizard is reached, so accessible mode only matters for `NO_COLOR=1` or `TERM=dumb` without `CI=true`.

**Testing strategy**: Use huh's accessible mode with `bytes.Buffer` as input/output. Accessible mode uses line-by-line prompts driven by `io.Reader`, which is testable without a TTY. Integration test: feed newline-delimited answers, verify `WizardResult` fields.

### 3. AI Manifest Generator (`internal/generate/`)

New package per the spec's module layout (docs/spec.md §4.1). Implements:

```go
type manifestGenerator interface {
    Generate(ctx context.Context, opts ManifestGenerateOptions) (*ManifestGenerateResult, error)
}
```

**Provider abstraction**:

```go
type LLMProvider interface {
    Complete(ctx context.Context, prompt string, model string) (string, error)
    Name() string
    DefaultModel() string
}
```

Three implementations using raw `net/http` (no SDK dependencies):

| Provider | Auth Header | API Endpoint | Default Model |
|----------|------------|--------------|---------------|
| Anthropic | `x-api-key` + `anthropic-version` | `api.anthropic.com/v1/messages` | claude-sonnet-4-20250514 |
| OpenAI | `Authorization: Bearer` | `api.openai.com/v1/chat/completions` | gpt-4o |
| Gemini | API key in query param | `generativelanguage.googleapis.com/v1beta/models/{model}:generateContent` | gemini-2.0-flash |

**Why no SDKs**: Each provider is ~100 lines of HTTP client code. Three SDKs would add significant dependency weight for an optional feature. The trade-off is maintenance cost if APIs change — acceptable for a tool CLI.

**API key resolution**: Read from environment (`ANTHROPIC_API_KEY`, `OPENAI_API_KEY`, `GEMINI_API_KEY`). Keys are never logged, printed, or included in error messages (Constitution rule 23). Provider structs do not store the key in exported fields — read from env at call time.

**YAML extraction from LLM response**: LLMs often wrap YAML in markdown code fences. Extraction strategy:
1. Look for ` ```yaml ... ``` ` fenced block — extract content
2. Look for ` ``` ... ``` ` generic fenced block — extract content
3. Fall back to raw response text
4. Parse with `yaml.Unmarshal` into `manifest.Toolkit`
5. Validate with `manifest.Validate()`

**Retry on failure**: Per spec §3.10, retry once if the first call fails or returns invalid YAML. Two attempts maximum.

**Response safety**: Use `io.LimitReader` on HTTP response body (Constitution rule 26). Limit to 256KB — a manifest should never be larger.

**Context timeout**: The `context.Context` from the command carries the timeout. Providers pass it to `http.NewRequestWithContext`.

**CLI additions needed**: Add `--model` and `--no-merge` flags to `generate_manifest.go` (missing from current implementation, required by spec §3.10):
- `--model`: Override provider default. Added to `ManifestGenerateOptions` as `Model string`.
- `--no-merge`: Fail if output file exists. Added to `ManifestGenerateOptions` as `NoMerge bool`.

### 4. Manifest JSON Schema (`schemas/toolwright.schema.json`)

Hand-written JSON Schema describing the `toolwright.yaml` format. Covers:
- Required top-level fields: `apiVersion`, `kind`, `metadata`, `tools`
- Metadata: name pattern `[a-z0-9-]+`, semver version, description max 200 chars
- Auth: type enum (none/token/oauth2), conditional fields per type
- Tools: entrypoint, parameters (JSON Schema reference), auth override
- Generate: cli/mcp targets and options

The `internal/schema` package already has a `Validator` that reads from `fs.FS`. The schema will be embedded via `embed.Schemas` and consumable by validate and other commands.

### 5. Embedded Assets (`embed.go`)

Extend the module-root `embed.go` to embed both schemas and templates:

```go
package toolwright

import "embed"

//go:embed schemas/*
var Schemas embed.FS

//go:embed all:templates/init
var InitTemplates embed.FS
```

Using `all:templates/init` to include all files recursively (including any that start with `.`). Template files use `.tmpl` extension and are processed through `text/template`. Non-`.tmpl` files are copied verbatim.

### 6. Infrastructure Wiring

**Exit codes** (`cmd/toolwright/main.go`): Define error types in `internal/cli/`:

```go
type UsageError struct{ Err error }    // exit 2
type IOError struct{ Err error }       // exit 3
```

Commands wrap errors with the appropriate type. `main.go` switches on error type:
```go
var usageErr *cli.UsageError
var ioErr *cli.IOError
switch {
case errors.As(err, &usageErr): os.Exit(cli.ExitUsage)
case errors.As(err, &ioErr):    os.Exit(cli.ExitIO)
default:                        os.Exit(cli.ExitError)
}
```

**Debug logging**: Add `debugLog(cmd, format, args...)` calls in:
- `runValidate`: manifest load, structural validation, entrypoint checks, online checks
- `runTool`: manifest load, auth resolution, tool execution
- `runTest`: test dir parsing, suite execution
- `runLogin`: manifest load, auth type resolution, OAuth flow start

**Production wiring** (`wire.go`): Replace nil deps:
```go
// init: wire real scaffolder and wizard.
initCfg := &initConfig{
    Scaffolder: scaffold.New(toolwright.InitTemplates),
    Wizard:     tui.NewWizard(isColorDisabled()),
}

// generate manifest: wire real AI generator.
genCmd.AddCommand(newGenerateManifestCmd(&manifestGenerateConfig{
    Generator: generate.NewGenerator(),
}))
```

## Risk Assessment

| Risk | Severity | Mitigation |
|------|----------|------------|
| huh v0.8.0 is pre-1.0 | Medium | API is stable in practice; single-file consumption. Pin version. |
| LLM API changes break providers | Medium | Raw HTTP with no SDK. Each provider is isolated. Version-pin API endpoints. |
| Template rendering with user input containing `{{` | Low | Template data passed via struct fields, not string interpolation. Static files not processed through template engine. |
| `go:embed` glob not matching subdirectories | Low | Use `all:templates/init` directive for recursive inclusion. |
| Scaffolded project fails validation for some runtime | Medium | Integration tests for each runtime: scaffold → validate → run → test. |
| LLM returns non-YAML or incomplete manifest | Medium | Multi-stage extraction (fenced → raw → parse → validate). Retry once. |

## What This Design Does NOT Change

- No changes to `internal/manifest/`, `internal/auth/`, `internal/runner/`, `internal/tooltest/`, `internal/codegen/`
- No changes to existing CLI command logic (only adding nil-guard removals and debug calls)
- No changes to the test infrastructure or CI pipeline
- No changes to the Constitution or Charter

## Alternatives Considered

### TUI Library
| Option | Pros | Cons | Decision |
|--------|------|------|----------|
| `charmbracelet/huh` v0.8.0 | Form framework, 3-field wizard in ~30 lines, stable | bubbletea v1 not v2, pre-1.0 | **Chosen** |
| Raw `charm.land/bubbletea/v2` | Matches spec dep table exactly | Elm architecture for 3 fields is ~200 lines | Rejected |
| `fmt.Fscanf` / `bufio.Scanner` | Zero deps, simple | Violates Charter ("Bubble Tea" is non-negotiable) | Rejected |

### LLM Integration
| Option | Pros | Cons | Decision |
|--------|------|------|----------|
| Raw `net/http` | No SDK deps, lean binary, full control | ~100 lines per provider, maintenance cost | **Chosen** |
| Official Go SDKs | Easier API, maintained | 3 heavy dep trees for an optional feature | Rejected |
| Anthropic SDK only | One dep, test with real provider | Spec says all 3 | Rejected |

### Template Engine for Scaffolding
| Option | Pros | Cons | Decision |
|--------|------|------|----------|
| `text/template` + embedded files | Maintainable, reviewable, standard | Runtime template errors | **Chosen** |
| Inline const strings (like codegen) | No embed wiring, precedent exists | Scaffolding templates are multi-file; inline strings unmaintainable at scale | Rejected |
| quicktemplate | Compile-time safety | Codegen already uses text/template despite design.md saying quicktemplate. Adds build step. | Rejected |
