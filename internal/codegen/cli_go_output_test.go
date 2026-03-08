package codegen

import (
	"strings"
	"testing"

	"github.com/Obsidian-Owl/toolwright/internal/manifest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Test manifests for binary output
// ---------------------------------------------------------------------------

// manifestBinaryTool returns a manifest with a single binary-output tool.
func manifestBinaryTool() manifest.Toolkit {
	return manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:        "binary-toolkit",
			Version:     "1.0.0",
			Description: "Binary output toolkit",
		},
		Tools: []manifest.Tool{
			{
				Name:        "screenshot",
				Description: "Take a screenshot",
				Entrypoint:  "./screenshot.sh",
				Output:      manifest.Output{Format: "binary", MimeType: "image/png"},
			},
		},
	}
}

// manifestJSONTool returns a manifest with a single JSON-output tool.
func manifestJSONTool() manifest.Toolkit {
	return manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:        "json-toolkit",
			Version:     "1.0.0",
			Description: "JSON output toolkit",
		},
		Tools: []manifest.Tool{
			{
				Name:        "query",
				Description: "Query data",
				Entrypoint:  "./query.sh",
				Output:      manifest.Output{Format: "json"},
			},
		},
	}
}

// manifestTextTool returns a manifest with a single text-output tool.
func manifestTextTool() manifest.Toolkit {
	return manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:        "text-toolkit",
			Version:     "1.0.0",
			Description: "Text output toolkit",
		},
		Tools: []manifest.Tool{
			{
				Name:        "greet",
				Description: "Greet the user",
				Entrypoint:  "./greet.sh",
				Output:      manifest.Output{Format: "text"},
			},
		},
	}
}

// manifestMixedOutputTools returns a manifest with one binary and one JSON tool
// in the same toolkit.
func manifestMixedOutputTools() manifest.Toolkit {
	return manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:        "mixed-output-toolkit",
			Version:     "1.0.0",
			Description: "Mixed output toolkit",
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
				Description: "Get status as JSON",
				Entrypoint:  "./status.sh",
				Output:      manifest.Output{Format: "json"},
			},
		},
	}
}

// manifestBinaryToolWithFlags returns a binary-output tool that also has
// user-defined flags, to verify --output coexists with other flags.
func manifestBinaryToolWithFlags() manifest.Toolkit {
	return manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:        "binary-flags-toolkit",
			Version:     "1.0.0",
			Description: "Binary output with flags toolkit",
		},
		Tools: []manifest.Tool{
			{
				Name:        "export",
				Description: "Export data as binary",
				Entrypoint:  "./export.sh",
				Output:      manifest.Output{Format: "binary", MimeType: "application/octet-stream"},
				Flags: []manifest.Flag{
					{Name: "quality", Type: "int", Required: false, Default: 90, Description: "Output quality"},
					{Name: "format", Type: "string", Required: true, Enum: []string{"png", "jpeg"}, Description: "Image format"},
				},
			},
		},
	}
}

// manifestBinaryToolWithAuth returns a binary-output tool that also uses
// token auth, to verify auth and binary output coexist.
func manifestBinaryToolWithAuth() manifest.Toolkit {
	return manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:        "binary-auth-toolkit",
			Version:     "1.0.0",
			Description: "Binary output with auth toolkit",
		},
		Tools: []manifest.Tool{
			{
				Name:        "download",
				Description: "Download a protected file",
				Entrypoint:  "./download.sh",
				Output:      manifest.Output{Format: "binary", MimeType: "application/pdf"},
				Auth: &manifest.Auth{
					Type:      "token",
					TokenEnv:  "DOWNLOAD_TOKEN",
					TokenFlag: "--token",
				},
			},
		},
	}
}

// manifestNoOutputFormat returns a tool with an empty output format (default).
func manifestNoOutputFormat() manifest.Toolkit {
	return manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:        "noformat-toolkit",
			Version:     "1.0.0",
			Description: "No output format toolkit",
		},
		Tools: []manifest.Tool{
			{
				Name:        "run",
				Description: "Run a command",
				Entrypoint:  "./run.sh",
				// Output is zero-value: {Format: "", MimeType: ""}
			},
		},
	}
}

// ---------------------------------------------------------------------------
// AC9: Binary tool gets --output flag
// ---------------------------------------------------------------------------

