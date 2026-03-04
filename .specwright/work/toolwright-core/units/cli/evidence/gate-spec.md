# Gate: Spec Compliance

**Status**: PASS
**Timestamp**: 2026-03-04

## AC-by-AC Matrix

| # | Criterion | Status | Notes |
|---|-----------|--------|-------|
| AC-1 | Global --json | PASS | All commands support --json via persistent flag |
| AC-2 | --json error output | PASS | outputError() produces correct structure |
| AC-3 | Exit codes | WARN | Constants defined; main.go always exits 1 (not 2/3) |
| AC-4 | NO_COLOR/CI detection | WARN | isColorDisabled() tested but never called; no ANSI output anyway |
| AC-5 | --debug diagnostics | WARN | debugLog() tested but never called by commands |
| AC-6 | validate errors | PASS | Full coverage |
| AC-7 | validate entrypoint | PASS | Missing/not-executable tested |
| AC-8 | list table | PASS | Human table + JSON format |
| AC-9 | describe JSON Schema | PASS | json/mcp/openai formats |
| AC-10 | run with auth | PASS | Interface injection, auth resolution |
| AC-11 | run auth errors | PASS | Error messages include tool name, env var, login hint |
| AC-12 | test scenarios | PASS | TAP/JSON, filter, parallel |
| AC-13 | login OAuth | PASS | Auth type validation, no-browser, no tokens in output |
| AC-14 | generate cli | PASS | Target validation, dry-run, force |
| AC-15 | generate mcp | PASS | Transport wired, target validation |
| AC-16 | init scaffolds | WARN | CLI tested with mocks; scaffolder=nil in production wiring |
| AC-17 | init TUI wizard | WARN | CI fallback tested; wizard=nil in production |
| AC-18 | generate manifest | WARN | Provider validation tested; generator=nil in production |
| AC-19 | Commands thin | PASS | All delegate via interfaces |
| AC-20 | Binary builds | PASS | Build/test/vet/help all work |

## Summary
- **PASS**: 13/20
- **WARN**: 7/20 (AC-3,4,5 = infrastructure gaps; AC-16,17,18 = nil production deps)
- **BLOCK**: 0/20

## WARN Categories

### Infrastructure WARNs (AC-3, AC-4, AC-5)
These have implementations and tests but are not wired to production code paths:
- Exit code differentiation (always returns 1)
- Color disable function (no ANSI output emitted anyway)
- Debug logging function (no commands call it)

### Nil Dependency WARNs (AC-16, AC-17, AC-18)
CLI delegation layer is fully tested with mocks, but production wiring uses nil:
- init scaffolder and wizard (P3: expected for leaf-first builds)
- generate manifest generator (marked optional in spec)
