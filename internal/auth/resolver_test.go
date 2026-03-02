package auth

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Obsidian-Owl/toolwright/internal/manifest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Fake TokenStore for resolver tests
// ---------------------------------------------------------------------------

// fakeTokenStore implements TokenStore for testing the resolver.
// It returns preconfigured tokens or errors by key.
type fakeTokenStore struct {
	tokens map[string]*StoredToken
	err    error // if set, all Get calls return this error
}

func newFakeTokenStore() *fakeTokenStore {
	return &fakeTokenStore{
		tokens: make(map[string]*StoredToken),
	}
}

func (f *fakeTokenStore) Get(key string) (*StoredToken, error) {
	if f.err != nil {
		return nil, f.err
	}
	tok, ok := f.tokens[key]
	if !ok {
		return nil, fmt.Errorf("token not found for key %q", key)
	}
	copy := *tok
	return &copy, nil
}

// ---------------------------------------------------------------------------
// Test helpers and fixtures
// ---------------------------------------------------------------------------

func validToken(accessToken string) *StoredToken {
	return &StoredToken{
		AccessToken:  accessToken,
		RefreshToken: "refresh-" + accessToken,
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(1 * time.Hour), // valid for 1 hour
		Scopes:       []string{"read", "write"},
	}
}

func expiredToken(accessToken string) *StoredToken {
	return &StoredToken{
		AccessToken:  accessToken,
		RefreshToken: "",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(-1 * time.Hour), // expired 1 hour ago
		Scopes:       []string{"read"},
	}
}

func neverExpiringToken(accessToken string) *StoredToken {
	return &StoredToken{
		AccessToken:  accessToken,
		RefreshToken: "",
		TokenType:    "Bearer",
		Expiry:       time.Time{}, // zero means never expires
		Scopes:       []string{"all"},
	}
}

func tokenAuth(tokenEnv string) manifest.Auth {
	return manifest.Auth{
		Type:     "token",
		TokenEnv: tokenEnv,
	}
}

func oauth2Auth(tokenEnv string) manifest.Auth {
	return manifest.Auth{
		Type:     "oauth2",
		TokenEnv: tokenEnv,
	}
}

func noneAuth() manifest.Auth {
	return manifest.Auth{
		Type: "none",
	}
}

// ---------------------------------------------------------------------------
// Chain priority tests: verify the resolution order flag > env > keyring > filestore > error
// ---------------------------------------------------------------------------

func TestResolver_ChainPriority_FlagWinsOverEverything(t *testing.T) {
	// AC-1: Flag value is returned when provided, regardless of env/keyring/filestore.
	keyring := newFakeTokenStore()
	keyring.tokens["my-tool"] = validToken("keyring-token-value")

	filestore := newFakeTokenStore()
	filestore.tokens["my-tool"] = validToken("filestore-token-value")

	t.Setenv("MY_TOKEN", "env-token-value")

	r := &Resolver{Keyring: keyring, Store: filestore}
	auth := tokenAuth("MY_TOKEN")

	got, err := r.Resolve(context.Background(), auth, "my-tool", "flag-token-value")
	require.NoError(t, err, "Resolve must not error when flag is provided")
	assert.Equal(t, "flag-token-value", got,
		"Flag value must win over env, keyring, and filestore")
}

func TestResolver_ChainPriority_EnvWinsOverKeyringAndFilestore(t *testing.T) {
	// AC-2: Env var wins when flag is empty.
	keyring := newFakeTokenStore()
	keyring.tokens["my-tool"] = validToken("keyring-token-value")

	filestore := newFakeTokenStore()
	filestore.tokens["my-tool"] = validToken("filestore-token-value")

	t.Setenv("MY_TOKEN", "env-token-value")

	r := &Resolver{Keyring: keyring, Store: filestore}
	auth := tokenAuth("MY_TOKEN")

	got, err := r.Resolve(context.Background(), auth, "my-tool", "")
	require.NoError(t, err, "Resolve must not error when env var is set")
	assert.Equal(t, "env-token-value", got,
		"Env var must win over keyring and filestore when flag is empty")
}

func TestResolver_ChainPriority_KeyringWinsOverFilestore(t *testing.T) {
	// AC-3: Keyring wins when flag and env are empty.
	keyring := newFakeTokenStore()
	keyring.tokens["my-tool"] = validToken("keyring-token-value")

	filestore := newFakeTokenStore()
	filestore.tokens["my-tool"] = validToken("filestore-token-value")

	r := &Resolver{Keyring: keyring, Store: filestore}
	auth := tokenAuth("UNSET_TOKEN_VAR_CHAIN_TEST")

	got, err := r.Resolve(context.Background(), auth, "my-tool", "")
	require.NoError(t, err, "Resolve must not error when keyring has valid token")
	assert.Equal(t, "keyring-token-value", got,
		"Keyring token must win over filestore when flag and env are empty")
}

