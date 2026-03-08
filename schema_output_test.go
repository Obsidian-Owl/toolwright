package toolwright_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// AC-8: JSON Schema accepts output union (schema oneOf) and mimeType
// ---------------------------------------------------------------------------

// minimalManifestWithOutput returns a full valid manifest JSON string with
// the given output JSON fragment embedded in the first tool. This avoids
// duplicate boilerplate while ensuring every test exercises real schema
// validation with all required top-level fields present.
func minimalManifestWithOutput(outputJSON string) string {
	return `{
		"apiVersion": "toolwright/v1",
		"kind": "Toolkit",
		"metadata": {
			"name": "output-test",
			"version": "1.0.0",
			"description": "Output schema tests"
		},
		"tools": [{
			"name": "out-tool",
			"description": "Tool with output",
			"entrypoint": "./tool.sh",
			"output": ` + outputJSON + `
		}]
	}`
}

// ---------------------------------------------------------------------------
// Schema structure tests: inspect raw JSON Schema definition for output
// ---------------------------------------------------------------------------

// navigateToOutputProps is a test helper that loads the schema, parses it,
// and returns the properties object inside tools.items.properties.output.
func navigateToOutputProps(t *testing.T) map[string]any {
	t.Helper()
	data := loadSchemaBytes(t)

	var raw map[string]any
	err := json.Unmarshal(data, &raw)
	require.NoError(t, err, "schema must parse as valid JSON")

	props := raw["properties"].(map[string]any)
	tools := props["tools"].(map[string]any)
	items := tools["items"].(map[string]any)
	itemProps := items["properties"].(map[string]any)
	output := itemProps["output"].(map[string]any)
	outputProps, ok := output["properties"].(map[string]any)
	require.True(t, ok, "output must have a 'properties' object")
	return outputProps
}

func TestSchemaOutput_StructureSchemaIsOneOf(t *testing.T) {
	// output.schema MUST be defined as oneOf (union type), not plain string.
	// This test catches a schema that still has "schema": {"type": "string"}.
	outputProps := navigateToOutputProps(t)

	schemaDef, ok := outputProps["schema"].(map[string]any)
	require.True(t, ok, "output.properties must contain 'schema'")

	// It must NOT have a top-level "type" key — it should use "oneOf" instead.
	_, hasType := schemaDef["type"]
	assert.False(t, hasType,
		"output.schema must NOT have a top-level 'type' field; it should use 'oneOf' instead")

	// It must have a "oneOf" key.
	oneOf, hasOneOf := schemaDef["oneOf"]
	require.True(t, hasOneOf,
		"output.schema must have a 'oneOf' field for the string|object union")

	// oneOf must be an array.
	oneOfSlice, ok := oneOf.([]any)
	require.True(t, ok, "output.schema.oneOf must be an array")

	// Must have exactly 2 entries.
	require.Len(t, oneOfSlice, 2,
		"output.schema.oneOf must have exactly 2 entries (string and object)")
}

func TestSchemaOutput_StructureOneOfContainsStringAndObject(t *testing.T) {
	// Verify the oneOf array contains exactly {type: "string"} and {type: "object"}.
	// This catches a schema that has oneOf but with wrong types (e.g., two strings).
	outputProps := navigateToOutputProps(t)
	schemaDef := outputProps["schema"].(map[string]any)

	oneOfRaw, ok := schemaDef["oneOf"]
	require.True(t, ok, "output.schema must have 'oneOf'")

	oneOfSlice := oneOfRaw.([]any)
	require.Len(t, oneOfSlice, 2)

	// Collect the type values from each entry.
	types := make(map[string]bool)
	for i, entry := range oneOfSlice {
		entryMap, ok := entry.(map[string]any)
		require.True(t, ok, "oneOf[%d] must be an object", i)
		typVal, ok := entryMap["type"].(string)
		require.True(t, ok, "oneOf[%d] must have a 'type' string field", i)
		types[typVal] = true
	}

	assert.True(t, types["string"],
		"output.schema.oneOf must include {type: \"string\"}")
	assert.True(t, types["object"],
		"output.schema.oneOf must include {type: \"object\"}")
	assert.Len(t, types, 2,
		"output.schema.oneOf must contain exactly 2 distinct types, got: %v", types)
}

