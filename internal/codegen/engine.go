package codegen

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Obsidian-Owl/toolwright/internal/manifest"
)

// Generator is implemented by each code generation target.
type Generator interface {
	Generate(ctx context.Context, data TemplateData, outputDir string) ([]GeneratedFile, error)
	Mode() string   // "cli" or "mcp"
	Target() string // "go", "typescript"
}

// TemplateData holds all data passed to templates.
type TemplateData struct {
	Manifest  manifest.Toolkit
	Timestamp string
	Version   string
}

// GeneratedFile represents a single file to be generated.
type GeneratedFile struct {
	Path    string // relative path within output dir
	Content []byte
}

// GenerateOptions controls generation behavior.
type GenerateOptions struct {
	Mode      string
	Target    string
	OutputDir string
	Force     bool
	DryRun    bool
	Version   string
}

// GenerateResult is the output of generation.
type GenerateResult struct {
	Files  []string // relative paths written (or that would be written in dry-run)
	DryRun bool
	Mode   string
	Target string
}

const markerFile = ".toolwright-generated"

// Engine dispatches generation to registered generators.
type Engine struct {
	generators map[string]Generator
}

// NewEngine returns a new Engine.
func NewEngine() *Engine {
	return &Engine{
		generators: make(map[string]Generator),
	}
}

// Register adds a generator to the engine. If a generator with the same
// mode/target is already registered, the new one replaces it.
func (e *Engine) Register(g Generator) {
	key := g.Mode() + "/" + g.Target()
	e.generators[key] = g
}

// Generate dispatches to the appropriate registered generator.
func (e *Engine) Generate(ctx context.Context, m manifest.Toolkit, opts GenerateOptions) (*GenerateResult, error) {
	// Check context cancellation first.
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("generation cancelled: %w", err)
	}

	// Check for existing marker file when not doing a dry run.
	if !opts.DryRun {
		markerPath := filepath.Join(opts.OutputDir, markerFile)
		if _, err := os.Stat(markerPath); err == nil {
			// Marker exists.
			if !opts.Force {
				return nil, fmt.Errorf(
					"output directory %q already contains a generated project (found %s): re-run with --force to overwrite",
					opts.OutputDir, markerFile,
				)
			}
		}
	}

	// Find the matching generator.
	key := opts.Mode + "/" + opts.Target
	gen, ok := e.generators[key]
	if !ok {
		available := make([]string, 0, len(e.generators))
		for k := range e.generators {
			available = append(available, k)
		}
		return nil, fmt.Errorf(
			"no generator registered for mode=%q target=%q: available combinations are [%s]",
			opts.Mode, opts.Target, strings.Join(available, ", "),
		)
	}

	// Build template data.
	data := TemplateData{
		Manifest:  m,
		Version:   opts.Version,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}

	// Call the generator.
	files, err := gen.Generate(ctx, data, opts.OutputDir)
	if err != nil {
		return nil, fmt.Errorf("generator %s/%s failed: %w", opts.Mode, opts.Target, err)
	}

	// Collect result file paths.
	resultFiles := make([]string, 0, len(files)+1)
	for _, f := range files {
		resultFiles = append(resultFiles, f.Path)
	}
	// Always include marker in the result paths.
	resultFiles = append(resultFiles, markerFile)

	if !opts.DryRun {
		// Write generated files to disk.
		for _, f := range files {
			destPath := filepath.Join(opts.OutputDir, f.Path)
			if mkErr := os.MkdirAll(filepath.Dir(destPath), 0755); mkErr != nil {
				return nil, fmt.Errorf("creating directory for %s: %w", f.Path, mkErr)
			}
			if writeErr := os.WriteFile(destPath, f.Content, 0644); writeErr != nil {
				return nil, fmt.Errorf("writing file %s: %w", f.Path, writeErr)
			}
		}

		// Write marker file.
		markerContent := fmt.Sprintf("version: %s\ntimestamp: %s\nmode: %s\ntarget: %s\n",
			opts.Version, data.Timestamp, opts.Mode, opts.Target)
		markerPath := filepath.Join(opts.OutputDir, markerFile)
		if writeErr := os.WriteFile(markerPath, []byte(markerContent), 0644); writeErr != nil {
			return nil, fmt.Errorf("writing marker file: %w", writeErr)
		}
	}

	return &GenerateResult{
		Files:  resultFiles,
		DryRun: opts.DryRun,
		Mode:   opts.Mode,
		Target: opts.Target,
	}, nil
}
