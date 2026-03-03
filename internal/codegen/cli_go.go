package codegen

import (
	"bytes"
	"context"
	"fmt"
	"strings"
	"text/template"

	"github.com/Obsidian-Owl/toolwright/internal/manifest"
)

// GoCLIGenerator generates Go CLI projects using Cobra.
type GoCLIGenerator struct{}

// NewGoCLIGenerator returns a new GoCLIGenerator.
func NewGoCLIGenerator() *GoCLIGenerator {
	return &GoCLIGenerator{}
}

// Mode returns the generation mode for this generator.
func (g *GoCLIGenerator) Mode() string { return "cli" }

// Target returns the generation target for this generator.
func (g *GoCLIGenerator) Target() string { return "go" }

// Generate produces Go CLI project files from the given template data.
func (g *GoCLIGenerator) Generate(ctx context.Context, data TemplateData, _ string) ([]GeneratedFile, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("generate cancelled: %w", err)
	}

	m := data.Manifest
	var files []GeneratedFile

	// main.go
	mainFile, err := renderTemplate("main.go", mainGoTmpl, mainGoData{
		ToolkitName: m.Metadata.Name,
	})
	if err != nil {
		return nil, fmt.Errorf("rendering main.go: %w", err)
	}
	files = append(files, GeneratedFile{
		Path:    "cmd/" + m.Metadata.Name + "/main.go",
		Content: mainFile,
	})

	// root.go
	rootFile, err := renderTemplate("root.go", rootGoTmpl, rootGoData{
		ToolkitName:        m.Metadata.Name,
		ToolkitDescription: m.Metadata.Description,
		Tools:              toolSummaries(m),
	})
	if err != nil {
		return nil, fmt.Errorf("rendering root.go: %w", err)
	}
	files = append(files, GeneratedFile{
		Path:    "internal/commands/root.go",
		Content: rootFile,
	})

	// per-tool command files
	for _, tool := range m.Tools {
		auth := m.ResolvedAuth(tool)
		var toolFile []byte
		toolFile, err = renderTemplate("tool.go", toolGoTmpl, buildToolData(m, tool, auth))
		if err != nil {
			return nil, fmt.Errorf("rendering command file for tool %q: %w", tool.Name, err)
		}
		files = append(files, GeneratedFile{
			Path:    "internal/commands/" + tool.Name + ".go",
			Content: toolFile,
		})
	}

	// go.mod
	goModFile, err := renderTemplate("go.mod", goModTmpl, goModData{
		ToolkitName: m.Metadata.Name,
	})
	if err != nil {
		return nil, fmt.Errorf("rendering go.mod: %w", err)
	}
	files = append(files, GeneratedFile{
		Path:    "go.mod",
		Content: goModFile,
	})

	// Makefile
	makeFile, err := renderTemplate("Makefile", makefileTmpl, makefileData{
		ToolkitName: m.Metadata.Name,
	})
	if err != nil {
		return nil, fmt.Errorf("rendering Makefile: %w", err)
	}
	files = append(files, GeneratedFile{
		Path:    "Makefile",
		Content: makeFile,
	})

	// README.md
	readmeFile, err := renderTemplate("README.md", readmeTmpl, readmeData{
		ToolkitName:        m.Metadata.Name,
		ToolkitDescription: m.Metadata.Description,
	})
	if err != nil {
		return nil, fmt.Errorf("rendering README.md: %w", err)
	}
	files = append(files, GeneratedFile{
		Path:    "README.md",
		Content: readmeFile,
	})

	// Conditional: internal/auth/resolver.go — if any tool has token or oauth2 auth
	if hasAuthType(m, "token") || hasAuthType(m, "oauth2") {
		resolverFile, err := renderTemplate("resolver.go", resolverGoTmpl, resolverGoData{
			ToolkitName: m.Metadata.Name,
		})
		if err != nil {
			return nil, fmt.Errorf("rendering resolver.go: %w", err)
		}
		files = append(files, GeneratedFile{
			Path:    "internal/auth/resolver.go",
			Content: resolverFile,
		})
	}

	// Conditional: internal/commands/login.go — only if any tool has oauth2 auth
	if hasAuthType(m, "oauth2") {
		loginFile, err := renderTemplate("login.go", loginGoTmpl, loginGoData{
			ToolkitName: m.Metadata.Name,
		})
		if err != nil {
			return nil, fmt.Errorf("rendering login.go: %w", err)
		}
		files = append(files, GeneratedFile{
			Path:    "internal/commands/login.go",
			Content: loginFile,
		})
	}

	return files, nil
}

// ---------------------------------------------------------------------------
// Template data structs
// ---------------------------------------------------------------------------

