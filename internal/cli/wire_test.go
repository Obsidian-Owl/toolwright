package cli

import (
	"bytes"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Obsidian-Owl/toolwright/internal/tui"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// cmdNames extracts subcommand names from a cobra.Command, excluding
// auto-added commands like help and completion.
func cmdNames(cmd *cobra.Command) []string {
	var names []string
	for _, sub := range cmd.Commands() {
		n := sub.Name()
		if n != "help" && n != "completion" {
			names = append(names, n)
		}
	}
	return names
}

// cmdNameSet returns subcommand names as a map for O(1) lookup.
func cmdNameSet(cmd *cobra.Command) map[string]bool {
	set := map[string]bool{}
	for _, sub := range cmd.Commands() {
		set[sub.Name()] = true
	}
	return set
}

// ---------------------------------------------------------------------------
// AC-19/AC-20: BuildRootCommand wires all subcommands with production deps
// ---------------------------------------------------------------------------

func TestBuildRootCommand_ReturnsNonNil(t *testing.T) {
	cmd := BuildRootCommand()
	require.NotNil(t, cmd, "BuildRootCommand must return a non-nil *cobra.Command")
}

func TestBuildRootCommand_RootUseIsToolwright(t *testing.T) {
	cmd := BuildRootCommand()
	require.NotNil(t, cmd, "precondition: BuildRootCommand must return non-nil")
	assert.Equal(t, "toolwright", cmd.Use,
		"root command Use field must be 'toolwright'")
}

// ---------------------------------------------------------------------------
// AC-19: All expected subcommands are registered on the root
// ---------------------------------------------------------------------------

func TestBuildRootCommand_HasAllSubcommands(t *testing.T) {
	cmd := BuildRootCommand()
	require.NotNil(t, cmd, "precondition: BuildRootCommand must return non-nil")

	registered := cmdNameSet(cmd)

	// All of these must be present. A lazy implementation that registers
	// only some commands must fail.
	expected := []string{
		"validate",
		"list",
		"describe",
		"run",
		"test",
		"login",
		"generate",
		"init",
		"version",
	}
	for _, name := range expected {
		assert.True(t, registered[name],
			"subcommand %q must be registered on the root command; got commands: %v",
			name, cmdNames(cmd))
	}
}

// Table-driven test: verify each expected subcommand individually so that a
// partial registration is caught per-command (constitution rule 9).
func TestBuildRootCommand_EachSubcommandRegistered_TableDriven(t *testing.T) {
	cmd := BuildRootCommand()
	require.NotNil(t, cmd, "precondition: BuildRootCommand must return non-nil")

	tests := []struct {
		name string
	}{
		{name: "validate"},
		{name: "list"},
		{name: "describe"},
		{name: "run"},
		{name: "test"},
		{name: "login"},
		{name: "generate"},
		{name: "init"},
		{name: "version"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			registered := cmdNameSet(cmd)
			assert.True(t, registered[tc.name],
				"subcommand %q must be registered; got: %v", tc.name, cmdNames(cmd))
		})
	}
}

// Verify the EXACT count of subcommands so that extra or missing commands are
// caught. A lazy implementation that adds one hardcoded command but misses
// others will fail.
func TestBuildRootCommand_SubcommandCount(t *testing.T) {
	cmd := BuildRootCommand()
	require.NotNil(t, cmd, "precondition: BuildRootCommand must return non-nil")

	names := cmdNames(cmd)

	// Exactly 9 subcommands: validate, list, describe, run, test, login,
	// generate, init, version.
	assert.Len(t, names, 9,
		"BuildRootCommand must register exactly 9 subcommands (excluding auto-added help/completion); got: %v", names)
}

// ---------------------------------------------------------------------------
// AC-19: generate subcommand has its own children (cli, mcp, manifest)
// ---------------------------------------------------------------------------

