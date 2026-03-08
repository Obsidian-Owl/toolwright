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
// Test manifests for TS MCP generator
// ---------------------------------------------------------------------------

// mcpManifestTwoToolsNoAuth returns a manifest with 2 tools (both auth:none),
// transport [stdio]. This is the primary manifest for AC-7.
func mcpManifestTwoToolsNoAuth() manifest.Toolkit {
	return manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:        "my-mcp-server",
			Version:     "1.0.0",
			Description: "An MCP server with two tools",
		},
		Tools: []manifest.Tool{
			{
				Name:        "get_weather",
				Description: "Get weather for a location",
				Entrypoint:  "./weather.sh",
				Auth:        &manifest.Auth{Type: "none"},
				Args: []manifest.Arg{
					{Name: "location", Type: "string", Required: true, Description: "City name"},
				},
			},
			{
				Name:        "search_docs",
				Description: "Search documentation",
				Entrypoint:  "./search.sh",
				Auth:        &manifest.Auth{Type: "none"},
				Flags: []manifest.Flag{
					{Name: "query", Type: "string", Required: true, Description: "Search query"},
					{Name: "limit", Type: "int", Required: false, Default: 10, Description: "Max results"},
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

// mcpManifestTokenAuth returns a manifest with a tool that uses token auth.
func mcpManifestTokenAuth() manifest.Toolkit {
	return manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:        "auth-mcp-server",
			Version:     "1.0.0",
			Description: "MCP server with token auth",
		},
		Tools: []manifest.Tool{
			{
				Name:        "deploy",
				Description: "Deploy a service",
				Entrypoint:  "./deploy.sh",
				Auth: &manifest.Auth{
					Type:        "token",
					TokenEnv:    "DEPLOY_TOKEN",
					TokenHeader: "Authorization",
				},
			},
		},
		Generate: manifest.Generate{
			MCP: manifest.MCPConfig{
				Target:    "typescript",
				Transport: []string{"streamable-http"},
			},
		},
	}
}

// mcpManifestOAuth2StreamableHTTP returns a manifest with oauth2 auth and
// streamable-http transport.
func mcpManifestOAuth2StreamableHTTP() manifest.Toolkit {
	return manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:        "oauth-mcp-server",
			Version:     "2.0.0",
			Description: "OAuth2 MCP server",
		},
		Tools: []manifest.Tool{
			{
				Name:        "protected_resource",
				Description: "Access a protected resource",
				Entrypoint:  "./protected.sh",
				Auth: &manifest.Auth{
					Type:        "oauth2",
					ProviderURL: "https://auth.example.com",
					Scopes:      []string{"read", "write"},
				},
			},
		},
		Generate: manifest.Generate{
			MCP: manifest.MCPConfig{
				Target:    "typescript",
				Transport: []string{"streamable-http"},
			},
		},
	}
}

// mcpManifestOAuth2StdioOnly returns a manifest with oauth2 auth but ONLY
// stdio transport. metadata.ts should NOT be generated (stdio is local).
func mcpManifestOAuth2StdioOnly() manifest.Toolkit {
	return manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:        "stdio-oauth-server",
			Version:     "1.0.0",
			Description: "OAuth2 but stdio only",
		},
		Tools: []manifest.Tool{
			{
				Name:        "local_tool",
				Description: "A local tool with oauth2",
				Entrypoint:  "./local.sh",
				Auth: &manifest.Auth{
					Type:        "oauth2",
					ProviderURL: "https://auth.example.com",
					Scopes:      []string{"read"},
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

// mcpManifestOAuth2BothTransports returns a manifest with oauth2 and both
// stdio + streamable-http transports. metadata.ts should be generated
// because streamable-http is present.
func mcpManifestOAuth2BothTransports() manifest.Toolkit {
	return manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:        "dual-transport-server",
			Version:     "1.0.0",
			Description: "Both transports with oauth2",
		},
		Tools: []manifest.Tool{
			{
				Name:        "dual_tool",
				Description: "Works on both transports",
				Entrypoint:  "./dual.sh",
				Auth: &manifest.Auth{
					Type:        "oauth2",
					ProviderURL: "https://auth.example.com",
					Scopes:      []string{"read", "write", "admin"},
				},
			},
		},
		Generate: manifest.Generate{
			MCP: manifest.MCPConfig{
				Target:    "typescript",
				Transport: []string{"stdio", "streamable-http"},
			},
		},
	}
}

// mcpManifestMixedAuth returns a manifest with one auth:none tool and one
// auth:token tool.
func mcpManifestMixedAuth() manifest.Toolkit {
	return manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:        "mixed-auth-server",
			Version:     "1.0.0",
			Description: "Mixed auth MCP server",
		},
		Tools: []manifest.Tool{
			{
				Name:        "public_info",
				Description: "Get public information",
				Entrypoint:  "./public.sh",
				Auth:        &manifest.Auth{Type: "none"},
			},
			{
				Name:        "admin_action",
				Description: "Perform admin action",
				Entrypoint:  "./admin.sh",
				Auth: &manifest.Auth{
					Type:        "token",
					TokenEnv:    "ADMIN_TOKEN",
					TokenHeader: "Authorization",
				},
			},
		},
		Generate: manifest.Generate{
			MCP: manifest.MCPConfig{
				Target:    "typescript",
				Transport: []string{"streamable-http"},
			},
		},
	}
}

