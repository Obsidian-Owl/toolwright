package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Obsidian-Owl/toolwright/internal/manifest"
	"github.com/Obsidian-Owl/toolwright/internal/runner"
)

// ---------------------------------------------------------------------------
// Mock types
// ---------------------------------------------------------------------------

type mockRunner struct {
	result     *runner.Result
	err        error
	calledWith struct {
		tool  manifest.Tool
		args  []string
		flags map[string]string
		token string
	}
	called bool
}

func (m *mockRunner) Run(_ context.Context, tool manifest.Tool, args []string, flags map[string]string, token string) (*runner.Result, error) {
	m.called = true
	m.calledWith.tool = tool
	m.calledWith.args = args
	m.calledWith.flags = flags
	m.calledWith.token = token
	return m.result, m.err
}

type mockResolver struct {
	token      string
	err        error
	calledWith struct {
		auth      manifest.Auth
		toolName  string
		flagValue string
	}
	called bool
}

func (m *mockResolver) Resolve(_ context.Context, auth manifest.Auth, toolName string, flagValue string) (string, error) {
	m.called = true
	m.calledWith.auth = auth
	m.calledWith.toolName = toolName
	m.calledWith.flagValue = flagValue
	return m.token, m.err
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// executeRunCmd runs the run command through the root command tree and returns
// stdout, stderr, and the error (if any).
func executeRunCmd(cfg *runConfig, args ...string) (stdout, stderr string, err error) {
	root := NewRootCommand()
	run := newRunCmd(cfg)
	root.AddCommand(run)
	outBuf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	root.SetOut(outBuf)
	root.SetErr(errBuf)
	root.SetArgs(append([]string{"run"}, args...))
	execErr := root.Execute()
	return outBuf.String(), errBuf.String(), execErr
}

// writeRunManifest writes manifest content to a temp dir and returns the path.
func writeRunManifest(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, "toolwright.yaml")
	err := os.WriteFile(path, []byte(content), 0644)
	require.NoError(t, err, "test setup: writing manifest file")
	return path
}

// runManifestScanTool returns a manifest with a "scan" tool that has auth:none,
// args, and flags suitable for testing `toolwright run scan ./src --severity high`.
func runManifestScanTool() string {
	return `apiVersion: toolwright/v1
kind: Toolkit
metadata:
  name: scanner
  version: 1.0.0
  description: Security scanner
tools:
  - name: scan
    description: Scan for vulnerabilities
    entrypoint: ./scan.sh
    auth: none
    args:
      - name: target
        type: string
        required: true
        description: Scan target path
    flags:
      - name: severity
        type: string
        required: false
        description: Minimum severity level
`
}

// runManifestTokenTool returns a manifest with an "upload" tool that requires
// token auth with TOKEN_ENV set.
func runManifestTokenTool() string {
	return `apiVersion: toolwright/v1
kind: Toolkit
metadata:
  name: uploader
  version: 1.0.0
  description: File uploader
tools:
  - name: upload
    description: Upload files
    entrypoint: ./upload.sh
    auth:
      type: token
      token_env: UPLOAD_TOKEN
`
}

// runManifestMultiTool returns a manifest with two tools: one auth:none, one auth:token.
func runManifestMultiTool() string {
	return `apiVersion: toolwright/v1
kind: Toolkit
metadata:
  name: multi
  version: 1.0.0
  description: Multi-tool toolkit
tools:
  - name: greet
    description: Greet someone
    entrypoint: ./greet.sh
    auth: none
    args:
      - name: name
        type: string
        required: true
        description: Person to greet
  - name: deploy
    description: Deploy application
    entrypoint: ./deploy.sh
    auth:
      type: token
      token_env: DEPLOY_TOKEN
    flags:
      - name: env
        type: string
        required: true
        description: Target environment
      - name: dry-run
        type: bool
        required: false
        description: Simulate deployment
`
}

// ---------------------------------------------------------------------------
// AC-10 / AC-11: Command structure
// ---------------------------------------------------------------------------

func TestNewRunCmd_ReturnsNonNil(t *testing.T) {
	cfg := &runConfig{}
	cmd := newRunCmd(cfg)
	require.NotNil(t, cmd, "newRunCmd must return a non-nil *cobra.Command")
}

func TestNewRunCmd_HasCorrectUseField(t *testing.T) {
	cfg := &runConfig{}
	cmd := newRunCmd(cfg)
	assert.Equal(t, "run <tool-name> [args...]", cmd.Use,
		"run command Use field must be 'run <tool-name> [args...]'")
}

func TestNewRunCmd_HasManifestFlag(t *testing.T) {
	cfg := &runConfig{}
	cmd := newRunCmd(cfg)
	f := cmd.Flags().Lookup("manifest")
	require.NotNil(t, f, "--manifest flag must exist on the run command")
	assert.Equal(t, "toolwright.yaml", f.DefValue,
		"--manifest flag default must be 'toolwright.yaml'")
}

