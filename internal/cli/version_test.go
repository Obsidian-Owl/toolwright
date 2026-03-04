package cli

import (
	"bytes"
	"encoding/json"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Test helper
// ---------------------------------------------------------------------------

// executeVersionCmd runs the version command through the root command tree
// and returns stdout and the error (if any).
func executeVersionCmd(args ...string) (stdout string, err error) {
	root := NewRootCommand()
	root.AddCommand(newVersionCmd())
	outBuf := &bytes.Buffer{}
	root.SetOut(outBuf)
	root.SetErr(&bytes.Buffer{})
	root.SetArgs(append([]string{"version"}, args...))
	execErr := root.Execute()
	return outBuf.String(), execErr
}

// ---------------------------------------------------------------------------
// AC-20: version command structure
// ---------------------------------------------------------------------------

func TestNewVersionCmd_ReturnsNonNil(t *testing.T) {
	cmd := newVersionCmd()
	require.NotNil(t, cmd, "newVersionCmd must return a non-nil *cobra.Command")
}

func TestNewVersionCmd_HasCorrectUseField(t *testing.T) {
	cmd := newVersionCmd()
	assert.Equal(t, "version", cmd.Use,
		"version command Use field must be 'version'")
}

func TestNewVersionCmd_HasShortDescription(t *testing.T) {
	cmd := newVersionCmd()
	assert.NotEmpty(t, cmd.Short,
		"version command must have a Short description")
}

func TestNewVersionCmd_HasRunE(t *testing.T) {
	cmd := newVersionCmd()
	assert.NotNil(t, cmd.RunE,
		"version command must have a RunE function for error propagation")
}

// ---------------------------------------------------------------------------
// AC-20: version command produces output
// ---------------------------------------------------------------------------

func TestVersion_ProducesOutput(t *testing.T) {
	stdout, err := executeVersionCmd()
	require.NoError(t, err,
		"version command must not return an error")
	assert.NotEmpty(t, stdout,
		"version command must produce output to stdout")
}

// ---------------------------------------------------------------------------
// AC-20: version human output mentions toolwright and version
// ---------------------------------------------------------------------------

func TestVersion_HumanOutput_MentionsToolwright(t *testing.T) {
	stdout, err := executeVersionCmd()
	require.NoError(t, err)
	assert.Contains(t, strings.ToLower(stdout), "toolwright",
		"human version output must mention 'toolwright'")
}

func TestVersion_HumanOutput_ContainsVersionString(t *testing.T) {
	stdout, err := executeVersionCmd()
	require.NoError(t, err)

	// The output must contain some version string. In dev mode this is "dev".
	// We verify at least one known version indicator is present.
	hasVersion := strings.Contains(stdout, "dev") ||
		strings.Contains(stdout, "v0.") ||
		strings.Contains(stdout, "v1.") ||
		strings.Contains(stdout, Version)
	assert.True(t, hasVersion,
		"human version output must contain a version string (e.g., 'dev' or 'vX.Y.Z'); got: %s", stdout)
}

func TestVersion_HumanOutput_IsNotJSON(t *testing.T) {
	stdout, err := executeVersionCmd()
	require.NoError(t, err)

	// Without --json, output must NOT be JSON.
	var probe map[string]any
	jsonErr := json.Unmarshal([]byte(stdout), &probe)
	if jsonErr == nil {
		// If it parses as JSON, it should not have the structured version fields.
		_, hasVersion := probe["version"]
		assert.False(t, hasVersion,
			"without --json, output must not have structured JSON version fields")
	}
}

// ---------------------------------------------------------------------------
// AC-20: version --json outputs valid JSON
// ---------------------------------------------------------------------------

func TestVersion_JSON_IsValidJSON(t *testing.T) {
	stdout, err := executeVersionCmd("--json")
	require.NoError(t, err,
		"version --json must not return an error")
	require.NotEmpty(t, stdout,
		"version --json must produce output")
	assert.True(t, json.Valid([]byte(stdout)),
		"version --json must produce valid JSON; got: %s", stdout)
}

// ---------------------------------------------------------------------------
// AC-20: version JSON has all required fields
// ---------------------------------------------------------------------------

func TestVersion_JSON_HasRequiredFields(t *testing.T) {
	stdout, err := executeVersionCmd("--json")
	require.NoError(t, err)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result),
		"version --json output must be parseable JSON; got: %s", stdout)

	requiredFields := []string{"version", "go_version", "commit", "build_date"}
	for _, field := range requiredFields {
		assert.Contains(t, result, field,
			"version JSON must contain field %q; got keys: %v",
			field, mapKeys(result))
	}
}

