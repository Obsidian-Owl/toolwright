package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Obsidian-Owl/toolwright/internal/auth"
	"github.com/Obsidian-Owl/toolwright/internal/runner"
)

// ---------------------------------------------------------------------------
// AC-2: Missing positional arg on `init` returns UsageError (exit 2)
// ---------------------------------------------------------------------------

func TestExitCode_Init_MissingProjectName_ReturnsUsageError(t *testing.T) {
	scaf := &mockScaffolder{result: defaultScaffoldResult("x")}
	cfg := &initConfig{Scaffolder: scaf, Wizard: &mockWizard{}}

	_, err := executeInitCmd(cfg, "--yes")
	require.Error(t, err,
		"init with no project name must return an error")

	var usageErr *UsageError
	assert.True(t, errors.As(err, &usageErr),
		"init missing project name must return a *UsageError; got %T: %v", err, err)
}

func TestExitCode_Init_MissingProjectName_ExitCodeIs2(t *testing.T) {
	scaf := &mockScaffolder{result: defaultScaffoldResult("x")}
	cfg := &initConfig{Scaffolder: scaf, Wizard: &mockWizard{}}

	_, err := executeInitCmd(cfg, "--yes")
	require.Error(t, err)

	assert.Equal(t, ExitUsage, ExitCodeForError(err),
		"init missing project name must map to exit code 2 (ExitUsage)")
}

// ---------------------------------------------------------------------------
// AC-2: Missing positional arg on `run` returns UsageError (exit 2)
// ---------------------------------------------------------------------------

func TestExitCode_Run_MissingToolName_ReturnsUsageError(t *testing.T) {
	dir := t.TempDir()
	path := writeRunManifest(t, dir, runManifestScanTool())
	cfg := &runConfig{Runner: &mockRunner{result: &runner.Result{ExitCode: 0}}, Resolver: &mockResolver{}}

	_, _, err := executeRunCmd(cfg, "-m", path)
	require.Error(t, err,
		"run with no tool name must return an error")

	var usageErr *UsageError
	assert.True(t, errors.As(err, &usageErr),
		"run missing tool name must return a *UsageError; got %T: %v", err, err)
}

func TestExitCode_Run_MissingToolName_ExitCodeIs2(t *testing.T) {
	cfg := &runConfig{Runner: &mockRunner{result: &runner.Result{ExitCode: 0}}, Resolver: &mockResolver{}}

	_, _, err := executeRunCmd(cfg)
	require.Error(t, err)

	assert.Equal(t, ExitUsage, ExitCodeForError(err),
		"run missing tool name must map to exit code 2 (ExitUsage)")
}

// ---------------------------------------------------------------------------
// AC-2: Missing positional arg on `login` returns UsageError (exit 2)
// ---------------------------------------------------------------------------

func TestExitCode_Login_MissingToolName_ReturnsUsageError(t *testing.T) {
	dir := t.TempDir()
	path := writeLoginManifest(t, dir, loginManifestOAuth())
	mock := &mockLoginFunc{token: &auth.StoredToken{AccessToken: "tok"}}
	cfg := &loginConfig{Login: mock.login}

	_, _, err := executeLoginCmd(cfg, "-m", path)
	require.Error(t, err,
		"login with no tool name must return an error")

	var usageErr *UsageError
	assert.True(t, errors.As(err, &usageErr),
		"login missing tool name must return a *UsageError; got %T: %v", err, err)
}

func TestExitCode_Login_MissingToolName_ExitCodeIs2(t *testing.T) {
	dir := t.TempDir()
	path := writeLoginManifest(t, dir, loginManifestOAuth())
	mock := &mockLoginFunc{token: &auth.StoredToken{AccessToken: "tok"}}
	cfg := &loginConfig{Login: mock.login}

	_, _, err := executeLoginCmd(cfg, "-m", path)
	require.Error(t, err)

	assert.Equal(t, ExitUsage, ExitCodeForError(err),
		"login missing tool name must map to exit code 2 (ExitUsage)")
}

