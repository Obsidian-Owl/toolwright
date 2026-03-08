package toolwright_test

import (
	"encoding/json"
	"io/fs"
	"testing"

	"github.com/Obsidian-Owl/toolwright"
	"github.com/Obsidian-Owl/toolwright/internal/schema"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const schemaPath = "schemas/toolwright.schema.json"

// ---------------------------------------------------------------------------
// Helper: load and validate using the production schema.Validator
// ---------------------------------------------------------------------------

// loadSchemaBytes reads the schema file from the embedded FS and returns raw bytes.
func loadSchemaBytes(t *testing.T) []byte {
	t.Helper()
	data, err := fs.ReadFile(toolwright.Schemas, schemaPath)
	require.NoError(t, err, "schema file must be readable from toolwright.Schemas embed.FS at %s", schemaPath)
	require.NotEmpty(t, data, "schema file must not be empty")
	return data
}

// validateJSON validates the given JSON string against the embedded schema
// using the production schema.Validator (kaptinlin/jsonschema).
func validateJSON(t *testing.T, jsonStr string) error {
	t.Helper()
	v := schema.NewValidator(toolwright.Schemas)
	return v.Validate(schemaPath, []byte(jsonStr))
}

// ---------------------------------------------------------------------------
// AC-10: Schema embedded via embed.FS
// ---------------------------------------------------------------------------

func TestSchema_EmbedFS_FileReadable(t *testing.T) {
	// The schema file must be readable at the exact path "schemas/toolwright.schema.json"
	// from the toolwright.Schemas embed.FS. This is the primary embed test.
	data, err := fs.ReadFile(toolwright.Schemas, schemaPath)
	require.NoError(t, err,
		"toolwright.Schemas must contain %s; got read error", schemaPath)
	require.NotEmpty(t, data,
		"schema file at %s must have non-zero content", schemaPath)
}

func TestSchema_EmbedFS_GitkeepRemoved(t *testing.T) {
	// AC-10 specifies .gitkeep should be removed. After the schema file is
	// added, .gitkeep should no longer exist in the schemas directory.
	_, err := fs.ReadFile(toolwright.Schemas, "schemas/.gitkeep")
	assert.Error(t, err,
		"schemas/.gitkeep should be removed after the schema file is added")
}

// ---------------------------------------------------------------------------
// AC-1: Schema is valid JSON and valid JSON Schema (draft 2020-12)
// ---------------------------------------------------------------------------

func TestSchema_IsValidJSON(t *testing.T) {
	data := loadSchemaBytes(t)

	var parsed map[string]any
	err := json.Unmarshal(data, &parsed)
	require.NoError(t, err, "schema file must parse as valid JSON")
	require.NotEmpty(t, parsed, "schema must be a non-empty JSON object")
}

func TestSchema_HasCorrectDraftVersion(t *testing.T) {
	data := loadSchemaBytes(t)

	var parsed map[string]any
	err := json.Unmarshal(data, &parsed)
	require.NoError(t, err)

	schemaField, ok := parsed["$schema"]
	require.True(t, ok, "schema must have a $schema field")
	assert.Equal(t, "https://json-schema.org/draft/2020-12/schema", schemaField,
		"$schema must reference draft 2020-12 exactly")
}

func TestSchema_CompilesAsJSONSchema(t *testing.T) {
	// This proves the file is not just valid JSON but actually a valid JSON Schema
	// that a compliant validator can compile. We validate a known-good document
	// through the production validator to confirm compilation succeeds.
	err := validateJSON(t, `{
		"apiVersion": "toolwright/v1",
		"kind": "Toolkit",
		"metadata": {"name": "test", "version": "1.0.0", "description": "T"},
		"tools": [{"name": "r", "description": "R", "entrypoint": "./r.sh"}]
	}`)
	require.NoError(t, err, "schema must compile and validate a known-good document")
}

// ---------------------------------------------------------------------------
// AC-1: Schema requires apiVersion, kind, metadata, tools
// ---------------------------------------------------------------------------

func TestSchema_RequiredTopLevelFields(t *testing.T) {
	data := loadSchemaBytes(t)

	var parsed map[string]any
	err := json.Unmarshal(data, &parsed)
	require.NoError(t, err)

	reqRaw, ok := parsed["required"]
	require.True(t, ok, "schema must have a top-level 'required' array")

	reqSlice, ok := reqRaw.([]any)
	require.True(t, ok, "'required' must be a JSON array")

	required := make([]string, len(reqSlice))
	for i, v := range reqSlice {
		s, ok := v.(string)
		require.True(t, ok, "each element in 'required' must be a string, got %T", v)
		required[i] = s
	}

	expectedFields := []string{"apiVersion", "kind", "metadata", "tools"}
	for _, field := range expectedFields {
		assert.Contains(t, required, field,
			"top-level 'required' must include %q; got %v", field, required)
	}
}

// ---------------------------------------------------------------------------
// AC-1: Valid manifest passes schema validation
// ---------------------------------------------------------------------------

func TestSchema_ValidManifest_Passes(t *testing.T) {
	tests := []struct {
		name string
		json string
	}{
		{
			name: "minimal valid manifest",
			json: `{
				"apiVersion": "toolwright/v1",
				"kind": "Toolkit",
				"metadata": {
					"name": "my-tool",
					"version": "1.0.0",
					"description": "A test tool"
				},
				"tools": [{
					"name": "run",
					"description": "Runs something",
					"entrypoint": "./run.sh"
				}]
			}`,
		},
		{
			name: "manifest with auth none",
			json: `{
				"apiVersion": "toolwright/v1",
				"kind": "Toolkit",
				"metadata": {
					"name": "another-tool",
					"version": "2.0.0",
					"description": "Another tool"
				},
				"tools": [{
					"name": "exec",
					"description": "Executes something",
					"entrypoint": "./exec.sh"
				}],
				"auth": {
					"type": "none"
				}
			}`,
		},
		{
			name: "manifest with token auth",
			json: `{
				"apiVersion": "toolwright/v1",
				"kind": "Toolkit",
				"metadata": {
					"name": "token-tool",
					"version": "0.1.0",
					"description": "Token auth tool"
				},
				"tools": [{
					"name": "fetch",
					"description": "Fetches data",
					"entrypoint": "./fetch.sh"
				}],
				"auth": {
					"type": "token",
					"token_env": "MY_TOKEN",
					"token_flag": "--token"
				}
			}`,
		},
		{
			name: "manifest with oauth2 auth",
			json: `{
				"apiVersion": "toolwright/v1",
				"kind": "Toolkit",
				"metadata": {
					"name": "oauth-tool",
					"version": "3.0.0",
					"description": "OAuth2 tool"
				},
				"tools": [{
					"name": "query",
					"description": "Queries API",
					"entrypoint": "./query.sh"
				}],
				"auth": {
					"type": "oauth2",
					"provider_url": "https://auth.example.com",
					"scopes": ["read", "write"]
				}
			}`,
		},
		{
			name: "manifest with multiple tools",
			json: `{
				"apiVersion": "toolwright/v1",
				"kind": "Toolkit",
				"metadata": {
					"name": "multi-tool",
					"version": "1.2.3",
					"description": "Multiple tools"
				},
				"tools": [
					{
						"name": "tool-a",
						"description": "Tool A",
						"entrypoint": "./a.sh"
					},
					{
						"name": "tool-b",
						"description": "Tool B",
						"entrypoint": "./b.sh"
					}
				]
			}`,
		},
		{
			name: "manifest with optional metadata fields",
			json: `{
				"apiVersion": "toolwright/v1",
				"kind": "Toolkit",
				"metadata": {
					"name": "full-meta",
					"version": "1.0.0",
					"description": "Full metadata",
					"author": "Jane Doe",
					"license": "MIT",
					"repository": "https://github.com/example/repo"
				},
				"tools": [{
					"name": "run",
					"description": "Runs",
					"entrypoint": "./run.sh"
				}]
			}`,
		},
		{
			name: "manifest with tool args and flags",
			json: `{
				"apiVersion": "toolwright/v1",
				"kind": "Toolkit",
				"metadata": {
					"name": "complex-tool",
					"version": "1.0.0",
					"description": "Complex tool with args and flags"
				},
				"tools": [{
					"name": "search",
					"description": "Searches things",
					"entrypoint": "./search.sh",
					"args": [{
						"name": "query",
						"type": "string",
						"required": true,
						"description": "Search query"
					}],
					"flags": [{
						"name": "limit",
						"type": "int",
						"required": false,
						"default": 10,
						"description": "Max results"
					}]
				}]
			}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateJSON(t, tc.json)
			assert.NoError(t, err, "valid manifest should pass schema validation")
		})
	}
}

// ---------------------------------------------------------------------------
// AC-1: Invalid manifests fail schema validation — missing required fields
// ---------------------------------------------------------------------------

func TestSchema_MissingRequiredFields_Fail(t *testing.T) {
	tests := []struct {
		name string
		json string
	}{
		{
			name: "missing apiVersion",
			json: `{
				"kind": "Toolkit",
				"metadata": {
					"name": "test",
					"version": "1.0.0",
					"description": "Test"
				},
				"tools": [{"name": "r", "description": "R", "entrypoint": "./r.sh"}]
			}`,
		},
		{
			name: "missing kind",
			json: `{
				"apiVersion": "toolwright/v1",
				"metadata": {
					"name": "test",
					"version": "1.0.0",
					"description": "Test"
				},
				"tools": [{"name": "r", "description": "R", "entrypoint": "./r.sh"}]
			}`,
		},
		{
			name: "missing metadata",
			json: `{
				"apiVersion": "toolwright/v1",
				"kind": "Toolkit",
				"tools": [{"name": "r", "description": "R", "entrypoint": "./r.sh"}]
			}`,
		},
		{
			name: "missing tools",
			json: `{
				"apiVersion": "toolwright/v1",
				"kind": "Toolkit",
				"metadata": {
					"name": "test",
					"version": "1.0.0",
					"description": "Test"
				}
			}`,
		},
		{
			name: "missing all required fields",
			json: `{}`,
		},
		{
			name: "missing metadata.name",
			json: `{
				"apiVersion": "toolwright/v1",
				"kind": "Toolkit",
				"metadata": {
					"version": "1.0.0",
					"description": "Test"
				},
				"tools": [{"name": "r", "description": "R", "entrypoint": "./r.sh"}]
			}`,
		},
		{
			name: "missing tool.name",
			json: `{
				"apiVersion": "toolwright/v1",
				"kind": "Toolkit",
				"metadata": {
					"name": "test",
					"version": "1.0.0",
					"description": "Test"
				},
				"tools": [{"description": "R", "entrypoint": "./r.sh"}]
			}`,
		},
		{
			name: "missing tool.description",
			json: `{
				"apiVersion": "toolwright/v1",
				"kind": "Toolkit",
				"metadata": {
					"name": "test",
					"version": "1.0.0",
					"description": "Test"
				},
				"tools": [{"name": "r", "entrypoint": "./r.sh"}]
			}`,
		},
		{
			name: "missing tool.entrypoint",
			json: `{
				"apiVersion": "toolwright/v1",
				"kind": "Toolkit",
				"metadata": {
					"name": "test",
					"version": "1.0.0",
					"description": "Test"
				},
				"tools": [{"name": "r", "description": "R"}]
			}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateJSON(t, tc.json)
			assert.Error(t, err, "manifest with %s should fail schema validation", tc.name)
		})
	}
}

