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
// Spec criterion 1: IsArrayType and BaseType helpers
// ---------------------------------------------------------------------------

func TestIsArrayType(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantTrue bool
	}{
		// Valid array types — all 4 must return true.
		{name: "string[]", input: "string[]", wantTrue: true},
		{name: "int[]", input: "int[]", wantTrue: true},
		{name: "float[]", input: "float[]", wantTrue: true},
		{name: "bool[]", input: "bool[]", wantTrue: true},

		// Scalar types — all 4 must return false.
		{name: "string scalar", input: "string", wantTrue: false},
		{name: "int scalar", input: "int", wantTrue: false},
		{name: "float scalar", input: "float", wantTrue: false},
		{name: "bool scalar", input: "bool", wantTrue: false},

		// Edge cases — all must return false.
		{name: "empty string", input: "", wantTrue: false},
		{name: "bare brackets", input: "[]", wantTrue: false},
		{name: "object array", input: "object[]", wantTrue: false},
		{name: "nested array", input: "string[][]", wantTrue: false},
		{name: "uppercase STRING[]", input: "STRING[]", wantTrue: false},
		{name: "mixed case String[]", input: "String[]", wantTrue: false},
		{name: "unknown type array", input: "unknown[]", wantTrue: false},
		{name: "array prefix not suffix", input: "[]string", wantTrue: false},
		{name: "whitespace around brackets", input: "string[ ]", wantTrue: false},
		{name: "just brackets with space", input: "[ ]", wantTrue: false},
		{name: "number array", input: "number[]", wantTrue: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := IsArrayType(tc.input)
			assert.Equal(t, tc.wantTrue, got,
				"IsArrayType(%q) = %v, want %v", tc.input, got, tc.wantTrue)
		})
	}
}

// TestIsArrayType_ConsistentWithBaseType verifies that every type recognized
// by IsArrayType also produces a non-empty BaseType, and vice versa. This
// catches implementations where the two functions disagree.
func TestIsArrayType_ConsistentWithBaseType(t *testing.T) {
	types := []string{
		"string[]", "int[]", "float[]", "bool[]",
		"string", "int", "float", "bool",
		"", "[]", "object[]", "string[][]", "unknown",
	}

	for _, typ := range types {
		t.Run(typ, func(t *testing.T) {
			isArray := IsArrayType(typ)
			base := BaseType(typ)

			if isArray {
				assert.NotEmpty(t, base,
					"IsArrayType(%q) is true but BaseType returns empty", typ)
			} else {
				assert.Empty(t, base,
					"IsArrayType(%q) is false but BaseType returns %q", typ, base)
			}
		})
	}
}

func TestBaseType(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantBase string
	}{
		// Valid array types — must return the base scalar.
		{name: "string[] base", input: "string[]", wantBase: "string"},
		{name: "int[] base", input: "int[]", wantBase: "int"},
		{name: "float[] base", input: "float[]", wantBase: "float"},
		{name: "bool[] base", input: "bool[]", wantBase: "bool"},

		// Non-array types — must return empty string.
		{name: "string scalar", input: "string", wantBase: ""},
		{name: "int scalar", input: "int", wantBase: ""},
		{name: "float scalar", input: "float", wantBase: ""},
		{name: "bool scalar", input: "bool", wantBase: ""},

		// Edge cases — must return empty string.
		{name: "empty string", input: "", wantBase: ""},
		{name: "bare brackets", input: "[]", wantBase: ""},
		{name: "object array", input: "object[]", wantBase: ""},
		{name: "nested array", input: "string[][]", wantBase: ""},
		{name: "uppercase STRING[]", input: "STRING[]", wantBase: ""},
		{name: "unknown type array", input: "unknown[]", wantBase: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := BaseType(tc.input)
			assert.Equal(t, tc.wantBase, got,
				"BaseType(%q) = %q, want %q", tc.input, got, tc.wantBase)
		})
	}
}

// TestBaseType_ReturnsExactScalarName verifies that BaseType returns the
// canonical lowercase scalar name, not a trimmed/sliced variant that could
// contain extra characters.
func TestBaseType_ReturnsExactScalarName(t *testing.T) {
	validArrayTypes := map[string]string{
		"string[]": "string",
		"int[]":    "int",
		"float[]":  "float",
		"bool[]":   "bool",
	}

	for input, wantExact := range validArrayTypes {
		got := BaseType(input)
		// Use == not Contains: must be exactly the scalar name, nothing more.
		if got != wantExact {
			t.Errorf("BaseType(%q) = %q, want exactly %q", input, got, wantExact)
		}
	}
}

// ---------------------------------------------------------------------------
// Spec criterion 2: Manifest with array-typed flags parses successfully
// ---------------------------------------------------------------------------