// ---------------------------------------------------------------------------
// AC-2: Root command with no args returns UsageError (exit 2)
// ---------------------------------------------------------------------------

func TestExitCode_Root_NoArgs_ReturnsUsageError(t *testing.T) {
	root := NewRootCommand()
	root.SetArgs([]string{})
	root.SetOut(&nullWriter{})
	root.SetErr(&nullWriter{})

	err := root.Execute()
	require.Error(t, err,
		"root with no args must return an error")

	var usageErr *UsageError
	assert.True(t, errors.As(err, &usageErr),
		"root with no args must return a *UsageError; got %T: %v", err, err)
}

func TestExitCode_Root_UnknownCommand_ReturnsUsageError(t *testing.T) {
	root := NewRootCommand()
	root.SetArgs([]string{"nonexistent-command"})
	root.SetOut(&nullWriter{})
	root.SetErr(&nullWriter{})

	err := root.Execute()
	require.Error(t, err,
		"root with unknown command must return an error")

	var usageErr *UsageError
	assert.True(t, errors.As(err, &usageErr),
		"root with unknown command must return a *UsageError; got %T: %v", err, err)
}

// ---------------------------------------------------------------------------
// AC-3: Missing manifest file returns IOError (exit 3)
// ---------------------------------------------------------------------------

func TestExitCode_Run_ManifestNotFound_ReturnsIOError(t *testing.T) {
	cfg := &runConfig{Runner: &mockRunner{}, Resolver: &mockResolver{}}

	_, _, err := executeRunCmd(cfg, "-m", "/nonexistent/path/toolwright.yaml", "scan")
	require.Error(t, err,
		"run with missing manifest must return an error")

	var ioErr *IOError
	assert.True(t, errors.As(err, &ioErr),
		"run with missing manifest must return a *IOError; got %T: %v", err, err)
}

func TestExitCode_Run_ManifestNotFound_ExitCodeIs3(t *testing.T) {
	cfg := &runConfig{Runner: &mockRunner{}, Resolver: &mockResolver{}}

	_, _, err := executeRunCmd(cfg, "-m", "/nonexistent/path/toolwright.yaml", "scan")
	require.Error(t, err)

	assert.Equal(t, ExitIO, ExitCodeForError(err),
		"run with missing manifest must map to exit code 3 (ExitIO)")
}

func TestExitCode_Login_ManifestNotFound_ReturnsIOError(t *testing.T) {
	mock := &mockLoginFunc{}
	cfg := &loginConfig{Login: mock.login}

	_, _, err := executeLoginCmd(cfg, "-m", "/nonexistent/path/toolwright.yaml", "deploy")
	require.Error(t, err,
		"login with missing manifest must return an error")

	var ioErr *IOError
	assert.True(t, errors.As(err, &ioErr),
		"login with missing manifest must return a *IOError; got %T: %v", err, err)
}

func TestExitCode_Login_ManifestNotFound_ExitCodeIs3(t *testing.T) {
	mock := &mockLoginFunc{}
	cfg := &loginConfig{Login: mock.login}

	_, _, err := executeLoginCmd(cfg, "-m", "/nonexistent/path/toolwright.yaml", "deploy")
	require.Error(t, err)

	assert.Equal(t, ExitIO, ExitCodeForError(err),
		"login with missing manifest must map to exit code 3 (ExitIO)")
}

// ---------------------------------------------------------------------------
// AC-3: Manifest file permission denied returns IOError (exit 3)
// ---------------------------------------------------------------------------

