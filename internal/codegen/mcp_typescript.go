package codegen

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"text/template"

	"github.com/Obsidian-Owl/toolwright/internal/manifest"
)

// uriParamRe matches {param} placeholders in a URI template.
var uriParamRe = regexp.MustCompile(`\{([^}]+)\}`)

// validIdentifier matches safe JavaScript/TypeScript identifier names.
var validIdentifier = regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

// extractURIParams returns the list of unique parameter names found in a URI template.
// Returns an error if any parameter name is not a valid identifier or appears more than once.
func extractURIParams(uri string) ([]string, error) {
	matches := uriParamRe.FindAllStringSubmatch(uri, -1)
	params := make([]string, 0, len(matches))
	seen := make(map[string]bool, len(matches))
	for _, m := range matches {
		name := m[1]
		if !validIdentifier.MatchString(name) {
			return nil, fmt.Errorf("URI template parameter %q is not a valid identifier", name)
		}
		if seen[name] {
			return nil, fmt.Errorf("URI template parameter %q appears more than once", name)
		}
		seen[name] = true
		params = append(params, name)
	}
	return params, nil
}

// TSMCPGenerator generates TypeScript MCP server projects.
type TSMCPGenerator struct{}

// NewTSMCPGenerator returns a new TSMCPGenerator.
func NewTSMCPGenerator() *TSMCPGenerator {
	return &TSMCPGenerator{}
}

// Mode returns the generation mode for this generator.
func (g *TSMCPGenerator) Mode() string { return "mcp" }

// Target returns the generation target for this generator.
func (g *TSMCPGenerator) Target() string { return "typescript" }

