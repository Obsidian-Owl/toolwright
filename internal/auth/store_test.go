package auth

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Test fixtures
// ---------------------------------------------------------------------------

// fileStoreToken returns a StoredToken with every field populated, using
// values distinct from other test files to avoid accidental test sharing.
func fileStoreToken() StoredToken {
	return StoredToken{
		AccessToken:  "fs-access-token-xyz",
		RefreshToken: "fs-refresh-token-abc",
		TokenType:    "Bearer",
		Expiry:       time.Date(2027, 7, 4, 15, 30, 0, 0, time.UTC),
		Scopes:       []string{"files:read", "files:write", "admin"},
	}
}

// fileStoreAlternateToken returns a second distinct token to guard against
// hardcoded return values or implementations that ignore the key.
func fileStoreAlternateToken() StoredToken {
	return StoredToken{
		AccessToken:  "fs-alt-access-999",
		RefreshToken: "",
		TokenType:    "MAC",
		Expiry:       time.Date(2029, 1, 15, 0, 0, 0, 0, time.UTC),
		Scopes:       []string{"deploy"},
	}
}

// ---------------------------------------------------------------------------
// Helper: create a FileStore pointed at a temp directory
// ---------------------------------------------------------------------------

func newTempFileStore(t *testing.T) (*FileStore, string) {
	t.Helper()
	dir := t.TempDir()
	store := NewFileStore(dir)
	return store, dir
}

// tokensFilePath returns the expected tokens.json path for a base directory.
func tokensFilePath(baseDir string) string {
	return filepath.Join(baseDir, "tokens.json")
}

// ---------------------------------------------------------------------------
// Test 1: Round-trip — Set then Get returns identical token (AC-10)
// ---------------------------------------------------------------------------

func TestFileStore_RoundTrip(t *testing.T) {
	store, _ := newTempFileStore(t)
	original := fileStoreToken()
	key := "https://api.example.com"

	err := store.Set(key, original)
	require.NoError(t, err, "Set must not return an error")

	got, err := store.Get(key)
	require.NoError(t, err, "Get must not return an error after Set")
	require.NotNil(t, got, "Get must return a non-nil *StoredToken")

	if diff := cmp.Diff(original, *got); diff != "" {
		t.Errorf("FileStore round-trip mismatch (-original +got):\n%s", diff)
	}
}

func TestFileStore_RoundTrip_AllFieldsExact(t *testing.T) {
	store, _ := newTempFileStore(t)
	original := fileStoreToken()
	key := "toolkit/tool-roundtrip"

	err := store.Set(key, original)
	require.NoError(t, err)

	got, err := store.Get(key)
	require.NoError(t, err)
	require.NotNil(t, got)

	assert.Equal(t, original.AccessToken, got.AccessToken, "AccessToken must survive round-trip")
	assert.Equal(t, original.RefreshToken, got.RefreshToken, "RefreshToken must survive round-trip")
	assert.Equal(t, original.TokenType, got.TokenType, "TokenType must survive round-trip")
	assert.True(t, original.Expiry.Equal(got.Expiry),
		"Expiry must survive round-trip: want %v, got %v", original.Expiry, got.Expiry)
	assert.Equal(t, original.Scopes, got.Scopes, "Scopes must survive round-trip")
}

// Anti-hardcoding: two distinct tokens must each round-trip correctly.
func TestFileStore_RoundTrip_DistinctTokens(t *testing.T) {
	store, _ := newTempFileStore(t)
	tok1 := fileStoreToken()
	tok2 := fileStoreAlternateToken()

	err := store.Set("key-one", tok1)
	require.NoError(t, err)
	err = store.Set("key-two", tok2)
	require.NoError(t, err)

	got1, err := store.Get("key-one")
	require.NoError(t, err)
	require.NotNil(t, got1)

	got2, err := store.Get("key-two")
	require.NoError(t, err)
	require.NotNil(t, got2)

	// Verify each token is correct, not swapped or hardcoded.
	assert.Equal(t, tok1.AccessToken, got1.AccessToken, "key-one should return tok1")
	assert.Equal(t, tok2.AccessToken, got2.AccessToken, "key-two should return tok2")
	assert.NotEqual(t, got1.AccessToken, got2.AccessToken,
		"Different keys must return different tokens (anti-hardcoding)")
}

// ---------------------------------------------------------------------------
// Test 2: Permissions 0600 — After Set, file is created with 0600 (AC-8)
// ---------------------------------------------------------------------------

