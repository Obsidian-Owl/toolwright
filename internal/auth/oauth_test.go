package auth

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Obsidian-Owl/toolwright/internal/manifest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// discoveryResponse is the JSON body returned by mock well-known endpoints.
type discoveryResponse struct {
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	JWKSURI               string `json:"jwks_uri,omitempty"`
}

// newDiscoveryServer creates an httptest.Server that responds to well-known
// paths. Each handler is optional; if nil, the server returns 404 for that path.
// The caller should defer server.Close().
func newDiscoveryServer(
	rfc8414Handler http.HandlerFunc,
	oidcHandler http.HandlerFunc,
) *httptest.Server {
	mux := http.NewServeMux()
	if rfc8414Handler != nil {
		mux.HandleFunc("/.well-known/oauth-authorization-server", rfc8414Handler)
	}
	if oidcHandler != nil {
		mux.HandleFunc("/.well-known/openid-configuration", oidcHandler)
	}
	return httptest.NewServer(mux)
}

// jsonHandler returns an http.HandlerFunc that writes a JSON body with
// the given status code.
func jsonHandler(status int, body interface{}) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		data, _ := json.Marshal(body)
		w.Write(data)
	}
}

// errorHandler returns an http.HandlerFunc that writes the given status code
// with no body.
func errorHandler(status int) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(status)
	}
}

