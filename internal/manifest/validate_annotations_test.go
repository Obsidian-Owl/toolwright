package manifest

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// AC3: Annotations are structurally valid — validation contract tests
//
// These tests establish the contract that:
//   - annotations with any combination of fields pass validation
//   - annotations are entirely optional (nil, empty, partial all pass)
//   - no field within annotations is required
//   - annotations do not interfere with other validation rules
//
// Current state: Validate() does not inspect annotations at all, so these
// all pass today. They serve as regression guards — if someone adds required-
// field validation on any annotation field, these tests will catch it.
// ---------------------------------------------------------------------------

// validToolkitWithAnnotations returns a valid Toolkit whose single tool has
// the given ToolAnnotations. Callers can pass nil for no annotations.
func validToolkitWithAnnotations(annot *ToolAnnotations) *Toolkit {
	tk := validToolkit()
	tk.Tools[0].Annotations = annot
	return tk
}

// onlyErrors filters a ValidationError slice to only severity "error",
// excluding warnings. This is important because some tools may produce
// warnings (e.g., missing itemSchema on object flags) that are not
// validation failures.
func onlyErrors(errs []ValidationError) []ValidationError {
	var out []ValidationError
	for _, e := range errs {
		if e.Severity == SeverityError {
			out = append(out, e)
		}
	}
	return out
}

// ---------------------------------------------------------------------------
// Table-driven: annotation combinations that must all pass validation
// ---------------------------------------------------------------------------

func TestValidateAnnotations_AllCombinationsPass(t *testing.T) {
	tests := []struct {
		name  string
		annot *ToolAnnotations
	}{
		{
			name:  "nil annotations (no annotations field)",
			annot: nil,
		},
		{
			name:  "empty annotations (zero-value struct)",
			annot: &ToolAnnotations{},
		},
		{
			name: "all fields set to true",
			annot: &ToolAnnotations{
				ReadOnly:    BoolPtr(true),
				Destructive: BoolPtr(true),
				Idempotent:  BoolPtr(true),
				OpenWorld:   BoolPtr(true),
				Title:       "Full Annotations",
			},
		},
		{
			name: "all bools false",
			annot: &ToolAnnotations{
				ReadOnly:    BoolPtr(false),
				Destructive: BoolPtr(false),
				Idempotent:  BoolPtr(false),
				OpenWorld:   BoolPtr(false),
			},
		},
		{
			name: "only title set",
			annot: &ToolAnnotations{
				Title: "Just a Title",
			},
		},
		{
			name: "only readOnly true",
			annot: &ToolAnnotations{
				ReadOnly: BoolPtr(true),
			},
		},
		{
			name: "only destructive false",
			annot: &ToolAnnotations{
				Destructive: BoolPtr(false),
			},
		},
		{
			name: "only idempotent true",
			annot: &ToolAnnotations{
				Idempotent: BoolPtr(true),
			},
		},
		{
			name: "only openWorld false",
			annot: &ToolAnnotations{
				OpenWorld: BoolPtr(false),
			},
		},
		{
			name: "readOnly and destructive only",
			annot: &ToolAnnotations{
				ReadOnly:    BoolPtr(true),
				Destructive: BoolPtr(false),
			},
		},
		{
			name: "idempotent and title only",
			annot: &ToolAnnotations{
				Idempotent: BoolPtr(true),
				Title:      "Idempotent Operation",
			},
		},
		{
			name: "all bools true with title",
			annot: &ToolAnnotations{
				ReadOnly:    BoolPtr(true),
				Destructive: BoolPtr(true),
				Idempotent:  BoolPtr(true),
				OpenWorld:   BoolPtr(true),
				Title:       "Everything True",
			},
		},
		{
			name: "all bools false with title",
			annot: &ToolAnnotations{
				ReadOnly:    BoolPtr(false),
				Destructive: BoolPtr(false),
				Idempotent:  BoolPtr(false),
				OpenWorld:   BoolPtr(false),
				Title:       "Everything False",
			},
		},
		{
			name: "mixed true and false with title",
			annot: &ToolAnnotations{
				ReadOnly:    BoolPtr(true),
				Destructive: BoolPtr(false),
				Idempotent:  BoolPtr(true),
				OpenWorld:   BoolPtr(false),
				Title:       "Mixed Bools",
			},
		},
		{
			name: "title with special characters",
			annot: &ToolAnnotations{
				Title: "Tool: List Files (read-only) — v2",
			},
		},
		{
			name: "empty title string is valid",
			annot: &ToolAnnotations{
				ReadOnly: BoolPtr(true),
				Title:    "",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tk := validToolkitWithAnnotations(tc.annot)
			errs := Validate(tk)
			errors := onlyErrors(errs)
			assert.Empty(t, errors,
				"Tool with annotations %+v must pass validation with zero errors, got: %v",
				tc.annot, errors)
		})
	}
}

