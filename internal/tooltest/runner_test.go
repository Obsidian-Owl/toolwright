package tooltest

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Obsidian-Owl/toolwright/internal/manifest"
	"github.com/Obsidian-Owl/toolwright/internal/runner"
)

// ---------------------------------------------------------------------------
// Mock executor
// ---------------------------------------------------------------------------

// mockCall records a single invocation of the mock executor.
type mockCall struct {
	ToolName string
	Args     []string
	Flags    map[string]string
	Token    string
}

// mockResponse configures what the mock executor returns for a given test.
type mockResponse struct {
	result *runner.Result
	err    error
	delay  time.Duration
}

// mockExecutor implements ToolExecutor for testing. It tracks calls, supports
// configurable responses per test name, and measures peak concurrency.
type mockExecutor struct {
	mu            sync.Mutex
	calls         []mockCall
	responses     map[string]mockResponse // keyed by first positional arg as test identifier
	defaultResult *runner.Result

	// Concurrency tracking via atomics (safe for -race).
	concurrent    int32
	maxConcurrent int32
}

// newMockExecutor creates a mock executor with a default success result.
func newMockExecutor() *mockExecutor {
	return &mockExecutor{
		responses: make(map[string]mockResponse),
		defaultResult: &runner.Result{
			ExitCode: 0,
			Stdout:   []byte(`{"ok":true}`),
			Stderr:   nil,
			Duration: 10 * time.Millisecond,
		},
	}
}

// setResponse configures the response for a test identified by name.
// The name must match the TestCase.Name that the runner maps to executor calls.
func (m *mockExecutor) setResponse(name string, resp mockResponse) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.responses[name] = resp
}

// getCalls returns a copy of all recorded calls.
func (m *mockExecutor) getCalls() []mockCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]mockCall, len(m.calls))
	copy(result, m.calls)
	return result
}

// getMaxConcurrent returns the peak number of concurrent Run invocations.
func (m *mockExecutor) getMaxConcurrent() int32 {
	return atomic.LoadInt32(&m.maxConcurrent)
}

