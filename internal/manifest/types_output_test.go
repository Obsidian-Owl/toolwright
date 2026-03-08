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
// Test fixtures — manifest prefix for output tests
// ---------------------------------------------------------------------------

const outputManifestPrefix = `apiVersion: toolwright/v1
kind: Toolkit
metadata:
  name: output-test
  version: 1.0.0
  description: Output type tests
tools:
`

// ---------------------------------------------------------------------------
// AC1: Output.Schema union type — string path form
// ---------------------------------------------------------------------------

func TestOutput_Schema_StringPath(t *testing.T) {
	input := outputManifestPrefix + `  - name: string-schema
    description: Tool with string schema path
    entrypoint: ./tool.sh
    output:
      format: json
      schema: "path/to/schema.json"
`
	got, err := Parse(strings.NewReader(input))
	require.NoError(t, err, "Parse must succeed for string schema path")
	require.NotNil(t, got)
	require.Len(t, got.Tools, 1)

	schema := got.Tools[0].Output.Schema
	require.NotNil(t, schema, "Schema must not be nil when set to a string path")

	// Verify Schema is a string, not a map or any other type.
	s, ok := schema.(string)
	require.True(t, ok, "Schema must be a string when YAML value is a scalar, got %T", schema)
	assert.Equal(t, "path/to/schema.json", s,
		"Schema string value must match the YAML input exactly")
}

func TestOutput_Schema_StringPath_VariousPathForms(t *testing.T) {
	tests := []struct {
		name       string
		schemaVal  string
		wantString string
	}{
		{
			name:       "relative path with directory",
			schemaVal:  "schemas/pets.schema.json",
			wantString: "schemas/pets.schema.json",
		},
		{
			name:       "bare filename",
			schemaVal:  "schema.json",
			wantString: "schema.json",
		},
		{
			name:       "deeply nested path",
			schemaVal:  "a/b/c/d/schema.json",
			wantString: "a/b/c/d/schema.json",
		},
		{
			name:       "path with dots",
			schemaVal:  "../schemas/output.schema.json",
			wantString: "../schemas/output.schema.json",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			input := outputManifestPrefix + `  - name: path-test
    description: Path form test
    entrypoint: ./tool.sh
    output:
      format: json
      schema: "` + tc.schemaVal + `"
`
			got, err := Parse(strings.NewReader(input))
			require.NoError(t, err)
			require.Len(t, got.Tools, 1)

			s, ok := got.Tools[0].Output.Schema.(string)
			require.True(t, ok, "Schema must be string, got %T", got.Tools[0].Output.Schema)
			assert.Equal(t, tc.wantString, s)
		})
	}
}

// ---------------------------------------------------------------------------
// AC1: Output.Schema union type — inline object form
// ---------------------------------------------------------------------------

func TestOutput_Schema_InlineObject(t *testing.T) {
	input := outputManifestPrefix + `  - name: inline-schema
    description: Tool with inline JSON Schema
    entrypoint: ./tool.sh
    output:
      format: json
      schema:
        type: object
        properties:
          name:
            type: string
          count:
            type: integer
        required:
          - name
`
	got, err := Parse(strings.NewReader(input))
	require.NoError(t, err, "Parse must succeed for inline object schema")
	require.NotNil(t, got)
	require.Len(t, got.Tools, 1)

	schema := got.Tools[0].Output.Schema
	require.NotNil(t, schema, "Schema must not be nil when set to an inline object")

	// Verify Schema is a map[string]any, not a string.
	m, ok := schema.(map[string]any)
	require.True(t, ok,
		"Schema must be map[string]any when YAML value is a mapping, got %T", schema)

	// Verify specific keys exist with correct values.
	assert.Equal(t, "object", m["type"],
		"Inline schema 'type' must be 'object'")

	props, ok := m["properties"].(map[string]any)
	require.True(t, ok,
		"Schema 'properties' must be map[string]any, got %T", m["properties"])
	assert.Contains(t, props, "name", "Schema properties must contain 'name'")
	assert.Contains(t, props, "count", "Schema properties must contain 'count'")

	// Verify nested property type values.
	nameProp, ok := props["name"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "string", nameProp["type"])

	countProp, ok := props["count"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "integer", countProp["type"])

	// Verify required array.
	reqRaw, ok := m["required"]
	require.True(t, ok, "Inline schema must have 'required' key")
	reqSlice, ok := reqRaw.([]any)
	require.True(t, ok, "Schema 'required' must be []any, got %T", reqRaw)
	require.Len(t, reqSlice, 1)
	assert.Equal(t, "name", reqSlice[0])
}

func TestOutput_Schema_InlineObject_FlowForm(t *testing.T) {
	// YAML inline/flow form: schema: { type: object }
	input := outputManifestPrefix + `  - name: flow-schema
    description: Tool with flow-style inline schema
    entrypoint: ./tool.sh
    output:
      format: json
      schema: { type: object, properties: { id: { type: integer } } }
`
	got, err := Parse(strings.NewReader(input))
	require.NoError(t, err, "Parse must succeed for flow-style inline schema")
	require.Len(t, got.Tools, 1)

	schema := got.Tools[0].Output.Schema
	m, ok := schema.(map[string]any)
	require.True(t, ok, "Flow-style schema must parse as map[string]any, got %T", schema)
	assert.Equal(t, "object", m["type"])

	props, ok := m["properties"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, props, "id")
}