// ---------------------------------------------------------------------------
// Annotations must not interfere with other validation rules
// ---------------------------------------------------------------------------

func TestValidateAnnotations_DoNotMaskOtherErrors(t *testing.T) {
	// A tool with valid annotations but an invalid toolkit-level field must
	// still fail on the invalid field. Annotations must not short-circuit
	// or suppress other validation.
	tk := validToolkitWithAnnotations(&ToolAnnotations{
		ReadOnly:    BoolPtr(true),
		Destructive: BoolPtr(false),
		Idempotent:  BoolPtr(true),
		OpenWorld:   BoolPtr(false),
		Title:       "Valid Annotations",
	})
	// Break the metadata name to trigger a known validation error.
	tk.Metadata.Name = ""

	errs := Validate(tk)
	require.NotEmpty(t, errs, "Invalid metadata.name must still produce errors even with valid annotations")

	ve := findError(errs, "metadata.name", "name-format")
	require.NotNil(t, ve,
		"metadata.name error must be present despite valid annotations; got errors: %v",
		errPaths(errs))
	assert.Equal(t, SeverityError, ve.Severity)
}

func TestValidateAnnotations_DoNotMaskVersionError(t *testing.T) {
	tk := validToolkitWithAnnotations(&ToolAnnotations{
		ReadOnly: BoolPtr(true),
		Title:    "Has Title",
	})
	tk.Metadata.Version = "not-semver"

	errs := Validate(tk)
	ve := findError(errs, "metadata.version", "semver")
	require.NotNil(t, ve,
		"metadata.version error must still be reported when tool has annotations; got errors: %v",
		errPaths(errs))
}

func TestValidateAnnotations_DoNotMaskDescriptionError(t *testing.T) {
	tk := validToolkitWithAnnotations(&ToolAnnotations{
		Destructive: BoolPtr(true),
	})
	tk.Metadata.Description = ""

	errs := Validate(tk)
	ve := findError(errs, "metadata.description", "description-required")
	require.NotNil(t, ve,
		"metadata.description error must still be reported when tool has annotations; got errors: %v",
		errPaths(errs))
}

// ---------------------------------------------------------------------------
// Multiple tools with mixed annotations
// ---------------------------------------------------------------------------

func TestValidateAnnotations_MultipleToolsMixedAnnotations(t *testing.T) {
	tk := validToolkit()
	tk.Tools = []Tool{
		{
			Name:        "tool-annotated-full",
			Description: "Has full annotations",
			Entrypoint:  "./a.sh",
			Annotations: &ToolAnnotations{
				ReadOnly:    BoolPtr(true),
				Destructive: BoolPtr(false),
				Idempotent:  BoolPtr(true),
				OpenWorld:   BoolPtr(false),
				Title:       "Annotated Tool",
			},
		},
		{
			Name:        "tool-no-annotations",
			Description: "Has no annotations at all",
			Entrypoint:  "./b.sh",
			Annotations: nil,
		},
		{
			Name:        "tool-empty-annotations",
			Description: "Has empty annotations block",
			Entrypoint:  "./c.sh",
			Annotations: &ToolAnnotations{},
		},
		{
			Name:        "tool-partial-annotations",
			Description: "Has only destructive set",
			Entrypoint:  "./d.sh",
			Annotations: &ToolAnnotations{
				Destructive: BoolPtr(true),
			},
		},
		{
			Name:        "tool-title-only",
			Description: "Has only title",
			Entrypoint:  "./e.sh",
			Annotations: &ToolAnnotations{
				Title: "Title Only Tool",
			},
		},
	}

	errs := Validate(tk)
	errors := onlyErrors(errs)
	assert.Empty(t, errors,
		"Five tools with mixed annotation states must all pass validation with zero errors, got: %v",
		errors)
}

