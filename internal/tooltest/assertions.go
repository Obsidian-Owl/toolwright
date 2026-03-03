package tooltest

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"regexp"
	"strings"

	"github.com/ohler55/ojg/jp"
	"github.com/ohler55/ojg/oj"

	"github.com/Obsidian-Owl/toolwright/internal/schema"
)

// AssertionResult represents the outcome of a single assertion check.
type AssertionResult struct {
	Type     string // "exit_code", "stdout_is_json", "stdout_schema", "stdout_contains", "stderr_contains"
	Path     string // JSONPath for stdout_contains, empty for others
	Passed   bool
	Expected string // human-readable expected value
	Actual   string // human-readable actual value
	Error    string // error message if assertion errored (e.g., invalid regex)
}

// EvaluateAssertions evaluates all assertions from an Expectation and returns
// individual results in the order: exit_code, stdout_is_json, stdout_schema,
// stdout_contains (one per assertion), stderr_contains (one per string).
func EvaluateAssertions(stdout []byte, stderr []byte, exitCode int, expect Expectation, schemaFS fs.FS) []AssertionResult {
	var results []AssertionResult

	// exit_code
	if expect.ExitCode != nil {
		results = append(results, evalExitCode(*expect.ExitCode, exitCode))
	}

	// stdout_is_json
	if expect.StdoutIsJSON != nil {
		results = append(results, evalStdoutIsJSON(*expect.StdoutIsJSON, stdout))
	}

	// stdout_schema
	if expect.StdoutSchema != "" {
		results = append(results, evalStdoutSchema(expect.StdoutSchema, stdout, schemaFS))
	}

	// stdout_contains
	for _, assertion := range expect.StdoutContains {
		results = append(results, evalStdoutContains(assertion, stdout))
	}

	// stderr_contains
	for _, substr := range expect.StderrContains {
		results = append(results, evalStderrContains(substr, stderr))
	}

	return results
}

func evalExitCode(expected, actual int) AssertionResult {
	passed := expected == actual
	return AssertionResult{
		Type:     "exit_code",
		Passed:   passed,
		Expected: fmt.Sprintf("%d", expected),
		Actual:   fmt.Sprintf("%d", actual),
	}
}

func evalStdoutIsJSON(wantJSON bool, stdout []byte) AssertionResult {
	isJSON := json.Valid(stdout) && len(stdout) > 0
	passed := wantJSON == isJSON

	var expected, actual string
	if wantJSON {
		expected = "valid JSON"
	} else {
		expected = "not valid JSON"
	}
	if isJSON {
		actual = "valid JSON"
	} else {
		actual = "not valid JSON"
	}

	return AssertionResult{
		Type:     "stdout_is_json",
		Passed:   passed,
		Expected: expected,
		Actual:   actual,
	}
}

func evalStdoutSchema(schemaPath string, stdout []byte, schemaFS fs.FS) AssertionResult {
	if schemaFS == nil {
		return AssertionResult{
			Type:   "stdout_schema",
			Passed: false,
			Error:  "schemaFS is nil: cannot validate schema",
		}
	}

	v := schema.NewValidator(schemaFS)
	err := v.Validate(schemaPath, stdout)
	if err != nil {
		return AssertionResult{
			Type:   "stdout_schema",
			Passed: false,
			Actual: err.Error(),
			Error:  err.Error(),
		}
	}

	return AssertionResult{
		Type:     "stdout_schema",
		Passed:   true,
		Expected: fmt.Sprintf("valid against schema %s", schemaPath),
		Actual:   fmt.Sprintf("valid against schema %s", schemaPath),
	}
}

func evalStdoutContains(assertion Assertion, stdout []byte) AssertionResult {
	base := AssertionResult{
		Type: "stdout_contains",
		Path: assertion.Path,
	}

	// Parse the JSON
	parsed, err := oj.Parse(stdout)
	if err != nil {
		base.Error = fmt.Errorf("parse stdout as JSON: %w", err).Error()
		base.Actual = base.Error
		base.Passed = false
		return base
	}

	// Parse the JSONPath expression
	expr, err := jp.ParseString(assertion.Path)
	if err != nil {
		base.Error = fmt.Errorf("parse JSONPath %q: %w", assertion.Path, err).Error()
		base.Passed = false
		return base
	}

	// Evaluate the expression
	jpResults := expr.Get(parsed)

	// Determine which operator to apply
	switch {
	case assertion.Exists != nil:
		return evalExists(base, assertion, jpResults)
	case assertion.Length != nil:
		return evalLength(base, assertion, jpResults)
	case assertion.Matches != "":
		return evalMatches(base, assertion, jpResults)
	case assertion.Contains != nil:
		return evalContains(base, assertion, jpResults)
	default:
		// equals (also handles nil equals for null comparison)
		return evalEquals(base, assertion, jpResults)
	}
}

