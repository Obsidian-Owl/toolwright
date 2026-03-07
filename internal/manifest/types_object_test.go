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
// AC1: ItemSchema field parses from YAML into map[string]any
// ---------------------------------------------------------------------------

// objectFlagManifestYAML is a realistic manifest with a flag that has an
// inline JSON Schema under itemSchema. The schema has type, properties, and
// required -- the three most common JSON Schema keywords.
const objectFlagManifestYAML = `apiVersion: toolwright/v1
kind: Toolkit
metadata:
  name: object-test
  version: 1.0.0
  description: Tests object-typed flags with itemSchema
tools:
  - name: create-item
    description: Create a structured item
    entrypoint: ./create.sh
    flags:
      - name: item
        type: "object"
        required: true
        description: The item to create
        itemSchema:
          type: object
          properties:
            name:
              type: string
            age:
              type: integer
            active:
              type: boolean
          required:
            - name
`

func TestObjectFlag_ItemSchema_ParsesIntoMap(t *testing.T) {
	got, err := Parse(strings.NewReader(objectFlagManifestYAML))
	require.NoError(t, err, "Parse should accept manifests with object-typed flags and itemSchema")
	require.NotNil(t, got)
	require.Len(t, got.Tools, 1)
	require.Len(t, got.Tools[0].Flags, 1)

	flag := got.Tools[0].Flags[0]

	// The ItemSchema field must exist on the Flag struct and be populated.
	require.NotNil(t, flag.ItemSchema,
		"Flag.ItemSchema must not be nil when itemSchema is present in YAML")

	// Verify it is a map with the expected top-level keys.
	assert.Contains(t, flag.ItemSchema, "type",
		"ItemSchema should contain 'type' key")
	assert.Contains(t, flag.ItemSchema, "properties",
		"ItemSchema should contain 'properties' key")
	assert.Contains(t, flag.ItemSchema, "required",
		"ItemSchema should contain 'required' key")

	// Verify the 'type' value is the string "object".
	assert.Equal(t, "object", flag.ItemSchema["type"],
		"ItemSchema 'type' should be 'object'")
}

func TestObjectFlag_ItemSchema_PropertiesAreParsed(t *testing.T) {
	got, err := Parse(strings.NewReader(objectFlagManifestYAML))
	require.NoError(t, err)
	require.Len(t, got.Tools, 1)
	require.Len(t, got.Tools[0].Flags, 1)

	schema := got.Tools[0].Flags[0].ItemSchema
	require.NotNil(t, schema)

	// properties should be a nested map.
	props, ok := schema["properties"].(map[string]any)
	require.True(t, ok,
		"ItemSchema 'properties' should be map[string]any, got %T", schema["properties"])

	// Verify each property key exists with the correct nested type.
	require.Contains(t, props, "name")
	require.Contains(t, props, "age")
	require.Contains(t, props, "active")

	// Each property should itself be a map with a "type" key.
	nameProp, ok := props["name"].(map[string]any)
	require.True(t, ok, "name property should be a map, got %T", props["name"])
	assert.Equal(t, "string", nameProp["type"])

	ageProp, ok := props["age"].(map[string]any)
	require.True(t, ok, "age property should be a map, got %T", props["age"])
	assert.Equal(t, "integer", ageProp["type"])

	activeProp, ok := props["active"].(map[string]any)
	require.True(t, ok, "active property should be a map, got %T", props["active"])
	assert.Equal(t, "boolean", activeProp["type"])
}

func TestObjectFlag_ItemSchema_RequiredArray(t *testing.T) {
	got, err := Parse(strings.NewReader(objectFlagManifestYAML))
	require.NoError(t, err)
	require.Len(t, got.Tools, 1)
	require.Len(t, got.Tools[0].Flags, 1)

	schema := got.Tools[0].Flags[0].ItemSchema
	require.NotNil(t, schema)

	// required should be a slice of interface{} containing "name".
	reqRaw, ok := schema["required"]
	require.True(t, ok, "ItemSchema must have 'required' key")

	reqSlice, ok := reqRaw.([]any)
	require.True(t, ok,
		"ItemSchema 'required' should be []any, got %T", reqRaw)
	require.Len(t, reqSlice, 1)
	assert.Equal(t, "name", reqSlice[0])
}

