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
// Test data helpers
// ---------------------------------------------------------------------------

// manifestWithArgsAndFlags returns a manifest YAML containing a single tool
// with specified args and flags, for testing describe output.
func manifestWithArgsAndFlags() string {
	return `apiVersion: toolwright/v1
kind: Toolkit
metadata:
  name: test-toolkit
  version: 1.0.0
  description: A test toolkit
tools:
  - name: scan
    description: Scan for vulnerabilities
    entrypoint: ./scan.sh
    auth: token
    args:
      - name: target
        type: string
        required: true
        description: Scan target
      - name: profile
        type: string
        required: false
        description: Scan profile to use
    flags:
      - name: severity
        type: string
        required: false
        description: Min severity
        default: low
      - name: verbose
        type: boolean
        required: true
        description: Enable verbose output
`
}

// manifestWithNoArgsNoFlags returns a manifest YAML with a tool that has
// neither args nor flags.
func manifestWithNoArgsNoFlags() string {
	return `apiVersion: toolwright/v1
kind: Toolkit
metadata:
  name: simple-toolkit
  version: 1.0.0
  description: Simple toolkit
tools:
  - name: ping
    description: Ping the service
    entrypoint: ./ping.sh
    auth: none
`
}

// manifestWithTwoTools returns a manifest with two differently-configured
// tools, used for anti-hardcoding tests and tool lookup tests.
func manifestWithTwoTools() string {
	return `apiVersion: toolwright/v1
kind: Toolkit
metadata:
  name: dual-toolkit
  version: 1.0.0
  description: Dual toolkit
tools:
  - name: deploy
    description: Deploy the application
    entrypoint: ./deploy.sh
    auth: oauth2
    args:
      - name: environment
        type: string
        required: true
        description: Target environment
    flags:
      - name: dry-run
        type: boolean
        required: false
        description: Simulate deployment
  - name: rollback
    description: Rollback deployment
    entrypoint: ./rollback.sh
    auth: token
    flags:
      - name: revision
        type: integer
        required: true
        description: Revision to rollback to
`
}

// manifestNoAuth returns a manifest with a tool that has no auth field set.
func manifestNoAuth() string {
	return `apiVersion: toolwright/v1
kind: Toolkit
metadata:
  name: noauth-toolkit
  version: 1.0.0
  description: No auth toolkit
tools:
  - name: status
    description: Check status
    entrypoint: ./status.sh
`
}

// manifestOnlyArgs returns a manifest with a tool that has only args, no flags.
func manifestOnlyArgs() string {
	return `apiVersion: toolwright/v1
kind: Toolkit
metadata:
  name: args-toolkit
  version: 1.0.0
  description: Args only
tools:
  - name: greet
    description: Greet a person
    entrypoint: ./greet.sh
    auth: none
    args:
      - name: name
        type: string
        required: true
        description: Name to greet
      - name: greeting
        type: string
        required: false
        description: Custom greeting
`
}

// manifestOnlyFlags returns a manifest with a tool that has only flags, no args.
func manifestOnlyFlags() string {
	return `apiVersion: toolwright/v1
kind: Toolkit
metadata:
  name: flags-toolkit
  version: 1.0.0
  description: Flags only
tools:
  - name: configure
    description: Configure settings
    entrypoint: ./configure.sh
    auth: none
    flags:
      - name: timeout
        type: integer
        required: false
        description: Timeout in seconds
      - name: retries
        type: integer
        required: true
        description: Number of retries
`
}

// executeDescribeCmd runs the describe command through the root command tree
// and returns stdout, stderr, and the error (if any).
func executeDescribeCmd(args ...string) (stdout, stderr string, err error) {
	root := NewRootCommand()
	describe := newDescribeCmd()
	root.AddCommand(describe)
	outBuf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	root.SetOut(outBuf)
	root.SetErr(errBuf)
	root.SetArgs(append([]string{"describe"}, args...))
	execErr := root.Execute()
	return outBuf.String(), errBuf.String(), execErr
}

// writeDescribeManifest writes manifest content to a temp dir and returns the path.
func writeDescribeManifest(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, "toolwright.yaml")
	err := os.WriteFile(path, []byte(content), 0644)
	require.NoError(t, err, "test setup: writing manifest file")
	return path
}

// describeOutput represents the expected JSON output of the describe command
// for the default "json" format.
type describeOutput struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Auth        *describeAuth   `json:"auth,omitempty"`
	Parameters  *describeSchema `json:"parameters,omitempty"`
	InputSchema *describeSchema `json:"inputSchema,omitempty"`
}

type describeAuth struct {
	Type string `json:"type"`
}

type describeSchema struct {
	Type       string                      `json:"type"`
	Properties map[string]describeProperty `json:"properties"`
	Required   []string                    `json:"required"`
}

type describeProperty struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

// ---------------------------------------------------------------------------
// AC-9: newDescribeCmd returns a properly configured Cobra command
// ---------------------------------------------------------------------------

func TestNewDescribeCmd_ReturnsNonNil(t *testing.T) {
	cmd := newDescribeCmd()
	require.NotNil(t, cmd, "newDescribeCmd must return a non-nil *cobra.Command")
}

func TestNewDescribeCmd_HasCorrectUseField(t *testing.T) {
	cmd := newDescribeCmd()
	assert.Equal(t, "describe <tool-name>", cmd.Use,
		"describe command Use field must be 'describe <tool-name>'")
}

func TestNewDescribeCmd_HasFormatFlag(t *testing.T) {
	cmd := newDescribeCmd()
	f := cmd.Flags().Lookup("format")
	require.NotNil(t, f, "--format flag must be registered on the describe command")
	assert.Equal(t, "string", f.Value.Type(),
		"--format flag must be a string")
	assert.Equal(t, "json", f.DefValue,
		"--format flag must default to 'json'")
}

func TestNewDescribeCmd_HasFormatShortFlag(t *testing.T) {
	cmd := newDescribeCmd()
	f := cmd.Flags().ShorthandLookup("f")
	require.NotNil(t, f, "-f shorthand must be registered for --format")
	assert.Equal(t, "format", f.Name,
		"-f must be shorthand for --format")
}

