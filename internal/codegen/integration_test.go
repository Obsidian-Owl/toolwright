package codegen

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Obsidian-Owl/toolwright/internal/manifest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Reference manifest: exercises all auth types, various arg/flag types, both
// transports. This is the comprehensive manifest for integration tests.
// ---------------------------------------------------------------------------

func integrationManifest() manifest.Toolkit {
	return manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:        "integ-toolkit",
			Version:     "1.0.0",
			Description: "Integration test toolkit with all auth types",
		},
		Tools: []manifest.Tool{
			{
				Name:        "check-health",
				Description: "Check the health of the service",
				Entrypoint:  "./health.sh",
				Auth:        &manifest.Auth{Type: "none"},
				Args: []manifest.Arg{
					{Name: "endpoint", Type: "string", Required: true, Description: "URL of the service"},
				},
				Flags: []manifest.Flag{
					{Name: "verbose", Type: "bool", Required: false, Default: false, Description: "Enable verbose output"},
				},
			},
			{
				Name:        "deploy-app",
				Description: "Deploy an application to the cluster",
				Entrypoint:  "./deploy.sh",
				Auth: &manifest.Auth{
					Type:        "token",
					TokenEnv:    "DEPLOY_TOKEN",
					TokenFlag:   "--token",
					TokenHeader: "Authorization",
				},
				Args: []manifest.Arg{
					{Name: "app-name", Type: "string", Required: true, Description: "Name of the application"},
					{Name: "version", Type: "string", Required: false, Description: "Version to deploy"},
				},
				Flags: []manifest.Flag{
					{Name: "replicas", Type: "int", Required: false, Default: 3, Description: "Number of replicas"},
					{Name: "timeout", Type: "float", Required: false, Default: 60.0, Description: "Deploy timeout in seconds"},
					{Name: "region", Type: "string", Required: true, Enum: []string{"us-east", "us-west", "eu-west"}, Description: "Target region"},
					{Name: "dry-run", Type: "bool", Required: false, Default: false, Description: "Simulate the deployment"},
				},
			},
			{
				Name:        "fetch-secrets",
				Description: "Fetch secrets from the vault using OAuth2",
				Entrypoint:  "./secrets.sh",
				Auth: &manifest.Auth{
					Type:        "oauth2",
					ProviderURL: "https://auth.example.com",
					Scopes:      []string{"secrets:read", "secrets:write"},
				},
				Args: []manifest.Arg{
					{Name: "path", Type: "string", Required: true, Description: "Secret path"},
				},
				Flags: []manifest.Flag{
					{Name: "format", Type: "string", Required: false, Default: "json", Description: "Output format"},
				},
			},
		},
		Generate: manifest.Generate{
			CLI: manifest.CLIConfig{
				Target: "go",
			},
			MCP: manifest.MCPConfig{
				Target:    "typescript",
				Transport: []string{"stdio", "streamable-http"},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// Helper: write generated files to a temp directory
// ---------------------------------------------------------------------------

func writeGeneratedFiles(t *testing.T, dir string, files []GeneratedFile) {
	t.Helper()
	for _, f := range files {
		destPath := filepath.Join(dir, f.Path)
		require.NoError(t, os.MkdirAll(filepath.Dir(destPath), 0755),
			"creating directory for %s", f.Path)
		require.NoError(t, os.WriteFile(destPath, f.Content, 0644),
			"writing file %s", f.Path)
	}
}

// ---------------------------------------------------------------------------
// AC-6: Go CLI compilation test
//
// This test generates the full Go CLI project, writes it to a temp dir,
// runs go mod tidy + go build, and asserts success. If the generated code
// has syntax errors, missing imports, or an invalid go.mod, this will catch it.
// ---------------------------------------------------------------------------

func TestIntegration_GoCLI_Compiles(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	m := integrationManifest()
	gen := NewGoCLIGenerator()
	data := TemplateData{
		Manifest:  m,
		Timestamp: "2026-03-04T12:00:00Z",
		Version:   "0.1.0",
	}

	files, err := gen.Generate(context.Background(), data, "")
	require.NoError(t, err, "GoCLIGenerator.Generate must not error")
	require.NotEmpty(t, files, "must generate at least one file")

	dir := t.TempDir()
	writeGeneratedFiles(t, dir, files)

	// Verify all expected files exist on disk before attempting build
	expectedPaths := []string{
		"cmd/integ-toolkit/main.go",
		"internal/commands/root.go",
		"internal/commands/check-health.go",
		"internal/commands/deploy-app.go",
		"internal/commands/fetch-secrets.go",
		"internal/auth/resolver.go",
		"internal/commands/login.go",
		"go.mod",
		"Makefile",
		"README.md",
	}
	for _, p := range expectedPaths {
		fullPath := filepath.Join(dir, p)
		_, statErr := os.Stat(fullPath)
		require.NoError(t, statErr, "expected file %q to exist on disk", p)
	}

	// Run go mod tidy to resolve dependency checksums.
	// This is needed because the generated go.mod only has require directives,
	// not a go.sum file.
	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Dir = dir
	tidyOutput, tidyErr := tidyCmd.CombinedOutput()
	require.NoError(t, tidyErr,
		"go mod tidy failed in generated project:\n%s", string(tidyOutput))

	// Run go build ./... to verify the generated code compiles.
	buildCmd := exec.Command("go", "build", "./...")
	buildCmd.Dir = dir
	buildOutput, buildErr := buildCmd.CombinedOutput()
	assert.NoError(t, buildErr,
		"go build ./... failed in generated project:\n%s", string(buildOutput))
}

// ---------------------------------------------------------------------------
// AC-6: Go CLI go.mod validation
//
// Verifies the generated go.mod has a valid module path and requires cobra.
// This is a structural check that does not need compilation.
// ---------------------------------------------------------------------------

func TestIntegration_GoCLI_GoModValid(t *testing.T) {
	m := integrationManifest()
	gen := NewGoCLIGenerator()
	data := TemplateData{
		Manifest:  m,
		Timestamp: "2026-03-04T12:00:00Z",
		Version:   "0.1.0",
	}

	files, err := gen.Generate(context.Background(), data, "")
	require.NoError(t, err)

	goModFile := requireFile(t, files, "go.mod")
	goModContent := string(goModFile.Content)

	// Must start with a valid module directive
	assert.True(t, strings.HasPrefix(goModContent, "module "),
		"go.mod must start with 'module' directive, got:\n%s", goModContent)

	// Module path must include the toolkit name
	lines := strings.Split(goModContent, "\n")
	require.NotEmpty(t, lines, "go.mod must have at least one line")
	moduleLine := lines[0]
	assert.Contains(t, moduleLine, "integ-toolkit",
		"module path must reference the toolkit name, got: %s", moduleLine)

	// Must contain a Go version directive
	assert.Regexp(t, `go \d+\.\d+`, goModContent,
		"go.mod must contain a 'go X.Y' version directive")

	// Must require cobra
	assert.Contains(t, goModContent, "github.com/spf13/cobra",
		"go.mod must require github.com/spf13/cobra")

	// Cobra version must be specified (not just the path)
	assert.Regexp(t, `github\.com/spf13/cobra\s+v\d+`, goModContent,
		"go.mod must pin cobra to a specific version")
}

// ---------------------------------------------------------------------------
// AC-6: Go CLI go.mod has a require block (not just inline requires)
//
// Catches a lazy implementation that just writes "module foo\ngo 1.21"
// without any dependency declarations.
// ---------------------------------------------------------------------------

func TestIntegration_GoCLI_GoModHasRequireBlock(t *testing.T) {
	m := integrationManifest()
	gen := NewGoCLIGenerator()
	data := TemplateData{
		Manifest:  m,
		Timestamp: "2026-03-04T12:00:00Z",
		Version:   "0.1.0",
	}

	files, err := gen.Generate(context.Background(), data, "")
	require.NoError(t, err)

	content := fileContent(t, files, "go.mod")
	assert.Contains(t, content, "require",
		"go.mod must contain a 'require' block for dependencies")
}

// ---------------------------------------------------------------------------
// AC-6: Go CLI main.go imports the commands package correctly
//
// A compilation test catches this too, but this unit-level check gives
// faster feedback and a clearer error message.
// ---------------------------------------------------------------------------

func TestIntegration_GoCLI_MainGoImportsCommands(t *testing.T) {
	m := integrationManifest()
	files := generateCLI(t, m)
	content := fileContent(t, files, "cmd/integ-toolkit/main.go")

	// The import path must match the module name + internal/commands
	assert.Contains(t, content, "integ-toolkit/internal/commands",
		"main.go must import the internal/commands package using the correct module path")
}

// ---------------------------------------------------------------------------
// AC-6: Generated Go files all have valid package declarations
//
// Catches template bugs that produce "package " with an empty name or
// no package declaration at all.
// ---------------------------------------------------------------------------

func TestIntegration_GoCLI_AllGoFilesHaveValidPackageDecl(t *testing.T) {
	m := integrationManifest()
	files := generateCLI(t, m)

	for _, f := range files {
		if !strings.HasSuffix(f.Path, ".go") {
			continue
		}
		content := string(f.Content)

		// Must contain "package <name>" where <name> is a non-empty identifier
		assert.Regexp(t, `package [a-z]\w*`, content,
			"Go file %q must contain a valid 'package <name>' declaration", f.Path)
	}
}

// ---------------------------------------------------------------------------
// AC-6: Generated Go files that import cobra use the correct import path
// ---------------------------------------------------------------------------

func TestIntegration_GoCLI_CobraImportPath(t *testing.T) {
	m := integrationManifest()
	files := generateCLI(t, m)

	for _, f := range files {
		if !strings.HasSuffix(f.Path, ".go") {
			continue
		}
		content := string(f.Content)
		if strings.Contains(content, "cobra") {
			assert.Contains(t, content, `"github.com/spf13/cobra"`,
				"file %q imports cobra but must use the canonical import path", f.Path)
		}
	}
}

// ---------------------------------------------------------------------------
// Integration: Go CLI with all three auth types produces complete file set
//
// The integration manifest has auth:none, auth:token, and auth:oauth2 tools.
// This verifies the conditional file generation logic works correctly when
// all three auth types coexist.
// ---------------------------------------------------------------------------

func TestIntegration_GoCLI_AllAuthTypes_ProducesCompleteFileSet(t *testing.T) {
	m := integrationManifest()
	files := generateCLI(t, m)

	// Per-tool files for all 3 tools
	requireFile(t, files, "internal/commands/check-health.go")
	requireFile(t, files, "internal/commands/deploy-app.go")
	requireFile(t, files, "internal/commands/fetch-secrets.go")

	// Auth resolver must be present (we have token and oauth2 tools)
	requireFile(t, files, "internal/auth/resolver.go")

	// Login must be present (we have an oauth2 tool)
	requireFile(t, files, "internal/commands/login.go")

	// Infrastructure files
	requireFile(t, files, "cmd/integ-toolkit/main.go")
	requireFile(t, files, "internal/commands/root.go")
	requireFile(t, files, "go.mod")
	requireFile(t, files, "Makefile")
	requireFile(t, files, "README.md")

	// Total: 3 tools + root + login + main + resolver + go.mod + Makefile + README = 10
	assert.GreaterOrEqual(t, len(files), 10,
		"integration manifest with 3 tools + all auth types must produce at least 10 files, got %d", len(files))
}

// ---------------------------------------------------------------------------
// Integration: Auth code correctness per tool
//
// With 3 tools of different auth types, verify each tool command file has
// auth code appropriate to its auth type (not a copy of another tool).
// ---------------------------------------------------------------------------

func TestIntegration_GoCLI_PerToolAuthCode(t *testing.T) {
	m := integrationManifest()
	files := generateCLI(t, m)

	// check-health (auth:none) - must NOT have auth token resolution
	healthContent := fileContent(t, files, "internal/commands/check-health.go")
	healthLower := strings.ToLower(healthContent)
	assert.False(t,
		strings.Contains(healthLower, "resolvetoken") ||
			strings.Contains(healthLower, "getenv") && strings.Contains(healthLower, "token"),
		"check-health (auth:none) must not contain token resolution code")

	// deploy-app (auth:token) - must reference DEPLOY_TOKEN
	deployContent := fileContent(t, files, "internal/commands/deploy-app.go")
	assert.Contains(t, deployContent, "DEPLOY_TOKEN",
		"deploy-app (auth:token) must reference the DEPLOY_TOKEN env var")

	// fetch-secrets (auth:oauth2) - must have auth code
	secretsContent := fileContent(t, files, "internal/commands/fetch-secrets.go")
	secretsLower := strings.ToLower(secretsContent)
	assert.True(t,
		strings.Contains(secretsLower, "token") ||
			strings.Contains(secretsLower, "auth") ||
			strings.Contains(secretsLower, "oauth"),
		"fetch-secrets (auth:oauth2) must contain auth-related code")
}

// ---------------------------------------------------------------------------
// Integration: Each per-tool command file is unique (not duplicated)
//
// Catches a template bug where all tool files get the same content.
// ---------------------------------------------------------------------------

func TestIntegration_GoCLI_PerToolFilesAreDistinct(t *testing.T) {
	m := integrationManifest()
	files := generateCLI(t, m)

	healthContent := fileContent(t, files, "internal/commands/check-health.go")
	deployContent := fileContent(t, files, "internal/commands/deploy-app.go")
	secretsContent := fileContent(t, files, "internal/commands/fetch-secrets.go")

	assert.NotEqual(t, healthContent, deployContent,
		"check-health and deploy-app must produce different command files")
	assert.NotEqual(t, deployContent, secretsContent,
		"deploy-app and fetch-secrets must produce different command files")
	assert.NotEqual(t, healthContent, secretsContent,
		"check-health and fetch-secrets must produce different command files")

	// Each file must reference its own tool name
	assert.Contains(t, healthContent, "check-health",
		"check-health.go must reference its tool name")
	assert.Contains(t, deployContent, "deploy-app",
		"deploy-app.go must reference its tool name")
	assert.Contains(t, secretsContent, "fetch-secrets",
		"fetch-secrets.go must reference its tool name")
}

// ---------------------------------------------------------------------------
// Integration: Flags with all four types are correctly generated
//
// The integration manifest has bool, int, float, and string flags on the
// deploy-app tool. Verify all types appear in the generated code.
// ---------------------------------------------------------------------------

func TestIntegration_GoCLI_AllFlagTypesPresent(t *testing.T) {
	m := integrationManifest()
	files := generateCLI(t, m)
	content := fileContent(t, files, "internal/commands/deploy-app.go")

	// The deploy-app tool has:
	// - replicas: int
	// - timeout: float (should map to float64)
	// - region: string (with enum)
	// - dry-run: bool
	assert.Contains(t, content, "replicas",
		"deploy-app must contain the 'replicas' flag")
	assert.Contains(t, content, "timeout",
		"deploy-app must contain the 'timeout' flag")
	assert.Contains(t, content, "region",
		"deploy-app must contain the 'region' flag")
	assert.Contains(t, content, "dry-run",
		"deploy-app must contain the 'dry-run' flag")

	// Type verification
	assert.True(t,
		strings.Contains(content, "IntVar") || strings.Contains(content, "int"),
		"deploy-app must use int type for replicas")
	assert.True(t,
		strings.Contains(content, "Float64Var") || strings.Contains(content, "float64"),
		"deploy-app must use float64 type for timeout")
	assert.True(t,
		strings.Contains(content, "BoolVar") || strings.Contains(content, "bool"),
		"deploy-app must use bool type for dry-run")
}

// ---------------------------------------------------------------------------
// AC-7/AC-16: TypeScript MCP structure validation
//
// Generates the full TS MCP project and verifies structural correctness:
// all expected files exist, package.json and tsconfig.json parse as valid JSON.
// ---------------------------------------------------------------------------

func TestIntegration_TSMCP_StructureValid(t *testing.T) {
	m := integrationManifest()
	gen := NewTSMCPGenerator()
	data := TemplateData{
		Manifest:  m,
		Timestamp: "2026-03-04T12:00:00Z",
		Version:   "0.1.0",
	}

	files, err := gen.Generate(context.Background(), data, "")
	require.NoError(t, err, "TSMCPGenerator.Generate must not error")
	require.NotEmpty(t, files, "must generate at least one file")

	dir := t.TempDir()
	writeGeneratedFiles(t, dir, files)

	// Expected files for the integration manifest:
	// src/index.ts
	// src/tools/check-health.ts
	// src/tools/deploy-app.ts
	// src/tools/fetch-secrets.ts
	// src/search.ts
	// package.json
	// tsconfig.json
	// README.md
	// src/auth/middleware.ts (because we have token and oauth2 tools)
	// src/auth/metadata.ts (because we have oauth2 + streamable-http)
	expectedFiles := []string{
		"src/index.ts",
		"src/tools/check-health.ts",
		"src/tools/deploy-app.ts",
		"src/tools/fetch-secrets.ts",
		"src/search.ts",
		"package.json",
		"tsconfig.json",
		"README.md",
		"src/auth/middleware.ts",
		"src/auth/metadata.ts",
	}

	for _, p := range expectedFiles {
		fullPath := filepath.Join(dir, p)
		_, statErr := os.Stat(fullPath)
		assert.NoError(t, statErr,
			"expected file %q to exist on disk", p)
	}
}

func TestIntegration_TSMCP_PackageJSONIsValidJSON(t *testing.T) {
	m := integrationManifest()
	gen := NewTSMCPGenerator()
	data := TemplateData{
		Manifest:  m,
		Timestamp: "2026-03-04T12:00:00Z",
		Version:   "0.1.0",
	}

	files, err := gen.Generate(context.Background(), data, "")
	require.NoError(t, err)

	content := fileContent(t, files, "package.json")

	var parsed map[string]interface{}
	jsonErr := json.Unmarshal([]byte(content), &parsed)
	require.NoError(t, jsonErr,
		"package.json must be valid JSON, got error: %v\ncontent:\n%s", jsonErr, content)

	// Verify essential fields
	assert.Equal(t, "integ-toolkit", parsed["name"],
		"package.json 'name' must match toolkit name")
	assert.NotEmpty(t, parsed["version"],
		"package.json must have a 'version' field")

	// Must have dependencies
	deps, ok := parsed["dependencies"].(map[string]interface{})
	require.True(t, ok, "package.json must have a 'dependencies' object")
	assert.Contains(t, deps, "@modelcontextprotocol/sdk",
		"package.json must depend on @modelcontextprotocol/sdk")
	assert.Contains(t, deps, "zod",
		"package.json must depend on zod")

	// Must have scripts
	scripts, ok := parsed["scripts"].(map[string]interface{})
	require.True(t, ok, "package.json must have a 'scripts' object")
	assert.Contains(t, scripts, "build",
		"package.json must have a 'build' script")
}

func TestIntegration_TSMCP_TsconfigIsValidJSON(t *testing.T) {
	m := integrationManifest()
	gen := NewTSMCPGenerator()
	data := TemplateData{
		Manifest:  m,
		Timestamp: "2026-03-04T12:00:00Z",
		Version:   "0.1.0",
	}

	files, err := gen.Generate(context.Background(), data, "")
	require.NoError(t, err)

	content := fileContent(t, files, "tsconfig.json")

	var parsed map[string]interface{}
	jsonErr := json.Unmarshal([]byte(content), &parsed)
	require.NoError(t, jsonErr,
		"tsconfig.json must be valid JSON, got error: %v\ncontent:\n%s", jsonErr, content)

	// Verify essential fields
	compilerOpts, ok := parsed["compilerOptions"].(map[string]interface{})
	require.True(t, ok, "tsconfig.json must have a 'compilerOptions' object")
	assert.Contains(t, compilerOpts, "strict",
		"tsconfig.json must set 'strict' in compiler options")
	assert.Equal(t, true, compilerOpts["strict"],
		"tsconfig.json strict mode must be enabled")
}

// ---------------------------------------------------------------------------
// AC-7: TS MCP metadata.ts is present only when oauth2 + streamable-http
// ---------------------------------------------------------------------------

func TestIntegration_TSMCP_MetadataOnlyWithOAuth2AndStreamableHTTP(t *testing.T) {
	// Our integration manifest has oauth2 + streamable-http -> metadata.ts must exist
	m := integrationManifest()
	gen := NewTSMCPGenerator()
	data := TemplateData{
		Manifest:  m,
		Timestamp: "2026-03-04T12:00:00Z",
		Version:   "0.1.0",
	}

	files, err := gen.Generate(context.Background(), data, "")
	require.NoError(t, err)

	requireFile(t, files, "src/auth/metadata.ts")

	// Now test without streamable-http: metadata.ts must NOT exist
	mStdioOnly := integrationManifest()
	mStdioOnly.Generate.MCP.Transport = []string{"stdio"}

	files2, err := gen.Generate(context.Background(), TemplateData{
		Manifest:  mStdioOnly,
		Timestamp: "2026-03-04T12:00:00Z",
		Version:   "0.1.0",
	}, "")
	require.NoError(t, err)
	assertNoFile(t, files2, "src/auth/metadata.ts")
}

// ---------------------------------------------------------------------------
// AC-7: TS MCP middleware.ts is present only when tools need auth
// ---------------------------------------------------------------------------

func TestIntegration_TSMCP_MiddlewareOnlyWithAuth(t *testing.T) {
	// Integration manifest has token + oauth2 tools -> middleware must exist
	m := integrationManifest()
	gen := NewTSMCPGenerator()

	files, err := gen.Generate(context.Background(), TemplateData{
		Manifest:  m,
		Timestamp: "2026-03-04T12:00:00Z",
		Version:   "0.1.0",
	}, "")
	require.NoError(t, err)
	requireFile(t, files, "src/auth/middleware.ts")

	// All auth:none -> no middleware
	mNoAuth := manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:    "noauth-mcp",
			Version: "1.0.0",
		},
		Tools: []manifest.Tool{
			{
				Name:       "ping",
				Entrypoint: "./ping.sh",
				Auth:       &manifest.Auth{Type: "none"},
			},
		},
		Generate: manifest.Generate{
			MCP: manifest.MCPConfig{
				Target:    "typescript",
				Transport: []string{"stdio"},
			},
		},
	}

	files2, err := gen.Generate(context.Background(), TemplateData{
		Manifest:  mNoAuth,
		Timestamp: "2026-03-04T12:00:00Z",
		Version:   "0.1.0",
	}, "")
	require.NoError(t, err)
	assertNoFile(t, files2, "src/auth/middleware.ts")
}

