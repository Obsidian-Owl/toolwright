package runner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/Obsidian-Owl/toolwright/internal/manifest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Test helper: tool builder for concise test table entries
// ---------------------------------------------------------------------------

// makeTool builds a manifest.Tool with the given args, flags, and optional auth.
// This avoids boilerplate in every table row.
func makeTool(args []manifest.Arg, flags []manifest.Flag, auth *manifest.Auth) manifest.Tool {
	return manifest.Tool{
		Name:       "test-tool",
		Entrypoint: "./test.sh",
		Args:       args,
		Flags:      flags,
		Auth:       auth,
	}
}

// ---------------------------------------------------------------------------
// AC-1: Positional args mapped correctly
// AC-2: Flags mapped correctly
// AC-3: Auth token injected as flag
// Combined table-driven test with exact slice equality
// ---------------------------------------------------------------------------

func TestBuildArgs_TableDriven(t *testing.T) {
	tests := []struct {
		name           string
		tool           manifest.Tool
		positionalArgs []string
		flags          map[string]string
		token          string
		want           []string
	}{
		// -----------------------------------------------------------------
		// Empty / baseline cases
		// -----------------------------------------------------------------
		{
			name:           "no args, no flags, no token yields empty slice",
			tool:           makeTool(nil, nil, nil),
			positionalArgs: nil,
			flags:          nil,
			token:          "",
			want:           []string{},
		},
		{
			name:           "empty positional slice and empty flags map yields empty slice",
			tool:           makeTool(nil, nil, nil),
			positionalArgs: []string{},
			flags:          map[string]string{},
			token:          "",
			want:           []string{},
		},

		// -----------------------------------------------------------------
		// AC-1: Positional args
		// -----------------------------------------------------------------
		{
			name: "single positional arg",
			tool: makeTool(
				[]manifest.Arg{{Name: "path", Type: "string", Required: true}},
				nil,
				nil,
			),
			positionalArgs: []string{"./src"},
			flags:          nil,
			token:          "",
			want:           []string{"./src"},
		},
		{
			name: "multiple positional args preserve order",
			tool: makeTool(
				[]manifest.Arg{
					{Name: "name", Type: "string", Required: true},
					{Name: "species", Type: "string", Required: true},
				},
				nil,
				nil,
			),
			positionalArgs: []string{"Whiskers", "cat"},
			flags:          nil,
			token:          "",
			want:           []string{"Whiskers", "cat"},
		},
		{
			name: "three positional args in order",
			tool: makeTool(
				[]manifest.Arg{
					{Name: "a", Type: "string", Required: true},
					{Name: "b", Type: "string", Required: true},
					{Name: "c", Type: "string", Required: true},
				},
				nil,
				nil,
			),
			positionalArgs: []string{"first", "second", "third"},
			flags:          nil,
			token:          "",
			want:           []string{"first", "second", "third"},
		},

		// -----------------------------------------------------------------
		// AC-2: String flags
		// -----------------------------------------------------------------
		{
			name: "single string flag becomes --name value",
			tool: makeTool(
				nil,
				[]manifest.Flag{{Name: "severity", Type: "string"}},
				nil,
			),
			positionalArgs: nil,
			flags:          map[string]string{"severity": "high"},
			token:          "",
			want:           []string{"--severity", "high"},
		},
		{
			name: "multiple string flags each become --name value pairs",
			tool: makeTool(
				nil,
				[]manifest.Flag{
					{Name: "format", Type: "string"},
					{Name: "output", Type: "string"},
				},
				nil,
			),
			positionalArgs: nil,
			flags:          map[string]string{"format": "json", "output": "/tmp/out.txt"},
			token:          "",
			// Flags should appear in the order defined in tool.Flags, not map iteration order
			want: []string{"--format", "json", "--output", "/tmp/out.txt"},
		},

		// -----------------------------------------------------------------
		// AC-2: Bool flags
		// -----------------------------------------------------------------
		{
			name: "bool flag true becomes --name with no value",
			tool: makeTool(
				nil,
				[]manifest.Flag{{Name: "verbose", Type: "bool"}},
				nil,
			),
			positionalArgs: nil,
			flags:          map[string]string{"verbose": "true"},
			token:          "",
			want:           []string{"--verbose"},
		},
		{
			name: "bool flag false is omitted entirely",
			tool: makeTool(
				nil,
				[]manifest.Flag{{Name: "verbose", Type: "bool"}},
				nil,
			),
			positionalArgs: nil,
			flags:          map[string]string{"verbose": "false"},
			token:          "",
			want:           []string{},
		},
		{
			name: "mixed bool true and bool false flags",
			tool: makeTool(
				nil,
				[]manifest.Flag{
					{Name: "verbose", Type: "bool"},
					{Name: "quiet", Type: "bool"},
					{Name: "debug", Type: "bool"},
				},
				nil,
			),
			positionalArgs: nil,
			flags:          map[string]string{"verbose": "true", "quiet": "false", "debug": "true"},
			token:          "",
			// verbose and debug present, quiet omitted. Order matches tool.Flags definition.
			want: []string{"--verbose", "--debug"},
		},

		// -----------------------------------------------------------------
		// AC-2: Mixed string and bool flags
		// -----------------------------------------------------------------
		{
			name: "mixed string and bool flags",
			tool: makeTool(
				nil,
				[]manifest.Flag{
					{Name: "format", Type: "string"},
					{Name: "verbose", Type: "bool"},
					{Name: "limit", Type: "int"},
				},
				nil,
			),
			positionalArgs: nil,
			flags:          map[string]string{"format": "json", "verbose": "true", "limit": "10"},
			token:          "",
			want:           []string{"--format", "json", "--verbose", "--limit", "10"},
		},

		// -----------------------------------------------------------------
		// AC-3: Token injection
		// -----------------------------------------------------------------
		{
			name: "token injected via Auth.TokenFlag",
			tool: makeTool(
				nil,
				nil,
				&manifest.Auth{Type: "token", TokenFlag: "--api-key"},
			),
			positionalArgs: nil,
			flags:          nil,
			token:          "secret123",
			want:           []string{"--api-key", "secret123"},
		},
		{
			name: "empty token means no token flag appended",
			tool: makeTool(
				nil,
				nil,
				&manifest.Auth{Type: "token", TokenFlag: "--api-key"},
			),
			positionalArgs: nil,
			flags:          nil,
			token:          "",
			want:           []string{},
		},
		{
			name:           "nil auth with non-empty token produces no token flag",
			tool:           makeTool(nil, nil, nil),
			positionalArgs: nil,
			flags:          nil,
			token:          "secret123",
			want:           []string{},
		},

		// -----------------------------------------------------------------
		// Ordering: positional first, flags second, token last
		// -----------------------------------------------------------------
		{
			name: "positional args come before flags",
			tool: makeTool(
				[]manifest.Arg{{Name: "path", Type: "string", Required: true}},
				[]manifest.Flag{{Name: "format", Type: "string"}},
				nil,
			),
			positionalArgs: []string{"./src"},
			flags:          map[string]string{"format": "json"},
			token:          "",
			want:           []string{"./src", "--format", "json"},
		},
		{
			name: "token appears after all other args and flags",
			tool: makeTool(
				[]manifest.Arg{{Name: "path", Type: "string", Required: true}},
				[]manifest.Flag{{Name: "format", Type: "string"}},
				&manifest.Auth{Type: "token", TokenFlag: "--token"},
			),
			positionalArgs: []string{"./src"},
			flags:          map[string]string{"format": "json"},
			token:          "mytoken",
			want:           []string{"./src", "--format", "json", "--token", "mytoken"},
		},
		{
			name: "full combination: multiple positional + multiple flags + token",
			tool: makeTool(
				[]manifest.Arg{
					{Name: "name", Type: "string", Required: true},
					{Name: "species", Type: "string", Required: true},
				},
				[]manifest.Flag{
					{Name: "age", Type: "int"},
					{Name: "vaccinated", Type: "bool"},
				},
				&manifest.Auth{Type: "token", TokenFlag: "--token"},
			),
			positionalArgs: []string{"Whiskers", "cat"},
			flags:          map[string]string{"age": "3", "vaccinated": "true"},
			token:          "secret",
			want:           []string{"Whiskers", "cat", "--age", "3", "--vaccinated", "--token", "secret"},
		},

		// -----------------------------------------------------------------
		// Edge cases
		// -----------------------------------------------------------------
		{
			name: "flag with empty string value is omitted",
			tool: makeTool(
				nil,
				[]manifest.Flag{{Name: "label", Type: "string"}},
				nil,
			),
			positionalArgs: nil,
			flags:          map[string]string{"label": ""},
			token:          "",
			want:           []string{},
		},
		{
			name: "flag not present in input flags map is omitted",
			tool: makeTool(
				nil,
				[]manifest.Flag{
					{Name: "format", Type: "string"},
					{Name: "verbose", Type: "bool"},
				},
				nil,
			),
			positionalArgs: nil,
			flags:          map[string]string{"format": "json"},
			token:          "",
			// verbose not in flags map, so it should not appear
			want: []string{"--format", "json"},
		},
		{
			name: "positional arg with spaces preserved as single element",
			tool: makeTool(
				[]manifest.Arg{{Name: "message", Type: "string", Required: true}},
				nil,
				nil,
			),
			positionalArgs: []string{"hello world"},
			flags:          nil,
			token:          "",
			want:           []string{"hello world"},
		},
		{
			name: "flag value with spaces preserved as single element",
			tool: makeTool(
				nil,
				[]manifest.Flag{{Name: "message", Type: "string"}},
				nil,
			),
			positionalArgs: nil,
			flags:          map[string]string{"message": "hello world"},
			token:          "",
			want:           []string{"--message", "hello world"},
		},
		{
			name: "token value with special characters preserved exactly",
			tool: makeTool(
				nil,
				nil,
				&manifest.Auth{Type: "token", TokenFlag: "--api-key"},
			),
			positionalArgs: nil,
			flags:          nil,
			token:          "tok3n-with_sp3c!@l#chars",
			want:           []string{"--api-key", "tok3n-with_sp3c!@l#chars"},
		},
		{
			name: "token flag from auth uses exact string (including leading dashes)",
			tool: makeTool(
				nil,
				nil,
				&manifest.Auth{Type: "token", TokenFlag: "--api-key"},
			),
			positionalArgs: nil,
			flags:          nil,
			token:          "abc",
			// The token_flag value should be used as-is; BuildArgs should NOT add extra dashes
			want: []string{"--api-key", "abc"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := BuildArgs(tc.tool, tc.positionalArgs, tc.flags, tc.token)

			require.NotNil(t, got,
				"BuildArgs must return a non-nil slice (use empty slice, not nil)")
			assert.Equal(t, tc.want, got,
				"BuildArgs result must exactly match expected args slice")
		})
	}
}

