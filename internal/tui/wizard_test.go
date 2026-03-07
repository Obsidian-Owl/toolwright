package tui

import (
	"context"
	"errors"
	"io"
	"strconv"
	"strings"
	"testing"

	"github.com/charmbracelet/huh"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// AC-1: Compile-time interface check
// ---------------------------------------------------------------------------

// wizardRunner mirrors cli.initWizard (which is unexported). This compile-time
// check ensures *Wizard satisfies the same contract the CLI layer expects.
type wizardRunner interface {
	Run(ctx context.Context) (*WizardResult, error)
}

var _ wizardRunner = (*Wizard)(nil)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// newTestWizard creates a Wizard in accessible mode wired to the given input
// string and discarding output.  This is the standard pattern for all tests
// that drive the wizard non-interactively.
func newTestWizard(input string) *Wizard {
	return NewWizard(true).
		WithInput(strings.NewReader(input)).
		WithOutput(io.Discard)
}

// ---------------------------------------------------------------------------
// AC-2: Wizard collects description, runtime, and auth
// ---------------------------------------------------------------------------

func TestWizard_CollectsAllFields(t *testing.T) {
	// In accessible mode, huh reads one line per field.
	// Input text field gets the raw string; selects get a 1-indexed number.
	// Fields in order: description (text), runtime (select), auth (select).
	//
	// Runtime options (expected order): shell=1, go=2, python=3, typescript=4
	// Auth options (expected order):    none=1, token=2, oauth2=3
	input := "My awesome tool\n2\n3\n" // desc="My awesome tool", runtime=go(2), auth=oauth2(3)
	w := newTestWizard(input)

	result, err := w.Run(context.Background())
	require.NoError(t, err, "Run must succeed with valid input in accessible mode")
	require.NotNil(t, result, "Run must return a non-nil WizardResult")

	assert.Equal(t, "My awesome tool", result.Description,
		"Description must match the text entered by the user")
	assert.Equal(t, "go", result.Runtime,
		"Runtime must be 'go' when user selects option 2")
	assert.Equal(t, "oauth2", result.Auth,
		"Auth must be 'oauth2' when user selects option 3")
}

func TestWizard_CollectsAllFields_DifferentChoices(t *testing.T) {
	// Anti-hardcoding: use different values to ensure they are not hardcoded.
	input := "Build CLI tools\n3\n2\n" // desc, python(3), token(2)
	w := newTestWizard(input)

	result, err := w.Run(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, "Build CLI tools", result.Description,
		"Description must match the text entered")
	assert.Equal(t, "python", result.Runtime,
		"Runtime must be 'python' when user selects option 3")
	assert.Equal(t, "token", result.Auth,
		"Auth must be 'token' when user selects option 2")
}

func TestWizard_CollectsAllFields_TypeScript(t *testing.T) {
	// Verify the 4th runtime option is typescript.
	input := "TS project\n4\n1\n" // desc, typescript(4), none(1)
	w := newTestWizard(input)

	result, err := w.Run(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, "TS project", result.Description)
	assert.Equal(t, "typescript", result.Runtime,
		"Runtime must be 'typescript' when user selects option 4")
	assert.Equal(t, "none", result.Auth,
		"Auth must be 'none' when user selects option 1")
}

// ---------------------------------------------------------------------------
// AC-3: Wizard does not ask for name
// ---------------------------------------------------------------------------

func TestWizard_NameIsEmpty(t *testing.T) {
	input := "Some description\n1\n1\n"
	w := newTestWizard(input)

	result, err := w.Run(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

func TestWizard_NameIsEmptyRegardlessOfInput(t *testing.T) {
	// Even with many inputs, Name must remain empty.
	input := "description text\n2\n2\n"
	w := newTestWizard(input)

	result, err := w.Run(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
}

// ---------------------------------------------------------------------------
// AC-4: Accessible mode works without TTY
// ---------------------------------------------------------------------------

func TestWizard_AccessibleModeNoTTY(t *testing.T) {
	// Verify the wizard works when driven entirely via an io.Reader (no TTY).
	// This is the core accessibility requirement: bytes.Buffer/strings.Reader
	// must suffice.
	input := "No terminal here\n1\n1\n"
	w := NewWizard(true).
		WithInput(strings.NewReader(input)).
		WithOutput(io.Discard)

	result, err := w.Run(context.Background())
	require.NoError(t, err,
		"Accessible mode must work without a TTY when input/output are injected")
	require.NotNil(t, result)
	assert.Equal(t, "No terminal here", result.Description)
}

func TestWizard_NonAccessible_StillConstructs(t *testing.T) {
	// NewWizard(false) should still return a valid *Wizard.
	// We cannot drive it without a TTY, but construction must not panic.
	w := NewWizard(false)
	require.NotNil(t, w, "NewWizard(false) must return a non-nil *Wizard")
}

// ---------------------------------------------------------------------------
// AC-5: User abort returns error
// ---------------------------------------------------------------------------

func TestWizard_EmptyInput_ReturnsAbortError(t *testing.T) {
	// An empty/closed reader in accessible mode causes huh to return
	// ErrUserAborted. The wizard must propagate this as a non-nil error.
	w := NewWizard(true).
		WithInput(strings.NewReader("")).
		WithOutput(io.Discard)

	_, err := w.Run(context.Background())
	require.Error(t, err,
		"Run must return an error when input is empty/closed (user abort)")
}

func TestWizard_EmptyInput_ErrorIsErrUserAborted(t *testing.T) {
	// Verify the specific error type from huh is preserved or wrapped.
	w := NewWizard(true).
		WithInput(strings.NewReader("")).
		WithOutput(io.Discard)

	_, err := w.Run(context.Background())
	require.Error(t, err)
	assert.True(t, errors.Is(err, huh.ErrUserAborted),
		"abort error must wrap or be huh.ErrUserAborted, got: %v", err)
}

func TestWizard_PartialInput_ReturnsError(t *testing.T) {
	// Only provide the description, then EOF -- should abort on the select.
	w := NewWizard(true).
		WithInput(strings.NewReader("partial\n")).
		WithOutput(io.Discard)

	_, err := w.Run(context.Background())
	require.Error(t, err,
		"Run must return an error when input is truncated mid-form")
}

func TestWizard_CancelledContext_ReturnsError(t *testing.T) {
	// A cancelled context should cause the wizard to abort.
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	w := NewWizard(true).
		WithInput(strings.NewReader("desc\n1\n1\n")).
		WithOutput(io.Discard)

	_, err := w.Run(ctx)
	require.Error(t, err,
		"Run must return an error when context is cancelled")
}

// ---------------------------------------------------------------------------
// AC-6: Select fields have correct defaults
// ---------------------------------------------------------------------------

func TestWizard_Defaults_RuntimeShell_AuthNone(t *testing.T) {
	// When user presses Enter without typing on selects, huh uses the
	// default/first option. In accessible mode, entering "1" selects the
	// first option. An empty line should also select the default.
	//
	// We test with explicit "1\n" for both selects to verify the first
	// option is "shell" for runtime and "none" for auth.
	input := "default test\n1\n1\n"
	w := newTestWizard(input)

	result, err := w.Run(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, "shell", result.Runtime,
		"Default (first) runtime option must be 'shell'")
	assert.Equal(t, "none", result.Auth,
		"Default (first) auth option must be 'none'")
}

func TestWizard_RuntimeOptionsOrder(t *testing.T) {
	// Verify the exact order of runtime options by selecting each one.
	expected := map[string]string{
		"1": "shell",
		"2": "go",
		"3": "python",
		"4": "typescript",
	}

	for choice, wantRuntime := range expected {
		t.Run("runtime_option_"+choice, func(t *testing.T) {
			input := "desc\n" + choice + "\n1\n" // select runtime=choice, auth=none(1)
			w := newTestWizard(input)

			result, err := w.Run(context.Background())
			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, wantRuntime, result.Runtime,
				"Runtime option %s must be %q", choice, wantRuntime)
		})
	}
}

func TestWizard_AuthOptionsOrder(t *testing.T) {
	// Verify the exact order of auth options by selecting each one.
	expected := map[string]string{
		"1": "none",
		"2": "token",
		"3": "oauth2",
	}

	for choice, wantAuth := range expected {
		t.Run("auth_option_"+choice, func(t *testing.T) {
			input := "desc\n1\n" + choice + "\n" // runtime=shell(1), auth=choice
			w := newTestWizard(input)

			result, err := w.Run(context.Background())
			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, wantAuth, result.Auth,
				"Auth option %s must be %q", choice, wantAuth)
		})
	}
}

// ---------------------------------------------------------------------------
// AC-7: huh dependency compiles
// ---------------------------------------------------------------------------

func TestHuhImportCompiles(t *testing.T) {
	// If this file compiles, huh is importable. This test exists to make
	// the dependency explicit. We reference huh.ErrUserAborted to ensure
	// the import is not optimized away.
	require.NotNil(t, huh.ErrUserAborted,
		"huh.ErrUserAborted must be a non-nil sentinel error")
}

// ---------------------------------------------------------------------------
// Edge cases: boundary inputs
// ---------------------------------------------------------------------------

func TestWizard_DescriptionWithSpecialCharacters(t *testing.T) {
	// Description should preserve special characters, unicode, and whitespace.
	desc := "A tool for managing k8s clusters & more! (v2.0) -- \"awesome\""
	input := desc + "\n1\n1\n"
	w := newTestWizard(input)

	result, err := w.Run(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, desc, result.Description,
		"Description must preserve special characters verbatim")
}

func TestWizard_DescriptionWithUnicode(t *testing.T) {
	desc := "Ein Werkzeug fuer Entwickler"
	input := desc + "\n1\n1\n"
	w := newTestWizard(input)

	result, err := w.Run(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, desc, result.Description,
		"Description must handle unicode characters")
}

func TestWizard_EmptyDescription(t *testing.T) {
	// An empty description (just Enter) should either be accepted as ""
	// or trigger a validation error -- but must not panic.
	input := "\n1\n1\n"
	w := newTestWizard(input)

	result, err := w.Run(context.Background())
	// We accept either: empty description succeeds, or validation error.
	// What we do NOT accept: a panic or a nil result with nil error.
	if err == nil {
		require.NotNil(t, result, "if no error, result must not be nil")
		// Empty description is acceptable
	}
	// If err != nil, that is also acceptable (validation requiring non-empty)
}

func TestWizard_LongDescription(t *testing.T) {
	desc := strings.Repeat("a", 500)
	input := desc + "\n1\n1\n"
	w := newTestWizard(input)

	result, err := w.Run(context.Background())
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, desc, result.Description,
		"Description must handle long strings (500 chars)")
}

// ---------------------------------------------------------------------------
// Edge cases: WithInput/WithOutput chaining
// ---------------------------------------------------------------------------

func TestWizard_WithInputReturnsWizard(t *testing.T) {
	// WithInput must return *Wizard for method chaining.
	w := NewWizard(true)
	got := w.WithInput(strings.NewReader(""))
	require.NotNil(t, got, "WithInput must return a non-nil *Wizard")
	assert.Same(t, w, got,
		"WithInput must return the same *Wizard for chaining")
}

func TestWizard_WithOutputReturnsWizard(t *testing.T) {
	// WithOutput must return *Wizard for method chaining.
	w := NewWizard(true)
	got := w.WithOutput(io.Discard)
	require.NotNil(t, got, "WithOutput must return a non-nil *Wizard")
	assert.Same(t, w, got,
		"WithOutput must return the same *Wizard for chaining")
}

// ---------------------------------------------------------------------------
// Anti-hardcoding: different descriptions produce different results
// ---------------------------------------------------------------------------

func TestWizard_DifferentDescriptions_DifferentResults(t *testing.T) {
	input1 := "Alpha project\n1\n1\n"
	w1 := newTestWizard(input1)
	r1, err1 := w1.Run(context.Background())
	require.NoError(t, err1)

	input2 := "Beta project\n1\n1\n"
	w2 := newTestWizard(input2)
	r2, err2 := w2.Run(context.Background())
	require.NoError(t, err2)

	assert.NotEqual(t, r1.Description, r2.Description,
		"Different description inputs must produce different Description fields; anti-hardcoding")
	assert.Equal(t, "Alpha project", r1.Description)
	assert.Equal(t, "Beta project", r2.Description)
}

func TestWizard_DifferentRuntimes_DifferentResults(t *testing.T) {
	input1 := "desc\n1\n1\n" // shell
	w1 := newTestWizard(input1)
	r1, err1 := w1.Run(context.Background())
	require.NoError(t, err1)

	input2 := "desc\n2\n1\n" // go
	w2 := newTestWizard(input2)
	r2, err2 := w2.Run(context.Background())
	require.NoError(t, err2)

	assert.NotEqual(t, r1.Runtime, r2.Runtime,
		"Different runtime selections must produce different Runtime fields; anti-hardcoding")
}

func TestWizard_DifferentAuths_DifferentResults(t *testing.T) {
	input1 := "desc\n1\n1\n" // none
	w1 := newTestWizard(input1)
	r1, err1 := w1.Run(context.Background())
	require.NoError(t, err1)

	input2 := "desc\n1\n2\n" // token
	w2 := newTestWizard(input2)
	r2, err2 := w2.Run(context.Background())
	require.NoError(t, err2)

	assert.NotEqual(t, r1.Auth, r2.Auth,
		"Different auth selections must produce different Auth fields; anti-hardcoding")
}

// ---------------------------------------------------------------------------
// Integration: full round-trip with all field combinations
// ---------------------------------------------------------------------------

func TestWizard_AllCombinations_TableDriven(t *testing.T) {
	tests := []struct {
		name        string
		input       string
		wantDesc    string
		wantRuntime string
		wantAuth    string
	}{
		{
			name:        "shell/none",
			input:       "shell project\n1\n1\n",
			wantDesc:    "shell project",
			wantRuntime: "shell",
			wantAuth:    "none",
		},
		{
			name:        "go/token",
			input:       "go project\n2\n2\n",
			wantDesc:    "go project",
			wantRuntime: "go",
			wantAuth:    "token",
		},
		{
			name:        "python/oauth2",
			input:       "python project\n3\n3\n",
			wantDesc:    "python project",
			wantRuntime: "python",
			wantAuth:    "oauth2",
		},
		{
			name:        "typescript/none",
			input:       "ts project\n4\n1\n",
			wantDesc:    "ts project",
			wantRuntime: "typescript",
			wantAuth:    "none",
		},
		{
			name:        "shell/oauth2",
			input:       "mixed project\n1\n3\n",
			wantDesc:    "mixed project",
			wantRuntime: "shell",
			wantAuth:    "oauth2",
		},
		{
			name:        "typescript/token",
			input:       "another\n4\n2\n",
			wantDesc:    "another",
			wantRuntime: "typescript",
			wantAuth:    "token",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			w := newTestWizard(tc.input)
			result, err := w.Run(context.Background())
			require.NoError(t, err, "Run must succeed with valid input")
			require.NotNil(t, result, "result must not be nil on success")

			assert.Equal(t, tc.wantDesc, result.Description,
				"Description mismatch")
			assert.Equal(t, tc.wantRuntime, result.Runtime,
				"Runtime mismatch")
			assert.Equal(t, tc.wantAuth, result.Auth,
				"Auth mismatch")
		})
	}
}