// ---------------------------------------------------------------------------
// AC-1: Schema validates metadata name pattern [a-z0-9-]+
// ---------------------------------------------------------------------------

func TestSchema_MetadataNamePattern(t *testing.T) {
	// Build a helper that produces a complete valid manifest with just the name varied.
	makeManifest := func(name string) string {
		// Use json.Marshal to properly escape the name value.
		nameJSON, _ := json.Marshal(name)
		return `{
			"apiVersion": "toolwright/v1",
			"kind": "Toolkit",
			"metadata": {
				"name": ` + string(nameJSON) + `,
				"version": "1.0.0",
				"description": "Test"
			},
			"tools": [{"name": "r", "description": "R", "entrypoint": "./r.sh"}]
		}`
	}

	tests := []struct {
		name     string
		metaName string
		wantErr  bool
	}{
		// Valid names: [a-z0-9-]+
		{name: "lowercase only", metaName: "mytool", wantErr: false},
		{name: "with hyphens", metaName: "my-tool", wantErr: false},
		{name: "with numbers", metaName: "tool123", wantErr: false},
		{name: "numbers and hyphens", metaName: "123-tool-456", wantErr: false},
		{name: "single char", metaName: "a", wantErr: false},
		{name: "single digit", metaName: "1", wantErr: false},
		{name: "leading hyphen", metaName: "-tool", wantErr: false},
		{name: "trailing hyphen", metaName: "tool-", wantErr: false},
		{name: "all digits", metaName: "12345", wantErr: false},
		{name: "long valid name", metaName: "my-very-long-tool-name-123", wantErr: false},

		// Invalid names: anything outside [a-z0-9-]+
		{name: "uppercase letters", metaName: "MyTool", wantErr: true},
		{name: "all uppercase", metaName: "MYTOOL", wantErr: true},
		{name: "mixed case", metaName: "myTool", wantErr: true},
		{name: "underscores", metaName: "my_tool", wantErr: true},
		{name: "spaces", metaName: "my tool", wantErr: true},
		{name: "dots", metaName: "my.tool", wantErr: true},
		{name: "at sign", metaName: "my@tool", wantErr: true},
		{name: "exclamation", metaName: "tool!", wantErr: true},
		{name: "slash", metaName: "my/tool", wantErr: true},
		{name: "backslash", metaName: "my\\tool", wantErr: true},
		{name: "colon", metaName: "my:tool", wantErr: true},
		{name: "unicode", metaName: "werkzeug-\u00fc", wantErr: true},
		{name: "tab character", metaName: "my\ttool", wantErr: true},
		{name: "newline", metaName: "my\ntool", wantErr: true},
		{name: "empty string", metaName: "", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateJSON(t, makeManifest(tc.metaName))
			if tc.wantErr {
				assert.Error(t, err,
					"metadata.name %q should be rejected by pattern [a-z0-9-]+", tc.metaName)
			} else {
				assert.NoError(t, err,
					"metadata.name %q should be accepted by pattern [a-z0-9-]+", tc.metaName)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// AC-1: Schema validates auth type enum (none, token, oauth2)
// ---------------------------------------------------------------------------

func TestSchema_AuthTypeEnum(t *testing.T) {
	makeManifest := func(authType string) string {
		authJSON, _ := json.Marshal(authType)
		return `{
			"apiVersion": "toolwright/v1",
			"kind": "Toolkit",
			"metadata": {
				"name": "test",
				"version": "1.0.0",
				"description": "Test"
			},
			"tools": [{"name": "r", "description": "R", "entrypoint": "./r.sh"}],
			"auth": {
				"type": ` + string(authJSON) + `
			}
		}`
	}

	tests := []struct {
		name     string
		authType string
		wantErr  bool
	}{
		// Valid auth types
		{name: "none", authType: "none", wantErr: false},
		{name: "token", authType: "token", wantErr: false},
		{name: "oauth2", authType: "oauth2", wantErr: false},

		// Invalid auth types
		{name: "basic", authType: "basic", wantErr: true},
		{name: "apikey", authType: "apikey", wantErr: true},
		{name: "digest", authType: "digest", wantErr: true},
		{name: "bearer", authType: "bearer", wantErr: true},
		{name: "empty string", authType: "", wantErr: true},
		{name: "capitalized None", authType: "None", wantErr: true},
		{name: "capitalized Token", authType: "Token", wantErr: true},
		{name: "capitalized OAuth2", authType: "OAuth2", wantErr: true},
		{name: "uppercase NONE", authType: "NONE", wantErr: true},
		{name: "random string", authType: "foobar", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateJSON(t, makeManifest(tc.authType))
			if tc.wantErr {
				assert.Error(t, err,
					"auth.type %q should be rejected by enum constraint", tc.authType)
			} else {
				assert.NoError(t, err,
					"auth.type %q should be accepted by enum constraint", tc.authType)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// AC-1: Schema validates tool objects (name, description, entrypoint required)
// ---------------------------------------------------------------------------

func TestSchema_ToolRequiredFields(t *testing.T) {
	baseManifest := func(toolJSON string) string {
		return `{
			"apiVersion": "toolwright/v1",
			"kind": "Toolkit",
			"metadata": {
				"name": "test",
				"version": "1.0.0",
				"description": "Test"
			},
			"tools": [` + toolJSON + `]
		}`
	}

	tests := []struct {
		name    string
		tool    string
		wantErr bool
	}{
		{
			name:    "complete tool",
			tool:    `{"name": "r", "description": "R", "entrypoint": "./r.sh"}`,
			wantErr: false,
		},
		{
			name:    "missing name",
			tool:    `{"description": "R", "entrypoint": "./r.sh"}`,
			wantErr: true,
		},
		{
			name:    "missing description",
			tool:    `{"name": "r", "entrypoint": "./r.sh"}`,
			wantErr: true,
		},
		{
			name:    "missing entrypoint",
			tool:    `{"name": "r", "description": "R"}`,
			wantErr: true,
		},
		{
			name:    "empty object",
			tool:    `{}`,
			wantErr: true,
		},
		{
			name:    "only name",
			tool:    `{"name": "r"}`,
			wantErr: true,
		},
		{
			name:    "only description",
			tool:    `{"description": "R"}`,
			wantErr: true,
		},
		{
			name:    "only entrypoint",
			tool:    `{"entrypoint": "./r.sh"}`,
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateJSON(t, baseManifest(tc.tool))
			if tc.wantErr {
				assert.Error(t, err,
					"tool with %s should fail schema validation", tc.name)
			} else {
				assert.NoError(t, err,
					"tool with %s should pass schema validation", tc.name)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// AC-1: Schema validates tools is an array (not object, string, etc.)
// ---------------------------------------------------------------------------

func TestSchema_ToolsMustBeArray(t *testing.T) {
	tests := []struct {
		name string
		json string
	}{
		{
			name: "tools is an object",
			json: `{
				"apiVersion": "toolwright/v1",
				"kind": "Toolkit",
				"metadata": {"name": "test", "version": "1.0.0", "description": "T"},
				"tools": {"name": "r", "description": "R", "entrypoint": "./r.sh"}
			}`,
		},
		{
			name: "tools is a string",
			json: `{
				"apiVersion": "toolwright/v1",
				"kind": "Toolkit",
				"metadata": {"name": "test", "version": "1.0.0", "description": "T"},
				"tools": "run"
			}`,
		},
		{
			name: "tools is a number",
			json: `{
				"apiVersion": "toolwright/v1",
				"kind": "Toolkit",
				"metadata": {"name": "test", "version": "1.0.0", "description": "T"},
				"tools": 42
			}`,
		},
		{
			name: "tools is null",
			json: `{
				"apiVersion": "toolwright/v1",
				"kind": "Toolkit",
				"metadata": {"name": "test", "version": "1.0.0", "description": "T"},
				"tools": null
			}`,
		},
		{
			name: "tools is boolean",
			json: `{
				"apiVersion": "toolwright/v1",
				"kind": "Toolkit",
				"metadata": {"name": "test", "version": "1.0.0", "description": "T"},
				"tools": true
			}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateJSON(t, tc.json)
			assert.Error(t, err,
				"tools must be an array; %s should fail", tc.name)
		})
	}
}

// ---------------------------------------------------------------------------
// AC-1: Schema validates metadata is an object with required name field
// ---------------------------------------------------------------------------

func TestSchema_MetadataMustBeObject(t *testing.T) {
	tests := []struct {
		name string
		json string
	}{
		{
			name: "metadata is a string",
			json: `{
				"apiVersion": "toolwright/v1",
				"kind": "Toolkit",
				"metadata": "my-tool",
				"tools": [{"name": "r", "description": "R", "entrypoint": "./r.sh"}]
			}`,
		},
		{
			name: "metadata is an array",
			json: `{
				"apiVersion": "toolwright/v1",
				"kind": "Toolkit",
				"metadata": ["my-tool"],
				"tools": [{"name": "r", "description": "R", "entrypoint": "./r.sh"}]
			}`,
		},
		{
			name: "metadata is null",
			json: `{
				"apiVersion": "toolwright/v1",
				"kind": "Toolkit",
				"metadata": null,
				"tools": [{"name": "r", "description": "R", "entrypoint": "./r.sh"}]
			}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateJSON(t, tc.json)
			assert.Error(t, err,
				"metadata must be an object; %s should fail", tc.name)
		})
	}
}

// ---------------------------------------------------------------------------
// Anti-hardcoding: multiple distinct valid manifests all pass
// ---------------------------------------------------------------------------

func TestSchema_NotHardcoded_MultipleValidManifests(t *testing.T) {
	manifests := []string{
		`{
			"apiVersion": "toolwright/v1",
			"kind": "Toolkit",
			"metadata": {"name": "alpha", "version": "0.1.0", "description": "Alpha"},
			"tools": [{"name": "a", "description": "A", "entrypoint": "./a.sh"}]
		}`,
		`{
			"apiVersion": "toolwright/v1",
			"kind": "Toolkit",
			"metadata": {"name": "bravo-99", "version": "99.0.0", "description": "Bravo"},
			"tools": [{"name": "b", "description": "B", "entrypoint": "./b.sh"}]
		}`,
		`{
			"apiVersion": "toolwright/v1",
			"kind": "Toolkit",
			"metadata": {"name": "charlie", "version": "1.2.3-beta.1", "description": "Charlie"},
			"tools": [
				{"name": "c1", "description": "C1", "entrypoint": "./c1.sh"},
				{"name": "c2", "description": "C2", "entrypoint": "./c2.sh"},
				{"name": "c3", "description": "C3", "entrypoint": "./c3.sh"}
			]
		}`,
	}

	for i, m := range manifests {
		err := validateJSON(t, m)
		assert.NoError(t, err,
			"valid manifest #%d should pass schema validation", i)
	}
}

// ---------------------------------------------------------------------------
// Anti-hardcoding: multiple distinct invalid manifests all fail
// ---------------------------------------------------------------------------

func TestSchema_NotHardcoded_MultipleInvalidManifests(t *testing.T) {
	manifests := []struct {
		name string
		json string
	}{
		{
			name: "missing apiVersion only",
			json: `{
				"kind": "Toolkit",
				"metadata": {"name": "x", "version": "1.0.0", "description": "X"},
				"tools": [{"name": "r", "description": "R", "entrypoint": "./r.sh"}]
			}`,
		},
		{
			name: "uppercase metadata name",
			json: `{
				"apiVersion": "toolwright/v1",
				"kind": "Toolkit",
				"metadata": {"name": "BadName", "version": "1.0.0", "description": "X"},
				"tools": [{"name": "r", "description": "R", "entrypoint": "./r.sh"}]
			}`,
		},
		{
			name: "invalid auth type bearer",
			json: `{
				"apiVersion": "toolwright/v1",
				"kind": "Toolkit",
				"metadata": {"name": "x", "version": "1.0.0", "description": "X"},
				"tools": [{"name": "r", "description": "R", "entrypoint": "./r.sh"}],
				"auth": {"type": "bearer"}
			}`,
		},
		{
			name: "tool missing entrypoint",
			json: `{
				"apiVersion": "toolwright/v1",
				"kind": "Toolkit",
				"metadata": {"name": "x", "version": "1.0.0", "description": "X"},
				"tools": [{"name": "r", "description": "R"}]
			}`,
		},
		{
			name: "empty tools array with underscore name",
			json: `{
				"apiVersion": "toolwright/v1",
				"kind": "Toolkit",
				"metadata": {"name": "bad_name", "version": "1.0.0", "description": "X"},
				"tools": [{"name": "r", "description": "R", "entrypoint": "./r.sh"}]
			}`,
		},
	}

	for _, tc := range manifests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateJSON(t, tc.json)
			assert.Error(t, err,
				"invalid manifest (%s) should fail schema validation", tc.name)
		})
	}
}

// ---------------------------------------------------------------------------
// Edge case: schema structure — type constraints
// ---------------------------------------------------------------------------

func TestSchema_TypeConstraints(t *testing.T) {
	tests := []struct {
		name string
		json string
	}{
		{
			name: "apiVersion is number",
			json: `{
				"apiVersion": 1,
				"kind": "Toolkit",
				"metadata": {"name": "test", "version": "1.0.0", "description": "T"},
				"tools": [{"name": "r", "description": "R", "entrypoint": "./r.sh"}]
			}`,
		},
		{
			name: "kind is number",
			json: `{
				"apiVersion": "toolwright/v1",
				"kind": 42,
				"metadata": {"name": "test", "version": "1.0.0", "description": "T"},
				"tools": [{"name": "r", "description": "R", "entrypoint": "./r.sh"}]
			}`,
		},
		{
			name: "tool name is number",
			json: `{
				"apiVersion": "toolwright/v1",
				"kind": "Toolkit",
				"metadata": {"name": "test", "version": "1.0.0", "description": "T"},
				"tools": [{"name": 42, "description": "R", "entrypoint": "./r.sh"}]
			}`,
		},
		{
			name: "tool entrypoint is number",
			json: `{
				"apiVersion": "toolwright/v1",
				"kind": "Toolkit",
				"metadata": {"name": "test", "version": "1.0.0", "description": "T"},
				"tools": [{"name": "r", "description": "R", "entrypoint": 42}]
			}`,
		},
		{
			name: "metadata name is number",
			json: `{
				"apiVersion": "toolwright/v1",
				"kind": "Toolkit",
				"metadata": {"name": 42, "version": "1.0.0", "description": "T"},
				"tools": [{"name": "r", "description": "R", "entrypoint": "./r.sh"}]
			}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateJSON(t, tc.json)
			assert.Error(t, err,
				"wrong type for field in %s should fail schema validation", tc.name)
		})
	}
}

// ---------------------------------------------------------------------------
// Edge case: tools array contains non-object elements
// ---------------------------------------------------------------------------

func TestSchema_ToolsArrayItemsMustBeObjects(t *testing.T) {
	tests := []struct {
		name string
		json string
	}{
		{
			name: "tools contains a string",
			json: `{
				"apiVersion": "toolwright/v1",
				"kind": "Toolkit",
				"metadata": {"name": "test", "version": "1.0.0", "description": "T"},
				"tools": ["not-an-object"]
			}`,
		},
		{
			name: "tools contains a number",
			json: `{
				"apiVersion": "toolwright/v1",
				"kind": "Toolkit",
				"metadata": {"name": "test", "version": "1.0.0", "description": "T"},
				"tools": [42]
			}`,
		},
		{
			name: "tools contains null",
			json: `{
				"apiVersion": "toolwright/v1",
				"kind": "Toolkit",
				"metadata": {"name": "test", "version": "1.0.0", "description": "T"},
				"tools": [null]
			}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateJSON(t, tc.json)
			assert.Error(t, err,
				"tools array items must be objects; %s should fail", tc.name)
		})
	}
}

// ---------------------------------------------------------------------------
// Edge case: metadata.name pattern is anchored (full string match)
// ---------------------------------------------------------------------------

func TestSchema_MetadataNamePattern_Anchored(t *testing.T) {
	// If the pattern is not anchored (missing ^ and $), a name like "valid-BUT-INVALID"
	// could pass because "valid-" matches [a-z0-9-]+. This test catches unanchored patterns.
	tests := []struct {
		name     string
		metaName string
	}{
		{name: "valid prefix uppercase suffix", metaName: "valid-UPPER"},
		{name: "uppercase prefix valid suffix", metaName: "UPPER-valid"},
		{name: "valid with underscore at end", metaName: "valid_"},
		{name: "underscore then valid", metaName: "_valid"},
		{name: "valid with space in middle", metaName: "va lid"},
	}

	makeManifest := func(name string) string {
		nameJSON, _ := json.Marshal(name)
		return `{
			"apiVersion": "toolwright/v1",
			"kind": "Toolkit",
			"metadata": {
				"name": ` + string(nameJSON) + `,
				"version": "1.0.0",
				"description": "Test"
			},
			"tools": [{"name": "r", "description": "R", "entrypoint": "./r.sh"}]
		}`
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateJSON(t, makeManifest(tc.metaName))
			assert.Error(t, err,
				"metadata.name %q must be rejected; pattern must be anchored to full string", tc.metaName)
		})
	}
}

// ---------------------------------------------------------------------------
// Edge case: auth.type must be present when auth object exists
// ---------------------------------------------------------------------------

func TestSchema_AuthObjectRequiresType(t *testing.T) {
	manifest := `{
		"apiVersion": "toolwright/v1",
		"kind": "Toolkit",
		"metadata": {"name": "test", "version": "1.0.0", "description": "T"},
		"tools": [{"name": "r", "description": "R", "entrypoint": "./r.sh"}],
		"auth": {}
	}`

	err := validateJSON(t, manifest)
	assert.Error(t, err,
		"auth object without type field should fail schema validation")
}

// ---------------------------------------------------------------------------
// Structural: schema has properties for all Go struct fields
// ---------------------------------------------------------------------------

func TestSchema_HasProperties_TopLevel(t *testing.T) {
	data := loadSchemaBytes(t)

	var parsed map[string]any
	err := json.Unmarshal(data, &parsed)
	require.NoError(t, err)

	props, ok := parsed["properties"].(map[string]any)
	require.True(t, ok, "schema must have a 'properties' object at top level")

	// These properties correspond to the Go Toolkit struct fields.
	expectedProps := []string{"apiVersion", "kind", "metadata", "tools"}
	for _, prop := range expectedProps {
		_, exists := props[prop]
		assert.True(t, exists,
			"schema properties must include %q", prop)
	}
}

func TestSchema_HasProperties_Metadata(t *testing.T) {
	data := loadSchemaBytes(t)

	var parsed map[string]any
	err := json.Unmarshal(data, &parsed)
	require.NoError(t, err)

	props := parsed["properties"].(map[string]any)
	metaSchema, ok := props["metadata"].(map[string]any)
	require.True(t, ok, "metadata must be defined in properties")

	// Metadata can be defined inline or via $ref. Check for properties or $ref.
	metaProps, hasProps := metaSchema["properties"].(map[string]any)
	_, hasRef := metaSchema["$ref"]
	_, hasDefs := parsed["$defs"]
	if !hasDefs {
		_, hasDefs = parsed["definitions"]
	}

	if hasRef && hasDefs {
		// Schema uses $ref, which is fine — we can't easily inspect the ref target
		// in this test, but the validation tests cover correctness.
		return
	}

	require.True(t, hasProps,
		"metadata schema must have 'properties' (or use $ref with $defs)")

	// Check metadata has 'name' property.
	_, hasName := metaProps["name"]
	assert.True(t, hasName, "metadata properties must include 'name'")
}

// ---------------------------------------------------------------------------
// AC-6: Schema validates array flag types
// ---------------------------------------------------------------------------

func TestSchema_FlagTypeEnum(t *testing.T) {
	makeFlagManifest := func(flagType string) string {
		flagTypeJSON, _ := json.Marshal(flagType)
		return `{
			"apiVersion": "toolwright/v1",
			"kind": "Toolkit",
			"metadata": {
				"name": "test",
				"version": "1.0.0",
				"description": "Test"
			},
			"tools": [{
				"name": "run",
				"description": "Runs something",
				"entrypoint": "./run.sh",
				"flags": [{
					"name": "items",
					"type": ` + string(flagTypeJSON) + `,
					"description": "Some flag"
				}]
			}]
		}`
	}

	tests := []struct {
		name     string
		flagType string
		wantErr  bool
	}{
		// Valid scalar types (regression — must still pass)
		{name: "scalar string", flagType: "string", wantErr: false},
		{name: "scalar int", flagType: "int", wantErr: false},
		{name: "scalar float", flagType: "float", wantErr: false},
		{name: "scalar bool", flagType: "bool", wantErr: false},

		// Valid array types
		{name: "array string[]", flagType: "string[]", wantErr: false},
		{name: "array int[]", flagType: "int[]", wantErr: false},
		{name: "array float[]", flagType: "float[]", wantErr: false},
		{name: "array bool[]", flagType: "bool[]", wantErr: false},

		// Invalid types
		{name: "unknown[]", flagType: "unknown[]", wantErr: true},
		{name: "unknown scalar", flagType: "unknown", wantErr: true},
		{name: "String uppercase", flagType: "String", wantErr: true},
		{name: "INT uppercase", flagType: "INT", wantErr: true},
		{name: "String[] uppercase", flagType: "String[]", wantErr: true},
		{name: "[]string Go style", flagType: "[]string", wantErr: true},
		{name: "empty string", flagType: "", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateJSON(t, makeFlagManifest(tc.flagType))
			if tc.wantErr {
				assert.Error(t, err,
					"flag.type %q should be rejected by enum constraint", tc.flagType)
			} else {
				assert.NoError(t, err,
					"flag.type %q should be accepted by enum constraint", tc.flagType)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Structural: root schema type is "object"
// ---------------------------------------------------------------------------

func TestSchema_RootTypeIsObject(t *testing.T) {
	data := loadSchemaBytes(t)

	var parsed map[string]any
	err := json.Unmarshal(data, &parsed)
	require.NoError(t, err)

	schemaType, ok := parsed["type"]
	require.True(t, ok, "schema must have a 'type' field at root")
	assert.Equal(t, "object", schemaType,
		"root schema type must be 'object'")
}

// ---------------------------------------------------------------------------
// AC-7: Object types (object, object[]) accepted in flag type enum
// ---------------------------------------------------------------------------

func TestSchema_ObjectFlagTypes(t *testing.T) {
	makeFlagManifest := func(flagType string) string {
		flagTypeJSON, _ := json.Marshal(flagType)
		return `{
			"apiVersion": "toolwright/v1",
			"kind": "Toolkit",
			"metadata": {
				"name": "test",
				"version": "1.0.0",
				"description": "Test"
			},
			"tools": [{
				"name": "run",
				"description": "Runs something",
				"entrypoint": "./run.sh",
				"flags": [{
					"name": "data",
					"type": ` + string(flagTypeJSON) + `,
					"description": "Structured data flag"
				}]
			}]
		}`
	}

	tests := []struct {
		name     string
		flagType string
		wantErr  bool
	}{
		// New object types — must be accepted by the enum
		{name: "object type", flagType: "object", wantErr: false},
		{name: "object array type", flagType: "object[]", wantErr: false},

		// Existing scalar types must still pass (regression guard)
		{name: "string still valid", flagType: "string", wantErr: false},
		{name: "int still valid", flagType: "int", wantErr: false},
		{name: "float still valid", flagType: "float", wantErr: false},
		{name: "bool still valid", flagType: "bool", wantErr: false},

		// Existing array types must still pass (regression guard)
		{name: "string[] still valid", flagType: "string[]", wantErr: false},
		{name: "int[] still valid", flagType: "int[]", wantErr: false},
		{name: "float[] still valid", flagType: "float[]", wantErr: false},
		{name: "bool[] still valid", flagType: "bool[]", wantErr: false},

		// Invalid types must still be rejected
		{name: "object[][] double array invalid", flagType: "object[][]", wantErr: true},
		{name: "Object uppercase invalid", flagType: "Object", wantErr: true},
		{name: "Object[] uppercase invalid", flagType: "Object[]", wantErr: true},
		{name: "OBJECT uppercase invalid", flagType: "OBJECT", wantErr: true},
		{name: "objects plural invalid", flagType: "objects", wantErr: true},
		{name: "obj abbreviated invalid", flagType: "obj", wantErr: true},
		{name: "map type invalid", flagType: "map", wantErr: true},
		{name: "json type invalid", flagType: "json", wantErr: true},
		{name: "struct type invalid", flagType: "struct", wantErr: true},
		{name: "unknown still invalid", flagType: "unknown", wantErr: true},
		{name: "empty still invalid", flagType: "", wantErr: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateJSON(t, makeFlagManifest(tc.flagType))
			if tc.wantErr {
				assert.Error(t, err,
					"flag.type %q should be rejected by schema enum constraint", tc.flagType)
			} else {
				assert.NoError(t, err,
					"flag.type %q should be accepted by schema enum constraint", tc.flagType)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// AC-7: Structural — flag type enum in schema includes object and object[]
// ---------------------------------------------------------------------------

func TestSchema_FlagTypeEnum_IncludesObjectTypes(t *testing.T) {
	data := loadSchemaBytes(t)

	var parsed map[string]any
	err := json.Unmarshal(data, &parsed)
	require.NoError(t, err)

	// Navigate: properties -> tools -> items -> properties -> flags -> items -> properties -> type -> enum
	props := parsed["properties"].(map[string]any)
	tools := props["tools"].(map[string]any)
	toolItems := tools["items"].(map[string]any)
	toolProps := toolItems["properties"].(map[string]any)
	flags := toolProps["flags"].(map[string]any)
	flagItems := flags["items"].(map[string]any)
	flagProps := flagItems["properties"].(map[string]any)
	flagType := flagProps["type"].(map[string]any)
	enumRaw, ok := flagType["enum"].([]any)
	require.True(t, ok, "flag type must have an 'enum' array in the schema")

	enumValues := make([]string, len(enumRaw))
	for i, v := range enumRaw {
		s, ok := v.(string)
		require.True(t, ok, "each enum element must be a string, got %T at index %d", v, i)
		enumValues[i] = s
	}

	assert.Contains(t, enumValues, "object",
		"flag type enum must include 'object'; got %v", enumValues)
	assert.Contains(t, enumValues, "object[]",
		"flag type enum must include 'object[]'; got %v", enumValues)

	// Verify all original types are still present (regression)
	for _, expected := range []string{"string", "int", "float", "bool", "string[]", "int[]", "float[]", "bool[]"} {
		assert.Contains(t, enumValues, expected,
			"flag type enum must still include %q; got %v", expected, enumValues)
	}
}

// ---------------------------------------------------------------------------
// AC-7: itemSchema property accepted on flag definitions
// ---------------------------------------------------------------------------

func TestSchema_ObjectFlag_WithItemSchema(t *testing.T) {
	tests := []struct {
		name string
		json string
	}{
		{
			name: "object flag with simple itemSchema",
			json: `{
				"apiVersion": "toolwright/v1",
				"kind": "Toolkit",
				"metadata": {
					"name": "test",
					"version": "1.0.0",
					"description": "Test"
				},
				"tools": [{
					"name": "run",
					"description": "Runs something",
					"entrypoint": "./run.sh",
					"flags": [{
						"name": "config",
						"type": "object",
						"description": "Configuration object",
						"itemSchema": {
							"type": "object",
							"properties": {
								"name": {"type": "string"}
							}
						}
					}]
				}]
			}`,
		},
		{
			name: "object flag with nested properties in itemSchema",
			json: `{
				"apiVersion": "toolwright/v1",
				"kind": "Toolkit",
				"metadata": {
					"name": "test",
					"version": "1.0.0",
					"description": "Test"
				},
				"tools": [{
					"name": "run",
					"description": "Runs something",
					"entrypoint": "./run.sh",
					"flags": [{
						"name": "filter",
						"type": "object",
						"description": "Filter criteria",
						"itemSchema": {
							"type": "object",
							"properties": {
								"field": {"type": "string"},
								"operator": {"type": "string"},
								"value": {"type": "number"}
							},
							"required": ["field", "operator"]
						}
					}]
				}]
			}`,
		},
		{
			name: "object[] flag with itemSchema",
			json: `{
				"apiVersion": "toolwright/v1",
				"kind": "Toolkit",
				"metadata": {
					"name": "test",
					"version": "1.0.0",
					"description": "Test"
				},
				"tools": [{
					"name": "run",
					"description": "Runs something",
					"entrypoint": "./run.sh",
					"flags": [{
						"name": "items",
						"type": "object[]",
						"description": "List of items",
						"itemSchema": {
							"type": "object",
							"properties": {
								"id": {"type": "integer"},
								"label": {"type": "string"}
							}
						}
					}]
				}]
			}`,
		},
		{
			name: "itemSchema with empty properties object",
			json: `{
				"apiVersion": "toolwright/v1",
				"kind": "Toolkit",
				"metadata": {
					"name": "test",
					"version": "1.0.0",
					"description": "Test"
				},
				"tools": [{
					"name": "run",
					"description": "Runs something",
					"entrypoint": "./run.sh",
					"flags": [{
						"name": "data",
						"type": "object",
						"description": "Open-ended data",
						"itemSchema": {
							"type": "object",
							"properties": {}
						}
					}]
				}]
			}`,
		},
		{
			name: "itemSchema with minimal content",
			json: `{
				"apiVersion": "toolwright/v1",
				"kind": "Toolkit",
				"metadata": {
					"name": "test",
					"version": "1.0.0",
					"description": "Test"
				},
				"tools": [{
					"name": "run",
					"description": "Runs something",
					"entrypoint": "./run.sh",
					"flags": [{
						"name": "payload",
						"type": "object",
						"description": "Generic payload",
						"itemSchema": {
							"type": "object"
						}
					}]
				}]
			}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateJSON(t, tc.json)
			assert.NoError(t, err,
				"manifest with flag itemSchema should pass schema validation")
		})
	}
}

// ---------------------------------------------------------------------------
// AC-7: itemSchema accepted on non-object flag types (schema does not
// enforce the type/itemSchema relationship — that is the Go validator's job)
// ---------------------------------------------------------------------------

func TestSchema_ItemSchema_OnNonObjectType(t *testing.T) {
	tests := []struct {
		name     string
		flagType string
	}{
		{name: "string with itemSchema", flagType: "string"},
		{name: "int with itemSchema", flagType: "int"},
		{name: "bool with itemSchema", flagType: "bool"},
		{name: "string[] with itemSchema", flagType: "string[]"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			flagTypeJSON, _ := json.Marshal(tc.flagType)
			manifest := `{
				"apiVersion": "toolwright/v1",
				"kind": "Toolkit",
				"metadata": {
					"name": "test",
					"version": "1.0.0",
					"description": "Test"
				},
				"tools": [{
					"name": "run",
					"description": "Runs something",
					"entrypoint": "./run.sh",
					"flags": [{
						"name": "field",
						"type": ` + string(flagTypeJSON) + `,
						"description": "Some flag",
						"itemSchema": {
							"type": "object",
							"properties": {
								"key": {"type": "string"}
							}
						}
					}]
				}]
			}`
			err := validateJSON(t, manifest)
			assert.NoError(t, err,
				"itemSchema on flag type %q should be accepted by JSON Schema (relationship enforcement is Go validator's job)", tc.flagType)
		})
	}
}

// ---------------------------------------------------------------------------
// AC-7: Object type without itemSchema passes schema validation
// ---------------------------------------------------------------------------

func TestSchema_ObjectType_WithoutItemSchema(t *testing.T) {
	tests := []struct {
		name     string
		flagType string
	}{
		{name: "object without itemSchema", flagType: "object"},
		{name: "object[] without itemSchema", flagType: "object[]"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			flagTypeJSON, _ := json.Marshal(tc.flagType)
			manifest := `{
				"apiVersion": "toolwright/v1",
				"kind": "Toolkit",
				"metadata": {
					"name": "test",
					"version": "1.0.0",
					"description": "Test"
				},
				"tools": [{
					"name": "run",
					"description": "Runs something",
					"entrypoint": "./run.sh",
					"flags": [{
						"name": "data",
						"type": ` + string(flagTypeJSON) + `,
						"description": "Structured data without schema"
					}]
				}]
			}`
			err := validateJSON(t, manifest)
			assert.NoError(t, err,
				"flag type %q without itemSchema should pass schema validation", tc.flagType)
		})
	}
}

// ---------------------------------------------------------------------------
// AC-7: Structural — flag properties include itemSchema in the schema
// ---------------------------------------------------------------------------

func TestSchema_FlagProperties_IncludesItemSchema(t *testing.T) {
	data := loadSchemaBytes(t)

	var parsed map[string]any
	err := json.Unmarshal(data, &parsed)
	require.NoError(t, err)

	// Navigate: properties -> tools -> items -> properties -> flags -> items -> properties
	props := parsed["properties"].(map[string]any)
	tools := props["tools"].(map[string]any)
	toolItems := tools["items"].(map[string]any)
	toolProps := toolItems["properties"].(map[string]any)
	flags := toolProps["flags"].(map[string]any)
	flagItems := flags["items"].(map[string]any)
	flagProps, ok := flagItems["properties"].(map[string]any)
	require.True(t, ok, "flag items must have a 'properties' object")

	_, hasItemSchema := flagProps["itemSchema"]
	assert.True(t, hasItemSchema,
		"flag properties must include 'itemSchema'; got keys: %v", keys(flagProps))
}

// ---------------------------------------------------------------------------
// AC-7: itemSchema must be an object type in the schema definition
// ---------------------------------------------------------------------------

func TestSchema_ItemSchema_IsObjectType(t *testing.T) {
	data := loadSchemaBytes(t)

	var parsed map[string]any
	err := json.Unmarshal(data, &parsed)
	require.NoError(t, err)

	// Navigate: properties -> tools -> items -> properties -> flags -> items -> properties -> itemSchema
	props := parsed["properties"].(map[string]any)
	tools := props["tools"].(map[string]any)
	toolItems := tools["items"].(map[string]any)
	toolProps := toolItems["properties"].(map[string]any)
	flags := toolProps["flags"].(map[string]any)
	flagItems := flags["items"].(map[string]any)
	flagProps := flagItems["properties"].(map[string]any)

	itemSchemaRaw, ok := flagProps["itemSchema"]
	require.True(t, ok, "flag properties must include 'itemSchema'")

	itemSchemaDef, ok := itemSchemaRaw.(map[string]any)
	require.True(t, ok, "itemSchema definition must be a JSON object, got %T", itemSchemaRaw)

	schemaType, ok := itemSchemaDef["type"]
	require.True(t, ok, "itemSchema definition must have a 'type' field")
	assert.Equal(t, "object", schemaType,
		"itemSchema must be typed as 'object' in the schema definition")
}

// ---------------------------------------------------------------------------
// AC-7: Multiple object flags in the same tool pass validation
// ---------------------------------------------------------------------------

func TestSchema_MultipleObjectFlags(t *testing.T) {
	manifest := `{
		"apiVersion": "toolwright/v1",
		"kind": "Toolkit",
		"metadata": {
			"name": "test",
			"version": "1.0.0",
			"description": "Test"
		},
		"tools": [{
			"name": "run",
			"description": "Runs something",
			"entrypoint": "./run.sh",
			"flags": [
				{
					"name": "config",
					"type": "object",
					"description": "Config object",
					"itemSchema": {
						"type": "object",
						"properties": {
							"host": {"type": "string"},
							"port": {"type": "integer"}
						}
					}
				},
				{
					"name": "tags",
					"type": "object[]",
					"description": "Tag objects",
					"itemSchema": {
						"type": "object",
						"properties": {
							"key": {"type": "string"},
							"value": {"type": "string"}
						}
					}
				},
				{
					"name": "verbose",
					"type": "bool",
					"description": "Verbose output"
				}
			]
		}]
	}`

	err := validateJSON(t, manifest)
	assert.NoError(t, err,
		"manifest with multiple object flags alongside scalar flags should pass schema validation")
}

// keys is a test helper that returns sorted map keys for readable error messages.
func keys(m map[string]any) []string {
	result := make([]string, 0, len(m))
	for k := range m {
		result = append(result, k)
	}
	return result
}
