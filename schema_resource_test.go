package toolwright_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// AC-6: JSON Schema accepts manifests with a resources array
// ---------------------------------------------------------------------------

// minimalManifestWithResources returns a full valid manifest JSON string with
// the given resources JSON array embedded at the top level. The manifest
// includes the required tools array so it validates against all other rules.
func minimalManifestWithResources(resourcesJSON string) string {
	return `{
		"apiVersion": "toolwright/v1",
		"kind": "Toolkit",
		"metadata": {
			"name": "resource-test",
			"version": "1.0.0",
			"description": "Resource schema tests"
		},
		"tools": [{
			"name": "some-tool",
			"description": "A tool for testing",
			"entrypoint": "./tool.sh"
		}],
		"resources": ` + resourcesJSON + `
	}`
}

// ---------------------------------------------------------------------------
// Schema structure tests: inspect raw JSON Schema definition for resources
// ---------------------------------------------------------------------------

// navigateToResourceDef is a test helper that loads the schema, parses it,
// and returns the resources definition object from the top-level properties.
func navigateToResourceDef(t *testing.T) map[string]any {
	t.Helper()
	data := loadSchemaBytes(t)

	var raw map[string]any
	err := json.Unmarshal(data, &raw)
	require.NoError(t, err, "schema must parse as valid JSON")

	props, ok := raw["properties"].(map[string]any)
	require.True(t, ok, "schema must have a 'properties' object")

	resources, ok := props["resources"].(map[string]any)
	require.True(t, ok, "schema properties must contain 'resources'")
	return resources
}

// navigateToResourceItemProps returns the properties map inside the
// resources array item definition.
func navigateToResourceItemProps(t *testing.T) map[string]any {
	t.Helper()
	resources := navigateToResourceDef(t)

	items, ok := resources["items"].(map[string]any)
	require.True(t, ok, "resources must have an 'items' definition")

	itemProps, ok := items["properties"].(map[string]any)
	require.True(t, ok, "resources.items must have a 'properties' object")
	return itemProps
}

func TestSchemaResource_StructureResourcesExistsAtTopLevel(t *testing.T) {
	// The resources property must exist in the top-level properties.
	// This test MUST FAIL before implementation (no resources in current schema).
	data := loadSchemaBytes(t)

	var raw map[string]any
	err := json.Unmarshal(data, &raw)
	require.NoError(t, err, "schema must parse as valid JSON")

	props, ok := raw["properties"].(map[string]any)
	require.True(t, ok, "schema must have a 'properties' object")

	_, ok = props["resources"]
	assert.True(t, ok,
		"schema top-level properties must contain 'resources'")
}

func TestSchemaResource_StructureResourcesIsArrayType(t *testing.T) {
	// resources must be defined as type "array".
	resources := navigateToResourceDef(t)

	resType, ok := resources["type"].(string)
	require.True(t, ok, "resources must have a 'type' field")
	assert.Equal(t, "array", resType,
		"resources type must be 'array', got %q", resType)
}

func TestSchemaResource_StructureRequiredFields(t *testing.T) {
	// Resource items must require: uri, name, entrypoint.
	// This catches a schema that forgets to mark required fields.
	resources := navigateToResourceDef(t)

	items, ok := resources["items"].(map[string]any)
	require.True(t, ok, "resources must have an 'items' definition")

	requiredRaw, ok := items["required"].([]any)
	require.True(t, ok, "resources.items must have a 'required' array")

	// Collect required fields into a set.
	required := make(map[string]bool, len(requiredRaw))
	for _, r := range requiredRaw {
		s, ok := r.(string)
		require.True(t, ok, "each entry in required must be a string")
		required[s] = true
	}

	expectedRequired := []string{"uri", "name", "entrypoint"}
	for _, field := range expectedRequired {
		assert.True(t, required[field],
			"resource items must require field %q", field)
	}

	// Ensure optional fields are NOT in the required list.
	optionalFields := []string{"description", "mimeType"}
	for _, field := range optionalFields {
		assert.False(t, required[field],
			"resource items must NOT require optional field %q", field)
	}

	// Required count must match exactly — catches extra surprise required fields.
	assert.Len(t, requiredRaw, len(expectedRequired),
		"resources.items.required must have exactly %d entries, got %d",
		len(expectedRequired), len(requiredRaw))
}

