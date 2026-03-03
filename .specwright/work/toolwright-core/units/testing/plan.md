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

## As-Built Notes

### Package naming
Plan specified `internal/testing/` but this collides with Go's stdlib `testing` package. All files are in `internal/tooltest/` with package name `tooltest` instead.

### Actual file paths
| File | Package |
|------|---------|
| `internal/tooltest/types.go` | tooltest |
| `internal/tooltest/types_test.go` | tooltest |
| `internal/tooltest/parser.go` | tooltest |
| `internal/tooltest/parser_test.go` | tooltest |
| `internal/tooltest/assertions.go` | tooltest |
| `internal/tooltest/assertions_test.go` | tooltest |
| `internal/tooltest/runner.go` | tooltest |
| `internal/tooltest/runner_test.go` | tooltest |
| `internal/tooltest/output.go` | tooltest |
| `internal/tooltest/output_test.go` | tooltest |

### API deviations from plan
- `EvaluateAssertions` signature: `func EvaluateAssertions(stdout []byte, stderr []byte, exitCode int, expect Expectation, schemaFS fs.FS) []AssertionResult` — takes `fs.FS` directly instead of `schema.Validator` for simpler API. Creates validator internally.
- `TestRunner.Executor` uses `ToolExecutor` interface (not `runner.Executor` directly) for testability. `runner.Executor` satisfies this interface.
- `TestRunner.SchemaFS` is `fs.FS` (not `schema.Validator`) — constitution rule 17a.
- Auth token env var expansion happens in parser (Task 2), not runner (Task 4) — cleaner separation.

### Test counts
| File | Tests |
|------|-------|
| types_test.go | 45 |
| parser_test.go | 29 |
| assertions_test.go | 80+ |
| runner_test.go | 30+ |
| output_test.go | 35+ |
| **Total** | **200+** |

### Post-build review: APPROVED (no BLOCKs, no WARNs)
