package schema

import (
	"strings"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Test fixtures — JSON Schemas as MapFS entries
// ---------------------------------------------------------------------------

// simpleSchema requires an object with "name" (string) and "count" (integer).
// "tags" (array of strings) is optional.
const simpleSchemaJSON = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "required": ["name", "count"],
  "properties": {
    "name": { "type": "string" },
    "count": { "type": "integer" },
    "tags": {
      "type": "array",
      "items": { "type": "string" }
    }
  }
}`

// nestedSchema requires an object with a nested "address" object that itself
// has required fields. This tests that validation drills into nested objects
// and reports paths correctly.
const nestedSchemaJSON = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "required": ["user"],
  "properties": {
    "user": {
      "type": "object",
      "required": ["email", "age"],
      "properties": {
        "email": { "type": "string" },
        "age": { "type": "integer" },
        "address": {
          "type": "object",
          "required": ["city"],
          "properties": {
            "city": { "type": "string" },
            "zip": { "type": "string" }
          }
        }
      }
    }
  }
}`

// strictTypeSchema has properties with various types for type-mismatch testing.
const strictTypeSchemaJSON = `{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "required": ["id", "active", "score"],
  "properties": {
    "id": { "type": "integer" },
    "active": { "type": "boolean" },
    "score": { "type": "number" }
  }
}`

// testFS builds a MapFS with all test schemas.
func testFS() fstest.MapFS {
	return fstest.MapFS{
		"schemas/simple.schema.json": &fstest.MapFile{
			Data: []byte(simpleSchemaJSON),
		},
		"schemas/nested.schema.json": &fstest.MapFile{
			Data: []byte(nestedSchemaJSON),
		},
		"schemas/strict.schema.json": &fstest.MapFile{
			Data: []byte(strictTypeSchemaJSON),
		},
	}
}

// ---------------------------------------------------------------------------
// AC-9: Valid JSON matching schema -> no error
// ---------------------------------------------------------------------------

func TestValidate_ValidJSON_NoError(t *testing.T) {
	v := NewValidator(testFS())

	tests := []struct {
		name       string
		schemaPath string
		data       string
	}{
		{
			name:       "all required fields present with correct types",
			schemaPath: "schemas/simple.schema.json",
			data:       `{"name": "alice", "count": 42}`,
		},
		{
			name:       "required fields plus optional field",
			schemaPath: "schemas/simple.schema.json",
			data:       `{"name": "bob", "count": 7, "tags": ["go", "rust"]}`,
		},
		{
			name:       "required fields with empty tags array",
			schemaPath: "schemas/simple.schema.json",
			data:       `{"name": "carol", "count": 0, "tags": []}`,
		},
		{
			name:       "nested schema fully populated",
			schemaPath: "schemas/nested.schema.json",
			data:       `{"user": {"email": "a@b.com", "age": 30, "address": {"city": "NYC", "zip": "10001"}}}`,
		},
		{
			name:       "nested schema required only (no optional address)",
			schemaPath: "schemas/nested.schema.json",
			data:       `{"user": {"email": "a@b.com", "age": 25}}`,
		},
		{
			name:       "strict types all correct",
			schemaPath: "schemas/strict.schema.json",
			data:       `{"id": 1, "active": true, "score": 9.5}`,
		},
		{
			name:       "integer is valid number",
			schemaPath: "schemas/strict.schema.json",
			data:       `{"id": 1, "active": false, "score": 100}`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := v.Validate(tc.schemaPath, []byte(tc.data))
			assert.NoError(t, err, "valid JSON should produce no validation error")
		})
	}
}

// ---------------------------------------------------------------------------
// AC-9: JSON missing a required field -> error with path info
// ---------------------------------------------------------------------------