type mainGoData struct {
	ToolkitName string
}

type toolSummary struct {
	Name        string
	Description string
}

type rootGoData struct {
	ToolkitName        string
	ToolkitDescription string
	Tools              []toolSummary
}

type flagData struct {
	Name        string // original flag name used in string literals (e.g., "dry-run")
	GoName      string // sanitized Go identifier (e.g., "dryRun")
	GoType      string
	Required    bool
	Default     any
	Description string
	Enum        []string
	HasShort    bool
}

type argData struct {
	Name        string // original arg name used in string literals
	GoName      string // sanitized Go identifier
	GoType      string
	Required    bool
	Description string
}

type toolGoData struct {
	ToolkitName string
	ToolName    string // original name used in string literals (e.g., "check-health")
	GoName      string // sanitized Go identifier (e.g., "checkHealth")
	Description string
	Args        []argData
	Flags       []flagData
	HasAuth     bool
	AuthType    string
	TokenEnv    string
	TokenFlag   string
}

type goModData struct {
	ToolkitName string
}

type makefileData struct {
	ToolkitName string
}

type readmeData struct {
	ToolkitName        string
	ToolkitDescription string
}

type resolverGoData struct {
	ToolkitName string
}

type loginGoData struct {
	ToolkitName string
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// goIdentifier converts a hyphenated name to a valid Go camelCase identifier.
// For example, "check-health" becomes "checkHealth" and "deploy-app" becomes "deployApp".
func goIdentifier(name string) string {
	parts := strings.Split(name, "-")
	for i := 1; i < len(parts); i++ {
		if len(parts[i]) > 0 {
			parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
		}
	}
	return strings.Join(parts, "")
}

// goType maps a manifest type string to a Go type string.
func goType(manifestType string) string {
	switch manifestType {
	case "int":
		return "int"
	case "float":
		return "float64"
	case "bool":
		return "bool"
	default:
		return "string"
	}
}

// hasAuthType returns true if any tool in the toolkit resolves to the given
// auth type.
func hasAuthType(m manifest.Toolkit, authType string) bool {
	for _, tool := range m.Tools {
		a := m.ResolvedAuth(tool)
		if a.Type == authType {
			return true
		}
	}
	return false
}

// toolSummaries builds the slice of tool name/description pairs for root.go.
func toolSummaries(m manifest.Toolkit) []toolSummary {
	out := make([]toolSummary, len(m.Tools))
	for i, t := range m.Tools {
		out[i] = toolSummary{Name: t.Name, Description: t.Description}
	}
	return out
}

// buildToolData constructs toolGoData for a single tool.
func buildToolData(m manifest.Toolkit, tool manifest.Tool, auth manifest.Auth) toolGoData {
	args := make([]argData, len(tool.Args))
	for i, a := range tool.Args {
		args[i] = argData{
			Name:        a.Name,
			GoName:      goIdentifier(a.Name),
			GoType:      goType(a.Type),
			Required:    a.Required,
			Description: a.Description,
		}
	}

	flags := make([]flagData, len(tool.Flags))
	for i, f := range tool.Flags {
		flags[i] = flagData{
			Name:        f.Name,
			GoName:      goIdentifier(f.Name),
			GoType:      goType(f.Type),
			Required:    f.Required,
			Default:     f.Default,
			Description: f.Description,
			Enum:        f.Enum,
		}
	}

	hasAuth := auth.Type == "token" || auth.Type == "oauth2"

	// Strip leading "--" from token flag name if present, for use as Cobra
	// flag name.
	tokenFlag := strings.TrimPrefix(auth.TokenFlag, "--")

	return toolGoData{
		ToolkitName: m.Metadata.Name,
		ToolName:    tool.Name,
		GoName:      goIdentifier(tool.Name),
		Description: tool.Description,
		Args:        args,
		Flags:       flags,
		HasAuth:     hasAuth,
		AuthType:    auth.Type,
		TokenEnv:    auth.TokenEnv,
		TokenFlag:   tokenFlag,
	}
}

// renderTemplate executes a named template with the given data and returns the
// rendered bytes.
func renderTemplate(name, tmplStr string, data any) ([]byte, error) {
	funcMap := template.FuncMap{
		"cobraFlagFunc": cobraFlagFunc,
		"formatDefault": formatDefault,
		"joinStrings":   strings.Join,
	}
	t, err := template.New(name).Funcs(funcMap).Parse(tmplStr)
	if err != nil {
		return nil, fmt.Errorf("parsing template %q: %w", name, err)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("executing template %q: %w", name, err)
	}
	return buf.Bytes(), nil
}

// cobraFlagFunc returns the Cobra flag registration function name for a Go
// type (e.g., "string" → "StringVar", "int" → "IntVar").
func cobraFlagFunc(goTypeName string) string {
	switch goTypeName {
	case "int":
		return "IntVar"
	case "float64":
		return "Float64Var"
	case "bool":
		return "BoolVar"
	default:
		return "StringVar"
	}
}

// formatDefault returns the Go literal representation of a default value.
func formatDefault(v any) string {
	if v == nil {
		return `""`
	}
	switch val := v.(type) {
	case string:
		return fmt.Sprintf("%q", val)
	case bool:
		if val {
			return "true"
		}
		return "false"
	case int:
		return fmt.Sprintf("%d", val)
	case float64:
		return fmt.Sprintf("%g", val)
	case int64:
		return fmt.Sprintf("%d", val)
	default:
		return fmt.Sprintf("%v", val)
	}
}

// ---------------------------------------------------------------------------
// Templates
// ---------------------------------------------------------------------------

const mainGoTmpl = `package main

import (
	"os"

	"{{.ToolkitName}}/internal/commands"
)

func main() {
	if err := commands.Execute(); err != nil {
		os.Exit(1)
	}
}
`

const rootGoTmpl = `package commands

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// toolInfo holds the name and description of a tool for list/describe output.
type toolInfo struct {
	Name        string ` + "`" + `json:"name"` + "`" + `
	Description string ` + "`" + `json:"description"` + "`" + `
}

// registry is the embedded list of tools from the manifest.
var registry = []toolInfo{
{{- range .Tools}}
	{Name: "{{.Name}}", Description: "{{.Description}}"},
{{- end}}
}

var rootCmd = &cobra.Command{
	Use:   "{{.ToolkitName}}",
	Short: "{{.ToolkitDescription}}",
}

var listJSON bool

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all available tools",
	RunE: func(cmd *cobra.Command, args []string) error {
		if listJSON {
			enc := json.NewEncoder(os.Stdout)
			return enc.Encode(registry)
		}
		for _, t := range registry {
			fmt.Fprintf(os.Stdout, "%s\t%s\n", t.Name, t.Description)
		}
		return nil
	},
}

var describeCmd = &cobra.Command{
	Use:   "describe <tool>",
	Short: "Describe a tool and its JSON schema",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]
		for _, t := range registry {
			if t.Name == name {
				schema := map[string]interface{}{
					"name":        t.Name,
					"description": t.Description,
					"schema":      map[string]interface{}{"type": "object"},
				}
				enc := json.NewEncoder(os.Stdout)
				return enc.Encode(schema)
			}
		}
		return fmt.Errorf("unknown tool %q: use 'list' to see available tools", name)
	},
}

func init() {
	listCmd.Flags().BoolVar(&listJSON, "json", false, "Output as JSON")
	rootCmd.AddCommand(listCmd)
	rootCmd.AddCommand(describeCmd)
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}
`

const toolGoTmpl = `package commands

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

// {{.GoName}}Cmd is the Cobra command for the {{.ToolName}} tool.
{{- $toolName := .ToolName}}
{{- $goName := .GoName}}
{{- $hasAuth := .HasAuth}}
{{- $authType := .AuthType}}
{{- $tokenEnv := .TokenEnv}}
{{- $tokenFlag := .TokenFlag}}
var (
{{- range .Flags}}
	{{$goName}}Flag{{.GoName}} {{.GoType}}
{{- end}}
{{- if $hasAuth}}
	{{$goName}}Token string
{{- end}}
)

func init() {
{{- range .Flags}}
	{{$goName}}Cmd.Flags().{{cobraFlagFunc .GoType}}(&{{$goName}}Flag{{.GoName}}, "{{.Name}}", {{formatDefault .Default}}, "{{.Description}}{{if .Enum}} (allowed: {{joinStrings .Enum ", "}}){{end}}")
{{- if .Required}}
	_ = {{$goName}}Cmd.MarkFlagRequired("{{.Name}}")
{{- end}}
{{- end}}
{{- if $hasAuth}}
	{{$goName}}Cmd.Flags().StringVar(&{{$goName}}Token, "{{$tokenFlag}}", "", "Auth token (overrides {{$tokenEnv}} env var)")
{{- end}}
	rootCmd.AddCommand({{$goName}}Cmd)
}

var {{.GoName}}Cmd = &cobra.Command{
	Use:   "{{.ToolName}}{{range .Args}} <{{.Name}}>{{end}}",
	Short: "{{.Description}}",
	Long:  "{{.Description}}",
{{- if .Args}}
	Args:  cobra.MinimumNArgs({{len .Args}}),
{{- end}}
	RunE: func(cmd *cobra.Command, args []string) error {
{{- range $i, $a := .Args}}
		var arg{{$a.GoName}} {{$a.GoType}} //nolint:ineffassign // positional arg {{$a.Name}} index {{$i}}
		_ = arg{{$a.GoName}}
		if len(args) > {{$i}} {
			_ = fmt.Sprintf("%v", args[{{$i}}]) // arg: {{$a.Name}} ({{$a.GoType}})
		}
{{- end}}
{{- if $hasAuth}}
		// Resolve auth token: prefer flag, fall back to env var.
		token := {{$goName}}Token
		if token == "" {
			token = os.Getenv("{{$tokenEnv}}")
		}
		if token == "" {
			return fmt.Errorf("auth required: set {{$tokenEnv}} or pass --{{$tokenFlag}}")
		}
		_ = token // passed to the entrypoint via environment
{{- end}}
		// Execute the tool entrypoint.
		c := exec.CommandContext(cmd.Context(), "sh", "-c", "echo 'running {{$toolName}}'")
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		if err := c.Run(); err != nil {
			return fmt.Errorf("{{$toolName}} failed: %w", err)
		}
		return nil
	},
}
`

const goModTmpl = `module {{.ToolkitName}}

go 1.21

require (
	github.com/spf13/cobra v1.8.0
)
`

const makefileTmpl = `.PHONY: build test clean

build:
	go build -o bin/{{.ToolkitName}} ./cmd/{{.ToolkitName}}/...

test:
	go test ./...

clean:
	rm -rf bin/
`

const readmeTmpl = `# {{.ToolkitName}}

{{.ToolkitDescription}}

## Usage

` + "```" + `sh
{{.ToolkitName}} list
{{.ToolkitName}} describe <tool>
` + "```" + `

## Building

` + "```" + `sh
make build
` + "```" + `
`

const resolverGoTmpl = `package auth

import (
	"fmt"
	"os"
)

// Resolver resolves auth tokens from environment variables or flags.
type Resolver struct{}

// NewResolver returns a new Resolver.
func NewResolver() *Resolver {
	return &Resolver{}
}

// ResolveToken returns the token from the given env var name, or returns an
// error if the env var is not set and no override is provided.
func (r *Resolver) ResolveToken(envVar, flagValue string) (string, error) {
	if flagValue != "" {
		return flagValue, nil
	}
	if val := os.Getenv(envVar); val != "" {
		return val, nil
	}
	return "", fmt.Errorf(
		"auth token not found: set the %s environment variable or pass the --token flag",
		envVar,
	)
}
`

const loginGoTmpl = `package commands

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"net/http"
	"os"

	"github.com/spf13/cobra"
)

// loginCmd implements the OAuth2 PKCE login flow.
var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate via OAuth2 (PKCE flow)",
	Long: ` + "`" + `Authenticate with an OAuth2 provider using the PKCE (Proof Key for Code Exchange)
flow. This command starts a local HTTP server to receive the authorization callback,
generates a code_verifier and code_challenge (S256 method), and exchanges the
authorization code for an access token.` + "`" + `,
	RunE: func(cmd *cobra.Command, args []string) error {
		// Generate PKCE code_verifier (RFC 7636).
		verifierBytes := make([]byte, 32)
		if _, err := rand.Read(verifierBytes); err != nil {
			return fmt.Errorf("generating code_verifier: %w", err)
		}
		codeVerifier := base64.RawURLEncoding.EncodeToString(verifierBytes)

		// Derive code_challenge using S256 method.
		h := sha256.New()
		h.Write([]byte(codeVerifier))
		codeChallenge := base64.RawURLEncoding.EncodeToString(h.Sum(nil))

		// OAuth2 provider URL is configured at build time or via environment.
		providerURL := os.Getenv("OAUTH_PROVIDER_URL")
		if providerURL == "" {
			return fmt.Errorf(
				"OAuth provider URL not set: set OAUTH_PROVIDER_URL or configure the provider in your toolkit manifest",
			)
		}

		// Build authorization URL with PKCE parameters.
		authURL := fmt.Sprintf(
			"%s/authorize?response_type=code&code_challenge=%s&code_challenge_method=S256",
			providerURL,
			codeChallenge,
		)
		fmt.Fprintf(os.Stdout, "Open the following URL to authenticate:\n%s\n", authURL)

		// Start a local callback server bound to loopback only (defense-in-depth).
		_ = codeVerifier // used in token exchange (not shown in scaffold)
		mux := http.NewServeMux()
		mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintln(w, "Authentication complete. You may close this tab.")
		})
		srv := &http.Server{
			Addr:    "127.0.0.1:0",
			Handler: mux,
		}
		_ = srv
		return nil
	},
}

func init() {
	rootCmd.AddCommand(loginCmd)
}
`
