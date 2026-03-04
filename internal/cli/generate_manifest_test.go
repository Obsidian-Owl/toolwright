package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Mock manifest generator
// ---------------------------------------------------------------------------

type mockManifestGenerator struct {
	called     bool
	calledWith ManifestGenerateOptions
	result     *ManifestGenerateResult
	err        error
}

func (m *mockManifestGenerator) Generate(_ context.Context, opts ManifestGenerateOptions) (*ManifestGenerateResult, error) {
	m.called = true
	m.calledWith = opts
	if m.err != nil {
		return nil, m.err
	}
	return m.result, nil
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// executeGenerateManifestCmd wires the generate manifest subcommand into the
// root command tree and executes it, returning stdout and the error (if any).
func executeGenerateManifestCmd(cfg *manifestGenerateConfig, args ...string) (stdout string, err error) {
	root := NewRootCommand()
	gen := &cobra.Command{
		Use:   "generate",
		Short: "Generate code from a manifest",
	}
	gen.AddCommand(newGenerateManifestCmd(cfg))
	root.AddCommand(gen)
	outBuf := &bytes.Buffer{}
	root.SetOut(outBuf)
	root.SetErr(&bytes.Buffer{})
	root.SetArgs(append([]string{"generate", "manifest"}, args...))
	execErr := root.Execute()
	return outBuf.String(), execErr
}

// defaultManifestResult returns a ManifestGenerateResult suitable for most
// tests.
func defaultManifestResult() *ManifestGenerateResult {
	return &ManifestGenerateResult{
		Manifest: "apiVersion: toolwright/v1\nkind: Toolkit\nmetadata:\n  name: generated\n  version: 1.0.0\n  description: AI-generated manifest\ntools:\n  - name: hello\n    description: Say hello\n    entrypoint: ./hello.sh\n",
		Provider: "anthropic",
	}
}

// ---------------------------------------------------------------------------
// Command structure tests
// ---------------------------------------------------------------------------

func TestGenerateManifestCmd_Use(t *testing.T) {
	cfg := &manifestGenerateConfig{}
	cmd := newGenerateManifestCmd(cfg)
	assert.Equal(t, "manifest", cmd.Use,
		"generate manifest subcommand Use field must be 'manifest'")
}

func TestGenerateManifestCmd_Short(t *testing.T) {
	cfg := &manifestGenerateConfig{}
	cmd := newGenerateManifestCmd(cfg)
	assert.NotEmpty(t, cmd.Short,
		"generate manifest subcommand must have a Short description")
}

func TestGenerateManifestCmd_HasProviderFlag(t *testing.T) {
	cfg := &manifestGenerateConfig{}
	cmd := newGenerateManifestCmd(cfg)
	f := cmd.Flags().Lookup("provider")
	require.NotNil(t, f, "generate manifest must have a --provider flag")
}

func TestGenerateManifestCmd_ProviderShortFlag(t *testing.T) {
	cfg := &manifestGenerateConfig{}
	cmd := newGenerateManifestCmd(cfg)
	f := cmd.Flags().ShorthandLookup("p")
	require.NotNil(t, f, "--provider must have -p short form")
	assert.Equal(t, "provider", f.Name,
		"-p shorthand must map to --provider, not another flag")
}

func TestGenerateManifestCmd_HasDescriptionFlag(t *testing.T) {
	cfg := &manifestGenerateConfig{}
	cmd := newGenerateManifestCmd(cfg)
	f := cmd.Flags().Lookup("description")
	require.NotNil(t, f, "generate manifest must have a --description flag")
}

func TestGenerateManifestCmd_DescriptionShortFlag(t *testing.T) {
	cfg := &manifestGenerateConfig{}
	cmd := newGenerateManifestCmd(cfg)
	f := cmd.Flags().ShorthandLookup("d")
	require.NotNil(t, f, "--description must have -d short form")
	assert.Equal(t, "description", f.Name,
		"-d shorthand must map to --description, not another flag")
}

func TestGenerateManifestCmd_HasOutputFlag(t *testing.T) {
	cfg := &manifestGenerateConfig{}
	cmd := newGenerateManifestCmd(cfg)
	f := cmd.Flags().Lookup("output")
	require.NotNil(t, f, "generate manifest must have an --output flag")
}

func TestGenerateManifestCmd_OutputShortFlag(t *testing.T) {
	cfg := &manifestGenerateConfig{}
	cmd := newGenerateManifestCmd(cfg)
	f := cmd.Flags().ShorthandLookup("o")
	require.NotNil(t, f, "--output must have -o short form")
	assert.Equal(t, "output", f.Name,
		"-o shorthand must map to --output, not another flag")
}

func TestGenerateManifestCmd_OutputDefaultIsToolwrightYaml(t *testing.T) {
	cfg := &manifestGenerateConfig{}
	cmd := newGenerateManifestCmd(cfg)
	f := cmd.Flags().Lookup("output")
	require.NotNil(t, f)
	assert.Equal(t, "toolwright.yaml", f.DefValue,
		"--output default must be 'toolwright.yaml'")
}

func TestGenerateManifestCmd_HasDryRunFlag(t *testing.T) {
	cfg := &manifestGenerateConfig{}
	cmd := newGenerateManifestCmd(cfg)
	f := cmd.Flags().Lookup("dry-run")
	require.NotNil(t, f, "generate manifest must have a --dry-run flag")
	assert.Equal(t, "false", f.DefValue,
		"--dry-run default must be false")
}

// ---------------------------------------------------------------------------
// Provider validation — valid providers (table-driven per Constitution 9)
// ---------------------------------------------------------------------------

func TestGenerateManifest_ValidProviders(t *testing.T) {
	tests := []struct {
		provider string
	}{
		{provider: "anthropic"},
		{provider: "openai"},
		{provider: "gemini"},
	}

	for _, tc := range tests {
		t.Run(tc.provider, func(t *testing.T) {
			mock := &mockManifestGenerator{
				result: &ManifestGenerateResult{
					Manifest: "apiVersion: toolwright/v1\nkind: Toolkit\n",
					Provider: tc.provider,
				},
			}
			cfg := &manifestGenerateConfig{Generator: mock}

			_, err := executeGenerateManifestCmd(cfg, "--provider", tc.provider, "--dry-run")
			require.NoError(t, err,
				"provider %q must be accepted without error", tc.provider)
			require.True(t, mock.called,
				"generator must be called for valid provider %q", tc.provider)
			assert.Equal(t, tc.provider, mock.calledWith.Provider,
				"provider %q must be passed through to generator options", tc.provider)
		})
	}
}

// ---------------------------------------------------------------------------
// Provider validation — missing provider
// ---------------------------------------------------------------------------

func TestGenerateManifest_NoProvider_Error(t *testing.T) {
	mock := &mockManifestGenerator{result: defaultManifestResult()}
	cfg := &manifestGenerateConfig{Generator: mock}

	_, err := executeGenerateManifestCmd(cfg)
	require.Error(t, err,
		"omitting --provider must produce an error")
	assert.False(t, mock.called,
		"generator must not be called when --provider is missing")
}

// ---------------------------------------------------------------------------
// Provider validation — invalid providers (table-driven)
// ---------------------------------------------------------------------------

func TestGenerateManifest_InvalidProviders(t *testing.T) {
	tests := []struct {
		name     string
		provider string
	}{
		{name: "unknown_provider", provider: "invalid"},
		{name: "empty_string", provider: ""},
		{name: "close_misspelling", provider: "antropic"},
		{name: "azure_not_supported", provider: "azure"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mock := &mockManifestGenerator{result: defaultManifestResult()}
			cfg := &manifestGenerateConfig{Generator: mock}

			_, err := executeGenerateManifestCmd(cfg, "--provider", tc.provider, "--dry-run")
			require.Error(t, err,
				"provider %q must be rejected", tc.provider)
			assert.False(t, mock.called,
				"generator must not be called for invalid provider %q", tc.provider)
		})
	}
}

