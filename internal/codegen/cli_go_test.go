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
// Test manifests
// ---------------------------------------------------------------------------

// manifestTwoToolsMixed returns a manifest with two tools: one auth:none, one
// auth:token. This is the primary test manifest for AC-2.
func manifestTwoToolsMixed() manifest.Toolkit {
	return manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:        "my-toolkit",
			Version:     "1.0.0",
			Description: "A toolkit with mixed auth",
		},
		Tools: []manifest.Tool{
			{
				Name:        "status",
				Description: "Check service status",
				Entrypoint:  "./status.sh",
				Auth:        &manifest.Auth{Type: "none"},
			},
			{
				Name:        "deploy",
				Description: "Deploy the service",
				Entrypoint:  "./deploy.sh",
				Auth: &manifest.Auth{
					Type:        "token",
					TokenEnv:    "DEPLOY_TOKEN",
					TokenFlag:   "--token",
					TokenHeader: "Authorization",
				},
			},
		},
	}
}

// manifestOAuth2 returns a manifest with an oauth2 tool.
func manifestOAuth2() manifest.Toolkit {
	return manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:        "oauth-toolkit",
			Version:     "2.0.0",
			Description: "OAuth2 toolkit",
		},
		Tools: []manifest.Tool{
			{
				Name:        "protected",
				Description: "A protected resource",
				Entrypoint:  "./protected.sh",
				Auth: &manifest.Auth{
					Type:        "oauth2",
					ProviderURL: "https://auth.example.com",
					Scopes:      []string{"read", "write"},
				},
			},
		},
	}
}

// manifestAllAuthNone returns a manifest where all tools use auth:none.
func manifestAllAuthNone() manifest.Toolkit {
	return manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:        "noauth-toolkit",
			Version:     "1.0.0",
			Description: "No-auth toolkit",
		},
		Tools: []manifest.Tool{
			{
				Name:        "ping",
				Description: "Ping a host",
				Entrypoint:  "./ping.sh",
				Auth:        &manifest.Auth{Type: "none"},
			},
			{
				Name:        "echo",
				Description: "Echo input",
				Entrypoint:  "./echo.sh",
				Auth:        &manifest.Auth{Type: "none"},
			},
		},
	}
}

// manifestWithArgsAndFlags returns a manifest whose tool has various args
// and flags to test per-tool subcommand generation (AC-5).
func manifestWithArgsAndFlags() manifest.Toolkit {
	return manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:        "argflag-toolkit",
			Version:     "1.0.0",
			Description: "Args and flags toolkit",
		},
		Tools: []manifest.Tool{
			{
				Name:        "upload",
				Description: "Upload a file to the server",
				Entrypoint:  "./upload.sh",
				Auth:        &manifest.Auth{Type: "none"},
				Args: []manifest.Arg{
					{Name: "source", Type: "string", Required: true, Description: "Source file path"},
					{Name: "destination", Type: "string", Required: false, Description: "Destination path"},
				},
				Flags: []manifest.Flag{
					{Name: "verbose", Type: "bool", Required: false, Default: false, Description: "Enable verbose output"},
					{Name: "retries", Type: "int", Required: false, Default: 3, Description: "Number of retries"},
					{Name: "timeout", Type: "float", Required: false, Default: 30.0, Description: "Timeout in seconds"},
					{Name: "format", Type: "string", Required: true, Enum: []string{"json", "yaml", "csv"}, Description: "Output format"},
				},
			},
		},
	}
}

// manifestInheritedAuth returns a manifest with toolkit-level auth that tools
// inherit (no per-tool auth override).
func manifestInheritedAuth() manifest.Toolkit {
	return manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:        "inherited-auth",
			Version:     "1.0.0",
			Description: "Toolkit-level auth inheritance",
		},
		Auth: &manifest.Auth{
			Type:     "token",
			TokenEnv: "API_TOKEN",
		},
		Tools: []manifest.Tool{
			{
				Name:        "list",
				Description: "List resources",
				Entrypoint:  "./list.sh",
				// No per-tool auth: inherits toolkit-level token auth.
			},
		},
	}
}

// ---------------------------------------------------------------------------
// Generator instantiation and interface compliance
// ---------------------------------------------------------------------------

func generateCLI(t *testing.T, m manifest.Toolkit) []GeneratedFile {
	t.Helper()
	gen := NewGoCLIGenerator()
	data := TemplateData{
		Manifest:  m,
		Timestamp: "2026-03-04T12:00:00Z",
		Version:   "0.1.0",
	}
	files, err := gen.Generate(context.Background(), data, "")
	require.NoError(t, err, "GoCLIGenerator.Generate must not error for a valid manifest")
	require.NotEmpty(t, files, "GoCLIGenerator.Generate must return at least one file")
	return files
}

func TestGoCLIGenerator_ImplementsGeneratorInterface(t *testing.T) {
	var _ Generator = (*GoCLIGenerator)(nil)
}

func TestGoCLIGenerator_Mode(t *testing.T) {
	gen := NewGoCLIGenerator()
	assert.Equal(t, "cli", gen.Mode())
}

func TestGoCLIGenerator_Target(t *testing.T) {
	gen := NewGoCLIGenerator()
	assert.Equal(t, "go", gen.Target())
}

// ---------------------------------------------------------------------------
// AC-2: Go CLI generates valid project structure
// ---------------------------------------------------------------------------

func TestGoCLI_AC2_MainGoPresent(t *testing.T) {
	files := generateCLI(t, manifestTwoToolsMixed())
	// cmd/{name}/main.go where name is the toolkit name
	requireFile(t, files, "cmd/my-toolkit/main.go")
}

func TestGoCLI_AC2_MainGoContainsPackageMain(t *testing.T) {
	files := generateCLI(t, manifestTwoToolsMixed())
	content := fileContent(t, files, "cmd/my-toolkit/main.go")
	assert.Contains(t, content, "package main",
		"main.go must declare package main")
	assert.Contains(t, content, "func main()",
		"main.go must contain func main()")
}

func TestGoCLI_AC2_RootGoPresent(t *testing.T) {
	files := generateCLI(t, manifestTwoToolsMixed())
	requireFile(t, files, "internal/commands/root.go")
}

func TestGoCLI_AC2_PerToolCommandFilesPresent(t *testing.T) {
	files := generateCLI(t, manifestTwoToolsMixed())
	requireFile(t, files, "internal/commands/status.go")
	requireFile(t, files, "internal/commands/deploy.go")
}

func TestGoCLI_AC2_AuthResolverPresent_WhenTokenAuth(t *testing.T) {
	// One tool uses token auth, so resolver.go must be generated.
	files := generateCLI(t, manifestTwoToolsMixed())
	requireFile(t, files, "internal/auth/resolver.go")
}