// Generate produces TypeScript MCP server project files from the given template data.
func (g *TSMCPGenerator) Generate(ctx context.Context, data TemplateData, _ string) ([]GeneratedFile, error) {
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("generate cancelled: %w", err)
	}

	m := data.Manifest
	transport := m.Generate.MCP.Transport
	hasStdio := containsTransport(transport, "stdio")
	hasStreamableHTTP := containsTransport(transport, "streamable-http")

	var files []GeneratedFile

	// src/index.ts
	indexData := buildIndexData(m, hasStdio, hasStreamableHTTP)
	indexFile, err := renderTSTemplate("src/index.ts", indexTSTmpl, indexData)
	if err != nil {
		return nil, fmt.Errorf("rendering src/index.ts: %w", err)
	}
	files = append(files, GeneratedFile{Path: "src/index.ts", Content: indexFile})

	// per-tool handler files
	for _, tool := range m.Tools {
		if !validToolName.MatchString(tool.Name) {
			return nil, fmt.Errorf("tool name %q contains invalid characters: must match %s", tool.Name, validToolName.String())
		}
		auth := m.ResolvedAuth(tool)
		toolData, buildErr := buildTSToolData(tool, auth)
		if buildErr != nil {
			return nil, buildErr
		}
		var toolFile []byte
		toolFile, err = renderTSTemplate("tool.ts", tsToolTmpl, toolData)
		if err != nil {
			return nil, fmt.Errorf("rendering tool file for %q: %w", tool.Name, err)
		}
		files = append(files, GeneratedFile{
			Path:    "src/tools/" + tool.Name + ".ts",
			Content: toolFile,
		})
	}

	// per-resource handler files
	for _, res := range m.Resources {
		if !validToolName.MatchString(res.Name) {
			return nil, fmt.Errorf("resource name %q contains invalid characters: must match %s", res.Name, validToolName.String())
		}
		mimeType := res.MimeType
		if mimeType == "" {
			mimeType = "text/plain"
		}
		uriParams, uriErr := extractURIParams(res.URI)
		if uriErr != nil {
			return nil, fmt.Errorf("resource %q: %w", res.Name, uriErr)
		}
		resData := tsResourceData{
			Name:        res.Name,
			Description: res.Description,
			URI:         res.URI,
			MimeType:    mimeType,
			Entrypoint:  res.Entrypoint,
			URIParams:   uriParams,
		}
		var resFile []byte
		resFile, err = renderTSTemplate("resource.ts", tsResourceTmpl, resData)
		if err != nil {
			return nil, fmt.Errorf("rendering resource file for %q: %w", res.Name, err)
		}
		files = append(files, GeneratedFile{
			Path:    "src/resources/" + res.Name + ".ts",
			Content: resFile,
		})
	}

	// src/search.ts — progressive discovery meta-tool
	searchData := buildSearchData(m)
	searchFile, err := renderTSTemplate("src/search.ts", searchTSTmpl, searchData)
	if err != nil {
		return nil, fmt.Errorf("rendering src/search.ts: %w", err)
	}
	files = append(files, GeneratedFile{Path: "src/search.ts", Content: searchFile})

	// package.json
	pkgFile, err := renderTSTemplate("package.json", packageJSONTmpl, packageJSONData{
		ToolkitName: m.Metadata.Name,
	})
	if err != nil {
		return nil, fmt.Errorf("rendering package.json: %w", err)
	}
	files = append(files, GeneratedFile{Path: "package.json", Content: pkgFile})

	// tsconfig.json
	tsconfigFile, err := renderTSTemplate("tsconfig.json", tsconfigTmpl, nil)
	if err != nil {
		return nil, fmt.Errorf("rendering tsconfig.json: %w", err)
	}
	files = append(files, GeneratedFile{Path: "tsconfig.json", Content: tsconfigFile})

	// README.md
	readmeTSFile, err := renderTSTemplate("README.md", readmeTSTmpl, readmeTSData{
		ToolkitName:        m.Metadata.Name,
		ToolkitDescription: m.Metadata.Description,
	})
	if err != nil {
		return nil, fmt.Errorf("rendering README.md: %w", err)
	}
	files = append(files, GeneratedFile{Path: "README.md", Content: readmeTSFile})

	// Conditional: src/auth/middleware.ts — if any tool has token or oauth2 auth
	if hasAuthType(m, "token") || hasAuthType(m, "oauth2") {
		middlewareFile, err := renderTSTemplate("middleware.ts", middlewareTSTmpl, nil)
		if err != nil {
			return nil, fmt.Errorf("rendering src/auth/middleware.ts: %w", err)
		}
		files = append(files, GeneratedFile{Path: "src/auth/middleware.ts", Content: middlewareFile})
	}

	// Conditional: src/auth/metadata.ts — only if oauth2 AND streamable-http transport
	if hasAuthType(m, "oauth2") && hasStreamableHTTP {
		metaData := buildMetadataData(m)
		metadataFile, err := renderTSTemplate("metadata.ts", metadataTSTmpl, metaData)
		if err != nil {
			return nil, fmt.Errorf("rendering src/auth/metadata.ts: %w", err)
		}
		files = append(files, GeneratedFile{Path: "src/auth/metadata.ts", Content: metadataFile})
	}

	return files, nil
}

// ---------------------------------------------------------------------------
// Template data structs
// ---------------------------------------------------------------------------

type tsToolSummary struct {
	Name        string
	Description string
}

type tsArgData struct {
	Name        string
	TSType      string
	Required    bool
	Description string
}

type tsFlagData struct {
	Name         string
	TSType       string
	ZodType      string
	Required     bool
	Description  string
	ManifestType string
}

type tsAnnotations struct {
	ReadOnly    *bool
	Destructive *bool
	Idempotent  *bool
	OpenWorld   *bool
}

type tsToolData struct {
	ToolName        string
	Description     string
	Args            []tsArgData
	Flags           []tsFlagData
	HasAuth         bool
	AuthType        string
	TokenEnv        string
	TokenFlag       string
	Entrypoint      string
	HasAnnotations  bool
	Annotations     tsAnnotations
	Title           string
	HasOutputSchema bool
	OutputSchema    string // JSON string of the schema object
	SchemaPath      string // set when Schema is a file-path string
	IsBinaryOutput  bool
	IsImageMime     bool // true when MimeType starts with "image/"
	MimeType        string
}

type tsResourceSummary struct {
	Name string
	URI  string
}

type tsResourceData struct {
	Name        string
	Description string
	URI         string
	MimeType    string
	Entrypoint  string
	URIParams   []string // extracted from URI template {param} patterns
}

type indexData struct {
	ToolkitName       string
	Tools             []tsToolSummary
	Resources         []tsResourceSummary
	HasStdio          bool
	HasStreamableHTTP bool
}

type searchData struct {
	Tools []tsToolSummary
}