func TestOutput_Schema_InlineObject_ComplexNested(t *testing.T) {
	// Test deeply nested schema to verify the YAML decoder recurses properly.
	input := outputManifestPrefix + `  - name: deep-schema
    description: Tool with deeply nested inline schema
    entrypoint: ./tool.sh
    output:
      format: json
      schema:
        type: object
        properties:
          address:
            type: object
            properties:
              street:
                type: string
              city:
                type: string
              geo:
                type: object
                properties:
                  lat:
                    type: number
                  lng:
                    type: number
          tags:
            type: array
            items:
              type: string
`
	got, err := Parse(strings.NewReader(input))
	require.NoError(t, err)
	require.Len(t, got.Tools, 1)

	schema := got.Tools[0].Output.Schema
	m, ok := schema.(map[string]any)
	require.True(t, ok, "Schema must be map[string]any, got %T", schema)

	props, ok := m["properties"].(map[string]any)
	require.True(t, ok)

	// Verify three levels deep: address -> geo -> lat.
	addrProp, ok := props["address"].(map[string]any)
	require.True(t, ok)
	addrProps, ok := addrProp["properties"].(map[string]any)
	require.True(t, ok)
	geoProp, ok := addrProps["geo"].(map[string]any)
	require.True(t, ok)
	geoProps, ok := geoProp["properties"].(map[string]any)
	require.True(t, ok)
	latProp, ok := geoProps["lat"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "number", latProp["type"],
		"Deeply nested property type must survive parsing")

	// Verify array items.
	tagsProp, ok := props["tags"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "array", tagsProp["type"])
	items, ok := tagsProp["items"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "string", items["type"])
}

// ---------------------------------------------------------------------------
// AC1: No schema field — Schema is nil
// ---------------------------------------------------------------------------

func TestOutput_Schema_NilWhenOmitted(t *testing.T) {
	input := outputManifestPrefix + `  - name: no-schema
    description: Tool with no schema
    entrypoint: ./tool.sh
    output:
      format: text
`
	got, err := Parse(strings.NewReader(input))
	require.NoError(t, err)
	require.Len(t, got.Tools, 1)

	assert.Nil(t, got.Tools[0].Output.Schema,
		"Schema must be nil when omitted from YAML")
}

// ---------------------------------------------------------------------------
// AC1: Edge case — empty string schema
// ---------------------------------------------------------------------------

func TestOutput_Schema_EmptyString(t *testing.T) {
	input := outputManifestPrefix + `  - name: empty-schema
    description: Tool with empty string schema
    entrypoint: ./tool.sh
    output:
      format: json
      schema: ""
`
	got, err := Parse(strings.NewReader(input))
	require.NoError(t, err)
	require.Len(t, got.Tools, 1)

	schema := got.Tools[0].Output.Schema
	// An empty string should parse as a string, not nil.
	// The UnmarshalYAML receives a scalar node with value "".
	s, ok := schema.(string)
	require.True(t, ok, "Empty string schema must parse as string, got %T", schema)
	assert.Equal(t, "", s, "Empty string schema must be empty string")
}

// ---------------------------------------------------------------------------
// AC1: Edge case — empty object schema
// ---------------------------------------------------------------------------

func TestOutput_Schema_EmptyObject(t *testing.T) {
	input := outputManifestPrefix + `  - name: empty-obj-schema
    description: Tool with empty object schema
    entrypoint: ./tool.sh
    output:
      format: json
      schema: {}
`
	got, err := Parse(strings.NewReader(input))
	require.NoError(t, err)
	require.Len(t, got.Tools, 1)

	schema := got.Tools[0].Output.Schema
	require.NotNil(t, schema, "schema: {} must not parse as nil")

	m, ok := schema.(map[string]any)
	require.True(t, ok, "schema: {} must parse as map[string]any, got %T", schema)
	assert.Len(t, m, 0, "schema: {} must be an empty map")
}

// ---------------------------------------------------------------------------
// AC2: Output.MimeType field — populated when present
// ---------------------------------------------------------------------------

func TestOutput_MimeType_Populated(t *testing.T) {
	input := outputManifestPrefix + `  - name: binary-tool
    description: Tool with binary output
    entrypoint: ./tool.sh
    output:
      format: binary
      mimeType: "image/png"
`
	got, err := Parse(strings.NewReader(input))
	require.NoError(t, err, "Parse must succeed for output with mimeType")
	require.Len(t, got.Tools, 1)

	output := got.Tools[0].Output
	assert.Equal(t, "binary", output.Format)
	assert.Equal(t, "image/png", output.MimeType,
		"MimeType must be populated when specified in YAML")
}

func TestOutput_MimeType_VariousTypes(t *testing.T) {
	tests := []struct {
		name     string
		mimeType string
	}{
		{name: "image/png", mimeType: "image/png"},
		{name: "image/jpeg", mimeType: "image/jpeg"},
		{name: "application/pdf", mimeType: "application/pdf"},
		{name: "application/octet-stream", mimeType: "application/octet-stream"},
		{name: "audio/mpeg", mimeType: "audio/mpeg"},
		{name: "video/mp4", mimeType: "video/mp4"},
		{name: "application/zip", mimeType: "application/zip"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			input := outputManifestPrefix + `  - name: mime-test
    description: MIME type test
    entrypoint: ./tool.sh
    output:
      format: binary
      mimeType: "` + tc.mimeType + `"
`
			got, err := Parse(strings.NewReader(input))
			require.NoError(t, err)
			require.Len(t, got.Tools, 1)
			assert.Equal(t, tc.mimeType, got.Tools[0].Output.MimeType,
				"MimeType must match the YAML value exactly")
		})
	}
}