// ---------------------------------------------------------------------------
// AC-2: Flag ordering follows tool.Flags definition order, not map iteration
// ---------------------------------------------------------------------------

func TestBuildArgs_FlagOrderFollowsToolDefinition(t *testing.T) {
	// Maps in Go have non-deterministic iteration order. The implementation
	// must iterate tool.Flags (which is a slice) and look up values from the
	// flags map, NOT iterate the map directly. We verify by defining flags
	// in a specific order and checking the output matches that order.
	//
	// We run this test multiple times to increase the chance of catching
	// map-iteration-order bugs (which are non-deterministic).
	tool := makeTool(
		nil,
		[]manifest.Flag{
			{Name: "alpha", Type: "string"},
			{Name: "bravo", Type: "string"},
			{Name: "charlie", Type: "string"},
			{Name: "delta", Type: "string"},
			{Name: "echo", Type: "string"},
		},
		nil,
	)

	flags := map[string]string{
		"alpha":   "1",
		"bravo":   "2",
		"charlie": "3",
		"delta":   "4",
		"echo":    "5",
	}

	want := []string{
		"--alpha", "1",
		"--bravo", "2",
		"--charlie", "3",
		"--delta", "4",
		"--echo", "5",
	}

	// Run 10 times to shake out non-deterministic map iteration bugs.
	for i := 0; i < 10; i++ {
		got := BuildArgs(tool, nil, flags, "")
		assert.Equal(t, want, got,
			"iteration %d: flags must appear in tool.Flags definition order", i)
	}
}