func TestResolver_ChainPriority_FilestoreIsLastResort(t *testing.T) {
	// AC-4: Filestore is used when flag, env, and keyring are all unavailable.
	keyring := newFakeTokenStore()
	keyring.err = fmt.Errorf("keyring unavailable")

	filestore := newFakeTokenStore()
	filestore.tokens["my-tool"] = validToken("filestore-token-value")

	r := &Resolver{Keyring: keyring, Store: filestore}
	auth := tokenAuth("UNSET_TOKEN_VAR_LASTRESORT")

	got, err := r.Resolve(context.Background(), auth, "my-tool", "")
	require.NoError(t, err, "Resolve must not error when filestore has valid token")
	assert.Equal(t, "filestore-token-value", got,
		"Filestore token must be returned when flag, env, and keyring are all unavailable")
}

// ---------------------------------------------------------------------------
// AC-1: Flag value tests
// ---------------------------------------------------------------------------

func TestResolver_FlagReturnedAsIs(t *testing.T) {
	tests := []struct {
		name      string
		flagValue string
	}{
		{name: "simple token", flagValue: "my-secret-token"},
		{name: "JWT-like token", flagValue: "eyJhbGciOiJSUzI1NiJ9.eyJzdWIiOiJ1c2VyIn0.sig"},
		{name: "token with special chars", flagValue: "tok+en/value=="},
		{name: "very long token", flagValue: strings.Repeat("a", 4096)},
		{name: "single character", flagValue: "x"},
		{name: "whitespace-only token", flagValue: "   "},
		{name: "token with newline", flagValue: "line1\nline2"},
		{name: "unicode token", flagValue: "token-\u00fc\u00e4\u00f6"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := &Resolver{Keyring: newFakeTokenStore(), Store: newFakeTokenStore()}
			auth := tokenAuth("UNUSED_ENV")

			got, err := r.Resolve(context.Background(), auth, "any-tool", tc.flagValue)
			require.NoError(t, err)
			assert.Equal(t, tc.flagValue, got,
				"Flag value must be returned unchanged")
		})
	}
}

func TestResolver_FlagWorksWithAuthTypeNone(t *testing.T) {
	// AC-1: Flag works regardless of auth type.
	r := &Resolver{Keyring: newFakeTokenStore(), Store: newFakeTokenStore()}
	auth := noneAuth()

	got, err := r.Resolve(context.Background(), auth, "tool-none", "explicit-token")
	require.NoError(t, err)
	assert.Equal(t, "explicit-token", got,
		"Flag must be returned even when auth type is 'none'")
}

func TestResolver_FlagWorksWithAuthTypeOAuth2(t *testing.T) {
	// AC-1: Flag works regardless of auth type.
	r := &Resolver{Keyring: newFakeTokenStore(), Store: newFakeTokenStore()}
	auth := oauth2Auth("SOME_ENV")

	got, err := r.Resolve(context.Background(), auth, "tool-oauth", "explicit-token")
	require.NoError(t, err)
	assert.Equal(t, "explicit-token", got,
		"Flag must be returned even when auth type is 'oauth2'")
}

func TestResolver_FlagWorksWithAuthTypeToken(t *testing.T) {
	r := &Resolver{Keyring: newFakeTokenStore(), Store: newFakeTokenStore()}
	auth := tokenAuth("SOME_ENV")

	got, err := r.Resolve(context.Background(), auth, "tool-token", "explicit-token")
	require.NoError(t, err)
	assert.Equal(t, "explicit-token", got,
		"Flag must be returned even when auth type is 'token'")
}

// ---------------------------------------------------------------------------
// AC-2: Environment variable fallback tests
// ---------------------------------------------------------------------------

func TestResolver_EnvVarResolvedByName(t *testing.T) {
	// AC-2: Env var resolved using the name from auth.TokenEnv.
	t.Setenv("MY_CUSTOM_TOKEN", "env-secret-123")

	r := &Resolver{Keyring: newFakeTokenStore(), Store: newFakeTokenStore()}
	auth := tokenAuth("MY_CUSTOM_TOKEN")

	got, err := r.Resolve(context.Background(), auth, "my-tool", "")
	require.NoError(t, err)
	assert.Equal(t, "env-secret-123", got,
		"Resolver must read the env var named by auth.TokenEnv")
}

func TestResolver_EnvVarDifferentNames(t *testing.T) {
	// Anti-hardcoding: verify the resolver actually reads the named env var,
	// not a hardcoded name.
	tests := []struct {
		envName  string
		envValue string
	}{
		{"ALPHA_TOKEN", "alpha-value"},
		{"BETA_TOKEN", "beta-value"},
		{"GITHUB_TOKEN", "gh-pat-xyz"},
	}

	for _, tc := range tests {
		t.Run(tc.envName, func(t *testing.T) {
			t.Setenv(tc.envName, tc.envValue)

			r := &Resolver{Keyring: newFakeTokenStore(), Store: newFakeTokenStore()}
			auth := tokenAuth(tc.envName)

			got, err := r.Resolve(context.Background(), auth, "tool", "")
			require.NoError(t, err)
			assert.Equal(t, tc.envValue, got,
				"Resolver must return the value of env var %q", tc.envName)
		})
	}
}