func TestBuildRootCommand_GenerateHasChildren(t *testing.T) {
	cmd := BuildRootCommand()
	require.NotNil(t, cmd, "precondition: BuildRootCommand must return non-nil")

	// Find the generate subcommand.
	var genSub *cobra.Command
	for _, sub := range cmd.Commands() {
		if sub.Name() == "generate" {
			genSub = sub
			break
		}
	}
	require.NotNil(t, genSub,
		"'generate' subcommand must exist on root command")

	genChildren := cmdNames(genSub)

	// generate must have cli, mcp, manifest children.
	expected := []string{"cli", "mcp", "manifest"}
	childSet := map[string]bool{}
	for _, n := range genChildren {
		childSet[n] = true
	}
	for _, name := range expected {
		assert.True(t, childSet[name],
			"generate must have child subcommand %q; got: %v", name, genChildren)
	}
}

func TestBuildRootCommand_GenerateChildCount(t *testing.T) {
	cmd := BuildRootCommand()
	require.NotNil(t, cmd, "precondition: BuildRootCommand must return non-nil")

	var genSub *cobra.Command
	for _, sub := range cmd.Commands() {
		if sub.Name() == "generate" {
			genSub = sub
			break
		}
	}
	require.NotNil(t, genSub,
		"'generate' subcommand must exist on root command")

	genChildren := cmdNames(genSub)

	assert.Len(t, genChildren, 3,
		"generate must have exactly 3 child subcommands (cli, mcp, manifest); got: %v", genChildren)
}

// ---------------------------------------------------------------------------
// AC-20: --help shows all command names in output
// ---------------------------------------------------------------------------

func TestBuildRootCommand_HelpShowsAllCommands(t *testing.T) {
	cmd := BuildRootCommand()
	require.NotNil(t, cmd, "precondition: BuildRootCommand must return non-nil")

	outBuf := &bytes.Buffer{}
	cmd.SetOut(outBuf)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"--help"})

	err := cmd.Execute()
	require.NoError(t, err, "--help must not return an error")

	helpOutput := outBuf.String()
	require.NotEmpty(t, helpOutput, "--help must produce output")

	// All subcommand names must appear in the help text.
	expected := []string{
		"validate",
		"list",
		"describe",
		"run",
		"test",
		"login",
		"generate",
		"init",
		"version",
	}
	for _, name := range expected {
		assert.Contains(t, helpOutput, name,
			"--help output must mention the %q subcommand", name)
	}
}

// ---------------------------------------------------------------------------
// AC-19: Root persistent flags are still present on the wired command
// ---------------------------------------------------------------------------

func TestBuildRootCommand_HasPersistentFlags(t *testing.T) {
	cmd := BuildRootCommand()
	require.NotNil(t, cmd, "precondition: BuildRootCommand must return non-nil")

	flags := []string{"json", "debug", "no-color"}
	for _, name := range flags {
		f := cmd.PersistentFlags().Lookup(name)
		assert.NotNil(t, f,
			"persistent flag --%s must exist on the root command returned by BuildRootCommand", name)
	}
}

// ---------------------------------------------------------------------------
// AC-19: Subcommands are not duplicated
// ---------------------------------------------------------------------------

func TestBuildRootCommand_NoDuplicateSubcommands(t *testing.T) {
	cmd := BuildRootCommand()
	require.NotNil(t, cmd, "precondition: BuildRootCommand must return non-nil")

	seen := map[string]int{}
	for _, sub := range cmd.Commands() {
		seen[sub.Name()]++
	}

	for name, count := range seen {
		assert.Equal(t, 1, count,
			"subcommand %q must be registered exactly once; found %d times", name, count)
	}
}

// ---------------------------------------------------------------------------
// AC-19: Each subcommand has a RunE or is a parent (has children)
// ---------------------------------------------------------------------------

func TestBuildRootCommand_SubcommandsAreRunnable(t *testing.T) {
	cmd := BuildRootCommand()
	require.NotNil(t, cmd, "precondition: BuildRootCommand must return non-nil")

	for _, sub := range cmd.Commands() {
		if sub.Name() == "help" || sub.Name() == "completion" {
			continue
		}

		hasRunE := sub.RunE != nil
		hasRun := sub.Run != nil
		hasChildren := sub.HasSubCommands()

		// A command must either be directly runnable or be a parent with children.
		assert.True(t, hasRunE || hasRun || hasChildren,
			"subcommand %q must be runnable (RunE/Run) or have child subcommands; it has neither",
			sub.Name())
	}
}

