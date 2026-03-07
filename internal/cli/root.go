package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Obsidian-Owl/toolwright/internal/manifest"
)

// Exit code constants used by CLI commands.
const (
	ExitSuccess = 0
	ExitError   = 1
	ExitUsage   = 2
	ExitIO      = 3
)

// NewRootCommand returns the configured root Cobra command for toolwright.
func NewRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "toolwright",
		Short:         "AI agent tool framework",
		Long:          "toolwright is a CLI framework for building, running, and shipping AI agent tools.",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			jsonMode, _ := cmd.Flags().GetBool("json")

			var errCode, errMsg string
			if len(args) > 0 {
				errCode = "usage_error"
				errMsg = fmt.Sprintf("unknown command %q for %q", args[0], cmd.CommandPath())
			} else {
				errCode = "usage_error"
				errMsg = "no command provided"
			}

			err := &UsageError{Err: fmt.Errorf("%s", errMsg)} //nolint:err113 // dynamic usage error, not a sentinel

			if jsonMode {
				_ = outputError(cmd.OutOrStdout(), errCode, errMsg, fmt.Sprintf("run '%s --help' for usage", cmd.CommandPath()))
			}

			return err
		},
	}

	cmd.PersistentFlags().Bool("json", false, "output in JSON format")
	cmd.PersistentFlags().Bool("debug", false, "enable debug output to stderr")
	cmd.PersistentFlags().Bool("no-color", false, "disable color output")

	return cmd
}

// loadManifest reads and parses a manifest file at path. It wraps any error
// with the file path for user debugging.
func loadManifest(path string) (*manifest.Toolkit, error) {
	tk, err := manifest.ParseFile(path)
	if err != nil {
		return nil, &IOError{Err: fmt.Errorf("loading manifest %s: %w", path, err)}
	}
	if tk == nil {
		return nil, &IOError{Err: fmt.Errorf("loading manifest %s: empty file", path)}
	}
	return tk, nil
}