// ---------------------------------------------------------------------------
// AC2: Output.MimeType field — empty when omitted
// ---------------------------------------------------------------------------

func TestOutput_MimeType_EmptyWhenOmitted(t *testing.T) {
	input := outputManifestPrefix + `  - name: no-mime
    description: Tool without mimeType
    entrypoint: ./tool.sh
    output:
      format: json
`
	got, err := Parse(strings.NewReader(input))
	require.NoError(t, err)
	require.Len(t, got.Tools, 1)

	assert.Equal(t, "", got.Tools[0].Output.MimeType,
		"MimeType must be empty string when omitted from YAML")
}

func TestOutput_MimeType_EmptyWhenFormatIsText(t *testing.T) {
	input := outputManifestPrefix + `  - name: text-tool
    description: Tool with text output
    entrypoint: ./tool.sh
    output:
      format: text
`
	got, err := Parse(strings.NewReader(input))
	require.NoError(t, err)
	require.Len(t, got.Tools, 1)

	assert.Equal(t, "text", got.Tools[0].Output.Format)
	assert.Equal(t, "", got.Tools[0].Output.MimeType,
		"MimeType must be empty for text format without mimeType")
}

// ---------------------------------------------------------------------------
// AC2: MimeType with non-binary format (both fields populated)
// ---------------------------------------------------------------------------

func TestOutput_MimeType_WithNonBinaryFormat(t *testing.T) {
	// Validation is a separate concern (Task 2). The parser should accept
	// mimeType with any format and populate both fields.
	input := outputManifestPrefix + `  - name: json-with-mime
    description: JSON format with mimeType
    entrypoint: ./tool.sh
    output:
      format: json
      mimeType: "application/json"
`
	got, err := Parse(strings.NewReader(input))
	require.NoError(t, err, "Parse must succeed even with mimeType on non-binary format")
	require.Len(t, got.Tools, 1)

	output := got.Tools[0].Output
	assert.Equal(t, "json", output.Format)
	assert.Equal(t, "application/json", output.MimeType,
		"MimeType must be populated regardless of format (validation is separate)")
}

// ---------------------------------------------------------------------------
// AC2: MimeType YAML tag correctness
// ---------------------------------------------------------------------------

func TestOutput_MimeType_YAMLTagIsCorrect(t *testing.T) {
	// Only the exact YAML key "mimeType" should populate the field.
	// Wrong keys must not work.
	wrongKeys := []struct {
		name    string
		yamlKey string
	}{
		{name: "snake_case mime_type", yamlKey: "mime_type"},
		{name: "lowercase mimetype", yamlKey: "mimetype"},
		{name: "PascalCase MimeType", yamlKey: "MimeType"},
		{name: "uppercase MIMETYPE", yamlKey: "MIMETYPE"},
		{name: "hyphenated mime-type", yamlKey: "mime-type"},
		{name: "content_type", yamlKey: "content_type"},
		{name: "contentType", yamlKey: "contentType"},
	}

	for _, tc := range wrongKeys {
		t.Run(tc.name, func(t *testing.T) {
			input := outputManifestPrefix + `  - name: tag-test
    description: YAML tag test
    entrypoint: ./tool.sh
    output:
      format: binary
      ` + tc.yamlKey + `: "image/png"
`
			got, err := Parse(strings.NewReader(input))
			require.NoError(t, err)
			require.Len(t, got.Tools, 1)
			assert.Equal(t, "", got.Tools[0].Output.MimeType,
				"MimeType must be empty when YAML key is %q (only 'mimeType' should work)",
				tc.yamlKey)
		})
	}
}

// ---------------------------------------------------------------------------
// AC2: Full output block with all fields
// ---------------------------------------------------------------------------

func TestOutput_AllFieldsPopulated(t *testing.T) {
	input := outputManifestPrefix + `  - name: full-output
    description: Tool with all output fields
    entrypoint: ./tool.sh
    output:
      format: binary
      schema: "schemas/output.schema.json"
      mimeType: "image/png"
`
	got, err := Parse(strings.NewReader(input))
	require.NoError(t, err)
	require.Len(t, got.Tools, 1)

	output := got.Tools[0].Output
	assert.Equal(t, "binary", output.Format)

	s, ok := output.Schema.(string)
	require.True(t, ok, "Schema must be string when set to a path, got %T", output.Schema)
	assert.Equal(t, "schemas/output.schema.json", s)

	assert.Equal(t, "image/png", output.MimeType)
}

func TestOutput_InlineSchemaWithMimeType(t *testing.T) {
	input := outputManifestPrefix + `  - name: inline-with-mime
    description: Inline schema plus mimeType
    entrypoint: ./tool.sh
    output:
      format: binary
      schema:
        type: object
        properties:
          data:
            type: string
      mimeType: "application/octet-stream"
`
	got, err := Parse(strings.NewReader(input))
	require.NoError(t, err)
	require.Len(t, got.Tools, 1)

	output := got.Tools[0].Output
	assert.Equal(t, "binary", output.Format)

	m, ok := output.Schema.(map[string]any)
	require.True(t, ok, "Schema must be map[string]any, got %T", output.Schema)
	assert.Equal(t, "object", m["type"])

	assert.Equal(t, "application/octet-stream", output.MimeType)
}

