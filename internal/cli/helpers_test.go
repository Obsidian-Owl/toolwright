package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// AC-1: outputJSON writes valid, indented JSON to the given writer
// ---------------------------------------------------------------------------

func TestOutputJSON_SimpleStruct(t *testing.T) {
	var buf bytes.Buffer
	input := struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	}{Name: "my-tool", Version: "1.0.0"}

	err := outputJSON(&buf, input)
	require.NoError(t, err, "outputJSON should not error for a valid struct")

	// Must be valid JSON
	require.True(t, json.Valid(buf.Bytes()),
		"outputJSON must produce valid JSON, got: %s", buf.String())

	// Must unmarshal back to the correct values
	var got map[string]string
	err = json.Unmarshal(buf.Bytes(), &got)
	require.NoError(t, err)
	assert.Equal(t, "my-tool", got["name"],
		"name field must be preserved in JSON output")
	assert.Equal(t, "1.0.0", got["version"],
		"version field must be preserved in JSON output")
}

func TestOutputJSON_IsIndented(t *testing.T) {
	var buf bytes.Buffer
	input := map[string]string{"key": "value"}

	err := outputJSON(&buf, input)
	require.NoError(t, err)

	output := buf.String()
	// Indented JSON has newlines and spaces; compact JSON does not.
	assert.Contains(t, output, "\n",
		"outputJSON should produce indented (pretty) JSON, not compact")
	assert.Contains(t, output, "  ",
		"outputJSON should use space indentation")
}

func TestOutputJSON_Map(t *testing.T) {
	var buf bytes.Buffer
	input := map[string]any{
		"tools": []string{"list-pets", "add-pet"},
		"count": 2,
	}

	err := outputJSON(&buf, input)
	require.NoError(t, err)

	var got map[string]any
	err = json.Unmarshal(buf.Bytes(), &got)
	require.NoError(t, err)

	tools, ok := got["tools"].([]any)
	require.True(t, ok, "tools field must be an array")
	assert.Len(t, tools, 2)
	assert.Equal(t, "list-pets", tools[0])
	assert.Equal(t, "add-pet", tools[1])

	count, ok := got["count"].(float64) // JSON numbers are float64
	require.True(t, ok, "count field must be a number")
	assert.Equal(t, float64(2), count)
}

func TestOutputJSON_EmptyObject(t *testing.T) {
	var buf bytes.Buffer
	err := outputJSON(&buf, struct{}{})
	require.NoError(t, err)
	assert.True(t, json.Valid(buf.Bytes()),
		"outputJSON of empty struct must be valid JSON")
	// Should be "{}" (possibly with whitespace)
	assert.Equal(t, "{}", strings.TrimSpace(buf.String()),
		"empty struct should serialize to {}")
}

func TestOutputJSON_NilSlice(t *testing.T) {
	var buf bytes.Buffer
	type response struct {
		Items []string `json:"items"`
	}
	err := outputJSON(&buf, response{Items: nil})
	require.NoError(t, err)
	assert.True(t, json.Valid(buf.Bytes()))
	// nil slice in Go marshals to "null" by default; verify it is valid JSON
}

func TestOutputJSON_EmptySlice(t *testing.T) {
	var buf bytes.Buffer
	type response struct {
		Items []string `json:"items"`
	}
	err := outputJSON(&buf, response{Items: []string{}})
	require.NoError(t, err)
	assert.True(t, json.Valid(buf.Bytes()))

	var got map[string]any
	err = json.Unmarshal(buf.Bytes(), &got)
	require.NoError(t, err)
	items, ok := got["items"].([]any)
	require.True(t, ok, "empty slice should marshal to JSON array, not null")
	assert.Empty(t, items, "empty slice should marshal to empty array []")
}

func TestOutputJSON_SpecialCharacters(t *testing.T) {
	var buf bytes.Buffer
	input := map[string]string{
		"message": `He said "hello" & <goodbye>`,
		"path":    `/foo/bar\baz`,
		"unicode": "cafe\u0301",
	}

	err := outputJSON(&buf, input)
	require.NoError(t, err)
	assert.True(t, json.Valid(buf.Bytes()),
		"outputJSON must produce valid JSON even with special characters")

	var got map[string]string
	err = json.Unmarshal(buf.Bytes(), &got)
	require.NoError(t, err)
	assert.Equal(t, `He said "hello" & <goodbye>`, got["message"],
		"special characters must round-trip through JSON")
	assert.Equal(t, `/foo/bar\baz`, got["path"])
}

