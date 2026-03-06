package generate

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Obsidian-Owl/toolwright/internal/cli"
	"github.com/Obsidian-Owl/toolwright/internal/manifest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// validManifestYAML is a well-formed manifest that passes manifest.Parse and
// manifest.Validate. Used by tests that need a successful LLM response.
const validManifestYAML = `apiVersion: toolwright/v1
kind: Toolkit
metadata:
  name: test-tool
  version: 1.0.0
  description: A test tool
tools:
  - name: hello
    description: Says hello
    entrypoint: bin/hello
`

// ---------------------------------------------------------------------------
// mockProvider implements LLMProvider for testing.
// ---------------------------------------------------------------------------

type mockProvider struct {
	name         string
	defaultModel string
	// completeFn is called on each Complete invocation. Use it to control
	// success/failure sequences.
	completeFn func(ctx context.Context, prompt, model string) (string, error)
}

func (m *mockProvider) Complete(ctx context.Context, prompt, model string) (string, error) {
	return m.completeFn(ctx, prompt, model)
}

func (m *mockProvider) Name() string {
	return m.name
}

func (m *mockProvider) DefaultModel() string {
	return m.defaultModel
}

// newSuccessProvider returns a mock that always returns the given YAML.
func newSuccessProvider(name, defaultModel, yamlContent string) *mockProvider {
	return &mockProvider{
		name:         name,
		defaultModel: defaultModel,
		completeFn: func(_ context.Context, _, _ string) (string, error) {
			return yamlContent, nil
		},
	}
}

// newCountingProvider returns a mock that tracks call count and lets the caller
// control per-attempt behavior via the responses slice. Each element is either
// a string (success) or an error.
func newCountingProvider(defaultModel string, responses []any) (*mockProvider, *int32) { //nolint:unparam // defaultModel varies across test scenarios
	return newNamedCountingProvider("test-provider", defaultModel, responses)
}

func newNamedCountingProvider(name, defaultModel string, responses []any) (*mockProvider, *int32) { //nolint:unparam // name used by callers wanting custom provider names
	var count int32
	return &mockProvider{
		name:         name,
		defaultModel: defaultModel,
		completeFn: func(_ context.Context, _, _ string) (string, error) {
			idx := int(atomic.AddInt32(&count, 1)) - 1
			if idx >= len(responses) {
				return "", fmt.Errorf("unexpected call #%d (only %d responses configured)", idx+1, len(responses))
			}
			switch v := responses[idx].(type) {
			case string:
				return v, nil
			case error:
				return "", v
			default:
				return "", fmt.Errorf("bad response type at index %d", idx)
			}
		},
	}, &count
}

// testProviders returns a map with a single mock provider for use with
// NewGeneratorWithProviders.
func testProviders(p LLMProvider) map[string]LLMProvider {
	return map[string]LLMProvider{p.Name(): p}
}

// ---------------------------------------------------------------------------
// AC-1: Compile-time interface check.
// ---------------------------------------------------------------------------

// TestGenerator_ImplementsManifestGeneratorInterface is a compile-time check
// that *Generator satisfies cli.manifestGenerator. If the interface changes or
// Generator's signature drifts, this fails at compile time.
var _ cli.ManifestGenerator = (*Generator)(nil)

// ---------------------------------------------------------------------------
// AC-1: NewGenerator has all 3 providers.
// ---------------------------------------------------------------------------

func TestGenerator_NewGenerator_HasAllProviders(t *testing.T) {
	g := NewGenerator()
	require.NotNil(t, g)

	// The generator must have exactly the 3 built-in providers registered.
	providers := g.Providers()
	expected := []string{"anthropic", "gemini", "openai"}
	var got []string
	for name := range providers {
		got = append(got, name)
	}
	// Sort for deterministic comparison.
	assert.ElementsMatch(t, expected, got,
		"NewGenerator must register exactly anthropic, openai, gemini")
}

// ---------------------------------------------------------------------------
// AC-6: Generated manifest is valid YAML that parses and validates.
// ---------------------------------------------------------------------------