func TestNewRunCmd_HasManifestShortFlag(t *testing.T) {
	cfg := &runConfig{}
	cmd := newRunCmd(cfg)
	f := cmd.Flags().ShorthandLookup("m")
	require.NotNil(t, f, "-m shorthand must exist for --manifest flag")
	assert.Equal(t, "manifest", f.Name,
		"-m must be shorthand for --manifest")
}

func TestNewRunCmd_HasTokenFlag(t *testing.T) {
	cfg := &runConfig{}
	cmd := newRunCmd(cfg)
	f := cmd.Flags().Lookup("token")
	require.NotNil(t, f, "--token flag must exist on the run command")
	assert.Equal(t, "", f.DefValue,
		"--token flag default must be empty string")
}

func TestNewRunCmd_InheritsJsonFlag(t *testing.T) {
	root := NewRootCommand()
	cfg := &runConfig{}
	run := newRunCmd(cfg)
	root.AddCommand(run)

	// Parse args so flags are initialized.
	root.SetArgs([]string{"run", "--json", "--help"})
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	_ = root.Execute()

	f := run.Flags().Lookup("json")
	require.NotNil(t, f, "--json must be accessible on the run subcommand via persistent flags")
}

// ---------------------------------------------------------------------------
// AC-10: Tool lookup
// ---------------------------------------------------------------------------

func TestRun_MissingToolName_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	path := writeRunManifest(t, dir, runManifestScanTool())
	mr := &mockRunner{result: &runner.Result{ExitCode: 0}}
	cfg := &runConfig{Runner: mr, Resolver: &mockResolver{}}

	_, _, err := executeRunCmd(cfg, "-m", path)
	require.Error(t, err, "run with no tool name must return an error")
	assert.False(t, mr.called,
		"runner must NOT be called when tool name is missing")
}

func TestRun_UnknownToolName_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	path := writeRunManifest(t, dir, runManifestScanTool())
	mr := &mockRunner{result: &runner.Result{ExitCode: 0}}
	cfg := &runConfig{Runner: mr, Resolver: &mockResolver{}}

	_, _, err := executeRunCmd(cfg, "-m", path, "nonexistent-tool")
	require.Error(t, err, "run with unknown tool name must return an error")
	assert.Contains(t, err.Error(), "nonexistent-tool",
		"error message must contain the unknown tool name")
	assert.False(t, mr.called,
		"runner must NOT be called when tool is not found")
}

func TestRun_UnknownToolName_JSON_HasToolNotFoundCode(t *testing.T) {
	dir := t.TempDir()
	path := writeRunManifest(t, dir, runManifestScanTool())
	cfg := &runConfig{Runner: &mockRunner{}, Resolver: &mockResolver{}}

	stdout, _, _ := executeRunCmd(cfg, "--json", "-m", path, "nonexistent-tool")
	require.NotEmpty(t, stdout,
		"JSON output must be produced for unknown tool error")

	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &got),
		"--json output must be valid JSON, got: %s", stdout)

	errObj, ok := got["error"].(map[string]any)
	require.True(t, ok,
		"JSON output must have top-level 'error' object, got: %v", got)
	assert.Equal(t, "tool_not_found", errObj["code"],
		"error code for unknown tool must be 'tool_not_found'")
}

// ---------------------------------------------------------------------------
// AC-10: Auth resolution — auth:none skips resolver
// ---------------------------------------------------------------------------

func TestRun_AuthNone_DoesNotCallResolver(t *testing.T) {
	dir := t.TempDir()
	path := writeRunManifest(t, dir, runManifestScanTool())
	mr := &mockRunner{result: &runner.Result{ExitCode: 0, Stdout: []byte("ok"), Stderr: []byte("")}}
	resolver := &mockResolver{token: "should-not-be-used"}
	cfg := &runConfig{Runner: mr, Resolver: resolver}

	_, _, err := executeRunCmd(cfg, "-m", path, "scan", "./src")
	require.NoError(t, err,
		"run with auth:none tool must not error")
	assert.False(t, resolver.called,
		"resolver must NOT be called when tool auth is 'none'")
	assert.True(t, mr.called,
		"runner must be called for auth:none tool")
}

func TestRun_AuthNone_PassesEmptyToken(t *testing.T) {
	dir := t.TempDir()
	path := writeRunManifest(t, dir, runManifestScanTool())
	mr := &mockRunner{result: &runner.Result{ExitCode: 0}}
	cfg := &runConfig{Runner: mr, Resolver: &mockResolver{}}

	_, _, err := executeRunCmd(cfg, "-m", path, "scan", "./src")
	require.NoError(t, err)
	assert.Equal(t, "", mr.calledWith.token,
		"token passed to runner must be empty for auth:none")
}

// ---------------------------------------------------------------------------
// AC-10: Auth resolution — auth:token with successful resolver
// ---------------------------------------------------------------------------

