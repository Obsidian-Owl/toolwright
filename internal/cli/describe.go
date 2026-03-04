package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Obsidian-Owl/toolwright/internal/manifest"
)

type schemaObject struct {
	Type       string                    `json:"type"`
	Properties map[string]schemaProperty `json:"properties"`
	Required   []string                  `json:"required"`
}

type schemaProperty struct {
	Type        string `json:"type"`
	Description string `json:"description"`
}

type describeDefaultOutput struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Auth        map[string]string `json:"auth"`
	Parameters  schemaObject      `json:"parameters"`
}

type describeMCPOutput struct {
	Name        string       `json:"name"`
	Description string       `json:"description"`
	InputSchema schemaObject `json:"inputSchema"`
}

type describeOpenAIOutput struct {
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Parameters  schemaObject `json:"parameters"`
}

// newDescribeCmd returns the describe subcommand.
func newDescribeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "describe <tool-name>",
		Short: "Output JSON Schema for a tool",
		Args:  describeArgsValidator,
		RunE:  runDescribe,
	}
	cmd.Flags().StringP("format", "f", "json", "output format: json, mcp, openai")
	cmd.Flags().StringP("manifest", "m", "toolwright.yaml", "path to manifest file")
	return cmd
}

// describeArgsValidator wraps cobra.ExactArgs(1) and writes the error to
// stderr so it is visible even when SilenceErrors is set on the root command.
func describeArgsValidator(cmd *cobra.Command, args []string) error {
	if err := cobra.ExactArgs(1)(cmd, args); err != nil {
		fmt.Fprintf(cmd.ErrOrStderr(), "error: %s\nUsage: %s\n", err, cmd.UseLine())
		return err
	}
	return nil
}

func runDescribe(cmd *cobra.Command, args []string) error {
	jsonMode, _ := cmd.Flags().GetBool("json")
	format, _ := cmd.Flags().GetString("format")
	manifestPath, _ := cmd.Flags().GetString("manifest")

	toolName := args[0]

	// Validate format.
	switch format {
	case "json", "mcp", "openai":
		// valid
	default:
		err := fmt.Errorf("unsupported format %q: must be json, mcp, or openai", format)
		fmt.Fprintf(cmd.ErrOrStderr(), "error: %s\n", err)
		return err
	}

	// Load manifest.
	tk, err := loadManifest(manifestPath)
	if err != nil {
		if jsonMode {
			_ = outputError(cmd.OutOrStdout(), "io_error",
				fmt.Sprintf("cannot load manifest: %s", manifestPath),
				"check that the file exists and is readable")
		} else {
			fmt.Fprintf(cmd.ErrOrStderr(), "error: %s\n", err)
		}
		return err
	}

	// Find tool by name.
	idx := -1
	for i, t := range tk.Tools {
		if t.Name == toolName {
			idx = i
			break
		}
	}

	if idx == -1 {
		msg := fmt.Sprintf(`tool "%s" not found in manifest`, toolName)
		if jsonMode {
			_ = outputError(cmd.OutOrStdout(), "tool_not_found", msg, "run 'toolwright list' to see available tools")
		} else {
			fmt.Fprintf(cmd.ErrOrStderr(), "error: %s\n", msg)
		}
		return fmt.Errorf("%s", msg) //nolint:err113 // dynamic user-facing error, not a sentinel
	}

	t := tk.Tools[idx]

	// Build schema.
	schema := buildToolSchema(t.Args, t.Flags)

	// Resolve auth.
	resolvedAuth := tk.ResolvedAuth(t)

	switch format {
	case "json":
		out := describeDefaultOutput{
			Name:        t.Name,
			Description: t.Description,
			Auth:        map[string]string{"type": resolvedAuth.Type},
			Parameters:  schema,
		}
		return outputJSON(cmd.OutOrStdout(), out)
	case "mcp":
		out := describeMCPOutput{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: schema,
		}
		return outputJSON(cmd.OutOrStdout(), out)
	case "openai":
		out := describeOpenAIOutput{
			Name:        t.Name,
			Description: t.Description,
			Parameters:  schema,
		}
		return outputJSON(cmd.OutOrStdout(), out)
	}
	return nil
}

// buildToolSchema constructs a JSON Schema-like object from a tool's args and flags.
func buildToolSchema(args []manifest.Arg, flags []manifest.Flag) schemaObject {
	props := make(map[string]schemaProperty)
	required := []string{}

	for _, a := range args {
		props[a.Name] = schemaProperty{
			Type:        a.Type,
			Description: a.Description,
		}
		if a.Required {
			required = append(required, a.Name)
		}
	}

	for _, f := range flags {
		props[f.Name] = schemaProperty{
			Type:        f.Type,
			Description: f.Description,
		}
		if f.Required {
			required = append(required, f.Name)
		}
	}

	return schemaObject{
		Type:       "object",
		Properties: props,
		Required:   required,
	}
}