// ---------------------------------------------------------------------------
// AC-20: version subcommand is present and properly configured
// ---------------------------------------------------------------------------

func TestBuildRootCommand_VersionSubcommandExists(t *testing.T) {
	cmd := BuildRootCommand()
	require.NotNil(t, cmd, "precondition: BuildRootCommand must return non-nil")

	var versionCmd *cobra.Command
	for _, sub := range cmd.Commands() {
		if sub.Name() == "version" {
			versionCmd = sub
			break
		}
	}
	require.NotNil(t, versionCmd,
		"version subcommand must be registered on the root command")
	assert.NotEmpty(t, versionCmd.Short,
		"version subcommand must have a Short description")
	assert.NotNil(t, versionCmd.RunE,
		"version subcommand must have a RunE function")
}

// ---------------------------------------------------------------------------
// AC-19: All subcommands have Short descriptions (thin but complete)
// ---------------------------------------------------------------------------

func TestBuildRootCommand_AllSubcommandsHaveShortDescription(t *testing.T) {
	cmd := BuildRootCommand()
	require.NotNil(t, cmd, "precondition: BuildRootCommand must return non-nil")

	for _, sub := range cmd.Commands() {
		if sub.Name() == "help" || sub.Name() == "completion" {
			continue
		}
		assert.NotEmpty(t, sub.Short,
			"subcommand %q must have a non-empty Short description", sub.Name())
	}
}

// ---------------------------------------------------------------------------
// AC-20: BuildRootCommand returns independent instances (no global state)
// ---------------------------------------------------------------------------

func TestBuildRootCommand_ReturnsIndependentInstances(t *testing.T) {
	cmd1 := BuildRootCommand()
	cmd2 := BuildRootCommand()
	require.NotNil(t, cmd1)
	require.NotNil(t, cmd2)

	// They must be different pointers -- two calls must not return the same
	// mutable command tree.
	assert.NotSame(t, cmd1, cmd2,
		"BuildRootCommand must return a new command instance each call (no global state)")
}

// ---------------------------------------------------------------------------
// AC-7: BuildRootCommand wires REAL implementations (not nil)
// AC-8: Nil-guards are removed — commands must not return "not yet implemented"
// ---------------------------------------------------------------------------

// TestBuildRootCommand_InitHasRealScaffolder verifies that BuildRootCommand
// wires a real scaffolder into the init command. The current code passes
// Scaffolder: nil, so "init my-proj --yes" hits the nil-guard and returns
// "project scaffolding is not yet implemented". This test MUST FAIL against
// the current code because the scaffolder IS nil.
func TestBuildRootCommand_InitHasRealScaffolder(t *testing.T) {
	cmd := BuildRootCommand()
	require.NotNil(t, cmd)

	// Wire output to buffers so we capture everything.
	outBuf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	cmd.SetOut(outBuf)
	cmd.SetErr(errBuf)

	// Use --yes to skip the wizard (non-interactive) and point output at a
	// temp dir so that a real scaffolder writes somewhere safe.
	tmpDir := t.TempDir()
	cmd.SetArgs([]string{"init", "test-proj", "--yes", "--output", tmpDir})

	err := cmd.Execute()

	// The key assertion: even if the real scaffolder fails for some other
	// reason (e.g., template error), the error must NOT be the nil-guard
	// sentinel message. If the scaffolder is wired, we will never see this
	// message.
	if err != nil {
		assert.NotContains(t, err.Error(), "project scaffolding is not yet implemented",
			"init must not return the nil-guard error — BuildRootCommand must wire a real scaffolder")
		assert.NotContains(t, err.Error(), "not yet implemented",
			"init must not return any 'not yet implemented' error — all deps must be wired")
	}
	// If err is nil, the scaffolder ran successfully — that also passes.
}

