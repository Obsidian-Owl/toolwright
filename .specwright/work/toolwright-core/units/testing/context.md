# Context: Unit 4 — testing

## Purpose

Implement the test framework for `toolwright test`. Parses YAML test scenarios, executes tools via the runner, evaluates assertions (including JSONPath), and outputs results in TAP or JSON format.

## Key Spec Sections

- §3.4: `toolwright test` — YAML format, assertion operators, auth in tests, `--json` output schema
- Test assertions: equals, contains, matches, exists, length

## Files to Create

```
internal/testing/
├── types.go             # TestSuite, TestCase, Expectation types
├── parser.go            # YAML test file parser
├── parser_test.go
├── runner.go            # Test orchestration, parallel execution
├── runner_test.go
├── assertions.go        # JSONPath evaluation + assertion operators
├── assertions_test.go
├── output.go            # TAP and JSON formatters
└── output_test.go
```

## Dependencies

- `internal/manifest` (Unit 1) — `Tool` type for looking up tool definitions
- `internal/runner` (Unit 3) — `Executor` for running tools under test
- `internal/schema` (Unit 1) — `Validator` for `stdout_schema` assertions
- `github.com/ohler55/ojg` — JSONPath evaluation

## Test YAML Format

```yaml
tool: scan
tests:
  - name: finds sql injection
    args: [./fixtures/vulnerable.py]
    flags: { severity: low }
    auth_token: "${DEPLOY_TEST_TOKEN}"    # Optional, supports env var expansion
    expect:
      exit_code: 0
      stdout_is_json: true
      stdout_schema: schemas/scan-output.json
      stdout_contains:
        - path: $.findings[0].type
          equals: sql_injection
        - path: $.findings
          length: 3
    timeout: 10s
```

## Assertion Operators

| Operator | Type | Description |
|----------|------|-------------|
| `equals` | any | Exact match |
| `contains` | string/array | Substring or array element |
| `matches` | string | Regex match |
| `exists` | bool | Path exists (true) or not (false) |
| `length` | int | Array/string length |

## Output Formats

**TAP** (default):
```
TAP version 13
1..3
ok 1 - finds sql injection
not ok 2 - exits 2 on invalid path
  ---
  expected_exit_code: 2
  actual_exit_code: 1
  ...
ok 3 - deploys with auth
```

**JSON** (`--json`):
```json
{
  "tool": "scan",
  "total": 3,
  "passed": 2,
  "failed": 1,
  "results": [...]
}
```

## Gotchas

1. **`auth_token` env var expansion** — `"${DEPLOY_TEST_TOKEN}"` resolves from environment. Also check `TOOLWRIGHT_TEST_TOKEN` as default fallback.
2. **Parallel execution** — `--parallel > 1` runs tests concurrently. Each test gets its own runner invocation (no shared state). Results must be collected thread-safely.
3. **JSONPath via ojg** — use `jp.ParseString()` then `expr.Get()` on parsed JSON data
4. **`stdout_schema` validation** — delegates to `internal/schema.Validator` from Unit 1
5. **`stderr_contains`** — simple substring match on stderr output (string array, not JSONPath)
6. **Timeout per test** — each test case has its own timeout (default from runner, overrideable)