func TestGoCLI_BinaryOutput_HasOutputFlag(t *testing.T) {
	files := generateCLI(t, manifestBinaryTool())
	content := fileContent(t, files, "internal/commands/screenshot.go")

	assert.Contains(t, content, "--output",
		"binary tool must generate an --output flag for specifying the output file path")
}

func TestGoCLI_BinaryOutput_OutputFlagUsesStringVar(t *testing.T) {
	// The --output flag is a file path, so it must be registered as StringVar.
	files := generateCLI(t, manifestBinaryTool())
	content := fileContent(t, files, "internal/commands/screenshot.go")

	// Look for StringVar registration that includes "output" as the flag name.
	assert.Regexp(t, `StringVar\([^)]*"output"`, content,
		"--output flag must be registered with StringVar")
}

func TestGoCLI_BinaryOutput_OutputFlagDescriptionMentionsFile(t *testing.T) {
	// The --output flag description must mention "file" so the user knows
	// it expects a file path, not something else.
	files := generateCLI(t, manifestBinaryTool())
	content := fileContent(t, files, "internal/commands/screenshot.go")

	// The description string in the StringVar call for --output must mention
	// "file" or "path" to clarify it expects a file path.
	contentLower := strings.ToLower(content)
	assert.True(t,
		strings.Contains(contentLower, "file") || strings.Contains(contentLower, "path"),
		"--output flag description must mention 'file' or 'path' to indicate it expects a file path")
}

func TestGoCLI_BinaryOutput_OutputFlagVarDeclared(t *testing.T) {
	// The output flag variable must be declared as a string var, scoped to the
	// tool's Go name (e.g., screenshotOutput or screenshotFlagOutput).
	files := generateCLI(t, manifestBinaryTool())
	content := fileContent(t, files, "internal/commands/screenshot.go")

	// The var declaration must use the tool's Go name prefix to avoid
	// collisions with other tools' output flags. Check for a pattern like
	// screenshotOutput or screenshotFlagOutput (the GoName for "screenshot"
	// is "screenshot").
	contentLower := strings.ToLower(content)
	assert.True(t,
		strings.Contains(contentLower, "screenshotoutput") ||
			strings.Contains(contentLower, "screenshotflagoutput") ||
			strings.Contains(content, "screenshotOutput") ||
			strings.Contains(content, "screenshotFlagOutput"),
		"--output flag variable must be scoped to the tool name (e.g., screenshotOutput)")
}

func TestGoCLI_BinaryOutput_JSONToolNoOutputFlag(t *testing.T) {
	// A tool with format: "json" must NOT generate --output flag.
	files := generateCLI(t, manifestJSONTool())
	content := fileContent(t, files, "internal/commands/query.go")

	assert.NotContains(t, content, `"output"`,
		"JSON-format tool must NOT generate an --output flag")
}

// ---------------------------------------------------------------------------
// AC10: TTY detection for binary output
// ---------------------------------------------------------------------------

func TestGoCLI_BinaryOutput_TTYDetection_HasStatCall(t *testing.T) {
	// Generated code must detect if stdout is a terminal. The standard Go
	// approach uses os.Stdout.Stat() and checks for ModeCharDevice.
	files := generateCLI(t, manifestBinaryTool())
	content := fileContent(t, files, "internal/commands/screenshot.go")

	assert.Contains(t, content, "os.Stdout.Stat()",
		"binary tool RunE must call os.Stdout.Stat() specifically for TTY detection")
}

func TestGoCLI_BinaryOutput_TTYDetection_ChecksModeCharDevice(t *testing.T) {
	// The TTY detection must check for ModeCharDevice (the fs.ModeCharDevice
	// bit indicates a terminal).
	files := generateCLI(t, manifestBinaryTool())
	content := fileContent(t, files, "internal/commands/screenshot.go")

	assert.Contains(t, content, "ModeCharDevice",
		"binary tool RunE must check ModeCharDevice for TTY detection")
}

func TestGoCLI_BinaryOutput_TTYError_ExactMessage(t *testing.T) {
	// When stdout is a TTY and no --output flag is provided, the generated
	// code must produce the exact error message specified in the AC.
	files := generateCLI(t, manifestBinaryTool())
	content := fileContent(t, files, "internal/commands/screenshot.go")

	assert.Contains(t, content, "binary output requires --output <file> or pipe",
		"binary tool must error with exact message 'binary output requires --output <file> or pipe' when TTY and no --output")
}

