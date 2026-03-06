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
// Compile-time interface check: AnthropicProvider must implement LLMProvider.
// ---------------------------------------------------------------------------

var _ LLMProvider = (*AnthropicProvider)(nil)

// ---------------------------------------------------------------------------
// AC-2: Name() and DefaultModel()
// ---------------------------------------------------------------------------

func TestAnthropicProvider_Name(t *testing.T) {
	p := NewAnthropicProvider("test-key", nil)
	assert.Equal(t, "anthropic", p.Name())
}

func TestAnthropicProvider_DefaultModel(t *testing.T) {
	p := NewAnthropicProvider("test-key", nil)
	assert.Equal(t, "claude-sonnet-4-20250514", p.DefaultModel())
}

// ---------------------------------------------------------------------------
// AC-2, AC-12: Complete sends correct headers and body via httptest server.
// ---------------------------------------------------------------------------

func TestAnthropicProvider_Complete_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := `{"content":[{"type":"text","text":"hello world"}]}`
		_, _ = w.Write([]byte(resp))
	}))
	defer srv.Close()

	p := newAnthropicProviderWithURL("test-key", srv.Client(), srv.URL)
	result, err := p.Complete(context.Background(), "say hello", "")
	require.NoError(t, err)
	assert.Equal(t, "hello world", result)
}

func TestAnthropicProvider_Complete_SendsCorrectHeaders(t *testing.T) {
	var mu sync.Mutex
	var capturedHeaders http.Header

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		capturedHeaders = r.Header.Clone()
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"ok"}]}`))
	}))
	defer srv.Close()

	p := newAnthropicProviderWithURL("my-secret-key", srv.Client(), srv.URL)
	_, err := p.Complete(context.Background(), "test prompt", "")
	require.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()

	assert.Equal(t, "my-secret-key", capturedHeaders.Get("x-api-key"),
		"must send x-api-key header with the API key")
	assert.Equal(t, "2023-06-01", capturedHeaders.Get("anthropic-version"),
		"must send anthropic-version header")
	assert.Equal(t, "application/json", capturedHeaders.Get("Content-Type"),
		"must send Content-Type application/json")
}

func TestAnthropicProvider_Complete_SendsCorrectBody(t *testing.T) {
	var mu sync.Mutex
	var capturedBody []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		capturedBody, _ = io.ReadAll(r.Body)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"ok"}]}`))
	}))
	defer srv.Close()

	p := newAnthropicProviderWithURL("test-key", srv.Client(), srv.URL)
	_, err := p.Complete(context.Background(), "generate a manifest", "")
	require.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()

	var body map[string]any
	require.NoError(t, json.Unmarshal(capturedBody, &body))

	// Verify model defaults to DefaultModel when empty string passed.
	assert.Equal(t, "claude-sonnet-4-20250514", body["model"],
		"model must default to claude-sonnet-4-20250514")

	// Verify max_tokens is set.
	assert.Equal(t, float64(4096), body["max_tokens"],
		"max_tokens must be 4096")

	// Verify messages array structure.
	messages, ok := body["messages"].([]any)
	require.True(t, ok, "messages must be an array")
	require.Len(t, messages, 1, "messages must contain exactly one user message")

	msg, ok := messages[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "user", msg["role"])
	assert.Equal(t, "generate a manifest", msg["content"])
}

func TestAnthropicProvider_Complete_UsesCustomModel(t *testing.T) {
	var mu sync.Mutex
	var capturedBody []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		capturedBody, _ = io.ReadAll(r.Body)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"ok"}]}`))
	}))
	defer srv.Close()

	p := newAnthropicProviderWithURL("test-key", srv.Client(), srv.URL)
	_, err := p.Complete(context.Background(), "test", "claude-opus-4")
	require.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()

	var body map[string]any
	require.NoError(t, json.Unmarshal(capturedBody, &body))
	assert.Equal(t, "claude-opus-4", body["model"],
		"must use the custom model when one is specified")
}

func TestAnthropicProvider_Complete_PostsToCorrectPath(t *testing.T) {
	var capturedPath string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"ok"}]}`))
	}))
	defer srv.Close()

	p := newAnthropicProviderWithURL("test-key", srv.Client(), srv.URL)
	_, err := p.Complete(context.Background(), "test", "")
	require.NoError(t, err)
	assert.Equal(t, "/v1/messages", capturedPath,
		"must POST to /v1/messages")
}

