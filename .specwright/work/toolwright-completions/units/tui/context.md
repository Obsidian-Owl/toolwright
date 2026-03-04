# Context: Unit 2 — tui

## Purpose

Implement `internal/tui/` package — a Bubble Tea wizard using `charmbracelet/huh` v0.8.0. The wizard collects project configuration for `toolwright init` when run interactively.

## Interface to Implement

From `internal/cli/init.go`:

```go
type initWizard interface {
    Run(ctx context.Context) (*WizardResult, error)
}

type WizardResult struct {
    Name        string  // DEAD FIELD — name comes from CLI positional arg
    Description string
    Runtime     string  // "shell", "go", "python", "typescript"
    Auth        string  // "none", "token", "oauth2"
}
```

**Important**: `WizardResult.Name` is never used by `runInit()` (line 139 ignores it). This unit should NOT remove it — that's a CLI layer change for the wiring unit. The wizard simply does not set it.

## When the Wizard is Called

`runInit()` in `init.go` calls the wizard only when:
- `--yes` flag is NOT set AND
- `isCI()` returns false (CI env var not set)

See `init.go:114,126`. The wizard is never reached in CI.

## huh v0.8.0

Library: `github.com/charmbracelet/huh` v0.8.0 (stable)

### Form Pattern
```go
form := huh.NewForm(
    huh.NewGroup(fields...),  // Group 1 = page 1
    huh.NewGroup(fields...),  // Group 2 = page 2
)
err := form.Run()
```

### Field Types
- `huh.NewInput()` — text input
- `huh.NewSelect[string]()` — dropdown select with typed options
- `huh.NewOption("Label", "value")` — select option

### Accessible Mode
- `form.WithAccessible(true)` — line-by-line text prompts (no TUI renderer)
- Auto-detected only when `TERM=dumb`
- **Does NOT auto-detect CI=true or NO_COLOR** — must pass explicitly
- Accessible mode reads from `io.Reader`, writes to `io.Writer`
- **Does NOT support WithTimeout()** — don't combine

### User Abort
- `huh.ErrUserAborted` — returned when user presses Ctrl+C

### Testing
huh's own tests drive the form via `tea.KeyMsg`:
```go
_, cmd := form.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'f'}})
```
But accessible mode is simpler to test — feed newline-delimited answers via bytes.Buffer.

## Wizard Fields

The wizard collects 3 fields (Name comes from CLI):

| Field | Type | Options | Default |
|-------|------|---------|---------|
| Description | text input | free text | (required) |
| Runtime | select | shell, go, python, typescript | shell |
| Auth | select | none, token, oauth2 | none |

## Key Files

| File | Purpose |
|------|---------|
| `internal/cli/init.go` | initWizard interface, WizardResult type, runInit() |
| `internal/cli/init_test.go` | mockWizard, test expectations |
| `internal/cli/helpers.go` | isColorDisabled() at line 44 — used for accessible flag |

## Gotchas

1. huh accessible mode doesn't support timeouts — don't use `WithTimeout()` with `WithAccessible(true)`
2. `huh.ErrUserAborted` is the sentinel for Ctrl+C — propagate to caller
3. Context cancellation: huh v0.8.0 form.Run() does NOT accept context directly — need to handle via goroutine + channel or accept the limitation
4. Testing: accessible mode + bytes.Buffer is the simplest TTY-free test path
5. The wizard should NOT validate runtime/auth — `runInit()` does that after receiving WizardResult
