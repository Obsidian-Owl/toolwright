package tooltest

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// TAP output test fixtures
// ---------------------------------------------------------------------------

// allPassReport returns a report where every test passes.
func allPassReport() TestReport {
	return TestReport{
		Tool:   "my-tool",
		Total:  2,
		Passed: 2,
		Failed: 0,
		Results: []TestResult{
			{
				Name:     "creates output file",
				Status:   "pass",
				Duration: 120 * time.Millisecond,
				Stdout:   []byte("ok"),
			},
			{
				Name:     "reads input correctly",
				Status:   "pass",
				Duration: 45 * time.Millisecond,
				Stdout:   []byte("data"),
			},
		},
	}
}

// mixedReport returns a report with one pass and one fail.
func mixedReport() TestReport {
	return TestReport{
		Tool:   "scanner",
		Total:  2,
		Passed: 1,
		Failed: 1,
		Results: []TestResult{
			{
				Name:     "finds vulnerabilities",
				Status:   "pass",
				Duration: 300 * time.Millisecond,
				Stdout:   []byte(`{"found": 3}`),
			},
			{
				Name:     "handles missing file",
				Status:   "fail",
				Duration: 15 * time.Millisecond,
				Error:    "expected exit code 0 but got 1",
				Stderr:   []byte("fatal: file not found"),
			},
		},
	}
}

// allFailReport returns a report where every test fails.
func allFailReport() TestReport {
	return TestReport{
		Tool:   "broken-tool",
		Total:  3,
		Passed: 0,
		Failed: 3,
		Results: []TestResult{
			{
				Name:     "test alpha",
				Status:   "fail",
				Duration: 10 * time.Millisecond,
				Error:    "timeout exceeded",
				Stderr:   []byte("deadline"),
			},
			{
				Name:     "test beta",
				Status:   "fail",
				Duration: 20 * time.Millisecond,
				Error:    "wrong output format",
				Stdout:   []byte("garbage"),
			},
			{
				Name:     "test gamma",
				Status:   "fail",
				Duration: 5 * time.Millisecond,
				Error:    "schema validation failed",
			},
		},
	}
}

// emptyReport returns a report with zero tests.
func emptyReport() TestReport {
	return TestReport{
		Tool:    "empty-tool",
		Total:   0,
		Passed:  0,
		Failed:  0,
		Results: []TestResult{},
	}
}

// singlePassReport returns a report with exactly one passing test.
func singlePassReport() TestReport {
	return TestReport{
		Tool:   "single-tool",
		Total:  1,
		Passed: 1,
		Failed: 0,
		Results: []TestResult{
			{
				Name:     "the only test",
				Status:   "pass",
				Duration: 77 * time.Millisecond,
			},
		},
	}
}

// specialCharsReport returns a report with test names containing special characters.
func specialCharsReport() TestReport {
	return TestReport{
		Tool:   "special-tool",
		Total:  3,
		Passed: 2,
		Failed: 1,
		Results: []TestResult{
			{
				Name:     "handles 'quoted' names",
				Status:   "pass",
				Duration: 10 * time.Millisecond,
			},
			{
				Name:     "supports unicode: cafe\u0301",
				Status:   "pass",
				Duration: 20 * time.Millisecond,
			},
			{
				Name:     "checks $PATH & env vars <special>",
				Status:   "fail",
				Duration: 30 * time.Millisecond,
				Error:    "env var not set",
			},
		},
	}
}

// ---------------------------------------------------------------------------
// TAP format helper: parse TAP lines for structural verification
// ---------------------------------------------------------------------------

