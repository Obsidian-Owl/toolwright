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
// AC2: BoolPtr helper
// ---------------------------------------------------------------------------

func TestAnnotations_BoolPtr_True(t *testing.T) {
	got := BoolPtr(true)
	require.NotNil(t, got, "BoolPtr(true) must return non-nil pointer")
	assert.True(t, *got, "BoolPtr(true) must point to true")
}

func TestAnnotations_BoolPtr_False(t *testing.T) {
	got := BoolPtr(false)
	require.NotNil(t, got, "BoolPtr(false) must return non-nil pointer")
	assert.False(t, *got, "BoolPtr(false) must point to false")
}

func TestAnnotations_BoolPtr_ReturnsDistinctPointers(t *testing.T) {
	// Each call must return a new pointer, not a shared singleton.
	// A lazy impl that returns &globalTrue would cause mutations to leak.
	a := BoolPtr(true)
	b := BoolPtr(true)
	require.NotNil(t, a)
	require.NotNil(t, b)

	// Verify they point to the same value but are different pointers.
	assert.True(t, *a)
	assert.True(t, *b)
	if a == b {
		t.Error("BoolPtr must return distinct pointers on each call, got same address")
	}

	// Mutating one must not affect the other.
	*a = false
	assert.True(t, *b, "Mutating one BoolPtr result must not affect another")
}

// ---------------------------------------------------------------------------
// AC1: Parsing annotations from YAML manifests
// ---------------------------------------------------------------------------

// annotationManifestPrefix is the boilerplate for a minimal valid manifest.
// Tests append tool definitions with varying annotation configurations.
const annotationManifestPrefix = `apiVersion: toolwright/v1
kind: Toolkit
metadata:
  name: annot-test
  version: 1.0.0
  description: Annotation parsing tests
tools:
`

func TestAnnotations_Parse_FullAnnotations(t *testing.T) {
	input := annotationManifestPrefix + `  - name: full-annot
    description: Tool with all annotations
    entrypoint: ./tool.sh
    annotations:
      readOnly: true
      destructive: false
      idempotent: true
      openWorld: false
      title: "My Tool Title"
`
	got, err := Parse(strings.NewReader(input))
	require.NoError(t, err, "Parse must succeed for manifest with full annotations")
	require.NotNil(t, got)
	require.Len(t, got.Tools, 1)

	annot := got.Tools[0].Annotations
	require.NotNil(t, annot, "Annotations must not be nil when annotations block is present")

	// Verify each *bool field points to the correct value.
	require.NotNil(t, annot.ReadOnly, "ReadOnly must not be nil when set to true")
	assert.True(t, *annot.ReadOnly, "ReadOnly must be true")

	require.NotNil(t, annot.Destructive, "Destructive must not be nil when set to false")
	assert.False(t, *annot.Destructive, "Destructive must be false")

	require.NotNil(t, annot.Idempotent, "Idempotent must not be nil when set to true")
	assert.True(t, *annot.Idempotent, "Idempotent must be true")

	require.NotNil(t, annot.OpenWorld, "OpenWorld must not be nil when set to false")
	assert.False(t, *annot.OpenWorld, "OpenWorld must be false")

	assert.Equal(t, "My Tool Title", annot.Title, "Title must match the YAML value")
}

func TestAnnotations_Parse_PartialAnnotations(t *testing.T) {
	input := annotationManifestPrefix + `  - name: partial-annot
    description: Tool with only readOnly
    entrypoint: ./tool.sh
    annotations:
      readOnly: true
`
	got, err := Parse(strings.NewReader(input))
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Len(t, got.Tools, 1)

	annot := got.Tools[0].Annotations
	require.NotNil(t, annot, "Annotations must not be nil when annotations block is present")

	require.NotNil(t, annot.ReadOnly, "ReadOnly must not be nil when explicitly set")
	assert.True(t, *annot.ReadOnly, "ReadOnly must be true")

	// All other *bool fields must be nil, not &false.
	assert.Nil(t, annot.Destructive, "Destructive must be nil when not specified")
	assert.Nil(t, annot.Idempotent, "Idempotent must be nil when not specified")
	assert.Nil(t, annot.OpenWorld, "OpenWorld must be nil when not specified")

	// Title must be empty string (zero value), not some default.
	assert.Equal(t, "", annot.Title, "Title must be empty when not specified")
}