func TestObjectFlag_ItemSchema_OtherFlagFieldsPreserved(t *testing.T) {
	got, err := Parse(strings.NewReader(objectFlagManifestYAML))
	require.NoError(t, err)
	require.Len(t, got.Tools, 1)
	require.Len(t, got.Tools[0].Flags, 1)

	flag := got.Tools[0].Flags[0]

	// The presence of itemSchema must not interfere with other flag fields.
	assert.Equal(t, "item", flag.Name)
	assert.Equal(t, "object", flag.Type)
	assert.True(t, flag.Required)
	assert.Equal(t, "The item to create", flag.Description)
}

// ---------------------------------------------------------------------------
// AC1: ItemSchema omitempty -- nil when absent
// ---------------------------------------------------------------------------

func TestObjectFlag_ItemSchema_NilWhenAbsent(t *testing.T) {
	// A standard flag with no itemSchema should have nil ItemSchema.
	input := `apiVersion: toolwright/v1
kind: Toolkit
metadata:
  name: no-schema
  version: 1.0.0
  description: Flag without itemSchema
tools:
  - name: cmd
    description: A command
    entrypoint: ./cmd.sh
    flags:
      - name: verbose
        type: bool
        required: false
        description: Enable verbose
      - name: limit
        type: int
        required: false
        default: 10
        description: Limit results
`
	got, err := Parse(strings.NewReader(input))
	require.NoError(t, err)
	require.Len(t, got.Tools, 1)
	require.Len(t, got.Tools[0].Flags, 2)

	for _, flag := range got.Tools[0].Flags {
		assert.Nil(t, flag.ItemSchema,
			"Flag %q should have nil ItemSchema when not specified", flag.Name)
	}
}

func TestObjectFlag_ItemSchema_OmitEmptyRoundTrip(t *testing.T) {
	// When ItemSchema is absent, a marshal/unmarshal round-trip should
	// preserve the absence (not introduce an empty map).
	input := `apiVersion: toolwright/v1
kind: Toolkit
metadata:
  name: omit-test
  version: 1.0.0
  description: Omitempty test
tools:
  - name: cmd
    description: A command
    entrypoint: ./cmd.sh
    flags:
      - name: verbose
        type: bool
        required: false
        description: Enable verbose
`
	original, err := Parse(strings.NewReader(input))
	require.NoError(t, err)
	require.Len(t, original.Tools, 1)
	require.Len(t, original.Tools[0].Flags, 1)
	require.Nil(t, original.Tools[0].Flags[0].ItemSchema,
		"Pre-condition: ItemSchema is nil before round-trip")

	marshalled, err := yaml.Marshal(original)
	require.NoError(t, err)

	// The marshalled YAML should NOT contain "itemSchema" at all.
	assert.NotContains(t, string(marshalled), "itemSchema",
		"Marshalled YAML must not contain 'itemSchema' when it is nil (omitempty)")

	roundTripped, err := Parse(strings.NewReader(string(marshalled)))
	require.NoError(t, err)
	require.Len(t, roundTripped.Tools, 1)
	require.Len(t, roundTripped.Tools[0].Flags, 1)
	assert.Nil(t, roundTripped.Tools[0].Flags[0].ItemSchema,
		"ItemSchema must remain nil after round-trip")
}

// ---------------------------------------------------------------------------
// AC2: "object" and "object[]" are valid types
// ---------------------------------------------------------------------------

func TestObjectType_IsArrayType(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantTrue bool
	}{
		{name: "object[] is array type", input: "object[]", wantTrue: true},
		{name: "object is NOT array type", input: "object", wantTrue: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := IsArrayType(tc.input)
			assert.Equal(t, tc.wantTrue, got,
				"IsArrayType(%q) = %v, want %v", tc.input, got, tc.wantTrue)
		})
	}
}

