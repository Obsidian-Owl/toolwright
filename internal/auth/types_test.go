package auth

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Test fixtures
// ---------------------------------------------------------------------------

// fullStoredToken returns a StoredToken with every field populated.
// Used across multiple tests to guard against sloppy partial implementations.
func fullStoredToken() StoredToken {
	return StoredToken{
		AccessToken:  "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.test",
		RefreshToken: "dGhpcyBpcyBhIHJlZnJlc2ggdG9rZW4",
		TokenType:    "Bearer",
		Expiry:       time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC),
		Scopes:       []string{"read", "write", "admin"},
	}
}

// fullTokenFile returns a TokenFile with version and multiple token entries.
func fullTokenFile() TokenFile {
	return TokenFile{
		Version: 1,
		Tokens: map[string]StoredToken{
			"https://api.example.com": {
				AccessToken:  "access-example",
				RefreshToken: "refresh-example",
				TokenType:    "Bearer",
				Expiry:       time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC),
				Scopes:       []string{"read", "write"},
			},
			"https://api.other.com": {
				AccessToken:  "access-other",
				RefreshToken: "",
				TokenType:    "MAC",
				Expiry:       time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
				Scopes:       []string{"admin"},
			},
		},
	}
}

// ---------------------------------------------------------------------------
// StoredToken: construction with all fields
// ---------------------------------------------------------------------------

func TestStoredToken_ConstructWithAllFields(t *testing.T) {
	tok := fullStoredToken()

	// Verify each field individually to catch missing or misspelled field names.
	assert.Equal(t, "eyJhbGciOiJSUzI1NiIsInR5cCI6IkpXVCJ9.test", tok.AccessToken,
		"AccessToken field must hold the assigned value")
	assert.Equal(t, "dGhpcyBpcyBhIHJlZnJlc2ggdG9rZW4", tok.RefreshToken,
		"RefreshToken field must hold the assigned value")
	assert.Equal(t, "Bearer", tok.TokenType,
		"TokenType field must hold the assigned value")
	assert.Equal(t, time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC), tok.Expiry,
		"Expiry field must hold the assigned time.Time value")
	assert.Equal(t, []string{"read", "write", "admin"}, tok.Scopes,
		"Scopes field must hold the assigned string slice")
}

func TestStoredToken_ZeroValue(t *testing.T) {
	// The zero value should have empty strings, zero time, and nil scopes.
	var tok StoredToken
	assert.Empty(t, tok.AccessToken, "zero-value AccessToken should be empty string")
	assert.Empty(t, tok.RefreshToken, "zero-value RefreshToken should be empty string")
	assert.Empty(t, tok.TokenType, "zero-value TokenType should be empty string")
	assert.True(t, tok.Expiry.IsZero(), "zero-value Expiry should be zero time")
	assert.Nil(t, tok.Scopes, "zero-value Scopes should be nil")
}

// ---------------------------------------------------------------------------
// StoredToken: JSON round-trip with snake_case tags
// ---------------------------------------------------------------------------

func TestStoredToken_JSON_RoundTrip(t *testing.T) {
	original := fullStoredToken()

	data, err := json.Marshal(original)
	require.NoError(t, err, "json.Marshal(StoredToken) must not error")

	var restored StoredToken
	err = json.Unmarshal(data, &restored)
	require.NoError(t, err, "json.Unmarshal(StoredToken) must not error")

	if diff := cmp.Diff(original, restored); diff != "" {
		t.Errorf("StoredToken JSON round-trip mismatch (-original +restored):\n%s", diff)
	}
}

func TestStoredToken_JSON_FieldNames_SnakeCase(t *testing.T) {
	tok := fullStoredToken()

	data, err := json.Marshal(tok)
	require.NoError(t, err)

	// Unmarshal into a raw map to inspect actual JSON key names.
	var raw map[string]json.RawMessage
	err = json.Unmarshal(data, &raw)
	require.NoError(t, err, "StoredToken JSON should be a JSON object")

	// These are the exact snake_case keys required by the spec.
	requiredKeys := []string{"access_token", "refresh_token", "token_type", "expiry", "scopes"}
	for _, key := range requiredKeys {
		_, exists := raw[key]
		assert.True(t, exists, "JSON output must contain snake_case key %q", key)
	}

	// Ensure PascalCase/camelCase keys are NOT present (catches missing json tags).
	forbiddenKeys := []string{
		"AccessToken", "accessToken",
		"RefreshToken", "refreshToken",
		"TokenType", "tokenType",
		"Expiry",
		"Scopes",
	}
	for _, key := range forbiddenKeys {
		_, exists := raw[key]
		assert.False(t, exists,
			"JSON output must NOT contain key %q (should be snake_case via json tags)", key)
	}
}

