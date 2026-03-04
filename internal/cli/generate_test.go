package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Obsidian-Owl/toolwright/internal/codegen"
	"github.com/Obsidian-Owl/toolwright/internal/manifest"
)

// ---------------------------------------------------------------------------
// Mock code generator
// ---------------------------------------------------------------------------

type mockCodeGenerator struct {
	result     *codegen.GenerateResult
	err        error
	calledWith codegen.GenerateOptions
	manifest   *manifest.Toolkit
	called     bool
}

func (m *mockCodeGenerator) Generate(_ context.Context, tk manifest.Toolkit, opts codegen.GenerateOptions) (*codegen.GenerateResult, error) {
	m.called = true
	m.calledWith = opts
	m.manifest = &tk
	return m.result, m.err
}

// ---------------------------------------------------------------------------
// Test data helpers
// ---------------------------------------------------------------------------

// generateManifest returns a minimal valid manifest with the given toolkit name.
func generateManifest(name string) string {
	return `apiVersion: toolwright/v1
kind: Toolkit
metadata:
  name: ` + name + `
  version: 1.0.0
  description: A test toolkit
tools:
  - name: hello
    description: Say hello
    entrypoint: ./hello.sh
`
}

// writeGenerateManifest writes manifest content to a temp dir and returns the path.
func writeGenerateManifest(t *testing.T, dir, content string) string {
	t.Helper()
	path := filepath.Join(dir, "toolwright.yaml")
	err := os.WriteFile(path, []byte(content), 0644)
	require.NoError(t, err, "test setup: writing manifest file")
	return path
}

// executeGenerateCmd runs the generate command through the root command tree
// and returns stdout and the error (if any).
func executeGenerateCmd(cfg *generateConfig, args ...string) (stdout string, err error) {
	root := NewRootCommand()
	gen := newGenerateCmd(cfg)
	root.AddCommand(gen)
	outBuf := &bytes.Buffer{}
	root.SetOut(outBuf)
	root.SetErr(&bytes.Buffer{})
	root.SetArgs(append([]string{"generate"}, args...))
	execErr := root.Execute()
	return outBuf.String(), execErr
}

// findGenerateSubcommand finds a subcommand by name on the generate command.
// It fails the test if the subcommand is not found.
func findGenerateSubcommand(t *testing.T, parent *cobra.Command, name string) *cobra.Command {
	t.Helper()
	for _, sub := range parent.Commands() {
		if sub.Name() == name {
			return sub
		}
	}
	var names []string
	for _, sub := range parent.Commands() {
		names = append(names, sub.Name())
	}
	t.Fatalf("subcommand %q not found on %q; available: %v", name, parent.Name(), names)
	return nil
}

// mapKeys returns the keys of a map for diagnostic output.
func mapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// ---------------------------------------------------------------------------
// Command structure tests
// ---------------------------------------------------------------------------

func TestNewGenerateCmd_ReturnsNonNil(t *testing.T) {
	cfg := &generateConfig{}
	cmd := newGenerateCmd(cfg)
	require.NotNil(t, cmd, "newGenerateCmd must return a non-nil *cobra.Command")
}

func TestNewGenerateCmd_HasCorrectUseField(t *testing.T) {
	cfg := &generateConfig{}
	cmd := newGenerateCmd(cfg)
	assert.Equal(t, "generate", cmd.Use,
		"generate command Use field must be 'generate'")
}

func TestNewGenerateCmd_HasCLISubcommand(t *testing.T) {
	cfg := &generateConfig{}
	cmd := newGenerateCmd(cfg)

	var found bool
	for _, sub := range cmd.Commands() {
		if sub.Name() == "cli" {
			found = true
			break
		}
	}
	assert.True(t, found,
		"generate must have a 'cli' subcommand")
}

func TestNewGenerateCmd_HasMCPSubcommand(t *testing.T) {
	cfg := &generateConfig{}
	cmd := newGenerateCmd(cfg)

	var found bool
	for _, sub := range cmd.Commands() {
		if sub.Name() == "mcp" {
			found = true
			break
		}
	}
	assert.True(t, found,
		"generate must have an 'mcp' subcommand")
}

// ---------------------------------------------------------------------------
// generate cli: flag tests
// ---------------------------------------------------------------------------

func TestGenerateCLI_HasOutputFlag(t *testing.T) {
	cfg := &generateConfig{}
	cmd := newGenerateCmd(cfg)
	cli := findGenerateSubcommand(t, cmd, "cli")

	f := cli.Flags().Lookup("output")
	require.NotNil(t, f, "generate cli must have an --output flag")
}

func TestGenerateCLI_HasTargetFlag(t *testing.T) {
	cfg := &generateConfig{}
	cmd := newGenerateCmd(cfg)
	cli := findGenerateSubcommand(t, cmd, "cli")

	f := cli.Flags().Lookup("target")
	require.NotNil(t, f, "generate cli must have a --target flag")
}

func TestGenerateCLI_TargetDefaultIsGo(t *testing.T) {
	cfg := &generateConfig{}
	cmd := newGenerateCmd(cfg)
	cli := findGenerateSubcommand(t, cmd, "cli")

	f := cli.Flags().Lookup("target")
	require.NotNil(t, f, "generate cli must have a --target flag")
	assert.Equal(t, "go", f.DefValue,
		"generate cli --target default must be 'go'")
}

func TestGenerateCLI_HasDryRunFlag(t *testing.T) {
	cfg := &generateConfig{}
	cmd := newGenerateCmd(cfg)
	cli := findGenerateSubcommand(t, cmd, "cli")

	f := cli.Flags().Lookup("dry-run")
	require.NotNil(t, f, "generate cli must have a --dry-run flag")
	assert.Equal(t, "false", f.DefValue,
		"--dry-run default must be false")
}

func TestGenerateCLI_HasForceFlag(t *testing.T) {
	cfg := &generateConfig{}
	cmd := newGenerateCmd(cfg)
	cli := findGenerateSubcommand(t, cmd, "cli")

	f := cli.Flags().Lookup("force")
	require.NotNil(t, f, "generate cli must have a --force flag")
	assert.Equal(t, "false", f.DefValue,
		"--force default must be false")
}