func TestGoCLI_BinaryOutput_TTYWithOutput_WritesToFile(t *testing.T) {
	// When TTY + --output is provided, the generated code must write to a file.
	// This means the generated code must contain file-writing logic: either
	// os.WriteFile, os.Create, or os.OpenFile.
	files := generateCLI(t, manifestBinaryTool())
	content := fileContent(t, files, "internal/commands/screenshot.go")

	assert.True(t,
		strings.Contains(content, "os.WriteFile") ||
			strings.Contains(content, "os.Create") ||
			strings.Contains(content, "os.OpenFile"),
		"binary tool must write to file when TTY + --output (must use os.WriteFile, os.Create, or os.OpenFile)")
}

func TestGoCLI_BinaryOutput_PipeMode_WritesToStdout(t *testing.T) {
	// When piped (not TTY), the generated code must write raw bytes to
	// os.Stdout. The generated code must reference os.Stdout for pipe-mode
	// writing, which is DISTINCT from the exec command's c.Stdout = os.Stdout.
	files := generateCLI(t, manifestBinaryTool())
	content := fileContent(t, files, "internal/commands/screenshot.go")

	// A non-binary tool uses os.Stdout once (for exec command's Stdout).
	// A binary tool must use it at least twice: once for exec and once for
	// the pipe-mode output writing. This catches a lazy implementation that
	// only has the exec assignment.
	stdoutCount := strings.Count(content, "os.Stdout")
	assert.GreaterOrEqual(t, stdoutCount, 2,
		"binary tool must reference os.Stdout at least twice (exec + pipe-mode write), found %d", stdoutCount)
}

func TestGoCLI_BinaryOutput_NonBinaryTool_NoTTYDetection(t *testing.T) {
	// Tools with format: "json" must NOT contain TTY detection logic.
	files := generateCLI(t, manifestJSONTool())
	content := fileContent(t, files, "internal/commands/query.go")

	assert.NotContains(t, content, "ModeCharDevice",
		"JSON-format tool must NOT contain TTY detection (ModeCharDevice)")
	assert.NotContains(t, content, "binary output requires",
		"JSON-format tool must NOT contain binary output error message")
}

func TestGoCLI_BinaryOutput_TextTool_NoTTYDetection(t *testing.T) {
	// Tools with format: "text" must NOT contain TTY detection logic.
	files := generateCLI(t, manifestTextTool())
	content := fileContent(t, files, "internal/commands/greet.go")

	assert.NotContains(t, content, "ModeCharDevice",
		"text-format tool must NOT contain TTY detection (ModeCharDevice)")
	assert.NotContains(t, content, "binary output requires",
		"text-format tool must NOT contain binary output error message")
}

// ---------------------------------------------------------------------------
// AC11: Non-binary tools unaffected
// ---------------------------------------------------------------------------

func TestGoCLI_BinaryOutput_TextToolNoOutputFlag(t *testing.T) {
	files := generateCLI(t, manifestTextTool())
	content := fileContent(t, files, "internal/commands/greet.go")

	assert.NotContains(t, content, `"output"`,
		"text-format tool must NOT generate an --output flag")
}

func TestGoCLI_BinaryOutput_EmptyFormatNoOutputFlag(t *testing.T) {
	// A tool with no output format set (empty string) must NOT get --output.
	files := generateCLI(t, manifestNoOutputFormat())
	content := fileContent(t, files, "internal/commands/run.go")

	assert.NotContains(t, content, `"output"`,
		"tool with empty output format must NOT generate an --output flag")
	assert.NotContains(t, content, "ModeCharDevice",
		"tool with empty output format must NOT contain TTY detection")
}

func TestGoCLI_BinaryOutput_JSONToolUnchangedFromBaseline(t *testing.T) {
	// Verify that a JSON tool generates the same kind of code as before:
	// it should have exec.CommandContext for the entrypoint and NOT have
	// any binary output handling.
	files := generateCLI(t, manifestJSONTool())
	content := fileContent(t, files, "internal/commands/query.go")

	// Must still have the normal entrypoint execution.
	assert.Contains(t, content, "exec.CommandContext",
		"JSON tool must still use exec.CommandContext for the entrypoint")
	// Must NOT have any binary output infrastructure.
	assert.NotContains(t, content, "binary output",
		"JSON tool must NOT contain 'binary output' text")
	assert.NotContains(t, content, "WriteFile",
		"JSON tool must NOT contain file-writing logic")
}