// ---------------------------------------------------------------------------
// AC-3: Token flag position is ALWAYS last
// ---------------------------------------------------------------------------

func TestBuildArgs_TokenAlwaysLast(t *testing.T) {
	// Even with many flags and args, the token must be the final elements.
	tool := makeTool(
		[]manifest.Arg{
			{Name: "a1", Type: "string", Required: true},
			{Name: "a2", Type: "string", Required: true},
		},
		[]manifest.Flag{
			{Name: "f1", Type: "string"},
			{Name: "f2", Type: "string"},
			{Name: "f3", Type: "bool"},
		},
		&manifest.Auth{Type: "token", TokenFlag: "--secret"},
	)

	got := BuildArgs(tool, []string{"x", "y"}, map[string]string{
		"f1": "v1",
		"f2": "v2",
		"f3": "true",
	}, "tok")

	require.NotNil(t, got)
	require.True(t, len(got) >= 2, "result must have at least token flag + value")

	// Last two elements must be the token flag and value
	lastTwo := got[len(got)-2:]
	assert.Equal(t, []string{"--secret", "tok"}, lastTwo,
		"token flag and value must be the last two elements in the result")

	// Verify the full ordering: positional args, then flags, then token
	// Positional args should be at the beginning
	assert.Equal(t, "x", got[0], "first positional arg should be first element")
	assert.Equal(t, "y", got[1], "second positional arg should be second element")
}

// ---------------------------------------------------------------------------
// AC-3: Empty TokenFlag in auth with non-empty token
// ---------------------------------------------------------------------------

func TestBuildArgs_AuthWithEmptyTokenFlag(t *testing.T) {
	// If the auth exists but TokenFlag is empty, no token should be appended
	// even if a token string is provided.
	tool := makeTool(
		nil,
		nil,
		&manifest.Auth{Type: "token", TokenFlag: ""},
	)

	got := BuildArgs(tool, nil, nil, "secret123")
	require.NotNil(t, got)
	assert.Equal(t, []string{}, got,
		"empty TokenFlag should result in no token being appended")
}

// ---------------------------------------------------------------------------
// Anti-hardcoding: distinct inputs produce distinct outputs
// ---------------------------------------------------------------------------

func TestBuildArgs_DistinctInputsProduceDistinctOutputs(t *testing.T) {
	// Guard against a hardcoded implementation that always returns the same thing.
	tool := makeTool(
		[]manifest.Arg{{Name: "path", Type: "string", Required: true}},
		[]manifest.Flag{{Name: "format", Type: "string"}},
		&manifest.Auth{Type: "token", TokenFlag: "--token"},
	)

	result1 := BuildArgs(tool, []string{"./src"}, map[string]string{"format": "json"}, "tok1")
	result2 := BuildArgs(tool, []string{"./dst"}, map[string]string{"format": "csv"}, "tok2")

	assert.NotEqual(t, result1, result2,
		"different inputs must produce different outputs")

	// Verify specific differences
	assert.Contains(t, result1, "./src")
	assert.Contains(t, result2, "./dst")
	assert.Contains(t, result1, "json")
	assert.Contains(t, result2, "csv")
	assert.Contains(t, result1, "tok1")
	assert.Contains(t, result2, "tok2")
}

// ---------------------------------------------------------------------------
// AC-2: Bool flag edge -- only type "bool" gets presence-only treatment
// ---------------------------------------------------------------------------

func TestBuildArgs_OnlyBoolTypeFlagsGetPresenceOnlyTreatment(t *testing.T) {
	// A string flag with value "true" should NOT be treated as a bool flag.
	// Only flags with Type == "bool" get the special treatment.
	tool := makeTool(
		nil,
		[]manifest.Flag{
			{Name: "enabled", Type: "string"},
			{Name: "verbose", Type: "bool"},
		},
		nil,
	)

	got := BuildArgs(tool, nil, map[string]string{
		"enabled": "true",
		"verbose": "true",
	}, "")

	require.NotNil(t, got)
	// "enabled" is a string flag so "true" should appear as a value
	// "verbose" is a bool flag so "true" should result in just --verbose
	assert.Equal(t, []string{"--enabled", "true", "--verbose"}, got,
		"string flag with value 'true' must include the value; bool flag must not")
}

// ---------------------------------------------------------------------------
// Edge: unicode and special characters in args and flag values
// ---------------------------------------------------------------------------

func TestBuildArgs_UnicodeAndSpecialChars(t *testing.T) {
	tool := makeTool(
		[]manifest.Arg{{Name: "msg", Type: "string", Required: true}},
		[]manifest.Flag{{Name: "label", Type: "string"}},
		nil,
	)

	got := BuildArgs(tool, []string{"hello"}, map[string]string{"label": "cafe\u0301"}, "")
	require.NotNil(t, got)
	assert.Equal(t, []string{"hello", "--label", "cafe\u0301"}, got,
		"unicode characters in flag values must be preserved exactly")
}

// ---------------------------------------------------------------------------
// Edge: positional arg that looks like a flag (starts with --)
// ---------------------------------------------------------------------------