func TestGenerateCLI_HasManifestFlag(t *testing.T) {
	cfg := &generateConfig{}
	cmd := newGenerateCmd(cfg)
	cli := findGenerateSubcommand(t, cmd, "cli")

	f := cli.Flags().Lookup("manifest")
	require.NotNil(t, f, "generate cli must have a --manifest flag")
}

// ---------------------------------------------------------------------------
// generate mcp: flag tests
// ---------------------------------------------------------------------------

func TestGenerateMCP_HasOutputFlag(t *testing.T) {
	cfg := &generateConfig{}
	cmd := newGenerateCmd(cfg)
	mcp := findGenerateSubcommand(t, cmd, "mcp")

	f := mcp.Flags().Lookup("output")
	require.NotNil(t, f, "generate mcp must have an --output flag")
}

func TestGenerateMCP_HasTargetFlag(t *testing.T) {
	cfg := &generateConfig{}
	cmd := newGenerateCmd(cfg)
	mcp := findGenerateSubcommand(t, cmd, "mcp")

	f := mcp.Flags().Lookup("target")
	require.NotNil(t, f, "generate mcp must have a --target flag")
}

func TestGenerateMCP_TargetDefaultIsTypescript(t *testing.T) {
	cfg := &generateConfig{}
	cmd := newGenerateCmd(cfg)
	mcp := findGenerateSubcommand(t, cmd, "mcp")

	f := mcp.Flags().Lookup("target")
	require.NotNil(t, f, "generate mcp must have a --target flag")
	assert.Equal(t, "typescript", f.DefValue,
		"generate mcp --target default must be 'typescript'")
}

func TestGenerateMCP_HasDryRunFlag(t *testing.T) {
	cfg := &generateConfig{}
	cmd := newGenerateCmd(cfg)
	mcp := findGenerateSubcommand(t, cmd, "mcp")

	f := mcp.Flags().Lookup("dry-run")
	require.NotNil(t, f, "generate mcp must have a --dry-run flag")
}

func TestGenerateMCP_HasForceFlag(t *testing.T) {
	cfg := &generateConfig{}
	cmd := newGenerateCmd(cfg)
	mcp := findGenerateSubcommand(t, cmd, "mcp")

	f := mcp.Flags().Lookup("force")
	require.NotNil(t, f, "generate mcp must have a --force flag")
}

func TestGenerateMCP_HasManifestFlag(t *testing.T) {
	cfg := &generateConfig{}
	cmd := newGenerateCmd(cfg)
	mcp := findGenerateSubcommand(t, cmd, "mcp")

	f := mcp.Flags().Lookup("manifest")
	require.NotNil(t, f, "generate mcp must have a --manifest flag")
}

func TestGenerateMCP_HasTransportFlag(t *testing.T) {
	cfg := &generateConfig{}
	cmd := newGenerateCmd(cfg)
	mcp := findGenerateSubcommand(t, cmd, "mcp")

	f := mcp.Flags().Lookup("transport")
	require.NotNil(t, f, "generate mcp must have a --transport flag")
}

func TestGenerateCLI_DoesNotHaveTransportFlag(t *testing.T) {
	cfg := &generateConfig{}
	cmd := newGenerateCmd(cfg)
	cli := findGenerateSubcommand(t, cmd, "cli")

	f := cli.Flags().Lookup("transport")
	assert.Nil(t, f,
		"generate cli must NOT have a --transport flag (transport is MCP-only)")
}

// ---------------------------------------------------------------------------
// AC-14: generate cli - default output directory
// ---------------------------------------------------------------------------

func TestGenerateCLI_DefaultOutputDir_UsesToolkitName(t *testing.T) {
	dir := t.TempDir()
	path := writeGenerateManifest(t, dir, generateManifest("pet-store"))

	mock := &mockCodeGenerator{
		result: &codegen.GenerateResult{
			Files:  []string{"main.go", "go.mod"},
			DryRun: false,
			Mode:   "cli",
			Target: "go",
		},
	}
	cfg := &generateConfig{Engine: mock}

	_, err := executeGenerateCmd(cfg, "cli", "--manifest", path)
	require.NoError(t, err, "generate cli with valid manifest must not error")

	require.True(t, mock.called, "engine.Generate must be called")
	assert.Contains(t, mock.calledWith.OutputDir, "cli-pet-store",
		"default output dir must contain 'cli-{toolkit-name}'; got %q", mock.calledWith.OutputDir)
}

func TestGenerateCLI_DefaultOutputDir_DifferentName(t *testing.T) {
	dir := t.TempDir()
	path := writeGenerateManifest(t, dir, generateManifest("weather-api"))

	mock := &mockCodeGenerator{
		result: &codegen.GenerateResult{
			Files:  []string{"main.go"},
			DryRun: false,
			Mode:   "cli",
			Target: "go",
		},
	}
	cfg := &generateConfig{Engine: mock}

	_, err := executeGenerateCmd(cfg, "cli", "--manifest", path)
	require.NoError(t, err)

	require.True(t, mock.called, "engine.Generate must be called")
	assert.Contains(t, mock.calledWith.OutputDir, "cli-weather-api",
		"default output dir must derive from toolkit name, not be hardcoded; got %q",
		mock.calledWith.OutputDir)
}

// ---------------------------------------------------------------------------
// AC-14: generate cli - --output overrides output dir
// ---------------------------------------------------------------------------

func TestGenerateCLI_OutputFlagOverridesDefault(t *testing.T) {
	dir := t.TempDir()
	path := writeGenerateManifest(t, dir, generateManifest("pet-store"))
	customDir := filepath.Join(dir, "custom-output")

	mock := &mockCodeGenerator{
		result: &codegen.GenerateResult{
			Files:  []string{"main.go"},
			DryRun: false,
			Mode:   "cli",
			Target: "go",
		},
	}
	cfg := &generateConfig{Engine: mock}

	_, err := executeGenerateCmd(cfg, "cli", "--manifest", path, "--output", customDir)
	require.NoError(t, err)

	require.True(t, mock.called, "engine.Generate must be called")
	assert.Equal(t, customDir, mock.calledWith.OutputDir,
		"--output must override the default output dir")
}