func TestExitCode_Run_ManifestPermissionDenied_ReturnsIOError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "toolwright.yaml")
	err := os.WriteFile(path, []byte("apiVersion: toolwright/v1\nkind: Toolkit\nmetadata:\n  name: test\n  version: 1.0.0\n  description: test\ntools: []\n"), 0000)
	require.NoError(t, err, "test setup: writing manifest file")
	t.Cleanup(func() {
		// Restore permissions so t.TempDir() cleanup can remove it.
		_ = os.Chmod(path, 0644)
	})

	cfg := &runConfig{Runner: &mockRunner{}, Resolver: &mockResolver{}}
	_, _, runErr := executeRunCmd(cfg, "-m", path, "scan")
	require.Error(t, runErr,
		"run with permission-denied manifest must return an error")

	var ioErr *IOError
	assert.True(t, errors.As(runErr, &ioErr),
		"run with permission-denied manifest must return a *IOError; got %T: %v", runErr, runErr)
}

func TestExitCode_Run_ManifestPermissionDenied_ExitCodeIs3(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "toolwright.yaml")
	err := os.WriteFile(path, []byte("apiVersion: toolwright/v1\nkind: Toolkit\nmetadata:\n  name: test\n  version: 1.0.0\n  description: test\ntools: []\n"), 0000)
	require.NoError(t, err, "test setup: writing manifest file")
	t.Cleanup(func() {
		_ = os.Chmod(path, 0644)
	})

	cfg := &runConfig{Runner: &mockRunner{}, Resolver: &mockResolver{}}
	_, _, runErr := executeRunCmd(cfg, "-m", path, "scan")
	require.Error(t, runErr)

	assert.Equal(t, ExitIO, ExitCodeForError(runErr),
		"run with permission-denied manifest must map to exit code 3 (ExitIO)")
}

// ---------------------------------------------------------------------------
// AC-3: loadManifest directly returns IOError for file-not-found
// ---------------------------------------------------------------------------

func TestExitCode_LoadManifest_FileNotFound_ReturnsIOError(t *testing.T) {
	_, err := loadManifest("/nonexistent/path/toolwright.yaml")
	require.Error(t, err,
		"loadManifest with nonexistent path must return an error")

	var ioErr *IOError
	assert.True(t, errors.As(err, &ioErr),
		"loadManifest with nonexistent file must return a *IOError; got %T: %v", err, err)
}

func TestExitCode_LoadManifest_PermissionDenied_ReturnsIOError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "toolwright.yaml")
	err := os.WriteFile(path, []byte("apiVersion: toolwright/v1\nkind: Toolkit\nmetadata:\n  name: test\n  version: 1.0.0\n  description: test\ntools: []\n"), 0000)
	require.NoError(t, err, "test setup: writing manifest file")
	t.Cleanup(func() {
		_ = os.Chmod(path, 0644)
	})

	_, loadErr := loadManifest(path)
	require.Error(t, loadErr,
		"loadManifest with permission-denied file must return an error")

	var ioErr *IOError
	assert.True(t, errors.As(loadErr, &ioErr),
		"loadManifest with permission-denied file must return a *IOError; got %T: %v", loadErr, loadErr)
}

// ---------------------------------------------------------------------------
// AC-4: Tool not found returns plain error, NOT UsageError or IOError (exit 1)
// ---------------------------------------------------------------------------

func TestExitCode_Run_ToolNotFound_ReturnsPlainError(t *testing.T) {
	dir := t.TempDir()
	path := writeRunManifest(t, dir, runManifestScanTool())
	cfg := &runConfig{Runner: &mockRunner{result: &runner.Result{ExitCode: 0}}, Resolver: &mockResolver{}}

	_, _, err := executeRunCmd(cfg, "-m", path, "nonexistent-tool")
	require.Error(t, err,
		"run with unknown tool must return an error")

	var usageErr *UsageError
	assert.False(t, errors.As(err, &usageErr),
		"tool-not-found must NOT be a UsageError (it is a validation failure, exit 1)")

	var ioErr *IOError
	assert.False(t, errors.As(err, &ioErr),
		"tool-not-found must NOT be an IOError (it is a validation failure, exit 1)")

	assert.Equal(t, ExitError, ExitCodeForError(err),
		"tool-not-found must map to exit code 1 (ExitError)")
}