// ---------------------------------------------------------------------------
// AC3: Backward compat — string schema round-trip
// ---------------------------------------------------------------------------

func TestOutput_RoundTrip_StringSchema(t *testing.T) {
	input := outputManifestPrefix + `  - name: rt-string
    description: Round-trip string schema
    entrypoint: ./tool.sh
    output:
      format: json
      schema: "schemas/pets.schema.json"
`
	original, err := Parse(strings.NewReader(input))
	require.NoError(t, err)
	require.Len(t, original.Tools, 1)

	// Pre-condition: Schema is a string.
	origSchema, ok := original.Tools[0].Output.Schema.(string)
	require.True(t, ok, "Pre-condition: Schema must be string")
	assert.Equal(t, "schemas/pets.schema.json", origSchema)

	// Marshal and re-parse.
	marshalled, err := yaml.Marshal(original)
	require.NoError(t, err)
	require.NotEmpty(t, marshalled)

	roundTripped, err := Parse(strings.NewReader(string(marshalled)))
	require.NoError(t, err)
	require.Len(t, roundTripped.Tools, 1)

	// Post-condition: Schema is still a string with the same value.
	rtSchema, ok := roundTripped.Tools[0].Output.Schema.(string)
	require.True(t, ok,
		"After round-trip, Schema must still be string, got %T",
		roundTripped.Tools[0].Output.Schema)
	assert.Equal(t, "schemas/pets.schema.json", rtSchema,
		"String schema value must survive round-trip unchanged")
}

func TestOutput_RoundTrip_StringSchema_DeepEqual(t *testing.T) {
	// Full structural comparison using go-cmp to catch any subtle diffs.
	input := outputManifestPrefix + `  - name: rt-deep
    description: Deep equal round-trip
    entrypoint: ./tool.sh
    output:
      format: json
      schema: "path.json"
`
	original, err := Parse(strings.NewReader(input))
	require.NoError(t, err)

	marshalled, err := yaml.Marshal(original)
	require.NoError(t, err)

	roundTripped, err := Parse(strings.NewReader(string(marshalled)))
	require.NoError(t, err)

	if diff := cmp.Diff(original, roundTripped); diff != "" {
		t.Errorf("Round-trip mismatch (-original +roundTripped):\n%s", diff)
	}
}

// ---------------------------------------------------------------------------
// AC3: Backward compat — inline object schema round-trip
// ---------------------------------------------------------------------------

func TestOutput_RoundTrip_InlineSchema(t *testing.T) {
	input := outputManifestPrefix + `  - name: rt-inline
    description: Round-trip inline schema
    entrypoint: ./tool.sh
    output:
      format: json
      schema:
        type: object
        properties:
          id:
            type: integer
          name:
            type: string
        required:
          - id
`
	original, err := Parse(strings.NewReader(input))
	require.NoError(t, err)
	require.Len(t, original.Tools, 1)

	// Pre-condition: Schema is a map.
	origSchema, ok := original.Tools[0].Output.Schema.(map[string]any)
	require.True(t, ok, "Pre-condition: Schema must be map[string]any")
	assert.Equal(t, "object", origSchema["type"])

	// Marshal and re-parse.
	marshalled, err := yaml.Marshal(original)
	require.NoError(t, err)

	roundTripped, err := Parse(strings.NewReader(string(marshalled)))
	require.NoError(t, err)
	require.Len(t, roundTripped.Tools, 1)

	// Post-condition: Schema is still a map with the same structure.
	rtSchema, ok := roundTripped.Tools[0].Output.Schema.(map[string]any)
	require.True(t, ok,
		"After round-trip, Schema must still be map[string]any, got %T",
		roundTripped.Tools[0].Output.Schema)
	assert.Equal(t, "object", rtSchema["type"])

	// Verify nested properties survived.
	rtProps, ok := rtSchema["properties"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, rtProps, "id")
	assert.Contains(t, rtProps, "name")

	idProp, ok := rtProps["id"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "integer", idProp["type"],
		"Nested property type must survive round-trip")

	// Verify required array survived.
	reqRaw := rtSchema["required"]
	reqSlice, ok := reqRaw.([]any)
	require.True(t, ok)
	require.Len(t, reqSlice, 1)
	assert.Equal(t, "id", reqSlice[0])
}

// ---------------------------------------------------------------------------
// AC3: Backward compat — MimeType round-trip
// ---------------------------------------------------------------------------

func TestOutput_RoundTrip_MimeType(t *testing.T) {
	input := outputManifestPrefix + `  - name: rt-mime
    description: Round-trip mimeType
    entrypoint: ./tool.sh
    output:
      format: binary
      mimeType: "image/png"
`
	original, err := Parse(strings.NewReader(input))
	require.NoError(t, err)
	require.Len(t, original.Tools, 1)
	assert.Equal(t, "image/png", original.Tools[0].Output.MimeType)

	marshalled, err := yaml.Marshal(original)
	require.NoError(t, err)

	// Verify the marshalled YAML contains "mimeType".
	assert.Contains(t, string(marshalled), "mimeType",
		"Marshalled YAML must contain 'mimeType' key")
	assert.Contains(t, string(marshalled), "image/png",
		"Marshalled YAML must contain the mimeType value")

	roundTripped, err := Parse(strings.NewReader(string(marshalled)))
	require.NoError(t, err)
	require.Len(t, roundTripped.Tools, 1)

	assert.Equal(t, "image/png", roundTripped.Tools[0].Output.MimeType,
		"MimeType must survive round-trip unchanged")
}

