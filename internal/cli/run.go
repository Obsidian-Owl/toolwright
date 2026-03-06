package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Obsidian-Owl/toolwright/internal/manifest"
	"github.com/Obsidian-Owl/toolwright/internal/runner"
)

// toolRunner abstracts tool execution so the CLI layer can be tested without
// spawning real child processes. Production code wires runner.Executor here.
type toolRunner interface {
	Run(ctx context.Context, tool manifest.Tool, args []string, flags map[string]string, token string) (*runner.Result, error)
}

// tokenResolver abstracts auth token resolution so the CLI layer can be tested
// without reading real keystores or environment variables. Production code wires
// auth.Resolver here.
type tokenResolver interface {
	Resolve(ctx context.Context, auth manifest.Auth, toolName string, flagValue string) (string, error)
}

// runConfig holds the injectable dependencies for the run command.
type runConfig struct {
	Runner   toolRunner
	Resolver tokenResolver
}

// newRunCmd returns the run subcommand. cfg provides the runner and resolver
// dependencies; in production these are wired to real implementations.
//
// DisableFlagParsing is set so that unknown tool flags (e.g. --severity high)
// are passed through as raw args rather than causing cobra parse errors.
// Known flags (--manifest/-m, --token, --json) are extracted manually in RunE.
func newRunCmd(cfg *runConfig) *cobra.Command {
	cmd := &cobra.Command{
		Use:                "run <tool-name> [args...]",
		Short:              "Run a tool defined in the manifest",
		DisableFlagParsing: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runTool(cmd, args, cfg)
		},
	}

	// Declare flags so they are visible via cmd.Flags().Lookup() for tests
	// that inspect the command's flag set. These are parsed manually in RunE.
	cmd.Flags().StringP("manifest", "m", "toolwright.yaml", "path to manifest file")
	cmd.Flags().String("token", "", "explicit auth token (overrides env/keystore lookup)")

	return cmd
}

// runTool implements the core logic of the run subcommand.
// Because DisableFlagParsing is set, all args arrive unparsed and we extract
// our own flags (--manifest/-m, --token, --json) manually before splitting the
// remainder into tool name + tool args.
func runTool(cmd *cobra.Command, args []string, cfg *runConfig) error {
	// Extract known flags from the raw arg list.
	jsonMode, debugMode, manifestPath, tokenFlag, remaining := extractRunFlags(args)

	// Propagate --debug into the persistent flag so debugLog can read it.
	if debugMode {
		_ = cmd.Flags().Set("debug", "true")
	}

	// Require at least one arg: the tool name.
	if len(remaining) == 0 {
		return &UsageError{Err: fmt.Errorf("requires tool name: usage: toolwright run <tool-name> [args...]")}
	}

	toolName := remaining[0]
	toolArgs := remaining[1:]

	// Load and validate the manifest.
	tk, err := loadManifest(manifestPath)
	if err != nil {
		if jsonMode {
			_ = outputError(cmd.OutOrStdout(), "io_error",
				fmt.Sprintf("cannot load manifest: %s", manifestPath),
				"check that the file exists and is readable")
		}
		return err
	}
	debugLog(cmd, "loading manifest from %s", manifestPath)

	// Find the tool by name.
	var tool manifest.Tool
	found := false
	for _, t := range tk.Tools {
		if t.Name == toolName {
			tool = t
			found = true
			break
		}
	}
	if !found {
		msg := fmt.Sprintf("tool %q not found in manifest", toolName)
		if jsonMode {
			_ = outputError(cmd.OutOrStdout(), "tool_not_found", msg,
				"run 'toolwright list' to see available tools")
		}
		return fmt.Errorf("%s", msg) //nolint:err113 // dynamic tool-not-found error, not a sentinel
	}

	// Resolve auth token if required.
	auth := tk.ResolvedAuth(tool)
	var token string
	if auth.Type != "none" {
		resolved, resolveErr := cfg.Resolver.Resolve(cmd.Context(), auth, toolName, tokenFlag)
		if resolveErr != nil {
			if jsonMode {
				_ = outputError(cmd.OutOrStdout(), "auth_required",
					resolveErr.Error(),
					fmt.Sprintf("run 'toolwright login %s' to authenticate", toolName))
			}
			return resolveErr
		}
		token = resolved
	}
	debugLog(cmd, "resolving auth for tool %s (type: %s)", toolName, auth.Type)

	// Split remaining tool args into positional args and flags for the tool.
	positional, flagMap := splitToolArgs(toolArgs)

	// Execute the tool.
	debugLog(cmd, "executing: %s", tool.Entrypoint)
	result, runErr := cfg.Runner.Run(cmd.Context(), tool, positional, flagMap, token)
	if runErr != nil {
		return runErr
	}

	debugLog(cmd, "tool exited with code %d", result.ExitCode)

	// Forward tool stdout and stderr.
	if len(result.Stdout) > 0 {
		_, _ = cmd.OutOrStdout().Write(result.Stdout)
	}
	if len(result.Stderr) > 0 {
		_, _ = cmd.ErrOrStderr().Write(result.Stderr)
	}

	// Propagate non-zero exit codes as errors.
	if result.ExitCode != 0 {
		return fmt.Errorf("tool exited with code %d", result.ExitCode)
	}

	return nil
}

// extractRunFlags scans raw args and pulls out the flags the run command owns
// (--manifest/-m, --token, --json, --debug). It returns the extracted values
// and the remaining args with those flags and their values removed.
func extractRunFlags(args []string) (jsonMode bool, debugMode bool, manifestPath, tokenFlagValue string, remaining []string) {
	manifestPath = "toolwright.yaml" // default

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--json", strings.HasPrefix(arg, "--json="):
			jsonMode = true
		case arg == "--debug", strings.HasPrefix(arg, "--debug="):
			debugMode = true
		case arg == "--manifest" || arg == "-m":
			if i+1 < len(args) {
				i++
				manifestPath = args[i]
			}
		case strings.HasPrefix(arg, "--manifest="):
			manifestPath = strings.TrimPrefix(arg, "--manifest=")
		case arg == "--token":
			if i+1 < len(args) {
				i++
				tokenFlagValue = args[i]
			}
		case strings.HasPrefix(arg, "--token="):
			tokenFlagValue = strings.TrimPrefix(arg, "--token=")
		default:
			remaining = append(remaining, arg)
		}
	}

	return jsonMode, debugMode, manifestPath, tokenFlagValue, remaining
}

// splitToolArgs splits a slice of raw args (after the tool name) into
// positional args and a flags map. Args that start with "--" are treated as
// flags: "--key value" pairs are consumed together. Positional args are
// everything else.
func splitToolArgs(raw []string) ([]string, map[string]string) {
	var positional []string
	flags := map[string]string{}

	for i := 0; i < len(raw); i++ {
		arg := raw[i]
		if strings.HasPrefix(arg, "--") {
			key := strings.TrimPrefix(arg, "--")
			// Consume the next element as the value if available and not itself a flag.
			if i+1 < len(raw) && !strings.HasPrefix(raw[i+1], "--") {
				flags[key] = raw[i+1]
				i++
			} else {
				flags[key] = ""
			}
		} else {
			positional = append(positional, arg)
		}
	}

	return positional, flags
}