func TestGenerateManifest_InvalidProvider_ErrorMentionsValidProviders(t *testing.T) {
	mock := &mockManifestGenerator{result: defaultManifestResult()}
	cfg := &manifestGenerateConfig{Generator: mock}

	_, err := executeGenerateManifestCmd(cfg, "--provider", "invalid", "--dry-run")
	require.Error(t, err)

	errMsg := err.Error()
	assert.Contains(t, errMsg, "anthropic",
		"error for invalid provider must mention 'anthropic' as a valid option")
	assert.Contains(t, errMsg, "openai",
		"error for invalid provider must mention 'openai' as a valid option")
	assert.Contains(t, errMsg, "gemini",
		"error for invalid provider must mention 'gemini' as a valid option")
}

// ---------------------------------------------------------------------------
// Provider validation — case sensitivity (table-driven)
// ---------------------------------------------------------------------------

func TestGenerateManifest_ProviderCaseSensitive(t *testing.T) {
	tests := []struct {
		name     string
		provider string
	}{
		{name: "capital_A", provider: "Anthropic"},
		{name: "all_caps_OPENAI", provider: "OPENAI"},
		{name: "mixed_case_Gemini", provider: "Gemini"},
		{name: "all_caps_ANTHROPIC", provider: "ANTHROPIC"},
		{name: "title_case_Openai", provider: "Openai"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mock := &mockManifestGenerator{result: defaultManifestResult()}
			cfg := &manifestGenerateConfig{Generator: mock}

			_, err := executeGenerateManifestCmd(cfg, "--provider", tc.provider, "--dry-run")
			require.Error(t, err,
				"provider %q (wrong case) must be rejected — validation must be case-sensitive", tc.provider)
			assert.False(t, mock.called,
				"generator must not be called for case-variant %q", tc.provider)
		})
	}
}

