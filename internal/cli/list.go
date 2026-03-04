package cli

import (
	"fmt"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

type listOutput struct {
	Tools []listTool `json:"tools"`
}

type listTool struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	AuthType    string `json:"auth_type"`
}

// newListCmd returns the list subcommand.
func newListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List tools defined in a manifest",
		RunE:  runList,
	}
	return cmd
}

func runList(cmd *cobra.Command, args []string) error {
	jsonMode, _ := cmd.Flags().GetBool("json")

	path := "toolwright.yaml"
	if len(args) > 0 {
		path = args[0]
	}

	tk, err := loadManifest(path)
	if err != nil {
		if jsonMode {
			_ = outputError(cmd.OutOrStdout(), "io_error",
				fmt.Sprintf("cannot load manifest: %s", path),
				"check that the file exists and is readable")
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "error: cannot load manifest %s\n", path)
		}
		return err
	}

	tools := []listTool{}
	for _, tool := range tk.Tools {
		authType := tk.ResolvedAuth(tool).Type
		tools = append(tools, listTool{
			Name:        tool.Name,
			Description: tool.Description,
			AuthType:    authType,
		})
	}

	if jsonMode {
		return outputJSON(cmd.OutOrStdout(), listOutput{Tools: tools})
	}

	// Human table output using tabwriter.
	w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tDESCRIPTION\tAUTH")
	for _, t := range tools {
		fmt.Fprintf(w, "%s\t%s\t%s\n", t.Name, t.Description, t.AuthType)
	}
	return w.Flush()
}