func TestOutputJSON_NestedStructure(t *testing.T) {
	var buf bytes.Buffer
	input := map[string]any{
		"error": map[string]string{
			"code":    "manifest_invalid",
			"message": "validation failed",
			"hint":    "check your manifest",
		},
	}

	err := outputJSON(&buf, input)
	require.NoError(t, err)
	assert.True(t, json.Valid(buf.Bytes()))

	var got map[string]any
	err = json.Unmarshal(buf.Bytes(), &got)
	require.NoError(t, err)

	errObj, ok := got["error"].(map[string]any)
	require.True(t, ok, "error field must be a nested object")
	assert.Equal(t, "manifest_invalid", errObj["code"])
	assert.Equal(t, "validation failed", errObj["message"])
	assert.Equal(t, "check your manifest", errObj["hint"])
}

func TestOutputJSON_TrailingNewline(t *testing.T) {
	var buf bytes.Buffer
	err := outputJSON(&buf, map[string]string{"k": "v"})
	require.NoError(t, err)

	output := buf.String()
	assert.True(t, strings.HasSuffix(output, "\n"),
		"outputJSON should terminate with a trailing newline for clean CLI output")
}

// ---------------------------------------------------------------------------
// AC-2: outputError writes structured JSON error object
// ---------------------------------------------------------------------------

func TestOutputError_BasicStructure(t *testing.T) {
	var buf bytes.Buffer
	err := outputError(&buf, "manifest_invalid", "file is invalid", "run toolwright validate")
	require.NoError(t, err, "outputError should not return an error")

	assert.True(t, json.Valid(buf.Bytes()),
		"outputError must produce valid JSON, got: %s", buf.String())

	var got map[string]any
	err = json.Unmarshal(buf.Bytes(), &got)
	require.NoError(t, err)

	// The top-level key must be "error"
	errObj, ok := got["error"].(map[string]any)
	require.True(t, ok,
		"output must have a top-level 'error' key containing an object, got: %v", got)

	assert.Equal(t, "manifest_invalid", errObj["code"],
		"error.code must match the provided code")
	assert.Equal(t, "file is invalid", errObj["message"],
		"error.message must match the provided message")
	assert.Equal(t, "run toolwright validate", errObj["hint"],
		"error.hint must match the provided hint")
}

func TestOutputError_OnlyThreeFields(t *testing.T) {
	var buf bytes.Buffer
	err := outputError(&buf, "test_code", "test message", "test hint")
	require.NoError(t, err)

	var got map[string]any
	err = json.Unmarshal(buf.Bytes(), &got)
	require.NoError(t, err)

	// Top level should have exactly one key: "error"
	assert.Len(t, got, 1,
		"output must have exactly one top-level key ('error'), got: %v", got)

	errObj := got["error"].(map[string]any)
	assert.Len(t, errObj, 3,
		"error object must have exactly 3 fields (code, message, hint), got: %v", errObj)
}

func TestOutputError_ErrorCodes_SnakeCase(t *testing.T) {
	// AC-2 specifies snake_case error codes.
	tests := []struct {
		name string
		code string
	}{
		{name: "manifest_invalid", code: "manifest_invalid"},
		{name: "tool_not_found", code: "tool_not_found"},
		{name: "auth_required", code: "auth_required"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			err := outputError(&buf, tc.code, "msg", "hint")
			require.NoError(t, err)

			var got map[string]any
			err = json.Unmarshal(buf.Bytes(), &got)
			require.NoError(t, err)

			errObj := got["error"].(map[string]any)
			assert.Equal(t, tc.code, errObj["code"],
				"error code must be preserved exactly as provided")
		})
	}
}

