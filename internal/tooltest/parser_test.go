package tooltest

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// YAML fixtures
// ---------------------------------------------------------------------------

// fullTestYAML exercises every field in the test YAML format. A parser that
// skips any field, drops an assertion operator, or mangles the timeout will
// fail against this fixture.
const fullTestYAML = `tool: scan
tests:
  - name: finds sql injection
    args: [./fixtures/vulnerable.py]
    flags:
      severity: low
    auth_token: "${DEPLOY_TEST_TOKEN}"
    expect:
      exit_code: 0
      stdout_is_json: true
      stdout_schema: schemas/scan-output.json
      stdout_contains:
        - path: $.findings[0].type
          equals: sql_injection
        - path: $.findings
          length: 3
      stderr_contains:
        - "processing complete"
    timeout: 10s
`

// minimalTestYAML has only the required fields: tool name and a single test
// with just a name. All other fields must fall to their zero/default values.
const minimalTestYAML = `tool: simple
tests:
  - name: baseline
`

// multiTestYAML has three tests in one file. Order must be preserved.
const multiTestYAML = `tool: multi
tests:
  - name: first
    args: [a]
    timeout: 1s
  - name: second
    args: [b, c]
    timeout: 2s
  - name: third
    args: [d, e, f]
    timeout: 3s
`

// allOperatorsYAML exercises every assertion operator on separate assertions
// so a parser that only handles one or two will fail.
const allOperatorsYAML = `tool: ops
tests:
  - name: all operators
    expect:
      stdout_contains:
        - path: $.name
          equals: fido
        - path: $.tags
          contains: friendly
        - path: $.id
          matches: "^\\d+$"
        - path: $.active
          exists: true
        - path: $.items
          length: 5
`

// boolFlagYAML verifies that flag values like "true" stay as strings in the
// map[string]string, not get converted to bool or dropped.
const boolFlagYAML = `tool: flagtest
tests:
  - name: bool flags
    flags:
      verbose: "true"
      dry_run: "false"
      count: "42"
`

// numericEqualsYAML verifies numeric assertion values are preserved as numbers,
// not silently converted to strings.
const numericEqualsYAML = `tool: numtest
tests:
  - name: numeric equals
    expect:
      stdout_contains:
        - path: $.count
          equals: 42
        - path: $.ratio
          equals: 3.14
`

// literalAuthTokenYAML has an auth_token WITHOUT ${} wrapping.
// It must be treated as a literal string, not expanded.
const literalAuthTokenYAML = `tool: literalauth
tests:
  - name: literal token
    auth_token: plain-token-value
`

// multipleTimeoutsYAML tests various duration formats.
const multipleTimeoutsYAML = `tool: timeouts
tests:
  - name: ten seconds
    timeout: 10s
  - name: five hundred ms
    timeout: 500ms
  - name: one minute thirty seconds
    timeout: 1m30s
`

// writeTestFile is a helper that writes content to a .test.yaml file inside
// the given directory and returns the full path.
func writeTestFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	err := os.WriteFile(path, []byte(content), 0644)
	require.NoError(t, err, "failed to write test file %s", name)
	return path
}

// ---------------------------------------------------------------------------
// AC-1: Test YAML parses into TestSuite
// ---------------------------------------------------------------------------

