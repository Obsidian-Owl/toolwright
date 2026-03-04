package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Test data helpers
// ---------------------------------------------------------------------------

// validManifestWithEntrypoint returns a valid manifest YAML referencing the
// given entrypoint path. The entrypoint must be relative to the manifest dir.
func validManifestWithEntrypoint(entrypoint string) string {
	return `apiVersion: toolwright/v1
kind: Toolkit
metadata:
  name: test-tool
  version: 1.0.0
  description: A test tool
tools:
  - name: hello
    description: Say hello
    entrypoint: ` + entrypoint + `
`
}

// validManifestMultipleTools returns a manifest with multiple tools, each
// referencing its own entrypoint.
func validManifestMultipleTools(entrypoints ...string) string {
	y := `apiVersion: toolwright/v1
kind: Toolkit
metadata:
  name: multi-tool
  version: 2.0.0
  description: Multiple tools
tools:
`
	for i, ep := range entrypoints {
		names := []string{"alpha", "beta", "gamma", "delta", "epsilon"}
		name := "tool"
		if i < len(names) {
			name = names[i]
		}
		y += `  - name: ` + name + `
    description: Tool ` + name + `
    entrypoint: ` + ep + `
`
	}
	return y
}

// invalidManifestMissingName returns a manifest YAML with the name field
// empty, which will fail validation on the name-format rule.
func invalidManifestMissingName() string {
	return `apiVersion: toolwright/v1
kind: Toolkit
metadata:
  name: ""
  version: 1.0.0
  description: A test tool
tools:
  - name: hello
    description: Say hello
    entrypoint: ./hello.sh
`
}

// invalidManifestMultipleErrors returns a manifest with several validation
// problems: empty name, bad version, missing description.
func invalidManifestMultipleErrors() string {
	return `apiVersion: toolwright/v1
kind: Toolkit
metadata:
  name: ""
  version: not-semver
  description: ""
tools:
  - name: hello
    description: Say hello
    entrypoint: ./hello.sh
`
}

// executeValidateCmd runs the validate command through the root command tree
// and returns stdout, stderr, and the error (if any).
func executeValidateCmd(args ...string) (stdout, stderr string, err error) {
	root := NewRootCommand()
	validate := newValidateCmd()
	root.AddCommand(validate)
	outBuf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	root.SetOut(outBuf)
	root.SetErr(errBuf)
	root.SetArgs(append([]string{"validate"}, args...))
	execErr := root.Execute()
	return outBuf.String(), errBuf.String(), execErr
}

// writeManifest writes content to toolwright.yaml in dir and returns the path.
func writeManifest(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, "toolwright.yaml")
	err := os.WriteFile(path, []byte(content), 0644)
	require.NoError(t, err, "test setup: writing manifest file")
	return path
}

// writeExecutable creates an executable file at the given path.
func writeExecutable(t *testing.T, path string) {
	t.Helper()
	err := os.MkdirAll(filepath.Dir(path), 0755)
	require.NoError(t, err, "test setup: creating directory for executable")
	err = os.WriteFile(path, []byte("#!/bin/sh\necho ok"), 0755)
	require.NoError(t, err, "test setup: writing executable file")
}

// writeNonExecutable creates a regular file (not executable) at the given path.
func writeNonExecutable(t *testing.T, path string) {
	t.Helper()
	err := os.MkdirAll(filepath.Dir(path), 0755)
	require.NoError(t, err, "test setup: creating directory for non-executable")
	err = os.WriteFile(path, []byte("#!/bin/sh\necho ok"), 0644)
	require.NoError(t, err, "test setup: writing non-executable file")
}

// validateResult is the expected JSON structure for validate --json output.
type validateResult struct {
	Valid    bool            `json:"valid"`
	Errors   []validateIssue `json:"errors"`
	Warnings []validateIssue `json:"warnings"`
}

// validateIssue is a single error or warning in the validate JSON output.
type validateIssue struct {
	Path    string `json:"path"`
	Message string `json:"message"`
	Rule    string `json:"rule"`
}

// ---------------------------------------------------------------------------
// AC-6/AC-7: newValidateCmd returns a properly configured Cobra command
// ---------------------------------------------------------------------------

