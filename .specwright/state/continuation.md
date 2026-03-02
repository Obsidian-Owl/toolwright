# Continuation: toolwright-core / auth

## Status
All 7 tasks complete. Ready for /sw-verify.

## Completed tasks
1. Token types (StoredToken, TokenFile, IsExpired)
2. Keyring store (KeyringStore with Keyring interface, JSON serialization)
3. Fallback file store (FileStore with XDG, 0600 permissions, read-modify-write)
4. Token resolver (flag → env → keyring → file store → error chain)
5. OAuth discovery (RFC 8414 → OIDC → manual endpoints)
6. OAuth PKCE login (verifier, state, callback server, token exchange, storage)
7. Token refresh (oauth2.TokenSource, re-login error on failure)

## Key files modified
- `internal/auth/types.go` — StoredToken, TokenFile, IsExpired
- `internal/auth/keyring.go` — KeyringStore wrapping Keyring interface
- `internal/auth/store.go` — FileStore with XDG and permission enforcement
- `internal/auth/resolver.go` — Resolver with TokenStore interface
- `internal/auth/oauth.go` — DiscoverEndpoints, Login (PKCE), Refresh
- `internal/auth/*_test.go` — 216 tests

## Branch
`feat/auth` — 7 commits ahead of `main`

## Next
Run /sw-verify to check quality gates.
