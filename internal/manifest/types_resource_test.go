package manifest

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.yaml.in/yaml/v3"
)

// ---------------------------------------------------------------------------
// Test fixtures -- manifest prefix for resource tests
// ---------------------------------------------------------------------------

const resourceManifestPrefix = `apiVersion: toolwright/v1
kind: Toolkit
metadata:
  name: resource-test
  version: 1.0.0
  description: Resource type tests
tools:
  - name: placeholder
    description: Placeholder tool
    entrypoint: ./tool.sh
`

// ---------------------------------------------------------------------------
// AC1: Resource struct exists and parses from YAML
// ---------------------------------------------------------------------------

func TestResource_Parse_SingleResource_AllFields(t *testing.T) {
	input := resourceManifestPrefix + `resources:
  - uri: "file:///data/config.json"
    name: app-config
    description: Application configuration file
    mimeType: application/json
    entrypoint: ./scripts/get-config.sh
`
	got, err := Parse(strings.NewReader(input))
	require.NoError(t, err, "Parse must succeed for manifest with a single resource")
	require.NotNil(t, got)

	require.NotNil(t, got.Resources, "Resources must not be nil when resources array is present")
	require.Len(t, got.Resources, 1, "Must have exactly 1 resource")

	r := got.Resources[0]
	assert.Equal(t, "file:///data/config.json", r.URI,
		"URI must match the YAML value exactly")
	assert.Equal(t, "app-config", r.Name,
		"Name must match the YAML value exactly")
	assert.Equal(t, "Application configuration file", r.Description,
		"Description must match the YAML value exactly")
	assert.Equal(t, "application/json", r.MimeType,
		"MimeType must match the YAML value exactly")
	assert.Equal(t, "./scripts/get-config.sh", r.Entrypoint,
		"Entrypoint must match the YAML value exactly")
}

func TestResource_Parse_MultipleResources(t *testing.T) {
	input := resourceManifestPrefix + `resources:
  - uri: "file:///data/config.json"
    name: app-config
    description: Application configuration
    mimeType: application/json
    entrypoint: ./get-config.sh
  - uri: "https://api.example.com/schema"
    name: api-schema
    description: API schema definition
    mimeType: application/yaml
    entrypoint: ./get-schema.sh
  - uri: "db://main/users"
    name: user-table
    description: User data table
    entrypoint: ./query-users.sh
`
	got, err := Parse(strings.NewReader(input))
	require.NoError(t, err, "Parse must succeed for manifest with multiple resources")
	require.NotNil(t, got)
	require.Len(t, got.Resources, 3, "Must have exactly 3 resources")

	// Verify ordering is preserved (not shuffled by a map-based parser).
	assert.Equal(t, "app-config", got.Resources[0].Name,
		"First resource must be app-config")
	assert.Equal(t, "api-schema", got.Resources[1].Name,
		"Second resource must be api-schema")
	assert.Equal(t, "user-table", got.Resources[2].Name,
		"Third resource must be user-table")

	// Verify each resource has distinct, correct field values.
	assert.Equal(t, "file:///data/config.json", got.Resources[0].URI)
	assert.Equal(t, "application/json", got.Resources[0].MimeType)
	assert.Equal(t, "./get-config.sh", got.Resources[0].Entrypoint)

	assert.Equal(t, "https://api.example.com/schema", got.Resources[1].URI)
	assert.Equal(t, "API schema definition", got.Resources[1].Description)
	assert.Equal(t, "application/yaml", got.Resources[1].MimeType)
	assert.Equal(t, "./get-schema.sh", got.Resources[1].Entrypoint)

	// Third resource has no mimeType.
	assert.Equal(t, "db://main/users", got.Resources[2].URI)
	assert.Equal(t, "User data table", got.Resources[2].Description)
	assert.Equal(t, "", got.Resources[2].MimeType,
		"MimeType must be empty string when not specified")
	assert.Equal(t, "./query-users.sh", got.Resources[2].Entrypoint)
}

// ---------------------------------------------------------------------------
// AC1: Optional fields within Resource (MimeType, Description)
// ---------------------------------------------------------------------------