const arrayFlagManifestYAML = `apiVersion: toolwright/v1
kind: Toolkit
metadata:
  name: array-test
  version: 1.0.0
  description: Tests array-typed flags
tools:
  - name: search
    description: Search with multiple terms
    entrypoint: ./search.sh
    flags:
      - name: tags
        type: "string[]"
        required: false
        default: ["a", "b"]
        description: Tags to search for
      - name: ids
        type: "int[]"
        required: false
        description: IDs to filter by
      - name: weights
        type: "float[]"
        required: false
        description: Weight values
      - name: features
        type: "bool[]"
        required: false
        description: Feature toggles
`

func TestParse_ArrayTypedFlag_TypePreserved(t *testing.T) {
	got, err := Parse(strings.NewReader(arrayFlagManifestYAML))
	require.NoError(t, err, "Parse should accept manifests with array-typed flags")
	require.NotNil(t, got)
	require.Len(t, got.Tools, 1)
	require.Len(t, got.Tools[0].Flags, 4, "Should have 4 flags")

	// Verify each flag's type string is preserved verbatim.
	flags := got.Tools[0].Flags
	assert.Equal(t, "string[]", flags[0].Type,
		"Flag type should be the exact string 'string[]'")
	assert.Equal(t, "int[]", flags[1].Type,
		"Flag type should be the exact string 'int[]'")
	assert.Equal(t, "float[]", flags[2].Type,
		"Flag type should be the exact string 'float[]'")
	assert.Equal(t, "bool[]", flags[3].Type,
		"Flag type should be the exact string 'bool[]'")
}

func TestParse_ArrayTypedFlag_DefaultIsSlice(t *testing.T) {
	got, err := Parse(strings.NewReader(arrayFlagManifestYAML))
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Len(t, got.Tools, 1)
	require.Len(t, got.Tools[0].Flags, 4)

	tagsFlag := got.Tools[0].Flags[0]

	// The default value must be a slice type. YAML decodes
	// ["a", "b"] as []interface{} when the target field is `any`.
	require.NotNil(t, tagsFlag.Default,
		"tags flag default should not be nil")

	defaultSlice, ok := tagsFlag.Default.([]interface{})
	require.True(t, ok,
		"Default should be []interface{}, got %T", tagsFlag.Default)
	require.Len(t, defaultSlice, 2,
		"Default slice should have exactly 2 elements")
	assert.Equal(t, "a", defaultSlice[0],
		"First default element should be 'a'")
	assert.Equal(t, "b", defaultSlice[1],
		"Second default element should be 'b'")
}

func TestParse_ArrayTypedFlag_NoDefault_IsNil(t *testing.T) {
	got, err := Parse(strings.NewReader(arrayFlagManifestYAML))
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Len(t, got.Tools, 1)
	require.Len(t, got.Tools[0].Flags, 4)

	// The int[], float[], and bool[] flags have no default specified.
	for _, idx := range []int{1, 2, 3} {
		assert.Nil(t, got.Tools[0].Flags[idx].Default,
			"Flag %q should have nil default when not specified",
			got.Tools[0].Flags[idx].Name)
	}
}

func TestParse_ArrayTypedFlag_OtherFieldsPreserved(t *testing.T) {
	got, err := Parse(strings.NewReader(arrayFlagManifestYAML))
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Len(t, got.Tools, 1)

	tagsFlag := got.Tools[0].Flags[0]
	assert.Equal(t, "tags", tagsFlag.Name,
		"Flag name should be preserved")
	assert.False(t, tagsFlag.Required,
		"Flag required should be preserved")
	assert.Equal(t, "Tags to search for", tagsFlag.Description,
		"Flag description should be preserved")
}

func TestParse_ArrayTypedFlag_EmptyDefaultSlice(t *testing.T) {
	input := `apiVersion: toolwright/v1
kind: Toolkit
metadata:
  name: empty-array-default
  version: 1.0.0
  description: Array flag with empty default
tools:
  - name: cmd
    description: A command
    entrypoint: ./cmd.sh
    flags:
      - name: items
        type: "string[]"
        required: false
        default: []
        description: An empty default list
`
	got, err := Parse(strings.NewReader(input))
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Len(t, got.Tools, 1)
	require.Len(t, got.Tools[0].Flags, 1)

	itemsFlag := got.Tools[0].Flags[0]
	assert.Equal(t, "string[]", itemsFlag.Type)

	// Empty YAML array `[]` should parse as an empty slice, not nil.
	defaultSlice, ok := itemsFlag.Default.([]interface{})
	require.True(t, ok,
		"Empty default should be []interface{}, got %T", itemsFlag.Default)
	assert.Len(t, defaultSlice, 0,
		"Empty default should have 0 elements")
}

