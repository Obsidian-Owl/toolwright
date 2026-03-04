package cli

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Obsidian-Owl/toolwright/internal/codegen"
	"github.com/Obsidian-Owl/toolwright/internal/manifest"
)

// codeGenerator abstracts the codegen engine so the CLI layer can be tested
// without running real code generation. Production code wires codegen.Engine.
type codeGenerator interface {
	Generate(ctx context.Context, m manifest.Toolkit, opts codegen.GenerateOptions) (*codegen.GenerateResult, error)
}

// generateConfig holds the injectable dependencies for the generate command.
type generateConfig struct {
	Engine codeGenerator
}

// generateResultOutput is the JSON shape for a successful generate result.
type generateResultOutput struct {
	Files  []string `json:"files"`
	Mode   string   `json:"mode"`
	Target string   `json:"target"`
	DryRun bool     `json:"dry_run"`
}

// newGenerateCmd returns the generate parent command with cli and mcp subcommands.
func newGenerateCmd(cfg *generateConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Generate code from a manifest",
	}
	cmd.AddCommand(newGenerateCLICmd(cfg))
	cmd.AddCommand(newGenerateMCPCmd(cfg))
	return cmd
}

// newGenerateCLICmd returns the "generate cli" subcommand.
func newGenerateCLICmd(cfg *generateConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "cli",
		Short: "Generate a CLI client from a manifest",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGenerateCLI(cmd, cfg)
		},
	}
	cmd.Flags().StringP("output", "o", "", "output directory (default: ./cli-{toolkit-name}/)")
	cmd.Flags().String("target", "go", "generation target language")
	cmd.Flags().Bool("dry-run", false, "print files that would be generated without writing them")
	cmd.Flags().Bool("force", false, "overwrite existing generated project")
	cmd.Flags().StringP("manifest", "m", "toolwright.yaml", "path to toolwright manifest")
	return cmd
}

// newGenerateMCPCmd returns the "generate mcp" subcommand.
func newGenerateMCPCmd(cfg *generateConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Generate an MCP server from a manifest",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGenerateMCP(cmd, cfg)
		},
	}
	cmd.Flags().StringP("output", "o", "", "output directory (default: ./mcp-server-{toolkit-name}/)")
	cmd.Flags().String("target", "typescript", "generation target language")
	cmd.Flags().Bool("dry-run", false, "print files that would be generated without writing them")
	cmd.Flags().Bool("force", false, "overwrite existing generated project")
	cmd.Flags().StringP("manifest", "m", "toolwright.yaml", "path to toolwright manifest")
	cmd.Flags().String("transport", "stdio", "MCP transport(s) to support (e.g. stdio, streamable-http)")
	return cmd
}

func runGenerateCLI(cmd *cobra.Command, cfg *generateConfig) error {
	jsonMode, _ := cmd.Flags().GetBool("json")

	target, _ := cmd.Flags().GetString("target")
	if target != "go" {
		err := fmt.Errorf("%s CLI target is not yet implemented", target)
		if jsonMode {
			_ = outputError(cmd.OutOrStdout(), "unsupported_target", err.Error(),
				"only 'go' is currently supported for generate cli")
		}
		return err
	}

	manifestPath, _ := cmd.Flags().GetString("manifest")
	tk, err := loadManifest(manifestPath)
	if err != nil {
		if jsonMode {
			_ = outputError(cmd.OutOrStdout(), "io_error",
				fmt.Sprintf("cannot load manifest: %s", manifestPath),
				"check that the file exists and is readable")
		}
		return err
	}

	outputDir, _ := cmd.Flags().GetString("output")
	if outputDir == "" {
		outputDir = filepath.Join(".", fmt.Sprintf("cli-%s", tk.Metadata.Name))
	}

	dryRun, _ := cmd.Flags().GetBool("dry-run")
	force, _ := cmd.Flags().GetBool("force")

	opts := codegen.GenerateOptions{
		Mode:      "cli",
		Target:    target,
		OutputDir: outputDir,
		Force:     force,
		DryRun:    dryRun,
	}

	result, err := cfg.Engine.Generate(cmd.Context(), *tk, opts)
	if err != nil {
		if jsonMode {
			_ = outputError(cmd.OutOrStdout(), "generate_error", err.Error(), "")
		}
		return err
	}

	return outputGenerateResult(cmd, jsonMode, result)
}

func runGenerateMCP(cmd *cobra.Command, cfg *generateConfig) error {
	jsonMode, _ := cmd.Flags().GetBool("json")

	target, _ := cmd.Flags().GetString("target")
	if target != "typescript" {
		err := fmt.Errorf("%s MCP target is not yet implemented", target)
		if jsonMode {
			_ = outputError(cmd.OutOrStdout(), "unsupported_target", err.Error(),
				"only 'typescript' is currently supported for generate mcp")
		}
		return err
	}

	manifestPath, _ := cmd.Flags().GetString("manifest")
	tk, err := loadManifest(manifestPath)
	if err != nil {
		if jsonMode {
			_ = outputError(cmd.OutOrStdout(), "io_error",
				fmt.Sprintf("cannot load manifest: %s", manifestPath),
				"check that the file exists and is readable")
		}
		return err
	}

	outputDir, _ := cmd.Flags().GetString("output")
	if outputDir == "" {
		outputDir = filepath.Join(".", fmt.Sprintf("mcp-server-%s", tk.Metadata.Name))
	}

	dryRun, _ := cmd.Flags().GetBool("dry-run")
	force, _ := cmd.Flags().GetBool("force")
	transport, _ := cmd.Flags().GetString("transport")

	// Override manifest transport with CLI flag value.
	if transport != "" {
		tk.Generate.MCP.Transport = strings.Split(transport, ",")
	}

	opts := codegen.GenerateOptions{
		Mode:      "mcp",
		Target:    target,
		OutputDir: outputDir,
		Force:     force,
		DryRun:    dryRun,
	}

	result, err := cfg.Engine.Generate(cmd.Context(), *tk, opts)
	if err != nil {
		if jsonMode {
			_ = outputError(cmd.OutOrStdout(), "generate_error", err.Error(), "")
		}
		return err
	}

	return outputGenerateResult(cmd, jsonMode, result)
}

// outputGenerateResult writes either JSON or human-readable output for a
// successful generation result.
func outputGenerateResult(cmd *cobra.Command, jsonMode bool, result *codegen.GenerateResult) error {
	if jsonMode {
		out := generateResultOutput{
			Files:  result.Files,
			Mode:   result.Mode,
			Target: result.Target,
			DryRun: result.DryRun,
		}
		return outputJSON(cmd.OutOrStdout(), out)
	}

	// Human output: list each file.
	w := cmd.OutOrStdout()
	for _, f := range result.Files {
		fmt.Fprintln(w, f)
	}
	return nil
}
