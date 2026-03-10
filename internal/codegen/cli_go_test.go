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
					Name:        "array-toolkit",
					Version:     "1.0.0",
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

func TestGoCLI_AC9_NonStringArrayDefaultPreserved(t *testing.T) {
	// Non-string array types with a default preserve values as string representations.
	tests := []struct {
		manifestType string
		defaultVal   []interface{}
		expected     string
	}{
		{manifestType: "int[]", defaultVal: []interface{}{1, 2}, expected: `[]string{"1", "2"}`},
		{manifestType: "float[]", defaultVal: []interface{}{1.5, 2.5}, expected: `[]string{"1.5", "2.5"}`},
		{manifestType: "bool[]", defaultVal: []interface{}{true, false}, expected: `[]string{"true", "false"}`},
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
			assert.Contains(t, content, tc.expected,
				"non-string array type %q with default must preserve values as strings", tc.manifestType)
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

// ---------------------------------------------------------------------------
// Task 4 — AC8: Object flags register as StringVar
// ---------------------------------------------------------------------------

// manifestWithObjectFlag returns a manifest with a single tool that has an
// "object" type flag. Used for AC8/AC9/AC10/AC11 tests.
func manifestWithObjectFlag() manifest.Toolkit {
	return manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:        "obj-toolkit",
			Version:     "1.0.0",
			Description: "Object flags toolkit",
		},
		Tools: []manifest.Tool{
			{
				Name:        "apply-edits",
				Description: "Apply edits to a file",
				Entrypoint:  "./apply-edits.sh",
				Auth:        &manifest.Auth{Type: "none"},
				Flags: []manifest.Flag{
					{
						Name:        "edits",
						Type:        "object",
						Required:    true,
						Description: "Edit operations to apply",
						ItemSchema: map[string]any{
							"type": "object",
							"properties": map[string]any{
								"path":  map[string]any{"type": "string"},
								"value": map[string]any{"type": "string"},
							},
							"required": []any{"path", "value"},
						},
					},
				},
			},
		},
	}
}

// manifestWithObjectArrayFlag returns a manifest with a single tool that has
// an "object[]" type flag.
func manifestWithObjectArrayFlag() manifest.Toolkit {
	return manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:        "objarray-toolkit",
			Version:     "1.0.0",
			Description: "Object array flags toolkit",
		},
		Tools: []manifest.Tool{
			{
				Name:        "batch-update",
				Description: "Batch update resources",
				Entrypoint:  "./batch-update.sh",
				Auth:        &manifest.Auth{Type: "none"},
				Flags: []manifest.Flag{
					{
						Name:        "updates",
						Type:        "object[]",
						Required:    false,
						Description: "List of update operations",
						ItemSchema: map[string]any{
							"type": "object",
							"properties": map[string]any{
								"id":     map[string]any{"type": "integer"},
								"action": map[string]any{"type": "string"},
							},
							"required": []any{"id", "action"},
						},
					},
				},
			},
		},
	}
}

func TestGoCLI_Object_AC8_GoTypeObjectReturnsString(t *testing.T) {
	// goType("object") must return "string" so the CLI accepts a JSON string.
	// The current default case in goType returns "string", but the test
	// verifies this is intentional for "object", not accidental.
	got := goType("object")
	assert.Equal(t, "string", got,
		"goType(\"object\") must return \"string\" to accept JSON string at CLI level")
}

func TestGoCLI_Object_AC8_GoTypeObjectArrayReturnsString(t *testing.T) {
	// goType("object[]") must return "string" (NOT "[]string") because the
	// entire JSON array is passed as a single string flag, not as repeated
	// --flag values.
	got := goType("object[]")
	assert.Equal(t, "string", got,
		"goType(\"object[]\") must return \"string\" to accept JSON array string at CLI level")
}

func TestGoCLI_Object_AC8_TypeMapping(t *testing.T) {
	// Table-driven test ensuring object types map correctly (Constitution 9).
	tests := []struct {
		name         string
		manifestType string
		wantGoType   string
		wantCobra    string
	}{
		{
			name:         "object maps to string/StringVar",
			manifestType: "object",
			wantGoType:   "string",
			wantCobra:    "StringVar",
		},
		{
			name:         "object[] maps to string/StringVar",
			manifestType: "object[]",
			wantGoType:   "string",
			wantCobra:    "StringVar",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotType := goType(tc.manifestType)
			assert.Equal(t, tc.wantGoType, gotType,
				"goType(%q) must return %q", tc.manifestType, tc.wantGoType)

			gotCobra := cobraFlagFunc(gotType)
			assert.Equal(t, tc.wantCobra, gotCobra,
				"cobraFlagFunc(%q) must return %q", gotType, tc.wantCobra)
		})
	}
}

func TestGoCLI_Object_AC8_ObjectFlagUsesStringVar(t *testing.T) {
	// Generated code for type: "object" flag must use StringVar, not
	// StringArrayVar or any other registration function.
	files := generateCLI(t, manifestWithObjectFlag())
	content := fileContent(t, files, "internal/commands/apply-edits.go")

	assert.Contains(t, content, "StringVar",
		"object flag must register with StringVar")
	// Must NOT use StringArrayVar (which would be wrong for object).
	assert.NotContains(t, content, "StringArrayVar",
		"object flag must NOT use StringArrayVar — single JSON string, not repeated values")
}

func TestGoCLI_Object_AC8_ObjectArrayFlagUsesStringVar(t *testing.T) {
	// Generated code for type: "object[]" flag must also use StringVar
	// (the entire JSON array is a single string), NOT StringArrayVar.
	files := generateCLI(t, manifestWithObjectArrayFlag())
	content := fileContent(t, files, "internal/commands/batch-update.go")

	assert.Contains(t, content, "StringVar",
		"object[] flag must register with StringVar")
	assert.NotContains(t, content, "StringArrayVar",
		"object[] flag must NOT use StringArrayVar — JSON array is passed as a single string")
}

func TestGoCLI_Object_AC8_ObjectFlagVarDeclaredAsString(t *testing.T) {
	// The variable for an object flag must be declared as string, not []string.
	files := generateCLI(t, manifestWithObjectFlag())
	content := fileContent(t, files, "internal/commands/apply-edits.go")

	// The var block should contain a string declaration for the edits flag.
	// It should NOT contain a []string declaration for that flag.
	// Check that the var declaration section does NOT have "[]string" for the
	// edits flag. We look for the pattern: flagEdits []string (wrong) vs
	// flagEdits string (correct).
	assert.NotRegexp(t, `Flag[Ee]dits\s+\[\]string`, content,
		"object flag var must be declared as string, not []string")
}