// mcpManifestWithAllTypes returns a manifest with a tool that has args and
// flags covering all four manifest types (string, int, float, bool).
func mcpManifestWithAllTypes() manifest.Toolkit {
	return manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:        "typed-server",
			Version:     "1.0.0",
			Description: "Type mapping test",
		},
		Tools: []manifest.Tool{
			{
				Name:        "typed_tool",
				Description: "Tool with all types",
				Entrypoint:  "./typed.sh",
				Auth:        &manifest.Auth{Type: "none"},
				Args: []manifest.Arg{
					{Name: "name", Type: "string", Required: true, Description: "A string arg"},
					{Name: "count", Type: "int", Required: true, Description: "An int arg"},
				},
				Flags: []manifest.Flag{
					{Name: "ratio", Type: "float", Required: false, Default: 1.5, Description: "A float flag"},
					{Name: "verbose", Type: "bool", Required: false, Default: false, Description: "A bool flag"},
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

// ---------------------------------------------------------------------------
// Generator helper — mirrors generateCLI from cli_go_test.go
// ---------------------------------------------------------------------------

func generateTSMCP(t *testing.T, m manifest.Toolkit) []GeneratedFile {
	t.Helper()
	gen := NewTSMCPGenerator()
	data := TemplateData{
		Manifest:  m,
		Timestamp: "2026-03-04T12:00:00Z",
		Version:   "0.1.0",
	}
	files, err := gen.Generate(context.Background(), data, "")
	require.NoError(t, err, "TSMCPGenerator.Generate must not error for a valid manifest")
	require.NotEmpty(t, files, "TSMCPGenerator.Generate must return at least one file")
	return files
}

// ---------------------------------------------------------------------------
// Generator instantiation and interface compliance
// ---------------------------------------------------------------------------

func TestTSMCPGenerator_ImplementsGeneratorInterface(t *testing.T) {
	var _ Generator = (*TSMCPGenerator)(nil)
}

func TestTSMCPGenerator_Mode(t *testing.T) {
	gen := NewTSMCPGenerator()
	assert.Equal(t, "mcp", gen.Mode())
}

func TestTSMCPGenerator_Target(t *testing.T) {
	gen := NewTSMCPGenerator()
	assert.Equal(t, "typescript", gen.Target())
}

// ---------------------------------------------------------------------------
// AC-7: TypeScript MCP generates valid project structure
// ---------------------------------------------------------------------------

func TestTSMCP_AC7_IndexTSPresent(t *testing.T) {
	files := generateTSMCP(t, mcpManifestTwoToolsNoAuth())
	requireFile(t, files, "src/index.ts")
}

func TestTSMCP_AC7_IndexTSContainsStdioServer(t *testing.T) {
	files := generateTSMCP(t, mcpManifestTwoToolsNoAuth())
	content := fileContent(t, files, "src/index.ts")
	// The index.ts must set up an MCP server with stdio transport
	contentLower := strings.ToLower(content)
	assert.True(t,
		strings.Contains(contentLower, "stdio") ||
			strings.Contains(content, "StdioServerTransport"),
		"src/index.ts must reference stdio transport for a stdio-only manifest")
}

func TestTSMCP_AC7_IndexTSImportsMCPSDK(t *testing.T) {
	files := generateTSMCP(t, mcpManifestTwoToolsNoAuth())
	content := fileContent(t, files, "src/index.ts")
	assert.Contains(t, content, "@modelcontextprotocol/sdk",
		"src/index.ts must import from @modelcontextprotocol/sdk")
}

func TestTSMCP_AC7_PerToolFilesPresent(t *testing.T) {
	files := generateTSMCP(t, mcpManifestTwoToolsNoAuth())
	requireFile(t, files, "src/tools/get_weather.ts")
	requireFile(t, files, "src/tools/search_docs.ts")
}

func TestTSMCP_AC7_PerToolFilesContainToolName(t *testing.T) {
	files := generateTSMCP(t, mcpManifestTwoToolsNoAuth())

	weatherContent := fileContent(t, files, "src/tools/get_weather.ts")
	assert.Contains(t, weatherContent, "get_weather",
		"get_weather.ts must reference its tool name")
	assert.Contains(t, weatherContent, "Get weather for a location",
		"get_weather.ts must include the tool description")

	searchContent := fileContent(t, files, "src/tools/search_docs.ts")
	assert.Contains(t, searchContent, "search_docs",
		"search_docs.ts must reference its tool name")
	assert.Contains(t, searchContent, "Search documentation",
		"search_docs.ts must include the tool description")
}

func TestTSMCP_AC7_PerToolFilesAreDistinct(t *testing.T) {
	files := generateTSMCP(t, mcpManifestTwoToolsNoAuth())
	weatherContent := fileContent(t, files, "src/tools/get_weather.ts")
	searchContent := fileContent(t, files, "src/tools/search_docs.ts")
	assert.NotEqual(t, weatherContent, searchContent,
		"per-tool files must not be identical (catches lazy copy-paste)")
}

func TestTSMCP_AC7_SearchTSPresent(t *testing.T) {
	files := generateTSMCP(t, mcpManifestTwoToolsNoAuth())
	requireFile(t, files, "src/search.ts")
}

func TestTSMCP_AC7_PackageJSONPresent(t *testing.T) {
	files := generateTSMCP(t, mcpManifestTwoToolsNoAuth())
	requireFile(t, files, "package.json")
}

func TestTSMCP_AC7_PackageJSONContainsMCPSDK(t *testing.T) {
	files := generateTSMCP(t, mcpManifestTwoToolsNoAuth())
	content := fileContent(t, files, "package.json")
	assert.Contains(t, content, "@modelcontextprotocol/sdk",
		"package.json must list @modelcontextprotocol/sdk as a dependency")
}

func TestTSMCP_AC7_PackageJSONContainsToolkitName(t *testing.T) {
	files := generateTSMCP(t, mcpManifestTwoToolsNoAuth())
	content := fileContent(t, files, "package.json")
	assert.Contains(t, content, "my-mcp-server",
		"package.json must reference the toolkit name")
}

func TestTSMCP_AC7_PackageJSONIsValidStructure(t *testing.T) {
	files := generateTSMCP(t, mcpManifestTwoToolsNoAuth())
	content := fileContent(t, files, "package.json")
	// Must contain key JSON fields, not just the string "@modelcontextprotocol/sdk"
	assert.Contains(t, content, `"dependencies"`,
		"package.json must have a dependencies section")
	assert.Contains(t, content, `"name"`,
		"package.json must have a name field")
}

func TestTSMCP_AC7_PackageJSONContainsZodDependency(t *testing.T) {
	files := generateTSMCP(t, mcpManifestTwoToolsNoAuth())
	content := fileContent(t, files, "package.json")
	assert.Contains(t, content, `"zod"`,
		"package.json must list zod as a dependency (required by MCP SDK for schema validation)")
}

func TestTSMCP_AC7_TSConfigPresent(t *testing.T) {
	files := generateTSMCP(t, mcpManifestTwoToolsNoAuth())
	requireFile(t, files, "tsconfig.json")
}

func TestTSMCP_AC7_TSConfigContainsCompilerOptions(t *testing.T) {
	files := generateTSMCP(t, mcpManifestTwoToolsNoAuth())
	content := fileContent(t, files, "tsconfig.json")
	assert.Contains(t, content, "compilerOptions",
		"tsconfig.json must contain compilerOptions")
}

func TestTSMCP_AC7_ReadmePresent(t *testing.T) {
	files := generateTSMCP(t, mcpManifestTwoToolsNoAuth())
	requireFile(t, files, "README.md")
}

func TestTSMCP_AC7_ReadmeContainsToolkitName(t *testing.T) {
	files := generateTSMCP(t, mcpManifestTwoToolsNoAuth())
	content := fileContent(t, files, "README.md")
	assert.Contains(t, content, "my-mcp-server",
		"README.md must reference the toolkit name")
}

func TestTSMCP_AC7_NoAuthMiddleware_WhenAllAuthNone(t *testing.T) {
	files := generateTSMCP(t, mcpManifestTwoToolsNoAuth())
	assertNoFile(t, files, "src/auth/middleware.ts")
}

func TestTSMCP_AC7_NoMetadataTS_WhenAllAuthNoneStdio(t *testing.T) {
	files := generateTSMCP(t, mcpManifestTwoToolsNoAuth())
	assertNoFile(t, files, "src/auth/metadata.ts")
}

func TestTSMCP_AC7_CorrectFileSet(t *testing.T) {
	files := generateTSMCP(t, mcpManifestTwoToolsNoAuth())
	paths := filePaths(files)
	expectedPaths := []string{
		"src/index.ts",
		"src/tools/get_weather.ts",
		"src/tools/search_docs.ts",
		"src/search.ts",
		"package.json",
		"tsconfig.json",
		"README.md",
	}
	for _, ep := range expectedPaths {
		assert.Contains(t, paths, ep, "expected file %q in output", ep)
	}
}

func TestTSMCP_AC7_IndexTSRegistersAllTools(t *testing.T) {
	files := generateTSMCP(t, mcpManifestTwoToolsNoAuth())
	content := fileContent(t, files, "src/index.ts")
	// The index file must reference both tools — a lazy implementation that
	// only registers one tool would fail this.
	assert.Contains(t, content, "get_weather",
		"src/index.ts must register the get_weather tool")
	assert.Contains(t, content, "search_docs",
		"src/index.ts must register the search_docs tool")
}

// ---------------------------------------------------------------------------
// AC-8: TypeScript MCP generates auth middleware for token auth
// ---------------------------------------------------------------------------

func TestTSMCP_AC8_MiddlewareTSPresent_WhenTokenAuth(t *testing.T) {
	files := generateTSMCP(t, mcpManifestTokenAuth())
	requireFile(t, files, "src/auth/middleware.ts")
}

func TestTSMCP_AC8_MiddlewareTSValidatesBearerHeader(t *testing.T) {
	files := generateTSMCP(t, mcpManifestTokenAuth())
	content := fileContent(t, files, "src/auth/middleware.ts")
	contentLower := strings.ToLower(content)
	// Must reference Bearer token validation
	assert.True(t,
		strings.Contains(content, "Bearer") || strings.Contains(contentLower, "bearer"),
		"middleware.ts must reference Bearer token validation")
	assert.True(t,
		strings.Contains(content, "Authorization") || strings.Contains(contentLower, "authorization"),
		"middleware.ts must reference the Authorization header")
}

func TestTSMCP_AC8_MiddlewareTSExportsFunction(t *testing.T) {
	files := generateTSMCP(t, mcpManifestTokenAuth())
	content := fileContent(t, files, "src/auth/middleware.ts")
	assert.Contains(t, content, "export",
		"middleware.ts must export its auth validation function")
}

func TestTSMCP_AC8_MiddlewareTSHandlesInvalidToken(t *testing.T) {
	files := generateTSMCP(t, mcpManifestTokenAuth())
	content := fileContent(t, files, "src/auth/middleware.ts")
	contentLower := strings.ToLower(content)
	// Must have error handling for unauthorized access
	assert.True(t,
		strings.Contains(contentLower, "401") ||
			strings.Contains(contentLower, "unauthorized") ||
			strings.Contains(contentLower, "forbidden") ||
			strings.Contains(contentLower, "error"),
		"middleware.ts must handle invalid/missing tokens with an error response")
}

func TestTSMCP_AC8_MiddlewareAbsent_WhenAuthNone(t *testing.T) {
	files := generateTSMCP(t, mcpManifestTwoToolsNoAuth())
	assertNoFile(t, files, "src/auth/middleware.ts")
}

// ---------------------------------------------------------------------------
// AC-9: TypeScript MCP generates PRM endpoint for oauth2
// ---------------------------------------------------------------------------

func TestTSMCP_AC9_MetadataTSPresent_WhenOAuth2StreamableHTTP(t *testing.T) {
	files := generateTSMCP(t, mcpManifestOAuth2StreamableHTTP())
	requireFile(t, files, "src/auth/metadata.ts")
}

func TestTSMCP_AC9_MetadataTSContainsWellKnownPath(t *testing.T) {
	files := generateTSMCP(t, mcpManifestOAuth2StreamableHTTP())
	content := fileContent(t, files, "src/auth/metadata.ts")
	assert.Contains(t, content, ".well-known/oauth-protected-resource",
		"metadata.ts must define the /.well-known/oauth-protected-resource endpoint")
}

func TestTSMCP_AC9_MetadataTSContainsResourceField(t *testing.T) {
	files := generateTSMCP(t, mcpManifestOAuth2StreamableHTTP())
	content := fileContent(t, files, "src/auth/metadata.ts")
	assert.Contains(t, content, "resource",
		"PRM response must include 'resource' field")
}

func TestTSMCP_AC9_MetadataTSContainsAuthorizationServers(t *testing.T) {
	files := generateTSMCP(t, mcpManifestOAuth2StreamableHTTP())
	content := fileContent(t, files, "src/auth/metadata.ts")
	assert.Contains(t, content, "authorization_servers",
		"PRM response must include 'authorization_servers' field")
}

func TestTSMCP_AC9_MetadataTSContainsScopesSupported(t *testing.T) {
	files := generateTSMCP(t, mcpManifestOAuth2StreamableHTTP())
	content := fileContent(t, files, "src/auth/metadata.ts")
	assert.Contains(t, content, "scopes_supported",
		"PRM response must include 'scopes_supported' field")
}

func TestTSMCP_AC9_MetadataTSReferencesProviderURL(t *testing.T) {
	files := generateTSMCP(t, mcpManifestOAuth2StreamableHTTP())
	content := fileContent(t, files, "src/auth/metadata.ts")
	assert.Contains(t, content, "https://auth.example.com",
		"metadata.ts must reference the provider URL from the manifest")
}

func TestTSMCP_AC9_MetadataTSReferencesScopes(t *testing.T) {
	files := generateTSMCP(t, mcpManifestOAuth2StreamableHTTP())
	content := fileContent(t, files, "src/auth/metadata.ts")
	assert.Contains(t, content, "read",
		"metadata.ts must include scope 'read'")
	assert.Contains(t, content, "write",
		"metadata.ts must include scope 'write'")
}

func TestTSMCP_AC9_MetadataTSAbsent_WhenOAuth2StdioOnly(t *testing.T) {
	// Critical: oauth2 + stdio-only should NOT produce metadata.ts
	// because stdio is a local transport with no HTTP layer.
	files := generateTSMCP(t, mcpManifestOAuth2StdioOnly())
	assertNoFile(t, files, "src/auth/metadata.ts")
}

func TestTSMCP_AC9_MetadataTSPresent_WhenOAuth2BothTransports(t *testing.T) {
	// If both transports are present, streamable-http triggers metadata.ts.
	files := generateTSMCP(t, mcpManifestOAuth2BothTransports())
	requireFile(t, files, "src/auth/metadata.ts")
}

func TestTSMCP_AC9_MetadataTSWithBothTransports_IncludesAllScopes(t *testing.T) {
	files := generateTSMCP(t, mcpManifestOAuth2BothTransports())
	content := fileContent(t, files, "src/auth/metadata.ts")
	// The dual-transport manifest has scopes: read, write, admin
	assert.Contains(t, content, "read",
		"metadata.ts must include scope 'read'")
	assert.Contains(t, content, "write",
		"metadata.ts must include scope 'write'")
	assert.Contains(t, content, "admin",
		"metadata.ts must include scope 'admin'")
}

func TestTSMCP_AC9_MetadataTSAbsent_WhenNoOAuth2(t *testing.T) {
	// Token auth (not oauth2) should not produce metadata.ts even with
	// streamable-http transport.
	files := generateTSMCP(t, mcpManifestTokenAuth())
	assertNoFile(t, files, "src/auth/metadata.ts")
}

// ---------------------------------------------------------------------------
// AC-10: TypeScript MCP search_tools meta-tool
// ---------------------------------------------------------------------------

func TestTSMCP_AC10_SearchTSListsToolNames(t *testing.T) {
	files := generateTSMCP(t, mcpManifestTwoToolsNoAuth())
	content := fileContent(t, files, "src/search.ts")
	assert.Contains(t, content, "get_weather",
		"search.ts must include tool name 'get_weather'")
	assert.Contains(t, content, "search_docs",
		"search.ts must include tool name 'search_docs'")
}

func TestTSMCP_AC10_SearchTSListsToolDescriptions(t *testing.T) {
	files := generateTSMCP(t, mcpManifestTwoToolsNoAuth())
	content := fileContent(t, files, "src/search.ts")
	assert.Contains(t, content, "Get weather for a location",
		"search.ts must include tool description for get_weather")
	assert.Contains(t, content, "Search documentation",
		"search.ts must include tool description for search_docs")
}

func TestTSMCP_AC10_SearchTSImplementsToolInterface(t *testing.T) {
	files := generateTSMCP(t, mcpManifestTwoToolsNoAuth())
	content := fileContent(t, files, "src/search.ts")
	contentLower := strings.ToLower(content)
	// Must implement a search/list tool that agents can call
	assert.True(t,
		strings.Contains(contentLower, "search") ||
			strings.Contains(contentLower, "list"),
		"search.ts must implement a search or list tool")
	assert.Contains(t, content, "export",
		"search.ts must export its tool definition")
}

func TestTSMCP_AC10_SearchTSExposesProgressiveDiscovery(t *testing.T) {
	// The search tool should expose names+descriptions for progressive
	// discovery: agents call search first, then get full schema on demand.
	files := generateTSMCP(t, mcpManifestTwoToolsNoAuth())
	content := fileContent(t, files, "src/search.ts")
	contentLower := strings.ToLower(content)
	// Must reference both "name" and "description" fields in the response
	assert.True(t,
		strings.Contains(contentLower, "name") &&
			strings.Contains(contentLower, "description"),
		"search.ts must expose tool names and descriptions for progressive discovery")
}

func TestTSMCP_AC10_SearchTSRegisteredInIndex(t *testing.T) {
	files := generateTSMCP(t, mcpManifestTwoToolsNoAuth())
	content := fileContent(t, files, "src/index.ts")
	// The search meta-tool must be registered as a tool in the server
	assert.True(t,
		strings.Contains(content, "search") || strings.Contains(content, "Search"),
		"src/index.ts must register the search meta-tool")
}

// ---------------------------------------------------------------------------
// AC-11: Type mapping (table-driven per Constitution rule 9)
// ---------------------------------------------------------------------------

func TestTSMCP_AC11_TypeMapping(t *testing.T) {
	tests := []struct {
		name         string
		manifestType string
		wantZodCall  string // Zod schema method (z.string(), z.number(), z.boolean())
	}{
		{name: "string maps to z.string()", manifestType: "string", wantZodCall: "z.string()"},
		{name: "int maps to z.number()", manifestType: "int", wantZodCall: "z.number()"},
		{name: "float maps to z.number()", manifestType: "float", wantZodCall: "z.number()"},
		{name: "bool maps to z.boolean()", manifestType: "bool", wantZodCall: "z.boolean()"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := manifest.Toolkit{
				APIVersion: "toolwright/v1",
				Kind:       "Toolkit",
				Metadata: manifest.Metadata{
					Name:    "typemap-ts-server",
					Version: "1.0.0",
				},
				Tools: []manifest.Tool{
					{
						Name:       "type_test",
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
				Generate: manifest.Generate{
					MCP: manifest.MCPConfig{
						Target:    "typescript",
						Transport: []string{"stdio"},
					},
				},
			}

			files := generateTSMCP(t, m)
			content := fileContent(t, files, "src/tools/type_test.ts")
			assert.Contains(t, content, tc.wantZodCall,
				"manifest type %q must produce Zod schema call %q in generated code",
				tc.manifestType, tc.wantZodCall)
		})
	}
}

func TestTSMCP_AC11_TypeMappingArgs(t *testing.T) {
	tests := []struct {
		name         string
		manifestType string
		wantTSType   string
	}{
		{name: "string arg maps to string", manifestType: "string", wantTSType: "string"},
		{name: "int arg maps to number", manifestType: "int", wantTSType: "number"},
		{name: "float arg maps to number", manifestType: "float", wantTSType: "number"},
		{name: "bool arg maps to boolean", manifestType: "bool", wantTSType: "boolean"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := manifest.Toolkit{
				APIVersion: "toolwright/v1",
				Kind:       "Toolkit",
				Metadata: manifest.Metadata{
					Name:    "typemap-arg-server",
					Version: "1.0.0",
				},
				Tools: []manifest.Tool{
					{
						Name:       "arg_test",
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
				Generate: manifest.Generate{
					MCP: manifest.MCPConfig{
						Target:    "typescript",
						Transport: []string{"stdio"},
					},
				},
			}

			files := generateTSMCP(t, m)
			content := fileContent(t, files, "src/tools/arg_test.ts")
			assert.Contains(t, content, tc.wantTSType,
				"manifest arg type %q must map to TS type %q in generated code",
				tc.manifestType, tc.wantTSType)
		})
	}
}

func TestTSMCP_AC11_IntAndFloatBothMapToNumber_ButAreDistinct(t *testing.T) {
	// Both int and float map to "number" in TS, but the generated tool file
	// should still handle both types correctly. We verify the tool file with
	// both types contains "number" used in the right context for each.
	files := generateTSMCP(t, mcpManifestWithAllTypes())
	content := fileContent(t, files, "src/tools/typed_tool.ts")

	// Must contain references to both arg names with their types
	assert.Contains(t, content, "count",
		"typed_tool.ts must reference int arg 'count'")
	assert.Contains(t, content, "ratio",
		"typed_tool.ts must reference float flag 'ratio'")
	assert.Contains(t, content, "verbose",
		"typed_tool.ts must reference bool flag 'verbose'")
	assert.Contains(t, content, "name",
		"typed_tool.ts must reference string arg 'name'")

	// Must contain the correct TS types
	assert.Contains(t, content, "number",
		"typed_tool.ts must contain 'number' type for int/float")
	assert.Contains(t, content, "boolean",
		"typed_tool.ts must contain 'boolean' type for bool")
	assert.Contains(t, content, "z.string()",
		"typed_tool.ts must contain z.string() Zod schema for string type")
}

// ---------------------------------------------------------------------------
// AC-13: No secrets in generated code
// ---------------------------------------------------------------------------

func TestTSMCP_AC13_NoLiteralTokenValues(t *testing.T) {
	files := generateTSMCP(t, mcpManifestTokenAuth())

	secretPatterns := []string{
		"sk-",           // Common API key prefix
		"ghp_",          // GitHub PAT prefix
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

func TestTSMCP_AC13_NoHardcodedTokenValues_OAuth2(t *testing.T) {
	files := generateTSMCP(t, mcpManifestOAuth2StreamableHTTP())

	secretPatterns := []string{
		"sk-",
		"ghp_",
		"password:",
		"client_secret",
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

func TestTSMCP_AC13_TokenEnvReferencedByName_NotValue(t *testing.T) {
	files := generateTSMCP(t, mcpManifestTokenAuth())

	var foundEnvRef bool
	for _, f := range files {
		content := string(f.Content)
		if strings.Contains(content, "DEPLOY_TOKEN") {
			foundEnvRef = true
			// Must reference the env var by name (process.env pattern in TS)
			contentLower := strings.ToLower(content)
			assert.True(t,
				strings.Contains(content, "process.env") ||
					strings.Contains(contentLower, "env") ||
					strings.Contains(contentLower, "environment"),
				"file %q references env var name but must use process.env or similar, not a literal value",
				f.Path)
			break
		}
	}
	assert.True(t, foundEnvRef,
		"at least one generated file must reference the token env var name 'DEPLOY_TOKEN'")
}

// ---------------------------------------------------------------------------
// AC-15: Generated code handles tools with no auth / mixed auth
// ---------------------------------------------------------------------------

func TestTSMCP_AC15_NoAuthCodeForNoneAuthTool(t *testing.T) {
	files := generateTSMCP(t, mcpManifestMixedAuth())
	content := fileContent(t, files, "src/tools/public_info.ts")
	contentLower := strings.ToLower(content)
	assert.False(t,
		strings.Contains(contentLower, "authorization") ||
			strings.Contains(contentLower, "bearer") ||
			strings.Contains(contentLower, "token") ||
			strings.Contains(contentLower, "middleware"),
		"public_info.ts (auth:none) must not contain auth-related code")
}

func TestTSMCP_AC15_AuthCodePresentForTokenAuthTool(t *testing.T) {
	files := generateTSMCP(t, mcpManifestMixedAuth())
	content := fileContent(t, files, "src/tools/admin_action.ts")
	contentLower := strings.ToLower(content)
	assert.True(t,
		strings.Contains(contentLower, "token") ||
			strings.Contains(contentLower, "auth") ||
			strings.Contains(contentLower, "middleware"),
		"admin_action.ts (auth:token) must contain auth-related code")
}

func TestTSMCP_AC15_MixedAuth_AuthCountDiffers(t *testing.T) {
	files := generateTSMCP(t, mcpManifestMixedAuth())

	publicContent := fileContent(t, files, "src/tools/public_info.ts")
	adminContent := fileContent(t, files, "src/tools/admin_action.ts")

	publicAuthRefs := countTSAuthReferences(publicContent)
	adminAuthRefs := countTSAuthReferences(adminContent)

	assert.Greater(t, adminAuthRefs, publicAuthRefs,
		"admin_action.ts (auth:token) must have more auth references than public_info.ts (auth:none): admin=%d, public=%d",
		adminAuthRefs, publicAuthRefs)
}

func TestTSMCP_AC15_AllNoneAuth_NoAuthFiles(t *testing.T) {
	files := generateTSMCP(t, mcpManifestTwoToolsNoAuth())
	// When all tools are auth:none, there should be no auth directory files
	for _, f := range files {
		assert.Falsef(t, strings.HasPrefix(f.Path, "src/auth/"),
			"file %q should not exist when all tools are auth:none", f.Path)
	}
}

func TestTSMCP_AC15_AllNoneAuth_ToolFilesHaveNoAuthImports(t *testing.T) {
	files := generateTSMCP(t, mcpManifestTwoToolsNoAuth())
	for _, f := range files {
		if strings.HasPrefix(f.Path, "src/tools/") {
			content := string(f.Content)
			assert.NotContainsf(t, content, "auth/middleware",
				"file %q should not import auth middleware when all tools are auth:none", f.Path)
		}
	}
}

func TestTSMCP_AC15_MiddlewarePresent_WhenMixedAuth(t *testing.T) {
	// When there is at least one tool with token auth, middleware must be generated.
	files := generateTSMCP(t, mcpManifestMixedAuth())
	requireFile(t, files, "src/auth/middleware.ts")
}

// countTSAuthReferences counts occurrences of auth-related terms in TS content.
func countTSAuthReferences(content string) int {
	lower := strings.ToLower(content)
	terms := []string{"token", "auth", "bearer", "credential", "middleware", "authorization"}
	count := 0
	for _, term := range terms {
		count += strings.Count(lower, term)
	}
	return count
}

// ---------------------------------------------------------------------------
// Transport-specific tests
// ---------------------------------------------------------------------------

func TestTSMCP_StdioTransport_IndexUsesStdioTransport(t *testing.T) {
	files := generateTSMCP(t, mcpManifestTwoToolsNoAuth())
	content := fileContent(t, files, "src/index.ts")
	contentLower := strings.ToLower(content)
	assert.True(t,
		strings.Contains(contentLower, "stdio"),
		"index.ts must reference stdio transport when manifest specifies stdio")
}

func TestTSMCP_StreamableHTTPTransport_IndexUsesHTTPTransport(t *testing.T) {
	files := generateTSMCP(t, mcpManifestTokenAuth())
	content := fileContent(t, files, "src/index.ts")
	contentLower := strings.ToLower(content)
	assert.True(t,
		strings.Contains(contentLower, "http") ||
			strings.Contains(contentLower, "streamable") ||
			strings.Contains(content, "StreamableHTTPServerTransport") ||
			strings.Contains(content, "SSEServerTransport"),
		"index.ts must reference HTTP/streamable transport when manifest specifies streamable-http")
}

func TestTSMCP_BothTransports_IndexReferencesBoth(t *testing.T) {
	files := generateTSMCP(t, mcpManifestOAuth2BothTransports())
	content := fileContent(t, files, "src/index.ts")
	contentLower := strings.ToLower(content)
	assert.True(t,
		strings.Contains(contentLower, "stdio"),
		"index.ts must reference stdio when both transports are specified")
	assert.True(t,
		strings.Contains(contentLower, "http") ||
			strings.Contains(contentLower, "streamable"),
		"index.ts must reference HTTP when both transports are specified")
}

// ---------------------------------------------------------------------------
// Edge cases and adversarial tests
// ---------------------------------------------------------------------------

func TestTSMCP_SingleTool_GeneratesCorrectStructure(t *testing.T) {
	m := manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:    "single-tool-server",
			Version: "1.0.0",
		},
		Tools: []manifest.Tool{
			{
				Name:       "only_tool",
				Entrypoint: "./only.sh",
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

	files := generateTSMCP(t, m)
	requireFile(t, files, "src/index.ts")
	requireFile(t, files, "src/tools/only_tool.ts")
	requireFile(t, files, "src/search.ts")
	requireFile(t, files, "package.json")
	requireFile(t, files, "tsconfig.json")
	requireFile(t, files, "README.md")
}

func TestTSMCP_ManyTools_AllGetFiles(t *testing.T) {
	tools := make([]manifest.Tool, 5)
	names := []string{"alpha", "bravo", "charlie", "delta", "echo_tool"}
	for i := range tools {
		tools[i] = manifest.Tool{
			Name:        names[i],
			Description: "Tool " + names[i],
			Entrypoint:  "./" + names[i] + ".sh",
			Auth:        &manifest.Auth{Type: "none"},
		}
	}

	m := manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:    "many-tools-server",
			Version: "1.0.0",
		},
		Tools: tools,
		Generate: manifest.Generate{
			MCP: manifest.MCPConfig{
				Target:    "typescript",
				Transport: []string{"stdio"},
			},
		},
	}

	files := generateTSMCP(t, m)
	for _, name := range names {
		requireFile(t, files, "src/tools/"+name+".ts")
	}

	// search.ts should list all 5 tools
	searchContent := fileContent(t, files, "src/search.ts")
	for _, name := range names {
		assert.Contains(t, searchContent, name,
			"search.ts must list tool %q", name)
	}
}

func TestTSMCP_NoDuplicateFilePaths(t *testing.T) {
	files := generateTSMCP(t, mcpManifestTwoToolsNoAuth())
	seen := make(map[string]bool)
	for _, f := range files {
		assert.Falsef(t, seen[f.Path],
			"duplicate file path %q in generated output", f.Path)
		seen[f.Path] = true
	}
}

func TestTSMCP_NoEmptyFiles(t *testing.T) {
	files := generateTSMCP(t, mcpManifestTwoToolsNoAuth())
	for _, f := range files {
		assert.NotEmptyf(t, f.Content,
			"generated file %q must not have empty content", f.Path)
	}
}

func TestTSMCP_FilePathsAreRelative(t *testing.T) {
	files := generateTSMCP(t, mcpManifestTwoToolsNoAuth())
	for _, f := range files {
		assert.Falsef(t, strings.HasPrefix(f.Path, "/"),
			"generated file path %q must be relative, not absolute", f.Path)
	}
}

func TestTSMCP_TSFilesAreValidTypeScript(t *testing.T) {
	// All .ts files should contain export or import (basic TS syntax check)
	files := generateTSMCP(t, mcpManifestTwoToolsNoAuth())
	for _, f := range files {
		if strings.HasSuffix(f.Path, ".ts") {
			content := string(f.Content)
			assert.True(t,
				strings.Contains(content, "export") ||
					strings.Contains(content, "import"),
				"TypeScript file %q must contain import/export statements", f.Path)
		}
	}
}

func TestTSMCP_ContextCancellationRespected(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	gen := NewTSMCPGenerator()
	data := TemplateData{
		Manifest:  mcpManifestTwoToolsNoAuth(),
		Timestamp: "2026-03-04T12:00:00Z",
		Version:   "0.1.0",
	}
	_, err := gen.Generate(ctx, data, "")
	// Must return an error when context is cancelled.
	require.Error(t, err, "Generate must error when context is cancelled")
	assert.ErrorIs(t, err, context.Canceled,
		"error from cancelled context should wrap context.Canceled")
}

func TestTSMCP_ToolWithNoArgsNoFlags(t *testing.T) {
	m := manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:    "bare-tool-server",
			Version: "1.0.0",
		},
		Tools: []manifest.Tool{
			{
				Name:        "simple",
				Description: "A simple tool with no args or flags",
				Entrypoint:  "./simple.sh",
				Auth:        &manifest.Auth{Type: "none"},
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
	content := fileContent(t, files, "src/tools/simple.ts")
	assert.Contains(t, content, "simple",
		"tool file must reference the tool name even with no args/flags")
	assert.Contains(t, content, "export",
		"tool file must export its handler even with no args/flags")
}

func TestTSMCP_InheritedAuth_ToolGetsAuthCode(t *testing.T) {
	// Toolkit-level auth with no per-tool override
	m := manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:    "inherited-mcp-server",
			Version: "1.0.0",
		},
		Auth: &manifest.Auth{
			Type:     "token",
			TokenEnv: "API_TOKEN",
		},
		Tools: []manifest.Tool{
			{
				Name:        "inherited_tool",
				Description: "Inherits toolkit auth",
				Entrypoint:  "./inherited.sh",
				// No per-tool auth: inherits toolkit-level token auth
			},
		},
		Generate: manifest.Generate{
			MCP: manifest.MCPConfig{
				Target:    "typescript",
				Transport: []string{"streamable-http"},
			},
		},
	}

	files := generateTSMCP(t, m)
	// Middleware should be generated because toolkit-level auth is token
	requireFile(t, files, "src/auth/middleware.ts")

	// The tool's file should have auth references
	content := fileContent(t, files, "src/tools/inherited_tool.ts")
	contentLower := strings.ToLower(content)
	assert.True(t,
		strings.Contains(contentLower, "token") ||
			strings.Contains(contentLower, "auth") ||
			strings.Contains(contentLower, "middleware"),
		"inherited_tool.ts must have auth code when toolkit-level auth is token")
}

func TestTSMCP_OAuth2_GeneratesMiddleware(t *testing.T) {
	// oauth2 auth should also produce auth middleware
	files := generateTSMCP(t, mcpManifestOAuth2StreamableHTTP())
	requireFile(t, files, "src/auth/middleware.ts")
}

func TestTSMCP_PackageJSON_HasScripts(t *testing.T) {
	files := generateTSMCP(t, mcpManifestTwoToolsNoAuth())
	content := fileContent(t, files, "package.json")
	assert.Contains(t, content, `"scripts"`,
		"package.json must have a scripts section")
}

func TestTSMCP_ToolFileContainsInputSchema(t *testing.T) {
	// Tool files should define input schemas for MCP tool registration
	files := generateTSMCP(t, mcpManifestTwoToolsNoAuth())
	content := fileContent(t, files, "src/tools/get_weather.ts")
	// MCP tools need input schema definitions
	contentLower := strings.ToLower(content)
	assert.True(t,
		strings.Contains(contentLower, "schema") ||
			strings.Contains(contentLower, "input") ||
			strings.Contains(contentLower, "parameters") ||
			strings.Contains(contentLower, "properties"),
		"get_weather.ts must define an input schema for MCP tool registration")
}

func TestTSMCP_ToolFileContainsArgDefinitions(t *testing.T) {
	files := generateTSMCP(t, mcpManifestTwoToolsNoAuth())
	content := fileContent(t, files, "src/tools/get_weather.ts")
	// get_weather has arg "location" of type string
	assert.Contains(t, content, "location",
		"get_weather.ts must reference arg 'location'")
}

func TestTSMCP_ToolFileContainsFlagDefinitions(t *testing.T) {
	files := generateTSMCP(t, mcpManifestTwoToolsNoAuth())
	content := fileContent(t, files, "src/tools/search_docs.ts")
	// search_docs has flags "query" (string) and "limit" (int)
	assert.Contains(t, content, "query",
		"search_docs.ts must reference flag 'query'")
	assert.Contains(t, content, "limit",
		"search_docs.ts must reference flag 'limit'")
}

func TestTSMCP_ToolFileContainsFlagDescriptions(t *testing.T) {
	files := generateTSMCP(t, mcpManifestTwoToolsNoAuth())
	content := fileContent(t, files, "src/tools/search_docs.ts")
	assert.Contains(t, content, "Search query",
		"search_docs.ts must include flag description for 'query'")
	assert.Contains(t, content, "Max results",
		"search_docs.ts must include flag description for 'limit'")
}

// ---------------------------------------------------------------------------
// AC-11 (array): Zod array schemas for array flag types
// ---------------------------------------------------------------------------

func TestTSMCP_AC11_ArrayFlagZodSchema(t *testing.T) {
	tests := []struct {
		name         string
		manifestType string
		wantZodExpr  string // expected fragment in generated code
	}{
		{
			name:         "string[] maps to z.array(z.string())",
			manifestType: "string[]",
			wantZodExpr:  "z.array(z.string())",
		},
		{
			name:         "int[] maps to z.array(z.number())",
			manifestType: "int[]",
			wantZodExpr:  "z.array(z.number())",
		},
		{
			name:         "float[] maps to z.array(z.number())",
			manifestType: "float[]",
			wantZodExpr:  "z.array(z.number())",
		},
		{
			name:         "bool[] maps to z.array(z.boolean())",
			manifestType: "bool[]",
			wantZodExpr:  "z.array(z.boolean())",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := manifest.Toolkit{
				APIVersion: "toolwright/v1",
				Kind:       "Toolkit",
				Metadata: manifest.Metadata{
					Name:    "array-zod-server",
					Version: "1.0.0",
				},
				Tools: []manifest.Tool{
					{
						Name:       "array_test",
						Entrypoint: "./test.sh",
						Auth:       &manifest.Auth{Type: "none"},
						Flags: []manifest.Flag{
							{
								Name:        "items",
								Type:        tc.manifestType,
								Required:    true,
								Description: "test array flag",
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
			content := fileContent(t, files, "src/tools/array_test.ts")
			assert.Contains(t, content, tc.wantZodExpr,
				"manifest type %q must produce Zod schema %q in generated code",
				tc.manifestType, tc.wantZodExpr)
		})
	}
}

// ---------------------------------------------------------------------------
// AC-12: Required vs optional array flags
// ---------------------------------------------------------------------------

func TestTSMCP_AC12_RequiredArrayFlag_NoOptional(t *testing.T) {
	m := manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:    "req-array-server",
			Version: "1.0.0",
		},
		Tools: []manifest.Tool{
			{
				Name:       "req_array_tool",
				Entrypoint: "./test.sh",
				Auth:       &manifest.Auth{Type: "none"},
				Flags: []manifest.Flag{
					{
						Name:        "tags",
						Type:        "string[]",
						Required:    true,
						Description: "required array flag",
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
	content := fileContent(t, files, "src/tools/req_array_tool.ts")
	assert.Contains(t, content, "z.array(z.string())",
		"required string[] flag must emit z.array(z.string())")
	// Required flag must NOT have .optional() appended to the array schema
	assert.NotContains(t, content, "z.array(z.string()).optional()",
		"required string[] flag must not emit .optional()")
}

func TestTSMCP_AC12_OptionalArrayFlag_HasOptional(t *testing.T) {
	m := manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:    "opt-array-server",
			Version: "1.0.0",
		},
		Tools: []manifest.Tool{
			{
				Name:       "opt_array_tool",
				Entrypoint: "./test.sh",
				Auth:       &manifest.Auth{Type: "none"},
				Flags: []manifest.Flag{
					{
						Name:        "tags",
						Type:        "string[]",
						Required:    false,
						Description: "optional array flag",
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
	content := fileContent(t, files, "src/tools/opt_array_tool.ts")
	assert.Contains(t, content, "z.array(z.string()).optional()",
		"optional string[] flag must emit z.array(z.string()).optional()")
}

func TestTSMCP_AC12_ArrayFlagRequiredOptionalVariants(t *testing.T) {
	tests := []struct {
		name         string
		manifestType string
		required     bool
		wantContains string
		wantAbsent   string
	}{
		{
			name:         "required int[] has no optional",
			manifestType: "int[]",
			required:     true,
			wantContains: "z.array(z.number())",
			wantAbsent:   "z.array(z.number()).optional()",
		},
		{
			name:         "optional int[] has optional",
			manifestType: "int[]",
			required:     false,
			wantContains: "z.array(z.number()).optional()",
			wantAbsent:   "",
		},
		{
			name:         "optional bool[] has optional",
			manifestType: "bool[]",
			required:     false,
			wantContains: "z.array(z.boolean()).optional()",
			wantAbsent:   "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := manifest.Toolkit{
				APIVersion: "toolwright/v1",
				Kind:       "Toolkit",
				Metadata: manifest.Metadata{
					Name:    "array-req-server",
					Version: "1.0.0",
				},
				Tools: []manifest.Tool{
					{
						Name:       "array_req_test",
						Entrypoint: "./test.sh",
						Auth:       &manifest.Auth{Type: "none"},
						Flags: []manifest.Flag{
							{
								Name:        "items",
								Type:        tc.manifestType,
								Required:    tc.required,
								Description: "test flag",
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
			content := fileContent(t, files, "src/tools/array_req_test.ts")
			assert.Contains(t, content, tc.wantContains,
				"manifest type %q required=%v must produce %q",
				tc.manifestType, tc.required, tc.wantContains)
			if tc.wantAbsent != "" {
				assert.NotContains(t, content, tc.wantAbsent,
					"manifest type %q required=%v must NOT produce %q",
					tc.manifestType, tc.required, tc.wantAbsent)
			}
		})
	}
}

func TestTSMCP_EmptyToolsSlice(t *testing.T) {
	// Boundary case: manifest with zero tools should still produce structural
	// files (index.ts, search.ts, package.json, tsconfig.json, README) without panic.
	m := manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:        "empty-server",
			Version:     "1.0.0",
			Description: "No tools",
		},
		Tools: []manifest.Tool{},
		Generate: manifest.Generate{
			MCP: manifest.MCPConfig{
				Target:    "typescript",
				Transport: []string{"stdio"},
			},
		},
	}
	files := generateTSMCP(t, m)
	requireFile(t, files, "src/index.ts")
	requireFile(t, files, "src/search.ts")
	requireFile(t, files, "package.json")
	requireFile(t, files, "tsconfig.json")
	requireFile(t, files, "README.md")
	// No per-tool files, no auth middleware
	for _, f := range files {
		assert.Falsef(t, strings.HasPrefix(f.Path, "src/tools/"),
			"empty tools manifest should not produce per-tool file %q", f.Path)
		assert.Falsef(t, strings.HasPrefix(f.Path, "src/auth/"),
			"empty tools manifest should not produce auth file %q", f.Path)
	}
}

// ---------------------------------------------------------------------------
// AC-12: itemSchemaToZod converts common JSON Schema
// ---------------------------------------------------------------------------

// mcpManifestObjectFlagWithSchema returns a manifest with a single tool that has
// an object-typed flag with an itemSchema. Useful for AC-12/AC-14 tests.
func mcpManifestObjectFlagWithSchema(flagType string, required bool, schema map[string]any) manifest.Toolkit {
	return manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:    "object-schema-server",
			Version: "1.0.0",
		},
		Tools: []manifest.Tool{
			{
				Name:        "obj_tool",
				Description: "Tool with object flag",
				Entrypoint:  "./obj.sh",
				Auth:        &manifest.Auth{Type: "none"},
				Flags: []manifest.Flag{
					{
						Name:        "config",
						Type:        flagType,
						Required:    required,
						Description: "structured config",
						ItemSchema:  schema,
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

func TestTSMCP_Object_AC12_PrimitiveSchemaTypes(t *testing.T) {
	// Table-driven: each JSON Schema primitive type must map to the correct
	// Zod expression when used as a property inside an object itemSchema.
	// We test as properties inside an object rather than top-level because
	// top-level {"type": "string"} is indistinguishable from the zodType()
	// fallback for "object" type. Testing inside properties proves the
	// converter recurses correctly.
	tests := []struct {
		name       string
		propSchema map[string]any
		wantZod    string
	}{
		{
			name:       "string property",
			propSchema: map[string]any{"type": "string"},
			wantZod:    "z.string()",
		},
		{
			name:       "number property",
			propSchema: map[string]any{"type": "number"},
			wantZod:    "z.number()",
		},
		{
			name:       "integer property maps to z.number()",
			propSchema: map[string]any{"type": "integer"},
			wantZod:    "z.number()",
		},
		{
			name:       "boolean property",
			propSchema: map[string]any{"type": "boolean"},
			wantZod:    "z.boolean()",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			schema := map[string]any{
				"type": "object",
				"properties": map[string]any{
					"prop": tc.propSchema,
				},
			}
			m := mcpManifestObjectFlagWithSchema("object", true, schema)
			files := generateTSMCP(t, m)
			content := fileContent(t, files, "src/tools/obj_tool.ts")

			// The config flag line must NOT use z.string() as its Zod expression
			// (which is what the zodType() fallback produces for unknown types).
			// Instead it must contain z.object() from the itemSchema conversion.
			lines := strings.Split(content, "\n")
			var configLine string
			for _, line := range lines {
				trimmed := strings.TrimSpace(line)
				if strings.HasPrefix(trimmed, "config:") {
					configLine = trimmed
					break
				}
			}
			require.NotEmpty(t, configLine,
				"generated output must have a line starting with 'config:' for the object flag")
			assert.Contains(t, configLine, "z.object(",
				"config flag line must use z.object() from itemSchema, not z.string() from zodType fallback")

			// The inner property "prop" must appear as its own line with the correct Zod type
			var propLine string
			for _, line := range lines {
				trimmed := strings.TrimSpace(line)
				if strings.HasPrefix(trimmed, "prop:") {
					propLine = trimmed
					break
				}
			}
			require.NotEmpty(t, propLine,
				"generated output must have a line starting with 'prop:' for the schema property")
			assert.Contains(t, propLine, tc.wantZod,
				"property 'prop' line must contain %q", tc.wantZod)
		})
	}
}

func TestTSMCP_Object_AC12_ObjectWithProperties(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{"type": "string"},
		},
	}
	m := mcpManifestObjectFlagWithSchema("object", true, schema)
	files := generateTSMCP(t, m)
	content := fileContent(t, files, "src/tools/obj_tool.ts")

	// Must contain z.object
	assert.Contains(t, content, "z.object(",
		"object schema with properties must emit z.object(...)")
	// Must contain the property name
	assert.Contains(t, content, "name:",
		"object schema property 'name' must appear in generated Zod schema")
	// Must contain z.string() for the name property
	assert.Contains(t, content, "z.string()",
		"object schema property 'name' with type:string must emit z.string()")
}

func TestTSMCP_Object_AC12_ObjectRequiredVsOptionalProperties(t *testing.T) {
	// Schema has two properties: "name" is required, "age" is not.
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"name": map[string]any{"type": "string"},
			"age":  map[string]any{"type": "number"},
		},
		"required": []any{"name"},
	}
	m := mcpManifestObjectFlagWithSchema("object", true, schema)
	files := generateTSMCP(t, m)
	content := fileContent(t, files, "src/tools/obj_tool.ts")

	// The "name" property is required, so it must NOT have .optional()
	// The "age" property is NOT required, so it MUST have .optional()
	assert.Contains(t, content, "z.object(",
		"must emit z.object for object schema with properties")

	// We need to verify that "age" has .optional() but "name" does not.
	// Extract the lines for each property to verify independently.
	lines := strings.Split(content, "\n")
	var nameLine, ageLine string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "name:") {
			nameLine = trimmed
		}
		if strings.HasPrefix(trimmed, "age:") {
			ageLine = trimmed
		}
	}

	require.NotEmpty(t, nameLine,
		"generated output must contain a line starting with 'name:' for the name property")
	require.NotEmpty(t, ageLine,
		"generated output must contain a line starting with 'age:' for the age property")

	assert.NotContains(t, nameLine, ".optional()",
		"required property 'name' must NOT have .optional()")
	assert.Contains(t, ageLine, ".optional()",
		"non-required property 'age' MUST have .optional()")
}

func TestTSMCP_Object_AC12_ArrayWithItems(t *testing.T) {
	schema := map[string]any{
		"type": "array",
		"items": map[string]any{
			"type": "string",
		},
	}
	m := mcpManifestObjectFlagWithSchema("object", true, schema)
	files := generateTSMCP(t, m)
	content := fileContent(t, files, "src/tools/obj_tool.ts")

	assert.Contains(t, content, "z.array(z.string())",
		"array schema with items:{type:string} must emit z.array(z.string())")
}

func TestTSMCP_Object_AC12_NestedObjectWithArray(t *testing.T) {
	// Object with a property "tags" that is an array of strings.
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"tags": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "string",
				},
			},
		},
	}
	m := mcpManifestObjectFlagWithSchema("object", true, schema)
	files := generateTSMCP(t, m)
	content := fileContent(t, files, "src/tools/obj_tool.ts")

	assert.Contains(t, content, "z.object(",
		"nested schema must emit z.object()")
	assert.Contains(t, content, "z.array(z.string())",
		"nested array property must emit z.array(z.string())")
	assert.Contains(t, content, "tags:",
		"nested schema must include property name 'tags'")
}

func TestTSMCP_Object_AC12_MultipleProperties_AllPresent(t *testing.T) {
	// Ensure all properties appear, not just the first or last.
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"host":    map[string]any{"type": "string"},
			"port":    map[string]any{"type": "integer"},
			"verbose": map[string]any{"type": "boolean"},
		},
		"required": []any{"host", "port"},
	}
	m := mcpManifestObjectFlagWithSchema("object", true, schema)
	files := generateTSMCP(t, m)
	content := fileContent(t, files, "src/tools/obj_tool.ts")

	// All three properties must appear in the generated schema.
	assert.Contains(t, content, "host:",
		"property 'host' must appear in generated schema")
	assert.Contains(t, content, "port:",
		"property 'port' must appear in generated schema")
	assert.Contains(t, content, "verbose:",
		"property 'verbose' must appear in generated schema")

	// Verify correct types for each
	assert.Contains(t, content, "z.string()",
		"host property must use z.string()")
	assert.Contains(t, content, "z.number()",
		"port property must use z.number()")
	assert.Contains(t, content, "z.boolean()",
		"verbose property must use z.boolean()")

	// verbose is not required, so it must be optional
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "verbose:") {
			assert.Contains(t, trimmed, ".optional()",
				"non-required property 'verbose' must have .optional()")
		}
		if strings.HasPrefix(trimmed, "host:") {
			assert.NotContains(t, trimmed, ".optional()",
				"required property 'host' must NOT have .optional()")
		}
	}
}

func TestTSMCP_Object_AC12_NestedObject_InnerObject(t *testing.T) {
	// Object containing another object property.
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"address": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"city": map[string]any{"type": "string"},
				},
			},
		},
	}
	m := mcpManifestObjectFlagWithSchema("object", true, schema)
	files := generateTSMCP(t, m)
	content := fileContent(t, files, "src/tools/obj_tool.ts")

	// Must contain nested z.object
	assert.Contains(t, content, "address:",
		"nested object property 'address' must appear in schema")
	assert.Contains(t, content, "city:",
		"deeply nested property 'city' must appear in schema")

	// Count z.object occurrences: outer schema + inner address = at least 2
	// (the tool's top-level inputSchema z.object is separate from itemSchema)
	zodObjectCount := strings.Count(content, "z.object(")
	assert.GreaterOrEqual(t, zodObjectCount, 2,
		"nested object schema must produce at least 2 z.object() calls: "+
			"one for the outer object, one for the inner 'address' object (got %d)", zodObjectCount)
}

// ---------------------------------------------------------------------------
// AC-13: Object flag without itemSchema emits z.record
// ---------------------------------------------------------------------------

func TestTSMCP_Object_AC13_ObjectNoSchema_EmitsZRecord(t *testing.T) {
	// type: "object" with no ItemSchema must produce z.record(z.unknown())
	m := mcpManifestObjectFlagWithSchema("object", true, nil)
	files := generateTSMCP(t, m)
	content := fileContent(t, files, "src/tools/obj_tool.ts")

	assert.Contains(t, content, "z.record(z.unknown())",
		"object flag with no itemSchema must emit z.record(z.unknown())")
	// Must NOT contain z.string() for the config flag — that would mean the
	// object type fell through to the default string case.
	// We check that the config flag line specifically uses z.record, not z.string.
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "config:") {
			assert.Contains(t, trimmed, "z.record(",
				"config flag line must use z.record(), not z.string()")
			assert.NotContains(t, trimmed, "z.string()",
				"config flag line must NOT use z.string() for object type")
			break
		}
	}
}

func TestTSMCP_Object_AC13_ObjectNoSchema_EmptyMapEqualsNil(t *testing.T) {
	// Empty ItemSchema (not nil, but empty map) should also emit z.record
	m := mcpManifestObjectFlagWithSchema("object", true, map[string]any{})
	files := generateTSMCP(t, m)
	content := fileContent(t, files, "src/tools/obj_tool.ts")

	assert.Contains(t, content, "z.record(z.unknown())",
		"object flag with empty itemSchema must emit z.record(z.unknown())")
}

func TestTSMCP_Object_AC13_ObjectArrayNoSchema_EmitsZArrayZRecord(t *testing.T) {
	// type: "object[]" with no ItemSchema must produce z.array(z.record(z.unknown()))
	m := mcpManifestObjectFlagWithSchema("object[]", true, nil)
	files := generateTSMCP(t, m)
	content := fileContent(t, files, "src/tools/obj_tool.ts")

	assert.Contains(t, content, "z.array(z.record(z.unknown()))",
		"object[] flag with no itemSchema must emit z.array(z.record(z.unknown()))")
}

func TestTSMCP_Object_AC13_ObjectArrayNoSchema_NotJustZRecord(t *testing.T) {
	// Verify object[] doesn't just emit z.record without the z.array wrapper.
	m := mcpManifestObjectFlagWithSchema("object[]", true, nil)
	files := generateTSMCP(t, m)
	content := fileContent(t, files, "src/tools/obj_tool.ts")

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "config:") {
			assert.Contains(t, trimmed, "z.array(",
				"object[] flag line must be wrapped in z.array()")
			break
		}
	}
}

