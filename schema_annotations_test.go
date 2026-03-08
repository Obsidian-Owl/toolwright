package toolwright_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// AC-4: JSON Schema accepts manifests with annotations
// ---------------------------------------------------------------------------

// minimalManifestWithTool returns a full valid manifest JSON string with
// the given tool JSON fragment embedded. This avoids duplicate boilerplate
// while ensuring every test exercises real schema validation with all
// required top-level fields present.
func minimalManifestWithTool(toolJSON string) string {
	return `{
		"apiVersion": "toolwright/v1",
		"kind": "Toolkit",
		"metadata": {
			"name": "annot-test",
			"version": "1.0.0",
			"description": "Annotation schema tests"
		},
		"tools": [` + toolJSON + `]
	}`
}

// ---------------------------------------------------------------------------
// Positive: manifests with various annotation shapes must pass
// ---------------------------------------------------------------------------

func TestSchemaAnnotations_FullAnnotationsPasses(t *testing.T) {
	// All 4 boolean fields + title, each with explicit values.
	manifest := minimalManifestWithTool(`{
		"name": "full-annot",
		"description": "Tool with all annotations",
		"entrypoint": "./tool.sh",
		"annotations": {
			"readOnly": true,
			"destructive": false,
			"idempotent": true,
			"openWorld": false,
			"title": "Full Annotation Tool"
		}
	}`)
	err := validateJSON(t, manifest)
	assert.NoError(t, err,
		"manifest with all 5 annotation fields must pass schema validation")
}

func TestSchemaAnnotations_PartialAnnotations_ReadOnlyOnly(t *testing.T) {
	manifest := minimalManifestWithTool(`{
		"name": "partial-annot",
		"description": "Tool with only readOnly",
		"entrypoint": "./tool.sh",
		"annotations": {
			"readOnly": true
		}
	}`)
	err := validateJSON(t, manifest)
	assert.NoError(t, err,
		"manifest with only readOnly annotation must pass schema validation")
}

func TestSchemaAnnotations_NoAnnotationsField(t *testing.T) {
	// Tools without annotations at all must still pass — annotations are optional.
	manifest := minimalManifestWithTool(`{
		"name": "no-annot",
		"description": "Tool without annotations",
		"entrypoint": "./tool.sh"
	}`)
	err := validateJSON(t, manifest)
	assert.NoError(t, err,
		"manifest without annotations field must pass schema validation")
}

func TestSchemaAnnotations_EmptyAnnotationsObject(t *testing.T) {
	manifest := minimalManifestWithTool(`{
		"name": "empty-annot",
		"description": "Tool with empty annotations",
		"entrypoint": "./tool.sh",
		"annotations": {}
	}`)
	err := validateJSON(t, manifest)
	assert.NoError(t, err,
		"manifest with empty annotations object must pass schema validation")
}

func TestSchemaAnnotations_OnlyTitle(t *testing.T) {
	manifest := minimalManifestWithTool(`{
		"name": "title-only",
		"description": "Tool with only title",
		"entrypoint": "./tool.sh",
		"annotations": {
			"title": "Human-Readable Name"
		}
	}`)
	err := validateJSON(t, manifest)
	assert.NoError(t, err,
		"manifest with only title annotation must pass schema validation")
}

func TestSchemaAnnotations_FalseValuesPass(t *testing.T) {
	// Explicitly false booleans must be accepted — this catches a schema
	// that accidentally uses "const": true or "enum": [true].
	manifest := minimalManifestWithTool(`{
		"name": "false-annot",
		"description": "Tool with all false annotations",
		"entrypoint": "./tool.sh",
		"annotations": {
			"readOnly": false,
			"destructive": false,
			"idempotent": false,
			"openWorld": false
		}
	}`)
	err := validateJSON(t, manifest)
	assert.NoError(t, err,
		"manifest with all false annotation booleans must pass schema validation")
}

// ---------------------------------------------------------------------------
// Positive: each boolean field individually with true and false
// ---------------------------------------------------------------------------