func TestResource_Parse_OptionalFieldsOmitted(t *testing.T) {
	// MimeType and Description are optional. When omitted, they should be
	// empty strings (zero value), NOT cause a parse error.
	input := resourceManifestPrefix + `resources:
  - uri: "file:///data/raw.bin"
    name: raw-data
    entrypoint: ./get-raw.sh
`
	got, err := Parse(strings.NewReader(input))
	require.NoError(t, err, "Parse must succeed when optional resource fields are omitted")
	require.NotNil(t, got)
	require.Len(t, got.Resources, 1)

	r := got.Resources[0]
	assert.Equal(t, "file:///data/raw.bin", r.URI)
	assert.Equal(t, "raw-data", r.Name)
	assert.Equal(t, "", r.Description,
		"Description must be empty string when omitted")
	assert.Equal(t, "", r.MimeType,
		"MimeType must be empty string when omitted")
	assert.Equal(t, "./get-raw.sh", r.Entrypoint)
}

func TestResource_Parse_OnlyMimeTypeOmitted(t *testing.T) {
	input := resourceManifestPrefix + `resources:
  - uri: "file:///logs/app.log"
    name: app-log
    description: Application log file
    entrypoint: ./tail-log.sh
`
	got, err := Parse(strings.NewReader(input))
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Len(t, got.Resources, 1)

	r := got.Resources[0]
	assert.Equal(t, "Application log file", r.Description,
		"Description must be present when specified")
	assert.Equal(t, "", r.MimeType,
		"MimeType must be empty when omitted")
}

func TestResource_Parse_OnlyDescriptionOmitted(t *testing.T) {
	input := resourceManifestPrefix + `resources:
  - uri: "file:///data/image.png"
    name: screenshot
    mimeType: image/png
    entrypoint: ./get-image.sh
`
	got, err := Parse(strings.NewReader(input))
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Len(t, got.Resources, 1)

	r := got.Resources[0]
	assert.Equal(t, "", r.Description,
		"Description must be empty when omitted")
	assert.Equal(t, "image/png", r.MimeType,
		"MimeType must be present when specified")
}

// ---------------------------------------------------------------------------
// AC2: Resources are optional -- nil when absent
// ---------------------------------------------------------------------------

func TestResource_Parse_NoResourcesField(t *testing.T) {
	// A manifest without a resources key at all must parse with Resources == nil.
	input := `apiVersion: toolwright/v1
kind: Toolkit
metadata:
  name: no-resources
  version: 1.0.0
  description: Manifest without resources
tools:
  - name: hello
    description: Say hello
    entrypoint: ./hello.sh
`
	got, err := Parse(strings.NewReader(input))
	require.NoError(t, err, "Parse must succeed for manifest without resources")
	require.NotNil(t, got)

	assert.Nil(t, got.Resources,
		"Resources must be nil when not specified in YAML (not empty slice)")
}

func TestResource_Parse_EmptyResourcesArray(t *testing.T) {
	// resources: [] -- explicit empty array.
	input := resourceManifestPrefix + `resources: []
`
	got, err := Parse(strings.NewReader(input))
	require.NoError(t, err, "Parse must succeed for empty resources array")
	require.NotNil(t, got)

	// An explicit empty array may parse as nil or empty slice depending on
	// implementation. Either is acceptable, but it must NOT have any elements.
	assert.Empty(t, got.Resources,
		"resources: [] must result in empty (zero-length) Resources")
}

// ---------------------------------------------------------------------------
// AC2: Existing manifests are unaffected -- backward compatibility
// ---------------------------------------------------------------------------

func TestResource_Parse_ExistingFullManifest_Unaffected(t *testing.T) {
	// The full manifest fixture from parser_test.go must still parse correctly
	// after adding the Resources field. Resources must be nil.
	got, err := Parse(strings.NewReader(fullManifestYAML))
	require.NoError(t, err, "Existing full manifest must still parse after adding Resources field")
	require.NotNil(t, got)

	// Verify existing fields are not disturbed.
	assert.Equal(t, "toolwright/v1", got.APIVersion)
	assert.Equal(t, "Toolkit", got.Kind)
	assert.Equal(t, "petstore-tools", got.Metadata.Name)
	require.Len(t, got.Tools, 2)
	assert.Equal(t, "list-pets", got.Tools[0].Name)
	assert.Equal(t, "add-pet", got.Tools[1].Name)
	require.NotNil(t, got.Auth)
	assert.Equal(t, "oauth2", got.Auth.Type)

	// Resources must be nil for the existing manifest.
	assert.Nil(t, got.Resources,
		"Existing manifest without resources must have nil Resources")
}