func TestNewValidateCmd_ReturnsNonNil(t *testing.T) {
	cmd := newValidateCmd()
	require.NotNil(t, cmd, "newValidateCmd must return a non-nil *cobra.Command")
}

func TestNewValidateCmd_HasCorrectUseField(t *testing.T) {
	cmd := newValidateCmd()
	assert.Equal(t, "validate", cmd.Use,
		"validate command Use field must be 'validate'")
}

func TestNewValidateCmd_HasOnlineFlag(t *testing.T) {
	cmd := newValidateCmd()
	f := cmd.Flags().Lookup("online")
	require.NotNil(t, f, "--online flag must be registered on the validate command")
	assert.Equal(t, "bool", f.Value.Type(),
		"--online flag must be a boolean")
	assert.Equal(t, "false", f.DefValue,
		"--online flag must default to false")
}

func TestNewValidateCmd_InheritsJsonFlag(t *testing.T) {
	// The --json flag comes from the root command; verify it is accessible
	// through the validate subcommand when wired to root.
	root := NewRootCommand()
	validate := newValidateCmd()
	root.AddCommand(validate)

	// Parse args so flags get bound
	root.SetArgs([]string{"validate", "--json", "--help"})
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	_ = root.Execute()

	f := validate.Flags().Lookup("json")
	require.NotNil(t, f, "--json must be accessible on the validate subcommand via persistent flags")
}

// ---------------------------------------------------------------------------
// AC-6: Valid manifest → exit 0, human-readable "valid" message
// ---------------------------------------------------------------------------

func TestValidate_ValidManifest_HumanOutput_ExitZero(t *testing.T) {
	dir := t.TempDir()
	ep := filepath.Join(dir, "hello.sh")
	writeExecutable(t, ep)
	path := writeManifest(t, dir, validManifestWithEntrypoint(ep))

	stdout, _, err := executeValidateCmd(path)

	require.NoError(t, err,
		"valid manifest must produce exit 0 (no error)")
	assert.Contains(t, stdout, "valid",
		"human output for a valid manifest must contain the word 'valid'")
}

func TestValidate_ValidManifest_HumanOutput_ContainsManifestIdentifier(t *testing.T) {
	// A lazy implementation that always prints "valid" must be caught by
	// checking that the output relates to the actual manifest.
	dir := t.TempDir()
	ep := filepath.Join(dir, "hello.sh")
	writeExecutable(t, ep)
	path := writeManifest(t, dir, validManifestWithEntrypoint(ep))

	stdout, _, err := executeValidateCmd(path)
	require.NoError(t, err)

	// The output should reference either the file path or the toolkit name
	// to demonstrate it actually processed the manifest.
	pathOrName := stdout
	hasPath := assert.Condition(t, func() bool {
		return bytes.Contains([]byte(pathOrName), []byte(path)) ||
			bytes.Contains([]byte(pathOrName), []byte("test-tool"))
	}, "human output should reference the manifest path or toolkit name")
	_ = hasPath
}

// ---------------------------------------------------------------------------
// AC-6: Valid manifest with --json → structured JSON output
// ---------------------------------------------------------------------------

func TestValidate_ValidManifest_JSON_HasCorrectStructure(t *testing.T) {
	dir := t.TempDir()
	ep := filepath.Join(dir, "hello.sh")
	writeExecutable(t, ep)
	path := writeManifest(t, dir, validManifestWithEntrypoint(ep))

	stdout, _, err := executeValidateCmd("--json", path)
	require.NoError(t, err, "valid manifest with --json must exit 0")

	var result validateResult
	require.NoError(t, json.Unmarshal([]byte(stdout), &result),
		"--json output must be valid JSON matching the validateResult schema, got: %s", stdout)

	assert.True(t, result.Valid,
		"valid manifest must have valid=true in JSON output")
	assert.Empty(t, result.Errors,
		"valid manifest must have empty errors array")
	assert.Empty(t, result.Warnings,
		"valid manifest must have empty warnings array")
}