func TestOutput_RoundTrip_MimeTypeOmittedWhenEmpty(t *testing.T) {
	input := outputManifestPrefix + `  - name: rt-no-mime
    description: Round-trip omit mime
    entrypoint: ./tool.sh
    output:
      format: json
`
	original, err := Parse(strings.NewReader(input))
	require.NoError(t, err)
	require.Len(t, original.Tools, 1)
	assert.Equal(t, "", original.Tools[0].Output.MimeType)

	marshalled, err := yaml.Marshal(original)
	require.NoError(t, err)

	// MimeType must NOT appear in marshalled YAML when empty (omitempty).
	assert.NotContains(t, string(marshalled), "mimeType",
		"Marshalled YAML must not contain 'mimeType' when it is empty (omitempty)")

	roundTripped, err := Parse(strings.NewReader(string(marshalled)))
	require.NoError(t, err)
	require.Len(t, roundTripped.Tools, 1)
	assert.Equal(t, "", roundTripped.Tools[0].Output.MimeType,
		"MimeType must remain empty after round-trip")
}

// ---------------------------------------------------------------------------
// AC3: Backward compat — schema nil round-trip
// ---------------------------------------------------------------------------

func TestOutput_RoundTrip_SchemaOmittedWhenNil(t *testing.T) {
	input := outputManifestPrefix + `  - name: rt-nil-path
    description: Round-trip omit output path
    entrypoint: ./tool.sh
    output:
      format: text
`
	original, err := Parse(strings.NewReader(input))
	require.NoError(t, err)
	require.Len(t, original.Tools, 1)
	assert.Nil(t, original.Tools[0].Output.Schema)

	marshalled, err := yaml.Marshal(original)
	require.NoError(t, err)

	// Schema must NOT appear in marshalled YAML when nil (omitempty).
	assert.NotContains(t, string(marshalled), "schema",
		"Marshalled YAML must not contain 'schema' when it is nil")

	roundTripped, err := Parse(strings.NewReader(string(marshalled)))
	require.NoError(t, err)
	require.Len(t, roundTripped.Tools, 1)
	assert.Nil(t, roundTripped.Tools[0].Output.Schema,
		"Schema must remain nil after round-trip")
}

// ---------------------------------------------------------------------------
// AC3: Backward compat — full round-trip with all output fields
// ---------------------------------------------------------------------------

func TestOutput_RoundTrip_AllFields(t *testing.T) {
	input := outputManifestPrefix + `  - name: rt-full
    description: Round-trip all output fields
    entrypoint: ./tool.sh
    output:
      format: binary
      schema: "schemas/output.schema.json"
      mimeType: "image/png"
`
	original, err := Parse(strings.NewReader(input))
	require.NoError(t, err)

	marshalled, err := yaml.Marshal(original)
	require.NoError(t, err)

	yamlStr := string(marshalled)
	assert.Contains(t, yamlStr, "schema:", "intermediate YAML must contain schema key")
	assert.Contains(t, yamlStr, "mimeType:", "intermediate YAML must contain mimeType key")
	assert.Contains(t, yamlStr, "format:", "intermediate YAML must contain format key")

	roundTripped, err := Parse(strings.NewReader(yamlStr))
	require.NoError(t, err)

	if diff := cmp.Diff(original, roundTripped); diff != "" {
		t.Errorf("Round-trip mismatch (-original +roundTripped):\n%s", diff)
	}
}

func TestOutput_RoundTrip_InlineSchemaWithMimeType(t *testing.T) {
	input := outputManifestPrefix + `  - name: rt-full-inline
    description: Round-trip inline schema plus mimeType
    entrypoint: ./tool.sh
    output:
      format: binary
      schema:
        type: object
        properties:
          data:
            type: string
      mimeType: "application/octet-stream"
`
	original, err := Parse(strings.NewReader(input))
	require.NoError(t, err)

	marshalled, err := yaml.Marshal(original)
	require.NoError(t, err)

	yamlStr := string(marshalled)
	assert.Contains(t, yamlStr, "schema:", "intermediate YAML must contain schema key")
	assert.Contains(t, yamlStr, "mimeType:", "intermediate YAML must contain mimeType key")
	assert.Contains(t, yamlStr, "type: object", "intermediate YAML must contain inline schema type")

	roundTripped, err := Parse(strings.NewReader(yamlStr))
	require.NoError(t, err)

	if diff := cmp.Diff(original, roundTripped); diff != "" {
		t.Errorf("Round-trip mismatch (-original +roundTripped):\n%s", diff)
	}
}

// ---------------------------------------------------------------------------
// Full manifest integration: output fields in complete manifest
// ---------------------------------------------------------------------------

