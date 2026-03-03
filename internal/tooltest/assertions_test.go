package tooltest

import (
	"io/fs"
	"testing"
	"testing/fstest"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// findResultByType returns the first AssertionResult matching the given type.
// It calls t.Fatal if no match is found.
func findResultByType(t *testing.T, results []AssertionResult, typ string) AssertionResult {
	t.Helper()
	for _, r := range results {
		if r.Type == typ {
			return r
		}
	}
	t.Fatalf("no result found with Type=%q in %d results", typ, len(results))
	return AssertionResult{} // unreachable
}

// ---------------------------------------------------------------------------
// AC-4: equals operator
// ---------------------------------------------------------------------------

func TestEvaluateAssertions_Equals_StringMatch(t *testing.T) {
	stdout := []byte(`{"status": "ok"}`)
	expect := Expectation{
		StdoutContains: []Assertion{
			{Path: "$.status", Equals: "ok"},
		},
	}

	results := EvaluateAssertions(stdout, nil, 0, expect, nil)

	require.Len(t, results, 1, "exactly one assertion result expected")
	r := results[0]
	assert.True(t, r.Passed, "equals 'ok' against 'ok' must pass")
	assert.Equal(t, "stdout_contains", r.Type)
	assert.Equal(t, "$.status", r.Path)
	assert.Empty(t, r.Error, "no error expected on passing assertion")
}

func TestEvaluateAssertions_Equals_StringMismatch(t *testing.T) {
	stdout := []byte(`{"status": "fail"}`)
	expect := Expectation{
		StdoutContains: []Assertion{
			{Path: "$.status", Equals: "ok"},
		},
	}

	results := EvaluateAssertions(stdout, nil, 0, expect, nil)

	require.Len(t, results, 1)
	r := results[0]
	assert.False(t, r.Passed, "equals 'ok' against 'fail' must not pass")
	assert.Equal(t, "stdout_contains", r.Type)
	assert.Equal(t, "$.status", r.Path)
	// Expected and Actual must contain human-readable representations of the values
	assert.Contains(t, r.Expected, "ok", "Expected must mention the expected value 'ok'")
	assert.Contains(t, r.Actual, "fail", "Actual must mention the actual value 'fail'")
}

func TestEvaluateAssertions_Equals_Numeric(t *testing.T) {
	stdout := []byte(`{"count": 3}`)
	expect := Expectation{
		StdoutContains: []Assertion{
			{Path: "$.count", Equals: 3},
		},
	}

	results := EvaluateAssertions(stdout, nil, 0, expect, nil)

	require.Len(t, results, 1)
	r := results[0]
	assert.True(t, r.Passed, "equals 3 against 3 must pass")
	assert.NotEqual(t, "$.status", r.Path, "path must not be hardcoded to $.status")
	assert.Equal(t, "$.count", r.Path)
}

func TestEvaluateAssertions_Equals_NumericMismatch(t *testing.T) {
	stdout := []byte(`{"count": 5}`)
	expect := Expectation{
		StdoutContains: []Assertion{
			{Path: "$.count", Equals: 3},
		},
	}

	results := EvaluateAssertions(stdout, nil, 0, expect, nil)

	require.Len(t, results, 1)
	r := results[0]
	assert.False(t, r.Passed, "equals 3 against 5 must fail")
}

func TestEvaluateAssertions_Equals_Boolean(t *testing.T) {
	stdout := []byte(`{"active": true}`)
	expect := Expectation{
		StdoutContains: []Assertion{
			{Path: "$.active", Equals: true},
		},
	}

	results := EvaluateAssertions(stdout, nil, 0, expect, nil)

	require.Len(t, results, 1)
	assert.True(t, results[0].Passed, "equals true against true must pass")
}

func TestEvaluateAssertions_Equals_BooleanMismatch(t *testing.T) {
	stdout := []byte(`{"active": false}`)
	expect := Expectation{
		StdoutContains: []Assertion{
			{Path: "$.active", Equals: true},
		},
	}

	results := EvaluateAssertions(stdout, nil, 0, expect, nil)

	require.Len(t, results, 1)
	assert.False(t, results[0].Passed, "equals true against false must fail")
}

// ---------------------------------------------------------------------------
// AC-5: contains operator
// ---------------------------------------------------------------------------

func TestEvaluateAssertions_Contains_ArrayElement(t *testing.T) {
	stdout := []byte(`{"tags": ["urgent", "low"]}`)
	expect := Expectation{
		StdoutContains: []Assertion{
			{Path: "$.tags", Contains: "urgent"},
		},
	}

	results := EvaluateAssertions(stdout, nil, 0, expect, nil)

	require.Len(t, results, 1)
	r := results[0]
	assert.True(t, r.Passed, "contains 'urgent' in ['urgent','low'] must pass")
	assert.Equal(t, "stdout_contains", r.Type)
	assert.Equal(t, "$.tags", r.Path)
}

func TestEvaluateAssertions_Contains_Substring(t *testing.T) {
	stdout := []byte(`{"message": "an error occurred"}`)
	expect := Expectation{
		StdoutContains: []Assertion{
			{Path: "$.message", Contains: "error"},
		},
	}

	results := EvaluateAssertions(stdout, nil, 0, expect, nil)

	require.Len(t, results, 1)
	assert.True(t, results[0].Passed, "contains 'error' in 'an error occurred' must pass (substring)")
}

func TestEvaluateAssertions_Contains_ArrayElement_Missing(t *testing.T) {
	stdout := []byte(`{"tags": ["urgent"]}`)
	expect := Expectation{
		StdoutContains: []Assertion{
			{Path: "$.tags", Contains: "missing"},
		},
	}

	results := EvaluateAssertions(stdout, nil, 0, expect, nil)

	require.Len(t, results, 1)
	r := results[0]
	assert.False(t, r.Passed, "contains 'missing' in ['urgent'] must fail")
	assert.Contains(t, r.Expected, "missing",
		"Expected must mention the value being searched for")
}

func TestEvaluateAssertions_Contains_SubstringNotFound(t *testing.T) {
	stdout := []byte(`{"message": "all good"}`)
	expect := Expectation{
		StdoutContains: []Assertion{
			{Path: "$.message", Contains: "error"},
		},
	}

	results := EvaluateAssertions(stdout, nil, 0, expect, nil)

	require.Len(t, results, 1)
	assert.False(t, results[0].Passed, "contains 'error' in 'all good' must fail")
}

// ---------------------------------------------------------------------------
// AC-6: matches operator
// ---------------------------------------------------------------------------

func TestEvaluateAssertions_Matches_Pass(t *testing.T) {
	stdout := []byte(`{"version": "1.2.3"}`)
	expect := Expectation{
		StdoutContains: []Assertion{
			{Path: "$.version", Matches: `^\d+\.\d+\.\d+$`},
		},
	}

	results := EvaluateAssertions(stdout, nil, 0, expect, nil)

	require.Len(t, results, 1)
	r := results[0]
	assert.True(t, r.Passed, "version '1.2.3' must match semver regex")
	assert.Equal(t, "stdout_contains", r.Type)
	assert.Equal(t, "$.version", r.Path)
	assert.Empty(t, r.Error)
}

func TestEvaluateAssertions_Matches_Fail(t *testing.T) {
	stdout := []byte(`{"version": "latest"}`)
	expect := Expectation{
		StdoutContains: []Assertion{
			{Path: "$.version", Matches: `^\d+\.\d+\.\d+$`},
		},
	}

	results := EvaluateAssertions(stdout, nil, 0, expect, nil)

	require.Len(t, results, 1)
	r := results[0]
	assert.False(t, r.Passed, "version 'latest' must not match semver regex")
	assert.Contains(t, r.Actual, "latest",
		"Actual must include the value that did not match")
}

func TestEvaluateAssertions_Matches_InvalidRegex_Error(t *testing.T) {
	stdout := []byte(`{"version": "1.2.3"}`)
	expect := Expectation{
		StdoutContains: []Assertion{
			{Path: "$.version", Matches: `[invalid`},
		},
	}

	// Must not panic
	results := EvaluateAssertions(stdout, nil, 0, expect, nil)

	require.Len(t, results, 1)
	r := results[0]
	assert.False(t, r.Passed, "invalid regex must not produce a pass")
	assert.NotEmpty(t, r.Error, "invalid regex must set the Error field")
}

// ---------------------------------------------------------------------------
// AC-7: exists operator
// ---------------------------------------------------------------------------

func TestEvaluateAssertions_Exists_True_FieldPresent(t *testing.T) {
	stdout := []byte(`{"field": "value"}`)
	expect := Expectation{
		StdoutContains: []Assertion{
			{Path: "$.field", Exists: boolPtr(true)},
		},
	}

	results := EvaluateAssertions(stdout, nil, 0, expect, nil)

	require.Len(t, results, 1)
	r := results[0]
	assert.True(t, r.Passed, "exists=true when field is present must pass")
	assert.Equal(t, "stdout_contains", r.Type)
	assert.Equal(t, "$.field", r.Path)
}

func TestEvaluateAssertions_Exists_True_FieldAbsent(t *testing.T) {
	stdout := []byte(`{}`)
	expect := Expectation{
		StdoutContains: []Assertion{
			{Path: "$.field", Exists: boolPtr(true)},
		},
	}

	results := EvaluateAssertions(stdout, nil, 0, expect, nil)

	require.Len(t, results, 1)
	r := results[0]
	assert.False(t, r.Passed, "exists=true when field absent must fail")
	assert.Equal(t, "$.field", r.Path)
}

func TestEvaluateAssertions_Exists_False_FieldAbsent(t *testing.T) {
	stdout := []byte(`{}`)
	expect := Expectation{
		StdoutContains: []Assertion{
			{Path: "$.field", Exists: boolPtr(false)},
		},
	}

	results := EvaluateAssertions(stdout, nil, 0, expect, nil)

	require.Len(t, results, 1)
	r := results[0]
	assert.True(t, r.Passed, "exists=false when field absent must pass")
}

func TestEvaluateAssertions_Exists_False_FieldPresent(t *testing.T) {
	stdout := []byte(`{"field": "value"}`)
	expect := Expectation{
		StdoutContains: []Assertion{
			{Path: "$.field", Exists: boolPtr(false)},
		},
	}

	results := EvaluateAssertions(stdout, nil, 0, expect, nil)

	require.Len(t, results, 1)
	assert.False(t, results[0].Passed, "exists=false when field present must fail")
}

func TestEvaluateAssertions_Exists_NullValue_StillExists(t *testing.T) {
	// A field with null value exists. exists=true should pass.
	stdout := []byte(`{"field": null}`)
	expect := Expectation{
		StdoutContains: []Assertion{
			{Path: "$.field", Exists: boolPtr(true)},
		},
	}

	results := EvaluateAssertions(stdout, nil, 0, expect, nil)

	require.Len(t, results, 1)
	assert.True(t, results[0].Passed,
		"a field with null value still exists; exists=true must pass")
}

// ---------------------------------------------------------------------------
// AC-8: length operator
// ---------------------------------------------------------------------------

func TestEvaluateAssertions_Length_Array_Pass(t *testing.T) {
	stdout := []byte(`{"items": [1, 2, 3]}`)
	expect := Expectation{
		StdoutContains: []Assertion{
			{Path: "$.items", Length: intPtr(3)},
		},
	}

	results := EvaluateAssertions(stdout, nil, 0, expect, nil)

	require.Len(t, results, 1)
	r := results[0]
	assert.True(t, r.Passed, "length 3 against [1,2,3] must pass")
	assert.Equal(t, "stdout_contains", r.Type)
	assert.Equal(t, "$.items", r.Path)
}

func TestEvaluateAssertions_Length_String_Pass(t *testing.T) {
	stdout := []byte(`{"name": "hello"}`)
	expect := Expectation{
		StdoutContains: []Assertion{
			{Path: "$.name", Length: intPtr(5)},
		},
	}

	results := EvaluateAssertions(stdout, nil, 0, expect, nil)

	require.Len(t, results, 1)
	assert.True(t, results[0].Passed, "length 5 against 'hello' must pass")
}

func TestEvaluateAssertions_Length_Array_Fail(t *testing.T) {
	stdout := []byte(`{"items": [1, 2, 3]}`)
	expect := Expectation{
		StdoutContains: []Assertion{
			{Path: "$.items", Length: intPtr(2)},
		},
	}

	results := EvaluateAssertions(stdout, nil, 0, expect, nil)

	require.Len(t, results, 1)
	r := results[0]
	assert.False(t, r.Passed, "length 2 against [1,2,3] must fail")
	assert.Contains(t, r.Expected, "2", "Expected must mention 2")
	assert.Contains(t, r.Actual, "3", "Actual must mention 3")
}

func TestEvaluateAssertions_Length_EmptyArray(t *testing.T) {
	stdout := []byte(`{"items": []}`)
	expect := Expectation{
		StdoutContains: []Assertion{
			{Path: "$.items", Length: intPtr(0)},
		},
	}

	results := EvaluateAssertions(stdout, nil, 0, expect, nil)

	require.Len(t, results, 1)
	assert.True(t, results[0].Passed, "length 0 against [] must pass")
}

func TestEvaluateAssertions_Length_EmptyString(t *testing.T) {
	stdout := []byte(`{"name": ""}`)
	expect := Expectation{
		StdoutContains: []Assertion{
			{Path: "$.name", Length: intPtr(0)},
		},
	}

	results := EvaluateAssertions(stdout, nil, 0, expect, nil)

	require.Len(t, results, 1)
	assert.True(t, results[0].Passed, "length 0 against '' must pass")
}

// ---------------------------------------------------------------------------
// AC-9: exit_code
// ---------------------------------------------------------------------------

func TestEvaluateAssertions_ExitCode_Match(t *testing.T) {
	expect := Expectation{
		ExitCode: intPtr(0),
	}

	results := EvaluateAssertions(nil, nil, 0, expect, nil)

	require.Len(t, results, 1)
	r := results[0]
	assert.True(t, r.Passed, "exit_code 0 expected, got 0: must pass")
	assert.Equal(t, "exit_code", r.Type)
	assert.Empty(t, r.Path, "exit_code results should have empty Path")
}

func TestEvaluateAssertions_ExitCode_Mismatch(t *testing.T) {
	expect := Expectation{
		ExitCode: intPtr(2),
	}

	results := EvaluateAssertions(nil, nil, 1, expect, nil)

	require.Len(t, results, 1)
	r := results[0]
	assert.False(t, r.Passed, "exit_code 2 expected, got 1: must fail")
	assert.Equal(t, "exit_code", r.Type)
	assert.Contains(t, r.Expected, "2", "Expected must mention 2")
	assert.Contains(t, r.Actual, "1", "Actual must mention 1")
}

func TestEvaluateAssertions_ExitCode_Nil_NoResult(t *testing.T) {
	expect := Expectation{
		ExitCode: nil, // no check
	}

	results := EvaluateAssertions(nil, nil, 42, expect, nil)

	// No exit_code result should be emitted
	for _, r := range results {
		assert.NotEqual(t, "exit_code", r.Type,
			"nil ExitCode must not emit an exit_code result")
	}
}

func TestEvaluateAssertions_ExitCode_NonZeroMatch(t *testing.T) {
	// Ensure the implementation doesn't hardcode success = 0
	expect := Expectation{
		ExitCode: intPtr(137),
	}

	results := EvaluateAssertions(nil, nil, 137, expect, nil)

	exitResults := 0
	for _, r := range results {
		if r.Type == "exit_code" {
			exitResults++
			assert.True(t, r.Passed, "exit_code 137 expected, got 137: must pass")
		}
	}
	assert.Equal(t, 1, exitResults, "exactly one exit_code result expected")
}

// ---------------------------------------------------------------------------
// AC-10: stdout_is_json
// ---------------------------------------------------------------------------

func TestEvaluateAssertions_StdoutIsJSON_ValidJSON_Pass(t *testing.T) {
	stdout := []byte(`{"key": "value"}`)
	expect := Expectation{
		StdoutIsJSON: boolPtr(true),
	}

	results := EvaluateAssertions(stdout, nil, 0, expect, nil)

	r := findResultByType(t, results, "stdout_is_json")
	assert.True(t, r.Passed, "valid JSON stdout with stdout_is_json=true must pass")
}

func TestEvaluateAssertions_StdoutIsJSON_PlainText_Fail(t *testing.T) {
	stdout := []byte(`this is not json`)
	expect := Expectation{
		StdoutIsJSON: boolPtr(true),
	}

	results := EvaluateAssertions(stdout, nil, 0, expect, nil)

	r := findResultByType(t, results, "stdout_is_json")
	assert.False(t, r.Passed, "plain text stdout with stdout_is_json=true must fail")
}

func TestEvaluateAssertions_StdoutIsJSON_Nil_NoResult(t *testing.T) {
	stdout := []byte(`not json`)
	expect := Expectation{
		StdoutIsJSON: nil, // no check
	}

	results := EvaluateAssertions(stdout, nil, 0, expect, nil)

	for _, r := range results {
		assert.NotEqual(t, "stdout_is_json", r.Type,
			"nil StdoutIsJSON must not emit a stdout_is_json result")
	}
}

func TestEvaluateAssertions_StdoutIsJSON_False_ValidJSON_Fail(t *testing.T) {
	// stdout_is_json: false means "assert output is NOT JSON"
	stdout := []byte(`{"key": "value"}`)
	expect := Expectation{
		StdoutIsJSON: boolPtr(false),
	}

	results := EvaluateAssertions(stdout, nil, 0, expect, nil)

	r := findResultByType(t, results, "stdout_is_json")
	assert.False(t, r.Passed,
		"valid JSON stdout with stdout_is_json=false must fail (it IS json, but we expect it not to be)")
}

func TestEvaluateAssertions_StdoutIsJSON_False_PlainText_Pass(t *testing.T) {
	stdout := []byte(`just text`)
	expect := Expectation{
		StdoutIsJSON: boolPtr(false),
	}

	results := EvaluateAssertions(stdout, nil, 0, expect, nil)

	r := findResultByType(t, results, "stdout_is_json")
	assert.True(t, r.Passed,
		"plain text stdout with stdout_is_json=false must pass (it's NOT json, as expected)")
}

func TestEvaluateAssertions_StdoutIsJSON_EmptyStdout_NotJSON(t *testing.T) {
	expect := Expectation{
		StdoutIsJSON: boolPtr(true),
	}

	results := EvaluateAssertions([]byte{}, nil, 0, expect, nil)

	r := findResultByType(t, results, "stdout_is_json")
	assert.False(t, r.Passed, "empty stdout is not valid JSON")
}

func TestEvaluateAssertions_StdoutIsJSON_Array(t *testing.T) {
	// JSON arrays are valid JSON
	stdout := []byte(`[1, 2, 3]`)
	expect := Expectation{
		StdoutIsJSON: boolPtr(true),
	}

	results := EvaluateAssertions(stdout, nil, 0, expect, nil)

	r := findResultByType(t, results, "stdout_is_json")
	assert.True(t, r.Passed, "JSON array is valid JSON")
}

// ---------------------------------------------------------------------------
// AC-11: stdout_schema
// ---------------------------------------------------------------------------

func TestEvaluateAssertions_StdoutSchema_ValidJSON_MatchingSchema(t *testing.T) {
	stdout := []byte(`{"name": "test", "count": 5}`)
	schemaJSON := `{
		"type": "object",
		"required": ["name", "count"],
		"properties": {
			"name": {"type": "string"},
			"count": {"type": "integer"}
		}
	}`
	schemaFS := fstest.MapFS{
		"schemas/output.json": &fstest.MapFile{Data: []byte(schemaJSON)},
	}

	expect := Expectation{
		StdoutSchema: "schemas/output.json",
	}

	results := EvaluateAssertions(stdout, nil, 0, expect, schemaFS)

	r := findResultByType(t, results, "stdout_schema")
	assert.True(t, r.Passed, "JSON matching schema must pass")
}

func TestEvaluateAssertions_StdoutSchema_MissingRequiredField(t *testing.T) {
	stdout := []byte(`{"name": "test"}`) // missing "count"
	schemaJSON := `{
		"type": "object",
		"required": ["name", "count"],
		"properties": {
			"name": {"type": "string"},
			"count": {"type": "integer"}
		}
	}`
	schemaFS := fstest.MapFS{
		"schemas/output.json": &fstest.MapFile{Data: []byte(schemaJSON)},
	}

	expect := Expectation{
		StdoutSchema: "schemas/output.json",
	}

	results := EvaluateAssertions(stdout, nil, 0, expect, schemaFS)

	r := findResultByType(t, results, "stdout_schema")
	assert.False(t, r.Passed, "JSON missing required field must fail schema validation")
	// Should have some indication of what went wrong
	errorOrActual := r.Error + r.Actual
	assert.NotEmpty(t, errorOrActual,
		"failed schema validation must provide error details")
}

func TestEvaluateAssertions_StdoutSchema_EmptyString_NoResult(t *testing.T) {
	stdout := []byte(`{"key": "value"}`)
	expect := Expectation{
		StdoutSchema: "", // no schema check
	}

	results := EvaluateAssertions(stdout, nil, 0, expect, nil)

	for _, r := range results {
		assert.NotEqual(t, "stdout_schema", r.Type,
			"empty StdoutSchema must not emit a stdout_schema result")
	}
}

func TestEvaluateAssertions_StdoutSchema_WrongType(t *testing.T) {
	stdout := []byte(`{"name": 123}`) // name should be string, not number
	schemaJSON := `{
		"type": "object",
		"required": ["name"],
		"properties": {
			"name": {"type": "string"}
		}
	}`
	schemaFS := fstest.MapFS{
		"schemas/output.json": &fstest.MapFile{Data: []byte(schemaJSON)},
	}

	expect := Expectation{
		StdoutSchema: "schemas/output.json",
	}

	results := EvaluateAssertions(stdout, nil, 0, expect, schemaFS)

	r := findResultByType(t, results, "stdout_schema")
	assert.False(t, r.Passed, "wrong type for property must fail schema validation")
}

// ---------------------------------------------------------------------------
// AC-12: stderr_contains
// ---------------------------------------------------------------------------

func TestEvaluateAssertions_StderrContains_SingleString_Pass(t *testing.T) {
	stderr := []byte("error: path does not exist\n")
	expect := Expectation{
		StderrContains: []string{"path does not exist"},
	}

	results := EvaluateAssertions(nil, stderr, 0, expect, nil)

	stderrResults := 0
	for _, r := range results {
		if r.Type == "stderr_contains" {
			stderrResults++
			assert.True(t, r.Passed,
				"stderr contains 'path does not exist' must pass")
		}
	}
	assert.Equal(t, 1, stderrResults, "one stderr_contains result expected")
}

func TestEvaluateAssertions_StderrContains_MultipleStrings_AllPresent(t *testing.T) {
	stderr := []byte("warning: deprecated feature\nerror: path does not exist\n")
	expect := Expectation{
		StderrContains: []string{"deprecated", "path does not exist"},
	}

	results := EvaluateAssertions(nil, stderr, 0, expect, nil)

	stderrResults := []AssertionResult{}
	for _, r := range results {
		if r.Type == "stderr_contains" {
			stderrResults = append(stderrResults, r)
		}
	}
	require.Len(t, stderrResults, 2,
		"two stderr_contains strings must produce two results (AND semantics)")
	for _, r := range stderrResults {
		assert.True(t, r.Passed, "both stderr strings are present, both must pass")
	}
}

func TestEvaluateAssertions_StderrContains_StringNotFound(t *testing.T) {
	stderr := []byte("all good\n")
	expect := Expectation{
		StderrContains: []string{"error"},
	}

	results := EvaluateAssertions(nil, stderr, 0, expect, nil)

	stderrResults := 0
	for _, r := range results {
		if r.Type == "stderr_contains" {
			stderrResults++
			assert.False(t, r.Passed, "stderr does not contain 'error', must fail")
			assert.Contains(t, r.Expected, "error",
				"Expected must mention the string being searched for")
		}
	}
	assert.Equal(t, 1, stderrResults, "one stderr_contains result expected")
}

func TestEvaluateAssertions_StderrContains_ANDSemantics_OneMissing(t *testing.T) {
	stderr := []byte("warning: deprecated\n")
	expect := Expectation{
		StderrContains: []string{"deprecated", "fatal error"},
	}

	results := EvaluateAssertions(nil, stderr, 0, expect, nil)

	stderrResults := []AssertionResult{}
	for _, r := range results {
		if r.Type == "stderr_contains" {
			stderrResults = append(stderrResults, r)
		}
	}
	require.Len(t, stderrResults, 2,
		"two stderr_contains strings must produce two separate results")

	passCount := 0
	failCount := 0
	for _, r := range stderrResults {
		if r.Passed {
			passCount++
		} else {
			failCount++
		}
	}
	assert.Equal(t, 1, passCount, "exactly one string is present")
	assert.Equal(t, 1, failCount, "exactly one string is missing")
}

func TestEvaluateAssertions_StderrContains_EmptySlice_NoResults(t *testing.T) {
	stderr := []byte("some output")
	expect := Expectation{
		StderrContains: []string{},
	}

	results := EvaluateAssertions(nil, stderr, 0, expect, nil)

	for _, r := range results {
		assert.NotEqual(t, "stderr_contains", r.Type,
			"empty StderrContains must not emit any stderr_contains results")
	}
}

// ---------------------------------------------------------------------------
// AC-17: JSONPath with array index and nested paths
// ---------------------------------------------------------------------------

func TestEvaluateAssertions_JSONPath_ArrayIndex(t *testing.T) {
	stdout := []byte(`{
		"findings": [
			{"type": "sql_injection", "severity": "high"},
			{"type": "xss", "severity": "medium"}
		]
	}`)
	expect := Expectation{
		StdoutContains: []Assertion{
			{Path: "$.findings[0].type", Equals: "sql_injection"},
		},
	}

	results := EvaluateAssertions(stdout, nil, 0, expect, nil)

	require.Len(t, results, 1)
	assert.True(t, results[0].Passed,
		"$.findings[0].type must resolve to 'sql_injection'")
}

func TestEvaluateAssertions_JSONPath_ArrayIndex_SecondElement(t *testing.T) {
	stdout := []byte(`{
		"findings": [
			{"type": "sql_injection"},
			{"type": "xss"}
		]
	}`)
	expect := Expectation{
		StdoutContains: []Assertion{
			{Path: "$.findings[1].type", Equals: "xss"},
		},
	}

	results := EvaluateAssertions(stdout, nil, 0, expect, nil)

	require.Len(t, results, 1)
	assert.True(t, results[0].Passed,
		"$.findings[1].type must resolve to 'xss', not hardcoded to first element")
}

func TestEvaluateAssertions_JSONPath_Wildcard_Contains(t *testing.T) {
	stdout := []byte(`{
		"findings": [
			{"type": "sql_injection"},
			{"type": "xss"},
			{"type": "csrf"}
		]
	}`)
	expect := Expectation{
		StdoutContains: []Assertion{
			{Path: "$.findings[*].type", Contains: "xss"},
		},
	}

	results := EvaluateAssertions(stdout, nil, 0, expect, nil)

	require.Len(t, results, 1)
	assert.True(t, results[0].Passed,
		"$.findings[*].type must resolve to array containing 'xss'")
}

func TestEvaluateAssertions_JSONPath_Wildcard_Length(t *testing.T) {
	stdout := []byte(`{
		"findings": [
			{"type": "sql_injection"},
			{"type": "xss"},
			{"type": "csrf"}
		]
	}`)
	expect := Expectation{
		StdoutContains: []Assertion{
			{Path: "$.findings[*].type", Length: intPtr(3)},
		},
	}

	results := EvaluateAssertions(stdout, nil, 0, expect, nil)

	require.Len(t, results, 1)
	assert.True(t, results[0].Passed,
		"$.findings[*].type should resolve to 3 elements")
}

func TestEvaluateAssertions_JSONPath_DeeplyNested(t *testing.T) {
	stdout := []byte(`{
		"deeply": {
			"nested": {
				"path": "found"
			}
		}
	}`)
	expect := Expectation{
		StdoutContains: []Assertion{
			{Path: "$.deeply.nested.path", Equals: "found"},
		},
	}

	results := EvaluateAssertions(stdout, nil, 0, expect, nil)

	require.Len(t, results, 1)
	assert.True(t, results[0].Passed,
		"$.deeply.nested.path must resolve to 'found'")
}

func TestEvaluateAssertions_JSONPath_DeeplyNested_WrongValue(t *testing.T) {
	stdout := []byte(`{
		"deeply": {
			"nested": {
				"path": "other"
			}
		}
	}`)
	expect := Expectation{
		StdoutContains: []Assertion{
			{Path: "$.deeply.nested.path", Equals: "found"},
		},
	}

	results := EvaluateAssertions(stdout, nil, 0, expect, nil)

	require.Len(t, results, 1)
	assert.False(t, results[0].Passed,
		"$.deeply.nested.path is 'other', not 'found': must fail")
}

// ---------------------------------------------------------------------------
// Combined: multiple assertion types in one Expectation
// ---------------------------------------------------------------------------

func TestEvaluateAssertions_MultipleAssertionTypes_Combined(t *testing.T) {
	stdout := []byte(`{
		"status": "ok",
		"count": 3,
		"items": ["a", "b", "c"],
		"version": "2.0.1"
	}`)
	stderr := []byte("processing complete\n")

	expect := Expectation{
		ExitCode:     intPtr(0),
		StdoutIsJSON: boolPtr(true),
		StdoutContains: []Assertion{
			{Path: "$.status", Equals: "ok"},
			{Path: "$.items", Contains: "b"},
			{Path: "$.version", Matches: `^\d+\.\d+\.\d+$`},
			{Path: "$.count", Exists: boolPtr(true)},
			{Path: "$.items", Length: intPtr(3)},
		},
		StderrContains: []string{"processing complete"},
	}

	results := EvaluateAssertions(stdout, stderr, 0, expect, nil)

	// Count expected results:
	// 1 exit_code + 1 stdout_is_json + 5 stdout_contains + 1 stderr_contains = 8
	require.Len(t, results, 8,
		"must have exactly 8 results: 1 exit_code + 1 stdout_is_json + 5 stdout_contains + 1 stderr_contains")

	for _, r := range results {
		assert.True(t, r.Passed,
			"all assertions in this test must pass: %s (path=%s) failed: expected=%s actual=%s error=%s",
			r.Type, r.Path, r.Expected, r.Actual, r.Error)
	}
}

func TestEvaluateAssertions_MultipleAssertionTypes_SomeFail(t *testing.T) {
	stdout := []byte(`{
		"status": "error",
		"count": 5,
		"items": ["a"]
	}`)
	stderr := []byte("done\n")

	expect := Expectation{
		ExitCode:     intPtr(0),
		StdoutIsJSON: boolPtr(true),
		StdoutContains: []Assertion{
			{Path: "$.status", Equals: "ok"},         // FAIL: status is "error"
			{Path: "$.items", Length: intPtr(3)},     // FAIL: length is 1
			{Path: "$.count", Exists: boolPtr(true)}, // PASS: count exists
		},
		StderrContains: []string{"fatal"}, // FAIL: stderr is "done"
	}

	results := EvaluateAssertions(stdout, stderr, 0, expect, nil)

	// 1 exit_code + 1 stdout_is_json + 3 stdout_contains + 1 stderr_contains = 6
	require.Len(t, results, 6)

	passCount := 0
	failCount := 0
	for _, r := range results {
		if r.Passed {
			passCount++
		} else {
			failCount++
		}
	}
	// Pass: exit_code=0, stdout_is_json=true, count exists = 3 passes
	// Fail: status equals, items length, stderr contains = 3 fails
	assert.Equal(t, 3, passCount, "3 assertions should pass")
	assert.Equal(t, 3, failCount, "3 assertions should fail")
}

// ---------------------------------------------------------------------------
// Empty/nil Expectation: no results emitted
// ---------------------------------------------------------------------------

func TestEvaluateAssertions_EmptyExpectation_NoResults(t *testing.T) {
	expect := Expectation{}

	results := EvaluateAssertions([]byte(`{}`), nil, 0, expect, nil)

	assert.Empty(t, results,
		"empty Expectation (all nil/zero) must produce no results")
}

// ---------------------------------------------------------------------------
// AssertionResult structural tests
// ---------------------------------------------------------------------------

func TestAssertionResult_Type_Values(t *testing.T) {
	// Verify that the implementation uses the exact Type strings specified.
	// This tests against implementations that use different type strings.
	tests := []struct {
		name   string
		expect Expectation
		typ    string
	}{
		{
			name:   "exit_code type",
			expect: Expectation{ExitCode: intPtr(0)},
			typ:    "exit_code",
		},
		{
			name:   "stdout_is_json type",
			expect: Expectation{StdoutIsJSON: boolPtr(true)},
			typ:    "stdout_is_json",
		},
		{
			name: "stdout_contains type",
			expect: Expectation{
				StdoutContains: []Assertion{{Path: "$.x", Equals: "y"}},
			},
			typ: "stdout_contains",
		},
		{
			name:   "stderr_contains type",
			expect: Expectation{StderrContains: []string{"err"}},
			typ:    "stderr_contains",
		},
	}

	stdout := []byte(`{"x": "y"}`)
	stderr := []byte("err")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := EvaluateAssertions(stdout, stderr, 0, tt.expect, nil)
			require.NotEmpty(t, results, "expected at least one result")
			found := false
			for _, r := range results {
				if r.Type == tt.typ {
					found = true
					break
				}
			}
			assert.True(t, found, "expected to find result with Type=%q", tt.typ)
		})
	}
}

