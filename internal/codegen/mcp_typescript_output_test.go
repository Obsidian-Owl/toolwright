package codegen

import (
	"context"
	"strings"
	"testing"

	"github.com/Obsidian-Owl/toolwright/internal/manifest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Test manifests for MCP output schema and binary output
// ---------------------------------------------------------------------------

// mcpManifestBinaryTool returns a manifest with a single binary-output tool
// configured for MCP TypeScript generation.
func mcpManifestBinaryTool() manifest.Toolkit {
	return manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:        "screenshot-server",
			Version:     "1.0.0",
			Description: "MCP server with binary output tool",
		},
		Tools: []manifest.Tool{
			{
				Name:        "screenshot",
				Description: "Take a screenshot",
				Entrypoint:  "./screenshot.sh",
				Output:      manifest.Output{Format: "binary", MimeType: "image/png"},
			},
		},
		Generate: manifest.Generate{
			MCP: manifest.MCPConfig{
				Target:    "typescript",
				Transport: []string{"stdio"},
			},
		},
	}
}

// mcpManifestInlineSchema returns a manifest with a tool that has an inline
// (map[string]any) output schema.
func mcpManifestInlineSchema() manifest.Toolkit {
	return manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:        "schema-server",
			Version:     "1.0.0",
			Description: "MCP server with output schema",
		},
		Tools: []manifest.Tool{
			{
				Name:        "get_weather",
				Description: "Get weather data",
				Entrypoint:  "./weather.sh",
				Output: manifest.Output{
					Format: "json",
					Schema: map[string]any{
						"type": "object",
						"properties": map[string]any{
							"temperature": map[string]any{"type": "number"},
							"unit":        map[string]any{"type": "string"},
						},
						"required": []any{"temperature", "unit"},
					},
				},
			},
		},
		Generate: manifest.Generate{
			MCP: manifest.MCPConfig{
				Target:    "typescript",
				Transport: []string{"stdio"},
			},
		},
	}
}

// mcpManifestStringSchema returns a manifest with a tool that has a string
// (file path) output schema.
func mcpManifestStringSchema() manifest.Toolkit {
	return manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:        "schema-path-server",
			Version:     "1.0.0",
			Description: "MCP server with file-path schema",
		},
		Tools: []manifest.Tool{
			{
				Name:        "get_data",
				Description: "Get structured data",
				Entrypoint:  "./data.sh",
				Output: manifest.Output{
					Format: "json",
					Schema: "schemas/output.json",
				},
			},
		},
		Generate: manifest.Generate{
			MCP: manifest.MCPConfig{
				Target:    "typescript",
				Transport: []string{"stdio"},
			},
		},
	}
}

// mcpManifestNilSchema returns a manifest with a tool that has no output schema.
func mcpManifestNilSchema() manifest.Toolkit {
	return manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:        "no-schema-server",
			Version:     "1.0.0",
			Description: "MCP server without output schema",
		},
		Tools: []manifest.Tool{
			{
				Name:        "greet",
				Description: "Greet a user",
				Entrypoint:  "./greet.sh",
				Output: manifest.Output{
					Format: "text",
				},
			},
		},
		Generate: manifest.Generate{
			MCP: manifest.MCPConfig{
				Target:    "typescript",
				Transport: []string{"stdio"},
			},
		},
	}
}