func TestSchemaResource_StructureOptionalPropertiesExist(t *testing.T) {
	// description and mimeType must exist as defined properties.
	itemProps := navigateToResourceItemProps(t)

	descDef, ok := itemProps["description"].(map[string]any)
	require.True(t, ok, "resource item properties must contain 'description'")
	descType, ok := descDef["type"].(string)
	require.True(t, ok, "description must have a 'type' field")
	assert.Equal(t, "string", descType,
		"resource description type must be 'string', got %q", descType)

	mimeDef, ok := itemProps["mimeType"].(map[string]any)
	require.True(t, ok, "resource item properties must contain 'mimeType'")
	mimeType, ok := mimeDef["type"].(string)
	require.True(t, ok, "mimeType must have a 'type' field")
	assert.Equal(t, "string", mimeType,
		"resource mimeType type must be 'string', got %q", mimeType)
}

func TestSchemaResource_StructureRequiredPropertyTypes(t *testing.T) {
	// uri, name, entrypoint must all be type "string".
	itemProps := navigateToResourceItemProps(t)

	stringFields := []string{"uri", "name", "entrypoint"}
	for _, field := range stringFields {
		fieldDef, ok := itemProps[field].(map[string]any)
		require.True(t, ok, "resource item properties must contain %q", field)

		fieldType, ok := fieldDef["type"].(string)
		require.True(t, ok, "resource.%s must have a 'type' field", field)
		assert.Equal(t, "string", fieldType,
			"resource.%s type must be 'string', got %q", field, fieldType)
	}
}

func TestSchemaResource_StructureItemPropertyCount(t *testing.T) {
	// After the change, resource items should have exactly 5 properties:
	// uri, name, description, mimeType, entrypoint.
	// This catches extra or missing fields.
	itemProps := navigateToResourceItemProps(t)

	expected := []string{"uri", "name", "description", "mimeType", "entrypoint"}
	assert.Len(t, itemProps, len(expected),
		"resource items must have exactly %d properties, got %d: %v",
		len(expected), len(itemProps), resourceKeys(itemProps))

	for _, name := range expected {
		_, ok := itemProps[name]
		assert.True(t, ok, "resource item properties must contain %q", name)
	}
}

// ---------------------------------------------------------------------------
// Validation positive tests: valid manifests must pass schema validation
// ---------------------------------------------------------------------------

func TestSchemaResource_SingleResource_AllFields_Validates(t *testing.T) {
	// A single resource with all 5 fields must validate.
	manifest := minimalManifestWithResources(`[{
		"uri": "file:///data/config.json",
		"name": "config",
		"description": "Application configuration",
		"mimeType": "application/json",
		"entrypoint": "./resources/config.sh"
	}]`)
	err := validateJSON(t, manifest)
	assert.NoError(t, err,
		"manifest with single resource (all fields) must pass validation")
}

func TestSchemaResource_MultipleResources_Validates(t *testing.T) {
	// Multiple resources in the array must all validate.
	manifest := minimalManifestWithResources(`[
		{
			"uri": "file:///data/config.json",
			"name": "config",
			"description": "Config file",
			"mimeType": "application/json",
			"entrypoint": "./resources/config.sh"
		},
		{
			"uri": "file:///data/logo.png",
			"name": "logo",
			"description": "Brand logo",
			"mimeType": "image/png",
			"entrypoint": "./resources/logo.sh"
		},
		{
			"uri": "file:///data/readme.txt",
			"name": "readme",
			"description": "Documentation",
			"mimeType": "text/plain",
			"entrypoint": "./resources/readme.sh"
		}
	]`)
	err := validateJSON(t, manifest)
	assert.NoError(t, err,
		"manifest with multiple resources must pass validation")
}