func TestResolver_EmptyEnvVarFallsThrough(t *testing.T) {
	// AC-2: Empty env var does not count as a value.
	t.Setenv("EMPTY_TOKEN", "")

	keyring := newFakeTokenStore()
	keyring.tokens["my-tool"] = validToken("keyring-fallback")

	r := &Resolver{Keyring: keyring, Store: newFakeTokenStore()}
	auth := tokenAuth("EMPTY_TOKEN")

	got, err := r.Resolve(context.Background(), auth, "my-tool", "")
	require.NoError(t, err)
	assert.Equal(t, "keyring-fallback", got,
		"Empty env var must fall through to keyring")
}

func TestResolver_UnsetEnvVarFallsThrough(t *testing.T) {
	// AC-2: Unset env var falls through.
	keyring := newFakeTokenStore()
	keyring.tokens["my-tool"] = validToken("keyring-fallback")

	r := &Resolver{Keyring: keyring, Store: newFakeTokenStore()}
	// Use a unique env var name that is definitely not set.
	auth := tokenAuth("DEFINITELY_UNSET_VAR_XYZ_RESOLVER_TEST")

	got, err := r.Resolve(context.Background(), auth, "my-tool", "")
	require.NoError(t, err)
	assert.Equal(t, "keyring-fallback", got,
		"Unset env var must fall through to keyring")
}

// ---------------------------------------------------------------------------
// AC-3: Keyring fallback tests
// ---------------------------------------------------------------------------

func TestResolver_ValidKeyringTokenReturned(t *testing.T) {
	// AC-3: Valid (non-expired) token in keyring is returned.
	keyring := newFakeTokenStore()
	keyring.tokens["my-tool"] = validToken("keyring-access-token")

	r := &Resolver{Keyring: keyring, Store: newFakeTokenStore()}
	auth := tokenAuth("UNSET_ENV_KEYRING_TEST")

	got, err := r.Resolve(context.Background(), auth, "my-tool", "")
	require.NoError(t, err)
	assert.Equal(t, "keyring-access-token", got,
		"Resolver must return the AccessToken from keyring when valid")
}

func TestResolver_NeverExpiringKeyringTokenReturned(t *testing.T) {
	// Zero expiry means never expires -- must be returned.
	keyring := newFakeTokenStore()
	keyring.tokens["my-tool"] = neverExpiringToken("keyring-pat")

	r := &Resolver{Keyring: keyring, Store: newFakeTokenStore()}
	auth := tokenAuth("UNSET_ENV_KEYRING_PAT")

	got, err := r.Resolve(context.Background(), auth, "my-tool", "")
	require.NoError(t, err)
	assert.Equal(t, "keyring-pat", got,
		"Token with zero expiry (never expires) must be returned from keyring")
}

func TestResolver_ExpiredKeyringTokenSkipped(t *testing.T) {
	// AC-3: Expired token in keyring with no refresh token falls through.
	keyring := newFakeTokenStore()
	keyring.tokens["my-tool"] = expiredToken("expired-keyring-token")

	filestore := newFakeTokenStore()
	filestore.tokens["my-tool"] = validToken("filestore-fallback")

	r := &Resolver{Keyring: keyring, Store: filestore}
	auth := tokenAuth("UNSET_ENV_EXPIRED_KEYRING")

	got, err := r.Resolve(context.Background(), auth, "my-tool", "")
	require.NoError(t, err)
	assert.Equal(t, "filestore-fallback", got,
		"Expired keyring token must fall through to filestore")
}

func TestResolver_KeyringErrorFallsThrough(t *testing.T) {
	// AC-3: Keyring error (not found, unavailable) falls through to filestore.
	keyring := newFakeTokenStore()
	keyring.err = fmt.Errorf("keyring: no such entry")

	filestore := newFakeTokenStore()
	filestore.tokens["my-tool"] = validToken("filestore-after-keyring-error")

	r := &Resolver{Keyring: keyring, Store: filestore}
	auth := tokenAuth("UNSET_ENV_KEYRING_ERR")

	got, err := r.Resolve(context.Background(), auth, "my-tool", "")
	require.NoError(t, err)
	assert.Equal(t, "filestore-after-keyring-error", got,
		"Keyring error must fall through to filestore")
}