// invalidJSONHandler returns an http.HandlerFunc that writes invalid JSON.
func invalidJSONHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{this is not valid json!!!`))
	}
}

// ---------------------------------------------------------------------------
// AC-11: RFC 8414 discovery finds endpoints
// ---------------------------------------------------------------------------

func TestDiscoverEndpoints_RFC8414_Success(t *testing.T) {
	// AC-11: Mock server at /.well-known/oauth-authorization-server returns
	// authorization + token + jwks endpoints. DiscoverEndpoints must extract
	// all three.
	resp := discoveryResponse{
		AuthorizationEndpoint: "https://auth.example.com/authorize",
		TokenEndpoint:         "https://auth.example.com/token",
		JWKSURI:               "https://auth.example.com/.well-known/jwks.json",
	}
	srv := newDiscoveryServer(jsonHandler(http.StatusOK, resp), nil)
	defer srv.Close()

	endpoint, jwksURI, err := DiscoverEndpoints(context.Background(), srv.URL, nil)
	require.NoError(t, err, "DiscoverEndpoints must succeed when RFC 8414 returns valid metadata")
	require.NotNil(t, endpoint, "returned endpoint must not be nil")

	assert.Equal(t, "https://auth.example.com/authorize", endpoint.AuthURL,
		"AuthURL must match the authorization_endpoint from discovery")
	assert.Equal(t, "https://auth.example.com/token", endpoint.TokenURL,
		"TokenURL must match the token_endpoint from discovery")
	assert.Equal(t, "https://auth.example.com/.well-known/jwks.json", jwksURI,
		"JWKS URI must match the jwks_uri from discovery")
}

func TestDiscoverEndpoints_RFC8414_PartialResponse_MissingJWKS(t *testing.T) {
	// AC-11: RFC 8414 response missing jwks_uri should still succeed.
	// JWKS URI should be empty.
	resp := discoveryResponse{
		AuthorizationEndpoint: "https://auth.example.com/authorize",
		TokenEndpoint:         "https://auth.example.com/token",
		// JWKSURI intentionally omitted
	}
	srv := newDiscoveryServer(jsonHandler(http.StatusOK, resp), nil)
	defer srv.Close()

	endpoint, jwksURI, err := DiscoverEndpoints(context.Background(), srv.URL, nil)
	require.NoError(t, err, "DiscoverEndpoints must succeed even without jwks_uri")
	require.NotNil(t, endpoint)

	assert.Equal(t, "https://auth.example.com/authorize", endpoint.AuthURL)
	assert.Equal(t, "https://auth.example.com/token", endpoint.TokenURL)
	assert.Empty(t, jwksURI, "JWKS URI should be empty when not in discovery response")
}

func TestDiscoverEndpoints_RFC8414_ExactValues(t *testing.T) {
	// Anti-hardcoding: use distinct endpoint URLs and verify each is returned
	// exactly. A sloppy implementation that swaps or hardcodes values will fail.
	tests := []struct {
		name     string
		authURL  string
		tokenURL string
		jwksURI  string
	}{
		{
			name:     "example.com endpoints",
			authURL:  "https://example.com/oauth/authorize",
			tokenURL: "https://example.com/oauth/token",
			jwksURI:  "https://example.com/.well-known/jwks",
		},
		{
			name:     "auth0 style endpoints",
			authURL:  "https://myapp.auth0.com/authorize",
			tokenURL: "https://myapp.auth0.com/oauth/token",
			jwksURI:  "https://myapp.auth0.com/.well-known/jwks.json",
		},
		{
			name:     "keycloak style endpoints with realm path",
			authURL:  "https://keycloak.internal/realms/myrealm/protocol/openid-connect/auth",
			tokenURL: "https://keycloak.internal/realms/myrealm/protocol/openid-connect/token",
			jwksURI:  "https://keycloak.internal/realms/myrealm/protocol/openid-connect/certs",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp := discoveryResponse{
				AuthorizationEndpoint: tc.authURL,
				TokenEndpoint:         tc.tokenURL,
				JWKSURI:               tc.jwksURI,
			}
			srv := newDiscoveryServer(jsonHandler(http.StatusOK, resp), nil)
			defer srv.Close()

			endpoint, jwksURI, err := DiscoverEndpoints(context.Background(), srv.URL, nil)
			require.NoError(t, err)
			require.NotNil(t, endpoint)

			assert.Equal(t, tc.authURL, endpoint.AuthURL,
				"AuthURL must exactly match the discovery response")
			assert.Equal(t, tc.tokenURL, endpoint.TokenURL,
				"TokenURL must exactly match the discovery response")
			assert.Equal(t, tc.jwksURI, jwksURI,
				"JWKS URI must exactly match the discovery response")
		})
	}
}

// ---------------------------------------------------------------------------
// AC-12: OIDC fallback
// ---------------------------------------------------------------------------

func TestDiscoverEndpoints_OIDCFallback_RFC8414Returns404(t *testing.T) {
	// AC-12: RFC 8414 returns 404, OIDC endpoint returns valid metadata.
	oidcResp := discoveryResponse{
		AuthorizationEndpoint: "https://oidc.example.com/authorize",
		TokenEndpoint:         "https://oidc.example.com/token",
		JWKSURI:               "https://oidc.example.com/jwks",
	}
	srv := newDiscoveryServer(
		errorHandler(http.StatusNotFound),
		jsonHandler(http.StatusOK, oidcResp),
	)
	defer srv.Close()

	endpoint, jwksURI, err := DiscoverEndpoints(context.Background(), srv.URL, nil)
	require.NoError(t, err, "DiscoverEndpoints must succeed via OIDC fallback when RFC 8414 returns 404")
	require.NotNil(t, endpoint)

	assert.Equal(t, "https://oidc.example.com/authorize", endpoint.AuthURL)
	assert.Equal(t, "https://oidc.example.com/token", endpoint.TokenURL)
	assert.Equal(t, "https://oidc.example.com/jwks", jwksURI)
}

func TestDiscoverEndpoints_OIDCFallback_RFC8414Returns500(t *testing.T) {
	// AC-12: Non-404 server errors (500) should also trigger OIDC fallback.
	oidcResp := discoveryResponse{
		AuthorizationEndpoint: "https://oidc.example.com/auth",
		TokenEndpoint:         "https://oidc.example.com/tok",
		JWKSURI:               "https://oidc.example.com/keys",
	}
	srv := newDiscoveryServer(
		errorHandler(http.StatusInternalServerError),
		jsonHandler(http.StatusOK, oidcResp),
	)
	defer srv.Close()

	endpoint, jwksURI, err := DiscoverEndpoints(context.Background(), srv.URL, nil)
	require.NoError(t, err, "DiscoverEndpoints must fall back to OIDC when RFC 8414 returns 500")
	require.NotNil(t, endpoint)

	assert.Equal(t, "https://oidc.example.com/auth", endpoint.AuthURL)
	assert.Equal(t, "https://oidc.example.com/tok", endpoint.TokenURL)
	assert.Equal(t, "https://oidc.example.com/keys", jwksURI)
}

func TestDiscoverEndpoints_RFC8414TakesPriority_OIDCNotTried(t *testing.T) {
	// AC-12 (implicit): When both RFC 8414 and OIDC are valid, RFC 8414
	// takes priority and OIDC should NOT be tried.
	var oidcCalled atomic.Int32

	rfc8414Resp := discoveryResponse{
		AuthorizationEndpoint: "https://rfc8414.example.com/authorize",
		TokenEndpoint:         "https://rfc8414.example.com/token",
		JWKSURI:               "https://rfc8414.example.com/jwks",
	}

	oidcResp := discoveryResponse{
		AuthorizationEndpoint: "https://oidc.example.com/authorize",
		TokenEndpoint:         "https://oidc.example.com/token",
		JWKSURI:               "https://oidc.example.com/jwks",
	}

	oidcHandler := func(w http.ResponseWriter, r *http.Request) {
		oidcCalled.Add(1)
		w.Header().Set("Content-Type", "application/json")
		data, _ := json.Marshal(oidcResp)
		w.Write(data)
	}

	srv := newDiscoveryServer(
		jsonHandler(http.StatusOK, rfc8414Resp),
		oidcHandler,
	)
	defer srv.Close()

	endpoint, jwksURI, err := DiscoverEndpoints(context.Background(), srv.URL, nil)
	require.NoError(t, err)
	require.NotNil(t, endpoint)

	// Must return RFC 8414 endpoints, not OIDC.
	assert.Equal(t, "https://rfc8414.example.com/authorize", endpoint.AuthURL,
		"RFC 8414 must take priority over OIDC")
	assert.Equal(t, "https://rfc8414.example.com/token", endpoint.TokenURL,
		"RFC 8414 must take priority over OIDC")
	assert.Equal(t, "https://rfc8414.example.com/jwks", jwksURI)

	// OIDC endpoint should NOT have been called.
	assert.Equal(t, int32(0), oidcCalled.Load(),
		"OIDC endpoint must NOT be called when RFC 8414 succeeds")
}

// ---------------------------------------------------------------------------
// AC-13: Manual endpoint fallback
// ---------------------------------------------------------------------------

func TestDiscoverEndpoints_ManualFallback_BothDiscoveryFail(t *testing.T) {
	// AC-13: Both discovery endpoints fail, manual endpoints provided ->
	// returns manual endpoints as oauth2.Endpoint.
	srv := newDiscoveryServer(
		errorHandler(http.StatusNotFound),
		errorHandler(http.StatusNotFound),
	)
	defer srv.Close()

	manual := &manifest.Endpoints{
		Authorization: "https://manual.example.com/authorize",
		Token:         "https://manual.example.com/token",
		JWKS:          "https://manual.example.com/jwks",
	}

	endpoint, jwksURI, err := DiscoverEndpoints(context.Background(), srv.URL, manual)
	require.NoError(t, err, "DiscoverEndpoints must succeed with manual endpoints when discovery fails")
	require.NotNil(t, endpoint)

	assert.Equal(t, "https://manual.example.com/authorize", endpoint.AuthURL,
		"AuthURL must come from manual endpoints")
	assert.Equal(t, "https://manual.example.com/token", endpoint.TokenURL,
		"TokenURL must come from manual endpoints")
	assert.Equal(t, "https://manual.example.com/jwks", jwksURI,
		"JWKS URI must come from manual endpoints")
}

func TestDiscoverEndpoints_ManualFallback_CorrectMapping(t *testing.T) {
	// AC-13: Verify each manual endpoint field maps to the correct output field.
	// Use completely distinct values for each to catch field swaps.
	srv := newDiscoveryServer(
		errorHandler(http.StatusNotFound),
		errorHandler(http.StatusNotFound),
	)
	defer srv.Close()

	manual := &manifest.Endpoints{
		Authorization: "https://AUTHORIZATION.example.com/path",
		Token:         "https://TOKEN.example.com/path",
		JWKS:          "https://JWKS.example.com/path",
	}

	endpoint, jwksURI, err := DiscoverEndpoints(context.Background(), srv.URL, manual)
	require.NoError(t, err)
	require.NotNil(t, endpoint)

	// Verify no field swaps.
	assert.Equal(t, "https://AUTHORIZATION.example.com/path", endpoint.AuthURL,
		"manual Authorization must map to AuthURL, not to TokenURL or JWKS")
	assert.Equal(t, "https://TOKEN.example.com/path", endpoint.TokenURL,
		"manual Token must map to TokenURL, not to AuthURL or JWKS")
	assert.Equal(t, "https://JWKS.example.com/path", jwksURI,
		"manual JWKS must map to JWKS URI string, not to AuthURL or TokenURL")
}

func TestDiscoverEndpoints_ManualFallback_NoManualEndpoints_Error(t *testing.T) {
	// AC-13: Both discovery endpoints fail, no manual endpoints -> error
	// naming both URLs tried.
	srv := newDiscoveryServer(
		errorHandler(http.StatusNotFound),
		errorHandler(http.StatusNotFound),
	)
	defer srv.Close()

	endpoint, jwksURI, err := DiscoverEndpoints(context.Background(), srv.URL, nil)
	require.Error(t, err, "DiscoverEndpoints must error when all sources fail and no manual endpoints")
	assert.Nil(t, endpoint, "endpoint must be nil on error")
	assert.Empty(t, jwksURI, "JWKS URI must be empty on error")

	errMsg := err.Error()
	// AC-13: error naming both URLs tried.
	assert.Contains(t, errMsg, ".well-known/oauth-authorization-server",
		"Error must mention the RFC 8414 URL tried")
	assert.Contains(t, errMsg, ".well-known/openid-configuration",
		"Error must mention the OIDC URL tried")
}

func TestDiscoverEndpoints_ManualFallback_EmptyManualEndpoints_Error(t *testing.T) {
	// Empty (zero-value) manual endpoints should be treated the same as nil:
	// fall through to error.
	srv := newDiscoveryServer(
		errorHandler(http.StatusNotFound),
		errorHandler(http.StatusNotFound),
	)
	defer srv.Close()

	emptyManual := &manifest.Endpoints{}

	_, _, err := DiscoverEndpoints(context.Background(), srv.URL, emptyManual)
	require.Error(t, err,
		"DiscoverEndpoints must error when manual endpoints are all empty strings")
}

// ---------------------------------------------------------------------------
// Edge cases: invalid JSON from discovery
// ---------------------------------------------------------------------------

func TestDiscoverEndpoints_RFC8414_InvalidJSON_FallsToOIDC(t *testing.T) {
	// Edge case: RFC 8414 returns invalid JSON -> must fall through to OIDC.
	oidcResp := discoveryResponse{
		AuthorizationEndpoint: "https://oidc-fallback.example.com/authorize",
		TokenEndpoint:         "https://oidc-fallback.example.com/token",
		JWKSURI:               "https://oidc-fallback.example.com/jwks",
	}

	srv := newDiscoveryServer(
		invalidJSONHandler(),
		jsonHandler(http.StatusOK, oidcResp),
	)
	defer srv.Close()

	endpoint, jwksURI, err := DiscoverEndpoints(context.Background(), srv.URL, nil)
	require.NoError(t, err, "Must fall through to OIDC when RFC 8414 returns invalid JSON")
	require.NotNil(t, endpoint)

	assert.Equal(t, "https://oidc-fallback.example.com/authorize", endpoint.AuthURL)
	assert.Equal(t, "https://oidc-fallback.example.com/token", endpoint.TokenURL)
	assert.Equal(t, "https://oidc-fallback.example.com/jwks", jwksURI)
}

func TestDiscoverEndpoints_BothInvalidJSON_FallsToManual(t *testing.T) {
	// Edge case: Both RFC 8414 and OIDC return invalid JSON -> fall to manual.
	srv := newDiscoveryServer(
		invalidJSONHandler(),
		invalidJSONHandler(),
	)
	defer srv.Close()

	manual := &manifest.Endpoints{
		Authorization: "https://manual-after-json.example.com/auth",
		Token:         "https://manual-after-json.example.com/token",
		JWKS:          "https://manual-after-json.example.com/jwks",
	}

	endpoint, jwksURI, err := DiscoverEndpoints(context.Background(), srv.URL, manual)
	require.NoError(t, err, "Must fall through to manual when both discovery endpoints return invalid JSON")
	require.NotNil(t, endpoint)

	assert.Equal(t, "https://manual-after-json.example.com/auth", endpoint.AuthURL)
	assert.Equal(t, "https://manual-after-json.example.com/token", endpoint.TokenURL)
	assert.Equal(t, "https://manual-after-json.example.com/jwks", jwksURI)
}

func TestDiscoverEndpoints_OIDC_InvalidJSON_FallsToManual(t *testing.T) {
	// Edge case: RFC 8414 returns 404, OIDC returns invalid JSON -> fall to manual.
	srv := newDiscoveryServer(
		errorHandler(http.StatusNotFound),
		invalidJSONHandler(),
	)
	defer srv.Close()

	manual := &manifest.Endpoints{
		Authorization: "https://manual.example.com/auth",
		Token:         "https://manual.example.com/token",
		JWKS:          "https://manual.example.com/jwks",
	}

	endpoint, jwksURI, err := DiscoverEndpoints(context.Background(), srv.URL, manual)
	require.NoError(t, err, "Must fall through to manual when OIDC returns invalid JSON")
	require.NotNil(t, endpoint)

	assert.Equal(t, "https://manual.example.com/auth", endpoint.AuthURL)
	assert.Equal(t, "https://manual.example.com/token", endpoint.TokenURL)
	assert.Equal(t, "https://manual.example.com/jwks", jwksURI)
}

// ---------------------------------------------------------------------------
// Edge cases: context cancellation
// ---------------------------------------------------------------------------

func TestDiscoverEndpoints_ContextCancelled(t *testing.T) {
	// Edge case: Cancelled context must return an error, not hang or panic.
	resp := discoveryResponse{
		AuthorizationEndpoint: "https://auth.example.com/authorize",
		TokenEndpoint:         "https://auth.example.com/token",
	}
	srv := newDiscoveryServer(jsonHandler(http.StatusOK, resp), nil)
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	_, _, err := DiscoverEndpoints(ctx, srv.URL, nil)
	require.Error(t, err, "DiscoverEndpoints must return error for cancelled context")

	// The error must be or wrap context.Canceled.
	assert.True(t, errors.Is(err, context.Canceled),
		"Error must wrap context.Canceled; got: %v", err)
}

// ---------------------------------------------------------------------------
// Edge cases: provider URL formatting
// ---------------------------------------------------------------------------

func TestDiscoverEndpoints_ProviderURL_WithTrailingSlash(t *testing.T) {
	// Edge case: Provider URL with trailing slash should not produce double slashes
	// in the well-known URL.
	resp := discoveryResponse{
		AuthorizationEndpoint: "https://auth.example.com/authorize",
		TokenEndpoint:         "https://auth.example.com/token",
		JWKSURI:               "https://auth.example.com/jwks",
	}

	// Track the actual request paths to verify no double slashes.
	var requestPaths []string
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		requestPaths = append(requestPaths, r.URL.Path)
		if r.URL.Path == "/.well-known/oauth-authorization-server" {
			w.Header().Set("Content-Type", "application/json")
			data, _ := json.Marshal(resp)
			w.Write(data)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	// Add trailing slash to provider URL.
	endpoint, _, err := DiscoverEndpoints(context.Background(), srv.URL+"/", nil)
	require.NoError(t, err, "Provider URL with trailing slash must work")
	require.NotNil(t, endpoint)
	assert.Equal(t, "https://auth.example.com/authorize", endpoint.AuthURL)

	// Verify no double-slash paths were requested.
	for _, path := range requestPaths {
		assert.False(t, strings.Contains(path, "//"),
			"Request path must not contain double slashes, got: %q", path)
	}
}

func TestDiscoverEndpoints_ProviderURL_WithoutTrailingSlash(t *testing.T) {
	// Edge case: Provider URL without trailing slash should work correctly.
	resp := discoveryResponse{
		AuthorizationEndpoint: "https://auth.example.com/authorize",
		TokenEndpoint:         "https://auth.example.com/token",
	}
	srv := newDiscoveryServer(jsonHandler(http.StatusOK, resp), nil)
	defer srv.Close()

	endpoint, _, err := DiscoverEndpoints(context.Background(), srv.URL, nil)
	require.NoError(t, err, "Provider URL without trailing slash must work")
	require.NotNil(t, endpoint)
	assert.Equal(t, "https://auth.example.com/authorize", endpoint.AuthURL)
}

func TestDiscoverEndpoints_EmptyProviderURL(t *testing.T) {
	// Edge case: Empty provider URL must return an error, not panic.
	assert.NotPanics(t, func() {
		_, _, _ = DiscoverEndpoints(context.Background(), "", nil)
	}, "Empty provider URL must not panic")

	_, _, err := DiscoverEndpoints(context.Background(), "", nil)
	require.Error(t, err, "Empty provider URL must return an error")
}

// ---------------------------------------------------------------------------
// Edge case: JWKS URI extraction from different sources
// ---------------------------------------------------------------------------

func TestDiscoverEndpoints_JWKSURI_ExtractedFromRFC8414(t *testing.T) {
	// Verify JWKS URI is specifically extracted from the discovery response.
	jwksValue := "https://unique-jwks-value.example.com/keys"
	resp := discoveryResponse{
		AuthorizationEndpoint: "https://auth.example.com/authorize",
		TokenEndpoint:         "https://auth.example.com/token",
		JWKSURI:               jwksValue,
	}
	srv := newDiscoveryServer(jsonHandler(http.StatusOK, resp), nil)
	defer srv.Close()

	_, jwksURI, err := DiscoverEndpoints(context.Background(), srv.URL, nil)
	require.NoError(t, err)
	assert.Equal(t, jwksValue, jwksURI,
		"JWKS URI must be extracted from the jwks_uri field of the discovery response")
}

func TestDiscoverEndpoints_JWKSURI_ExtractedFromOIDC(t *testing.T) {
	// Verify JWKS URI is extracted from OIDC fallback.
	jwksValue := "https://oidc-jwks.example.com/certs"
	oidcResp := discoveryResponse{
		AuthorizationEndpoint: "https://oidc.example.com/auth",
		TokenEndpoint:         "https://oidc.example.com/token",
		JWKSURI:               jwksValue,
	}
	srv := newDiscoveryServer(
		errorHandler(http.StatusNotFound),
		jsonHandler(http.StatusOK, oidcResp),
	)
	defer srv.Close()

	_, jwksURI, err := DiscoverEndpoints(context.Background(), srv.URL, nil)
	require.NoError(t, err)
	assert.Equal(t, jwksValue, jwksURI,
		"JWKS URI must be extracted from the OIDC discovery response")
}

// ---------------------------------------------------------------------------
// Return type verification
// ---------------------------------------------------------------------------

func TestDiscoverEndpoints_ReturnsOauth2Endpoint(t *testing.T) {
	// Verify the returned endpoint is a proper *oauth2.Endpoint with both
	// AuthURL and TokenURL set. Guard against returning nil fields or a
	// different type.
	resp := discoveryResponse{
		AuthorizationEndpoint: "https://ep.example.com/authorize",
		TokenEndpoint:         "https://ep.example.com/token",
	}
	srv := newDiscoveryServer(jsonHandler(http.StatusOK, resp), nil)
	defer srv.Close()

	endpoint, _, err := DiscoverEndpoints(context.Background(), srv.URL, nil)
	require.NoError(t, err)
	require.NotNil(t, endpoint, "returned *oauth2.Endpoint must not be nil on success")

	assert.NotEmpty(t, endpoint.AuthURL, "AuthURL in returned endpoint must not be empty")
	assert.NotEmpty(t, endpoint.TokenURL, "TokenURL in returned endpoint must not be empty")
}

// ---------------------------------------------------------------------------
// Discovery order: comprehensive table-driven test
// ---------------------------------------------------------------------------

func TestDiscoverEndpoints_DiscoveryOrder_TableDriven(t *testing.T) {
	tests := []struct {
		name          string
		rfc8414Status int
		rfc8414Body   *discoveryResponse
		rfc8414Valid  bool // false means invalid JSON
		oidcStatus    int
		oidcBody      *discoveryResponse
		oidcValid     bool // false means invalid JSON
		manual        *manifest.Endpoints
		wantAuthURL   string
		wantTokenURL  string
		wantJWKSURI   string
		wantErr       bool
		wantErrSubs   []string // substrings that must appear in error
	}{
		{
			name:          "RFC 8414 succeeds",
			rfc8414Status: http.StatusOK,
			rfc8414Body: &discoveryResponse{
				AuthorizationEndpoint: "https://rfc.example.com/auth",
				TokenEndpoint:         "https://rfc.example.com/token",
				JWKSURI:               "https://rfc.example.com/jwks",
			},
			rfc8414Valid: true,
			oidcStatus:   http.StatusOK,
			oidcBody: &discoveryResponse{
				AuthorizationEndpoint: "https://oidc.example.com/auth",
				TokenEndpoint:         "https://oidc.example.com/token",
			},
			oidcValid:    true,
			wantAuthURL:  "https://rfc.example.com/auth",
			wantTokenURL: "https://rfc.example.com/token",
			wantJWKSURI:  "https://rfc.example.com/jwks",
		},
		{
			name:          "RFC 8414 fails, OIDC succeeds",
			rfc8414Status: http.StatusNotFound,
			rfc8414Valid:  true,
			oidcStatus:    http.StatusOK,
			oidcBody: &discoveryResponse{
				AuthorizationEndpoint: "https://oidc.example.com/auth",
				TokenEndpoint:         "https://oidc.example.com/token",
				JWKSURI:               "https://oidc.example.com/jwks",
			},
			oidcValid:    true,
			wantAuthURL:  "https://oidc.example.com/auth",
			wantTokenURL: "https://oidc.example.com/token",
			wantJWKSURI:  "https://oidc.example.com/jwks",
		},
		{
			name:          "both discovery fail, manual provided",
			rfc8414Status: http.StatusNotFound,
			rfc8414Valid:  true,
			oidcStatus:    http.StatusNotFound,
			oidcValid:     true,
			manual: &manifest.Endpoints{
				Authorization: "https://manual.example.com/auth",
				Token:         "https://manual.example.com/token",
				JWKS:          "https://manual.example.com/jwks",
			},
			wantAuthURL:  "https://manual.example.com/auth",
			wantTokenURL: "https://manual.example.com/token",
			wantJWKSURI:  "https://manual.example.com/jwks",
		},
		{
			name:          "all fail, no manual",
			rfc8414Status: http.StatusNotFound,
			rfc8414Valid:  true,
			oidcStatus:    http.StatusNotFound,
			oidcValid:     true,
			wantErr:       true,
			wantErrSubs:   []string{"oauth-authorization-server", "openid-configuration"},
		},
		{
			name:         "RFC 8414 invalid JSON, OIDC succeeds",
			rfc8414Valid: false,
			oidcStatus:   http.StatusOK,
			oidcBody: &discoveryResponse{
				AuthorizationEndpoint: "https://oidc-after-json.example.com/auth",
				TokenEndpoint:         "https://oidc-after-json.example.com/token",
			},
			oidcValid:    true,
			wantAuthURL:  "https://oidc-after-json.example.com/auth",
			wantTokenURL: "https://oidc-after-json.example.com/token",
		},
		{
			name:         "both invalid JSON, manual provided",
			rfc8414Valid: false,
			oidcValid:    false,
			manual: &manifest.Endpoints{
				Authorization: "https://manual-json.example.com/auth",
				Token:         "https://manual-json.example.com/token",
				JWKS:          "https://manual-json.example.com/jwks",
			},
			wantAuthURL:  "https://manual-json.example.com/auth",
			wantTokenURL: "https://manual-json.example.com/token",
			wantJWKSURI:  "https://manual-json.example.com/jwks",
		},
		{
			name:         "both invalid JSON, no manual",
			rfc8414Valid: false,
			oidcValid:    false,
			wantErr:      true,
			wantErrSubs:  []string{"oauth-authorization-server", "openid-configuration"},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var rfc8414Handler http.HandlerFunc
			var oidcHandler http.HandlerFunc

			if !tc.rfc8414Valid {
				rfc8414Handler = invalidJSONHandler()
			} else if tc.rfc8414Body != nil {
				rfc8414Handler = jsonHandler(tc.rfc8414Status, tc.rfc8414Body)
			} else {
				rfc8414Handler = errorHandler(tc.rfc8414Status)
			}

			if !tc.oidcValid {
				oidcHandler = invalidJSONHandler()
			} else if tc.oidcBody != nil {
				oidcHandler = jsonHandler(tc.oidcStatus, tc.oidcBody)
			} else {
				oidcHandler = errorHandler(tc.oidcStatus)
			}

			srv := newDiscoveryServer(rfc8414Handler, oidcHandler)
			defer srv.Close()

			endpoint, jwksURI, err := DiscoverEndpoints(context.Background(), srv.URL, tc.manual)

			if tc.wantErr {
				require.Error(t, err, "expected error for %q", tc.name)
				assert.Nil(t, endpoint, "endpoint must be nil on error")
				for _, sub := range tc.wantErrSubs {
					assert.Contains(t, err.Error(), sub,
						"error must contain %q", sub)
				}
				return
			}

			require.NoError(t, err, "unexpected error for %q", tc.name)
			require.NotNil(t, endpoint, "endpoint must not be nil for %q", tc.name)
			assert.Equal(t, tc.wantAuthURL, endpoint.AuthURL, "AuthURL")
			assert.Equal(t, tc.wantTokenURL, endpoint.TokenURL, "TokenURL")
			assert.Equal(t, tc.wantJWKSURI, jwksURI, "JWKS URI")
		})
	}
}

// ---------------------------------------------------------------------------
// Edge case: server not reachable
// ---------------------------------------------------------------------------

func TestDiscoverEndpoints_UnreachableServer_ManualFallback(t *testing.T) {
	// When the provider URL points to a server that is not reachable,
	// and manual endpoints are provided, manual should be used.
	manual := &manifest.Endpoints{
		Authorization: "https://manual-unreachable.example.com/auth",
		Token:         "https://manual-unreachable.example.com/token",
		JWKS:          "https://manual-unreachable.example.com/jwks",
	}

	// Use a URL that will definitely fail to connect.
	endpoint, jwksURI, err := DiscoverEndpoints(
		context.Background(),
		"http://127.0.0.1:1", // port 1 is almost certainly not listening
		manual,
	)
	require.NoError(t, err, "Must fall back to manual when server is unreachable")
	require.NotNil(t, endpoint)

	assert.Equal(t, "https://manual-unreachable.example.com/auth", endpoint.AuthURL)
	assert.Equal(t, "https://manual-unreachable.example.com/token", endpoint.TokenURL)
	assert.Equal(t, "https://manual-unreachable.example.com/jwks", jwksURI)
}

func TestDiscoverEndpoints_UnreachableServer_NoManual_Error(t *testing.T) {
	// When the provider URL is unreachable and no manual endpoints, error.
	_, _, err := DiscoverEndpoints(
		context.Background(),
		"http://127.0.0.1:1",
		nil,
	)
	require.Error(t, err, "Must error when server unreachable and no manual endpoints")
}

// ---------------------------------------------------------------------------
// Edge case: discovery returns 200 but empty body
// ---------------------------------------------------------------------------

func TestDiscoverEndpoints_RFC8414_EmptyBody_FallsToOIDC(t *testing.T) {
	emptyHandler := func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{}`))
	}

	oidcResp := discoveryResponse{
		AuthorizationEndpoint: "https://oidc-empty.example.com/auth",
		TokenEndpoint:         "https://oidc-empty.example.com/token",
		JWKSURI:               "https://oidc-empty.example.com/jwks",
	}

	srv := newDiscoveryServer(
		emptyHandler,
		jsonHandler(http.StatusOK, oidcResp),
	)
	defer srv.Close()

	// An empty JSON object with no authorization_endpoint or token_endpoint
	// should be treated as a failed discovery and fall through to OIDC.
	endpoint, jwksURI, err := DiscoverEndpoints(context.Background(), srv.URL, nil)
	require.NoError(t, err, "Must fall back to OIDC when RFC 8414 returns empty JSON object")
	require.NotNil(t, endpoint)

	assert.Equal(t, "https://oidc-empty.example.com/auth", endpoint.AuthURL)
	assert.Equal(t, "https://oidc-empty.example.com/token", endpoint.TokenURL)
	assert.Equal(t, "https://oidc-empty.example.com/jwks", jwksURI)
}

