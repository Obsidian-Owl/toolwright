# Plan: Unit 2 — auth

## Task Breakdown

### Task 1: Token types
- File: `internal/auth/types.go`
- `StoredToken` struct: `AccessToken`, `RefreshToken`, `TokenType`, `Expiry time.Time`, `Scopes []string`
- `TokenFile` struct: `Version int`, `Tokens map[string]StoredToken`

### Task 2: Keyring store
- File: `internal/auth/keyring.go`
- `KeyringStore` struct wrapping `go-keyring`
- `Get(key string) (*StoredToken, error)` — deserialize from keyring
- `Set(key string, token StoredToken) error` — serialize to keyring
- `Delete(key string) error`
- Service name: `"toolwright"`

### Task 3: Fallback file store
- File: `internal/auth/store.go`
- `FileStore` struct with configurable base path
- `NewFileStore(basePath string) *FileStore` — defaults to `$XDG_CONFIG_HOME/toolwright/`
- `Get(key string) (*StoredToken, error)` — read tokens.json, check permissions
- `Set(key string, token StoredToken) error` — read-modify-write tokens.json with 0600
- `Delete(key string) error`
- Permission check: refuse to read if file mode > 0600

### Task 4: Token resolver
- File: `internal/auth/resolver.go`
- `Resolver` struct with `KeyringStore` and `FileStore`
- `Resolve(ctx context.Context, auth manifest.Auth, flagValue string) (string, error)`
- Resolution chain: flag → env → keyring → file store → error
- Expiry check on stored tokens
- Error message format: `tool "{name}" requires authentication. Set {TOKEN_ENV} or run "toolwright login {name}".`

### Task 5: OAuth discovery
- File: `internal/auth/oauth.go` (discovery portion)
- `DiscoverEndpoints(ctx context.Context, providerURL string, manual *manifest.Endpoints) (*oauth2.Endpoint, string, error)`
- Try `{providerURL}/.well-known/oauth-authorization-server` (RFC 8414)
- Fallback to `{providerURL}/.well-known/openid-configuration`
- Fallback to manual endpoints if provided
- Error if none succeed: name the URLs tried

### Task 6: OAuth PKCE login flow
- File: `internal/auth/oauth.go` (login portion)
- `Login(ctx context.Context, auth manifest.Auth, openBrowser bool) (*StoredToken, error)`
- PKCE verifier via `oauth2.GenerateVerifier()`
- State parameter: 32 bytes crypto/rand, base64url
- Callback server on 127.0.0.1:8085 (fallback port 0)
- 120s timeout
- State validation on callback
- Token exchange via `oauth2.Config.Exchange()` with `VerifierOption`
- Store result in keyring (fallback file store)

### Task 7: Token refresh
- File: `internal/auth/oauth.go` (refresh portion)
- `Refresh(ctx context.Context, auth manifest.Auth, stored StoredToken) (*StoredToken, error)`
- Use `oauth2.TokenSource` for silent refresh
- On failure: return error directing user to re-run login

## File Change Map

| File | Action | Package |
|------|--------|---------|
| `internal/auth/types.go` | Create | auth |
| `internal/auth/keyring.go` | Create | auth |
| `internal/auth/keyring_test.go` | Create | auth |
| `internal/auth/store.go` | Create | auth |
| `internal/auth/store_test.go` | Create | auth |
| `internal/auth/resolver.go` | Create | auth |
| `internal/auth/resolver_test.go` | Create | auth |
| `internal/auth/oauth.go` | Create | auth |
| `internal/auth/oauth_test.go` | Create | auth |
| `go.mod` | Update | root (add go-keyring, x/oauth2) |