func TestExitCode_Login_ToolNotFound_ReturnsPlainError(t *testing.T) {
	dir := t.TempDir()
	path := writeLoginManifest(t, dir, loginManifestOAuth())
	mock := &mockLoginFunc{}
	cfg := &loginConfig{Login: mock.login}

	_, _, err := executeLoginCmd(cfg, "-m", path, "nonexistent-tool")
	require.Error(t, err,
		"login with unknown tool must return an error")

	var usageErr *UsageError
	assert.False(t, errors.As(err, &usageErr),
		"tool-not-found in login must NOT be a UsageError")

	var ioErr *IOError
	assert.False(t, errors.As(err, &ioErr),
		"tool-not-found in login must NOT be an IOError")

	assert.Equal(t, ExitError, ExitCodeForError(err),
		"tool-not-found in login must map to exit code 1 (ExitError)")
}

// ---------------------------------------------------------------------------
// AC-4: Auth resolution failure returns plain error (exit 1)
// ---------------------------------------------------------------------------

func TestExitCode_Run_AuthFailure_ReturnsPlainError(t *testing.T) {
	dir := t.TempDir()
	path := writeRunManifest(t, dir, runManifestTokenTool())
	resolver := &mockResolver{err: fmt.Errorf("auth resolution failed")}
	cfg := &runConfig{Runner: &mockRunner{}, Resolver: resolver}

	_, _, err := executeRunCmd(cfg, "-m", path, "upload")
	require.Error(t, err,
		"run with auth failure must return an error")

	var usageErr *UsageError
	assert.False(t, errors.As(err, &usageErr),
		"auth failure must NOT be a UsageError")

	var ioErr *IOError
	assert.False(t, errors.As(err, &ioErr),
		"auth failure must NOT be an IOError")

	assert.Equal(t, ExitError, ExitCodeForError(err),
		"auth failure must map to exit code 1 (ExitError)")
}

// ---------------------------------------------------------------------------
// AC-4: Validation failure (invalid runtime) returns plain error (exit 1)
// ---------------------------------------------------------------------------

func TestExitCode_Init_InvalidRuntime_ReturnsPlainError(t *testing.T) {
	scaf := &mockScaffolder{result: defaultScaffoldResult("proj")}
	cfg := &initConfig{Scaffolder: scaf, Wizard: &mockWizard{}}

	_, err := executeInitCmd(cfg, "proj", "--yes", "--runtime", "ruby")
	require.Error(t, err,
		"init with invalid runtime must return an error")

	var usageErr *UsageError
	assert.False(t, errors.As(err, &usageErr),
		"invalid runtime must NOT be a UsageError (it is a validation failure)")

	var ioErr *IOError
	assert.False(t, errors.As(err, &ioErr),
		"invalid runtime must NOT be an IOError")

	assert.Equal(t, ExitError, ExitCodeForError(err),
		"invalid runtime must map to exit code 1 (ExitError)")
}

// ---------------------------------------------------------------------------
// AC-4: Login auth type mismatch (none, token) returns plain error (exit 1)
// ---------------------------------------------------------------------------

func TestExitCode_Login_AuthNone_ReturnsPlainError(t *testing.T) {
	dir := t.TempDir()
	path := writeLoginManifest(t, dir, loginManifestNone())
	mock := &mockLoginFunc{}
	cfg := &loginConfig{Login: mock.login}

	_, _, err := executeLoginCmd(cfg, "-m", path, "greet")
	require.Error(t, err)

	var usageErr *UsageError
	assert.False(t, errors.As(err, &usageErr),
		"login auth:none rejection must NOT be a UsageError")

	var ioErr *IOError
	assert.False(t, errors.As(err, &ioErr),
		"login auth:none rejection must NOT be an IOError")

	assert.Equal(t, ExitError, ExitCodeForError(err),
		"login auth:none rejection must map to exit code 1 (ExitError)")
}