func TestBuildArgs_PositionalArgThatLooksLikeFlag(t *testing.T) {
	// Positional args should be passed as-is, even if they start with --.
	// BuildArgs is not responsible for escaping; it just builds the []string.
	tool := makeTool(
		[]manifest.Arg{{Name: "pattern", Type: "string", Required: true}},
		nil,
		nil,
	)

	got := BuildArgs(tool, []string{"--not-a-flag"}, nil, "")
	require.NotNil(t, got)
	assert.Equal(t, []string{"--not-a-flag"}, got,
		"positional args should be passed through as-is regardless of content")
}

// ---------------------------------------------------------------------------
// Edge: multiple calls do not leak state between invocations
// ---------------------------------------------------------------------------

func TestBuildArgs_NoStateLeak(t *testing.T) {
	tool := makeTool(
		[]manifest.Arg{{Name: "x", Type: "string", Required: true}},
		[]manifest.Flag{{Name: "f", Type: "string"}},
		&manifest.Auth{Type: "token", TokenFlag: "--tok"},
	)

	got1 := BuildArgs(tool, []string{"a"}, map[string]string{"f": "v1"}, "t1")
	got2 := BuildArgs(tool, []string{"b"}, map[string]string{"f": "v2"}, "t2")
	got3 := BuildArgs(tool, nil, nil, "")

	assert.Equal(t, []string{"a", "--f", "v1", "--tok", "t1"}, got1)
	assert.Equal(t, []string{"b", "--f", "v2", "--tok", "t2"}, got2)
	assert.Equal(t, []string{}, got3,
		"third call with empty inputs must return empty, not leftovers from prior calls")
}

// ---------------------------------------------------------------------------
// Edge: flag defined in tool.Flags but absent from input map
// ---------------------------------------------------------------------------

func TestBuildArgs_UndefinedFlagsIgnored(t *testing.T) {
	// Only flags defined in tool.Flags should appear. Extra keys in the
	// flags map that are NOT in tool.Flags should be ignored.
	tool := makeTool(
		nil,
		[]manifest.Flag{{Name: "format", Type: "string"}},
		nil,
	)

	got := BuildArgs(tool, nil, map[string]string{
		"format":  "json",
		"unknown": "should-not-appear",
	}, "")

	require.NotNil(t, got)
	assert.Equal(t, []string{"--format", "json"}, got,
		"flags not defined in tool.Flags must not appear in result")
	assert.NotContains(t, got, "--unknown",
		"unknown flag must not be in result")
	assert.NotContains(t, got, "should-not-appear",
		"unknown flag value must not be in result")
}

// ===========================================================================
// Executor.Run tests (AC-4 through AC-11)
// ===========================================================================

// ---------------------------------------------------------------------------
// Test helper: write a bash script in a temp directory and return its path
// ---------------------------------------------------------------------------

// writeScript creates a bash script in dir with the given name and content,
// marks it executable, and returns the absolute path. Fails the test on error.
func writeScript(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	err := os.WriteFile(path, []byte("#!/bin/bash\n"+content), 0755)
	require.NoError(t, err, "writeScript: failed to write %s", path)
	return path
}

// makeToolWithEntrypoint builds a manifest.Tool with a custom entrypoint path
// and optional auth. Convenience for Executor tests.
func makeToolWithEntrypoint(entrypoint string, auth *manifest.Auth) manifest.Tool {
	return manifest.Tool{
		Name:       "exec-test-tool",
		Entrypoint: entrypoint,
		Auth:       auth,
	}
}

// ---------------------------------------------------------------------------
// AC-5: stdout captured correctly
// ---------------------------------------------------------------------------

func TestExecutor_CapturesStdout(t *testing.T) {
	dir := t.TempDir()
	script := writeScript(t, dir, "stdout.sh", `echo -n '{"ok":true}'`)

	tool := makeToolWithEntrypoint(script, nil)
	exec := Executor{WorkDir: dir}
	ctx := context.Background()

	result, err := exec.Run(ctx, tool, nil, nil, "")
	require.NoError(t, err, "Run should not return an error for a successful script")
	require.NotNil(t, result, "Result must not be nil")

	assert.Equal(t, `{"ok":true}`, string(result.Stdout),
		"Stdout must contain exactly what the script wrote to stdout")
}

// ---------------------------------------------------------------------------
// AC-5: stderr captured correctly
// ---------------------------------------------------------------------------

func TestExecutor_CapturesStderr(t *testing.T) {
	dir := t.TempDir()
	script := writeScript(t, dir, "stderr.sh", `echo -n 'debug info' >&2`)

	tool := makeToolWithEntrypoint(script, nil)
	exec := Executor{WorkDir: dir}
	ctx := context.Background()

	result, err := exec.Run(ctx, tool, nil, nil, "")
	require.NoError(t, err, "Run should not return an error for a successful script")
	require.NotNil(t, result, "Result must not be nil")

	assert.Equal(t, "debug info", string(result.Stderr),
		"Stderr must contain exactly what the script wrote to stderr")
}

// ---------------------------------------------------------------------------
// AC-5: stdout and stderr do not contaminate each other
// ---------------------------------------------------------------------------

func TestExecutor_StdoutStderrSeparate(t *testing.T) {
	dir := t.TempDir()
	script := writeScript(t, dir, "both.sh", `
echo -n '{"ok":true}'
echo -n 'debug info' >&2
`)

	tool := makeToolWithEntrypoint(script, nil)
	exec := Executor{WorkDir: dir}
	ctx := context.Background()

	result, err := exec.Run(ctx, tool, nil, nil, "")
	require.NoError(t, err, "Run should not return an error for a successful script")
	require.NotNil(t, result, "Result must not be nil")

	assert.Equal(t, `{"ok":true}`, string(result.Stdout),
		"Stdout must contain only stdout data, not stderr")
	assert.Equal(t, "debug info", string(result.Stderr),
		"Stderr must contain only stderr data, not stdout")

	// Explicitly verify no cross-contamination
	assert.NotContains(t, string(result.Stdout), "debug info",
		"Stdout must not contain stderr content")
	assert.NotContains(t, string(result.Stderr), `{"ok":true}`,
		"Stderr must not contain stdout content")
}