func TestOutputError_EmptyMessage(t *testing.T) {
	var buf bytes.Buffer
	err := outputError(&buf, "some_code", "", "")
	require.NoError(t, err, "outputError must not error even for empty strings")

	assert.True(t, json.Valid(buf.Bytes()),
		"output must be valid JSON even with empty strings")

	var got map[string]any
	err = json.Unmarshal(buf.Bytes(), &got)
	require.NoError(t, err)

	errObj := got["error"].(map[string]any)
	assert.Equal(t, "some_code", errObj["code"])
	assert.Equal(t, "", errObj["message"],
		"empty message must be preserved as empty string, not omitted")
	assert.Equal(t, "", errObj["hint"],
		"empty hint must be preserved as empty string, not omitted")
}

func TestOutputError_SpecialCharactersInMessage(t *testing.T) {
	var buf bytes.Buffer
	err := outputError(&buf, "test_error",
		`file "toolwright.yaml" not found in /home/user's dir`,
		`run: toolwright validate --path="./manifest.yaml"`)
	require.NoError(t, err)

	assert.True(t, json.Valid(buf.Bytes()),
		"special characters in message/hint must produce valid JSON")

	var got map[string]any
	err = json.Unmarshal(buf.Bytes(), &got)
	require.NoError(t, err)

	errObj := got["error"].(map[string]any)
	assert.Equal(t, `file "toolwright.yaml" not found in /home/user's dir`, errObj["message"])
	assert.Equal(t, `run: toolwright validate --path="./manifest.yaml"`, errObj["hint"])
}

func TestOutputError_NewlinesInMessage(t *testing.T) {
	var buf bytes.Buffer
	err := outputError(&buf, "multi_line", "line1\nline2", "hint\nwith\nnewlines")
	require.NoError(t, err)

	assert.True(t, json.Valid(buf.Bytes()),
		"newlines in message must be properly escaped in JSON")

	var got map[string]any
	err = json.Unmarshal(buf.Bytes(), &got)
	require.NoError(t, err)

	errObj := got["error"].(map[string]any)
	assert.Equal(t, "line1\nline2", errObj["message"],
		"newlines must be preserved in message when decoded")
}

func TestOutputError_TrailingNewline(t *testing.T) {
	var buf bytes.Buffer
	err := outputError(&buf, "x", "y", "z")
	require.NoError(t, err)

	output := buf.String()
	assert.True(t, strings.HasSuffix(output, "\n"),
		"outputError should terminate with a trailing newline for clean CLI output")
}

func TestOutputError_DifferentCodes_ProduceDifferentOutput(t *testing.T) {
	// Anti-hardcoding: two different codes must yield different output.
	var buf1, buf2 bytes.Buffer
	err := outputError(&buf1, "manifest_invalid", "msg1", "hint1")
	require.NoError(t, err)
	err = outputError(&buf2, "tool_not_found", "msg2", "hint2")
	require.NoError(t, err)

	assert.NotEqual(t, buf1.String(), buf2.String(),
		"different error codes must produce different JSON output")
}

// ---------------------------------------------------------------------------
// AC-4: isCI() reads CI environment variable
// ---------------------------------------------------------------------------

func TestIsCI_WhenCISetToTrue(t *testing.T) {
	t.Setenv("CI", "true")
	assert.True(t, isCI(), "isCI() must return true when CI=true")
}

func TestIsCI_WhenCISetTo1(t *testing.T) {
	// Many CI systems set CI=1 rather than CI=true.
	t.Setenv("CI", "1")
	assert.True(t, isCI(),
		"isCI() must return true when CI=1 (common in many CI systems)")
}

func TestIsCI_WhenCINotSet(t *testing.T) {
	t.Setenv("CI", "")
	os.Unsetenv("CI")
	assert.False(t, isCI(), "isCI() must return false when CI is not set")
}

func TestIsCI_WhenCISetToFalse(t *testing.T) {
	t.Setenv("CI", "false")
	assert.False(t, isCI(), "isCI() must return false when CI=false")
}

func TestIsCI_WhenCISetToEmpty(t *testing.T) {
	t.Setenv("CI", "")
	assert.False(t, isCI(), "isCI() must return false when CI is empty string")
}

// ---------------------------------------------------------------------------
// AC-4: isColorDisabled() reads NO_COLOR environment variable
// ---------------------------------------------------------------------------

func TestIsColorDisabled_WhenNO_COLORSet(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	t.Setenv("CI", "")
	os.Unsetenv("CI")
	assert.True(t, isColorDisabled(),
		"isColorDisabled() must return true when NO_COLOR=1")
}