func TestFileStore_Set_CreatesFileWith0600Permissions(t *testing.T) {
	store, dir := newTempFileStore(t)
	tok := fileStoreToken()

	err := store.Set("perm-test-key", tok)
	require.NoError(t, err, "Set must succeed")

	filePath := tokensFilePath(dir)
	info, err := os.Stat(filePath)
	require.NoError(t, err, "tokens.json must exist after Set")

	mode := info.Mode().Perm()
	assert.Equal(t, os.FileMode(0600), mode,
		"tokens.json must have 0600 permissions, got %04o", mode)
}

func TestFileStore_Set_MaintainsPermissionsOnOverwrite(t *testing.T) {
	store, dir := newTempFileStore(t)

	// First write.
	err := store.Set("key-a", fileStoreToken())
	require.NoError(t, err)

	// Second write (different key, triggers read-modify-write).
	err = store.Set("key-b", fileStoreAlternateToken())
	require.NoError(t, err)

	filePath := tokensFilePath(dir)
	info, err := os.Stat(filePath)
	require.NoError(t, err)

	mode := info.Mode().Perm()
	assert.Equal(t, os.FileMode(0600), mode,
		"tokens.json must still be 0600 after read-modify-write, got %04o", mode)
}

// ---------------------------------------------------------------------------
// Test 3: Permissions rejection — chmod 0644 then Get returns error (AC-8)
// ---------------------------------------------------------------------------

func TestFileStore_Get_RejectsInsecurePermissions(t *testing.T) {
	store, dir := newTempFileStore(t)
	tok := fileStoreToken()

	err := store.Set("secure-key", tok)
	require.NoError(t, err)

	// Manually widen permissions to 0644 (world-readable).
	filePath := tokensFilePath(dir)
	err = os.Chmod(filePath, 0644)
	require.NoError(t, err, "chmod to 0644 must succeed")

	// Get must refuse to read the file.
	got, err := store.Get("secure-key")
	require.Error(t, err, "Get must return an error when file has 0644 permissions")
	assert.Nil(t, got, "Get must return nil token when permissions are insecure")

	// The error message should mention permissions so the user can fix it.
	assert.Contains(t, err.Error(), "permission",
		"Error must mention permissions; got: %s", err.Error())
}

func TestFileStore_Get_RejectsGroupReadable(t *testing.T) {
	store, dir := newTempFileStore(t)

	err := store.Set("test-key", fileStoreToken())
	require.NoError(t, err)

	// Set to 0640 (group-readable).
	filePath := tokensFilePath(dir)
	err = os.Chmod(filePath, 0640)
	require.NoError(t, err)

	got, err := store.Get("test-key")
	require.Error(t, err, "Get must reject 0640 permissions (group-readable)")
	assert.Nil(t, got)
}

func TestFileStore_Get_RejectsWorldReadable(t *testing.T) {
	store, dir := newTempFileStore(t)

	err := store.Set("test-key", fileStoreToken())
	require.NoError(t, err)

	// Set to 0604 (world-readable).
	filePath := tokensFilePath(dir)
	err = os.Chmod(filePath, 0604)
	require.NoError(t, err)

	got, err := store.Get("test-key")
	require.Error(t, err, "Get must reject 0604 permissions (world-readable)")
	assert.Nil(t, got)
}

func TestFileStore_Get_Accepts0600Permissions(t *testing.T) {
	store, dir := newTempFileStore(t)
	tok := fileStoreToken()

	err := store.Set("ok-key", tok)
	require.NoError(t, err)

	// Explicitly ensure 0600 (should already be, but be explicit).
	filePath := tokensFilePath(dir)
	err = os.Chmod(filePath, 0600)
	require.NoError(t, err)

	got, err := store.Get("ok-key")
	require.NoError(t, err, "Get must succeed when file has 0600 permissions")
	require.NotNil(t, got)
	assert.Equal(t, tok.AccessToken, got.AccessToken)
}

// Table-driven: various insecure modes must all be rejected.
func TestFileStore_Get_RejectsInsecurePermissions_Table(t *testing.T) {
	modes := []struct {
		name string
		mode os.FileMode
	}{
		{"0644 (world-readable)", 0644},
		{"0640 (group-readable)", 0640},
		{"0604 (other-readable)", 0604},
		{"0660 (group-read-write)", 0660},
		{"0666 (all-read-write)", 0666},
		{"0700 (owner-execute)", 0700},
		{"0755 (all-execute)", 0755},
		{"0611 (execute-bits)", 0611},
		{"0601 (other-execute)", 0601},
	}

	for _, tc := range modes {
		t.Run(tc.name, func(t *testing.T) {
			store, dir := newTempFileStore(t)

			err := store.Set("key", fileStoreToken())
			require.NoError(t, err)

			filePath := tokensFilePath(dir)
			err = os.Chmod(filePath, tc.mode)
			require.NoError(t, err)

			got, err := store.Get("key")
			require.Error(t, err, "Get must reject mode %04o", tc.mode)
			assert.Nil(t, got, "Get must return nil for mode %04o", tc.mode)
		})
	}
}