func TestStoredToken_JSON_Unmarshal_SnakeCaseKeys(t *testing.T) {
	// Prove that unmarshalling from snake_case JSON populates the struct correctly.
	input := `{
		"access_token": "my-access-token",
		"refresh_token": "my-refresh-token",
		"token_type": "Bearer",
		"expiry": "2026-06-15T12:00:00Z",
		"scopes": ["read", "write"]
	}`

	var tok StoredToken
	err := json.Unmarshal([]byte(input), &tok)
	require.NoError(t, err)

	assert.Equal(t, "my-access-token", tok.AccessToken)
	assert.Equal(t, "my-refresh-token", tok.RefreshToken)
	assert.Equal(t, "Bearer", tok.TokenType)
	assert.Equal(t, time.Date(2026, 6, 15, 12, 0, 0, 0, time.UTC), tok.Expiry)
	assert.Equal(t, []string{"read", "write"}, tok.Scopes)
}

func TestStoredToken_JSON_FieldCount(t *testing.T) {
	// Ensure no extra fields appear in the JSON output (catches accidental
	// extra exported fields that would leak data).
	tok := fullStoredToken()

	data, err := json.Marshal(tok)
	require.NoError(t, err)

	var raw map[string]json.RawMessage
	err = json.Unmarshal(data, &raw)
	require.NoError(t, err)

	assert.Len(t, raw, 5,
		"StoredToken JSON should have exactly 5 fields; got keys: %v", mapKeys(raw))
}

func TestStoredToken_JSON_EmptyScopes(t *testing.T) {
	// A token with an empty (but non-nil) scopes slice should round-trip
	// to an empty JSON array, not null.
	tok := StoredToken{
		AccessToken:  "a",
		RefreshToken: "b",
		TokenType:    "Bearer",
		Expiry:       time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Scopes:       []string{},
	}

	data, err := json.Marshal(tok)
	require.NoError(t, err)

	var restored StoredToken
	err = json.Unmarshal(data, &restored)
	require.NoError(t, err)

	// After round-trip, Scopes may be nil or empty slice -- both are acceptable.
	// But the other fields must survive.
	assert.Equal(t, "a", restored.AccessToken)
	assert.Equal(t, "b", restored.RefreshToken)
	assert.Equal(t, "Bearer", restored.TokenType)
}

func TestStoredToken_JSON_EmptyRefreshToken(t *testing.T) {
	// Some token responses have no refresh token. The field should still be
	// present in JSON (even if empty string) and round-trip correctly.
	tok := StoredToken{
		AccessToken:  "access-only",
		RefreshToken: "",
		TokenType:    "Bearer",
		Expiry:       time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Scopes:       []string{"read"},
	}

	data, err := json.Marshal(tok)
	require.NoError(t, err)

	var restored StoredToken
	err = json.Unmarshal(data, &restored)
	require.NoError(t, err)

	assert.Equal(t, "access-only", restored.AccessToken)
	assert.Empty(t, restored.RefreshToken, "Empty RefreshToken must survive round-trip as empty string")
}

// ---------------------------------------------------------------------------
// TokenFile: construction with all fields
// ---------------------------------------------------------------------------

func TestTokenFile_ConstructWithAllFields(t *testing.T) {
	tf := fullTokenFile()

	assert.Equal(t, 1, tf.Version, "Version field must hold assigned value")
	require.Len(t, tf.Tokens, 2, "Tokens map must have 2 entries")

	tok1, ok := tf.Tokens["https://api.example.com"]
	require.True(t, ok, "Tokens must contain key 'https://api.example.com'")
	assert.Equal(t, "access-example", tok1.AccessToken)

	tok2, ok := tf.Tokens["https://api.other.com"]
	require.True(t, ok, "Tokens must contain key 'https://api.other.com'")
	assert.Equal(t, "access-other", tok2.AccessToken)
}

