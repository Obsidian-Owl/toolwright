package codegen

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// findFile returns the GeneratedFile with the given path, or nil if not found.
func findFile(files []GeneratedFile, path string) *GeneratedFile {
	for i := range files {
		if files[i].Path == path {
			return &files[i]
		}
	}
	return nil
}

// requireFile returns the GeneratedFile with the given path, failing the test
// if not found. Uses t.Helper so the failure points to the caller.
func requireFile(t *testing.T, files []GeneratedFile, path string) GeneratedFile {
	t.Helper()
	f := findFile(files, path)
	require.NotNilf(t, f, "expected file %q in generated output, got paths: %v",
		path, filePaths(files))
	return *f
}

// filePaths returns a slice of all paths in the generated files.
func filePaths(files []GeneratedFile) []string {
	paths := make([]string, len(files))
	for i, f := range files {
		paths[i] = f.Path
	}
	return paths
}

// fileContent returns the string content of a generated file by path, failing
// the test if the file is missing.
func fileContent(t *testing.T, files []GeneratedFile, path string) string {
	t.Helper()
	f := requireFile(t, files, path)
	return string(f.Content)
}

// assertNoFile asserts that the given path does NOT appear in the output.
func assertNoFile(t *testing.T, files []GeneratedFile, path string) {
	t.Helper()
	f := findFile(files, path)
	assert.Nilf(t, f, "unexpected file %q in generated output", path)
}