func evalEquals(base AssertionResult, assertion Assertion, jpResults []any) AssertionResult {
	if len(jpResults) == 0 {
		base.Passed = false
		base.Expected = fmt.Sprintf("%v", assertion.Equals)
		base.Actual = "<path not found>"
		return base
	}

	actual := jpResults[0]

	// Handle nil/null case: both expected and actual are nil
	if assertion.Equals == nil && actual == nil {
		base.Passed = true
		base.Expected = "<null>"
		base.Actual = "<null>"
		return base
	}
	if assertion.Equals == nil || actual == nil {
		base.Passed = false
		base.Expected = fmt.Sprintf("%v", assertion.Equals)
		base.Actual = fmt.Sprintf("%v", actual)
		return base
	}

	// Normalize using fmt.Sprintf to handle int vs int64 vs float64 differences
	expectedStr := fmt.Sprintf("%v", assertion.Equals)
	actualStr := fmt.Sprintf("%v", actual)
	base.Expected = expectedStr
	base.Actual = actualStr
	base.Passed = expectedStr == actualStr
	return base
}

func evalContains(base AssertionResult, assertion Assertion, jpResults []any) AssertionResult {
	base.Expected = fmt.Sprintf("contains %v", assertion.Contains)

	if len(jpResults) == 0 {
		base.Passed = false
		base.Actual = "<path not found>"
		return base
	}

	// If there's one result and it's a slice, check within that slice (e.g., $.tags)
	if len(jpResults) == 1 {
		if innerSlice, ok := jpResults[0].([]any); ok {
			// Check if any element of the inner slice matches
			for _, elem := range innerSlice {
				if valuesEqual(elem, assertion.Contains) {
					base.Passed = true
					base.Actual = fmt.Sprintf("%v", jpResults[0])
					return base
				}
			}
			base.Passed = false
			base.Actual = fmt.Sprintf("%v", jpResults[0])
			return base
		}

		// Single non-slice result: check substring if both are strings
		if str, ok := jpResults[0].(string); ok {
			if containsStr, ok := assertion.Contains.(string); ok {
				base.Passed = strings.Contains(str, containsStr)
				base.Actual = str
				return base
			}
		}

		// Single non-slice, non-string: check equality
		base.Passed = valuesEqual(jpResults[0], assertion.Contains)
		base.Actual = fmt.Sprintf("%v", jpResults[0])
		return base
	}

	// Multiple results (wildcard path like $.findings[*].type): check each element
	for _, elem := range jpResults {
		if valuesEqual(elem, assertion.Contains) {
			base.Passed = true
			base.Actual = fmt.Sprintf("%v", jpResults)
			return base
		}
	}
	base.Passed = false
	base.Actual = fmt.Sprintf("%v", jpResults)
	return base
}

func evalMatches(base AssertionResult, assertion Assertion, jpResults []any) AssertionResult {
	re, err := regexp.Compile(assertion.Matches)
	if err != nil {
		base.Passed = false
		base.Error = fmt.Errorf("compile regex %q: %w", assertion.Matches, err).Error()
		return base
	}

	base.Expected = fmt.Sprintf("matches %s", assertion.Matches)

	if len(jpResults) == 0 {
		base.Passed = false
		base.Actual = "<path not found>"
		return base
	}

	str := fmt.Sprintf("%v", jpResults[0])
	base.Actual = str
	base.Passed = re.MatchString(str)
	return base
}

func evalExists(base AssertionResult, assertion Assertion, jpResults []any) AssertionResult {
	found := len(jpResults) > 0
	wantExists := *assertion.Exists
	base.Passed = found == wantExists

	if wantExists {
		base.Expected = "field exists"
	} else {
		base.Expected = "field does not exist"
	}
	if found {
		base.Actual = "field exists"
	} else {
		base.Actual = "field does not exist"
	}
	return base
}

func evalLength(base AssertionResult, assertion Assertion, jpResults []any) AssertionResult {
	expectedLen := *assertion.Length
	base.Expected = fmt.Sprintf("length %d", expectedLen)

	if len(jpResults) == 0 {
		base.Passed = false
		base.Actual = "<path not found>"
		return base
	}

	// Multiple results (wildcard): use len(jpResults)
	if len(jpResults) > 1 {
		actualLen := len(jpResults)
		base.Actual = fmt.Sprintf("length %d", actualLen)
		base.Passed = actualLen == expectedLen
		return base
	}

	// Single result: check if it's a slice or a string
	val := jpResults[0]
	switch v := val.(type) {
	case []any:
		actualLen := len(v)
		base.Actual = fmt.Sprintf("length %d", actualLen)
		base.Passed = actualLen == expectedLen
	case string:
		actualLen := len(v)
		base.Actual = fmt.Sprintf("length %d", actualLen)
		base.Passed = actualLen == expectedLen
	default:
		base.Passed = false
		base.Actual = fmt.Sprintf("unsupported type for length: %T", val)
	}
	return base
}

func evalStderrContains(substr string, stderr []byte) AssertionResult {
	stderrStr := string(stderr)
	passed := strings.Contains(stderrStr, substr)
	return AssertionResult{
		Type:     "stderr_contains",
		Passed:   passed,
		Expected: fmt.Sprintf("contains %q", substr),
		Actual:   stderrStr,
	}
}

// valuesEqual compares two values with numeric normalization.
// It uses fmt.Sprintf for cross-type comparison (e.g., int vs int64).
func valuesEqual(a, b any) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}