func TestGenerator_Generate_ValidYAML_ReturnsResult(t *testing.T) {
	p := newSuccessProvider("test-provider", "test-model", validManifestYAML)
	g := NewGeneratorWithProviders(testProviders(p))

	result, err := g.Generate(context.Background(), cli.ManifestGenerateOptions{
		Provider:    "test-provider",
		Description: "a test toolkit",
	})
	require.NoError(t, err)
	require.NotNil(t, result)

	// Parse the result manifest YAML.
	tk, err := manifest.Parse(strings.NewReader(result.Manifest))
	require.NoError(t, err, "result manifest must parse as valid YAML")

	// Validate the parsed toolkit.
	errs := manifest.Validate(tk)
	assert.Empty(t, errs, "parsed manifest must pass Validate: %v", errs)

	// Verify required fields.
	assert.Equal(t, "toolwright/v1", tk.APIVersion)
	assert.Equal(t, "Toolkit", tk.Kind)
	assert.NotEmpty(t, tk.Metadata.Name)
	assert.NotEmpty(t, tk.Metadata.Description)
}

func TestGenerator_Generate_ResultContainsAPIVersionAndKind(t *testing.T) {
	p := newSuccessProvider("test-provider", "test-model", validManifestYAML)
	g := NewGeneratorWithProviders(testProviders(p))

	result, err := g.Generate(context.Background(), cli.ManifestGenerateOptions{
		Provider:    "test-provider",
		Description: "test",
	})
	require.NoError(t, err)

	assert.Contains(t, result.Manifest, "apiVersion: toolwright/v1",
		"result YAML must include apiVersion")
	assert.Contains(t, result.Manifest, "kind: Toolkit",
		"result YAML must include kind")
}

// ---------------------------------------------------------------------------
// AC-6: YAML extraction from fenced code blocks.
// ---------------------------------------------------------------------------

func TestGenerator_Generate_YAMLExtractedFromFencedBlock(t *testing.T) {
	fencedResponse := "Here is the manifest:\n```yaml\n" + validManifestYAML + "```\nEnjoy!"
	p := newSuccessProvider("test-provider", "test-model", fencedResponse)
	g := NewGeneratorWithProviders(testProviders(p))

	result, err := g.Generate(context.Background(), cli.ManifestGenerateOptions{
		Provider:    "test-provider",
		Description: "test",
	})
	require.NoError(t, err)

	// The extracted YAML must parse cleanly.
	tk, err := manifest.Parse(strings.NewReader(result.Manifest))
	require.NoError(t, err, "YAML extracted from fenced block must parse")
	assert.Equal(t, "test-tool", tk.Metadata.Name)
}

// ---------------------------------------------------------------------------
// AC-1: Generate returns correct result type.
// ---------------------------------------------------------------------------

func TestGenerator_Generate_ResultContainsProvider(t *testing.T) {
	p := newSuccessProvider("my-provider", "default-m", validManifestYAML)
	g := NewGeneratorWithProviders(testProviders(p))

	result, err := g.Generate(context.Background(), cli.ManifestGenerateOptions{
		Provider:    "my-provider",
		Description: "test",
	})
	require.NoError(t, err)
	assert.Equal(t, "my-provider", result.Provider,
		"result.Provider must match the provider used")
}

func TestGenerator_Generate_ResultContainsManifest(t *testing.T) {
	p := newSuccessProvider("test-provider", "test-model", validManifestYAML)
	g := NewGeneratorWithProviders(testProviders(p))

	result, err := g.Generate(context.Background(), cli.ManifestGenerateOptions{
		Provider:    "test-provider",
		Description: "test",
	})
	require.NoError(t, err)
	assert.NotEmpty(t, result.Manifest,
		"result.Manifest must be non-empty YAML")
	// Verify it actually contains YAML content, not garbage.
	assert.Contains(t, result.Manifest, "apiVersion:")
}

// ---------------------------------------------------------------------------
// Error: unknown provider.
// ---------------------------------------------------------------------------