func TestGoCLI_AC2_AuthResolverAbsent_WhenAllAuthNone(t *testing.T) {
	// All tools are auth:none, so no auth resolver should be generated.
	files := generateCLI(t, manifestAllAuthNone())
	assertNoFile(t, files, "internal/auth/resolver.go")
}

func TestGoCLI_AC2_GoModPresent(t *testing.T) {
	files := generateCLI(t, manifestTwoToolsMixed())
	requireFile(t, files, "go.mod")
}

func TestGoCLI_AC2_GoModContainsModulePath(t *testing.T) {
	files := generateCLI(t, manifestTwoToolsMixed())
	content := fileContent(t, files, "go.mod")
	// The module path should reference the toolkit name
	assert.Contains(t, content, "my-toolkit",
		"go.mod module path should reference the toolkit name")
	assert.Contains(t, content, "module",
		"go.mod must contain a module directive")
}

func TestGoCLI_AC2_GoModContainsGoDependencies(t *testing.T) {
	files := generateCLI(t, manifestTwoToolsMixed())
	content := fileContent(t, files, "go.mod")
	// Must declare a Go version
	assert.Contains(t, content, "go ",
		"go.mod must specify a Go version")
}

func TestGoCLI_AC2_GoModContainsCobraDependency(t *testing.T) {
	files := generateCLI(t, manifestTwoToolsMixed())
	content := fileContent(t, files, "go.mod")
	assert.Contains(t, content, "cobra",
		"go.mod must include cobra as a dependency")
}

func TestGoCLI_AC2_MakefilePresent(t *testing.T) {
	files := generateCLI(t, manifestTwoToolsMixed())
	requireFile(t, files, "Makefile")
}

func TestGoCLI_AC2_MakefileContainsBuildTarget(t *testing.T) {
	files := generateCLI(t, manifestTwoToolsMixed())
	content := fileContent(t, files, "Makefile")
	assert.Contains(t, content, "build",
		"Makefile must contain a build target")
}

func TestGoCLI_AC2_ReadmePresent(t *testing.T) {
	files := generateCLI(t, manifestTwoToolsMixed())
	requireFile(t, files, "README.md")
}

func TestGoCLI_AC2_ReadmeContainsToolkitName(t *testing.T) {
	files := generateCLI(t, manifestTwoToolsMixed())
	content := fileContent(t, files, "README.md")
	assert.Contains(t, content, "my-toolkit",
		"README should reference the toolkit name")
}

func TestGoCLI_AC2_NoLoginGoWithoutOAuth2(t *testing.T) {
	// Manifest has only none and token auth -- no login.go
	files := generateCLI(t, manifestTwoToolsMixed())
	assertNoFile(t, files, "internal/commands/login.go")
}

func TestGoCLI_AC2_CorrectFileCount(t *testing.T) {
	// With 2 tools (one auth:none, one auth:token), expected files:
	// cmd/my-toolkit/main.go
	// internal/commands/root.go
	// internal/commands/status.go
	// internal/commands/deploy.go
	// internal/auth/resolver.go
	// go.mod
	// Makefile
	// README.md
	// That's at least 8 files. There may be more (e.g., go.sum placeholder),
	// but we assert at least these 8.
	files := generateCLI(t, manifestTwoToolsMixed())
	paths := filePaths(files)
	expectedPaths := []string{
		"cmd/my-toolkit/main.go",
		"internal/commands/root.go",
		"internal/commands/status.go",
		"internal/commands/deploy.go",
		"internal/auth/resolver.go",
		"go.mod",
		"Makefile",
		"README.md",
	}
	for _, ep := range expectedPaths {
		assert.Contains(t, paths, ep, "expected file %q in output", ep)
	}
}

// ---------------------------------------------------------------------------
// AC-3: Go CLI generates login command for oauth2 tools
// ---------------------------------------------------------------------------

func TestGoCLI_AC3_LoginGoPresent_WhenOAuth2(t *testing.T) {
	files := generateCLI(t, manifestOAuth2())
	requireFile(t, files, "internal/commands/login.go")
}

func TestGoCLI_AC3_LoginGoContainsPKCE(t *testing.T) {
	files := generateCLI(t, manifestOAuth2())
	content := fileContent(t, files, "internal/commands/login.go")
	// PKCE flow requires a code verifier and code challenge
	contentLower := strings.ToLower(content)
	assert.True(t,
		strings.Contains(contentLower, "pkce") ||
			strings.Contains(contentLower, "code_verifier") ||
			strings.Contains(contentLower, "codeverifier") ||
			strings.Contains(contentLower, "code_challenge") ||
			strings.Contains(contentLower, "codechallenge") ||
			strings.Contains(contentLower, "s256"),
		"login.go must include PKCE flow scaffolding (code_verifier, code_challenge, or S256)")
}

func TestGoCLI_AC3_LoginGoContainsLoginCommand(t *testing.T) {
	files := generateCLI(t, manifestOAuth2())
	content := fileContent(t, files, "internal/commands/login.go")
	assert.Contains(t, content, "login",
		"login.go must define a login command")
}

func TestGoCLI_AC3_LoginGoNotPresent_WhenOnlyTokenAuth(t *testing.T) {
	// Token-only auth should not produce login.go
	m := manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:    "token-only",
			Version: "1.0.0",
		},
		Tools: []manifest.Tool{
			{
				Name:       "fetch",
				Entrypoint: "./fetch.sh",
				Auth:       &manifest.Auth{Type: "token", TokenEnv: "MY_TOKEN"},
			},
		},
	}
	files := generateCLI(t, m)
	assertNoFile(t, files, "internal/commands/login.go")
}

func TestGoCLI_AC3_LoginGoNotPresent_WhenAuthNone(t *testing.T) {
	files := generateCLI(t, manifestAllAuthNone())
	assertNoFile(t, files, "internal/commands/login.go")
}

func TestGoCLI_AC3_AuthResolverPresent_WhenOAuth2(t *testing.T) {
	// OAuth2 also needs auth infrastructure
	files := generateCLI(t, manifestOAuth2())
	requireFile(t, files, "internal/auth/resolver.go")
}

func TestGoCLI_AC3_OAuthManifestIncludesProviderURL(t *testing.T) {
	files := generateCLI(t, manifestOAuth2())
	content := fileContent(t, files, "internal/commands/login.go")
	// The login command should reference the provider URL mechanism
	// (either directly or via a config pattern)
	assert.True(t,
		strings.Contains(content, "provider") ||
			strings.Contains(content, "Provider") ||
			strings.Contains(content, "issuer") ||
			strings.Contains(content, "Issuer") ||
			strings.Contains(content, "oauth") ||
			strings.Contains(content, "OAuth"),
		"login.go must reference the OAuth provider or issuer")
}

