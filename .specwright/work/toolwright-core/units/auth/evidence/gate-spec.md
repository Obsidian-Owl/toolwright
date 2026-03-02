# Gate: Spec Compliance

**Verdict:** PASS
**Timestamp:** 2026-03-02T14:05:00Z
**Unit:** auth

## Acceptance Criteria Mapping

| AC | Criterion | Implementation | Test Evidence | Status |
|----|-----------|---------------|---------------|--------|
| AC-1 | Resolver returns flag value | resolver.go:32-34 | TestResolver_FlagReturnedAsIs (8 subtests) | PASS |
| AC-2 | Resolver falls back to env | resolver.go:37-41 | TestResolver_EnvVarResolvedByName + 4 more | PASS |
| AC-3 | Resolver falls back to keyring | resolver.go:44-49 | TestResolver_ValidKeyringTokenReturned + 3 more | PASS |
| AC-4 | Resolver falls back to file store | resolver.go:52-57 | TestResolver_ValidFilestoreTokenReturned + 2 more | PASS |
| AC-5 | Actionable error message | resolver.go:60-61 | TestResolver_ErrorMessageExactFormat (3 subtests) | PASS |
| AC-6 | Never logs token values | No logging in auth pkg | TestResolver_ErrorDoesNotContainTokenValues + 2 more | PASS |
| AC-7 | Keyring round-trips tokens | keyring.go:29-59 | TestKeyringStore_RoundTrip + 5 more | PASS |
| AC-8 | File store enforces 0600 | store.go:83,91-100 | TestFileStore_Get_RejectsInsecurePermissions_Table (9 modes) | PASS |
| AC-9 | File store respects XDG | store.go:25-38 | TestFileStore_XDG_CONFIG_HOME + 2 more | PASS |
| AC-10 | Valid JSON with version | store.go:130-138 | TestFileStore_WritesValidJSON_WithVersionField + 3 more | PASS |
| AC-11 | RFC 8414 discovery | oauth.go:263-269 | TestDiscoverEndpoints_RFC8414_Success + 2 more | PASS |
| AC-12 | OIDC fallback | oauth.go:277-279 | TestDiscoverEndpoints_OIDCFallback_RFC8414Returns404 + 2 more | PASS |
| AC-13 | Manual endpoint fallback | oauth.go:287-294 | TestDiscoverEndpoints_ManualFallback_BothDiscoveryFail + 2 more | PASS |
| AC-14 | PKCE flow | oauth.go:54,85,101,151,165 | TestLogin_CodeChallengeMatchesVerifier + 6 more | PASS |
| AC-15 | Callback server errors | oauth.go:101-104,141,119-123 | TestLogin_StateMismatch + Timeout + Shutdown tests | PASS |
| AC-16 | Port selection | oauth.go:64-70,73-74 | TestLogin_DefaultListenAddr_FallbackOnPortConflict + 2 more | PASS |
| AC-17 | Token refresh | oauth.go:186-237 | TestRefresh_ExpiredToken_RefreshSucceeds + 4 more | PASS |
| AC-18 | Build and tests pass | go build/vet/test all exit 0 | 216 tests, 0 failures | PASS |

## Coverage: 18/18 acceptance criteria verified

## Summary

- 0 BLOCK
- 0 WARN
- 0 INFO