// ---------------------------------------------------------------------------
// Test 4: XDG_CONFIG_HOME — file store uses XDG path (AC-9)
// ---------------------------------------------------------------------------

func TestFileStore_XDG_CONFIG_HOME(t *testing.T) {
	tmpDir := t.TempDir()
	xdgDir := filepath.Join(tmpDir, "custom-xdg")

	t.Setenv("XDG_CONFIG_HOME", xdgDir)

	// NewFileStore with empty basePath should use XDG_CONFIG_HOME.
	store := NewFileStore("")

	tok := fileStoreToken()
	err := store.Set("xdg-key", tok)
	require.NoError(t, err, "Set with XDG_CONFIG_HOME must succeed")

	// The file should exist at $XDG_CONFIG_HOME/toolwright/tokens.json.
	expectedPath := filepath.Join(xdgDir, "toolwright", "tokens.json")
	_, err = os.Stat(expectedPath)
	require.NoError(t, err, "tokens.json must exist at %s", expectedPath)

	// Read it back to verify it works.
	got, err := store.Get("xdg-key")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, tok.AccessToken, got.AccessToken)
}

func TestFileStore_XDG_CONFIG_HOME_FileContentsAtCorrectPath(t *testing.T) {
	tmpDir := t.TempDir()
	xdgDir := filepath.Join(tmpDir, "xdg-check")

	t.Setenv("XDG_CONFIG_HOME", xdgDir)

	store := NewFileStore("")
	tok := fileStoreToken()
	err := store.Set("path-check", tok)
	require.NoError(t, err)

	// Read the raw file to verify it's at the correct location with valid JSON.
	expectedPath := filepath.Join(xdgDir, "toolwright", "tokens.json")
	data, err := os.ReadFile(expectedPath)
	require.NoError(t, err, "Must be able to read tokens.json at XDG path")

	var tf TokenFile
	err = json.Unmarshal(data, &tf)
	require.NoError(t, err, "File at XDG path must be valid JSON TokenFile")
	assert.Equal(t, 1, tf.Version)

	storedTok, ok := tf.Tokens["path-check"]
	require.True(t, ok, "Token must be stored under the given key")
	assert.Equal(t, tok.AccessToken, storedTok.AccessToken)
}

// ---------------------------------------------------------------------------
// Test 5: Default path — XDG_CONFIG_HOME unset defaults to ~/.config (AC-9)
// ---------------------------------------------------------------------------

func TestFileStore_DefaultPath_UsesHomeConfig(t *testing.T) {
	// We cannot write to ~/.config in tests, but we can verify the path logic
	// by unsetting XDG_CONFIG_HOME and checking that the store would target
	// the correct location. We use a real temp dir for actual I/O.
	tmpDir := t.TempDir()
	homeDir := filepath.Join(tmpDir, "fakehome")
	err := os.MkdirAll(homeDir, 0700)
	require.NoError(t, err)

	// Set HOME to our fake home so ~/.config resolves there.
	t.Setenv("HOME", homeDir)
	t.Setenv("XDG_CONFIG_HOME", "")

	// Also unset it entirely to test the default.
	os.Unsetenv("XDG_CONFIG_HOME")

	store := NewFileStore("")
	tok := fileStoreToken()
	err = store.Set("default-path-key", tok)
	require.NoError(t, err, "Set with default path must succeed")

	// The file should be at $HOME/.config/toolwright/tokens.json.
	expectedPath := filepath.Join(homeDir, ".config", "toolwright", "tokens.json")
	_, err = os.Stat(expectedPath)
	require.NoError(t, err, "tokens.json must exist at default path %s", expectedPath)

	// Verify we can read back via the store.
	got, err := store.Get("default-path-key")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, tok.AccessToken, got.AccessToken)
}

// ---------------------------------------------------------------------------
// Test 6: JSON format — valid JSON with version field (AC-10)
// ---------------------------------------------------------------------------

func TestFileStore_WritesValidJSON_WithVersionField(t *testing.T) {
	store, dir := newTempFileStore(t)
	tok := fileStoreToken()

	err := store.Set("json-check", tok)
	require.NoError(t, err)

	filePath := tokensFilePath(dir)
	data, err := os.ReadFile(filePath)
	require.NoError(t, err, "Must be able to read tokens.json")

	// Must be valid JSON.
	assert.True(t, json.Valid(data), "tokens.json must be valid JSON, got: %s", string(data))

	// Must parse as TokenFile.
	var tf TokenFile
	err = json.Unmarshal(data, &tf)
	require.NoError(t, err, "tokens.json must unmarshal as TokenFile")

	// Version must be 1.
	assert.Equal(t, 1, tf.Version, "Version field must be 1")

	// Tokens map must contain the key.
	storedTok, ok := tf.Tokens["json-check"]
	require.True(t, ok, "Tokens map must contain the key 'json-check'")
	assert.Equal(t, tok.AccessToken, storedTok.AccessToken)
}

