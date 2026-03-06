package cli

import (
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Obsidian-Owl/toolwright/internal/auth"
	"github.com/Obsidian-Owl/toolwright/internal/runner"
	"github.com/Obsidian-Owl/toolwright/internal/tooltest"
)

// debugLineRE matches the expected debug line format: [DEBUG <RFC3339-timestamp>] <message>
// RFC3339 looks like 2026-03-06T14:05:07Z or 2026-03-06T14:05:07-05:00.
var debugLineRE = regexp.MustCompile(`\[DEBUG \d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}[^\]]*\] .+`)

// ---------------------------------------------------------------------------
// AC-5: validate --debug writes debug lines to stderr
// ---------------------------------------------------------------------------

func TestDebugValidate_WithDebug_StderrContainsDebugLines(t *testing.T) {
	dir := t.TempDir()
	ep := filepath.Join(dir, "hello.sh")
	writeExecutable(t, ep)
	path := writeManifest(t, dir, validManifestWithEntrypoint(ep))

	_, stderr, _ := executeValidateCmd("--debug", path)

	assert.Contains(t, stderr, "[DEBUG ",
		"validate --debug must produce [DEBUG ...] lines on stderr")
}

func TestDebugValidate_WithDebug_StderrContainsManifestPath(t *testing.T) {
	dir := t.TempDir()
	ep := filepath.Join(dir, "hello.sh")
	writeExecutable(t, ep)
	path := writeManifest(t, dir, validManifestWithEntrypoint(ep))

	_, stderr, _ := executeValidateCmd("--debug", path)

	assert.Contains(t, stderr, path,
		"validate --debug must log the manifest file path in debug output")
}

func TestDebugValidate_WithDebug_StdoutDoesNotContainDebug(t *testing.T) {
	dir := t.TempDir()
	ep := filepath.Join(dir, "hello.sh")
	writeExecutable(t, ep)
	path := writeManifest(t, dir, validManifestWithEntrypoint(ep))

	stdout, _, _ := executeValidateCmd("--debug", path)

	assert.NotContains(t, stdout, "[DEBUG ",
		"debug output must NEVER appear on stdout; it must only go to stderr")
}

func TestDebugValidate_WithoutDebug_NoDebugOnStderr(t *testing.T) {
	dir := t.TempDir()
	ep := filepath.Join(dir, "hello.sh")
	writeExecutable(t, ep)
	path := writeManifest(t, dir, validManifestWithEntrypoint(ep))

	_, stderr, _ := executeValidateCmd(path)

	assert.NotContains(t, stderr, "[DEBUG ",
		"without --debug, no debug output must appear on stderr")
}

func TestDebugValidate_DebugLinesMatchFormat(t *testing.T) {
	dir := t.TempDir()
	ep := filepath.Join(dir, "hello.sh")
	writeExecutable(t, ep)
	path := writeManifest(t, dir, validManifestWithEntrypoint(ep))

	_, stderr, _ := executeValidateCmd("--debug", path)

	// Every line containing [DEBUG must match the full pattern.
	lines := strings.Split(stderr, "\n")
	debugCount := 0
	for _, line := range lines {
		if strings.Contains(line, "[DEBUG") {
			debugCount++
			assert.Regexp(t, debugLineRE, line,
				"each debug line must match [DEBUG <RFC3339>] <message> pattern, got: %s", line)
		}
	}
	assert.Greater(t, debugCount, 0,
		"validate --debug must produce at least one debug line")
}

func TestDebugValidate_DebugFormat_NotOldTimestampFormat(t *testing.T) {
	// The old format was "[HH:MM:SS] msg" -- verify we are NOT using that.
	dir := t.TempDir()
	ep := filepath.Join(dir, "hello.sh")
	writeExecutable(t, ep)
	path := writeManifest(t, dir, validManifestWithEntrypoint(ep))

	_, stderr, _ := executeValidateCmd("--debug", path)

	oldFormatRE := regexp.MustCompile(`^\[\d{2}:\d{2}:\d{2}\] `)
	for _, line := range strings.Split(stderr, "\n") {
		if line == "" {
			continue
		}
		assert.False(t, oldFormatRE.MatchString(line),
			"debug output must use [DEBUG <timestamp>] format, not the old [HH:MM:SS] format; got: %s", line)
	}
}

// ---------------------------------------------------------------------------
// AC-5: validate --debug -- anti-hardcoding
// ---------------------------------------------------------------------------