func TestAnnotations_Parse_NoAnnotations(t *testing.T) {
	input := annotationManifestPrefix + `  - name: no-annot
    description: Tool without annotations
    entrypoint: ./tool.sh
`
	got, err := Parse(strings.NewReader(input))
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Len(t, got.Tools, 1)

	assert.Nil(t, got.Tools[0].Annotations,
		"Annotations must be nil when not specified in YAML")
}

func TestAnnotations_Parse_FalseValuesAreNotNil(t *testing.T) {
	// This is the critical *bool test: false must parse as &false, NOT nil.
	// A naive implementation using plain bool would lose this distinction.
	input := annotationManifestPrefix + `  - name: false-annot
    description: Tool with all false annotations
    entrypoint: ./tool.sh
    annotations:
      readOnly: false
      destructive: false
      idempotent: false
      openWorld: false
`
	got, err := Parse(strings.NewReader(input))
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Len(t, got.Tools, 1)

	annot := got.Tools[0].Annotations
	require.NotNil(t, annot, "Annotations must not be nil when annotations block is present")

	// Each field must be &false (non-nil pointer to false), not nil.
	require.NotNil(t, annot.ReadOnly,
		"ReadOnly set to false must parse as &false, not nil")
	assert.False(t, *annot.ReadOnly)

	require.NotNil(t, annot.Destructive,
		"Destructive set to false must parse as &false, not nil")
	assert.False(t, *annot.Destructive)

	require.NotNil(t, annot.Idempotent,
		"Idempotent set to false must parse as &false, not nil")
	assert.False(t, *annot.Idempotent)

	require.NotNil(t, annot.OpenWorld,
		"OpenWorld set to false must parse as &false, not nil")
	assert.False(t, *annot.OpenWorld)
}

func TestAnnotations_Parse_OnlyTitle(t *testing.T) {
	input := annotationManifestPrefix + `  - name: title-only
    description: Tool with only title
    entrypoint: ./tool.sh
    annotations:
      title: "Human-Readable Name"
`
	got, err := Parse(strings.NewReader(input))
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Len(t, got.Tools, 1)

	annot := got.Tools[0].Annotations
	require.NotNil(t, annot, "Annotations must not be nil when title is set")

	assert.Equal(t, "Human-Readable Name", annot.Title)

	// All bool pointers must be nil when only title is set.
	assert.Nil(t, annot.ReadOnly, "ReadOnly must be nil when not specified")
	assert.Nil(t, annot.Destructive, "Destructive must be nil when not specified")
	assert.Nil(t, annot.Idempotent, "Idempotent must be nil when not specified")
	assert.Nil(t, annot.OpenWorld, "OpenWorld must be nil when not specified")
}

func TestAnnotations_Parse_EmptyAnnotationsBlock(t *testing.T) {
	// annotations: {} should parse as non-nil Annotations with all zero values.
	// This is distinct from annotations being absent entirely (nil pointer).
	input := annotationManifestPrefix + `  - name: empty-annot
    description: Tool with empty annotations
    entrypoint: ./tool.sh
    annotations: {}
`
	got, err := Parse(strings.NewReader(input))
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Len(t, got.Tools, 1)

	annot := got.Tools[0].Annotations
	require.NotNil(t, annot,
		"annotations: {} must parse as non-nil pointer (distinct from absent)")

	// All fields at zero values.
	assert.Nil(t, annot.ReadOnly, "ReadOnly must be nil in empty annotations")
	assert.Nil(t, annot.Destructive, "Destructive must be nil in empty annotations")
	assert.Nil(t, annot.Idempotent, "Idempotent must be nil in empty annotations")
	assert.Nil(t, annot.OpenWorld, "OpenWorld must be nil in empty annotations")
	assert.Equal(t, "", annot.Title, "Title must be empty in empty annotations")
}

// ---------------------------------------------------------------------------
// Table-driven: multiple annotation combinations
// ---------------------------------------------------------------------------