// ---------------------------------------------------------------------------
// AC-14: Object[] flag with itemSchema wraps in z.array
// ---------------------------------------------------------------------------

func TestTSMCP_Object_AC14_ObjectArrayWithSchema_EmitsZArrayZObject(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"id":   map[string]any{"type": "integer"},
			"name": map[string]any{"type": "string"},
		},
		"required": []any{"id"},
	}
	m := mcpManifestObjectFlagWithSchema("object[]", true, schema)
	files := generateTSMCP(t, m)
	content := fileContent(t, files, "src/tools/obj_tool.ts")

	// Must wrap the converted object schema in z.array(...)
	assert.Contains(t, content, "z.array(z.object(",
		"object[] with itemSchema must emit z.array(z.object(...))")

	// The inner properties must still be present
	assert.Contains(t, content, "id:",
		"object[] schema property 'id' must appear")
	assert.Contains(t, content, "name:",
		"object[] schema property 'name' must appear")
}

func TestTSMCP_Object_AC14_ObjectArrayWithSchema_InnerRequiredOptional(t *testing.T) {
	// Even within the array wrapper, required/optional distinction on inner
	// properties must be respected.
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"id":    map[string]any{"type": "integer"},
			"label": map[string]any{"type": "string"},
		},
		"required": []any{"id"},
	}
	m := mcpManifestObjectFlagWithSchema("object[]", true, schema)
	files := generateTSMCP(t, m)
	content := fileContent(t, files, "src/tools/obj_tool.ts")

	lines := strings.Split(content, "\n")
	var idLine, labelLine string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "id:") {
			idLine = trimmed
		}
		if strings.HasPrefix(trimmed, "label:") {
			labelLine = trimmed
		}
	}

	require.NotEmpty(t, idLine, "must have line for property 'id'")
	require.NotEmpty(t, labelLine, "must have line for property 'label'")

	assert.NotContains(t, idLine, ".optional()",
		"required property 'id' in object[] schema must NOT have .optional()")
	assert.Contains(t, labelLine, ".optional()",
		"non-required property 'label' in object[] schema MUST have .optional()")
}