// ---------------------------------------------------------------------------
// Anti-hardcoding: multiple distinct provider URLs produce distinct results
// ---------------------------------------------------------------------------

func TestDiscoverEndpoints_DistinctProviders_DistinctResults(t *testing.T) {
	// Guard against an implementation that ignores the provider URL and returns
	// hardcoded values.
	providers := []struct {
		authURL  string
		tokenURL string
		jwksURI  string
	}{
		{
			authURL:  "https://provider-one.example.com/authorize",
			tokenURL: "https://provider-one.example.com/token",
			jwksURI:  "https://provider-one.example.com/jwks",
		},
		{
			authURL:  "https://provider-two.example.com/authorize",
			tokenURL: "https://provider-two.example.com/token",
			jwksURI:  "https://provider-two.example.com/jwks",
		},
		{
			authURL:  "https://provider-three.example.com/authorize",
			tokenURL: "https://provider-three.example.com/token",
			jwksURI:  "https://provider-three.example.com/jwks",
		},
	}

	var results []string

	for i, p := range providers {
		resp := discoveryResponse{
			AuthorizationEndpoint: p.authURL,
			TokenEndpoint:         p.tokenURL,
			JWKSURI:               p.jwksURI,
		}
		srv := newDiscoveryServer(jsonHandler(http.StatusOK, resp), nil)

		endpoint, jwksURI, err := DiscoverEndpoints(context.Background(), srv.URL, nil)
		srv.Close()

		require.NoError(t, err, "provider %d must succeed", i)
		require.NotNil(t, endpoint)

		assert.Equal(t, p.authURL, endpoint.AuthURL, "provider %d AuthURL", i)
		assert.Equal(t, p.tokenURL, endpoint.TokenURL, "provider %d TokenURL", i)
		assert.Equal(t, p.jwksURI, jwksURI, "provider %d JWKS URI", i)

		results = append(results, endpoint.AuthURL)
	}

	// All three must be different from each other.
	assert.NotEqual(t, results[0], results[1], "provider 0 and 1 must produce different results")
	assert.NotEqual(t, results[1], results[2], "provider 1 and 2 must produce different results")
	assert.NotEqual(t, results[0], results[2], "provider 0 and 2 must produce different results")
}