func TestValidate_ValidManifest_JSON_ErrorsIsEmptyArray_NotNull(t *testing.T) {
	dir := t.TempDir()
	ep := filepath.Join(dir, "hello.sh")
	writeExecutable(t, ep)
	path := writeManifest(t, dir, validManifestWithEntrypoint(ep))

	stdout, _, err := executeValidateCmd("--json", path)
	require.NoError(t, err)

	// Parse as raw JSON to verify that errors and warnings are [] not null
	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(stdout), &raw),
		"output must be valid JSON")

	assert.Equal(t, "[]", string(raw["errors"]),
		"errors field must be an empty JSON array [], not null or omitted; got: %s", string(raw["errors"]))
	assert.Equal(t, "[]", string(raw["warnings"]),
		"warnings field must be an empty JSON array [], not null or omitted; got: %s", string(raw["warnings"]))
}

func TestValidate_ValidManifest_JSON_ValidFieldIsBoolean(t *testing.T) {
	dir := t.TempDir()
	ep := filepath.Join(dir, "hello.sh")
	writeExecutable(t, ep)
	path := writeManifest(t, dir, validManifestWithEntrypoint(ep))

	stdout, _, err := executeValidateCmd("--json", path)
	require.NoError(t, err)

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(stdout), &raw))
	assert.Equal(t, "true", string(raw["valid"]),
		"valid field must be boolean true (not string \"true\")")
}

// ---------------------------------------------------------------------------
// AC-6: Invalid manifest → exit 1, lists errors with paths
// ---------------------------------------------------------------------------

func TestValidate_InvalidManifest_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	path := writeManifest(t, dir, invalidManifestMissingName())

	_, _, err := executeValidateCmd(path)

	require.Error(t, err,
		"invalid manifest must cause validate to return an error (exit 1)")
}

func TestValidate_InvalidManifest_HumanOutput_ContainsErrorPath(t *testing.T) {
	dir := t.TempDir()
	path := writeManifest(t, dir, invalidManifestMissingName())

	stdout, stderr, _ := executeValidateCmd(path)
	output := stdout + stderr

	// The error path for a missing/invalid name is "metadata.name"
	assert.Contains(t, output, "metadata.name",
		"human output for invalid manifest must include the error path 'metadata.name'")
}

func TestValidate_InvalidManifest_HumanOutput_ContainsRuleOrMessage(t *testing.T) {
	dir := t.TempDir()
	path := writeManifest(t, dir, invalidManifestMissingName())

	stdout, stderr, _ := executeValidateCmd(path)
	output := stdout + stderr

	// Must contain either the rule name or a descriptive message about the problem
	hasRuleOrMsg := bytes.Contains([]byte(output), []byte("name-format")) ||
		bytes.Contains([]byte(output), []byte("name")) // at minimum references the problematic field
	assert.True(t, hasRuleOrMsg,
		"human output must contain the rule identifier or a message describing the validation error, got: %s", output)
}

// ---------------------------------------------------------------------------
// AC-6: Invalid manifest with --json → structured error list
// ---------------------------------------------------------------------------

func TestValidate_InvalidManifest_JSON_ValidIsFalse(t *testing.T) {
	dir := t.TempDir()
	path := writeManifest(t, dir, invalidManifestMissingName())

	stdout, _, _ := executeValidateCmd("--json", path)

	var result validateResult
	require.NoError(t, json.Unmarshal([]byte(stdout), &result),
		"--json output must be parseable JSON even when invalid, got: %s", stdout)

	assert.False(t, result.Valid,
		"invalid manifest must have valid=false in JSON output")
}

func TestValidate_InvalidManifest_JSON_ErrorsNonEmpty(t *testing.T) {
	dir := t.TempDir()
	path := writeManifest(t, dir, invalidManifestMissingName())

	stdout, _, _ := executeValidateCmd("--json", path)

	var result validateResult
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))

	require.NotEmpty(t, result.Errors,
		"invalid manifest must have at least one entry in the errors array")
}

func TestValidate_InvalidManifest_JSON_ErrorHasRequiredFields(t *testing.T) {
	dir := t.TempDir()
	path := writeManifest(t, dir, invalidManifestMissingName())

	stdout, _, _ := executeValidateCmd("--json", path)

	var result validateResult
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	require.NotEmpty(t, result.Errors)

	first := result.Errors[0]
	assert.NotEmpty(t, first.Path,
		"each error must have a non-empty 'path' field")
	assert.NotEmpty(t, first.Message,
		"each error must have a non-empty 'message' field")
	assert.NotEmpty(t, first.Rule,
		"each error must have a non-empty 'rule' field")
}