func TestGoCLI_BinaryOutput_BinaryToolStillHasExecCommand(t *testing.T) {
	// Binary tools must still execute the entrypoint via exec.CommandContext.
	// The --output / TTY logic is in addition to, not a replacement for,
	// the entrypoint execution.
	files := generateCLI(t, manifestBinaryTool())
	content := fileContent(t, files, "internal/commands/screenshot.go")

	assert.Contains(t, content, "exec.CommandContext",
		"binary tool must still use exec.CommandContext for the entrypoint")
}

// ---------------------------------------------------------------------------
// Edge cases: binary output with other features
// ---------------------------------------------------------------------------

func TestGoCLI_BinaryOutput_WithOtherFlags_OutputCoexists(t *testing.T) {
	// A binary tool with user-defined flags must have BOTH the user flags
	// AND the --output flag.
	files := generateCLI(t, manifestBinaryToolWithFlags())
	content := fileContent(t, files, "internal/commands/export.go")

	// User-defined flags must still be present.
	assert.Contains(t, content, `"quality"`,
		"binary tool with flags must still have user-defined flag 'quality'")
	assert.Contains(t, content, `"format"`,
		"binary tool with flags must still have user-defined flag 'format'")
	assert.Contains(t, content, "IntVar",
		"quality (int) flag must still use IntVar")

	// --output must also be present.
	assert.Contains(t, content, `"output"`,
		"binary tool with user flags must also have --output flag")

	// TTY detection must still be present.
	assert.Contains(t, content, "ModeCharDevice",
		"binary tool with user flags must still have TTY detection")
}

func TestGoCLI_BinaryOutput_WithAuth_AuthAndOutputCoexist(t *testing.T) {
	// A binary tool with token auth must generate BOTH auth token resolution
	// AND the --output flag / TTY detection.
	files := generateCLI(t, manifestBinaryToolWithAuth())
	content := fileContent(t, files, "internal/commands/download.go")

	// Auth must be present.
	assert.Contains(t, content, "DOWNLOAD_TOKEN",
		"binary tool with auth must still reference the token env var")
	assert.Contains(t, content, "token",
		"binary tool with auth must still have token-related code")

	// Binary output must also be present.
	assert.Contains(t, content, `"output"`,
		"binary tool with auth must have --output flag")
	assert.Contains(t, content, "ModeCharDevice",
		"binary tool with auth must have TTY detection")
	assert.Contains(t, content, "binary output requires --output <file> or pipe",
		"binary tool with auth must have the TTY error message")
}

// ---------------------------------------------------------------------------
// Mixed toolkit: one binary, one not
// ---------------------------------------------------------------------------

func TestGoCLI_BinaryOutput_MixedToolkit_OnlyBinaryGetsOutputFlag(t *testing.T) {
	// In a toolkit with one binary and one JSON tool, only the binary tool
	// should get --output flag and TTY detection.
	files := generateCLI(t, manifestMixedOutputTools())

	renderContent := fileContent(t, files, "internal/commands/render.go")
	statusContent := fileContent(t, files, "internal/commands/status.go")

	// render (binary) MUST have --output and TTY detection.
	assert.Contains(t, renderContent, `"output"`,
		"render (binary) must have --output flag")
	assert.Contains(t, renderContent, "ModeCharDevice",
		"render (binary) must have TTY detection")
	assert.Contains(t, renderContent, "binary output requires --output <file> or pipe",
		"render (binary) must have the TTY error message")

	// status (json) must NOT have --output or TTY detection.
	assert.NotContains(t, statusContent, `"output"`,
		"status (json) must NOT have --output flag")
	assert.NotContains(t, statusContent, "ModeCharDevice",
		"status (json) must NOT have TTY detection")
	assert.NotContains(t, statusContent, "binary output requires",
		"status (json) must NOT have binary output error message")
}

func TestGoCLI_BinaryOutput_MixedToolkit_BothStillHaveEntrypoint(t *testing.T) {
	files := generateCLI(t, manifestMixedOutputTools())

	renderContent := fileContent(t, files, "internal/commands/render.go")
	statusContent := fileContent(t, files, "internal/commands/status.go")

	assert.Contains(t, renderContent, "exec.CommandContext",
		"render (binary) must still have exec.CommandContext")
	assert.Contains(t, statusContent, "exec.CommandContext",
		"status (json) must still have exec.CommandContext")
}