func TestSchemaResource_WithToolsTogether_Validates(t *testing.T) {
	// Resources alongside tools in the same manifest must validate.
	// This is the primary coexistence test.
	manifest := `{
		"apiVersion": "toolwright/v1",
		"kind": "Toolkit",
		"metadata": {
			"name": "mixed-test",
			"version": "2.0.0",
			"description": "Manifest with tools and resources"
		},
		"tools": [{
			"name": "processor",
			"description": "Processes data",
			"entrypoint": "./process.sh",
			"args": [{
				"name": "input",
				"type": "string",
				"required": true,
				"description": "Input file"
			}],
			"output": {
				"format": "json"
			}
		}],
		"resources": [{
			"uri": "file:///templates/default.json",
			"name": "default-template",
			"description": "Default processing template",
			"mimeType": "application/json",
			"entrypoint": "./resources/template.sh"
		}]
	}`
	err := validateJSON(t, manifest)
	assert.NoError(t, err,
		"manifest with both tools and resources must pass validation")
}

func TestSchemaResource_OmitOptionalFields_Validates(t *testing.T) {
	// Resource with only required fields (uri, name, entrypoint) must validate.
	// description and mimeType are optional.
	manifest := minimalManifestWithResources(`[{
		"uri": "file:///data/item",
		"name": "minimal-resource",
		"entrypoint": "./resources/item.sh"
	}]`)
	err := validateJSON(t, manifest)
	assert.NoError(t, err,
		"resource with only required fields must pass validation")
}

func TestSchemaResource_OmitOnlyDescription_Validates(t *testing.T) {
	// Resource with mimeType but no description must validate.
	manifest := minimalManifestWithResources(`[{
		"uri": "file:///data/image.png",
		"name": "image",
		"mimeType": "image/png",
		"entrypoint": "./resources/image.sh"
	}]`)
	err := validateJSON(t, manifest)
	assert.NoError(t, err,
		"resource without description (but with mimeType) must pass validation")
}

func TestSchemaResource_OmitOnlyMimeType_Validates(t *testing.T) {
	// Resource with description but no mimeType must validate.
	manifest := minimalManifestWithResources(`[{
		"uri": "file:///data/notes",
		"name": "notes",
		"description": "Some notes",
		"entrypoint": "./resources/notes.sh"
	}]`)
	err := validateJSON(t, manifest)
	assert.NoError(t, err,
		"resource without mimeType (but with description) must pass validation")
}

func TestSchemaResource_EmptyResourcesArray_Validates(t *testing.T) {
	// An empty resources array must validate — having the field but no entries.
	manifest := minimalManifestWithResources(`[]`)
	err := validateJSON(t, manifest)
	assert.NoError(t, err,
		"manifest with empty resources array must pass validation")
}

func TestSchemaResource_VariousURIFormats_Validates(t *testing.T) {
	// Various URI formats must be accepted (uri is a string, not pattern-restricted).
	uris := []struct {
		name string
		uri  string
	}{
		{name: "file URI", uri: "file:///path/to/resource"},
		{name: "https URI", uri: "https://example.com/resource"},
		{name: "custom scheme", uri: "custom://resource/id"},
		{name: "relative path", uri: "data/local-file.json"},
		{name: "URN style", uri: "urn:toolwright:resource:123"},
	}
	for _, tc := range uris {
		t.Run(tc.name, func(t *testing.T) {
			uriJSON, err := json.Marshal(tc.uri)
			require.NoError(t, err)
			manifest := minimalManifestWithResources(`[{
				"uri": ` + string(uriJSON) + `,
				"name": "uri-test",
				"entrypoint": "./resources/test.sh"
			}]`)
			err = validateJSON(t, manifest)
			assert.NoError(t, err,
				"resource with URI %q must pass validation", tc.uri)
		})
	}
}

func TestSchemaResource_VariousMimeTypes_Validates(t *testing.T) {
	// Various MIME type strings must be accepted.
	mimeTypes := []string{
		"application/json",
		"application/octet-stream",
		"image/png",
		"text/plain",
		"text/html",
		"application/pdf",
	}
	for _, mime := range mimeTypes {
		t.Run(mime, func(t *testing.T) {
			mimeJSON, err := json.Marshal(mime)
			require.NoError(t, err)
			manifest := minimalManifestWithResources(`[{
				"uri": "file:///data/item",
				"name": "mime-test",
				"mimeType": ` + string(mimeJSON) + `,
				"entrypoint": "./resources/item.sh"
			}]`)
			err = validateJSON(t, manifest)
			assert.NoError(t, err,
				"resource with mimeType %q must pass validation", mime)
		})
	}
}

