package manifest

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Test helpers for resource validation
// ---------------------------------------------------------------------------

// validToolkitWithResources returns a valid Toolkit with the given resources.
// The toolkit has valid metadata and one valid tool so that resource validation
// errors are the only defects present.
func validToolkitWithResources(resources ...Resource) *Toolkit {
	tk := validToolkit()
	tk.Resources = resources
	return tk
}

// validResource returns a minimal but fully valid Resource.
func validResource() Resource {
	return Resource{
		URI:         "file:///data/report.csv",
		Name:        "report-data",
		Description: "Monthly report data",
		MimeType:    "text/csv",
		Entrypoint:  "./fetch-report.sh",
	}
}

// countErrorsByRule counts how many ValidationErrors have the given rule.
func countErrorsByRule(errs []ValidationError, rule string) int {
	n := 0
	for _, e := range errs {
		if e.Rule == rule {
			n++
		}
	}
	return n
}

// ---------------------------------------------------------------------------
// AC3: Required resource fields
//
// Missing URI → "resource-uri-required"
// Missing Name → "resource-name-required"
// Missing Entrypoint → "resource-entrypoint-required"
// ---------------------------------------------------------------------------

func TestValidateResource_MissingURI(t *testing.T) {
	r := validResource()
	r.URI = ""
	tk := validToolkitWithResources(r)

	errs := Validate(tk)
	ve := findErrorByRule(errs, "resource-uri-required")
	require.NotNil(t, ve,
		"resource with empty URI must emit 'resource-uri-required'; got rules: %v",
		errRules(errs))
	assert.Equal(t, SeverityError, ve.Severity)
	assert.Equal(t, "resources[0].uri", ve.Path,
		"error path must be 'resources[0].uri', got %q", ve.Path)
	assert.NotEmpty(t, ve.Message, "error message must not be empty")
}

func TestValidateResource_MissingName(t *testing.T) {
	r := validResource()
	r.Name = ""
	tk := validToolkitWithResources(r)

	errs := Validate(tk)
	ve := findErrorByRule(errs, "resource-name-required")
	require.NotNil(t, ve,
		"resource with empty Name must emit 'resource-name-required'; got rules: %v",
		errRules(errs))
	assert.Equal(t, SeverityError, ve.Severity)
	assert.Equal(t, "resources[0].name", ve.Path,
		"error path must be 'resources[0].name', got %q", ve.Path)
	assert.NotEmpty(t, ve.Message)
}

func TestValidateResource_MissingEntrypoint(t *testing.T) {
	r := validResource()
	r.Entrypoint = ""
	tk := validToolkitWithResources(r)

	errs := Validate(tk)
	ve := findErrorByRule(errs, "resource-entrypoint-required")
	require.NotNil(t, ve,
		"resource with empty Entrypoint must emit 'resource-entrypoint-required'; got rules: %v",
		errRules(errs))
	assert.Equal(t, SeverityError, ve.Severity)
	assert.Equal(t, "resources[0].entrypoint", ve.Path,
		"error path must be 'resources[0].entrypoint', got %q", ve.Path)
	assert.NotEmpty(t, ve.Message)
}

// All three required fields missing at once must produce three separate errors.
func TestValidateResource_AllRequiredFieldsMissing(t *testing.T) {
	tk := validToolkitWithResources(Resource{
		Description: "only description set",
		MimeType:    "text/plain",
	})

	errs := Validate(tk)

	uriErr := findErrorByRule(errs, "resource-uri-required")
	require.NotNil(t, uriErr,
		"must report 'resource-uri-required' when URI missing; got rules: %v", errRules(errs))

	nameErr := findErrorByRule(errs, "resource-name-required")
	require.NotNil(t, nameErr,
		"must report 'resource-name-required' when Name missing; got rules: %v", errRules(errs))

	entrypointErr := findErrorByRule(errs, "resource-entrypoint-required")
	require.NotNil(t, entrypointErr,
		"must report 'resource-entrypoint-required' when Entrypoint missing; got rules: %v", errRules(errs))

	// Verify we got exactly one of each (not fail-fast after the first).
	assert.Equal(t, 1, countErrorsByRule(errs, "resource-uri-required"),
		"should have exactly one 'resource-uri-required' error")
	assert.Equal(t, 1, countErrorsByRule(errs, "resource-name-required"),
		"should have exactly one 'resource-name-required' error")
	assert.Equal(t, 1, countErrorsByRule(errs, "resource-entrypoint-required"),
		"should have exactly one 'resource-entrypoint-required' error")
}