func TestRun_AuthToken_ResolvesAndPassesToken(t *testing.T) {
	dir := t.TempDir()
	path := writeRunManifest(t, dir, runManifestTokenTool())
	mr := &mockRunner{result: &runner.Result{ExitCode: 0}}
	resolver := &mockResolver{token: "secret-token-123"}
	cfg := &runConfig{Runner: mr, Resolver: resolver}

	_, _, err := executeRunCmd(cfg, "-m", path, "upload")
	require.NoError(t, err,
		"run with auth:token and resolved token must not error")
	assert.True(t, resolver.called,
		"resolver must be called for auth:token tool")
	assert.Equal(t, "upload", resolver.calledWith.toolName,
		"resolver must receive the tool name")
	assert.Equal(t, "secret-token-123", mr.calledWith.token,
		"runner must receive the resolved token")
}

func TestRun_AuthToken_ResolverReceivesAuthConfig(t *testing.T) {
	dir := t.TempDir()
	path := writeRunManifest(t, dir, runManifestTokenTool())
	mr := &mockRunner{result: &runner.Result{ExitCode: 0}}
	resolver := &mockResolver{token: "tok"}
	cfg := &runConfig{Runner: mr, Resolver: resolver}

	_, _, err := executeRunCmd(cfg, "-m", path, "upload")
	require.NoError(t, err)
	assert.Equal(t, "token", resolver.calledWith.auth.Type,
		"resolver must receive the correct auth type from manifest")
	assert.Equal(t, "UPLOAD_TOKEN", resolver.calledWith.auth.TokenEnv,
		"resolver must receive the correct token_env from manifest")
}

// ---------------------------------------------------------------------------
// AC-10: --token flag passes value to resolver
// ---------------------------------------------------------------------------

func TestRun_TokenFlag_PassedToResolver(t *testing.T) {
	dir := t.TempDir()
	path := writeRunManifest(t, dir, runManifestTokenTool())
	mr := &mockRunner{result: &runner.Result{ExitCode: 0}}
	resolver := &mockResolver{token: "explicit-flag-token"}
	cfg := &runConfig{Runner: mr, Resolver: resolver}

	_, _, err := executeRunCmd(cfg, "-m", path, "--token", "explicit-flag-token", "upload")
	require.NoError(t, err,
		"run with --token flag must not error")
	assert.True(t, resolver.called,
		"resolver must still be called (it handles flag priority)")
	assert.Equal(t, "explicit-flag-token", resolver.calledWith.flagValue,
		"resolver must receive the explicit --token flag value")
}

// ---------------------------------------------------------------------------
// AC-11: Auth error — clear error message
// ---------------------------------------------------------------------------

func TestRun_AuthError_ContainsToolName(t *testing.T) {
	dir := t.TempDir()
	path := writeRunManifest(t, dir, runManifestTokenTool())
	mr := &mockRunner{}
	resolver := &mockResolver{
		err: errors.New(`tool "upload" requires authentication, set UPLOAD_TOKEN or run "toolwright login upload"`),
	}
	cfg := &runConfig{Runner: mr, Resolver: resolver}

	_, _, err := executeRunCmd(cfg, "-m", path, "upload")
	require.Error(t, err,
		"run with auth error must return an error")
	assert.Contains(t, err.Error(), "upload",
		"auth error message must contain the tool name")
}

func TestRun_AuthError_ContainsTokenEnv(t *testing.T) {
	dir := t.TempDir()
	path := writeRunManifest(t, dir, runManifestTokenTool())
	resolver := &mockResolver{
		err: errors.New(`tool "upload" requires authentication, set UPLOAD_TOKEN or run "toolwright login upload"`),
	}
	cfg := &runConfig{Runner: &mockRunner{}, Resolver: resolver}

	_, _, err := executeRunCmd(cfg, "-m", path, "upload")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "UPLOAD_TOKEN",
		"auth error message must contain the TOKEN_ENV variable name")
}

func TestRun_AuthError_ContainsLoginHint(t *testing.T) {
	dir := t.TempDir()
	path := writeRunManifest(t, dir, runManifestTokenTool())
	resolver := &mockResolver{
		err: errors.New(`tool "upload" requires authentication, set UPLOAD_TOKEN or run "toolwright login upload"`),
	}
	cfg := &runConfig{Runner: &mockRunner{}, Resolver: resolver}

	_, _, err := executeRunCmd(cfg, "-m", path, "upload")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "toolwright login",
		"auth error message must contain 'toolwright login' hint")
}

func TestRun_AuthError_RunnerNotCalled(t *testing.T) {
	dir := t.TempDir()
	path := writeRunManifest(t, dir, runManifestTokenTool())
	mr := &mockRunner{}
	resolver := &mockResolver{err: fmt.Errorf("auth error")}
	cfg := &runConfig{Runner: mr, Resolver: resolver}

	_, _, _ = executeRunCmd(cfg, "-m", path, "upload")
	assert.False(t, mr.called,
		"runner must NOT be called when auth resolution fails")
}