func TestResource_Parse_ExistingMinimalManifest_Unaffected(t *testing.T) {
	input := `apiVersion: toolwright/v1
kind: Toolkit
metadata:
  name: minimal
  version: 0.1.0
  description: Minimal manifest
tools:
  - name: hello
    description: Says hello
    entrypoint: ./hello.sh
`
	got, err := Parse(strings.NewReader(input))
	require.NoError(t, err, "Minimal manifest must still parse")
	require.NotNil(t, got)

	assert.Equal(t, "minimal", got.Metadata.Name)
	require.Len(t, got.Tools, 1)
	assert.Equal(t, "hello", got.Tools[0].Name)
	assert.Nil(t, got.Resources,
		"Minimal manifest must have nil Resources")
	assert.Nil(t, got.Auth, "Auth must remain nil")
}

// ---------------------------------------------------------------------------
// AC1: Resources coexist with tools and other fields
// ---------------------------------------------------------------------------

func TestResource_Parse_ResourcesAlongsideAllFields(t *testing.T) {
	// Manifest with tools, auth, generate, AND resources all present.
	input := `apiVersion: toolwright/v1
kind: Toolkit
metadata:
  name: full-with-resources
  version: 2.0.0
  description: Full manifest including resources
  author: Test Author
tools:
  - name: list-items
    description: List all items
    entrypoint: ./list.sh
    args:
      - name: filter
        type: string
        required: false
        description: Filter expression
  - name: add-item
    description: Add an item
    entrypoint: ./add.sh
auth:
  type: token
  token_env: MY_TOKEN
  token_flag: --token
  token_header: "Authorization: Bearer"
resources:
  - uri: "file:///data/items.json"
    name: items-data
    description: Items data file
    mimeType: application/json
    entrypoint: ./get-items.sh
  - uri: "https://api.example.com/docs"
    name: api-docs
    description: API documentation
    mimeType: text/html
    entrypoint: ./get-docs.sh
generate:
  cli:
    target: go
  mcp:
    target: typescript
    transport:
      - stdio
`
	got, err := Parse(strings.NewReader(input))
	require.NoError(t, err, "Parse must succeed for manifest with all sections")
	require.NotNil(t, got)

	// Verify tools are intact.
	require.Len(t, got.Tools, 2)
	assert.Equal(t, "list-items", got.Tools[0].Name)
	assert.Equal(t, "add-item", got.Tools[1].Name)
	require.Len(t, got.Tools[0].Args, 1)
	assert.Equal(t, "filter", got.Tools[0].Args[0].Name)

	// Verify auth is intact.
	require.NotNil(t, got.Auth)
	assert.Equal(t, "token", got.Auth.Type)
	assert.Equal(t, "MY_TOKEN", got.Auth.TokenEnv)

	// Verify resources are correct.
	require.Len(t, got.Resources, 2)
	assert.Equal(t, "file:///data/items.json", got.Resources[0].URI)
	assert.Equal(t, "items-data", got.Resources[0].Name)
	assert.Equal(t, "Items data file", got.Resources[0].Description)
	assert.Equal(t, "application/json", got.Resources[0].MimeType)
	assert.Equal(t, "./get-items.sh", got.Resources[0].Entrypoint)

	assert.Equal(t, "https://api.example.com/docs", got.Resources[1].URI)
	assert.Equal(t, "api-docs", got.Resources[1].Name)
	assert.Equal(t, "text/html", got.Resources[1].MimeType)
	assert.Equal(t, "./get-docs.sh", got.Resources[1].Entrypoint)

	// Verify generate is intact.
	assert.Equal(t, "go", got.Generate.CLI.Target)
	assert.Equal(t, "typescript", got.Generate.MCP.Target)
}

// ---------------------------------------------------------------------------
// Table-driven: YAML tag correctness for Resource fields
// ---------------------------------------------------------------------------