type packageJSONData struct {
	ToolkitName string
}

type readmeTSData struct {
	ToolkitName        string
	ToolkitDescription string
}

type metadataTSData struct {
	ProviderURL string
	Scopes      []string
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// tsType maps a manifest type string to a TypeScript type string.
func tsType(manifestType string) string {
	if manifest.IsArrayType(manifestType) {
		return tsType(manifest.BaseType(manifestType)) + "[]"
	}
	switch manifestType {
	case "int", "float":
		return "number"
	case "bool":
		return "boolean"
	case "object":
		return "Record<string, unknown>"
	default:
		return "string"
	}
}

// zodType maps a manifest type string to a Zod schema expression string.
func zodType(manifestType string) string {
	if manifest.IsArrayType(manifestType) {
		return "z.array(" + zodType(manifest.BaseType(manifestType)) + ")"
	}
	switch manifestType {
	case "int", "float":
		return "z.number()"
	case "bool":
		return "z.boolean()"
	default:
		return "z.string()"
	}
}

// requiredSet returns a set of property names listed in the JSON Schema "required" array.
func requiredSet(schema map[string]any) map[string]bool {
	req, _ := schema["required"].([]any)
	set := make(map[string]bool, len(req))
	for _, v := range req {
		if s, ok := v.(string); ok {
			set[s] = true
		}
	}
	return set
}

// buildZodObject converts a JSON Schema object (with optional "properties" and
// "required") into a z.object({...}) expression. Properties are emitted in
// alphabetical order. Non-required properties get .optional().
// If properties is nil or empty, returns z.record(z.unknown()).
// When compact is true, properties are rendered on a single line.
func buildZodObject(schema map[string]any, compact bool) string {
	propsRaw, _ := schema["properties"].(map[string]any)
	if len(propsRaw) == 0 {
		return "z.record(z.unknown())"
	}
	reqSet := requiredSet(schema)

	// Sort property names for deterministic output.
	names := make([]string, 0, len(propsRaw))
	for k := range propsRaw {
		names = append(names, k)
	}
	sort.Strings(names)

	if compact {
		// Single-line format: z.object({ p1: z.t1(), p2: z.t2().optional() })
		var parts []string
		for _, name := range names {
			propSchema, _ := propsRaw[name].(map[string]any)
			zodExpr := itemSchemaToZod(propSchema)
			if !reqSet[name] {
				zodExpr += ".optional()"
			}
			parts = append(parts, name+": "+zodExpr)
		}
		return "z.object({ " + strings.Join(parts, ", ") + " })"
	}

	// Multi-line format: each property on its own line.
	var sb strings.Builder
	sb.WriteString("z.object({\n")
	for _, name := range names {
		propSchema, _ := propsRaw[name].(map[string]any)
		zodExpr := itemSchemaToZod(propSchema)
		if !reqSet[name] {
			zodExpr += ".optional()"
		}
		sb.WriteString("  ")
		sb.WriteString(name)
		sb.WriteString(": ")
		sb.WriteString(zodExpr)
		sb.WriteString(",\n")
	}
	sb.WriteString("})")
	return sb.String()
}

// itemSchemaToZod converts a JSON Schema map to a Zod schema expression string.
// It handles the primitive types (string, number, integer, boolean), array, and object.
func itemSchemaToZod(schema map[string]any) string {
	if schema == nil {
		return "z.unknown()"
	}
	typeVal, _ := schema["type"].(string)
	switch typeVal {
	case "string":
		return "z.string()"
	case "number", "integer":
		return "z.number()"
	case "boolean":
		return "z.boolean()"
	case "array":
		items, _ := schema["items"].(map[string]any)
		if items != nil {
			return "z.array(" + itemSchemaToZod(items) + ")"
		}
		return "z.array(z.unknown())"
	case "object":
		return buildZodObject(schema, false)
	default:
		return "z.unknown()"
	}
}

// objectZodType returns the Zod expression for an object or object[] flag,
// using itemSchemaToZod when itemSchema is present and non-empty, or
// z.record(z.unknown()) when absent.
// compact controls whether z.object properties are on one line (for optional flags)
// or on separate lines (for required flags).
func objectZodType(flagType string, itemSchema map[string]any, compact bool) string {
	isArray := manifest.IsArrayType(flagType)
	var inner string
	if len(itemSchema) > 0 {
		typeVal, _ := itemSchema["type"].(string)
		if typeVal == "object" {
			inner = buildZodObject(itemSchema, compact)
		} else {
			inner = itemSchemaToZod(itemSchema)
		}
	} else {
		inner = "z.record(z.unknown())"
	}
	if isArray {
		return "z.array(" + inner + ")"
	}
	return inner
}

// containsTransport returns true if the transport slice contains the given value.
func containsTransport(transport []string, value string) bool {
	for _, t := range transport {
		if t == value {
			return true
		}
	}
	return false
}

// buildIndexData constructs indexData for the index.ts template.
func buildIndexData(m manifest.Toolkit, hasStdio, hasStreamableHTTP bool) indexData {
	summaries := make([]tsToolSummary, len(m.Tools))
	for i, t := range m.Tools {
		summaries[i] = tsToolSummary{Name: t.Name, Description: t.Description}
	}
	resourceSummaries := make([]tsResourceSummary, len(m.Resources))
	for i, r := range m.Resources {
		resourceSummaries[i] = tsResourceSummary{Name: r.Name, URI: r.URI}
	}
	return indexData{
		ToolkitName:       m.Metadata.Name,
		Tools:             summaries,
		Resources:         resourceSummaries,
		HasStdio:          hasStdio,
		HasStreamableHTTP: hasStreamableHTTP,
	}
}

// buildTSToolData constructs tsToolData for a single tool.
func buildTSToolData(tool manifest.Tool, auth manifest.Auth) (tsToolData, error) {
	args := make([]tsArgData, len(tool.Args))
	for i, a := range tool.Args {
		args[i] = tsArgData{
			Name:        a.Name,
			TSType:      tsType(a.Type),
			Required:    a.Required,
			Description: a.Description,
		}
	}
	flags := make([]tsFlagData, len(tool.Flags))
	for i, f := range tool.Flags {
		baseType := manifest.BaseType(f.Type)
		if baseType == "" {
			baseType = f.Type
		}
		var zodExpr string
		if baseType == "object" {
			// Use compact (single-line) format for optional flags so the flag-level
			// .optional() appears on the same line as the flag name.
			zodExpr = objectZodType(f.Type, f.ItemSchema, !f.Required)
		} else {
			zodExpr = zodType(f.Type)
		}
		flags[i] = tsFlagData{
			Name:         f.Name,
			TSType:       tsType(f.Type),
			ZodType:      zodExpr,
			Required:     f.Required,
			Description:  f.Description,
			ManifestType: f.Type,
		}
	}
	hasAuth := auth.Type == "token" || auth.Type == "oauth2"

	var annot tsAnnotations
	var title string
	hasAnnotations := false
	if tool.Annotations != nil {
		annot = tsAnnotations{
			ReadOnly:    tool.Annotations.ReadOnly,
			Destructive: tool.Annotations.Destructive,
			Idempotent:  tool.Annotations.Idempotent,
			OpenWorld:   tool.Annotations.OpenWorld,
		}
		title = tool.Annotations.Title
		hasAnnotations = annot.ReadOnly != nil || annot.Destructive != nil ||
			annot.Idempotent != nil || annot.OpenWorld != nil ||
			title != ""
	}

	// Output schema
	var hasOutputSchema bool
	var outputSchemaJSON string
	var schemaPath string
	if schemaMap, ok := tool.Output.Schema.(map[string]any); ok {
		jsonBytes, err := json.Marshal(schemaMap)
		if err != nil {
			return tsToolData{}, fmt.Errorf("marshaling output schema for tool %q: %w", tool.Name, err)
		}
		hasOutputSchema = true
		outputSchemaJSON = string(jsonBytes)
	} else if s, ok := tool.Output.Schema.(string); ok {
		schemaPath = s
	}

	// Binary output
	isBinaryOutput := tool.Output.Format == "binary"
	mimeType := ""
	isImageMime := false
	if isBinaryOutput {
		mimeType = tool.Output.MimeType
		isImageMime = strings.HasPrefix(mimeType, "image/")
	}

	return tsToolData{
		ToolName:        tool.Name,
		Description:     tool.Description,
		Args:            args,
		Flags:           flags,
		HasAuth:         hasAuth,
		AuthType:        auth.Type,
		TokenEnv:        auth.TokenEnv,
		TokenFlag:       auth.TokenFlag,
		Entrypoint:      tool.Entrypoint,
		HasAnnotations:  hasAnnotations,
		Annotations:     annot,
		Title:           title,
		HasOutputSchema: hasOutputSchema,
		OutputSchema:    outputSchemaJSON,
		SchemaPath:      schemaPath,
		IsBinaryOutput:  isBinaryOutput,
		IsImageMime:     isImageMime,
		MimeType:        mimeType,
	}, nil
}

// buildSearchData constructs searchData for the search.ts template.
func buildSearchData(m manifest.Toolkit) searchData {
	summaries := make([]tsToolSummary, len(m.Tools))
	for i, t := range m.Tools {
		summaries[i] = tsToolSummary{Name: t.Name, Description: t.Description}
	}
	return searchData{Tools: summaries}
}

// buildMetadataData constructs metadataTSData from the manifest's oauth2 auth config.
func buildMetadataData(m manifest.Toolkit) metadataTSData {
	var providerURL string
	var scopes []string
	for _, tool := range m.Tools {
		auth := m.ResolvedAuth(tool)
		if auth.Type == "oauth2" {
			providerURL = auth.ProviderURL
			scopes = auth.Scopes
			break
		}
	}
	return metadataTSData{
		ProviderURL: providerURL,
		Scopes:      scopes,
	}
}

// renderTSTemplate executes a named template with the given data and returns the
// rendered bytes.
func renderTSTemplate(name, tmplStr string, data any) ([]byte, error) {
	funcMap := template.FuncMap{
		"joinStrings": strings.Join,
		"tsType":      tsType,
		"esc":         escStringLiteral,
		"joinEsc":     joinEscStringLiterals,
		"derefBool": func(b *bool) string {
			if b == nil {
				// Unreachable: template {{if}} guards ensure non-nil before calling.
				return "false"
			}
			if *b {
				return "true"
			}
			return "false"
		},
	}
	t, err := template.New(name).Funcs(funcMap).Parse(tmplStr)
	if err != nil {
		return nil, fmt.Errorf("parsing template %q: %w", name, err)
	}
	var buf strings.Builder
	if err := t.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("executing template %q: %w", name, err)
	}
	return []byte(buf.String()), nil
}