func TestGenerator_Generate_ProviderNotFound_ReturnsError(t *testing.T) {
	p := newSuccessProvider("known-provider", "model", validManifestYAML)
	g := NewGeneratorWithProviders(testProviders(p))

	_, err := g.Generate(context.Background(), cli.ManifestGenerateOptions{
		Provider:    "unknown",
		Description: "test",
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown",
		"error must mention the unknown provider name")
}

// ---------------------------------------------------------------------------
// AC-7: Retry on provider error.
// ---------------------------------------------------------------------------

func TestGenerator_Generate_RetryOnProviderError(t *testing.T) {
	p, count := newCountingProvider("model", []any{
		fmt.Errorf("temporary API failure"),
		validManifestYAML,
	})
	g := NewGeneratorWithProviders(testProviders(p))

	result, err := g.Generate(context.Background(), cli.ManifestGenerateOptions{
		Provider:    "test-provider",
		Description: "test",
	})
	require.NoError(t, err, "must succeed on retry after first failure")
	require.NotNil(t, result)
	assert.Equal(t, int32(2), atomic.LoadInt32(count),
		"provider must be called exactly 2 times (1 failure + 1 retry)")
}

func TestGenerator_Generate_NoRetryOnSuccess(t *testing.T) {
	p, count := newCountingProvider("model", []any{
		validManifestYAML,
	})
	g := NewGeneratorWithProviders(testProviders(p))

	result, err := g.Generate(context.Background(), cli.ManifestGenerateOptions{
		Provider:    "test-provider",
		Description: "test",
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, int32(1), atomic.LoadInt32(count),
		"provider must be called exactly 1 time when first attempt succeeds")
}

// ---------------------------------------------------------------------------
// AC-7: Retry on invalid YAML.
// ---------------------------------------------------------------------------

func TestGenerator_Generate_RetryOnInvalidYAML(t *testing.T) {
	invalidYAML := "this is not valid YAML: [{"
	p, count := newCountingProvider("model", []any{
		invalidYAML,
		validManifestYAML,
	})
	g := NewGeneratorWithProviders(testProviders(p))

	result, err := g.Generate(context.Background(), cli.ManifestGenerateOptions{
		Provider:    "test-provider",
		Description: "test",
	})
	require.NoError(t, err, "must succeed on retry after invalid YAML")
	require.NotNil(t, result)
	assert.Equal(t, int32(2), atomic.LoadInt32(count),
		"provider must be called exactly 2 times (1 invalid YAML + 1 retry)")

	// Verify the result is actually the valid YAML, not the invalid one.
	tk, err := manifest.Parse(strings.NewReader(result.Manifest))
	require.NoError(t, err)
	assert.Equal(t, "test-tool", tk.Metadata.Name)
}

// ---------------------------------------------------------------------------
// AC-7: Both calls fail -> return error.
// ---------------------------------------------------------------------------

func TestGenerator_Generate_BothCallsFail_ReturnsError(t *testing.T) {
	p, count := newCountingProvider("model", []any{
		fmt.Errorf("first failure"),
		fmt.Errorf("second failure"),
	})
	g := NewGeneratorWithProviders(testProviders(p))

	_, err := g.Generate(context.Background(), cli.ManifestGenerateOptions{
		Provider:    "test-provider",
		Description: "test",
	})
	require.Error(t, err, "must return error when both attempts fail")
	assert.Equal(t, int32(2), atomic.LoadInt32(count),
		"provider must be called exactly 2 times before giving up")
}

// ---------------------------------------------------------------------------
// AC-7: Max two attempts.
// ---------------------------------------------------------------------------

func TestGenerator_Generate_MaxTwoAttempts(t *testing.T) {
	// Provide 5 responses, but generator should stop after 2.
	p, count := newCountingProvider("model", []any{
		fmt.Errorf("fail 1"),
		fmt.Errorf("fail 2"),
		validManifestYAML, // Should never be reached.
		validManifestYAML,
		validManifestYAML,
	})
	g := NewGeneratorWithProviders(testProviders(p))

	_, err := g.Generate(context.Background(), cli.ManifestGenerateOptions{
		Provider:    "test-provider",
		Description: "test",
	})
	require.Error(t, err)
	assert.Equal(t, int32(2), atomic.LoadInt32(count),
		"total attempts must never exceed 2, even if more responses are available")
}

func TestGenerator_Generate_MaxTwoAttempts_InvalidYAML(t *testing.T) {
	// Both return invalid YAML. Third would succeed but must not be reached.
	invalidYAML := "not: [valid: yaml: {{"
	p, count := newCountingProvider("model", []any{
		invalidYAML,
		invalidYAML,
		validManifestYAML, // Must not be reached.
	})
	g := NewGeneratorWithProviders(testProviders(p))

	_, err := g.Generate(context.Background(), cli.ManifestGenerateOptions{
		Provider:    "test-provider",
		Description: "test",
	})
	require.Error(t, err, "both invalid YAML attempts must produce an error")
	assert.Equal(t, int32(2), atomic.LoadInt32(count),
		"total attempts must never exceed 2 for invalid YAML retries")
}

// ---------------------------------------------------------------------------
// AC-8: --model flag behavior.
// ---------------------------------------------------------------------------

func TestGenerator_Generate_ModelPassedToProvider(t *testing.T) {
	var receivedModel string
	p := &mockProvider{
		name:         "test-provider",
		defaultModel: "default-model",
		completeFn: func(_ context.Context, _, model string) (string, error) {
			receivedModel = model
			return validManifestYAML, nil
		},
	}
	g := NewGeneratorWithProviders(testProviders(p))

	_, err := g.Generate(context.Background(), cli.ManifestGenerateOptions{
		Provider:    "test-provider",
		Description: "test",
		Model:       "my-custom-model",
	})
	require.NoError(t, err)
	assert.Equal(t, "my-custom-model", receivedModel,
		"when Model is set, provider must receive that exact model string")
}

func TestGenerator_Generate_EmptyModel_UsesProviderDefault(t *testing.T) {
	var receivedModel string
	p := &mockProvider{
		name:         "test-provider",
		defaultModel: "provider-default-model",
		completeFn: func(_ context.Context, _, model string) (string, error) {
			receivedModel = model
			return validManifestYAML, nil
		},
	}
	g := NewGeneratorWithProviders(testProviders(p))

	_, err := g.Generate(context.Background(), cli.ManifestGenerateOptions{
		Provider:    "test-provider",
		Description: "test",
		Model:       "", // Empty -> use default.
	})
	require.NoError(t, err)
	assert.Equal(t, "provider-default-model", receivedModel,
		"when Model is empty, provider must receive its DefaultModel()")
}

// ---------------------------------------------------------------------------
// AC-9: --no-merge flag behavior.
// ---------------------------------------------------------------------------

func TestGenerator_Generate_NoMerge_FileExists_ReturnsError(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "existing.yaml")
	err := os.WriteFile(tmpFile, []byte("existing content"), 0o644)
	require.NoError(t, err)

	p := newSuccessProvider("test-provider", "model", validManifestYAML)
	g := NewGeneratorWithProviders(testProviders(p))

	_, genErr := g.Generate(context.Background(), cli.ManifestGenerateOptions{
		Provider:    "test-provider",
		Description: "test",
		OutputPath:  tmpFile,
		NoMerge:     true,
	})
	require.Error(t, genErr, "NoMerge=true with existing file must error")
	assert.Contains(t, genErr.Error(), tmpFile,
		"error must mention the output file path")
}

func TestGenerator_Generate_NoMerge_FileNotExists_NoError(t *testing.T) {
	nonexistent := filepath.Join(t.TempDir(), "does-not-exist.yaml")

	p := newSuccessProvider("test-provider", "model", validManifestYAML)
	g := NewGeneratorWithProviders(testProviders(p))

	result, err := g.Generate(context.Background(), cli.ManifestGenerateOptions{
		Provider:    "test-provider",
		Description: "test",
		OutputPath:  nonexistent,
		NoMerge:     true,
	})
	require.NoError(t, err, "NoMerge=true with non-existent file must not error")
	require.NotNil(t, result)
}

func TestGenerator_Generate_NoMerge_False_FileExists_NoError(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "existing.yaml")
	err := os.WriteFile(tmpFile, []byte("existing content"), 0o644)
	require.NoError(t, err)

	p := newSuccessProvider("test-provider", "model", validManifestYAML)
	g := NewGeneratorWithProviders(testProviders(p))

	result, genErr := g.Generate(context.Background(), cli.ManifestGenerateOptions{
		Provider:    "test-provider",
		Description: "test",
		OutputPath:  tmpFile,
		NoMerge:     false,
	})
	require.NoError(t, genErr, "NoMerge=false with existing file must not error")
	require.NotNil(t, result)
}

