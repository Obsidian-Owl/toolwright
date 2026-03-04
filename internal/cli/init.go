package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"
)

// ScaffoldOptions describes what the scaffolder should create.
type ScaffoldOptions struct {
	Name        string
	Description string
	OutputDir   string
	Runtime     string
	Auth        string
}

// ScaffoldResult describes what the scaffolder created.
type ScaffoldResult struct {
	Dir   string
	Files []string
}

// scaffolder abstracts project scaffolding so the CLI layer can be tested
// without real filesystem writes or embedded templates.
type scaffolder interface {
	Scaffold(ctx context.Context, opts ScaffoldOptions) (*ScaffoldResult, error)
}

// WizardResult describes the user's choices from the TUI wizard.
type WizardResult struct {
	Name        string
	Description string
	Runtime     string
	Auth        string
}

// initWizard abstracts the TUI wizard interaction.
type initWizard interface {
	Run(ctx context.Context) (*WizardResult, error)
}

// initConfig holds injectable dependencies for the init command.
type initConfig struct {
	Scaffolder scaffolder
	Wizard     initWizard
}

// initResultOutput is the JSON shape for a successful init result.
type initResultOutput struct {
	Dir   string   `json:"dir"`
	Files []string `json:"files"`
}

// validRuntimes is the set of accepted runtime values.
var validRuntimes = map[string]bool{
	"shell":      true,
	"go":         true,
	"python":     true,
	"typescript": true,
}

// newInitCmd returns the init subcommand. cfg provides injectable dependencies;
// in production the scaffolder and wizard are wired to real implementations.
func newInitCmd(cfg *initConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init <project-name>",
		Short: "Initialize a new toolwright project",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runInit(cmd, args, cfg)
		},
	}

	cmd.Flags().BoolP("yes", "y", false, "skip interactive wizard and use defaults")
	cmd.Flags().StringP("runtime", "r", "shell", "runtime for the project (shell, go, python, typescript)")
	cmd.Flags().StringP("description", "d", "", "short description of the project")
	cmd.Flags().StringP("output", "o", "", "output directory (default: current directory)")

	return cmd
}

func runInit(cmd *cobra.Command, args []string, cfg *initConfig) error {
	jsonMode, _ := cmd.Flags().GetBool("json")

	// Require exactly one positional argument: the project name.
	if len(args) == 0 {
		err := fmt.Errorf("requires project name: run 'toolwright init <project-name>'")
		if jsonMode {
			_ = outputError(cmd.OutOrStdout(), "usage_error", err.Error(),
				"provide a project name as the first argument")
		}
		return err
	}
	name := args[0]

	yes, _ := cmd.Flags().GetBool("yes")
	runtime, _ := cmd.Flags().GetString("runtime")
	description, _ := cmd.Flags().GetString("description")
	outputDir, _ := cmd.Flags().GetString("output")

	// Validate runtime before doing anything else.
	if !validRuntimes[runtime] {
		err := fmt.Errorf("invalid runtime %q: must be one of shell, go, python, typescript", runtime)
		if jsonMode {
			_ = outputError(cmd.OutOrStdout(), "invalid_runtime", err.Error(),
				"choose one of: shell, go, python, typescript")
		}
		return err
	}

	var opts ScaffoldOptions

	if yes || isCI() {
		// Non-interactive mode: use flags/defaults.
		if description == "" {
			description = fmt.Sprintf("A %s toolkit", name)
		}
		opts = ScaffoldOptions{
			Name:        name,
			Description: description,
			OutputDir:   outputDir,
			Runtime:     runtime,
			Auth:        "none",
		}
	} else {
		// Interactive mode: run the wizard.
		wizResult, err := cfg.Wizard.Run(cmd.Context())
		if err != nil {
			if jsonMode {
				_ = outputError(cmd.OutOrStdout(), "wizard_error", err.Error(), "re-run to try again")
			}
			return err
		}
		// Name always comes from the positional arg, not the wizard.
		opts = ScaffoldOptions{
			Name:        name,
			Description: wizResult.Description,
			OutputDir:   outputDir,
			Runtime:     wizResult.Runtime,
			Auth:        wizResult.Auth,
		}
	}

	result, err := cfg.Scaffolder.Scaffold(cmd.Context(), opts)
	if err != nil {
		if jsonMode {
			_ = outputError(cmd.OutOrStdout(), "scaffold_error", err.Error(), "check permissions and try again")
		}
		return err
	}

	if jsonMode {
		return outputJSON(cmd.OutOrStdout(), initResultOutput{
			Dir:   result.Dir,
			Files: result.Files,
		})
	}

	// Human output: summary line then file listing.
	w := cmd.OutOrStdout()
	fmt.Fprintf(w, "Created %s (%d files)\n", result.Dir, len(result.Files))
	for _, f := range result.Files {
		fmt.Fprintln(w, f)
	}
	return nil
}
