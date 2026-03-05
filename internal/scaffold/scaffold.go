package scaffold

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/template"
)

// ScaffoldOptions describes what the scaffolder should create.
type ScaffoldOptions struct {
	Name        string
	Description string
	OutputDir   string
	Runtime     string
	Auth        string
}

// ScaffoldResult describes what the scaffolder created.
type ScaffoldResult struct {
	Dir   string
	Files []string
}

// templateData is the data struct passed to every template during rendering.
type templateData struct {
	Name        string
	Description string
	Runtime     string
	Auth        string
}

// renderedFile holds the output path and content of a rendered template.
type renderedFile struct {
	relPath    string
	content    []byte
	executable bool
}

// templateEntry maps a template path in the FS to an output relative path and
// whether it should be executable.
type templateEntry struct {
	tmplPath   string // path inside fs.FS
	outPath    string // relative output path (no project prefix)
	executable bool
	static     bool // copy verbatim, do not template-process
}

// sharedEntries returns template entries that are included for every runtime.
func sharedEntries() []templateEntry {
	return []templateEntry{
		{
			tmplPath: "templates/init/toolwright.yaml.tmpl",
			outPath:  "toolwright.yaml",
		},
		{
			tmplPath: "templates/init/hello-output.schema.json",
			outPath:  "schemas/hello-output.json",
			static:   true,
		},
		{
			tmplPath: "templates/init/hello.test.yaml.tmpl",
			outPath:  "tests/hello.test.yaml",
			static:   true,
		},
		{
			tmplPath: "templates/init/README.md.tmpl",
			outPath:  "README.md",
		},
	}
}

// runtimeEntries returns template entries specific to the given runtime.
func runtimeEntries(runtime string) []templateEntry {
	switch runtime {
	case "shell":
		return []templateEntry{
			{
				tmplPath:   "templates/init/shell/hello.sh.tmpl",
				outPath:    "bin/hello",
				executable: true,
			},
		}
	case "go":
		return []templateEntry{
			{
				tmplPath:   "templates/init/go/hello.sh.tmpl",
				outPath:    "bin/hello",
				executable: true,
			},
			{
				tmplPath: "templates/init/go/main.go.tmpl",
				outPath:  "src/hello/main.go",
			},
		}
	case "python":
		return []templateEntry{
			{
				tmplPath:   "templates/init/python/hello.py.tmpl",
				outPath:    "bin/hello",
				executable: true,
			},
		}
	case "typescript":
		return []templateEntry{
			{
				tmplPath:   "templates/init/typescript/hello.sh.tmpl",
				outPath:    "bin/hello",
				executable: true,
			},
			{
				tmplPath: "templates/init/typescript/hello.ts.tmpl",
				outPath:  "src/hello/index.ts",
			},
			{
				tmplPath: "templates/init/typescript/package.json.tmpl",
				outPath:  "package.json",
			},
		}
	default:
		return nil
	}
}

// Scaffolder creates new toolwright projects from templates.
type Scaffolder struct {
	templates fs.FS
}

// New creates a Scaffolder that reads templates from the given filesystem.
// The fs.FS should contain templates rooted at "templates/init/".
func New(templates fs.FS) *Scaffolder {
	return &Scaffolder{templates: templates}
}