func TestResource_Parse_YAMLTagCorrectness(t *testing.T) {
	// Verify each Resource field maps to the correct YAML key.
	// Wrong YAML tags (e.g., "mime_type" instead of "mimeType") would
	// silently produce zero values.
	tests := []struct {
		name      string
		yamlField string
		value     string
		checkFn   func(*testing.T, Resource)
	}{
		{
			name:      "uri tag",
			yamlField: "uri",
			value:     "file:///test/uri",
			checkFn: func(t *testing.T, r Resource) {
				t.Helper()
				assert.Equal(t, "file:///test/uri", r.URI,
					"uri YAML key must map to URI field")
			},
		},
		{
			name:      "name tag",
			yamlField: "name",
			value:     "test-name",
			checkFn: func(t *testing.T, r Resource) {
				t.Helper()
				assert.Equal(t, "test-name", r.Name,
					"name YAML key must map to Name field")
			},
		},
		{
			name:      "description tag",
			yamlField: "description",
			value:     "A test description",
			checkFn: func(t *testing.T, r Resource) {
				t.Helper()
				assert.Equal(t, "A test description", r.Description,
					"description YAML key must map to Description field")
			},
		},
		{
			name:      "mimeType tag (camelCase)",
			yamlField: "mimeType",
			value:     "text/plain",
			checkFn: func(t *testing.T, r Resource) {
				t.Helper()
				assert.Equal(t, "text/plain", r.MimeType,
					"mimeType YAML key must map to MimeType field")
			},
		},
		{
			name:      "entrypoint tag",
			yamlField: "entrypoint",
			value:     "./my-script.sh",
			checkFn: func(t *testing.T, r Resource) {
				t.Helper()
				assert.Equal(t, "./my-script.sh", r.Entrypoint,
					"entrypoint YAML key must map to Entrypoint field")
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Build a minimal resource with only the field under test.
			input := resourceManifestPrefix + `resources:
  - ` + tc.yamlField + `: "` + tc.value + `"
`
			got, err := Parse(strings.NewReader(input))
			require.NoError(t, err)
			require.NotNil(t, got)
			require.Len(t, got.Resources, 1)
			tc.checkFn(t, got.Resources[0])
		})
	}
}

func TestResource_Parse_WrongMimeTypeKey(t *testing.T) {
	// "mime_type" (snake_case) must NOT populate MimeType (camelCase tag).
	// A lazy implementation using snake_case yaml tag would fail this.
	wrongKeys := []string{
		"mime_type",
		"mimetype",
		"MimeType",
		"MIMETYPE",
		"content_type",
		"contentType",
	}

	for _, key := range wrongKeys {
		t.Run(key, func(t *testing.T) {
			input := resourceManifestPrefix + `resources:
  - uri: "file:///test"
    name: test
    entrypoint: ./test.sh
    ` + key + `: text/plain
`
			got, err := Parse(strings.NewReader(input))
			require.NoError(t, err)
			require.NotNil(t, got)
			require.Len(t, got.Resources, 1)
			assert.Equal(t, "", got.Resources[0].MimeType,
				"MimeType must be empty when YAML key is %q (only 'mimeType' should work)", key)
		})
	}
}

func TestResource_Parse_WrongResourcesKey(t *testing.T) {
	// Only "resources" (lowercase) should map to Toolkit.Resources.
	wrongKeys := []struct {
		name    string
		yamlKey string
	}{
		{name: "PascalCase", yamlKey: "Resources"},
		{name: "singular", yamlKey: "resource"},
		{name: "uppercase", yamlKey: "RESOURCES"},
		{name: "snake_case", yamlKey: "resources_list"},
	}

	for _, tc := range wrongKeys {
		t.Run(tc.name, func(t *testing.T) {
			input := `apiVersion: toolwright/v1
kind: Toolkit
metadata:
  name: wrong-key-test
  version: 1.0.0
  description: Wrong resources key
tools:
  - name: hello
    description: Hello
    entrypoint: ./hello.sh
` + tc.yamlKey + `:
  - uri: "file:///test"
    name: test
    entrypoint: ./test.sh
`
			got, err := Parse(strings.NewReader(input))
			require.NoError(t, err)
			require.NotNil(t, got)
			assert.Nil(t, got.Resources,
				"Resources must be nil when YAML key is %q (only 'resources' should work)", tc.yamlKey)
		})
	}
}

// ---------------------------------------------------------------------------
// AC1: Resource struct direct construction
// ---------------------------------------------------------------------------

func TestResource_StructConstruction(t *testing.T) {
	// Verify Resource struct exists with expected field names and types
	// by constructing one directly. Catches compile-time errors.
	r := Resource{
		URI:         "file:///data/test.json",
		Name:        "test-resource",
		Description: "A test resource",
		MimeType:    "application/json",
		Entrypoint:  "./get-test.sh",
	}

	assert.Equal(t, "file:///data/test.json", r.URI)
	assert.Equal(t, "test-resource", r.Name)
	assert.Equal(t, "A test resource", r.Description)
	assert.Equal(t, "application/json", r.MimeType)
	assert.Equal(t, "./get-test.sh", r.Entrypoint)
}

