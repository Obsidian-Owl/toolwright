# Gate: Tests

**Verdict:** PASS
**Timestamp:** 2026-03-02T14:05:00Z
**Unit:** auth

## Test Results

| Package | Status | Duration |
|---------|--------|----------|
| `internal/auth` | PASS | ~0.9s |
| `internal/manifest` | PASS | ~42s |
| `internal/schema` | PASS | ~0.01s |

**Total: 216 auth tests + 248 manifest/schema tests = 464+ assertions, 0 failures.**

## Test Coverage Areas

- **types_test.go**: 33 tests — StoredToken/TokenFile JSON round-trip, IsExpired edge cases
- **keyring_test.go**: 25 tests — KeyringStore with fake keyring, service name, error propagation
- **store_test.go**: 39 tests — FileStore XDG, permissions (9 insecure modes), JSON format, multi-key
- **resolver_test.go**: 42 tests — Full chain priority (12 table-driven), token leak prevention
- **oauth_test.go**: 77 tests — Discovery (31), Login PKCE (28), Refresh (18)

## Summary

- 0 BLOCK
- 0 WARN
- 0 INFO
