package codegen

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Obsidian-Owl/toolwright/internal/manifest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Test double: mockGenerator records calls and returns configurable output.
// ---------------------------------------------------------------------------

type mockGenerator struct {
	mode   string
	target string

	mu    sync.Mutex
	calls []mockGenerateCall
	files []GeneratedFile
	err   error
}

type mockGenerateCall struct {
	Data      TemplateData
	OutputDir string
}

func newMockGenerator(mode, target string, files []GeneratedFile) *mockGenerator {
	return &mockGenerator{
		mode:   mode,
		target: target,
		files:  files,
	}
}

func (m *mockGenerator) Mode() string   { return m.mode }
func (m *mockGenerator) Target() string { return m.target }

func (m *mockGenerator) Generate(ctx context.Context, data TemplateData, outputDir string) ([]GeneratedFile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, mockGenerateCall{Data: data, OutputDir: outputDir})
	if m.err != nil {
		return nil, m.err
	}
	return m.files, nil
}

func (m *mockGenerator) callCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.calls)
}

func (m *mockGenerator) lastCall() mockGenerateCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls[len(m.calls)-1]
}

// ---------------------------------------------------------------------------
// Helper: minimal manifest for test usage
// ---------------------------------------------------------------------------

func testManifest() manifest.Toolkit {
	return manifest.Toolkit{
		APIVersion: "toolwright/v1",
		Kind:       "Toolkit",
		Metadata: manifest.Metadata{
			Name:        "test-toolkit",
			Version:     "1.0.0",
			Description: "A test toolkit",
		},
		Tools: []manifest.Tool{
			{
				Name:        "hello",
				Description: "Says hello",
				Entrypoint:  "./hello.sh",
			},
		},
	}
}

// markerFileName is the expected marker file name. Defined once so tests
// break obviously if the name ever changes.
const markerFileName = ".toolwright-generated"

// ---------------------------------------------------------------------------
// AC-1: Engine.Register + dispatch by mode+target
// ---------------------------------------------------------------------------

func TestEngine_Generate_DispatchesToRegisteredGenerator(t *testing.T) {
	goFiles := []GeneratedFile{
		{Path: "main.go", Content: []byte("package main")},
	}
	gen := newMockGenerator("cli", "go", goFiles)

	eng := NewEngine()
	eng.Register(gen)

	dir := t.TempDir()
	result, err := eng.Generate(context.Background(), testManifest(), GenerateOptions{
		Mode:      "cli",
		Target:    "go",
		OutputDir: dir,
		Version:   "0.1.0",
	})

	require.NoError(t, err)
	require.NotNil(t, result, "Generate must return a non-nil result")
	assert.Equal(t, 1, gen.callCount(), "Generator.Generate must be called exactly once")
	// Result must list the file the generator produced.
	assert.Contains(t, result.Files, "main.go",
		"Result.Files should contain the file returned by the generator")
}

func TestEngine_Generate_DispatchesCorrectModeTarget(t *testing.T) {
	tests := []struct {
		name       string
		mode       string
		target     string
		wantMode   string
		wantTarget string
	}{
		{
			name:       "cli/go dispatches to cli/go generator",
			mode:       "cli",
			target:     "go",
			wantMode:   "cli",
			wantTarget: "go",
		},
		{
			name:       "mcp/typescript dispatches to mcp/typescript generator",
			mode:       "mcp",
			target:     "typescript",
			wantMode:   "mcp",
			wantTarget: "typescript",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gen := newMockGenerator(tc.mode, tc.target, []GeneratedFile{
				{Path: "out.txt", Content: []byte("content")},
			})

			eng := NewEngine()
			eng.Register(gen)

			dir := t.TempDir()
			result, err := eng.Generate(context.Background(), testManifest(), GenerateOptions{
				Mode:      tc.mode,
				Target:    tc.target,
				OutputDir: dir,
				Version:   "0.1.0",
			})

			require.NoError(t, err)
			require.NotNil(t, result)
			assert.Equal(t, tc.wantMode, result.Mode, "Result.Mode should match requested mode")
			assert.Equal(t, tc.wantTarget, result.Target, "Result.Target should match requested target")
			assert.Equal(t, 1, gen.callCount(),
				"Only the matching generator should be called")
		})
	}
}