// ---------------------------------------------------------------------------
// Generator delegation — options passthrough
// ---------------------------------------------------------------------------

func TestGenerateManifest_OptionsPassthrough(t *testing.T) {
	mock := &mockManifestGenerator{
		result: &ManifestGenerateResult{
			Manifest: "apiVersion: toolwright/v1\nkind: Toolkit\n",
			Provider: "anthropic",
		},
	}
	cfg := &manifestGenerateConfig{Generator: mock}

	_, err := executeGenerateManifestCmd(cfg,
		"--provider", "anthropic",
		"--description", "A weather forecast toolkit",
		"--output", "custom-manifest.yaml",
		"--dry-run",
	)
	require.NoError(t, err)
	require.True(t, mock.called, "generator must be called")

	assert.Equal(t, "anthropic", mock.calledWith.Provider,
		"Provider must be passed through to generator")
	assert.Equal(t, "A weather forecast toolkit", mock.calledWith.Description,
		"Description must be passed through to generator")
	assert.Equal(t, "custom-manifest.yaml", mock.calledWith.OutputPath,
		"OutputPath must be set to the --output value")
	assert.True(t, mock.calledWith.DryRun,
		"DryRun must be true when --dry-run is set")
}

func TestGenerateManifest_DefaultOutputPath_PassedToGenerator(t *testing.T) {
	mock := &mockManifestGenerator{
		result: &ManifestGenerateResult{
			Manifest: "apiVersion: toolwright/v1\n",
			Provider: "openai",
		},
	}
	cfg := &manifestGenerateConfig{Generator: mock}

	_, err := executeGenerateManifestCmd(cfg, "--provider", "openai", "--dry-run")
	require.NoError(t, err)
	require.True(t, mock.called)
	assert.Equal(t, "toolwright.yaml", mock.calledWith.OutputPath,
		"default output path must be 'toolwright.yaml' when --output is not specified")
}

func TestGenerateManifest_EmptyDescription_PassedToGenerator(t *testing.T) {
	mock := &mockManifestGenerator{
		result: &ManifestGenerateResult{
			Manifest: "apiVersion: toolwright/v1\n",
			Provider: "gemini",
		},
	}
	cfg := &manifestGenerateConfig{Generator: mock}

	_, err := executeGenerateManifestCmd(cfg, "--provider", "gemini", "--dry-run")
	require.NoError(t, err)
	require.True(t, mock.called)
	assert.Equal(t, "", mock.calledWith.Description,
		"description must default to empty string when --description is not specified")
}