// ---------------------------------------------------------------------------
// AC-11: Auth error with --json → JSON error with code auth_required
// ---------------------------------------------------------------------------

func TestRun_AuthError_JSON_HasAuthRequiredCode(t *testing.T) {
	dir := t.TempDir()
	path := writeRunManifest(t, dir, runManifestTokenTool())
	resolver := &mockResolver{
		err: errors.New(`tool "upload" requires authentication, set UPLOAD_TOKEN or run "toolwright login upload"`),
	}
	cfg := &runConfig{Runner: &mockRunner{}, Resolver: resolver}

	stdout, _, _ := executeRunCmd(cfg, "--json", "-m", path, "upload")
	require.NotEmpty(t, stdout,
		"JSON output must be produced for auth error")

	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &got),
		"--json output must be valid JSON, got: %s", stdout)

	errObj, ok := got["error"].(map[string]any)
	require.True(t, ok,
		"JSON output must have top-level 'error' object, got: %v", got)
	assert.Equal(t, "auth_required", errObj["code"],
		"error code for auth failure must be 'auth_required'")
}

func TestRun_AuthError_JSON_ContainsMessage(t *testing.T) {
	dir := t.TempDir()
	path := writeRunManifest(t, dir, runManifestTokenTool())
	resolver := &mockResolver{
		err: errors.New(`tool "upload" requires authentication, set UPLOAD_TOKEN or run "toolwright login upload"`),
	}
	cfg := &runConfig{Runner: &mockRunner{}, Resolver: resolver}

	stdout, _, _ := executeRunCmd(cfg, "--json", "-m", path, "upload")
	require.NotEmpty(t, stdout)

	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &got))

	errObj := got["error"].(map[string]any)
	msg, ok := errObj["message"].(string)
	require.True(t, ok, "error.message must be a string")
	assert.Contains(t, msg, "upload",
		"JSON error message must contain tool name")
	assert.Contains(t, msg, "authentication",
		"JSON error message must mention authentication")
}

// ---------------------------------------------------------------------------
// AC-10: Execution — runner called with correct tool and args
// ---------------------------------------------------------------------------

func TestRun_ExecutorReceivesCorrectTool(t *testing.T) {
	dir := t.TempDir()
	path := writeRunManifest(t, dir, runManifestScanTool())
	mr := &mockRunner{result: &runner.Result{ExitCode: 0}}
	cfg := &runConfig{Runner: mr, Resolver: &mockResolver{}}

	_, _, err := executeRunCmd(cfg, "-m", path, "scan", "./src")
	require.NoError(t, err)
	assert.True(t, mr.called, "runner must be called")
	assert.Equal(t, "scan", mr.calledWith.tool.Name,
		"runner must receive the tool with correct name")
	assert.Equal(t, "./scan.sh", mr.calledWith.tool.Entrypoint,
		"runner must receive the tool with correct entrypoint")
}

func TestRun_ExecutorReceivesPositionalArgs(t *testing.T) {
	dir := t.TempDir()
	path := writeRunManifest(t, dir, runManifestScanTool())
	mr := &mockRunner{result: &runner.Result{ExitCode: 0}}
	cfg := &runConfig{Runner: mr, Resolver: &mockResolver{}}

	_, _, err := executeRunCmd(cfg, "-m", path, "scan", "./src", "./lib")
	require.NoError(t, err)
	assert.Contains(t, mr.calledWith.args, "./src",
		"runner must receive './src' as a positional arg")
	assert.Contains(t, mr.calledWith.args, "./lib",
		"runner must receive './lib' as a positional arg")
}

func TestRun_ExecutorReceivesFlags(t *testing.T) {
	dir := t.TempDir()
	path := writeRunManifest(t, dir, runManifestScanTool())
	mr := &mockRunner{result: &runner.Result{ExitCode: 0}}
	cfg := &runConfig{Runner: mr, Resolver: &mockResolver{}}

	_, _, err := executeRunCmd(cfg, "-m", path, "scan", "./src", "--severity", "high")
	require.NoError(t, err)
	require.NotNil(t, mr.calledWith.flags,
		"runner must receive a non-nil flags map")
	assert.Equal(t, "high", mr.calledWith.flags["severity"],
		"runner must receive severity=high in flags map")
}

// ---------------------------------------------------------------------------
// AC-10: Exit code propagation
// ---------------------------------------------------------------------------

func TestRun_ExitCode0_Success(t *testing.T) {
	dir := t.TempDir()
	path := writeRunManifest(t, dir, runManifestScanTool())
	mr := &mockRunner{result: &runner.Result{ExitCode: 0}}
	cfg := &runConfig{Runner: mr, Resolver: &mockResolver{}}

	_, _, err := executeRunCmd(cfg, "-m", path, "scan", "./src")
	assert.NoError(t, err,
		"run must succeed when tool exits with code 0")
}