func TestResource_ZeroValue(t *testing.T) {
	// Zero-value Resource must have all empty strings.
	var r Resource
	assert.Equal(t, "", r.URI, "Zero-value URI must be empty")
	assert.Equal(t, "", r.Name, "Zero-value Name must be empty")
	assert.Equal(t, "", r.Description, "Zero-value Description must be empty")
	assert.Equal(t, "", r.MimeType, "Zero-value MimeType must be empty")
	assert.Equal(t, "", r.Entrypoint, "Zero-value Entrypoint must be empty")
}

func TestResource_ToolkitResourcesField(t *testing.T) {
	// Verify Toolkit has a Resources field of type []Resource.
	tk := Toolkit{
		Resources: []Resource{
			{URI: "file:///a", Name: "a", Entrypoint: "./a.sh"},
			{URI: "file:///b", Name: "b", Entrypoint: "./b.sh"},
		},
	}

	require.Len(t, tk.Resources, 2)
	assert.Equal(t, "a", tk.Resources[0].Name)
	assert.Equal(t, "b", tk.Resources[1].Name)
}

func TestResource_ToolkitResourcesNilByDefault(t *testing.T) {
	// A zero-value Toolkit must have nil Resources (slice zero value).
	tk := Toolkit{}
	assert.Nil(t, tk.Resources,
		"Zero-value Toolkit must have nil Resources (slice type)")
}

// ---------------------------------------------------------------------------
// Round-trip: resources survive marshal/unmarshal
// ---------------------------------------------------------------------------

func TestResource_RoundTrip_FullResources(t *testing.T) {
	input := resourceManifestPrefix + `resources:
  - uri: "file:///data/config.json"
    name: app-config
    description: Application configuration
    mimeType: application/json
    entrypoint: ./get-config.sh
  - uri: "https://api.example.com/schema"
    name: api-schema
    description: API schema
    mimeType: application/yaml
    entrypoint: ./get-schema.sh
`
	original, err := Parse(strings.NewReader(input))
	require.NoError(t, err, "First parse should succeed")

	marshalled, err := yaml.Marshal(original)
	require.NoError(t, err, "Marshal should succeed")
	require.NotEmpty(t, marshalled, "Marshalled YAML must not be empty")

	roundTripped, err := Parse(strings.NewReader(string(marshalled)))
	require.NoError(t, err, "Second parse (round-trip) should succeed")

	if diff := cmp.Diff(original, roundTripped); diff != "" {
		t.Errorf("Round-trip mismatch (-original +roundTripped):\n%s", diff)
	}

	// Explicit field checks to catch subtle issues cmp might miss.
	require.Len(t, roundTripped.Resources, 2)
	assert.Equal(t, "file:///data/config.json", roundTripped.Resources[0].URI)
	assert.Equal(t, "app-config", roundTripped.Resources[0].Name)
	assert.Equal(t, "Application configuration", roundTripped.Resources[0].Description)
	assert.Equal(t, "application/json", roundTripped.Resources[0].MimeType)
	assert.Equal(t, "./get-config.sh", roundTripped.Resources[0].Entrypoint)

	assert.Equal(t, "https://api.example.com/schema", roundTripped.Resources[1].URI)
	assert.Equal(t, "api-schema", roundTripped.Resources[1].Name)
}

func TestResource_RoundTrip_NoResources(t *testing.T) {
	input := `apiVersion: toolwright/v1
kind: Toolkit
metadata:
  name: bare-toolkit-rt
  version: 1.0.0
  description: Bare toolkit round-trip
tools:
  - name: hello
    description: Hello
    entrypoint: ./hello.sh
`
	original, err := Parse(strings.NewReader(input))
	require.NoError(t, err)

	marshalled, err := yaml.Marshal(original)
	require.NoError(t, err)

	// The marshalled YAML should NOT contain "resources" at all (omitempty).
	assert.NotContains(t, string(marshalled), "resources",
		"Marshalled YAML must not contain 'resources' when Resources is nil (omitempty)")

	roundTripped, err := Parse(strings.NewReader(string(marshalled)))
	require.NoError(t, err)
	assert.Nil(t, roundTripped.Resources,
		"Resources must remain nil after round-trip")
}

