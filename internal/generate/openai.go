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
	openAIDefaultBaseURL = "https://api.openai.com"
	openAIDefaultModel   = "gpt-4o"
	openAIBodyLimit      = 256 * 1024
)

// OpenAIProvider implements LLMProvider for the OpenAI Chat Completions API.
type OpenAIProvider struct {
	apiKey     string // never logged or included in errors
	httpClient *http.Client
	baseURL    string
}

// NewOpenAIProvider returns an OpenAIProvider using the production base URL.
// If client is nil, http.DefaultClient is used.
func NewOpenAIProvider(apiKey string, client *http.Client) *OpenAIProvider {
	return newOpenAIProviderWithURL(apiKey, client, openAIDefaultBaseURL)
}

// newOpenAIProviderWithURL returns an OpenAIProvider with a custom base URL.
// This is used in tests to point at an httptest server.
func newOpenAIProviderWithURL(apiKey string, client *http.Client, baseURL string) *OpenAIProvider {
	if client == nil {
		client = http.DefaultClient
	}
	return &OpenAIProvider{
		apiKey:     apiKey,
		httpClient: client,
		baseURL:    baseURL,
	}
}

// Name returns the provider identifier.
func (p *OpenAIProvider) Name() string {
	return "openai"
}

// DefaultModel returns the default model name for this provider.
func (p *OpenAIProvider) DefaultModel() string {
	return openAIDefaultModel
}

// Complete sends a prompt to the OpenAI Chat Completions API and returns the text response.
func (p *OpenAIProvider) Complete(ctx context.Context, prompt, model string) (string, error) {
	if p.apiKey == "" {
		return "", fmt.Errorf("openai: OPENAI_API_KEY is not set")
	}

	if model == "" {
		model = p.DefaultModel()
	}

	reqBody := openAIRequest{
		Model: model,
		Messages: []openAIMessage{
			{Role: "user", Content: prompt},
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("openai: marshal request: %w", err)
	}

	url := p.baseURL + "/v1/chat/completions"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("openai: create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+p.apiKey)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("openai: send request: %w", sanitiseHTTPError("openai", err))
	}
	defer resp.Body.Close() //nolint:errcheck // response body close error is not actionable

	limited := io.LimitReader(resp.Body, openAIBodyLimit)
	respBytes, err := io.ReadAll(limited)
	if err != nil {
		return "", fmt.Errorf("openai: read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("openai: unexpected status %d", resp.StatusCode)
	}

	var parsed openAIResponse
	if err := json.Unmarshal(respBytes, &parsed); err != nil {
		return "", fmt.Errorf("openai: decode response: %w", err)
	}

	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("openai: response contained no choices")
	}

	return parsed.Choices[0].Message.Content, nil
}

type openAIRequest struct {
	Model    string          `json:"model"`
	Messages []openAIMessage `json:"messages"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIResponse struct {
	Choices []struct {
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}