// ---------------------------------------------------------------------------
// AC-4: Go CLI list and describe subcommands
// ---------------------------------------------------------------------------

func TestGoCLI_AC4_RootGoContainsListSubcommand(t *testing.T) {
	files := generateCLI(t, manifestTwoToolsMixed())
	content := fileContent(t, files, "internal/commands/root.go")
	assert.Contains(t, content, "list",
		"root.go must define a 'list' subcommand")
}

func TestGoCLI_AC4_RootGoListSupportsJSON(t *testing.T) {
	files := generateCLI(t, manifestTwoToolsMixed())
	content := fileContent(t, files, "internal/commands/root.go")
	// Must support --json flag for list output
	assert.Contains(t, content, "json",
		"root.go list subcommand must support JSON output (--json flag)")
}

func TestGoCLI_AC4_RootGoContainsDescribeSubcommand(t *testing.T) {
	files := generateCLI(t, manifestTwoToolsMixed())
	content := fileContent(t, files, "internal/commands/root.go")
	assert.Contains(t, content, "describe",
		"root.go must define a 'describe' subcommand")
}

func TestGoCLI_AC4_RootGoDescribeReferencesSchema(t *testing.T) {
	files := generateCLI(t, manifestTwoToolsMixed())
	content := fileContent(t, files, "internal/commands/root.go")
	// The describe command should reference JSON Schema concepts
	contentLower := strings.ToLower(content)
	assert.True(t,
		strings.Contains(contentLower, "schema") ||
			strings.Contains(contentLower, "json"),
		"root.go describe subcommand must reference JSON Schema output")
}

func TestGoCLI_AC4_RootGoContainsToolNamesForList(t *testing.T) {
	files := generateCLI(t, manifestTwoToolsMixed())
	content := fileContent(t, files, "internal/commands/root.go")
	// The root.go must know about the tool names to list them
	assert.Contains(t, content, "status",
		"root.go must reference tool name 'status' for list output")
	assert.Contains(t, content, "deploy",
		"root.go must reference tool name 'deploy' for list output")
}

func TestGoCLI_AC4_RootGoContainsToolDescriptions(t *testing.T) {
	files := generateCLI(t, manifestTwoToolsMixed())
	content := fileContent(t, files, "internal/commands/root.go")
	assert.Contains(t, content, "Check service status",
		"root.go must include tool descriptions for list output")
	assert.Contains(t, content, "Deploy the service",
		"root.go must include tool descriptions for list output")
}

// ---------------------------------------------------------------------------
// AC-5: Go CLI per-tool subcommands map args and flags
// ---------------------------------------------------------------------------

func TestGoCLI_AC5_ToolCommandContainsToolDescription(t *testing.T) {
	files := generateCLI(t, manifestWithArgsAndFlags())
	content := fileContent(t, files, "internal/commands/upload.go")
	assert.Contains(t, content, "Upload a file to the server",
		"tool command file must include the tool description for Cobra help text")
}

func TestGoCLI_AC5_ToolCommandMapsPositionalArgs(t *testing.T) {
	files := generateCLI(t, manifestWithArgsAndFlags())
	content := fileContent(t, files, "internal/commands/upload.go")
	// Args should be referenced by name
	assert.Contains(t, content, "source",
		"tool command must reference positional arg 'source'")
	assert.Contains(t, content, "destination",
		"tool command must reference positional arg 'destination'")
}

func TestGoCLI_AC5_ToolCommandMapsFlags(t *testing.T) {
	files := generateCLI(t, manifestWithArgsAndFlags())
	content := fileContent(t, files, "internal/commands/upload.go")
	// Each flag should be defined
	assert.Contains(t, content, "verbose",
		"tool command must define flag 'verbose'")
	assert.Contains(t, content, "retries",
		"tool command must define flag 'retries'")
	assert.Contains(t, content, "timeout",
		"tool command must define flag 'timeout'")
	assert.Contains(t, content, "format",
		"tool command must define flag 'format'")
}

func TestGoCLI_AC5_ToolCommandFlagTypes(t *testing.T) {
	files := generateCLI(t, manifestWithArgsAndFlags())
	content := fileContent(t, files, "internal/commands/upload.go")
	// Each flag type must use the correct Cobra registration function.
	assert.Contains(t, content, "BoolVar",
		"verbose (bool) flag must use BoolVar registration")
	assert.Contains(t, content, "IntVar",
		"retries (int) flag must use IntVar registration")
	assert.Contains(t, content, "Float64Var",
		"timeout (float) flag must use Float64Var registration")
	assert.Contains(t, content, "StringVar",
		"format (string) flag must use StringVar registration")
}

func TestGoCLI_AC5_ToolCommandRequiredFlagEnforced(t *testing.T) {
	files := generateCLI(t, manifestWithArgsAndFlags())
	content := fileContent(t, files, "internal/commands/upload.go")
	// The "format" flag is required. Cobra enforces this via MarkFlagRequired
	// or equivalent mechanism.
	contentLower := strings.ToLower(content)
	assert.True(t,
		strings.Contains(contentLower, "required") ||
			strings.Contains(content, "MarkFlagRequired"),
		"tool command must enforce required flag 'format' via MarkFlagRequired or equivalent")
}

func TestGoCLI_AC5_ToolCommandEnumValues(t *testing.T) {
	files := generateCLI(t, manifestWithArgsAndFlags())
	content := fileContent(t, files, "internal/commands/upload.go")
	// Enum values for "format": json, yaml, csv
	assert.Contains(t, content, "json",
		"tool command must reference enum value 'json'")
	assert.Contains(t, content, "yaml",
		"tool command must reference enum value 'yaml'")
	assert.Contains(t, content, "csv",
		"tool command must reference enum value 'csv'")
}

func TestGoCLI_AC5_ToolCommandDefaultValues(t *testing.T) {
	files := generateCLI(t, manifestWithArgsAndFlags())
	content := fileContent(t, files, "internal/commands/upload.go")
	// Default for retries is 3 — assert the IntVar registration includes the default.
	// Pattern: IntVar(&..., "retries", 3, ...)
	assert.Contains(t, content, `"retries", 3`,
		"tool command must include default value 3 for retries flag in IntVar registration")
	// Default for timeout is 30 (float)
	assert.Contains(t, content, `"timeout", 30`,
		"tool command must include default value 30 for timeout flag in Float64Var registration")
}

