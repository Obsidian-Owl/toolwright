package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Obsidian-Owl/toolwright/internal/manifest"
	"github.com/Obsidian-Owl/toolwright/internal/tooltest"
)

// ---------------------------------------------------------------------------
// Mock types
// ---------------------------------------------------------------------------

type mockSuiteRunner struct {
	reports           map[string]*tooltest.TestReport // keyed by suite.Tool
	errs              map[string]error
	runCalled         bool
	runParallelCalled bool
	parallelWorkers   int
	runSuites         []string // tool names in order of Run calls
	runParallelSuites []string // tool names in order of RunParallel calls
}

func (m *mockSuiteRunner) Run(_ context.Context, suite tooltest.TestSuite, _ manifest.Toolkit) (*tooltest.TestReport, error) {
	m.runCalled = true
	m.runSuites = append(m.runSuites, suite.Tool)
	if m.errs != nil {
		if err, ok := m.errs[suite.Tool]; ok {
			return nil, err
		}
	}
	if m.reports != nil {
		if r, ok := m.reports[suite.Tool]; ok {
			return r, nil
		}
	}
	return &tooltest.TestReport{Tool: suite.Tool}, nil
}

func (m *mockSuiteRunner) RunParallel(_ context.Context, suite tooltest.TestSuite, _ manifest.Toolkit, workers int) (*tooltest.TestReport, error) {
	m.runParallelCalled = true
	m.parallelWorkers = workers
	m.runParallelSuites = append(m.runParallelSuites, suite.Tool)
	if m.errs != nil {
		if err, ok := m.errs[suite.Tool]; ok {
			return nil, err
		}
	}
	if m.reports != nil {
		if r, ok := m.reports[suite.Tool]; ok {
			return r, nil
		}
	}
	return &tooltest.TestReport{Tool: suite.Tool}, nil
}

type mockTestParser struct {
	suites []tooltest.TestSuite
	err    error
	dir    string // the dir argument received
}

func (m *mockTestParser) ParseDir(dir string) ([]tooltest.TestSuite, error) {
	m.dir = dir
	return m.suites, m.err
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// executeTestCmd runs the test command through the root command tree and
// returns stdout, stderr, and the error (if any).
func executeTestCmd(cfg *testConfig, args ...string) (stdout, stderr string, err error) {
	root := NewRootCommand()
	testCmd := newTestCmd(cfg)
	root.AddCommand(testCmd)
	outBuf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	root.SetOut(outBuf)
	root.SetErr(errBuf)
	root.SetArgs(append([]string{"test"}, args...))
	execErr := root.Execute()
	return outBuf.String(), errBuf.String(), execErr
}

// writeTestManifest writes a manifest file to the given dir and returns the path.
func writeTestManifest(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, "toolwright.yaml")
	err := os.WriteFile(path, []byte(content), 0644)
	require.NoError(t, err, "test setup: writing manifest file")
	return path
}

// testManifestSingleTool returns a minimal manifest with one tool.
func testManifestSingleTool() string {
	return `apiVersion: toolwright/v1
kind: Toolkit
metadata:
  name: test-toolkit
  version: 1.0.0
  description: Test toolkit
tools:
  - name: hello
    description: Say hello
    entrypoint: ./hello.sh
    auth: none
`
}

// testManifestMultiTool returns a manifest with two tools.
func testManifestMultiTool() string {
	return `apiVersion: toolwright/v1
kind: Toolkit
metadata:
  name: multi-toolkit
  version: 1.0.0
  description: Multi-tool toolkit
tools:
  - name: alpha
    description: Alpha tool
    entrypoint: ./alpha.sh
    auth: none
  - name: beta
    description: Beta tool
    entrypoint: ./beta.sh
    auth: none
`
}

// passingReport returns a report with all tests passing for the given tool.
func passingReport(tool string) *tooltest.TestReport {
	return &tooltest.TestReport{
		Tool:   tool,
		Total:  2,
		Passed: 2,
		Failed: 0,
		Results: []tooltest.TestResult{
			{Name: "test-one", Status: "pass", Duration: 10 * time.Millisecond},
			{Name: "test-two", Status: "pass", Duration: 20 * time.Millisecond},
		},
	}
}

// failingReport returns a report with one pass and one fail.
func failingReport(tool string) *tooltest.TestReport {
	return &tooltest.TestReport{
		Tool:   tool,
		Total:  2,
		Passed: 1,
		Failed: 1,
		Results: []tooltest.TestResult{
			{Name: "test-one", Status: "pass", Duration: 10 * time.Millisecond},
			{Name: "test-two", Status: "fail", Duration: 30 * time.Millisecond, Error: "exit code mismatch: expected 0, got 1"},
		},
	}
}

// singleSuite returns a TestSuite for the given tool with the given test names.
func singleSuite(tool string, testNames ...string) tooltest.TestSuite {
	tests := make([]tooltest.TestCase, len(testNames))
	for i, name := range testNames {
		tests[i] = tooltest.TestCase{Name: name}
	}
	return tooltest.TestSuite{Tool: tool, Tests: tests}
}

// ---------------------------------------------------------------------------
// AC-12: Command structure
// ---------------------------------------------------------------------------

func TestNewTestCmd_ReturnsNonNil(t *testing.T) {
	cfg := &testConfig{}
	cmd := newTestCmd(cfg)
	require.NotNil(t, cmd, "newTestCmd must return a non-nil *cobra.Command")
}

func TestNewTestCmd_HasCorrectUseField(t *testing.T) {
	cfg := &testConfig{}
	cmd := newTestCmd(cfg)
	assert.Equal(t, "test", cmd.Use,
		"test command Use field must be 'test'")
}

func TestNewTestCmd_HasManifestFlag(t *testing.T) {
	cfg := &testConfig{}
	cmd := newTestCmd(cfg)
	f := cmd.Flags().Lookup("manifest")
	require.NotNil(t, f, "--manifest flag must exist on the test command")
	assert.Equal(t, "toolwright.yaml", f.DefValue,
		"--manifest flag default must be 'toolwright.yaml'")
}

func TestNewTestCmd_HasManifestShortFlag(t *testing.T) {
	cfg := &testConfig{}
	cmd := newTestCmd(cfg)
	f := cmd.Flags().ShorthandLookup("m")
	require.NotNil(t, f, "-m shorthand must exist for --manifest flag")
	assert.Equal(t, "manifest", f.Name,
		"-m must be shorthand for --manifest")
}