func TestGenerateManifest_GeneratorCalledExactlyOnce(t *testing.T) {
	mock := &mockManifestGenerator{
		result: defaultManifestResult(),
	}
	cfg := &manifestGenerateConfig{Generator: mock}

	_, err := executeGenerateManifestCmd(cfg, "--provider", "anthropic", "--dry-run")
	require.NoError(t, err)
	require.True(t, mock.called,
		"generator must be called at least once")
}

// countingManifestGenerator counts the number of times Generate is called.
type countingManifestGenerator struct {
	callCount int
	result    *ManifestGenerateResult
	err       error
}

func (c *countingManifestGenerator) Generate(_ context.Context, _ ManifestGenerateOptions) (*ManifestGenerateResult, error) {
	c.callCount++
	return c.result, c.err
}

func TestGenerateManifest_GeneratorCalledExactlyOnce_Counting(t *testing.T) {
	gen := &countingManifestGenerator{
		result: defaultManifestResult(),
	}
	cfg := &manifestGenerateConfig{Generator: gen}

	_, err := executeGenerateManifestCmd(cfg, "--provider", "anthropic", "--dry-run")
	require.NoError(t, err)
	assert.Equal(t, 1, gen.callCount,
		"generator must be called exactly once, not %d times", gen.callCount)
}

// ---------------------------------------------------------------------------
// Generator error propagation
// ---------------------------------------------------------------------------

func TestGenerateManifest_GeneratorError_Propagated(t *testing.T) {
	mock := &mockManifestGenerator{
		err: errors.New("API rate limit exceeded"),
	}
	cfg := &manifestGenerateConfig{Generator: mock}

	_, err := executeGenerateManifestCmd(cfg, "--provider", "anthropic", "--dry-run")
	require.Error(t, err,
		"generator error must propagate to the command")
	assert.Contains(t, err.Error(), "API rate limit exceeded",
		"propagated error must preserve the original error message")
}

func TestGenerateManifest_GeneratorError_DifferentMessages(t *testing.T) {
	// Anti-hardcoding: different errors must produce different results.
	tests := []struct {
		name string
		err  error
	}{
		{name: "rate_limit", err: errors.New("API rate limit exceeded")},
		{name: "auth_failure", err: errors.New("invalid API key")},
		{name: "network_error", err: errors.New("connection refused")},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mock := &mockManifestGenerator{err: tc.err}
			cfg := &manifestGenerateConfig{Generator: mock}

			_, err := executeGenerateManifestCmd(cfg, "--provider", "anthropic", "--dry-run")
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.err.Error(),
				"error message must contain the original error text")
		})
	}
}

// ---------------------------------------------------------------------------
// Dry-run mode
// ---------------------------------------------------------------------------

func TestGenerateManifest_DryRun_SetsFlag(t *testing.T) {
	mock := &mockManifestGenerator{
		result: defaultManifestResult(),
	}
	cfg := &manifestGenerateConfig{Generator: mock}

	_, err := executeGenerateManifestCmd(cfg, "--provider", "anthropic", "--dry-run")
	require.NoError(t, err)
	require.True(t, mock.called)
	assert.True(t, mock.calledWith.DryRun,
		"--dry-run must set DryRun=true in generator options")
}

func TestGenerateManifest_NoDryRun_DefaultFalse(t *testing.T) {
	mock := &mockManifestGenerator{
		result: defaultManifestResult(),
	}
	cfg := &manifestGenerateConfig{Generator: mock}

	_, err := executeGenerateManifestCmd(cfg, "--provider", "anthropic")
	require.NoError(t, err)
	require.True(t, mock.called)
	assert.False(t, mock.calledWith.DryRun,
		"DryRun must default to false when --dry-run is not specified")
}

func TestGenerateManifest_DryRun_OutputContainsManifest(t *testing.T) {
	expectedYAML := "apiVersion: toolwright/v1\nkind: Toolkit\nmetadata:\n  name: my-tool\n"
	mock := &mockManifestGenerator{
		result: &ManifestGenerateResult{
			Manifest: expectedYAML,
			Provider: "anthropic",
		},
	}
	cfg := &manifestGenerateConfig{Generator: mock}

	stdout, err := executeGenerateManifestCmd(cfg, "--provider", "anthropic", "--dry-run")
	require.NoError(t, err)
	assert.Contains(t, stdout, "apiVersion: toolwright/v1",
		"dry-run stdout must contain the manifest YAML content")
	assert.Contains(t, stdout, "name: my-tool",
		"dry-run stdout must contain specific manifest content, not a generic message")
}