func TestAnnotations_Parse_Combinations(t *testing.T) {
	tests := []struct {
		name            string
		annotationYAML  string
		wantReadOnly    *bool
		wantDestructive *bool
		wantIdempotent  *bool
		wantOpenWorld   *bool
		wantTitle       string
	}{
		{
			name:           "readOnly true only",
			annotationYAML: "readOnly: true",
			wantReadOnly:   BoolPtr(true),
			wantTitle:      "",
		},
		{
			name:            "destructive false only",
			annotationYAML:  "destructive: false",
			wantDestructive: BoolPtr(false),
			wantTitle:       "",
		},
		{
			name:           "idempotent and openWorld mixed",
			annotationYAML: "idempotent: true\n      openWorld: false",
			wantIdempotent: BoolPtr(true),
			wantOpenWorld:  BoolPtr(false),
			wantTitle:      "",
		},
		{
			name:           "title with readOnly",
			annotationYAML: "readOnly: true\n      title: \"Search\"",
			wantReadOnly:   BoolPtr(true),
			wantTitle:      "Search",
		},
		{
			name:            "all fields set to true",
			annotationYAML:  "readOnly: true\n      destructive: true\n      idempotent: true\n      openWorld: true\n      title: \"All True\"",
			wantReadOnly:    BoolPtr(true),
			wantDestructive: BoolPtr(true),
			wantIdempotent:  BoolPtr(true),
			wantOpenWorld:   BoolPtr(true),
			wantTitle:       "All True",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			input := annotationManifestPrefix + `  - name: combo-test
    description: Combination test
    entrypoint: ./tool.sh
    annotations:
      ` + tc.annotationYAML + "\n"

			got, err := Parse(strings.NewReader(input))
			require.NoError(t, err, "Parse should succeed")
			require.NotNil(t, got)
			require.Len(t, got.Tools, 1)

			annot := got.Tools[0].Annotations
			require.NotNil(t, annot, "Annotations must not be nil")

			assertBoolPtrEqual(t, tc.wantReadOnly, annot.ReadOnly, "ReadOnly")
			assertBoolPtrEqual(t, tc.wantDestructive, annot.Destructive, "Destructive")
			assertBoolPtrEqual(t, tc.wantIdempotent, annot.Idempotent, "Idempotent")
			assertBoolPtrEqual(t, tc.wantOpenWorld, annot.OpenWorld, "OpenWorld")
			assert.Equal(t, tc.wantTitle, annot.Title)
		})
	}
}

// assertBoolPtrEqual compares two *bool values, checking both nil-ness and pointed-to value.
func assertBoolPtrEqual(t *testing.T, want, got *bool, fieldName string) {
	t.Helper()
	if want == nil {
		assert.Nil(t, got, "%s: expected nil, got %v", fieldName, got)
		return
	}
	require.NotNil(t, got, "%s: expected &%v, got nil", fieldName, *want)
	assert.Equal(t, *want, *got, "%s: expected %v, got %v", fieldName, *want, *got)
}

// ---------------------------------------------------------------------------
// Annotations on existing tools -- backward compat
// ---------------------------------------------------------------------------

func TestAnnotations_Parse_ExistingToolFieldsPreserved(t *testing.T) {
	// Annotations must not interfere with other Tool fields.
	input := annotationManifestPrefix + `  - name: full-tool
    description: Tool with everything
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
      format: json
    annotations:
      readOnly: true
      title: "Full Tool"
    exit_codes:
      0: success
      1: failure
`
	got, err := Parse(strings.NewReader(input))
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Len(t, got.Tools, 1)

	tool := got.Tools[0]
	assert.Equal(t, "full-tool", tool.Name)
	assert.Equal(t, "Tool with everything", tool.Description)
	assert.Equal(t, "./run.sh", tool.Entrypoint)
	require.Len(t, tool.Args, 1)
	assert.Equal(t, "input", tool.Args[0].Name)
	require.Len(t, tool.Flags, 1)
	assert.Equal(t, "verbose", tool.Flags[0].Name)
	assert.Equal(t, "json", tool.Output.Format)
	require.Len(t, tool.ExitCodes, 2)
	assert.Equal(t, "success", tool.ExitCodes[0])

	// And annotations are also present.
	require.NotNil(t, tool.Annotations)
	require.NotNil(t, tool.Annotations.ReadOnly)
	assert.True(t, *tool.Annotations.ReadOnly)
	assert.Equal(t, "Full Tool", tool.Annotations.Title)
}

