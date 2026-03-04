package scaffold

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/Obsidian-Owl/toolwright/internal/cli"
	"github.com/Obsidian-Owl/toolwright/internal/manifest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"
)

// ---------------------------------------------------------------------------
// Template content fixtures — inline templates for fstest.MapFS
//
// These represent what the real templates/init/ directory will contain.
// Tests use these so they verify rendering logic without depending on the
// filesystem or embed.FS. The templates must use text/template syntax with
// the templateData struct fields: .Name, .Description, .Runtime, .Auth
// ---------------------------------------------------------------------------

const manifestTemplate = `apiVersion: toolwright/v1
kind: Toolkit
metadata:
  name: {{.Name}}
  version: 0.1.0
  description: {{.Description}}
tools:
  - name: hello
    description: Hello world tool
    entrypoint: bin/hello
    output:
      format: json
      schema: schemas/hello-output.json
{{- if eq .Auth "token"}}
auth:
  type: token
  token_env: {{.Name | upper}}_TOKEN
  token_flag: --token
{{- else if eq .Auth "oauth2"}}
auth:
  type: oauth2
  provider_url: https://auth.example.com
  scopes:
    - read
{{- end}}
`

const shellEntrypointTemplate = `#!/bin/bash
set -euo pipefail
echo '{"message":"hello"}'
`

const goWrapperTemplate = `#!/bin/bash
set -euo pipefail
exec go run ./src/hello/main.go "$@"
`

const goMainTemplate = `package main

import (
	"encoding/json"
	"fmt"
	"os"
)

func main() {
	out := map[string]string{"message": "hello"}
	data, err := json.Marshal(out)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(data))
}
`

const pythonEntrypointTemplate = `#!/usr/bin/env python3
import json
print(json.dumps({"message": "hello"}))
`

const typescriptWrapperTemplate = `#!/bin/bash
set -euo pipefail
exec npx ts-node src/hello/index.ts "$@"
`

const typescriptSourceTemplate = `const output = { message: "hello" };
console.log(JSON.stringify(output));
`

const typescriptPackageJSONTemplate = `{
  "name": "{{.Name}}",
  "version": "0.1.0",
  "description": "{{.Description}}",
  "dependencies": {
    "typescript": "^5.0.0"
  },
  "devDependencies": {
    "ts-node": "^10.0.0",
    "@types/node": "^20.0.0"
  }
}
`

const readmeTemplate = `# {{.Name}}

{{.Description}}

## Getting Started

Run the hello tool:

` + "```" + `bash
./bin/hello
` + "```" + `
`

const testScenarioTemplate = `name: hello-test
tool: hello
steps:
  - name: hello outputs message
    assert:
      output:
        contains:
          message: hello
`

