# Gate: Wiring

**Verdict:** PASS
**Timestamp:** 2026-03-02T14:05:00Z
**Unit:** auth

## Structural Analysis

### Architecture layers
- `internal/auth` depends on `internal/manifest` (correct per design)
- `internal/auth` depends on `golang.org/x/oauth2` (correct)
- No circular dependencies
- No imports from downstream packages (runner, cli, codegen, testing)

### Interface compliance
- `*KeyringStore` implements `TokenStore` and `WritableTokenStore` — compile-time verified
- `*FileStore` implements `TokenStore` and `WritableTokenStore` — compile-time verified

### Plan deviations
All documented in as-built notes: resolver/login/refresh signatures changed for testability.

## Findings

| # | Severity | Finding |
|---|----------|---------|
| 1 | INFO | Exports not yet consumed outside tests — expected for Unit 2 in bottom-up build |
| 2 | INFO | go-keyring not in go.mod — Keyring interface decouples; wired at CLI layer |
| 3 | WARN | Three near-identical `keysOf`/`jsonKeys`/`rawKeys` test helpers across test files |
| 4 | WARN | `fetchDiscoveryDoc` uses `http.DefaultClient` — doesn't respect custom HTTPClient |
| 5 | WARN | `LoginConfig.ListenAddr` default not enforced in `Login()` |

## Summary

- 0 BLOCK
- 3 WARN
- 2 INFO