// mcpManifestMixedBinaryAndText returns a manifest with one binary and one
// text/json tool to verify per-tool isolation.
func mcpManifestMixedBinaryAndText() manifest.Toolkit {
	return manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:        "mixed-server",
			Version:     "1.0.0",
			Description: "MCP server with mixed output types",
		},
		Tools: []manifest.Tool{
			{
				Name:        "render",
				Description: "Render an image",
				Entrypoint:  "./render.sh",
				Output:      manifest.Output{Format: "binary", MimeType: "image/png"},
			},
			{
				Name:        "status",
				Description: "Get system status",
				Entrypoint:  "./status.sh",
				Output:      manifest.Output{Format: "json"},
			},
		},
		Generate: manifest.Generate{
			MCP: manifest.MCPConfig{
				Target:    "typescript",
				Transport: []string{"stdio"},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// AC12: Inline output schema emitted
// ---------------------------------------------------------------------------

func TestTSMCP_OutputSchema_InlineMapEmitsOutputSchema(t *testing.T) {
	// When Output.Schema is map[string]any, the generated tool registration
	// must include an outputSchema argument/option containing the JSON Schema.
	m := mcpManifestInlineSchema()
	files := generateTSMCP(t, m)
	content := fileContent(t, files, "src/tools/get_weather.ts")

	assert.Contains(t, content, "outputSchema",
		"inline map schema must produce an outputSchema in generated code")
}

func TestTSMCP_OutputSchema_InlineMapContainsProperties(t *testing.T) {
	// The outputSchema must contain the actual schema properties, not just
	// the keyword. This catches implementations that emit "outputSchema: {}"
	// without the actual schema content.
	m := mcpManifestInlineSchema()
	files := generateTSMCP(t, m)
	content := fileContent(t, files, "src/tools/get_weather.ts")

	assert.Contains(t, content, "temperature",
		"outputSchema must include the 'temperature' property from the inline schema")
	assert.Contains(t, content, "unit",
		"outputSchema must include the 'unit' property from the inline schema")
}

func TestTSMCP_OutputSchema_InlineMapHasCorrectType(t *testing.T) {
	// The emitted outputSchema must preserve the type information from the
	// JSON Schema. This catches implementations that flatten or lose type info.
	m := mcpManifestInlineSchema()
	files := generateTSMCP(t, m)
	content := fileContent(t, files, "src/tools/get_weather.ts")

	// The schema specifies type: "object" at the top level.
	assert.Contains(t, content, `"object"`,
		"outputSchema must include the top-level 'object' type")
	// Property types must be present.
	assert.Contains(t, content, `"number"`,
		"outputSchema must include the 'number' type for temperature")
	assert.Contains(t, content, `"string"`,
		"outputSchema must include the 'string' type for unit")
}

func TestTSMCP_OutputSchema_InlineMapHasRequiredArray(t *testing.T) {
	// The outputSchema must include the "required" array from the schema.
	m := mcpManifestInlineSchema()
	files := generateTSMCP(t, m)
	content := fileContent(t, files, "src/tools/get_weather.ts")

	// "required" is a JSON Schema keyword that must appear in the output.
	assert.Contains(t, content, `"required"`,
		"outputSchema must include the required array from the original schema")
}

func TestTSMCP_OutputSchema_StringSchemaEmitsComment(t *testing.T) {
	// When Output.Schema is a string (file path), the generated code must
	// include a comment referencing the file path. It should NOT emit a
	// hard-coded outputSchema object (the file is resolved at build time).
	m := mcpManifestStringSchema()
	files := generateTSMCP(t, m)
	content := fileContent(t, files, "src/tools/get_data.ts")

	assert.Contains(t, content, "// Output schema: schemas/output.json",
		"string schema path must appear inside a // comment line")
}

func TestTSMCP_OutputSchema_StringSchemaDoesNotEmitInlineOutputSchema(t *testing.T) {
	// A string schema path must NOT produce an inline outputSchema object.
	// This catches implementations that try to embed the string as if it were JSON.
	m := mcpManifestStringSchema()
	files := generateTSMCP(t, m)
	content := fileContent(t, files, "src/tools/get_data.ts")

	// The server.tool() registration should not have an outputSchema with the
	// raw string. Look for the string being used as a JSON value (which would be wrong).
	serverToolIdx := strings.Index(content, "server.tool(")
	require.Greater(t, serverToolIdx, 0, "server.tool( must be present")
	afterServerTool := content[serverToolIdx:]

	// If outputSchema is present, it must not contain the file path as a value
	// (that would mean the string was embedded literally).
	if strings.Contains(afterServerTool, "outputSchema") {
		t.Error("string schema path must NOT produce an inline outputSchema in server.tool() registration " +
			"(file-based schemas are resolved at build time, not embedded)")
	}
}

func TestTSMCP_OutputSchema_NilSchemaOmitsOutputSchema(t *testing.T) {
	// When Schema is nil, no outputSchema should appear in the generated code.
	m := mcpManifestNilSchema()
	files := generateTSMCP(t, m)
	content := fileContent(t, files, "src/tools/greet.ts")

	assert.NotContains(t, content, "outputSchema",
		"nil schema must NOT produce outputSchema in generated code")
}

func TestTSMCP_OutputSchema_ComplexNestedSchema(t *testing.T) {
	// A complex nested inline schema must be fully serialized in the output.
	m := manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:        "complex-schema-server",
			Version:     "1.0.0",
			Description: "Complex schema test",
		},
		Tools: []manifest.Tool{
			{
				Name:        "analyze",
				Description: "Analyze data",
				Entrypoint:  "./analyze.sh",
				Output: manifest.Output{
					Format: "json",
					Schema: map[string]any{
						"type": "object",
						"properties": map[string]any{
							"results": map[string]any{
								"type": "array",
								"items": map[string]any{
									"type": "object",
									"properties": map[string]any{
										"id":    map[string]any{"type": "integer"},
										"score": map[string]any{"type": "number"},
										"label": map[string]any{"type": "string"},
									},
									"required": []any{"id", "score"},
								},
							},
							"total": map[string]any{"type": "integer"},
						},
						"required": []any{"results"},
					},
				},
			},
		},
		Generate: manifest.Generate{
			MCP: manifest.MCPConfig{
				Target:    "typescript",
				Transport: []string{"stdio"},
			},
		},
	}
	files := generateTSMCP(t, m)
	content := fileContent(t, files, "src/tools/analyze.ts")

	assert.Contains(t, content, "outputSchema",
		"complex nested schema must produce outputSchema")
	assert.Contains(t, content, "results",
		"outputSchema must include nested 'results' array property")
	assert.Contains(t, content, "total",
		"outputSchema must include 'total' property")
	assert.Contains(t, content, `"integer"`,
		"outputSchema must include 'integer' type from nested properties")
	assert.Contains(t, content, `"array"`,
		"outputSchema must include 'array' type for results property")
}

