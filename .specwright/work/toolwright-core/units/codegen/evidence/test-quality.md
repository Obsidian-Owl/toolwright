# Test Quality Gate Evidence

**Timestamp**: 2026-03-04
**Work Unit**: codegen

## Verdict: WARN

0 BLOCK, 11 WARN, 5 INFO

## WARN Findings

### W-1: Context cancellation tests accept silent success
- **Location**: engine_test.go:1016-1019, cli_go_test.go:1092-1096, mcp_typescript_test.go:1086-1090
- Tests use `if err != nil` pattern — a no-op implementation would pass
- **Recommendation**: Use `require.Error(t, err)` then `assert.ErrorIs`

### W-2: Empty version test accepts either behavior
- **Location**: engine_test.go:1107-1114
- Dual-path `if err != nil / else` — any behavior passes
- **Recommendation**: Commit to one expected behavior

### W-3: TS string type mapping is tautological
- **Location**: mcp_typescript_test.go:648-649
- `assert.Contains(t, content, "string")` matches incidental occurrences
- **Recommendation**: Assert `z.string()` specifically

### W-4: Go string type mapping is tautological
- **Location**: cli_go_test.go:589
- Same issue as W-3 for Go
- **Recommendation**: Assert `StringVar` or `StringP` pattern

### W-5: Go bool type mapping is weak
- **Location**: cli_go_test.go:591, 498
- `assert.Contains(t, content, "bool")` too broad
- **Recommendation**: Assert `BoolVar` or `BoolP` pattern

### W-6: Default value assertion too weak
- **Location**: cli_go_test.go:538
- `assert.Contains(t, content, "3")` — "3" appears incidentally
- **Recommendation**: Assert more specific pattern or use distinctive default

### W-7: Secret detection has pattern gaps
- **Location**: cli_go_test.go:707-720, mcp_typescript_test.go:781-796
- Pattern list is narrow; "Bearer " has trailing space that misses concatenation
- **Recommendation**: Best-effort acknowledged; add `"Bearer"` without trailing space

### W-8: Missing test for empty tools slice
- **Location**: All test files
- No test passes `Tools: []manifest.Tool{}` to either generator
- **Recommendation**: Add boundary test for zero tools

### W-9: Missing test for nil Auth pointer on tool
- **Location**: All test files
- Partly covered by `manifestInheritedAuth()` tests through `ResolvedAuth()`
- **Recommendation**: Verify coverage through ResolvedAuth path is sufficient

### W-10: Marker file content assertions use loose Contains
- **Location**: engine_test.go:515-521
- `assert.Contains(t, contentStr, "go")` could match incidentally
- **Recommendation**: Assert `"target: go"` instead

### W-11: Package.json Zod dependency not tested in unit tests
- **Location**: mcp_typescript_test.go
- Only integration test checks for Zod in package.json
- **Recommendation**: Add assertion in unit-level TS tests

## INFO Findings

- I-1: `countAuthReferences` duplicated across test files (different term lists)
- I-2: Test manifests duplicated between unit and integration files (different purposes)
- I-3: Some assertion messages could be more precise (acceptable)
- I-4: Integration test uses `assert.NoError` instead of `require.NoError` for structure
- I-5: DryRun+ExistingMarker test is an excellent adversarial test (positive)

## Strengths

- All 16 acceptance criteria covered
- Conditional file generation thoroughly tested
- Clean mock discipline — mocks only for Engine dispatch
- Strong integration tests with `go build`/`go vet`
- Well-designed helpers in testhelpers_test.go