func TestSchemaAnnotations_EachBoolField_TrueAndFalse(t *testing.T) {
	// This catches a schema that defines readOnly but not destructive, etc.
	fields := []string{"readOnly", "destructive", "idempotent", "openWorld"}
	for _, field := range fields {
		for _, val := range []string{"true", "false"} {
			t.Run(field+"_"+val, func(t *testing.T) {
				manifest := minimalManifestWithTool(`{
					"name": "bool-test",
					"description": "Bool field test",
					"entrypoint": "./tool.sh",
					"annotations": {
						"` + field + `": ` + val + `
					}
				}`)
				err := validateJSON(t, manifest)
				assert.NoError(t, err,
					"annotation field %q with value %s must pass schema validation",
					field, val)
			})
		}
	}
}

// ---------------------------------------------------------------------------
// Positive: title with various valid string content
// ---------------------------------------------------------------------------

func TestSchemaAnnotations_TitleVariousStrings(t *testing.T) {
	tests := []struct {
		name  string
		title string
	}{
		{name: "simple", title: "My Tool"},
		{name: "empty string", title: ""},
		{name: "special chars", title: "Tool: List (read-only) — v2"},
		{name: "unicode", title: "Werkzeug \u00fc\u00e4\u00f6"},
		{name: "long title", title: "This is a very long title that might be used to describe a tool in great detail for accessibility purposes"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Use json.Marshal to safely escape the title string.
			titleBytes, err := json.Marshal(tc.title)
			require.NoError(t, err)
			manifest := minimalManifestWithTool(`{
				"name": "title-test",
				"description": "Title test",
				"entrypoint": "./tool.sh",
				"annotations": {
					"title": ` + string(titleBytes) + `
				}
			}`)
			err = validateJSON(t, manifest)
			assert.NoError(t, err,
				"title %q must pass schema validation", tc.title)
		})
	}
}

// ---------------------------------------------------------------------------
// Positive: annotations alongside other tool properties
// ---------------------------------------------------------------------------

func TestSchemaAnnotations_WithOtherToolProperties(t *testing.T) {
	// Annotations must not interfere with args, flags, output, auth, exit_codes.
	manifest := minimalManifestWithTool(`{
		"name": "full-tool",
		"description": "Tool with everything",
		"entrypoint": "./run.sh",
		"args": [{
			"name": "input",
			"type": "string",
			"required": true,
			"description": "Input value"
		}],
		"flags": [{
			"name": "verbose",
			"type": "bool",
			"required": false,
			"default": false,
			"description": "Enable verbose"
		}],
		"output": {
			"format": "json"
		},
		"auth": {
			"type": "token",
			"token_env": "MY_TOKEN"
		},
		"exit_codes": {
			"0": "success",
			"1": "failure"
		},
		"annotations": {
			"readOnly": true,
			"destructive": false,
			"idempotent": true,
			"openWorld": false,
			"title": "Full Tool"
		}
	}`)
	err := validateJSON(t, manifest)
	assert.NoError(t, err,
		"manifest with annotations alongside args, flags, output, auth, exit_codes must pass")
}

// ---------------------------------------------------------------------------
// Positive: multiple tools with mixed annotation states
// ---------------------------------------------------------------------------

func TestSchemaAnnotations_MultipleToolsMixed(t *testing.T) {
	manifest := `{
		"apiVersion": "toolwright/v1",
		"kind": "Toolkit",
		"metadata": {
			"name": "multi-annot",
			"version": "1.0.0",
			"description": "Multiple tools with mixed annotations"
		},
		"tools": [
			{
				"name": "annotated",
				"description": "Has full annotations",
				"entrypoint": "./a.sh",
				"annotations": {
					"readOnly": true,
					"destructive": false,
					"title": "Annotated"
				}
			},
			{
				"name": "plain",
				"description": "No annotations",
				"entrypoint": "./b.sh"
			},
			{
				"name": "empty-annot",
				"description": "Empty annotations",
				"entrypoint": "./c.sh",
				"annotations": {}
			}
		]
	}`
	err := validateJSON(t, manifest)
	assert.NoError(t, err,
		"multiple tools with mixed annotation states must all pass")
}

