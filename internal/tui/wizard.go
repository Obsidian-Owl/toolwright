package tui

import (
	"context"
	"fmt"
	"io"

	"github.com/charmbracelet/huh"
)

// WizardResult describes the user's choices from the TUI wizard.
type WizardResult struct {
	Description string
	Runtime     string
	Auth        string
}

// Wizard drives the interactive TUI prompts that collect project metadata.
type Wizard struct {
	accessible bool
	in         io.Reader
	out        io.Writer
}

// NewWizard returns a new Wizard. When accessible is true, huh runs in
// accessible (plain-text) mode suitable for non-TTY environments.
func NewWizard(accessible bool) *Wizard {
	return &Wizard{accessible: accessible}
}

// WithInput sets the reader used as stdin by the huh form.
func (w *Wizard) WithInput(r io.Reader) *Wizard {
	w.in = r
	return w
}

// WithOutput sets the writer used as stdout by the huh form.
func (w *Wizard) WithOutput(wr io.Writer) *Wizard {
	w.out = wr
	return w
}

// lineTracker wraps an io.Reader to return at most one byte per Read call and
// to track how many newlines have been consumed, and whether EOF was reached.
//
// huh's accessible mode creates a new bufio.Scanner per field. Scanners buffer
// read from the underlying reader in 4 KiB chunks, which means the first
// scanner consumes the entire input string, starving subsequent fields.
// Limiting reads to one byte at a time prevents this buffering problem.
//
// We also count newlines so that after the form runs we can detect whether EOF
// was reached before all three expected newlines (one per field) were read.
// That condition indicates the user aborted (partial or empty input).
//
// lineTracker is only used in accessible mode; it must not be injected in
// bubbletea (non-accessible) mode where WithInput carries raw terminal events.
type lineTracker struct {
	r        io.Reader
	newlines int
	eofSeen  bool
}

func (t *lineTracker) Read(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	n, err := t.r.Read(p[:1])
	if n > 0 && p[0] == '\n' {
		t.newlines++
	}
	if err == io.EOF {
		t.eofSeen = true
	}
	return n, err
}

// Run presents the wizard prompts and returns the collected values.
// It respects ctx cancellation and propagates huh.ErrUserAborted as-is.
func (w *Wizard) Run(ctx context.Context) (*WizardResult, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("wizard: context already cancelled: %w", err)
	}

	var description, runtime, auth string

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewInput().
				Title("Description").
				Value(&description),
		),
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Runtime").
				Options(
					huh.NewOption("Shell", "shell"),
					huh.NewOption("Go", "go"),
					huh.NewOption("Python", "python"),
					huh.NewOption("TypeScript", "typescript"),
				).
				Value(&runtime),
			huh.NewSelect[string]().
				Title("Auth").
				Options(
					huh.NewOption("None", "none"),
					huh.NewOption("Token", "token"),
					huh.NewOption("OAuth2", "oauth2"),
				).
				Value(&auth),
		),
	)

	form.WithAccessible(w.accessible)

	// numFields is the number of line-terminated inputs expected by the form:
	// one per field (Description input, Runtime select, Auth select).
	// Update this constant if the form gains or loses fields.
	const numFields = 3 // Description + Runtime + Auth

	// lineTracker is only applicable in accessible mode, where huh uses
	// bufio.Scanner for line-oriented reads. In non-accessible (bubbletea)
	// mode WithInput carries raw terminal event bytes; wrapping with
	// lineTracker there would garble input.
	var tracker *lineTracker
	if w.accessible && w.in != nil {
		tracker = &lineTracker{r: w.in}
		form.WithInput(tracker)
	} else if w.in != nil {
		form.WithInput(w.in)
	}
	if w.out != nil {
		form.WithOutput(w.out)
	}

	if err := form.RunWithContext(ctx); err != nil {
		return nil, fmt.Errorf("wizard: form run: %w", err)
	}

	// huh's runAccessible silently swallows EOF and returns nil. Detect the
	// case where the reader ran out of input before all fields were filled.
	if tracker != nil && tracker.eofSeen && tracker.newlines < numFields {
		return nil, fmt.Errorf("wizard: form run: %w", huh.ErrUserAborted)
	}

	// Defence-in-depth: select fields should always be non-empty after a
	// successful form run (huh constrains them to the provided options).
	// Guard here catches any future huh behaviour change that might silently
	// return empty values.
	if runtime == "" || auth == "" {
		return nil, fmt.Errorf("wizard: form run: %w", huh.ErrUserAborted)
	}

	return &WizardResult{
		Description: description,
		Runtime:     runtime,
		Auth:        auth,
	}, nil
}