// parseTAPLines splits TAP output into non-empty lines.
func parseTAPLines(output string) []string {
	var lines []string
	for _, line := range strings.Split(output, "\n") {
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

// ---------------------------------------------------------------------------
// AC-13: TAP output format — header and plan
// ---------------------------------------------------------------------------

func TestFormatTAP_AllPass_HeaderAndPlan(t *testing.T) {
	report := allPassReport()
	var buf bytes.Buffer

	err := FormatTAP(report, &buf)
	require.NoError(t, err, "FormatTAP must not return an error for a valid report")

	output := buf.String()
	lines := parseTAPLines(output)

	require.GreaterOrEqual(t, len(lines), 4,
		"output must have at least 4 lines: header, plan, and 2 test lines")

	// First line must be exactly the TAP version header.
	assert.Equal(t, "TAP version 13", lines[0],
		"first line must be exactly 'TAP version 13'")

	// Second line must be the plan.
	assert.Equal(t, "1..2", lines[1],
		"plan line must be '1..2' for 2 tests")
}

func TestFormatTAP_AllPass_TestLines(t *testing.T) {
	report := allPassReport()
	var buf bytes.Buffer

	err := FormatTAP(report, &buf)
	require.NoError(t, err)

	output := buf.String()
	lines := parseTAPLines(output)

	// Find the test result lines (lines starting with "ok " or "not ok ").
	var testLines []string
	for _, line := range lines {
		if strings.HasPrefix(line, "ok ") || strings.HasPrefix(line, "not ok ") {
			testLines = append(testLines, line)
		}
	}

	require.Len(t, testLines, 2, "must have exactly 2 test result lines")
	assert.Equal(t, "ok 1 - creates output file", testLines[0],
		"first test line must be 'ok 1 - creates output file'")
	assert.Equal(t, "ok 2 - reads input correctly", testLines[1],
		"second test line must be 'ok 2 - reads input correctly'")
}

func TestFormatTAP_AllPass_NoYAMLDiagnostics(t *testing.T) {
	report := allPassReport()
	var buf bytes.Buffer

	err := FormatTAP(report, &buf)
	require.NoError(t, err)

	output := buf.String()

	// Passing tests must NOT have YAML diagnostics blocks.
	assert.NotContains(t, output, "  ---",
		"passing tests must not have YAML diagnostics block")
	assert.NotContains(t, output, "  ...",
		"passing tests must not have YAML diagnostics end marker")
}

// ---------------------------------------------------------------------------
// AC-13: TAP output format — mixed pass/fail with diagnostics
// ---------------------------------------------------------------------------

func TestFormatTAP_MixedPassFail_TestLines(t *testing.T) {
	report := mixedReport()
	var buf bytes.Buffer

	err := FormatTAP(report, &buf)
	require.NoError(t, err)

	output := buf.String()
	lines := parseTAPLines(output)

	// Header and plan.
	assert.Equal(t, "TAP version 13", lines[0])
	assert.Equal(t, "1..2", lines[1])

	// First test passes.
	assert.Contains(t, output, "ok 1 - finds vulnerabilities",
		"first test must show as 'ok 1 - finds vulnerabilities'")

	// Second test fails.
	assert.Contains(t, output, "not ok 2 - handles missing file",
		"second test must show as 'not ok 2 - handles missing file'")
}

func TestFormatTAP_FailedTest_HasYAMLDiagnostics(t *testing.T) {
	report := mixedReport()
	var buf bytes.Buffer

	err := FormatTAP(report, &buf)
	require.NoError(t, err)

	output := buf.String()

	// The YAML diagnostics block must appear after the "not ok" line.
	notOkIdx := strings.Index(output, "not ok 2 - handles missing file")
	require.NotEqual(t, -1, notOkIdx,
		"must contain 'not ok 2 - handles missing file'")

	afterNotOk := output[notOkIdx:]

	// YAML block must start with "  ---" (2-space indented).
	assert.Contains(t, afterNotOk, "\n  ---",
		"YAML diagnostics must start with '  ---' after failed test line")

	// YAML block must end with "  ..." (2-space indented).
	assert.Contains(t, afterNotOk, "\n  ...",
		"YAML diagnostics must end with '  ...' after failed test line")

	// The --- must appear before ... in the output after "not ok".
	dashIdx := strings.Index(afterNotOk, "\n  ---")
	dotsIdx := strings.Index(afterNotOk, "\n  ...")
	assert.Less(t, dashIdx, dotsIdx,
		"'---' must appear before '...' in YAML diagnostics")
}

func TestFormatTAP_FailedTest_DiagnosticsContainError(t *testing.T) {
	report := mixedReport()
	var buf bytes.Buffer

	err := FormatTAP(report, &buf)
	require.NoError(t, err)

	output := buf.String()

	// Extract the YAML diagnostics block for the failed test.
	notOkIdx := strings.Index(output, "not ok 2 - handles missing file")
	require.NotEqual(t, -1, notOkIdx)

	afterNotOk := output[notOkIdx:]
	dashIdx := strings.Index(afterNotOk, "\n  ---")
	dotsIdx := strings.Index(afterNotOk, "\n  ...")
	require.NotEqual(t, -1, dashIdx, "YAML block must have ---")
	require.NotEqual(t, -1, dotsIdx, "YAML block must have ...")

	// Extract the block between --- and ...
	diagnostics := afterNotOk[dashIdx:dotsIdx]

	// Must contain the error message.
	assert.Contains(t, diagnostics, "error:",
		"diagnostics must contain 'error:' key")
	assert.Contains(t, diagnostics, "expected exit code 0 but got 1",
		"diagnostics must contain the actual error message")
}

func TestFormatTAP_FailedTest_DiagnosticsContainDuration(t *testing.T) {
	report := mixedReport()
	var buf bytes.Buffer

	err := FormatTAP(report, &buf)
	require.NoError(t, err)

	output := buf.String()

	// Extract diagnostics block.
	notOkIdx := strings.Index(output, "not ok 2 - handles missing file")
	require.NotEqual(t, -1, notOkIdx)

	afterNotOk := output[notOkIdx:]
	dashIdx := strings.Index(afterNotOk, "\n  ---")
	dotsIdx := strings.Index(afterNotOk, "\n  ...")
	require.NotEqual(t, -1, dashIdx)
	require.NotEqual(t, -1, dotsIdx)

	diagnostics := afterNotOk[dashIdx:dotsIdx]

	// Must contain duration_ms.
	assert.Contains(t, diagnostics, "duration_ms:",
		"diagnostics must contain 'duration_ms:' key")

	// The duration should be a number, not a string. 15ms = 15.
	assert.Contains(t, diagnostics, "15",
		"duration_ms for the failed test must be 15 (from 15ms)")
}

// ---------------------------------------------------------------------------
// AC-13: TAP output format — exact format verification
// ---------------------------------------------------------------------------

func TestFormatTAP_MixedPassFail_ExactOutput(t *testing.T) {
	// This test verifies the complete TAP output structure for a mixed report.
	// A sloppy implementation that gets partial format right but misses details
	// (indentation, ordering, YAML block placement) will fail here.
	report := TestReport{
		Tool:   "exact-tool",
		Total:  2,
		Passed: 1,
		Failed: 1,
		Results: []TestResult{
			{
				Name:     "passing test",
				Status:   "pass",
				Duration: 100 * time.Millisecond,
			},
			{
				Name:     "failing test",
				Status:   "fail",
				Duration: 250 * time.Millisecond,
				Error:    "assertion failed",
			},
		},
	}

	var buf bytes.Buffer
	err := FormatTAP(report, &buf)
	require.NoError(t, err)

	output := buf.String()

	// Verify the output line by line.
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	require.GreaterOrEqual(t, len(lines), 7,
		"exact output must have at least 7 lines: header, plan, ok, not ok, ---, 2 diag lines, ...")

	assert.Equal(t, "TAP version 13", lines[0], "line 1: TAP header")
	assert.Equal(t, "1..2", lines[1], "line 2: plan")
	assert.Equal(t, "ok 1 - passing test", lines[2], "line 3: passing test")
	assert.Equal(t, "not ok 2 - failing test", lines[3], "line 4: failing test")
	assert.Equal(t, "  ---", lines[4], "line 5: YAML block start")

	// Lines 5+ are YAML diagnostics. Find the end marker.
	foundEnd := false
	var diagLines []string
	for i := 5; i < len(lines); i++ {
		if lines[i] == "  ..." {
			foundEnd = true
			break
		}
		diagLines = append(diagLines, lines[i])
	}
	require.True(t, foundEnd, "YAML block must end with '  ...'")

	// The diagnostics must contain error and duration_ms keys with proper indentation.
	diagBlock := strings.Join(diagLines, "\n")
	assert.Contains(t, diagBlock, "error: assertion failed",
		"diagnostics must contain error value")
	assert.Contains(t, diagBlock, "duration_ms: 250",
		"diagnostics must contain duration_ms value")

	// Each diagnostic line must be indented with exactly 4 spaces.
	for _, dl := range diagLines {
		assert.True(t, strings.HasPrefix(dl, "    "),
			"diagnostic line %q must be indented with 4 spaces", dl)
	}
}

// ---------------------------------------------------------------------------
// AC-13: TAP output format — zero tests
// ---------------------------------------------------------------------------

func TestFormatTAP_EmptyReport(t *testing.T) {
	report := emptyReport()
	var buf bytes.Buffer

	err := FormatTAP(report, &buf)
	require.NoError(t, err)

	output := buf.String()
	lines := parseTAPLines(output)

	require.Len(t, lines, 2,
		"empty report must have exactly 2 lines: header and plan")
	assert.Equal(t, "TAP version 13", lines[0])
	assert.Equal(t, "1..0", lines[1],
		"empty test suite must output '1..0' as the plan")
}

// ---------------------------------------------------------------------------
// AC-13: TAP output format — all fail
// ---------------------------------------------------------------------------

func TestFormatTAP_AllFail(t *testing.T) {
	report := allFailReport()
	var buf bytes.Buffer

	err := FormatTAP(report, &buf)
	require.NoError(t, err)

	output := buf.String()

	// Plan must show 3 tests.
	assert.Contains(t, output, "1..3",
		"plan must show '1..3' for 3 tests")

	// Every test line must be "not ok".
	assert.Contains(t, output, "not ok 1 - test alpha")
	assert.Contains(t, output, "not ok 2 - test beta")
	assert.Contains(t, output, "not ok 3 - test gamma")

	// Must NOT contain any bare "ok " line (that isn't "not ok").
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "ok ") {
			t.Errorf("found bare 'ok' line in all-fail output: %q", line)
		}
	}

	// Each failed test must have its own YAML diagnostics block.
	assert.Equal(t, 3, strings.Count(output, "  ---"),
		"each of the 3 failed tests must have a YAML diagnostics start marker")
	assert.Equal(t, 3, strings.Count(output, "  ..."),
		"each of the 3 failed tests must have a YAML diagnostics end marker")
}

