package cli

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/Obsidian-Owl/toolwright/internal/manifest"
)

type validateOutput struct {
	Valid    bool           `json:"valid"`
	Errors   []validateItem `json:"errors"`
	Warnings []validateItem `json:"warnings"`
}

type validateItem struct {
	Path    string `json:"path"`
	Message string `json:"message"`
	Rule    string `json:"rule"`
}

// newValidateCmd returns the validate subcommand.
func newValidateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate a toolwright manifest",
		RunE:  runValidate,
	}
	cmd.Flags().Bool("online", false, "perform online checks (e.g., reachability of provider_url)")
	return cmd
}

func runValidate(cmd *cobra.Command, args []string) error {
	jsonMode, _ := cmd.Flags().GetBool("json")
	online, _ := cmd.Flags().GetBool("online")

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
	debugLog(cmd, "loaded manifest from %s", path)

	errs := []validateItem{}
	warns := []validateItem{}

	// Structural validation.
	for _, ve := range manifest.Validate(tk) {
		item := validateItem{
			Path:    ve.Path,
			Message: ve.Message,
			Rule:    ve.Rule,
		}
		if ve.Severity == manifest.SeverityWarning {
			warns = append(warns, item)
		} else {
			errs = append(errs, item)
		}
	}
	debugLog(cmd, "structural validation: %d errors, %d warnings", len(errs), len(warns))

	// Entrypoint checks.
	for i, tool := range tk.Tools {
		epPath := fmt.Sprintf("tools[%d].entrypoint", i)
		debugLog(cmd, "checking entrypoint: %s", tool.Entrypoint)
		info, statErr := os.Stat(tool.Entrypoint)
		if statErr != nil {
			errs = append(errs, validateItem{
				Path:    epPath,
				Message: fmt.Sprintf("entrypoint not found: %s", tool.Entrypoint),
				Rule:    "entrypoint-exists",
			})
		} else {
			if info.Mode()&0111 == 0 {
				warns = append(warns, validateItem{
					Path:    epPath,
					Message: fmt.Sprintf("entrypoint is not executable: %s", tool.Entrypoint),
					Rule:    "entrypoint-executable",
				})
			}
		}
	}

	// Online checks.
	if online && tk.Auth != nil && tk.Auth.ProviderURL != "" {
		debugLog(cmd, "online check: %s", tk.Auth.ProviderURL)
		ctx, cancel := context.WithTimeout(cmd.Context(), 5*time.Second)
		defer cancel()
		req, reqErr := http.NewRequestWithContext(ctx, http.MethodHead, tk.Auth.ProviderURL, nil)
		if reqErr != nil {
			warns = append(warns, validateItem{
				Path:    "auth.provider_url",
				Message: fmt.Sprintf("provider_url is not a valid URL: %s", tk.Auth.ProviderURL),
				Rule:    "provider-url-reachable",
			})
		} else if resp, httpErr := http.DefaultClient.Do(req); httpErr != nil {
			warns = append(warns, validateItem{
				Path:    "auth.provider_url",
				Message: fmt.Sprintf("provider_url is unreachable: %s", tk.Auth.ProviderURL),
				Rule:    "provider-url-reachable",
			})
		} else {
			resp.Body.Close()
		}
	}

	result := validateOutput{
		Valid:    len(errs) == 0,
		Errors:   errs,
		Warnings: warns,
	}

	if jsonMode {
		return outputJSON(cmd.OutOrStdout(), result)
	}

	// Human output.
	for _, w := range result.Warnings {
		fmt.Fprintf(cmd.OutOrStdout(), "warning [%s] %s: %s\n", w.Rule, w.Path, w.Message)
	}
	if result.Valid {
		fmt.Fprintf(cmd.OutOrStdout(), "valid: %s (%s)\n", path, tk.Metadata.Name)
		return nil
	}

	for _, e := range result.Errors {
		fmt.Fprintf(cmd.OutOrStdout(), "error [%s] %s: %s\n", e.Rule, e.Path, e.Message)
	}
	return fmt.Errorf("manifest validation failed: %d error(s)", len(result.Errors))
}