// ---------------------------------------------------------------------------
// Full engine round-trip: register both generators, generate both targets
// ---------------------------------------------------------------------------

func TestIntegration_Engine_FullRoundTrip_GoCLI(t *testing.T) {
	m := integrationManifest()
	eng := NewEngine()
	eng.Register(NewGoCLIGenerator())
	eng.Register(NewTSMCPGenerator())

	dir := t.TempDir()
	result, err := eng.Generate(context.Background(), m, GenerateOptions{
		Mode:      "cli",
		Target:    "go",
		OutputDir: dir,
		Version:   "0.1.0",
	})

	require.NoError(t, err, "engine.Generate for cli/go must not error")
	require.NotNil(t, result)
	assert.Equal(t, "cli", result.Mode)
	assert.Equal(t, "go", result.Target)
	assert.False(t, result.DryRun)

	// Must produce at least 10 files (3 tools + root + login + main + resolver + go.mod + Makefile + README)
	// plus the marker file
	assert.GreaterOrEqual(t, len(result.Files), 11,
		"engine round-trip for cli/go must produce at least 11 files (including marker), got %d: %v",
		len(result.Files), result.Files)

	// Verify files were written to disk
	for _, f := range result.Files {
		fullPath := filepath.Join(dir, f)
		_, statErr := os.Stat(fullPath)
		assert.NoError(t, statErr,
			"file %q listed in result must exist on disk", f)
	}
}