// ---------------------------------------------------------------------------
// Templates
// ---------------------------------------------------------------------------

const indexTSTmpl = `import { McpServer{{if .Resources}}, ResourceTemplate{{end}} } from "@modelcontextprotocol/sdk/server/mcp.js";
{{- if .HasStdio}}
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
{{- end}}
{{- if .HasStreamableHTTP}}
import { StreamableHTTPServerTransport } from "@modelcontextprotocol/sdk/server/streamableHttp.js";
{{- end}}
import { z } from "zod";

// Import tool handlers
{{- range .Tools}}
import { register as register_{{.Name}} } from "./tools/{{.Name}}.js";
{{- end}}
import { register as register_search_tools } from "./search.js";
{{- if .Resources}}

// Import resource handlers
{{- range .Resources}}
import { handle as handle_{{.Name}} } from "./resources/{{.Name}}.js";
{{- end}}
{{- end}}

// Create the MCP server for {{.ToolkitName}}
const server = new McpServer({
  name: "{{.ToolkitName}}",
  version: "1.0.0",
});

// Register all tools — each register() call invokes server.tool() internally
{{- range .Tools}}
register_{{.Name}}(server);
{{- end}}
register_search_tools(server);
{{- if .Resources}}

// Register all resources
{{- range .Resources}}
server.resource(
  "{{.Name | esc}}",
  new ResourceTemplate("{{.URI | esc}}", { list: undefined }),
  handle_{{.Name}},
);
{{- end}}
{{- end}}

// Start the server with the configured transport
async function main() {
{{- if .HasStdio}}
  const stdioTransport = new StdioServerTransport();
  await server.connect(stdioTransport);
  console.error("{{.ToolkitName}} MCP server running via stdio");
{{- end}}
{{- if .HasStreamableHTTP}}
  // Streamable HTTP transport setup
  const httpTransport = new StreamableHTTPServerTransport({ sessionIdGenerator: undefined });
  await server.connect(httpTransport);
  console.error("{{.ToolkitName}} MCP server ready for streamable-http connections");
{{- end}}
}

main().catch(console.error);

export { server };
`

