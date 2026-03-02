package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Fake keyring implementation for testing
// ---------------------------------------------------------------------------

// errKeyNotFound is the sentinel returned by fakeKeyring when a key does not exist.
var errKeyNotFound = errors.New("key not found in fake keyring")

// fakeKeyring implements the Keyring interface using an in-memory map.
// It records every call so tests can verify service names, keys, and values.
type fakeKeyring struct {
	store map[string]string // "service:key" -> value

	// Call logs for inspection.
	setCalls    []fakeKeyringCall
	getCalls    []fakeKeyringCall
	deleteCalls []fakeKeyringCall

	// Optional error injection.
	setErr    error
	getErr    error
	deleteErr error
}

type fakeKeyringCall struct {
	Service string
	Key     string
	Value   string // only populated for Set calls
}

func newFakeKeyring() *fakeKeyring {
	return &fakeKeyring{
		store: make(map[string]string),
	}
}

func (f *fakeKeyring) Set(service, key, value string) error {
	f.setCalls = append(f.setCalls, fakeKeyringCall{Service: service, Key: key, Value: value})
	if f.setErr != nil {
		return f.setErr
	}
	f.store[service+"\x00"+key] = value
	return nil
}

func (f *fakeKeyring) Get(service, key string) (string, error) {
	f.getCalls = append(f.getCalls, fakeKeyringCall{Service: service, Key: key})
	if f.getErr != nil {
		return "", f.getErr
	}
	v, ok := f.store[service+"\x00"+key]
	if !ok {
		return "", errKeyNotFound
	}
	return v, nil
}

func (f *fakeKeyring) Delete(service, key string) error {
	f.deleteCalls = append(f.deleteCalls, fakeKeyringCall{Service: service, Key: key})
	if f.deleteErr != nil {
		return f.deleteErr
	}
	delete(f.store, service+"\x00"+key)
	return nil
}

// ---------------------------------------------------------------------------
// Test fixtures
// ---------------------------------------------------------------------------

// keyringTestToken returns a StoredToken with every field populated, using
// values distinct from the fixture in types_test.go so we catch any
// accidental sharing.
func keyringTestToken() StoredToken {
	return StoredToken{
		AccessToken:  "kr-access-abc123",
		RefreshToken: "kr-refresh-xyz789",
		TokenType:    "Bearer",
		Expiry:       time.Date(2027, 3, 15, 9, 30, 0, 0, time.UTC),
		Scopes:       []string{"repo", "user", "org:read"},
	}
}

// alternateToken returns a second distinct token to guard against
// implementations that hardcode a single return value.
func alternateToken() StoredToken {
	return StoredToken{
		AccessToken:  "alt-access-different",
		RefreshToken: "",
		TokenType:    "MAC",
		Expiry:       time.Date(2028, 12, 25, 0, 0, 0, 0, time.UTC),
		Scopes:       []string{"admin"},
	}
}

// ---------------------------------------------------------------------------
// Test 1: Round-trip — Set then Get returns identical token
// ---------------------------------------------------------------------------

func TestKeyringStore_RoundTrip(t *testing.T) {
	fk := newFakeKeyring()
	store := NewKeyringStore(fk)

	original := keyringTestToken()
	key := "my-toolkit/my-tool"

	err := store.Set(key, original)
	require.NoError(t, err, "Set must not return an error")

	got, err := store.Get(key)
	require.NoError(t, err, "Get must not return an error after Set")
	require.NotNil(t, got, "Get must return a non-nil *StoredToken")

	// Deep structural comparison — catches any field that is lost or mangled.
	if diff := cmp.Diff(original, *got); diff != "" {
		t.Errorf("KeyringStore round-trip mismatch (-original +got):\n%s", diff)
	}
}

func TestKeyringStore_RoundTrip_AllFieldsExact(t *testing.T) {
	fk := newFakeKeyring()
	store := NewKeyringStore(fk)

	original := keyringTestToken()
	key := "my-toolkit/my-tool"

	err := store.Set(key, original)
	require.NoError(t, err)

	got, err := store.Get(key)
	require.NoError(t, err)
	require.NotNil(t, got)

	// Field-by-field assertions to catch individual field omissions.
	assert.Equal(t, original.AccessToken, got.AccessToken, "AccessToken must survive round-trip")
	assert.Equal(t, original.RefreshToken, got.RefreshToken, "RefreshToken must survive round-trip")
	assert.Equal(t, original.TokenType, got.TokenType, "TokenType must survive round-trip")
	assert.True(t, original.Expiry.Equal(got.Expiry),
		"Expiry must survive round-trip: want %v, got %v", original.Expiry, got.Expiry)
	assert.Equal(t, original.Scopes, got.Scopes, "Scopes must survive round-trip")
}