// Table-driven: check each field individually for specificity.
func TestVersion_JSON_FieldValues_TableDriven(t *testing.T) {
	stdout, err := executeVersionCmd("--json")
	require.NoError(t, err)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))

	tests := []struct {
		field          string
		mustBeNonEmpty bool
		description    string
	}{
		{field: "version", mustBeNonEmpty: true, description: "version string"},
		{field: "go_version", mustBeNonEmpty: true, description: "Go version"},
		{field: "commit", mustBeNonEmpty: true, description: "commit hash or placeholder"},
		{field: "build_date", mustBeNonEmpty: true, description: "build date or placeholder"},
	}

	for _, tc := range tests {
		t.Run(tc.field, func(t *testing.T) {
			val, ok := result[tc.field]
			require.True(t, ok,
				"field %q must exist in version JSON", tc.field)

			if tc.mustBeNonEmpty {
				strVal, isStr := val.(string)
				require.True(t, isStr,
					"field %q must be a string; got %T", tc.field, val)
				assert.NotEmpty(t, strVal,
					"field %q must not be empty", tc.field)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// AC-20: version JSON go_version matches actual runtime
// ---------------------------------------------------------------------------

func TestVersion_JSON_GoVersionMatchesRuntime(t *testing.T) {
	stdout, err := executeVersionCmd("--json")
	require.NoError(t, err)

	var result map[string]string
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))

	goVer := result["go_version"]
	require.NotEmpty(t, goVer,
		"go_version must not be empty")

	// runtime.Version() returns something like "go1.25.0".
	// The version output must contain the actual Go version to catch a
	// hardcoded fake value.
	assert.Contains(t, goVer, runtime.Version(),
		"go_version in JSON must match the actual Go runtime version; "+
			"got %q, runtime reports %q", goVer, runtime.Version())
}

// ---------------------------------------------------------------------------
// AC-20: default version value when ldflags not set
// ---------------------------------------------------------------------------

func TestVersion_DefaultVersion_IsDev(t *testing.T) {
	// When the binary is not built with ldflags, the version should be "dev"
	// or a similar sentinel value. Since we are running via `go test`, ldflags
	// are not set.
	stdout, err := executeVersionCmd("--json")
	require.NoError(t, err)

	var result map[string]string
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))

	version := result["version"]
	assert.Equal(t, "dev", version,
		"when ldflags are not set (as in go test), version must default to 'dev'; got: %q", version)
}

func TestVersion_DefaultCommit_IsPlaceholder(t *testing.T) {
	// When ldflags are not set, commit should be a placeholder like "unknown".
	stdout, err := executeVersionCmd("--json")
	require.NoError(t, err)

	var result map[string]string
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))

	commit := result["commit"]
	assert.Equal(t, "unknown", commit,
		"when ldflags are not set, commit must default to 'unknown'; got: %q", commit)
}

func TestVersion_DefaultBuildDate_IsPlaceholder(t *testing.T) {
	// When ldflags are not set, build_date should be a placeholder like "unknown".
	stdout, err := executeVersionCmd("--json")
	require.NoError(t, err)

	var result map[string]string
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))

	buildDate := result["build_date"]
	assert.Equal(t, "unknown", buildDate,
		"when ldflags are not set, build_date must default to 'unknown'; got: %q", buildDate)
}

// ---------------------------------------------------------------------------
// AC-20: version --json is ONLY JSON (no mixed human text)
// ---------------------------------------------------------------------------

