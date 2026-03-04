# Plan: Unit 4 — wiring

## Tasks

### Task 1: Manifest JSON Schema

Create `schemas/toolwright.schema.json`:
- JSON Schema draft 2020-12
- Covers: apiVersion, kind, metadata, auth, tools[], generate
- Metadata: name pattern, version semver, description maxLength 200
- Auth: type enum with conditional required fields per type
- Tools: name, description, entrypoint required; parameters, auth optional

Remove `schemas/.gitkeep`.

### Task 2: Exit code error types

New file `internal/cli/errors.go`:

```go
type UsageError struct{ Err error }
func (e *UsageError) Error() string { return e.Err.Error() }
func (e *UsageError) Unwrap() error { return e.Err }

type IOError struct{ Err error }
func (e *IOError) Error() string { return e.Err.Error() }
func (e *IOError) Unwrap() error { return e.Err }
```

Update commands to wrap errors appropriately:
- Missing positional args → `&UsageError{}`
- File not found / permission denied → `&IOError{}`

Update `cmd/toolwright/main.go`:
```go
var usageErr *cli.UsageError
var ioErr *cli.IOError
switch {
case errors.As(err, &usageErr): os.Exit(cli.ExitUsage)
case errors.As(err, &ioErr):    os.Exit(cli.ExitIO)
default:                        os.Exit(cli.ExitError)
}
```

### Task 3: Debug logging

Add `debugLog(cmd, ...)` calls to existing command functions:
- `runValidate()`: "loading manifest from %s", "structural validation: %d errors", "checking entrypoint: %s", "online check: %s"
- `runTool()`: "loading manifest from %s", "resolving auth for tool %s", "executing: %s", "tool exited with code %d"
- `runTest()`: "parsing test directory: %s", "found %d test suites", "running suite: %s"
- `runLogin()`: "loading manifest from %s", "auth type for %s: %s", "starting OAuth flow"

### Task 4: Production wiring + cleanup

Update `internal/cli/wire.go`:
- Import `internal/scaffold`, `internal/tui`, `internal/generate`, module root
- Replace nil Scaffolder with `scaffold.New(toolwright.InitTemplates)`
- Replace nil Wizard with `tui.NewWizard(isColorDisabled())`
- Replace nil Generator with `generate.NewGenerator()`

Cleanup:
- Remove `WizardResult.Name` from `internal/cli/init.go`
- Remove nil-guard for Wizard (init.go) — real dep now exists
- Remove nil-guard for Scaffolder (init.go) — real dep now exists
- Remove nil-guard for Generator (generate_manifest.go) — real dep now exists
- Update any tests that reference WizardResult.Name

## File Change Map

| File | Action |
|------|--------|
| `schemas/toolwright.schema.json` | CREATE — manifest JSON Schema |
| `schemas/.gitkeep` | DELETE — no longer needed |
| `internal/cli/errors.go` | CREATE — UsageError, IOError types |
| `internal/cli/errors_test.go` | CREATE — error type tests |
| `cmd/toolwright/main.go` | EDIT — exit code switch |
| `internal/cli/wire.go` | EDIT — replace nil deps, add imports |
| `internal/cli/init.go` | EDIT — remove WizardResult.Name, remove nil-guards |
| `internal/cli/generate_manifest.go` | EDIT — remove nil-guard |
| `internal/cli/validate.go` | EDIT — add debugLog calls |
| `internal/cli/run.go` | EDIT — add debugLog calls |
| `internal/cli/test.go` | EDIT — add debugLog calls |
| `internal/cli/login.go` | EDIT — add debugLog calls |
| `internal/cli/init_test.go` | EDIT — update WizardResult references if needed |

## Architecture Decisions

1. **Error types over sentinel errors**: `errors.As()` works across wrapping layers. Sentinels don't carry context.
2. **debugLog calls are minimal**: One per major operation (load, validate, execute). Not per-line tracing.
3. **Schema hand-written**: More precise than generated-from-types. Includes descriptions, examples, patterns.
4. **Nil-guard removal after wiring**: Guards only removed in the same commit that wires real deps.