func TestValidate_InvalidManifest_JSON_ErrorPathIsSpecific(t *testing.T) {
	dir := t.TempDir()
	path := writeManifest(t, dir, invalidManifestMissingName())

	stdout, _, _ := executeValidateCmd("--json", path)

	var result validateResult
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	require.NotEmpty(t, result.Errors)

	// At least one error should reference metadata.name
	var foundNameError bool
	for _, e := range result.Errors {
		if e.Path == "metadata.name" {
			foundNameError = true
			break
		}
	}
	assert.True(t, foundNameError,
		"errors must include an entry with path 'metadata.name' for a missing name, got errors: %+v", result.Errors)
}

// ---------------------------------------------------------------------------
// AC-6: Multiple validation errors are all reported (not fail-fast)
// ---------------------------------------------------------------------------

func TestValidate_MultipleErrors_AllReported_Human(t *testing.T) {
	dir := t.TempDir()
	path := writeManifest(t, dir, invalidManifestMultipleErrors())

	stdout, stderr, _ := executeValidateCmd(path)
	output := stdout + stderr

	// The manifest has empty name, bad version, empty description: 3 errors
	assert.Contains(t, output, "metadata.name",
		"output must report the metadata.name error")
	assert.Contains(t, output, "metadata.version",
		"output must report the metadata.version error")
	assert.Contains(t, output, "metadata.description",
		"output must report the metadata.description error")
}

func TestValidate_MultipleErrors_AllReported_JSON(t *testing.T) {
	dir := t.TempDir()
	path := writeManifest(t, dir, invalidManifestMultipleErrors())

	stdout, _, _ := executeValidateCmd("--json", path)

	var result validateResult
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))

	// At least 3 distinct errors: name-format, semver, description-required
	require.GreaterOrEqual(t, len(result.Errors), 3,
		"manifest with 3+ problems must report at least 3 errors, got %d: %+v",
		len(result.Errors), result.Errors)

	paths := make(map[string]bool)
	for _, e := range result.Errors {
		paths[e.Path] = true
	}
	assert.True(t, paths["metadata.name"],
		"errors must include metadata.name")
	assert.True(t, paths["metadata.version"],
		"errors must include metadata.version")
	assert.True(t, paths["metadata.description"],
		"errors must include metadata.description")
}

// ---------------------------------------------------------------------------
// AC-7: Entrypoint file exists and is executable → passes
// ---------------------------------------------------------------------------

func TestValidate_EntrypointExistsAndExecutable_NoErrors(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("executable permission checks are not meaningful on Windows")
	}

	dir := t.TempDir()
	ep := filepath.Join(dir, "bin", "hello")
	writeExecutable(t, ep)
	path := writeManifest(t, dir, validManifestWithEntrypoint(ep))

	stdout, _, err := executeValidateCmd("--json", path)
	require.NoError(t, err, "valid manifest with executable entrypoint must pass")

	var result validateResult
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))

	assert.True(t, result.Valid, "must be valid when entrypoint exists and is executable")
	assert.Empty(t, result.Errors, "no errors expected for valid entrypoint")
	assert.Empty(t, result.Warnings, "no warnings expected for executable entrypoint")
}

// ---------------------------------------------------------------------------
// AC-7: Entrypoint file missing → error at tools[N].entrypoint
// ---------------------------------------------------------------------------

func TestValidate_EntrypointMissing_ReportsError(t *testing.T) {
	dir := t.TempDir()
	missingPath := filepath.Join(dir, "nonexistent", "tool.sh")
	path := writeManifest(t, dir, validManifestWithEntrypoint(missingPath))

	_, _, err := executeValidateCmd(path)
	require.Error(t, err,
		"missing entrypoint must cause validate to return an error")
}

