package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Obsidian-Owl/toolwright/internal/scaffold"
	"github.com/Obsidian-Owl/toolwright/internal/tui"
)

// ---------------------------------------------------------------------------
// Mock types
// ---------------------------------------------------------------------------

type mockScaffolder struct {
	called     bool
	calledWith scaffold.ScaffoldOptions
	result     *scaffold.ScaffoldResult
	err        error
}

func (m *mockScaffolder) Scaffold(_ context.Context, opts scaffold.ScaffoldOptions) (*scaffold.ScaffoldResult, error) {
	m.called = true
	m.calledWith = opts
	if m.err != nil {
		return nil, m.err
	}
	return m.result, nil
}

type mockWizard struct {
	called bool
	result *tui.WizardResult
	err    error
}

func (m *mockWizard) Run(_ context.Context) (*tui.WizardResult, error) {
	m.called = true
	if m.err != nil {
		return nil, m.err
	}
	return m.result, nil
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// executeInitCmd runs the init command through the root command tree and
// returns stdout and the error (if any).
func executeInitCmd(cfg *initConfig, args ...string) (stdout string, err error) {
	root := NewRootCommand()
	initCmd := newInitCmd(cfg)
	root.AddCommand(initCmd)
	outBuf := &bytes.Buffer{}
	root.SetOut(outBuf)
	root.SetErr(&bytes.Buffer{})
	root.SetArgs(append([]string{"init"}, args...))
	execErr := root.Execute()
	return outBuf.String(), execErr
}

// defaultScaffoldResult returns a scaffold result suitable for most tests.
func defaultScaffoldResult(name string) *scaffold.ScaffoldResult {
	return &scaffold.ScaffoldResult{
		Dir: name,
		Files: []string{
			name + "/toolwright.yaml",
			name + "/tools/hello.sh",
			name + "/schema/hello.json",
			name + "/tests/hello_test.yaml",
		},
	}
}

// ---------------------------------------------------------------------------
// AC-16: Command structure
// ---------------------------------------------------------------------------

func TestNewInitCmd_ReturnsNonNil(t *testing.T) {
	cfg := &initConfig{}
	cmd := newInitCmd(cfg)
	require.NotNil(t, cmd, "newInitCmd must return a non-nil *cobra.Command")
}

func TestNewInitCmd_HasCorrectUseField(t *testing.T) {
	cfg := &initConfig{}
	cmd := newInitCmd(cfg)
	assert.Contains(t, cmd.Use, "init",
		"init command Use field must contain 'init'")
}

func TestNewInitCmd_HasShortDescription(t *testing.T) {
	cfg := &initConfig{}
	cmd := newInitCmd(cfg)
	assert.NotEmpty(t, cmd.Short,
		"init command must have a Short description")
}

func TestNewInitCmd_HasYesFlag(t *testing.T) {
	cfg := &initConfig{}
	cmd := newInitCmd(cfg)
	f := cmd.Flags().Lookup("yes")
	require.NotNil(t, f, "--yes flag must exist on the init command")
	assert.Equal(t, "bool", f.Value.Type(),
		"--yes flag must be a boolean")
	assert.Equal(t, "false", f.DefValue,
		"--yes flag default must be false")
}

func TestNewInitCmd_HasYesShortFlag(t *testing.T) {
	cfg := &initConfig{}
	cmd := newInitCmd(cfg)
	f := cmd.Flags().ShorthandLookup("y")
	require.NotNil(t, f, "-y shorthand must exist for --yes flag")
	assert.Equal(t, "yes", f.Name,
		"-y must be shorthand for --yes")
}

func TestNewInitCmd_HasRuntimeFlag(t *testing.T) {
	cfg := &initConfig{}
	cmd := newInitCmd(cfg)
	f := cmd.Flags().Lookup("runtime")
	require.NotNil(t, f, "--runtime flag must exist on the init command")
	assert.Equal(t, "shell", f.DefValue,
		"--runtime flag default must be 'shell'")
}

func TestNewInitCmd_HasRuntimeShortFlag(t *testing.T) {
	cfg := &initConfig{}
	cmd := newInitCmd(cfg)
	f := cmd.Flags().ShorthandLookup("r")
	require.NotNil(t, f, "-r shorthand must exist for --runtime flag")
	assert.Equal(t, "runtime", f.Name,
		"-r must be shorthand for --runtime")
}

func TestNewInitCmd_HasDescriptionFlag(t *testing.T) {
	cfg := &initConfig{}
	cmd := newInitCmd(cfg)
	f := cmd.Flags().Lookup("description")
	require.NotNil(t, f, "--description flag must exist on the init command")
}

func TestNewInitCmd_HasDescriptionShortFlag(t *testing.T) {
	cfg := &initConfig{}
	cmd := newInitCmd(cfg)
	f := cmd.Flags().ShorthandLookup("d")
	require.NotNil(t, f, "-d shorthand must exist for --description flag")
	assert.Equal(t, "description", f.Name,
		"-d must be shorthand for --description")
}

func TestNewInitCmd_HasOutputFlag(t *testing.T) {
	cfg := &initConfig{}
	cmd := newInitCmd(cfg)
	f := cmd.Flags().Lookup("output")
	require.NotNil(t, f, "--output flag must exist on the init command")
}

func TestNewInitCmd_HasOutputShortFlag(t *testing.T) {
	cfg := &initConfig{}
	cmd := newInitCmd(cfg)
	f := cmd.Flags().ShorthandLookup("o")
	require.NotNil(t, f, "-o shorthand must exist for --output flag")
	assert.Equal(t, "output", f.Name,
		"-o must be shorthand for --output")
}

func TestNewInitCmd_InheritsJsonFlag(t *testing.T) {
	root := NewRootCommand()
	cfg := &initConfig{}
	init := newInitCmd(cfg)
	root.AddCommand(init)

	root.SetArgs([]string{"init", "--json", "--help"})
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	_ = root.Execute()

	f := init.Flags().Lookup("json")
	require.NotNil(t, f, "--json must be accessible on the init subcommand via persistent flags")
}

// ---------------------------------------------------------------------------
// AC-16: Missing project name
// ---------------------------------------------------------------------------

func TestInit_MissingProjectName_ReturnsError(t *testing.T) {
	scaf := &mockScaffolder{result: defaultScaffoldResult("x")}
	wiz := &mockWizard{}
	cfg := &initConfig{Scaffolder: scaf, Wizard: wiz}

	_, err := executeInitCmd(cfg, "--yes")
	require.Error(t, err,
		"init with no project name must return an error")
	assert.NotContains(t, err.Error(), "unknown command",
		"error must be from the init command itself, not from cobra failing to find the command")
	assert.False(t, scaf.called,
		"scaffolder must NOT be called when project name is missing")
	assert.False(t, wiz.called,
		"wizard must NOT be called when project name is missing")
}

func TestInit_MissingProjectName_JSON_HasStructuredError(t *testing.T) {
	scaf := &mockScaffolder{result: defaultScaffoldResult("x")}
	wiz := &mockWizard{}
	cfg := &initConfig{Scaffolder: scaf, Wizard: wiz}

	stdout, _ := executeInitCmd(cfg, "--json", "--yes")
	require.NotEmpty(t, stdout,
		"JSON output must be produced for missing project name error")

	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &got),
		"--json output must be valid JSON, got: %s", stdout)

	errObj, ok := got["error"].(map[string]any)
	require.True(t, ok,
		"JSON output must have top-level 'error' object, got: %v", got)
	assert.Contains(t, errObj, "code",
		"error object must have 'code' field")
	assert.Contains(t, errObj, "message",
		"error object must have 'message' field")
}

