package generate

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

const (
	geminiDefaultBaseURL = "https://generativelanguage.googleapis.com"
	geminiDefaultModel   = "gemini-2.0-flash"
	geminiBodyLimit      = 256 * 1024
)

// GeminiProvider implements LLMProvider for the Google Gemini generateContent API.
type GeminiProvider struct {
	apiKey     string // never logged or included in errors
	httpClient *http.Client
	baseURL    string
}

// NewGeminiProvider returns a GeminiProvider using the production base URL.
// If client is nil, http.DefaultClient is used.
func NewGeminiProvider(apiKey string, client *http.Client) *GeminiProvider {
	return newGeminiProviderWithURL(apiKey, client, geminiDefaultBaseURL)
}

// newGeminiProviderWithURL returns a GeminiProvider with a custom base URL.
// This is used in tests to point at an httptest server.
func newGeminiProviderWithURL(apiKey string, client *http.Client, baseURL string) *GeminiProvider {
	if client == nil {
		client = http.DefaultClient
	}
	return &GeminiProvider{
		apiKey:     apiKey,
		httpClient: client,
		baseURL:    baseURL,
	}
}

// Name returns the provider identifier.
func (p *GeminiProvider) Name() string {
	return "gemini"
}

// DefaultModel returns the default model name for this provider.
func (p *GeminiProvider) DefaultModel() string {
	return geminiDefaultModel
}

// Complete sends a prompt to the Gemini generateContent API and returns the text response.
func (p *GeminiProvider) Complete(ctx context.Context, prompt, model string) (string, error) {
	if p.apiKey == "" {
		return "", fmt.Errorf("gemini: GEMINI_API_KEY is not set")
	}

	if model == "" {
		model = p.DefaultModel()
	}

	reqBody := geminiRequest{
		Contents: []geminiContent{
			{
				Parts: []geminiPart{
					{Text: prompt},
				},
			},
		},
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("gemini: marshal request: %w", err)
	}

	params := url.Values{}
	params.Set("key", p.apiKey)
	endpoint := fmt.Sprintf("%s/v1beta/models/%s:generateContent?%s",
		p.baseURL, url.PathEscape(model), params.Encode())
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(bodyBytes))
	if err != nil {
		return "", fmt.Errorf("gemini: create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("gemini: send request: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck // response body close error is not actionable

	limited := io.LimitReader(resp.Body, geminiBodyLimit)
	respBytes, err := io.ReadAll(limited)
	if err != nil {
		return "", fmt.Errorf("gemini: read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("gemini: unexpected status %d", resp.StatusCode)
	}

	var parsed geminiResponse
	if err := json.Unmarshal(respBytes, &parsed); err != nil {
		return "", fmt.Errorf("gemini: decode response: %w", err)
	}

	if len(parsed.Candidates) == 0 {
		return "", fmt.Errorf("gemini: response contained no candidates")
	}

	parts := parsed.Candidates[0].Content.Parts
	if len(parts) == 0 {
		return "", fmt.Errorf("gemini: response candidate contained no parts")
	}

	return parts[0].Text, nil
}

type geminiRequest struct {
	Contents []geminiContent `json:"contents"`
}

type geminiContent struct {
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []geminiPart `json:"parts"`
		} `json:"content"`
	} `json:"candidates"`
}