func TestSchemaOutput_StructureMimeTypeExists(t *testing.T) {
	// output.mimeType must exist as {type: "string"}.
	// This test fails against the current schema which has no mimeType.
	outputProps := navigateToOutputProps(t)

	mimeTypeDef, ok := outputProps["mimeType"].(map[string]any)
	require.True(t, ok,
		"output.properties must contain 'mimeType'")

	mimeTypeType, ok := mimeTypeDef["type"].(string)
	require.True(t, ok,
		"output.properties.mimeType must have a 'type' field")
	assert.Equal(t, "string", mimeTypeType,
		"output.mimeType type must be 'string', got %q", mimeTypeType)
}

func TestSchemaOutput_StructureFormatStillExists(t *testing.T) {
	// Backward compat: output.format must still be {type: "string"}.
	// This catches an implementation that accidentally removes format.
	outputProps := navigateToOutputProps(t)

	formatDef, ok := outputProps["format"].(map[string]any)
	require.True(t, ok,
		"output.properties must still contain 'format'")

	formatType, ok := formatDef["type"].(string)
	require.True(t, ok,
		"output.properties.format must have a 'type' field")
	assert.Equal(t, "string", formatType,
		"output.format type must be 'string', got %q", formatType)
}

func TestSchemaOutput_StructureOutputPropertyCount(t *testing.T) {
	// After the change, output should have exactly 3 properties:
	// format, schema, mimeType. This catches extra or missing fields.
	outputProps := navigateToOutputProps(t)

	expected := []string{"format", "schema", "mimeType"}
	assert.Len(t, outputProps, len(expected),
		"output must have exactly %d properties, got %d: %v",
		len(expected), len(outputProps), mapKeys(outputProps))

	for _, name := range expected {
		_, ok := outputProps[name]
		assert.True(t, ok, "output.properties must contain %q", name)
	}
}

// ---------------------------------------------------------------------------
// Validation positive tests: manifests that must pass schema validation
// ---------------------------------------------------------------------------

func TestSchemaOutput_SchemaAsString_Validates(t *testing.T) {
	// A string value for schema (file path reference) must validate.
	// This should pass even before the change.
	manifest := minimalManifestWithOutput(`{
		"format": "json",
		"schema": "path/to/file.json"
	}`)
	err := validateJSON(t, manifest)
	assert.NoError(t, err,
		"output with schema as string must pass validation")
}

func TestSchemaOutput_SchemaAsObject_Validates(t *testing.T) {
	// An inline JSON Schema object for schema must validate.
	// This is the key new behavior — MUST FAIL before implementation.
	manifest := minimalManifestWithOutput(`{
		"format": "json",
		"schema": {
			"type": "object",
			"properties": {
				"name": { "type": "string" }
			}
		}
	}`)
	err := validateJSON(t, manifest)
	assert.NoError(t, err,
		"output with schema as inline object must pass validation")
}

func TestSchemaOutput_SchemaAsEmptyObject_Validates(t *testing.T) {
	// An empty object is still a valid JSON Schema — must be accepted.
	manifest := minimalManifestWithOutput(`{
		"format": "json",
		"schema": {}
	}`)
	err := validateJSON(t, manifest)
	assert.NoError(t, err,
		"output with schema as empty object must pass validation")
}

func TestSchemaOutput_SchemaAsComplexObject_Validates(t *testing.T) {
	// A deeply nested inline JSON Schema must be accepted, not just shallow objects.
	manifest := minimalManifestWithOutput(`{
		"format": "json",
		"schema": {
			"type": "array",
			"items": {
				"type": "object",
				"required": ["id"],
				"properties": {
					"id": { "type": "integer" },
					"tags": {
						"type": "array",
						"items": { "type": "string" }
					}
				}
			}
		}
	}`)
	err := validateJSON(t, manifest)
	assert.NoError(t, err,
		"output with schema as complex nested object must pass validation")
}

func TestSchemaOutput_MimeType_Validates(t *testing.T) {
	// mimeType as a string must validate.
	// MUST FAIL before implementation (no mimeType in current schema).
	manifest := minimalManifestWithOutput(`{
		"format": "binary",
		"mimeType": "image/png"
	}`)
	err := validateJSON(t, manifest)
	assert.NoError(t, err,
		"output with mimeType as string must pass validation")
}