func TestVersion_JSON_NoExtraContent(t *testing.T) {
	stdout, err := executeVersionCmd("--json")
	require.NoError(t, err)

	// Trim whitespace and verify the entire stdout is a single JSON object.
	trimmed := strings.TrimSpace(stdout)
	require.True(t, strings.HasPrefix(trimmed, "{"),
		"version --json stdout must start with '{'; got: %s", stdout)
	require.True(t, strings.HasSuffix(trimmed, "}"),
		"version --json stdout must end with '}'; got: %s", stdout)

	// Verify it is valid as a whole.
	assert.True(t, json.Valid([]byte(trimmed)),
		"version --json stdout must be entirely valid JSON; got: %s", stdout)
}

// ---------------------------------------------------------------------------
// AC-20: version --json field count (no extra junk fields)
// ---------------------------------------------------------------------------

func TestVersion_JSON_FieldCount(t *testing.T) {
	stdout, err := executeVersionCmd("--json")
	require.NoError(t, err)

	var result map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))

	// Exactly 4 fields: version, go_version, commit, build_date.
	assert.Len(t, result, 4,
		"version JSON must have exactly 4 fields; got %d: %v",
		len(result), mapKeys(result))
}

// ---------------------------------------------------------------------------
// AC-20: version inherits --json persistent flag from root
// ---------------------------------------------------------------------------

func TestVersion_InheritsJsonFlag(t *testing.T) {
	root := NewRootCommand()
	versionCmd := newVersionCmd()
	root.AddCommand(versionCmd)

	// Parse args so flags get bound.
	root.SetArgs([]string{"version", "--json", "--help"})
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	_ = root.Execute()

	f := versionCmd.Flags().Lookup("json")
	require.NotNil(t, f,
		"--json must be accessible on the version subcommand via persistent flags")
}

// ---------------------------------------------------------------------------
// AC-20: version human output includes Go version
// ---------------------------------------------------------------------------

func TestVersion_HumanOutput_ContainsGoVersion(t *testing.T) {
	stdout, err := executeVersionCmd()
	require.NoError(t, err)

	// The human output should include the Go version somewhere.
	assert.Contains(t, stdout, runtime.Version(),
		"human version output should include the Go runtime version; got: %s", stdout)
}

// ---------------------------------------------------------------------------
// Anti-hardcoding: JSON output has real runtime values, not static strings
// ---------------------------------------------------------------------------

func TestVersion_JSON_GoVersionIsNotHardcoded(t *testing.T) {
	// A lazy implementation might hardcode "go1.21" or similar. We check
	// it matches the actual runtime, which varies per environment.
	stdout, err := executeVersionCmd("--json")
	require.NoError(t, err)

	var result map[string]string
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))

	goVer := result["go_version"]
	// The go_version must contain the actual Go version string.
	assert.Equal(t, runtime.Version(), goVer,
		"go_version must reflect the actual Go runtime, not a hardcoded value")
}

// ---------------------------------------------------------------------------
// Version variable exists and has expected default
// ---------------------------------------------------------------------------

func TestVersion_VariableDefault(t *testing.T) {
	// The package-level Version variable must exist and default to "dev"
	// when not overridden by ldflags.
	assert.Equal(t, "dev", Version,
		"Version package variable must default to 'dev'")
}

func TestCommit_VariableDefault(t *testing.T) {
	assert.Equal(t, "unknown", Commit,
		"Commit package variable must default to 'unknown'")
}

func TestBuildDate_VariableDefault(t *testing.T) {
	assert.Equal(t, "unknown", BuildDate,
		"BuildDate package variable must default to 'unknown'")
}

// ---------------------------------------------------------------------------
// Edge case: version command with no args, no flags
// ---------------------------------------------------------------------------

func TestVersion_NoArgs_Succeeds(t *testing.T) {
	stdout, err := executeVersionCmd()
	require.NoError(t, err,
		"version with no args must succeed")
	assert.NotEmpty(t, stdout,
		"version with no args must produce output")
}

// ---------------------------------------------------------------------------
// Edge case: version does not require a manifest file
// ---------------------------------------------------------------------------

func TestVersion_DoesNotRequireManifest(t *testing.T) {
	// Running version from a directory with no toolwright.yaml must succeed.
	// This is important: version should never load a manifest.
	stdout, err := executeVersionCmd()
	require.NoError(t, err,
		"version must succeed regardless of manifest presence")
	assert.NotEmpty(t, stdout)
}

// mapKeys is already defined in generate_test.go in this package.