func TestParse_ArrayTypedFlag_RoundTrip(t *testing.T) {
	// Step 1: Parse the manifest.
	original, err := Parse(strings.NewReader(arrayFlagManifestYAML))
	require.NoError(t, err, "First parse should succeed")
	require.NotNil(t, original)

	// Step 2: Marshal back to YAML.
	marshalled, err := yaml.Marshal(original)
	require.NoError(t, err, "Marshal should succeed")
	require.NotEmpty(t, marshalled, "Marshalled YAML must not be empty")

	// Step 3: Parse the marshalled YAML again.
	roundTripped, err := Parse(strings.NewReader(string(marshalled)))
	require.NoError(t, err, "Second parse (round-trip) should succeed")
	require.NotNil(t, roundTripped)

	// Step 4: Compare the two parsed structs deeply.
	if diff := cmp.Diff(original, roundTripped); diff != "" {
		t.Errorf("Round-trip mismatch for array-typed flags (-original +roundTripped):\n%s", diff)
	}
}

func TestParse_ArrayTypedFlag_RoundTrip_TypeStringPreserved(t *testing.T) {
	// Ensure the type string "string[]" is not mangled during round-trip.
	original, err := Parse(strings.NewReader(arrayFlagManifestYAML))
	require.NoError(t, err)

	marshalled, err := yaml.Marshal(original)
	require.NoError(t, err)

	roundTripped, err := Parse(strings.NewReader(string(marshalled)))
	require.NoError(t, err)

	require.Len(t, roundTripped.Tools, 1)
	require.Len(t, roundTripped.Tools[0].Flags, 4)

	assert.Equal(t, "string[]", roundTripped.Tools[0].Flags[0].Type,
		"Type string 'string[]' must survive round-trip")
	assert.Equal(t, "int[]", roundTripped.Tools[0].Flags[1].Type,
		"Type string 'int[]' must survive round-trip")
	assert.Equal(t, "float[]", roundTripped.Tools[0].Flags[2].Type,
		"Type string 'float[]' must survive round-trip")
	assert.Equal(t, "bool[]", roundTripped.Tools[0].Flags[3].Type,
		"Type string 'bool[]' must survive round-trip")
}

func TestParse_ArrayTypedFlag_RoundTrip_DefaultSlicePreserved(t *testing.T) {
	// Ensure the default ["a", "b"] round-trips correctly.
	original, err := Parse(strings.NewReader(arrayFlagManifestYAML))
	require.NoError(t, err)

	marshalled, err := yaml.Marshal(original)
	require.NoError(t, err)

	roundTripped, err := Parse(strings.NewReader(string(marshalled)))
	require.NoError(t, err)

	require.Len(t, roundTripped.Tools, 1)
	require.Len(t, roundTripped.Tools[0].Flags, 4)

	rtDefault := roundTripped.Tools[0].Flags[0].Default
	require.NotNil(t, rtDefault,
		"Default should survive round-trip")

	rtSlice, ok := rtDefault.([]interface{})
	require.True(t, ok,
		"Round-tripped default should be []interface{}, got %T", rtDefault)
	require.Len(t, rtSlice, 2)
	assert.Equal(t, "a", rtSlice[0])
	assert.Equal(t, "b", rtSlice[1])
}

// ---------------------------------------------------------------------------
// Adversarial: catch hardcoded or shortcut implementations
// ---------------------------------------------------------------------------

// TestIsArrayType_NotJustSuffixCheck verifies that IsArrayType doesn't
// merely check for a "[]" suffix — it must also validate the base type.
func TestIsArrayType_NotJustSuffixCheck(t *testing.T) {
	// These all end in "[]" but should NOT be recognized as valid array types.
	invalidSuffixed := []string{
		"object[]",
		"number[]",
		"any[]",
		"void[]",
		"map[]",
		"list[]",
		"[]",       // bare brackets
		"custom[]", // user-defined type
		"String[]", // wrong case
		"INT[]",    // wrong case
	}

	for _, input := range invalidSuffixed {
		got := IsArrayType(input)
		assert.False(t, got,
			"IsArrayType(%q) should return false — only string/int/float/bool arrays are valid", input)
	}
}

// TestBaseType_NotJustStringTrimming verifies that BaseType doesn't
// just trim "[]" from any string — it must validate the base type first.
func TestBaseType_NotJustStringTrimming(t *testing.T) {
	invalidTypes := []string{
		"object[]",
		"custom[]",
		"[]",
		"anything[]",
		"String[]",
	}

	for _, input := range invalidTypes {
		got := BaseType(input)
		assert.Empty(t, got,
			"BaseType(%q) should return empty string for invalid array types, got %q", input, got)
	}
}