func TestFormatTAP_AllFail_EachHasDistinctError(t *testing.T) {
	report := allFailReport()
	var buf bytes.Buffer

	err := FormatTAP(report, &buf)
	require.NoError(t, err)

	output := buf.String()

	// Each failed test must contain its specific error message.
	assert.Contains(t, output, "timeout exceeded",
		"alpha error must appear in diagnostics")
	assert.Contains(t, output, "wrong output format",
		"beta error must appear in diagnostics")
	assert.Contains(t, output, "schema validation failed",
		"gamma error must appear in diagnostics")
}

// ---------------------------------------------------------------------------
// AC-13: TAP output format — single test
// ---------------------------------------------------------------------------

func TestFormatTAP_SingleTest(t *testing.T) {
	report := singlePassReport()
	var buf bytes.Buffer

	err := FormatTAP(report, &buf)
	require.NoError(t, err)

	output := buf.String()
	lines := parseTAPLines(output)

	require.GreaterOrEqual(t, len(lines), 3,
		"single test output must have at least 3 lines")
	assert.Equal(t, "TAP version 13", lines[0])
	assert.Equal(t, "1..1", lines[1],
		"single test plan must be '1..1'")
	assert.Equal(t, "ok 1 - the only test", lines[2])
}

// ---------------------------------------------------------------------------
// AC-13: TAP output format — special characters in names
// ---------------------------------------------------------------------------

func TestFormatTAP_SpecialCharacters(t *testing.T) {
	report := specialCharsReport()
	var buf bytes.Buffer

	err := FormatTAP(report, &buf)
	require.NoError(t, err)

	output := buf.String()

	// Test names with special characters must be preserved exactly.
	assert.Contains(t, output, "ok 1 - handles 'quoted' names",
		"single-quoted name must be preserved")
	assert.Contains(t, output, "ok 2 - supports unicode: cafe\u0301",
		"unicode name must be preserved")
	assert.Contains(t, output, "not ok 3 - checks $PATH & env vars <special>",
		"special chars ($, &, <, >) must be preserved")
}

// ---------------------------------------------------------------------------
// AC-13: TAP output format — output captured in bytes.Buffer
// ---------------------------------------------------------------------------

func TestFormatTAP_OutputCapturedInBuffer(t *testing.T) {
	report := allPassReport()
	var buf bytes.Buffer

	err := FormatTAP(report, &buf)
	require.NoError(t, err)

	output := buf.String()

	// The entire output must be non-empty and contain the essential TAP elements.
	assert.NotEmpty(t, output, "buffer must contain output")
	assert.True(t, strings.HasPrefix(output, "TAP version 13\n"),
		"output must start with TAP header followed by newline")
	assert.True(t, strings.HasSuffix(output, "\n"),
		"output must end with a trailing newline")
}

// ---------------------------------------------------------------------------
// AC-13: TAP output — test numbering is sequential starting at 1
// ---------------------------------------------------------------------------