func TestNewTestCmd_HasTestsFlag(t *testing.T) {
	cfg := &testConfig{}
	cmd := newTestCmd(cfg)
	f := cmd.Flags().Lookup("tests")
	require.NotNil(t, f, "--tests flag must exist on the test command")
	assert.Equal(t, "tests/", f.DefValue,
		"--tests flag default must be 'tests/'")
}

func TestNewTestCmd_HasTestsShortFlag(t *testing.T) {
	cfg := &testConfig{}
	cmd := newTestCmd(cfg)
	f := cmd.Flags().ShorthandLookup("t")
	require.NotNil(t, f, "-t shorthand must exist for --tests flag")
	assert.Equal(t, "tests", f.Name,
		"-t must be shorthand for --tests")
}

func TestNewTestCmd_HasFilterFlag(t *testing.T) {
	cfg := &testConfig{}
	cmd := newTestCmd(cfg)
	f := cmd.Flags().Lookup("filter")
	require.NotNil(t, f, "--filter flag must exist on the test command")
	assert.Equal(t, "", f.DefValue,
		"--filter flag default must be empty string")
}

func TestNewTestCmd_HasFilterShortFlag(t *testing.T) {
	cfg := &testConfig{}
	cmd := newTestCmd(cfg)
	f := cmd.Flags().ShorthandLookup("f")
	require.NotNil(t, f, "-f shorthand must exist for --filter flag")
	assert.Equal(t, "filter", f.Name,
		"-f must be shorthand for --filter")
}

func TestNewTestCmd_HasParallelFlag(t *testing.T) {
	cfg := &testConfig{}
	cmd := newTestCmd(cfg)
	f := cmd.Flags().Lookup("parallel")
	require.NotNil(t, f, "--parallel flag must exist on the test command")
	assert.Equal(t, "1", f.DefValue,
		"--parallel flag default must be '1'")
}

func TestNewTestCmd_HasParallelShortFlag(t *testing.T) {
	cfg := &testConfig{}
	cmd := newTestCmd(cfg)
	f := cmd.Flags().ShorthandLookup("p")
	require.NotNil(t, f, "-p shorthand must exist for --parallel flag")
	assert.Equal(t, "parallel", f.Name,
		"-p must be shorthand for --parallel")
}

func TestNewTestCmd_InheritsJsonFlag(t *testing.T) {
	root := NewRootCommand()
	cfg := &testConfig{}
	testCmd := newTestCmd(cfg)
	root.AddCommand(testCmd)

	// Parse args so flags are initialized.
	root.SetArgs([]string{"test", "--json", "--help"})
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	_ = root.Execute()

	f := testCmd.Flags().Lookup("json")
	require.NotNil(t, f, "--json must be accessible on the test subcommand via persistent flags")
}

// ---------------------------------------------------------------------------
// AC-12: Test execution — passing report → TAP output
// ---------------------------------------------------------------------------

func TestTest_PassingReport_OutputsTAP(t *testing.T) {
	dir := t.TempDir()
	mPath := writeTestManifest(t, dir, testManifestSingleTool())

	parser := &mockTestParser{
		suites: []tooltest.TestSuite{singleSuite("hello", "test-one", "test-two")},
	}
	runner := &mockSuiteRunner{
		reports: map[string]*tooltest.TestReport{"hello": passingReport("hello")},
	}
	cfg := &testConfig{Runner: runner, Parser: parser}

	stdout, _, err := executeTestCmd(cfg, "-m", mPath)
	require.NoError(t, err, "test with all passing must not return error")

	assert.Contains(t, stdout, "TAP version 13",
		"TAP output must start with TAP version header")
	assert.Contains(t, stdout, "1..2",
		"TAP output must contain plan line showing test count")
	assert.Contains(t, stdout, "ok",
		"TAP output must contain 'ok' for passing tests")
}

// ---------------------------------------------------------------------------
// AC-12: Test execution — passing report + --json → JSON output
// ---------------------------------------------------------------------------

func TestTest_PassingReport_JSON_OutputsJSON(t *testing.T) {
	dir := t.TempDir()
	mPath := writeTestManifest(t, dir, testManifestSingleTool())

	parser := &mockTestParser{
		suites: []tooltest.TestSuite{singleSuite("hello", "test-one", "test-two")},
	}
	runner := &mockSuiteRunner{
		reports: map[string]*tooltest.TestReport{"hello": passingReport("hello")},
	}
	cfg := &testConfig{Runner: runner, Parser: parser}

	stdout, _, err := executeTestCmd(cfg, "-m", mPath, "--json")
	require.NoError(t, err, "test with --json and all passing must not return error")

	require.True(t, json.Valid([]byte(stdout)),
		"--json output must be valid JSON, got: %s", stdout)

	// Must not contain TAP output when --json is specified.
	assert.NotContains(t, stdout, "TAP version",
		"--json output must not contain TAP headers")
}

// ---------------------------------------------------------------------------
// AC-12: Test execution — failing report → error returned, TAP shows failures
// ---------------------------------------------------------------------------

func TestTest_FailingReport_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	mPath := writeTestManifest(t, dir, testManifestSingleTool())

	parser := &mockTestParser{
		suites: []tooltest.TestSuite{singleSuite("hello", "test-one", "test-two")},
	}
	runner := &mockSuiteRunner{
		reports: map[string]*tooltest.TestReport{"hello": failingReport("hello")},
	}
	cfg := &testConfig{Runner: runner, Parser: parser}

	stdout, _, err := executeTestCmd(cfg, "-m", mPath)
	require.Error(t, err, "test with failures must return an error")

	assert.Contains(t, stdout, "not ok",
		"TAP output must contain 'not ok' for failing tests")
}

func TestTest_FailingReport_JSON_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	mPath := writeTestManifest(t, dir, testManifestSingleTool())

	parser := &mockTestParser{
		suites: []tooltest.TestSuite{singleSuite("hello", "test-one", "test-two")},
	}
	runner := &mockSuiteRunner{
		reports: map[string]*tooltest.TestReport{"hello": failingReport("hello")},
	}
	cfg := &testConfig{Runner: runner, Parser: parser}

	stdout, _, err := executeTestCmd(cfg, "-m", mPath, "--json")
	require.Error(t, err, "test with failures and --json must return an error")
	require.True(t, json.Valid([]byte(stdout)),
		"--json error output must be valid JSON, got: %s", stdout)
}