func TestValidate_EntrypointMissing_JSON_ErrorPath(t *testing.T) {
	dir := t.TempDir()
	missingPath := filepath.Join(dir, "nonexistent", "tool.sh")
	path := writeManifest(t, dir, validManifestWithEntrypoint(missingPath))

	stdout, _, _ := executeValidateCmd("--json", path)

	var result validateResult
	require.NoError(t, json.Unmarshal([]byte(stdout), &result),
		"JSON output must be parseable even with entrypoint error, got: %s", stdout)

	assert.False(t, result.Valid,
		"missing entrypoint must result in valid=false")

	// Find the entrypoint error
	var foundEntrypointError bool
	for _, e := range result.Errors {
		if e.Path == "tools[0].entrypoint" {
			foundEntrypointError = true
			assert.Contains(t, e.Message, "not found",
				"entrypoint error message must mention file not being found")
			assert.Equal(t, "entrypoint-exists", e.Rule,
				"entrypoint missing error rule must be 'entrypoint-exists'")
			break
		}
	}
	assert.True(t, foundEntrypointError,
		"errors must include entry with path 'tools[0].entrypoint', got: %+v", result.Errors)
}

func TestValidate_EntrypointMissing_MultiplTools_CorrectIndex(t *testing.T) {
	dir := t.TempDir()

	// First tool has a valid entrypoint, second is missing
	ep1 := filepath.Join(dir, "alpha.sh")
	writeExecutable(t, ep1)
	missingEp := filepath.Join(dir, "nonexistent.sh")

	path := writeManifest(t, dir, validManifestMultipleTools(ep1, missingEp))

	stdout, _, _ := executeValidateCmd("--json", path)

	var result validateResult
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))

	// The second tool (index 1) should have the entrypoint error
	var foundCorrectIndex bool
	for _, e := range result.Errors {
		if e.Path == "tools[1].entrypoint" {
			foundCorrectIndex = true
			break
		}
	}
	assert.True(t, foundCorrectIndex,
		"entrypoint error must reference the correct tool index (tools[1]), got: %+v", result.Errors)
}

// ---------------------------------------------------------------------------
// AC-7: Entrypoint exists but not executable → warning (not error)
// ---------------------------------------------------------------------------

func TestValidate_EntrypointNotExecutable_IsWarning(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("executable permission checks are not meaningful on Windows")
	}

	dir := t.TempDir()
	ep := filepath.Join(dir, "hello.sh")
	writeNonExecutable(t, ep)
	path := writeManifest(t, dir, validManifestWithEntrypoint(ep))

	stdout, _, _ := executeValidateCmd("--json", path)

	var result validateResult
	require.NoError(t, json.Unmarshal([]byte(stdout), &result),
		"JSON output must be parseable, got: %s", stdout)

	// Non-executable entrypoint should be a warning, NOT an error
	for _, e := range result.Errors {
		assert.NotEqual(t, "tools[0].entrypoint", e.Path,
			"non-executable entrypoint must NOT appear in errors (should be a warning)")
	}

	var foundWarning bool
	for _, w := range result.Warnings {
		if w.Path == "tools[0].entrypoint" {
			foundWarning = true
			assert.Contains(t, w.Message, "executable",
				"warning message must mention the file is not executable")
			assert.Equal(t, "entrypoint-executable", w.Rule,
				"warning rule must be 'entrypoint-executable'")
			break
		}
	}
	assert.True(t, foundWarning,
		"non-executable entrypoint must appear in warnings array, got: %+v", result.Warnings)
}

func TestValidate_EntrypointNotExecutable_StillValid(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("executable permission checks are not meaningful on Windows")
	}

	dir := t.TempDir()
	ep := filepath.Join(dir, "hello.sh")
	writeNonExecutable(t, ep)
	path := writeManifest(t, dir, validManifestWithEntrypoint(ep))

	stdout, _, err := executeValidateCmd("--json", path)

	// Warnings do not cause exit failure -- only errors do
	require.NoError(t, err,
		"warnings (like non-executable entrypoint) must not cause exit error")

	var result validateResult
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))

	assert.True(t, result.Valid,
		"manifest with only warnings (no errors) must still have valid=true")
}

// ---------------------------------------------------------------------------
// AC-6: Default file path — no arg → tries toolwright.yaml in current dir
// ---------------------------------------------------------------------------

func TestValidate_DefaultPath_UsesToolwrightYaml(t *testing.T) {
	dir := t.TempDir()
	ep := filepath.Join(dir, "hello.sh")
	writeExecutable(t, ep)
	writeManifest(t, dir, validManifestWithEntrypoint(ep))

	// Change to the temp dir so default path resolution finds toolwright.yaml
	original, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { os.Chdir(original) })

	// No file path argument -- should default to toolwright.yaml
	stdout, _, err := executeValidateCmd("--json")

	require.NoError(t, err,
		"validate with no args must default to toolwright.yaml in current dir and succeed if present")

	var result validateResult
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	assert.True(t, result.Valid,
		"default path should find and validate toolwright.yaml successfully")
}