const helloOutputSchema = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "required": ["message"],
  "properties": {
    "message": {
      "type": "string"
    }
  }
}
`

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// buildTemplateFS creates an fstest.MapFS with all templates for a given set
// of runtimes. Pass nil to include all runtimes.
func buildTemplateFS() fstest.MapFS {
	return fstest.MapFS{
		"templates/init/toolwright.yaml.tmpl": &fstest.MapFile{
			Data: []byte(manifestTemplate),
		},
		"templates/init/hello-output.schema.json": &fstest.MapFile{
			Data: []byte(helloOutputSchema),
		},
		"templates/init/hello.test.yaml.tmpl": &fstest.MapFile{
			Data: []byte(testScenarioTemplate),
		},
		"templates/init/README.md.tmpl": &fstest.MapFile{
			Data: []byte(readmeTemplate),
		},
		"templates/init/shell/hello.sh.tmpl": &fstest.MapFile{
			Data: []byte(shellEntrypointTemplate),
		},
		"templates/init/go/hello.sh.tmpl": &fstest.MapFile{
			Data: []byte(goWrapperTemplate),
		},
		"templates/init/go/main.go.tmpl": &fstest.MapFile{
			Data: []byte(goMainTemplate),
		},
		"templates/init/python/hello.py.tmpl": &fstest.MapFile{
			Data: []byte(pythonEntrypointTemplate),
		},
		"templates/init/typescript/hello.sh.tmpl": &fstest.MapFile{
			Data: []byte(typescriptWrapperTemplate),
		},
		"templates/init/typescript/hello.ts.tmpl": &fstest.MapFile{
			Data: []byte(typescriptSourceTemplate),
		},
		"templates/init/typescript/package.json.tmpl": &fstest.MapFile{
			Data: []byte(typescriptPackageJSONTemplate),
		},
	}
}

// scaffoldInTempDir is a convenience helper that creates a Scaffolder, scaffolds
// into a temp directory, and returns the result plus the absolute path of the
// created project directory.
func scaffoldInTempDir(t *testing.T, opts cli.ScaffoldOptions) (*cli.ScaffoldResult, string) {
	t.Helper()
	tmpDir := t.TempDir()
	if opts.OutputDir == "" {
		opts.OutputDir = tmpDir
	}

	fs := buildTemplateFS()
	s := New(fs)
	result, err := s.Scaffold(context.Background(), opts)
	require.NoError(t, err, "Scaffold must not error for valid options: %+v", opts)
	require.NotNil(t, result, "Scaffold must return a non-nil result")

	projectDir := filepath.Join(opts.OutputDir, opts.Name)
	return result, projectDir
}

// readProjectFile reads a file relative to the project directory.
func readProjectFile(t *testing.T, projectDir, relPath string) string {
	t.Helper()
	content, err := os.ReadFile(filepath.Join(projectDir, relPath))
	require.NoError(t, err, "must be able to read %s", relPath)
	return string(content)
}

// ---------------------------------------------------------------------------
// AC-1: Scaffolder creates spec-compliant directory structure
// ---------------------------------------------------------------------------

func TestScaffold_Shell_CreatesExpectedFiles(t *testing.T) {
	result, projectDir := scaffoldInTempDir(t, cli.ScaffoldOptions{
		Name:        "my-toolkit",
		Description: "A test toolkit",
		Runtime:     "shell",
		Auth:        "none",
	})

	// Verify the project directory was created.
	info, err := os.Stat(projectDir)
	require.NoError(t, err, "project directory must exist")
	assert.True(t, info.IsDir(), "project path must be a directory")

	// Verify each required file exists.
	requiredFiles := []string{
		"toolwright.yaml",
		"bin/hello",
		"schemas/hello-output.json",
		"tests/hello.test.yaml",
		"README.md",
	}
	for _, f := range requiredFiles {
		fullPath := filepath.Join(projectDir, f)
		_, err := os.Stat(fullPath)
		assert.NoError(t, err, "required file %s must exist at %s", f, fullPath)
	}

	// Verify result.Files contains relative paths of all created files.
	assert.GreaterOrEqual(t, len(result.Files), len(requiredFiles),
		"result.Files must contain at least all required files")
	for _, f := range requiredFiles {
		assert.Contains(t, result.Files, f,
			"result.Files must contain relative path %q", f)
	}
}

func TestScaffold_ResultDir_MatchesExpectedPath(t *testing.T) {
	tmpDir := t.TempDir()
	fs := buildTemplateFS()
	s := New(fs)
	result, err := s.Scaffold(context.Background(), cli.ScaffoldOptions{
		Name:        "test-proj",
		Description: "desc",
		OutputDir:   tmpDir,
		Runtime:     "shell",
		Auth:        "none",
	})
	require.NoError(t, err)
	require.NotNil(t, result)

	expectedDir := filepath.Join(tmpDir, "test-proj")
	assert.Equal(t, expectedDir, result.Dir,
		"ScaffoldResult.Dir must be the full path to the created directory")
}

func TestScaffold_OutputDir_Empty_UsesCurrentDirectory(t *testing.T) {
	// When OutputDir is empty, the project should be created in the current directory.
	// We use a temp dir as the working directory to avoid polluting the repo.
	tmpDir := t.TempDir()

	// Change to tmpDir for this test, then restore.
	origDir, err := os.Getwd()
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = os.Chdir(origDir)
	})
	require.NoError(t, os.Chdir(tmpDir))

	fs := buildTemplateFS()
	s := New(fs)
	result, err := s.Scaffold(context.Background(), cli.ScaffoldOptions{
		Name:        "local-proj",
		Description: "local project",
		OutputDir:   "",
		Runtime:     "shell",
		Auth:        "none",
	})
	require.NoError(t, err)
	require.NotNil(t, result)

	// The project should exist under the current directory.
	expectedDir := filepath.Join(tmpDir, "local-proj")
	_, err = os.Stat(expectedDir)
	assert.NoError(t, err, "project must be created in current directory when OutputDir is empty")
}

func TestScaffold_ResultFiles_AreRelativePaths(t *testing.T) {
	result, _ := scaffoldInTempDir(t, cli.ScaffoldOptions{
		Name:        "relpath-test",
		Description: "test",
		Runtime:     "shell",
		Auth:        "none",
	})

	for _, f := range result.Files {
		assert.False(t, filepath.IsAbs(f),
			"result.Files must contain relative paths, got absolute: %s", f)
		assert.False(t, strings.HasPrefix(f, "relpath-test/"),
			"result.Files paths must not include the project name prefix, got: %s", f)
	}
}

func TestScaffold_ResultFiles_NoDuplicates(t *testing.T) {
	result, _ := scaffoldInTempDir(t, cli.ScaffoldOptions{
		Name:        "dup-test",
		Description: "test",
		Runtime:     "shell",
		Auth:        "none",
	})

	seen := make(map[string]bool)
	for _, f := range result.Files {
		assert.False(t, seen[f], "duplicate file in result.Files: %s", f)
		seen[f] = true
	}
}

// ---------------------------------------------------------------------------
// AC-1: Different names produce different directories (anti-hardcoding)
// ---------------------------------------------------------------------------

func TestScaffold_DifferentNames_DifferentDirectories(t *testing.T) {
	tmpDir := t.TempDir()
	fs := buildTemplateFS()
	s := New(fs)

	result1, err := s.Scaffold(context.Background(), cli.ScaffoldOptions{
		Name: "alpha", Description: "d", OutputDir: tmpDir, Runtime: "shell", Auth: "none",
	})
	require.NoError(t, err)

	result2, err := s.Scaffold(context.Background(), cli.ScaffoldOptions{
		Name: "beta", Description: "d", OutputDir: tmpDir, Runtime: "shell", Auth: "none",
	})
	require.NoError(t, err)

	assert.NotEqual(t, result1.Dir, result2.Dir,
		"different names must create different directories")
	assert.Contains(t, result1.Dir, "alpha")
	assert.Contains(t, result2.Dir, "beta")
}

// ---------------------------------------------------------------------------
// AC-2: Shell runtime produces working entrypoint
// ---------------------------------------------------------------------------

func TestScaffold_Shell_BinHello_IsExecutable(t *testing.T) {
	_, projectDir := scaffoldInTempDir(t, cli.ScaffoldOptions{
		Name: "exec-test", Description: "d", Runtime: "shell", Auth: "none",
	})

	info, err := os.Stat(filepath.Join(projectDir, "bin/hello"))
	require.NoError(t, err, "bin/hello must exist")
	mode := info.Mode()
	assert.True(t, mode&0111 != 0,
		"bin/hello must be executable, got mode %v", mode)
	assert.Equal(t, os.FileMode(0755), mode.Perm(),
		"bin/hello must have mode 0755, got %v", mode.Perm())
}

func TestScaffold_Shell_BinHello_HasBashShebang(t *testing.T) {
	_, projectDir := scaffoldInTempDir(t, cli.ScaffoldOptions{
		Name: "shebang-test", Description: "d", Runtime: "shell", Auth: "none",
	})

	content := readProjectFile(t, projectDir, "bin/hello")
	assert.True(t, strings.HasPrefix(content, "#!/bin/bash"),
		"shell entrypoint must start with #!/bin/bash shebang, got: %s",
		firstLine(content))
}

func TestScaffold_Shell_BinHello_OutputsValidJSON(t *testing.T) {
	_, projectDir := scaffoldInTempDir(t, cli.ScaffoldOptions{
		Name: "json-test", Description: "d", Runtime: "shell", Auth: "none",
	})

	content := readProjectFile(t, projectDir, "bin/hello")

	// The script must contain a JSON output with "message":"hello".
	// We check the content contains the expected JSON literal.
	assert.Contains(t, content, `"message"`,
		"shell entrypoint must output JSON containing 'message' key")
	assert.Contains(t, content, `"hello"`,
		"shell entrypoint must output JSON containing 'hello' value")

	// Verify the JSON string in the echo/printf is valid JSON by extracting it.
	// Look for a JSON object pattern in the script.
	jsonStart := strings.Index(content, "{")
	jsonEnd := strings.LastIndex(content, "}")
	if jsonStart >= 0 && jsonEnd > jsonStart {
		jsonStr := content[jsonStart : jsonEnd+1]
		// Remove shell quoting if single-quoted: '{"message":"hello"}'
		jsonStr = strings.Trim(jsonStr, "'\"")
		if json.Valid([]byte(jsonStr)) {
			var obj map[string]interface{}
			err := json.Unmarshal([]byte(jsonStr), &obj)
			if err == nil {
				assert.Equal(t, "hello", obj["message"],
					"shell entrypoint JSON must have message=hello")
			}
		}
	}
}

// ---------------------------------------------------------------------------
// AC-3: Go runtime produces working entrypoint
// ---------------------------------------------------------------------------

func TestScaffold_Go_CreatesExpectedFiles(t *testing.T) {
	result, projectDir := scaffoldInTempDir(t, cli.ScaffoldOptions{
		Name: "go-proj", Description: "d", Runtime: "go", Auth: "none",
	})

	// Go runtime needs: bin/hello (wrapper), src/hello/main.go, plus shared files
	goSpecificFiles := []string{
		"bin/hello",
		"src/hello/main.go",
	}
	for _, f := range goSpecificFiles {
		_, err := os.Stat(filepath.Join(projectDir, f))
		assert.NoError(t, err, "Go runtime must create %s", f)
	}

	for _, f := range goSpecificFiles {
		assert.Contains(t, result.Files, f,
			"result.Files must include %s for Go runtime", f)
	}
}

func TestScaffold_Go_BinHello_IsWrapper(t *testing.T) {
	_, projectDir := scaffoldInTempDir(t, cli.ScaffoldOptions{
		Name: "go-wrapper", Description: "d", Runtime: "go", Auth: "none",
	})

	content := readProjectFile(t, projectDir, "bin/hello")

	assert.True(t, strings.HasPrefix(content, "#!/bin/bash"),
		"Go wrapper must start with bash shebang")
	assert.Contains(t, content, "go run",
		"Go wrapper must call 'go run'")
	assert.Contains(t, content, "src/hello/main.go",
		"Go wrapper must reference src/hello/main.go")
}

func TestScaffold_Go_BinHello_IsExecutable(t *testing.T) {
	_, projectDir := scaffoldInTempDir(t, cli.ScaffoldOptions{
		Name: "go-exec", Description: "d", Runtime: "go", Auth: "none",
	})

	info, err := os.Stat(filepath.Join(projectDir, "bin/hello"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0755), info.Mode().Perm(),
		"Go bin/hello must have mode 0755")
}

func TestScaffold_Go_MainGo_HasPackageMain(t *testing.T) {
	_, projectDir := scaffoldInTempDir(t, cli.ScaffoldOptions{
		Name: "go-main", Description: "d", Runtime: "go", Auth: "none",
	})

	content := readProjectFile(t, projectDir, "src/hello/main.go")

	assert.Contains(t, content, "package main",
		"Go main.go must have package main declaration")
	assert.Contains(t, content, `"message"`,
		"Go main.go must output JSON with message field")
	assert.Contains(t, content, `"hello"`,
		"Go main.go must output JSON with hello value")
}

func TestScaffold_Go_MainGo_ImportsJSON(t *testing.T) {
	_, projectDir := scaffoldInTempDir(t, cli.ScaffoldOptions{
		Name: "go-import", Description: "d", Runtime: "go", Auth: "none",
	})

	content := readProjectFile(t, projectDir, "src/hello/main.go")
	assert.Contains(t, content, `"encoding/json"`,
		"Go main.go must import encoding/json to produce valid JSON output")
}

// ---------------------------------------------------------------------------
// AC-4: Python runtime produces working entrypoint
// ---------------------------------------------------------------------------

func TestScaffold_Python_BinHello_HasPython3Shebang(t *testing.T) {
	_, projectDir := scaffoldInTempDir(t, cli.ScaffoldOptions{
		Name: "py-proj", Description: "d", Runtime: "python", Auth: "none",
	})

	content := readProjectFile(t, projectDir, "bin/hello")
	assert.True(t, strings.HasPrefix(content, "#!/usr/bin/env python3"),
		"Python entrypoint must start with #!/usr/bin/env python3 shebang, got: %s",
		firstLine(content))
}

func TestScaffold_Python_BinHello_IsExecutable(t *testing.T) {
	_, projectDir := scaffoldInTempDir(t, cli.ScaffoldOptions{
		Name: "py-exec", Description: "d", Runtime: "python", Auth: "none",
	})

	info, err := os.Stat(filepath.Join(projectDir, "bin/hello"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0755), info.Mode().Perm(),
		"Python bin/hello must have mode 0755")
}

func TestScaffold_Python_BinHello_OutputsJSONWithMessage(t *testing.T) {
	_, projectDir := scaffoldInTempDir(t, cli.ScaffoldOptions{
		Name: "py-json", Description: "d", Runtime: "python", Auth: "none",
	})

	content := readProjectFile(t, projectDir, "bin/hello")
	assert.Contains(t, content, `"message"`,
		"Python entrypoint must output JSON with message key")
	assert.Contains(t, content, `"hello"`,
		"Python entrypoint must output JSON with hello value")
	assert.Contains(t, content, "json",
		"Python entrypoint must use json module")
}

// ---------------------------------------------------------------------------
// AC-5: TypeScript runtime produces working entrypoint
// ---------------------------------------------------------------------------

func TestScaffold_TypeScript_CreatesExpectedFiles(t *testing.T) {
	result, projectDir := scaffoldInTempDir(t, cli.ScaffoldOptions{
		Name: "ts-proj", Description: "d", Runtime: "typescript", Auth: "none",
	})

	tsSpecificFiles := []string{
		"bin/hello",
		"src/hello/index.ts",
		"package.json",
	}
	for _, f := range tsSpecificFiles {
		_, err := os.Stat(filepath.Join(projectDir, f))
		assert.NoError(t, err, "TypeScript runtime must create %s", f)
	}
	for _, f := range tsSpecificFiles {
		assert.Contains(t, result.Files, f,
			"result.Files must include %s for TypeScript runtime", f)
	}
}

func TestScaffold_TypeScript_BinHello_IsExecutable(t *testing.T) {
	_, projectDir := scaffoldInTempDir(t, cli.ScaffoldOptions{
		Name: "ts-exec", Description: "d", Runtime: "typescript", Auth: "none",
	})

	info, err := os.Stat(filepath.Join(projectDir, "bin/hello"))
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0755), info.Mode().Perm(),
		"TypeScript bin/hello must have mode 0755")
}

func TestScaffold_TypeScript_IndexTs_OutputsMessage(t *testing.T) {
	_, projectDir := scaffoldInTempDir(t, cli.ScaffoldOptions{
		Name: "ts-index", Description: "d", Runtime: "typescript", Auth: "none",
	})

	content := readProjectFile(t, projectDir, "src/hello/index.ts")
	assert.Contains(t, content, "message",
		"TypeScript index.ts must reference message field")
	assert.Contains(t, content, "hello",
		"TypeScript index.ts must contain hello value")
}

func TestScaffold_TypeScript_PackageJSON_IsValidJSON(t *testing.T) {
	_, projectDir := scaffoldInTempDir(t, cli.ScaffoldOptions{
		Name: "ts-pkg", Description: "A typescript toolkit", Runtime: "typescript", Auth: "none",
	})

	content := readProjectFile(t, projectDir, "package.json")
	var pkg map[string]interface{}
	err := json.Unmarshal([]byte(content), &pkg)
	require.NoError(t, err, "package.json must be valid JSON, got: %s", content)

	assert.Equal(t, "ts-pkg", pkg["name"],
		"package.json name must match project name")
}

func TestScaffold_TypeScript_PackageJSON_HasDependencies(t *testing.T) {
	_, projectDir := scaffoldInTempDir(t, cli.ScaffoldOptions{
		Name: "ts-deps", Description: "d", Runtime: "typescript", Auth: "none",
	})

	content := readProjectFile(t, projectDir, "package.json")
	var pkg map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(content), &pkg))

	// Must have some form of typescript dependency.
	raw, _ := json.Marshal(pkg)
	jsonStr := string(raw)
	assert.Contains(t, jsonStr, "typescript",
		"package.json must include typescript dependency")
}

// ---------------------------------------------------------------------------
// AC-6: Generated manifest is valid — table-driven across all runtimes
// ---------------------------------------------------------------------------

func TestScaffold_Manifest_ValidForAllRuntimes(t *testing.T) {
	runtimes := []string{"shell", "go", "python", "typescript"}

	for _, rt := range runtimes {
		t.Run("runtime="+rt, func(t *testing.T) {
			_, projectDir := scaffoldInTempDir(t, cli.ScaffoldOptions{
				Name:        "valid-" + rt,
				Description: "A " + rt + " toolkit",
				Runtime:     rt,
				Auth:        "none",
			})

			content := readProjectFile(t, projectDir, "toolwright.yaml")

			// Parse the manifest
			tk, err := manifest.Parse(strings.NewReader(content))
			require.NoError(t, err, "generated toolwright.yaml must be parseable for runtime %s", rt)
			require.NotNil(t, tk)

			// Validate the manifest
			validationErrors := manifest.Validate(tk)
			assert.Empty(t, validationErrors,
				"generated manifest must pass validation for runtime %s, errors: %v", rt, validationErrors)
		})
	}
}

func TestScaffold_Manifest_HasRequiredTopLevelFields(t *testing.T) {
	_, projectDir := scaffoldInTempDir(t, cli.ScaffoldOptions{
		Name: "fields-test", Description: "A test toolkit", Runtime: "shell", Auth: "none",
	})

	content := readProjectFile(t, projectDir, "toolwright.yaml")
	tk, err := manifest.Parse(strings.NewReader(content))
	require.NoError(t, err)

	assert.Equal(t, "toolwright/v1", tk.APIVersion,
		"manifest must have apiVersion: toolwright/v1")
	assert.Equal(t, "Toolkit", tk.Kind,
		"manifest must have kind: Toolkit")
	assert.Equal(t, "fields-test", tk.Metadata.Name,
		"manifest metadata.name must match project name")
	assert.Equal(t, "A test toolkit", tk.Metadata.Description,
		"manifest metadata.description must match provided description")
	assert.NotEmpty(t, tk.Metadata.Version,
		"manifest metadata.version must not be empty")
}

func TestScaffold_Manifest_ReferencesEntrypoint(t *testing.T) {
	_, projectDir := scaffoldInTempDir(t, cli.ScaffoldOptions{
		Name: "ep-test", Description: "d", Runtime: "shell", Auth: "none",
	})

	content := readProjectFile(t, projectDir, "toolwright.yaml")
	tk, err := manifest.Parse(strings.NewReader(content))
	require.NoError(t, err)

	require.NotEmpty(t, tk.Tools, "manifest must have at least one tool")
	assert.Equal(t, "bin/hello", tk.Tools[0].Entrypoint,
		"manifest tool entrypoint must be bin/hello")
}

func TestScaffold_Manifest_ToolNameIsHello(t *testing.T) {
	_, projectDir := scaffoldInTempDir(t, cli.ScaffoldOptions{
		Name: "toolname-test", Description: "d", Runtime: "shell", Auth: "none",
	})

	content := readProjectFile(t, projectDir, "toolwright.yaml")
	tk, err := manifest.Parse(strings.NewReader(content))
	require.NoError(t, err)

	require.NotEmpty(t, tk.Tools)
	assert.Equal(t, "hello", tk.Tools[0].Name,
		"generated tool must be named 'hello'")
}

func TestScaffold_Manifest_NameVariesWithProject(t *testing.T) {
	// Anti-hardcoding: two different project names should produce different manifest names.
	_, projDir1 := scaffoldInTempDir(t, cli.ScaffoldOptions{
		Name: "alpha-tool", Description: "d", Runtime: "shell", Auth: "none",
	})
	_, projDir2 := scaffoldInTempDir(t, cli.ScaffoldOptions{
		Name: "beta-tool", Description: "d", Runtime: "shell", Auth: "none",
	})

	content1 := readProjectFile(t, projDir1, "toolwright.yaml")
	content2 := readProjectFile(t, projDir2, "toolwright.yaml")

	tk1, _ := manifest.Parse(strings.NewReader(content1))
	tk2, _ := manifest.Parse(strings.NewReader(content2))

	assert.Equal(t, "alpha-tool", tk1.Metadata.Name)
	assert.Equal(t, "beta-tool", tk2.Metadata.Name)
	assert.NotEqual(t, tk1.Metadata.Name, tk2.Metadata.Name,
		"manifest names must differ for different projects (anti-hardcoding)")
}

// ---------------------------------------------------------------------------
// AC-6: Auth variants in generated manifest
// ---------------------------------------------------------------------------

func TestScaffold_Manifest_AuthNone_NoAuthBlock(t *testing.T) {
	_, projectDir := scaffoldInTempDir(t, cli.ScaffoldOptions{
		Name: "noauth", Description: "d", Runtime: "shell", Auth: "none",
	})

	content := readProjectFile(t, projectDir, "toolwright.yaml")
	tk, err := manifest.Parse(strings.NewReader(content))
	require.NoError(t, err)

	assert.Nil(t, tk.Auth,
		"auth=none must not produce an auth block in the manifest")
}

func TestScaffold_Manifest_AuthToken_HasTokenFields(t *testing.T) {
	_, projectDir := scaffoldInTempDir(t, cli.ScaffoldOptions{
		Name: "tokenauth", Description: "d", Runtime: "shell", Auth: "token",
	})

	content := readProjectFile(t, projectDir, "toolwright.yaml")
	tk, err := manifest.Parse(strings.NewReader(content))
	require.NoError(t, err)

	require.NotNil(t, tk.Auth,
		"auth=token must produce an auth block in the manifest")
	assert.Equal(t, "token", tk.Auth.Type,
		"auth block type must be 'token'")
	assert.NotEmpty(t, tk.Auth.TokenEnv,
		"token auth must have token_env field")
}

func TestScaffold_Manifest_AuthOAuth2_HasOAuthFields(t *testing.T) {
	_, projectDir := scaffoldInTempDir(t, cli.ScaffoldOptions{
		Name: "oauthtest", Description: "d", Runtime: "shell", Auth: "oauth2",
	})

	content := readProjectFile(t, projectDir, "toolwright.yaml")
	tk, err := manifest.Parse(strings.NewReader(content))
	require.NoError(t, err)

	require.NotNil(t, tk.Auth,
		"auth=oauth2 must produce an auth block in the manifest")
	assert.Equal(t, "oauth2", tk.Auth.Type,
		"auth block type must be 'oauth2'")
	assert.NotEmpty(t, tk.Auth.ProviderURL,
		"oauth2 auth must have provider_url field")
	assert.NotEmpty(t, tk.Auth.Scopes,
		"oauth2 auth must have scopes field")
}

func TestScaffold_Manifest_AuthVariants_StillPassValidation(t *testing.T) {
	// Ensure that token and oauth2 auth produce manifests that pass validation.
	// This is more stringent than just checking fields exist.
	auths := []struct {
		name string
		auth string
	}{
		{name: "token", auth: "token"},
		{name: "oauth2", auth: "oauth2"},
	}

	for _, tc := range auths {
		t.Run(tc.name, func(t *testing.T) {
			_, projectDir := scaffoldInTempDir(t, cli.ScaffoldOptions{
				Name:        "auth-valid-" + tc.name,
				Description: "d",
				Runtime:     "shell",
				Auth:        tc.auth,
			})

			content := readProjectFile(t, projectDir, "toolwright.yaml")
			tk, err := manifest.Parse(strings.NewReader(content))
			require.NoError(t, err)

			validationErrors := manifest.Validate(tk)
			assert.Empty(t, validationErrors,
				"manifest with auth=%s must pass validation, errors: %v",
				tc.auth, validationErrors)
		})
	}
}

// ---------------------------------------------------------------------------
// AC-7: Generated test scenario is valid
// ---------------------------------------------------------------------------

func TestScaffold_TestYAML_IsValidYAML(t *testing.T) {
	_, projectDir := scaffoldInTempDir(t, cli.ScaffoldOptions{
		Name: "testyaml", Description: "d", Runtime: "shell", Auth: "none",
	})

	content := readProjectFile(t, projectDir, "tests/hello.test.yaml")

	var parsed interface{}
	err := yaml.Unmarshal([]byte(content), &parsed)
	require.NoError(t, err, "tests/hello.test.yaml must be valid YAML, got: %s", content)
	assert.NotNil(t, parsed, "tests/hello.test.yaml must not be empty YAML")
}

func TestScaffold_TestYAML_NamesToolHello(t *testing.T) {
	_, projectDir := scaffoldInTempDir(t, cli.ScaffoldOptions{
		Name: "testname", Description: "d", Runtime: "shell", Auth: "none",
	})

	content := readProjectFile(t, projectDir, "tests/hello.test.yaml")

	// The test file must reference tool "hello" somewhere.
	assert.Contains(t, content, "hello",
		"test scenario must reference the hello tool")
}

func TestScaffold_TestYAML_AssertsMessageField(t *testing.T) {
	_, projectDir := scaffoldInTempDir(t, cli.ScaffoldOptions{
		Name: "testassert", Description: "d", Runtime: "shell", Auth: "none",
	})

	content := readProjectFile(t, projectDir, "tests/hello.test.yaml")

	assert.Contains(t, content, "message",
		"test scenario must assert on the 'message' field in output")
}

// ---------------------------------------------------------------------------
// AC-8: Schema file describes hello output
// ---------------------------------------------------------------------------

func TestScaffold_Schema_IsValidJSON(t *testing.T) {
	_, projectDir := scaffoldInTempDir(t, cli.ScaffoldOptions{
		Name: "schematest", Description: "d", Runtime: "shell", Auth: "none",
	})

	content := readProjectFile(t, projectDir, "schemas/hello-output.json")

	assert.True(t, json.Valid([]byte(content)),
		"schemas/hello-output.json must be valid JSON, got: %s", content)
}

func TestScaffold_Schema_IsJSONSchema(t *testing.T) {
	_, projectDir := scaffoldInTempDir(t, cli.ScaffoldOptions{
		Name: "jsonschema", Description: "d", Runtime: "shell", Auth: "none",
	})

	content := readProjectFile(t, projectDir, "schemas/hello-output.json")

	var schema map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(content), &schema))

	// Must be a JSON Schema document.
	assert.Contains(t, content, "$schema",
		"schema must include $schema keyword")
	assert.Equal(t, "object", schema["type"],
		"schema type must be 'object'")
}

func TestScaffold_Schema_RequiresMessageProperty(t *testing.T) {
	_, projectDir := scaffoldInTempDir(t, cli.ScaffoldOptions{
		Name: "schemamsg", Description: "d", Runtime: "shell", Auth: "none",
	})

	content := readProjectFile(t, projectDir, "schemas/hello-output.json")

	var schema map[string]interface{}
	require.NoError(t, json.Unmarshal([]byte(content), &schema))

	// Check "required" includes "message".
	required, ok := schema["required"].([]interface{})
	require.True(t, ok, "schema must have a 'required' array, got: %v", schema["required"])
	assert.Contains(t, required, "message",
		"schema required must include 'message'")

	// Check "properties" includes "message" with type "string".
	props, ok := schema["properties"].(map[string]interface{})
	require.True(t, ok, "schema must have a 'properties' object")
	msgProp, ok := props["message"].(map[string]interface{})
	require.True(t, ok, "schema properties must include 'message'")
	assert.Equal(t, "string", msgProp["type"],
		"schema message property must have type 'string'")
}

// ---------------------------------------------------------------------------
// AC-9: Existing directory is rejected
// ---------------------------------------------------------------------------

func TestScaffold_ExistingDirectory_ReturnsError(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "existing-proj")
	require.NoError(t, os.Mkdir(projectDir, 0755))

	fs := buildTemplateFS()
	s := New(fs)

	result, err := s.Scaffold(context.Background(), cli.ScaffoldOptions{
		Name:        "existing-proj",
		Description: "d",
		OutputDir:   tmpDir,
		Runtime:     "shell",
		Auth:        "none",
	})

	require.Error(t, err, "Scaffold must error when target directory already exists")
	assert.Nil(t, result, "result must be nil on error")
}

func TestScaffold_ExistingDirectory_ErrorContainsPath(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "conflict-proj")
	require.NoError(t, os.Mkdir(projectDir, 0755))

	fs := buildTemplateFS()
	s := New(fs)

	_, err := s.Scaffold(context.Background(), cli.ScaffoldOptions{
		Name:        "conflict-proj",
		Description: "d",
		OutputDir:   tmpDir,
		Runtime:     "shell",
		Auth:        "none",
	})

	require.Error(t, err)
	assert.Contains(t, err.Error(), "conflict-proj",
		"error message must include the directory path/name")
}

func TestScaffold_ExistingDirectory_NoPartialFiles(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "partial-test")
	require.NoError(t, os.Mkdir(projectDir, 0755))

	// Pre-populate with a known file to verify nothing is added.
	sentinel := filepath.Join(projectDir, "sentinel.txt")
	require.NoError(t, os.WriteFile(sentinel, []byte("original"), 0644))

	fs := buildTemplateFS()
	s := New(fs)

	_, _ = s.Scaffold(context.Background(), cli.ScaffoldOptions{
		Name:        "partial-test",
		Description: "d",
		OutputDir:   tmpDir,
		Runtime:     "shell",
		Auth:        "none",
	})

	// No new files should have been created.
	entries, err := os.ReadDir(projectDir)
	require.NoError(t, err)
	assert.Len(t, entries, 1,
		"existing directory must not have any new files after rejection, got: %v", entryNames(entries))
	assert.Equal(t, "sentinel.txt", entries[0].Name())
}

func TestScaffold_ExistingDirectory_EvenIfEmpty_StillRejected(t *testing.T) {
	tmpDir := t.TempDir()
	projectDir := filepath.Join(tmpDir, "empty-existing")
	require.NoError(t, os.Mkdir(projectDir, 0755))

	fs := buildTemplateFS()
	s := New(fs)

	_, err := s.Scaffold(context.Background(), cli.ScaffoldOptions{
		Name:        "empty-existing",
		Description: "d",
		OutputDir:   tmpDir,
		Runtime:     "shell",
		Auth:        "none",
	})

	require.Error(t, err,
		"even an empty existing directory must be rejected")
}

// ---------------------------------------------------------------------------
// AC-10: Template rendering failures are atomic
// ---------------------------------------------------------------------------

func TestScaffold_BadTemplate_NoFilesWritten(t *testing.T) {
	// Create a template FS with a malformed template that will fail to render.
	badFS := fstest.MapFS{
		"templates/init/toolwright.yaml.tmpl": &fstest.MapFile{
			Data: []byte(`{{.NonexistentField | badFunc}}`),
		},
		"templates/init/hello-output.schema.json": &fstest.MapFile{
			Data: []byte(helloOutputSchema),
		},
		"templates/init/hello.test.yaml.tmpl": &fstest.MapFile{
			Data: []byte(testScenarioTemplate),
		},
		"templates/init/README.md.tmpl": &fstest.MapFile{
			Data: []byte(readmeTemplate),
		},
		"templates/init/shell/hello.sh.tmpl": &fstest.MapFile{
			Data: []byte(shellEntrypointTemplate),
		},
	}

	tmpDir := t.TempDir()
	s := New(badFS)

	_, err := s.Scaffold(context.Background(), cli.ScaffoldOptions{
		Name:        "atomic-fail",
		Description: "d",
		OutputDir:   tmpDir,
		Runtime:     "shell",
		Auth:        "none",
	})

	require.Error(t, err, "Scaffold must error when template rendering fails")

	// The project directory must not exist.
	projectDir := filepath.Join(tmpDir, "atomic-fail")
	_, statErr := os.Stat(projectDir)
	assert.True(t, os.IsNotExist(statErr),
		"project directory must not exist when template rendering fails (atomicity)")
}

func TestScaffold_BadTemplate_ErrorIdentifiesTemplate(t *testing.T) {
	badFS := fstest.MapFS{
		"templates/init/toolwright.yaml.tmpl": &fstest.MapFile{
			Data: []byte(manifestTemplate),
		},
		"templates/init/hello-output.schema.json": &fstest.MapFile{
			Data: []byte(helloOutputSchema),
		},
		"templates/init/hello.test.yaml.tmpl": &fstest.MapFile{
			Data: []byte(`{{.Boom | undefinedFunc}}`),
		},
		"templates/init/README.md.tmpl": &fstest.MapFile{
			Data: []byte(readmeTemplate),
		},
		"templates/init/shell/hello.sh.tmpl": &fstest.MapFile{
			Data: []byte(shellEntrypointTemplate),
		},
	}

	tmpDir := t.TempDir()
	s := New(badFS)

	_, err := s.Scaffold(context.Background(), cli.ScaffoldOptions{
		Name:        "error-ident",
		Description: "d",
		OutputDir:   tmpDir,
		Runtime:     "shell",
		Auth:        "none",
	})

	require.Error(t, err)
	// The error should identify which template failed. We check for something
	// related to the test template that has the bad function.
	errMsg := err.Error()
	assert.True(t,
		strings.Contains(errMsg, "template") || strings.Contains(errMsg, "tmpl") ||
			strings.Contains(errMsg, "hello.test") || strings.Contains(errMsg, "render"),
		"error must identify which template failed, got: %s", errMsg)
}

// ---------------------------------------------------------------------------
// AC-11: Scaffolder accepts fs.FS for templates
// ---------------------------------------------------------------------------

func TestNew_AcceptsMapFS(t *testing.T) {
	// This test verifies that the constructor compiles and works with
	// fstest.MapFS, ensuring the signature accepts fs.FS, not embed.FS.
	fs := fstest.MapFS{
		"templates/init/toolwright.yaml.tmpl": &fstest.MapFile{
			Data: []byte(manifestTemplate),
		},
	}

	s := New(fs)
	require.NotNil(t, s, "New(fstest.MapFS) must return a non-nil Scaffolder")
}

func TestNew_DifferentFS_DifferentBehavior(t *testing.T) {
	// Two different FS implementations should produce different output,
	// proving the constructor actually uses the provided FS.
	fs1 := fstest.MapFS{
		"templates/init/toolwright.yaml.tmpl": &fstest.MapFile{
			Data: []byte(manifestTemplate),
		},
		"templates/init/hello-output.schema.json": &fstest.MapFile{
			Data: []byte(helloOutputSchema),
		},
		"templates/init/hello.test.yaml.tmpl": &fstest.MapFile{
			Data: []byte(testScenarioTemplate),
		},
		"templates/init/README.md.tmpl": &fstest.MapFile{
			Data: []byte(readmeTemplate),
		},
		"templates/init/shell/hello.sh.tmpl": &fstest.MapFile{
			Data: []byte(shellEntrypointTemplate),
		},
	}

	// fs2 has a different shell template that would produce different content.
	fs2 := fstest.MapFS{
		"templates/init/toolwright.yaml.tmpl": &fstest.MapFile{
			Data: []byte(manifestTemplate),
		},
		"templates/init/hello-output.schema.json": &fstest.MapFile{
			Data: []byte(helloOutputSchema),
		},
		"templates/init/hello.test.yaml.tmpl": &fstest.MapFile{
			Data: []byte(testScenarioTemplate),
		},
		"templates/init/README.md.tmpl": &fstest.MapFile{
			Data: []byte(readmeTemplate),
		},
		"templates/init/shell/hello.sh.tmpl": &fstest.MapFile{
			Data: []byte("#!/bin/bash\necho '{\"message\":\"different\"}'\n"),
		},
	}

	tmpDir1 := t.TempDir()
	s1 := New(fs1)
	_, err := s1.Scaffold(context.Background(), cli.ScaffoldOptions{
		Name: "fs1-proj", Description: "d", OutputDir: tmpDir1, Runtime: "shell", Auth: "none",
	})
	require.NoError(t, err)

	tmpDir2 := t.TempDir()
	s2 := New(fs2)
	_, err = s2.Scaffold(context.Background(), cli.ScaffoldOptions{
		Name: "fs2-proj", Description: "d", OutputDir: tmpDir2, Runtime: "shell", Auth: "none",
	})
	require.NoError(t, err)

	content1, err := os.ReadFile(filepath.Join(tmpDir1, "fs1-proj/bin/hello"))
	require.NoError(t, err)
	content2, err := os.ReadFile(filepath.Join(tmpDir2, "fs2-proj/bin/hello"))
	require.NoError(t, err)

	assert.NotEqual(t, string(content1), string(content2),
		"different template FS must produce different output (anti-hardcoding)")
}

// ---------------------------------------------------------------------------
// Cross-runtime table-driven: all runtimes create shared files
// ---------------------------------------------------------------------------

func TestScaffold_AllRuntimes_CreateSharedFiles(t *testing.T) {
	runtimes := []string{"shell", "go", "python", "typescript"}
	sharedFiles := []string{
		"toolwright.yaml",
		"bin/hello",
		"schemas/hello-output.json",
		"tests/hello.test.yaml",
		"README.md",
	}

	for _, rt := range runtimes {
		t.Run("runtime="+rt, func(t *testing.T) {
			result, projectDir := scaffoldInTempDir(t, cli.ScaffoldOptions{
				Name:        "shared-" + rt,
				Description: "d",
				Runtime:     rt,
				Auth:        "none",
			})

			for _, f := range sharedFiles {
				_, err := os.Stat(filepath.Join(projectDir, f))
				assert.NoError(t, err,
					"runtime=%s must create shared file %s", rt, f)
				assert.Contains(t, result.Files, f,
					"runtime=%s result.Files must include %s", rt, f)
			}
		})
	}
}

func TestScaffold_AllRuntimes_BinHelloIsExecutable(t *testing.T) {
	runtimes := []string{"shell", "go", "python", "typescript"}

	for _, rt := range runtimes {
		t.Run("runtime="+rt, func(t *testing.T) {
			_, projectDir := scaffoldInTempDir(t, cli.ScaffoldOptions{
				Name:        "perm-" + rt,
				Description: "d",
				Runtime:     rt,
				Auth:        "none",
			})

			info, err := os.Stat(filepath.Join(projectDir, "bin/hello"))
			require.NoError(t, err, "bin/hello must exist for runtime %s", rt)
			assert.Equal(t, os.FileMode(0755), info.Mode().Perm(),
				"bin/hello must have 0755 permissions for runtime %s, got %v", rt, info.Mode().Perm())
		})
	}
}

// ---------------------------------------------------------------------------
// Cross-runtime table-driven: each runtime has specific extra files
// ---------------------------------------------------------------------------

func TestScaffold_RuntimeSpecificFiles(t *testing.T) {
	tests := []struct {
		runtime       string
		expectedFiles []string
		absentFiles   []string
	}{
		{
			runtime:       "shell",
			expectedFiles: []string{"bin/hello"},
			absentFiles:   []string{"src/hello/main.go", "src/hello/index.ts", "package.json"},
		},
		{
			runtime:       "go",
			expectedFiles: []string{"bin/hello", "src/hello/main.go"},
			absentFiles:   []string{"src/hello/index.ts", "package.json"},
		},
		{
			runtime:       "python",
			expectedFiles: []string{"bin/hello"},
			absentFiles:   []string{"src/hello/main.go", "src/hello/index.ts", "package.json"},
		},
		{
			runtime:       "typescript",
			expectedFiles: []string{"bin/hello", "src/hello/index.ts", "package.json"},
			absentFiles:   []string{"src/hello/main.go"},
		},
	}

	for _, tc := range tests {
		t.Run("runtime="+tc.runtime, func(t *testing.T) {
			_, projectDir := scaffoldInTempDir(t, cli.ScaffoldOptions{
				Name:        "specific-" + tc.runtime,
				Description: "d",
				Runtime:     tc.runtime,
				Auth:        "none",
			})

			for _, f := range tc.expectedFiles {
				_, err := os.Stat(filepath.Join(projectDir, f))
				assert.NoError(t, err,
					"runtime=%s must create %s", tc.runtime, f)
			}
			for _, f := range tc.absentFiles {
				_, err := os.Stat(filepath.Join(projectDir, f))
				assert.True(t, os.IsNotExist(err),
					"runtime=%s must NOT create %s (belongs to other runtime)", tc.runtime, f)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Description is rendered into files
// ---------------------------------------------------------------------------

func TestScaffold_Description_AppearsInManifest(t *testing.T) {
	_, projectDir := scaffoldInTempDir(t, cli.ScaffoldOptions{
		Name:        "desc-proj",
		Description: "My unique description 42",
		Runtime:     "shell",
		Auth:        "none",
	})

	content := readProjectFile(t, projectDir, "toolwright.yaml")
	assert.Contains(t, content, "My unique description 42",
		"manifest must contain the provided description")
}

func TestScaffold_Description_AppearsInReadme(t *testing.T) {
	_, projectDir := scaffoldInTempDir(t, cli.ScaffoldOptions{
		Name:        "readme-desc",
		Description: "Special readme description",
		Runtime:     "shell",
		Auth:        "none",
	})

	content := readProjectFile(t, projectDir, "README.md")
	assert.Contains(t, content, "Special readme description",
		"README must contain the provided description")
}

func TestScaffold_Name_AppearsInReadme(t *testing.T) {
	_, projectDir := scaffoldInTempDir(t, cli.ScaffoldOptions{
		Name:        "fancy-toolkit",
		Description: "d",
		Runtime:     "shell",
		Auth:        "none",
	})

	content := readProjectFile(t, projectDir, "README.md")
	assert.Contains(t, content, "fancy-toolkit",
		"README must contain the project name")
}

// ---------------------------------------------------------------------------
// File content is non-empty
// ---------------------------------------------------------------------------

func TestScaffold_AllFiles_NonEmpty(t *testing.T) {
	result, projectDir := scaffoldInTempDir(t, cli.ScaffoldOptions{
		Name:        "nonempty",
		Description: "d",
		Runtime:     "shell",
		Auth:        "none",
	})

	for _, f := range result.Files {
		content, err := os.ReadFile(filepath.Join(projectDir, f))
		require.NoError(t, err, "must be able to read %s", f)
		assert.NotEmpty(t, content,
			"file %s must not be empty", f)
	}
}

// ---------------------------------------------------------------------------
// Result.Files sorted for determinism
// ---------------------------------------------------------------------------

func TestScaffold_ResultFiles_AreSorted(t *testing.T) {
	result, _ := scaffoldInTempDir(t, cli.ScaffoldOptions{
		Name:        "sorted-test",
		Description: "d",
		Runtime:     "shell",
		Auth:        "none",
	})

	sorted := make([]string, len(result.Files))
	copy(sorted, result.Files)
	sort.Strings(sorted)

	assert.Equal(t, sorted, result.Files,
		"result.Files should be sorted for deterministic output")
}

// ---------------------------------------------------------------------------
// Context cancellation
// ---------------------------------------------------------------------------

func TestScaffold_CancelledContext_ReturnsError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	tmpDir := t.TempDir()
	fs := buildTemplateFS()
	s := New(fs)

	_, err := s.Scaffold(ctx, cli.ScaffoldOptions{
		Name:        "cancelled",
		Description: "d",
		OutputDir:   tmpDir,
		Runtime:     "shell",
		Auth:        "none",
	})

	require.Error(t, err, "Scaffold must error when context is cancelled")
}

// ---------------------------------------------------------------------------
// Edge case: project name with special characters
// ---------------------------------------------------------------------------

func TestScaffold_NameWithHyphens_Works(t *testing.T) {
	result, _ := scaffoldInTempDir(t, cli.ScaffoldOptions{
		Name:        "my-cool-tool",
		Description: "d",
		Runtime:     "shell",
		Auth:        "none",
	})

	assert.Contains(t, result.Dir, "my-cool-tool",
		"hyphenated names must work correctly")
}

func TestScaffold_NameWithNumbers_Works(t *testing.T) {
	result, _ := scaffoldInTempDir(t, cli.ScaffoldOptions{
		Name:        "tool42",
		Description: "d",
		Runtime:     "shell",
		Auth:        "none",
	})

	assert.Contains(t, result.Dir, "tool42")
}

// ---------------------------------------------------------------------------
// Edge case: schema file is static (not templated)
// ---------------------------------------------------------------------------

func TestScaffold_Schema_IdenticalAcrossRuntimes(t *testing.T) {
	// The JSON schema is a static file, not a template. It should be identical
	// regardless of runtime choice.
	runtimes := []string{"shell", "go", "python", "typescript"}
	var contents []string

	for _, rt := range runtimes {
		_, projectDir := scaffoldInTempDir(t, cli.ScaffoldOptions{
			Name:        "schema-" + rt,
			Description: "d",
			Runtime:     rt,
			Auth:        "none",
		})
		content := readProjectFile(t, projectDir, "schemas/hello-output.json")
		contents = append(contents, content)
	}

	for i := 1; i < len(contents); i++ {
		assert.Equal(t, contents[0], contents[i],
			"schema must be identical across runtimes (%s vs %s)",
			runtimes[0], runtimes[i])
	}
}

// ---------------------------------------------------------------------------
// Edge case: schema file is identical to what we provide in the FS
// ---------------------------------------------------------------------------

func TestScaffold_Schema_MatchesStaticContent(t *testing.T) {
	_, projectDir := scaffoldInTempDir(t, cli.ScaffoldOptions{
		Name: "schema-static", Description: "d", Runtime: "shell", Auth: "none",
	})

	content := readProjectFile(t, projectDir, "schemas/hello-output.json")
	assert.Equal(t, helloOutputSchema, content,
		"schema file must be copied verbatim from template FS")
}

// ---------------------------------------------------------------------------
// Edge case: non-template files are not template-processed
// ---------------------------------------------------------------------------

func TestScaffold_StaticFiles_NotTemplateProcessed(t *testing.T) {
	// Create a FS where the static schema has Go template syntax in it.
	// The scaffolder must copy it verbatim, not try to render it.
	trickFS := buildTemplateFS()
	trickFS["templates/init/hello-output.schema.json"] = &fstest.MapFile{
		Data: []byte(`{"$schema": "https://json-schema.org/draft/2020-12/schema", "type": "object", "description": "{{.Name}} should NOT be rendered", "required": ["message"], "properties": {"message": {"type": "string"}}}`),
	}

	tmpDir := t.TempDir()
	s := New(trickFS)
	_, err := s.Scaffold(context.Background(), cli.ScaffoldOptions{
		Name:        "static-trick",
		Description: "d",
		OutputDir:   tmpDir,
		Runtime:     "shell",
		Auth:        "none",
	})
	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(tmpDir, "static-trick/schemas/hello-output.json"))
	require.NoError(t, err)

	// The {{.Name}} must remain as-is, not be replaced.
	assert.Contains(t, string(content), "{{.Name}}",
		"static files (no .tmpl extension) must not be template-processed")
}

// ---------------------------------------------------------------------------
// Edge case: OutputDir does not exist yet
// ---------------------------------------------------------------------------

func TestScaffold_NonexistentOutputDir_ReturnsError(t *testing.T) {
	fs := buildTemplateFS()
	s := New(fs)

	_, err := s.Scaffold(context.Background(), cli.ScaffoldOptions{
		Name:        "orphan-proj",
		Description: "d",
		OutputDir:   "/nonexistent/path/that/does/not/exist",
		Runtime:     "shell",
		Auth:        "none",
	})

	require.Error(t, err,
		"Scaffold must error when OutputDir does not exist")
}

// ---------------------------------------------------------------------------
// Edge case: README.md has useful content, not just a placeholder
// ---------------------------------------------------------------------------

func TestScaffold_Readme_HasUsefulContent(t *testing.T) {
	_, projectDir := scaffoldInTempDir(t, cli.ScaffoldOptions{
		Name:        "readme-test",
		Description: "A readme test project",
		Runtime:     "shell",
		Auth:        "none",
	})

	content := readProjectFile(t, projectDir, "README.md")

	// README must be a Markdown document with at least a heading and the project name.
	assert.Contains(t, content, "#",
		"README must contain a Markdown heading")
	assert.Contains(t, content, "readme-test",
		"README must reference the project name")
	assert.True(t, len(content) > 50,
		"README must have meaningful content, not just a title. Got %d bytes", len(content))
}

// ---------------------------------------------------------------------------
// Edge case: manifests for different runtimes have same structure
// ---------------------------------------------------------------------------

func TestScaffold_Manifest_DifferentRuntimes_SameEntrypoint(t *testing.T) {
	// All runtimes should have bin/hello as the entrypoint in the manifest,
	// even though the file content differs per runtime.
	runtimes := []string{"shell", "go", "python", "typescript"}

	for _, rt := range runtimes {
		t.Run(rt, func(t *testing.T) {
			_, projectDir := scaffoldInTempDir(t, cli.ScaffoldOptions{
				Name: "ep-" + rt, Description: "d", Runtime: rt, Auth: "none",
			})

			content := readProjectFile(t, projectDir, "toolwright.yaml")
			tk, err := manifest.Parse(strings.NewReader(content))
			require.NoError(t, err)
			require.NotEmpty(t, tk.Tools)
			assert.Equal(t, "bin/hello", tk.Tools[0].Entrypoint,
				"all runtimes must reference bin/hello as entrypoint")
		})
	}
}

// ---------------------------------------------------------------------------
// Edge case: multiple scaffolds in same output dir (different names)
// ---------------------------------------------------------------------------

func TestScaffold_MultipleProjects_SameOutputDir(t *testing.T) {
	tmpDir := t.TempDir()
	fs := buildTemplateFS()
	s := New(fs)

	_, err := s.Scaffold(context.Background(), cli.ScaffoldOptions{
		Name: "proj-a", Description: "d", OutputDir: tmpDir, Runtime: "shell", Auth: "none",
	})
	require.NoError(t, err)

	_, err = s.Scaffold(context.Background(), cli.ScaffoldOptions{
		Name: "proj-b", Description: "d", OutputDir: tmpDir, Runtime: "shell", Auth: "none",
	})
	require.NoError(t, err)

	// Both projects should exist independently.
	_, err = os.Stat(filepath.Join(tmpDir, "proj-a/toolwright.yaml"))
	assert.NoError(t, err, "proj-a must exist after creating proj-b in same output dir")
	_, err = os.Stat(filepath.Join(tmpDir, "proj-b/toolwright.yaml"))
	assert.NoError(t, err, "proj-b must exist")
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func firstLine(s string) string {
	if idx := strings.IndexByte(s, '\n'); idx >= 0 {
		return s[:idx]
	}
	return s
}

func entryNames(entries []os.DirEntry) []string {
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name()
	}
	return names
}