// ---------------------------------------------------------------------------
// AC3: Whitespace-only required fields should be treated as missing
// ---------------------------------------------------------------------------

func TestValidateResource_WhitespaceOnlyFields(t *testing.T) {
	tests := []struct {
		name     string
		resource Resource
		rule     string
		path     string
	}{
		{
			name: "whitespace-only URI",
			resource: Resource{
				URI:        "   \t ",
				Name:       "valid-name",
				Entrypoint: "./fetch.sh",
			},
			rule: "resource-uri-required",
			path: "resources[0].uri",
		},
		{
			name: "whitespace-only Name",
			resource: Resource{
				URI:        "file:///data.csv",
				Name:       "  \n  ",
				Entrypoint: "./fetch.sh",
			},
			rule: "resource-name-required",
			path: "resources[0].name",
		},
		{
			name: "whitespace-only Entrypoint",
			resource: Resource{
				URI:        "file:///data.csv",
				Name:       "valid-name",
				Entrypoint: "   ",
			},
			rule: "resource-entrypoint-required",
			path: "resources[0].entrypoint",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tk := validToolkitWithResources(tc.resource)
			errs := Validate(tk)
			ve := findErrorByRule(errs, tc.rule)
			require.NotNil(t, ve,
				"whitespace-only field must trigger %q; got rules: %v",
				tc.rule, errRules(errs))
			assert.Equal(t, SeverityError, ve.Severity)
			assert.Equal(t, tc.path, ve.Path,
				"error path must be %q, got %q", tc.path, ve.Path)
		})
	}
}

// ---------------------------------------------------------------------------
// AC3: Required field errors use the correct index in multi-resource toolkit
// ---------------------------------------------------------------------------

func TestValidateResource_ErrorReferencesCorrectIndex(t *testing.T) {
	good := validResource()
	bad := Resource{
		URI:         "",
		Name:        "second-resource",
		Description: "second resource missing URI",
		Entrypoint:  "./second.sh",
	}
	tk := validToolkitWithResources(good, bad)

	errs := Validate(tk)
	ve := findErrorByRule(errs, "resource-uri-required")
	require.NotNil(t, ve,
		"second resource's missing URI must be reported; got rules: %v", errRules(errs))
	assert.Equal(t, "resources[1].uri", ve.Path,
		"error path must reference resources[1] (the second resource), got %q", ve.Path)
}

// ---------------------------------------------------------------------------
// AC4: Duplicate resource names rejected
// ---------------------------------------------------------------------------

func TestValidateResource_DuplicateNames(t *testing.T) {
	r1 := Resource{
		URI:        "file:///alpha.csv",
		Name:       "shared-name",
		Entrypoint: "./alpha.sh",
	}
	r2 := Resource{
		URI:        "file:///beta.csv",
		Name:       "shared-name",
		Entrypoint: "./beta.sh",
	}
	tk := validToolkitWithResources(r1, r2)

	errs := Validate(tk)
	ve := findErrorByRule(errs, "unique-resource-name")
	require.NotNil(t, ve,
		"duplicate resource names must emit 'unique-resource-name'; got rules: %v",
		errRules(errs))
	assert.Equal(t, SeverityError, ve.Severity)
	assert.Contains(t, ve.Message, "shared-name",
		"error message must mention the duplicate name")
	assert.Contains(t, ve.Path, "resources",
		"error path must reference 'resources'")
}

func TestValidateResource_DuplicateNames_ThreeWayDuplicate(t *testing.T) {
	r := Resource{
		URI:        "file:///data.csv",
		Name:       "dup-name",
		Entrypoint: "./fetch.sh",
	}
	r1 := r
	r1.URI = "file:///one.csv"
	r2 := r
	r2.URI = "file:///two.csv"
	r3 := r
	r3.URI = "file:///three.csv"
	tk := validToolkitWithResources(r1, r2, r3)

	errs := Validate(tk)
	n := countErrorsByRule(errs, "unique-resource-name")
	assert.GreaterOrEqual(t, n, 1,
		"three resources with same name must produce at least one 'unique-resource-name' error; got %d", n)
}

// Case-sensitive: "Report" and "report" are different names and should not
// conflict. This verifies the implementation does exact-match, not
// case-insensitive comparison.
func TestValidateResource_DifferentCase_NoDuplicate(t *testing.T) {
	r1 := Resource{
		URI:        "file:///upper.csv",
		Name:       "Report",
		Entrypoint: "./upper.sh",
	}
	r2 := Resource{
		URI:        "file:///lower.csv",
		Name:       "report",
		Entrypoint: "./lower.sh",
	}
	tk := validToolkitWithResources(r1, r2)

	errs := Validate(tk)
	ve := findErrorByRule(errs, "unique-resource-name")
	assert.Nil(t, ve,
		"names differing only in case must NOT trigger 'unique-resource-name'; got rules: %v",
		errRules(errs))
}