func TestTSMCP_Object_AC14_ObjectArrayWithSchema_NotDoubleWrapped(t *testing.T) {
	// Verify the output contains exactly one z.array wrapping (not z.array(z.array(...)))
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"x": map[string]any{"type": "number"},
		},
	}
	m := mcpManifestObjectFlagWithSchema("object[]", true, schema)
	files := generateTSMCP(t, m)
	content := fileContent(t, files, "src/tools/obj_tool.ts")

	assert.NotContains(t, content, "z.array(z.array(",
		"object[] must not double-wrap in z.array(z.array(...))")
}

// ---------------------------------------------------------------------------
// Cross-cutting: object flag with itemSchema uses converted result
// ---------------------------------------------------------------------------

func TestTSMCP_Object_CrossCutting_ObjectWithSchema_UsesConvertedSchema(t *testing.T) {
	// type: "object" WITH itemSchema must use the converted schema, not z.record.
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"key": map[string]any{"type": "string"},
		},
	}
	m := mcpManifestObjectFlagWithSchema("object", true, schema)
	files := generateTSMCP(t, m)
	content := fileContent(t, files, "src/tools/obj_tool.ts")

	// Must NOT emit z.record — should use the itemSchema conversion instead.
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "config:") {
			assert.NotContains(t, trimmed, "z.record(",
				"object flag WITH itemSchema must NOT emit z.record(); must use itemSchemaToZod")
			assert.Contains(t, trimmed, "z.object(",
				"object flag WITH itemSchema must emit z.object()")
			break
		}
	}
}

