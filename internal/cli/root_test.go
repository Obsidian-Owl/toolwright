package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// AC-1 + AC-5: NewRootCommand returns a Cobra command with required flags
// ---------------------------------------------------------------------------

func TestNewRootCommand_ReturnsNonNil(t *testing.T) {
	cmd := NewRootCommand()
	require.NotNil(t, cmd, "NewRootCommand must return a non-nil *cobra.Command")
}

func TestNewRootCommand_HasUseField(t *testing.T) {
	cmd := NewRootCommand()
	require.NotNil(t, cmd, "precondition: NewRootCommand must return non-nil")
	assert.Equal(t, "toolwright", cmd.Use,
		"Root command Use must be 'toolwright'")
}

func TestNewRootCommand_HasJsonPersistentFlag(t *testing.T) {
	cmd := NewRootCommand()
	require.NotNil(t, cmd, "precondition: NewRootCommand must return non-nil")
	f := cmd.PersistentFlags().Lookup("json")
	require.NotNil(t, f, "--json persistent flag must exist on root command")
	assert.Equal(t, "bool", f.Value.Type(),
		"--json flag must be a boolean")
	assert.Equal(t, "false", f.DefValue,
		"--json flag default must be false")
}

func TestNewRootCommand_HasDebugPersistentFlag(t *testing.T) {
	cmd := NewRootCommand()
	require.NotNil(t, cmd, "precondition: NewRootCommand must return non-nil")
	f := cmd.PersistentFlags().Lookup("debug")
	require.NotNil(t, f, "--debug persistent flag must exist on root command")
	assert.Equal(t, "bool", f.Value.Type(),
		"--debug flag must be a boolean")
	assert.Equal(t, "false", f.DefValue,
		"--debug flag default must be false")
}

func TestNewRootCommand_HasNoColorPersistentFlag(t *testing.T) {
	cmd := NewRootCommand()
	require.NotNil(t, cmd, "precondition: NewRootCommand must return non-nil")
	f := cmd.PersistentFlags().Lookup("no-color")
	require.NotNil(t, f, "--no-color persistent flag must exist on root command")
	assert.Equal(t, "bool", f.Value.Type(),
		"--no-color flag must be a boolean")
	assert.Equal(t, "false", f.DefValue,
		"--no-color flag default must be false")
}

// Ensure the flags are persistent (available to subcommands), not local-only.
func TestNewRootCommand_FlagsArePersistent_NotLocal(t *testing.T) {
	cmd := NewRootCommand()
	require.NotNil(t, cmd, "precondition: NewRootCommand must return non-nil")

	flags := []string{"json", "debug", "no-color"}
	for _, name := range flags {
		// Must be found in PersistentFlags
		pf := cmd.PersistentFlags().Lookup(name)
		require.NotNil(t, pf, "--%s must be a persistent flag", name)

		// Must NOT be found in non-persistent local flags
		lf := cmd.LocalNonPersistentFlags().Lookup(name)
		assert.Nil(t, lf, "--%s should not be a local-only flag", name)
	}
}

// ---------------------------------------------------------------------------
// AC-3: Exit code constants
// ---------------------------------------------------------------------------

func TestExitCode_Values(t *testing.T) {
	tests := []struct {
		name     string
		got      int
		wantVal  int
		wantName string
	}{
		{name: "ExitSuccess", got: ExitSuccess, wantVal: 0, wantName: "ExitSuccess"},
		{name: "ExitError", got: ExitError, wantVal: 1, wantName: "ExitError"},
		{name: "ExitUsage", got: ExitUsage, wantVal: 2, wantName: "ExitUsage"},
		{name: "ExitIO", got: ExitIO, wantVal: 3, wantName: "ExitIO"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.wantVal, tc.got,
				"%s must equal %d", tc.wantName, tc.wantVal)
		})
	}
}

func TestExitCode_AllDistinct(t *testing.T) {
	// A lazy implementation that sets all to the same value must fail.
	codes := map[int]string{
		ExitSuccess: "ExitSuccess",
		ExitError:   "ExitError",
		ExitUsage:   "ExitUsage",
		ExitIO:      "ExitIO",
	}
	assert.Len(t, codes, 4,
		"All four exit codes must have distinct values; got collisions")
}

