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
// Compile-time interface check: OpenAIProvider must implement LLMProvider.
// ---------------------------------------------------------------------------

var _ LLMProvider = (*OpenAIProvider)(nil)

// ---------------------------------------------------------------------------
// AC-2: Name() and DefaultModel()
// ---------------------------------------------------------------------------

func TestOpenAIProvider_Name(t *testing.T) {
	p := NewOpenAIProvider("test-key", nil)
	assert.Equal(t, "openai", p.Name())
}

func TestOpenAIProvider_DefaultModel(t *testing.T) {
	p := NewOpenAIProvider("test-key", nil)
	assert.Equal(t, "gpt-4o", p.DefaultModel())
}

// ---------------------------------------------------------------------------
// AC-2, AC-12: Complete sends correct headers and body via httptest server.
// ---------------------------------------------------------------------------

func TestOpenAIProvider_Complete_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := `{"choices":[{"message":{"role":"assistant","content":"hello world"}}]}`
		_, _ = w.Write([]byte(resp))
	}))
	defer srv.Close()

	p := newOpenAIProviderWithURL("test-key", srv.Client(), srv.URL)
	result, err := p.Complete(context.Background(), "say hello", "")
	require.NoError(t, err)
	assert.Equal(t, "hello world", result)
}

func TestOpenAIProvider_Complete_SendsCorrectHeaders(t *testing.T) {
	var mu sync.Mutex
	var capturedHeaders http.Header

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		capturedHeaders = r.Header.Clone()
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"ok"}}]}`))
	}))
	defer srv.Close()

	p := newOpenAIProviderWithURL("my-secret-key", srv.Client(), srv.URL)
	_, err := p.Complete(context.Background(), "test prompt", "")
	require.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()

	assert.Equal(t, "Bearer my-secret-key", capturedHeaders.Get("Authorization"),
		"must send Authorization: Bearer header with the API key")
	assert.Equal(t, "application/json", capturedHeaders.Get("Content-Type"),
		"must send Content-Type application/json")
	assert.Empty(t, capturedHeaders.Get("x-api-key"),
		"must NOT use x-api-key header (that is Anthropic's style)")
}

func TestOpenAIProvider_Complete_SendsCorrectBody(t *testing.T) {
	var mu sync.Mutex
	var capturedBody []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		capturedBody, _ = io.ReadAll(r.Body)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"ok"}}]}`))
	}))
	defer srv.Close()

	p := newOpenAIProviderWithURL("test-key", srv.Client(), srv.URL)
	_, err := p.Complete(context.Background(), "generate a manifest", "")
	require.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()

	var body map[string]any
	require.NoError(t, json.Unmarshal(capturedBody, &body))

	// Verify model defaults to DefaultModel when empty string passed.
	assert.Equal(t, "gpt-4o", body["model"],
		"model must default to gpt-4o")

	// Verify messages array structure (OpenAI format).
	messages, ok := body["messages"].([]any)
	require.True(t, ok, "messages must be an array")
	require.Len(t, messages, 1, "messages must contain exactly one user message")

	msg, ok := messages[0].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "user", msg["role"])
	assert.Equal(t, "generate a manifest", msg["content"])

	// Must NOT have Anthropic-style fields.
	assert.Nil(t, body["max_tokens"],
		"OpenAI request should not include max_tokens unless explicitly set")
}

func TestOpenAIProvider_Complete_UsesCustomModel(t *testing.T) {
	var mu sync.Mutex
	var capturedBody []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		capturedBody, _ = io.ReadAll(r.Body)
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"ok"}}]}`))
	}))
	defer srv.Close()

	p := newOpenAIProviderWithURL("test-key", srv.Client(), srv.URL)
	_, err := p.Complete(context.Background(), "test", "gpt-4-turbo")
	require.NoError(t, err)

	mu.Lock()
	defer mu.Unlock()

	var body map[string]any
	require.NoError(t, json.Unmarshal(capturedBody, &body))
	assert.Equal(t, "gpt-4-turbo", body["model"],
		"must use the custom model when one is specified")
}

func TestOpenAIProvider_Complete_PostsToCorrectPath(t *testing.T) {
	var capturedPath string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"ok"}}]}`))
	}))
	defer srv.Close()

	p := newOpenAIProviderWithURL("test-key", srv.Client(), srv.URL)
	_, err := p.Complete(context.Background(), "test", "")
	require.NoError(t, err)
	assert.Equal(t, "/v1/chat/completions", capturedPath,
		"must POST to /v1/chat/completions")
}

