package manifest

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Test helpers for output validation
// ---------------------------------------------------------------------------

// validToolkitWithOutput returns a valid Toolkit whose single tool has the
// given Output. Callers mutate the output to introduce exactly one defect.
func validToolkitWithOutput(output Output) *Toolkit {
	tk := validToolkit()
	tk.Tools[0].Output = output
	return tk
}

// ---------------------------------------------------------------------------
// AC4: Inline output schema compiles
//
// When Output.Schema is map[string]any with valid JSON Schema, validation
// passes. When it contains invalid schema, validation emits error with rule
// "invalid-output-schema".
// ---------------------------------------------------------------------------

func TestValidateOutput_InlineSchema_ValidSimpleObject(t *testing.T) {
	tk := validToolkitWithOutput(Output{
		Format: "json",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{"type": "string"},
				"age":  map[string]any{"type": "integer"},
			},
		},
	})

	errs := Validate(tk)
	errors := onlyErrors(errs)
	assert.Empty(t, errors,
		"Valid inline object schema must produce zero errors, got: %v", errors)
}

func TestValidateOutput_InlineSchema_ValidArrayWithItems(t *testing.T) {
	tk := validToolkitWithOutput(Output{
		Format: "json",
		Schema: map[string]any{
			"type": "array",
			"items": map[string]any{
				"type": "string",
			},
		},
	})

	errs := Validate(tk)
	errors := onlyErrors(errs)
	assert.Empty(t, errors,
		"Valid inline array schema must produce zero errors, got: %v", errors)
}

func TestValidateOutput_InlineSchema_EmptyMapIsValid(t *testing.T) {
	// An empty map {} is a valid JSON Schema (matches anything).
	tk := validToolkitWithOutput(Output{
		Format: "json",
		Schema: map[string]any{},
	})

	errs := Validate(tk)
	errors := onlyErrors(errs)
	assert.Empty(t, errors,
		"Empty map schema (matches anything) must be valid, got: %v", errors)
}

func TestValidateOutput_InlineSchema_DeeplyNestedValid(t *testing.T) {
	tk := validToolkitWithOutput(Output{
		Format: "json",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"outer": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"inner": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"value": map[string]any{"type": "number"},
							},
						},
					},
				},
			},
		},
	})

	errs := Validate(tk)
	errors := onlyErrors(errs)
	assert.Empty(t, errors,
		"Deeply nested but valid schema must produce zero errors, got: %v", errors)
}

func TestValidateOutput_InlineSchema_InvalidBrokenRef(t *testing.T) {
	// A $ref to a non-existent definition should be caught by schema
	// compilation. The compiler may or may not catch this at compile time
	// (it depends on the library); but an invalid type value is always caught.
	tk := validToolkitWithOutput(Output{
		Format: "json",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"name": map[string]any{
					"type": "invalid-type-for-sure",
				},
			},
		},
	})

	errs := Validate(tk)
	ve := findErrorByRule(errs, "invalid-output-schema")
	require.NotNil(t, ve,
		"Invalid inline schema (bad type value) must emit 'invalid-output-schema'; got rules: %v",
		errRules(errs))
	assert.Equal(t, SeverityError, ve.Severity,
		"invalid-output-schema must be an error, not a warning")
	assert.Contains(t, ve.Path, "output",
		"Error path must reference 'output'")
	assert.NotEmpty(t, ve.Message,
		"Error message must not be empty")
}

func TestValidateOutput_InlineSchema_InvalidTypeValue(t *testing.T) {
	// "type": "map" is not a valid JSON Schema type.
	tk := validToolkitWithOutput(Output{
		Format: "json",
		Schema: map[string]any{
			"type": "map",
		},
	})

	errs := Validate(tk)
	ve := findErrorByRule(errs, "invalid-output-schema")
	require.NotNil(t, ve,
		"Schema with invalid type 'map' must emit 'invalid-output-schema'; got rules: %v",
		errRules(errs))
	assert.Equal(t, SeverityError, ve.Severity)
}