func TestTokenFile_ZeroValue(t *testing.T) {
	var tf TokenFile
	assert.Equal(t, 0, tf.Version, "zero-value Version should be 0")
	assert.Nil(t, tf.Tokens, "zero-value Tokens should be nil")
}

// ---------------------------------------------------------------------------
// TokenFile: JSON round-trip
// ---------------------------------------------------------------------------

func TestTokenFile_JSON_RoundTrip(t *testing.T) {
	original := fullTokenFile()

	data, err := json.Marshal(original)
	require.NoError(t, err, "json.Marshal(TokenFile) must not error")

	var restored TokenFile
	err = json.Unmarshal(data, &restored)
	require.NoError(t, err, "json.Unmarshal(TokenFile) must not error")

	if diff := cmp.Diff(original, restored); diff != "" {
		t.Errorf("TokenFile JSON round-trip mismatch (-original +restored):\n%s", diff)
	}
}

func TestTokenFile_JSON_FieldNames(t *testing.T) {
	tf := fullTokenFile()

	data, err := json.Marshal(tf)
	require.NoError(t, err)

	var raw map[string]json.RawMessage
	err = json.Unmarshal(data, &raw)
	require.NoError(t, err)

	// Must have exactly "version" and "tokens" as keys.
	_, hasVersion := raw["version"]
	assert.True(t, hasVersion, "JSON output must contain key 'version'")

	_, hasTokens := raw["tokens"]
	assert.True(t, hasTokens, "JSON output must contain key 'tokens'")

	// Must NOT have PascalCase keys.
	_, hasVersionPascal := raw["Version"]
	assert.False(t, hasVersionPascal, "JSON output must NOT contain 'Version' (needs json tag)")

	_, hasTokensPascal := raw["Tokens"]
	assert.False(t, hasTokensPascal, "JSON output must NOT contain 'Tokens' (needs json tag)")

	assert.Len(t, raw, 2, "TokenFile JSON should have exactly 2 fields")
}

func TestTokenFile_JSON_Unmarshal_FromSnakeCase(t *testing.T) {
	input := `{
		"version": 2,
		"tokens": {
			"https://example.com": {
				"access_token": "tok-abc",
				"refresh_token": "ref-abc",
				"token_type": "Bearer",
				"expiry": "2026-12-31T23:59:59Z",
				"scopes": ["all"]
			}
		}
	}`

	var tf TokenFile
	err := json.Unmarshal([]byte(input), &tf)
	require.NoError(t, err)

	assert.Equal(t, 2, tf.Version)
	require.Len(t, tf.Tokens, 1)

	tok, ok := tf.Tokens["https://example.com"]
	require.True(t, ok)
	assert.Equal(t, "tok-abc", tok.AccessToken)
	assert.Equal(t, "ref-abc", tok.RefreshToken)
	assert.Equal(t, "Bearer", tok.TokenType)
	assert.Equal(t, time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC), tok.Expiry)
	assert.Equal(t, []string{"all"}, tok.Scopes)
}

func TestTokenFile_JSON_EmptyTokensMap(t *testing.T) {
	tf := TokenFile{
		Version: 1,
		Tokens:  map[string]StoredToken{},
	}

	data, err := json.Marshal(tf)
	require.NoError(t, err)

	var restored TokenFile
	err = json.Unmarshal(data, &restored)
	require.NoError(t, err)

	assert.Equal(t, 1, restored.Version)
	// After round-trip, empty map should be empty (not nil with entries).
	assert.Empty(t, restored.Tokens, "empty tokens map should round-trip as empty")
}

func TestTokenFile_JSON_MultipleTokens_AllPreserved(t *testing.T) {
	// Guard against implementations that only store the first or last token.
	tf := TokenFile{
		Version: 1,
		Tokens: map[string]StoredToken{
			"provider-a": {AccessToken: "a", TokenType: "Bearer", Scopes: []string{"x"}},
			"provider-b": {AccessToken: "b", TokenType: "Bearer", Scopes: []string{"y"}},
			"provider-c": {AccessToken: "c", TokenType: "Bearer", Scopes: []string{"z"}},
		},
	}

	data, err := json.Marshal(tf)
	require.NoError(t, err)

	var restored TokenFile
	err = json.Unmarshal(data, &restored)
	require.NoError(t, err)

	require.Len(t, restored.Tokens, 3, "all 3 token entries must survive round-trip")
	assert.Equal(t, "a", restored.Tokens["provider-a"].AccessToken)
	assert.Equal(t, "b", restored.Tokens["provider-b"].AccessToken)
	assert.Equal(t, "c", restored.Tokens["provider-c"].AccessToken)
}