func TestRun_ExitCode1_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	path := writeRunManifest(t, dir, runManifestScanTool())
	mr := &mockRunner{result: &runner.Result{ExitCode: 1}}
	cfg := &runConfig{Runner: mr, Resolver: &mockResolver{}}

	_, _, err := executeRunCmd(cfg, "-m", path, "scan", "./src")
	require.Error(t, err,
		"run must return an error when tool exits with code 1")
}

func TestRun_ExitCode42_Preserved(t *testing.T) {
	dir := t.TempDir()
	path := writeRunManifest(t, dir, runManifestScanTool())
	mr := &mockRunner{result: &runner.Result{ExitCode: 42}}
	cfg := &runConfig{Runner: mr, Resolver: &mockResolver{}}

	_, _, err := executeRunCmd(cfg, "-m", path, "scan", "./src")
	require.Error(t, err,
		"run must return an error when tool exits with non-zero code")
	assert.Contains(t, err.Error(), "42",
		"error message must contain the exit code")
}

// ---------------------------------------------------------------------------
// AC-10: Stdout/stderr forwarding
// ---------------------------------------------------------------------------

func TestRun_Stdout_Forwarded(t *testing.T) {
	dir := t.TempDir()
	path := writeRunManifest(t, dir, runManifestScanTool())
	mr := &mockRunner{result: &runner.Result{
		ExitCode: 0,
		Stdout:   []byte("scan results: no vulnerabilities found\n"),
		Stderr:   []byte(""),
	}}
	cfg := &runConfig{Runner: mr, Resolver: &mockResolver{}}

	stdout, _, err := executeRunCmd(cfg, "-m", path, "scan", "./src")
	require.NoError(t, err)
	assert.Contains(t, stdout, "scan results: no vulnerabilities found",
		"tool stdout must be forwarded to Toolwright's stdout")
}

func TestRun_Stderr_Forwarded(t *testing.T) {
	dir := t.TempDir()
	path := writeRunManifest(t, dir, runManifestScanTool())
	mr := &mockRunner{result: &runner.Result{
		ExitCode: 0,
		Stdout:   []byte(""),
		Stderr:   []byte("warning: slow scan detected\n"),
	}}
	cfg := &runConfig{Runner: mr, Resolver: &mockResolver{}}

	_, stderr, err := executeRunCmd(cfg, "-m", path, "scan", "./src")
	require.NoError(t, err)
	assert.Contains(t, stderr, "warning: slow scan detected",
		"tool stderr must be forwarded to Toolwright's stderr")
}

func TestRun_Stdout_ContentIsFromTool_NotHardcoded(t *testing.T) {
	dir := t.TempDir()
	path := writeRunManifest(t, dir, runManifestScanTool())

	// First run with one output.
	mr1 := &mockRunner{result: &runner.Result{
		ExitCode: 0,
		Stdout:   []byte("output-alpha"),
	}}
	cfg1 := &runConfig{Runner: mr1, Resolver: &mockResolver{}}
	stdout1, _, err1 := executeRunCmd(cfg1, "-m", path, "scan", "./src")
	require.NoError(t, err1)

	// Second run with different output.
	mr2 := &mockRunner{result: &runner.Result{
		ExitCode: 0,
		Stdout:   []byte("output-beta"),
	}}
	cfg2 := &runConfig{Runner: mr2, Resolver: &mockResolver{}}
	stdout2, _, err2 := executeRunCmd(cfg2, "-m", path, "scan", "./src")
	require.NoError(t, err2)

	assert.NotEqual(t, stdout1, stdout2,
		"different tool outputs must produce different Toolwright stdout; anti-hardcoding")
}

// ---------------------------------------------------------------------------
// AC-10: Runner execution failure (not exit code, but true failure)
// ---------------------------------------------------------------------------

func TestRun_RunnerError_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	path := writeRunManifest(t, dir, runManifestScanTool())
	mr := &mockRunner{err: fmt.Errorf("tool execution failed: entrypoint not found")}
	cfg := &runConfig{Runner: mr, Resolver: &mockResolver{}}

	_, _, err := executeRunCmd(cfg, "-m", path, "scan", "./src")
	require.Error(t, err,
		"run must return an error when runner returns an error")
	assert.Contains(t, err.Error(), "execution",
		"error message must describe the execution failure")
}

// ---------------------------------------------------------------------------
// AC-10: Flag forwarding — extra args parsed correctly
// ---------------------------------------------------------------------------

func TestRun_ExtraArgsAfterToolName_PassedAsPositional(t *testing.T) {
	dir := t.TempDir()
	path := writeRunManifest(t, dir, runManifestScanTool())
	mr := &mockRunner{result: &runner.Result{ExitCode: 0}}
	cfg := &runConfig{Runner: mr, Resolver: &mockResolver{}}

	_, _, err := executeRunCmd(cfg, "-m", path, "scan", "arg1", "arg2", "arg3")
	require.NoError(t, err)
	assert.Equal(t, []string{"arg1", "arg2", "arg3"}, mr.calledWith.args,
		"all args after tool name must be passed to runner as positional args")
}