// ---------------------------------------------------------------------------
// AC-16: Non-interactive mode (--yes) — defaults
// ---------------------------------------------------------------------------

func TestInit_Yes_CallsScaffolderWithCorrectDefaults(t *testing.T) {
	scaf := &mockScaffolder{result: defaultScaffoldResult("my-tool")}
	wiz := &mockWizard{}
	cfg := &initConfig{Scaffolder: scaf, Wizard: wiz}

	_, err := executeInitCmd(cfg, "my-tool", "--yes")
	require.NoError(t, err,
		"init --yes with valid name must succeed")
	require.True(t, scaf.called,
		"scaffolder must be called in non-interactive mode")

	assert.Equal(t, "my-tool", scaf.calledWith.Name,
		"scaffolder must receive the project name from the positional arg")
	assert.Equal(t, "shell", scaf.calledWith.Runtime,
		"scaffolder must receive default runtime 'shell' when --runtime is not specified")
	assert.Equal(t, "none", scaf.calledWith.Auth,
		"scaffolder must receive default auth 'none' when no auth flag is provided")
}

func TestInit_Yes_WizardNotCalled(t *testing.T) {
	scaf := &mockScaffolder{result: defaultScaffoldResult("my-tool")}
	wiz := &mockWizard{result: &tui.WizardResult{}}
	cfg := &initConfig{Scaffolder: scaf, Wizard: wiz}

	_, err := executeInitCmd(cfg, "my-tool", "--yes")
	require.NoError(t, err)
	assert.False(t, wiz.called,
		"wizard must NOT be called when --yes is set")
}

func TestInit_Yes_DefaultDescription(t *testing.T) {
	scaf := &mockScaffolder{result: defaultScaffoldResult("my-tool")}
	cfg := &initConfig{Scaffolder: scaf, Wizard: &mockWizard{}}

	_, err := executeInitCmd(cfg, "my-tool", "--yes")
	require.NoError(t, err)
	require.True(t, scaf.called)

	// When no --description is provided, a reasonable default must be set.
	assert.NotEmpty(t, scaf.calledWith.Description,
		"scaffolder must receive a non-empty default description when --description is omitted")
	assert.Contains(t, scaf.calledWith.Description, "my-tool",
		"default description should reference the project name to be meaningful")
}

// ---------------------------------------------------------------------------
// AC-16: Non-interactive mode — runtime flag variants (table-driven)
// ---------------------------------------------------------------------------