func TestAnnotations_Parse_MultipleToolsMixedAnnotations(t *testing.T) {
	// First tool has annotations, second does not. Verify independence.
	input := annotationManifestPrefix + `  - name: annotated
    description: Has annotations
    entrypoint: ./a.sh
    annotations:
      readOnly: true
      destructive: false
  - name: plain
    description: No annotations
    entrypoint: ./b.sh
`
	got, err := Parse(strings.NewReader(input))
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Len(t, got.Tools, 2)

	// First tool: has annotations.
	require.NotNil(t, got.Tools[0].Annotations)
	require.NotNil(t, got.Tools[0].Annotations.ReadOnly)
	assert.True(t, *got.Tools[0].Annotations.ReadOnly)
	require.NotNil(t, got.Tools[0].Annotations.Destructive)
	assert.False(t, *got.Tools[0].Annotations.Destructive)

	// Second tool: no annotations.
	assert.Nil(t, got.Tools[1].Annotations,
		"Second tool must have nil Annotations when not specified")
}

// ---------------------------------------------------------------------------
// YAML tag correctness
// ---------------------------------------------------------------------------

func TestAnnotations_Parse_YAMLTagIsAnnotations(t *testing.T) {
	// Only the exact YAML key "annotations" should map to the Annotations field.
	// Wrong keys must not populate the field.
	wrongKeys := []struct {
		name    string
		yamlKey string
	}{
		{name: "PascalCase Annotations", yamlKey: "Annotations"},
		{name: "snake_case annotations_", yamlKey: "annotations_"},
		{name: "singular annotation", yamlKey: "annotation"},
		{name: "uppercase ANNOTATIONS", yamlKey: "ANNOTATIONS"},
	}

	for _, tc := range wrongKeys {
		t.Run(tc.name, func(t *testing.T) {
			input := annotationManifestPrefix + `  - name: tag-test
    description: YAML tag test
    entrypoint: ./tool.sh
    ` + tc.yamlKey + `:
      readOnly: true
`
			got, err := Parse(strings.NewReader(input))
			require.NoError(t, err)
			require.NotNil(t, got)
			require.Len(t, got.Tools, 1)
			assert.Nil(t, got.Tools[0].Annotations,
				"Annotations must be nil when YAML key is %q (only 'annotations' should work)",
				tc.yamlKey)
		})
	}
}

func TestAnnotations_Parse_BoolFieldYAMLTags(t *testing.T) {
	// Verify the exact camelCase YAML keys for each bool field.
	// A wrong tag (e.g., "read_only" instead of "readOnly") would
	// silently drop the field.
	tests := []struct {
		name    string
		yamlKey string
		checkFn func(*testing.T, *ToolAnnotations)
	}{
		{
			name:    "readOnly key",
			yamlKey: "readOnly",
			checkFn: func(t *testing.T, a *ToolAnnotations) {
				t.Helper()
				require.NotNil(t, a.ReadOnly, "readOnly key must map to ReadOnly field")
				assert.True(t, *a.ReadOnly)
			},
		},
		{
			name:    "destructive key",
			yamlKey: "destructive",
			checkFn: func(t *testing.T, a *ToolAnnotations) {
				t.Helper()
				require.NotNil(t, a.Destructive, "destructive key must map to Destructive field")
				assert.True(t, *a.Destructive)
			},
		},
		{
			name:    "idempotent key",
			yamlKey: "idempotent",
			checkFn: func(t *testing.T, a *ToolAnnotations) {
				t.Helper()
				require.NotNil(t, a.Idempotent, "idempotent key must map to Idempotent field")
				assert.True(t, *a.Idempotent)
			},
		},
		{
			name:    "openWorld key",
			yamlKey: "openWorld",
			checkFn: func(t *testing.T, a *ToolAnnotations) {
				t.Helper()
				require.NotNil(t, a.OpenWorld, "openWorld key must map to OpenWorld field")
				assert.True(t, *a.OpenWorld)
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			input := annotationManifestPrefix + `  - name: tag-test
    description: Bool tag test
    entrypoint: ./tool.sh
    annotations:
      ` + tc.yamlKey + `: true
`
			got, err := Parse(strings.NewReader(input))
			require.NoError(t, err)
			require.NotNil(t, got)
			require.Len(t, got.Tools, 1)

			annot := got.Tools[0].Annotations
			require.NotNil(t, annot, "Annotations must not be nil")
			tc.checkFn(t, annot)
		})
	}
}