func TestValidate_MissingRequiredField_Error(t *testing.T) {
	v := NewValidator(testFS())

	tests := []struct {
		name         string
		schemaPath   string
		data         string
		missingField string // must appear in error message
		wantPathHint string // partial path that error should reference
	}{
		{
			name:         "missing name field",
			schemaPath:   "schemas/simple.schema.json",
			data:         `{"count": 5}`,
			missingField: "name",
			wantPathHint: "name",
		},
		{
			name:         "missing count field",
			schemaPath:   "schemas/simple.schema.json",
			data:         `{"name": "alice"}`,
			missingField: "count",
			wantPathHint: "count",
		},
		{
			name:         "missing both required fields",
			schemaPath:   "schemas/simple.schema.json",
			data:         `{}`,
			missingField: "name",
			wantPathHint: "name",
		},
		{
			name:         "missing nested required field (user.email)",
			schemaPath:   "schemas/nested.schema.json",
			data:         `{"user": {"age": 30}}`,
			missingField: "email",
			wantPathHint: "email",
		},
		{
			name:         "missing top-level required (user)",
			schemaPath:   "schemas/nested.schema.json",
			data:         `{}`,
			missingField: "user",
			wantPathHint: "user",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := v.Validate(tc.schemaPath, []byte(tc.data))
			require.Error(t, err, "missing required field %q should produce an error", tc.missingField)

			errMsg := err.Error()
			assert.Contains(t, errMsg, tc.wantPathHint,
				"error for missing field %q should reference the field name/path; got: %s",
				tc.missingField, errMsg)
		})
	}
}

// ---------------------------------------------------------------------------
// AC-9: JSON with wrong type -> error
// ---------------------------------------------------------------------------

func TestValidate_WrongType_Error(t *testing.T) {
	v := NewValidator(testFS())

	tests := []struct {
		name       string
		schemaPath string
		data       string
		desc       string
	}{
		{
			name:       "string where integer expected",
			schemaPath: "schemas/simple.schema.json",
			data:       `{"name": "alice", "count": "not-a-number"}`,
			desc:       "count should be integer, not string",
		},
		{
			name:       "integer where string expected",
			schemaPath: "schemas/simple.schema.json",
			data:       `{"name": 42, "count": 1}`,
			desc:       "name should be string, not integer",
		},
		{
			name:       "string where boolean expected",
			schemaPath: "schemas/strict.schema.json",
			data:       `{"id": 1, "active": "yes", "score": 1.0}`,
			desc:       "active should be boolean, not string",
		},
		{
			name:       "boolean where integer expected",
			schemaPath: "schemas/strict.schema.json",
			data:       `{"id": true, "active": true, "score": 1.0}`,
			desc:       "id should be integer, not boolean",
		},
		{
			name:       "string where number expected",
			schemaPath: "schemas/strict.schema.json",
			data:       `{"id": 1, "active": true, "score": "high"}`,
			desc:       "score should be number, not string",
		},
		{
			name:       "tags is string instead of array",
			schemaPath: "schemas/simple.schema.json",
			data:       `{"name": "alice", "count": 1, "tags": "not-an-array"}`,
			desc:       "tags should be array, not string",
		},
		{
			name:       "tag items are numbers instead of strings",
			schemaPath: "schemas/simple.schema.json",
			data:       `{"name": "alice", "count": 1, "tags": [1, 2, 3]}`,
			desc:       "tag items should be strings, not numbers",
		},
		{
			name:       "nested type mismatch (user.age is string)",
			schemaPath: "schemas/nested.schema.json",
			data:       `{"user": {"email": "a@b.com", "age": "thirty"}}`,
			desc:       "age should be integer, not string",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := v.Validate(tc.schemaPath, []byte(tc.data))
			require.Error(t, err, "%s: should produce a type error", tc.desc)
		})
	}
}

// ---------------------------------------------------------------------------
// AC-9: Schema loaded from embed.FS (via fs.FS interface)
// ---------------------------------------------------------------------------

func TestValidate_SchemaLoadedFromFS(t *testing.T) {
	// This test verifies the validator actually reads from the provided FS,
	// not from hardcoded logic or any other source. We create a unique schema
	// that only exists in this specific FS and validate against it.
	customSchema := `{
		"$schema": "https://json-schema.org/draft/2020-12/schema",
		"type": "object",
		"required": ["magic"],
		"properties": {
			"magic": { "type": "string", "const": "abracadabra" }
		}
	}`

	fs := fstest.MapFS{
		"custom/magic.schema.json": &fstest.MapFile{
			Data: []byte(customSchema),
		},
	}
	v := NewValidator(fs)

	// Valid: matches the const constraint
	err := v.Validate("custom/magic.schema.json", []byte(`{"magic": "abracadabra"}`))
	assert.NoError(t, err, "JSON matching custom schema should pass")

	// Invalid: wrong value for const
	err = v.Validate("custom/magic.schema.json", []byte(`{"magic": "hocus pocus"}`))
	assert.Error(t, err, "JSON not matching const should fail")
}