// ---------------------------------------------------------------------------
// AC-14: generate cli - engine called with correct mode and target
// ---------------------------------------------------------------------------

func TestGenerateCLI_EngineCalledWithModeAndTarget(t *testing.T) {
	dir := t.TempDir()
	path := writeGenerateManifest(t, dir, generateManifest("pet-store"))

	mock := &mockCodeGenerator{
		result: &codegen.GenerateResult{
			Files:  []string{"main.go"},
			DryRun: false,
			Mode:   "cli",
			Target: "go",
		},
	}
	cfg := &generateConfig{Engine: mock}

	_, err := executeGenerateCmd(cfg, "cli", "--manifest", path)
	require.NoError(t, err)

	require.True(t, mock.called, "engine.Generate must be called")
	assert.Equal(t, "cli", mock.calledWith.Mode,
		"engine must be called with Mode='cli'")
	assert.Equal(t, "go", mock.calledWith.Target,
		"engine must be called with Target='go' (the default)")
}

// ---------------------------------------------------------------------------
// AC-14: generate cli --target typescript -> error
// ---------------------------------------------------------------------------

func TestGenerateCLI_TargetTypescript_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	path := writeGenerateManifest(t, dir, generateManifest("pet-store"))

	mock := &mockCodeGenerator{}
	cfg := &generateConfig{Engine: mock}

	_, err := executeGenerateCmd(cfg, "cli", "--manifest", path, "--target", "typescript")
	require.Error(t, err,
		"generate cli --target typescript must return an error")
	assert.Contains(t, strings.ToLower(err.Error()), "not yet implemented",
		"error message must contain 'not yet implemented'; got: %s", err.Error())
}

func TestGenerateCLI_TargetTypescript_EngineNotCalled(t *testing.T) {
	dir := t.TempDir()
	path := writeGenerateManifest(t, dir, generateManifest("pet-store"))

	mock := &mockCodeGenerator{}
	cfg := &generateConfig{Engine: mock}

	_, _ = executeGenerateCmd(cfg, "cli", "--manifest", path, "--target", "typescript")
	assert.False(t, mock.called,
		"engine.Generate must NOT be called when target is unsupported")
}

func TestGenerateCLI_TargetTypescript_ErrorMentionsTypescript(t *testing.T) {
	dir := t.TempDir()
	path := writeGenerateManifest(t, dir, generateManifest("pet-store"))

	mock := &mockCodeGenerator{}
	cfg := &generateConfig{Engine: mock}

	_, err := executeGenerateCmd(cfg, "cli", "--manifest", path, "--target", "typescript")
	require.Error(t, err)
	errLower := strings.ToLower(err.Error())
	assert.Contains(t, errLower, "typescript",
		"error message must mention 'typescript' so the user knows which target failed")
}

// ---------------------------------------------------------------------------
// AC-14: generate cli --dry-run
// ---------------------------------------------------------------------------

func TestGenerateCLI_DryRun_EngineCalledWithDryRunTrue(t *testing.T) {
	dir := t.TempDir()
	path := writeGenerateManifest(t, dir, generateManifest("pet-store"))

	mock := &mockCodeGenerator{
		result: &codegen.GenerateResult{
			Files:  []string{"main.go", "go.mod"},
			DryRun: true,
			Mode:   "cli",
			Target: "go",
		},
	}
	cfg := &generateConfig{Engine: mock}

	_, err := executeGenerateCmd(cfg, "cli", "--manifest", path, "--dry-run")
	require.NoError(t, err, "generate cli --dry-run must not error when engine succeeds")

	require.True(t, mock.called, "engine.Generate must be called even in dry-run mode")
	assert.True(t, mock.calledWith.DryRun,
		"engine must be called with DryRun=true when --dry-run is set")
}

func TestGenerateCLI_DryRun_OutputListsFiles(t *testing.T) {
	dir := t.TempDir()
	path := writeGenerateManifest(t, dir, generateManifest("pet-store"))

	mock := &mockCodeGenerator{
		result: &codegen.GenerateResult{
			Files:  []string{"main.go", "go.mod", "cmd/root.go"},
			DryRun: true,
			Mode:   "cli",
			Target: "go",
		},
	}
	cfg := &generateConfig{Engine: mock}

	stdout, err := executeGenerateCmd(cfg, "cli", "--manifest", path, "--dry-run")
	require.NoError(t, err)

	// The output must list the files that would be generated.
	assert.Contains(t, stdout, "main.go",
		"dry-run output must list generated files")
	assert.Contains(t, stdout, "go.mod",
		"dry-run output must list all generated files")
	assert.Contains(t, stdout, "cmd/root.go",
		"dry-run output must list all generated files including nested paths")
}

// ---------------------------------------------------------------------------
// AC-14: generate cli --force
// ---------------------------------------------------------------------------

func TestGenerateCLI_Force_EngineCalledWithForceTrue(t *testing.T) {
	dir := t.TempDir()
	path := writeGenerateManifest(t, dir, generateManifest("pet-store"))

	mock := &mockCodeGenerator{
		result: &codegen.GenerateResult{
			Files:  []string{"main.go"},
			DryRun: false,
			Mode:   "cli",
			Target: "go",
		},
	}
	cfg := &generateConfig{Engine: mock}

	_, err := executeGenerateCmd(cfg, "cli", "--manifest", path, "--force")
	require.NoError(t, err)

	require.True(t, mock.called, "engine.Generate must be called")
	assert.True(t, mock.calledWith.Force,
		"engine must be called with Force=true when --force is set")
}

func TestGenerateCLI_NoForce_EngineCalledWithForceFalse(t *testing.T) {
	dir := t.TempDir()
	path := writeGenerateManifest(t, dir, generateManifest("pet-store"))

	mock := &mockCodeGenerator{
		result: &codegen.GenerateResult{
			Files:  []string{"main.go"},
			DryRun: false,
			Mode:   "cli",
			Target: "go",
		},
	}
	cfg := &generateConfig{Engine: mock}

	_, err := executeGenerateCmd(cfg, "cli", "--manifest", path)
	require.NoError(t, err)

	require.True(t, mock.called)
	assert.False(t, mock.calledWith.Force,
		"engine must be called with Force=false when --force is not set")
}