func TestParseTestFile_FullYAML_AllFields(t *testing.T) {
	dir := t.TempDir()
	path := writeTestFile(t, dir, "scan.test.yaml", fullTestYAML)

	// Set the env var referenced in the fixture so it resolves.
	t.Setenv("DEPLOY_TEST_TOKEN", "secret-deploy-token")

	suite, err := ParseTestFile(path)
	require.NoError(t, err, "ParseTestFile must not error on valid YAML")
	require.NotNil(t, suite, "ParseTestFile must return a non-nil TestSuite")

	// Tool name
	assert.Equal(t, "scan", suite.Tool, "Tool name must be 'scan'")

	// Exactly one test case
	require.Len(t, suite.Tests, 1, "Suite must have exactly 1 test case")
	tc := suite.Tests[0]

	// Test case fields
	assert.Equal(t, "finds sql injection", tc.Name)
	assert.Equal(t, []string{"./fixtures/vulnerable.py"}, tc.Args)
	require.NotNil(t, tc.Flags, "Flags must not be nil")
	assert.Equal(t, map[string]string{"severity": "low"}, tc.Flags)
	assert.Equal(t, "secret-deploy-token", tc.AuthToken,
		"auth_token with ${} must be expanded from env")

	// Expectation
	ex := tc.Expect
	require.NotNil(t, ex.ExitCode, "ExitCode pointer must be non-nil")
	assert.Equal(t, 0, *ex.ExitCode)
	require.NotNil(t, ex.StdoutIsJSON, "StdoutIsJSON pointer must be non-nil")
	assert.True(t, *ex.StdoutIsJSON)
	assert.Equal(t, "schemas/scan-output.json", ex.StdoutSchema)

	// StdoutContains assertions
	require.Len(t, ex.StdoutContains, 2, "Must have 2 stdout_contains assertions")

	eqAssertion := ex.StdoutContains[0]
	assert.Equal(t, "$.findings[0].type", eqAssertion.Path)
	assert.Equal(t, "sql_injection", eqAssertion.Equals)

	lenAssertion := ex.StdoutContains[1]
	assert.Equal(t, "$.findings", lenAssertion.Path)
	require.NotNil(t, lenAssertion.Length)
	assert.Equal(t, 3, *lenAssertion.Length)

	// StderrContains
	assert.Equal(t, []string{"processing complete"}, ex.StderrContains)

	// Timeout
	assert.Equal(t, 10*time.Second, tc.Timeout,
		"Timeout '10s' must parse to 10 seconds")
}

func TestParseTestFile_MinimalYAML_Defaults(t *testing.T) {
	dir := t.TempDir()
	path := writeTestFile(t, dir, "minimal.test.yaml", minimalTestYAML)

	suite, err := ParseTestFile(path)
	require.NoError(t, err)
	require.NotNil(t, suite)

	assert.Equal(t, "simple", suite.Tool)
	require.Len(t, suite.Tests, 1)
	tc := suite.Tests[0]

	assert.Equal(t, "baseline", tc.Name)
	assert.Empty(t, tc.Args, "Args should be empty/nil when not specified")
	assert.Empty(t, tc.Flags, "Flags should be empty/nil when not specified")
	assert.Empty(t, tc.AuthToken, "AuthToken should be empty when not specified")
	assert.Equal(t, time.Duration(0), tc.Timeout,
		"Timeout should be zero when not specified")

	// Expectation should be zero-valued
	assert.Nil(t, tc.Expect.ExitCode,
		"ExitCode must be nil when not specified (not *int(0))")
	assert.Nil(t, tc.Expect.StdoutIsJSON,
		"StdoutIsJSON must be nil when not specified")
	assert.Empty(t, tc.Expect.StdoutSchema)
	assert.Empty(t, tc.Expect.StdoutContains)
	assert.Empty(t, tc.Expect.StderrContains)
}

func TestParseTestFile_MultipleTests_OrderPreserved(t *testing.T) {
	dir := t.TempDir()
	path := writeTestFile(t, dir, "multi.test.yaml", multiTestYAML)

	suite, err := ParseTestFile(path)
	require.NoError(t, err)
	require.NotNil(t, suite)

	assert.Equal(t, "multi", suite.Tool)
	require.Len(t, suite.Tests, 3, "Must have exactly 3 test cases")

	// Verify order and distinct values (anti-hardcoding)
	assert.Equal(t, "first", suite.Tests[0].Name)
	assert.Equal(t, []string{"a"}, suite.Tests[0].Args)
	assert.Equal(t, 1*time.Second, suite.Tests[0].Timeout)

	assert.Equal(t, "second", suite.Tests[1].Name)
	assert.Equal(t, []string{"b", "c"}, suite.Tests[1].Args)
	assert.Equal(t, 2*time.Second, suite.Tests[1].Timeout)

	assert.Equal(t, "third", suite.Tests[2].Name)
	assert.Equal(t, []string{"d", "e", "f"}, suite.Tests[2].Args)
	assert.Equal(t, 3*time.Second, suite.Tests[2].Timeout)
}

// ---------------------------------------------------------------------------
// AC-1: All assertion operators parsed
// ---------------------------------------------------------------------------