func TestAssertionResult_StdoutSchema_Type(t *testing.T) {
	schemaJSON := `{"type": "object"}`
	schemaFS := fstest.MapFS{
		"schema.json": &fstest.MapFile{Data: []byte(schemaJSON)},
	}
	expect := Expectation{
		StdoutSchema: "schema.json",
	}

	results := EvaluateAssertions([]byte(`{}`), nil, 0, expect, schemaFS)

	r := findResultByType(t, results, "stdout_schema")
	assert.Equal(t, "stdout_schema", r.Type)
}

// ---------------------------------------------------------------------------
// Table-driven: equals with various JSON types
// ---------------------------------------------------------------------------

func TestEvaluateAssertions_Equals_TableDriven(t *testing.T) {
	tests := []struct {
		name     string
		json     string
		path     string
		equals   any
		wantPass bool
	}{
		{
			name:     "string match",
			json:     `{"v": "hello"}`,
			path:     "$.v",
			equals:   "hello",
			wantPass: true,
		},
		{
			name:     "string mismatch",
			json:     `{"v": "world"}`,
			path:     "$.v",
			equals:   "hello",
			wantPass: false,
		},
		{
			name:     "int match",
			json:     `{"v": 42}`,
			path:     "$.v",
			equals:   42,
			wantPass: true,
		},
		{
			name:     "int mismatch",
			json:     `{"v": 99}`,
			path:     "$.v",
			equals:   42,
			wantPass: false,
		},
		{
			name:     "float match",
			json:     `{"v": 3.14}`,
			path:     "$.v",
			equals:   3.14,
			wantPass: true,
		},
		{
			name:     "bool true match",
			json:     `{"v": true}`,
			path:     "$.v",
			equals:   true,
			wantPass: true,
		},
		{
			name:     "bool false match",
			json:     `{"v": false}`,
			path:     "$.v",
			equals:   false,
			wantPass: true,
		},
		{
			name:     "bool true vs false",
			json:     `{"v": false}`,
			path:     "$.v",
			equals:   true,
			wantPass: false,
		},
		{
			name:     "null match",
			json:     `{"v": null}`,
			path:     "$.v",
			equals:   nil,
			wantPass: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expect := Expectation{
				StdoutContains: []Assertion{
					{Path: tt.path, Equals: tt.equals},
				},
			}
			results := EvaluateAssertions([]byte(tt.json), nil, 0, expect, nil)
			require.Len(t, results, 1, "exactly one result expected")
			assert.Equal(t, tt.wantPass, results[0].Passed,
				"equals assertion: expected Passed=%v, got Passed=%v (expected=%s actual=%s)",
				tt.wantPass, results[0].Passed, results[0].Expected, results[0].Actual)
		})
	}
}