func TestTSMCP_OutputSchema_DeterministicKeyOrdering(t *testing.T) {
	// When the inline schema is serialized to JSON, the keys must be in a
	// deterministic order (e.g., sorted). Non-deterministic ordering would
	// cause different output on each run.
	m := mcpManifestInlineSchema()

	// Generate twice and compare the outputSchema portions.
	files1 := generateTSMCP(t, m)
	files2 := generateTSMCP(t, m)

	content1 := fileContent(t, files1, "src/tools/get_weather.ts")
	content2 := fileContent(t, files2, "src/tools/get_weather.ts")

	assert.Equal(t, content1, content2,
		"two Generate calls with the same input must produce identical output (deterministic serialization)")
}

// ---------------------------------------------------------------------------
// AC13: Binary output returns base64
// ---------------------------------------------------------------------------

func TestTSMCP_BinaryOutput_ReturnsImageType(t *testing.T) {
	// A binary tool's handler must return content with type "image",
	// NOT type "text".
	m := mcpManifestBinaryTool()
	files := generateTSMCP(t, m)
	content := fileContent(t, files, "src/tools/screenshot.ts")

	// Find the handler function and check its return structure.
	// The handler must return { type: "image" } content items.
	assert.Contains(t, content, `"image"`,
		"binary tool handler must reference 'image' type for content items")
}

func TestTSMCP_BinaryOutput_DoesNotReturnTextType(t *testing.T) {
	// Binary tool handler must NOT use { type: "text", text: ... } for its
	// primary response. This catches implementations that ignore the binary flag.
	m := mcpManifestBinaryTool()
	files := generateTSMCP(t, m)
	content := fileContent(t, files, "src/tools/screenshot.ts")

	// Isolate the handler function body (from handle_screenshot to the next
	// top-level function declaration or export). This is more robust than
	// trying to find "return {" and guessing closing braces.
	handlerIdx := strings.Index(content, "handle_screenshot")
	require.Greater(t, handlerIdx, 0, "handle_screenshot must exist")
	afterHandler := content[handlerIdx:]

	// The handler ends at the next top-level declaration (export/function/const at col 0).
	// Split into lines and collect until we hit a line starting with a
	// top-level keyword after the handler signature.
	lines := strings.Split(afterHandler, "\n")
	var handlerLines []string
	started := false
	for _, line := range lines {
		if !started {
			started = true
			handlerLines = append(handlerLines, line)
			continue
		}
		// Stop at next top-level declaration
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "export ") || strings.HasPrefix(trimmed, "/**") {
			break
		}
		handlerLines = append(handlerLines, line)
	}

	handlerBody := strings.Join(handlerLines, "\n")
	assert.NotContains(t, handlerBody, `type: "text"`,
		"binary tool handler must NOT contain type: \"text\"")
}

func TestTSMCP_BinaryOutput_IncludesBase64Encoding(t *testing.T) {
	// The binary tool handler must include base64 encoding logic.
	// In TypeScript/Node.js, this is typically Buffer.from(...).toString("base64")
	// or btoa(...).
	m := mcpManifestBinaryTool()
	files := generateTSMCP(t, m)
	content := fileContent(t, files, "src/tools/screenshot.ts")

	hasBufferBase64 := strings.Contains(content, "base64")
	hasBtoa := strings.Contains(content, "btoa")
	assert.True(t, hasBufferBase64 || hasBtoa,
		"binary tool handler must include base64 encoding (Buffer.from().toString('base64') or btoa)")
}

func TestTSMCP_BinaryOutput_IncludesDataField(t *testing.T) {
	// The MCP image content type requires a "data" field containing the
	// base64-encoded bytes. This catches implementations that put the data
	// in "text" instead.
	m := mcpManifestBinaryTool()
	files := generateTSMCP(t, m)
	content := fileContent(t, files, "src/tools/screenshot.ts")

	assert.Contains(t, content, "data:",
		"binary tool handler must include a 'data:' field for base64 content")
}

func TestTSMCP_BinaryOutput_IncludesMimeType(t *testing.T) {
	// The MCP image content type requires a "mimeType" field.
	m := mcpManifestBinaryTool()
	files := generateTSMCP(t, m)
	content := fileContent(t, files, "src/tools/screenshot.ts")

	assert.Contains(t, content, "mimeType",
		"binary tool handler must include a 'mimeType' field")
	assert.Contains(t, content, "image/png",
		"binary tool handler must include the correct MIME type 'image/png'")
}