func TestParseTestFile_AllAssertionOperators(t *testing.T) {
	dir := t.TempDir()
	path := writeTestFile(t, dir, "ops.test.yaml", allOperatorsYAML)

	suite, err := ParseTestFile(path)
	require.NoError(t, err)
	require.NotNil(t, suite)
	require.Len(t, suite.Tests, 1)

	assertions := suite.Tests[0].Expect.StdoutContains
	require.Len(t, assertions, 5, "Must have exactly 5 assertions, one per operator")

	// equals
	assert.Equal(t, "$.name", assertions[0].Path)
	assert.Equal(t, "fido", assertions[0].Equals)
	assert.Nil(t, assertions[0].Contains, "equals assertion must not set contains")
	assert.Empty(t, assertions[0].Matches, "equals assertion must not set matches")
	assert.Nil(t, assertions[0].Exists, "equals assertion must not set exists")
	assert.Nil(t, assertions[0].Length, "equals assertion must not set length")

	// contains
	assert.Equal(t, "$.tags", assertions[1].Path)
	assert.Equal(t, "friendly", assertions[1].Contains)
	assert.Nil(t, assertions[1].Equals, "contains assertion must not set equals")

	// matches
	assert.Equal(t, "$.id", assertions[2].Path)
	assert.Equal(t, `^\d+$`, assertions[2].Matches)
	assert.Nil(t, assertions[2].Equals, "matches assertion must not set equals")
	assert.Nil(t, assertions[2].Contains, "matches assertion must not set contains")

	// exists
	assert.Equal(t, "$.active", assertions[3].Path)
	require.NotNil(t, assertions[3].Exists, "exists must be a non-nil pointer")
	assert.True(t, *assertions[3].Exists)
	assert.Nil(t, assertions[3].Equals, "exists assertion must not set equals")

	// length
	assert.Equal(t, "$.items", assertions[4].Path)
	require.NotNil(t, assertions[4].Length, "length must be a non-nil pointer")
	assert.Equal(t, 5, *assertions[4].Length)
	assert.Nil(t, assertions[4].Equals, "length assertion must not set equals")
}

// ---------------------------------------------------------------------------
// AC-1: Timeout parsing with various duration formats
// ---------------------------------------------------------------------------

func TestParseTestFile_TimeoutFormats(t *testing.T) {
	dir := t.TempDir()
	path := writeTestFile(t, dir, "timeouts.test.yaml", multipleTimeoutsYAML)

	suite, err := ParseTestFile(path)
	require.NoError(t, err)
	require.NotNil(t, suite)
	require.Len(t, suite.Tests, 3)

	assert.Equal(t, 10*time.Second, suite.Tests[0].Timeout,
		"'10s' must parse to 10 seconds")
	assert.Equal(t, 500*time.Millisecond, suite.Tests[1].Timeout,
		"'500ms' must parse to 500 milliseconds")
	assert.Equal(t, 90*time.Second, suite.Tests[2].Timeout,
		"'1m30s' must parse to 90 seconds")

	// Anti-hardcoding: all three must be distinct
	assert.NotEqual(t, suite.Tests[0].Timeout, suite.Tests[1].Timeout)
	assert.NotEqual(t, suite.Tests[1].Timeout, suite.Tests[2].Timeout)
	assert.NotEqual(t, suite.Tests[0].Timeout, suite.Tests[2].Timeout)
}

// ---------------------------------------------------------------------------
// AC-2: Auth token env var expansion
// ---------------------------------------------------------------------------

func TestParseTestFile_AuthToken_EnvVarExpansion(t *testing.T) {
	t.Setenv("MY_TOKEN", "secret-value-123")

	yaml := `tool: authtest
tests:
  - name: with token
    auth_token: "${MY_TOKEN}"
`
	dir := t.TempDir()
	path := writeTestFile(t, dir, "auth.test.yaml", yaml)

	suite, err := ParseTestFile(path)
	require.NoError(t, err)
	require.NotNil(t, suite)
	require.Len(t, suite.Tests, 1)

	assert.Equal(t, "secret-value-123", suite.Tests[0].AuthToken,
		"auth_token '${MY_TOKEN}' must resolve to the env var value")
}