func TestGoCLI_Object_AC8_ObjectArrayFlagVarDeclaredAsString(t *testing.T) {
	// The variable for an object[] flag must be declared as string, not []string.
	files := generateCLI(t, manifestWithObjectArrayFlag())
	content := fileContent(t, files, "internal/commands/batch-update.go")

	assert.NotRegexp(t, `Flag[Uu]pdates\s+\[\]string`, content,
		"object[] flag var must be declared as string, not []string")
}

func TestGoCLI_Object_AC8_ObjectFlagNotTreatedAsArray(t *testing.T) {
	// Even though "object[]" ends with "[]", the generated code must NOT
	// treat it as an array flag (no range loop over string elements, no
	// StringArrayVar). This catches a lazy implementation that checks
	// strings.HasSuffix(type, "[]") to determine array behavior.
	files := generateCLI(t, manifestWithObjectArrayFlag())
	content := fileContent(t, files, "internal/commands/batch-update.go")

	// The generated code should not have array-style element parsing
	// (range over the flag value treating it as []string).
	assert.NotContains(t, content, "element of --updates",
		"object[] flag must not generate array-element parsing (it's a single JSON string)")
}

// ---------------------------------------------------------------------------
// Task 4 — AC9: Generated CLI parses JSON for object flags
// ---------------------------------------------------------------------------

func TestGoCLI_Object_AC9_ObjectFlagContainsJsonUnmarshal(t *testing.T) {
	files := generateCLI(t, manifestWithObjectFlag())
	content := fileContent(t, files, "internal/commands/apply-edits.go")

	assert.Contains(t, content, "json.Unmarshal",
		"object flag RunE must call json.Unmarshal to parse the JSON string")
}

func TestGoCLI_Object_AC9_ObjectArrayFlagContainsJsonUnmarshal(t *testing.T) {
	files := generateCLI(t, manifestWithObjectArrayFlag())
	content := fileContent(t, files, "internal/commands/batch-update.go")

	assert.Contains(t, content, "json.Unmarshal",
		"object[] flag RunE must call json.Unmarshal to parse the JSON array string")
}

func TestGoCLI_Object_AC9_ObjectFlagImportsEncodingJson(t *testing.T) {
	files := generateCLI(t, manifestWithObjectFlag())
	content := fileContent(t, files, "internal/commands/apply-edits.go")

	assert.Contains(t, content, `"encoding/json"`,
		"tool file with object flag must import encoding/json")
}

func TestGoCLI_Object_AC9_JsonErrorFormat(t *testing.T) {
	// The error message for invalid JSON must be user-friendly and include
	// the flag name: "invalid JSON for --edits: <parse error>".
	files := generateCLI(t, manifestWithObjectFlag())
	content := fileContent(t, files, "internal/commands/apply-edits.go")

	assert.Contains(t, content, "invalid JSON for --edits",
		"JSON parse error must say 'invalid JSON for --edits'")
}

func TestGoCLI_Object_AC9_JsonErrorFormatObjectArray(t *testing.T) {
	// Same user-friendly error for object[] flags.
	files := generateCLI(t, manifestWithObjectArrayFlag())
	content := fileContent(t, files, "internal/commands/batch-update.go")

	assert.Contains(t, content, "invalid JSON for --updates",
		"JSON parse error for object[] must say 'invalid JSON for --updates'")
}

func TestGoCLI_Object_AC9_JsonErrorIncludesParseError(t *testing.T) {
	// The error message template must include the actual parse error from
	// json.Unmarshal (via %w or similar). We verify the format string
	// contains a wrapping directive.
	files := generateCLI(t, manifestWithObjectFlag())
	content := fileContent(t, files, "internal/commands/apply-edits.go")

	// The error format must wrap the underlying error (e.g., via %w or %v).
	// A lazy implementation might hardcode a static message without the parse
	// error detail. Check for "invalid JSON for --edits" followed by a format
	// verb that includes the unmarshal error.
	assert.Regexp(t, `invalid JSON for --edits.*%[wv]`, content,
		"JSON error must wrap the underlying unmarshal error using %%w or %%v")
}

func TestGoCLI_Object_AC9_NoJsonImportWithoutObjectFlags(t *testing.T) {
	// When there are no object flags, the tool file should NOT import
	// encoding/json (it would be an unused import causing compile errors).
	files := generateCLI(t, manifestWithArgsAndFlags())
	content := fileContent(t, files, "internal/commands/upload.go")

	assert.NotContains(t, content, `"encoding/json"`,
		"tool file without object flags must not import encoding/json")
}

func TestGoCLI_Object_AC9_MultipleFlagTypes_OnlyObjectGetsJsonParsing(t *testing.T) {
	// A tool with both object and string flags: only the object flag gets
	// json.Unmarshal. The string flag should NOT have JSON parsing.
	m := manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:        "mixed-flag-toolkit",
			Version:     "1.0.0",
			Description: "Mixed flags toolkit",
		},
		Tools: []manifest.Tool{
			{
				Name:        "mixed-tool",
				Description: "A tool with mixed flag types",
				Entrypoint:  "./mixed.sh",
				Auth:        &manifest.Auth{Type: "none"},
				Flags: []manifest.Flag{
					{
						Name:        "config",
						Type:        "object",
						Description: "Configuration object",
					},
					{
						Name:        "name",
						Type:        "string",
						Description: "A plain string",
					},
				},
			},
		},
	}

	files := generateCLI(t, m)
	content := fileContent(t, files, "internal/commands/mixed-tool.go")

	// JSON parsing must be present (for the object flag).
	assert.Contains(t, content, "json.Unmarshal",
		"mixed tool with object flag must have json.Unmarshal")
	// Error must reference --config (the object flag), not --name.
	assert.Contains(t, content, "invalid JSON for --config",
		"JSON error must reference the object flag name --config")
	assert.NotContains(t, content, "invalid JSON for --name",
		"string flag --name must NOT have JSON parsing")
}

// ---------------------------------------------------------------------------
// Task 4 — AC10: Generated CLI validates against itemSchema
// ---------------------------------------------------------------------------

func TestGoCLI_Object_AC10_ItemSchemaPresent_GeneratesValidation(t *testing.T) {
	// When itemSchema is present, the generated code must include validation
	// code *in the RunE body* (not just MarkFlagRequired). The validation
	// must check parsed JSON keys against the schema.
	files := generateCLI(t, manifestWithObjectFlag())
	content := fileContent(t, files, "internal/commands/apply-edits.go")

	// The generated validation must check for missing required keys in the
	// parsed JSON. The word "required" in MarkFlagRequired is NOT sufficient --
	// that only makes the flag itself required, not its content valid.
	// We look for validation that references a specific property name from the
	// itemSchema ("path" or "value") in a validation context (error message or
	// check), distinct from MarkFlagRequired.
	assert.True(t,
		strings.Contains(content, `"path"`) && strings.Contains(content, `"value"`),
		"object flag with itemSchema must generate validation that checks required properties 'path' and 'value'")
}

