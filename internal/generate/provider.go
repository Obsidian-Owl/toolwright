package generate

import (
	"context"
	"errors"
	"net/url"
)

// LLMProvider is the interface all LLM provider implementations must satisfy.
type LLMProvider interface {
	Complete(ctx context.Context, prompt string, model string) (string, error)
	Name() string
	DefaultModel() string
}

// sanitiseHTTPError strips the URL from a *url.Error before wrapping it.
//
// Go's http.Client.Do returns *url.Error on transport failures (DNS, TLS,
// context cancellation). The error's string representation includes the full
// request URL, which for Gemini contains the API key as a query parameter.
// This helper extracts only the operation name and root cause, discarding the
// URL so the key never appears in error output.
func sanitiseHTTPError(provider string, err error) error {
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		return &sanitisedHTTPError{provider: provider, op: urlErr.Op, cause: urlErr.Err}
	}
	return err
}

type sanitisedHTTPError struct {
	provider string
	op       string
	cause    error
}

func (e *sanitisedHTTPError) Error() string {
	return e.provider + ": " + e.op + ": " + e.cause.Error()
}

func (e *sanitisedHTTPError) Unwrap() error {
	return e.cause
}
