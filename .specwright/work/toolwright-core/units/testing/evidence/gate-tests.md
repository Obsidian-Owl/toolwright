# Gate: Tests
**Status**: PASS
**Timestamp**: 2026-03-03T16:36:00Z

## Results
- `go test ./... -count=1 -timeout 120s` — all packages pass
- `go test -race ./internal/tooltest/... -count=1` — race detector clean (1.981s)
- `go vet ./internal/tooltest/...` — clean

## Package Results
| Package | Status | Duration |
|---------|--------|----------|
| internal/auth | PASS | 0.916s |
| internal/manifest | PASS | 0.025s |
| internal/runner | PASS | 4.027s |
| internal/schema | PASS | 0.014s |
| internal/tooltest | PASS | 0.906s |

## Test Counts (tooltest package)
- types_test.go: 45 tests
- parser_test.go: 29 tests
- assertions_test.go: 80+ tests
- runner_test.go: 30+ tests
- output_test.go: 35+ tests
- Total: 200+ tests, 0 failures

## Findings
None.
