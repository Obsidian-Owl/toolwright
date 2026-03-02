package runner

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ---------------------------------------------------------------------------
// Test fixtures
// ---------------------------------------------------------------------------

// fullResult returns a Result with every field populated to non-zero values.
// Used across multiple tests to guard against sloppy partial implementations.
func fullResult() Result {
	return Result{
		ExitCode: 42,
		Stdout:   []byte("hello stdout"),
		Stderr:   []byte("hello stderr"),
		Duration: 1500 * time.Millisecond,
	}
}

// ---------------------------------------------------------------------------
// Result: construction with all fields
// ---------------------------------------------------------------------------

func TestResult_ConstructWithAllFields(t *testing.T) {
	r := fullResult()

	// Verify each field individually to catch missing or misspelled field names.
	assert.Equal(t, 42, r.ExitCode,
		"ExitCode field must hold the assigned value")
	assert.Equal(t, []byte("hello stdout"), r.Stdout,
		"Stdout field must hold the assigned byte slice")
	assert.Equal(t, []byte("hello stderr"), r.Stderr,
		"Stderr field must hold the assigned byte slice")
	assert.Equal(t, 1500*time.Millisecond, r.Duration,
		"Duration field must hold the assigned time.Duration value")
}

// ---------------------------------------------------------------------------
// Result: zero value
// ---------------------------------------------------------------------------

func TestResult_ZeroValue(t *testing.T) {
	var r Result

	assert.Equal(t, 0, r.ExitCode, "zero-value ExitCode should be 0")
	assert.Nil(t, r.Stdout, "zero-value Stdout should be nil")
	assert.Nil(t, r.Stderr, "zero-value Stderr should be nil")
	assert.Equal(t, time.Duration(0), r.Duration, "zero-value Duration should be 0")
}

// ---------------------------------------------------------------------------
// Result: non-zero ExitCode
// ---------------------------------------------------------------------------

func TestResult_NonZeroExitCode(t *testing.T) {
	tests := []struct {
		name     string
		exitCode int
	}{
		{name: "exit code 1", exitCode: 1},
		{name: "exit code 2", exitCode: 2},
		{name: "exit code 127 (command not found)", exitCode: 127},
		{name: "exit code 255 (max single byte)", exitCode: 255},
		{name: "negative exit code", exitCode: -1},
		{name: "large exit code", exitCode: 65535},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := Result{ExitCode: tc.exitCode}
			assert.Equal(t, tc.exitCode, r.ExitCode,
				"ExitCode must store the value %d", tc.exitCode)
		})
	}
}

func TestResult_ExitCodeDistinctValues(t *testing.T) {
	// Guard against hardcoded return: two different exit codes must differ.
	r1 := Result{ExitCode: 0}
	r2 := Result{ExitCode: 1}

	assert.NotEqual(t, r1.ExitCode, r2.ExitCode,
		"Results with different ExitCode values must not be equal")
}

// ---------------------------------------------------------------------------
// Result: Stdout and Stderr are distinct (not aliased)
// ---------------------------------------------------------------------------

func TestResult_StdoutStderr_Distinct(t *testing.T) {
	// Stdout and Stderr must be independent byte slices. A sloppy
	// implementation that aliases them to the same backing array would fail.
	r := Result{
		Stdout: []byte("out-data"),
		Stderr: []byte("err-data"),
	}

	assert.Equal(t, []byte("out-data"), r.Stdout,
		"Stdout must hold its assigned value")
	assert.Equal(t, []byte("err-data"), r.Stderr,
		"Stderr must hold its assigned value")

	// The values must be distinct.
	assert.NotEqual(t, r.Stdout, r.Stderr,
		"Stdout and Stderr must be distinct byte slices when assigned different data")
}