func TestResource_RoundTrip_OptionalFieldsOmitted(t *testing.T) {
	// Resource with only required fields -- MimeType omitted.
	input := resourceManifestPrefix + `resources:
  - uri: "file:///data/raw.bin"
    name: raw-data
    entrypoint: ./get-raw.sh
`
	original, err := Parse(strings.NewReader(input))
	require.NoError(t, err)
	require.Len(t, original.Resources, 1)
	assert.Equal(t, "", original.Resources[0].MimeType,
		"Pre-condition: MimeType must be empty")

	marshalled, err := yaml.Marshal(original)
	require.NoError(t, err)

	roundTripped, err := Parse(strings.NewReader(string(marshalled)))
	require.NoError(t, err)
	require.Len(t, roundTripped.Resources, 1)

	assert.Equal(t, "file:///data/raw.bin", roundTripped.Resources[0].URI)
	assert.Equal(t, "raw-data", roundTripped.Resources[0].Name)
	assert.Equal(t, "", roundTripped.Resources[0].MimeType,
		"MimeType must remain empty after round-trip (omitempty)")
	assert.Equal(t, "./get-raw.sh", roundTripped.Resources[0].Entrypoint)
}

func TestResource_RoundTrip_MimeTypeOmittedFromYAML(t *testing.T) {
	// When MimeType is empty, the marshalled YAML should NOT contain "mimeType"
	// (the omitempty tag should suppress it).
	input := resourceManifestPrefix + `resources:
  - uri: "file:///data/raw.bin"
    name: raw-data
    entrypoint: ./get-raw.sh
`
	original, err := Parse(strings.NewReader(input))
	require.NoError(t, err)

	marshalled, err := yaml.Marshal(original)
	require.NoError(t, err)

	// Count occurrences of "mimeType" in marshalled output.
	assert.NotContains(t, string(marshalled), "mimeType",
		"Marshalled YAML must not contain 'mimeType' when MimeType is empty (omitempty)")
}

// ---------------------------------------------------------------------------
// Edge cases: unusual but valid inputs
// ---------------------------------------------------------------------------

func TestResource_Parse_URISchemes(t *testing.T) {
	// Resources may use various URI schemes. The parser should not validate
	// or reject any particular scheme.
	tests := []struct {
		name string
		uri  string
	}{
		{name: "file URI", uri: "file:///path/to/file.txt"},
		{name: "https URI", uri: "https://example.com/resource"},
		{name: "http URI", uri: "http://example.com/resource"},
		{name: "custom scheme", uri: "custom://host/path"},
		{name: "data URI", uri: "data:text/plain;base64,SGVsbG8="},
		{name: "relative path (no scheme)", uri: "./relative/path.txt"},
		{name: "absolute path (no scheme)", uri: "/absolute/path.txt"},
		{name: "URN", uri: "urn:isbn:0451450523"},
		{name: "URI with query", uri: "https://example.com/data?format=json&limit=10"},
		{name: "URI with fragment", uri: "https://example.com/doc#section-3"},
		{name: "URI with special chars", uri: "https://example.com/path%20with%20spaces"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			input := resourceManifestPrefix + `resources:
  - uri: "` + tc.uri + `"
    name: uri-test
    entrypoint: ./test.sh
`
			got, err := Parse(strings.NewReader(input))
			require.NoError(t, err, "Parse must succeed for URI: %s", tc.uri)
			require.NotNil(t, got)
			require.Len(t, got.Resources, 1)
			assert.Equal(t, tc.uri, got.Resources[0].URI,
				"URI must be preserved exactly as specified")
		})
	}
}

func TestResource_Parse_EmptyStringFields(t *testing.T) {
	// Explicitly empty strings in YAML should parse as empty strings,
	// not as some default or nil.
	input := resourceManifestPrefix + `resources:
  - uri: ""
    name: ""
    description: ""
    mimeType: ""
    entrypoint: ""
`
	got, err := Parse(strings.NewReader(input))
	require.NoError(t, err, "Parse must succeed for resource with empty strings")
	require.NotNil(t, got)
	require.Len(t, got.Resources, 1)

	r := got.Resources[0]
	assert.Equal(t, "", r.URI, "Empty URI must be preserved")
	assert.Equal(t, "", r.Name, "Empty Name must be preserved")
	assert.Equal(t, "", r.Description, "Empty Description must be preserved")
	assert.Equal(t, "", r.MimeType, "Empty MimeType must be preserved")
	assert.Equal(t, "", r.Entrypoint, "Empty Entrypoint must be preserved")
}