func TestEngine_Generate_MultipleGenerators_OnlyCallsMatching(t *testing.T) {
	goGen := newMockGenerator("cli", "go", []GeneratedFile{
		{Path: "main.go", Content: []byte("package main")},
	})
	tsGen := newMockGenerator("mcp", "typescript", []GeneratedFile{
		{Path: "index.ts", Content: []byte("export {}")},
	})

	eng := NewEngine()
	eng.Register(goGen)
	eng.Register(tsGen)

	dir := t.TempDir()
	result, err := eng.Generate(context.Background(), testManifest(), GenerateOptions{
		Mode:      "mcp",
		Target:    "typescript",
		OutputDir: dir,
		Version:   "0.1.0",
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 0, goGen.callCount(),
		"cli/go generator must NOT be called when mcp/typescript is requested")
	assert.Equal(t, 1, tsGen.callCount(),
		"mcp/typescript generator must be called")
	assert.Contains(t, result.Files, "index.ts")
	assert.NotContains(t, result.Files, "main.go",
		"Result must only contain files from the matched generator")
}

func TestEngine_Generate_PassesTemplateDataToGenerator(t *testing.T) {
	gen := newMockGenerator("cli", "go", []GeneratedFile{
		{Path: "out.go", Content: []byte("package out")},
	})

	eng := NewEngine()
	eng.Register(gen)

	m := testManifest()
	dir := t.TempDir()
	_, err := eng.Generate(context.Background(), m, GenerateOptions{
		Mode:      "cli",
		Target:    "go",
		OutputDir: dir,
		Version:   "2.5.0",
	})

	require.NoError(t, err)
	require.Equal(t, 1, gen.callCount())

	call := gen.lastCall()
	assert.Equal(t, m, call.Data.Manifest,
		"TemplateData.Manifest must be the manifest passed to Generate")
	assert.Equal(t, "2.5.0", call.Data.Version,
		"TemplateData.Version must match opts.Version")
	assert.NotEmpty(t, call.Data.Timestamp,
		"TemplateData.Timestamp must be set by the engine")
}

func TestEngine_Generate_PassesOutputDirToGenerator(t *testing.T) {
	gen := newMockGenerator("cli", "go", []GeneratedFile{
		{Path: "out.go", Content: []byte("package out")},
	})

	eng := NewEngine()
	eng.Register(gen)

	dir := t.TempDir()
	_, err := eng.Generate(context.Background(), testManifest(), GenerateOptions{
		Mode:      "cli",
		Target:    "go",
		OutputDir: dir,
		Version:   "0.1.0",
	})

	require.NoError(t, err)
	require.Equal(t, 1, gen.callCount())
	assert.Equal(t, dir, gen.lastCall().OutputDir,
		"Generator must receive the output directory from options")
}

// ---------------------------------------------------------------------------
// AC-1: Unknown mode/target combination returns error listing available options
// ---------------------------------------------------------------------------

func TestEngine_Generate_UnknownMode_ReturnsError(t *testing.T) {
	gen := newMockGenerator("cli", "go", nil)

	eng := NewEngine()
	eng.Register(gen)

	dir := t.TempDir()
	result, err := eng.Generate(context.Background(), testManifest(), GenerateOptions{
		Mode:      "nonexistent",
		Target:    "go",
		OutputDir: dir,
		Version:   "0.1.0",
	})

	require.Error(t, err, "Generate must error for unknown mode")
	assert.Nil(t, result, "Result must be nil on error")
	// Error must list available options so user knows what to fix (Constitution 22).
	assert.Contains(t, err.Error(), "cli",
		"Error for unknown mode should list available mode 'cli'")
}

func TestEngine_Generate_UnknownTarget_ReturnsError(t *testing.T) {
	gen := newMockGenerator("cli", "go", nil)

	eng := NewEngine()
	eng.Register(gen)

	dir := t.TempDir()
	result, err := eng.Generate(context.Background(), testManifest(), GenerateOptions{
		Mode:      "cli",
		Target:    "rust",
		OutputDir: dir,
		Version:   "0.1.0",
	})

	require.Error(t, err, "Generate must error for unknown target")
	assert.Nil(t, result, "Result must be nil on error")
	// Error must list available targets so user knows what to fix.
	assert.Contains(t, err.Error(), "go",
		"Error for unknown target should list available target 'go'")
}

func TestEngine_Generate_UnknownModeAndTarget_ListsAllAvailable(t *testing.T) {
	goGen := newMockGenerator("cli", "go", nil)
	tsGen := newMockGenerator("mcp", "typescript", nil)

	eng := NewEngine()
	eng.Register(goGen)
	eng.Register(tsGen)

	dir := t.TempDir()
	_, err := eng.Generate(context.Background(), testManifest(), GenerateOptions{
		Mode:      "api",
		Target:    "python",
		OutputDir: dir,
		Version:   "0.1.0",
	})

	require.Error(t, err)
	errMsg := err.Error()
	// Must list all registered mode/target combos.
	assert.Contains(t, errMsg, "cli",
		"Error should list available mode 'cli'")
	assert.Contains(t, errMsg, "go",
		"Error should list available target 'go'")
	assert.Contains(t, errMsg, "mcp",
		"Error should list available mode 'mcp'")
	assert.Contains(t, errMsg, "typescript",
		"Error should list available target 'typescript'")
}

func TestEngine_Generate_NoRegisteredGenerators_ReturnsError(t *testing.T) {
	eng := NewEngine()

	dir := t.TempDir()
	result, err := eng.Generate(context.Background(), testManifest(), GenerateOptions{
		Mode:      "cli",
		Target:    "go",
		OutputDir: dir,
		Version:   "0.1.0",
	})

	require.Error(t, err, "Generate with no registered generators must error")
	assert.Nil(t, result)
}

// ---------------------------------------------------------------------------
// AC-1: Generator error propagation
// ---------------------------------------------------------------------------

func TestEngine_Generate_GeneratorError_PropagatedWithContext(t *testing.T) {
	gen := newMockGenerator("cli", "go", nil)
	gen.err = fmt.Errorf("template rendering failed")

	eng := NewEngine()
	eng.Register(gen)

	dir := t.TempDir()
	result, err := eng.Generate(context.Background(), testManifest(), GenerateOptions{
		Mode:      "cli",
		Target:    "go",
		OutputDir: dir,
		Version:   "0.1.0",
	})

	require.Error(t, err, "Generator error must propagate")
	assert.Nil(t, result, "Result must be nil when generator errors")
	assert.Contains(t, err.Error(), "template rendering failed",
		"Wrapped error must contain the original generator error message")
}

// ---------------------------------------------------------------------------
// AC-12: .toolwright-generated marker file
// ---------------------------------------------------------------------------

func TestEngine_Generate_CreatesMarkerFile(t *testing.T) {
	gen := newMockGenerator("cli", "go", []GeneratedFile{
		{Path: "main.go", Content: []byte("package main")},
	})

	eng := NewEngine()
	eng.Register(gen)

	dir := t.TempDir()
	_, err := eng.Generate(context.Background(), testManifest(), GenerateOptions{
		Mode:      "cli",
		Target:    "go",
		OutputDir: dir,
		Version:   "1.2.3",
	})

	require.NoError(t, err)

	markerPath := filepath.Join(dir, markerFileName)
	info, err := os.Stat(markerPath)
	require.NoError(t, err, "Marker file %s must exist after generation", markerFileName)
	assert.False(t, info.IsDir(), "Marker must be a file, not a directory")
}

func TestEngine_Generate_MarkerFileContainsVersion(t *testing.T) {
	gen := newMockGenerator("cli", "go", []GeneratedFile{
		{Path: "main.go", Content: []byte("package main")},
	})

	eng := NewEngine()
	eng.Register(gen)

	dir := t.TempDir()
	_, err := eng.Generate(context.Background(), testManifest(), GenerateOptions{
		Mode:      "cli",
		Target:    "go",
		OutputDir: dir,
		Version:   "3.7.1",
	})

	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(dir, markerFileName))
	require.NoError(t, err)
	assert.Contains(t, string(content), "3.7.1",
		"Marker file must contain the Toolwright version")
}