func TestInit_Yes_RuntimeVariants_TableDriven(t *testing.T) {
	tests := []struct {
		name        string
		runtime     string
		wantRuntime string
	}{
		{name: "go runtime", runtime: "go", wantRuntime: "go"},
		{name: "python runtime", runtime: "python", wantRuntime: "python"},
		{name: "typescript runtime", runtime: "typescript", wantRuntime: "typescript"},
		{name: "shell runtime (explicit)", runtime: "shell", wantRuntime: "shell"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			scaf := &mockScaffolder{result: defaultScaffoldResult("test-proj")}
			cfg := &initConfig{Scaffolder: scaf, Wizard: &mockWizard{}}

			_, err := executeInitCmd(cfg, "test-proj", "--yes", "--runtime", tc.runtime)
			require.NoError(t, err)
			require.True(t, scaf.called)
			assert.Equal(t, tc.wantRuntime, scaf.calledWith.Runtime,
				"scaffolder must receive runtime %q from --runtime flag", tc.wantRuntime)
		})
	}
}

// ---------------------------------------------------------------------------
// AC-16: Non-interactive mode — description flag
// ---------------------------------------------------------------------------

func TestInit_Yes_DescriptionFlag(t *testing.T) {
	scaf := &mockScaffolder{result: defaultScaffoldResult("my-tool")}
	cfg := &initConfig{Scaffolder: scaf, Wizard: &mockWizard{}}

	_, err := executeInitCmd(cfg, "my-tool", "--yes", "--description", "My awesome tool")
	require.NoError(t, err)
	require.True(t, scaf.called)
	assert.Equal(t, "My awesome tool", scaf.calledWith.Description,
		"scaffolder must receive the exact description from --description flag")
}

// ---------------------------------------------------------------------------
// AC-16: Non-interactive mode — output flag
// ---------------------------------------------------------------------------

func TestInit_Yes_OutputFlag(t *testing.T) {
	outDir := t.TempDir()
	scaf := &mockScaffolder{result: defaultScaffoldResult("my-tool")}
	cfg := &initConfig{Scaffolder: scaf, Wizard: &mockWizard{}}

	_, err := executeInitCmd(cfg, "my-tool", "--yes", "--output", outDir)
	require.NoError(t, err)
	require.True(t, scaf.called)
	assert.Equal(t, outDir, scaf.calledWith.OutputDir,
		"scaffolder must receive the output directory from --output flag")
}

// ---------------------------------------------------------------------------
// AC-16: Non-interactive mode — short flags
// ---------------------------------------------------------------------------

func TestInit_ShortFlags(t *testing.T) {
	scaf := &mockScaffolder{result: defaultScaffoldResult("proj")}
	cfg := &initConfig{Scaffolder: scaf, Wizard: &mockWizard{}}

	_, err := executeInitCmd(cfg, "proj", "-y", "-r", "go", "-d", "desc")
	require.NoError(t, err)
	require.True(t, scaf.called)
	assert.Equal(t, "go", scaf.calledWith.Runtime,
		"-r must work as shorthand for --runtime")
	assert.Equal(t, "desc", scaf.calledWith.Description,
		"-d must work as shorthand for --description")
}

// ---------------------------------------------------------------------------
// AC-17: Interactive mode (no --yes) — wizard called
// ---------------------------------------------------------------------------

func TestInit_NoYes_WizardCalled(t *testing.T) {
	// Unset CI so interactive mode is not suppressed.
	t.Setenv("CI", "")
	os.Unsetenv("CI")

	scaf := &mockScaffolder{result: defaultScaffoldResult("my-tool")}
	wiz := &mockWizard{result: &tui.WizardResult{
		Description: "From wizard",
		Runtime:     "go",
		Auth:        "token",
	}}
	cfg := &initConfig{Scaffolder: scaf, Wizard: wiz}

	_, err := executeInitCmd(cfg, "my-tool")
	require.NoError(t, err)
	assert.True(t, wiz.called,
		"wizard must be called when --yes is NOT set and CI is not active")
}

func TestInit_NoYes_WizardResultUsedForScaffolder(t *testing.T) {
	t.Setenv("CI", "")
	os.Unsetenv("CI")

	scaf := &mockScaffolder{result: defaultScaffoldResult("my-tool")}
	wiz := &mockWizard{result: &tui.WizardResult{
		Description: "Wizard description",
		Runtime:     "python",
		Auth:        "oauth2",
	}}
	cfg := &initConfig{Scaffolder: scaf, Wizard: wiz}

	_, err := executeInitCmd(cfg, "my-tool")
	require.NoError(t, err)
	require.True(t, scaf.called,
		"scaffolder must be called after wizard completes")

	// The project name comes from the positional arg, not the wizard.
	assert.Equal(t, "my-tool", scaf.calledWith.Name,
		"scaffolder Name must come from the positional arg, not the wizard")
	// Description, Runtime, Auth come from the wizard.
	assert.Equal(t, "Wizard description", scaf.calledWith.Description,
		"scaffolder must receive description from wizard result")
	assert.Equal(t, "python", scaf.calledWith.Runtime,
		"scaffolder must receive runtime from wizard result")
	assert.Equal(t, "oauth2", scaf.calledWith.Auth,
		"scaffolder must receive auth from wizard result")
}