func TestResolver_KeyringTokenNotFoundFallsThrough(t *testing.T) {
	// Keyring has no entry for this specific tool (but no global error).
	keyring := newFakeTokenStore()
	// keyring has a token for a DIFFERENT tool, not for "my-tool"
	keyring.tokens["other-tool"] = validToken("other-tool-token")

	filestore := newFakeTokenStore()
	filestore.tokens["my-tool"] = validToken("filestore-correct")

	r := &Resolver{Keyring: keyring, Store: filestore}
	auth := tokenAuth("UNSET_ENV_KEYRING_MISS")

	got, err := r.Resolve(context.Background(), auth, "my-tool", "")
	require.NoError(t, err)
	assert.Equal(t, "filestore-correct", got,
		"Missing keyring entry for tool must fall through to filestore")
}

// ---------------------------------------------------------------------------
// AC-4: File store fallback tests
// ---------------------------------------------------------------------------

func TestResolver_ValidFilestoreTokenReturned(t *testing.T) {
	// AC-4: Valid token in filestore is returned as last resort.
	keyring := newFakeTokenStore()
	keyring.err = fmt.Errorf("keyring unavailable")

	filestore := newFakeTokenStore()
	filestore.tokens["my-tool"] = validToken("filestore-access-token")

	r := &Resolver{Keyring: keyring, Store: filestore}
	auth := tokenAuth("UNSET_ENV_FILESTORE_TEST")

	got, err := r.Resolve(context.Background(), auth, "my-tool", "")
	require.NoError(t, err)
	assert.Equal(t, "filestore-access-token", got,
		"Resolver must return the AccessToken from filestore")
}

func TestResolver_NeverExpiringFilestoreTokenReturned(t *testing.T) {
	keyring := newFakeTokenStore()
	keyring.err = fmt.Errorf("keyring unavailable")

	filestore := newFakeTokenStore()
	filestore.tokens["my-tool"] = neverExpiringToken("filestore-pat")

	r := &Resolver{Keyring: keyring, Store: filestore}
	auth := tokenAuth("UNSET_ENV_FS_PAT")

	got, err := r.Resolve(context.Background(), auth, "my-tool", "")
	require.NoError(t, err)
	assert.Equal(t, "filestore-pat", got,
		"Token with zero expiry (never expires) must be returned from filestore")
}

func TestResolver_ExpiredFilestoreTokenFallsToError(t *testing.T) {
	// AC-4: Expired token in filestore falls through to error.
	keyring := newFakeTokenStore()
	keyring.err = fmt.Errorf("keyring unavailable")

	filestore := newFakeTokenStore()
	filestore.tokens["my-tool"] = expiredToken("expired-filestore-token")

	r := &Resolver{Keyring: keyring, Store: filestore}
	auth := tokenAuth("MY_FS_TOKEN")

	_, err := r.Resolve(context.Background(), auth, "my-tool", "")
	require.Error(t, err, "Expired filestore token must result in an error")
}

func TestResolver_FilestoreErrorFallsToError(t *testing.T) {
	// Filestore itself errors out.
	keyring := newFakeTokenStore()
	keyring.err = fmt.Errorf("keyring unavailable")

	filestore := newFakeTokenStore()
	filestore.err = fmt.Errorf("filestore: permission denied")

	r := &Resolver{Keyring: keyring, Store: filestore}
	auth := tokenAuth("MY_TOKEN_FS_ERR")

	_, err := r.Resolve(context.Background(), auth, "my-tool", "")
	require.Error(t, err, "Must error when all sources fail")
}

// ---------------------------------------------------------------------------
// AC-5: Error message format tests
// ---------------------------------------------------------------------------

func TestResolver_ErrorMessageFormat(t *testing.T) {
	// AC-5: Error message includes tool name, TOKEN_ENV var name, and "toolwright login".
	r := &Resolver{Keyring: newFakeTokenStore(), Store: newFakeTokenStore()}
	auth := tokenAuth("GITHUB_TOKEN")

	_, err := r.Resolve(context.Background(), auth, "github-cli", "")
	require.Error(t, err, "Must error when no token is found anywhere")

	errMsg := err.Error()
	assert.Contains(t, errMsg, "github-cli",
		"Error must contain the tool name")
	assert.Contains(t, errMsg, "GITHUB_TOKEN",
		"Error must contain the TOKEN_ENV var name")
	assert.Contains(t, errMsg, "toolwright login",
		"Error must contain the 'toolwright login' command")
}

func TestResolver_ErrorMessageExactFormat(t *testing.T) {
	// AC-5: Verify the exact error format specified in the spec.
	tests := []struct {
		name     string
		toolName string
		tokenEnv string
		wantErr  string
	}{
		{
			name:     "github tool",
			toolName: "github-cli",
			tokenEnv: "GITHUB_TOKEN",
			wantErr:  `tool "github-cli" requires authentication. Set GITHUB_TOKEN or run "toolwright login github-cli".`,
		},
		{
			name:     "custom tool",
			toolName: "my-api",
			tokenEnv: "MY_API_KEY",
			wantErr:  `tool "my-api" requires authentication. Set MY_API_KEY or run "toolwright login my-api".`,
		},
		{
			name:     "tool with special name",
			toolName: "acme-tool-v2",
			tokenEnv: "ACME_TOKEN",
			wantErr:  `tool "acme-tool-v2" requires authentication. Set ACME_TOKEN or run "toolwright login acme-tool-v2".`,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := &Resolver{Keyring: newFakeTokenStore(), Store: newFakeTokenStore()}
			auth := tokenAuth(tc.tokenEnv)

			_, err := r.Resolve(context.Background(), auth, tc.toolName, "")
			require.Error(t, err)
			assert.Equal(t, tc.wantErr, err.Error(),
				"Error message must match the exact format from the spec")
		})
	}
}