func TestDebugValidate_DifferentPaths_ProduceDifferentDebugMessages(t *testing.T) {
	// First manifest.
	dir1 := t.TempDir()
	ep1 := filepath.Join(dir1, "tool-a.sh")
	writeExecutable(t, ep1)
	path1 := writeManifest(t, dir1, validManifestWithEntrypoint(ep1))

	_, stderr1, _ := executeValidateCmd("--debug", path1)

	// Second manifest with a different path.
	dir2 := t.TempDir()
	ep2 := filepath.Join(dir2, "tool-b.sh")
	writeExecutable(t, ep2)
	path2 := writeManifest(t, dir2, validManifestWithEntrypoint(ep2))

	_, stderr2, _ := executeValidateCmd("--debug", path2)

	// The debug output must differ because the manifest paths differ.
	assert.NotEqual(t, stderr1, stderr2,
		"different manifest paths must produce different debug output (anti-hardcoding)")
	assert.Contains(t, stderr1, path1,
		"first debug output must mention first manifest path")
	assert.Contains(t, stderr2, path2,
		"second debug output must mention second manifest path")
}

// ---------------------------------------------------------------------------
// AC-5/AC-6: run --debug writes debug lines to stderr
// ---------------------------------------------------------------------------

func TestDebugRun_WithDebug_StderrContainsDebugLines(t *testing.T) {
	dir := t.TempDir()
	path := writeRunManifest(t, dir, runManifestScanTool())
	mr := &mockRunner{result: &runner.Result{ExitCode: 0}}
	cfg := &runConfig{Runner: mr, Resolver: &mockResolver{}}

	_, stderr, err := executeRunCmd(cfg, "--debug", "-m", path, "scan", "./src")
	require.NoError(t, err)

	assert.Contains(t, stderr, "[DEBUG ",
		"run --debug must produce [DEBUG ...] lines on stderr")
}

func TestDebugRun_WithDebug_LogsManifestPath(t *testing.T) {
	dir := t.TempDir()
	path := writeRunManifest(t, dir, runManifestScanTool())
	mr := &mockRunner{result: &runner.Result{ExitCode: 0}}
	cfg := &runConfig{Runner: mr, Resolver: &mockResolver{}}

	_, stderr, err := executeRunCmd(cfg, "--debug", "-m", path, "scan", "./src")
	require.NoError(t, err)

	assert.Contains(t, stderr, path,
		"run --debug must log the manifest file path in debug output")
}

func TestDebugRun_WithDebug_LogsAuthResolution(t *testing.T) {
	dir := t.TempDir()
	path := writeRunManifest(t, dir, runManifestTokenTool())
	mr := &mockRunner{result: &runner.Result{ExitCode: 0}}
	resolver := &mockResolver{token: "test-token"}
	cfg := &runConfig{Runner: mr, Resolver: resolver}

	_, stderr, err := executeRunCmd(cfg, "--debug", "-m", path, "upload")
	require.NoError(t, err)

	// Debug must mention the tool name and auth type during resolution.
	assert.Contains(t, stderr, "upload",
		"run --debug must log the tool name during auth resolution")
}

func TestDebugRun_WithDebug_LogsEntrypoint(t *testing.T) {
	dir := t.TempDir()
	path := writeRunManifest(t, dir, runManifestScanTool())
	mr := &mockRunner{result: &runner.Result{ExitCode: 0, Stdout: []byte("ok")}}
	cfg := &runConfig{Runner: mr, Resolver: &mockResolver{}}

	_, stderr, err := executeRunCmd(cfg, "--debug", "-m", path, "scan", "./src")
	require.NoError(t, err)

	// Debug must log the entrypoint being executed.
	assert.Contains(t, stderr, "./scan.sh",
		"run --debug must log the tool entrypoint")
}

func TestDebugRun_WithDebug_StdoutDoesNotContainDebug(t *testing.T) {
	dir := t.TempDir()
	path := writeRunManifest(t, dir, runManifestScanTool())
	mr := &mockRunner{result: &runner.Result{ExitCode: 0, Stdout: []byte("tool output\n")}}
	cfg := &runConfig{Runner: mr, Resolver: &mockResolver{}}

	stdout, _, err := executeRunCmd(cfg, "--debug", "-m", path, "scan", "./src")
	require.NoError(t, err)

	assert.NotContains(t, stdout, "[DEBUG ",
		"debug output must NEVER appear on stdout for the run command")
}

func TestDebugRun_WithoutDebug_NoDebugOnStderr(t *testing.T) {
	dir := t.TempDir()
	path := writeRunManifest(t, dir, runManifestScanTool())
	mr := &mockRunner{result: &runner.Result{ExitCode: 0}}
	cfg := &runConfig{Runner: mr, Resolver: &mockResolver{}}

	_, stderr, err := executeRunCmd(cfg, "-m", path, "scan", "./src")
	require.NoError(t, err)

	assert.NotContains(t, stderr, "[DEBUG ",
		"without --debug, run command must not produce any debug output on stderr")
}