func TestInit_NoYes_WizardError_Propagates(t *testing.T) {
	t.Setenv("CI", "")
	os.Unsetenv("CI")

	scaf := &mockScaffolder{result: defaultScaffoldResult("my-tool")}
	wiz := &mockWizard{err: fmt.Errorf("user cancelled wizard")}
	cfg := &initConfig{Scaffolder: scaf, Wizard: wiz}

	_, err := executeInitCmd(cfg, "my-tool")
	require.Error(t, err,
		"wizard error must propagate as command error")
	assert.Contains(t, err.Error(), "cancel",
		"error message must describe what went wrong")
	assert.False(t, scaf.called,
		"scaffolder must NOT be called when wizard fails")
}

func TestInit_NoYes_ScaffolderNotCalledBeforeWizard(t *testing.T) {
	// Ensure the scaffolder is only called AFTER wizard returns.
	t.Setenv("CI", "")
	os.Unsetenv("CI")

	wiz := &mockWizard{err: fmt.Errorf("wizard failed early")}
	scaf := &mockScaffolder{result: defaultScaffoldResult("my-tool")}
	cfg := &initConfig{Scaffolder: scaf, Wizard: wiz}

	_, _ = executeInitCmd(cfg, "my-tool")
	assert.True(t, wiz.called,
		"wizard must be called in interactive mode")
	assert.False(t, scaf.called,
		"scaffolder must not be called if wizard errors out")
}

// ---------------------------------------------------------------------------
// AC-17: CI detection — CI=true falls back to non-interactive
// ---------------------------------------------------------------------------

func TestInit_CITrue_WizardNotCalled(t *testing.T) {
	t.Setenv("CI", "true")

	scaf := &mockScaffolder{result: defaultScaffoldResult("my-tool")}
	wiz := &mockWizard{result: &tui.WizardResult{
		Description: "should not use this",
		Runtime:     "typescript",
		Auth:        "oauth2",
	}}
	cfg := &initConfig{Scaffolder: scaf, Wizard: wiz}

	_, err := executeInitCmd(cfg, "my-tool")
	require.NoError(t, err)
	assert.False(t, wiz.called,
		"wizard must NOT be called when CI=true, even without --yes")
	assert.True(t, scaf.called,
		"scaffolder must be called with defaults when CI=true suppresses wizard")
}

func TestInit_CI1_WizardNotCalled(t *testing.T) {
	t.Setenv("CI", "1")

	scaf := &mockScaffolder{result: defaultScaffoldResult("my-tool")}
	wiz := &mockWizard{result: &tui.WizardResult{
		Description: "should not use this",
		Runtime:     "go",
		Auth:        "token",
	}}
	cfg := &initConfig{Scaffolder: scaf, Wizard: wiz}

	_, err := executeInitCmd(cfg, "my-tool")
	require.NoError(t, err)
	assert.False(t, wiz.called,
		"wizard must NOT be called when CI=1, even without --yes")
	assert.True(t, scaf.called,
		"scaffolder must be called with defaults when CI=1 suppresses wizard")
}

func TestInit_CITrue_UsesDefaults(t *testing.T) {
	// When CI suppresses the wizard, defaults should be used (same as --yes).
	t.Setenv("CI", "true")

	scaf := &mockScaffolder{result: defaultScaffoldResult("ci-proj")}
	cfg := &initConfig{Scaffolder: scaf, Wizard: &mockWizard{}}

	_, err := executeInitCmd(cfg, "ci-proj")
	require.NoError(t, err)
	require.True(t, scaf.called)
	assert.Equal(t, "ci-proj", scaf.calledWith.Name,
		"scaffolder name must come from positional arg even in CI mode")
	assert.Equal(t, "shell", scaf.calledWith.Runtime,
		"CI fallback must use default runtime 'shell'")
	assert.Equal(t, "none", scaf.calledWith.Auth,
		"CI fallback must use default auth 'none'")
}

func TestInit_CIFalse_WizardCalled(t *testing.T) {
	// CI=false should NOT suppress the wizard.
	t.Setenv("CI", "false")

	scaf := &mockScaffolder{result: defaultScaffoldResult("proj")}
	wiz := &mockWizard{result: &tui.WizardResult{
		Description: "from wizard",
		Runtime:     "shell",
		Auth:        "none",
	}}
	cfg := &initConfig{Scaffolder: scaf, Wizard: wiz}

	_, err := executeInitCmd(cfg, "proj")
	require.NoError(t, err)
	assert.True(t, wiz.called,
		"wizard must be called when CI=false (it is not a CI environment)")
}

// ---------------------------------------------------------------------------
// AC-17: --runtime go flag with --yes
// ---------------------------------------------------------------------------

func TestInit_Yes_RuntimeGo(t *testing.T) {
	scaf := &mockScaffolder{result: defaultScaffoldResult("go-proj")}
	cfg := &initConfig{Scaffolder: scaf, Wizard: &mockWizard{}}

	_, err := executeInitCmd(cfg, "go-proj", "--yes", "--runtime", "go")
	require.NoError(t, err)
	require.True(t, scaf.called)
	assert.Equal(t, "go", scaf.calledWith.Runtime,
		"--runtime go must pass 'go' to scaffolder")
}

// ---------------------------------------------------------------------------
// Error handling: invalid runtime
// ---------------------------------------------------------------------------

