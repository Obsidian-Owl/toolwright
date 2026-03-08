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
// Test manifests for annotations
// ---------------------------------------------------------------------------

// annotationsManifestBase returns a minimal MCP manifest with a single tool
// and no annotations. Used as a baseline for backward-compat tests.
func annotationsManifestBase() manifest.Toolkit {
	return manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:        "anno-server",
			Version:     "1.0.0",
			Description: "Annotations test server",
		},
		Tools: []manifest.Tool{
			{
				Name:        "read_file",
				Description: "Read a file from disk",
				Entrypoint:  "./read.sh",
				Auth:        &manifest.Auth{Type: "none"},
				Args: []manifest.Arg{
					{Name: "path", Type: "string", Required: true, Description: "File path"},
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

// annotationsManifestWithReadOnlyDestructive returns a manifest with a tool
// that has readOnly:true, destructive:false annotations.
func annotationsManifestWithReadOnlyDestructive() manifest.Toolkit {
	m := annotationsManifestBase()
	m.Tools[0].Annotations = &manifest.ToolAnnotations{
		ReadOnly:    manifest.BoolPtr(true),
		Destructive: manifest.BoolPtr(false),
	}
	return m
}

// annotationsManifestAllFourBoolFields returns a manifest with all four
// boolean annotation fields set.
func annotationsManifestAllFourBoolFields() manifest.Toolkit {
	m := annotationsManifestBase()
	m.Tools[0].Annotations = &manifest.ToolAnnotations{
		ReadOnly:    manifest.BoolPtr(true),
		Destructive: manifest.BoolPtr(false),
		Idempotent:  manifest.BoolPtr(true),
		OpenWorld:   manifest.BoolPtr(false),
	}
	return m
}

// annotationsManifestOnlyReadOnly returns a manifest where only readOnly is
// set (others nil).
func annotationsManifestOnlyReadOnly() manifest.Toolkit {
	m := annotationsManifestBase()
	m.Tools[0].Annotations = &manifest.ToolAnnotations{
		ReadOnly: manifest.BoolPtr(true),
	}
	return m
}

// annotationsManifestAllNilBools returns a manifest with a non-nil Annotations
// pointer but all *bool fields nil and title empty.
func annotationsManifestAllNilBools() manifest.Toolkit {
	m := annotationsManifestBase()
	m.Tools[0].Annotations = &manifest.ToolAnnotations{}
	return m
}

// annotationsManifestWithTitle returns a manifest whose tool has a title
// annotation set.
func annotationsManifestWithTitle() manifest.Toolkit {
	m := annotationsManifestBase()
	m.Tools[0].Annotations = &manifest.ToolAnnotations{
		Title: "Read File Tool",
	}
	return m
}

// annotationsManifestTitleAndBools returns a manifest with both title and
// boolean annotations.
func annotationsManifestTitleAndBools() manifest.Toolkit {
	m := annotationsManifestBase()
	m.Tools[0].Annotations = &manifest.ToolAnnotations{
		ReadOnly: manifest.BoolPtr(true),
		Title:    "Read File Tool",
	}
	return m
}

// ---------------------------------------------------------------------------
// AC5 — Annotations emitted in tool registration
// ---------------------------------------------------------------------------

func TestMCPAnnotations_ReadOnlyTrueDestructiveFalse(t *testing.T) {
	m := annotationsManifestWithReadOnlyDestructive()
	files := generateTSMCP(t, m)
	content := fileContent(t, files, "src/tools/read_file.ts")

	assert.Contains(t, content, "readOnlyHint: true",
		"readOnly:true must be emitted as readOnlyHint: true")
	assert.Contains(t, content, "destructiveHint: false",
		"destructive:false must be emitted as destructiveHint: false")
}

func TestMCPAnnotations_AllFourFieldsMapped(t *testing.T) {
	m := annotationsManifestAllFourBoolFields()
	files := generateTSMCP(t, m)
	content := fileContent(t, files, "src/tools/read_file.ts")

	assert.Contains(t, content, "readOnlyHint: true",
		"readOnly must map to readOnlyHint")
	assert.Contains(t, content, "destructiveHint: false",
		"destructive must map to destructiveHint")
	assert.Contains(t, content, "idempotentHint: true",
		"idempotent must map to idempotentHint")
	assert.Contains(t, content, "openWorldHint: false",
		"openWorld must map to openWorldHint")
}

func TestMCPAnnotations_AnnotationsInsideOptionsObject(t *testing.T) {
	// The MCP SDK expects server.tool(name, desc, schema, OPTIONS, handler).
	// Annotations must be in an options object, not as separate arguments.
	m := annotationsManifestWithReadOnlyDestructive()
	files := generateTSMCP(t, m)
	content := fileContent(t, files, "src/tools/read_file.ts")

	// Find the server.tool() call. With annotations, it should have 5 args,
	// and the 4th arg should be an object containing "annotations".
	require.Contains(t, content, "server.tool(",
		"generated TS must contain a server.tool() call")

	// The options object must contain the word "annotations" (the key that
	// nests readOnlyHint etc.).
	serverToolIdx := strings.Index(content, "server.tool(")
	require.Greater(t, serverToolIdx, 0)
	afterServerTool := content[serverToolIdx:]

	// Between "server.tool(" and the handler function, there should be an
	// object literal with "annotations:" key.
	assert.Contains(t, afterServerTool, "annotations:",
		"the options object passed to server.tool() must contain an annotations key")
}

func TestMCPAnnotations_FalseValuesEmitted(t *testing.T) {
	// *bool set to false must produce "Hint: false", not be omitted.
	m := annotationsManifestBase()
	m.Tools[0].Annotations = &manifest.ToolAnnotations{
		ReadOnly:    manifest.BoolPtr(false),
		Destructive: manifest.BoolPtr(false),
	}
	files := generateTSMCP(t, m)
	content := fileContent(t, files, "src/tools/read_file.ts")

	assert.Contains(t, content, "readOnlyHint: false",
		"readOnly:false must still emit readOnlyHint: false")
	assert.Contains(t, content, "destructiveHint: false",
		"destructive:false must still emit destructiveHint: false")
}

// ---------------------------------------------------------------------------
// AC6 — Nil annotations omitted
// ---------------------------------------------------------------------------

func TestMCPAnnotations_OnlySetFieldsEmitted(t *testing.T) {
	// Only readOnly is set; the other three bools are nil and must NOT appear.
	m := annotationsManifestOnlyReadOnly()
	files := generateTSMCP(t, m)
	content := fileContent(t, files, "src/tools/read_file.ts")

	assert.Contains(t, content, "readOnlyHint: true",
		"readOnly:true must be emitted")
	assert.NotContains(t, content, "destructiveHint",
		"nil destructive must not produce destructiveHint")
	assert.NotContains(t, content, "idempotentHint",
		"nil idempotent must not produce idempotentHint")
	assert.NotContains(t, content, "openWorldHint",
		"nil openWorld must not produce openWorldHint")
}

func TestMCPAnnotations_AllNilBools_NoAnnotationsBlock(t *testing.T) {
	// Annotations pointer is non-nil but all fields are zero values.
	// Should NOT emit an annotations block at all.
	m := annotationsManifestAllNilBools()
	files := generateTSMCP(t, m)
	content := fileContent(t, files, "src/tools/read_file.ts")

	// The server.tool() call should NOT contain an annotations key.
	serverToolIdx := strings.Index(content, "server.tool(")
	require.Greater(t, serverToolIdx, 0)
	afterServerTool := content[serverToolIdx:]

	assert.NotContains(t, afterServerTool, "annotations:",
		"when all annotation bools are nil and title is empty, no annotations block should appear")
}

func TestMCPAnnotations_NilAnnotationsPointer_NoAnnotationsBlock(t *testing.T) {
	// Annotations pointer is nil — no annotations at all.
	m := annotationsManifestBase()
	require.Nil(t, m.Tools[0].Annotations, "precondition: annotations pointer must be nil")
	files := generateTSMCP(t, m)
	content := fileContent(t, files, "src/tools/read_file.ts")

	assert.NotContains(t, content, "annotations:",
		"nil Annotations pointer must not produce annotations block")
	assert.NotContains(t, content, "readOnlyHint",
		"nil Annotations must not produce any hint fields")
	assert.NotContains(t, content, "destructiveHint",
		"nil Annotations must not produce any hint fields")
}

// ---------------------------------------------------------------------------
// AC7 — Title emitted in tool registration
// ---------------------------------------------------------------------------

func TestMCPAnnotations_TitleEmitted(t *testing.T) {
	m := annotationsManifestWithTitle()
	files := generateTSMCP(t, m)
	content := fileContent(t, files, "src/tools/read_file.ts")

	// The title must appear in the options object passed to server.tool().
	serverToolIdx := strings.Index(content, "server.tool(")
	require.Greater(t, serverToolIdx, 0)
	afterServerTool := content[serverToolIdx:]

	assert.Contains(t, afterServerTool, `title: "Read File Tool"`,
		"title annotation must be emitted as title in the options object")
}

func TestMCPAnnotations_EmptyTitleOmitted(t *testing.T) {
	// Title is "" → must NOT appear in generated code.
	m := annotationsManifestBase()
	m.Tools[0].Annotations = &manifest.ToolAnnotations{
		ReadOnly: manifest.BoolPtr(true),
		Title:    "",
	}
	files := generateTSMCP(t, m)
	content := fileContent(t, files, "src/tools/read_file.ts")

	serverToolIdx := strings.Index(content, "server.tool(")
	require.Greater(t, serverToolIdx, 0)
	afterServerTool := content[serverToolIdx:]

	assert.NotContains(t, afterServerTool, "title:",
		"empty title must not be emitted in the options object")
	// But readOnlyHint should still be present.
	assert.Contains(t, content, "readOnlyHint: true",
		"readOnly annotation must still be emitted when title is empty")
}

func TestMCPAnnotations_TitleAndAnnotationsTogether(t *testing.T) {
	m := annotationsManifestTitleAndBools()
	files := generateTSMCP(t, m)
	content := fileContent(t, files, "src/tools/read_file.ts")

	serverToolIdx := strings.Index(content, "server.tool(")
	require.Greater(t, serverToolIdx, 0)
	afterServerTool := content[serverToolIdx:]

	// Both title and annotations must be in the same options object.
	assert.Contains(t, afterServerTool, `title: "Read File Tool"`,
		"title must be present in the options object")
	assert.Contains(t, afterServerTool, "readOnlyHint: true",
		"readOnlyHint must be present alongside title")
	assert.Contains(t, afterServerTool, "annotations:",
		"annotations key must be present when bool fields are set")
}

func TestMCPAnnotations_TitleWithSpecialChars(t *testing.T) {
	// Title containing double quotes must be properly escaped.
	m := annotationsManifestBase()
	m.Tools[0].Annotations = &manifest.ToolAnnotations{
		Title: `Read "Special" File`,
	}
	files := generateTSMCP(t, m)
	content := fileContent(t, files, "src/tools/read_file.ts")

	serverToolIdx := strings.Index(content, "server.tool(")
	require.Greater(t, serverToolIdx, 0)
	afterServerTool := content[serverToolIdx:]

	// The title must be escaped for a valid TS string literal.
	assert.Contains(t, afterServerTool, `title: "Read \"Special\" File"`,
		"title with double quotes must be escaped in generated TS")
}

// ---------------------------------------------------------------------------
// AC8 — CLI codegen ignores annotations
// ---------------------------------------------------------------------------

func TestMCPAnnotations_CLICodegenIdenticalWithAndWithoutAnnotations(t *testing.T) {
	// Generate CLI code for a tool WITHOUT annotations.
	mNoAnno := annotationsManifestBase()
	// Set the CLI config so the CLI generator works.
	mNoAnno.Generate.CLI = manifest.CLIConfig{Target: "go"}
	filesNoAnno := generateCLI(t, mNoAnno)

	// Generate CLI code for the same tool WITH annotations.
	mWithAnno := annotationsManifestBase()
	mWithAnno.Generate.CLI = manifest.CLIConfig{Target: "go"}
	mWithAnno.Tools[0].Annotations = &manifest.ToolAnnotations{
		ReadOnly:    manifest.BoolPtr(true),
		Destructive: manifest.BoolPtr(false),
		Idempotent:  manifest.BoolPtr(true),
		OpenWorld:   manifest.BoolPtr(false),
		Title:       "Read File Tool",
	}
	filesWithAnno := generateCLI(t, mWithAnno)

	// The file sets should be the same size.
	require.Equal(t, len(filesNoAnno), len(filesWithAnno),
		"CLI generator must produce the same number of files with and without annotations")

	// Every file must have identical content.
	for _, fNoAnno := range filesNoAnno {
		fWithAnno := findFile(filesWithAnno, fNoAnno.Path)
		require.NotNilf(t, fWithAnno, "CLI file %q missing from annotated output", fNoAnno.Path)
		assert.Equal(t, string(fNoAnno.Content), string(fWithAnno.Content),
			"CLI file %q must be identical with and without annotations", fNoAnno.Path)
	}
}

// ---------------------------------------------------------------------------
// AC9 — Backward compatibility
// ---------------------------------------------------------------------------

func TestMCPAnnotations_NoAnnotations_OutputUnchanged(t *testing.T) {
	// A tool without annotations must generate with 4-arg server.tool() call:
	// server.tool(name, description, schema, handler)
	m := annotationsManifestBase()
	require.Nil(t, m.Tools[0].Annotations, "precondition: no annotations")
	files := generateTSMCP(t, m)
	content := fileContent(t, files, "src/tools/read_file.ts")

	// Must NOT contain any annotations-related keywords.
	assert.NotContains(t, content, "annotations:")
	assert.NotContains(t, content, "readOnlyHint")
	assert.NotContains(t, content, "destructiveHint")
	assert.NotContains(t, content, "idempotentHint")
	assert.NotContains(t, content, "openWorldHint")
	assert.NotContains(t, content, `title:`)
}

func TestMCPAnnotations_NoAnnotations_ServerToolArgCount(t *testing.T) {
	// Without annotations: server.tool(name, desc, schema, handler) — 4 args.
	// The handler is the last arg and is an async function.
	m := annotationsManifestBase()
	files := generateTSMCP(t, m)
	content := fileContent(t, files, "src/tools/read_file.ts")

	// Extract the server.tool() call block.
	serverToolIdx := strings.Index(content, "server.tool(")
	require.Greater(t, serverToolIdx, 0)

	// Find the matching closing ");". The server.tool() block should have
	// exactly: string, string, schema, handler.
	// Verify the handler (async function) comes right after the schema,
	// with no options object in between.
	afterServerTool := content[serverToolIdx:]

	// "inputSchema.shape," should be followed (after whitespace) by "async"
	// meaning there is no options object between schema and handler.
	schemaIdx := strings.Index(afterServerTool, "inputSchema.shape,")
	require.Greater(t, schemaIdx, 0, "inputSchema.shape must be present in server.tool() call")
	afterSchema := afterServerTool[schemaIdx+len("inputSchema.shape,"):]
	afterSchema = strings.TrimSpace(afterSchema)

	assert.True(t, strings.HasPrefix(afterSchema, "async "),
		"without annotations, the handler (async function) must immediately follow inputSchema.shape — got %q",
		afterSchema[:min(60, len(afterSchema))])
}

func TestMCPAnnotations_WithAnnotations_ServerToolArgCount(t *testing.T) {
	// With annotations: server.tool(name, desc, schema, OPTIONS, handler) — 5 args.
	// The options object must appear between schema and handler.
	m := annotationsManifestWithReadOnlyDestructive()
	files := generateTSMCP(t, m)
	content := fileContent(t, files, "src/tools/read_file.ts")

	serverToolIdx := strings.Index(content, "server.tool(")
	require.Greater(t, serverToolIdx, 0)
	afterServerTool := content[serverToolIdx:]

	// "inputSchema.shape," should be followed by an options object (starting
	// with "{") — NOT directly by "async".
	schemaIdx := strings.Index(afterServerTool, "inputSchema.shape,")
	require.Greater(t, schemaIdx, 0, "inputSchema.shape must be present")
	afterSchema := afterServerTool[schemaIdx+len("inputSchema.shape,"):]
	afterSchema = strings.TrimSpace(afterSchema)

	assert.True(t, strings.HasPrefix(afterSchema, "{"),
		"with annotations, an options object must follow inputSchema.shape — got %q",
		afterSchema[:min(60, len(afterSchema))])

	// After the options object, the handler should still be present.
	assert.Contains(t, afterServerTool, "async (input:",
		"handler function must still be present after the options object")
}

// ---------------------------------------------------------------------------
// Edge cases
// ---------------------------------------------------------------------------

func TestMCPAnnotations_MultipleTools_OnlyAnnotatedToolGetsOptions(t *testing.T) {
	// Two tools: one with annotations, one without.
	// Only the annotated tool should get the options object.
	m := manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:        "multi-tool-server",
			Version:     "1.0.0",
			Description: "Multi-tool test",
		},
		Tools: []manifest.Tool{
			{
				Name:        "read_file",
				Description: "Read a file",
				Entrypoint:  "./read.sh",
				Auth:        &manifest.Auth{Type: "none"},
				Args: []manifest.Arg{
					{Name: "path", Type: "string", Required: true, Description: "File path"},
				},
				Annotations: &manifest.ToolAnnotations{
					ReadOnly: manifest.BoolPtr(true),
				},
			},
			{
				Name:        "write_file",
				Description: "Write a file",
				Entrypoint:  "./write.sh",
				Auth:        &manifest.Auth{Type: "none"},
				Args: []manifest.Arg{
					{Name: "path", Type: "string", Required: true, Description: "File path"},
					{Name: "content", Type: "string", Required: true, Description: "Content"},
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

	// read_file has annotations → options object present.
	readContent := fileContent(t, files, "src/tools/read_file.ts")
	assert.Contains(t, readContent, "readOnlyHint: true",
		"annotated tool must have readOnlyHint")

	// write_file has NO annotations → no options object.
	writeContent := fileContent(t, files, "src/tools/write_file.ts")
	assert.NotContains(t, writeContent, "annotations:",
		"non-annotated tool must not have annotations block")
	assert.NotContains(t, writeContent, "readOnlyHint",
		"non-annotated tool must not have readOnlyHint")
}

func TestMCPAnnotations_TitleOnly_NoAnnotationsKey(t *testing.T) {
	// Title is set but no bool fields — options object should have title but
	// NOT an "annotations:" key (since there are no hint fields to nest).
	m := annotationsManifestWithTitle()
	files := generateTSMCP(t, m)
	content := fileContent(t, files, "src/tools/read_file.ts")

	serverToolIdx := strings.Index(content, "server.tool(")
	require.Greater(t, serverToolIdx, 0)
	afterServerTool := content[serverToolIdx:]

	assert.Contains(t, afterServerTool, `title: "Read File Tool"`,
		"title must be present in options object")
	assert.NotContains(t, afterServerTool, "readOnlyHint",
		"no bool hints should appear when none are set")
	assert.NotContains(t, afterServerTool, "destructiveHint",
		"no bool hints should appear when none are set")

	// The "annotations:" key (which nests readOnlyHint etc.) should NOT appear
	// when no bool fields are set. Title lives at the top level of options.
	assert.NotContains(t, afterServerTool, "annotations:",
		"annotations key must not appear when only title is set (title is top-level in options)")
}

func TestMCPAnnotations_GenerateDoesNotError(t *testing.T) {
	// Ensure Generate succeeds with annotations (no panics or template errors).
	m := annotationsManifestTitleAndBools()
	gen := NewTSMCPGenerator()
	data := TemplateData{
		Manifest:  m,
		Timestamp: "2026-03-04T12:00:00Z",
		Version:   "0.1.0",
	}
	files, err := gen.Generate(context.Background(), data, "")
	require.NoError(t, err, "Generate must succeed with annotated tools")
	require.NotEmpty(t, files)

	// Verify the tool file exists.
	f := findFile(files, "src/tools/read_file.ts")
	require.NotNil(t, f, "tool file must be generated for annotated tool")
}
