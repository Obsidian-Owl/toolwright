# Context: Unit 4 — wiring

## Purpose

Connect all new packages to the CLI, add manifest JSON Schema, implement exit code differentiation, wire debug logging, and clean up dead code. This unit depends on units 1-3 being merged.

## Scope

### 1. Manifest JSON Schema
Create `schemas/toolwright.schema.json` — hand-written JSON Schema describing toolwright.yaml format. Already embedded via `//go:embed schemas/*` in `embed.go`.

### 2. Exit Code Differentiation
Current: `main.go` always exits with `cli.ExitError` (1).
Required (docs/spec.md §3):
- Exit 0: success
- Exit 1: validation/general error
- Exit 2: usage error (invalid args, missing flags)
- Exit 3: IO error (file not found, permission denied)

Constants already defined at `internal/cli/root.go:12-17`:
```go
const (
    ExitSuccess = 0
    ExitError   = 1
    ExitUsage   = 2
    ExitIO      = 3
)
```

Need: error types in `internal/cli/` + switch in `main.go`.

### 3. Debug Logging
`debugLog()` defined at `internal/cli/helpers.go:54-57`:
```go
func debugLog(cmd *cobra.Command, format string, args ...any) {
    debug, _ := cmd.Flags().GetBool("debug")
    if debug {
        fmt.Fprintf(cmd.ErrOrStderr(), "[DEBUG %s] %s\n", time.Now().Format(time.RFC3339), fmt.Sprintf(format, args...))
    }
}
```

Currently never called. Need to add calls in:
- `runValidate()`: manifest load, structural validation, entrypoint checks, online checks
- `runTool()`: manifest load, auth resolution, tool execution
- `runTest()`: test dir parsing, suite execution
- `runLogin()`: manifest load, auth type resolution, OAuth flow

### 4. Production Wiring (wire.go)
Replace nil deps:
```go
// Current:
initCfg := &initConfig{Scaffolder: nil, Wizard: nil}
genCmd.AddCommand(newGenerateManifestCmd(&manifestGenerateConfig{Generator: nil}))

// After:
initCfg := &initConfig{
    Scaffolder: scaffold.New(toolwright.InitTemplates),
    Wizard:     tui.NewWizard(isColorDisabled()),
}
genCmd.AddCommand(newGenerateManifestCmd(&manifestGenerateConfig{
    Generator: generate.NewGenerator(),
}))
```

### 5. Dead Code Cleanup
- Remove `WizardResult.Name` field from `internal/cli/init.go` (dead — never used by runInit)
- Remove nil-guards added as PR review fixes (real deps replace nil)
- Update `internal/cli/wire_test.go` if it exists

## Key Files

| File | Purpose |
|------|---------|
| `schemas/.gitkeep` | DELETE — replace with real schema |
| `schemas/toolwright.schema.json` | CREATE — manifest JSON Schema |
| `internal/cli/errors.go` | CREATE — UsageError, IOError types |
| `cmd/toolwright/main.go` | EDIT — exit code switch |
| `internal/cli/wire.go` | EDIT — replace nil deps |
| `internal/cli/init.go` | EDIT — remove WizardResult.Name, remove nil-guards |
| `internal/cli/generate_manifest.go` | EDIT — remove nil-guard |
| `internal/cli/validate.go` | EDIT — add debugLog calls |
| `internal/cli/run.go` | EDIT — add debugLog calls |
| `internal/cli/test.go` | EDIT — add debugLog calls |
| `internal/cli/login.go` | EDIT — add debugLog calls |
| `internal/cli/helpers.go` | Existing — debugLog, isColorDisabled |
| `internal/manifest/types.go` | Reference — Toolkit struct for JSON Schema |

## Existing Test Contracts

- `init_test.go` uses mockScaffolder/mockWizard — tests should still pass after removing nil-guards and WizardResult.Name
- `generate_manifest_test.go` uses mockManifestGenerator — tests should still pass
- `wire_test.go` (if exists) tests BuildRootCommand() — may need updates for new imports
- `root_test.go` has exit code constant tests — already pass

## Manifest JSON Schema Structure

Based on `internal/manifest/types.go` Toolkit struct:
```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "required": ["apiVersion", "kind", "metadata", "tools"],
  "properties": {
    "apiVersion": { "const": "toolwright/v1" },
    "kind": { "const": "Toolkit" },
    "metadata": { ... },
    "auth": { ... },
    "tools": { ... },
    "generate": { ... }
  }
}
```

## Gotchas

1. Removing WizardResult.Name may break tests that set it — check init_test.go
2. Removing nil-guards must happen AFTER real deps are wired — if wire.go still has nil, panics return
3. `wire.go` needs new imports: `internal/scaffold`, `internal/tui`, `internal/generate`, module root `toolwright`
4. Import cycle risk: `internal/tui` imports `internal/cli` for WizardResult, `internal/cli/wire.go` imports `internal/tui` — this is fine (wire.go is wiring layer)
5. JSON Schema must match current Toolkit struct fields — compare against types.go
6. Exit code types need `Error() string` and `Unwrap() error` methods for `errors.As()` to work