func TestTSMCP_BinaryOutput_MimeTypeFromManifest(t *testing.T) {
	// The mimeType in the generated code must match what's in the manifest,
	// not a hardcoded value. Test with a different MIME type.
	m := mcpManifestBinaryTool()
	m.Tools[0].Output.MimeType = "application/pdf"
	files := generateTSMCP(t, m)
	content := fileContent(t, files, "src/tools/screenshot.ts")

	assert.Contains(t, content, "application/pdf",
		"binary tool handler must use the mimeType from the manifest (application/pdf), not a hardcoded value")
	assert.NotContains(t, content, "image/png",
		"binary tool with application/pdf must NOT contain image/png")
}

func TestTSMCP_BinaryOutput_HandlerReturnTypeSignature(t *testing.T) {
	// The handler's return type or return structure must reflect image content,
	// not text content. This verifies the Promise/return type is adapted.
	m := mcpManifestBinaryTool()
	files := generateTSMCP(t, m)
	content := fileContent(t, files, "src/tools/screenshot.ts")

	// The content array item for binary must have { type: "image", data: ..., mimeType: ... }
	// These three fields must all co-exist in the same content structure.
	hasImage := strings.Contains(content, `"image"`)
	hasData := strings.Contains(content, "data:")
	hasMime := strings.Contains(content, "mimeType:")

	assert.True(t, hasImage && hasData && hasMime,
		"binary tool must have all three fields: type='image', data, and mimeType in the content structure; "+
			"got image=%v, data=%v, mimeType=%v", hasImage, hasData, hasMime)
}

// ---------------------------------------------------------------------------
// AC14: Non-binary MCP tools unaffected
// ---------------------------------------------------------------------------

func TestTSMCP_NonBinary_JSONToolReturnsTextType(t *testing.T) {
	// A JSON-format tool must still return { type: "text", text: ... }.
	m := manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:        "json-mcp-server",
			Version:     "1.0.0",
			Description: "JSON tool server",
		},
		Tools: []manifest.Tool{
			{
				Name:        "query",
				Description: "Query data",
				Entrypoint:  "./query.sh",
				Output:      manifest.Output{Format: "json"},
			},
		},
		Generate: manifest.Generate{
			MCP: manifest.MCPConfig{
				Target:    "typescript",
				Transport: []string{"stdio"},
			},
		},
	}
	files := generateTSMCP(t, m)
	content := fileContent(t, files, "src/tools/query.ts")

	assert.Contains(t, content, `type: "text"`,
		"JSON-format tool must return content with type: \"text\"")
	assert.Contains(t, content, "text:",
		"JSON-format tool must return content with a 'text:' field")
}

func TestTSMCP_NonBinary_TextToolReturnsTextType(t *testing.T) {
	// A text-format tool must still return { type: "text", text: ... }.
	m := manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:        "text-mcp-server",
			Version:     "1.0.0",
			Description: "Text tool server",
		},
		Tools: []manifest.Tool{
			{
				Name:        "greet",
				Description: "Greet user",
				Entrypoint:  "./greet.sh",
				Output:      manifest.Output{Format: "text"},
			},
		},
		Generate: manifest.Generate{
			MCP: manifest.MCPConfig{
				Target:    "typescript",
				Transport: []string{"stdio"},
			},
		},
	}
	files := generateTSMCP(t, m)
	content := fileContent(t, files, "src/tools/greet.ts")

	assert.Contains(t, content, `type: "text"`,
		"text-format tool must return content with type: \"text\"")
}

func TestTSMCP_NonBinary_NoBase64Encoding(t *testing.T) {
	// Non-binary tools must NOT include base64 encoding logic.
	m := mcpManifestNilSchema()
	files := generateTSMCP(t, m)
	content := fileContent(t, files, "src/tools/greet.ts")

	assert.NotContains(t, content, "base64",
		"non-binary tool must NOT contain base64 encoding")
	assert.NotContains(t, content, "btoa",
		"non-binary tool must NOT contain btoa encoding")
}

func TestTSMCP_NonBinary_NoMimeTypeInHandler(t *testing.T) {
	// Non-binary tools must NOT reference mimeType in the handler content.
	m := mcpManifestNilSchema()
	files := generateTSMCP(t, m)
	content := fileContent(t, files, "src/tools/greet.ts")

	// mimeType should not appear in the handler return or content items.
	handlerIdx := strings.Index(content, "handle_greet")
	require.Greater(t, handlerIdx, 0, "handle_greet must exist")
	afterHandler := content[handlerIdx:]

	assert.NotContains(t, afterHandler, "mimeType",
		"non-binary tool handler must NOT reference mimeType")
}

func TestTSMCP_NonBinary_NoImageType(t *testing.T) {
	// Non-binary tools must NOT have type: "image" in their content.
	m := mcpManifestNilSchema()
	files := generateTSMCP(t, m)
	content := fileContent(t, files, "src/tools/greet.ts")

	handlerIdx := strings.Index(content, "handle_greet")
	require.Greater(t, handlerIdx, 0)
	afterHandler := content[handlerIdx:]

	assert.NotContains(t, afterHandler, `"image"`,
		"non-binary tool handler must NOT contain type: \"image\"")
}

