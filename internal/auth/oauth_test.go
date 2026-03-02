package auth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

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
