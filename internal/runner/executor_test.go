package runner

import (
	"testing"

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