func TestFormatTAP_SequentialNumbering(t *testing.T) {
	// Use a larger report to verify sequential numbering.
	report := TestReport{
		Tool:   "numbered-tool",
		Total:  5,
		Passed: 3,
		Failed: 2,
		Results: []TestResult{
			{Name: "test-a", Status: "pass", Duration: 10 * time.Millisecond},
			{Name: "test-b", Status: "fail", Duration: 20 * time.Millisecond, Error: "err-b"},
			{Name: "test-c", Status: "pass", Duration: 30 * time.Millisecond},
			{Name: "test-d", Status: "pass", Duration: 40 * time.Millisecond},
			{Name: "test-e", Status: "fail", Duration: 50 * time.Millisecond, Error: "err-e"},
		},
	}

	var buf bytes.Buffer
	err := FormatTAP(report, &buf)
	require.NoError(t, err)

	output := buf.String()

	assert.Contains(t, output, "1..5")
	assert.Contains(t, output, "ok 1 - test-a")
	assert.Contains(t, output, "not ok 2 - test-b")
	assert.Contains(t, output, "ok 3 - test-c")
	assert.Contains(t, output, "ok 4 - test-d")
	assert.Contains(t, output, "not ok 5 - test-e")
}

// ---------------------------------------------------------------------------
// AC-13: TAP output — diagnostics YAML block structure (indentation)
// ---------------------------------------------------------------------------

func TestFormatTAP_DiagnosticsYAMLIndentation(t *testing.T) {
	report := TestReport{
		Tool:   "indent-tool",
		Total:  1,
		Passed: 0,
		Failed: 1,
		Results: []TestResult{
			{
				Name:     "indentation check",
				Status:   "fail",
				Duration: 42 * time.Millisecond,
				Error:    "something broke",
			},
		},
	}

	var buf bytes.Buffer
	err := FormatTAP(report, &buf)
	require.NoError(t, err)

	output := buf.String()
	lines := strings.Split(output, "\n")

	// Find the YAML block.
	inBlock := false
	var blockLines []string
	for _, line := range lines {
		if line == "  ---" {
			inBlock = true
			continue
		}
		if line == "  ..." {
			inBlock = false
			continue
		}
		if inBlock {
			blockLines = append(blockLines, line)
		}
	}

	require.NotEmpty(t, blockLines,
		"YAML diagnostics block must contain at least one line")

	// Every line inside the YAML block must be indented with 4 spaces.
	for _, bl := range blockLines {
		assert.True(t, strings.HasPrefix(bl, "    "),
			"YAML diagnostic line %q must start with 4 spaces", bl)
	}

	// Verify specific key-value pairs in the YAML block.
	blockStr := strings.Join(blockLines, "\n")
	assert.Contains(t, blockStr, "error: something broke")
	assert.Contains(t, blockStr, "duration_ms: 42")
}

// ---------------------------------------------------------------------------
// AC-13: TAP output — error return on nil writer
// ---------------------------------------------------------------------------

func TestFormatTAP_ReturnsNoErrorOnValidWriter(t *testing.T) {
	report := allPassReport()
	var buf bytes.Buffer

	err := FormatTAP(report, &buf)
	assert.NoError(t, err, "FormatTAP must not error with a valid writer")
}

// ---------------------------------------------------------------------------
// AC-13: TAP output — result ordering preserved
// ---------------------------------------------------------------------------

func TestFormatTAP_ResultOrderPreserved(t *testing.T) {
	report := mixedReport()
	var buf bytes.Buffer

	err := FormatTAP(report, &buf)
	require.NoError(t, err)

	output := buf.String()

	// "finds vulnerabilities" must appear before "handles missing file".
	idxPass := strings.Index(output, "finds vulnerabilities")
	idxFail := strings.Index(output, "handles missing file")
	require.NotEqual(t, -1, idxPass)
	require.NotEqual(t, -1, idxFail)
	assert.Less(t, idxPass, idxFail,
		"test results must appear in the order they are in the Results slice")
}

// ---------------------------------------------------------------------------
// AC-13: TAP output — table-driven test for plan lines
// ---------------------------------------------------------------------------

func TestFormatTAP_PlanLine_TableDriven(t *testing.T) {
	tests := []struct {
		name     string
		total    int
		results  []TestResult
		expected string
	}{
		{
			name:     "zero tests",
			total:    0,
			results:  []TestResult{},
			expected: "1..0",
		},
		{
			name:  "one test",
			total: 1,
			results: []TestResult{
				{Name: "t1", Status: "pass", Duration: time.Millisecond},
			},
			expected: "1..1",
		},
		{
			name:  "five tests",
			total: 5,
			results: []TestResult{
				{Name: "t1", Status: "pass", Duration: time.Millisecond},
				{Name: "t2", Status: "pass", Duration: time.Millisecond},
				{Name: "t3", Status: "fail", Duration: time.Millisecond, Error: "e"},
				{Name: "t4", Status: "pass", Duration: time.Millisecond},
				{Name: "t5", Status: "fail", Duration: time.Millisecond, Error: "e"},
			},
			expected: "1..5",
		},
		{
			name:  "ten tests",
			total: 10,
			results: func() []TestResult {
				r := make([]TestResult, 10)
				for i := range r {
					r[i] = TestResult{
						Name:     fmt.Sprintf("test-%d", i+1),
						Status:   "pass",
						Duration: time.Millisecond,
					}
				}
				return r
			}(),
			expected: "1..10",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			report := TestReport{
				Tool:    "plan-tool",
				Total:   tc.total,
				Passed:  tc.total,
				Results: tc.results,
			}

			var buf bytes.Buffer
			err := FormatTAP(report, &buf)
			require.NoError(t, err)

			lines := parseTAPLines(buf.String())
			require.GreaterOrEqual(t, len(lines), 2,
				"output must have at least header and plan")
			assert.Equal(t, tc.expected, lines[1],
				"plan line must be %q", tc.expected)
		})
	}
}

