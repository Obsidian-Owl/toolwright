package cli

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// AC-2: UsageError satisfies error interface and unwrap contract
// ---------------------------------------------------------------------------

func TestUsageError_ImplementsErrorInterface(t *testing.T) {
	inner := fmt.Errorf("missing argument")
	var err error = &UsageError{Err: inner}

	// Must be assignable to error (compile-time check via the variable above).
	assert.Equal(t, "missing argument", err.Error(),
		"UsageError.Error() must return the wrapped error's message")
}

func TestUsageError_ErrorMessage_MatchesWrappedError(t *testing.T) {
	tests := []struct {
		name    string
		inner   error
		wantMsg string
	}{
		{
			name:    "simple message",
			inner:   fmt.Errorf("requires tool name"),
			wantMsg: "requires tool name",
		},
		{
			name:    "message with special chars",
			inner:   fmt.Errorf("requires project name: run 'toolwright init <project-name>'"),
			wantMsg: "requires project name: run 'toolwright init <project-name>'",
		},
		{
			name:    "empty message",
			inner:   fmt.Errorf(""),
			wantMsg: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ue := &UsageError{Err: tc.inner}
			assert.Equal(t, tc.wantMsg, ue.Error(),
				"UsageError.Error() must delegate to Err.Error()")
		})
	}
}

func TestUsageError_Unwrap_ReturnsInnerError(t *testing.T) {
	inner := fmt.Errorf("some usage problem")
	ue := &UsageError{Err: inner}

	unwrapped := ue.Unwrap()
	require.NotNil(t, unwrapped,
		"UsageError.Unwrap() must return the inner error, not nil")
	assert.Same(t, inner, unwrapped,
		"UsageError.Unwrap() must return the exact inner error instance")
}

func TestUsageError_ErrorsAs_MatchesDirect(t *testing.T) {
	inner := fmt.Errorf("no tool name provided")
	err := &UsageError{Err: inner}

	var target *UsageError
	ok := errors.As(err, &target)
	require.True(t, ok,
		"errors.As must find *UsageError when err IS a *UsageError")
	assert.Equal(t, inner.Error(), target.Err.Error(),
		"matched UsageError must contain the original inner error message")
}

func TestUsageError_ErrorsAs_MatchesThroughWrapping(t *testing.T) {
	inner := fmt.Errorf("no args")
	usageErr := &UsageError{Err: inner}
	wrapped := fmt.Errorf("command failed: %w", usageErr)

	var target *UsageError
	ok := errors.As(wrapped, &target)
	require.True(t, ok,
		"errors.As must find *UsageError through fmt.Errorf wrapping")
	assert.Equal(t, "no args", target.Err.Error(),
		"matched UsageError must contain the original inner error message")
}

func TestUsageError_ErrorsIs_DoesNotMatchIOError(t *testing.T) {
	usageErr := &UsageError{Err: fmt.Errorf("usage problem")}

	var target *IOError
	ok := errors.As(usageErr, &target)
	assert.False(t, ok,
		"errors.As for *IOError must NOT match a *UsageError")
}

func TestUsageError_DifferentMessages_DifferentOutput(t *testing.T) {
	ue1 := &UsageError{Err: fmt.Errorf("error alpha")}
	ue2 := &UsageError{Err: fmt.Errorf("error beta")}
	assert.NotEqual(t, ue1.Error(), ue2.Error(),
		"different inner errors must produce different Error() output; anti-hardcoding")
}

// ---------------------------------------------------------------------------
// AC-3: IOError satisfies error interface and unwrap contract
// ---------------------------------------------------------------------------

func TestIOError_ImplementsErrorInterface(t *testing.T) {
	inner := fmt.Errorf("file not found")
	var err error = &IOError{Err: inner}

	assert.Equal(t, "file not found", err.Error(),
		"IOError.Error() must return the wrapped error's message")
}

func TestIOError_ErrorMessage_MatchesWrappedError(t *testing.T) {
	tests := []struct {
		name    string
		inner   error
		wantMsg string
	}{
		{
			name:    "file not found",
			inner:   fmt.Errorf("loading manifest /tmp/x.yaml: no such file"),
			wantMsg: "loading manifest /tmp/x.yaml: no such file",
		},
		{
			name:    "permission denied",
			inner:   fmt.Errorf("loading manifest /tmp/x.yaml: permission denied"),
			wantMsg: "loading manifest /tmp/x.yaml: permission denied",
		},
		{
			name:    "empty message",
			inner:   fmt.Errorf(""),
			wantMsg: "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ie := &IOError{Err: tc.inner}
			assert.Equal(t, tc.wantMsg, ie.Error(),
				"IOError.Error() must delegate to Err.Error()")
		})
	}
}

func TestIOError_Unwrap_ReturnsInnerError(t *testing.T) {
	inner := fmt.Errorf("permission denied")
	ie := &IOError{Err: inner}

	unwrapped := ie.Unwrap()
	require.NotNil(t, unwrapped,
		"IOError.Unwrap() must return the inner error, not nil")
	assert.Same(t, inner, unwrapped,
		"IOError.Unwrap() must return the exact inner error instance")
}

func TestIOError_ErrorsAs_MatchesDirect(t *testing.T) {
	inner := fmt.Errorf("cannot read file")
	err := &IOError{Err: inner}

	var target *IOError
	ok := errors.As(err, &target)
	require.True(t, ok,
		"errors.As must find *IOError when err IS a *IOError")
	assert.Equal(t, inner.Error(), target.Err.Error(),
		"matched IOError must contain the original inner error message")
}

