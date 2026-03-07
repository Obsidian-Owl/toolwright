package cli

import (
	"github.com/spf13/cobra"

	toolwright "github.com/Obsidian-Owl/toolwright"
	"github.com/Obsidian-Owl/toolwright/internal/auth"
	"github.com/Obsidian-Owl/toolwright/internal/codegen"
	"github.com/Obsidian-Owl/toolwright/internal/generate"
	"github.com/Obsidian-Owl/toolwright/internal/runner"
	"github.com/Obsidian-Owl/toolwright/internal/scaffold"
	"github.com/Obsidian-Owl/toolwright/internal/tooltest"
	"github.com/Obsidian-Owl/toolwright/internal/tui"
)

// parseDirAdapter adapts the tooltest.ParseTestDir free function to the
// testParser interface consumed by newTestCmd.
type parseDirAdapter struct{}

func (parseDirAdapter) ParseDir(dir string) ([]tooltest.TestSuite, error) {
	return tooltest.ParseTestDir(dir)
}

// BuildRootCommand creates the fully wired root command with all subcommands
// and production dependencies. This is the main entry point for the CLI binary.
func BuildRootCommand() *cobra.Command {
	root := NewRootCommand()
	root.Version = Version

	// Commands with no external dependencies.
	root.AddCommand(newValidateCmd())
	root.AddCommand(newListCmd())
	root.AddCommand(newDescribeCmd())

	// run: wires runner.Executor and auth.Resolver.
	runCfg := &runConfig{
		Runner: &runner.Executor{},
		Resolver: &auth.Resolver{
			Keyring: auth.NewFileStore(""),
			Store:   auth.NewFileStore(""),
		},
	}
	root.AddCommand(newRunCmd(runCfg))

	// test: wires tooltest.TestRunner and parseDirAdapter.
	testCfg := &testConfig{
		Runner: &tooltest.TestRunner{
			Executor: &runner.Executor{},
		},
		Parser: parseDirAdapter{},
	}
	root.AddCommand(newTestCmd(testCfg))

	// login: wires auth.Login.
	loginCfg := &loginConfig{
		Login: auth.Login,
	}
	root.AddCommand(newLoginCmd(loginCfg))

	// generate: wires codegen.Engine with CLI and MCP generators, plus manifest child.
	engine := codegen.NewEngine()
	engine.Register(codegen.NewGoCLIGenerator())
	engine.Register(codegen.NewTSMCPGenerator())
	genCfg := &generateConfig{Engine: engine}
	genCmd := newGenerateCmd(genCfg)
	genCmd.AddCommand(newGenerateManifestCmd(&manifestGenerateConfig{Generator: generate.NewGenerator()}))
	root.AddCommand(genCmd)

	// init: wires real scaffolder and wizard.
	initCfg := &initConfig{
		Scaffolder: scaffold.New(toolwright.InitTemplates),
		Wizard:     tui.NewWizard(isColorDisabled()),
	}
	root.AddCommand(newInitCmd(initCfg))

	// version: no external dependencies.
	root.AddCommand(newVersionCmd())

	return root
}
