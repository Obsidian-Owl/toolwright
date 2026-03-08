package codegen

import (
	"strings"
	"testing"

	"github.com/Obsidian-Owl/toolwright/internal/manifest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Test manifests for MCP resource codegen
// ---------------------------------------------------------------------------

// mcpManifestSingleResource returns a manifest with one resource and one tool.
func mcpManifestSingleResource() manifest.Toolkit {
	return manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:        "resource-server",
			Version:     "1.0.0",
			Description: "MCP server with a resource",
		},
		Tools: []manifest.Tool{
			{
				Name:        "get_weather",
				Description: "Get weather for a location",
				Entrypoint:  "./weather.sh",
			},
		},
		Resources: []manifest.Resource{
			{
				URI:         "file://{path}",
				Name:        "file_reader",
				Description: "Read a file by path",
				MimeType:    "text/plain",
				Entrypoint:  "./read_file.sh",
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

// mcpManifestThreeResources returns a manifest with 3 resources for AC9 testing.
func mcpManifestThreeResources() manifest.Toolkit {
	return manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:        "multi-resource-server",
			Version:     "1.0.0",
			Description: "MCP server with multiple resources",
		},
		Tools: []manifest.Tool{
			{
				Name:        "ping",
				Description: "Ping the server",
				Entrypoint:  "./ping.sh",
			},
		},
		Resources: []manifest.Resource{
			{
				URI:         "file://{path}",
				Name:        "file_reader",
				Description: "Read a file by path",
				MimeType:    "text/plain",
				Entrypoint:  "./read_file.sh",
			},
			{
				URI:         "db://{table}/{id}",
				Name:        "db_record",
				Description: "Fetch a database record",
				MimeType:    "application/json",
				Entrypoint:  "./db_fetch.sh",
			},
			{
				URI:         "config://{key}",
				Name:        "config_value",
				Description: "Read a configuration value",
				MimeType:    "text/plain",
				Entrypoint:  "./read_config.sh",
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

// mcpManifestNoResources returns a manifest with tools but no resources.
func mcpManifestNoResources() manifest.Toolkit {
	return manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:        "tools-only-server",
			Version:     "1.0.0",
			Description: "MCP server with tools only",
		},
		Tools: []manifest.Tool{
			{
				Name:        "greet",
				Description: "Greet a user",
				Entrypoint:  "./greet.sh",
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

// mcpManifestResourceNoMimeType returns a manifest with a resource that has
// no mimeType, verifying the default behaviour.
func mcpManifestResourceNoMimeType() manifest.Toolkit {
	return manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:        "no-mime-server",
			Version:     "1.0.0",
			Description: "MCP server with resource missing mimeType",
		},
		Tools: []manifest.Tool{
			{
				Name:        "noop",
				Description: "No-op tool",
				Entrypoint:  "./noop.sh",
			},
		},
		Resources: []manifest.Resource{
			{
				URI:         "log://{id}",
				Name:        "log_entry",
				Description: "Read a log entry",
				Entrypoint:  "./read_log.sh",
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

// mcpManifestResourceMultiParams returns a manifest with a resource whose URI
// template has multiple parameters.
func mcpManifestResourceMultiParams() manifest.Toolkit {
	return manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:        "multi-param-server",
			Version:     "1.0.0",
			Description: "MCP server with multi-param resource",
		},
		Tools: []manifest.Tool{
			{
				Name:        "noop",
				Description: "No-op tool",
				Entrypoint:  "./noop.sh",
			},
		},
		Resources: []manifest.Resource{
			{
				URI:         "repo://{owner}/{repo}/issues/{number}",
				Name:        "github_issue",
				Description: "Fetch a GitHub issue",
				MimeType:    "application/json",
				Entrypoint:  "./fetch_issue.sh",
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
// AC7: Resources registered in MCP server
// ---------------------------------------------------------------------------

func TestTSMCP_Resource_IndexImportsResourceHandler(t *testing.T) {
	// Index.ts must import each resource handler file so that the registration
	// function is available.
	m := mcpManifestSingleResource()
	files := generateTSMCP(t, m)
	index := fileContent(t, files, "src/index.ts")

	assert.Contains(t, index, "file_reader",
		"index.ts must import the file_reader resource handler")
	assert.Contains(t, index, "resources/file_reader",
		"index.ts must import resource handler from resources/ directory")
}

func TestTSMCP_Resource_IndexCallsServerResource(t *testing.T) {
	// Index.ts must call server.resource() for each resource.
	m := mcpManifestSingleResource()
	files := generateTSMCP(t, m)
	index := fileContent(t, files, "src/index.ts")

	assert.Contains(t, index, "server.resource(",
		"index.ts must contain server.resource() call to register the resource")
}

func TestTSMCP_Resource_IndexRegistrationContainsURI(t *testing.T) {
	// The server.resource() call must include the URI template from the manifest.
	m := mcpManifestSingleResource()
	files := generateTSMCP(t, m)
	index := fileContent(t, files, "src/index.ts")

	// The URI template must appear in the index (passed to server.resource).
	assert.Contains(t, index, "file://{path}",
		"index.ts server.resource() call must include the URI template 'file://{path}'")
}

func TestTSMCP_Resource_IndexRegistrationContainsName(t *testing.T) {
	// The server.resource() call must include the resource name.
	m := mcpManifestSingleResource()
	files := generateTSMCP(t, m)
	index := fileContent(t, files, "src/index.ts")

	// Find the server.resource section and confirm the name appears there.
	resIdx := strings.Index(index, "server.resource(")
	require.Greater(t, resIdx, 0,
		"server.resource( must be present in index.ts")
	afterRes := index[resIdx:]
	assert.Contains(t, afterRes, "file_reader",
		"server.resource() call must include the resource name 'file_reader'")
}

func TestTSMCP_Resource_RegistrationIncludesResourceNameString(t *testing.T) {
	// Use a different resource name to catch hardcoding.
	m := mcpManifestThreeResources()
	files := generateTSMCP(t, m)
	index := fileContent(t, files, "src/index.ts")

	// All three names must appear in server.resource() calls.
	assert.Contains(t, index, `"file_reader"`,
		"index.ts must contain quoted resource name 'file_reader'")
	assert.Contains(t, index, `"db_record"`,
		"index.ts must contain quoted resource name 'db_record'")
	assert.Contains(t, index, `"config_value"`,
		"index.ts must contain quoted resource name 'config_value'")
}

// ---------------------------------------------------------------------------
// AC8: Resource handler calls entrypoint
// ---------------------------------------------------------------------------

func TestTSMCP_Resource_HandlerFileExists(t *testing.T) {
	// A resource handler file must be generated at src/resources/{name}.ts.
	m := mcpManifestSingleResource()
	files := generateTSMCP(t, m)

	f := findFile(files, "src/resources/file_reader.ts")
	require.NotNil(t, f,
		"resource handler file must exist at src/resources/file_reader.ts; got paths: %v",
		filePaths(files))
}

func TestTSMCP_Resource_HandlerContainsEntrypoint(t *testing.T) {
	// The handler must reference the entrypoint from the manifest so the
	// runtime can invoke the backing script.
	m := mcpManifestSingleResource()
	files := generateTSMCP(t, m)
	content := fileContent(t, files, "src/resources/file_reader.ts")

	assert.Contains(t, content, "./read_file.sh",
		"resource handler must contain the entrypoint path './read_file.sh'")
}

func TestTSMCP_Resource_HandlerContainsMimeType(t *testing.T) {
	// The handler must reference the mimeType so the response includes it.
	m := mcpManifestSingleResource()
	files := generateTSMCP(t, m)
	content := fileContent(t, files, "src/resources/file_reader.ts")

	assert.Contains(t, content, "text/plain",
		"resource handler must contain the mimeType 'text/plain'")
}

func TestTSMCP_Resource_HandlerContainsMimeTypeField(t *testing.T) {
	// The handler must set a mimeType field in the returned resource content.
	m := mcpManifestSingleResource()
	files := generateTSMCP(t, m)
	content := fileContent(t, files, "src/resources/file_reader.ts")

	assert.Contains(t, content, "mimeType",
		"resource handler must set a mimeType field in the resource content")
}

func TestTSMCP_Resource_HandlerReferencesURIParam(t *testing.T) {
	// The handler must extract parameters from the URI template.
	// For URI "file://{path}", the handler must reference "path".
	m := mcpManifestSingleResource()
	files := generateTSMCP(t, m)
	content := fileContent(t, files, "src/resources/file_reader.ts")

	assert.Contains(t, content, "path",
		"resource handler for URI 'file://{path}' must reference the 'path' parameter")
}

func TestTSMCP_Resource_HandlerMimeTypeFromManifest(t *testing.T) {
	// The mimeType in the handler must come from the manifest, not be hardcoded.
	// Use a distinctive MIME type that no default would match.
	m := mcpManifestSingleResource()
	m.Resources[0].MimeType = "application/x-custom-format"
	files := generateTSMCP(t, m)
	content := fileContent(t, files, "src/resources/file_reader.ts")

	assert.Contains(t, content, "application/x-custom-format",
		"resource handler mimeType must come from manifest, not be hardcoded")
}

func TestTSMCP_Resource_HandlerEntrypointFromManifest(t *testing.T) {
	// The entrypoint must come from the manifest. Use a distinctive path.
	m := mcpManifestSingleResource()
	m.Resources[0].Entrypoint = "./custom_reader.py"
	files := generateTSMCP(t, m)
	content := fileContent(t, files, "src/resources/file_reader.ts")

	assert.Contains(t, content, "./custom_reader.py",
		"resource handler entrypoint must come from manifest, not hardcoded")
	assert.NotContains(t, content, "./read_file.sh",
		"changed entrypoint must not still reference the old entrypoint")
}

func TestTSMCP_Resource_HandlerMultipleURIParams(t *testing.T) {
	// A URI template with multiple parameters must have all params extracted
	// in the handler. URI: "repo://{owner}/{repo}/issues/{number}"
	m := mcpManifestResourceMultiParams()
	files := generateTSMCP(t, m)
	content := fileContent(t, files, "src/resources/github_issue.ts")

	assert.Contains(t, content, "owner",
		"handler for multi-param URI must reference 'owner' parameter")
	assert.Contains(t, content, "repo",
		"handler for multi-param URI must reference 'repo' parameter")
	assert.Contains(t, content, "number",
		"handler for multi-param URI must reference 'number' parameter")
}

func TestTSMCP_Resource_HandlerPassesParamsToEntrypoint(t *testing.T) {
	// The handler must pass URI params as arguments to the entrypoint.
	// For a single-param resource, the param should appear in the exec/spawn call
	// alongside the entrypoint.
	m := mcpManifestSingleResource()
	files := generateTSMCP(t, m)
	content := fileContent(t, files, "src/resources/file_reader.ts")

	// The handler must have both the entrypoint and the parameter in a way
	// that suggests the param is passed to the entrypoint call.
	hasEntrypoint := strings.Contains(content, "./read_file.sh")
	hasParam := strings.Contains(content, "path")
	require.True(t, hasEntrypoint && hasParam,
		"handler must contain both entrypoint and URI param; entrypoint=%v, param=%v",
		hasEntrypoint, hasParam)
}

func TestTSMCP_Resource_HandlerReturnsResourceContent(t *testing.T) {
	// The resource handler must return content with a structure that includes
	// the resource contents. MCP resources return { contents: [...] }.
	m := mcpManifestSingleResource()
	files := generateTSMCP(t, m)
	content := fileContent(t, files, "src/resources/file_reader.ts")

	assert.Contains(t, content, "contents",
		"resource handler must return a structure with 'contents' array")
}

func TestTSMCP_Resource_HandlerContainsURIInResponse(t *testing.T) {
	// MCP resource responses include the URI. The handler should reference it.
	m := mcpManifestSingleResource()
	files := generateTSMCP(t, m)
	content := fileContent(t, files, "src/resources/file_reader.ts")

	assert.Contains(t, content, "uri",
		"resource handler response must include a 'uri' field")
}

// ---------------------------------------------------------------------------
// AC9: Multiple resources generate correctly
// ---------------------------------------------------------------------------

func TestTSMCP_Resource_ThreeResourcesProduceThreeHandlerFiles(t *testing.T) {
	// 3 resources in the manifest must produce 3 handler files.
	m := mcpManifestThreeResources()
	files := generateTSMCP(t, m)

	paths := filePaths(files)

	f1 := findFile(files, "src/resources/file_reader.ts")
	require.NotNilf(t, f1, "file_reader handler must exist; got paths: %v", paths)

	f2 := findFile(files, "src/resources/db_record.ts")
	require.NotNilf(t, f2, "db_record handler must exist; got paths: %v", paths)

	f3 := findFile(files, "src/resources/config_value.ts")
	require.NotNilf(t, f3, "config_value handler must exist; got paths: %v", paths)
}

func TestTSMCP_Resource_ThreeResourcesProduceThreeRegistrations(t *testing.T) {
	// 3 resources must produce 3 server.resource() calls in index.ts.
	m := mcpManifestThreeResources()
	files := generateTSMCP(t, m)
	index := fileContent(t, files, "src/index.ts")

	count := strings.Count(index, "server.resource(")
	assert.Equal(t, 3, count,
		"index.ts must contain exactly 3 server.resource() calls for 3 resources; got %d", count)
}

func TestTSMCP_Resource_EachHandlerHasOwnEntrypoint(t *testing.T) {
	// Each handler must reference its own entrypoint, not share one.
	m := mcpManifestThreeResources()
	files := generateTSMCP(t, m)

	c1 := fileContent(t, files, "src/resources/file_reader.ts")
	c2 := fileContent(t, files, "src/resources/db_record.ts")
	c3 := fileContent(t, files, "src/resources/config_value.ts")

	assert.Contains(t, c1, "./read_file.sh",
		"file_reader handler must reference its own entrypoint")
	assert.Contains(t, c2, "./db_fetch.sh",
		"db_record handler must reference its own entrypoint")
	assert.Contains(t, c3, "./read_config.sh",
		"config_value handler must reference its own entrypoint")

	// Cross-contamination check: each handler must NOT contain another's entrypoint.
	assert.NotContains(t, c1, "./db_fetch.sh",
		"file_reader handler must not contain db_record's entrypoint")
	assert.NotContains(t, c1, "./read_config.sh",
		"file_reader handler must not contain config_value's entrypoint")
	assert.NotContains(t, c2, "./read_file.sh",
		"db_record handler must not contain file_reader's entrypoint")
	assert.NotContains(t, c2, "./read_config.sh",
		"db_record handler must not contain config_value's entrypoint")
	assert.NotContains(t, c3, "./read_file.sh",
		"config_value handler must not contain file_reader's entrypoint")
	assert.NotContains(t, c3, "./db_fetch.sh",
		"config_value handler must not contain db_record's entrypoint")
}

func TestTSMCP_Resource_EachHandlerHasOwnMimeType(t *testing.T) {
	// Each handler must reference its own mimeType.
	m := mcpManifestThreeResources()
	files := generateTSMCP(t, m)

	c1 := fileContent(t, files, "src/resources/file_reader.ts")
	c2 := fileContent(t, files, "src/resources/db_record.ts")
	c3 := fileContent(t, files, "src/resources/config_value.ts")

	assert.Contains(t, c1, "text/plain",
		"file_reader handler must have mimeType text/plain")
	assert.Contains(t, c2, "application/json",
		"db_record handler must have mimeType application/json")
	assert.Contains(t, c3, "text/plain",
		"config_value handler must have mimeType text/plain")
}

func TestTSMCP_Resource_EachHandlerHasOwnURI(t *testing.T) {
	// Each resource registration in index.ts must use the correct URI template.
	m := mcpManifestThreeResources()
	files := generateTSMCP(t, m)
	index := fileContent(t, files, "src/index.ts")

	assert.Contains(t, index, "file://{path}",
		"index.ts must contain URI template 'file://{path}'")
	assert.Contains(t, index, "db://{table}/{id}",
		"index.ts must contain URI template 'db://{table}/{id}'")
	assert.Contains(t, index, "config://{key}",
		"index.ts must contain URI template 'config://{key}'")
}

func TestTSMCP_Resource_ThreeResourcesThreeImports(t *testing.T) {
	// Index.ts must import all three resource handlers.
	m := mcpManifestThreeResources()
	files := generateTSMCP(t, m)
	index := fileContent(t, files, "src/index.ts")

	assert.Contains(t, index, "resources/file_reader",
		"index.ts must import file_reader from resources/")
	assert.Contains(t, index, "resources/db_record",
		"index.ts must import db_record from resources/")
	assert.Contains(t, index, "resources/config_value",
		"index.ts must import config_value from resources/")
}

// ---------------------------------------------------------------------------
// Edge cases
// ---------------------------------------------------------------------------

func TestTSMCP_Resource_NoResources_NoServerResourceCall(t *testing.T) {
	// A manifest with no resources must NOT generate any server.resource() calls.
	m := mcpManifestNoResources()
	files := generateTSMCP(t, m)
	index := fileContent(t, files, "src/index.ts")

	assert.NotContains(t, index, "server.resource(",
		"index.ts must NOT contain server.resource() when manifest has no resources")
}

func TestTSMCP_Resource_NoResources_NoResourceImports(t *testing.T) {
	// A manifest with no resources must NOT import from the resources/ directory.
	m := mcpManifestNoResources()
	files := generateTSMCP(t, m)
	index := fileContent(t, files, "src/index.ts")

	assert.NotContains(t, index, "/resources/",
		"index.ts must NOT contain resource imports when manifest has no resources")
}

func TestTSMCP_Resource_NoResources_NoResourceHandlerFiles(t *testing.T) {
	// A manifest with no resources must NOT generate any files in src/resources/.
	m := mcpManifestNoResources()
	files := generateTSMCP(t, m)

	for _, f := range files {
		assert.False(t, strings.HasPrefix(f.Path, "src/resources/"),
			"no resource handler files should exist when manifest has no resources; found: %s", f.Path)
	}
}

func TestTSMCP_Resource_ToolsAndResourcesCoexist(t *testing.T) {
	// A manifest with both tools and resources must generate both tool and
	// resource registrations in index.ts.
	m := mcpManifestSingleResource()
	files := generateTSMCP(t, m)
	index := fileContent(t, files, "src/index.ts")

	// Tools must still be registered.
	assert.Contains(t, index, "server.tool(",
		"index.ts must contain server.tool() even when resources are present")
	assert.Contains(t, index, "tools/get_weather",
		"index.ts must import tool handler even when resources are present")

	// Resources must also be registered.
	assert.Contains(t, index, "server.resource(",
		"index.ts must contain server.resource() alongside tool registrations")
	assert.Contains(t, index, "resources/file_reader",
		"index.ts must import resource handler alongside tool imports")
}

func TestTSMCP_Resource_ToolHandlerStillExists(t *testing.T) {
	// Tool handler files must still be generated when resources are present.
	m := mcpManifestSingleResource()
	files := generateTSMCP(t, m)

	f := findFile(files, "src/tools/get_weather.ts")
	require.NotNil(t, f,
		"tool handler must still exist at src/tools/get_weather.ts when resources are present")
}

func TestTSMCP_Resource_NoMimeType_DefaultsToTextPlain(t *testing.T) {
	// When a resource has no mimeType in the manifest, the handler must
	// default to "text/plain".
	m := mcpManifestResourceNoMimeType()
	files := generateTSMCP(t, m)
	content := fileContent(t, files, "src/resources/log_entry.ts")

	assert.Contains(t, content, "text/plain",
		"resource handler with no mimeType must default to 'text/plain'")
}

func TestTSMCP_Resource_NoMimeType_HandlerStillHasMimeTypeField(t *testing.T) {
	// Even without an explicit mimeType, the handler must set the mimeType field.
	m := mcpManifestResourceNoMimeType()
	files := generateTSMCP(t, m)
	content := fileContent(t, files, "src/resources/log_entry.ts")

	assert.Contains(t, content, "mimeType",
		"resource handler must include mimeType field even when manifest omits it")
}

func TestTSMCP_Resource_MultiParamURI_AllParamsInHandler(t *testing.T) {
	// URI "repo://{owner}/{repo}/issues/{number}" has 3 params.
	// All must be extracted in the handler.
	m := mcpManifestResourceMultiParams()
	files := generateTSMCP(t, m)
	content := fileContent(t, files, "src/resources/github_issue.ts")

	// Each param must appear in the handler logic.
	for _, param := range []string{"owner", "repo", "number"} {
		assert.Contains(t, content, param,
			"handler must reference URI parameter %q from template 'repo://{owner}/{repo}/issues/{number}'", param)
	}
}

func TestTSMCP_Resource_MultiParamURI_AllParamsPassedToEntrypoint(t *testing.T) {
	// All URI params must be passed as arguments to the entrypoint.
	// The handler must have the entrypoint and all param names present.
	m := mcpManifestResourceMultiParams()
	files := generateTSMCP(t, m)
	content := fileContent(t, files, "src/resources/github_issue.ts")

	assert.Contains(t, content, "./fetch_issue.sh",
		"handler must reference the entrypoint")
	// All three params should be present for the call.
	assert.Contains(t, content, "owner",
		"handler must pass 'owner' to entrypoint")
	assert.Contains(t, content, "number",
		"handler must pass 'number' to entrypoint")
}

// ---------------------------------------------------------------------------
// Table-driven: resource registration properties per resource
// (Constitution 9: table-driven tests)
// ---------------------------------------------------------------------------

func TestTSMCP_Resource_RegistrationProperties(t *testing.T) {
	tests := []struct {
		name          string
		resource      manifest.Resource
		wantInIndex   []string // substrings expected in index.ts
		wantInHandler []string // substrings expected in handler file
		handlerPath   string
	}{
		{
			name: "simple_file_resource",
			resource: manifest.Resource{
				URI:         "file://{path}",
				Name:        "file_reader",
				Description: "Read a file",
				MimeType:    "text/plain",
				Entrypoint:  "./read.sh",
			},
			wantInIndex:   []string{"server.resource(", "file://{path}", "file_reader"},
			wantInHandler: []string{"./read.sh", "text/plain", "path", "mimeType"},
			handlerPath:   "src/resources/file_reader.ts",
		},
		{
			name: "json_api_resource",
			resource: manifest.Resource{
				URI:         "api://{endpoint}",
				Name:        "api_proxy",
				Description: "Proxy an API call",
				MimeType:    "application/json",
				Entrypoint:  "./api_proxy.py",
			},
			wantInIndex:   []string{"server.resource(", "api://{endpoint}", "api_proxy"},
			wantInHandler: []string{"./api_proxy.py", "application/json", "endpoint", "mimeType"},
			handlerPath:   "src/resources/api_proxy.ts",
		},
		{
			name: "resource_with_special_mimetype",
			resource: manifest.Resource{
				URI:         "blob://{hash}",
				Name:        "blob_store",
				Description: "Fetch a blob",
				MimeType:    "application/octet-stream",
				Entrypoint:  "./blob.sh",
			},
			wantInIndex:   []string{"server.resource(", "blob://{hash}", "blob_store"},
			wantInHandler: []string{"./blob.sh", "application/octet-stream", "hash", "mimeType"},
			handlerPath:   "src/resources/blob_store.ts",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := manifest.Toolkit{
				APIVersion: "toolwright/v1",
				Kind:       "Toolkit",
				Metadata: manifest.Metadata{
					Name:        tc.name + "-server",
					Version:     "1.0.0",
					Description: "Test server for " + tc.name,
				},
				Tools: []manifest.Tool{
					{
						Name:        "noop",
						Description: "No-op tool",
						Entrypoint:  "./noop.sh",
					},
				},
				Resources: []manifest.Resource{tc.resource},
				Generate: manifest.Generate{
					MCP: manifest.MCPConfig{
						Target:    "typescript",
						Transport: []string{"stdio"},
					},
				},
			}
			files := generateTSMCP(t, m)
			index := fileContent(t, files, "src/index.ts")

			for _, want := range tc.wantInIndex {
				assert.Contains(t, index, want,
					"index.ts must contain %q for resource %q", want, tc.resource.Name)
			}

			handler := fileContent(t, files, tc.handlerPath)
			for _, want := range tc.wantInHandler {
				assert.Contains(t, handler, want,
					"handler %s must contain %q", tc.handlerPath, want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Table-driven: handler file path matches resource name
// ---------------------------------------------------------------------------

func TestTSMCP_Resource_HandlerPathMatchesName(t *testing.T) {
	names := []string{"file_reader", "db_record", "config_value"}

	m := mcpManifestThreeResources()
	files := generateTSMCP(t, m)

	for _, name := range names {
		t.Run(name, func(t *testing.T) {
			expectedPath := "src/resources/" + name + ".ts"
			f := findFile(files, expectedPath)
			require.NotNilf(t, f,
				"resource handler must exist at %q; got paths: %v",
				expectedPath, filePaths(files))
		})
	}
}

// ---------------------------------------------------------------------------
// Determinism: generating twice produces identical output
// ---------------------------------------------------------------------------

func TestTSMCP_Resource_DeterministicOutput(t *testing.T) {
	m := mcpManifestThreeResources()

	files1 := generateTSMCP(t, m)
	files2 := generateTSMCP(t, m)

	// Compare index.ts
	index1 := fileContent(t, files1, "src/index.ts")
	index2 := fileContent(t, files2, "src/index.ts")
	assert.Equal(t, index1, index2,
		"two Generate calls must produce identical index.ts")

	// Compare each handler file
	for _, name := range []string{"file_reader", "db_record", "config_value"} {
		path := "src/resources/" + name + ".ts"
		c1 := fileContent(t, files1, path)
		c2 := fileContent(t, files2, path)
		assert.Equal(t, c1, c2,
			"two Generate calls must produce identical %s", path)
	}
}

// ---------------------------------------------------------------------------
// Registration function pattern in handlers
// ---------------------------------------------------------------------------

func TestTSMCP_Resource_HandlerExportsRegisterFunction(t *testing.T) {
	// Each resource handler should export a register function, matching the
	// pattern used by tool handlers.
	m := mcpManifestSingleResource()
	files := generateTSMCP(t, m)
	content := fileContent(t, files, "src/resources/file_reader.ts")

	// The handler needs some form of exported registration or handler function.
	hasExport := strings.Contains(content, "export")
	assert.True(t, hasExport,
		"resource handler must export a function (register or handler)")
}

func TestTSMCP_Resource_IndexRegistersViaImportedFunction(t *testing.T) {
	// The index.ts file must call the imported resource registration function,
	// not inline the server.resource() call without using the import.
	m := mcpManifestSingleResource()
	files := generateTSMCP(t, m)
	index := fileContent(t, files, "src/index.ts")

	// The import and registration must both be present and connected.
	hasImport := strings.Contains(index, "resources/file_reader")
	hasRegistration := strings.Contains(index, "server.resource(")
	assert.True(t, hasImport && hasRegistration,
		"index.ts must both import resource handler and call server.resource(); import=%v, registration=%v",
		hasImport, hasRegistration)
}

// ---------------------------------------------------------------------------
// Resource handler is not placed in the tools directory
// ---------------------------------------------------------------------------

func TestTSMCP_Resource_HandlerNotInToolsDirectory(t *testing.T) {
	// Resource handlers must be in src/resources/, not src/tools/.
	m := mcpManifestSingleResource()
	files := generateTSMCP(t, m)

	assertNoFile(t, files, "src/tools/file_reader.ts")
}

func TestTSMCP_Resource_ToolNotInResourcesDirectory(t *testing.T) {
	// Tool handlers must remain in src/tools/, not appear in src/resources/.
	m := mcpManifestSingleResource()
	files := generateTSMCP(t, m)

	assertNoFile(t, files, "src/resources/get_weather.ts")
}