func TestNewDescribeCmd_HasManifestFlag(t *testing.T) {
	cmd := newDescribeCmd()
	f := cmd.Flags().Lookup("manifest")
	require.NotNil(t, f, "--manifest flag must be registered on the describe command")
	assert.Equal(t, "string", f.Value.Type(),
		"--manifest flag must be a string")
}

func TestNewDescribeCmd_HasManifestShortFlag(t *testing.T) {
	cmd := newDescribeCmd()
	f := cmd.Flags().ShorthandLookup("m")
	require.NotNil(t, f, "-m shorthand must be registered for --manifest")
	assert.Equal(t, "manifest", f.Name,
		"-m must be shorthand for --manifest")
}

func TestNewDescribeCmd_InheritsJsonFlag(t *testing.T) {
	root := NewRootCommand()
	describe := newDescribeCmd()
	root.AddCommand(describe)

	root.SetArgs([]string{"describe", "--json", "--help"})
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	_ = root.Execute()

	f := describe.Flags().Lookup("json")
	require.NotNil(t, f, "--json must be accessible on the describe subcommand via persistent flags")
}

// ---------------------------------------------------------------------------
// AC-9: Missing tool name -> error (required arg)
// ---------------------------------------------------------------------------

func TestDescribe_MissingToolName_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	path := writeDescribeManifest(t, dir, manifestWithArgsAndFlags())

	_, _, err := executeDescribeCmd("-m", path)
	require.Error(t, err,
		"describe without a tool name argument must return an error")
}

func TestDescribe_MissingToolName_ErrorMentionsUsage(t *testing.T) {
	dir := t.TempDir()
	path := writeDescribeManifest(t, dir, manifestWithArgsAndFlags())

	stdout, stderr, _ := executeDescribeCmd("-m", path)
	output := stdout + stderr

	// The error should indicate that a tool name is required.
	hasUsageHint := assert.Condition(t, func() bool {
		return bytes.Contains([]byte(output), []byte("tool")) ||
			bytes.Contains([]byte(output), []byte("required")) ||
			bytes.Contains([]byte(output), []byte("argument")) ||
			bytes.Contains([]byte(output), []byte("usage"))
	}, "error output must indicate that a tool name argument is required, got: %s", output)
	_ = hasUsageHint
}

// ---------------------------------------------------------------------------
// AC-9: Unknown tool name -> error with "tool not found"
// ---------------------------------------------------------------------------

func TestDescribe_UnknownTool_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	path := writeDescribeManifest(t, dir, manifestWithArgsAndFlags())

	_, _, err := executeDescribeCmd("nonexistent-tool", "-m", path)
	require.Error(t, err,
		"describe with an unknown tool name must return an error")
}

func TestDescribe_UnknownTool_ErrorContainsToolName(t *testing.T) {
	dir := t.TempDir()
	path := writeDescribeManifest(t, dir, manifestWithArgsAndFlags())

	stdout, stderr, _ := executeDescribeCmd("xyz", "-m", path)
	output := stdout + stderr

	assert.Contains(t, output, `"xyz"`,
		"error output must include the unknown tool name in quotes")
	assert.Contains(t, output, "not found",
		"error output must indicate the tool was not found")
}

func TestDescribe_UnknownTool_JSON_HasErrorStructure(t *testing.T) {
	dir := t.TempDir()
	path := writeDescribeManifest(t, dir, manifestWithArgsAndFlags())

	stdout, _, _ := executeDescribeCmd("--json", "xyz", "-m", path)

	require.NotEmpty(t, stdout,
		"JSON output must be produced for unknown tool errors")
	require.True(t, json.Valid([]byte(stdout)),
		"output with --json must be valid JSON for unknown tool error, got: %s", stdout)

	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &got))

	errObj, ok := got["error"].(map[string]any)
	require.True(t, ok,
		"JSON error output must have top-level 'error' object, got: %v", got)

	assert.Equal(t, "tool_not_found", errObj["code"],
		"error code must be 'tool_not_found'")
	assert.Contains(t, errObj["message"], "xyz",
		"error message must include the tool name that was not found")
}

func TestDescribe_UnknownTool_JSON_DoesNotContainSchemaKeys(t *testing.T) {
	dir := t.TempDir()
	path := writeDescribeManifest(t, dir, manifestWithArgsAndFlags())

	stdout, _, _ := executeDescribeCmd("--json", "nonexistent", "-m", path)

	if !json.Valid([]byte(stdout)) {
		t.Skipf("stdout is not valid JSON; other tests cover this: %s", stdout)
	}

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(stdout), &raw))

	_, hasName := raw["name"]
	_, hasParams := raw["parameters"]
	_, hasInput := raw["inputSchema"]
	assert.False(t, hasName,
		"error JSON must NOT contain 'name' key from describe output")
	assert.False(t, hasParams,
		"error JSON must NOT contain 'parameters' key")
	assert.False(t, hasInput,
		"error JSON must NOT contain 'inputSchema' key")
}

// ---------------------------------------------------------------------------
// AC-9: Valid tool, default format (json) -> full schema output
// ---------------------------------------------------------------------------

func TestDescribe_ValidTool_DefaultFormat_OutputIsValidJSON(t *testing.T) {
	dir := t.TempDir()
	path := writeDescribeManifest(t, dir, manifestWithArgsAndFlags())

	stdout, _, err := executeDescribeCmd("scan", "-m", path)
	require.NoError(t, err, "describe with a valid tool name must not error")
	require.True(t, json.Valid([]byte(stdout)),
		"describe output must be valid JSON, got: %s", stdout)
}

func TestDescribe_ValidTool_DefaultFormat_HasName(t *testing.T) {
	dir := t.TempDir()
	path := writeDescribeManifest(t, dir, manifestWithArgsAndFlags())

	stdout, _, err := executeDescribeCmd("scan", "-m", path)
	require.NoError(t, err)

	var result describeOutput
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	assert.Equal(t, "scan", result.Name,
		"describe output must include the tool name")
}

func TestDescribe_ValidTool_DefaultFormat_HasDescription(t *testing.T) {
	dir := t.TempDir()
	path := writeDescribeManifest(t, dir, manifestWithArgsAndFlags())

	stdout, _, err := executeDescribeCmd("scan", "-m", path)
	require.NoError(t, err)

	var result describeOutput
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	assert.Equal(t, "Scan for vulnerabilities", result.Description,
		"describe output must include the tool description")
}