func TestGoCLI_AC5_ToolCommandFlagDescriptions(t *testing.T) {
	files := generateCLI(t, manifestWithArgsAndFlags())
	content := fileContent(t, files, "internal/commands/upload.go")
	assert.Contains(t, content, "Enable verbose output",
		"tool command must include flag description for verbose")
	assert.Contains(t, content, "Number of retries",
		"tool command must include flag description for retries")
}

func TestGoCLI_AC5_ToolCommandCobraUsage(t *testing.T) {
	// The tool command file must import cobra or use cobra-like patterns
	files := generateCLI(t, manifestWithArgsAndFlags())
	content := fileContent(t, files, "internal/commands/upload.go")
	assert.True(t,
		strings.Contains(content, "cobra") || strings.Contains(content, "Cobra"),
		"tool command must use cobra for command definition")
}

func TestGoCLI_AC5_EachToolGetsOwnFile(t *testing.T) {
	files := generateCLI(t, manifestTwoToolsMixed())
	// Each tool gets its own file, not all tools in one file
	statusFile := requireFile(t, files, "internal/commands/status.go")
	deployFile := requireFile(t, files, "internal/commands/deploy.go")

	statusContent := string(statusFile.Content)
	deployContent := string(deployFile.Content)

	// status.go should reference "status" but not "deploy"
	assert.Contains(t, statusContent, "status",
		"status.go must reference tool name 'status'")
	// deploy.go should reference "deploy" but not just be a copy of status.go
	assert.Contains(t, deployContent, "deploy",
		"deploy.go must reference tool name 'deploy'")
	// They should not be identical (catches a lazy implementation that copies the same content)
	assert.NotEqual(t, statusContent, deployContent,
		"per-tool command files must not be identical")
}

// ---------------------------------------------------------------------------
// AC-11: Type mapping (table-driven as required by Constitution rule 9)
// ---------------------------------------------------------------------------

func TestGoCLI_AC11_TypeMapping(t *testing.T) {
	tests := []struct {
		name          string
		manifestType  string
		wantCobraFunc string // Cobra flag registration function (StringVar, IntVar, etc.)
	}{
		{name: "string maps to StringVar", manifestType: "string", wantCobraFunc: "StringVar"},
		{name: "int maps to IntVar", manifestType: "int", wantCobraFunc: "IntVar"},
		{name: "float maps to Float64Var", manifestType: "float", wantCobraFunc: "Float64Var"},
		{name: "bool maps to BoolVar", manifestType: "bool", wantCobraFunc: "BoolVar"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := manifest.Toolkit{
				APIVersion: "toolwright/v1",
				Kind:       "Toolkit",
				Metadata: manifest.Metadata{
					Name:    "typemap-toolkit",
					Version: "1.0.0",
				},
				Tools: []manifest.Tool{
					{
						Name:       "test-tool",
						Entrypoint: "./test.sh",
						Auth:       &manifest.Auth{Type: "none"},
						Flags: []manifest.Flag{
							{
								Name:        "myflag",
								Type:        tc.manifestType,
								Required:    false,
								Description: "test flag",
							},
						},
					},
				},
			}

			files := generateCLI(t, m)
			content := fileContent(t, files, "internal/commands/test-tool.go")
			assert.Contains(t, content, tc.wantCobraFunc,
				"manifest type %q must use Cobra function %q in generated code",
				tc.manifestType, tc.wantCobraFunc)
		})
	}
}

func TestGoCLI_AC11_TypeMappingArgs(t *testing.T) {
	// Also test type mapping for args (not just flags)
	tests := []struct {
		name         string
		manifestType string
		wantGoType   string
	}{
		{name: "string arg maps to string", manifestType: "string", wantGoType: "string"},
		{name: "int arg maps to int", manifestType: "int", wantGoType: "int"},
		{name: "float arg maps to float64", manifestType: "float", wantGoType: "float64"},
		{name: "bool arg maps to bool", manifestType: "bool", wantGoType: "bool"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := manifest.Toolkit{
				APIVersion: "toolwright/v1",
				Kind:       "Toolkit",
				Metadata: manifest.Metadata{
					Name:    "typemap-arg-toolkit",
					Version: "1.0.0",
				},
				Tools: []manifest.Tool{
					{
						Name:       "argtest",
						Entrypoint: "./test.sh",
						Auth:       &manifest.Auth{Type: "none"},
						Args: []manifest.Arg{
							{
								Name:        "myarg",
								Type:        tc.manifestType,
								Required:    true,
								Description: "test arg",
							},
						},
					},
				},
			}

			files := generateCLI(t, m)
			content := fileContent(t, files, "internal/commands/argtest.go")
			assert.Contains(t, content, tc.wantGoType,
				"manifest arg type %q must map to Go type %q in generated code",
				tc.manifestType, tc.wantGoType)
		})
	}
}

// ---------------------------------------------------------------------------
// AC-13: No secrets in generated code
// ---------------------------------------------------------------------------

func TestGoCLI_AC13_NoLiteralTokenValues(t *testing.T) {
	m := manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:    "secret-test",
			Version: "1.0.0",
		},
		Tools: []manifest.Tool{
			{
				Name:       "secured",
				Entrypoint: "./secured.sh",
				Auth: &manifest.Auth{
					Type:        "token",
					TokenEnv:    "MY_SECRET_TOKEN",
					TokenFlag:   "--token",
					TokenHeader: "Authorization",
				},
			},
		},
	}

	files := generateCLI(t, m)

	// Scan ALL generated files for secret-like patterns
	secretPatterns := []string{
		"sk-",           // Common API key prefix
		"ghp_",          // GitHub personal access token prefix
		"Bearer ",       // Literal bearer token with space
		"\"Bearer\"",    // Literal bearer string in quotes
		"password:",     // Hardcoded password
		"AKIA",          // AWS access key prefix
		"client_secret", // OAuth client secret
		"private_key",   // Private key reference
	}

	for _, f := range files {
		content := string(f.Content)
		for _, pattern := range secretPatterns {
			assert.NotContainsf(t, content, pattern,
				"file %q must not contain secret-like pattern %q",
				f.Path, pattern)
		}
	}
}

