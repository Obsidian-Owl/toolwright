package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// ManifestGenerator abstracts AI manifest generation so the CLI layer
// can be tested without making real API calls.
type ManifestGenerator interface {
	Generate(ctx context.Context, opts ManifestGenerateOptions) (*ManifestGenerateResult, error)
}

// manifestGenerator is kept as an alias for internal wiring.
type manifestGenerator = ManifestGenerator

// manifestGenerateConfig holds injectable dependencies for the
// generate manifest subcommand.
type manifestGenerateConfig struct {
	Generator manifestGenerator
}

// ManifestGenerateOptions describes what the manifest generator should produce.
type ManifestGenerateOptions struct {
	Provider    string // "anthropic", "openai", "gemini"
	Description string // User-provided description of what the toolkit does
	OutputPath  string // Where to write the manifest (empty = stdout for dry-run)
	DryRun      bool   // If true, print to stdout instead of writing
	Model       string // LLM model override; empty = use provider default
	NoMerge     bool   // If true and OutputPath exists, return error instead of overwriting
}

// ManifestGenerateResult holds the output of a manifest generation.
type ManifestGenerateResult struct {
	Manifest string // The generated YAML content
	Provider string // Which provider was used
}

// validManifestProviders is the set of accepted provider values (case-sensitive).
var validManifestProviders = []string{"anthropic", "openai", "gemini"}

// manifestGenerateSuccessOutput is the JSON shape for a successful generate manifest result.
type manifestGenerateSuccessOutput struct {
	Provider string `json:"provider"`
	Manifest string `json:"manifest"`
}

// newGenerateManifestCmd returns the "generate manifest" subcommand.
func newGenerateManifestCmd(cfg *manifestGenerateConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "manifest",
		Short: "Generate a toolwright manifest using an AI provider",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runGenerateManifest(cmd, cfg)
		},
	}
	cmd.Flags().StringP("provider", "p", "", "AI provider to use (anthropic, openai, gemini)")
	cmd.Flags().StringP("description", "d", "", "description of what the toolkit should do")
	cmd.Flags().StringP("output", "o", "toolwright.yaml", "output file path for the generated manifest")
	cmd.Flags().Bool("dry-run", false, "print manifest to stdout instead of writing to file")
	cmd.Flags().StringP("model", "m", "", "override provider default model")
	cmd.Flags().Bool("no-merge", false, "fail if output file already exists")
	return cmd
}

func runGenerateManifest(cmd *cobra.Command, cfg *manifestGenerateConfig) error {
	jsonMode, _ := cmd.Flags().GetBool("json")

	provider, _ := cmd.Flags().GetString("provider")
	if err := validateManifestProvider(provider); err != nil {
		if jsonMode {
			_ = outputError(cmd.OutOrStdout(), "invalid_provider", err.Error(), "")
		}
		return err
	}

	description, _ := cmd.Flags().GetString("description")
	outputPath, _ := cmd.Flags().GetString("output")
	dryRun, _ := cmd.Flags().GetBool("dry-run")
	model, _ := cmd.Flags().GetString("model")
	noMerge, _ := cmd.Flags().GetBool("no-merge")

	opts := ManifestGenerateOptions{
		Provider:    provider,
		Description: description,
		OutputPath:  outputPath,
		DryRun:      dryRun,
		Model:       model,
		NoMerge:     noMerge,
	}

	if cfg.Generator == nil {
		return fmt.Errorf("AI manifest generation is not yet implemented")
	}
	result, err := cfg.Generator.Generate(cmd.Context(), opts)
	if err != nil {
		if jsonMode {
			_ = outputError(cmd.OutOrStdout(), "generate_error", err.Error(), "")
		}
		return err
	}

	return outputManifestResult(cmd, jsonMode, dryRun, outputPath, result)
}

// validateManifestProvider returns an error if provider is not in validManifestProviders.
func validateManifestProvider(provider string) error {
	for _, v := range validManifestProviders {
		if provider == v {
			return nil
		}
	}
	return fmt.Errorf(
		"invalid provider %q: must be one of %s",
		provider,
		strings.Join(validManifestProviders, ", "),
	)
}

// outputManifestResult writes either JSON or human-readable output for a
// successful manifest generation result.
func outputManifestResult(cmd *cobra.Command, jsonMode, dryRun bool, outputPath string, result *ManifestGenerateResult) error {
	w := cmd.OutOrStdout()

	if jsonMode {
		out := manifestGenerateSuccessOutput{
			Provider: result.Provider,
			Manifest: result.Manifest,
		}
		return outputJSON(w, out)
	}

	if dryRun {
		fmt.Fprint(w, result.Manifest)
		return nil
	}

	fmt.Fprintf(w, "Manifest generated using %s and written to %s\n", result.Provider, outputPath)
	return nil
}