func TestAnthropicProvider_Complete_UsesPostMethod(t *testing.T) {
	var capturedMethod string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"ok"}]}`))
	}))
	defer srv.Close()

	p := newAnthropicProviderWithURL("test-key", srv.Client(), srv.URL)
	_, err := p.Complete(context.Background(), "test", "")
	require.NoError(t, err)
	assert.Equal(t, http.MethodPost, capturedMethod, "must use POST method")
}

// ---------------------------------------------------------------------------
// AC-3: Missing API key error.
// ---------------------------------------------------------------------------

func TestAnthropicProvider_Complete_MissingAPIKey(t *testing.T) {
	// Construct with empty key to simulate missing env var.
	p := NewAnthropicProvider("", nil)
	_, err := p.Complete(context.Background(), "test", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "ANTHROPIC_API_KEY",
		"error must mention ANTHROPIC_API_KEY so users know what to set")
}

// ---------------------------------------------------------------------------
// AC-4: API key never leaked in errors.
// ---------------------------------------------------------------------------

func TestAnthropicProvider_Complete_MissingAPIKey_ErrorHasNoKey(t *testing.T) {
	// Even if someone passes a key value, if we decide it's "missing" (empty),
	// the error must not contain any key. But more importantly, test that a
	// real key is never in error output during HTTP failures.
	secretKey := "sk-ant-secret-value-12345"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"internal server error"}`))
	}))
	defer srv.Close()

	p := newAnthropicProviderWithURL(secretKey, srv.Client(), srv.URL)
	_, err := p.Complete(context.Background(), "test", "")
	require.Error(t, err)
	assert.NotContains(t, err.Error(), secretKey,
		"error must NEVER contain the API key value")
}

func TestAnthropicProvider_Complete_HTTPError_NoHeaders(t *testing.T) {
	secretKey := "sk-ant-do-not-leak-me"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"forbidden"}`))
	}))
	defer srv.Close()

	p := newAnthropicProviderWithURL(secretKey, srv.Client(), srv.URL)
	_, err := p.Complete(context.Background(), "test", "")
	require.Error(t, err)
	assert.NotContains(t, err.Error(), secretKey,
		"HTTP failure error must not include API key from headers")
	assert.NotContains(t, err.Error(), "x-api-key",
		"HTTP failure error must not reference request header names")
}

// ---------------------------------------------------------------------------
// AC-9: HTTP errors surface status information.
// ---------------------------------------------------------------------------

func TestAnthropicProvider_Complete_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"something broke"}`))
	}))
	defer srv.Close()

	p := newAnthropicProviderWithURL("test-key", srv.Client(), srv.URL)
	_, err := p.Complete(context.Background(), "test", "")
	require.Error(t, err)
	// Error should give some indication of what went wrong (status code).
	assert.Contains(t, err.Error(), "500",
		"error should include HTTP status code")
}

func TestAnthropicProvider_Complete_HTTPError_429(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":"rate limited"}`))
	}))
	defer srv.Close()

	p := newAnthropicProviderWithURL("test-key", srv.Client(), srv.URL)
	_, err := p.Complete(context.Background(), "test", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "429",
		"error should include HTTP status code for rate limit")
}

// ---------------------------------------------------------------------------
// AC-11: Context cancellation aborts request.
// ---------------------------------------------------------------------------

func TestAnthropicProvider_Complete_CancelledContext(t *testing.T) {
	// Server that blocks until the context is done, ensuring we test real
	// cancellation rather than a fast response beating the cancel.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
		// Don't write response — client should get cancel error.
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	p := newAnthropicProviderWithURL("test-key", srv.Client(), srv.URL)
	_, err := p.Complete(ctx, "test", "")
	require.Error(t, err, "cancelled context must produce an error")
}

// ---------------------------------------------------------------------------
// AC-10: io.LimitReader caps response body at 256KB.
// ---------------------------------------------------------------------------