func TestTSMCP_Object_CrossCutting_RequiredObjectFlag_NoOptional(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"val": map[string]any{"type": "string"},
		},
	}
	m := mcpManifestObjectFlagWithSchema("object", true, schema)
	files := generateTSMCP(t, m)
	content := fileContent(t, files, "src/tools/obj_tool.ts")

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "config:") {
			assert.Contains(t, trimmed, "z.object(",
				"required object flag with schema must use z.object(), not z.string()")
			assert.NotContains(t, trimmed, ".optional()",
				"required object flag must NOT have .optional()")
			break
		}
	}
}

func TestTSMCP_Object_CrossCutting_OptionalObjectFlag_HasOptional(t *testing.T) {
	schema := map[string]any{
		"type": "object",
		"properties": map[string]any{
			"val": map[string]any{"type": "string"},
		},
	}
	m := mcpManifestObjectFlagWithSchema("object", false, schema)
	files := generateTSMCP(t, m)
	content := fileContent(t, files, "src/tools/obj_tool.ts")

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "config:") {
			assert.Contains(t, trimmed, "z.object(",
				"optional object flag with schema must use z.object(), not z.string()")
			assert.Contains(t, trimmed, ".optional()",
				"optional object flag MUST have .optional()")
			break
		}
	}
}