// ---------------------------------------------------------------------------
// Error wrapping: errors must include context (Constitution rule)
// ---------------------------------------------------------------------------

func TestDiscoverEndpoints_Error_IncludesProviderURL(t *testing.T) {
	// When all discovery fails, the error should include the provider URL
	// for debuggability.
	providerURL := "http://127.0.0.1:1"

	_, _, err := DiscoverEndpoints(context.Background(), providerURL, nil)
	require.Error(t, err)

	// The error should help the user understand which provider failed.
	errMsg := err.Error()
	assert.True(t,
		strings.Contains(errMsg, "oauth-authorization-server") &&
			strings.Contains(errMsg, "openid-configuration"),
		"Error must mention both well-known paths; got: %s", errMsg)
}

// ---------------------------------------------------------------------------
// Edge case: manual endpoints with only partial fields
// ---------------------------------------------------------------------------

func TestDiscoverEndpoints_ManualEndpoints_NoJWKS(t *testing.T) {
	// Manual endpoints with Authorization and Token but no JWKS.
	srv := newDiscoveryServer(
		errorHandler(http.StatusNotFound),
		errorHandler(http.StatusNotFound),
	)
	defer srv.Close()

	manual := &manifest.Endpoints{
		Authorization: "https://manual.example.com/auth",
		Token:         "https://manual.example.com/token",
		// JWKS intentionally empty
	}

	endpoint, jwksURI, err := DiscoverEndpoints(context.Background(), srv.URL, manual)
	require.NoError(t, err, "Manual endpoints without JWKS should succeed")
	require.NotNil(t, endpoint)

	assert.Equal(t, "https://manual.example.com/auth", endpoint.AuthURL)
	assert.Equal(t, "https://manual.example.com/token", endpoint.TokenURL)
	assert.Empty(t, jwksURI, "JWKS URI should be empty when not provided in manual endpoints")
}

// ---------------------------------------------------------------------------
// Edge case: verify correct well-known paths are requested
// ---------------------------------------------------------------------------