// ---------------------------------------------------------------------------
// AC-6: Exit codes captured correctly (table-driven)
// ---------------------------------------------------------------------------

func TestExecutor_ExitCodes(t *testing.T) {
	tests := []struct {
		name     string
		exitCode int
	}{
		{name: "exit 0", exitCode: 0},
		{name: "exit 1", exitCode: 1},
		{name: "exit 2", exitCode: 2},
		{name: "exit 127", exitCode: 127},
		{name: "exit 255", exitCode: 255},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			script := writeScript(t, dir, "exit.sh",
				fmt.Sprintf("exit %d", tc.exitCode))

			tool := makeToolWithEntrypoint(script, nil)
			exec := Executor{WorkDir: dir}
			ctx := context.Background()

			result, err := exec.Run(ctx, tool, nil, nil, "")

			// Non-zero exit is captured in Result, NOT as an error.
			require.NoError(t, err,
				"Run must not return an error for exit code %d; exit code belongs in Result", tc.exitCode)
			require.NotNil(t, result, "Result must not be nil")
			assert.Equal(t, tc.exitCode, result.ExitCode,
				"ExitCode must be %d", tc.exitCode)
		})
	}
}

// ---------------------------------------------------------------------------
// AC-6 reinforcement: non-zero exit code is NOT an error return
// ---------------------------------------------------------------------------

func TestExecutor_NonZeroExitCodeIsNotError(t *testing.T) {
	// This test is critical: a sloppy implementation might return an error
	// for any non-zero exit code. The spec says only true execution failures
	// (can't start process, timeout) should return an error. Non-zero exits
	// are normal tool results.
	dir := t.TempDir()
	script := writeScript(t, dir, "fail.sh", `
echo -n 'some output'
echo -n 'some error' >&2
exit 42
`)

	tool := makeToolWithEntrypoint(script, nil)
	exec := Executor{WorkDir: dir}
	ctx := context.Background()

	result, err := exec.Run(ctx, tool, nil, nil, "")

	require.NoError(t, err,
		"Run must NOT return an error for non-zero exit; err is for execution failures only")
	require.NotNil(t, result, "Result must not be nil even on non-zero exit")
	assert.Equal(t, 42, result.ExitCode,
		"ExitCode must be 42")
	assert.Equal(t, "some output", string(result.Stdout),
		"Stdout must be captured even on non-zero exit")
	assert.Equal(t, "some error", string(result.Stderr),
		"Stderr must be captured even on non-zero exit")
}

// ---------------------------------------------------------------------------
// AC-6: distinct exit codes produce distinct results (anti-hardcoding)
// ---------------------------------------------------------------------------

func TestExecutor_ExitCodeDistinctValues(t *testing.T) {
	dir := t.TempDir()
	script0 := writeScript(t, dir, "exit0.sh", "exit 0")
	script1 := writeScript(t, dir, "exit1.sh", "exit 1")

	exec := Executor{WorkDir: dir}
	ctx := context.Background()

	result0, err0 := exec.Run(ctx, makeToolWithEntrypoint(script0, nil), nil, nil, "")
	require.NoError(t, err0)
	result1, err1 := exec.Run(ctx, makeToolWithEntrypoint(script1, nil), nil, nil, "")
	require.NoError(t, err1)

	assert.NotEqual(t, result0.ExitCode, result1.ExitCode,
		"Different exit codes must produce different ExitCode values in Result")
	assert.Equal(t, 0, result0.ExitCode)
	assert.Equal(t, 1, result1.ExitCode)
}

// ---------------------------------------------------------------------------
// AC-7: Timeout kills process
// ---------------------------------------------------------------------------

func TestExecutor_Timeout(t *testing.T) {
	dir := t.TempDir()
	// Script that sleeps for 10 seconds -- much longer than the timeout
	script := writeScript(t, dir, "sleep.sh", "sleep 10")

	tool := makeToolWithEntrypoint(script, nil)
	exec := Executor{WorkDir: dir}

	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	start := time.Now()
	result, err := exec.Run(ctx, tool, nil, nil, "")
	elapsed := time.Since(start)

	// Must return an error indicating deadline/cancellation
	require.Error(t, err, "Run must return an error when context times out")
	assert.ErrorIs(t, err, context.DeadlineExceeded,
		"error must wrap context.DeadlineExceeded")

	// The result may be nil on timeout -- that's acceptable.
	// But we must verify the process was killed promptly (not after 10s).
	assert.Less(t, elapsed, 5*time.Second,
		"process must be killed promptly on timeout, not allowed to run to completion")

	// If result is returned on timeout, ExitCode should not indicate success.
	if result != nil {
		assert.NotEqual(t, 0, result.ExitCode,
			"ExitCode must not be 0 when process was killed by timeout")
	}
}

// ---------------------------------------------------------------------------
// AC-7 reinforcement: context cancellation (not just timeout)
// ---------------------------------------------------------------------------

