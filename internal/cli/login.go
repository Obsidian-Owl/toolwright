package cli

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/Obsidian-Owl/toolwright/internal/auth"
)

// loginFunc abstracts the auth.Login call so the CLI layer can be tested
// without performing real OAuth flows.
type loginFunc func(ctx context.Context, cfg auth.LoginConfig) (*auth.StoredToken, error)

// loginConfig holds injectable dependencies for the login command.
type loginConfig struct {
	Login loginFunc
}

// newLoginCmd returns the login subcommand. cfg provides the loginFunc
// dependency; in production this is wired to auth.Login.
func newLoginCmd(cfg *loginConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "login <tool-name>",
		Short: "Authenticate a tool via OAuth",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLogin(cmd, args, cfg)
		},
	}
	cmd.Flags().StringP("manifest", "m", "toolwright.yaml", "path to manifest file")
	cmd.Flags().Bool("no-browser", false, "print the authorization URL instead of opening a browser")
	return cmd
}

// loginSuccessOutput is the JSON payload written on successful login.
// It deliberately omits all token fields (Constitution rule 23).
type loginSuccessOutput struct {
	Tool   string `json:"tool"`
	Status string `json:"status"`
}

func runLogin(cmd *cobra.Command, args []string, cfg *loginConfig) error {
	jsonMode, _ := cmd.Flags().GetBool("json")
	manifestPath, _ := cmd.Flags().GetString("manifest")
	noBrowser, _ := cmd.Flags().GetBool("no-browser")

	// Require exactly one positional argument: the tool name.
	if len(args) == 0 {
		err := &UsageError{Err: fmt.Errorf("requires tool name: run 'toolwright login <tool-name>'")}
		if jsonMode {
			_ = outputError(cmd.OutOrStdout(), "usage_error", err.Error(), "provide a tool name as the first argument")
		}
		return err
	}
	toolName := args[0]

	// Load manifest.
	tk, err := loadManifest(manifestPath)
	if err != nil {
		if jsonMode {
			_ = outputError(cmd.OutOrStdout(), "io_error",
				fmt.Sprintf("cannot load manifest: %s", manifestPath),
				"check that the file exists and is readable")
		}
		return err
	}

	// Find tool by name.
	toolIdx := -1
	for i, t := range tk.Tools {
		if t.Name == toolName {
			toolIdx = i
			break
		}
	}
	if toolIdx == -1 {
		msg := fmt.Sprintf("tool %q not found in manifest", toolName)
		if jsonMode {
			_ = outputError(cmd.OutOrStdout(), "tool_not_found", msg, "run 'toolwright list' to see available tools")
		}
		return fmt.Errorf("%s", msg) //nolint:err113 // dynamic user-facing error, not a sentinel
	}

	tool := tk.Tools[toolIdx]

	// Resolve effective auth for this tool.
	resolvedAuth := tk.ResolvedAuth(tool)

	// Validate auth type.
	switch resolvedAuth.Type {
	case "none":
		msg := fmt.Sprintf("tool %q does not require authentication", toolName)
		if jsonMode {
			_ = outputError(cmd.OutOrStdout(), "auth_not_required", msg,
				"this tool uses auth:none and does not need login")
		}
		return fmt.Errorf("%s", msg) //nolint:err113 // dynamic user-facing error, not a sentinel
	case "token":
		msg := fmt.Sprintf("'login' is only available for tools with OAuth; tool %q uses token auth", toolName)
		if jsonMode {
			_ = outputError(cmd.OutOrStdout(), "auth_type_mismatch", msg,
				"set the environment variable specified by token_env to authenticate")
		}
		return fmt.Errorf("%s", msg) //nolint:err113 // dynamic user-facing error, not a sentinel
	case "oauth2":
		// proceed
	}

	// Build OpenBrowser function.
	var openBrowserFn func(string) error
	if noBrowser {
		w := cmd.OutOrStdout()
		openBrowserFn = func(url string) error {
			fmt.Fprintf(w, "Open the following URL in your browser to authenticate:\n%s\n", url)
			return nil
		}
	} else {
		openBrowserFn = openBrowser
	}

	// Build LoginConfig.
	loginCfg := auth.LoginConfig{
		Auth:        resolvedAuth,
		ToolName:    toolName,
		OpenBrowser: openBrowserFn,
	}

	// Delegate to the injected login function.
	_, loginErr := cfg.Login(cmd.Context(), loginCfg)
	if loginErr != nil {
		if jsonMode {
			_ = outputError(cmd.OutOrStdout(), "login_failed", loginErr.Error(),
				fmt.Sprintf("re-run 'toolwright login %s' to try again", toolName))
		}
		return loginErr
	}

	// Success — never print token values (Constitution rule 23).
	if jsonMode {
		return outputJSON(cmd.OutOrStdout(), loginSuccessOutput{
			Tool:   toolName,
			Status: "authenticated",
		})
	}
	fmt.Fprintf(cmd.OutOrStdout(), "Successfully logged in for tool %q.\n", toolName)
	return nil
}

// openBrowser opens url in the system default browser.
// It tries common platform openers: xdg-open (Linux), open (macOS), cmd (Windows).
func openBrowser(url string) error {
	var args []string
	switch runtime.GOOS {
	case "windows":
		args = []string{"rundll32", "url.dll,FileProtocolHandler", url}
	case "darwin":
		args = []string{"open", url}
	default:
		args = []string{"xdg-open", url}
	}
	cmd := exec.CommandContext(context.Background(), args[0], args[1:]...) //nolint:gosec // url originates from OAuth provider config, not user input
	return cmd.Start()
}