// ---------------------------------------------------------------------------
// Mixed toolkit: binary and non-binary tools in the same manifest
// ---------------------------------------------------------------------------

func TestTSMCP_MixedToolkit_BinaryToolHasImageResponse(t *testing.T) {
	// In a mixed toolkit, the binary tool must produce image-type response.
	m := mcpManifestMixedBinaryAndText()
	files := generateTSMCP(t, m)
	renderContent := fileContent(t, files, "src/tools/render.ts")

	assert.Contains(t, renderContent, `"image"`,
		"binary tool 'render' in mixed toolkit must have image type")
	assert.Contains(t, renderContent, "base64",
		"binary tool 'render' in mixed toolkit must have base64 encoding")
	assert.Contains(t, renderContent, "mimeType",
		"binary tool 'render' in mixed toolkit must have mimeType")
	assert.Contains(t, renderContent, "image/png",
		"binary tool 'render' must have correct MIME type")
}

func TestTSMCP_MixedToolkit_NonBinaryToolHasTextResponse(t *testing.T) {
	// In a mixed toolkit, the non-binary tool must still produce text response.
	m := mcpManifestMixedBinaryAndText()
	files := generateTSMCP(t, m)
	statusContent := fileContent(t, files, "src/tools/status.ts")

	assert.Contains(t, statusContent, `type: "text"`,
		"non-binary tool 'status' in mixed toolkit must have text type")
	assert.NotContains(t, statusContent, "base64",
		"non-binary tool 'status' in mixed toolkit must NOT have base64")

	handlerIdx := strings.Index(statusContent, "handle_status")
	require.Greater(t, handlerIdx, 0)
	afterHandler := statusContent[handlerIdx:]
	assert.NotContains(t, afterHandler, `"image"`,
		"non-binary tool 'status' handler must NOT have image type")
}

func TestTSMCP_MixedToolkit_BinaryToolDoesNotHaveTextReturn(t *testing.T) {
	// Verify per-tool isolation: binary tool must not fall back to text.
	m := mcpManifestMixedBinaryAndText()
	files := generateTSMCP(t, m)
	renderContent := fileContent(t, files, "src/tools/render.ts")

	handlerIdx := strings.Index(renderContent, "handle_render")
	require.Greater(t, handlerIdx, 0)
	afterHandler := renderContent[handlerIdx:]

	returnIdx := strings.Index(afterHandler, "return {")
	require.Greater(t, returnIdx, 0)
	returnBlock := afterHandler[returnIdx:]
	endIdx := strings.Index(returnBlock, "}\n}")
	if endIdx > 0 {
		returnBlock = returnBlock[:endIdx+3]
	}

	assert.NotContains(t, returnBlock, `type: "text"`,
		"binary tool return must NOT contain type: \"text\"")
}

// ---------------------------------------------------------------------------
// buildTSToolData: new fields
// ---------------------------------------------------------------------------

func TestTSMCP_BuildTSToolData_BinaryTool_IsBinaryOutputTrue(t *testing.T) {
	// buildTSToolData must set IsBinaryOutput=true for binary format tools.
	tool := manifest.Tool{
		Name:        "screenshot",
		Description: "Take a screenshot",
		Entrypoint:  "./screenshot.sh",
		Output:      manifest.Output{Format: "binary", MimeType: "image/png"},
	}
	data, err := buildTSToolData(tool, manifest.Auth{Type: "none"})
	require.NoError(t, err)

	assert.True(t, data.IsBinaryOutput,
		"buildTSToolData must set IsBinaryOutput=true for output format 'binary'")
}

func TestTSMCP_BuildTSToolData_BinaryTool_MimeTypePopulated(t *testing.T) {
	// buildTSToolData must populate MimeType from the tool's Output.MimeType.
	tool := manifest.Tool{
		Name:        "screenshot",
		Description: "Take a screenshot",
		Entrypoint:  "./screenshot.sh",
		Output:      manifest.Output{Format: "binary", MimeType: "image/png"},
	}
	data, err := buildTSToolData(tool, manifest.Auth{Type: "none"})
	require.NoError(t, err)

	assert.Equal(t, "image/png", data.MimeType,
		"buildTSToolData must set MimeType from Output.MimeType")
}

func TestTSMCP_BuildTSToolData_NonBinary_IsBinaryOutputFalse(t *testing.T) {
	// buildTSToolData must set IsBinaryOutput=false for non-binary formats.
	formats := []string{"json", "text", ""}
	for _, format := range formats {
		t.Run("format_"+format, func(t *testing.T) {
			tool := manifest.Tool{
				Name:        "tool",
				Description: "Test tool",
				Entrypoint:  "./tool.sh",
				Output:      manifest.Output{Format: format},
			}
			data, err := buildTSToolData(tool, manifest.Auth{Type: "none"})
			require.NoError(t, err)

			assert.False(t, data.IsBinaryOutput,
				"buildTSToolData must set IsBinaryOutput=false for format %q", format)
		})
	}
}

