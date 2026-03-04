# Plan: Unit 1 — scaffold

## Tasks

### Task 1: Template files

Create all template files under `templates/init/`:

```
templates/init/
├── toolwright.yaml.tmpl       # Manifest template (runtime/auth-aware)
├── hello-output.schema.json   # Static JSON Schema for hello output
├── hello.test.yaml.tmpl       # Test scenario template
├── README.md.tmpl             # Project README
├── shell/hello.sh.tmpl        # Shell entrypoint
├── go/hello.sh.tmpl           # Go wrapper script
├── go/main.go.tmpl            # Go source
├── python/hello.py.tmpl       # Python entrypoint
├── typescript/hello.ts.tmpl   # TypeScript source
├── typescript/hello.sh.tmpl   # TypeScript wrapper script
└── typescript/package.json.tmpl  # TypeScript package.json
```

Template data struct:
```go
type templateData struct {
    Name        string
    Description string
    Runtime     string
    Auth        string // "none", "token", "oauth2"
}
```

### Task 2: Scaffold package

New package `internal/scaffold/`:

```go
// scaffold.go
type Scaffolder struct {
    templates fs.FS
}

func New(templates fs.FS) *Scaffolder

func (s *Scaffolder) Scaffold(ctx context.Context, opts cli.ScaffoldOptions) (*cli.ScaffoldResult, error)
```

Internal helpers:
- `renderTemplates(data templateData, runtime string) (map[string][]byte, error)` — render all templates, return path→content map
- `writeFiles(dir string, files map[string][]byte) ([]string, error)` — write files atomically, set permissions

### Task 3: embed.go + integration tests

- Update `embed.go` to add `InitTemplates embed.FS`
- Integration tests: scaffold each runtime → verify file structure, content, permissions
- Verify rendered `toolwright.yaml` is valid per `manifest.Validate()`

## File Change Map

| File | Action |
|------|--------|
| `templates/init/*.tmpl` | CREATE — all template files |
| `templates/init/{shell,go,python,typescript}/*.tmpl` | CREATE — runtime-specific templates |
| `templates/init/hello-output.schema.json` | CREATE — static schema |
| `internal/scaffold/scaffold.go` | CREATE — Scaffolder implementation |
| `internal/scaffold/scaffold_test.go` | CREATE — unit + integration tests |
| `embed.go` | EDIT — add InitTemplates |

## Architecture Decisions

1. **fs.FS over embed.FS**: The Scaffolder accepts `fs.FS` so tests can use `fstest.MapFS` (Constitution 17a)
2. **Render-then-write**: All templates rendered to memory first; files written only if all succeed
3. **Runtime subdirectories in templates**: Keep runtime-specific templates separate from shared ones
4. **No template processing for static files**: `hello-output.schema.json` is copied verbatim

## As-Built Notes

### Plan Deviations
- Tasks 1+2 merged per pattern P1 (tightly coupled test surface). Template files and scaffold package tested together since tests use `fstest.MapFS` with inline template content.
- Integration tests placed in separate file `integration_test.go` (same package) rather than appended to `scaffold_test.go`.

### Implementation Decisions
- Explicit `templateEntry` table maps template paths to output paths (no path-derivation heuristics). `sharedEntries()` and `runtimeEntries(runtime)` functions return the entries.
- `static` field on `templateEntry` gates verbatim copy vs `text/template` processing.
- `upper` template function registered for `{{.Name | upper}}` in manifest token auth.
- Two-phase: render all to `[]renderedFile` in memory, then write. Context checked at entry and before write phase.
- Pre-commit hook caught `govet` shadow warnings in integration tests — fixed with variable renaming.

### Actual Files
| File | Lines |
|------|-------|
| `internal/scaffold/scaffold.go` | ~215 |
| `internal/scaffold/scaffold_test.go` | ~1587 |
| `internal/scaffold/integration_test.go` | ~470 |
| `embed.go` | +4 lines (InitTemplates) |
| `templates/init/` | 11 template files |