func TestResource_Parse_UnicodeFields(t *testing.T) {
	input := resourceManifestPrefix + `resources:
  - uri: "file:///data/datos.json"
    name: "recurso-datos"
    description: "Datos del recurso en otro idioma"
    mimeType: application/json
    entrypoint: ./obtener-datos.sh
`
	got, err := Parse(strings.NewReader(input))
	require.NoError(t, err, "Parse must succeed for resource with unicode content")
	require.NotNil(t, got)
	require.Len(t, got.Resources, 1)

	assert.Equal(t, "Datos del recurso en otro idioma",
		got.Resources[0].Description)
}

func TestResource_Parse_LongDescription(t *testing.T) {
	longDesc := strings.Repeat("A very long description. ", 50)
	input := resourceManifestPrefix + `resources:
  - uri: "file:///data/big.json"
    name: big-resource
    description: "` + longDesc + `"
    entrypoint: ./get-big.sh
`
	got, err := Parse(strings.NewReader(input))
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Len(t, got.Resources, 1)
	assert.Equal(t, longDesc, got.Resources[0].Description,
		"Long descriptions must be preserved without truncation")
}

func TestResource_Parse_MimeTypeVariants(t *testing.T) {
	// Various valid MIME types should be preserved as-is.
	tests := []struct {
		name     string
		mimeType string
	}{
		{name: "application/json", mimeType: "application/json"},
		{name: "text/plain", mimeType: "text/plain"},
		{name: "text/html", mimeType: "text/html"},
		{name: "application/xml", mimeType: "application/xml"},
		{name: "image/png", mimeType: "image/png"},
		{name: "application/octet-stream", mimeType: "application/octet-stream"},
		{name: "with parameter", mimeType: "text/plain; charset=utf-8"},
		{name: "vendor type", mimeType: "application/vnd.api+json"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			input := resourceManifestPrefix + `resources:
  - uri: "file:///test"
    name: mime-test
    mimeType: "` + tc.mimeType + `"
    entrypoint: ./test.sh
`
			got, err := Parse(strings.NewReader(input))
			require.NoError(t, err)
			require.NotNil(t, got)
			require.Len(t, got.Resources, 1)
			assert.Equal(t, tc.mimeType, got.Resources[0].MimeType,
				"MimeType must be preserved exactly")
		})
	}
}

// ---------------------------------------------------------------------------
// Adversarial: YAML flow (inline) form
// ---------------------------------------------------------------------------

func TestResource_Parse_InlineForm(t *testing.T) {
	// YAML flow style: resources: [{ uri: ..., name: ..., ... }]
	input := resourceManifestPrefix + `resources:
  - { uri: "file:///inline", name: inline-res, description: "Inline resource", mimeType: "text/plain", entrypoint: "./inline.sh" }
`
	got, err := Parse(strings.NewReader(input))
	require.NoError(t, err, "Parse must succeed for inline/flow YAML form")
	require.NotNil(t, got)
	require.Len(t, got.Resources, 1)

	r := got.Resources[0]
	assert.Equal(t, "file:///inline", r.URI)
	assert.Equal(t, "inline-res", r.Name)
	assert.Equal(t, "Inline resource", r.Description)
	assert.Equal(t, "text/plain", r.MimeType)
	assert.Equal(t, "./inline.sh", r.Entrypoint)
}

// ---------------------------------------------------------------------------
// Adversarial: resources with many entries (not just 1 or 2)
// ---------------------------------------------------------------------------

func TestResource_Parse_ManyResources(t *testing.T) {
	// Verify parsing handles more than a trivial number of resources.
	// A hardcoded if/else chain would fail here.
	var resourcesYAML strings.Builder
	resourcesYAML.WriteString("resources:\n")
	const count = 20
	for i := 0; i < count; i++ {
		resourcesYAML.WriteString("  - uri: \"file:///data/res-")
		resourcesYAML.WriteString(strings.Repeat("x", i+1)) // unique URI
		resourcesYAML.WriteString("\"\n    name: res-")
		resourcesYAML.WriteString(strings.Repeat("x", i+1))
		resourcesYAML.WriteString("\n    entrypoint: ./get-res.sh\n")
	}

	input := resourceManifestPrefix + resourcesYAML.String()
	got, err := Parse(strings.NewReader(input))
	require.NoError(t, err, "Parse must succeed for manifest with %d resources", count)
	require.NotNil(t, got)
	require.Len(t, got.Resources, count,
		"Must parse exactly %d resources", count)

	// Verify first and last are distinct.
	assert.NotEqual(t, got.Resources[0].URI, got.Resources[count-1].URI,
		"Resources must have distinct URIs")
	assert.Equal(t, "file:///data/res-x", got.Resources[0].URI)
}