// ---------------------------------------------------------------------------
// AC-14: generate cli - engine returns files -> success message
// ---------------------------------------------------------------------------

func TestGenerateCLI_Success_OutputContainsFiles(t *testing.T) {
	dir := t.TempDir()
	path := writeGenerateManifest(t, dir, generateManifest("pet-store"))

	mock := &mockCodeGenerator{
		result: &codegen.GenerateResult{
			Files:  []string{"main.go", "go.mod", "internal/cmd/root.go"},
			DryRun: false,
			Mode:   "cli",
			Target: "go",
		},
	}
	cfg := &generateConfig{Engine: mock}

	stdout, err := executeGenerateCmd(cfg, "cli", "--manifest", path)
	require.NoError(t, err)

	// The output should contain some indication of the files generated.
	assert.Contains(t, stdout, "main.go",
		"success output must reference generated files")
}

// ---------------------------------------------------------------------------
// AC-14: generate cli - engine returns error -> error propagated
// ---------------------------------------------------------------------------

func TestGenerateCLI_EngineError_Propagated(t *testing.T) {
	dir := t.TempDir()
	path := writeGenerateManifest(t, dir, generateManifest("pet-store"))

	engineErr := errors.New("disk full")
	mock := &mockCodeGenerator{err: engineErr}
	cfg := &generateConfig{Engine: mock}

	_, err := executeGenerateCmd(cfg, "cli", "--manifest", path)
	require.Error(t, err, "engine error must be propagated")
	assert.Contains(t, err.Error(), "disk full",
		"propagated error must contain the engine's original error message")
}

// ---------------------------------------------------------------------------
// AC-14: generate cli --json -> JSON output
// ---------------------------------------------------------------------------

func TestGenerateCLI_JSON_OutputIsValidJSON(t *testing.T) {
	dir := t.TempDir()
	path := writeGenerateManifest(t, dir, generateManifest("pet-store"))

	mock := &mockCodeGenerator{
		result: &codegen.GenerateResult{
			Files:  []string{"main.go", "go.mod"},
			DryRun: false,
			Mode:   "cli",
			Target: "go",
		},
	}
	cfg := &generateConfig{Engine: mock}

	stdout, err := executeGenerateCmd(cfg, "cli", "--manifest", path, "--json")
	require.NoError(t, err)

	require.True(t, json.Valid([]byte(stdout)),
		"--json output must be valid JSON; got: %s", stdout)
}

func TestGenerateCLI_JSON_ContainsFilesList(t *testing.T) {
	dir := t.TempDir()
	path := writeGenerateManifest(t, dir, generateManifest("pet-store"))

	mock := &mockCodeGenerator{
		result: &codegen.GenerateResult{
			Files:  []string{"main.go", "go.mod"},
			DryRun: false,
			Mode:   "cli",
			Target: "go",
		},
	}
	cfg := &generateConfig{Engine: mock}

	stdout, err := executeGenerateCmd(cfg, "cli", "--manifest", path, "--json")
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &got),
		"JSON output must be parseable")

	// The output must contain a files list.
	files, ok := got["files"].([]any)
	require.True(t, ok,
		"JSON output must have a 'files' array; got keys: %v", mapKeys(got))
	assert.Len(t, files, 2,
		"files array must contain all generated file paths")
	assert.Equal(t, "main.go", files[0],
		"files[0] must be 'main.go'")
	assert.Equal(t, "go.mod", files[1],
		"files[1] must be 'go.mod'")
}

func TestGenerateCLI_JSON_ContainsModeAndTarget(t *testing.T) {
	dir := t.TempDir()
	path := writeGenerateManifest(t, dir, generateManifest("pet-store"))

	mock := &mockCodeGenerator{
		result: &codegen.GenerateResult{
			Files:  []string{"main.go"},
			DryRun: false,
			Mode:   "cli",
			Target: "go",
		},
	}
	cfg := &generateConfig{Engine: mock}

	stdout, err := executeGenerateCmd(cfg, "cli", "--manifest", path, "--json")
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &got))

	assert.Equal(t, "cli", got["mode"],
		"JSON output must include mode='cli'")
	assert.Equal(t, "go", got["target"],
		"JSON output must include target='go'")
}

// ---------------------------------------------------------------------------
// AC-15: generate mcp - default output directory
// ---------------------------------------------------------------------------

func TestGenerateMCP_DefaultOutputDir_UsesToolkitName(t *testing.T) {
	dir := t.TempDir()
	path := writeGenerateManifest(t, dir, generateManifest("pet-store"))

	mock := &mockCodeGenerator{
		result: &codegen.GenerateResult{
			Files:  []string{"src/index.ts", "package.json"},
			DryRun: false,
			Mode:   "mcp",
			Target: "typescript",
		},
	}
	cfg := &generateConfig{Engine: mock}

	_, err := executeGenerateCmd(cfg, "mcp", "--manifest", path)
	require.NoError(t, err, "generate mcp with valid manifest must not error")

	require.True(t, mock.called, "engine.Generate must be called")
	assert.Contains(t, mock.calledWith.OutputDir, "mcp-server-pet-store",
		"default output dir must contain 'mcp-server-{toolkit-name}'; got %q",
		mock.calledWith.OutputDir)
}

func TestGenerateMCP_DefaultOutputDir_DifferentName(t *testing.T) {
	dir := t.TempDir()
	path := writeGenerateManifest(t, dir, generateManifest("weather-api"))

	mock := &mockCodeGenerator{
		result: &codegen.GenerateResult{
			Files:  []string{"src/index.ts"},
			DryRun: false,
			Mode:   "mcp",
			Target: "typescript",
		},
	}
	cfg := &generateConfig{Engine: mock}

	_, err := executeGenerateCmd(cfg, "mcp", "--manifest", path)
	require.NoError(t, err)

	require.True(t, mock.called)
	assert.Contains(t, mock.calledWith.OutputDir, "mcp-server-weather-api",
		"default output dir must derive from toolkit name, not be hardcoded; got %q",
		mock.calledWith.OutputDir)
}