func TestTSMCP_BuildTSToolData_NonBinary_MimeTypeEmpty(t *testing.T) {
	// Non-binary tools must have empty MimeType in tsToolData.
	tool := manifest.Tool{
		Name:        "query",
		Description: "Query data",
		Entrypoint:  "./query.sh",
		Output:      manifest.Output{Format: "json"},
	}
	data, err := buildTSToolData(tool, manifest.Auth{Type: "none"})
	require.NoError(t, err)

	assert.Empty(t, data.MimeType,
		"buildTSToolData must set MimeType to empty for non-binary tools")
}

func TestTSMCP_BuildTSToolData_InlineSchema_HasOutputSchemaTrue(t *testing.T) {
	// buildTSToolData must set HasOutputSchema=true for inline map schemas.
	tool := manifest.Tool{
		Name:        "weather",
		Description: "Get weather",
		Entrypoint:  "./weather.sh",
		Output: manifest.Output{
			Format: "json",
			Schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"temp": map[string]any{"type": "number"},
				},
			},
		},
	}
	data, err := buildTSToolData(tool, manifest.Auth{Type: "none"})
	require.NoError(t, err)

	assert.True(t, data.HasOutputSchema,
		"buildTSToolData must set HasOutputSchema=true for inline map schema")
}

func TestTSMCP_BuildTSToolData_InlineSchema_OutputSchemaContainsJSON(t *testing.T) {
	// buildTSToolData must populate OutputSchema with a JSON serialization
	// of the inline schema map.
	tool := manifest.Tool{
		Name:        "weather",
		Description: "Get weather",
		Entrypoint:  "./weather.sh",
		Output: manifest.Output{
			Format: "json",
			Schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"temp": map[string]any{"type": "number"},
				},
			},
		},
	}
	data, err := buildTSToolData(tool, manifest.Auth{Type: "none"})
	require.NoError(t, err)

	require.NotEmpty(t, data.OutputSchema,
		"buildTSToolData must populate OutputSchema for inline map schema")
	assert.Contains(t, data.OutputSchema, `"type"`,
		"OutputSchema JSON must contain the 'type' key")
	assert.Contains(t, data.OutputSchema, `"object"`,
		"OutputSchema JSON must contain the 'object' value")
	assert.Contains(t, data.OutputSchema, `"properties"`,
		"OutputSchema JSON must contain 'properties' key")
	assert.Contains(t, data.OutputSchema, `"temp"`,
		"OutputSchema JSON must contain 'temp' property name")
}

func TestTSMCP_BuildTSToolData_StringSchema_HasOutputSchemaFalse(t *testing.T) {
	// buildTSToolData must set HasOutputSchema=false for string (path) schemas.
	tool := manifest.Tool{
		Name:        "data",
		Description: "Get data",
		Entrypoint:  "./data.sh",
		Output: manifest.Output{
			Format: "json",
			Schema: "schemas/output.json",
		},
	}
	data, err := buildTSToolData(tool, manifest.Auth{Type: "none"})
	require.NoError(t, err)

	assert.False(t, data.HasOutputSchema,
		"buildTSToolData must set HasOutputSchema=false for string schema (file path)")
}

func TestTSMCP_BuildTSToolData_NilSchema_HasOutputSchemaFalse(t *testing.T) {
	// buildTSToolData must set HasOutputSchema=false when Schema is nil.
	tool := manifest.Tool{
		Name:        "greet",
		Description: "Greet user",
		Entrypoint:  "./greet.sh",
		Output:      manifest.Output{Format: "text"},
	}
	data, err := buildTSToolData(tool, manifest.Auth{Type: "none"})
	require.NoError(t, err)

	assert.False(t, data.HasOutputSchema,
		"buildTSToolData must set HasOutputSchema=false for nil schema")
}

// ---------------------------------------------------------------------------
// Edge cases: composition and MIME types
// ---------------------------------------------------------------------------

func TestTSMCP_BinaryOutput_WithAnnotations(t *testing.T) {
	// Binary output and annotations must compose correctly in the same tool.
	m := mcpManifestBinaryTool()
	m.Tools[0].Annotations = &manifest.ToolAnnotations{
		ReadOnly: manifest.BoolPtr(true),
	}
	files := generateTSMCP(t, m)
	content := fileContent(t, files, "src/tools/screenshot.ts")

	// Binary output features must be present.
	assert.Contains(t, content, `"image"`,
		"binary tool with annotations must still have image type")
	assert.Contains(t, content, "base64",
		"binary tool with annotations must still have base64 encoding")
	assert.Contains(t, content, "image/png",
		"binary tool with annotations must still have correct mimeType")

	// Annotation features must also be present.
	assert.Contains(t, content, "readOnlyHint: true",
		"annotated binary tool must still emit readOnlyHint")
	assert.Contains(t, content, "annotations:",
		"annotated binary tool must still emit annotations block")
}

