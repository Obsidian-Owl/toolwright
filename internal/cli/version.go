package cli

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

// Set via ldflags at build time.
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildDate = "unknown"
)

// versionOutput is the JSON shape for version output.
type versionOutput struct {
	Version   string `json:"version"`
	GoVersion string `json:"go_version"`
	Commit    string `json:"commit"`
	BuildDate string `json:"build_date"`
}

// newVersionCmd returns the version subcommand.
func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		RunE: func(cmd *cobra.Command, _ []string) error {
			jsonMode, _ := cmd.Flags().GetBool("json")

			if jsonMode {
				return outputJSON(cmd.OutOrStdout(), versionOutput{
					Version:   Version,
					GoVersion: runtime.Version(),
					Commit:    Commit,
					BuildDate: BuildDate,
				})
			}

			fmt.Fprintf(cmd.OutOrStdout(),
				"toolwright %s (%s) built with %s on %s\n",
				Version, Commit, runtime.Version(), BuildDate,
			)
			return nil
		},
	}
}