func TestSchemaOutput_MimeTypeVariousValues_Validates(t *testing.T) {
	// Various realistic MIME type strings must all pass.
	mimeTypes := []string{
		"application/json",
		"application/octet-stream",
		"image/png",
		"image/jpeg",
		"text/plain",
		"application/pdf",
		"audio/mpeg",
		"video/mp4",
	}
	for _, mime := range mimeTypes {
		t.Run(mime, func(t *testing.T) {
			mimeJSON, err := json.Marshal(mime)
			require.NoError(t, err)
			manifest := minimalManifestWithOutput(`{
				"format": "binary",
				"mimeType": ` + string(mimeJSON) + `
			}`)
			err = validateJSON(t, manifest)
			assert.NoError(t, err,
				"output with mimeType %q must pass validation", mime)
		})
	}
}

func TestSchemaOutput_BothSchemaStringAndMimeType_Validates(t *testing.T) {
	// Using both schema (string) and mimeType together must pass.
	manifest := minimalManifestWithOutput(`{
		"format": "json",
		"schema": "schemas/response.json",
		"mimeType": "application/json"
	}`)
	err := validateJSON(t, manifest)
	assert.NoError(t, err,
		"output with string schema and mimeType must pass validation")
}

func TestSchemaOutput_BothSchemaObjectAndMimeType_Validates(t *testing.T) {
	// Using both schema (object) and mimeType together must pass.
	manifest := minimalManifestWithOutput(`{
		"format": "json",
		"schema": { "type": "object" },
		"mimeType": "application/json"
	}`)
	err := validateJSON(t, manifest)
	assert.NoError(t, err,
		"output with object schema and mimeType must pass validation")
}

func TestSchemaOutput_MimeTypeOnly_Validates(t *testing.T) {
	// Output with only mimeType (no format, no schema) must pass if
	// none of those fields are required.
	manifest := minimalManifestWithOutput(`{
		"mimeType": "image/png"
	}`)
	err := validateJSON(t, manifest)
	assert.NoError(t, err,
		"output with only mimeType must pass validation")
}

func TestSchemaOutput_AllThreeFields_Validates(t *testing.T) {
	// All three output fields simultaneously must pass.
	manifest := minimalManifestWithOutput(`{
		"format": "json",
		"schema": { "type": "string" },
		"mimeType": "application/json"
	}`)
	err := validateJSON(t, manifest)
	assert.NoError(t, err,
		"output with format, schema (object), and mimeType must all pass together")
}

// ---------------------------------------------------------------------------
// Backward compat: existing manifests must still validate
// ---------------------------------------------------------------------------

func TestSchemaOutput_FormatOnly_StillValidates(t *testing.T) {
	// A manifest with only format (no schema, no mimeType) must still pass.
	// This catches a regression that accidentally requires the new fields.
	manifest := minimalManifestWithOutput(`{
		"format": "json"
	}`)
	err := validateJSON(t, manifest)
	assert.NoError(t, err,
		"output with only format must still pass validation (backward compat)")
}

func TestSchemaOutput_EmptyOutputObject_StillValidates(t *testing.T) {
	// An empty output object must still pass (no required fields inside output).
	manifest := minimalManifestWithOutput(`{}`)
	err := validateJSON(t, manifest)
	assert.NoError(t, err,
		"empty output object must still pass validation (backward compat)")
}

func TestSchemaOutput_FormatAndStringSchema_StillValidates(t *testing.T) {
	// The old-style format+string schema must still validate after the change.
	manifest := minimalManifestWithOutput(`{
		"format": "json",
		"schema": "response.json"
	}`)
	err := validateJSON(t, manifest)
	assert.NoError(t, err,
		"output with format and string schema must still pass (backward compat)")
}

// ---------------------------------------------------------------------------
// Validation negative tests: manifests that must FAIL schema validation
// ---------------------------------------------------------------------------

func TestSchemaOutput_SchemaAsInteger_Fails(t *testing.T) {
	// schema must be string|object — an integer must be rejected.
	manifest := minimalManifestWithOutput(`{
		"format": "json",
		"schema": 42
	}`)
	err := validateJSON(t, manifest)
	assert.Error(t, err,
		"output with schema as integer 42 must fail validation")
}