const tsToolTmpl = `import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { z } from "zod";
import { execFile as execFileCb } from "node:child_process";
import { promisify } from "node:util";

const execFile = promisify(execFileCb);

// Tool: {{.ToolName}}
// Description: {{.Description}}
{{- if .SchemaPath}}
// Output schema: {{.SchemaPath | esc}} (resolved at build time)
{{- end}}

// Input schema for {{.ToolName}}
const inputSchema = z.object({
{{- range .Args}}
  {{.Name}}: z.{{.TSType}}().describe("{{.Description | esc}}"),{{if not .Required}}// optional{{end}}
{{- end}}
{{- range .Flags}}
  {{.Name}}: {{.ZodType}}{{if not .Required}}.optional(){{end}}.describe("{{.Description | esc}}"),
{{- end}}
});

type {{.ToolName}}Input = z.infer<typeof inputSchema>;

/**
 * Handler for the {{.ToolName}} tool.
 * {{.Description}}
 */
{{- if .IsBinaryOutput}}
async function handle_{{.ToolName}}(input: {{.ToolName}}Input): Promise<{ content: Array<Record<string, unknown>> }> {
{{- else}}
async function handle_{{.ToolName}}(input: {{.ToolName}}Input): Promise<{ content: Array<{ type: string; text: string }> }> {
{{- end}}
{{- if .HasAuth}}
  // Resolve auth: read from environment variable {{.TokenEnv}}
  const envToken = process.env["{{.TokenEnv | esc}}"];
  if (!envToken) {
    throw new Error("auth required: set the {{.TokenEnv | esc}} environment variable");
  }
{{- end}}
  // Empty entrypoint guard
  const entrypoint = "{{.Entrypoint | esc}}";
  if (!entrypoint) {
    throw new Error("{{.ToolName | esc}}: entrypoint not configured");
  }
  const args: string[] = [];
  // Positional args first
{{- range .Args}}
  args.push(String(input.{{.Name}}));
{{- end}}
  // Flags in definition order
{{- range .Flags}}
{{- if eq .ManifestType "bool"}}
  if (input.{{.Name}} === true) {
    args.push("--{{.Name}}");
  }
{{- else if or (eq .ManifestType "object") (eq .ManifestType "object[]")}}
  if (input.{{.Name}} !== undefined) {
    args.push("--{{.Name}}", JSON.stringify(input.{{.Name}}));
  }
{{- else if or (eq .ManifestType "string[]") (eq .ManifestType "int[]") (eq .ManifestType "float[]") (eq .ManifestType "bool[]")}}
  if (input.{{.Name}} !== undefined) {
    for (const v of input.{{.Name}}) {
      args.push("--{{.Name}}", String(v));
    }
  }
{{- else}}
  if (input.{{.Name}} !== undefined) {
    args.push("--{{.Name}}", String(input.{{.Name}}));
  }
{{- end}}
{{- end}}
{{- if .HasAuth}}
  // Auth token last (via CLI flag, constitution rule 24)
  args.push("{{.TokenFlag | esc}}", envToken);
{{- end}}
  // Execute the entrypoint
{{- if .IsBinaryOutput}}
  const { stdout } = await execFile(entrypoint, args, { encoding: "buffer" });
  const base64Data = Buffer.from(stdout).toString("base64");
{{- if .IsImageMime}}
  return {
    content: [{
      type: "image",
      data: base64Data,
      mimeType: "{{.MimeType | esc}}",
    }],
  };
{{- else}}
  return {
    content: [{
      type: "resource",
      resource: {
        uri: "data:{{.MimeType | esc}};base64," + base64Data,
        mimeType: "{{.MimeType | esc}}",
      },
    }],
  };
{{- end}}
{{- else}}
  const { stdout } = await execFile(entrypoint, args);
  return {
    content: [{ type: "text", text: stdout }],
  };
{{- end}}
}

/**
 * Register the {{.ToolName}} tool with the MCP server.
 */
export function register(server: McpServer): void {
  server.tool(
    "{{.ToolName | esc}}",
    "{{.Description | esc}}",
    inputSchema.shape,
{{- if or .HasAnnotations .HasOutputSchema}}
    {
{{- if or .Annotations.ReadOnly .Annotations.Destructive .Annotations.Idempotent .Annotations.OpenWorld}}
      annotations: {
{{- /* Go templates treat non-nil *bool as truthy; derefBool dereferences to the actual value. */}}
{{- if .Annotations.ReadOnly}}
        readOnlyHint: {{derefBool .Annotations.ReadOnly}},
{{- end}}
{{- if .Annotations.Destructive}}
        destructiveHint: {{derefBool .Annotations.Destructive}},
{{- end}}
{{- if .Annotations.Idempotent}}
        idempotentHint: {{derefBool .Annotations.Idempotent}},
{{- end}}
{{- if .Annotations.OpenWorld}}
        openWorldHint: {{derefBool .Annotations.OpenWorld}},
{{- end}}
      },
{{- end}}
{{- if .Title}}
      title: "{{.Title | esc}}",
{{- end}}
{{- if .HasOutputSchema}}
      outputSchema: {{.OutputSchema}},
{{- end}}
    },
{{- end}}
    async (input: {{.ToolName}}Input) => {
      return handle_{{.ToolName}}(input);
    },
  );
}

export default handle_{{.ToolName}};
`