func TestOutput_FullManifest_StringSchema(t *testing.T) {
	// Verify the existing full manifest's output section still parses
	// correctly with the new Output type.
	input := `apiVersion: toolwright/v1
kind: Toolkit
metadata:
  name: petstore-tools
  version: 1.0.0
  description: Pet store management toolkit
tools:
  - name: list-pets
    description: List all pets
    entrypoint: ./list.sh
    output:
      format: json
      schema: schemas/pets.schema.json
  - name: add-pet
    description: Add a pet
    entrypoint: ./add.sh
    output:
      format: json
  - name: render-image
    description: Render pet image
    entrypoint: ./render.sh
    output:
      format: binary
      mimeType: "image/png"
  - name: analyze
    description: Analyze data
    entrypoint: ./analyze.sh
    output:
      format: json
      schema:
        type: object
        properties:
          score:
            type: number
          label:
            type: string
`
	got, err := Parse(strings.NewReader(input))
	require.NoError(t, err)
	require.Len(t, got.Tools, 4)

	// Tool 0: string schema.
	t0 := got.Tools[0].Output
	assert.Equal(t, "json", t0.Format)
	s, ok := t0.Schema.(string)
	require.True(t, ok, "Tool 0 Schema must be string, got %T", t0.Schema)
	assert.Equal(t, "schemas/pets.schema.json", s)
	assert.Equal(t, "", t0.MimeType)

	// Tool 1: no schema, no mimeType.
	t1 := got.Tools[1].Output
	assert.Equal(t, "json", t1.Format)
	assert.Nil(t, t1.Schema)
	assert.Equal(t, "", t1.MimeType)

	// Tool 2: binary with mimeType, no schema.
	t2 := got.Tools[2].Output
	assert.Equal(t, "binary", t2.Format)
	assert.Nil(t, t2.Schema)
	assert.Equal(t, "image/png", t2.MimeType)

	// Tool 3: inline schema, no mimeType.
	t3 := got.Tools[3].Output
	assert.Equal(t, "json", t3.Format)
	m, ok := t3.Schema.(map[string]any)
	require.True(t, ok, "Tool 3 Schema must be map[string]any, got %T", t3.Schema)
	assert.Equal(t, "object", m["type"])
	assert.Equal(t, "", t3.MimeType)
}

// ---------------------------------------------------------------------------
// Adversarial: struct field existence and YAML tag correctness
// ---------------------------------------------------------------------------

func TestOutput_StructFields_Exist(t *testing.T) {
	// Compile-time verification that Output has the expected fields with the
	// correct types. If Schema is still `string`, this will fail to compile
	// because we assign a map to it.
	o := Output{
		Format:   "binary",
		Schema:   map[string]any{"type": "object"},
		MimeType: "image/png",
	}

	assert.Equal(t, "binary", o.Format)
	m, ok := o.Schema.(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "object", m["type"])
	assert.Equal(t, "image/png", o.MimeType)

	// Also verify string assignment works (Schema is `any`).
	o2 := Output{
		Schema: "path/to/schema.json",
	}
	s, ok := o2.Schema.(string)
	require.True(t, ok)
	assert.Equal(t, "path/to/schema.json", s)
}

func TestOutput_StructFields_ZeroValue(t *testing.T) {
	// Zero-value Output must have nil Schema and empty MimeType.
	o := Output{}
	assert.Equal(t, "", o.Format)
	assert.Nil(t, o.Schema, "Zero-value Output.Schema must be nil (any zero value)")
	assert.Equal(t, "", o.MimeType, "Zero-value Output.MimeType must be empty")
}

// ---------------------------------------------------------------------------
// Adversarial: schema YAML tag correctness
// ---------------------------------------------------------------------------

func TestOutput_Schema_YAMLTagIsCorrect(t *testing.T) {
	// Only the exact YAML key "schema" should populate the Schema field.
	wrongKeys := []struct {
		name    string
		yamlKey string
	}{
		{name: "PascalCase Schema", yamlKey: "Schema"},
		{name: "uppercase SCHEMA", yamlKey: "SCHEMA"},
		{name: "snake_case output_schema", yamlKey: "output_schema"},
		{name: "camelCase outputSchema", yamlKey: "outputSchema"},
	}

	for _, tc := range wrongKeys {
		t.Run(tc.name, func(t *testing.T) {
			input := outputManifestPrefix + `  - name: tag-test
    description: Schema tag test
    entrypoint: ./tool.sh
    output:
      format: json
      ` + tc.yamlKey + `: "path.json"
`
			got, err := Parse(strings.NewReader(input))
			require.NoError(t, err)
			require.Len(t, got.Tools, 1)
			assert.Nil(t, got.Tools[0].Output.Schema,
				"Schema must be nil when YAML key is %q (only 'schema' should work)",
				tc.yamlKey)
		})
	}
}

// ---------------------------------------------------------------------------
// Adversarial: Output format field is preserved alongside new fields
// ---------------------------------------------------------------------------

func TestOutput_Format_PreservedWithNewFields(t *testing.T) {
	// Ensure the introduction of Schema union type and MimeType does not
	// break the existing Format field parsing.
	tests := []struct {
		name       string
		format     string
		wantFormat string
	}{
		{name: "json format", format: "json", wantFormat: "json"},
		{name: "text format", format: "text", wantFormat: "text"},
		{name: "binary format", format: "binary", wantFormat: "binary"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			input := outputManifestPrefix + `  - name: format-test
    description: Format preservation test
    entrypoint: ./tool.sh
    output:
      format: ` + tc.format + `
`
			got, err := Parse(strings.NewReader(input))
			require.NoError(t, err)
			require.Len(t, got.Tools, 1)
			assert.Equal(t, tc.wantFormat, got.Tools[0].Output.Format)
		})
	}
}

// ---------------------------------------------------------------------------
// Adversarial: Union type discrimination — string vs map
// ---------------------------------------------------------------------------

