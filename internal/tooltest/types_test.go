package tooltest

import (
	"bytes"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Test fixtures
// ---------------------------------------------------------------------------

// intPtr returns a pointer to the given int.
func intPtr(v int) *int { return &v }

// boolPtr returns a pointer to the given bool.
func boolPtr(v bool) *bool { return &v }

// fullTestSuite returns a TestSuite with every field populated at every level.
// A sloppy implementation that drops or renames any field will be caught.
func fullTestSuite() TestSuite {
	return TestSuite{
		Tool: "my-tool",
		Tests: []TestCase{
			{
				Name:      "basic invocation",
				Args:      []string{"arg1", "arg2"},
				Flags:     map[string]string{"verbose": "true", "output": "json"},
				AuthToken: "tok-secret-123",
				Expect: Expectation{
					ExitCode:     intPtr(0),
					StdoutIsJSON: boolPtr(true),
					StdoutSchema: "schemas/output.json",
					StdoutContains: []Assertion{
						{Path: "$.name", Equals: "fido"},
						{Path: "$.tags", Contains: "friendly"},
						{Path: "$.id", Matches: `^\d+$`},
						{Path: "$.active", Exists: boolPtr(true)},
						{Path: "$.items", Length: intPtr(3)},
					},
					StderrContains: []string{"warning: deprecated"},
				},
				Timeout: 30 * time.Second,
			},
			{
				Name:      "error case",
				Args:      []string{"--bad-flag"},
				Flags:     nil,
				AuthToken: "",
				Expect: Expectation{
					ExitCode:     intPtr(1),
					StdoutIsJSON: boolPtr(false),
				},
				Timeout: 5 * time.Second,
			},
		},
	}
}

// fullTestReport returns a TestReport with mixed pass/fail results.
func fullTestReport() TestReport {
	return TestReport{
		Tool:   "my-tool",
		Total:  3,
		Passed: 2,
		Failed: 1,
		Results: []TestResult{
			{
				Name:     "test-pass-1",
				Status:   "pass",
				Duration: 150 * time.Millisecond,
				Error:    "",
				Stdout:   []byte(`{"ok": true}`),
				Stderr:   nil,
			},
			{
				Name:     "test-pass-2",
				Status:   "pass",
				Duration: 200 * time.Millisecond,
				Error:    "",
				Stdout:   []byte("success"),
				Stderr:   []byte(""),
			},
			{
				Name:     "test-fail-1",
				Status:   "fail",
				Duration: 50 * time.Millisecond,
				Error:    "expected exit code 0 but got 1",
				Stdout:   nil,
				Stderr:   []byte("fatal: something went wrong"),
			},
		},
	}
}

// ---------------------------------------------------------------------------
// 1. TestSuite: construct with all fields populated
// ---------------------------------------------------------------------------

func TestTestSuite_ConstructWithAllFields(t *testing.T) {
	ts := fullTestSuite()

	assert.Equal(t, "my-tool", ts.Tool,
		"Tool field must hold the assigned value")
	require.Len(t, ts.Tests, 2,
		"Tests slice must hold exactly 2 test cases")

	// Verify first test case fields in detail to catch partial implementations.
	tc := ts.Tests[0]
	assert.Equal(t, "basic invocation", tc.Name)
	assert.Equal(t, []string{"arg1", "arg2"}, tc.Args)
	assert.Equal(t, map[string]string{"verbose": "true", "output": "json"}, tc.Flags)
	assert.Equal(t, "tok-secret-123", tc.AuthToken)
	assert.Equal(t, 30*time.Second, tc.Timeout)

	// Verify expectation fields.
	ex := tc.Expect
	require.NotNil(t, ex.ExitCode, "ExitCode pointer must not be nil")
	assert.Equal(t, 0, *ex.ExitCode)
	require.NotNil(t, ex.StdoutIsJSON, "StdoutIsJSON pointer must not be nil")
	assert.True(t, *ex.StdoutIsJSON)
	assert.Equal(t, "schemas/output.json", ex.StdoutSchema)
	require.Len(t, ex.StdoutContains, 5, "StdoutContains must have 5 assertions")
	assert.Equal(t, []string{"warning: deprecated"}, ex.StderrContains)

	// Verify second test case differs from the first (anti-hardcoding).
	tc2 := ts.Tests[1]
	assert.Equal(t, "error case", tc2.Name)
	assert.NotEqual(t, tc.Name, tc2.Name,
		"multiple test cases must have distinct names")
	require.NotNil(t, tc2.Expect.ExitCode)
	assert.Equal(t, 1, *tc2.Expect.ExitCode,
		"second test case must have exit code 1, not 0")
}

// ---------------------------------------------------------------------------
// 2. TestCase: zero value has correct empty/nil defaults
// ---------------------------------------------------------------------------

func TestTestCase_ZeroValue(t *testing.T) {
	var tc TestCase

	assert.Empty(t, tc.Name, "zero-value Name should be empty string")
	assert.Nil(t, tc.Args, "zero-value Args should be nil")
	assert.Nil(t, tc.Flags, "zero-value Flags should be nil")
	assert.Empty(t, tc.AuthToken, "zero-value AuthToken should be empty string")
	assert.Equal(t, time.Duration(0), tc.Timeout,
		"zero-value Timeout should be zero duration")

	// Expectation zero value.
	assert.Nil(t, tc.Expect.ExitCode, "zero-value ExitCode should be nil pointer")
	assert.Nil(t, tc.Expect.StdoutIsJSON, "zero-value StdoutIsJSON should be nil pointer")
	assert.Empty(t, tc.Expect.StdoutSchema, "zero-value StdoutSchema should be empty string")
	assert.Nil(t, tc.Expect.StdoutContains, "zero-value StdoutContains should be nil")
	assert.Nil(t, tc.Expect.StderrContains, "zero-value StderrContains should be nil")
}

// ---------------------------------------------------------------------------
// 3. Expectation: all assertion types populated simultaneously
// ---------------------------------------------------------------------------

func TestExpectation_AllAssertionTypesPopulated(t *testing.T) {
	ex := Expectation{
		ExitCode:     intPtr(0),
		StdoutIsJSON: boolPtr(true),
		StdoutSchema: "schema.json",
		StdoutContains: []Assertion{
			{Path: "$.a", Equals: "hello"},
			{Path: "$.b", Contains: 42},
			{Path: "$.c", Matches: `^foo\d+$`},
			{Path: "$.d", Exists: boolPtr(true)},
			{Path: "$.e", Length: intPtr(10)},
		},
		StderrContains: []string{"warn", "error"},
	}

	// Verify pointer fields are non-nil and hold correct values.
	require.NotNil(t, ex.ExitCode)
	assert.Equal(t, 0, *ex.ExitCode)
	require.NotNil(t, ex.StdoutIsJSON)
	assert.True(t, *ex.StdoutIsJSON)
	assert.Equal(t, "schema.json", ex.StdoutSchema)

	// Verify each assertion has the correct operator value.
	require.Len(t, ex.StdoutContains, 5)

	equals := ex.StdoutContains[0]
	assert.Equal(t, "$.a", equals.Path)
	assert.Equal(t, "hello", equals.Equals)
	assert.Nil(t, equals.Contains, "Equals assertion should not have Contains set")
	assert.Empty(t, equals.Matches, "Equals assertion should not have Matches set")
	assert.Nil(t, equals.Exists, "Equals assertion should not have Exists set")
	assert.Nil(t, equals.Length, "Equals assertion should not have Length set")

	contains := ex.StdoutContains[1]
	assert.Equal(t, "$.b", contains.Path)
	assert.Equal(t, 42, contains.Contains)
	assert.Nil(t, contains.Equals, "Contains assertion should not have Equals set")

	matches := ex.StdoutContains[2]
	assert.Equal(t, "$.c", matches.Path)
	assert.Equal(t, `^foo\d+$`, matches.Matches)
	assert.Nil(t, matches.Equals, "Matches assertion should not have Equals set")

	exists := ex.StdoutContains[3]
	assert.Equal(t, "$.d", exists.Path)
	require.NotNil(t, exists.Exists)
	assert.True(t, *exists.Exists)

	length := ex.StdoutContains[4]
	assert.Equal(t, "$.e", length.Path)
	require.NotNil(t, length.Length)
	assert.Equal(t, 10, *length.Length)

	// StderrContains must preserve both entries in order.
	assert.Equal(t, []string{"warn", "error"}, ex.StderrContains)
}

// ---------------------------------------------------------------------------
// 4. TestResult: status values
// ---------------------------------------------------------------------------

func TestTestResult_StatusPass(t *testing.T) {
	r := TestResult{
		Name:   "passing-test",
		Status: "pass",
	}
	assert.Equal(t, "passing-test", r.Name)
	assert.Equal(t, "pass", r.Status,
		"Status must hold the exact string 'pass'")
}

func TestTestResult_StatusFail(t *testing.T) {
	r := TestResult{
		Name:   "failing-test",
		Status: "fail",
		Error:  "assertion failed: expected 0 got 1",
	}
	assert.Equal(t, "failing-test", r.Name)
	assert.Equal(t, "fail", r.Status,
		"Status must hold the exact string 'fail'")
	assert.NotEmpty(t, r.Error,
		"a failing test result should carry an error message")
}

func TestTestResult_StatusDistinct(t *testing.T) {
	// Guard against an implementation that maps both to the same value.
	pass := TestResult{Status: "pass"}
	fail := TestResult{Status: "fail"}
	assert.NotEqual(t, pass.Status, fail.Status,
		"'pass' and 'fail' must be distinct status values")
}

// ---------------------------------------------------------------------------
// 5. TestReport: computed field invariants
// ---------------------------------------------------------------------------

func TestTestReport_TotalEqualsResultsLength(t *testing.T) {
	report := fullTestReport()

	assert.Equal(t, len(report.Results), report.Total,
		"Total must equal len(Results)")
}

func TestTestReport_PassedPlusFailedEqualsTotal(t *testing.T) {
	report := fullTestReport()

	assert.Equal(t, report.Total, report.Passed+report.Failed,
		"Passed + Failed must equal Total")
}

func TestTestReport_FieldValuesMatchFixture(t *testing.T) {
	report := fullTestReport()

	assert.Equal(t, "my-tool", report.Tool)
	assert.Equal(t, 3, report.Total)
	assert.Equal(t, 2, report.Passed)
	assert.Equal(t, 1, report.Failed)
	require.Len(t, report.Results, 3)

	// Verify each result's status is exactly "pass" or "fail".
	for _, r := range report.Results {
		assert.Contains(t, []string{"pass", "fail"}, r.Status,
			"Result %q has invalid status %q", r.Name, r.Status)
	}
}

func TestTestReport_CountsConsistentWithResults(t *testing.T) {
	report := fullTestReport()

	var passCount, failCount int
	for _, r := range report.Results {
		switch r.Status {
		case "pass":
			passCount++
		case "fail":
			failCount++
		default:
			t.Errorf("unexpected status %q in result %q", r.Status, r.Name)
		}
	}
	assert.Equal(t, report.Passed, passCount,
		"Passed field must match actual number of pass results")
	assert.Equal(t, report.Failed, failCount,
		"Failed field must match actual number of fail results")
}

func TestTestReport_ZeroValue(t *testing.T) {
	var report TestReport

	assert.Empty(t, report.Tool, "zero-value Tool should be empty string")
	assert.Equal(t, 0, report.Total, "zero-value Total should be 0")
	assert.Equal(t, 0, report.Passed, "zero-value Passed should be 0")
	assert.Equal(t, 0, report.Failed, "zero-value Failed should be 0")
	assert.Nil(t, report.Results, "zero-value Results should be nil")
}

// ---------------------------------------------------------------------------
// 6. Assertion: each operator individually
// ---------------------------------------------------------------------------

func TestAssertion_EqualsOnly(t *testing.T) {
	a := Assertion{
		Path:   "$.name",
		Equals: "fido",
	}
	assert.Equal(t, "$.name", a.Path)
	assert.Equal(t, "fido", a.Equals)
	assert.Nil(t, a.Contains, "unused operator must be nil")
	assert.Empty(t, a.Matches, "unused operator must be empty")
	assert.Nil(t, a.Exists, "unused operator must be nil")
	assert.Nil(t, a.Length, "unused operator must be nil")
}

func TestAssertion_ContainsOnly(t *testing.T) {
	a := Assertion{
		Path:     "$.tags",
		Contains: "friendly",
	}
	assert.Equal(t, "$.tags", a.Path)
	assert.Equal(t, "friendly", a.Contains)
	assert.Nil(t, a.Equals, "unused operator must be nil")
	assert.Empty(t, a.Matches, "unused operator must be empty")
	assert.Nil(t, a.Exists, "unused operator must be nil")
	assert.Nil(t, a.Length, "unused operator must be nil")
}

func TestAssertion_MatchesOnly(t *testing.T) {
	a := Assertion{
		Path:    "$.id",
		Matches: `^\d{4}-\d{2}-\d{2}$`,
	}
	assert.Equal(t, "$.id", a.Path)
	assert.Equal(t, `^\d{4}-\d{2}-\d{2}$`, a.Matches)
	assert.Nil(t, a.Equals, "unused operator must be nil")
	assert.Nil(t, a.Contains, "unused operator must be nil")
	assert.Nil(t, a.Exists, "unused operator must be nil")
	assert.Nil(t, a.Length, "unused operator must be nil")
}

func TestAssertion_ExistsOnly_True(t *testing.T) {
	a := Assertion{
		Path:   "$.field",
		Exists: boolPtr(true),
	}
	assert.Equal(t, "$.field", a.Path)
	require.NotNil(t, a.Exists, "Exists must be non-nil pointer")
	assert.True(t, *a.Exists)
	assert.Nil(t, a.Equals, "unused operator must be nil")
	assert.Nil(t, a.Contains, "unused operator must be nil")
	assert.Empty(t, a.Matches, "unused operator must be empty")
	assert.Nil(t, a.Length, "unused operator must be nil")
}

func TestAssertion_ExistsOnly_False(t *testing.T) {
	a := Assertion{
		Path:   "$.missing",
		Exists: boolPtr(false),
	}
	assert.Equal(t, "$.missing", a.Path)
	require.NotNil(t, a.Exists, "Exists must be non-nil pointer")
	assert.False(t, *a.Exists,
		"Exists=false must be distinguishable from Exists=nil")
}

func TestAssertion_LengthOnly(t *testing.T) {
	a := Assertion{
		Path:   "$.items",
		Length: intPtr(7),
	}
	assert.Equal(t, "$.items", a.Path)
	require.NotNil(t, a.Length, "Length must be non-nil pointer")
	assert.Equal(t, 7, *a.Length)
	assert.Nil(t, a.Equals, "unused operator must be nil")
	assert.Nil(t, a.Contains, "unused operator must be nil")
	assert.Empty(t, a.Matches, "unused operator must be empty")
	assert.Nil(t, a.Exists, "unused operator must be nil")
}

func TestAssertion_LengthZero(t *testing.T) {
	// Length=0 is meaningful (empty array). Must be distinguishable from nil.
	a := Assertion{
		Path:   "$.items",
		Length: intPtr(0),
	}
	assert.Equal(t, "$.items", a.Path)
	require.NotNil(t, a.Length, "Length pointer must be non-nil even when value is 0")
	assert.Equal(t, 0, *a.Length,
		"Length=0 must store the value 0, not be confused with nil")
}

func TestAssertion_EqualsAny_NumericValue(t *testing.T) {
	// Equals is typed `any`. Verify it holds non-string values correctly.
	a := Assertion{
		Path:   "$.count",
		Equals: 42,
	}
	assert.Equal(t, "$.count", a.Path)
	assert.Equal(t, 42, a.Equals,
		"Equals must hold an int value without type coercion")
}

func TestAssertion_EqualsAny_BoolValue(t *testing.T) {
	a := Assertion{
		Path:   "$.active",
		Equals: true,
	}
	assert.Equal(t, "$.active", a.Path)
	assert.Equal(t, true, a.Equals,
		"Equals must hold a bool value without type coercion")
}

func TestAssertion_EqualsAny_NilValue(t *testing.T) {
	// Equals=nil means "assert the value is null in JSON".
	// This must be distinguishable from Equals not being set at all
	// when used with a non-zero-value implementation.
	a := Assertion{
		Path:   "$.deleted",
		Equals: nil,
	}
	assert.Equal(t, "$.deleted", a.Path)
	assert.Nil(t, a.Equals,
		"Equals=nil must be representable")
}

func TestAssertion_ContainsAny_SliceValue(t *testing.T) {
	// Contains is typed `any`. Verify it holds a slice.
	a := Assertion{
		Path:     "$.data",
		Contains: []string{"a", "b"},
	}
	assert.Equal(t, "$.data", a.Path)
	assert.Equal(t, []string{"a", "b"}, a.Contains,
		"Contains must hold a slice value")
}

// ---------------------------------------------------------------------------
// 7. TestSuite: empty Tests slice (not nil)
// ---------------------------------------------------------------------------

func TestTestSuite_EmptyTestsSlice_NotNil(t *testing.T) {
	ts := TestSuite{
		Tool:  "no-tests-tool",
		Tests: []TestCase{},
	}

	assert.Equal(t, "no-tests-tool", ts.Tool)
	require.NotNil(t, ts.Tests,
		"explicitly initialized empty slice must not be nil")
	assert.Len(t, ts.Tests, 0,
		"empty Tests slice must have length 0")
}

func TestTestSuite_NilTestsSlice(t *testing.T) {
	// Nil Tests is distinct from empty Tests.
	ts := TestSuite{
		Tool:  "nil-tests-tool",
		Tests: nil,
	}

	assert.Equal(t, "nil-tests-tool", ts.Tool)
	assert.Nil(t, ts.Tests,
		"nil Tests must be nil, not an empty slice")
}

func TestTestSuite_EmptyVsNilTestsDistinct(t *testing.T) {
	empty := TestSuite{Tool: "a", Tests: []TestCase{}}
	nilTS := TestSuite{Tool: "b", Tests: nil}

	assert.Equal(t, "a", empty.Tool)
	assert.Equal(t, "b", nilTS.Tool)

	// Both have zero length, but they are structurally different.
	assert.Len(t, empty.Tests, 0)
	assert.Len(t, nilTS.Tests, 0)

	// The nil check distinguishes them.
	assert.NotNil(t, empty.Tests, "empty slice is not nil")
	assert.Nil(t, nilTS.Tests, "nil slice is nil")
}

// ---------------------------------------------------------------------------
// 8. Multiple TestCases with different Timeout values
// ---------------------------------------------------------------------------

func TestTestCase_DifferentTimeouts(t *testing.T) {
	tests := []struct {
		name    string
		timeout time.Duration
	}{
		{"zero timeout", 0},
		{"1 second", 1 * time.Second},
		{"30 seconds", 30 * time.Second},
		{"5 minutes", 5 * time.Minute},
		{"sub-second", 500 * time.Millisecond},
		{"large timeout", 10 * time.Minute},
	}

	cases := make([]TestCase, len(tests))
	for i, tt := range tests {
		cases[i] = TestCase{
			Name:    tt.name,
			Timeout: tt.timeout,
		}
	}

	// Verify each timeout is stored distinctly and retrievable.
	for i, tt := range tests {
		assert.Equal(t, tt.timeout, cases[i].Timeout,
			"TestCase[%d] timeout must be %v", i, tt.timeout)
	}

	// Anti-hardcoding: verify not all timeouts are the same.
	seen := make(map[time.Duration]bool)
	for _, tc := range cases {
		seen[tc.Timeout] = true
	}
	assert.Equal(t, len(tests), len(seen),
		"all timeout values must be distinct")
}

func TestTestCase_TimeoutIsDuration(t *testing.T) {
	// Verify Timeout is time.Duration (not int, int64, or float).
	tc := TestCase{
		Timeout: 2*time.Second + 500*time.Millisecond,
	}
	assert.Equal(t, 2500*time.Millisecond, tc.Timeout,
		"Timeout must support sub-second precision via time.Duration")
}

// ---------------------------------------------------------------------------
// 9. TestResult: large stdout/stderr (not truncated)
// ---------------------------------------------------------------------------

func TestTestResult_LargeStdout(t *testing.T) {
	// 1MB of data to verify no truncation.
	largeOutput := bytes.Repeat([]byte("x"), 1024*1024)

	r := TestResult{
		Name:   "large-output",
		Status: "pass",
		Stdout: largeOutput,
	}

	assert.Equal(t, "large-output", r.Name)
	assert.Equal(t, "pass", r.Status)
	assert.Len(t, r.Stdout, 1024*1024,
		"Stdout must not be truncated; expected 1MB")
	assert.Equal(t, largeOutput, r.Stdout,
		"Stdout content must be preserved byte-for-byte")
}

func TestTestResult_LargeStderr(t *testing.T) {
	largeErr := bytes.Repeat([]byte("E"), 512*1024)

	r := TestResult{
		Name:   "large-stderr",
		Status: "fail",
		Stderr: largeErr,
	}

	assert.Equal(t, "large-stderr", r.Name)
	assert.Equal(t, "fail", r.Status)
	assert.Len(t, r.Stderr, 512*1024,
		"Stderr must not be truncated; expected 512KB")
	assert.Equal(t, largeErr, r.Stderr,
		"Stderr content must be preserved byte-for-byte")
}

func TestTestResult_BinaryStdout(t *testing.T) {
	// Stdout is []byte, so it must handle arbitrary binary content
	// including null bytes and non-UTF8 sequences.
	binary := []byte{0x00, 0xFF, 0x01, 0xFE, 0x00, 0x80}

	r := TestResult{
		Name:   "binary-output",
		Status: "pass",
		Stdout: binary,
	}

	assert.Equal(t, "binary-output", r.Name)
	assert.Equal(t, "pass", r.Status)
	assert.Equal(t, binary, r.Stdout,
		"Stdout must preserve arbitrary binary content including null bytes")
}

func TestTestResult_NilVsEmptyStdout(t *testing.T) {
	nilResult := TestResult{Stdout: nil}
	emptyResult := TestResult{Stdout: []byte{}}

	assert.Nil(t, nilResult.Stdout, "nil Stdout must remain nil")
	assert.NotNil(t, emptyResult.Stdout, "empty Stdout must not be nil")
	assert.Len(t, emptyResult.Stdout, 0, "empty Stdout must have length 0")
}

// ---------------------------------------------------------------------------
// 10. Pointer fields (*int, *bool): nil vs non-nil distinction
// ---------------------------------------------------------------------------

func TestExpectation_ExitCode_NilVsZero(t *testing.T) {
	// nil means "don't check exit code".
	// *int(0) means "assert exit code is 0".
	// These MUST be distinguishable.
	noCheck := Expectation{ExitCode: nil}
	checkZero := Expectation{ExitCode: intPtr(0)}
	checkOne := Expectation{ExitCode: intPtr(1)}

	assert.Nil(t, noCheck.ExitCode,
		"nil ExitCode means no assertion on exit code")
	require.NotNil(t, checkZero.ExitCode,
		"*int(0) ExitCode must be non-nil")
	assert.Equal(t, 0, *checkZero.ExitCode,
		"*int(0) ExitCode must dereference to 0")
	require.NotNil(t, checkOne.ExitCode)
	assert.Equal(t, 1, *checkOne.ExitCode)

	// The three cases must be distinguishable from each other.
	assert.NotEqual(t, noCheck.ExitCode, checkZero.ExitCode,
		"nil and *0 must be different")
	assert.NotEqual(t, *checkZero.ExitCode, *checkOne.ExitCode,
		"*0 and *1 must be different")
}

func TestExpectation_StdoutIsJSON_NilVsFalse(t *testing.T) {
	// nil means "don't check JSON-ness".
	// *bool(false) means "assert output is NOT JSON".
	// These MUST be distinguishable.
	noCheck := Expectation{StdoutIsJSON: nil}
	checkFalse := Expectation{StdoutIsJSON: boolPtr(false)}
	checkTrue := Expectation{StdoutIsJSON: boolPtr(true)}

	assert.Nil(t, noCheck.StdoutIsJSON,
		"nil StdoutIsJSON means no assertion")
	require.NotNil(t, checkFalse.StdoutIsJSON,
		"*bool(false) must be non-nil")
	assert.False(t, *checkFalse.StdoutIsJSON,
		"*bool(false) must dereference to false")
	require.NotNil(t, checkTrue.StdoutIsJSON)
	assert.True(t, *checkTrue.StdoutIsJSON)
}

func TestAssertion_Exists_NilVsFalse(t *testing.T) {
	// nil means "don't check existence".
	// *bool(false) means "assert field does NOT exist".
	noCheck := Assertion{Path: "$.x", Exists: nil}
	mustNotExist := Assertion{Path: "$.x", Exists: boolPtr(false)}
	mustExist := Assertion{Path: "$.x", Exists: boolPtr(true)}

	assert.Equal(t, "$.x", noCheck.Path)
	assert.Equal(t, "$.x", mustNotExist.Path)
	assert.Equal(t, "$.x", mustExist.Path)
	assert.Nil(t, noCheck.Exists)
	require.NotNil(t, mustNotExist.Exists)
	assert.False(t, *mustNotExist.Exists)
	require.NotNil(t, mustExist.Exists)
	assert.True(t, *mustExist.Exists)
}

func TestAssertion_Length_NilVsZero(t *testing.T) {
	// nil means "don't check length".
	// *int(0) means "assert length is 0" (empty array/string).
	noCheck := Assertion{Path: "$.x", Length: nil}
	checkZero := Assertion{Path: "$.x", Length: intPtr(0)}
	checkFive := Assertion{Path: "$.x", Length: intPtr(5)}

	assert.Equal(t, "$.x", noCheck.Path)
	assert.Equal(t, "$.x", checkZero.Path)
	assert.Equal(t, "$.x", checkFive.Path)
	assert.Nil(t, noCheck.Length)
	require.NotNil(t, checkZero.Length)
	assert.Equal(t, 0, *checkZero.Length,
		"*int(0) must dereference to 0, not be treated as nil")
	require.NotNil(t, checkFive.Length)
	assert.Equal(t, 5, *checkFive.Length)
}

func TestExpectation_ExitCode_NegativeValue(t *testing.T) {
	// Exit codes can be negative on some systems (signal-based).
	ex := Expectation{ExitCode: intPtr(-1)}
	require.NotNil(t, ex.ExitCode)
	assert.Equal(t, -1, *ex.ExitCode,
		"ExitCode must support negative values")
}

func TestExpectation_ExitCode_LargeValue(t *testing.T) {
	// Some tools return exit codes > 128 (128 + signal number).
	ex := Expectation{ExitCode: intPtr(137)}
	require.NotNil(t, ex.ExitCode)
	assert.Equal(t, 137, *ex.ExitCode,
		"ExitCode must support values > 128")
}

// ---------------------------------------------------------------------------
// Structural equality: go-cmp tests
// ---------------------------------------------------------------------------

func TestTestSuite_CmpEqual(t *testing.T) {
	a := fullTestSuite()
	b := fullTestSuite()

	if diff := cmp.Diff(a, b); diff != "" {
		t.Errorf("two independently constructed fullTestSuite() values must be equal (-a +b):\n%s", diff)
	}
}

func TestTestReport_CmpEqual(t *testing.T) {
	a := fullTestReport()
	b := fullTestReport()

	if diff := cmp.Diff(a, b); diff != "" {
		t.Errorf("two independently constructed fullTestReport() values must be equal (-a +b):\n%s", diff)
	}
}

func TestTestSuite_CmpNotEqual_DifferentTool(t *testing.T) {
	a := fullTestSuite()
	b := fullTestSuite()
	b.Tool = "other-tool"

	diff := cmp.Diff(a, b)
	assert.NotEmpty(t, diff,
		"TestSuites with different Tool names must not be equal")
}

// ---------------------------------------------------------------------------
// Anti-hardcoding: multiple distinct Assertions
// ---------------------------------------------------------------------------

func TestAssertion_MultipleDistinctValues(t *testing.T) {
	// A hardcoded Assertion implementation returning fixed values would fail.
	assertions := []Assertion{
		{Path: "$.a", Equals: "alpha"},
		{Path: "$.b", Equals: 999},
		{Path: "$.c", Contains: true},
		{Path: "$.d", Matches: `^test$`},
		{Path: "$.e", Exists: boolPtr(false)},
		{Path: "$.f", Length: intPtr(42)},
	}

	// Every assertion must have a unique Path.
	paths := make(map[string]bool)
	for _, a := range assertions {
		assert.False(t, paths[a.Path],
			"duplicate path %q found — each assertion should be distinct", a.Path)
		paths[a.Path] = true
	}
	assert.Len(t, paths, 6, "must have 6 unique paths")

	// Spot-check values.
	assert.Equal(t, "alpha", assertions[0].Equals)
	assert.Equal(t, 999, assertions[1].Equals)
	assert.Equal(t, true, assertions[2].Contains)
	assert.Equal(t, `^test$`, assertions[3].Matches)
	require.NotNil(t, assertions[4].Exists)
	assert.False(t, *assertions[4].Exists)
	require.NotNil(t, assertions[5].Length)
	assert.Equal(t, 42, *assertions[5].Length)
}

// ---------------------------------------------------------------------------
// TestResult: Duration field
// ---------------------------------------------------------------------------

func TestTestResult_DurationPrecision(t *testing.T) {
	r := TestResult{
		Name:     "precise-timing",
		Status:   "pass",
		Duration: 1234567 * time.Nanosecond,
	}
	assert.Equal(t, "precise-timing", r.Name)
	assert.Equal(t, "pass", r.Status)
	assert.Equal(t, 1234567*time.Nanosecond, r.Duration,
		"Duration must preserve nanosecond precision")
}

func TestTestResult_ZeroDuration(t *testing.T) {
	r := TestResult{
		Name:     "instant",
		Status:   "pass",
		Duration: 0,
	}
	assert.Equal(t, "instant", r.Name)
	assert.Equal(t, "pass", r.Status)
	assert.Equal(t, time.Duration(0), r.Duration,
		"zero Duration must be exactly 0")
}

// ---------------------------------------------------------------------------
// TestCase: Flags map behavior
// ---------------------------------------------------------------------------

func TestTestCase_FlagsMapPreservesEntries(t *testing.T) {
	tc := TestCase{
		Flags: map[string]string{
			"format":  "json",
			"verbose": "true",
			"limit":   "100",
		},
	}

	require.Len(t, tc.Flags, 3, "Flags map must preserve all 3 entries")
	assert.Equal(t, "json", tc.Flags["format"])
	assert.Equal(t, "true", tc.Flags["verbose"])
	assert.Equal(t, "100", tc.Flags["limit"])
}

func TestTestCase_FlagsEmptyVsNil(t *testing.T) {
	nilFlags := TestCase{Flags: nil}
	emptyFlags := TestCase{Flags: map[string]string{}}

	assert.Nil(t, nilFlags.Flags, "nil Flags must be nil")
	assert.NotNil(t, emptyFlags.Flags, "empty Flags must not be nil")
	assert.Len(t, emptyFlags.Flags, 0)
}

// ---------------------------------------------------------------------------
// TestCase: Args slice ordering
// ---------------------------------------------------------------------------

func TestTestCase_ArgsPreserveOrder(t *testing.T) {
	tc := TestCase{
		Args: []string{"third", "first", "second"},
	}

	require.Len(t, tc.Args, 3)
	assert.Equal(t, "third", tc.Args[0], "Args must preserve insertion order, not sort")
	assert.Equal(t, "first", tc.Args[1])
	assert.Equal(t, "second", tc.Args[2])
}