// ---------------------------------------------------------------------------
// AC-12: Multiple suites → all run and reported
// ---------------------------------------------------------------------------

func TestTest_MultipleSuites_AllRunAndReported(t *testing.T) {
	dir := t.TempDir()
	mPath := writeTestManifest(t, dir, testManifestMultiTool())

	parser := &mockTestParser{
		suites: []tooltest.TestSuite{
			singleSuite("alpha", "alpha-test"),
			singleSuite("beta", "beta-test"),
		},
	}
	runner := &mockSuiteRunner{
		reports: map[string]*tooltest.TestReport{
			"alpha": {
				Tool: "alpha", Total: 1, Passed: 1, Failed: 0,
				Results: []tooltest.TestResult{
					{Name: "alpha-test", Status: "pass", Duration: 5 * time.Millisecond},
				},
			},
			"beta": {
				Tool: "beta", Total: 1, Passed: 1, Failed: 0,
				Results: []tooltest.TestResult{
					{Name: "beta-test", Status: "pass", Duration: 5 * time.Millisecond},
				},
			},
		},
	}
	cfg := &testConfig{Runner: runner, Parser: parser}

	stdout, _, err := executeTestCmd(cfg, "-m", mPath)
	require.NoError(t, err, "test with multiple passing suites must not error")

	// Both suite tools must have been run.
	assert.Contains(t, runner.runSuites, "alpha",
		"runner must have been called for alpha suite")
	assert.Contains(t, runner.runSuites, "beta",
		"runner must have been called for beta suite")
	// Output must mention both.
	assert.Contains(t, stdout, "alpha-test",
		"output must include results for alpha suite")
	assert.Contains(t, stdout, "beta-test",
		"output must include results for beta suite")
}

// ---------------------------------------------------------------------------
// AC-12: --filter → regex filters test names
// ---------------------------------------------------------------------------

func TestTest_Filter_OnlyMatchingTestsRun(t *testing.T) {
	dir := t.TempDir()
	mPath := writeTestManifest(t, dir, testManifestMultiTool())

	parser := &mockTestParser{
		suites: []tooltest.TestSuite{
			singleSuite("alpha", "alpha-test-1", "alpha-test-2"),
			singleSuite("beta", "beta-test-1"),
		},
	}
	alphaReport := &tooltest.TestReport{
		Tool: "alpha", Total: 2, Passed: 2, Failed: 0,
		Results: []tooltest.TestResult{
			{Name: "alpha-test-1", Status: "pass", Duration: 5 * time.Millisecond},
			{Name: "alpha-test-2", Status: "pass", Duration: 5 * time.Millisecond},
		},
	}
	runner := &mockSuiteRunner{
		reports: map[string]*tooltest.TestReport{
			"alpha": alphaReport,
		},
	}
	cfg := &testConfig{Runner: runner, Parser: parser}

	stdout, _, err := executeTestCmd(cfg, "-m", mPath, "--filter", "alpha")
	require.NoError(t, err, "test with --filter matching suites must not error")

	// Only alpha should have been run, not beta.
	assert.Contains(t, stdout, "alpha",
		"output must include alpha results when filter matches")
	// beta-test should not appear in runner calls or output.
	assert.NotContains(t, runner.runSuites, "beta",
		"runner must NOT be called for beta suite when filter is 'alpha'")
}

func TestTest_Filter_RegexPattern(t *testing.T) {
	dir := t.TempDir()
	mPath := writeTestManifest(t, dir, testManifestSingleTool())

	parser := &mockTestParser{
		suites: []tooltest.TestSuite{
			{
				Tool: "hello",
				Tests: []tooltest.TestCase{
					{Name: "returns-greeting"},
					{Name: "handles-empty-name"},
					{Name: "returns-error-on-missing"},
				},
			},
		},
	}
	runner := &mockSuiteRunner{
		reports: map[string]*tooltest.TestReport{
			"hello": {
				Tool: "hello", Total: 1, Passed: 1, Failed: 0,
				Results: []tooltest.TestResult{
					{Name: "returns-greeting", Status: "pass", Duration: 5 * time.Millisecond},
				},
			},
		},
	}
	cfg := &testConfig{Runner: runner, Parser: parser}

	stdout, _, err := executeTestCmd(cfg, "-m", mPath, "--filter", "^returns")
	require.NoError(t, err, "test with regex filter must not error")

	// Only tests matching "^returns" should appear.
	assert.Contains(t, stdout, "returns",
		"filtered output must include matching tests")
	assert.NotContains(t, stdout, "handles-empty-name",
		"filtered output must NOT include non-matching tests")
}

func TestTest_Filter_NoMatches_NoError(t *testing.T) {
	dir := t.TempDir()
	mPath := writeTestManifest(t, dir, testManifestSingleTool())

	parser := &mockTestParser{
		suites: []tooltest.TestSuite{singleSuite("hello", "test-one")},
	}
	runner := &mockSuiteRunner{}
	cfg := &testConfig{Runner: runner, Parser: parser}

	stdout, _, err := executeTestCmd(cfg, "-m", mPath, "--filter", "nonexistent-pattern")
	require.NoError(t, err, "filter with no matches must not return error")

	// Runner should not be called since nothing matches.
	assert.False(t, runner.runCalled,
		"runner must NOT be called when filter matches no tests")
	// Output should indicate zero tests.
	assert.Contains(t, stdout, "0",
		"output must indicate zero tests when filter matches nothing")
}

// ---------------------------------------------------------------------------
// AC-12: --parallel N → RunParallel called with workers=N
// ---------------------------------------------------------------------------