// Verify ordering: ExitSuccess < ExitError < ExitUsage < ExitIO
func TestExitCode_Ordering(t *testing.T) {
	assert.Less(t, ExitSuccess, ExitError,
		"ExitSuccess must be less than ExitError")
	assert.Less(t, ExitError, ExitUsage,
		"ExitError must be less than ExitUsage")
	assert.Less(t, ExitUsage, ExitIO,
		"ExitUsage must be less than ExitIO")
}

// ---------------------------------------------------------------------------
// AC-3: Root command returns error for invalid usage
// ---------------------------------------------------------------------------

func TestNewRootCommand_UnknownSubcommand_ReturnsError(t *testing.T) {
	cmd := NewRootCommand()
	require.NotNil(t, cmd, "precondition: NewRootCommand must return non-nil")
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"nonexistent-subcommand"})

	err := cmd.Execute()
	// Cobra should return an error for unknown subcommand.
	require.Error(t, err, "executing unknown subcommand must return an error")
}

// ---------------------------------------------------------------------------
// AC-1: --json flag is parseable when set
// ---------------------------------------------------------------------------

func TestNewRootCommand_JsonFlag_CanBeSet(t *testing.T) {
	cmd := NewRootCommand()
	require.NotNil(t, cmd, "precondition: NewRootCommand must return non-nil")
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--json"})

	// We just need the flags to parse. The command may still error because
	// there is no default action, but the flag should parse.
	_ = cmd.Execute()

	val, err := cmd.PersistentFlags().GetBool("json")
	require.NoError(t, err, "GetBool(\"json\") should not error after parsing")
	assert.True(t, val, "--json flag should be true when set")
}

func TestNewRootCommand_DebugFlag_CanBeSet(t *testing.T) {
	cmd := NewRootCommand()
	require.NotNil(t, cmd, "precondition: NewRootCommand must return non-nil")
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--debug"})

	_ = cmd.Execute()

	val, err := cmd.PersistentFlags().GetBool("debug")
	require.NoError(t, err, "GetBool(\"debug\") should not error after parsing")
	assert.True(t, val, "--debug flag should be true when set")
}

// ---------------------------------------------------------------------------
// AC-1: --json mode error output through command execution
// ---------------------------------------------------------------------------

func TestNewRootCommand_JsonMode_ErrorOutput_IsValidJSON(t *testing.T) {
	// When --json is set and the command errors, stdout should contain
	// valid JSON error output.
	cmd := NewRootCommand()
	require.NotNil(t, cmd, "precondition: NewRootCommand must return non-nil")
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{"--json", "nonexistent-subcommand"})

	_ = cmd.Execute()

	// Stdout must contain valid JSON when --json is active and there is an error.
	require.Greater(t, stdout.Len(), 0,
		"stdout must contain JSON error output when --json is set and command errors")
	assert.True(t, json.Valid(stdout.Bytes()),
		"stdout in --json mode must be valid JSON, got: %s", stdout.String())
}

func TestNewRootCommand_JsonMode_ErrorOutput_HasErrorStructure(t *testing.T) {
	// The JSON error output must have the {error: {code, message, hint}} shape.
	cmd := NewRootCommand()
	require.NotNil(t, cmd, "precondition: NewRootCommand must return non-nil")
	stdout := &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--json", "nonexistent-subcommand"})

	_ = cmd.Execute()

	if stdout.Len() == 0 {
		t.Fatal("stdout must contain JSON error output when --json is set and command errors")
	}

	var got map[string]any
	err := json.Unmarshal(stdout.Bytes(), &got)
	require.NoError(t, err, "stdout must be parseable JSON")

	errObj, ok := got["error"].(map[string]any)
	require.True(t, ok, "JSON error output must have top-level 'error' object")

	assert.Contains(t, errObj, "code", "error object must have 'code' field")
	assert.Contains(t, errObj, "message", "error object must have 'message' field")
	assert.Contains(t, errObj, "hint", "error object must have 'hint' field")
}

// ---------------------------------------------------------------------------
// AC-5: --debug output goes to stderr, not stdout
// ---------------------------------------------------------------------------