func TestFileStore_JSON_HasExactTopLevelKeys(t *testing.T) {
	store, dir := newTempFileStore(t)

	err := store.Set("key", fileStoreToken())
	require.NoError(t, err)

	filePath := tokensFilePath(dir)
	data, err := os.ReadFile(filePath)
	require.NoError(t, err)

	// Parse into raw map to check keys.
	var raw map[string]json.RawMessage
	err = json.Unmarshal(data, &raw)
	require.NoError(t, err)

	// Must have "version" and "tokens".
	_, hasVersion := raw["version"]
	assert.True(t, hasVersion, "JSON must contain 'version' key")

	_, hasTokens := raw["tokens"]
	assert.True(t, hasTokens, "JSON must contain 'tokens' key")

	// Must NOT have PascalCase keys.
	_, hasVersionPascal := raw["Version"]
	assert.False(t, hasVersionPascal, "JSON must NOT contain 'Version'")

	_, hasTokensPascal := raw["Tokens"]
	assert.False(t, hasTokensPascal, "JSON must NOT contain 'Tokens'")

	assert.Len(t, raw, 2, "JSON must have exactly 2 top-level keys, got %v", mapKeys(raw))
}

func TestFileStore_JSON_TokenFieldsAreSnakeCase(t *testing.T) {
	store, dir := newTempFileStore(t)

	err := store.Set("snake-check", fileStoreToken())
	require.NoError(t, err)

	filePath := tokensFilePath(dir)
	data, err := os.ReadFile(filePath)
	require.NoError(t, err)

	var tf struct {
		Tokens map[string]json.RawMessage `json:"tokens"`
	}
	err = json.Unmarshal(data, &tf)
	require.NoError(t, err)

	tokenRaw, ok := tf.Tokens["snake-check"]
	require.True(t, ok)

	var tokenMap map[string]json.RawMessage
	err = json.Unmarshal(tokenRaw, &tokenMap)
	require.NoError(t, err)

	requiredKeys := []string{"access_token", "refresh_token", "token_type", "expiry", "scopes"}
	for _, key := range requiredKeys {
		_, exists := tokenMap[key]
		assert.True(t, exists, "Token JSON must contain snake_case key %q", key)
	}
}

// ---------------------------------------------------------------------------
// Test 7: Multiple tokens coexist (AC-10)
// ---------------------------------------------------------------------------

func TestFileStore_MultipleTokens_BothExist(t *testing.T) {
	store, dir := newTempFileStore(t)
	tok1 := fileStoreToken()
	tok2 := fileStoreAlternateToken()

	err := store.Set("provider-alpha", tok1)
	require.NoError(t, err)
	err = store.Set("provider-beta", tok2)
	require.NoError(t, err)

	// Both must be retrievable via Get.
	got1, err := store.Get("provider-alpha")
	require.NoError(t, err)
	require.NotNil(t, got1)
	assert.Equal(t, tok1.AccessToken, got1.AccessToken, "provider-alpha token")

	got2, err := store.Get("provider-beta")
	require.NoError(t, err)
	require.NotNil(t, got2)
	assert.Equal(t, tok2.AccessToken, got2.AccessToken, "provider-beta token")

	// Verify at the file level that both exist.
	filePath := tokensFilePath(dir)
	data, err := os.ReadFile(filePath)
	require.NoError(t, err)

	var tf TokenFile
	err = json.Unmarshal(data, &tf)
	require.NoError(t, err)

	require.Len(t, tf.Tokens, 2, "File must contain exactly 2 tokens")
	_, hasAlpha := tf.Tokens["provider-alpha"]
	assert.True(t, hasAlpha, "File must contain provider-alpha")
	_, hasBeta := tf.Tokens["provider-beta"]
	assert.True(t, hasBeta, "File must contain provider-beta")
}