func TestDescribe_ValidTool_DefaultFormat_HasAuth(t *testing.T) {
	dir := t.TempDir()
	path := writeDescribeManifest(t, dir, manifestWithArgsAndFlags())

	stdout, _, err := executeDescribeCmd("scan", "-m", path)
	require.NoError(t, err)

	var result describeOutput
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	require.NotNil(t, result.Auth,
		"describe output in default format must include auth object")
	assert.Equal(t, "token", result.Auth.Type,
		"auth.type must reflect the tool's auth type")
}

func TestDescribe_ValidTool_DefaultFormat_HasParameters(t *testing.T) {
	dir := t.TempDir()
	path := writeDescribeManifest(t, dir, manifestWithArgsAndFlags())

	stdout, _, err := executeDescribeCmd("scan", "-m", path)
	require.NoError(t, err)

	var result describeOutput
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	require.NotNil(t, result.Parameters,
		"describe output in default format must include parameters object")
	assert.Equal(t, "object", result.Parameters.Type,
		"parameters.type must be 'object'")
}

func TestDescribe_ValidTool_DefaultFormat_HasNoInputSchema(t *testing.T) {
	dir := t.TempDir()
	path := writeDescribeManifest(t, dir, manifestWithArgsAndFlags())

	stdout, _, err := executeDescribeCmd("scan", "-m", path)
	require.NoError(t, err)

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(stdout), &raw))
	_, hasInputSchema := raw["inputSchema"]
	assert.False(t, hasInputSchema,
		"default format must NOT have 'inputSchema' key (that is MCP format)")
}

// ---------------------------------------------------------------------------
// AC-9: Parameters from args appear as properties
// ---------------------------------------------------------------------------

func TestDescribe_ArgsAppearAsProperties(t *testing.T) {
	dir := t.TempDir()
	path := writeDescribeManifest(t, dir, manifestWithArgsAndFlags())

	stdout, _, err := executeDescribeCmd("scan", "-m", path)
	require.NoError(t, err)

	var result describeOutput
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	require.NotNil(t, result.Parameters)

	// "target" is an arg
	targetProp, ok := result.Parameters.Properties["target"]
	require.True(t, ok,
		"args must appear as properties in parameters.properties; 'target' missing from %v",
		result.Parameters.Properties)
	assert.Equal(t, "string", targetProp.Type,
		"arg type must map to JSON Schema type")
	assert.Equal(t, "Scan target", targetProp.Description,
		"arg description must be preserved in property")

	// "profile" is an arg
	profileProp, ok := result.Parameters.Properties["profile"]
	require.True(t, ok,
		"args must appear as properties; 'profile' missing from %v",
		result.Parameters.Properties)
	assert.Equal(t, "string", profileProp.Type)
	assert.Equal(t, "Scan profile to use", profileProp.Description)
}

// ---------------------------------------------------------------------------
// AC-9: Parameters from flags appear as properties
// ---------------------------------------------------------------------------

func TestDescribe_FlagsAppearAsProperties(t *testing.T) {
	dir := t.TempDir()
	path := writeDescribeManifest(t, dir, manifestWithArgsAndFlags())

	stdout, _, err := executeDescribeCmd("scan", "-m", path)
	require.NoError(t, err)

	var result describeOutput
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	require.NotNil(t, result.Parameters)

	// "severity" is a flag
	sevProp, ok := result.Parameters.Properties["severity"]
	require.True(t, ok,
		"flags must appear as properties in parameters.properties; 'severity' missing from %v",
		result.Parameters.Properties)
	assert.Equal(t, "string", sevProp.Type,
		"flag type must map to JSON Schema type")
	assert.Equal(t, "Min severity", sevProp.Description,
		"flag description must be preserved in property")

	// "verbose" is a flag
	verbProp, ok := result.Parameters.Properties["verbose"]
	require.True(t, ok,
		"flags must appear as properties; 'verbose' missing from %v",
		result.Parameters.Properties)
	assert.Equal(t, "boolean", verbProp.Type,
		"flag type 'boolean' must map to JSON Schema type 'boolean'")
	assert.Equal(t, "Enable verbose output", verbProp.Description)
}

// ---------------------------------------------------------------------------
// AC-9: Required args/flags -> in "required" array
// ---------------------------------------------------------------------------

func TestDescribe_RequiredArgsInRequiredArray(t *testing.T) {
	dir := t.TempDir()
	path := writeDescribeManifest(t, dir, manifestWithArgsAndFlags())

	stdout, _, err := executeDescribeCmd("scan", "-m", path)
	require.NoError(t, err)

	var result describeOutput
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	require.NotNil(t, result.Parameters)

	assert.Contains(t, result.Parameters.Required, "target",
		"required arg 'target' must appear in required array")
}

func TestDescribe_RequiredFlagsInRequiredArray(t *testing.T) {
	dir := t.TempDir()
	path := writeDescribeManifest(t, dir, manifestWithArgsAndFlags())

	stdout, _, err := executeDescribeCmd("scan", "-m", path)
	require.NoError(t, err)

	var result describeOutput
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	require.NotNil(t, result.Parameters)

	assert.Contains(t, result.Parameters.Required, "verbose",
		"required flag 'verbose' must appear in required array")
}

// ---------------------------------------------------------------------------
// AC-9: Optional args/flags -> NOT in "required" array
// ---------------------------------------------------------------------------

func TestDescribe_OptionalArgsNotInRequiredArray(t *testing.T) {
	dir := t.TempDir()
	path := writeDescribeManifest(t, dir, manifestWithArgsAndFlags())

	stdout, _, err := executeDescribeCmd("scan", "-m", path)
	require.NoError(t, err)

	var result describeOutput
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	require.NotNil(t, result.Parameters)

	assert.NotContains(t, result.Parameters.Required, "profile",
		"optional arg 'profile' must NOT appear in required array")
}

func TestDescribe_OptionalFlagsNotInRequiredArray(t *testing.T) {
	dir := t.TempDir()
	path := writeDescribeManifest(t, dir, manifestWithArgsAndFlags())

	stdout, _, err := executeDescribeCmd("scan", "-m", path)
	require.NoError(t, err)

	var result describeOutput
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	require.NotNil(t, result.Parameters)

	assert.NotContains(t, result.Parameters.Required, "severity",
		"optional flag 'severity' must NOT appear in required array")
}