// ===========================================================================
// JSON output format tests (AC-14)
// ===========================================================================

// jsonOutput is the expected structure of FormatJSON output.
// Defined here so we can unmarshal into it and check schema.
type jsonOutput struct {
	Tool    string             `json:"tool"`
	Total   int                `json:"total"`
	Passed  int                `json:"passed"`
	Failed  int                `json:"failed"`
	Results []jsonOutputResult `json:"results"`
}

type jsonOutputResult struct {
	Name       string  `json:"name"`
	Status     string  `json:"status"`
	DurationMS float64 `json:"duration_ms"`
	Error      string  `json:"error,omitempty"`
	Stdout     string  `json:"stdout,omitempty"`
	Stderr     string  `json:"stderr,omitempty"`
}

// ---------------------------------------------------------------------------
// AC-14: JSON output — valid JSON
// ---------------------------------------------------------------------------

func TestFormatJSON_ValidJSON(t *testing.T) {
	report := fullTestReport()
	var buf bytes.Buffer

	err := FormatJSON(report, &buf)
	require.NoError(t, err, "FormatJSON must not return an error for a valid report")

	// Must be valid JSON.
	var raw json.RawMessage
	err = json.Unmarshal(buf.Bytes(), &raw)
	assert.NoError(t, err, "output must be valid JSON: %s", buf.String())
}

// ---------------------------------------------------------------------------
// AC-14: JSON output — schema structure
// ---------------------------------------------------------------------------

func TestFormatJSON_SchemaStructure(t *testing.T) {
	report := fullTestReport()
	var buf bytes.Buffer

	err := FormatJSON(report, &buf)
	require.NoError(t, err)

	// Parse into a generic map to verify field names exist.
	var m map[string]any
	err = json.Unmarshal(buf.Bytes(), &m)
	require.NoError(t, err, "output must parse as JSON object")

	// Required top-level fields.
	requiredFields := []string{"tool", "total", "passed", "failed", "results"}
	for _, field := range requiredFields {
		_, exists := m[field]
		assert.True(t, exists, "JSON output must contain field %q", field)
	}

	// results must be an array.
	results, ok := m["results"].([]any)
	require.True(t, ok, "results must be a JSON array")
	require.Len(t, results, 3, "results array must have 3 entries")
}

// ---------------------------------------------------------------------------
// AC-14: JSON output — passing result fields
// ---------------------------------------------------------------------------

func TestFormatJSON_PassingResult(t *testing.T) {
	report := fullTestReport()
	var buf bytes.Buffer

	err := FormatJSON(report, &buf)
	require.NoError(t, err)

	var out jsonOutput
	err = json.Unmarshal(buf.Bytes(), &out)
	require.NoError(t, err)

	require.GreaterOrEqual(t, len(out.Results), 1, "must have at least one result")
	pass := out.Results[0]

	assert.Equal(t, "test-pass-1", pass.Name,
		"passing result name must match")
	assert.Equal(t, "pass", pass.Status,
		"passing result status must be 'pass'")
	assert.InDelta(t, 150.0, pass.DurationMS, 1.0,
		"passing result duration_ms must be approximately 150")
}

// ---------------------------------------------------------------------------
// AC-14: JSON output — failing result fields
// ---------------------------------------------------------------------------

func TestFormatJSON_FailingResult(t *testing.T) {
	report := fullTestReport()
	var buf bytes.Buffer

	err := FormatJSON(report, &buf)
	require.NoError(t, err)

	var out jsonOutput
	err = json.Unmarshal(buf.Bytes(), &out)
	require.NoError(t, err)

	require.GreaterOrEqual(t, len(out.Results), 3, "must have at least 3 results")
	fail := out.Results[2]

	assert.Equal(t, "test-fail-1", fail.Name,
		"failing result name must match")
	assert.Equal(t, "fail", fail.Status,
		"failing result status must be 'fail'")
	assert.InDelta(t, 50.0, fail.DurationMS, 1.0,
		"failing result duration_ms must be approximately 50")
	assert.Equal(t, "expected exit code 0 but got 1", fail.Error,
		"failing result must include the error message")
	assert.Contains(t, fail.Stderr, "fatal: something went wrong",
		"failing result must include stderr")
}

// ---------------------------------------------------------------------------
// AC-14: JSON output — tool name preserved
// ---------------------------------------------------------------------------

func TestFormatJSON_ToolNamePreserved(t *testing.T) {
	tests := []struct {
		name string
		tool string
	}{
		{"simple name", "my-tool"},
		{"dotted name", "org.tool.scanner"},
		{"slashed name", "github/tool"},
		{"empty name", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			report := TestReport{
				Tool:    tc.tool,
				Results: []TestResult{},
			}

			var buf bytes.Buffer
			err := FormatJSON(report, &buf)
			require.NoError(t, err)

			var out jsonOutput
			err = json.Unmarshal(buf.Bytes(), &out)
			require.NoError(t, err)

			assert.Equal(t, tc.tool, out.Tool,
				"tool name must be preserved in JSON output")
		})
	}
}

// ---------------------------------------------------------------------------
// AC-14: JSON output — counts match
// ---------------------------------------------------------------------------

func TestFormatJSON_CountsMatch(t *testing.T) {
	report := fullTestReport()
	var buf bytes.Buffer

	err := FormatJSON(report, &buf)
	require.NoError(t, err)

	var out jsonOutput
	err = json.Unmarshal(buf.Bytes(), &out)
	require.NoError(t, err)

	assert.Equal(t, report.Total, out.Total,
		"total must match report.Total")
	assert.Equal(t, report.Passed, out.Passed,
		"passed must match report.Passed")
	assert.Equal(t, report.Failed, out.Failed,
		"failed must match report.Failed")
}