func TestResolver_ErrorMessageContainsLoginCommand(t *testing.T) {
	// The error must suggest "toolwright login {name}" with the actual tool name.
	r := &Resolver{Keyring: newFakeTokenStore(), Store: newFakeTokenStore()}
	auth := tokenAuth("SOME_TOKEN")

	_, err := r.Resolve(context.Background(), auth, "special-tool", "")
	require.Error(t, err)

	errMsg := err.Error()
	assert.Contains(t, errMsg, `toolwright login special-tool`,
		"Error must contain 'toolwright login {toolName}'")
}

// ---------------------------------------------------------------------------
// AC-6: Security tests — tokens never in errors or logs
// ---------------------------------------------------------------------------

func TestResolver_ErrorDoesNotContainTokenValues(t *testing.T) {
	// AC-6: Token strings must never appear in error messages.
	keyringTokenValue := "super-secret-keyring-token-abc123"
	filestoreTokenValue := "ultra-secret-filestore-token-xyz789"

	keyring := newFakeTokenStore()
	keyring.tokens["my-tool"] = expiredToken(keyringTokenValue)

	filestore := newFakeTokenStore()
	filestore.tokens["my-tool"] = expiredToken(filestoreTokenValue)

	r := &Resolver{Keyring: keyring, Store: filestore}
	auth := tokenAuth("MY_TOKEN_SEC")

	_, err := r.Resolve(context.Background(), auth, "my-tool", "")
	require.Error(t, err, "Must error when all tokens are expired")

	errMsg := err.Error()
	assert.NotContains(t, errMsg, keyringTokenValue,
		"Error message must NOT contain the keyring token value")
	assert.NotContains(t, errMsg, filestoreTokenValue,
		"Error message must NOT contain the filestore token value")
}

func TestResolver_ErrorDoesNotContainEnvTokenValue(t *testing.T) {
	// Even if we set the env var and something else causes failure,
	// token values from env must not appear in errors.
	// This test verifies the error path when env is empty but a
	// token value exists in a store.
	secretToken := "env-secret-value-never-show"

	keyring := newFakeTokenStore()
	keyring.tokens["my-tool"] = &StoredToken{
		AccessToken: secretToken,
		TokenType:   "Bearer",
		Expiry:      time.Now().Add(-1 * time.Hour), // expired
	}

	filestore := newFakeTokenStore()
	filestore.err = fmt.Errorf("filestore unavailable")

	r := &Resolver{Keyring: keyring, Store: filestore}
	auth := tokenAuth("UNSET_ENV_SEC_TEST")

	_, err := r.Resolve(context.Background(), auth, "my-tool", "")
	require.Error(t, err)

	assert.NotContains(t, err.Error(), secretToken,
		"Error message must NEVER contain token values")
}

func TestResolver_ReturnedValueIsAccessTokenString(t *testing.T) {
	// AC-6/implicit: The returned string must be just the access token,
	// not a JSON-serialized struct or anything else.
	keyring := newFakeTokenStore()
	keyring.tokens["my-tool"] = validToken("the-actual-access-token")

	r := &Resolver{Keyring: keyring, Store: newFakeTokenStore()}
	auth := tokenAuth("UNSET_ENV_RETURN_TEST")

	got, err := r.Resolve(context.Background(), auth, "my-tool", "")
	require.NoError(t, err)

	// Must be exactly the access token string, not JSON, not the whole struct.
	assert.Equal(t, "the-actual-access-token", got,
		"Returned value must be the bare AccessToken string")
	assert.NotContains(t, got, "refresh",
		"Returned value must not contain refresh token data")
	assert.NotContains(t, got, "{",
		"Returned value must not be JSON-serialized")
}

// ---------------------------------------------------------------------------
// Edge cases
// ---------------------------------------------------------------------------

func TestResolver_NilKeyringAndFilestore_FlagProvided(t *testing.T) {
	// Nil stores should not cause a panic when flag is provided.
	r := &Resolver{Keyring: nil, Store: nil}
	auth := tokenAuth("UNUSED")

	got, err := r.Resolve(context.Background(), auth, "my-tool", "my-flag-token")
	require.NoError(t, err)
	assert.Equal(t, "my-flag-token", got,
		"Nil stores must not panic when flag is provided")
}