func TestDiscoverEndpoints_RequestsCorrectWellKnownPaths(t *testing.T) {
	// Verify the implementation actually requests the correct paths on the
	// server. A lazy implementation might request the wrong paths.
	var requestedPaths []string

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		requestedPaths = append(requestedPaths, r.URL.Path)
		if r.URL.Path == "/.well-known/oauth-authorization-server" {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{
				"authorization_endpoint": "https://auth.example.com/auth",
				"token_endpoint": "https://auth.example.com/token"
			}`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	endpoint, _, err := DiscoverEndpoints(context.Background(), srv.URL, nil)
	require.NoError(t, err)
	require.NotNil(t, endpoint)

	// The first path requested must be the RFC 8414 path.
	require.NotEmpty(t, requestedPaths, "Must have made at least one request")
	assert.Equal(t, "/.well-known/oauth-authorization-server", requestedPaths[0],
		"First request must be to RFC 8414 well-known path")
}

func TestDiscoverEndpoints_OIDC_RequestsCorrectPath(t *testing.T) {
	// When RFC 8414 fails, the OIDC path must be tried.
	var requestedPaths []string

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		requestedPaths = append(requestedPaths, r.URL.Path)
		if r.URL.Path == "/.well-known/openid-configuration" {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{
				"authorization_endpoint": "https://oidc.example.com/auth",
				"token_endpoint": "https://oidc.example.com/token"
			}`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	endpoint, _, err := DiscoverEndpoints(context.Background(), srv.URL, nil)
	require.NoError(t, err)
	require.NotNil(t, endpoint)

	// Should have tried RFC 8414 first, then OIDC.
	require.GreaterOrEqual(t, len(requestedPaths), 2,
		"Must have made at least 2 requests (RFC 8414 then OIDC)")
	assert.Equal(t, "/.well-known/oauth-authorization-server", requestedPaths[0],
		"First request must be to RFC 8414 path")
	assert.Equal(t, "/.well-known/openid-configuration", requestedPaths[1],
		"Second request must be to OIDC path")
}

// ---------------------------------------------------------------------------
// Edge case: discovery with extra fields (forward compatibility)
// ---------------------------------------------------------------------------

func TestDiscoverEndpoints_ExtraFieldsInResponse(t *testing.T) {
	// Discovery responses may include extra fields not used by our code.
	// This must not cause errors.
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/oauth-authorization-server", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{
			"authorization_endpoint": "https://auth.example.com/auth",
			"token_endpoint": "https://auth.example.com/token",
			"jwks_uri": "https://auth.example.com/jwks",
			"issuer": "https://auth.example.com",
			"response_types_supported": ["code"],
			"grant_types_supported": ["authorization_code"],
			"unknown_future_field": true
		}`)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	endpoint, jwksURI, err := DiscoverEndpoints(context.Background(), srv.URL, nil)
	require.NoError(t, err, "Extra fields in discovery response must not cause errors")
	require.NotNil(t, endpoint)

	assert.Equal(t, "https://auth.example.com/auth", endpoint.AuthURL)
	assert.Equal(t, "https://auth.example.com/token", endpoint.TokenURL)
	assert.Equal(t, "https://auth.example.com/jwks", jwksURI)
}

// ---------------------------------------------------------------------------
// Edge case: HTTP method
// ---------------------------------------------------------------------------

func TestDiscoverEndpoints_UsesGETMethod(t *testing.T) {
	// Discovery requests should use GET, not POST or other methods.
	var receivedMethods []string

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		receivedMethods = append(receivedMethods, r.Method)
		if r.URL.Path == "/.well-known/oauth-authorization-server" {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprintf(w, `{
				"authorization_endpoint": "https://auth.example.com/auth",
				"token_endpoint": "https://auth.example.com/token"
			}`)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	_, _, err := DiscoverEndpoints(context.Background(), srv.URL, nil)
	require.NoError(t, err)

	for _, method := range receivedMethods {
		assert.Equal(t, http.MethodGet, method,
			"Discovery requests must use GET method, got %q", method)
	}
}

// ===========================================================================
// AC-14, AC-15, AC-16: OAuth PKCE Login flow tests
// ===========================================================================

// ---------------------------------------------------------------------------
// Test helpers for Login tests
// ---------------------------------------------------------------------------

// fakeWritableTokenStore implements WritableTokenStore for Login tests.
type fakeWritableTokenStore struct {
	tokens map[string]StoredToken
}

func newFakeWritableTokenStore() *fakeWritableTokenStore {
	return &fakeWritableTokenStore{
		tokens: make(map[string]StoredToken),
	}
}

func (f *fakeWritableTokenStore) Get(key string) (*StoredToken, error) {
	tok, ok := f.tokens[key]
	if !ok {
		return nil, fmt.Errorf("token not found for key %q", key)
	}
	copy := tok
	return &copy, nil
}

func (f *fakeWritableTokenStore) Set(key string, token StoredToken) error {
	f.tokens[key] = token
	return nil
}

// Compile-time interface checks.
var _ WritableTokenStore = (*fakeWritableTokenStore)(nil)
var _ WritableTokenStore = (*KeyringStore)(nil)
var _ WritableTokenStore = (*FileStore)(nil)

// newMockOAuthServer creates an httptest.Server that acts as a mock OAuth
// provider. It serves:
//   - /.well-known/oauth-authorization-server  (discovery)
//   - /authorize                                (auth endpoint -- not actually hit by Login)
//   - /token                                    (token exchange)
//
// The token endpoint validates that code_verifier is present in the request
// and returns a configurable token response.
//
// tokenValidator is called with the token request's form values so tests can
// inspect the exchange request. If tokenValidator returns an error, the token
// endpoint returns a 400.
type tokenExchangeLog struct {
	CodeVerifier string
	Code         string
	GrantType    string
	RedirectURI  string
	ClientID     string
}

func newMockOAuthServer(t *testing.T, tokenValidator func(log tokenExchangeLog) error) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	// We need to know the server URL to build the discovery response, but
	// we don't know it until after httptest.NewServer. Use a pointer that
	// gets set right after.
	var serverURL string

	mux.HandleFunc("/.well-known/oauth-authorization-server", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{
			"authorization_endpoint": %q,
			"token_endpoint": %q
		}`, serverURL+"/authorize", serverURL+"/token")
	})

	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}
		if err := r.ParseForm(); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, `{"error": "invalid_request"}`)
			return
		}

		log := tokenExchangeLog{
			CodeVerifier: r.FormValue("code_verifier"),
			Code:         r.FormValue("code"),
			GrantType:    r.FormValue("grant_type"),
			RedirectURI:  r.FormValue("redirect_uri"),
			ClientID:     r.FormValue("client_id"),
		}

		if tokenValidator != nil {
			if err := tokenValidator(log); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				fmt.Fprintf(w, `{"error": %q}`, err.Error())
				return
			}
		}

		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{
			"access_token": "mock-access-token-xyz",
			"refresh_token": "mock-refresh-token-abc",
			"token_type": "Bearer",
			"expires_in": 3600
		}`)
	})

	srv := httptest.NewServer(mux)
	serverURL = srv.URL
	return srv
}

// simulateCallback parses the auth URL captured from openBrowser, extracts
// the state parameter, and makes an HTTP GET to the callback server with
// the authorization code and matching state. Returns an error if any step fails.
func simulateCallback(t *testing.T, authURL string, code string, stateOverride *string) error {
	t.Helper()
	parsed, err := url.Parse(authURL)
	if err != nil {
		return fmt.Errorf("parsing auth URL: %w", err)
	}

	state := parsed.Query().Get("state")
	if stateOverride != nil {
		state = *stateOverride
	}

	// Extract the redirect_uri from the auth URL to know where the callback server is.
	redirectURI := parsed.Query().Get("redirect_uri")
	if redirectURI == "" {
		return fmt.Errorf("auth URL missing redirect_uri parameter")
	}

	callbackURL := fmt.Sprintf("%s?code=%s&state=%s", redirectURI, url.QueryEscape(code), url.QueryEscape(state))
	resp, err := http.Get(callbackURL)
	if err != nil {
		return fmt.Errorf("GET callback: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("callback returned %d: %s", resp.StatusCode, string(body))
	}
	return nil
}

// defaultLoginConfig creates a LoginConfig suitable for testing. It uses:
//   - ListenAddr ":0" to avoid port conflicts
//   - A short timeout (5s) to catch hangs
//   - The provided mock server URL as the provider
//   - A channel-based openBrowser to capture the auth URL
func defaultLoginConfig(providerURL string, store WritableTokenStore, authURLCh chan<- string) LoginConfig {
	return LoginConfig{
		Auth: manifest.Auth{
			Type:        "oauth",
			ProviderURL: providerURL,
			Scopes:      []string{"read", "write"},
		},
		ToolName:   "test-tool",
		ClientID:   "test-client-id",
		ListenAddr: ":0",
		Timeout:    5 * time.Second,
		Store:      store,
		OpenBrowser: func(u string) error {
			authURLCh <- u
			return nil
		},
	}
}

// runLoginAndSimulateCallback starts Login in a goroutine, waits for the
// auth URL, simulates the callback with the given code, and returns the
// Login result. This is the common pattern for happy-path tests.
func runLoginAndSimulateCallback(t *testing.T, ctx context.Context, cfg LoginConfig, authURLCh <-chan string, code string) (*StoredToken, error) {
	t.Helper()

	var result *StoredToken
	var loginErr error
	done := make(chan struct{})

	go func() {
		defer close(done)
		result, loginErr = Login(ctx, cfg)
	}()

	// Wait for the auth URL from the browser callback.
	select {
	case authURL := <-authURLCh:
		err := simulateCallback(t, authURL, code, nil)
		if err != nil {
			t.Fatalf("simulateCallback failed: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for openBrowser to be called")
	}

	// Wait for Login to complete.
	select {
	case <-done:
		// ok
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for Login to complete")
	}

	return result, loginErr
}

// ---------------------------------------------------------------------------
// AC-14: OAuth login performs PKCE flow
// ---------------------------------------------------------------------------

func TestLogin_FullPKCEFlow_ReturnsToken(t *testing.T) {
	// AC-14: Complete login flow. Start login, capture auth URL, simulate
	// callback with correct state, verify token returned with correct fields.
	var exchangeLog tokenExchangeLog
	srv := newMockOAuthServer(t, func(log tokenExchangeLog) error {
		exchangeLog = log
		return nil
	})
	defer srv.Close()

	store := newFakeWritableTokenStore()
	authURLCh := make(chan string, 1)
	cfg := defaultLoginConfig(srv.URL, store, authURLCh)
	ctx := context.Background()

	result, err := runLoginAndSimulateCallback(t, ctx, cfg, authURLCh, "test-auth-code")

	require.NoError(t, err, "Login must succeed for a valid PKCE flow")
	require.NotNil(t, result, "Login must return a non-nil StoredToken")

	// Verify the returned token has the values from our mock server's response.
	assert.Equal(t, "mock-access-token-xyz", result.AccessToken,
		"AccessToken must match the mock server's response")
	assert.Equal(t, "mock-refresh-token-abc", result.RefreshToken,
		"RefreshToken must match the mock server's response")
	assert.Equal(t, "Bearer", result.TokenType,
		"TokenType must match the mock server's response")

	// Verify the token exchange included a code_verifier (PKCE).
	assert.NotEmpty(t, exchangeLog.CodeVerifier,
		"Token exchange must include a code_verifier parameter (PKCE)")
}

func TestLogin_AuthURL_HasS256CodeChallenge(t *testing.T) {
	// AC-14: Auth URL must include code_challenge with S256 method.
	srv := newMockOAuthServer(t, nil)
	defer srv.Close()

	store := newFakeWritableTokenStore()
	authURLCh := make(chan string, 1)
	cfg := defaultLoginConfig(srv.URL, store, authURLCh)
	ctx := context.Background()

	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = Login(ctx, cfg)
	}()

	var authURL string
	select {
	case authURL = <-authURLCh:
		// Got it
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for openBrowser to be called")
	}

	parsed, err := url.Parse(authURL)
	require.NoError(t, err, "auth URL must be parseable")

	q := parsed.Query()

	// code_challenge must be present and non-empty.
	codeChallenge := q.Get("code_challenge")
	assert.NotEmpty(t, codeChallenge,
		"Auth URL must contain a non-empty code_challenge parameter")

	// code_challenge_method must be "S256".
	codeChallengeMethod := q.Get("code_challenge_method")
	assert.Equal(t, "S256", codeChallengeMethod,
		"Auth URL must use code_challenge_method=S256, got %q", codeChallengeMethod)

	// Simulate callback to let Login complete and avoid goroutine leak.
	_ = simulateCallback(t, authURL, "cleanup-code", nil)
	<-done
}

func TestLogin_AuthURL_HasNonEmptyState(t *testing.T) {
	// AC-14: Auth URL must include a state parameter that is non-empty.
	srv := newMockOAuthServer(t, nil)
	defer srv.Close()

	store := newFakeWritableTokenStore()
	authURLCh := make(chan string, 1)
	cfg := defaultLoginConfig(srv.URL, store, authURLCh)
	ctx := context.Background()

	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = Login(ctx, cfg)
	}()

	var authURL string
	select {
	case authURL = <-authURLCh:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for openBrowser to be called")
	}

	parsed, err := url.Parse(authURL)
	require.NoError(t, err)

	state := parsed.Query().Get("state")
	assert.NotEmpty(t, state, "Auth URL must include a non-empty state parameter")

	// State should have sufficient entropy — at least 16 characters.
	assert.GreaterOrEqual(t, len(state), 16,
		"State parameter must have sufficient entropy (at least 16 chars), got %d", len(state))

	_ = simulateCallback(t, authURL, "cleanup-code", nil)
	<-done
}

func TestLogin_StateIsRandomPerCall(t *testing.T) {
	// AC-14: Each Login invocation must generate a unique state to prevent
	// CSRF attacks. A hardcoded state would fail this test.
	srv := newMockOAuthServer(t, nil)
	defer srv.Close()

	states := make([]string, 3)

	for i := 0; i < 3; i++ {
		store := newFakeWritableTokenStore()
		authURLCh := make(chan string, 1)
		cfg := defaultLoginConfig(srv.URL, store, authURLCh)
		ctx := context.Background()

		done := make(chan struct{})
		go func() {
			defer close(done)
			_, _ = Login(ctx, cfg)
		}()

		select {
		case authURL := <-authURLCh:
			parsed, err := url.Parse(authURL)
			require.NoError(t, err)
			states[i] = parsed.Query().Get("state")
			_ = simulateCallback(t, authURL, "code", nil)
		case <-time.After(3 * time.Second):
			t.Fatal("timed out waiting for openBrowser")
		}
		<-done
	}

	// All three states must be distinct.
	assert.NotEqual(t, states[0], states[1],
		"State must be randomly generated per call (call 0 vs 1)")
	assert.NotEqual(t, states[1], states[2],
		"State must be randomly generated per call (call 1 vs 2)")
	assert.NotEqual(t, states[0], states[2],
		"State must be randomly generated per call (call 0 vs 2)")
}

func TestLogin_CodeExchangeIncludesVerifier(t *testing.T) {
	// AC-14: The token exchange request must include code_verifier param.
	// The verifier must be non-empty and have sufficient length (per RFC 7636,
	// between 43 and 128 characters).
	var capturedVerifier string
	srv := newMockOAuthServer(t, func(log tokenExchangeLog) error {
		capturedVerifier = log.CodeVerifier
		if log.CodeVerifier == "" {
			return fmt.Errorf("missing code_verifier")
		}
		return nil
	})
	defer srv.Close()

	store := newFakeWritableTokenStore()
	authURLCh := make(chan string, 1)
	cfg := defaultLoginConfig(srv.URL, store, authURLCh)
	ctx := context.Background()

	_, err := runLoginAndSimulateCallback(t, ctx, cfg, authURLCh, "test-code")
	require.NoError(t, err, "Login must succeed when token endpoint accepts the verifier")

	// RFC 7636 Section 4.1: verifier is 43-128 characters.
	assert.GreaterOrEqual(t, len(capturedVerifier), 43,
		"code_verifier must be at least 43 characters (RFC 7636), got %d", len(capturedVerifier))
	assert.LessOrEqual(t, len(capturedVerifier), 128,
		"code_verifier must be at most 128 characters (RFC 7636), got %d", len(capturedVerifier))
}

func TestLogin_CodeChallengeMatchesVerifier(t *testing.T) {
	// AC-14: The code_challenge in the auth URL must be the S256 hash of
	// the code_verifier sent in the token exchange. This is the core PKCE
	// security property.
	var capturedVerifier string
	srv := newMockOAuthServer(t, func(log tokenExchangeLog) error {
		capturedVerifier = log.CodeVerifier
		return nil
	})
	defer srv.Close()

	store := newFakeWritableTokenStore()
	authURLCh := make(chan string, 1)
	cfg := defaultLoginConfig(srv.URL, store, authURLCh)
	ctx := context.Background()

	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = Login(ctx, cfg)
	}()

	var capturedChallenge string
	select {
	case authURL := <-authURLCh:
		parsed, err := url.Parse(authURL)
		require.NoError(t, err)
		capturedChallenge = parsed.Query().Get("code_challenge")
		_ = simulateCallback(t, authURL, "test-code", nil)
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for openBrowser")
	}
	<-done

	require.NotEmpty(t, capturedVerifier, "must have captured code_verifier")
	require.NotEmpty(t, capturedChallenge, "must have captured code_challenge")

	// Compute expected S256 challenge from the verifier.
	hash := sha256.Sum256([]byte(capturedVerifier))
	expectedChallenge := base64.RawURLEncoding.EncodeToString(hash[:])

	assert.Equal(t, expectedChallenge, capturedChallenge,
		"code_challenge must be base64url(SHA256(code_verifier))")
}

func TestLogin_TokenStoredInStore(t *testing.T) {
	// AC-14: The resulting token must be stored in the token store.
	srv := newMockOAuthServer(t, nil)
	defer srv.Close()

	store := newFakeWritableTokenStore()
	authURLCh := make(chan string, 1)
	cfg := defaultLoginConfig(srv.URL, store, authURLCh)
	ctx := context.Background()

	result, err := runLoginAndSimulateCallback(t, ctx, cfg, authURLCh, "test-code")
	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify the token was stored under the tool name.
	stored, err := store.Get("test-tool")
	require.NoError(t, err, "Token must be stored in the store under the tool name")
	require.NotNil(t, stored)

	assert.Equal(t, result.AccessToken, stored.AccessToken,
		"Stored token's AccessToken must match the returned token")
	assert.Equal(t, result.RefreshToken, stored.RefreshToken,
		"Stored token's RefreshToken must match the returned token")
	assert.Equal(t, result.TokenType, stored.TokenType,
		"Stored token's TokenType must match the returned token")
}

func TestLogin_TokenFieldsPopulated(t *testing.T) {
	// AC-14: Returned token must have AccessToken, RefreshToken, and Expiry
	// populated from the OAuth response. Expiry should be approximately
	// now + expires_in seconds.
	srv := newMockOAuthServer(t, nil)
	defer srv.Close()

	store := newFakeWritableTokenStore()
	authURLCh := make(chan string, 1)
	cfg := defaultLoginConfig(srv.URL, store, authURLCh)
	ctx := context.Background()

	beforeLogin := time.Now()
	result, err := runLoginAndSimulateCallback(t, ctx, cfg, authURLCh, "test-code")
	afterLogin := time.Now()

	require.NoError(t, err)
	require.NotNil(t, result)

	assert.Equal(t, "mock-access-token-xyz", result.AccessToken)
	assert.Equal(t, "mock-refresh-token-abc", result.RefreshToken)

	// Expiry should be approximately now + 3600 seconds (from mock response).
	// Allow a 30-second window to account for test execution time.
	assert.False(t, result.Expiry.IsZero(), "Expiry must not be zero when expires_in is provided")
	expectedEarliestExpiry := beforeLogin.Add(3600 * time.Second)
	expectedLatestExpiry := afterLogin.Add(3600 * time.Second)
	assert.True(t, result.Expiry.After(expectedEarliestExpiry.Add(-30*time.Second)),
		"Expiry %v should be approximately now+3600s, earliest acceptable: %v",
		result.Expiry, expectedEarliestExpiry.Add(-30*time.Second))
	assert.True(t, result.Expiry.Before(expectedLatestExpiry.Add(30*time.Second)),
		"Expiry %v should be approximately now+3600s, latest acceptable: %v",
		result.Expiry, expectedLatestExpiry.Add(30*time.Second))
}

func TestLogin_AuthURL_ContainsClientID(t *testing.T) {
	// The auth URL must contain the client_id parameter matching cfg.ClientID.
	srv := newMockOAuthServer(t, nil)
	defer srv.Close()

	store := newFakeWritableTokenStore()
	authURLCh := make(chan string, 1)
	cfg := defaultLoginConfig(srv.URL, store, authURLCh)
	cfg.ClientID = "my-specific-client-id"
	ctx := context.Background()

	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = Login(ctx, cfg)
	}()

	select {
	case authURL := <-authURLCh:
		parsed, err := url.Parse(authURL)
		require.NoError(t, err)
		assert.Equal(t, "my-specific-client-id", parsed.Query().Get("client_id"),
			"Auth URL must include the configured client_id")
		_ = simulateCallback(t, authURL, "code", nil)
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for openBrowser")
	}
	<-done
}

func TestLogin_AuthURL_ContainsScopes(t *testing.T) {
	// Auth URL must include scopes from the Auth config.
	srv := newMockOAuthServer(t, nil)
	defer srv.Close()

	store := newFakeWritableTokenStore()
	authURLCh := make(chan string, 1)
	cfg := defaultLoginConfig(srv.URL, store, authURLCh)
	cfg.Auth.Scopes = []string{"openid", "profile", "email"}
	ctx := context.Background()

	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = Login(ctx, cfg)
	}()

	select {
	case authURL := <-authURLCh:
		parsed, err := url.Parse(authURL)
		require.NoError(t, err)

		scopeParam := parsed.Query().Get("scope")
		assert.NotEmpty(t, scopeParam, "Auth URL must include a scope parameter")

		// Verify all configured scopes are present.
		for _, s := range []string{"openid", "profile", "email"} {
			assert.Contains(t, scopeParam, s,
				"Auth URL scope parameter must contain %q", s)
		}
		_ = simulateCallback(t, authURL, "code", nil)
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for openBrowser")
	}
	<-done
}

func TestLogin_AuthURL_ResponseTypeIsCode(t *testing.T) {
	// The auth URL must request response_type=code for authorization code flow.
	srv := newMockOAuthServer(t, nil)
	defer srv.Close()

	store := newFakeWritableTokenStore()
	authURLCh := make(chan string, 1)
	cfg := defaultLoginConfig(srv.URL, store, authURLCh)
	ctx := context.Background()

	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = Login(ctx, cfg)
	}()

	select {
	case authURL := <-authURLCh:
		parsed, err := url.Parse(authURL)
		require.NoError(t, err)
		assert.Equal(t, "code", parsed.Query().Get("response_type"),
			"Auth URL must have response_type=code")
		_ = simulateCallback(t, authURL, "code", nil)
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for openBrowser")
	}
	<-done
}

// ---------------------------------------------------------------------------
// AC-15: OAuth callback server handles errors
// ---------------------------------------------------------------------------

func TestLogin_StateMismatch_ReturnsError(t *testing.T) {
	// AC-15: State mismatch on callback must return an error containing
	// "state mismatch".
	srv := newMockOAuthServer(t, nil)
	defer srv.Close()

	store := newFakeWritableTokenStore()
	authURLCh := make(chan string, 1)
	cfg := defaultLoginConfig(srv.URL, store, authURLCh)
	ctx := context.Background()

	var loginErr error
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, loginErr = Login(ctx, cfg)
	}()

	select {
	case authURL := <-authURLCh:
		// Simulate callback with a WRONG state.
		wrongState := "completely-wrong-state-value"
		err := simulateCallback(t, authURL, "test-code", &wrongState)
		// The callback itself might return an error response, that's fine.
		_ = err
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for openBrowser")
	}

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for Login to complete after state mismatch")
	}

	require.Error(t, loginErr, "Login must return error on state mismatch")
	assert.Contains(t, loginErr.Error(), "state mismatch",
		"Error must contain 'state mismatch', got: %q", loginErr.Error())
}

func TestLogin_Timeout_ReturnsError(t *testing.T) {
	// AC-15: No callback within the timeout period must return an error
	// containing "timed out".
	srv := newMockOAuthServer(t, nil)
	defer srv.Close()

	store := newFakeWritableTokenStore()
	authURLCh := make(chan string, 1)
	cfg := defaultLoginConfig(srv.URL, store, authURLCh)
	cfg.Timeout = 200 * time.Millisecond // Very short timeout for testing.
	ctx := context.Background()

	var result *StoredToken
	var loginErr error
	done := make(chan struct{})
	go func() {
		defer close(done)
		result, loginErr = Login(ctx, cfg)
	}()

	// Wait for the browser to be called but do NOT simulate the callback.
	select {
	case <-authURLCh:
		// Intentionally do nothing — let it time out.
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for openBrowser to be called")
	}

	// Wait for Login to complete (should be after 200ms timeout).
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Login did not return after timeout; possible goroutine leak")
	}

	require.Error(t, loginErr, "Login must return error when callback times out")
	assert.Contains(t, loginErr.Error(), "timed out",
		"Timeout error must contain 'timed out', got: %q", loginErr.Error())
	assert.Nil(t, result, "Result must be nil on timeout")
}

func TestLogin_CallbackServerShutsDown_AfterSuccess(t *testing.T) {
	// AC-15: After successful callback, the callback server must stop
	// accepting connections.
	srv := newMockOAuthServer(t, nil)
	defer srv.Close()

	store := newFakeWritableTokenStore()
	authURLCh := make(chan string, 1)
	cfg := defaultLoginConfig(srv.URL, store, authURLCh)
	ctx := context.Background()

	done := make(chan struct{})
	var authURL string
	go func() {
		defer close(done)
		_, _ = Login(ctx, cfg)
	}()

	select {
	case authURL = <-authURLCh:
		_ = simulateCallback(t, authURL, "test-code", nil)
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for openBrowser")
	}

	<-done

	// Extract the redirect URI (callback server address) from the auth URL.
	parsed, err := url.Parse(authURL)
	require.NoError(t, err)
	redirectURI := parsed.Query().Get("redirect_uri")
	require.NotEmpty(t, redirectURI, "auth URL must have redirect_uri")

	// Attempt another connection to the callback server — it should fail.
	// Give it a short time to shut down.
	time.Sleep(100 * time.Millisecond)
	client := &http.Client{Timeout: 1 * time.Second}
	_, err = client.Get(redirectURI + "?code=sneaky&state=bad")
	assert.Error(t, err,
		"Callback server must be shut down after successful callback; expected connection error")
}

func TestLogin_CallbackServerShutsDown_AfterTimeout(t *testing.T) {
	// AC-15: After timeout, the callback server must also shut down.
	srv := newMockOAuthServer(t, nil)
	defer srv.Close()

	store := newFakeWritableTokenStore()
	authURLCh := make(chan string, 1)
	cfg := defaultLoginConfig(srv.URL, store, authURLCh)
	cfg.Timeout = 200 * time.Millisecond
	ctx := context.Background()

	done := make(chan struct{})
	var authURL string
	go func() {
		defer close(done)
		_, _ = Login(ctx, cfg)
	}()

	select {
	case authURL = <-authURLCh:
		// Don't callback — let it time out.
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for openBrowser")
	}

	<-done

	// The callback server should be shut down.
	parsed, err := url.Parse(authURL)
	require.NoError(t, err)
	redirectURI := parsed.Query().Get("redirect_uri")
	require.NotEmpty(t, redirectURI)

	time.Sleep(100 * time.Millisecond)
	client := &http.Client{Timeout: 1 * time.Second}
	_, err = client.Get(redirectURI + "?code=late&state=late")
	assert.Error(t, err,
		"Callback server must be shut down after timeout")
}

// ---------------------------------------------------------------------------
// AC-16: OAuth callback server port selection
// ---------------------------------------------------------------------------

func TestLogin_ListenAddr_UsesSpecifiedAddr(t *testing.T) {
	// AC-16: When ListenAddr is ":0", server starts on a random port.
	// The redirect_uri in the auth URL must reflect the actual port.
	srv := newMockOAuthServer(t, nil)
	defer srv.Close()

	store := newFakeWritableTokenStore()
	authURLCh := make(chan string, 1)
	cfg := defaultLoginConfig(srv.URL, store, authURLCh)
	cfg.ListenAddr = ":0"
	ctx := context.Background()

	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = Login(ctx, cfg)
	}()

	select {
	case authURL := <-authURLCh:
		parsed, err := url.Parse(authURL)
		require.NoError(t, err)

		redirectURI := parsed.Query().Get("redirect_uri")
		require.NotEmpty(t, redirectURI, "Auth URL must have redirect_uri")

		redirectParsed, err := url.Parse(redirectURI)
		require.NoError(t, err)

		port := redirectParsed.Port()
		assert.NotEmpty(t, port, "redirect_uri must include a port")
		assert.NotEqual(t, "0", port, "redirect_uri must use the actual port, not 0")

		// Port should be a valid number.
		portNum, err := strconv.Atoi(port)
		require.NoError(t, err, "Port must be a valid number")
		assert.Greater(t, portNum, 0, "Port must be greater than 0")
		assert.LessOrEqual(t, portNum, 65535, "Port must be a valid TCP port")

		_ = simulateCallback(t, authURL, "code", nil)
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for openBrowser")
	}
	<-done
}

func TestLogin_RedirectURI_ReflectsActualPort(t *testing.T) {
	// AC-16: The redirect_uri in the auth URL must be reachable (i.e., it
	// reflects the actual port the server is listening on).
	srv := newMockOAuthServer(t, nil)
	defer srv.Close()

	store := newFakeWritableTokenStore()
	authURLCh := make(chan string, 1)
	cfg := defaultLoginConfig(srv.URL, store, authURLCh)
	cfg.ListenAddr = ":0"
	ctx := context.Background()

	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = Login(ctx, cfg)
	}()

	select {
	case authURL := <-authURLCh:
		parsed, err := url.Parse(authURL)
		require.NoError(t, err)

		redirectURI := parsed.Query().Get("redirect_uri")
		require.NotEmpty(t, redirectURI)

		// The redirect_uri must point to 127.0.0.1 or localhost.
		redirectParsed, err := url.Parse(redirectURI)
		require.NoError(t, err)
		hostname := redirectParsed.Hostname()
		assert.True(t,
			hostname == "127.0.0.1" || hostname == "localhost",
			"redirect_uri must point to localhost or 127.0.0.1, got %q", hostname)

		// The callback must actually be reachable at this URL.
		state := parsed.Query().Get("state")
		callbackURL := fmt.Sprintf("%s?code=test&state=%s", redirectURI, url.QueryEscape(state))
		resp, err := http.Get(callbackURL)
		require.NoError(t, err, "Callback server must be reachable at the redirect_uri")
		resp.Body.Close()
		assert.Less(t, resp.StatusCode, 500,
			"Callback server must respond without 5xx errors")
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for openBrowser")
	}
	<-done
}

func TestLogin_DefaultListenAddr_FallbackOnPortConflict(t *testing.T) {
	// AC-16: When the default port (8085) is in use, Login should fall back
	// to an OS-assigned port. We simulate this by occupying port 8085.
	//
	// Occupy port 8085 to force fallback.
	blocker, err := net.Listen("tcp", "127.0.0.1:8085")
	if err != nil {
		t.Skip("cannot occupy port 8085 for testing; port already in use by another process")
	}
	defer blocker.Close()

	srv := newMockOAuthServer(t, nil)
	defer srv.Close()

	store := newFakeWritableTokenStore()
	authURLCh := make(chan string, 1)
	cfg := defaultLoginConfig(srv.URL, store, authURLCh)
	cfg.ListenAddr = "127.0.0.1:8085" // Explicitly request the blocked port.
	ctx := context.Background()

	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = Login(ctx, cfg)
	}()

	select {
	case authURL := <-authURLCh:
		parsed, err := url.Parse(authURL)
		require.NoError(t, err)

		redirectURI := parsed.Query().Get("redirect_uri")
		require.NotEmpty(t, redirectURI)

		redirectParsed, err := url.Parse(redirectURI)
		require.NoError(t, err)

		port := redirectParsed.Port()
		assert.NotEmpty(t, port, "redirect_uri must have a port")
		assert.NotEqual(t, "8085", port,
			"When port 8085 is occupied, Login must fall back to a different port, got 8085")

		_ = simulateCallback(t, authURL, "code", nil)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for openBrowser -- Login may have failed to bind a port")
	}
	<-done
}

// ---------------------------------------------------------------------------
// Edge cases
// ---------------------------------------------------------------------------

func TestLogin_ContextCancellation_ReturnsError(t *testing.T) {
	// Edge case: Cancelling the context before the callback should cause
	// Login to return an error.
	srv := newMockOAuthServer(t, nil)
	defer srv.Close()

	store := newFakeWritableTokenStore()
	authURLCh := make(chan string, 1)
	cfg := defaultLoginConfig(srv.URL, store, authURLCh)
	cfg.Timeout = 10 * time.Second // Long timeout so cancellation beats it.
	ctx, cancel := context.WithCancel(context.Background())

	var loginErr error
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, loginErr = Login(ctx, cfg)
	}()

	// Wait for the browser to be called, then cancel the context.
	select {
	case <-authURLCh:
		cancel()
	case <-time.After(3 * time.Second):
		cancel()
		t.Fatal("timed out waiting for openBrowser")
	}

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Login did not return after context cancellation")
	}

	require.Error(t, loginErr, "Login must return error when context is cancelled")
}

func TestLogin_OpenBrowserCalledWithWellFormedURL(t *testing.T) {
	// Edge case: The URL passed to openBrowser must be a valid, well-formed URL.
	srv := newMockOAuthServer(t, nil)
	defer srv.Close()

	store := newFakeWritableTokenStore()
	authURLCh := make(chan string, 1)
	cfg := defaultLoginConfig(srv.URL, store, authURLCh)
	ctx := context.Background()

	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = Login(ctx, cfg)
	}()

	select {
	case authURL := <-authURLCh:
		parsed, err := url.Parse(authURL)
		require.NoError(t, err, "URL passed to openBrowser must be parseable")

		// Must have a scheme.
		assert.True(t, parsed.Scheme == "http" || parsed.Scheme == "https",
			"Auth URL must have http or https scheme, got %q", parsed.Scheme)
		// Must have a host.
		assert.NotEmpty(t, parsed.Host, "Auth URL must have a host")
		// Must have required OAuth parameters.
		q := parsed.Query()
		assert.NotEmpty(t, q.Get("response_type"), "Auth URL must have response_type")
		assert.NotEmpty(t, q.Get("client_id"), "Auth URL must have client_id")
		assert.NotEmpty(t, q.Get("redirect_uri"), "Auth URL must have redirect_uri")
		assert.NotEmpty(t, q.Get("state"), "Auth URL must have state")
		assert.NotEmpty(t, q.Get("code_challenge"), "Auth URL must have code_challenge")
		assert.NotEmpty(t, q.Get("code_challenge_method"), "Auth URL must have code_challenge_method")

		_ = simulateCallback(t, authURL, "code", nil)
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for openBrowser")
	}
	<-done
}

func TestLogin_OpenBrowserError_LoginFails(t *testing.T) {
	// Edge case: If openBrowser returns an error, Login must propagate it.
	srv := newMockOAuthServer(t, nil)
	defer srv.Close()

	store := newFakeWritableTokenStore()
	browserErr := fmt.Errorf("xdg-open: command not found")
	cfg := LoginConfig{
		Auth: manifest.Auth{
			Type:        "oauth",
			ProviderURL: srv.URL,
			Scopes:      []string{"read"},
		},
		ToolName:   "test-tool",
		ClientID:   "test-client-id",
		ListenAddr: ":0",
		Timeout:    5 * time.Second,
		Store:      store,
		OpenBrowser: func(u string) error {
			return browserErr
		},
	}

	result, err := Login(context.Background(), cfg)
	require.Error(t, err, "Login must return error when openBrowser fails")
	assert.Nil(t, result, "Result must be nil when openBrowser fails")
	assert.Contains(t, err.Error(), "xdg-open",
		"Error should contain the browser error message")
}

func TestLogin_TokenExchangeError_LoginFails(t *testing.T) {
	// Edge case: Token exchange failure should propagate as a Login error.
	srv := newMockOAuthServer(t, func(log tokenExchangeLog) error {
		return fmt.Errorf("invalid_grant: authorization code expired")
	})
	defer srv.Close()

	store := newFakeWritableTokenStore()
	authURLCh := make(chan string, 1)
	cfg := defaultLoginConfig(srv.URL, store, authURLCh)
	ctx := context.Background()

	var loginErr error
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, loginErr = Login(ctx, cfg)
	}()

	select {
	case authURL := <-authURLCh:
		_ = simulateCallback(t, authURL, "test-code", nil)
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for openBrowser")
	}

	<-done
	require.Error(t, loginErr, "Login must return error when token exchange fails")
}

func TestLogin_GrantTypeIsAuthorizationCode(t *testing.T) {
	// The token exchange must use grant_type=authorization_code.
	var capturedGrantType string
	srv := newMockOAuthServer(t, func(log tokenExchangeLog) error {
		capturedGrantType = log.GrantType
		return nil
	})
	defer srv.Close()

	store := newFakeWritableTokenStore()
	authURLCh := make(chan string, 1)
	cfg := defaultLoginConfig(srv.URL, store, authURLCh)
	ctx := context.Background()

	_, err := runLoginAndSimulateCallback(t, ctx, cfg, authURLCh, "test-code")
	require.NoError(t, err)

	assert.Equal(t, "authorization_code", capturedGrantType,
		"Token exchange must use grant_type=authorization_code")
}

func TestLogin_TokenExchangeIncludesCode(t *testing.T) {
	// The authorization code from the callback must be forwarded to the
	// token exchange.
	var capturedCode string
	srv := newMockOAuthServer(t, func(log tokenExchangeLog) error {
		capturedCode = log.Code
		return nil
	})
	defer srv.Close()

	store := newFakeWritableTokenStore()
	authURLCh := make(chan string, 1)
	cfg := defaultLoginConfig(srv.URL, store, authURLCh)
	ctx := context.Background()

	_, err := runLoginAndSimulateCallback(t, ctx, cfg, authURLCh, "my-unique-auth-code-12345")
	require.NoError(t, err)

	assert.Equal(t, "my-unique-auth-code-12345", capturedCode,
		"Token exchange must forward the exact authorization code from the callback")
}

func TestLogin_TokenExchangeIncludesRedirectURI(t *testing.T) {
	// The token exchange must include redirect_uri matching the one from
	// the authorization request.
	var capturedRedirectURI string
	srv := newMockOAuthServer(t, func(log tokenExchangeLog) error {
		capturedRedirectURI = log.RedirectURI
		return nil
	})
	defer srv.Close()

	store := newFakeWritableTokenStore()
	authURLCh := make(chan string, 1)
	cfg := defaultLoginConfig(srv.URL, store, authURLCh)
	ctx := context.Background()

	done := make(chan struct{})
	var authRedirectURI string
	go func() {
		defer close(done)
		_, _ = Login(ctx, cfg)
	}()

	select {
	case authURL := <-authURLCh:
		parsed, err := url.Parse(authURL)
		require.NoError(t, err)
		authRedirectURI = parsed.Query().Get("redirect_uri")
		_ = simulateCallback(t, authURL, "code", nil)
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for openBrowser")
	}
	<-done

	assert.NotEmpty(t, capturedRedirectURI, "Token exchange must include redirect_uri")
	assert.Equal(t, authRedirectURI, capturedRedirectURI,
		"Token exchange redirect_uri must match the one from the auth URL")
}

func TestLogin_TokenWithNoRefreshToken(t *testing.T) {
	// Edge case: Some OAuth providers don't return a refresh_token.
	// Login should still succeed.
	mux := http.NewServeMux()
	var srvURL string
	mux.HandleFunc("/.well-known/oauth-authorization-server", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{
			"authorization_endpoint": %q,
			"token_endpoint": %q
		}`, srvURL+"/authorize", srvURL+"/token")
	})
	mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// No refresh_token in response.
		fmt.Fprintf(w, `{
			"access_token": "access-no-refresh",
			"token_type": "Bearer",
			"expires_in": 1800
		}`)
	})
	oauthSrv := httptest.NewServer(mux)
	srvURL = oauthSrv.URL
	defer oauthSrv.Close()

	store := newFakeWritableTokenStore()
	authURLCh := make(chan string, 1)
	cfg := defaultLoginConfig(oauthSrv.URL, store, authURLCh)
	ctx := context.Background()

	result, err := runLoginAndSimulateCallback(t, ctx, cfg, authURLCh, "code")
	require.NoError(t, err, "Login must succeed even without a refresh_token")
	require.NotNil(t, result)

	assert.Equal(t, "access-no-refresh", result.AccessToken)
	assert.Empty(t, result.RefreshToken,
		"RefreshToken should be empty when server doesn't return one")
}