func TestAnthropicProvider_Complete_LargeResponse(t *testing.T) {
	// Build a response > 256KB. The text field alone is 300KB.
	largeText := strings.Repeat("x", 300*1024)
	respBody := `{"content":[{"type":"text","text":"` + largeText + `"}]}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(respBody))
	}))
	defer srv.Close()

	p := newAnthropicProviderWithURL("test-key", srv.Client(), srv.URL)

	// Must not panic. May return an error (truncated JSON) or truncated text.
	// The key contract is: no panic, and we don't allocate unbounded memory.
	result, err := p.Complete(context.Background(), "test", "")
	_ = result // We don't care about the value, just no panic.

	// With a 256KB limit, the 300KB+ response will be truncated, producing
	// invalid JSON. The implementation should return an error or handle it.
	// A correct implementation using io.LimitReader will truncate the body.
	if err == nil {
		// If no error, the result must be shorter than the full 300KB text,
		// proving truncation happened.
		assert.Less(t, len(result), 300*1024,
			"response must be truncated by io.LimitReader, not returned in full")
	}
	// If err != nil, that's also acceptable — truncated JSON should error.
}

// ---------------------------------------------------------------------------
// AC-12: Invalid JSON response.
// ---------------------------------------------------------------------------

func TestAnthropicProvider_Complete_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{not valid json at all`))
	}))
	defer srv.Close()

	p := newAnthropicProviderWithURL("test-key", srv.Client(), srv.URL)
	_, err := p.Complete(context.Background(), "test", "")
	require.Error(t, err, "invalid JSON response must produce an error")
}

// ---------------------------------------------------------------------------
// Edge case: empty content array in response.
// ---------------------------------------------------------------------------

func TestAnthropicProvider_Complete_EmptyContentArray(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"content":[]}`))
	}))
	defer srv.Close()

	p := newAnthropicProviderWithURL("test-key", srv.Client(), srv.URL)
	_, err := p.Complete(context.Background(), "test", "")
	require.Error(t, err, "empty content array should produce an error")
}

// ---------------------------------------------------------------------------
// Edge case: response with multiple content blocks returns first text block.
// ---------------------------------------------------------------------------

func TestAnthropicProvider_Complete_MultipleContentBlocks(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := `{"content":[{"type":"text","text":"first"},{"type":"text","text":"second"}]}`
		_, _ = w.Write([]byte(resp))
	}))
	defer srv.Close()

	p := newAnthropicProviderWithURL("test-key", srv.Client(), srv.URL)
	result, err := p.Complete(context.Background(), "test", "")
	require.NoError(t, err)
	// Should include at minimum the first text block.
	assert.Contains(t, result, "first",
		"must extract text from the content blocks")
}

// ---------------------------------------------------------------------------
// Edge case: empty prompt.
// ---------------------------------------------------------------------------

func TestAnthropicProvider_Complete_EmptyPrompt(t *testing.T) {
	var capturedBody []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"ok"}]}`))
	}))
	defer srv.Close()

	p := newAnthropicProviderWithURL("test-key", srv.Client(), srv.URL)
	_, err := p.Complete(context.Background(), "", "")
	// Whether this succeeds or fails, the body must still have proper structure.
	if err == nil {
		var body map[string]any
		require.NoError(t, json.Unmarshal(capturedBody, &body))
		messages := body["messages"].([]any)
		msg := messages[0].(map[string]any)
		assert.Equal(t, "", msg["content"], "empty prompt passed through as empty content")
	}
}

// ---------------------------------------------------------------------------
// AC-4: Transport-level errors must not leak the API key embedded in headers.
// This covers the *url.Error path where http.Client.Do itself fails (DNS, TLS,
// context cancel). For Anthropic, keys are in headers (not the URL), so the
// risk is lower, but sanitiseHTTPError is still applied for defence-in-depth.
// ---------------------------------------------------------------------------

func TestAnthropicProvider_Complete_TransportError_NoKeyInError(t *testing.T) {
	secretKey := "sk-ant-anthropic-transport-secret"
	// Hijack and close so the client gets a transport error, not an HTTP error.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		conn, _, _ := w.(http.Hijacker).Hijack()
		conn.Close()
	}))
	defer srv.Close()

	p := newAnthropicProviderWithURL(secretKey, srv.Client(), srv.URL)
	_, err := p.Complete(context.Background(), "test", "")
	require.Error(t, err, "transport failure must return error")
	assert.NotContains(t, err.Error(), secretKey,
		"transport-level error must NEVER contain the API key")
}

// ---------------------------------------------------------------------------
// AC-12: Constructor accepts *http.Client (nil uses default).
// ---------------------------------------------------------------------------

func TestAnthropicProvider_NilClient(t *testing.T) {
	// Must not panic when nil client is passed.
	p := NewAnthropicProvider("test-key", nil)
	require.NotNil(t, p, "NewAnthropicProvider with nil client must return a valid provider")
}

func TestAnthropicProvider_CustomClient(t *testing.T) {
	customClient := &http.Client{}
	p := NewAnthropicProvider("test-key", customClient)
	require.NotNil(t, p, "NewAnthropicProvider with custom client must return a valid provider")
}