// TestBuildRootCommand_InitHasRealWizard verifies that BuildRootCommand
// wires a real wizard into the init command. The current code passes
// Wizard: nil, so running init without --yes hits the nil-guard:
// "interactive wizard is not yet implemented".
// We test this by running init WITHOUT --yes and checking the error is
// not the nil-guard sentinel. In a non-TTY test environment the wizard
// may fail for other reasons (no terminal), but it must NOT be because
// the wizard is nil.
func TestBuildRootCommand_InitHasRealWizard(t *testing.T) {
	// Ensure we are not in CI mode so the wizard path is taken.
	t.Setenv("CI", "")

	cmd := BuildRootCommand()
	require.NotNil(t, cmd)

	outBuf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	cmd.SetOut(outBuf)
	cmd.SetErr(errBuf)

	// Run init without --yes to trigger the wizard path.
	cmd.SetArgs([]string{"init", "test-proj"})

	err := cmd.Execute()
	// The wizard will likely error in a non-TTY test environment, but the
	// error must NOT be the nil-guard message.
	if err != nil {
		assert.NotContains(t, err.Error(), "interactive wizard is not yet implemented",
			"init without --yes must not return the nil-guard error — BuildRootCommand must wire a real wizard")
		assert.NotContains(t, err.Error(), "not yet implemented",
			"init must not return any 'not yet implemented' error — wizard must be wired")
	}
}

// TestBuildRootCommand_GenerateManifestHasRealGenerator verifies that
// BuildRootCommand wires a real ManifestGenerator into the generate manifest
// subcommand. The current code passes Generator: nil, so running
// "generate manifest --provider anthropic --dry-run" returns
// "AI manifest generation is not yet implemented". This test MUST FAIL
// against the current code.
func TestBuildRootCommand_GenerateManifestHasRealGenerator(t *testing.T) {
	cmd := BuildRootCommand()
	require.NotNil(t, cmd)

	outBuf := &bytes.Buffer{}
	errBuf := &bytes.Buffer{}
	cmd.SetOut(outBuf)
	cmd.SetErr(errBuf)

	// Use --dry-run so nothing is written to disk, and provide a valid
	// provider. The real generator needs an API key, so it may fail with
	// an auth error — but it must NOT fail with the nil-guard sentinel.
	cmd.SetArgs([]string{"generate", "manifest", "--provider", "anthropic", "--dry-run"})

	err := cmd.Execute()
	// The generator will likely fail (no API key in test env), but the error
	// must NOT be the nil-guard message.
	if err != nil {
		assert.NotContains(t, err.Error(), "AI manifest generation is not yet implemented",
			"generate manifest must not return the nil-guard error — BuildRootCommand must wire a real generator")
		assert.NotContains(t, err.Error(), "not yet implemented",
			"generate manifest must not return any 'not yet implemented' error — generator must be wired")
	}
}

// TestBuildRootCommand_NoNilPanics_InitWithYes ensures that calling init
// through the fully-wired command tree does not panic due to nil deps.
// This is a regression safeguard — it verifies structural soundness.
func TestBuildRootCommand_NoNilPanics_InitWithYes(t *testing.T) {
	cmd := BuildRootCommand()
	require.NotNil(t, cmd)

	outBuf := &bytes.Buffer{}
	cmd.SetOut(outBuf)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"init", "panic-check-proj", "--yes", "--output", t.TempDir()})

	// This must not panic. We use require.NotPanics to catch nil-pointer
	// dereferences from unwired deps.
	require.NotPanics(t, func() {
		_ = cmd.Execute()
	}, "init --yes must not panic — all deps must be non-nil")
}