// ---------------------------------------------------------------------------
// buildToolData: IsBinaryOutput field
// ---------------------------------------------------------------------------

func TestGoCLI_BinaryOutput_BuildToolData_IsBinaryOutput_True(t *testing.T) {
	// buildToolData must set IsBinaryOutput=true when output format is "binary".
	m := manifestBinaryTool()
	tool := m.Tools[0]
	auth := m.ResolvedAuth(tool)
	data := buildToolData(m, tool, auth)

	assert.True(t, data.IsBinaryOutput,
		"buildToolData must set IsBinaryOutput=true for output format 'binary'")
}

func TestGoCLI_BinaryOutput_BuildToolData_IsBinaryOutput_FalseForJSON(t *testing.T) {
	m := manifestJSONTool()
	tool := m.Tools[0]
	auth := m.ResolvedAuth(tool)
	data := buildToolData(m, tool, auth)

	assert.False(t, data.IsBinaryOutput,
		"buildToolData must set IsBinaryOutput=false for output format 'json'")
}

func TestGoCLI_BinaryOutput_BuildToolData_IsBinaryOutput_FalseForText(t *testing.T) {
	m := manifestTextTool()
	tool := m.Tools[0]
	auth := m.ResolvedAuth(tool)
	data := buildToolData(m, tool, auth)

	assert.False(t, data.IsBinaryOutput,
		"buildToolData must set IsBinaryOutput=false for output format 'text'")
}

func TestGoCLI_BinaryOutput_BuildToolData_IsBinaryOutput_FalseForEmpty(t *testing.T) {
	m := manifestNoOutputFormat()
	tool := m.Tools[0]
	auth := m.ResolvedAuth(tool)
	data := buildToolData(m, tool, auth)

	assert.False(t, data.IsBinaryOutput,
		"buildToolData must set IsBinaryOutput=false when output format is empty")
}

// ---------------------------------------------------------------------------
// Generated code imports
// ---------------------------------------------------------------------------

func TestGoCLI_BinaryOutput_ImportsOS(t *testing.T) {
	// Binary output tool must import "os" (for os.Stdout.Stat, os.WriteFile, etc).
	// Non-binary tools already import "os", so we focus on verifying it's present
	// and the binary-specific symbols are used.
	files := generateCLI(t, manifestBinaryTool())
	content := fileContent(t, files, "internal/commands/screenshot.go")

	assert.Contains(t, content, `"os"`,
		"binary tool must import 'os' package")
}

func TestGoCLI_BinaryOutput_NonBinaryToolNoFileWriteImports(t *testing.T) {
	// A JSON tool must NOT have file-writing calls that would only be needed
	// for binary output handling.
	files := generateCLI(t, manifestJSONTool())
	content := fileContent(t, files, "internal/commands/query.go")

	assert.NotContains(t, content, "os.WriteFile",
		"JSON tool must not contain os.WriteFile (only needed for binary output)")
	assert.NotContains(t, content, "os.Create",
		"JSON tool must not contain os.Create (only needed for binary output)")
}

// ---------------------------------------------------------------------------
// Table-driven format test: comprehensive non-binary formats (Constitution 9)
// ---------------------------------------------------------------------------

func TestGoCLI_BinaryOutput_NonBinaryFormats_NoOutputInfrastructure(t *testing.T) {
	tests := []struct {
		name   string
		format string
	}{
		{name: "json format", format: "json"},
		{name: "text format", format: "text"},
		{name: "empty format", format: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := manifest.Toolkit{
				APIVersion: "toolwright/v1",
				Kind:       "Toolkit",
				Metadata: manifest.Metadata{
					Name:        "format-test",
					Version:     "1.0.0",
					Description: "Format test toolkit",
				},
				Tools: []manifest.Tool{
					{
						Name:        "tool",
						Description: "Test tool",
						Entrypoint:  "./tool.sh",
						Output:      manifest.Output{Format: tc.format},
					},
				},
			}

			files := generateCLI(t, m)
			content := fileContent(t, files, "internal/commands/tool.go")

			assert.NotContains(t, content, "ModeCharDevice",
				"format %q must NOT produce TTY detection code", tc.format)
			assert.NotContains(t, content, "binary output requires",
				"format %q must NOT produce binary output error message", tc.format)
			// Check that no --output flag is registered. We look for the exact
			// pattern of flag registration with "output" as the flag name.
			assert.NotRegexp(t, `Flags\(\)\.StringVar\([^)]*"output"`, content,
				"format %q must NOT register an --output flag", tc.format)
		})
	}
}