func TestExecutor_ContextCancellation(t *testing.T) {
	dir := t.TempDir()
	script := writeScript(t, dir, "longsleep.sh", "sleep 10")

	tool := makeToolWithEntrypoint(script, nil)
	exec := Executor{WorkDir: dir}

	ctx, cancel := context.WithCancel(context.Background())

	// Cancel after a short delay
	go func() {
		time.Sleep(500 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	_, err := exec.Run(ctx, tool, nil, nil, "")
	elapsed := time.Since(start)

	require.Error(t, err, "Run must return an error when context is cancelled")
	assert.ErrorIs(t, err, context.Canceled,
		"error must wrap context.Canceled")
	assert.Less(t, elapsed, 5*time.Second,
		"process must be killed promptly on cancellation")
}

// ---------------------------------------------------------------------------
// AC-8: Non-existent entrypoint returns error
// ---------------------------------------------------------------------------

func TestExecutor_NonExistentEntrypoint(t *testing.T) {
	dir := t.TempDir()
	nonexistent := filepath.Join(dir, "nonexistent.sh")

	tool := makeToolWithEntrypoint(nonexistent, nil)
	exec := Executor{WorkDir: dir}
	ctx := context.Background()

	result, err := exec.Run(ctx, tool, nil, nil, "")

	require.Error(t, err, "Run must return an error for non-existent entrypoint")
	assert.Nil(t, result, "Result must be nil when entrypoint does not exist")

	// Error message must indicate the file doesn't exist -- not a generic error
	errMsg := err.Error()
	assert.True(t,
		strings.Contains(errMsg, "no such file") ||
			strings.Contains(errMsg, "not found") ||
			strings.Contains(errMsg, "does not exist") ||
			strings.Contains(errMsg, "executable file not found"),
		"error message %q must indicate the file does not exist", errMsg)
}

// ---------------------------------------------------------------------------
// AC-9: Non-executable entrypoint returns permission denied error
// ---------------------------------------------------------------------------

func TestExecutor_NonExecutableEntrypoint(t *testing.T) {
	dir := t.TempDir()
	// Create a file but do NOT mark it executable
	path := filepath.Join(dir, "noexec.sh")
	err := os.WriteFile(path, []byte("#!/bin/bash\necho hello"), 0644)
	require.NoError(t, err, "failed to write test file")

	tool := makeToolWithEntrypoint(path, nil)
	exec := Executor{WorkDir: dir}
	ctx := context.Background()

	result, runErr := exec.Run(ctx, tool, nil, nil, "")

	require.Error(t, runErr, "Run must return an error for non-executable entrypoint")
	assert.Nil(t, result, "Result must be nil when entrypoint is not executable")

	errMsg := runErr.Error()
	assert.True(t,
		strings.Contains(errMsg, "permission denied") ||
			strings.Contains(errMsg, "Permission denied") ||
			strings.Contains(errMsg, "not executable"),
		"error message %q must indicate permission denied", errMsg)
}

// ---------------------------------------------------------------------------
// AC-10: Working directory set correctly
// ---------------------------------------------------------------------------

func TestExecutor_WorkDir(t *testing.T) {
	scriptDir := t.TempDir()
	workDir := t.TempDir() // different directory from where the script lives

	script := writeScript(t, scriptDir, "pwd.sh", "pwd")

	tool := makeToolWithEntrypoint(script, nil)
	exec := Executor{WorkDir: workDir}
	ctx := context.Background()

	result, err := exec.Run(ctx, tool, nil, nil, "")
	require.NoError(t, err, "Run should not return an error")
	require.NotNil(t, result, "Result must not be nil")

	// pwd output will have a trailing newline; trim it
	actual := strings.TrimSpace(string(result.Stdout))

	// Resolve any symlinks (e.g., /tmp on macOS is often /private/tmp)
	expectedResolved, err := filepath.EvalSymlinks(workDir)
	require.NoError(t, err)
	actualResolved, err := filepath.EvalSymlinks(actual)
	require.NoError(t, err)

	assert.Equal(t, expectedResolved, actualResolved,
		"child process must run in the Executor's WorkDir")
}

// ---------------------------------------------------------------------------
// AC-10: WorkDir is used, not the script's directory
// ---------------------------------------------------------------------------

func TestExecutor_WorkDirIsNotScriptDir(t *testing.T) {
	// Ensure WorkDir is respected even when the script lives in a different directory.
	// A sloppy implementation might use the script's parent directory instead.
	scriptDir := t.TempDir()
	workDir := t.TempDir()

	// These must be different directories for the test to be meaningful
	require.NotEqual(t, scriptDir, workDir,
		"scriptDir and workDir must differ for this test to be meaningful")

	script := writeScript(t, scriptDir, "pwd.sh", "pwd")

	tool := makeToolWithEntrypoint(script, nil)
	exec := Executor{WorkDir: workDir}
	ctx := context.Background()

	result, err := exec.Run(ctx, tool, nil, nil, "")
	require.NoError(t, err)
	require.NotNil(t, result)

	actual := strings.TrimSpace(string(result.Stdout))
	actualResolved, err := filepath.EvalSymlinks(actual)
	require.NoError(t, err)
	scriptDirResolved, err := filepath.EvalSymlinks(scriptDir)
	require.NoError(t, err)

	assert.NotEqual(t, scriptDirResolved, actualResolved,
		"child process must NOT run in the script's directory; it must use WorkDir")
}

// ---------------------------------------------------------------------------
// AC-11: Duration captured
// ---------------------------------------------------------------------------

func TestExecutor_Duration(t *testing.T) {
	dir := t.TempDir()
	// Script that sleeps for ~100ms
	script := writeScript(t, dir, "duration.sh", "sleep 0.1")

	tool := makeToolWithEntrypoint(script, nil)
	exec := Executor{WorkDir: dir}
	ctx := context.Background()

	result, err := exec.Run(ctx, tool, nil, nil, "")
	require.NoError(t, err, "Run should not return an error")
	require.NotNil(t, result, "Result must not be nil")

	// Duration should be at least 50ms (the script sleeps 100ms, with tolerance for startup)
	assert.GreaterOrEqual(t, result.Duration, 50*time.Millisecond,
		"Duration must be at least 50ms for a script that sleeps 100ms")

	// Duration should not be unreasonably long (under 5s for a 100ms sleep)
	assert.Less(t, result.Duration, 5*time.Second,
		"Duration must not be unreasonably long")
}

// ---------------------------------------------------------------------------
// AC-11: Duration is not always zero (anti-hardcoding)
// ---------------------------------------------------------------------------

func TestExecutor_DurationNonZero(t *testing.T) {
	dir := t.TempDir()
	script := writeScript(t, dir, "quick.sh", "echo hello")

	tool := makeToolWithEntrypoint(script, nil)
	exec := Executor{WorkDir: dir}
	ctx := context.Background()

	result, err := exec.Run(ctx, tool, nil, nil, "")
	require.NoError(t, err)
	require.NotNil(t, result)

	// Even a fast script takes some non-zero time to execute
	assert.Greater(t, result.Duration, time.Duration(0),
		"Duration must be > 0 even for a fast script")
}

// ---------------------------------------------------------------------------
// AC-11: Duration reflects actual elapsed time (anti-hardcoding with two scripts)
// ---------------------------------------------------------------------------

func TestExecutor_DurationReflectsActualTime(t *testing.T) {
	dir := t.TempDir()
	fastScript := writeScript(t, dir, "fast.sh", "echo fast")
	slowScript := writeScript(t, dir, "slow.sh", "sleep 0.2")

	exec := Executor{WorkDir: dir}
	ctx := context.Background()

	fastResult, err := exec.Run(ctx, makeToolWithEntrypoint(fastScript, nil), nil, nil, "")
	require.NoError(t, err)
	require.NotNil(t, fastResult)

	slowResult, err := exec.Run(ctx, makeToolWithEntrypoint(slowScript, nil), nil, nil, "")
	require.NoError(t, err)
	require.NotNil(t, slowResult)

	// The slow script must have a meaningfully longer duration
	assert.Greater(t, slowResult.Duration, fastResult.Duration,
		"a 200ms sleep script must take longer than a simple echo script")
}

// ---------------------------------------------------------------------------
// AC-4: Token never passed via environment
// ---------------------------------------------------------------------------

func TestExecutor_TokenNotInEnvironment(t *testing.T) {
	dir := t.TempDir()
	// Script dumps all environment variables
	script := writeScript(t, dir, "env.sh", "env")

	tool := makeToolWithEntrypoint(script,
		&manifest.Auth{Type: "token", TokenFlag: "--api-key"})
	exec := Executor{WorkDir: dir}
	ctx := context.Background()

	token := "super-secret-token-value-12345"
	result, err := exec.Run(ctx, tool, nil, nil, token)
	require.NoError(t, err, "Run should not return an error")
	require.NotNil(t, result, "Result must not be nil")

	envOutput := string(result.Stdout)

	// The token value must NOT appear anywhere in the environment
	assert.NotContains(t, envOutput, token,
		"token value must NEVER appear in the child process environment")

	// Also check each line individually in case the token is a substring
	for _, line := range strings.Split(envOutput, "\n") {
		assert.NotContains(t, line, token,
			"environment variable line %q must not contain the token", line)
	}
}

// ---------------------------------------------------------------------------
// AC-4: Token appears only in argv
// ---------------------------------------------------------------------------

func TestExecutor_TokenInArgv(t *testing.T) {
	dir := t.TempDir()
	// Script prints all its arguments, one per line
	script := writeScript(t, dir, "args.sh", `
for arg in "$@"; do
    echo "$arg"
done
`)

	tool := makeToolWithEntrypoint(script,
		&manifest.Auth{Type: "token", TokenFlag: "--api-key"})
	exec := Executor{WorkDir: dir}
	ctx := context.Background()

	token := "my-secret-token-99"
	result, err := exec.Run(ctx, tool, nil, nil, token)
	require.NoError(t, err, "Run should not return an error")
	require.NotNil(t, result, "Result must not be nil")

	argsOutput := string(result.Stdout)
	lines := strings.Split(strings.TrimSpace(argsOutput), "\n")

	// The token flag and value must appear in the arguments
	assert.Contains(t, lines, "--api-key",
		"--api-key flag must appear in the process arguments")
	assert.Contains(t, lines, token,
		"token value must appear in the process arguments")
}

// ---------------------------------------------------------------------------
// AC-4: Token not leaked even with unusual token values
// ---------------------------------------------------------------------------

func TestExecutor_TokenNotInEnvironment_SpecialChars(t *testing.T) {
	dir := t.TempDir()
	script := writeScript(t, dir, "env2.sh", "env")

	tool := makeToolWithEntrypoint(script,
		&manifest.Auth{Type: "token", TokenFlag: "--key"})
	exec := Executor{WorkDir: dir}
	ctx := context.Background()

	// Token with special characters that might trip up naive environment handling
	token := "tok=with&special chars!@#"
	result, err := exec.Run(ctx, tool, nil, nil, token)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.NotContains(t, string(result.Stdout), token,
		"token with special characters must not leak into environment")
}

// ---------------------------------------------------------------------------
// Args are passed through to the child process
// ---------------------------------------------------------------------------

func TestExecutor_ArgsPassedToProcess(t *testing.T) {
	dir := t.TempDir()
	script := writeScript(t, dir, "args.sh", `
for arg in "$@"; do
    echo "$arg"
done
`)

	tool := manifest.Tool{
		Name:       "args-tool",
		Entrypoint: script,
		Args: []manifest.Arg{
			{Name: "path", Type: "string", Required: true},
		},
		Flags: []manifest.Flag{
			{Name: "format", Type: "string"},
		},
	}
	exec := Executor{WorkDir: dir}
	ctx := context.Background()

	result, err := exec.Run(ctx, tool, []string{"./src"}, map[string]string{"format": "json"}, "")
	require.NoError(t, err)
	require.NotNil(t, result)

	argsOutput := strings.TrimSpace(string(result.Stdout))
	lines := strings.Split(argsOutput, "\n")

	// Verify specific argument values appear
	assert.Contains(t, lines, "./src",
		"positional arg must be passed to the child process")
	assert.Contains(t, lines, "--format",
		"flag name must be passed to the child process")
	assert.Contains(t, lines, "json",
		"flag value must be passed to the child process")
}

// ---------------------------------------------------------------------------
// Executor with no args or flags runs the entrypoint with no arguments
// ---------------------------------------------------------------------------

func TestExecutor_NoArgsNoFlags(t *testing.T) {
	dir := t.TempDir()
	script := writeScript(t, dir, "noargs.sh", `echo -n "argc=$#"`)

	tool := makeToolWithEntrypoint(script, nil)
	exec := Executor{WorkDir: dir}
	ctx := context.Background()

	result, err := exec.Run(ctx, tool, nil, nil, "")
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, "argc=0", string(result.Stdout),
		"script should receive zero arguments when none are provided")
}

// ---------------------------------------------------------------------------
// Multiple sequential runs do not leak state
// ---------------------------------------------------------------------------

func TestExecutor_NoStateBetweenRuns(t *testing.T) {
	dir := t.TempDir()
	script1 := writeScript(t, dir, "run1.sh", `echo -n "run1"`)
	script2 := writeScript(t, dir, "run2.sh", `echo -n "run2"`)

	exec := Executor{WorkDir: dir}
	ctx := context.Background()

	result1, err := exec.Run(ctx, makeToolWithEntrypoint(script1, nil), nil, nil, "")
	require.NoError(t, err)
	require.NotNil(t, result1)

	result2, err := exec.Run(ctx, makeToolWithEntrypoint(script2, nil), nil, nil, "")
	require.NoError(t, err)
	require.NotNil(t, result2)

	assert.Equal(t, "run1", string(result1.Stdout),
		"first run must produce its own output")
	assert.Equal(t, "run2", string(result2.Stdout),
		"second run must produce its own output, not contaminated by first")
}

// ---------------------------------------------------------------------------
// Empty stdout and stderr are captured as empty (not nil with garbage)
// ---------------------------------------------------------------------------

func TestExecutor_EmptyOutput(t *testing.T) {
	dir := t.TempDir()
	// Script that produces no output at all
	script := writeScript(t, dir, "silent.sh", "exit 0")

	tool := makeToolWithEntrypoint(script, nil)
	exec := Executor{WorkDir: dir}
	ctx := context.Background()

	result, err := exec.Run(ctx, tool, nil, nil, "")
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, 0, result.ExitCode)
	assert.Empty(t, result.Stdout,
		"Stdout must be empty when the script produces no stdout")
	assert.Empty(t, result.Stderr,
		"Stderr must be empty when the script produces no stderr")
}