// ---------------------------------------------------------------------------
// Test 2: Service name — must be "toolwright"
// ---------------------------------------------------------------------------

func TestKeyringStore_ServiceName_Set(t *testing.T) {
	fk := newFakeKeyring()
	store := NewKeyringStore(fk)

	_ = store.Set("some/key", keyringTestToken())

	require.Len(t, fk.setCalls, 1, "Set must call keyring.Set exactly once")
	assert.Equal(t, "toolwright", fk.setCalls[0].Service,
		"KeyringStore must use service name 'toolwright', got %q", fk.setCalls[0].Service)
}

func TestKeyringStore_ServiceName_Get(t *testing.T) {
	fk := newFakeKeyring()
	store := NewKeyringStore(fk)

	// Pre-populate so Get does not error.
	_ = store.Set("some/key", keyringTestToken())

	_, _ = store.Get("some/key")

	require.Len(t, fk.getCalls, 1, "Get must call keyring.Get exactly once")
	assert.Equal(t, "toolwright", fk.getCalls[0].Service,
		"KeyringStore.Get must use service name 'toolwright', got %q", fk.getCalls[0].Service)
}

func TestKeyringStore_ServiceName_Delete(t *testing.T) {
	fk := newFakeKeyring()
	store := NewKeyringStore(fk)

	_ = store.Delete("some/key")

	require.Len(t, fk.deleteCalls, 1, "Delete must call keyring.Delete exactly once")
	assert.Equal(t, "toolwright", fk.deleteCalls[0].Service,
		"KeyringStore.Delete must use service name 'toolwright', got %q", fk.deleteCalls[0].Service)
}

// ---------------------------------------------------------------------------
// Test 3: Key passthrough — key is forwarded unchanged
// ---------------------------------------------------------------------------

func TestKeyringStore_KeyPassthrough(t *testing.T) {
	tests := []struct {
		name string
		key  string
	}{
		{name: "toolkit/tool path", key: "my-toolkit/my-tool"},
		{name: "nested path", key: "org/toolkit/tool"},
		{name: "simple name", key: "tool-name"},
		{name: "with special chars", key: "my-toolkit/tool@v2"},
		{name: "unicode key", key: "toolkit/werkzeug-\u00fc"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fk := newFakeKeyring()
			store := NewKeyringStore(fk)

			tok := keyringTestToken()
			_ = store.Set(tc.key, tok)
			_, _ = store.Get(tc.key)
			_ = store.Delete(tc.key)

			require.Len(t, fk.setCalls, 1)
			assert.Equal(t, tc.key, fk.setCalls[0].Key,
				"Set must forward the key unchanged")

			require.Len(t, fk.getCalls, 1)
			assert.Equal(t, tc.key, fk.getCalls[0].Key,
				"Get must forward the key unchanged")

			require.Len(t, fk.deleteCalls, 1)
			assert.Equal(t, tc.key, fk.deleteCalls[0].Key,
				"Delete must forward the key unchanged")
		})
	}
}

// ---------------------------------------------------------------------------
// Test 4: Delete then Get — returns not-found error
// ---------------------------------------------------------------------------

func TestKeyringStore_DeleteThenGet(t *testing.T) {
	fk := newFakeKeyring()
	store := NewKeyringStore(fk)

	key := "my-toolkit/my-tool"
	tok := keyringTestToken()

	// Set the token.
	err := store.Set(key, tok)
	require.NoError(t, err)

	// Confirm it is retrievable.
	got, err := store.Get(key)
	require.NoError(t, err, "Get before Delete must succeed")
	require.NotNil(t, got)

	// Delete it.
	err = store.Delete(key)
	require.NoError(t, err, "Delete must not error for an existing key")

	// Get must now fail.
	got, err = store.Get(key)
	require.Error(t, err, "Get after Delete must return an error")
	assert.Nil(t, got, "Get after Delete must return nil token")
}