func TestParseTestFile_AuthToken_UnsetEnvVar_EmptyString(t *testing.T) {
	// Ensure UNSET_VAR is truly unset (t.Setenv will restore after test)
	t.Setenv("UNSET_VAR", "")
	os.Unsetenv("UNSET_VAR")

	yaml := `tool: authtest
tests:
  - name: unset token
    auth_token: "${UNSET_VAR}"
`
	dir := t.TempDir()
	path := writeTestFile(t, dir, "auth-unset.test.yaml", yaml)

	suite, err := ParseTestFile(path)
	require.NoError(t, err)
	require.NotNil(t, suite)
	require.Len(t, suite.Tests, 1)

	assert.Equal(t, "", suite.Tests[0].AuthToken,
		"auth_token referencing unset env var must resolve to empty string, not literal '${UNSET_VAR}'")
}

func TestParseTestFile_AuthToken_FallbackToToolwrightTestToken(t *testing.T) {
	t.Setenv("TOOLWRIGHT_TEST_TOKEN", "fallback-token")

	// No auth_token field in the test case at all.
	yaml := `tool: authtest
tests:
  - name: no explicit token
`
	dir := t.TempDir()
	path := writeTestFile(t, dir, "auth-fallback.test.yaml", yaml)

	suite, err := ParseTestFile(path)
	require.NoError(t, err)
	require.NotNil(t, suite)
	require.Len(t, suite.Tests, 1)

	assert.Equal(t, "fallback-token", suite.Tests[0].AuthToken,
		"When auth_token is not specified, TOOLWRIGHT_TEST_TOKEN env var must be used as fallback")
}

func TestParseTestFile_AuthToken_NoFallbackWhenUnset(t *testing.T) {
	// Make sure TOOLWRIGHT_TEST_TOKEN is unset.
	t.Setenv("TOOLWRIGHT_TEST_TOKEN", "")
	os.Unsetenv("TOOLWRIGHT_TEST_TOKEN")

	yaml := `tool: authtest
tests:
  - name: no token anywhere
`
	dir := t.TempDir()
	path := writeTestFile(t, dir, "auth-none.test.yaml", yaml)

	suite, err := ParseTestFile(path)
	require.NoError(t, err)
	require.NotNil(t, suite)
	require.Len(t, suite.Tests, 1)

	assert.Equal(t, "", suite.Tests[0].AuthToken,
		"When no auth_token and no TOOLWRIGHT_TEST_TOKEN, AuthToken must be empty string")
}

func TestParseTestFile_AuthToken_LiteralString_NoExpansion(t *testing.T) {
	dir := t.TempDir()
	path := writeTestFile(t, dir, "auth-literal.test.yaml", literalAuthTokenYAML)

	suite, err := ParseTestFile(path)
	require.NoError(t, err)
	require.NotNil(t, suite)
	require.Len(t, suite.Tests, 1)

	assert.Equal(t, "plain-token-value", suite.Tests[0].AuthToken,
		"auth_token without ${} must be treated as a literal string")
}

func TestParseTestFile_AuthToken_ExplicitOverridesFallback(t *testing.T) {
	// When both explicit auth_token and TOOLWRIGHT_TEST_TOKEN are present,
	// the explicit one wins.
	t.Setenv("TOOLWRIGHT_TEST_TOKEN", "fallback-token")
	t.Setenv("EXPLICIT_TOKEN", "explicit-value")

	yaml := `tool: authtest
tests:
  - name: explicit wins
    auth_token: "${EXPLICIT_TOKEN}"
`
	dir := t.TempDir()
	path := writeTestFile(t, dir, "auth-priority.test.yaml", yaml)

	suite, err := ParseTestFile(path)
	require.NoError(t, err)
	require.NotNil(t, suite)
	require.Len(t, suite.Tests, 1)

	assert.Equal(t, "explicit-value", suite.Tests[0].AuthToken,
		"Explicit auth_token must take priority over TOOLWRIGHT_TEST_TOKEN fallback")
}

// ---------------------------------------------------------------------------
// AC-3: Test directory globbing
// ---------------------------------------------------------------------------