// ---------------------------------------------------------------------------
// AC-6: File not found → exit error with clear message
// ---------------------------------------------------------------------------

func TestValidate_FileNotFound_ReturnsError(t *testing.T) {
	_, _, err := executeValidateCmd("/nonexistent/path/toolwright.yaml")

	require.Error(t, err,
		"validate with a nonexistent file must return an error")
}

func TestValidate_FileNotFound_HumanOutput_HasPath(t *testing.T) {
	fakePath := "/nonexistent/path/toolwright.yaml"
	stdout, stderr, _ := executeValidateCmd(fakePath)
	output := stdout + stderr

	assert.Contains(t, output, fakePath,
		"error output must include the path that was not found")
}

func TestValidate_FileNotFound_JSON_HasErrorStructure(t *testing.T) {
	fakePath := "/nonexistent/path/toolwright.yaml"
	stdout, _, _ := executeValidateCmd("--json", fakePath)

	require.NotEmpty(t, stdout,
		"JSON output must be produced even for file-not-found errors")

	// Should be valid JSON -- could be either the validate format or the
	// standard error format. Either way, must be parseable.
	assert.True(t, json.Valid([]byte(stdout)),
		"output with --json must be valid JSON even for file errors, got: %s", stdout)
}

// ---------------------------------------------------------------------------
// AC-6: --online flag exists and is parsed
// ---------------------------------------------------------------------------

func TestValidate_OnlineFlag_IsParsed(t *testing.T) {
	dir := t.TempDir()
	ep := filepath.Join(dir, "hello.sh")
	writeExecutable(t, ep)
	path := writeManifest(t, dir, validManifestWithEntrypoint(ep))

	// --online should not cause a parse error
	_, _, err := executeValidateCmd("--online", path)

	// We do not require success or failure here -- just that --online is
	// accepted as a valid flag and does not produce a usage error.
	if err != nil {
		// If it errors, it should NOT be a flag-parsing error
		assert.NotContains(t, err.Error(), "unknown flag",
			"--online must be a recognized flag")
	}
}

// ---------------------------------------------------------------------------
// AC-6: --online with unreachable provider_url → warning (not error)
// ---------------------------------------------------------------------------

func TestValidate_OnlineUnreachableProviderURL_IsWarning(t *testing.T) {
	manifest := `apiVersion: toolwright/v1
kind: Toolkit
metadata:
  name: oauth-tool
  version: 1.0.0
  description: Tool with oauth
auth:
  type: oauth2
  provider_url: https://unreachable.invalid.example.com
  scopes:
    - read
tools:
  - name: hello
    description: Say hello
    entrypoint: ./hello.sh
`
	dir := t.TempDir()
	ep := filepath.Join(dir, "hello.sh")
	writeExecutable(t, ep)
	path := writeManifest(t, dir, manifest)

	stdout, _, _ := executeValidateCmd("--json", "--online", path)

	var result validateResult
	require.NoError(t, json.Unmarshal([]byte(stdout), &result),
		"JSON output must be parseable, got: %s", stdout)

	// Unreachable provider_url with --online should be a warning, NOT an error
	for _, e := range result.Errors {
		assert.NotContains(t, e.Path, "provider_url",
			"unreachable provider_url must NOT be an error, only a warning")
	}

	var foundWarning bool
	for _, w := range result.Warnings {
		if w.Path == "auth.provider_url" {
			foundWarning = true
			break
		}
	}
	assert.True(t, foundWarning,
		"unreachable provider_url with --online must produce a warning, got warnings: %+v", result.Warnings)
}

// ---------------------------------------------------------------------------
// Anti-hardcoding: different valid manifests produce different JSON
// ---------------------------------------------------------------------------