// ---------------------------------------------------------------------------
// Backward compatibility: manifests without resources must still validate
// ---------------------------------------------------------------------------

func TestSchemaResource_NoResourcesField_BackwardCompat(t *testing.T) {
	// A manifest without the resources field must still pass.
	// This is critical: resources are optional at the manifest level.
	manifest := `{
		"apiVersion": "toolwright/v1",
		"kind": "Toolkit",
		"metadata": {
			"name": "no-resources",
			"version": "1.0.0",
			"description": "Manifest without resources"
		},
		"tools": [{
			"name": "basic-tool",
			"description": "A basic tool",
			"entrypoint": "./tool.sh"
		}]
	}`
	err := validateJSON(t, manifest)
	assert.NoError(t, err,
		"manifest without resources field must still pass validation (backward compat)")
}

// ---------------------------------------------------------------------------
// Validation negative tests: invalid manifests must fail schema validation
// ---------------------------------------------------------------------------

func TestSchemaResource_MissingURI_Fails(t *testing.T) {
	// A resource without uri must fail validation because uri is required.
	manifest := minimalManifestWithResources(`[{
		"name": "no-uri",
		"description": "Missing URI",
		"entrypoint": "./resources/test.sh"
	}]`)
	err := validateJSON(t, manifest)
	require.Error(t, err,
		"resource missing 'uri' must fail schema validation")
	assert.Contains(t, err.Error(), "uri",
		"error must reference the missing 'uri' field")
}

func TestSchemaResource_MissingName_Fails(t *testing.T) {
	// A resource without name must fail validation because name is required.
	manifest := minimalManifestWithResources(`[{
		"uri": "file:///data/item",
		"description": "Missing name",
		"entrypoint": "./resources/test.sh"
	}]`)
	err := validateJSON(t, manifest)
	require.Error(t, err,
		"resource missing 'name' must fail schema validation")
	assert.Contains(t, err.Error(), "name",
		"error must reference the missing 'name' field")
}

func TestSchemaResource_MissingEntrypoint_Fails(t *testing.T) {
	// A resource without entrypoint must fail validation because entrypoint is required.
	manifest := minimalManifestWithResources(`[{
		"uri": "file:///data/item",
		"name": "no-entrypoint",
		"description": "Missing entrypoint"
	}]`)
	err := validateJSON(t, manifest)
	require.Error(t, err,
		"resource missing 'entrypoint' must fail schema validation")
	assert.Contains(t, err.Error(), "entrypoint",
		"error must reference the missing 'entrypoint' field")
}

func TestSchemaResource_MissingAllRequired_Fails(t *testing.T) {
	// A resource with only optional fields and no required fields must fail.
	manifest := minimalManifestWithResources(`[{
		"description": "Only optional fields",
		"mimeType": "text/plain"
	}]`)
	err := validateJSON(t, manifest)
	assert.Error(t, err,
		"resource with only optional fields must fail schema validation")
}

func TestSchemaResource_EmptyObject_Fails(t *testing.T) {
	// An empty object as a resource item must fail (missing all required fields).
	manifest := minimalManifestWithResources(`[{}]`)
	err := validateJSON(t, manifest)
	assert.Error(t, err,
		"empty resource object must fail schema validation (missing required fields)")
}

func TestSchemaResource_ResourcesAsNonArray_Fails(t *testing.T) {
	// resources must be an array. String, object, number, boolean, null must fail.
	wrongTypes := []struct {
		name  string
		value string
	}{
		{name: "string", value: `"not-an-array"`},
		{name: "object", value: `{"uri": "file:///x", "name": "x", "entrypoint": "./x"}`},
		{name: "number", value: `42`},
		{name: "boolean", value: `true`},
		{name: "null", value: `null`},
	}
	for _, tc := range wrongTypes {
		t.Run(tc.name, func(t *testing.T) {
			manifest := minimalManifestWithResources(tc.value)
			err := validateJSON(t, manifest)
			assert.Error(t, err,
				"resources as %s must fail schema validation", tc.name)
		})
	}
}