// ---------------------------------------------------------------------------
// StoredToken.IsExpired(): table-driven tests
// ---------------------------------------------------------------------------

func TestStoredToken_IsExpired(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name        string
		expiry      time.Time
		wantExpired bool
	}{
		{
			name:        "expiry in the past by 1 hour",
			expiry:      now.Add(-1 * time.Hour),
			wantExpired: true,
		},
		{
			name:        "expiry in the past by 1 second",
			expiry:      now.Add(-1 * time.Second),
			wantExpired: true,
		},
		{
			name:        "expiry in the past by 1 nanosecond",
			expiry:      now.Add(-1 * time.Nanosecond),
			wantExpired: true,
		},
		{
			name:        "expiry far in the past (year 2000)",
			expiry:      time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC),
			wantExpired: true,
		},
		{
			name:        "expiry in the future by 1 hour",
			expiry:      now.Add(1 * time.Hour),
			wantExpired: false,
		},
		{
			name:        "expiry in the future by 1 second",
			expiry:      now.Add(1 * time.Second),
			wantExpired: false,
		},
		{
			name:        "expiry far in the future (year 2099)",
			expiry:      time.Date(2099, 12, 31, 23, 59, 59, 0, time.UTC),
			wantExpired: false,
		},
		{
			name:        "zero expiry means never expires",
			expiry:      time.Time{},
			wantExpired: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			tok := StoredToken{
				AccessToken: "test-token",
				TokenType:   "Bearer",
				Expiry:      tc.expiry,
			}

			got := tok.IsExpired()

			if tc.wantExpired {
				assert.True(t, got, "IsExpired() should return true for %s", tc.name)
			} else {
				assert.False(t, got, "IsExpired() should return false for %s", tc.name)
			}
		})
	}
}

func TestStoredToken_IsExpired_ZeroExpiry_NeverExpires(t *testing.T) {
	// Dedicated test: zero time.Time must mean "never expires", returning false.
	// This is critical for tokens with no expiration (e.g., personal access tokens).
	tok := StoredToken{
		AccessToken: "pat-never-expires",
		TokenType:   "Bearer",
		Expiry:      time.Time{},
	}

	assert.True(t, tok.Expiry.IsZero(), "precondition: Expiry must be zero time")
	assert.False(t, tok.IsExpired(),
		"IsExpired() must return false when Expiry is zero (never expires)")
}