// ---------------------------------------------------------------------------
// Guard: no annotation path should appear in validation errors
// ---------------------------------------------------------------------------

func TestValidateAnnotations_NoAnnotationPathInErrors(t *testing.T) {
	// Even with all annotation combinations, no validation error should ever
	// have a path containing "annotations". This guards against someone
	// accidentally adding required-field checks on annotation sub-fields.
	toolkits := []*Toolkit{
		validToolkitWithAnnotations(nil),
		validToolkitWithAnnotations(&ToolAnnotations{}),
		validToolkitWithAnnotations(&ToolAnnotations{
			ReadOnly:    BoolPtr(true),
			Destructive: BoolPtr(false),
			Idempotent:  BoolPtr(true),
			OpenWorld:   BoolPtr(false),
			Title:       "Full",
		}),
		validToolkitWithAnnotations(&ToolAnnotations{
			ReadOnly: BoolPtr(false),
		}),
		validToolkitWithAnnotations(&ToolAnnotations{
			Title: "Just Title",
		}),
	}

	for i, tk := range toolkits {
		errs := Validate(tk)
		for _, ve := range errs {
			if ve.Severity == SeverityError {
				assert.NotContains(t, ve.Path, "annotations",
					"Toolkit[%d]: no validation error should reference an annotations path, but got path=%q rule=%q message=%q",
					i, ve.Path, ve.Rule, ve.Message)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// Guard: annotation presence does not change the error count for valid tools
// ---------------------------------------------------------------------------

func TestValidateAnnotations_ErrorCountUnchangedByAnnotations(t *testing.T) {
	// Validate the same toolkit with and without annotations.
	// The error count must be identical — annotations must not add or remove
	// any errors. This catches both false-positive (annotations cause errors)
	// and false-negative (annotations suppress other errors) regressions.

	tkWithout := validToolkit()
	tkWithout.Tools[0].Annotations = nil

	tkWith := validToolkit()
	tkWith.Tools[0].Annotations = &ToolAnnotations{
		ReadOnly:    BoolPtr(true),
		Destructive: BoolPtr(false),
		Idempotent:  BoolPtr(true),
		OpenWorld:   BoolPtr(false),
		Title:       "Some Title",
	}

	errsWithout := onlyErrors(Validate(tkWithout))
	errsWith := onlyErrors(Validate(tkWith))

	assert.Equal(t, len(errsWithout), len(errsWith),
		"Adding annotations to a valid tool must not change validation error count; without=%d, with=%d\nwithout errors: %v\nwith errors: %v",
		len(errsWithout), len(errsWith), errsWithout, errsWith)
}

func TestValidateAnnotations_ErrorCountUnchangedForInvalidToolkit(t *testing.T) {
	// Same as above but with an INVALID toolkit (bad metadata).
	// Annotations must not change the error count even when errors exist.

	tkWithout := validToolkit()
	tkWithout.Metadata.Name = "INVALID"       // uppercase not allowed
	tkWithout.Metadata.Version = "not-semver" // invalid version
	tkWithout.Tools[0].Annotations = nil

	tkWith := validToolkit()
	tkWith.Metadata.Name = "INVALID"
	tkWith.Metadata.Version = "not-semver"
	tkWith.Tools[0].Annotations = &ToolAnnotations{
		ReadOnly:    BoolPtr(true),
		Destructive: BoolPtr(false),
		Title:       "Still has annotations",
	}

	errsWithout := Validate(tkWithout)
	errsWith := Validate(tkWith)

	assert.Equal(t, len(errsWithout), len(errsWith),
		"Annotations must not change error count for invalid toolkit; without=%d, with=%d\nwithout: %v\nwith: %v",
		len(errsWithout), len(errsWith), errsWithout, errsWith)

	// Both must have at least the two known errors.
	require.GreaterOrEqual(t, len(errsWithout), 2,
		"Invalid toolkit must have at least 2 errors (name + version)")
}
