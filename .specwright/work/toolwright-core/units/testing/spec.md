# Spec: Unit 4 — testing

## Acceptance Criteria

### AC-1: Test YAML parses into TestSuite
- Parse example from spec §3.4 → correct tool name, test names, args, flags, expectations
- All assertion operators parsed: equals, contains, matches, exists, length

### AC-2: Auth token supports env var expansion
- `auth_token: "${MY_TOKEN}"` with `MY_TOKEN=secret` → resolves to `"secret"`
- `auth_token: "${UNSET_VAR}"` → empty string (not literal `${UNSET_VAR}`)
- No `auth_token` field, `TOOLWRIGHT_TEST_TOKEN=fallback` → uses `"fallback"`

### AC-3: Test directory globbing
- Directory with `foo.test.yaml` and `bar.test.yaml` → both parsed
- Directory with `README.md` and `notes.txt` → neither parsed
- Empty directory → empty suite list (not error)

### AC-4: Assertion — equals
- JSONPath `$.status` equals `"ok"` against `{"status": "ok"}` → pass
- JSONPath `$.status` equals `"ok"` against `{"status": "fail"}` → fail with expected vs actual
- JSONPath `$.count` equals `3` against `{"count": 3}` → pass (numeric)

### AC-5: Assertion — contains
- JSONPath `$.tags` contains `"urgent"` against `{"tags": ["urgent", "low"]}` → pass (array element)
- JSONPath `$.message` contains `"error"` against `{"message": "an error occurred"}` → pass (substring)
- JSONPath `$.tags` contains `"missing"` against `{"tags": ["urgent"]}` → fail

### AC-6: Assertion — matches
- JSONPath `$.version` matches `"^\\d+\\.\\d+\\.\\d+$"` against `{"version": "1.2.3"}` → pass
- Same regex against `{"version": "latest"}` → fail
- Invalid regex → error (not panic)

### AC-7: Assertion — exists
- JSONPath `$.field` exists `true` against `{"field": "value"}` → pass
- JSONPath `$.field` exists `true` against `{}` → fail
- JSONPath `$.field` exists `false` against `{}` → pass

### AC-8: Assertion — length
- JSONPath `$.items` length `3` against `{"items": [1,2,3]}` → pass (array)
- JSONPath `$.name` length `5` against `{"name": "hello"}` → pass (string)
- JSONPath `$.items` length `2` against `{"items": [1,2,3]}` → fail with expected vs actual

### AC-9: Exit code assertion
- Test expects `exit_code: 0`, tool exits 0 → pass
- Test expects `exit_code: 2`, tool exits 1 → fail with expected vs actual

### AC-10: stdout_is_json assertion
- Tool outputs valid JSON → pass
- Tool outputs plain text → fail

### AC-11: stdout_schema validation
- Tool output matches schema → pass
- Tool output missing required field per schema → fail with schema error details

### AC-12: stderr_contains assertion
- `stderr_contains: ["path does not exist"]` and stderr includes that string → pass
- Multiple strings: all must be present (AND semantics)
- String not found → fail

### AC-13: TAP output format
- 2 tests (1 pass, 1 fail) → TAP version 13 header, `1..2`, `ok 1`, `not ok 2` with YAML diagnostics
- Failed test diagnostics include expected/actual values

### AC-14: JSON output format
- Matches schema from spec §3.4: `tool`, `total`, `passed`, `failed`, `results[]` with `name`, `status`, `duration_ms`
- Failed results include `error`, `stdout`, `stderr` fields

### AC-15: Parallel execution
- 3 tests with `--parallel 2` → at most 2 tests running concurrently
- Results collected correctly regardless of execution order
- No data races (`go test -race` clean)

### AC-16: Per-test timeout
- Test with `timeout: 1s` and tool that sleeps 10s → test fails with timeout error
- Test without timeout → uses default (30s from runner)

### AC-17: JSONPath with array index and nested paths
- `$.findings[0].type` resolves correctly on nested array data
- `$.findings[*].type` resolves to array of all type values
- `$.deeply.nested.path` resolves multi-level nesting

### AC-18: Build and tests pass
- `go build ./...` succeeds
- `go test ./internal/testing/...` passes
- `go test -race ./internal/testing/...` passes
- `go vet ./...` clean