func TestGoCLI_AC13_AuthReferencesEnvVarsNotLiterals(t *testing.T) {
	m := manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:    "envref-test",
			Version: "1.0.0",
		},
		Tools: []manifest.Tool{
			{
				Name:       "fetch",
				Entrypoint: "./fetch.sh",
				Auth: &manifest.Auth{
					Type:        "token",
					TokenEnv:    "FETCH_API_TOKEN",
					TokenFlag:   "--token",
					TokenHeader: "Authorization",
				},
			},
		},
	}

	files := generateCLI(t, m)

	// The auth resolver or tool command should reference the env var name
	// as a string (for os.Getenv or similar), not a literal token value.
	var foundEnvRef bool
	for _, f := range files {
		content := string(f.Content)
		if strings.Contains(content, "FETCH_API_TOKEN") {
			foundEnvRef = true
			// It should reference the env var name, not a value
			assert.True(t,
				strings.Contains(content, "os.Getenv") ||
					strings.Contains(content, "Getenv") ||
					strings.Contains(content, "LookupEnv") ||
					strings.Contains(content, "env") ||
					strings.Contains(content, "Env"),
				"file %q references env var name but must use os.Getenv or similar, not a literal value", f.Path)
			break
		}
	}
	assert.True(t, foundEnvRef,
		"at least one generated file must reference the token env var name 'FETCH_API_TOKEN'")
}

func TestGoCLI_AC13_AuthReferencesTokenFlag(t *testing.T) {
	m := manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:    "flagref-test",
			Version: "1.0.0",
		},
		Tools: []manifest.Tool{
			{
				Name:       "push",
				Entrypoint: "./push.sh",
				Auth: &manifest.Auth{
					Type:      "token",
					TokenEnv:  "PUSH_TOKEN",
					TokenFlag: "--token",
				},
			},
		},
	}

	files := generateCLI(t, m)

	// The generated code should define a --token flag (the name from TokenFlag)
	var foundTokenFlag bool
	for _, f := range files {
		content := string(f.Content)
		if strings.Contains(content, "token") && strings.Contains(content, "flag") ||
			strings.Contains(content, "Token") && strings.Contains(content, "Flag") ||
			strings.Contains(content, "StringVar") && strings.Contains(content, "token") {
			foundTokenFlag = true
			break
		}
	}
	assert.True(t, foundTokenFlag,
		"generated code must define a token flag for auth resolution")
}

// ---------------------------------------------------------------------------
// AC-15: Generated code handles tools with no auth
// ---------------------------------------------------------------------------

func TestGoCLI_AC15_NoAuthCodeForNoneAuthTool(t *testing.T) {
	files := generateCLI(t, manifestTwoToolsMixed())
	content := fileContent(t, files, "internal/commands/status.go")
	// The status tool has auth:none, so its command file should not contain
	// auth-related code (token resolution, auth headers, etc.)
	contentLower := strings.ToLower(content)
	assert.False(t,
		strings.Contains(contentLower, "resolvetoken") ||
			strings.Contains(contentLower, "resolve_token") ||
			strings.Contains(contentLower, "authorization") ||
			strings.Contains(contentLower, "bearer"),
		"status.go (auth:none) must not contain token resolution or auth header code")
}

func TestGoCLI_AC15_AuthCodePresentForTokenAuthTool(t *testing.T) {
	files := generateCLI(t, manifestTwoToolsMixed())
	content := fileContent(t, files, "internal/commands/deploy.go")
	// The deploy tool has auth:token, so its command must reference auth/token
	contentLower := strings.ToLower(content)
	assert.True(t,
		strings.Contains(contentLower, "token") ||
			strings.Contains(contentLower, "auth"),
		"deploy.go (auth:token) must contain auth/token-related code")
}

func TestGoCLI_AC15_AllNoneAuth_NoAuthImports(t *testing.T) {
	files := generateCLI(t, manifestAllAuthNone())
	// When all tools are auth:none, per-tool command files should not import
	// the auth package
	for _, f := range files {
		if strings.HasPrefix(f.Path, "internal/commands/") && f.Path != "internal/commands/root.go" {
			content := string(f.Content)
			assert.NotContainsf(t, content, "internal/auth",
				"file %q should not import auth package when all tools are auth:none", f.Path)
		}
	}
}

func TestGoCLI_AC15_MixedAuth_OnlyAuthToolHasAuthCode(t *testing.T) {
	files := generateCLI(t, manifestTwoToolsMixed())

	statusContent := fileContent(t, files, "internal/commands/status.go")
	deployContent := fileContent(t, files, "internal/commands/deploy.go")

	// deploy (token) should have more auth-related content than status (none)
	statusAuthRefs := countAuthReferences(statusContent)
	deployAuthRefs := countAuthReferences(deployContent)

	assert.Greater(t, deployAuthRefs, statusAuthRefs,
		"deploy.go (auth:token) must have more auth references than status.go (auth:none): deploy=%d, status=%d",
		deployAuthRefs, statusAuthRefs)
}

// countAuthReferences counts occurrences of auth-related terms in content.
func countAuthReferences(content string) int {
	lower := strings.ToLower(content)
	terms := []string{"token", "auth", "bearer", "credential", "resolve"}
	count := 0
	for _, term := range terms {
		count += strings.Count(lower, term)
	}
	return count
}

func TestGoCLI_AC15_InheritedAuth_ToolGetsAuthCode(t *testing.T) {
	// When toolkit-level auth is token and tool has no override, the tool
	// should still get auth code via ResolvedAuth.
	files := generateCLI(t, manifestInheritedAuth())

	// The auth resolver should be generated because toolkit-level auth is token
	requireFile(t, files, "internal/auth/resolver.go")

	// The tool's command file should reference auth
	content := fileContent(t, files, "internal/commands/list.go")
	contentLower := strings.ToLower(content)
	assert.True(t,
		strings.Contains(contentLower, "token") ||
			strings.Contains(contentLower, "auth"),
		"list.go must have auth code when toolkit-level auth is token")
}

// ---------------------------------------------------------------------------
// Edge cases and adversarial tests
// ---------------------------------------------------------------------------

func TestGoCLI_SingleToolManifest(t *testing.T) {
	m := manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:    "single-tool",
			Version: "1.0.0",
		},
		Tools: []manifest.Tool{
			{
				Name:       "only",
				Entrypoint: "./only.sh",
				Auth:       &manifest.Auth{Type: "none"},
			},
		},
	}
	files := generateCLI(t, m)
	requireFile(t, files, "cmd/single-tool/main.go")
	requireFile(t, files, "internal/commands/root.go")
	requireFile(t, files, "internal/commands/only.go")
	requireFile(t, files, "go.mod")
}

func TestGoCLI_ManyTools(t *testing.T) {
	// 5 tools to ensure the generator handles more than 2
	tools := make([]manifest.Tool, 5)
	for i := range tools {
		name := []string{"alpha", "bravo", "charlie", "delta", "echo"}[i]
		tools[i] = manifest.Tool{
			Name:        name,
			Description: "Tool " + name,
			Entrypoint:  "./" + name + ".sh",
			Auth:        &manifest.Auth{Type: "none"},
		}
	}

	m := manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:    "many-tools",
			Version: "1.0.0",
		},
		Tools: tools,
	}

	files := generateCLI(t, m)
	for _, name := range []string{"alpha", "bravo", "charlie", "delta", "echo"} {
		requireFile(t, files, "internal/commands/"+name+".go")
	}
}