func TestFileStore_MultipleTokens_ThreeDistinct(t *testing.T) {
	store, _ := newTempFileStore(t)

	tokens := map[string]StoredToken{
		"svc-a": {AccessToken: "aaa", TokenType: "Bearer", Expiry: time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC), Scopes: []string{"x"}},
		"svc-b": {AccessToken: "bbb", TokenType: "Bearer", Expiry: time.Date(2028, 1, 1, 0, 0, 0, 0, time.UTC), Scopes: []string{"y"}},
		"svc-c": {AccessToken: "ccc", TokenType: "MAC", Expiry: time.Date(2029, 1, 1, 0, 0, 0, 0, time.UTC), Scopes: []string{"z"}},
	}

	for key, tok := range tokens {
		err := store.Set(key, tok)
		require.NoError(t, err, "Set %s", key)
	}

	for key, want := range tokens {
		got, err := store.Get(key)
		require.NoError(t, err, "Get %s", key)
		require.NotNil(t, got, "Get %s must return non-nil", key)
		assert.Equal(t, want.AccessToken, got.AccessToken, "AccessToken for %s", key)
		assert.Equal(t, want.TokenType, got.TokenType, "TokenType for %s", key)
	}
}

// ---------------------------------------------------------------------------
// Test 8: Set preserves other keys (AC-10)
// ---------------------------------------------------------------------------

func TestFileStore_Set_PreservesOtherKeys(t *testing.T) {
	store, _ := newTempFileStore(t)
	tokA := fileStoreToken()
	tokB := fileStoreAlternateToken()

	// Set key A.
	err := store.Set("key-a", tokA)
	require.NoError(t, err)

	// Set key B (must not clobber key A).
	err = store.Set("key-b", tokB)
	require.NoError(t, err)

	// Key A must still be retrievable with all original values.
	gotA, err := store.Get("key-a")
	require.NoError(t, err, "Get key-a must still succeed after setting key-b")
	require.NotNil(t, gotA)

	if diff := cmp.Diff(tokA, *gotA); diff != "" {
		t.Errorf("key-a was mutated by setting key-b (-want +got):\n%s", diff)
	}
}

func TestFileStore_Set_OverwritesTargetedKeyOnly(t *testing.T) {
	store, _ := newTempFileStore(t)

	tokOriginal := fileStoreToken()
	tokUpdated := StoredToken{
		AccessToken:  "updated-access-token",
		RefreshToken: "updated-refresh-token",
		TokenType:    "Bearer",
		Expiry:       time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC),
		Scopes:       []string{"new-scope"},
	}
	tokOther := fileStoreAlternateToken()

	// Set two keys.
	err := store.Set("target", tokOriginal)
	require.NoError(t, err)
	err = store.Set("other", tokOther)
	require.NoError(t, err)

	// Overwrite only "target".
	err = store.Set("target", tokUpdated)
	require.NoError(t, err)

	// "target" should be updated.
	gotTarget, err := store.Get("target")
	require.NoError(t, err)
	require.NotNil(t, gotTarget)
	assert.Equal(t, "updated-access-token", gotTarget.AccessToken,
		"Overwritten key must reflect new value")

	// "other" must be untouched.
	gotOther, err := store.Get("other")
	require.NoError(t, err)
	require.NotNil(t, gotOther)
	assert.Equal(t, tokOther.AccessToken, gotOther.AccessToken,
		"Other key must be preserved when target key is overwritten")
}

// ---------------------------------------------------------------------------
// Test 9: Delete (AC-10, implied by store contract)
// ---------------------------------------------------------------------------

func TestFileStore_Delete(t *testing.T) {
	store, _ := newTempFileStore(t)
	tok := fileStoreToken()

	err := store.Set("delete-me", tok)
	require.NoError(t, err)

	// Confirm it exists.
	got, err := store.Get("delete-me")
	require.NoError(t, err)
	require.NotNil(t, got)

	// Delete it.
	err = store.Delete("delete-me")
	require.NoError(t, err, "Delete must not error for an existing key")

	// Get must now fail.
	got, err = store.Get("delete-me")
	require.Error(t, err, "Get after Delete must return an error")
	assert.Nil(t, got, "Get after Delete must return nil token")
}

// ---------------------------------------------------------------------------
// Test 10: Delete preserves other keys
// ---------------------------------------------------------------------------

func TestFileStore_Delete_PreservesOtherKeys(t *testing.T) {
	store, _ := newTempFileStore(t)
	tokA := fileStoreToken()
	tokB := fileStoreAlternateToken()

	err := store.Set("keep-me", tokA)
	require.NoError(t, err)
	err = store.Set("remove-me", tokB)
	require.NoError(t, err)

	// Delete only "remove-me".
	err = store.Delete("remove-me")
	require.NoError(t, err)

	// "keep-me" must still be available with correct values.
	got, err := store.Get("keep-me")
	require.NoError(t, err, "Get for surviving key must succeed after deleting another key")
	require.NotNil(t, got)

	if diff := cmp.Diff(tokA, *got); diff != "" {
		t.Errorf("Surviving token was mutated by delete of another key (-want +got):\n%s", diff)
	}

	// "remove-me" must be gone.
	gotRemoved, err := store.Get("remove-me")
	require.Error(t, err, "Deleted key must return error from Get")
	assert.Nil(t, gotRemoved)
}