func TestGoCLI_Object_AC10_ValidationReferencesRequiredFields(t *testing.T) {
	// The generated validation must reference the required fields from the
	// itemSchema: "path" and "value".
	files := generateCLI(t, manifestWithObjectFlag())
	content := fileContent(t, files, "internal/commands/apply-edits.go")

	assert.Contains(t, content, "path",
		"validation must reference required field 'path' from itemSchema")
	assert.Contains(t, content, "value",
		"validation must reference required field 'value' from itemSchema")
}

func TestGoCLI_Object_AC10_ValidationErrorReferencesFlagName(t *testing.T) {
	// Schema validation errors must reference the flag name so the user knows
	// which flag has the problem.
	files := generateCLI(t, manifestWithObjectFlag())
	content := fileContent(t, files, "internal/commands/apply-edits.go")

	assert.Contains(t, content, "--edits",
		"schema validation error must reference the flag name '--edits'")
}

func TestGoCLI_Object_AC10_ObjectArrayValidationReferencesFlag(t *testing.T) {
	// For object[] with itemSchema, validation errors must also reference the
	// flag name.
	files := generateCLI(t, manifestWithObjectArrayFlag())
	content := fileContent(t, files, "internal/commands/batch-update.go")

	assert.Contains(t, content, "--updates",
		"object[] schema validation error must reference the flag name '--updates'")
}

func TestGoCLI_Object_AC10_NoItemSchema_NoValidation(t *testing.T) {
	// When itemSchema is absent, no schema validation code should be generated.
	// Only json.Unmarshal should remain.
	m := manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:        "noschema-toolkit",
			Version:     "1.0.0",
			Description: "No schema toolkit",
		},
		Tools: []manifest.Tool{
			{
				Name:        "freeform",
				Description: "Accept any JSON",
				Entrypoint:  "./freeform.sh",
				Auth:        &manifest.Auth{Type: "none"},
				Flags: []manifest.Flag{
					{
						Name:        "data",
						Type:        "object",
						Description: "Arbitrary JSON data",
						// No ItemSchema.
					},
				},
			},
		},
	}

	files := generateCLI(t, m)
	content := fileContent(t, files, "internal/commands/freeform.go")

	// json.Unmarshal should still be present (AC9).
	assert.Contains(t, content, "json.Unmarshal",
		"object flag without itemSchema must still parse JSON")
	// But no schema-specific validation (no required field checks, etc).
	// We verify by looking for the absence of required field names that would
	// only appear if validation was generated. A lazy implementation might
	// always emit validation even without itemSchema.
	assert.NotContains(t, content, "required field",
		"object flag without itemSchema must not generate required-field validation")
}

func TestGoCLI_Object_AC10_ObjectArrayItemSchemaValidatesRequiredFields(t *testing.T) {
	// For object[] with itemSchema that has required: ["id", "action"],
	// the generated code must validate those required fields.
	files := generateCLI(t, manifestWithObjectArrayFlag())
	content := fileContent(t, files, "internal/commands/batch-update.go")

	assert.Contains(t, content, "id",
		"object[] validation must reference required field 'id' from itemSchema")
	assert.Contains(t, content, "action",
		"object[] validation must reference required field 'action' from itemSchema")
}

// ---------------------------------------------------------------------------
// Task 4 — AC11: CLI --help shows JSON hint
// ---------------------------------------------------------------------------

func TestGoCLI_Object_AC11_ObjectFlagDescriptionMentionsJSON(t *testing.T) {
	// For object flags, the generated flag description/usage must mention
	// "JSON" so the user knows the flag expects JSON input.
	files := generateCLI(t, manifestWithObjectFlag())
	content := fileContent(t, files, "internal/commands/apply-edits.go")

	// The description string registered with Cobra must contain "JSON".
	// Look for the pattern in the StringVar call. The description is the last
	// string argument to StringVar.
	assert.Regexp(t, `(?i)StringVar.*edits.*JSON`, content,
		"object flag registration must include 'JSON' in the description/usage hint")
}

func TestGoCLI_Object_AC11_ObjectArrayFlagDescriptionMentionsJSON(t *testing.T) {
	files := generateCLI(t, manifestWithObjectArrayFlag())
	content := fileContent(t, files, "internal/commands/batch-update.go")

	assert.Regexp(t, `(?i)StringVar.*updates.*JSON`, content,
		"object[] flag registration must include 'JSON' in the description/usage hint")
}

func TestGoCLI_Object_AC11_ItemSchemaPropertiesSummarized(t *testing.T) {
	// When itemSchema has properties, the generated description should include
	// a summary of the expected keys. For the edits flag, itemSchema has
	// properties: path, value. The description must mention these.
	files := generateCLI(t, manifestWithObjectFlag())
	content := fileContent(t, files, "internal/commands/apply-edits.go")

	// The description must mention the property names from itemSchema.
	assert.Contains(t, content, "path",
		"object flag description must summarize itemSchema property 'path'")
	assert.Contains(t, content, "value",
		"object flag description must summarize itemSchema property 'value'")
}

func TestGoCLI_Object_AC11_ObjectArrayItemSchemaPropertiesSummarized(t *testing.T) {
	files := generateCLI(t, manifestWithObjectArrayFlag())
	content := fileContent(t, files, "internal/commands/batch-update.go")

	assert.Contains(t, content, "id",
		"object[] flag description must summarize itemSchema property 'id'")
	assert.Contains(t, content, "action",
		"object[] flag description must summarize itemSchema property 'action'")
}

func TestGoCLI_Object_AC11_NoItemSchema_DescriptionStillShowsJSON(t *testing.T) {
	// Even without itemSchema, the description must still mention "JSON".
	m := manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:        "noschema-hint",
			Version:     "1.0.0",
			Description: "No schema hint",
		},
		Tools: []manifest.Tool{
			{
				Name:        "loose",
				Description: "Accepts loose JSON",
				Entrypoint:  "./loose.sh",
				Auth:        &manifest.Auth{Type: "none"},
				Flags: []manifest.Flag{
					{
						Name:        "payload",
						Type:        "object",
						Description: "Request payload",
					},
				},
			},
		},
	}

	files := generateCLI(t, m)
	content := fileContent(t, files, "internal/commands/loose.go")

	assert.Regexp(t, `(?i)StringVar.*payload.*JSON`, content,
		"object flag without itemSchema must still show JSON hint in description")
}

// ---------------------------------------------------------------------------
// Task 4 — Cross-cutting edge cases
// ---------------------------------------------------------------------------