func TestGenerateManifest_DryRun_ManifestContentVaries(t *testing.T) {
	// Anti-hardcoding: different manifest content must produce different stdout.
	manifests := []string{
		"apiVersion: toolwright/v1\nkind: Toolkit\nmetadata:\n  name: alpha\n",
		"apiVersion: toolwright/v1\nkind: Toolkit\nmetadata:\n  name: beta\n",
	}

	outputs := make([]string, 0, len(manifests))
	for _, m := range manifests {
		mock := &mockManifestGenerator{
			result: &ManifestGenerateResult{
				Manifest: m,
				Provider: "anthropic",
			},
		}
		cfg := &manifestGenerateConfig{Generator: mock}
		stdout, err := executeGenerateManifestCmd(cfg, "--provider", "anthropic", "--dry-run")
		require.NoError(t, err)
		outputs = append(outputs, stdout)
	}

	assert.NotEqual(t, outputs[0], outputs[1],
		"different manifest content must produce different dry-run output")
}

// ---------------------------------------------------------------------------
// Output path
// ---------------------------------------------------------------------------

func TestGenerateManifest_CustomOutputPath(t *testing.T) {
	mock := &mockManifestGenerator{
		result: defaultManifestResult(),
	}
	cfg := &manifestGenerateConfig{Generator: mock}

	_, err := executeGenerateManifestCmd(cfg, "--provider", "anthropic", "--output", "my-manifest.yaml")
	require.NoError(t, err)
	require.True(t, mock.called)
	assert.Equal(t, "my-manifest.yaml", mock.calledWith.OutputPath,
		"--output value must be passed as OutputPath to generator")
}

func TestGenerateManifest_OutputShortForm(t *testing.T) {
	mock := &mockManifestGenerator{
		result: defaultManifestResult(),
	}
	cfg := &manifestGenerateConfig{Generator: mock}

	_, err := executeGenerateManifestCmd(cfg, "-p", "anthropic", "-o", "short.yaml", "--dry-run")
	require.NoError(t, err)
	require.True(t, mock.called)
	assert.Equal(t, "short.yaml", mock.calledWith.OutputPath,
		"-o shorthand must work the same as --output")
}

// ---------------------------------------------------------------------------
// JSON mode — success
// ---------------------------------------------------------------------------

func TestGenerateManifest_JSONMode_Success(t *testing.T) {
	mock := &mockManifestGenerator{
		result: &ManifestGenerateResult{
			Manifest: "apiVersion: toolwright/v1\nkind: Toolkit\n",
			Provider: "anthropic",
		},
	}
	cfg := &manifestGenerateConfig{Generator: mock}

	stdout, err := executeGenerateManifestCmd(cfg, "--provider", "anthropic", "--dry-run", "--json")
	require.NoError(t, err)

	require.True(t, json.Valid([]byte(stdout)),
		"--json mode must produce valid JSON, got: %s", stdout)

	var got map[string]any
	err = json.Unmarshal([]byte(stdout), &got)
	require.NoError(t, err)

	assert.Contains(t, got, "provider",
		"JSON success output must include 'provider' field")
	assert.Equal(t, "anthropic", got["provider"],
		"JSON provider field must match the provider used")

	assert.Contains(t, got, "manifest",
		"JSON success output must include 'manifest' field")
	manifest, ok := got["manifest"].(string)
	require.True(t, ok, "manifest field must be a string")
	assert.Contains(t, manifest, "apiVersion: toolwright/v1",
		"manifest field must contain the actual manifest content")
}