// ---------------------------------------------------------------------------
// Edge: invalid manifest YAML on both attempts.
// ---------------------------------------------------------------------------

func TestGenerator_Generate_InvalidManifestYAML_ReturnsError(t *testing.T) {
	// YAML that parses structurally but fails manifest.Parse (wrong apiVersion).
	badManifest := `apiVersion: wrong/v99
kind: NotAToolkit
metadata:
  name: bad
`
	p, count := newCountingProvider("model", []any{
		badManifest,
		badManifest,
	})
	g := NewGeneratorWithProviders(testProviders(p))

	_, err := g.Generate(context.Background(), cli.ManifestGenerateOptions{
		Provider:    "test-provider",
		Description: "test",
	})
	require.Error(t, err, "structurally invalid manifest must produce error after retries")
	assert.Equal(t, int32(2), atomic.LoadInt32(count),
		"must attempt exactly 2 times before returning error")
}

// ---------------------------------------------------------------------------
// Edge: cancelled context propagation.
// ---------------------------------------------------------------------------

func TestGenerator_Generate_CancelledContext_ReturnsError(t *testing.T) {
	p := &mockProvider{
		name:         "test-provider",
		defaultModel: "model",
		completeFn: func(ctx context.Context, _, _ string) (string, error) {
			// Simulate respecting context cancellation.
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			default:
				return validManifestYAML, nil
			}
		},
	}
	g := NewGeneratorWithProviders(testProviders(p))

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel before calling Generate.

	_, err := g.Generate(ctx, cli.ManifestGenerateOptions{
		Provider:    "test-provider",
		Description: "test",
	})
	require.Error(t, err, "cancelled context must propagate as error")
}

