package tooltest

import (
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// FormatTAP writes TAP version 13 output for the given report to w.
func FormatTAP(report TestReport, w io.Writer) error {
	if _, err := fmt.Fprintf(w, "TAP version 13\n"); err != nil {
		return fmt.Errorf("FormatTAP: write header: %w", err)
	}
	if _, err := fmt.Fprintf(w, "1..%d\n", report.Total); err != nil {
		return fmt.Errorf("FormatTAP: write plan: %w", err)
	}

	for i, r := range report.Results {
		n := i + 1
		if r.Status == "pass" {
			if _, err := fmt.Fprintf(w, "ok %d - %s\n", n, r.Name); err != nil {
				return fmt.Errorf("FormatTAP: write ok line: %w", err)
			}
		} else {
			if _, err := fmt.Fprintf(w, "not ok %d - %s\n", n, r.Name); err != nil {
				return fmt.Errorf("FormatTAP: write not ok line: %w", err)
			}
			durationMS := float64(r.Duration) / float64(time.Millisecond)
			_, err := fmt.Fprintf(w, "  ---\n    error: %s\n    duration_ms: %g\n  ...\n",
				r.Error, durationMS)
			if err != nil {
				return fmt.Errorf("FormatTAP: write diagnostics: %w", err)
			}
		}
	}

	return nil
}

// jsonReport is the top-level JSON output structure.
type jsonReport struct {
	Tool    string `json:"tool"`
	Total   int    `json:"total"`
	Passed  int    `json:"passed"`
	Failed  int    `json:"failed"`
	Results []any  `json:"results"`
}

// FormatJSON writes JSON output for the given report to w.
func FormatJSON(report TestReport, w io.Writer) error {
	results := make([]any, 0, len(report.Results))
	for _, r := range report.Results {
		durationMS := float64(r.Duration) / float64(time.Millisecond)
		if r.Status == "fail" {
			results = append(results, map[string]any{
				"name":        r.Name,
				"status":      r.Status,
				"duration_ms": durationMS,
				"error":       r.Error,
				"stdout":      string(r.Stdout),
				"stderr":      string(r.Stderr),
			})
		} else {
			entry := map[string]any{
				"name":        r.Name,
				"status":      r.Status,
				"duration_ms": durationMS,
			}
			if len(r.Stdout) > 0 {
				entry["stdout"] = string(r.Stdout)
			}
			results = append(results, entry)
		}
	}

	out := jsonReport{
		Tool:    report.Tool,
		Total:   report.Total,
		Passed:  report.Passed,
		Failed:  report.Failed,
		Results: results,
	}

	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return fmt.Errorf("FormatJSON: marshal: %w", err)
	}
	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("FormatJSON: write: %w", err)
	}
	if _, err := w.Write([]byte("\n")); err != nil {
		return fmt.Errorf("FormatJSON: write newline: %w", err)
	}
	return nil
}