func TestDebugRun_DebugLinesMatchFormat(t *testing.T) {
	dir := t.TempDir()
	path := writeRunManifest(t, dir, runManifestScanTool())
	mr := &mockRunner{result: &runner.Result{ExitCode: 0}}
	cfg := &runConfig{Runner: mr, Resolver: &mockResolver{}}

	_, stderr, err := executeRunCmd(cfg, "--debug", "-m", path, "scan", "./src")
	require.NoError(t, err)

	lines := strings.Split(stderr, "\n")
	debugCount := 0
	for _, line := range lines {
		if strings.Contains(line, "[DEBUG") {
			debugCount++
			assert.Regexp(t, debugLineRE, line,
				"each debug line in run must match [DEBUG <RFC3339>] format, got: %s", line)
		}
	}
	assert.Greater(t, debugCount, 0,
		"run --debug must produce at least one debug line")
}

// ---------------------------------------------------------------------------
// AC-5/AC-6: run --debug -- anti-hardcoding with different tools
// ---------------------------------------------------------------------------

func TestDebugRun_DifferentTools_DifferentDebugMessages(t *testing.T) {
	dir := t.TempDir()
	path := writeRunManifest(t, dir, runManifestMultiTool())

	// Run "greet" tool.
	mr1 := &mockRunner{result: &runner.Result{ExitCode: 0}}
	cfg1 := &runConfig{Runner: mr1, Resolver: &mockResolver{}}
	_, stderr1, err1 := executeRunCmd(cfg1, "--debug", "-m", path, "greet", "world")
	require.NoError(t, err1)

	// Run "deploy" tool.
	mr2 := &mockRunner{result: &runner.Result{ExitCode: 0}}
	resolver2 := &mockResolver{token: "dep-tok"}
	cfg2 := &runConfig{Runner: mr2, Resolver: resolver2}
	_, stderr2, err2 := executeRunCmd(cfg2, "--debug", "-m", path, "deploy", "--env", "staging")
	require.NoError(t, err2)

	// Debug output for different tools must differ.
	assert.NotEqual(t, stderr1, stderr2,
		"debug output for different tools must differ (anti-hardcoding)")
	assert.Contains(t, stderr1, "greet",
		"debug output for greet tool must mention 'greet'")
	assert.Contains(t, stderr2, "deploy",
		"debug output for deploy tool must mention 'deploy'")
}

// ---------------------------------------------------------------------------
// AC-5: run --debug -- tool output not contaminated by debug
// ---------------------------------------------------------------------------

func TestDebugRun_ToolStderrAndDebugBothOnStderr(t *testing.T) {
	dir := t.TempDir()
	path := writeRunManifest(t, dir, runManifestScanTool())
	mr := &mockRunner{result: &runner.Result{
		ExitCode: 0,
		Stdout:   []byte("tool stdout\n"),
		Stderr:   []byte("tool warning\n"),
	}}
	cfg := &runConfig{Runner: mr, Resolver: &mockResolver{}}

	stdout, stderr, err := executeRunCmd(cfg, "--debug", "-m", path, "scan", "./src")
	require.NoError(t, err)

	// Tool stderr and debug lines coexist on stderr.
	assert.Contains(t, stderr, "tool warning",
		"tool stderr must still appear on stderr with --debug")
	assert.Contains(t, stderr, "[DEBUG ",
		"debug lines must appear on stderr alongside tool stderr")

	// But debug must not leak to stdout.
	assert.NotContains(t, stdout, "[DEBUG ",
		"debug lines must not appear on stdout")
	// Tool stdout must still appear on stdout.
	assert.Contains(t, stdout, "tool stdout",
		"tool stdout must still appear on stdout with --debug")
}

// ---------------------------------------------------------------------------
// AC-5/AC-6: test --debug writes debug lines to stderr
// ---------------------------------------------------------------------------

func TestDebugTest_WithDebug_StderrContainsDebugLines(t *testing.T) {
	dir := t.TempDir()
	path := writeTestManifest(t, dir, testManifestSingleTool())

	parser := &mockTestParser{
		suites: []tooltest.TestSuite{
			{Tool: "hello", Tests: []tooltest.TestCase{{Name: "basic"}}},
		},
	}
	sr := &mockSuiteRunner{
		reports: map[string]*tooltest.TestReport{
			"hello": {Tool: "hello", Total: 1, Passed: 1},
		},
	}
	cfg := &testConfig{Runner: sr, Parser: parser}

	_, stderr, err := executeTestCmd(cfg, "--debug", "-m", path, "-t", "tests/")
	require.NoError(t, err)

	assert.Contains(t, stderr, "[DEBUG ",
		"test --debug must produce [DEBUG ...] lines on stderr")
}

