# Gate: Security

**Verdict:** PASS
**Timestamp:** 2026-03-02T14:05:00Z
**Unit:** auth

## Scan Scope

Files analyzed: types.go, keyring.go, store.go, resolver.go, oauth.go + all test files

## Findings

| # | Severity | Finding |
|---|----------|---------|
| 1 | WARN | Callback server fallback `":0"` binds to 0.0.0.0 instead of 127.0.0.1 (oauth.go:66). Mitigated by 256-bit state entropy. Fix: change to `"127.0.0.1:0"`. |
| 2 | WARN | No body size limit on discovery doc fetch (oauth.go:317). Could cause OOM with malicious provider. Fix: use `io.LimitReader`. |
| 3 | WARN | No HTTPS enforcement on providerURL — manifest validation layer should enforce this. |
| 4 | WARN | TOCTOU between permission check and file read in FileStore.Get (store.go:107-111). Negligible practical impact — attacker with that access already has file access. |
| 5 | WARN | basePath not sanitized in FileStore — low risk since caller is trusted code. |
| 6 | INFO | Test tokens are clearly synthetic (fake JWTs, "test-token" strings). No real credentials. |

## Checks Passed

- No hardcoded secrets or API keys
- Tokens never appear in error messages (Constitution rule 23) — verified by tests
- PKCE uses crypto/rand via oauth2.GenerateVerifier
- State parameter uses crypto/rand (32 bytes, 256-bit entropy)
- No os/exec usage (no command injection)
- No logging/printing of token values

## Summary

- 0 BLOCK
- 5 WARN
- 1 INFO