func TestIOError_ErrorsAs_MatchesThroughWrapping(t *testing.T) {
	inner := fmt.Errorf("open /tmp/x: no such file or directory")
	ioErr := &IOError{Err: inner}
	wrapped := fmt.Errorf("manifest loading failed: %w", ioErr)

	var target *IOError
	ok := errors.As(wrapped, &target)
	require.True(t, ok,
		"errors.As must find *IOError through fmt.Errorf wrapping")
	assert.Equal(t, inner.Error(), target.Err.Error(),
		"matched IOError must contain the original inner error message")
}

func TestIOError_ErrorsAs_DoesNotMatchUsageError(t *testing.T) {
	ioErr := &IOError{Err: fmt.Errorf("file not found")}

	var target *UsageError
	ok := errors.As(ioErr, &target)
	assert.False(t, ok,
		"errors.As for *UsageError must NOT match a *IOError")
}

func TestIOError_DifferentMessages_DifferentOutput(t *testing.T) {
	ie1 := &IOError{Err: fmt.Errorf("error alpha")}
	ie2 := &IOError{Err: fmt.Errorf("error beta")}
	assert.NotEqual(t, ie1.Error(), ie2.Error(),
		"different inner errors must produce different Error() output; anti-hardcoding")
}

// ---------------------------------------------------------------------------
// AC-2/AC-3/AC-4: ExitCodeForError maps error types to exit codes
// ---------------------------------------------------------------------------

func TestExitCodeForError_TableDriven(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode int
	}{
		{
			name:     "nil error returns ExitSuccess (0)",
			err:      nil,
			wantCode: ExitSuccess,
		},
		{
			name:     "UsageError returns ExitUsage (2)",
			err:      &UsageError{Err: fmt.Errorf("missing arg")},
			wantCode: ExitUsage,
		},
		{
			name:     "IOError returns ExitIO (3)",
			err:      &IOError{Err: fmt.Errorf("file not found")},
			wantCode: ExitIO,
		},
		{
			name:     "plain error returns ExitError (1)",
			err:      fmt.Errorf("something went wrong"),
			wantCode: ExitError,
		},
		{
			name:     "wrapped UsageError returns ExitUsage (2)",
			err:      fmt.Errorf("init failed: %w", &UsageError{Err: fmt.Errorf("no name")}),
			wantCode: ExitUsage,
		},
		{
			name:     "wrapped IOError returns ExitIO (3)",
			err:      fmt.Errorf("run failed: %w", &IOError{Err: fmt.Errorf("no file")}),
			wantCode: ExitIO,
		},
		{
			name:     "double-wrapped UsageError returns ExitUsage (2)",
			err:      fmt.Errorf("outer: %w", fmt.Errorf("inner: %w", &UsageError{Err: fmt.Errorf("deep")})),
			wantCode: ExitUsage,
		},
		{
			name:     "double-wrapped IOError returns ExitIO (3)",
			err:      fmt.Errorf("outer: %w", fmt.Errorf("inner: %w", &IOError{Err: fmt.Errorf("deep")})),
			wantCode: ExitIO,
		},
		{
			name:     "generic wrapped error returns ExitError (1)",
			err:      fmt.Errorf("outer: %w", fmt.Errorf("validation failed")),
			wantCode: ExitError,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := ExitCodeForError(tc.err)
			assert.Equal(t, tc.wantCode, got,
				"ExitCodeForError must return %d for %v", tc.wantCode, tc.err)
		})
	}
}

func TestExitCodeForError_UsageBeatsIO_WhenBothWrapped(t *testing.T) {
	// If somehow both are in the chain, the first match wins.
	// This tests that the function uses a deterministic order.
	usageInner := &UsageError{Err: fmt.Errorf("usage issue")}
	ioOuter := &IOError{Err: usageInner}

	code := ExitCodeForError(ioOuter)
	// It should match IOError first since that's the outermost type.
	// The exact behavior depends on implementation, but it must NOT return ExitError (1).
	assert.NotEqual(t, ExitError, code,
		"ExitCodeForError must not default to ExitError when a typed error is present")
	// It should be either ExitIO (3) or ExitUsage (2), depending on which
	// errors.As finds first.
	assert.Contains(t, []int{ExitUsage, ExitIO}, code,
		"ExitCodeForError must return ExitUsage or ExitIO when both types are in the chain")
}

// Anti-hardcoding: ExitCodeForError must not just return a constant.
func TestExitCodeForError_DifferentErrorTypes_DifferentCodes(t *testing.T) {
	codes := map[int]bool{}
	codes[ExitCodeForError(nil)] = true
	codes[ExitCodeForError(&UsageError{Err: fmt.Errorf("x")})] = true
	codes[ExitCodeForError(&IOError{Err: fmt.Errorf("y")})] = true
	codes[ExitCodeForError(fmt.Errorf("z"))] = true

	assert.Len(t, codes, 4,
		"ExitCodeForError must return 4 distinct codes for nil, UsageError, IOError, and plain error; got %v", codes)
}

// Verify the exact numeric values match the constants.
func TestExitCodeForError_NumericValues(t *testing.T) {
	assert.Equal(t, 0, ExitCodeForError(nil),
		"nil must map to exit code 0")
	assert.Equal(t, 2, ExitCodeForError(&UsageError{Err: fmt.Errorf("x")}),
		"UsageError must map to exit code 2")
	assert.Equal(t, 3, ExitCodeForError(&IOError{Err: fmt.Errorf("x")}),
		"IOError must map to exit code 3")
	assert.Equal(t, 1, ExitCodeForError(fmt.Errorf("x")),
		"plain error must map to exit code 1")
}