func TestDescribe_RequiredArray_OnlyContainsRequiredItems(t *testing.T) {
	dir := t.TempDir()
	path := writeDescribeManifest(t, dir, manifestWithArgsAndFlags())

	stdout, _, err := executeDescribeCmd("scan", "-m", path)
	require.NoError(t, err)

	var result describeOutput
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	require.NotNil(t, result.Parameters)

	// Only "target" (required arg) and "verbose" (required flag) should be required.
	// The required array must contain exactly these two.
	require.Len(t, result.Parameters.Required, 2,
		"required array must contain exactly the 2 required items (target, verbose), got: %v",
		result.Parameters.Required)
	assert.Contains(t, result.Parameters.Required, "target")
	assert.Contains(t, result.Parameters.Required, "verbose")
}

// ---------------------------------------------------------------------------
// AC-9: Tool with no args/flags -> empty properties, empty/absent required
// ---------------------------------------------------------------------------

func TestDescribe_NoArgsNoFlags_EmptyProperties(t *testing.T) {
	dir := t.TempDir()
	path := writeDescribeManifest(t, dir, manifestWithNoArgsNoFlags())

	stdout, _, err := executeDescribeCmd("ping", "-m", path)
	require.NoError(t, err, "describe of tool with no args/flags must not error")

	var result describeOutput
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	require.NotNil(t, result.Parameters,
		"parameters must be present even for tool with no args/flags")
	assert.Equal(t, "object", result.Parameters.Type,
		"parameters.type must be 'object' even when no args/flags")
	assert.Empty(t, result.Parameters.Properties,
		"properties must be empty for tool with no args/flags")
}

func TestDescribe_NoArgsNoFlags_EmptyOrAbsentRequired(t *testing.T) {
	dir := t.TempDir()
	path := writeDescribeManifest(t, dir, manifestWithNoArgsNoFlags())

	stdout, _, err := executeDescribeCmd("ping", "-m", path)
	require.NoError(t, err)

	var result describeOutput
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	require.NotNil(t, result.Parameters)

	// Required can be either nil/absent or an empty array.
	assert.Empty(t, result.Parameters.Required,
		"required array must be empty or absent for tool with no args/flags, got: %v",
		result.Parameters.Required)
}

// ---------------------------------------------------------------------------
// AC-9: Only args, no flags -> properties only from args
// ---------------------------------------------------------------------------

func TestDescribe_OnlyArgs_PropertiesFromArgs(t *testing.T) {
	dir := t.TempDir()
	path := writeDescribeManifest(t, dir, manifestOnlyArgs())

	stdout, _, err := executeDescribeCmd("greet", "-m", path)
	require.NoError(t, err)

	var result describeOutput
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	require.NotNil(t, result.Parameters)

	assert.Len(t, result.Parameters.Properties, 2,
		"tool with 2 args and 0 flags must have 2 properties")
	assert.Contains(t, result.Parameters.Properties, "name",
		"arg 'name' must appear as property")
	assert.Contains(t, result.Parameters.Properties, "greeting",
		"arg 'greeting' must appear as property")
	assert.Contains(t, result.Parameters.Required, "name",
		"required arg 'name' must be in required array")
	assert.NotContains(t, result.Parameters.Required, "greeting",
		"optional arg 'greeting' must NOT be in required array")
}

// ---------------------------------------------------------------------------
// AC-9: Only flags, no args -> properties only from flags
// ---------------------------------------------------------------------------

func TestDescribe_OnlyFlags_PropertiesFromFlags(t *testing.T) {
	dir := t.TempDir()
	path := writeDescribeManifest(t, dir, manifestOnlyFlags())

	stdout, _, err := executeDescribeCmd("configure", "-m", path)
	require.NoError(t, err)

	var result describeOutput
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	require.NotNil(t, result.Parameters)

	assert.Len(t, result.Parameters.Properties, 2,
		"tool with 0 args and 2 flags must have 2 properties")
	assert.Contains(t, result.Parameters.Properties, "timeout",
		"flag 'timeout' must appear as property")
	assert.Contains(t, result.Parameters.Properties, "retries",
		"flag 'retries' must appear as property")
	assert.Contains(t, result.Parameters.Required, "retries",
		"required flag 'retries' must be in required array")
	assert.NotContains(t, result.Parameters.Required, "timeout",
		"optional flag 'timeout' must NOT be in required array")
}

// ---------------------------------------------------------------------------
// AC-9: Type mapping from arg/flag types to JSON Schema types
// ---------------------------------------------------------------------------

func TestDescribe_TypeMapping_IntegerArg(t *testing.T) {
	dir := t.TempDir()
	path := writeDescribeManifest(t, dir, manifestOnlyFlags())

	stdout, _, err := executeDescribeCmd("configure", "-m", path)
	require.NoError(t, err)

	var result describeOutput
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	require.NotNil(t, result.Parameters)

	// "timeout" and "retries" are type "integer" in the manifest
	timeoutProp, ok := result.Parameters.Properties["timeout"]
	require.True(t, ok)
	assert.Equal(t, "integer", timeoutProp.Type,
		"manifest type 'integer' must map to JSON Schema type 'integer'")
}

func TestDescribe_TypeMapping_BooleanFlag(t *testing.T) {
	dir := t.TempDir()
	path := writeDescribeManifest(t, dir, manifestWithArgsAndFlags())

	stdout, _, err := executeDescribeCmd("scan", "-m", path)
	require.NoError(t, err)

	var result describeOutput
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	require.NotNil(t, result.Parameters)

	verbProp, ok := result.Parameters.Properties["verbose"]
	require.True(t, ok)
	assert.Equal(t, "boolean", verbProp.Type,
		"manifest type 'boolean' must map to JSON Schema type 'boolean'")
}

// ---------------------------------------------------------------------------
// AC-9: --format mcp -> inputSchema key, no parameters key, no auth
// ---------------------------------------------------------------------------

func TestDescribe_FormatMCP_HasInputSchema(t *testing.T) {
	dir := t.TempDir()
	path := writeDescribeManifest(t, dir, manifestWithArgsAndFlags())

	stdout, _, err := executeDescribeCmd("scan", "-m", path, "--format", "mcp")
	require.NoError(t, err, "describe with --format mcp must not error")

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(stdout), &raw))

	_, hasInputSchema := raw["inputSchema"]
	assert.True(t, hasInputSchema,
		"MCP format must have 'inputSchema' key, got keys: %v", keysOf(raw))
}