func TestTest_Parallel_CallsRunParallel(t *testing.T) {
	dir := t.TempDir()
	mPath := writeTestManifest(t, dir, testManifestSingleTool())

	parser := &mockTestParser{
		suites: []tooltest.TestSuite{singleSuite("hello", "test-one")},
	}
	runner := &mockSuiteRunner{
		reports: map[string]*tooltest.TestReport{
			"hello": passingReport("hello"),
		},
	}
	cfg := &testConfig{Runner: runner, Parser: parser}

	_, _, err := executeTestCmd(cfg, "-m", mPath, "--parallel", "4")
	require.NoError(t, err, "test with --parallel 4 must not error")

	assert.True(t, runner.runParallelCalled,
		"RunParallel must be called when --parallel > 1")
	assert.False(t, runner.runCalled,
		"Run must NOT be called when --parallel > 1")
	assert.Equal(t, 4, runner.parallelWorkers,
		"RunParallel must receive workers=4")
}

func TestTest_DefaultParallel_CallsRun(t *testing.T) {
	dir := t.TempDir()
	mPath := writeTestManifest(t, dir, testManifestSingleTool())

	parser := &mockTestParser{
		suites: []tooltest.TestSuite{singleSuite("hello", "test-one")},
	}
	runner := &mockSuiteRunner{
		reports: map[string]*tooltest.TestReport{
			"hello": passingReport("hello"),
		},
	}
	cfg := &testConfig{Runner: runner, Parser: parser}

	_, _, err := executeTestCmd(cfg, "-m", mPath)
	require.NoError(t, err, "test with default parallel must not error")

	assert.True(t, runner.runCalled,
		"Run must be called when --parallel is 1 (default)")
	assert.False(t, runner.runParallelCalled,
		"RunParallel must NOT be called when --parallel is 1")
}

func TestTest_Parallel1_ExplicitlyCallsRun(t *testing.T) {
	dir := t.TempDir()
	mPath := writeTestManifest(t, dir, testManifestSingleTool())

	parser := &mockTestParser{
		suites: []tooltest.TestSuite{singleSuite("hello", "test-one")},
	}
	runner := &mockSuiteRunner{
		reports: map[string]*tooltest.TestReport{
			"hello": passingReport("hello"),
		},
	}
	cfg := &testConfig{Runner: runner, Parser: parser}

	_, _, err := executeTestCmd(cfg, "-m", mPath, "--parallel", "1")
	require.NoError(t, err, "test with --parallel 1 must not error")

	assert.True(t, runner.runCalled,
		"Run must be called when --parallel 1 is explicitly specified")
	assert.False(t, runner.runParallelCalled,
		"RunParallel must NOT be called when --parallel is 1")
}

// ---------------------------------------------------------------------------
// AC-12: Error handling — manifest not found
// ---------------------------------------------------------------------------

func TestTest_ManifestNotFound_ReturnsError(t *testing.T) {
	parser := &mockTestParser{}
	runner := &mockSuiteRunner{}
	cfg := &testConfig{Runner: runner, Parser: parser}

	_, _, err := executeTestCmd(cfg, "-m", "/nonexistent/path/toolwright.yaml")
	require.Error(t, err, "test with nonexistent manifest must return an error")
	assert.False(t, runner.runCalled,
		"runner must NOT be called when manifest is missing")
}

func TestTest_ManifestNotFound_JSON_HasError(t *testing.T) {
	parser := &mockTestParser{}
	runner := &mockSuiteRunner{}
	cfg := &testConfig{Runner: runner, Parser: parser}

	stdout, _, _ := executeTestCmd(cfg, "--json", "-m", "/nonexistent/path/toolwright.yaml")
	require.NotEmpty(t, stdout,
		"JSON output must be produced for manifest error")

	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &got),
		"output must be valid JSON, got: %s", stdout)

	_, hasError := got["error"]
	assert.True(t, hasError,
		"JSON output must have error object for missing manifest")
}

// ---------------------------------------------------------------------------
// AC-12: Error handling — tests directory not found
// ---------------------------------------------------------------------------

func TestTest_TestsDirNotFound_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	mPath := writeTestManifest(t, dir, testManifestSingleTool())

	parser := &mockTestParser{
		err: fmt.Errorf("read test directory %q: no such file or directory", "/nonexistent/tests"),
	}
	runner := &mockSuiteRunner{}
	cfg := &testConfig{Runner: runner, Parser: parser}

	_, _, err := executeTestCmd(cfg, "-m", mPath, "--tests", "/nonexistent/tests")
	require.Error(t, err, "test with nonexistent tests dir must return an error")
	assert.False(t, runner.runCalled,
		"runner must NOT be called when tests directory is missing")
}

// ---------------------------------------------------------------------------
// AC-12: Error handling — runner returns error → propagated
// ---------------------------------------------------------------------------

func TestTest_RunnerError_Propagated(t *testing.T) {
	dir := t.TempDir()
	mPath := writeTestManifest(t, dir, testManifestSingleTool())

	parser := &mockTestParser{
		suites: []tooltest.TestSuite{singleSuite("hello", "test-one")},
	}
	runner := &mockSuiteRunner{
		errs: map[string]error{
			"hello": errors.New("tool \"hello\" not found in toolkit"),
		},
	}
	cfg := &testConfig{Runner: runner, Parser: parser}

	_, _, err := executeTestCmd(cfg, "-m", mPath)
	require.Error(t, err, "test must propagate runner errors")
	assert.Contains(t, err.Error(), "hello",
		"error message must contain the tool name from the runner error")
}

// ---------------------------------------------------------------------------
// AC-12: --json error → JSON error output
// ---------------------------------------------------------------------------

func TestTest_RunnerError_JSON_OutputsJSON(t *testing.T) {
	dir := t.TempDir()
	mPath := writeTestManifest(t, dir, testManifestSingleTool())

	parser := &mockTestParser{
		suites: []tooltest.TestSuite{singleSuite("hello", "test-one")},
	}
	runner := &mockSuiteRunner{
		errs: map[string]error{
			"hello": errors.New("tool \"hello\" not found in toolkit"),
		},
	}
	cfg := &testConfig{Runner: runner, Parser: parser}

	stdout, _, err := executeTestCmd(cfg, "-m", mPath, "--json")
	require.Error(t, err, "test with runner error and --json must return error")
	require.True(t, json.Valid([]byte(stdout)),
		"--json error output must be valid JSON, got: %s", stdout)
}

// ---------------------------------------------------------------------------
// AC-12: TAP output format
// ---------------------------------------------------------------------------

