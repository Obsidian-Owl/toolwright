package generate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const (
	anthropicDefaultBaseURL = "https://api.anthropic.com"
	anthropicDefaultModel   = "claude-sonnet-4-20250514"
	anthropicVersion        = "2023-06-01"
	anthropicMaxTokens      = 4096
	anthropicBodyLimit      = 256 * 1024
)

// AnthropicProvider implements LLMProvider for the Anthropic Messages API.
type AnthropicProvider struct {
	apiKey     string // never logged or included in errors
	httpClient *http.Client
	baseURL    string
}

// NewAnthropicProvider returns an AnthropicProvider using the production base URL.
// If client is nil, http.DefaultClient is used.
func NewAnthropicProvider(apiKey string, client *http.Client) *AnthropicProvider {
	return newAnthropicProviderWithURL(apiKey, client, anthropicDefaultBaseURL)
}

// newAnthropicProviderWithURL returns an AnthropicProvider with a custom base URL.
// This is used in tests to point at an httptest server.
func newAnthropicProviderWithURL(apiKey string, client *http.Client, baseURL string) *AnthropicProvider {
	if client == nil {
		client = http.DefaultClient
	}
	return &AnthropicProvider{
		apiKey:     apiKey,
		httpClient: client,
		baseURL:    baseURL,
	}
}

// Name returns the provider identifier.
func (p *AnthropicProvider) Name() string {
	return "anthropic"
}

// DefaultModel returns the default model name for this provider.
func (p *AnthropicProvider) DefaultModel() string {
	return anthropicDefaultModel
}

// Complete sends a prompt to the Anthropic Messages API and returns the text response.
func (p *AnthropicProvider) Complete(ctx context.Context, prompt, model string) (string, error) {
	if p.apiKey == "" {
		return "", fmt.Errorf("anthropic: ANTHROPIC_API_KEY is not set")
	}

	if model == "" {
		model = p.DefaultModel()
	}

	reqBody := anthropicRequest{
		Model:     model,
		MaxTokens: anthropicMaxTokens,
		Messages: []anthropicMessage{
			{Role: "user", Content: prompt},
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("anthropic: marshal request: %w", err)
	}

	url := p.baseURL + "/v1/messages"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("anthropic: create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", p.apiKey)
	req.Header.Set("anthropic-version", anthropicVersion)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("anthropic: send request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // response body close error is not actionable

	limited := io.LimitReader(resp.Body, anthropicBodyLimit)
	respBytes, err := io.ReadAll(limited)
	if err != nil {
		return "", fmt.Errorf("anthropic: read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("anthropic: unexpected status %d: %s", resp.StatusCode, respBytes)
	}

	var parsed anthropicResponse
	if err := json.Unmarshal(respBytes, &parsed); err != nil {
		return "", fmt.Errorf("anthropic: decode response: %w", err)
	}

	if len(parsed.Content) == 0 {
		return "", fmt.Errorf("anthropic: response contained no content blocks")
	}

	return parsed.Content[0].Text, nil
}

type anthropicRequest struct {
	Model     string             `json:"model"`
	MaxTokens int                `json:"max_tokens"`
	Messages  []anthropicMessage `json:"messages"`
}

type anthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type anthropicResponse struct {
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
}