func TestInit_InvalidRuntime_ReturnsError(t *testing.T) {
	scaf := &mockScaffolder{result: defaultScaffoldResult("proj")}
	cfg := &initConfig{Scaffolder: scaf, Wizard: &mockWizard{}}

	_, err := executeInitCmd(cfg, "proj", "--yes", "--runtime", "ruby")
	require.Error(t, err,
		"invalid runtime 'ruby' must return an error")
	assert.Contains(t, err.Error(), "ruby",
		"error message must contain the invalid runtime value")
	assert.False(t, scaf.called,
		"scaffolder must NOT be called with an invalid runtime")
}

func TestInit_InvalidRuntime_JSON_HasStructuredError(t *testing.T) {
	scaf := &mockScaffolder{result: defaultScaffoldResult("proj")}
	cfg := &initConfig{Scaffolder: scaf, Wizard: &mockWizard{}}

	stdout, _ := executeInitCmd(cfg, "--json", "proj", "--yes", "--runtime", "ruby")
	require.NotEmpty(t, stdout,
		"JSON output must be produced for invalid runtime error")

	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &got),
		"--json output must be valid JSON, got: %s", stdout)

	errObj, ok := got["error"].(map[string]any)
	require.True(t, ok,
		"JSON output must have top-level 'error' object")
	msg, ok := errObj["message"].(string)
	require.True(t, ok, "error.message must be a string")
	assert.Contains(t, msg, "ruby",
		"JSON error message must mention the invalid runtime")
}

// ---------------------------------------------------------------------------
// Error handling: scaffolder error propagates
// ---------------------------------------------------------------------------

func TestInit_ScaffolderError_Propagates(t *testing.T) {
	scaf := &mockScaffolder{err: fmt.Errorf("disk full: cannot create directory")}
	cfg := &initConfig{Scaffolder: scaf, Wizard: &mockWizard{}}

	_, err := executeInitCmd(cfg, "my-tool", "--yes")
	require.Error(t, err,
		"scaffolder error must propagate as command error")
	assert.Contains(t, err.Error(), "disk full",
		"error message must contain the scaffolder error")
}

func TestInit_ScaffolderError_JSON_HasStructuredError(t *testing.T) {
	scaf := &mockScaffolder{err: fmt.Errorf("permission denied")}
	cfg := &initConfig{Scaffolder: scaf, Wizard: &mockWizard{}}

	stdout, _ := executeInitCmd(cfg, "--json", "my-tool", "--yes")
	require.NotEmpty(t, stdout,
		"JSON output must be produced for scaffolder error")

	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &got),
		"--json output must be valid JSON, got: %s", stdout)

	errObj, ok := got["error"].(map[string]any)
	require.True(t, ok,
		"JSON output must have top-level 'error' object")
	msg, ok := errObj["message"].(string)
	require.True(t, ok, "error.message must be a string")
	assert.Contains(t, msg, "permission denied",
		"JSON error message must contain the scaffolder error")
}

// ---------------------------------------------------------------------------
// JSON mode: success output
// ---------------------------------------------------------------------------

func TestInit_Success_JSON_HasFilesAndDir(t *testing.T) {
	scaf := &mockScaffolder{result: &scaffold.ScaffoldResult{
		Dir: "my-tool",
		Files: []string{
			"my-tool/toolwright.yaml",
			"my-tool/tools/hello.sh",
		},
	}}
	cfg := &initConfig{Scaffolder: scaf, Wizard: &mockWizard{}}

	stdout, err := executeInitCmd(cfg, "--json", "my-tool", "--yes")
	require.NoError(t, err)
	require.NotEmpty(t, stdout,
		"JSON output must be produced on success")

	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &got),
		"--json success output must be valid JSON, got: %s", stdout)

	// Must NOT have an error key on success.
	_, hasError := got["error"]
	assert.False(t, hasError,
		"successful init JSON must not contain 'error' key")

	// Must contain the directory created.
	raw, _ := json.Marshal(got)
	assert.Contains(t, string(raw), "my-tool",
		"JSON success output must reference the created directory")

	// Must contain the files list.
	files, ok := got["files"]
	require.True(t, ok,
		"JSON success output must contain a 'files' field")
	filesArr, ok := files.([]any)
	require.True(t, ok,
		"files must be a JSON array")
	assert.Len(t, filesArr, 2,
		"files array must contain exactly the files returned by scaffolder")
	assert.Equal(t, "my-tool/toolwright.yaml", filesArr[0],
		"files[0] must match scaffolder result")
	assert.Equal(t, "my-tool/tools/hello.sh", filesArr[1],
		"files[1] must match scaffolder result")
}

func TestInit_Success_JSON_IsOnlyJSON(t *testing.T) {
	scaf := &mockScaffolder{result: defaultScaffoldResult("proj")}
	cfg := &initConfig{Scaffolder: scaf, Wizard: &mockWizard{}}

	stdout, err := executeInitCmd(cfg, "--json", "proj", "--yes")
	require.NoError(t, err)
	require.True(t, json.Valid([]byte(stdout)),
		"with --json, stdout must contain ONLY valid JSON (no human text mixed in), got: %s", stdout)
}

// ---------------------------------------------------------------------------
// Human output: success mentions directory and files
// ---------------------------------------------------------------------------

