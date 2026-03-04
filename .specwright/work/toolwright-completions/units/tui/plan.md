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