func TestEngine_Generate_MarkerFileContainsTimestamp(t *testing.T) {
	gen := newMockGenerator("cli", "go", []GeneratedFile{
		{Path: "main.go", Content: []byte("package main")},
	})

	eng := NewEngine()
	eng.Register(gen)

	before := time.Now().UTC()

	dir := t.TempDir()
	_, err := eng.Generate(context.Background(), testManifest(), GenerateOptions{
		Mode:      "cli",
		Target:    "go",
		OutputDir: dir,
		Version:   "1.0.0",
	})

	require.NoError(t, err)

	after := time.Now().UTC()

	content, err := os.ReadFile(filepath.Join(dir, markerFileName))
	require.NoError(t, err)

	contentStr := string(content)
	// Marker must contain a timestamp. We check that at least the date portion
	// of either before or after is present (to avoid flaky second boundaries).
	// This catches an implementation that omits the timestamp entirely.
	datePrefix := before.Format("2006-01-02")
	datePrefixAfter := after.Format("2006-01-02")
	assert.True(t,
		strings.Contains(contentStr, datePrefix) || strings.Contains(contentStr, datePrefixAfter),
		"Marker file must contain a timestamp with today's date (expected %s or %s), got:\n%s",
		datePrefix, datePrefixAfter, contentStr,
	)
}