func TestExitCode_Login_AuthToken_ReturnsPlainError(t *testing.T) {
	dir := t.TempDir()
	path := writeLoginManifest(t, dir, loginManifestToken())
	mock := &mockLoginFunc{}
	cfg := &loginConfig{Login: mock.login}

	_, _, err := executeLoginCmd(cfg, "-m", path, "upload")
	require.Error(t, err)

	var usageErr *UsageError
	assert.False(t, errors.As(err, &usageErr),
		"login auth:token rejection must NOT be a UsageError")

	var ioErr *IOError
	assert.False(t, errors.As(err, &ioErr),
		"login auth:token rejection must NOT be an IOError")

	assert.Equal(t, ExitError, ExitCodeForError(err),
		"login auth:token rejection must map to exit code 1 (ExitError)")
}

// ---------------------------------------------------------------------------
// AC-2: UsageError message is preserved through wrapping (not lost)
// ---------------------------------------------------------------------------

func TestExitCode_Init_MissingProjectName_ErrorMessage_Preserved(t *testing.T) {
	scaf := &mockScaffolder{result: defaultScaffoldResult("x")}
	cfg := &initConfig{Scaffolder: scaf, Wizard: &mockWizard{}}

	_, err := executeInitCmd(cfg, "--yes")
	require.Error(t, err)

	// The error message must still be useful and mention what's wrong.
	assert.Contains(t, err.Error(), "project name",
		"error message must mention 'project name' even when wrapped as UsageError")
}

func TestExitCode_Run_MissingToolName_ErrorMessage_Preserved(t *testing.T) {
	cfg := &runConfig{Runner: &mockRunner{result: &runner.Result{ExitCode: 0}}, Resolver: &mockResolver{}}

	_, _, err := executeRunCmd(cfg)
	require.Error(t, err)

	assert.Contains(t, err.Error(), "tool name",
		"error message must mention 'tool name' even when wrapped as UsageError")
}

func TestExitCode_Login_MissingToolName_ErrorMessage_Preserved(t *testing.T) {
	dir := t.TempDir()
	path := writeLoginManifest(t, dir, loginManifestOAuth())
	mock := &mockLoginFunc{token: &auth.StoredToken{AccessToken: "tok"}}
	cfg := &loginConfig{Login: mock.login}

	_, _, err := executeLoginCmd(cfg, "-m", path)
	require.Error(t, err)

	assert.Contains(t, err.Error(), "tool name",
		"error message must mention 'tool name' even when wrapped as UsageError")
}

// ---------------------------------------------------------------------------
// AC-3: IOError message includes file path for debugging
// ---------------------------------------------------------------------------

func TestExitCode_LoadManifest_IOError_ContainsFilePath(t *testing.T) {
	path := "/nonexistent/deeply/nested/toolwright.yaml"
	_, err := loadManifest(path)
	require.Error(t, err)

	assert.Contains(t, err.Error(), path,
		"IOError message must contain the file path for user debugging")
}

// ---------------------------------------------------------------------------
// Discrimination tests: exact boundary between error types
// ---------------------------------------------------------------------------