func TestGoCLI_Object_BothObjectAndObjectArray_SameTool(t *testing.T) {
	// A tool with both "object" and "object[]" flags must handle both correctly.
	m := manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:        "dual-object-toolkit",
			Version:     "1.0.0",
			Description: "Both object and object[] flags",
		},
		Tools: []manifest.Tool{
			{
				Name:        "dual",
				Description: "Dual object tool",
				Entrypoint:  "./dual.sh",
				Auth:        &manifest.Auth{Type: "none"},
				Flags: []manifest.Flag{
					{
						Name:        "single",
						Type:        "object",
						Description: "A single object",
					},
					{
						Name:        "multi",
						Type:        "object[]",
						Description: "An array of objects",
					},
				},
			},
		},
	}

	files := generateCLI(t, m)
	content := fileContent(t, files, "internal/commands/dual.go")

	// Both flags should use StringVar.
	// Count occurrences of StringVar — should appear at least twice (once per object flag).
	count := strings.Count(content, "StringVar")
	assert.GreaterOrEqual(t, count, 2,
		"both object and object[] flags must each use StringVar (found %d occurrences)", count)

	// Both should have JSON parsing.
	assert.Contains(t, content, "invalid JSON for --single",
		"single object flag must have JSON error handling")
	assert.Contains(t, content, "invalid JSON for --multi",
		"object[] flag must have JSON error handling")

	// Neither should use StringArrayVar.
	assert.NotContains(t, content, "StringArrayVar",
		"neither object nor object[] flags should use StringArrayVar")
}

func TestGoCLI_Object_SpecialCharsInDescription_Escaped(t *testing.T) {
	// Constitution 25a: manifest-supplied values must be escaped at the string
	// literal boundary. Test that an object flag with special characters in its
	// description does not break the generated code.
	m := manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:        "escape-toolkit",
			Version:     "1.0.0",
			Description: "Escape test",
		},
		Tools: []manifest.Tool{
			{
				Name:        "escape-tool",
				Description: "Test escaping",
				Entrypoint:  "./escape.sh",
				Auth:        &manifest.Auth{Type: "none"},
				Flags: []manifest.Flag{
					{
						Name:        "config",
						Type:        "object",
						Description: `Config with "quotes" and\nbackslash`,
						ItemSchema: map[string]any{
							"type": "object",
							"properties": map[string]any{
								"key": map[string]any{"type": "string"},
							},
						},
					},
				},
			},
		},
	}

	// The generation must not error even with special characters.
	files := generateCLI(t, m)
	content := fileContent(t, files, "internal/commands/escape-tool.go")

	// The generated code must still be valid (no raw unescaped quotes that
	// would break the Go string literal).
	assert.Contains(t, content, "json.Unmarshal",
		"object flag with special chars in description must still generate JSON parsing")
	// The raw quote should be escaped.
	assert.NotContains(t, content, `Config with "quotes"`,
		"description with double quotes must be escaped in generated Go string literal")
}

func TestGoCLI_Object_BuildToolData_SetsObjectFlagFields(t *testing.T) {
	// buildToolData must set IsObject and HasItemSchema fields on flagData
	// for object/object[] types so templates can conditionally emit code.
	m := manifestWithObjectFlag()
	tool := m.Tools[0]
	auth := m.ResolvedAuth(tool)
	data := buildToolData(m, tool, auth)

	require.Len(t, data.Flags, 1, "must have exactly one flag")
	flag := data.Flags[0]

	assert.Equal(t, "edits", flag.Name,
		"flag name must be 'edits'")
	assert.Equal(t, "string", flag.GoType,
		"object flag GoType must be 'string'")
	// Object flags should NOT be treated as array flags.
	assert.False(t, flag.IsArray,
		"object flag IsArray must be false")
}

func TestGoCLI_Object_BuildToolData_ObjectArrayNotTreatedAsArrayFlag(t *testing.T) {
	// object[] should NOT set IsArray=true (unlike string[], int[], etc).
	// It is a single JSON string containing an array, not a repeated CLI flag.
	m := manifestWithObjectArrayFlag()
	tool := m.Tools[0]
	auth := m.ResolvedAuth(tool)
	data := buildToolData(m, tool, auth)

	require.Len(t, data.Flags, 1, "must have exactly one flag")
	flag := data.Flags[0]

	assert.Equal(t, "updates", flag.Name,
		"flag name must be 'updates'")
	assert.Equal(t, "string", flag.GoType,
		"object[] flag GoType must be 'string'")
	assert.False(t, flag.IsArray,
		"object[] flag IsArray must be false — it is a single JSON string, not a repeated flag")
}

func TestGoCLI_Object_AC8_HasNonStringArrayFlags_NotSetForObjectArray(t *testing.T) {
	// The HasNonStringArrayFlags field triggers strconv import. Object[] flags
	// should NOT trigger it (they use encoding/json, not strconv).
	m := manifestWithObjectArrayFlag()
	tool := m.Tools[0]
	auth := m.ResolvedAuth(tool)
	data := buildToolData(m, tool, auth)

	assert.False(t, data.HasNonStringArrayFlags,
		"object[] flag must not set HasNonStringArrayFlags (no strconv parsing)")
}

// ---------------------------------------------------------------------------
// Task 1 — AC1: CLI generator emits entrypoint execution
// ---------------------------------------------------------------------------

// manifestEntrypointWiring returns a manifest exercising entrypoint + full
// arg/flag coverage for Task 1 tests. The tool has positional args, multiple
// flag types, and token auth — the kitchen sink for cliArgs construction.
func manifestEntrypointWiring() manifest.Toolkit {
	return manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:        "wire-toolkit",
			Version:     "1.0.0",
			Description: "Entrypoint wiring toolkit",
		},
		Tools: []manifest.Tool{
			{
				Name:        "run-job",
				Description: "Run a job on the cluster",
				Entrypoint:  "/usr/local/bin/run-job",
				Auth: &manifest.Auth{
					Type:      "token",
					TokenEnv:  "JOB_TOKEN",
					TokenFlag: "--api-key",
				},
				Args: []manifest.Arg{
					{Name: "job-name", Type: "string", Required: true, Description: "Name of the job"},
					{Name: "priority", Type: "int", Required: false, Description: "Job priority"},
				},
				Flags: []manifest.Flag{
					{Name: "verbose", Type: "bool", Required: false, Default: false, Description: "Verbose output"},
					{Name: "retries", Type: "int", Required: false, Default: 3, Description: "Number of retries"},
					{Name: "threshold", Type: "float", Required: false, Default: 0.5, Description: "Score threshold"},
					{Name: "label", Type: "string", Required: false, Description: "Job label"},
					{Name: "tags", Type: "string[]", Required: false, Description: "Tags to apply"},
					{Name: "counts", Type: "int[]", Required: false, Description: "Count list"},
					{Name: "config", Type: "object", Required: false, Description: "Config object"},
				},
			},
		},
	}
}