func TestDescribe_FormatMCP_NoParametersKey(t *testing.T) {
	dir := t.TempDir()
	path := writeDescribeManifest(t, dir, manifestWithArgsAndFlags())

	stdout, _, err := executeDescribeCmd("scan", "-m", path, "--format", "mcp")
	require.NoError(t, err)

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(stdout), &raw))

	_, hasParameters := raw["parameters"]
	assert.False(t, hasParameters,
		"MCP format must NOT have 'parameters' key (use inputSchema instead)")
}

func TestDescribe_FormatMCP_NoAuth(t *testing.T) {
	dir := t.TempDir()
	path := writeDescribeManifest(t, dir, manifestWithArgsAndFlags())

	stdout, _, err := executeDescribeCmd("scan", "-m", path, "--format", "mcp")
	require.NoError(t, err)

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(stdout), &raw))

	_, hasAuth := raw["auth"]
	assert.False(t, hasAuth,
		"MCP format must NOT have 'auth' key (MCP only has name, description, inputSchema)")
}

func TestDescribe_FormatMCP_HasNameAndDescription(t *testing.T) {
	dir := t.TempDir()
	path := writeDescribeManifest(t, dir, manifestWithArgsAndFlags())

	stdout, _, err := executeDescribeCmd("scan", "-m", path, "--format", "mcp")
	require.NoError(t, err)

	var result describeOutput
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	assert.Equal(t, "scan", result.Name,
		"MCP format must include name")
	assert.Equal(t, "Scan for vulnerabilities", result.Description,
		"MCP format must include description")
}

func TestDescribe_FormatMCP_InputSchemaHasCorrectProperties(t *testing.T) {
	dir := t.TempDir()
	path := writeDescribeManifest(t, dir, manifestWithArgsAndFlags())

	stdout, _, err := executeDescribeCmd("scan", "-m", path, "--format", "mcp")
	require.NoError(t, err)

	var result describeOutput
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	require.NotNil(t, result.InputSchema,
		"MCP format must populate inputSchema")
	assert.Equal(t, "object", result.InputSchema.Type)
	assert.Contains(t, result.InputSchema.Properties, "target",
		"inputSchema must contain arg 'target' as property")
	assert.Contains(t, result.InputSchema.Properties, "severity",
		"inputSchema must contain flag 'severity' as property")
	assert.Contains(t, result.InputSchema.Required, "target",
		"inputSchema required must contain required arg 'target'")
}

func TestDescribe_FormatMCP_OnlyThreeTopLevelKeys(t *testing.T) {
	dir := t.TempDir()
	path := writeDescribeManifest(t, dir, manifestWithArgsAndFlags())

	stdout, _, err := executeDescribeCmd("scan", "-m", path, "--format", "mcp")
	require.NoError(t, err)

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(stdout), &raw))
	assert.Len(t, raw, 3,
		"MCP format must have exactly 3 top-level keys: name, description, inputSchema; got: %v",
		keysOf(raw))
}

// ---------------------------------------------------------------------------
// AC-9: --format openai -> parameters key, no inputSchema, no auth
// ---------------------------------------------------------------------------

func TestDescribe_FormatOpenAI_HasParameters(t *testing.T) {
	dir := t.TempDir()
	path := writeDescribeManifest(t, dir, manifestWithArgsAndFlags())

	stdout, _, err := executeDescribeCmd("scan", "-m", path, "--format", "openai")
	require.NoError(t, err, "describe with --format openai must not error")

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(stdout), &raw))

	_, hasParameters := raw["parameters"]
	assert.True(t, hasParameters,
		"OpenAI format must have 'parameters' key, got keys: %v", keysOf(raw))
}

func TestDescribe_FormatOpenAI_NoInputSchemaKey(t *testing.T) {
	dir := t.TempDir()
	path := writeDescribeManifest(t, dir, manifestWithArgsAndFlags())

	stdout, _, err := executeDescribeCmd("scan", "-m", path, "--format", "openai")
	require.NoError(t, err)

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(stdout), &raw))

	_, hasInputSchema := raw["inputSchema"]
	assert.False(t, hasInputSchema,
		"OpenAI format must NOT have 'inputSchema' key")
}

func TestDescribe_FormatOpenAI_NoAuth(t *testing.T) {
	dir := t.TempDir()
	path := writeDescribeManifest(t, dir, manifestWithArgsAndFlags())

	stdout, _, err := executeDescribeCmd("scan", "-m", path, "--format", "openai")
	require.NoError(t, err)

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(stdout), &raw))

	_, hasAuth := raw["auth"]
	assert.False(t, hasAuth,
		"OpenAI format must NOT have 'auth' key (only name, description, parameters)")
}

func TestDescribe_FormatOpenAI_OnlyThreeTopLevelKeys(t *testing.T) {
	dir := t.TempDir()
	path := writeDescribeManifest(t, dir, manifestWithArgsAndFlags())

	stdout, _, err := executeDescribeCmd("scan", "-m", path, "--format", "openai")
	require.NoError(t, err)

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(stdout), &raw))
	assert.Len(t, raw, 3,
		"OpenAI format must have exactly 3 top-level keys: name, description, parameters; got: %v",
		keysOf(raw))
}

func TestDescribe_FormatOpenAI_HasNameAndDescription(t *testing.T) {
	dir := t.TempDir()
	path := writeDescribeManifest(t, dir, manifestWithArgsAndFlags())

	stdout, _, err := executeDescribeCmd("scan", "-m", path, "--format", "openai")
	require.NoError(t, err)

	var result describeOutput
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	assert.Equal(t, "scan", result.Name,
		"OpenAI format must include name")
	assert.Equal(t, "Scan for vulnerabilities", result.Description,
		"OpenAI format must include description")
}

// ---------------------------------------------------------------------------
// AC-9: --format invalid -> error
// ---------------------------------------------------------------------------

func TestDescribe_FormatInvalid_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	path := writeDescribeManifest(t, dir, manifestWithArgsAndFlags())

	_, _, err := executeDescribeCmd("scan", "-m", path, "--format", "xml")
	require.Error(t, err,
		"describe with an invalid format must return an error")
}