// ---------------------------------------------------------------------------
// Test 5: Get non-existent — returns error
// ---------------------------------------------------------------------------

func TestKeyringStore_GetNonExistent(t *testing.T) {
	fk := newFakeKeyring()
	store := NewKeyringStore(fk)

	got, err := store.Get("never-set/key")
	require.Error(t, err, "Get for a never-set key must return an error")
	assert.Nil(t, got, "Get for a never-set key must return nil token")
}

// ---------------------------------------------------------------------------
// Test 6: Serialization is JSON — stored value is valid JSON with expected fields
// ---------------------------------------------------------------------------

func TestKeyringStore_SerializesAsJSON(t *testing.T) {
	fk := newFakeKeyring()
	store := NewKeyringStore(fk)

	tok := keyringTestToken()
	err := store.Set("test/key", tok)
	require.NoError(t, err)

	require.Len(t, fk.setCalls, 1, "Set must call keyring exactly once")
	storedValue := fk.setCalls[0].Value

	// Must be valid JSON.
	assert.True(t, json.Valid([]byte(storedValue)),
		"Value stored in keyring must be valid JSON, got: %s", storedValue)

	// Parse into a raw map to check field names.
	var raw map[string]json.RawMessage
	err = json.Unmarshal([]byte(storedValue), &raw)
	require.NoError(t, err, "Stored JSON must unmarshal into a map")

	// Verify all expected snake_case fields are present.
	expectedKeys := []string{"access_token", "refresh_token", "token_type", "expiry", "scopes"}
	for _, key := range expectedKeys {
		_, exists := raw[key]
		assert.True(t, exists, "Stored JSON must contain field %q, keys present: %v",
			key, mapKeys(raw))
	}

	// Verify PascalCase fields are NOT present (catches missing json tags on StoredToken).
	forbiddenKeys := []string{"AccessToken", "RefreshToken", "TokenType", "Expiry", "Scopes"}
	for _, key := range forbiddenKeys {
		_, exists := raw[key]
		assert.False(t, exists,
			"Stored JSON must NOT contain PascalCase key %q — use json tags", key)
	}
}

func TestKeyringStore_SerializedValues_Match(t *testing.T) {
	fk := newFakeKeyring()
	store := NewKeyringStore(fk)

	tok := keyringTestToken()
	err := store.Set("test/key", tok)
	require.NoError(t, err)

	require.Len(t, fk.setCalls, 1)
	storedValue := fk.setCalls[0].Value

	// Unmarshal the stored JSON back into a StoredToken and verify fields.
	var parsed StoredToken
	err = json.Unmarshal([]byte(storedValue), &parsed)
	require.NoError(t, err, "Stored JSON must unmarshal into StoredToken")

	assert.Equal(t, tok.AccessToken, parsed.AccessToken)
	assert.Equal(t, tok.RefreshToken, parsed.RefreshToken)
	assert.Equal(t, tok.TokenType, parsed.TokenType)
	assert.True(t, tok.Expiry.Equal(parsed.Expiry),
		"Expiry in JSON: want %v, got %v", tok.Expiry, parsed.Expiry)
	assert.Equal(t, tok.Scopes, parsed.Scopes)
}

// ---------------------------------------------------------------------------
// Test 7: Multiple tokens — different keys, no shared state
// ---------------------------------------------------------------------------

func TestKeyringStore_MultipleTokens(t *testing.T) {
	fk := newFakeKeyring()
	store := NewKeyringStore(fk)

	tok1 := keyringTestToken()
	tok2 := alternateToken()

	key1 := "toolkit-a/tool-1"
	key2 := "toolkit-b/tool-2"

	// Set both tokens.
	err := store.Set(key1, tok1)
	require.NoError(t, err)
	err = store.Set(key2, tok2)
	require.NoError(t, err)

	// Get each one back and verify isolation.
	got1, err := store.Get(key1)
	require.NoError(t, err)
	require.NotNil(t, got1)

	got2, err := store.Get(key2)
	require.NoError(t, err)
	require.NotNil(t, got2)

	// Verify tok1 is returned for key1.
	if diff := cmp.Diff(tok1, *got1); diff != "" {
		t.Errorf("Token for key1 mismatch (-want +got):\n%s", diff)
	}

	// Verify tok2 is returned for key2 (not tok1).
	if diff := cmp.Diff(tok2, *got2); diff != "" {
		t.Errorf("Token for key2 mismatch (-want +got):\n%s", diff)
	}

	// Extra: make sure they are actually different from each other.
	assert.NotEqual(t, got1.AccessToken, got2.AccessToken,
		"Tokens under different keys must not be the same (anti-hardcoding)")
}

