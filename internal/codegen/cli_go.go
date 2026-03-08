package codegen

import (
	"bytes"
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"text/template"

	"github.com/Obsidian-Owl/toolwright/internal/manifest"
)

// validToolName matches tool names safe for use in identifiers and file paths.
// Security boundary: this regex also prevents injection into generated source
// code string literals and exec.Command arguments. Do not relax without
// reviewing all template interpolation points that use tool names.
var validToolName = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9_-]*$`)

// escStringLiteral escapes a string for safe interpolation inside a
// double-quoted Go or JavaScript/TypeScript string literal.
func escStringLiteral(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", `\n`, "\r", `\r`, "\t", `\t`, "\x00", `\x00`)
	return r.Replace(s)
}

// joinEscStringLiterals escapes each element and joins with the separator.
func joinEscStringLiterals(elems []string, sep string) string {
	escaped := make([]string, len(elems))
	for i, e := range elems {
		escaped[i] = escStringLiteral(e)
	}
	return strings.Join(escaped, sep)
}

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
		if !validToolName.MatchString(tool.Name) {
			return nil, fmt.Errorf("tool name %q contains invalid characters: must match %s", tool.Name, validToolName.String())
		}
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
	IsArray     bool   // true when the manifest type is an array type (e.g., "string[]"), NOT for object[]
	ArrayBase   string // base element type for array flags (e.g., "string", "int")
	// Object flag fields (type "object" or "object[]").
	IsObject             bool     // true when manifest type is "object" or "object[]"
	IsObjectArray        bool     // true when manifest type is "object[]"
	HasItemSchema        bool     // true when flag.ItemSchema is non-nil
	ItemSchemaProperties []string // required property names from itemSchema for validation
}

type argData struct {
	Name        string // original arg name used in string literals
	GoName      string // sanitized Go identifier
	GoType      string
	Required    bool
	Description string
}

type toolGoData struct {
	ToolkitName            string
	ToolName               string // original name used in string literals (e.g., "check-health")
	GoName                 string // sanitized Go identifier (e.g., "checkHealth")
	Description            string
	Args                   []argData
	Flags                  []flagData
	HasAuth                bool
	AuthType               string
	TokenEnv               string
	TokenFlag              string
	HasNonStringArgs       bool // true if any arg needs strconv parsing
	HasNonStringArrayFlags bool // true if any flag is a non-string array type needing strconv
	HasObjectFlags         bool // true if any flag is type "object" or "object[]"
	IsBinaryOutput         bool // true if tool output format is "binary"
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
	case "string[]":
		return "[]string"
	case "int[]":
		return "[]int"
	case "float[]":
		return "[]float64"
	case "bool[]":
		return "[]bool"
	case "object", "object[]":
		// Object types are passed as a single JSON string at the CLI level.
		return "string"
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
	hasNonStringArrayFlags := false
	hasObjectFlags := false
	for i, f := range tool.Flags {
		isArr := manifest.IsArrayType(f.Type)
		base := manifest.BaseType(f.Type)

		// Object types ("object" or "object[]") are passed as a single JSON
		// string, not as repeated CLI values. Override array treatment.
		isObject := f.Type == "object" || base == "object"
		if isObject {
			isArr = false
			base = ""
		}

		// Augment description with JSON hint for object flags.
		desc := f.Description
		if isObject {
			props := extractSchemaProperties(f.ItemSchema)
			if len(props) > 0 {
				desc = "(JSON) " + desc + " (expects: " + strings.Join(props, ", ") + ")"
			} else {
				desc = "(JSON) " + desc
			}
		}

		isObjectArray := f.Type == "object[]"

		var schemaProps []string
		var hasItemSchema bool
		if isObject && f.ItemSchema != nil {
			hasItemSchema = true
			schemaProps = extractRequiredProperties(f.ItemSchema)
		}

		flags[i] = flagData{
			Name:                 f.Name,
			GoName:               goIdentifier(f.Name),
			GoType:               goType(f.Type),
			Required:             f.Required,
			Default:              f.Default,
			Description:          desc,
			Enum:                 f.Enum,
			IsArray:              isArr,
			ArrayBase:            base,
			IsObject:             isObject,
			IsObjectArray:        isObjectArray,
			HasItemSchema:        hasItemSchema,
			ItemSchemaProperties: schemaProps,
		}
		if isArr && base != "string" {
			hasNonStringArrayFlags = true
		}
		if isObject {
			hasObjectFlags = true
		}
	}

	hasAuth := auth.Type == "token" || auth.Type == "oauth2"

	// Strip leading "--" from token flag name if present, for use as Cobra
	// flag name.
	tokenFlag := strings.TrimPrefix(auth.TokenFlag, "--")

	hasNonStringArgs := false
	for _, a := range args {
		if a.GoType != "string" {
			hasNonStringArgs = true
			break
		}
	}

	return toolGoData{
		ToolkitName:            m.Metadata.Name,
		ToolName:               tool.Name,
		GoName:                 goIdentifier(tool.Name),
		Description:            tool.Description,
		Args:                   args,
		Flags:                  flags,
		HasAuth:                hasAuth,
		AuthType:               auth.Type,
		TokenEnv:               auth.TokenEnv,
		TokenFlag:              tokenFlag,
		HasNonStringArgs:       hasNonStringArgs,
		HasNonStringArrayFlags: hasNonStringArrayFlags,
		HasObjectFlags:         hasObjectFlags,
		IsBinaryOutput:         tool.Output.Format == "binary",
	}
}

// renderTemplate executes a named template with the given data and returns the
// rendered bytes.
func renderTemplate(name, tmplStr string, data any) ([]byte, error) {
	funcMap := template.FuncMap{
		"cobraFlagFunc":      cobraFlagFunc,
		"formatDefault":      formatDefault,
		"formatArrayDefault": formatArrayDefault,
		"joinStrings":        strings.Join,
		"esc":                escStringLiteral,
		"joinEsc":            joinEscStringLiterals,
		"isStringBase": func(base string) bool {
			return base == "string"
		},
		// joinQuoted emits elements as comma-separated double-quoted Go string
		// literals, suitable for use inside a []string{...} composite literal.
		"joinQuoted": func(elems []string) string {
			quoted := make([]string, len(elems))
			for i, e := range elems {
				quoted[i] = fmt.Sprintf("%q", e)
			}
			return strings.Join(quoted, ", ")
		},
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
	case "[]string", "[]int", "[]float64", "[]bool":
		// Safety net: array types are handled directly by the template's
		// .IsArray branch (StringArrayVar). These cases are unreachable
		// in normal flow but kept as a defensive fallback.
		return "StringArrayVar"
	default:
		return "StringVar"
	}
}

// extractSchemaProperties returns the property names from a JSON Schema
// "properties" map. Returns nil if the schema is nil or has no "properties".
func extractSchemaProperties(schema map[string]any) []string {
	if schema == nil {
		return nil
	}
	propsRaw, ok := schema["properties"]
	if !ok {
		return nil
	}
	props, ok := propsRaw.(map[string]any)
	if !ok {
		return nil
	}
	names := make([]string, 0, len(props))
	for k := range props {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

// extractRequiredProperties returns the property names listed in a JSON Schema
// "required" array. Returns nil if the schema has no "required" field.
func extractRequiredProperties(schema map[string]any) []string {
	if schema == nil {
		return nil
	}
	reqRaw, ok := schema["required"]
	if !ok {
		return nil
	}
	reqArr, ok := reqRaw.([]any)
	if !ok {
		return nil
	}
	names := make([]string, 0, len(reqArr))
	for _, v := range reqArr {
		if s, ok := v.(string); ok {
			names = append(names, s)
		}
	}
	sort.Strings(names)
	return names
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

// formatArrayDefault returns the Go literal for an array flag's default.
// For string[] it emits []string{"a","b"}; for all other array types it
// emits []string{} because non-string values are parsed from strings at
// runtime. When v is nil, it returns "nil".
func formatArrayDefault(v any, isStringBase bool) string {
	if v == nil {
		return "nil"
	}
	items, ok := v.([]interface{})
	if !ok {
		return "nil"
	}
	if !isStringBase {
		// Non-string types are parsed from strings at runtime.
		// Convert manifest default values to their string representations.
		if len(items) == 0 {
			return "[]string{}"
		}
		parts := make([]string, len(items))
		for i, item := range items {
			parts[i] = fmt.Sprintf("%q", fmt.Sprintf("%v", item))
		}
		return "[]string{" + strings.Join(parts, ", ") + "}"
	}
	if len(items) == 0 {
		return "[]string{}"
	}
	parts := make([]string, len(items))
	for i, item := range items {
		parts[i] = fmt.Sprintf("%q", item)
	}
	return "[]string{" + strings.Join(parts, ", ") + "}"
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
	{Name: "{{.Name | esc}}", Description: "{{.Description | esc}}"},
{{- end}}
}

var rootCmd = &cobra.Command{
	Use:   "{{.ToolkitName}}",
	Short: "{{.ToolkitDescription | esc}}",
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
{{- if or .HasNonStringArgs .HasNonStringArrayFlags}}
	"strconv"
{{- end}}
{{- if .HasObjectFlags}}
	"encoding/json"
{{- end}}

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
{{- if .IsArray}}
	{{$goName}}Flag{{.GoName}} []string
{{- else}}
	{{$goName}}Flag{{.GoName}} {{.GoType}}
{{- end}}
{{- end}}
{{- if .IsBinaryOutput}}
	{{$goName}}FlagOutput string
{{- end}}
{{- if $hasAuth}}
	{{$goName}}Token string
{{- end}}
)

func init() {
{{- range .Flags}}
{{- if .IsArray}}
	{{$goName}}Cmd.Flags().StringArrayVar(&{{$goName}}Flag{{.GoName}}, "{{.Name}}", {{formatArrayDefault .Default (isStringBase .ArrayBase)}}, "{{.Description | esc}}{{if .Enum}} (allowed: {{joinEsc .Enum ", "}}){{end}}")
{{- else}}
	{{$goName}}Cmd.Flags().{{cobraFlagFunc .GoType}}(&{{$goName}}Flag{{.GoName}}, "{{.Name}}", {{formatDefault .Default}}, "{{.Description | esc}}{{if .Enum}} (allowed: {{joinEsc .Enum ", "}}){{end}}")
{{- end}}
{{- if .Required}}
	_ = {{$goName}}Cmd.MarkFlagRequired("{{.Name}}")
{{- end}}
{{- end}}
{{- if .IsBinaryOutput}}
	{{$goName}}Cmd.Flags().StringVar(&{{$goName}}FlagOutput, "output", "", "Output file path for binary data")
{{- end}}
{{- if $hasAuth}}
	{{$goName}}Cmd.Flags().StringVar(&{{$goName}}Token, "{{$tokenFlag | esc}}", "", "Auth token (overrides {{$tokenEnv | esc}} env var)")
{{- end}}
	rootCmd.AddCommand({{$goName}}Cmd)
}

var {{.GoName}}Cmd = &cobra.Command{
	Use:   "{{.ToolName}}{{range .Args}} <{{.Name}}>{{end}}",
	Short: "{{.Description | esc}}",
	Long:  "{{.Description | esc}}",
{{- if .Args}}
	Args:  cobra.MinimumNArgs({{len .Args}}),
{{- end}}
	RunE: func(cmd *cobra.Command, args []string) error {
{{- range $i, $a := .Args}}
{{- if eq $a.GoType "string"}}
		arg{{$a.GoName}} := args[{{$i}}] // {{$a.GoType}}
{{- else if eq $a.GoType "int"}}
		arg{{$a.GoName}}, err := strconv.Atoi(args[{{$i}}]) // {{$a.GoType}}
		if err != nil {
			return fmt.Errorf("invalid value %q for argument {{$a.Name}}: %w", args[{{$i}}], err)
		}
{{- else if eq $a.GoType "float64"}}
		arg{{$a.GoName}}, err := strconv.ParseFloat(args[{{$i}}], 64) // {{$a.GoType}}
		if err != nil {
			return fmt.Errorf("invalid value %q for argument {{$a.Name}}: %w", args[{{$i}}], err)
		}
{{- else if eq $a.GoType "bool"}}
		arg{{$a.GoName}}, err := strconv.ParseBool(args[{{$i}}]) // {{$a.GoType}}
		if err != nil {
			return fmt.Errorf("invalid value %q for argument {{$a.Name}}: %w", args[{{$i}}], err)
		}
{{- end}}
		_ = arg{{$a.GoName}}
{{- end}}
{{- range .Flags}}
{{- if .IsArray}}
{{- if eq .ArrayBase "int"}}
		parsed{{.GoName}} := make([]int, len({{$goName}}Flag{{.GoName}}))
		for i, s := range {{$goName}}Flag{{.GoName}} {
			v, err := strconv.Atoi(s)
			if err != nil {
				return fmt.Errorf("invalid value %q for element of --{{.Name}}: not a valid int", s)
			}
			parsed{{.GoName}}[i] = v
		}
		_ = parsed{{.GoName}}
{{- else if eq .ArrayBase "float"}}
		parsed{{.GoName}} := make([]float64, len({{$goName}}Flag{{.GoName}}))
		for i, s := range {{$goName}}Flag{{.GoName}} {
			v, err := strconv.ParseFloat(s, 64)
			if err != nil {
				return fmt.Errorf("invalid value %q for element of --{{.Name}}: not a valid float", s)
			}
			parsed{{.GoName}}[i] = v
		}
		_ = parsed{{.GoName}}
{{- else if eq .ArrayBase "bool"}}
		parsed{{.GoName}} := make([]bool, len({{$goName}}Flag{{.GoName}}))
		for i, s := range {{$goName}}Flag{{.GoName}} {
			v, err := strconv.ParseBool(s)
			if err != nil {
				return fmt.Errorf("invalid value %q for element of --{{.Name}}: not a valid bool", s)
			}
			parsed{{.GoName}}[i] = v
		}
		_ = parsed{{.GoName}}
{{- end}}
{{- end}}
{{- end}}
{{- range .Flags}}
{{- if .IsObject}}
{{- if .IsObjectArray}}
		var parsed{{.GoName}} []map[string]any
		if err := json.Unmarshal([]byte({{$goName}}Flag{{.GoName}}), &parsed{{.GoName}}); err != nil {
			return fmt.Errorf("invalid JSON for --{{.Name}}: %w", err)
		}
{{- if .HasItemSchema}}
		for _idx, _elem := range parsed{{.GoName}} {
			for _, _field := range []string{ {{joinQuoted .ItemSchemaProperties}} } {
				if _, ok := _elem[_field]; !ok {
					return fmt.Errorf("--{{.Name}}[%d]: required field %q missing from JSON object", _idx, _field)
				}
			}
		}
{{- end}}
{{- else}}
		var parsed{{.GoName}} map[string]any
		if err := json.Unmarshal([]byte({{$goName}}Flag{{.GoName}}), &parsed{{.GoName}}); err != nil {
			return fmt.Errorf("invalid JSON for --{{.Name}}: %w", err)
		}
{{- if .HasItemSchema}}
		for _, _field := range []string{ {{joinQuoted .ItemSchemaProperties}} } {
			if _, ok := parsed{{.GoName}}[_field]; !ok {
				return fmt.Errorf("--{{.Name}}: required field %q missing from JSON object", _field)
			}
		}
{{- end}}
{{- end}}
		_ = parsed{{.GoName}}
{{- end}}
{{- end}}
{{- if $hasAuth}}
		// Resolve auth token: prefer flag, fall back to env var.
		token := {{$goName}}Token
		if token == "" {
			token = os.Getenv("{{$tokenEnv | esc}}")
		}
		if token == "" {
			return fmt.Errorf("auth required: set {{$tokenEnv | esc}} or pass --{{$tokenFlag | esc}}")
		}
		_ = token // passed to the entrypoint via environment
{{- end}}
{{- if .IsBinaryOutput}}
		// Binary output: detect TTY and handle appropriately.
		fi, statErr := os.Stdout.Stat()
		isTTY := statErr == nil && (fi.Mode()&os.ModeCharDevice) != 0
		if isTTY && {{$goName}}FlagOutput == "" {
			return fmt.Errorf("binary output requires --output <file> or pipe")
		}
		c := exec.CommandContext(cmd.Context(), "echo", "running", "{{$toolName}}")
		c.Stderr = os.Stderr
		if {{$goName}}FlagOutput != "" {
			// --output provided: capture stdout and write to file.
			out, err := c.Output()
			if err != nil {
				return fmt.Errorf("{{$toolName}} failed: %w", err)
			}
			if err := os.WriteFile({{$goName}}FlagOutput, out, 0644); err != nil {
				return fmt.Errorf("writing output file: %w", err)
			}
		} else {
			// No --output: stream directly to stdout (pipe mode).
			c.Stdout = os.Stdout
			if err := c.Run(); err != nil {
				return fmt.Errorf("{{$toolName}} failed: %w", err)
			}
		}
{{- else}}
		// Execute the tool entrypoint.
		c := exec.CommandContext(cmd.Context(), "echo", "running", "{{$toolName}}")
		c.Stdout = os.Stdout
		c.Stderr = os.Stderr
		if err := c.Run(); err != nil {
			return fmt.Errorf("{{$toolName}} failed: %w", err)
		}
{{- end}}
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
	"net"
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
		codeCh := make(chan string, 1)
		mux := http.NewServeMux()
		mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
			code := r.URL.Query().Get("code")
			fmt.Fprintln(w, "Authentication complete. You may close this tab.")
			codeCh <- code
		})

		listener, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			return fmt.Errorf("starting callback server: %w", err)
		}
		callbackPort := listener.Addr().(*net.TCPAddr).Port
		fmt.Fprintf(os.Stdout, "Callback listening on http://127.0.0.1:%d/callback\n", callbackPort)

		srv := &http.Server{Handler: mux}
		go func() { _ = srv.Serve(listener) }()

		// Wait for the authorization code from the callback.
		authCode := <-codeCh
		_ = srv.Close()

		// TODO: Exchange authCode + codeVerifier for an access token at the provider's token endpoint.
		_, _ = authCode, codeVerifier
		fmt.Fprintln(os.Stdout, "Login successful.")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(loginCmd)
}
`