func TestDebugTest_WithDebug_LogsTestDirectory(t *testing.T) {
	dir := t.TempDir()
	path := writeTestManifest(t, dir, testManifestSingleTool())

	parser := &mockTestParser{
		suites: []tooltest.TestSuite{
			{Tool: "hello", Tests: []tooltest.TestCase{{Name: "basic"}}},
		},
	}
	sr := &mockSuiteRunner{
		reports: map[string]*tooltest.TestReport{
			"hello": {Tool: "hello", Total: 1, Passed: 1},
		},
	}
	cfg := &testConfig{Runner: sr, Parser: parser}

	testsDir := filepath.Join(dir, "my-tests")
	_, stderr, err := executeTestCmd(cfg, "--debug", "-m", path, "-t", testsDir)
	// Don't require NoError -- the parser mock returns suites regardless of dir,
	// but debug for the test directory should still be emitted.
	_ = err

	assert.Contains(t, stderr, testsDir,
		"test --debug must log the test directory path")
}

func TestDebugTest_WithDebug_StdoutDoesNotContainDebug(t *testing.T) {
	dir := t.TempDir()
	path := writeTestManifest(t, dir, testManifestSingleTool())

	parser := &mockTestParser{
		suites: []tooltest.TestSuite{
			{Tool: "hello", Tests: []tooltest.TestCase{{Name: "basic"}}},
		},
	}
	sr := &mockSuiteRunner{
		reports: map[string]*tooltest.TestReport{
			"hello": {Tool: "hello", Total: 1, Passed: 1},
		},
	}
	cfg := &testConfig{Runner: sr, Parser: parser}

	stdout, _, err := executeTestCmd(cfg, "--debug", "-m", path, "-t", "tests/")
	require.NoError(t, err)

	assert.NotContains(t, stdout, "[DEBUG ",
		"debug output must NEVER appear on stdout for the test command")
}

func TestDebugTest_WithoutDebug_NoDebugOnStderr(t *testing.T) {
	dir := t.TempDir()
	path := writeTestManifest(t, dir, testManifestSingleTool())

	parser := &mockTestParser{
		suites: []tooltest.TestSuite{
			{Tool: "hello", Tests: []tooltest.TestCase{{Name: "basic"}}},
		},
	}
	sr := &mockSuiteRunner{
		reports: map[string]*tooltest.TestReport{
			"hello": {Tool: "hello", Total: 1, Passed: 1},
		},
	}
	cfg := &testConfig{Runner: sr, Parser: parser}

	_, stderr, err := executeTestCmd(cfg, "-m", path, "-t", "tests/")
	require.NoError(t, err)

	assert.NotContains(t, stderr, "[DEBUG ",
		"without --debug, test command must not produce any debug output")
}

// ---------------------------------------------------------------------------
// AC-5/AC-6: login --debug writes debug lines to stderr
// ---------------------------------------------------------------------------

func TestDebugLogin_WithDebug_StderrContainsDebugLines(t *testing.T) {
	dir := t.TempDir()
	path := writeLoginManifest(t, dir, loginManifestOAuth())
	mock := &mockLoginFunc{token: &auth.StoredToken{AccessToken: "tok"}}
	cfg := &loginConfig{Login: mock.login}

	_, stderr, err := executeLoginCmd(cfg, "--debug", "-m", path, "deploy")
	require.NoError(t, err)

	assert.Contains(t, stderr, "[DEBUG ",
		"login --debug must produce [DEBUG ...] lines on stderr")
}

func TestDebugLogin_WithDebug_LogsManifestPath(t *testing.T) {
	dir := t.TempDir()
	path := writeLoginManifest(t, dir, loginManifestOAuth())
	mock := &mockLoginFunc{token: &auth.StoredToken{AccessToken: "tok"}}
	cfg := &loginConfig{Login: mock.login}

	_, stderr, err := executeLoginCmd(cfg, "--debug", "-m", path, "deploy")
	require.NoError(t, err)

	assert.Contains(t, stderr, path,
		"login --debug must log the manifest file path")
}

func TestDebugLogin_WithDebug_LogsAuthType(t *testing.T) {
	dir := t.TempDir()
	path := writeLoginManifest(t, dir, loginManifestOAuth())
	mock := &mockLoginFunc{token: &auth.StoredToken{AccessToken: "tok"}}
	cfg := &loginConfig{Login: mock.login}

	_, stderr, err := executeLoginCmd(cfg, "--debug", "-m", path, "deploy")
	require.NoError(t, err)

	// Debug must mention the tool name and auth type.
	assert.Contains(t, stderr, "deploy",
		"login --debug must log the tool name")
	assert.Contains(t, stderr, "oauth2",
		"login --debug must log the auth type")
}

