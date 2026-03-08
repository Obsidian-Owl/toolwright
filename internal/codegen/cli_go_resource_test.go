package codegen

import (
	"strings"
	"testing"

	"github.com/Obsidian-Owl/toolwright/internal/manifest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Test manifests for CLI resource-ignorance tests
// ---------------------------------------------------------------------------

// baseToolsForResourceTests returns the common tool set used across
// resource-ignorance tests. Using the same tools in both "with" and "without"
// resource manifests ensures any difference in output is attributable solely
// to the Resources field.
func baseToolsForResourceTests() []manifest.Tool {
	return []manifest.Tool{
		{
			Name:        "status",
			Description: "Check service status",
			Entrypoint:  "./status.sh",
		},
		{
			Name:        "deploy",
			Description: "Deploy the service",
			Entrypoint:  "./deploy.sh",
			Auth: &manifest.Auth{
				Type:      "token",
				TokenEnv:  "DEPLOY_TOKEN",
				TokenFlag: "--token",
			},
			Flags: []manifest.Flag{
				{Name: "env", Type: "string", Required: true, Description: "Target environment"},
			},
		},
	}
}

// manifestWithoutResources returns a manifest with two tools and no resources.
func manifestWithoutResources() manifest.Toolkit {
	return manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:        "resource-test-toolkit",
			Version:     "1.0.0",
			Description: "Toolkit for testing resource ignorance",
		},
		Tools: baseToolsForResourceTests(),
	}
}

// manifestWithResources returns the same manifest as manifestWithoutResources
// but with resources added. The tools are identical.
func manifestWithResources() manifest.Toolkit {
	m := manifestWithoutResources()
	m.Resources = []manifest.Resource{
		{
			URI:         "file://{path}",
			Name:        "file_reader",
			Description: "Read a file by path",
			MimeType:    "text/plain",
			Entrypoint:  "./read_file.sh",
		},
	}
	return m
}

// manifestWithThreeResources returns the same manifest but with 3 resources.
func manifestWithThreeResources() manifest.Toolkit {
	m := manifestWithoutResources()
	m.Resources = []manifest.Resource{
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
	}
	return m
}

// ---------------------------------------------------------------------------
// AC10: CLI generation ignores resources
// ---------------------------------------------------------------------------

func TestGoCLI_Resource_IdenticalOutput_WithAndWithout(t *testing.T) {
	// Generate CLI from the manifest WITHOUT resources.
	filesWithout := generateCLI(t, manifestWithoutResources())
	// Generate CLI from the SAME manifest WITH resources.
	filesWith := generateCLI(t, manifestWithResources())

	require.Equal(t, len(filesWithout), len(filesWith),
		"file count must be identical with and without resources")

	// Build a map path->content for the no-resource version.
	withoutMap := make(map[string]string, len(filesWithout))
	for _, f := range filesWithout {
		withoutMap[f.Path] = string(f.Content)
	}

	// Every file from the with-resources version must match exactly.
	for _, f := range filesWith {
		expected, ok := withoutMap[f.Path]
		require.True(t, ok,
			"file %q exists in with-resources output but not in without-resources output", f.Path)
		assert.Equal(t, expected, string(f.Content),
			"file %q must be byte-identical with and without resources", f.Path)
	}
}

func TestGoCLI_Resource_FileCountUnchanged(t *testing.T) {
	filesWithout := generateCLI(t, manifestWithoutResources())
	filesWith := generateCLI(t, manifestWithResources())

	assert.Equal(t, len(filesWithout), len(filesWith),
		"number of generated files must be the same with and without resources")
}

func TestGoCLI_Resource_FilePathsIdentical(t *testing.T) {
	filesWithout := generateCLI(t, manifestWithoutResources())
	filesWith := generateCLI(t, manifestWithResources())

	pathsWithout := filePaths(filesWithout)
	pathsWith := filePaths(filesWith)

	assert.Equal(t, pathsWithout, pathsWith,
		"generated file paths must be identical with and without resources")
}

func TestGoCLI_Resource_NoResourceCommands_InRootGo(t *testing.T) {
	// The root.go file must NOT reference any resource names as commands.
	files := generateCLI(t, manifestWithResources())
	root := fileContent(t, files, "internal/commands/root.go")

	assert.NotContains(t, root, "file_reader",
		"root.go must NOT contain resource name 'file_reader' as a command")
	assert.NotContains(t, root, "read_file",
		"root.go must NOT contain resource entrypoint reference 'read_file'")
}

