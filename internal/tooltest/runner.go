package tooltest

import (
	"context"
	"fmt"
	"io/fs"
	"strings"
	"sync"
	"time"

	"github.com/Obsidian-Owl/toolwright/internal/manifest"
	"github.com/Obsidian-Owl/toolwright/internal/runner"
)

// DefaultTimeout is the per-test timeout applied when TestCase.Timeout is zero.
const DefaultTimeout = 30 * time.Second

// ToolExecutor abstracts running a single tool invocation.
type ToolExecutor interface {
	Run(ctx context.Context, tool manifest.Tool, args []string, flags map[string]string, token string) (*runner.Result, error)
}

// TestRunner orchestrates test suite execution.
type TestRunner struct {
	Executor ToolExecutor
	SchemaFS fs.FS // for stdout_schema validation; may be nil
}

// Run executes all test cases in the suite sequentially against the matching
// tool in toolkit. Returns an error only for suite-level failures (e.g., tool
// not found). Individual test failures are recorded in the returned TestReport.
func (tr *TestRunner) Run(ctx context.Context, suite TestSuite, toolkit manifest.Toolkit) (*TestReport, error) {
	tool, err := findTool(suite.Tool, toolkit)
	if err != nil {
		return nil, err
	}

	results := make([]TestResult, 0, len(suite.Tests))
	for _, tc := range suite.Tests {
		// Stop if the parent context is done.
		if ctx.Err() != nil {
			break
		}
		result := tr.runTestCase(ctx, tool, tc)
		results = append(results, result)
	}

	return buildReport(suite.Tool, results), nil
}

// RunParallel executes all test cases in the suite concurrently, limited to
// workers goroutines at a time. Results are returned in original suite order.
// If workers <= 0, it is treated as 1.
func (tr *TestRunner) RunParallel(ctx context.Context, suite TestSuite, toolkit manifest.Toolkit, workers int) (*TestReport, error) {
	tool, err := findTool(suite.Tool, toolkit)
	if err != nil {
		return nil, err
	}

	if workers <= 0 {
		workers = 1
	}

	n := len(suite.Tests)
	results := make([]TestResult, n)

	// Semaphore channel limits concurrency.
	sem := make(chan struct{}, workers)
	var wg sync.WaitGroup

	for i, tc := range suite.Tests {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}        // acquire
			defer func() { <-sem }() // release
			results[i] = tr.runTestCase(ctx, tool, tc)
		}()
	}

	wg.Wait()
	return buildReport(suite.Tool, results), nil
}

// runTestCase executes a single test case and returns a TestResult.
func (tr *TestRunner) runTestCase(ctx context.Context, tool manifest.Tool, tc TestCase) TestResult {
	timeout := tc.Timeout
	if timeout == 0 {
		timeout = DefaultTimeout
	}

	tctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	start := time.Now()
	result, err := tr.Executor.Run(tctx, tool, tc.Args, tc.Flags, tc.AuthToken)
	wallDuration := time.Since(start)

	if err != nil {
		return TestResult{
			Name:     tc.Name,
			Status:   "fail",
			Duration: wallDuration,
			Error:    err.Error(),
		}
	}

	// Determine duration: prefer result.Duration if non-zero, else wall clock.
	duration := result.Duration
	if duration == 0 {
		duration = wallDuration
	}

	assertionResults := EvaluateAssertions(result.Stdout, result.Stderr, result.ExitCode, tc.Expect, tr.SchemaFS)

	var failed []AssertionResult
	for _, ar := range assertionResults {
		if !ar.Passed {
			failed = append(failed, ar)
		}
	}

	if len(failed) > 0 {
		return TestResult{
			Name:     tc.Name,
			Status:   "fail",
			Duration: duration,
			Error:    buildFailureMessage(failed),
			Stdout:   result.Stdout,
			Stderr:   result.Stderr,
		}
	}

	return TestResult{
		Name:     tc.Name,
		Status:   "pass",
		Duration: duration,
		Stdout:   result.Stdout,
		Stderr:   result.Stderr,
	}
}

// findTool returns the tool with the given name from toolkit, or an error.
func findTool(name string, toolkit manifest.Toolkit) (manifest.Tool, error) {
	for _, t := range toolkit.Tools {
		if t.Name == name {
			return t, nil
		}
	}
	return manifest.Tool{}, fmt.Errorf("tool %q not found in toolkit", name)
}

// buildReport assembles a TestReport from a slice of results.
func buildReport(toolName string, results []TestResult) *TestReport {
	var passed, failed int
	for _, r := range results {
		switch r.Status {
		case "pass":
			passed++
		case "fail":
			failed++
		}
	}
	return &TestReport{
		Tool:    toolName,
		Total:   len(results),
		Passed:  passed,
		Failed:  failed,
		Results: results,
	}
}

// buildFailureMessage constructs a human-readable description of assertion failures.
func buildFailureMessage(failed []AssertionResult) string {
	var b strings.Builder
	for i, ar := range failed {
		if i > 0 {
			b.WriteString("; ")
		}
		b.WriteString(ar.Type)
		if ar.Path != "" {
			b.WriteString(" at ")
			b.WriteString(ar.Path)
		}
		if ar.Expected != "" || ar.Actual != "" {
			fmt.Fprintf(&b, " (expected %s, got %s)", ar.Expected, ar.Actual)
		}
		if ar.Error != "" {
			b.WriteString(": ")
			b.WriteString(ar.Error)
		}
	}
	return b.String()
}