func TestGenerateManifest_JSONMode_Success_ProviderVaries(t *testing.T) {
	// Anti-hardcoding: different providers must produce different JSON.
	tests := []struct {
		provider string
	}{
		{provider: "anthropic"},
		{provider: "openai"},
		{provider: "gemini"},
	}

	for _, tc := range tests {
		t.Run(tc.provider, func(t *testing.T) {
			mock := &mockManifestGenerator{
				result: &ManifestGenerateResult{
					Manifest: "apiVersion: toolwright/v1\n",
					Provider: tc.provider,
				},
			}
			cfg := &manifestGenerateConfig{Generator: mock}

			stdout, err := executeGenerateManifestCmd(cfg, "--provider", tc.provider, "--dry-run", "--json")
			require.NoError(t, err)

			var got map[string]any
			err = json.Unmarshal([]byte(stdout), &got)
			require.NoError(t, err)
			assert.Equal(t, tc.provider, got["provider"],
				"JSON output provider must reflect the actual provider %q", tc.provider)
		})
	}
}

// ---------------------------------------------------------------------------
// JSON mode — errors
// ---------------------------------------------------------------------------

func TestGenerateManifest_JSONMode_GeneratorError(t *testing.T) {
	mock := &mockManifestGenerator{
		err: errors.New("model context window exceeded"),
	}
	cfg := &manifestGenerateConfig{Generator: mock}

	stdout, err := executeGenerateManifestCmd(cfg, "--provider", "anthropic", "--dry-run", "--json")
	require.Error(t, err,
		"generator error must still propagate even in JSON mode")

	require.True(t, json.Valid([]byte(stdout)),
		"--json error mode must produce valid JSON, got: %s", stdout)

	var got map[string]any
	err2 := json.Unmarshal([]byte(stdout), &got)
	require.NoError(t, err2)

	errObj, ok := got["error"].(map[string]any)
	require.True(t, ok,
		"JSON error output must have top-level 'error' object, got keys: %v", mapKeysFromAny(got))

	assert.Contains(t, errObj, "code",
		"error object must have 'code' field")
	assert.Contains(t, errObj, "message",
		"error object must have 'message' field")

	msg, ok := errObj["message"].(string)
	require.True(t, ok)
	assert.Contains(t, msg, "model context window exceeded",
		"JSON error message must include the original error text")
}

func TestGenerateManifest_JSONMode_MissingProvider(t *testing.T) {
	mock := &mockManifestGenerator{result: defaultManifestResult()}
	cfg := &manifestGenerateConfig{Generator: mock}

	stdout, err := executeGenerateManifestCmd(cfg, "--json")
	require.Error(t, err,
		"missing --provider must error even in JSON mode")

	require.True(t, json.Valid([]byte(stdout)),
		"--json mode for missing provider must produce valid JSON, got: %s", stdout)

	var got map[string]any
	err2 := json.Unmarshal([]byte(stdout), &got)
	require.NoError(t, err2)

	errObj, ok := got["error"].(map[string]any)
	require.True(t, ok,
		"JSON error output must have top-level 'error' object")
	assert.Contains(t, errObj, "code",
		"error object must have 'code' field")
	assert.Contains(t, errObj, "message",
		"error object must have 'message' field")
}

func TestGenerateManifest_JSONMode_InvalidProvider(t *testing.T) {
	mock := &mockManifestGenerator{result: defaultManifestResult()}
	cfg := &manifestGenerateConfig{Generator: mock}

	stdout, err := executeGenerateManifestCmd(cfg, "--provider", "invalid", "--json")
	require.Error(t, err,
		"invalid --provider must error even in JSON mode")

	require.True(t, json.Valid([]byte(stdout)),
		"--json mode for invalid provider must produce valid JSON, got: %s", stdout)

	var got map[string]any
	err2 := json.Unmarshal([]byte(stdout), &got)
	require.NoError(t, err2)

	_, ok := got["error"].(map[string]any)
	require.True(t, ok,
		"JSON error output for invalid provider must have top-level 'error' object")
}

// ---------------------------------------------------------------------------
// Human output — success messages
// ---------------------------------------------------------------------------

func TestGenerateManifest_HumanOutput_MentionsProvider(t *testing.T) {
	mock := &mockManifestGenerator{
		result: &ManifestGenerateResult{
			Manifest: "apiVersion: toolwright/v1\n",
			Provider: "anthropic",
		},
	}
	cfg := &manifestGenerateConfig{Generator: mock}

	stdout, err := executeGenerateManifestCmd(cfg, "--provider", "anthropic")
	require.NoError(t, err)
	assert.Contains(t, strings.ToLower(stdout), "anthropic",
		"human output must mention the provider used")
}