func TestKeyringStore_MultipleTokens_DeleteOneKeepsOther(t *testing.T) {
	fk := newFakeKeyring()
	store := NewKeyringStore(fk)

	tok1 := keyringTestToken()
	tok2 := alternateToken()

	key1 := "toolkit-a/tool-1"
	key2 := "toolkit-b/tool-2"

	err := store.Set(key1, tok1)
	require.NoError(t, err)
	err = store.Set(key2, tok2)
	require.NoError(t, err)

	// Delete key1.
	err = store.Delete(key1)
	require.NoError(t, err)

	// key1 should be gone.
	got1, err := store.Get(key1)
	require.Error(t, err, "Get for deleted key1 must return error")
	assert.Nil(t, got1)

	// key2 should still exist.
	got2, err := store.Get(key2)
	require.NoError(t, err, "Get for key2 must still succeed after deleting key1")
	require.NotNil(t, got2)
	assert.Equal(t, tok2.AccessToken, got2.AccessToken)
}

// ---------------------------------------------------------------------------
// Test 8: Set overwrites — same key, second token wins
// ---------------------------------------------------------------------------

func TestKeyringStore_SetOverwrites(t *testing.T) {
	fk := newFakeKeyring()
	store := NewKeyringStore(fk)

	key := "same/key"
	tok1 := keyringTestToken()
	tok2 := alternateToken()

	// Set first token.
	err := store.Set(key, tok1)
	require.NoError(t, err)

	// Overwrite with second token.
	err = store.Set(key, tok2)
	require.NoError(t, err)

	// Get must return the second token.
	got, err := store.Get(key)
	require.NoError(t, err)
	require.NotNil(t, got)

	if diff := cmp.Diff(tok2, *got); diff != "" {
		t.Errorf("Set overwrite: expected second token (-want +got):\n%s", diff)
	}

	// Explicitly verify it is NOT the first token.
	assert.NotEqual(t, tok1.AccessToken, got.AccessToken,
		"After overwrite, Get must return the new token, not the original")
	assert.Equal(t, tok2.AccessToken, got.AccessToken,
		"After overwrite, Get must return the second token's AccessToken")
	assert.Equal(t, tok2.TokenType, got.TokenType,
		"After overwrite, Get must return the second token's TokenType")
}

// ---------------------------------------------------------------------------
// Test 9: Empty key — should work or return a clear error
// ---------------------------------------------------------------------------

func TestKeyringStore_EmptyKey(t *testing.T) {
	fk := newFakeKeyring()
	store := NewKeyringStore(fk)

	tok := keyringTestToken()

	// Attempt to Set with empty key. The implementation may either succeed
	// (treating "" as a valid key) or return a clear error. It must not panic.
	err := store.Set("", tok)

	if err != nil {
		// If it returns an error, that is acceptable. But the error message
		// must not be empty or unhelpful.
		assert.NotEmpty(t, err.Error(), "Error for empty key should have a message")
		return
	}

	// If Set succeeded, Get with the same empty key must return the token.
	got, err := store.Get("")
	require.NoError(t, err, "If Set('') succeeded, Get('') must also succeed")
	require.NotNil(t, got)
	assert.Equal(t, tok.AccessToken, got.AccessToken)
}

// ---------------------------------------------------------------------------
// Test 10: Error propagation — keyring errors are wrapped and propagated
// ---------------------------------------------------------------------------

func TestKeyringStore_ErrorPropagation_Set(t *testing.T) {
	injectedErr := errors.New("keyring locked: authentication required")
	fk := newFakeKeyring()
	fk.setErr = injectedErr
	store := NewKeyringStore(fk)

	err := store.Set("any/key", keyringTestToken())
	require.Error(t, err, "Set must propagate keyring errors")

	// The error must wrap the original so errors.Is works.
	assert.True(t, errors.Is(err, injectedErr),
		"Set error must wrap the original keyring error; got: %v", err)
}