// ---------------------------------------------------------------------------
// AC-15: generate mcp - engine called with correct mode and target
// ---------------------------------------------------------------------------

func TestGenerateMCP_EngineCalledWithModeAndTarget(t *testing.T) {
	dir := t.TempDir()
	path := writeGenerateManifest(t, dir, generateManifest("pet-store"))

	mock := &mockCodeGenerator{
		result: &codegen.GenerateResult{
			Files:  []string{"src/index.ts"},
			DryRun: false,
			Mode:   "mcp",
			Target: "typescript",
		},
	}
	cfg := &generateConfig{Engine: mock}

	_, err := executeGenerateCmd(cfg, "mcp", "--manifest", path)
	require.NoError(t, err)

	require.True(t, mock.called, "engine.Generate must be called")
	assert.Equal(t, "mcp", mock.calledWith.Mode,
		"engine must be called with Mode='mcp'")
	assert.Equal(t, "typescript", mock.calledWith.Target,
		"engine must be called with Target='typescript' (the default)")
}

// ---------------------------------------------------------------------------
// AC-15: generate mcp --target go -> error
// ---------------------------------------------------------------------------

func TestGenerateMCP_TargetGo_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	path := writeGenerateManifest(t, dir, generateManifest("pet-store"))

	mock := &mockCodeGenerator{}
	cfg := &generateConfig{Engine: mock}

	_, err := executeGenerateCmd(cfg, "mcp", "--manifest", path, "--target", "go")
	require.Error(t, err,
		"generate mcp --target go must return an error")
	assert.Contains(t, strings.ToLower(err.Error()), "not yet implemented",
		"error message must contain 'not yet implemented'; got: %s", err.Error())
}

func TestGenerateMCP_TargetGo_EngineNotCalled(t *testing.T) {
	dir := t.TempDir()
	path := writeGenerateManifest(t, dir, generateManifest("pet-store"))

	mock := &mockCodeGenerator{}
	cfg := &generateConfig{Engine: mock}

	_, _ = executeGenerateCmd(cfg, "mcp", "--manifest", path, "--target", "go")
	assert.False(t, mock.called,
		"engine.Generate must NOT be called when target is unsupported")
}

func TestGenerateMCP_TargetGo_ErrorMentionsGo(t *testing.T) {
	dir := t.TempDir()
	path := writeGenerateManifest(t, dir, generateManifest("pet-store"))

	mock := &mockCodeGenerator{}
	cfg := &generateConfig{Engine: mock}

	_, err := executeGenerateCmd(cfg, "mcp", "--manifest", path, "--target", "go")
	require.Error(t, err)
	errLower := strings.ToLower(err.Error())
	assert.Contains(t, errLower, "go",
		"error message must mention 'go' so the user knows which target failed")
}

// ---------------------------------------------------------------------------
// AC-15: generate mcp --transport
// ---------------------------------------------------------------------------

func TestGenerateMCP_TransportStdio_Accepted(t *testing.T) {
	dir := t.TempDir()
	path := writeGenerateManifest(t, dir, generateManifest("pet-store"))

	mock := &mockCodeGenerator{
		result: &codegen.GenerateResult{
			Files:  []string{"src/index.ts"},
			DryRun: false,
			Mode:   "mcp",
			Target: "typescript",
		},
	}
	cfg := &generateConfig{Engine: mock}

	_, err := executeGenerateCmd(cfg, "mcp", "--manifest", path, "--transport", "stdio")
	require.NoError(t, err,
		"generate mcp --transport stdio must not error")
	require.True(t, mock.called, "engine.Generate must be called")
}

func TestGenerateMCP_TransportDual_Accepted(t *testing.T) {
	dir := t.TempDir()
	path := writeGenerateManifest(t, dir, generateManifest("pet-store"))

	mock := &mockCodeGenerator{
		result: &codegen.GenerateResult{
			Files:  []string{"src/index.ts"},
			DryRun: false,
			Mode:   "mcp",
			Target: "typescript",
		},
	}
	cfg := &generateConfig{Engine: mock}

	_, err := executeGenerateCmd(cfg, "mcp", "--manifest", path,
		"--transport", "stdio,streamable-http")
	require.NoError(t, err,
		"generate mcp --transport stdio,streamable-http must not error")
	require.True(t, mock.called, "engine.Generate must be called")
}

// ---------------------------------------------------------------------------
// AC-15: generate mcp --dry-run
// ---------------------------------------------------------------------------

func TestGenerateMCP_DryRun_EngineCalledWithDryRunTrue(t *testing.T) {
	dir := t.TempDir()
	path := writeGenerateManifest(t, dir, generateManifest("pet-store"))

	mock := &mockCodeGenerator{
		result: &codegen.GenerateResult{
			Files:  []string{"src/index.ts", "package.json"},
			DryRun: true,
			Mode:   "mcp",
			Target: "typescript",
		},
	}
	cfg := &generateConfig{Engine: mock}

	_, err := executeGenerateCmd(cfg, "mcp", "--manifest", path, "--dry-run")
	require.NoError(t, err)

	require.True(t, mock.called, "engine.Generate must be called even in dry-run mode")
	assert.True(t, mock.calledWith.DryRun,
		"engine must be called with DryRun=true when --dry-run is set")
}

func TestGenerateMCP_DryRun_OutputListsFiles(t *testing.T) {
	dir := t.TempDir()
	path := writeGenerateManifest(t, dir, generateManifest("pet-store"))

	mock := &mockCodeGenerator{
		result: &codegen.GenerateResult{
			Files:  []string{"src/index.ts", "package.json", "tsconfig.json"},
			DryRun: true,
			Mode:   "mcp",
			Target: "typescript",
		},
	}
	cfg := &generateConfig{Engine: mock}

	stdout, err := executeGenerateCmd(cfg, "mcp", "--manifest", path, "--dry-run")
	require.NoError(t, err)

	assert.Contains(t, stdout, "src/index.ts",
		"dry-run output must list generated files")
	assert.Contains(t, stdout, "package.json",
		"dry-run output must list all generated files")
	assert.Contains(t, stdout, "tsconfig.json",
		"dry-run output must list all generated files")
}