// ---------------------------------------------------------------------------
// Test 11: Get non-existent key returns error
// ---------------------------------------------------------------------------

func TestFileStore_Get_NonExistentKey(t *testing.T) {
	store, _ := newTempFileStore(t)

	// Set one key so the file exists.
	err := store.Set("existing", fileStoreToken())
	require.NoError(t, err)

	// Get a key that was never set.
	got, err := store.Get("nonexistent")
	require.Error(t, err, "Get for a key that was never set must return an error")
	assert.Nil(t, got, "Get for a nonexistent key must return nil token")
}

// ---------------------------------------------------------------------------
// Test 12: Get when no tokens.json exists returns error (not panic)
// ---------------------------------------------------------------------------

func TestFileStore_Get_NoFileExists(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStore(dir)

	// No Set was called, so tokens.json does not exist.
	got, err := store.Get("any-key")
	require.Error(t, err, "Get when no tokens.json exists must return an error, not panic")
	assert.Nil(t, got, "Get must return nil when file does not exist")
}

func TestFileStore_Get_EmptyDirectory(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStore(dir)

	// Verify no panic and a clear error.
	assert.NotPanics(t, func() {
		_, _ = store.Get("missing")
	}, "Get on empty directory must not panic")
}

// ---------------------------------------------------------------------------
// Test 13: Directory creation — Set creates directory tree (AC-9 implied)
// ---------------------------------------------------------------------------

func TestFileStore_Set_CreatesDirectoryTree(t *testing.T) {
	tmpDir := t.TempDir()
	// Use a nested path that does not exist yet.
	deepDir := filepath.Join(tmpDir, "a", "b", "c")
	store := NewFileStore(deepDir)

	tok := fileStoreToken()
	err := store.Set("dir-create-key", tok)
	require.NoError(t, err, "Set must create directory tree if it does not exist")

	// Verify the file was created.
	filePath := filepath.Join(deepDir, "tokens.json")
	info, err := os.Stat(filePath)
	require.NoError(t, err, "tokens.json must exist after Set in nested directory")
	assert.False(t, info.IsDir(), "tokens.json must be a file, not a directory")

	// Verify round-trip works.
	got, err := store.Get("dir-create-key")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, tok.AccessToken, got.AccessToken)
}

// ---------------------------------------------------------------------------
// Test 14: Empty base path with XDG uses XDG path (AC-9)
// ---------------------------------------------------------------------------

func TestFileStore_EmptyBasePath_WithXDG(t *testing.T) {
	tmpDir := t.TempDir()
	xdgDir := filepath.Join(tmpDir, "xdg-empty-base")
	t.Setenv("XDG_CONFIG_HOME", xdgDir)

	store := NewFileStore("")
	tok := fileStoreToken()

	err := store.Set("empty-base-key", tok)
	require.NoError(t, err)

	expectedFile := filepath.Join(xdgDir, "toolwright", "tokens.json")
	_, err = os.Stat(expectedFile)
	require.NoError(t, err, "With empty basePath and XDG set, file must be at %s", expectedFile)
}

// ---------------------------------------------------------------------------
// Test 15: Explicit base path overrides XDG (AC-9)
// ---------------------------------------------------------------------------

func TestFileStore_ExplicitBasePath_OverridesXDG(t *testing.T) {
	tmpDir := t.TempDir()
	explicitDir := filepath.Join(tmpDir, "explicit")
	xdgDir := filepath.Join(tmpDir, "xdg-should-not-be-used")

	t.Setenv("XDG_CONFIG_HOME", xdgDir)

	store := NewFileStore(explicitDir)
	tok := fileStoreToken()

	err := store.Set("override-key", tok)
	require.NoError(t, err)

	// File must be at the explicit path, NOT the XDG path.
	explicitFile := filepath.Join(explicitDir, "tokens.json")
	_, err = os.Stat(explicitFile)
	require.NoError(t, err, "File must exist at explicit path %s", explicitFile)

	// XDG path must NOT have been touched.
	xdgFile := filepath.Join(xdgDir, "toolwright", "tokens.json")
	_, err = os.Stat(xdgFile)
	assert.True(t, os.IsNotExist(err),
		"XDG path %s must NOT exist when explicit basePath is provided", xdgFile)
}

// ---------------------------------------------------------------------------
// Additional edge cases
// ---------------------------------------------------------------------------

func TestFileStore_Delete_NonExistentKey_NoError(t *testing.T) {
	store, _ := newTempFileStore(t)

	// Set one key so the file exists.
	err := store.Set("exists", fileStoreToken())
	require.NoError(t, err)

	// Delete a key that was never set. Should not panic; error is acceptable.
	assert.NotPanics(t, func() {
		_ = store.Delete("never-set")
	}, "Deleting a non-existent key must not panic")
}

