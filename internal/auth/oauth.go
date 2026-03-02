package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/Obsidian-Owl/toolwright/internal/manifest"
	"golang.org/x/oauth2"
)

// WritableTokenStore extends TokenStore with a Set method for persisting tokens.
// Both KeyringStore and FileStore satisfy this interface.
type WritableTokenStore interface {
	TokenStore
	Set(key string, token StoredToken) error
}

// LoginConfig holds all configuration for an OAuth PKCE login flow.
type LoginConfig struct {
	Auth        manifest.Auth
	ToolName    string
	ClientID    string
	HTTPClient  *http.Client // for testing — mock the token exchange
	OpenBrowser func(string) error
	ListenAddr  string        // default "127.0.0.1:8085", for testing use ":0"
	Timeout     time.Duration // default 120s
	Store       WritableTokenStore
}

// Login performs an OAuth 2.0 Authorization Code flow with PKCE.
// It starts a local callback server, opens the authorization URL in the browser,
// waits for the callback, exchanges the code for a token, and stores the result.
func Login(ctx context.Context, cfg LoginConfig) (*StoredToken, error) {
	// Inject custom HTTP client into context if provided.
	if cfg.HTTPClient != nil {
		ctx = context.WithValue(ctx, oauth2.HTTPClient, cfg.HTTPClient)
	}

	// Step 1: Discover endpoints.
	endpoint, _, err := DiscoverEndpoints(ctx, cfg.Auth.ProviderURL, cfg.Auth.Endpoints)
	if err != nil {
		return nil, fmt.Errorf("discover OAuth endpoints: %w", err)
	}

	// Step 2: Generate PKCE verifier and random state.
	verifier := oauth2.GenerateVerifier()

	stateBytes := make([]byte, 32)
	if _, err := rand.Read(stateBytes); err != nil {
		return nil, fmt.Errorf("generate state: %w", err)
	}
	state := base64.RawURLEncoding.EncodeToString(stateBytes)

	// Step 3: Start callback HTTP server.
	// Try the configured address first; if it fails, fall back to a random port.
	ln, err := net.Listen("tcp", cfg.ListenAddr)
	if err != nil {
		ln, err = net.Listen("tcp", ":0")
		if err != nil {
			return nil, fmt.Errorf("start callback server: %w", err)
		}
	}

	// Step 4: Build redirect URI from actual server address.
	port := ln.Addr().(*net.TCPAddr).Port
	redirectURI := fmt.Sprintf("http://127.0.0.1:%d/callback", port)

	// Step 5: Create oauth2.Config.
	oauthCfg := &oauth2.Config{
		ClientID:    cfg.ClientID,
		Scopes:      cfg.Auth.Scopes,
		Endpoint:    *endpoint,
		RedirectURL: redirectURI,
	}

	// Step 6: Build auth URL with PKCE challenge and state.
	authURL := oauthCfg.AuthCodeURL(state, oauth2.S256ChallengeOption(verifier))

	// Step 7: Channel to receive the callback result.
	type callbackResult struct {
		code string
		err  error
	}
	resultCh := make(chan callbackResult, 1)

	// Step 8: Start callback server.
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		code := q.Get("code")
		receivedState := q.Get("state")

		if receivedState != state {
			http.Error(w, "Security check failed (state mismatch)", http.StatusBadRequest)
			resultCh <- callbackResult{err: fmt.Errorf("Security check failed (state mismatch)")}
			return
		}

		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("<html><body>Login successful. You may close this window.</body></html>"))
		resultCh <- callbackResult{code: code}
	})

	server := &http.Server{Handler: mux}
	go func() {
		_ = server.Serve(ln)
	}()

	// Ensure the server shuts down when we return.
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = server.Shutdown(shutdownCtx)
	}()

	// Step 9: Open browser.
	if err := cfg.OpenBrowser(authURL); err != nil {
		return nil, fmt.Errorf("open browser: %w", err)
	}

	// Step 10: Wait for callback or timeout.
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = 120 * time.Second
	}

	var cbResult callbackResult
	select {
	case cbResult = <-resultCh:
		// Got callback.
	case <-time.After(timeout):
		return nil, fmt.Errorf("Login cancelled or timed out")
	case <-ctx.Done():
		return nil, fmt.Errorf("login cancelled: %w", ctx.Err())
	}

	if cbResult.err != nil {
		return nil, cbResult.err
	}

	// Step 11: Exchange code for token.
	token, err := oauthCfg.Exchange(ctx, cbResult.code, oauth2.VerifierOption(verifier))
	if err != nil {
		return nil, fmt.Errorf("exchange authorization code: %w", err)
	}

	// Step 12: Convert to StoredToken.
	stored := &StoredToken{
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		TokenType:    token.TokenType,
		Expiry:       token.Expiry,
	}

	// Step 13: Persist to store if provided.
	if cfg.Store != nil {
		if err := cfg.Store.Set(cfg.ToolName, *stored); err != nil {
			return nil, fmt.Errorf("store token: %w", err)
		}
	}

	return stored, nil
}

// RefreshConfig holds all configuration for a token refresh operation.
type RefreshConfig struct {
	Endpoint   oauth2.Endpoint
	HTTPClient *http.Client // for testing — mock the token endpoint
	Store      WritableTokenStore
	ToolName   string
}