func TestParseTestDir_MultipleTestFiles(t *testing.T) {
	dir := t.TempDir()

	fooYAML := `tool: foo
tests:
  - name: foo test
    args: [x]
`
	barYAML := `tool: bar
tests:
  - name: bar test
    args: [y]
`
	writeTestFile(t, dir, "foo.test.yaml", fooYAML)
	writeTestFile(t, dir, "bar.test.yaml", barYAML)

	suites, err := ParseTestDir(dir)
	require.NoError(t, err, "ParseTestDir must not error on valid directory")
	require.Len(t, suites, 2, "Must find exactly 2 test suites")

	// Collect tool names — both must be present (order may vary by filesystem).
	tools := map[string]bool{}
	for _, s := range suites {
		tools[s.Tool] = true
	}
	assert.True(t, tools["foo"], "Must find foo.test.yaml")
	assert.True(t, tools["bar"], "Must find bar.test.yaml")

	// Verify actual content was parsed, not just filenames found.
	for _, s := range suites {
		require.Len(t, s.Tests, 1, "Each suite must have 1 test")
		if s.Tool == "foo" {
			assert.Equal(t, "foo test", s.Tests[0].Name)
			assert.Equal(t, []string{"x"}, s.Tests[0].Args)
		} else {
			assert.Equal(t, "bar test", s.Tests[0].Name)
			assert.Equal(t, []string{"y"}, s.Tests[0].Args)
		}
	}
}

func TestParseTestDir_NonTestFilesIgnored(t *testing.T) {
	dir := t.TempDir()

	// Create non-test files that must NOT be parsed.
	writeTestFile(t, dir, "README.md", "# Test suite\n")
	writeTestFile(t, dir, "notes.txt", "Some notes\n")
	writeTestFile(t, dir, "config.yaml", "key: value\n")
	writeTestFile(t, dir, "data.test.json", `{"test": true}`)

	// Create one valid test file.
	writeTestFile(t, dir, "valid.test.yaml", `tool: valid
tests:
  - name: only one
`)

	suites, err := ParseTestDir(dir)
	require.NoError(t, err)
	require.Len(t, suites, 1, "Only .test.yaml files must be parsed")
	assert.Equal(t, "valid", suites[0].Tool)
}

func TestParseTestDir_EmptyDirectory(t *testing.T) {
	dir := t.TempDir()

	suites, err := ParseTestDir(dir)
	require.NoError(t, err, "Empty directory must not produce an error")
	require.NotNil(t, suites, "Result must be non-nil (empty slice, not nil)")
	assert.Len(t, suites, 0, "Empty directory must produce empty suite list")
}

func TestParseTestDir_NonexistentDirectory(t *testing.T) {
	_, err := ParseTestDir("/nonexistent/directory/that/does/not/exist")
	require.Error(t, err, "Nonexistent directory must produce an error")
}

// ---------------------------------------------------------------------------
// Error paths
// ---------------------------------------------------------------------------

func TestParseTestFile_NonexistentFile(t *testing.T) {
	_, err := ParseTestFile("/nonexistent/file/that/does/not/exist.test.yaml")
	require.Error(t, err, "Nonexistent file must produce an error")
}

func TestParseTestFile_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	path := writeTestFile(t, dir, "bad.test.yaml", "{{{\nnot: valid: yaml: [}")

	result, err := ParseTestFile(path)
	require.Error(t, err, "Invalid YAML must produce an error")
	assert.Nil(t, result, "Invalid YAML must return nil suite")
}

func TestParseTestFile_MissingToolField(t *testing.T) {
	dir := t.TempDir()
	yaml := `tests:
  - name: orphan test
    args: [x]
`
	path := writeTestFile(t, dir, "notool.test.yaml", yaml)

	result, err := ParseTestFile(path)
	require.Error(t, err, "Missing 'tool' field must produce an error")
	assert.Nil(t, result, "Missing 'tool' must return nil suite")
}

func TestParseTestFile_EmptyToolField(t *testing.T) {
	dir := t.TempDir()
	yaml := `tool: ""
tests:
  - name: empty tool test
`
	path := writeTestFile(t, dir, "emptytool.test.yaml", yaml)

	result, err := ParseTestFile(path)
	require.Error(t, err, "Empty 'tool' field must produce an error")
	assert.Nil(t, result, "Empty 'tool' must return nil suite")
}

