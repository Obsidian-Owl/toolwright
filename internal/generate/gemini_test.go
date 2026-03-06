package generate

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Compile-time interface check: GeminiProvider must implement LLMProvider.
// ---------------------------------------------------------------------------

var _ LLMProvider = (*GeminiProvider)(nil)

// ---------------------------------------------------------------------------
// AC-2: Name() and DefaultModel()
// ---------------------------------------------------------------------------

func TestGeminiProvider_Name(t *testing.T) {
	p := NewGeminiProvider("test-key", nil)
	assert.Equal(t, "gemini", p.Name())
}

func TestGeminiProvider_DefaultModel(t *testing.T) {
	p := NewGeminiProvider("test-key", nil)
	assert.Equal(t, "gemini-2.0-flash", p.DefaultModel())
}

// ---------------------------------------------------------------------------
// AC-2, AC-12: Complete sends correct headers and body via httptest server.
// ---------------------------------------------------------------------------

func TestGeminiProvider_Complete_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := `{"candidates":[{"content":{"parts":[{"text":"hello world"}]}}]}`
		_, _ = w.Write([]byte(resp))
	}))
	defer srv.Close()

	p := newGeminiProviderWithURL("test-key", srv.Client(), srv.URL)
	result, err := p.Complete(context.Background(), "say hello", "")
	require.NoError(t, err)
	assert.Equal(t, "hello world", result)
}

func TestGeminiProvider_Complete_APIKeyAsQueryParam(t *testing.T) {
	var mu sync.Mutex
	var capturedHeaders http.Header
	var capturedQuery string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		capturedHeaders = r.Header.Clone()
		capturedQuery = r.URL.Query().Get("key")
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"ok"}]}}]}`))
	}))
	defer srv.Close()

	p := newGeminiProviderWithURL("my-gemini-key", srv.Client(), srv.URL)
	_, err := p.Complete(context.Background(), "test prompt", "")
	require.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()

	assert.Equal(t, "my-gemini-key", capturedQuery,
		"API key must be sent as ?key= query parameter")
	assert.Empty(t, capturedHeaders.Get("Authorization"),
		"Gemini must NOT use Authorization header")
	assert.Equal(t, "application/json", capturedHeaders.Get("Content-Type"),
		"must send Content-Type application/json")
}

func TestGeminiProvider_Complete_NoAuthorizationHeader(t *testing.T) {
	var mu sync.Mutex
	var capturedHeaders http.Header

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		capturedHeaders = r.Header.Clone()
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"ok"}]}}]}`))
	}))
	defer srv.Close()

	p := newGeminiProviderWithURL("test-key", srv.Client(), srv.URL)
	_, err := p.Complete(context.Background(), "test", "")
	require.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()

	assert.Empty(t, capturedHeaders.Get("Authorization"),
		"Gemini must NOT set Authorization header (uses query param)")
	assert.Empty(t, capturedHeaders.Get("x-api-key"),
		"Gemini must NOT set x-api-key header")
}

func TestGeminiProvider_Complete_SendsCorrectBody(t *testing.T) {
	var mu sync.Mutex
	var capturedBody []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		capturedBody, _ = io.ReadAll(r.Body)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"ok"}]}}]}`))
	}))
	defer srv.Close()

	p := newGeminiProviderWithURL("test-key", srv.Client(), srv.URL)
	_, err := p.Complete(context.Background(), "generate a manifest", "")
	require.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()

	var body map[string]any
	require.NoError(t, json.Unmarshal(capturedBody, &body))

	// Verify contents array structure (Gemini format).
	contents, ok := body["contents"].([]any)
	require.True(t, ok, "body must have a 'contents' array")
	require.Len(t, contents, 1, "contents must contain exactly one entry")

	content, ok := contents[0].(map[string]any)
	require.True(t, ok)

	parts, ok := content["parts"].([]any)
	require.True(t, ok, "content must have a 'parts' array")
	require.Len(t, parts, 1, "parts must contain exactly one part")

	part, ok := parts[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "generate a manifest", part["text"],
		"part text must match the prompt")

	// Must NOT have OpenAI/Anthropic-style fields.
	assert.Nil(t, body["messages"], "Gemini request must not have 'messages' (that is OpenAI/Anthropic)")
	assert.Nil(t, body["model"], "Gemini request must not have 'model' in body (model is in URL path)")
}

func TestGeminiProvider_Complete_URLContainsModel(t *testing.T) {
	var capturedPath string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"ok"}]}}]}`))
	}))
	defer srv.Close()

	p := newGeminiProviderWithURL("test-key", srv.Client(), srv.URL)
	_, err := p.Complete(context.Background(), "test", "")
	require.NoError(t, err)
	assert.Equal(t, "/v1beta/models/gemini-2.0-flash:generateContent", capturedPath,
		"URL path must contain default model name")
}

func TestGeminiProvider_Complete_UsesCustomModelInURL(t *testing.T) {
	var capturedPath string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"ok"}]}}]}`))
	}))
	defer srv.Close()

	p := newGeminiProviderWithURL("test-key", srv.Client(), srv.URL)
	_, err := p.Complete(context.Background(), "test", "gemini-1.5-pro")
	require.NoError(t, err)
	assert.Equal(t, "/v1beta/models/gemini-1.5-pro:generateContent", capturedPath,
		"URL path must contain the custom model name")
}

func TestGeminiProvider_Complete_UsesPostMethod(t *testing.T) {
	var capturedMethod string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"ok"}]}}]}`))
	}))
	defer srv.Close()

	p := newGeminiProviderWithURL("test-key", srv.Client(), srv.URL)
	_, err := p.Complete(context.Background(), "test", "")
	require.NoError(t, err)
	assert.Equal(t, http.MethodPost, capturedMethod, "must use POST method")
}