func TestRun_FlagsAfterToolName_PassedAsFlags(t *testing.T) {
	dir := t.TempDir()
	path := writeRunManifest(t, dir, runManifestMultiTool())
	mr := &mockRunner{result: &runner.Result{ExitCode: 0}}
	resolver := &mockResolver{token: "deploy-token"}
	cfg := &runConfig{Runner: mr, Resolver: resolver}

	_, _, err := executeRunCmd(cfg, "-m", path, "deploy", "--env", "production", "--dry-run", "true")
	require.NoError(t, err)
	require.NotNil(t, mr.calledWith.flags)
	assert.Equal(t, "production", mr.calledWith.flags["env"],
		"--env flag must be parsed and passed in flags map")
}

func TestRun_MixedArgsAndFlags_CorrectSplit(t *testing.T) {
	dir := t.TempDir()
	path := writeRunManifest(t, dir, runManifestScanTool())
	mr := &mockRunner{result: &runner.Result{ExitCode: 0}}
	cfg := &runConfig{Runner: mr, Resolver: &mockResolver{}}

	_, _, err := executeRunCmd(cfg, "-m", path, "scan", "./src", "--severity", "critical")
	require.NoError(t, err)
	assert.Contains(t, mr.calledWith.args, "./src",
		"positional arg './src' must be in args")
	assert.Equal(t, "critical", mr.calledWith.flags["severity"],
		"--severity must be in flags map")
}

// ---------------------------------------------------------------------------
// AC-10/AC-11: Auth type variations (table-driven per constitution rule 9)
// ---------------------------------------------------------------------------

func TestRun_AuthTypes_TableDriven(t *testing.T) {
	tests := []struct {
		name             string
		manifest         string
		toolName         string
		resolverToken    string
		resolverErr      error
		wantResolverCall bool
		wantRunnerCall   bool
		wantError        bool
	}{
		{
			name:             "auth:none → no resolver, runner called",
			manifest:         runManifestScanTool(),
			toolName:         "scan",
			resolverToken:    "",
			resolverErr:      nil,
			wantResolverCall: false,
			wantRunnerCall:   true,
			wantError:        false,
		},
		{
			name:             "auth:token, resolved → runner called with token",
			manifest:         runManifestTokenTool(),
			toolName:         "upload",
			resolverToken:    "abc123",
			resolverErr:      nil,
			wantResolverCall: true,
			wantRunnerCall:   true,
			wantError:        false,
		},
		{
			name:             "auth:token, resolver fails → error, no runner",
			manifest:         runManifestTokenTool(),
			toolName:         "upload",
			resolverToken:    "",
			resolverErr:      fmt.Errorf("auth failed"),
			wantResolverCall: true,
			wantRunnerCall:   false,
			wantError:        true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := writeRunManifest(t, dir, tc.manifest)
			mr := &mockRunner{result: &runner.Result{ExitCode: 0}}
			resolver := &mockResolver{token: tc.resolverToken, err: tc.resolverErr}
			cfg := &runConfig{Runner: mr, Resolver: resolver}

			_, _, err := executeRunCmd(cfg, "-m", path, tc.toolName)
			if tc.wantError {
				require.Error(t, err, "expected error")
			} else {
				require.NoError(t, err, "expected no error")
			}

			assert.Equal(t, tc.wantResolverCall, resolver.called,
				"resolver.called mismatch")
			assert.Equal(t, tc.wantRunnerCall, mr.called,
				"runner.called mismatch")
		})
	}
}

// ---------------------------------------------------------------------------
// Anti-hardcoding: different tools produce different behavior
// ---------------------------------------------------------------------------

func TestRun_DifferentTools_DifferentRunnerCalls(t *testing.T) {
	dir := t.TempDir()
	path := writeRunManifest(t, dir, runManifestMultiTool())

	// Run "greet" tool (auth:none).
	mr1 := &mockRunner{result: &runner.Result{ExitCode: 0}}
	cfg1 := &runConfig{Runner: mr1, Resolver: &mockResolver{}}
	_, _, err1 := executeRunCmd(cfg1, "-m", path, "greet", "world")
	require.NoError(t, err1)

	// Run "deploy" tool (auth:token).
	mr2 := &mockRunner{result: &runner.Result{ExitCode: 0}}
	resolver2 := &mockResolver{token: "dep-tok"}
	cfg2 := &runConfig{Runner: mr2, Resolver: resolver2}
	_, _, err2 := executeRunCmd(cfg2, "-m", path, "deploy", "--env", "staging")
	require.NoError(t, err2)

	assert.Equal(t, "greet", mr1.calledWith.tool.Name,
		"first call must be for 'greet' tool")
	assert.Equal(t, "./greet.sh", mr1.calledWith.tool.Entrypoint,
		"first call must use greet's entrypoint")

	assert.Equal(t, "deploy", mr2.calledWith.tool.Name,
		"second call must be for 'deploy' tool")
	assert.Equal(t, "./deploy.sh", mr2.calledWith.tool.Entrypoint,
		"second call must use deploy's entrypoint")
}