func TestDebugLogin_WithDebug_StdoutDoesNotContainDebug(t *testing.T) {
	dir := t.TempDir()
	path := writeLoginManifest(t, dir, loginManifestOAuth())
	mock := &mockLoginFunc{token: &auth.StoredToken{AccessToken: "tok"}}
	cfg := &loginConfig{Login: mock.login}

	stdout, _, err := executeLoginCmd(cfg, "--debug", "-m", path, "deploy")
	require.NoError(t, err)

	assert.NotContains(t, stdout, "[DEBUG ",
		"debug output must NEVER appear on stdout for the login command")
}

func TestDebugLogin_WithoutDebug_NoDebugOnStderr(t *testing.T) {
	dir := t.TempDir()
	path := writeLoginManifest(t, dir, loginManifestOAuth())
	mock := &mockLoginFunc{token: &auth.StoredToken{AccessToken: "tok"}}
	cfg := &loginConfig{Login: mock.login}

	_, stderr, err := executeLoginCmd(cfg, "-m", path, "deploy")
	require.NoError(t, err)

	assert.NotContains(t, stderr, "[DEBUG ",
		"without --debug, login command must not produce any debug output")
}

// ---------------------------------------------------------------------------
// AC-6: Each command has at least one debug line (table-driven)
// ---------------------------------------------------------------------------

func TestDebug_EachCommand_HasAtLeastOneDebugLine(t *testing.T) {
	tests := []struct {
		name    string
		execute func() (stdout, stderr string, err error)
	}{
		{
			name: "validate",
			execute: func() (string, string, error) {
				dir := t.TempDir()
				ep := filepath.Join(dir, "hello.sh")
				writeExecutable(t, ep)
				path := writeManifest(t, dir, validManifestWithEntrypoint(ep))
				return executeValidateCmd("--debug", path)
			},
		},
		{
			name: "run",
			execute: func() (string, string, error) {
				dir := t.TempDir()
				path := writeRunManifest(t, dir, runManifestScanTool())
				mr := &mockRunner{result: &runner.Result{ExitCode: 0}}
				cfg := &runConfig{Runner: mr, Resolver: &mockResolver{}}
				return executeRunCmd(cfg, "--debug", "-m", path, "scan", "./src")
			},
		},
		{
			name: "test",
			execute: func() (string, string, error) {
				dir := t.TempDir()
				path := writeTestManifest(t, dir, testManifestSingleTool())
				parser := &mockTestParser{
					suites: []tooltest.TestSuite{
						{Tool: "hello", Tests: []tooltest.TestCase{{Name: "basic"}}},
					},
				}
				sr := &mockSuiteRunner{
					reports: map[string]*tooltest.TestReport{
						"hello": {Tool: "hello", Total: 1, Passed: 1},
					},
				}
				cfg := &testConfig{Runner: sr, Parser: parser}
				return executeTestCmd(cfg, "--debug", "-m", path, "-t", "tests/")
			},
		},
		{
			name: "login",
			execute: func() (string, string, error) {
				dir := t.TempDir()
				path := writeLoginManifest(t, dir, loginManifestOAuth())
				mock := &mockLoginFunc{token: &auth.StoredToken{AccessToken: "tok"}}
				cfg := &loginConfig{Login: mock.login}
				return executeLoginCmd(cfg, "--debug", "-m", path, "deploy")
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, stderr, _ := tc.execute()

			debugLines := 0
			for _, line := range strings.Split(stderr, "\n") {
				if strings.Contains(line, "[DEBUG ") {
					debugLines++
				}
			}
			assert.Greater(t, debugLines, 0,
				"%s --debug must produce at least one debug line on stderr", tc.name)
		})
	}
}

// ---------------------------------------------------------------------------
// AC-5: --debug never leaks to stdout (table-driven across all commands)
// ---------------------------------------------------------------------------

func TestDebug_NeverOnStdout_AllCommands(t *testing.T) {
	tests := []struct {
		name    string
		execute func() (stdout, stderr string, err error)
	}{
		{
			name: "validate",
			execute: func() (string, string, error) {
				dir := t.TempDir()
				ep := filepath.Join(dir, "hello.sh")
				writeExecutable(t, ep)
				path := writeManifest(t, dir, validManifestWithEntrypoint(ep))
				return executeValidateCmd("--debug", path)
			},
		},
		{
			name: "run",
			execute: func() (string, string, error) {
				dir := t.TempDir()
				path := writeRunManifest(t, dir, runManifestScanTool())
				mr := &mockRunner{result: &runner.Result{ExitCode: 0}}
				cfg := &runConfig{Runner: mr, Resolver: &mockResolver{}}
				return executeRunCmd(cfg, "--debug", "-m", path, "scan", "./src")
			},
		},
		{
			name: "test",
			execute: func() (string, string, error) {
				dir := t.TempDir()
				path := writeTestManifest(t, dir, testManifestSingleTool())
				parser := &mockTestParser{
					suites: []tooltest.TestSuite{
						{Tool: "hello", Tests: []tooltest.TestCase{{Name: "basic"}}},
					},
				}
				sr := &mockSuiteRunner{
					reports: map[string]*tooltest.TestReport{
						"hello": {Tool: "hello", Total: 1, Passed: 1},
					},
				}
				cfg := &testConfig{Runner: sr, Parser: parser}
				return executeTestCmd(cfg, "--debug", "-m", path, "-t", "tests/")
			},
		},
		{
			name: "login",
			execute: func() (string, string, error) {
				dir := t.TempDir()
				path := writeLoginManifest(t, dir, loginManifestOAuth())
				mock := &mockLoginFunc{token: &auth.StoredToken{AccessToken: "tok"}}
				cfg := &loginConfig{Login: mock.login}
				return executeLoginCmd(cfg, "--debug", "-m", path, "deploy")
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			stdout, _, _ := tc.execute()
			assert.NotContains(t, stdout, "[DEBUG ",
				"%s: debug output must NEVER appear on stdout", tc.name)
		})
	}
}