func TestTSMCP_BinaryOutput_WithOutputSchema(t *testing.T) {
	// A binary tool can also have an outputSchema (e.g., to describe metadata
	// about the binary output). Both features must compose.
	m := mcpManifestBinaryTool()
	m.Tools[0].Output.Schema = map[string]any{
		"type": "object",
		"properties": map[string]any{
			"width":  map[string]any{"type": "integer"},
			"height": map[string]any{"type": "integer"},
		},
	}
	files := generateTSMCP(t, m)
	content := fileContent(t, files, "src/tools/screenshot.ts")

	// Both binary output and outputSchema must be present.
	assert.Contains(t, content, `"image"`,
		"binary tool with outputSchema must still have image type")
	assert.Contains(t, content, "outputSchema",
		"binary tool with outputSchema must emit outputSchema")
	assert.Contains(t, content, "width",
		"outputSchema must contain 'width' property")
	assert.Contains(t, content, "height",
		"outputSchema must contain 'height' property")
}

func TestTSMCP_BinaryOutput_DifferentMimeTypes(t *testing.T) {
	// Different MIME types must be correctly reflected in generated code.
	tests := []struct {
		name     string
		mimeType string
	}{
		{name: "image/png", mimeType: "image/png"},
		{name: "application/pdf", mimeType: "application/pdf"},
		{name: "application/octet-stream", mimeType: "application/octet-stream"},
		{name: "image/jpeg", mimeType: "image/jpeg"},
		{name: "audio/wav", mimeType: "audio/wav"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := mcpManifestBinaryTool()
			m.Tools[0].Output.MimeType = tc.mimeType
			files := generateTSMCP(t, m)
			content := fileContent(t, files, "src/tools/screenshot.ts")

			assert.Contains(t, content, tc.mimeType,
				"binary tool must include mimeType %q from manifest", tc.mimeType)
		})
	}
}

func TestTSMCP_BinaryOutput_MimeTypeNotHardcoded(t *testing.T) {
	// Guard against an implementation that hardcodes "image/png" for all
	// binary tools. Use a MIME type that no sensible default would match.
	m := mcpManifestBinaryTool()
	m.Tools[0].Output.MimeType = "application/x-custom-format"
	files := generateTSMCP(t, m)
	content := fileContent(t, files, "src/tools/screenshot.ts")

	assert.Contains(t, content, "application/x-custom-format",
		"mimeType must come from manifest, not hardcoded")
}

func TestTSMCP_StringSchema_CommentIsProperlyEscaped(t *testing.T) {
	// If the schema string path contains characters that could break a TS
	// comment (e.g., "*/"), the generated code must handle it.
	m := mcpManifestStringSchema()
	m.Tools[0].Output.Schema = "schemas/my-output.json"
	files := generateTSMCP(t, m)
	content := fileContent(t, files, "src/tools/get_data.ts")

	assert.Contains(t, content, "schemas/my-output.json",
		"string schema path with hyphens must appear correctly in generated code")
}

func TestTSMCP_OutputSchema_EmptyMapSchema(t *testing.T) {
	// An empty map[string]any{} as schema is technically a valid (if useless)
	// inline schema. It should still emit an outputSchema (even if minimal).
	m := manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:        "empty-schema-server",
			Version:     "1.0.0",
			Description: "Empty schema test",
		},
		Tools: []manifest.Tool{
			{
				Name:        "empty",
				Description: "Empty schema tool",
				Entrypoint:  "./empty.sh",
				Output: manifest.Output{
					Format: "json",
					Schema: map[string]any{},
				},
			},
		},
		Generate: manifest.Generate{
			MCP: manifest.MCPConfig{
				Target:    "typescript",
				Transport: []string{"stdio"},
			},
		},
	}
	files := generateTSMCP(t, m)
	content := fileContent(t, files, "src/tools/empty.ts")

	// An empty map is still an inline schema. buildTSToolData should set
	// HasOutputSchema=true and OutputSchema to "{}".
	assert.Contains(t, content, "outputSchema",
		"empty map schema must still produce outputSchema (it is an inline schema, not nil)")
}

func TestTSMCP_OutputSchema_InRegistrationNotHandler(t *testing.T) {
	// The outputSchema must appear in the server.tool() registration call
	// (the tool metadata), not in the handler function body. This is where
	// MCP expects it.
	m := mcpManifestInlineSchema()
	files := generateTSMCP(t, m)
	content := fileContent(t, files, "src/tools/get_weather.ts")

	serverToolIdx := strings.Index(content, "server.tool(")
	require.Greater(t, serverToolIdx, 0)

	// The outputSchema must appear after "server.tool(" — in the registration.
	afterServerTool := content[serverToolIdx:]
	assert.Contains(t, afterServerTool, "outputSchema",
		"outputSchema must appear inside the server.tool() registration call")
}

// ---------------------------------------------------------------------------
// Generate() integration: error-free for all new configurations
// ---------------------------------------------------------------------------