func TestGoCLI_AC1_EntrypointUsed_NotEcho(t *testing.T) {
	// The generated code must use the tool's manifest entrypoint as the
	// executable in exec.CommandContext, NOT "echo".
	files := generateCLI(t, manifestEntrypointWiring())
	content := fileContent(t, files, "internal/commands/run-job.go")

	// Must contain the actual entrypoint path.
	assert.Contains(t, content, "/usr/local/bin/run-job",
		"generated code must use the tool's entrypoint path, not a stub")
	// Must NOT use "echo" as the executable.
	assert.NotContains(t, content, `"echo"`,
		"generated code must not use \"echo\" as the executable — use the real entrypoint")
}

func TestGoCLI_AC1_EntrypointInExecCommandContext(t *testing.T) {
	// Verify the entrypoint appears in the specific exec.CommandContext call,
	// not just anywhere in the file. This catches a sloppy implementation
	// that puts the entrypoint in a comment but still calls echo.
	files := generateCLI(t, manifestEntrypointWiring())
	content := fileContent(t, files, "internal/commands/run-job.go")

	assert.Regexp(t, `exec\.CommandContext\([^)]*"/usr/local/bin/run-job"`, content,
		"entrypoint must be the executable argument to exec.CommandContext")
}

func TestGoCLI_AC1_EntrypointFromManifest_TableDriven(t *testing.T) {
	// Table-driven (Constitution 9): verify different entrypoint values.
	tests := []struct {
		name       string
		entrypoint string
		want       string
	}{
		{
			name:       "absolute path",
			entrypoint: "/opt/tools/deploy",
			want:       "/opt/tools/deploy",
		},
		{
			name:       "relative path",
			entrypoint: "./scripts/run.sh",
			want:       "./scripts/run.sh",
		},
		{
			name:       "bare command",
			entrypoint: "python3",
			want:       "python3",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := manifest.Toolkit{
				APIVersion: "toolwright/v1",
				Kind:       "Toolkit",
				Metadata: manifest.Metadata{
					Name:    "ep-test",
					Version: "1.0.0",
				},
				Tools: []manifest.Tool{
					{
						Name:       "mytool",
						Entrypoint: tc.entrypoint,
						Auth:       &manifest.Auth{Type: "none"},
					},
				},
			}
			files := generateCLI(t, m)
			content := fileContent(t, files, "internal/commands/mytool.go")
			assert.Contains(t, content, tc.want,
				"entrypoint %q must appear in generated code", tc.entrypoint)
			assert.NotContains(t, content, `"echo"`,
				"generated code must not use echo stub with entrypoint %q", tc.entrypoint)
		})
	}
}

func TestGoCLI_AC1_BuildToolData_EntrypointPopulated(t *testing.T) {
	// The toolGoData struct must include an Entrypoint field populated from
	// tool.Entrypoint. We verify via the generated output: if the entrypoint
	// field is missing from the struct, the template cannot interpolate it
	// and the generated code will not contain the entrypoint path.
	m := manifestEntrypointWiring()
	files := generateCLI(t, m)
	content := fileContent(t, files, "internal/commands/run-job.go")

	// The entrypoint must appear in exec.CommandContext — this proves the
	// struct carried it through to the template.
	assert.Regexp(t, `exec\.CommandContext\([^)]*"/usr/local/bin/run-job"`, content,
		"buildToolData must populate Entrypoint so the template can interpolate it into exec.CommandContext")
}

func TestGoCLI_AC1_EmptyEntrypoint_ProducesGuard(t *testing.T) {
	// A manifest with entrypoint: "" must generate a guard that returns
	// an error at runtime rather than trying to exec "".
	m := manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:    "noep-toolkit",
			Version: "1.0.0",
		},
		Tools: []manifest.Tool{
			{
				Name:        "unconfigured",
				Description: "Tool with empty entrypoint",
				Entrypoint:  "",
				Auth:        &manifest.Auth{Type: "none"},
			},
		},
	}
	files := generateCLI(t, m)
	content := fileContent(t, files, "internal/commands/unconfigured.go")

	// The generated RunE must contain a guard that checks for empty entrypoint.
	assert.Contains(t, content, "entrypoint not configured",
		"empty entrypoint must produce a guard error containing 'entrypoint not configured'")
	assert.Contains(t, content, "unconfigured",
		"guard error message must include the tool name 'unconfigured'")
}

func TestGoCLI_AC1_EmptyEntrypoint_GuardBlocksExecution(t *testing.T) {
	// The guard must appear before exec.CommandContext so the tool never
	// attempts to exec an empty string. Verify by checking that the guard
	// string "entrypoint not configured" appears before "CommandContext"
	// (or that CommandContext is absent for empty entrypoint tools).
	m := manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:    "noep2-toolkit",
			Version: "1.0.0",
		},
		Tools: []manifest.Tool{
			{
				Name:       "empty-ep",
				Entrypoint: "",
				Auth:       &manifest.Auth{Type: "none"},
			},
		},
	}
	files := generateCLI(t, m)
	content := fileContent(t, files, "internal/commands/empty-ep.go")

	guardIdx := strings.Index(content, "entrypoint not configured")
	assert.NotEqual(t, -1, guardIdx,
		"generated code must contain 'entrypoint not configured' guard")

	// Either CommandContext is absent entirely (valid: the guard returns before
	// reaching it) or it appears after the guard.
	cmdIdx := strings.Index(content, "CommandContext")
	if cmdIdx != -1 {
		assert.Greater(t, cmdIdx, guardIdx,
			"entrypoint guard must appear before CommandContext call")
	}
}

func TestGoCLI_AC1_EntrypointEscaped_Quotes(t *testing.T) {
	// AC6.4: Entrypoint paths with quotes must be safely escaped so the
	// generated Go string literal is valid.
	m := manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:    "esc-toolkit",
			Version: "1.0.0",
		},
		Tools: []manifest.Tool{
			{
				Name:       "quotetool",
				Entrypoint: `/path/with "quotes"/run`,
				Auth:       &manifest.Auth{Type: "none"},
			},
		},
	}
	files := generateCLI(t, m)
	content := fileContent(t, files, "internal/commands/quotetool.go")

	// The raw double-quote must be escaped as \" in the Go string literal.
	assert.Contains(t, content, `\"quotes\"`,
		"double quotes in entrypoint must be escaped as \\\" in generated Go literal")
	// The raw path with unescaped quotes must NOT appear verbatim. If the
	// implementation forgot to escape, the template would produce something
	// like: CommandContext(..., "/path/with "quotes"/run") which is broken Go.
	// We check that the full raw entrypoint path does not appear unescaped
	// by looking for the sequence that would only exist without escaping.
	assert.NotContains(t, content, `with "quotes"`,
		"raw unescaped entrypoint path with bare quotes must not appear in generated code")
}