// ---------------------------------------------------------------------------
// Edge: retry with fenced YAML - first attempt returns garbage in fence,
// second attempt returns valid fenced YAML.
// ---------------------------------------------------------------------------

func TestGenerator_Generate_RetryOnInvalidFencedYAML(t *testing.T) {
	invalidFenced := "```yaml\nnot: [valid: {{\n```"
	validFenced := "```yaml\n" + validManifestYAML + "```"
	p, count := newCountingProvider("model", []any{
		invalidFenced,
		validFenced,
	})
	g := NewGeneratorWithProviders(testProviders(p))

	result, err := g.Generate(context.Background(), cli.ManifestGenerateOptions{
		Provider:    "test-provider",
		Description: "test",
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, int32(2), atomic.LoadInt32(count),
		"must retry when fenced YAML is invalid")
}

// ---------------------------------------------------------------------------
// Edge: description is passed through to the prompt.
// ---------------------------------------------------------------------------

func TestGenerator_Generate_DescriptionPassedToPrompt(t *testing.T) {
	var receivedPrompt string
	p := &mockProvider{
		name:         "test-provider",
		defaultModel: "model",
		completeFn: func(_ context.Context, prompt, _ string) (string, error) {
			receivedPrompt = prompt
			return validManifestYAML, nil
		},
	}
	g := NewGeneratorWithProviders(testProviders(p))

	_, err := g.Generate(context.Background(), cli.ManifestGenerateOptions{
		Provider:    "test-provider",
		Description: "A CLI tool for managing DNS records",
	})
	require.NoError(t, err)
	assert.Contains(t, receivedPrompt, "DNS records",
		"prompt sent to provider must contain the user's description")
}