// ---------------------------------------------------------------------------
// Negative: wrong types must fail schema validation
// ---------------------------------------------------------------------------

func TestSchemaAnnotations_WrongBoolType_StringInsteadOfBool(t *testing.T) {
	// Each boolean field must reject non-boolean types.
	// This catches a schema that uses "type": "string" or omits type constraints.
	fields := []string{"readOnly", "destructive", "idempotent", "openWorld"}
	for _, field := range fields {
		t.Run(field, func(t *testing.T) {
			manifest := minimalManifestWithTool(`{
				"name": "wrong-type",
				"description": "Wrong type test",
				"entrypoint": "./tool.sh",
				"annotations": {
					"` + field + `": "yes"
				}
			}`)
			err := validateJSON(t, manifest)
			assert.Error(t, err,
				"annotation field %q with string value \"yes\" must fail schema validation", field)
		})
	}
}

func TestSchemaAnnotations_WrongBoolType_NumberInsteadOfBool(t *testing.T) {
	// Numbers are not booleans — catches a schema missing type constraint.
	fields := []string{"readOnly", "destructive", "idempotent", "openWorld"}
	for _, field := range fields {
		t.Run(field, func(t *testing.T) {
			manifest := minimalManifestWithTool(`{
				"name": "wrong-type",
				"description": "Wrong type test",
				"entrypoint": "./tool.sh",
				"annotations": {
					"` + field + `": 1
				}
			}`)
			err := validateJSON(t, manifest)
			assert.Error(t, err,
				"annotation field %q with number value 1 must fail schema validation", field)
		})
	}
}

func TestSchemaAnnotations_WrongBoolType_NullInsteadOfBool(t *testing.T) {
	// null is not a boolean — should fail.
	manifest := minimalManifestWithTool(`{
		"name": "null-bool",
		"description": "Null bool test",
		"entrypoint": "./tool.sh",
		"annotations": {
			"readOnly": null
		}
	}`)
	err := validateJSON(t, manifest)
	assert.Error(t, err,
		"annotation field readOnly with null must fail schema validation")
}

func TestSchemaAnnotations_WrongTitleType_Number(t *testing.T) {
	manifest := minimalManifestWithTool(`{
		"name": "wrong-title",
		"description": "Wrong title type",
		"entrypoint": "./tool.sh",
		"annotations": {
			"title": 42
		}
	}`)
	err := validateJSON(t, manifest)
	assert.Error(t, err,
		"annotation title with number value 42 must fail schema validation")
}

func TestSchemaAnnotations_WrongTitleType_Bool(t *testing.T) {
	manifest := minimalManifestWithTool(`{
		"name": "wrong-title",
		"description": "Wrong title type",
		"entrypoint": "./tool.sh",
		"annotations": {
			"title": true
		}
	}`)
	err := validateJSON(t, manifest)
	assert.Error(t, err,
		"annotation title with boolean value true must fail schema validation")
}

func TestSchemaAnnotations_WrongTitleType_Array(t *testing.T) {
	manifest := minimalManifestWithTool(`{
		"name": "wrong-title",
		"description": "Wrong title type",
		"entrypoint": "./tool.sh",
		"annotations": {
			"title": ["a", "b"]
		}
	}`)
	err := validateJSON(t, manifest)
	assert.Error(t, err,
		"annotation title with array value must fail schema validation")
}