func TestGoCLI_ToolNameWithHyphens(t *testing.T) {
	m := manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:    "hyphen-toolkit",
			Version: "1.0.0",
		},
		Tools: []manifest.Tool{
			{
				Name:       "my-tool",
				Entrypoint: "./my-tool.sh",
				Auth:       &manifest.Auth{Type: "none"},
			},
		},
	}

	files := generateCLI(t, m)
	// Tool file should be named after the tool
	requireFile(t, files, "internal/commands/my-tool.go")
}

func TestGoCLI_ToolWithNoArgs_NoFlags(t *testing.T) {
	m := manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:    "bare-tool",
			Version: "1.0.0",
		},
		Tools: []manifest.Tool{
			{
				Name:        "simple",
				Description: "A simple tool",
				Entrypoint:  "./simple.sh",
				Auth:        &manifest.Auth{Type: "none"},
			},
		},
	}

	files := generateCLI(t, m)
	content := fileContent(t, files, "internal/commands/simple.go")
	// Should still generate a valid command file even with no args/flags
	assert.Contains(t, content, "simple",
		"tool command file must reference the tool name even with no args/flags")
	assert.Contains(t, content, "package",
		"tool command file must be valid Go (contain package declaration)")
}

func TestGoCLI_OAuth2WithMultipleTools_MixedAuth(t *testing.T) {
	// One oauth2 tool, one none tool. Should produce login.go AND per-tool files,
	// with auth only on the oauth2 tool.
	m := manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:    "mixed-oauth",
			Version: "1.0.0",
		},
		Tools: []manifest.Tool{
			{
				Name:       "public",
				Entrypoint: "./public.sh",
				Auth:       &manifest.Auth{Type: "none"},
			},
			{
				Name:       "private",
				Entrypoint: "./private.sh",
				Auth: &manifest.Auth{
					Type:        "oauth2",
					ProviderURL: "https://auth.example.com",
					Scopes:      []string{"api"},
				},
			},
		},
	}

	files := generateCLI(t, m)
	// login.go must be present because of oauth2 tool
	requireFile(t, files, "internal/commands/login.go")
	// auth resolver must be present
	requireFile(t, files, "internal/auth/resolver.go")
	// Both tool files present
	requireFile(t, files, "internal/commands/public.go")
	requireFile(t, files, "internal/commands/private.go")

	// public (none) should not have auth code
	publicContent := fileContent(t, files, "internal/commands/public.go")
	publicLower := strings.ToLower(publicContent)
	assert.False(t,
		strings.Contains(publicLower, "resolvetoken") ||
			strings.Contains(publicLower, "bearer") ||
			strings.Contains(publicLower, "authorization"),
		"public.go (auth:none) must not contain auth resolution code even when sibling uses oauth2")
}

func TestGoCLI_GeneratedGoFilesHavePackageDeclaration(t *testing.T) {
	files := generateCLI(t, manifestTwoToolsMixed())
	for _, f := range files {
		if strings.HasSuffix(f.Path, ".go") {
			content := string(f.Content)
			assert.Containsf(t, content, "package ",
				"Go file %q must contain a package declaration", f.Path)
		}
	}
}

func TestGoCLI_CommandFilesUseCommandsPackage(t *testing.T) {
	files := generateCLI(t, manifestTwoToolsMixed())
	for _, f := range files {
		if strings.HasPrefix(f.Path, "internal/commands/") && strings.HasSuffix(f.Path, ".go") {
			content := string(f.Content)
			assert.Containsf(t, content, "package commands",
				"file %q under internal/commands/ must use package commands", f.Path)
		}
	}
}

func TestGoCLI_MainGoUsesPackageMain(t *testing.T) {
	files := generateCLI(t, manifestTwoToolsMixed())
	content := fileContent(t, files, "cmd/my-toolkit/main.go")
	assert.Contains(t, content, "package main")
}

func TestGoCLI_AuthResolverUsesAuthPackage(t *testing.T) {
	files := generateCLI(t, manifestTwoToolsMixed())
	content := fileContent(t, files, "internal/auth/resolver.go")
	assert.Contains(t, content, "package auth",
		"resolver.go must use package auth")
}

func TestGoCLI_ContextCancellationRespected(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	gen := NewGoCLIGenerator()
	data := TemplateData{
		Manifest:  manifestTwoToolsMixed(),
		Timestamp: "2026-03-04T12:00:00Z",
		Version:   "0.1.0",
	}
	_, err := gen.Generate(ctx, data, "")
	// Must return an error when context is cancelled.
	require.Error(t, err, "Generate must error when context is cancelled")
	assert.ErrorIs(t, err, context.Canceled,
		"error from cancelled context should wrap context.Canceled")
}

func TestGoCLI_NoDuplicateFilePaths(t *testing.T) {
	files := generateCLI(t, manifestTwoToolsMixed())
	seen := make(map[string]bool)
	for _, f := range files {
		assert.Falsef(t, seen[f.Path],
			"duplicate file path %q in generated output", f.Path)
		seen[f.Path] = true
	}
}

func TestGoCLI_NoEmptyFiles(t *testing.T) {
	files := generateCLI(t, manifestTwoToolsMixed())
	for _, f := range files {
		assert.NotEmptyf(t, f.Content,
			"generated file %q must not have empty content", f.Path)
	}
}

func TestGoCLI_FilePathsAreRelative(t *testing.T) {
	files := generateCLI(t, manifestTwoToolsMixed())
	for _, f := range files {
		assert.Falsef(t, strings.HasPrefix(f.Path, "/"),
			"generated file path %q must be relative, not absolute", f.Path)
	}
}

func TestGoCLI_EmptyToolsSlice(t *testing.T) {
	// Boundary case: manifest with zero tools should still produce structural
	// files (main.go, root.go, go.mod, Makefile, README) without panic.
	m := manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:        "empty-toolkit",
			Version:     "1.0.0",
			Description: "No tools",
		},
		Tools: []manifest.Tool{},
	}
	files := generateCLI(t, m)
	requireFile(t, files, "cmd/empty-toolkit/main.go")
	requireFile(t, files, "internal/commands/root.go")
	requireFile(t, files, "go.mod")
	requireFile(t, files, "Makefile")
	requireFile(t, files, "README.md")
	// No per-tool command files, no auth resolver, no login.go
	assertNoFile(t, files, "internal/auth/resolver.go")
	assertNoFile(t, files, "internal/commands/login.go")
}

// ---------------------------------------------------------------------------
// AC-7: goType maps array types
// ---------------------------------------------------------------------------