// ---------------------------------------------------------------------------
// Large output is captured without truncation
// ---------------------------------------------------------------------------

func TestExecutor_LargeOutput(t *testing.T) {
	dir := t.TempDir()
	// Generate 10000 lines of output (roughly 110KB)
	script := writeScript(t, dir, "large.sh", `
for i in $(seq 1 10000); do
    echo "line $i: some padding data here"
done
`)

	tool := makeToolWithEntrypoint(script, nil)
	exec := Executor{WorkDir: dir}
	ctx := context.Background()

	result, err := exec.Run(ctx, tool, nil, nil, "")
	require.NoError(t, err)
	require.NotNil(t, result)

	lines := strings.Split(strings.TrimSpace(string(result.Stdout)), "\n")
	assert.Equal(t, 10000, len(lines),
		"all 10000 lines of output must be captured without truncation")

	// Verify first and last lines to ensure order is preserved
	assert.Equal(t, "line 1: some padding data here", lines[0],
		"first line must be preserved")
	assert.Equal(t, "line 10000: some padding data here", lines[len(lines)-1],
		"last line must be preserved")
}

// ---------------------------------------------------------------------------
// Binary data in stdout is preserved
// ---------------------------------------------------------------------------

func TestExecutor_BinaryOutput(t *testing.T) {
	dir := t.TempDir()
	// Write specific bytes including null byte to stdout using printf
	script := writeScript(t, dir, "binary.sh", `printf '\x48\x45\x4c\x4c\x4f'`)

	tool := makeToolWithEntrypoint(script, nil)
	exec := Executor{WorkDir: dir}
	ctx := context.Background()

	result, err := exec.Run(ctx, tool, nil, nil, "")
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, []byte("HELLO"), result.Stdout,
		"binary output must be preserved exactly")
}