const tsResourceTmpl = `import { execFile as execFileCb } from "node:child_process";
import { promisify } from "node:util";

const execFile = promisify(execFileCb);

// Resource: {{.Name}}
// Description: {{.Description}}
// URI template: {{.URI}}

/**
 * Handler for the {{.Name}} resource.
 * Executes {{.Entrypoint}} with URI parameters as arguments.
 */
export async function handle(
  uri: URL,
  { {{joinStrings .URIParams ", "}} }: { {{- range $i, $p := .URIParams}}{{if $i}}, {{end}}{{$p}}: string{{end}} },
): Promise<{ contents: Array<{ uri: string; mimeType: string; text: string }> }> {
  const { stdout } = await execFile("{{.Entrypoint | esc}}", [{{- range $i, $p := .URIParams}}{{if $i}}, {{end}}{{$p}}{{end}}]);
  return {
    contents: [
      {
        uri: uri.href,
        mimeType: "{{.MimeType | esc}}",
        text: stdout,
      },
    ],
  };
}
`

const searchTSTmpl = `import { McpServer } from "@modelcontextprotocol/sdk/server/mcp.js";
import { z } from "zod";

// search_tools — progressive discovery meta-tool
// Allows agents to search available tools by name and description before
// fetching full schemas. This enables progressive discovery: call search_tools
// first to find relevant tools, then call the specific tool.

interface ToolEntry {
  name: string;
  description: string;
}

// Registry of all available tools with their names and descriptions
const toolRegistry: ToolEntry[] = [
{{- range .Tools}}
  { name: "{{.Name | esc}}", description: "{{.Description | esc}}" },
{{- end}}
];

/**
 * Search available tools by query string.
 * Returns tools whose name or description matches the query.
 * Supports progressive discovery by exposing tool names and descriptions.
 */
export function searchTools(query: string): ToolEntry[] {
  if (!query || query.trim() === "") {
    return toolRegistry;
  }
  const q = query.toLowerCase();
  return toolRegistry.filter(
    (t) => t.name.toLowerCase().includes(q) || t.description.toLowerCase().includes(q),
  );
}

/**
 * Register the search_tools meta-tool with the MCP server.
 * This tool enables agents to list and search available tools for progressive discovery.
 */
export function register(server: McpServer): void {
  server.tool(
    "search_tools",
    "Search available tools by name and description for progressive discovery",
    {
      query: z.string().optional().describe("Search query to filter tools by name or description"),
    },
    async (input: { query?: string }) => {
      const results = searchTools(input.query ?? "");
      return {
        content: [
          {
            type: "text",
            text: JSON.stringify(results, null, 2),
          },
        ],
      };
    },
  );
}
`