func TestSchemaResource_ResourceItemAsNonObject_Fails(t *testing.T) {
	// Each resource item must be an object. String, number, array, boolean, null must fail.
	wrongTypes := []struct {
		name  string
		value string
	}{
		{name: "string", value: `"file:///data/item"`},
		{name: "number", value: `42`},
		{name: "array", value: `["uri", "name"]`},
		{name: "boolean", value: `true`},
		{name: "null", value: `null`},
	}
	for _, tc := range wrongTypes {
		t.Run(tc.name, func(t *testing.T) {
			manifest := minimalManifestWithResources(`[` + tc.value + `]`)
			err := validateJSON(t, manifest)
			assert.Error(t, err,
				"resource item as %s must fail schema validation", tc.name)
		})
	}
}

func TestSchemaResource_URIWrongType_Fails(t *testing.T) {
	// uri must be a string — number and object must be rejected.
	wrongTypes := []struct {
		name  string
		value string
	}{
		{name: "number", value: `42`},
		{name: "boolean", value: `true`},
		{name: "array", value: `["file:///x"]`},
		{name: "object", value: `{"path": "/x"}`},
		{name: "null", value: `null`},
	}
	for _, tc := range wrongTypes {
		t.Run(tc.name, func(t *testing.T) {
			manifest := minimalManifestWithResources(`[{
				"uri": ` + tc.value + `,
				"name": "wrong-uri",
				"entrypoint": "./resources/test.sh"
			}]`)
			err := validateJSON(t, manifest)
			require.Error(t, err,
				"resource with uri as %s must fail schema validation", tc.name)
			assert.Contains(t, err.Error(), "uri",
				"error must reference the 'uri' field")
		})
	}
}

func TestSchemaResource_NameWrongType_Fails(t *testing.T) {
	// name must be a string — other types must be rejected.
	wrongTypes := []struct {
		name  string
		value string
	}{
		{name: "number", value: `123`},
		{name: "boolean", value: `false`},
		{name: "array", value: `["name1"]`},
		{name: "object", value: `{"n": "x"}`},
	}
	for _, tc := range wrongTypes {
		t.Run(tc.name, func(t *testing.T) {
			manifest := minimalManifestWithResources(`[{
				"uri": "file:///data/item",
				"name": ` + tc.value + `,
				"entrypoint": "./resources/test.sh"
			}]`)
			err := validateJSON(t, manifest)
			require.Error(t, err,
				"resource with name as %s must fail schema validation", tc.name)
			assert.Contains(t, err.Error(), "name",
				"error must reference the 'name' field")
		})
	}
}

func TestSchemaResource_EntrypointWrongType_Fails(t *testing.T) {
	// entrypoint must be a string — other types must be rejected.
	wrongTypes := []struct {
		name  string
		value string
	}{
		{name: "number", value: `99`},
		{name: "boolean", value: `true`},
		{name: "array", value: `["./a.sh"]`},
		{name: "object", value: `{"cmd": "./a.sh"}`},
	}
	for _, tc := range wrongTypes {
		t.Run(tc.name, func(t *testing.T) {
			manifest := minimalManifestWithResources(`[{
				"uri": "file:///data/item",
				"name": "wrong-ep",
				"entrypoint": ` + tc.value + `
			}]`)
			err := validateJSON(t, manifest)
			require.Error(t, err,
				"resource with entrypoint as %s must fail schema validation", tc.name)
			assert.Contains(t, err.Error(), "entrypoint",
				"error must reference the 'entrypoint' field")
		})
	}
}