func TestGoCLI_Resource_NoResourceFlags(t *testing.T) {
	// No generated file should contain flags related to resources.
	files := generateCLI(t, manifestWithResources())

	for _, f := range files {
		content := string(f.Content)
		assert.NotContains(t, content, "file://{path}",
			"file %q must NOT contain resource URI template 'file://{path}'", f.Path)
		assert.NotContains(t, content, "--uri",
			"file %q must NOT contain a --uri flag from resources", f.Path)
		assert.NotContains(t, content, `"mimeType"`,
			"file %q must NOT contain resource mimeType field as a flag", f.Path)
	}
}

func TestGoCLI_Resource_ResourceNamesDoNotAppear(t *testing.T) {
	// Resource-specific identifiers (URI, mimeType, entrypoint, name) must
	// not appear anywhere in the generated CLI code.
	files := generateCLI(t, manifestWithResources())

	resourceStrings := []string{
		"file://{path}",  // URI
		"text/plain",     // mimeType (resource-specific, not used by tools in this manifest)
		"./read_file.sh", // entrypoint
		"file_reader",    // resource name
	}

	for _, f := range files {
		content := string(f.Content)
		for _, rs := range resourceStrings {
			assert.NotContains(t, content, rs,
				"file %q must NOT contain resource-specific string %q", f.Path, rs)
		}
	}
}

func TestGoCLI_Resource_NoResourceHandlerFiles(t *testing.T) {
	// The CLI generator must NOT produce files in a resources/ directory
	// or files named after resource names.
	files := generateCLI(t, manifestWithResources())

	for _, f := range files {
		assert.False(t, strings.Contains(f.Path, "/resources/"),
			"no generated file should be in a resources/ directory; found: %s", f.Path)
		assert.NotEqual(t, "internal/commands/file_reader.go", f.Path,
			"no generated file should be a resource handler; found: %s", f.Path)
	}
}

// ---------------------------------------------------------------------------
// AC10: Multiple resources still ignored
// ---------------------------------------------------------------------------

func TestGoCLI_Resource_ThreeResourcesIdenticalToNone(t *testing.T) {
	// Even with 3 resources, the CLI output must be identical to no resources.
	filesWithout := generateCLI(t, manifestWithoutResources())
	filesWith := generateCLI(t, manifestWithThreeResources())

	require.Equal(t, len(filesWithout), len(filesWith),
		"file count must be identical with 3 resources vs no resources")

	withoutMap := make(map[string]string, len(filesWithout))
	for _, f := range filesWithout {
		withoutMap[f.Path] = string(f.Content)
	}

	for _, f := range filesWith {
		expected, ok := withoutMap[f.Path]
		require.True(t, ok,
			"file %q exists with 3 resources but not without; extra file generated", f.Path)
		assert.Equal(t, expected, string(f.Content),
			"file %q must be byte-identical with 3 resources and without any", f.Path)
	}
}

func TestGoCLI_Resource_ThreeResourceNamesDoNotAppear(t *testing.T) {
	files := generateCLI(t, manifestWithThreeResources())

	resourceStrings := []string{
		// Resource 1
		"file_reader", "file://{path}", "./read_file.sh",
		// Resource 2
		"db_record", "db://{table}/{id}", "./db_fetch.sh",
		// Resource 3
		"config_value", "config://{key}", "./read_config.sh",
	}

	for _, f := range files {
		content := string(f.Content)
		for _, rs := range resourceStrings {
			assert.NotContains(t, content, rs,
				"file %q must NOT contain resource string %q (3-resource manifest)", f.Path, rs)
		}
	}
}

func TestGoCLI_Resource_ThreeResources_NoResourceCommandRegistrations(t *testing.T) {
	// root.go must not register any commands for resources.
	files := generateCLI(t, manifestWithThreeResources())
	root := fileContent(t, files, "internal/commands/root.go")

	// Count AddCommand calls -- should only be list + describe + tool commands.
	addCmdCount := strings.Count(root, "AddCommand")
	// We have 2 base commands (list, describe) in init(). No resource commands.
	// The tool commands are registered in their own init() functions in tool files.
	assert.Equal(t, 2, addCmdCount,
		"root.go must have exactly 2 AddCommand calls (list, describe), no resource commands; got %d", addCmdCount)
}