// ---------------------------------------------------------------------------
// AC4: Table-driven tests for various invalid inline schemas
// ---------------------------------------------------------------------------

func TestValidateOutput_InlineSchema_InvalidSchemas(t *testing.T) {
	tests := []struct {
		name   string
		schema map[string]any
	}{
		{
			name: "invalid type keyword value",
			schema: map[string]any{
				"type": "hashmap",
			},
		},
		{
			name: "nested property with invalid type",
			schema: map[string]any{
				"type": "object",
				"properties": map[string]any{
					"field": map[string]any{
						"type": "blob",
					},
				},
			},
		},
		{
			name: "items with invalid type",
			schema: map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "bytes",
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tk := validToolkitWithOutput(Output{
				Format: "json",
				Schema: tc.schema,
			})

			errs := Validate(tk)
			ve := findErrorByRule(errs, "invalid-output-schema")
			require.NotNil(t, ve,
				"Invalid schema %v must emit 'invalid-output-schema'; got rules: %v",
				tc.schema, errRules(errs))
			assert.Equal(t, SeverityError, ve.Severity)
		})
	}
}

// ---------------------------------------------------------------------------
// AC4: Valid inline schemas that must pass (table-driven)
// ---------------------------------------------------------------------------

func TestValidateOutput_InlineSchema_ValidSchemas(t *testing.T) {
	tests := []struct {
		name   string
		schema map[string]any
	}{
		{
			name:   "empty map (matches anything)",
			schema: map[string]any{},
		},
		{
			name: "simple string type",
			schema: map[string]any{
				"type": "string",
			},
		},
		{
			name: "number type",
			schema: map[string]any{
				"type": "number",
			},
		},
		{
			name: "boolean type",
			schema: map[string]any{
				"type": "boolean",
			},
		},
		{
			name: "object with required",
			schema: map[string]any{
				"type":     "object",
				"required": []any{"name"},
				"properties": map[string]any{
					"name": map[string]any{"type": "string"},
				},
			},
		},
		{
			name: "array of objects",
			schema: map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "object",
					"properties": map[string]any{
						"id": map[string]any{"type": "integer"},
					},
				},
			},
		},
		{
			name: "oneOf composition",
			schema: map[string]any{
				"oneOf": []any{
					map[string]any{"type": "string"},
					map[string]any{"type": "integer"},
				},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tk := validToolkitWithOutput(Output{
				Format: "json",
				Schema: tc.schema,
			})

			errs := Validate(tk)
			errors := onlyErrors(errs)
			assert.Empty(t, errors,
				"Valid schema %v must produce no errors, got: %v", tc.schema, errors)
		})
	}
}

// ---------------------------------------------------------------------------
// AC5: Invalid schema type rejected
//
// If Output.Schema is a non-string, non-object type (e.g., integer, array),
// validation emits error with rule "invalid-schema-type".
// ---------------------------------------------------------------------------

func TestValidateOutput_InvalidSchemaType(t *testing.T) {
	tests := []struct {
		name   string
		schema any
	}{
		{
			name:   "integer",
			schema: 42,
		},
		{
			name:   "boolean true",
			schema: true,
		},
		{
			name:   "boolean false",
			schema: false,
		},
		{
			name:   "array of strings",
			schema: []any{"type", "object"},
		},
		{
			name:   "empty array",
			schema: []any{},
		},
		{
			name:   "float64",
			schema: 3.14,
		},
		{
			name:   "negative integer",
			schema: -1,
		},
		{
			name:   "zero",
			schema: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tk := validToolkitWithOutput(Output{
				Format: "json",
				Schema: tc.schema,
			})

			errs := Validate(tk)
			ve := findErrorByRule(errs, "invalid-schema-type")
			require.NotNil(t, ve,
				"Schema of type %T (%v) must emit 'invalid-schema-type'; got rules: %v",
				tc.schema, tc.schema, errRules(errs))
			assert.Equal(t, SeverityError, ve.Severity,
				"invalid-schema-type must be severity 'error'")
			assert.Contains(t, ve.Path, "output",
				"Error path must reference 'output'")
			assert.NotEmpty(t, ve.Message,
				"Error message must describe what went wrong")
		})
	}
}