func TestInit_Success_HumanOutput_MentionsDirectory(t *testing.T) {
	scaf := &mockScaffolder{result: &scaffold.ScaffoldResult{
		Dir: "awesome-tool",
		Files: []string{
			"awesome-tool/toolwright.yaml",
			"awesome-tool/tools/hello.sh",
			"awesome-tool/schema/hello.json",
		},
	}}
	cfg := &initConfig{Scaffolder: scaf, Wizard: &mockWizard{}}

	stdout, err := executeInitCmd(cfg, "awesome-tool", "--yes")
	require.NoError(t, err)
	assert.Contains(t, stdout, "awesome-tool",
		"human success output must mention the created directory name")
}

func TestInit_Success_HumanOutput_MentionsFileCount(t *testing.T) {
	scaf := &mockScaffolder{result: &scaffold.ScaffoldResult{
		Dir: "my-tool",
		Files: []string{
			"my-tool/toolwright.yaml",
			"my-tool/tools/hello.sh",
			"my-tool/schema/hello.json",
			"my-tool/tests/hello_test.yaml",
		},
	}}
	cfg := &initConfig{Scaffolder: scaf, Wizard: &mockWizard{}}

	stdout, err := executeInitCmd(cfg, "my-tool", "--yes")
	require.NoError(t, err)
	// Output should mention the number of files created or list them.
	assert.Contains(t, stdout, "4",
		"human success output must mention the file count (4 files created)")
}

func TestInit_Success_HumanOutput_ListsFiles(t *testing.T) {
	scaf := &mockScaffolder{result: &scaffold.ScaffoldResult{
		Dir: "proj",
		Files: []string{
			"proj/toolwright.yaml",
			"proj/tools/hello.sh",
		},
	}}
	cfg := &initConfig{Scaffolder: scaf, Wizard: &mockWizard{}}

	stdout, err := executeInitCmd(cfg, "proj", "--yes")
	require.NoError(t, err)
	// At minimum, output should list the files created.
	assert.Contains(t, stdout, "toolwright.yaml",
		"human output should reference the created manifest file")
}

// ---------------------------------------------------------------------------
// Anti-hardcoding: different names produce different scaffolder calls
// ---------------------------------------------------------------------------

func TestInit_DifferentNames_DifferentScaffolderCalls(t *testing.T) {
	scaf1 := &mockScaffolder{result: defaultScaffoldResult("alpha")}
	cfg1 := &initConfig{Scaffolder: scaf1, Wizard: &mockWizard{}}
	_, err1 := executeInitCmd(cfg1, "alpha", "--yes")
	require.NoError(t, err1)

	scaf2 := &mockScaffolder{result: defaultScaffoldResult("beta")}
	cfg2 := &initConfig{Scaffolder: scaf2, Wizard: &mockWizard{}}
	_, err2 := executeInitCmd(cfg2, "beta", "--yes")
	require.NoError(t, err2)

	assert.NotEqual(t, scaf1.calledWith.Name, scaf2.calledWith.Name,
		"different project names must produce different scaffolder Name values; anti-hardcoding")
	assert.Equal(t, "alpha", scaf1.calledWith.Name)
	assert.Equal(t, "beta", scaf2.calledWith.Name)
}

func TestInit_DifferentRuntimes_DifferentScaffolderCalls(t *testing.T) {
	scaf1 := &mockScaffolder{result: defaultScaffoldResult("proj")}
	cfg1 := &initConfig{Scaffolder: scaf1, Wizard: &mockWizard{}}
	_, err1 := executeInitCmd(cfg1, "proj", "--yes", "--runtime", "go")
	require.NoError(t, err1)

	scaf2 := &mockScaffolder{result: defaultScaffoldResult("proj")}
	cfg2 := &initConfig{Scaffolder: scaf2, Wizard: &mockWizard{}}
	_, err2 := executeInitCmd(cfg2, "proj", "--yes", "--runtime", "python")
	require.NoError(t, err2)

	assert.NotEqual(t, scaf1.calledWith.Runtime, scaf2.calledWith.Runtime,
		"different --runtime values must produce different scaffolder Runtime values; anti-hardcoding")
}

// ---------------------------------------------------------------------------
// Anti-hardcoding: JSON success output reflects actual scaffolder result
// ---------------------------------------------------------------------------

func TestInit_JSON_OutputReflectsScaffolderResult(t *testing.T) {
	// Two different scaffold results must produce different JSON output.
	scaf1 := &mockScaffolder{result: &scaffold.ScaffoldResult{
		Dir:   "proj-a",
		Files: []string{"proj-a/toolwright.yaml"},
	}}
	cfg1 := &initConfig{Scaffolder: scaf1, Wizard: &mockWizard{}}
	stdout1, err1 := executeInitCmd(cfg1, "--json", "proj-a", "--yes")
	require.NoError(t, err1)

	scaf2 := &mockScaffolder{result: &scaffold.ScaffoldResult{
		Dir:   "proj-b",
		Files: []string{"proj-b/toolwright.yaml", "proj-b/main.go"},
	}}
	cfg2 := &initConfig{Scaffolder: scaf2, Wizard: &mockWizard{}}
	stdout2, err2 := executeInitCmd(cfg2, "--json", "proj-b", "--yes")
	require.NoError(t, err2)

	assert.NotEqual(t, stdout1, stdout2,
		"different scaffold results must produce different JSON output; anti-hardcoding")
}