// ---------------------------------------------------------------------------
// AC11: Existing manifests without resources unchanged
// ---------------------------------------------------------------------------

func TestGoCLI_Resource_NoResourceField_GeneratesNormally(t *testing.T) {
	// A manifest with zero-value Resources (nil) generates the expected files.
	m := manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:        "plain-toolkit",
			Version:     "1.0.0",
			Description: "A plain toolkit",
		},
		Tools: []manifest.Tool{
			{
				Name:        "greet",
				Description: "Greet the user",
				Entrypoint:  "./greet.sh",
			},
		},
	}

	files := generateCLI(t, m)

	// Must contain core files.
	require.NotNil(t, findFile(files, "cmd/plain-toolkit/main.go"),
		"main.go must be generated for manifest without resources")
	require.NotNil(t, findFile(files, "internal/commands/root.go"),
		"root.go must be generated for manifest without resources")
	require.NotNil(t, findFile(files, "internal/commands/greet.go"),
		"greet.go must be generated for manifest without resources")
	require.NotNil(t, findFile(files, "go.mod"),
		"go.mod must be generated for manifest without resources")

	// greet.go must have the tool command.
	greetContent := fileContent(t, files, "internal/commands/greet.go")
	assert.Contains(t, greetContent, "greet",
		"greet.go must contain the tool name")
	assert.Contains(t, greetContent, "exec.CommandContext",
		"greet.go must execute the entrypoint via exec.CommandContext")
}

func TestGoCLI_Resource_NilVsEmptyResources_IdenticalOutput(t *testing.T) {
	// A manifest with nil Resources and one with an empty slice []Resource{}
	// must produce identical output.
	mNil := manifestWithoutResources()
	mNil.Resources = nil

	mEmpty := manifestWithoutResources()
	mEmpty.Resources = []manifest.Resource{}

	filesNil := generateCLI(t, mNil)
	filesEmpty := generateCLI(t, mEmpty)

	require.Equal(t, len(filesNil), len(filesEmpty),
		"file count must be identical for nil vs empty resources")

	nilMap := make(map[string]string, len(filesNil))
	for _, f := range filesNil {
		nilMap[f.Path] = string(f.Content)
	}

	for _, f := range filesEmpty {
		expected, ok := nilMap[f.Path]
		require.True(t, ok,
			"file %q exists with empty resources but not with nil", f.Path)
		assert.Equal(t, expected, string(f.Content),
			"file %q must be byte-identical between nil and empty resources", f.Path)
	}
}

// ---------------------------------------------------------------------------
// Table-driven: resource data not leaked into any generated file
// (Constitution 9: table-driven tests)
// ---------------------------------------------------------------------------