// Refresh attempts to refresh an expired OAuth token using its refresh_token.
// If the refresh succeeds, the new token is stored (if Store is non-nil) and returned.
// If the refresh fails or no refresh_token is available, an error is returned
// directing the user to re-run "toolwright login {toolName}".
func Refresh(ctx context.Context, _ manifest.Auth, stored StoredToken, cfg RefreshConfig) (*StoredToken, error) {
	// Step 1: Require a refresh token.
	if stored.RefreshToken == "" {
		return nil, fmt.Errorf("token refresh failed for %s: no refresh token available. Re-run \"toolwright login %s\"",
			cfg.ToolName, cfg.ToolName)
	}

	// Step 2: Build an oauth2.Token from the stored token.
	existing := &oauth2.Token{
		AccessToken:  stored.AccessToken,
		RefreshToken: stored.RefreshToken,
		TokenType:    stored.TokenType,
		Expiry:       stored.Expiry,
	}

	// Step 3: Create an oauth2.Config with the provided endpoint.
	oauthCfg := &oauth2.Config{
		Endpoint: cfg.Endpoint,
	}

	// Step 4: Inject custom HTTP client into context if provided.
	if cfg.HTTPClient != nil {
		ctx = context.WithValue(ctx, oauth2.HTTPClient, cfg.HTTPClient)
	}

	// Step 5: Create a token source that auto-refreshes when expired.
	tokenSource := oauthCfg.TokenSource(ctx, existing)

	// Step 6: Obtain a valid token (refreshing if necessary).
	newToken, err := tokenSource.Token()
	if err != nil {
		return nil, fmt.Errorf("token refresh failed for %s: %w. Re-run \"toolwright login %s\"",
			cfg.ToolName, err, cfg.ToolName)
	}

	// Step 7: Convert oauth2.Token to StoredToken.
	result := &StoredToken{
		AccessToken:  newToken.AccessToken,
		RefreshToken: newToken.RefreshToken,
		TokenType:    newToken.TokenType,
		Expiry:       newToken.Expiry,
	}

	// Step 8: Persist to store if provided.
	if cfg.Store != nil {
		if err := cfg.Store.Set(cfg.ToolName, *result); err != nil {
			return nil, fmt.Errorf("store refreshed token for %s: %w", cfg.ToolName, err)
		}
	}

	return result, nil
}

// discoveryDoc is the minimal set of fields we parse from a well-known
// discovery document (RFC 8414 or OIDC).
type discoveryDoc struct {
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	JWKSURI               string `json:"jwks_uri"`
}

// DiscoverEndpoints attempts to discover OAuth endpoints for the given provider.
// Discovery order:
//  1. Try {providerURL}/.well-known/oauth-authorization-server (RFC 8414)
//  2. Fallback to {providerURL}/.well-known/openid-configuration
//  3. Fallback to manual endpoints if provided
//  4. Error if none succeed
//
// Returns: oauth2.Endpoint (AuthURL + TokenURL), JWKS URI string, error.
func DiscoverEndpoints(ctx context.Context, providerURL string, manual *manifest.Endpoints) (*oauth2.Endpoint, string, error) {
	if providerURL == "" {
		return nil, "", fmt.Errorf("providerURL must not be empty")
	}

	// Strip trailing slash to avoid double-slashes when appending well-known paths.
	base := strings.TrimRight(providerURL, "/")

	rfc8414URL := base + "/.well-known/oauth-authorization-server"
	oidcURL := base + "/.well-known/openid-configuration"

	// Try RFC 8414 first.
	if doc, ok := fetchDiscoveryDoc(ctx, rfc8414URL); ok {
		return &oauth2.Endpoint{AuthURL: doc.AuthorizationEndpoint, TokenURL: doc.TokenEndpoint}, doc.JWKSURI, nil
	}

	// Check context after first attempt so context.Canceled propagates.
	if err := ctx.Err(); err != nil {
		return nil, "", err
	}

	// Try OIDC configuration.
	if doc, ok := fetchDiscoveryDoc(ctx, oidcURL); ok {
		return &oauth2.Endpoint{AuthURL: doc.AuthorizationEndpoint, TokenURL: doc.TokenEndpoint}, doc.JWKSURI, nil
	}

	// Check context after second attempt.
	if err := ctx.Err(); err != nil {
		return nil, "", err
	}

	// Fall back to manual endpoints if provided and non-empty.
	if manual != nil && (manual.Authorization != "" || manual.Token != "") {
		return &oauth2.Endpoint{AuthURL: manual.Authorization, TokenURL: manual.Token}, manual.JWKS, nil
	}

	return nil, "", fmt.Errorf(
		"OAuth endpoint discovery failed: tried %s and %s; no manual endpoints provided",
		rfc8414URL, oidcURL,
	)
}

// fetchDiscoveryDoc performs a GET request to url and attempts to parse the
// response as a discoveryDoc. Returns (doc, true) on success, or (zero, false)
// if the request fails, the response is not 2xx, the JSON is invalid, or the
// document is missing required endpoint fields.
func fetchDiscoveryDoc(ctx context.Context, url string) (discoveryDoc, bool) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return discoveryDoc{}, false
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return discoveryDoc{}, false
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return discoveryDoc{}, false
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return discoveryDoc{}, false
	}

	var doc discoveryDoc
	if err := json.Unmarshal(body, &doc); err != nil {
		return discoveryDoc{}, false
	}

	// Require both endpoints to be present.
	if doc.AuthorizationEndpoint == "" || doc.TokenEndpoint == "" {
		return discoveryDoc{}, false
	}

	return doc, true
}