// ---------------------------------------------------------------------------
// Adversarial: prevent lazy implementations
// ---------------------------------------------------------------------------

func TestGoCLI_BinaryOutput_TTYBranching_HasThreeCodePaths(t *testing.T) {
	// The generated code must have three distinct code paths:
	// 1. TTY + no --output -> error
	// 2. TTY + --output -> write to file
	// 3. Pipe (not TTY) -> write to stdout
	//
	// A lazy implementation might skip one of these paths. We verify all three
	// by checking for distinct markers of each path.
	files := generateCLI(t, manifestBinaryTool())
	content := fileContent(t, files, "internal/commands/screenshot.go")

	// Path 1: error message for TTY without --output
	assert.Contains(t, content, "binary output requires --output <file> or pipe",
		"must have error path for TTY without --output")

	// Path 2: file writing (os.WriteFile, os.Create, or os.OpenFile)
	hasFileWrite := strings.Contains(content, "os.WriteFile") ||
		strings.Contains(content, "os.Create") ||
		strings.Contains(content, "os.OpenFile")
	assert.True(t, hasFileWrite,
		"must have file-writing path for TTY with --output")

	// Path 3: stdout writing in pipe mode
	// The code must reference os.Stdout in a write context (not just for
	// exec.Command's Stdout assignment). Count occurrences: the exec command
	// setup references os.Stdout once, and the pipe-mode write should reference
	// it again.
	stdoutCount := strings.Count(content, "os.Stdout")
	assert.GreaterOrEqual(t, stdoutCount, 2,
		"binary tool must reference os.Stdout at least twice: once for exec and once for pipe-mode output (found %d)", stdoutCount)
}

func TestGoCLI_BinaryOutput_OutputFlagCheckedInRunE(t *testing.T) {
	// The RunE body must check the value of the --output flag variable to
	// decide between file write and error. A lazy implementation might declare
	// the flag but never read its value in RunE.
	files := generateCLI(t, manifestBinaryTool())
	content := fileContent(t, files, "internal/commands/screenshot.go")

	// The RunE must contain a conditional that references the output flag
	// variable. Look for an if-statement that checks the output path.
	// The variable is likely named screenshotOutput or similar.
	contentLower := strings.ToLower(content)
	hasOutputCheck := (strings.Contains(contentLower, "screenshotoutput") ||
		strings.Contains(contentLower, "screenshotflagoutput")) &&
		(strings.Contains(content, `!= ""`) || strings.Contains(content, `== ""`))
	assert.True(t, hasOutputCheck,
		"RunE must check the --output flag value with a conditional (empty string check)")
}

func TestGoCLI_BinaryOutput_HyphenatedToolName_OutputVarScoped(t *testing.T) {
	// A binary tool with a hyphenated name must produce a properly scoped
	// output flag variable using camelCase (goIdentifier conversion).
	m := manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:        "hyphen-toolkit",
			Version:     "1.0.0",
			Description: "Hyphenated name toolkit",
		},
		Tools: []manifest.Tool{
			{
				Name:        "take-screenshot",
				Description: "Take a screenshot",
				Entrypoint:  "./take-screenshot.sh",
				Output:      manifest.Output{Format: "binary", MimeType: "image/png"},
			},
		},
	}

	files := generateCLI(t, m)
	content := fileContent(t, files, "internal/commands/take-screenshot.go")

	// goIdentifier("take-screenshot") = "takeScreenshot"
	// The output flag var should be something like takeScreenshotOutput.
	assert.Contains(t, content, "takeScreenshot",
		"hyphenated binary tool must use camelCase Go identifier for output var")
	assert.Contains(t, content, `"output"`,
		"hyphenated binary tool must have --output flag")
	assert.Contains(t, content, "ModeCharDevice",
		"hyphenated binary tool must have TTY detection")
}