// ---------------------------------------------------------------------------
// AC-15: generate mcp --json -> JSON output
// ---------------------------------------------------------------------------

func TestGenerateMCP_JSON_OutputIsValidJSON(t *testing.T) {
	dir := t.TempDir()
	path := writeGenerateManifest(t, dir, generateManifest("pet-store"))

	mock := &mockCodeGenerator{
		result: &codegen.GenerateResult{
			Files:  []string{"src/index.ts", "package.json"},
			DryRun: false,
			Mode:   "mcp",
			Target: "typescript",
		},
	}
	cfg := &generateConfig{Engine: mock}

	stdout, err := executeGenerateCmd(cfg, "mcp", "--manifest", path, "--json")
	require.NoError(t, err)

	require.True(t, json.Valid([]byte(stdout)),
		"--json output must be valid JSON; got: %s", stdout)
}

func TestGenerateMCP_JSON_ContainsModeAndTarget(t *testing.T) {
	dir := t.TempDir()
	path := writeGenerateManifest(t, dir, generateManifest("pet-store"))

	mock := &mockCodeGenerator{
		result: &codegen.GenerateResult{
			Files:  []string{"src/index.ts"},
			DryRun: false,
			Mode:   "mcp",
			Target: "typescript",
		},
	}
	cfg := &generateConfig{Engine: mock}

	stdout, err := executeGenerateCmd(cfg, "mcp", "--manifest", path, "--json")
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &got))

	assert.Equal(t, "mcp", got["mode"],
		"JSON output must include mode='mcp'")
	assert.Equal(t, "typescript", got["target"],
		"JSON output must include target='typescript'")
}

// ---------------------------------------------------------------------------
// AC-15: generate mcp --output overrides default dir
// ---------------------------------------------------------------------------

func TestGenerateMCP_OutputFlagOverridesDefault(t *testing.T) {
	dir := t.TempDir()
	path := writeGenerateManifest(t, dir, generateManifest("pet-store"))
	customDir := filepath.Join(dir, "my-mcp-output")

	mock := &mockCodeGenerator{
		result: &codegen.GenerateResult{
			Files:  []string{"src/index.ts"},
			DryRun: false,
			Mode:   "mcp",
			Target: "typescript",
		},
	}
	cfg := &generateConfig{Engine: mock}

	_, err := executeGenerateCmd(cfg, "mcp", "--manifest", path, "--output", customDir)
	require.NoError(t, err)

	require.True(t, mock.called)
	assert.Equal(t, customDir, mock.calledWith.OutputDir,
		"--output must override the default output dir")
}

// ---------------------------------------------------------------------------
// Error handling: manifest not found
// ---------------------------------------------------------------------------

func TestGenerateCLI_ManifestNotFound_ReturnsError(t *testing.T) {
	mock := &mockCodeGenerator{}
	cfg := &generateConfig{Engine: mock}

	_, err := executeGenerateCmd(cfg, "cli", "--manifest", "/nonexistent/path/toolwright.yaml")
	require.Error(t, err,
		"generate cli with a nonexistent manifest must return an error")
	assert.False(t, mock.called,
		"engine.Generate must NOT be called when manifest is not found")
}

func TestGenerateMCP_ManifestNotFound_ReturnsError(t *testing.T) {
	mock := &mockCodeGenerator{}
	cfg := &generateConfig{Engine: mock}

	_, err := executeGenerateCmd(cfg, "mcp", "--manifest", "/nonexistent/path/toolwright.yaml")
	require.Error(t, err,
		"generate mcp with a nonexistent manifest must return an error")
	assert.False(t, mock.called,
		"engine.Generate must NOT be called when manifest is not found")
}

// ---------------------------------------------------------------------------
// Error handling: --json with manifest not found -> JSON error
// ---------------------------------------------------------------------------

func TestGenerateCLI_ManifestNotFound_JSON_HasErrorStructure(t *testing.T) {
	mock := &mockCodeGenerator{}
	cfg := &generateConfig{Engine: mock}

	stdout, _ := executeGenerateCmd(cfg, "cli", "--json", "--manifest", "/nonexistent/toolwright.yaml")

	require.NotEmpty(t, stdout,
		"JSON output must be produced even for manifest-not-found errors")

	require.True(t, json.Valid([]byte(stdout)),
		"output with --json must be valid JSON even for errors; got: %s", stdout)

	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &got))

	errObj, ok := got["error"].(map[string]any)
	require.True(t, ok,
		"JSON error output must have top-level 'error' object; got: %v", got)
	assert.Contains(t, errObj, "code",
		"error object must have 'code' field")
	assert.Contains(t, errObj, "message",
		"error object must have 'message' field")
}

func TestGenerateMCP_ManifestNotFound_JSON_HasErrorStructure(t *testing.T) {
	mock := &mockCodeGenerator{}
	cfg := &generateConfig{Engine: mock}

	stdout, _ := executeGenerateCmd(cfg, "mcp", "--json", "--manifest", "/nonexistent/toolwright.yaml")

	require.NotEmpty(t, stdout,
		"JSON output must be produced even for manifest-not-found errors")

	require.True(t, json.Valid([]byte(stdout)),
		"output with --json must be valid JSON even for errors; got: %s", stdout)

	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &got))

	errObj, ok := got["error"].(map[string]any)
	require.True(t, ok,
		"JSON error output must have top-level 'error' object; got: %v", got)
	assert.Contains(t, errObj, "code",
		"error object must have 'code' field")
	assert.Contains(t, errObj, "message",
		"error object must have 'message' field")
}

// ---------------------------------------------------------------------------
// Anti-hardcoding: different manifests produce different output dirs
// ---------------------------------------------------------------------------