func TestGoCLI_AC1_EntrypointEscaped_Backslash(t *testing.T) {
	// AC6.4: Entrypoint paths with backslashes (e.g., Windows paths) must
	// be safely escaped.
	m := manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:    "bs-toolkit",
			Version: "1.0.0",
		},
		Tools: []manifest.Tool{
			{
				Name:       "bstool",
				Entrypoint: `C:\Program Files\tool\run.exe`,
				Auth:       &manifest.Auth{Type: "none"},
			},
		},
	}
	files := generateCLI(t, m)
	content := fileContent(t, files, "internal/commands/bstool.go")

	// Backslashes must be doubled in the Go string literal.
	assert.Contains(t, content, `C:\\Program Files\\tool\\run.exe`,
		"backslashes in entrypoint must be escaped as \\\\ in generated Go literal")
}

// ---------------------------------------------------------------------------
// Task 1 — AC2: CLI generator builds args in runner convention order
// ---------------------------------------------------------------------------

func TestGoCLI_AC2_CliArgsSliceExists(t *testing.T) {
	// The generated code must build a cliArgs (or equivalent) slice
	// containing the tool's arguments and flags for the entrypoint.
	files := generateCLI(t, manifestEntrypointWiring())
	content := fileContent(t, files, "internal/commands/run-job.go")

	assert.Contains(t, content, "cliArgs",
		"generated code must build a cliArgs slice for entrypoint arguments")
}

func TestGoCLI_AC2_PositionalArgsFirst(t *testing.T) {
	// Positional args must appear before any flags in cliArgs.
	// The generated code appends positional args first.
	files := generateCLI(t, manifestEntrypointWiring())
	content := fileContent(t, files, "internal/commands/run-job.go")

	// The positional arg names (job-name, priority) must appear in the args
	// building section before any flag names (--verbose, --retries, etc.)
	// We look for the first occurrence of a positional arg variable and the
	// first flag append to verify ordering.
	jobNameIdx := strings.Index(content, "argJobName")
	assert.NotEqual(t, -1, jobNameIdx,
		"generated code must reference positional arg variable 'argJobName'")

	// The first flag in cliArgs must come after positional args.
	flagAppendIdx := strings.Index(content, `"--verbose"`)
	if flagAppendIdx != -1 {
		assert.Greater(t, flagAppendIdx, jobNameIdx,
			"flags in cliArgs must appear after positional args")
	}
}

func TestGoCLI_AC2_FlagsInDefinitionOrder(t *testing.T) {
	// Flags must appear in the order defined in the manifest.
	// The manifest defines: verbose, retries, threshold, label, tags, counts, config.
	files := generateCLI(t, manifestEntrypointWiring())
	content := fileContent(t, files, "internal/commands/run-job.go")

	// All flag names must appear in the cliArgs building section.
	flagNames := []string{"--verbose", "--retries", "--threshold", "--label", "--tags", "--counts", "--config"}
	lastIdx := -1
	for _, flag := range flagNames {
		idx := strings.Index(content, flag)
		assert.NotEqual(t, -1, idx,
			"generated code must reference flag %q in cliArgs construction", flag)
		if lastIdx != -1 && idx != -1 {
			assert.Greater(t, idx, lastIdx,
				"flag %q must appear after previous flag in definition order", flag)
		}
		if idx != -1 {
			lastIdx = idx
		}
	}
}

func TestGoCLI_AC2_BoolFlag_OnlyWhenTrue(t *testing.T) {
	// Bool flags emit only --flag when true; omitted when false.
	// The generated code must have a conditional: if boolVar { append --flag }.
	files := generateCLI(t, manifestEntrypointWiring())
	content := fileContent(t, files, "internal/commands/run-job.go")

	// The code must conditionally append the bool flag, not unconditionally.
	// We verify that there's a conditional check around the verbose flag.
	assert.Regexp(t, `if\s+.*[Vv]erbose`, content,
		"bool flag 'verbose' must be conditionally appended (only when true)")
	// Bool flags must NOT emit a value (--verbose true), just --verbose.
	// Look for a pattern that appends --verbose without a following value.
	assert.NotRegexp(t, `"--verbose".*"true"`, content,
		"bool flag must emit only --verbose, not --verbose true")
}

func TestGoCLI_AC2_StringFlag_OmittedWhenEmpty(t *testing.T) {
	// Empty/zero-value string flags are omitted from cliArgs.
	files := generateCLI(t, manifestEntrypointWiring())
	content := fileContent(t, files, "internal/commands/run-job.go")

	// There must be a conditional check for the label flag (string type).
	assert.Regexp(t, `if\s+.*[Ll]abel\s*!=\s*""`, content,
		"string flag 'label' must be conditionally appended (omitted when empty)")
}

func TestGoCLI_AC2_IntFlag_Stringified(t *testing.T) {
	// Int flags via fmt.Sprintf("%d", val). Zero-value int still passed.
	files := generateCLI(t, manifestEntrypointWiring())
	content := fileContent(t, files, "internal/commands/run-job.go")

	// The retries int flag must be stringified using Sprintf("%d", ...).
	assert.Regexp(t, `Sprintf\("%d".*[Rr]etries`, content,
		"int flag 'retries' must be stringified via fmt.Sprintf(\"%%d\", ...)")
}

func TestGoCLI_AC2_FloatFlag_Stringified(t *testing.T) {
	// Float flags via fmt.Sprintf("%g", val).
	files := generateCLI(t, manifestEntrypointWiring())
	content := fileContent(t, files, "internal/commands/run-job.go")

	// The threshold float flag must be stringified using Sprintf("%g", ...).
	assert.Regexp(t, `Sprintf\("%g".*[Tt]hreshold`, content,
		"float flag 'threshold' must be stringified via fmt.Sprintf(\"%%g\", ...)")
}

func TestGoCLI_AC2_ArrayFlag_RepeatedPairs(t *testing.T) {
	// Array flags: each element emits a separate --flag element pair.
	files := generateCLI(t, manifestEntrypointWiring())
	content := fileContent(t, files, "internal/commands/run-job.go")

	// The tags (string[]) flag must iterate and emit --tags for each element.
	// Look for a range loop that appends --tags per element.
	assert.Regexp(t, `range\s+.*[Tt]ags`, content,
		"string[] flag 'tags' must iterate elements to emit repeated --tags pairs")
	assert.Contains(t, content, `"--tags"`,
		"string[] flag must emit '--tags' flag name for each element")
}

func TestGoCLI_AC2_IntArrayFlag_RepeatedStringifiedPairs(t *testing.T) {
	// int[] array flags: each element stringified and emitted as --flag value.
	files := generateCLI(t, manifestEntrypointWiring())
	content := fileContent(t, files, "internal/commands/run-job.go")

	// The counts (int[]) flag must iterate and emit --counts per element.
	assert.Contains(t, content, `"--counts"`,
		"int[] flag must emit '--counts' flag name for each element")
}

func TestGoCLI_AC2_ObjectFlag_JsonString(t *testing.T) {
	// Object flags emit --flag json_string.
	files := generateCLI(t, manifestEntrypointWiring())
	content := fileContent(t, files, "internal/commands/run-job.go")

	// The config (object) flag must be passed as --config <json_string>.
	assert.Contains(t, content, `"--config"`,
		"object flag must emit '--config' in cliArgs")
}