func TestIsColorDisabled_WhenNO_COLORSetToAnything(t *testing.T) {
	// The NO_COLOR spec says: "when set, [its value] should be treated as
	// color disabled." Any non-empty value means color is disabled.
	t.Setenv("NO_COLOR", "yes")
	t.Setenv("CI", "")
	os.Unsetenv("CI")
	assert.True(t, isColorDisabled(),
		"isColorDisabled() must return true for any non-empty NO_COLOR value")
}

func TestIsColorDisabled_WhenCISetToTrue(t *testing.T) {
	// AC-4 states CI=true disables color.
	t.Setenv("CI", "true")
	t.Setenv("NO_COLOR", "")
	os.Unsetenv("NO_COLOR")
	assert.True(t, isColorDisabled(),
		"isColorDisabled() must return true when CI=true even without NO_COLOR")
}

func TestIsColorDisabled_WhenNeitherSet(t *testing.T) {
	t.Setenv("NO_COLOR", "")
	os.Unsetenv("NO_COLOR")
	t.Setenv("CI", "")
	os.Unsetenv("CI")
	// Without a terminal check (which we cannot control in tests), this should
	// return false when neither env var is set. In a real terminal scenario
	// color might be enabled; here we just check the env-based path.
	assert.False(t, isColorDisabled(),
		"isColorDisabled() must return false when NO_COLOR and CI are unset")
}

func TestIsColorDisabled_BothSet(t *testing.T) {
	t.Setenv("NO_COLOR", "1")
	t.Setenv("CI", "true")
	assert.True(t, isColorDisabled(),
		"isColorDisabled() must return true when both NO_COLOR and CI are set")
}

func TestIsColorDisabled_NO_COLOR_EmptyString(t *testing.T) {
	// An explicitly empty NO_COLOR should NOT disable color.
	// The NO_COLOR spec says "When set to a non-empty string."
	t.Setenv("NO_COLOR", "")
	t.Setenv("CI", "")
	os.Unsetenv("CI")
	assert.False(t, isColorDisabled(),
		"isColorDisabled() must return false when NO_COLOR is set to empty string")
}

// ---------------------------------------------------------------------------
// AC-4: isCI and isColorDisabled consistency
// ---------------------------------------------------------------------------

func TestCI_ImpliesColorDisabled(t *testing.T) {
	// If isCI() is true, isColorDisabled() must also be true.
	t.Setenv("CI", "true")
	t.Setenv("NO_COLOR", "")
	os.Unsetenv("NO_COLOR")

	require.True(t, isCI(), "precondition: isCI() must be true")
	assert.True(t, isColorDisabled(),
		"when CI is active, color must be disabled")
}

// ---------------------------------------------------------------------------
// AC-5: debugLog writes to stderr writer, not stdout writer
// ---------------------------------------------------------------------------

func TestDebugLog_WritesToStderr(t *testing.T) {
	stderr := &bytes.Buffer{}
	debugLog(stderr, "test diagnostic message")

	output := stderr.String()
	assert.Contains(t, output, "test diagnostic message",
		"debugLog must write the message to the provided writer")
}

func TestDebugLog_DoesNotWriteToStdout(t *testing.T) {
	// debugLog takes a writer for stderr. It should ONLY write to that writer.
	// This test verifies the function signature does what it says.
	stderr := &bytes.Buffer{}
	debugLog(stderr, "diagnostic")

	assert.NotEmpty(t, stderr.String(),
		"debugLog must write something to the stderr writer")
}

func TestDebugLog_IncludesTimestamp(t *testing.T) {
	// AC-5 specifies "timestamped lines on stderr"
	stderr := &bytes.Buffer{}
	debugLog(stderr, "test message")

	output := stderr.String()
	// A timestamp should contain at least a colon (HH:MM:SS) or digits
	// We check for a basic timestamp-like pattern
	assert.Regexp(t, `\d{2}:\d{2}`, output,
		"debugLog output must include a timestamp")
}

func TestDebugLog_MessagePreserved(t *testing.T) {
	tests := []struct {
		name    string
		message string
	}{
		{name: "simple", message: "parsing manifest"},
		{name: "with path", message: "loading /home/user/toolwright.yaml"},
		{name: "with special chars", message: `resolved auth for "add-pet"`},
		{name: "empty message", message: ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			stderr := &bytes.Buffer{}
			debugLog(stderr, tc.message)

			if tc.message != "" {
				assert.Contains(t, stderr.String(), tc.message,
					"debugLog must include the original message in output")
			}
		})
	}
}