func TestFormatJSON_CountsMatch_AntiHardcoding(t *testing.T) {
	// Use different values than fullTestReport to catch hardcoded returns.
	report := TestReport{
		Tool:   "anti-hardcode",
		Total:  7,
		Passed: 5,
		Failed: 2,
		Results: []TestResult{
			{Name: "a", Status: "pass", Duration: time.Millisecond},
			{Name: "b", Status: "pass", Duration: time.Millisecond},
			{Name: "c", Status: "pass", Duration: time.Millisecond},
			{Name: "d", Status: "pass", Duration: time.Millisecond},
			{Name: "e", Status: "pass", Duration: time.Millisecond},
			{Name: "f", Status: "fail", Duration: time.Millisecond, Error: "e1"},
			{Name: "g", Status: "fail", Duration: time.Millisecond, Error: "e2"},
		},
	}

	var buf bytes.Buffer
	err := FormatJSON(report, &buf)
	require.NoError(t, err)

	var out jsonOutput
	err = json.Unmarshal(buf.Bytes(), &out)
	require.NoError(t, err)

	assert.Equal(t, 7, out.Total, "total must be 7, not hardcoded to 3")
	assert.Equal(t, 5, out.Passed, "passed must be 5, not hardcoded to 2")
	assert.Equal(t, 2, out.Failed, "failed must be 2, not hardcoded to 1")
	assert.Len(t, out.Results, 7, "results must have 7 entries")
}

// ---------------------------------------------------------------------------
// AC-14: JSON output — duration_ms is numeric
// ---------------------------------------------------------------------------

func TestFormatJSON_DurationMSIsNumeric(t *testing.T) {
	report := fullTestReport()
	var buf bytes.Buffer

	err := FormatJSON(report, &buf)
	require.NoError(t, err)

	// Parse as raw map to check the actual JSON type of duration_ms.
	var m map[string]any
	err = json.Unmarshal(buf.Bytes(), &m)
	require.NoError(t, err)

	results, ok := m["results"].([]any)
	require.True(t, ok, "results must be an array")

	for i, r := range results {
		result, ok := r.(map[string]any)
		require.True(t, ok, "each result must be a JSON object")

		durationVal, exists := result["duration_ms"]
		require.True(t, exists, "result[%d] must have duration_ms field", i)

		// In Go's encoding/json, numbers unmarshal as float64.
		_, isFloat := durationVal.(float64)
		assert.True(t, isFloat,
			"duration_ms in result[%d] must be a number, got %T: %v", i, durationVal, durationVal)
	}
}

func TestFormatJSON_DurationMSValues(t *testing.T) {
	// Verify specific duration_ms values to catch unit conversion errors.
	report := TestReport{
		Tool:   "duration-tool",
		Total:  3,
		Passed: 3,
		Failed: 0,
		Results: []TestResult{
			{Name: "sub-ms", Status: "pass", Duration: 500 * time.Microsecond},
			{Name: "exact-1s", Status: "pass", Duration: 1 * time.Second},
			{Name: "2.5s", Status: "pass", Duration: 2500 * time.Millisecond},
		},
	}

	var buf bytes.Buffer
	err := FormatJSON(report, &buf)
	require.NoError(t, err)

	var out jsonOutput
	err = json.Unmarshal(buf.Bytes(), &out)
	require.NoError(t, err)

	require.Len(t, out.Results, 3)

	// 500 microseconds = 0.5 milliseconds.
	assert.InDelta(t, 0.5, out.Results[0].DurationMS, 0.01,
		"500us should be 0.5ms")

	// 1 second = 1000 milliseconds.
	assert.InDelta(t, 1000.0, out.Results[1].DurationMS, 0.01,
		"1s should be 1000ms")

	// 2500 milliseconds.
	assert.InDelta(t, 2500.0, out.Results[2].DurationMS, 0.01,
		"2.5s should be 2500ms")
}

// ---------------------------------------------------------------------------
// AC-14: JSON output — empty results
// ---------------------------------------------------------------------------

func TestFormatJSON_EmptyResults(t *testing.T) {
	report := emptyReport()
	var buf bytes.Buffer

	err := FormatJSON(report, &buf)
	require.NoError(t, err)

	var out jsonOutput
	err = json.Unmarshal(buf.Bytes(), &out)
	require.NoError(t, err)

	assert.Equal(t, "empty-tool", out.Tool)
	assert.Equal(t, 0, out.Total)
	assert.Equal(t, 0, out.Passed)
	assert.Equal(t, 0, out.Failed)

	// Results must be an empty array, not null.
	var m map[string]any
	err = json.Unmarshal(buf.Bytes(), &m)
	require.NoError(t, err)

	results, exists := m["results"]
	require.True(t, exists, "results field must exist")

	// Must be an array (even if empty), not null.
	resultsArr, ok := results.([]any)
	require.True(t, ok, "results must be a JSON array, not null")
	assert.Len(t, resultsArr, 0, "results array must be empty")
}

// ---------------------------------------------------------------------------
// AC-14: JSON output — stdout/stderr in results
// ---------------------------------------------------------------------------