func TestExitCode_Discrimination_TableDriven(t *testing.T) {
	// Build manifests and configs for various scenarios.
	dir := t.TempDir()
	scanPath := writeRunManifest(t, dir, runManifestScanTool())

	dir2 := t.TempDir()
	tokenPath := writeRunManifest(t, dir2, runManifestTokenTool())

	dir3 := t.TempDir()
	oauthPath := writeLoginManifest(t, dir3, loginManifestOAuth())

	tests := []struct {
		name         string
		wantExitCode int
		runTest      func(t *testing.T) error
	}{
		{
			name:         "run: missing tool name -> exit 2 (usage)",
			wantExitCode: ExitUsage,
			runTest: func(t *testing.T) error {
				t.Helper()
				cfg := &runConfig{Runner: &mockRunner{result: &runner.Result{ExitCode: 0}}, Resolver: &mockResolver{}}
				_, _, err := executeRunCmd(cfg, "-m", scanPath)
				return err
			},
		},
		{
			name:         "run: missing manifest -> exit 3 (IO)",
			wantExitCode: ExitIO,
			runTest: func(t *testing.T) error {
				t.Helper()
				cfg := &runConfig{Runner: &mockRunner{}, Resolver: &mockResolver{}}
				_, _, err := executeRunCmd(cfg, "-m", "/no/such/file.yaml", "scan")
				return err
			},
		},
		{
			name:         "run: tool not found -> exit 1 (general)",
			wantExitCode: ExitError,
			runTest: func(t *testing.T) error {
				t.Helper()
				cfg := &runConfig{Runner: &mockRunner{result: &runner.Result{ExitCode: 0}}, Resolver: &mockResolver{}}
				_, _, err := executeRunCmd(cfg, "-m", scanPath, "nonexistent")
				return err
			},
		},
		{
			name:         "run: auth failure -> exit 1 (general)",
			wantExitCode: ExitError,
			runTest: func(t *testing.T) error {
				t.Helper()
				resolver := &mockResolver{err: fmt.Errorf("no token")}
				cfg := &runConfig{Runner: &mockRunner{}, Resolver: resolver}
				_, _, err := executeRunCmd(cfg, "-m", tokenPath, "upload")
				return err
			},
		},
		{
			name:         "login: missing tool name -> exit 2 (usage)",
			wantExitCode: ExitUsage,
			runTest: func(t *testing.T) error {
				t.Helper()
				mock := &mockLoginFunc{token: &auth.StoredToken{AccessToken: "tok"}}
				cfg := &loginConfig{Login: mock.login}
				_, _, err := executeLoginCmd(cfg, "-m", oauthPath)
				return err
			},
		},
		{
			name:         "login: missing manifest -> exit 3 (IO)",
			wantExitCode: ExitIO,
			runTest: func(t *testing.T) error {
				t.Helper()
				mock := &mockLoginFunc{}
				cfg := &loginConfig{Login: mock.login}
				_, _, err := executeLoginCmd(cfg, "-m", "/no/such/file.yaml", "deploy")
				return err
			},
		},
		{
			name:         "login: tool not found -> exit 1 (general)",
			wantExitCode: ExitError,
			runTest: func(t *testing.T) error {
				t.Helper()
				mock := &mockLoginFunc{}
				cfg := &loginConfig{Login: mock.login}
				_, _, err := executeLoginCmd(cfg, "-m", oauthPath, "nonexistent")
				return err
			},
		},
		{
			name:         "init: missing project name -> exit 2 (usage)",
			wantExitCode: ExitUsage,
			runTest: func(t *testing.T) error {
				t.Helper()
				scaf := &mockScaffolder{result: defaultScaffoldResult("x")}
				cfg := &initConfig{Scaffolder: scaf, Wizard: &mockWizard{}}
				_, err := executeInitCmd(cfg, "--yes")
				return err
			},
		},
		{
			name:         "init: invalid runtime -> exit 1 (general)",
			wantExitCode: ExitError,
			runTest: func(t *testing.T) error {
				t.Helper()
				scaf := &mockScaffolder{result: defaultScaffoldResult("proj")}
				cfg := &initConfig{Scaffolder: scaf, Wizard: &mockWizard{}}
				_, err := executeInitCmd(cfg, "proj", "--yes", "--runtime", "ruby")
				return err
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.runTest(t)
			require.Error(t, err,
				"test %q must produce an error", tc.name)
			assert.Equal(t, tc.wantExitCode, ExitCodeForError(err),
				"test %q must map to exit code %d", tc.name, tc.wantExitCode)
		})
	}
}

// ---------------------------------------------------------------------------
// nullWriter is a no-op io.Writer for suppressing command output in tests.
// ---------------------------------------------------------------------------

type nullWriter struct{}

func (nullWriter) Write(p []byte) (int, error) { return len(p), nil }