const packageJSONTmpl = `{
  "name": "{{.ToolkitName}}",
  "version": "1.0.0",
  "description": "MCP server generated by toolwright",
  "type": "module",
  "scripts": {
    "build": "tsc",
    "start": "node dist/index.js",
    "dev": "ts-node src/index.ts"
  },
  "dependencies": {
    "@modelcontextprotocol/sdk": "^1.0.0",
    "zod": "^3.22.0"
  },
  "devDependencies": {
    "typescript": "^5.3.0",
    "@types/node": "^20.0.0",
    "ts-node": "^10.9.0"
  }
}
`

const tsconfigTmpl = `{
  "compilerOptions": {
    "target": "ES2022",
    "module": "Node16",
    "moduleResolution": "Node16",
    "outDir": "./dist",
    "rootDir": "./src",
    "strict": true,
    "esModuleInterop": true,
    "skipLibCheck": true,
    "declaration": true,
    "declarationMap": true,
    "sourceMap": true
  },
  "include": ["src/**/*"],
  "exclude": ["node_modules", "dist"]
}
`

const readmeTSTmpl = `# {{.ToolkitName}}

{{.ToolkitDescription}}

## Setup

` + "```" + `sh
npm install
npm run build
` + "```" + `

## Usage

` + "```" + `sh
npm start
` + "```" + `

## Tools

This MCP server was generated by [toolwright](https://github.com/Obsidian-Owl/toolwright).
`