func TestStoredToken_IsExpired_PastExpiry_ReturnsTrue(t *testing.T) {
	// Dedicated test: a definitely-past expiry must return true.
	tok := StoredToken{
		AccessToken: "expired-token",
		TokenType:   "Bearer",
		Expiry:      time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	assert.True(t, tok.IsExpired(),
		"IsExpired() must return true for a token that expired in 2020")
}

func TestStoredToken_IsExpired_FutureExpiry_ReturnsFalse(t *testing.T) {
	// Dedicated test: a definitely-future expiry must return false.
	tok := StoredToken{
		AccessToken: "valid-token",
		TokenType:   "Bearer",
		Expiry:      time.Date(2099, 12, 31, 23, 59, 59, 0, time.UTC),
	}

	assert.False(t, tok.IsExpired(),
		"IsExpired() must return false for a token expiring in 2099")
}

func TestStoredToken_IsExpired_ReturnsBool(t *testing.T) {
	// Guard against an implementation that always returns the same value.
	// We call IsExpired on two different tokens and expect different results.
	expired := StoredToken{
		AccessToken: "old",
		Expiry:      time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	valid := StoredToken{
		AccessToken: "new",
		Expiry:      time.Date(2099, 1, 1, 0, 0, 0, 0, time.UTC),
	}

	assert.True(t, expired.IsExpired(), "expired token should be expired")
	assert.False(t, valid.IsExpired(), "valid token should not be expired")

	// Both results must be different from each other.
	assert.NotEqual(t, expired.IsExpired(), valid.IsExpired(),
		"IsExpired must return different values for past vs future expiry")
}

func TestStoredToken_IsExpired_DoesNotMutateToken(t *testing.T) {
	// IsExpired should be a read-only operation. Verify the token is unchanged.
	original := fullStoredToken()
	before := original // copy

	_ = original.IsExpired()

	if diff := cmp.Diff(before, original); diff != "" {
		t.Errorf("IsExpired() mutated the StoredToken (-before +after):\n%s", diff)
	}
}

// ---------------------------------------------------------------------------
// Anti-hardcoding: multiple distinct tokens must all round-trip
// ---------------------------------------------------------------------------

func TestStoredToken_JSON_RoundTrip_MultipleDistinctTokens(t *testing.T) {
	// A hardcoded implementation that returns a fixed token would fail this.
	tokens := []StoredToken{
		{
			AccessToken:  "alpha-access",
			RefreshToken: "alpha-refresh",
			TokenType:    "Bearer",
			Expiry:       time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC),
			Scopes:       []string{"read"},
		},
		{
			AccessToken:  "bravo-access",
			RefreshToken: "",
			TokenType:    "MAC",
			Expiry:       time.Date(2027, 6, 15, 12, 30, 0, 0, time.UTC),
			Scopes:       []string{"write", "delete"},
		},
		{
			AccessToken:  "charlie-access",
			RefreshToken: "charlie-refresh",
			TokenType:    "Bearer",
			Expiry:       time.Time{}, // zero / never expires
			Scopes:       nil,
		},
	}

	for i, original := range tokens {
		data, err := json.Marshal(original)
		require.NoError(t, err, "Marshal token %d", i)

		var restored StoredToken
		err = json.Unmarshal(data, &restored)
		require.NoError(t, err, "Unmarshal token %d", i)

		// Compare the core fields that must survive (Scopes nil vs empty is allowed).
		assert.Equal(t, original.AccessToken, restored.AccessToken, "token %d AccessToken", i)
		assert.Equal(t, original.RefreshToken, restored.RefreshToken, "token %d RefreshToken", i)
		assert.Equal(t, original.TokenType, restored.TokenType, "token %d TokenType", i)
		assert.Equal(t, original.Expiry, restored.Expiry, "token %d Expiry", i)
	}
}

// ---------------------------------------------------------------------------
// Edge case: JSON with unknown fields should not error (forward compat)
// ---------------------------------------------------------------------------

func TestStoredToken_JSON_Unmarshal_UnknownFields(t *testing.T) {
	// Tokens from a newer version of the tool may have extra fields.
	// Unmarshalling should not error on unknown fields.
	input := `{
		"access_token": "tok",
		"refresh_token": "ref",
		"token_type": "Bearer",
		"expiry": "2026-01-01T00:00:00Z",
		"scopes": ["read"],
		"extra_field": "should be ignored",
		"another_unknown": 42
	}`

	var tok StoredToken
	err := json.Unmarshal([]byte(input), &tok)
	require.NoError(t, err, "unknown fields should not cause unmarshal error")
	assert.Equal(t, "tok", tok.AccessToken)
}

func TestTokenFile_JSON_Unmarshal_UnknownFields(t *testing.T) {
	input := `{
		"version": 1,
		"tokens": {},
		"unknown_field": "ignored"
	}`

	var tf TokenFile
	err := json.Unmarshal([]byte(input), &tf)
	require.NoError(t, err, "unknown fields should not cause unmarshal error")
	assert.Equal(t, 1, tf.Version)
}

// ---------------------------------------------------------------------------
// Edge case: Token map key is an arbitrary string (URL, hostname, etc.)
// ---------------------------------------------------------------------------

func TestTokenFile_JSON_SpecialMapKeys(t *testing.T) {
	// Token map keys can be URLs with special characters. They must round-trip.
	tf := TokenFile{
		Version: 1,
		Tokens: map[string]StoredToken{
			"https://auth.example.com/oauth2?audience=api": {
				AccessToken: "special-key-token",
				TokenType:   "Bearer",
				Expiry:      time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
				Scopes:      []string{"read"},
			},
		},
	}

	data, err := json.Marshal(tf)
	require.NoError(t, err)

	var restored TokenFile
	err = json.Unmarshal(data, &restored)
	require.NoError(t, err)

	_, ok := restored.Tokens["https://auth.example.com/oauth2?audience=api"]
	assert.True(t, ok, "URL key with query params must survive JSON round-trip")
}
