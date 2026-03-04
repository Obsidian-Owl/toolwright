package cli

import (
	"bytes"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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