func TestTest_TAP_StartsWithVersion13(t *testing.T) {
	dir := t.TempDir()
	mPath := writeTestManifest(t, dir, testManifestSingleTool())

	parser := &mockTestParser{
		suites: []tooltest.TestSuite{singleSuite("hello", "test-one")},
	}
	runner := &mockSuiteRunner{
		reports: map[string]*tooltest.TestReport{
			"hello": {
				Tool: "hello", Total: 1, Passed: 1, Failed: 0,
				Results: []tooltest.TestResult{
					{Name: "test-one", Status: "pass", Duration: 5 * time.Millisecond},
				},
			},
		},
	}
	cfg := &testConfig{Runner: runner, Parser: parser}

	stdout, _, err := executeTestCmd(cfg, "-m", mPath)
	require.NoError(t, err)

	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	require.NotEmpty(t, lines, "stdout must not be empty")
	assert.Equal(t, "TAP version 13", lines[0],
		"TAP output must begin with 'TAP version 13' as the first line")
}

func TestTest_TAP_ContainsTestCount(t *testing.T) {
	dir := t.TempDir()
	mPath := writeTestManifest(t, dir, testManifestSingleTool())

	parser := &mockTestParser{
		suites: []tooltest.TestSuite{singleSuite("hello", "a", "b", "c")},
	}
	runner := &mockSuiteRunner{
		reports: map[string]*tooltest.TestReport{
			"hello": {
				Tool: "hello", Total: 3, Passed: 3, Failed: 0,
				Results: []tooltest.TestResult{
					{Name: "a", Status: "pass", Duration: 1 * time.Millisecond},
					{Name: "b", Status: "pass", Duration: 1 * time.Millisecond},
					{Name: "c", Status: "pass", Duration: 1 * time.Millisecond},
				},
			},
		},
	}
	cfg := &testConfig{Runner: runner, Parser: parser}

	stdout, _, err := executeTestCmd(cfg, "-m", mPath)
	require.NoError(t, err)

	assert.Contains(t, stdout, "1..3",
		"TAP output must contain plan line with total test count")
}

func TestTest_TAP_OkForPassing_NotOkForFailing(t *testing.T) {
	dir := t.TempDir()
	mPath := writeTestManifest(t, dir, testManifestSingleTool())

	parser := &mockTestParser{
		suites: []tooltest.TestSuite{singleSuite("hello", "pass-test", "fail-test")},
	}
	runner := &mockSuiteRunner{
		reports: map[string]*tooltest.TestReport{
			"hello": {
				Tool: "hello", Total: 2, Passed: 1, Failed: 1,
				Results: []tooltest.TestResult{
					{Name: "pass-test", Status: "pass", Duration: 5 * time.Millisecond},
					{Name: "fail-test", Status: "fail", Duration: 10 * time.Millisecond, Error: "assertion failed"},
				},
			},
		},
	}
	cfg := &testConfig{Runner: runner, Parser: parser}

	stdout, _, _ := executeTestCmd(cfg, "-m", mPath)

	// Verify "ok" and "not ok" lines are present with correct test names.
	assert.Regexp(t, `ok \d+ - pass-test`, stdout,
		"TAP output must have 'ok N - pass-test' for passing test")
	assert.Regexp(t, `not ok \d+ - fail-test`, stdout,
		"TAP output must have 'not ok N - fail-test' for failing test")
}

// ---------------------------------------------------------------------------
// AC-12: JSON output format
// ---------------------------------------------------------------------------

func TestTest_JSON_HasRequiredFields(t *testing.T) {
	dir := t.TempDir()
	mPath := writeTestManifest(t, dir, testManifestSingleTool())

	parser := &mockTestParser{
		suites: []tooltest.TestSuite{singleSuite("hello", "test-one")},
	}
	runner := &mockSuiteRunner{
		reports: map[string]*tooltest.TestReport{
			"hello": passingReport("hello"),
		},
	}
	cfg := &testConfig{Runner: runner, Parser: parser}

	stdout, _, err := executeTestCmd(cfg, "-m", mPath, "--json")
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &got),
		"JSON output must parse, got: %s", stdout)

	_, hasTool := got["tool"]
	assert.True(t, hasTool, "JSON output must have 'tool' field")
	_, hasTotal := got["total"]
	assert.True(t, hasTotal, "JSON output must have 'total' field")
	_, hasPassed := got["passed"]
	assert.True(t, hasPassed, "JSON output must have 'passed' field")
	_, hasFailed := got["failed"]
	assert.True(t, hasFailed, "JSON output must have 'failed' field")
	_, hasResults := got["results"]
	assert.True(t, hasResults, "JSON output must have 'results' field")
}

func TestTest_JSON_FieldValuesMatch(t *testing.T) {
	dir := t.TempDir()
	mPath := writeTestManifest(t, dir, testManifestSingleTool())

	report := &tooltest.TestReport{
		Tool: "hello", Total: 3, Passed: 2, Failed: 1,
		Results: []tooltest.TestResult{
			{Name: "a", Status: "pass", Duration: 5 * time.Millisecond},
			{Name: "b", Status: "pass", Duration: 5 * time.Millisecond},
			{Name: "c", Status: "fail", Duration: 10 * time.Millisecond, Error: "oops"},
		},
	}
	parser := &mockTestParser{
		suites: []tooltest.TestSuite{singleSuite("hello", "a", "b", "c")},
	}
	runner := &mockSuiteRunner{
		reports: map[string]*tooltest.TestReport{"hello": report},
	}
	cfg := &testConfig{Runner: runner, Parser: parser}

	stdout, _, _ := executeTestCmd(cfg, "-m", mPath, "--json")

	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &got))

	assert.Equal(t, "hello", got["tool"],
		"JSON 'tool' field must match report tool name")
	assert.EqualValues(t, 3, got["total"],
		"JSON 'total' field must match report total")
	assert.EqualValues(t, 2, got["passed"],
		"JSON 'passed' field must match report passed count")
	assert.EqualValues(t, 1, got["failed"],
		"JSON 'failed' field must match report failed count")

	results, ok := got["results"].([]any)
	require.True(t, ok, "JSON 'results' must be an array")
	assert.Len(t, results, 3,
		"JSON 'results' must have 3 entries matching report results")
}