func TestIntegration_Engine_FullRoundTrip_TSMCP(t *testing.T) {
	m := integrationManifest()
	eng := NewEngine()
	eng.Register(NewGoCLIGenerator())
	eng.Register(NewTSMCPGenerator())

	dir := t.TempDir()
	result, err := eng.Generate(context.Background(), m, GenerateOptions{
		Mode:      "mcp",
		Target:    "typescript",
		OutputDir: dir,
		Version:   "0.1.0",
	})

	require.NoError(t, err, "engine.Generate for mcp/typescript must not error")
	require.NotNil(t, result)
	assert.Equal(t, "mcp", result.Mode)
	assert.Equal(t, "typescript", result.Target)

	// Must produce at least: index.ts, 3 tool files, search.ts, package.json,
	// tsconfig.json, README, middleware.ts, metadata.ts, marker = 11
	assert.GreaterOrEqual(t, len(result.Files), 11,
		"engine round-trip for mcp/typescript must produce at least 11 files (including marker), got %d: %v",
		len(result.Files), result.Files)

	// Verify files were written to disk
	for _, f := range result.Files {
		fullPath := filepath.Join(dir, f)
		_, statErr := os.Stat(fullPath)
		assert.NoError(t, statErr,
			"file %q listed in result must exist on disk", f)
	}
}