const middlewareTSTmpl = `import { IncomingMessage, ServerResponse } from "node:http";

// The Authorization header scheme used by this server.
const AUTH_SCHEME = "Bearer";

/**
 * validateBearerToken extracts and validates the Authorization header.
 * Returns the token string if valid, or throws an error for unauthorized requests.
 */
export function validateBearerToken(authHeader: string | undefined): string {
  if (!authHeader) {
    throw new UnauthorizedError("Missing Authorization header");
  }
  const schemePrefix = AUTH_SCHEME + " ";
  if (!authHeader.startsWith(schemePrefix)) {
    throw new UnauthorizedError("Authorization header must specify a valid auth token");
  }
  const token = authHeader.slice(schemePrefix.length).trim();
  if (!token) {
    throw new UnauthorizedError("Auth token value is empty");
  }
  return token;
}

/**
 * UnauthorizedError represents a 401 unauthorized error.
 */
export class UnauthorizedError extends Error {
  readonly statusCode = 401;

  constructor(message: string) {
    super(message);
    this.name = "UnauthorizedError";
  }
}

/**
 * validateRequest is HTTP middleware that enforces token authentication.
 * Responds with HTTP 401 if the Authorization header is missing or invalid.
 */
export function validateRequest(
  req: IncomingMessage,
  res: ServerResponse,
  next: (err?: unknown) => void,
): void {
  try {
    validateBearerToken(req.headers["authorization"]);
    next();
  } catch (err) {
    if (err instanceof UnauthorizedError) {
      res.writeHead(401, { "Content-Type": "application/json" });
      res.end(JSON.stringify({ error: "Unauthorized", message: err.message }));
    } else {
      next(err);
    }
  }
}
`

const metadataTSTmpl = `import { IncomingMessage, ServerResponse } from "node:http";

// OAuth 2.0 Protected Resource Metadata (RFC 9728)
// Serves the /.well-known/oauth-protected-resource endpoint

const protectedResourceMetadata = {
  resource: process.env["RESOURCE_URL"] ?? "{{.ProviderURL | esc}}",
  authorization_servers: ["{{.ProviderURL | esc}}"],
  scopes_supported: [{{range $i, $s := .Scopes}}{{if $i}}, {{end}}"{{$s | esc}}"{{end}}],
  bearer_methods_supported: ["header"],
};

/**
 * serveProtectedResourceMetadata handles requests to
 * /.well-known/oauth-protected-resource and returns the PRM document.
 */
export function serveProtectedResourceMetadata(_req: IncomingMessage, res: ServerResponse): void {
  res.writeHead(200, { "Content-Type": "application/json" });
  res.end(JSON.stringify(protectedResourceMetadata));
}

/**
 * registerMetadataEndpoint registers the /.well-known/oauth-protected-resource
 * endpoint on the given HTTP router or compatible app.
 */
export function registerMetadataEndpoint(app: {
  get(path: string, handler: (req: IncomingMessage, res: ServerResponse) => void): void;
}): void {
  app.get("/.well-known/oauth-protected-resource", serveProtectedResourceMetadata);
}
`