// Scaffold creates a new project directory with rendered template files.
func (s *Scaffolder) Scaffold(ctx context.Context, opts ScaffoldOptions) (*ScaffoldResult, error) {
	// Check context before doing any work.
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context cancelled: %w", err)
	}

	// Validate required fields.
	if opts.Name == "" {
		return nil, fmt.Errorf("project name must not be empty")
	}

	// Resolve output directory: use current directory when empty.
	outputDir := opts.OutputDir
	if outputDir == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, fmt.Errorf("get working directory: %w", err)
		}
		outputDir = cwd
	}

	// Verify the output directory exists.
	if _, err := os.Stat(outputDir); err != nil {
		return nil, fmt.Errorf("output directory %q: %w", outputDir, err)
	}

	// Compute the target project directory.
	projectDir := filepath.Join(outputDir, opts.Name)

	// Reject names that escape the output directory (path traversal defense).
	// Use a Clean+prefix check rather than filepath.Rel so that names like
	// "..foo" (safe) are accepted while "../../escape" (unsafe) are rejected.
	if !strings.HasPrefix(
		filepath.Clean(projectDir)+string(filepath.Separator),
		filepath.Clean(outputDir)+string(filepath.Separator),
	) {
		return nil, fmt.Errorf("project name %q would escape output directory", opts.Name)
	}

	// Reject if the project directory already exists.
	if _, err := os.Stat(projectDir); err == nil {
		return nil, fmt.Errorf("directory %q already exists", projectDir)
	}

	// Validate runtime.
	switch opts.Runtime {
	case "shell", "go", "python", "typescript":
		// valid
	default:
		return nil, fmt.Errorf("unknown runtime %q: must be one of shell, go, python, typescript", opts.Runtime)
	}

	// Collect all template entries for this runtime.
	entries := append(sharedEntries(), runtimeEntries(opts.Runtime)...)

	data := templateData{
		Name:        opts.Name,
		Description: opts.Description,
		Runtime:     opts.Runtime,
		Auth:        opts.Auth,
	}

	// Template function map.
	funcMap := template.FuncMap{
		"upper": strings.ToUpper,
		// jsonEscape marshals s as a JSON string and strips the surrounding quotes,
		// making it safe for interpolation inside JSON string literals.
		"jsonEscape": func(s string) (string, error) {
			b, err := json.Marshal(s)
			if err != nil {
				return "", err
			}
			return string(b[1 : len(b)-1]), nil
		},
		// yamlEscape escapes s for use inside a YAML double-quoted scalar:
		// backslashes and double-quotes are escaped, newlines become \n.
		"yamlEscape": func(s string) string {
			s = strings.ReplaceAll(s, `\`, `\\`)
			s = strings.ReplaceAll(s, `"`, `\"`)
			s = strings.ReplaceAll(s, "\n", `\n`)
			return s
		},
	}

	// Render all templates to memory first (atomicity: render before writing).
	rendered := make([]renderedFile, 0, len(entries))
	for _, entry := range entries {
		raw, err := fs.ReadFile(s.templates, entry.tmplPath)
		if err != nil {
			return nil, fmt.Errorf("read template %q: %w", entry.tmplPath, err)
		}

		var content []byte
		if entry.static {
			// Static files are copied verbatim.
			content = raw
		} else {
			// Template files are processed through text/template.
			tmpl, err := template.New(entry.tmplPath).Funcs(funcMap).Parse(string(raw))
			if err != nil {
				return nil, fmt.Errorf("parse template %q: %w", entry.tmplPath, err)
			}
			var buf bytes.Buffer
			if err := tmpl.Execute(&buf, data); err != nil {
				return nil, fmt.Errorf("render template %q: %w", entry.tmplPath, err)
			}
			content = buf.Bytes()
		}

		rendered = append(rendered, renderedFile{
			relPath:    entry.outPath,
			content:    content,
			executable: entry.executable,
		})
	}

	// Check context again before writing files.
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("context cancelled before writing files: %w", err)
	}

	// All templates rendered successfully. Now write files.
	// If any write fails, clean up the partially-created project directory
	// so the caller can retry with the same name.
	writeErr := func() error {
		for _, rf := range rendered {
			fullPath := filepath.Join(projectDir, rf.relPath)

			// Ensure parent directory exists.
			if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
				return fmt.Errorf("create directory for %q: %w", rf.relPath, err)
			}

			mode := os.FileMode(0644)
			if rf.executable {
				mode = 0755
			}

			if err := os.WriteFile(fullPath, rf.content, mode); err != nil {
				return fmt.Errorf("write file %q: %w", rf.relPath, err)
			}
		}
		return nil
	}()
	if writeErr != nil {
		_ = os.RemoveAll(projectDir) // best-effort cleanup
		return nil, writeErr
	}

	// Collect sorted relative file paths.
	files := make([]string, len(rendered))
	for i, rf := range rendered {
		files[i] = rf.relPath
	}
	sort.Strings(files)

	absDir, err := filepath.Abs(projectDir)
	if err != nil {
		return nil, fmt.Errorf("resolve absolute path for %q: %w", projectDir, err)
	}

	return &ScaffoldResult{
		Dir:   absDir,
		Files: files,
	}, nil
}