// ---------------------------------------------------------------------------
// Full engine round-trip: both generators in sequence with separate output dirs
// ---------------------------------------------------------------------------

func TestIntegration_Engine_BothGenerators_Independently(t *testing.T) {
	m := integrationManifest()
	eng := NewEngine()
	eng.Register(NewGoCLIGenerator())
	eng.Register(NewTSMCPGenerator())

	goDir := t.TempDir()
	tsDir := t.TempDir()

	// Generate Go CLI
	goResult, goErr := eng.Generate(context.Background(), m, GenerateOptions{
		Mode:      "cli",
		Target:    "go",
		OutputDir: goDir,
		Version:   "0.1.0",
	})
	require.NoError(t, goErr, "cli/go generation must not error")
	require.NotNil(t, goResult)

	// Generate TS MCP
	tsResult, tsErr := eng.Generate(context.Background(), m, GenerateOptions{
		Mode:      "mcp",
		Target:    "typescript",
		OutputDir: tsDir,
		Version:   "0.1.0",
	})
	require.NoError(t, tsErr, "mcp/typescript generation must not error")
	require.NotNil(t, tsResult)

	// Go CLI must have .go files and no .ts files
	for _, f := range goResult.Files {
		assert.False(t, strings.HasSuffix(f, ".ts"),
			"Go CLI output must not contain TypeScript files, found: %s", f)
	}

	// TS MCP must have .ts files and no .go files
	for _, f := range tsResult.Files {
		assert.False(t, strings.HasSuffix(f, ".go"),
			"TS MCP output must not contain Go files, found: %s", f)
	}

	// Both must have marker files
	assert.Contains(t, goResult.Files, ".toolwright-generated")
	assert.Contains(t, tsResult.Files, ".toolwright-generated")
}