// ---------------------------------------------------------------------------
// Edge cases: flag values, numeric assertions
// ---------------------------------------------------------------------------

func TestParseTestFile_BoolFlagValues_PreservedAsStrings(t *testing.T) {
	dir := t.TempDir()
	path := writeTestFile(t, dir, "flags.test.yaml", boolFlagYAML)

	suite, err := ParseTestFile(path)
	require.NoError(t, err)
	require.NotNil(t, suite)
	require.Len(t, suite.Tests, 1)

	flags := suite.Tests[0].Flags
	require.Len(t, flags, 3, "Must have 3 flag entries")

	assert.Equal(t, "true", flags["verbose"],
		"Flag value 'true' must remain the string \"true\", not be converted to bool")
	assert.Equal(t, "false", flags["dry_run"],
		"Flag value 'false' must remain the string \"false\"")
	assert.Equal(t, "42", flags["count"],
		"Flag value '42' must remain the string \"42\"")
}

func TestParseTestFile_NumericAssertionValues_Preserved(t *testing.T) {
	dir := t.TempDir()
	path := writeTestFile(t, dir, "numeric.test.yaml", numericEqualsYAML)

	suite, err := ParseTestFile(path)
	require.NoError(t, err)
	require.NotNil(t, suite)
	require.Len(t, suite.Tests, 1)

	assertions := suite.Tests[0].Expect.StdoutContains
	require.Len(t, assertions, 2)

	// The integer 42 must remain numeric, not become the string "42".
	intVal := assertions[0].Equals
	assert.Equal(t, "$.count", assertions[0].Path)
	// YAML parses 42 as int. The Equals field is `any`, so it should hold
	// a numeric type (int or float64 depending on YAML library), not a string.
	switch v := intVal.(type) {
	case int:
		assert.Equal(t, 42, v)
	case int64:
		assert.Equal(t, int64(42), v)
	case float64:
		assert.Equal(t, float64(42), v)
	default:
		t.Errorf("Expected numeric type for equals: 42, got %T (%v)", intVal, intVal)
	}

	// The float 3.14 must remain numeric.
	floatVal := assertions[1].Equals
	assert.Equal(t, "$.ratio", assertions[1].Path)
	switch v := floatVal.(type) {
	case float64:
		assert.InDelta(t, 3.14, v, 0.001)
	case float32:
		assert.InDelta(t, 3.14, float64(v), 0.001)
	default:
		t.Errorf("Expected float type for equals: 3.14, got %T (%v)", floatVal, floatVal)
	}
}

// ---------------------------------------------------------------------------
// Deep structural comparison: parsed full YAML against expected struct
// ---------------------------------------------------------------------------

func TestParseTestFile_FullYAML_StructuralEquality(t *testing.T) {
	t.Setenv("DEPLOY_TEST_TOKEN", "test-token")

	dir := t.TempDir()
	path := writeTestFile(t, dir, "scan.test.yaml", fullTestYAML)

	suite, err := ParseTestFile(path)
	require.NoError(t, err)
	require.NotNil(t, suite)

	want := &TestSuite{
		Tool: "scan",
		Tests: []TestCase{
			{
				Name:      "finds sql injection",
				Args:      []string{"./fixtures/vulnerable.py"},
				Flags:     map[string]string{"severity": "low"},
				AuthToken: "test-token",
				Expect: Expectation{
					ExitCode:     intPtr(0),
					StdoutIsJSON: boolPtr(true),
					StdoutSchema: "schemas/scan-output.json",
					StdoutContains: []Assertion{
						{Path: "$.findings[0].type", Equals: "sql_injection"},
						{Path: "$.findings", Length: intPtr(3)},
					},
					StderrContains: []string{"processing complete"},
				},
				Timeout: 10 * time.Second,
			},
		},
	}

	if diff := cmp.Diff(want, suite); diff != "" {
		t.Errorf("ParseTestFile(fullTestYAML) mismatch (-want +got):\n%s", diff)
	}
}

// ---------------------------------------------------------------------------
// ParseTestDir: verify actual content, not just count
// ---------------------------------------------------------------------------