func TestTest_JSON_ResultEntryFields(t *testing.T) {
	dir := t.TempDir()
	mPath := writeTestManifest(t, dir, testManifestSingleTool())

	report := &tooltest.TestReport{
		Tool: "hello", Total: 1, Passed: 1, Failed: 0,
		Results: []tooltest.TestResult{
			{Name: "test-one", Status: "pass", Duration: 42 * time.Millisecond},
		},
	}
	parser := &mockTestParser{
		suites: []tooltest.TestSuite{singleSuite("hello", "test-one")},
	}
	runner := &mockSuiteRunner{
		reports: map[string]*tooltest.TestReport{"hello": report},
	}
	cfg := &testConfig{Runner: runner, Parser: parser}

	stdout, _, _ := executeTestCmd(cfg, "-m", mPath, "--json")

	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &got))

	results := got["results"].([]any)
	require.Len(t, results, 1)

	entry, ok := results[0].(map[string]any)
	require.True(t, ok, "results entry must be a JSON object")
	assert.Equal(t, "test-one", entry["name"],
		"result entry must have correct name")
	assert.Equal(t, "pass", entry["status"],
		"result entry must have correct status")
	_, hasDuration := entry["duration_ms"]
	assert.True(t, hasDuration,
		"result entry must have 'duration_ms' field")
}

// ---------------------------------------------------------------------------
// AC-12: --tests flag is passed to parser
// ---------------------------------------------------------------------------

func TestTest_TestsFlag_PassedToParser(t *testing.T) {
	dir := t.TempDir()
	mPath := writeTestManifest(t, dir, testManifestSingleTool())
	customDir := filepath.Join(dir, "custom-tests")
	require.NoError(t, os.MkdirAll(customDir, 0755))

	parser := &mockTestParser{
		suites: []tooltest.TestSuite{},
	}
	runner := &mockSuiteRunner{}
	cfg := &testConfig{Runner: runner, Parser: parser}

	_, _, _ = executeTestCmd(cfg, "-m", mPath, "--tests", customDir)
	assert.Equal(t, customDir, parser.dir,
		"parser must receive the --tests directory path")
}

func TestTest_DefaultTestsDir_IsTests(t *testing.T) {
	dir := t.TempDir()
	mPath := writeTestManifest(t, dir, testManifestSingleTool())

	parser := &mockTestParser{
		suites: []tooltest.TestSuite{},
	}
	runner := &mockSuiteRunner{}
	cfg := &testConfig{Runner: runner, Parser: parser}

	_, _, _ = executeTestCmd(cfg, "-m", mPath)
	assert.Equal(t, "tests/", parser.dir,
		"parser must receive default 'tests/' directory when --tests is not specified")
}

// ---------------------------------------------------------------------------
// AC-12: Anti-hardcoding — different reports produce different output
// ---------------------------------------------------------------------------

func TestTest_DifferentReports_DifferentOutput(t *testing.T) {
	dir := t.TempDir()
	mPath := writeTestManifest(t, dir, testManifestSingleTool())

	// First run with passing report.
	parser1 := &mockTestParser{
		suites: []tooltest.TestSuite{singleSuite("hello", "test-alpha")},
	}
	report1 := &tooltest.TestReport{
		Tool: "hello", Total: 1, Passed: 1, Failed: 0,
		Results: []tooltest.TestResult{
			{Name: "test-alpha", Status: "pass", Duration: 10 * time.Millisecond},
		},
	}
	runner1 := &mockSuiteRunner{
		reports: map[string]*tooltest.TestReport{"hello": report1},
	}
	cfg1 := &testConfig{Runner: runner1, Parser: parser1}
	stdout1, _, err1 := executeTestCmd(cfg1, "-m", mPath)
	require.NoError(t, err1)

	// Second run with different report.
	parser2 := &mockTestParser{
		suites: []tooltest.TestSuite{singleSuite("hello", "test-beta")},
	}
	report2 := &tooltest.TestReport{
		Tool: "hello", Total: 1, Passed: 0, Failed: 1,
		Results: []tooltest.TestResult{
			{Name: "test-beta", Status: "fail", Duration: 50 * time.Millisecond, Error: "assertion failed"},
		},
	}
	runner2 := &mockSuiteRunner{
		reports: map[string]*tooltest.TestReport{"hello": report2},
	}
	cfg2 := &testConfig{Runner: runner2, Parser: parser2}
	stdout2, _, _ := executeTestCmd(cfg2, "-m", mPath)

	assert.NotEqual(t, stdout1, stdout2,
		"different test reports must produce different output; anti-hardcoding")
}

func TestTest_DifferentReports_DifferentJSON(t *testing.T) {
	dir := t.TempDir()
	mPath := writeTestManifest(t, dir, testManifestSingleTool())

	// First run.
	parser1 := &mockTestParser{
		suites: []tooltest.TestSuite{singleSuite("hello", "test-alpha")},
	}
	runner1 := &mockSuiteRunner{
		reports: map[string]*tooltest.TestReport{"hello": passingReport("hello")},
	}
	cfg1 := &testConfig{Runner: runner1, Parser: parser1}
	stdout1, _, _ := executeTestCmd(cfg1, "-m", mPath, "--json")

	// Second run with failing report (different total/passed/failed).
	parser2 := &mockTestParser{
		suites: []tooltest.TestSuite{singleSuite("hello", "test-alpha")},
	}
	runner2 := &mockSuiteRunner{
		reports: map[string]*tooltest.TestReport{"hello": failingReport("hello")},
	}
	cfg2 := &testConfig{Runner: runner2, Parser: parser2}
	stdout2, _, _ := executeTestCmd(cfg2, "-m", mPath, "--json")

	assert.NotEqual(t, stdout1, stdout2,
		"different test reports must produce different JSON output; anti-hardcoding")
}

// ---------------------------------------------------------------------------
// AC-12: Parallel with multiple suites
// ---------------------------------------------------------------------------

