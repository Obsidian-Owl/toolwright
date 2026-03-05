# Context: Unit 1 — scaffold

## Purpose

Implement `internal/scaffold/` package and create embedded template files in `templates/init/`. The scaffolder creates complete, working Toolwright projects from templates.

## Interface to Implement

From `internal/cli/init.go`:

```go
type scaffolder interface {
    Scaffold(ctx context.Context, opts ScaffoldOptions) (*ScaffoldResult, error)
}

type ScaffoldOptions struct {
    Name        string
    Description string
    OutputDir   string
    Runtime     string  // "shell", "go", "python", "typescript"
    Auth        string  // "none", "token", "oauth2"
}

type ScaffoldResult struct {
    Dir   string   // Directory created
    Files []string // List of file paths created
}
```

The CLI layer (`runInit`) validates runtime before calling Scaffold(). The scaffolder does NOT need to re-validate runtime values.

## Scaffolded Project Structure (docs/spec.md §3.1)

```
<n>/
├── toolwright.yaml          # Valid manifest
├── bin/hello                # Entrypoint stub (executable)
├── schemas/hello-output.json  # Output JSON Schema
├── tests/hello.test.yaml    # Passing test scenario
└── README.md
```

### Runtime-specific behavior

`bin/hello` content varies by runtime:
- **shell**: `#!/bin/bash` printing `{"message":"hello"}`
- **go**: `#!/bin/bash` wrapper running `go run ./src/hello/main.go "$@"`. Additional file: `src/hello/main.go`
- **python**: `#!/usr/bin/env python3` printing `{"message":"hello"}`
- **typescript**: `#!/usr/bin/env npx ts-node` printing JSON. Additional files: `src/hello/index.ts`, `package.json`

### Auth in manifest

When `opts.Auth` is "token" or "oauth2", the generated `toolwright.yaml` includes an auth block. The hello stub itself doesn't require auth — it just demonstrates the manifest structure.

## Template Architecture

- Templates live in `templates/init/` (embedded via `go:embed`)
- `.tmpl` files are processed through `text/template`; other files copied verbatim
- The scaffold package accepts `fs.FS` (Constitution 17a) — NOT `embed.FS`
- Tests use `fstest.MapFS` with inline template content (Pattern P2/Constitution 17a)

## embed.go

Current (`embed.go` at module root):
```go
package toolwright

import "embed"

//go:embed schemas/*
var Schemas embed.FS
```

Must add:
```go
//go:embed all:templates/init
var InitTemplates embed.FS
```

## Validation Contracts

The scaffolded project must pass these when run from the generated directory:
- `toolwright validate` — valid manifest, entrypoint exists and is executable
- `toolwright run hello` — prints `{"message":"hello"}` and exits 0
- `toolwright test` — test scenario passes

## Failure Handling

- **Existing directory**: Fail fast with clear error if target directory exists
- **Atomic writes**: Render all templates first, write files only after all templates succeed
- **Permissions**: Set executable bit on `bin/hello` (0755)

## Key Files

| File | Purpose |
|------|---------|
| `internal/cli/init.go` | scaffolder interface (lines 27-29), ScaffoldOptions (11-17), ScaffoldResult (20-23) |
| `internal/cli/init_test.go` | mockScaffolder implementation, test expectations |
| `internal/manifest/types.go` | Toolkit struct for toolwright.yaml format reference |
| `internal/manifest/validate.go` | Validate() rules the manifest must satisfy |
| `embed.go` | Extend with InitTemplates |
| `templates/init/` | Currently empty, populate with template files |

## Gotchas

1. `go:embed` with `*` is not recursive — use `all:templates/init` for subdirectories
2. `text/template` interprets `{{` in data — pass data via struct, never interpolation
3. Template file naming: use `.tmpl` suffix for files needing variable substitution
4. `os.MkdirAll` for nested directories (`bin/`, `schemas/`, `tests/`, `src/hello/`)
5. `os.Chmod(path, 0755)` for executable entrypoints — cross-platform consideration
6. Constitution rule 2: no init() functions in scaffolded Go code