func TestGoType_ArrayTypes(t *testing.T) {
	tests := []struct {
		manifestType string
		wantGoType   string
	}{
		{manifestType: "string[]", wantGoType: "[]string"},
		{manifestType: "int[]", wantGoType: "[]int"},
		{manifestType: "float[]", wantGoType: "[]float64"},
		{manifestType: "bool[]", wantGoType: "[]bool"},
		// Scalar types still work.
		{manifestType: "string", wantGoType: "string"},
		{manifestType: "int", wantGoType: "int"},
		{manifestType: "float", wantGoType: "float64"},
		{manifestType: "bool", wantGoType: "bool"},
	}
	for _, tc := range tests {
		t.Run(tc.manifestType, func(t *testing.T) {
			assert.Equal(t, tc.wantGoType, goType(tc.manifestType))
		})
	}
}

// ---------------------------------------------------------------------------
// AC-8: Generated CLI registers array flags as StringArrayVar
// ---------------------------------------------------------------------------

func TestGoCLI_AC8_ArrayFlagsUseStringArrayVar(t *testing.T) {
	tests := []struct {
		name         string
		manifestType string
	}{
		{name: "string[] uses StringArrayVar", manifestType: "string[]"},
		{name: "int[] uses StringArrayVar", manifestType: "int[]"},
		{name: "float[] uses StringArrayVar", manifestType: "float[]"},
		{name: "bool[] uses StringArrayVar", manifestType: "bool[]"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := manifest.Toolkit{
				APIVersion: "toolwright/v1",
				Kind:       "Toolkit",
				Metadata: manifest.Metadata{
					Name:    "array-toolkit",
					Version: "1.0.0",
					Description: "Array flags toolkit",
				},
				Tools: []manifest.Tool{
					{
						Name:        "array-tool",
						Description: "A tool with array flags",
						Entrypoint:  "./array-tool.sh",
						Auth:        &manifest.Auth{Type: "none"},
						Flags: []manifest.Flag{
							{
								Name:        "items",
								Type:        tc.manifestType,
								Required:    false,
								Description: "list of items",
							},
						},
					},
				},
			}
			files := generateCLI(t, m)
			content := fileContent(t, files, "internal/commands/array-tool.go")
			assert.Contains(t, content, "StringArrayVar",
				"flag type %q must register with StringArrayVar", tc.manifestType)
		})
	}
}

func TestGoCLI_AC8_ArrayFlagVarDeclaredAsStringSlice(t *testing.T) {
	// The var declaration for an array flag must be []string (all array types
	// are stored as []string and parsed at runtime).
	m := manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:        "array-decl-toolkit",
			Version:     "1.0.0",
			Description: "Array declaration toolkit",
		},
		Tools: []manifest.Tool{
			{
				Name:        "multi",
				Description: "Multi-value tool",
				Entrypoint:  "./multi.sh",
				Auth:        &manifest.Auth{Type: "none"},
				Flags: []manifest.Flag{
					{Name: "tags", Type: "string[]", Description: "tag list"},
					{Name: "counts", Type: "int[]", Description: "count list"},
				},
			},
		},
	}
	files := generateCLI(t, m)
	content := fileContent(t, files, "internal/commands/multi.go")
	// All array flags are stored as []string.
	assert.Contains(t, content, "[]string", "array flag vars must be declared as []string")
}

// ---------------------------------------------------------------------------
// AC-9: Generated CLI handles array defaults
// ---------------------------------------------------------------------------

func TestGoCLI_AC9_StringArrayDefaultValues(t *testing.T) {
	// A string[] flag with a default ["a", "b"] must produce []string{"a", "b"}.
	m := manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:        "strarray-default",
			Version:     "1.0.0",
			Description: "String array default toolkit",
		},
		Tools: []manifest.Tool{
			{
				Name:        "strarray",
				Description: "String array tool",
				Entrypoint:  "./strarray.sh",
				Auth:        &manifest.Auth{Type: "none"},
				Flags: []manifest.Flag{
					{
						Name:        "tags",
						Type:        "string[]",
						Default:     []interface{}{"a", "b"},
						Description: "tag list",
					},
				},
			},
		},
	}
	files := generateCLI(t, m)
	content := fileContent(t, files, "internal/commands/strarray.go")
	assert.Contains(t, content, `[]string{"a", "b"}`,
		"string[] flag with default [\"a\", \"b\"] must generate []string{\"a\", \"b\"}")
}

func TestGoCLI_AC9_NonStringArrayDefaultIsEmpty(t *testing.T) {
	// Non-string array types with a default emit []string{} (parse at runtime).
	tests := []struct {
		manifestType string
		defaultVal   []interface{}
	}{
		{manifestType: "int[]", defaultVal: []interface{}{1, 2}},
		{manifestType: "float[]", defaultVal: []interface{}{1.5, 2.5}},
		{manifestType: "bool[]", defaultVal: []interface{}{true, false}},
	}
	for _, tc := range tests {
		t.Run(tc.manifestType, func(t *testing.T) {
			m := manifest.Toolkit{
				APIVersion: "toolwright/v1",
				Kind:       "Toolkit",
				Metadata: manifest.Metadata{
					Name:        "nonstr-default",
					Version:     "1.0.0",
					Description: "Non-string array default toolkit",
				},
				Tools: []manifest.Tool{
					{
						Name:        "nonstr",
						Description: "Non-string array tool",
						Entrypoint:  "./nonstr.sh",
						Auth:        &manifest.Auth{Type: "none"},
						Flags: []manifest.Flag{
							{
								Name:        "values",
								Type:        tc.manifestType,
								Default:     tc.defaultVal,
								Description: "value list",
							},
						},
					},
				},
			}
			files := generateCLI(t, m)
			content := fileContent(t, files, "internal/commands/nonstr.go")
			assert.Contains(t, content, "[]string{}",
				"non-string array type %q with default must generate []string{}", tc.manifestType)
		})
	}
}

func TestGoCLI_AC9_ArrayFlagNoDefaultIsNil(t *testing.T) {
	// An array flag with no default must not set a default (nil / omitted).
	m := manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:        "nodefault-toolkit",
			Version:     "1.0.0",
			Description: "No default toolkit",
		},
		Tools: []manifest.Tool{
			{
				Name:        "nodefault",
				Description: "No default tool",
				Entrypoint:  "./nodefault.sh",
				Auth:        &manifest.Auth{Type: "none"},
				Flags: []manifest.Flag{
					{
						Name:        "tags",
						Type:        "string[]",
						Description: "tag list",
						// Default is nil / omitted.
					},
				},
			},
		},
	}
	files := generateCLI(t, m)
	content := fileContent(t, files, "internal/commands/nodefault.go")
	// No literal default should appear for this flag.
	assert.NotContains(t, content, `[]string{"`,
		"array flag with no default must not emit a non-empty default literal")
}