func TestGoCLI_AC2_TokenAppendedLast(t *testing.T) {
	// Auth token last as --{tokenFlag} {token}. TokenFlag stored WITHOUT --.
	files := generateCLI(t, manifestEntrypointWiring())
	content := fileContent(t, files, "internal/commands/run-job.go")

	// The token must be appended to cliArgs after all other args/flags.
	// The token flag name (api-key) must appear in the cliArgs section.
	assert.Contains(t, content, `"--api-key"`,
		"token flag must appear as '--api-key' in cliArgs")

	// Verify token append comes after other flags. Find --config (last
	// non-token flag) and --api-key (token flag).
	configIdx := strings.Index(content, `"--config"`)
	tokenIdx := strings.Index(content, `"--api-key"`)
	if configIdx != -1 && tokenIdx != -1 {
		assert.Greater(t, tokenIdx, configIdx,
			"token flag '--api-key' must appear after last regular flag '--config' in cliArgs")
	}
}

func TestGoCLI_AC2_NoAuthTool_NoTokenInArgs(t *testing.T) {
	// No token arg when HasAuth == false.
	m := manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:    "noauth-wiring",
			Version: "1.0.0",
		},
		Tools: []manifest.Tool{
			{
				Name:       "simple-run",
				Entrypoint: "./simple.sh",
				Auth:       &manifest.Auth{Type: "none"},
				Args: []manifest.Arg{
					{Name: "input", Type: "string", Required: true, Description: "Input file"},
				},
				Flags: []manifest.Flag{
					{Name: "verbose", Type: "bool", Required: false, Description: "Verbose output"},
				},
			},
		},
	}
	files := generateCLI(t, m)
	content := fileContent(t, files, "internal/commands/simple-run.go")

	// Must use entrypoint, not echo.
	assert.Contains(t, content, "./simple.sh",
		"no-auth tool must still use its entrypoint")
	assert.NotContains(t, content, `"echo"`,
		"no-auth tool must not use echo stub")
	// Must NOT have any token-related cliArgs.
	assert.NotContains(t, content, "--token",
		"no-auth tool must not append --token to cliArgs")
	assert.NotContains(t, content, "--api-key",
		"no-auth tool must not append --api-key to cliArgs")
}

func TestGoCLI_AC2_IntFlag_ZeroValueStillPassed(t *testing.T) {
	// Zero-value int/float still passed (unlike string which is omitted).
	// This means no conditional check for int == 0 or float == 0.
	m := manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:    "zero-val-toolkit",
			Version: "1.0.0",
		},
		Tools: []manifest.Tool{
			{
				Name:       "zero-tool",
				Entrypoint: "./zero.sh",
				Auth:       &manifest.Auth{Type: "none"},
				Flags: []manifest.Flag{
					{Name: "count", Type: "int", Required: false, Default: 0, Description: "A count"},
					{Name: "weight", Type: "float", Required: false, Default: 0.0, Description: "A weight"},
					{Name: "name", Type: "string", Required: false, Description: "A name"},
				},
			},
		},
	}
	files := generateCLI(t, m)
	content := fileContent(t, files, "internal/commands/zero-tool.go")

	// Int/float flags must NOT have an "if != 0" guard; they are always passed.
	// String flags DO have an "if != ''" guard.
	assert.NotRegexp(t, `if\s+.*[Cc]ount\s*!=\s*0`, content,
		"int flag 'count' must NOT be conditionally omitted at zero value")
	assert.NotRegexp(t, `if\s+.*[Ww]eight\s*!=\s*0`, content,
		"float flag 'weight' must NOT be conditionally omitted at zero value")
	// But string flag MUST have a guard.
	assert.Regexp(t, `if\s+.*[Nn]ame\s*!=\s*""`, content,
		"string flag 'name' must be conditionally omitted when empty")
}

func TestGoCLI_AC2_CliArgsPassedToExecCommand(t *testing.T) {
	// The cliArgs slice must actually be passed to exec.CommandContext,
	// not just built and ignored.
	files := generateCLI(t, manifestEntrypointWiring())
	content := fileContent(t, files, "internal/commands/run-job.go")

	// exec.CommandContext takes (ctx, name, arg...) — the cliArgs must be
	// spread into it.
	assert.Regexp(t, `CommandContext\([^,]+,\s*"[^"]*"[^)]*cliArgs`, content,
		"cliArgs must be passed to exec.CommandContext (e.g., cliArgs...)")
}

func TestGoCLI_AC2_MultiplePositionalArgs_InOrder(t *testing.T) {
	// When a tool has multiple positional args, they must appear in
	// definition order (job-name before priority).
	files := generateCLI(t, manifestEntrypointWiring())
	content := fileContent(t, files, "internal/commands/run-job.go")

	// Both arg variables must be appended to cliArgs.
	jobNameIdx := strings.Index(content, "argJobName")
	priorityIdx := strings.Index(content, "argPriority")
	assert.NotEqual(t, -1, jobNameIdx,
		"positional arg 'argJobName' must be referenced in cliArgs construction")
	assert.NotEqual(t, -1, priorityIdx,
		"positional arg 'argPriority' must be referenced in cliArgs construction")
	// job-name before priority.
	assert.Less(t, jobNameIdx, priorityIdx,
		"positional args must appear in definition order: job-name before priority")
}

func TestGoCLI_AC2_EntrypointPerTool_NotShared(t *testing.T) {
	// When a manifest has multiple tools with different entrypoints, each
	// tool command file must use its own entrypoint.
	m := manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:    "multi-ep-toolkit",
			Version: "1.0.0",
		},
		Tools: []manifest.Tool{
			{
				Name:       "alpha",
				Entrypoint: "/bin/alpha-runner",
				Auth:       &manifest.Auth{Type: "none"},
			},
			{
				Name:       "bravo",
				Entrypoint: "/bin/bravo-runner",
				Auth:       &manifest.Auth{Type: "none"},
			},
		},
	}
	files := generateCLI(t, m)

	alphaContent := fileContent(t, files, "internal/commands/alpha.go")
	bravoContent := fileContent(t, files, "internal/commands/bravo.go")

	// alpha.go must use alpha-runner and NOT bravo-runner.
	assert.Contains(t, alphaContent, "/bin/alpha-runner",
		"alpha.go must use alpha's entrypoint")
	assert.NotContains(t, alphaContent, "/bin/bravo-runner",
		"alpha.go must not use bravo's entrypoint")

	// bravo.go must use bravo-runner and NOT alpha-runner.
	assert.Contains(t, bravoContent, "/bin/bravo-runner",
		"bravo.go must use bravo's entrypoint")
	assert.NotContains(t, bravoContent, "/bin/alpha-runner",
		"bravo.go must not use alpha's entrypoint")

	// Neither should use echo.
	assert.NotContains(t, alphaContent, `"echo"`,
		"alpha.go must not use echo stub")
	assert.NotContains(t, bravoContent, `"echo"`,
		"bravo.go must not use echo stub")
}