func TestResolver_NilKeyringAndFilestore_NoFlag(t *testing.T) {
	// Nil stores, no flag, no env -> must error, not panic.
	r := &Resolver{Keyring: nil, Store: nil}
	auth := tokenAuth("UNSET_ENV_NIL_STORES")

	_, err := r.Resolve(context.Background(), auth, "my-tool", "")
	require.Error(t, err, "Must error when all sources are nil/unavailable")
}

func TestResolver_NilKeyringOnly_FilestoreHasToken(t *testing.T) {
	// Nil keyring should fall through gracefully to filestore.
	filestore := newFakeTokenStore()
	filestore.tokens["my-tool"] = validToken("filestore-with-nil-keyring")

	r := &Resolver{Keyring: nil, Store: filestore}
	auth := tokenAuth("UNSET_ENV_NIL_KR")

	got, err := r.Resolve(context.Background(), auth, "my-tool", "")
	require.NoError(t, err)
	assert.Equal(t, "filestore-with-nil-keyring", got,
		"Nil keyring must fall through to filestore")
}

func TestResolver_NilFilestoreOnly_KeyringHasToken(t *testing.T) {
	// Nil filestore should not be a problem if keyring succeeds.
	keyring := newFakeTokenStore()
	keyring.tokens["my-tool"] = validToken("keyring-with-nil-fs")

	r := &Resolver{Keyring: keyring, Store: nil}
	auth := tokenAuth("UNSET_ENV_NIL_FS")

	got, err := r.Resolve(context.Background(), auth, "my-tool", "")
	require.NoError(t, err)
	assert.Equal(t, "keyring-with-nil-fs", got,
		"Nil filestore must not prevent keyring from working")
}

func TestResolver_KeyLookupUsesToolName(t *testing.T) {
	// The key used for keyring/filestore lookup should be the tool name.
	// If the resolver uses a different key, it will miss the token.
	keyring := newFakeTokenStore()
	keyring.tokens["specific-tool-name"] = validToken("found-by-tool-name")

	r := &Resolver{Keyring: keyring, Store: newFakeTokenStore()}
	auth := tokenAuth("UNSET_ENV_KEY_FORMAT")

	got, err := r.Resolve(context.Background(), auth, "specific-tool-name", "")
	require.NoError(t, err)
	assert.Equal(t, "found-by-tool-name", got,
		"Keyring lookup key must be the tool name")
}

func TestResolver_KeyLookupForDifferentTools(t *testing.T) {
	// Anti-hardcoding: different tool names must look up different keys.
	tools := []struct {
		toolName    string
		accessToken string
	}{
		{"tool-alpha", "alpha-tok"},
		{"tool-beta", "beta-tok"},
		{"tool-gamma", "gamma-tok"},
	}

	for _, tc := range tools {
		t.Run(tc.toolName, func(t *testing.T) {
			keyring := newFakeTokenStore()
			keyring.tokens[tc.toolName] = validToken(tc.accessToken)

			r := &Resolver{Keyring: keyring, Store: newFakeTokenStore()}
			auth := tokenAuth("UNSET_ENV_MULTI_TOOLS")

			got, err := r.Resolve(context.Background(), auth, tc.toolName, "")
			require.NoError(t, err)
			assert.Equal(t, tc.accessToken, got,
				"Each tool must look up its own key")
		})
	}
}

func TestResolver_AuthTypeNone_StillFollowsChainWithEnv(t *testing.T) {
	// Auth type "none" should still follow the chain if env is provided.
	t.Setenv("NONE_AUTH_TOKEN", "env-for-none-auth")

	r := &Resolver{Keyring: newFakeTokenStore(), Store: newFakeTokenStore()}
	auth := manifest.Auth{
		Type:     "none",
		TokenEnv: "NONE_AUTH_TOKEN",
	}

	got, err := r.Resolve(context.Background(), auth, "none-tool", "")
	require.NoError(t, err)
	assert.Equal(t, "env-for-none-auth", got,
		"Auth type 'none' with env set should still return the env token")
}

// ---------------------------------------------------------------------------
// Comprehensive table-driven test for the full chain
// ---------------------------------------------------------------------------