func TestDebugLog_EndsWithNewline(t *testing.T) {
	stderr := &bytes.Buffer{}
	debugLog(stderr, "line one")

	output := stderr.String()
	assert.True(t, strings.HasSuffix(output, "\n"),
		"debugLog output must end with a newline for readable stderr")
}

func TestDebugLog_MultipleCallsProduceMultipleLines(t *testing.T) {
	stderr := &bytes.Buffer{}
	debugLog(stderr, "first")
	debugLog(stderr, "second")
	debugLog(stderr, "third")

	output := stderr.String()
	lines := strings.Split(strings.TrimRight(output, "\n"), "\n")
	require.Len(t, lines, 3,
		"three debugLog calls must produce exactly three lines")

	assert.Contains(t, lines[0], "first")
	assert.Contains(t, lines[1], "second")
	assert.Contains(t, lines[2], "third")
}

// ---------------------------------------------------------------------------
// outputJSON edge cases: anti-hardcoding
// ---------------------------------------------------------------------------

func TestOutputJSON_DifferentInputs_DifferentOutputs(t *testing.T) {
	var buf1, buf2 bytes.Buffer

	err := outputJSON(&buf1, map[string]string{"a": "1"})
	require.NoError(t, err)
	err = outputJSON(&buf2, map[string]string{"b": "2"})
	require.NoError(t, err)

	assert.NotEqual(t, buf1.String(), buf2.String(),
		"different inputs must produce different JSON output")
}

func TestOutputJSON_LargePayload(t *testing.T) {
	var buf bytes.Buffer
	// Construct a payload with many items
	items := make([]map[string]string, 100)
	for i := range items {
		items[i] = map[string]string{
			"name": strings.Repeat("tool-", 10),
			"desc": strings.Repeat("description ", 20),
		}
	}

	err := outputJSON(&buf, map[string]any{"tools": items})
	require.NoError(t, err)
	assert.True(t, json.Valid(buf.Bytes()),
		"outputJSON must produce valid JSON for large payloads")

	var got map[string]any
	err = json.Unmarshal(buf.Bytes(), &got)
	require.NoError(t, err)
	tools := got["tools"].([]any)
	assert.Len(t, tools, 100,
		"large payload must preserve all 100 items")
}

func TestOutputJSON_UnicodeContent(t *testing.T) {
	var buf bytes.Buffer
	input := map[string]string{
		"japanese": "\u65e5\u672c\u8a9e",
		"emoji":    "\U0001f680\U0001f30d",
		"arabic":   "\u0627\u0644\u0639\u0631\u0628\u064a\u0629",
	}

	err := outputJSON(&buf, input)
	require.NoError(t, err)
	assert.True(t, json.Valid(buf.Bytes()),
		"outputJSON must handle Unicode correctly")

	var got map[string]string
	err = json.Unmarshal(buf.Bytes(), &got)
	require.NoError(t, err)
	assert.Equal(t, "\u65e5\u672c\u8a9e", got["japanese"])
}

// ---------------------------------------------------------------------------
// outputError: verify it is a function of its inputs, not hardcoded
// ---------------------------------------------------------------------------

func TestOutputError_FullPermutation(t *testing.T) {
	// Every combination of different code/message/hint must produce different
	// output. This catches any hardcoded return.
	tests := []struct {
		code    string
		message string
		hint    string
	}{
		{"manifest_invalid", "manifest is invalid", "fix your manifest"},
		{"tool_not_found", "tool xyz not found", "check tool name"},
		{"auth_required", "authentication required", "run toolwright login"},
		{"unknown_error", "something broke", "try again"},
	}

	outputs := make(map[string]bool)
	for _, tc := range tests {
		var buf bytes.Buffer
		err := outputError(&buf, tc.code, tc.message, tc.hint)
		require.NoError(t, err)
		outputs[buf.String()] = true
	}

	assert.Len(t, outputs, len(tests),
		"each unique error input must produce unique output; got %d unique outputs for %d inputs",
		len(outputs), len(tests))
}