// Run implements ToolExecutor. It records the call, sleeps for any configured
// delay, and returns the configured response. The testName is passed as the
// first element of args to identify which response to use.
func (m *mockExecutor) Run(ctx context.Context, tool manifest.Tool, args []string, flags map[string]string, token string) (*runner.Result, error) {
	// Track concurrency.
	cur := atomic.AddInt32(&m.concurrent, 1)
	for {
		old := atomic.LoadInt32(&m.maxConcurrent)
		if cur <= old || atomic.CompareAndSwapInt32(&m.maxConcurrent, old, cur) {
			break
		}
	}
	defer atomic.AddInt32(&m.concurrent, -1)

	// Record the call.
	m.mu.Lock()
	// We need a way to identify which test case triggered this call.
	// The runner should pass the test case name somehow. Since the spec says
	// args come from TestCase.Args, we look up by what's available.
	// For the mock, we'll look up by tool name + some identifier.
	// We store all info and look up the response by iterating.
	call := mockCall{
		ToolName: tool.Name,
		Args:     args,
		Flags:    flags,
		Token:    token,
	}
	m.calls = append(m.calls, call)

	// Find matching response. The test must set up responses keyed by
	// a test-case-identifiable key. We'll look up by the response map.
	// Since we don't have the test case name directly, the test setup
	// must use args or flags to differentiate. But the runner knows the
	// test case name — we need a way to map. We'll use a sequential
	// counter approach: look up by call index.
	callIdx := len(m.calls) - 1
	m.mu.Unlock()

	// Look up response by call index or by a key derived from args.
	m.mu.Lock()
	// Try a name-based lookup first (test setups can use whatever key).
	var resp mockResponse
	var found bool
	// Try keying by first arg, which many tests use as an identifier.
	if len(args) > 0 {
		resp, found = m.responses[args[0]]
	}
	if !found {
		// Try keying by call index formatted as string.
		resp, found = m.responses[fmt.Sprintf("%d", callIdx)]
	}
	m.mu.Unlock()

	if !found {
		resp = mockResponse{result: m.defaultResult}
	}

	// Simulate delay (respecting context cancellation).
	if resp.delay > 0 {
		select {
		case <-time.After(resp.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	if resp.err != nil {
		return nil, resp.err
	}
	return resp.result, nil
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// sampleToolkit returns a toolkit with the given tool names for testing.
func sampleToolkit(toolNames ...string) manifest.Toolkit {
	tools := make([]manifest.Tool, len(toolNames))
	for i, name := range toolNames {
		tools[i] = manifest.Tool{
			Name:       name,
			Entrypoint: fmt.Sprintf("./%s.sh", name),
		}
	}
	return manifest.Toolkit{Tools: tools}
}

// ---------------------------------------------------------------------------
// AC-9: Exit code assertion (via runner)
// ---------------------------------------------------------------------------

func TestRun_ExitCodePass(t *testing.T) {
	// Test expects exit_code: 0, tool exits 0 → test passes.
	mock := newMockExecutor()
	mock.defaultResult = &runner.Result{
		ExitCode: 0,
		Stdout:   []byte("ok"),
		Duration: 5 * time.Millisecond,
	}

	tr := &TestRunner{Executor: mock}

	suite := TestSuite{
		Tool: "mytool",
		Tests: []TestCase{
			{
				Name: "exits zero",
				Args: []string{"arg1"},
				Expect: Expectation{
					ExitCode: intPtr(0),
				},
			},
		},
	}

	report, err := tr.Run(context.Background(), suite, sampleToolkit("mytool"))
	require.NoError(t, err, "Run must not return error for valid suite")
	require.NotNil(t, report, "Report must not be nil")

	assert.Equal(t, "mytool", report.Tool, "Report.Tool must match suite.Tool")
	assert.Equal(t, 1, report.Total, "Total must be 1")
	assert.Equal(t, 1, report.Passed, "Passed must be 1")
	assert.Equal(t, 0, report.Failed, "Failed must be 0")
	require.Len(t, report.Results, 1, "Results must have exactly 1 entry")

	r := report.Results[0]
	assert.Equal(t, "exits zero", r.Name, "Result name must match test case name")
	assert.Equal(t, "pass", r.Status, "Status must be 'pass' when exit codes match")
	assert.Empty(t, r.Error, "Error must be empty on pass")
}

func TestRun_ExitCodeFail_ExpectedVsActual(t *testing.T) {
	// Test expects exit_code: 2, tool exits 1 → test fails.
	mock := newMockExecutor()
	mock.defaultResult = &runner.Result{
		ExitCode: 1,
		Stdout:   nil,
		Stderr:   []byte("error: bad input"),
		Duration: 5 * time.Millisecond,
	}

	tr := &TestRunner{Executor: mock}

	suite := TestSuite{
		Tool: "mytool",
		Tests: []TestCase{
			{
				Name: "expect exit 2",
				Args: []string{"bad-input"},
				Expect: Expectation{
					ExitCode: intPtr(2),
				},
			},
		},
	}

	report, err := tr.Run(context.Background(), suite, sampleToolkit("mytool"))
	require.NoError(t, err, "Run must not return error; test failure is in the report")
	require.NotNil(t, report)

	assert.Equal(t, 1, report.Total)
	assert.Equal(t, 0, report.Passed, "Passed must be 0 when exit code mismatches")
	assert.Equal(t, 1, report.Failed, "Failed must be 1 when exit code mismatches")
	require.Len(t, report.Results, 1)

	r := report.Results[0]
	assert.Equal(t, "expect exit 2", r.Name)
	assert.Equal(t, "fail", r.Status, "Status must be 'fail' when exit code mismatches")
	assert.NotEmpty(t, r.Error, "Error must describe the exit code mismatch")
	// The error message must contain both expected and actual values so the
	// user can diagnose. A lazy implementation that says "assertion failed"
	// without specifics is inadequate.
	assert.Contains(t, r.Error, "2", "Error must mention expected exit code 2")
	assert.Contains(t, r.Error, "1", "Error must mention actual exit code 1")
}

func TestRun_ExitCodeMismatch_MultipleTests(t *testing.T) {
	// 3 tests: first passes (exit 0), second fails (expect 2 got 1), third passes (exit 1).
	// Verifies counting is correct with mixed results.
	mock := newMockExecutor()
	mock.setResponse("pass1", mockResponse{
		result: &runner.Result{ExitCode: 0, Duration: 5 * time.Millisecond},
	})
	mock.setResponse("failcase", mockResponse{
		result: &runner.Result{ExitCode: 1, Duration: 5 * time.Millisecond},
	})
	mock.setResponse("pass2", mockResponse{
		result: &runner.Result{ExitCode: 1, Duration: 5 * time.Millisecond},
	})

	tr := &TestRunner{Executor: mock}

	suite := TestSuite{
		Tool: "multi-exit",
		Tests: []TestCase{
			{
				Name:   "first pass",
				Args:   []string{"pass1"},
				Expect: Expectation{ExitCode: intPtr(0)},
			},
			{
				Name:   "second fail",
				Args:   []string{"failcase"},
				Expect: Expectation{ExitCode: intPtr(2)}, // expects 2, gets 1
			},
			{
				Name:   "third pass",
				Args:   []string{"pass2"},
				Expect: Expectation{ExitCode: intPtr(1)}, // expects 1, gets 1
			},
		},
	}

	report, err := tr.Run(context.Background(), suite, sampleToolkit("multi-exit"))
	require.NoError(t, err)
	require.NotNil(t, report)

	assert.Equal(t, "multi-exit", report.Tool)
	assert.Equal(t, 3, report.Total, "Total must be 3")
	assert.Equal(t, 2, report.Passed, "Passed must be 2")
	assert.Equal(t, 1, report.Failed, "Failed must be 1")
	require.Len(t, report.Results, 3)

	assert.Equal(t, "first pass", report.Results[0].Name)
	assert.Equal(t, "pass", report.Results[0].Status)

	assert.Equal(t, "second fail", report.Results[1].Name)
	assert.Equal(t, "fail", report.Results[1].Status)

	assert.Equal(t, "third pass", report.Results[2].Name)
	assert.Equal(t, "pass", report.Results[2].Status)
}

// ---------------------------------------------------------------------------
// Tool not found
// ---------------------------------------------------------------------------

func TestRun_ToolNotFound_ReturnsError(t *testing.T) {
	mock := newMockExecutor()
	tr := &TestRunner{Executor: mock}

	suite := TestSuite{
		Tool: "nonexistent-tool",
		Tests: []TestCase{
			{Name: "some test", Args: []string{"x"}},
		},
	}

	// Toolkit does NOT contain "nonexistent-tool".
	toolkit := sampleToolkit("other-tool", "another-tool")

	report, err := tr.Run(context.Background(), suite, toolkit)
	require.Error(t, err, "Run must return error when tool not found in toolkit")
	assert.Nil(t, report, "Report must be nil when tool lookup fails")
	assert.Contains(t, err.Error(), "nonexistent-tool",
		"Error must mention the missing tool name")
}

// ---------------------------------------------------------------------------
// Empty suite
// ---------------------------------------------------------------------------

func TestRun_EmptySuite_NoTests(t *testing.T) {
	mock := newMockExecutor()
	tr := &TestRunner{Executor: mock}

	suite := TestSuite{
		Tool:  "mytool",
		Tests: []TestCase{},
	}

	report, err := tr.Run(context.Background(), suite, sampleToolkit("mytool"))
	require.NoError(t, err, "Empty suite must not produce an error")
	require.NotNil(t, report, "Report must not be nil for empty suite")

	assert.Equal(t, "mytool", report.Tool)
	assert.Equal(t, 0, report.Total, "Total must be 0 for empty suite")
	assert.Equal(t, 0, report.Passed)
	assert.Equal(t, 0, report.Failed)
	assert.Empty(t, report.Results, "Results must be empty for empty suite")

	// Executor must not have been called.
	assert.Empty(t, mock.getCalls(), "Executor must not be called with no tests")
}

// ---------------------------------------------------------------------------
// Executor error propagation
// ---------------------------------------------------------------------------

func TestRun_ExecutorError_ResultIsFail(t *testing.T) {
	// When Executor.Run returns an error (not a non-zero exit code, but a
	// true execution failure), the test result should be "fail" with the
	// error message.
	mock := newMockExecutor()
	mock.setResponse("boom", mockResponse{
		err: errors.New("entrypoint not found: ./missing.sh"),
	})

	tr := &TestRunner{Executor: mock}

	suite := TestSuite{
		Tool: "mytool",
		Tests: []TestCase{
			{
				Name:   "executor fails",
				Args:   []string{"boom"},
				Expect: Expectation{ExitCode: intPtr(0)},
			},
		},
	}

	report, err := tr.Run(context.Background(), suite, sampleToolkit("mytool"))
	require.NoError(t, err, "Suite-level error must be nil; test failure goes into report")
	require.NotNil(t, report)

	assert.Equal(t, 1, report.Total)
	assert.Equal(t, 0, report.Passed)
	assert.Equal(t, 1, report.Failed)
	require.Len(t, report.Results, 1)

	r := report.Results[0]
	assert.Equal(t, "executor fails", r.Name)
	assert.Equal(t, "fail", r.Status)
	assert.NotEmpty(t, r.Error, "Error must describe the executor failure")
	assert.Contains(t, r.Error, "entrypoint not found",
		"Error must propagate the executor's error message")
}

// ---------------------------------------------------------------------------
// Auth token, args, and flags passthrough
// ---------------------------------------------------------------------------

func TestRun_ArgsPassedToExecutor(t *testing.T) {
	mock := newMockExecutor()
	tr := &TestRunner{Executor: mock}

	suite := TestSuite{
		Tool: "mytool",
		Tests: []TestCase{
			{
				Name: "with args",
				Args: []string{"file.txt", "--verbose"},
				Expect: Expectation{
					ExitCode: intPtr(0),
				},
			},
		},
	}

	_, err := tr.Run(context.Background(), suite, sampleToolkit("mytool"))
	require.NoError(t, err)

	calls := mock.getCalls()
	require.Len(t, calls, 1, "Executor must be called exactly once")
	assert.Equal(t, []string{"file.txt", "--verbose"}, calls[0].Args,
		"Args must be passed through to executor unchanged")
}

func TestRun_FlagsPassedToExecutor(t *testing.T) {
	mock := newMockExecutor()
	tr := &TestRunner{Executor: mock}

	suite := TestSuite{
		Tool: "mytool",
		Tests: []TestCase{
			{
				Name:  "with flags",
				Args:  []string{"x"},
				Flags: map[string]string{"format": "json", "limit": "50"},
				Expect: Expectation{
					ExitCode: intPtr(0),
				},
			},
		},
	}

	_, err := tr.Run(context.Background(), suite, sampleToolkit("mytool"))
	require.NoError(t, err)

	calls := mock.getCalls()
	require.Len(t, calls, 1)
	assert.Equal(t, map[string]string{"format": "json", "limit": "50"}, calls[0].Flags,
		"Flags must be passed through to executor unchanged")
}

func TestRun_AuthTokenPassedToExecutor(t *testing.T) {
	mock := newMockExecutor()
	tr := &TestRunner{Executor: mock}

	suite := TestSuite{
		Tool: "authtool",
		Tests: []TestCase{
			{
				Name:      "with token",
				Args:      []string{"x"},
				AuthToken: "secret-token-123",
				Expect: Expectation{
					ExitCode: intPtr(0),
				},
			},
		},
	}

	_, err := tr.Run(context.Background(), suite, sampleToolkit("authtool"))
	require.NoError(t, err)

	calls := mock.getCalls()
	require.Len(t, calls, 1)
	assert.Equal(t, "secret-token-123", calls[0].Token,
		"AuthToken from test case must be passed as token to executor")
}

func TestRun_EmptyAuthToken_PassedAsEmpty(t *testing.T) {
	mock := newMockExecutor()
	tr := &TestRunner{Executor: mock}

	suite := TestSuite{
		Tool: "mytool",
		Tests: []TestCase{
			{
				Name:      "no token",
				Args:      []string{"x"},
				AuthToken: "",
				Expect:    Expectation{ExitCode: intPtr(0)},
			},
		},
	}

	_, err := tr.Run(context.Background(), suite, sampleToolkit("mytool"))
	require.NoError(t, err)

	calls := mock.getCalls()
	require.Len(t, calls, 1)
	assert.Equal(t, "", calls[0].Token,
		"Empty AuthToken must be passed as empty string, not omitted")
}

func TestRun_ToolNamePassedToExecutor(t *testing.T) {
	// The correct tool from the toolkit must be resolved and passed to executor.
	mock := newMockExecutor()
	tr := &TestRunner{Executor: mock}

	toolkit := manifest.Toolkit{
		Tools: []manifest.Tool{
			{Name: "other", Entrypoint: "./other.sh"},
			{Name: "target", Entrypoint: "./target.sh"},
		},
	}

	suite := TestSuite{
		Tool: "target",
		Tests: []TestCase{
			{
				Name:   "correct tool",
				Args:   []string{"x"},
				Expect: Expectation{ExitCode: intPtr(0)},
			},
		},
	}

	_, err := tr.Run(context.Background(), suite, toolkit)
	require.NoError(t, err)

	calls := mock.getCalls()
	require.Len(t, calls, 1)
	assert.Equal(t, "target", calls[0].ToolName,
		"Executor must receive the correct tool resolved from the toolkit")
}

// ---------------------------------------------------------------------------
// Stdout/Stderr capture in results
// ---------------------------------------------------------------------------

func TestRun_StdoutStderrCaptured(t *testing.T) {
	mock := newMockExecutor()
	mock.setResponse("capture", mockResponse{
		result: &runner.Result{
			ExitCode: 0,
			Stdout:   []byte(`{"result":"captured"}`),
			Stderr:   []byte("warning: something"),
			Duration: 10 * time.Millisecond,
		},
	})

	tr := &TestRunner{Executor: mock}

	suite := TestSuite{
		Tool: "mytool",
		Tests: []TestCase{
			{
				Name:   "capture test",
				Args:   []string{"capture"},
				Expect: Expectation{ExitCode: intPtr(0)},
			},
		},
	}

	report, err := tr.Run(context.Background(), suite, sampleToolkit("mytool"))
	require.NoError(t, err)
	require.Len(t, report.Results, 1)

	r := report.Results[0]
	assert.Equal(t, []byte(`{"result":"captured"}`), r.Stdout,
		"Stdout from executor must be captured in test result")
	assert.Equal(t, []byte("warning: something"), r.Stderr,
		"Stderr from executor must be captured in test result")
}

// ---------------------------------------------------------------------------
// Duration capture
// ---------------------------------------------------------------------------

func TestRun_DurationCaptured(t *testing.T) {
	mock := newMockExecutor()
	mock.setResponse("slow", mockResponse{
		result: &runner.Result{
			ExitCode: 0,
			Duration: 100 * time.Millisecond,
		},
		delay: 50 * time.Millisecond,
	})

	tr := &TestRunner{Executor: mock}

	suite := TestSuite{
		Tool: "mytool",
		Tests: []TestCase{
			{
				Name:   "timed test",
				Args:   []string{"slow"},
				Expect: Expectation{ExitCode: intPtr(0)},
			},
		},
	}

	report, err := tr.Run(context.Background(), suite, sampleToolkit("mytool"))
	require.NoError(t, err)
	require.Len(t, report.Results, 1)

	r := report.Results[0]
	assert.Greater(t, r.Duration, time.Duration(0),
		"Duration must be non-zero for a test that took time")
}

// ---------------------------------------------------------------------------
// Report fields correctness
// ---------------------------------------------------------------------------

func TestRun_ReportFieldsFromSuite_NotHardcoded(t *testing.T) {
	// Anti-hardcoding: run the same test with two different tool names
	// and verify the report reflects each one.
	for _, toolName := range []string{"alpha-tool", "beta-tool"} {
		t.Run(toolName, func(t *testing.T) {
			mock := newMockExecutor()
			tr := &TestRunner{Executor: mock}

			suite := TestSuite{
				Tool: toolName,
				Tests: []TestCase{
					{
						Name:   "test-a",
						Args:   []string{"x"},
						Expect: Expectation{ExitCode: intPtr(0)},
					},
					{
						Name:   "test-b",
						Args:   []string{"x"},
						Expect: Expectation{ExitCode: intPtr(0)},
					},
				},
			}

			report, err := tr.Run(context.Background(), suite, sampleToolkit(toolName))
			require.NoError(t, err)
			require.NotNil(t, report)

			assert.Equal(t, toolName, report.Tool,
				"Report.Tool must come from suite, not be hardcoded")
			assert.Equal(t, 2, report.Total,
				"Total must equal len(suite.Tests)")
			assert.Equal(t, report.Total, len(report.Results),
				"Total must equal len(Results)")
			assert.Equal(t, report.Total, report.Passed+report.Failed,
				"Passed + Failed must equal Total")
		})
	}
}

func TestRun_ResultsPreserveOrder(t *testing.T) {
	// Results must be in the same order as Tests in the suite.
	mock := newMockExecutor()
	tr := &TestRunner{Executor: mock}

	names := []string{"aaa", "bbb", "ccc", "ddd"}
	cases := make([]TestCase, len(names))
	for i, n := range names {
		cases[i] = TestCase{
			Name:   n,
			Args:   []string{"x"},
			Expect: Expectation{ExitCode: intPtr(0)},
		}
	}

	suite := TestSuite{
		Tool:  "mytool",
		Tests: cases,
	}

	report, err := tr.Run(context.Background(), suite, sampleToolkit("mytool"))
	require.NoError(t, err)
	require.Len(t, report.Results, 4)

	for i, n := range names {
		assert.Equal(t, n, report.Results[i].Name,
			"Result[%d] must be %q (preserving original order)", i, n)
	}
}

// ---------------------------------------------------------------------------
// AC-16: Per-test timeout
// ---------------------------------------------------------------------------

func TestRun_Timeout_TestTimesOut(t *testing.T) {
	// Test with timeout 100ms, mock delays 2s → test must fail with timeout.
	mock := newMockExecutor()
	mock.setResponse("slowop", mockResponse{
		result: &runner.Result{ExitCode: 0, Duration: 2 * time.Second},
		delay:  2 * time.Second,
	})

	tr := &TestRunner{Executor: mock}

	suite := TestSuite{
		Tool: "mytool",
		Tests: []TestCase{
			{
				Name:    "times out",
				Args:    []string{"slowop"},
				Timeout: 100 * time.Millisecond,
				Expect:  Expectation{ExitCode: intPtr(0)},
			},
		},
	}

	start := time.Now()
	report, err := tr.Run(context.Background(), suite, sampleToolkit("mytool"))
	elapsed := time.Since(start)

	require.NoError(t, err, "Suite-level error must be nil; timeout is a test failure")
	require.NotNil(t, report)
	require.Len(t, report.Results, 1)

	r := report.Results[0]
	assert.Equal(t, "fail", r.Status, "Timed-out test must have status 'fail'")
	assert.NotEmpty(t, r.Error, "Timed-out test must have an error message")
	// The error should mention timeout or context deadline.
	assert.True(t,
		containsAny(r.Error, "timeout", "deadline", "context"),
		"Error message %q must mention timeout or deadline", r.Error)

	// The test must not have waited the full 2s delay.
	assert.Less(t, elapsed, 1*time.Second,
		"Test with 100ms timeout must not wait 2s; elapsed=%v", elapsed)

	// Report counts.
	assert.Equal(t, 0, report.Passed)
	assert.Equal(t, 1, report.Failed)
}

func TestRun_Timeout_FastTestWithTimeout_Passes(t *testing.T) {
	// Test with timeout 5s and mock that responds in 10ms → passes.
	mock := newMockExecutor()
	mock.setResponse("fastop", mockResponse{
		result: &runner.Result{ExitCode: 0, Duration: 10 * time.Millisecond},
		delay:  10 * time.Millisecond,
	})

	tr := &TestRunner{Executor: mock}

	suite := TestSuite{
		Tool: "mytool",
		Tests: []TestCase{
			{
				Name:    "fast enough",
				Args:    []string{"fastop"},
				Timeout: 5 * time.Second,
				Expect:  Expectation{ExitCode: intPtr(0)},
			},
		},
	}

	report, err := tr.Run(context.Background(), suite, sampleToolkit("mytool"))
	require.NoError(t, err)
	require.Len(t, report.Results, 1)

	assert.Equal(t, "pass", report.Results[0].Status,
		"Fast test within timeout must pass")
}

func TestRun_Timeout_DefaultApplied(t *testing.T) {
	// Test without explicit Timeout. The runner should apply a default (30s).
	// We verify by checking that a fast test still passes (i.e., the runner
	// does not use zero timeout which would cancel immediately).
	mock := newMockExecutor()
	mock.setResponse("quickop", mockResponse{
		result: &runner.Result{ExitCode: 0, Duration: 5 * time.Millisecond},
		delay:  5 * time.Millisecond,
	})

	tr := &TestRunner{Executor: mock}

	suite := TestSuite{
		Tool: "mytool",
		Tests: []TestCase{
			{
				Name:    "no explicit timeout",
				Args:    []string{"quickop"},
				Timeout: 0, // zero = use default
				Expect:  Expectation{ExitCode: intPtr(0)},
			},
		},
	}

	report, err := tr.Run(context.Background(), suite, sampleToolkit("mytool"))
	require.NoError(t, err)
	require.Len(t, report.Results, 1)

	assert.Equal(t, "pass", report.Results[0].Status,
		"Test with no explicit timeout must use a default and not timeout immediately")
}

func TestRun_Timeout_MixedTimeouts(t *testing.T) {
	// Suite with 3 tests: one times out, two pass. Verifies that per-test
	// timeouts are isolated (one test timing out does not affect others).
	mock := newMockExecutor()
	mock.setResponse("fast1", mockResponse{
		result: &runner.Result{ExitCode: 0, Duration: 5 * time.Millisecond},
		delay:  5 * time.Millisecond,
	})
	mock.setResponse("slowblock", mockResponse{
		result: &runner.Result{ExitCode: 0, Duration: 5 * time.Second},
		delay:  5 * time.Second,
	})
	mock.setResponse("fast2", mockResponse{
		result: &runner.Result{ExitCode: 0, Duration: 5 * time.Millisecond},
		delay:  5 * time.Millisecond,
	})

	tr := &TestRunner{Executor: mock}

	suite := TestSuite{
		Tool: "mytool",
		Tests: []TestCase{
			{
				Name:    "first fast",
				Args:    []string{"fast1"},
				Timeout: 1 * time.Second,
				Expect:  Expectation{ExitCode: intPtr(0)},
			},
			{
				Name:    "slow blocks",
				Args:    []string{"slowblock"},
				Timeout: 50 * time.Millisecond,
				Expect:  Expectation{ExitCode: intPtr(0)},
			},
			{
				Name:    "second fast",
				Args:    []string{"fast2"},
				Timeout: 1 * time.Second,
				Expect:  Expectation{ExitCode: intPtr(0)},
			},
		},
	}

	report, err := tr.Run(context.Background(), suite, sampleToolkit("mytool"))
	require.NoError(t, err)
	require.NotNil(t, report)
	require.Len(t, report.Results, 3)

	assert.Equal(t, "pass", report.Results[0].Status, "First fast test must pass")
	assert.Equal(t, "fail", report.Results[1].Status, "Slow test must timeout and fail")
	assert.Equal(t, "pass", report.Results[2].Status,
		"Third test must still pass; previous timeout must not cascade")

	assert.Equal(t, 2, report.Passed)
	assert.Equal(t, 1, report.Failed)
}

// ---------------------------------------------------------------------------
// AC-15: Parallel execution
// ---------------------------------------------------------------------------

func TestRunParallel_CorrectResults(t *testing.T) {
	// 3 tests with workers=2 must produce the same results as sequential.
	mock := newMockExecutor()
	mock.setResponse("a", mockResponse{
		result: &runner.Result{ExitCode: 0, Stdout: []byte("A"), Duration: 10 * time.Millisecond},
		delay:  20 * time.Millisecond,
	})
	mock.setResponse("b", mockResponse{
		result: &runner.Result{ExitCode: 1, Stderr: []byte("err-B"), Duration: 10 * time.Millisecond},
		delay:  20 * time.Millisecond,
	})
	mock.setResponse("c", mockResponse{
		result: &runner.Result{ExitCode: 0, Stdout: []byte("C"), Duration: 10 * time.Millisecond},
		delay:  20 * time.Millisecond,
	})

	tr := &TestRunner{Executor: mock}

	suite := TestSuite{
		Tool: "mytool",
		Tests: []TestCase{
			{Name: "test-a", Args: []string{"a"}, Expect: Expectation{ExitCode: intPtr(0)}},
			{Name: "test-b", Args: []string{"b"}, Expect: Expectation{ExitCode: intPtr(0)}}, // fails: exit 1 != 0
			{Name: "test-c", Args: []string{"c"}, Expect: Expectation{ExitCode: intPtr(0)}},
		},
	}

	report, err := tr.RunParallel(context.Background(), suite, sampleToolkit("mytool"), 2)
	require.NoError(t, err)
	require.NotNil(t, report)

	assert.Equal(t, "mytool", report.Tool)
	assert.Equal(t, 3, report.Total)
	assert.Equal(t, 2, report.Passed)
	assert.Equal(t, 1, report.Failed)
	require.Len(t, report.Results, 3)

	// test-b should fail.
	assert.Equal(t, "pass", report.Results[0].Status)
	assert.Equal(t, "fail", report.Results[1].Status)
	assert.Equal(t, "pass", report.Results[2].Status)
}

func TestRunParallel_ResultsInOriginalOrder(t *testing.T) {
	// Despite parallel execution, results must maintain the original test
	// order from the suite. We add varying delays to make execution order
	// differ from definition order.
	mock := newMockExecutor()
	// Third finishes first, first finishes second, second finishes last.
	mock.setResponse("slow", mockResponse{
		result: &runner.Result{ExitCode: 0, Duration: 50 * time.Millisecond},
		delay:  80 * time.Millisecond,
	})
	mock.setResponse("slowest", mockResponse{
		result: &runner.Result{ExitCode: 0, Duration: 100 * time.Millisecond},
		delay:  120 * time.Millisecond,
	})
	mock.setResponse("fast", mockResponse{
		result: &runner.Result{ExitCode: 0, Duration: 5 * time.Millisecond},
		delay:  10 * time.Millisecond,
	})

	tr := &TestRunner{Executor: mock}

	suite := TestSuite{
		Tool: "mytool",
		Tests: []TestCase{
			{Name: "first-slow", Args: []string{"slow"}, Expect: Expectation{ExitCode: intPtr(0)}},
			{Name: "second-slowest", Args: []string{"slowest"}, Expect: Expectation{ExitCode: intPtr(0)}},
			{Name: "third-fast", Args: []string{"fast"}, Expect: Expectation{ExitCode: intPtr(0)}},
		},
	}

	report, err := tr.RunParallel(context.Background(), suite, sampleToolkit("mytool"), 3)
	require.NoError(t, err)
	require.Len(t, report.Results, 3)

	// Order must match definition order, not completion order.
	assert.Equal(t, "first-slow", report.Results[0].Name,
		"Results[0] must be 'first-slow' regardless of completion order")
	assert.Equal(t, "second-slowest", report.Results[1].Name,
		"Results[1] must be 'second-slowest' regardless of completion order")
	assert.Equal(t, "third-fast", report.Results[2].Name,
		"Results[2] must be 'third-fast' regardless of completion order")
}

func TestRunParallel_LimitsConcurrency(t *testing.T) {
	// 4 tests with workers=2. Each test delays 100ms. Peak concurrency
	// must not exceed 2.
	mock := newMockExecutor()
	for _, id := range []string{"w1", "w2", "w3", "w4"} {
		mock.setResponse(id, mockResponse{
			result: &runner.Result{ExitCode: 0, Duration: 100 * time.Millisecond},
			delay:  100 * time.Millisecond,
		})
	}

	tr := &TestRunner{Executor: mock}

	suite := TestSuite{
		Tool: "mytool",
		Tests: []TestCase{
			{Name: "t1", Args: []string{"w1"}, Expect: Expectation{ExitCode: intPtr(0)}},
			{Name: "t2", Args: []string{"w2"}, Expect: Expectation{ExitCode: intPtr(0)}},
			{Name: "t3", Args: []string{"w3"}, Expect: Expectation{ExitCode: intPtr(0)}},
			{Name: "t4", Args: []string{"w4"}, Expect: Expectation{ExitCode: intPtr(0)}},
		},
	}

	report, err := tr.RunParallel(context.Background(), suite, sampleToolkit("mytool"), 2)
	require.NoError(t, err)
	require.NotNil(t, report)
	assert.Equal(t, 4, report.Total)
	assert.Equal(t, 4, report.Passed)

	maxConc := mock.getMaxConcurrent()
	assert.LessOrEqual(t, maxConc, int32(2),
		"Peak concurrency must not exceed workers=2, got %d", maxConc)
	assert.GreaterOrEqual(t, maxConc, int32(2),
		"With 4 tests and workers=2, peak concurrency should reach 2, got %d", maxConc)
}

func TestRunParallel_SingleWorker_Sequential(t *testing.T) {
	// workers=1 must behave like sequential execution: max concurrency = 1.
	mock := newMockExecutor()
	for _, id := range []string{"s1", "s2", "s3"} {
		mock.setResponse(id, mockResponse{
			result: &runner.Result{ExitCode: 0, Duration: 5 * time.Millisecond},
			delay:  30 * time.Millisecond,
		})
	}

	tr := &TestRunner{Executor: mock}

	suite := TestSuite{
		Tool: "mytool",
		Tests: []TestCase{
			{Name: "seq1", Args: []string{"s1"}, Expect: Expectation{ExitCode: intPtr(0)}},
			{Name: "seq2", Args: []string{"s2"}, Expect: Expectation{ExitCode: intPtr(0)}},
			{Name: "seq3", Args: []string{"s3"}, Expect: Expectation{ExitCode: intPtr(0)}},
		},
	}

	report, err := tr.RunParallel(context.Background(), suite, sampleToolkit("mytool"), 1)
	require.NoError(t, err)
	assert.Equal(t, 3, report.Passed)

	maxConc := mock.getMaxConcurrent()
	assert.Equal(t, int32(1), maxConc,
		"workers=1 must limit concurrency to 1")
}

func TestRunParallel_ToolNotFound(t *testing.T) {
	mock := newMockExecutor()
	tr := &TestRunner{Executor: mock}

	suite := TestSuite{
		Tool: "missing",
		Tests: []TestCase{
			{Name: "t1", Args: []string{"x"}},
		},
	}

	report, err := tr.RunParallel(context.Background(), suite, sampleToolkit("other"), 2)
	require.Error(t, err, "RunParallel must error when tool not found")
	assert.Nil(t, report)
}

func TestRunParallel_EmptySuite(t *testing.T) {
	mock := newMockExecutor()
	tr := &TestRunner{Executor: mock}

	suite := TestSuite{Tool: "mytool", Tests: []TestCase{}}
	report, err := tr.RunParallel(context.Background(), suite, sampleToolkit("mytool"), 4)
	require.NoError(t, err)
	require.NotNil(t, report)
	assert.Equal(t, 0, report.Total)
	assert.Empty(t, report.Results)
}

func TestRunParallel_Timeout(t *testing.T) {
	// Parallel execution must still respect per-test timeouts.
	mock := newMockExecutor()
	mock.setResponse("pfast", mockResponse{
		result: &runner.Result{ExitCode: 0, Duration: 5 * time.Millisecond},
		delay:  5 * time.Millisecond,
	})
	mock.setResponse("pslow", mockResponse{
		result: &runner.Result{ExitCode: 0, Duration: 5 * time.Second},
		delay:  5 * time.Second,
	})

	tr := &TestRunner{Executor: mock}

	suite := TestSuite{
		Tool: "mytool",
		Tests: []TestCase{
			{
				Name:    "fast parallel",
				Args:    []string{"pfast"},
				Timeout: 1 * time.Second,
				Expect:  Expectation{ExitCode: intPtr(0)},
			},
			{
				Name:    "slow timeout",
				Args:    []string{"pslow"},
				Timeout: 50 * time.Millisecond,
				Expect:  Expectation{ExitCode: intPtr(0)},
			},
		},
	}

	report, err := tr.RunParallel(context.Background(), suite, sampleToolkit("mytool"), 2)
	require.NoError(t, err)
	require.Len(t, report.Results, 2)

	assert.Equal(t, "pass", report.Results[0].Status, "Fast test must pass in parallel")
	assert.Equal(t, "fail", report.Results[1].Status, "Timed out test must fail in parallel")
	assert.Equal(t, 1, report.Passed)
	assert.Equal(t, 1, report.Failed)
}

// ---------------------------------------------------------------------------
// Context cancellation at suite level
// ---------------------------------------------------------------------------

func TestRun_ContextCancellation(t *testing.T) {
	// If the parent context is cancelled, the runner should stop.
	mock := newMockExecutor()
	mock.defaultResult = &runner.Result{ExitCode: 0, Duration: time.Second}
	// Each test delays 500ms; we cancel after 100ms.
	mock.setResponse("x", mockResponse{
		result: &runner.Result{ExitCode: 0},
		delay:  500 * time.Millisecond,
	})

	tr := &TestRunner{Executor: mock}

	suite := TestSuite{
		Tool: "mytool",
		Tests: []TestCase{
			{Name: "t1", Args: []string{"x"}, Expect: Expectation{ExitCode: intPtr(0)}},
			{Name: "t2", Args: []string{"x"}, Expect: Expectation{ExitCode: intPtr(0)}},
			{Name: "t3", Args: []string{"x"}, Expect: Expectation{ExitCode: intPtr(0)}},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, _ = tr.Run(ctx, suite, sampleToolkit("mytool"))
	elapsed := time.Since(start)

	// Should not have taken 1.5s (3 tests * 500ms each).
	assert.Less(t, elapsed, 1*time.Second,
		"Context cancellation must stop the runner; elapsed=%v", elapsed)
}

// ---------------------------------------------------------------------------
// All-pass and all-fail sanity checks
// ---------------------------------------------------------------------------

func TestRun_AllPass(t *testing.T) {
	mock := newMockExecutor()
	tr := &TestRunner{Executor: mock}

	suite := TestSuite{
		Tool: "mytool",
		Tests: []TestCase{
			{Name: "a", Args: []string{"x"}, Expect: Expectation{ExitCode: intPtr(0)}},
			{Name: "b", Args: []string{"x"}, Expect: Expectation{ExitCode: intPtr(0)}},
			{Name: "c", Args: []string{"x"}, Expect: Expectation{ExitCode: intPtr(0)}},
		},
	}

	report, err := tr.Run(context.Background(), suite, sampleToolkit("mytool"))
	require.NoError(t, err)
	require.NotNil(t, report)

	assert.Equal(t, 3, report.Total)
	assert.Equal(t, 3, report.Passed, "All 3 tests must pass")
	assert.Equal(t, 0, report.Failed, "No tests must fail")

	for _, r := range report.Results {
		assert.Equal(t, "pass", r.Status,
			"Every result must have status 'pass'")
		assert.Empty(t, r.Error,
			"Passing tests must have empty error")
	}
}

func TestRun_AllFail(t *testing.T) {
	mock := newMockExecutor()
	mock.defaultResult = &runner.Result{ExitCode: 99, Duration: 5 * time.Millisecond}
	tr := &TestRunner{Executor: mock}

	suite := TestSuite{
		Tool: "mytool",
		Tests: []TestCase{
			{Name: "a", Args: []string{"x"}, Expect: Expectation{ExitCode: intPtr(0)}},
			{Name: "b", Args: []string{"x"}, Expect: Expectation{ExitCode: intPtr(0)}},
		},
	}

	report, err := tr.Run(context.Background(), suite, sampleToolkit("mytool"))
	require.NoError(t, err)

	assert.Equal(t, 2, report.Total)
	assert.Equal(t, 0, report.Passed, "No tests must pass")
	assert.Equal(t, 2, report.Failed, "All tests must fail")

	for _, r := range report.Results {
		assert.Equal(t, "fail", r.Status)
		assert.NotEmpty(t, r.Error,
			"Failing tests must have a non-empty error")
	}
}

// ---------------------------------------------------------------------------
// No exit_code expectation: test with no assertions passes
// ---------------------------------------------------------------------------

func TestRun_NoExpectation_Passes(t *testing.T) {
	// A test with empty Expectation (no assertions at all) should pass
	// as long as the executor does not error.
	mock := newMockExecutor()
	mock.defaultResult = &runner.Result{ExitCode: 42, Duration: 5 * time.Millisecond}
	tr := &TestRunner{Executor: mock}

	suite := TestSuite{
		Tool: "mytool",
		Tests: []TestCase{
			{
				Name:   "no expectations",
				Args:   []string{"x"},
				Expect: Expectation{}, // all nil/empty
			},
		},
	}

	report, err := tr.Run(context.Background(), suite, sampleToolkit("mytool"))
	require.NoError(t, err)
	require.Len(t, report.Results, 1)

	assert.Equal(t, "pass", report.Results[0].Status,
		"Test with no assertions must pass (exit code 42 is irrelevant without an exit_code expectation)")
}

// ---------------------------------------------------------------------------
// SchemaFS field: nil is acceptable (no schema validation)
// ---------------------------------------------------------------------------

func TestRun_NilSchemaFS_StillWorks(t *testing.T) {
	mock := newMockExecutor()
	tr := &TestRunner{Executor: mock, SchemaFS: nil}

	suite := TestSuite{
		Tool: "mytool",
		Tests: []TestCase{
			{
				Name:   "no schema",
				Args:   []string{"x"},
				Expect: Expectation{ExitCode: intPtr(0)},
			},
		},
	}

	report, err := tr.Run(context.Background(), suite, sampleToolkit("mytool"))
	require.NoError(t, err)
	require.Len(t, report.Results, 1)
	assert.Equal(t, "pass", report.Results[0].Status)
}

// ---------------------------------------------------------------------------
// Sequential vs parallel produce same report
// ---------------------------------------------------------------------------

func TestRunParallel_MatchesSequentialResults(t *testing.T) {
	// Run the same suite both sequentially and in parallel.
	// The reports must match (same pass/fail, same order, same names).
	buildMock := func() *mockExecutor {
		m := newMockExecutor()
		m.setResponse("q1", mockResponse{
			result: &runner.Result{ExitCode: 0, Stdout: []byte("Q1"), Duration: 5 * time.Millisecond},
			delay:  10 * time.Millisecond,
		})
		m.setResponse("q2", mockResponse{
			result: &runner.Result{ExitCode: 1, Stderr: []byte("Q2 err"), Duration: 5 * time.Millisecond},
			delay:  10 * time.Millisecond,
		})
		m.setResponse("q3", mockResponse{
			result: &runner.Result{ExitCode: 0, Stdout: []byte("Q3"), Duration: 5 * time.Millisecond},
			delay:  10 * time.Millisecond,
		})
		return m
	}

	suite := TestSuite{
		Tool: "mytool",
		Tests: []TestCase{
			{Name: "r1", Args: []string{"q1"}, Expect: Expectation{ExitCode: intPtr(0)}},
			{Name: "r2", Args: []string{"q2"}, Expect: Expectation{ExitCode: intPtr(0)}}, // fails
			{Name: "r3", Args: []string{"q3"}, Expect: Expectation{ExitCode: intPtr(0)}},
		},
	}
	tk := sampleToolkit("mytool")

	// Sequential.
	seqMock := buildMock()
	seqRunner := &TestRunner{Executor: seqMock}
	seqReport, err := seqRunner.Run(context.Background(), suite, tk)
	require.NoError(t, err)

	// Parallel.
	parMock := buildMock()
	parRunner := &TestRunner{Executor: parMock}
	parReport, err := parRunner.RunParallel(context.Background(), suite, tk, 2)
	require.NoError(t, err)

	// Compare logical results (not timing).
	assert.Equal(t, seqReport.Tool, parReport.Tool)
	assert.Equal(t, seqReport.Total, parReport.Total)
	assert.Equal(t, seqReport.Passed, parReport.Passed)
	assert.Equal(t, seqReport.Failed, parReport.Failed)
	require.Len(t, parReport.Results, len(seqReport.Results))

	for i := range seqReport.Results {
		assert.Equal(t, seqReport.Results[i].Name, parReport.Results[i].Name,
			"Result[%d] name must match between sequential and parallel", i)
		assert.Equal(t, seqReport.Results[i].Status, parReport.Results[i].Status,
			"Result[%d] status must match between sequential and parallel", i)
	}
}

// ---------------------------------------------------------------------------
// Worker count edge cases
// ---------------------------------------------------------------------------

func TestRunParallel_WorkersExceedTests(t *testing.T) {
	// Workers > number of tests must still work correctly.
	mock := newMockExecutor()
	tr := &TestRunner{Executor: mock}

	suite := TestSuite{
		Tool: "mytool",
		Tests: []TestCase{
			{Name: "only-one", Args: []string{"x"}, Expect: Expectation{ExitCode: intPtr(0)}},
		},
	}

	report, err := tr.RunParallel(context.Background(), suite, sampleToolkit("mytool"), 10)
	require.NoError(t, err)
	assert.Equal(t, 1, report.Total)
	assert.Equal(t, 1, report.Passed)
}

// ---------------------------------------------------------------------------
// Multiple executor calls: verify call count and per-test isolation
// ---------------------------------------------------------------------------

func TestRun_ExecutorCalledOncePerTest(t *testing.T) {
	mock := newMockExecutor()
	tr := &TestRunner{Executor: mock}

	suite := TestSuite{
		Tool: "mytool",
		Tests: []TestCase{
			{Name: "t1", Args: []string{"x"}, Expect: Expectation{ExitCode: intPtr(0)}},
			{Name: "t2", Args: []string{"y"}, Expect: Expectation{ExitCode: intPtr(0)}},
			{Name: "t3", Args: []string{"z"}, Expect: Expectation{ExitCode: intPtr(0)}},
		},
	}

	_, err := tr.Run(context.Background(), suite, sampleToolkit("mytool"))
	require.NoError(t, err)

	calls := mock.getCalls()
	assert.Len(t, calls, 3, "Executor must be called exactly once per test case")
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// containsAny returns true if s contains any of the substrings.
func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if len(sub) > 0 && len(s) >= len(sub) {
			for i := 0; i <= len(s)-len(sub); i++ {
				if s[i:i+len(sub)] == sub {
					return true
				}
			}
		}
	}
	return false
}