func TestGenerate_DifferentManifests_DifferentOutputDirs(t *testing.T) {
	tests := []struct {
		name      string
		mode      string
		toolkit   string
		wantInDir string
	}{
		{
			name:      "cli with pet-store",
			mode:      "cli",
			toolkit:   "pet-store",
			wantInDir: "cli-pet-store",
		},
		{
			name:      "cli with weather-api",
			mode:      "cli",
			toolkit:   "weather-api",
			wantInDir: "cli-weather-api",
		},
		{
			name:      "mcp with pet-store",
			mode:      "mcp",
			toolkit:   "pet-store",
			wantInDir: "mcp-server-pet-store",
		},
		{
			name:      "mcp with weather-api",
			mode:      "mcp",
			toolkit:   "weather-api",
			wantInDir: "mcp-server-weather-api",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := writeGenerateManifest(t, dir, generateManifest(tc.toolkit))

			mock := &mockCodeGenerator{
				result: &codegen.GenerateResult{
					Files:  []string{"placeholder.txt"},
					DryRun: false,
					Mode:   tc.mode,
					Target: "go",
				},
			}
			cfg := &generateConfig{Engine: mock}

			_, err := executeGenerateCmd(cfg, tc.mode, "--manifest", path)
			require.NoError(t, err)

			require.True(t, mock.called)
			assert.Contains(t, mock.calledWith.OutputDir, tc.wantInDir,
				"output dir must contain %q for toolkit %q in mode %q; got %q",
				tc.wantInDir, tc.toolkit, tc.mode, mock.calledWith.OutputDir)
		})
	}
}

// ---------------------------------------------------------------------------
// AC-14: generate cli - engine receives correct manifest data
// ---------------------------------------------------------------------------

func TestGenerateCLI_EngineReceivesManifestData(t *testing.T) {
	dir := t.TempDir()
	path := writeGenerateManifest(t, dir, generateManifest("pet-store"))

	mock := &mockCodeGenerator{
		result: &codegen.GenerateResult{
			Files:  []string{"main.go"},
			DryRun: false,
			Mode:   "cli",
			Target: "go",
		},
	}
	cfg := &generateConfig{Engine: mock}

	_, err := executeGenerateCmd(cfg, "cli", "--manifest", path)
	require.NoError(t, err)

	require.True(t, mock.called)
	require.NotNil(t, mock.manifest, "engine must receive the parsed manifest")
	assert.Equal(t, "pet-store", mock.manifest.Metadata.Name,
		"engine must receive the correct toolkit; got name=%q", mock.manifest.Metadata.Name)
}

func TestGenerateMCP_EngineReceivesManifestData(t *testing.T) {
	dir := t.TempDir()
	path := writeGenerateManifest(t, dir, generateManifest("weather-api"))

	mock := &mockCodeGenerator{
		result: &codegen.GenerateResult{
			Files:  []string{"src/index.ts"},
			DryRun: false,
			Mode:   "mcp",
			Target: "typescript",
		},
	}
	cfg := &generateConfig{Engine: mock}

	_, err := executeGenerateCmd(cfg, "mcp", "--manifest", path)
	require.NoError(t, err)

	require.True(t, mock.called)
	require.NotNil(t, mock.manifest, "engine must receive the parsed manifest")
	assert.Equal(t, "weather-api", mock.manifest.Metadata.Name,
		"engine must receive the correct toolkit; got name=%q", mock.manifest.Metadata.Name)
}

// ---------------------------------------------------------------------------
// AC-14: generate cli - engine error + --json -> JSON error output
// ---------------------------------------------------------------------------

func TestGenerateCLI_EngineError_JSON_HasErrorStructure(t *testing.T) {
	dir := t.TempDir()
	path := writeGenerateManifest(t, dir, generateManifest("pet-store"))

	engineErr := errors.New("template rendering failed")
	mock := &mockCodeGenerator{err: engineErr}
	cfg := &generateConfig{Engine: mock}

	stdout, err := executeGenerateCmd(cfg, "cli", "--json", "--manifest", path)
	require.Error(t, err, "engine error must still propagate")

	require.NotEmpty(t, stdout,
		"JSON error output must be produced when engine fails and --json is set")
	require.True(t, json.Valid([]byte(stdout)),
		"output must be valid JSON; got: %s", stdout)

	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &got))

	errObj, ok := got["error"].(map[string]any)
	require.True(t, ok,
		"JSON error output must have 'error' object; got: %v", got)
	assert.Contains(t, errObj, "code")
	assert.Contains(t, errObj, "message")
}

// ---------------------------------------------------------------------------
// AC-15: generate mcp - engine error + --json -> JSON error output
// ---------------------------------------------------------------------------

func TestGenerateMCP_EngineError_JSON_HasErrorStructure(t *testing.T) {
	dir := t.TempDir()
	path := writeGenerateManifest(t, dir, generateManifest("pet-store"))

	engineErr := errors.New("template rendering failed")
	mock := &mockCodeGenerator{err: engineErr}
	cfg := &generateConfig{Engine: mock}

	stdout, err := executeGenerateCmd(cfg, "mcp", "--json", "--manifest", path)
	require.Error(t, err, "engine error must still propagate")

	require.NotEmpty(t, stdout,
		"JSON error output must be produced when engine fails and --json is set")
	require.True(t, json.Valid([]byte(stdout)),
		"output must be valid JSON; got: %s", stdout)

	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &got))

	errObj, ok := got["error"].(map[string]any)
	require.True(t, ok,
		"JSON error output must have 'error' object; got: %v", got)
	assert.Contains(t, errObj, "code")
	assert.Contains(t, errObj, "message")
}

// ---------------------------------------------------------------------------
// generate cli --json --dry-run -> JSON output with dry_run field
// ---------------------------------------------------------------------------

func TestGenerateCLI_JSON_DryRun_HasDryRunField(t *testing.T) {
	dir := t.TempDir()
	path := writeGenerateManifest(t, dir, generateManifest("pet-store"))

	mock := &mockCodeGenerator{
		result: &codegen.GenerateResult{
			Files:  []string{"main.go"},
			DryRun: true,
			Mode:   "cli",
			Target: "go",
		},
	}
	cfg := &generateConfig{Engine: mock}

	stdout, err := executeGenerateCmd(cfg, "cli", "--manifest", path, "--json", "--dry-run")
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &got))

	dryRun, ok := got["dry_run"].(bool)
	require.True(t, ok,
		"JSON output must have a 'dry_run' boolean field; got: %v", got)
	assert.True(t, dryRun,
		"dry_run must be true when --dry-run is set")
}

