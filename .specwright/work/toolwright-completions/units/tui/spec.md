# Spec: Unit 2 — tui

## Acceptance Criteria

### AC-1: Wizard implements initWizard interface
- `tui.Wizard` satisfies `cli.initWizard` interface (compile-time check)
- `NewWizard(accessible bool)` returns a `*Wizard`
- `Run(ctx)` returns `(*cli.WizardResult, error)`

### AC-2: Wizard collects description, runtime, and auth
- Description is collected as free text input
- Runtime is a select with exactly 4 options: shell, go, python, typescript
- Auth is a select with exactly 3 options: none, token, oauth2
- Returned `WizardResult.Description` matches user input
- Returned `WizardResult.Runtime` matches selected option value
- Returned `WizardResult.Auth` matches selected option value

### AC-3: Wizard does not ask for name
- No form field prompts for project name
- `WizardResult.Name` is left as zero value (empty string)

### AC-4: Accessible mode works without TTY
- `NewWizard(true)` creates a wizard that runs in accessible (line-by-line) mode
- Accessible mode reads from stdin and writes to stdout without TUI rendering
- Can be driven programmatically in tests via `bytes.Buffer`

### AC-5: User abort returns error
- Pressing Ctrl+C during the wizard returns an error
- The error is or wraps `huh.ErrUserAborted`
- No partial result is returned on abort

### AC-6: Select fields have correct defaults
- Runtime defaults to "shell" (first option)
- Auth defaults to "none" (first option)
- Description has no default (empty, required input)

### AC-7: huh dependency added correctly
- `go.mod` includes `github.com/charmbracelet/huh` v0.8.0 or compatible
- `go build ./...` succeeds with the new dependency
- No import cycle: `internal/tui/` imports only `internal/cli` types (for WizardResult) and huh