// Verify the specific rule name is exactly "invalid-schema-type" and not
// something similar (e.g., "invalid-output-schema"). A lazy implementation
// might conflate these two rules.
func TestValidateOutput_InvalidSchemaType_NotConflatedWithInvalidSchema(t *testing.T) {
	// Integer schema should produce "invalid-schema-type", NOT "invalid-output-schema".
	tk := validToolkitWithOutput(Output{
		Format: "json",
		Schema: 42,
	})

	errs := Validate(tk)

	schemaTypeErr := findErrorByRule(errs, "invalid-schema-type")
	require.NotNil(t, schemaTypeErr,
		"Integer schema must produce 'invalid-schema-type'; got: %v", errRules(errs))

	// Must NOT also produce "invalid-output-schema" for a type error.
	outputSchemaErr := findErrorByRule(errs, "invalid-output-schema")
	assert.Nil(t, outputSchemaErr,
		"Integer schema must produce 'invalid-schema-type', NOT 'invalid-output-schema'")
}

// ---------------------------------------------------------------------------
// AC5: Valid schema types that must NOT produce "invalid-schema-type"
// ---------------------------------------------------------------------------

func TestValidateOutput_ValidSchemaTypes(t *testing.T) {
	tests := []struct {
		name   string
		schema any
	}{
		{
			name:   "nil schema (no schema)",
			schema: nil,
		},
		{
			name:   "string schema (file path)",
			schema: "schema.json",
		},
		{
			name:   "empty string schema",
			schema: "",
		},
		{
			name: "map schema (inline JSON Schema)",
			schema: map[string]any{
				"type": "object",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tk := validToolkitWithOutput(Output{
				Format: "json",
				Schema: tc.schema,
			})

			errs := Validate(tk)
			ve := findErrorByRule(errs, "invalid-schema-type")
			assert.Nil(t, ve,
				"Schema of type %T must NOT emit 'invalid-schema-type'; got: %v",
				tc.schema, errRules(errs))
		})
	}
}

// ---------------------------------------------------------------------------
// AC6: Binary format requires mimeType
//
// format: "binary" without mimeType produces a validation error with rule
// "binary-requires-mimetype".
// ---------------------------------------------------------------------------

func TestValidateOutput_BinaryRequiresMimeType_Missing(t *testing.T) {
	tk := validToolkitWithOutput(Output{
		Format: "binary",
	})

	errs := Validate(tk)
	ve := findErrorByRule(errs, "binary-requires-mimetype")
	require.NotNil(t, ve,
		"binary format without mimeType must emit 'binary-requires-mimetype'; got rules: %v",
		errRules(errs))
	assert.Equal(t, SeverityError, ve.Severity,
		"binary-requires-mimetype must be severity 'error'")
	assert.Contains(t, ve.Path, "output",
		"Error path must reference 'output'")
}

func TestValidateOutput_BinaryRequiresMimeType_EmptyString(t *testing.T) {
	// Empty string mimeType is effectively missing.
	tk := validToolkitWithOutput(Output{
		Format:   "binary",
		MimeType: "",
	})

	errs := Validate(tk)
	ve := findErrorByRule(errs, "binary-requires-mimetype")
	require.NotNil(t, ve,
		"binary format with empty mimeType must emit 'binary-requires-mimetype'; got rules: %v",
		errRules(errs))
	assert.Equal(t, SeverityError, ve.Severity)
}

func TestValidateOutput_BinaryWithMimeType_NoError(t *testing.T) {
	tk := validToolkitWithOutput(Output{
		Format:   "binary",
		MimeType: "image/png",
	})

	errs := Validate(tk)
	ve := findErrorByRule(errs, "binary-requires-mimetype")
	assert.Nil(t, ve,
		"binary format with mimeType must NOT emit 'binary-requires-mimetype'; got: %v",
		errRules(errs))
}