// ---------------------------------------------------------------------------
// AC-5: Without --debug, no debug output at all (table-driven)
// ---------------------------------------------------------------------------

func TestDebug_WithoutFlag_NoDebugOutput_AllCommands(t *testing.T) {
	tests := []struct {
		name    string
		execute func() (stdout, stderr string, err error)
	}{
		{
			name: "validate",
			execute: func() (string, string, error) {
				dir := t.TempDir()
				ep := filepath.Join(dir, "hello.sh")
				writeExecutable(t, ep)
				path := writeManifest(t, dir, validManifestWithEntrypoint(ep))
				return executeValidateCmd(path)
			},
		},
		{
			name: "run",
			execute: func() (string, string, error) {
				dir := t.TempDir()
				path := writeRunManifest(t, dir, runManifestScanTool())
				mr := &mockRunner{result: &runner.Result{ExitCode: 0}}
				cfg := &runConfig{Runner: mr, Resolver: &mockResolver{}}
				return executeRunCmd(cfg, "-m", path, "scan", "./src")
			},
		},
		{
			name: "test",
			execute: func() (string, string, error) {
				dir := t.TempDir()
				path := writeTestManifest(t, dir, testManifestSingleTool())
				parser := &mockTestParser{
					suites: []tooltest.TestSuite{
						{Tool: "hello", Tests: []tooltest.TestCase{{Name: "basic"}}},
					},
				}
				sr := &mockSuiteRunner{
					reports: map[string]*tooltest.TestReport{
						"hello": {Tool: "hello", Total: 1, Passed: 1},
					},
				}
				cfg := &testConfig{Runner: sr, Parser: parser}
				return executeTestCmd(cfg, "-m", path, "-t", "tests/")
			},
		},
		{
			name: "login",
			execute: func() (string, string, error) {
				dir := t.TempDir()
				path := writeLoginManifest(t, dir, loginManifestOAuth())
				mock := &mockLoginFunc{token: &auth.StoredToken{AccessToken: "tok"}}
				cfg := &loginConfig{Login: mock.login}
				return executeLoginCmd(cfg, "-m", path, "deploy")
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			stdout, stderr, _ := tc.execute()
			assert.NotContains(t, stdout, "[DEBUG ",
				"%s without --debug: stdout must not contain debug output", tc.name)
			assert.NotContains(t, stderr, "[DEBUG ",
				"%s without --debug: stderr must not contain debug output", tc.name)
		})
	}
}

// ---------------------------------------------------------------------------
// AC-6: Specific diagnostic points -- validate
// ---------------------------------------------------------------------------

func TestDebugValidate_LogsStructuralValidationCount(t *testing.T) {
	dir := t.TempDir()
	path := writeManifest(t, dir, invalidManifestMissingName())

	_, stderr, _ := executeValidateCmd("--debug", path)

	// Should log something about structural validation and how many errors.
	assert.Contains(t, stderr, "validation",
		"validate --debug must log a line about structural validation")
}

func TestDebugValidate_LogsEntrypointCheck(t *testing.T) {
	dir := t.TempDir()
	ep := filepath.Join(dir, "hello.sh")
	writeExecutable(t, ep)
	path := writeManifest(t, dir, validManifestWithEntrypoint(ep))

	_, stderr, _ := executeValidateCmd("--debug", path)

	// Should log something about checking entrypoints.
	assert.Contains(t, stderr, "entrypoint",
		"validate --debug must log a line about entrypoint checking")
}

// ---------------------------------------------------------------------------
// AC-6: Specific diagnostic points -- run
// ---------------------------------------------------------------------------

