# Context: Unit 2 — auth

## Purpose

Implement the authentication subsystem: token resolution chain, platform keyring storage, fallback file-based storage, and OAuth 2.1 Authorization Code + PKCE flow with discovery.

## Key Spec Sections

- §2.4: Auth configuration (none, token, oauth2)
- §2.5: Token resolution order (flag → env → keyring → error)
- §2.6: Token storage (keyring primary, file fallback, XDG compliance)
- §3.7: Login flow (discovery, PKCE, callback server, error handling, manual endpoints)
- §4.3: Auth resolution pipeline

## Files to Create

```
internal/auth/
├── resolver.go          # Token resolution chain
├── resolver_test.go
├── keyring.go           # Platform keyring via go-keyring
├── keyring_test.go
├── store.go             # Fallback file-based token storage
├── store_test.go
├── oauth.go             # PKCE flow, callback server, discovery
└── oauth_test.go
```

## Dependencies

- `internal/manifest` (Unit 1) — imports `Auth`, `Endpoints` types
- `github.com/zalando/go-keyring` — platform keyring access
- `golang.org/x/oauth2` — OAuth 2.1 + PKCE (GenerateVerifier, S256ChallengeOption, VerifierOption)

## Auth Resolution Chain

```
Resolver.Resolve(ctx, authConfig, flagValue)
  1. flagValue non-empty? → return it
  2. os.Getenv(authConfig.TokenEnv) non-empty? → return it
  3. keyring.Get("toolwright", key) succeeds? → check expiry → return it
  4. store.Get(key) succeeds? (fallback) → check expiry → return it
  5. → error with guidance message
```

Key format for keyring/store: `{toolkit-name}/{tool-name}`

## OAuth PKCE Flow

1. Discover endpoints: `{provider_url}/.well-known/oauth-authorization-server` → fallback to `/.well-known/openid-configuration` → fallback to manual `auth.Endpoints`
2. Generate PKCE verifier (43-128 char URL-safe) + challenge (SHA-256, S256)
3. Generate state parameter (32 bytes, base64url)
4. Start callback server on `127.0.0.1:8085` (fallback to port 0)
5. Build auth URL, open browser (or print with --no-browser)
6. Wait for callback (120s timeout), validate state
7. Exchange code for tokens
8. Store in keyring (fallback to file store)

## Gotchas

1. **go-keyring** must work without CGO — verified (D-Bus on Linux, /usr/bin/security on macOS)
2. **Fallback file store** at `$XDG_CONFIG_HOME/toolwright/tokens.json` — 0600 permissions, refuse to read if more permissive
3. **Token expiry** — stored tokens have an `expiry` field; resolver must check before returning
4. **Silent refresh** — if access token expired and refresh token available, attempt refresh before erroring
5. **Tokens never logged** — Constitution rule 23
6. **x/oauth2 PKCE** — use `oauth2.GenerateVerifier()`, `oauth2.S256ChallengeOption(verifier)`, `oauth2.VerifierOption(verifier)`