func TestFormatJSON_StdoutStderrInResults(t *testing.T) {
	report := TestReport{
		Tool:   "io-tool",
		Total:  2,
		Passed: 1,
		Failed: 1,
		Results: []TestResult{
			{
				Name:     "with-stdout",
				Status:   "pass",
				Duration: 10 * time.Millisecond,
				Stdout:   []byte("hello world"),
			},
			{
				Name:     "with-stderr",
				Status:   "fail",
				Duration: 20 * time.Millisecond,
				Error:    "failed",
				Stderr:   []byte("error output"),
			},
		},
	}

	var buf bytes.Buffer
	err := FormatJSON(report, &buf)
	require.NoError(t, err)

	// Parse raw to check that stdout/stderr appear for the right results.
	var m map[string]any
	err = json.Unmarshal(buf.Bytes(), &m)
	require.NoError(t, err)

	results := m["results"].([]any)
	require.Len(t, results, 2)

	// First result: has stdout.
	r0 := results[0].(map[string]any)
	assert.Equal(t, "with-stdout", r0["name"])
	if stdoutVal, exists := r0["stdout"]; exists {
		// stdout must contain "hello world" in some form.
		stdoutStr, ok := stdoutVal.(string)
		require.True(t, ok, "stdout must be a string in JSON")
		assert.Contains(t, stdoutStr, "hello world",
			"stdout content must be preserved")
	}

	// Second result: has stderr.
	r1 := results[1].(map[string]any)
	assert.Equal(t, "with-stderr", r1["name"])
	if stderrVal, exists := r1["stderr"]; exists {
		stderrStr, ok := stderrVal.(string)
		require.True(t, ok, "stderr must be a string in JSON")
		assert.Contains(t, stderrStr, "error output",
			"stderr content must be preserved")
	}
}

// ---------------------------------------------------------------------------
// AC-14: JSON output — results array preserves order
// ---------------------------------------------------------------------------

func TestFormatJSON_ResultsOrderPreserved(t *testing.T) {
	report := fullTestReport()
	var buf bytes.Buffer

	err := FormatJSON(report, &buf)
	require.NoError(t, err)

	var out jsonOutput
	err = json.Unmarshal(buf.Bytes(), &out)
	require.NoError(t, err)

	require.Len(t, out.Results, 3)
	assert.Equal(t, "test-pass-1", out.Results[0].Name,
		"first result must be test-pass-1")
	assert.Equal(t, "test-pass-2", out.Results[1].Name,
		"second result must be test-pass-2")
	assert.Equal(t, "test-fail-1", out.Results[2].Name,
		"third result must be test-fail-1")
}

// ---------------------------------------------------------------------------
// AC-14: JSON output — failing result includes error, stdout, stderr
// ---------------------------------------------------------------------------

func TestFormatJSON_FailingResultContainsAllDiagnosticFields(t *testing.T) {
	report := TestReport{
		Tool:   "diag-tool",
		Total:  1,
		Passed: 0,
		Failed: 1,
		Results: []TestResult{
			{
				Name:     "full-failure",
				Status:   "fail",
				Duration: 99 * time.Millisecond,
				Error:    "assertion mismatch: expected 42 got 0",
				Stdout:   []byte("partial output"),
				Stderr:   []byte("stack trace here"),
			},
		},
	}

	var buf bytes.Buffer
	err := FormatJSON(report, &buf)
	require.NoError(t, err)

	var out jsonOutput
	err = json.Unmarshal(buf.Bytes(), &out)
	require.NoError(t, err)

	require.Len(t, out.Results, 1)
	fail := out.Results[0]

	assert.Equal(t, "full-failure", fail.Name)
	assert.Equal(t, "fail", fail.Status)
	assert.InDelta(t, 99.0, fail.DurationMS, 1.0)
	assert.Equal(t, "assertion mismatch: expected 42 got 0", fail.Error,
		"error field must contain the full error message")
	assert.Contains(t, fail.Stdout, "partial output",
		"stdout must be included for failed results")
	assert.Contains(t, fail.Stderr, "stack trace here",
		"stderr must be included for failed results")
}

// ---------------------------------------------------------------------------
// AC-14: JSON output — passing result should not have error field
// ---------------------------------------------------------------------------

func TestFormatJSON_PassingResultOmitsErrorField(t *testing.T) {
	report := singlePassReport()
	var buf bytes.Buffer

	err := FormatJSON(report, &buf)
	require.NoError(t, err)

	// Parse as raw map to check field presence.
	var m map[string]any
	err = json.Unmarshal(buf.Bytes(), &m)
	require.NoError(t, err)

	results := m["results"].([]any)
	require.Len(t, results, 1)

	r := results[0].(map[string]any)
	assert.Equal(t, "the only test", r["name"])
	assert.Equal(t, "pass", r["status"])

	// A passing result should either not have an "error" field or have it as empty.
	if errVal, exists := r["error"]; exists {
		assert.Equal(t, "", errVal,
			"if error field exists for a passing result, it must be empty string")
	}
}

// ---------------------------------------------------------------------------
// AC-14: JSON output — result status values are exact strings
// ---------------------------------------------------------------------------

func TestFormatJSON_StatusValuesExact(t *testing.T) {
	report := fullTestReport()
	var buf bytes.Buffer

	err := FormatJSON(report, &buf)
	require.NoError(t, err)

	var out jsonOutput
	err = json.Unmarshal(buf.Bytes(), &out)
	require.NoError(t, err)

	// Status must be exactly "pass" or "fail", not "passed", "PASS", "success", etc.
	for _, r := range out.Results {
		assert.Contains(t, []string{"pass", "fail"}, r.Status,
			"result %q status must be exactly 'pass' or 'fail', got %q", r.Name, r.Status)
	}
}

// ---------------------------------------------------------------------------
// AC-14: JSON output — write to bytes.Buffer (io.Writer)
// ---------------------------------------------------------------------------

func TestFormatJSON_WritesToWriter(t *testing.T) {
	report := allPassReport()
	var buf bytes.Buffer

	err := FormatJSON(report, &buf)
	require.NoError(t, err)

	output := buf.Bytes()
	assert.NotEmpty(t, output, "JSON output must not be empty")

	// Must parse as valid JSON.
	assert.True(t, json.Valid(output),
		"output written to buffer must be valid JSON")
}

// ---------------------------------------------------------------------------
// AC-14: JSON output — no extra fields beyond schema
// ---------------------------------------------------------------------------