// ---------------------------------------------------------------------------
// AC-10: Manifest loading errors
// ---------------------------------------------------------------------------

func TestRun_ManifestNotFound_ReturnsError(t *testing.T) {
	cfg := &runConfig{Runner: &mockRunner{}, Resolver: &mockResolver{}}

	_, _, err := executeRunCmd(cfg, "-m", "/nonexistent/path/toolwright.yaml", "scan")
	require.Error(t, err,
		"run with nonexistent manifest must return an error")
}

func TestRun_ManifestNotFound_JSON_HasError(t *testing.T) {
	cfg := &runConfig{Runner: &mockRunner{}, Resolver: &mockResolver{}}

	stdout, _, _ := executeRunCmd(cfg, "--json", "-m", "/nonexistent/path/toolwright.yaml", "scan")
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
// AC-10: Default manifest path
// ---------------------------------------------------------------------------

func TestRun_DefaultManifestPath_UsesToolwrightYaml(t *testing.T) {
	dir := t.TempDir()
	writeRunManifest(t, dir, runManifestScanTool())

	original, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(original) })

	mr := &mockRunner{result: &runner.Result{ExitCode: 0}}
	cfg := &runConfig{Runner: mr, Resolver: &mockResolver{}}

	_, _, err = executeRunCmd(cfg, "scan", "./src")
	require.NoError(t, err,
		"run with no -m flag must default to toolwright.yaml in current dir")
	assert.True(t, mr.called,
		"runner must be called when default manifest is found")
	assert.Equal(t, "scan", mr.calledWith.tool.Name,
		"runner must receive the correct tool from default manifest")
}

// ---------------------------------------------------------------------------
// AC-10: Toolkit-level auth inherited by tool
// ---------------------------------------------------------------------------

func TestRun_ToolkitLevelAuth_InheritedByTool(t *testing.T) {
	manifest := `apiVersion: toolwright/v1
kind: Toolkit
metadata:
  name: inherited-auth
  version: 1.0.0
  description: Test toolkit-level auth
auth:
  type: token
  token_env: GLOBAL_TOKEN
tools:
  - name: fetcher
    description: Fetch data
    entrypoint: ./fetch.sh
`
	dir := t.TempDir()
	path := writeRunManifest(t, dir, manifest)
	mr := &mockRunner{result: &runner.Result{ExitCode: 0}}
	resolver := &mockResolver{token: "inherited-tok"}
	cfg := &runConfig{Runner: mr, Resolver: resolver}

	_, _, err := executeRunCmd(cfg, "-m", path, "fetcher")
	require.NoError(t, err)
	assert.True(t, resolver.called,
		"resolver must be called for tool inheriting toolkit-level token auth")
	assert.Equal(t, "token", resolver.calledWith.auth.Type,
		"resolver must receive inherited auth type 'token'")
	assert.Equal(t, "GLOBAL_TOKEN", resolver.calledWith.auth.TokenEnv,
		"resolver must receive inherited token_env")
}

// ---------------------------------------------------------------------------
// AC-10: Tool-level auth overrides toolkit-level auth
// ---------------------------------------------------------------------------

func TestRun_ToolLevelAuth_OverridesToolkitAuth(t *testing.T) {
	manifest := `apiVersion: toolwright/v1
kind: Toolkit
metadata:
  name: override-auth
  version: 1.0.0
  description: Tool overrides toolkit auth
auth:
  type: oauth2
  provider_url: https://example.com
tools:
  - name: uploader
    description: Upload stuff
    entrypoint: ./upload.sh
    auth:
      type: token
      token_env: UPLOAD_TOKEN
`
	dir := t.TempDir()
	path := writeRunManifest(t, dir, manifest)
	mr := &mockRunner{result: &runner.Result{ExitCode: 0}}
	resolver := &mockResolver{token: "tool-tok"}
	cfg := &runConfig{Runner: mr, Resolver: resolver}

	_, _, err := executeRunCmd(cfg, "-m", path, "uploader")
	require.NoError(t, err)
	assert.Equal(t, "token", resolver.calledWith.auth.Type,
		"resolver must receive tool-level auth 'token', not toolkit-level 'oauth2'")
	assert.Equal(t, "UPLOAD_TOKEN", resolver.calledWith.auth.TokenEnv,
		"resolver must receive tool-level token_env")
}

// ---------------------------------------------------------------------------
// AC-10: Runner receives no args when none given
// ---------------------------------------------------------------------------

func TestRun_NoExtraArgs_RunnerReceivesEmptyArgs(t *testing.T) {
	dir := t.TempDir()
	path := writeRunManifest(t, dir, runManifestScanTool())
	mr := &mockRunner{result: &runner.Result{ExitCode: 0}}
	cfg := &runConfig{Runner: mr, Resolver: &mockResolver{}}

	_, _, err := executeRunCmd(cfg, "-m", path, "scan")
	require.NoError(t, err)
	assert.Empty(t, mr.calledWith.args,
		"runner must receive empty args when no extra args given")
}

