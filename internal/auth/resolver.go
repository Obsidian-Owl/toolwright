package auth

import (
	"context"
	"fmt"
	"os"

	"github.com/Obsidian-Owl/toolwright/internal/manifest"
)

// TokenStore is the interface for retrieving stored tokens.
// Both KeyringStore and FileStore satisfy this interface.
type TokenStore interface {
	Get(key string) (*StoredToken, error)
}

// Resolver resolves an authentication token by checking a chain of sources:
// flag value → environment variable → keyring → file store → error.
type Resolver struct {
	Keyring TokenStore
	Store   TokenStore
}

// Resolve returns an authentication token by checking sources in priority order:
// 1. Explicit flag value
// 2. Environment variable (auth.TokenEnv)
// 3. Keyring store
// 4. File store
// 5. Error with actionable message
func (r *Resolver) Resolve(_ context.Context, auth manifest.Auth, toolName string, flagValue string) (string, error) {
	// Step 1: flag value wins unconditionally.
	if flagValue != "" {
		return flagValue, nil
	}

	// Step 2: environment variable.
	if auth.TokenEnv != "" {
		if val := os.Getenv(auth.TokenEnv); val != "" {
			return val, nil
		}
	}

	// Step 3: keyring store.
	if r.Keyring != nil {
		tok, err := r.Keyring.Get(toolName)
		if err == nil && !tok.IsExpired() {
			return tok.AccessToken, nil
		}
	}

	// Step 4: file store.
	if r.Store != nil {
		tok, err := r.Store.Get(toolName)
		if err == nil && !tok.IsExpired() {
			return tok.AccessToken, nil
		}
	}

	// Step 5: all sources exhausted — return actionable error.
	return "", fmt.Errorf("tool %q requires authentication. Set %s or run %q.",
		toolName, auth.TokenEnv, "toolwright login "+toolName)
}