// ---------------------------------------------------------------------------
// AC-6: Generated Go code has no unused imports (go vet would catch this)
//
// This is a weaker check than compilation but catches template bugs that
// produce unused import statements.
// ---------------------------------------------------------------------------

func TestIntegration_GoCLI_GoVetClean(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	m := integrationManifest()
	gen := NewGoCLIGenerator()
	data := TemplateData{
		Manifest:  m,
		Timestamp: "2026-03-04T12:00:00Z",
		Version:   "0.1.0",
	}

	files, err := gen.Generate(context.Background(), data, "")
	require.NoError(t, err)

	dir := t.TempDir()
	writeGeneratedFiles(t, dir, files)

	// go mod tidy first
	tidyCmd := exec.Command("go", "mod", "tidy")
	tidyCmd.Dir = dir
	tidyOutput, tidyErr := tidyCmd.CombinedOutput()
	require.NoError(t, tidyErr,
		"go mod tidy failed:\n%s", string(tidyOutput))

	// go vet ./...
	vetCmd := exec.Command("go", "vet", "./...")
	vetCmd.Dir = dir
	vetOutput, vetErr := vetCmd.CombinedOutput()
	assert.NoError(t, vetErr,
		"go vet ./... failed in generated project:\n%s", string(vetOutput))
}

// ---------------------------------------------------------------------------
// TS MCP: generated index.ts imports correct tools
// ---------------------------------------------------------------------------

func TestIntegration_TSMCP_IndexImportsAllTools(t *testing.T) {
	m := integrationManifest()
	gen := NewTSMCPGenerator()
	data := TemplateData{
		Manifest:  m,
		Timestamp: "2026-03-04T12:00:00Z",
		Version:   "0.1.0",
	}

	files, err := gen.Generate(context.Background(), data, "")
	require.NoError(t, err)

	content := fileContent(t, files, "src/index.ts")

	// Must import each tool's handler
	for _, toolName := range []string{"check-health", "deploy-app", "fetch-secrets"} {
		assert.Contains(t, content, toolName,
			"index.ts must import the %s tool handler", toolName)
	}

	// Must import MCP SDK
	assert.Contains(t, content, "@modelcontextprotocol/sdk",
		"index.ts must import from @modelcontextprotocol/sdk")

	// Must import both transports (our manifest has stdio and streamable-http)
	assert.Contains(t, content, "StdioServerTransport",
		"index.ts must import StdioServerTransport for stdio transport")
	assert.Contains(t, content, "StreamableHTTPServerTransport",
		"index.ts must import StreamableHTTPServerTransport for streamable-http transport")
}

// ---------------------------------------------------------------------------
// TS MCP: per-tool files are distinct and reference their own tool name
// ---------------------------------------------------------------------------

