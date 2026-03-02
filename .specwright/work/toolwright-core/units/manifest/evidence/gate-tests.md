# Gate: Tests

**Verdict:** PASS
**Timestamp:** 2026-03-02T13:15:00Z
**Unit:** manifest

## Test Results

| Package | Status | Tests (incl. subtests) | Duration |
|---------|--------|------------------------|----------|
| `internal/manifest` | PASS | 200+ | ~37s |
| `internal/schema` | PASS | 48+ | ~0.02s |
| **Total** | **PASS** | **248** | ~37s |

## Test Coverage Areas

### Manifest Parser (`parser_test.go`)
- Full manifest parsing with all fields
- Auth string shorthand and full object form
- Endpoints parsing
- Malformed input handling (9 cases)
- Round-trip marshal/unmarshal
- Adversarial fuzzing (11 inputs, no panics)
- Error wrapping consistency

### Manifest Validation (`validate_test.go`)
- Name format validation (21 cases)
- SemVer validation (20 cases)
- Description boundary values (empty, 1, 199, 200, 201 chars)
- Duplicate tool/arg/flag name detection
- Auth type validation (token, oauth2, none, unknown)
- Flag type/default mismatch detection (13 cases)
- Non-fail-fast behavior verification
- ValidationError field structure
- Bracket-notation path format

### Schema Validator (`validator_test.go`)
- Valid JSON with correct/extra fields
- Missing required fields (top-level and nested)
- Wrong type detection (8 cases)
- Empty/nil/whitespace input handling
- Non-object JSON rejection
- Malformed JSON handling (6 cases)
- Multiple schemas in same validator
- Boundary values (zero, negative, unicode, floats)
- Never-panics robustness (6 adversarial inputs)

## Findings

None. All tests pass.

## Summary

- 0 BLOCK
- 0 WARN
- 0 INFO