// ---------------------------------------------------------------------------
// AC-10: Execution duration is not lost (the result includes it)
// ---------------------------------------------------------------------------

func TestRun_ResultDuration_NotZero(t *testing.T) {
	dir := t.TempDir()
	path := writeRunManifest(t, dir, runManifestScanTool())
	mr := &mockRunner{result: &runner.Result{
		ExitCode: 0,
		Duration: 42 * time.Millisecond,
	}}
	cfg := &runConfig{Runner: mr, Resolver: &mockResolver{}}

	_, _, err := executeRunCmd(cfg, "-m", path, "scan", "./src")
	require.NoError(t, err,
		"run must succeed; duration is informational")
	// This test verifies the runner is called, which is the prerequisite
	// for duration to be captured.
	assert.True(t, mr.called, "runner must be called")
}

// ---------------------------------------------------------------------------
// AC-11: Auth error message format for different tools (anti-hardcoding)
// ---------------------------------------------------------------------------

func TestRun_AuthError_MessageFormat_DifferentTools(t *testing.T) {
	tests := []struct {
		name        string
		manifest    string
		toolName    string
		wantInError []string
	}{
		{
			name:     "upload tool",
			manifest: runManifestTokenTool(),
			toolName: "upload",
			wantInError: []string{
				"upload",
				"UPLOAD_TOKEN",
				"toolwright login",
			},
		},
		{
			name:     "deploy tool",
			manifest: runManifestMultiTool(),
			toolName: "deploy",
			wantInError: []string{
				"deploy",
				"DEPLOY_TOKEN",
				"toolwright login",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := writeRunManifest(t, dir, tc.manifest)
			resolverErr := fmt.Errorf(
				`tool %q requires authentication. Set %s or run "toolwright login %s".`, //nolint:revive // mock error matching AC-11 message format
				tc.toolName, tc.wantInError[1], tc.toolName,
			)
			resolver := &mockResolver{err: resolverErr}
			cfg := &runConfig{Runner: &mockRunner{}, Resolver: resolver}

			_, _, err := executeRunCmd(cfg, "-m", path, tc.toolName)
			require.Error(t, err)
			for _, want := range tc.wantInError {
				assert.Contains(t, err.Error(), want,
					"auth error for tool %q must contain %q", tc.toolName, want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// AC-10: Both stdout and stderr forwarded simultaneously
// ---------------------------------------------------------------------------

func TestRun_StdoutAndStderr_BothForwarded(t *testing.T) {
	dir := t.TempDir()
	path := writeRunManifest(t, dir, runManifestScanTool())
	mr := &mockRunner{result: &runner.Result{
		ExitCode: 0,
		Stdout:   []byte("results here"),
		Stderr:   []byte("diagnostics here"),
	}}
	cfg := &runConfig{Runner: mr, Resolver: &mockResolver{}}

	stdout, stderr, err := executeRunCmd(cfg, "-m", path, "scan", "./src")
	require.NoError(t, err)
	assert.Contains(t, stdout, "results here",
		"tool stdout must appear in Toolwright stdout")
	assert.Contains(t, stderr, "diagnostics here",
		"tool stderr must appear in Toolwright stderr")
	// Cross-contamination check.
	assert.NotContains(t, stdout, "diagnostics here",
		"stderr content must NOT leak into stdout")
	assert.NotContains(t, stderr, "results here",
		"stdout content must NOT leak into stderr")
}

// ---------------------------------------------------------------------------
// Edge case: empty stdout/stderr from runner
// ---------------------------------------------------------------------------

func TestRun_EmptyOutput_NoExtraOutput(t *testing.T) {
	dir := t.TempDir()
	path := writeRunManifest(t, dir, runManifestScanTool())
	mr := &mockRunner{result: &runner.Result{
		ExitCode: 0,
		Stdout:   []byte{},
		Stderr:   []byte{},
	}}
	cfg := &runConfig{Runner: mr, Resolver: &mockResolver{}}

	stdout, stderr, err := executeRunCmd(cfg, "-m", path, "scan", "./src")
	require.NoError(t, err)
	assert.Empty(t, stdout,
		"stdout must be empty when tool produces no stdout")
	assert.Empty(t, stderr,
		"stderr must be empty when tool produces no stderr")
}

// ---------------------------------------------------------------------------
// Edge case: nil result from runner (should not happen, but defense-in-depth)
// ---------------------------------------------------------------------------

func TestRun_NilResult_WithError_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	path := writeRunManifest(t, dir, runManifestScanTool())
	mr := &mockRunner{result: nil, err: fmt.Errorf("catastrophic failure")}
	cfg := &runConfig{Runner: mr, Resolver: &mockResolver{}}

	_, _, err := executeRunCmd(cfg, "-m", path, "scan", "./src")
	require.Error(t, err,
		"run must return an error when runner returns nil result with error")
}