func TestNewRootCommand_DebugMode_NoDebugInStdout(t *testing.T) {
	cmd := NewRootCommand()
	require.NotNil(t, cmd, "precondition: NewRootCommand must return non-nil")
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.SetArgs([]string{"--debug"})

	_ = cmd.Execute()

	// Debug mode MUST NOT write diagnostics to stdout. Stdout should remain
	// clean for structured output (per Constitution rule 18).
	assert.NotContains(t, stdout.String(), "[DEBUG]",
		"debug output must not appear in stdout")
	assert.NotContains(t, stdout.String(), "debug:",
		"debug output must not appear in stdout (alternate format)")
}

// ---------------------------------------------------------------------------
// AC-3: loadManifest returns correct error for missing file
// ---------------------------------------------------------------------------

func TestLoadManifest_NonexistentFile_ReturnsError(t *testing.T) {
	_, err := loadManifest("/nonexistent/path/toolwright.yaml")
	require.Error(t, err, "loadManifest must return error for nonexistent file")
}

func TestLoadManifest_NonexistentFile_ErrorContainsPath(t *testing.T) {
	path := "/nonexistent/path/toolwright.yaml"
	_, err := loadManifest(path)
	require.Error(t, err)
	assert.Contains(t, err.Error(), path,
		"error message must include the file path for user debugging")
}

func TestLoadManifest_ValidFile_ReturnsToolkit(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "toolwright.yaml")
	content := `apiVersion: toolwright/v1
kind: Toolkit
metadata:
  name: test-tool
  version: 1.0.0
  description: A test tool
tools:
  - name: hello
    description: Say hello
    entrypoint: ./hello.sh
`
	err := os.WriteFile(path, []byte(content), 0644)
	require.NoError(t, err)

	tk, err := loadManifest(path)
	require.NoError(t, err, "loadManifest should succeed for a valid manifest file")
	require.NotNil(t, tk, "loadManifest must return a non-nil Toolkit for valid input")
	assert.Equal(t, "test-tool", tk.Metadata.Name,
		"loadManifest must parse manifest correctly")
	assert.Len(t, tk.Tools, 1,
		"loadManifest must parse all tools")
}

func TestLoadManifest_InvalidYAML_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.yaml")
	err := os.WriteFile(path, []byte("not: valid: yaml: [[["), 0644)
	require.NoError(t, err)

	_, err = loadManifest(path)
	require.Error(t, err, "loadManifest must return error for invalid YAML")
}

func TestLoadManifest_EmptyFile_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "empty.yaml")
	err := os.WriteFile(path, []byte(""), 0644)
	require.NoError(t, err)

	_, err = loadManifest(path)
	require.Error(t, err, "loadManifest must return error for empty file")
}

func TestLoadManifest_ValidFile_ParsesAllTools(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "toolwright.yaml")
	content := `apiVersion: toolwright/v1
kind: Toolkit
metadata:
  name: multi-tool
  version: 2.0.0
  description: Multiple tools
tools:
  - name: alpha
    description: First tool
    entrypoint: ./alpha.sh
  - name: beta
    description: Second tool
    entrypoint: ./beta.sh
  - name: gamma
    description: Third tool
    entrypoint: ./gamma.sh
`
	err := os.WriteFile(path, []byte(content), 0644)
	require.NoError(t, err)

	tk, err := loadManifest(path)
	require.NoError(t, err)
	require.NotNil(t, tk)
	require.Len(t, tk.Tools, 3,
		"loadManifest must parse all three tools")
	assert.Equal(t, "alpha", tk.Tools[0].Name)
	assert.Equal(t, "beta", tk.Tools[1].Name)
	assert.Equal(t, "gamma", tk.Tools[2].Name)
}

func TestLoadManifest_WrapsError(t *testing.T) {
	// Constitution rule 4: errors wrapped with context.
	_, err := loadManifest("/does/not/exist.yaml")
	require.Error(t, err)
	// The error should not be a bare os.PathError; it should have context.
	errMsg := err.Error()
	assert.Contains(t, errMsg, "/does/not/exist.yaml",
		"wrapped error must include the file path")
}