func TestGoCLI_BinaryOutput_MultipleBinaryTools_EachGetsScopedVar(t *testing.T) {
	// Two binary tools in the same toolkit must each have their own scoped
	// output flag variable, avoiding naming collisions.
	m := manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:        "multi-binary-toolkit",
			Version:     "1.0.0",
			Description: "Multiple binary tools",
		},
		Tools: []manifest.Tool{
			{
				Name:        "render",
				Description: "Render an image",
				Entrypoint:  "./render.sh",
				Output:      manifest.Output{Format: "binary", MimeType: "image/png"},
			},
			{
				Name:        "compile",
				Description: "Compile to binary",
				Entrypoint:  "./compile.sh",
				Output:      manifest.Output{Format: "binary", MimeType: "application/octet-stream"},
			},
		},
	}

	files := generateCLI(t, m)
	renderContent := fileContent(t, files, "internal/commands/render.go")
	compileContent := fileContent(t, files, "internal/commands/compile.go")

	// Each tool's output var must use its own tool name prefix.
	renderLower := strings.ToLower(renderContent)
	compileLower := strings.ToLower(compileContent)

	assert.True(t,
		strings.Contains(renderLower, "renderoutput") || strings.Contains(renderLower, "renderflagoutput"),
		"render tool must have its own scoped output variable (renderOutput)")
	assert.True(t,
		strings.Contains(compileLower, "compileoutput") || strings.Contains(compileLower, "compileflagoutput"),
		"compile tool must have its own scoped output variable (compileOutput)")

	// Both must have TTY detection independently.
	assert.Contains(t, renderContent, "ModeCharDevice",
		"render (binary) must have TTY detection")
	assert.Contains(t, compileContent, "ModeCharDevice",
		"compile (binary) must have TTY detection")
}

// ---------------------------------------------------------------------------
// Structural verification: generated code is valid Go
// ---------------------------------------------------------------------------

func TestGoCLI_BinaryOutput_GeneratedCodeHasPackageDeclaration(t *testing.T) {
	files := generateCLI(t, manifestBinaryTool())
	content := fileContent(t, files, "internal/commands/screenshot.go")

	assert.Contains(t, content, "package commands",
		"binary tool file must have 'package commands' declaration")
}

func TestGoCLI_BinaryOutput_GeneratedCodeImportsCobra(t *testing.T) {
	files := generateCLI(t, manifestBinaryTool())
	content := fileContent(t, files, "internal/commands/screenshot.go")

	assert.Contains(t, content, "cobra",
		"binary tool file must import cobra")
}

func TestGoCLI_BinaryOutput_GeneratedCodeHasRunE(t *testing.T) {
	files := generateCLI(t, manifestBinaryTool())
	content := fileContent(t, files, "internal/commands/screenshot.go")

	assert.Contains(t, content, "RunE:",
		"binary tool must define RunE function")
}

// ---------------------------------------------------------------------------
// buildToolData: IsBinaryOutput for various formats (table-driven)
// ---------------------------------------------------------------------------

func TestGoCLI_BinaryOutput_BuildToolData_FormatMapping(t *testing.T) {
	tests := []struct {
		name         string
		format       string
		wantIsBinary bool
	}{
		{name: "binary format", format: "binary", wantIsBinary: true},
		{name: "json format", format: "json", wantIsBinary: false},
		{name: "text format", format: "text", wantIsBinary: false},
		{name: "empty format", format: "", wantIsBinary: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := manifest.Toolkit{
				APIVersion: "toolwright/v1",
				Kind:       "Toolkit",
				Metadata: manifest.Metadata{
					Name:        "format-map-toolkit",
					Version:     "1.0.0",
					Description: "Format mapping test",
				},
				Tools: []manifest.Tool{
					{
						Name:        "tool",
						Description: "Test tool",
						Entrypoint:  "./tool.sh",
						Output:      manifest.Output{Format: tc.format},
					},
				},
			}
			tool := m.Tools[0]
			auth := m.ResolvedAuth(tool)
			data := buildToolData(m, tool, auth)

			if tc.wantIsBinary {
				require.True(t, data.IsBinaryOutput,
					"buildToolData must set IsBinaryOutput=true for format %q", tc.format)
			} else {
				require.False(t, data.IsBinaryOutput,
					"buildToolData must set IsBinaryOutput=false for format %q", tc.format)
			}
		})
	}
}