func TestTest_Parallel_MultipleSuites_AllRun(t *testing.T) {
	dir := t.TempDir()
	mPath := writeTestManifest(t, dir, testManifestMultiTool())

	parser := &mockTestParser{
		suites: []tooltest.TestSuite{
			singleSuite("alpha", "a1"),
			singleSuite("beta", "b1"),
		},
	}
	runner := &mockSuiteRunner{
		reports: map[string]*tooltest.TestReport{
			"alpha": {Tool: "alpha", Total: 1, Passed: 1, Results: []tooltest.TestResult{
				{Name: "a1", Status: "pass", Duration: 5 * time.Millisecond},
			}},
			"beta": {Tool: "beta", Total: 1, Passed: 1, Results: []tooltest.TestResult{
				{Name: "b1", Status: "pass", Duration: 5 * time.Millisecond},
			}},
		},
	}
	cfg := &testConfig{Runner: runner, Parser: parser}

	_, _, err := executeTestCmd(cfg, "-m", mPath, "--parallel", "2")
	require.NoError(t, err, "parallel test with multiple suites must not error")

	assert.True(t, runner.runParallelCalled,
		"RunParallel must be called with --parallel 2")
	// Both suites must have been dispatched.
	assert.Len(t, runner.runParallelSuites, 2,
		"RunParallel must be called for both suites")
}

// ---------------------------------------------------------------------------
// AC-12: Empty test suites — no tests found
// ---------------------------------------------------------------------------

func TestTest_NoSuites_NoError(t *testing.T) {
	dir := t.TempDir()
	mPath := writeTestManifest(t, dir, testManifestSingleTool())

	parser := &mockTestParser{
		suites: []tooltest.TestSuite{},
	}
	runner := &mockSuiteRunner{}
	cfg := &testConfig{Runner: runner, Parser: parser}

	_, _, err := executeTestCmd(cfg, "-m", mPath)
	require.NoError(t, err, "test with no suites must not error")
	assert.False(t, runner.runCalled,
		"runner must NOT be called when no test suites are found")
}

// ---------------------------------------------------------------------------
// AC-12: TAP output — test names in TAP lines match report
// ---------------------------------------------------------------------------

func TestTest_TAP_TestNamesInOutput(t *testing.T) {
	dir := t.TempDir()
	mPath := writeTestManifest(t, dir, testManifestSingleTool())

	parser := &mockTestParser{
		suites: []tooltest.TestSuite{singleSuite("hello", "unique-test-name-xyz")},
	}
	runner := &mockSuiteRunner{
		reports: map[string]*tooltest.TestReport{
			"hello": {
				Tool: "hello", Total: 1, Passed: 1, Failed: 0,
				Results: []tooltest.TestResult{
					{Name: "unique-test-name-xyz", Status: "pass", Duration: 5 * time.Millisecond},
				},
			},
		},
	}
	cfg := &testConfig{Runner: runner, Parser: parser}

	stdout, _, err := executeTestCmd(cfg, "-m", mPath)
	require.NoError(t, err)

	assert.Contains(t, stdout, "unique-test-name-xyz",
		"TAP output must include the actual test name from the report")
}

// ---------------------------------------------------------------------------
// AC-12: Manifest flag is used to load toolkit
// ---------------------------------------------------------------------------

func TestTest_ManifestFlag_UsedToLoadToolkit(t *testing.T) {
	// If the manifest is invalid or missing, the command must fail even if
	// the parser has suites ready.
	parser := &mockTestParser{
		suites: []tooltest.TestSuite{singleSuite("hello", "test-one")},
	}
	runner := &mockSuiteRunner{
		reports: map[string]*tooltest.TestReport{
			"hello": passingReport("hello"),
		},
	}
	cfg := &testConfig{Runner: runner, Parser: parser}

	_, _, err := executeTestCmd(cfg, "-m", "/does/not/exist.yaml")
	require.Error(t, err, "test must fail when manifest path is invalid")
}

// ---------------------------------------------------------------------------
// AC-12: Table-driven — flag existence and defaults (constitution rule 9)
// ---------------------------------------------------------------------------

