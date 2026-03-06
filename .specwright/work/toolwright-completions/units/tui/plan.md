# Plan: Unit 2 — tui

## Tasks

### Task 1: Wizard implementation

New package `internal/tui/`:

```go
// wizard.go
type Wizard struct {
    accessible bool
}

func NewWizard(accessible bool) *Wizard

func (w *Wizard) Run(ctx context.Context) (*cli.WizardResult, error)
```

The Run method:
1. Creates a huh.Form with 2 groups (description input, runtime+auth selects)
2. Sets `WithAccessible(w.accessible)` when accessible is true
3. Calls `form.Run()`
4. Maps collected values to `cli.WizardResult`
5. Returns `huh.ErrUserAborted` on Ctrl+C

### Task 2: Tests

Test via accessible mode (no TTY needed):
- Happy path: provide all inputs, verify WizardResult fields
- User abort: verify error propagation
- Empty description handling
- Verify default selections (shell runtime, none auth)

## File Change Map

| File | Action |
|------|--------|
| `internal/tui/wizard.go` | CREATE — Wizard struct + Run method |
| `internal/tui/wizard_test.go` | CREATE — unit tests |
| `go.mod` | EDIT — add `github.com/charmbracelet/huh` dependency |
| `go.sum` | EDIT — updated by go mod tidy |

## Architecture Decisions

1. **huh v0.8.0 over raw bubbletea**: Form framework handles field rendering, validation, navigation. 3 fields don't justify writing Elm architecture from scratch.
2. **Accessible mode for tests**: Bypasses TUI renderer entirely, uses line-by-line stdin/stdout. Testable with bytes.Buffer.
3. **No context cancellation in form.Run()**: huh v0.8.0 doesn't accept context. User can Ctrl+C (returns ErrUserAborted). Acknowledged limitation.
4. **Wizard does not validate runtime/auth values**: CLI layer validates after wizard returns. Wizard offers fixed select options, so invalid values aren't possible from the TUI itself.

## As-Built Notes

### Plan deviations
- Plan called for a bare `Wizard{accessible bool}` struct; implementation added `in io.Reader` and `out io.Writer` fields (with `WithInput`/`WithOutput` fluent setters) to support testable I/O without TTY. This was necessary to meet AC-4.
- Plan said `form.Run()` — actual implementation uses `form.RunWithContext(ctx)` (huh v0.8.0 supports context via this method, not the base `Run`).
- Plan's "No context cancellation" limitation turned out not to apply: `RunWithContext` exists and was used.

### Implementation decisions
- **lineTracker**: Added to work around huh's accessible mode creating a `bufio.Scanner` per field, which buffers 4 KiB at once from the underlying reader. Limiting reads to 1 byte per call prevents scanner cross-field input starvation.
- **EOF abort detection**: huh's `runAccessible` silently discards errors from field I/O. After `RunWithContext` returns nil, checking `tracker.eofSeen && tracker.newlines < 3` detects premature EOF (user abort simulation). Without this, empty/partial input returns success with empty string values.
- **Error wrapping added in REFACTOR**: Post-build reviewer flagged bare `return nil, err` (constitution rule 4). Wrapped with `fmt.Errorf("wizard: ...")` preserving chain for `errors.Is` callers.
- **go.mod tidy in REFACTOR**: `huh` was marked `// indirect` after initial `go get`; `go mod tidy` promoted it to direct.

### Actual file paths
| File | Status |
|------|--------|
| `internal/tui/wizard.go` | Created (133 → 137 lines after REFACTOR) |
| `internal/tui/wizard_test.go` | Created (35 tests, all pass) |
| `go.mod` | Edited — huh v0.8.0 added as direct dependency |
| `go.sum` | Edited — updated by go mod tidy |

### Commits
- `ca17c36` — RED phase tests
- `a443b92` — GREEN phase implementation
- `e25060e` — REFACTOR (error wrapping, go.mod tidy)