func TestDescribe_FormatInvalid_ErrorMentionsFormat(t *testing.T) {
	dir := t.TempDir()
	path := writeDescribeManifest(t, dir, manifestWithArgsAndFlags())

	stdout, stderr, _ := executeDescribeCmd("scan", "-m", path, "--format", "yaml")
	output := stdout + stderr

	hasFormatMention := assert.Condition(t, func() bool {
		return bytes.Contains([]byte(output), []byte("yaml")) ||
			bytes.Contains([]byte(output), []byte("format")) ||
			bytes.Contains([]byte(output), []byte("unsupported"))
	}, "error output for invalid format must mention the format or that it is unsupported, got: %s", output)
	_ = hasFormatMention
}

// ---------------------------------------------------------------------------
// AC-9: Format variations — table-driven (Constitution rule 9)
// ---------------------------------------------------------------------------

func TestDescribe_FormatVariations(t *testing.T) {
	dir := t.TempDir()
	path := writeDescribeManifest(t, dir, manifestWithArgsAndFlags())

	tests := []struct {
		name            string
		format          string
		wantParameters  bool
		wantInputSchema bool
		wantAuth        bool
	}{
		{
			name:            "json format has parameters and auth",
			format:          "json",
			wantParameters:  true,
			wantInputSchema: false,
			wantAuth:        true,
		},
		{
			name:            "mcp format has inputSchema, no auth",
			format:          "mcp",
			wantParameters:  false,
			wantInputSchema: true,
			wantAuth:        false,
		},
		{
			name:            "openai format has parameters, no auth",
			format:          "openai",
			wantParameters:  true,
			wantInputSchema: false,
			wantAuth:        false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			stdout, _, err := executeDescribeCmd("scan", "-m", path, "--format", tc.format)
			require.NoError(t, err,
				"describe with format %q must not error", tc.format)

			var raw map[string]json.RawMessage
			require.NoError(t, json.Unmarshal([]byte(stdout), &raw),
				"output must be valid JSON for format %q, got: %s", tc.format, stdout)

			_, hasParams := raw["parameters"]
			_, hasInput := raw["inputSchema"]
			_, hasAuth := raw["auth"]

			assert.Equal(t, tc.wantParameters, hasParams,
				"format %q: parameters presence mismatch", tc.format)
			assert.Equal(t, tc.wantInputSchema, hasInput,
				"format %q: inputSchema presence mismatch", tc.format)
			assert.Equal(t, tc.wantAuth, hasAuth,
				"format %q: auth presence mismatch", tc.format)

			// All formats must have name and description
			_, hasName := raw["name"]
			_, hasDesc := raw["description"]
			assert.True(t, hasName,
				"format %q: must have 'name' key", tc.format)
			assert.True(t, hasDesc,
				"format %q: must have 'description' key", tc.format)
		})
	}
}

// ---------------------------------------------------------------------------
// AC-9: Auth in default format output
// ---------------------------------------------------------------------------

func TestDescribe_DefaultFormat_AuthNone(t *testing.T) {
	dir := t.TempDir()
	path := writeDescribeManifest(t, dir, manifestWithNoArgsNoFlags())

	stdout, _, err := executeDescribeCmd("ping", "-m", path)
	require.NoError(t, err)

	var result describeOutput
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	require.NotNil(t, result.Auth,
		"auth must be present in default format even for auth type 'none'")
	assert.Equal(t, "none", result.Auth.Type,
		"auth.type must be 'none' for tool with auth: none")
}

func TestDescribe_DefaultFormat_AuthInheritsFromToolkit(t *testing.T) {
	m := `apiVersion: toolwright/v1
kind: Toolkit
metadata:
  name: inherited-auth
  version: 1.0.0
  description: Toolkit auth test
auth:
  type: oauth2
  provider_url: https://example.com
  scopes:
    - read
tools:
  - name: inheriter
    description: Inherits auth
    entrypoint: ./inheriter.sh
`
	dir := t.TempDir()
	path := writeDescribeManifest(t, dir, m)

	stdout, _, err := executeDescribeCmd("inheriter", "-m", path)
	require.NoError(t, err)

	var result describeOutput
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	require.NotNil(t, result.Auth,
		"auth must be present even when inherited from toolkit level")
	assert.Equal(t, "oauth2", result.Auth.Type,
		"auth.type must be inherited from toolkit-level auth")
}

func TestDescribe_DefaultFormat_NoAuthField_DefaultsToNone(t *testing.T) {
	dir := t.TempDir()
	path := writeDescribeManifest(t, dir, manifestNoAuth())

	stdout, _, err := executeDescribeCmd("status", "-m", path)
	require.NoError(t, err)

	var result describeOutput
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	require.NotNil(t, result.Auth,
		"auth must be present in default format output")
	assert.Equal(t, "none", result.Auth.Type,
		"auth.type must default to 'none' when no auth is configured")
}

// ---------------------------------------------------------------------------
// AC-9: Anti-hardcoding — different tools produce different describe output
// ---------------------------------------------------------------------------

func TestDescribe_DifferentTools_DifferentOutput(t *testing.T) {
	dir := t.TempDir()
	path := writeDescribeManifest(t, dir, manifestWithTwoTools())

	stdout1, _, err1 := executeDescribeCmd("deploy", "-m", path)
	stdout2, _, err2 := executeDescribeCmd("rollback", "-m", path)
	require.NoError(t, err1)
	require.NoError(t, err2)

	assert.NotEqual(t, stdout1, stdout2,
		"describing different tools must produce different output")

	var result1, result2 describeOutput
	require.NoError(t, json.Unmarshal([]byte(stdout1), &result1))
	require.NoError(t, json.Unmarshal([]byte(stdout2), &result2))

	assert.Equal(t, "deploy", result1.Name,
		"first describe must be for 'deploy'")
	assert.Equal(t, "rollback", result2.Name,
		"second describe must be for 'rollback'")

	assert.NotEqual(t, result1.Description, result2.Description,
		"different tools must have different descriptions")
}