func TestIntegration_TSMCP_PerToolFilesAreDistinct(t *testing.T) {
	m := integrationManifest()
	gen := NewTSMCPGenerator()
	data := TemplateData{
		Manifest:  m,
		Timestamp: "2026-03-04T12:00:00Z",
		Version:   "0.1.0",
	}

	files, err := gen.Generate(context.Background(), data, "")
	require.NoError(t, err)

	healthContent := fileContent(t, files, "src/tools/check-health.ts")
	deployContent := fileContent(t, files, "src/tools/deploy-app.ts")
	secretsContent := fileContent(t, files, "src/tools/fetch-secrets.ts")

	// All three must be different
	assert.NotEqual(t, healthContent, deployContent)
	assert.NotEqual(t, deployContent, secretsContent)
	assert.NotEqual(t, healthContent, secretsContent)

	// Each must reference its own tool name
	assert.Contains(t, healthContent, "check-health")
	assert.Contains(t, deployContent, "deploy-app")
	assert.Contains(t, secretsContent, "fetch-secrets")
}

// ---------------------------------------------------------------------------
// TS MCP: metadata.ts contains OAuth2 provider URL and scopes
// ---------------------------------------------------------------------------

func TestIntegration_TSMCP_MetadataContainsOAuthConfig(t *testing.T) {
	m := integrationManifest()
	gen := NewTSMCPGenerator()
	data := TemplateData{
		Manifest:  m,
		Timestamp: "2026-03-04T12:00:00Z",
		Version:   "0.1.0",
	}

	files, err := gen.Generate(context.Background(), data, "")
	require.NoError(t, err)

	content := fileContent(t, files, "src/auth/metadata.ts")

	// Must reference the OAuth provider URL from the manifest
	assert.Contains(t, content, "https://auth.example.com",
		"metadata.ts must contain the OAuth provider URL")

	// Must reference the scopes
	assert.Contains(t, content, "secrets:read",
		"metadata.ts must contain the 'secrets:read' scope")
	assert.Contains(t, content, "secrets:write",
		"metadata.ts must contain the 'secrets:write' scope")
}

// ---------------------------------------------------------------------------
// TS MCP: file count validation for integration manifest
// ---------------------------------------------------------------------------

func TestIntegration_TSMCP_FileCount(t *testing.T) {
	m := integrationManifest()
	gen := NewTSMCPGenerator()
	data := TemplateData{
		Manifest:  m,
		Timestamp: "2026-03-04T12:00:00Z",
		Version:   "0.1.0",
	}

	files, err := gen.Generate(context.Background(), data, "")
	require.NoError(t, err)

	// Expected: index.ts, 3 tool files, search.ts, package.json, tsconfig.json,
	// README.md, middleware.ts, metadata.ts = 10 files
	assert.Equal(t, 10, len(files),
		"integration manifest should produce exactly 10 TS MCP files, got %d: %v",
		len(files), filePaths(files))
}

// ---------------------------------------------------------------------------
// Go CLI: file count validation for integration manifest
// ---------------------------------------------------------------------------

func TestIntegration_GoCLI_FileCount(t *testing.T) {
	m := integrationManifest()
	files := generateCLI(t, m)

	// Expected:
	// cmd/integ-toolkit/main.go
	// internal/commands/root.go
	// internal/commands/check-health.go
	// internal/commands/deploy-app.go
	// internal/commands/fetch-secrets.go
	// internal/auth/resolver.go
	// internal/commands/login.go
	// go.mod
	// Makefile
	// README.md
	// = 10 files
	assert.Equal(t, 10, len(files),
		"integration manifest should produce exactly 10 Go CLI files, got %d: %v",
		len(files), filePaths(files))
}

// ---------------------------------------------------------------------------
// Integration: No duplicate file paths across all generated files
// ---------------------------------------------------------------------------

func TestIntegration_GoCLI_NoDuplicatePaths(t *testing.T) {
	m := integrationManifest()
	files := generateCLI(t, m)

	seen := make(map[string]bool)
	for _, f := range files {
		assert.Falsef(t, seen[f.Path],
			"duplicate file path %q in Go CLI output", f.Path)
		seen[f.Path] = true
	}
}

func TestIntegration_TSMCP_NoDuplicatePaths(t *testing.T) {
	m := integrationManifest()
	gen := NewTSMCPGenerator()
	data := TemplateData{
		Manifest:  m,
		Timestamp: "2026-03-04T12:00:00Z",
		Version:   "0.1.0",
	}

	files, err := gen.Generate(context.Background(), data, "")
	require.NoError(t, err)

	seen := make(map[string]bool)
	for _, f := range files {
		assert.Falsef(t, seen[f.Path],
			"duplicate file path %q in TS MCP output", f.Path)
		seen[f.Path] = true
	}
}

// ---------------------------------------------------------------------------
// Integration: No empty generated files
// ---------------------------------------------------------------------------

func TestIntegration_AllGeneratedFilesNonEmpty(t *testing.T) {
	m := integrationManifest()

	// Go CLI
	goFiles := generateCLI(t, m)
	for _, f := range goFiles {
		assert.NotEmptyf(t, f.Content,
			"Go CLI file %q must not be empty", f.Path)
	}

	// TS MCP
	tsGen := NewTSMCPGenerator()
	tsData := TemplateData{
		Manifest:  m,
		Timestamp: "2026-03-04T12:00:00Z",
		Version:   "0.1.0",
	}
	tsFiles, err := tsGen.Generate(context.Background(), tsData, "")
	require.NoError(t, err)
	for _, f := range tsFiles {
		assert.NotEmptyf(t, f.Content,
			"TS MCP file %q must not be empty", f.Path)
	}
}

// ---------------------------------------------------------------------------
// AC5.3: CLI generated code uses entrypoints, not echo stubs
//
// Every tool file must invoke exec.CommandContext with the tool's manifest
// entrypoint, not a placeholder like "echo". This catches regressions where
// the template falls back to stub behavior instead of wiring the real
// entrypoint.
// ---------------------------------------------------------------------------

func TestIntegration_GoCLI_EntrypointNotEcho(t *testing.T) {
	m := integrationManifest()
	files := generateCLI(t, m)

	toolFiles := map[string]string{
		"internal/commands/check-health.go":  "./health.sh",
		"internal/commands/deploy-app.go":    "./deploy.sh",
		"internal/commands/fetch-secrets.go": "./secrets.sh",
	}

	for path, entrypoint := range toolFiles {
		content := fileContent(t, files, path)

		// Must contain the exact entrypoint string from the manifest
		assert.Contains(t, content, entrypoint,
			"%s must contain its manifest entrypoint %q", path, entrypoint)

		// Must NOT use "echo" as the executable (stub pattern)
		assert.NotContains(t, content, `"echo"`,
			"%s must not use \"echo\" as stub executable", path)

		// Must use exec.CommandContext (the real execution path)
		assert.Contains(t, content, "exec.CommandContext",
			"%s must use exec.CommandContext to invoke the entrypoint", path)
	}
}