func TestGoCLI_Resource_NoResourceDataLeaked(t *testing.T) {
	tests := []struct {
		name      string
		resources []manifest.Resource
		forbidden []string // strings that must NOT appear in any generated file
	}{
		{
			name: "single_resource",
			resources: []manifest.Resource{
				{
					URI:         "file://{path}",
					Name:        "file_reader",
					Description: "Read a file by path",
					MimeType:    "text/plain",
					Entrypoint:  "./read_file.sh",
				},
			},
			forbidden: []string{
				"file://{path}", "file_reader", "./read_file.sh",
			},
		},
		{
			name: "resource_with_json_mime",
			resources: []manifest.Resource{
				{
					URI:         "api://{endpoint}",
					Name:        "api_proxy",
					Description: "Proxy an API call",
					MimeType:    "application/json",
					Entrypoint:  "./api_proxy.py",
				},
			},
			forbidden: []string{
				"api://{endpoint}", "api_proxy", "./api_proxy.py",
			},
		},
		{
			name: "resource_without_mimetype",
			resources: []manifest.Resource{
				{
					URI:         "log://{id}",
					Name:        "log_entry",
					Description: "Read a log entry",
					Entrypoint:  "./read_log.sh",
				},
			},
			forbidden: []string{
				"log://{id}", "log_entry", "./read_log.sh",
			},
		},
		{
			name: "three_resources",
			resources: []manifest.Resource{
				{
					URI: "file://{path}", Name: "file_reader",
					Description: "Read a file", MimeType: "text/plain",
					Entrypoint: "./read_file.sh",
				},
				{
					URI: "db://{table}/{id}", Name: "db_record",
					Description: "Fetch a record", MimeType: "application/json",
					Entrypoint: "./db_fetch.sh",
				},
				{
					URI: "config://{key}", Name: "config_value",
					Description: "Read config", MimeType: "text/plain",
					Entrypoint: "./read_config.sh",
				},
			},
			forbidden: []string{
				"file_reader", "db_record", "config_value",
				"file://{path}", "db://{table}/{id}", "config://{key}",
				"./read_file.sh", "./db_fetch.sh", "./read_config.sh",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := manifestWithoutResources()
			m.Resources = tc.resources

			files := generateCLI(t, m)

			for _, f := range files {
				content := string(f.Content)
				for _, s := range tc.forbidden {
					assert.NotContains(t, content, s,
						"file %q must NOT contain resource data %q (test: %s)", f.Path, s, tc.name)
				}
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Adversarial: prevent lazy implementations that iterate resources
// ---------------------------------------------------------------------------

func TestGoCLI_Resource_NoToolFilePerResource(t *testing.T) {
	// A lazy implementation might generate a command file for each resource.
	// Verify that ONLY tool names produce command files, not resource names.
	files := generateCLI(t, manifestWithThreeResources())

	// Expected tool command files.
	require.NotNil(t, findFile(files, "internal/commands/status.go"),
		"tool status.go must exist")
	require.NotNil(t, findFile(files, "internal/commands/deploy.go"),
		"tool deploy.go must exist")

	// Resource names must NOT produce command files.
	assertNoFile(t, files, "internal/commands/file_reader.go")
	assertNoFile(t, files, "internal/commands/db_record.go")
	assertNoFile(t, files, "internal/commands/config_value.go")
	// Also check for hyphenated variants.
	assertNoFile(t, files, "internal/commands/file-reader.go")
	assertNoFile(t, files, "internal/commands/db-record.go")
	assertNoFile(t, files, "internal/commands/config-value.go")
}

func TestGoCLI_Resource_RootRegistryOnlyListsTools(t *testing.T) {
	// The registry in root.go must list only tools, not resources.
	files := generateCLI(t, manifestWithThreeResources())
	root := fileContent(t, files, "internal/commands/root.go")

	// Tools from the manifest should be in the registry.
	assert.Contains(t, root, `"status"`,
		"root.go registry must include tool 'status'")
	assert.Contains(t, root, `"deploy"`,
		"root.go registry must include tool 'deploy'")

	// Resources must NOT appear in the registry.
	assert.NotContains(t, root, "file_reader",
		"root.go registry must NOT include resource 'file_reader'")
	assert.NotContains(t, root, "db_record",
		"root.go registry must NOT include resource 'db_record'")
	assert.NotContains(t, root, "config_value",
		"root.go registry must NOT include resource 'config_value'")
}

func TestGoCLI_Resource_ToolCommandsUnaffected(t *testing.T) {
	// Tool command files must still have their expected content even when
	// resources are present. This verifies resources don't corrupt tool codegen.
	files := generateCLI(t, manifestWithResources())

	statusContent := fileContent(t, files, "internal/commands/status.go")
	deployContent := fileContent(t, files, "internal/commands/deploy.go")

	// status tool (no auth, no flags).
	assert.Contains(t, statusContent, "package commands",
		"status.go must have package declaration")
	assert.Contains(t, statusContent, "exec.CommandContext",
		"status.go must execute entrypoint")
	assert.Contains(t, statusContent, "RunE:",
		"status.go must have RunE")

	// deploy tool (token auth, flags).
	assert.Contains(t, deployContent, "package commands",
		"deploy.go must have package declaration")
	assert.Contains(t, deployContent, "exec.CommandContext",
		"deploy.go must execute entrypoint")
	assert.Contains(t, deployContent, "DEPLOY_TOKEN",
		"deploy.go must reference token env var")
	assert.Contains(t, deployContent, `"env"`,
		"deploy.go must have the env flag")
}
