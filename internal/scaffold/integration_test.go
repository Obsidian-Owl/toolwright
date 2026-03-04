package scaffold

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	toolwright "github.com/Obsidian-Owl/toolwright"
	"github.com/Obsidian-Owl/toolwright/internal/manifest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Integration tests: real embed.FS + scaffold package
//
// These tests use the actual toolwright.InitTemplates (embed.FS) rather than
// fstest.MapFS fixtures, verifying the full template-embedding-to-scaffold
// pipeline. They will not compile until embed.go exports InitTemplates.
// ---------------------------------------------------------------------------

// expectedFilesForRuntime returns the exact set of relative file paths that
// must be produced for each runtime. This is the canonical source of truth;
// any drift between scaffold code and these expectations is a test failure.
func expectedFilesForRuntime(runtime string) []string {
	shared := []string{
		"toolwright.yaml",
		"schemas/hello-output.json",
		"tests/hello.test.yaml",
		"README.md",
	}
	switch runtime {
	case "shell":
		return append(shared, "bin/hello")
	case "go":
		return append(shared, "bin/hello", "src/hello/main.go")
	case "python":
		return append(shared, "bin/hello")
	case "typescript":
		return append(shared, "bin/hello", "src/hello/index.ts", "package.json")
	default:
		return shared
	}
}

// ---------------------------------------------------------------------------
// AC-12: embed.go exports InitTemplates
// ---------------------------------------------------------------------------

func TestIntegration_InitTemplates_IsAccessible(t *testing.T) {
	// The embed.FS must be usable: we can open a known path inside it.
	f, err := toolwright.InitTemplates.Open("templates/init/toolwright.yaml.tmpl")
	require.NoError(t, err, "InitTemplates must contain templates/init/toolwright.yaml.tmpl")
	require.NoError(t, f.Close())
}

func TestIntegration_InitTemplates_ContainsAllTemplateFiles(t *testing.T) {
	// Verify every template file referenced by sharedEntries + runtimeEntries
	// actually exists inside the embedded FS.
	allPaths := []string{
		"templates/init/toolwright.yaml.tmpl",
		"templates/init/hello-output.schema.json",
		"templates/init/hello.test.yaml.tmpl",
		"templates/init/README.md.tmpl",
		"templates/init/shell/hello.sh.tmpl",
		"templates/init/go/hello.sh.tmpl",
		"templates/init/go/main.go.tmpl",
		"templates/init/python/hello.py.tmpl",
		"templates/init/typescript/hello.sh.tmpl",
		"templates/init/typescript/hello.ts.tmpl",
		"templates/init/typescript/package.json.tmpl",
	}

	for _, p := range allPaths {
		f, err := toolwright.InitTemplates.Open(p)
		require.NoError(t, err, "InitTemplates must contain %s", p)
		require.NoError(t, f.Close())
	}
}

// ---------------------------------------------------------------------------
// Integration: scaffold with real embed.FS for all 4 runtimes
// ---------------------------------------------------------------------------