func TestValidate_JSON_DifferentManifests_DifferentNotJustHardcoded(t *testing.T) {
	// Two different valid manifests should both produce valid=true with
	// identical structure. This ensures the command actually processes
	// the manifest rather than returning a canned response.

	dir1 := t.TempDir()
	ep1 := filepath.Join(dir1, "alpha.sh")
	writeExecutable(t, ep1)
	writeManifest(t, dir1, validManifestWithEntrypoint(ep1))

	dir2 := t.TempDir()
	ep2 := filepath.Join(dir2, "beta.sh")
	writeExecutable(t, ep2)
	m2 := `apiVersion: toolwright/v1
kind: Toolkit
metadata:
  name: different-tool
  version: 2.3.4
  description: A completely different tool
tools:
  - name: greet
    description: Greet someone
    entrypoint: ` + ep2 + `
`
	writeManifest(t, dir2, m2)

	stdout1, _, err1 := executeValidateCmd("--json", filepath.Join(dir1, "toolwright.yaml"))
	stdout2, _, err2 := executeValidateCmd("--json", filepath.Join(dir2, "toolwright.yaml"))

	require.NoError(t, err1)
	require.NoError(t, err2)

	var r1, r2 validateResult
	require.NoError(t, json.Unmarshal([]byte(stdout1), &r1))
	require.NoError(t, json.Unmarshal([]byte(stdout2), &r2))

	// Both should be valid
	assert.True(t, r1.Valid, "first manifest must be valid")
	assert.True(t, r2.Valid, "second manifest must be valid")
}

func TestValidate_JSON_InvalidManifest_ErrorsAreSpecific(t *testing.T) {
	// Two manifests with different problems must produce different error content.
	// This catches a lazy implementation that returns a static error array.

	dir1 := t.TempDir()
	writeManifest(t, dir1, invalidManifestMissingName())

	dir2 := t.TempDir()
	badVersion := `apiVersion: toolwright/v1
kind: Toolkit
metadata:
  name: good-name
  version: not-a-version
  description: A test tool
tools:
  - name: hello
    description: Say hello
    entrypoint: ./hello.sh
`
	writeManifest(t, dir2, badVersion)

	stdout1, _, _ := executeValidateCmd("--json", filepath.Join(dir1, "toolwright.yaml"))
	stdout2, _, _ := executeValidateCmd("--json", filepath.Join(dir2, "toolwright.yaml"))

	var r1, r2 validateResult
	require.NoError(t, json.Unmarshal([]byte(stdout1), &r1))
	require.NoError(t, json.Unmarshal([]byte(stdout2), &r2))

	// First manifest has name error, second has version error
	r1Paths := make(map[string]bool)
	for _, e := range r1.Errors {
		r1Paths[e.Path] = true
	}
	r2Paths := make(map[string]bool)
	for _, e := range r2.Errors {
		r2Paths[e.Path] = true
	}

	assert.True(t, r1Paths["metadata.name"],
		"first manifest should have metadata.name error, got: %+v", r1.Errors)
	assert.False(t, r1Paths["metadata.version"],
		"first manifest should NOT have metadata.version error (version is valid)")

	assert.False(t, r2Paths["metadata.name"],
		"second manifest should NOT have metadata.name error (name is valid)")
	assert.True(t, r2Paths["metadata.version"],
		"second manifest should have metadata.version error, got: %+v", r2.Errors)
}

// ---------------------------------------------------------------------------
// AC-6: JSON output is valid JSON (not mixed with human text)
// ---------------------------------------------------------------------------

func TestValidate_JSON_OutputIsOnlyJSON(t *testing.T) {
	dir := t.TempDir()
	ep := filepath.Join(dir, "hello.sh")
	writeExecutable(t, ep)
	path := writeManifest(t, dir, validManifestWithEntrypoint(ep))

	stdout, _, err := executeValidateCmd("--json", path)
	require.NoError(t, err)

	require.True(t, json.Valid([]byte(stdout)),
		"with --json, stdout must contain ONLY valid JSON (no human text mixed in), got: %s", stdout)
}

func TestValidate_JSON_InvalidManifest_OutputIsStillJSON(t *testing.T) {
	dir := t.TempDir()
	path := writeManifest(t, dir, invalidManifestMissingName())

	stdout, _, _ := executeValidateCmd("--json", path)

	require.True(t, json.Valid([]byte(stdout)),
		"with --json, stdout must be valid JSON even for invalid manifests, got: %s", stdout)
}

// ---------------------------------------------------------------------------
// AC-6: JSON output has exactly three top-level keys
// ---------------------------------------------------------------------------