// ---------------------------------------------------------------------------
// Table-driven: contains with various scenarios
// ---------------------------------------------------------------------------

func TestEvaluateAssertions_Contains_TableDriven(t *testing.T) {
	tests := []struct {
		name     string
		json     string
		path     string
		contains any
		wantPass bool
	}{
		{
			name:     "array contains string element",
			json:     `{"arr": ["a", "b", "c"]}`,
			path:     "$.arr",
			contains: "b",
			wantPass: true,
		},
		{
			name:     "array does not contain element",
			json:     `{"arr": ["a", "b"]}`,
			path:     "$.arr",
			contains: "z",
			wantPass: false,
		},
		{
			name:     "string contains substring",
			json:     `{"msg": "hello world"}`,
			path:     "$.msg",
			contains: "world",
			wantPass: true,
		},
		{
			name:     "string does not contain substring",
			json:     `{"msg": "hello world"}`,
			path:     "$.msg",
			contains: "goodbye",
			wantPass: false,
		},
		{
			name:     "array contains numeric element",
			json:     `{"arr": [1, 2, 3]}`,
			path:     "$.arr",
			contains: 2,
			wantPass: true,
		},
		{
			name:     "empty array contains nothing",
			json:     `{"arr": []}`,
			path:     "$.arr",
			contains: "x",
			wantPass: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expect := Expectation{
				StdoutContains: []Assertion{
					{Path: tt.path, Contains: tt.contains},
				},
			}
			results := EvaluateAssertions([]byte(tt.json), nil, 0, expect, nil)
			require.Len(t, results, 1)
			assert.Equal(t, tt.wantPass, results[0].Passed,
				"contains assertion: expected Passed=%v (expected=%s actual=%s error=%s)",
				tt.wantPass, results[0].Expected, results[0].Actual, results[0].Error)
		})
	}
}