func TestResolver_FullChain_TableDriven(t *testing.T) {
	tests := []struct {
		name         string
		flagValue    string
		envName      string
		envValue     string // if non-empty, set this env var
		keyringToken *StoredToken
		keyringErr   error
		fsToken      *StoredToken
		fsErr        error
		toolName     string
		wantToken    string
		wantErr      bool
		wantErrSub   string // substring that must appear in error
	}{
		{
			name:      "flag provided, nothing else",
			flagValue: "flag-only",
			envName:   "UNUSED_1",
			toolName:  "t1",
			wantToken: "flag-only",
		},
		{
			name:         "flag provided with everything else set",
			flagValue:    "flag-wins",
			envName:      "ENV_SET_1",
			envValue:     "env-value",
			keyringToken: validToken("kr-tok"),
			fsToken:      validToken("fs-tok"),
			toolName:     "t2",
			wantToken:    "flag-wins",
		},
		{
			name:      "env fallback",
			flagValue: "",
			envName:   "ENV_FALLBACK_1",
			envValue:  "env-val-fallback",
			toolName:  "t3",
			wantToken: "env-val-fallback",
		},
		{
			name:         "env over keyring",
			flagValue:    "",
			envName:      "ENV_OVER_KR",
			envValue:     "env-over-kr-val",
			keyringToken: validToken("kr-ignored"),
			toolName:     "t4",
			wantToken:    "env-over-kr-val",
		},
		{
			name:         "keyring fallback",
			flagValue:    "",
			envName:      "UNSET_KR_FB",
			keyringToken: validToken("kr-fb-val"),
			toolName:     "t5",
			wantToken:    "kr-fb-val",
		},
		{
			name:         "keyring expired, filestore valid",
			flagValue:    "",
			envName:      "UNSET_KR_EXP",
			keyringToken: expiredToken("kr-expired"),
			fsToken:      validToken("fs-valid"),
			toolName:     "t6",
			wantToken:    "fs-valid",
		},
		{
			name:       "keyring error, filestore valid",
			flagValue:  "",
			envName:    "UNSET_KR_ERR",
			keyringErr: fmt.Errorf("keyring broke"),
			fsToken:    validToken("fs-after-err"),
			toolName:   "t7",
			wantToken:  "fs-after-err",
		},
		{
			name:       "all fail",
			flagValue:  "",
			envName:    "UNSET_ALL_FAIL",
			keyringErr: fmt.Errorf("no keyring"),
			fsErr:      fmt.Errorf("no filestore"),
			toolName:   "all-fail-tool",
			wantErr:    true,
			wantErrSub: "all-fail-tool",
		},
		{
			name:         "keyring expired, filestore expired",
			flagValue:    "",
			envName:      "UNSET_BOTH_EXP",
			keyringToken: expiredToken("kr-exp"),
			fsToken:      expiredToken("fs-exp"),
			toolName:     "both-exp-tool",
			wantErr:      true,
			wantErrSub:   "both-exp-tool",
		},
		{
			name:       "keyring error, filestore error",
			flagValue:  "",
			envName:    "UNSET_BOTH_ERR",
			keyringErr: fmt.Errorf("kr err"),
			fsErr:      fmt.Errorf("fs err"),
			toolName:   "both-err-tool",
			wantErr:    true,
			wantErrSub: "both-err-tool",
		},
		{
			name:       "filestore as last resort",
			flagValue:  "",
			envName:    "UNSET_FS_LAST",
			keyringErr: fmt.Errorf("no keyring"),
			fsToken:    validToken("fs-last-resort-tok"),
			toolName:   "t10",
			wantToken:  "fs-last-resort-tok",
		},
		{
			name:         "never-expiring keyring token",
			flagValue:    "",
			envName:      "UNSET_NEVER_EXP",
			keyringToken: neverExpiringToken("kr-never-exp"),
			toolName:     "t11",
			wantToken:    "kr-never-exp",
		},
		{
			name:       "never-expiring filestore token",
			flagValue:  "",
			envName:    "UNSET_FS_NEVER",
			keyringErr: fmt.Errorf("no keyring"),
			fsToken:    neverExpiringToken("fs-never-exp"),
			toolName:   "t12",
			wantToken:  "fs-never-exp",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.envValue != "" {
				t.Setenv(tc.envName, tc.envValue)
			}

			keyring := newFakeTokenStore()
			if tc.keyringErr != nil {
				keyring.err = tc.keyringErr
			}
			if tc.keyringToken != nil {
				keyring.tokens[tc.toolName] = tc.keyringToken
			}

			filestore := newFakeTokenStore()
			if tc.fsErr != nil {
				filestore.err = tc.fsErr
			}
			if tc.fsToken != nil {
				filestore.tokens[tc.toolName] = tc.fsToken
			}

			r := &Resolver{Keyring: keyring, Store: filestore}
			auth := tokenAuth(tc.envName)

			got, err := r.Resolve(context.Background(), auth, tc.toolName, tc.flagValue)

			if tc.wantErr {
				require.Error(t, err, "Expected error for test case %q", tc.name)
				if tc.wantErrSub != "" {
					assert.Contains(t, err.Error(), tc.wantErrSub,
						"Error must contain %q", tc.wantErrSub)
				}
			} else {
				require.NoError(t, err, "Expected no error for test case %q", tc.name)
				assert.Equal(t, tc.wantToken, got,
					"Token mismatch for test case %q", tc.name)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Verify the Resolver struct uses the TokenStore interface
// ---------------------------------------------------------------------------

func TestResolver_AcceptsTokenStoreInterface(t *testing.T) {
	// Verify that the Resolver fields accept any TokenStore implementation,
	// not just concrete *KeyringStore or *FileStore types.
	// This is a compile-time check that the interface is correctly defined.
	var keyringStore TokenStore = newFakeTokenStore()
	var fileStore TokenStore = newFakeTokenStore()

	r := &Resolver{
		Keyring: keyringStore,
		Store:   fileStore,
	}

	// Just verify it doesn't panic with the interface values.
	_, _ = r.Resolve(context.Background(), tokenAuth("UNSET_IFACE"), "tool", "flag-val")
}

// ---------------------------------------------------------------------------
// Verify that both KeyringStore and FileStore satisfy TokenStore
// ---------------------------------------------------------------------------

func TestTokenStoreInterface_KeyringStoreImplements(t *testing.T) {
	// Compile-time check: *KeyringStore must implement TokenStore.
	var _ TokenStore = (*KeyringStore)(nil)
}

func TestTokenStoreInterface_FileStoreImplements(t *testing.T) {
	// Compile-time check: *FileStore must implement TokenStore.
	var _ TokenStore = (*FileStore)(nil)
}

// ---------------------------------------------------------------------------
// Stress test: multiple sequential resolves return correct independent results
// ---------------------------------------------------------------------------

func TestResolver_MultipleResolveCallsAreIndependent(t *testing.T) {
	// Guard against implementations that cache or mutate state between calls.
	keyring := newFakeTokenStore()
	keyring.tokens["tool-a"] = validToken("token-a")
	keyring.tokens["tool-b"] = validToken("token-b")

	r := &Resolver{Keyring: keyring, Store: newFakeTokenStore()}

	gotA, err := r.Resolve(context.Background(), tokenAuth("UNSET_MULTI_A"), "tool-a", "")
	require.NoError(t, err)

	gotB, err := r.Resolve(context.Background(), tokenAuth("UNSET_MULTI_B"), "tool-b", "")
	require.NoError(t, err)

	assert.Equal(t, "token-a", gotA)
	assert.Equal(t, "token-b", gotB)
	assert.NotEqual(t, gotA, gotB,
		"Different tools must resolve to different tokens")
}

func TestResolver_SameToolResolvedTwiceReturnsSameResult(t *testing.T) {
	keyring := newFakeTokenStore()
	keyring.tokens["my-tool"] = validToken("consistent-token")

	r := &Resolver{Keyring: keyring, Store: newFakeTokenStore()}
	auth := tokenAuth("UNSET_CONSISTENT")

	got1, err := r.Resolve(context.Background(), auth, "my-tool", "")
	require.NoError(t, err)

	got2, err := r.Resolve(context.Background(), auth, "my-tool", "")
	require.NoError(t, err)

	assert.Equal(t, got1, got2,
		"Same tool resolved twice must return the same token")
	assert.Equal(t, "consistent-token", got1)
}

// ---------------------------------------------------------------------------
// Error path: empty TokenEnv in auth
// ---------------------------------------------------------------------------

func TestResolver_EmptyTokenEnv_FallsToKeyring(t *testing.T) {
	// If auth.TokenEnv is empty, the env step should be skipped entirely
	// (there is no env var to check).
	keyring := newFakeTokenStore()
	keyring.tokens["my-tool"] = validToken("keyring-no-env")

	r := &Resolver{Keyring: keyring, Store: newFakeTokenStore()}
	auth := manifest.Auth{
		Type:     "token",
		TokenEnv: "", // no env var configured
	}

	got, err := r.Resolve(context.Background(), auth, "my-tool", "")
	require.NoError(t, err)
	assert.Equal(t, "keyring-no-env", got,
		"Empty TokenEnv must skip env step and fall through to keyring")
}

// ---------------------------------------------------------------------------
// Error path: all expired, error does not leak tokens
// ---------------------------------------------------------------------------

func TestResolver_AllExpired_ErrorDoesNotLeakAnyToken(t *testing.T) {
	keyringSecret := "keyring-secret-xyzzy-12345"
	filestoreSecret := "filestore-secret-plugh-67890"

	keyring := newFakeTokenStore()
	keyring.tokens["my-tool"] = expiredToken(keyringSecret)

	filestore := newFakeTokenStore()
	filestore.tokens["my-tool"] = expiredToken(filestoreSecret)

	r := &Resolver{Keyring: keyring, Store: filestore}
	auth := tokenAuth("MY_TOKEN_LEAK_TEST")

	_, err := r.Resolve(context.Background(), auth, "my-tool", "")
	require.Error(t, err)

	errMsg := err.Error()
	// Check that neither secret token value appears in the error.
	assert.NotContains(t, errMsg, keyringSecret)
	assert.NotContains(t, errMsg, filestoreSecret)
	// Also check for the refresh token values.
	assert.NotContains(t, errMsg, "refresh-"+keyringSecret)
	assert.NotContains(t, errMsg, "refresh-"+filestoreSecret)
}