func TestDescribe_DifferentTools_DifferentAuth(t *testing.T) {
	dir := t.TempDir()
	path := writeDescribeManifest(t, dir, manifestWithTwoTools())

	stdout1, _, err1 := executeDescribeCmd("deploy", "-m", path)
	stdout2, _, err2 := executeDescribeCmd("rollback", "-m", path)
	require.NoError(t, err1)
	require.NoError(t, err2)

	var result1, result2 describeOutput
	require.NoError(t, json.Unmarshal([]byte(stdout1), &result1))
	require.NoError(t, json.Unmarshal([]byte(stdout2), &result2))

	require.NotNil(t, result1.Auth)
	require.NotNil(t, result2.Auth)
	assert.Equal(t, "oauth2", result1.Auth.Type,
		"deploy auth must be oauth2")
	assert.Equal(t, "token", result2.Auth.Type,
		"rollback auth must be token")
}

func TestDescribe_DifferentTools_DifferentParameters(t *testing.T) {
	dir := t.TempDir()
	path := writeDescribeManifest(t, dir, manifestWithTwoTools())

	stdout1, _, err1 := executeDescribeCmd("deploy", "-m", path)
	stdout2, _, err2 := executeDescribeCmd("rollback", "-m", path)
	require.NoError(t, err1)
	require.NoError(t, err2)

	var result1, result2 describeOutput
	require.NoError(t, json.Unmarshal([]byte(stdout1), &result1))
	require.NoError(t, json.Unmarshal([]byte(stdout2), &result2))

	require.NotNil(t, result1.Parameters)
	require.NotNil(t, result2.Parameters)

	// deploy has "environment" and "dry-run"
	assert.Contains(t, result1.Parameters.Properties, "environment",
		"deploy must have 'environment' property")
	assert.Contains(t, result1.Parameters.Properties, "dry-run",
		"deploy must have 'dry-run' property")

	// rollback has "revision"
	assert.Contains(t, result2.Parameters.Properties, "revision",
		"rollback must have 'revision' property")
	assert.NotContains(t, result2.Parameters.Properties, "environment",
		"rollback must NOT have deploy's 'environment' property")
}

// ---------------------------------------------------------------------------
// AC-9: Manifest file not found -> error
// ---------------------------------------------------------------------------

func TestDescribe_ManifestNotFound_ReturnsError(t *testing.T) {
	_, _, err := executeDescribeCmd("scan", "-m", "/nonexistent/path/toolwright.yaml")
	require.Error(t, err,
		"describe with a nonexistent manifest must return an error")
}

func TestDescribe_ManifestNotFound_ErrorContainsPath(t *testing.T) {
	fakePath := "/nonexistent/path/toolwright.yaml"
	stdout, stderr, _ := executeDescribeCmd("scan", "-m", fakePath)
	output := stdout + stderr

	assert.Contains(t, output, fakePath,
		"error output must include the manifest path that was not found")
}

func TestDescribe_ManifestNotFound_JSON_HasErrorStructure(t *testing.T) {
	fakePath := "/nonexistent/path/toolwright.yaml"
	stdout, _, _ := executeDescribeCmd("--json", "scan", "-m", fakePath)

	require.NotEmpty(t, stdout,
		"JSON output must be produced even for manifest-not-found errors")
	require.True(t, json.Valid([]byte(stdout)),
		"output with --json must be valid JSON even for manifest errors, got: %s", stdout)

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

// ---------------------------------------------------------------------------
// AC-9: Default manifest path (no -m flag)
// ---------------------------------------------------------------------------

func TestDescribe_DefaultManifestPath_UsesToolwrightYaml(t *testing.T) {
	dir := t.TempDir()
	writeDescribeManifest(t, dir, manifestWithNoArgsNoFlags())

	original, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { os.Chdir(original) })

	stdout, _, err := executeDescribeCmd("ping")
	require.NoError(t, err,
		"describe without -m must default to toolwright.yaml in current dir and succeed if present")

	var result describeOutput
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	assert.Equal(t, "ping", result.Name,
		"default path should find and describe tool from toolwright.yaml")
}

// ---------------------------------------------------------------------------
// AC-9: JSON output has no stderr pollution (Constitution rule 18)
// ---------------------------------------------------------------------------

func TestDescribe_NothingOnStderr(t *testing.T) {
	dir := t.TempDir()
	path := writeDescribeManifest(t, dir, manifestWithArgsAndFlags())

	_, stderr, err := executeDescribeCmd("scan", "-m", path)
	require.NoError(t, err)

	assert.Empty(t, stderr,
		"stderr must be empty for successful describe (Constitution rule 18)")
}

// ---------------------------------------------------------------------------
// AC-9: Output is valid JSON (not mixed with human text)
// ---------------------------------------------------------------------------

func TestDescribe_OutputIsOnlyJSON(t *testing.T) {
	dir := t.TempDir()
	path := writeDescribeManifest(t, dir, manifestWithArgsAndFlags())

	stdout, _, err := executeDescribeCmd("scan", "-m", path)
	require.NoError(t, err)

	require.True(t, json.Valid([]byte(stdout)),
		"describe output must contain ONLY valid JSON (no human text mixed in), got: %s", stdout)
}

// ---------------------------------------------------------------------------
// AC-9: Top-level keys for default format
// ---------------------------------------------------------------------------

func TestDescribe_DefaultFormat_TopLevelKeys(t *testing.T) {
	dir := t.TempDir()
	path := writeDescribeManifest(t, dir, manifestWithArgsAndFlags())

	stdout, _, err := executeDescribeCmd("scan", "-m", path)
	require.NoError(t, err)

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(stdout), &raw))

	assert.Len(t, raw, 4,
		"default format must have exactly 4 top-level keys: name, description, auth, parameters; got: %v",
		keysOf(raw))
	assert.Contains(t, raw, "name", "must have 'name' key")
	assert.Contains(t, raw, "description", "must have 'description' key")
	assert.Contains(t, raw, "auth", "must have 'auth' key")
	assert.Contains(t, raw, "parameters", "must have 'parameters' key")
}

// ---------------------------------------------------------------------------
// AC-9: Schema structure correctness (parameters.type is "object")
// ---------------------------------------------------------------------------