func TestOpenAIProvider_Complete_UsesPostMethod(t *testing.T) {
	var capturedMethod string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedMethod = r.Method
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"ok"}}]}`))
	}))
	defer srv.Close()

	p := newOpenAIProviderWithURL("test-key", srv.Client(), srv.URL)
	_, err := p.Complete(context.Background(), "test", "")
	require.NoError(t, err)
	assert.Equal(t, http.MethodPost, capturedMethod, "must use POST method")
}

// ---------------------------------------------------------------------------
// AC-3: Missing API key error.
// ---------------------------------------------------------------------------

func TestOpenAIProvider_Complete_MissingAPIKey(t *testing.T) {
	p := NewOpenAIProvider("", nil)
	_, err := p.Complete(context.Background(), "test", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "OPENAI_API_KEY",
		"error must mention OPENAI_API_KEY so users know what to set")
}

// ---------------------------------------------------------------------------
// AC-4: API key never leaked in errors.
// ---------------------------------------------------------------------------

func TestOpenAIProvider_Complete_MissingAPIKey_ErrorHasNoKey(t *testing.T) {
	secretKey := "sk-openai-secret-value-12345"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"internal server error"}`))
	}))
	defer srv.Close()

	p := newOpenAIProviderWithURL(secretKey, srv.Client(), srv.URL)
	_, err := p.Complete(context.Background(), "test", "")
	require.Error(t, err)
	assert.NotContains(t, err.Error(), secretKey,
		"error must NEVER contain the API key value")
}

func TestOpenAIProvider_Complete_HTTPError_NoKeyInError(t *testing.T) {
	secretKey := "sk-openai-do-not-leak-me"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"forbidden"}`))
	}))
	defer srv.Close()

	p := newOpenAIProviderWithURL(secretKey, srv.Client(), srv.URL)
	_, err := p.Complete(context.Background(), "test", "")
	require.Error(t, err)
	assert.NotContains(t, err.Error(), secretKey,
		"HTTP failure error must not include API key")
}

// ---------------------------------------------------------------------------
// AC-9: HTTP errors surface status information.
// ---------------------------------------------------------------------------

func TestOpenAIProvider_Complete_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"something broke"}`))
	}))
	defer srv.Close()

	p := newOpenAIProviderWithURL("test-key", srv.Client(), srv.URL)
	_, err := p.Complete(context.Background(), "test", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500",
		"error should include HTTP status code")
}

// ---------------------------------------------------------------------------
// AC-11: Context cancellation aborts request.
// ---------------------------------------------------------------------------

func TestOpenAIProvider_Complete_CancelledContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	p := newOpenAIProviderWithURL("test-key", srv.Client(), srv.URL)
	_, err := p.Complete(ctx, "test", "")
	require.Error(t, err, "cancelled context must produce an error")
}

// ---------------------------------------------------------------------------
// AC-10: io.LimitReader caps response body at 256KB.
// ---------------------------------------------------------------------------

func TestOpenAIProvider_Complete_LargeResponse(t *testing.T) {
	largeText := strings.Repeat("x", 300*1024)
	respBody := `{"choices":[{"message":{"role":"assistant","content":"` + largeText + `"}}]}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(respBody))
	}))
	defer srv.Close()

	p := newOpenAIProviderWithURL("test-key", srv.Client(), srv.URL)

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

func TestOpenAIProvider_Complete_InvalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{not valid json at all`))
	}))
	defer srv.Close()

	p := newOpenAIProviderWithURL("test-key", srv.Client(), srv.URL)
	_, err := p.Complete(context.Background(), "test", "")
	require.Error(t, err, "invalid JSON response must produce an error")
}

// ---------------------------------------------------------------------------
// Edge case: empty choices array in response.
// ---------------------------------------------------------------------------

func TestOpenAIProvider_Complete_EmptyChoicesArray(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[]}`))
	}))
	defer srv.Close()

	p := newOpenAIProviderWithURL("test-key", srv.Client(), srv.URL)
	_, err := p.Complete(context.Background(), "test", "")
	require.Error(t, err, "empty choices array should produce an error")
}

// ---------------------------------------------------------------------------
// Edge case: empty prompt.
// ---------------------------------------------------------------------------

func TestOpenAIProvider_Complete_EmptyPrompt(t *testing.T) {
	var capturedBody []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedBody, _ = io.ReadAll(r.Body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"ok"}}]}`))
	}))
	defer srv.Close()

	p := newOpenAIProviderWithURL("test-key", srv.Client(), srv.URL)
	_, err := p.Complete(context.Background(), "", "")
	if err == nil {
		var body map[string]any
		require.NoError(t, json.Unmarshal(capturedBody, &body))
		messages := body["messages"].([]any)
		msg := messages[0].(map[string]any)
		assert.Equal(t, "", msg["content"], "empty prompt passed through as empty content")
	}
}

// ---------------------------------------------------------------------------
// AC-12: Constructor accepts *http.Client (nil uses default).
// ---------------------------------------------------------------------------

func TestOpenAIProvider_NilClient(t *testing.T) {
	p := NewOpenAIProvider("test-key", nil)
	require.NotNil(t, p, "NewOpenAIProvider with nil client must return a valid provider")
}

func TestOpenAIProvider_CustomClient(t *testing.T) {
	customClient := &http.Client{}
	p := NewOpenAIProvider("test-key", customClient)
	require.NotNil(t, p, "NewOpenAIProvider with custom client must return a valid provider")
}