// ---------------------------------------------------------------------------
// Edge case: project name with special characters
// ---------------------------------------------------------------------------

func TestInit_ProjectNameWithHyphens(t *testing.T) {
	scaf := &mockScaffolder{result: defaultScaffoldResult("my-awesome-tool")}
	cfg := &initConfig{Scaffolder: scaf, Wizard: &mockWizard{}}

	_, err := executeInitCmd(cfg, "my-awesome-tool", "--yes")
	require.NoError(t, err)
	assert.Equal(t, "my-awesome-tool", scaf.calledWith.Name,
		"hyphenated project names must be passed through to scaffolder unchanged")
}

func TestInit_ProjectNameWithUnderscore(t *testing.T) {
	scaf := &mockScaffolder{result: defaultScaffoldResult("my_tool")}
	cfg := &initConfig{Scaffolder: scaf, Wizard: &mockWizard{}}

	_, err := executeInitCmd(cfg, "my_tool", "--yes")
	require.NoError(t, err)
	assert.Equal(t, "my_tool", scaf.calledWith.Name,
		"underscored project names must be passed through to scaffolder unchanged")
}

// ---------------------------------------------------------------------------
// Edge case: wizard result fields override CLI runtime flag in interactive
// ---------------------------------------------------------------------------

func TestInit_NoYes_RuntimeFlagIgnoredWhenWizardUsed(t *testing.T) {
	// When the wizard is active, --runtime from CLI should be overridden
	// by the wizard's choice (the wizard gathers all options interactively).
	t.Setenv("CI", "")
	os.Unsetenv("CI")

	scaf := &mockScaffolder{result: defaultScaffoldResult("proj")}
	wiz := &mockWizard{result: &tui.WizardResult{
		Description: "Wizard picked this",
		Runtime:     "typescript",
		Auth:        "none",
	}}
	cfg := &initConfig{Scaffolder: scaf, Wizard: wiz}

	_, err := executeInitCmd(cfg, "proj", "--runtime", "go")
	require.NoError(t, err)
	require.True(t, scaf.called)
	assert.Equal(t, "typescript", scaf.calledWith.Runtime,
		"when wizard is active, wizard's runtime choice must override --runtime flag")
}

// ---------------------------------------------------------------------------
// Edge case: all flags combined
// ---------------------------------------------------------------------------

func TestInit_AllFlagsCombined(t *testing.T) {
	outDir := t.TempDir()
	scaf := &mockScaffolder{result: &scaffold.ScaffoldResult{
		Dir:   outDir + "/combined-proj",
		Files: []string{outDir + "/combined-proj/toolwright.yaml"},
	}}
	cfg := &initConfig{Scaffolder: scaf, Wizard: &mockWizard{}}

	_, err := executeInitCmd(cfg,
		"combined-proj",
		"--yes",
		"--runtime", "typescript",
		"--description", "A combined test",
		"--output", outDir,
	)
	require.NoError(t, err)
	require.True(t, scaf.called)

	assert.Equal(t, "combined-proj", scaf.calledWith.Name,
		"name must come from positional arg")
	assert.Equal(t, "typescript", scaf.calledWith.Runtime,
		"runtime must come from --runtime flag")
	assert.Equal(t, "A combined test", scaf.calledWith.Description,
		"description must come from --description flag")
	assert.Equal(t, outDir, scaf.calledWith.OutputDir,
		"output dir must come from --output flag")
}

// ---------------------------------------------------------------------------
// Edge case: empty description flag should be accepted
// ---------------------------------------------------------------------------

func TestInit_EmptyDescription_Accepted(t *testing.T) {
	scaf := &mockScaffolder{result: defaultScaffoldResult("proj")}
	cfg := &initConfig{Scaffolder: scaf, Wizard: &mockWizard{}}

	_, err := executeInitCmd(cfg, "proj", "--yes", "--description", "")
	require.NoError(t, err)
	require.True(t, scaf.called)
	// An explicitly empty description should be passed through (or use default).
	// The key point: it must not error.
}

// ---------------------------------------------------------------------------
// Edge case: scaffolder nil result with error
// ---------------------------------------------------------------------------

func TestInit_ScaffolderNilResultWithError_ReturnsError(t *testing.T) {
	scaf := &mockScaffolder{result: nil, err: fmt.Errorf("catastrophic failure")}
	cfg := &initConfig{Scaffolder: scaf, Wizard: &mockWizard{}}

	_, err := executeInitCmd(cfg, "my-tool", "--yes")
	require.Error(t, err,
		"nil result with error from scaffolder must propagate")
	assert.Contains(t, err.Error(), "catastrophic failure")
}

// ---------------------------------------------------------------------------
// Edge case: valid runtimes boundary (only shell, go, python, typescript)
// ---------------------------------------------------------------------------