// ---------------------------------------------------------------------------
// AC5.3: MCP generated code uses execFile, not TODO stubs
//
// Every tool file must use execFile with its manifest entrypoint. No TODO
// stubs should remain in the generated output.
// ---------------------------------------------------------------------------

func TestIntegration_TSMCP_EntrypointNotTODO(t *testing.T) {
	m := integrationManifest()
	files := generateTSMCP(t, m)

	toolFiles := map[string]string{
		"src/tools/check-health.ts":  "./health.sh",
		"src/tools/deploy-app.ts":    "./deploy.sh",
		"src/tools/fetch-secrets.ts": "./secrets.sh",
	}

	for path, entrypoint := range toolFiles {
		content := fileContent(t, files, path)

		// Must contain the exact entrypoint string from the manifest
		assert.Contains(t, content, entrypoint,
			"%s must contain its manifest entrypoint %q", path, entrypoint)

		// Must NOT contain TODO stub comments
		assert.NotContains(t, content, "// TODO: implement",
			"%s must not contain TODO implementation stubs", path)

		// Must use execFile (the real execution mechanism)
		assert.Contains(t, content, "await execFile(",
			"%s must use await execFile() to invoke the entrypoint", path)
	}
}

// ---------------------------------------------------------------------------
// AC5: CLI code has cliArgs construction for entrypoint invocation
//
// All tool files must build a cliArgs slice that is passed to the entrypoint.
// This catches a lazy implementation that calls exec.Command with no arguments.
// ---------------------------------------------------------------------------

func TestIntegration_GoCLI_CliArgsConstruction(t *testing.T) {
	m := integrationManifest()
	files := generateCLI(t, m)

	toolFiles := []string{
		"internal/commands/check-health.go",
		"internal/commands/deploy-app.go",
		"internal/commands/fetch-secrets.go",
	}

	for _, path := range toolFiles {
		content := fileContent(t, files, path)

		// Must declare the cliArgs variable
		assert.Contains(t, content, "cliArgs",
			"%s must build a cliArgs slice for entrypoint invocation", path)

		// Must use cliArgs in the exec.CommandContext call (not an empty call)
		assert.Contains(t, content, "cliArgs...",
			"%s must spread cliArgs into exec.CommandContext", path)
	}
}

// ---------------------------------------------------------------------------
// AC5: MCP code has args construction for entrypoint invocation
//
// All tool files must declare const args: string[] and pass it to execFile.
// ---------------------------------------------------------------------------

func TestIntegration_TSMCP_ArgsConstruction(t *testing.T) {
	m := integrationManifest()
	files := generateTSMCP(t, m)

	toolFiles := []string{
		"src/tools/check-health.ts",
		"src/tools/deploy-app.ts",
		"src/tools/fetch-secrets.ts",
	}

	for _, path := range toolFiles {
		content := fileContent(t, files, path)

		// Must declare the args array
		assert.Contains(t, content, "const args: string[]",
			"%s must declare a typed args array for entrypoint invocation", path)

		// Must push positional args (all tools have at least one arg)
		assert.Contains(t, content, "args.push(",
			"%s must push arguments into the args array", path)
	}
}

// ---------------------------------------------------------------------------
// AC5: CLI token tool passes --token in cliArgs
//
// deploy-app has auth:token with TokenFlag "--token". The generated code must
// append "--token" and the resolved token value to cliArgs so the entrypoint
// receives the token via CLI flag (constitution rule 24).
// ---------------------------------------------------------------------------

func TestIntegration_GoCLI_TokenInCliArgs(t *testing.T) {
	m := integrationManifest()
	files := generateCLI(t, m)
	content := fileContent(t, files, "internal/commands/deploy-app.go")

	// Must pass --token as a flag in cliArgs (not just declare the token var)
	assert.Contains(t, content, `"--token"`,
		"deploy-app must pass --token flag in cliArgs for entrypoint invocation")

	// Must also reference the token variable in the cliArgs append
	assert.Contains(t, content, "token",
		"deploy-app must include the resolved token value in cliArgs")

	// The token flag must be appended to cliArgs, not just used in cobra flags
	assert.Regexp(t, `cliArgs\s*=\s*append\(cliArgs.*"--token"`, content,
		"deploy-app must append --token to cliArgs for the entrypoint")
}

// ---------------------------------------------------------------------------
// AC5: MCP token tool passes --token in args and reads token from env
//
// deploy-app has auth:token with TokenEnv "DEPLOY_TOKEN" and
// TokenFlag "--token". The MCP handler must read the env var and pass
// the token via --token flag to the entrypoint.
// ---------------------------------------------------------------------------

func TestIntegration_TSMCP_TokenInArgs(t *testing.T) {
	m := integrationManifest()
	files := generateTSMCP(t, m)
	content := fileContent(t, files, "src/tools/deploy-app.ts")

	// Must read the token from the environment variable
	assert.Contains(t, content, "DEPLOY_TOKEN",
		"deploy-app.ts must reference the DEPLOY_TOKEN env var")
	assert.Contains(t, content, "envToken",
		"deploy-app.ts must store env token in envToken variable")

	// Must pass --token flag in args for the entrypoint
	assert.Contains(t, content, `"--token"`,
		"deploy-app.ts must pass --token flag in args for the entrypoint")

	// Must push the token into args (not just read it)
	assert.Contains(t, content, "args.push(",
		"deploy-app.ts must push --token and envToken into args")
}

// ---------------------------------------------------------------------------
// AC6: CLI no-auth tool doesn't reference --token in cliArgs
//
// check-health has auth:none. Its generated code must not inject any
// token-related arguments into cliArgs — the entrypoint should not
// receive credentials it doesn't need.
// ---------------------------------------------------------------------------