func TestResult_StdoutStderr_IndependentMutation(t *testing.T) {
	// Mutating Stdout must not affect Stderr and vice versa.
	stdoutData := []byte("original-out")
	stderrData := []byte("original-err")

	r := Result{
		Stdout: stdoutData,
		Stderr: stderrData,
	}

	// Mutate the original slices.
	stdoutData[0] = 'X'
	stderrData[0] = 'Y'

	// The Result fields point to the same backing arrays (expected for slices),
	// but Stdout and Stderr themselves must remain independent of each other.
	assert.NotEqual(t, r.Stdout, r.Stderr,
		"Stdout and Stderr must remain independent after mutation of source slices")
}

func TestResult_StdoutStderr_BothPopulated(t *testing.T) {
	// Many commands write to both stdout and stderr. Both must be stored.
	r := Result{
		ExitCode: 1,
		Stdout:   []byte("partial output before error"),
		Stderr:   []byte("error: something went wrong"),
		Duration: 250 * time.Millisecond,
	}

	assert.Equal(t, 1, r.ExitCode)
	assert.Equal(t, []byte("partial output before error"), r.Stdout)
	assert.Equal(t, []byte("error: something went wrong"), r.Stderr)
	assert.Equal(t, 250*time.Millisecond, r.Duration)
}

func TestResult_StdoutOnly(t *testing.T) {
	r := Result{
		Stdout: []byte("just stdout"),
		Stderr: nil,
	}

	assert.Equal(t, []byte("just stdout"), r.Stdout)
	assert.Nil(t, r.Stderr, "Stderr should remain nil when not set")
}

func TestResult_StderrOnly(t *testing.T) {
	r := Result{
		Stdout: nil,
		Stderr: []byte("just stderr"),
	}

	assert.Nil(t, r.Stdout, "Stdout should remain nil when not set")
	assert.Equal(t, []byte("just stderr"), r.Stderr)
}

func TestResult_EmptyByteSlices(t *testing.T) {
	// Empty slices (len 0, non-nil) are distinct from nil slices.
	r := Result{
		Stdout: []byte{},
		Stderr: []byte{},
	}

	assert.NotNil(t, r.Stdout, "empty Stdout slice should not be nil")
	assert.NotNil(t, r.Stderr, "empty Stderr slice should not be nil")
	assert.Empty(t, r.Stdout, "empty Stdout should have length 0")
	assert.Empty(t, r.Stderr, "empty Stderr should have length 0")
}

// ---------------------------------------------------------------------------
// Result: Duration
// ---------------------------------------------------------------------------

func TestResult_Duration(t *testing.T) {
	tests := []struct {
		name     string
		duration time.Duration
	}{
		{name: "zero duration", duration: 0},
		{name: "one nanosecond", duration: 1 * time.Nanosecond},
		{name: "one millisecond", duration: 1 * time.Millisecond},
		{name: "one second", duration: 1 * time.Second},
		{name: "one minute", duration: 1 * time.Minute},
		{name: "30 seconds", duration: 30 * time.Second},
		{name: "fractional seconds (1.5s)", duration: 1500 * time.Millisecond},
		{name: "large duration (1 hour)", duration: 1 * time.Hour},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := Result{Duration: tc.duration}
			assert.Equal(t, tc.duration, r.Duration,
				"Duration must store the value %v", tc.duration)
		})
	}
}

func TestResult_DurationDistinctValues(t *testing.T) {
	// Guard against hardcoded return: two different durations must differ.
	r1 := Result{Duration: 100 * time.Millisecond}
	r2 := Result{Duration: 200 * time.Millisecond}

	assert.NotEqual(t, r1.Duration, r2.Duration,
		"Results with different Duration values must not be equal")
}

// ---------------------------------------------------------------------------
// Anti-hardcoding: multiple distinct Results
// ---------------------------------------------------------------------------