func TestLogin_DiscoverEndpointsIntegration(t *testing.T) {
	// Integration test: Login must use DiscoverEndpoints to find the token
	// endpoint from the provider URL. If the discovery fails, Login should
	// fail too.
	mux := http.NewServeMux()
	// Serve NO discovery documents — all well-known paths return 404.
	srv := httptest.NewServer(mux)
	defer srv.Close()

	store := newFakeWritableTokenStore()
	authURLCh := make(chan string, 1)
	cfg := defaultLoginConfig(srv.URL, store, authURLCh)
	// No manual endpoints either.
	cfg.Auth.Endpoints = nil
	ctx := context.Background()

	result, err := Login(ctx, cfg)
	require.Error(t, err, "Login must fail when endpoint discovery fails")
	assert.Nil(t, result, "Result must be nil when discovery fails")
}

func TestLogin_ManualEndpoints_UsedWhenDiscoveryFails(t *testing.T) {
	// When discovery fails but manual endpoints are provided, Login should
	// use the manual endpoints successfully.
	mux := http.NewServeMux()
	var srvURL string

	// This server has no well-known endpoints, but does have a /token endpoint.
	mux.HandleFunc("/manual-token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{
			"access_token": "manual-endpoint-token",
			"refresh_token": "manual-refresh",
			"token_type": "Bearer",
			"expires_in": 3600
		}`)
	})
	srv := httptest.NewServer(mux)
	srvURL = srv.URL
	defer srv.Close()

	store := newFakeWritableTokenStore()
	authURLCh := make(chan string, 1)
	cfg := defaultLoginConfig(srv.URL, store, authURLCh)
	cfg.Auth.Endpoints = &manifest.Endpoints{
		Authorization: srvURL + "/manual-authorize",
		Token:         srvURL + "/manual-token",
	}
	ctx := context.Background()

	result, err := runLoginAndSimulateCallback(t, ctx, cfg, authURLCh, "code")
	require.NoError(t, err, "Login must succeed with manual endpoints")
	require.NotNil(t, result)
	assert.Equal(t, "manual-endpoint-token", result.AccessToken)
}

func TestLogin_HTTPClientUsedForTokenExchange(t *testing.T) {
	// When HTTPClient is set, it should be used for the token exchange,
	// allowing tests to intercept the request. This verifies the test
	// seam works correctly.
	srv := newMockOAuthServer(t, nil)
	defer srv.Close()

	// Create a custom HTTP client that records whether it was used.
	var clientUsed atomic.Bool
	customTransport := &recordingTransport{
		wrapped: http.DefaultTransport,
		onRequest: func(req *http.Request) {
			clientUsed.Store(true)
		},
	}

	store := newFakeWritableTokenStore()
	authURLCh := make(chan string, 1)
	cfg := defaultLoginConfig(srv.URL, store, authURLCh)
	cfg.HTTPClient = &http.Client{Transport: customTransport}
	ctx := context.Background()

	_, err := runLoginAndSimulateCallback(t, ctx, cfg, authURLCh, "code")
	require.NoError(t, err)

	// The custom HTTP client should have been used for something
	// (either discovery or token exchange).
	assert.True(t, clientUsed.Load(),
		"HTTPClient from LoginConfig must be used for HTTP requests")
}

// recordingTransport wraps an http.RoundTripper and calls onRequest for
// each request.
type recordingTransport struct {
	wrapped   http.RoundTripper
	onRequest func(req *http.Request)
}

func (rt *recordingTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if rt.onRequest != nil {
		rt.onRequest(req)
	}
	return rt.wrapped.RoundTrip(req)
}

func TestLogin_MultipleDistinctTokens_AntiHardcoding(t *testing.T) {
	// Anti-hardcoding: Run two Login flows with different mock servers
	// returning different tokens. Verify each Login returns its own token.
	tokens := []struct {
		access  string
		refresh string
	}{
		{"access-alpha-111", "refresh-alpha-111"},
		{"access-beta-222", "refresh-beta-222"},
	}

	for i, expected := range tokens {
		t.Run(fmt.Sprintf("token-%d", i), func(t *testing.T) {
			access := expected.access
			refresh := expected.refresh

			mux := http.NewServeMux()
			var srvURL string
			mux.HandleFunc("/.well-known/oauth-authorization-server", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprintf(w, `{
					"authorization_endpoint": %q,
					"token_endpoint": %q
				}`, srvURL+"/authorize", srvURL+"/token")
			})
			mux.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprintf(w, `{
					"access_token": %q,
					"refresh_token": %q,
					"token_type": "Bearer",
					"expires_in": 3600
				}`, access, refresh)
			})
			oauthSrv := httptest.NewServer(mux)
			srvURL = oauthSrv.URL
			defer oauthSrv.Close()

			store := newFakeWritableTokenStore()
			authURLCh := make(chan string, 1)
			cfg := defaultLoginConfig(oauthSrv.URL, store, authURLCh)
			ctx := context.Background()

			result, err := runLoginAndSimulateCallback(t, ctx, cfg, authURLCh, "code")
			require.NoError(t, err)
			require.NotNil(t, result)

			assert.Equal(t, access, result.AccessToken,
				"Token %d: AccessToken must match the server's response", i)
			assert.Equal(t, refresh, result.RefreshToken,
				"Token %d: RefreshToken must match the server's response", i)
		})
	}
}

func TestLogin_VerifierIsUniquePerCall(t *testing.T) {
	// Anti-hardcoding: Each Login call must generate a unique PKCE verifier.
	var verifiers []string
	var mu sync.Mutex

	srv := newMockOAuthServer(t, func(log tokenExchangeLog) error {
		mu.Lock()
		verifiers = append(verifiers, log.CodeVerifier)
		mu.Unlock()
		return nil
	})
	defer srv.Close()

	for i := 0; i < 3; i++ {
		store := newFakeWritableTokenStore()
		authURLCh := make(chan string, 1)
		cfg := defaultLoginConfig(srv.URL, store, authURLCh)
		ctx := context.Background()

		_, err := runLoginAndSimulateCallback(t, ctx, cfg, authURLCh, "code")
		require.NoError(t, err, "Login %d must succeed", i)
	}

	require.Len(t, verifiers, 3, "Must have captured 3 verifiers")
	assert.NotEqual(t, verifiers[0], verifiers[1], "Verifiers must differ (call 0 vs 1)")
	assert.NotEqual(t, verifiers[1], verifiers[2], "Verifiers must differ (call 1 vs 2)")
	assert.NotEqual(t, verifiers[0], verifiers[2], "Verifiers must differ (call 0 vs 2)")
}

func TestLogin_StateMismatch_FullErrorMessage(t *testing.T) {
	// AC-15: The exact error message for state mismatch must contain the
	// specified string: "Security check failed (state mismatch)".
	srv := newMockOAuthServer(t, nil)
	defer srv.Close()

	store := newFakeWritableTokenStore()
	authURLCh := make(chan string, 1)
	cfg := defaultLoginConfig(srv.URL, store, authURLCh)
	ctx := context.Background()

	var loginErr error
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, loginErr = Login(ctx, cfg)
	}()

	select {
	case authURL := <-authURLCh:
		wrongState := "wrong-state-entirely"
		_ = simulateCallback(t, authURL, "code", &wrongState)
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for openBrowser")
	}
	<-done

	require.Error(t, loginErr)
	assert.Contains(t, loginErr.Error(), "Security check failed (state mismatch)",
		"Error must contain the exact phrase per spec; got: %q", loginErr.Error())
}

func TestLogin_Timeout_FullErrorMessage(t *testing.T) {
	// AC-15: The timeout error must contain: "Login cancelled or timed out".
	srv := newMockOAuthServer(t, nil)
	defer srv.Close()

	store := newFakeWritableTokenStore()
	authURLCh := make(chan string, 1)
	cfg := defaultLoginConfig(srv.URL, store, authURLCh)
	cfg.Timeout = 200 * time.Millisecond
	ctx := context.Background()

	var loginErr error
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, loginErr = Login(ctx, cfg)
	}()

	<-authURLCh // Don't simulate callback.
	<-done

	require.Error(t, loginErr)
	assert.Contains(t, loginErr.Error(), "Login cancelled or timed out",
		"Error must contain the exact phrase per spec; got: %q", loginErr.Error())
}

func TestLogin_CallbackPath_IsWellKnown(t *testing.T) {
	// The callback server should handle requests on the /callback path
	// (or whatever path the redirect_uri points to).
	srv := newMockOAuthServer(t, nil)
	defer srv.Close()

	store := newFakeWritableTokenStore()
	authURLCh := make(chan string, 1)
	cfg := defaultLoginConfig(srv.URL, store, authURLCh)
	ctx := context.Background()

	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = Login(ctx, cfg)
	}()

	select {
	case authURL := <-authURLCh:
		parsed, err := url.Parse(authURL)
		require.NoError(t, err)

		redirectURI := parsed.Query().Get("redirect_uri")
		require.NotEmpty(t, redirectURI)

		// The redirect_uri must be a valid URL we can actually call.
		redirectParsed, err := url.Parse(redirectURI)
		require.NoError(t, err)
		assert.NotEmpty(t, redirectParsed.Scheme, "redirect_uri must have a scheme")

		_ = simulateCallback(t, authURL, "code", nil)
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for openBrowser")
	}
	<-done
}

func TestLogin_NilStore_DoesNotPanic(t *testing.T) {
	// Edge case: If Store is nil, Login should still work but may skip
	// storing the token. It must not panic.
	srv := newMockOAuthServer(t, nil)
	defer srv.Close()

	authURLCh := make(chan string, 1)
	cfg := defaultLoginConfig(srv.URL, nil, authURLCh)
	cfg.Store = nil
	ctx := context.Background()

	assert.NotPanics(t, func() {
		done := make(chan struct{})
		go func() {
			defer close(done)
			_, _ = Login(ctx, cfg)
		}()

		select {
		case authURL := <-authURLCh:
			_ = simulateCallback(t, authURL, "code", nil)
		case <-time.After(3 * time.Second):
			// May fail early if nil store causes error, that's OK.
		}

		select {
		case <-done:
		case <-time.After(5 * time.Second):
		}
	}, "Login must not panic even with nil Store")
}