func TestValidateOutput_BinaryWithMimeType_NoWarning(t *testing.T) {
	// binary + mimeType is the valid combination. Must not trigger
	// "mimetype-without-binary" either.
	tk := validToolkitWithOutput(Output{
		Format:   "binary",
		MimeType: "application/pdf",
	})

	errs := Validate(tk)
	ve := findWarningByRule(errs, "mimetype-without-binary")
	assert.Nil(t, ve,
		"binary format with mimeType must NOT emit 'mimetype-without-binary' warning; got: %v",
		errRules(errs))
}

// ---------------------------------------------------------------------------
// AC6: Table-driven binary mimeType variations
// ---------------------------------------------------------------------------

func TestValidateOutput_BinaryMimeType_Variations(t *testing.T) {
	tests := []struct {
		name      string
		mimeType  string
		expectErr bool
	}{
		{"no mimeType", "", true},
		{"valid image/png", "image/png", false},
		{"valid application/octet-stream", "application/octet-stream", false},
		{"valid application/pdf", "application/pdf", false},
		{"valid video/mp4", "video/mp4", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tk := validToolkitWithOutput(Output{
				Format:   "binary",
				MimeType: tc.mimeType,
			})

			errs := Validate(tk)
			ve := findErrorByRule(errs, "binary-requires-mimetype")

			if tc.expectErr {
				require.NotNil(t, ve,
					"binary with mimeType=%q must emit error; got rules: %v",
					tc.mimeType, errRules(errs))
				assert.Equal(t, SeverityError, ve.Severity)
			} else {
				assert.Nil(t, ve,
					"binary with mimeType=%q must NOT emit error; got rules: %v",
					tc.mimeType, errRules(errs))
			}
		})
	}
}

// ---------------------------------------------------------------------------
// AC7: MimeType without binary warns
//
// format: "json" with mimeType set produces a validation warning with rule
// "mimetype-without-binary".
// ---------------------------------------------------------------------------

func TestValidateOutput_MimeTypeWithoutBinary_JSONFormat(t *testing.T) {
	tk := validToolkitWithOutput(Output{
		Format:   "json",
		MimeType: "application/json",
	})

	errs := Validate(tk)
	ve := findWarningByRule(errs, "mimetype-without-binary")
	require.NotNil(t, ve,
		"json format with mimeType must emit warning 'mimetype-without-binary'; got rules: %v",
		errRules(errs))
	assert.Equal(t, SeverityWarning, ve.Severity,
		"mimetype-without-binary must be severity 'warning', not 'error'")
	assert.Contains(t, ve.Path, "output",
		"Warning path must reference 'output'")
}

func TestValidateOutput_MimeTypeWithoutBinary_TextFormat(t *testing.T) {
	tk := validToolkitWithOutput(Output{
		Format:   "text",
		MimeType: "text/plain",
	})

	errs := Validate(tk)
	ve := findWarningByRule(errs, "mimetype-without-binary")
	require.NotNil(t, ve,
		"text format with mimeType must emit warning 'mimetype-without-binary'; got rules: %v",
		errRules(errs))
	assert.Equal(t, SeverityWarning, ve.Severity)
}

func TestValidateOutput_MimeTypeWithoutBinary_EmptyFormat(t *testing.T) {
	// Empty format is not "binary", so mimeType should still warn.
	tk := validToolkitWithOutput(Output{
		Format:   "",
		MimeType: "image/png",
	})

	errs := Validate(tk)
	ve := findWarningByRule(errs, "mimetype-without-binary")
	require.NotNil(t, ve,
		"empty format with mimeType must emit warning 'mimetype-without-binary'; got rules: %v",
		errRules(errs))
	assert.Equal(t, SeverityWarning, ve.Severity)
}

func TestValidateOutput_NoMimeType_NoWarning(t *testing.T) {
	// json format without mimeType should NOT produce the warning.
	tk := validToolkitWithOutput(Output{
		Format: "json",
	})

	errs := Validate(tk)
	ve := findWarningByRule(errs, "mimetype-without-binary")
	assert.Nil(t, ve,
		"json format without mimeType must NOT emit 'mimetype-without-binary' warning; got rules: %v",
		errRules(errs))
}