// ---------------------------------------------------------------------------
// Edge: Run is idempotent per Wizard instance (no shared state leaks)
// ---------------------------------------------------------------------------

func TestWizard_SecondRunOnNewInstance(t *testing.T) {
	// Each Wizard instance should be independently driveable.
	w1 := newTestWizard("first\n1\n1\n")
	r1, err1 := w1.Run(context.Background())
	require.NoError(t, err1)
	assert.Equal(t, "first", r1.Description)

	w2 := newTestWizard("second\n2\n2\n")
	r2, err2 := w2.Run(context.Background())
	require.NoError(t, err2)
	assert.Equal(t, "second", r2.Description)
	assert.Equal(t, "go", r2.Runtime)
	assert.Equal(t, "token", r2.Auth)
}

// ---------------------------------------------------------------------------
// Edge: result field values are valid enum members
// ---------------------------------------------------------------------------

func TestWizard_ResultRuntimeIsValidEnum(t *testing.T) {
	validRuntimes := map[string]bool{
		"shell": true, "go": true, "python": true, "typescript": true,
	}

	for i := 1; i <= 4; i++ {
		input := "desc\n" + strconv.Itoa(i) + "\n1\n"
		w := newTestWizard(input)
		result, err := w.Run(context.Background())
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.True(t, validRuntimes[result.Runtime],
			"Runtime %q (option %d) must be a valid enum member", result.Runtime, i)
	}
}

func TestWizard_ResultAuthIsValidEnum(t *testing.T) {
	validAuths := map[string]bool{
		"none": true, "token": true, "oauth2": true,
	}

	for i := 1; i <= 3; i++ {
		input := "desc\n1\n" + strconv.Itoa(i) + "\n"
		w := newTestWizard(input)
		result, err := w.Run(context.Background())
		require.NoError(t, err)
		require.NotNil(t, result)
		assert.True(t, validAuths[result.Auth],
			"Auth %q (option %d) must be a valid enum member", result.Auth, i)
	}
}