func TestIntegration_GoCLI_NoAuthTool_NoTokenInCliArgs(t *testing.T) {
	m := integrationManifest()
	files := generateCLI(t, m)
	content := fileContent(t, files, "internal/commands/check-health.go")

	// Must NOT contain --token in the generated code at all
	assert.NotContains(t, content, `"--token"`,
		"check-health (auth:none) must not pass --token flag")

	// Must NOT reference DEPLOY_TOKEN (that belongs to deploy-app)
	assert.NotContains(t, content, "DEPLOY_TOKEN",
		"check-health (auth:none) must not reference DEPLOY_TOKEN")

	// Must NOT have token resolution code
	assert.NotContains(t, content, "os.Getenv",
		"check-health (auth:none) must not call os.Getenv for token resolution")
}

// ---------------------------------------------------------------------------
// AC6: MCP no-auth tool doesn't reference tokens in args
//
// check-health (auth:none) must not have any token-related argument pushing.
// ---------------------------------------------------------------------------

func TestIntegration_TSMCP_NoAuthTool_NoTokenInArgs(t *testing.T) {
	m := integrationManifest()
	files := generateTSMCP(t, m)
	content := fileContent(t, files, "src/tools/check-health.ts")

	// Must NOT contain --token flag
	assert.NotContains(t, content, `"--token"`,
		"check-health.ts (auth:none) must not pass --token flag")

	// Must NOT reference DEPLOY_TOKEN
	assert.NotContains(t, content, "DEPLOY_TOKEN",
		"check-health.ts (auth:none) must not reference DEPLOY_TOKEN")

	// Must NOT have envToken variable
	assert.NotContains(t, content, "envToken",
		"check-health.ts (auth:none) must not have an envToken variable")
}

// ---------------------------------------------------------------------------
// AC5: Entrypoint values match manifest exactly per tool
//
// This verifies that each tool gets its OWN entrypoint, not a copy of another
// tool's entrypoint. Catches a template bug where all tools share the same
// entrypoint value.
// ---------------------------------------------------------------------------

func TestIntegration_GoCLI_EntrypointMatchesManifestPerTool(t *testing.T) {
	m := integrationManifest()
	files := generateCLI(t, m)

	// Each tool must contain ONLY its own entrypoint, not other tools' entrypoints.
	healthContent := fileContent(t, files, "internal/commands/check-health.go")
	deployContent := fileContent(t, files, "internal/commands/deploy-app.go")
	secretsContent := fileContent(t, files, "internal/commands/fetch-secrets.go")

	// check-health -> ./health.sh only
	assert.Contains(t, healthContent, "./health.sh")
	assert.NotContains(t, healthContent, "./deploy.sh",
		"check-health.go must not contain deploy-app's entrypoint")
	assert.NotContains(t, healthContent, "./secrets.sh",
		"check-health.go must not contain fetch-secrets' entrypoint")

	// deploy-app -> ./deploy.sh only
	assert.Contains(t, deployContent, "./deploy.sh")
	assert.NotContains(t, deployContent, "./health.sh",
		"deploy-app.go must not contain check-health's entrypoint")
	assert.NotContains(t, deployContent, "./secrets.sh",
		"deploy-app.go must not contain fetch-secrets' entrypoint")

	// fetch-secrets -> ./secrets.sh only
	assert.Contains(t, secretsContent, "./secrets.sh")
	assert.NotContains(t, secretsContent, "./health.sh",
		"fetch-secrets.go must not contain check-health's entrypoint")
	assert.NotContains(t, secretsContent, "./deploy.sh",
		"fetch-secrets.go must not contain deploy-app's entrypoint")
}

// ---------------------------------------------------------------------------
// AC5: TS MCP entrypoint values match manifest exactly per tool
// ---------------------------------------------------------------------------

func TestIntegration_TSMCP_EntrypointMatchesManifestPerTool(t *testing.T) {
	m := integrationManifest()
	files := generateTSMCP(t, m)

	healthContent := fileContent(t, files, "src/tools/check-health.ts")
	deployContent := fileContent(t, files, "src/tools/deploy-app.ts")
	secretsContent := fileContent(t, files, "src/tools/fetch-secrets.ts")

	// check-health -> ./health.sh only
	assert.Contains(t, healthContent, "./health.sh")
	assert.NotContains(t, healthContent, "./deploy.sh",
		"check-health.ts must not contain deploy-app's entrypoint")
	assert.NotContains(t, healthContent, "./secrets.sh",
		"check-health.ts must not contain fetch-secrets' entrypoint")

	// deploy-app -> ./deploy.sh only
	assert.Contains(t, deployContent, "./deploy.sh")
	assert.NotContains(t, deployContent, "./health.sh",
		"deploy-app.ts must not contain check-health's entrypoint")
	assert.NotContains(t, deployContent, "./secrets.sh",
		"deploy-app.ts must not contain fetch-secrets' entrypoint")

	// fetch-secrets -> ./secrets.sh only
	assert.Contains(t, secretsContent, "./secrets.sh")
	assert.NotContains(t, secretsContent, "./health.sh",
		"fetch-secrets.ts must not contain check-health's entrypoint")
	assert.NotContains(t, secretsContent, "./deploy.sh",
		"fetch-secrets.ts must not contain deploy-app's entrypoint")
}

// ---------------------------------------------------------------------------
// AC5.4: TS files have balanced braces (basic syntax validity)
//
// Catches template rendering bugs that leave unclosed { or } in generated
// TypeScript files. This is a structural check, not a full parse, but it
// catches the most common class of template brace-mismatch bugs.
// ---------------------------------------------------------------------------

func TestIntegration_TSMCP_BalancedBraces(t *testing.T) {
	m := integrationManifest()
	files := generateTSMCP(t, m)

	for _, f := range files {
		if !strings.HasSuffix(f.Path, ".ts") {
			continue
		}
		content := string(f.Content)
		openBraces := strings.Count(content, "{")
		closeBraces := strings.Count(content, "}")
		assert.Equal(t, openBraces, closeBraces,
			"TS file %q has unbalanced braces: %d open vs %d close",
			f.Path, openBraces, closeBraces)
	}
}