func TestInit_InvalidRuntimes_TableDriven(t *testing.T) {
	invalidRuntimes := []string{"ruby", "rust", "java", "c++", "", "Shell", "GO", "PYTHON"}
	for _, rt := range invalidRuntimes {
		t.Run("runtime_"+rt, func(t *testing.T) {
			scaf := &mockScaffolder{result: defaultScaffoldResult("proj")}
			cfg := &initConfig{Scaffolder: scaf, Wizard: &mockWizard{}}

			_, err := executeInitCmd(cfg, "proj", "--yes", "--runtime", rt)
			require.Error(t, err,
				"invalid runtime %q must return an error", rt)
			assert.NotContains(t, err.Error(), "unknown command",
				"error must be from runtime validation, not from cobra failing to find the command")
			assert.False(t, scaf.called,
				"scaffolder must NOT be called with invalid runtime %q", rt)
		})
	}
}

// ---------------------------------------------------------------------------
// CI + --yes combined: --yes takes precedence (no wizard)
// ---------------------------------------------------------------------------

func TestInit_CIAndYes_ScaffolderCalledWizardNot(t *testing.T) {
	t.Setenv("CI", "true")

	scaf := &mockScaffolder{result: defaultScaffoldResult("proj")}
	wiz := &mockWizard{result: &tui.WizardResult{Runtime: "go"}}
	cfg := &initConfig{Scaffolder: scaf, Wizard: wiz}

	_, err := executeInitCmd(cfg, "proj", "--yes")
	require.NoError(t, err)
	assert.True(t, scaf.called)
	assert.False(t, wiz.called,
		"wizard must NOT be called when both CI=true and --yes are set")
}

// ---------------------------------------------------------------------------
// JSON mode: error when missing project name
// ---------------------------------------------------------------------------

func TestInit_MissingProjectName_NoFlags_ReturnsError(t *testing.T) {
	scaf := &mockScaffolder{result: defaultScaffoldResult("x")}
	cfg := &initConfig{Scaffolder: scaf, Wizard: &mockWizard{}}

	// No positional arg, no --yes — should still error about missing name.
	_, err := executeInitCmd(cfg)
	require.Error(t, err,
		"init with no arguments must return an error about missing project name")
	assert.NotContains(t, err.Error(), "unknown command",
		"error must be from the init command itself, not from cobra failing to find the command")
	assert.False(t, scaf.called,
		"scaffolder must NOT be called when project name is missing")
}

// ---------------------------------------------------------------------------
// Output dir defaults to current directory when not specified
// ---------------------------------------------------------------------------

func TestInit_OutputDir_DefaultsToEmpty(t *testing.T) {
	// When --output is not specified, OutputDir should be empty or "."
	// (implementation decides, but it must NOT be some random path).
	scaf := &mockScaffolder{result: defaultScaffoldResult("proj")}
	cfg := &initConfig{Scaffolder: scaf, Wizard: &mockWizard{}}

	_, err := executeInitCmd(cfg, "proj", "--yes")
	require.NoError(t, err)
	require.True(t, scaf.called)

	// OutputDir should be empty string or "." when not specified.
	outputDir := scaf.calledWith.OutputDir
	assert.Condition(t, func() bool {
		return outputDir == "" || outputDir == "."
	}, "OutputDir must default to empty string or '.' when --output is not specified, got: %q", outputDir)
}

// ---------------------------------------------------------------------------
// --auth flag: non-interactive mode
// ---------------------------------------------------------------------------

func TestInit_Auth_DefaultsToNone(t *testing.T) {
	scaf := &mockScaffolder{result: defaultScaffoldResult("proj")}
	cfg := &initConfig{Scaffolder: scaf, Wizard: &mockWizard{}}

	_, err := executeInitCmd(cfg, "proj", "--yes")
	require.NoError(t, err)
	assert.Equal(t, "none", scaf.calledWith.Auth,
		"auth must default to 'none' when --auth is not specified")
}

func TestInit_Auth_TokenFlag(t *testing.T) {
	scaf := &mockScaffolder{result: defaultScaffoldResult("proj")}
	cfg := &initConfig{Scaffolder: scaf, Wizard: &mockWizard{}}

	_, err := executeInitCmd(cfg, "proj", "--yes", "--auth", "token")
	require.NoError(t, err)
	assert.Equal(t, "token", scaf.calledWith.Auth,
		"scaffolder must receive auth=token when --auth token is specified")
}

func TestInit_Auth_OAuth2Flag(t *testing.T) {
	scaf := &mockScaffolder{result: defaultScaffoldResult("proj")}
	cfg := &initConfig{Scaffolder: scaf, Wizard: &mockWizard{}}

	_, err := executeInitCmd(cfg, "proj", "--yes", "--auth", "oauth2")
	require.NoError(t, err)
	assert.Equal(t, "oauth2", scaf.calledWith.Auth,
		"scaffolder must receive auth=oauth2 when --auth oauth2 is specified")
}

func TestInit_InvalidAuth_ReturnsError(t *testing.T) {
	scaf := &mockScaffolder{result: defaultScaffoldResult("proj")}
	cfg := &initConfig{Scaffolder: scaf, Wizard: &mockWizard{}}

	_, err := executeInitCmd(cfg, "proj", "--yes", "--auth", "ldap")
	require.Error(t, err,
		"invalid auth 'ldap' must return an error")
	assert.Contains(t, err.Error(), "ldap",
		"error message must contain the invalid auth value")
	assert.False(t, scaf.called,
		"scaffolder must NOT be called with an invalid auth")
}