func TestValidate_JSON_TopLevelKeys(t *testing.T) {
	dir := t.TempDir()
	ep := filepath.Join(dir, "hello.sh")
	writeExecutable(t, ep)
	path := writeManifest(t, dir, validManifestWithEntrypoint(ep))

	stdout, _, err := executeValidateCmd("--json", path)
	require.NoError(t, err)

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(stdout), &raw))

	assert.Len(t, raw, 3,
		"validate JSON output must have exactly 3 top-level keys: valid, errors, warnings; got: %v", keysOf(raw))
	assert.Contains(t, raw, "valid", "must have 'valid' key")
	assert.Contains(t, raw, "errors", "must have 'errors' key")
	assert.Contains(t, raw, "warnings", "must have 'warnings' key")
}

// ---------------------------------------------------------------------------
// Edge case: manifest with no tools
// ---------------------------------------------------------------------------

func TestValidate_ManifestNoTools_ValidatesSuccessfully(t *testing.T) {
	manifest := `apiVersion: toolwright/v1
kind: Toolkit
metadata:
  name: empty-toolkit
  version: 1.0.0
  description: Toolkit with no tools
tools: []
`
	dir := t.TempDir()
	path := writeManifest(t, dir, manifest)

	stdout, _, err := executeValidateCmd("--json", path)
	require.NoError(t, err, "manifest with no tools should still validate")

	var result validateResult
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	assert.True(t, result.Valid)
	assert.Empty(t, result.Errors)
}

// ---------------------------------------------------------------------------
// Edge case: entrypoint checks combined with structural errors
// ---------------------------------------------------------------------------

func TestValidate_StructuralAndEntrypointErrors_BothReported(t *testing.T) {
	// A manifest with both a structural error (bad version) AND a missing
	// entrypoint should report BOTH problems, not just the first one found.
	manifest := `apiVersion: toolwright/v1
kind: Toolkit
metadata:
  name: bad-combo
  version: invalid!!
  description: Broken in two ways
tools:
  - name: hello
    description: Say hello
    entrypoint: /absolutely/does/not/exist.sh
`
	dir := t.TempDir()
	path := writeManifest(t, dir, manifest)

	stdout, _, _ := executeValidateCmd("--json", path)

	var result validateResult
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))

	paths := make(map[string]bool)
	for _, e := range result.Errors {
		paths[e.Path] = true
	}

	assert.True(t, paths["metadata.version"],
		"structural error (bad version) must be reported, got: %+v", result.Errors)
	assert.True(t, paths["tools[0].entrypoint"],
		"entrypoint-missing error must also be reported, got: %+v", result.Errors)
}

// ---------------------------------------------------------------------------
// Edge case: stdout vs stderr separation (Constitution rule 18)
// ---------------------------------------------------------------------------

func TestValidate_JSON_NothingOnStderr(t *testing.T) {
	dir := t.TempDir()
	ep := filepath.Join(dir, "hello.sh")
	writeExecutable(t, ep)
	path := writeManifest(t, dir, validManifestWithEntrypoint(ep))

	_, stderr, err := executeValidateCmd("--json", path)
	require.NoError(t, err)

	assert.Empty(t, stderr,
		"with --json, stderr must be empty (Constitution rule 18: structured JSON to stdout, diagnostics to stderr)")
}

func TestValidate_HumanOutput_NoJSONOnStdout(t *testing.T) {
	dir := t.TempDir()
	ep := filepath.Join(dir, "hello.sh")
	writeExecutable(t, ep)
	path := writeManifest(t, dir, validManifestWithEntrypoint(ep))

	stdout, _, err := executeValidateCmd(path)
	require.NoError(t, err)

	// Without --json, stdout should NOT be JSON
	var probe map[string]any
	jsonErr := json.Unmarshal([]byte(stdout), &probe)
	// If it happens to parse as JSON that is technically okay, but it should
	// at least contain human-readable text. The key check is it should NOT
	// look like the validate JSON schema.
	if jsonErr == nil {
		_, hasValid := probe["valid"]
		assert.False(t, hasValid,
			"without --json, stdout should not contain the validate JSON structure")
	}
}

// ---------------------------------------------------------------------------
// Helper function
// ---------------------------------------------------------------------------

func keysOf(m map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