func TestSchemaOutput_SchemaAsBoolean_Fails(t *testing.T) {
	// schema must be string|object — a boolean must be rejected.
	manifest := minimalManifestWithOutput(`{
		"format": "json",
		"schema": true
	}`)
	err := validateJSON(t, manifest)
	assert.Error(t, err,
		"output with schema as boolean true must fail validation")
}

func TestSchemaOutput_SchemaAsBooleanFalse_Fails(t *testing.T) {
	// Both true and false must be rejected — catches accepting "falsy" values.
	manifest := minimalManifestWithOutput(`{
		"format": "json",
		"schema": false
	}`)
	err := validateJSON(t, manifest)
	assert.Error(t, err,
		"output with schema as boolean false must fail validation")
}

func TestSchemaOutput_SchemaAsArray_Fails(t *testing.T) {
	// schema must be string|object — an array must be rejected.
	manifest := minimalManifestWithOutput(`{
		"format": "json",
		"schema": ["one", "two"]
	}`)
	err := validateJSON(t, manifest)
	assert.Error(t, err,
		"output with schema as array must fail validation")
}

func TestSchemaOutput_SchemaAsNull_Fails(t *testing.T) {
	// null is none of string|object — must be rejected.
	manifest := minimalManifestWithOutput(`{
		"format": "json",
		"schema": null
	}`)
	err := validateJSON(t, manifest)
	assert.Error(t, err,
		"output with schema as null must fail validation")
}

func TestSchemaOutput_SchemaAsNumber_Fails(t *testing.T) {
	// A floating point number is not string|object.
	manifest := minimalManifestWithOutput(`{
		"format": "json",
		"schema": 3.14
	}`)
	err := validateJSON(t, manifest)
	assert.Error(t, err,
		"output with schema as float 3.14 must fail validation")
}

func TestSchemaOutput_MimeTypeAsInteger_Fails(t *testing.T) {
	// mimeType must be a string — integer must be rejected.
	manifest := minimalManifestWithOutput(`{
		"format": "binary",
		"mimeType": 42
	}`)
	err := validateJSON(t, manifest)
	assert.Error(t, err,
		"output with mimeType as integer 42 must fail validation")
}

func TestSchemaOutput_MimeTypeAsBoolean_Fails(t *testing.T) {
	// mimeType must be a string — boolean must be rejected.
	manifest := minimalManifestWithOutput(`{
		"format": "binary",
		"mimeType": true
	}`)
	err := validateJSON(t, manifest)
	assert.Error(t, err,
		"output with mimeType as boolean must fail validation")
}

func TestSchemaOutput_MimeTypeAsObject_Fails(t *testing.T) {
	// mimeType must be a string — object must be rejected.
	manifest := minimalManifestWithOutput(`{
		"format": "binary",
		"mimeType": { "type": "image/png" }
	}`)
	err := validateJSON(t, manifest)
	assert.Error(t, err,
		"output with mimeType as object must fail validation")
}

func TestSchemaOutput_MimeTypeAsArray_Fails(t *testing.T) {
	// mimeType must be a string — array must be rejected.
	manifest := minimalManifestWithOutput(`{
		"format": "binary",
		"mimeType": ["image/png", "image/jpeg"]
	}`)
	err := validateJSON(t, manifest)
	assert.Error(t, err,
		"output with mimeType as array must fail validation")
}

func TestSchemaOutput_MimeTypeAsNull_Fails(t *testing.T) {
	// mimeType must be a string — null must be rejected.
	manifest := minimalManifestWithOutput(`{
		"format": "binary",
		"mimeType": null
	}`)
	err := validateJSON(t, manifest)
	assert.Error(t, err,
		"output with mimeType as null must fail validation")
}

func TestSchemaOutput_OutputAsNonObject_Fails(t *testing.T) {
	// output itself must be an object. String, array, number, bool must fail.
	wrongTypes := []struct {
		name  string
		value string
	}{
		{name: "string", value: `"json"`},
		{name: "number", value: `42`},
		{name: "array", value: `["json"]`},
		{name: "boolean", value: `true`},
	}
	for _, tc := range wrongTypes {
		t.Run(tc.name, func(t *testing.T) {
			manifest := minimalManifestWithOutput(tc.value)
			err := validateJSON(t, manifest)
			assert.Error(t, err,
				"output as %s must fail schema validation", tc.name)
		})
	}
}

// mapKeys returns the keys of a map for diagnostic messages.
func mapKeys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