// ---------------------------------------------------------------------------
// Integration: go-cmp deep comparison for full parse
// ---------------------------------------------------------------------------

func TestResource_Parse_DeepComparison(t *testing.T) {
	input := resourceManifestPrefix + `resources:
  - uri: "file:///data/config.json"
    name: app-config
    description: Application configuration file
    mimeType: application/json
    entrypoint: ./scripts/get-config.sh
  - uri: "https://api.example.com/schema"
    name: api-schema
    description: API schema definition
    entrypoint: ./get-schema.sh
`
	got, err := Parse(strings.NewReader(input))
	require.NoError(t, err)
	require.NotNil(t, got)

	wantResources := []Resource{
		{
			URI:         "file:///data/config.json",
			Name:        "app-config",
			Description: "Application configuration file",
			MimeType:    "application/json",
			Entrypoint:  "./scripts/get-config.sh",
		},
		{
			URI:         "https://api.example.com/schema",
			Name:        "api-schema",
			Description: "API schema definition",
			MimeType:    "",
			Entrypoint:  "./get-schema.sh",
		},
	}

	if diff := cmp.Diff(wantResources, got.Resources); diff != "" {
		t.Errorf("Resources mismatch (-want +got):\n%s", diff)
	}
}

// ---------------------------------------------------------------------------
// Adversarial: Resource field independence
// ---------------------------------------------------------------------------

func TestResource_Parse_FieldsAreIndependent(t *testing.T) {
	// Two resources with overlapping field values must not share data.
	// A buggy implementation that reuses pointers or buffers would fail.
	input := resourceManifestPrefix + `resources:
  - uri: "file:///alpha"
    name: alpha
    description: Alpha resource
    mimeType: text/plain
    entrypoint: ./alpha.sh
  - uri: "file:///beta"
    name: beta
    description: Beta resource
    mimeType: application/json
    entrypoint: ./beta.sh
`
	got, err := Parse(strings.NewReader(input))
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Len(t, got.Resources, 2)

	// Verify no cross-contamination between resources.
	assert.Equal(t, "file:///alpha", got.Resources[0].URI)
	assert.Equal(t, "alpha", got.Resources[0].Name)
	assert.Equal(t, "Alpha resource", got.Resources[0].Description)
	assert.Equal(t, "text/plain", got.Resources[0].MimeType)
	assert.Equal(t, "./alpha.sh", got.Resources[0].Entrypoint)

	assert.Equal(t, "file:///beta", got.Resources[1].URI)
	assert.Equal(t, "beta", got.Resources[1].Name)
	assert.Equal(t, "Beta resource", got.Resources[1].Description)
	assert.Equal(t, "application/json", got.Resources[1].MimeType)
	assert.Equal(t, "./beta.sh", got.Resources[1].Entrypoint)
}

// ---------------------------------------------------------------------------
// Adversarial: Resources does not interfere with tool parsing
// ---------------------------------------------------------------------------

func TestResource_Parse_ToolsNotAffectedByResources(t *testing.T) {
	// Adding resources must not alter how tools are parsed. Specifically,
	// resource fields (uri, mimeType) are NOT tool fields and must not
	// bleed into tool parsing.
	input := `apiVersion: toolwright/v1
kind: Toolkit
metadata:
  name: isolation-test
  version: 1.0.0
  description: Tools and resources isolation
tools:
  - name: my-tool
    description: A tool
    entrypoint: ./tool.sh
    output:
      format: json
      mimeType: application/json
resources:
  - uri: "file:///data/res.json"
    name: my-resource
    description: A resource
    mimeType: text/plain
    entrypoint: ./resource.sh
`
	got, err := Parse(strings.NewReader(input))
	require.NoError(t, err)
	require.NotNil(t, got)

	// Tool checks.
	require.Len(t, got.Tools, 1)
	assert.Equal(t, "my-tool", got.Tools[0].Name)
	assert.Equal(t, "./tool.sh", got.Tools[0].Entrypoint)
	assert.Equal(t, "json", got.Tools[0].Output.Format)
	assert.Equal(t, "application/json", got.Tools[0].Output.MimeType)

	// Resource checks.
	require.Len(t, got.Resources, 1)
	assert.Equal(t, "file:///data/res.json", got.Resources[0].URI)
	assert.Equal(t, "my-resource", got.Resources[0].Name)
	assert.Equal(t, "text/plain", got.Resources[0].MimeType)
	assert.Equal(t, "./resource.sh", got.Resources[0].Entrypoint)
}