// ---------------------------------------------------------------------------
// AC-10: Non-existent schema path -> error (not panic)
// ---------------------------------------------------------------------------

func TestValidate_NonExistentSchema_ErrorNotPanic(t *testing.T) {
	v := NewValidator(testFS())

	// This must not panic. It must return a clear error.
	err := v.Validate("schemas/does-not-exist.schema.json", []byte(`{"anything": true}`))
	require.Error(t, err, "validating against a non-existent schema must return an error, not panic")
}

// ---------------------------------------------------------------------------
// AC-10: Error message references the schema path
// ---------------------------------------------------------------------------

func TestValidate_NonExistentSchema_ErrorContainsPath(t *testing.T) {
	v := NewValidator(testFS())

	missingPath := "schemas/nonexistent.schema.json"
	err := v.Validate(missingPath, []byte(`{}`))
	require.Error(t, err)

	assert.Contains(t, err.Error(), missingPath,
		"error for missing schema should reference the schema path %q; got: %s",
		missingPath, err.Error())
}

func TestValidate_NonExistentSchema_MultiplePathsDistinct(t *testing.T) {
	// Ensure the error message actually uses the specific path, not a generic
	// hardcoded message. Test with two different missing paths.
	v := NewValidator(testFS())

	path1 := "schemas/missing-one.schema.json"
	path2 := "deeply/nested/other-missing.schema.json"

	err1 := v.Validate(path1, []byte(`{}`))
	require.Error(t, err1)
	assert.Contains(t, err1.Error(), path1)

	err2 := v.Validate(path2, []byte(`{}`))
	require.Error(t, err2)
	assert.Contains(t, err2.Error(), path2)

	// The two error messages must be different (not hardcoded to the same string).
	assert.NotEqual(t, err1.Error(), err2.Error(),
		"error messages for different missing paths must differ")
}

// ---------------------------------------------------------------------------
// Edge case: Empty JSON input
// ---------------------------------------------------------------------------

func TestValidate_EmptyInput_Error(t *testing.T) {
	v := NewValidator(testFS())

	tests := []struct {
		name string
		data []byte
	}{
		{name: "nil byte slice", data: nil},
		{name: "empty byte slice", data: []byte{}},
		{name: "whitespace only", data: []byte("   \n\t  ")},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := v.Validate("schemas/simple.schema.json", tc.data)
			require.Error(t, err,
				"empty/blank JSON input should produce an error, not silently succeed")
		})
	}
}

// ---------------------------------------------------------------------------
// Edge case: Valid JSON but not an object (schema requires object)
// ---------------------------------------------------------------------------

func TestValidate_NonObjectJSON_Error(t *testing.T) {
	v := NewValidator(testFS())

	tests := []struct {
		name string
		data string
	}{
		{name: "json null", data: `null`},
		{name: "json array", data: `[1, 2, 3]`},
		{name: "json string", data: `"hello"`},
		{name: "json number", data: `42`},
		{name: "json boolean", data: `true`},
		{name: "json empty array", data: `[]`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := v.Validate("schemas/simple.schema.json", []byte(tc.data))
			require.Error(t, err,
				"non-object JSON %q should fail against a schema requiring type:object", tc.data)
		})
	}
}

// ---------------------------------------------------------------------------
// Edge case: Extra properties in JSON -> still valid
// ---------------------------------------------------------------------------

func TestValidate_ExtraProperties_StillValid(t *testing.T) {
	v := NewValidator(testFS())

	// The schema does not set additionalProperties: false, so extra props
	// should be allowed by default.
	data := `{
		"name": "alice",
		"count": 5,
		"unexpected_field": "surprise",
		"another_extra": 999
	}`

	err := v.Validate("schemas/simple.schema.json", []byte(data))
	assert.NoError(t, err,
		"extra properties should be allowed when schema does not restrict additionalProperties")
}

// ---------------------------------------------------------------------------
// Edge case: Nested object validation
// ---------------------------------------------------------------------------