func TestAnnotations_Parse_WrongBoolKeyNames(t *testing.T) {
	// Snake_case or other wrong key names must NOT populate the bool fields.
	wrongKeys := []string{
		"read_only",
		"ReadOnly",
		"readonly",
		"is_read_only",
	}

	for _, key := range wrongKeys {
		t.Run(key, func(t *testing.T) {
			input := annotationManifestPrefix + `  - name: wrong-key
    description: Wrong bool key
    entrypoint: ./tool.sh
    annotations:
      ` + key + `: true
`
			got, err := Parse(strings.NewReader(input))
			require.NoError(t, err)
			require.NotNil(t, got)
			require.Len(t, got.Tools, 1)

			annot := got.Tools[0].Annotations
			// The annotations block IS present (it has content), so annot may
			// or may not be nil depending on how the YAML library handles
			// unknown keys. But ReadOnly must definitely be nil.
			if annot != nil {
				assert.Nil(t, annot.ReadOnly,
					"ReadOnly must be nil when YAML key is %q (only 'readOnly' should work)", key)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Round-trip: annotations survive marshal/unmarshal
// ---------------------------------------------------------------------------

func TestAnnotations_RoundTrip_FullAnnotations(t *testing.T) {
	input := annotationManifestPrefix + `  - name: rt-test
    description: Round-trip test
    entrypoint: ./tool.sh
    annotations:
      readOnly: true
      destructive: false
      idempotent: true
      openWorld: false
      title: "Round Trip"
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

	// Explicit checks to ensure *bool survives and isn't flattened.
	annot := roundTripped.Tools[0].Annotations
	require.NotNil(t, annot)

	require.NotNil(t, annot.ReadOnly)
	assert.True(t, *annot.ReadOnly)

	require.NotNil(t, annot.Destructive)
	assert.False(t, *annot.Destructive)

	require.NotNil(t, annot.Idempotent)
	assert.True(t, *annot.Idempotent)

	require.NotNil(t, annot.OpenWorld)
	assert.False(t, *annot.OpenWorld)

	assert.Equal(t, "Round Trip", annot.Title)
}

func TestAnnotations_RoundTrip_NoAnnotations(t *testing.T) {
	input := annotationManifestPrefix + `  - name: rt-no-annot
    description: Plain tool round-trip
    entrypoint: ./tool.sh
`
	original, err := Parse(strings.NewReader(input))
	require.NoError(t, err)

	marshalled, err := yaml.Marshal(original)
	require.NoError(t, err)

	// The marshalled YAML should NOT contain "annotations" at all.
	assert.NotContains(t, string(marshalled), "annotations",
		"Marshalled YAML must not contain 'annotations' when Annotations is nil (omitempty)")

	roundTripped, err := Parse(strings.NewReader(string(marshalled)))
	require.NoError(t, err)
	require.Len(t, roundTripped.Tools, 1)
	assert.Nil(t, roundTripped.Tools[0].Annotations,
		"Annotations must remain nil after round-trip")
}

func TestAnnotations_RoundTrip_PartialAnnotations(t *testing.T) {
	input := annotationManifestPrefix + `  - name: rt-partial
    description: Partial annotations round-trip
    entrypoint: ./tool.sh
    annotations:
      destructive: true
`
	original, err := Parse(strings.NewReader(input))
	require.NoError(t, err)

	marshalled, err := yaml.Marshal(original)
	require.NoError(t, err)

	roundTripped, err := Parse(strings.NewReader(string(marshalled)))
	require.NoError(t, err)

	annot := roundTripped.Tools[0].Annotations
	require.NotNil(t, annot, "Partial annotations must survive round-trip")

	require.NotNil(t, annot.Destructive)
	assert.True(t, *annot.Destructive)

	// Unset fields must remain nil after round-trip (omitempty must work).
	assert.Nil(t, annot.ReadOnly, "ReadOnly must remain nil after round-trip")
	assert.Nil(t, annot.Idempotent, "Idempotent must remain nil after round-trip")
	assert.Nil(t, annot.OpenWorld, "OpenWorld must remain nil after round-trip")
	assert.Equal(t, "", annot.Title, "Title must remain empty after round-trip")
}

func TestAnnotations_RoundTrip_FalsePreserved(t *testing.T) {
	// Critical: *bool false must survive round-trip as &false, not become nil.
	input := annotationManifestPrefix + `  - name: rt-false
    description: False round-trip
    entrypoint: ./tool.sh
    annotations:
      readOnly: false
`
	original, err := Parse(strings.NewReader(input))
	require.NoError(t, err)
	require.NotNil(t, original.Tools[0].Annotations)
	require.NotNil(t, original.Tools[0].Annotations.ReadOnly,
		"Pre-condition: ReadOnly must be &false, not nil")
	assert.False(t, *original.Tools[0].Annotations.ReadOnly)

	marshalled, err := yaml.Marshal(original)
	require.NoError(t, err)

	// The marshalled YAML must contain "readOnly: false" -- if omitempty
	// incorrectly treats &false as empty, it will be dropped.
	assert.Contains(t, string(marshalled), "readOnly: false",
		"Marshalled YAML must preserve readOnly: false (not omit it)")

	roundTripped, err := Parse(strings.NewReader(string(marshalled)))
	require.NoError(t, err)

	annot := roundTripped.Tools[0].Annotations
	require.NotNil(t, annot, "Annotations must survive round-trip")
	require.NotNil(t, annot.ReadOnly,
		"ReadOnly: false must round-trip as &false, not nil")
	assert.False(t, *annot.ReadOnly,
		"ReadOnly must be false after round-trip")
}

// ---------------------------------------------------------------------------
// Adversarial: ToolAnnotations struct field types
// ---------------------------------------------------------------------------

func TestAnnotations_StructHasCorrectFieldTypes(t *testing.T) {
	// Verify the ToolAnnotations struct exists with the expected fields
	// by constructing one directly. This catches missing fields or wrong types
	// at compile time via the test itself.
	annot := ToolAnnotations{
		ReadOnly:    BoolPtr(true),
		Destructive: BoolPtr(false),
		Idempotent:  BoolPtr(true),
		OpenWorld:   BoolPtr(false),
		Title:       "Test",
	}

	// Verify we can read back each field with the correct type.
	require.NotNil(t, annot.ReadOnly)
	assert.True(t, *annot.ReadOnly)

	require.NotNil(t, annot.Destructive)
	assert.False(t, *annot.Destructive)

	require.NotNil(t, annot.Idempotent)
	assert.True(t, *annot.Idempotent)

	require.NotNil(t, annot.OpenWorld)
	assert.False(t, *annot.OpenWorld)

	assert.Equal(t, "Test", annot.Title)
}

func TestAnnotations_ToolHasAnnotationsField(t *testing.T) {
	// Verify Tool struct has an Annotations field of type *ToolAnnotations.
	tool := Tool{
		Name:        "test",
		Description: "test",
		Entrypoint:  "./test.sh",
		Annotations: &ToolAnnotations{
			ReadOnly: BoolPtr(true),
			Title:    "Test Tool",
		},
	}

	require.NotNil(t, tool.Annotations)
	require.NotNil(t, tool.Annotations.ReadOnly)
	assert.True(t, *tool.Annotations.ReadOnly)
	assert.Equal(t, "Test Tool", tool.Annotations.Title)
}

func TestAnnotations_ToolAnnotationsNilByDefault(t *testing.T) {
	// A zero-value Tool must have nil Annotations.
	tool := Tool{}
	assert.Nil(t, tool.Annotations,
		"Zero-value Tool must have nil Annotations (pointer type)")
}

// ---------------------------------------------------------------------------
// Adversarial: annotations inline form { readOnly: true }
// ---------------------------------------------------------------------------

func TestAnnotations_Parse_InlineForm(t *testing.T) {
	// YAML inline/flow form: annotations: { readOnly: true }
	input := annotationManifestPrefix + `  - name: inline-annot
    description: Inline annotations
    entrypoint: ./tool.sh
    annotations: { readOnly: true, destructive: false, title: "Inline" }
`
	got, err := Parse(strings.NewReader(input))
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Len(t, got.Tools, 1)

	annot := got.Tools[0].Annotations
	require.NotNil(t, annot)

	require.NotNil(t, annot.ReadOnly)
	assert.True(t, *annot.ReadOnly)

	require.NotNil(t, annot.Destructive)
	assert.False(t, *annot.Destructive)

	assert.Equal(t, "Inline", annot.Title)
}