func TestIntegration_Scaffold_AllRuntimes(t *testing.T) {
	type testCase struct {
		runtime       string
		extraFiles    []string // runtime-specific files beyond shared set
		entrypointSig string   // substring expected in bin/hello content
	}

	tests := []testCase{
		{
			runtime:       "shell",
			extraFiles:    []string{"bin/hello"},
			entrypointSig: `echo '{"message":"hello"}'`,
		},
		{
			runtime:       "go",
			extraFiles:    []string{"bin/hello", "src/hello/main.go"},
			entrypointSig: "exec go run ./src/hello/main.go",
		},
		{
			runtime:       "python",
			extraFiles:    []string{"bin/hello"},
			entrypointSig: "#!/usr/bin/env python3",
		},
		{
			runtime:       "typescript",
			extraFiles:    []string{"bin/hello", "src/hello/index.ts", "package.json"},
			entrypointSig: "exec npx ts-node src/hello/index.ts",
		},
	}

	for _, tc := range tests {
		t.Run(tc.runtime, func(t *testing.T) {
			tmpDir := t.TempDir()
			projectName := "integ-" + tc.runtime
			description := "Integration test for " + tc.runtime

			s := New(toolwright.InitTemplates)
			result, err := s.Scaffold(context.Background(), ScaffoldOptions{
				Name:        projectName,
				Description: description,
				OutputDir:   tmpDir,
				Runtime:     tc.runtime,
				Auth:        "none",
			})
			require.NoError(t, err, "Scaffold must succeed for runtime %s", tc.runtime)
			require.NotNil(t, result)

			projectDir := filepath.Join(tmpDir, projectName)

			// --- All expected files exist ---
			expected := expectedFilesForRuntime(tc.runtime)
			for _, relPath := range expected {
				fullPath := filepath.Join(projectDir, relPath)
				fInfo, statErr := os.Stat(fullPath)
				require.NoError(t, statErr, "file %s must exist", relPath)
				assert.False(t, fInfo.IsDir(), "%s must be a file, not a directory", relPath)
			}

			// --- result.Files matches expected set exactly ---
			assert.ElementsMatch(t, expected, result.Files,
				"result.Files must match the exact expected set for runtime %s", tc.runtime)

			// --- result.Dir is correct absolute path ---
			assert.Equal(t, projectDir, result.Dir,
				"result.Dir must be the absolute path to the project directory")

			// --- All files are non-empty ---
			for _, relPath := range expected {
				content, readErr := os.ReadFile(filepath.Join(projectDir, relPath))
				require.NoError(t, readErr)
				assert.NotEmpty(t, content,
					"file %s must have non-empty content", relPath)
			}

			// --- bin/hello has executable permissions ---
			binHello := filepath.Join(projectDir, "bin/hello")
			info, err := os.Stat(binHello)
			require.NoError(t, err)
			mode := info.Mode().Perm()
			assert.Equal(t, os.FileMode(0755), mode,
				"bin/hello must have 0755 permissions, got %04o", mode)

			// --- bin/hello starts with shebang ---
			entryContent, err := os.ReadFile(binHello)
			require.NoError(t, err)
			assert.True(t, strings.HasPrefix(string(entryContent), "#!"),
				"bin/hello must start with a shebang line")

			// --- bin/hello contains runtime-specific content ---
			assert.Contains(t, string(entryContent), tc.entrypointSig,
				"bin/hello must contain runtime-specific content %q", tc.entrypointSig)

			// --- Non-executable files do NOT have 0755 ---
			for _, relPath := range expected {
				if relPath == "bin/hello" {
					continue
				}
				fi, err := os.Stat(filepath.Join(projectDir, relPath))
				require.NoError(t, err)
				filePerm := fi.Mode().Perm()
				assert.Equal(t, os.FileMode(0644), filePerm,
					"non-executable file %s must have 0644 permissions, got %04o", relPath, filePerm)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Integration: toolwright.yaml passes Parse + Validate
// ---------------------------------------------------------------------------

func TestIntegration_Manifest_ParseAndValidate(t *testing.T) {
	runtimes := []string{"shell", "go", "python", "typescript"}

	for _, runtime := range runtimes {
		t.Run(runtime, func(t *testing.T) {
			tmpDir := t.TempDir()
			projectName := "valid-" + runtime

			s := New(toolwright.InitTemplates)
			_, err := s.Scaffold(context.Background(), ScaffoldOptions{
				Name:        projectName,
				Description: "Manifest validation test",
				OutputDir:   tmpDir,
				Runtime:     runtime,
				Auth:        "none",
			})
			require.NoError(t, err)

			manifestPath := filepath.Join(tmpDir, projectName, "toolwright.yaml")
			tk, err := manifest.ParseFile(manifestPath)
			require.NoError(t, err, "manifest.ParseFile must succeed for runtime %s", runtime)
			require.NotNil(t, tk)

			// Validate returns zero errors for a well-formed manifest.
			validationErrors := manifest.Validate(tk)
			assert.Empty(t, validationErrors,
				"manifest.Validate must return no errors, got: %v", validationErrors)

			// Verify specific manifest fields match scaffold input.
			assert.Equal(t, "toolwright/v1", tk.APIVersion)
			assert.Equal(t, "Toolkit", tk.Kind)
			assert.Equal(t, projectName, tk.Metadata.Name,
				"metadata.name must match the project name passed to scaffold")
			assert.Equal(t, "Manifest validation test", tk.Metadata.Description,
				"metadata.description must match the description passed to scaffold")
			assert.Equal(t, "0.1.0", tk.Metadata.Version)

			// Verify tool structure.
			require.Len(t, tk.Tools, 1, "must have exactly one tool")
			assert.Equal(t, "hello", tk.Tools[0].Name)
			assert.Equal(t, "bin/hello", tk.Tools[0].Entrypoint)
			assert.Equal(t, "json", tk.Tools[0].Output.Format)
			assert.Equal(t, "schemas/hello-output.json", tk.Tools[0].Output.Schema)

			// No auth for "none".
			assert.Nil(t, tk.Auth,
				"toolkit auth must be nil when auth=none")
		})
	}
}

// ---------------------------------------------------------------------------
// Integration: auth variants produce valid manifests
// ---------------------------------------------------------------------------

func TestIntegration_Manifest_TokenAuth(t *testing.T) {
	tmpDir := t.TempDir()
	projectName := "auth-token-proj"

	s := New(toolwright.InitTemplates)
	_, err := s.Scaffold(context.Background(), ScaffoldOptions{
		Name:        projectName,
		Description: "Token auth test",
		OutputDir:   tmpDir,
		Runtime:     "shell",
		Auth:        "token",
	})
	require.NoError(t, err)

	manifestPath := filepath.Join(tmpDir, projectName, "toolwright.yaml")
	tk, err := manifest.ParseFile(manifestPath)
	require.NoError(t, err)

	validationErrors := manifest.Validate(tk)
	assert.Empty(t, validationErrors,
		"token auth manifest must validate cleanly: %v", validationErrors)

	require.NotNil(t, tk.Auth, "toolkit auth must not be nil for token auth")
	assert.Equal(t, "token", tk.Auth.Type)
	assert.Equal(t, "AUTH-TOKEN-PROJ_TOKEN", tk.Auth.TokenEnv,
		"token_env must use uppercased project name")
	assert.Equal(t, "--token", tk.Auth.TokenFlag)
}

func TestIntegration_Manifest_OAuth2Auth(t *testing.T) {
	tmpDir := t.TempDir()
	projectName := "auth-oauth2-proj"

	s := New(toolwright.InitTemplates)
	_, err := s.Scaffold(context.Background(), ScaffoldOptions{
		Name:        projectName,
		Description: "OAuth2 auth test",
		OutputDir:   tmpDir,
		Runtime:     "shell",
		Auth:        "oauth2",
	})
	require.NoError(t, err)

	manifestPath := filepath.Join(tmpDir, projectName, "toolwright.yaml")
	tk, err := manifest.ParseFile(manifestPath)
	require.NoError(t, err)

	validationErrors := manifest.Validate(tk)
	assert.Empty(t, validationErrors,
		"oauth2 auth manifest must validate cleanly: %v", validationErrors)

	require.NotNil(t, tk.Auth, "toolkit auth must not be nil for oauth2 auth")
	assert.Equal(t, "oauth2", tk.Auth.Type)
	assert.True(t, strings.HasPrefix(tk.Auth.ProviderURL, "https://"),
		"oauth2 provider_url must use HTTPS")
	assert.NotEmpty(t, tk.Auth.Scopes,
		"oauth2 must have at least one scope")
}

// ---------------------------------------------------------------------------
// Integration: template variable rendering
// ---------------------------------------------------------------------------

func TestIntegration_TemplateRendering_ProjectNameInContent(t *testing.T) {
	tmpDir := t.TempDir()
	projectName := "rendering-check"
	description := "Unique description for rendering test"

	s := New(toolwright.InitTemplates)
	_, err := s.Scaffold(context.Background(), ScaffoldOptions{
		Name:        projectName,
		Description: description,
		OutputDir:   tmpDir,
		Runtime:     "shell",
		Auth:        "none",
	})
	require.NoError(t, err)

	projectDir := filepath.Join(tmpDir, projectName)

	// toolwright.yaml must contain the project name and description.
	manifestContent, err := os.ReadFile(filepath.Join(projectDir, "toolwright.yaml"))
	require.NoError(t, err)
	assert.Contains(t, string(manifestContent), "name: "+projectName,
		"manifest must contain rendered project name")
	assert.Contains(t, string(manifestContent), "description: "+description,
		"manifest must contain rendered description")

	// README.md must contain the project name as a heading.
	readmeContent, err := os.ReadFile(filepath.Join(projectDir, "README.md"))
	require.NoError(t, err)
	assert.Contains(t, string(readmeContent), "# "+projectName,
		"README must contain project name as heading")
	assert.Contains(t, string(readmeContent), description,
		"README must contain the project description")

	// Verify no unrendered template directives remain.
	assert.NotContains(t, string(manifestContent), "{{",
		"manifest must not contain unrendered template directives")
	assert.NotContains(t, string(manifestContent), "}}",
		"manifest must not contain unrendered template directives")
	assert.NotContains(t, string(readmeContent), "{{",
		"README must not contain unrendered template directives")
	assert.NotContains(t, string(readmeContent), "}}",
		"README must not contain unrendered template directives")
}

// ---------------------------------------------------------------------------
// Integration: static files copied verbatim
// ---------------------------------------------------------------------------

func TestIntegration_StaticFile_SchemaJSON_IsValidJSON(t *testing.T) {
	tmpDir := t.TempDir()
	projectName := "static-check"

	s := New(toolwright.InitTemplates)
	_, err := s.Scaffold(context.Background(), ScaffoldOptions{
		Name:        projectName,
		Description: "Static file test",
		OutputDir:   tmpDir,
		Runtime:     "shell",
		Auth:        "none",
	})
	require.NoError(t, err)

	schemaPath := filepath.Join(tmpDir, projectName, "schemas/hello-output.json")
	content, err := os.ReadFile(schemaPath)
	require.NoError(t, err)

	// Must be valid JSON.
	var parsed map[string]interface{}
	err = json.Unmarshal(content, &parsed)
	require.NoError(t, err, "hello-output.json must be valid JSON")

	// Verify it's a JSON Schema with expected structure.
	assert.Equal(t, "object", parsed["type"],
		"schema type must be 'object'")
	assert.Contains(t, parsed, "$schema",
		"schema must have $schema field")
	assert.Contains(t, parsed, "required",
		"schema must have required field")
	assert.Contains(t, parsed, "properties",
		"schema must have properties field")

	// The required field must include "message".
	reqSlice, ok := parsed["required"].([]interface{})
	require.True(t, ok, "required must be an array")
	assert.Contains(t, reqSlice, "message",
		"required must include 'message'")

	// Static file must NOT have template markers (no {{.Name}} etc.)
	assert.NotContains(t, string(content), "{{",
		"static file must not contain template directives")
}

// ---------------------------------------------------------------------------
// Integration: TypeScript-specific files
// ---------------------------------------------------------------------------

func TestIntegration_TypeScript_PackageJSON(t *testing.T) {
	tmpDir := t.TempDir()
	projectName := "ts-pkg-test"
	description := "TypeScript package.json test"

	s := New(toolwright.InitTemplates)
	_, err := s.Scaffold(context.Background(), ScaffoldOptions{
		Name:        projectName,
		Description: description,
		OutputDir:   tmpDir,
		Runtime:     "typescript",
		Auth:        "none",
	})
	require.NoError(t, err)

	pkgPath := filepath.Join(tmpDir, projectName, "package.json")
	content, err := os.ReadFile(pkgPath)
	require.NoError(t, err)

	// Must be valid JSON.
	var pkg map[string]interface{}
	err = json.Unmarshal(content, &pkg)
	require.NoError(t, err, "package.json must be valid JSON")

	// Name must be the rendered project name, not a template variable.
	assert.Equal(t, projectName, pkg["name"],
		"package.json name must be rendered project name")
	assert.Equal(t, description, pkg["description"],
		"package.json description must be rendered description")
	assert.Equal(t, "0.1.0", pkg["version"],
		"package.json version must be 0.1.0")

	// Must not contain unrendered template directives.
	assert.NotContains(t, string(content), "{{",
		"package.json must not contain template directives")
}

func TestIntegration_TypeScript_SourceFile(t *testing.T) {
	tmpDir := t.TempDir()
	projectName := "ts-src-test"

	s := New(toolwright.InitTemplates)
	_, err := s.Scaffold(context.Background(), ScaffoldOptions{
		Name:        projectName,
		Description: "TS source test",
		OutputDir:   tmpDir,
		Runtime:     "typescript",
		Auth:        "none",
	})
	require.NoError(t, err)

	srcPath := filepath.Join(tmpDir, projectName, "src/hello/index.ts")
	content, err := os.ReadFile(srcPath)
	require.NoError(t, err)

	assert.Contains(t, string(content), "hello",
		"TypeScript source must contain hello message")
	assert.Contains(t, string(content), "JSON.stringify",
		"TypeScript source must output JSON")
}

// ---------------------------------------------------------------------------
// Integration: Go-specific files
// ---------------------------------------------------------------------------

func TestIntegration_Go_MainFile(t *testing.T) {
	tmpDir := t.TempDir()
	projectName := "go-main-test"

	s := New(toolwright.InitTemplates)
	_, err := s.Scaffold(context.Background(), ScaffoldOptions{
		Name:        projectName,
		Description: "Go main test",
		OutputDir:   tmpDir,
		Runtime:     "go",
		Auth:        "none",
	})
	require.NoError(t, err)

	mainPath := filepath.Join(tmpDir, projectName, "src/hello/main.go")
	content, err := os.ReadFile(mainPath)
	require.NoError(t, err)

	assert.Contains(t, string(content), "package main",
		"Go main file must declare package main")
	assert.Contains(t, string(content), "func main()",
		"Go main file must have main function")
	assert.Contains(t, string(content), "encoding/json",
		"Go main file must import encoding/json")
}

// ---------------------------------------------------------------------------
// Integration: Python-specific entrypoint
// ---------------------------------------------------------------------------

func TestIntegration_Python_Entrypoint(t *testing.T) {
	tmpDir := t.TempDir()
	projectName := "py-entry-test"

	s := New(toolwright.InitTemplates)
	_, err := s.Scaffold(context.Background(), ScaffoldOptions{
		Name:        projectName,
		Description: "Python entrypoint test",
		OutputDir:   tmpDir,
		Runtime:     "python",
		Auth:        "none",
	})
	require.NoError(t, err)

	binPath := filepath.Join(tmpDir, projectName, "bin/hello")
	content, err := os.ReadFile(binPath)
	require.NoError(t, err)

	assert.Contains(t, string(content), "#!/usr/bin/env python3",
		"Python entrypoint must have python3 shebang")
	assert.Contains(t, string(content), "import json",
		"Python entrypoint must import json")
	assert.Contains(t, string(content), "hello",
		"Python entrypoint must produce hello message")
}

// ---------------------------------------------------------------------------
// Integration: test scenario file
// ---------------------------------------------------------------------------

func TestIntegration_TestScenarioFile(t *testing.T) {
	tmpDir := t.TempDir()
	projectName := "scenario-test"

	s := New(toolwright.InitTemplates)
	_, err := s.Scaffold(context.Background(), ScaffoldOptions{
		Name:        projectName,
		Description: "Test scenario check",
		OutputDir:   tmpDir,
		Runtime:     "shell",
		Auth:        "none",
	})
	require.NoError(t, err)

	testPath := filepath.Join(tmpDir, projectName, "tests/hello.test.yaml")
	content, err := os.ReadFile(testPath)
	require.NoError(t, err)

	contentStr := string(content)
	assert.Contains(t, contentStr, "tool: hello",
		"test scenario must reference the hello tool")
	assert.Contains(t, contentStr, "steps:",
		"test scenario must contain steps")
	assert.Contains(t, contentStr, "assert:",
		"test scenario must contain assertions")
	assert.Contains(t, contentStr, "message: hello",
		"test scenario must assert hello message output")
}

// ---------------------------------------------------------------------------
// Integration: embed.FS template files have non-zero content
//
// This catches a broken embed directive that embeds paths but produces
// empty files (e.g., wrong path prefix or missing all: prefix).
// ---------------------------------------------------------------------------

func TestIntegration_EmbeddedTemplateFiles_HaveContent(t *testing.T) {
	templatePaths := []string{
		"templates/init/toolwright.yaml.tmpl",
		"templates/init/hello-output.schema.json",
		"templates/init/hello.test.yaml.tmpl",
		"templates/init/README.md.tmpl",
		"templates/init/shell/hello.sh.tmpl",
		"templates/init/go/hello.sh.tmpl",
		"templates/init/go/main.go.tmpl",
		"templates/init/python/hello.py.tmpl",
		"templates/init/typescript/hello.sh.tmpl",
		"templates/init/typescript/hello.ts.tmpl",
		"templates/init/typescript/package.json.tmpl",
	}

	for _, p := range templatePaths {
		t.Run(p, func(t *testing.T) {
			f, err := toolwright.InitTemplates.Open(p)
			require.NoError(t, err, "must be able to open %s", p)
			defer f.Close()

			info, err := f.Stat()
			require.NoError(t, err)
			assert.Greater(t, info.Size(), int64(0),
				"embedded file %s must have non-zero size", p)
		})
	}
}