func TestParseTestDir_ParsesContent_NotJustFilenames(t *testing.T) {
	dir := t.TempDir()

	writeTestFile(t, dir, "alpha.test.yaml", `tool: alpha
tests:
  - name: alpha test
    args: [--alpha]
    timeout: 5s
`)
	writeTestFile(t, dir, "beta.test.yaml", `tool: beta
tests:
  - name: beta test one
    args: [--beta]
  - name: beta test two
    args: [--beta, --extra]
`)

	suites, err := ParseTestDir(dir)
	require.NoError(t, err)
	require.Len(t, suites, 2)

	// Build a map for order-independent access.
	byTool := map[string]*TestSuite{}
	for i := range suites {
		byTool[suites[i].Tool] = &suites[i]
	}

	// Verify alpha content
	alpha, ok := byTool["alpha"]
	require.True(t, ok, "Must find alpha suite")
	require.Len(t, alpha.Tests, 1)
	assert.Equal(t, "alpha test", alpha.Tests[0].Name)
	assert.Equal(t, []string{"--alpha"}, alpha.Tests[0].Args)
	assert.Equal(t, 5*time.Second, alpha.Tests[0].Timeout)

	// Verify beta content — two tests
	beta, ok := byTool["beta"]
	require.True(t, ok, "Must find beta suite")
	require.Len(t, beta.Tests, 2)
	assert.Equal(t, "beta test one", beta.Tests[0].Name)
	assert.Equal(t, []string{"--beta"}, beta.Tests[0].Args)
	assert.Equal(t, "beta test two", beta.Tests[1].Name)
	assert.Equal(t, []string{"--beta", "--extra"}, beta.Tests[1].Args)
}

// ---------------------------------------------------------------------------
// ParseTestDir: only .test.yaml suffix, not .yaml or .test.yml
// ---------------------------------------------------------------------------

func TestParseTestDir_StrictGlobPattern(t *testing.T) {
	dir := t.TempDir()

	// These should NOT match the glob *.test.yaml
	writeTestFile(t, dir, "regular.yaml", `tool: wrong
tests:
  - name: should not match
`)
	writeTestFile(t, dir, "also.test.yml", `tool: wrong2
tests:
  - name: yml extension
`)
	writeTestFile(t, dir, "test.yaml.bak", `tool: wrong3
tests:
  - name: backup file
`)

	// Only this should match
	writeTestFile(t, dir, "real.test.yaml", `tool: correct
tests:
  - name: correct match
`)

	suites, err := ParseTestDir(dir)
	require.NoError(t, err)
	require.Len(t, suites, 1, "Only *.test.yaml files must be matched")
	assert.Equal(t, "correct", suites[0].Tool)
}

// ---------------------------------------------------------------------------
// ParseTestFile: does not panic on adversarial inputs
// ---------------------------------------------------------------------------

func TestParseTestFile_DoesNotPanic(t *testing.T) {
	adversarial := []string{
		"",
		"null",
		"~",
		"42",
		"[]",
		`"just a string"`,
		"tool: x\ntests: not-a-list\n",
		"\x00\x01\x02\xff\xfe",
	}

	dir := t.TempDir()
	for i, input := range adversarial {
		t.Run("", func(t *testing.T) {
			path := writeTestFile(t, dir, filepath.Base(
				filepath.Join(dir, "adversarial"+string(rune('0'+i))+".test.yaml")),
				input,
			)
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("ParseTestFile panicked on adversarial input %d: %v", i, r)
					}
				}()
				_, _ = ParseTestFile(path)
			}()
		})
	}
}

// ---------------------------------------------------------------------------
// Existence assertion: exists: false must be distinguishable from nil
// ---------------------------------------------------------------------------

func TestParseTestFile_ExistsFalse_DistinctFromNil(t *testing.T) {
	yaml := `tool: existstest
tests:
  - name: exists false
    expect:
      stdout_contains:
        - path: $.missing
          exists: false
`
	dir := t.TempDir()
	path := writeTestFile(t, dir, "exists.test.yaml", yaml)

	suite, err := ParseTestFile(path)
	require.NoError(t, err)
	require.NotNil(t, suite)
	require.Len(t, suite.Tests, 1)
	require.Len(t, suite.Tests[0].Expect.StdoutContains, 1)

	a := suite.Tests[0].Expect.StdoutContains[0]
	assert.Equal(t, "$.missing", a.Path)
	require.NotNil(t, a.Exists, "exists: false must produce non-nil *bool, not nil")
	assert.False(t, *a.Exists, "exists: false must dereference to false")
}