// TestBuildRootCommand_NoNilPanics_GenerateManifest ensures that calling
// generate manifest through the fully-wired command tree does not panic.
func TestBuildRootCommand_NoNilPanics_GenerateManifest(t *testing.T) {
	cmd := BuildRootCommand()
	require.NotNil(t, cmd)

	outBuf := &bytes.Buffer{}
	cmd.SetOut(outBuf)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"generate", "manifest", "--provider", "anthropic", "--dry-run"})

	require.NotPanics(t, func() {
		_ = cmd.Execute()
	}, "generate manifest must not panic — generator must be non-nil")
}

// TestBuildRootCommand_InitErrorIsNotNilGuard_MultipleRuntimes exercises
// the init command with various runtimes through the fully-wired tree to
// ensure none of them hit the nil-guard. A lazy implementation that only
// wires for one runtime would be caught.
func TestBuildRootCommand_InitErrorIsNotNilGuard_MultipleRuntimes(t *testing.T) {
	runtimes := []string{"shell", "go", "python", "typescript"}
	for _, rt := range runtimes {
		t.Run(rt, func(t *testing.T) {
			cmd := BuildRootCommand()
			require.NotNil(t, cmd)

			outBuf := &bytes.Buffer{}
			cmd.SetOut(outBuf)
			cmd.SetErr(&bytes.Buffer{})
			cmd.SetArgs([]string{"init", "proj-" + rt, "--yes", "--runtime", rt, "--output", t.TempDir()})

			err := cmd.Execute()
			if err != nil {
				assert.NotContains(t, err.Error(), "not yet implemented",
					"init --yes --runtime %s must not hit nil-guard", rt)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// AC-9: WizardResult.Name field must be removed
// ---------------------------------------------------------------------------

// TestWizardResult_HasNoNameField uses reflection to verify the WizardResult
// struct does NOT have a Name field. The current struct has Name as the first
// field, so this test MUST FAIL against the current code.
func TestWizardResult_HasNoNameField(t *testing.T) {
	rt := reflect.TypeOf(tui.WizardResult{})

	_, hasName := rt.FieldByName("Name")
	assert.False(t, hasName,
		"WizardResult must NOT have a Name field — the wizard never sets it "+
			"(name comes from the positional arg); found field Name on WizardResult")
}

// TestWizardResult_OnlyHasExpectedFields verifies the exact set of fields
// in WizardResult. After removing Name, only Description, Runtime, and Auth
// should remain. This catches partial removals or accidental additions.
func TestWizardResult_OnlyHasExpectedFields(t *testing.T) {
	rt := reflect.TypeOf(tui.WizardResult{})

	expectedFields := map[string]bool{
		"Description": true,
		"Runtime":     true,
		"Auth":        true,
	}

	actualFields := make(map[string]bool)
	for i := 0; i < rt.NumField(); i++ {
		actualFields[rt.Field(i).Name] = true
	}

	// Check no unexpected fields exist.
	for name := range actualFields {
		assert.True(t, expectedFields[name],
			"WizardResult has unexpected field %q — only Description, Runtime, Auth are expected", name)
	}

	// Check all expected fields exist.
	for name := range expectedFields {
		assert.True(t, actualFields[name],
			"WizardResult is missing expected field %q", name)
	}

	assert.Equal(t, len(expectedFields), rt.NumField(),
		"WizardResult must have exactly 3 fields (Description, Runtime, Auth), got %d", rt.NumField())
}

// TestWizardResult_FieldCount is a belt-and-suspenders check: the struct
// must have exactly 3 fields after Name is removed (currently it has 4).
func TestWizardResult_FieldCount(t *testing.T) {
	rt := reflect.TypeOf(tui.WizardResult{})
	assert.Equal(t, 3, rt.NumField(),
		"WizardResult must have exactly 3 fields (Description, Runtime, Auth); "+
			"currently has %d — the Name field should be removed", rt.NumField())
}

// ---------------------------------------------------------------------------
// AC-8: Nil-guard sentinel messages must not appear in the source code
// These tests verify the BEHAVIOR: running through BuildRootCommand must
// never produce the sentinel error strings. The above tests cover this.
// Below we add specific assertions about the exact sentinel strings to
// ensure we catch the EXACT nil-guard patterns.
// ---------------------------------------------------------------------------

// TestBuildRootCommand_InitScaffolderError_NotSentinel runs init through
// the production wiring and verifies the exact sentinel string from the
// nil-guard at init.go line 155 is absent from any error.
func TestBuildRootCommand_InitScaffolderError_NotSentinel(t *testing.T) {
	cmd := BuildRootCommand()
	require.NotNil(t, cmd)

	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"init", "sentinel-check", "--yes", "--output", t.TempDir()})

	err := cmd.Execute()
	if err != nil {
		// These are the exact sentinel strings from the nil-guards.
		assert.NotEqual(t, "project scaffolding is not yet implemented", err.Error(),
			"init must not return the exact scaffolder nil-guard sentinel")
		assert.NotEqual(t, "interactive wizard is not yet implemented; use --yes or set CI=true", err.Error(),
			"init must not return the exact wizard nil-guard sentinel")
	}
}

// TestBuildRootCommand_GenerateManifestError_NotSentinel runs generate
// manifest through the production wiring and verifies the exact sentinel
// string from generate_manifest.go line 94 is absent.
func TestBuildRootCommand_GenerateManifestError_NotSentinel(t *testing.T) {
	cmd := BuildRootCommand()
	require.NotNil(t, cmd)

	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"generate", "manifest", "--provider", "anthropic", "--dry-run"})

	err := cmd.Execute()
	if err != nil {
		assert.NotEqual(t, "AI manifest generation is not yet implemented", err.Error(),
			"generate manifest must not return the exact generator nil-guard sentinel")
	}
}

// ---------------------------------------------------------------------------
// AC-11: Binary builds and all commands are findable (regression)
// ---------------------------------------------------------------------------

// TestBuildRootCommand_InitSubcommandFindable verifies that the init
// subcommand can be found by traversal in the fully-wired command tree.
func TestBuildRootCommand_InitSubcommandFindable(t *testing.T) {
	cmd := BuildRootCommand()
	require.NotNil(t, cmd)

	var initCmd *cobra.Command
	for _, sub := range cmd.Commands() {
		if sub.Name() == "init" {
			initCmd = sub
			break
		}
	}

	require.NotNil(t, initCmd,
		"init subcommand must be findable in the BuildRootCommand tree")
	assert.NotNil(t, initCmd.RunE,
		"init subcommand must have a RunE function (it is directly runnable)")
}

// TestBuildRootCommand_GenerateManifestSubcommandFindable verifies that
// generate manifest can be found by traversal.
func TestBuildRootCommand_GenerateManifestSubcommandFindable(t *testing.T) {
	cmd := BuildRootCommand()
	require.NotNil(t, cmd)

	var genCmd *cobra.Command
	for _, sub := range cmd.Commands() {
		if sub.Name() == "generate" {
			genCmd = sub
			break
		}
	}
	require.NotNil(t, genCmd,
		"generate subcommand must exist on root")

	var manifestCmd *cobra.Command
	for _, sub := range genCmd.Commands() {
		if sub.Name() == "manifest" {
			manifestCmd = sub
			break
		}
	}
	require.NotNil(t, manifestCmd,
		"manifest subcommand must be findable under generate")
	assert.NotNil(t, manifestCmd.RunE,
		"generate manifest subcommand must have a RunE function")
}

// TestBuildRootCommand_InitHelpDoesNotMentionNotImplemented verifies that
// even the help output for init does not leak "not yet implemented".
func TestBuildRootCommand_InitHelpDoesNotMentionNotImplemented(t *testing.T) {
	cmd := BuildRootCommand()
	require.NotNil(t, cmd)

	outBuf := &bytes.Buffer{}
	cmd.SetOut(outBuf)
	cmd.SetErr(&bytes.Buffer{})
	cmd.SetArgs([]string{"init", "--help"})

	err := cmd.Execute()
	require.NoError(t, err, "init --help must not error")

	helpText := outBuf.String()
	assert.NotContains(t, strings.ToLower(helpText), "not yet implemented",
		"init help text must not mention 'not yet implemented'")
}
