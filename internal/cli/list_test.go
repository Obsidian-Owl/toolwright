package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Test data helpers
// ---------------------------------------------------------------------------

// manifestWithToolAuth returns a valid manifest YAML with a single tool that
// has the given auth type set via string shorthand.
func manifestWithToolAuth(authType string) string {
	return `apiVersion: toolwright/v1
kind: Toolkit
metadata:
  name: test-tool
  version: 1.0.0
  description: A test tool
tools:
  - name: hello
    description: Say hello
    entrypoint: ./hello.sh
    auth: ` + authType + `
`
}

// manifestNoToolAuth returns a valid manifest with a tool that has no auth
// field at all (nil Auth pointer).
func manifestNoToolAuth() string {
	return `apiVersion: toolwright/v1
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
}

// manifestEmptyTools returns a valid manifest with an empty tools list.
func manifestEmptyTools() string {
	return `apiVersion: toolwright/v1
kind: Toolkit
metadata:
  name: empty-toolkit
  version: 1.0.0
  description: A toolkit with no tools
tools: []
`
}

// manifestMultipleTools returns a manifest with three tools, each with a
// distinct auth type, to test multi-tool listing.
func manifestMultipleTools() string {
	return `apiVersion: toolwright/v1
kind: Toolkit
metadata:
  name: multi-toolkit
  version: 2.0.0
  description: Multiple tools
tools:
  - name: alpha
    description: First tool
    entrypoint: ./alpha.sh
    auth: none
  - name: beta
    description: Second tool
    entrypoint: ./beta.sh
    auth: token
  - name: gamma
    description: Third tool
    entrypoint: ./gamma.sh
    auth: oauth2
`
}

// manifestAlternate returns a different manifest to catch hardcoded responses.
func manifestAlternate() string {
	return `apiVersion: toolwright/v1
kind: Toolkit
metadata:
  name: alternate-toolkit
  version: 3.0.0
  description: An alternate toolkit
tools:
  - name: zulu
    description: Zulu tool
    entrypoint: ./zulu.sh
    auth: token
  - name: yankee
    description: Yankee tool
    entrypoint: ./yankee.sh
`
}

// executeListCmd runs the list command through the root command tree and
// returns stdout, stderr, and the error (if any).
func executeListCmd(args ...string) (stdout, stderr string, err error) {
	root := NewRootCommand()
	list := newListCmd()
	root.AddCommand(list)
	outBuf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	root.SetOut(outBuf)
	root.SetErr(errBuf)
	root.SetArgs(append([]string{"list"}, args...))
	execErr := root.Execute()
	return outBuf.String(), errBuf.String(), execErr
}

// writeListManifest writes manifest content to a temp dir and returns the path.
func writeListManifest(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, "toolwright.yaml")
	err := os.WriteFile(path, []byte(content), 0644)
	require.NoError(t, err, "test setup: writing manifest file")
	return path
}

// listToolJSON represents a single tool in the JSON output of the list command.
type listToolJSON struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	AuthType    string `json:"auth_type"`
}

// listOutputJSON represents the JSON output structure of the list command.
type listOutputJSON struct {
	Tools []listToolJSON `json:"tools"`
}

// ---------------------------------------------------------------------------
// AC-8: newListCmd returns a properly configured Cobra command
// ---------------------------------------------------------------------------

func TestNewListCmd_ReturnsNonNil(t *testing.T) {
	cmd := newListCmd()
	require.NotNil(t, cmd, "newListCmd must return a non-nil *cobra.Command")
}

func TestNewListCmd_HasCorrectUseField(t *testing.T) {
	cmd := newListCmd()
	assert.Equal(t, "list", cmd.Use,
		"list command Use field must be 'list'")
}

func TestNewListCmd_InheritsJsonFlag(t *testing.T) {
	root := NewRootCommand()
	list := newListCmd()
	root.AddCommand(list)

	root.SetArgs([]string{"list", "--json", "--help"})
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	_ = root.Execute()

	f := list.Flags().Lookup("json")
	require.NotNil(t, f, "--json must be accessible on the list subcommand via persistent flags")
}

// ---------------------------------------------------------------------------
// AC-8: Valid manifest with tools -> human output contains names + descriptions
// ---------------------------------------------------------------------------

func TestList_HumanOutput_ContainsToolNames(t *testing.T) {
	dir := t.TempDir()
	path := writeListManifest(t, dir, manifestMultipleTools())

	stdout, _, err := executeListCmd(path)
	require.NoError(t, err, "list with valid manifest must not error")

	assert.Contains(t, stdout, "alpha",
		"human output must contain tool name 'alpha'")
	assert.Contains(t, stdout, "beta",
		"human output must contain tool name 'beta'")
	assert.Contains(t, stdout, "gamma",
		"human output must contain tool name 'gamma'")
}

func TestList_HumanOutput_ContainsToolDescriptions(t *testing.T) {
	dir := t.TempDir()
	path := writeListManifest(t, dir, manifestMultipleTools())

	stdout, _, err := executeListCmd(path)
	require.NoError(t, err)

	assert.Contains(t, stdout, "First tool",
		"human output must contain tool description 'First tool'")
	assert.Contains(t, stdout, "Second tool",
		"human output must contain tool description 'Second tool'")
	assert.Contains(t, stdout, "Third tool",
		"human output must contain tool description 'Third tool'")
}

func TestList_HumanOutput_ContainsAuthTypes(t *testing.T) {
	dir := t.TempDir()
	path := writeListManifest(t, dir, manifestMultipleTools())

	stdout, _, err := executeListCmd(path)
	require.NoError(t, err)

	// The auth types must appear in the output. We check that each
	// distinct type shows at least once (not just a generic "auth" label).
	assert.Contains(t, stdout, "none",
		"human output must contain auth type 'none'")
	assert.Contains(t, stdout, "token",
		"human output must contain auth type 'token'")
	assert.Contains(t, stdout, "oauth2",
		"human output must contain auth type 'oauth2'")
}

// ---------------------------------------------------------------------------
// AC-8: Valid manifest with tools, --json -> structured JSON output
// ---------------------------------------------------------------------------

func TestList_JSON_HasCorrectStructure(t *testing.T) {
	dir := t.TempDir()
	path := writeListManifest(t, dir, manifestWithToolAuth("token"))

	stdout, _, err := executeListCmd("--json", path)
	require.NoError(t, err, "list --json with valid manifest must not error")

	var result listOutputJSON
	require.NoError(t, json.Unmarshal([]byte(stdout), &result),
		"--json output must be valid JSON matching {tools: [...]}, got: %s", stdout)

	require.Len(t, result.Tools, 1,
		"manifest with one tool must produce exactly one tool in JSON output")

	tool := result.Tools[0]
	assert.Equal(t, "hello", tool.Name,
		"tool name in JSON must match manifest")
	assert.Equal(t, "Say hello", tool.Description,
		"tool description in JSON must match manifest")
	assert.Equal(t, "token", tool.AuthType,
		"tool auth_type in JSON must match manifest auth type")
}

func TestList_JSON_OutputIsOnlyJSON(t *testing.T) {
	dir := t.TempDir()
	path := writeListManifest(t, dir, manifestWithToolAuth("none"))

	stdout, _, err := executeListCmd("--json", path)
	require.NoError(t, err)

	require.True(t, json.Valid([]byte(stdout)),
		"with --json, stdout must contain ONLY valid JSON, got: %s", stdout)
}

func TestList_JSON_TopLevelHasToolsKey(t *testing.T) {
	dir := t.TempDir()
	path := writeListManifest(t, dir, manifestWithToolAuth("none"))

	stdout, _, err := executeListCmd("--json", path)
	require.NoError(t, err)

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(stdout), &raw))

	_, hasTools := raw["tools"]
	assert.True(t, hasTools,
		"JSON output must have a top-level 'tools' key, got keys: %v", keysOf(raw))
}

func TestList_JSON_ToolHasAllRequiredFields(t *testing.T) {
	dir := t.TempDir()
	path := writeListManifest(t, dir, manifestWithToolAuth("token"))

	stdout, _, err := executeListCmd("--json", path)
	require.NoError(t, err)

	// Parse as raw to check field names exist
	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(stdout), &raw))

	var tools []map[string]json.RawMessage
	require.NoError(t, json.Unmarshal(raw["tools"], &tools))
	require.Len(t, tools, 1)

	tool := tools[0]
	assert.Contains(t, tool, "name",
		"each tool in JSON must have 'name' field")
	assert.Contains(t, tool, "description",
		"each tool in JSON must have 'description' field")
	assert.Contains(t, tool, "auth_type",
		"each tool in JSON must have 'auth_type' field")
}

// ---------------------------------------------------------------------------
// AC-8: Auth type variations in JSON output
// ---------------------------------------------------------------------------

func TestList_JSON_AuthTypes(t *testing.T) {
	tests := []struct {
		name     string
		manifest string
		wantAuth string
	}{
		{
			name:     "auth none",
			manifest: manifestWithToolAuth("none"),
			wantAuth: "none",
		},
		{
			name:     "auth token",
			manifest: manifestWithToolAuth("token"),
			wantAuth: "token",
		},
		{
			name:     "auth oauth2",
			manifest: manifestWithToolAuth("oauth2"),
			wantAuth: "oauth2",
		},
		{
			name:     "auth nil defaults to none",
			manifest: manifestNoToolAuth(),
			wantAuth: "none",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := writeListManifest(t, dir, tc.manifest)

			stdout, _, err := executeListCmd("--json", path)
			require.NoError(t, err,
				"list --json must not error for valid manifest")

			var result listOutputJSON
			require.NoError(t, json.Unmarshal([]byte(stdout), &result),
				"output must be valid JSON, got: %s", stdout)
			require.Len(t, result.Tools, 1,
				"manifest with one tool must have one tool in output")

			assert.Equal(t, tc.wantAuth, result.Tools[0].AuthType,
				"auth_type must be %q when auth is %q", tc.wantAuth, tc.name)
		})
	}
}

// ---------------------------------------------------------------------------
// AC-8: Empty tools list -> empty table / empty array (not error)
// ---------------------------------------------------------------------------

func TestList_EmptyTools_HumanOutput_NoError(t *testing.T) {
	dir := t.TempDir()
	path := writeListManifest(t, dir, manifestEmptyTools())

	stdout, _, err := executeListCmd(path)
	require.NoError(t, err,
		"list with empty tools must not error")

	// The output should not contain tool-like content -- it should be
	// empty or contain only a header/message about no tools.
	// We cannot assert the output is completely empty (there may be a header),
	// but we verify no error occurred and no tool names leaked in.
	assert.NotContains(t, stdout, "hello",
		"empty tools list should not contain any tool name")
}

func TestList_EmptyTools_JSON_EmptyArray(t *testing.T) {
	dir := t.TempDir()
	path := writeListManifest(t, dir, manifestEmptyTools())

	stdout, _, err := executeListCmd("--json", path)
	require.NoError(t, err,
		"list --json with empty tools must not error")

	var result listOutputJSON
	require.NoError(t, json.Unmarshal([]byte(stdout), &result),
		"output must be valid JSON, got: %s", stdout)

	require.NotNil(t, result.Tools,
		"tools field must not be null in JSON output")
	assert.Empty(t, result.Tools,
		"tools array must be empty for manifest with no tools")
}

func TestList_EmptyTools_JSON_ToolsIsEmptyArray_NotNull(t *testing.T) {
	dir := t.TempDir()
	path := writeListManifest(t, dir, manifestEmptyTools())

	stdout, _, err := executeListCmd("--json", path)
	require.NoError(t, err)

	// Parse as raw JSON to verify the tools field is [] not null
	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(stdout), &raw))

	toolsRaw := strings.TrimSpace(string(raw["tools"]))
	assert.Equal(t, "[]", toolsRaw,
		"tools field must be an empty JSON array [], not null; got: %s", toolsRaw)
}

// ---------------------------------------------------------------------------
// AC-8: Multiple tools -> all appear in JSON output with correct data
// ---------------------------------------------------------------------------

func TestList_MultipleTools_JSON_AllPresent(t *testing.T) {
	dir := t.TempDir()
	path := writeListManifest(t, dir, manifestMultipleTools())

	stdout, _, err := executeListCmd("--json", path)
	require.NoError(t, err)

	var result listOutputJSON
	require.NoError(t, json.Unmarshal([]byte(stdout), &result),
		"output must be valid JSON, got: %s", stdout)

	require.Len(t, result.Tools, 3,
		"manifest with 3 tools must produce 3 tools in JSON output")

	// Verify each tool has the correct name, description, and auth_type.
	// Order must match manifest order.
	expected := []listToolJSON{
		{Name: "alpha", Description: "First tool", AuthType: "none"},
		{Name: "beta", Description: "Second tool", AuthType: "token"},
		{Name: "gamma", Description: "Third tool", AuthType: "oauth2"},
	}

	for i, want := range expected {
		got := result.Tools[i]
		assert.Equal(t, want.Name, got.Name,
			"tool[%d] name must be %q", i, want.Name)
		assert.Equal(t, want.Description, got.Description,
			"tool[%d] description must be %q", i, want.Description)
		assert.Equal(t, want.AuthType, got.AuthType,
			"tool[%d] auth_type must be %q", i, want.AuthType)
	}
}

func TestList_MultipleTools_HumanOutput_AllPresent(t *testing.T) {
	dir := t.TempDir()
	path := writeListManifest(t, dir, manifestMultipleTools())

	stdout, _, err := executeListCmd(path)
	require.NoError(t, err)

	// All three tools must appear
	for _, name := range []string{"alpha", "beta", "gamma"} {
		assert.Contains(t, stdout, name,
			"human output must contain tool name %q", name)
	}
	for _, desc := range []string{"First tool", "Second tool", "Third tool"} {
		assert.Contains(t, stdout, desc,
			"human output must contain tool description %q", desc)
	}
}

// ---------------------------------------------------------------------------
// AC-8: Anti-hardcoding: different manifests produce different JSON output
// ---------------------------------------------------------------------------

func TestList_JSON_DifferentManifests_DifferentOutput(t *testing.T) {
	dir1 := t.TempDir()
	path1 := writeListManifest(t, dir1, manifestMultipleTools())

	dir2 := t.TempDir()
	path2 := writeListManifest(t, dir2, manifestAlternate())

	stdout1, _, err1 := executeListCmd("--json", path1)
	stdout2, _, err2 := executeListCmd("--json", path2)
	require.NoError(t, err1)
	require.NoError(t, err2)

	var result1, result2 listOutputJSON
	require.NoError(t, json.Unmarshal([]byte(stdout1), &result1))
	require.NoError(t, json.Unmarshal([]byte(stdout2), &result2))

	// Different manifests must produce different tool lists
	assert.NotEqual(t, len(result1.Tools), len(result2.Tools),
		"manifests with different tool counts must produce different-length arrays; "+
			"manifest1 has %d tools, manifest2 has %d tools",
		len(result1.Tools), len(result2.Tools))

	// Verify manifest2 has its own tool names, not manifest1's
	names2 := make(map[string]bool)
	for _, tool := range result2.Tools {
		names2[tool.Name] = true
	}
	assert.True(t, names2["zulu"],
		"alternate manifest must show 'zulu' tool, got: %+v", result2.Tools)
	assert.True(t, names2["yankee"],
		"alternate manifest must show 'yankee' tool, got: %+v", result2.Tools)
	assert.False(t, names2["alpha"],
		"alternate manifest must NOT show 'alpha' (from first manifest)")
}

func TestList_JSON_ToolDataReflectsManifest_NotStatic(t *testing.T) {
	// Create two manifests with same structure but different names/descriptions
	// to verify the command reads from the manifest, not returns canned data.
	m1 := `apiVersion: toolwright/v1
kind: Toolkit
metadata:
  name: toolkit-a
  version: 1.0.0
  description: Toolkit A
tools:
  - name: foo-tool
    description: Does foo things
    entrypoint: ./foo.sh
    auth: token
`
	m2 := `apiVersion: toolwright/v1
kind: Toolkit
metadata:
  name: toolkit-b
  version: 1.0.0
  description: Toolkit B
tools:
  - name: bar-tool
    description: Does bar things
    entrypoint: ./bar.sh
    auth: oauth2
`
	dir1 := t.TempDir()
	path1 := writeListManifest(t, dir1, m1)
	dir2 := t.TempDir()
	path2 := writeListManifest(t, dir2, m2)

	stdout1, _, err1 := executeListCmd("--json", path1)
	stdout2, _, err2 := executeListCmd("--json", path2)
	require.NoError(t, err1)
	require.NoError(t, err2)

	var result1, result2 listOutputJSON
	require.NoError(t, json.Unmarshal([]byte(stdout1), &result1))
	require.NoError(t, json.Unmarshal([]byte(stdout2), &result2))

	require.Len(t, result1.Tools, 1)
	require.Len(t, result2.Tools, 1)

	assert.Equal(t, "foo-tool", result1.Tools[0].Name,
		"first manifest tool name must be 'foo-tool'")
	assert.Equal(t, "Does foo things", result1.Tools[0].Description,
		"first manifest tool description must match")
	assert.Equal(t, "token", result1.Tools[0].AuthType,
		"first manifest tool auth_type must be 'token'")

	assert.Equal(t, "bar-tool", result2.Tools[0].Name,
		"second manifest tool name must be 'bar-tool'")
	assert.Equal(t, "Does bar things", result2.Tools[0].Description,
		"second manifest tool description must match")
	assert.Equal(t, "oauth2", result2.Tools[0].AuthType,
		"second manifest tool auth_type must be 'oauth2'")
}

// ---------------------------------------------------------------------------
// AC-8: Default path -- no arg -> tries toolwright.yaml in current dir
// ---------------------------------------------------------------------------

func TestList_DefaultPath_UsesToolwrightYaml(t *testing.T) {
	dir := t.TempDir()
	writeListManifest(t, dir, manifestWithToolAuth("none"))

	original, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { os.Chdir(original) })

	stdout, _, err := executeListCmd("--json")
	require.NoError(t, err,
		"list with no args must default to toolwright.yaml and succeed if present")

	var result listOutputJSON
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	require.Len(t, result.Tools, 1)
	assert.Equal(t, "hello", result.Tools[0].Name,
		"default path should find and list tools from toolwright.yaml")
}

// ---------------------------------------------------------------------------
// AC-8: File not found -> error
// ---------------------------------------------------------------------------

func TestList_FileNotFound_ReturnsError(t *testing.T) {
	_, _, err := executeListCmd("/nonexistent/path/toolwright.yaml")
	require.Error(t, err,
		"list with a nonexistent file must return an error")
}

func TestList_FileNotFound_HumanOutput_HasPath(t *testing.T) {
	fakePath := "/nonexistent/path/toolwright.yaml"
	stdout, stderr, _ := executeListCmd(fakePath)
	output := stdout + stderr

	assert.Contains(t, output, fakePath,
		"error output must include the path that was not found")
}

// ---------------------------------------------------------------------------
// AC-8: --json with file not found -> JSON error output
// ---------------------------------------------------------------------------

func TestList_FileNotFound_JSON_HasErrorStructure(t *testing.T) {
	fakePath := "/nonexistent/path/toolwright.yaml"
	stdout, _, _ := executeListCmd("--json", fakePath)

	require.NotEmpty(t, stdout,
		"JSON output must be produced even for file-not-found errors")

	require.True(t, json.Valid([]byte(stdout)),
		"output with --json must be valid JSON even for file errors, got: %s", stdout)

	// Must contain error structure
	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &got))

	errObj, ok := got["error"].(map[string]any)
	require.True(t, ok,
		"JSON error output must have top-level 'error' object, got: %v", got)

	assert.Contains(t, errObj, "code",
		"error object must have 'code' field")
	assert.Contains(t, errObj, "message",
		"error object must have 'message' field")
}

func TestList_FileNotFound_JSON_DoesNotContainToolsKey(t *testing.T) {
	fakePath := "/nonexistent/path/toolwright.yaml"
	stdout, _, _ := executeListCmd("--json", fakePath)

	if !json.Valid([]byte(stdout)) {
		t.Skipf("stdout is not valid JSON; other tests cover this: %s", stdout)
	}

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(stdout), &raw))

	_, hasTools := raw["tools"]
	assert.False(t, hasTools,
		"error JSON must NOT contain 'tools' key; it should be an error object only")
}

// ---------------------------------------------------------------------------
// AC-8: Human output is tabular (has column-like structure)
// ---------------------------------------------------------------------------

func TestList_HumanOutput_IsTabular(t *testing.T) {
	dir := t.TempDir()
	path := writeListManifest(t, dir, manifestMultipleTools())

	stdout, _, err := executeListCmd(path)
	require.NoError(t, err)

	lines := strings.Split(strings.TrimSpace(stdout), "\n")

	// A table must have at least a header + 3 data rows (3 tools)
	require.GreaterOrEqual(t, len(lines), 3,
		"human output for 3 tools must have at least 3 lines (header + data), got %d lines: %s",
		len(lines), stdout)
}

func TestList_HumanOutput_HeaderContainsColumnNames(t *testing.T) {
	dir := t.TempDir()
	path := writeListManifest(t, dir, manifestMultipleTools())

	stdout, _, err := executeListCmd(path)
	require.NoError(t, err)

	upper := strings.ToUpper(stdout)
	// The table header should contain column labels. We check case-insensitively.
	assert.Contains(t, upper, "NAME",
		"human table output should have a 'NAME' column header")
	assert.Contains(t, upper, "DESCRIPTION",
		"human table output should have a 'DESCRIPTION' column header")
	assert.Contains(t, upper, "AUTH",
		"human table output should have an 'AUTH' column header")
}

// ---------------------------------------------------------------------------
// AC-8: JSON output has no stderr pollution
// ---------------------------------------------------------------------------

func TestList_JSON_NothingOnStderr(t *testing.T) {
	dir := t.TempDir()
	path := writeListManifest(t, dir, manifestWithToolAuth("none"))

	_, stderr, err := executeListCmd("--json", path)
	require.NoError(t, err)

	assert.Empty(t, stderr,
		"with --json, stderr must be empty (Constitution rule 18)")
}

// ---------------------------------------------------------------------------
// AC-8: Human output without --json does NOT produce JSON structure
// ---------------------------------------------------------------------------

func TestList_HumanOutput_NotJSON(t *testing.T) {
	dir := t.TempDir()
	path := writeListManifest(t, dir, manifestWithToolAuth("none"))

	stdout, _, err := executeListCmd(path)
	require.NoError(t, err)

	var probe map[string]any
	jsonErr := json.Unmarshal([]byte(stdout), &probe)
	if jsonErr == nil {
		_, hasTools := probe["tools"]
		assert.False(t, hasTools,
			"without --json, stdout should not contain the list JSON structure")
	}
}

// ---------------------------------------------------------------------------
// AC-8: Tool ordering in JSON matches manifest order
// ---------------------------------------------------------------------------

func TestList_JSON_ToolOrder_MatchesManifest(t *testing.T) {
	dir := t.TempDir()
	path := writeListManifest(t, dir, manifestMultipleTools())

	stdout, _, err := executeListCmd("--json", path)
	require.NoError(t, err)

	var result listOutputJSON
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	require.Len(t, result.Tools, 3)

	// Manifest order is alpha, beta, gamma
	assert.Equal(t, "alpha", result.Tools[0].Name,
		"first tool must be 'alpha' (manifest order)")
	assert.Equal(t, "beta", result.Tools[1].Name,
		"second tool must be 'beta' (manifest order)")
	assert.Equal(t, "gamma", result.Tools[2].Name,
		"third tool must be 'gamma' (manifest order)")
}

// ---------------------------------------------------------------------------
// AC-8: Toolkit-level auth is inherited when tool has no auth
// ---------------------------------------------------------------------------

func TestList_JSON_ToolkitLevelAuth_InheritedByTool(t *testing.T) {
	m := `apiVersion: toolwright/v1
kind: Toolkit
metadata:
  name: inherited-auth
  version: 1.0.0
  description: Toolkit-level auth test
auth:
  type: oauth2
  provider_url: https://example.com
  scopes:
    - read
tools:
  - name: inheriter
    description: Inherits toolkit auth
    entrypoint: ./inheriter.sh
`
	dir := t.TempDir()
	path := writeListManifest(t, dir, m)

	stdout, _, err := executeListCmd("--json", path)
	require.NoError(t, err)

	var result listOutputJSON
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	require.Len(t, result.Tools, 1)

	assert.Equal(t, "oauth2", result.Tools[0].AuthType,
		"tool with no auth should inherit toolkit-level auth type 'oauth2'")
}

func TestList_JSON_ToolAuthOverridesToolkitAuth(t *testing.T) {
	m := `apiVersion: toolwright/v1
kind: Toolkit
metadata:
  name: override-auth
  version: 1.0.0
  description: Tool-level auth overrides toolkit
auth:
  type: oauth2
  provider_url: https://example.com
  scopes:
    - read
tools:
  - name: overrider
    description: Has its own auth
    entrypoint: ./overrider.sh
    auth: token
`
	dir := t.TempDir()
	path := writeListManifest(t, dir, m)

	stdout, _, err := executeListCmd("--json", path)
	require.NoError(t, err)

	var result listOutputJSON
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	require.Len(t, result.Tools, 1)

	assert.Equal(t, "token", result.Tools[0].AuthType,
		"tool-level auth 'token' must override toolkit-level auth 'oauth2'")
}