func TestKeyringStore_ErrorPropagation_Get(t *testing.T) {
	injectedErr := errors.New("keyring daemon unavailable")
	fk := newFakeKeyring()
	fk.getErr = injectedErr
	store := NewKeyringStore(fk)

	got, err := store.Get("any/key")
	require.Error(t, err, "Get must propagate keyring errors")
	assert.Nil(t, got, "Get must return nil when keyring errors")

	assert.True(t, errors.Is(err, injectedErr),
		"Get error must wrap the original keyring error; got: %v", err)
}

func TestKeyringStore_ErrorPropagation_Delete(t *testing.T) {
	injectedErr := errors.New("permission denied")
	fk := newFakeKeyring()
	fk.deleteErr = injectedErr
	store := NewKeyringStore(fk)

	err := store.Delete("any/key")
	require.Error(t, err, "Delete must propagate keyring errors")

	assert.True(t, errors.Is(err, injectedErr),
		"Delete error must wrap the original keyring error; got: %v", err)
}

// ---------------------------------------------------------------------------
// Additional edge cases
// ---------------------------------------------------------------------------

func TestKeyringStore_TokenWithZeroExpiry(t *testing.T) {
	// Tokens that never expire have a zero-value Expiry. Must round-trip.
	fk := newFakeKeyring()
	store := NewKeyringStore(fk)

	tok := StoredToken{
		AccessToken:  "pat-never-expires",
		RefreshToken: "",
		TokenType:    "Bearer",
		Expiry:       time.Time{}, // zero
		Scopes:       nil,
	}

	err := store.Set("never-expires/key", tok)
	require.NoError(t, err)

	got, err := store.Get("never-expires/key")
	require.NoError(t, err)
	require.NotNil(t, got)

	assert.Equal(t, "pat-never-expires", got.AccessToken)
	assert.Empty(t, got.RefreshToken)
	assert.Equal(t, "Bearer", got.TokenType)
	assert.True(t, got.Expiry.IsZero(),
		"Zero Expiry must survive round-trip, got: %v", got.Expiry)
}

func TestKeyringStore_TokenWithEmptyScopes(t *testing.T) {
	fk := newFakeKeyring()
	store := NewKeyringStore(fk)

	tok := StoredToken{
		AccessToken:  "scoped-token",
		RefreshToken: "refresh",
		TokenType:    "Bearer",
		Expiry:       time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC),
		Scopes:       []string{},
	}

	err := store.Set("empty-scopes/key", tok)
	require.NoError(t, err)

	got, err := store.Get("empty-scopes/key")
	require.NoError(t, err)
	require.NotNil(t, got)

	assert.Equal(t, "scoped-token", got.AccessToken)
	// Scopes may be nil or empty after round-trip — both are acceptable.
	// But the other fields must match exactly.
	assert.Equal(t, "refresh", got.RefreshToken)
	assert.Equal(t, "Bearer", got.TokenType)
}

func TestKeyringStore_TokenWithLargeScopes(t *testing.T) {
	fk := newFakeKeyring()
	store := NewKeyringStore(fk)

	// Create a token with many scopes to verify slices survive serialization.
	scopes := make([]string, 20)
	for i := range scopes {
		scopes[i] = fmt.Sprintf("scope-%d", i)
	}

	tok := StoredToken{
		AccessToken:  "many-scopes",
		RefreshToken: "refresh",
		TokenType:    "Bearer",
		Expiry:       time.Date(2027, 6, 1, 0, 0, 0, 0, time.UTC),
		Scopes:       scopes,
	}

	err := store.Set("big-scopes/key", tok)
	require.NoError(t, err)

	got, err := store.Get("big-scopes/key")
	require.NoError(t, err)
	require.NotNil(t, got)

	require.Len(t, got.Scopes, 20, "All 20 scopes must survive round-trip")
	for i, s := range got.Scopes {
		expected := fmt.Sprintf("scope-%d", i)
		assert.Equal(t, expected, s, "Scope at index %d", i)
	}
}