// ---------------------------------------------------------------------------
// AC-7: Process group kill — child processes are also terminated
// ---------------------------------------------------------------------------

func TestExecutor_ProcessGroupKill(t *testing.T) {
	dir := t.TempDir()
	pidFile := filepath.Join(dir, "child.pid")

	// Parent script spawns a background child (sleep 300), writes its PID
	// to a file, then waits forever. When the process group is killed,
	// both parent and child must die.
	script := writeScript(t, dir, "parent.sh", fmt.Sprintf(`
sleep 300 &
echo $! > %s
wait
`, pidFile))

	tool := makeToolWithEntrypoint(script, nil)
	exec := Executor{WorkDir: dir}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := exec.Run(ctx, tool, nil, nil, "")
	require.Error(t, err, "Run must return an error on context timeout")

	// Read the child PID that the parent wrote before being killed.
	pidBytes, readErr := os.ReadFile(pidFile)
	if readErr != nil {
		// If the PID file wasn't written, the parent was killed before
		// spawning the child — the test is still valid (nothing to leak).
		t.Logf("PID file not written (parent killed early): %v", readErr)
		return
	}

	var childPID int
	_, scanErr := fmt.Sscanf(strings.TrimSpace(string(pidBytes)), "%d", &childPID)
	require.NoError(t, scanErr, "PID file must contain a valid integer")

	// Give the OS a moment to clean up the process table entry.
	time.Sleep(100 * time.Millisecond)

	// Sending signal 0 checks whether the process exists without affecting it.
	proc, findErr := os.FindProcess(childPID)
	if findErr != nil {
		return // process doesn't exist — good
	}
	err = proc.Signal(syscall.Signal(0))
	assert.Error(t, err,
		"child process (PID %d) must be killed when process group is terminated", childPID)
}