func TestOutput_Schema_UnionDiscrimination(t *testing.T) {
	// A lazy implementation might always return one type or the other.
	// This test verifies BOTH forms work in the same manifest.
	input := outputManifestPrefix + `  - name: tool-with-string-schema
    description: String schema tool
    entrypoint: ./a.sh
    output:
      format: json
      schema: "schemas/output.json"
  - name: tool-with-inline-schema
    description: Inline schema tool
    entrypoint: ./b.sh
    output:
      format: json
      schema:
        type: object
        properties:
          result:
            type: string
`
	got, err := Parse(strings.NewReader(input))
	require.NoError(t, err)
	require.Len(t, got.Tools, 2)

	// First tool: string schema.
	schema0 := got.Tools[0].Output.Schema
	_, isString := schema0.(string)
	_, isMap := schema0.(map[string]any)
	assert.True(t, isString, "First tool's Schema must be string, got %T", schema0)
	assert.False(t, isMap, "First tool's Schema must not be map")

	// Second tool: map schema.
	schema1 := got.Tools[1].Output.Schema
	_, isString = schema1.(string)
	_, isMap = schema1.(map[string]any)
	assert.False(t, isString, "Second tool's Schema must not be string, got %T", schema1)
	assert.True(t, isMap, "Second tool's Schema must be map[string]any")

	// Verify the actual values.
	assert.Equal(t, "schemas/output.json", got.Tools[0].Output.Schema)

	m := got.Tools[1].Output.Schema.(map[string]any)
	assert.Equal(t, "object", m["type"])
	props, ok := m["properties"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, props, "result")
}

// ---------------------------------------------------------------------------
// Adversarial: catch hardcoded returns
// ---------------------------------------------------------------------------

func TestOutput_Schema_NotHardcoded(t *testing.T) {
	// Parse two different string schema values and verify they differ.
	// This catches a hardcoded return.
	input1 := outputManifestPrefix + `  - name: t1
    description: First
    entrypoint: ./t1.sh
    output:
      format: json
      schema: "first/path.json"
`
	input2 := outputManifestPrefix + `  - name: t2
    description: Second
    entrypoint: ./t2.sh
    output:
      format: json
      schema: "second/different.json"
`
	got1, err := Parse(strings.NewReader(input1))
	require.NoError(t, err)
	got2, err := Parse(strings.NewReader(input2))
	require.NoError(t, err)

	s1, ok := got1.Tools[0].Output.Schema.(string)
	require.True(t, ok)
	s2, ok := got2.Tools[0].Output.Schema.(string)
	require.True(t, ok)

	assert.NotEqual(t, s1, s2,
		"Different schema inputs must produce different schema values")
	assert.Equal(t, "first/path.json", s1)
	assert.Equal(t, "second/different.json", s2)
}

func TestOutput_MimeType_NotHardcoded(t *testing.T) {
	// Parse two different mimeType values and verify they differ.
	input1 := outputManifestPrefix + `  - name: t1
    description: First
    entrypoint: ./t1.sh
    output:
      format: binary
      mimeType: "image/png"
`
	input2 := outputManifestPrefix + `  - name: t2
    description: Second
    entrypoint: ./t2.sh
    output:
      format: binary
      mimeType: "application/pdf"
`
	got1, err := Parse(strings.NewReader(input1))
	require.NoError(t, err)
	got2, err := Parse(strings.NewReader(input2))
	require.NoError(t, err)

	assert.NotEqual(t, got1.Tools[0].Output.MimeType, got2.Tools[0].Output.MimeType,
		"Different mimeType inputs must produce different values")
	assert.Equal(t, "image/png", got1.Tools[0].Output.MimeType)
	assert.Equal(t, "application/pdf", got2.Tools[0].Output.MimeType)
}

// ---------------------------------------------------------------------------
// Table-driven: Output parsing combinations
// ---------------------------------------------------------------------------