func TestKeyringStore_TokenWithSpecialCharacters(t *testing.T) {
	fk := newFakeKeyring()
	store := NewKeyringStore(fk)

	// Tokens often contain Base64 or JWT characters. Verify no escaping issues.
	tok := StoredToken{
		AccessToken:  "eyJhbGciOiJSUzI1NiJ9.eyJzdWIiOiJ1c2VyMTIzIn0.signature+base64/chars==",
		RefreshToken: "refresh/with+special=chars&more",
		TokenType:    "Bearer",
		Expiry:       time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC),
		Scopes:       []string{"scope:with:colons", "scope/with/slashes"},
	}

	err := store.Set("special-chars/key", tok)
	require.NoError(t, err)

	got, err := store.Get("special-chars/key")
	require.NoError(t, err)
	require.NotNil(t, got)

	assert.Equal(t, tok.AccessToken, got.AccessToken,
		"AccessToken with special characters must survive round-trip")
	assert.Equal(t, tok.RefreshToken, got.RefreshToken,
		"RefreshToken with special characters must survive round-trip")
	assert.Equal(t, tok.Scopes, got.Scopes,
		"Scopes with special characters must survive round-trip")
}

func TestKeyringStore_GetReturnsPointer(t *testing.T) {
	// Per the plan: Get returns *StoredToken, not StoredToken.
	// Verify that modifying the returned pointer does not affect
	// subsequent Get calls (i.e., it is a fresh deserialization each time).
	fk := newFakeKeyring()
	store := NewKeyringStore(fk)

	tok := keyringTestToken()
	err := store.Set("pointer/test", tok)
	require.NoError(t, err)

	got1, err := store.Get("pointer/test")
	require.NoError(t, err)
	require.NotNil(t, got1)

	// Mutate the returned token.
	got1.AccessToken = "MUTATED"
	got1.Scopes = append(got1.Scopes, "injected")

	// A second Get should return the original, not the mutated copy.
	got2, err := store.Get("pointer/test")
	require.NoError(t, err)
	require.NotNil(t, got2)

	assert.Equal(t, tok.AccessToken, got2.AccessToken,
		"Mutating a returned *StoredToken must not affect subsequent Get calls")
	assert.Equal(t, tok.Scopes, got2.Scopes,
		"Mutating returned Scopes must not affect subsequent Get calls")
}

func TestKeyringStore_ServiceNameIsExactlyToolwright(t *testing.T) {
	// Guard against common mistakes: "Toolwright", "TOOLWRIGHT", "toolwright-auth", etc.
	fk := newFakeKeyring()
	store := NewKeyringStore(fk)

	_ = store.Set("test/key", keyringTestToken())
	_, _ = store.Get("test/key")
	_ = store.Delete("test/key")

	allCalls := make([]string, 0)
	for _, c := range fk.setCalls {
		allCalls = append(allCalls, c.Service)
	}
	for _, c := range fk.getCalls {
		allCalls = append(allCalls, c.Service)
	}
	for _, c := range fk.deleteCalls {
		allCalls = append(allCalls, c.Service)
	}

	for i, svc := range allCalls {
		assert.Equal(t, "toolwright", svc,
			"Call %d: service must be exactly 'toolwright' (lowercase, no prefix/suffix), got %q", i, svc)
		// Explicitly check it is not one of common wrong values.
		assert.NotEqual(t, "Toolwright", svc)
		assert.NotEqual(t, "TOOLWRIGHT", svc)
		assert.False(t, strings.Contains(svc, "-"), "Service name should not contain hyphens: %q", svc)
		assert.False(t, strings.Contains(svc, "/"), "Service name should not contain slashes: %q", svc)
	}
}

func TestKeyringStore_ErrorPropagation_HasContext(t *testing.T) {
	// Constitution rule 4: errors are wrapped with context.
	// The error from KeyringStore should add context beyond the raw keyring error.
	tests := []struct {
		name      string
		operation string
		setup     func(fk *fakeKeyring)
		call      func(store *KeyringStore) error
	}{
		{
			name:      "Set wraps with context",
			operation: "Set",
			setup: func(fk *fakeKeyring) {
				fk.setErr = errors.New("raw keyring error")
			},
			call: func(store *KeyringStore) error {
				return store.Set("key", keyringTestToken())
			},
		},
		{
			name:      "Get wraps with context",
			operation: "Get",
			setup: func(fk *fakeKeyring) {
				fk.getErr = errors.New("raw keyring error")
			},
			call: func(store *KeyringStore) error {
				_, err := store.Get("key")
				return err
			},
		},
		{
			name:      "Delete wraps with context",
			operation: "Delete",
			setup: func(fk *fakeKeyring) {
				fk.deleteErr = errors.New("raw keyring error")
			},
			call: func(store *KeyringStore) error {
				return store.Delete("key")
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fk := newFakeKeyring()
			tc.setup(fk)
			store := NewKeyringStore(fk)

			err := tc.call(store)
			require.Error(t, err)

			// The wrapped error should contain more context than just
			// "raw keyring error". It should mention what operation or
			// component was involved.
			errMsg := err.Error()
			assert.Contains(t, errMsg, "raw keyring error",
				"Wrapped error must still contain the original message")
			assert.True(t, len(errMsg) > len("raw keyring error"),
				"Error should have additional context beyond the raw message, got: %q", errMsg)
		})
	}
}