func TestEngine_Generate_MarkerFileContainsModeAndTarget(t *testing.T) {
	gen := newMockGenerator("mcp", "typescript", []GeneratedFile{
		{Path: "index.ts", Content: []byte("export {}")},
	})

	eng := NewEngine()
	eng.Register(gen)

	dir := t.TempDir()
	_, err := eng.Generate(context.Background(), testManifest(), GenerateOptions{
		Mode:      "mcp",
		Target:    "typescript",
		OutputDir: dir,
		Version:   "1.0.0",
	})

	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(dir, markerFileName))
	require.NoError(t, err)

	contentStr := string(content)
	assert.Contains(t, contentStr, "mcp",
		"Marker file must contain the generation mode")
	assert.Contains(t, contentStr, "typescript",
		"Marker file must contain the generation target")
}

func TestEngine_Generate_MarkerFileContainsAllRequiredFields(t *testing.T) {
	// Single test verifying all four required fields are present,
	// catching an implementation that includes some but not all.
	gen := newMockGenerator("cli", "go", []GeneratedFile{
		{Path: "main.go", Content: []byte("package main")},
	})

	eng := NewEngine()
	eng.Register(gen)

	dir := t.TempDir()
	_, err := eng.Generate(context.Background(), testManifest(), GenerateOptions{
		Mode:      "cli",
		Target:    "go",
		OutputDir: dir,
		Version:   "4.5.6",
	})

	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(dir, markerFileName))
	require.NoError(t, err)
	contentStr := string(content)

	assert.Contains(t, contentStr, "4.5.6", "Marker must contain version")
	assert.Contains(t, contentStr, "cli", "Marker must contain mode")
	assert.Contains(t, contentStr, "go", "Marker must contain target")
	// Timestamp: just verify it contains a year-like pattern from the current time.
	assert.Contains(t, contentStr, time.Now().UTC().Format("2006"),
		"Marker must contain a timestamp with the current year")
}