func TestObjectType_BaseType(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantBase string
	}{
		{name: "object[] base is object", input: "object[]", wantBase: "object"},
		{name: "object scalar has no base", input: "object", wantBase: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := BaseType(tc.input)
			assert.Equal(t, tc.wantBase, got,
				"BaseType(%q) = %q, want %q", tc.input, got, tc.wantBase)
		})
	}
}

func TestObjectType_IsArrayType_ConsistentWithBaseType(t *testing.T) {
	// Verify that IsArrayType and BaseType agree for object types, the same
	// way the existing test does for other types.
	objectTypes := []string{"object", "object[]"}

	for _, typ := range objectTypes {
		t.Run(typ, func(t *testing.T) {
			isArray := IsArrayType(typ)
			base := BaseType(typ)

			if isArray {
				assert.NotEmpty(t, base,
					"IsArrayType(%q) is true but BaseType returns empty", typ)
				assert.Equal(t, "object", base,
					"BaseType(%q) should return 'object' for object array type", typ)
			} else {
				assert.Empty(t, base,
					"IsArrayType(%q) is false but BaseType returns %q", typ, base)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// AC2: "object[]" flag parses correctly
// ---------------------------------------------------------------------------

func TestObjectArrayFlag_Parse(t *testing.T) {
	input := `apiVersion: toolwright/v1
kind: Toolkit
metadata:
  name: object-array-test
  version: 1.0.0
  description: Tests object[] flag type
tools:
  - name: batch-create
    description: Create multiple items
    entrypoint: ./batch.sh
    flags:
      - name: items
        type: "object[]"
        required: true
        description: Items to create
        itemSchema:
          type: object
          properties:
            id:
              type: integer
            label:
              type: string
`
	got, err := Parse(strings.NewReader(input))
	require.NoError(t, err, "Parse should accept object[] flag type")
	require.NotNil(t, got)
	require.Len(t, got.Tools, 1)
	require.Len(t, got.Tools[0].Flags, 1)

	flag := got.Tools[0].Flags[0]
	assert.Equal(t, "object[]", flag.Type)
	assert.Equal(t, "items", flag.Name)
	assert.True(t, flag.Required)

	// ItemSchema should still be parsed for object[] type.
	require.NotNil(t, flag.ItemSchema)
	assert.Equal(t, "object", flag.ItemSchema["type"])

	props, ok := flag.ItemSchema["properties"].(map[string]any)
	require.True(t, ok)
	assert.Contains(t, props, "id")
	assert.Contains(t, props, "label")
}

// ---------------------------------------------------------------------------
// AC1 + AC2: Object flag round-trip
// ---------------------------------------------------------------------------

func TestObjectFlag_RoundTrip(t *testing.T) {
	original, err := Parse(strings.NewReader(objectFlagManifestYAML))
	require.NoError(t, err, "First parse should succeed")
	require.NotNil(t, original)

	marshalled, err := yaml.Marshal(original)
	require.NoError(t, err, "Marshal should succeed")
	require.NotEmpty(t, marshalled, "Marshalled YAML must not be empty")

	roundTripped, err := Parse(strings.NewReader(string(marshalled)))
	require.NoError(t, err, "Second parse (round-trip) should succeed")
	require.NotNil(t, roundTripped)

	if diff := cmp.Diff(original, roundTripped); diff != "" {
		t.Errorf("Round-trip mismatch for object flag (-original +roundTripped):\n%s", diff)
	}
}

func TestObjectFlag_RoundTrip_ItemSchemaPreserved(t *testing.T) {
	original, err := Parse(strings.NewReader(objectFlagManifestYAML))
	require.NoError(t, err)

	marshalled, err := yaml.Marshal(original)
	require.NoError(t, err)

	roundTripped, err := Parse(strings.NewReader(string(marshalled)))
	require.NoError(t, err)

	require.Len(t, roundTripped.Tools, 1)
	require.Len(t, roundTripped.Tools[0].Flags, 1)

	schema := roundTripped.Tools[0].Flags[0].ItemSchema
	require.NotNil(t, schema,
		"ItemSchema must survive round-trip")
	assert.Equal(t, "object", schema["type"],
		"ItemSchema 'type' must survive round-trip")
	assert.Contains(t, schema, "properties",
		"ItemSchema 'properties' must survive round-trip")
	assert.Contains(t, schema, "required",
		"ItemSchema 'required' must survive round-trip")

	// Verify that nested property types survive round-trip.
	props, ok := schema["properties"].(map[string]any)
	require.True(t, ok)
	nameProp, ok := props["name"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "string", nameProp["type"],
		"Nested property type 'string' must survive round-trip")
}

func TestObjectArrayFlag_RoundTrip(t *testing.T) {
	input := `apiVersion: toolwright/v1
kind: Toolkit
metadata:
  name: object-array-rt
  version: 1.0.0
  description: Object array round-trip
tools:
  - name: batch
    description: Batch op
    entrypoint: ./batch.sh
    flags:
      - name: items
        type: "object[]"
        required: false
        description: Items to process
        itemSchema:
          type: object
          properties:
            key:
              type: string
            value:
              type: number
`
	original, err := Parse(strings.NewReader(input))
	require.NoError(t, err)

	marshalled, err := yaml.Marshal(original)
	require.NoError(t, err)

	roundTripped, err := Parse(strings.NewReader(string(marshalled)))
	require.NoError(t, err)

	if diff := cmp.Diff(original, roundTripped); diff != "" {
		t.Errorf("Round-trip mismatch for object[] flag (-original +roundTripped):\n%s", diff)
	}

	// Verify type string survives.
	assert.Equal(t, "object[]", roundTripped.Tools[0].Flags[0].Type,
		"Type 'object[]' must survive round-trip")
}

// ---------------------------------------------------------------------------
// Mixed: object flags coexist with scalar and array flags
// ---------------------------------------------------------------------------

func TestObjectFlag_MixedWithOtherTypes(t *testing.T) {
	input := `apiVersion: toolwright/v1
kind: Toolkit
metadata:
  name: mixed-object
  version: 1.0.0
  description: Mixed flag types including object
tools:
  - name: cmd
    description: A command
    entrypoint: ./cmd.sh
    flags:
      - name: verbose
        type: bool
        required: false
        default: false
        description: Verbose output
      - name: config
        type: "object"
        required: true
        description: Configuration object
        itemSchema:
          type: object
          properties:
            host:
              type: string
            port:
              type: integer
      - name: tags
        type: "string[]"
        required: false
        description: Tags
      - name: entries
        type: "object[]"
        required: false
        description: Entry objects
        itemSchema:
          type: object
          properties:
            name:
              type: string
`
	got, err := Parse(strings.NewReader(input))
	require.NoError(t, err)
	require.Len(t, got.Tools, 1)
	flags := got.Tools[0].Flags
	require.Len(t, flags, 4)

	// Scalar flag -- no itemSchema.
	assert.Equal(t, "bool", flags[0].Type)
	assert.Nil(t, flags[0].ItemSchema,
		"Scalar bool flag should not have ItemSchema")

	// Object flag -- has itemSchema.
	assert.Equal(t, "object", flags[1].Type)
	require.NotNil(t, flags[1].ItemSchema,
		"Object flag should have ItemSchema")
	assert.Equal(t, "object", flags[1].ItemSchema["type"])

	// Array flag -- no itemSchema.
	assert.Equal(t, "string[]", flags[2].Type)
	assert.Nil(t, flags[2].ItemSchema,
		"string[] flag should not have ItemSchema")

	// Object array flag -- has itemSchema.
	assert.Equal(t, "object[]", flags[3].Type)
	require.NotNil(t, flags[3].ItemSchema,
		"object[] flag should have ItemSchema")
	assert.Equal(t, "object", flags[3].ItemSchema["type"])
}

// ---------------------------------------------------------------------------
// AC1: ItemSchema with complex/nested schemas
// ---------------------------------------------------------------------------

func TestObjectFlag_ItemSchema_ComplexNestedSchema(t *testing.T) {
	// A more complex schema with nested objects and arrays to ensure
	// the parser handles arbitrary depth in map[string]any.
	input := `apiVersion: toolwright/v1
kind: Toolkit
metadata:
  name: nested-schema
  version: 1.0.0
  description: Complex nested itemSchema
tools:
  - name: deep
    description: Deep schema test
    entrypoint: ./deep.sh
    flags:
      - name: payload
        type: "object"
        required: true
        description: Complex payload
        itemSchema:
          type: object
          properties:
            address:
              type: object
              properties:
                street:
                  type: string
                city:
                  type: string
            tags:
              type: array
              items:
                type: string
          required:
            - address
`
	got, err := Parse(strings.NewReader(input))
	require.NoError(t, err)
	require.Len(t, got.Tools, 1)
	require.Len(t, got.Tools[0].Flags, 1)

	schema := got.Tools[0].Flags[0].ItemSchema
	require.NotNil(t, schema)

	// Verify nested object property.
	props, ok := schema["properties"].(map[string]any)
	require.True(t, ok)

	addrProp, ok := props["address"].(map[string]any)
	require.True(t, ok, "address property should be a map")
	assert.Equal(t, "object", addrProp["type"])

	addrProps, ok := addrProp["properties"].(map[string]any)
	require.True(t, ok, "address.properties should be a map")
	assert.Contains(t, addrProps, "street")
	assert.Contains(t, addrProps, "city")

	// Verify array property with items.
	tagsProp, ok := props["tags"].(map[string]any)
	require.True(t, ok, "tags property should be a map")
	assert.Equal(t, "array", tagsProp["type"])
	assert.Contains(t, tagsProp, "items")
}

func TestObjectFlag_ItemSchema_EmptySchema(t *testing.T) {
	// An empty itemSchema (just `itemSchema: {}`) should parse as an
	// empty map, not nil.
	input := `apiVersion: toolwright/v1
kind: Toolkit
metadata:
  name: empty-schema
  version: 1.0.0
  description: Empty itemSchema
tools:
  - name: cmd
    description: A command
    entrypoint: ./cmd.sh
    flags:
      - name: data
        type: "object"
        required: false
        description: Unstructured data
        itemSchema: {}
`
	got, err := Parse(strings.NewReader(input))
	require.NoError(t, err)
	require.Len(t, got.Tools, 1)
	require.Len(t, got.Tools[0].Flags, 1)

	schema := got.Tools[0].Flags[0].ItemSchema
	// An explicit empty map should not be nil.
	require.NotNil(t, schema,
		"itemSchema: {} should parse as non-nil empty map")
	assert.Len(t, schema, 0,
		"itemSchema: {} should parse as an empty map")
}

// ---------------------------------------------------------------------------
// Adversarial: catch hardcoded or shortcut implementations
// ---------------------------------------------------------------------------

func TestObjectType_NotJustAnyBracketSuffix(t *testing.T) {
	// Verify that random types with [] suffix still don't pass,
	// even now that object[] is valid.
	invalidTypes := []string{
		"custom[]",
		"map[]",
		"any[]",
		"struct[]",
		"Object[]",  // wrong case
		"OBJECT[]",  // wrong case
		"[]object",  // wrong order
		"object [[", // malformed brackets
	}

	for _, typ := range invalidTypes {
		t.Run(typ, func(t *testing.T) {
			assert.False(t, IsArrayType(typ),
				"IsArrayType(%q) should return false", typ)
			assert.Empty(t, BaseType(typ),
				"BaseType(%q) should return empty", typ)
		})
	}
}

func TestObjectType_BaseType_ReturnsExactString(t *testing.T) {
	// BaseType("object[]") must return exactly "object", not "Object",
	// not "object[]", not "objec".
	base := BaseType("object[]")
	assert.Equal(t, "object", base,
		"BaseType(\"object[]\") must return exactly \"object\"")

	// Verify it is exactly 6 characters.
	assert.Len(t, base, 6,
		"BaseType result should be exactly 6 characters long")
}

func TestObjectType_AllFiveDistinct(t *testing.T) {
	// After adding object[], there should be 5 valid array types.
	// All must be recognized and produce distinct base types.
	expected := map[string]string{
		"string[]": "string",
		"int[]":    "int",
		"float[]":  "float",
		"bool[]":   "bool",
		"object[]": "object",
	}

	for input, wantBase := range expected {
		t.Run(input, func(t *testing.T) {
			assert.True(t, IsArrayType(input),
				"IsArrayType(%q) must return true", input)
			assert.Equal(t, wantBase, BaseType(input),
				"BaseType(%q) must return %q", input, wantBase)
		})
	}

	// Verify all 5 base types are distinct from each other.
	seen := make(map[string]bool)
	for input := range expected {
		got := BaseType(input)
		if seen[got] {
			t.Errorf("BaseType(%q) returned %q which was already seen -- base types must be distinct", input, got)
		}
		seen[got] = true
	}
}

func TestObjectFlag_ItemSchema_YAMLTagIsCorrect(t *testing.T) {
	// This test verifies the YAML tag is exactly "itemSchema,omitempty"
	// by checking that the key "itemSchema" in YAML maps to the struct field.
	// If someone uses "item_schema" or "schema" as the tag, this fails.
	input := `apiVersion: toolwright/v1
kind: Toolkit
metadata:
  name: tag-test
  version: 1.0.0
  description: YAML tag test
tools:
  - name: cmd
    description: A command
    entrypoint: ./cmd.sh
    flags:
      - name: data
        type: "object"
        required: false
        description: Data field
        itemSchema:
          type: object
`
	got, err := Parse(strings.NewReader(input))
	require.NoError(t, err)
	require.Len(t, got.Tools, 1)
	require.Len(t, got.Tools[0].Flags, 1)

	// If the tag were wrong, this would be nil.
	require.NotNil(t, got.Tools[0].Flags[0].ItemSchema,
		"Flag.ItemSchema must be populated when YAML key is 'itemSchema'")
	assert.Equal(t, "object", got.Tools[0].Flags[0].ItemSchema["type"])
}

func TestObjectFlag_ItemSchema_AlternateKeyNames_DoNotParse(t *testing.T) {
	// Verify that alternate key names like "item_schema" or "schema" do NOT
	// populate ItemSchema. Only the exact YAML key "itemSchema" should work.
	tests := []struct {
		name    string
		yamlKey string
	}{
		{name: "snake_case item_schema", yamlKey: "item_schema"},
		{name: "bare schema", yamlKey: "schema"},
		{name: "camelCase itemschema (no cap)", yamlKey: "itemschema"},
		{name: "ItemSchema (PascalCase)", yamlKey: "ItemSchema"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			input := `apiVersion: toolwright/v1
kind: Toolkit
metadata:
  name: wrong-key
  version: 1.0.0
  description: Wrong key test
tools:
  - name: cmd
    description: A command
    entrypoint: ./cmd.sh
    flags:
      - name: data
        type: "object"
        required: false
        description: Data field
        ` + tc.yamlKey + `:
          type: object
`
			got, err := Parse(strings.NewReader(input))
			require.NoError(t, err)
			require.Len(t, got.Tools, 1)
			require.Len(t, got.Tools[0].Flags, 1)

			assert.Nil(t, got.Tools[0].Flags[0].ItemSchema,
				"Flag.ItemSchema must be nil when YAML key is %q (only 'itemSchema' should work)",
				tc.yamlKey)
		})
	}
}