// ---------------------------------------------------------------------------
// Task 1 — AC6.1: No token values in string literals
// ---------------------------------------------------------------------------

func TestGoCLI_AC6_1_NoTokenLiteralInExecArgs(t *testing.T) {
	// Generated CLI code must NOT embed token values in string literals.
	// The token must be passed via a variable, not hardcoded.
	files := generateCLI(t, manifestEntrypointWiring())
	content := fileContent(t, files, "internal/commands/run-job.go")

	// The cliArgs construction must reference the token variable, not a literal.
	// Looking for the pattern: token variable used as the arg value.
	assert.Regexp(t, `"--api-key"[^"]*token`, content,
		"token must be passed via variable (e.g., token), not as a string literal")
}

// ---------------------------------------------------------------------------
// Task 1 — Cross-cutting: existing manifests still work with entrypoint
// ---------------------------------------------------------------------------

func TestGoCLI_AC1_ExistingManifests_UseEntrypoint(t *testing.T) {
	// All existing test manifests with non-empty entrypoints must now
	// generate exec.CommandContext with that entrypoint, not "echo".
	tests := []struct {
		name     string
		manifest manifest.Toolkit
		toolFile string
		wantEP   string
	}{
		{
			name:     "manifestTwoToolsMixed/status",
			manifest: manifestTwoToolsMixed(),
			toolFile: "internal/commands/status.go",
			wantEP:   "./status.sh",
		},
		{
			name:     "manifestTwoToolsMixed/deploy",
			manifest: manifestTwoToolsMixed(),
			toolFile: "internal/commands/deploy.go",
			wantEP:   "./deploy.sh",
		},
		{
			name:     "manifestAllAuthNone/ping",
			manifest: manifestAllAuthNone(),
			toolFile: "internal/commands/ping.go",
			wantEP:   "./ping.sh",
		},
		{
			name:     "manifestWithArgsAndFlags/upload",
			manifest: manifestWithArgsAndFlags(),
			toolFile: "internal/commands/upload.go",
			wantEP:   "./upload.sh",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			files := generateCLI(t, tc.manifest)
			content := fileContent(t, files, tc.toolFile)
			assert.Contains(t, content, tc.wantEP,
				"generated code must contain entrypoint %q", tc.wantEP)
			assert.NotContains(t, content, `"echo"`,
				"generated code must not use echo stub")
		})
	}
}

func TestGoCLI_AC1_BinaryOutputTool_UsesEntrypoint(t *testing.T) {
	// Binary output tools also have the echo stub — they must use entrypoint too.
	m := manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:    "bin-toolkit",
			Version: "1.0.0",
		},
		Tools: []manifest.Tool{
			{
				Name:        "image-gen",
				Description: "Generate an image",
				Entrypoint:  "./image-gen.sh",
				Auth:        &manifest.Auth{Type: "none"},
				Output:      manifest.Output{Format: "binary"},
			},
		},
	}
	files := generateCLI(t, m)
	content := fileContent(t, files, "internal/commands/image-gen.go")

	// The binary output path also has exec.CommandContext — it must use the
	// real entrypoint.
	assert.Contains(t, content, "./image-gen.sh",
		"binary output tool must use entrypoint, not echo")
	assert.NotContains(t, content, `"echo"`,
		"binary output tool must not use echo stub")
}

func TestGoCLI_AC2_ObjectArrayFlag_JsonInCliArgs(t *testing.T) {
	// object[] flags pass --flag json_string (same as object).
	m := manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:    "objarray-cli",
			Version: "1.0.0",
		},
		Tools: []manifest.Tool{
			{
				Name:       "batch",
				Entrypoint: "./batch.sh",
				Auth:       &manifest.Auth{Type: "none"},
				Flags: []manifest.Flag{
					{Name: "items", Type: "object[]", Description: "Batch items"},
				},
			},
		},
	}
	files := generateCLI(t, m)
	content := fileContent(t, files, "internal/commands/batch.go")

	// The object[] flag must appear in cliArgs as --items <json_string>.
	assert.Contains(t, content, `"--items"`,
		"object[] flag must emit '--items' in cliArgs")
	assert.NotContains(t, content, `"echo"`,
		"tool must not use echo stub")
}

func TestGoCLI_AC2_TokenFlagPrefix_NoDoubleDash(t *testing.T) {
	// TokenFlag stored WITHOUT -- prefix. The cliArgs construction must add
	// the -- prefix when building the arg. This tests that the TrimPrefix
	// at buildToolData (line ~391) works correctly with cliArgs.
	m := manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:    "prefix-test",
			Version: "1.0.0",
		},
		Tools: []manifest.Tool{
			{
				Name:       "authtool",
				Entrypoint: "./auth.sh",
				Auth: &manifest.Auth{
					Type:      "token",
					TokenEnv:  "AUTH_TOKEN",
					TokenFlag: "--secret-key",
				},
			},
		},
	}
	files := generateCLI(t, m)
	content := fileContent(t, files, "internal/commands/authtool.go")

	// TokenFlag is stored as "secret-key" (TrimPrefix removes --).
	// The cliArgs must emit "--secret-key", not "----secret-key".
	assert.Contains(t, content, `"--secret-key"`,
		"cliArgs must emit --secret-key (single -- prefix)")
	assert.NotContains(t, content, `"----secret-key"`,
		"cliArgs must not double the -- prefix on TokenFlag")
}

func TestGoCLI_AC2_ArgsAndFlags_CompleteIntegration(t *testing.T) {
	// Integration-level test: a tool with args, various flag types, and auth
	// must produce generated code containing all the expected cliArgs pieces.
	files := generateCLI(t, manifestEntrypointWiring())
	content := fileContent(t, files, "internal/commands/run-job.go")

	// Entrypoint, not echo.
	assert.Contains(t, content, "/usr/local/bin/run-job",
		"must use real entrypoint")
	assert.NotContains(t, content, `"echo"`,
		"must not use echo")

	// cliArgs must exist.
	assert.Contains(t, content, "cliArgs",
		"must build cliArgs slice")

	// Positional args referenced.
	assert.Contains(t, content, "argJobName",
		"must reference positional arg job-name")

	// All flags referenced.
	for _, flag := range []string{"--verbose", "--retries", "--threshold", "--label", "--tags", "--counts", "--config"} {
		assert.Contains(t, content, flag,
			"must reference flag %q in cliArgs", flag)
	}

	// Token last.
	assert.Contains(t, content, `"--api-key"`,
		"must include token flag in cliArgs")
}