// ---------------------------------------------------------------------------
// AC-3: Missing API key error.
// ---------------------------------------------------------------------------

func TestGeminiProvider_Complete_MissingAPIKey(t *testing.T) {
	p := NewGeminiProvider("", nil)
	_, err := p.Complete(context.Background(), "test", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "GEMINI_API_KEY",
		"error must mention GEMINI_API_KEY so users know what to set")
}

// ---------------------------------------------------------------------------
// AC-4: API key never leaked in errors.
// ---------------------------------------------------------------------------

func TestGeminiProvider_Complete_MissingAPIKey_ErrorHasNoKey(t *testing.T) {
	secretKey := "AIza-gemini-secret-value-12345"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"internal server error"}`))
	}))
	defer srv.Close()

	p := newGeminiProviderWithURL(secretKey, srv.Client(), srv.URL)
	_, err := p.Complete(context.Background(), "test", "")
	require.Error(t, err)
	assert.NotContains(t, err.Error(), secretKey,
		"error must NEVER contain the API key value")
}

func TestGeminiProvider_Complete_HTTPError_NoKeyInError(t *testing.T) {
	secretKey := "AIza-gemini-do-not-leak-me"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"forbidden"}`))
	}))
	defer srv.Close()

	p := newGeminiProviderWithURL(secretKey, srv.Client(), srv.URL)
	_, err := p.Complete(context.Background(), "test", "")
	require.Error(t, err)
	assert.NotContains(t, err.Error(), secretKey,
		"HTTP failure error must not include API key")
}

// ---------------------------------------------------------------------------
// AC-9: HTTP errors surface status information.
// ---------------------------------------------------------------------------

func TestGeminiProvider_Complete_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"something broke"}`))
	}))
	defer srv.Close()

	p := newGeminiProviderWithURL("test-key", srv.Client(), srv.URL)
	_, err := p.Complete(context.Background(), "test", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500",
		"error should include HTTP status code")
}

// ---------------------------------------------------------------------------
// AC-11: Context cancellation aborts request.
// ---------------------------------------------------------------------------

func TestGeminiProvider_Complete_CancelledContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	p := newGeminiProviderWithURL("test-key", srv.Client(), srv.URL)
	_, err := p.Complete(ctx, "test", "")
	require.Error(t, err, "cancelled context must produce an error")
}

// ---------------------------------------------------------------------------
// AC-10: io.LimitReader caps response body at 256KB.
// ---------------------------------------------------------------------------

func TestGeminiProvider_Complete_LargeResponse(t *testing.T) {
	largeText := strings.Repeat("x", 300*1024)
	respBody := `{"candidates":[{"content":{"parts":[{"text":"` + largeText + `"}]}}]}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(respBody))
	}))
	defer srv.Close()

	p := newGeminiProviderWithURL("test-key", srv.Client(), srv.URL)

	// Must not panic.
	result, err := p.Complete(context.Background(), "test", "")
	_ = result

	if err == nil {
		assert.Less(t, len(result), 300*1024,
			"response must be truncated by io.LimitReader, not returned in full")
	}
}

// ---------------------------------------------------------------------------
// AC-12: Invalid JSON response.
// ---------------------------------------------------------------------------

func TestGeminiProvider_Complete_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{not valid json at all`))
	}))
	defer srv.Close()

	p := newGeminiProviderWithURL("test-key", srv.Client(), srv.URL)
	_, err := p.Complete(context.Background(), "test", "")
	require.Error(t, err, "invalid JSON response must produce an error")
}

// ---------------------------------------------------------------------------
// Edge case: empty candidates array in response.
// ---------------------------------------------------------------------------

func TestGeminiProvider_Complete_EmptyCandidatesArray(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"candidates":[]}`))
	}))
	defer srv.Close()

	p := newGeminiProviderWithURL("test-key", srv.Client(), srv.URL)
	_, err := p.Complete(context.Background(), "test", "")
	require.Error(t, err, "empty candidates array should produce an error")
}

// ---------------------------------------------------------------------------
// Edge case: empty prompt.
// ---------------------------------------------------------------------------

func TestGeminiProvider_Complete_EmptyPrompt(t *testing.T) {
	var capturedBody []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"ok"}]}}]}`))
	}))
	defer srv.Close()

	p := newGeminiProviderWithURL("test-key", srv.Client(), srv.URL)
	_, err := p.Complete(context.Background(), "", "")
	if err == nil {
		var body map[string]any
		require.NoError(t, json.Unmarshal(capturedBody, &body))
		contents := body["contents"].([]any)
		content := contents[0].(map[string]any)
		parts := content["parts"].([]any)
		part := parts[0].(map[string]any)
		assert.Equal(t, "", part["text"], "empty prompt passed through as empty text")
	}
}

// ---------------------------------------------------------------------------
// AC-12: Constructor accepts *http.Client (nil uses default).
// ---------------------------------------------------------------------------

func TestGeminiProvider_NilClient(t *testing.T) {
	p := NewGeminiProvider("test-key", nil)
	require.NotNil(t, p, "NewGeminiProvider with nil client must return a valid provider")
}

func TestGeminiProvider_CustomClient(t *testing.T) {
	customClient := &http.Client{}
	p := NewGeminiProvider("test-key", customClient)
	require.NotNil(t, p, "NewGeminiProvider with custom client must return a valid provider")
}