func TestDescribe_ParametersType_IsAlwaysObject(t *testing.T) {
	tests := []struct {
		name     string
		manifest string
		toolName string
	}{
		{
			name:     "tool with args and flags",
			manifest: manifestWithArgsAndFlags(),
			toolName: "scan",
		},
		{
			name:     "tool with no args/flags",
			manifest: manifestWithNoArgsNoFlags(),
			toolName: "ping",
		},
		{
			name:     "tool with only args",
			manifest: manifestOnlyArgs(),
			toolName: "greet",
		},
		{
			name:     "tool with only flags",
			manifest: manifestOnlyFlags(),
			toolName: "configure",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := writeDescribeManifest(t, dir, tc.manifest)

			stdout, _, err := executeDescribeCmd(tc.toolName, "-m", path)
			require.NoError(t, err)

			var result describeOutput
			require.NoError(t, json.Unmarshal([]byte(stdout), &result))
			require.NotNil(t, result.Parameters)
			assert.Equal(t, "object", result.Parameters.Type,
				"parameters.type must always be 'object'")
		})
	}
}

// ---------------------------------------------------------------------------
// AC-9: MCP inputSchema.type is also "object"
// ---------------------------------------------------------------------------

func TestDescribe_FormatMCP_InputSchemaType_IsObject(t *testing.T) {
	dir := t.TempDir()
	path := writeDescribeManifest(t, dir, manifestWithArgsAndFlags())

	stdout, _, err := executeDescribeCmd("scan", "-m", path, "--format", "mcp")
	require.NoError(t, err)

	var result describeOutput
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	require.NotNil(t, result.InputSchema)
	assert.Equal(t, "object", result.InputSchema.Type,
		"MCP inputSchema.type must be 'object'")
}

// ---------------------------------------------------------------------------
// AC-9: Properties count matches total args + flags
// ---------------------------------------------------------------------------

func TestDescribe_PropertiesCount_MatchesArgsAndFlags(t *testing.T) {
	dir := t.TempDir()
	path := writeDescribeManifest(t, dir, manifestWithArgsAndFlags())

	stdout, _, err := executeDescribeCmd("scan", "-m", path)
	require.NoError(t, err)

	var result describeOutput
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	require.NotNil(t, result.Parameters)

	// scan has 2 args (target, profile) + 2 flags (severity, verbose) = 4 properties
	assert.Len(t, result.Parameters.Properties, 4,
		"properties count must equal total args + flags (2+2=4), got: %v",
		result.Parameters.Properties)
}

// ---------------------------------------------------------------------------
// AC-9: -f shorthand works same as --format
// ---------------------------------------------------------------------------

func TestDescribe_ShortFormatFlag_MCP(t *testing.T) {
	dir := t.TempDir()
	path := writeDescribeManifest(t, dir, manifestWithArgsAndFlags())

	stdout, _, err := executeDescribeCmd("scan", "-m", path, "-f", "mcp")
	require.NoError(t, err,
		"-f shorthand must work same as --format")

	var raw map[string]json.RawMessage
	require.NoError(t, json.Unmarshal([]byte(stdout), &raw))

	_, hasInputSchema := raw["inputSchema"]
	assert.True(t, hasInputSchema,
		"-f mcp must produce inputSchema key")
}

// ---------------------------------------------------------------------------
// AC-9: Property descriptions are tool-specific, not generic
// ---------------------------------------------------------------------------

func TestDescribe_PropertyDescriptions_AreSpecific(t *testing.T) {
	dir := t.TempDir()
	path := writeDescribeManifest(t, dir, manifestWithArgsAndFlags())

	stdout, _, err := executeDescribeCmd("scan", "-m", path)
	require.NoError(t, err)

	var result describeOutput
	require.NoError(t, json.Unmarshal([]byte(stdout), &result))
	require.NotNil(t, result.Parameters)

	// Each property description must match the manifest, not be generic
	targetProp := result.Parameters.Properties["target"]
	assert.Equal(t, "Scan target", targetProp.Description,
		"property description must be tool-specific, matching the manifest arg description")

	sevProp := result.Parameters.Properties["severity"]
	assert.Equal(t, "Min severity", sevProp.Description,
		"property description must be tool-specific, matching the manifest flag description")
}

// ---------------------------------------------------------------------------
// AC-9: Schema consistency across formats (same properties in all formats)
// ---------------------------------------------------------------------------

func TestDescribe_SchemaConsistency_AcrossFormats(t *testing.T) {
	dir := t.TempDir()
	path := writeDescribeManifest(t, dir, manifestWithArgsAndFlags())

	stdoutJSON, _, err1 := executeDescribeCmd("scan", "-m", path, "--format", "json")
	stdoutMCP, _, err2 := executeDescribeCmd("scan", "-m", path, "--format", "mcp")
	stdoutOpenAI, _, err3 := executeDescribeCmd("scan", "-m", path, "--format", "openai")
	require.NoError(t, err1)
	require.NoError(t, err2)
	require.NoError(t, err3)

	var resultJSON, resultMCP, resultOpenAI describeOutput
	require.NoError(t, json.Unmarshal([]byte(stdoutJSON), &resultJSON))
	require.NoError(t, json.Unmarshal([]byte(stdoutMCP), &resultMCP))
	require.NoError(t, json.Unmarshal([]byte(stdoutOpenAI), &resultOpenAI))

	// Get the schema from each format
	jsonSchema := resultJSON.Parameters
	mcpSchema := resultMCP.InputSchema
	openaiSchema := resultOpenAI.Parameters

	require.NotNil(t, jsonSchema, "json format must have parameters")
	require.NotNil(t, mcpSchema, "mcp format must have inputSchema")
	require.NotNil(t, openaiSchema, "openai format must have parameters")

	// All three must have the same properties count
	assert.Equal(t, len(jsonSchema.Properties), len(mcpSchema.Properties),
		"json and mcp formats must have same number of properties")
	assert.Equal(t, len(jsonSchema.Properties), len(openaiSchema.Properties),
		"json and openai formats must have same number of properties")

	// All three must have the same required array contents
	assert.ElementsMatch(t, jsonSchema.Required, mcpSchema.Required,
		"json and mcp formats must have same required array")
	assert.ElementsMatch(t, jsonSchema.Required, openaiSchema.Required,
		"json and openai formats must have same required array")

	// All three must have the same property names
	for propName := range jsonSchema.Properties {
		assert.Contains(t, mcpSchema.Properties, propName,
			"mcp format must have same property %q as json format", propName)
		assert.Contains(t, openaiSchema.Properties, propName,
			"openai format must have same property %q as json format", propName)
	}
}