func TestSchemaAnnotations_WrongAnnotationsType_NotObject(t *testing.T) {
	// annotations itself must be an object — string, array, number must fail.
	wrongTypes := []struct {
		name  string
		value string
	}{
		{name: "string", value: `"not-an-object"`},
		{name: "number", value: `42`},
		{name: "array", value: `[true, false]`},
		{name: "boolean", value: `true`},
	}
	for _, tc := range wrongTypes {
		t.Run(tc.name, func(t *testing.T) {
			manifest := minimalManifestWithTool(`{
				"name": "wrong-annot-type",
				"description": "Wrong annotations type",
				"entrypoint": "./tool.sh",
				"annotations": ` + tc.value + `
			}`)
			err := validateJSON(t, manifest)
			assert.Error(t, err,
				"annotations as %s must fail schema validation", tc.name)
		})
	}
}

// ---------------------------------------------------------------------------
// Schema structure: verify raw JSON has correct annotations definition
// ---------------------------------------------------------------------------

func TestSchemaAnnotations_RawSchemaContainsAnnotationsDefinition(t *testing.T) {
	data := loadSchemaBytes(t)

	var raw map[string]any
	err := json.Unmarshal(data, &raw)
	require.NoError(t, err, "schema must parse as valid JSON")

	// Navigate to tools.items.properties.annotations
	props, ok := raw["properties"].(map[string]any)
	require.True(t, ok, "schema must have a 'properties' object")

	tools, ok := props["tools"].(map[string]any)
	require.True(t, ok, "properties must contain 'tools'")

	items, ok := tools["items"].(map[string]any)
	require.True(t, ok, "tools must contain 'items'")

	itemProps, ok := items["properties"].(map[string]any)
	require.True(t, ok, "tools.items must contain 'properties'")

	annotations, ok := itemProps["annotations"].(map[string]any)
	require.True(t, ok, "tool properties must contain 'annotations'")

	// Verify annotations is typed as object.
	annotType, ok := annotations["type"].(string)
	require.True(t, ok, "annotations must have a 'type' field")
	assert.Equal(t, "object", annotType,
		"annotations type must be 'object'")

	// Verify annotations has a properties object with the 5 expected fields.
	annotProps, ok := annotations["properties"].(map[string]any)
	require.True(t, ok, "annotations must have a 'properties' object")

	expectedBoolFields := []string{"readOnly", "destructive", "idempotent", "openWorld"}
	for _, field := range expectedBoolFields {
		fieldDef, ok := annotProps[field].(map[string]any)
		require.True(t, ok, "annotations.properties must contain %q", field)

		fieldType, ok := fieldDef["type"].(string)
		require.True(t, ok, "annotations.properties.%s must have a 'type' field", field)
		assert.Equal(t, "boolean", fieldType,
			"annotations.properties.%s type must be 'boolean', got %q", field, fieldType)
	}

	// Verify title is defined as string.
	titleDef, ok := annotProps["title"].(map[string]any)
	require.True(t, ok, "annotations.properties must contain 'title'")

	titleType, ok := titleDef["type"].(string)
	require.True(t, ok, "annotations.properties.title must have a 'type' field")
	assert.Equal(t, "string", titleType,
		"annotations.properties.title type must be 'string', got %q", titleType)
}

func TestSchemaAnnotations_RawSchemaHasExactlyFiveAnnotationProperties(t *testing.T) {
	// Guard against extra or missing properties in the annotations definition.
	data := loadSchemaBytes(t)

	var raw map[string]any
	err := json.Unmarshal(data, &raw)
	require.NoError(t, err)

	// Navigate to annotations.properties
	annotProps := raw["properties"].(map[string]any)["tools"].(map[string]any)["items"].(map[string]any)["properties"].(map[string]any)["annotations"].(map[string]any)["properties"].(map[string]any)

	expectedFields := []string{"readOnly", "destructive", "idempotent", "openWorld", "title"}
	assert.Len(t, annotProps, len(expectedFields),
		"annotations must have exactly %d properties, got %d: %v",
		len(expectedFields), len(annotProps), annotKeys(annotProps))

	for _, field := range expectedFields {
		_, ok := annotProps[field]
		assert.True(t, ok, "annotations.properties must contain %q", field)
	}
}

// annotKeys returns the keys of a map for diagnostic messages.
func annotKeys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