// ---------------------------------------------------------------------------
// Table-driven: matches with various patterns
// ---------------------------------------------------------------------------

func TestEvaluateAssertions_Matches_TableDriven(t *testing.T) {
	tests := []struct {
		name     string
		json     string
		path     string
		pattern  string
		wantPass bool
		wantErr  bool
	}{
		{
			name:     "semver match",
			json:     `{"v": "1.2.3"}`,
			path:     "$.v",
			pattern:  `^\d+\.\d+\.\d+$`,
			wantPass: true,
		},
		{
			name:     "semver no match",
			json:     `{"v": "latest"}`,
			path:     "$.v",
			pattern:  `^\d+\.\d+\.\d+$`,
			wantPass: false,
		},
		{
			name:     "email-like pattern",
			json:     `{"email": "user@example.com"}`,
			path:     "$.email",
			pattern:  `^[^@]+@[^@]+\.[^@]+$`,
			wantPass: true,
		},
		{
			name:    "invalid regex",
			json:    `{"v": "anything"}`,
			path:    "$.v",
			pattern: `[unclosed`,
			wantErr: true,
		},
		{
			name:     "anchored pattern prevents partial match",
			json:     `{"v": "abc123def"}`,
			path:     "$.v",
			pattern:  `^\d+$`,
			wantPass: false,
		},
		{
			name:     "partial match without anchors",
			json:     `{"v": "abc123def"}`,
			path:     "$.v",
			pattern:  `\d+`,
			wantPass: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			expect := Expectation{
				StdoutContains: []Assertion{
					{Path: tt.path, Matches: tt.pattern},
				},
			}
			results := EvaluateAssertions([]byte(tt.json), nil, 0, expect, nil)
			require.Len(t, results, 1)
			if tt.wantErr {
				assert.False(t, results[0].Passed, "invalid regex must not pass")
				assert.NotEmpty(t, results[0].Error, "invalid regex must set Error")
			} else {
				assert.Equal(t, tt.wantPass, results[0].Passed)
				assert.Empty(t, results[0].Error, "valid regex must not set Error")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Result count verification: anti-hardcoding
// ---------------------------------------------------------------------------

func TestEvaluateAssertions_ResultCount_MatchesAssertionCount(t *testing.T) {
	// The number of results must exactly correspond to the number of assertions.
	// This catches implementations that return a fixed number of results.
	tests := []struct {
		name      string
		expect    Expectation
		wantCount int
	}{
		{
			name:      "exit_code only",
			expect:    Expectation{ExitCode: intPtr(0)},
			wantCount: 1,
		},
		{
			name:      "stdout_is_json only",
			expect:    Expectation{StdoutIsJSON: boolPtr(true)},
			wantCount: 1,
		},
		{
			name: "three stdout_contains",
			expect: Expectation{
				StdoutContains: []Assertion{
					{Path: "$.a", Equals: "x"},
					{Path: "$.b", Equals: "y"},
					{Path: "$.c", Equals: "z"},
				},
			},
			wantCount: 3,
		},
		{
			name: "two stderr_contains",
			expect: Expectation{
				StderrContains: []string{"alpha", "beta"},
			},
			wantCount: 2,
		},
		{
			name: "all types combined",
			expect: Expectation{
				ExitCode:     intPtr(0),
				StdoutIsJSON: boolPtr(true),
				StdoutContains: []Assertion{
					{Path: "$.x", Equals: "v"},
				},
				StderrContains: []string{"err"},
			},
			wantCount: 4,
		},
	}

	stdout := []byte(`{"a":"x","b":"y","c":"z","x":"v"}`)
	stderr := []byte("alpha beta err")

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results := EvaluateAssertions(stdout, stderr, 0, tt.expect, nil)
			assert.Len(t, results, tt.wantCount,
				"result count must match assertion count")
		})
	}
}

// ---------------------------------------------------------------------------
// Edge: JSONPath resolves to no value
// ---------------------------------------------------------------------------

func TestEvaluateAssertions_JSONPath_NoMatch_Equals(t *testing.T) {
	stdout := []byte(`{"other": "value"}`)
	expect := Expectation{
		StdoutContains: []Assertion{
			{Path: "$.missing", Equals: "anything"},
		},
	}

	results := EvaluateAssertions(stdout, nil, 0, expect, nil)

	require.Len(t, results, 1)
	assert.False(t, results[0].Passed,
		"equals on a path that doesn't exist must fail")
}

func TestEvaluateAssertions_JSONPath_NoMatch_Length(t *testing.T) {
	stdout := []byte(`{"other": "value"}`)
	expect := Expectation{
		StdoutContains: []Assertion{
			{Path: "$.missing", Length: intPtr(0)},
		},
	}

	results := EvaluateAssertions(stdout, nil, 0, expect, nil)

	require.Len(t, results, 1)
	assert.False(t, results[0].Passed,
		"length on a path that doesn't exist must fail (not silently return 0)")
}

// ---------------------------------------------------------------------------
// Edge: invalid JSON stdout with stdout_contains assertions
// ---------------------------------------------------------------------------

func TestEvaluateAssertions_InvalidJSON_StdoutContains(t *testing.T) {
	stdout := []byte(`not json at all`)
	expect := Expectation{
		StdoutContains: []Assertion{
			{Path: "$.field", Equals: "value"},
		},
	}

	// Must not panic
	results := EvaluateAssertions(stdout, nil, 0, expect, nil)

	require.Len(t, results, 1)
	r := results[0]
	assert.False(t, r.Passed, "assertion on invalid JSON must fail")
	// Should indicate an error, not just a mismatch
	errorOrExpected := r.Error + r.Actual
	assert.NotEmpty(t, errorOrExpected,
		"invalid JSON should produce an error or actual description")
}

// ---------------------------------------------------------------------------
// AssertionResult: structural comparison with go-cmp
// ---------------------------------------------------------------------------

func TestAssertionResult_StructuralEquality(t *testing.T) {
	a := AssertionResult{
		Type:     "exit_code",
		Path:     "",
		Passed:   true,
		Expected: "0",
		Actual:   "0",
		Error:    "",
	}
	b := AssertionResult{
		Type:     "exit_code",
		Path:     "",
		Passed:   true,
		Expected: "0",
		Actual:   "0",
		Error:    "",
	}

	if diff := cmp.Diff(a, b); diff != "" {
		t.Errorf("identical AssertionResult values must be equal (-a +b):\n%s", diff)
	}
}

func TestAssertionResult_StructuralInequality(t *testing.T) {
	a := AssertionResult{
		Type:   "exit_code",
		Passed: true,
	}
	b := AssertionResult{
		Type:   "exit_code",
		Passed: false,
	}

	diff := cmp.Diff(a, b)
	assert.NotEmpty(t, diff,
		"AssertionResults with different Passed values must not be equal")
}

// ---------------------------------------------------------------------------
// Edge: schemaFS is nil but StdoutSchema is set
// ---------------------------------------------------------------------------

func TestEvaluateAssertions_StdoutSchema_NilFS(t *testing.T) {
	stdout := []byte(`{"key": "value"}`)
	expect := Expectation{
		StdoutSchema: "schema.json",
	}

	// Must not panic when schemaFS is nil
	results := EvaluateAssertions(stdout, nil, 0, expect, nil)

	r := findResultByType(t, results, "stdout_schema")
	assert.False(t, r.Passed,
		"stdout_schema with nil schemaFS must fail")
	assert.NotEmpty(t, r.Error,
		"stdout_schema with nil schemaFS must report an error")
}

// ---------------------------------------------------------------------------
// Edge: nil and empty stderr with stderr_contains
// ---------------------------------------------------------------------------

func TestEvaluateAssertions_StderrContains_NilStderr(t *testing.T) {
	expect := Expectation{
		StderrContains: []string{"anything"},
	}

	results := EvaluateAssertions(nil, nil, 0, expect, nil)

	stderrResults := 0
	for _, r := range results {
		if r.Type == "stderr_contains" {
			stderrResults++
			assert.False(t, r.Passed, "nil stderr cannot contain any string")
		}
	}
	assert.Equal(t, 1, stderrResults)
}

func TestEvaluateAssertions_StderrContains_EmptyStderr(t *testing.T) {
	expect := Expectation{
		StderrContains: []string{"anything"},
	}

	results := EvaluateAssertions(nil, []byte{}, 0, expect, nil)

	for _, r := range results {
		if r.Type == "stderr_contains" {
			assert.False(t, r.Passed, "empty stderr cannot contain any string")
		}
	}
}

// ---------------------------------------------------------------------------
// Verify schemaFS uses fs.FS interface (constitution 17a)
// ---------------------------------------------------------------------------

func TestEvaluateAssertions_SchemaFS_AcceptsMapFS(t *testing.T) {
	// This test verifies at compile time that EvaluateAssertions accepts fs.FS.
	// The fstest.MapFS satisfies fs.FS.
	var schemaFS fs.FS = fstest.MapFS{
		"test.json": &fstest.MapFile{Data: []byte(`{"type": "object"}`)},
	}

	expect := Expectation{
		StdoutSchema: "test.json",
	}

	// Must not panic, must accept the fs.FS
	results := EvaluateAssertions([]byte(`{}`), nil, 0, expect, schemaFS)
	require.NotEmpty(t, results)
	r := findResultByType(t, results, "stdout_schema")
	assert.True(t, r.Passed, "empty object matches {type: object} schema")
}

// ---------------------------------------------------------------------------
// Anti-hardcoding: multiple distinct calls must produce distinct results
// ---------------------------------------------------------------------------

func TestEvaluateAssertions_AntiHardcoding_DistinctInputsDistinctOutputs(t *testing.T) {
	// Call EvaluateAssertions with two different inputs and verify the results differ.
	// A hardcoded implementation would return the same thing both times.
	stdout1 := []byte(`{"status": "ok"}`)
	stdout2 := []byte(`{"status": "fail"}`)

	expect := Expectation{
		StdoutContains: []Assertion{
			{Path: "$.status", Equals: "ok"},
		},
	}

	results1 := EvaluateAssertions(stdout1, nil, 0, expect, nil)
	results2 := EvaluateAssertions(stdout2, nil, 0, expect, nil)

	require.Len(t, results1, 1)
	require.Len(t, results2, 1)

	assert.True(t, results1[0].Passed, "first call: status is ok, should pass")
	assert.False(t, results2[0].Passed, "second call: status is fail, should not pass")
}

func TestEvaluateAssertions_AntiHardcoding_DifferentExitCodes(t *testing.T) {
	expect := Expectation{ExitCode: intPtr(0)}

	r0 := EvaluateAssertions(nil, nil, 0, expect, nil)
	r1 := EvaluateAssertions(nil, nil, 1, expect, nil)

	require.Len(t, r0, 1)
	require.Len(t, r1, 1)

	assert.True(t, r0[0].Passed, "exit code 0 expected, got 0: pass")
	assert.False(t, r1[0].Passed, "exit code 0 expected, got 1: fail")
	assert.NotEqual(t, r0[0].Actual, r1[0].Actual,
		"Actual field must differ for different actual exit codes")
}

// ---------------------------------------------------------------------------
// Order independence: results should cover all assertions regardless of order
// ---------------------------------------------------------------------------

func TestEvaluateAssertions_AllAssertionsCovered(t *testing.T) {
	stdout := []byte(`{
		"name": "test",
		"tags": ["a", "b"],
		"version": "1.0.0",
		"active": true,
		"items": [1, 2]
	}`)

	expect := Expectation{
		StdoutContains: []Assertion{
			{Path: "$.name", Equals: "test"},
			{Path: "$.tags", Contains: "a"},
			{Path: "$.version", Matches: `^\d`},
			{Path: "$.active", Exists: boolPtr(true)},
			{Path: "$.items", Length: intPtr(2)},
		},
	}

	results := EvaluateAssertions(stdout, nil, 0, expect, nil)

	require.Len(t, results, 5, "all 5 stdout_contains assertions must produce results")

	// Each path must appear in results
	paths := make(map[string]bool)
	for _, r := range results {
		paths[r.Path] = true
		assert.True(t, r.Passed,
			"assertion for path %s must pass: expected=%s actual=%s error=%s",
			r.Path, r.Expected, r.Actual, r.Error)
	}

	assert.True(t, paths["$.name"], "$.name must appear in results")
	assert.True(t, paths["$.tags"], "$.tags must appear in results")
	assert.True(t, paths["$.version"], "$.version must appear in results")
	assert.True(t, paths["$.active"], "$.active must appear in results")
	assert.True(t, paths["$.items"], "$.items must appear in results")
}