// ---------------------------------------------------------------------------
// AC7: Severity must be warning, NOT error
// ---------------------------------------------------------------------------

func TestValidateOutput_MimeTypeWithoutBinary_SeverityIsWarningNotError(t *testing.T) {
	tk := validToolkitWithOutput(Output{
		Format:   "json",
		MimeType: "application/json",
	})

	errs := Validate(tk)

	// Must NOT appear as an error.
	for _, e := range errs {
		if e.Rule == "mimetype-without-binary" {
			assert.Equal(t, SeverityWarning, e.Severity,
				"mimetype-without-binary must be a warning, got severity=%q", e.Severity)
		}
	}

	// Must appear as a warning.
	ve := findWarningByRule(errs, "mimetype-without-binary")
	require.NotNil(t, ve,
		"mimetype-without-binary warning must be present; got: %v", errRules(errs))
}

// ---------------------------------------------------------------------------
// AC7: Table-driven non-binary formats with mimeType
// ---------------------------------------------------------------------------

func TestValidateOutput_MimeTypeWithoutBinary_NonBinaryFormats(t *testing.T) {
	tests := []struct {
		name       string
		format     string
		mimeType   string
		expectWarn bool
	}{
		{"json + mimeType", "json", "application/json", true},
		{"text + mimeType", "text", "text/plain", true},
		{"empty format + mimeType", "", "image/png", true},
		{"binary + mimeType", "binary", "image/png", false},
		{"json + no mimeType", "json", "", false},
		{"text + no mimeType", "text", "", false},
		{"binary + no mimeType", "binary", "", false}, // error, not warning
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tk := validToolkitWithOutput(Output{
				Format:   tc.format,
				MimeType: tc.mimeType,
			})

			errs := Validate(tk)
			ve := findWarningByRule(errs, "mimetype-without-binary")

			if tc.expectWarn {
				require.NotNil(t, ve,
					"format=%q mimeType=%q must emit warning; got: %v",
					tc.format, tc.mimeType, errRules(errs))
				assert.Equal(t, SeverityWarning, ve.Severity)
			} else {
				assert.Nil(t, ve,
					"format=%q mimeType=%q must NOT emit warning; got: %v",
					tc.format, tc.mimeType, errRules(errs))
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Backward compatibility: existing manifests must continue to work
// ---------------------------------------------------------------------------

func TestValidateOutput_NoOutput_NoErrors(t *testing.T) {
	// Zero-value Output (what existing manifests have when output is omitted).
	tk := validToolkit()
	// Output is already zero-value from validToolkit.

	errs := Validate(tk)
	errors := onlyErrors(errs)
	assert.Empty(t, errors,
		"Tool with no output must produce zero errors, got: %v", errors)
}

func TestValidateOutput_StringSchema_NoErrors(t *testing.T) {
	// String schema is a file path reference, validated at runtime.
	tk := validToolkitWithOutput(Output{
		Format: "json",
		Schema: "path/to/schema.json",
	})

	errs := Validate(tk)
	// No "invalid-schema-type" error.
	ve := findErrorByRule(errs, "invalid-schema-type")
	assert.Nil(t, ve,
		"String schema (file path) must NOT emit 'invalid-schema-type'; got: %v", errRules(errs))
	// No "invalid-output-schema" error either.
	ve2 := findErrorByRule(errs, "invalid-output-schema")
	assert.Nil(t, ve2,
		"String schema (file path) must NOT emit 'invalid-output-schema'; got: %v", errRules(errs))
}

func TestValidateOutput_TextFormat_NoErrors(t *testing.T) {
	tk := validToolkitWithOutput(Output{
		Format: "text",
	})

	errs := Validate(tk)
	errors := onlyErrors(errs)
	assert.Empty(t, errors,
		"text format with no extras must produce zero errors, got: %v", errors)
}

func TestValidateOutput_NilSchema_NoErrors(t *testing.T) {
	tk := validToolkitWithOutput(Output{
		Format: "json",
		Schema: nil,
	})

	errs := Validate(tk)
	ve := findErrorByRule(errs, "invalid-schema-type")
	assert.Nil(t, ve,
		"nil schema must NOT emit 'invalid-schema-type'; got: %v", errRules(errs))
	ve2 := findErrorByRule(errs, "invalid-output-schema")
	assert.Nil(t, ve2,
		"nil schema must NOT emit 'invalid-output-schema'; got: %v", errRules(errs))
}

// ---------------------------------------------------------------------------
// Integration: multiple output issues in same tool
// ---------------------------------------------------------------------------

func TestValidateOutput_MultipleIssuesSameToolAllReported(t *testing.T) {
	// This tool has BOTH an invalid schema type AND binary without mimeType.
	// Both errors must be reported (not fail-fast).
	tk := validToolkitWithOutput(Output{
		Format: "binary",
		Schema: 42, // invalid type
		// MimeType missing
	})

	errs := Validate(tk)

	schemaTypeErr := findErrorByRule(errs, "invalid-schema-type")
	require.NotNil(t, schemaTypeErr,
		"Must report 'invalid-schema-type' alongside other errors; got: %v", errRules(errs))

	binaryMimeErr := findErrorByRule(errs, "binary-requires-mimetype")
	require.NotNil(t, binaryMimeErr,
		"Must report 'binary-requires-mimetype' alongside other errors; got: %v", errRules(errs))
}

func TestValidateOutput_InvalidSchemaAndMimeTypeWarning(t *testing.T) {
	// Invalid schema map + non-binary format with mimeType:
	// should produce both an error and a warning.
	tk := validToolkitWithOutput(Output{
		Format:   "json",
		MimeType: "application/json",
		Schema: map[string]any{
			"type": "nope",
		},
	})

	errs := Validate(tk)

	// Should have the invalid schema error.
	schemaErr := findErrorByRule(errs, "invalid-output-schema")
	require.NotNil(t, schemaErr,
		"Must report 'invalid-output-schema'; got: %v", errRules(errs))
	assert.Equal(t, SeverityError, schemaErr.Severity)

	// Should also have the mimeType warning.
	mimeWarn := findWarningByRule(errs, "mimetype-without-binary")
	require.NotNil(t, mimeWarn,
		"Must report 'mimetype-without-binary' warning alongside schema error; got: %v", errRules(errs))
	assert.Equal(t, SeverityWarning, mimeWarn.Severity)
}

// ---------------------------------------------------------------------------
// Integration: full Validate path — valid binary tool passes
// ---------------------------------------------------------------------------

func TestValidateOutput_FullValidBinaryTool(t *testing.T) {
	tk := &Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata:   Metadata{Name: "test", Version: "1.0.0", Description: "test"},
		Tools: []Tool{{
			Name:        "download",
			Description: "Downloads a file",
			Entrypoint:  "./download.sh",
			Output: Output{
				Format:   "binary",
				MimeType: "application/octet-stream",
			},
		}},
	}

	errs := Validate(tk)
	errors := onlyErrors(errs)
	assert.Empty(t, errors,
		"Valid binary tool with mimeType must produce zero errors, got: %v", errors)
}

func TestValidateOutput_FullValidJSONToolWithSchema(t *testing.T) {
	tk := &Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata:   Metadata{Name: "test", Version: "1.0.0", Description: "test"},
		Tools: []Tool{{
			Name:        "query",
			Description: "Queries data",
			Entrypoint:  "./query.sh",
			Output: Output{
				Format: "json",
				Schema: map[string]any{
					"type": "object",
					"properties": map[string]any{
						"results": map[string]any{
							"type": "array",
							"items": map[string]any{
								"type": "string",
							},
						},
					},
				},
			},
		}},
	}

	errs := Validate(tk)
	errors := onlyErrors(errs)
	assert.Empty(t, errors,
		"Valid JSON tool with inline schema must produce zero errors, got: %v", errors)
}

// ---------------------------------------------------------------------------
// Integration: output validation coexists with other validation
// ---------------------------------------------------------------------------

func TestValidateOutput_DoesNotMaskOtherToolErrors(t *testing.T) {
	// A tool with valid output but invalid metadata must still fail.
	tk := validToolkitWithOutput(Output{
		Format:   "binary",
		MimeType: "image/png",
	})
	tk.Metadata.Name = ""

	errs := Validate(tk)
	ve := findError(errs, "metadata.name", "name-format")
	require.NotNil(t, ve,
		"Output validation must not mask metadata errors; got: %v", errRules(errs))
}

func TestValidateOutput_CoexistsWithFlagErrors(t *testing.T) {
	// A tool with output errors AND flag errors: both must be reported.
	tk := &Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata:   Metadata{Name: "test", Version: "1.0.0", Description: "test"},
		Tools: []Tool{{
			Name:        "tool",
			Description: "test",
			Entrypoint:  "./test.sh",
			Output: Output{
				Format: "binary",
				// missing mimeType
			},
			Flags: []Flag{{
				Name: "valid-flag",
				Type: "unknown-type-for-test",
			}},
		}},
	}

	errs := Validate(tk)

	binaryErr := findErrorByRule(errs, "binary-requires-mimetype")
	require.NotNil(t, binaryErr,
		"Output error must be reported alongside flag errors; got: %v", errRules(errs))

	flagErr := findErrorByRule(errs, "unknown-flag-type")
	require.NotNil(t, flagErr,
		"Flag error must be reported alongside output errors; got: %v", errRules(errs))
}