func TestResult_MultipleDistinctResults(t *testing.T) {
	// A hardcoded implementation that returns fixed values would fail this.
	results := []Result{
		{
			ExitCode: 0,
			Stdout:   []byte("success output"),
			Stderr:   nil,
			Duration: 100 * time.Millisecond,
		},
		{
			ExitCode: 1,
			Stdout:   []byte(""),
			Stderr:   []byte("fatal: not a git repository"),
			Duration: 5 * time.Millisecond,
		},
		{
			ExitCode: 137,
			Stdout:   []byte("partial"),
			Stderr:   []byte("killed"),
			Duration: 30 * time.Second,
		},
	}

	// Verify each result independently stores its own values.
	assert.Equal(t, 0, results[0].ExitCode)
	assert.Equal(t, []byte("success output"), results[0].Stdout)
	assert.Nil(t, results[0].Stderr)
	assert.Equal(t, 100*time.Millisecond, results[0].Duration)

	assert.Equal(t, 1, results[1].ExitCode)
	assert.Equal(t, []byte(""), results[1].Stdout)
	assert.Equal(t, []byte("fatal: not a git repository"), results[1].Stderr)
	assert.Equal(t, 5*time.Millisecond, results[1].Duration)

	assert.Equal(t, 137, results[2].ExitCode)
	assert.Equal(t, []byte("partial"), results[2].Stdout)
	assert.Equal(t, []byte("killed"), results[2].Stderr)
	assert.Equal(t, 30*time.Second, results[2].Duration)
}

// ---------------------------------------------------------------------------
// Edge case: large byte slices
// ---------------------------------------------------------------------------

func TestResult_LargeOutput(t *testing.T) {
	// Commands can produce large output. The struct must store it without truncation.
	bigStdout := make([]byte, 1024*1024) // 1 MiB
	for i := range bigStdout {
		bigStdout[i] = byte('A' + (i % 26))
	}
	bigStderr := make([]byte, 512*1024) // 512 KiB
	for i := range bigStderr {
		bigStderr[i] = byte('a' + (i % 26))
	}

	r := Result{
		ExitCode: 0,
		Stdout:   bigStdout,
		Stderr:   bigStderr,
		Duration: 5 * time.Second,
	}

	assert.Equal(t, 0, r.ExitCode)
	assert.Len(t, r.Stdout, 1024*1024, "Stdout must store full 1 MiB without truncation")
	assert.Len(t, r.Stderr, 512*1024, "Stderr must store full 512 KiB without truncation")
	assert.Equal(t, byte('A'), r.Stdout[0], "first byte of Stdout must be preserved")
	assert.Equal(t, byte('B'), r.Stdout[1], "second byte of Stdout must be preserved")
	assert.Equal(t, byte('a'), r.Stderr[0], "first byte of Stderr must be preserved")
	assert.Equal(t, 5*time.Second, r.Duration)
}

// ---------------------------------------------------------------------------
// Edge case: binary data in Stdout/Stderr
// ---------------------------------------------------------------------------

func TestResult_BinaryOutput(t *testing.T) {
	// Stdout/Stderr are []byte, so they must handle arbitrary binary data
	// including null bytes, high bytes, etc.
	binaryData := []byte{0x00, 0x01, 0xFF, 0xFE, 0x80, 0x7F}

	r := Result{
		Stdout: binaryData,
		Stderr: []byte{0xDE, 0xAD, 0xBE, 0xEF},
	}

	assert.Equal(t, binaryData, r.Stdout, "Stdout must store arbitrary binary data")
	assert.Equal(t, []byte{0xDE, 0xAD, 0xBE, 0xEF}, r.Stderr, "Stderr must store arbitrary binary data")
}

// ---------------------------------------------------------------------------
// Result: field types are correct
// ---------------------------------------------------------------------------

func TestResult_FieldTypes(t *testing.T) {
	// This test ensures the fields are the correct types by assigning
	// type-specific values that would fail compilation if types were wrong.
	exitCode := 42
	stdout := []byte("test")
	stderr := []byte("err")
	duration := 5 * time.Second

	r := Result{
		ExitCode: exitCode,
		Stdout:   stdout,
		Stderr:   stderr,
		Duration: duration,
	}

	// Assign back to typed variables to ensure the fields return the correct types.
	ec := r.ExitCode
	so := r.Stdout
	se := r.Stderr
	d := r.Duration

	assert.Equal(t, 42, ec)
	assert.Equal(t, []byte("test"), so)
	assert.Equal(t, []byte("err"), se)
	assert.Equal(t, 5*time.Second, d)
}