// ---------------------------------------------------------------------------
// AC-10: Generated CLI parses non-string array elements
// ---------------------------------------------------------------------------

func TestGoCLI_AC10_IntArrayParsingInRunE(t *testing.T) {
	m := manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:        "int-array-toolkit",
			Version:     "1.0.0",
			Description: "Int array toolkit",
		},
		Tools: []manifest.Tool{
			{
				Name:        "counter",
				Description: "Counter tool",
				Entrypoint:  "./counter.sh",
				Auth:        &manifest.Auth{Type: "none"},
				Flags: []manifest.Flag{
					{
						Name:        "counts",
						Type:        "int[]",
						Description: "list of counts",
					},
				},
			},
		},
	}
	files := generateCLI(t, m)
	content := fileContent(t, files, "internal/commands/counter.go")
	// RunE must contain parsing code for int[] — strconv.Atoi per element.
	assert.Contains(t, content, "strconv",
		"int[] flag must import strconv for runtime parsing")
	assert.Contains(t, content, "Atoi",
		"int[] flag RunE must use strconv.Atoi to parse elements")
	// User-friendly error message.
	assert.Contains(t, content, "not a valid int",
		"int[] parsing error must say 'not a valid int'")
	assert.Contains(t, content, "--counts",
		"int[] parsing error must name the flag '--counts'")
}

func TestGoCLI_AC10_FloatArrayParsingInRunE(t *testing.T) {
	m := manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:        "float-array-toolkit",
			Version:     "1.0.0",
			Description: "Float array toolkit",
		},
		Tools: []manifest.Tool{
			{
				Name:        "floater",
				Description: "Floater tool",
				Entrypoint:  "./floater.sh",
				Auth:        &manifest.Auth{Type: "none"},
				Flags: []manifest.Flag{
					{
						Name:        "weights",
						Type:        "float[]",
						Description: "list of weights",
					},
				},
			},
		},
	}
	files := generateCLI(t, m)
	content := fileContent(t, files, "internal/commands/floater.go")
	assert.Contains(t, content, "strconv",
		"float[] flag must import strconv for runtime parsing")
	assert.Contains(t, content, "ParseFloat",
		"float[] flag RunE must use strconv.ParseFloat to parse elements")
	assert.Contains(t, content, "not a valid float",
		"float[] parsing error must say 'not a valid float'")
	assert.Contains(t, content, "--weights",
		"float[] parsing error must name the flag '--weights'")
}

func TestGoCLI_AC10_BoolArrayParsingInRunE(t *testing.T) {
	m := manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:        "bool-array-toolkit",
			Version:     "1.0.0",
			Description: "Bool array toolkit",
		},
		Tools: []manifest.Tool{
			{
				Name:        "toggler",
				Description: "Toggler tool",
				Entrypoint:  "./toggler.sh",
				Auth:        &manifest.Auth{Type: "none"},
				Flags: []manifest.Flag{
					{
						Name:        "flags",
						Type:        "bool[]",
						Description: "list of boolean flags",
					},
				},
			},
		},
	}
	files := generateCLI(t, m)
	content := fileContent(t, files, "internal/commands/toggler.go")
	assert.Contains(t, content, "strconv",
		"bool[] flag must import strconv for runtime parsing")
	assert.Contains(t, content, "ParseBool",
		"bool[] flag RunE must use strconv.ParseBool to parse elements")
	assert.Contains(t, content, "not a valid bool",
		"bool[] parsing error must say 'not a valid bool'")
	assert.Contains(t, content, "--flags",
		"bool[] parsing error must name the flag '--flags'")
}

func TestGoCLI_AC10_StringArrayNoParsingInRunE(t *testing.T) {
	// string[] must NOT generate strconv parsing — values are used directly.
	m := manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:        "strarray-noparse",
			Version:     "1.0.0",
			Description: "String array no-parse toolkit",
		},
		Tools: []manifest.Tool{
			{
				Name:        "strtool",
				Description: "String array tool",
				Entrypoint:  "./strtool.sh",
				Auth:        &manifest.Auth{Type: "none"},
				Flags: []manifest.Flag{
					{
						Name:        "names",
						Type:        "string[]",
						Description: "list of names",
					},
				},
			},
		},
	}
	files := generateCLI(t, m)
	content := fileContent(t, files, "internal/commands/strtool.go")
	// string[] should not produce strconv parsing (values used directly).
	assert.NotContains(t, content, "strconv",
		"string[] flag must not import strconv (no parsing needed)")
}

func TestGoCLI_AC10_EmptyArrayProducesNoParseError(t *testing.T) {
	// The generated parse loop must handle an empty slice without errors.
	// This is verified structurally: the loop must use a range over the slice,
	// which produces zero iterations for an empty slice.
	m := manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:        "empty-array-toolkit",
			Version:     "1.0.0",
			Description: "Empty array toolkit",
		},
		Tools: []manifest.Tool{
			{
				Name:        "empty-counter",
				Description: "Empty counter tool",
				Entrypoint:  "./counter.sh",
				Auth:        &manifest.Auth{Type: "none"},
				Flags: []manifest.Flag{
					{
						Name:        "counts",
						Type:        "int[]",
						Description: "list of counts",
					},
				},
			},
		},
	}
	files := generateCLI(t, m)
	content := fileContent(t, files, "internal/commands/empty-counter.go")
	// The loop must use range so it is a no-op for empty slices.
	assert.Contains(t, content, "range",
		"int[] parse loop must use range (handles empty slice correctly)")
}

func TestGoCLI_AC10_ArrayParseErrorFormat(t *testing.T) {
	// Error format: invalid value "abc" for element of --counts: not a valid int
	m := manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:        "errfmt-toolkit",
			Version:     "1.0.0",
			Description: "Error format toolkit",
		},
		Tools: []manifest.Tool{
			{
				Name:        "errfmt",
				Description: "Error format tool",
				Entrypoint:  "./errfmt.sh",
				Auth:        &manifest.Auth{Type: "none"},
				Flags: []manifest.Flag{
					{
						Name:        "counts",
						Type:        "int[]",
						Description: "list of counts",
					},
				},
			},
		},
	}
	files := generateCLI(t, m)
	content := fileContent(t, files, "internal/commands/errfmt.go")
	// The error must reference the flag name and "element of".
	assert.Contains(t, content, "element of",
		"array parse error must say 'element of'")
	assert.Contains(t, content, "invalid value",
		"array parse error must say 'invalid value'")
}