func TestNewTestCmd_FlagExistenceAndDefaults(t *testing.T) {
	tests := []struct {
		name         string
		flagName     string
		shorthand    string
		defaultValue string
	}{
		{
			name:         "manifest flag",
			flagName:     "manifest",
			shorthand:    "m",
			defaultValue: "toolwright.yaml",
		},
		{
			name:         "tests flag",
			flagName:     "tests",
			shorthand:    "t",
			defaultValue: "tests/",
		},
		{
			name:         "filter flag",
			flagName:     "filter",
			shorthand:    "f",
			defaultValue: "",
		},
		{
			name:         "parallel flag",
			flagName:     "parallel",
			shorthand:    "p",
			defaultValue: "1",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &testConfig{}
			cmd := newTestCmd(cfg)

			f := cmd.Flags().Lookup(tc.flagName)
			require.NotNil(t, f, "--%s flag must exist", tc.flagName)
			assert.Equal(t, tc.defaultValue, f.DefValue,
				"--%s flag default must be %q", tc.flagName, tc.defaultValue)

			if tc.shorthand != "" {
				sf := cmd.Flags().ShorthandLookup(tc.shorthand)
				require.NotNil(t, sf, "-%s shorthand must exist", tc.shorthand)
				assert.Equal(t, tc.flagName, sf.Name,
					"-%s must be shorthand for --%s", tc.shorthand, tc.flagName)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// AC-12: TAP failure diagnostic block
// ---------------------------------------------------------------------------

func TestTest_TAP_FailureDiagnosticBlock(t *testing.T) {
	dir := t.TempDir()
	mPath := writeTestManifest(t, dir, testManifestSingleTool())

	report := &tooltest.TestReport{
		Tool: "hello", Total: 1, Passed: 0, Failed: 1,
		Results: []tooltest.TestResult{
			{Name: "failing-test", Status: "fail", Duration: 15 * time.Millisecond, Error: "exit code mismatch"},
		},
	}
	parser := &mockTestParser{
		suites: []tooltest.TestSuite{singleSuite("hello", "failing-test")},
	}
	runner := &mockSuiteRunner{
		reports: map[string]*tooltest.TestReport{"hello": report},
	}
	cfg := &testConfig{Runner: runner, Parser: parser}

	stdout, _, _ := executeTestCmd(cfg, "-m", mPath)

	// TAP failure diagnostics should include error message.
	assert.Contains(t, stdout, "exit code mismatch",
		"TAP failure diagnostic must include the error message")
}

// ---------------------------------------------------------------------------
// AC-12: Multiple suites with mixed results
// ---------------------------------------------------------------------------

func TestTest_MultipleSuites_MixedResults_ErrorReturned(t *testing.T) {
	dir := t.TempDir()
	mPath := writeTestManifest(t, dir, testManifestMultiTool())

	parser := &mockTestParser{
		suites: []tooltest.TestSuite{
			singleSuite("alpha", "a1"),
			singleSuite("beta", "b1"),
		},
	}
	runner := &mockSuiteRunner{
		reports: map[string]*tooltest.TestReport{
			"alpha": passingReport("alpha"),
			"beta":  failingReport("beta"),
		},
	}
	cfg := &testConfig{Runner: runner, Parser: parser}

	stdout, _, err := executeTestCmd(cfg, "-m", mPath)
	require.Error(t, err,
		"test must return error when any suite has failures")
	// Verify the runner was actually called (not just failing at flag parse).
	assert.True(t, runner.runCalled,
		"runner must have been called to detect failures")
	// TAP output must show the failing test.
	assert.Contains(t, stdout, "not ok",
		"TAP must show 'not ok' for the failing suite")
}

// ---------------------------------------------------------------------------
// AC-12: JSON multi-suite output
// ---------------------------------------------------------------------------

func TestTest_JSON_MultipleSuites_OutputContainsBothTools(t *testing.T) {
	dir := t.TempDir()
	mPath := writeTestManifest(t, dir, testManifestMultiTool())

	parser := &mockTestParser{
		suites: []tooltest.TestSuite{
			singleSuite("alpha", "a1"),
			singleSuite("beta", "b1"),
		},
	}
	runner := &mockSuiteRunner{
		reports: map[string]*tooltest.TestReport{
			"alpha": {
				Tool: "alpha", Total: 1, Passed: 1, Results: []tooltest.TestResult{
					{Name: "a1", Status: "pass", Duration: 5 * time.Millisecond},
				},
			},
			"beta": {
				Tool: "beta", Total: 1, Passed: 1, Results: []tooltest.TestResult{
					{Name: "b1", Status: "pass", Duration: 5 * time.Millisecond},
				},
			},
		},
	}
	cfg := &testConfig{Runner: runner, Parser: parser}

	stdout, _, err := executeTestCmd(cfg, "-m", mPath, "--json")
	require.NoError(t, err)

	// For multiple suites, output should mention both tool names.
	assert.Contains(t, stdout, "alpha",
		"JSON output must reference alpha tool")
	assert.Contains(t, stdout, "beta",
		"JSON output must reference beta tool")
}

// ---------------------------------------------------------------------------
// Edge: filter + parallel combined
// ---------------------------------------------------------------------------

func TestTest_Filter_And_Parallel_Combined(t *testing.T) {
	dir := t.TempDir()
	mPath := writeTestManifest(t, dir, testManifestMultiTool())

	parser := &mockTestParser{
		suites: []tooltest.TestSuite{
			singleSuite("alpha", "alpha-test"),
			singleSuite("beta", "beta-test"),
		},
	}
	runner := &mockSuiteRunner{
		reports: map[string]*tooltest.TestReport{
			"alpha": {
				Tool: "alpha", Total: 1, Passed: 1, Results: []tooltest.TestResult{
					{Name: "alpha-test", Status: "pass", Duration: 5 * time.Millisecond},
				},
			},
		},
	}
	cfg := &testConfig{Runner: runner, Parser: parser}

	_, _, err := executeTestCmd(cfg, "-m", mPath, "--filter", "alpha", "--parallel", "3")
	require.NoError(t, err)

	// Only alpha should run, and it should use RunParallel.
	assert.True(t, runner.runParallelCalled,
		"RunParallel must be called with --parallel 3 and --filter")
	assert.Contains(t, runner.runParallelSuites, "alpha",
		"filtered parallel must run alpha")
	assert.NotContains(t, runner.runParallelSuites, "beta",
		"filtered parallel must NOT run beta")
	assert.Equal(t, 3, runner.parallelWorkers,
		"parallel workers must be 3")
}

// ---------------------------------------------------------------------------
// Edge: --parallel with invalid value (not a number)
// ---------------------------------------------------------------------------

func TestTest_ParallelInvalidValue_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	mPath := writeTestManifest(t, dir, testManifestSingleTool())

	parser := &mockTestParser{}
	runner := &mockSuiteRunner{}
	cfg := &testConfig{Runner: runner, Parser: parser}

	_, _, err := executeTestCmd(cfg, "-m", mPath, "--parallel", "abc")
	require.Error(t, err,
		"--parallel with non-numeric value must return an error")
	assert.False(t, runner.runCalled,
		"runner must NOT be called when --parallel value is invalid")
}

// ---------------------------------------------------------------------------
// Edge: --filter with invalid regex → error
// ---------------------------------------------------------------------------

// ---------------------------------------------------------------------------
// AC-12: Diagnostics go to stderr, not stdout
// ---------------------------------------------------------------------------

func TestTest_OutputGoesToStdout_NotStderr(t *testing.T) {
	dir := t.TempDir()
	mPath := writeTestManifest(t, dir, testManifestSingleTool())

	parser := &mockTestParser{
		suites: []tooltest.TestSuite{singleSuite("hello", "test-one")},
	}
	runner := &mockSuiteRunner{
		reports: map[string]*tooltest.TestReport{
			"hello": passingReport("hello"),
		},
	}
	cfg := &testConfig{Runner: runner, Parser: parser}

	stdout, stderr, err := executeTestCmd(cfg, "-m", mPath)
	require.NoError(t, err)

	assert.NotEmpty(t, stdout,
		"TAP output must be written to stdout")
	assert.NotContains(t, stderr, "TAP version",
		"TAP output must NOT leak into stderr")
}

// ---------------------------------------------------------------------------
// Edge: --filter with invalid regex → error
// ---------------------------------------------------------------------------

func TestTest_FilterInvalidRegex_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	mPath := writeTestManifest(t, dir, testManifestSingleTool())

	parser := &mockTestParser{
		suites: []tooltest.TestSuite{singleSuite("hello", "test-one")},
	}
	runner := &mockSuiteRunner{}
	cfg := &testConfig{Runner: runner, Parser: parser}

	_, _, err := executeTestCmd(cfg, "-m", mPath, "--filter", "[invalid")
	require.Error(t, err,
		"--filter with invalid regex must return an error")
	assert.Contains(t, err.Error(), "regex",
		"error message must mention regex parsing failure")
	assert.False(t, runner.runCalled,
		"runner must NOT be called when filter regex is invalid")
}