func TestDebugRun_LogsExitCode(t *testing.T) {
	dir := t.TempDir()
	path := writeRunManifest(t, dir, runManifestScanTool())
	mr := &mockRunner{result: &runner.Result{ExitCode: 42}}
	cfg := &runConfig{Runner: mr, Resolver: &mockResolver{}}

	_, stderr, _ := executeRunCmd(cfg, "--debug", "-m", path, "scan", "./src")

	assert.Contains(t, stderr, "42",
		"run --debug must log the tool exit code")
}

func TestDebugRun_AuthNone_LogsToolName(t *testing.T) {
	dir := t.TempDir()
	path := writeRunManifest(t, dir, runManifestScanTool())
	mr := &mockRunner{result: &runner.Result{ExitCode: 0}}
	cfg := &runConfig{Runner: mr, Resolver: &mockResolver{}}

	_, stderr, err := executeRunCmd(cfg, "--debug", "-m", path, "scan", "./src")
	require.NoError(t, err)

	// Even for auth:none, the debug should mention the tool.
	assert.Contains(t, stderr, "scan",
		"run --debug for auth:none must mention the tool name")
}

func TestDebugRun_AuthToken_LogsAuthType(t *testing.T) {
	dir := t.TempDir()
	path := writeRunManifest(t, dir, runManifestTokenTool())
	mr := &mockRunner{result: &runner.Result{ExitCode: 0}}
	resolver := &mockResolver{token: "tok123"}
	cfg := &runConfig{Runner: mr, Resolver: resolver}

	_, stderr, err := executeRunCmd(cfg, "--debug", "-m", path, "upload")
	require.NoError(t, err)

	assert.Contains(t, stderr, "token",
		"run --debug for auth:token must mention auth type 'token'")
}

// ---------------------------------------------------------------------------
// AC-6: Specific diagnostic points -- test
// ---------------------------------------------------------------------------

func TestDebugTest_LogsSuiteCount(t *testing.T) {
	dir := t.TempDir()
	path := writeTestManifest(t, dir, testManifestMultiTool())

	parser := &mockTestParser{
		suites: []tooltest.TestSuite{
			{Tool: "alpha", Tests: []tooltest.TestCase{{Name: "t1"}}},
			{Tool: "beta", Tests: []tooltest.TestCase{{Name: "t2"}}},
		},
	}
	sr := &mockSuiteRunner{
		reports: map[string]*tooltest.TestReport{
			"alpha": {Tool: "alpha", Total: 1, Passed: 1},
			"beta":  {Tool: "beta", Total: 1, Passed: 1},
		},
	}
	cfg := &testConfig{Runner: sr, Parser: parser}

	_, stderr, err := executeTestCmd(cfg, "--debug", "-m", path, "-t", "tests/")
	require.NoError(t, err)

	// Should log how many suites were found.
	assert.Contains(t, stderr, "2",
		"test --debug must log the number of test suites found")
}

func TestDebugTest_LogsSuiteNames(t *testing.T) {
	dir := t.TempDir()
	path := writeTestManifest(t, dir, testManifestSingleTool())

	parser := &mockTestParser{
		suites: []tooltest.TestSuite{
			{Tool: "hello", Tests: []tooltest.TestCase{{Name: "basic"}}},
		},
	}
	sr := &mockSuiteRunner{
		reports: map[string]*tooltest.TestReport{
			"hello": {Tool: "hello", Total: 1, Passed: 1},
		},
	}
	cfg := &testConfig{Runner: sr, Parser: parser}

	_, stderr, err := executeTestCmd(cfg, "--debug", "-m", path, "-t", "tests/")
	require.NoError(t, err)

	assert.Contains(t, stderr, "hello",
		"test --debug must log the suite name being run")
}

// ---------------------------------------------------------------------------
// AC-6: Specific diagnostic points -- login
// ---------------------------------------------------------------------------

func TestDebugLogin_LogsOAuthFlow(t *testing.T) {
	dir := t.TempDir()
	path := writeLoginManifest(t, dir, loginManifestOAuth())
	mock := &mockLoginFunc{token: &auth.StoredToken{AccessToken: "tok"}}
	cfg := &loginConfig{Login: mock.login}

	_, stderr, err := executeLoginCmd(cfg, "--debug", "-m", path, "deploy")
	require.NoError(t, err)

	// Should log something about starting the OAuth flow.
	assert.Contains(t, stderr, "OAuth",
		"login --debug must log a line about the OAuth flow")
}

// ---------------------------------------------------------------------------
// AC-5: run command with DisableFlagParsing -- --debug extracted correctly
// ---------------------------------------------------------------------------