// ---------------------------------------------------------------------------
// AC5: Valid resource passes
// ---------------------------------------------------------------------------

func TestValidateResource_ValidWithAllFields(t *testing.T) {
	r := Resource{
		URI:         "file:///data/report.csv",
		Name:        "report-data",
		Description: "Monthly report data",
		MimeType:    "text/csv",
		Entrypoint:  "./fetch-report.sh",
	}
	tk := validToolkitWithResources(r)

	errs := Validate(tk)
	// Filter to resource-specific rules only, to avoid noise from unrelated
	// warnings (e.g., missing-item-schema on tools).
	resourceErrors := filterResourceErrors(errs)
	assert.Empty(t, resourceErrors,
		"valid resource with all fields must produce zero resource errors, got: %v",
		resourceErrors)
}

func TestValidateResource_ValidWithOptionalFieldsOmitted(t *testing.T) {
	// Description and MimeType are optional. A resource with only URI, Name,
	// and Entrypoint must pass.
	r := Resource{
		URI:        "file:///data.csv",
		Name:       "minimal-resource",
		Entrypoint: "./fetch.sh",
	}
	tk := validToolkitWithResources(r)

	errs := Validate(tk)
	resourceErrors := filterResourceErrors(errs)
	assert.Empty(t, resourceErrors,
		"resource with only required fields must produce zero resource errors, got: %v",
		resourceErrors)
}

func TestValidateResource_MultipleValidResources(t *testing.T) {
	r1 := Resource{
		URI:         "file:///alpha.csv",
		Name:        "alpha",
		Description: "Alpha data",
		MimeType:    "text/csv",
		Entrypoint:  "./alpha.sh",
	}
	r2 := Resource{
		URI:         "file:///beta.json",
		Name:        "beta",
		Description: "Beta data",
		MimeType:    "application/json",
		Entrypoint:  "./beta.sh",
	}
	r3 := Resource{
		URI:        "file:///gamma.txt",
		Name:       "gamma",
		Entrypoint: "./gamma.sh",
	}
	tk := validToolkitWithResources(r1, r2, r3)

	errs := Validate(tk)
	resourceErrors := filterResourceErrors(errs)
	assert.Empty(t, resourceErrors,
		"multiple valid resources with distinct names must produce zero resource errors, got: %v",
		resourceErrors)
}

// ---------------------------------------------------------------------------
// Edge cases
// ---------------------------------------------------------------------------

func TestValidateResource_EmptyResourcesSlice(t *testing.T) {
	tk := validToolkit()
	tk.Resources = []Resource{}

	errs := Validate(tk)
	errors := onlyErrors(errs)
	assert.Empty(t, errors,
		"empty resources slice must produce zero errors, got: %v", errors)
}

func TestValidateResource_NilResourcesSlice(t *testing.T) {
	tk := validToolkit()
	tk.Resources = nil

	errs := Validate(tk)
	errors := onlyErrors(errs)
	assert.Empty(t, errors,
		"nil resources slice must produce zero errors, got: %v", errors)
}

// ---------------------------------------------------------------------------
// Table-driven: required field validation for each field individually
// ---------------------------------------------------------------------------

func TestValidateResource_RequiredFields_TableDriven(t *testing.T) {
	tests := []struct {
		name     string
		resource Resource
		rule     string
		pathEnd  string
	}{
		{
			name: "URI empty string",
			resource: Resource{
				URI:        "",
				Name:       "ok-name",
				Entrypoint: "./ok.sh",
			},
			rule:    "resource-uri-required",
			pathEnd: ".uri",
		},
		{
			name: "Name empty string",
			resource: Resource{
				URI:        "file:///data.csv",
				Name:       "",
				Entrypoint: "./ok.sh",
			},
			rule:    "resource-name-required",
			pathEnd: ".name",
		},
		{
			name: "Entrypoint empty string",
			resource: Resource{
				URI:        "file:///data.csv",
				Name:       "ok-name",
				Entrypoint: "",
			},
			rule:    "resource-entrypoint-required",
			pathEnd: ".entrypoint",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tk := validToolkitWithResources(tc.resource)
			errs := Validate(tk)

			ve := findErrorByRule(errs, tc.rule)
			require.NotNil(t, ve,
				"missing field must emit rule %q; got rules: %v", tc.rule, errRules(errs))
			assert.Equal(t, SeverityError, ve.Severity,
				"required-field error must be severity 'error'")
			assert.Contains(t, ve.Path, tc.pathEnd,
				"error path must end with %q; got %q", tc.pathEnd, ve.Path)
		})
	}
}