func TestTSMCP_Object_CrossCutting_MixedScalarAndObjectFlags(t *testing.T) {
	// A tool with both a scalar flag and an object flag: each must use the
	// correct Zod expression. Scalars use zodType(), objects use itemSchemaToZod()
	// or z.record().
	m := manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:    "mixed-flag-server",
			Version: "1.0.0",
		},
		Tools: []manifest.Tool{
			{
				Name:        "mixed_tool",
				Description: "Tool with mixed flags",
				Entrypoint:  "./mixed.sh",
				Auth:        &manifest.Auth{Type: "none"},
				Flags: []manifest.Flag{
					{
						Name:        "count",
						Type:        "int",
						Required:    true,
						Description: "a scalar int flag",
					},
					{
						Name:        "metadata",
						Type:        "object",
						Required:    false,
						Description: "an unstructured object flag",
					},
					{
						Name:        "options",
						Type:        "object",
						Required:    true,
						Description: "a structured object flag",
						ItemSchema: map[string]any{
							"type": "object",
							"properties": map[string]any{
								"debug": map[string]any{"type": "boolean"},
							},
						},
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
	content := fileContent(t, files, "src/tools/mixed_tool.ts")

	lines := strings.Split(content, "\n")
	var countLine, metadataLine, optionsLine string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "count:") {
			countLine = trimmed
		}
		if strings.HasPrefix(trimmed, "metadata:") {
			metadataLine = trimmed
		}
		if strings.HasPrefix(trimmed, "options:") {
			optionsLine = trimmed
		}
	}

	require.NotEmpty(t, countLine, "must have line for scalar flag 'count'")
	require.NotEmpty(t, metadataLine, "must have line for unstructured object flag 'metadata'")
	require.NotEmpty(t, optionsLine, "must have line for structured object flag 'options'")

	// Scalar flag: z.number(), no z.record or z.object
	assert.Contains(t, countLine, "z.number()",
		"scalar int flag must use z.number()")

	// Unstructured object (no itemSchema): z.record(z.unknown())
	assert.Contains(t, metadataLine, "z.record(z.unknown())",
		"unstructured object flag must use z.record(z.unknown())")

	// Structured object (with itemSchema): z.object(...)
	assert.Contains(t, optionsLine, "z.object(",
		"structured object flag must use z.object()")
}

func TestTSMCP_Object_CrossCutting_OptionalObjectRecord_HasOptional(t *testing.T) {
	// An optional object flag with no schema must emit z.record(z.unknown()).optional()
	m := mcpManifestObjectFlagWithSchema("object", false, nil)
	files := generateTSMCP(t, m)
	content := fileContent(t, files, "src/tools/obj_tool.ts")

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "config:") {
			assert.Contains(t, trimmed, "z.record(z.unknown()).optional()",
				"optional object flag with no schema must emit z.record(z.unknown()).optional()")
			break
		}
	}
}

func TestTSMCP_Object_CrossCutting_RequiredObjectRecord_NoOptional(t *testing.T) {
	// A required object flag with no schema must emit z.record(z.unknown()) without .optional()
	m := mcpManifestObjectFlagWithSchema("object", true, nil)
	files := generateTSMCP(t, m)
	content := fileContent(t, files, "src/tools/obj_tool.ts")

	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "config:") {
			assert.Contains(t, trimmed, "z.record(z.unknown())",
				"required object flag with no schema must use z.record(z.unknown())")
			assert.NotContains(t, trimmed, ".optional()",
				"required object flag must NOT have .optional()")
			break
		}
	}
}