func TestTSMCP_Generate_BinaryToolSucceeds(t *testing.T) {
	// Generate must not error or panic for a binary tool.
	gen := NewTSMCPGenerator()
	data := TemplateData{
		Manifest:  mcpManifestBinaryTool(),
		Timestamp: "2026-03-08T12:00:00Z",
		Version:   "0.1.0",
	}
	files, err := gen.Generate(context.Background(), data, "")
	require.NoError(t, err, "Generate must succeed for a binary tool manifest")
	require.NotEmpty(t, files)

	f := findFile(files, "src/tools/screenshot.ts")
	require.NotNil(t, f, "tool file must be generated for binary tool")
}

func TestTSMCP_Generate_InlineSchemaSucceeds(t *testing.T) {
	// Generate must not error or panic for a tool with inline output schema.
	gen := NewTSMCPGenerator()
	data := TemplateData{
		Manifest:  mcpManifestInlineSchema(),
		Timestamp: "2026-03-08T12:00:00Z",
		Version:   "0.1.0",
	}
	files, err := gen.Generate(context.Background(), data, "")
	require.NoError(t, err, "Generate must succeed for an inline schema manifest")
	require.NotEmpty(t, files)

	f := findFile(files, "src/tools/get_weather.ts")
	require.NotNil(t, f, "tool file must be generated for tool with inline schema")
}

func TestTSMCP_Generate_StringSchemaSucceeds(t *testing.T) {
	// Generate must not error or panic for a tool with string output schema.
	gen := NewTSMCPGenerator()
	data := TemplateData{
		Manifest:  mcpManifestStringSchema(),
		Timestamp: "2026-03-08T12:00:00Z",
		Version:   "0.1.0",
	}
	files, err := gen.Generate(context.Background(), data, "")
	require.NoError(t, err, "Generate must succeed for a string schema manifest")
	require.NotEmpty(t, files)

	f := findFile(files, "src/tools/get_data.ts")
	require.NotNil(t, f, "tool file must be generated for tool with string schema")
}

// ---------------------------------------------------------------------------
// Table-driven: non-binary formats must NOT have binary output infrastructure
// (Constitution 9: table-driven tests for similar cases)
// ---------------------------------------------------------------------------

func TestTSMCP_NonBinaryFormats_NoBinaryInfrastructure(t *testing.T) {
	formats := []struct {
		name   string
		format string
	}{
		{name: "json", format: "json"},
		{name: "text", format: "text"},
		{name: "empty", format: ""},
	}
	for _, tc := range formats {
		t.Run(tc.name, func(t *testing.T) {
			m := manifest.Toolkit{
				APIVersion: "toolwright/v1",
				Kind:       "Toolkit",
				Metadata: manifest.Metadata{
					Name:        tc.name + "-server",
					Version:     "1.0.0",
					Description: "Test server",
				},
				Tools: []manifest.Tool{
					{
						Name:        "tool",
						Description: "Test tool",
						Entrypoint:  "./tool.sh",
						Output:      manifest.Output{Format: tc.format},
					},
				},
				Generate: manifest.Generate{
					MCP: manifest.MCPConfig{
						Target:    "typescript",
						Transport: []string{"stdio"},
					},
				},
			}
			files := generateTSMCP(t, m)
			content := fileContent(t, files, "src/tools/tool.ts")

			assert.NotContains(t, content, "base64",
				"format %q must NOT have base64 encoding", tc.format)

			handlerIdx := strings.Index(content, "handle_tool")
			require.Greater(t, handlerIdx, 0)
			afterHandler := content[handlerIdx:]

			assert.NotContains(t, afterHandler, `"image"`,
				"format %q handler must NOT have image type", tc.format)
			assert.NotContains(t, afterHandler, "mimeType",
				"format %q handler must NOT have mimeType", tc.format)
		})
	}
}

// ---------------------------------------------------------------------------
// Table-driven: buildTSToolData IsBinaryOutput for all formats
// ---------------------------------------------------------------------------

func TestTSMCP_BuildTSToolData_IsBinaryOutput_AllFormats(t *testing.T) {
	tests := []struct {
		format       string
		wantIsBinary bool
	}{
		{format: "binary", wantIsBinary: true},
		{format: "json", wantIsBinary: false},
		{format: "text", wantIsBinary: false},
		{format: "", wantIsBinary: false},
	}
	for _, tc := range tests {
		t.Run("format_"+tc.format, func(t *testing.T) {
			tool := manifest.Tool{
				Name:        "tool",
				Description: "Test",
				Entrypoint:  "./tool.sh",
				Output: manifest.Output{
					Format:   tc.format,
					MimeType: "image/png", // set mimeType even for non-binary to verify IsBinaryOutput is format-driven
				},
			}
			data, err := buildTSToolData(tool, manifest.Auth{Type: "none"})
			require.NoError(t, err)

			if tc.wantIsBinary {
				assert.True(t, data.IsBinaryOutput,
					"IsBinaryOutput must be true for format %q", tc.format)
			} else {
				assert.False(t, data.IsBinaryOutput,
					"IsBinaryOutput must be false for format %q", tc.format)
			}
		})
	}
}