// ---------------------------------------------------------------------------
// Integration: resource validation does not mask other validation errors
// ---------------------------------------------------------------------------

func TestValidateResource_DoesNotMaskMetadataErrors(t *testing.T) {
	tk := validToolkitWithResources(validResource())
	tk.Metadata.Name = "" // break metadata

	errs := Validate(tk)
	ve := findError(errs, "metadata.name", "name-format")
	require.NotNil(t, ve,
		"resource validation must not mask metadata errors; got rules: %v", errRules(errs))
}

func TestValidateResource_DoesNotMaskToolErrors(t *testing.T) {
	tk := validToolkitWithResources(validResource())
	tk.Tools[0].Flags = []Flag{{
		Name: "ok-flag",
		Type: "unknown-type-for-test",
	}}

	errs := Validate(tk)
	flagErr := findErrorByRule(errs, "unknown-flag-type")
	require.NotNil(t, flagErr,
		"resource validation must not mask tool/flag errors; got rules: %v", errRules(errs))
}

func TestValidateResource_ErrorsCoexistWithToolErrors(t *testing.T) {
	// Resource with missing URI AND tool with unknown flag type: both must be reported.
	r := validResource()
	r.URI = ""
	tk := validToolkitWithResources(r)
	tk.Tools[0].Flags = []Flag{{
		Name: "ok-flag",
		Type: "unknown-type-for-test",
	}}

	errs := Validate(tk)

	resourceErr := findErrorByRule(errs, "resource-uri-required")
	require.NotNil(t, resourceErr,
		"resource error must be reported alongside tool errors; got rules: %v", errRules(errs))

	flagErr := findErrorByRule(errs, "unknown-flag-type")
	require.NotNil(t, flagErr,
		"tool error must be reported alongside resource errors; got rules: %v", errRules(errs))
}

// ---------------------------------------------------------------------------
// Guard: duplicate resource names error mentions the offending name
// ---------------------------------------------------------------------------

func TestValidateResource_DuplicateName_MessageContainsName(t *testing.T) {
	r1 := Resource{
		URI:        "file:///one.csv",
		Name:       "my-resource",
		Entrypoint: "./one.sh",
	}
	r2 := Resource{
		URI:        "file:///two.csv",
		Name:       "my-resource",
		Entrypoint: "./two.sh",
	}
	tk := validToolkitWithResources(r1, r2)

	errs := Validate(tk)
	ve := findErrorByRule(errs, "unique-resource-name")
	require.NotNil(t, ve,
		"duplicate names must emit 'unique-resource-name'")
	assert.Contains(t, ve.Message, "my-resource",
		"error message must mention the duplicate name 'my-resource'; got %q", ve.Message)
}

// ---------------------------------------------------------------------------
// Guard: each required-field error rule is distinct (no conflation)
// ---------------------------------------------------------------------------

func TestValidateResource_RulesAreDistinct(t *testing.T) {
	// A resource missing only URI must NOT emit name-required or entrypoint-required.
	r := validResource()
	r.URI = ""
	tk := validToolkitWithResources(r)

	errs := Validate(tk)

	ve := findErrorByRule(errs, "resource-uri-required")
	require.NotNil(t, ve, "must emit 'resource-uri-required'")

	nameErr := findErrorByRule(errs, "resource-name-required")
	assert.Nil(t, nameErr,
		"resource with valid Name must NOT emit 'resource-name-required'; got rules: %v", errRules(errs))

	epErr := findErrorByRule(errs, "resource-entrypoint-required")
	assert.Nil(t, epErr,
		"resource with valid Entrypoint must NOT emit 'resource-entrypoint-required'; got rules: %v", errRules(errs))
}

// ---------------------------------------------------------------------------
// Helpers local to this file
// ---------------------------------------------------------------------------

// filterResourceErrors returns only validation errors whose Rule starts with
// "resource-" or "unique-resource-", filtering out tool/metadata/auth errors.
func filterResourceErrors(errs []ValidationError) []ValidationError {
	var out []ValidationError
	for _, e := range errs {
		switch e.Rule {
		case "resource-uri-required",
			"resource-name-required",
			"resource-entrypoint-required",
			"unique-resource-name":
			out = append(out, e)
		}
	}
	return out
}