func TestOutput_Parse_Combinations(t *testing.T) {
	tests := []struct {
		name           string
		outputYAML     string
		wantFormat     string
		wantSchemaType string // "string", "map", or "nil"
		wantSchemaStr  string // if wantSchemaType == "string"
		wantMimeType   string
	}{
		{
			name:           "json with string schema",
			outputYAML:     "format: json\n      schema: \"path.json\"",
			wantFormat:     "json",
			wantSchemaType: "string",
			wantSchemaStr:  "path.json",
			wantMimeType:   "",
		},
		{
			name:           "json with no schema",
			outputYAML:     "format: json",
			wantFormat:     "json",
			wantSchemaType: "nil",
			wantMimeType:   "",
		},
		{
			name:           "binary with mimeType",
			outputYAML:     "format: binary\n      mimeType: \"image/jpeg\"",
			wantFormat:     "binary",
			wantSchemaType: "nil",
			wantMimeType:   "image/jpeg",
		},
		{
			name:           "binary with mimeType and schema",
			outputYAML:     "format: binary\n      schema: \"out.json\"\n      mimeType: \"image/png\"",
			wantFormat:     "binary",
			wantSchemaType: "string",
			wantSchemaStr:  "out.json",
			wantMimeType:   "image/png",
		},
		{
			name:           "text with no extras",
			outputYAML:     "format: text",
			wantFormat:     "text",
			wantSchemaType: "nil",
			wantMimeType:   "",
		},
		{
			name:           "json with inline schema",
			outputYAML:     "format: json\n      schema:\n        type: object",
			wantFormat:     "json",
			wantSchemaType: "map",
			wantMimeType:   "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			input := outputManifestPrefix + `  - name: combo-test
    description: Combination test
    entrypoint: ./tool.sh
    output:
      ` + tc.outputYAML + "\n"

			got, err := Parse(strings.NewReader(input))
			require.NoError(t, err, "Parse should succeed")
			require.Len(t, got.Tools, 1)

			output := got.Tools[0].Output
			assert.Equal(t, tc.wantFormat, output.Format, "Format")
			assert.Equal(t, tc.wantMimeType, output.MimeType, "MimeType")

			switch tc.wantSchemaType {
			case "nil":
				assert.Nil(t, output.Schema, "Schema should be nil")
			case "string":
				s, ok := output.Schema.(string)
				require.True(t, ok, "Schema should be string, got %T", output.Schema)
				assert.Equal(t, tc.wantSchemaStr, s)
			case "map":
				_, ok := output.Schema.(map[string]any)
				require.True(t, ok, "Schema should be map[string]any, got %T", output.Schema)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Adversarial: Other Tool fields are not disturbed by Output changes
// ---------------------------------------------------------------------------

func TestOutput_OtherToolFieldsPreserved(t *testing.T) {
	input := outputManifestPrefix + `  - name: full-tool
    description: Tool with all fields plus new output
    entrypoint: ./run.sh
    args:
      - name: input
        type: string
        required: true
        description: Input value
    flags:
      - name: verbose
        type: bool
        required: false
        default: false
        description: Enable verbose
    output:
      format: binary
      schema:
        type: object
        properties:
          data:
            type: string
      mimeType: "image/png"
    examples:
      - description: Run example
        args:
          - foo
    exit_codes:
      0: success
      1: failure
`
	got, err := Parse(strings.NewReader(input))
	require.NoError(t, err)
	require.Len(t, got.Tools, 1)

	tool := got.Tools[0]

	// Verify all non-output fields are intact.
	assert.Equal(t, "full-tool", tool.Name)
	assert.Equal(t, "Tool with all fields plus new output", tool.Description)
	assert.Equal(t, "./run.sh", tool.Entrypoint)
	require.Len(t, tool.Args, 1)
	assert.Equal(t, "input", tool.Args[0].Name)
	require.Len(t, tool.Flags, 1)
	assert.Equal(t, "verbose", tool.Flags[0].Name)
	require.Len(t, tool.Examples, 1)
	assert.Equal(t, "Run example", tool.Examples[0].Description)
	require.Len(t, tool.ExitCodes, 2)

	// And the output fields are correct.
	assert.Equal(t, "binary", tool.Output.Format)
	_, ok := tool.Output.Schema.(map[string]any)
	require.True(t, ok, "Schema must be map[string]any")
	assert.Equal(t, "image/png", tool.Output.MimeType)
}

// ---------------------------------------------------------------------------
// Adversarial: Output with schema as integer/boolean (unexpected types)
// The custom UnmarshalYAML should handle whatever YAML sends. Per plan,
// Schema ends up as string or map[string]any. Integer scalars from YAML
// would arrive as int through the `any` field. The parser should not crash.
// ---------------------------------------------------------------------------

func TestOutput_Schema_IntegerValueDoesNotPanic(t *testing.T) {
	input := outputManifestPrefix + `  - name: int-schema
    description: Schema set to integer
    entrypoint: ./tool.sh
    output:
      format: json
      schema: 42
`
	// Must not panic. Parse may succeed (storing int in any) or error —
	// either is acceptable, but we assert the outcome explicitly.
	var parsed *Toolkit
	var parseErr error
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("Parse panicked on integer schema: %v", r)
			}
		}()
		parsed, parseErr = Parse(strings.NewReader(input))
	}()
	if parseErr != nil {
		return // error is an acceptable outcome
	}
	require.Len(t, parsed.Tools, 1)
	assert.Equal(t, 42, parsed.Tools[0].Output.Schema,
		"integer schema should be stored as-is in the any field")
}

func TestOutput_Schema_BooleanValueDoesNotPanic(t *testing.T) {
	input := outputManifestPrefix + `  - name: bool-schema
    description: Schema set to boolean
    entrypoint: ./tool.sh
    output:
      format: json
      schema: true
`
	var parsed *Toolkit
	var parseErr error
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("Parse panicked on boolean schema: %v", r)
			}
		}()
		parsed, parseErr = Parse(strings.NewReader(input))
	}()
	if parseErr != nil {
		return
	}
	require.Len(t, parsed.Tools, 1)
	assert.Equal(t, true, parsed.Tools[0].Output.Schema,
		"boolean schema should be stored as-is in the any field")
}

func TestOutput_Schema_ArrayValueDoesNotPanic(t *testing.T) {
	input := outputManifestPrefix + `  - name: array-schema
    description: Schema set to array
    entrypoint: ./tool.sh
    output:
      format: json
      schema:
        - item1
        - item2
`
	var parsed *Toolkit
	var parseErr error
	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("Parse panicked on array schema: %v", r)
			}
		}()
		parsed, parseErr = Parse(strings.NewReader(input))
	}()
	if parseErr != nil {
		return
	}
	require.Len(t, parsed.Tools, 1)
	assert.IsType(t, []any{}, parsed.Tools[0].Output.Schema,
		"array schema should be stored as []any in the any field")
}