// TestIsArrayType_AllFourDistinct verifies all 4 array types are handled
// independently — a lazy impl that only handles "string[]" would fail.
func TestIsArrayType_AllFourDistinct(t *testing.T) {
	arrayTypes := []string{"string[]", "int[]", "float[]", "bool[]"}
	results := make(map[string]bool, 4)

	for _, at := range arrayTypes {
		results[at] = IsArrayType(at)
	}

	// All 4 must be true. A hardcoded implementation that only checks
	// one specific type would fail on the others.
	for _, at := range arrayTypes {
		assert.True(t, results[at],
			"IsArrayType(%q) must return true", at)
	}
}

// TestBaseType_AllFourDistinct verifies all 4 base types are returned
// correctly — not all mapped to the same string.
func TestBaseType_AllFourDistinct(t *testing.T) {
	expected := map[string]string{
		"string[]": "string",
		"int[]":    "int",
		"float[]":  "float",
		"bool[]":   "bool",
	}

	gotBases := make(map[string]string, 4)
	for input, want := range expected {
		got := BaseType(input)
		assert.Equal(t, want, got,
			"BaseType(%q) = %q, want %q", input, got, want)
		gotBases[input] = got
	}

	// Verify all 4 base types are distinct from each other.
	seen := make(map[string]bool)
	for input, base := range gotBases {
		if seen[base] {
			t.Errorf("BaseType returned %q for %q, but that base type was already seen — implementations must not collapse types", base, input)
		}
		seen[base] = true
	}
}

// TestParse_ArrayTypedFlag_SingleElementDefault verifies that a single-element
// default array is still parsed as a slice, not collapsed to a scalar.
func TestParse_ArrayTypedFlag_SingleElementDefault(t *testing.T) {
	input := `apiVersion: toolwright/v1
kind: Toolkit
metadata:
  name: single-elem
  version: 1.0.0
  description: Single element default
tools:
  - name: cmd
    description: A command
    entrypoint: ./cmd.sh
    flags:
      - name: tags
        type: "string[]"
        required: false
        default: ["only"]
        description: A single-element default
`
	got, err := Parse(strings.NewReader(input))
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Len(t, got.Tools, 1)
	require.Len(t, got.Tools[0].Flags, 1)

	dflt := got.Tools[0].Flags[0].Default
	require.NotNil(t, dflt)

	slice, ok := dflt.([]interface{})
	require.True(t, ok,
		"Single-element default should still be []interface{}, got %T", dflt)
	require.Len(t, slice, 1)
	assert.Equal(t, "only", slice[0])
}

// TestParse_ArrayTypedFlag_IntDefault verifies integer array defaults
// are parsed with correct element types.
func TestParse_ArrayTypedFlag_IntDefault(t *testing.T) {
	input := `apiVersion: toolwright/v1
kind: Toolkit
metadata:
  name: int-array-default
  version: 1.0.0
  description: Int array default
tools:
  - name: cmd
    description: A command
    entrypoint: ./cmd.sh
    flags:
      - name: ids
        type: "int[]"
        required: false
        default: [1, 2, 3]
        description: ID list
`
	got, err := Parse(strings.NewReader(input))
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Len(t, got.Tools, 1)
	require.Len(t, got.Tools[0].Flags, 1)

	dflt := got.Tools[0].Flags[0].Default
	require.NotNil(t, dflt)

	slice, ok := dflt.([]interface{})
	require.True(t, ok,
		"Int array default should be []interface{}, got %T", dflt)
	require.Len(t, slice, 3)
	assert.Equal(t, 1, slice[0])
	assert.Equal(t, 2, slice[1])
	assert.Equal(t, 3, slice[2])
}

// TestParse_ArrayTypedFlag_MixedWithScalarFlags verifies that array-typed
// flags coexist with scalar-typed flags in the same tool without interference.
func TestParse_ArrayTypedFlag_MixedWithScalarFlags(t *testing.T) {
	input := `apiVersion: toolwright/v1
kind: Toolkit
metadata:
  name: mixed-flags
  version: 1.0.0
  description: Mixed scalar and array flags
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
      - name: tags
        type: "string[]"
        required: false
        default: ["x", "y"]
        description: Tags
      - name: limit
        type: int
        required: false
        default: 10
        description: Limit
`
	got, err := Parse(strings.NewReader(input))
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Len(t, got.Tools, 1)
	flags := got.Tools[0].Flags
	require.Len(t, flags, 3)

	// Scalar flags still work as before.
	assert.Equal(t, "bool", flags[0].Type)
	assert.Equal(t, false, flags[0].Default)
	assert.Equal(t, "int", flags[2].Type)
	assert.Equal(t, 10, flags[2].Default)

	// Array flag is correct.
	assert.Equal(t, "string[]", flags[1].Type)
	slice, ok := flags[1].Default.([]interface{})
	require.True(t, ok)
	require.Len(t, slice, 2)
	assert.Equal(t, "x", slice[0])
	assert.Equal(t, "y", slice[1])
}
