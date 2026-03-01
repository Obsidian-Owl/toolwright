# Plan: Unit 4 — testing

## Task Breakdown

### Task 1: Test types
- File: `internal/testing/types.go`
- `TestSuite` struct: `Tool string`, `Tests []TestCase`
- `TestCase` struct: `Name`, `Args`, `Flags`, `AuthToken`, `Expect`, `Timeout`
- `Expectation` struct: `ExitCode *int`, `StdoutIsJSON *bool`, `StdoutSchema string`, `StdoutContains []Assertion`, `StderrContains []string`
- `Assertion` struct: `Path string`, `Equals any`, `Contains any`, `Matches string`, `Exists *bool`, `Length *int`
- `TestResult` struct: `Name`, `Status` (pass/fail), `Duration`, `Error`, `Stdout`, `Stderr`
- `TestReport` struct: `Tool`, `Total`, `Passed`, `Failed`, `Results []TestResult`

### Task 2: Test YAML parser
- File: `internal/testing/parser.go`
- `ParseTestFile(path string) (*TestSuite, error)`
- `ParseTestDir(dir string) ([]TestSuite, error)` — glob `*.test.yaml`
- Env var expansion for `auth_token` field (`${VAR}` syntax)

### Task 3: Assertion engine
- File: `internal/testing/assertions.go`
- `EvaluateAssertions(stdout []byte, expect Expectation, schemaValidator schema.Validator) []AssertionError`
- JSONPath evaluation via `ojg/jp`
- Each operator: `equals`, `contains`, `matches`, `exists`, `length`
- `exit_code` check
- `stdout_is_json` check
- `stdout_schema` check (delegates to schema.Validator)
- `stderr_contains` check (substring match)

### Task 4: Test runner
- File: `internal/testing/runner.go`
- `TestRunner` struct with `runner.Executor` and `schema.Validator`
- `Run(ctx context.Context, suite TestSuite, manifest manifest.Toolkit) (*TestReport, error)`
- `RunParallel(ctx context.Context, suite TestSuite, manifest manifest.Toolkit, workers int) (*TestReport, error)`
- Auth token resolution: test-level `auth_token` → `TOOLWRIGHT_TEST_TOKEN` env var

### Task 5: Output formatters
- File: `internal/testing/output.go`
- `FormatTAP(report TestReport, w io.Writer) error`
- `FormatJSON(report TestReport, w io.Writer) error`
- TAP version 13 format
- JSON matches §3.4 schema

## File Change Map

| File | Action | Package |
|------|--------|---------|
| `internal/testing/types.go` | Create | testing |
| `internal/testing/parser.go` | Create | testing |
| `internal/testing/parser_test.go` | Create | testing |
| `internal/testing/assertions.go` | Create | testing |
| `internal/testing/assertions_test.go` | Create | testing |
| `internal/testing/runner.go` | Create | testing |
| `internal/testing/runner_test.go` | Create | testing |
| `internal/testing/output.go` | Create | testing |
| `internal/testing/output_test.go` | Create | testing |
| `go.mod` | Update | root (add ojg) |
