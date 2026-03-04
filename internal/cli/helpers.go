package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"
)

// outputJSON writes v as indented JSON followed by a newline to w.
func outputJSON(w io.Writer, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal JSON: %w", err)
	}
	b = append(b, '\n')
	if _, err := w.Write(b); err != nil {
		return fmt.Errorf("write JSON: %w", err)
	}
	return nil
}

// outputError writes a structured JSON error object to w with the shape:
//
//	{"error":{"code":"...","message":"...","hint":"..."}}
func outputError(w io.Writer, code, message, hint string) error {
	payload := map[string]any{
		"error": map[string]string{
			"code":    code,
			"message": message,
			"hint":    hint,
		},
	}
	return outputJSON(w, payload)
}

// isCI returns true when the CI environment variable is "true" or "1".
func isCI() bool {
	v := os.Getenv("CI")
	return v == "true" || v == "1"
}

// isColorDisabled returns true when NO_COLOR is set to a non-empty string or
// when isCI() returns true (CI environments imply no color per AC-4).
func isColorDisabled() bool {
	if os.Getenv("NO_COLOR") != "" {
		return true
	}
	return isCI()
}

// debugLog writes a timestamped diagnostic line to w.
func debugLog(w io.Writer, msg string) {
	ts := time.Now().Format("15:04:05")
	fmt.Fprintf(w, "[%s] %s\n", ts, msg)
}