func TestKeyringStore_DeleteNonExistent(t *testing.T) {
	// Deleting a key that was never set. The behavior depends on the keyring
	// implementation — it may return an error or succeed silently.
	// Either way, it must not panic.
	fk := newFakeKeyring()
	store := NewKeyringStore(fk)

	// This should not panic. Whether it errors is implementation-defined.
	_ = store.Delete("never-set/key")
}

// ---------------------------------------------------------------------------
// Anti-hardcoding: table-driven round-trip with multiple distinct tokens
// ---------------------------------------------------------------------------

func TestKeyringStore_RoundTrip_TableDriven(t *testing.T) {
	tests := []struct {
		name string
		key  string
		tok  StoredToken
	}{
		{
			name: "standard token with all fields",
			key:  "org/tool-a",
			tok: StoredToken{
				AccessToken:  "access-aaa",
				RefreshToken: "refresh-aaa",
				TokenType:    "Bearer",
				Expiry:       time.Date(2027, 6, 1, 12, 0, 0, 0, time.UTC),
				Scopes:       []string{"read", "write"},
			},
		},
		{
			name: "token with no refresh token",
			key:  "org/tool-b",
			tok: StoredToken{
				AccessToken:  "access-bbb",
				RefreshToken: "",
				TokenType:    "Bearer",
				Expiry:       time.Date(2028, 1, 1, 0, 0, 0, 0, time.UTC),
				Scopes:       []string{"admin"},
			},
		},
		{
			name: "token with MAC type",
			key:  "different/tool",
			tok: StoredToken{
				AccessToken:  "mac-access-ccc",
				RefreshToken: "mac-refresh",
				TokenType:    "MAC",
				Expiry:       time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC),
				Scopes:       []string{"scope-one", "scope-two", "scope-three"},
			},
		},
		{
			name: "token with zero expiry (never expires)",
			key:  "pat/key",
			tok: StoredToken{
				AccessToken:  "pat-token",
				RefreshToken: "",
				TokenType:    "Bearer",
				Expiry:       time.Time{},
				Scopes:       nil,
			},
		},
		{
			name: "token with single scope",
			key:  "single-scope/t",
			tok: StoredToken{
				AccessToken:  "single-scope-tok",
				RefreshToken: "single-scope-ref",
				TokenType:    "Bearer",
				Expiry:       time.Date(2027, 3, 1, 0, 0, 0, 0, time.UTC),
				Scopes:       []string{"only-one"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fk := newFakeKeyring()
			store := NewKeyringStore(fk)

			err := store.Set(tc.key, tc.tok)
			require.NoError(t, err, "Set must succeed")

			got, err := store.Get(tc.key)
			require.NoError(t, err, "Get must succeed after Set")
			require.NotNil(t, got, "Get must return non-nil token")

			// Compare core fields that must always survive.
			assert.Equal(t, tc.tok.AccessToken, got.AccessToken, "AccessToken")
			assert.Equal(t, tc.tok.RefreshToken, got.RefreshToken, "RefreshToken")
			assert.Equal(t, tc.tok.TokenType, got.TokenType, "TokenType")
			assert.True(t, tc.tok.Expiry.Equal(got.Expiry),
				"Expiry: want %v, got %v", tc.tok.Expiry, got.Expiry)

			// Scopes: nil and empty slice are both acceptable for nil input.
			if tc.tok.Scopes != nil {
				assert.Equal(t, tc.tok.Scopes, got.Scopes, "Scopes")
			}
		})
	}
}