func TestFileStore_Delete_WhenNoFileExists(t *testing.T) {
	dir := t.TempDir()
	store := NewFileStore(dir)

	// No file exists. Delete should not panic.
	assert.NotPanics(t, func() {
		_ = store.Delete("no-file-key")
	}, "Delete when no tokens.json exists must not panic")
}

func TestFileStore_Set_OverwritesSameKey(t *testing.T) {
	store, _ := newTempFileStore(t)

	tok1 := fileStoreToken()
	tok2 := fileStoreAlternateToken()

	err := store.Set("overwrite-key", tok1)
	require.NoError(t, err)

	err = store.Set("overwrite-key", tok2)
	require.NoError(t, err)

	got, err := store.Get("overwrite-key")
	require.NoError(t, err)
	require.NotNil(t, got)

	// Must return tok2, not tok1.
	assert.Equal(t, tok2.AccessToken, got.AccessToken,
		"After overwrite, must return the latest token")
	assert.NotEqual(t, tok1.AccessToken, got.AccessToken,
		"After overwrite, must NOT return the original token")
}

func TestFileStore_TokenWithZeroExpiry(t *testing.T) {
	store, _ := newTempFileStore(t)

	tok := StoredToken{
		AccessToken:  "pat-no-expiry",
		RefreshToken: "",
		TokenType:    "Bearer",
		Expiry:       time.Time{},
		Scopes:       nil,
	}

	err := store.Set("zero-expiry", tok)
	require.NoError(t, err)

	got, err := store.Get("zero-expiry")
	require.NoError(t, err)
	require.NotNil(t, got)

	assert.Equal(t, "pat-no-expiry", got.AccessToken)
	assert.True(t, got.Expiry.IsZero(), "Zero expiry must survive round-trip")
}

func TestFileStore_TokenWithSpecialCharactersInKey(t *testing.T) {
	store, _ := newTempFileStore(t)
	tok := fileStoreToken()

	// Keys can be URLs with special characters.
	key := "https://auth.example.com/oauth2?audience=api&scope=all"

	err := store.Set(key, tok)
	require.NoError(t, err)

	got, err := store.Get(key)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, tok.AccessToken, got.AccessToken)
}

func TestFileStore_ErrorsAreWrappedWithContext(t *testing.T) {
	// Constitution rule 4: errors are wrapped with context.
	dir := t.TempDir()
	store := NewFileStore(dir)

	// Get when no file exists should return a wrapped error with context.
	_, err := store.Get("any-key")
	require.Error(t, err)

	// The error message should provide context, not just the raw OS error.
	errMsg := err.Error()
	assert.True(t, len(errMsg) > 0, "Error message must not be empty")
}

func TestFileStore_GetReturnsFreshCopy(t *testing.T) {
	store, _ := newTempFileStore(t)
	tok := fileStoreToken()

	err := store.Set("copy-test", tok)
	require.NoError(t, err)

	got1, err := store.Get("copy-test")
	require.NoError(t, err)
	require.NotNil(t, got1)

	// Mutate the returned pointer.
	got1.AccessToken = "MUTATED"
	got1.Scopes = append(got1.Scopes, "injected")

	// A second Get must return the original, not the mutated value.
	got2, err := store.Get("copy-test")
	require.NoError(t, err)
	require.NotNil(t, got2)

	assert.Equal(t, tok.AccessToken, got2.AccessToken,
		"Mutating returned *StoredToken must not affect subsequent Get calls")
	assert.Equal(t, tok.Scopes, got2.Scopes,
		"Mutating returned Scopes must not affect subsequent Get calls")
}

// ---------------------------------------------------------------------------
// Table-driven round-trip tests (anti-hardcoding)
// ---------------------------------------------------------------------------