// ---------------------------------------------------------------------------
// Inherited --json flag from root
// ---------------------------------------------------------------------------

func TestGenerateCLI_InheritsJsonFlag(t *testing.T) {
	cfg := &generateConfig{}
	root := NewRootCommand()
	gen := newGenerateCmd(cfg)
	root.AddCommand(gen)

	cli := findGenerateSubcommand(t, gen, "cli")

	// json is a persistent flag on root; it should be accessible via InheritedFlags
	// after InitDefaultHelpCmd is called. We trigger that by executing --help.
	root.SetArgs([]string{"generate", "cli", "--help"})
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	_ = root.Execute()

	f := cli.Flags().Lookup("json")
	if f == nil {
		f = cli.InheritedFlags().Lookup("json")
	}
	require.NotNil(t, f,
		"--json must be accessible on generate cli via root's persistent flags")
}

func TestGenerateMCP_InheritsJsonFlag(t *testing.T) {
	cfg := &generateConfig{}
	root := NewRootCommand()
	gen := newGenerateCmd(cfg)
	root.AddCommand(gen)

	mcp := findGenerateSubcommand(t, gen, "mcp")

	root.SetArgs([]string{"generate", "mcp", "--help"})
	root.SetOut(&bytes.Buffer{})
	root.SetErr(&bytes.Buffer{})
	_ = root.Execute()

	f := mcp.Flags().Lookup("json")
	if f == nil {
		f = mcp.InheritedFlags().Lookup("json")
	}
	require.NotNil(t, f,
		"--json must be accessible on generate mcp via root's persistent flags")
}

// ---------------------------------------------------------------------------
// Default --manifest behavior (defaults to toolwright.yaml)
// ---------------------------------------------------------------------------

func TestGenerateCLI_DefaultManifest_UsesToolwrightYaml(t *testing.T) {
	dir := t.TempDir()
	writeGenerateManifest(t, dir, generateManifest("default-toolkit"))

	original, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(dir))
	t.Cleanup(func() { _ = os.Chdir(original) })

	mock := &mockCodeGenerator{
		result: &codegen.GenerateResult{
			Files:  []string{"main.go"},
			DryRun: false,
			Mode:   "cli",
			Target: "go",
		},
	}
	cfg := &generateConfig{Engine: mock}

	_, err = executeGenerateCmd(cfg, "cli")
	require.NoError(t, err,
		"generate cli with no --manifest must default to toolwright.yaml and succeed if present")

	require.True(t, mock.called)
	assert.Equal(t, "default-toolkit", mock.manifest.Metadata.Name,
		"must read the default toolwright.yaml in cwd")
}

// ---------------------------------------------------------------------------
// AC-15: generate mcp - engine error propagated
// ---------------------------------------------------------------------------

func TestGenerateMCP_EngineError_Propagated(t *testing.T) {
	dir := t.TempDir()
	path := writeGenerateManifest(t, dir, generateManifest("pet-store"))

	engineErr := errors.New("npm install failed")
	mock := &mockCodeGenerator{err: engineErr}
	cfg := &generateConfig{Engine: mock}

	_, err := executeGenerateCmd(cfg, "mcp", "--manifest", path)
	require.Error(t, err, "engine error must be propagated")
	assert.Contains(t, err.Error(), "npm install failed",
		"propagated error must contain the engine's original error message")
}

// ---------------------------------------------------------------------------
// AC-15: generate mcp --force
// ---------------------------------------------------------------------------

func TestGenerateMCP_Force_EngineCalledWithForceTrue(t *testing.T) {
	dir := t.TempDir()
	path := writeGenerateManifest(t, dir, generateManifest("pet-store"))

	mock := &mockCodeGenerator{
		result: &codegen.GenerateResult{
			Files:  []string{"src/index.ts"},
			DryRun: false,
			Mode:   "mcp",
			Target: "typescript",
		},
	}
	cfg := &generateConfig{Engine: mock}

	_, err := executeGenerateCmd(cfg, "mcp", "--manifest", path, "--force")
	require.NoError(t, err)

	require.True(t, mock.called)
	assert.True(t, mock.calledWith.Force,
		"engine must be called with Force=true when --force is set")
}

// ---------------------------------------------------------------------------
// AC-14 + AC-15: JSON error output for unsupported target
// ---------------------------------------------------------------------------

func TestGenerateCLI_TargetTypescript_JSON_HasErrorStructure(t *testing.T) {
	dir := t.TempDir()
	path := writeGenerateManifest(t, dir, generateManifest("pet-store"))

	mock := &mockCodeGenerator{}
	cfg := &generateConfig{Engine: mock}

	stdout, err := executeGenerateCmd(cfg, "cli", "--json", "--manifest", path, "--target", "typescript")
	require.Error(t, err)

	require.NotEmpty(t, stdout,
		"JSON output must be produced for unsupported target errors when --json is set")
	require.True(t, json.Valid([]byte(stdout)),
		"output must be valid JSON; got: %s", stdout)

	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &got))

	_, ok := got["error"].(map[string]any)
	require.True(t, ok,
		"JSON error output must have 'error' object")
}

func TestGenerateMCP_TargetGo_JSON_HasErrorStructure(t *testing.T) {
	dir := t.TempDir()
	path := writeGenerateManifest(t, dir, generateManifest("pet-store"))

	mock := &mockCodeGenerator{}
	cfg := &generateConfig{Engine: mock}

	stdout, err := executeGenerateCmd(cfg, "mcp", "--json", "--manifest", path, "--target", "go")
	require.Error(t, err)

	require.NotEmpty(t, stdout,
		"JSON output must be produced for unsupported target errors when --json is set")
	require.True(t, json.Valid([]byte(stdout)),
		"output must be valid JSON; got: %s", stdout)

	var got map[string]any
	require.NoError(t, json.Unmarshal([]byte(stdout), &got))

	_, ok := got["error"].(map[string]any)
	require.True(t, ok,
		"JSON error output must have 'error' object")
}
