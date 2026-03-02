package auth

import (
	"encoding/json"
	"fmt"
)

const keyringService = "toolwright"

// Keyring abstracts platform keyring operations for testability.
// The real implementation wraps go-keyring; tests use a fake.
type Keyring interface {
	Set(service, key, value string) error
	Get(service, key string) (string, error)
	Delete(service, key string) error
}

// KeyringStore stores and retrieves tokens via a platform keyring.
type KeyringStore struct {
	keyring Keyring
}

// NewKeyringStore creates a KeyringStore backed by the given Keyring.
func NewKeyringStore(kr Keyring) *KeyringStore {
	return &KeyringStore{keyring: kr}
}

// Get retrieves a stored token from the keyring.
func (ks *KeyringStore) Get(key string) (*StoredToken, error) {
	val, err := ks.keyring.Get(keyringService, key)
	if err != nil {
		return nil, fmt.Errorf("keyring get %q: %w", key, err)
	}
	var tok StoredToken
	if err := json.Unmarshal([]byte(val), &tok); err != nil {
		return nil, fmt.Errorf("keyring get %q: unmarshal token: %w", key, err)
	}
	return &tok, nil
}

// Set stores a token in the keyring.
func (ks *KeyringStore) Set(key string, token StoredToken) error {
	data, err := json.Marshal(token)
	if err != nil {
		return fmt.Errorf("keyring set %q: marshal token: %w", key, err)
	}
	if err := ks.keyring.Set(keyringService, key, string(data)); err != nil {
		return fmt.Errorf("keyring set %q: %w", key, err)
	}
	return nil
}

// Delete removes a token from the keyring.
func (ks *KeyringStore) Delete(key string) error {
	if err := ks.keyring.Delete(keyringService, key); err != nil {
		return fmt.Errorf("keyring delete %q: %w", key, err)
	}
	return nil
}
