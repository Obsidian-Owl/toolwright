# Spec: Unit 2 — auth

## Acceptance Criteria

### AC-1: Resolver returns flag value when provided
- `Resolve(ctx, auth, "explicit-token")` returns `"explicit-token"` regardless of env/keyring state
- Works for all auth types (none, token, oauth2)

### AC-2: Resolver falls back to environment variable
- Flag value is empty, `TOKEN_ENV` env var set → returns env var value
- Flag value is empty, env var empty → does not return env var

### AC-3: Resolver falls back to keyring
- Flag empty, env empty, token in keyring (not expired) → returns keyring token
- Flag empty, env empty, token in keyring (expired, no refresh token) → falls through to error

### AC-4: Resolver falls back to file store
- Flag empty, env empty, keyring unavailable, token in file store (not expired) → returns file store token
- Token in file store but expired → falls through to error

### AC-5: Resolver returns actionable error when no token found
- Error message includes the `token_env` var name and `toolwright login` command
- Error message format: `tool "{name}" requires authentication. Set {TOKEN_ENV} or run "toolwright login {name}".`

### AC-6: Resolver never logs or prints token values
- Token strings never appear in error messages, debug output, or log lines
- Only metadata (source, expiry status) may be logged

### AC-7: Keyring store round-trips tokens
- `Set("my-toolkit/my-tool", token)` then `Get("my-toolkit/my-tool")` returns identical token
- Service name is `"toolwright"`, key is the toolkit/tool path
- `Delete("my-toolkit/my-tool")` then `Get()` returns not-found error

### AC-8: File store enforces 0600 permissions
- File created with 0600 permissions
- File with 0644 permissions → `Get()` returns error (refuses to read)
- File with 0600 permissions → `Get()` succeeds

### AC-9: File store respects XDG_CONFIG_HOME
- `XDG_CONFIG_HOME=/tmp/test` → file store reads/writes `/tmp/test/toolwright/tokens.json`
- `XDG_CONFIG_HOME` unset → defaults to `~/.config/toolwright/tokens.json`

### AC-10: File store is valid JSON with version field
- Written file parses as `{"version": 1, "tokens": {...}}`
- Multiple tokens stored under different keys coexist
- Set overwrites only the targeted key, preserving others

### AC-11: OAuth discovery finds endpoints via RFC 8414
- Mock server at `{providerURL}/.well-known/oauth-authorization-server` → returns authorization + token endpoints
- Discovery extracts `authorization_endpoint`, `token_endpoint`, and `jwks_uri`

### AC-12: OAuth discovery falls back to OIDC
- RFC 8414 endpoint returns 404, OIDC endpoint at `/.well-known/openid-configuration` returns metadata → succeeds

### AC-13: OAuth discovery falls back to manual endpoints
- Both discovery endpoints fail, `auth.Endpoints` provided → uses manual endpoints
- Both discovery endpoints fail, no manual endpoints → error naming both URLs tried

### AC-14: OAuth login performs PKCE flow
- PKCE verifier generated via `oauth2.GenerateVerifier()`
- Auth URL includes `code_challenge` (S256) and `state` parameter
- Callback validates state matches
- Exchange includes `VerifierOption(verifier)`
- Resulting token stored in keyring

### AC-15: OAuth callback server handles errors
- State mismatch on callback → error: "Security check failed (state mismatch)"
- No callback within 120 seconds → error: "Login cancelled or timed out"
- Callback server shuts down cleanly after receiving callback or timeout

### AC-16: OAuth callback server port selection
- Default port 8085 available → uses 8085
- Port 8085 in use → falls back to OS-assigned port
- Redirect URI reflects actual port used

### AC-17: Token refresh on expired access token
- Stored token expired, refresh token available → silent refresh attempted
- Refresh succeeds → new token stored, returned
- Refresh fails → error directing user to re-run `toolwright login`

### AC-18: Build and tests pass
- `go build ./...` succeeds
- `go test ./internal/auth/...` passes
- `go vet ./...` clean
