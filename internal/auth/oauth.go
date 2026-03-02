package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/Obsidian-Owl/toolwright/internal/manifest"
	"golang.org/x/oauth2"
)

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