func TestGenerateManifest_HumanOutput_MentionsOutputPath(t *testing.T) {
	mock := &mockManifestGenerator{
		result: &ManifestGenerateResult{
			Manifest: "apiVersion: toolwright/v1\n",
			Provider: "openai",
		},
	}
	cfg := &manifestGenerateConfig{Generator: mock}

	stdout, err := executeGenerateManifestCmd(cfg, "--provider", "openai", "--output", "custom.yaml")
	require.NoError(t, err)
	assert.Contains(t, stdout, "custom.yaml",
		"human output must mention the output file path")
}

func TestGenerateManifest_HumanOutput_DryRun_MentionsStdout(t *testing.T) {
	mock := &mockManifestGenerator{
		result: &ManifestGenerateResult{
			Manifest: "apiVersion: toolwright/v1\nkind: Toolkit\nmetadata:\n  name: test\n",
			Provider: "anthropic",
		},
	}
	cfg := &manifestGenerateConfig{Generator: mock}

	stdout, err := executeGenerateManifestCmd(cfg, "--provider", "anthropic", "--dry-run")
	require.NoError(t, err)
	// In dry-run mode, the manifest YAML content should be printed directly.
	assert.Contains(t, stdout, "apiVersion: toolwright/v1",
		"dry-run human output must contain the manifest YAML content")
}

func TestGenerateManifest_HumanOutput_DifferentProviders_DifferentOutput(t *testing.T) {
	// Anti-hardcoding: different providers must produce different human output.
	outputs := make(map[string]string)
	for _, provider := range []string{"anthropic", "openai", "gemini"} {
		mock := &mockManifestGenerator{
			result: &ManifestGenerateResult{
				Manifest: "apiVersion: toolwright/v1\n",
				Provider: provider,
			},
		}
		cfg := &manifestGenerateConfig{Generator: mock}
		stdout, err := executeGenerateManifestCmd(cfg, "--provider", provider)
		require.NoError(t, err)
		outputs[provider] = stdout
	}

	// Each provider's output should be unique (they should at minimum mention
	// the provider name).
	assert.NotEqual(t, outputs["anthropic"], outputs["openai"],
		"anthropic and openai must produce different human output")
	assert.NotEqual(t, outputs["openai"], outputs["gemini"],
		"openai and gemini must produce different human output")
}

// ---------------------------------------------------------------------------
// Edge cases
// ---------------------------------------------------------------------------

func TestGenerateManifest_DescriptionWithSpecialChars(t *testing.T) {
	mock := &mockManifestGenerator{
		result: defaultManifestResult(),
	}
	cfg := &manifestGenerateConfig{Generator: mock}

	desc := `A toolkit for "weather" & <climate> data`
	_, err := executeGenerateManifestCmd(cfg, "--provider", "anthropic", "--description", desc, "--dry-run")
	require.NoError(t, err)
	require.True(t, mock.called)
	assert.Equal(t, desc, mock.calledWith.Description,
		"special characters in --description must be preserved exactly")
}

func TestGenerateManifest_DescriptionWithUnicode(t *testing.T) {
	mock := &mockManifestGenerator{
		result: defaultManifestResult(),
	}
	cfg := &manifestGenerateConfig{Generator: mock}

	desc := "A toolkit for \u65e5\u672c\u8a9e data"
	_, err := executeGenerateManifestCmd(cfg, "--provider", "anthropic", "--description", desc, "--dry-run")
	require.NoError(t, err)
	require.True(t, mock.called)
	assert.Equal(t, desc, mock.calledWith.Description,
		"Unicode in --description must be preserved")
}

func TestGenerateManifest_ProviderShortForm(t *testing.T) {
	mock := &mockManifestGenerator{
		result: defaultManifestResult(),
	}
	cfg := &manifestGenerateConfig{Generator: mock}

	_, err := executeGenerateManifestCmd(cfg, "-p", "anthropic", "--dry-run")
	require.NoError(t, err)
	require.True(t, mock.called)
	assert.Equal(t, "anthropic", mock.calledWith.Provider,
		"-p shorthand must correctly set the provider")
}