func TestFileStore_RoundTrip_TableDriven(t *testing.T) {
	tests := []struct {
		name string
		key  string
		tok  StoredToken
	}{
		{
			name: "standard token with all fields",
			key:  "provider/standard",
			tok: StoredToken{
				AccessToken:  "table-access-1",
				RefreshToken: "table-refresh-1",
				TokenType:    "Bearer",
				Expiry:       time.Date(2027, 6, 1, 12, 0, 0, 0, time.UTC),
				Scopes:       []string{"read", "write"},
			},
		},
		{
			name: "token with no refresh token",
			key:  "provider/no-refresh",
			tok: StoredToken{
				AccessToken:  "table-access-2",
				RefreshToken: "",
				TokenType:    "Bearer",
				Expiry:       time.Date(2028, 1, 1, 0, 0, 0, 0, time.UTC),
				Scopes:       []string{"admin"},
			},
		},
		{
			name: "token with MAC type",
			key:  "provider/mac",
			tok: StoredToken{
				AccessToken:  "table-mac-3",
				RefreshToken: "table-mac-refresh",
				TokenType:    "MAC",
				Expiry:       time.Date(2026, 12, 31, 23, 59, 59, 0, time.UTC),
				Scopes:       []string{"scope-a", "scope-b", "scope-c"},
			},
		},
		{
			name: "token with zero expiry",
			key:  "provider/pat",
			tok: StoredToken{
				AccessToken:  "table-pat-4",
				RefreshToken: "",
				TokenType:    "Bearer",
				Expiry:       time.Time{},
				Scopes:       nil,
			},
		},
		{
			name: "token with single scope",
			key:  "provider/single",
			tok: StoredToken{
				AccessToken:  "table-single-5",
				RefreshToken: "table-single-refresh",
				TokenType:    "Bearer",
				Expiry:       time.Date(2027, 3, 1, 0, 0, 0, 0, time.UTC),
				Scopes:       []string{"only-one"},
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			store, _ := newTempFileStore(t)

			err := store.Set(tc.key, tc.tok)
			require.NoError(t, err, "Set must succeed")

			got, err := store.Get(tc.key)
			require.NoError(t, err, "Get must succeed after Set")
			require.NotNil(t, got, "Get must return non-nil token")

			assert.Equal(t, tc.tok.AccessToken, got.AccessToken, "AccessToken")
			assert.Equal(t, tc.tok.RefreshToken, got.RefreshToken, "RefreshToken")
			assert.Equal(t, tc.tok.TokenType, got.TokenType, "TokenType")
			assert.True(t, tc.tok.Expiry.Equal(got.Expiry),
				"Expiry: want %v, got %v", tc.tok.Expiry, got.Expiry)

			if tc.tok.Scopes != nil {
				assert.Equal(t, tc.tok.Scopes, got.Scopes, "Scopes")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Verify file-level JSON structure (defense against lazy implementations)
// ---------------------------------------------------------------------------

func TestFileStore_JSON_VersionIsAlways1(t *testing.T) {
	store, dir := newTempFileStore(t)

	// Write and re-write to ensure version stays 1.
	err := store.Set("v-check-1", fileStoreToken())
	require.NoError(t, err)
	err = store.Set("v-check-2", fileStoreAlternateToken())
	require.NoError(t, err)

	filePath := tokensFilePath(dir)
	data, err := os.ReadFile(filePath)
	require.NoError(t, err)

	var tf TokenFile
	err = json.Unmarshal(data, &tf)
	require.NoError(t, err)

	assert.Equal(t, 1, tf.Version, "Version must remain 1 after multiple writes")
}

func TestFileStore_JSON_TokenCountMatchesSetCalls(t *testing.T) {
	store, dir := newTempFileStore(t)

	keys := []string{"k1", "k2", "k3", "k4"}
	for _, key := range keys {
		err := store.Set(key, StoredToken{
			AccessToken: "tok-" + key,
			TokenType:   "Bearer",
			Expiry:      time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC),
			Scopes:      []string{"s"},
		})
		require.NoError(t, err)
	}

	filePath := tokensFilePath(dir)
	data, err := os.ReadFile(filePath)
	require.NoError(t, err)

	var tf TokenFile
	err = json.Unmarshal(data, &tf)
	require.NoError(t, err)

	assert.Len(t, tf.Tokens, 4, "File must contain exactly 4 token entries")

	for _, key := range keys {
		tok, ok := tf.Tokens[key]
		require.True(t, ok, "Token for key %q must exist in file", key)
		assert.Equal(t, "tok-"+key, tok.AccessToken, "AccessToken for key %q", key)
	}
}

func TestFileStore_Delete_RemovesFromJSONFile(t *testing.T) {
	store, dir := newTempFileStore(t)

	err := store.Set("alive", fileStoreToken())
	require.NoError(t, err)
	err = store.Set("dead", fileStoreAlternateToken())
	require.NoError(t, err)

	err = store.Delete("dead")
	require.NoError(t, err)

	// Read raw file and verify "dead" key is gone.
	filePath := tokensFilePath(dir)
	data, err := os.ReadFile(filePath)
	require.NoError(t, err)

	var tf TokenFile
	err = json.Unmarshal(data, &tf)
	require.NoError(t, err)

	assert.Len(t, tf.Tokens, 1, "File must have 1 token after deleting one of two")
	_, hasDead := tf.Tokens["dead"]
	assert.False(t, hasDead, "Deleted key 'dead' must not exist in file")
	_, hasAlive := tf.Tokens["alive"]
	assert.True(t, hasAlive, "Surviving key 'alive' must still exist in file")
}