func TestValidate_NestedObjectValidation(t *testing.T) {
	v := NewValidator(testFS())

	tests := []struct {
		name    string
		data    string
		wantErr bool
		errHint string // substring expected in error, if any
	}{
		{
			name:    "nested address missing required city",
			data:    `{"user": {"email": "a@b.com", "age": 30, "address": {"zip": "10001"}}}`,
			wantErr: true,
			errHint: "city",
		},
		{
			name:    "nested address with all required fields",
			data:    `{"user": {"email": "a@b.com", "age": 30, "address": {"city": "NYC"}}}`,
			wantErr: false,
		},
		{
			name:    "user is string instead of object",
			data:    `{"user": "not-an-object"}`,
			wantErr: true,
			errHint: "user",
		},
		{
			name:    "address is array instead of object",
			data:    `{"user": {"email": "a@b.com", "age": 30, "address": [1,2,3]}}`,
			wantErr: true,
			errHint: "address",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := v.Validate("schemas/nested.schema.json", []byte(tc.data))
			if tc.wantErr {
				require.Error(t, err, "expected validation error for: %s", tc.name)
				if tc.errHint != "" {
					assert.Contains(t, err.Error(), tc.errHint,
						"error should reference %q", tc.errHint)
				}
			} else {
				assert.NoError(t, err, "expected no error for: %s", tc.name)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Edge case: Invalid JSON syntax (not schema violation — parse error)
// ---------------------------------------------------------------------------

func TestValidate_MalformedJSON_Error(t *testing.T) {
	v := NewValidator(testFS())

	tests := []struct {
		name string
		data string
	}{
		{name: "trailing comma", data: `{"name": "a", "count": 1,}`},
		{name: "single quotes", data: `{'name': 'a', 'count': 1}`},
		{name: "unquoted keys", data: `{name: "a", count: 1}`},
		{name: "truncated", data: `{"name": "a", "cou`},
		{name: "just open brace", data: `{`},
		{name: "random text", data: `not json at all`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := v.Validate("schemas/simple.schema.json", []byte(tc.data))
			require.Error(t, err,
				"malformed JSON %q should produce an error", tc.data)
		})
	}
}

// ---------------------------------------------------------------------------
// Edge case: Same validator instance reused for multiple schemas
// ---------------------------------------------------------------------------

func TestValidate_MultipleSchemas_SameValidator(t *testing.T) {
	v := NewValidator(testFS())

	// Validate against simple schema
	err := v.Validate("schemas/simple.schema.json", []byte(`{"name": "a", "count": 1}`))
	assert.NoError(t, err, "first schema validation should pass")

	// Validate against nested schema with same validator
	err = v.Validate("schemas/nested.schema.json", []byte(`{"user": {"email": "x@y.com", "age": 20}}`))
	assert.NoError(t, err, "second schema validation should pass")

	// Validate against strict schema with same validator
	err = v.Validate("schemas/strict.schema.json", []byte(`{"id": 1, "active": true, "score": 5.5}`))
	assert.NoError(t, err, "third schema validation should pass")

	// Now fail against simple schema to prove it is not just returning nil
	err = v.Validate("schemas/simple.schema.json", []byte(`{"name": 42, "count": "wrong"}`))
	assert.Error(t, err, "type mismatch should still fail after other validations")
}

// ---------------------------------------------------------------------------
// Edge case: Boundary values
// ---------------------------------------------------------------------------

func TestValidate_BoundaryValues(t *testing.T) {
	v := NewValidator(testFS())

	tests := []struct {
		name    string
		data    string
		wantErr bool
	}{
		{
			name:    "count is zero (valid integer)",
			data:    `{"name": "x", "count": 0}`,
			wantErr: false,
		},
		{
			name:    "count is negative (valid integer)",
			data:    `{"name": "x", "count": -1}`,
			wantErr: false,
		},
		{
			name:    "count is very large",
			data:    `{"name": "x", "count": 9999999999}`,
			wantErr: false,
		},
		{
			name:    "name is empty string (valid string)",
			data:    `{"name": "", "count": 1}`,
			wantErr: false,
		},
		{
			name:    "name contains unicode",
			data:    `{"name": "\u00e9\u00e8\u00ea\u4e16\u754c", "count": 1}`,
			wantErr: false,
		},
		{
			name:    "count is float (should fail for integer type)",
			data:    `{"name": "x", "count": 1.5}`,
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := v.Validate("schemas/simple.schema.json", []byte(tc.data))
			if tc.wantErr {
				assert.Error(t, err, "expected validation error for: %s", tc.name)
			} else {
				assert.NoError(t, err, "expected no error for: %s", tc.name)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Cross-cutting: NewValidator returns non-nil Validator
// ---------------------------------------------------------------------------

func TestNewValidator_ReturnsNonNil(t *testing.T) {
	v := NewValidator(testFS())
	require.NotNil(t, v, "NewValidator must return a non-nil Validator")
}

// ---------------------------------------------------------------------------
// Cross-cutting: Validate does not panic on any input combination
// ---------------------------------------------------------------------------

func TestValidate_NeverPanics(t *testing.T) {
	v := NewValidator(testFS())

	adversarialInputs := []struct {
		name       string
		schemaPath string
		data       []byte
	}{
		{"nil data, valid schema", "schemas/simple.schema.json", nil},
		{"empty data, valid schema", "schemas/simple.schema.json", []byte{}},
		{"valid data, empty schema path", "", []byte(`{"name":"a","count":1}`)},
		{"valid data, missing schema", "nope.json", []byte(`{"name":"a","count":1}`)},
		{"garbage data, valid schema", "schemas/simple.schema.json", []byte{0xff, 0xfe, 0x00}},
		{"huge JSON", "schemas/simple.schema.json", []byte(`{"name":"` + strings.Repeat("a", 100000) + `","count":1}`)},
	}

	for _, tc := range adversarialInputs {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("Validate panicked on %q: %v", tc.name, r)
				}
			}()
			_ = v.Validate(tc.schemaPath, tc.data)
		})
	}
}

// ---------------------------------------------------------------------------
// Robustness: Invalid schema file content in FS
// ---------------------------------------------------------------------------

func TestValidate_InvalidSchemaContent_Error(t *testing.T) {
	// The FS contains a file at the schema path, but the content is not
	// valid JSON Schema. The validator should return an error, not panic.
	badFS := fstest.MapFS{
		"schemas/bad.schema.json": &fstest.MapFile{
			Data: []byte(`this is not json`),
		},
	}
	v := NewValidator(badFS)

	err := v.Validate("schemas/bad.schema.json", []byte(`{"anything": true}`))
	require.Error(t, err, "invalid schema content should produce an error")
}

func TestValidate_EmptySchemaFile_Error(t *testing.T) {
	emptyFS := fstest.MapFS{
		"schemas/empty.schema.json": &fstest.MapFile{
			Data: []byte{},
		},
	}
	v := NewValidator(emptyFS)

	err := v.Validate("schemas/empty.schema.json", []byte(`{"anything": true}`))
	require.Error(t, err, "empty schema file should produce an error")
}

// ---------------------------------------------------------------------------
// Robustness: Validate with empty schema path
// ---------------------------------------------------------------------------

func TestValidate_EmptySchemaPath_Error(t *testing.T) {
	v := NewValidator(testFS())

	err := v.Validate("", []byte(`{"name": "a", "count": 1}`))
	require.Error(t, err, "empty schema path should produce an error, not silently succeed")
}

// ---------------------------------------------------------------------------
// AC-9 Anti-hardcoding: Different valid/invalid combos for same schema
// ---------------------------------------------------------------------------

func TestValidate_NotHardcoded(t *testing.T) {
	// Guard against implementations that return nil for specific schema paths
	// or specific JSON strings. We test multiple distinct valid inputs and
	// multiple distinct invalid inputs against the same schema.
	v := NewValidator(testFS())

	valids := []string{
		`{"name": "alpha", "count": 1}`,
		`{"name": "bravo", "count": 2}`,
		`{"name": "charlie", "count": 999, "tags": ["a"]}`,
		`{"name": "", "count": 0}`,
	}

	invalids := []string{
		`{"name": "alpha"}`,              // missing count
		`{"count": 1}`,                   // missing name
		`{}`,                             // missing both
		`{"name": 1, "count": 1}`,        // wrong type name
		`{"name": "a", "count": "b"}`,    // wrong type count
		`{"name": true, "count": false}`, // both wrong type
	}

	for _, data := range valids {
		err := v.Validate("schemas/simple.schema.json", []byte(data))
		assert.NoError(t, err, "valid JSON %q should pass", data)
	}

	for _, data := range invalids {
		err := v.Validate("schemas/simple.schema.json", []byte(data))
		assert.Error(t, err, "invalid JSON %q should fail", data)
	}
}