func TestSchemaResource_MimeTypeWrongType_Fails(t *testing.T) {
	// mimeType must be a string — other types must be rejected.
	wrongTypes := []struct {
		name  string
		value string
	}{
		{name: "number", value: `42`},
		{name: "boolean", value: `true`},
		{name: "array", value: `["text/plain"]`},
		{name: "object", value: `{"type": "text/plain"}`},
	}
	for _, tc := range wrongTypes {
		t.Run(tc.name, func(t *testing.T) {
			manifest := minimalManifestWithResources(`[{
				"uri": "file:///data/item",
				"name": "wrong-mime",
				"entrypoint": "./resources/test.sh",
				"mimeType": ` + tc.value + `
			}]`)
			err := validateJSON(t, manifest)
			assert.Error(t, err,
				"resource with mimeType as %s must fail schema validation", tc.name)
		})
	}
}

func TestSchemaResource_DescriptionWrongType_Fails(t *testing.T) {
	// description must be a string — other types must be rejected.
	wrongTypes := []struct {
		name  string
		value string
	}{
		{name: "number", value: `42`},
		{name: "boolean", value: `true`},
		{name: "array", value: `["desc"]`},
		{name: "object", value: `{"text": "desc"}`},
	}
	for _, tc := range wrongTypes {
		t.Run(tc.name, func(t *testing.T) {
			manifest := minimalManifestWithResources(`[{
				"uri": "file:///data/item",
				"name": "wrong-desc",
				"entrypoint": "./resources/test.sh",
				"description": ` + tc.value + `
			}]`)
			err := validateJSON(t, manifest)
			assert.Error(t, err,
				"resource with description as %s must fail schema validation", tc.name)
		})
	}
}

// ---------------------------------------------------------------------------
// Edge cases: multiple resources with mixed validity, ordering, etc.
// ---------------------------------------------------------------------------

func TestSchemaResource_SecondResourceInvalid_Fails(t *testing.T) {
	// If the first resource is valid but the second is missing required fields,
	// validation must still fail. This catches validators that only check the first item.
	manifest := minimalManifestWithResources(`[
		{
			"uri": "file:///data/good",
			"name": "good-resource",
			"entrypoint": "./resources/good.sh"
		},
		{
			"description": "bad resource — missing uri, name, entrypoint"
		}
	]`)
	err := validateJSON(t, manifest)
	assert.Error(t, err,
		"manifest with second invalid resource must fail validation")
}

func TestSchemaResource_MixedValidAndInvalidItems_Fails(t *testing.T) {
	// Valid resources cannot mask an invalid one in the middle.
	manifest := minimalManifestWithResources(`[
		{
			"uri": "file:///data/a",
			"name": "a",
			"entrypoint": "./resources/a.sh"
		},
		{
			"uri": "file:///data/b",
			"name": "b"
		},
		{
			"uri": "file:///data/c",
			"name": "c",
			"entrypoint": "./resources/c.sh"
		}
	]`)
	err := validateJSON(t, manifest)
	assert.Error(t, err,
		"manifest with one invalid resource among valid ones must fail validation")
}

func TestSchemaResource_EachRequiredFieldMissing_Individually(t *testing.T) {
	// Table-driven: remove one required field at a time. Each must fail independently.
	// This catches schemas that only enforce some required fields.
	tests := []struct {
		name      string
		resource  string
		wantField string
	}{
		{
			name: "missing uri only",
			resource: `{
				"name": "no-uri",
				"entrypoint": "./resources/test.sh"
			}`,
			wantField: "uri",
		},
		{
			name: "missing name only",
			resource: `{
				"uri": "file:///data/item",
				"entrypoint": "./resources/test.sh"
			}`,
			wantField: "name",
		},
		{
			name: "missing entrypoint only",
			resource: `{
				"uri": "file:///data/item",
				"name": "no-ep"
			}`,
			wantField: "entrypoint",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			manifest := minimalManifestWithResources(`[` + tc.resource + `]`)
			err := validateJSON(t, manifest)
			require.Error(t, err,
				"resource with %s must fail schema validation", tc.name)
			assert.Contains(t, err.Error(), tc.wantField,
				"error must reference the missing %q field", tc.wantField)
		})
	}
}

// resourceKeys returns the keys of a map for diagnostic messages.
func resourceKeys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