func TestFormatJSON_NoExtraTopLevelFields(t *testing.T) {
	report := fullTestReport()
	var buf bytes.Buffer

	err := FormatJSON(report, &buf)
	require.NoError(t, err)

	var m map[string]any
	err = json.Unmarshal(buf.Bytes(), &m)
	require.NoError(t, err)

	allowed := map[string]bool{
		"tool": true, "total": true, "passed": true, "failed": true, "results": true,
	}

	for key := range m {
		assert.True(t, allowed[key],
			"unexpected top-level field %q in JSON output", key)
	}
}

// ---------------------------------------------------------------------------
// AC-14: JSON output — result field schema
// ---------------------------------------------------------------------------

func TestFormatJSON_ResultFieldSchema(t *testing.T) {
	report := fullTestReport()
	var buf bytes.Buffer

	err := FormatJSON(report, &buf)
	require.NoError(t, err)

	var m map[string]any
	err = json.Unmarshal(buf.Bytes(), &m)
	require.NoError(t, err)

	results := m["results"].([]any)

	// Check each result has the required fields.
	for i, r := range results {
		result := r.(map[string]any)

		// Every result must have name, status, and duration_ms.
		_, hasName := result["name"]
		assert.True(t, hasName, "result[%d] must have 'name' field", i)

		_, hasStatus := result["status"]
		assert.True(t, hasStatus, "result[%d] must have 'status' field", i)

		_, hasDuration := result["duration_ms"]
		assert.True(t, hasDuration, "result[%d] must have 'duration_ms' field", i)
	}
}

// ---------------------------------------------------------------------------
// AC-14: JSON output — total/passed/failed are integers (not floats/strings)
// ---------------------------------------------------------------------------

func TestFormatJSON_CountFieldTypes(t *testing.T) {
	report := fullTestReport()
	var buf bytes.Buffer

	err := FormatJSON(report, &buf)
	require.NoError(t, err)

	var m map[string]any
	err = json.Unmarshal(buf.Bytes(), &m)
	require.NoError(t, err)

	// In Go's encoding/json, numbers unmarshal as float64.
	// We verify they are numbers, and that they decode to the right integer values.
	total, ok := m["total"].(float64)
	require.True(t, ok, "total must be a number, got %T", m["total"])
	assert.Equal(t, float64(3), total, "total must be 3")

	passed, ok := m["passed"].(float64)
	require.True(t, ok, "passed must be a number, got %T", m["passed"])
	assert.Equal(t, float64(2), passed, "passed must be 2")

	failed, ok := m["failed"].(float64)
	require.True(t, ok, "failed must be a number, got %T", m["failed"])
	assert.Equal(t, float64(1), failed, "failed must be 1")
}

// ---------------------------------------------------------------------------
// AC-14: JSON output — multiple independent reports produce distinct output
// ---------------------------------------------------------------------------

func TestFormatJSON_DistinctReports(t *testing.T) {
	// Anti-hardcoding: two different reports must produce different JSON.
	report1 := allPassReport()
	report2 := allFailReport()

	var buf1, buf2 bytes.Buffer

	err := FormatJSON(report1, &buf1)
	require.NoError(t, err)

	err = FormatJSON(report2, &buf2)
	require.NoError(t, err)

	assert.NotEqual(t, buf1.String(), buf2.String(),
		"different reports must produce different JSON output")

	// Parse both and verify they have different tool names and counts.
	var out1, out2 jsonOutput
	err = json.Unmarshal(buf1.Bytes(), &out1)
	require.NoError(t, err)
	err = json.Unmarshal(buf2.Bytes(), &out2)
	require.NoError(t, err)

	assert.NotEqual(t, out1.Tool, out2.Tool,
		"different reports must have different tool names")
	assert.NotEqual(t, out1.Failed, out2.Failed,
		"different reports must have different failed counts")
}

// ---------------------------------------------------------------------------
// AC-14: JSON output — zero duration
// ---------------------------------------------------------------------------

func TestFormatJSON_ZeroDuration(t *testing.T) {
	report := TestReport{
		Tool:   "zero-dur-tool",
		Total:  1,
		Passed: 1,
		Failed: 0,
		Results: []TestResult{
			{Name: "instant", Status: "pass", Duration: 0},
		},
	}

	var buf bytes.Buffer
	err := FormatJSON(report, &buf)
	require.NoError(t, err)

	var out jsonOutput
	err = json.Unmarshal(buf.Bytes(), &out)
	require.NoError(t, err)

	require.Len(t, out.Results, 1)
	assert.Equal(t, 0.0, out.Results[0].DurationMS,
		"zero duration must be represented as 0 in JSON")
}

// ---------------------------------------------------------------------------
// AC-14: JSON output — large number of results
// ---------------------------------------------------------------------------

func TestFormatJSON_LargeResultSet(t *testing.T) {
	const n = 100
	results := make([]TestResult, n)
	for i := 0; i < n; i++ {
		results[i] = TestResult{
			Name:     fmt.Sprintf("test-%03d", i),
			Status:   "pass",
			Duration: time.Duration(i+1) * time.Millisecond,
		}
	}

	report := TestReport{
		Tool:    "bulk-tool",
		Total:   n,
		Passed:  n,
		Failed:  0,
		Results: results,
	}

	var buf bytes.Buffer
	err := FormatJSON(report, &buf)
	require.NoError(t, err)

	var out jsonOutput
	err = json.Unmarshal(buf.Bytes(), &out)
	require.NoError(t, err)

	assert.Len(t, out.Results, n, "must preserve all %d results", n)

	// Spot-check first and last to catch truncation.
	assert.Equal(t, "test-000", out.Results[0].Name)
	assert.Equal(t, fmt.Sprintf("test-%03d", n-1), out.Results[n-1].Name)

	// Verify ordering by checking sequential names.
	for i := 0; i < n; i++ {
		assert.Equal(t, fmt.Sprintf("test-%03d", i), out.Results[i].Name,
			"result[%d] must have correct name", i)
	}
}