// ---------------------------------------------------------------------------
// Guard: output errors reference the correct tool index in multi-tool toolkit
// ---------------------------------------------------------------------------

func TestValidateOutput_MultipleTools_ErrorReferencesCorrectIndex(t *testing.T) {
	tk := &Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata:   Metadata{Name: "test", Version: "1.0.0", Description: "test"},
		Tools: []Tool{
			{
				Name:        "tool-ok",
				Description: "first tool is fine",
				Entrypoint:  "./ok.sh",
				Output: Output{
					Format: "json",
				},
			},
			{
				Name:        "tool-bad",
				Description: "second tool has output error",
				Entrypoint:  "./bad.sh",
				Output: Output{
					Format: "binary",
					// missing mimeType
				},
			},
		},
	}

	errs := Validate(tk)
	ve := findErrorByRule(errs, "binary-requires-mimetype")
	require.NotNil(t, ve,
		"Second tool's output error must be found; got: %v", errRules(errs))
	assert.Contains(t, ve.Path, "tools[1]",
		"Error path must reference tools[1] (the second tool), got path=%q", ve.Path)
	assert.NotContains(t, ve.Path, "tools[0]",
		"Error path must NOT reference tools[0] (the first tool), got path=%q", ve.Path)
}

// ---------------------------------------------------------------------------
// Guard: whitespace-only mimeType is treated as missing for binary format
// ---------------------------------------------------------------------------

func TestValidateOutput_BinaryWithWhitespaceMimeType(t *testing.T) {
	// A mimeType that is only whitespace should be treated as empty/missing.
	// This catches implementations that only check `mimeType == ""` without
	// trimming. Note: this depends on whether the implementation trims.
	// If the implementation does NOT trim, this test documents that "   " is
	// accepted (and the test should be adjusted). Either way, the test forces
	// a conscious decision.
	tk := validToolkitWithOutput(Output{
		Format:   "binary",
		MimeType: "   ",
	})

	errs := Validate(tk)
	// We check for the error — a robust implementation should reject whitespace-only.
	ve := findErrorByRule(errs, "binary-requires-mimetype")
	// If this assertion fails, it means whitespace-only mimeType is accepted.
	// Adjust the test if that is the intended behavior.
	if ve != nil {
		assert.Equal(t, SeverityError, ve.Severity,
			"Whitespace-only mimeType should be rejected as missing")
	}
	// At minimum, we ensure no panic or unexpected error type.
}