func TestEngine_Generate_MarkerFileIncludedInResult(t *testing.T) {
	gen := newMockGenerator("cli", "go", []GeneratedFile{
		{Path: "main.go", Content: []byte("package main")},
	})

	eng := NewEngine()
	eng.Register(gen)

	dir := t.TempDir()
	result, err := eng.Generate(context.Background(), testManifest(), GenerateOptions{
		Mode:      "cli",
		Target:    "go",
		OutputDir: dir,
		Version:   "1.0.0",
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Contains(t, result.Files, markerFileName,
		"Result.Files should include the marker file")
}

// ---------------------------------------------------------------------------
// AC-12: Existing marker without --force returns error
// ---------------------------------------------------------------------------

func TestEngine_Generate_ExistingMarker_NoForce_ReturnsError(t *testing.T) {
	gen := newMockGenerator("cli", "go", []GeneratedFile{
		{Path: "main.go", Content: []byte("package main")},
	})

	eng := NewEngine()
	eng.Register(gen)

	dir := t.TempDir()
	// Pre-create a marker file.
	markerPath := filepath.Join(dir, markerFileName)
	err := os.WriteFile(markerPath, []byte("existing marker"), 0644)
	require.NoError(t, err)

	result, err := eng.Generate(context.Background(), testManifest(), GenerateOptions{
		Mode:      "cli",
		Target:    "go",
		OutputDir: dir,
		Force:     false,
		Version:   "1.0.0",
	})

	require.Error(t, err, "Must error when marker exists and Force=false")
	assert.Nil(t, result, "Result must be nil on error")
	// The error should be helpful (Constitution 22).
	assert.Contains(t, err.Error(), "force",
		"Error should hint to use --force")
}

func TestEngine_Generate_ExistingMarker_NoForce_DoesNotCallGenerator(t *testing.T) {
	gen := newMockGenerator("cli", "go", []GeneratedFile{
		{Path: "main.go", Content: []byte("package main")},
	})

	eng := NewEngine()
	eng.Register(gen)

	dir := t.TempDir()
	markerPath := filepath.Join(dir, markerFileName)
	err := os.WriteFile(markerPath, []byte("existing marker"), 0644)
	require.NoError(t, err)

	_, _ = eng.Generate(context.Background(), testManifest(), GenerateOptions{
		Mode:      "cli",
		Target:    "go",
		OutputDir: dir,
		Force:     false,
		Version:   "1.0.0",
	})

	assert.Equal(t, 0, gen.callCount(),
		"Generator must NOT be called when marker exists and Force=false")
}

func TestEngine_Generate_ExistingMarker_NoForce_PreservesExistingFiles(t *testing.T) {
	gen := newMockGenerator("cli", "go", []GeneratedFile{
		{Path: "main.go", Content: []byte("package main")},
	})

	eng := NewEngine()
	eng.Register(gen)

	dir := t.TempDir()
	// Pre-create marker and an existing file.
	markerPath := filepath.Join(dir, markerFileName)
	err := os.WriteFile(markerPath, []byte("old marker"), 0644)
	require.NoError(t, err)
	existingFile := filepath.Join(dir, "existing.txt")
	err = os.WriteFile(existingFile, []byte("do not delete"), 0644)
	require.NoError(t, err)

	_, _ = eng.Generate(context.Background(), testManifest(), GenerateOptions{
		Mode:      "cli",
		Target:    "go",
		OutputDir: dir,
		Force:     false,
		Version:   "1.0.0",
	})

	// Existing files must be untouched.
	content, err := os.ReadFile(existingFile)
	require.NoError(t, err)
	assert.Equal(t, "do not delete", string(content),
		"Existing files must not be modified when generation is refused")
	// Marker must be unchanged.
	markerContent, err := os.ReadFile(markerPath)
	require.NoError(t, err)
	assert.Equal(t, "old marker", string(markerContent),
		"Existing marker must not be modified when Force=false")
}

// ---------------------------------------------------------------------------
// AC-12: Existing marker with --force overwrites
// ---------------------------------------------------------------------------

func TestEngine_Generate_ExistingMarker_WithForce_Succeeds(t *testing.T) {
	gen := newMockGenerator("cli", "go", []GeneratedFile{
		{Path: "main.go", Content: []byte("package main")},
	})

	eng := NewEngine()
	eng.Register(gen)

	dir := t.TempDir()
	markerPath := filepath.Join(dir, markerFileName)
	err := os.WriteFile(markerPath, []byte("old marker content"), 0644)
	require.NoError(t, err)

	result, err := eng.Generate(context.Background(), testManifest(), GenerateOptions{
		Mode:      "cli",
		Target:    "go",
		OutputDir: dir,
		Force:     true,
		Version:   "2.0.0",
	})

	require.NoError(t, err, "Generate with Force=true must succeed even with existing marker")
	require.NotNil(t, result)
	assert.Equal(t, 1, gen.callCount(),
		"Generator must be called when Force=true")

	// Marker should be overwritten with new content.
	newContent, err := os.ReadFile(markerPath)
	require.NoError(t, err)
	assert.Contains(t, string(newContent), "2.0.0",
		"Overwritten marker must contain the new version")
	assert.NotContains(t, string(newContent), "old marker content",
		"Overwritten marker must not contain old content")
}

// ---------------------------------------------------------------------------
// AC-14: Dry run outputs without writing
// ---------------------------------------------------------------------------

func TestEngine_Generate_DryRun_WritesNothingToDisk(t *testing.T) {
	gen := newMockGenerator("cli", "go", []GeneratedFile{
		{Path: "main.go", Content: []byte("package main")},
		{Path: "cmd/root.go", Content: []byte("package cmd")},
	})

	eng := NewEngine()
	eng.Register(gen)

	dir := t.TempDir()
	result, err := eng.Generate(context.Background(), testManifest(), GenerateOptions{
		Mode:      "cli",
		Target:    "go",
		OutputDir: dir,
		DryRun:    true,
		Version:   "1.0.0",
	})

	require.NoError(t, err)
	require.NotNil(t, result)

	// Verify NO files were written to disk.
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Empty(t, entries,
		"Dry run must not write any files to disk, but found: %v", dirEntryNames(entries))
}

func TestEngine_Generate_DryRun_ReturnsFileList(t *testing.T) {
	gen := newMockGenerator("cli", "go", []GeneratedFile{
		{Path: "main.go", Content: []byte("package main")},
		{Path: "cmd/root.go", Content: []byte("package cmd")},
	})

	eng := NewEngine()
	eng.Register(gen)

	dir := t.TempDir()
	result, err := eng.Generate(context.Background(), testManifest(), GenerateOptions{
		Mode:      "cli",
		Target:    "go",
		OutputDir: dir,
		DryRun:    true,
		Version:   "1.0.0",
	})

	require.NoError(t, err)
	require.NotNil(t, result)

	// File list should contain the files that WOULD be generated.
	assert.Contains(t, result.Files, "main.go",
		"Dry run result must list files that would be generated")
	assert.Contains(t, result.Files, "cmd/root.go",
		"Dry run result must list files that would be generated")
	assert.Contains(t, result.Files, markerFileName,
		"Dry run result must also list the marker file")
}

func TestEngine_Generate_DryRun_SetsDryRunFlag(t *testing.T) {
	gen := newMockGenerator("cli", "go", []GeneratedFile{
		{Path: "main.go", Content: []byte("package main")},
	})

	eng := NewEngine()
	eng.Register(gen)

	dir := t.TempDir()
	result, err := eng.Generate(context.Background(), testManifest(), GenerateOptions{
		Mode:      "cli",
		Target:    "go",
		OutputDir: dir,
		DryRun:    true,
		Version:   "1.0.0",
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.DryRun, "Result.DryRun must be true when dry-run is requested")
}

func TestEngine_Generate_NoDryRun_SetsDryRunFalse(t *testing.T) {
	gen := newMockGenerator("cli", "go", []GeneratedFile{
		{Path: "main.go", Content: []byte("package main")},
	})

	eng := NewEngine()
	eng.Register(gen)

	dir := t.TempDir()
	result, err := eng.Generate(context.Background(), testManifest(), GenerateOptions{
		Mode:      "cli",
		Target:    "go",
		OutputDir: dir,
		DryRun:    false,
		Version:   "1.0.0",
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.DryRun, "Result.DryRun must be false when dry-run is not requested")
}

func TestEngine_Generate_DryRun_DoesNotWriteMarkerFile(t *testing.T) {
	gen := newMockGenerator("cli", "go", []GeneratedFile{
		{Path: "main.go", Content: []byte("package main")},
	})

	eng := NewEngine()
	eng.Register(gen)

	dir := t.TempDir()
	_, err := eng.Generate(context.Background(), testManifest(), GenerateOptions{
		Mode:      "cli",
		Target:    "go",
		OutputDir: dir,
		DryRun:    true,
		Version:   "1.0.0",
	})

	require.NoError(t, err)

	markerPath := filepath.Join(dir, markerFileName)
	_, err = os.Stat(markerPath)
	assert.True(t, os.IsNotExist(err),
		"Dry run must not create the marker file on disk")
}

// ---------------------------------------------------------------------------
// AC-14: Dry run with existing marker does NOT error (it's informational only)
// ---------------------------------------------------------------------------

func TestEngine_Generate_DryRun_WithExistingMarker_NoForce_StillSucceeds(t *testing.T) {
	// Dry run is read-only, so an existing marker should not block it
	// even without --force. This catches an implementation that checks
	// the marker before checking dry-run.
	gen := newMockGenerator("cli", "go", []GeneratedFile{
		{Path: "main.go", Content: []byte("package main")},
	})

	eng := NewEngine()
	eng.Register(gen)

	dir := t.TempDir()
	markerPath := filepath.Join(dir, markerFileName)
	err := os.WriteFile(markerPath, []byte("old"), 0644)
	require.NoError(t, err)

	result, err := eng.Generate(context.Background(), testManifest(), GenerateOptions{
		Mode:      "cli",
		Target:    "go",
		OutputDir: dir,
		DryRun:    true,
		Force:     false,
		Version:   "1.0.0",
	})

	require.NoError(t, err,
		"Dry run must succeed even with existing marker and Force=false")
	require.NotNil(t, result)

	// Existing marker must not be modified.
	content, err := os.ReadFile(markerPath)
	require.NoError(t, err)
	assert.Equal(t, "old", string(content),
		"Dry run must not modify existing marker file")
}

// ---------------------------------------------------------------------------
// File writing: normal (non-dry-run) writes files to disk
// ---------------------------------------------------------------------------

func TestEngine_Generate_WritesGeneratedFilesToDisk(t *testing.T) {
	gen := newMockGenerator("cli", "go", []GeneratedFile{
		{Path: "main.go", Content: []byte("package main\n\nfunc main() {}\n")},
		{Path: "cmd/root.go", Content: []byte("package cmd\n")},
	})

	eng := NewEngine()
	eng.Register(gen)

	dir := t.TempDir()
	_, err := eng.Generate(context.Background(), testManifest(), GenerateOptions{
		Mode:      "cli",
		Target:    "go",
		OutputDir: dir,
		Version:   "1.0.0",
	})

	require.NoError(t, err)

	// Verify files were written with correct content.
	mainContent, err := os.ReadFile(filepath.Join(dir, "main.go"))
	require.NoError(t, err, "main.go must be written to disk")
	assert.Equal(t, "package main\n\nfunc main() {}\n", string(mainContent))

	rootContent, err := os.ReadFile(filepath.Join(dir, "cmd/root.go"))
	require.NoError(t, err, "cmd/root.go must be written (with subdirectory created)")
	assert.Equal(t, "package cmd\n", string(rootContent))
}

func TestEngine_Generate_CreatesSubdirectories(t *testing.T) {
	gen := newMockGenerator("cli", "go", []GeneratedFile{
		{Path: "deeply/nested/dir/file.go", Content: []byte("package deep")},
	})

	eng := NewEngine()
	eng.Register(gen)

	dir := t.TempDir()
	_, err := eng.Generate(context.Background(), testManifest(), GenerateOptions{
		Mode:      "cli",
		Target:    "go",
		OutputDir: dir,
		Version:   "1.0.0",
	})

	require.NoError(t, err)

	content, err := os.ReadFile(filepath.Join(dir, "deeply/nested/dir/file.go"))
	require.NoError(t, err, "Engine must create intermediate directories")
	assert.Equal(t, "package deep", string(content))
}

// ---------------------------------------------------------------------------
// Result metadata
// ---------------------------------------------------------------------------

func TestEngine_Generate_ResultContainsAllGeneratedFilePaths(t *testing.T) {
	gen := newMockGenerator("cli", "go", []GeneratedFile{
		{Path: "main.go", Content: []byte("package main")},
		{Path: "go.mod", Content: []byte("module example")},
		{Path: "internal/app.go", Content: []byte("package internal")},
	})

	eng := NewEngine()
	eng.Register(gen)

	dir := t.TempDir()
	result, err := eng.Generate(context.Background(), testManifest(), GenerateOptions{
		Mode:      "cli",
		Target:    "go",
		OutputDir: dir,
		Version:   "1.0.0",
	})

	require.NoError(t, err)
	require.NotNil(t, result)

	// Should have 3 generator files + 1 marker file = 4 total.
	assert.Len(t, result.Files, 4,
		"Result.Files should contain all generated files plus the marker file")
	assert.Contains(t, result.Files, "main.go")
	assert.Contains(t, result.Files, "go.mod")
	assert.Contains(t, result.Files, "internal/app.go")
	assert.Contains(t, result.Files, markerFileName)
}

func TestEngine_Generate_ResultModeAndTarget(t *testing.T) {
	gen := newMockGenerator("mcp", "typescript", []GeneratedFile{
		{Path: "index.ts", Content: []byte("export {}")},
	})

	eng := NewEngine()
	eng.Register(gen)

	dir := t.TempDir()
	result, err := eng.Generate(context.Background(), testManifest(), GenerateOptions{
		Mode:      "mcp",
		Target:    "typescript",
		OutputDir: dir,
		Version:   "1.0.0",
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "mcp", result.Mode)
	assert.Equal(t, "typescript", result.Target)
}

// ---------------------------------------------------------------------------
// Edge cases
// ---------------------------------------------------------------------------

func TestEngine_Generate_GeneratorReturnsNoFiles(t *testing.T) {
	// Generator returns zero files. The marker should still be written.
	gen := newMockGenerator("cli", "go", []GeneratedFile{})

	eng := NewEngine()
	eng.Register(gen)

	dir := t.TempDir()
	result, err := eng.Generate(context.Background(), testManifest(), GenerateOptions{
		Mode:      "cli",
		Target:    "go",
		OutputDir: dir,
		Version:   "1.0.0",
	})

	require.NoError(t, err)
	require.NotNil(t, result)

	// Marker file should still be created even with zero generated files.
	markerPath := filepath.Join(dir, markerFileName)
	_, err = os.Stat(markerPath)
	require.NoError(t, err, "Marker must be written even when generator returns zero files")

	// Result should contain just the marker.
	assert.Contains(t, result.Files, markerFileName)
}

func TestEngine_Generate_ContextCancelled(t *testing.T) {
	gen := newMockGenerator("cli", "go", []GeneratedFile{
		{Path: "main.go", Content: []byte("package main")},
	})

	eng := NewEngine()
	eng.Register(gen)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	dir := t.TempDir()
	_, err := eng.Generate(ctx, testManifest(), GenerateOptions{
		Mode:      "cli",
		Target:    "go",
		OutputDir: dir,
		Version:   "1.0.0",
	})

	// A well-behaved engine should respect context cancellation.
	// We accept either an error or context.Canceled behavior.
	// The key test is that it does not panic.
	if err != nil {
		assert.ErrorIs(t, err, context.Canceled,
			"Error from cancelled context should wrap context.Canceled")
	}
}

func TestEngine_Generate_TimestampIsRFC3339OrISO8601(t *testing.T) {
	// Verify the timestamp passed to the generator is parseable, not arbitrary.
	gen := newMockGenerator("cli", "go", []GeneratedFile{
		{Path: "main.go", Content: []byte("package main")},
	})

	eng := NewEngine()
	eng.Register(gen)

	dir := t.TempDir()
	_, err := eng.Generate(context.Background(), testManifest(), GenerateOptions{
		Mode:      "cli",
		Target:    "go",
		OutputDir: dir,
		Version:   "1.0.0",
	})

	require.NoError(t, err)
	require.Equal(t, 1, gen.callCount())

	ts := gen.lastCall().Data.Timestamp
	require.NotEmpty(t, ts, "Timestamp must not be empty")

	// Try parsing as RFC3339 (the standard Go format).
	_, parseErr := time.Parse(time.RFC3339, ts)
	if parseErr != nil {
		// Also accept RFC3339Nano.
		_, parseErr = time.Parse(time.RFC3339Nano, ts)
	}
	assert.NoError(t, parseErr,
		"Timestamp %q must be parseable as RFC3339 or RFC3339Nano", ts)
}

func TestEngine_Register_SameModeTarget_LastWins(t *testing.T) {
	// Registering two generators with the same mode/target: last one wins.
	// This is a design decision test — if the behavior should be "error on
	// duplicate" instead, this test documents the expected behavior either way.
	gen1 := newMockGenerator("cli", "go", []GeneratedFile{
		{Path: "from-gen1.go", Content: []byte("gen1")},
	})
	gen2 := newMockGenerator("cli", "go", []GeneratedFile{
		{Path: "from-gen2.go", Content: []byte("gen2")},
	})

	eng := NewEngine()
	eng.Register(gen1)
	eng.Register(gen2)

	dir := t.TempDir()
	result, err := eng.Generate(context.Background(), testManifest(), GenerateOptions{
		Mode:      "cli",
		Target:    "go",
		OutputDir: dir,
		Version:   "1.0.0",
	})

	require.NoError(t, err)
	require.NotNil(t, result)
	// Exactly one generator should have been called.
	totalCalls := gen1.callCount() + gen2.callCount()
	assert.Equal(t, 1, totalCalls,
		"Only one generator should be called when duplicates are registered")
	// The last registered should win.
	assert.Equal(t, 1, gen2.callCount(),
		"Last registered generator for same mode/target should be used")
}

func TestEngine_Generate_EmptyVersion(t *testing.T) {
	// Version is empty string — engine should still work (or error clearly).
	gen := newMockGenerator("cli", "go", []GeneratedFile{
		{Path: "main.go", Content: []byte("package main")},
	})

	eng := NewEngine()
	eng.Register(gen)

	dir := t.TempDir()
	result, err := eng.Generate(context.Background(), testManifest(), GenerateOptions{
		Mode:      "cli",
		Target:    "go",
		OutputDir: dir,
		Version:   "",
	})

	// Either succeeds with empty version or errors clearly.
	if err != nil {
		assert.Contains(t, strings.ToLower(err.Error()), "version",
			"If empty version errors, the error should mention 'version'")
	} else {
		require.NotNil(t, result)
		// If it succeeds, the version in TemplateData should be empty.
		assert.Equal(t, "", gen.lastCall().Data.Version)
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func dirEntryNames(entries []os.DirEntry) []string {
	names := make([]string, len(entries))
	for i, e := range entries {
		names[i] = e.Name()
	}
	return names
}