func TestDebugRun_DebugFlagExtractedDespiteDisableFlagParsing(t *testing.T) {
	// The run command has DisableFlagParsing: true, so --debug must be
	// manually extracted in extractRunFlags. This test verifies that
	// --debug is correctly recognized and produces debug output.
	dir := t.TempDir()
	path := writeRunManifest(t, dir, runManifestScanTool())
	mr := &mockRunner{result: &runner.Result{ExitCode: 0}}
	cfg := &runConfig{Runner: mr, Resolver: &mockResolver{}}

	_, stderr, err := executeRunCmd(cfg, "--debug", "-m", path, "scan", "./src")
	require.NoError(t, err)

	assert.Contains(t, stderr, "[DEBUG ",
		"run --debug must work despite DisableFlagParsing; "+
			"--debug must be extracted in extractRunFlags")
}

func TestDebugRun_DebugFlagNotPassedToTool(t *testing.T) {
	// --debug is a toolwright flag, not a tool flag. It must not appear
	// in the args or flags passed to the runner.
	dir := t.TempDir()
	path := writeRunManifest(t, dir, runManifestScanTool())
	mr := &mockRunner{result: &runner.Result{ExitCode: 0}}
	cfg := &runConfig{Runner: mr, Resolver: &mockResolver{}}

	_, _, err := executeRunCmd(cfg, "--debug", "-m", path, "scan", "./src")
	require.NoError(t, err)

	// --debug must not leak into tool args.
	for _, arg := range mr.calledWith.args {
		assert.NotEqual(t, "--debug", arg,
			"--debug must not be passed as a positional arg to the tool runner")
	}
	// --debug must not leak into tool flags.
	_, hasDebugFlag := mr.calledWith.flags["debug"]
	assert.False(t, hasDebugFlag,
		"--debug must not appear in the tool flags map")
}

// ---------------------------------------------------------------------------
// AC-5: --debug combined with --json (should not interfere)
// ---------------------------------------------------------------------------

func TestDebugValidate_WithDebugAndJSON_JSONOnStdoutDebugOnStderr(t *testing.T) {
	dir := t.TempDir()
	ep := filepath.Join(dir, "hello.sh")
	writeExecutable(t, ep)
	path := writeManifest(t, dir, validManifestWithEntrypoint(ep))

	stdout, stderr, err := executeValidateCmd("--debug", "--json", path)
	require.NoError(t, err)

	// stdout must be pure JSON -- no debug contamination.
	assert.NotContains(t, stdout, "[DEBUG ",
		"with --debug --json, stdout must be pure JSON without debug lines")
	assert.True(t, len(stdout) > 0,
		"with --json, stdout must have JSON output")

	// stderr must have debug lines.
	assert.Contains(t, stderr, "[DEBUG ",
		"with --debug --json, stderr must contain debug lines")
}

func TestDebugRun_WithDebugAndJSON_JSONOnStdoutDebugOnStderr(t *testing.T) {
	dir := t.TempDir()
	path := writeRunManifest(t, dir, runManifestScanTool())
	mr := &mockRunner{result: &runner.Result{ExitCode: 0}}
	cfg := &runConfig{Runner: mr, Resolver: &mockResolver{}}

	stdout, stderr, err := executeRunCmd(cfg, "--debug", "--json", "-m", path, "scan", "./src")
	require.NoError(t, err)

	assert.NotContains(t, stdout, "[DEBUG ",
		"run --debug --json must not have debug on stdout")
	assert.Contains(t, stderr, "[DEBUG ",
		"run --debug --json must have debug on stderr")
}

// ---------------------------------------------------------------------------
// AC-6: Login anti-hardcoding -- different tools, different debug content
// ---------------------------------------------------------------------------

func TestDebugLogin_DifferentTools_DifferentDebugMessages(t *testing.T) {
	// First tool: deploy (oauth2).
	dir1 := t.TempDir()
	path1 := writeLoginManifest(t, dir1, loginManifestOAuth())
	mock1 := &mockLoginFunc{token: &auth.StoredToken{AccessToken: "tok1"}}
	cfg1 := &loginConfig{Login: mock1.login}

	_, stderr1, err1 := executeLoginCmd(cfg1, "--debug", "-m", path1, "deploy")
	require.NoError(t, err1)

	// Second tool: publish (oauth2 with different provider).
	dir2 := t.TempDir()
	path2 := writeLoginManifest(t, dir2, loginManifestOAuthAlt())
	mock2 := &mockLoginFunc{token: &auth.StoredToken{AccessToken: "tok2"}}
	cfg2 := &loginConfig{Login: mock2.login}

	_, stderr2, err2 := executeLoginCmd(cfg2, "--debug", "-m", path2, "publish")
	require.NoError(t, err2)

	assert.NotEqual(t, stderr1, stderr2,
		"debug output for different login tools must differ (anti-hardcoding)")
	assert.Contains(t, stderr1, "deploy",
		"debug output must mention 'deploy' for the deploy tool")
	assert.Contains(t, stderr2, "publish",
		"debug output must mention 'publish' for the publish tool")
}