// ---------------------------------------------------------------------------
// Length=0 must be distinguishable from nil
// ---------------------------------------------------------------------------

func TestParseTestFile_LengthZero_DistinctFromNil(t *testing.T) {
	yaml := `tool: lentest
tests:
  - name: length zero
    expect:
      stdout_contains:
        - path: $.items
          length: 0
`
	dir := t.TempDir()
	path := writeTestFile(t, dir, "length.test.yaml", yaml)

	suite, err := ParseTestFile(path)
	require.NoError(t, err)
	require.NotNil(t, suite)
	require.Len(t, suite.Tests, 1)
	require.Len(t, suite.Tests[0].Expect.StdoutContains, 1)

	a := suite.Tests[0].Expect.StdoutContains[0]
	assert.Equal(t, "$.items", a.Path)
	require.NotNil(t, a.Length, "length: 0 must produce non-nil *int, not nil")
	assert.Equal(t, 0, *a.Length, "length: 0 must dereference to 0")
}

// ---------------------------------------------------------------------------
// ExitCode=0 must be distinguishable from omitted
// ---------------------------------------------------------------------------

func TestParseTestFile_ExitCodeZero_DistinctFromOmitted(t *testing.T) {
	yamlWithExit := `tool: exittest
tests:
  - name: with exit code
    expect:
      exit_code: 0
`
	yamlWithoutExit := `tool: exittest
tests:
  - name: without exit code
    expect:
      stdout_schema: schema.json
`
	dir := t.TempDir()
	pathWith := writeTestFile(t, dir, "with.test.yaml", yamlWithExit)
	pathWithout := writeTestFile(t, dir, "without.test.yaml", yamlWithoutExit)

	suiteWith, err := ParseTestFile(pathWith)
	require.NoError(t, err)
	require.NotNil(t, suiteWith)
	require.Len(t, suiteWith.Tests, 1)

	suiteWithout, err := ParseTestFile(pathWithout)
	require.NoError(t, err)
	require.NotNil(t, suiteWithout)
	require.Len(t, suiteWithout.Tests, 1)

	// exit_code: 0 must produce *int(0)
	require.NotNil(t, suiteWith.Tests[0].Expect.ExitCode,
		"exit_code: 0 must produce non-nil pointer")
	assert.Equal(t, 0, *suiteWith.Tests[0].Expect.ExitCode)

	// omitted exit_code must produce nil
	assert.Nil(t, suiteWithout.Tests[0].Expect.ExitCode,
		"Omitted exit_code must be nil, not *int(0)")
}

// ---------------------------------------------------------------------------
// StdoutIsJSON false vs omitted
// ---------------------------------------------------------------------------

func TestParseTestFile_StdoutIsJSON_FalseVsOmitted(t *testing.T) {
	yamlWithFalse := `tool: jsontest
tests:
  - name: not json
    expect:
      stdout_is_json: false
`
	yamlOmitted := `tool: jsontest
tests:
  - name: unspecified
    expect:
      exit_code: 0
`
	dir := t.TempDir()
	pathFalse := writeTestFile(t, dir, "false.test.yaml", yamlWithFalse)
	pathOmitted := writeTestFile(t, dir, "omitted.test.yaml", yamlOmitted)

	suiteFalse, err := ParseTestFile(pathFalse)
	require.NoError(t, err)
	require.NotNil(t, suiteFalse)

	suiteOmitted, err := ParseTestFile(pathOmitted)
	require.NoError(t, err)
	require.NotNil(t, suiteOmitted)

	require.NotNil(t, suiteFalse.Tests[0].Expect.StdoutIsJSON,
		"stdout_is_json: false must produce non-nil *bool")
	assert.False(t, *suiteFalse.Tests[0].Expect.StdoutIsJSON)

	assert.Nil(t, suiteOmitted.Tests[0].Expect.StdoutIsJSON,
		"Omitted stdout_is_json must be nil, not *bool(false)")
}