func TestGenerateManifest_DescriptionShortForm(t *testing.T) {
	mock := &mockManifestGenerator{
		result: defaultManifestResult(),
	}
	cfg := &manifestGenerateConfig{Generator: mock}

	_, err := executeGenerateManifestCmd(cfg, "-p", "anthropic", "-d", "My toolkit", "--dry-run")
	require.NoError(t, err)
	require.True(t, mock.called)
	assert.Equal(t, "My toolkit", mock.calledWith.Description,
		"-d shorthand must correctly set the description")
}

func TestGenerateManifest_AllShortForms(t *testing.T) {
	mock := &mockManifestGenerator{
		result: defaultManifestResult(),
	}
	cfg := &manifestGenerateConfig{Generator: mock}

	_, err := executeGenerateManifestCmd(cfg,
		"-p", "gemini",
		"-d", "Short form toolkit",
		"-o", "short.yaml",
		"--dry-run",
	)
	require.NoError(t, err)
	require.True(t, mock.called)
	assert.Equal(t, "gemini", mock.calledWith.Provider)
	assert.Equal(t, "Short form toolkit", mock.calledWith.Description)
	assert.Equal(t, "short.yaml", mock.calledWith.OutputPath)
	assert.True(t, mock.calledWith.DryRun)
}

// ---------------------------------------------------------------------------
// Verify the command integrates as a child of "generate"
// ---------------------------------------------------------------------------

func TestGenerateManifest_SubcommandIntegration(t *testing.T) {
	// The manifest command must be reachable through "generate manifest"
	// in the command tree.
	mock := &mockManifestGenerator{
		result: defaultManifestResult(),
	}
	cfg := &manifestGenerateConfig{Generator: mock}

	// executeGenerateManifestCmd already wires it under "generate manifest"
	_, err := executeGenerateManifestCmd(cfg, "--provider", "anthropic", "--dry-run")
	require.NoError(t, err,
		"generate manifest must be reachable as a subcommand of generate")
}

// ---------------------------------------------------------------------------
// Generator not called on validation failure
// ---------------------------------------------------------------------------

func TestGenerateManifest_InvalidProvider_GeneratorNotCalled(t *testing.T) {
	mock := &mockManifestGenerator{
		result: defaultManifestResult(),
	}
	cfg := &manifestGenerateConfig{Generator: mock}

	_, _ = executeGenerateManifestCmd(cfg, "--provider", "bogus")
	assert.False(t, mock.called,
		"generator must never be called when provider validation fails")
}

func TestGenerateManifest_MissingProvider_GeneratorNotCalled(t *testing.T) {
	mock := &mockManifestGenerator{
		result: defaultManifestResult(),
	}
	cfg := &manifestGenerateConfig{Generator: mock}

	_, _ = executeGenerateManifestCmd(cfg)
	assert.False(t, mock.called,
		"generator must never be called when --provider is missing")
}

// ---------------------------------------------------------------------------
// JSON mode — no JSON pollution in non-JSON mode
// ---------------------------------------------------------------------------

func TestGenerateManifest_NonJSONMode_NoJSONInOutput(t *testing.T) {
	mock := &mockManifestGenerator{
		result: defaultManifestResult(),
	}
	cfg := &manifestGenerateConfig{Generator: mock}

	stdout, err := executeGenerateManifestCmd(cfg, "--provider", "anthropic")
	require.NoError(t, err)

	// Non-JSON mode should not produce JSON output.
	trimmed := strings.TrimSpace(stdout)
	if len(trimmed) > 0 {
		assert.False(t, json.Valid([]byte(trimmed)) && strings.HasPrefix(trimmed, "{"),
			"non-JSON mode must not produce JSON-formatted output; got: %s", trimmed)
	}
}

// ---------------------------------------------------------------------------
// Helper function
// ---------------------------------------------------------------------------

// mapKeysFromAny returns the keys of a map[string]any for diagnostic output.
func mapKeysFromAny(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
